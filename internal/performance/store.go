package performance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// =============================================================================
// Memória Experimental (L316) — guarda resultados ACEITOS E REJEITADOS.
//
// Um experimento refutado é conhecimento útil: um futuro agente que pensar
// "vamos pré-calcular normas!" consulta o histórico e encontra o registro
// REJECTED — não gasta tokens redescobrindo o que já foi refutado com dado.
//
// Persistência: arquivo JSONL append-only (mesma filosofia do chain.dat —
// histórico imutável, auditável).
// =============================================================================

// Store persiste ExperimentResults. Thread-safe.
type Store struct {
	mu   sync.Mutex
	path string
}

// NewStore cria um store em `path` (arquivo JSONL). Cria o diretório se
// necessário. Vazio/inexistente é válido (histórico começa vazio).
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Record append um resultado ao histórico (aceito ou rejeitado — ambos contam).
func (s *Store) Record(res *ExperimentResult) error {
	if res == nil {
		return fmt.Errorf("record: nil result")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.path == "" {
		return nil // sem persistência (modo volátil)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("record: mkdir: %w", err)
	}
	data, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("record: marshal: %w", err)
	}
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("record: open: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("record: write: %w", err)
	}
	return nil
}

// Lookup consulta o histórico por (workload name, experiment name) e devolve
// o resultado MAIS RECENTE — para que um agente descubra "já testamos isso".
func (s *Store) Lookup(workloadName, experimentName string) (*ExperimentResult, bool) {
	results, err := s.LoadAll()
	if err != nil {
		return nil, false
	}
	var found *ExperimentResult
	for i := range results {
		r := &results[i]
		if r.Workload.Name == workloadName && r.Experiment.Name == experimentName {
			found = r // itera em ordem; o último vence
		}
	}
	return found, found != nil
}

// LoadAll carrega todo o histórico (para relatórios e consultas).
func (s *Store) LoadAll() ([]ExperimentResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("load: %w", err)
	}
	var results []ExperimentResult
	for _, line := range splitLines(string(data)) {
		if line == "" {
			continue
		}
		var r ExperimentResult
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue // linha corrompida: ignora, não derruba (append-only resiliente)
		}
		results = append(results, r)
	}
	return results, nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
