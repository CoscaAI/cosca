package learning

import (
	"os"
	"path/filepath"
	"testing"
)

// TestVaultForAgent_AllAgentsMapped: every known agent resolves to a real vault.
func TestVaultForAgent_AllAgentsMapped(t *testing.T) {
	agents := []string{
		"cosca-kernel", "cosca-backend", "cosca-frontend", "cosca-database",
		"cosca-security", "cosca-ai", "cosca-devops", "cosca-architecture",
		"cosca-qa", "cosca-product", "cosca-runtime", "cosca-cli",
		"cosca-workflow-chief", "cosca-ceo", "cosca-cto", "cosca-documentation",
		"cosca-mobile", "cosca-release", "cosca-cache",
		"cosca-specialist-backend-api", "cosca-specialist-testing-unit",
	}
	for _, a := range agents {
		if VaultForAgent(a) == "misc" {
			t.Errorf("agent %s caiu em misc — mapeamento faltando", a)
		}
	}
	// Unknown agent falls back to misc (never panics).
	if VaultForAgent("cosca-unknown") != "misc" {
		t.Error("agent desconhecido deveria cair em misc")
	}
}

// TestRebuild_FromChainAndBlocks: rebuild deterministic + idempotent.
func TestRebuild_FromChainAndBlocks(t *testing.T) {
	dir := t.TempDir()
	agentsRoot := filepath.Join(dir, "agents")
	vaultDir := filepath.Join(dir, "vaults")

	// Fake agent with 2 chain rows + 2 blocks.
	agentDir := filepath.Join(agentsRoot, "cosca-test")
	if err := os.MkdirAll(filepath.Join(agentDir, "blocks"), 0o755); err != nil {
		t.Fatal(err)
	}

	hashA := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	hashB := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	genesis := "0000000000000000000000000000000000000000000000000000000000000000"

	chain := "# test chain\n" +
		hashA + "|" + genesis + "|2026-09-01|L1|Primeiro aprendizado\n" +
		hashB + "|" + hashA + "|2026-09-02|L2|Segundo aprendizado\n"
	if err := os.WriteFile(filepath.Join(agentDir, "chain.dat"), []byte(chain), 0o644); err != nil {
		t.Fatal(err)
	}

	block1 := "PREV: " + genesis + "\nID: L1\nTIME: 2026-09-01\nLEVEL: 3\nTAGS: #test #nivel3\n---\n## L1 content"
	block2 := "PREV: " + hashA + "\nID: L2\nTIME: 2026-09-02\nLEVEL: 4\nTAGS: #test #nivel4\n---\n## L2 content"
	if err := os.WriteFile(filepath.Join(agentDir, "blocks", hashA+".md"), []byte(block1), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "blocks", hashB+".md"), []byte(block2), 0o644); err != nil {
		t.Fatal(err)
	}

	// First rebuild.
	res, err := Rebuild(vaultDir, agentsRoot)
	if err != nil {
		t.Fatalf("rebuild 1: %v", err)
	}
	if res.TotalTriggers != 2 {
		t.Fatalf("esperava 2 triggers, got %d", res.TotalTriggers)
	}
	if res.MissingBlocks != 0 {
		t.Fatalf("esperava 0 missing, got %d", res.MissingBlocks)
	}

	// Vault content: level/tags extracted from blocks.
	db, err := OpenVault(vaultDir, "misc")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	all, err := AllTriggers(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("esperava 2 triggers no vault, got %d", len(all))
	}
	for _, tr := range all {
		if tr.ID == "L1" {
			if tr.Level != 3 || tr.Tags != "#test #nivel3" {
				t.Errorf("L1: level=%d tags=%q (esperava 3 / #test #nivel3)", tr.Level, tr.Tags)
			}
		}
		if tr.ID == "L2" {
			if tr.Level != 4 || tr.Tags != "#test #nivel4" {
				t.Errorf("L2: level=%d tags=%q (esperava 4 / #test #nivel4)", tr.Level, tr.Tags)
			}
		}
	}

	// Second rebuild: idempotent (same counts). Close the vault first —
	// Windows locks open DB files during the wipe.
	db.Close()

	res2, err := Rebuild(vaultDir, agentsRoot)
	if err != nil {
		t.Fatalf("rebuild 2: %v", err)
	}
	if res2.TotalTriggers != 2 {
		t.Fatalf("rebuild 2: esperava 2 triggers, got %d", res2.TotalTriggers)
	}
}

// TestUpsertTrigger_BlockHashIdempotent: same block_hash overwrites, never duplicates.
func TestUpsertTrigger_BlockHashIdempotent(t *testing.T) {
	dir := t.TempDir()
	db, err := OpenVault(dir, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	tr := Trigger{
		ID:        "L1",
		Agent:     "cosca-a",
		Vault:     "test",
		Date:      "2026-09-01",
		Title:     "Titulo",
		Level:     3,
		Tags:      "#a #b",
		Hash16:    "abcdef1234567890",
		BlockHash: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
	}
	if err := UpsertTrigger(db, tr); err != nil {
		t.Fatal(err)
	}
	// Same block_hash, different title → overwrite (idempotent).
	tr.Title = "Titulo atualizado"
	if err := UpsertTrigger(db, tr); err != nil {
		t.Fatal(err)
	}

	n, err := Count(db)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("esperava 1 trigger (idempotente por block_hash), got %d", n)
	}

	all, _ := AllTriggers(db)
	if all[0].Title != "Titulo atualizado" {
		t.Errorf("titulo deveria ser o atualizado, got %q", all[0].Title)
	}
}

// TestSearch_FTS5: search finds triggers by title/tags.
func TestSearch_FTS5(t *testing.T) {
	dir := t.TempDir()
	db, err := OpenVault(dir, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	tr := Trigger{
		ID:        "L1",
		Agent:     "cosca-kernel",
		Vault:     "test",
		Date:      "2026-09-08",
		Title:     "Fix chain invalida",
		Level:     4,
		Tags:      "#chain #learnings",
		Hash16:    "91f0cdc4f16729a0",
		BlockHash: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
	}
	if err := UpsertTrigger(db, tr); err != nil {
		t.Fatal(err)
	}

	results, err := Search(db, SanitizeFTS("chain invalida"), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("esperava 1 resultado, got %d", len(results))
	}
	if results[0].ID != "L1" {
		t.Errorf("esperava L1, got %s", results[0].ID)
	}
}