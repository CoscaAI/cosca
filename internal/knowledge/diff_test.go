//
// Tests for internal/knowledge/diff.go — Knowledge Diff.
//
// Cobre:
//   - VersionDiffs round-trip via JSON (field ADD-ON omitempty no manifesto)
//   - Diff: chave exata "from->to"; reversa com kinds negados (new↔removed)
//   - Nenhum diff registrado → diff vazio + Summary
//   - AssessRisk: 0 itens → none; 2 → low; 6 → high; AffectedKnowledge certo
//   - ProjectImpact conta símbolos changed/removed
//
// Determinístico, sem rede. No-regression: package_test.go continua verde.
//

package knowledge

import (
	"encoding/json"
	"strings"
	"testing"
)

// =============================================================================
// VersionDiffs round-trip (manifesto ADD-ON omitempty)
// =============================================================================

func TestKnowledgePackage_VersionDiffsRoundTrip(t *testing.T) {
	t.Parallel()

	pkg := KnowledgePackage{
		ID: "prisma", Kind: PackageKindLibrary, Ecosystem: "typescript",
		Status:         PackageStatusManifest,
		KnowledgeLevel: PackageKnowledgeNone,
		VersionDiffs: map[string][]DiffEntry{
			"6.0->6.1": {
				{Kind: DiffKindNew, Symbol: "Resource X", Detail: "novo recurso de deploy"},
				{Kind: DiffKindChanged, Symbol: "schema.migrations", Detail: "novo formato do arquivo"},
				{Kind: DiffKindDeprecated, Symbol: "API A", Detail: "será removida na 7.0"},
				{Kind: DiffKindRemoved, Symbol: "API B", Detail: "substituída por Resource X"},
			},
		},
	}

	data, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var back KnowledgePackage
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(back.VersionDiffs) != 1 {
		t.Fatalf("round-trip deveria preservar version_diffs, got %d chaves", len(back.VersionDiffs))
	}
	entries := back.VersionDiffs["6.0->6.1"]
	if len(entries) != 4 {
		t.Fatalf("esperava 4 entradas, got %d", len(entries))
	}
	if entries[0].Kind != DiffKindNew || entries[0].Symbol != "Resource X" {
		t.Errorf("entrada 0 não preservada: %+v", entries[0])
	}
	if entries[3].Kind != DiffKindRemoved || entries[3].Symbol != "API B" {
		t.Errorf("entrada 3 não preservada: %+v", entries[3])
	}

	// Sem version_diffs → omitempty mantém o campo ausente no JSON.
	plain := KnowledgePackage{ID: "pgx", Kind: PackageKindLibrary, Ecosystem: "go"}
	plainData, err := json.Marshal(plain)
	if err != nil {
		t.Fatalf("marshal plain: %v", err)
	}
	if strings.Contains(string(plainData), "version_diffs") {
		t.Errorf("manifesto sem diffs não deveria serializar version_diffs: %s", plainData)
	}
}

// =============================================================================
// Diff — chave exata, reversa negada, vazio
// =============================================================================

func TestDiff_ExactKey(t *testing.T) {
	t.Parallel()

	pkg := KnowledgePackage{
		ID: "prisma",
		VersionDiffs: map[string][]DiffEntry{
			"6.0->6.1": {
				{Kind: DiffKindNew, Symbol: "Resource X", Detail: "novo"},
				{Kind: DiffKindRemoved, Symbol: "API B", Detail: "saiu"},
			},
		},
	}

	d, err := pkg.Diff("6.0", "6.1")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.PackageID != "prisma" || d.FromVersion != "6.0" || d.ToVersion != "6.1" {
		t.Errorf("metadados errados: %+v", d)
	}
	if len(d.Entries) != 2 {
		t.Fatalf("esperava 2 entradas, got %d", len(d.Entries))
	}
	if d.Entries[0].Kind != DiffKindNew || d.Entries[0].Symbol != "Resource X" {
		t.Errorf("entrada 0 errada: %+v", d.Entries[0])
	}
	if d.Entries[1].Kind != DiffKindRemoved || d.Entries[1].Symbol != "API B" {
		t.Errorf("entrada 1 errada: %+v", d.Entries[1])
	}
	if d.RiskLevel != RiskNone {
		t.Errorf("risk inicial deveria ser none, got %q", d.RiskLevel)
	}
}

