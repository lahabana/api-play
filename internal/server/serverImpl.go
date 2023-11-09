package server

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/lahabana/api-play/pkg/api"
	"io"
	"log/slog"
	"maps"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"sync/atomic"
	"time"
)

var version = "dev"
var commit = "dev"

type srv struct {
	status atomic.Int32
	apis   atomic.Pointer[map[string]api.ConfigureAPI]
	rand   *rand.Rand
	l      *slog.Logger
}

func (s *srv) Reload(ctx context.Context, apis api.ParamsAPI) error {
	apis.Normalize()
	s.l.InfoContext(ctx, "validation with new config", "config", apis.Apis[0].Conf.Latency)
	if err := apis.Validate(); err != nil {
		return err
	}
	newApis := map[string]api.ConfigureAPI{}
	for _, item := range apis.Apis {
		newApis[item.Path] = item.Conf
	}
	s.apis.Store(&newApis)
	s.l.InfoContext(ctx, "reloaded with new config", "config", apis)
	return nil
}

func (s *srv) Home(c *gin.Context) {
	host, _ := os.Hostname()
	c.PureJSON(http.StatusOK, api.HomeResponse{
		Hostname: host,
		Version:  version,
		Commit:   commit,
		Target:   fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	})
}

func (s *srv) ParamsApi(c *gin.Context) {
	apis := *s.apis.Load()

	var keys []string
	for k := range apis {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := api.ParamsAPI{
		Apis: []api.ConfigureAPIItem{},
	}
	for _, k := range keys {
		out.Apis = append(out.Apis, api.ConfigureAPIItem{Conf: apis[k], Path: k})
	}
	c.PureJSON(http.StatusOK, out)
}

func (s *srv) GetApi(c *gin.Context, path string) {
	entry, exists := (*s.apis.Load())[path]
	if !exists {
		c.PureJSON(http.StatusNotFound, api.ErrorResponse{Status: http.StatusNotFound, Details: fmt.Sprintf("No such api at: %s", path)})
		return
	}
	latency := time.Duration(0)
	if entry.Latency != nil { // .MaxMillis != 0 {
		latency = time.Duration(s.rand.Intn(entry.Latency.MaxMillis-entry.Latency.MinMillis)+entry.Latency.MinMillis) * time.Millisecond
	}
	if latency != 0 {
		time.Sleep(latency)
	}
	callStatus := http.StatusOK
	var calls []api.CallOutcome
	for _, call := range entry.Call {
		resp, err := http.Get(call.Url)
		outcome := api.CallOutcome{
			Url: call.Url,
		}
		if err != nil {
			outcome.Status = http.StatusInternalServerError
		} else {
			outcome.Status = resp.StatusCode
			if !call.TrimBody {
				b, _ := io.ReadAll(resp.Body)
				sb := string(b)
				outcome.Body = &sb
			}
			_ = resp.Body.Close()
		}
		// The worst status from children calls defines the status of type 'inherit'
		if !call.IgnoreStatus && outcome.Status > callStatus {
			callStatus = outcome.Status
		}
		calls = append(calls, outcome)
	}

	// Default to the status of the children
	status := callStatus
	n := s.rand.Intn(100000)
	for _, v := range entry.Statuses {
		if n < v.Ratio {
			if v.Code == "inherit" {
				status = callStatus
			} else {
				status, _ = strconv.Atoi(v.Code)
			}
			break
		}
		n -= v.Ratio
	}

	out := api.APIResponse{
		Body:          entry.Body,
		LatencyMillis: int(latency.Milliseconds()),
		Status:        status,
		Calls:         calls,
	}
	c.PureJSON(out.Status, out)
}

func (s *srv) ConfigureApi(c *gin.Context, path string) {
	ctx := c.Request.Context()
	req := api.ConfigureAPI{}
	err := c.Bind(&req)
	if err != nil {
		s.l.InfoContext(ctx, "failed", "req", req.Latency)
		c.PureJSON(http.StatusBadRequest, api.BadRequestResponse(err))
		return
	}
	req.Normalize()

	oldApi := s.apis.Load()
	_, exists := (*oldApi)[path]
	if exists {
		s.l.InfoContext(ctx, "overriding existing API, this will not be persisted across reloads of the config and restarts")
	}
	newApi := map[string]api.ConfigureAPI{}
	maps.Copy(newApi, *oldApi)
	newApi[path] = req
	if !s.apis.CompareAndSwap(oldApi, &newApi) {
		c.PureJSON(http.StatusConflict, api.ErrorResponse{
			Status:  http.StatusConflict,
			Details: "Concurrent updates to api list",
		})
		return
	}

	c.PureJSON(http.StatusOK, api.ConfigureAPIItem{
		Conf: req,
		Path: path,
	})
}

func (s *srv) Health(c *gin.Context) {
	st := int(s.status.Load())
	c.PureJSON(st, api.HealthResponse{Status: st})
}

func (s *srv) DegradeHealth(c *gin.Context) {
	req := api.DegradeHealthJSONRequestBody{}
	err := c.Bind(&req)
	if err != nil {
		c.PureJSON(http.StatusBadRequest, api.BadRequestResponse(err))
		return
	} else {
		st := req.Status
		if st == 0 {
			st = http.StatusOK
		}
		s.status.Store(int32(st))
		c.PureJSON(http.StatusOK, api.HealthResponse{Status: st})
	}
}

func NewServerImpl(l *slog.Logger, seed int64) api.ServerInterface {
	s := &srv{
		l:      l.WithGroup("api-server"),
		status: atomic.Int32{},
		apis:   atomic.Pointer[map[string]api.ConfigureAPI]{},
		rand:   rand.New(rand.NewSource(seed)),
	}
	s.status.Store(http.StatusOK)
	s.apis.Store(&map[string]api.ConfigureAPI{})
	return s
}
