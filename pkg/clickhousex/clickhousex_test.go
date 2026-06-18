package clickhousex

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type counterCall struct {
	name   string
	labels map[string]string
}

type histogramCall struct {
	name   string
	value  float64
	labels map[string]string
}

type gaugeCall struct {
	name   string
	value  float64
	labels map[string]string
}

type expiredDeadlineContext struct {
	context.Context
}

func (expiredDeadlineContext) Deadline() (time.Time, bool) {
	return time.Now().Add(-time.Millisecond), true
}

func (expiredDeadlineContext) Err() error {
	return nil
}

type delayedCanceledDeadlineContext struct {
	context.Context
	errCalls int
}

func (delayedCanceledDeadlineContext) Deadline() (time.Time, bool) {
	return time.Now().Add(-time.Millisecond), true
}

func (c *delayedCanceledDeadlineContext) Err() error {
	c.errCalls++
	if c.errCalls > 1 {
		return context.Canceled
	}
	return nil
}

type recordingMetrics struct {
	counters   []counterCall
	histograms []histogramCall
	gauges     []gaugeCall
}

func (m *recordingMetrics) IncCounter(name string, labels map[string]string) {
	m.counters = append(m.counters, counterCall{name: name, labels: cloneLabels(labels)})
}

func (m *recordingMetrics) ObserveHistogram(name string, value float64, labels map[string]string) {
	m.histograms = append(m.histograms, histogramCall{name: name, value: value, labels: cloneLabels(labels)})
}

func (m *recordingMetrics) SetGauge(name string, value float64, labels map[string]string) {
	m.gauges = append(m.gauges, gaugeCall{name: name, value: value, labels: cloneLabels(labels)})
}

func cloneLabels(labels map[string]string) map[string]string {
	if labels == nil {
		return nil
	}
	copy := make(map[string]string, len(labels))
	for key, value := range labels {
		copy[key] = value
	}
	return copy
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

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	if err := validConfig().Validate(); err != nil {
		t.Fatalf("Validate(valid) returned error: %v", err)
	}

	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{name: "missing name", cfg: func() Config { cfg := validConfig(); cfg.Name = ""; return cfg }(), want: "validation: Config.Validate: name is required"},
		{name: "missing host", cfg: func() Config { cfg := validConfig(); cfg.Host = ""; return cfg }(), want: "validation: Config.Validate: host is required"},
		{name: "negative port", cfg: func() Config { cfg := validConfig(); cfg.Port = -1; return cfg }(), want: "validation: Config.Validate: port must not be negative"},
		{name: "negative max open", cfg: func() Config { cfg := validConfig(); cfg.MaxOpenConns = -1; return cfg }(), want: "validation: Config.Validate: max_open_conns must not be negative"},
		{name: "negative max idle", cfg: func() Config { cfg := validConfig(); cfg.MaxIdleConns = -1; return cfg }(), want: "validation: Config.Validate: max_idle_conns must not be negative"},
		{name: "negative lifetime", cfg: func() Config { cfg := validConfig(); cfg.ConnMaxLifetime = -1; return cfg }(), want: "validation: Config.Validate: conn_max_lifetime must not be negative"},
		{name: "negative timeout", cfg: func() Config { cfg := validConfig(); cfg.Timeout = -1; return cfg }(), want: "validation: Config.Validate: timeout must not be negative"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if err == nil {
				t.Fatal("Validate returned nil, want error")
			}
			if got := err.Error(); got != tt.want {
				t.Fatalf("Validate error = %q, want %q", got, tt.want)
			}
			if !IsKind(err, ErrorKindValidation) {
				t.Fatalf("Validate error kind mismatch: %v", err)
			}
		})
	}
}

func TestConfigSanitize(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	got := cfg.Sanitize()
	want := SanitizedConfig{
		Name:            cfg.Name,
		Host:            cfg.Host,
		Port:            cfg.Port,
		Database:        cfg.Database,
		Username:        cfg.Username,
		Password:        "***",
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime,
		Timeout:         cfg.Timeout,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Sanitize() = %#v, want %#v", got, want)
	}

	cfg.Password = ""
	if got := cfg.Sanitize().Password; got != "" {
		t.Fatalf("Sanitize(empty password) = %q, want empty", got)
	}
}

