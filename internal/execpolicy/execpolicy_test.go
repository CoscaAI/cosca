package execpolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPolicyYAML = `
version: "1"
resolve_host_executables: true
rules:
  - pattern: ["git", "reset", "--hard"]
    decision: forbidden
    justification: "operação destrutiva"
    match: [["git", "reset", "--hard"]]
    not_match: [["git", "reset", "--keep"]]
  - pattern: ["ls"]
    decision: allow
    match: [["ls", "-l"]]
  - pattern: ["cp"]
    decision: prompt
    match: [["cp", "foo", "bar"]]
`

func TestLoadYAMLParsesRules(t *testing.T) {
	p, err := LoadYAML([]byte(testPolicyYAML))
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	if p.Version != "1" {
		t.Errorf("version = %q, want %q", p.Version, "1")
	}
	if !p.ResolveHostExecutables {
		t.Error("resolve_host_executables = false, want true")
	}
	if len(p.Rules) != 3 {
		t.Fatalf("len(rules) = %d, want 3", len(p.Rules))
	}
	r := p.Rules[0]
	if r.Decision != Forbidden {
		t.Errorf("rule[0].decision = %q, want %q", r.Decision, Forbidden)
	}
	if r.Justification != "operação destrutiva" {
		t.Errorf("rule[0].justification = %q", r.Justification)
	}
	if len(r.Pattern) != 3 {
		t.Fatalf("rule[0] pattern positions = %d, want 3", len(r.Pattern))
	}
	for i, want := range []string{"git", "reset", "--hard"} {
		if got := r.Pattern[i][0]; got != want {
			t.Errorf("rule[0].pattern[%d] = %q, want %q", i, got, want)
		}
	}
}

func TestEvaluateExactMatch(t *testing.T) {
	p, err := LoadYAML([]byte(testPolicyYAML))
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	d, j := p.Evaluate([]string{"ls", "-l"})
	if d != Allow {
		t.Errorf("decision = %q, want %q", d, Allow)
	}
	if j != "" {
		t.Errorf("justification = %q, want empty", j)
	}
}

func TestEvaluateAlternatives(t *testing.T) {
	yaml := `
rules:
  - pattern: [["rm", "del"], "-rf"]
    decision: forbidden
    justification: "forçar remoção"
    match: [["rm", "-rf"], ["del", "-rf"]]
    not_match: [["rm", "-i"]]
`
	p, err := LoadYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	for _, argv := range [][]string{{"rm", "-rf"}, {"del", "-rf"}} {
		d, j := p.Evaluate(argv)
		if d != Forbidden {
			t.Errorf("Evaluate(%v) = %q, want forbidden", argv, d)
		}
		if j != "forçar remoção" {
			t.Errorf("Evaluate(%v) justification = %q", argv, j)
		}
	}
}

func TestEvaluateForbiddenWithJustification(t *testing.T) {
	p, err := LoadYAML([]byte(testPolicyYAML))
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	d, j := p.Evaluate([]string{"git", "reset", "--hard"})
	if d != Forbidden {
		t.Errorf("decision = %q, want %q", d, Forbidden)
	}
	if j != "operação destrutiva" {
		t.Errorf("justification = %q, want %q", j, "operação destrutiva")
	}
}

func TestEvaluatePrompt(t *testing.T) {
	p, err := LoadYAML([]byte(testPolicyYAML))
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	d, _ := p.Evaluate([]string{"cp", "foo", "bar"})
	if d != Prompt {
		t.Errorf("decision = %q, want %q", d, Prompt)
	}
}

func TestEvaluateNoMatchDefaultsAllow(t *testing.T) {
	p, err := LoadYAML([]byte(testPolicyYAML))
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	d, j := p.Evaluate([]string{"echo", "hi"})
	if d != Allow {
		t.Errorf("decision = %q, want %q", d, Allow)
	}
	if j != "" {
		t.Errorf("justification = %q, want empty", j)
	}

	// A nil/empty policy also defaults to allow.
	var empty *Policy
	if d, _ := empty.Evaluate([]string{"git"}); d != Allow {
		t.Errorf("nil policy Evaluate = %q, want allow", d)
	}
}

func TestEvaluateDecisionDefaultsToAllow(t *testing.T) {
	p, err := LoadYAML([]byte(`
rules:
  - pattern: ["mytool"]
    match: [["mytool", "--go"]]
`))
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	d, _ := p.Evaluate([]string{"mytool", "--go"})
	if d != Allow {
		t.Errorf("decision = %q, want default allow", d)
	}
}

func TestEvaluatePrefixMatchBlocksLongerCommand(t *testing.T) {
	p, err := LoadYAML([]byte(testPolicyYAML))
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	d, _ := p.Evaluate([]string{"git", "reset", "--hard", "--force"})
	if d != Forbidden {
		t.Errorf("decision = %q, want forbidden for longer command", d)
	}
}

