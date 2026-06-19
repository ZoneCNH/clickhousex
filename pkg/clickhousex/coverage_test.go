package clickhousex

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	chproto "github.com/ClickHouse/ch-go/proto"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/shopspring/decimal"
)

type nativeStubConn struct {
	execErr    error
	queryErr   error
	prepareErr error
	pingErr    error
	closeErr   error
	rows       chdriver.Rows
	batch      chdriver.Batch
	stats      chdriver.Stats

	execCalls    int
	queryCalls   int
	prepareCalls int
	pingCalls    int
	closeCalls   int
}

func (c *nativeStubConn) Contributors() []string {
	return []string{"tester"}
}

func (c *nativeStubConn) ServerVersion() (*chdriver.ServerVersion, error) {
	return nil, nil
}

func (c *nativeStubConn) Select(ctx context.Context, dest any, query string, args ...any) error {
	_ = ctx
	_ = dest
	_ = query
	_ = args
	return nil
}

func (c *nativeStubConn) Query(ctx context.Context, query string, args ...any) (chdriver.Rows, error) {
	_ = ctx
	_ = query
	_ = args
	c.queryCalls++
	if c.queryErr != nil {
		return nil, c.queryErr
	}
	if c.rows == nil {
		c.rows = &nativeStubRows{}
	}
	return c.rows, nil
}

func (c *nativeStubConn) QueryRow(ctx context.Context, query string, args ...any) chdriver.Row {
	_ = ctx
	_ = query
	_ = args
	return nativeStubRow{}
}

func (c *nativeStubConn) PrepareBatch(ctx context.Context, query string, opts ...chdriver.PrepareBatchOption) (chdriver.Batch, error) {
	_ = ctx
	_ = query
	_ = opts
	c.prepareCalls++
	if c.prepareErr != nil {
		return nil, c.prepareErr
	}
	if c.batch == nil {
		c.batch = &nativeStubBatch{}
	}
	return c.batch, nil
}

func (c *nativeStubConn) Exec(ctx context.Context, query string, args ...any) error {
	_ = ctx
	_ = query
	_ = args
	c.execCalls++
	return c.execErr
}

func (c *nativeStubConn) AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error {
	_ = ctx
	_ = query
	_ = wait
	_ = args
	return nil
}

func (c *nativeStubConn) Ping(ctx context.Context) error {
	_ = ctx
	c.pingCalls++
	return c.pingErr
}

func (c *nativeStubConn) Stats() chdriver.Stats {
	return c.stats
}

func (c *nativeStubConn) Close() error {
	c.closeCalls++
	return c.closeErr
}

type nativeStubRows struct {
	next       []bool
	scanErr    error
	closeErr   error
	err        error
	columns    []chdriver.ColumnType
	scanCalls  int
	closeCalls int
}

func (r *nativeStubRows) Next() bool {
	if len(r.next) == 0 {
		return false
	}
	got := r.next[0]
	r.next = r.next[1:]
	return got
}

func (r *nativeStubRows) Scan(dest ...any) error {
	_ = dest
	r.scanCalls++
	return r.scanErr
}

func (r *nativeStubRows) ScanStruct(dest any) error {
	_ = dest
	return nil
}

func (r *nativeStubRows) ColumnTypes() []chdriver.ColumnType {
	return append([]chdriver.ColumnType(nil), r.columns...)
}

func (r *nativeStubRows) Totals(dest ...any) error {
	_ = dest
	return nil
}

func (r *nativeStubRows) Columns() []string {
	return []string{"id"}
}

func (r *nativeStubRows) Close() error {
	r.closeCalls++
	return r.closeErr
}

func (r *nativeStubRows) Err() error {
	return r.err
}

type nativeStubColumnType struct {
	name     string
	typ      string
	nullable bool
}

func (c nativeStubColumnType) Name() string {
	return c.name
}

func (c nativeStubColumnType) Nullable() bool {
	return c.nullable
}

func (c nativeStubColumnType) ScanType() reflect.Type {
	return reflect.TypeOf("")
}

func (c nativeStubColumnType) DatabaseTypeName() string {
	return c.typ
}

type nativeStubBatch struct {
	appendErr  error
	sendErr    error
	abortErr   error
	closeErr   error
	rows       int
	appendCall int
	sendCall   int
	abortCall  int
	closeCall  int
}

func (b *nativeStubBatch) Abort() error {
	b.abortCall++
	return b.abortErr
}

func (b *nativeStubBatch) Append(v ...any) error {
	_ = v
	b.appendCall++
	if b.appendErr != nil {
		return b.appendErr
	}
	b.rows++
	return nil
}

func (b *nativeStubBatch) AppendStruct(v any) error {
	_ = v
	return nil
}

func (b *nativeStubBatch) Column(i int) chdriver.BatchColumn {
	_ = i
	return nativeStubBatchColumn{}
}

func (b *nativeStubBatch) Flush() error {
	return nil
}

func (b *nativeStubBatch) Send() error {
	b.sendCall++
	return b.sendErr
}

