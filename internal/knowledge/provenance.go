//
// Procedência das evidências (P0-P5) — de onde a evidência veio.
//
// Conforme a conversa do Don: "Descobrir quem é a fonte — eu criaria níveis
// de procedência P0—desconhecida, P1—fonte externa não verificada,
// P2—repositório identificável, P3—fonte oficial, P4—código/teste
// reproduzível, P5—múltiplas fontes independentes."
//
// E git commit é ouro: não registre apenas github.com/project/foo, registre
// repository, commit, path, sha256, retrieved. Assim é possível reproduzir:
// "Foi desse código, exatamente nessa versão, que essa evidência veio."
//
// A migração das evidências antigas é a ponte ADD-ON: campos com omitempty
// e, no load/register, Provenance vazio → P0 (desconhecida). Nenhum campo
// existente do Evidence é alterado.
//

package knowledge

import (
	"strings"
	"time"
)

// ProvenanceLevel é o nível de confiança na origem de uma evidência.
type ProvenanceLevel string

const (
	// ProvenanceUnknown é procedência desconhecida (P0).
	ProvenanceUnknown ProvenanceLevel = "P0"
	// ProvenanceExternalUnverified é fonte externa não verificada (P1).
	ProvenanceExternalUnverified ProvenanceLevel = "P1"
	// ProvenanceIdentifiableRepo é repositório identificável (P2).
	ProvenanceIdentifiableRepo ProvenanceLevel = "P2"
	// ProvenanceOfficial é fonte oficial (P3).
	ProvenanceOfficial ProvenanceLevel = "P3"
	// ProvenanceReproducible é código/teste reproduzível (P4).
	ProvenanceReproducible ProvenanceLevel = "P4"
	// ProvenanceIndependent é múltiplas fontes independentes (P5).
	ProvenanceIndependent ProvenanceLevel = "P5"
)

// Rank devolve a posição da procedência na escala; maior é mais confiável.
// Procedências desconhecidas rankeiam abaixo de P0 para comparações seguras.
func (p ProvenanceLevel) Rank() int {
	return p.rank()
}

// rank devolve a posição da procedência na escala; maior é mais confiável.
// Procedências desconhecidas rankeiam abaixo de P0 para comparações seguras.
func (p ProvenanceLevel) rank() int {
	switch p {
	case ProvenanceIndependent:
		return 5
	case ProvenanceReproducible:
		return 4
	case ProvenanceOfficial:
		return 3
	case ProvenanceIdentifiableRepo:
		return 2
	case ProvenanceExternalUnverified:
		return 1
	case ProvenanceUnknown:
		return 0
	default:
		return -1
	}
}

// Valid devolve true quando a procedência é um dos seis níveis P0-P5.
func (p ProvenanceLevel) Valid() bool {
	return p.rank() >= 0
}

// Description devolve a descrição pt-BR da procedência.
func (p ProvenanceLevel) Description() string {
	switch p {
	case ProvenanceUnknown:
		return "desconhecida"
	case ProvenanceExternalUnverified:
		return "fonte externa não verificada"
	case ProvenanceIdentifiableRepo:
		return "repositório identificável"
	case ProvenanceOfficial:
		return "fonte oficial"
	case ProvenanceReproducible:
		return "código/teste reproduzível"
	case ProvenanceIndependent:
		return "múltiplas fontes independentes"
	default:
		return "sem procedência definida"
	}
}

// ParseProvenance interpreta "P0".."P5" (case-insensitive, com ou sem
// espaços) e devolve o nível correspondente. Níveis inválidos → ok=false.
func ParseProvenance(s string) (ProvenanceLevel, bool) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "P0":
		return ProvenanceUnknown, true
	case "P1":
		return ProvenanceExternalUnverified, true
	case "P2":
		return ProvenanceIdentifiableRepo, true
	case "P3":
		return ProvenanceOfficial, true
	case "P4":
		return ProvenanceReproducible, true
	case "P5":
		return ProvenanceIndependent, true
	default:
		return "", false
	}
}

// SourceRef é a referência de fonte auditável de uma evidência: exatamente
// o repository, commit, path, sha256 e retrieved que permitem reproduzir
// "foi desse código, exatamente nessa versão, que essa evidência veio".
type SourceRef struct {
	Repository string    `json:"repository,omitempty"`
	Commit     string    `json:"commit,omitempty"`
	Path       string    `json:"path,omitempty"`
	SHA256     string    `json:"sha256,omitempty"`
	Retrieved  time.Time `json:"retrieved,omitempty"`
}

// SourceRef devolve a referência de fonte auditável da evidência.
func (e Evidence) SourceRef() SourceRef {
	return SourceRef{
		Repository: e.Repository,
		Commit:     e.Commit,
		Path:       e.Path,
		SHA256:     e.SHA256,
		Retrieved:  e.Retrieved,
	}
}

// FillSourceRef copia uma referência de fonte auditável para a evidência.
func (e *Evidence) FillSourceRef(ref SourceRef) {
	e.Repository = ref.Repository
	e.Commit = ref.Commit
	e.Path = ref.Path
	e.SHA256 = ref.SHA256
	e.Retrieved = ref.Retrieved
}

// FillProvenanceDefaults aplica a ponte de migração: procedência vazia →
// P0 (desconhecida). Evidências antigas — escritas antes da procedência —
// carregam sem o campo e ganham P0 no load/register, sem alterar nenhum
// outro campo.
func (e *Evidence) FillProvenanceDefaults() {
	if e.Provenance == "" {
		e.Provenance = ProvenanceUnknown
	}
}

// IsReproducible devolve true quando a evidência é totalmente reproduzível:
// procedência >= P4 (código/teste reproduzível ou múltiplas fontes
// independentes) E tem repository + commit + sha256 — o trio que permite
// re-buscar o código exato daquela versão.
func (e Evidence) IsReproducible() bool {
	return e.Provenance.rank() >= ProvenanceReproducible.rank() &&
		strings.TrimSpace(e.Repository) != "" &&
		strings.TrimSpace(e.Commit) != "" &&
		strings.TrimSpace(e.SHA256) != ""
}

// ReproducibleEvidence devolve as evidências do item que são totalmente
// reproduzíveis (IsReproducible), na ordem em que aparecem.
func (k *KnowledgeItem) ReproducibleEvidence() []Evidence {
	out := make([]Evidence, 0, len(k.Evidence))
	for _, ev := range k.Evidence {
		if ev.IsReproducible() {
			out = append(out, ev)
		}
	}
	return out
}
