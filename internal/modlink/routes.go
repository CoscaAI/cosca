package modlink

import (
	"fmt"
	"strings"
)

// DefaultRoutes returns the production route registry of the Modular Semantic
// architecture (ADR-013 §3.2, FASE 1 — routing/scope). Each Module was audited:
// it matches a real segment of `documents.path` (e.g. /memory/, /adr/,
// /knowledge/, /docs/) OR a real `entity_type` (e.g. skill, pattern, bug,
// template, provider, plugin, workflow, agent, project).
//
// The registry is deliberately small and closed: every route is a deterministic
// trigger (agent phrase) mapping a query to the module that owns a capability.
// `Capability` follows `<module>.<name>`; `Priority` is 1 (uniform); `Conditions`
// is nil (no contextual pre-conditions in FASE 1).
func DefaultRoutes() []Route {
	return []Route{
		{Trigger: "memória do agente", Module: "memory", Capability: "memory.search", Priority: 1},
		{Trigger: "decisão arquitetural", Module: "adr", Capability: "adr.record", Priority: 1},
		{Trigger: "arquitetura", Module: "architecture", Capability: "architecture.overview", Priority: 1},
		{Trigger: "conhecimento", Module: "knowledge", Capability: "knowledge.search", Priority: 1},
		{Trigger: "documentação", Module: "docs", Capability: "docs.read", Priority: 1},
		{Trigger: "skills", Module: "skill", Capability: "skill.discover", Priority: 1},
		{Trigger: "padrão", Module: "pattern", Capability: "pattern.apply", Priority: 1},
		{Trigger: "bug", Module: "bug", Capability: "bug.triage", Priority: 1},
		{Trigger: "template", Module: "template", Capability: "template.use", Priority: 1},
		{Trigger: "provider", Module: "provider", Capability: "provider.select", Priority: 1},
		{Trigger: "plugin", Module: "plugin", Capability: "plugin.invoke", Priority: 1},
		{Trigger: "workflow", Module: "workflow", Capability: "workflow.execute", Priority: 1},
		{Trigger: "agentes", Module: "agent", Capability: "agent.route", Priority: 1},
		{Trigger: "projeto cosca", Module: "project", Capability: "project.context", Priority: 1},
	}
}

// ValidateRoutes checks a route registry for integrity. It returns an error if:
//   - any route has an empty Module, Capability or Trigger;
//   - two routes share the same canonical Trigger (duplicate triggers would make
//     the resolver ambiguous or silently merge domains).
//
// Validation is a pure function of the route set — the same input always yields
// the same error. It never mutates the routes.
func ValidateRoutes(routes []Route) error {
	seenTriggers := make(map[string]string, len(routes))
	for i, r := range routes {
		if strings.TrimSpace(r.Module) == "" {
			return fmt.Errorf("route[%d]: Module is required (module owns the capability)", i)
		}
		if strings.TrimSpace(r.Capability) == "" {
			return fmt.Errorf("route[%d] (%s): Capability is required (logical function of the module)", i, r.Module)
		}
		if strings.TrimSpace(r.Trigger) == "" {
			return fmt.Errorf("route[%d] (%s): Trigger is required (deterministic hook)", i, r.Module)
		}
		key := strings.Join(canonicalTokens(r.Trigger), " ")
		if prev, dup := seenTriggers[key]; dup {
			return fmt.Errorf("duplicate trigger %q (routes %q and %q) — triggers must be unique", r.Trigger, prev, r.Module)
		}
		seenTriggers[key] = r.Module
	}
	return nil
}

// RoutesByModule indexes a route registry by module name. Each module occurs at
// most once, so the map is 1:1 (a registry with duplicate modules is ambiguous;
// the last write wins deterministically in the input order). It is the convenient
// lookup used to validate a user-provided `--scope` against the known registry.
func RoutesByModule(routes []Route) map[string]Route {
	out := make(map[string]Route, len(routes))
	for _, r := range routes {
		out[r.Module] = r
	}
	return out
}
