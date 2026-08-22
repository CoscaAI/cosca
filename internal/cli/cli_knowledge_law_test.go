//
// Tests for the `cosca knowledge law` command tree (internal/cli/knowledge_law.go).
//
// Covers:
//   - Registration of `law` (and its 4 subcommands) in the knowledge command
//   - `law list` with an existing empty laws.json → "nenhuma lei" (never an error)
//   - `law list` with persisted laws → table (ID | Título | Nível | Evidências | Confiança)
//   - Seed inicial: `law list` sem laws.json cria as 5 leis reais (K-01..K-05)
//     na primeira execução e não duplica na segunda (idempotente)
//   - `law add-evidence <id>` → adds evidence, promotes, persists to
//     .cosca/knowledge/laws.json (in a t.TempDir(), never the real .cosca)
//   - `law add-evidence` with --title → names the created law (default: the ID)
//   - `law add-evidence` with --confidence (0-1) → measured confidence allows
//     promotion to learning; out-of-range values are rejected
//   - `law show <id>` → full law + WhyExists() (the evidence narrative)
//   - `law approve <id>` → Don approval elevates the law to constitution
//   - JSON output for list/add-evidence/show/approve
//
// NOTE: tests that chdir() must NOT run in parallel — they mutate the
// process working directory.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/knowledge"
)

// chdirTemp muda para um diretório temporário e restaura o cwd no cleanup.
// Os testes do CKL persistem em <tmp>/.cosca/knowledge/laws.json.
func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Chdir registra o restore do cwd DEPOIS do RemoveAll do TempDir
	// (cleanups rodam em LIFO) — no Windows não dá para remover o
	// diretório que é o CWD do processo.
	t.Chdir(dir)
	return dir
}

// runLawCommand executa o RunE de um comando do CKL com formatter no buffer.
func runLawCommand(t *testing.T, cmd *cobra.Command, args []string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true) // noColor
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	err := cmd.RunE(cmd, args)
	return buf.String(), err
}

// lawStore espelha o layout JSON on-disk do PromotionEngine (versão + items).
type lawStore struct {
	Version int                       `json:"version"`
	Items   []knowledge.KnowledgeItem `json:"items"`
}

// readLawsFile lê o arquivo runtime de leis do diretório dado.
func readLawsFile(t *testing.T, dir string) []knowledge.KnowledgeItem {
	t.Helper()
	path := filepath.Join(dir, ".cosca", "knowledge", "laws.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read laws.json: %v", err)
	}
	var store lawStore
	if err := json.Unmarshal(data, &store); err != nil {
		t.Fatalf("unmarshal laws.json: %v", err)
	}
	if store.Items == nil {
		return []knowledge.KnowledgeItem{}
	}
	return store.Items
}

// seedLawsFile cria um arquivo runtime com as leis dadas usando o
// PromotionEngine. (Nome distinto de seedLaws — o seed real do CKL.)
func seedLawsFile(t *testing.T, dir string, items ...*knowledge.KnowledgeItem) {
	t.Helper()
	engine := knowledge.NewPromotionEngine()
	for _, it := range items {
		if err := engine.Register(it); err != nil {
			t.Fatalf("register: %v", err)
		}
	}
	if err := engine.Save(filepath.Join(dir, ".cosca", "knowledge", "laws.json")); err != nil {
		t.Fatalf("seed laws.json: %v", err)
	}
}

// =============================================================================
// Registration — `law` in the knowledge command
// =============================================================================

func TestKnowledgeLawCommand_RegisteredInKnowledge(t *testing.T) {
	cmd := NewKnowledgeCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "law" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("law subcommand not registered in knowledge command")
	}
}

func TestKnowledgeLawCommand_Properties(t *testing.T) {
	cmd := NewKnowledgeLawCommand()
	if cmd == nil {
		t.Fatal("NewKnowledgeLawCommand returned nil")
	}
	if cmd.Use != "law" {
		t.Errorf("expected Use='law', got %q", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}

	expected := []string{"list", "show", "add-evidence", "approve", "promote", "upgrade"}
	if len(cmd.Commands()) != len(expected) {
		t.Errorf("expected %d subcommands, got %d", len(expected), len(cmd.Commands()))
	}
	registered := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("missing law subcommand: %s", name)
		}
	}
}

