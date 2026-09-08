// ledger_test.go — ledger de tentativas: recusa após 3 falhas no mesmo
// fingerprint, aceita fingerprint diferente (critério de aceite 2 do
// ADR-043 §8).
package staterepair

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// ledgerTestPath monta um dbPath isolado por teste (ledger sidecar colateral).
func ledgerTestPath(t *testing.T) string {
	return filepath.Join(t.TempDir(), "session.db")
}

// TestLedger_ExhaustedAfterThreeFailuresSameFingerprint prova que 3 falhas do
// MESMO fingerprint esgotam o orçamento — a quarta cirurgia é recusada.
func TestLedger_ExhaustedAfterThreeFailuresSameFingerprint(t *testing.T) {
	dbPath := ledgerTestPath(t)
	fp := "123:abcdef"

	require.False(t, AttemptsExhausted(dbPath, fp))

	require.NoError(t, RecordFailure(dbPath, fp))
	require.NoError(t, RecordFailure(dbPath, fp))
	require.False(t, AttemptsExhausted(dbPath, fp), "2 falhas < MaxAttempts")

	require.NoError(t, RecordFailure(dbPath, fp))
	require.True(t, AttemptsExhausted(dbPath, fp), "3 falhas no mesmo fingerprint devem esgotar")

	entries := ReadAttempts(dbPath)
	require.Equal(t, 3, entries[fp].Attempts)
	require.Equal(t, statusFailed, entries[fp].LastStatus)
	require.Equal(t, dbPath, entries[fp].Path)
}

// TestLedger_NewFingerprintResetsBudget prova que um fingerprint DIFERENTE
// (o arquivo mudou — truncamento/restauração real) recomeça o orçamento: o
// ledger aceita nova cirurgia para o arquivo novo.
func TestLedger_NewFingerprintResetsBudget(t *testing.T) {
	dbPath := ledgerTestPath(t)
	fpA := "100:aaaa"
	fpB := "200:bbbb"

	for i := 0; i < MaxAttempts; i++ {
		require.NoError(t, RecordFailure(dbPath, fpA))
	}
	require.True(t, AttemptsExhausted(dbPath, fpA))

	// Fingerprint novo → orçamento recomeça em 1 e NÃO está exausto.
	require.NoError(t, RecordFailure(dbPath, fpB))
	require.False(t, AttemptsExhausted(dbPath, fpB), "fingerprint diferente recomeça o orçamento")

	entries := ReadAttempts(dbPath)
	require.Equal(t, 3, entries[fpA].Attempts, "histórico do fingerprint antigo preservado")
	require.Equal(t, 1, entries[fpB].Attempts)
}

// TestLedger_RecordSuccessRemovesEntry prova que um repair bem-sucedido limpa
// a entrada do fingerprint (idempotente) e remove o sidecar quando vazio.
func TestLedger_RecordSuccessRemovesEntry(t *testing.T) {
	dbPath := ledgerTestPath(t)
	fp := "100:aaaa"

	require.NoError(t, RecordFailure(dbPath, fp))
	require.NoError(t, RecordFailure(dbPath, fp))
	require.False(t, AttemptsExhausted(dbPath, fp), "2 falhas < MaxAttempts")
	require.NoError(t, RecordFailure(dbPath, fp))
	require.True(t, AttemptsExhausted(dbPath, fp))

	require.NoError(t, RecordSuccess(dbPath, fp))
	require.False(t, AttemptsExhausted(dbPath, fp), "sucesso limpa o histórico de falhas")
	assertFileMissing(t, LedgerPath(dbPath))

	// Idempotente: segundo RecordSuccess não falha e nada recria.
	require.NoError(t, RecordSuccess(dbPath, fp))
	assertFileMissing(t, LedgerPath(dbPath))
}

// TestLedger_SidecarPathAndFormat prova o contrato de localização/nome do
// sidecar: `<db>.repair-attempts.json` com JSON parseável.
func TestLedger_SidecarPathAndFormat(t *testing.T) {
	dbPath := ledgerTestPath(t)
	require.Equal(t, dbPath+".repair-attempts.json", LedgerPath(dbPath))

	require.NoError(t, RecordFailure(dbPath, "1:fp"))

	raw, err := os.ReadFile(LedgerPath(dbPath))
	require.NoError(t, err)
	require.Contains(t, string(raw), `"1:fp"`)
}

// TestLedger_CorruptLedgerReadsAsEmpty prova que um sidecar corrompido NUNCA
// bloqueia o repair: lê como "não exausto" (regra do Hermes — best effort).
func TestLedger_CorruptLedgerReadsAsEmpty(t *testing.T) {
	dbPath := ledgerTestPath(t)
	require.NoError(t, os.WriteFile(LedgerPath(dbPath), []byte("{json inválido!!!"), 0o600))

	require.False(t, AttemptsExhausted(dbPath, "qualquer:fp"))
	entries := ReadAttempts(dbPath)
	require.Empty(t, entries)
}

// TestLedger_EmptyFingerprintIsIgnored prova que chamadas com fingerprint vazio
// são no-ops seguros (sem identidade não existe chave).
func TestLedger_EmptyFingerprintIsIgnored(t *testing.T) {
	dbPath := ledgerTestPath(t)
	require.NoError(t, RecordFailure(dbPath, ""))
	require.NoError(t, RecordSuccess(dbPath, ""))
	assertFileMissing(t, LedgerPath(dbPath))
}
