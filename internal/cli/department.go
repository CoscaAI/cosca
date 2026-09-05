//
// `cosca department` — Conversas entre departamentos (Dept→Dept) com trilha
// de auditoria.
//
// Regra do Don: "Os departamentos podem conversar." Um departamento não
// precisa confiar cegamente no outro: o Security pode auditar o Developer.
// Cada pergunta/resposta/decisão fica registrada no ledger append-only
// .cosca/department.db — quem perguntou, quem respondeu, o que foi decidido.
//
// Fluxo do Don (exemplo): Executive pergunta → Security responde "há 2 riscos
// críticos" → Developer responde "um deles já foi corrigido" → Security
// valida → Executive resolve "release aprovado". Tudo agrupado em uma thread
// (topic:data) recuperável como trilha de auditoria.
//
// Subcomandos:
//   ask <--from --to --topic --msg>   Inicia uma conversa (pergunta) e cria a thread
//   answer <thread> --from --msg      Responde a quem falou por último na thread
//   resolve <thread> --from --msg     Registra a decisão final da thread
//   thread <thread>                   Trilha de auditoria completa da conversa
//   list [--limit N]                  Mensagens recentes de todas as conversas
//

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/department"
)

// resolveDepartmentStore abre o ledger de conversas entre departamentos do
// projeto atual (<projeto>/.cosca/department.db).
func resolveDepartmentStore() (*department.ConversationStore, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getwd: %w", err)
	}
	return department.NewConversationStore(filepath.Join(dir, ".cosca", "department.db"))
}

// NewDepartmentCommand cria a árvore de comandos `cosca department`.
func NewDepartmentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "department",
		Short: "Conversas entre departamentos (Dept→Dept) com trilha de auditoria",
		Long: `Conversas entre departamentos (Dept→Dept) com trilha de auditoria.

Os departamentos podem conversar: um departamento pergunta, o outro responde
com evidência e a decisão final é registrada. Ninguém precisa confiar cegamente
no outro — o Security pode auditar o Developer. Cada mensagem fica no ledger
append-only .cosca/department.db (quem perguntou, quem respondeu, o que foi
decidido), agrupada em uma thread (<topic>:<data>).

Fluxo do Don:
  Executive pergunta → Security responde "há 2 riscos críticos" → Developer
  responde "um deles já foi corrigido" → Security valida → Executive resolve
  "release aprovado".

Subcomandos:
  ask      Inicia uma conversa (pergunta) e cria a thread
  answer   Responde a quem falou por último na thread
  resolve  Registra a decisão final da thread
  thread   Trilha de auditoria completa da conversa
  list     Mensagens recentes de todas as conversas`,
		Example: `  cosca department ask --from executive --to security --topic release-approval --msg "podemos liberar a versão?"
  cosca department answer release-approval:20260802 --from security --msg "há 2 riscos críticos"
  cosca department answer release-approval:20260802 --from developer --msg "um deles já foi corrigido"
  cosca department answer release-approval:20260802 --from security --msg "validação passou"
  cosca department resolve release-approval:20260802 --from executive --msg "release aprovado"
  cosca department thread release-approval:20260802
  cosca department list --limit 20`,
	}

	cmd.AddCommand(
		NewDepartmentAskCommand(),
		NewDepartmentAnswerCommand(),
		NewDepartmentResolveCommand(),
		NewDepartmentThreadCommand(),
		NewDepartmentListCommand(),
	)
	return cmd
}

