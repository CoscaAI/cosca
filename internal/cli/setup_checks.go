// Package cli — Checks reais do COSCA Environment Provisioner.
//
// Cada Check implementa a interface installer.Check: Detect (sem instalar) →
// Install (se precisar) → Validate (prova que funciona). Estes são os
// PRIMEIROS checks reais — orquestram capabilities já existentes do COSCA.
package cli

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/CoscaAI/cosca/internal/installer"
)

// toolCheck verifica a presença de uma ferramenta no PATH (git, go, ollama).
// Detect: procura no PATH. Install: não instala (as ferramentas são externas;
// o instalador orienta o usuário). Validate: roda `tool --version`.
type toolCheck struct {
	id   string
	name string
	cmd  string
}

func (t *toolCheck) ID() string   { return t.id }
func (t *toolCheck) Name() string { return t.name }

func (t *toolCheck) Detect() (installer.Result, []string) {
	if _, err := exec.LookPath(t.cmd); err != nil {
		return installer.ResultFail, []string{t.cmd + " não encontrado no PATH"}
	}
	return installer.ResultPass, []string{t.cmd + " encontrado no PATH"}
}

func (t *toolCheck) Install() error {
	// Ferramentas externas (git/go/ollama) NÃO são instaladas pelo COSCA —
	// a instalação é feita pelo usuário (o instalador orienta). Retornar
	// erro aqui bloqueia a fase com orientação clara.
	return fmt.Errorf("%s não está instalado — instale manualmente (o instalador não baixa ferramentas de sistema)", t.name)
}

func (t *toolCheck) Validate() (installer.Result, []string) {
	// Prova que a ferramenta FUNCIONA (não só que existe): roda --version.
	out, err := exec.Command(t.cmd, "--version").CombinedOutput()
	if err != nil {
		return installer.ResultFail, []string{t.cmd + " falhou ao executar: " + err.Error()}
	}
	return installer.ResultPass, []string{t.cmd + " --version: " + firstLine(string(out))}
}

// machineProfileCheck valida a máquina (preflight): sistema operacional,
// arquitetura, CPU/memória detectáveis via runtime. Aprofunda com o
// capability profile do COSCA (cosca machine) em fases futuras.
type machineProfileCheck struct{}

func (m *machineProfileCheck) ID() string   { return "machine.profile" }
func (m *machineProfileCheck) Name() string { return "Machine Profile" }

func (m *machineProfileCheck) Detect() (installer.Result, []string) {
	// Detecção determinística via runtime (sem instalar nada).
	ev := []string{
		"os: " + runtime.GOOS,
		"arch: " + runtime.GOARCH,
	}
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return installer.ResultFail, append(ev, "SO não suportado: "+runtime.GOOS)
	}
	return installer.ResultPass, ev
}

func (m *machineProfileCheck) Install() error {
	// Não há instalação para o perfil de máquina — é detecção.
	return nil
}

func (m *machineProfileCheck) Validate() (installer.Result, []string) {
	// Valida que o binário roda neste SO/arquitetura (o próprio processo é a
	// prova — se chegou aqui, executa).
	return installer.ResultPass, []string{
		"binário executa em " + runtime.GOOS + "/" + runtime.GOARCH,
	}
}

// firstLine devolve a primeira linha de uma string (para evidência curta).
func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' || s[i] == '\r' {
			return s[:i]
		}
	}
	return s
}
