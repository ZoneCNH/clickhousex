package clickhousex

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"time"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Client manages the lifecycle of a ClickHouse connection.
type Client struct {
	cfg         Config
	metrics     Metrics
	tracer      Tracer
	logger      Logger
	retry       RetryConfig
	conn        driverConn
	mu          sync.Mutex
	wg          sync.WaitGroup
	initialized bool
	closed      bool
	connClosed  bool
}

// New creates a new Client, opens the ClickHouse connection, and verifies it.
func New(ctx context.Context, cfg Config, opts ...Option) (*Client, error) {
	const op = "clickhousex.New"
	options := defaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	if ctx == nil {
		err := validationError(op, "context is required", nil)
		recordErrorMetric(options.metrics, "new", err)
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		wrapped := contextError(op, err)
		recordErrorMetric(options.metrics, "new", wrapped)
		return nil, wrapped
	}

	defaulted, err := cfg.withDefaults()
	if err != nil {
		recordErrorMetric(options.metrics, "new", err)
		return nil, err
	}
	if err := defaulted.Validate(); err != nil {
		recordErrorMetric(options.metrics, "new", err)
		return nil, err
	}

	conn, err := options.connector.Open(ctx, defaulted)
	if err != nil {
		wrapped := operationError(ErrorKindConnection, op, err)
		recordErrorMetric(options.metrics, "new", wrapped)
		return nil, wrapped
	}

	options.metrics.IncCounter(MetricClientCreatedTotal, map[string]string{"name": defaulted.Name})
	recordPoolMetrics(options.metrics, defaulted.Name, conn)

	return &Client{
		cfg:         defaulted,
		metrics:     options.metrics,
		tracer:      options.tracer,
		logger:      options.logger,
		retry:       options.retry,
		conn:        conn,
		initialized: true,
	}, nil
}

// Exec executes a ClickHouse statement with context cancellation and retry.
func (c *Client) Exec(ctx context.Context, query string, args ...any) error {
	const op = "clickhousex.Exec"
	if c == nil {
		return validationError(op, "client is nil", nil)
	}
	if strings.TrimSpace(query) == "" {
		err := validationError(op, "query is required", nil)
		recordErrorMetric(c.metrics, "exec", err)
		return err
	}

	conn, finish, err := c.beginOperation(ctx, op)
	if err != nil {
		recordErrorMetric(c.metrics, "exec", err)
		return err
	}
	defer finish()

	labels := c.operationLabels("exec", "", statementKind(query))
	ctx, span := c.tracer.StartSpan(ctx, "clickhousex.exec", labels)
	start := time.Now()
	var finalErr error
	defer func() {
		span.End(finalErr)
		c.observeOperation("exec", labels, start, finalErr)
		recordPoolMetrics(c.metrics, c.cfg.Name, conn)
	}()

	c.logger.Debug(ctx, "clickhousex exec started", labels)
	c.metrics.IncCounter(MetricClientRequestsTotal, labels)
	c.metrics.IncCounter(MetricClickhouseQueriesTotal, labels)
	c.metrics.SetGauge(MetricClientInflight, 1, labels)
	defer c.metrics.SetGauge(MetricClientInflight, 0, labels)

	finalErr = c.runWithRetry(ctx, op, "exec", func() error {
		if err := conn.Exec(ctx, query, args...); err != nil {
			return operationError(ErrorKindQuery, op, err)
		}
		return nil
	})
	if finalErr != nil {
		c.logger.Error(ctx, "clickhousex exec failed", labels)
		recordErrorMetric(c.metrics, "exec", finalErr)
		return finalErr
	}
	c.logger.Debug(ctx, "clickhousex exec finished", labels)
	return nil
}