func (b *nativeStubBatch) IsSent() bool {
	return b.sendCall > 0
}

func (b *nativeStubBatch) Rows() int {
	return b.rows
}

func (b *nativeStubBatch) Columns() []column.Interface {
	return []column.Interface{nativeStubColumn{}}
}

func (b *nativeStubBatch) Close() error {
	b.closeCall++
	return b.closeErr
}

type nativeStubBatchColumn struct{}

func (nativeStubBatchColumn) Append(v any) error {
	_ = v
	return nil
}

func (nativeStubBatchColumn) AppendRow(v any) error {
	_ = v
	return nil
}

type nativeStubColumn struct{}

func (nativeStubColumn) Name() string {
	return "id"
}

func (nativeStubColumn) Type() column.Type {
	return column.Type("String")
}

func (nativeStubColumn) Rows() int {
	return 0
}

func (nativeStubColumn) Row(i int, ptr bool) any {
	_ = i
	_ = ptr
	return nil
}

func (nativeStubColumn) ScanRow(dest any, row int) error {
	_ = dest
	_ = row
	return nil
}

func (nativeStubColumn) Append(v any) ([]uint8, error) {
	_ = v
	return nil, nil
}

func (nativeStubColumn) AppendRow(v any) error {
	_ = v
	return nil
}

func (nativeStubColumn) Decode(reader *chproto.Reader, rows int) error {
	_ = reader
	_ = rows
	return nil
}

func (nativeStubColumn) Encode(buffer *chproto.Buffer) {
	_ = buffer
}

func (nativeStubColumn) ScanType() reflect.Type {
	return reflect.TypeOf("")
}

func (nativeStubColumn) Reset() {}

type nativeStubRow struct{}

func (nativeStubRow) Err() error {
	return nil
}

func (nativeStubRow) Scan(dest ...any) error {
	_ = dest
	return nil
}

func (nativeStubRow) ScanStruct(dest any) error {
	_ = dest
	return nil
}

func TestClickhouseConnectorOpenBranches(t *testing.T) {
	originalOpen := openClickHouse
	t.Cleanup(func() {
		openClickHouse = originalOpen
	})

	_, err := (&clickhouseConnector{}).Open(context.Background(), Config{Name: "primary", DSN: "%"})
	if err == nil || !IsKind(err, ErrorKindValidation) {
		t.Fatalf("Open(invalid dsn) = %v", err)
	}

	openClickHouse = func(opt *clickhouse.Options) (chdriver.Conn, error) {
		_ = opt
		return nil, errors.New("dial refused")
	}
	_, err = (&clickhouseConnector{}).Open(context.Background(), validConfig())
	if err == nil || !IsKind(err, ErrorKindConnection) {
		t.Fatalf("Open(driver error) = %v", err)
	}

	pingConn := &nativeStubConn{pingErr: errors.New("server closed")}
	openClickHouse = func(opt *clickhouse.Options) (chdriver.Conn, error) {
		_ = opt
		return pingConn, nil
	}
	_, err = (&clickhouseConnector{}).Open(context.Background(), validConfig())
	if err == nil || !IsKind(err, ErrorKindConnection) || pingConn.closeCalls != 1 {
		t.Fatalf("Open(ping error) err=%v closeCalls=%d", err, pingConn.closeCalls)
	}

	var gotOptions *clickhouse.Options
	okConn := &nativeStubConn{}
	openClickHouse = func(opt *clickhouse.Options) (chdriver.Conn, error) {
		gotOptions = opt
		return okConn, nil
	}
	conn, err := (&clickhouseConnector{}).Open(context.Background(), validConfig())
	if err != nil {
		t.Fatalf("Open(valid) returned error: %v", err)
	}
	if _, ok := conn.(nativeConn); !ok {
		t.Fatalf("Open(valid) conn type = %T, want nativeConn", conn)
	}
	if okConn.pingCalls != 1 || gotOptions == nil || len(gotOptions.Addr) != 1 || gotOptions.Addr[0] != "localhost:9000" {
		t.Fatalf("Open(valid) pingCalls=%d options=%#v", okConn.pingCalls, gotOptions)
	}
}