func TestDiff_ReverseKeyNegatesKinds(t *testing.T) {
	t.Parallel()

	// Só o caminho reverso "6.1->6.0" registrado; Diff("6.0","6.1") deve
	// usá-lo negando os kinds: new↔removed; changed/deprecated preservados.
	pkg := KnowledgePackage{
		ID: "prisma",
		VersionDiffs: map[string][]DiffEntry{
			"6.1->6.0": {
				{Kind: DiffKindNew, Symbol: "API B", Detail: "não existia na 6.0"},
				{Kind: DiffKindRemoved, Symbol: "Resource X", Detail: "não existia na 6.0"},
				{Kind: DiffKindChanged, Symbol: "schema.migrations", Detail: "reverte formato"},
				{Kind: DiffKindDeprecated, Symbol: "API C", Detail: "não deprecada ainda"},
			},
		},
	}

	d, err := pkg.Diff("6.0", "6.1")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if len(d.Entries) != 4 {
		t.Fatalf("esperava 4 entradas, got %d", len(d.Entries))
	}
	want := []struct {
		kind   string
		symbol string
	}{
		{DiffKindRemoved, "API B"},  // new no reverso → removed
		{DiffKindNew, "Resource X"}, // removed no reverso → new
		{DiffKindChanged, "schema.migrations"},
		{DiffKindDeprecated, "API C"},
	}
	for i, w := range want {
		if d.Entries[i].Kind != w.kind || d.Entries[i].Symbol != w.symbol {
			t.Errorf("entrada %d = %+v, want kind=%s symbol=%s", i, d.Entries[i], w.kind, w.symbol)
		}
	}
}

func TestDiff_NoneRecorded_EmptyWithSummary(t *testing.T) {
	t.Parallel()

	pkg := KnowledgePackage{ID: "prisma"} // sem version_diffs
	d, err := pkg.Diff("6.0", "6.1")
	if err != nil {
		t.Fatalf("Diff sem diff registrado não deveria errar: %v", err)
	}
	if len(d.Entries) != 0 {
		t.Fatalf("esperava diff vazio, got %d entradas", len(d.Entries))
	}
	if d.Summary != "nenhuma mudança registrada entre 6.0 e 6.1" {
		t.Errorf("summary errado: %q", d.Summary)
	}
	if d.RiskLevel != RiskNone {
		t.Errorf("risk deveria ser none, got %q", d.RiskLevel)
	}
}

func TestDiff_InvalidArgs(t *testing.T) {
	t.Parallel()

	pkg := KnowledgePackage{ID: "prisma"}
	if _, err := pkg.Diff("", "6.1"); err == nil {
		t.Error("from vazio deveria falhar")
	}
	if _, err := pkg.Diff("6.0", "  "); err == nil {
		t.Error("to em branco deveria falhar")
	}
	if _, err := pkg.Diff("6.0", "6.0"); err == nil {
		t.Error("versões iguais deveriam falhar")
	}
}

// =============================================================================
// AssessRisk — escalada por itens afetados
// =============================================================================

func TestAssessRisk_NoAffected(t *testing.T) {
	t.Parallel()

	pkg := KnowledgePackage{ID: "prisma"}
	d, _ := pkg.Diff("6.0", "6.1")
	items := []KnowledgeItem{
		{ID: "K-1", Title: "regra de segurança"},
	}

	out := pkg.AssessRisk(d, items)
	if out.RiskLevel != RiskNone {
		t.Errorf("sem itens afetados ⇒ none, got %q", out.RiskLevel)
	}
	if len(out.AffectedKnowledge) != 0 {
		t.Errorf("nenhum item deveria estar afetado, got %v", out.AffectedKnowledge)
	}
}

func TestAssessRisk_LowAndHighEscalation(t *testing.T) {
	t.Parallel()

	pkg := KnowledgePackage{
		ID: "prisma",
		VersionDiffs: map[string][]DiffEntry{
			"6.0->6.1": {
				{Kind: DiffKindNew, Symbol: "Resource X", Detail: "novo"},
				{Kind: DiffKindChanged, Symbol: "schema.migrations", Detail: "formato novo"},
				{Kind: DiffKindRemoved, Symbol: "API B", Detail: "removida"},
			},
		},
	}
	d, _ := pkg.Diff("6.0", "6.1")

	makeItems := func(n int) []KnowledgeItem {
		items := make([]KnowledgeItem, 0, n)
		for i := 1; i <= n; i++ {
			items = append(items, KnowledgeItem{
				ID:    "K-" + string(rune('0'+i)),
				Title: "dummy",
				Evidence: []Evidence{
					{Source: "prisma", Description: "usa API B no schema"},
				},
			})
		}
		return items
	}

	// 2 itens → low.
	low := pkg.AssessRisk(d, makeItems(2))
	if low.RiskLevel != RiskLow {
		t.Errorf("2 itens ⇒ low, got %q", low.RiskLevel)
	}
	if len(low.AffectedKnowledge) != 2 {
		t.Errorf("2 itens deveriam estar afetados, got %v", low.AffectedKnowledge)
	}
	if low.AffectedKnowledge[0] != "K-1" || low.AffectedKnowledge[1] != "K-2" {
		t.Errorf("AffectedKnowledge errado: %v", low.AffectedKnowledge)
	}
	if low.Summary != "2 padrões existentes podem ser afetados" {
		t.Errorf("summary errado: %q", low.Summary)
	}

	// 6 itens → high.
	high := pkg.AssessRisk(d, makeItems(6))
	if high.RiskLevel != RiskHigh {
		t.Errorf("6 itens ⇒ high, got %q", high.RiskLevel)
	}
	if len(high.AffectedKnowledge) != 6 {
		t.Errorf("6 itens deveriam estar afetados, got %v", high.AffectedKnowledge)
	}
	if high.AffectedKnowledge[0] != "K-1" || high.AffectedKnowledge[5] != "K-6" {
		t.Errorf("AffectedKnowledge deveria estar ordenado: %v", high.AffectedKnowledge)
	}
}

