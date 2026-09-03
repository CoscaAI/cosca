package datasetgen

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ─── Conversão para o formato de fine-tune (ChatML) ─────────────────────────
//
// O dataset do datasetgen usa o formato internal (Example: task + initial_state
// + trajectory). Para treinar um LoRA (unsloth / llamafactory / transformers
// SFTTrainer), o formato esperado é o CHAT ML JSONL: uma lista de mensagens
// {role, content} por exemplo, incluindo a chamada de ferramenta e o resultado.
//
// Este conversor produz um JSONL compatível com SFT (supervised fine-tuning):
//   - o "user" carrega a task + estado inicial,
//   - o "assistant" carrega a trajetória (chamadas de ferramenta + texto)
//     como conteúdo textual no formato tool_call do modelo.
//
// Motivação (professor): destilar o COMPORTAMENTO OPERACIONAL (seguir o
// protocolo read→edit→verify→recover), não "programação". O alvo do primeiro
// LoRA é recovery + resolução, mantendo read→edit=1.00 e 0 violações críticas.

// ChatMessage é uma mensagem no formato ChatML (unsloth/SFT).
type ChatMessage struct {
	Role    string `json:"role"`    // system | user | assistant
	Content string `json:"content"`
}

// SFTExample é um exemplo no formato de fine-tune supervisionado.
type SFTExample struct {
	Messages []ChatMessage `json:"messages"`
}

// ToSFTFormat converte um Example (positivo) no formato SFT ChatML.
// Só usa exemplos POSITIVOS (SUCCESS/RECOVERY_SUCCESS) como demonstrações;
// os contrastes NÃO são demonstrações — seriam material de preferência (DPO),
// não de SFT.
func (e *Example) ToSFTFormat() (*SFTExample, error) {
	if !e.Label.IsPositive() {
		return nil, fmt.Errorf("exemplo %s não é positivo (label=%s) — SFT usa apenas demonstrações", e.Focus, e.Label)
	}

	var msgs []ChatMessage

	// System: o contrato operacional (o que o modelo precisa seguir).
	sys := "You are a coding agent in a workspace. You MUST use tools to accomplish the task. "
	sys += "Always read_file before edit_file. old_string must be an exact substring you observed. "
	sys += "If a tool errors, read again and retry. Execute by calling tools, not describing."
	msgs = append(msgs, ChatMessage{Role: "system", Content: sys})

	// User: a task + estado inicial (se houver).
	user := e.Task
	if len(e.InitialState) > 0 {
		user += "\n\nInitial workspace:"
		for path, content := range e.InitialState {
			user += fmt.Sprintf("\n--- %s ---\n%s", path, content)
		}
	}
	msgs = append(msgs, ChatMessage{Role: "user", Content: user})

	// Assistant: a trajetória (cada passo de tool-call convertido para texto).
	// Apenas os passos de assistant com tool-calls (ou a resposta final).
	for _, step := range e.Trajectory {
		if step.Role != "assistant" {
			continue
		}
		content := step.Content
		for _, tc := range step.ToolCalls {
			if content != "" {
				content += "\n"
			}
			content += fmt.Sprintf("Call %s(%s)", tc.Name, string(tc.Arguments))
		}
		if strings.TrimSpace(content) != "" {
			msgs = append(msgs, ChatMessage{Role: "assistant", Content: content})
		}
	}

	return &SFTExample{Messages: msgs}, nil
}

// ConvertDatasetToSFT lê um JSONL de Examples (datasetgen), filtra os
// positivos e escreve um JSONL no formato SFT ChatML.
func ConvertDatasetToSFT(inPath, outPath string) (int, error) {
	data, err := os.ReadFile(inPath)
	if err != nil {
		return 0, err
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var out []string
	pos := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var ex Example
		if err := json.Unmarshal([]byte(line), &ex); err != nil {
			continue // pula linhas malformadas
		}
		if !ex.Label.IsPositive() {
			continue // só demonstrações positivas no SFT
		}
		sft, err := ex.ToSFTFormat()
		if err != nil {
			continue
		}
		b, err := json.Marshal(sft)
		if err != nil {
			continue
		}
		out = append(out, string(b))
		pos++
	}

	if err := os.WriteFile(outPath, []byte(strings.Join(out, "\n")+"\n"), 0o644); err != nil {
		return pos, err
	}
	return pos, nil
}

// WriteSFTBatches divide o dataset SFT em arquivos de treino/validação.
// ratioVal: proporção (0-1) para validação. Default se 0: 10%.
func WriteSFTBatches(sftPath string, ratioVal float64) (trainPath, valPath string, err error) {
	if ratioVal <= 0 || ratioVal >= 1 {
		ratioVal = 0.1
	}
	data, err := os.ReadFile(sftPath)
	if err != nil {
		return "", "", err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		return "", "", fmt.Errorf("dataset SFT vazio")
	}

	valCount := int(float64(len(lines)) * ratioVal)
	if valCount < 1 {
		valCount = 1
	}
	if valCount >= len(lines) {
		valCount = len(lines) / 2
	}

	trainPath = sftPath + ".train.jsonl"
	valPath = sftPath + ".val.jsonl"

	if err := os.WriteFile(trainPath, []byte(strings.Join(lines[:len(lines)-valCount], "\n")+"\n"), 0o644); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(valPath, []byte(strings.Join(lines[len(lines)-valCount:], "\n")+"\n"), 0o644); err != nil {
		return "", "", err
	}
	return trainPath, valPath, nil
}
