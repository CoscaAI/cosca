// backup.go — backup forense deduplicado por conteúdo.
//
// ForensicBackup copia o arquivo doente (bytes crus — o banco não abre
// limpo, então a cópia precisa preservar exatamente o que está no disco)
// para `.cosca/backups/staterepair/<nome>-<ts>-<hash8>.db`.
//
// Lições do incidente Hermes (#86747) adaptadas:
//   - DEDUPE por CONTEÚDO (hash sha256 do arquivo inteiro): um arquivo doente
//     idêntico NUNCA é copiado 2x (um loop de repair já copiou 105x o mesmo
//     arquivo — 89 GB de cópias mortas);
//   - RETENÇÃO: no máximo MaxBackups (3) cópias forenses por banco (nome);
//     as mais velhas são removidas.
//
// A identidade de conteúdo é DIFERENTE do Fingerprint (head+tail mascarado):
// um writer vivo pode commitar numa página interior preservando tamanho e as
// duas amostras — o dedupe de backup precisa de identidade BYTE a byte do
// arquivo inteiro para nunca reusar uma cópia que pré-data dados reais.
package staterepair

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MaxBackups é o número máximo de cópias forenses mantidas por banco.
const MaxBackups = 3

// backupTimeLayout é o timestamp do nome do backup (largura fixa → a ordenação
// lexicográfica do nome coincide com a cronológica).
const backupTimeLayout = "20060102_150405"

// coscaRootOf descobre o diretório raiz `.cosca` de um caminho de banco
// (subindo os ancestrais). Quando o arquivo não vive sob um `.cosca`
// (ex.: teste em t.TempDir()), devolve o próprio diretório do arquivo —
// determinístico e seguro para ambientes isolados.
func coscaRootOf(dbPath string) string {
	dir := filepath.Dir(dbPath)
	if filepath.Base(dir) == ".cosca" {
		return dir
	}
	for d := dir; ; d = filepath.Dir(d) {
		if filepath.Base(d) == ".cosca" {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
	}
	return dir
}

// backupsDir devolve `.cosca/backups/staterepair`.
func backupsDir(dbPath string) string {
	return filepath.Join(coscaRootOf(dbPath), "backups", "staterepair")
}

// stemOf devolve o nome-base sem extensão (ex.: "knowledge.db" → "knowledge",
// "vector-code.db" → "vector-code", "memory/index.db" → "index").
func stemOf(dbPath string) string {
	base := filepath.Base(dbPath)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// contentHash devolve o sha256 hex do arquivo inteiro (identidade byte a byte).
func contentHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("abrir para hash: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hashear conteúdo: %w", err)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// backupName monta `<nome>-<ts>-<hash8>.db` com colisão de mesmo segundo
// resolvida por sufixo sequencial.
func backupName(stem, ts, hash8 string, existing map[string]bool) string {
	base := fmt.Sprintf("%s-%s-%s.db", stem, ts, hash8)
	name := base
	for seq := 1; existing[name]; seq++ {
		name = fmt.Sprintf("%s-%s_%d-%s.db", stem, ts, seq, hash8)
	}
	return name
}

// existingBackups lista as cópias forenses do stem (as que terminam em .db),
// ordenadas da mais nova para a mais velha. A ordem usa o mtime do arquivo
// (com desempate pelo nome): a retenção "remove as mais velhas" é cronológica
// de verdade, não lexical.
func existingBackups(dir, stem string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	prefix := stem + "-"
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".db") {
			continue
		}
		out = append(out, filepath.Join(dir, name))
	}
	sort.Slice(out, func(i, j int) bool {
		mi, ei := os.Stat(out[i])
		mj, ej := os.Stat(out[j])
		if ei == nil && ej == nil && !mi.ModTime().Equal(mj.ModTime()) {
			return mi.ModTime().After(mj.ModTime()) // mais nova primeiro
		}
		return out[i] > out[j] // desempate: nome (mais novo primeiro)
	})
	return out
}

// pruneBackups mantém apenas as MaxBackups cópias mais novas do stem.
func pruneBackups(dir, stem string) {
	all := existingBackups(dir, stem)
	if len(all) <= MaxBackups {
		return
	}
	for _, stale := range all[MaxBackups:] {
		_ = os.Remove(stale)
	}
}

// ForensicBackup copia o arquivo doente para `.cosca/backups/staterepair/` com
// dedupe por conteúdo e retenção máxima de MaxBackups por banco.
//
// Devolve o caminho do backup criado (ou reusado no dedupe). NUNCA copia 2x o
// mesmo conteúdo: antes de copiar, compara o sha256 do arquivo inteiro com o
// das cópias existentes do mesmo nome (mesmo tamanho). Um backup reusado
// continua contando para a retenção.
func ForensicBackup(sickPath string) (string, error) {
	dir := backupsDir(sickPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("criar diretório de backup %s: %w", dir, err)
	}

	stem := stemOf(sickPath)

	// Identidade de conteúdo do arquivo doente (byte a byte).
	srcHash, err := contentHash(sickPath)
	if err != nil {
		return "", fmt.Errorf("conteúdo de %s: %w", sickPath, err)
	}
	srcInfo, err := os.Stat(sickPath)
	if err != nil {
		return "", fmt.Errorf("stat de %s: %w", sickPath, err)
	}

	// Dedupe: reusa a cópia existente byte-idêntica (mesmo tamanho + mesmo
	// sha256). Só hasheia candidatos do MESMO tamanho — hashing de um arquivo
	// grande antes de copiar seria desperdício.
	for _, cand := range existingBackups(dir, stem) {
		ci, statErr := os.Stat(cand)
		if statErr != nil || ci.Size() != srcInfo.Size() {
			continue
		}
		candHash, hashErr := contentHash(cand)
		if hashErr != nil {
			continue
		}
		if candHash == srcHash {
			pruneBackups(dir, stem)
			return cand, nil
		}
	}

	ts := time.Now().UTC().Format(backupTimeLayout)
	existing := map[string]bool{}
	for _, b := range existingBackups(dir, stem) {
		existing[filepath.Base(b)] = true
	}
	dest := filepath.Join(dir, backupName(stem, ts, srcHash[:8], existing))

	if err := copyFile(sickPath, dest); err != nil {
		return "", fmt.Errorf("copiar backup forense: %w", err)
	}
	pruneBackups(dir, stem)
	return dest, nil
}

// copyFile copia src → dst preservando bytes crus (0600, privado como o resto
// do runtime do Cosca).
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return err
	}
	return out.Close()
}