func TestKnowledgeLawSubcommands_NonNil(t *testing.T) {
	tests := []struct {
		name    string
		factory func() *cobra.Command
		use     string
	}{
		{"list", NewKnowledgeLawListCommand, "list"},
		{"show", NewKnowledgeLawShowCommand, "show <id>"},
		{"add-evidence", NewKnowledgeLawAddEvidenceCommand, "add-evidence <id>"},
		{"approve", NewKnowledgeLawApproveCommand, "approve <id>"},
		{"promote", NewKnowledgeLawPromoteCommand, "promote <id>"},
		{"upgrade", NewKnowledgeLawUpgradeCommand, "upgrade [id]"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.factory()
			if cmd == nil {
				t.Fatalf("factory for %q returned nil", tt.name)
			}
			if !strings.HasPrefix(cmd.Use, tt.use) {
				t.Errorf("expected Use to start with %q, got %q", tt.use, cmd.Use)
			}
			if cmd.Short == "" {
				t.Error("expected non-empty Short description")
			}
			if cmd.RunE == nil {
				t.Error("expected RunE to be set")
			}
		})
	}
}

// =============================================================================
// `law list` — empty engine
// =============================================================================

// TestKnowledgeLawList_EmptyEngine verifies the "nenhuma lei" path: an
// existing laws.json with zero items (the seed never runs over an existing
// file, so this path is still reachable).
func TestKnowledgeLawList_EmptyEngine(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	// Arquivo existente sem leis → seed não roda (idempotente).
	seedLawsFile(t, dir)

	cmd := NewKnowledgeLawListCommand()
	output, err := runLawCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	if !strings.Contains(output, "Nenhuma lei registrada") {
		t.Errorf("expected 'nenhuma lei' message, got output: %q", output)
	}
}

// =============================================================================
// `law list` — with persisted laws
// =============================================================================

func TestKnowledgeLawList_WithLaws(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	seedLawsFile(t, dir, &knowledge.KnowledgeItem{
		ID:         "K-1",
		Title:      "Limites são a espinha",
		Level:      knowledge.LevelLaw,
		Confidence: 0.99,
		Evidence: []knowledge.Evidence{
			{ID: "ev-1", Kind: "benchmark", Source: "test/", Description: "reduziu latência", Timestamp: time.Now()},
		},
	})

	cmd := NewKnowledgeLawListCommand()
	output, err := runLawCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	for _, want := range []string{"ID", "Título", "Nível", "Evidências", "Confiança", "K-1", "law", "99%"} {
		if !strings.Contains(output, want) {
			t.Errorf("list output missing %q; output:\n%s", want, output)
		}
	}
}

func TestKnowledgeLawList_EmptyEngine_JSON(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{JSON: true}

	// Arquivo existente sem leis → seed não roda; JSON vazio.
	seedLawsFile(t, dir)

	cmd := NewKnowledgeLawListCommand()
	output, err := runLawCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	output = strings.TrimSpace(output)
	if output != "[]" {
		t.Errorf("expected '[]' for empty engine JSON, got: %q", output)
	}
}

// =============================================================================
// `law add-evidence`
// =============================================================================

func TestKnowledgeLawAddEvidence(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeLawAddEvidenceCommand()
	if err := cmd.Flags().Set("kind", "benchmark"); err != nil {
		t.Fatalf("set kind: %v", err)
	}
	if err := cmd.Flags().Set("source", "test/"); err != nil {
		t.Fatalf("set source: %v", err)
	}
	if err := cmd.Flags().Set("desc", "reduziu latência em 40%"); err != nil {
		t.Fatalf("set desc: %v", err)
	}

	output, err := runLawCommand(t, cmd, []string{"K-1"})
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	if !strings.Contains(output, "Evidência adicionada a K-1") {
		t.Errorf("expected success message, got: %q", output)
	}

	// Persistência: a lei K-1 existe com 1 evidência (nasceu como observação).
	items := readLawsFile(t, dir)
	if len(items) != 1 {
		t.Fatalf("expected 1 law persisted, got %d", len(items))
	}
	law := items[0]
	if law.ID != "K-1" {
		t.Errorf("law ID = %q, want K-1", law.ID)
	}
	if len(law.Evidence) != 1 {
		t.Errorf("expected 1 evidence, got %d", len(law.Evidence))
	}
	if law.Evidence[0].Kind != "benchmark" || law.Evidence[0].Source != "test/" {
		t.Errorf("evidence mismatch: %+v", law.Evidence[0])
	}
	if law.Evidence[0].Description != "reduziu latência em 40%" {
		t.Errorf("evidence description = %q", law.Evidence[0].Description)
	}
	if law.Level != knowledge.LevelObservation {
		t.Errorf("expected initial level %q (1 evidence, confiança baixa), got %q",
			knowledge.LevelObservation, law.Level)
	}
	if law.Confidence != 0.10 {
		t.Errorf("expected observation confidence 0.10, got %.2f", law.Confidence)
	}
}

