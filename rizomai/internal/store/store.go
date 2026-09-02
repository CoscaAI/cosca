// Package store implementa o acesso a dados sobre Postgres (ADR-002):
// pool pgx, migrations embedded e repositórios por agregado.
//
// IDs textuais com prefixo (ADR-005 §1.4) são gerados no domínio; o store
// persiste o valor exato exposto no contrato.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound é retornado quando o registro não existe (ou é de outro tenant).
var ErrNotFound = errors.New("store: not found")

// DBTX é a interface mínima de banco usada pelos repositórios.
// *pgxpool.Pool e *pgxmock.Pool (testes) implementam — permite validar o SQL
// sem banco real.
type DBTX interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Store agrega os repositórios do domínio sobre Postgres.
type Store struct {
	db DBTX
}

// New abre o pool pgx, valida a conexão (ping) e devolve o Store.
// Falha rápido se o banco não responder.
func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Store{db: pool}, nil
}

// Ping verifica a conectividade com o banco (usado pelo /healthz).
func (s *Store) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var one int
	return s.db.QueryRow(ctx, `SELECT 1`).Scan(&one)
}

// jsonBytes serializa para JSONB; slice/map nil ou erro vira "[]".
func jsonBytes(v any) []byte {
	if isNil(v) {
		return []byte("[]")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("[]")
	}
	return b
}

// jsonObject serializa um objeto para JSONB; nil ou erro vira "{}".
func jsonObject(v any) []byte {
	if isNil(v) {
		return []byte("{}")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}

// isNil detecta nil de forma segura inclusive quando uma interface não-nil
// carrega um map/slice/ponteiro nil (ex.: map[string]any(nil) em any).
func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Map, reflect.Slice, reflect.Array, reflect.Ptr, reflect.Interface, reflect.Func, reflect.Chan:
		return rv.IsNil()
	default:
		return false
	}
}
