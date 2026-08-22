package cli

import (
	"context"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/providers/bootstrap"
)

// ollamaBootHint performs a side-effect-free detection of the local Ollama
// provider and returns its report. It is only meaningful when the CONFIGURED
// provider is ollama (the chat registry always succeeds for ollama — its
// factory needs no API key — so registry.Name() can never tell us the daemon
// is down). When the configured provider is not ollama it returns nil.
//
// Detection only: DetectOnly=true forces install/start/pull/validate to be
// skipped, so `cosca run` / `cosca serve` never mutate the machine. Automatic
// bootstrap stays an explicit user action: `cosca provider bootstrap
// --install --pull`.
func ollamaBootHint(ctx context.Context, projectCfg *config.Config) *bootstrap.Report {
	if projectCfg == nil {
		c, err := config.Load()
		if err != nil {
			log.Debug().Err(err).Msg("ollama boot hint: config load failed")
			return nil
		}
		projectCfg = c
	}
	if !strings.EqualFold(projectCfg.Provider.Name, "ollama") {
		return nil
	}

	cfg := bootstrap.Config{
		Model:      projectCfg.Provider.Model,
		BaseURL:    projectCfg.Provider.BaseURL,
		DetectOnly: true,
	}
	if cfg.Model == "" {
		cfg.Model = config.DefaultProviderModel
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = bootstrap.DefaultBaseURL
	}

	report, err := bootstrap.EnsureOllama(ctx, cfg)
	if err != nil {
		log.Warn().Err(err).Msg("ollama bootstrap detection failed")
		return nil
	}
	return report
}
