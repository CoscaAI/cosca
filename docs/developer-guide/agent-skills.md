# Agent Skills Standard Support

> **Version**: 1.0.0 | **Last Updated**: 2026-08-11 | **Status**: active

Cosca implements the [Anthropic Agent Skills standard](https://agentskills.io/specification).
This means skills from the Anthropic ecosystem — any repo with a `skills/`
folder of `skill-name/SKILL.md` directories — can be installed and used by
Cosca, alongside the existing legacy Cosca skill format.

## The standard at a glance

A skill is a directory named after the skill, containing a required
`SKILL.md` plus optional resource directories:

```
web-scraper/
├── SKILL.md          # required: YAML frontmatter + markdown body (instructions)
├── scripts/          # optional: executable helpers, referenced from SKILL.md
├── references/       # optional: supporting documentation
└── assets/           # optional: non-code files
```

`SKILL.md` frontmatter fields supported by Cosca:

| Field | Required | Notes |
|-------|----------|-------|
| `name` | yes | 1-64 chars, lowercase alphanumeric + hyphens, must match the parent directory name |
| `description` | yes | 1-1024 chars; describes what + when to use |
| `license` | no | license name or reference to a bundled LICENSE |
| `compatibility` | no | 1-500 chars, environment requirements |
| `metadata` | no | arbitrary string→string map |
| `allowed-tools` | no | space-separated pre-approved tools, e.g. `Bash(git:*) Read` |

Progressive disclosure works as the standard intends:

1. **Metadata** (name, description) — loaded at startup for all skills; this is
   what `cosca skill list` / `cosca skill search` use.
2. **Instructions** (the `SKILL.md` body) — the activation layer, retrieved on
   demand via `cosca skill show <name>` / `GetInstructions`.
3. **Resources** (`scripts/`, `references/`, `assets/`) — discovered on load,
   read on demand via `GetResource`.

## Using ecosystem skills

`cosca skill install` accepts a source in the standard layout. The source may
be:

- a single skill directory containing `SKILL.md`, or
- a folder of `skill-name/SKILL.md` subdirectories (e.g. a cloned ecosystem
  repo's `skills/` folder).

Security note: like file sources, directory sources must live inside
`.cosca/skills` (the manager persists there and enforces containment). Copy
the skill(s) into `.cosca/skills/` first, then install:

```sh
# single skill directory
cp -r ~/claude-code/skills/web-scraper .cosca/skills/
cosca skill install web-scraper --source .cosca/skills/web-scraper

# a skills repo folder (installs every skill-name/SKILL.md subdirectory)
cosca skill install web-scraper --source .cosca/skills/my-repo
```

The installed skill is copied to `.cosca/skills/<name>/` and is picked up
automatically by the manager on the next load.

## Legacy Cosca skills (backward compatible)

Cosca's 87 framework skills still use the legacy format — flat `.md` files
with descriptive names and a blockquoted status line:

```
> **Version**: 1.0.0 | **Status**: active | **Owner**: API Chief
>
> # API DOCUMENTATION SKILL
```

These continue to parse and work exactly as before. They do not conform to the
standard's naming rules, so they surface as **warnings** (not errors) in
`cosca skill validate`.

## Validation

```sh
cosca skill validate            # per-skill violations + summary; exit 0
cosca skill validate --strict   # exit non-zero if any skill fails
```

## Migrating legacy skills

`cosca skill migrate` prints the frontmatter block that would make each legacy
skill spec-conformant; `--write` prepends it to local files. Embedded
framework skills are always shown read-only (they live in the compiled
binary).

```sh
cosca skill migrate             # dry-run: print frontmatter for all legacy skills
cosca skill migrate --write     # prepend frontmatter to local .cosca/skills files
```

Migration is deliberately non-destructive and can be run selectively: each
migrated file keeps its original body intact, and the parser handles
frontmatter, so migrated skills validate immediately.
