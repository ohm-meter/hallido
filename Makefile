# Sonos Radio Agent -- build and development targets.
# "make help" lists all targets.

BINARY      := sonos-radio-agent
PKG         := ./...
# -race only where supported (not linux/arm, needs cgo/gcc); disable: make test RACE=
RACE        ?= $(shell [ "$$(go env GOARCH)" != arm ] && command -v gcc >/dev/null 2>&1 && echo -race)
BUILD_DIR   := build
# Pi default: arm64 (Pi 3/4/5/Zero 2 W with 64-bit Raspberry Pi OS).
# For older 32-bit Pis: make build-pi PI_ARCH=arm PI_ARM=7
PI_ARCH     ?= arm64
PI_ARM      ?=
# -race exists only on amd64/arm64 (not on linux/arm, e.g. 32-bit Pi).
RACE        := $(shell go env GOARCH | grep -qE '^(amd64|arm64)$$' && echo -race)

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: fmt
fmt: ## Format the code
	gofmt -w cmd internal

.PHONY: vet
vet: ## Static analysis
	go vet $(PKG)

.PHONY: test
test: vet test-install test-nas ## Run tests + vet (required before a commit)
	go test $(RACE) $(PKG)

.PHONY: test-install
test-install: ## Install layout: .install-meta parser, unit migration (no root)
	./scripts/test/install-layout.sh

.PHONY: test-nas
test-nas: ## NAS project file: render template, check hardening
	./scripts/test/nas-compose.sh

.PHONY: image
image: ## Container image for this machine (hallido:dev), see Dockerfile
	docker buildx build --load -t hallido:dev .

.PHONY: test-install-docker
test-install-docker: ## Migration old -> new layout with real paths (Docker)
	./scripts/test/install-migration-docker.sh

.PHONY: test-api
test-api: ## API integration against the mock (starts the agent if needed)
	./scripts/test/api-mock.sh

.PHONY: test-api-smoke
test-api-smoke: ## Minimal API smoke check (3 requests)
	./scripts/test/api-smoke.sh

.PHONY: build-gui
build-gui: ## Binary with the current web app (go:embed) for layout tests
	mkdir -p $(BUILD_DIR)
	go build -trimpath -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY) ./cmd/server

.PHONY: test-gui
test-gui: build-gui ## Web app checks (static + Playwright layout)
	./scripts/test/gui-static.sh
	./scripts/test/gui-layout.sh

.PHONY: test-scripts
test-scripts: ## All shell tests (API + web app)
	./scripts/test/run-all.sh

.PHONY: cover
cover: ## Show test coverage
	go test -cover $(PKG)

.PHONY: run
run: ## Run locally with the mock driver (needs config.json)
	go run ./cmd/server -config config.json -debug

.PHONY: dev
dev: ## Development: Sonos driver, debug (scripts/dev.sh)
	./scripts/dev.sh

.PHONY: prod-local
prod-local: ## Local prod simulation: build and start the binary
	./scripts/prod.sh

.PHONY: build
build: test ## Build the binary for the current platform (after green tests)
	mkdir -p $(BUILD_DIR)
	go build -trimpath -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY) ./cmd/server

.PHONY: build-pi
build-pi: test ## Cross-compile for Raspberry Pi (default arm64)
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=$(PI_ARCH) GOARM=$(PI_ARM) \
		go build -trimpath -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY)-pi ./cmd/server
	@echo "Built: $(BUILD_DIR)/$(BINARY)-pi (linux/$(PI_ARCH))"

.PHONY: dist
dist: test ## Release archives (Linux amd64/arm64/armv7/armv6, macOS) + SHA256SUMS into dist/
	./scripts/dist.sh $(VERSION)

.PHONY: clean
clean: ## Delete build artifacts
	rm -rf $(BUILD_DIR) dist
