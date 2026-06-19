package clickhousex

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type metricEvent struct {
	name   string
	value  float64
	labels map[string]string
}

type recordingMetrics struct {
	counters   []metricEvent
	histograms []metricEvent
	gauges     []metricEvent
}

func (m *recordingMetrics) IncCounter(name string, labels map[string]string) {
	m.counters = append(m.counters, metricEvent{name: name, labels: cloneLabels(labels)})
}

func (m *recordingMetrics) ObserveHistogram(name string, value float64, labels map[string]string) {
	m.histograms = append(m.histograms, metricEvent{name: name, value: value, labels: cloneLabels(labels)})
}

func (m *recordingMetrics) SetGauge(name string, value float64, labels map[string]string) {
	m.gauges = append(m.gauges, metricEvent{name: name, value: value, labels: cloneLabels(labels)})
}

func cloneLabels(labels map[string]string) map[string]string {
	out := make(map[string]string, len(labels))
	for key, value := range labels {
		out[key] = value
	}
	return out
}

type deadlineOnlyContext struct {
	deadline time.Time
}

func (ctx deadlineOnlyContext) Deadline() (time.Time, bool) {
	return ctx.deadline, true
}

func (ctx deadlineOnlyContext) Done() <-chan struct{} {
	return nil
}

func (ctx deadlineOnlyContext) Err() error {
	return nil
}

func (ctx deadlineOnlyContext) Value(key any) any {
	return nil
}

type delayedCanceledDeadlineContext struct {
	context.Context
	errCalls int
}

func (ctx delayedCanceledDeadlineContext) Deadline() (time.Time, bool) {
	return time.Now().Add(-time.Millisecond), true
}

func (ctx *delayedCanceledDeadlineContext) Err() error {
	ctx.errCalls++
	if ctx.errCalls > 1 {
		return context.Canceled
	}
	return nil
}

func validConfig() Config {
	return Config{
		Name:            "primary",
		Host:            "localhost",
		Port:            DefaultPort,
		Database:        "default",
		Username:        "default",
		Password:        "secret",
		MaxOpenConns:    DefaultMaxOpenConns,
		MaxIdleConns:    DefaultMaxIdleConns,
		ConnMaxLifetime: time.Minute,
		Timeout:         DefaultTimeout,
	}
}

func nilContext() context.Context {
	return nil
}

func assertKind(t *testing.T, err error, kind ErrorKind) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s error, got nil", kind)
	}
	if !IsKind(err, kind) {
		t.Fatalf("expected %s error, got %T: %v", kind, err, err)
	}
}

func assertCounter(t *testing.T, metrics *recordingMetrics, name string) {
	t.Helper()
	for _, event := range metrics.counters {
		if event.name == name {
			return
		}
	}
	t.Fatalf("expected counter %q in %#v", name, metrics.counters)
}

func TestConfigValidate(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("valid config should pass: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "missing name", mutate: func(cfg *Config) { cfg.Name = "" }},
		{name: "missing host", mutate: func(cfg *Config) { cfg.Host = "" }},
		{name: "negative port", mutate: func(cfg *Config) { cfg.Port = -1 }},
		{name: "negative max open", mutate: func(cfg *Config) { cfg.MaxOpenConns = -1 }},
		{name: "negative max idle", mutate: func(cfg *Config) { cfg.MaxIdleConns = -1 }},
		{name: "negative lifetime", mutate: func(cfg *Config) { cfg.ConnMaxLifetime = -1 }},
		{name: "negative timeout", mutate: func(cfg *Config) { cfg.Timeout = -1 }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			tc.mutate(&cfg)
			assertKind(t, cfg.Validate(), ErrorKindValidation)
		})
	}
}

func TestConfigSanitize(t *testing.T) {
	cfg := validConfig()
	cfg.Password = "super-secret"
	safe := cfg.Sanitize()

	if safe.Password != "***" {
		t.Fatalf("expected masked password, got %q", safe.Password)
	}
	if safe.Name != cfg.Name || safe.Host != cfg.Host || safe.Port != cfg.Port {
		t.Fatalf("sanitized config changed non-secret fields: %#v", safe)
	}
}

