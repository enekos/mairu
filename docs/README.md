# Mairu Documentation

Welcome to the Mairu documentation. This repository contains detailed guides and reference materials for using, extending, and developing with Mairu.

## Core Concepts

- **ContextFS**: A hierarchical, graph-based context storage system.
- **Memories**: Fact-based storage for project knowledge, rules, and preferences.
- **Skills**: Capability definitions for agents.
- **Context Nodes**: URI-addressed nodes containing abstract, overview, and content fields (AST-based).
- **Browser Extension**: A Chrome extension (Rust/WASM) that syncs real-time web browsing context into Mairu.
- **Vibe Engine**: An LLM-powered engine for high-level queries and state mutations.
- **Daemon**: A background process for automatic codebase ingestion and AST extraction.

## Guides

- [Project Structure](PROJECT_STRUCTURE.md): Detailed breakdown of the repository.
- [Architecture Guide](ARCHITECTURE_GUIDE.md): Runtime architecture and major subsystems.
- [Specifications](specs): Design specs and implementation planning notes.

## Contributor Notes

- Keep top-level docs and CLI command examples in sync.
- Prefer commands rooted at repository root unless explicitly noted otherwise.

## License

ISC
