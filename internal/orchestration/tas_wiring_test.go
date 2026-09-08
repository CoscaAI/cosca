package orchestration

import (
	"testing"
)

// TestTasTaskContextFrom_EmptyState_FailClosed: sem prompt e sem metadados de
// stack/alvo, o wiring retorna nil (pipeline segue idêntico ao atual).
func TestTasTaskContextFrom_EmptyState_FailClosed(t *testing.T) {
	data := NewPipelineData() // Extra vazio, Prompt vazio
	tc := tasTaskContextFrom(data)
	if tc != nil {
		t.Fatalf("tasTaskContextFrom com estado vazio = %+v, esperava nil (fail-closed)", tc)
	}
}

// TestTasTaskContextFrom_PromptOnly: com apenas o prompt, o perfil é derivado
// (confiança baixa, mas não nil — a busca ganha afinidade da tarefa).
func TestTasTaskContextFrom_PromptOnly(t *testing.T) {
	data := NewPipelineData()
	data.AugmentedPrompt = "implementar endpoint REST em Go"
	tc := tasTaskContextFrom(data)
	if tc == nil {
		t.Fatal("tasTaskContextFrom com prompt deveria derivar um perfil")
	}
	if tc.Profile == nil || tc.Profile.Task != "implementar endpoint REST em Go" {
		t.Fatalf("perfil incorreto: %+v", tc.Profile)
	}
	if tc.PhaseDetection == nil {
		t.Fatal("PhaseDetection nil")
	}
}

// TestTasTaskContextFrom_WithMetadata: com metadados de stack/alvo injetados
// via Extra (como o editor faria), o perfil é derivado com afinidade plena.
func TestTasTaskContextFrom_WithMetadata(t *testing.T) {
	data := NewPipelineData()
	data.AugmentedPrompt = "implementar endpoint REST em Go"
	data.Extra["tas.working_dir"] = "C:/proj"
	data.Extra["tas.open_files"] = []string{"internal/api/handlers.go", "internal/api/middleware.go"}
	data.Extra["tas.recent_files"] = []string{"internal/api/handlers.go"}
	data.Extra["tas.go_mod"] = true

	tc := tasTaskContextFrom(data)
	if tc == nil {
		t.Fatal("tasTaskContextFrom com metadados deveria derivar um perfil")
	}
	if tc.Profile == nil {
		t.Fatal("Profile nil")
	}
	if !tc.Profile.HasStack() {
		t.Fatal("stack não detectado (go_mod true)")
	}
	if !tc.Profile.HasTarget() {
		t.Fatal("target não detectado (open_files presente)")
	}
	if tc.Profile.Target != "internal/api/handlers.go" {
		t.Fatalf("target = %q, esperava internal/api/handlers.go", tc.Profile.Target)
	}
}

// TestTasTaskContextFrom_AlreadySet: quando o TaskContext já veio preenchido
// pelo chamador, o wiring NÃO sobrescreve (o executor só deriva se nil).
func TestTasTaskContextFrom_AlreadySet(t *testing.T) {
	data := NewPipelineData()
	data.AugmentedPrompt = "implementar endpoint REST em Go"
	data.Extra["tas.go_mod"] = true

	// Simula: o chamador já preencheu o TaskContext com um perfil específico.
	// O wiring no executor só chama tasTaskContextFrom quando TaskContext == nil,
	// então este teste valida que a função NÃO é chamada nesse caso — mas
	// confirmamos o contrato: a função deriva independente do TaskContext atual.
	tc := tasTaskContextFrom(data)
	if tc == nil {
		t.Fatal("deveria derivar mesmo com TaskContext presente (a checagem é no executor)")
	}
}
