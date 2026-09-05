//
// Tests for the `cosca knowledge discovery` command tree
// (internal/cli/knowledge_discovery.go + seed_discoveries.go).
//
// Covers:
//   - Registration of `discovery` (and its 3 subcommands) in the knowledge
//     command
//   - `discovery list` with seed: 5 real discoveries (D-01..D-05) created on
//     the first run, table sorted by impact (D-05 first), stars, idempotent
//     on the second run
//   - `discovery show <id>` → full detail
//   - `discovery add` → registers a new discovery (auto ID D-06) and
//     persists to .cosca/knowledge/hall-of-fame.json (in t.TempDir(), never
//     the real .cosca); impact validation 1-5
//   - JSON output for list/show/add
//
// NOTE: tests that chdir() must NOT run in parallel — they mutate the
// process working directory.

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/knowledge"
)

// discoveryStore espelha o layout JSON on-disk do HallOfFame
// (versão + discoveries).
type discoveryStore struct {
	Version     int                    `json:"version"`
	Discoveries []*knowledge.Discovery `json:"discoveries"`
}

// readHallOfFameFile lê o arquivo runtime do Hall da Fama do diretório dado.
func readHallOfFameFile(t *testing.T, dir string) []*knowledge.Discovery {
	t.Helper()
	path := filepath.Join(dir, ".cosca", "knowledge", "hall-of-fame.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read hall-of-fame.json: %v", err)
	}
	var store discoveryStore
	if err := json.Unmarshal(data, &store); err != nil {
		t.Fatalf("unmarshal hall-of-fame.json: %v", err)
	}
	if store.Discoveries == nil {
		return []*knowledge.Discovery{}
	}
	return store.Discoveries
}

// runDiscoveryCommand executa o RunE de um comando do Hall da Fama com
// formatter no buffer (mesmo helper do CKL).
func runDiscoveryCommand(t *testing.T, cmd *cobra.Command, args []string) (string, error) {
	t.Helper()
	return runLawCommand(t, cmd, args)
}

// seedDiscoveryFile cria um arquivo runtime com as descobertas dadas usando
// o HallOfFame.
func seedDiscoveryFile(t *testing.T, dir string, items ...*knowledge.Discovery) {
	t.Helper()
	h := knowledge.NewHallOfFame()
	for _, d := range items {
		if err := h.Register(d); err != nil {
			t.Fatalf("register: %v", err)
		}
	}
	if err := h.Save(filepath.Join(dir, ".cosca", "knowledge", "hall-of-fame.json")); err != nil {
		t.Fatalf("seed hall-of-fame.json: %v", err)
	}
}

// =============================================================================
// Registration — `discovery` in the knowledge command
// =============================================================================

func TestKnowledgeDiscoveryCommand_RegisteredInKnowledge(t *testing.T) {
	cmd := NewKnowledgeCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "discovery" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("discovery subcommand not registered in knowledge command")
	}
}

func TestKnowledgeDiscoveryCommand_Properties(t *testing.T) {
	cmd := NewKnowledgeDiscoveryCommand()
	if cmd == nil {
		t.Fatal("NewKnowledgeDiscoveryCommand returned nil")
	}
	if cmd.Use != "discovery" {
		t.Errorf("expected Use='discovery', got %q", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}

	expected := []string{"list", "show", "add"}
	if len(cmd.Commands()) != len(expected) {
		t.Errorf("expected %d subcommands, got %d", len(expected), len(cmd.Commands()))
	}
	registered := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("missing discovery subcommand: %s", name)
		}
	}
}