func TestAssessRisk_MediumAndOnlyReferencesPackage(t *testing.T) {
	t.Parallel()

	pkg := KnowledgePackage{
		ID: "prisma",
		VersionDiffs: map[string][]DiffEntry{
			"6.0->6.1": {
				{Kind: DiffKindRemoved, Symbol: "API B", Detail: "removida"},
			},
		},
	}
	d, _ := pkg.Diff("6.0", "6.1")

	items := []KnowledgeItem{
		// 3 que referenciam prisma E mencionam API B → afetadas.
		{ID: "K-182", Title: "Prisma: usar API B no datasource", Evidence: []Evidence{{Source: "prisma", Description: "API B"}}},
		{ID: "K-219", Title: "migrações prisma", Evidence: []Evidence{{Source: "official-docs", Description: "prisma API B"}}},
		{ID: "K-441", Title: "padrão de seed", Evidence: []Evidence{{Source: "prisma", Description: "API B removida"}}},
		// NÃO referencia o pacote → ignorada mesmo mencionando API B.
		{ID: "K-999", Title: "API B de outro contexto"},
	}

	out := pkg.AssessRisk(d, items)
	if out.RiskLevel != RiskMedium {
		t.Errorf("3 itens ⇒ medium, got %q", out.RiskLevel)
	}
	want := []string{"K-182", "K-219", "K-441"}
	if len(out.AffectedKnowledge) != len(want) {
		t.Fatalf("AffectedKnowledge = %v, want %v", out.AffectedKnowledge, want)
	}
	for i := range want {
		if out.AffectedKnowledge[i] != want[i] {
			t.Errorf("AffectedKnowledge[%d] = %q, want %q", i, out.AffectedKnowledge[i], want[i])
		}
	}
	if out.Summary != "3 padrões existentes podem ser afetados" {
		t.Errorf("summary errado: %q", out.Summary)
	}

	// "new" nunca afeta conhecimento existente.
	newOnly := KnowledgePackage{
		ID: "prisma",
		VersionDiffs: map[string][]DiffEntry{
			"6.0->6.1": {{Kind: DiffKindNew, Symbol: "Resource X", Detail: "novo"}},
		},
	}
	nd, _ := newOnly.Diff("6.0", "6.1")
	noRisk := newOnly.AssessRisk(nd, items)
	if noRisk.RiskLevel != RiskNone || len(noRisk.AffectedKnowledge) != 0 {
		t.Errorf("só 'new' não deveria afetar nada: %+v", noRisk)
	}
}

// =============================================================================
// ProjectImpact — conta símbolos changed/removed nas dependências
// =============================================================================

func TestProjectImpact_CountsChangedRemovedSymbols(t *testing.T) {
	t.Parallel()

	pkg := KnowledgePackage{ID: "prisma"}
	d, _ := pkg.Diff("6.0", "6.1")
	d.Entries = []DiffEntry{
		{Kind: DiffKindNew, Symbol: "Resource X"},            // não conta
		{Kind: DiffKindChanged, Symbol: "schema.migrations"}, // conta
		{Kind: DiffKindRemoved, Symbol: "API B"},             // conta
		{Kind: DiffKindDeprecated, Symbol: "API A"},          // não conta
	}

	deps := []string{
		"projeto usa schema.migrations no pipeline",
		"API B chamada em 3 services",
	}
	got := pkg.ProjectImpact(d, deps)
	if got != 2 {
		t.Errorf("esperava 2 símbolos impactados, got %d", got)
	}

	// Símbolo ausente das dependências → não conta.
	depsMissing := []string{"nada a ver aqui"}
	if got := pkg.ProjectImpact(d, depsMissing); got != 0 {
		t.Errorf("esperava 0, got %d", got)
	}

	// "or related items": título/evidência de item relacionado conta.
	related := []string{"item relacionado: usa API B em produção"}
	if got := pkg.ProjectImpact(d, related); got != 1 {
		t.Errorf("esperava 1 via item relacionado, got %d", got)
	}
}
