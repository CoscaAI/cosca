// fingerprint.go — impressão digital do arquivo doente.
//
// A identidade do arquivo para o ledger de tentativas é (tamanho + amostra de
// conteúdo) com os ranges voláteis do header SQLite MASCARADOS antes do hash.
// Adaptação direta de `hermes_state_repair.py::_db_fingerprint`:
//
//   - amostra "head": primeiros 64 KiB do arquivo (contém o header de 100
//     bytes do SQLite) com os ranges voláteis zerados;
//   - amostra "tail": últimos 4 KiB do arquivo (quando ele tem mais de 64 KiB);
//   - hash = sha256(tamanho + head mascarado + tail).
//
// Ranges voláteis do header SQLite (mudam em COMMIT/checkpoint comum, não em
// repair): bytes 24-28 (change counter / file change counter) e 92-96
// (version-valid-for). Sem a máscara, qualquer escrita viva re-chaveia o
// ledger e o orçamento de repair (3 falhas no mesmo fingerprint) resetaria
// para sempre. mtime é EXCLUÍDO de propósito (a mesma razão do Hermes).
package staterepair

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

const (
	// headSampleBytes é o tamanho da amostra inicial do arquivo (64 KiB),
	// incluindo o header SQLite de 100 bytes com os ranges voláteis mascarados.
	headSampleBytes = 64 * 1024
	// tailSampleBytes é o tamanho da amostra final do arquivo (últimos ~4 KiB).
	tailSampleBytes = 4 * 1024
)

// volatileHeaderRanges são os intervalos [start,end) do header SQLite zerados
// antes do hash — escrita viva (WAL ativo) não pode rearmar o ledger.
var volatileHeaderRanges = [][2]int{
	{24, 28}, // file change counter
	{92, 96}, // version-valid-for
}

// maskVolatileHeader zera os bytes voláteis do header no início da amostra.
// Best-effort: arquivos menores que o range são devolvidos intactos (o hash
// cobre o que existe — um arquivo truncado é uma identidade diferente).
func maskVolatileHeader(head []byte) []byte {
	if len(head) < 96 {
		return head
	}
	buf := append([]byte(nil), head...)
	for _, r := range volatileHeaderRanges {
		start, end := r[0], r[1]
		if end > len(buf) {
			end = len(buf)
		}
		if start >= len(buf) {
			continue
		}
		for i := start; i < end; i++ {
			buf[i] = 0
		}
	}
	return buf
}

// readN lê até n bytes do offset atual. Tolerante a EOF (arquivo pode encolher
// entre o Stat e a leitura — o que mudaria o tamanho e, por consequência, o
// fingerprint).
func readN(f *os.File, n int) ([]byte, error) {
	buf := make([]byte, n)
	total := 0
	for total < n {
		m, err := f.Read(buf[total:])
		total += m
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
	}
	return buf[:total], nil
}

// Fingerprint devolve a identidade do arquivo doente no formato
// "<tamanho>:<sha256 hex>". O hash cobre: tamanho em bytes + amostra head
// (64 KiB com header volátil mascarado) + amostra tail (últimos 4 KiB, quando
// o arquivo tem mais que 64 KiB).
//
// A identidade é barata O(1) no tamanho do arquivo (nunca hasheia um banco
// multi-GB inteiro por open) e estável sob escrita viva: um COMMIT real muda
// apenas os bytes mascarados/amostras não cobertas — nunca o fingerprint. Uma
// mudança real (truncamento, restauração, repair) muda o tamanho ou as
// amostras e re-chaveia o ledger.
func Fingerprint(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("abrir %s para fingerprint: %w", path, err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", path, err)
	}
	size := fi.Size()

	headLen := int(headSampleBytes)
	if size < int64(headLen) {
		headLen = int(size)
	}
	head, err := readN(f, headLen)
	if err != nil {
		return "", fmt.Errorf("ler amostra head de %s: %w", path, err)
	}

	var tail []byte
	if size > int64(headSampleBytes) {
		tailLen := int64(tailSampleBytes)
		if size-int64(headSampleBytes) < tailLen {
			tailLen = size - int64(headSampleBytes)
		}
		if _, err := f.Seek(size-tailLen, io.SeekStart); err != nil {
			return "", fmt.Errorf("seek tail de %s: %w", path, err)
		}
		tail, err = readN(f, int(tailLen))
		if err != nil {
			return "", fmt.Errorf("ler amostra tail de %s: %w", path, err)
		}
	}

	h := sha256.New()
	fmt.Fprintf(h, "%d:", size)
	h.Write(maskVolatileHeader(head))
	h.Write(tail)
	return fmt.Sprintf("%d:%x", size, h.Sum(nil)), nil
}
