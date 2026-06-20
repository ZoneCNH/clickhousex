package clickhousex

import obs "github.com/ZoneCNH/observex/pkg/observex"

// MetricClient* constants are aliases to observex shared constants so all
// foundation x-modules report identical cross-component metric names.
const (
	MetricClientCreatedTotal           = obs.MetricClientCreatedTotal
	MetricClientClosedTotal            = obs.MetricClientClosedTotal
	MetricClientErrorsTotal            = obs.MetricClientErrorsTotal
	MetricClientHealthStatus           = obs.MetricClientHealthStatus
	MetricClientHealthLatencyMS        = obs.MetricClientHealthLatencyMS
	MetricClientRequestsTotal          = obs.MetricClientRequestsTotal
	MetricClientRequestDurationSeconds = obs.MetricClientRequestDurationSeconds
	MetricClientRetriesTotal           = obs.MetricClientRetriesTotal
	MetricClientInflight               = obs.MetricClientInflight

	// clickhousex-specific metric names.
	MetricClickhouseQueriesTotal      = "clickhousex_queries_total"
	MetricClickhouseBatchInsertsTotal = "clickhousex_batch_inserts_total"
	MetricClickhouseQueryDuration     = "clickhousex.query.duration"
	MetricClickhouseWriteDuration     = "clickhousex.write.duration"
	MetricClickhouseWriteRows         = "clickhousex.write.rows"
	MetricClickhouseWriteBytes        = "clickhousex.write.bytes"
	MetricClickhousePoolActive        = "clickhousex.pool.active"
	MetricClickhousePoolIdle          = "clickhousex.pool.idle"
	MetricClickhousePoolExhausted     = "clickhousex.pool.exhausted"
)

// Metrics defines the hook interface for observability instrumentation.
// It is a 3-method subset of observex.Metrics; any observex.Metrics
// implementation satisfies this interface.
type Metrics interface {
	IncCounter(name string, labels map[string]string)
	ObserveHistogram(name string, value float64, labels map[string]string)
	SetGauge(name string, value float64, labels map[string]string)
}

// NoopMetrics is an alias for observex.NoopMetrics; it satisfies Metrics and
// discards all observations. Aliasing removes the duplicate no-op body.
type NoopMetrics = obs.NoopMetrics
