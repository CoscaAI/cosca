package pipeline

import (
	"strings"
	"testing"
)

func TestOpClassifierClassify(t *testing.T) {
	c := NewOpClassifier()

	cases := []struct {
		cmd  string
		want OpLevel
	}{
		// Read
		{"git status", OpRead},
		{"ls -la", OpRead},
		{"cat file.txt", OpRead},
		{"grep -r foo .", OpRead},
		// Write
		{"git add .", OpWrite},
		{"mkdir -p build", OpWrite},
		{"touch notes.txt", OpWrite},
		// Redirection classifies as dangerous (matches "> " before write).
		{"echo hello > out.txt", OpDangerous},
		// Dangerous
		{"rm /tmp/old.txt", OpDangerous},
		{"mv a b", OpDangerous},
		{"git reset HEAD~1", OpDangerous},
		{"chmod +x script.sh", OpDangerous},
		// System
		{"apt install curl", OpSystem},
		{"npm install -g pkg", OpSystem},
		{"systemctl restart nginx", OpSystem},
		{"docker compose up", OpSystem},
		// Irreversible
		{"rm -rf /tmp/x", OpIrreversible},
		{"git push --force origin main", OpIrreversible},
		{"DROP TABLE users", OpIrreversible},
		{"dd if=/dev/zero of=/dev/sda", OpIrreversible},
		{"shutdown now", OpIrreversible},
		// Case-insensitive + trim
		{"  RM -RF /  ", OpIrreversible},
		{"Git StAtUs", OpRead},
		// Unknown falls back to read
		{"some unknown command", OpRead},
	}

	for _, tc := range cases {
		if got := c.Classify(tc.cmd); got != tc.want {
			t.Errorf("Classify(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}

func TestOpClassifierRequireConfirmation(t *testing.T) {
	c := NewOpClassifier()

	levels := []struct {
		level     OpLevel
		required  bool
		autoBlock bool
		opName    string
	}{
		{OpRead, false, false, "read"},
		{OpWrite, false, false, "safe_write"},
		{OpModify, false, false, "modify"},
		{OpDangerous, true, false, "dangerous_write"},
		{OpSystem, true, false, "system"},
		{OpIrreversible, true, true, "irreversible"},
	}

	for _, tc := range levels {
		conf := c.RequireConfirmation(tc.level)
		if conf.Required != tc.required {
			t.Errorf("level %v: Required = %v, want %v", tc.level, conf.Required, tc.required)
		}
		if conf.AutoBlock != tc.autoBlock {
			t.Errorf("level %v: AutoBlock = %v, want %v", tc.level, conf.AutoBlock, tc.autoBlock)
		}
		if conf.Operation != tc.opName {
			t.Errorf("level %v: Operation = %q, want %q", tc.level, conf.Operation, tc.opName)
		}
		if conf.Reason == "" {
			t.Errorf("level %v: empty reason", tc.level)
		}
	}
}

func TestSecurityGateEvaluate(t *testing.T) {
	g := NewSecurityGate()

	// Read-only operations are allowed.
	conf, err := g.Evaluate("git log")
	if err != nil {
		t.Fatalf("Evaluate(read) unexpected error: %v", err)
	}
	if conf.AutoBlock {
		t.Fatal("read operation must not auto-block")
	}

	// Irreversible operations are blocked and recorded.
	_, err = g.Evaluate("rm -rf /tmp/x")
	if err == nil {
		t.Fatal("Evaluate(irreversible) expected error")
	}
	if !strings.Contains(err.Error(), "operation blocked") {
		t.Fatalf("unexpected error text: %v", err)
	}
	blocked := g.BlockedOperations()
	if len(blocked) != 1 || blocked[0] != "rm -rf /tmp/x" {
		t.Fatalf("BlockedOperations = %v, want [rm -rf /tmp/x]", blocked)
	}
}

func TestSecurityGateCanProceed(t *testing.T) {
	g := NewSecurityGate()

	if !g.CanProceed("ls") {
		t.Fatal("CanProceed(read) should be true")
	}
	if g.CanProceed("git push --force origin main") {
		t.Fatal("CanProceed(irreversible) should be false")
	}
}

func TestSecurityGateOverride(t *testing.T) {
	g := NewSecurityGate()

	// Irreversible + dangerous can be overridden (AutoBlock disabled).
	for _, op := range []string{"rm -rf /x", "rm old.txt", "git reset HEAD~1"} {
		conf, err := g.Override(op)
		if err != nil {
			t.Fatalf("Override(%q) unexpected error: %v", op, err)
		}
		if conf.AutoBlock {
			t.Fatalf("Override(%q): AutoBlock should be false after override", op)
		}
	}

	// Safe operations cannot be overridden.
	if _, err := g.Override("git add ."); err == nil {
		t.Fatal("Override(write) expected error")
	}
	if _, err := g.Override("ls"); err == nil {
		t.Fatal("Override(read) expected error")
	}
}

func TestSecurityGateBlockedOperationsIsolatedCopy(t *testing.T) {
	g := NewSecurityGate()
	_, _ = g.Evaluate("rm -rf /a")

	got := g.BlockedOperations()
	got[0] = "mutated"
	got2 := g.BlockedOperations()
	if got2[0] != "rm -rf /a" {
		t.Fatalf("BlockedOperations must return a copy, got %v", got2)
	}
}
