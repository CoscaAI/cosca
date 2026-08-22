//
// SEED inicial do Cosca Knowledge Lifecycle (CKL) — as leis REAIS que o
// Cosca já produziu nas ondas de segurança.
//
// Cada lei nasce com evidências reais do repositório: o incidente F001
// (fuga da jail), as auditorias red team (red-team-A1..C2, A4) e os testes
// que comprovam o fix (TestJail_RootUnsafe, TestRegisterDisabled,
// TestIndexPathTraversal, TestValidateMemoryID, TestRevokeAndRotate_*,
// TestRefresh_*). É o "why" do Cosca: conhecimento nunca nasce como verdade
// (Princípio 1) — nasce com evidência rastreável (Princípio 2).
//
// O seed roda uma única vez: na primeira execução de `cosca knowledge law
// list`, quando .cosca/knowledge/laws.json ainda não existe. É idempotente
// por construção — um arquivo existente nunca é sobrescrito. Usa apenas as
// assinaturas existentes do PromotionEngine (Register, Save, Load).
//

package cli

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/CoscaAI/cosca/internal/knowledge"
)

// ── Seed metadados (para o comando upgrade) ──────────────────────────────────

// SeedMetadata contém os metadados canônicos de uma lei seed
// (reproducible + provenance) para upgrade de leis existentes.
type SeedMetadata struct {
	ID           string
	Reproducible bool
	Evidences    []knowledge.Evidence // com provenance fields preenchidos
}

// SeedDefinitions retorna os metadados canônicos das 5 leis seed. Usado pelo
// comando `cosca knowledge law upgrade` para atualizar metadados de leis já
// existentes no disco (que não passam pelo seed — o seed só roda uma vez).
func SeedDefinitions() []SeedMetadata {
	return []SeedMetadata{
		{
			ID:           "K-01",
			Reproducible: true,
			Evidences: []knowledge.Evidence{
				seedEvidenceProvenance("F001", "incident", knowledge.ProvenanceOfficial),
				seedEvidenceProvenance("red-team-A1", "audit", knowledge.ProvenanceOfficial),
				seedEvidenceProvenance("TestJail_RootUnsafe", "test", knowledge.ProvenanceIndependent,
					"pkg/cosca/jail_test.go"),
				seedEvidenceProvenance("Onda-1-fix", "test", knowledge.ProvenanceOfficial),
			},
		},
		{
			ID:           "K-02",
			Reproducible: true,
			Evidences: []knowledge.Evidence{
				seedEvidenceProvenance("red-team-A2", "audit", knowledge.ProvenanceOfficial),
				seedEvidenceProvenance("Onda-1-fix", "fix", knowledge.ProvenanceOfficial),
				seedEvidenceProvenance("TestRegisterDisabled", "test", knowledge.ProvenanceIndependent,
					"api/rest/handler/onda2_test.go"),
			},
		},
		{
			ID:           "K-03",
			Reproducible: true,
			Evidences: []knowledge.Evidence{
				seedEvidenceProvenance("red-team-C1", "audit", knowledge.ProvenanceOfficial),
				seedEvidenceProvenance("red-team-C2", "audit", knowledge.ProvenanceOfficial),
				seedEvidenceProvenance("Onda-1-fix", "fix", knowledge.ProvenanceOfficial),
				seedEvidenceProvenance("TestIndexPathTraversal", "test", knowledge.ProvenanceIndependent,
					"internal/indexer/indexer_test.go"),
				seedEvidenceProvenance("TestValidateMemoryID", "test", knowledge.ProvenanceIndependent,
					"internal/memory/store_test.go"),
			},
		},
		{
			ID:           "K-04",
			Reproducible: true,
			Evidences: []knowledge.Evidence{
				seedEvidenceProvenance("red-team-A4", "audit", knowledge.ProvenanceOfficial),
				seedEvidenceProvenance("Onda-2-fix", "fix", knowledge.ProvenanceOfficial),
				seedEvidenceProvenance("TestRevokeAndRotate_ReuseDetected", "test", knowledge.ProvenanceIndependent,
					"internal/auth/tokenstore_test.go"),
				seedEvidenceProvenance("TestRefresh_RotatesAndRevokes", "test", knowledge.ProvenanceIndependent,
					"internal/auth/tokenstore_test.go"),
			},
		},
		{
			ID:           "K-05",
			Reproducible: true,
			Evidences: []knowledge.Evidence{
				seedEvidenceProvenance("red-team-A1", "audit", knowledge.ProvenanceOfficial),
				seedEvidenceProvenance("Onda-1-fix", "fix", knowledge.ProvenanceOfficial),
				seedEvidenceProvenance("TestJail_RootUnsafe", "test", knowledge.ProvenanceIndependent,
					"pkg/cosca/jail_test.go"),
			},
		},
	}
}

