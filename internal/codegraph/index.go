// F3 do ADR-019 — Índice persistente e RAM-first do code graph.
//
// Index pre-computa o grafo + os sinais de cada arquivo uma vez (RAM-first) e
// busca sem re-ler o codebase. Publicação ATÔMICA (I2, fail-closed): Save grava
// em <path>.tmp e renomeia — uma interrupção nunca deixa um índice parcial
// (o índice antigo permanece íntegro até o rename).
//
// Determinístico (I1), zero LLM/zero rede. Compõe sobre codegraph.BuildGraph +
// codeembed.Signals (nada duplicado).
package codegraph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/CoscaAI/cosca/internal/codeembed"
	"github.com/CoscaAI/cosca/internal/graph"
)

// Index é o índice em memória do code graph + sinais por arquivo.
type Index struct {
	Root    string                         `json:"root"`
	Dim     int                            `json:"dim"`
	BuiltAt time.Time                      `json:"built_at"`
	Graph   *graph.Graph                   `json:"graph"`
	Signals map[string]codeembed.SignalSet `json:"signals"` // node ID (file) -> sinais
}

// BuildIndex constrói o índice (RAM-first) do diretório `root`.
func BuildIndex(root string, dim int) (*Index, error) {
	if dim <= 0 {
		dim = codeembed.DefaultDim
	}
	g, err := BuildGraph(root)
	if err != nil {
		return nil, fmt.Errorf("build graph: %w", err)
	}
	ix := &Index{
		Root:    root,
		Dim:     dim,
		BuiltAt: time.Now().UTC(),
		Graph:   g,
		Signals: map[string]codeembed.SignalSet{},
	}
	for _, n := range g.GetAllNodes() {
		if n.Type != "file" || n.Path == "" {
			continue
		}
		content, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(n.Path)))
		if rerr != nil {
			continue // fail-open: arquivo ilegível é pulado
		}
		ix.Signals[n.ID] = codeembed.Signals(string(content), dim)
	}
	return ix, nil
}

// SearchSimilar devolve os arquivos mais similares à consulta, usando os sinais
// pré-computados (sem re-ler o codebase). Determinístico (I1).
func (ix *Index) SearchSimilar(query string, limit int) []SearchHit {
	if ix == nil {
		return nil
	}
	if limit <= 0 {
		limit = 10
	}
	q := codeembed.Signals(query, ix.Dim)
	var hits []SearchHit
	for id, sig := range ix.Signals {
		score := codeembed.Fuse(q, sig)
		node, ok := ix.Graph.GetNode(id)
		if !ok {
			continue
		}
		lang, _ := metadataString(node, "lang")
		hits = append(hits, SearchHit{
			File:  node.Name,
			Path:  node.Path,
			Lang:  lang,
			Score: score,
		})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Path < hits[j].Path // tie-break determinístico (I1)
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

// Save publica o índice de forma ATÔMICA (I2, fail-closed): grava em <path>.tmp
// e renomeia. Uma falha durante o write deixa o índice ANTIGO intacto.
func (ix *Index) Save(path string) error {
	if ix == nil {
		return fmt.Errorf("index: nil")
	}
	data, err := json.Marshal(ix)
	if err != nil {
		return fmt.Errorf("index save: marshal: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("index save: mkdir: %w", err)
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("index save: write tmp: %w", err)
	}
	// fsync + rename atômico: o nome final só existe se o tmp estiver completo.
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("index save: rename: %w", err)
	}
	return nil
}

// Load lê um índice previamente publicado. Inexistente devolve nil,nil.
func LoadIndex(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("index load: %w", err)
	}
	var ix Index
	if err := json.Unmarshal(data, &ix); err != nil {
		return nil, fmt.Errorf("index load: unmarshal: %w", err)
	}
	return &ix, nil
}
