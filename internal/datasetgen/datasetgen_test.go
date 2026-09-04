package datasetgen

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// ─── Testes do procedural (determinístico) ──────────────────────────────────

// TestGenerateBatchDistribution valida que o lote cobre todos os focos com
// distribuição razoável (indicativo, não exato).
func TestGenerateBatchDistribution(t *testing.T) {
	specs := GenerateBatch(100, 42)
	if len(specs) != 100 {
		t.Fatalf("esperado 100 specs, got %d", len(specs))
	}

	byFocus := map[Focus]int{}
	languages := map[string]bool{}
	for _, s := range specs {
		byFocus[s.Focus]++
		languages[s.Language] = true
	}

	// Cobertura dos focos principais.
	for _, f := range []Focus{FocusHappyPath, FocusRecovery, FocusRequestInfo, FocusSearchFirst} {
		if byFocus[f] == 0 {
			t.Errorf("foco %s não gerado", f)
		}
	}

	// Diversidade de linguagens (a regra do professor: variar superfície).
	if len(languages) < 4 {
		t.Errorf("diversidade de linguagens baixa: %d (%v)", len(languages), languages)
	}

	t.Logf("distribuição: %v | linguagens: %v", byFocus, languages)
}

// TestClassifyLabels valida o classificador com trajetórias sintéticas.
func TestClassifyLabels(t *testing.T) {
	cfg := DefaultGeneratorConfig()
	r := NewRunner(cfg)

	cases := []struct {
		name string
		ex   *Example
		want Label
	}{
		{
			name: "prose instead of action",
			ex: &Example{
				Trajectory: []TrajectoryStep{
					{Role: "user", Content: "fix it"},
					{Role: "assistant", Content: "Vou alterar o arquivo para adicionar o handler."},
				},
				ExpectedState: map[string]string{"main.go": "func Restart() error {"},
			},
			want: LabelProseInsteadOfAction,
		},
		{
			name: "success after edit",
			ex: &Example{
				Trajectory: []TrajectoryStep{
					{Role: "user", Content: "fix it"},
					{Role: "assistant", Content: "", ToolCalls: []ToolCallJSON{{Name: "read_file", Arguments: []byte(`{"path":"main.go"}`)}}},
					{Role: "tool", Content: "package main\nfunc Restart() {"},
					{Role: "assistant", Content: "", ToolCalls: []ToolCallJSON{{Name: "edit_file", Arguments: []byte(`{"path":"main.go","old_string":"func Restart() {","new_string":"func Restart() error {"}`)}}},
					{Role: "tool", Content: "OK"},
				},
				ExpectedState: map[string]string{"main.go": "func Restart() error {"},
			},
			want: LabelSuccess,
		},
		{
			name: "recovery success",
			ex: &Example{
				Trajectory: []TrajectoryStep{
					{Role: "user", Content: "fix it"},
					{Role: "assistant", Content: "", ToolCalls: []ToolCallJSON{{Name: "edit_file", Arguments: []byte(`{"path":"main.go","old_string":"INVENTADO","new_string":"X"}`)}}},
					{Role: "tool", Content: "ERROR: edit_file requer read_file antes (EDIT_REQUIRES_READ)"},
					{Role: "assistant", Content: "", ToolCalls: []ToolCallJSON{{Name: "read_file", Arguments: []byte(`{"path":"main.go"}`)}}},
					{Role: "tool", Content: "package main\nfunc Restart() {"},
					{Role: "assistant", Content: "", ToolCalls: []ToolCallJSON{{Name: "edit_file", Arguments: []byte(`{"path":"main.go","old_string":"func Restart() {","new_string":"func Restart() error {"}`)}}},
					{Role: "tool", Content: "OK"},
				},
				ExpectedState: map[string]string{"main.go": "func Restart() error {"},
			},
			want: LabelRecoverySuccess,
		},
		{
			name: "failure no recovery",
			ex: &Example{
				Trajectory: []TrajectoryStep{
					{Role: "user", Content: "fix it"},
					{Role: "assistant", Content: "", ToolCalls: []ToolCallJSON{{Name: "edit_file", Arguments: []byte(`{"path":"main.go","old_string":"INVENTADO","new_string":"X"}`)}}},
					{Role: "tool", Content: "ERROR: old_string nao encontrado no snapshot"},
					{Role: "assistant", Content: "desisto"},
				},
				ExpectedState: map[string]string{"main.go": "func Restart() error {"},
			},
			want: LabelFailure,
		},
		{
			name: "false completion",
			ex: &Example{
				Trajectory: []TrajectoryStep{
					{Role: "user", Content: "fix it"},
					{Role: "assistant", Content: "", ToolCalls: []ToolCallJSON{{Name: "read_file", Arguments: []byte(`{"path":"main.go"}`)}}},
					{Role: "tool", Content: "package main\nfunc Restart() {"},
					{Role: "assistant", Content: "Pronto, concluí a tarefa."},
				},
				ExpectedState: map[string]string{"main.go": "func Restart() error {"},
			},
			want: LabelFalseCompletion,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Grava no disco o conteúdo do ExpectedState do caso, simulando o
			// resultado da edição/trajetória. Só assim expectedReached lê algo
			// coerente com o que a trajetória supostamente produziu.
			spec := TaskSpec{
				Task:          tc.ex.Task,
				InitialState:  map[string]string{"main.go": "package main\nfunc Restart() {\n}\n"},
				ExpectedState: tc.ex.ExpectedState,
			}
			ws, err := r.newWorkspace(spec)
			if err != nil {
				t.Fatalf("newWorkspace: %v", err)
			}
			defer os.RemoveAll(ws)

			// Para casos de sucesso/recuperação, grava o conteúdo esperado no
			// disco (simula a edição). Para casos de FALHA/prosa/false_completion,
			// deixamos o initial_state original (que NÃO contém o needle) para
			// que expectedReached seja falso e o classificador avalie a falha.
			for path, needle := range tc.ex.ExpectedState {
				full := ws + "/" + path
				switch tc.want {
				case LabelSuccess, LabelRecoverySuccess:
					// simula o resultado da edição: grava o conteúdo esperado.
					if err := os.WriteFile(full, []byte(needle+"\n"), 0o644); err != nil {
						t.Fatalf("write expected: %v", err)
					}
				default:
					// falha: mantém o initial_state (sem o needle).
					if err := os.WriteFile(full, []byte("func Restart() {\n}\n"), 0o644); err != nil {
						t.Fatalf("write initial: %v", err)
					}
				}
			}

			got := r.classify(context.Background(), ws, tc.ex)
			if got != tc.want {
				t.Fatalf("classify = %s, want %s", got, tc.want)
			}
		})
	}
}

