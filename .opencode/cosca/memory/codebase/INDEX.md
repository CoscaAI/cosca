# Codebase Memory — Project Structure Map

> **Version**: 1.0.0 | **Status**: active | **Last Updated**: 2026-07-26

## Purpose
Complete map of the Cosca codebase. Every major directory, package, and file purpose is documented here so the Kernel can navigate instantly without filesystem discovery.

## Records

| Key | Scope | Lines |
|-----|-------|-------|
| [overview](overview.md) | Full directory tree with purpose annotations | ~200 |
| [go-packages](go-packages.md) | All 42+ internal Go packages with responsibilities | ~150 |
| [web-frontend](web-frontend.md) | Next.js 15 app structure (188 TSX/TS files) | ~120 |

## Usage
Load `overview.md` on session start (fast). Drill into `go-packages.md` or `web-frontend.md` when working on specific layers.