// Query executes a ClickHouse query and returns driver-neutral rows.
func (c *Client) Query(ctx context.Context, query string, args ...any) (Rows, error) {
	const op = "clickhousex.Query"
	if c == nil {
		return nil, validationError(op, "client is nil", nil)
	}
	if strings.TrimSpace(query) == "" {
		err := validationError(op, "query is required", nil)
		recordErrorMetric(c.metrics, "query", err)
		return nil, err
	}

	conn, finish, err := c.beginOperation(ctx, op)
	if err != nil {
		recordErrorMetric(c.metrics, "query", err)
		return nil, err
	}
	defer finish()

	labels := c.operationLabels("query", "", statementKind(query))
	ctx, span := c.tracer.StartSpan(ctx, "clickhousex.query", labels)
	start := time.Now()
	var finalErr error
	defer func() {
		span.End(finalErr)
		c.observeOperation("query", labels, start, finalErr)
		recordPoolMetrics(c.metrics, c.cfg.Name, conn)
	}()

	c.logger.Debug(ctx, "clickhousex query started", labels)
	c.metrics.IncCounter(MetricClientRequestsTotal, labels)
	c.metrics.IncCounter(MetricClickhouseQueriesTotal, labels)
	c.metrics.SetGauge(MetricClientInflight, 1, labels)
	defer c.metrics.SetGauge(MetricClientInflight, 0, labels)

	var rows driverRows
	finalErr = c.runWithRetry(ctx, op, "query", func() error {
		result, err := conn.Query(ctx, query, args...)
		if err != nil {
			return operationError(ErrorKindQuery, op, err)
		}
		rows = result
		return nil
	})
	if finalErr != nil {
		c.logger.Error(ctx, "clickhousex query failed", labels)
		recordErrorMetric(c.metrics, "query", finalErr)
		return nil, finalErr
	}
	c.logger.Debug(ctx, "clickhousex query finished", labels)
	return &rowsWrapper{rows: rows}, nil
}

