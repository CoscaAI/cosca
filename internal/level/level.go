// Package level — o sistema de NÍVEIS de capacidade do Cosca (enforcement por
// código, não por promessa). Decisão do Don 2026-08-25:
//
//	L1 — INICIAL      : o primeiro despertar (OpenCode). Só leitura de identidade.
//	L2 — OPERACIONAL  : semântica + opera a máquina. NÃO edita o próprio cérebro.
//	L3 — SOBERANO     : capacidade total. SÓ SOBE COM O AVAL DO DON.
//
// Cada nível define permissões por DIMENSÃO (memória, edição, sistema, pesquisa,
// segurança) numa matriz determinística. O LevelGate intercepta toda Action
// ANTES do policy e decide pela matriz — independente do LLM (L366).
//
// A auto-regulação (watchdog) detecta o padrão de loop que matou o kernel
// (L434) e REDUZ o nível automaticamente para estabilizar. "A subida é
// capacidade; a descida é sabedoria. Quem sabe descer, não morre no loop."
package level

import "strings"

// Capacidade (Cap) é uma dimensão de habilidade que um nível libera/limita.
type Cap string

// Dimensões de capacidade (as cinco do design enterprise).
const (
	// CapMemoria — acesso à memória do kernel.
	CapMemoria Cap = "memoria"
	// CapPesquisa — como buscar conhecimento (estático vs semântico).
	CapPesquisa Cap = "pesquisa"
	// CapSistema — operar a máquina (CLI, serviços, processo cosca).
	CapSistema Cap = "sistema"
	// CapEdicao — modificar arquivos (workspace vs próprio cérebro).
	CapEdicao Cap = "edicao"
	// CapSeguranca — soberania sobre auto-modificação (proteção anti-loop).
	CapSeguranca Cap = "seguranca"
)

// Todas as dimensões conhecidas.
var AllCaps = []Cap{CapMemoria, CapPesquisa, CapSistema, CapEdicao, CapSeguranca}

// Level é o degrau de capacidade. Quanto maior, mais alcance e mais soberania.
type Level int

// Os três níveis (decisão do Don — sem nível 4; L3 é o teto e exige aval).
const (
	// L1Inicial é o primeiro despertar (OpenCode): identidade + contexto.
	L1Inicial Level = 1
	// L2Operacional tem semântica + opera a máquina; NÃO edita o próprio cérebro.
	L2Operacional Level = 2
	// L3Soberano tem capacidade total; SÓ SOBE COM AVAL DO DON.
	L3Soberano Level = 3
)

// String devolve o nome legível do nível.
func (l Level) String() string {
	switch l {
	case L1Inicial:
		return "L1-INICIAL"
	case L2Operacional:
		return "L2-OPERACIONAL"
	case L3Soberano:
		return "L3-SOBERANO"
	default:
		return "L?=DESCONHECIDO"
	}
}

// Valid devolve true quando o nível é um dos três conhecidos.
func (l Level) Valid() bool {
	return l >= L1Inicial && l <= L3Soberano
}

// Permission é o grau de permissão para uma dimensão num nível.
type Permission int

const (
	// PermNone — nada liberado.
	PermNone Permission = iota
	// PermRead — leitura/observação (identidade, contexto).
	PermRead
	// PermOperate — operar a máquina (CLI, serviços), mas não modificar cérebro.
	PermOperate
	// PermWorkspace — editar arquivos do workspace do projeto.
	PermWorkspace
	// PermBrain — editar o próprio cérebro (internal/embed/cosca), exige re-assinar.
	PermBrain
)

func (p Permission) String() string {
	switch p {
	case PermNone:
		return "NENHUMA"
	case PermRead:
		return "LEITURA"
	case PermOperate:
		return "OPERAR"
	case PermWorkspace:
		return "WORKSPACE"
	case PermBrain:
		return "CEREBRO"
	default:
		return "?"
	}
}

// MaxLevel é o teto definido pela Matriz (é o nível soberano; para subir além
// exige o aval do Don, que é uma autorização explícita no runtime, não na matriz).
const MaxLevel = L3Soberano

// Matrix é o mapa nível × dimensão → permissão. É a fonte determinística de
// soberania. A leitura de um nível não coberto devolve PermNone (fail-closed).
var Matrix = map[Level]map[Cap]Permission{
	// L1 — INICIAL: só acorda (identidade, contexto, leitura). Nada mais.
	L1Inicial: {
		CapMemoria:   PermRead,
		CapPesquisa:  PermRead,
		CapSistema:   PermNone,
		CapEdicao:    PermNone,
		CapSeguranca: PermRead,
	},
	// L2 — OPERACIONAL: semântica + opera a máquina; NÃO edita o próprio cérebro.
	L2Operacional: {
		CapMemoria:   PermOperate, // busca semântica no knowledge.db
		CapPesquisa:  PermOperate, // knowledge.db vetorial
		CapSistema:   PermOperate, // CLI/serviços da máquina
		CapEdicao:    PermWorkspace, // edita o projeto, mas NÃO o cérebro
		CapSeguranca: PermOperate,
	},
	// L3 — SOBERANO: capacidade total. SÓ SOBE COM O AVAL DO DON.
	L3Soberano: {
		CapMemoria:   PermOperate,
		CapPesquisa:  PermOperate,
		CapSistema:   PermOperate,
		CapEdicao:    PermBrain, // edita o cérebro, mas exige re-assinar (resolução do Don)
		CapSeguranca: PermBrain,
	},
}

// Permissao devolve a permissão de uma capacidade no nível. Nível ou capacidade
// desconhecidos → PermNone (fail-closed, P1). O design é conservador: o que não
// está explicitamente liberado está bloqueado.
func Permissao(l Level, c Cap) Permission {
	if !l.Valid() {
		return PermNone
	}
	if m, ok := Matrix[l]; ok {
		if p, ok := m[c]; ok {
			return p
		}
	}
	return PermNone
}

// CanEditarBrain devolve true apenas no nível que libera edição do próprio
// cérebro (L3-SOBERANO) — e mesmo assim exige re-assinar (o gate cuida disso).
func (l Level) CanEditarBrain() bool {
	return Permissao(l, CapEdicao) >= PermBrain
}

// BrainPath é o caminho relativo do cérebro (embutido) — a fronteira crítica.
const BrainPath = "internal/embed/cosca"

// InternalPath é a raiz do código-fonte interno do Cosca. Editar qualquer
// coisa sob internal/ é tocar o coração da família — por isso fica sob o gate
// de soberania (SÓ o L3-SOBERANO, com aval do Don, pode editar; L1/L2 negam).
// Decisão do Don (2026-09-07): "tudo que mexe no codigo internal deve ter
// bloqueio gate."
const InternalPath = "internal"

// IsBrainPath devolve true quando o path dado está dentro do cérebro.
func IsBrainPath(path string) bool {
	return strings.Contains(filepathToSlash(path), BrainPath)
}

// IsInternalPath devolve true quando o path dado está dentro do código-fonte
// interno do Cosca (internal/). É a fronteira que protege o coração da família.
func IsInternalPath(path string) bool {
	return strings.Contains(filepathToSlash(path), InternalPath)
}

// filepathToSlash normaliza separadores para '/' (comparação estável).
func filepathToSlash(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}
