// Package queue abstrai a fila de jobs do RIZOMAI (ADR-003).
//
// Duas implementações da interface Jobs:
//   - RiverQueue    (Fase 3): River durável sobre Postgres, transacional com o
//     domínio, retry/backoff por job, fan-out paralelo por target.
//   - SimulatedQueue (fallback): stub que loga e marca published (demo sem
//     credenciais/banco de fila) — modo RIZOMAI_QUEUE=simulated.
//
// Jobs (payloads) e workers vivem em river.go.
package queue

import (
	"context"
	"log"
	"time"

	"github.com/rizomai/rizomai/internal/domain"
	"github.com/rizomai/rizomai/internal/store"
)

// Jobs é a interface da fila usada pelos handlers.
type Jobs interface {
	// PublishPost enfileira o fan-out de publicação de um post (1 job por target).
	PublishPost(ctx context.Context, post *domain.Post) error
}

// SimulatedQueue é o stub de fallback: publica "sincronamente" cada target não
// agendado, logando como se o conector existisse, e deriva o status agregado.
type SimulatedQueue struct {
	Store *store.Store
	Log   *log.Logger
}

// PublishPost simula o fan-out com isolamento por target (ADR-007 §1.1).
func (q *SimulatedQueue) PublishPost(ctx context.Context, post *domain.Post) error {
	logger := q.Log
	if logger == nil {
		logger = log.Default()
	}

	for i := range post.Platforms {
		t := &post.Platforms[i]

		if post.ScheduledFor != nil && post.ScheduledFor.After(time.Now()) {
			continue // agendado: job real dispararia no due time
		}
		if t.Status != domain.TargetStatusPending {
			continue
		}

		logger.Printf("job publish.target (simulado): publicando em %s (target %s, post %s)", t.Platform, t.ID, post.ID)

		simURL := "https://" + string(t.Platform) + ".social/" + t.ID
		if err := q.Store.MarkTargetPublished(ctx, t.ID, post.ID, simURL, "sim_"+t.ID); err != nil {
			logger.Printf("job publish.target (simulado): erro ao simular publicação de %s: %v", t.ID, err)
			continue
		}
		t.Status = domain.TargetStatusPublished
	}

	statuses := make([]domain.TargetStatus, 0, len(post.Platforms))
	for _, t := range post.Platforms {
		statuses = append(statuses, t.Status)
	}
	derived := domain.DerivePostStatus(statuses)
	if post.Status != derived {
		post.Status = derived
		return q.Store.UpdatePostStatus(ctx, post.ID, derived)
	}
	return nil
}