// TestKnowledgeLawAddEvidence_PromotesToLearning verifies automatic promotion:
// 3 evidences + confidence >= 0.70 → level "learning" (engine threshold).
func TestKnowledgeLawAddEvidence_PromotesToLearning(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	// Lei pré-registrada com confiança medida (0.80 ≥ 0.70).
	seedLawsFile(t, dir, &knowledge.KnowledgeItem{
		ID:         "K-2",
		Title:      "Regra validada",
		Confidence: 0.80,
	})

	for _, d := range []string{"evidência 1", "evidência 2", "evidência 3"} {
		cmd := NewKnowledgeLawAddEvidenceCommand()
		_ = cmd.Flags().Set("kind", "teste")
		_ = cmd.Flags().Set("source", "test/")
		_ = cmd.Flags().Set("desc", d)
		if _, err := runLawCommand(t, cmd, []string{"K-2"}); err != nil {
			t.Fatalf("add-evidence: %v", err)
		}
	}

	items := readLawsFile(t, dir)
	if len(items) != 1 || items[0].ID != "K-2" {
		t.Fatalf("unexpected laws: %+v", items)
	}
	if len(items[0].Evidence) != 3 {
		t.Errorf("expected 3 evidences, got %d", len(items[0].Evidence))
	}
	if items[0].Level != knowledge.LevelLearning {
		t.Errorf("expected promotion to %q after 3 evidences + confiança 0.80, got %q",
			knowledge.LevelLearning, items[0].Level)
	}
}

func TestKnowledgeLawAddEvidence_RequiresFlags(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeLawAddEvidenceCommand()
	// Sem --desc: a adição deve falhar (evidência sem descrição não é evidência).
	_ = cmd.Flags().Set("kind", "benchmark")
	_ = cmd.Flags().Set("source", "test/")
	if _, err := runLawCommand(t, cmd, []string{"K-1"}); err == nil {
		t.Fatal("expected error when --desc is missing")
	}
}

// TestAddEvidence_WithTitle verifies that --title names the law when
// add-evidence creates it: `cosca knowledge law add-evidence K-27
// --title "Nunca executar como root automaticamente"` cria a lei com o
// título correto (e não o ID).
func TestAddEvidence_WithTitle(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeLawAddEvidenceCommand()
	_ = cmd.Flags().Set("kind", "audit")
	_ = cmd.Flags().Set("source", "red-team-A1")
	_ = cmd.Flags().Set("desc", "jail deve recusar root automaticamente")
	_ = cmd.Flags().Set("title", "Nunca executar como root automaticamente")

	output, err := runLawCommand(t, cmd, []string{"K-27"})
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	if !strings.Contains(output, "Evidência adicionada a K-27") {
		t.Errorf("expected success message, got: %q", output)
	}

	items := readLawsFile(t, dir)
	if len(items) != 1 || items[0].ID != "K-27" {
		t.Fatalf("unexpected laws: %+v", items)
	}
	if items[0].Title != "Nunca executar como root automaticamente" {
		t.Errorf("law title = %q, want %q", items[0].Title, "Nunca executar como root automaticamente")
	}
	if items[0].Title == items[0].ID {
		t.Error("title should not fall back to the ID when --title is provided")
	}
}

// TestAddEvidence_WithTitle_FallsBackToID verifies the default: sem --title,
// a lei criada recebe o ID como título.
func TestAddEvidence_WithTitle_FallsBackToID(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeLawAddEvidenceCommand()
	_ = cmd.Flags().Set("kind", "audit")
	_ = cmd.Flags().Set("source", "red-team-A1")
	_ = cmd.Flags().Set("desc", "evidência")

	if _, err := runLawCommand(t, cmd, []string{"K-27"}); err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	items := readLawsFile(t, dir)
	if len(items) != 1 {
		t.Fatalf("expected 1 law, got %d", len(items))
	}
	if items[0].Title != "K-27" {
		t.Errorf("law title = %q, want fallback to ID %q", items[0].Title, "K-27")
	}
}

