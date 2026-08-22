// Evidence Counter + Promotion Engine
//
// This file implements the Cosca Knowledge Lifecycle: knowledge treated as
// code, with traceable evidence and gradual promotion. Every item starts as
// an observation and climbs the ladder
// (observation → learning → hypothesis → theory → law) as evidence
// accumulates. Only the Don's manual approval reaches constitution.
//
// Persistence is JSON at .cosca/knowledge/laws.json — the runtime calls
// Save; the CLI may call it too.
package knowledge

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ── Knowledge levels ────────────────────────────────────────────────────────

// KnowledgeLevel represents the maturity of a knowledge item on the
// evidence ladder.
type KnowledgeLevel string

const (
	// LevelObservation is the entry level: at least 1 evidence.
	// "hoje aconteceu isso".
	LevelObservation KnowledgeLevel = "observation"

	// LevelLearning requires >= 3 evidences and confidence >= 0.70.
	LevelLearning KnowledgeLevel = "learning"

	// LevelHypothesis requires >= 10 evidences and the pattern must be
	// reproducible.
	LevelHypothesis KnowledgeLevel = "hypothesis"

	// LevelTheory requires >= 50 evidences, >= 5 projects and
	// confidence >= 0.90.
	LevelTheory KnowledgeLevel = "theory"

	// LevelLaw requires >= 200 evidences, zero rollbacks and
	// confidence >= 0.99.
	LevelLaw KnowledgeLevel = "law"

	// LevelConstitution is reached exclusively through the Don's manual
	// approval. Automatic promotion never reaches it.
	LevelConstitution KnowledgeLevel = "constitution"
)

// Exact thresholds from the Don's conversation on the Knowledge Lifecycle.
const (
	// ThresholdObservationEvidence is the minimum evidence for observation.
	ThresholdObservationEvidence = 1
	// ThresholdLearningEvidence is the minimum evidence for learning.
	ThresholdLearningEvidence = 3
	// ThresholdHypothesisEvidence is the minimum evidence for hypothesis.
	ThresholdHypothesisEvidence = 10
	// ThresholdTheoryEvidence is the minimum evidence for theory.
	ThresholdTheoryEvidence = 50
	// ThresholdLawEvidence is the minimum evidence for law.
	ThresholdLawEvidence = 200

	// MinConfidenceLearning is the minimum confidence for learning.
	MinConfidenceLearning = 0.70
	// MinConfidenceTheory is the minimum confidence for theory.
	MinConfidenceTheory = 0.90
	// MinConfidenceLaw is the minimum confidence for law.
	MinConfidenceLaw = 0.99

	// MinProjectsForTheory is the minimum number of validating projects
	// required for theory.
	MinProjectsForTheory = 5
)

// rank returns the ladder position of a level; higher means more mature.
// Unknown levels rank below observation so Promote can still fix them.
func (l KnowledgeLevel) rank() int {
	switch l {
	case LevelConstitution:
		return 5
	case LevelLaw:
		return 4
	case LevelTheory:
		return 3
	case LevelHypothesis:
		return 2
	case LevelLearning:
		return 1
	case LevelObservation:
		return 0
	default:
		return -1
	}
}

// ── Evidence ────────────────────────────────────────────────────────────────

// Evidence is a single traceable occurrence that supports a knowledge item.
type Evidence struct {
	ID          string    `json:"id"`          // e.g. "jail-break-1"
	Kind        string    `json:"kind"`        // incident/audit/test/benchmark/project/session
	Source      string    `json:"source"`      // e.g. "auditoria-seguranca-2026-07-31", "test-sandbox"
	Description string    `json:"description"` // what happened
	Timestamp   time.Time `json:"timestamp"`

	// Procedência (P0-P5): de onde a evidência veio. Ausente em evidências
	// antigas → P0 aplicado no load/register (ponte de migração ADD-ON).
	// Para P4/P5 (reproduzível), repository+commit+sha256 permitem reproduzir
	// a evidência exatamente daquela versão do código.
	Provenance ProvenanceLevel `json:"provenance,omitempty"`
	Repository string          `json:"repository,omitempty"`
	Commit     string          `json:"commit,omitempty"`
	Path       string          `json:"path,omitempty"`
	SHA256     string          `json:"sha256,omitempty"`
	Retrieved  time.Time       `json:"retrieved,omitempty"`
}

