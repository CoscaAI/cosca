package worldmodel

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// EntityStamp é o selo epistêmico por entidade (I3/I4) — o "resourceVersion" do
// informer k8s traduzido ao Cosca (ADR-023 item 5). Cada entrada do lister sabe
// QUANDO foi vista (AsOf), DE ONDE veio (Source, I3) e em QUE nível epistêmico
// (TrustState, I4). Revision é monotônica e serve de fencing (ordem de escrita).
type EntityStamp struct {
	AsOf     time.Time         `json:"as_of"`
	Source   ObservationSource `json:"source,omitempty"`
	Trust    TrustState        `json:"trust"`
	Revision uint64            `json:"revision"`
	Hash     string            `json:"hash,omitempty"`
}

// StampedEntity é o par entidade + selo epistêmico visto pelo lister. É um READ
// CACHE: o chamador NUNCA deve mutar as slices/maps retornados (contrato do
// informer — o reconciler lê, não escreve; I3/I4 por construção).
type StampedEntity struct {
	Entity WorldEntity
	Stamp  EntityStamp
}

// KnownKind devolve o rótulo epistêmico final (I4 — saber≠ver): combina o
// TrustState do selo com a Visibility da entidade. Se o selo é Unknown, a crença
// recua (degradada) — o sistema NÃO age sobre dado que sabe que não sabe.
func (se StampedEntity) KnownKind() Visibility {
	if se.Stamp.Trust == TrustUnknown {
		return VisibilityStale
	}
	if se.Entity.Visibility != "" {
		return se.Entity.Visibility
	}
	return se.Entity.KnownKind()
}

// UpdateOutcome classifica o resultado de um Upsert.
type UpdateOutcome struct {
	Applied  bool   // false = rejeitado por fencing (escrita velha/stale)
	IsNew    bool   // entidade nova no cache
	Changed  bool   // entidade existente com conteúdo diferente
	Revision uint64 // revision da entrada após a operação (0 se rejeitado)
}

// ResyncDiff é o resultado de um resync (reconciliação com a fonte autoritativa).
type ResyncDiff struct {
	Version uint64
	Added   []string
	Updated []string
	Removed []string
}

// Empty informa se o resync não detectou divergência.
func (d ResyncDiff) Empty() bool {
	return len(d.Added) == 0 && len(d.Updated) == 0 && len(d.Removed) == 0
}

// Lister é o cache de leitura do mundo (padrão "shared informer → lister" do k8s;
// ADR-023 item 5): decisores/reconcilers leem AQUI, nunca de uma única request —
// I3/I4 por construção. Suporta resync periódico (auto-cura de eventos perdidos):
// o cache é reconciliado contra um snapshot autoritativo (I2 — divergência é
// exposta, nunca "consertada" silenciosamente).
//
// Determinístico e stdlib-only. Concorrente (RWMutex).
type Lister struct {
	mu       sync.RWMutex
	entities map[string]*entry
	byType   map[EntityType]map[string]struct{}
	version  uint64        // resourceVersion do cache inteiro
	resync   time.Duration // período de resync (0 = manual)
	lastSync time.Time
}

type entry struct {
	entity WorldEntity
	stamp  EntityStamp
}

// ListerOption configura o Lister.
type ListerOption func(*Lister)

// WithResyncPeriod define o período de resync automático (k8s resync period).
func WithResyncPeriod(d time.Duration) ListerOption {
	return func(l *Lister) { l.resync = d }
}

// NewLister cria um lister de mundo vazio.
func NewLister(opts ...ListerOption) *Lister {
	l := &Lister{
		entities: make(map[string]*entry),
		byType:   make(map[EntityType]map[string]struct{}),
	}
	for _, o := range opts {
		o(l)
	}
	return l
}

// hashEntity computa um hash de conteúdo da entidade (detecção de mudança).
func hashEntity(e WorldEntity) string {
	b, err := json.Marshal(e)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:])
}

// Upsert insere/atualiza uma entidade com selo epistêmico. Fencing (I2): uma
// escrita com AsOf ANTERIOR à já armazenada é REJEITADA (nunca regride a uma
// crença mais velha). Revision é monotônica. Retorna o outcome da operação.
func (l *Lister) Upsert(e WorldEntity, src ObservationSource, trust TrustState, asOf time.Time) UpdateOutcome {
	l.mu.Lock()
	defer l.mu.Unlock()

	if ex, ok := l.entities[e.ID]; ok {
		// Fencing: não regride no tempo epistêmico.
		if !asOf.IsZero() && !ex.stamp.AsOf.IsZero() && asOf.Before(ex.stamp.AsOf) {
			return UpdateOutcome{Applied: false, Revision: ex.stamp.Revision}
		}
		hash := hashEntity(e)
		changed := hash != ex.stamp.Hash
		newV := ex.stamp.Revision + 1
		ex.entity = e
		ex.stamp = EntityStamp{AsOf: asOf, Source: src, Trust: trust, Revision: newV, Hash: hash}
		l.version++
		return UpdateOutcome{Applied: true, IsNew: false, Changed: changed, Revision: newV}
	}

	hash := hashEntity(e)
	l.entities[e.ID] = &entry{
		entity: e,
		stamp:  EntityStamp{AsOf: asOf, Source: src, Trust: trust, Revision: 1, Hash: hash},
	}
	if set, ok := l.byType[e.Type]; ok {
		set[e.ID] = struct{}{}
	} else {
		l.byType[e.Type] = map[string]struct{}{e.ID: {}}
	}
	l.version++
	return UpdateOutcome{Applied: true, IsNew: true, Changed: true, Revision: 1}
}

