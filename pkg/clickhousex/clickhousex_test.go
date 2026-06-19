package clickhousex

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
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
	clone := make(map[string]string, len(labels))
	for key, value := range labels {
		clone[key] = value
	}
	return clone
}

func nilContext() context.Context {
	return nil
}

type fakeConnector struct {
	conn   driverConn
	err    error
	opened bool
	cfg    Config
}

func (f *fakeConnector) Open(ctx context.Context, cfg Config) (driverConn, error) {
	_ = ctx
	f.opened = true
	f.cfg = cfg
	if f.err != nil {
		return nil, f.err
	}
	if f.conn == nil {
		f.conn = &fakeConn{}
	}
	return f.conn, nil
}

type fakeConn struct {
	execErrs    []error
	queryErrs   []error
	prepareErrs []error
	pingErr     error
	closeErr    error
	rows        driverRows
	batch       *fakeBatch
	stats       driverStats

	execCalls    int
	queryCalls   int
	prepareCalls int
	pingCalls    int
	closeCalls   int

	execQueries    []string
	queryQueries   []string
	prepareQueries []string
}

func (c *fakeConn) Exec(ctx context.Context, query string, args ...any) error {
	_ = ctx
	_ = args
	c.execCalls++
	c.execQueries = append(c.execQueries, query)
	return popErr(&c.execErrs)
}

func (c *fakeConn) Query(ctx context.Context, query string, args ...any) (driverRows, error) {
	_ = ctx
	_ = args
	c.queryCalls++
	c.queryQueries = append(c.queryQueries, query)
	if err := popErr(&c.queryErrs); err != nil {
		return nil, err
	}
	if c.rows == nil {
		c.rows = &fakeRows{}
	}
	return c.rows, nil
}

func (c *fakeConn) PrepareBatch(ctx context.Context, query string) (driverBatch, error) {
	_ = ctx
	c.prepareCalls++
	c.prepareQueries = append(c.prepareQueries, query)
	if err := popErr(&c.prepareErrs); err != nil {
		return nil, err
	}
	if c.batch == nil {
		c.batch = &fakeBatch{}
	}
	return c.batch, nil
}

func (c *fakeConn) Ping(ctx context.Context) error {
	_ = ctx
	c.pingCalls++
	return c.pingErr
}

func (c *fakeConn) Stats() driverStats {
	if c.stats == (driverStats{}) {
		return driverStats{MaxOpenConns: 10, MaxIdleConns: 5, Open: 2, Idle: 1}
	}
	return c.stats
}

func (c *fakeConn) Close() error {
	c.closeCalls++
	return c.closeErr
}

func popErr(errs *[]error) error {
	if len(*errs) == 0 {
		return nil
	}
	err := (*errs)[0]
	*errs = (*errs)[1:]
	return err
}

type fakeRows struct {
	next      []bool
	scanErr   error
	closeErr  error
	err       error
	columns   []ColumnType
	scanFn    func(...any) error
	scanCalls int
}

func (r *fakeRows) Next() bool {
	if len(r.next) == 0 {
		return false
	}
	got := r.next[0]
	r.next = r.next[1:]
	return got
}

func (r *fakeRows) Scan(dest ...any) error {
	r.scanCalls++
	if r.scanFn != nil {
		return r.scanFn(dest...)
	}
	return r.scanErr
}

func (r *fakeRows) Close() error {
	return r.closeErr
}

func (r *fakeRows) Err() error {
	return r.err
}

func (r *fakeRows) ColumnTypes() []ColumnType {
	return append([]ColumnType(nil), r.columns...)
}

type fakeBatch struct {
	appendErrs []error
	sendErr    error
	closeErr   error
	appended   [][]any
	sent       bool
	aborted    bool
}

func (b *fakeBatch) Append(values ...any) error {
	if err := popErr(&b.appendErrs); err != nil {
		return err
	}
	b.appended = append(b.appended, append([]any(nil), values...))
	return nil
}

