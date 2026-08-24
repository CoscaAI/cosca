// Package main is the entry point for the Cosca Enterprise Platform.
// It initializes the runtime environment, loads configuration, and
// executes the root command with graceful shutdown support.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/internal/cli"
	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/env"
	"github.com/CoscaAI/cosca/internal/hardening"
	"github.com/CoscaAI/cosca/internal/memoryintegrity"
	"github.com/CoscaAI/cosca/internal/providers/anthropic"
	"github.com/CoscaAI/cosca/internal/providers/azure"
	"github.com/CoscaAI/cosca/internal/providers/bedrock"
	"github.com/CoscaAI/cosca/internal/providers/deepseek"
	"github.com/CoscaAI/cosca/internal/providers/google"
	"github.com/CoscaAI/cosca/internal/providers/groq"
	"github.com/CoscaAI/cosca/internal/providers/local"
	"github.com/CoscaAI/cosca/internal/providers/mistral"
	"github.com/CoscaAI/cosca/internal/providers/ollama"
	"github.com/CoscaAI/cosca/internal/providers/openai"
	"github.com/CoscaAI/cosca/pkg/cosca"
)

func main() {
	// ----------------------------------------------------------------
	// 0. Process hardening (pre-tudo)
	// ----------------------------------------------------------------
	// Desativa core dumps (RLIMIT_CORE=0), desativa ptrace attach
	// (PR_SET_DUMPABLE=0), bloqueia escalonamento de privilegio via exec
	// (PR_SET_NO_NEW_PRIVS) e remove env vars perigosas (LD_PRELOAD,
	// LD_LIBRARY_PATH, DYLD_*). Best-effort — nunca falha. Aplicado ANTES
	// de qualquer leitura de config, segredo ou reexec da jaula.
	hardening.PreMain()

	// ----------------------------------------------------------------
	// 0.1 Inicializacao pre-jail
	// ----------------------------------------------------------------
	// Segredos COSCA_* (COSCA_JWT_SECRET, COSCA_METRICS_SECRET) são entregues
	// à jaula via /.cosca/jail-secrets.env (NUNCA por --setenv — ficaria no
	// argv, visível por ps/journald). Carrega-os assim que o processo entra
	// na jaula, ANTES de qualquer leitura de config. No-op fora da jaula.
	cosca.LoadJailSecrets()

	// Salva o hostname real antes de qualquer coisa. A crypto (AES-256-GCM)
	// usa COSCA_REAL_HOSTNAME para derivar a chave. Isso garante que a chave
	// seja a mesma dentro e fora da jaula (mesmo com --hostname cosca-jail).
	// Se ja estiver setado (ex: propagado pelo bwrap via --setenv), mantem.
	if os.Getenv("COSCA_REAL_HOSTNAME") == "" {
		if hostname, err := os.Hostname(); err == nil {
			os.Setenv("COSCA_REAL_HOSTNAME", hostname)
		}
	}

	// Carrega .env do diretorio do projeto. Dentro da jaula o workspace
	// e montado como "/" (--bind /workspace /), portanto o .env projetado
	// em /.env esta acessivel — sem ele, provider env vars (OLLAMA_HOST,
	// OPENAI_BASE_URL etc.) sao perdidos pelo --clearenv do bwrap.
	env.LoadDefault(true)

	// Nivel de log global ANTES de qualquer inicializacao de providers/config.
	// Sem isso, mensagens debug vazam para o stderr/TTY antes do initLogger()
	// rodar (ex: config.Load + registro de providers) — sujeira que polui o
	// terminal (TUI). Default e info; COSCA_DEV=true ou COSCA_LOG_LEVEL=debug
	// habilitam debug explicitamente.
	configureGlobalLogLevel()

	if err := enforceMemoryIntegrityGate(os.Args[1:]); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}

	// Carrega o config e propaga chaves de provider como variaveis de
	// ambiente. Roda FORA do bloco da jaula para funcionar nos 2 cenarios:
	// (a) dentro da jaula (COSCA_JAILED=1) — provider le via os.Getenv()
	// (b) fora da jaula — env vars sao herdadas pelo bwrap via os.Environ()
	propagateProviderEnv()

	// ----------------------------------------------------------------
	// 1. Auto-jail: reexecuta via bwrap se nao estiver ja dentro
	// ----------------------------------------------------------------
	// O binario no disco e 644 (chmod -x). So root executa via sudo.
	// O auto-jail le /proc/self/exe, cria copia em RAM (memfd ou /tmp),
	// e reexecuta dentro de uma bolha Bubblewrap com COSCA_JAILED=1.
	//
	// Excecoes: comandos administrativos que nao executam codigo de agente
	// (init, config) rodam fora da jaula. O workspace pode nao existir ainda
	// e o --unshare-user quebraria bind de diretorios do usuario.
	if !cosca.InsideJail() && !isAdminCommand() {
		cosca.ReexecInJail() // nao retorna
	}

	// ----------------------------------------------------------------
	// 2. Initialize embedding providers
	// ----------------------------------------------------------------
	initProviders()

	// ----------------------------------------------------------------
	// 3. Initialize build info
	// ----------------------------------------------------------------
	initBuildInfo()

	// ----------------------------------------------------------------
	// 4. Set up logging
	// ----------------------------------------------------------------
	logger := initLogger()
	log.Logger = logger

	// ----------------------------------------------------------------
	// 5. Create root context with cancellation
	// ----------------------------------------------------------------
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ----------------------------------------------------------------
	// 6. Build root command using the CLI package
	// ----------------------------------------------------------------
	rootCmd := cli.NewRootCommand()

	// Version info is sourced from pkg/cosca (single source of truth)
	rootCmd.Version = cosca.Version

	// ----------------------------------------------------------------
	// 7. Handle graceful shutdown on signals
	// ----------------------------------------------------------------
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		sig := <-sigCh
		log.Info().Str("signal", sig.String()).Msg("received signal, shutting down...")
		cancel()

		// Force exit after a grace period
		go func() {
			<-sigCh
			log.Warn().Msg("second signal received, forcing exit")
			os.Exit(1)
		}()

		// If root command doesn't exit within grace period, force exit
		if err := rootCmd.ExecuteContext(ctx); err != nil {
			log.Error().Err(err).Msg("shutdown error")
		}
		os.Exit(0)
	}()

	// ----------------------------------------------------------------
	// 8. Execute
	// ----------------------------------------------------------------
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		if code := cli.ExitCode(err); code != 0 {
			os.Exit(code)
		}
		// O erro é SEMPRE impresso de forma legível no stderr, independente
		// do nível de log ou do comportamento da jaula (que pode engolir o
		// stderr do processo interno). O log.Fatal preserva o registro
		// estruturado; a linha direta garante que o Don veja a falha.
		_, _ = fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		log.Fatal().Err(err).Msg("command execution failed")
		os.Exit(1)
	}
}

