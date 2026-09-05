// sourcecache.go — Cache de fonte resiliente (single-flight + TTL + stale-on-error).
//
// PRINCÍPIO minerado (Osiris, MIT — src/lib/sourceCache.ts) reimplementado segundo
// os contratos do COSCA. Ver ADR-039. NENHUM código do repositório-fonte foi
// importado; apenas a ideia foi portada para uma primitiva Go idiomática.
//
// O que esta primitiva faz, e o `cache.Cache` (value-store) NÃO faz:
//   - single-flight: N acessos concorrentes ao MESMO cache-miss compartilham UMA
//     única chamada ao fetcher (a fonte nunca é estampada por N requests).
//   - TTL: serve da memória até expirar.
//   - stale-on-error: um refresh que FALHA serve o último dado bom (marcado
//     stale/degraded) em vez de zerar a informação.
//   - mapa com teto: limite de entradas; evicta a mais antiga; NUNCA evicta um
//     request in-flight.
//
// É uma primitiva ADITIVA. Não altera `cache.Cache`, não altera APIs públicas
// existentes e não refatora consumidores. `cache.Cache` continua sendo o
// value-store genérico; `CachedSource` é o wrapper de fonte resiliente.
package cache

import (
	"container/list"
	"context"
	"sync"
	"time"
)

// SourceFetcher carrega o valor de uma fonte a partir de uma chave. O wrapper
// garante que ela não é chamada mais de UMA vez por janela TTL (single-flight).
// Quando T é uma coleção de comprimento zero, a semântica "vazio = refresh falho"
// (ADR-039) é responsabilidade do fetcher: retornar um erro (ou um sentinel) para
// que o wrapper sirva o último dado bom, em vez de cachear vazio.
type SourceFetcher[T any] func(ctx context.Context, key string) (T, error)

// SourceResult é o resultado de carregar uma fonte resiliente.
type SourceResult[T any] struct {
	// Value é o valor corrente válido.
	Value T
	// Stale é true quando Value veio da última cópia boa, porque um refresh
	// falhou (degraded) — não é um fetch fresco.
	Stale bool
}

// SourceConfig configura o CachedSource.
type SourceConfig struct {
	// TTL é a janela durante a qual um valor serve da memória sem re-toque.
	// Default: 30 minutos.
	TTL time.Duration
	// RetryTTL é a janela CURTA para re-tentar um refresh que falhou, menor que
	// o TTL cheio, para não martelar a fonte. Default: 60 segundos.
	RetryTTL time.Duration
	// MaxEntries é o teto do mapa. Default: 500.
	MaxEntries int
}

func (c SourceConfig) withDefaults() SourceConfig {
	if c.TTL <= 0 {
		c.TTL = 30 * time.Minute
	}
	if c.RetryTTL <= 0 {
		c.RetryTTL = 60 * time.Second
	}
	if c.MaxEntries <= 0 {
		c.MaxEntries = 500
	}
	return c
}

// CachedSource envolve um fetcher com single-flight, TTL e stale-on-error.
// Concorrente é seguro (sync). A chave identifica a fonte dentro de um mesmo
// wrapper (ex.: um endpoint com várias chaves regionais).
type CachedSource[T any] struct {
	mu      sync.Mutex
	fetcher SourceFetcher[T]
	cfg     SourceConfig
	// now é o relógio (test seam). Default: time.Now.
	now     func() time.Time
	entries map[string]*sourceEntry[T]
	order   *list.List // ordem de inserção (FIFO) para evictar a mais antiga
}

// sourceEntry guarda o estado de uma chave dentro do CachedSource.
type sourceEntry[T any] struct {
	value     T
	valid     bool // há um valor utilizável (fresco ou stale)
	stale     bool // o valor atual é stale (veio de um refresh que falhou)
	expiresAt time.Time
	inflight  *sourceCall[T]
	elem      *list.Element // posição na ordem de inserção
}

// sourceCall é o "future" compartilhado do single-flight: o goroutine que
// dispara o fetch popula value/stale antes de fechar done; quem se junta espera.
type sourceCall[T any] struct {
	done  chan struct{}
	value T
	stale bool
	err   error
}

