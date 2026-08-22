package grpcclient

import (
	"context"
	"testing"
)

// ── LOOP V5: FALSIFICAÇÃO — domínio clients ────────────────────────────
//
// Os grpc clients acessam c.conn/c.service() lazy. Com addr vazio (config
// ausente), service() deve retornar erro, não PANIC.

func TestFalsify_KnowledgeClientEmptyAddr(t *testing.T) {
	c := &KnowledgeClient{addr: ""} // sem endereço
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		_, _ = c.Search(context.Background(), nil)
	}()
	if panicked {
		t.Log("CONFIRMAÇÃO: grpc client PANIC com addr vazio")
	} else {
		t.Log("CONTRAEXEMPLO: grpc client não panicou com addr vazio")
	}
}
