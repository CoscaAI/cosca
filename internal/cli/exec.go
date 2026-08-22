package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/engine"
	"github.com/CoscaAI/cosca/internal/pipeline"
)

// ─── Feature Gate ────────────────────────────────────────────────────────────
//
// O `cosca exec` vive no binário único `cosca` (o standalone cosca-chat foi
// removido) e está DESATIVADO POR PADRÃO (regra do Don). Sem COSCA_ENABLE_EXEC=1/true
// o comando nunca executa trabalho (fail-closed).

// execEnabled retorna erro fail-closed quando o feature gate está fechado.
func execEnabled() error {
	v := strings.ToLower(os.Getenv("COSCA_ENABLE_EXEC"))
	if v != "1" && v != "true" {
		return fmt.Errorf("comando desativado por padrão: defina COSCA_ENABLE_EXEC=1 para habilitar 'cosca exec'")
	}
	return nil
}

// NewExecCommand creates the `cosca exec` command.
func NewExecCommand() *cobra.Command {
	execCmd := &cobra.Command{
		Use:   "exec <prompt>",
		Short: "Execute a non-interactive prompt (CI/CD)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := execEnabled(); err != nil {
				return err
			}

			model, _ := cmd.Flags().GetString("model")
			maxTurns, _ := cmd.Flags().GetInt("max-turns")
			jsonOutput, _ := cmd.Flags().GetBool("json")
			sessionID, _ := cmd.Flags().GetString("session")
			clearSession, _ := cmd.Flags().GetBool("clear-session")
			agentName, _ := cmd.Flags().GetString("agent")

			eng, err := buildEngine(model)
			if err != nil {
				return err
			}

			// Build pipeline history from session.
			var history []pipeline.Message
			// ── Persistent conversation memory ──────────────────────────────
			var sm *engine.SessionManager
			var sess *engine.Session
			if sessionID != "" {
				sm = engine.NewSessionManagerDefault()
				if clearSession {
					if err := sm.DeleteSession(sessionID); err != nil {
						return fmt.Errorf("clear session: %w", err)
					}
				}
				sess, err = sm.LoadSession(sessionID)
				if err != nil {
					sess = sm.CreateSessionWithID(sessionID, model, "default")
					fmt.Fprintf(os.Stderr, "Memória criada: %s (nova sessão em .cosca/sessions/%s.jsonl)\n",
						sessionID, sessionID)
				} else {
					fmt.Fprintf(os.Stderr, "Memória carregada: %s (%d mensagens de histórico)\n",
						sessionID, len(sess.Messages))
				}
				for _, m := range sess.Messages {
					history = append(history, pipeline.Message{
						Role:    string(m.Role),
						Content: m.Content,
					})
				}
			}

			runner := pipeline.NewEngineAdapter(eng)
			result, err := runner.Run(cmd.Context(), pipeline.RunRequest{
				Prompt:  args[0],
				Agent:   agentName,
				History: history,
				Options: pipeline.RunOptions{
					MaxTurns: maxTurns,
				},
			})
			if err != nil {
				return err
			}

			if sessionID != "" {
				if err := sm.AppendMessage(sessionID, chat.Message{Role: chat.RoleUser, Content: args[0]}); err != nil {
					return fmt.Errorf("append user message: %w", err)
				}
				if err := sm.AppendMessage(sessionID, chat.Message{Role: chat.RoleAssistant, Content: result.Response}); err != nil {
					return fmt.Errorf("append assistant message: %w", err)
				}
				sess.TokenUsage = chat.Usage{
					PromptTokens:     result.TokenUsage.Input,
					CompletionTokens: result.TokenUsage.Output,
					TotalTokens:      result.TokenUsage.Input + result.TokenUsage.Output,
				}
				if err := sm.SaveSession(sessionID); err != nil {
					return fmt.Errorf("save session: %w", err)
				}
				fmt.Fprintf(os.Stderr, "Memória salva: %s\n", sessionID)
			}

			if jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			fmt.Println(result.Response)
			fmt.Fprintf(os.Stderr, "Tokens: %d total, Turns: %d\n",
				result.TokenUsage.Input+result.TokenUsage.Output, result.TurnCount)
			return nil
		},
	}
	execCmd.Flags().StringP("model", "m", "deepseek", "LLM model")
	execCmd.Flags().Int("max-turns", 10, "maximum agent turns")
	execCmd.Flags().Bool("json", false, "output as JSON")
	execCmd.Flags().String("session", "", "session ID to persist conversation history (creates/loads {id}.jsonl under .cosca/sessions/)")
	execCmd.Flags().Bool("clear-session", false, "delete the session before creating a new one (resets conversation memory)")
	execCmd.Flags().String("agent", "cosca-kernel", "agent to use (default: cosca-kernel — the Kernel/consigliere)")
	return execCmd
}