func TestValidateCatchesViolations(t *testing.T) {
	yaml := `
rules:
  - pattern: ["git", "reset", "--hard"]
    decision: forbidden
    match: [["git", "status"]]
    not_match: [["git", "reset", "--hard"]]
`
	_, err := LoadYAML([]byte(yaml))
	if err == nil {
		t.Fatal("LoadYAML succeeded, want validation error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "match example 0") {
		t.Errorf("error does not mention bad match example: %v", msg)
	}
	if !strings.Contains(msg, "not_match example 0") {
		t.Errorf("error does not mention bad not_match example: %v", msg)
	}
}

func TestValidateInvalidDecision(t *testing.T) {
	_, err := LoadYAML([]byte(`
rules:
  - pattern: ["tool"]
    decision: maybe
    match: [["tool"]]
`))
	if err == nil || !strings.Contains(err.Error(), "invalid decision") {
		t.Fatalf("want invalid-decision error, got: %v", err)
	}
}

func TestValidateEmptyPattern(t *testing.T) {
	_, err := LoadYAML([]byte(`
rules:
  - decision: allow
    match: [["tool"]]
`))
	if err == nil || !strings.Contains(err.Error(), "pattern must not be empty") {
		t.Fatalf("want empty-pattern error, got: %v", err)
	}
}

func TestBasenameFallback(t *testing.T) {
	p, err := LoadYAML([]byte(testPolicyYAML))
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	// Usa um caminho absoluto de plataforma cujo basename é "git". O literal
	// POSIX "/usr/bin/git" não é filepath.IsAbs no Windows, o que impedia o
	// fallback de disparar nesta plataforma.
	execPath := filepath.Join(t.TempDir(), "git")
	if !filepath.IsAbs(execPath) {
		t.Fatalf("test fixture path %q should be absolute", execPath)
	}
	d, j := p.Evaluate([]string{execPath, "reset", "--hard"})
	if d != Forbidden {
		t.Errorf("decision = %q, want forbidden via basename fallback", d)
	}
	if j != "operação destrutiva" {
		t.Errorf("justification = %q", j)
	}

	// With ResolveHostExecutables disabled there is no fallback.
	p2, err := LoadYAML([]byte(strings.Replace(testPolicyYAML, "resolve_host_executables: true", "resolve_host_executables: false", 1)))
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	if d, _ := p2.Evaluate([]string{execPath, "reset", "--hard"}); d != Allow {
		t.Errorf("decision without fallback = %q, want allow", d)
	}
}

func TestLoadValidatesExamples(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(path, []byte(testPolicyYAML), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	p, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(p.Rules) != 3 {
		t.Errorf("len(rules) = %d, want 3", len(p.Rules))
	}

	// A policy whose examples violate the rule must fail to load.
	badPath := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(badPath, []byte(`
rules:
  - pattern: ["ls"]
    match: [["rm", "-rf"]]
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := Load(badPath); err == nil {
		t.Error("Load succeeded for policy with violating match example")
	}

	// Missing file must error.
	if _, err := Load(filepath.Join(dir, "missing.yaml")); err == nil {
		t.Error("Load succeeded for missing file")
	}
}

func TestTokenize(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"git reset --hard", []string{"git", "reset", "--hard"}},
		{"  ls  -l  ", []string{"ls", "-l"}},
		{"cp foo bar", []string{"cp", "foo", "bar"}},
		{"/usr/bin/git status", []string{"/usr/bin/git", "status"}},
		{`echo "hello world"`, []string{"echo", "hello world"}},
		{`echo 'single quoted'`, []string{"echo", "single quoted"}},
		{`echo escaped\ value`, []string{"echo", "escaped value"}},
		{`git status && ls`, []string{"git", "status", "&&", "ls"}},
		{``, nil},
		{"", []string{}},
	}
	for _, tc := range cases {
		got := Tokenize(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("Tokenize(%q) = %v, want %v", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("Tokenize(%q)[%d] = %q, want %q (full %v)", tc.in, i, got[i], tc.want[i], got)
			}
		}
	}
}

func TestEvaluateUsesTokenizedCommand(t *testing.T) {
	p, err := LoadYAML([]byte(testPolicyYAML))
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	d, _ := p.Evaluate(Tokenize("git reset --hard"))
	if d != Forbidden {
		t.Errorf("tokenized git reset --hard = %q, want forbidden", d)
	}
	d, _ = p.Evaluate(Tokenize("ls -l /tmp"))
	if d != Allow {
		t.Errorf("tokenized ls -l /tmp = %q, want allow", d)
	}
}
