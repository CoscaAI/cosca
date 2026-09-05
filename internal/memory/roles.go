package memory

import "strings"

// Role is the hierarchical role of an agent in the Cosca family.
type Role string

const (
	// RoleDon is the Don — absolute authority, full access.
	RoleDon Role = "don"
	// RoleKernel is the consigliere — orchestrator, full visibility.
	RoleKernel Role = "kernel"
	// RoleExecutive is a CEO/CTO — command-level, full visibility.
	RoleExecutive Role = "executive"
	// RoleCritic is a reviewer — needs visibility to judge quality.
	RoleCritic Role = "critic"
	// RoleChief is a department head — own memory + specialists' failures.
	RoleChief Role = "chief"
	// RoleSpecialist is an executor — own memory only (raw), curated access via Phase 3.
	RoleSpecialist Role = "specialist"
)

// RoleOf resolves the hierarchical role of an agent. Exact-match table first,
// then prefix rules. The safe default is RoleChief (department chiefs plus
// anything unclassified).
func RoleOf(agent string) Role {
	switch agent {
	case "don":
		return RoleDon
	case "cosca-kernel":
		return RoleKernel
	case "cosca-ceo", "cosca-cto":
		return RoleExecutive
	case "cosca-critic":
		return RoleCritic
	}
	if strings.HasPrefix(agent, "cosca-specialist") {
		return RoleSpecialist
	}
	return RoleChief
}

// DepartmentOf extracts the domain from an agent name.
//   - "cosca-specialist-<domain>" → <domain>
//   - "cosca-<domain>" (chief) → <domain>
//   - kernel/executives/critic/don → ""
//   - anything else → the agent name as-is
func DepartmentOf(agent string) string {
	switch agent {
	case "don", "cosca-kernel", "cosca-ceo", "cosca-cto", "cosca-critic":
		return ""
	}
	if strings.HasPrefix(agent, "cosca-specialist-") {
		return strings.TrimPrefix(agent, "cosca-specialist-")
	}
	if strings.HasPrefix(agent, "cosca-") {
		return strings.TrimPrefix(agent, "cosca-")
	}
	return agent
}

// CanReadMemory is the access policy for raw memory reads. It answers whether
// reader may read target's raw memory records.
func CanReadMemory(reader, target string) bool {
	if reader == target {
		return true // one's own memory, always
	}
	switch RoleOf(reader) {
	case RoleDon, RoleKernel, RoleExecutive, RoleCritic:
		// Full visibility: the Don, the Kernel, executives, and the critic
		// need oversight.
		return true
	case RoleChief:
		if RoleOf(target) == RoleSpecialist {
			// P5: chiefs learn from specialists' failures — cross-agent
			// learning within the command line.
			return true
		}
		// Peer chiefs do NOT read each other's raw memory.
		return false
	case RoleSpecialist:
		// Specialists read only their own raw memory; cross-agent exposure
		// comes curated in Phase 3.
		return false
	default:
		return false
	}
}

// AgentFilterFor returns the AgentFilter value a reader may pass to Search.
//   - Don, Kernel, Executive, Critic, Chief → "" (full visibility — empty
//     disables scoping in the engine, per A7)
//   - Specialist → their own agent name (the engine returns own +
//     system/global records)
func AgentFilterFor(reader string) string {
	switch RoleOf(reader) {
	case RoleSpecialist:
		return reader
	default:
		return ""
	}
}