func TestKnowledgeDiscoverySubcommands_NonNil(t *testing.T) {
	tests := []struct {
		name    string
		factory func() *cobra.Command
		use     string
	}{
		{"list", NewKnowledgeDiscoveryListCommand, "list"},
		{"show", NewKnowledgeDiscoveryShowCommand, "show <id>"},
		{"add", NewKnowledgeDiscoveryAddCommand, "add"},
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
// `discovery list` — seed na primeira execução
// =============================================================================

// TestKnowledgeDiscoveryList_Seed verifies the first run seeds the 5 real
// discoveries and renders the table sorted by impact (D-05 first) with stars.
func TestKnowledgeDiscoveryList_Seed(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeDiscoveryListCommand()
	output, err := runDiscoveryCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	// O arquivo foi criado com as 5 descobertas seed.
	items := readHallOfFameFile(t, dir)
	if len(items) != 5 {
		t.Fatalf("expected 5 seed discoveries, got %d", len(items))
	}

	// Colunas da tabela + IDs + estrelas + risco.
	for _, want := range []string{"ID", "Nome", "Impacto", "Projetos", "Risco ↓", "Relacionado",
		"D-01", "D-02", "D-03", "D-04", "D-05",
		"★★★★★", "-87%", "-70%", "-65%", "-60%",
		"K-01", "K-02", "K-03", "K-04", "K-05"} {
		if !strings.Contains(output, want) {
			t.Errorf("list output missing %q; output:\n%s", want, output)
		}
	}

	// Ordenado por impacto desc: D-05 (5 estrelas) vem antes de D-01 (5) e
	// antes de D-04 (4). Procura a posição relativa dos IDs no output.
	pos := func(id string) int { return strings.Index(output, id) }
	if pos("D-05") > pos("D-04") || pos("D-05") > pos("D-03") {
		t.Errorf("D-05 (impacto 5) deve vir antes das de impacto 4; output:\n%s", output)
	}
	if pos("D-01") > pos("D-04") {
		t.Errorf("D-01 (impacto 5) deve vir antes de D-04 (impacto 4); output:\n%s", output)
	}
}

// TestKnowledgeDiscoveryList_Idempotent verifies a second run does not
// duplicate or overwrite the seed: still 5 discoveries.
func TestKnowledgeDiscoveryList_Idempotent(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeDiscoveryListCommand()
	if _, err := runDiscoveryCommand(t, cmd, nil); err != nil {
		t.Fatalf("first RunE: %v", err)
	}
	first := readHallOfFameFile(t, dir)
	if len(first) != 5 {
		t.Fatalf("expected 5 after first run, got %d", len(first))
	}

	cmd2 := NewKnowledgeDiscoveryListCommand()
	if _, err := runDiscoveryCommand(t, cmd2, nil); err != nil {
		t.Fatalf("second RunE: %v", err)
	}
	second := readHallOfFameFile(t, dir)
	if len(second) != 5 {
		t.Fatalf("expected still 5 after second run, got %d", len(second))
	}

	byID := make(map[string]*knowledge.Discovery, len(first))
	for _, d := range first {
		byID[d.ID] = d
	}
	for _, d := range second {
		orig, ok := byID[d.ID]
		if !ok {
			t.Errorf("discovery %q disappeared between runs", d.ID)
			continue
		}
		if orig.Name != d.Name || orig.Impact != d.Impact {
			t.Errorf("discovery %s mutated between runs: %+v -> %+v", d.ID, orig, d)
		}
	}
}

// TestKnowledgeDiscoveryList_DoesNotOverwriteExisting verifies the
// idempotency contract: um hall-of-fame.json já existente (mesmo com 1
// descoberta) nunca é sobrescrito pelo seed.
func TestKnowledgeDiscoveryList_DoesNotOverwriteExisting(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	seedDiscoveryFile(t, dir, &knowledge.Discovery{
		ID:     "D-99",
		Name:   "Descoberta do Don",
		Impact: 5,
	})

	cmd := NewKnowledgeDiscoveryListCommand()
	if _, err := runDiscoveryCommand(t, cmd, nil); err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	items := readHallOfFameFile(t, dir)
	if len(items) != 1 {
		t.Fatalf("existing hall-of-fame.json was overwritten by seed: got %d discoveries", len(items))
	}
	if items[0].ID != "D-99" || items[0].Name != "Descoberta do Don" {
		t.Errorf("existing discovery mutated: %+v", items[0])
	}
}

// =============================================================================
// `discovery show`
// =============================================================================

func TestKnowledgeDiscoveryShow(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{}

	// Semeia via list (primeira execução) e depois mostra D-01.
	list := NewKnowledgeDiscoveryListCommand()
	if _, err := runDiscoveryCommand(t, list, nil); err != nil {
		t.Fatalf("list: %v", err)
	}

	cmd := NewKnowledgeDiscoveryShowCommand()
	output, err := runDiscoveryCommand(t, cmd, []string{"D-01"})
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	for _, want := range []string{
		"Descoberta D-01",
		"Jail fail-closed — recusa executar como root",
		"★★★★★",
		"5/5",
		"Descrição",
		"Projetos afetados",
		"-87%",
		"Sessões validadas",
		"Origem",
		"red-team-onda-1 (auditoria A1+C3)",
		"Lei relacionada (CKL)",
		"K-01",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("show output missing %q; output:\n%s", want, output)
		}
	}
}

func TestKnowledgeDiscoveryShow_NotFound(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeDiscoveryShowCommand()
	if _, err := runDiscoveryCommand(t, cmd, []string{"D-999"}); err == nil {
		t.Fatal("expected error for unknown discovery")
	}
}

// =============================================================================
// `discovery add`
// =============================================================================

func TestKnowledgeDiscoveryAdd(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	// Pre-create an empty hall-of-fame file so the seed does not run;
	// the add should then register the first discovery as D-01.
	if err := os.MkdirAll(filepath.Join(dir, ".cosca", "knowledge"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cosca", "knowledge", "hall-of-fame.json"), []byte(`{"version":1,"discoveries":[]}`), 0o600); err != nil {
		t.Fatalf("write empty hall of fame: %v", err)
	}

	cmd := NewKnowledgeDiscoveryAddCommand()
	_ = cmd.Flags().Set("name", "RAG otimizado")
	_ = cmd.Flags().Set("desc", "embeddings em lote reduziram o custo")
	_ = cmd.Flags().Set("impact", "4")
	_ = cmd.Flags().Set("risk-reduction", "55")
	_ = cmd.Flags().Set("projects", "2")
	_ = cmd.Flags().Set("sessions", "12")
	_ = cmd.Flags().Set("economy-percent", "22")
	_ = cmd.Flags().Set("economy-usd", "312")
	_ = cmd.Flags().Set("origin", "conversa do Don")
	_ = cmd.Flags().Set("law", "K-99")

	output, err := runDiscoveryCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	if !strings.Contains(output, "D-01") {
		t.Errorf("expected auto-generated ID D-01 in output, got: %q", output)
	}
	if !strings.Contains(output, "RAG otimizado") {
		t.Errorf("expected name in success message, got: %q", output)
	}

	// Persistência: 1 descoberta D-06 com todos os campos.
	items := readHallOfFameFile(t, dir)
	if len(items) != 1 {
		t.Fatalf("expected 1 discovery persisted, got %d", len(items))
	}
	d := items[0]
	if d.ID != "D-01" {
		t.Errorf("id = %q, want D-01", d.ID)
	}
	if d.Name != "RAG otimizado" {
		t.Errorf("name = %q", d.Name)
	}
	if d.Impact != 4 {
		t.Errorf("impact = %d, want 4", d.Impact)
	}
	if d.RiskReductionPercent != 55 {
		t.Errorf("risk_reduction_percent = %.1f, want 55", d.RiskReductionPercent)
	}
	if d.ProjectsAffected != 2 {
		t.Errorf("projects_affected = %d, want 2", d.ProjectsAffected)
	}
	if d.ValidatedSessions != 12 {
		t.Errorf("validated_sessions = %d, want 12", d.ValidatedSessions)
	}
	if d.EconomyPercent != 22 {
		t.Errorf("economy_percent = %.1f, want 22", d.EconomyPercent)
	}
	if d.EconomyUSD != 312 {
		t.Errorf("economy_usd = %.1f, want 312", d.EconomyUSD)
	}
	if d.Origin != "conversa do Don" {
		t.Errorf("origin = %q", d.Origin)
	}
	if d.RelatedLaw != "K-99" {
		t.Errorf("related_law = %q, want K-99", d.RelatedLaw)
	}
	if d.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
}

// TestKnowledgeDiscoveryAdd_NextID verifies the ID auto-increment: after the
// seed (D-05 max), the first add creates D-06 and the second D-07.
func TestKnowledgeDiscoveryAdd_NextID(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	list := NewKnowledgeDiscoveryListCommand()
	if _, err := runDiscoveryCommand(t, list, nil); err != nil {
		t.Fatalf("seed list: %v", err)
	}

	for _, wantID := range []string{"D-06", "D-07"} {
		cmd := NewKnowledgeDiscoveryAddCommand()
		_ = cmd.Flags().Set("name", "Descoberta "+wantID)
		_ = cmd.Flags().Set("impact", "3")
		output, err := runDiscoveryCommand(t, cmd, nil)
		if err != nil {
			t.Fatalf("add %s: %v", wantID, err)
		}
		if !strings.Contains(output, wantID) {
			t.Errorf("expected %s in output, got: %q", wantID, output)
		}
	}

	items := readHallOfFameFile(t, dir)
	if len(items) != 7 {
		t.Fatalf("expected 7 discoveries (5 seed + 2 add), got %d", len(items))
	}
}

// TestKnowledgeDiscoveryAdd_RequiresName verifies --name is mandatory.
func TestKnowledgeDiscoveryAdd_RequiresName(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeDiscoveryAddCommand()
	_ = cmd.Flags().Set("impact", "4")
	if _, err := runDiscoveryCommand(t, cmd, nil); err == nil {
		t.Fatal("expected error when --name is missing")
	}
}

// TestKnowledgeDiscoveryAdd_ImpactRange verifies the 1-5 star scale: values
// outside are rejected (impacto em estrelas, não confidence 0-1 do CKL).
func TestKnowledgeDiscoveryAdd_ImpactRange(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{}

	for _, bad := range []string{"0", "6", "-1"} {
		cmd := NewKnowledgeDiscoveryAddCommand()
		_ = cmd.Flags().Set("name", "X")
		_ = cmd.Flags().Set("impact", bad)
		if _, err := runDiscoveryCommand(t, cmd, nil); err == nil {
			t.Errorf("expected error for --impact=%s (fora de 1-5)", bad)
		}
	}
}

// =============================================================================
// JSON output
// =============================================================================

func TestKnowledgeDiscoveryList_JSON(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{JSON: true}

	cmd := NewKnowledgeDiscoveryListCommand()
	output, err := runDiscoveryCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	var items []*knowledge.Discovery
	if err := json.Unmarshal([]byte(output), &items); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, output)
	}
	if len(items) != 5 {
		t.Fatalf("json list has %d discoveries, want 5", len(items))
	}
	if items[0].ID != "D-05" {
		t.Errorf("json list first item = %q, want D-05 (impacto 5 primeiro)", items[0].ID)
	}
	if items[0].Impact != 5 {
		t.Errorf("json first impact = %d, want 5", items[0].Impact)
	}
}