// initProviders registers embedding providers.
// By default, only local (Ollama + TF-IDF fallback) providers are active.
// External cloud providers (OpenAI, DeepSeek, Azure, Bedrock, Google, Mistral,
// Groq, Anthropic) are gated behind the COSCA_ENABLE_EXTERNAL_PROVIDERS
// environment variable. Set it to "1" or "true" to enable them.
func initProviders() {
	local.Register()
	ollama.Register()

	if os.Getenv("COSCA_ENABLE_EXTERNAL_PROVIDERS") == "1" ||
		strings.EqualFold(os.Getenv("COSCA_ENABLE_EXTERNAL_PROVIDERS"), "true") {
		openai.Register()
		anthropic.Register()
		deepseek.Register()
		google.Register()
		azure.Register()
		mistral.Register()
		groq.Register()
		bedrock.Register()
	}
}

// propagateProviderEnv loads the config from disk (decrypts the API key)
// and sets the corresponding environment variables. It runs both inside
// and outside the jail — essential for providers to read keys via
// os.Getenv() and for the jail (which strips non-COSCA_ env vars via
// --clearenv) to recover base URLs configured in .cosca/config.yaml.
func propagateProviderEnv() {
	cfg, err := config.Load()
	if err != nil {
		return // inacessivel — providers usam env vars .env ou manuais
	}

	// API key — always propagated from config (preferred over .env)
	if key := cfg.Provider.APIKey; key != "" {
		providerSetEnv(cfg.Provider.Name, key)
	}

	// Base URL — propagate mesmo quando nao ha API key (ex: Ollama
	// local nao usa chave, mas precisa de OLLAMA_HOST). O bwrap faz
	// --clearenv e perde a var; a config do projeto e a unica fonte
	// autoritativa dentro da jaula.
	if baseURL := cfg.Provider.BaseURL; baseURL != "" {
		providerSetBaseURL(cfg.Provider.Name, baseURL)
	}
	if baseURL := cfg.Embedding.BaseURL; baseURL != "" {
		providerSetBaseURL(cfg.Embedding.Provider, baseURL)
	}

	// Embedding provider key (pode ser diferente do chat provider)
	if key := cfg.Embedding.APIKey; key != "" {
		providerSetEnv(cfg.Embedding.Provider, key)
	}
}

