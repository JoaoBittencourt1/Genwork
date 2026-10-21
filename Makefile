# Minimum statement/line coverage per component (constitution rule 70).
MIN_COVERAGE := 75

# usage: $(call check_go_coverage,<module dir>)
define check_go_coverage
	cd $(1) && go tool cover -func=coverage.out | awk -v min=$(MIN_COVERAGE) -v name=$(1) \
		'/^total:/ { sub("%", "", $$NF); printf "%s coverage: %.1f%% (minimum %d%%)\n", name, $$NF, min; if ($$NF + 0 < min) { print name " is below the minimum coverage"; exit 1 } }'
endef

.PHONY: lint lint-core lint-cli lint-web lint-fix test test-core test-cli test-web build build-core build-cli build-web \
	install-cli

lint: lint-core lint-cli lint-web

lint-core:
	@unformatted="$$(cd core && gofmt -l .)"; if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi
	cd core && go vet ./...

lint-cli:
	@unformatted="$$(cd cli && gofmt -l .)"; if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi
	cd cli && go vet ./...

lint-web:
	cd web && npm run lint && npm run typecheck

lint-fix:
	cd core && gofmt -w .
	cd cli && gofmt -w .
	cd web && npm run lint:fix

test: test-core test-cli test-web

# Needs TEST_DATABASE_URL, e.g. postgres://genwork:change-me@localhost:5484/genwork?sslmode=disable with `genwork db` running.
test-core:
	cd core && go test -coverpkg=./... -coverprofile=coverage.out ./...
	$(call check_go_coverage,core)

test-cli:
	cd cli && go test -coverpkg=./... -coverprofile=coverage.out ./...
	$(call check_go_coverage,cli)

test-web:
	cd web && npm run test:coverage -- --coverage.thresholds.lines=$(MIN_COVERAGE) --coverage.thresholds.statements=$(MIN_COVERAGE)

build: build-core build-cli build-web

build-core:
	cd core && go build -o bin/ ./cmd/...

build-cli:
	cd cli && go build -o bin/genwork ./cmd/genwork

build-web:
	cd web && npm run build

# Installs `genwork` into $(go env GOPATH)/bin; that directory must be on PATH.
install-cli:
	cd cli && go install ./cmd/genwork
