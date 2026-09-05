package pipeline

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// ── ReBAC — Relationship-Based Access Control (OpenFGA Pattern #1) ──────
//
// ReBAC replaces binary JWT roles with relationship graphs.
// Access is determined by traversing relationships between entities.
//
// Example:
//   department:engineering → parent → agent:backend
//   agent:backend → can_execute → skill:code-generation
//   Check("user:don", "can_access", "skill:code-generation")
//     → resolves via BFS on relationship graph

// RelationType names a relationship between entities.
type RelationType string

const (
	RelOwner       RelationType = "owner"
	RelMember      RelationType = "member"
	RelParent      RelationType = "parent"
	RelCanExecute  RelationType = "can_execute"
	RelCanRead     RelationType = "can_read"
	RelCanWrite    RelationType = "can_write"
	RelCanApprove  RelationType = "can_approve"
	RelBelongsTo   RelationType = "belongs_to"
)

// Entity represents a node in the relationship graph.
type Entity struct {
	Type string // "user", "agent", "department", "skill", "resource", "project"
	ID   string
}

// String returns the canonical entity reference: type:id
func (e Entity) String() string { return e.Type + ":" + e.ID }

// Relation is an edge in the relationship graph.
type Relation struct {
	From   Entity
	Type   RelationType
	To     Entity
}

// ReBACGraph is an in-memory relationship graph with BFS traversal.
type ReBACGraph struct {
	mu    sync.RWMutex
	edges map[string][]Relation // from entity string → outgoing relations
}

// NewReBACGraph creates an empty relationship graph.
func NewReBACGraph() *ReBACGraph {
	return &ReBACGraph{edges: make(map[string][]Relation)}
}

// AddRelation adds a directed edge to the graph.
func (g *ReBACGraph) AddRelation(from Entity, rel RelationType, to Entity) {
	g.mu.Lock()
	defer g.mu.Unlock()
	key := from.String()
	g.edges[key] = append(g.edges[key], Relation{From: from, Type: rel, To: to})
}

// Check verifies if a relationship exists between two entities.
// Uses BFS with max 4 hops (configurable).
//
// Example:
//
//	g.Check("user:don", "can_access", "resource:config")
//	Traverses: user:don → owner → project:cosca → contains → resource:config
func (g *ReBACGraph) Check(from, relation, to string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	fromEntity := parseEntityRef(from)
	toEntity := parseEntityRef(to)

	return g.bfs(fromEntity, RelationType(relation), toEntity, 4)
}

// CheckWithContext is like Check but with context for cancellation.
func (g *ReBACGraph) CheckWithContext(_ context.Context, from, relation, to string) bool {
	return g.Check(from, relation, to)
}

// Userset expands a userset reference (e.g., "department:engineering#member")
// into the set of individual entities that are members.
func (g *ReBACGraph) Userset(ref string) []Entity {
	g.mu.RLock()
	defer g.mu.RUnlock()

	parts := strings.SplitN(ref, "#", 2)
	if len(parts) != 2 {
		// Not a userset — return as single entity
		return []Entity{parseEntityRef(ref)}
	}

	entityRef := parts[0]
	relationType := parts[1]
	entity := parseEntityRef(entityRef)

	return g.resolveUserset(entity, RelationType(relationType))
}

func (g *ReBACGraph) resolveUserset(entity Entity, rel RelationType) []Entity {
	var members []Entity
	key := entity.String()

	for _, r := range g.edges[key] {
		if r.Type == rel {
			members = append(members, r.To)
		}
	}
	return members
}

// bfs traverses the graph looking for a path from → to where every edge
// along the path matches targetRel. This prevents authorization bypass:
// Check("user:don", "can_access", "resource:config") only passes if a path
// of "can_access" edges exists, not if ANY relation chain reaches the target.
func (g *ReBACGraph) bfs(from Entity, targetRel RelationType, to Entity, maxHops int) bool {
	if from.String() == to.String() {
		return true // self-reference
	}

	visited := make(map[string]bool)
	queue := []struct {
		entity Entity
		hops   int
	}{{from, 0}}
	visited[from.String()] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.hops >= maxHops {
			continue
		}

		for _, rel := range g.edges[current.entity.String()] {
			if rel.Type != targetRel {
				continue
			}

			if rel.To.String() == to.String() {
				return true
			}

			if !visited[rel.To.String()] {
				visited[rel.To.String()] = true
				queue = append(queue, struct {
					entity Entity
					hops   int
				}{rel.To, current.hops + 1})
			}
		}
	}

	return false
}

// ── ReBAC Permission Policy ──────────────────────────────────────────────

// ReBACPolicy is a PermissionPolicy backed by a ReBAC graph.
type ReBACPolicy struct {
	graph *ReBACGraph
}