// TestAddEvidence_WithConfidence verifies that --confidence (medida real,
// 0-1) permite a promoção: 3 evidências com confiança 0.8 → learning.
func TestAddEvidence_WithConfidence(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	for _, d := range []string{"evidência 1", "evidência 2", "evidência 3"} {
		cmd := NewKnowledgeLawAddEvidenceCommand()
		_ = cmd.Flags().Set("kind", "teste")
		_ = cmd.Flags().Set("source", "test/")
		_ = cmd.Flags().Set("desc", d)
		_ = cmd.Flags().Set("confidence", "0.8")
		if _, err := runLawCommand(t, cmd, []string{"K-9"}); err != nil {
			t.Fatalf("add-evidence: %v", err)
		}
	}

	items := readLawsFile(t, dir)
	if len(items) != 1 || items[0].ID != "K-9" {
		t.Fatalf("unexpected laws: %+v", items)
	}
	if len(items[0].Evidence) != 3 {
		t.Errorf("expected 3 evidences, got %d", len(items[0].Evidence))
	}
	if items[0].Confidence != 0.8 {
		t.Errorf("expected confidence 0.8, got %.2f", items[0].Confidence)
	}
	if items[0].Level != knowledge.LevelLearning {
		t.Errorf("expected promotion to %q with --confidence 0.8 + 3 evidences, got %q",
			knowledge.LevelLearning, items[0].Level)
	}
}

// TestAddEvidence_WithConfidence_UpdatesExisting verifies that --confidence
// also updates an EXISTING law (not only at creation): a lei com 3
// evidências na observação é promovida quando a confiança medida chega.
func TestAddEvidence_WithConfidence_UpdatesExisting(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	// Lei existente com 3 evidências, mas confiança de observação (0.10).
	seedLawsFile(t, dir, &knowledge.KnowledgeItem{
		ID:         "K-11",
		Title:      "Regra existente",
		Confidence: 0.10,
		Evidence: []knowledge.Evidence{
			{ID: "ev-1", Kind: "test", Source: "test/", Description: "e1", Timestamp: time.Now()},
			{ID: "ev-2", Kind: "test", Source: "test/", Description: "e2", Timestamp: time.Now()},
			{ID: "ev-3", Kind: "test", Source: "test/", Description: "e3", Timestamp: time.Now()},
		},
	})

	cmd := NewKnowledgeLawAddEvidenceCommand()
	_ = cmd.Flags().Set("kind", "teste")
	_ = cmd.Flags().Set("source", "test/")
	_ = cmd.Flags().Set("desc", "evidência 4")
	_ = cmd.Flags().Set("confidence", "0.8")
	if _, err := runLawCommand(t, cmd, []string{"K-11"}); err != nil {
		t.Fatalf("add-evidence: %v", err)
	}

	items := readLawsFile(t, dir)
	if len(items) != 1 || items[0].ID != "K-11" {
		t.Fatalf("unexpected laws: %+v", items)
	}
	if items[0].Confidence != 0.8 {
		t.Errorf("expected confidence 0.8, got %.2f", items[0].Confidence)
	}
	if items[0].Level != knowledge.LevelLearning {
		t.Errorf("expected promotion to %q after measured confidence 0.8, got %q",
			knowledge.LevelLearning, items[0].Level)
	}
}

// TestAddEvidence_WithConfidence_Invalid verifies the 0-1 validation:
// valores fora da escala são rejeitados.
func TestAddEvidence_WithConfidence_Invalid(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{}

	for _, bad := range []string{"-0.1", "1.5"} {
		cmd := NewKnowledgeLawAddEvidenceCommand()
		_ = cmd.Flags().Set("kind", "audit")
		_ = cmd.Flags().Set("source", "red-team-A1")
		_ = cmd.Flags().Set("desc", "evidência")
		_ = cmd.Flags().Set("confidence", bad)
		if _, err := runLawCommand(t, cmd, []string{"K-99"}); err == nil {
			t.Errorf("expected error for --confidence=%s (fora de 0-1)", bad)
		}
	}
}

// =============================================================================
// `law show`
// =============================================================================

