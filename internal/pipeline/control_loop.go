// control_loop.go — Control Loop contínuo (Fase 1 do ADR-023; mineração
// kubernetes/sample-controller).
//
// O Cosca JÁ tem as peças (internal/pipeline/workqueue.go = cópia do client-go;
// internal/pipeline/reconciler.go = desired vs atual → drift). O que faltava era
// SOLDAR: um loop contínuo `workqueue → worker → Get → reconcile(processNext) →
// Done → AddRateLimited (falha) / Forget (sucesso)`, com backoff per-item e DROP
// após maxRetries (fail-closed — nunca re-spin infinito de um trabalho
// permanentemente falho).
//
// Determinístico (I1), fail-closed (I2). O `reconcile` é a função do usuário (que
// pode envolver o Reconciler.Reconcile), e roda NO MÁXIMO uma vez por key em
// paralelo (a workqueue garante exclusão por key).
package pipeline

import (
	"context"
	"sync"
	"time"
)

// ReconcileFunc aplica o syncHandler de uma key. Retorna error para requeue.
type ReconcileFunc func(ctx context.Context, key string) error

// ControlLoop é o loop de reconciliação contínuo (workqueue + workers).
type ControlLoop struct {
	queue     *Workqueue
	reconcile ReconcileFunc
	workers   int
	maxRetries int
	onDrop    func(key string, err error)
}

// NewControlLoop cria um loop com backoff exponencial per-item (5ms→1000s,
// como o k8s) e o contrato de erro: falha → AddRateLimited; sucesso → Forget;
// após maxRetries → drop (fail-closed, observável via OnDrop).
func NewControlLoop(reconcile ReconcileFunc, workers, maxRetries int) *ControlLoop {
	if workers <= 0 {
		workers = 1
	}
	if maxRetries <= 0 {
		maxRetries = 15 // default k8s
	}
	limiter := NewItemExponentialFailureRateLimiter(5*time.Millisecond, 1000*time.Second)
	return &ControlLoop{
		queue:      NewWorkqueue(limiter),
		reconcile:  reconcile,
		workers:    workers,
		maxRetries: maxRetries,
	}
}

// OnDrop registra um observador para quando um item é dropado (após maxRetries).
func (c *ControlLoop) OnDrop(fn func(key string, err error)) { c.onDrop = fn }

// Enqueue adiciona uma key para reconciliação (idempotente/dedup por key).
func (c *ControlLoop) Enqueue(key string) { c.queue.Add(key) }

// Run bloqueia executando os workers até ShutDown. Cada worker: Get → reconcile
// → Done → (AddRateLimited | Forget). A workqueue garante exclusão por key.
func (c *ControlLoop) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < c.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.worker(ctx)
		}()
	}
	wg.Wait()
}

func (c *ControlLoop) worker(ctx context.Context) {
	for {
		key, shutdown := c.queue.Get()
		if shutdown {
			return
		}
		err := c.reconcile(ctx, key)
		c.queue.Done(key)
		if err != nil {
			c.handleErr(key, err)
		} else {
			c.queue.Forget(key)
		}
	}
}

// handleErr: o contrato k8s. Falha → AddRateLimited (backoff exp.); após
// maxRetries → Forget + drop observável (fail-closed: não re-spin infinito).
func (c *ControlLoop) handleErr(key string, err error) {
	if c.queue.NumRequeues(key) < c.maxRetries {
		c.queue.AddRateLimited(key)
		return
	}
	c.queue.Forget(key)
	if c.onDrop != nil {
		c.onDrop(key, err)
	}
}

// ShutDown encerra o loop (workers retornam após o item em voo).
func (c *ControlLoop) ShutDown() { c.queue.ShutDown() }

// Len devolve quantas keys estão enfileiradas.
func (c *ControlLoop) Len() int { return c.queue.Len() }
