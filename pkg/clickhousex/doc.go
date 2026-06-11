// Package clickhousex provides the public contract surface for a ClickHouse L2 adapter.
//
// The package keeps ClickHouse-facing APIs driver-neutral so callers can depend on
// Config, HealthCheck, Error model, Metrics hooks, and Client lifecycle without
// importing a concrete ClickHouse client implementation.
//
// This package must not depend on github.com/bytechainx/x.go, github.com/ZoneCNH/x.go,
// or any x.go internal package.
package clickhousex
