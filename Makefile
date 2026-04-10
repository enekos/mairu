SHELL := /bin/bash

.DEFAULT_GOAL := help

desktop-dev:
	cd mairu/cmd/desktop && wails dev

desktop-build:
	cd mairu/cmd/desktop && wails build -o mairu-desktop

desktop-clean:
	rm -rf mairu/cmd/desktop/build/bin

.PHONY: help install install-dashboard setup build lint test clean dashboard dashboard-api dashboard-dev mairu-build mairu-web desktop-dev desktop-build desktop-clean
.PHONY: fmt-go fmt-go-check lint-go test-go test-go-race test-go-cover check-go check-go-ci install-hooks
.PHONY: eval-retrieval eval-seed eval-llm
.PHONY: meili-up meili-down meili-status meili-clean setup-no-docker dev-no-docker mairu-no-docker
.PHONY: build-browser-extension install-browser-extension test-browser-extension

help:
	@echo "mairu monorepo Makefile"
	@echo
	@echo "Core:"
	@echo "  make install            Install root Bun dependencies"
	@echo "  make install-dashboard  Install dashboard dependencies"
	@echo "  make build              Build Go output"
	@echo "  make lint               Run Go vet"
	@echo "  make test               Run Go tests"
	@echo "  make fmt-go             Format Go code"
	@echo "  make check-go           Run fmt check + lint + tests"
	@echo "  make check-go-ci        Run CI-grade Go checks (includes race)"
	@echo "  make install-hooks      Install local git pre-commit hook"
	@echo "  make clean              Remove dist artifacts"
	@echo
	@echo "Browser Extension:"
	@echo "  make build-browser-extension    Build the browser extension and native host"
	@echo "  make install-browser-extension  Install the native host for Chrome"
	@echo "  make test-browser-extension     Run tests for the browser extension"
	@echo
	@echo "Runtime:"
	@echo "  make setup              Initialize indexes (requires Meilisearch)"
	@echo "  make dashboard          Start dashboard API + UI"
	@echo "  make mairu-build        Build mairu binary"
	@echo "  make mairu-web          Start mairu web UI (expects binary)"
	@echo
	@echo "No Docker (self-contained Meilisearch fallback):"
	@echo "  make meili-up           Download/run local Meilisearch"
	@echo "  make meili-down         Stop local Meilisearch"
	@echo "  make meili-status       Show local Meilisearch status"
	@echo "  make meili-clean        Stop and wipe local Meilisearch data"
	@echo "  make setup-no-docker    Install deps, start local Meilisearch, setup indexes"
	@echo "  make dev-no-docker      Start local Meilisearch + dashboard"
	@echo "  make mairu-no-docker    Build mairu and run web UI with local Meilisearch"

install:
	bun install

install-dashboard:
	bun install --cwd mairu/ui

setup:
	$(MAKE) mairu-build
	./mairu/bin/mairu setup

build:
	$(MAKE) mairu-build

lint:
	$(MAKE) lint-go

test:
	$(MAKE) test-go

fmt-go:
	./mairu/scripts/go-dev.sh fmt

fmt-go-check:
	./mairu/scripts/go-dev.sh fmt-check

lint-go:
	./mairu/scripts/go-dev.sh lint

test-go:
	./mairu/scripts/go-dev.sh test

test-go-race:
	./mairu/scripts/go-dev.sh test-race

test-go-cover:
	./mairu/scripts/go-dev.sh coverage

check-go:
	./mairu/scripts/go-dev.sh check

check-go-ci:
	./mairu/scripts/go-dev.sh check-ci

install-hooks:
	./scripts/install-hooks.sh

clean:
	rm -rf mairu/bin

dashboard-api:
	$(MAKE) mairu-build
	./mairu/bin/mairu context-server -p 8788

dashboard-dev:
	bun run --cwd mairu/ui dev

dashboard:
	$(MAKE) mairu-build
	./mairu/bin/mairu context-server -p 8788 & MAIRU_CONTEXT_SERVER_URL=http://localhost:8788 ./mairu/bin/mairu web -p 8080 & bun run --cwd mairu/ui dev

mairu-build:
	mkdir -p mairu/bin
	go build -C mairu -o bin/mairu ./cmd/mairu

mairu-web:
	$(MAKE) mairu-build
	./mairu/bin/mairu web -p 8080

eval-retrieval:
	$(MAKE) mairu-build
	./mairu/bin/mairu eval:retrieval

eval-seed:
	$(MAKE) mairu-build
	./mairu/bin/mairu seed

meili-up:
	./mairu/scripts/meili-local.sh up

meili-down:
	./mairu/scripts/meili-local.sh down

meili-status:
	./mairu/scripts/meili-local.sh status

meili-clean:
	./mairu/scripts/meili-local.sh clean

setup-no-docker: install install-dashboard meili-up setup

dev-no-docker: meili-up
	$(MAKE) dashboard

mairu-no-docker: meili-up mairu-build
	./mairu/bin/mairu web -p 8080

eval-llm:
	cd llmeval && go build -o bin/llmeval ./cmd/llmeval
	./llmeval/bin/llmeval --dataset ./llmeval/sample_dataset.json --model gemini-2.5-flash

eval-llm-vibe:
	cd llmeval && go build -o bin/llmeval ./cmd/llmeval
	./llmeval/bin/llmeval --dataset ./llmeval/mairu_vibe_mutation_eval.json --model gemini-2.5-flash

build-browser-extension:
	cd browser-extension && cargo build --release -p browser-extension-host
	@echo "Building WASM for browser extension... (requires wasm-pack)"
	cd browser-extension/crates/wasm && wasm-pack build --target web --out-dir ../../extension/pkg

install-browser-extension: build-browser-extension
	cd browser-extension && ./install.sh

test-browser-extension:
	cd browser-extension && cargo test