// providerSetEnv sets the API key env var for a named provider.
func providerSetEnv(provider string, key string) {
	switch provider {
	case "deepseek":
		os.Setenv("DEEPSEEK_API_KEY", key)
	case "openai":
		os.Setenv("OPENAI_API_KEY", key)
	case "anthropic":
		os.Setenv("ANTHROPIC_API_KEY", key)
	case "google":
		os.Setenv("GOOGLE_API_KEY", key)
	case "mistral":
		os.Setenv("MISTRAL_API_KEY", key)
	case "groq":
		os.Setenv("GROQ_API_KEY", key)
	default:
		os.Setenv(strings.ToUpper(provider)+"_API_KEY", key)
	}
}

// providerSetBaseURL sets the base URL env var for a named provider.
func providerSetBaseURL(provider string, baseURL string) {
	switch provider {
	case "ollama":
		os.Setenv("OLLAMA_HOST", baseURL)
	case "openai":
		os.Setenv("OPENAI_BASE_URL", baseURL)
	case "deepseek":
		os.Setenv("DEEPSEEK_BASE_URL", baseURL)
	case "anthropic":
		os.Setenv("ANTHROPIC_BASE_URL", baseURL)
	case "azure":
		os.Setenv("AZURE_OPENAI_ENDPOINT", baseURL)
	case "mistral":
		os.Setenv("MISTRAL_BASE_URL", baseURL)
	case "groq":
		os.Setenv("GROQ_BASE_URL", baseURL)
	default:
		os.Setenv(strings.ToUpper(provider)+"_BASE_URL", baseURL)
	}
}

