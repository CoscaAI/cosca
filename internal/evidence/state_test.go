package evidence

import "testing"

// TestObservationStatesCanonical valida os 6 estados canônicos e que Valid
// reconhece cada um.
func TestObservationStatesCanonical(t *testing.T) {
	states := []ObservationState{
		ObservationFound, ObservationNotFound, ObservationBlocked,
		ObservationInconclusive, ObservationSkipped, ObservationError,
	}
	for _, s := range states {
		if !s.Valid() {
			t.Errorf("estado %q deveria ser válido", s)
		}
		if s.String() == "" {
			t.Errorf("estado %q sem String()", s)
		}
	}
	if ObservationState("junk").Valid() {
		t.Error("estado inválido 'junk' não deveria ser válido")
	}
	if (ObservationState("junk")).String() != "junk" {
		t.Errorf("String() de estado inválido = %q", ObservationState("junk").String())
	}
}

// TestIsAbsence_OnlyProvenAbsence é o LEMA CENTRAL do ADR-037: apenas
// ObservationNotFound é ausência. Blocked/Inconclusive NÃO são ausência.
func TestIsAbsence_OnlyProvenAbsence(t *testing.T) {
	cases := map[ObservationState]bool{
		ObservationFound:        false,
		ObservationNotFound:     true,  // única ausência válida
		ObservationBlocked:      false, // recusou ≠ ausência
		ObservationInconclusive: false, // soft-404 ≠ ausência
		ObservationSkipped:      false,
		ObservationError:        false,
	}
	for s, want := range cases {
		if got := s.IsAbsence(); got != want {
			t.Errorf("IsAbsence(%q) = %v, esperava %v", s, got, want)
		}
	}
}

// TestIsUncertain valida que Blocked e Inconclusive são "incognoscíveis", não
// um veredito sobre o mundo.
func TestIsUncertain(t *testing.T) {
	for _, s := range []ObservationState{ObservationBlocked, ObservationInconclusive} {
		if !s.IsUncertain() {
			t.Errorf("IsUncertain(%q) = false, esperava true", s)
		}
	}
	for _, s := range []ObservationState{ObservationFound, ObservationNotFound, ObservationSkipped, ObservationError} {
		if s.IsUncertain() {
			t.Errorf("IsUncertain(%q) = true, esperava false", s)
		}
	}
}

// TestIsConfident valida que apenas observações diretas são confiáveis.
func TestIsConfident(t *testing.T) {
	for _, s := range []ObservationState{ObservationFound, ObservationNotFound} {
		if !s.IsConfident() {
			t.Errorf("IsConfident(%q) = false", s)
		}
	}
	for _, s := range []ObservationState{ObservationBlocked, ObservationInconclusive, ObservationSkipped, ObservationError} {
		if s.IsConfident() {
			t.Errorf("IsConfident(%q) = true", s)
		}
	}
}

// TestObservation_Validate verifica a integridade: estado canônico e, para
// ObservationError, motivo explícito presente (não-engolimento implícito).
func TestObservation_Validate(t *testing.T) {
	if err := (Observation{State: "junk"}).Validate(); err == nil {
		t.Error("estado inválido deveria dar erro")
	}
	if err := (Observation{State: ObservationError}).Validate(); err == nil {
		t.Error("ObservationError sem Reason deveria dar erro")
	}
	if err := (Observation{State: ObservationError, Reason: "servidor 503"}).Validate(); err != nil {
		t.Errorf("ObservationError com Reason deveria ser válido: %v", err)
	}
	if err := (Observation{State: ObservationNotFound, Reason: "busca exaustiva"}).Validate(); err != nil {
		t.Errorf("ObservationNotFound deveria ser válido: %v", err)
	}
}

// TestCalibrate_TwoControls aplica o princípio dos DOIS controles: qualquer
// controle aleatório "encontrado" → a observação Found vira Inconclusive.
func TestCalibrate_TwoControls(t *testing.T) {
	if got := Calibrate(ObservationFound, 0); got != ObservationFound {
		t.Errorf("Calibrate(Found, 0) = %q, esperava Found", got)
	}
	if got := Calibrate(ObservationFound, 1); got != ObservationInconclusive {
		t.Errorf("Calibrate(Found, 1) = %q, esperava Inconclusive (soft-404)", got)
	}
	// Dois controles, qualquer um dispara → Inconclusive (robustez).
	if got := Calibrate(ObservationFound, 2); got != ObservationInconclusive {
		t.Errorf("Calibrate(Found, 2) = %q, esperava Inconclusive", got)
	}
	// Para estados não-otimistas, Calibrate NÃO fabrica Inconclusive nem Found.
	if got := Calibrate(ObservationNotFound, 1); got != ObservationNotFound {
		t.Errorf("Calibrate(NotFound, 1) = %q, esperava NotFound", got)
	}
	if got := Calibrate(ObservationBlocked, 1); got != ObservationBlocked {
		t.Errorf("Calibrate(Blocked, 1) = %q, esperava Blocked", got)
	}
}

// TestNeverCollapseBlockedToAbsence é o guarda-corpo: um estado Blocked não pode
// ser reportado como ausência e não pode ser tratado como encontrado/confiável.
func TestNeverCollapseBlockedToAbsence(t *testing.T) {
	blocked := Observation{State: ObservationBlocked, Reason: "HTTP 403"}
	if blocked.State.IsAbsence() {
		t.Fatal("Blocked não pode ser ausência")
	}
	if blocked.State.IsConfident() {
		t.Fatal("Blocked não é uma observação confiável (não estabeleceu o mundo)")
	}
	if !blocked.State.IsUncertain() {
		t.Fatal("Blocked deve ser Incognoscível/Uncertain")
	}
}
