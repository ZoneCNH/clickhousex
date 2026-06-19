package clickhousex

import (
	"context"
	"testing"
)

type benchmarkConn struct{}

func (c *benchmarkConn) Exec(ctx context.Context, query string, args ...any) error {
	_ = ctx
	_ = query
	_ = args
	return nil
}

func (c *benchmarkConn) Query(ctx context.Context, query string, args ...any) (driverRows, error) {
	_ = ctx
	_ = query
	_ = args
	return &benchmarkRows{next: true}, nil
}

func (c *benchmarkConn) PrepareBatch(ctx context.Context, query string) (driverBatch, error) {
	_ = ctx
	_ = query
	return &benchmarkBatch{}, nil
}

func (c *benchmarkConn) Ping(ctx context.Context) error {
	_ = ctx
	return nil
}

func (c *benchmarkConn) Stats() driverStats {
	return driverStats{MaxOpenConns: 10, MaxIdleConns: 5, Open: 2, Idle: 1}
}

func (c *benchmarkConn) Close() error {
	return nil
}

type benchmarkRows struct {
	next bool
}

func (r *benchmarkRows) Next() bool {
	if !r.next {
		return false
	}
	r.next = false
	return true
}

func (r *benchmarkRows) Scan(dest ...any) error {
	for _, target := range dest {
		switch ptr := target.(type) {
		case *uint64:
			*ptr = 1
		case *string:
			*ptr = "alpha"
		case *int:
			*ptr = 1
		case *any:
			*ptr = uint64(1)
		}
	}
	return nil
}

func (r *benchmarkRows) Close() error {
	return nil
}

func (r *benchmarkRows) Err() error {
	return nil
}

func (r *benchmarkRows) ColumnTypes() []ColumnType {
	return []ColumnType{
		{Name: "id", Type: "UInt64"},
		{Name: "name", Type: "String"},
	}
}

type benchmarkBatch struct {
	rows int
}

func (b *benchmarkBatch) Append(values ...any) error {
	_ = values
	b.rows++
	return nil
}

func (b *benchmarkBatch) Send() error {
	return nil
}

func (b *benchmarkBatch) Abort() error {
	return nil
}

func (b *benchmarkBatch) Close() error {
	return nil
}

func (b *benchmarkBatch) Rows() int {
	return b.rows
}

func newBenchmarkClient(b *testing.B) *Client {
	b.Helper()
	client, err := New(
		context.Background(),
		validConfig(),
		withConnector(&fakeConnector{conn: &benchmarkConn{}}),
		WithRetryConfig(RetryConfig{MaxAttempts: 1}),
	)
	if err != nil {
		b.Fatalf("new benchmark client: %v", err)
	}
	b.Cleanup(func() {
		if err := client.Close(); err != nil {
			b.Fatalf("close benchmark client: %v", err)
		}
	})
	return client
}

func BenchmarkClientExec(b *testing.B) {
	client := newBenchmarkClient(b)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := client.Exec(ctx, "ALTER TABLE events DELETE WHERE id = ?", uint64(i)); err != nil {
			b.Fatalf("exec benchmark: %v", err)
		}
	}
}

func BenchmarkClientQueryRowsScan(b *testing.B) {
	client := newBenchmarkClient(b)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := client.Query(ctx, "SELECT id, name FROM events LIMIT 1")
		if err != nil {
			b.Fatalf("query benchmark: %v", err)
		}
		if !rows.Next() {
			b.Fatal("query benchmark returned no rows")
		}
		var id uint64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			b.Fatalf("scan benchmark row: %v", err)
		}
		if err := rows.Close(); err != nil {
			b.Fatalf("close benchmark rows: %v", err)
		}
	}
}

func BenchmarkClientInsertBatch(b *testing.B) {
	client := newBenchmarkClient(b)
	ctx := context.Background()
	rows := [][]any{
		{uint64(1), "alpha"},
		{uint64(2), "beta"},
		{uint64(3), "gamma"},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := client.InsertBatch(ctx, "events", []string{"id", "name"}, rows); err != nil {
			b.Fatalf("insert batch benchmark: %v", err)
		}
	}
}

func BenchmarkClientHealthCheck(b *testing.B) {
	client := newBenchmarkClient(b)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		status := client.HealthCheck(ctx)
		if status.Status != HealthHealthy {
			b.Fatalf("health benchmark status = %s", status.Status)
		}
	}
}
