---
name: react-native-audit
description: Use when the user asks to audit a React Native app for performance, native/build settings, and best practices.
---

# React Native Audit

> **Version**: 1.0.0 | **Status**: active | **Owner**: Mobile Chief | **Last Updated**: 2026-07-27

## Purpose
Audit React Native applications for performance, platform compliance, and best practices.

## Process
1. Performance audit: JS frame rate, bridge traffic, memory usage, bundle size.
2. Platform compliance: iOS (App Store guidelines), Android (Play Store requirements).
3. UI audit: platform-specific components used correctly, responsive layout, safe areas.
4. Navigation audit: deep linking, back button behavior, tab bar conventions.
5. Offline audit: local storage strategy, sync conflict resolution, cache invalidation.
6. Accessibility: screen reader support, touch targets, color contrast.

## Success Criteria
- 60fps maintained during animations and scrolling
- Zero platform-specific guideline violations
- Offline mode works with data sync on reconnect
