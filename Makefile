# Image registry and version
REGISTRY ?= ghcr.io/zrs-io/hetero-compute-router
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Component images
IMG_NODE_AGENT ?= $(REGISTRY)/node-agent:$(VERSION)
IMG_SCHEDULER ?= $(REGISTRY)/scheduler:$(VERSION)
IMG_WEBHOOK ?= $(REGISTRY)/webhook:$(VERSION)

# Legacy single image (deprecated)
IMG ?= controller:latest

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Tool versions
CONTROLLER_TOOLS_VERSION ?= v0.20.0

.PHONY: all
all: build

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate CRD manifests
	$(CONTROLLER_GEN) crd:allowDangerousTypes=true paths="./pkg/api/..." output:crd:artifacts:config=config/crd

.PHONY: generate
generate: controller-gen ## Generate DeepCopy methods
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./pkg/api/..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run ./...

.PHONY: test
test: fmt vet ## Run tests.
	go test ./... -coverprofile cover.out

##@ Build

.PHONY: build
build: fmt vet ## Build all binaries.
	go build -o bin/node-agent ./cmd/node-agent
	go build -o bin/scheduler ./cmd/scheduler
	go build -o bin/webhook ./cmd/webhook

.PHONY: run
run: fmt vet ## Run a controller from your host.
	go run ./cmd/node-agent/main.go

.PHONY: docker-build
docker-build: docker-build-node-agent docker-build-scheduler docker-build-webhook ## Build all docker images.

.PHONY: docker-build-node-agent
docker-build-node-agent: ## Build node-agent docker image.
	docker build -t $(IMG_NODE_AGENT) -f Dockerfile .

.PHONY: docker-build-scheduler
docker-build-scheduler: ## Build scheduler docker image.
	docker build -t $(IMG_SCHEDULER) -f Dockerfile.scheduler .

.PHONY: docker-build-webhook
docker-build-webhook: ## Build webhook docker image.
	docker build -t $(IMG_WEBHOOK) -f Dockerfile.webhook .

.PHONY: docker-push
docker-push: ## Push all docker images.
	docker push $(IMG_NODE_AGENT)
	docker push $(IMG_SCHEDULER)
	docker push $(IMG_WEBHOOK)

.PHONY: clean
clean: ## Clean build artifacts
	rm -rf bin/

##@ Tools

CONTROLLER_GEN = $(GOBIN)/controller-gen
.PHONY: controller-gen
controller-gen: ## Download controller-gen locally if necessary
	@test -s $(CONTROLLER_GEN) || GOBIN=$(GOBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)