func TestKnowledgeDiscoveryShow_JSON(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{JSON: true}

	list := NewKnowledgeDiscoveryListCommand()
	if _, err := runDiscoveryCommand(t, list, nil); err != nil {
		t.Fatalf("seed list: %v", err)
	}

	cmd := NewKnowledgeDiscoveryShowCommand()
	output, err := runDiscoveryCommand(t, cmd, []string{"D-03"})
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	var d knowledge.Discovery
	if err := json.Unmarshal([]byte(output), &d); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, output)
	}
	if d.ID != "D-03" {
		t.Errorf("json id = %q, want D-03", d.ID)
	}
	if d.Name != "Path traversal bloqueado por containment" {
		t.Errorf("json name = %q", d.Name)
	}
	if d.RiskReductionPercent != 65 {
		t.Errorf("json risk_reduction_percent = %.1f, want 65", d.RiskReductionPercent)
	}
	if d.RelatedLaw != "K-03" {
		t.Errorf("json related_law = %q, want K-03", d.RelatedLaw)
	}
}

func TestKnowledgeDiscoveryAdd_JSON(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{JSON: true}

	cmd := NewKnowledgeDiscoveryAddCommand()
	_ = cmd.Flags().Set("name", "Descoberta JSON")
	_ = cmd.Flags().Set("impact", "5")
	_ = cmd.Flags().Set("risk-reduction", "80")
	_ = cmd.Flags().Set("origin", "auditoria #52")

	output, err := runDiscoveryCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	var d knowledge.Discovery
	if err := json.Unmarshal([]byte(output), &d); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, output)
	}
	if d.ID != "D-01" {
		t.Errorf("json id = %q, want D-01 (primeira descoberta)", d.ID)
	}
	if d.Origin != "auditoria #52" {
		t.Errorf("json origin = %q", d.Origin)
	}
}

