.PHONY: build test integration-test soak-test benchmark profile lint vet fmt

PROFILE_DIR ?= /tmp/clickhousex-profile

build:
	go build ./...

test:
	go test -race -count=1 ./...

integration-test:
	CLICKHOUSEX_RUN_INTEGRATION=1 GOWORK=off go test -count=1 -run TestClickHouseLiveIntegration -v ./pkg/clickhousex

soak-test:
	CLICKHOUSEX_RUN_INTEGRATION=1 CLICKHOUSEX_RUN_SOAK=1 GOWORK=off go test -count=1 -run TestClickHouseLiveSoak -v ./pkg/clickhousex

benchmark:
	GOWORK=off go test -run '^$$' -bench . -benchmem ./pkg/clickhousex

profile:
	mkdir -p $(PROFILE_DIR)
	GOWORK=off go test -run '^$$' -bench . -benchmem -cpuprofile $(PROFILE_DIR)/cpu.pprof -memprofile $(PROFILE_DIR)/mem.pprof ./pkg/clickhousex

lint:
	golangci-lint run ./...

vet:
	go vet ./...

fmt:
	gofmt -s -w .