func TestNativeAdaptersForwardCallsAndMapTypes(t *testing.T) {
	rows := &nativeStubRows{
		next: []bool{true},
		columns: []chdriver.ColumnType{
			nativeStubColumnType{name: "price", typ: "Decimal(18,2)", nullable: true},
		},
	}
	batch := &nativeStubBatch{}
	stub := &nativeStubConn{
		rows:  rows,
		batch: batch,
		stats: chdriver.Stats{MaxOpenConns: 2, MaxIdleConns: 1, Open: 1, Idle: 1},
	}
	conn := &nativeConn{conn: stub}

	if err := conn.Exec(context.Background(), "SELECT 1"); err != nil {
		t.Fatalf("native Exec returned error: %v", err)
	}
	gotRows, err := conn.Query(context.Background(), "SELECT 1")
	if err != nil {
		t.Fatalf("native Query returned error: %v", err)
	}
	if !gotRows.Next() {
		t.Fatal("native rows Next returned false")
	}
	if err := gotRows.Scan(new(string)); err != nil {
		t.Fatalf("native rows Scan returned error: %v", err)
	}
	mapped := gotRows.ColumnTypes()
	if len(mapped) != 1 || mapped[0].Name != "price" || mapped[0].Type != "Decimal(18,2)" || !mapped[0].Nullable {
		t.Fatalf("native ColumnTypes = %#v", mapped)
	}
	if err := gotRows.Close(); err != nil || rows.closeCalls != 1 {
		t.Fatalf("native rows Close err=%v calls=%d", err, rows.closeCalls)
	}
	if err := gotRows.Err(); err != nil {
		t.Fatalf("native rows Err returned error: %v", err)
	}

	gotBatch, err := conn.PrepareBatch(context.Background(), "INSERT INTO t VALUES")
	if err != nil {
		t.Fatalf("native PrepareBatch returned error: %v", err)
	}
	if err := gotBatch.Append(1); err != nil {
		t.Fatalf("native batch Append returned error: %v", err)
	}
	if gotBatch.Rows() != 1 {
		t.Fatalf("native batch Rows = %d, want 1", gotBatch.Rows())
	}
	if err := gotBatch.Send(); err != nil {
		t.Fatalf("native batch Send returned error: %v", err)
	}
	if err := gotBatch.Abort(); err != nil {
		t.Fatalf("native batch Abort returned error: %v", err)
	}
	if err := gotBatch.Close(); err != nil {
		t.Fatalf("native batch Close returned error: %v", err)
	}
	if err := conn.Ping(context.Background()); err != nil {
		t.Fatalf("native Ping returned error: %v", err)
	}
	if stats := conn.Stats(); stats.Open != 1 || stats.Idle != 1 {
		t.Fatalf("native Stats = %#v", stats)
	}
	if err := conn.Close(); err != nil || stub.closeCalls != 1 {
		t.Fatalf("native Close err=%v calls=%d", err, stub.closeCalls)
	}

	stub.execErr = errors.New("exec")
	if err := conn.Exec(context.Background(), "SELECT 1"); err == nil {
		t.Fatal("native Exec(error) returned nil")
	}
	stub.queryErr = errors.New("query")
	if _, err := conn.Query(context.Background(), "SELECT 1"); err == nil {
		t.Fatal("native Query(error) returned nil")
	}
	stub.prepareErr = errors.New("prepare")
	if _, err := conn.PrepareBatch(context.Background(), "INSERT"); err == nil {
		t.Fatal("native PrepareBatch(error) returned nil")
	}
}

func TestConfigBranchCoverageAndOptions(t *testing.T) {
	cases := []Config{
		{},
		{Name: "primary"},
		{Name: "primary", Host: "localhost", Port: -1},
		{Name: "primary", Host: "localhost", MaxOpenConns: -1},
		{Name: "primary", Host: "localhost", MaxOpenConns: MaxAllowedOpenConns + 1},
		{Name: "primary", Host: "localhost", MaxIdleConns: -1},
		{Name: "primary", Host: "localhost", MaxOpenConns: 1, MaxIdleConns: 2},
		{Name: "primary", Host: "localhost", ConnMaxLifetime: -1},
		{Name: "primary", Host: "localhost", Timeout: -1},
		{Name: "primary", DSN: "%"},
	}
	for _, cfg := range cases {
		if err := cfg.Validate(); err == nil {
			t.Fatalf("Validate(%#v) returned nil", cfg)
		}
	}

	defaulted, err := (Config{Name: "primary", Host: "localhost"}).withDefaults()
	if err != nil {
		t.Fatalf("withDefaults returned error: %v", err)
	}
	if defaulted.Port != DefaultPort || defaulted.ConnMaxLifetime != DefaultConnMaxLifetime {
		t.Fatalf("withDefaults = %#v", defaulted)
	}

	opt, err := defaulted.clickhouseOptions()
	if err != nil {
		t.Fatalf("clickhouseOptions(host) returned error: %v", err)
	}
	if got := opt.Addr[0]; got != "localhost:9000" {
		t.Fatalf("clickhouseOptions(host) addr = %q", got)
	}

	dsnCfg := validConfig()
	dsnCfg.DSN = "clickhouse://default:secret@localhost:9000/default"
	dsnCfg.Host = ""
	opt, err = dsnCfg.clickhouseOptions()
	if err != nil {
		t.Fatalf("clickhouseOptions(dsn) returned error: %v", err)
	}
	if opt.MaxOpenConns != dsnCfg.MaxOpenConns || opt.DialTimeout != dsnCfg.Timeout {
		t.Fatalf("clickhouseOptions(dsn) = %#v", opt)
	}

	badDSN := validConfig()
	badDSN.DSN = "%"
	if _, err := badDSN.clickhouseOptions(); err == nil {
		t.Fatal("clickhouseOptions(invalid dsn) returned nil")
	}

	if got := sanitizeDSN(""); got != "" {
		t.Fatalf("sanitizeDSN(empty) = %q", got)
	}
	if got := sanitizeDSN("%"); got != "***" {
		t.Fatalf("sanitizeDSN(invalid) = %q", got)
	}
	sanitized := sanitizeDSN("clickhouse://default@localhost:9000/default?password=a&pass=b&secret=c&token=d")
	if strings.Contains(sanitized, "password=a") || strings.Contains(sanitized, "token=d") {
		t.Fatalf("sanitizeDSN(query secrets) = %q", sanitized)
	}
}

