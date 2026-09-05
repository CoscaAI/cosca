package grounding

// ── GABARITO REAL (FASE 0 da campanha "monolithic vs modular") ─────────────
//
// O baseline congelado de recall era FICTÍCIO ("chunk-france-1" etc.) — não
// ligado a nenhum ID real do corpus — e por isso recall@K media 0. Este arquivo
// materializa o ground-truth REAL, extraído do benchmark pré-registrado
// `internal/search/vectorevidence_bench_test.go` (benchmarkQueries, v3 §10.3).
//
// FONTE DA VERDADE (somente leitura, nunca re-derivada da busca):
//   - "internal/search/vectorevidence_bench_test.go" → benchmarkQueries[].gt
//     (id = document.id; chunkID = chunk.id == vectors.id; module = segmento do
//     path; expectedPath = referência legível do documento).
//
// O ground-truth foi PRÉ-registrado ANTES de qualquer busca — não-circular. As
// queries expressam o CONTEÚDO real do chunk e os IDs foram fixados antes de
// qualquer execução do retriever.
//
// `RealBenchmarkQrels` devolve a lista de gabaritos no formato do Qrels; use-a
// para regenerar `testdata/qrels-baseline.json` (ou um baseline em outro caminho)
// de forma determinística. A assinatura de embedding é a do corpus
// (nomic-embed-text:768).

// realBenchmarkGT é o ground-truth registrado de uma query do benchmark.
type realBenchmarkGT struct {
	query        string
	docID        string // document_id real no corpus (RECALL_DOCUMENT).
	chunkID      string // chunk_id == vectors.id real (RECALL_CHUNK, o âncora).
	relevantIDs  []string // conjunto de chunks relevantes do doc (recall por região, não 1 agulha).
	module       string // domínio (segmento do path) onde a evidência mora.
	expectedPath string // referência de path legível (documentação).
}

// realBenchmarkSet é o query set v3 pré-registrado, na íntegra, com as 6 queries
// e seus ground-truths. A ordem é a do benchmark original.
var realBenchmarkSet = []realBenchmarkGT{
	{
		query:        "hot reload atualiza arquivos markdown sem reiniciar o runtime",
		docID:        "2f27dc0a-15f9-4086-8b65-cf015cc35399",
		chunkID:      "47659d61-351f-4d91-9889-d8fc5f31e643",
		relevantIDs:  []string{"47659d61-351f-4d91-9889-d8fc5f31e643", "b7783a8a-1baf-4eb2-9ca8-dccde79d65b7", "c6890d0e-a5df-4be3-9e89-827b2c09b1ae"},
		module:       "runtime",
		expectedPath: `internal\embed\cosca\runtime\HOT_RELOAD.md`,
	},
	{
		query:        "continuidade da memoria do kernel entre execucoes",
		docID:        "efd5b2c8-fd54-4243-aa92-07d74f97db41",
		chunkID:      "e84c5a50-0c7a-47ff-b54a-9bb92c760657",
		relevantIDs:  []string{"e84c5a50-0c7a-47ff-b54a-9bb92c760657", "a883ce3a-bc5f-43ed-92cc-fd5424a508ce", "27be6e41-c6a4-44e2-8a86-3ec36be8e93c", "c2b6d6e2-cb82-4376-892f-5fd9cf74c180", "89c9e139-5c01-4bcb-a6e9-8a6d3db2a550"},
		module:       "memory",
		expectedPath: `internal\embed\cosca\memory\agent\cosca-kernel\archive\L001-L214.md`,
	},
	{
		query:        "como funciona o motor de busca hibrida com fts5 e busca vetorial",
		docID:        "aff80ddc-f855-4dd8-a0e0-7ac39f2a03bf",
		chunkID:      "abc97c26-456f-405f-add7-1e6fd159e519",
		relevantIDs:  []string{"abc97c26-456f-405f-add7-1e6fd159e519", "99f8492d-2b9e-4e4e-9cbe-5b41559b194f", "66e5e707-3cc6-489e-be59-8fd4248591d5"},
		module:       "knowledge",
		expectedPath: `docs\knowledge\overview.md`,
	},
	{
		query:        "como o compute fabric gerencia os worker pools por load",
		docID:        "6a03e0f7-43be-43eb-b3fe-1093b4b6c48a",
		chunkID:      "93758a21-1ca2-45b5-a000-4da525c961c5",
		relevantIDs:  []string{"93758a21-1ca2-45b5-a000-4da525c961c5", "3cce6e59-42cf-4100-a87f-eaaf807ab837", "de30586f-daeb-451f-b25a-c2bb13d6e39a", "a08279c6-2a50-47d5-8b9e-00548c7ac2fc"},
		module:       "architecture",
		expectedPath: `docs\architecture\fabric-tuning.md`,
	},
	{
		query:        "como rodar benchmark de performance no cosca",
		docID:        "98877065-b3cd-43a1-9ce0-c383bcc2c42a",
		chunkID:      "6a5911f3-f055-4d12-9d7d-439aa97a9c0f",
		relevantIDs:  []string{"6a5911f3-f055-4d12-9d7d-439aa97a9c0f", "4c62fb72-1c4c-4295-b707-25d221195beb", "5db1ec24-961c-4126-8a5e-913082e2751b"},
		module:       "cli",
		expectedPath: `docs\cli\commands.md`,
	},
	{
		query:        "maquina de estados de seguranca valida autoriza executa verifica audita",
		docID:        "2e89bfb6-a751-4bd7-8653-ba3828b2fbb1",
		chunkID:      "06d4d3f4-e0fb-4af4-be50-4a05d29719fd",
		relevantIDs:  []string{"06d4d3f4-e0fb-4af4-be50-4a05d29719fd", "5dfe285e-b457-4815-b7ad-344b8b5c471f", "80c6456c-ed27-4e4a-a483-3170ab772cf6"},
		module:       "security",
		expectedPath: `internal\embed\cosca\knowledge\security\README.md`,
	},
}

// RealBenchmarkQrels devolve os gabaritos de recall do query set real (v3) no
// formato Qrels. Para cada query, o conjunto de chunks relevantes é o chunk_id
// exato pré-registrado e o first_relevant_chunk_id é o mesmo chunk. É a fonte
// usada para gerar `testdata/qrels-baseline.json`.
func RealBenchmarkQrels() []Qrels {
	out := make([]Qrels, 0, len(realBenchmarkSet))
	for _, gt := range realBenchmarkSet {
		rel := gt.relevantIDs
		if len(rel) == 0 {
			rel = []string{gt.chunkID}
		}
		out = append(out, Qrels{
			Query:                gt.query,
			RelevantChunkIDs:     rel,
			FirstRelevantChunkID: gt.chunkID,
		})
	}
	return out
}

// RealBaselineFile devolve o arquivo de baseline real (layout envolto) com a
// assinatura de embedding do corpus. É a forma de gerar o qrels-baseline.json.
func RealBaselineFile() QrelsFile {
	return QrelsFile{
		Version:   1,
		Embedding: EmbeddingSig{Model: "nomic-embed-text", Dim: "768"},
		Queries:   RealBenchmarkQrels(),
	}
}

// RealBenchmarkDocuments devolve o mapa chunk_id → document_id real, para
// rastreabilidade do GT (RECALL_DOCUMENT vs RECALL_CHUNK).
func RealBenchmarkDocuments() map[string]string {
	out := make(map[string]string, len(realBenchmarkSet))
	for _, gt := range realBenchmarkSet {
		out[gt.chunkID] = gt.docID
	}
	return out
}
