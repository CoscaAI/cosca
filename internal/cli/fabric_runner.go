package cli

import (
	"context"
	"time"

	"github.com/CoscaAI/cosca/internal/compute"
	"github.com/CoscaAI/cosca/internal/pipeline"
)

// fabricRunner é um pipeline.Runner que, quando um compute.Fabric está
// disponível, submete cada execução ao worker pool ("agent") do fabric.
// Isso conecta o caminho durável multi-step (workflow run / pipeline run)
// ao compute fabric — o enfileiramento observável que o Teste 2 do Google
// pede para provar. Sem fabric, degrada limpo para a chamada direta.
//
// ADITIVO e NÃO-invasivo: o StepRunner existente não muda; ele recebe um
// runner que, opcionalmente, vai pelo fabric. Backward-compatible.
type fabricRunner struct {
	inner  pipeline.Runner
	fabric *compute.Fabric
}

// newFabricRunner cria um wrapper. Se fabric for nil, devolve o runner puro
// (comportamento histórico — a degradação é limpa, nunca quebra).
func newFabricRunner(inner pipeline.Runner, fab *compute.Fabric) pipeline.Runner {
	if fab == nil {
		return inner
	}
	return &fabricRunner{inner: inner, fabric: fab}
}

// Run implementa pipeline.Runner. Submete o passo ao pool "agent" do fabric,
// aguardando o resultado com o timeout do task. O fabric aplica circuit
// breaker, rate limiter, backpressure e enfileira no worker pool — exatamente
// o que o teste precisa observar. Em erro de fabric (ex.: pool desconhecido),
// degrada para a chamada direta ao inner runner (não perde a execução).
func (f *fabricRunner) Run(ctx context.Context, req pipeline.RunRequest) (*pipeline.RunResult, error) {
	if f.fabric == nil {
		return f.inner.Run(ctx, req)
	}
	timeout := time.Duration(0)
	if req.Options.MaxTurns > 0 {
		timeout = 300 * time.Second
	}
	result, err := f.fabric.Submit(ctx, "agent", compute.Task{
		ID:      req.Agent + "::" + req.Prompt[:min(len(req.Prompt), 24)],
		Weight:  2, // médio: um passo de workflow
		Timeout: timeout,
		Fn: func(c context.Context) (interface{}, error) {
			return f.inner.Run(c, req)
		},
	})
	if err != nil {
		// Degrada limpo: o fabric não pode bloquear a execução do passo.
		return f.inner.Run(ctx, req)
	}
	if result.Err != nil {
		return nil, result.Err
	}
	if r, ok := result.Value.(*pipeline.RunResult); ok {
		return r, nil
	}
	return f.inner.Run(ctx, req)
}

// RunStream implementa pipeline.Runner (delega ao inner; o fabric não
// gerencia streaming — mantém o comportamento original).
func (f *fabricRunner) RunStream(ctx context.Context, req pipeline.RunRequest) (<-chan pipeline.RunEvent, error) {
	return f.inner.RunStream(ctx, req)
}

// min é um helper local para evitar import de slices (Go < 1.21).
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