// isAdminCommand retorna true se o comando atual e administrativo
// (nao executa codigo de agente e nao precisa de jaula).
// Init cria diretorios, config gerencia arquivos, version so imprime.
// Hook roda fora da jaula porque: (a) e disparado pelo git em ambientes
// sem sudo/bwrap; (b) precisa que os.Executable() aponte para o binario
// persistente (nao para a copia em memfd da jaula) para o script instalado
// em .git/hooks/ continuar valido apos rebuilds; (c) so faz leituras git e
// escrita de markdown em .cosca/.
//
// terminal roda FORA da jaula (OpenCode mode) por qualidade de TUI: TTY
// limpo, cores, rede e zero ruido de debug do bwrap. O trade-off de seguranca
// e mitigado mantendo o sandbox POR-COMANDO para a execucao de ferramentas:
// cada comando arbitrario emitido pelo LLM roda dentro de bwrap (gate
// per-command, o mesmo modelo do chat), nunca como o proprio processo
// do terminal. Os daemons (serve/runtime) continuam presos na jaula.
func isAdminCommand() bool {
	if len(os.Args) < 2 {
		return false
	}
	switch os.Args[1] {
	case "init", "config", "version", "hook", "project", "asset", "task", "model", "models", "gpu", "ngraph", "render", "media", "flow", "provenance", "security", "terminal", "desktop", "voice", "slop",
		// Comandos de inspeção/diagnóstico (leitura pura, sem execução de código
		// de agente). Rodam FORA da jaula porque: (a) a jaula monta o workspace
		// em "/" (--bind <workspace> /), fazendo os.Getwd() retornar "/" em vez
		// do path real; (b) a jaula remapeia PIDs (--unshare-pid), fazendo
		// processAlive(pid-do-host) falhar; (c) o --clearenv derruba as env vars
		// de provider, deixando "Provider: " vazio. Para reportar a VERDADE do
		// host, esses comandos precisam enxergar o namespace real.
		"status", "doctor", "health", "capability", "fabric", "gate", "skill",
		// `cofre` — a fronteria de validação do Cofre (ADR-012). É um guard
		// PURAMENTE local (nunca abre a rede, não executa código de agente):
		// valida um SemanticPackage e devolve a DECISÃO do Oráculo. Precisa ser
		// comandável tanto pelo Kernel (submeter o pacote, fora da jaula) quanto
		// DENTRO da jaula. Fora da jaula, roda sem sandbox porque não executa
		// nada que exija isolamento — a validação é 100% determinística
		// (Gate.Evaluate). Dentro da jaula (InsideJail==true), o reexec é
		// ignorado e o cofre roda normalmente na zona Cofre.
		"db":
		// `cosca db check` — leitura pura da Decisão 1 (ADR-013): mede tamanho
		// de cada banco (.cosca/*.db) e reporta % do teto de 100MB. NUNCA escreve,
		// nunca executa código de agente. Roda fora da jaula pelos mesmos motivos
		// de cofre/status (enxergar o workspace real). Sem este admin, no Windows
		// cairia no fail-closed ReexecInJail exit 1.
		return true
	case "cofre":
		// BUGFIX: o bloco de comandos admin precisa retornar true EXPLICITAMENTE.
		// Sem este return, todos estes comandos (init/version/status/cofre...)
		// cairiam no ReexecInJail e passariam a rodar presos na jaula — no Linux
		// quebraria `cosca version`/`--help`/leitura, e no Windows cairiam no
		// fail-closed exit 1 sem nem conseguir operar. O isAdminCommand é o que
		// garante que comandos de leitura/admin rodem FORA da jaula (ver contrato
		// fail-closed de pkg/cosca).
		return true
	case "runtime":
		// Subcomandos de leitura/controle do daemon (status/stop/logs/info)
		// precisam ver o PID real do host e o workspace real — mesmos motivos
		// acima. start/restart sobem o daemon (que executa código de agente) e
		// permanecem na jaula.
		if len(os.Args) > 2 {
			switch os.Args[2] {
			case "status", "stop", "logs", "info":
				return true
			}
		}
	}
	// Integrity e register são operações de governança OFFLINE. Ficam FORA da
	// jaula: (a) integrity audita o manifesto; (b) register precisa ler a chave
	// privada do kernel (fora da jaula, ~/.config/cosca/keys) para o portão de
	// autorização — dentro da jaula a chave é inacessível e o portão negaria
	// até o próprio kernel. Nenhum dos dois executa código de agente.
	return os.Args[1] == "memory" && len(os.Args) > 2 && (os.Args[2] == "integrity" || os.Args[2] == "register")
}

