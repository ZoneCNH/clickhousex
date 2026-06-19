package clickhousex

import (
	"context"
	"time"
)

// HealthStatusValue represents the health state of the client.
type HealthStatusValue string

const (
	HealthHealthy   HealthStatusValue = "healthy"
	HealthDegraded  HealthStatusValue = "degraded"
	HealthUnhealthy HealthStatusValue = "unhealthy"
)

// HealthStatus describes the result of a health check.
type HealthStatus struct {
	Name      string            `json:"name"`
	Status    HealthStatusValue `json:"status"`
	Ready     bool              `json:"ready"`
	Live      bool              `json:"live"`
	Message   string            `json:"message,omitempty"`
	CheckedAt time.Time         `json:"checked_at"`
	LatencyMs int64             `json:"latency_ms"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Health returns a best-effort health snapshot using the configured timeout.
func (c *Client) Health() HealthStatus {
	timeout := DefaultTimeout
	if c != nil {
		c.mu.Lock()
		if c.cfg.Timeout > 0 {
			timeout = c.cfg.Timeout
		}
		c.mu.Unlock()
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return c.HealthCheck(ctx)
}

// HealthCheck performs a health probe against the client.
func (c *Client) HealthCheck(ctx context.Context) HealthStatus {
	start := time.Now()
	name := "clickhousex"
	var metrics Metrics
	initialized := false
	closed := true
	var conn driverConn

	if c != nil {
		c.mu.Lock()
		name = c.cfg.Name
		metrics = c.metrics
		initialized = c.initialized
		closed = c.closed
		conn = c.conn
		c.mu.Unlock()
		if name == "" {
			name = "clickhousex"
		}
	}

	status := func(value HealthStatusValue, ready bool, live bool, message string) HealthStatus {
		result := HealthStatus{
			Name:      name,
			Status:    value,
			Ready:     ready,
			Live:      live,
			Message:   message,
			CheckedAt: time.Now(),
			LatencyMs: time.Since(start).Milliseconds(),
		}
		recordHealthMetric(metrics, result)
		return result
	}

	if ctx == nil {
		return status(HealthUnhealthy, false, false, "context is required")
	}
	if err := ctx.Err(); err != nil {
		return status(HealthUnhealthy, false, false, err.Error())
	}
	if !initialized {
		return status(HealthUnhealthy, false, false, "client is not initialized")
	}
	if closed {
		return status(HealthUnhealthy, false, false, "client is closed")
	}
	if conn == nil {
		return status(HealthUnhealthy, false, false, "client connection is nil")
	}
	if err := conn.Ping(ctx); err != nil {
		return status(HealthUnhealthy, false, true, err.Error())
	}
	return status(HealthHealthy, true, true, "ok")
}

func recordHealthMetric(metrics Metrics, status HealthStatus) {
	if metrics == nil {
		return
	}
	labels := map[string]string{
		"name":   status.Name,
		"status": string(status.Status),
	}
	metrics.SetGauge(MetricClientHealthStatus, healthGaugeValue(status.Status), labels)
	metrics.ObserveHistogram(MetricClientHealthLatencyMS, float64(status.LatencyMs), labels)
}

func healthGaugeValue(status HealthStatusValue) float64 {
	if status == HealthHealthy {
		return 1
	}
	return 0
}
