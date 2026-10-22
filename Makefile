# Minimum statement/line coverage per component (constitution rule 70).
MIN_COVERAGE := 75

# The platform version every component ships under; VERSION is its only owner (rule 50).
VERSION := $(strip $(file < VERSION))
ifeq ($(VERSION),)
$(error VERSION is missing or empty)
endif
MODULE_PATH := github.com/JoaoBittencourt1/Genwork
# usage: $(call version_ldflags,<package path of the Version variable>)
version_ldflags = -ldflags "-X $(MODULE_PATH)/$(1).Version=$(VERSION)"

# usage: $(call check_go_coverage,<module dir>)
define check_go_coverage
	cd $(1) && go tool cover -func=coverage.out | awk -v min=$(MIN_COVERAGE) -v name=$(1) \
		'/^total:/ { sub("%", "", $$NF); printf "%s coverage: %.1f%% (minimum %d%%)\n", name, $$NF, min; if ($$NF + 0 < min) { print name " is below the minimum coverage"; exit 1 } }'
endef

.PHONY: lint lint-core lint-sdk lint-gentasks lint-genmonitor lint-cli lint-mcp lint-web lint-fix test test-core test-sdk test-gentasks test-genmonitor test-cli test-mcp test-web \
	build build-core build-gentasks build-genmonitor build-cli build-mcp build-web install-cli install-mcp \
	check-release-tag release-images release-artifacts

lint: lint-core lint-sdk lint-gentasks lint-genmonitor lint-cli lint-mcp lint-web

lint-core:
	@unformatted="$$(cd core && gofmt -l .)"; if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi
	cd core && go vet ./...

lint-sdk:
	@unformatted="$$(cd sdk/go && gofmt -l .)"; if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi
	cd sdk/go && go vet ./...

lint-gentasks:
	@unformatted="$$(cd modules/gentasks && gofmt -l .)"; if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi
	cd modules/gentasks && go vet ./...

lint-genmonitor:
	@unformatted="$$(cd modules/genmonitor && gofmt -l .)"; if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi
	cd modules/genmonitor && go vet ./...

lint-cli:
	@unformatted="$$(cd cli && gofmt -l .)"; if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi
	cd cli && go vet ./...

lint-mcp:
	@unformatted="$$(cd mcp && gofmt -l .)"; if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi
	cd mcp && go vet ./...

lint-web:
	cd web && npm audit --audit-level=high && npm run lint && npm run typecheck

lint-fix:
	cd core && gofmt -w .
	cd sdk/go && gofmt -w .
	cd modules/gentasks && gofmt -w .
	cd modules/genmonitor && gofmt -w .
	cd cli && gofmt -w .
	cd mcp && gofmt -w .
	cd web && npm run lint:fix

test: test-core test-sdk test-gentasks test-genmonitor test-cli test-mcp test-web

# Needs TEST_DATABASE_URL, e.g. postgres://genwork:change-me@localhost:5484/genwork?sslmode=disable with `genwork db` running.
test-core:
	cd core && go test -coverpkg=./... -coverprofile=coverage.out ./...
	$(call check_go_coverage,core)

test-sdk:
	cd sdk/go && go test -coverpkg=./... -coverprofile=coverage.out ./...
	$(call check_go_coverage,sdk/go)

# Needs TEST_DATABASE_URL, like test-core.
test-gentasks:
	cd modules/gentasks && go test -coverpkg=./... -coverprofile=coverage.out ./...
	$(call check_go_coverage,modules/gentasks)

test-genmonitor:
	cd modules/genmonitor && go test -coverpkg=./... -coverprofile=coverage.out ./...
	$(call check_go_coverage,modules/genmonitor)

test-cli:
	cd cli && go test -coverpkg=./... -coverprofile=coverage.out ./...
	$(call check_go_coverage,cli)

test-mcp:
	cd mcp && go test -coverpkg=./... -coverprofile=coverage.out ./...
	$(call check_go_coverage,mcp)

test-web:
	cd web && npm run test:coverage -- --coverage.thresholds.lines=$(MIN_COVERAGE) --coverage.thresholds.statements=$(MIN_COVERAGE)

build: build-core build-gentasks build-genmonitor build-cli build-mcp build-web

build-core:
	cd core && go build $(call version_ldflags,core/internal/command) -o bin/ ./cmd/...

build-gentasks:
	cd modules/gentasks && go build $(call version_ldflags,modules/gentasks/internal/command) -o bin/ ./cmd/...

build-genmonitor:
	cd modules/genmonitor && go build $(call version_ldflags,modules/genmonitor/internal/command) -o bin/ ./cmd/...