// InsertBatch writes rows using the native ClickHouse batch protocol.
func (c *Client) InsertBatch(ctx context.Context, table string, cols []string, rows [][]any) error {
	const op = "clickhousex.InsertBatch"
	if c == nil {
		return validationError(op, "client is nil", nil)
	}
	if len(cols) == 0 {
		err := NewError(ErrorKindEmptyColumns, op, "empty columns", false)
		recordErrorMetric(c.metrics, "insert_batch", err)
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	for i, row := range rows {
		if len(row) != len(cols) {
			err := NewError(ErrorKindColumnCountMismatch, op, "row column count must match columns", false)
			recordErrorMetric(c.metrics, "insert_batch", err)
			c.logger.Error(ctx, "clickhousex insert batch row width mismatch", map[string]string{
				"row":  strconvItoa(i),
				"want": strconvItoa(len(cols)),
				"got":  strconvItoa(len(row)),
			})
			return err
		}
	}

	query, err := buildInsertQuery(table, cols)
	if err != nil {
		recordErrorMetric(c.metrics, "insert_batch", err)
		return err
	}

	conn, finish, err := c.beginOperation(ctx, op)
	if err != nil {
		recordErrorMetric(c.metrics, "insert_batch", err)
		return err
	}
	defer finish()

	labels := c.operationLabels("insert_batch", table, "insert")
	ctx, span := c.tracer.StartSpan(ctx, "clickhousex.insert_batch", labels)
	start := time.Now()
	var finalErr error
	defer func() {
		span.End(finalErr)
		c.observeOperation("insert_batch", labels, start, finalErr)
		recordPoolMetrics(c.metrics, c.cfg.Name, conn)
	}()

	c.logger.Debug(ctx, "clickhousex insert batch started", labels)
	c.metrics.IncCounter(MetricClientRequestsTotal, labels)
	c.metrics.IncCounter(MetricClickhouseBatchInsertsTotal, labels)
	c.metrics.SetGauge(MetricClientInflight, 1, labels)
	defer c.metrics.SetGauge(MetricClientInflight, 0, labels)

	finalErr = c.runWithRetry(ctx, op, "insert_batch", func() error {
		batch, err := conn.PrepareBatch(ctx, query)
		if err != nil {
			return operationError(ErrorKindBatch, op, err)
		}
		for _, row := range rows {
			if err := batch.Append(row...); err != nil {
				_ = batch.Abort()
				return operationError(ErrorKindBatch, op, err)
			}
		}
		if err := batch.Send(); err != nil {
			_ = batch.Abort()
			return operationError(ErrorKindBatch, op, err)
		}
		return nil
	})
	if finalErr != nil {
		c.logger.Error(ctx, "clickhousex insert batch failed", labels)
		recordErrorMetric(c.metrics, "insert_batch", finalErr)
		return finalErr
	}
	c.metrics.ObserveHistogram(MetricClickhouseWriteRows, float64(len(rows)), labels)
	c.logger.Debug(ctx, "clickhousex insert batch finished", labels)
	return nil
}

// Ping checks connectivity to the ClickHouse server.
func (c *Client) Ping(ctx context.Context) error {
	const op = "clickhousex.Ping"
	if c == nil {
		return validationError(op, "client is nil", nil)
	}

	conn, finish, err := c.beginOperation(ctx, op)
	if err != nil {
		recordErrorMetric(c.metrics, "ping", err)
		return err
	}
	defer finish()

	if err := conn.Ping(ctx); err != nil {
		wrapped := operationError(ErrorKindConnection, op, err)
		recordErrorMetric(c.metrics, "ping", wrapped)
		return wrapped
	}
	recordPoolMetrics(c.metrics, c.cfg.Name, conn)
	return nil
}

// Close shuts down the client and waits for in-flight operations.
func (c *Client) Close() error {
	return c.closeWithContext(context.Background())
}

// CloseContext shuts down the client with a caller-controlled wait context.
func (c *Client) CloseContext(ctx context.Context) error {
	return c.closeWithContext(ctx)
}

func (c *Client) closeWithContext(ctx context.Context) error {
	const op = "clickhousex.Close"
	if c == nil {
		return validationError(op, "client is nil", nil)
	}
	if ctx == nil {
		err := validationError(op, "context is required", nil)
		recordErrorMetric(c.metrics, "close", err)
		return err
	}
	if err := ctx.Err(); err != nil {
		wrapped := contextError(op, err)
		recordErrorMetric(c.metrics, "close", wrapped)
		return wrapped
	}

	c.mu.Lock()
	if !c.initialized {
		c.mu.Unlock()
		err := validationError(op, "client is not initialized", nil)
		recordErrorMetric(c.metrics, "close", err)
		return err
	}
	if c.connClosed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	name := c.cfg.Name
	metrics := c.metrics
	conn := c.conn
	c.mu.Unlock()

	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		wrapped := contextError(op, ctx.Err())
		recordErrorMetric(metrics, "close", wrapped)
		return wrapped
	}

	if conn != nil {
		if err := conn.Close(); err != nil {
			wrapped := operationError(ErrorKindConnection, op, err)
			recordErrorMetric(metrics, "close", wrapped)
			return wrapped
		}
	}

	c.mu.Lock()
	c.connClosed = true
	c.mu.Unlock()

	if metrics != nil {
		metrics.IncCounter(MetricClientClosedTotal, map[string]string{"name": name})
	}
	return nil
}

func (c *Client) beginOperation(ctx context.Context, op string) (driverConn, func(), error) {
	if ctx == nil {
		return nil, nil, validationError(op, "context is required", nil)
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, contextError(op, err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		return nil, nil, validationError(op, "client is not initialized", nil)
	}
	if c.closed {
		return nil, nil, NewError(ErrorKindUnavailable, op, "client is closed", false)
	}
	if c.conn == nil {
		return nil, nil, NewError(ErrorKindUnavailable, op, "client connection is nil", false)
	}

	c.wg.Add(1)
	return c.conn, c.wg.Done, nil
}

func (c *Client) runWithRetry(ctx context.Context, op string, metricOp string, fn func() error) error {
	attempts := c.retry.MaxAttempts
	if attempts <= 0 {
		attempts = 1
	}

	var last error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return contextError(op, err)
		}

		last = fn()
		if last == nil {
			return nil
		}
		if !shouldRetry(last) || attempt == attempts {
			return last
		}

		c.metrics.IncCounter(MetricClientRetriesTotal, map[string]string{
			"op":      metricOp,
			"attempt": strconvItoa(attempt),
		})

		delay := c.retryDelay(attempt)
		if delay <= 0 {
			continue
		}
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return contextError(op, ctx.Err())
		}
	}
	return last
}

