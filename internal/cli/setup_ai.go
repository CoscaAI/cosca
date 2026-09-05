// Package cli — Fase 4 do COSCA Environment Provisioner: AI Provisioning.
//
// Provisiona o stack de embeddings: provider (ollama) + modelo
// (nomic-embed-text, 768-dim) com um SMOKE TEST real — o conceito do
// professor: "o instalador não verifica apenas se o arquivo existe; verifica
// se o sistema realmente funciona".
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/CoscaAI/cosca/internal/installer"
	"github.com/CoscaAI/cosca/internal/providers/ollama"
)

// embedModel é o modelo de embedding canônico do COSCA (config do projeto).
const embedModel = "nomic-embed-text"

// ollamaBaseURL é o endpoint local do Ollama.
const ollamaBaseURL = "http://127.0.0.1:11434"

// aiProvisionCheck provisiona o stack de embeddings com smoke test real.
type aiProvisionCheck struct{}

func (a *aiProvisionCheck) ID() string   { return "ai.embedding" }
func (a *aiProvisionCheck) Name() string { return "AI / Embedding" }

// Detect verifica se o Ollama está de pé e o modelo está disponível.
func (a *aiProvisionCheck) Detect() (installer.Result, []string) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(ollamaBaseURL + "/api/tags")
	if err != nil {
		return installer.ResultFail, []string{"ollama não responde em " + ollamaBaseURL}
	}
	defer resp.Body.Close()

	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return installer.ResultFail, []string{"ollama respondeu mas resposta ilegível: " + err.Error()}
	}
	for _, m := range tags.Models {
		if strings.HasPrefix(m.Name, embedModel) {
			return installer.ResultPass, []string{
				"ollama respondendo em " + ollamaBaseURL,
				"modelo " + embedModel + " disponível",
			}
		}
	}
	return installer.ResultRebuildRequired, []string{
		"ollama ok, mas modelo " + embedModel + " NÃO está baixado",
	}
}

// Install baixa o modelo via `ollama pull` (subprocesso).
func (a *aiProvisionCheck) Install() error {
	out, err := runCmdOutput("ollama", "pull", embedModel)
	if err != nil {
		return fmt.Errorf("ollama pull %s falhou: %s: %w", embedModel, out, err)
	}
	return nil
}

// Validate faz o SMOKE TEST real: gera um embedding e prova a dimensão.
func (a *aiProvisionCheck) Validate() (installer.Result, []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Usa o registry de embeddings do COSCA (a capability real). Registra o
	// provider ollama explicitamente (o mesmo que a inicialização do binário
	// faz) para garantir que o smoke test não dependa de init de outro pacote.
	reg := embeddings.GetRegistry()
	ollama.Register()
	if err := reg.Select(ctx, embeddings.ProviderRegistryConfig{
		Primary: "ollama",
		Model:   embedModel,
	}); err != nil {
		return installer.ResultFail, []string{"seleção do provider falhou: " + err.Error()}
	}

	start := time.Now()
	res, err := reg.GenerateEmbedding(ctx, "The COSCA system is operational.")
	latency := time.Since(start)
	if err != nil {
		return installer.ResultFail, []string{"smoke test de embedding falhou: " + err.Error()}
	}

	dim := len(res.Vector)
	ev := []string{
		fmt.Sprintf("model: %s", res.Model),
		fmt.Sprintf("dimensions: %d", dim),
		fmt.Sprintf("latency: %d ms", latency.Milliseconds()),
	}
	if dim <= 0 {
		return installer.ResultFail, append(ev, "dimensão inesperada (0)")
	}
	return installer.ResultPass, append(ev, "embedding gerado — SMOKE TEST PASS")
}

// runCmdOutput roda um comando e captura a saída combinada (string).
func runCmdOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
