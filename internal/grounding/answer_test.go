// Package grounding tests — BuildAnswer ("sem evidência não gera") e a âncora
// anti-alucinação dos chunks injetados como [Chunk N]. Zero-LLM: a LLMFunc é
// sempre injetada pelo chamador (stub nos testes).
package grounding

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildAnswer_NoChunksNeverCallsLLM: sem chunks a resposta é extrativa e a
// LLMFunc NUNCA é chamada (o modelo não pode inventar de memória paramétrica).
func TestBuildAnswer_NoChunksNeverCallsLLM(t *testing.T) {
	t.Parallel()

	called := false
	llmFn := func(_ context.Context, _ string) (string, error) {
		called = true
		return "", nil
	}

	out, err := BuildAnswer(context.Background(), "q", nil, llmFn, BuildAnswerOptions{GroundingEnabled: true})
	require.NoError(t, err)
	assert.False(t, called, "sem chunks, llmFn não deve ser chamada")
	assert.Contains(t, out, "Sem evidência suficiente")
}

// TestBuildAnswer_NoChunksUngrounded: sem grounding o retorno extrativo é
// vazio (nada a juntar) e o LLM não é chamado.
func TestBuildAnswer_NoChunksUngrounded(t *testing.T) {
	t.Parallel()

	called := false
	llmFn := func(_ context.Context, _ string) (string, error) {
		called = true
		return "", nil
	}
	out, err := BuildAnswer(context.Background(), "q", nil, llmFn, BuildAnswerOptions{})
	require.NoError(t, err)
	assert.Equal(t, "", out)
	assert.False(t, called)
}

// TestBuildAnswer_WithChunksAndLLMInjectsChunkN: com chunks + llmFn, o prompt
// recebe os chunks como [Chunk N] e o LLM é chamado com ele.
func TestBuildAnswer_WithChunksAndLLMInjectsChunkN(t *testing.T) {
	t.Parallel()

	var gotPrompt string
	llmFn := func(_ context.Context, prompt string) (string, error) {
		gotPrompt = prompt
		return "  Resposta grounded.  ", nil
	}

	chunks := []SourceChunk{
		ch("c1", "O RAG reduz alucinação."),
		ch("c2", "As evidências sustentam a resposta."),
	}

	out, err := BuildAnswer(context.Background(), "o que o RAG faz?", chunks, llmFn, BuildAnswerOptions{})
	require.NoError(t, err)

	assert.Equal(t, "Resposta grounded.", out, "saída do LLM deve ser trimmed")
	assert.Contains(t, gotPrompt, "Pergunta: o que o RAG faz?")
	assert.Contains(t, gotPrompt, "[Chunk 1]")
	assert.Contains(t, gotPrompt, "O RAG reduz alucinação.")
	assert.Contains(t, gotPrompt, "[Chunk 2]")
	assert.Contains(t, gotPrompt, "APENAS com base nas evidências")

	// A ordem dos chunks deve ser preservada (1 antes de 2).
	assert.Less(t, strings.Index(gotPrompt, "[Chunk 1]"), strings.Index(gotPrompt, "[Chunk 2]"))
}

// TestBuildAnswer_EmptyLLMReturnsErrNoEvidence: chunk presente mas o LLM
// devolve vazio → recusa com ErrNoEvidence (não fabrica resposta).
func TestBuildAnswer_EmptyLLMReturnsErrNoEvidence(t *testing.T) {
	t.Parallel()

	llmFn := func(_ context.Context, _ string) (string, error) {
		return "", nil
	}

	_, err := BuildAnswer(context.Background(), "q", []SourceChunk{ch("c1", "evidência")}, llmFn, BuildAnswerOptions{})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoEvidence)

	whitespace := func(_ context.Context, _ string) (string, error) { return "   \n\t ", nil }
	_, err = BuildAnswer(context.Background(), "q", []SourceChunk{ch("c1", "evidência")}, whitespace, BuildAnswerOptions{})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoEvidence)
}

// TestBuildAnswer_LLMErrorWraps: o erro do LLM é preservado (wrapped) com um
// prefixo identificável.
func TestBuildAnswer_LLMErrorWraps(t *testing.T) {
	t.Parallel()

	llmFn := func(_ context.Context, _ string) (string, error) {
		return "", errors.New("provider unavailable")
	}

	_, err := BuildAnswer(context.Background(), "q", []SourceChunk{ch("c1", "evidência")}, llmFn, BuildAnswerOptions{})
	require.Error(t, err)
	assert.ErrorContains(t, err, "llm generate")
	assert.ErrorContains(t, err, "provider unavailable")
}

// TestBuildAnswer_ChunksWithoutLLMExtractive: chunks presentes e llmFn nil →
// resposta extrativa (join determinístico numerado).
func TestBuildAnswer_ChunksWithoutLLMExtractive(t *testing.T) {
	t.Parallel()

	chunks := []SourceChunk{ch("c1", "primeiro"), ch("c2", "segundo")}
	out, err := BuildAnswer(context.Background(), "q", chunks, nil, BuildAnswerOptions{})
	require.NoError(t, err)

	assert.Equal(t, "[Chunk 1]\nprimeiro\n\n[Chunk 2]\nsegundo", out)
}

// TestBuildAnswer_SingleChunkExtractive: um único chunk → extrate numerado
// sem separador de bloco extra.
func TestBuildAnswer_SingleChunkExtractive(t *testing.T) {
	t.Parallel()

	out, err := BuildAnswer(context.Background(), "q", []SourceChunk{ch("c1", "única evidência")}, nil, BuildAnswerOptions{})
	require.NoError(t, err)
	assert.Equal(t, "[Chunk 1]\núnica evidência", out)
}

// TestBuildAnswer_NoChunksNoEvidenceMessage: o placeholder explícito quando
// não há chunks e grounding ligado.
func TestBuildAnswer_NoChunksNoEvidenceMessage(t *testing.T) {
	t.Parallel()

	out, err := BuildAnswer(context.Background(), "q", []SourceChunk{}, nil, BuildAnswerOptions{GroundingEnabled: true})
	require.NoError(t, err)
	assert.Equal(t, "Sem evidência suficiente: não há chunks recuperados para sustentar uma resposta.", out)
}
