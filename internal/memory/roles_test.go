package memory

import "testing"

// TestRoleOf verifies exact-match and prefix role resolution, including the
// safe default.
func TestRoleOf(t *testing.T) {
	tests := []struct {
		name  string
		agent string
		want  Role
	}{
		{"don", "don", RoleDon},
		{"kernel", "cosca-kernel", RoleKernel},
		{"ceo", "cosca-ceo", RoleExecutive},
		{"cto", "cosca-cto", RoleExecutive},
		{"critic", "cosca-critic", RoleCritic},
		{"specialist backend", "cosca-specialist-backend", RoleSpecialist},
		{"specialist bare prefix", "cosca-specialist", RoleSpecialist},
		{"chief backend", "cosca-backend", RoleChief},
		{"chief architecture", "cosca-architecture", RoleChief},
		{"unknown agent", "unknown-agent", RoleChief},
		{"empty agent", "", RoleChief},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RoleOf(tt.agent); got != tt.want {
				t.Errorf("RoleOf(%q) = %q, want %q", tt.agent, got, tt.want)
			}
		})
	}
}

// TestDepartmentOf verifies domain extraction from agent names.
func TestDepartmentOf(t *testing.T) {
	tests := []struct {
		name  string
		agent string
		want  string
	}{
		{"specialist backend", "cosca-specialist-backend", "backend"},
		{"specialist nested", "cosca-specialist-frontend", "frontend"},
		{"chief backend", "cosca-backend", "backend"},
		{"chief architecture", "cosca-architecture", "architecture"},
		{"kernel", "cosca-kernel", ""},
		{"ceo", "cosca-ceo", ""},
		{"cto", "cosca-cto", ""},
		{"critic", "cosca-critic", ""},
		{"don", "don", ""},
		{"unknown agent", "unknown-agent", "unknown-agent"},
		{"empty agent", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DepartmentOf(tt.agent); got != tt.want {
				t.Errorf("DepartmentOf(%q) = %q, want %q", tt.agent, got, tt.want)
			}
		})
	}
}

// TestCanReadMemory is the full access-policy matrix for raw memory reads.
func TestCanReadMemory(t *testing.T) {
	tests := []struct {
		name   string
		reader string
		target string
		want   bool
	}{
		// Kernel: full visibility.
		{"kernel reads itself", "cosca-kernel", "cosca-kernel", true},
		{"kernel reads specialist", "cosca-kernel", "cosca-specialist-backend", true},
		{"kernel reads another chief", "cosca-kernel", "cosca-architecture", true},
		{"kernel reads executive", "cosca-kernel", "cosca-ceo", true},

		// Don: absolute authority, full access.
		{"don reads itself", "don", "don", true},
		{"don reads specialist", "don", "cosca-specialist-backend", true},
		{"don reads chief", "don", "cosca-backend", true},
		{"don reads kernel", "don", "cosca-kernel", true},

		// Executives: full visibility.
		{"ceo reads chief", "cosca-ceo", "cosca-backend", true},
		{"cto reads specialist", "cosca-cto", "cosca-specialist-backend", true},
		{"ceo reads kernel", "cosca-ceo", "cosca-kernel", true},

		// Critic: needs visibility to judge quality.
		{"critic reads anyone", "cosca-critic", "cosca-specialist-backend", true},
		{"critic reads chief", "cosca-critic", "cosca-backend", true},
		{"critic reads kernel", "cosca-critic", "cosca-kernel", true},

		// Specialist: own memory only.
		{"specialist reads itself", "cosca-specialist-backend", "cosca-specialist-backend", true},
		{"specialist reads another specialist", "cosca-specialist-backend", "cosca-specialist-frontend", false},
		{"specialist reads a chief", "cosca-specialist-backend", "cosca-backend", false},
		{"specialist reads kernel", "cosca-specialist-backend", "cosca-kernel", false},

		// Chief: own memory + specialists' failures (P5).
		{"chief reads own specialist", "cosca-backend", "cosca-specialist-backend", true},
		{"chief reads peer chief", "cosca-backend", "cosca-architecture", false},
		{"chief reads itself", "cosca-backend", "cosca-backend", true},
		{"chief reads kernel", "cosca-backend", "cosca-kernel", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanReadMemory(tt.reader, tt.target); got != tt.want {
				t.Errorf("CanReadMemory(%q, %q) = %v, want %v", tt.reader, tt.target, got, tt.want)
			}
		})
	}
}

// TestAgentFilterFor verifies the AgentFilter value each role may pass to
// Search (A7 scoping).
func TestAgentFilterFor(t *testing.T) {
	tests := []struct {
		name   string
		reader string
		want   string
	}{
		{"specialist scopes to own name", "cosca-specialist-backend", "cosca-specialist-backend"},
		{"chief full visibility", "cosca-backend", ""},
		{"kernel full visibility", "cosca-kernel", ""},
		{"executive full visibility", "cosca-ceo", ""},
		{"critic full visibility", "cosca-critic", ""},
		{"don full visibility", "don", ""},
		{"unknown defaults to chief full visibility", "unknown-agent", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AgentFilterFor(tt.reader); got != tt.want {
				t.Errorf("AgentFilterFor(%q) = %q, want %q", tt.reader, got, tt.want)
			}
		})
	}
}
