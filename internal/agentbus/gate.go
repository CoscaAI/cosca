package agentbus

import (
	"context"
	"fmt"

	"github.com/CoscaAI/cosca/internal/permission"
)

// AgentMessagePermission is the permission namespace the boundary gate evaluates
// for every agent↔agent message. It parallels the way tools and subagent
// capabilities are gated (see internal/agents/subagent_permissions.go and
// permission.ToolPermission): one permission namespace, resolved against the
// target agent as the pattern.
//
// Configuration ergonomics: a single allow rule opens the boundary for one
// receiver, e.g. `agent_message:cosca-backend allow`; a global deny
// `agent_message:* deny` closes it; the fail-safe default (no rule) is "ask",
// which the PermissionGate treats as deny (fail-closed, I2).
const AgentMessagePermission = "agent_message"

// PermissionGate is the ADR-017 [P6] access front on the agent→agent boundary.
// It evaluates the AgentMessagePermission namespace against the destination
// agent id (as the target pattern) using the Cosca permission subsystem, so the
// same ruleset language governs tool access, subagent capability derivation,
// and cross-agent delivery. Resolution is deterministic and zero-LLM (I1).
//
// Fail-closed (I2): only an explicit `allow` verdict permits delivery. The
// engine's default "ask" verdict (no configured rule, unknown action, or an
// empty ruleset) is treated as deny.
type PermissionGate struct {
	rules permission.Ruleset
}

// NewPermissionGate returns a boundary gate backed by the given permission
// ruleset. An empty/nil ruleset denies everything (default-deny, I2). Rules use
// the AgentMessagePermission namespace, with the target agent id as the pattern.
func NewPermissionGate(rules permission.Ruleset) *PermissionGate {
	return &PermissionGate{rules: rules}
}

// Allow implements Gate. It permits the message only when the permission
// subsystem resolves AgentMessagePermission against the destination to `allow`.
func (g *PermissionGate) Allow(ctx context.Context, msg Message) error {
	rule := permission.Evaluate(AgentMessagePermission, string(msg.To), g.rules)
	if rule.Action != permission.Allow {
		return fmt.Errorf("%w: %s -> %s (%s)", ErrDenied, msg.From, msg.To, rule)
	}
	return nil
}

// AllowMessage is a convenience predicate that reports, without constructing a
// Message, whether the destination agent is permitted to receive a message from
// the given sender. It mirrors PermissionGate.Allow and is deterministic.
func (g *PermissionGate) AllowMessage(from, to AgentID) bool {
	rule := permission.Evaluate(AgentMessagePermission, string(to), g.rules)
	return rule.Action == permission.Allow
}
