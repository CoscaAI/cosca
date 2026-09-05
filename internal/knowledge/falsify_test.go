package knowledge

import (
	"context"
	"testing"

	"github.com/CoscaAI/cosca/internal/search"
)

// ── LOOP V5: FALSIFICAÇÃO — domínio engines ────────────────────────────
//
// O knowledge.Engine acessa e.db/e.vecStore diretamente. Com zero-value,
// e.db nil → possível panic. Previsão: hipótese SOBREVIVE neste domínio.

func scanEnginePanic(t *testing.T, name string, fn func() error) (panicked bool, err error) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			panicked = true
		}
	}()
	return false, fn()
}

func TestFalsify_KnowledgeEngineZeroValue(t *testing.T) {
	e := &Engine{} // db/vecStore nil
	ctx := context.Background()

	panicked, _ := scanEnginePanic(t, "Engine.Search", func() error {
		_, err := e.Search(ctx, search.SearchParams{Query: "x"})
		return err
	})
	if panicked {
		t.Log("CONFIRMAÇÃO: Engine.Search PANIC com zero-value — hipótese sobrevive no domínio engines")
	} else {
		t.Log("CONTRAEXEMPLO: Engine.Search não panicou com zero-value")
	}
}