func TestKnowledgeLawShow(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{}

	add := NewKnowledgeLawAddEvidenceCommand()
	_ = add.Flags().Set("kind", "benchmark")
	_ = add.Flags().Set("source", "test/")
	_ = add.Flags().Set("desc", "reduziu latência em 40%")
	if _, err := runLawCommand(t, add, []string{"K-1"}); err != nil {
		t.Fatalf("add-evidence: %v", err)
	}

	cmd := NewKnowledgeLawShowCommand()
	output, err := runLawCommand(t, cmd, []string{"K-1"})
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	for _, want := range []string{"Lei K-1", "Nível", "Confiança", "Evidências", "WhyExists", "reduziu latência em 40%"} {
		if !strings.Contains(output, want) {
			t.Errorf("show output missing %q; output:\n%s", want, output)
		}
	}
}

func TestKnowledgeLawShow_NotFound(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeLawShowCommand()
	if _, err := runLawCommand(t, cmd, []string{"K-999"}); err == nil {
		t.Fatal("expected error for unknown law")
	}
}

// =============================================================================
// `law approve`
// =============================================================================

func TestKnowledgeLawApprove(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	add := NewKnowledgeLawAddEvidenceCommand()
	_ = add.Flags().Set("kind", "benchmark")
	_ = add.Flags().Set("source", "test/")
	_ = add.Flags().Set("desc", "evidência")
	if _, err := runLawCommand(t, add, []string{"K-1"}); err != nil {
		t.Fatalf("add-evidence: %v", err)
	}

	cmd := NewKnowledgeLawApproveCommand()
	output, err := runLawCommand(t, cmd, []string{"K-1"})
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	if !strings.Contains(output, "Constituição") {
		t.Errorf("expected constitution approval message, got: %q", output)
	}

	items := readLawsFile(t, dir)
	if len(items) != 1 {
		t.Fatalf("expected 1 law, got %d", len(items))
	}
	if items[0].Level != knowledge.LevelConstitution {
		t.Errorf("expected level %q after approval, got %q", knowledge.LevelConstitution, items[0].Level)
	}
}

func TestKnowledgeLawApprove_NotFound(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeLawApproveCommand()
	if _, err := runLawCommand(t, cmd, []string{"K-999"}); err == nil {
		t.Fatal("expected error for unknown law")
	}
}

// =============================================================================
// Seed inicial — primeira execução do `law list`
// =============================================================================

// TestSeedLaws_FirstRun verifies that `law list` without laws.json seeds
// the 5 real laws (K-01..K-05), each at level learning with >= 3 evidences.
func TestSeedLaws_FirstRun(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeLawListCommand()
	output, err := runLawCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	// O arquivo foi criado com as 5 leis seed.
	items := readLawsFile(t, dir)
	if len(items) != 5 {
		t.Fatalf("expected 5 seed laws, got %d", len(items))
	}

	want := map[string]string{
		"K-01": "Nunca executar como root automaticamente",
		"K-02": "Registro público desabilitado por padrão",
		"K-03": "Path traversal bloqueado por containment",
		"K-04": "Refresh token exige rotação e revogação",
		"K-05": "Jail recusa executar como root",
	}
	for _, it := range items {
		title, ok := want[it.ID]
		if !ok {
			t.Errorf("unexpected seed law ID %q", it.ID)
			continue
		}
		if it.Title != title {
			t.Errorf("seed law %s title = %q, want %q", it.ID, it.Title, title)
		}
		if it.Level != knowledge.LevelLearning {
			t.Errorf("seed law %s level = %q, want %q", it.ID, it.Level, knowledge.LevelLearning)
		}
		if len(it.Evidence) < knowledge.ThresholdLearningEvidence {
			t.Errorf("seed law %s has %d evidences, want >= %d",
				it.ID, len(it.Evidence), knowledge.ThresholdLearningEvidence)
		}
		if it.Confidence < knowledge.MinConfidenceLearning {
			t.Errorf("seed law %s confidence = %.2f, want >= %.2f",
				it.ID, it.Confidence, knowledge.MinConfidenceLearning)
		}
	}

	// O output da tabela mostra as 5 leis.
	for _, id := range []string{"K-01", "K-02", "K-03", "K-04", "K-05"} {
		if !strings.Contains(output, id) {
			t.Errorf("list output missing seed law %s; output:\n%s", id, output)
		}
	}
}