// KnowledgeItem is a single unit of knowledge with its evidence trail.
type KnowledgeItem struct {
	ID           string         `json:"id"`    // e.g. "K-27"
	Title        string         `json:"title"` // e.g. "Nunca executar como root automaticamente"
	Level        KnowledgeLevel `json:"level"`
	Evidence     []Evidence     `json:"evidence"`
	Projects     int            `json:"projects"`     // number of projects where it was validated
	Confidence   float64        `json:"confidence"`   // 0-1
	Rollbacks    int            `json:"rollbacks"`    // times it was reverted
	Reproducible bool           `json:"reproducible"` // hypothesis: can it be reproduced?
	CreatedAt    time.Time      `json:"created_at"`
	PromotedAt   time.Time      `json:"promoted_at,omitempty"`

	// Estado epistemológico tipado (CKL): o sistema sabe quando NÃO sabe.
	// Ausente em leis antigas → DefaultStatus aplicado no load/register.
	Status             EpistemicStatus `json:"status,omitempty"`
	LastVerified       time.Time       `json:"last_verified,omitempty"`
	VerificationCount  int             `json:"verification_count,omitempty"`
	ContradictionCount int             `json:"contradiction_count,omitempty"`
}

// AddEvidence appends a new evidence and recalculates the item's level.
// An evidence with the same ID replaces the existing one instead of being
// duplicated. A zero Timestamp is filled with the current time.
//
// KnowledgeItem methods are not goroutine-safe on their own; use
// PromotionEngine when items are shared across goroutines.
func (k *KnowledgeItem) AddEvidence(e Evidence) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	for i := range k.Evidence {
		if k.Evidence[i].ID == e.ID {
			k.Evidence[i] = e
			k.Promote()
			return
		}
	}
	k.Evidence = append(k.Evidence, e)
	k.Promote()
}

// Promote applies the thresholds and ratchets the level upward. It returns
// the level reached and whether the item was promoted in this call.
// Promotion is strictly upward: an item never loses its level here, and
// constitution is never reached automatically.
func (k *KnowledgeItem) Promote() (KnowledgeLevel, bool) {
	target := k.CurrentLevel()
	if k.Level == "" || k.Level.rank() < target.rank() {
		k.Level = target
		k.PromotedAt = time.Now()
		// When reaching law, the item is now established knowledge.
		if target == LevelLaw {
			k.Status = StatusKnown
		}
		return target, true
	}
	return k.Level, false
}

// CurrentLevel derives the level from the thresholds without mutating the
// item. It never returns LevelConstitution — that requires the Don's manual
// approval. With zero evidence the item sits at the observation floor.
func (k *KnowledgeItem) CurrentLevel() KnowledgeLevel {
	n := len(k.Evidence)
	switch {
	case n >= ThresholdLawEvidence && k.Rollbacks == 0 && k.Confidence >= MinConfidenceLaw:
		return LevelLaw
	case n >= ThresholdTheoryEvidence && k.Projects >= MinProjectsForTheory && k.Confidence >= MinConfidenceTheory:
		return LevelTheory
	case n >= ThresholdHypothesisEvidence && k.Reproducible:
		return LevelHypothesis
	case n >= ThresholdLearningEvidence && k.Confidence >= MinConfidenceLearning:
		return LevelLearning
	default:
		return LevelObservation
	}
}

// WhyExists answers "why do you exist?" with the evidence trail.
func (k *KnowledgeItem) WhyExists() string {
	level := k.Level
	if level == "" {
		level = k.CurrentLevel()
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s %q [%s]\n", k.ID, k.Title, level)
	fmt.Fprintf(&b, "Por que eu existo? %s, %s, confiança %s, %s.\n",
		pluralize(len(k.Evidence), "evidência", "evidências"),
		pluralize(k.Projects, "projeto", "projetos"),
		strconv.FormatFloat(k.Confidence, 'f', -1, 64),
		pluralize(k.Rollbacks, "rollback", "rollbacks"))
	if k.Reproducible {
		b.WriteString("Reproduzível: sim\n")
	}
	b.WriteString("Evidências:\n")
	for _, ev := range k.Evidence {
		ts := ev.Timestamp.Format("2006-01-02")
		fmt.Fprintf(&b, "  - [%s] %s: %s [%s, %s]\n", ev.ID, ev.Kind, ev.Description, ev.Source, ts)
	}
	return b.String()
}

// ── PromotionEngine ─────────────────────────────────────────────────────────

// PromotionEngine manages the knowledge item registry, automatic promotion
// and JSON persistence. All methods are safe for concurrent use.
type PromotionEngine struct {
	items map[string]*KnowledgeItem
	mu    sync.RWMutex
}

// NewPromotionEngine creates an empty promotion engine.
func NewPromotionEngine() *PromotionEngine {
	return &PromotionEngine{items: make(map[string]*KnowledgeItem)}
}

// Register adds an item to the engine, which takes ownership of it. The item
// needs a unique non-empty ID. A zero CreatedAt is filled in, and an empty
// Level is derived from the thresholds.
func (e *PromotionEngine) Register(item *KnowledgeItem) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if item == nil {
		return errors.New("knowledge: cannot register a nil item")
	}
	if item.ID == "" {
		return errors.New("knowledge: item ID is required")
	}
	if _, exists := e.items[item.ID]; exists {
		return fmt.Errorf("knowledge: item %q is already registered", item.ID)
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}
	if item.Level == "" {
		item.Promote()
	}
	if item.Status == "" {
		item.Status = DefaultStatus(string(item.Level))
	}
	for i := range item.Evidence {
		item.Evidence[i].FillProvenanceDefaults()
	}
	e.items[item.ID] = item
	return nil
}

