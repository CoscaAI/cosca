//
// `cosca session` — Busca determinística (zero LLM) nas conversas de sessão.
//
// As conversas vivem em .cosca/sessions/{id}.jsonl (linha 1 = meta com
// id/model/agent, depois records message com role/content/timestamp). Este
// comando indexa essas conversas num FTS5 dedicado (.cosca/session.db) e
// permite busca textual e linhagem de sessão — tudo local, determinístico e
// sem chamadas de modelo.
//
// O índice é EXPLÍCITO: nada é auto-indexado. O Don roda `cosca session
// index` quando quiser; `search` e `lineage` são somente leitura.
//
// Subcomandos:
//   index     Reindexa .cosca/sessions/*.jsonl → .cosca/session.db
//   search    Busca FTS5 nas mensagens indexadas (zero LLM)
//   lineage   Sessão + descendentes (via parent_session_id no meta)
//

package cli

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	_ "modernc.org/sqlite"

	"github.com/CoscaAI/cosca/internal/sessionindex"
	"github.com/CoscaAI/cosca/internal/sessionstatus"
)

// resolveSessionDB abre (ou cria) .cosca/session.db do projeto atual e garante
// o schema do índice. DB dedicado — nunca toca o knowledge.db. Chamado em
// toda leitura para que um search antes do primeiro index retorne zero
// ocorrências (em vez de "no such table").
func resolveSessionDB() (*sql.DB, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getwd: %w", err)
	}
	coscaDir := filepath.Join(dir, ".cosca")
	if err := os.MkdirAll(coscaDir, 0o700); err != nil {
		return nil, fmt.Errorf("create .cosca: %w", err)
	}
	db, err := sql.Open("sqlite", filepath.Join(coscaDir, "session.db"))
	if err != nil {
		return nil, fmt.Errorf("open session index: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("session pragma: %w", err)
	}
	if err := sessionindex.EnsureSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// NewSessionCommand cria a árvore de comandos `cosca session`.
func NewSessionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Busca FTS5 determinística (zero LLM) nas conversas de sessão",
		Long: `Busca FTS5 determinística (zero LLM) nas conversas de sessão.

As conversas vivem em .cosca/sessions/{id}.jsonl: linha 1 com o meta da
sessão (id, model, agent — e opcionalmente parent_session_id para linhagem),
seguida dos records message (role, content, timestamp).

Este comando mantém um índice FTS5 dedicado (.cosca/session.db). A indexação
é EXPLÍCITA (roda "cosca session index"); as buscas são somente leitura,
determinísticas e nunca chamam o LLM.

Subcomandos:
  index                 Reindexa .cosca/sessions/*.jsonl → .cosca/session.db
  search "<frase>"      Busca textual nas mensagens indexadas (zero LLM)
  lineage <id>          Sessão + descendentes (parent_session_id no meta)`,
		Example: `  cosca session index
  cosca session search "CKL"
  cosca session search --limit 20 "tatuagem"
  cosca session lineage cosca-voice`,
	}

	cmd.AddCommand(
		NewSessionIndexCommand(),
		NewSessionSearchCommand(),
		NewSessionLineageCommand(),
		NewSessionRegisterCommand(),
		NewSessionReadyCommand(),
		NewSessionStatusCommand(),
	)
	return cmd
}

// NewSessionIndexCommand cria `cosca session index`.
func NewSessionIndexCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "index",
		Short: "Reindexa as conversas de .cosca/sessions/*.jsonl no índice FTS5 (.cosca/session.db)",
		Long: `Reindexa TODAS as conversas de .cosca/sessions/*.jsonl no índice
FTS5 (.cosca/session.db). É a única operação de escrita deste comando — nada
é auto-indexado sem o Don rodar isto.

Idempotente: mensagens são sobrescritas por message_id (sessão:linha), então
rodar de novo nunca duplica. Linhas malformadas e records não-message (usage,
etc.) são ignorados.`,
		Example: `  cosca session index
  cosca session index --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getwd: %w", err)
			}

			db, err := resolveSessionDB()
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			count, err := sessionindex.IndexSessions(filepath.Join(dir, ".cosca"), db)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"indexed": count,
					"source":  filepath.Join(".cosca", "sessions", "*.jsonl"),
					"db":      filepath.Join(".cosca", "session.db"),
				})
			}
			formatter.Success(fmt.Sprintf(
				"%d mensagem(ns) indexada(s) de .cosca/sessions em .cosca/session.db", count))
			return nil
		},
	}
}

// NewSessionSearchCommand cria `cosca session search "<frase>"`.
func NewSessionSearchCommand() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "search <frase>",
		Short: "Busca FTS5 nas mensagens indexadas (zero LLM)",
		Long: `Busca full-text nas mensagens indexadas. Zero LLM: a busca é um
