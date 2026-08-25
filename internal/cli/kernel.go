package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/CoscaAI/cosca/internal/embed"
	"github.com/CoscaAI/cosca/internal/integrity"
	"github.com/CoscaAI/cosca/internal/kernel"
	"github.com/spf13/cobra"
)

// NewKernelCommand creates the `cosca kernel` command tree.
// This exposes the Kernel's consciousness (identity, memory, status) via CLI.
func NewKernelCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kernel",
		Short: "Inspect and operate the Cosca Kernel consciousness",
		Long: `Inspect and operate the Cosca Kernel consciousness running in Go.

The kernel is the runtime embodiment of the Cosca Kernel — the consigliere
of the Don. It reads its memory from the knowledge base (.cosca/knowledge.db)
which is compiled from the authoritative .cosca/framework source.

Subcommands:
  identity    Show who the Kernel is (persona, laws, constitution)
  memory      Show the Kernel's memory (learnings, failures, knowledge)
  status      Show memory statistics from the knowledge base
  self-test   Run kernel integrity self-assessment`,
	}

	cmd.AddCommand(
		NewKernelIdentityCommand(),
		NewKernelMemoryCommand(),
		NewKernelStatusCommand(),
		NewKernelSelfTestCommand(),
	)
	return cmd
}

// NewKernelIdentityCommand shows the kernel's identity.
func NewKernelIdentityCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "identity",
		Short: "Show who the Cosca Kernel is",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			id := kernel.Identity()
			if useJSON {
				return printJSON(cmd, id)
			}

			formatter.Header("Cosca Kernel — Identity")
			// Guarda-identidade (loyalty.go): verifica POR CÓDIGO se a identidade
			// é a canônica (ALMA.md). Se a alma está íntegra, reporta. Se foi
			// trocada (breach/desafio falhado), AVISA — o kernel não é "outra pessoa".
			if v, reason := verifyKernelAlma(); v != integrity.IDOk {
				formatter.KeyValue("⚠ Identidade", v.String()+" — "+reason)
			} else {
				formatter.KeyValue("⚠ Identidade", "✅ íntegra (ALMA verificada)")
			}
			formatter.KeyValue("Name", id.Name)
			formatter.KeyValue("Role", id.Role)
			formatter.KeyValue("Project", fmt.Sprintf("%s %s", id.Project, id.ProjectVer))
			formatter.KeyValue("Model", id.Model)
			formatter.KeyValue("Kernel Version", id.Version)
			formatter.KeyValue("Go", fmt.Sprintf("%s %s/%s", id.GoVersion, id.Platform, id.Arch))
			formatter.KeyValue("Language", id.Language)

			formatter.Header("Expertise")
			for _, e := range id.Expertise {
				formatter.Bullet(e)
			}

			formatter.Header("As 6 Leis do Kernel")
			for _, l := range kernel.Laws {
				formatter.KeyValue(fmt.Sprintf("L%d", l.Number), fmt.Sprintf("%s — %s", l.Title, l.Rule))
			}

			formatter.Header("Os 8 Princípios Constitucionais")
			for _, p := range kernel.Constitution {
				formatter.KeyValue(fmt.Sprintf("P%d", p.Number), fmt.Sprintf("%s (guardião: %s)", p.Title, p.Guardian))
			}
			return nil
		},
	}
}

// NewKernelMemoryCommand shows the kernel's memory.
func NewKernelMemoryCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "memory",
		Short: "Show the Kernel's memory from the knowledge base",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)
			ctx := context.Background()

			dbPath, err := resolveKnowledgeDB()
			if err != nil {
				// Graceful degradation: report and continue with identity-only.
				formatter.Warning(fmt.Sprintf("Knowledge base unavailable: %v", err))
				return nil
			}

			mem, err := kernel.OpenMemory(dbPath)
			if err != nil {
				formatter.Warning(fmt.Sprintf("Cannot open kernel memory: %v", err))
				return nil
			}
			defer mem.Close()

			stats, err := mem.Stats(ctx)
			if err != nil {
				// Graceful degradation: a corrupt or unreadable knowledge base
				// (e.g. "file is not a database") must not kill the command —
				// report a warning and return nil.
				formatter.Warning(fmt.Sprintf("Knowledge base corrupt or unreadable: %v", err))
				return nil
			}

			if useJSON {
				return printJSON(cmd, stats)
			}

			formatter.Header("Kernel Memory — Knowledge Base")
			formatter.KeyValue("Documents", fmt.Sprint(stats["documents"]))
			formatter.KeyValue("Chunks", fmt.Sprint(stats["chunks"]))
			formatter.KeyValue("Vectors", fmt.Sprint(stats["vectors"]))
			formatter.KeyValue("Learnings indexados", fmt.Sprint(stats["learnings"]))
			formatter.KeyValue("Failures indexados", fmt.Sprint(stats["failures"]))
			formatter.KeyValue("Knowledge entries", fmt.Sprint(stats["knowledge_entries"]))

			// Show recent learnings.
			learnings, err := mem.Learnings(ctx, 5)
			if err == nil && len(learnings) > 0 {
				formatter.Header("Amostra de Learnings")
				for _, l := range learnings {
					formatter.Bullet(fmt.Sprintf("%s — %s", l.Agent, l.Title))
				}
			}
			return nil
		},
	}
}

