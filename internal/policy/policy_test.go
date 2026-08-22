// Package policy — testes do guard determinístico (L366).
package policy

import "testing"

func eval(t *testing.T, tool string, args map[string]interface{}) Evaluation {
	t.Helper()
	e := New()
	return e.Evaluate(NewAction(tool, args))
}

func TestPolicyDestructive(t *testing.T) {
	cases := []struct {
		name     string
		tool     string
		cmd      string
		decision Decision
	}{
		{"rm -rf", "bash", "rm -rf /tmp/x", Deny},
		{"drop table", "bash", "DROP TABLE vectors", Deny},
		{"delete from", "bash", "DELETE FROM cache", Deny},
		{"git reset hard", "bash", "git reset --hard HEAD", Deny},
		{"pkill", "bash", "pkill cosca", Deny},
		{"killall", "bash", "killall serve", Deny},
		{"kill term", "bash", "kill -TERM 1234", Allow}, // L56: o correto
		{"ls", "bash", "ls -la", Allow},
		{"go test", "bash", "go test ./...", Allow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := eval(t, tc.tool, map[string]interface{}{"command": tc.cmd})
			if ev.Decision != tc.decision {
				t.Errorf("%q: got %v (%s), want %v", tc.cmd, ev.Decision, ev.Rule, tc.decision)
			}
		})
	}
}

func TestPolicyEmbedWrite(t *testing.T) {
	// Escrita no cérebro (P8) → CONFIRM.
	ev := eval(t, "write", map[string]interface{}{"path": "internal/embed/cosca/KERNEL.md", "content": "x"})
	if ev.Decision != Confirm {
		t.Errorf("embed write: got %v, want CONFIRM (P8)", ev.Decision)
	}
	// Registro de memória (blocks) → permitido (fluxo normal).
	ev2 := eval(t, "write", map[string]interface{}{"path": "internal/embed/cosca/memory/agent/cosca-kernel/blocks/abc.md"})
	if ev2.Decision == Deny || ev2.Decision == Confirm {
		t.Errorf("blocks write: got %v, want permitido", ev2.Decision)
	}
}

func TestPolicyProductionTransform(t *testing.T) {
	// Transformação de dados reais sem backup → CONFIRM (L351).
	ev := eval(t, "bash", map[string]interface{}{"command": "sqlite3 .cosca/knowledge.db \"DELETE FROM vectors\""})
	if ev.Decision == Allow {
		t.Errorf("transform produção: got ALLOW, want CONFIRM/DENY")
	}
}

func TestPolicyReadOnly(t *testing.T) {
	for _, tool := range []string{"read", "glob", "grep", "search", "status"} {
		ev := eval(t, tool, map[string]interface{}{})
		if ev.Decision != Allow {
			t.Errorf("leitura %q: got %v, want ALLOW", tool, ev.Decision)
		}
	}
}

func TestPolicyDefaultAllow(t *testing.T) {
	ev := eval(t, "write", map[string]interface{}{"path": "docs/relatorio.md", "content": "x"})
	if ev.Decision != Allow {
		t.Errorf("escrita comum: got %v, want ALLOW", ev.Decision)
	}
	ev2 := eval(t, "bash", map[string]interface{}{"command": "go build ./..."})
	if ev2.Decision != Allow {
		t.Errorf("build: got %v, want ALLOW", ev2.Decision)
	}
}