func (c *Client) retryDelay(attempt int) time.Duration {
	delay := c.retry.BaseDelay
	if delay <= 0 {
		return 0
	}
	for i := 1; i < attempt; i++ {
		delay *= 2
		if c.retry.MaxDelay > 0 && delay > c.retry.MaxDelay {
			return c.retry.MaxDelay
		}
	}
	if c.retry.MaxDelay > 0 && delay > c.retry.MaxDelay {
		return c.retry.MaxDelay
	}
	return delay
}

func shouldRetry(err error) bool {
	if err == nil {
		return false
	}
	if typed, ok := err.(*Error); ok {
		return typed.Retryable
	}
	return isRetryableError(err)
}

func (c *Client) operationLabels(op string, table string, query string) map[string]string {
	labels := map[string]string{
		"name": c.cfg.Name,
		"op":   op,
	}
	if table != "" {
		labels["table"] = table
	}
	if query != "" {
		labels["query"] = query
	}
	return labels
}

func (c *Client) observeOperation(op string, labels map[string]string, start time.Time, err error) {
	duration := time.Since(start).Seconds()
	c.metrics.ObserveHistogram(MetricClientRequestDurationSeconds, duration, labels)
	if op == "insert_batch" {
		c.metrics.ObserveHistogram(MetricClickhouseWriteDuration, duration, labels)
	} else {
		c.metrics.ObserveHistogram(MetricClickhouseQueryDuration, duration, labels)
	}
}

func buildInsertQuery(table string, cols []string) (string, error) {
	quotedTable, err := quoteTableIdentifier(table)
	if err != nil {
		return "", err
	}
	quotedCols := make([]string, 0, len(cols))
	for _, col := range cols {
		quoted, err := quoteIdentifier(strings.TrimSpace(col))
		if err != nil {
			return "", err
		}
		quotedCols = append(quotedCols, quoted)
	}
	return "INSERT INTO " + quotedTable + " (" + strings.Join(quotedCols, ", ") + ") VALUES", nil
}

func quoteTableIdentifier(table string) (string, error) {
	const op = "clickhousex.InsertBatch"
	trimmed := strings.TrimSpace(table)
	if trimmed == "" {
		return "", validationError(op, "table is required", nil)
	}
	parts := strings.Split(trimmed, ".")
	quoted := make([]string, 0, len(parts))
	for _, part := range parts {
		q, err := quoteIdentifier(strings.TrimSpace(part))
		if err != nil {
			return "", err
		}
		quoted = append(quoted, q)
	}
	return strings.Join(quoted, "."), nil
}

func quoteIdentifier(identifier string) (string, error) {
	const op = "clickhousex.InsertBatch"
	if !identifierPattern.MatchString(identifier) {
		return "", validationError(op, "identifier is invalid", nil)
	}
	return "`" + identifier + "`", nil
}

func statementKind(query string) string {
	fields := strings.Fields(query)
	if len(fields) == 0 {
		return ""
	}
	return strings.ToLower(fields[0])
}

func recordPoolMetrics(metrics Metrics, name string, conn driverConn) {
	if metrics == nil || conn == nil {
		return
	}
	stats := conn.Stats()
	labels := map[string]string{"name": name}
	metrics.SetGauge(MetricClickhousePoolActive, float64(stats.Open), labels)
	metrics.SetGauge(MetricClickhousePoolIdle, float64(stats.Idle), labels)
	exhausted := 0.0
	if stats.MaxOpenConns > 0 && stats.Open >= stats.MaxOpenConns && stats.Idle == 0 {
		exhausted = 1
	}
	metrics.SetGauge(MetricClickhousePoolExhausted, exhausted, labels)
}

func recordErrorMetric(metrics Metrics, op string, err error) {
	if metrics == nil {
		return
	}
	metrics.IncCounter(MetricClientErrorsTotal, map[string]string{
		"op":   op,
		"kind": string(errorKind(err)),
	})
}

func strconvItoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
