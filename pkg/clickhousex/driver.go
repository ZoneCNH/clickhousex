package clickhousex

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type connector interface {
	Open(context.Context, Config) (driverConn, error)
}

type driverConn interface {
	Exec(context.Context, string, ...any) error
	Query(context.Context, string, ...any) (driverRows, error)
	PrepareBatch(context.Context, string) (driverBatch, error)
	Ping(context.Context) error
	Stats() driverStats
	Close() error
}

type driverRows interface {
	Next() bool
	Scan(...any) error
	Close() error
	Err() error
	ColumnTypes() []ColumnType
}

type driverBatch interface {
	Append(...any) error
	Send() error
	Abort() error
	Close() error
	Rows() int
}

type driverStats struct {
	MaxOpenConns int
	MaxIdleConns int
	Open         int
	Idle         int
}

type clickhouseConnector struct{}

var openClickHouse = clickhouse.Open

func (clickhouseConnector) Open(ctx context.Context, cfg Config) (driverConn, error) {
	opt, err := cfg.clickhouseOptions()
	if err != nil {
		return nil, err
	}
	conn, err := openClickHouse(opt)
	if err != nil {
		return nil, operationError(ErrorKindConnection, "clickhousex.Open", err)
	}
	if err := conn.Ping(ctx); err != nil {
		_ = conn.Close()
		return nil, operationError(ErrorKindConnection, "clickhousex.Open", err)
	}
	return nativeConn{conn: conn}, nil
}

type nativeConn struct {
	conn chdriver.Conn
}

func (c nativeConn) Exec(ctx context.Context, query string, args ...any) error {
	return c.conn.Exec(ctx, query, args...)
}

func (c nativeConn) Query(ctx context.Context, query string, args ...any) (driverRows, error) {
	rows, err := c.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return nativeRows{rows: rows}, nil
}

func (c nativeConn) PrepareBatch(ctx context.Context, query string) (driverBatch, error) {
	batch, err := c.conn.PrepareBatch(ctx, query)
	if err != nil {
		return nil, err
	}
	return nativeBatch{batch: batch}, nil
}

func (c nativeConn) Ping(ctx context.Context) error {
	return c.conn.Ping(ctx)
}

func (c nativeConn) Stats() driverStats {
	stats := c.conn.Stats()
	return driverStats{
		MaxOpenConns: stats.MaxOpenConns,
		MaxIdleConns: stats.MaxIdleConns,
		Open:         stats.Open,
		Idle:         stats.Idle,
	}
}

func (c nativeConn) Close() error {
	return c.conn.Close()
}

type nativeRows struct {
	rows chdriver.Rows
}

func (r nativeRows) Next() bool {
	return r.rows.Next()
}

func (r nativeRows) Scan(dest ...any) error {
	return r.rows.Scan(dest...)
}

func (r nativeRows) Close() error {
	return r.rows.Close()
}

func (r nativeRows) Err() error {
	return r.rows.Err()
}

func (r nativeRows) ColumnTypes() []ColumnType {
	types := r.rows.ColumnTypes()
	columns := make([]ColumnType, 0, len(types))
	for _, col := range types {
		columns = append(columns, ColumnType{
			Name:     col.Name(),
			Type:     col.DatabaseTypeName(),
			Nullable: col.Nullable(),
		})
	}
	return columns
}

type nativeBatch struct {
	batch chdriver.Batch
}

func (b nativeBatch) Append(values ...any) error {
	return b.batch.Append(values...)
}

func (b nativeBatch) Send() error {
	return b.batch.Send()
}

func (b nativeBatch) Abort() error {
	return b.batch.Abort()
}

func (b nativeBatch) Close() error {
	return b.batch.Close()
}

func (b nativeBatch) Rows() int {
	return b.batch.Rows()
}