func (b *fakeBatch) Send() error {
	b.sent = true
	return b.sendErr
}

func (b *fakeBatch) Abort() error {
	b.aborted = true
	return nil
}

func (b *fakeBatch) Close() error {
	return b.closeErr
}

func (b *fakeBatch) Rows() int {
	return len(b.appended)
}

type recordingTracer struct {
	spans []*recordingSpan
}

func (t *recordingTracer) StartSpan(ctx context.Context, name string, labels map[string]string) (context.Context, Span) {
	span := &recordingSpan{name: name, labels: cloneLabels(labels)}
	t.spans = append(t.spans, span)
	return ctx, span
}

type recordingSpan struct {
	name   string
	labels map[string]string
	ended  bool
	err    error
}

func (s *recordingSpan) End(err error) {
	s.ended = true
	s.err = err
}

type recordingLogger struct {
	debugs []string
	errs   []string
}

func (l *recordingLogger) Debug(ctx context.Context, message string, labels map[string]string) {
	_ = ctx
	_ = labels
	l.debugs = append(l.debugs, message)
}

func (l *recordingLogger) Error(ctx context.Context, message string, labels map[string]string) {
	_ = ctx
	_ = labels
	l.errs = append(l.errs, message)
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

func newTestClient(t *testing.T, conn *fakeConn, opts ...Option) (*Client, *recordingMetrics, *recordingTracer, *recordingLogger) {
	t.Helper()
	if conn == nil {
		conn = &fakeConn{}
	}
	metrics := &recordingMetrics{}
	tracer := &recordingTracer{}
	logger := &recordingLogger{}
	options := []Option{
		WithMetrics(metrics),
		WithTracer(tracer),
		WithLogger(logger),
		withConnector(&fakeConnector{conn: conn}),
		WithRetryConfig(RetryConfig{MaxAttempts: 3, BaseDelay: 0, MaxDelay: 0}),
	}
	options = append(options, opts...)
	client, err := New(context.Background(), validConfig(), options...)
	if err != nil {
		t.Fatalf("New(valid) returned error: %v", err)
	}
	return client, metrics, tracer, logger
}

func TestConfigValidateAndSanitize(t *testing.T) {
	t.Parallel()

	if err := validConfig().Validate(); err != nil {
		t.Fatalf("Validate(valid) returned error: %v", err)
	}

	dsnCfg := validConfig()
	dsnCfg.DSN = "clickhouse://default:secret@localhost:9000/default"
	dsnCfg.Host = ""
	if err := dsnCfg.Validate(); err != nil {
		t.Fatalf("Validate(DSN without Host) returned error: %v", err)
	}

	idleTooHigh := validConfig()
	idleTooHigh.MaxOpenConns = 3
	idleTooHigh.MaxIdleConns = 4
	if err := idleTooHigh.Validate(); err == nil || !strings.Contains(err.Error(), "max_idle_conns must not exceed max_open_conns") {
		t.Fatalf("Validate(max idle > max open) = %v", err)
	}

	tooManyOpen := validConfig()
	tooManyOpen.MaxOpenConns = MaxAllowedOpenConns + 1
	tooManyOpen.MaxIdleConns = DefaultMaxIdleConns
	if err := tooManyOpen.Validate(); err == nil || !strings.Contains(err.Error(), "max_open_conns must not exceed 100") {
		t.Fatalf("Validate(max open too high) = %v", err)
	}

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

	cfg.DSN = "clickhouse://default:secret@localhost:9000/default?token=abc"
	sanitized := cfg.Sanitize()
	if strings.Contains(sanitized.DSN, "secret") || strings.Contains(sanitized.DSN, "abc") {
		t.Fatalf("Sanitize(DSN) leaked secret: %q", sanitized.DSN)
	}
}

func TestNewUsesConnectorAndMetrics(t *testing.T) {
	t.Parallel()

	conn := &fakeConn{stats: driverStats{MaxOpenConns: 2, MaxIdleConns: 1, Open: 2, Idle: 0}}
	connector := &fakeConnector{conn: conn}
	metrics := &recordingMetrics{}
	tracer := &recordingTracer{}
	logger := &recordingLogger{}

	client, err := New(
		context.Background(),
		Config{Name: "primary", Host: "localhost"},
		WithMetrics(metrics),
		WithTracer(tracer),
		WithLogger(logger),
		withConnector(connector),
		WithRetryConfig(RetryConfig{MaxAttempts: 1, BaseDelay: 0, MaxDelay: 0}),
	)
	if err != nil {
		t.Fatalf("New(valid) returned error: %v", err)
	}
	if client == nil || !client.initialized || client.closed {
		t.Fatalf("New(valid) client state = %#v", client)
	}
	if !connector.opened {
		t.Fatal("New did not call connector.Open")
	}
	if connector.cfg.Port != DefaultPort || connector.cfg.MaxOpenConns != DefaultMaxOpenConns || connector.cfg.Timeout != DefaultTimeout {
		t.Fatalf("connector received config without defaults: %#v", connector.cfg)
	}
	assertCounter(t, metrics, MetricClientCreatedTotal, map[string]string{"name": "primary"})
	assertGauge(t, metrics, MetricClickhousePoolActive, 2, map[string]string{"name": "primary"})
	assertGauge(t, metrics, MetricClickhousePoolExhausted, 1, map[string]string{"name": "primary"})

	metrics = &recordingMetrics{}
	client, err = New(nilContext(), validConfig(), WithMetrics(metrics), withConnector(connector))
	if client != nil || err == nil || !IsKind(err, ErrorKindValidation) {
		t.Fatalf("New(nil ctx) client=%#v err=%v", client, err)
	}
	assertErrorCounter(t, metrics, "new", ErrorKindValidation)
}

func TestCloseAndPing(t *testing.T) {
	t.Parallel()

	if err := (*Client)(nil).Close(); err == nil || err.Error() != "clickhousex: clickhousex.Close: client is nil" {
		t.Fatalf("Close(nil client) = %v", err)
	}
	if err := (*Client)(nil).CloseContext(context.Background()); err == nil || err.Error() != "clickhousex: clickhousex.Close: client is nil" {
		t.Fatalf("CloseContext(nil client) = %v", err)
	}

	conn := &fakeConn{}
	client, metrics, _, _ := newTestClient(t, conn)
	if err := client.CloseContext(nilContext()); err == nil || err.Error() != "clickhousex: clickhousex.Close: context is required" {
		t.Fatalf("CloseContext(nil ctx) = %v", err)
	}
	assertErrorCounter(t, metrics, "close", ErrorKindValidation)

	if err := (*Client)(nil).Ping(context.Background()); err == nil || err.Error() != "clickhousex: clickhousex.Ping: client is nil" {
		t.Fatalf("Ping(nil client) = %v", err)
	}
	if err := client.Ping(nilContext()); err == nil || err.Error() != "clickhousex: clickhousex.Ping: context is required" {
		t.Fatalf("Ping(nil ctx) = %v", err)
	}
	assertErrorCounter(t, metrics, "ping", ErrorKindValidation)
	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("Ping(valid) returned error: %v", err)
	}
	if conn.pingCalls != 1 {
		t.Fatalf("Ping(valid) calls = %d, want 1", conn.pingCalls)
	}

	if err := client.Close(); err != nil {
		t.Fatalf("Close(valid) returned error: %v", err)
	}
	if !client.closed || conn.closeCalls != 1 {
		t.Fatalf("Close(valid) state closed=%v closeCalls=%d", client.closed, conn.closeCalls)
	}
	assertCounter(t, metrics, MetricClientClosedTotal, map[string]string{"name": "primary"})
	if err := client.Close(); err != nil {
		t.Fatalf("Close(already closed) returned error: %v", err)
	}
	if conn.closeCalls != 1 {
		t.Fatalf("Close(already closed) closeCalls=%d, want 1", conn.closeCalls)
	}
	if err := client.Ping(context.Background()); err == nil || !IsKind(err, ErrorKindUnavailable) {
		t.Fatalf("Ping(closed) = %v", err)
	}
}

