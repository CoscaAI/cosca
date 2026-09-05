// P2.5 do ADR-020 — manutenção INCREMENTAL do índice (content-hash diff).
//
// Hoje BuildIndex re-embute TODOS os arquivos a cada chamada. Isto torna a
// manutenção O(tudo). O Update faz a manutenção O(delta):
//
//   - hash do conteúdo por arquivo (sha256, determinístico I1);
//   - diff com os hashes armazenados → added/changed/deleted;
//   - NOOP rápido se nada mudou (~ms);
//   - re-embute só changed/added; remove deleted; reconstroi o grafo de imports;
//   - Save atômico (tmp+rename) = guarda de crash (I2): interrupção no meio da
//     atualização deixa o índice ANTIGO (correto) intacto → próxima Update re-diff.
//
// Determinístico (I1), zero LLM/rede. Compõe sobre BuildIndex/collectFiles.
package codegraph

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"time"

	"github.com/CoscaAI/cosca/internal/codeembed"
	"github.com/CoscaAI/cosca/internal/codeindex"
)

// hashBytes devolve o sha256 hex de um trecho (content-hash; determinístico I1).
func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Update faz a manutenção incremental do índice em `root`. Se o índice é nil,
// equivale a BuildIndex. Retorna o índice atualizado (novo slice/map quando
// necessário). Determinístico (I1).
func (ix *Index) Update(root string, dim int) (*Index, error) {
	if ix == nil {
		return BuildIndex(root, dim)
	}
	if dim <= 0 {
		dim = codeembed.DefaultDim
	}

	files, _ := collectFiles(root)
	current := map[string]string{}
	for _, fi := range files {
		content, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(fi.rel)))
		if rerr != nil {
			continue // fail-open
		}
		current[fi.rel] = hashBytes(content)
	}

	changed, added, deleted := diffHashes(ix.Hashes, current)

	// NOOP rápido: nada mudou (manutenção O(0)).
	if len(changed) == 0 && len(added) == 0 && len(deleted) == 0 {
		ix.BuiltAt = time.Now().UTC()
		return ix, nil
	}

	// Remover deletados (o indexamento O(0) não os re-embute).
	for _, f := range deleted {
		delete(ix.Signals, f)
		delete(ix.Meta, f)
		delete(ix.Symbols, f)
	}

	// Re-embute só changed/added (a parte cara — embedding).
	for _, rel := range append(changed, added...) {
		content, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if rerr != nil {
			continue
		}
		ix.Signals[rel] = codeembed.Signals(string(content), dim)
		ix.Meta[rel] = FileMeta{Name: filepath.Base(rel), Path: rel, Lang: languageFor(rel)}
		if languageFor(rel) == "go" {
			if syms, serr := codeindex.ExtractFile(filepath.Join(root, filepath.FromSlash(rel))); serr == nil {
				ix.Symbols[rel] = syms
			}
		}
	}

	// Recompute do grafo de imports (topologia pode ter mudado com add/del/rename).
	if g, gerr := BuildGraph(root); gerr == nil {
		ix.Graph = g
	}

	ix.Hashes = current
	ix.BuiltAt = time.Now().UTC()
	return ix, nil
}

// diffHashes compara dois conjuntos de hashes (prev vs cur) e devolve
// (changed, added, deleted) por rel path.
func diffHashes(prev, cur map[string]string) (changed, added, deleted []string) {
	for rel, h := range cur {
		if ph, ok := prev[rel]; ok {
			if ph != h {
				changed = append(changed, rel)
			}
		} else {
			added = append(added, rel)
		}
	}
	for rel := range prev {
		if _, ok := cur[rel]; !ok {
			deleted = append(deleted, rel)
		}
	}
	return
}

// languageFor reusa LanguageForFile (extensão → lang).
func languageFor(rel string) string {
	lang, _ := LanguageForFile(rel)
	return lang
}
