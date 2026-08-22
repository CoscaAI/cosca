//
// Tests for `cosca search cache` (internal/cli/search.go) — fingerprint cache
// de busca com TTL configurável.
//
// Cobre:
//   - Registro de `search cache` sob `cosca search` (não remove subcomandos)
//   - Propriedades do comando + subcomandos stats/clear/config
//   - `search cache config --ttl 10m` persiste o TTL em .cosca/config.yaml
//   - `search cache stats` imprime hits, misses, entries e TTL configurado
//   - `search cache clear` limpa o cache
//   - `search --ttl` (uso único) valida e executa via SearchWithPolicy
//
// NOTE: os testes fazem chdir() e NÃO rodam em paralelo. Formatter injetado
// via newContextWithFormatter (padrão do CLI).
//

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/config"
)

// chdirSearchCacheTemp muda para um diretório temporário (com .cosca/) e
// restaura o cwd no cleanup. Reseta globalFlags e searchTTL (variáveis de
// pacote compartilhadas entre testes).
func chdirSearchCacheTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	coscaDir := filepath.Join(dir, ".cosca")
	_ = os.MkdirAll(coscaDir, 0755)
	// Chdir registra o restore do cwd DEPOIS do RemoveAll do TempDir
	// (cleanups rodam em LIFO) — no Windows não dá para remover o
	// diretório que é o CWD do processo.
	t.Chdir(dir)
	globalFlags = GlobalFlags{}
	searchTTL = ""
	return dir
}

// runSearchCacheCmd executa o RunE de um subcomando com formatter injetado em
// um buffer (padrão de injeção de formatter do CLI).
func runSearchCacheCmd(t *testing.T, cmd *cobra.Command, args []string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true) // noColor
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	err := cmd.RunE(cmd, args)
	return buf.String(), err
}

// =============================================================================
// Registro — `search cache` sob `cosca search`
// =============================================================================

func TestSearchCacheCommand_RegisteredInRootAndSearch(t *testing.T) {
	root := NewRootCommand()
	var searchCmd *cobra.Command
	for _, sub := range root.Commands() {
		if sub.Name() == "search" {
			searchCmd = sub
			break
		}
	}
	if searchCmd == nil {
		t.Fatal("search subcommand not registered in root command")
	}

	found := false
	for _, sub := range searchCmd.Commands() {
		if sub.Name() == "cache" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("cache subcommand not registered under cosca search")
	}
}

func TestSearchCacheCommand_Properties(t *testing.T) {
	cmd := NewSearchCacheCommand()
	if cmd == nil {
		t.Fatal("NewSearchCacheCommand returned nil")
	}
	if cmd.Use != "cache" {
		t.Errorf("Use = %q, want 'cache'", cmd.Use)
	}
	if cmd.Short == "" || cmd.Long == "" {
		t.Error("Short/Long não podem ser vazios")
	}

	expected := []string{"stats", "clear", "config"}
	if len(cmd.Commands()) != len(expected) {
		t.Errorf("expected %d subcommands, got %d", len(expected), len(cmd.Commands()))
	}
	registered := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("missing search cache subcommand: %s", name)
		}
	}
}

func TestSearchCacheConfigCommand_HasTTLFlag(t *testing.T) {
	cmd := NewSearchCacheConfigCommand()
	flag := cmd.Flags().Lookup("ttl")
	if flag == nil {
		t.Fatal("flag --ttl ausente em search cache config")
	}
	if flag.DefValue != "" {
		t.Errorf("--ttl default deveria ser vazio, got %q", flag.DefValue)
	}
}

// =============================================================================
// `search cache config` — persiste o TTL em .cosca/config.yaml
// =============================================================================

func TestSearchCacheConfig_SetsPersistedTTL(t *testing.T) {
	chdirSearchCacheTemp(t)

	cmd := NewSearchCacheConfigCommand()
	_ = cmd.Flags().Set("ttl", "10m")
	out, err := runSearchCacheCmd(t, cmd, nil)
	if err != nil {
		t.Fatalf("search cache config: %v\n%s", err, out)
	}
	if !strings.Contains(out, "cache.ttl set to 10m0s") {
		t.Errorf("esperava mensagem de sucesso, got: %q", out)
	}

	// O TTL precisa estar persistido no arquivo de config do projeto.
	cfg, err := config.LoadFromFile(configPath())
	if err != nil {
		t.Fatalf("load persisted config: %v", err)
	}
	if cfg.Cache.TTL != 10*time.Minute {
		t.Errorf("persisted TTL = %v, want 10m", cfg.Cache.TTL)
	}
}

func TestSearchCacheConfig_ShowsPersistedTTL(t *testing.T) {
	chdirSearchCacheTemp(t)

	cfgCmd := NewSearchCacheConfigCommand()
	_ = cfgCmd.Flags().Set("ttl", "10m")
	if _, err := runSearchCacheCmd(t, cfgCmd, nil); err != nil {
		t.Fatalf("set ttl: %v", err)
	}

	out, err := runSearchCacheCmd(t, NewSearchCacheConfigCommand(), nil)
	if err != nil {
		t.Fatalf("search cache config: %v\n%s", err, out)
	}
	if !strings.Contains(out, "10m0s") {
		t.Errorf("esperava o TTL persistido no output, got: %q", out)
	}
}

