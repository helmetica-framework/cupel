# Set Shell to bash, otherwise some targets fail with dash/zsh etc.
SHELL := /bin/bash

# Disable built-in rules
MAKEFLAGS += --no-builtin-rules
MAKEFLAGS += --no-builtin-variables
.SUFFIXES:
.SECONDARY:
.DEFAULT_GOAL := help

include Makefile.vars.mk

.PHONY: help
help: ## Show this help
	@grep -E -h '\s##\s' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

all: build ## Invokes the build target

.PHONY: test
test: generate ## Run tests
	go test ./... -race -coverprofile cover.out

.PHONY: build
build: generate fmt vet $(BIN_FILENAME) ## Build the cupel binary

.PHONY: docs
docs: ## Build the docs as they appear on docs.helmetica.io; fails on warnings
	GOTOOLCHAIN=auto GOEXPERIMENT=jsonv2 go run github.com/helmetica-framework/rosarium@v0.1.0 preview .

.PHONY: docs-serve
docs-serve: ## Build the docs and serve them on http://localhost:2020
	GOTOOLCHAIN=auto GOEXPERIMENT=jsonv2 go run github.com/helmetica-framework/rosarium@v0.1.0 preview . --serve :2020

.PHONY: generate
generate: ## Run code generators
	go generate ./...

.PHONY: fmt
fmt: ## Run go fmt against code
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code
	go vet ./...

.PHONY: lint
lint: fmt vet generate docs ## All-in-one linting
	@echo 'Check for uncommitted changes ...'
	git diff --exit-code

.PHONY: build.docker
build.docker: $(BIN_FILENAME) ## Build the docker image
	docker build . \
		--tag $(GHCR_IMG)

clean: ## Cleans up the generated resources
	rm -rf dist/ cover.out $(BIN_FILENAME) ||:

.PHONY: run
run: fmt vet ## Run cupel from your host, e.g. make run ARGS="compare myrelease 3 5"
	go run . $(ARGS)

###
### Assets
###

# Build the binary without running generators
.PHONY: $(BIN_FILENAME)
$(BIN_FILENAME): export CGO_ENABLED = 0
$(BIN_FILENAME):
	@echo "GOOS=$$(go env GOOS) GOARCH=$$(go env GOARCH)"
	go build -o $(BIN_FILENAME)
