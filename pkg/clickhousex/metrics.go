package clickhousex

const (
	MetricClientCreatedTotal           = "client_created_total"
	MetricClientClosedTotal            = "client_closed_total"
	MetricClientErrorsTotal            = "client_errors_total"
	MetricClientHealthStatus           = "client_health_status"
	MetricClientHealthLatencyMS        = "client_health_latency_ms"
	MetricClientRequestsTotal          = "client_requests_total"
	MetricClientRequestDurationSeconds = "client_request_duration_seconds"
	MetricClientRetriesTotal           = "client_retries_total"
	MetricClientInflight               = "client_inflight"
	MetricClickhouseQueriesTotal       = "clickhousex_queries_total"
	MetricClickhouseBatchInsertsTotal  = "clickhousex_batch_inserts_total"
	MetricClickhouseQueryDuration      = "clickhousex.query.duration"
	MetricClickhouseWriteDuration      = "clickhousex.write.duration"
	MetricClickhouseWriteRows          = "clickhousex.write.rows"
	MetricClickhouseWriteBytes         = "clickhousex.write.bytes"
	MetricClickhousePoolActive         = "clickhousex.pool.active"
	MetricClickhousePoolIdle           = "clickhousex.pool.idle"
	MetricClickhousePoolExhausted      = "clickhousex.pool.exhausted"
)

// Metrics defines the hook interface for observability instrumentation.
type Metrics interface {
	IncCounter(name string, labels map[string]string)
	ObserveHistogram(name string, value float64, labels map[string]string)
	SetGauge(name string, value float64, labels map[string]string)
}

// NoopMetrics is a Metrics implementation that discards all observations.
type NoopMetrics struct{}

func (NoopMetrics) IncCounter(name string, labels map[string]string) {
	_ = name
	_ = labels
}

func (NoopMetrics) ObserveHistogram(name string, value float64, labels map[string]string) {
	_ = name
	_ = value
	_ = labels
}

func (NoopMetrics) SetGauge(name string, value float64, labels map[string]string) {
	_ = name
	_ = value
	_ = labels
}