// Delete remove uma entidade do cache.
func (l *Lister) Delete(id string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	ex, ok := l.entities[id]
	if !ok {
		return false
	}
	delete(l.entities, id)
	if set, ok := l.byType[ex.entity.Type]; ok {
		delete(set, id)
		if len(set) == 0 {
			delete(l.byType, ex.entity.Type)
		}
	}
	l.version++
	return true
}

// Get devolve uma entidade pelo ID (cópia de leitura).
func (l *Lister) Get(id string) (StampedEntity, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	ex, ok := l.entities[id]
	if !ok {
		return StampedEntity{}, false
	}
	return StampedEntity{Entity: ex.entity, Stamp: ex.stamp}, true
}

// List devolve um snapshot ordenado por ID (determinístico).
func (l *Lister) List() []StampedEntity {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]StampedEntity, 0, len(l.entities))
	for _, ex := range l.entities {
		out = append(out, StampedEntity{Entity: ex.entity, Stamp: ex.stamp})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Entity.ID < out[j].Entity.ID })
	return out
}

// ListByType devolve um snapshot de um tipo, ordenado por ID (determinístico).
func (l *Lister) ListByType(t EntityType) []StampedEntity {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]StampedEntity, 0)
	for _, ex := range l.entities {
		if ex.entity.Type == t {
			out = append(out, StampedEntity{Entity: ex.entity, Stamp: ex.stamp})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Entity.ID < out[j].Entity.ID })
	return out
}

// Count devolve o número de entidades no cache.
func (l *Lister) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entities)
}

// Version devolve a resourceVersion corrente do cache.
func (l *Lister) Version() uint64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.version
}

// LastSync devolve o AsOf do último resync (zero se nunca resyncou).
func (l *Lister) LastSync() time.Time {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.lastSync
}

// Resync reconcilia o cache contra um snapshot autoritativo (auto-cura de eventos
// perdidos — k8s resync). Entidades no snapshot são upsertadas (a fonte manda);
// entidades no cache mas AUSENTES no snapshot são REMOVIDAS (drop perdido,
// curado). A divergência é exposta em diff (I2), nunca "consertada" em silêncio.
func (l *Lister) Resync(snapshot []WorldEntity, src ObservationSource, trust TrustState, asOf time.Time) ResyncDiff {
	l.mu.Lock()
	defer l.mu.Unlock()

	seen := make(map[string]struct{}, len(snapshot))
	diff := ResyncDiff{}
	for _, e := range snapshot {
		seen[e.ID] = struct{}{}
		if ex, ok := l.entities[e.ID]; ok {
			hash := hashEntity(e)
			changed := hash != ex.stamp.Hash
			newV := ex.stamp.Revision + 1
			ex.entity = e
			ex.stamp = EntityStamp{AsOf: asOf, Source: src, Trust: trust, Revision: newV, Hash: hash}
			if changed {
				diff.Updated = append(diff.Updated, e.ID)
			}
		} else {
			hash := hashEntity(e)
			l.entities[e.ID] = &entry{
				entity: e,
				stamp:  EntityStamp{AsOf: asOf, Source: src, Trust: trust, Revision: 1, Hash: hash},
			}
			if set, ok := l.byType[e.Type]; ok {
				set[e.ID] = struct{}{}
			} else {
				l.byType[e.Type] = map[string]struct{}{e.ID: {}}
			}
			diff.Added = append(diff.Added, e.ID)
		}
	}

	// Remover entidades que a fonte já não conhece (drop perdido, auto-curado).
	for id, ex := range l.entities {
		if _, ok := seen[id]; !ok {
			delete(l.entities, id)
			if set, ok := l.byType[ex.entity.Type]; ok {
				delete(set, id)
				if len(set) == 0 {
					delete(l.byType, ex.entity.Type)
				}
			}
			diff.Removed = append(diff.Removed, id)
		}
	}

	sort.Strings(diff.Added)
	sort.Strings(diff.Updated)
	sort.Strings(diff.Removed)
	l.version++
	diff.Version = l.version
	l.lastSync = asOf
	return diff
}

// ResyncIfDue faz o resync se o período decorreu (lastSync vazio ou
// now-lastSync >= resync). Retorna false + diff vazio se não está na hora. O
// snapshot() é chamado FORA do lock (a fonte não trava o lister). asOf é `now`.
func (l *Lister) ResyncIfDue(now time.Time, src ObservationSource, trust TrustState, snapshot func() []WorldEntity) (bool, ResyncDiff) {
	l.mu.RLock()
	due := l.resync > 0 && (l.lastSync.IsZero() || now.Sub(l.lastSync) >= l.resync)
	l.mu.RUnlock()
	if !due {
		return false, ResyncDiff{}
	}
	return true, l.Resync(snapshot(), src, trust, now)
}

// Stale devolve entidades cujo AsOf já excedeu maxAge — candidatas a degradação
// epistêmica (I4). O chamador decide como degradar (ex.: stale → inferred).
func (l *Lister) Stale(now time.Time, maxAge time.Duration) []StampedEntity {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]StampedEntity, 0)
	for _, ex := range l.entities {
		if !ex.stamp.AsOf.IsZero() && now.Sub(ex.stamp.AsOf) > maxAge {
			out = append(out, StampedEntity{Entity: ex.entity, Stamp: ex.stamp})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Entity.ID < out[j].Entity.ID })
	return out
}
