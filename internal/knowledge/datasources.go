// Package knowledge — D1 do PLANO D (corte do ADR-013, 2026-09-01).
//
// DataSources é o provider de datasources do corte: abre os 5 módulos
// físicos (core/graph/projects/vector-*/fts) em vez do knowledge.db único.
// É a fundação que permite ao indexer escrever nos módulos (D3) e ao ORC
// parar de reconstruir indexer com o banco antigo (D4).
//
// Hoje (D1) o provider é LEITURA — abre os módulos e expõe cada conexão;
// a escrita nos módulos (D3) usa esta mesma fundação. O catálogo de paths
// é o ModuleCatalog do vectoragg (mesma convenção do split Fase C).
//
// Fail-closed: um módulo ausente/corrompido NUNCA degrada silenciosamente —
// o Open devolve erro claro nomeando o módulo e o path.
package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/CoscaAI/cosca/internal/sqlite"
	"github.com/CoscaAI/cosca/internal/vectoragg"
)

// moduleName é o nome lógico de um módulo do corte (ADR-013 §2.1).
type moduleName string

// Nomes lógicos dos módulos físicos.
const (
	modCore     moduleName = "core"
	modGraph    moduleName = "graph"
	modProjects moduleName = "projects"
	modVector   moduleName = "vector"
	modFTS      moduleName = "fts"
)

// DataSources é o provider dos módulos físicos do corte.
type DataSources struct {
	// paths é o catálogo módulo-lógico → caminho físico.
	paths map[moduleName]string
	// dbs é a conexão SQLite aberta de cada módulo presente.
	dbs map[moduleName]*sqlite.DB
	// present lista os módulos que existem no disco.
	present []moduleName
}

// ModuleCatalogForDataSources resolve o catálogo módulo→path a partir do
// data dir (.cosca). Reusa a convenção do vectoragg (split Fase C): o vector
// é particionado por responsabilidade (vector-*.db) — o provider abre a soma
// das partições (cada uma é um datasource independente, agregado na leitura).
func ModuleCatalogForDataSources(dataDir string) vectoragg.ModuleCatalog {
	return vectoragg.ModuleCatalog{
		vectoragg.ModuleVector:   filepath.Join(dataDir, "vector-*.db"),
		vectoragg.ModuleGraph:    filepath.Join(dataDir, "graph.db"),
		vectoragg.ModuleFTS:      filepath.Join(dataDir, "fts.db"),
		vectoragg.ModuleProjects: filepath.Join(dataDir, "projects.db"),
	}
}

// OpenDataSources abre os módulos físicos do corte a partir do data dir.
// Retorna um provider com os módulos presentes; módulos ausentes são
// reportados (sem erro — o corte é progressivo: enquanto um módulo não
// existe, o Engine usa o knowledge.db para aquela fatia).
func OpenDataSources(dataDir string) (*DataSources, error) {
	ds := &DataSources{
		paths: make(map[moduleName]string),
		dbs:   make(map[moduleName]*sqlite.DB),
	}

	// Mapeamento módulo-lógico → path físico (convenção do split Fase C).
	// O vector é particionado: vector-*.db (o glob é expandido abaixo).
	candidates := map[moduleName]string{
		modCore:     filepath.Join(dataDir, "core.db"),
		modGraph:    filepath.Join(dataDir, "graph.db"),
		modProjects: filepath.Join(dataDir, "projects.db"),
		modFTS:      filepath.Join(dataDir, "fts.db"),
		modVector:   filepath.Join(dataDir, "vector-*.db"),
	}

	for name, path := range candidates {
		// Vector é particionado — expande o glob e abre cada partição.
		if name == modVector {
			parts, err := filepath.Glob(path)
			if err != nil || len(parts) == 0 {
				continue // vetor ainda não splitado — usa knowledge.db
			}
			sort.Strings(parts)
			for _, p := range parts {
				db, err := sqlite.Open(sqlite.DefaultConfig(p))
				if err != nil {
					ds.closeAll()
					return nil, fmt.Errorf("datasource %s (%s): %w", name, p, err)
				}
				ds.paths[name] = p
				ds.dbs[name] = db
				ds.present = append(ds.present, name)
				break // uma conexão por módulo (a partição principal)
			}
			continue
		}

		// Só abre módulo que EXISTE — nunca cria (o sqlite.Open criaria o
		// arquivo; o corte é progressivo: fatia ausente = usa knowledge.db).
		if _, statErr := os.Stat(path); statErr != nil {
			continue
		}
		db, err := sqlite.Open(sqlite.DefaultConfig(path))
		if err != nil {
			// Módulo corrompido = falha fechada (nunca degrada silencioso),
			// fechando os já abertos para não vazar conexões.
			ds.closeAll()
			return nil, fmt.Errorf("datasource %s (%s): %w", name, path, err)
		}
		ds.paths[name] = path
		ds.dbs[name] = db
		ds.present = append(ds.present, name)
	}

	sort.Slice(ds.present, func(i, j int) bool { return ds.present[i] < ds.present[j] })
	return ds, nil
}

// closeAll fecha todas as conexões abertas (usado no caminho de erro).
func (ds *DataSources) closeAll() {
	for _, db := range ds.dbs {
		_ = db.Close()
	}
}

// HasModule reporta se um módulo lógico está presente no disco.
func (ds *DataSources) HasModule(name moduleName) bool {
	if ds == nil {
		return false
	}
	_, ok := ds.dbs[name]
	return ok
}

// DB devolve a conexão de um módulo presente (nil se ausente).
func (ds *DataSources) DB(name moduleName) *sqlite.DB {
	if ds == nil {
		return nil
	}
	return ds.dbs[name]
}

// Path devolve o caminho físico de um módulo ("" se ausente).
func (ds *DataSources) Path(name moduleName) string {
	if ds == nil {
		return ""
	}
	return ds.paths[name]
}

// Present lista os módulos presentes (sorted).
func (ds *DataSources) Present() []moduleName {
	if ds == nil {
		return nil
	}
	out := append([]moduleName(nil), ds.present...)
	return out
}

// Close fecha todas as conexões abertas.
func (ds *DataSources) Close() error {
	if ds == nil {
		return nil
	}
	var firstErr error
	for _, db := range ds.dbs {
		if err := db.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