func TestNew(t *testing.T) {
	metrics := &recordingMetrics{}
	client, err := New(context.Background(), validConfig(), WithMetrics(metrics), WithMetrics(nil))
	if err != nil {
		t.Fatalf("new client failed: %v", err)
	}
	if client == nil || !client.initialized {
		t.Fatalf("expected initialized client, got %#v", client)
	}
	assertCounter(t, metrics, MetricClientCreatedTotal)

	_, err = New(nilContext(), validConfig(), WithMetrics(metrics))
	assertKind(t, err, ErrorKindValidation)

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = New(canceled, validConfig(), WithMetrics(metrics))
	assertKind(t, err, ErrorKindUnavailable)

	cfg := validConfig()
	cfg.Name = ""
	_, err = New(context.Background(), cfg, WithMetrics(metrics))
	assertKind(t, err, ErrorKindValidation)
	assertCounter(t, metrics, MetricClientErrorsTotal)
}

func TestClose(t *testing.T) {
	var nilClient *Client
	assertKind(t, nilClient.Close(context.Background()), ErrorKindValidation)

	metrics := &recordingMetrics{}
	client, err := New(context.Background(), validConfig(), WithMetrics(metrics))
	if err != nil {
		t.Fatalf("new client failed: %v", err)
	}
	assertKind(t, client.Close(nilContext()), ErrorKindValidation)

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	assertKind(t, client.Close(canceled), ErrorKindUnavailable)

	uninitialized := &Client{metrics: metrics}
	assertKind(t, uninitialized.Close(context.Background()), ErrorKindValidation)

	if err := client.Close(context.Background()); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if err := client.Close(context.Background()); err != nil {
		t.Fatalf("idempotent close failed: %v", err)
	}
	assertCounter(t, metrics, MetricClientClosedTotal)
}

func TestPing(t *testing.T) {
	var nilClient *Client
	assertKind(t, nilClient.Ping(context.Background()), ErrorKindValidation)

	metrics := &recordingMetrics{}
	client, err := New(context.Background(), validConfig(), WithMetrics(metrics))
	if err != nil {
		t.Fatalf("new client failed: %v", err)
	}
	assertKind(t, client.Ping(nilContext()), ErrorKindValidation)

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	assertKind(t, client.Ping(canceled), ErrorKindUnavailable)

	uninitialized := &Client{metrics: metrics}
	assertKind(t, uninitialized.Ping(context.Background()), ErrorKindValidation)

	if err := client.Close(context.Background()); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	assertKind(t, client.Ping(context.Background()), ErrorKindUnavailable)

	active, err := New(context.Background(), validConfig(), WithMetrics(metrics))
	if err != nil {
		t.Fatalf("new active client failed: %v", err)
	}
	if err := active.Ping(context.Background()); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
	assertCounter(t, metrics, MetricClientErrorsTotal)
}

