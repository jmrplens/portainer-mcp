BINARY  := portainer-mcp
PKG     := github.com/jmrplens/portainer-mcp
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X $(PKG)/internal/version.Version=$(VERSION) \
	-X $(PKG)/internal/version.Commit=$(COMMIT) \
	-X $(PKG)/internal/version.BuildDate=$(DATE)

.PHONY: build test test-race cover lint vulncheck fmt check clean gen-client update-spec fetch-history gen-applicability gen-action-inputs check-spec validate-spec e2e-up e2e-down e2e-k8s-up e2e-k8s-down test-e2e audit-e2e-gaps audit-1to1 audit-1to1-ratchet audit-discovery audit-spec-reality audit-spec-drift audit-spec-delta e2e-licence-release

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

# audit-discovery reports which sibling actions (same domain, same base name
# once CRUD and variant suffixes are stripped, e.g. registries.delete and
# registries.repository_tags_delete) cannot yet be told apart because their
# Usage text is identical or missing. Informational, like audit-e2e-gaps: it
# exits 0 unless the catalog itself fails to build. Discovery quality is a
# judgement call, and gating on one invites satisfying the gate with filler
# text instead of writing text that actually helps — audit_1to1 is the audit
# that blocks the build.
audit-discovery:
	go run ./cmd/audit_discovery

# audit-1to1 is the gate this whole rewrite exists for: it fails when any
# operation documented in either vendored spec has no matching catalog
# action. Unlike audit-e2e-gaps it is a real gate, not merely informational.
# It is the 100%-or-bust check a human asking "are we done" runs; see
# cmd/audit_1to1's package doc for why it currently fails (18 of 441 Business
# Edition operations declared) and why that is the correct state for most of
# P3. CI does not call this target directly — see audit-1to1-ratchet below.
audit-1to1:
	go run ./cmd/audit_1to1 -spec-version=$(SPEC_VERSION)

# audit-1to1-ratchet is what CI actually gates on: the same audit as
# audit-1to1, but passing once coverage meets the floor committed in
# api/coverage-baseline.yaml rather than requiring 100%. See runRatchet's own
# doc comment in cmd/audit_1to1/main.go for why a ratchet, rather than either
# a permanently-failing or a permanently-passing check, is the right gate
# while P3 is still landing domains.
audit-1to1-ratchet:
	go run ./cmd/audit_1to1 -spec-version=$(SPEC_VERSION) -ratchet

# audit-spec-reality probes a live estate (see e2e-up) for every operation
# the vendored specification documents, and reports which of them the
# running server does not actually serve — the mechanism cmd/audit_spec_reality's
# package doc describes in full. It is read-only against the estate (every
# probe carries a credential that is not, and will never be, valid) and it
# reports rather than gates: a divergence is a fact about Portainer, not a
# defect in this project's code, so this never fails the build over what it
# finds. It requires a running estate (PORTAINER_E2E_ESTATE, or the default
# test/e2e/.estate.json that e2e-up writes) and fails only when it cannot run
# at all — no estate, an unreadable spec, or a failed self-test.
audit-spec-reality:
	go run ./cmd/audit_spec_reality -spec-version=$(SPEC_VERSION)

# audit-spec-drift fails when a declared catalog action's parameter shape no
# longer matches the vendored specification operation it was generated from
# — a field renamed, a type widened, a "required" dropped. Unlike
# audit-spec-reality this gates: drift against the vendored specification is
# a defect in this project's own code, not a fact about Portainer, and it
# starts clean today because every action was generated from that
# specification. See cmd/audit_spec_drift's package doc for the mandatory
# canary self-test every run performs first, and for why a description-only
# change gates only when the specification itself published the text that
# drifted.
audit-spec-drift:
	go run ./cmd/audit_spec_drift -spec-version=$(SPEC_VERSION)

# audit-spec-delta reports what changed between two OpenAPI documents — the
# vendored one and a newer candidate — as a work list grouped by domain,
# ordered added/removed/struct-touching/cosmetic within each domain. It never
# gates, unlike audit-spec-drift above: a candidate version has not been
# adopted yet, so there is nothing about the difference for this project's
# own build to fail on. See cmd/audit_spec_delta's package doc for why its
# "changed" count is narrower than a full operation-node diff by design, and
# for the real 2.43.0 -> 2.44.0 measurement that verified it.
#
# BEFORE and AFTER are required file paths to two OpenAPI documents — the
# currently vendored api/specs/ee-$(SPEC_VERSION).json is the usual BEFORE;
# a newer version bundled into a scratch path with
# plan/research/specs/bundle.py (never into api/specs/, which stays vendored
# to the one version the catalog was generated from) is the usual AFTER.
# JSON=1 emits machine-readable output instead of the human work list.
#
# Example:
#   python3 plan/research/specs/bundle.py ee 2.45.0 /tmp/ee-2.45.0.json
#   make audit-spec-delta BEFORE=api/specs/ee-$(SPEC_VERSION).json AFTER=/tmp/ee-2.45.0.json
audit-spec-delta:
	@if [ -z "$(BEFORE)" ] || [ -z "$(AFTER)" ]; then \
		echo "usage: make audit-spec-delta BEFORE=<path-to-older-spec> AFTER=<path-to-newer-spec> [JSON=1]"; \
		exit 2; \
	fi
	go run ./cmd/audit_spec_delta -before $(BEFORE) -after $(AFTER) $(if $(JSON),-json,)

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

# gen-action-inputs regenerates internal/tools/<domain>/inputs.gen.go from the
# vendored Business Edition specification: one Input struct per operation
# already declared by a domain package, merging its path parameters, query
# parameters and request body into the flat, model-facing shape
# toolutil.ActionSpec.Input expects. See cmd/gen_action_inputs's package doc
# for what it refuses to guess rather than generate.
gen-action-inputs:
	go run ./cmd/gen_action_inputs -spec api/specs/ee-$(SPEC_VERSION).json -tools-dir internal/tools

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