func TestExecRetriesRetryableErrors(t *testing.T) {
	t.Parallel()

	conn := &fakeConn{execErrs: []error{errors.New("connection reset by peer"), errors.New("i/o timeout")}}
	client, metrics, tracer, logger := newTestClient(t, conn)

	if err := client.Exec(context.Background(), "SELECT 1"); err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if conn.execCalls != 3 {
		t.Fatalf("Exec calls = %d, want 3", conn.execCalls)
	}
	assertCounter(t, metrics, MetricClientRetriesTotal, map[string]string{"op": "exec", "attempt": "1"})
	assertCounter(t, metrics, MetricClientRetriesTotal, map[string]string{"op": "exec", "attempt": "2"})
	assertHistogram(t, metrics, MetricClickhouseQueryDuration, map[string]string{"op": "exec", "query": "select"})
	if len(tracer.spans) != 1 || tracer.spans[0].name != "clickhousex.exec" || !tracer.spans[0].ended || tracer.spans[0].err != nil {
		t.Fatalf("Exec span = %#v", tracer.spans)
	}
	if len(logger.errs) != 0 || len(logger.debugs) == 0 {
		t.Fatalf("Exec logger debug=%#v errs=%#v", logger.debugs, logger.errs)
	}
}

func TestQueryReturnsRowsAndValidatesScan(t *testing.T) {
	t.Parallel()

	rowText := "ready"
	fake := &fakeRows{
		next: []bool{true},
		columns: []ColumnType{
			{Name: "price", Type: "Decimal(18,2)"},
			{Name: "note", Type: "String", Nullable: true},
		},
		scanFn: func(dest ...any) error {
			*dest[0].(*decimal.Decimal) = decimal.NewFromInt(42)
			*dest[1].(**string) = &rowText
			return nil
		},
	}
	conn := &fakeConn{rows: fake}
	client, _, _, _ := newTestClient(t, conn)

	rows, err := client.Query(context.Background(), "SELECT price, note FROM events")
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if conn.queryCalls != 1 || !rows.Next() {
		t.Fatalf("Query calls=%d rows.Next() failed", conn.queryCalls)
	}
	var price decimal.Decimal
	var note *string
	if err := rows.Scan(&price, &note); err != nil {
		t.Fatalf("Rows.Scan(valid) returned error: %v", err)
	}
	if !price.Equal(decimal.NewFromInt(42)) || note == nil || *note != rowText {
		t.Fatalf("Rows.Scan values price=%s note=%v", price, note)
	}

	types := rows.ColumnTypes()
	types[0].Name = "changed"
	if rows.ColumnTypes()[0].Name == "changed" {
		t.Fatal("ColumnTypes returned mutable internal slice")
	}
	if err := rows.Scan(&price); err == nil || !errors.Is(err, ErrColumnCountMismatch) {
		t.Fatalf("Rows.Scan(count mismatch) = %v", err)
	}
	var wrongDecimal string
	if err := rows.Scan(&wrongDecimal, &note); err == nil || !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("Rows.Scan(decimal type mismatch) = %v", err)
	}
	var noteValue string
	if err := rows.Scan(&price, &noteValue); err == nil || !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("Rows.Scan(nullable mismatch) = %v", err)
	}
}