// seedEvidenceProvenance cria um Evidence stub com os campos de procedência
// preenchidos. Para evidências de teste (Kind="test"), o SHA256 do arquivo é
// calculado do sistema de arquivos local quando o arquivo existe; caso
// contrário (arquivo não encontrado), fica vazio e o Load preenche P0.
func seedEvidenceProvenance(id, kind string, provenance knowledge.ProvenanceLevel, repoPath ...string) knowledge.Evidence {
	ev := knowledge.Evidence{
		ID:         id,
		Kind:       kind,
		Provenance: provenance,
		Repository: "https://github.com/CoscaAI/coscaV2",
		Commit:     "ac34e4c01f866710299dc6704b029af8337cb6dc",
	}
	if len(repoPath) > 0 && repoPath[0] != "" {
		ev.Path = repoPath[0]
		// Tenta calcular SHA256 do arquivo real no workspace.
		if sha, err := fileSHA256(repoPath[0]); err == nil {
			ev.SHA256 = sha
		}
	}
	return ev
}

// fileSHA256 calcula o SHA256 do arquivo no sistema de arquivos local
// (relativo ao working directory). Retorna erro se o arquivo não existir
// ou não puder ser lido — o caller decide se trata como erro ou continua.
func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h), nil
}

// seedLaws registra as 5 leis REAIS do Cosca (ondas de segurança Onda-1 e
// Onda-2) no engine. Cada evidência aponta para um incidente, auditoria,
// fix ou teste que existe de fato no repositório.
//
// Nenhuma lei é inventada: os IDs e fontes abaixo correspondem a commits,
// auditorias e testes reais (ver git log e pkg/cosca/jail_test.go,
// api/rest/handler/onda2_test.go, internal/auth/tokenstore_test.go,
// internal/memory/store_test.go, internal/indexer/indexer_test.go).
//
// As 5 leis têm >= 3 evidências e confiança medida >= 0.70 → todas nascem
// no nível "learning" (PromotionEngine.Register recalcula o nível).
func seedLaws(engine *knowledge.PromotionEngine) error {
	// repo e commit do HEAD atual — compartilhado por todas as evidências.
	repo := "https://github.com/CoscaAI/coscaV2"
	commit := "ac34e4c01f866710299dc6704b029af8337cb6dc"

	// Helper: evidence de teste com procedência P5 (código/teste reproduzível
	// com repository + commit + path + sha256).
	testEvidence := func(id, source, desc string, createdAt time.Time, path string) knowledge.Evidence {
		ev := knowledge.Evidence{
			ID:          id,
			Kind:        "test",
			Source:      source,
			Description: desc,
			Timestamp:   createdAt,
			Provenance:  knowledge.ProvenanceIndependent, // P5
			Repository:  repo,
			Commit:      commit,
			Path:        path,
		}
		if sha, err := fileSHA256(path); err == nil {
			ev.SHA256 = sha
		}
		return ev
	}

	// Helper: evidência de auditoria/incidente/fix com procedência P3
	// (fonte oficial — nomeada mas sem commit reproduzível).
	auditEvidence := func(id, kind, source, desc string, createdAt time.Time) knowledge.Evidence {
		return knowledge.Evidence{
			ID:          id,
			Kind:        kind,
			Source:      source,
			Description: desc,
			Timestamp:   createdAt,
			Provenance:  knowledge.ProvenanceOfficial, // P3
			Repository:  repo,
		}
	}

	laws := []*knowledge.KnowledgeItem{
		seedLaw(
			"K-01",
			"Nunca executar como root automaticamente",
			seedTime(30, 9, 15),
			0.95, 1, true,
			auditEvidence("F001", "incident", "F001",
				"Fuga da jail: processo executou como root fora do sandbox",
				seedTime(30, 9, 15)),
			auditEvidence("red-team-A1", "audit", "red-team-A1",
				"Red team A1: jail anulável — executava como root",
				seedTime(31, 10, 0)),
			testEvidence("TestJail_RootUnsafe", "TestJail_RootUnsafe",
				"Teste confirma: sandbox recusa execução como root",
				seedTime(31, 14, 30), "pkg/cosca/jail_test.go"),
			auditEvidence("Onda-1-fix", "test", "Onda-1-fix",
				"Fix Onda 1: jail fail-closed recusa root automaticamente",
				seedTime(31, 15, 0)),
		),
		seedLaw(
			"K-02",
			"Registro público desabilitado por padrão",
			seedTime(31, 10, 10),
			0.90, 1, true, // reproducible: TestRegisterDisabled existe
			auditEvidence("red-team-A2", "audit", "red-team-A2",
				"Red team A2: endpoint de registro estava público",
				seedTime(31, 10, 10)),
			auditEvidence("Onda-1-fix", "fix", "Onda-1-fix",
				"Fix Onda 1: registro desabilitado por padrão (fail-closed)",
				seedTime(31, 15, 5)),
			testEvidence("TestRegisterDisabled", "TestRegisterDisabled",
				"Teste confirma: registro rejeitado por padrão",
				seedTime(31, 14, 35), "api/rest/handler/onda2_test.go"),
		),
		seedLaw(
			"K-03",
			"Path traversal bloqueado por containment",
			seedTime(31, 10, 20),
			0.93, 1, true, // reproducible: TestIndexPathTraversal + TestValidateMemoryID existem
			auditEvidence("red-team-C1", "audit", "red-team-C1",
				"Red team C1: path traversal no indexador",
				seedTime(31, 10, 20)),
			auditEvidence("red-team-C2", "audit", "red-team-C2",
				"Red team C2: path traversal via memory ID",
				seedTime(31, 10, 25)),
			auditEvidence("Onda-1-fix", "fix", "Onda-1-fix",
				"Fix Onda 1: containment bloqueia traversal (fail-closed)",
				seedTime(31, 15, 10)),
			testEvidence("TestIndexPathTraversal", "TestIndexPathTraversal",
				"Teste confirma: documentos fora da raiz são rejeitados",
				seedTime(31, 14, 40), "internal/indexer/indexer_test.go"),
			testEvidence("TestValidateMemoryID", "TestValidateMemoryID",
				"Teste confirma: memory ID com traversal é rejeitado",
				seedTime(31, 14, 45), "internal/memory/store_test.go"),
		),
		seedLaw(
			"K-04",
			"Refresh token exige rotação e revogação",
			seedTime(31, 10, 30),
			0.92, 1, true, // reproducible: TestRevokeAndRotate + TestRefresh existem
			auditEvidence("red-team-A4", "audit", "red-team-A4",
				"Red team A4: refresh token reusável era vulnerável",
				seedTime(31, 10, 30)),
			auditEvidence("Onda-2-fix", "fix", "Onda-2-fix",
				"Fix Onda 2: refresh rotaciona e revoga (fail-closed)",
				seedTime(31, 16, 0)),
			testEvidence("TestRevokeAndRotate_ReuseDetected", "TestRevokeAndRotate_ReuseDetected",
				"Teste confirma: reuse de token rotacionado é detectado",
				seedTime(31, 16, 30), "internal/auth/tokenstore_test.go"),
			testEvidence("TestRefresh_RotatesAndRevokes", "TestRefresh_RotatesAndRevokes",
				"Teste confirma: refresh rotaciona e revoga o antigo",
				seedTime(31, 16, 35), "internal/auth/tokenstore_test.go"),
		),
		seedLaw(
			"K-05",
			"Jail recusa executar como root",
			seedTime(31, 10, 0),
			0.95, 1, true, // reproducible: TestJail_RootUnsafe existe
			auditEvidence("red-team-A1", "audit", "red-team-A1",
				"Red team A1: jail deve recusar root automaticamente",
				seedTime(31, 10, 0)),
			auditEvidence("Onda-1-fix", "fix", "Onda-1-fix",
				"Fix Onda 1: jail fail-closed recusa root automaticamente",
				seedTime(31, 15, 0)),
			testEvidence("TestJail_RootUnsafe", "TestJail_RootUnsafe",
				"Teste confirma: sandbox recusa execução como root",
				seedTime(31, 14, 30), "pkg/cosca/jail_test.go"),
		),
	}

	for _, law := range laws {
		if err := engine.Register(law); err != nil {
			return fmt.Errorf("seed leis: %w", err)
		}
	}
	return nil
}

