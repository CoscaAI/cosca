// Implementação River (ADR-003): cliente + jobs + workers.
//
// River roda na MESMA instância Postgres (schema river), transacional com o
// domínio. Dois jobs:
//   - publish.target: publica UM target via conector (fan-out paralelo)
//   - webhook.deliver: entrega evento assinado HMAC-SHA256 com retry
package queue

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/rizomai/rizomai/internal/domain"
	"github.com/rizomai/rizomai/internal/oauth"
	"github.com/rizomai/rizomai/internal/platform"
	"github.com/rizomai/rizomai/internal/store"
)

// Jobs River.

// PublishTargetJob publica UM target (post + conta + plataforma).
type PublishTargetJob struct {
	TargetID string `json:"targetId"`
	PostID   string `json:"postId"`
}

// Kind identifica o job.
func (PublishTargetJob) Kind() string { return "publish.target" }

// DeliverWebhookJob entrega um evento assinado (ADR-009 §1.4: MESMO event id
// em todos os retries — dedup do consumidor).
type DeliverWebhookJob struct {
	WebhookID string
	EventID   string
	EventType string
	Data      map[string]any
	DeliveryID string
}

// Kind identifica o job.
func (DeliverWebhookJob) Kind() string { return "webhook.deliver" }

// RiverQueue implementa Jobs sobre River.
type RiverQueue struct {
	client *river.Client[pgx.Tx]
	logger *log.Logger
}

// NewRiverQueue envolve um cliente River já iniciado na interface Jobs.
func NewRiverQueue(client *river.Client[pgx.Tx], logger *log.Logger) *RiverQueue {
	return &RiverQueue{client: client, logger: logger}
}

// Stop encerra o cliente (graceful shutdown).
func (q *RiverQueue) Stop(ctx context.Context) error { return q.client.Stop(ctx) }

// RiverOptions carrega as dependências compartilhadas pelos workers.
type RiverOptions struct {
	Store    *store.Store
	Registry *platform.Registry
	TokenKey []byte // chave AES-256-GCM (RIZOMAI_TOKEN_KEY)
	Logger   *log.Logger

	// Seleção de workers (ADR-003: processos consumidores por responsabilidade).
	// false em ambos = registra tudo (padrão do gateway).
	PublishWorker bool
	WebhookWorker bool
}

// StartRiver aplica o schema do River e sobe o cliente com os workers.
// Reutilizável pelo gateway E pelos workers cmds (ADR-003: processos separados).
func StartRiver(ctx context.Context, pool *pgxpool.Pool, opts RiverOptions) (*river.Client[pgx.Tx], error) {
	logger := opts.Logger
	if logger == nil {
		logger = log.Default()
	}

	// Schema `river` (migrations embarcadas no pacote rivermigrate).
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), &rivermigrate.Config{})
	if err != nil {
		return nil, fmt.Errorf("rivermigrate.New: %w", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return nil, fmt.Errorf("rivermigrate.Migrate: %w", err)
	}
	logger.Print("schema river migrado")

	withPublish := opts.PublishWorker || (!opts.PublishWorker && !opts.WebhookWorker)
	withWebhook := opts.WebhookWorker || (!opts.PublishWorker && !opts.WebhookWorker)

	queues := map[string]river.QueueConfig{}
	if withPublish {
		queues["publish"] = river.QueueConfig{MaxWorkers: 10} // fan-out paralelo por target (ADR-007 §1.1)
	}
	if withWebhook {
		queues["webhook"] = river.QueueConfig{MaxWorkers: 4}
	}

	workers := river.NewWorkers()
	if withPublish {
		river.AddWorker(workers, &PublishTargetWorker{Store: opts.Store, Registry: opts.Registry, TokenKey: opts.TokenKey, Log: logger})
	}
	if withWebhook {
		river.AddWorker(workers, &DeliverWebhookWorker{Store: opts.Store, TokenKey: opts.TokenKey, Log: logger})
	}

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Queues: queues, Workers: workers})
	if err != nil {
		return nil, fmt.Errorf("river.NewClient: %w", err)
	}

	if err := client.Start(ctx); err != nil {
		return nil, fmt.Errorf("river.Start: %w", err)
	}
	logger.Print("river client iniciado")
	return client, nil
}

// PublishPost enfileira 1 job por target (agendamento respeita scheduledFor).
func (q *RiverQueue) PublishPost(ctx context.Context, post *domain.Post) error {
	for _, t := range post.Platforms {
		opts := &river.InsertOpts{Queue: "publish", MaxAttempts: 5}
		if post.ScheduledFor != nil {
			opts.ScheduledAt = *post.ScheduledFor
		}
		if _, err := q.client.Insert(ctx, &PublishTargetJob{TargetID: t.ID, PostID: post.ID}, opts); err != nil {
			return fmt.Errorf("river insert publish.target (%s): %w", t.ID, err)
		}
	}
	return nil
}