func TestHealthCheck(t *testing.T) {
	if status := (*Client)(nil).HealthCheck(context.Background()); status.Status != HealthUnhealthy || status.Name != "clickhousex" {
		t.Fatalf("unexpected nil-client status: %#v", status)
	}

	metrics := &recordingMetrics{}
	cfg := validConfig()
	cfg.Timeout = time.Second
	client, err := New(context.Background(), cfg, WithMetrics(metrics))
	if err != nil {
		t.Fatalf("new client failed: %v", err)
	}

	if status := client.HealthCheck(nilContext()); status.Status != HealthUnhealthy || status.Message != "context is required" {
		t.Fatalf("unexpected nil-context status: %#v", status)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if status := client.HealthCheck(canceled); status.Status != HealthUnhealthy || status.Message == "" {
		t.Fatalf("unexpected canceled status: %#v", status)
	}

	unnamed := &Client{metrics: metrics}
	if status := unnamed.HealthCheck(context.Background()); status.Status != HealthUnhealthy || status.Name != "clickhousex" {
		t.Fatalf("unexpected uninitialized status: %#v", status)
	}

	closed := &Client{cfg: cfg, metrics: metrics, initialized: true, closed: true}
	if status := closed.HealthCheck(context.Background()); status.Status != HealthUnhealthy || status.Message != "client is closed" {
		t.Fatalf("unexpected closed status: %#v", status)
	}

	expired := deadlineOnlyContext{deadline: time.Now().Add(-time.Second)}
	if status := client.HealthCheck(expired); status.Status != HealthUnhealthy || status.Message != context.DeadlineExceeded.Error() {
		t.Fatalf("unexpected expired-deadline status: %#v", status)
	}

	delayedCanceled := &delayedCanceledDeadlineContext{Context: context.Background()}
	if status := client.HealthCheck(delayedCanceled); status.Status != HealthUnhealthy || status.Message != context.Canceled.Error() {
		t.Fatalf("unexpected delayed-canceled status: %#v", status)
	}

	short := deadlineOnlyContext{deadline: time.Now().Add(time.Millisecond)}
	if status := client.HealthCheck(short); status.Status != HealthDegraded || status.Metadata["reason"] != "deadline_below_timeout" {
		t.Fatalf("unexpected degraded status: %#v", status)
	}

	if status := client.HealthCheck(context.Background()); status.Status != HealthHealthy || status.Message != "ok" {
		t.Fatalf("unexpected healthy status: %#v", status)
	}
	if len(metrics.gauges) == 0 || len(metrics.histograms) == 0 {
		t.Fatalf("expected health metrics, got gauges=%#v histograms=%#v", metrics.gauges, metrics.histograms)
	}
}

func TestErrors(t *testing.T) {
	var nilError *Error
	if nilError.Error() != "" {
		t.Fatalf("nil error should render empty string")
	}
	if nilError.Unwrap() != nil {
		t.Fatalf("nil error should unwrap to nil")
	}

	err := NewError(ErrorKindConfig, "Config.Validate", "host is required", false)
	if got := err.Error(); got != "clickhousex: Config.Validate: host is required" {
		t.Fatalf("unexpected error string: %q", got)
	}
	if !IsKind(err, ErrorKindConfig) || IsKind(err, ErrorKindQuery) {
		t.Fatalf("unexpected kind matching result")
	}

	cause := errors.New("dial refused")
	wrapped := WrapError(ErrorKindConnection, "connect", "", true, cause)
	if got := wrapped.Error(); got != "clickhousex: connect: dial refused" {
		t.Fatalf("unexpected wrapped error string: %q", got)
	}
	if !errors.Is(wrapped, cause) {
		t.Fatalf("expected wrapped cause")
	}

	causeOnly := &Error{Kind: ErrorKindConnection, Cause: cause}
	if got := causeOnly.Error(); got != "clickhousex: dial refused" {
		t.Fatalf("unexpected cause-only error string: %q", got)
	}

	detailOnly := NewError(ErrorKindInternal, "", "internal detail", false)
	if got := detailOnly.Error(); got != "clickhousex: internal detail" {
		t.Fatalf("unexpected detail-only error string: %q", got)
	}

	kindOnly := NewError(ErrorKindBatch, "", "", false)
	if got := kindOnly.Error(); got != "clickhousex: batch" {
		t.Fatalf("unexpected kind-only error string: %q", got)
	}

	opOnly := NewError(ErrorKindConnection, "connect", "", true)
	if got := opOnly.Error(); got != "clickhousex: connect" {
		t.Fatalf("unexpected op-only error string: %q", got)
	}

	if IsKind(errors.New("plain"), ErrorKindInternal) {
		t.Fatalf("plain error should not match structured kind")
	}
	if kind := errorKind(errors.New("plain")); kind != ErrorKindInternal {
		t.Fatalf("unexpected plain error kind: %s", kind)
	}

	timeoutErr := contextError("query", context.DeadlineExceeded)
	if !timeoutErr.Retryable || !IsKind(timeoutErr, ErrorKindTimeout) {
		t.Fatalf("unexpected timeout error: %#v", timeoutErr)
	}
	unavailableErr := contextError("query", context.Canceled)
	if unavailableErr.Retryable || !IsKind(unavailableErr, ErrorKindUnavailable) {
		t.Fatalf("unexpected unavailable error: %#v", unavailableErr)
	}

	validationErr := validationError("validate", "bad config", cause)
	if validationErr.Unwrap() != cause || !strings.Contains(validationErr.Error(), "bad config") {
		t.Fatalf("unexpected validation error: %#v", validationErr)
	}

	metrics := &recordingMetrics{}
	recordErrorMetric(nil, "query", err)
	recordErrorMetric(metrics, "query", errors.New("plain"))
	assertCounter(t, metrics, MetricClientErrorsTotal)
}

func TestMetricsAndVersion(t *testing.T) {
	NoopMetrics{}.IncCounter(MetricClientRequestsTotal, nil)
	NoopMetrics{}.ObserveHistogram(MetricClientRequestDurationSeconds, 1, nil)
	NoopMetrics{}.SetGauge(MetricClientInflight, 1, nil)

	if ModuleName != "github.com/ZoneCNH/clickhousex" {
		t.Fatalf("unexpected module name: %q", ModuleName)
	}
	if Version != "v1.0.2" {
		t.Fatalf("unexpected version: %q", Version)
	}
}
