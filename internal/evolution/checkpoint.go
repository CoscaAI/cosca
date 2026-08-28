// checkpoint.go — Checkpoint / Rollback O(1) (gema #5, ADR-022; mineração ruflo
// checkpoint-gate).
//
// Para ticks que MUTAM estado persistido (ex.: editar o mundo — ADR-021), a
// operação deve ser reversível de forma barata e segura. O padrão copy-on-write
// dá:
//
//	checkpoint = O(1)    (só guarda a referência do estado base)
//	rollback   = O(edits)(descarta o delta; o base fica intacto)
//
// Determinístico (I1), fail-closed (I2): se o tick falha OU é regressão, o
// Guard rebobina o estado e aborta. Degrade gracioso.
package evolution

import "fmt"

// Txn é uma transação copy-on-write sobre um estado (map[string]string).
type Txn struct {
	base     map[string]string
	delta    map[string]string
	touched  map[string]bool
	committed bool
}

// NewTxn inicia uma transação sobre um estado base. O checkpoint é O(1):
// guarda a referência; mutações vão para um delta separado.
func NewTxn(base map[string]string) *Txn {
	return &Txn{base: base, delta: map[string]string{}, touched: map[string]bool{}}
}

// Get lê o valor de uma chave (do delta se tocada, senão do base).
func (t *Txn) Get(key string) (string, bool) {
	if t.touched[key] {
		v, ok := t.delta[key]
		return v, ok
	}
	v, ok := t.base[key]
	return v, ok
}

// Set grava uma mutação no DELTA (não no base) — rollback é O(edits).
func (t *Txn) Set(key, value string) {
	if t.committed {
		return
	}
	t.delta[key] = value
	t.touched[key] = true
}

// Rollback descarta o delta — o estado base fica INTACTO (O(edits)).
func (t *Txn) Rollback() {
	t.delta = map[string]string{}
	t.touched = map[string]bool{}
}

// Commit aplica o delta ao base (sela a transação). Chamável uma vez.
func (t *Txn) Commit() {
	if t.committed {
		return
	}
	for k, v := range t.delta {
		t.base[k] = v
	}
	t.committed = true
	t.delta = map[string]string{}
	t.touched = map[string]bool{}
}

// Snapshot devolve o estado PÓS-TICK (base + delta) sem mutar nada — para
// validação antes do Commit.
func (t *Txn) Snapshot() map[string]string {
	out := make(map[string]string, len(t.base)+len(t.delta))
	for k, v := range t.base {
		out[k] = v
	}
	for k, v := range t.delta {
		out[k] = v
	}
	return out
}

// Validate é uma função que decide se um estado pós-tick é aceitável.
type Validate func(state map[string]string) error

// Guard roda `mutate` num Txn; se `mutate` falhar OU `validate` rejeitar, faz
// ROLLBACK (fail-closed I2) — o tick nunca deixa estado parcial/regredido.
// Degrade gracioso: retorna o resultado do mutate, mas com estado íntegro.
func Guard(base map[string]string, label string, strict bool, mutate func(*Txn) error, validate Validate) (*Txn, error) {
	tx := NewTxn(base)
	if err := mutate(tx); err != nil {
		tx.Rollback()
		return tx, fmt.Errorf("%s: tick failed, rolled back (I2): %w", label, err)
	}
	if validate != nil {
		if err := validate(tx.Snapshot()); err != nil {
			tx.Rollback()
			if strict {
				return tx, fmt.Errorf("%s: regression, rolled back (I2): %w", label, err)
			}
			// não-estrito: rejeita o tick mas não propaga erro fatal.
			return tx, nil
		}
	}
	tx.Commit()
	return tx, nil
}
