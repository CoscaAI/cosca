package level

import (
	"fmt"
	"strings"
)

// Verdict é a decisão do LevelGate para uma Action no nível atual.
type Verdict int

const (
	// VAllow — a ação é permitida no nível atual.
	VAllow Verdict = iota
	// VDeny — a ação é bloqueada pelo nível (capacidade insuficiente).
	VDeny
	// VNeedApproval — ação fora da soberania do nível atual: requer o aval do Don
	// (usado para subir ao L3-SOBERANO, que SÓ SOBE COM AVAL).
	VNeedApproval
)

func (v Verdict) String() string {
	switch v {
	case VAllow:
		return "LEVEL-ALLOW"
	case VDeny:
		return "LEVEL-DENY"
	case VNeedApproval:
		return "LEVEL-NEED-APPROVAL"
	default:
		return "?"
	}
}

// Action é uma proposta de ação (espelha a Action do policy, sem acoplar).
type Action struct {
	// Tool é o nome da ferramenta ("bash", "write", "edit", "read", "search"...).
	Tool string
	// RawCommand é o comando efetivo (para bash/exec).
	RawCommand string
	// TargetPath é o alvo de arquivo (para write/edit/delete).
	TargetPath string
}

// Gate decide se uma ação é permitida no nível atual, segundo a matriz de
// soberania. É ENFORCEMENT por código — independe do LLM.
type Gate struct {
	// current é o nível atual do agente neste processo.
	current Level
	// onElevate é o hook chamado quando uma ação pede subida de nível
	// (ex.: editar o cérebro no L2). Retorna o novo nível se autorizado.
	onElevate func(from Level, a Action) (Level, error)
}

// NewGate cria um Gate começando no nível dado.
func NewGate(start Level) *Gate {
	return &Gate{current: start}
}

// SetElevateHook registra o callback de elevação (implementado pelo runtime).
// É ele que consulta o Don / a aprovação antes de permitir o L3-SOBERANO.
func (g *Gate) SetElevateHook(fn func(from Level, a Action) (Level, error)) {
	g.onElevate = fn
}

// Current devolve o nível atual.
func (g *Gate) Current() Level { return g.current }

// Promote eleva o nível (1→2 automático por capacidade; 2→3 exige o hook do Don).
// Retorna erro quando a subida não é autorizada (fail-closed).
func (g *Gate) Promote(to Level) error {
	if to > MaxLevel {
		return fmt.Errorf("level: %s acima do teto %s", to, MaxLevel)
	}
	if to <= g.current {
		return nil // já está no nível ou acima — não retrocede aqui
	}
	// Subir direto para o SOBERANO (ou além de operacional) exige aval.
	// Se o hook de elevação não está configurado, NEGA (princípio L3).
	if to >= L3Soberano && g.current < L3Soberano {
		if g.onElevate == nil {
			return fmt.Errorf("level: %s requer AVAL do Don — sem hook de autorização, subida negada", to)
		}
		// O hook decide (consulta o Don). Ação vazia, só a subida.
		next, err := g.onElevate(g.current, Action{})
		if err != nil {
			return fmt.Errorf("level: subida para %s negada pelo Don: %w", to, err)
		}
		if next < to {
			return fmt.Errorf("level: subida para %s não autorizada (permanece %s)", to, next)
		}
	}
	g.current = to
	return nil
}

// Demote reduz o nível para estabilizar (auto-regulação — decisão do Don
// 2026-08-25). Ao contrário da subida, a descida NÃO exige aval: é a sabedoria
// de descer quando se vê perdendo o controle. "A subida é capacidade; a descida
// é sabedoria." Só desce até o piso L1-INICIAL — nunca abaixo.
func (g *Gate) Demote(to Level) error {
	if to >= g.current {
		return nil // não sobe nem mantém; descida só para níveis menores
	}
	if to < L1Inicial {
		to = L1Inicial
	}
	g.current = to
	return nil
}

// Check avalia uma ação no nível atual. Ordem: se a ação toca o cérebro e o
// nível não permite, tenta elevar (com aval do Don). Caso contrário, decide
// pela matriz. Nada é executado sem VAllow — fail-closed.
func (g *Gate) Check(a Action) Verdict {
	if !g.current.Valid() {
		return VDeny
	}

	// Ação de edição no cérebro (internal/embed/cosca).
	if isBrainEdit(a) {
		// No nível OPERACIONAL o cérebro é intocável (decisão do Don).
		if g.current == L2Operacional {
			return VDeny
		}
		if g.current == L1Inicial {
			return VDeny
		}
		// No L3-SOBERANO é permitido (permite editar o cérebro), já validado.
		if g.current == L3Soberano {
			return VAllow
		}
	}

	// Ações de escrita/operação precisam de permissão mínima = PermOperate
	// para o escopo do projeto. Leitura é sempre permitida.
	switch {
	case isRead(a):
		return VAllow
	}

	// Operação de máquina (bash/exec/CLI). Só em L2+.
	if isMachineOp(a) {
		if g.current >= L2Operacional {
			return VAllow
		}
		return VDeny
	}

	// Escrita no workspace. L2+ permite.
	if isWorkspaceWrite(a) {
		if g.current >= L2Operacional {
			return VAllow
		}
		return VDeny
	}

	// Ação de memória/pesquisa semântica. L2+.
	if isSemantic(a) {
		if g.current >= L2Operacional {
			return VAllow
		}
		return VDeny
	}

	return VAllow
}

// isBrainEdit devolve true quando a ação edita/escreve no cérebro (embed).
func isBrainEdit(a Action) bool {
	tool := a.Tool
	if tool == "write" || tool == "edit" || tool == "delete" || tool == "remove" {
		return IsBrainPath(a.TargetPath)
	}
	return strings.Contains(filepathToSlash(a.RawCommand), BrainPath)
}

// isRead devolve true para ferramentas de leitura (livres em qualquer nível).
func isRead(a Action) bool {
	switch a.Tool {
	case "read", "glob", "grep", "search", "list", "stat", "diff", "status", "info":
		return true
	}
	return false
}

// isMachineOp devolve true para operação direta de máquina (bash/exec/CLI).
func isMachineOp(a Action) bool {
	switch a.Tool {
	case "bash", "exec", "run", "terminal", "cmd":
		return true
	}
	return false
}

// isWorkspaceWrite devolve true para escrita no workspace do projeto.
func isWorkspaceWrite(a Action) bool {
	switch a.Tool {
	case "write", "edit", "create", "append", "patch":
		return true
	}
	return false
}

// isSemantic devolve true para acesso semântico (busca no knowledge.db).
func isSemantic(a Action) bool {
	switch a.Tool {
	case "search", "symbols", "session", "knowledge":
		return true
	}
	return false
}
