// Coverage — F4 do ADR-019. MÉTRICAS SOBRE o grafo, nunca misturadas aos FATOS.
//
// Um grafo de código é "heuristic v1, structural precision, not AST" e é
// fail-open: um arquivo que não é lido/parseado é PULADO (nunca fatal). A
// epistemologia do Cosca (I3/I4) exige que essa omissão seja EXPLÍCITA e
// separada dos fatos: "não registrado ≠ inexistente". Coverage guarda a
// contabilidade (quantos arquivos-fonte existiam, quantos foram indexados,
// quantos pulados, por linguagem) — best-effort, nunca uma declaração de
// completude.
//
// Regra: CONSULTE os fatos no grafo (nós/arestas); CONSULTE a cobertura aqui.
// As duas coisas são campos distintos de Index — nunca se sobrepõem.
package codegraph

import "time"

// Coverage é a contabilidade do índice, separada dos fatos do grafo.
type Coverage struct {
	// TotalSourceFiles é o número de arquivos-fonte (por extensão conhecida)
	// encontrados no diretório.
	TotalSourceFiles int `json:"total_source_files"`
	// IndexedFiles é quantos tiveram sinais computados (no grafo).
	IndexedFiles int `json:"indexed_files"`
	// SkippedFiles é quantos foram encontrados mas não indexados (ilegível, erro,
	// etc.) — a omissão é registrada, não silenciada.
	SkippedFiles int `json:"skipped_files"`
	// Langs é o total de arquivos-fonte por linguagem.
	Langs map[string]int `json:"langs"`
	// BestEffort é SEMPRE true: o índice é best-effort (heurístico v1), nunca
	// afirma completude. "No recorded gap ≠ completeness guarantee" (I4).
	BestEffort bool `json:"best_effort"`
	// CoversAll true somente se não houve arquivo pulado.
	CoversAll bool `json:"covers_all"`
	// BuiltAt quando a cobertura foi produzida.
	BuiltAt time.Time `json:"built_at"`
}
