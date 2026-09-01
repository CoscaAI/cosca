package installer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeCheck é um Check determinístico para testes.
type fakeCheck struct {
	id         string
	name       string
	detectRes  Result
	installErr error
	validateRes Result
}

func (f *fakeCheck) ID() string                { return f.id }
func (f *fakeCheck) Name() string              { return f.name }
func (f *fakeCheck) Detect() (Result, []string) { return f.detectRes, []string{"detected"} }
func (f *fakeCheck) Install() error            { return f.installErr }
func (f *fakeCheck) Validate() (Result, []string) {
	return f.validateRes, []string{"validated"}
}

// fakePhases monta as fases de teste: preflight → deps → auth.
func fakePhases() []Phase {
	return []Phase{
		{
			ID:   "preflight",
			Name: "Preflight",
			Checks: []Check{
				&fakeCheck{id: "system.os", name: "OS", detectRes: ResultPass, validateRes: ResultPass},
			},
			NextState: StatePreflightOK,
		},
		{
			ID:   "dependencies",
			Name: "Dependencies",
			Checks: []Check{
				&fakeCheck{id: "git.installed", name: "Git", detectRes: ResultFail, installErr: nil, validateRes: ResultPass},
			},
			NextState: StateDepsReady,
		},
		{
			ID:   "auth",
			Name: "Auth",
			Checks: []Check{
				&fakeCheck{id: "github.auth", name: "GitHub", detectRes: ResultPass, validateRes: ResultPass},
			},
			NextState: StateAuthReady,
		},
	}
}

func TestRun_CompleteFlow(t *testing.T) {
	dir := t.TempDir()

	rep, err := Run(dir, "test", fakePhases())
	require.NoError(t, err)
	// As fases vão até AUTH_READY — não certificam ainda (faltam as fases
	// finais). O importante: o estado avançou corretamente.
	require.Equal(t, StateAuthReady, rep.CurrentState)
	require.Len(t, rep.Steps, 3, "3 fases = 3 steps")

	// O git foi instalado (detect=Fail → install → validate).
	gitStep := rep.Steps[1]
	require.Equal(t, ResultPass, gitStep.Result)
	require.Equal(t, "install", gitStep.Action)
}

func TestRun_CertifiesWhenComplete(t *testing.T) {
	dir := t.TempDir()

	// Fases completas até CERTIFIED.
	phases := append(fakePhases(), Phase{
		ID:   "certify",
		Name: "Certification",
		Checks: []Check{
			&fakeCheck{id: "cosca.build", name: "Build", detectRes: ResultPass, validateRes: ResultPass},
		},
		NextState: StateCertified,
	})

	rep, err := Run(dir, "test", phases)
	require.NoError(t, err)
	require.True(t, rep.Certified, "fase final deve certificar")
	require.Equal(t, StateCertified, rep.CurrentState)
}

func TestRun_IdempotentResume(t *testing.T) {
	dir := t.TempDir()

	// Primeira execução completa (até AUTH_READY).
	rep, err := Run(dir, "test", fakePhases())
	require.NoError(t, err)
	require.Equal(t, StateAuthReady, rep.CurrentState)

	// Segunda execução: tudo já completo — deve pular com PASS, sem reinstalar.
	rep2, err := Run(dir, "test", fakePhases())
	require.NoError(t, err)
	require.Equal(t, StateAuthReady, rep2.CurrentState)
	// Todos os steps são "skip" (fase já completa).
	for _, s := range rep2.Steps {
		require.Equal(t, "skip", s.Action, "idempotência: nada re-executa")
		require.Equal(t, ResultPass, s.Result)
	}
}

func TestRun_FailureBlocks(t *testing.T) {
	dir := t.TempDir()

	// Git falha na instalação → a fase deps bloqueia.
	phases := []Phase{
		{
			ID:   "dependencies",
			Name: "Dependencies",
			Checks: []Check{
				&fakeCheck{id: "git.installed", name: "Git", detectRes: ResultFail, installErr: os.ErrNotExist, validateRes: ResultPass},
			},
			NextState: StateDepsReady,
		},
	}
	rep, err := Run(dir, "test", phases)
	require.Error(t, err, "falha na instalação deve bloquear")
	require.Equal(t, StateNotReady, rep.CurrentState, "estado não avança em falha")
}

func TestRun_PersistsState(t *testing.T) {
	dir := t.TempDir()

	// Persistência: o arquivo de evidência existe após a execução.
	_, err := Run(dir, "test", fakePhases())
	require.NoError(t, err)

	path := filepath.Join(PersistDir(dir), "installation.json")
	_, err = os.Stat(path)
	require.NoError(t, err, "installation.json deve existir")

	// E o LoadState recupera o estado.
	require.Equal(t, StateAuthReady, LoadState(dir))
}

func TestStepResult_Evidence(t *testing.T) {
	step := NewStep("github.repository_access", "verify", ResultPass, StateAuthReady,
		"repository reachable", "branch verified")
	require.Equal(t, "github.repository_access", step.Check)
	require.Len(t, step.Evidence, 2)
	require.Equal(t, StateAuthReady, step.State)
}
