# cosca-uiux — Reusable Patterns

> Discovered patterns that can be reapplied.

## Patterns Discovered

### Pattern 1: Cross-Surface UX Audit
**Context**: When asked to evaluate user experience of a software project.
**Method**: Run a simultaneous scan of all user-facing surfaces (CLI, Web, API docs).
**For CLI**: Check `--help`, output formatting, colors, completion, error messages, stub commands.
**For Web**: Check framework, responsive behavior, accessibility (WCAG), component library, theme system, loading/error/empty states.
**Then**: Map user personas, identify gaps, prioritize 3 recommendations.
**Why it works**: Gives a complete UX picture in a single pass. Catches surface-specific issues (e.g., CLI stub commands, web incomplete features).

### Pattern 2: Three-Persona Mapping
**Context**: Any project with diverse user types.
**Method**: Identify exactly 3 distinct personas:
1. **Developer** (power user, CLI-first, needs speed and precision)
2. **Admin** (web-first, needs monitoring and configuration)
3. **End User** (simplicity-first, needs guidance and mobile support)
**Then**: Map UX issues to the persona(s) they affect.
**Why it works**: Prevents designing for only one user type. Ensures CLI, web, and mobile get appropriate attention.
