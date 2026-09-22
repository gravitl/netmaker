# Netmaker build targets for Community Edition (CE) and Enterprise Edition (EE).
#
# Docker image tags mirror .github/workflows/publish-docker.yml:
#   CE: gravitl/netmaker:<TAG>
#   EE: gravitl/netmaker:<TAG>-ee
#
# EE builds are slow on first run (CGO + SQLite + large pro/ tree).
# Run `make deps` once, then use `make build-ee` — cached rebuilds are much faster.

.PHONY: help deps check-cgo build-env build build-ce build-ee build-ee-timed \
	image-ce image-ee images push-ce push-ee push clean

IMAGE ?= gravitl/netmaker
TAG ?= dev
DOCKERFILE ?= Dockerfile
GO_BUILDER_IMAGE ?= gravitl/go-builder:1.25.3

# CE includes arm/v7; EE matches CI (amd64 + arm64 only).
PLATFORMS_CE ?= linux/amd64,linux/arm64,linux/arm/v7
PLATFORMS_EE ?= linux/amd64,linux/arm64

BUILD_TAGS_CE ?= ce
BUILD_TAGS_EE ?= ee
LDFLAGS ?= -s -w
CGO_ENABLED ?= 1
BIN_DIR ?= bin
GO ?= go
GOMAXPROCS ?= $(shell nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4)

# Shared go build invocation — uses parallel compilation and build cache by default.
# Avoid `go build -a` unless you need a full rebuild.
GO_BUILD = CGO_ENABLED=$(CGO_ENABLED) GOMAXPROCS=$(GOMAXPROCS) $(GO) build -ldflags="$(LDFLAGS)"

.DEFAULT_GOAL := help

help: ## Show available targets
	@echo "Netmaker CE/EE build targets"
	@echo ""
	@echo "Variables:"
	@echo "  IMAGE=$(IMAGE)"
	@echo "  TAG=$(TAG)"
	@echo "  CGO_ENABLED=$(CGO_ENABLED)"
	@echo "  GOMAXPROCS=$(GOMAXPROCS)"
	@echo ""
	@echo "Tips for slow EE builds:"
	@echo "  1. make deps          — download modules first"
	@echo "  2. make check-cgo     — ensure gcc/build-essential is installed"
	@echo "  3. make build-env     — inspect GOCACHE / GOMODCACHE"
	@echo "  4. make build-ee-timed — time a full EE build"
	@echo "  5. Use ccache for repeated CGO/SQLite compiles (export CC=\"ccache gcc\")"
	@echo ""
	@grep -E '^[a-zA-Z0-9_.-]+:.*## ' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*## "}; {printf "  %-16s %s\n", $$1, $$2}'

deps: ## Download Go modules (run once before first build)
	$(GO) mod download

check-cgo: ## Verify C toolchain required for CGO/SQLite builds
	@echo "CGO_ENABLED=$(CGO_ENABLED)"
	@echo "GOMAXPROCS=$(GOMAXPROCS)"
	@command -v gcc >/dev/null 2>&1 || { \
		echo "ERROR: gcc not found."; \
		echo "  Debian/Ubuntu: sudo apt install build-essential"; \
		echo "  Alpine:        apk add build-base"; \
		exit 1; \
	}
	@gcc --version | head -1
	@echo "C toolchain OK"

build-env: ## Show Go cache paths and build settings
	@echo "GOMAXPROCS=$(GOMAXPROCS)"
	@echo "CGO_ENABLED=$(CGO_ENABLED)"
	@$(GO) env GOCACHE GOMODCACHE GOOS GOARCH GOPATH
	@command -v gcc >/dev/null 2>&1 && gcc --version | head -1 || echo "gcc: not found"

build: deps build-ce build-ee ## Build CE and EE binaries locally

build-ce: deps check-cgo ## Build CE binary to $(BIN_DIR)/netmaker
	@mkdir -p $(BIN_DIR)
	$(GO_BUILD) -o $(BIN_DIR)/netmaker .

build-ee: deps check-cgo ## Build EE binary to $(BIN_DIR)/netmaker-ee
	@mkdir -p $(BIN_DIR)
	$(GO_BUILD) -tags $(BUILD_TAGS_EE) -o $(BIN_DIR)/netmaker-ee .

build-ee-timed: check-cgo deps ## Time an EE build (useful for diagnosing slow compiles)
	@mkdir -p $(BIN_DIR)
	@echo "Building EE with GOMAXPROCS=$(GOMAXPROCS) CGO_ENABLED=$(CGO_ENABLED) ..."
	@time $(GO_BUILD) -tags $(BUILD_TAGS_EE) -o $(BIN_DIR)/netmaker-ee .

image-ce: ## Build CE Docker image ($(IMAGE):$(TAG))
	docker build \
		--build-arg tags=$(BUILD_TAGS_CE) \
		-t $(IMAGE):$(TAG) \
		-f $(DOCKERFILE) \
		.

image-ee: ## Build EE Docker image ($(IMAGE):$(TAG)-ee)
	docker build \
		--build-arg tags=$(BUILD_TAGS_EE) \
		-t $(IMAGE):$(TAG)-ee \
		-f $(DOCKERFILE) \
		.

images: image-ce image-ee ## Build both CE and EE Docker images

push-ce: ## Build and push CE multi-arch image ($(IMAGE):$(TAG))
	docker buildx build \
		--platform $(PLATFORMS_CE) \
		--build-arg tags=$(BUILD_TAGS_CE) \
		-t $(IMAGE):$(TAG) \
		-f $(DOCKERFILE) \
		--push \
		.

push-ee: ## Build and push EE multi-arch image ($(IMAGE):$(TAG)-ee)
	docker buildx build \
		--platform $(PLATFORMS_EE) \
		--build-arg tags=$(BUILD_TAGS_EE) \
		-t $(IMAGE):$(TAG)-ee \
		-f $(DOCKERFILE) \
		--push \
		.

push: push-ce push-ee ## Build and push both CE and EE multi-arch images

clean: ## Remove local build artifacts
	rm -rf $(BIN_DIR)