func TestClientFailureBranches(t *testing.T) {
	metrics := &recordingMetrics{}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if client, err := New(canceled, validConfig(), WithMetrics(metrics)); client != nil || err == nil || !IsKind(err, ErrorKindUnavailable) {
		t.Fatalf("New(canceled) client=%#v err=%v", client, err)
	}
	if client, err := New(nilContext(), validConfig(), WithMetrics(metrics)); client != nil || err == nil || !IsKind(err, ErrorKindValidation) {
		t.Fatalf("New(nil ctx) client=%#v err=%v", client, err)
	}
	if client, err := New(context.Background(), Config{Name: "primary", Host: "localhost", Port: -1}, WithMetrics(metrics)); client != nil || err == nil || !IsKind(err, ErrorKindValidation) {
		t.Fatalf("New(invalid config) client=%#v err=%v", client, err)
	}
	if client, err := New(context.Background(), Config{Name: "primary", DSN: "%"}, WithMetrics(metrics)); client != nil || err == nil || !IsKind(err, ErrorKindValidation) {
		t.Fatalf("New(invalid dsn defaults) client=%#v err=%v", client, err)
	}

	connectorErr := errors.New("connect refused")
	if client, err := New(context.Background(), validConfig(), withConnector(&fakeConnector{err: connectorErr})); client != nil || err == nil || !IsKind(err, ErrorKindConnection) {
		t.Fatalf("New(connector error) client=%#v err=%v", client, err)
	}

	if err := (*Client)(nil).Exec(context.Background(), "SELECT 1"); err == nil {
		t.Fatal("Exec(nil client) returned nil")
	}
	client, _, _, _ := newTestClient(t, &fakeConn{})
	if err := client.Exec(context.Background(), " "); err == nil || !IsKind(err, ErrorKindValidation) {
		t.Fatalf("Exec(empty) = %v", err)
	}
	if err := client.Exec(canceled, "SELECT 1"); err == nil || !IsKind(err, ErrorKindUnavailable) {
		t.Fatalf("Exec(canceled) = %v", err)
	}
	if err := (&Client{metrics: NoopMetrics{}, tracer: noopTracer{}, logger: noopLogger{}}).Exec(context.Background(), "SELECT 1"); err == nil || !IsKind(err, ErrorKindValidation) {
		t.Fatalf("Exec(uninitialized) = %v", err)
	}
	if err := (&Client{initialized: true, closed: true, metrics: NoopMetrics{}, tracer: noopTracer{}, logger: noopLogger{}}).Exec(context.Background(), "SELECT 1"); err == nil || !IsKind(err, ErrorKindUnavailable) {
		t.Fatalf("Exec(closed) = %v", err)
	}
	if err := (&Client{initialized: true, metrics: NoopMetrics{}, tracer: noopTracer{}, logger: noopLogger{}}).Exec(context.Background(), "SELECT 1"); err == nil || !IsKind(err, ErrorKindUnavailable) {
		t.Fatalf("Exec(nil conn) = %v", err)
	}
	conn := &fakeConn{execErrs: []error{errors.New("syntax error")}}
	client, _, _, _ = newTestClient(t, conn)
	if err := client.Exec(context.Background(), "SELECT 1"); err == nil || !IsKind(err, ErrorKindQuery) {
		t.Fatalf("Exec(nonretry error) = %v", err)
	}

	if rows, err := (*Client)(nil).Query(context.Background(), "SELECT 1"); rows != nil || err == nil {
		t.Fatalf("Query(nil client) rows=%#v err=%v", rows, err)
	}
	if rows, err := client.Query(context.Background(), " "); rows != nil || err == nil || !IsKind(err, ErrorKindValidation) {
		t.Fatalf("Query(empty) rows=%#v err=%v", rows, err)
	}
	if rows, err := client.Query(canceled, "SELECT 1"); rows != nil || err == nil || !IsKind(err, ErrorKindUnavailable) {
		t.Fatalf("Query(canceled) rows=%#v err=%v", rows, err)
	}
	conn = &fakeConn{queryErrs: []error{errors.New("syntax error")}}
	client, _, _, _ = newTestClient(t, conn)
	if rows, err := client.Query(context.Background(), "SELECT 1"); rows != nil || err == nil || !IsKind(err, ErrorKindQuery) {
		t.Fatalf("Query(error) rows=%#v err=%v", rows, err)
	}
	successRows := &fakeRows{
		next:    []bool{true},
		columns: []ColumnType{{Name: "id", Type: "UInt64"}},
		scanFn: func(dest ...any) error {
			*dest[0].(*uint64) = 42
			return nil
		},
	}
	conn = &fakeConn{rows: successRows}
	client, _, _, _ = newTestClient(t, conn)
	gotRows, err := client.Query(context.Background(), "SELECT 1")
	if err != nil || gotRows == nil {
		t.Fatalf("Query(success) rows=%#v err=%v", gotRows, err)
	}
	if !gotRows.Next() {
		t.Fatal("Query(success) rows.Next returned false")
	}
	var gotID uint64
	if err := gotRows.Scan(&gotID); err != nil {
		t.Fatalf("Query(success) rows.Scan = %v", err)
	}
	if gotID != 42 {
		t.Fatalf("Query(success) scanned id = %d", gotID)
	}

	if err := (*Client)(nil).InsertBatch(context.Background(), "events", []string{"id"}, [][]any{{1}}); err == nil {
		t.Fatal("InsertBatch(nil client) returned nil")
	}
	if err := client.InsertBatch(context.Background(), "", []string{"id"}, [][]any{{1}}); err == nil || !IsKind(err, ErrorKindValidation) {
		t.Fatalf("InsertBatch(empty table) = %v", err)
	}
	if err := client.InsertBatch(context.Background(), "bad-name", []string{"id"}, [][]any{{1}}); err == nil || !IsKind(err, ErrorKindValidation) {
		t.Fatalf("InsertBatch(invalid table) = %v", err)
	}
	if err := client.InsertBatch(context.Background(), "events", []string{"bad-name"}, [][]any{{1}}); err == nil || !IsKind(err, ErrorKindValidation) {
		t.Fatalf("InsertBatch(invalid column) = %v", err)
	}
	if err := client.InsertBatch(canceled, "events", []string{"id"}, [][]any{{1}}); err == nil || !IsKind(err, ErrorKindUnavailable) {
		t.Fatalf("InsertBatch(canceled) = %v", err)
	}
	conn = &fakeConn{prepareErrs: []error{errors.New("prepare failed")}}
	client, _, _, _ = newTestClient(t, conn)
	if err := client.InsertBatch(context.Background(), "events", []string{"id"}, [][]any{{1}}); err == nil || !IsKind(err, ErrorKindBatch) {
		t.Fatalf("InsertBatch(prepare error) = %v", err)
	}
	batch := &fakeBatch{appendErrs: []error{errors.New("append failed")}}
	conn = &fakeConn{batch: batch}
	client, _, _, _ = newTestClient(t, conn)
	if err := client.InsertBatch(context.Background(), "events", []string{"id"}, [][]any{{1}}); err == nil || !IsKind(err, ErrorKindBatch) || !batch.aborted {
		t.Fatalf("InsertBatch(append error) err=%v aborted=%v", err, batch.aborted)
	}
	batch = &fakeBatch{sendErr: errors.New("send failed")}
	conn = &fakeConn{batch: batch}
	client, _, _, _ = newTestClient(t, conn)
	if err := client.InsertBatch(context.Background(), "events", []string{"id"}, [][]any{{1}}); err == nil || !IsKind(err, ErrorKindBatch) || !batch.aborted {
		t.Fatalf("InsertBatch(send error) err=%v aborted=%v", err, batch.aborted)
	}

	conn = &fakeConn{pingErr: errors.New("ping failed")}
	client, _, _, _ = newTestClient(t, conn)
	if err := client.Ping(canceled); err == nil || !IsKind(err, ErrorKindUnavailable) {
		t.Fatalf("Ping(canceled) = %v", err)
	}
	if err := client.Ping(context.Background()); err == nil || !IsKind(err, ErrorKindConnection) {
		t.Fatalf("Ping(error) = %v", err)
	}

	if err := (&Client{metrics: NoopMetrics{}}).Close(); err == nil || !IsKind(err, ErrorKindValidation) {
		t.Fatalf("Close(uninitialized) = %v", err)
	}
	if err := (&Client{initialized: true, connClosed: true, metrics: NoopMetrics{}}).Close(); err != nil {
		t.Fatalf("Close(connClosed) = %v", err)
	}
	nilCtxClient, _, _, _ := newTestClient(t, &fakeConn{})
	if err := nilCtxClient.CloseContext(nilContext()); err == nil || !IsKind(err, ErrorKindValidation) {
		t.Fatalf("CloseContext(nil ctx) = %v", err)
	}
	canceledClose, closeCancel := context.WithCancel(context.Background())
	closeCancel()
	canceledCloseClient, _, _, _ := newTestClient(t, &fakeConn{})
	if err := canceledCloseClient.CloseContext(canceledClose); err == nil || !IsKind(err, ErrorKindUnavailable) {
		t.Fatalf("CloseContext(canceled) = %v", err)
	}
	okCloseClient, closeMetrics, _, _ := newTestClient(t, &fakeConn{})
	if err := okCloseClient.Close(); err != nil {
		t.Fatalf("Close(success) = %v", err)
	}
	assertCounter(t, closeMetrics, MetricClientClosedTotal, map[string]string{"name": "primary"})
	closeErrConn := &fakeConn{closeErr: errors.New("close failed")}
	closeErrClient, _, _, _ := newTestClient(t, closeErrConn)
	if err := closeErrClient.Close(); err == nil || !IsKind(err, ErrorKindConnection) {
		t.Fatalf("Close(close error) = %v", err)
	}
	nilConnClient := &Client{cfg: validConfig(), initialized: true, metrics: NoopMetrics{}}
	if err := nilConnClient.Close(); err != nil {
		t.Fatalf("Close(nil conn) = %v", err)
	}
	waitClient, _, _, _ := newTestClient(t, &fakeConn{})
	waitClient.wg.Add(1)
	shortCtx, shortCancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer shortCancel()
	if err := waitClient.CloseContext(shortCtx); err == nil || !IsKind(err, ErrorKindTimeout) {
		t.Fatalf("CloseContext(wait timeout) = %v", err)
	}
	waitClient.wg.Done()
	cancelWaitClient, _, _, _ := newTestClient(t, &fakeConn{})
	cancelWaitClient.wg.Add(1)
	waitCtx, waitCancel := context.WithCancel(context.Background())
	cancelDone := make(chan error, 1)
	go func() {
		cancelDone <- cancelWaitClient.CloseContext(waitCtx)
	}()
	time.Sleep(10 * time.Millisecond)
	waitCancel()
	if err := <-cancelDone; err == nil || !IsKind(err, ErrorKindUnavailable) {
		t.Fatalf("CloseContext(wait canceled) = %v", err)
	}
	cancelWaitClient.wg.Done()
}