// NewReBACPolicy creates a policy from a ReBAC graph.
func NewReBACPolicy(graph *ReBACGraph) *PermissionPolicy {
	rbac := &ReBACPolicy{graph: graph}

	return &PermissionPolicy{
		Name: "rebac-graph",
		Rules: []PermissionRule{
			{
				Description: "ReBAC graph traversal: user → resource via relations",
				Evaluate: func(ctx context.Context, pc *PermissionContext) bool {
					userRef := "user:" + pc.User
					resourceRef := "resource:" + pc.Resource
					actionRel := actionToRelation(pc.Action)

					return rbac.graph.Check(userRef, actionRel, resourceRef)
				},
			},
			{
				Description: "membership: user belongs to department that owns resource",
				Evaluate: func(ctx context.Context, pc *PermissionContext) bool {
					userRef := "user:" + pc.User
					// Check if user belongs to any group that has access
					userEntity := parseEntityRef(userRef)
					groups := rbac.graph.resolveUserset(userEntity, RelMember)
					for _, group := range groups {
						resourceRef := "resource:" + pc.Resource
						if rbac.graph.Check(group.String(), string(RelOwner), resourceRef) {
							return true
						}
					}
					return false
				},
			},
		},
	}
}

// ── Default ReBAC Graph (Cosca built-in) ──────────────────────────────────

// DefaultReBACGraph creates the built-in relationship graph.
func DefaultReBACGraph() *ReBACGraph {
	g := NewReBACGraph()

	// Departments → Agents
	g.AddRelation(Entity{"department", "engineering"}, RelParent, Entity{"agent", "cosca-backend"})
	g.AddRelation(Entity{"department", "engineering"}, RelParent, Entity{"agent", "cosca-frontend"})
	g.AddRelation(Entity{"department", "engineering"}, RelParent, Entity{"agent", "cosca-testing"})
	g.AddRelation(Entity{"department", "security"}, RelParent, Entity{"agent", "cosca-security"})
	g.AddRelation(Entity{"department", "ai"}, RelParent, Entity{"agent", "cosca-kernel"})

	// Agents → Skills
	g.AddRelation(Entity{"agent", "cosca-backend"}, RelCanExecute, Entity{"skill", "api-design"})
	g.AddRelation(Entity{"agent", "cosca-backend"}, RelCanExecute, Entity{"skill", "code-generation"})
	g.AddRelation(Entity{"agent", "cosca-testing"}, RelCanExecute, Entity{"skill", "test-generation"})
	g.AddRelation(Entity{"agent", "cosca-security"}, RelCanExecute, Entity{"skill", "security-audit"})
	g.AddRelation(Entity{"agent", "cosca-kernel"}, RelCanExecute, Entity{"skill", "orchestration"})

	// Users → Departments
	g.AddRelation(Entity{"user", "don"}, RelOwner, Entity{"department", "engineering"})
	g.AddRelation(Entity{"user", "don"}, RelOwner, Entity{"department", "security"})
	g.AddRelation(Entity{"user", "don"}, RelOwner, Entity{"department", "ai"})
	g.AddRelation(Entity{"user", "admin"}, RelMember, Entity{"department", "engineering"})
	g.AddRelation(Entity{"user", "editor"}, RelMember, Entity{"department", "engineering"})

	return g
}

// ── Helpers ──────────────────────────────────────────────────────────────

func parseEntityRef(ref string) Entity {
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) == 2 {
		return Entity{Type: parts[0], ID: parts[1]}
	}
	return Entity{Type: "unknown", ID: ref}
}

func actionToRelation(action PermissionAction) string {
	switch action {
	case ActionPlanExecute:
		return string(RelCanExecute)
	case ActionPlanRead:
		return string(RelCanRead)
	case ActionPlanApprove:
		return string(RelCanApprove)
	default:
		return string(RelCanRead)
	}
}

// ── ReBAC Check API (OpenFGA-compatible) ──────────────────────────────────

// CheckRequest mirrors OpenFGA's Check API.
type CheckRequest struct {
	User     string `json:"user"`     // "user:don"
	Relation string `json:"relation"` // "can_access"
	Object   string `json:"object"`   // "resource:config"
}

// CheckResponse mirrors OpenFGA's Check API response.
type CheckResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

// CheckAccess evaluates a ReBAC check request.
func CheckAccess(graph *ReBACGraph, req CheckRequest) CheckResponse {
	allowed := graph.Check(req.User, req.Relation, req.Object)
	if allowed {
		return CheckResponse{Allowed: true, Reason: "relationship path found"}
	}
	return CheckResponse{Allowed: false, Reason: fmt.Sprintf("no path from %s to %s via %s", req.User, req.Object, req.Relation)}
}
