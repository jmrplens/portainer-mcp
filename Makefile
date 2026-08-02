BINARY  := portainer-mcp
PKG     := github.com/jmrplens/portainer-mcp
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X $(PKG)/internal/version.Version=$(VERSION) \
	-X $(PKG)/internal/version.Commit=$(COMMIT) \
	-X $(PKG)/internal/version.BuildDate=$(DATE)

.PHONY: build test test-race cover lint vulncheck fmt check clean gen-client update-spec fetch-history gen-applicability

SPEC_VERSION ?= 2.44.0

build:
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o dist/$(BINARY) ./cmd/$(BINARY)

test:
	go test ./... -count=1

test-race:
	go test ./... -count=1 -race

cover:
	go test ./... -count=1 -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1

lint:
	golangci-lint run ./...

vulncheck:
	govulncheck ./...

fmt:
	golangci-lint fmt ./...

check: fmt lint vulncheck test

clean:
	rm -rf dist coverage.out

update-spec:
	go run ./cmd/fetch_spec -edition ee -version $(SPEC_VERSION)
	go run ./cmd/fetch_spec -edition ce -version $(SPEC_VERSION)

gen-client:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 \
		--config api/oapi-codegen-types.yaml api/specs/ee-$(SPEC_VERSION).json
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 \
		--config api/oapi-codegen-client.yaml api/specs/ee-$(SPEC_VERSION).json
	gofmt -w internal/portainer/gen

fetch-history:
	@for ed in ce ee; do \
		for v in $$(curl -sfL -A portainer-mcp https://api-docs.portainer.io/$$ed-versions.json | python3 -c 'import json,sys; print(" ".join(v["id"] for v in json.load(sys.stdin)))'); do \
			go run ./cmd/fetch_spec -edition $$ed -version $$v -out api/specs/history || exit 1; \
		done; \
	done

gen-applicability:
	go run ./cmd/gen_applicability -history api/specs/history -out internal/apiversion/applicability_gen.go
	gofmt -w internal/apiversion/applicability_gen.go
