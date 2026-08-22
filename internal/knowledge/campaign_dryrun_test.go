// Package knowledge — campanha performance, FASE L352 (dry-run dos filtros
// T1-T4). READ-ONLY: nenhuma escrita, nenhuma alteração de corpus/índice/
// produção. Classifica cada chunk como KEEP ou FILTER_CANDIDATE pelas regras:
//
//	T1 vazio            len(trim(content)) == 0
//	T2 separador puro   trim é só chars de separação (---, ***, ===, ...)
//	T3 zero alfanuméricos (unicode \p{L}\p{N}) — NUNCA por comprimento
//	T4 estrutura vazia  trim é só #, ` ou > (headings/fences/blockquotes)
//
// Nenhuma regra usa comprimento mínimo. Valida explicitamente que "Go 1.26",
// "v1.2.3", "RAG", "API", "SQL", "404" NÃO são filtrados (têm alfanuméricos).
package knowledge

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/cache"
)

var (
	reT2   = regexp.MustCompile(`^[\-*_=~—–·•]+$`)
	reAlnum = regexp.MustCompile(`[\p{L}\p{N}]`)
	reT4   = regexp.MustCompile("^[#`>]+$")
)

func t1Vazio(s string) bool          { return strings.TrimSpace(s) == "" }
func t2Separador(s string) bool      { return reT2.MatchString(strings.TrimSpace(s)) }
func t3ZeroAlnum(s string) bool      { return !reAlnum.MatchString(strings.TrimSpace(s)) }
func t4EstruturaVazia(s string) bool { return reT4.MatchString(strings.TrimSpace(s)) }

// filterCandidate: um chunk é candidato se QUALQUER regra T1-T4 dispara.
func filterCandidate(content string) (bool, [4]bool) {
	var f [4]bool
	f[0] = t1Vazio(content)
	f[1] = t2Separador(content)
	f[2] = t3ZeroAlnum(content)
	f[3] = t4EstruturaVazia(content)
	any := f[0] || f[1] || f[2] || f[3]
	return any, f
}

