# Copyright (c) 2017-2021, Habana Labs. All rights reserved.
TAG ?= $(shell git describe --abbrev=0 --tags --always | tr '[:upper:]' '[:lower:]')
HASH := $(shell git rev-parse HEAD)
DATE := $(shell date +%Y-%m-%d.%H:%M:%S)
UPDATE_TYPE ?= "patch"

DOCKER ?= docker
MKDIR  ?= mkdir
DIST_DIR ?= $(CURDIR)/dist
LOCAL_REGISTRY ?= ""
DOCKER_SOCK ?= /var/run/docker.sock
DOCKER_HOST ?= unix:///var/run/docker.sock

RUNTIME_BINARY := habana-container-runtime
HOOK_BINARY := habana-container-hook
CLI_BINARY := habana-container-cli
TOOLKIT_BINARY := habana-container-toolkit

IMAGE_TAG := artifactory-kfs.habana-labs.com/artifactory/docker-local/$(subst -$(word 2,$(subst -, ,$(TAG))),,$(TAG))/habanalabs/habana-container-runtime:$(TAG)

LIB_NAME := habanalabs-container-runtime
LIB_VERSION ?= 1.16.0
PKG_REV ?= 1

GOLANG_VERSION := 1.25.8
GO_RELEASER_VERSION := v2.13.3

# # Go CI related commands
build/bin: build-binary
build-binary: clean build-runtime build-hook build-cli build-toolkit
	@echo "Binaries built successfully in dist/linux_amd64/"

build-runtime:
	@echo "Building $(RUNTIME_BINARY)"
	@CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build  -o dist/linux_amd64/${RUNTIME_BINARY} ./cmd/habana-container-runtime/

build-hook:
	@echo "Building $(HOOK_BINARY)"
	@CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build  -o dist/linux_amd64/${HOOK_BINARY} ./cmd/habana-container-runtime-hook/

build-cli:
	@echo "Building $(CLI_BINARY)"
	@CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build  -o dist/linux_amd64/${CLI_BINARY} ./cmd/habana-container-cli/

build-toolkit:
	@echo "Building $(TOOLKIT_BINARY)"
	@CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build  -o dist/linux_amd64/${TOOLKIT_BINARY} ./cmd/habana-container-toolkit/

