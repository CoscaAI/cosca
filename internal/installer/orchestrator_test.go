package installer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeCheck Ã© um Check determinÃ­stico para testes.
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

// fakePhases monta as fases de teste: preflight â†’ deps â†’ auth.
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

	rep, err := Run(dir, "test", fakePhases(), nil)
	require.NoError(t, err)
	// As fases vÃ£o atÃ© AUTH_READY â€” nÃ£o certificam ainda (faltam as fases
	// finais). O importante: o estado avanÃ§ou corretamente.
	require.Equal(t, StateAuthReady, rep.CurrentState)
	require.Len(t, rep.Steps, 3, "3 fases = 3 steps")

	// O git foi instalado (detect=Fail â†’ install â†’ validate).
	gitStep := rep.Steps[1]
	require.Equal(t, ResultPass, gitStep.Result)
	require.Equal(t, "install", gitStep.Action)
}

func TestRun_CertifiesWhenComplete(t *testing.T) {
	dir := t.TempDir()

	// Fases completas atÃ© CERTIFIED.
	phases := append(fakePhases(), Phase{
		ID:   "certify",
		Name: "Certification",
		Checks: []Check{
			&fakeCheck{id: "cosca.build", name: "Build", detectRes: ResultPass, validateRes: ResultPass},
		},
		NextState: StateCertified,
	})

	rep, err := Run(dir, "test", phases, nil)
	require.NoError(t, err)
	require.True(t, rep.Certified, "fase final deve certificar")
	require.Equal(t, StateCertified, rep.CurrentState)
}

func TestRun_IdempotentResume(t *testing.T) {
	dir := t.TempDir()

	// Primeira execuÃ§Ã£o completa (atÃ© AUTH_READY).
	rep, err := Run(dir, "test", fakePhases(), nil)
	require.NoError(t, err)
	require.Equal(t, StateAuthReady, rep.CurrentState)

	// Segunda execuÃ§Ã£o: tudo jÃ¡ completo â€” deve pular com PASS, sem reinstalar.
	rep2, err := Run(dir, "test", fakePhases(), nil)
	require.NoError(t, err)
	require.Equal(t, StateAuthReady, rep2.CurrentState)
	// Todos os steps sÃ£o "skip" (fase jÃ¡ completa).
	for _, s := range rep2.Steps {
		require.Equal(t, "skip", s.Action, "idempotÃªncia: nada re-executa")
		require.Equal(t, ResultPass, s.Result)
	}
}

func TestRun_FailureBlocks(t *testing.T) {
	dir := t.TempDir()

	// Git falha na instalaÃ§Ã£o â†’ a fase deps bloqueia.
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
	rep, err := Run(dir, "test", phases, nil)
	require.Error(t, err, "falha na instalaÃ§Ã£o deve bloquear")
	require.Equal(t, StateNotReady, rep.CurrentState, "estado nÃ£o avanÃ§a em falha")
}

func TestRun_PersistsState(t *testing.T) {
	dir := t.TempDir()

	// PersistÃªncia: o arquivo de evidÃªncia existe apÃ³s a execuÃ§Ã£o.
	_, err := Run(dir, "test", fakePhases(), nil)
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