// seedLaw monta uma lei seed com CreatedAt = data da primeira evidência.
func seedLaw(id, title string, createdAt time.Time, confidence float64, projects int, reproducible bool, evs ...knowledge.Evidence) *knowledge.KnowledgeItem {
	return &knowledge.KnowledgeItem{
		ID:           id,
		Title:        title,
		Evidence:     evs,
		Projects:     projects,
		Confidence:   confidence,
		Reproducible: reproducible,
		CreatedAt:    createdAt,
	}
}

// seedTime devolve um timestamp determinístico do período das ondas de
// segurança (2026-07-30/31) — mantém o seed estável e reproduzível.
func seedTime(day, hour, min int) time.Time {
	return time.Date(2026, 7, day, hour, min, 0, 0, time.UTC)
}

// ensureSeededLaws garante que o arquivo runtime de leis exista. Na primeira
// execução (arquivo inexistente), cria-o com as 5 leis seed via
// PromotionEngine.Save. Retorna true se o seed foi aplicado.
//
// Idempotente: um arquivo existente — mesmo vazio — nunca é sobrescrito.
func ensureSeededLaws(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("stat %s: %w", path, err)
	}

	engine := knowledge.NewPromotionEngine()
	if err := seedLaws(engine); err != nil {
		return false, err
	}
	if err := engine.Save(path); err != nil {
		return false, fmt.Errorf("salvar seed em %s: %w", path, err)
	}
	return true, nil
}