func TestNew(t *testing.T) {
	t.Parallel()

	metrics := &recordingMetrics{}
	client, err := New(context.Background(), validConfig(), WithMetrics(metrics), WithMetrics(nil))
	if err != nil {
		t.Fatalf("New(valid) returned error: %v", err)
	}
	if client == nil || !client.initialized || client.closed {
		t.Fatalf("New(valid) client state = %#v", client)
	}
	assertCounter(t, metrics, MetricClientCreatedTotal, map[string]string{"name": "primary"})

	metrics = &recordingMetrics{}
	client, err = New(nil, validConfig(), WithMetrics(metrics))
	if client != nil || err == nil || err.Error() != "validation: clickhousex.New: context is required" {
		t.Fatalf("New(nil ctx) client=%#v err=%v", client, err)
	}
	assertErrorCounter(t, metrics, "new", ErrorKindValidation)

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	metrics = &recordingMetrics{}
	client, err = New(canceled, validConfig(), WithMetrics(metrics))
	if client != nil || err == nil || !errors.Is(err, context.Canceled) || !IsKind(err, ErrorKindUnavailable) {
		t.Fatalf("New(canceled) client=%#v err=%v", client, err)
	}
	assertErrorCounter(t, metrics, "new", ErrorKindUnavailable)

	metrics = &recordingMetrics{}
	bad := validConfig()
	bad.Name = ""
	client, err = New(context.Background(), bad, WithMetrics(metrics))
	if client != nil || err == nil || !IsKind(err, ErrorKindValidation) {
		t.Fatalf("New(invalid cfg) client=%#v err=%v", client, err)
	}
	assertErrorCounter(t, metrics, "new", ErrorKindValidation)
}

func TestClose(t *testing.T) {
	t.Parallel()

	if err := (*Client)(nil).Close(context.Background()); err == nil || err.Error() != "validation: clickhousex.Close: client is nil" {
		t.Fatalf("Close(nil client) = %v", err)
	}

	client, metrics := newTestClient(t)
	if err := client.Close(nil); err == nil || err.Error() != "validation: clickhousex.Close: context is required" {
		t.Fatalf("Close(nil ctx) = %v", err)
	}
	assertErrorCounter(t, metrics, "close", ErrorKindValidation)

	client, metrics = newTestClient(t)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := client.Close(canceled); err == nil || !errors.Is(err, context.Canceled) || !IsKind(err, ErrorKindUnavailable) {
		t.Fatalf("Close(canceled) = %v", err)
	}
	assertErrorCounter(t, metrics, "close", ErrorKindUnavailable)

	client = &Client{metrics: metrics}
	if err := client.Close(context.Background()); err == nil || err.Error() != "validation: clickhousex.Close: client is not initialized" {
		t.Fatalf("Close(uninitialized) = %v", err)
	}

	client, metrics = newTestClient(t)
	if err := client.Close(context.Background()); err != nil {
		t.Fatalf("Close(valid) returned error: %v", err)
	}
	if !client.closed {
		t.Fatal("Close(valid) did not mark client closed")
	}
	assertCounter(t, metrics, MetricClientClosedTotal, map[string]string{"name": "primary"})
	before := len(metrics.counters)
	if err := client.Close(context.Background()); err != nil {
		t.Fatalf("Close(already closed) returned error: %v", err)
	}
	if len(metrics.counters) != before {
		t.Fatalf("Close(already closed) recorded counter; counters=%#v", metrics.counters)
	}
}

func TestPing(t *testing.T) {
	t.Parallel()

	if err := (*Client)(nil).Ping(context.Background()); err == nil || err.Error() != "validation: clickhousex.Ping: client is nil" {
		t.Fatalf("Ping(nil client) = %v", err)
	}

	client, metrics := newTestClient(t)
	if err := client.Ping(nil); err == nil || err.Error() != "validation: clickhousex.Ping: context is required" {
		t.Fatalf("Ping(nil ctx) = %v", err)
	}
	assertErrorCounter(t, metrics, "ping", ErrorKindValidation)

	client, metrics = newTestClient(t)
	deadline, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Millisecond))
	defer cancel()
	if err := client.Ping(deadline); err == nil || !errors.Is(err, context.DeadlineExceeded) || !IsKind(err, ErrorKindTimeout) {
		t.Fatalf("Ping(expired deadline) = %v", err)
	}
	assertErrorCounter(t, metrics, "ping", ErrorKindTimeout)

	client = &Client{metrics: metrics}
	if err := client.Ping(context.Background()); err == nil || err.Error() != "validation: clickhousex.Ping: client is not initialized" {
		t.Fatalf("Ping(uninitialized) = %v", err)
	}

	client, metrics = newTestClient(t)
	client.closed = true
	if err := client.Ping(context.Background()); err == nil || err.Error() != "unavailable: clickhousex.Ping: client is closed" || !IsKind(err, ErrorKindUnavailable) {
		t.Fatalf("Ping(closed) = %v", err)
	}
	assertErrorCounter(t, metrics, "ping", ErrorKindUnavailable)

	client, _ = newTestClient(t)
	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("Ping(valid) returned error: %v", err)
	}
}

