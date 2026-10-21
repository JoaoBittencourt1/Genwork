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
	cd core && go test ./...

test-cli:
	cd cli && go test ./...

test-web:
	cd web && npm test

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
