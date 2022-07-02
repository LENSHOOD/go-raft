package comm

import (
	"github.com/opentracing/opentracing-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	jaegerlog "github.com/uber/jaeger-client-go/log"
	"github.com/uber/jaeger-lib/metrics"
	"io"
)

func InitGlobalTracer(tracerName string) io.Closer {
	cfg, err := jaegercfg.FromEnv()
	if err != nil {
		GetLogger().Fatalf("Fetch jaeger env failed: %v", err)
	}

	cfg.ServiceName = tracerName
	jLogger := jaegerlog.StdLogger
	jMetricsFactory := metrics.NullFactory

	tracer, closer, err := cfg.NewTracer(
		jaegercfg.Logger(jLogger),
		jaegercfg.Metrics(jMetricsFactory),
	)

	if err != nil {
		_ = closer.Close()
		GetLogger().Fatalf("Init tracer failed: %v", err)
	}
	// Set the singleton opentracing.Tracer with the Jaeger tracer.
	opentracing.SetGlobalTracer(tracer)

	return closer
}
