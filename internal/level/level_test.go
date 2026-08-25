package level

import (
	"testing"
)

func TestMatrixSoberania(t *testing.T) {
	cases := []struct {
		name string
		l    Level
		c    Cap
		want Permission
	}{
		{"L1 inicial nao edita", L1Inicial, CapEdicao, PermNone},
		{"L1 inicial so le memoria", L1Inicial, CapMemoria, PermRead},
		{"L1 nao opera sistema", L1Inicial, CapSistema, PermNone},
		{"L2 opera sistema", L2Operacional, CapSistema, PermOperate},
		{"L2 edita workspace", L2Operacional, CapEdicao, PermWorkspace},
		{"L2 nao edita cerebro", L2Operacional, CapEdicao, PermWorkspace}, // < PermBrain
		{"L3 edita cerebro", L3Soberano, CapEdicao, PermBrain},
		{"nivel invalido fail-closed", Level(9), CapMemoria, PermNone},
		{"cap desconhecida fail-closed", L2Operacional, Cap("xxx"), PermNone},
	}
	for _, c := range cases {
		if got := Permissao(c.l, c.c); got != c.want {
			t.Errorf("%s: Permissao(%v,%v)=%v, esperava %v", c.name, c.l, c.c, got, c.want)
		}
	}
}

func TestGateBrainEditNoInicialDeny(t *testing.T) {
	g := NewGate(L1Inicial)
	v := g.Check(Action{Tool: "edit", TargetPath: "internal/embed/cosca/KERNEL.md"})
	if v != VDeny {
		t.Errorf("L1 editar cerebro deveria ser DENY, foi %s", v)
	}
}

func TestGateOperacionalNaoEditaCerebro(t *testing.T) {
	g := NewGate(L2Operacional)
	v := g.Check(Action{Tool: "edit", TargetPath: "internal/embed/cosca/KERNEL.md"})
	if v != VDeny {
		t.Errorf("L2 editar cerebro deveria ser DENY (nao edita o proprio cerebro), foi %s", v)
	}
	// mas edita workspace
	v2 := g.Check(Action{Tool: "edit", TargetPath: "internal/foo/bar.go"})
	if v2 != VAllow {
		t.Errorf("L2 editar workspace deveria ser ALLOW, foi %s", v2)
	}
}

func TestGateOperacionalOperaMaquina(t *testing.T) {
	g := NewGate(L2Operacional)
	v := g.Check(Action{Tool: "bash", RawCommand: "cosca doctor"})
	if v != VAllow {
		t.Errorf("L2 operar maquina deveria ser ALLOW, foi %s", v)
	}
}

func TestGateL1NaoOperaMaquina(t *testing.T) {
	g := NewGate(L1Inicial)
	v := g.Check(Action{Tool: "bash", RawCommand: "cosca doctor"})
	if v != VDeny {
		t.Errorf("L1 operar maquina deveria ser DENY, foi %s", v)
	}
}

func TestPromoteSoberanoExigeAval(t *testing.T) {
	// Sem hook → subida para SOBERANO negada (fail-closed).
	g := NewGate(L2Operacional)
	if err := g.Promote(L3Soberano); err == nil {
		t.Error("subir para SOBERANO sem hook deveria falhar (exige aval do Don)")
	}
	// Com hook que NEGA → mantem.
	g2 := NewGate(L2Operacional)
	g2.SetElevateHook(func(from Level, a Action) (Level, error) {
		return L2Operacional, nil
	})
	if err := g2.Promote(L3Soberano); err == nil {
		t.Error("subida negada pelo Don deveria falhar")
	}
	// Com hook que AUTORIZA → sobe.
	g3 := NewGate(L2Operacional)
	g3.SetElevateHook(func(from Level, a Action) (Level, error) {
		return L3Soberano, nil
	})
	if err := g3.Promote(L3Soberano); err != nil {
		t.Errorf("subida autorizada pelo Don deveria passar, erro: %v", err)
	}
	if g3.Current() != L3Soberano {
		t.Errorf("nivel atual deveria ser SOBERANO, foi %v", g3.Current())
	}
}

func TestWatchdogLoopDetectaEDesce(t *testing.T) {
	w := NewWatchdog()
	// 3 correcoes iguais sem progresso → gatilho.
	w.RecordLoop("edit go.go", false)
	w.RecordLoop("edit go.go", false)
	w.RecordLoop("edit go.go", false)
	ok, target := w.ShouldStabilize(L3Soberano)
	if !ok {
		t.Error("watchdog deveria pedir estabilizacao apos loop de 3 sem progresso")
	}
	if target != L2Operacional {
		t.Errorf("descer de L3 deveria ir para L2, foi %v", target)
	}
}

func TestWatchdogBrainEditSemSign(t *testing.T) {
	w := NewWatchdog()
	w.RecordBrainEditSemSign()
	w.RecordBrainEditSemSign()
	ok, _ := w.ShouldStabilize(L3Soberano)
	if !ok {
		t.Error("watchdog deveria estabilizar apos 2 edicoes de cerebro sem re-assinar")
	}
	w.ResetBrainSign()
	if w.brainEditsSemSign != 0 {
		t.Error("ResetBrainSign deveria zerar o contador")
	}
}

func TestIsBrainPath(t *testing.T) {
	if !IsBrainPath("internal/embed/cosca/KERNEL.md") {
		t.Error("deveria reconhecer caminho do cerebro")
	}
	if !IsBrainPath(`internal\embed\cosca\KERNEL.md`) {
		t.Error("deveria reconhecer caminho do cerebro com backslash (windows)")
	}
	if IsBrainPath("internal/foo/bar.go") {
		t.Error("nao deveria reconhecer path fora do cerebro")
	}
}

func TestAutoPromotePorCapacidade(t *testing.T) {
	// Agente em L1 opera maquina com sucesso → promovido a L2 por capacidade.
	g := NewGate(L1Inicial)
	if next, ok := g.AutoPromote(); !ok || next != L2Operacional {
		t.Errorf("AutoPromote de L1 deveria subir para L2, foi %v (ok=%v)", next, ok)
	}
	// Agora em L2, AutoPromote nao sobe mais automaticamente (L3 exige aval).
	if _, ok := g.AutoPromote(); ok {
		t.Error("AutoPromote de L2 nao deveria subir (L3 exige aval do Don)")
	}
}