clean:
	go clean > /dev/null
	rm -rf dist/*

test:
	@go test ./... -coverprofile=coverage.out

coverage:
	@go tool cover -func coverage.out | grep "total:" | awk '{print  ((int($$3) > 80) != 1)}'

check-format:
	@test -z $$(go fmt ./...)

lint:
	@golangci-lint run ./...

# Build the binaries in all available architectures.
build:
	$(DOCKER) run --rm \
		-v $$PWD:/go/src/github.com/HabanaAI/habana-container-runtime \
		-w /go/src/github.com/HabanaAI/habana-container-runtime \
		-e GITHUB_TOKEN \
		-e DOCKER_USERNAME \
		-e DOCKER_PASSWORD \
		-e DOCKER_REGISTRY \
		artifactory-kfs.habana-labs.com/docker-mirror/goreleaser/goreleaser:$(GO_RELEASER_VERSION) build --snapshot --clean
	$(MAKE) update-dist-permissions

# Build binaries, create archives and OS packages and uploads all artifacts to github repo
release:
	$(DOCKER) run --rm \
		-v $$PWD:/go/src/github.com/HabanaAI/habana-container-runtime \
		-v $(DOCKER_SOCK):/var/run/docker.sock \
		-w /go/src/github.com/HabanaAI/habana-container-runtime \
		-e DOCKER_HOST=$(DOCKER_HOST) \
		-e GITHUB_TOKEN \
		-e DOCKER_USERNAME \
		-e DOCKER_PASSWORD \
		-e DOCKER_REGISTRY \
		artifactory-kfs.habana-labs.com/docker-mirror/goreleaser/goreleaser:$(GO_RELEASER_VERSION) release --clean --snapshot
	$(MAKE) update-dist-permissions
	$(DOCKER) tag habana-container-runtime:$(TAG) $(IMAGE_TAG)
	@$(DOCKER) rmi habana-container-runtime:$(TAG)

cd-release:
	$(DOCKER) run --rm \
		-v $$PWD:/go/src/github.com/HabanaAI/habana-container-runtime \
		-v $(DOCKER_SOCK):/var/run/docker.sock \
		-w /go/src/github.com/HabanaAI/habana-container-runtime \
		-v $$(dirname $$PWD)/.repo:/go/src/github.com/HabanaAI/.repo \
		-v $${HOME}/.gitconfig:/root/.gitconfig \
		-e DOCKER_HOST=$(DOCKER_HOST) \
		-e GITHUB_TOKEN \
		-e DOCKER_USERNAME \
		-e DOCKER_PASSWORD \
		-e DOCKER_REGISTRY \
		-e NFPM_PASSPHRASE \
		artifactory-kfs.habana-labs.com/docker-mirror/goreleaser/goreleaser:$(GO_RELEASER_VERSION) release --clean --snapshot --config .goreleaser-cd.yaml; \
	$(MAKE) update-dist-permissions; \
	$(MAKE) update-docker-tag;

update-dist-permissions:
	@sudo chown -R $$(id -u):$$(id -g) dist/ || :

update-docker-tag:
	$(DOCKER) tag habana-container-runtime:$(TAG) $(IMAGE_TAG); \
	$(DOCKER) rmi habana-container-runtime:$(TAG)

#######################################

# Supported OSs by architecture
AMD64_TARGETS := ubuntu20.04 ubuntu22.04 ubuntu18.04 debian10.10 ubuntu24.04
X86_64_TARGETS := centos7 centos8 rhel7 rhel8 amazonlinux1 amazonlinux2

# amd64 targets
AMD64_TARGETS := $(patsubst %, %-amd64, $(AMD64_TARGETS))
$(AMD64_TARGETS): ARCH := amd64
$(AMD64_TARGETS): %: --%
docker-amd64: $(AMD64_TARGETS)

# x86_64 targets
X86_64_TARGETS := $(patsubst %, %-x86_64, $(X86_64_TARGETS))
$(X86_64_TARGETS): ARCH := x86_64
$(X86_64_TARGETS): %: --%
docker-x86_64: $(X86_64_TARGETS)

# Default variables for all private '--' targets below.
# One private target is defined for each OS we support.
--%: TARGET_PLATFORM = $(*)
--%: VERSION = $(patsubst $(OS)%-$(ARCH),%,$(TARGET_PLATFORM))
--%: BASEIMAGE = $(OS):$(VERSION)

--%: BUILDIMAGE = habana/habana-container-runtime/$(OS)$(VERSION)-$(ARCH)
--%: DOCKERFILE = $(CURDIR)/docker/Dockerfile.$(OS)
--%: ARTIFACTS_DIR = $(DIST_DIR)/$(OS)$(VERSION)/$(ARCH)
--%: docker-build-%
	@

# private OS targets with defaults
--ubuntu%: OS := ubuntu
--debian%: OS := debian
--centos%: OS := centos
--amazonlinux%: OS := amazonlinux

--rhel%: OS := centos
--rhel%: VERSION = $(patsubst rhel%-$(ARCH),%,$(TARGET_PLATFORM))
--rhel%: ARTIFACTS_DIR = $(DIST_DIR)/rhel$(VERSION)/$(ARCH)

docker-build-%:
	@echo "Building for $(TARGET_PLATFORM)"
	docker pull --platform=linux/$(ARCH) $(LOCAL_REGISTRY)$(BASEIMAGE)
	DOCKER_BUILDKIT=1 \
	$(DOCKER) build \
	    --progress=plain \
	    --build-arg BASEIMAGE=$(LOCAL_REGISTRY)$(BASEIMAGE) \
	    --build-arg GOLANG_VERSION="$(GOLANG_VERSION)" \
	    --build-arg PKG_NAME="$(LIB_NAME)" \
	    --build-arg PKG_VERS="$(LIB_VERSION)" \
	    --build-arg PKG_REV="$(PKG_REV)" \
		--build-arg ARCH=$(ARCH) \
	    --tag $(BUILDIMAGE) \
	    --file $(DOCKERFILE) .
	$(DOCKER) run \
	    -e DISTRIB \
	    -e SECTION \
	    -v $(ARTIFACTS_DIR):/dist \
	    $(BUILDIMAGE)

# upgrade
.PHONY: update
update: ## Update Go dependencies.
	@if [ "$(UPDATE_TYPE)" = "patch" ]; then \
		GO_MINOR=$$(awk '/^go / {split($$2, v, "."); print v[1] "." v[2]; exit}' go.mod) && \
		go get go@$$GO_MINOR && \
		go get toolchain@go$$GO_MINOR; \
	else \
		go get go@latest && \
		go get toolchain@latest; \
	fi
	go get -u ./... && \
	GO_VERSION=$$(awk '/^go / {print $$2; exit}' go.mod) && \
		sed -i "s/FROM golang:.* AS golang/FROM golang:$$GO_VERSION AS golang/g" packaging/Dockerfile && \
		sed -i "s/^GOLANG_VERSION := .*/GOLANG_VERSION := $$GO_VERSION/g" Makefile

.PHONY: tidy
tidy: ## Run go mod tidy.
	go mod tidy

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

## Upgrade dependencies and run quality checks.
## Remember to 'export GOTOOLCHAIN=auto' before running this target to use the latest Go toolchain.
.PHONY: upgrade
upgrade: update tidy fmt vet
