.ONESHELL:
.SHELL      := $(shell which bash)
.SHELLFLAGS := -ec

.PHONY: help build test fmt vet install clean

help: ## Print this help message
	@grep -E "^[a-zA-Z_-]+:.*?## .*$$" $(MAKEFILE_LIST) |\
		sort |\
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Build gh-arc binary
	go build -o gh-arc

test: ## Run all tests
	go test ./...

fmt: ## Format .go files
	go fmt ./...

vet: ## Lint .go files
	go vet ./...

install: ## Install extension locally
	gh extension install .

clean: ## Remove capy binary
	rm -f gh-arc
