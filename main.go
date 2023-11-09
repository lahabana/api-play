package main

import (
	"context"
	"flag"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/lahabana/api-play/internal/reload"
	"github.com/lahabana/api-play/internal/server"
	"github.com/lahabana/api-play/pkg/api"
	api_errors "github.com/lahabana/api-play/pkg/errors"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"log/slog"
	"os"
	"time"
)

//go:generate go run github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@v2.0.0 -config openapi.cfg.yaml openapi.yaml

type Conf struct {
	configFile string
	seed       int64
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf := Conf{}
	flag.StringVar(&conf.configFile, "config-file", "", "A yaml config of the apis")
	flag.Int64Var(&conf.seed, "seed", time.Now().UnixMicro(), "Seed for random generators")
	flag.Parse()
	logLevel := &slog.LevelVar{}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     logLevel,
	}))
	logLevel.Set(slog.LevelDebug)
	tp, err := initTracer()
	if err != nil {
		log.Error("failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Error("Error shutting down tracer provider", "error", err)
		}
	}()
	serverInstance := server.NewServerImpl(log, time.Now().UnixMicro())
	if reloader, ok := serverInstance.(api.Reloader); ok && conf.configFile != "" {
		reload.BackgroundConfigReload(ctx, log.With("name", "config-loader"), conf.configFile, reloader)
	}

	engine := gin.New()
	binding.Validator = &localValidator{delegate: binding.Validator}
	engine.Use(gin.LoggerWithWriter(loggerWrap{logger: log.With("name", "gin")}), gin.Recovery(), otelgin.Middleware("api-play"))
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

type loggerWrap struct {
	logger *slog.Logger
}

func (l loggerWrap) Write(b []byte) (n int, err error) {
	l.logger.Info(string(b))
	return len(b), nil

}

func initTracer() (*sdktrace.TracerProvider, error) {
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return tp, nil
}
