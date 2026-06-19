package clickhousex

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestClickHouseLiveIntegration(t *testing.T) {
	if os.Getenv("CLICKHOUSEX_RUN_INTEGRATION") != "1" {
		t.Skip("set CLICKHOUSEX_RUN_INTEGRATION=1 to run live ClickHouse integration test")
	}
	if testing.Short() {
		t.Skip("skipping live integration test in short mode")
	}

	cfg := liveIntegrationConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := New(ctx, cfg, WithRetryConfig(RetryConfig{
		MaxAttempts: 1,
		BaseDelay:   0,
		MaxDelay:    0,
	}))
	if err != nil {
		t.Fatalf("new live client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("close live client: %v", err)
		}
	}()

	if err := client.Ping(ctx); err != nil {
		t.Fatalf("ping live ClickHouse: %v", err)
	}

	health := client.HealthCheck(ctx)
	if health.Status != HealthHealthy || !health.Ready || !health.Live {
		t.Fatalf("unexpected health: %+v", health)
	}

	table := liveIntegrationTableName()
	if err := client.Exec(ctx, fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id UInt64, name String) ENGINE = Memory", table)); err != nil {
		t.Fatalf("create live table: %v", err)
	}
	defer dropLiveIntegrationTable(t, client, table)

	rowsToInsert := [][]any{
		{uint64(1), "alpha"},
		{uint64(2), "beta"},
		{uint64(3), "gamma"},
	}
	if err := client.InsertBatch(ctx, table, []string{"id", "name"}, rowsToInsert); err != nil {
		t.Fatalf("insert live rows: %v", err)
	}

	rows, err := client.Query(ctx, fmt.Sprintf("SELECT toUInt64(count()) AS c, toUInt64(sum(id)) AS s FROM %s", table))
	if err != nil {
		t.Fatalf("query live rows: %v", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Errorf("close live rows: %v", err)
		}
	}()

	columnTypes := rows.ColumnTypes()
	if len(columnTypes) != 2 {
		t.Fatalf("column types length = %d, want 2", len(columnTypes))
	}
	if columnTypes[0].Name != "c" || columnTypes[1].Name != "s" {
		t.Fatalf("column names = %q/%q, want c/s", columnTypes[0].Name, columnTypes[1].Name)
	}

	if !rows.Next() {
		t.Fatalf("expected one aggregate row, err=%v", rows.Err())
	}
	var count, sum uint64
	if err := rows.Scan(&count, &sum); err != nil {
		t.Fatalf("scan aggregate row: %v", err)
	}
	if count != 3 || sum != 6 {
		t.Fatalf("aggregate row = count %d sum %d, want count 3 sum 6", count, sum)
	}
	if rows.Next() {
		t.Fatal("expected one aggregate row only")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows err: %v", err)
	}
}

func TestClickHouseLiveSoak(t *testing.T) {
	if os.Getenv("CLICKHOUSEX_RUN_INTEGRATION") != "1" {
		t.Skip("set CLICKHOUSEX_RUN_INTEGRATION=1 to run live ClickHouse soak test")
	}
	if os.Getenv("CLICKHOUSEX_RUN_SOAK") != "1" {
		t.Skip("set CLICKHOUSEX_RUN_SOAK=1 to run live ClickHouse soak test")
	}
	if testing.Short() {
		t.Skip("skipping live soak test in short mode")
	}

	duration := durationFromEnvOrDefault(t, time.Minute, "CLICKHOUSEX_SOAK_DURATION")
	interval := durationFromEnvOrDefault(t, 100*time.Millisecond, "CLICKHOUSEX_SOAK_INTERVAL")
	if duration <= 0 {
		t.Fatal("CLICKHOUSEX_SOAK_DURATION must be positive")
	}
	if interval < 0 {
		t.Fatal("CLICKHOUSEX_SOAK_INTERVAL must not be negative")
	}

	cfg := liveIntegrationConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), duration+30*time.Second)
	defer cancel()

	client, err := New(ctx, cfg, WithRetryConfig(RetryConfig{
		MaxAttempts: 1,
		BaseDelay:   0,
		MaxDelay:    0,
	}))
	if err != nil {
		t.Fatalf("new live soak client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("close live soak client: %v", err)
		}
	}()

	if err := client.Ping(ctx); err != nil {
		t.Fatalf("ping live ClickHouse for soak: %v", err)
	}

	table := liveIntegrationTableName()
	if err := client.Exec(ctx, fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id UInt64, name String) ENGINE = Memory", table)); err != nil {
		t.Fatalf("create live soak table: %v", err)
	}
	defer dropLiveIntegrationTable(t, client, table)

	deadline := time.Now().Add(duration)
	var iterations uint64
	for time.Now().Before(deadline) {
		iterations++
		name := fmt.Sprintf("soak-%d", iterations)
		if err := client.InsertBatch(ctx, table, []string{"id", "name"}, [][]any{{iterations, name}}); err != nil {
			t.Fatalf("insert live soak row %d: %v", iterations, err)
		}

		count := queryLiveSoakCount(t, client, table)
		if count != iterations {
			t.Fatalf("live soak row count = %d, want %d", count, iterations)
		}

		gotName := queryLiveSoakName(t, client, table, iterations)
		if gotName != name {
			t.Fatalf("live soak name for id %d = %q, want %q", iterations, gotName, name)
		}

		sleepFor := interval
		if remaining := time.Until(deadline); remaining < sleepFor {
			sleepFor = remaining
		}
		if err := sleepLiveSoakInterval(ctx, sleepFor); err != nil {
			t.Fatalf("sleep live soak interval: %v", err)
		}
	}

	if iterations == 0 {
		t.Fatal("live soak completed zero iterations")
	}
	t.Logf("completed live ClickHouse soak: duration=%s iterations=%d interval=%s", duration, iterations, interval)
}