func TestInsertBatchValidationAndSend(t *testing.T) {
	t.Parallel()

	conn := &fakeConn{batch: &fakeBatch{}}
	client, metrics, _, _ := newTestClient(t, conn)
	ctx := context.Background()

	if err := client.InsertBatch(ctx, "events", []string{"ts"}, nil); err != nil {
		t.Fatalf("InsertBatch(empty rows) returned error: %v", err)
	}
	if conn.prepareCalls != 0 {
		t.Fatalf("InsertBatch(empty rows) prepareCalls=%d, want 0", conn.prepareCalls)
	}
	if err := client.InsertBatch(ctx, "events", nil, [][]any{{1}}); err == nil || !errors.Is(err, ErrEmptyColumns) {
		t.Fatalf("InsertBatch(empty cols) = %v", err)
	}
	if err := client.InsertBatch(ctx, "events", []string{"a", "b"}, [][]any{{1}}); err == nil || !errors.Is(err, ErrColumnCountMismatch) {
		t.Fatalf("InsertBatch(row mismatch) = %v", err)
	}

	inputRows := [][]any{{time.Unix(0, 0), 1}, {time.Unix(1, 0), 2}}
	if err := client.InsertBatch(ctx, "analytics.events", []string{"ts", "value"}, inputRows); err != nil {
		t.Fatalf("InsertBatch(valid) returned error: %v", err)
	}
	wantQuery := "INSERT INTO `analytics`.`events` (`ts`, `value`) VALUES"
	if len(conn.prepareQueries) != 1 || conn.prepareQueries[0] != wantQuery {
		t.Fatalf("PrepareBatch query = %#v, want %q", conn.prepareQueries, wantQuery)
	}
	if len(conn.batch.appended) != len(inputRows) || !conn.batch.sent || conn.batch.aborted {
		t.Fatalf("batch state appended=%#v sent=%v aborted=%v", conn.batch.appended, conn.batch.sent, conn.batch.aborted)
	}
	assertHistogramValue(t, metrics, MetricClickhouseWriteRows, 2, map[string]string{"op": "insert_batch", "table": "analytics.events"})
}