// =============================================================================
// Integration: add → list reflects the new discovery
// =============================================================================

func TestKnowledgeDiscoveryAdd_ThenList(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{}

	// Primeiro run semeia; depois adiciona D-06.
	list := NewKnowledgeDiscoveryListCommand()
	if _, err := runDiscoveryCommand(t, list, nil); err != nil {
		t.Fatalf("seed list: %v", err)
	}

	add := NewKnowledgeDiscoveryAddCommand()
	_ = add.Flags().Set("name", "Descoberta do Don")
	_ = add.Flags().Set("impact", "5")
	_ = add.Flags().Set("risk-reduction", "90")
	_ = add.Flags().Set("origin", "conversa do Don")
	if _, err := runDiscoveryCommand(t, add, nil); err != nil {
		t.Fatalf("add: %v", err)
	}

	list2 := NewKnowledgeDiscoveryListCommand()
	output, err := runDiscoveryCommand(t, list2, nil)
	if err != nil {
		t.Fatalf("list after add: %v", err)
	}
	// O seed não roda de novo; D-06 aparece na tabela.
	if !strings.Contains(output, "D-06") {
		t.Errorf("list after add missing D-06; output:\n%s", output)
	}
	if !strings.Contains(output, "Descoberta do Don") {
		t.Errorf("list after add missing new name; output:\n%s", output)
	}
}
