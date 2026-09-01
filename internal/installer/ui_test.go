package installer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestUIEmitter_ReceivesEvents valida o contrato da UI: o Emitter do modelo
// recebe os eventos reais do orquestrador (a UI não finge — consome o stream).
func TestUIEmitter_ReceivesEvents(t *testing.T) {
	dir := t.TempDir()
	model := NewUIModel()
	emit, ch := model.Emitter()

	// Roda o provisioner com o emit do modelo (fases fake completas).
	phases := fakePhases()
	rep, err := Run(dir, "test", phases, emit)
	require.NoError(t, err)
	require.Equal(t, StateAuthReady, rep.CurrentState)

	// O canal deve ter recebido eventos reais (step_started/complete/state).
	got := 0
	for {
		select {
		case msg := <-ch:
			if e, ok := msg.(Event); ok {
				got++
				require.NotEmpty(t, e.Type, "evento deve ter tipo")
			}
		default:
			goto done
		}
	}
done:
	require.Greater(t, got, 0, "a UI deve receber eventos reais do provisioner")
}

// TestUIModel_ApplyEvent_Progress valida que o modelo renderiza o estado.
func TestUIModel_ApplyEvent_Progress(t *testing.T) {
	m := NewUIModel()

	m.applyEvent(Event{Type: EventStateChanged, FromState: StateNotReady, State: StatePreflightOK})
	require.Equal(t, StatePreflightOK, m.state)
	require.Greater(t, m.progress, 0)

	m.applyEvent(Event{Type: EventCertified, State: StateCertified})
	require.True(t, m.certified)
	require.Equal(t, 100, m.progress)

	// A View renderiza sem panic (o que a TUI mostra).
	v := m.View()
	require.Contains(t, v, "COSCA")
}