func TestInsertBatchMapsTableNotFound(t *testing.T) {
	t.Parallel()

	conn := &fakeConn{prepareErrs: []error{errors.New("unknown table events")}}
	client, _, _, _ := newTestClient(t, conn)

	err := client.InsertBatch(context.Background(), "events", []string{"id"}, [][]any{{1}})
	if err == nil || !errors.Is(err, ErrTableNotFound) || !IsKind(err, ErrorKindTableNotFound) {
		t.Fatalf("InsertBatch(table not found) = %v", err)
	}
	if conn.prepareCalls != 1 {
		t.Fatalf("PrepareBatch calls = %d, want 1", conn.prepareCalls)
	}
}

func TestHealthUsesPing(t *testing.T) {
	t.Parallel()

	conn := &fakeConn{}
	client, metrics, _, _ := newTestClient(t, conn)

	status := client.HealthCheck(context.Background())
	assertHealth(t, status, HealthHealthy, true, true)
	if conn.pingCalls != 1 {
		t.Fatalf("HealthCheck pingCalls=%d, want 1", conn.pingCalls)
	}
	assertGauge(t, metrics, MetricClientHealthStatus, 1, map[string]string{"status": string(HealthHealthy)})

	conn.pingErr = errors.New("network timeout")
	status = client.HealthCheck(context.Background())
	assertHealth(t, status, HealthUnhealthy, false, true)
	assertGauge(t, metrics, MetricClientHealthStatus, 0, map[string]string{"status": string(HealthUnhealthy)})

	if err := client.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	status = client.HealthCheck(context.Background())
	assertHealth(t, status, HealthUnhealthy, false, false)
}

