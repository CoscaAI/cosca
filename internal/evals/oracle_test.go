package evals

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestSubmitToOracleValidInvalid(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SubmitToOracle executa os comandos secret_verify via sh -c (shell POSIX) — sh ausente no Windows nativo")
	}
	dir := t.TempDir()

	// Passing secret verify.
	secret := &SecretCase{Case: "c1", SecretVerify: []string{"true"}}
	v, err := SubmitToOracle(context.Background(), dir, secret, FeedbackValidInvalid, time.Second)
	if err != nil {
		t.Fatalf("SubmitToOracle: %v", err)
	}
	if !v.Valid || v.Signal != "VALID" {
		t.Fatalf("expected VALID, got %+v", v)
	}
	if len(v.PerCommand) != 0 {
		t.Fatalf("valid_invalid feedback must not expose per-command results, got %d", len(v.PerCommand))
	}

	// Failing secret verify.
	secret.SecretVerify = []string{"false"}
	v, err = SubmitToOracle(context.Background(), dir, secret, FeedbackValidInvalid, time.Second)
	if err != nil {
		t.Fatalf("SubmitToOracle: %v", err)
	}
	if v.Valid || v.Signal != "INVALID" {
		t.Fatalf("expected INVALID, got %+v", v)
	}
}

func TestSubmitToOraclePassFail(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SubmitToOracle executa os comandos secret_verify via sh -c (shell POSIX) — sh ausente no Windows nativo")
	}
	dir := t.TempDir()
	secret := &SecretCase{Case: "c1", SecretVerify: []string{"true", "false", "true"}}

	v, err := SubmitToOracle(context.Background(), dir, secret, FeedbackPassFail, time.Second)
	if err != nil {
		t.Fatalf("SubmitToOracle: %v", err)
	}
	if v.Valid {
		t.Fatalf("expected not all valid (one false)")
	}
	if v.Signal != "t1=PASS t2=FAIL t3=PASS" {
		t.Fatalf("expected anonymous PASS/FAIL signal, got %q", v.Signal)
	}
	if len(v.PerCommand) != 3 {
		t.Fatalf("expected 3 per-command results, got %d", len(v.PerCommand))
	}
	// The signal must never leak the commands.
	for _, pc := range v.PerCommand {
		if pc.Command != "" {
			t.Fatalf("pass_fail feedback must not leak the command string, got %q", pc.Command)
		}
	}
}

func TestSubmissionLogAppendLoad(t *testing.T) {
	dir := t.TempDir()
	log := NewSubmissionLog(dir, "c1")

	idx1, err := log.Append(Submission{At: time.Now(), Valid: false, Signal: "INVALID"})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	idx2, err := log.Append(Submission{At: time.Now(), Valid: true, Signal: "VALID"})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if idx1 != 1 || idx2 != 2 {
		t.Fatalf("expected indexes 1,2, got %d,%d", idx1, idx2)
	}

	subs, err := log.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(subs) != 2 || subs[0].Valid || !subs[1].Valid {
		t.Fatalf("unexpected subs: %+v", subs)
	}

	// File should be 0600.
	path := filepath.Join(dir, ".cosca", "evals", "submissions", "c1.jsonl")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat log: %v", err)
	}
	// 0600 is a POSIX concept: on Windows the mode bits are not enforced and
	// a writable regular file reports 0666.
	if got := info.Mode().Perm(); got != 0o600 && !(runtime.GOOS == "windows" && got == 0o666) {
		t.Fatalf("expected 0600, got %o", got)
	}
}

func TestComputeDiscoveryMetrics(t *testing.T) {
	start := time.Now()
	subs := []Submission{
		{At: start.Add(5 * time.Second), Valid: false, Signal: "INVALID"},
		{At: start.Add(15 * time.Second), Valid: false, Signal: "INVALID"},
		{At: start.Add(30 * time.Second), Valid: true, Signal: "VALID"},
	}

	m := ComputeDiscoveryMetrics(subs, 7, 1.0, start)

	if m.NumHypotheses != 3 || m.NumDiscarded != 2 {
		t.Fatalf("expected 3 hypotheses / 2 discarded, got %d/%d", m.NumHypotheses, m.NumDiscarded)
	}
	if m.DiscardRate != 2.0/3.0 {
		t.Fatalf("expected discard rate 2/3, got %v", m.DiscardRate)
	}
	if m.TimeToSolution == "" {
		t.Fatalf("expected a time-to-solution")
	}
	// discovery_efficiency = reward / (hypotheses + experiments + 1) = 1/(3+7+1)
	want := 1.0 / 11.0
	if m.DiscoveryEfficiency != want {
		t.Fatalf("expected efficiency %v, got %v", want, m.DiscoveryEfficiency)
	}
}

func TestLoadSecretMissing(t *testing.T) {
	dir := t.TempDir()
	sc, err := LoadSecret(dir, "nonexistent")
	if err != nil {
		t.Fatalf("LoadSecret missing should not error, got %v", err)
	}
	if sc != nil {
		t.Fatalf("expected nil secret for missing case, got %+v", sc)
	}
}

func TestLoadSecretParses(t *testing.T) {
	dir := t.TempDir()
	secretDir := filepath.Join(dir, ".cosca", "evals", "secrets")
	if err := os.MkdirAll(secretDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "case: c1\nfeedback: pass_fail\nsecret_verify:\n  - \"true\"\n  - \"false\"\nreference_solution: \"hash:abc\"\n"
	if err := os.WriteFile(filepath.Join(secretDir, "c1.yaml"), []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	sc, err := LoadSecret(dir, "c1")
	if err != nil {
		t.Fatalf("LoadSecret: %v", err)
	}
	if sc == nil || sc.Case != "c1" || sc.Feedback != "pass_fail" {
		t.Fatalf("unexpected secret: %+v", sc)
	}
	if len(sc.SecretVerify) != 2 || sc.ReferenceSolution != "hash:abc" {
		t.Fatalf("unexpected secret fields: %+v", sc)
	}
}
