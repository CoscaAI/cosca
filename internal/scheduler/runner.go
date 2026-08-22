// Runner do cron — executa os jobs habilitados do Don.
//
// O daemon (`cosca cron daemon`) é a ÚNICA porta de execução automática:
// nada roda sozinho. Jobs são executados via `sh -c <command>` (os/exec) com
// timeout fixo de 5 minutos. O tick roda a cada 30s e processa os jobs
// habilitados cujo NextRun <= agora. Erros NUNCA derrubam o runner — são
// logados e o loop continua.
package scheduler

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/safe"
	"github.com/rs/zerolog"
)

// Constantes de operação do runner.
const (
	DefaultInterval = 30 * time.Second // tick do daemon
	DefaultTimeout  = 5 * time.Minute  // timeout por execução (sh -c)
)

// Runner executa os jobs do cron em um loop de 30s.
type Runner struct {
	store    *Store
	logger   zerolog.Logger
	interval time.Duration
	timeout  time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewRunner cria um runner com tick de DefaultInterval (30s) e timeout de
// DefaultTimeout (5m). Começa parado — Start(ctx) dispara o loop.
func NewRunner(store *Store, logger zerolog.Logger) *Runner {
	return &Runner{
		store:    store,
		logger:   logger,
		interval: DefaultInterval,
		timeout:  DefaultTimeout,
	}
}

// Start dispara o loop do runner em background. O primeiro tick roda
// imediatamente; depois a cada interval. A execução cessa quando ctx é
// cancelado ou Stop() é chamado.
func (r *Runner) Start(ctx context.Context) {
	r.ctx, r.cancel = context.WithCancel(ctx)
	r.wg.Add(1)
	go r.loop()
}

// Stop cancela o loop e aguarda sua saída. Execuções em andamento são
// canceladas via o contexto do runner.
func (r *Runner) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
}

// loop é o ticker de 30s do daemon.
func (r *Runner) loop() {
	defer r.wg.Done()
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	r.tick()
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.tick()
		}
	}
}

// tick processa os jobs habilitados: executa os vencidos (NextRun <= now) e
// recalcula os que nunca tiveram NextRun. Erros são logados e o runner segue.
func (r *Runner) tick() {
	jobs, err := r.store.List()
	if err != nil {
		r.logger.Error().Err(err).Msg("cron: falha ao listar jobs")
		return
	}
	now := time.Now()
	for i := range jobs {
		j := &jobs[i]
		if !j.Enabled {
			continue
		}
		if j.NextRun.IsZero() {
			// Nunca calculado (ex.: job adicionado sem NextRun) — agenda a
			// primeira ocorrência sem executar ainda.
			next, err := j.Schedule.NextAfter(now)
			if err != nil {
				r.logger.Error().Err(err).Str("job", j.ID).Str("name", j.Name).
					Msg("cron: falha ao calcular próxima execução")
				_ = r.store.UpdateNextRun(j.ID, now.Add(r.interval))
				continue
			}
			_ = r.store.UpdateNextRun(j.ID, next)
			continue
		}
		if !j.NextRun.After(now) {
			r.runJob(j, now)
		}
	}
}

// runJob executa o job uma vez (sh -c), registra LastRun/Runs e recalcula a
// próxima ocorrência. Nunca propaga erro para o loop.
func (r *Runner) runJob(j *Job, now time.Time) {
	execCtx := r.ctx
	if execCtx == nil {
		// tick() chamado diretamente (testes) sem Start().
		execCtx = context.Background()
	}

	start := time.Now()
	_, err := RunCommand(execCtx, j.Command, r.timeout)
	duration := time.Since(start)
	runs := j.Runs + 1

	next, nextErr := j.Schedule.NextAfter(now)
	if nextErr != nil {
		// Cron sem ocorrência nos próximos 2 anos: backoff de 1h para não
		// re-executar a cada tick. Logado — o runner nunca cai por isso.
		next = now.Add(time.Hour)
		r.logger.Error().Err(nextErr).Str("job", j.ID).Str("name", j.Name).
			Msg("cron: falha ao calcular próxima execução após rodar")
	}

	if err != nil {
		r.logger.Error().Err(err).Str("job", j.ID).Str("name", j.Name).
			Dur("duration", duration).Int("runs", runs).
			Msg("cron: job falhou")
	} else {
		r.logger.Info().Str("job", j.ID).Str("name", j.Name).
			Dur("duration", duration).Int("runs", runs).
			Msg("cron: job executado")
	}

	if err := r.store.RecordRun(j.ID, now, next, runs); err != nil {
		r.logger.Error().Err(err).Str("job", j.ID).
			Msg("cron: falha ao registrar execução")
	}
}

// RunCommand executa um comando via `sh -c` com timeout. Devolve a saída
// combinada (stdout+stderr) e o erro (se o processo falhou ou expirou).
func RunCommand(ctx context.Context, command string, timeout time.Duration) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd, tmpFile, err := safe.SafeShellExec(ctx, command)
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile)
	out, err2 := cmd.CombinedOutput()
	return string(out), err2
}
