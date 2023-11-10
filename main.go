package main

import (
	"context"
	"flag"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/lahabana/api-play/internal/observability"
	"github.com/lahabana/api-play/internal/reload"
	"github.com/lahabana/api-play/internal/server"
	"github.com/lahabana/api-play/pkg/api"
	api_errors "github.com/lahabana/api-play/pkg/errors"
	"time"
)

//go:generate go run github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@v2.0.0 -config openapi.cfg.yaml openapi.yaml

type Conf struct {
	configFile  string
	seed        int64
	otlpMetrics string
	otlpTraces  string
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf := Conf{}
	flag.StringVar(&conf.configFile, "config-file", "", "A yaml config of the apis")
	flag.Int64Var(&conf.seed, "seed", time.Now().UnixMicro(), "Seed for random generators")
	flag.StringVar(&conf.otlpMetrics, "otlp-metrics", "", "whether or not we should export metrics using otlp (options: http,grpc)")
	flag.StringVar(&conf.otlpTraces, "otlp-traces", "", "whether or not we should export traces using otlp (options: http,grpc)")
	flag.Parse()
	obs, err := observability.Init(ctx, "api-play", observability.OTLPFormat(conf.otlpMetrics), observability.OTLPFormat(conf.otlpTraces))
	if err != nil {
		panic(err)
	}
	serverInstance := server.NewServerImpl(obs.Logger(), time.Now().UnixMicro())
	if reloader, ok := serverInstance.(api.Reloader); ok && conf.configFile != "" {
		reload.BackgroundConfigReload(ctx, obs.Logger().With("name", "config-loader"), conf.configFile, reloader)
	}

	engine := gin.New()
	binding.Validator = &localValidator{delegate: binding.Validator}
	engine.Use(gin.Recovery(), obs.Middleware())
	api.RegisterHandlersWithOptions(engine, serverInstance, api.GinServerOptions{})
	err = engine.Run()
	cancel()
	if err != nil {
		panic(err)
	}
}

type localValidator struct {
	delegate binding.StructValidator
}

func (l localValidator) ValidateStruct(a any) error {
	if withValidator, ok := a.(api_errors.Validate); ok {
		return withValidator.Validate()
	}
	return l.delegate.ValidateStruct(a)
}

func (l localValidator) Engine() any {
	return "localEngine"
}