func TestRetryAndHelperBranches(t *testing.T) {
	client, metrics, _, _ := newTestClient(t, &fakeConn{})
	client.retry.MaxAttempts = 0
	calls := 0
	if err := client.runWithRetry(context.Background(), "op", "helper", func() error {
		calls++
		return errors.New("syntax")
	}); err == nil || calls != 1 {
		t.Fatalf("runWithRetry(max attempts zero) err=%v calls=%d", err, calls)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := client.runWithRetry(canceled, "op", "helper", func() error {
		t.Fatal("fn should not be called after context cancellation")
		return nil
	}); err == nil || !IsKind(err, ErrorKindUnavailable) {
		t.Fatalf("runWithRetry(pre-canceled) = %v", err)
	}

	client.retry = RetryConfig{MaxAttempts: 2, BaseDelay: time.Hour, MaxDelay: time.Hour}
	delayCtx, delayCancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(time.Millisecond)
		delayCancel()
	}()
	if err := client.runWithRetry(delayCtx, "op", "helper", func() error {
		return WrapError(ErrorKindConnection, "op", "retry", true, io.EOF)
	}); err == nil || !IsKind(err, ErrorKindUnavailable) {
		t.Fatalf("runWithRetry(cancel during delay) = %v", err)
	}
	assertCounter(t, metrics, MetricClientRetriesTotal, map[string]string{"op": "helper", "attempt": "1"})

	client.retry = RetryConfig{MaxAttempts: 3}
	retryCalls := 0
	if err := client.runWithRetry(context.Background(), "op", "helper", func() error {
		retryCalls++
		if retryCalls == 1 {
			return WrapError(ErrorKindConnection, "op", "retry", true, io.EOF)
		}
		return nil
	}); err != nil || retryCalls != 2 {
		t.Fatalf("runWithRetry(success after retry) err=%v calls=%d", err, retryCalls)
	}
	client.retry = RetryConfig{MaxAttempts: 2, BaseDelay: time.Nanosecond, MaxDelay: time.Millisecond}
	delayedCalls := 0
	if err := client.runWithRetry(context.Background(), "op", "helper", func() error {
		delayedCalls++
		if delayedCalls == 1 {
			return WrapError(ErrorKindConnection, "op", "retry", true, io.EOF)
		}
		return nil
	}); err != nil || delayedCalls != 2 {
		t.Fatalf("runWithRetry(success after timed retry) err=%v calls=%d", err, delayedCalls)
	}
	client.retry = RetryConfig{MaxAttempts: 2}
	exhaustedCalls := 0
	if err := client.runWithRetry(context.Background(), "op", "helper", func() error {
		exhaustedCalls++
		return WrapError(ErrorKindConnection, "op", "retry", true, io.EOF)
	}); err == nil || !IsKind(err, ErrorKindConnection) || exhaustedCalls != 2 {
		t.Fatalf("runWithRetry(exhausted) err=%v calls=%d", err, exhaustedCalls)
	}
	if got := client.retryDelay(1); got != 0 {
		t.Fatalf("retryDelay(no base delay) = %v", got)
	}
	client.retry = RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Hour}
	if got := client.retryDelay(1); got != time.Millisecond {
		t.Fatalf("retryDelay(base) = %v", got)
	}
	client.retry = RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond}
	if got := client.retryDelay(3); got != time.Millisecond {
		t.Fatalf("retryDelay(cap) = %v", got)
	}
	client.retry = RetryConfig{MaxAttempts: 3, BaseDelay: 2 * time.Millisecond, MaxDelay: time.Millisecond}
	if got := client.retryDelay(1); got != time.Millisecond {
		t.Fatalf("retryDelay(first cap) = %v", got)
	}
	if shouldRetry(nil) {
		t.Fatal("shouldRetry(nil) returned true")
	}
	if shouldRetry(NewError(ErrorKindQuery, "op", "no", false)) {
		t.Fatal("shouldRetry(nonretry typed) returned true")
	}
	if !shouldRetry(io.EOF) {
		t.Fatal("shouldRetry(io.EOF) returned false")
	}

	labels := client.operationLabels("insert_batch", "events", "insert")
	if labels["name"] != "primary" || labels["table"] != "events" || labels["query"] != "insert" {
		t.Fatalf("operationLabels = %#v", labels)
	}
	client.observeOperation("insert_batch", labels, time.Now(), nil)
	client.observeOperation("query", client.operationLabels("query", "", "select"), time.Now(), errors.New("x"))

	if got, err := buildInsertQuery("events", []string{"id", "ts"}); err != nil || got != "INSERT INTO `events` (`id`, `ts`) VALUES" {
		t.Fatalf("buildInsertQuery = %q, %v", got, err)
	}
	if kind := statementKind(" \n\t "); kind != "" {
		t.Fatalf("statementKind(empty) = %q", kind)
	}
	if got := strconvItoa(-12); got != "-12" {
		t.Fatalf("strconvItoa(-12) = %q", got)
	}
	if got := strconvItoa(120); got != "120" {
		t.Fatalf("strconvItoa(120) = %q", got)
	}
	recordPoolMetrics(nil, "primary", &fakeConn{})
	recordPoolMetrics(metrics, "primary", nil)
	recordPoolMetrics(metrics, "primary", &fakeConn{stats: driverStats{MaxOpenConns: 2, MaxIdleConns: 1, Open: 1, Idle: 1}})
	recordErrorMetric(nil, "op", errors.New("x"))
}