func queryLiveSoakCount(t *testing.T, client *Client, table string) uint64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := client.Query(ctx, fmt.Sprintf("SELECT toUInt64(count()) AS c FROM %s", table))
	if err != nil {
		t.Fatalf("query live soak count: %v", err)
	}
	defer closeLiveRows(t, rows, "count")

	if !rows.Next() {
		t.Fatalf("expected live soak count row, err=%v", rows.Err())
	}
	var count uint64
	if err := rows.Scan(&count); err != nil {
		t.Fatalf("scan live soak count: %v", err)
	}
	if rows.Next() {
		t.Fatal("expected one live soak count row only")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("live soak count rows err: %v", err)
	}
	return count
}

func queryLiveSoakName(t *testing.T, client *Client, table string, id uint64) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := client.Query(ctx, fmt.Sprintf("SELECT name FROM %s WHERE id = %d", table, id))
	if err != nil {
		t.Fatalf("query live soak name: %v", err)
	}
	defer closeLiveRows(t, rows, "name")

	if !rows.Next() {
		t.Fatalf("expected live soak name row, err=%v", rows.Err())
	}
	var name string
	if err := rows.Scan(&name); err != nil {
		t.Fatalf("scan live soak name: %v", err)
	}
	if rows.Next() {
		t.Fatal("expected one live soak name row only")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("live soak name rows err: %v", err)
	}
	return name
}

func closeLiveRows(t *testing.T, rows Rows, label string) {
	t.Helper()
	if err := rows.Close(); err != nil {
		t.Errorf("close live soak %s rows: %v", label, err)
	}
}

func sleepLiveSoakInterval(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		return nil
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func liveIntegrationConfig(t *testing.T) Config {
	t.Helper()
	if dsn := firstNonEmptyEnv("CLICKHOUSEX_TEST_DSN", "FOUNDATIONX_CLICKHOUSEX_DSN"); dsn != "" {
		return Config{
			Name:         "clickhousex-live-integration",
			DSN:          dsn,
			MaxOpenConns: 2,
			MaxIdleConns: 1,
			Timeout:      10 * time.Second,
		}
	}
	return Config{
		Name:         "clickhousex-live-integration",
		Host:         firstNonEmptyEnvOrDefault("127.0.0.1", "CLICKHOUSEX_TEST_HOST", "FOUNDATIONX_CLICKHOUSEX_HOST"),
		Port:         intFromEnvOrDefault(t, DefaultPort, "CLICKHOUSEX_TEST_PORT", "FOUNDATIONX_CLICKHOUSEX_PORT"),
		Database:     firstNonEmptyEnvOrDefault("default", "CLICKHOUSEX_TEST_DATABASE", "FOUNDATIONX_CLICKHOUSEX_DATABASE"),
		Username:     firstNonEmptyEnvOrDefault("default", "CLICKHOUSEX_TEST_USERNAME", "FOUNDATIONX_CLICKHOUSEX_USERNAME"),
		Password:     firstNonEmptyEnv("CLICKHOUSEX_TEST_PASSWORD", "FOUNDATIONX_CLICKHOUSEX_PASSWORD"),
		MaxOpenConns: 2,
		MaxIdleConns: 1,
		Timeout:      10 * time.Second,
	}
}

func liveIntegrationTableName() string {
	return fmt.Sprintf("clickhousex_it_%d", time.Now().UnixNano())
}

func dropLiveIntegrationTable(t *testing.T, client *Client, table string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", table)); err != nil {
		t.Errorf("drop live table %s: %v", table, err)
	}
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyEnvOrDefault(defaultValue string, keys ...string) string {
	if value := firstNonEmptyEnv(keys...); value != "" {
		return value
	}
	return defaultValue
}

func intFromEnvOrDefault(t *testing.T, defaultValue int, keys ...string) int {
	t.Helper()
	raw := firstNonEmptyEnv(keys...)
	if raw == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("invalid integer environment value for %v", keys)
	}
	return value
}

func durationFromEnvOrDefault(t *testing.T, defaultValue time.Duration, keys ...string) time.Duration {
	t.Helper()
	raw := firstNonEmptyEnv(keys...)
	if raw == "" {
		return defaultValue
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		t.Fatalf("invalid duration environment value for %v", keys)
	}
	return value
}
