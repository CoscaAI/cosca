// Command publish-worker consome jobs de publicação (fan-out) via River
// (ADR-003/007) — binário 2 do monorepo (ADR-004).
//
// Fase 1 (scaffold): estrutura de exemplo APENAS — SEM conexão real com fila.
// O código de inicialização do River está comentado de propósito: a dependência
// github.com/riverqueue/river e o pool pgx (internal/store) entram na Fase 2,
// junto com a migração do schema `river` (migrations/).
//
// Próximos passos (Fase 2):
//   1. cliente River sobre pgx (transacional com o domínio — ADR-003)
//   2. registro do job PublishTargetJob + política de retry/backoff
//   3. fan-out paralelo por target com goroutines, isolamento de falha
//      (falha em X não bloqueia Y — ADR-001/007)
package main

import (
	"context"
	"log"
	"os"
)

// PublishTargetJob é a unidade de trabalho do fan-out: publica UM target
// (post + conta + plataforma) de forma isolada (ADR-007 §1.1).
type PublishTargetJob struct {
	PostID    string `json:"postId"`
	TargetID  string `json:"targetId"`
	AccountID string `json:"accountId"`
	Platform  string `json:"platform"`
}

// Kind identifica o tipo do job no River.
func (PublishTargetJob) Kind() string { return "publish.target" }

// TODO(Fase 2) — política de retry/backoff por job (ADR-003/007):
//
//	Transitórios (429, 500, 502, 503): 5 tentativas, backoff 5s×2^n cap 5min.
//	Definitivos (invalid_grant, duplicate, quota...): SEM retry automático —
//	erro tipado em PublishAttempt e status do target = failed.

func main() {
	// Shape final do worker (Fase 2):
	//
	//   pool, err := store.NewPool(ctx, os.Getenv("DATABASE_URL")) // ADR-002
	//   if err != nil { log.Fatal(err) }
	//
	//   rv, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
	//       Queues: map[string]river.QueueConfig{
	//           river.QueueDefault: {MaxWorkers: 10}, // fan-out paralelo
	//       },
	//   })
	//   if err != nil { log.Fatal(err) }
	//   if err := rv.Start(ctx); err != nil { log.Fatal(err) }
	//   defer rv.Stop(ctx)
	//
	// Por ora (Fase 1) o worker apenas verifica env e informa o estado.

	ctx := context.Background()
	_ = ctx

	if os.Getenv("DATABASE_URL") == "" {
		// Não é erro na Fase 1: banco/fila ainda não são obrigatórios.
		log.Printf("publish-worker: scaffold ativo — fila River chega na Fase 2")
		return
	}

	log.Printf("publish-worker: DATABASE_URL presente — inicialização real na Fase 2")
}