func TestErrorAndRowsBranches(t *testing.T) {
	var nilErr *Error
	if nilErr.Error() != "" {
		t.Fatalf("nil Error string = %q", nilErr.Error())
	}
	if nilErr.Unwrap() != nil {
		t.Fatal("nil Error unwrap returned non-nil")
	}
	if nilErr.Is(ErrTypeMismatch) {
		t.Fatal("nil Error Is returned true")
	}
	if !strings.Contains(NewError(ErrorKindInternal, "", "", false).Error(), string(ErrorKindInternal)) {
		t.Fatal("NewError(empty) did not include kind")
	}
	causeOnly := &Error{Kind: ErrorKindQuery, Cause: errors.New("driver detail")}
	if !strings.Contains(causeOnly.Error(), "driver detail") {
		t.Fatalf("Error(cause only) = %q", causeOnly.Error())
	}
	if got := NewError(ErrorKindQuery, "op", "", false).Error(); got != "clickhousex: op" {
		t.Fatalf("Error(op only) = %q", got)
	}
	wrapped := WrapError(ErrorKindQuery, "op", "", false, errors.New("query failed"))
	if wrapped.Unwrap() == nil || !errors.Is(wrapped, NewError(ErrorKindQuery, "", "", false)) {
		t.Fatalf("WrapError/Is failed: %v", wrapped)
	}
	errWithKind := NewError(ErrorKindQuery, "", "", false)
	if errWithKind.Is(nil) {
		t.Fatal("Error.Is(nil) returned true")
	}
	if errWithKind.Is(errors.New("plain")) {
		t.Fatal("Error.Is(plain) returned true")
	}
	if errWithKind.Is(NewError("", "", "", false)) {
		t.Fatal("Error.Is(empty target kind) returned true")
	}
	if IsKind(errors.New("plain"), ErrorKindQuery) {
		t.Fatal("IsKind(plain) returned true")
	}
	if contextError("op", context.DeadlineExceeded).Kind != ErrorKindTimeout {
		t.Fatal("contextError(deadline) did not map timeout")
	}
	if operationError(ErrorKindQuery, "op", nil) != nil {
		t.Fatal("operationError(nil) returned non-nil")
	}
	if got := operationError(ErrorKindQuery, "op", ErrTableNotFound); got != ErrTableNotFound {
		t.Fatalf("operationError(typed) = %#v", got)
	}
	if got := operationError(ErrorKindQuery, "op", context.Canceled); got.Kind != ErrorKindUnavailable {
		t.Fatalf("operationError(canceled) = %#v", got)
	}
	if got := operationError(ErrorKindQuery, "op", errors.New("table does not exist")); got.Kind != ErrorKindTableNotFound || got.Retryable {
		t.Fatalf("operationError(table missing) = %#v", got)
	}
	if got := operationError(ErrorKindQuery, "op", errors.New("connection refused")); got.Kind != ErrorKindConnection || !got.Retryable {
		t.Fatalf("operationError(retryable) = %#v", got)
	}
	if got := operationError(ErrorKindQuery, "op", errors.New("syntax")); got.Kind != ErrorKindQuery || got.Retryable {
		t.Fatalf("operationError(nonretry) = %#v", got)
	}
	if isRetryableError(nil) || !isRetryableError(WrapError(ErrorKindConnection, "op", "retry", true, nil)) {
		t.Fatal("isRetryableError typed branches failed")
	}
	if isTableNotFoundError(nil) || !isTableNotFoundError(errors.New("unknown table events")) || !isTableNotFoundError(errors.New("table doesn't exist")) {
		t.Fatal("isTableNotFoundError marker branches failed")
	}
	if !isTableNotFoundError(ErrTableNotFound) || isTableNotFoundError(errors.New("table exists")) {
		t.Fatal("isTableNotFoundError typed/negative branches failed")
	}
	if got := errorKind(errors.New("plain")); got != ErrorKindInternal {
		t.Fatalf("errorKind(plain) = %s", got)
	}

	rowColumns := []ColumnType{{Name: "value", Type: "LowCardinality(Nullable(Decimal(18,2)))", Nullable: true}}
	anyRows := &rowsWrapper{rows: &fakeRows{columns: rowColumns}}
	var anyDest any
	if err := anyRows.Scan(&anyDest); err != nil {
		t.Fatalf("Rows.Scan(any pointer) = %v", err)
	}

	rowErr := errors.New("scan failed")
	rows := &rowsWrapper{rows: &fakeRows{
		columns: rowColumns,
		scanErr: rowErr,
	}}
	var decimalPtr *decimal.Decimal
	if err := rows.Scan(&decimalPtr); err == nil || !IsKind(err, ErrorKindQuery) {
		t.Fatalf("Rows.Scan(scan error) = %v", err)
	}
	if err := validateScanDestination(ColumnType{}, nil); err == nil || !IsKind(err, ErrorKindTypeMismatch) {
		t.Fatalf("validateScanDestination(nil) = %v", err)
	}
	var nilString *string
	if err := validateScanDestination(ColumnType{}, nilString); err == nil || !IsKind(err, ErrorKindTypeMismatch) {
		t.Fatalf("validateScanDestination(nil pointer) = %v", err)
	}
	var plainString string
	if err := validateScanDestination(ColumnType{}, plainString); err == nil || !IsKind(err, ErrorKindTypeMismatch) {
		t.Fatalf("validateScanDestination(non-pointer) = %v", err)
	}
	if isDecimalType("Nullable(") {
		t.Fatal("isDecimalType malformed nullable prefix unexpectedly true")
	}
	if isDecimalDestination(reflect.TypeOf(decimal.Decimal{})) {
		t.Fatal("isDecimalDestination(non-pointer) returned true")
	}

	okRows := &rowsWrapper{rows: &fakeRows{}}
	if err := okRows.Close(); err != nil {
		t.Fatalf("Rows.Close(success) = %v", err)
	}
	if err := okRows.Err(); err != nil {
		t.Fatalf("Rows.Err(success) = %v", err)
	}
	closeRows := &rowsWrapper{rows: &fakeRows{closeErr: errors.New("close failed"), err: errors.New("row failed")}}
	if err := closeRows.Close(); err == nil || !IsKind(err, ErrorKindQuery) {
		t.Fatalf("Rows.Close(error) = %v", err)
	}
	if err := closeRows.Err(); err == nil || !IsKind(err, ErrorKindQuery) {
		t.Fatalf("Rows.Err(error) = %v", err)
	}
}