MATCH determinístico no FTS5 de .cosca/session.db, sem chamadas de modelo.

A frase é sanitizada (caracteres especiais do FTS5 como ", (, : são
neutralizados) e tratada como termos literais. Resultados em ordem
cronológica decrescente (mais recentes primeiro).`,
		Example: `  cosca session search "CKL"
  cosca session search --limit 20 "tatuagem"
  cosca session search --json "memória"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			db, err := resolveSessionDB()
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			hits, err := sessionindex.Search(cmd.Context(), db, args[0], limit)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, hits)
			}
			if len(hits) == 0 {
				formatter.Warning("Nenhuma ocorrência")
				return nil
			}

			formatter.Header(fmt.Sprintf(
				"Busca em conversas — %d ocorrência(s) para %q", len(hits), args[0]))
			rows := make([][]string, 0, len(hits))
			for _, h := range hits {
				rows = append(rows, []string{
					h.SessionID,
					h.Role,
					formatSessionTimestamp(h.Timestamp),
					truncateSessionContent(h.Content, 80),
				})
			}
			formatter.Table([]string{"Sessão", "Papel", "Quando", "Conteúdo"}, rows)
			return nil
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "l", 10, "número máximo de ocorrências")
	return cmd
}

// NewSessionLineageCommand cria `cosca session lineage <id>`.
func NewSessionLineageCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "lineage <id>",
		Short: "Sessão + descendentes (via parent_session_id no meta)",
		Long: `Mostra a linhagem da sessão: o próprio id seguido de toda sessão
cujo meta referencia este id como parent_session_id (diretamente ou em
cadeia). Se o meta não carregar parent_session_id — ou a sessão não tiver
descendentes — apenas a sessão base é exibida (comportamento gracioso).`,
		Example: `  cosca session lineage cosca-voice
  cosca session lineage cosca-voice --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			db, err := resolveSessionDB()
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			ids, err := sessionindex.SessionLineage(db, args[0])
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, ids)
			}
			if len(ids) == 1 {
				formatter.Warning(fmt.Sprintf(
					"Sessão %q não possui descendentes (parent_session_id ausente em .cosca/sessions).", args[0]))
				return nil
			}

			formatter.Header(fmt.Sprintf("Linhagem de %q — %d sessão(ões)", args[0], len(ids)))
			for i, id := range ids {
				formatter.Bullet(fmt.Sprintf("%d. %s", i+1, id))
			}
			return nil
		},
	}
}

// formatSessionTimestamp imprime um timestamp Unix (segundos) no padrão pt-BR
// amigável da casa. Zero → "desconhecido".
func formatSessionTimestamp(ts int64) string {
	if ts == 0 {
		return "desconhecido"
	}
	return time.Unix(ts, 0).UTC().Format("2006-01-02 15:04:05")
}

// truncateSessionContent trunca o conteúdo em <max> caracteres (contando
// runas para não quebrar UTF-8), adicionando reticências.
func truncateSessionContent(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-3]) + "..."
}

// ── Session Lock Commands ─────────────────────────────────────────────────────

// NewSessionRegisterCommand creates `cosca session register`.
func NewSessionRegisterCommand() *cobra.Command {
	var files, agentName string

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Registra arquivos em uso por esta sessão",
		Long: `Marca os arquivos como "em uso" por esta sessão no quadro de avisos
entre sessões. Outros agentes podem consultar com 'cosca session status'
para saber se um arquivo está sendo trabalhado e esperar antes de tocá-lo.

