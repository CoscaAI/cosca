// Package sciengine implementa a Scientific Engine (§11 do manifesto
// Creative/Scientific/Media) — Fase 8.
//
// Cada EXPERIMENTO registra (§11): INPUT · PARAMETERS · CODE VERSION ·
// MODEL VERSION · ENVIRONMENT · RESULT · METRICS · TIMESTAMP — reprodutível.
// Integridade (§32): o resultado é classificado (observed/calculated/
// simulated/generated/hypothesis) — nunca inventar.
//
// O Registry persiste experimentos em <projeto>/.cosca/experiments.yaml.
package sciengine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

// ResultKind classifica o resultado (§32 — separar OBSERVED/CALCULATED/...).
type ResultKind string

// Kinds de resultado.
const (
	// Observed — medido/diretamente observado.
	ResultObserved ResultKind = "observed"
	// Calculated — derivado por cálculo determinístico.
	ResultCalculated ResultKind = "calculated"
	// Simulated — resultado de simulação.
	ResultSimulated ResultKind = "simulated"
	// Generated — produzido por IA/modelo.
	ResultGenerated ResultKind = "generated"
	// Hypothesis — hipótese, ainda não verificada.
	ResultHypothesis ResultKind = "hypothesis"
)

// Valid reports se o kind é canônico.
func (k ResultKind) Valid() bool {
	switch k {
	case ResultObserved, ResultCalculated, ResultSimulated, ResultGenerated, ResultHypothesis:
		return true
	}
	return false
}

// Experiment é um experimento reprodutível (§11/§12).
type Experiment struct {
	// ID único (ex.: "exp-001").
	ID string `yaml:"id" json:"id"`
	// Name do experimento.
	Name string `yaml:"name" json:"name"`
	// Input dos dados de entrada (asset IDs ou descrição).
	Input []string `yaml:"input,omitempty" json:"input,omitempty"`
	// Parameters do experimento (sweep: seed, lr, epochs...).
	Parameters map[string]any `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	// CodeVersion — versão do código que rodou (commit hash).
	CodeVersion string `yaml:"code_version" json:"code_version"`
	// ModelVersion — versão do modelo (se aplicável).
	ModelVersion string `yaml:"model_version,omitempty" json:"model_version,omitempty"`
	// Environment — runtime (OS, deps, GPU).
	Environment string `yaml:"environment,omitempty" json:"environment,omitempty"`
	// Result do experimento.
	Result any `yaml:"result" json:"result"`
	// Kind classifica o resultado (§32).
	Kind ResultKind `yaml:"kind" json:"kind"`
	// Metrics — métricas do experimento (accuracy, loss, time...).
	Metrics map[string]float64 `yaml:"metrics,omitempty" json:"metrics,omitempty"`
	// Timestamp UTC do experimento.
	Timestamp string `yaml:"timestamp" json:"timestamp"`
}

// New cria um experimento validado.
func New(id, name string, kind ResultKind) (*Experiment, error) {
	if id == "" || name == "" {
		return nil, fmt.Errorf("experiment id and name are required")
	}
	if !kind.Valid() {
		return nil, fmt.Errorf("invalid result kind %q (valid: observed, calculated, simulated, generated, hypothesis)", kind)
	}
	return &Experiment{
		ID: id, Name: name, Kind: kind,
		Parameters: map[string]any{},
		Metrics:    map[string]float64{},
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// SetMetric adiciona/atualiza uma métrica.
func (e *Experiment) SetMetric(key string, value float64) {
	if e.Metrics == nil {
		e.Metrics = map[string]float64{}
	}
	e.Metrics[key] = value
}

// Metric devolve uma métrica (0 se ausente).
func (e *Experiment) Metric(key string) float64 {
	if e.Metrics == nil {
		return 0
	}
	return e.Metrics[key]
}

// =============================================================================
// Registry
// =============================================================================

// Registry persiste experimentos em <projectRoot>/.cosca/experiments.yaml.
type Registry struct {
	root string
	path string
	// Experiments por ID.
	byID map[string]*Experiment
}

// DefaultDir é o diretório do registry dentro de um projeto.
const DefaultDir = ".cosca"

// FileName do registro de experimentos.
const FileName = "experiments.yaml"

// Open abre (ou cria) o registry de um projeto.
func Open(projectRoot string) (*Registry, error) {
	path := filepath.Join(projectRoot, DefaultDir, FileName)
	r := &Registry{root: projectRoot, path: path, byID: make(map[string]*Experiment)}
	if err := os.MkdirAll(filepath.Join(projectRoot, DefaultDir), 0o755); err != nil {
		return nil, fmt.Errorf("create .cosca: %w", err)
	}
	if data, err := os.ReadFile(path); err == nil {
		var exps []*Experiment
		if err := yaml.Unmarshal(data, &exps); err != nil {
			return nil, fmt.Errorf("parse experiments: %w", err)
		}
		for _, e := range exps {
			if e != nil && e.ID != "" {
				r.byID[e.ID] = e
			}
		}
	}
	return r, nil
}

// Path devolve o caminho do arquivo.
func (r *Registry) Path() string { return r.path }

// save persiste o registro (atômico: tmp + rename).
func (r *Registry) save() error {
	exps := make([]*Experiment, 0, len(r.byID))
	for _, e := range r.byID {
		exps = append(exps, e)
	}
	sort.Slice(exps, func(i, j int) bool { return exps[i].ID < exps[j].ID })
	data, err := yaml.Marshal(exps)
	if err != nil {
		return fmt.Errorf("marshal experiments: %w", err)
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write experiments tmp: %w", err)
	}
	return os.Rename(tmp, r.path)
}

// Add registra um experimento e salva.
func (r *Registry) Add(e *Experiment) error {
	if e == nil || e.ID == "" {
		return fmt.Errorf("experiment is required")
	}
	if _, exists := r.byID[e.ID]; exists {
		return fmt.Errorf("experiment %q already exists", e.ID)
	}
	r.byID[e.ID] = e
	return r.save()
}

// Get devolve um experimento por ID.
func (r *Registry) Get(id string) (*Experiment, bool) {
	e, ok := r.byID[id]
	return e, ok
}

// List devolve todos os experimentos ordenados por ID.
func (r *Registry) List() []*Experiment {
	out := make([]*Experiment, 0, len(r.byID))
	for _, e := range r.byID {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Count devolve o número de experimentos.
func (r *Registry) Count() int { return len(r.byID) }

// Best devolve o experimento com a melhor métrica (maior valor). Devolve
// false se a métrica não existir em nenhum experimento.
func (r *Registry) Best(metric string) (*Experiment, bool) {
	var best *Experiment
	var bestVal float64
	for _, e := range r.byID {
		v, exists := e.Metrics[metric]
		if !exists {
			continue
		}
		if best == nil || v > bestVal {
			best = e
			bestVal = v
		}
	}
	return best, best != nil
}

// ResultKindsList devolve a lista legível de kinds.
func ResultKindsList() string {
	return "observed, calculated, simulated, generated, hypothesis"
}
