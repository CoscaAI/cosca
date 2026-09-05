package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/websearch"
)

// webQuery é a entrada do comando `cosca web`.
var webLimit int

// NewWebCommand cria o comando `cosca web` — busca de informação na web via
// fontes públicas legítimas, SEM API key e SEM custo.
//
// Caso de uso (Don, 2026-09-02): investigar a realidade — "saiu uma notícia, é
// verdade ou fake?", "que problemas as pessoas enfrentam com X?", resolver com
// capacidade de raciocínio. Cada provider é uma camada de evidência; o operador
// (agente/humano) triangula as fontes.
func NewWebCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "web [provider] <query>",
		Short: "Busca informação na web via fontes públicas gratuitas (sem API key)",
		Long: `Busca informação na web usando fontes públicas legítimas — sem chave,
sem custo, sem ser barrado.

Providers (camadas de investigação):
  news     Notícias via RSS de agências (G1, UOL, BBC, Reuters)
  wiki     Contexto/verificação via Wikipedia (idioma: --lang)
  hn       O que a comunidade tech debate (Hacker News)
  github   Problemas/soluções reais de software (repos, issues, code)

Exemplos:
  cosca web news "estouro de montanha"        Notícia recente sobre o tema
  cosca web wiki "inteligência artificial"    Contexto enciclopédico
  cosca web hn "sqlite vector search"         Debate da comunidade
  cosca web github issues "flaky test"        Problemas que devs enfrentam
  cosca web github repos "go web search"      Ferramentas/tools existentes

Saída em JSON com --json para consumo por agentes. O conteúdo vindo da web é
dado NÃO-confiável — use como evidência, não como fato.`,
		Example: `  cosca web news "noticia"
  cosca web wiki "tema" --lang en
  cosca web hn "termo"
  cosca web github issues "problema"
  cosca web github repos "tool"
  cosca web --json news "noticia"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			if len(args) == 1 {
				return fmt.Errorf("invalid: `cosca web` requer provider + query. Ex: cosca web news \"query\". Veja cosca web --help")
			}

			providerName := strings.ToLower(args[0])
			query := strings.Join(args[1:], " ")
			lang, _ := cmd.Flags().GetString("lang")

			// Seleciona o provider pelo nome. Desconhecido → erro claro.
			provider, err := webProviderFor(providerName, lang)
			if err != nil {
				return err
			}

			// Feedback animado: spinner com a descrição do que está fazendo.
			// (só em modo texto — no JSON não há terminal).
			var spinner *Spinner
			spinnerLabel := webSpinnerLabel(providerName)
			if !useJSON && !globalFlags.Quiet {
				spinner = formatter.Spinner(spinnerLabel)
				spinner.Start()
			}

			// Timeout de contexto para a busca web nunca travar indefinidamente.
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			results, err := provider.Search(ctx, query, webLimit)
			if spinner != nil {
				if err != nil {
					spinner.Fail("busca falhou")
				} else {
					spinner.Stop(fmt.Sprintf("%d resultado(s) em %s", len(results), providerDisplayName(providerName)))
				}
			}
			if err != nil {
				return fmt.Errorf("%s: %w", provider.Name(), err)
			}

			if useJSON {
				return printJSON(cmd, map[string]any{
					"provider": provider.Name(),
					"query":    query,
					"count":    len(results),
					"results":  results,
				})
			}

			formatter.Header(fmt.Sprintf("%s — %q (%d)", providerDisplayName(providerName), query, len(results)))
			// Resumo do que foi buscado (uma linha por fonte, quando há).
			if sources := distinctSources(results); len(sources) > 0 {
				for _, s := range sources {
					formatter.Println("   fonte: " + s)
				}
				formatter.Println("")
			}
			if len(results) == 0 {
				formatter.Warning("Nenhum resultado encontrado.")
				return nil
			}
			for i, r := range results {
				formatter.Bullet(fmt.Sprintf("[%d] %s", i+1, r.Title))
				if r.Snippet != "" {
					formatter.Println("      " + r.Snippet)
				}
				if r.Link != "" {
					formatter.Println("      " + r.Link)
				}
				if r.Source != "" && providerName != "news" {
					formatter.Println("      fonte: " + r.Source)
				}
				formatter.Println("")
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&webLimit, "limit", "l", 5, "Máximo de resultados")
	cmd.Flags().String("lang", "pt", "Idioma da Wikipedia (ex: pt, en)")
	return cmd
}

// webProviderFor resolve o nome do provider para a implementação concreta.
// Retrocompatível e explícito: um nome inválido retorna erro, nunca silencioso.
func webProviderFor(name, lang string) (websearch.Provider, error) {
	switch name {
	case "news":
		return websearch.NewNewsProvider(), nil
	case "wiki":
		return websearch.NewWikiProvider().WithLang(lang), nil
	case "hn":
		return websearch.NewHNProvider(), nil
	case "github":
		// `cosca web github` sozinho busca repos. Use o subcomando de tipo
		// via query prefixada (ex: "github issues <q>") — ver nota abaixo.
		return websearch.NewGitHubProvider("repos"), nil
	case "github-issues", "issues":
		return websearch.NewGitHubProvider("issues"), nil
	case "github-code", "code":
		return websearch.NewGitHubProvider("codes"), nil
	default:
		return nil, fmt.Errorf("provider desconhecido: %q (disponíveis: news, wiki, hn, github, github-issues, github-code)", name)
	}
}

// webSpinnerLabel devolve a descrição amigável do que o spinner está fazendo.
// É o feedback "o que estou buscando agora" para o usuário/operador.
func webSpinnerLabel(providerName string) string {
	switch providerName {
	case "news":
		return "Buscando notícias recentes via RSS das agências…"
	case "wiki":
		return "Consultando a Wikipedia para contexto/verificação…"
	case "hn":
		return "Varrendo o Hacker News — o que a comunidade debate…"
	case "github-issues", "issues":
		return "Buscando problemas reais no GitHub (issues)…"
	case "github-code", "code":
		return "Buscando código no GitHub…"
	case "github":
		return "Buscando repositórios/projetos no GitHub…"
	default:
		return "Buscando informação na web…"
	}
}

// providerDisplayName devolve o nome legível do provider para o cabeçalho.
func providerDisplayName(providerName string) string {
	switch providerName {
	case "news":
		return "Notícias (RSS)"
	case "wiki":
		return "Wikipedia"
	case "hn":
		return "Hacker News"
	case "github-issues", "issues":
		return "GitHub Issues"
	case "github-code", "code":
		return "GitHub Code"
	case "github":
		return "GitHub Repos"
	default:
		return providerName
	}
}

// distinctSources devolve as fontes distintas (sem duplicar) presentes nos
// resultados — um resumo de onde a informação veio.
func distinctSources(results []websearch.Result) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range results {
		if r.Source == "" || seen[r.Source] {
			continue
		}
		seen[r.Source] = true
		out = append(out, r.Source)
	}
	return out
}