O agente deve chamar 'cosca session ready --commit <hash>' quando terminar.`,
		Example: `  cosca session register --files "knowledge.go,provider.go" --agent cosca-kernel`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)

			if files == "" {
				return fmt.Errorf("--files é obrigatório (lista separada por vírgula)")
			}

			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getwd: %w", err)
			}

			coscaDir := filepath.Join(dir, ".cosca")
			store, err := sessionstatus.Open(coscaDir)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer store.Close()

			fileList := splitAndTrim(files)
			sid := sessionstatus.SessionID()

			if err := store.Register(sid, agentName, fileList); err != nil {
				return fmt.Errorf("register: %w", err)
			}

			formatter.Success(fmt.Sprintf(
				"Sessão %s (%s): %d arquivo(s) registrado(s) como 'working'",
				sid, agentName, len(fileList),
			))
			for _, f := range fileList {
				formatter.Bullet(f)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&files, "files", "", "Arquivos em uso (separados por vírgula)")
	cmd.Flags().StringVar(&agentName, "agent", "", "Nome do agente que está trabalhando")
	_ = cmd.MarkFlagRequired("files")

	return cmd
}

// NewSessionReadyCommand creates `cosca session ready`.
func NewSessionReadyCommand() *cobra.Command {
	var commitHash string

	cmd := &cobra.Command{
		Use:   "ready",
		Short: "Marca os arquivos desta sessão como prontos",
		Long: `Transiciona todos os arquivos registrados com 'cosca session register'
de 'working' para 'ready', indicando que o trabalho está concluído e
outros agentes podem prosseguir.

Registra o hash do commit para rastreabilidade.`,
		Example: `  cosca session ready --commit abc1234`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)

			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getwd: %w", err)
			}

			coscaDir := filepath.Join(dir, ".cosca")
			store, err := sessionstatus.Open(coscaDir)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer store.Close()

			sid := sessionstatus.SessionID()

			if err := store.Ready(sid, commitHash); err != nil {
				return fmt.Errorf("ready: %w", err)
			}

			if commitHash != "" {
				formatter.Success(fmt.Sprintf(
					"Sessão %s: arquivos marcados como 'ready' (commit %s)", sid, commitHash,
				))
			} else {
				formatter.Success(fmt.Sprintf(
					"Sessão %s: arquivos marcados como 'ready'", sid,
				))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&commitHash, "commit", "", "Hash do commit que concluiu o trabalho")

	return cmd
}

// NewSessionStatusCommand creates `cosca session status`.
func NewSessionStatusCommand() *cobra.Command {
	var fileFilter string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Mostra o status dos arquivos entre sessões",
		Long: `Exibe o quadro de avisos entre sessões: quais arquivos estão em uso,
por qual agente, e quais já estão prontos.

Com --file, mostra apenas o status de um arquivo específico.`,
		Example: `  cosca session status
  cosca session status --file knowledge.go`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getwd: %w", err)
			}

			coscaDir := filepath.Join(dir, ".cosca")
			store, err := sessionstatus.Open(coscaDir)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer store.Close()

			if fileFilter != "" {
				fs, err := store.StatusForFile(fileFilter)
				if err != nil {
					return fmt.Errorf("status: %w", err)
				}
				if useJSON {
					return printJSON(cmd, fs)
				}
				if !fs.InUse && !fs.Ready {
					formatter.Println(fmt.Sprintf("%s — livre (nenhuma sessão trabalhando)", fs.FilePath))
				} else if fs.InUse {
					formatter.Warning(fmt.Sprintf("%s — em uso por %s (%s) desde %s",
						fs.FilePath, fs.Agent, fs.Owner, "hoje"))
					formatter.Println("  Aguarde 'cosca session ready' antes de tocar neste arquivo.")
				} else {
					formatter.Success(fmt.Sprintf("%s — pronto (commit %s)",
						fs.FilePath, fs.Commit))
				}
				return nil
			}

			items, err := store.Status()
			if err != nil {
				return fmt.Errorf("status: %w", err)
			}

			if useJSON {
				return printJSON(cmd, items)
			}

			if len(items) == 0 {
				formatter.Println("Nenhum arquivo registrado no quadro de avisos.")
				return nil
			}

			formatter.Header(fmt.Sprintf("Quadro de avisos — %d arquivo(s)", len(items)))
			for _, fs := range items {
				if fs.InUse {
					formatter.Warning(fmt.Sprintf("  ⏳ %s — em uso por %s (%s)",
						fs.FilePath, fs.Agent, fs.Owner))
				} else if fs.Ready {
					formatter.Success(fmt.Sprintf("  ✅ %s — pronto (commit %s)",
						fs.FilePath, fs.Commit))
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&fileFilter, "file", "", "Filtrar por arquivo específico")

	return cmd
}

// splitAndTrim splits a comma-separated string and trims each element.
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
