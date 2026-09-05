// Package vector — PartitionStore: a LEITURA modular pós-corte (Plano D D5).
//
// O corte (D3b/D4) roteou a ESCRITA para os módulos (vector-*.db), mas o
// vecStore — que a busca semântica usa — continuava apontando para o
// monolito (knowledge.db, 0 vetores pós-drenagem). O PartitionStore é o
// proxy que implementa vector.Store e agrega as PARTIÇÕES na leitura:
//
//	Search(query, limit) → busca em CADA partição → merge por score (top-K)
//
// A escrita (Store/Delete) continua no monolito (que o db build sincroniza
// para os módulos) — o PartitionStore é READ-aggregator, como o vectoragg.
package vector

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
)

// PartitionStore implementa vector.Store agregando as partições vector-*.db
// na leitura. As operações de escrita delegam ao store base (monolito).
type PartitionStore struct {
	base      Store         // o store do monolito (escrita + fallback)
	partition func() []Store // devolve os stores das partições (soma na leitura)
	dim       int
}

// PartitionStoreConfig configura o PartitionStore.
type PartitionStoreConfig struct {
	// Base é o store do monolito (escrita + fallback quando sem partições).
	Base Store
	// PartitionPaths são os caminhos das partições vector-*.db (soma na
	// leitura). Pode conter glob (ex: "vector-*.db").
	PartitionPaths []string
	// Dimension é a dimensionalidade esperada.
	Dimension int
}

// NewPartitionStore cria o PartitionStore. Cada partição é aberta como um
// SQLiteVec read-only na leitura (lazy).
func NewPartitionStore(cfg PartitionStoreConfig) (*PartitionStore, error) {
	if cfg.Base == nil {
		return nil, fmt.Errorf("partition store: base store required")
	}
	ps := &PartitionStore{
		base: cfg.Base,
		dim:  cfg.Dimension,
	}
	ps.partition = func() []Store {
		var stores []Store
		for _, pattern := range cfg.PartitionPaths {
			matches, err := filepath.Glob(pattern)
			if err != nil {
				continue
			}
			sort.Strings(matches)
			for _, p := range matches {
				db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(p)+"?mode=ro")
				if err != nil {
					continue
				}
				store, err := NewSQLiteVec(SQLiteVecConfig{
					DB:        db,
					Dimension: ps.dim,
				})
				if err != nil {
					_ = db.Close()
					continue
				}
				stores = append(stores, store)
			}
		}
		return stores
	}
	return ps, nil
}

// Search agrega as partições: busca em cada uma e faz merge por score.
// Retorna os top-K GLOBAIS (não por partição).
func (p *PartitionStore) Search(query []float64, limit int) ([]SearchResult, error) {
	parts := p.partition()
	if len(parts) == 0 {
		// Sem partições (corte não ativo) — usa o monolito.
		return p.base.Search(query, limit)
	}

	merged := make([]SearchResult, 0, limit)
	for _, part := range parts {
		res, err := part.Search(query, limit)
		if err != nil {
			// Uma partição com erro não derruba a busca — loga e segue.
			continue
		}
		merged = append(merged, res...)
	}
	// Merge: ordena por score desc (maior similaridade primeiro), top-K global.
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})
	if len(merged) > limit {
		merged = merged[:limit]
	}
	return merged, nil
}

// SearchWithFilter agrega com filtro de metadata (mesmo merge por score).
func (p *PartitionStore) SearchWithFilter(query []float64, limit int, filter map[string]string) ([]SearchResult, error) {
	parts := p.partition()
	if len(parts) == 0 {
		return p.base.SearchWithFilter(query, limit, filter)
	}
	merged := make([]SearchResult, 0, limit)
	for _, part := range parts {
		res, err := part.SearchWithFilter(query, limit, filter)
		if err != nil {
			continue
		}
		merged = append(merged, res...)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})
	if len(merged) > limit {
		merged = merged[:limit]
	}
	return merged, nil
}

// Count soma os vetores de todas as partições.
func (p *PartitionStore) Count() (int, error) {
	parts := p.partition()
	if len(parts) == 0 {
		return p.base.Count()
	}
	total := 0
	for _, part := range parts {
		if n, err := part.Count(); err == nil {
			total += n
		}
	}
	return total, nil
}

// Stats agrega as estatísticas das partições.
func (p *PartitionStore) Stats() (VectorStats, error) {
	parts := p.partition()
	if len(parts) == 0 {
		return p.base.Stats()
	}
	var total VectorStats
	for _, part := range parts {
		if s, err := part.Stats(); err == nil {
			total.TotalVectors += s.TotalVectors
		}
	}
	return total, nil
}

// Dimension devolve a dimensionalidade esperada.
func (p *PartitionStore) Dimension() int { return p.dim }

// ── Escrita (delega ao monolito — o db build sincroniza para os módulos) ──

func (p *PartitionStore) Store(dim int, vectors []VectorRecord) error {
	return p.base.Store(dim, vectors)
}
func (p *PartitionStore) Delete(ids []string) error          { return p.base.Delete(ids) }
func (p *PartitionStore) DeleteByDocument(id string) error   { return p.base.DeleteByDocument(id) }
func (p *PartitionStore) DeleteByEntity(id string) error     { return p.base.DeleteByEntity(id) }
func (p *PartitionStore) Rebuild() error                     { return p.base.Rebuild() }
func (p *PartitionStore) Close() error                       { return p.base.Close() }

// ── Interfaces opcionais (TransactionalStore / StoreTxCommitter) ──
// A escrita do PartitionStore delega ao monolito (base) — então as
// capacidades transacionais também delegam ao base quando ele as suporta.
// Isso preserva os contratos que os consumidores (indexer, search) esperam:
// sem suporte, os testes que usam StoreTx degradariam (como aconteceu).

// StoreTx implementa TransactionalStore delegando ao base.
func (p *PartitionStore) StoreTx(tx *sql.Tx, dimension int, vectors []VectorRecord) error {
	if ts, ok := p.base.(TransactionalStore); ok {
		return ts.StoreTx(tx, dimension, vectors)
	}
	return fmt.Errorf("partition store: base does not support StoreTx")
}

// StoreTxCommitted implementa StoreTxCommitter delegando ao base.
func (p *PartitionStore) StoreTxCommitted() {
	if c, ok := p.base.(StoreTxCommitter); ok {
		c.StoreTxCommitted()
	}
}
