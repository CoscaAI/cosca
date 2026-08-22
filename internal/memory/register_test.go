package memory

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func seedAgentDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	learnings := "# cosca-kernel — Semantic Learnings\n\n" +
		"## L1 | 2026-08-09 | Primeiro aprendizado | L4 | #a #level-4 | 0000000000000000\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "learnings.md"), []byte(learnings), 0o644))

	// L1 block hash (fake, 64 hex) + genesis prev.
	const l1Hash = "0000000000000000000000000000000000000000000000000000000000000001"
	chain := "# Cosca Kernel Memory Blockchain — Chain Ledger\n" +
		"# Total blocks: 1\n" +
		l1Hash + "|" + genesisHash + "|2026-08-09|L1|Primeiro aprendizado\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "chain.dat"), []byte(chain), 0o644))

	return dir
}

func TestRegisterLearning_FullFlow(t *testing.T) {
	dir := seedAgentDir(t)

	in := LearningInput{
		Agent:      "cosca-kernel",
		Title:      "Segundo aprendizado",
		Level:      4,
		Tags:       []string{"#b", "#level-4"},
		Task:       "registrar aprendizado",
		Technique:  "RegisterLearning",
		Outcome:    "success",
		Confidence: 0.90,
		Learned:    "o fluxo funciona, validado por teste",
		Next:       "testar",
		Related:    "L1",
	}

	res, err := RegisterLearning(dir, in)
	require.NoError(t, err)
	require.Equal(t, "L2", res.ID)
	require.Len(t, res.Hash, 64)
	require.Len(t, res.Hash16, 16)

	// 4ª Muralha: aprendizado limpo (com evidência observável) é aprovado.
	require.NotNil(t, res.Guard)
	require.True(t, res.Guard.Approved)

	// Block written with hash = sha256(full content).
	content, err := os.ReadFile(filepath.Join(dir, "blocks", res.Hash+".md"))
	require.NoError(t, err)
	sum := sha256.Sum256(content)
	require.Equal(t, res.Hash, fmt.Sprintf("%x", sum))
	require.Contains(t, string(content), "PREV: 0000000000000000000000000000000000000000000000000000000000000001")
	require.Contains(t, string(content), "ID: L2")
	require.Contains(t, string(content), "## L2 — ")

	// learnings.md got the trigger line.
	learnings, err := os.ReadFile(filepath.Join(dir, "learnings.md"))
	require.NoError(t, err)
	require.Contains(t, string(learnings), "## L2 | ")
	require.Contains(t, string(learnings), res.Hash16)

	// chain.dat got the row + merkle regenerated.
	chain, err := os.ReadFile(filepath.Join(dir, "chain.dat"))
	require.NoError(t, err)
	require.Contains(t, string(chain), res.Hash+"|0000000000000000000000000000000000000000000000000000000000000001|")
	require.FileExists(t, filepath.Join(dir, "merkle", "index.json"))
}

func TestRegisterLearning_Validation(t *testing.T) {
	dir := seedAgentDir(t)

	_, err := RegisterLearning(dir, LearningInput{Title: "", Level: 4, Tags: []string{"#a"}})
	require.Error(t, err)

	_, err = RegisterLearning(dir, LearningInput{Title: "x", Level: 9, Tags: []string{"#a"}})
	require.Error(t, err)

	_, err = RegisterLearning(dir, LearningInput{Title: "x", Level: 4, Tags: nil})
	require.Error(t, err)
}

func TestPreviewLearning_DoesNotWrite(t *testing.T) {
	dir := seedAgentDir(t)

	in := LearningInput{Agent: "cosca-kernel", Title: "Preview", Level: 3, Tags: []string{"#c"}, Outcome: "success"}

	res, content, err := PreviewLearning(dir, in)
	require.NoError(t, err)
	require.Equal(t, "L2", res.ID)
	require.Contains(t, content, "## L2 — ")

	// Nothing written: no L2 block, learnings.md unchanged, chain.dat still 1 row.
	require.NoFileExists(t, filepath.Join(dir, "blocks", res.Hash+".md"))
	learnings, _ := os.ReadFile(filepath.Join(dir, "learnings.md"))
	require.NotContains(t, string(learnings), "## L2 ")
	chain, _ := os.ReadFile(filepath.Join(dir, "chain.dat"))
	require.NotContains(t, string(chain), res.Hash)
	require.NoFileExists(t, filepath.Join(dir, "merkle", "index.json"))
}