// NewDepartmentAskCommand cria `cosca department ask`.
func NewDepartmentAskCommand() *cobra.Command {
	var from, to, topic, msg string

	cmd := &cobra.Command{
		Use:   "ask",
		Short: "Inicia uma conversa: pergunta de um departamento a outro e cria a thread",
		Long: `Inicia uma conversa Dept→Dept: <from> pergunta a <to> sobre <topic>.
Cria a thread (<topic>:<data>) e registra a pergunta (kind "question") no
ledger append-only. Imprime o ID da mensagem (DM-YYYYMMDD-XXXX) e a chave da
thread — use a thread nos comandos answer/resolve/thread.

  --from  departamento que pergunta (developer, security, executive, ...)
  --to    departamento que responde
  --topic tópico da conversa (ex.: release-approval)
  --msg   a mensagem da pergunta`,
		Example: `  cosca department ask --from developer --to security --topic release-approval --msg "podemos liberar?"`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)

			if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
				return fmt.Errorf("--from e --to são obrigatórios (departamentos conhecidos: %s)", departmentNames())
			}
			if strings.TrimSpace(topic) == "" {
				return fmt.Errorf("--topic é obrigatório (ex.: release-approval)")
			}
			if strings.TrimSpace(msg) == "" {
				return fmt.Errorf("--msg é obrigatório")
			}

			store, err := resolveDepartmentStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			director := department.NewDirector(store)
			msgID, thread, err := director.Ask(
				department.DepartmentID(from),
				department.DepartmentID(to),
				topic, msg,
			)
			if err != nil {
				return err
			}

			if IsJSONOutput(cmd) {
				return printJSON(cmd, map[string]string{
					"message_id": msgID,
					"thread_id":  thread,
					"from":       from,
					"to":         to,
					"topic":      topic,
					"kind":       "question",
				})
			}
			formatter.Success(fmt.Sprintf("Pergunta enviada de %s para %s", from, to))
			formatter.KeyValue("Mensagem", msgID)
			formatter.KeyValue("Thread", thread)
			return nil
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "departamento que pergunta — obrigatório")
	cmd.Flags().StringVar(&to, "to", "", "departamento que responde — obrigatório")
	cmd.Flags().StringVar(&topic, "topic", "", "tópico da conversa (ex.: release-approval) — obrigatório")
	cmd.Flags().StringVar(&msg, "msg", "", "a mensagem da pergunta — obrigatório")
	return cmd
}

// NewDepartmentAnswerCommand cria `cosca department answer <thread>`.
func NewDepartmentAnswerCommand() *cobra.Command {
	var from, msg string

	cmd := &cobra.Command{
		Use:   "answer <thread>",
		Short: "Responde a quem falou por último na thread (evidência/contraprova)",
		Long: `Responde dentro de uma thread existente: <from> responde a quem enviou
a última mensagem (o interlocutor é determinado pelo ledger — sem confiança
cega). A mensagem é do tipo "answer" (evidência / contraprova) e fica na
trilha de auditoria.

  --from  departamento que responde (developer, security, executive, ...)
  --msg   a resposta`,
		Example: `  cosca department answer release-approval:20260802 --from security --msg "há 2 riscos críticos"`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)

			if strings.TrimSpace(from) == "" {
				return fmt.Errorf("--from é obrigatório (departamentos conhecidos: %s)", departmentNames())
			}
			if strings.TrimSpace(msg) == "" {
				return fmt.Errorf("--msg é obrigatório")
			}

			store, err := resolveDepartmentStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			director := department.NewDirector(store)
			if err := director.Answer(args[0], from, msg); err != nil {
				return err
			}

			if IsJSONOutput(cmd) {
				return printJSON(cmd, map[string]string{
					"thread_id": args[0],
					"from":      from,
					"kind":      "answer",
					"message":   msg,
				})
			}
			formatter.Success(fmt.Sprintf("Resposta de %s registrada", from))
			formatter.KeyValue("Thread", args[0])
			return nil
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "departamento que responde — obrigatório")
	cmd.Flags().StringVar(&msg, "msg", "", "a resposta — obrigatório")
	return cmd
}

// NewDepartmentResolveCommand cria `cosca department resolve <thread>`.
func NewDepartmentResolveCommand() *cobra.Command {
	var from, msg string

	cmd := &cobra.Command{
		Use:   "resolve <thread>",
		Short: "Registra a decisão final da thread (approval/denial)",
		Long: `Registra a decisão final da thread: <from> decide e a mensagem é do tipo
"approval" — o fechamento do diálogo ("release aprovado"). O destinatário é o
último interlocutor da thread (determinado pelo ledger).

  --from  departamento que decide (developer, security, executive, ...)
  --msg   a decisão final`,
		Example: `  cosca department resolve release-approval:20260802 --from executive --msg "release aprovado"`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)

			if strings.TrimSpace(from) == "" {
				return fmt.Errorf("--from é obrigatório (departamentos conhecidos: %s)", departmentNames())
			}
			if strings.TrimSpace(msg) == "" {
				return fmt.Errorf("--msg é obrigatório")
			}

			store, err := resolveDepartmentStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			director := department.NewDirector(store)
			if err := director.Resolve(args[0], from, msg); err != nil {
				return err
			}

			if IsJSONOutput(cmd) {
				return printJSON(cmd, map[string]string{
					"thread_id": args[0],
					"from":      from,
					"kind":      "approval",
					"decision":  msg,
				})
			}
			formatter.Success(fmt.Sprintf("Decisão final de %s registrada", from))
			formatter.KeyValue("Thread", args[0])
			return nil
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "departamento que decide — obrigatório")
	cmd.Flags().StringVar(&msg, "msg", "", "a decisão final — obrigatório")
	return cmd
}

