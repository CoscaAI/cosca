package evolution

import (
	"errors"
	"testing"
)

func TestTxn_CopyOnWrite_RollbackRestores(t *testing.T) {
	base := map[string]string{"a": "1", "b": "2"}
	tx := NewTxn(base)
	tx.Set("a", "99")
	tx.Set("c", "3")
	// Durante a txn: delta visível, base INTACTO.
	if v, _ := tx.Get("a"); v != "99" {
		t.Fatalf("delta deveria ser visível: %s", v)
	}
	if v, _ := base["a"]; v != "1" {
		t.Fatalf("base NÃO deve mudar durante a txn (O(edits) rollback): %s", v)
	}
	tx.Rollback()
	if v, _ := tx.Get("a"); v != "1" {
		t.Fatalf("rollback deveria restaurar a=1: %s", v)
	}
	if _, ok := tx.Get("c"); ok {
		t.Fatal("rollback deveria remover a chave nova")
	}
}

func TestTxn_Commit_AppliesDelta(t *testing.T) {
	base := map[string]string{"a": "1"}
	tx := NewTxn(base)
	tx.Set("a", "42")
	tx.Set("b", "novo")
	tx.Commit()
	if base["a"] != "42" || base["b"] != "novo" {
		t.Fatalf("commit deveria aplicar o delta: %+v", base)
	}
}

func TestGuard_FailedTickRollsBack(t *testing.T) {
	base := map[string]string{"world": "ok"}
	_, err := Guard(base, "edit-world", true,
		func(tx *Txn) error {
			tx.Set("world", "corrompido")
			return errors.New("boom")
		}, nil)
	if err == nil {
		t.Fatal("tick que falha deve dar erro (fail-closed)")
	}
	if base["world"] != "ok" {
		t.Fatalf("estado base deve estar íntegro após falha: %+v", base)
	}
}

func TestGuard_RegressionRollsBack(t *testing.T) {
	base := map[string]string{"objects": "5"}
	_, err := Guard(base, "place", true,
		func(tx *Txn) error { tx.Set("objects", "0"); return nil },
		func(s map[string]string) error {
			// regressão: sumiu objetos
			return errors.New("objects sumiram (regression)")
		})
	if err == nil {
		t.Fatal("regressão deve dar erro (strict I2)")
	}
	if base["objects"] != "5" {
		t.Fatalf("regressão deve rollback o estado: %+v", base)
	}
}

func TestGuard_NonStrictDegrade(t *testing.T) {
	base := map[string]string{"objects": "5"}
	_, err := Guard(base, "place", false,
		func(tx *Txn) error { tx.Set("objects", "0"); return nil },
		func(s map[string]string) error { return errors.New("regression") })
	if err != nil {
		t.Fatalf("não-estrito não deve propagar erro fatal, apenas rejeitar o tick: %v", err)
	}
	if base["objects"] != "5" {
		t.Fatalf("não-estrito também deve rollback (estado íntegro): %+v", base)
	}
}

func TestGuard_ValidTickCommits(t *testing.T) {
	base := map[string]string{"objects": "5"}
	tx, err := Guard(base, "place", true,
		func(t *Txn) error { t.Set("objects", "6"); return nil },
		func(s map[string]string) error { return nil })
	if err != nil {
		t.Fatalf("tick válido: %v", err)
	}
	if tx == nil || base["objects"] != "6" {
		t.Fatalf("tick válido deve commitar: %+v", base)
	}
}