// TestCampaignDryRunFilters — o dry-run T1-T4 (FASE L352, read-only).
func TestCampaignDryRunFilters(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	root := "/home/cosca/Documents/cosca"
	coscaDir := filepath.Join(root, ".cosca")
	if _, err := os.Stat(filepath.Join(coscaDir, "knowledge.db")); err != nil {
		t.Skip("sem knowledge.db")
	}
	eng, err := New(Config{
		DBPath:      filepath.Join(coscaDir, "knowledge.db"),
		RootDir:     root,
		AutoMigrate: true,
		CacheConfig: cache.Config{EnabledLevels: []cache.Level{cache.Level(255)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.Init(); err != nil {
		t.Fatal(err)
	}
	defer eng.Close()

	// Carrega chunks + flag hasVector.
	rows, err := eng.db.Conn().Query(
		"SELECT c.id, c.document_id, c.content, EXISTS(SELECT 1 FROM vectors v WHERE v.chunk_id = c.id) FROM chunks c")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	type chunk struct {
		id, doc, content string
		hasVector        bool
	}
	var chunks []chunk
	for rows.Next() {
		var c chunk
		if err := rows.Scan(&c.id, &c.doc, &c.content, &c.hasVector); err != nil {
			t.Fatal(err)
		}
		chunks = append(chunks, c)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	// ── 1. Classificação ──
	byRule := [4]int{}
	union := 0
	var candidates []chunk
	var candFlags [][4]bool
	keepSmall := 0
	for _, c := range chunks {
		any, f := filterCandidate(c.content)
		if any {
			union++
			candidates = append(candidates, c)
			candFlags = append(candFlags, f)
			for i := 0; i < 4; i++ {
				if f[i] {
					byRule[i]++
				}
			}
		} else {
			// KEEP pequenos (próximos do limite): curto mas com alfanuméricos.
			if len(c.content) > 0 && len(c.content) <= 12 && reAlnum.MatchString(c.content) {
				keepSmall++
			}
		}
	}

	// Interseções (T2/T4 ⊆ T3 por construção — medir mesmo assim).
	t2AndT3, t4AndT3, t2Only, t4Only := 0, 0, 0, 0
	for _, f := range candFlags {
		if f[1] && f[2] {
			t2AndT3++
		}
		if f[3] && f[2] {
			t4AndT3++
		}
		if f[1] && !f[2] {
			t2Only++
		}
		if f[3] && !f[2] {
			t4Only++
		}
	}

	// ── 2. Percentuais ──
	withVec, orphans := 0, 0
	for _, c := range chunks {
		if c.hasVector {
			withVec++
		} else {
			orphans++
		}
	}
	canVec, canOrphan := 0, 0
	for _, c := range candidates {
		if c.hasVector {
			canVec++
		} else {
			canOrphan++
		}
	}

	// ── 3. Distribuição por documento ──
	byDoc := map[string]int{}
	for _, c := range candidates {
		byDoc[c.doc]++
	}
	docRank := make([]string, 0, len(byDoc))
	for d := range byDoc {
		docRank = append(docRank, d)
	}
	sort.Slice(docRank, func(a, b int) bool { return byDoc[docRank[a]] > byDoc[docRank[b]] })

	// ── 4. Amostras ──
	t.Logf("── L352 DRY-RUN T1-T4 (READ-ONLY, db real) ──")
	t.Logf("chunks=%d (com vetor=%d, órfãos=%d)", len(chunks), withVec, orphans)
	t.Logf("por regra: T1 vazio=%d  T2 separador=%d  T3 zero-alnum=%d  T4 estrutura=%d",
		byRule[0], byRule[1], byRule[2], byRule[3])
	t.Logf("interseções: T2∩T3=%d  T4∩T3=%d  T2 sem T3=%d  T4 sem T3=%d",
		t2AndT3, t4AndT3, t2Only, t4Only)
	t.Logf("CANDIDATOS ÚNICOS=%d (%.2f%% do corpus)  com vetor=%d (%.2f%% dos %d)  órfãos=%d (%.2f%% dos %d)",
		union, float64(union)*100/float64(len(chunks)),
		canVec, float64(canVec)*100/float64(withVec), withVec,
		canOrphan, float64(canOrphan)*100/float64(orphans), orphans)
	t.Logf("KEEP pequenos (≤12 chars, alnum): %d preservados", keepSmall)

	t.Logf("── distribuição por documento (top 8) ──")
	for i := 0; i < 8 && i < len(docRank); i++ {
		d := docRank[i]
		t.Logf("  %s: %d candidatos", shortID(d), byDoc[d])
	}
	t.Logf("  ... total de documentos afetados: %d", len(byDoc))

	t.Logf("── exemplos FILTER_CANDIDATE (amostra 15) ──")
	shown := 0
	for ci, c := range candidates {
		if shown >= 15 {
			break
		}
		if len(c.content) <= 60 {
			t.Logf("  [%s] %q", ruleTag(candFlags[ci]), c.content)
			shown++
		}
	}
	t.Logf("── exemplos KEEP pequenos (próximos do limite, amostra 10) ──")
	shown = 0
	for _, c := range chunks {
		if shown >= 10 {
			break
		}
		if !filterCandidateBool(c.content) && len(c.content) > 0 && len(c.content) <= 12 {
			t.Logf("  KEEP %q", c.content)
			shown++
		}
	}

	// ── 5. Auditoria: 100 candidatos ESPALHADOS → SAFE/REQUIRES/DO_NOT ──
	// (os 100 primeiros eram enviesados — início do db, fragmentos de JSON)
	safe, review, doNot := 0, 0, 0
	var safeExamples, reviewExamples, doNotExamples []string
	step := len(candidates) / 100
	if step < 1 {
		step = 1
	}
	for i := 0; i < len(candidates) && i < 100*step; i += step {
		c := candidates[i]
		trimmed := strings.TrimSpace(c.content)
		switch {
		case trimmed == "":
			safe++
			if len(safeExamples) < 3 {
				safeExamples = append(safeExamples, "<vazio>")
			}
		case reT2.MatchString(trimmed) || reT4.MatchString(trimmed):
			safe++ // separadores puros / estruturas vazias
			if len(safeExamples) < 3 {
				safeExamples = append(safeExamples, trimmed)
			}
		case reAlnum.MatchString(c.content):
			doNot++ // TEM alfanuméricos → bug do filtro (não deveria estar aqui)
			if len(doNotExamples) < 3 {
				doNotExamples = append(doNotExamples, c.content)
			}
		case isAmbiguous(c.content):
			review++ // símbolos com possível significado em código/JSON
			if len(reviewExamples) < 8 {
				reviewExamples = append(reviewExamples, c.content)
			}
		default:
			safe++ // símbolos sem estrutura (!!!, §§§, emoji-only)
			if len(safeExamples) < 3 {
				safeExamples = append(safeExamples, trimmed)
			}
		}
	}
	t.Logf("── AUDITORIA (100 candidatos espalhados) ──")
	t.Logf("SAFE_TO_FILTER=%d  REQUIRES_REVIEW=%d  DO_NOT_FILTER=%d", safe, review, doNot)
	for _, ex := range safeExamples {
		t.Logf("  SAFE   %q", ex)
	}
	for _, ex := range reviewExamples {
		t.Logf("  REVIEW %q", ex)
	}
	for _, ex := range doNotExamples {
		t.Logf("  DONOT  %q", ex)
	}

	// ── 6. Validação dos exemplos críticos (proibido filtrar por tamanho) ──
	for _, s := range []string{"Go 1.26", "v1.2.3", "RAG", "API", "SQL", "404", "C++17"} {
		any, _ := filterCandidate(s)
		if any {
			t.Errorf("FALSO POSITIVO ESTRUTURAL: %q seria filtrado!", s)
		} else {
			t.Logf("  PRESERVADO: %q", s)
		}
	}
}

func filterCandidateBool(s string) bool {
	any, _ := filterCandidate(s)
	return any
}

func ruleTag(f [4]bool) string {
	var b strings.Builder
	if f[0] {
		b.WriteString("T1")
	}
	if f[1] {
		b.WriteString("T2")
	}
	if f[2] {
		b.WriteString("T3")
	}
	if f[3] {
		b.WriteString("T4")
	}
	return b.String()
}

// isAmbiguous: conteúdo sem alfanuméricos mas com chars que PODEM ter
// significado em contexto de código/docs (->, =>, ::, [], {}, <>, >>>, ```).
func isAmbiguous(s string) bool {
	return strings.ContainsAny(s, "><=:[]{}()/\\|+&%$#@!?")
}

func shortID(s string) string {
	if len(s) > 16 {
		return s[:16] + "…"
	}
	return s
}
