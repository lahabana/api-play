package observability

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/lahabana/api-play/internal/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
	"os"
	"time"
)

type Observability interface {
	Middleware() gin.HandlerFunc
	Logger() *slog.Logger
}

type obs struct {
	service string
	tp      *sdktrace.TracerProvider
	l       *slog.Logger
}

func (o *obs) Logger() *slog.Logger {
	return o.l
}

func Init(ctx context.Context, service string) (Observability, error) {
	logLevel := &slog.LevelVar{}
	l := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     logLevel,
	}))
	logLevel.Set(slog.LevelDebug)
	host, _ := os.Hostname()
	res, err := resource.New(context.Background(),
		resource.WithAttributes(semconv.ServiceNameKey.String(service)),
		resource.WithAttributes(semconv.HostName(host)),
		resource.WithSchemaURL(semconv.SchemaURL),
	)
	if err != nil {
		return nil, err
	}
	tracingProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
	)
	metricsExporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}
	metricProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(metricsExporter),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(metricProvider)
	otel.SetTracerProvider(tracingProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	o := &obs{tp: tracingProvider, l: l, service: service}

	go func() {
		<-ctx.Done()
		l.InfoContext(ctx, "Shutting down observability")
		if err := tracingProvider.Shutdown(context.Background()); err != nil {
			l.ErrorContext(ctx, "Error shutting down tracer provider", "error", err)
		}
		if err := metricProvider.Shutdown(context.Background()); err != nil {
			l.ErrorContext(ctx, "Error shutting down metric provider", "error", err)

		}
	}()
	return o, nil
}

func key(k interface{}) string {
	return fmt.Sprintf("gin.server.%s", k)
}

func attrsToArgs(in []attribute.KeyValue) []interface{} {
	var res []interface{}
	for _, entry := range in {
		res = append(res, string(entry.Key), entry.Value.AsInterface())
	}
	return res
}

func (o *obs) Middleware() gin.HandlerFunc {
	l := o.l.With("name", "gin-observability")
	meter := otel.Meter("github.com/lahabana/api-play", metric.WithInstrumentationVersion(version.Version))
	requestCount, _ := meter.Int64UpDownCounter(key("http.request.count"), metric.WithDescription("Number of Requests"), metric.WithUnit("Count"))
	totalDuration, _ := meter.Int64Histogram(key("http.request.duration"), metric.WithDescription("Time Taken by request"), metric.WithUnit("Milliseconds"))
	activeRequestsCounter, _ := meter.Int64UpDownCounter(key("http.request.inflight"), metric.WithDescription("Number of requests inflight"), metric.WithUnit("Count"))
	requestSize, _ := meter.Int64Histogram(key(semconv.HTTPRequestBodySizeKey), metric.WithDescription("Request Size"), metric.WithUnit("Bytes"))
	responseSize, _ := meter.Int64Histogram(key(semconv.HTTPResponseBodySizeKey), metric.WithDescription("Response Size"), metric.WithUnit("Bytes"))

	promHandler := promhttp.Handler()
	tracer := otel.GetTracerProvider().Tracer(
		"github.com/lahabana/api-play",
		trace.WithInstrumentationVersion(version.Version),
	)
	propagators := otel.GetTextMapPropagator()
	tracerKey := "gin-observability-trace-key"

	return func(c *gin.Context) {
		if c.Request.URL.Path == "/metrics" {
			promHandler.ServeHTTP(c.Writer, c.Request)
			return
		}
		c.Set(tracerKey, tracer)
		savedCtx := c.Request.Context()
		ctx := propagators.Extract(savedCtx, propagation.HeaderCarrier(c.Request.Header))

		route := c.FullPath()
		if route == "" {
			route = "unknown-route"
		}
		commonAttrs := []attribute.KeyValue{
			semconv.HTTPMethodKey.String(c.Request.Method),
			semconv.HTTPRoute(route),
			semconv.ServiceName(o.service),
		}
		traceAttrs := append([]attribute.KeyValue{
			semconv.URLFull(c.Request.URL.String()),
			semconv.URLPath(c.Request.URL.Path),
			semconv.URLQuery(c.Request.URL.RawQuery),
		}, commonAttrs...)
		opts := []trace.SpanStartOption{
			trace.WithAttributes(traceAttrs...),
			trace.WithSpanKind(trace.SpanKindServer),
		}
		ctx, span := tracer.Start(ctx, route, opts...)
		// Change the context to include the span
		c.Request = c.Request.WithContext(ctx)

		start := time.Now()
		activeRequestsCounter.Add(ctx, 1, metric.WithAttributes(commonAttrs...))

		defer func() {
			latency := time.Since(start)
			c.Writer.Size()
			activeRequestsCounter.Add(ctx, -1, metric.WithAttributes(commonAttrs...))

			metricsAttrs := append([]attribute.KeyValue{
				semconv.HTTPStatusCode(c.Writer.Status()),
			}, commonAttrs...)
			extraAttrs := []attribute.KeyValue{
				semconv.HTTPRequestBodySize(int(c.Request.ContentLength)),
				semconv.HTTPResponseBodySize(c.Writer.Size()),
				semconv.SourceAddress(c.ClientIP()),
				attribute.String("latency", time.Since(start).String()),
			}

			requestCount.Add(ctx, 1, metric.WithAttributes(metricsAttrs...))
			totalDuration.Record(ctx, latency.Milliseconds(), metric.WithAttributes(metricsAttrs...))
			requestSize.Record(ctx, c.Request.ContentLength, metric.WithAttributes(metricsAttrs...))
			responseSize.Record(ctx, int64(c.Writer.Size()), metric.WithAttributes(metricsAttrs...))
			if len(c.Errors) > 0 {
				extraAttrs = append(extraAttrs, attribute.String("gin.errors", c.Errors.String()))
			}

			l.DebugContext(c.Request.Context(), "request handled",
				append(attrsToArgs(traceAttrs), attrsToArgs(extraAttrs)...)...,
			)

			if c.Writer.Status() < 100 || c.Writer.Status() >= 600 {
				span.SetStatus(codes.Unset, fmt.Sprintf("Invalid HTTP status %d", c.Writer.Status()))
			} else if c.Writer.Status() < 300 {
				span.SetStatus(codes.Ok, fmt.Sprintf("HTTP status %d", c.Writer.Status()))
			} else {
				span.SetStatus(codes.Error, fmt.Sprintf("HTTP status: %d", c.Writer.Status()))
			}
			span.SetAttributes(extraAttrs...)
			if len(c.Errors) > 0 {
				span.SetAttributes(attribute.String("gin.errors", c.Errors.String()))
			}
			span.End()
			c.Request = c.Request.WithContext(savedCtx)
		}()

		// Process the request
		c.Next()
	}
}