build-cli:
	cd cli && go build $(call version_ldflags,cli/internal/commands) -o bin/genwork ./cmd/genwork

build-mcp:
	cd mcp && go build $(call version_ldflags,mcp/internal/command) -o bin/genwork-mcp ./cmd/genwork-mcp

build-web:
	cd web && npm run build

# Installs `genwork` into $(go env GOPATH)/bin; that directory must be on PATH.
install-cli:
	cd cli && go install $(call version_ldflags,cli/internal/commands) ./cmd/genwork

# Installs `genwork-mcp`, the MCP server assistants launch, into $(go env GOPATH)/bin.
install-mcp:
	cd mcp && go install $(call version_ldflags,mcp/internal/command) ./cmd/genwork-mcp

# --- Releases (design §4). The release workflow runs only these targets (rule 75). ---

RELEASE_DIST := $(CURDIR)/dist
# Every module with a Dockerfile is released, so a new module needs no change here.
RELEASE_MODULES := $(patsubst modules/%/Dockerfile,%,$(wildcard modules/*/Dockerfile))
RELEASE_PLATFORMS ?= linux/amd64,linux/arm64
# --push publishes; a local check of one platform uses RELEASE_PLATFORMS=linux/amd64 RELEASE_OUTPUT=--load.
RELEASE_OUTPUT ?= --push
CLI_RELEASE_TARGETS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
MCP_RELEASE_TARGETS := $(CLI_RELEASE_TARGETS) windows/amd64 windows/arm64

# usage: $(call release_image,<image name>,<build context>,<Dockerfile>)
release_image = docker buildx build --platform $(RELEASE_PLATFORMS) $(RELEASE_OUTPUT) --build-arg VERSION=$(VERSION) \
	-t $(GENWORK_IMAGE_REGISTRY)/genwork-$(1):$(VERSION) -f $(2)/$(3) $(2)

# usage: $(call release_binaries,<component dir>,<binary>,<package path of the Version variable>,<os/arch targets>)
define release_binaries
	@set -e; for target in $(4); do \
		os=$${target%/*}; arch=$${target#*/}; work=$$(mktemp -d); executable=$(2); \
		if [ "$$os" = windows ]; then executable=$(2).exe; fi; \
		(cd $(1) && CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath $(call version_ldflags,$(3)) -o $$work/$$executable ./cmd/$(2)); \
		if [ "$$os" = windows ]; then (cd $$work && zip -q $(RELEASE_DIST)/$(2)_$(VERSION)_$${os}_$${arch}.zip $$executable); \
		else tar -czf $(RELEASE_DIST)/$(2)_$(VERSION)_$${os}_$${arch}.tar.gz -C $$work $$executable; fi; \
		rm -rf $$work; echo "built $(2) for $$os/$$arch"; \
	done
endef

# RELEASE_TAG is the pushed tag; a release is cut only when it names the version in VERSION.
check-release-tag:
	@test "$(RELEASE_TAG)" = "v$(VERSION)" || { echo "tag '$(RELEASE_TAG)' does not match VERSION ($(VERSION)); expected v$(VERSION)"; exit 1; }

# GENWORK_IMAGE_REGISTRY names the registry to push to, e.g. ghcr.io/<owner>.
release-images:
	@test -n "$(GENWORK_IMAGE_REGISTRY)" || { echo "GENWORK_IMAGE_REGISTRY must be set, e.g. ghcr.io/<owner>"; exit 1; }
	$(call release_image,core,core,Dockerfile)
	$(foreach module,$(RELEASE_MODULES),$(call release_image,$(module),.,modules/$(module)/Dockerfile) && ) true
	$(call release_image,gateway,.,deploy/gateway/Dockerfile)
	$(call release_image,mcp,.,mcp/Dockerfile)
	$(call release_image,updater,.,deploy/updater/Dockerfile)

# install.sh takes the release repository from GITHUB_SERVER_URL and GITHUB_REPOSITORY, which GitHub Actions sets.
release-artifacts:
	rm -rf $(RELEASE_DIST) && mkdir -p $(RELEASE_DIST)
	$(call release_binaries,cli,genwork,cli/internal/commands,$(CLI_RELEASE_TARGETS))
	$(call release_binaries,mcp,genwork-mcp,mcp/internal/command,$(MCP_RELEASE_TARGETS))
	cd cli && go run ./cmd/genwork-release bundle $(CURDIR) $(RELEASE_DIST)
	cd cli && go run ./cmd/genwork-release install-script $(CURDIR) $(RELEASE_DIST)
	cd $(RELEASE_DIST) && shasum -a 256 *.tar.gz *.zip install.sh > checksums.txt