// NewDepartmentThreadCommand cria `cosca department thread <thread>`.
func NewDepartmentThreadCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "thread <thread>",
		Short: "Trilha de auditoria completa de uma conversa (ordem cronológica)",
		Long: `Imprime a trilha de auditoria completa de uma conversa: tabela (tempo,
de, para, tipo, mensagem) ordenada por criação — quem perguntou, quem
respondeu, o que foi decidido. Ledger append-only — apenas leitura.

  --json   emite as mensagens brutas em JSON (mesmos campos do ledger).`,
		Example: `  cosca department thread release-approval:20260802
  cosca department thread release-approval:20260802 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)

			store, err := resolveDepartmentStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			msgs, err := store.Thread(args[0])
			if err != nil {
				return err
			}
			if IsJSONOutput(cmd) {
				return printJSON(cmd, msgs)
			}
			if len(msgs) == 0 {
				formatter.Warning(fmt.Sprintf("Thread %s sem mensagens — inicie com \"cosca department ask --topic ...\".", args[0]))
				return nil
			}

			formatter.Header(fmt.Sprintf("THREAD %s — trilha de auditoria", args[0]))
			formatter.KeyValue("Mensagens", fmt.Sprintf("%d", len(msgs)))
			formatter.KeyValue("Tópico", msgs[0].Topic)
			formatter.KeyValue("Primeira", msgs[0].CreatedAt.Format(time.RFC3339))
			formatter.KeyValue("Última", msgs[len(msgs)-1].CreatedAt.Format(time.RFC3339))

			rows := make([][]string, 0, len(msgs))
			for _, m := range msgs {
				rows = append(rows, []string{
					m.CreatedAt.Format("2006-01-02 15:04:05"),
					m.From.String(),
					m.To.String(),
					m.Kind,
					m.ID,
					truncateDepartmentMessage(m.Message, 50),
				})
			}
			formatter.Table([]string{"Tempo", "De", "Para", "Tipo", "Mensagem", "Detalhe"}, rows)
			return nil
		},
	}
}

// NewDepartmentListCommand cria `cosca department list [--limit N]`.
func NewDepartmentListCommand() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Mensagens recentes de todas as conversas entre departamentos",
		Long: `Lista as mensagens mais recentes de todas as conversas entre
departamentos, do mais novo para o mais antigo, em uma tabela (tempo, thread,
de, para, tipo, mensagem). Rastro append-only — apenas leitura.

  --limit N   número máximo de mensagens (padrão 20)`,
		Example: `  cosca department list
  cosca department list --limit 50
  cosca department list --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)

			store, err := resolveDepartmentStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			msgs, err := store.List(limit)
			if err != nil {
				return err
			}
			if IsJSONOutput(cmd) {
				return printJSON(cmd, msgs)
			}
			if len(msgs) == 0 {
				formatter.Warning("Nenhuma conversa ainda — rode \"cosca department ask --from ... --to ... --topic ... --msg ...\".")
				return nil
			}

			rows := make([][]string, 0, len(msgs))
			for _, m := range msgs {
				rows = append(rows, []string{
					m.CreatedAt.Format("2006-01-02 15:04:05"),
					m.ThreadID,
					m.From.String(),
					m.To.String(),
					m.Kind,
					m.ID,
					truncateDepartmentMessage(m.Message, 40),
				})
			}
			formatter.Header(fmt.Sprintf("Conversas recentes — %d mensagem(ns)", len(msgs)))
			formatter.Table([]string{"Tempo", "Thread", "De", "Para", "Tipo", "Mensagem", "Detalhe"}, rows)
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "número máximo de mensagens a listar")
	return cmd
}

// departmentNames devolve a lista estática de departamentos conhecidos em
// formato legível para mensagens de erro (pt-BR).
func departmentNames() string {
	names := make([]string, 0, len(department.KnownDepartments))
	for _, d := range department.KnownDepartments {
		names = append(names, string(d))
	}
	return strings.Join(names, ", ")
}

// truncateDepartmentMessage reduz a mensagem para o tamanho dado (com "…") —
// usado nas tabelas para não estourar a largura.
func truncateDepartmentMessage(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return strings.TrimSpace(s[:max-1]) + "…"
}