// TestSeedLaws_Idempotent verifies that a second `law list` does not
// duplicate or overwrite the seed: still 5 laws with the same evidence.
func TestSeedLaws_Idempotent(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeLawListCommand()
	if _, err := runLawCommand(t, cmd, nil); err != nil {
		t.Fatalf("first RunE: %v", err)
	}
	first := readLawsFile(t, dir)
	if len(first) != 5 {
		t.Fatalf("expected 5 laws after first run, got %d", len(first))
	}

	// Segunda chamada: não pode duplicar nem sobrescrever.
	cmd2 := NewKnowledgeLawListCommand()
	if _, err := runLawCommand(t, cmd2, nil); err != nil {
		t.Fatalf("second RunE: %v", err)
	}
	second := readLawsFile(t, dir)

	if len(second) != 5 {
		t.Fatalf("expected still 5 laws after second run, got %d", len(second))
	}
	if len(second) != len(first) {
		t.Fatalf("seed duplicated laws: first=%d second=%d", len(first), len(second))
	}

	// Contagem de evidências por lei não pode mudar (sem re-registro).
	byID := make(map[string]knowledge.KnowledgeItem, len(first))
	for _, it := range first {
		byID[it.ID] = it
	}
	for _, it := range second {
		orig, ok := byID[it.ID]
		if !ok {
			t.Errorf("law %q disappeared between runs", it.ID)
			continue
		}
		if len(it.Evidence) != len(orig.Evidence) {
			t.Errorf("law %s evidence count changed: first=%d second=%d",
				it.ID, len(orig.Evidence), len(it.Evidence))
		}
	}
}

// TestSeedLaws_DoesNotOverwriteExisting verifies the idempotency contract:
// um laws.json já existente (mesmo com 1 lei) nunca é sobrescrito pelo seed.
func TestSeedLaws_DoesNotOverwriteExisting(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	// Arquivo com 1 lei manual — o seed não pode substituí-lo por 5.
	seedLawsFile(t, dir, &knowledge.KnowledgeItem{
		ID:         "K-27",
		Title:      "Lei do Don",
		Confidence: 0.95,
	})

	cmd := NewKnowledgeLawListCommand()
	if _, err := runLawCommand(t, cmd, nil); err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	items := readLawsFile(t, dir)
	if len(items) != 1 {
		t.Fatalf("existing laws.json was overwritten by seed: got %d laws", len(items))
	}
	if items[0].ID != "K-27" || items[0].Title != "Lei do Don" {
		t.Errorf("existing law mutated: %+v", items[0])
	}
}

// =============================================================================
// JSON output
// =============================================================================

func TestKnowledgeLawAddEvidence_JSON(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{JSON: true}

	cmd := NewKnowledgeLawAddEvidenceCommand()
	_ = cmd.Flags().Set("kind", "auditoria")
	_ = cmd.Flags().Set("source", "audit/2026-07.md")
	_ = cmd.Flags().Set("desc", "sem violações")

	output, err := runLawCommand(t, cmd, []string{"K-3"})
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	var law knowledge.KnowledgeItem
	if err := json.Unmarshal([]byte(output), &law); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, output)
	}
	if law.ID != "K-3" {
		t.Errorf("json id = %q, want K-3", law.ID)
	}
	if len(law.Evidence) != 1 || law.Evidence[0].Kind != "auditoria" {
		t.Errorf("json evidence mismatch: %+v", law.Evidence)
	}
}

func TestKnowledgeLawShow_JSON(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{JSON: true}

	add := NewKnowledgeLawAddEvidenceCommand()
	_ = add.Flags().Set("kind", "teste")
	_ = add.Flags().Set("source", "test/")
	_ = add.Flags().Set("desc", "evidência")
	if _, err := runLawCommand(t, add, []string{"K-4"}); err != nil {
		t.Fatalf("add-evidence: %v", err)
	}

	cmd := NewKnowledgeLawShowCommand()
	output, err := runLawCommand(t, cmd, []string{"K-4"})
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	var payload struct {
		Item knowledge.KnowledgeItem `json:"item"`
		Why  string                  `json:"why"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, output)
	}
	if payload.Item.ID != "K-4" {
		t.Errorf("json item id = %q, want K-4", payload.Item.ID)
	}
	if !strings.Contains(payload.Why, "Por que eu existo?") {
		t.Errorf("json why missing WhyExists narrative: %q", payload.Why)
	}
}
