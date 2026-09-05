package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ── EvidenceStore ────────────────────────────────────────────────────────

func TestNewEvidenceStoreCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "evidence")
	s, err := NewEvidenceStore(dir)
	if err != nil {
		t.Fatalf("NewEvidenceStore: %v", err)
	}
	if s.dir != dir {
		t.Fatalf("dir = %q, want %q", s.dir, dir)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("directory not created: %v", err)
	}
}

func TestEvidenceStoreStoreAndLoad(t *testing.T) {
	s, err := NewEvidenceStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	e := &Evidence{Kind: "test_result", Status: "pass", Summary: "all green"}
	if err := s.Store(e); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if e.ID == "" {
		t.Fatal("Store must auto-generate an ID")
	}
	idRe := regexp.MustCompile(`^EV-\d{8}-[0-9A-Fa-f]{8}$`)
	if !idRe.MatchString(e.ID) {
		t.Fatalf("invalid generated ID %q", e.ID)
	}

	loaded, err := s.Load(e.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Kind != "test_result" || loaded.Status != "pass" || loaded.Summary != "all green" {
		t.Fatalf("roundtrip mismatch: %+v", loaded)
	}
}

func TestEvidenceStoreRejectsInvalidID(t *testing.T) {
	s, _ := NewEvidenceStore(t.TempDir())
	err := s.Store(&Evidence{ID: "not-an-evidence-id", Kind: "x"})
	if err == nil || !strings.Contains(err.Error(), "invalid ID") {
		t.Fatalf("expected invalid ID error, got %v", err)
	}
}

func TestEvidenceStoreComputesArtifactHash(t *testing.T) {
	s, _ := NewEvidenceStore(t.TempDir())
	artifact := filepath.Join(t.TempDir(), "report.txt")
	if err := os.WriteFile(artifact, []byte("evidence content"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := &Evidence{Kind: "file_diff", Status: "warning", Artifact: artifact}
	if err := s.Store(e); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if e.Hash == "" {
		t.Fatal("Store must compute SHA256 for artifacts")
	}
	if len(e.Hash) != 64 {
		t.Fatalf("hash length = %d, want 64", len(e.Hash))
	}

	// Hash matches VerifyHash.
	ok, err := e.VerifyHash()
	if err != nil || !ok {
		t.Fatalf("VerifyHash = %v, %v", ok, err)
	}
}

func TestEvidenceStoreLoadMissingAndList(t *testing.T) {
	s, _ := NewEvidenceStore(t.TempDir())

	if _, err := s.Load("EV-20260814-DEADBEEF"); err == nil {
		t.Fatal("Load(missing) expected error")
	}

	_ = s.Store(&Evidence{Kind: "a", Status: "pass"})
	_ = s.Store(&Evidence{Kind: "b", Status: "fail"})

	all, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("List len = %d, want 2", len(all))
	}
}

func TestEvidenceVerifyHash(t *testing.T) {
	// No artifact path → error.
	e := &Evidence{ID: "EV-20260814-DEADBEEF"}
	if _, err := e.VerifyHash(); err == nil {
		t.Fatal("VerifyHash without artifact expected error")
	}

	dir := t.TempDir()
	art := filepath.Join(dir, "f.txt")
	_ = os.WriteFile(art, []byte("data"), 0o644)

	good := &Evidence{ID: "EV-20260814-DEADBEEF", Artifact: art, Hash: sha256Hex("data")}
	ok, err := good.VerifyHash()
	if err != nil || !ok {
		t.Fatalf("VerifyHash(good) = %v, %v", ok, err)
	}

	bad := &Evidence{ID: "EV-20260814-DEADBEEF", Artifact: art, Hash: strings.Repeat("0", 64)}
	ok, _ = bad.VerifyHash()
	if ok {
		t.Fatal("VerifyHash(bad hash) should be false")
	}
}

// ── CheckpointStore ──────────────────────────────────────────────────────

func TestCheckpointStoreSaveLoadRoundtrip(t *testing.T) {
	s, err := NewCheckpointStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	cp := Checkpoint{
		PlanID:       "plan-1",
		LastSequence: 42,
		TaskStatuses: map[string]string{"t1": "completed", "t2": "running"},
	}
	if err := s.Save(cp); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := s.Load("plan-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.PlanID != "plan-1" || loaded.LastSequence != 42 {
		t.Fatalf("roundtrip mismatch: %+v", loaded)
	}
	if loaded.TaskStatuses["t1"] != "completed" || loaded.TaskStatuses["t2"] != "running" {
		t.Fatalf("task statuses mismatch: %+v", loaded.TaskStatuses)
	}
	// API contract: Load verifies the CRC32 integrity and then zeroes the
	// checksum field before returning (it is a transport detail, not state).
	if loaded.Checksum != 0 {
		t.Fatalf("Checksum must be zeroed on Load, got %d", loaded.Checksum)
	}
}

func TestCheckpointStoreDetectsCorruption(t *testing.T) {
	s, _ := NewCheckpointStore(t.TempDir())
	cp := Checkpoint{PlanID: "p", TaskStatuses: map[string]string{"t": "completed"}}
	if err := s.Save(cp); err != nil {
		t.Fatal(err)
	}

	// Corrupt the file.
	path := filepath.Join(s.dir, "p.checkpoint.json")
	if err := os.WriteFile(path, []byte(`{"plan_id":"p","task_statuses":{"t":"completed"},"checksum":1}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := s.Load("p")
	if err == nil {
		t.Fatal("Load(corrupted) expected checksum mismatch error")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckpointStoreLoadMissing(t *testing.T) {
	s, _ := NewCheckpointStore(t.TempDir())
	if _, err := s.Load("nope"); err == nil {
		t.Fatal("Load(missing) expected error")
	}
}

// ── HandoffStore ─────────────────────────────────────────────────────────

func TestHandoffStoreSaveLoadList(t *testing.T) {
	s, err := NewHandoffStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	art := &HandoffArtifact{
		FromAgent: "cosca-ceo",
		ToAgent:   "cosca-backend",
		Objective: "build the API",
		Changes:   []FileChange{{Path: "a.go", Action: "created", LinesAdded: 10}},
		Decisions: []Decision{{Question: "lang?", Choice: "Go", Reasoning: "team knows it"}},
	}
	if err := s.Save(art); err != nil {
		t.Fatalf("Save: %v", err)
	}
	idRe := regexp.MustCompile(`^HO-\d{8}-[0-9A-Fa-f]{8}$`)
	if !idRe.MatchString(art.ID) {
		t.Fatalf("invalid generated ID %q", art.ID)
	}
	if art.CreatedAt.IsZero() {
		t.Fatal("CreatedAt must be set")
	}

	loaded, err := s.Load(art.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.FromAgent != "cosca-ceo" || loaded.ToAgent != "cosca-backend" {
		t.Fatalf("roundtrip mismatch: %+v", loaded)
	}
	if len(loaded.Changes) != 1 || len(loaded.Decisions) != 1 {
		t.Fatalf("nested fields lost: %+v", loaded)
	}

	ids, err := s.List()
	if err != nil || len(ids) != 1 || ids[0] != art.ID {
		t.Fatalf("List = %v, %v", ids, err)
	}
	if s.Dir() == "" {
		t.Fatal("Dir() empty")
	}
}

func TestHandoffStoreRejectsInvalidID(t *testing.T) {
	s, _ := NewHandoffStore(t.TempDir())
	err := s.Save(&HandoffArtifact{ID: "bad-id"})
	if err == nil || !strings.Contains(err.Error(), "invalid artifact ID") {
		t.Fatalf("expected invalid ID error, got %v", err)
	}
}

func TestHandoffStoreLoadMissing(t *testing.T) {
	s, _ := NewHandoffStore(t.TempDir())
	if _, err := s.Load("HO-20260814-DEADBEEF"); err == nil {
		t.Fatal("Load(missing) expected error")
	}
}

func TestHandoffAndEvidenceIDFormats(t *testing.T) {
	idRe := regexp.MustCompile(`^HO-\d{8}-[0-9A-Fa-f]{8}$`)
	evRe := regexp.MustCompile(`^EV-\d{8}-[0-9A-Fa-f]{8}$`)
	for i := 0; i < 5; i++ {
		if !idRe.MatchString(NewHandoffID()) {
			t.Fatal("NewHandoffID produced invalid format")
		}
		if !evRe.MatchString(NewEvidenceID()) {
			t.Fatal("NewEvidenceID produced invalid format")
		}
	}
}

func TestFormatEvidenceReport(t *testing.T) {
	out := FormatEvidenceReport(nil, "done", "next")
	if !strings.Contains(out, "no evidence recorded") {
		t.Fatalf("empty report: %q", out)
	}

	evs := []Evidence{
		{ID: "EV-1", Kind: "test_result", Status: "pass", Summary: "ok"},
		{ID: "EV-2", Kind: "build_output", Status: "fail", Summary: "broken", Detail: strings.Repeat("d", 200), Source: "runner"},
		{ID: "EV-3", Kind: "lint", Status: "warning", Summary: "w", Artifact: "/tmp/x", Hash: "abc"},
	}
	out = FormatEvidenceReport(evs, "in-progress", "next-step")
	if !strings.Contains(out, "[PASS]") || !strings.Contains(out, "[FAIL]") || !strings.Contains(out, "[WARN]") {
		t.Fatal("missing status icons")
	}
	if !strings.Contains(out, "Task Status:  in-progress") || !strings.Contains(out, "Next Action:  next-step") {
		t.Fatal("missing header fields")
	}
	// Detail truncated to 120 + "...".
	if !strings.Contains(out, "...") {
		t.Fatal("long detail not truncated")
	}
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
