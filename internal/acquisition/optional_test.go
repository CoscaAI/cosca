package acquisition

import (
	"errors"
	"testing"
)

// TestOptional_Success valida que Optional devolve o valor e ok=true quando a
// função não falha.
func TestOptional_Success(t *testing.T) {
	v, ok := Optional(func() (int, error) { return 42, nil })
	if !ok {
		t.Fatal("ok deveria ser true")
	}
	if v != 42 {
		t.Fatalf("v = %d, want 42", v)
	}
}

// TestOptional_Error valida que Optional devolve (zero, false) quando a função
// falha — sem estourar o chamador.
func TestOptional_Error(t *testing.T) {
	boom := errors.New("boom")
	v, ok := Optional(func() (int, error) { return 0, boom })
	if ok {
		t.Fatal("ok deveria ser false em erro")
	}
	if v != 0 {
		t.Fatalf("v = %d, want zero", v)
	}
}

// TestOptional_DistinguishesZeroFromUnavailable valida que um zero legítimo
// (com ok=true) é diferente de "não disponível" (ok=false).
func TestOptional_DistinguishesZeroFromUnavailable(t *testing.T) {
	// zero legítimo (sem erro) → ok=true.
	v1, ok1 := Optional(func() (int, error) { return 0, nil })
	if !ok1 || v1 != 0 {
		t.Fatalf("zero legítimo: v=%d ok=%v, want 0/true", v1, ok1)
	}
	// indisponível (erro) → ok=false.
	_, ok2 := Optional(func() (int, error) { return 0, errors.New("down") })
	if ok2 {
		t.Fatal("indisponível deveria ter ok=false")
	}
}