// EnqueueWebhook enfileira a entrega de um evento (usado após publicar/falhar).
func (q *RiverQueue) EnqueueWebhook(ctx context.Context, webhookID, eventType string, data map[string]any) error {
	eventID, err := domain.NewEventID()
	if err != nil {
		return err
	}
	deliveryID, err := domain.NewDeliveryID()
	if err != nil {
		return err
	}
	_, err = q.client.Insert(ctx, &DeliverWebhookJob{
		WebhookID:  webhookID,
		EventID:    eventID,
		EventType:  eventType,
		Data:       data,
		DeliveryID: deliveryID,
	}, &river.InsertOpts{Queue: "webhook", MaxAttempts: 12})
	return err
}

// --- Workers -----------------------------------------------------------------

// PublishTargetWorker executa publish.target: despacha para o conector,
// registra PublishAttempt (append-only), atualiza o target e deriva o agregado.
type PublishTargetWorker struct {
	river.WorkerDefaults[PublishTargetJob]
	Store    *store.Store
	Registry *platform.Registry
	TokenKey []byte
	Log      *log.Logger
}

// Work implementa river.JobWorker.
func (w *PublishTargetWorker) Work(ctx context.Context, job *river.Job[PublishTargetJob]) error {
	logger := w.Log
	if logger == nil {
		logger = log.Default()
	}

	pc, err := w.Store.GetPublishContext(ctx, job.Args.TargetID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			logger.Printf("publish.target %s: target não encontrado — abortando sem retry", job.Args.TargetID)
			return nil
		}
		return err // erro transiente do banco → River retry
	}

	pub, err := w.Registry.Publisher(pc.Target.Platform)
	if err != nil {
		w.recordFailed(ctx, pc, &platform.Error{Platform: string(pc.Target.Platform), Code: "unsupported", Message: err.Error()})
		return nil
	}

	creds := platform.Credentials{
		AccessToken:  string(mustDecrypt(pc.Account.EncryptedToken, w.TokenKey, logger)),
		RefreshToken: string(mustDecrypt(pc.Account.RefreshTokenEncrypted, w.TokenKey, logger)),
		ExternalID:   pc.Account.ExternalIdentifier,
		ExpiresAt:    pc.Account.ExpiresAt,
	}

	started := time.Now().UTC()
	result, perr := pub.Publish(ctx, pc.Content, &pc.Target, creds)

	attempt := &domain.PublishAttempt{
		ID:        newAttemptID(),
		TargetID:  pc.Target.ID,
		Attempt:   int(job.Attempt),
		StartedAt: started,
		Outcome:   domain.OutcomeSuccess,
	}
	if perr != nil {
		attempt.FinishedAt = timePtr(time.Now().UTC())
		attempt.Error = &domain.TargetError{Code: platformCode(perr), Error: perr.Error()}
		if pe, ok := perr.(*platform.Error); ok && pe.Retryable {
			attempt.Outcome = domain.OutcomeRateLimited
		} else {
			attempt.Outcome = domain.OutcomeFailed
		}
		_ = w.Store.RecordAttempt(ctx, attempt)

		logger.Printf("publish.target %s (%s): falhou [%s] %v (retryable=%v)", pc.Target.ID, pc.Target.Platform, attempt.Outcome, perr, retryable(perr))

		if err := w.Store.MarkTargetFailed(ctx, pc.Target.ID, pc.PostID, platformCode(perr), perr.Error()); err != nil {
			logger.Printf("publish.target %s: erro ao marcar failed: %v", pc.Target.ID, err)
		}

		if retryable(perr) {
			// Transitórios (429/5xx): devolve erro → River faz retry com backoff
			// (ADR-003: 5 tentativas configuradas no insert). Definitivos:
			// já marcados failed — retorna nil (sem retry).
			return perr
		}

		// Falha definitiva: deriva o agregado e notifica post.failed/partial.
		if err := w.deriveAndNotify(ctx, pc.PostID, pc.Account.ProfileID, logger); err != nil {
			logger.Printf("publish.target %s: derive/notify: %v", pc.Target.ID, err)
		}
		return nil
	}

	// Sucesso.
	attempt.FinishedAt = timePtr(time.Now().UTC())
	attempt.Outcome = domain.OutcomeSuccess
	_ = w.Store.RecordAttempt(ctx, attempt)

	if err := w.Store.MarkTargetPublished(ctx, pc.Target.ID, pc.PostID, result.PublishedURL, result.ExternalID); err != nil {
		logger.Printf("publish.target %s: erro ao marcar published: %v", pc.Target.ID, err)
	}
	logger.Printf("publish.target %s (%s): publicado → %s", pc.Target.ID, pc.Target.Platform, result.PublishedURL)

	return w.deriveAndNotify(ctx, pc.PostID, pc.Account.ProfileID, logger)
}

