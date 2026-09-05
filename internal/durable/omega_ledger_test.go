package durable

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/CoscaAI/cosca/internal/sqlite"
)

// ── OMEGA NÍVEL 16+20: TRANSFERÊNCIA CEGA + ATAQUE AO APRENDIZADO ──────
//
// Heurística alvo (candidata a superajuste): "contadores/estado sem proteção
// = race" (derivada de UM caso — cache #40).
// PREVISÃO: durable/ledger tem race sob acesso concorrente real.
// HIPÓTESE ADVERSÁRIA: ledger usa locking disciplinado (writeMu no sqlite.DB)
// → NÃO há race → H1-concorrência é superajustada a um caso → DEMOTED.

func TestOmega_LedgerConcurrent(t *testing.T) {
	db, err := sqlite.Open(sqlite.DefaultConfig(filepath.Join(t.TempDir(), "ledger.db")))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	l, err := NewLedger(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// Cria um run
	run, err := l.Begin(ctx, BeginInput{
		RunID:       "run-omega",
		WorkflowRef: "wf-omega",
		InputHash:   Hash([]byte("input")),
	})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	run, err = l.Claim(ctx, run.RunID, "worker-omega", run.Fencing)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	fence := Fencing{Generation: run.Generation, Token: run.Fencing.Token}

	// Concorrência real: vários workers completam steps em paralelo
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("step-%d", n)
			err := l.StartStep(ctx, "run-omega", key, fence)
			if err != nil {
				t.Errorf("start %s: %v", key, err)
				return
			}
			if err := l.CompleteStep(ctx, "run-omega", key, fence, "ref-"+key, Hash([]byte(key))); err != nil {
				t.Errorf("complete %s: %v", key, err)
			}
		}(i)
	}
	wg.Wait()

	t.Log("ledger concorrente executado — se -race passou, H1-concorrência foi refutada aqui")
}
