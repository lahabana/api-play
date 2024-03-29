package server

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/lahabana/api-play/internal/version"
	"github.com/lahabana/api-play/pkg/api"
	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"io"
	"log/slog"
	"maps"
	"math/rand"
	"net/http"
	"net/http/httptrace"
	"os"
	"runtime"
	"sort"
	"strconv"
	"sync/atomic"
	"time"
)

type srv struct {
	healthStatus atomic.Int32
	readyStatus  atomic.Int32
	apis         atomic.Pointer[map[string]api.ConfigureAPI]
	rand         *rand.Rand
	l            *slog.Logger
}

func (s *srv) Reload(ctx context.Context, apis api.ParamsAPI) error {
	apis.Normalize()
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
		Version:  version.Version,
		Commit:   version.Commit,
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
		outcome := s.call(c.Request.Context(), call)
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
	handleHealth(c, &s.healthStatus)
}

func (s *srv) DegradeHealth(c *gin.Context) {
	degradeHealth(c, &s.healthStatus)
}

func (s *srv) Ready(c *gin.Context) {
	handleHealth(c, &s.readyStatus)
}

func (s *srv) DegradeReady(c *gin.Context) {
	degradeHealth(c, &s.readyStatus)
}

func (s *srv) call(ctx context.Context, call api.CallDef) api.CallOutcome {
	outcome := api.CallOutcome{
		Url: call.Url,
	}
	ctx, span := otel.Tracer("serverImpl").Start(ctx, "service-call")
	defer span.End()
	span.SetAttributes(attribute.Key("url").String(call.Url))
	ctx = httptrace.WithClientTrace(ctx, otelhttptrace.NewClientTrace(ctx))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, call.Url, nil)
	if err != nil {
		s.l.ErrorContext(ctx, "failed to create request", "error", err)
		outcome.Status = http.StatusInternalServerError
	} else {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			outcome.Status = http.StatusInternalServerError
		} else {
			span.SetStatus(codes.Ok, fmt.Sprintf("got http status: %d", resp.StatusCode))
			outcome.Status = resp.StatusCode
			if !call.TrimBody {
				b, _ := io.ReadAll(resp.Body)
				sb := string(b)
				outcome.Body = &sb
			}
			_ = resp.Body.Close()
		}
	}
	return outcome
}

func handleHealth(c *gin.Context, s *atomic.Int32) {
	st := int(s.Load())
	c.PureJSON(st, api.Health{Status: st})
}

func degradeHealth(c *gin.Context, status *atomic.Int32) {
	req := api.Health{}
	err := c.Bind(&req)
	if err != nil {
		c.PureJSON(http.StatusBadRequest, api.BadRequestResponse(err))
		return
	} else {
		st := req.Status
		if st == 0 {
			st = http.StatusOK
		}
		status.Store(int32(st))
		c.PureJSON(http.StatusOK, api.Health{Status: st})
	}
}

func NewServerImpl(l *slog.Logger, seed int64) api.ServerInterface {
	s := &srv{
		l:            l.WithGroup("api-server"),
		healthStatus: atomic.Int32{},
		readyStatus:  atomic.Int32{},
		apis:         atomic.Pointer[map[string]api.ConfigureAPI]{},
		rand:         rand.New(rand.NewSource(seed)),
	}
	s.healthStatus.Store(http.StatusOK)
	s.readyStatus.Store(http.StatusOK)
	s.apis.Store(&map[string]api.ConfigureAPI{})
	return s
}