// deriveAndNotify recomputa o status agregado e enfileira webhooks post.*
// (ADR-009) via ClientFromContext (o client é injetado pelo River no Work).
func (w *PublishTargetWorker) deriveAndNotify(ctx context.Context, postID, profileID string, logger *log.Logger) error {
	statuses, err := w.Store.ListTargetStatuses(ctx, postID)
	if err != nil {
		return err
	}
	derived := domain.DerivePostStatus(statuses)
	if err := w.Store.UpdatePostStatus(ctx, postID, derived); err != nil {
		return err
	}
	logger.Printf("publish.target: post %s → status agregado %s", postID, derived)

	switch derived {
	case domain.PostStatusPublished, domain.PostStatusPartial, domain.PostStatusFailed:
		eventType := "post." + string(derived)
		return w.enqueueWebhooks(ctx, profileID, eventType, map[string]any{
			"postId": postID,
			"status": string(derived),
		})
	}
	return nil
}

// enqueueWebhooks enfileira a entrega dos eventos post.* para os webhooks do
// profile que assinam o evento (ADR-009 §1.7 — default: post.published/failed).
func (w *PublishTargetWorker) enqueueWebhooks(ctx context.Context, profileID, eventType string, data map[string]any) error {
	client := river.ClientFromContext[pgx.Tx](ctx) // nil fora de um Work
	if client == nil {
		return nil
	}
	whs, err := w.Store.ListWebhooksByProfile(ctx, profileID)
	if err != nil {
		return err
	}
	for _, wh := range whs {
		if !matchesEvent(wh.Events, eventType) {
			continue
		}
		eventID, _ := domain.NewEventID()
		deliveryID, _ := domain.NewDeliveryID()
		if _, err := client.Insert(ctx, &DeliverWebhookJob{
			WebhookID:  wh.ID,
			EventID:    eventID,
			EventType:  eventType,
			Data:       data,
			DeliveryID: deliveryID,
		}, &river.InsertOpts{Queue: "webhook", MaxAttempts: 12}); err != nil {
			return err
		}
		logger := w.Log
		if logger != nil {
			logger.Printf("publish.target: webhook enfileirado %s → %s", wh.ID, eventType)
		}
	}
	return nil
}

// matchesEvent verifica se o webhook assina o evento (lista vazia = todos).
func matchesEvent(events []string, eventType string) bool {
	if len(events) == 0 {
		return true
	}
	for _, e := range events {
		if e == eventType {
			return true
		}
	}
	return false
}

func (w *PublishTargetWorker) recordFailed(ctx context.Context, pc *store.PublishContext, perr error) {
	attempt := &domain.PublishAttempt{
		ID:         newAttemptID(),
		TargetID:   pc.Target.ID,
		Attempt:    1,
		StartedAt:  time.Now().UTC(),
		FinishedAt: timePtr(time.Now().UTC()),
		Outcome:    domain.OutcomeFailed,
		Error:      &domain.TargetError{Code: platformCode(perr), Error: perr.Error()},
	}
	_ = w.Store.RecordAttempt(ctx, attempt)
	_ = w.Store.MarkTargetFailed(ctx, pc.Target.ID, pc.PostID, platformCode(perr), perr.Error())
}

// DeliverWebhookWorker executa webhook.deliver: envia payload assinado
// HMAC-SHA256 com timeout de 5s e registra delivery log (ADR-009).
type DeliverWebhookWorker struct {
	river.WorkerDefaults[DeliverWebhookJob]
	Store    *store.Store
	TokenKey []byte
	Log      *log.Logger
}

// Work implementa river.JobWorker.
func (w *DeliverWebhookWorker) Work(ctx context.Context, job *river.Job[DeliverWebhookJob]) error {
	logger := w.Log
	if logger == nil {
		logger = log.Default()
	}

	wh, err := w.Store.GetWebhook(ctx, job.Args.WebhookID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return err
	}
	if !wh.IsActive {
		return nil
	}

	// Entrega com HMAC-SHA256, timeout 5s e delivery log (ADR-009).
	if err := DeliverWebhook(ctx, w.Store, wh, job.Args.EventID, job.Args.EventType, job.Args.Data, w.TokenKey, logger); err != nil {
		return err // River retry — MESMO event id (dedup do consumidor)
	}
	return nil
}

// --- helpers -----------------------------------------------------------------

func retryable(err error) bool {
	var pe *platform.Error
	return errors.As(err, &pe) && pe.Retryable
}

func platformCode(err error) string {
	var pe *platform.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	return "unknown"
}

func mustDecrypt(ct, key []byte, logger *log.Logger) []byte {
	if len(ct) == 0 {
		return nil
	}
	b, err := oauth.Decrypt(ct, key)
	if err != nil {
		logger.Printf("aviso: falha ao descriptografar token: %v", err)
		return nil
	}
	return b
}

func timePtr(t time.Time) *time.Time { return &t }

func newAttemptID() string {
	id, _ := domain.NewAttemptID()
	return id
}