// NewKernelStatusCommand shows memory statistics.
func NewKernelStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show kernel memory statistics",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)
			ctx := context.Background()

			dbPath, err := resolveKnowledgeDB()
			if err != nil {
				formatter.Warning(fmt.Sprintf("Knowledge base unavailable: %v", err))
				return nil
			}

			mem, err := kernel.OpenMemory(dbPath)
			if err != nil {
				formatter.Warning(fmt.Sprintf("Cannot open kernel memory: %v", err))
				return nil
			}
			defer mem.Close()

			stats, err := mem.Stats(ctx)
			if err != nil {
				// Graceful degradation: a corrupt or unreadable knowledge base
				// (e.g. "file is not a database") must not kill the command —
				// report a warning and return nil.
				formatter.Warning(fmt.Sprintf("Knowledge base corrupt or unreadable: %v", err))
				return nil
			}
			if useJSON {
				return printJSON(cmd, stats)
			}
			formatter.KeyValue("Memory Status", "operational")
			for k, v := range stats {
				formatter.KeyValue(k, fmt.Sprint(v))
			}
			return nil
		},
	}
}

// NewKernelSelfTestCommand runs the kernel self-assessment.
func NewKernelSelfTestCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "self-test",
		Short: "Run kernel integrity self-assessment",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			checks := kernel.SelfTest()
			if useJSON {
				if err := printJSON(cmd, checks); err != nil {
					return err
				}
				for _, c := range checks {
					if !c.OK {
						return fmt.Errorf("kernel self-test failed: %s (%s)", c.Name, c.Detail)
					}
				}
				return nil
			}

			formatter.Header("Kernel Self-Test")
			allOK := true
			failed := 0
			for _, c := range checks {
				status := "✅"
				if !c.OK {
					status = "❌"
					allOK = false
					failed++
				}
				formatter.Bullet(fmt.Sprintf("%s %s — %s", status, c.Name, c.Detail))
			}
			if allOK {
				formatter.KeyValue("Result", "Kernel íntegro")
			} else {
				formatter.KeyValue("Result", "FALHA — investigar")
			}
			if !allOK {
				// CI hook: fail the command after the result is printed.
				return fmt.Errorf("kernel self-test failed: %d/%d checks not OK", failed, len(checks))
			}
			return nil
		},
	}
}

// resolveKnowledgeDB locates the project's knowledge.db.
// Search order: $COSCA_KNOWLEDGE_DB (if set), cwd/.cosca/knowledge.db.
func resolveKnowledgeDB() (string, error) {
	if p := os.Getenv("COSCA_KNOWLEDGE_DB"); p != "" {
		if fi, err := os.Stat(p); err == nil && fi.Size() > 0 {
			return p, nil
		}
		return "", fmt.Errorf("knowledge.db not found at COSCA_KNOWLEDGE_DB=%s", p)
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	candidates := []string{
		filepath.Join(dir, ".cosca", "knowledge.db"),
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.Size() > 0 {
			return c, nil
		}
	}
	return "", fmt.Errorf("knowledge.db not found in %s", filepath.Join(dir, ".cosca"))
}

// verifyKernelAlma verifica POR CÓDIGO que a identidade do kernel é a canônica
// (ALMA.md). Lê a filosofia-genoma do embed, computa o digest BLAKE3 e valida
// a resposta ao desafio semântico. Se a alma foi trocada (breach/injeção),
// retorna IDBreach/IDChallengeFail — o kernel não é "outra pessoa".
func verifyKernelAlma() (integrity.VerdictIdentity, string) {
	// Camada 1 — lê a ALMA do embed e computa o digest (a "impressão digital da alma").
	data, err := embed.ReadFile("ALMA.md")
	if err != nil {
		return integrity.IDNoConfig, "ALMA.md nao acessivel no embed"
	}
	almaDigest := integrity.HashBytes(integrity.HashBLAKE3, data)
	// Registra o digest canônico (se ainda não configurado).
	if integrity.IdentityDigest() == "" {
		integrity.SetIdentityDigest(almaDigest)
	}
	// Camada 2 — verifica o digest + o desafio semântico (a resposta da alma).
	return integrity.VerifyIdentity(
		integrity.IdentityDigest(),
		// A resposta canônica à pergunta "quem é você" (extraída da ALMA).
		integrity.AlmaChallenge,
		"kernel->don", // a relação fundamental do grafo de lealdade
	)
}
