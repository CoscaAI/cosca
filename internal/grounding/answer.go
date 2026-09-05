package grounding

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrNoEvidence é devolvido pelo BuildAnswer quando a função generativa
// retorna vazio mesmo com chunks presentes — "recusa gerar se nada": nunca
// fabricar uma resposta que as evidências não sustentam.
var ErrNoEvidence = errors.New("grounding: sem evidência suficiente para gerar uma resposta")

// LLMFunc é a assinatura da função generativa injetada no BuildAnswer. Recebe
// o contexto e o prompt montado (query + chunks como [Chunk N]) e devolve a
// resposta. O BuildAnswer nunca chama esta função sem chunks.
type LLMFunc func(ctx context.Context, prompt string) (string, error)

// BuildAnswerOptions controla a construção da resposta grounded.
type BuildAnswerOptions struct {
	// GroundingEnabled liga a proteção "sem evidência não gera". false =
	// comportamento mais permissivo (ainda assim extrativo se não houver
	// chunks, por segurança).
	GroundingEnabled bool
}

// BuildAnswer monta uma resposta a partir dos chunks recuperados, garantindo
// o piso de anti-alucinação:
//
//   - len(chunks) == 0 → "SEM EVIDÊNCIA NÃO GERA": devolve uma resposta
//     EXTRATIVA (join determinístico dos chunks — que aqui é vazio → uma
//     resposta degradada explícita). NUNCA chama o LLM: o modelo não pode
//     inventar a partir de conhecimento paramétrico.
//   - chunks presentes e llmFn fornecido → injeta os chunks como "[Chunk N]"
//     em um prompt grounded, chama llmFn e devolve a resposta. Se a função
//     devolver vazio (nada), recusa com ErrNoEvidence.
//   - chunks presentes e llmFn nil → resposta extrativa (join dos chunks).
//
// A função é determinística no que depende do chamador; a saída do LLM é por
// natureza não-determinística, mas o contrato de "só gera com evidência" é.
func BuildAnswer(ctx context.Context, query string, chunks []SourceChunk, llmFn LLMFunc, opts BuildAnswerOptions) (string, error) {
	if len(chunks) == 0 {
		// "Sem evidência não gera": resposta extrativa/degradada, nunca LLM.
		return extractiveAnswer(chunks, opts.GroundingEnabled), nil
	}

	if llmFn == nil {
		return extractiveAnswer(chunks, opts.GroundingEnabled), nil
	}

	prompt := buildGroundedPrompt(query, chunks)
	out, err := llmFn(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("grounding: llm generate: %w", err)
	}
	if strings.TrimSpace(out) == "" {
		return "", ErrNoEvidence
	}
	return strings.TrimSpace(out), nil
}

// buildGroundedPrompt injeta os chunks como [Chunk N] e instrui o modelo a
// responder APENAS com base neles — a âncora anti-alucinação.
func buildGroundedPrompt(query string, chunks []SourceChunk) string {
	var b strings.Builder
	b.WriteString("Pergunta: ")
	b.WriteString(strings.TrimSpace(query))
	b.WriteString("\n\nEvidências recuperadas (chunks):\n")
	for i, ch := range chunks {
		fmt.Fprintf(&b, "\n[Chunk %d]\n%s\n", i+1, ch.Content)
	}
	b.WriteString("\nResponda APENAS com base nas evidências acima, citando o ")
	b.WriteString("[Chunk N] de onde cada afirmação veio. Se as evidências não ")
	b.WriteString("sustentarem a resposta, diga explicitamente que não há evidência ")
	b.WriteString("suficiente — não invente.")
	return b.String()
}

// extractiveAnswer monta a resposta extrativa: um join determinístico dos
// conteúdos dos chunks, numerados como [Chunk N]. É a resposta degradada
// usada quando (a) não há chunks ou (b) não há função generativa.
func extractiveAnswer(chunks []SourceChunk, grounded bool) string {
	if len(chunks) == 0 {
		if grounded {
			return "Sem evidência suficiente: não há chunks recuperados para sustentar uma resposta."
		}
		return ""
	}
	var b strings.Builder
	for i, ch := range chunks {
		fmt.Fprintf(&b, "[Chunk %d]\n", i+1)
		b.WriteString(strings.TrimSpace(ch.Content))
		if i < len(chunks)-1 {
			b.WriteString("\n\n")
		}
	}
	return b.String()
}
