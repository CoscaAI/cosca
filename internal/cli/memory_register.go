package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/memoryguard"
	"github.com/spf13/cobra"
)

// NewMemoryRegisterCommand creates the `cosca memory register` subcommand.
// It automates the mechanical parts of registering a learning
// (MEMORY_ACCESS_PROTOCOL.md §3): derive next L-number + PREV, build the
// block, compute sha256(conteúdo completo), write block → trigger → chain.dat
// → merkle. Commit + sign remain explicit (ORDEM SAGRADA L199).
func NewMemoryRegisterCommand() *cobra.Command {
	var (
		agent      string
		title      string
		level      int
		tags       string
		task       string
		technique  string
		outcome    string
		confidence float64
		learned    string
		next       string
		related    string
		dryRun     bool
		force      bool
	)

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Registrar um aprendizado na memória do kernel (fluxo automático)",
		Long: `Registra um aprendizado na memória de um agente seguindo o
MEMORY_ACCESS_PROTOCOL.md §3. Deriva o próximo L-number e o PREV, monta o
block, calcula sha256(conteúdo completo) e atualiza learnings.md + chain.dat
+ merkle — tudo de uma vez.

NÃO faz commit nem assina a family chain (ordem sagrada L199).`,
		Example: `  cosca memory register \
    --title "Exemplo: padrão descoberto" \
    --level 4 \
    --tags "#dominio #padrao #level-4" \
    --task "o que estava sendo feito" \
    --technique "técnica aplicada" \
    --outcome success \
    --learned "o que foi descoberto" \
    --next "próximo passo"

  cosca memory register --title "..." --level 3 --tags "#a #b" --dry-run`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)

			dir, err := os.Getwd()
			if err != nil {
				return err
			}

			// Portão de autorização do Don: máquina (DPAPI, vínculo máquina+usuário)
			// + presença/consentimento-ao-conteúdo (nonce derivado do conteúdo,
			// TTY obrigatório). Só o Don presente no terminal consegue registrar
			// na memória do kernel.
			if err := verifyDonIdentity(dir, os.Stdin); err != nil {
				fmt.Fprintf(os.Stderr, "acesso negado — %v\n", err)
				return ExitCodeError{Code: 1}
			}

			agentDir := filepath.Join(dir, "internal", "embed", "cosca", "memory", "agent", agent)

			input := memory.LearningInput{
				Agent:      agent,
				Title:      title,
				Level:      level,
				Tags:       splitTags(tags),
				Task:       task,
				Technique:  technique,
				Outcome:    outcome,
				Confidence: confidence,
				Learned:    learned,
				Next:       next,
				Related:    related,
			}

			if dryRun {
				res, content, err := memory.PreviewLearning(agentDir, input)
				if err != nil {
					return err
				}
				formatter.Println("")
				formatter.Header("Preview (--dry-run, nada gravado)")
				formatter.Bullet(fmt.Sprintf("ID: %s | PREV: %.16s…", res.ID, res.Prev))
				formatter.Bullet(fmt.Sprintf("Hash: %s", res.Hash))
				formatter.Println("")
				formatter.Header("4ª Muralha (memoryguard)")
				printGuard(formatter, res.Guard)
				formatter.Println("")
				formatter.Println("--- block content ---")
				formatter.Println(content)
				return nil
			}

			var res *memory.LearningResult
			if force {
				res, err = memory.RegisterLearning(agentDir, input, memory.WithForce())
			} else {
				res, err = memory.RegisterLearning(agentDir, input)
			}
			if err != nil {
				return err
			}

			formatter.Println("")
			formatter.Success(fmt.Sprintf("Aprendizado %s registrado", res.ID))
			formatter.Bullet(fmt.Sprintf("Block: %s", res.BlockPath))
			formatter.Bullet(fmt.Sprintf("Hash16: %s", res.Hash16))
			if res.Guard != nil && !res.Guard.Approved {
				formatter.Warning("4ª Muralha reprovou — gravado por override do Don (--force); revisar")
				for _, r := range res.Guard.Reasons {
					formatter.Bullet("  ✗ " + r)
				}
			}

			// FASE 2 — ingestão automática pós-registro válido. O bloco
			// recém-registrado é classificado (fail-closed) e, se persistente,
			// ingerido no Knowledge Base com proveniência semântica
			// (scope/origin/kind/agent). Idempotente por SHA256 (nome do bloco).
			// O scope é decidido pelo classificador: para o cérebro embarcado
			// (internal/embed/cosca/**) → global; origem de projeto → project.
			// Ingestão NUNCA promove project→global. Best-effort: a falha de
			// ingestão não derruba o registro — o aprendizado já está commitado
			// em chain.dat/merkle.
			if res.Guard == nil || res.Guard.Approved {
				if ke, kErr := openKnowledgeEngine(filepath.Join(dir, ".cosca"), false, dir); kErr == nil {
					if iErr := ke.Init(); iErr == nil {
						ing, ingErr := ke.IngestLearningBlock(cmd.Context(), res.BlockPath, agent)
						if ingErr != nil {
							formatter.Warning(fmt.Sprintf("Ingestão automática falhou: %v", ingErr))
						} else if ing != nil {
							switch ing.Status {
							case knowledge.IngestStatusIngested:
								formatter.Success(fmt.Sprintf("Ingestado no KB — %s/%s (hash %.16s…)", ing.Scope, ing.Kind, ing.Hash))
							case knowledge.IngestStatusAlreadyIngested:
								formatter.Warning("Já estava ingerido no KB (idempotente por SHA256)")
							case knowledge.IngestStatusSkippedNotPersistent:
								formatter.Warning(fmt.Sprintf("Bloco NÃO ingerido (fail-closed): %s", ing.Reason))
							}
						}
						_ = ke.Close()
					}
				}
			}

			formatter.Println("")
			formatter.Println("Próximos passos (não automáticos):")
			formatter.Bullet("git add + commit (ORDEM SAGRADA: commit ANTES de assinar)")
			formatter.Bullet("cosca-check --sign-auto")
			return nil
		},
	}

	cmd.Flags().StringVar(&agent, "agent", "cosca-kernel", "Agente dono do aprendizado")
	cmd.Flags().StringVar(&title, "title", "", "Título do aprendizado (obrigatório)")
	cmd.Flags().IntVar(&level, "level", 0, "Nível de capacidade (1-5, obrigatório)")
	cmd.Flags().StringVar(&tags, "tags", "", "Tags separadas por espaço (#a #b #c, obrigatório)")
	cmd.Flags().StringVar(&task, "task", "", "Contexto/tarefa")
	cmd.Flags().StringVar(&technique, "technique", "", "Técnica aplicada")
	cmd.Flags().StringVar(&outcome, "outcome", "success", "Resultado (success/partial/failure)")
	cmd.Flags().Float64Var(&confidence, "confidence", 0.85, "Confiança (0.00-1.00)")
	cmd.Flags().StringVar(&learned, "learned", "", "O que foi descoberto")
	cmd.Flags().StringVar(&next, "next", "", "Próximo passo")
	cmd.Flags().StringVar(&related, "related", "", "Aprendizados relacionados")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Apenas mostrar o que seria feito (sem gravar)")
	cmd.Flags().BoolVar(&force, "force", false, "gravar mesmo se a 4ª Muralha reprovar (só o Don — portão de 3 fatores já validou a presença)")

	return cmd
}

// printGuard exibe o veredito da 4ª Muralha no output.
func printGuard(formatter *OutputFormatter, guard *memoryguard.Verdict) {
	if guard == nil {
		formatter.Bullet("oráculo não executado")
		return
	}
	if guard.Approved {
		formatter.Bullet("APROVADO — aprendizado dentro da régua")
		return
	}
	formatter.Bullet("REPROVADO — bloqueado no caminho de escrita (fail-closed); override só com --force (Don)")
	for _, r := range guard.Reasons {
		formatter.Bullet("  ✗ " + r)
	}
}

// splitTags splits a space/comma-separated tag string into a clean slice.
func splitTags(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == ',' || r == '\t'
	})
	var out []string
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}
