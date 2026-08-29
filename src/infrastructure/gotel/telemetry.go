package gotel
 
import (
	"context"
	"fmt"
	"os"
 
	"go.opentelemetry.io/contrib/bridges/otelzap"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)
 
type TelemetryProvider interface {
	GetServiceName() string
	LogInfo(args ...interface{})
	LogErrorln(args ...interface{})
	LogFatalln(args ...interface{})
	MeterInt64Histogram(metric Metric) (otelmetric.Int64Histogram, error)
	MeterInt64UpDownCounter(metric Metric) (otelmetric.Int64UpDownCounter, error)
	TraceStart(ctx context.Context, name string) (context.Context, oteltrace.Span)
	Shutdown(ctx context.Context)
}
 
type Telemetry struct {
	lp     *log.LoggerProvider
	mp     *metric.MeterProvider
	tp     *trace.TracerProvider
	log    *zap.SugaredLogger
	meter  otelmetric.Meter
	tracer oteltrace.Tracer
	cfg    Config
}
 
func NewTelemetry(ctx context.Context, cfg Config) (TelemetryProvider, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
			endpoint = "localhost:4317"
	}

	rp := newResource(cfg.serviceName, cfg.serviceVersion)
 
	lp, err := newLoggerProvider(ctx, rp, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}
 
	logger := zap.New(
		zapcore.NewTee(
			zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(os.Stdout), zapcore.InfoLevel),
			otelzap.NewCore(cfg.serviceName, otelzap.WithLoggerProvider(lp)),
		),
	)
 
	tp, err := newTracerProvider(ctx, rp, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create tracer: %w", err)
	}
	tracer := tp.Tracer(cfg.serviceName)	

	mp, err := newMeterProvider(ctx, rp, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create meter: %w", err)
	}

	meter := mp.Meter(cfg.serviceName)

	return &Telemetry{
		lp:     lp,
		tp:     tp,
		mp:    mp,
		log:    logger.Sugar(),
		meter:  meter,
		tracer: tracer,
		cfg:    cfg,
	}, nil
}
 
func (t *Telemetry) GetServiceName() string {
	return t.cfg.serviceName
}
 
func (t *Telemetry) LogInfo(args ...interface{}) {
	t.log.Info(args...)
}
 
func (t *Telemetry) LogErrorln(args ...interface{}) {
	t.log.Errorln(args...)
}
 
func (t *Telemetry) LogFatalln(args ...interface{}) {
	t.log.Fatalln(args...)
}

func (t *Telemetry) MeterInt64Histogram(metric Metric) (otelmetric.Int64Histogram, error) {
	histogram, err := t.meter.Int64Histogram(
		metric.Name,
		otelmetric.WithDescription(metric.Description),
		otelmetric.WithUnit(metric.Unit),
	)
 
	if err != nil {
		return nil, fmt.Errorf("failed to create histogram: %w", err)
	}
 
	return histogram, nil
}
 
func (t *Telemetry) MeterInt64UpDownCounter(metric Metric) (otelmetric.Int64UpDownCounter, error) {
	counter, err := t.meter.Int64UpDownCounter(
		metric.Name,
		otelmetric.WithDescription(metric.Description),
		otelmetric.WithUnit(metric.Unit),
	)
 
	if err != nil {
		return nil, fmt.Errorf("failed to create counter: %w", err)
	}
 
	return counter, nil
}

func (t *Telemetry) TraceStart(ctx context.Context, name string) (context.Context, oteltrace.Span) {
	return t.tracer.Start(ctx, name)
}

func (t *Telemetry) Shutdown(ctx context.Context) {
	t.lp.Shutdown(ctx)
	t.tp.Shutdown(ctx)
}
