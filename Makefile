SHELL := /bin/bash

.DEFAULT_GOAL := help

.PHONY: help install install-dashboard setup build lint test clean dashboard dashboard-api dashboard-dev mairu-build mairu-web
.PHONY: fmt-go fmt-go-check lint-go test-go test-go-race test-go-cover check-go check-go-ci install-hooks
.PHONY: eval-retrieval eval-seed
.PHONY: meili-up meili-down meili-status meili-clean setup-no-docker dev-no-docker mairu-no-docker

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
	./mairu/bin/mairu-agent setup

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
	./mairu/bin/mairu-agent context-server -p 8788

dashboard-dev:
	bun run --cwd mairu/ui dev

dashboard:
	$(MAKE) mairu-build
	./mairu/bin/mairu-agent context-server -p 8788 & bun run --cwd mairu/ui dev

mairu-build:
	mkdir -p mairu/bin
	go build -C mairu -o bin/mairu-agent ./cmd/mairu

mairu-web:
	$(MAKE) mairu-build
	./mairu/bin/mairu-agent web -p 8080

eval-retrieval:
	$(MAKE) mairu-build
	./mairu/bin/mairu-agent eval:retrieval

eval-seed:
	$(MAKE) mairu-build
	./mairu/bin/mairu-agent seed

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
	./mairu/bin/mairu-agent web -p 8080
