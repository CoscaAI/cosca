package cache

import (
	"sync"
	"testing"
)

// ── OMEGA NÍVEL 5-6: PRÉ-REGISTRO + EXPERIMENTO DISCRIMINATIVO ─────────
//
// H1: contadores int64 (memHits/sqlHits/fsHits/misses) incrementados sem
//     atomic e lidos sem lock em Stats() = data race.
// H2: locking consistente → sem race (refutação da minha exploração).
// H3: teste mal construído → falso "limpo".
//
// PREDIÇÃO: `go test -race` com Get/Set/Stats concorrentes DETECTA data race
// nos contadores.
// FALSIFIER: se -race passar limpo com acesso concorrente real → H1 refutada.

func TestOmega_ConcurrentCounterRace(t *testing.T) {
	c, err := New(Config{
		MemoryMaxEntries: 1000,
		EnabledLevels:    []Level{LevelMemory},
	})
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	// Escritores: Get (incrementa hits/misses) e Set
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 500; j++ {
				key := "k" + string(rune('a'+n%4))
				_, _ = c.Get(key)
				_ = c.Set(key, j, 1000)
			}
		}(i)
	}
	// Leitor: Stats (lê contadores sem lock)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 500; j++ {
				_ = c.Stats()
			}
		}()
	}
	wg.Wait()

	// Se chegou aqui sem race reportado, H1 foi refutada (ou H3: teste fraco)
	t.Log("sem data race reportado nesta execução")
}
