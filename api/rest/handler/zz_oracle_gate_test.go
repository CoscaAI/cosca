package handler

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/oracle"
)

func TestRunHandler_HasOracleGate(t *testing.T) {
	// O RunHandler deve ter o Gate do Oráculo inicializado (ORACLE_PROTOCOL).
	h := NewRunHandler(nil, nil, nil)
	if h.oracleGate == nil {
		t.Fatal("oracleGate nil — o Oráculo não está no serve")
	}
	// Request normal → ACCEPT (ou CAVEAT, nunca REJECT).
	v := h.oracleGate.Evaluate(oracle.SemanticPackage{
		Intent:  "responder ao prompt do usuário",
		Result:  "oi, como ta o carro?",
		Source:  "external",
		Context: "request do usuário via API",
	})
	if v.Decision == oracle.Reject {
		t.Errorf("request normal não deveria ser REJECT, got %s (%s)", v.Decision, v.Reason)
	}
	// Pacote vazio → REJECT (o oráculo bloqueia lixo).
	v2 := h.oracleGate.Evaluate(oracle.SemanticPackage{})
	if v2.Decision != oracle.Reject {
		t.Errorf("pacote vazio deveria ser REJECT, got %s", v2.Decision)
	}
}
