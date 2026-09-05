// Teste de ponta a ponta do fluxo de níveis: do boot (L1) à subida exigindo o
// aval do Don, e a auto-descida por watchdog. Reflete o design do engine_builder
// e a decisão do Don (2026-08-25).
package level

import (
	"errors"
	"testing"
)

// startGate simula o boot do engine_builder: agente acorda em L1, com um hook
// de elevação que replica VerifyDonPresence (fail-closed sem o Don).
func startGate(donApproves bool) (*Gate, *Watchdog) {
	g := NewGate(L1Inicial)
	g.SetElevateHook(func(from Level, a Action) (Level, error) {
		if !donApproves {
			return from, errors.New("presença do Don não confirmada")
		}
		return L3Soberano, nil
	})
	return g, NewWatchdog()
}

func TestFluxoCompletoL1ParaL3(t *testing.T) {
	// Boot: agente acorda em L1.
	g, _ := startGate(false)
	if g.Current() != L1Inicial {
		t.Fatalf("boot: agente deveria acordar em L1, foi %v", g.Current())
	}

	// L1 não pode operar máquina nem editar.
	if v := g.Check(Action{Tool: "bash", RawCommand: "cosca doctor"}); v != VDeny {
		t.Errorf("L1 operar maquina deveria ser DENY, foi %s", v)
	}

	// Subir direto para L3 sem o Don → NEGADO (fail-closed).
	if err := g.Promote(L3Soberano); err == nil {
		t.Error("subir para L3 sem aval do Don deveria falhar (fail-closed)")
	}
}

func TestFluxoSubidaSoberanoAposAval(t *testing.T) {
	// Agente já em L2 (promovido por capacidade), Don presente.
	g, _ := startGate(true)
	// Análogo à promoção real por capacidade para L2 (sem aval, permitido).
	if err := g.Promote(L2Operacional); err != nil {
		t.Fatalf("promover para L2 (por capacidade) não deveria falhar: %v", err)
	}
	if g.Current() != L2Operacional {
		t.Fatalf("agente deveria estar em L2, foi %v", g.Current())
	}
	// L2 edita workspace mas NÃO o cérebro.
	if v := g.Check(Action{Tool: "edit", TargetPath: "internal/embed/cosca/KERNEL.md"}); v != VDeny {
		t.Errorf("L2 editar o cerebro deveria ser DENY, foi %s", v)
	}
	if v := g.Check(Action{Tool: "edit", TargetPath: "internal/foo.go"}); v != VAllow {
		t.Errorf("L2 editar workspace deveria ser ALLOW, foi %s", v)
	}

	// Subida a L3 com aval do Don → permitida.
	if err := g.Promote(L3Soberano); err != nil {
		t.Fatalf("subir para L3 com aval do Don deveria passar: %v", err)
	}
	if g.Current() != L3Soberano {
		t.Fatalf("agente deveria estar em L3, foi %v", g.Current())
	}
	// Agora pode editar o cérebro (L3).
	if v := g.Check(Action{Tool: "edit", TargetPath: "internal/embed/cosca/KERNEL.md"}); v != VAllow {
		t.Errorf("L3 editar o cerebro deveria ser ALLOW, foi %s", v)
	}
}

func TestFluxoAutoDescidaPorWatchdog(t *testing.T) {
	// Em L3, o watchdog detecta loop e descende para L2.
	g, w := startGate(true)
	if err := g.Promote(L3Soberano); err != nil {
		t.Fatalf("promover para L3: %v", err)
	}
	// Loop de 3 correções sem progresso.
	w.RecordLoop("edit go.go", false)
	w.RecordLoop("edit go.go", false)
	w.RecordLoop("edit go.go", false)
	should, target := w.ShouldStabilize(g.Current())
	if !should {
		t.Fatal("watchdog deveria pedir estabilizacao (loop de 3 sem progresso)")
	}
	if target != L2Operacional {
		t.Fatalf("descer de L3 deveria ir para L2, foi %v", target)
	}
	// Aplica a descida (auto-regulação — Demote, sem aval; é a sabedoria).
	if err := g.Demote(target); err != nil {
		t.Fatalf("demover para %v: %v", target, err)
	}
	if g.Current() != L2Operacional {
		t.Fatalf("agente deveria estar em L2 após estabilizar, foi %v", g.Current())
	}
	// Agora L2 não pode mais editar o cérebro.
	if v := g.Check(Action{Tool: "edit", TargetPath: "internal/embed/cosca/KERNEL.md"}); v != VDeny {
		t.Errorf("após descer para L2, editar cerebro deveria ser DENY, foi %s", v)
	}
}
