package nodegraph

import (
	"context"
	"fmt"
	"sync"
)

// Executor executa um nó do grafo. Recebe o resultado de cada input (mapa
// inputID → valor) e devolve o valor de saída do nó.
//
// O Executor é a ponte entre o grafo (dados puros) e as engines reais
// (tasks/models/gpu/media). Um executor pode consultar o Model Registry para
// resolver o modelo da task, o Scheduler para decidir onde roda, etc.
type Executor interface {
	// Run executa o nó. input é map[inputNodeID]any (saída dos inputs).
	Run(ctx context.Context, node *Node, input map[string]any) (any, error)
}

// ExecutorFunc adapta uma função a Executor.
type ExecutorFunc func(ctx context.Context, node *Node, input map[string]any) (any, error)

// Run implementa Executor.
func (f ExecutorFunc) Run(ctx context.Context, node *Node, input map[string]any) (any, error) {
	return f(ctx, node, input)
}

// Cache armazena resultados por assinatura de nó (§23).
//
// A chave é a assinatura recursiva (type + params + assinaturas dos inputs):
// quando QUALQUER ancestral muda, a assinatura do nó muda e o cache devolve
// miss — nunca resultado obsoleto (anti-padrão A3 do Blueprint).
type Cache struct {
	mu sync.RWMutex
	m  map[string]any
}

// NewCache cria um cache vazio.
func NewCache() *Cache { return &Cache{m: make(map[string]any)} }

// Get devolve o resultado cacheado para a assinatura.
func (c *Cache) Get(sig string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.m[sig]
	return v, ok
}

// Put armazena o resultado para a assinatura.
func (c *Cache) Put(sig string, v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[sig] = v
}

// Len devolve o número de entradas no cache.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.m)
}

// Stats descreve o resultado de uma execução.
type Stats struct {
	// Executed é o número de nós efetivamente executados (cache miss).
	Executed int
	// CachedHits é o número de nós servidos do cache (cache hit).
	CachedHits int
	// Results é o resultado de cada nó (nodeID → output).
	Results map[string]any
}

// RunOptions configura a execução.
type RunOptions struct {
	// Cache opcional — quando nil, todos os nós executam.
	Cache *Cache
}

// Run executa o grafo em ordem topológica, por demanda, com cache por
// assinatura (§21/§22/§23). Cada nó recebe as saídas dos seus inputs e o
// Executor produz a saída.
func (g *Graph) Run(ctx context.Context, exec Executor, opts *RunOptions) (*Stats, error) {
	if exec == nil {
		return nil, fmt.Errorf("executor is required")
	}
	order, err := g.TopoOrder()
	if err != nil {
		return nil, err
	}

	stats := &Stats{Results: make(map[string]any, len(order))}
	var cache *Cache
	if opts != nil {
		cache = opts.Cache
	}

	for _, node := range order {
		// Monta o input do nó a partir das saídas dos inputs.
		input := make(map[string]any, len(node.Inputs))
		for _, in := range node.Inputs {
			v, ok := stats.Results[in]
			if !ok {
				return nil, fmt.Errorf("input %q of node %q has no result", in, node.ID)
			}
			input[in] = v
		}

		// Cache por assinatura: só executa o que mudou (§23).
		if cache != nil {
			sig, sigErr := g.Signature(node.ID)
			if sigErr != nil {
				return nil, sigErr
			}
			if cached, ok := cache.Get(sig); ok {
				stats.Results[node.ID] = cached
				stats.CachedHits++
				continue
			}
			out, runErr := exec.Run(ctx, node, input)
			if runErr != nil {
				return nil, fmt.Errorf("node %q (%s): %w", node.ID, node.Type, runErr)
			}
			cache.Put(sig, out)
			stats.Results[node.ID] = out
			stats.Executed++
			continue
		}

		out, runErr := exec.Run(ctx, node, input)
		if runErr != nil {
			return nil, fmt.Errorf("node %q (%s): %w", node.ID, node.Type, runErr)
		}
		stats.Results[node.ID] = out
		stats.Executed++
	}
	return stats, nil
}