func TestErrorsMetricsAndOptions(t *testing.T) {
	t.Parallel()

	NoopMetrics{}.IncCounter("x", nil)
	NoopMetrics{}.ObserveHistogram("x", 1, nil)
	NoopMetrics{}.SetGauge("x", 1, nil)
	noopSpan{}.End(nil)
	noopTracer{}.StartSpan(context.Background(), "x", nil)
	noopLogger{}.Debug(context.Background(), "x", nil)
	noopLogger{}.Error(context.Background(), "x", nil)

	options := defaultOptions()
	WithMetrics(nil)(&options)
	WithTracer(nil)(&options)
	WithLogger(nil)(&options)
	withConnector(nil)(&options)
	WithRetryConfig(RetryConfig{MaxAttempts: -1, BaseDelay: -1, MaxDelay: -1})(&options)
	if options.retry.MaxAttempts != 3 || options.retry.BaseDelay != 100*time.Millisecond || options.retry.MaxDelay != time.Second {
		t.Fatalf("nil/negative options changed defaults: %#v", options.retry)
	}

	if !errors.Is(NewError(ErrorKindEmptyColumns, "", "", false), ErrEmptyColumns) {
		t.Fatal("ErrEmptyColumns sentinel did not match by kind")
	}
	if !errors.Is(NewError(ErrorKindColumnCountMismatch, "", "", false), ErrColumnCountMismatch) {
		t.Fatal("ErrColumnCountMismatch sentinel did not match by kind")
	}
	if !errors.Is(NewError(ErrorKindTypeMismatch, "", "", false), ErrTypeMismatch) {
		t.Fatal("ErrTypeMismatch sentinel did not match by kind")
	}

	metrics := &recordingMetrics{}
	recordErrorMetric(metrics, "raw", errors.New("plain"))
	assertErrorCounter(t, metrics, "raw", ErrorKindInternal)
	if healthGaugeValue(HealthHealthy) != 1 || healthGaugeValue(HealthDegraded) != 0 || healthGaugeValue(HealthUnhealthy) != 0 {
		t.Fatalf("healthGaugeValue returned unexpected values")
	}
}

func assertCounter(t *testing.T, metrics *recordingMetrics, name string, labels map[string]string) {
	t.Helper()
	for _, call := range metrics.counters {
		if call.name == name && labelsContain(call.labels, labels) {
			return
		}
	}
	t.Fatalf("counter %q with labels %#v not found in %#v", name, labels, metrics.counters)
}

func assertErrorCounter(t *testing.T, metrics *recordingMetrics, op string, kind ErrorKind) {
	t.Helper()
	assertCounter(t, metrics, MetricClientErrorsTotal, map[string]string{"op": op, "kind": string(kind)})
}

func assertHistogram(t *testing.T, metrics *recordingMetrics, name string, labels map[string]string) {
	t.Helper()
	for _, call := range metrics.histograms {
		if call.name == name && labelsContain(call.labels, labels) {
			return
		}
	}
	t.Fatalf("histogram %q with labels %#v not found in %#v", name, labels, metrics.histograms)
}

func assertHistogramValue(t *testing.T, metrics *recordingMetrics, name string, value float64, labels map[string]string) {
	t.Helper()
	for _, call := range metrics.histograms {
		if call.name == name && call.value == value && labelsContain(call.labels, labels) {
			return
		}
	}
	t.Fatalf("histogram %q value %f with labels %#v not found in %#v", name, value, labels, metrics.histograms)
}

func assertGauge(t *testing.T, metrics *recordingMetrics, name string, value float64, labels map[string]string) {
	t.Helper()
	for _, call := range metrics.gauges {
		if call.name == name && call.value == value && labelsContain(call.labels, labels) {
			return
		}
	}
	t.Fatalf("gauge %q value %f with labels %#v not found in %#v", name, value, labels, metrics.gauges)
}

func assertHealth(t *testing.T, status HealthStatus, value HealthStatusValue, ready bool, live bool) {
	t.Helper()
	if status.Status != value || status.Ready != ready || status.Live != live {
		t.Fatalf("health status = %#v, want status=%s ready=%v live=%v", status, value, ready, live)
	}
}

func labelsContain(got map[string]string, want map[string]string) bool {
	for key, value := range want {
		if got[key] != value {
			return false
		}
	}
	return true
}
