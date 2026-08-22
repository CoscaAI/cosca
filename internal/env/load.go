package env

import (
	"os"
	"path/filepath"
	"strings"
)

// LoadDefault carrega as variáveis de ambiente do serve.env GLOBAL
// (~/.config/cosca/serve.env — fora do workspace, fora da jaula) e depois do
// .env local do projeto. O segredo principal (JWT) vive no serve.env, fora
// do workspace — assim um agente dentro do runtime NÃO lê o segredo (a jaula
// só monta o workspace, não o home do usuário). O .env LOCAL sobrepõe o
// global (o local é mais específico).
func LoadDefault(overwrite bool) error {
	if home, err := os.UserHomeDir(); err == nil {
		_ = Load(filepath.Join(home, ".config", "cosca", "serve.env"), false)
	}
	// O .env local sempre sobrepõe o serve.env global.
	return Load(".env", true)
}

// Load reads a .env file at path and sets environment variables.
// If overwrite is true, existing variables are replaced.
// If overwrite is false, only unset variables are populated.
// It silently returns nil if the file does not exist.
func Load(path string, overwrite bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		v = strings.Trim(v, `"'`)
		if k == "" {
			continue
		}
		if overwrite {
			os.Setenv(k, v)
		} else if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
	return nil
}