func TestHealthCheck(t *testing.T) {
	t.Parallel()

	status := (*Client)(nil).HealthCheck(context.Background())
	assertHealth(t, status, "clickhousex", HealthUnhealthy, "client is not initialized")

	status = (&Client{}).HealthCheck(nil)
	assertHealth(t, status, "clickhousex", HealthUnhealthy, "context is required")

	client, metrics := newTestClient(t)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	status = client.HealthCheck(canceled)
	assertHealth(t, status, "primary", HealthUnhealthy, context.Canceled.Error())
	assertHealthMetrics(t, metrics, "primary", HealthUnhealthy, 0)

	client, metrics = newTestClient(t)
	client.initialized = false
	status = client.HealthCheck(context.Background())
	assertHealth(t, status, "primary", HealthUnhealthy, "client is not initialized")
	assertHealthMetrics(t, metrics, "primary", HealthUnhealthy, 0)

	client, metrics = newTestClient(t)
	client.closed = true
	status = client.HealthCheck(context.Background())
	assertHealth(t, status, "primary", HealthUnhealthy, "client is closed")
	assertHealthMetrics(t, metrics, "primary", HealthUnhealthy, 0)

	client, metrics = newTestClient(t)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Millisecond))
	defer cancel()
	status = client.HealthCheck(ctx)
	assertHealth(t, status, "primary", HealthDegraded, "context deadline is shorter than client timeout")
	if got := status.Metadata["reason"]; got != "deadline_below_timeout" {
		t.Fatalf("HealthCheck degraded reason = %q", got)
	}
	assertHealthMetrics(t, metrics, "primary", HealthDegraded, 0)

	client, metrics = newTestClient(t)
	status = client.HealthCheck(expiredDeadlineContext{Context: context.Background()})
	assertHealth(t, status, "primary", HealthUnhealthy, context.DeadlineExceeded.Error())
	assertHealthMetrics(t, metrics, "primary", HealthUnhealthy, 0)

	client, metrics = newTestClient(t)
	status = client.HealthCheck(&delayedCanceledDeadlineContext{Context: context.Background()})
	assertHealth(t, status, "primary", HealthUnhealthy, context.Canceled.Error())
	assertHealthMetrics(t, metrics, "primary", HealthUnhealthy, 0)

	client, metrics = newTestClient(t)
	client.cfg.Name = ""
	client.cfg.Timeout = 0
	status = client.HealthCheck(context.Background())
	assertHealth(t, status, "clickhousex", HealthHealthy, "ok")
	assertHealthMetrics(t, metrics, "clickhousex", HealthHealthy, 1)

	client, metrics = newTestClient(t)
	status = client.HealthCheck(context.Background())
	assertHealth(t, status, "primary", HealthHealthy, "ok")
	assertHealthMetrics(t, metrics, "primary", HealthHealthy, 1)
}

func TestErrors(t *testing.T) {
	t.Parallel()

	if got := (*Error)(nil).Error(); got != "" {
		t.Fatalf("nil Error() = %q, want empty", got)
	}
	if got := (*Error)(nil).Unwrap(); got != nil {
		t.Fatalf("nil Unwrap() = %v, want nil", got)
	}

	base := errors.New("root")
	err := NewError(ErrorKindAuth, "login", "denied", false)
	if err.Error() != "auth: login: denied" {
		t.Fatalf("NewError().Error() = %q", err.Error())
	}
	if err.Unwrap() != nil {
		t.Fatalf("NewError().Unwrap() = %v, want nil", err.Unwrap())
	}

	manual := &Error{Kind: ErrorKindInternal, Op: "manual", Cause: base}
	if manual.Error() != "internal: manual: root" {
		t.Fatalf("manual Error().Error() = %q", manual.Error())
	}

	wrapped := WrapError(ErrorKindConnection, "dial", "", true, base)
	if wrapped.Error() != "connection: dial: root" {
		t.Fatalf("WrapError().Error() = %q", wrapped.Error())
	}
	if !errors.Is(wrapped, base) {
		t.Fatalf("WrapError did not wrap base error")
	}
	if !IsKind(wrapped, ErrorKindConnection) || IsKind(base, ErrorKindConnection) {
		t.Fatalf("IsKind mismatch")
	}
}

