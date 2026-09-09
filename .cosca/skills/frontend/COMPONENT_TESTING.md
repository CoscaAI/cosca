# Component Testing

> **Version**: 1.0.0 | **Status**: active | **Owner**: Frontend Chief | **Last Updated**: 2026-07-27

## Purpose
Test React components in isolation with all states covered.

## Process
1. Identify component states: loading, empty, error, success, edge cases.
2. Write Vitest + React Testing Library test per state.
3. Test user interactions: click, type, submit, navigate.
4. Test accessibility: role, label, keyboard navigation.
5. Test responsive behavior at breakpoints.
6. Run: `pnpm test -- --coverage`.

## Success Criteria
- All 4 states tested per component (loading/empty/error/success)
- Accessibility assertions pass
- Coverage > 80% for component files
