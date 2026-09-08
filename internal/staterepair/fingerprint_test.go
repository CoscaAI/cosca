// fingerprint_test.go — estabilidade do fingerprint sob escrita viva simulada
// (critério de aceite 1 do ADR-043 §8).
package staterepair

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestFingerprint_StableUnderVolatileHeaderMutation prova que mutar SOMENTE os
// bytes voláteis do header SQLite (24-28 change counter e 92-96
// version-valid-for — o que um COMMIT/escrita viva faz) NÃO muda o
// fingerprint: a escrita viva não "rearma" o ledger.
func TestFingerprint_StableUnderVolatileHeaderMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.db")
	seedSQLite(t, path)

	fp1, err := Fingerprint(path)
	require.NoError(t, err)

	// Simula um COMMIT de escrita viva: altera os dois ranges voláteis.
	corruptFileBytes(t, path, 24, []byte{0x11, 0x22, 0x33, 0x44})
	corruptFileBytes(t, path, 92, []byte{0xAA, 0xBB, 0xCC, 0xDD})

	fp2, err := Fingerprint(path)
	require.NoError(t, err)
	require.Equal(t, fp1, fp2, "escrita viva (ranges voláteis do header) não pode mudar o fingerprint")
}

// TestFingerprint_ChangesOnHeadContentMutation prova que mutar CONTEÚDO dentro
// da amostra head (64 KiB) muda o fingerprint.
func TestFingerprint_ChangesOnHeadContentMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "head.db")
	seedSQLite(t, path)

	fp1, err := Fingerprint(path)
	require.NoError(t, err)

	// Conteúdo na janela head (depois do header de 100 bytes, antes de 64 KiB).
	corruptFileBytes(t, path, 10_000, []byte{0xDE, 0xAD, 0xBE, 0xEF})

	fp2, err := Fingerprint(path)
	require.NoError(t, err)
	require.NotEqual(t, fp1, fp2, "mutação de conteúdo na amostra head deve mudar o fingerprint")
}

// TestFingerprint_ChangesOnTailContentMutation prova que mutar CONTEÚDO na
// amostra tail (últimos 4 KiB de um arquivo > 64 KiB) muda o fingerprint.
func TestFingerprint_ChangesOnTailContentMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tail.db")
	seedSQLite(t, path)

	fp1, err := Fingerprint(path)
	require.NoError(t, err)

	fi, err := os.Stat(path)
	require.NoError(t, err)
	// Último byte do arquivo — dentro da janela tail de 4 KiB.
	corruptFileBytes(t, path, fi.Size()-1, []byte{0x99})

	fp2, err := Fingerprint(path)
	require.NoError(t, err)
	require.NotEqual(t, fp1, fp2, "mutação de conteúdo na amostra tail deve mudar o fingerprint")
}

// TestFingerprint_Deterministic prova que o fingerprint é determinístico para
// o mesmo arquivo (e inclui o tamanho como prefixo).
func TestFingerprint_Deterministic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "det.db")
	seedSQLite(t, path)

	fp1, err := Fingerprint(path)
	require.NoError(t, err)
	fp2, err := Fingerprint(path)
	require.NoError(t, err)
	require.Equal(t, fp1, fp2)

	fi, err := os.Stat(path)
	require.NoError(t, err)
	prefix := fmt.Sprintf("%d:", fi.Size())
	require.Contains(t, fp1, prefix, "fingerprint deve carregar o tamanho como prefixo")
}

// TestFingerprint_EmptyFile devolve uma identidade determinística sem erro.
func TestFingerprint_EmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.db")
	require.NoError(t, os.WriteFile(path, nil, 0o600))

	fp1, err := Fingerprint(path)
	require.NoError(t, err)
	fp2, err := Fingerprint(path)
	require.NoError(t, err)
	require.Equal(t, fp1, fp2)
	require.Contains(t, fp1, "0:")
}

// TestFingerprint_MissingFile devolve erro.
func TestFingerprint_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nao-existe.db")
	_, err := Fingerprint(path)
	require.Error(t, err)
}
