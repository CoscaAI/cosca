package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMCPPolicyDefaults(t *testing.T) {
	p := NewMCPPolicy()
	assert.False(t, p.Shell, "shell deve ser default-deny")
	assert.False(t, p.Network, "network deve ser default-deny")
	assert.False(t, p.FileWrite, "file-write deve ser default-deny")
	assert.Equal(t, DefaultMaxToolCalls, p.MaxToolCallsPerTurn, "orçamento default = 200")
	assert.True(t, p.AuditLog, "audit log default = true")
	assert.NotNil(t, p.AllowedTools)
	assert.Empty(t, p.AllowedTools, "allowlist inicialmente vazia (default-deny)")
}

func TestMCPPolicyEvaluate(t *testing.T) {
	cases := []struct {
		name     string
		tool     string
		args     map[string]any
		setup    func(*MCPPolicy)
		decision Decision
		wantErr  bool
	}{
		{
			name:     "tool não allowlistada é deny (default-deny)",
			tool:     "bash",
			args:     map[string]any{"command": "ls"},
			decision: Deny,
		},
		{
			name:     "tool none allowlistada é allow",
			tool:     "read",
			setup:    func(p *MCPPolicy) { p.AllowedTools["read"] = true },
			decision: Allow,
		},
		{
			name:     "shell default é deny mesmo allowlistada",
			tool:     "bash",
			setup:    func(p *MCPPolicy) { p.AllowedTools["bash"] = true },
			decision: Deny,
		},
		{
			name:     "shell com capability habilitada é allow",
			tool:     "bash",
			setup:    func(p *MCPPolicy) { p.AllowedTools["bash"] = true; p.Shell = true },
			decision: Allow,
		},
		{
			name:     "network default é deny mesmo allowlistada",
			tool:     "curl",
			setup:    func(p *MCPPolicy) { p.AllowedTools["curl"] = true },
			decision: Deny,
		},
		{
			name:     "network com capability habilitada é allow",
			tool:     "curl",
			setup:    func(p *MCPPolicy) { p.AllowedTools["curl"] = true; p.Network = true },
			decision: Allow,
		},
		{
			name:     "file-write default é deny mesmo allowlistada",
			tool:     "write",
			setup:    func(p *MCPPolicy) { p.AllowedTools["write"] = true },
			decision: Deny,
		},
		{
			name:     "file-write com capability habilitada é allow",
			tool:     "write",
			setup:    func(p *MCPPolicy) { p.AllowedTools["write"] = true; p.FileWrite = true },
			decision: Allow,
		},
		{
			name:     "tool vazia é erro",
			tool:     "   ",
			decision: Deny,
			wantErr:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewMCPPolicy()
			if tc.setup != nil {
				tc.setup(p)
			}
			got, err := p.Evaluate(tc.tool, tc.args)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.decision, got)
		})
	}
}

func TestMCPPolicyNilReceiver(t *testing.T) {
	var p *MCPPolicy
	_, err := p.Evaluate("read", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil receiver")
}

func TestMCPPolicyToolCapability(t *testing.T) {
	cases := []struct {
		tool string
		want Capability
	}{
		{"bash", CapShell},
		{"powershell", CapShell},
		{"EXEC", CapShell},
		{"curl", CapNetwork},
		{"http", CapNetwork},
		{"write", CapFileWrite},
		{"edit", CapFileWrite},
		{"read", CapNone},
		{"glob", CapNone},
		{"desconhecida", CapNone},
	}
	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			assert.Equal(t, tc.want, toolCapability(tc.tool))
		})
	}
}
