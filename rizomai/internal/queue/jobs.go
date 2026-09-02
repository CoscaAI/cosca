// Package queue abstrai a fila de jobs do RIZOMAI (ADR-003).
//
// Fase 2: implementação é um STUB SIMULADO — loga o "publish" por target e
// marca como published, derivando o status agregado do post (ADR-007).
//
// Fase 3: substituir por River sobre Postgres — job TRANSACIONAL com o domínio
// (criar post agendado = mesma transação que enfileira o job), retry/backoff
// por job e fan-out paralelo com goroutines.
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
	// PublishPost enfileira/executa o fan-out de publicação de um post.
	PublishPost(ctx context.Context, post *domain.Post) error
}

// SimulatedQueue é o stub de Fase 2. Publica "sincronamente" cada target não
// agendado, logando como se o conector existisse, e deriva o status agregado.
type SimulatedQueue struct {
	Store *store.Store
	Log   *log.Logger
}

// PublishPost simula o fan-out com isolamento por target (ADR-007 §1.1):
// falha em X não bloqueia Y.
func (q *SimulatedQueue) PublishPost(ctx context.Context, post *domain.Post) error {
	logger := q.Log
	if logger == nil {
		logger = log.Default()
	}

	for i := range post.Platforms {
		t := &post.Platforms[i]

		// Targets agendados para o futuro ficam scheduled — o job real
		// (Fase 3/River) dispararia no due time.
		if post.ScheduledFor != nil && post.ScheduledFor.After(time.Now()) {
			continue
		}
		if t.Status != domain.TargetStatusPending {
			continue
		}

		// TODO(Fase 3): conector real em internal/platform/{x,linkedin,telegram} —
		//   publicar na rede → RecordAttempt (append-only) → status real.
		logger.Printf("job publish.target: publicando em %s (target %s, post %s) [Fase 3: conector real]", t.Platform, t.ID, post.ID)

		simURL := "https://" + string(t.Platform) + ".social/" + t.ID
		if err := q.Store.MarkTargetPublished(ctx, t.ID, post.ID, simURL, "sim_"+t.ID); err != nil {
			logger.Printf("job publish.target: erro ao simular publicação de %s: %v", t.ID, err)
			continue
		}
		t.Status = domain.TargetStatusPublished
	}

	// Derivar o status agregado do post a partir dos targets (ADR-007 §1).
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