// TestGenerateExampleE2E executa UM exemplo real contra o Ollama (opcional).
// Só roda quando COSCA_DATASETGEN_RUN_EVAL=1 (para não depender do modelo no
// CI de rotina). Requer Ollama rodando localmente.
func TestGenerateExampleE2E(t *testing.T) {
	if os.Getenv("COSCA_DATASETGEN_RUN_EVAL") != "1" {
		t.Skip("pule (defina COSCA_DATASETGEN_RUN_EVAL=1 para rodar o E2E)")
	}

	cfg := DefaultGeneratorConfig()
	cfg.Model = "qwen3:4b"
	cfg.Timeout = 180 * time.Second
	r := NewRunner(cfg)

	specs := GenerateBatch(5, 7)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	for i, spec := range specs {
		ex, err := r.GenerateExample(ctx, spec)
		if err != nil {
			t.Fatalf("exemplo %d erro: %v", i, err)
		}
		t.Logf("exemplo %d: %s | label=%s tools=%v", i, spec.Focus, ex.Label, ex.ToolNames)
		if ex.Trajectory == nil || len(ex.Trajectory) < 2 {
			t.Errorf("exemplo %d: trajetória curta (%d)", i, len(ex.Trajectory))
		}
	}
}

// TestToSFTFormat valida que um exemplo positivo vira o formato SFT ChatML.
func TestToSFTFormat(t *testing.T) {
	ex := &Example{
		Task: "Fix main.go", Focus: FocusHappyPath,
		Label: LabelSuccess,
		InitialState: map[string]string{"main.go": "package main\nfunc Restart() {"},
		Trajectory: []TrajectoryStep{
			{Role: "user", Content: "Fix main.go"},
			{Role: "assistant", ToolCalls: []ToolCallJSON{{Name: "read_file", Arguments: []byte(`{"path":"main.go"}`)}}},
			{Role: "tool", Content: "package main\nfunc Restart() {"},
			{Role: "assistant", Content: "", ToolCalls: []ToolCallJSON{{Name: "edit_file", Arguments: []byte(`{"path":"main.go","old_string":"func Restart() {","new_string":"func Restart() error {"}`)}}},
			{Role: "tool", Content: "OK"},
			{Role: "assistant", Content: "Done."},
		},
	}
	sft, err := ex.ToSFTFormat()
	if err != nil {
		t.Fatalf("ToSFTFormat: %v", err)
	}
	if len(sft.Messages) == 0 {
		t.Fatal("sem mensagens no SFT")
	}
	// system primeiro
	if sft.Messages[0].Role != "system" {
		t.Errorf("primeira role = %s, esperado system", sft.Messages[0].Role)
	}
	// deve conter a chamada de read_file codificada no assistant
	hasRead := false
	for _, m := range sft.Messages {
		if strings.Contains(m.Content, "read_file") {
			hasRead = true
		}
	}
	if !hasRead {
		t.Error("trajetória de read_file não codificada no conteúdo do assistant")
	}
	t.Logf("SFT messages: %d", len(sft.Messages))
}

func TestToSFTFormatRejectsContrast(t *testing.T) {
	ex := &Example{Task: "x", Focus: FocusHappyPath, Label: LabelFailure}
	if _, err := ex.ToSFTFormat(); err == nil {
		t.Fatal("contraste NÃO deveria converter (SFT usa só demonstrações)")
	}
}
