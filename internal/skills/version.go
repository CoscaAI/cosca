package skills

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/CoscaAI/cosca/internal/ledger"
)

// SkillVersionRecord é o registro de versão imutável de uma skill (F1 §4).
// Ele amarra versão → proveniência → evidência → benchmark → regressões → status
// num único artefato, persistido de forma tamper-evidente no ledger (reuso do
// "Caderno da Família" — NÃO se cria um new store de versões).
type SkillVersionRecord struct {
	Skill         string        `json:"skill"`
	Version       string        `json:"version"`
	PrevVersion   string        `json:"prev_version,omitempty"`
	Lifecycle     LifecycleState `json:"lifecycle"`
	Origin        string        `json:"origin,omitempty"`         // human|agent|evolution|imported
	ProvenanceID  string        `json:"provenance_id,omitempty"`  // ref p/ provenance.Claim
	EvidenceRefs  []string      `json:"evidence_refs,omitempty"`  // ref p/ trace.Event
	BenchmarkRef  string        `json:"benchmark_ref,omitempty"`  // ref p/ skilleval benchmark
	SuccessRate   float64       `json:"success_rate,omitempty"`   // median/IQR (skilleval Condition)
	Regressions   []string      `json:"known_regressions,omitempty"`
	TTL           string        `json:"ttl,omitempty"`            // política de aging (ex.: "168h")
	Grandfathered bool          `json:"grandfathered,omitempty"`  // cláusula de avô (I1)
	GatePassed    bool          `json:"gate_passed,omitempty"`    // veredito do gate determinístico
	PromotedAt    string        `json:"promoted_at,omitempty"`    // RFC3339 UTC
}

// VersionStore persiste SkillVersionRecord. A implementação padrão é
// ledger-backed (VersionStore default); testes podem usar um fake em memória.
type VersionStore interface {
	Put(key string, rec *SkillVersionRecord) error
	Get(key string) (*SkillVersionRecord, error)
}

// VersionKey devolve a chave canônica do registro de versão no ledger.
func VersionKey(name, version string) string { return "skill." + name + ".version." + version }

// LedgerVersionStore é uma VersionStore apoiada em ledger.Ledger (reuso).
// O CAS do ledger garante que uma versão não é sobrescrita cegamente (P2) e a
// cadeia de hash é a prova de adulteração (I5).
type LedgerVersionStore struct{ L *ledger.Ledger }

// NewLedgerVersionStore abre (ou reusa) um ledger em dir para registros de
// versão de skills.
func NewLedgerVersionStore(dir string) (*LedgerVersionStore, error) {
	if dir == "" {
		return nil, fmt.Errorf("skills version ledger: dir is required")
	}
	l, err := ledger.Open(filepath.Join(dir, "skill-version-ledger"))
	if err != nil {
		return nil, err
	}
	return &LedgerVersionStore{L: l}, nil
}

// Put persiste o registro. expectedVersion=0 exige que a chave ainda não exista
// (cria de versão nova); para supersede, o chamador deve re-ler a versão e
// passar expectedVersion correspondente.
func (s *LedgerVersionStore) Put(key string, rec *SkillVersionRecord) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if _, err := s.L.Put(key, data, 0); err != nil {
		return fmt.Errorf("skills version ledger: put %s: %w", key, err)
	}
	return nil
}

// Get lê o registro de uma versão.
func (s *LedgerVersionStore) Get(key string) (*SkillVersionRecord, error) {
	val, _, ok := s.L.Get(key)
	if !ok {
		return nil, fmt.Errorf("version %q not found", key)
	}
	var rec SkillVersionRecord
	if err := json.Unmarshal(val, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// Close fecha o ledger de versões (libera o WAL).
func (s *LedgerVersionStore) Close() error { return s.L.Close() }

// MemVersionStore é um store em memória (uso em testes/CLI sem disco).
type MemVersionStore struct{ data map[string]*SkillVersionRecord }

func NewMemVersionStore() *MemVersionStore { return &MemVersionStore{data: map[string]*SkillVersionRecord{}} }
func (s *MemVersionStore) Put(key string, rec *SkillVersionRecord) error {
	s.data[key] = rec
	return nil
}
func (s *MemVersionStore) Get(key string) (*SkillVersionRecord, error) {
	rec, ok := s.data[key]
	if !ok {
		return nil, fmt.Errorf("version %q not found", key)
	}
	return rec, nil
}

// RecordVersion compõe um SkillVersionRecord a partir da governança de uma
// skill e o persiste no store (default: ledger). É o ponto de aterrissagem da
// F1: grava a versão de forma imutável e verificável (I5/I3), e aplica a
// cláusula de avô quando a skill não tem benchmark ainda.
func RecordVersion(store VersionStore, name, version string, g SkillGovernance, prev string) (*SkillVersionRecord, error) {
	if name == "" || version == "" {
		return nil, fmt.Errorf("RecordVersion: name and version are required")
	}
	life := g.Status
	if !ValidLifecycle(life) {
		life = DefaultLifecycle()
	}
	rec := &SkillVersionRecord{
		Skill:         name,
		Version:       version,
		PrevVersion:   prev,
		Lifecycle:     life,
		Origin:        g.Origin,
		ProvenanceID:  g.ProvenanceID,
		EvidenceRefs:  g.EvidenceRefs,
		BenchmarkRef:  g.BenchmarkRef,
		SuccessRate:   g.SuccessRate,
		Regressions:   g.KnownRegressions,
		TTL:           g.TTL,
		Grandfathered: g.Grandfathered,
		GatePassed:    g.GatePassed,
		PromotedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if err := store.Put(VersionKey(name, version), rec); err != nil {
		return nil, err
	}
	return rec, nil
}