func TestSearchCacheConfig_InvalidTTL(t *testing.T) {
	chdirSearchCacheTemp(t)

	cmd := NewSearchCacheConfigCommand()
	_ = cmd.Flags().Set("ttl", "bogus")
	_, err := runSearchCacheCmd(t, cmd, nil)
	if err == nil {
		t.Fatal("esperava erro para --ttl inválido")
	}
	if !strings.Contains(err.Error(), "invalid --ttl") {
		t.Errorf("esperava 'invalid --ttl', got: %v", err)
	}

	_ = cmd.Flags().Set("ttl", "-5m")
	_, err = runSearchCacheCmd(t, cmd, nil)
	if err == nil {
		t.Fatal("esperava erro para TTL negativo")
	}
}

// =============================================================================
// `search cache stats` — hits, misses, entries, TTL
// =============================================================================

func TestSearchCacheStats_Output(t *testing.T) {
	chdirSearchCacheTemp(t)

	// Persiste um TTL conhecido para a asserção ficar determinística.
	cfgCmd := NewSearchCacheConfigCommand()
	_ = cfgCmd.Flags().Set("ttl", "10m")
	if _, err := runSearchCacheCmd(t, cfgCmd, nil); err != nil {
		t.Fatalf("set ttl: %v", err)
	}

	out, err := runSearchCacheCmd(t, NewSearchCacheStatsCommand(), nil)
	if err != nil {
		t.Fatalf("search cache stats: %v\n%s", err, out)
	}

	for _, want := range []string{
		"Search Cache Statistics",
		"Hits",
		"Misses",
		"Entries",
		"TTL",
		"10m0s",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stats output deveria conter %q: %q", want, out)
		}
	}
}

func TestSearchCacheStats_JSON(t *testing.T) {
	chdirSearchCacheTemp(t)

	globalFlags.JSON = true
	defer func() { globalFlags.JSON = false }()

	out, err := runSearchCacheCmd(t, NewSearchCacheStatsCommand(), nil)
	if err != nil {
		t.Fatalf("search cache stats --json: %v\n%s", err, out)
	}

	for _, want := range []string{
		`"hits"`,
		`"misses"`,
		`"entries"`,
		`"ttl"`,
		`"enabled"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON deveria conter %q: %s", want, out)
		}
	}
}

// =============================================================================
// `search cache clear`
// =============================================================================

func TestSearchCacheClear_Output(t *testing.T) {
	chdirSearchCacheTemp(t)

	out, err := runSearchCacheCmd(t, NewSearchCacheClearCommand(), nil)
	if err != nil {
		t.Fatalf("search cache clear: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Search cache cleared") {
		t.Errorf("esperava confirmação de clear, got: %q", out)
	}
}

func TestSearchCacheClear_JSON(t *testing.T) {
	chdirSearchCacheTemp(t)

	globalFlags.JSON = true
	defer func() { globalFlags.JSON = false }()

	out, err := runSearchCacheCmd(t, NewSearchCacheClearCommand(), nil)
	if err != nil {
		t.Fatalf("search cache clear --json: %v\n%s", err, out)
	}
	if !strings.Contains(out, `"cleared": true`) {
		t.Errorf("JSON deveria conter cleared=true: %s", out)
	}
}

// =============================================================================
// `cosca search --ttl` — uso único (opt-in) via SearchWithPolicy
// =============================================================================

func TestSearchCommand_TTLFlag_InvalidValue(t *testing.T) {
	chdirSearchCacheTemp(t)

	cmd := NewSearchCommand()
	_ = cmd.Flags().Set("ttl", "bogus")
	_, err := runSearchCacheCmd(t, cmd, []string{"alpha"})
	if err == nil {
		t.Fatal("esperava erro para --ttl inválido no search")
	}
	if !strings.Contains(err.Error(), "invalid --ttl") {
		t.Errorf("esperava 'invalid --ttl', got: %v", err)
	}
}

func TestSearchCommand_TTLFlag_OneShotSearch(t *testing.T) {
	chdirSearchCacheTemp(t)
	seedLayeredDB(t)

	cmd := NewSearchCommand()
	_ = cmd.Flags().Set("ttl", "10m")
	out, err := runSearchCacheCmd(t, cmd, []string{"backup"})
	if err != nil {
		t.Fatalf("search --ttl: %v\n%s", err, out)
	}

	if !strings.Contains(out, "Search Results") {
		t.Errorf("esperava output de resultados, got: %q", out)
	}
	if !strings.Contains(out, "Backup Strategy") {
		t.Errorf("esperava documento semeado nos resultados, got: %q", out)
	}
}
