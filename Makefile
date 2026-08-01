# Note: these can be overriden on the command line e.g. `make PLATFORM=<platform> ARCH=<arch>`
PLATFORM="$(shell go env GOOS)"
ARCH="$(shell go env GOARCH)"

VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT ?= $(shell git rev-parse --short HEAD)
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

LDFLAGS_STRING = -s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildDate=${BUILD_DATE}

.PHONY: clean pre build release run test test-coverage test-integration test-all fmt vet lint inspector check-docs

clean:
	rm -rf dist

pre:
	mkdir -p dist

build: pre
	GOOS=$(PLATFORM) GOARCH=$(ARCH) CGO_ENABLED=0 go build --ldflags '$(LDFLAGS_STRING)' -o dist/portainer-mcp-enhanced ./cmd/portainer-mcp-enhanced

release: pre
	GOOS=$(PLATFORM) GOARCH=$(ARCH) CGO_ENABLED=0 go build -trimpath --ldflags '$(LDFLAGS_STRING)' -o dist/portainer-mcp-enhanced ./cmd/portainer-mcp-enhanced

fmt:
	gofmt -s -w .

vet:
	go vet ./...

lint: vet
	@which golangci-lint > /dev/null 2>&1 || echo "golangci-lint not installed, skipping"
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run ./... || true

inspector: build
	npx @modelcontextprotocol/inspector dist/portainer-mcp-enhanced

test:
	go test -v $(shell go list ./... | grep -v /tests/integration)

test-coverage:
	go test -v $(shell go list ./... | grep -v /tests/integration) -coverprofile=./coverage.out

test-integration:
	go test -v ./tests/...

test-all: test test-integration

check-docs:
	@# AGENTS.md is the canonical source. CLAUDE.md and .github/copilot-instructions.md
	@# are derived from it with different formatting. This target warns if AGENTS.md has
	@# been modified more recently than either derived file, indicating they may need updating.
	@stale=""; \
	for f in CLAUDE.md .github/copilot-instructions.md; do \
		if [ AGENTS.md -nt "$$f" ]; then \
			stale="$$stale $$f"; \
		fi; \
	done; \
	if [ -n "$$stale" ]; then \
		echo "WARNING: AGENTS.md is newer than:$$stale"; \
		echo "Review AGENTS.md changes and update derived files if needed."; \
		exit 1; \
	else \
		echo "OK: CLAUDE.md and .github/copilot-instructions.md are up to date with AGENTS.md"; \
	fi

# Include custom make targets
-include $(wildcard .dev/*.make)