// middleware.go — Middleware sobre model-call e tool-call (LangChain).
//
// Padrão minerado do LangChain (AgentMiddleware: wrap_model_call /
// wrap_tool_call): o ponto exato de interceptar a chamada ao modelo e à
// ferramenta para (a) medir custo, (b) decidir retry/fallback, (c) aplicar
// limites, (d) redigir PII — SEM mudar a lógica do agente.
//
// Diferente do langchain (que usa funções que envolvem o handler), aqui
// modelamos o middleware como uma CADEIA que envolve a execução:
//
//	caller → mw1 → mw2 → handler → mw2' → mw1' → result
//
// Cada middleware pode pré-processar (antes), decidir o handler (wrap) e
// pós-processar (depois). Read-only, sem estado global — o estado é o
// MiddlewareChain por execução.

package middleware

import (
	"context"
)

// Context é o contexto que atravessa o middleware (trace_id, capability,
// custo acumulado, orçamento). Propaga para filhos — análogo ao
// RunnableConfig do LangChain.
type Context struct {
	TraceID     string
	Capability  string
	TokensIn    int
	TokensOut   int
	Attempts    int
	BudgetExceeded bool
}

// Handler é a função que o middleware envolve (a chamada real ao modelo/tool).
type Handler func(ctx context.Context, mctx *Context) (result string, err error)

// Middleware é um passo da cadeia. O Handler dado é o PRÓXIMO handler; o
// middleware decide se chama, com que args, e pode pós-processar o resultado.
type Middleware func(ctx context.Context, mctx *Context, next Handler) (string, error)

// Chain é uma cadeia de middlewares em volta do handler terminal.
type Chain struct {
	middlewares []Middleware
}

// NewChain cria uma caide de middlewares (ordem de aplicação: primeiro é a
// camada mais externa).
func NewChain(mws ...Middleware) *Chain {
	return &Chain{middlewares: mws}
}

// Use adiciona um middleware à caide (mais externa se composta depois).
func (c *Chain) Use(m Middleware) {
	c.middlewares = append(c.middlewares, m)
}

// Run executa a caide em volta do handler terminal. Os middlewares são
// compostos em camadas: o primeiro é o mais externo (chama o próximo, que
// chama o próximo... até o terminal).
func (c *Chain) Run(ctx context.Context, mctx *Context, terminal Handler) (string, error) {
	// Constrói a caide do terminal para fora.
	handler := terminal
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		mw := c.middlewares[i]
		next := handler
		handler = makeHandler(mw, next)
	}
	return handler(ctx, mctx)
}

// makeHandler envolve `next` com o middleware `mw` — a composição em camada.
func makeHandler(mw Middleware, next Handler) Handler {
	return func(ctx context.Context, mctx *Context) (string, error) {
		return mw(ctx, mctx, next)
	}
}