// Get returns a defensive copy of the item, or ok=false when it is not
// registered. Mutate items through the engine (AddEvidence,
// ApproveConstitution) so the internal state stays consistent.
func (e *PromotionEngine) Get(id string) (*KnowledgeItem, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	item, ok := e.items[id]
	if !ok {
		return nil, false
	}
	return cloneItem(item), true
}

// All returns defensive copies of every registered item, sorted by ID.
func (e *PromotionEngine) All() []*KnowledgeItem {
	e.mu.RLock()
	defer e.mu.RUnlock()

	items := make([]*KnowledgeItem, 0, len(e.items))
	for _, item := range e.items {
		items = append(items, cloneItem(item))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

// AddEvidence records a new evidence for an item and promotes it
// automatically. It returns a copy of the updated item.
func (e *PromotionEngine) AddEvidence(id string, ev Evidence) (*KnowledgeItem, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	item, ok := e.items[id]
	if !ok {
		return nil, fmt.Errorf("knowledge: item %q is not registered", id)
	}
	item.AddEvidence(ev)
	return cloneItem(item), nil
}

// SetConfidence records a measured confidence (0-1) for an item and
// re-evaluates its level. Measured confidence is what separates knowledge
// from opinion (Princípio 3): when the evidence was actually measured, the
// item can be promoted to learning (>= 0.70) instead of staying at the
// observation floor.
func (e *PromotionEngine) SetConfidence(id string, confidence float64) error {
	if confidence < 0 || confidence > 1 {
		return fmt.Errorf("knowledge: confidence %v out of range [0,1]", confidence)
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	item, ok := e.items[id]
	if !ok {
		return fmt.Errorf("knowledge: item %q is not registered", id)
	}
	item.Confidence = confidence
	item.Promote()
	return nil
}

// Reevaluate re-derives every registered item's level from the current
// thresholds (evidence count, confidence, projects, reproducibility) via
// Promote. It exists because the runtime may change an item's measured
// confidence or evidence after the last evaluation (e.g. the ORC pipeline),
// leaving the persisted level stale. It returns the number of items that
// were promoted and the IDs of items that newly reached the law level
// (sorted). Promotion is strictly upward: an item already at a higher level
// (e.g. constitution) is never demoted by a re-evaluation.
func (e *PromotionEngine) Reevaluate() (promoted int, newLaws []string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, item := range e.items {
		before := item.Level
		target, didPromote := item.Promote()
		if !didPromote {
			continue
		}
		promoted++
		if target == LevelLaw && before != LevelLaw {
			newLaws = append(newLaws, item.ID)
		}
	}
	sort.Strings(newLaws)
	return promoted, newLaws
}

// ApproveConstitution is the Don's manual approval: it promotes the item to
// LevelConstitution. This is the only way to reach that level.
func (e *PromotionEngine) ApproveConstitution(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	item, ok := e.items[id]
	if !ok {
		return fmt.Errorf("knowledge: item %q is not registered", id)
	}
	item.Level = LevelConstitution
	item.PromotedAt = time.Now()
	return nil
}

// ApproveLaw is the Don's manual promotion: it promotes the item to LevelLaw
// and marks it as KNOWN. This is the shortcut for items that cannot
// realistically reach 200 evidences but have been validated by the Don.
func (e *PromotionEngine) ApproveLaw(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	item, ok := e.items[id]
	if !ok {
		return fmt.Errorf("knowledge: item %q is not registered", id)
	}
	item.Level = LevelLaw
	item.Status = StatusKnown
	item.PromotedAt = time.Now()
	return nil
}

// VerifyItem atualiza o last_verified e incrementa o verification_count de um
// item registrado. A persistência em disco é responsabilidade do caller (e.g.
// CLI ou ORC) — este método apenas atualiza o estado em memória do engine.
func (e *PromotionEngine) VerifyItem(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	item, ok := e.items[id]
	if !ok {
		return fmt.Errorf("knowledge: item %q is not registered", id)
	}
	item.LastVerified = time.Now()
	item.VerificationCount++
	return nil
}

// VerifyAll itera sobre todos os itens registrados e atualiza o last_verified
// e verification_count de cada um. A operação é best-effort: um item que não
// puder ser verificado não bloqueia os demais (neste momento todos os itens
// são válidos por estarem no mapa, então a operação nunca falha).
func (e *PromotionEngine) VerifyAll() {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	for _, item := range e.items {
		item.LastVerified = now
		item.VerificationCount++
	}
}

// ── Persistence ─────────────────────────────────────────────────────────────

// knowledgeStore is the on-disk JSON layout.
type knowledgeStore struct {
	Version int              `json:"version"`
	Items   []*KnowledgeItem `json:"items"`
}

// LevelRegression describes a knowledge item whose persisted Level is lower
// than what the current evidence thresholds would assign — indicating the
// item regressed (e.g. a law dropped back to learning after a restart).
type LevelRegression struct {
	ID          string
	StoredLevel KnowledgeLevel
	ActualLevel KnowledgeLevel
}

// ValidateLevels checks every registered item for level regression:
// if the stored Level ranks BELOW what CurrentLevel() computes from the
// evidence, the item is restored to its correct level.
//
// Items whose stored Level ranks ABOVE CurrentLevel() (manual promotion
// by the Don via ApproveLaw) are left untouched — the Don's authority is
// never second-guessed.
//
// Returns the list of regressions that were auto-corrected.
func (e *PromotionEngine) ValidateLevels() []LevelRegression {
	e.mu.Lock()
	defer e.mu.Unlock()

	var regressions []LevelRegression
	for _, item := range e.items {
		stored := item.Level
		actual := item.CurrentLevel()
		if stored.rank() < actual.rank() {
			regressions = append(regressions, LevelRegression{
				ID:          item.ID,
				StoredLevel: stored,
				ActualLevel: actual,
			})
			item.Level = actual
			if actual == LevelLaw {
				item.Status = StatusKnown
			}
		}
	}
	return regressions
}

// Load reads a JSON file written by Save and replaces the engine contents.
// Corrupted entries (nil or missing ID) are skipped. After loading, runs
// ValidateLevels to auto-heal any items that regressed below their
// evidence-supported level.
func (e *PromotionEngine) Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("knowledge: load %s: %w", path, err)
	}

	var store knowledgeStore
	if err := json.Unmarshal(data, &store); err != nil {
		return fmt.Errorf("knowledge: parse %s: %w", path, err)
	}

	items := make(map[string]*KnowledgeItem, len(store.Items))
	for _, item := range store.Items {
		if item == nil || item.ID == "" {
			continue
		}
		if item.CreatedAt.IsZero() {
			item.CreatedAt = time.Now()
		}
		if item.Status == "" {
			item.Status = DefaultStatus(string(item.Level))
		}
		for i := range item.Evidence {
			item.Evidence[i].FillProvenanceDefaults()
		}
		items[item.ID] = item
	}

	e.mu.Lock()
	e.items = items
	e.mu.Unlock()

	// Auto-heal: restore any items that regressed below their
	// evidence-supported level (e.g. law→learning after restart).
	_ = e.ValidateLevels()
	return nil
}

// Save persists all items as JSON. The parent directory is created with
// owner-only permissions (0700) and the file with 0600, matching the .cosca
// tree conventions. Items are sorted by ID for deterministic output.
func (e *PromotionEngine) Save(path string) error {
	e.mu.RLock()
	items := make([]*KnowledgeItem, 0, len(e.items))
	for _, item := range e.items {
		items = append(items, cloneItem(item))
	}
	e.mu.RUnlock()

	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })

	data, err := json.MarshalIndent(knowledgeStore{Version: 1, Items: items}, "", "  ")
	if err != nil {
		return fmt.Errorf("knowledge: encode: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("knowledge: create dir %s: %w", dir, err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return fmt.Errorf("knowledge: restrict dir %s: %w", dir, err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("knowledge: write %s: %w", path, err)
	}
	return nil
}

// DefaultLawsPath returns the default persistence path for the law library:
// <home>/.cosca/knowledge/laws.json
func DefaultLawsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".cosca/knowledge/laws.json"
	}
	return filepath.Join(home, ".cosca", "knowledge", "laws.json")
}

// cloneItem returns a shallow copy of the item with a copied evidence slice,
// so callers cannot alias the engine's internal state.
func cloneItem(src *KnowledgeItem) *KnowledgeItem {
	if src == nil {
		return nil
	}
	c := *src
	c.Evidence = make([]Evidence, len(src.Evidence))
	copy(c.Evidence, src.Evidence)
	return &c
}

// pluralize formats n with the correct singular/plural Portuguese form.
func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, plural)
}
