BINARY  := portainer-mcp
PKG     := github.com/jmrplens/portainer-mcp
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X $(PKG)/internal/version.Version=$(VERSION) \
	-X $(PKG)/internal/version.Commit=$(COMMIT) \
	-X $(PKG)/internal/version.BuildDate=$(DATE)

.PHONY: build test test-race cover lint vulncheck fmt check clean gen-client update-spec fetch-history gen-applicability check-spec validate-spec e2e-up e2e-down e2e-k8s-up e2e-k8s-down test-e2e audit-e2e-gaps e2e-licence-release

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

e2e-up:
	./test/e2e/scripts/up.sh

e2e-down:
	./test/e2e/scripts/down.sh

e2e-k8s-up:
	./test/e2e/scripts/k3d-up.sh

e2e-k8s-down:
	./test/e2e/scripts/k3d-down.sh

# e2e-licence-release recovers a Business Edition licence stranded by a run
# that crashed before its own teardown (e2e-down / e2e-k8s-down, which
# release on every clean path) could release it. Attaches the licence to a
# throwaway server and releases it immediately; safe to run even when nothing
# is actually stranded.
e2e-licence-release:
	./test/e2e/scripts/licence-check.sh

test-e2e:
	go test -tags e2e -timeout 15m -count=1 ./test/e2e/suite/...

# audit-e2e-gaps reports which catalog actions no e2e test references. It is
# informational, not a CI gate, until P7: with the catalog in early phases of
# P3's growth to 441 actions, a hard gate would fail on almost the whole
# catalog and teach everyone to ignore it. Its exit code is 0 unless the
# catalog itself fails to build; the unexercised count is printed, never
# swallowed, so coverage nobody has never reads as coverage verified.
audit-e2e-gaps:
	go run ./cmd/audit_e2e_gaps

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

# check-spec verifies the committed specifications still match a fresh fetch.
# It writes into a temporary directory rather than over api/specs, so a failure
# never leaves the working tree modified — a check that mutates what it checks
# is a trap for whoever runs it locally.
check-spec:
	@tmp=$$(mktemp -d) && trap 'rm -rf "$$tmp"' EXIT; \
	for ed in ee ce; do \
		go run ./cmd/fetch_spec -edition $$ed -version $(SPEC_VERSION) -out "$$tmp" || exit 1; \
		diff -q "api/specs/$$ed-$(SPEC_VERSION).json" "$$tmp/$$ed-$(SPEC_VERSION).json" >/dev/null \
			|| { echo "committed $$ed spec differs from a fresh fetch; run make update-spec"; exit 1; }; \
	done; \
	echo "committed specs are current"

# validate-spec is a manual diagnostic tool, not a CI gate: ogen refuses to
# generate for the committed spec, and the failure does not have a narrow fix.
#
# The first error is a schema name collision on "TypesUpdateScheduleType",
# reached via POST /edge_update_schedules -> types.UpdateSchedule.properties.type.
# The property is `allOf: [$ref types.UpdateScheduleType]` plus a sibling
# `enum` narrowing the allowed values, which forces ogen to synthesize an
# inline type for the composed property; its generated name collides with the
# named component types.UpdateScheduleType, which PascalCases to the same
# string.
#
# Renaming that one component (verified with a local spike, not committed:
# renaming is spec-internal and does not change the wire contract) does not
# fix the document — it only uncovers the same allOf+sibling-enum-vs-named-
# component collision recurring elsewhere: next on portainer.RegistryType
# (reached through portainer.Registry.properties.Type), then again on
# policies.PolicyType. The pattern repeats throughout the spec wherever a
# shared "*Type" enum is reused with a narrower sibling enum, so a fix
# scoped to one occurrence is not narrow — it would mean auditing and
# renaming an unknown number of shared enum components across the whole
# document, which is a mechanical rule in name only. See task-7-report.md
# for the full trace.
#
# ogen is kept here as a spec linter for manual, occasional use (its
# diagnostics carry file:line and found three real upstream defects that
# oapi-codegen swallowed silently) but is deliberately not wired into CI.
# Do not add --ignore-not-implemented to force a pass: that flag makes ogen
# skip the operations it cannot read, which certifies a spec while ignoring
# the parts that are broken.
validate-spec:
	go run github.com/ogen-go/ogen/cmd/ogen@v1.23.0 \
		--target /tmp/ogen-validate --package validate --clean \
		api/specs/ee-$(SPEC_VERSION).json
