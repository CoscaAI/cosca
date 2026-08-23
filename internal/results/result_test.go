// Package results — testes do envelope uniforme (paridade D6, ADR-011).
package results

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOK(t *testing.T) {
	r := OK(map[string]string{"id": "abc"})
	assert.True(t, r.Success)
	assert.Equal(t, 0, r.ExitCode)
	assert.False(t, r.Degraded)
	assert.Empty(t, r.Reason)
	assert.False(t, r.WasError())
	assert.Nil(t, r.Err())
	assert.Equal(t, map[string]string{"id": "abc"}, r.Data)
}

func TestFail(t *testing.T) {
	r := Fail(2, "boom")
	assert.False(t, r.Success)
	assert.Equal(t, 2, r.ExitCode)
	assert.Equal(t, "boom", r.Reason)
	assert.True(t, r.WasError())
	err := r.Err()
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "exitCode 2")
	assert.Contains(t, err.Error(), "boom")
	// Preserva o envelope na borda.
	var re *ResultError
	assert.True(t, errors.As(err, &re))
	assert.Equal(t, 2, re.Result.ExitCode)
}

func TestFailNormalizesZeroExitCode(t *testing.T) {
	// Um exitCode 0 jamais deve parecer sucesso na borda (invariante D6).
	r := Fail(0, "proibido")
	assert.False(t, r.Success)
	assert.NotEqual(t, 0, r.ExitCode)
	assert.True(t, r.WasError())
}

func TestDegraded(t *testing.T) {
	r := Degraded("fallback de provedor")
	// Degradado é sucesso (exitCode == 0): "funcionou, mas degradado".
	assert.True(t, r.Success)
	assert.True(t, r.Degraded)
	assert.Equal(t, 0, r.ExitCode)
	assert.Equal(t, "fallback de provedor", r.Reason)
	assert.False(t, r.WasError())
	assert.Nil(t, r.Err())
}

// TestSuccessDerivedFromExitCode garante o invariante central (paridade D6):
// Success é SEMPRE igual a exitCode == 0, para todos os construtores.
func TestSuccessDerivedFromExitCode(t *testing.T) {
	all := []Result{
		OK(nil),
		Fail(1, "a"),
		Fail(0, "b"),
		Degraded("c"),
	}
	for _, r := range all {
		assert.Equal(t, r.ExitCode == 0, r.Success,
			"Success deve ser derivado de exitCode==0")
	}
}

// TestDegradedNeverError verifica que degradado nunca vira error sem
// exitCode == 0 (um resultado degradado não é uma falha).
func TestDegradedNeverError(t *testing.T) {
	r := Degraded("parcial")
	assert.True(t, r.Success)
	assert.Equal(t, 0, r.ExitCode)
	assert.Nil(t, r.Err(), "degraded não deve produzir error na fronteira")
	assert.False(t, r.WasError())
}

func TestJSONShape(t *testing.T) {
	r := Degraded("cache miss")
	got, err := json.Marshal(r)
	require.NoError(t, err)
	assert.JSONEq(t, `{"success":true,"degraded":true,"exitCode":0,"reason":"cache miss"}`, string(got))

	ok := OK(map[string]string{"x": "y"})
	got2, err := json.Marshal(ok)
	require.NoError(t, err)
	assert.JSONEq(t, `{"success":true,"data":{"x":"y"},"degraded":false,"exitCode":0}`, string(got2))

	fail := Fail(3, "negado")
	got3, err := json.Marshal(fail)
	require.NoError(t, err)
	assert.JSONEq(t, `{"success":false,"degraded":false,"exitCode":3,"reason":"negado"}`, string(got3))
}
