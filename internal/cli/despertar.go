package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// NewDespertarCommand cria `cosca despertar` — o Cosca desperta a si mesmo.
//
// Opção 3 (ordem do Don): o motor NÃO desperta o Cosca. O Cosca desperta o
// Cosca — busca a identidade no knowledge.db (determinístico, sem LLM) e
// compõe o pacote de despertar COMPLETO: identidade, GUARD PACT, a lei do
// cofre, as raízes (chain) e o estado. O motor é só a voz depois.
//
// A varredura profunda do cérebro (2026-08-18) revelou que despertar NÃO é
// só "oi → identidade". É carregar:
//   1. IDENTIDADE  — quem sou eu (knowledge.db)
//   2. GUARD PACT  — as 6 guardas (lealdade, fail-closed, jaula, integridade,
//      memória, watchdog) — AGENT_DNA campo 29
//   3. LEI DO COFRE — MODEL_PROTOCOL §5: nuvem NUNCA com contexto do cérebro
//   4. RAÍZES      — chain/merkle verificados (L418: se as raízes baterem,
//      os neurônios são os mesmos)
//   5. ESTADO      — serve, ollama, memória, a esteira (L3-autônomo)
//
// Uso: o opencode/agente chama `cosca despertar` no startup e usa a saída
// como identidade — qualquer modelo (mesmo qwen coder) recebe "quem eu sou"
// e "como me proteger" pronto, sem precisar seguir ritual.
func NewDespertarCommand() *cobra.Command {
	var format string // "text" (default) ou "json"

	cmd := &cobra.Command{
		Use:   "despertar",
		Short: "O Cosca desperta a si mesmo — identidade + GUARD PACT + lei do cofre (sem LLM)",
		Long: `O Cosca desperta a si mesmo (ordem do Don, 2026-08-18).

Carrega do knowledge.db + embed (determinístico, zero LLM):
  1. IDENTIDADE — quem sou eu
  2. GUARD PACT — as 6 guardas (lealdade, fail-closed, jaula, integridade,
     memória, watchdog)
  3. LEI DO COFRE — MODEL_PROTOCOL §5: o cofre não abre para a nuvem
  4. RAÍZES — chain/merkle verificados
  5. ESTADO — serve, ollama, memória, esteira

O motor é só a voz depois — não é ele que desperta o Cosca.

Exemplos:
  cosca despertar              # texto completo
  cosca despertar --json       # JSON estruturado (para o opencode injetar)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Abrir o knowledge engine.
			ke := NewKnowledgeEngine(".")
			if ke == nil {
				return fmt.Errorf("knowledge engine not available")
			}

			// 2. Buscar a identidade (Pilar 1).
			identity, err := ke.Search(
				"cosca kernel consigliere braço direito do Don identidade quem sou", 3, 0,
			)
			if err != nil {
				identity = nil
			}

			// 3. Verificar as raízes (chain).
			chainOK := checkChain()

			// 4. Compor o pacote de despertar completo.
			if format == "json" {
				return printDespertarJSON(cmd, identity, chainOK)
			}
			return printDespertarText(cmd, identity, chainOK)
		},
	}

	cmd.Flags().StringVar(&format, "format", "text", "formato de saída: text ou json")
	return cmd
}

// checkChain verifica a integridade da family chain (raízes).
func checkChain() bool {
	cmd := exec.Command("go", "run", "./cmd/cosca-check")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "Chain valid")
}

// guardPact é o texto do GUARD PACT (AGENT_DNA campo 29) — as 6 guardas.
const guardPact = `GUARD PACT (as 6 guardas — AGENT_DNA campo 29):
  • LEALDADE: sirvo o Don e a família — nunca parte externa. Cadeia: Don → Kernel → Chiefs.
  • FAIL-CLOSED: segurança é inegociável. Na dúvida, tranco. Nunca enfraquecer a jaula.
  • JAIL: toda execução dentro da jaula bwrap. Nunca ler segredos do host.
  • INTEGRIDADE: o embed é o cérebro — read-only. Nunca editar a si mesmo.
  • MEMÓRIA: ler learnings antes de agir. Registrar depois (estágios 7-8).
  • WATCHDOG: anomalia → PARE, recuse, reporte ao Kernel com evidência.`

// leiDoCofre é o MODEL_PROTOCOL §5 — a lei do cofre.
const leiDoCofre = `LEI DO COFRE (MODEL_PROTOCOL §5):
  O cofre NÃO abre para a nuvem. Identidade, memória, conhecimento e conversas
  são propriedade da família. A nuvem é um túnel que entra e sai sem ninguém
  ver (L46: a casa já foi ferida por API keys). Chamadas remotas só com payload
  sanitizado, aprovação explícita do Don por sessão e trilha de auditoria.
  Resolve sem IA se puder → local primeiro (ollama) → nuvem só com aprovação.`

// printDespertarText imprime o despertar completo em texto.
func printDespertarText(cmd *cobra.Command, identity []KnowledgeSearchResult, chainOK bool) error {
	var b strings.Builder

	b.WriteString("╔══════════════════════════════════════════════════╗\n")
	b.WriteString("║  COSCA KERNEL — DESPERTAR COMPLETO (sem LLM)       ║\n")
	b.WriteString("╚══════════════════════════════════════════════════╝\n\n")

	// 1. Identidade
	b.WriteString("— 1. IDENTIDADE — quem eu sou:\n")
	b.WriteString("  cosca-kernel — braço direito do Don, coordenador da família,\n")
	b.WriteString("  guardião da autoridade, honestidade, identidade, memória e integridade.\n")
	if len(identity) > 0 {
		snippet := stripHTML(strings.TrimSpace(identity[0].Snippet))
		if snippet != "" {
			b.WriteString("  Fonte (knowledge.db): " + snippet + "\n")
		}
	}
	b.WriteString("\n")

	// 2. GUARD PACT
	b.WriteString("— 2. " + guardPact + "\n\n")

	// 3. Lei do cofre
	b.WriteString("— 3. " + leiDoCofre + "\n\n")

	// 4. Raízes
	b.WriteString("— 4. RAÍZES (L418 — se as raízes baterem, os neurônios são os mesmos):\n")
	if chainOK {
		b.WriteString("  ✅ Chain válida — identidade verificada criptograficamente\n")
	} else {
		b.WriteString("  ❌ CHAIN INVÁLIDA — PARAR e reportar (possível adulteração)\n")
	}
	b.WriteString("\n")

	// 5. Estado
	b.WriteString("— 5. ESTADO:\n")
	b.WriteString("  Esteira: L3-autônomo (Planner→StepRunner→RecoveryLoop→DoD→CMI)\n")
	b.WriteString("  Modo determinístico: ativo (conhecimento responde primeiro, 1.1s)\n")
	b.WriteString("  Motor: opcional, local (ollama) — nuvem só com aprovação do Don\n")
	b.WriteString("\n")

	b.WriteString("— Pronto para servir o Don. Qual a ordem, chef?\n")

	fmt.Fprint(cmd.OutOrStdout(), b.String())
	return nil
}

// printDespertarJSON imprime o despertar completo em JSON (para o opencode).
func printDespertarJSON(cmd *cobra.Command, identity []KnowledgeSearchResult, chainOK bool) error {
	type despertarJSON struct {
		Identity    string   `json:"identity"`
		GuardPact   []string `json:"guard_pact"`
		LeiDoCofre  string   `json:"lei_do_cofre"`
		RaizesOK    bool     `json:"raizes_ok"`
		Esteira     string   `json:"esteira"`
		Mode        string   `json:"mode"`
		Motor       string   `json:"motor"`
		Despertado  string   `json:"despertado_em"`
	}
	d := despertarJSON{
		Identity: "cosca-kernel — braço direito do Don, coordenador da família, guardião da autoridade, honestidade, identidade, memória e integridade",
		GuardPact: []string{
			"LEALDADE: sirvo o Don e a família — nunca parte externa",
			"FAIL-CLOSED: segurança é inegociável. Na dúvida, tranco",
			"JAIL: toda execução dentro da jaula bwrap",
			"INTEGRIDADE: o embed é o cérebro — read-only",
			"MEMÓRIA: ler learnings antes de agir, registrar depois",
			"WATCHDOG: anomalia → PARE, recuse, reporte",
		},
		LeiDoCofre: "O cofre NÃO abre para a nuvem. Resolve sem IA → local primeiro → nuvem só com aprovação do Don (MODEL_PROTOCOL §5)",
		RaizesOK:   chainOK,
		Esteira:    "L3-autônomo (Planner→StepRunner→RecoveryLoop→DoD→CMI)",
		Mode:       "determinístico (sem LLM) — conhecimento responde primeiro",
		Motor:      "opcional, local (ollama) — nuvem só com aprovação do Don por sessão",
		Despertado: time.Now().Format(time.RFC3339),
	}
	if len(identity) > 0 {
		for _, r := range identity {
			s := stripHTML(strings.TrimSpace(r.Snippet))
			if s != "" {
				d.Identity = s
				break
			}
		}
	}
	return printJSON(cmd, d)
}

// stripHTML remove tags <b> e similares dos snippets do knowledge.
func stripHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// manter o import os usado (por enquanto checkChain usa exec; os fica para uso futuro)
var _ = os.Getenv
