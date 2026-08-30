// Knowledge Snapshot resolver — Fase 2A (ADR-029 §2.4).
//
// Para responder deterministicamente "qual conhecimento o cérebro tinha quando
// tomou esta decisão?", cada decisão registra o id do snapshot de conhecimento
// (ex: ks_2026_08_29_001) que estava carregado no momento. Esse id vive no
// `.cosca/knowledge/lock.yaml` (campo `knowledge_snapshot`), a âncora
// reprodutível da Fase 1 do ADR-029.
//
// A resolução é BEST-EFFORT: se o lock não existir, for ilegível, ou o campo
// não estiver presente, a função retorna "" — a decisão é gravada mesmo assim
// (o snapshot é um enriquecimento de rastreabilidade, nunca um requisito).
package audit

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// knowledgeLock é a projeção mínima do `.cosca/knowledge/lock.yaml` necessária
// para extrair o id do snapshot de conhecimento.
type knowledgeLock struct {
	KnowledgeSnapshot string `yaml:"knowledge_snapshot"`
}

// ResolveKnowledgeSnapshot lê o id do snapshot de conhecimento a partir do
// `.cosca/knowledge/lock.yaml`. `coscaDir` é o diretório `.cosca` (a mesma
// convenção de resolveCoscaDir/getCoscaDir/recordDecision); o lock esperado é
// `<coscaDir>/knowledge/lock.yaml`.
//
// Retorna "" (best-effort) quando:
//   - o lock não existe;
//   - o lock é ilegível (YAML malformado, permissão, etc.);
//   - o campo `knowledge_snapshot` está ausente/vazio.
//
// Nunca propaga erro: um snapshot não resolvido nunca deve impedir uma decisão.
func ResolveKnowledgeSnapshot(coscaDir string) string {
	if coscaDir == "" {
		return ""
	}

	lockPath := filepath.Join(coscaDir, "knowledge", "lock.yaml")
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return ""
	}

	var lock knowledgeLock
	if err := yaml.Unmarshal(data, &lock); err != nil {
		return ""
	}

	return lock.KnowledgeSnapshot
}
