# Changelog

All notable changes to this project will be documented in this file.

The format is inspired by [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- Configurable embedding strategy via `EMBEDDING_MODEL`, `EMBEDDING_DIM`, and `ALLOW_ZERO_EMBEDDINGS`.
- Hybrid ranking retrieval (vector + keyword + optional recency/importance signals).
- Retrieval evaluation harness with `Recall@K`, `MRR`, and latency metrics.
- `eval/dataset.example.json` template for benchmark datasets.
- Public release docs and repo hygiene files (`.gitignore`, `.env.example`, `CONTRIBUTING.md`, `LICENSE`).

### Changed
- Search APIs in `ContextManager` and `TursoVectorDB` now support richer option objects while preserving backward compatibility for `topK` + `threshold`.
- CLI and MCP search tools now expose hybrid retrieval filters/options.

## [1.0.0] - 2026-03-15

### Added
- Initial release with Turso-backed vector memory store.
- Support for memories, skills, and hierarchical context nodes.
- CLI commands for add/search/list/delete workflows.
- MCP server exposing context tools for agent clients.
- Svelte dashboard for data inspection.
