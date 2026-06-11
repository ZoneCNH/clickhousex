package clickhousex

import (
	"context"
	"sync"
)

// Client manages the lifecycle of a ClickHouse connection.
type Client struct {
	cfg         Config
	metrics     Metrics
	mu          sync.Mutex
	initialized bool
	closed      bool
}

// New creates a new Client. It validates the config and records a creation metric.
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
	if err := cfg.Validate(); err != nil {
		recordErrorMetric(options.metrics, "new", err)
		return nil, err
	}

	options.metrics.IncCounter(MetricClientCreatedTotal, map[string]string{"name": cfg.Name})
	return &Client{cfg: cfg, metrics: options.metrics, initialized: true}, nil
}

// Close shuts down the client and records a closure metric.
func (c *Client) Close(ctx context.Context) error {
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
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	name := c.cfg.Name
	metrics := c.metrics
	c.mu.Unlock()

	if metrics != nil {
		metrics.IncCounter(MetricClientClosedTotal, map[string]string{"name": name})
	}
	return nil
}

// Ping checks connectivity to the ClickHouse server.
func (c *Client) Ping(ctx context.Context) error {
	const op = "clickhousex.Ping"
	if c == nil {
		return validationError(op, "client is nil", nil)
	}
	if ctx == nil {
		err := validationError(op, "context is required", nil)
		recordErrorMetric(c.metrics, "ping", err)
		return err
	}
	if err := ctx.Err(); err != nil {
		wrapped := contextError(op, err)
		recordErrorMetric(c.metrics, "ping", wrapped)
		return wrapped
	}

	c.mu.Lock()
	if !c.initialized {
		c.mu.Unlock()
		err := validationError(op, "client is not initialized", nil)
		recordErrorMetric(c.metrics, "ping", err)
		return err
	}
	if c.closed {
		c.mu.Unlock()
		err := NewError(ErrorKindUnavailable, op, "client is closed", false)
		recordErrorMetric(c.metrics, "ping", err)
		return err
	}
	c.mu.Unlock()

	return nil
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
