.PHONY: build test test-unit test-race test-coverage integration-test soak-test benchmark test-bench profile lint vet fmt l2-plan test-contract test-integration test-chaos test-adoption test-arch test-security evidence release-check factory-check

PROFILE_DIR ?= /tmp/clickhousex-profile
FORBIDDEN_L2_DEPS := (configx|redisx|postgresx|kafkax|natsx|taosx|ossx)
SECRET_PATTERN := (AKIA[0-9A-Z]{16}|BEGIN (RSA|EC|OPENSSH) PRIVATE KEY|xox[baprs]-|ghp_[A-Za-z0-9_]{30,})

build:
	GOWORK=off go build ./...

test:
	$(MAKE) test-race

test-unit:
	GOWORK=off go test -count=1 ./...

test-race:
	GOWORK=off go test -race -count=1 ./...

test-coverage:
	GOWORK=off go test -count=1 ./... -covermode=atomic -coverpkg=./... -coverprofile=coverage.out
	GOWORK=off go tool cover -func=coverage.out | tee coverage.txt
	@awk '/^total:/ { if ($$3 != "100.0%") { print "coverage " $$3 " is below required 100.0%"; exit 1 } found=1 } END { if (!found) { print "coverage total not found"; exit 1 } }' coverage.txt

integration-test:
	CLICKHOUSEX_RUN_INTEGRATION=1 GOWORK=off go test -count=1 -run TestClickHouseLiveIntegration -v ./pkg/clickhousex

soak-test:
	CLICKHOUSEX_RUN_INTEGRATION=1 CLICKHOUSEX_RUN_SOAK=1 GOWORK=off go test -count=1 -run TestClickHouseLiveSoak -v ./pkg/clickhousex

benchmark:
	GOWORK=off go test -run '^$$' -bench . -benchmem ./pkg/clickhousex

test-bench:
	$(MAKE) benchmark

profile:
	mkdir -p $(PROFILE_DIR)
	GOWORK=off go test -run '^$$' -bench . -benchmem -cpuprofile $(PROFILE_DIR)/cpu.pprof -memprofile $(PROFILE_DIR)/mem.pprof ./pkg/clickhousex

lint:
	golangci-lint run ./...

vet:
	GOWORK=off go vet ./...

fmt:
	gofmt -s -w .

l2-plan:
	sed -n '1,220p' docs/09_clickhousex_execution_plan.md

test-contract:
	GOWORK=off go test -count=1 -run 'Test(ConfigValidateAndSanitize|NewUsesConnectorAndMetrics|CloseAndPing|ExecRetriesRetryableErrors|QueryReturnsRowsAndValidatesScan|InsertBatchValidationAndSend|InsertBatchMapsTableNotFound|HealthUsesPing|ErrorsMetricsAndOptions)$$' ./pkg/clickhousex

test-integration:
	$(MAKE) integration-test

test-chaos:
	GOWORK=off go test -count=1 -run 'Test(ClientFailureBranches|RetryAndHelperBranches|HealthBranches|ErrorsMetricsAndOptions|ExecRetriesRetryableErrors)$$' ./pkg/clickhousex

test-adoption:
	GOWORK=off go test -count=1 -run 'Test(NewUsesConnectorAndMetrics|QueryReturnsRowsAndValidatesScan|InsertBatchValidationAndSend|HealthUsesPing)$$' ./pkg/clickhousex

test-arch:
	@GOWORK=off go list -deps ./... > /tmp/clickhousex-deps.txt
	@if grep -E 'github\.com/ZoneCNH/$(FORBIDDEN_L2_DEPS)($$|/)' /tmp/clickhousex-deps.txt; then \
		echo "forbidden L2 dependency found"; \
		exit 1; \
	fi

test-security:
	@if command -v gitleaks >/dev/null 2>&1; then \
		gitleaks detect --redact --source .; \
	else \
		if command -v rg >/dev/null 2>&1; then \
			rg -n '$(SECRET_PATTERN)' .; \
			status=$$?; \
		else \
			grep -RInE --exclude-dir=.git --exclude-dir=.worktree '$(SECRET_PATTERN)' .; \
			status=$$?; \
		fi; \
		if [ "$$status" -eq 0 ]; then \
			echo "potential secret pattern found"; \
			exit 1; \
		elif [ "$$status" -gt 1 ]; then \
			echo "secret scan failed"; \
			exit "$$status"; \
		fi; \
	fi

evidence:
	sh scripts/release-check.sh release .

release-check: build test-unit test-race test-coverage vet test-contract test-chaos test-bench test-adoption test-arch test-security evidence

factory-check:
	sh scripts/release-check.sh factory .
