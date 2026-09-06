# iotwong — unified build/check entrypoints (AGENTS.md T01 contract).
# Every target must be runnable from the repository root.
#
# Prereqs: Go toolchain (>= go.mod directive), Node.js >= 22 + npm.
# The Go module proxy is only needed for `go mod tidy` on new deps.

SHELL := /bin/bash
ROOT  := $(abspath .)
BACKEND := $(ROOT)/backend
WEB    := $(ROOT)/web

# Locate toolchains: honor PATH first, fall back to the local /usr/local install.
GO  := $(or $(shell command -v go 2>/dev/null),/usr/local/go/bin/go)
NPM := $(or $(shell command -v npm 2>/dev/null),/usr/local/bin/npm)

# Optional module proxy override for environments where proxy.golang.org is
# unreachable (e.g. GOPROXY=https://goproxy.cn,direct make tidy).
GOPROXY ?= https://proxy.golang.org,direct
export GOPROXY

.PHONY: check test e2e build compose-check tidy frontend-install

## check — static checks: frontend type/lint + Go vet
check:
	@echo "== web typecheck/lint =="
	cd $(WEB) && npm run check && npm run lint
	@echo "== go vet =="
	cd $(BACKEND) && $(GO) vet ./...
	@echo "check OK"

## test — unit/integration tests
test:
	@echo "== go test =="
	cd $(BACKEND) && $(GO) test ./...
	@echo "test OK"

## e2e — end-to-end smoke against real processes
e2e: build
	@echo "== e2e health smoke =="
	PATH="$(shell dirname $(GO)):$$PATH" $(ROOT)/scripts/e2e-health.sh

## build — Go binaries + static SPA
build: 
	@echo "== go build =="
	cd $(BACKEND) && $(GO) build -o $(BACKEND)/bin/server ./cmd/server
	cd $(BACKEND) && $(GO) build -o $(BACKEND)/bin/ingestor ./cmd/ingestor
	cd $(BACKEND) && $(GO) build -o $(BACKEND)/bin/import-racebox ./cmd/import-racebox
	cd $(BACKEND) && $(GO) build -o $(BACKEND)/bin/migrate ./cmd/migrate
	cd $(BACKEND) && $(GO) build -o $(BACKEND)/bin/seed ./cmd/seed
	cd $(BACKEND) && $(GO) build -o $(BACKEND)/bin/sim-send ./cmd/sim-send
	@echo "== web build =="
	cd $(WEB) && npm run build
	@echo "build OK"
	@echo "note: seed/sim-send are T04 dev/verification tools (not part of runtime images)"

## migrate — apply db/migrations with PG* env (owner role only)
migrate:
	cd $(BACKEND) && $(GO) run ./cmd/migrate --dir $(ROOT)/db/migrations

## compose-check — validate deliverable compose files when they exist (T07+)
compose-check:
	@files=""; \
	for f in compose.yaml compose.standalone.yaml; do \
	  [ -f "$(ROOT)/$$f" ] || { echo "compose-check: missing $(ROOT)/$$f"; exit 1; }; \
	  envf=""; [ -f "$(ROOT)/.env" ] && envf="--env-file $(ROOT)/.env"; \
	  docker compose $${envf} -f "$(ROOT)/$$f" config --quiet \
	    || { echo "compose-check FAIL: $$f"; exit 1; }; \
	  echo "compose-check OK: $$f"; \
	done

## tidy — refresh dependency locks after editing go.mod / package.json
tidy:
	cd $(BACKEND) && $(GO) mod tidy
	cd $(WEB) && npm install
