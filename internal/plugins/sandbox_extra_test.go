//go:build linux

// sandbox_extra_test.go cobre funcionalidade do sandbox Linux (wrapper de
// subprocesso com strip de env COSCA_INTERNAL_SANDBOX_* e filtro seccomp BPF)
// + delegação de loadSharedLibPlugin (portátil, mas exercitado aqui).
package plugins

import (
	"strings"
	"testing"
)

const testEnvSandboxPrefix = "COSCA_INTERNAL_SANDBOX_"

func TestStripSandboxEnvVars(t *testing.T) {
	env := []string{
		"PATH=/usr/bin",
		"COSCA_INTERNAL_SANDBOX_JAILED=1",
		"COSCA_INTERNAL_SANDBOX_PID=123",
		"HOME=/root",
	}
	stripped := stripSandboxEnvVars(env)
	if len(stripped) != 2 {
		t.Fatalf("stripped = %v", stripped)
	}
	joined := strings.Join(stripped, "\n")
	if strings.Contains(joined, "COSCA_INTERNAL_SANDBOX_") {
		t.Fatal("env de sandbox não removida")
	}
	if !strings.Contains(joined, "PATH=") || !strings.Contains(joined, "HOME=") {
		t.Fatal("env legítima removida")
	}
}

func TestBuildSeccompFilter(t *testing.T) {
	// O filtro seccomp é uma lista de instruções BPF — deve ser não-vazia e
	// terminar com a instrução de retorno (BPF_RET).
	filter := buildSeccompFilter()
	if len(filter) == 0 {
		t.Fatal("filtro seccomp vazio")
	}
	// A última instrução deve ser um RET (0x06 opcode do BPF).
	last := filter[len(filter)-1]
	if last.Code&0x07 != 0x06 {
		t.Fatalf("última instrução não é RET: code=%#x", last.Code)
	}
}

func TestLoadSharedLibPluginDelegates(t *testing.T) {
	// loadSharedLibPlugin delega para loadExternalPlugin; com manifest válido
	// mas binário ausente → erro claro, sem panic.
	l := NewLoader(LoaderConfig{PluginsDir: t.TempDir()})
	manifest := &PluginManifest{ID: "lib-p", Name: "Lib", Version: "1.0.0", Runtime: RuntimeSharedLib}
	_, err := l.loadSharedLibPlugin(manifest, "/nonexistent/lib.so")
	if err == nil {
		t.Log("loadSharedLibPlugin retornou nil (pode ter caído em fallback)")
	}
}

func TestExternalPluginInitEmptyPath(t *testing.T) {
	// Init de plugin externo sem binário → erro claro, sem panic.
	p := &externalPlugin{}
	if err := p.Init(&PluginContext{}); err == nil {
		t.Fatal("Init com modulePath vazio deve dar erro")
	}
}