func enforceMemoryIntegrityGate(args []string) error {
	if integrityGateBypass(args) {
		return nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("gate de integridade: não foi possível detectar a raiz do projeto: %w", err)
	}
	root, err := memoryintegrity.ProjectRoot(wd)
	if err != nil {
		return fmt.Errorf("gate de integridade: não foi possível detectar a raiz do projeto: %w", err)
	}

	// Projeto novo (sem .cosca/) não tem memória para verificar —
	// o init cria o manifesto, não deve ser bloqueado pela ausência dele.
	if _, statErr := os.Stat(filepath.Join(root, ".cosca")); os.IsNotExist(statErr) {
		return nil
	}

	// FIRST-BOOT (máquina nova): se o .cosca existe mas o manifest de
	// integridade nunca foi criado, estabelecemos a linha de base automaticamente
	// em vez de recusar o boot. É o equivalente seguro e idempotente de
	// `cosca memory integrity init` no primeiro boot: o snapshot atual vira a
	// verdade de referência, e QUALQUER alteração posterior é capturada pelo
	// verify (fail-closed preservado). A decisão é registrada em log para
	// evidência/auditoria.
	if _, statErr := os.Stat(memoryintegrity.DefaultManifestPath(root)); os.IsNotExist(statErr) {
		if _, initErr := memoryintegrity.Write(root, ""); initErr != nil {
			return fmt.Errorf("INÍCIO RECUSADO: não foi possível estabelecer a linha de base de integridade da memória no first boot (%v)", initErr)
		}
		log.Warn().Str("root", root).Msg("FIRST BOOT: manifest de integridade da memória criado automaticamente (linha de base). Execute `cosca memory integrity verify` para conferir.")
	}
	result, err := memoryintegrity.Verify(root, "")
	if err != nil {
		return fmt.Errorf("INÍCIO RECUSADO: a integridade da memória não foi verificada (%v). Execute explicitamente `cosca memory integrity init` (ou `cosca memory integrity verify`) e revise o resultado; nenhum runtime/memória foi carregado", err)
	}
	if len(result.Changed)+len(result.Missing)+len(result.Added) != 0 {
		return fmt.Errorf("INÍCIO RECUSADO: mismatch no manifesto de integridade da memória (alterados=%d, ausentes=%d, novos=%d). Execute explicitamente `cosca memory integrity verify`, revise e, se apropriado, `cosca memory integrity init --force`; nenhum runtime/memória foi carregado", len(result.Changed), len(result.Missing), len(result.Added))
	}
	return nil
}

func integrityGateBypass(args []string) bool {
	words := commandWords(args)
	if len(words) >= 1 {
		switch words[0] {
		case "init", "version", "models", "desktop", "terminal", "start", "gate", "skill":
			return true
		}
	}
	return len(words) >= 3 && words[0] == "memory" && words[1] == "integrity" && (words[2] == "init" || words[2] == "verify")
}

func commandWords(args []string) []string {
	var words []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			if arg == "--config" || arg == "--format" || arg == "--root" || arg == "--manifest" {
				i++
			}
			continue
		}
		words = append(words, arg)
	}
	return words
}

// initBuildInfo provides a last-resort fallback for CommitHash and BuildDate
// from the module build info (useful for `go run` / `go build` without
// ldflags). Values are written directly into pkg/cosca, the single source of
// truth for build metadata.
func initBuildInfo() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if cosca.CommitHash == "unknown" || cosca.CommitHash == "" {
				cosca.CommitHash = s.Value
			}
		case "vcs.time":
			if cosca.BuildDate == "unknown" || cosca.BuildDate == "" {
				cosca.BuildDate = s.Value
			}
		}
	}
}

// configureGlobalLogLevel sets the zerolog global threshold so no debug/trace
// messages reach the terminal before initLogger runs (or from components that
// write via the global logger). COSCA_DEV=true and COSCA_LOG_LEVEL=debug/trace
// are the explicit opt-ins for debug output.
func configureGlobalLogLevel() {
	if os.Getenv("COSCA_DEV") == "true" {
		return
	}
	levelStr := strings.ToLower(os.Getenv("COSCA_LOG_LEVEL"))
	level, err := zerolog.ParseLevel(levelStr)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
}

// initLogger configures zerolog based on the environment.
func initLogger() zerolog.Logger {
	output := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: "15:04:05",
		NoColor:    os.Getenv("NO_COLOR") != "",
	}

	// Default level: info
	levelStr := strings.ToLower(os.Getenv("COSCA_LOG_LEVEL"))
	level, err := zerolog.ParseLevel(levelStr)
	if err != nil {
		level = zerolog.InfoLevel
	}

	// In development mode, use pretty console output
	devMode := os.Getenv("COSCA_DEV") == "true"
	if devMode {
		level = zerolog.DebugLevel
	}

	return zerolog.New(output).
		Level(level).
		With().
		Timestamp().
		Str("app", "cosca").
		Str("version", cosca.Version).
		Logger()
}
