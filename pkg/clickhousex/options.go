package clickhousex

import (
	"context"
	"time"
)

// Option configures the Client.
type Option func(*options)

type options struct {
	metrics   Metrics
	tracer    Tracer
	logger    Logger
	connector connector
	retry     RetryConfig
}

// RetryConfig controls retry behavior for retryable connection failures.
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

// Span is the minimal trace span hook used by Client operations.
type Span interface {
	End(error)
}

// Tracer starts operation spans without binding clickhousex to a tracing SDK.
type Tracer interface {
	StartSpan(ctx context.Context, name string, labels map[string]string) (context.Context, Span)
}

// Logger is the minimal structured logging hook used by Client operations.
type Logger interface {
	Debug(ctx context.Context, message string, labels map[string]string)
	Error(ctx context.Context, message string, labels map[string]string)
}

func defaultOptions() options {
	return options{
		metrics:   NoopMetrics{},
		tracer:    noopTracer{},
		logger:    noopLogger{},
		connector: clickhouseConnector{},
		retry: RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   100 * time.Millisecond,
			MaxDelay:    time.Second,
		},
	}
}

// WithMetrics sets the Metrics implementation for the Client.
func WithMetrics(metrics Metrics) Option {
	return func(o *options) {
		if metrics != nil {
			o.metrics = metrics
		}
	}
}

// WithTracer sets the Tracer implementation for the Client.
func WithTracer(tracer Tracer) Option {
	return func(o *options) {
		if tracer != nil {
			o.tracer = tracer
		}
	}
}

// WithLogger sets the Logger implementation for the Client.
func WithLogger(logger Logger) Option {
	return func(o *options) {
		if logger != nil {
			o.logger = logger
		}
	}
}

// WithRetryConfig sets retry behavior for retryable operation failures.
func WithRetryConfig(retry RetryConfig) Option {
	return func(o *options) {
		if retry.MaxAttempts > 0 {
			o.retry.MaxAttempts = retry.MaxAttempts
		}
		if retry.BaseDelay >= 0 {
			o.retry.BaseDelay = retry.BaseDelay
		}
		if retry.MaxDelay >= 0 {
			o.retry.MaxDelay = retry.MaxDelay
		}
	}
}

func withConnector(connector connector) Option {
	return func(o *options) {
		if connector != nil {
			o.connector = connector
		}
	}
}

type noopTracer struct{}

func (noopTracer) StartSpan(ctx context.Context, name string, labels map[string]string) (context.Context, Span) {
	_ = name
	_ = labels
	return ctx, noopSpan{}
}

type noopSpan struct{}

func (noopSpan) End(err error) {
	_ = err
}

type noopLogger struct{}

func (noopLogger) Debug(ctx context.Context, message string, labels map[string]string) {
	_ = ctx
	_ = message
	_ = labels
}

func (noopLogger) Error(ctx context.Context, message string, labels map[string]string) {
	_ = ctx
	_ = message
	_ = labels
}