func TestMetricsAndOptions(t *testing.T) {
	t.Parallel()

	NoopMetrics{}.IncCounter(MetricClientRequestsTotal, nil)
	NoopMetrics{}.ObserveHistogram(MetricClientRequestDurationSeconds, 1, nil)
	NoopMetrics{}.SetGauge(MetricClientInflight, 1, nil)

	defaults := defaultOptions()
	if _, ok := defaults.metrics.(NoopMetrics); !ok {
		t.Fatalf("defaultOptions metrics = %T, want NoopMetrics", defaults.metrics)
	}

	metrics := &recordingMetrics{}
	opts := defaultOptions()
	WithMetrics(metrics)(&opts)
	if opts.metrics != metrics {
		t.Fatalf("WithMetrics(non-nil) did not set metrics")
	}
	WithMetrics(nil)(&opts)
	if opts.metrics != metrics {
		t.Fatalf("WithMetrics(nil) changed metrics")
	}

	recordErrorMetric(nil, "op", errors.New("plain"))
	recordErrorMetric(metrics, "op", errors.New("plain"))
	assertErrorCounter(t, metrics, "op", ErrorKindInternal)

	if healthGaugeValue(HealthDegraded) != 0 || healthGaugeValue(HealthUnhealthy) != 0 || healthGaugeValue(HealthHealthy) != 1 {
		t.Fatalf("healthGaugeValue returned unexpected values")
	}
}

func newTestClient(t *testing.T) (*Client, *recordingMetrics) {
	t.Helper()
	metrics := &recordingMetrics{}
	client, err := New(context.Background(), validConfig(), WithMetrics(metrics))
	if err != nil {
		t.Fatalf("New test client: %v", err)
	}
	return client, metrics
}

func assertCounter(t *testing.T, metrics *recordingMetrics, name string, labels map[string]string) {
	t.Helper()
	for _, call := range metrics.counters {
		if call.name == name && reflect.DeepEqual(call.labels, labels) {
			return
		}
	}
	t.Fatalf("counter %s labels %#v not found in %#v", name, labels, metrics.counters)
}

func assertErrorCounter(t *testing.T, metrics *recordingMetrics, op string, kind ErrorKind) {
	t.Helper()
	assertCounter(t, metrics, MetricClientErrorsTotal, map[string]string{"op": op, "kind": string(kind)})
}

func assertHealth(t *testing.T, status HealthStatus, name string, value HealthStatusValue, message string) {
	t.Helper()
	if status.Name != name || status.Status != value || status.Message != message {
		t.Fatalf("HealthCheck() = %#v, want name=%q status=%q message=%q", status, name, value, message)
	}
	if status.CheckedAt.IsZero() {
		t.Fatalf("HealthCheck() CheckedAt is zero")
	}
	if status.LatencyMs < 0 {
		t.Fatalf("HealthCheck() LatencyMs = %d, want non-negative", status.LatencyMs)
	}
}

func assertHealthMetrics(t *testing.T, metrics *recordingMetrics, name string, status HealthStatusValue, gaugeValue float64) {
	t.Helper()
	labels := map[string]string{"name": name, "status": string(status)}
	for _, call := range metrics.gauges {
		if call.name == MetricClientHealthStatus && call.value == gaugeValue && reflect.DeepEqual(call.labels, labels) {
			for _, histogram := range metrics.histograms {
				if histogram.name == MetricClientHealthLatencyMS && reflect.DeepEqual(histogram.labels, labels) {
					return
				}
			}
		}
	}
	t.Fatalf("health metrics labels %#v gauge %.1f not found; gauges=%#v histograms=%#v", labels, gaugeValue, metrics.gauges, metrics.histograms)
}