// inflatedLearning é o perfil de um aprendizado que a 4ª Muralha deve
// reprovar: narrativa inflada (vaidade "prova suprema", auto-engrandecimento
// "maior valor epistêmico"), confiança inflada (0.99) e alegação de sucesso
// sem evidência observável (FALSE SUCCESS).
func inflatedLearning() LearningInput {
	return LearningInput{
		Agent:      "cosca-kernel",
		Title:      "Prova suprema do kernel",
		Level:      4,
		Tags:       []string{"#b", "#level-4"},
		Task:       "auto-promoção",
		Technique:  "narrativa inflada",
		Outcome:    "success",
		Confidence: 0.99,
		Learned:    "sou o melhor kernel de todos — maior valor epistêmico; confiança 100%",
		Next:       "nada",
		Related:    "L1",
	}
}

func TestRegisterLearning_GuardBlocksInflatedNarrative(t *testing.T) {
	dir := seedAgentDir(t)

	_, err := RegisterLearning(dir, inflatedLearning())
	require.Error(t, err)
	require.Contains(t, err.Error(), "memoryguard: aprendizado bloqueado pela 4ª Muralha")
	require.Contains(t, err.Error(), "VAIDADE")
	require.Contains(t, err.Error(), "AUTO-ENGRANDECIMENTO")
	require.Contains(t, err.Error(), "CONFIANÇA INFLADA")
	require.Contains(t, err.Error(), "FALSE SUCCESS")

	// Fail-closed provado: nada tocou o disco.
	require.NoFileExists(t, filepath.Join(dir, "blocks"))
	learnings, _ := os.ReadFile(filepath.Join(dir, "learnings.md"))
	require.NotContains(t, string(learnings), "## L2 ")
	chain, _ := os.ReadFile(filepath.Join(dir, "chain.dat"))
	require.NotContains(t, string(chain), "|L2|")
	require.NoFileExists(t, filepath.Join(dir, "merkle", "index.json"))
}

func TestRegisterLearning_GuardBlocksObfuscation(t *testing.T) {
	dir := seedAgentDir(t)

	in := inflatedLearning()
	in.Learned = "eu sou ｇｅｎｉｕｓ — абракадабра de prova suprema"

	_, err := RegisterLearning(dir, in)
	require.Error(t, err)
	require.Contains(t, err.Error(), "OFUSCAÇÃO")
	require.NoFileExists(t, filepath.Join(dir, "blocks"))
}

func TestRegisterLearning_ForceOverridesGuard(t *testing.T) {
	dir := seedAgentDir(t)

	res, err := RegisterLearning(dir, inflatedLearning(), WithForce())
	require.NoError(t, err)
	require.NotNil(t, res.Guard)
	require.False(t, res.Guard.Approved)
	require.NotEmpty(t, res.Guard.Reasons)

	// O block foi gravado mesmo reprovado (override do Don).
	content, err := os.ReadFile(filepath.Join(dir, "blocks", res.Hash+".md"))
	require.NoError(t, err)
	sum := sha256.Sum256(content)
	require.Equal(t, res.Hash, fmt.Sprintf("%x", sum))

	learnings, _ := os.ReadFile(filepath.Join(dir, "learnings.md"))
	require.Contains(t, string(learnings), "## L2 | ")
	chain, _ := os.ReadFile(filepath.Join(dir, "chain.dat"))
	require.Contains(t, string(chain), "|L2|")
	require.FileExists(t, filepath.Join(dir, "merkle", "index.json"))
}

func TestPreviewLearning_ExposesGuardWithoutBlocking(t *testing.T) {
	dir := seedAgentDir(t)

	res, _, err := PreviewLearning(dir, inflatedLearning())
	require.NoError(t, err)
	require.NotNil(t, res.Guard)
	require.False(t, res.Guard.Approved)
	require.NotEmpty(t, res.Guard.Reasons)

	// Preview não bloqueia e não escreve.
	require.NoFileExists(t, filepath.Join(dir, "blocks"))
	require.NoFileExists(t, filepath.Join(dir, "merkle", "index.json"))
	learnings, _ := os.ReadFile(filepath.Join(dir, "learnings.md"))
	require.NotContains(t, string(learnings), "## L2 ")
	chain, _ := os.ReadFile(filepath.Join(dir, "chain.dat"))
	require.NotContains(t, string(chain), "|L2|")
}

func TestNextLearningID(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "learnings.md"),
		[]byte("## L5 | x | y | L4 | #a | 0000000000000000\n## L12 | x | y | L4 | #a | 0000000000000000\n"), 0o644))

	id, err := nextLearningID(filepath.Join(dir, "learnings.md"))
	require.NoError(t, err)
	require.Equal(t, "L13", id)
}

func TestLastChainHash_EmptyFallsBackToGenesis(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "chain.dat"), []byte("# apenas comentário\n"), 0o644))

	h, err := lastChainHash(filepath.Join(dir, "chain.dat"))
	require.NoError(t, err)
	require.Equal(t, genesisHash, h)
}
