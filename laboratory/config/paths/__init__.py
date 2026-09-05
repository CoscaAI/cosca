"""
COSCA Laboratory — Canonical Path Resolver
==========================================

Single source of truth for every laboratory path. ALL paths are derived from
``LAB_ROOT`` (PROJECT_ROOT / laboratory). Never hardcode a ``laboratory/...``
string in code — import and resolve from here instead.

Rules enforced (PASC-002):
  - Everything derives from ``LAB_ROOT``.
  - No absolute paths; no bare ``laboratory/...`` strings outside this module.
  - Paths are lazy (no side effects on import) and cheap to reuse.

Usage
-----
    from config.paths import Paths
    lab = Paths()
    lab.training.datasets / "run-001"
    lab.experiments.active
    lab.reports.training / "campaign-002.md"

Project root resolution (in priority order):
  1. ``COSCA_LAB_PROJECT_ROOT`` env var (explicit override).
  2. Git top-level of the current repo (``git rev-parse --show-toplevel``).
  3. Default: parent of ``internal`` if it exists, else current working dir.

The laboratory root can be moved by setting ``COSCA_LAB_ROOT`` (relative to
project root or absolute). Defaults to ``<project_root>/laboratory``.
"""

from __future__ import annotations

import os
from pathlib import Path


def _project_root() -> Path:
    """Resolve the repository/project root (no side effects)."""
    env = os.environ.get("COSCA_LAB_PROJECT_ROOT")
    if env:
        return Path(env).resolve()
    # Prefer git top-level (works from any subdirectory).
    try:
        import subprocess

        out = subprocess.run(
            ["git", "rev-parse", "--show-toplevel"],
            capture_output=True,
            text=True,
            timeout=3,
        )
        if out.returncode == 0 and out.stdout.strip():
            return Path(out.stdout.strip()).resolve()
    except Exception:
        pass
    # Fallback: CWD, or parent of internal/ if present.
    cwd = Path.cwd()
    candidate = cwd / "internal"
    if candidate.is_dir():
        return cwd.resolve()
    return cwd.resolve()


# Top-level root of the laboratory (project-relative by default).
LAB_ROOT: Path = Path(os.environ.get("COSCA_LAB_ROOT", "laboratory"))
if not LAB_ROOT.is_absolute():
    LAB_ROOT = _project_root() / LAB_ROOT
LAB_ROOT = LAB_ROOT.resolve()


# Convenience: every canonical subdirectory below LAB_ROOT.
_CANON = [
    "config/paths",
    "config/environments",
    "config/profiles",
    "training/datasets",
    "training/runs",
    "training/checkpoints",
    "training/adapters/lora",
    "training/exports",
    "training/manifests",
    "experiments/active",
    "experiments/completed",
    "experiments/archived",
    "tests/unit",
    "tests/integration",
    "tests/e2e",
    "tests/regression",
    "tests/benchmark",
    "tests/fixtures",
    "logs/training",
    "logs/tests",
    "logs/runtime",
    "logs/build",
    "logs/errors",
    "traces/sessions",
    "traces/execution",
    "traces/performance",
    "traces/failures",
    "reports/training",
    "reports/tests",
    "reports/benchmarks",
    "reports/security",
    "reports/experiments",
    "artifacts/builds",
    "artifacts/packages",
    "artifacts/models",
    "artifacts/temporary",
    "snapshots/before",
    "snapshots/after",
    "snapshots/known-good",
    "tmp/cache",
    "tmp/work",
    "tmp/staging",
]


class _Node:
    """Tiny path node with attribute access for sub-paths."""

    def __init__(self, path: Path) -> None:
        self._path = path

    def __truediv__(self, other: str) -> Path:
        return self._path / other

    def __str__(self) -> str:  # pragma: no cover - trivial
        return str(self._path)

    def __fspath__(self) -> str:  # pragma: no cover - trivial
        return str(self._path)


class Paths:
    """Canonical, derived laboratory paths.

    Each attribute returns another :class:`Paths` (or :class:`_Node`) so
    sub-paths chain naturally: ``paths.training.datasets``.
    """

    def __init__(self, base: Path | None = None) -> None:
        self._base = (base or LAB_ROOT)

    # --- top-level ---
    @property
    def root(self) -> Path:
        return self._base

    @property
    def config(self) -> "Paths":
        return Paths(self._base / "config")

    @property
    def training(self) -> "Paths":
        return Paths(self._base / "training")

    @property
    def experiments(self) -> "Paths":
        return Paths(self._base / "experiments")

    @property
    def tests(self) -> "Paths":
        return Paths(self._base / "tests")

    @property
    def logs(self) -> "Paths":
        return Paths(self._base / "logs")

    @property
    def traces(self) -> "Paths":
        return Paths(self._base / "traces")

    @property
    def reports(self) -> "Paths":
        return Paths(self._base / "reports")

    @property
    def artifacts(self) -> "Paths":
        return Paths(self._base / "artifacts")

    @property
    def snapshots(self) -> "Paths":
        return Paths(self._base / "snapshots")

    @property
    def tmp(self) -> "Paths":
        return Paths(self._base / "tmp")

    # --- a few commonly-used leaf accessors (chainable) ---
    @property
    def datasets(self) -> Path:
        return self._base / "datasets"

    @property
    def runs(self) -> Path:
        return self._base / "runs"

    @property
    def checkpoints(self) -> Path:
        return self._base / "checkpoints"

    @property
    def active(self) -> Path:
        return self._base / "active"

    @property
    def completed(self) -> Path:
        return self._base / "completed"

    @property
    def archived(self) -> Path:
        return self._base / "archived"

    @property
    def unit(self) -> Path:
        return self._base / "unit"

    @property
    def fixtures(self) -> Path:
        return self._base / "fixtures"

    @property
    def known_good(self) -> Path:
        return self._base / "known-good"

    def __truediv__(self, other: str) -> Path:
        return self._base / other

    def __str__(self) -> str:  # pragma: no cover - trivial
        return str(self._base)

    def __fspath__(self) -> str:  # pragma: no cover - trivial
        return str(self._base)


# Module-level singleton (cheap; no IO on construction).
default_paths = Paths()


def ensure_canonical_dirs(p: Paths | None = None) -> int:
    """Create all canonical laboratory directories. Returns count created.

    Idempotent and safe — never touches anything outside LAB_ROOT.
    """
    root = (p or default_paths).root
    created = 0
    for rel in _CANON:
        d = root / rel
        if not d.exists():
            d.mkdir(parents=True, exist_ok=True)
            created += 1
    return created


__all__ = [
    "LAB_ROOT",
    "Paths",
    "default_paths",
    "ensure_canonical_dirs",
]
