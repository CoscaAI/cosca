# Knowledge Integration

> **Extracted from**: KERNEL.md section 16 | **Lines**: ~661 | **Date**: 2026-07-28

## Overview

Knowledge Integration connects the embed system, knowledge.db, and memory system into a unified knowledge store.

## Knowledge Sources

| Source | Format | Storage |
|--------|--------|---------|
| Embeds | Markdown | embed.FS (read-only) |
| Knowledge DB | SQLite FTS5 | .cosca/knowledge.db |
| Memory | Markdown | .cosca/memory/ |
| Vectors | Embeddings | .cosca/vectors/ |

## Knowledge Flow

```
Embed (.md files)
    │
    ▼
┌─────────────┐
│  Ingest     │  Parse, extract metadata
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Index      │  FTS5 + vector embeddings
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Store      │  SQLite + vectors
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Query      │  Search by text or semantic
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Rank       │  Relevance scoring
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Return     │  Top-K results
└─────────────┘
```

## Search Modes

| Mode | Method | Use Case |
|------|--------|----------|
| FTS5 | Full-text search | Exact keyword matching |
| Vector | Cosine similarity | Semantic search |
| Hybrid | FTS5 + Vector | Best of both |
| Graph | Relationship traversal | Related items |

## Knowledge Categories

| Category | Content | Example |
|----------|---------|---------|
| Identity | Who we are | CONSTITUTION.md |
| Architecture | How we're built | KERNEL.md |
| Patterns | What works | Learnings |
| Decisions | Why we chose | ADRs |
| Procedures | How to do | Protocols |