// NewSourceCache cria um CachedSource com as configurações dadas (defaults
// aplicados quando zero).
func NewSourceCache[T any](fetcher SourceFetcher[T], cfg SourceConfig) *CachedSource[T] {
	return &CachedSource[T]{
		fetcher: fetcher,
		cfg:     cfg.withDefaults(),
		now:     time.Now,
		entries: make(map[string]*sourceEntry[T]),
		order:   list.New(),
	}
}

// Load devolve o valor da fonte pela chave, aplicando single-flight + TTL +
// stale-on-error. Não bloqueia além de um fetch em andamento; N callers
// concorrentes ao mesmo miss compartilham o mesmo fetch.
func (s *CachedSource[T]) Load(ctx context.Context, key string) (SourceResult[T], error) {
	s.mu.Lock()
	now := s.now()

	e, exists := s.entries[key]
	// Junta-se a um fetch já em andamento (single-flight) — mesmo que o valor
	// tenha expirado, quem está refrescando serve o resultado a todos.
	if exists && e.inflight != nil {
		call := e.inflight
		s.mu.Unlock()
		<-call.done
		return SourceResult[T]{Value: call.value, Stale: call.stale}, call.err
	}

	// Hit fresco: serve da memória, não toca a fonte.
	if exists && e.valid && now.Before(e.expiresAt) {
		res := SourceResult[T]{Value: e.value, Stale: e.stale}
		s.mu.Unlock()
		return res, nil
	}

	// Miss ou expirado → inicia um refresh (single-flight). Guarda o estado
	// anterior (possível stale) para o fallback em caso de falha.
	var staleValue T
	var hasStale bool
	if exists && e.valid {
		staleValue = e.value
		hasStale = true
	}

	call := &sourceCall[T]{done: make(chan struct{})}
	if !exists {
		e = &sourceEntry[T]{}
		s.entries[key] = e
		e.elem = s.order.PushBack(key)
	}
	e.inflight = call
	s.evictIfNeededLocked()
	s.mu.Unlock()

	// Único goroutine que chama o fetcher para esta chave/janela.
	value, err := s.fetcher(ctx, key)

	s.mu.Lock()
	switch {
	case err != nil && hasStale:
		// Refresh falhou, mas há última cópia boa → serve stale (degraded),
		// re-tenta mais cedo que o TTL cheio.
		e.value = staleValue
		e.valid = true
		e.stale = true
		e.expiresAt = now.Add(s.cfg.RetryTTL)
		call.value = staleValue
		call.stale = true
	case err != nil:
		// Sem valor anterior → o erro permanece erro. A entrada fica inválida;
		// a próxima chamada re-tenta (não serve nada falso).
		e.valid = false
		e.stale = false
		call.err = err
	default:
		// Sucesso → valor fresco.
		e.value = value
		e.valid = true
		e.stale = false
		e.expiresAt = now.Add(s.cfg.TTL)
		call.value = value
		call.stale = false
	}
	e.inflight = nil
	close(call.done)
	s.mu.Unlock()

	return SourceResult[T]{Value: call.value, Stale: call.stale}, call.err
}

// Clear esvazia o cache e a ordem de evicção.
func (s *CachedSource[T]) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = make(map[string]*sourceEntry[T])
	s.order = list.New()
}

// evictIfNeededLocked remove as entradas mais antigas quando o mapa excede o
// teto. NUNCA evicta uma entrada com request in-flight (preserva o single-flight).
// Deve ser chamado com s.mu obtida.
func (s *CachedSource[T]) evictIfNeededLocked() {
	for len(s.entries) > s.cfg.MaxEntries {
		var toRemove *list.Element
		for el := s.order.Front(); el != nil; el = el.Next() {
			e := s.entries[el.Value.(string)]
			if e != nil && e.inflight == nil {
				toRemove = el
				break
			}
		}
		if toRemove == nil {
			// Todas as entradas estão in-flight; melhor preservar do que quebrar
			// o single-flight. O excesso é temporário e drena sozinho.
			return
		}
		delete(s.entries, toRemove.Value.(string))
		s.order.Remove(toRemove)
	}
}
