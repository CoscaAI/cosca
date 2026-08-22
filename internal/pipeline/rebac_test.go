package pipeline

import (
	"context"
	"strings"
	"testing"
)

func TestReBACEntityString(t *testing.T) {
	e := Entity{Type: "user", ID: "don"}
	if e.String() != "user:don" {
		t.Fatalf("String = %q", e.String())
	}
}

func TestReBACDirectRelation(t *testing.T) {
	g := NewReBACGraph()
	g.AddRelation(Entity{"user", "don"}, RelOwner, Entity{"project", "cosca"})

	if !g.Check("user:don", string(RelOwner), "project:cosca") {
		t.Fatal("direct relation must pass")
	}
	if g.Check("user:alice", string(RelOwner), "project:cosca") {
		t.Fatal("unrelated user must fail")
	}
}

func TestReBACMultiHop(t *testing.T) {
	g := NewReBACGraph()
	// BFS follows ONLY edges matching the target relation (no relation-type
	// mixing): a chain of can_execute edges from user to skill.
	g.AddRelation(Entity{"user", "don"}, RelCanExecute, Entity{"group", "eng"})
	g.AddRelation(Entity{"group", "eng"}, RelCanExecute, Entity{"agent", "backend"})
	g.AddRelation(Entity{"agent", "backend"}, RelCanExecute, Entity{"skill", "api"})

	if !g.Check("user:don", string(RelCanExecute), "skill:api") {
		t.Fatal("3-hop can_execute path must pass")
	}
	// Wrong relation type → no bypass even if the same nodes connect.
	if g.Check("user:don", string(RelCanWrite), "skill:api") {
		t.Fatal("wrong relation must fail (no relation-type bypass)")
	}
}

func TestReBACMaxHops(t *testing.T) {
	g := NewReBACGraph()
	// Chain longer than 4 hops → BFS gives up.
	prev := Entity{"user", "a"}
	for i := 0; i < 6; i++ {
		next := Entity{"node", string(rune('a' + i + 1))}
		g.AddRelation(prev, RelParent, next)
		prev = next
	}
	if g.Check("user:a", string(RelParent), "node:g") {
		t.Fatal("path beyond 4 hops must fail")
	}
}

func TestReBACSelfReference(t *testing.T) {
	g := NewReBACGraph()
	if !g.Check("user:x", "anything", "user:x") {
		t.Fatal("self-reference must pass")
	}
}

func TestReBACCheckWithContext(t *testing.T) {
	g := NewReBACGraph()
	g.AddRelation(Entity{"user", "don"}, RelCanRead, Entity{"resource", "config"})
	if !g.CheckWithContext(context.Background(), "user:don", string(RelCanRead), "resource:config") {
		t.Fatal("CheckWithContext must match Check")
	}
}

func TestReBACUserset(t *testing.T) {
	g := NewReBACGraph()
	g.AddRelation(Entity{"department", "eng"}, RelMember, Entity{"user", "alice"})
	g.AddRelation(Entity{"department", "eng"}, RelMember, Entity{"user", "bob"})

	members := g.Userset("department:eng#member")
	if len(members) != 2 {
		t.Fatalf("members = %v", members)
	}

	// Non-userset reference returns the single entity.
	single := g.Userset("user:don")
	if len(single) != 1 || single[0].ID != "don" {
		t.Fatalf("single = %v", single)
	}
}

func TestReBACParseEntityRef(t *testing.T) {
	e := parseEntityRef("user:don")
	if e.Type != "user" || e.ID != "don" {
		t.Fatalf("parse = %+v", e)
	}
	e2 := parseEntityRef("no-colon")
	if e2.Type != "unknown" || e2.ID != "no-colon" {
		t.Fatalf("parse fallback = %+v", e2)
	}
}

func TestActionToRelation(t *testing.T) {
	if actionToRelation(ActionPlanExecute) != string(RelCanExecute) {
		t.Fatal("execute mapping")
	}
	if actionToRelation(ActionPlanApprove) != string(RelCanApprove) {
		t.Fatal("approve mapping")
	}
	if actionToRelation("unknown") != string(RelCanRead) {
		t.Fatal("default mapping must be read")
	}
}

func TestReBACPolicy(t *testing.T) {
	g := NewReBACGraph()
	// Direct: user can_execute a resource.
	g.AddRelation(Entity{"user", "don"}, RelCanExecute, Entity{"resource", "api-design"})
	// Membership: user belongs to a group that owns a resource.
	g.AddRelation(Entity{"user", "alice"}, RelMember, Entity{"group", "eng"})
	g.AddRelation(Entity{"group", "eng"}, RelOwner, Entity{"resource", "config"})

	policy := NewReBACPolicy(g)
	ctx := context.Background()

	ok, reason := policy.Evaluate(ctx, &PermissionContext{User: "don", Resource: "api-design", Action: ActionPlanExecute})
	if !ok {
		t.Fatalf("don should execute via graph: %v", reason)
	}

	// Membership rule: alice (member of eng) can access config owned by eng.
	ok, _ = policy.Evaluate(ctx, &PermissionContext{User: "alice", Resource: "config", Action: ActionPlanRead})
	if !ok {
		t.Fatal("membership rule should allow alice")
	}

	// No path → denied.
	ok, _ = policy.Evaluate(ctx, &PermissionContext{User: "ghost", Resource: "config", Action: ActionPlanRead})
	if ok {
		t.Fatal("ghost must be denied")
	}
}

func TestReBACCheckAccess(t *testing.T) {
	g := NewReBACGraph()
	g.AddRelation(Entity{"user", "don"}, RelCanRead, Entity{"resource", "config"})

	res := CheckAccess(g, CheckRequest{User: "user:don", Relation: string(RelCanRead), Object: "resource:config"})
	if !res.Allowed || !strings.Contains(res.Reason, "relationship path found") {
		t.Fatalf("allowed check: %+v", res)
	}

	res = CheckAccess(g, CheckRequest{User: "user:ghost", Relation: string(RelCanRead), Object: "resource:config"})
	if res.Allowed || !strings.Contains(res.Reason, "no path") {
		t.Fatalf("denied check: %+v", res)
	}
}
