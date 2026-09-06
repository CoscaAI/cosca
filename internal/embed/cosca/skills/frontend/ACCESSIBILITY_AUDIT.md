# Accessibility Audit

> **Version**: 1.0.0 | **Status**: active | **Owner**: Frontend Chief | **Last Updated**: 2026-07-27

## Purpose
Audit frontend components for WCAG 2.2 AA+ compliance.

## Process
1. Run axe-core automated scan: `pnpm test:a11y`.
2. Manual keyboard navigation test (Tab, Enter, Escape, Arrow keys).
3. Screen reader test (VoiceOver/NVDA) for critical flows.
4. Color contrast verification (minimum 4.5:1 for text).
5. Focus management check (modals, dialogs, navigation).
6. Generate report with violations by severity.

## Success Criteria
- Zero critical a11y violations
- All interactive elements keyboard-accessible
- Focus trap works in modals