func TestHealthBranches(t *testing.T) {
	if status := (*Client)(nil).Health(); status.Status != HealthUnhealthy || status.Name != "clickhousex" {
		t.Fatalf("Health(nil) = %#v", status)
	}
	if status := (*Client)(nil).HealthCheck(nilContext()); status.Message != "context is required" {
		t.Fatalf("HealthCheck(nil ctx) = %#v", status)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if status := (*Client)(nil).HealthCheck(canceled); status.Status != HealthUnhealthy || status.Live {
		t.Fatalf("HealthCheck(canceled) = %#v", status)
	}
	if status := (&Client{}).HealthCheck(context.Background()); status.Message != "client is not initialized" {
		t.Fatalf("HealthCheck(uninitialized) = %#v", status)
	}
	if status := (&Client{cfg: Config{Name: ""}, initialized: true, closed: true}).HealthCheck(context.Background()); status.Name != "clickhousex" || status.Message != "client is closed" {
		t.Fatalf("HealthCheck(empty name closed) = %#v", status)
	}
	if status := (&Client{cfg: validConfig(), initialized: true}).HealthCheck(context.Background()); status.Message != "client connection is nil" {
		t.Fatalf("HealthCheck(nil conn) = %#v", status)
	}
	client, _, _, _ := newTestClient(t, &fakeConn{})
	client.cfg.Timeout = time.Second
	if status := client.Health(); status.Status != HealthHealthy {
		t.Fatalf("Health(valid) = %#v", status)
	}
	if healthGaugeValue(HealthDegraded) != 0 {
		t.Fatal("healthGaugeValue(degraded) != 0")
	}
	recordHealthMetric(nil, HealthStatus{})
}
