package cli

import (
	"net"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/capability"
	"github.com/CoscaAI/cosca/internal/config"
)

// capabilityLevelReport é o relatório estruturado de `cosca capability level`.
type capabilityLevelReport struct {
	Level       string   `json:"level"`
	Provider    string   `json:"provider"`
	HasRuntime  bool     `json:"has_runtime"`
	Description string   `json:"description"`
	Available   []string `json:"available"`
	Unavailable []string `json:"unavailable"`
}

// NewCapabilityCommand cria a árvore de comandos `cosca capability`.
func NewCapabilityCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "capability",
		Short: "Níveis de capacidade cognitiva (L0–L3)",
		Long: `Níveis de capacidade cognitiva da Cosca (L0–L3).

Cada nível expressa o que a Cosca consegue fazer com base no provider ativo:

  L0 — Determinístico   (sem IA: regras, estado, memória, knowledge, workflow, auditoria, segurança)
  L1 — Retrieval        (IA opcional: busca, classificação, matching semântico, extração)
  L2 — Raciocínio       (modelo disponível: análise, planejamento, inferência, código, diagnóstico)
  L3 — Execução autônoma (modelo + runtime: planeja → propõe → valida → executa → testa → observa → aprende)

Subcomandos:
  level   Mostra o nível atual, a descrição e as capacidades disponíveis/indisponíveis`,
		Example: `  cosca capability level
  cosca capability level --json`,
	}

	cmd.AddCommand(
		NewCapabilityLevelCommand(),
	)
	return cmd
}

// NewCapabilityLevelCommand cria `cosca capability level`.
func NewCapabilityLevelCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "level",
		Short: "Mostrar o nível de capacidade cognitiva atual",
		Long: `Detecta o provider ativo a partir da configuração (provider.primary /
provider.name / providers.primary) e imprime o nível de capacidade cognitiva
atual (L0–L3), a descrição e as capacidades disponíveis e indisponíveis.

Se o provider for "none" ou estiver vazio, o modo determinístico (L0) é
exibido claramente.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			provider := activeProviderFromConfig()
			hasRuntime := runtimeEnabledFromConfig()

			level := capability.CurrentLevel(provider, hasRuntime)
			report := capabilityLevelReport{
				Level:       level.String(),
				Provider:    provider,
				HasRuntime:  hasRuntime,
				Description: level.Description(),
				Available:   capability.Capabilities(level),
				Unavailable: capability.UnavailableCapabilities(level),
			}

			if useJSON {
				return printJSON(cmd, report)
			}

			formatter.Header("Nível de capacidade cognitiva")
			formatter.KeyValue("Nível", level.String())
			formatter.KeyValue("Provider ativo", provider)
			if hasRuntime {
				formatter.KeyValue("Runtime de execução", "habilitado")
			} else {
				formatter.KeyValue("Runtime de execução", "não habilitado")
			}
			if provider == "" || strings.ToLower(provider) == "none" {
				formatter.Warning("Provider ativo é \"none\"/vazio — operando em modo determinístico (sem IA).")
			}

			formatter.Println("")
			formatter.KeyValue("Descrição", level.Description())

			formatter.Header("Capacidades")
			rows := make([][]string, 0, len(capability.AllCapabilities()))
			for _, c := range capability.AllCapabilities() {
				rows = append(rows, []string{c, "✓ disponível"})
			}
			for _, c := range report.Unavailable {
				for i, row := range rows {
					if row[0] == c {
						rows[i][1] = "✗ indisponível"
					}
				}
			}
			formatter.Table([]string{"Capacidade", "Status"}, rows)

			return nil
		},
	}
}

// activeProviderFromConfig lê o provider ativo da configuração (viper),
// preferindo provider.primary e caindo para provider.name / providers.primary.
func activeProviderFromConfig() string {
	v := config.GetViper()
	if v == nil {
		return ""
	}
	for _, key := range []string{"provider.primary", "provider.name", "providers.primary"} {
		if name := v.GetString(key); name != "" {
			return name
		}
	}
	return ""
}

// runtimeEnabledFromConfig detecta se o runtime e o pipeline autônomo estão
// ativos. Estratégia em camadas:
//  1. Env vars (definitivo — setado pelo systemd, sobrevive ao jail)
//  2. TCP dial (live check — runtime :14123, serve :14120)
//  3. Viper config (útil em dev)
func runtimeEnabledFromConfig() bool {
	// Camada 1: variáveis de ambiente
	if os.Getenv("COSCA_RUNTIME_ENABLED") == "true" && os.Getenv("COSCA_PIPELINE_ENABLED") == "true" {
		return true
	}

	// Camada 2: live check via TCP nos ports padrão
	runtimeAlive := tcpDial("127.0.0.1:14123")
	pipelineAlive := tcpDial("127.0.0.1:14120")
	if runtimeAlive && pipelineAlive {
		return true
	}

	// Camada 3: viper config (útil em dev)
	v := config.GetViper()
	if v == nil {
		return false
	}
	hasRuntime := v.GetBool("runtime.enabled") || v.GetBool("execution.enabled")
	hasPipeline := v.GetBool("pipeline.enabled")
	return hasRuntime && hasPipeline
}

// tcpDial verifica se uma porta TCP está aceitando conexões (live check).
func tcpDial(addr string) bool {
	conn, err := (&net.Dialer{Timeout: 500 * time.Millisecond}).Dial("tcp", addr)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
