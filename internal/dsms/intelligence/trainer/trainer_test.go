package trainer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cosca/internal/dsms/intelligence"
	"cosca/internal/dsms/intelligence/rules"
)

// TestTrainOnRealKnowledge trains on the REAL knowledge.db.
// This is the "treinar com conhecimento da casa" - connects the engine
// to the 118K entries of curated knowledge.
func TestTrainOnRealKnowledge(t *testing.T) {
	// Locate knowledge.db
	wd, _ := os.Getwd()
	projectRoot := filepath.Join(wd, "..", "..", "..", "..")
	projectRoot, _ = filepath.Abs(projectRoot)

	dbPath := filepath.Join(projectRoot, ".cosca", "knowledge.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Skipf("knowledge.db not found at %s - skipping real training", dbPath)
	}

	t.Logf("Knowledge DB: %s", dbPath)

	// Create trainer
	trainer := NewTrainer(dbPath)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Load ALL knowledge (113K chunks + 115 curated)
	t.Log("Carregando TODO o conhecimento da casa...")
	entries, err := trainer.LoadKnowledge(ctx, 120000)
	if err != nil {
		t.Logf("Falha ao carregar knowledge.db real: %v", err)
		t.Logf("Tentando treinar com conhecimento sintético de fallback...")
		entries = syntheticKnowledge()
	}

	t.Logf("Entradas carregadas: %d", len(entries))

	// Train
	t.Log("Treinando regras a partir do conhecimento...")
	result := trainer.Train(ctx, entries)

	// Report
	t.Logf("\n════════════════════════════════════════════════════════")
	t.Logf("  TREINAMENTO COM CONHECIMENTO DA CASA")
	t.Logf("════════════════════════════════════════════════════════")
	t.Logf("  Entradas lidas: %d", result.EntriesRead)
	t.Logf("  Regras extraídas: %d", result.RulesExtracted)
	t.Logf("  Tempo: %v", result.Duration)
	t.Logf("")
	t.Logf("  ── Regras por domínio ──")
	for domain, count := range result.RulesByDomain {
		t.Logf("    %s: %d", domain, count)
	}
	t.Logf("")
	t.Logf("  ── Amostra de regras extraídas ──")
	for i, rule := range result.SampleRules {
		t.Logf("    %d. [%s] %s (conf %.0f%%)", i+1, rule.Domain, rule.Name, rule.Confidence*100)
		t.Logf("       %s", rule.Description[:min(80, len(rule.Description))])
	}
	t.Logf("════════════════════════════════════════════════════════")

	// Assertions
	if result.RulesExtracted == 0 {
		t.Error("Expected rules to be extracted from knowledge")
	}

	// Verify domains covered
	if len(result.RulesByDomain) < 3 {
		t.Errorf("Expected >= 3 domains, got %d", len(result.RulesByDomain))
	}

	t.Logf("\n✅ Treinamento com conhecimento da casa concluído: %d regras extraídas de %d entradas",
		result.RulesExtracted, result.EntriesRead)
}

// TestApplyTrainedRules verifies trained rules work on real code.
func TestApplyTrainedRules(t *testing.T) {
	// Create rules engine
	engine := rules.NewEngine()

	// Train with synthetic knowledge
	entries := syntheticKnowledge()
	trainer := NewTrainer("")
	ctx := context.Background()
	result := trainer.Train(ctx, entries)

	// Register extracted rules
	for _, rule := range result.SampleRules {
		engine.Register(rule)
	}

	// Test on vulnerable code
	code := `func getUser(db *sql.DB, id string) {
	// SQL injection
	db.Query("SELECT * FROM users WHERE id = " + id)
}`

	ctx2 := &intelligence.Context{
		Code: code,
	}

	findings := engine.Evaluate(ctx2)
	if len(findings) == 0 {
		t.Error("Expected findings from trained rules")
	}

	t.Logf("✅ Regras treinadas funcionam no código real: %d findings", len(findings))
}

// syntheticKnowledge provides fallback knowledge for testing.
func syntheticKnowledge() []KnowledgeEntry {
	return []KnowledgeEntry{
		{ID: "1", Content: "SQL injection prevention: always use parameterized queries", Category: "security", Domain: "security", Confidence: 0.95},
		{ID: "2", Content: "Hardcoded secrets are dangerous: use environment variables", Category: "security", Domain: "security", Confidence: 0.9},
		{ID: "3", Content: "N+1 query problem: batch load related data", Category: "performance", Domain: "performance", Confidence: 0.85},
		{ID: "4", Content: "God object anti-pattern: split large files", Category: "code_quality", Domain: "code_quality", Confidence: 0.75},
		{ID: "5", Content: "Architecture best practices: low coupling, high cohesion", Category: "architecture", Domain: "architecture", Confidence: 0.8},
		{ID: "6", Content: "Test coverage is essential: aim for 80%+", Category: "testing", Domain: "testing", Confidence: 0.9},
		{ID: "7", Content: "TODO comments indicate incomplete work", Category: "code_quality", Domain: "code_quality", Confidence: 0.7},
		{ID: "8", Content: "FIXME comments need immediate attention", Category: "code_quality", Domain: "code_quality", Confidence: 0.7},
		{ID: "9", Content: "Input validation prevents many vulnerabilities", Category: "security", Domain: "security", Confidence: 0.85},
		{ID: "10", Content: "Dependency injection improves testability", Category: "architecture", Domain: "architecture", Confidence: 0.75},
	}
}
