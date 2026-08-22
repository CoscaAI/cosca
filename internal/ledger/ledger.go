// Package ledger implementa o "Caderno da Família" — um armazenamento
// chave-valor que satisfaz SIMULTANEAMENTE oito propriedades que à primeira
// vista parecem conflitantes, sob restrições determinadas de memória, tempo e
// segurança. É a materialização da parábola L135: o valor não está no carro,
// está no caderno que sobrevive a ele.
//
// As oito propriedades (P1..P8):
//
//	P1 Durabilidade   — WAL com fsync + snapshot atômico (rename). Sobrevive a
//	                    crash/reboot: no Open, replay reconstrói o estado.
//	P2 Consistência   — MVCC com compare-and-swap de versão por chave
//	                    (Put/Delete exigem expectedVersion). Escrita perdida é
//	                    impossível: conflito vira erro, nunca sobrescrita cega.
//	P3 Concorrência   — leitores concorrentes (RWMutex); escrita serializada,
//	                    sem lock global de processo para leitura.
//	P4 Latência       — leitura pontual O(1) via hash index em memória
//	                    (map). Escrita O(1) amortizada.
//	P5 Memória        — o log vive no disco e é limitado por compactação; a
//	                    memória só guarda o estado vivo (chave→valor+versão) e
//	                    um índice invertido de tokens.
//	P6 Auditabilidade — append-only, replay-proof: a sequência (seq) é global e
//	                    monotônica, nunca reutilizada; remoção é tombstone
//	                    (registro), não apagamento da história.
//	P7 Buscabilidade  — índice invertido (token→chaves) mantido junto ao estado
//	                    vivo; Search(query) cruza os tokens.
//	P8 Integridade    — cadeia de hash SHA-256: cada registro commita o hash do
//	                    anterior; a compactação ancora a cadeia na raiz do
//	                    snapshot. Verify() detecta qualquer adulteração.
//
// O conflito é resolvido por uma estrutura única: um WAL (write-ahead log)
// encadeado por hash + snapshot periódico + hash index O(1) + MVCC por versão.
// É a mesma arquitetura dos LSM reais, acrescida da cadeia de integridade e do
// CAS de versão.
package ledger

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Op é a operação de um registro do caderno.
type Op uint8

const (
	// OpPut grava (ou sobrescreve, via CAS) uma chave.
	OpPut Op = 1
	// OpDel remove uma chave (tombstone — a história não é apagada).
	OpDel Op = 2
)

func opString(o Op) string {
	if o == OpDel {
		return "del"
	}
	return "put"
}

func parseOp(s string) (Op, error) {
	switch s {
	case "put":
		return OpPut, nil
	case "del":
		return OpDel, nil
	}
	return 0, fmt.Errorf("ledger: op desconhecida %q", s)
}

// SyncPolicy controla quando o WAL é sincronizado no disco (durabilidade ×
// latência).
type SyncPolicy uint8

const (
	// SyncEvery faz fsync a cada escrita (durabilidade máxima, latência maior).
	SyncEvery SyncPolicy = iota
	// SyncManual adia o fsync: o chamador invoca Sync() no ponto de durabilidade
	// (group commit). Reconcilia P1 com P4 sem pagar fsync por escrita.
	SyncManual
)

// ErrConflict é devolvido quando o expectedVersion não bate com a versão atual
// da chave — a escrita perdida foi impedida (P2).
var ErrConflict = errors.New("ledger: conflito de versão (escrita perdida impedida)")

// maxFrameSize limita o tamanho de um frame do WAL (guarda contra corrupção).
const maxFrameSize = 64 << 20 // 64 MiB

// Cell é o valor vivo de uma chave no memtable.
type Cell struct {
	Value   []byte `json:"value"`
	Version uint64 `json:"version"`
}

// Record é um registro imutável do caderno (uma linha do WAL).
type Record struct {
	Seq      uint64   // sequência global monotônica (ordem total)
	Op       Op       // operação
	Key      string   // chave
	Value    []byte   // valor (vazio em delete)
	Version  uint64   // versão POR CHAVE após esta escrita (CAS)
	PrevHash [32]byte // hash do registro anterior (ou raiz do snapshot)
	Hash     [32]byte // hash deste registro (SHA-256)
	TS       int64    // timestamp (unix nano)
}

// hashInput é a serialização canônica (determinística) que entra no hash. É
// independente da representação JSON do WAL: o mesmo registro produz os mesmos
// bytes, logo o mesmo hash, em qualquer replay/verify.
func (r Record) hashInput() []byte {
	buf := make([]byte, 0, 64+len(r.Key)+len(r.Value))
	buf = binary.BigEndian.AppendUint64(buf, r.Seq)
	buf = append(buf, byte(r.Op))
	buf = binary.BigEndian.AppendUint32(buf, uint32(len(r.Key)))
	buf = append(buf, r.Key...)
	buf = binary.BigEndian.AppendUint32(buf, uint32(len(r.Value)))
	buf = append(buf, r.Value...)
	buf = binary.BigEndian.AppendUint64(buf, r.Version)
	buf = append(buf, r.PrevHash[:]...)
	buf = binary.BigEndian.AppendUint64(buf, uint64(r.TS))
	return buf
}

func (r Record) computeHash() [32]byte {
	return sha256.Sum256(r.hashInput())
}

// wireRecord é a forma serializada no WAL (JSON, legível e debuggável, à la
// coluna JSON do gate). Hashes em hex; Value em base64 automático.
type wireRecord struct {
	Seq     uint64 `json:"seq"`
	Op      string `json:"op"`
	Key     string `json:"key"`
	Value   []byte `json:"value,omitempty"`
	Version uint64 `json:"version"`
	Prev    string `json:"prev"`
	Hash    string `json:"hash"`
	TS      int64  `json:"ts"`
}

func recordToWire(r Record) wireRecord {
	return wireRecord{
		Seq:     r.Seq,
		Op:      opString(r.Op),
		Key:     r.Key,
		Value:   r.Value,
		Version: r.Version,
		Prev:    hex.EncodeToString(r.PrevHash[:]),
		Hash:    hex.EncodeToString(r.Hash[:]),
		TS:      r.TS,
	}
}

func wireToRecord(w wireRecord) (Record, error) {
	op, err := parseOp(w.Op)
	if err != nil {
		return Record{}, err
	}
	prev, err := hex.DecodeString(w.Prev)
	if err != nil || len(prev) != 32 {
		return Record{}, fmt.Errorf("ledger: prev hash inválido no seq %d", w.Seq)
	}
	hash, err := hex.DecodeString(w.Hash)
	if err != nil || len(hash) != 32 {
		return Record{}, fmt.Errorf("ledger: hash inválido no seq %d", w.Seq)
	}
	var r Record
	r.Seq = w.Seq
	r.Op = op
	r.Key = w.Key
	r.Value = w.Value
	r.Version = w.Version
	r.TS = w.TS
	copy(r.PrevHash[:], prev)
	copy(r.Hash[:], hash)
	return r, nil
}

// Snapshot é o checkpoint imutável gravado na compactação. A RootHash ancora a
// cadeia de hash: commita a sequência, o hash anterior e o estado completo.
type Snapshot struct {
	Seq      uint64          `json:"seq"`
	PrevHash string          `json:"prev"` // hash do último registro antes da compactação
	RootHash string          `json:"root"`
	State    map[string]Cell `json:"state"`
}

// snapshotHash computa a raiz do snapshot de forma canônica (chaves ordenadas),
// ancorando-a ao hash anterior para continuidade da cadeia através da
// compactação.
func snapshotHash(seq uint64, prevHash [32]byte, state map[string]Cell) [32]byte {
	h := sha256.New()
	var b8 [8]byte
	binary.BigEndian.PutUint64(b8[:], seq)
	h.Write(b8[:])
	h.Write(prevHash[:])

	keys := make([]string, 0, len(state))
	for k := range state {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b4 [4]byte
	for _, k := range keys {
		c := state[k]
		binary.BigEndian.PutUint32(b4[:], uint32(len(k)))
		h.Write(b4[:])
		h.Write([]byte(k))
		binary.BigEndian.PutUint32(b4[:], uint32(len(c.Value)))
		h.Write(b4[:])
		h.Write(c.Value)
		binary.BigEndian.PutUint64(b8[:], c.Version)
		h.Write(b8[:])
	}

	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// Option configura um Ledger.
type Option func(*Ledger)

// WithMaxLogBytes define o teto do WAL; ao atingi-lo a compactação dispara
// automaticamente (best-effort). <= 0 desativa a compactação automática.
func WithMaxLogBytes(n int64) Option {
	return func(l *Ledger) { l.maxLogBytes = n }
}

// WithSyncPolicy define a política de durabilidade (SyncEvery ou SyncManual).
func WithSyncPolicy(p SyncPolicy) Option {
	return func(l *Ledger) { l.syncPolicy = p }
}

// Ledger é o caderno: um store chave-valor durável, com MVCC, append-only,
// memória-limitada, buscável e tamper-evidente.
type Ledger struct {
	mu  sync.RWMutex
	dir string

	wal     *os.File
	walSize int64

	seq      uint64   // último seq aplicado
	lastHash [32]byte // hash do último registro (ou raiz do snapshot)

	snapSeq  uint64   // seq coberto pelo último snapshot
	snapRoot [32]byte // raiz do último snapshot (âncora da cadeia)

	cells map[string]Cell                 // estado vivo: chave → valor+versão (memtable)
	inv   map[string]map[string]struct{}  // índice invertido: token → chaves

	syncPolicy  SyncPolicy
	maxLogBytes int64
	compactErr  error // último erro de compactação automática (não-fatal)
}

// Open abre (ou cria) o caderno no diretório dado, carrega o snapshot e faz
// replay do WAL verificando a cadeia de hash (recuperação + integridade).
func Open(dir string, opts ...Option) (*Ledger, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("ledger: criar diretório: %w", err)
	}

	l := &Ledger{
		dir:        dir,
		cells:      map[string]Cell{},
		inv:        map[string]map[string]struct{}{},
		syncPolicy: SyncEvery,
	}
	for _, o := range opts {
		o(l)
	}

	if err := l.loadSnapshot(); err != nil {
		return nil, err
	}

	walPath := filepath.Join(dir, "ledger.wal")
	f, err := os.OpenFile(walPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("ledger: abrir WAL: %w", err)
	}
	l.wal = f

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("ledger: seek WAL: %w", err)
	}
	if err := l.replay(bufio.NewReader(f)); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("ledger: replay do WAL: %w", err)
	}
	if fi, err := f.Stat(); err == nil {
		l.walSize = fi.Size()
	}
	return l, nil
}

// Put grava (ou sobrescreve) uma chave com CAS de versão. expectedVersion é a
// versão que o chamador leu por último (0 para chave nova). Em sucesso devolve
// a nova versão; em conflito devolve ErrConflict e NÃO grava nada (P2).
func (l *Ledger) Put(key string, value []byte, expectedVersion uint64) (uint64, error) {
	if key == "" {
		return 0, fmt.Errorf("ledger: chave vazia")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	cur, ok := l.cells[key]
	if ok && cur.Version != expectedVersion {
		return 0, fmt.Errorf("%w: %q esperava v%d, tem v%d", ErrConflict, key, expectedVersion, cur.Version)
	}
	if !ok && expectedVersion != 0 {
		return 0, fmt.Errorf("%w: %q não existe (esperava v0)", ErrConflict, key)
	}

	newVersion := uint64(1)
	if ok {
		newVersion = cur.Version + 1
	}

	rec := Record{
		Seq:      l.seq + 1,
		Op:       OpPut,
		Key:      key,
		Value:    append([]byte(nil), value...),
		Version:  newVersion,
		PrevHash: l.lastHash,
		TS:       time.Now().UnixNano(),
	}
	rec.Hash = rec.computeHash()

	if err := l.append(rec); err != nil {
		return 0, err
	}
	l.seq++
	l.lastHash = rec.Hash
	l.apply(rec)
	l.maybeCompactLocked()
	return newVersion, nil
}

// Delete remove uma chave com CAS de versão (tombstone: a história permanece
// no WAL). Devolve ErrConflict se a versão não bater.
func (l *Ledger) Delete(key string, expectedVersion uint64) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	cur, ok := l.cells[key]
	if !ok {
		return fmt.Errorf("ledger: chave %q não encontrada", key)
	}
	if cur.Version != expectedVersion {
		return fmt.Errorf("%w: %q esperava v%d, tem v%d", ErrConflict, key, expectedVersion, cur.Version)
	}

	rec := Record{
		Seq:      l.seq + 1,
		Op:       OpDel,
		Key:      key,
		Version:  cur.Version + 1,
		PrevHash: l.lastHash,
		TS:       time.Now().UnixNano(),
	}
	rec.Hash = rec.computeHash()

	if err := l.append(rec); err != nil {
		return err
	}
	l.seq++
	l.lastHash = rec.Hash
	l.apply(rec)
	l.maybeCompactLocked()
	return nil
}

// Get devolve o valor e a versão atuais da chave (O(1), P4). ok=false se a
// chave não existe.
func (l *Ledger) Get(key string) (value []byte, version uint64, ok bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	c, ok := l.cells[key]
	if !ok {
		return nil, 0, false
	}
	return append([]byte(nil), c.Value...), c.Version, true
}

// Search devolve as chaves cujo conteúdo (chave+valor) contém TODOS os tokens
// da query (AND), em ordem lexicográfica (P7).
func (l *Ledger) Search(query string) []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	tokens := tokenize(query)
	if len(tokens) == 0 {
		return nil
	}

	var inter map[string]struct{}
	for i, t := range tokens {
		keys := l.inv[t]
		if i == 0 {
			inter = make(map[string]struct{}, len(keys))
			for k := range keys {
				inter[k] = struct{}{}
			}
		} else {
			for k := range inter {
				if _, ok := keys[k]; !ok {
					delete(inter, k)
				}
			}
		}
		if len(inter) == 0 {
			return nil
		}
	}

	out := make([]string, 0, len(inter))
	for k := range inter {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Compact grava um snapshot do estado vivo, ancora a cadeia na raiz do
// snapshot e trunca o WAL (P5: log limitado; P1: checkpoint durável). A
// continuidade da cadeia é preservada via snapshot.PrevHash.
func (l *Ledger) Compact() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.compactLocked()
}

// Sync força o fsync do WAL (relevante em SyncManual — group commit).
func (l *Ledger) Sync() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.wal.Sync()
}

// Verify relê o WAL do disco e confere a cadeia de hash do início ao fim,
// ancorada na raiz do snapshot. Detecta qualquer adulteração (P8). Também
// confere que a cabeça da cadeia bate com o estado em memória.
func (l *Ledger) Verify() error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	f, err := os.Open(filepath.Join(l.dir, "ledger.wal"))
	if err != nil {
		return fmt.Errorf("ledger: verify: abrir WAL: %w", err)
	}
	defer f.Close()

	br := bufio.NewReader(f)
	running := l.snapRoot
	expectSeq := l.snapSeq

	for {
		data, err := readFrame(br)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("ledger: verify: ler frame: %w", err)
		}
		var w wireRecord
		if err := json.Unmarshal(data, &w); err != nil {
			return fmt.Errorf("ledger: verify: decodificar: %w", err)
		}
		rec, err := wireToRecord(w)
		if err != nil {
			return err
		}
		// Registro já coberto pelo snapshot (mesma janela de crash do
		// replay): pular em vez de falhar.
		if rec.Seq <= l.snapSeq {
			continue
		}
		expectSeq++
		if rec.Seq != expectSeq {
			return fmt.Errorf("ledger: verify: seq fora de ordem (esperava %d, veio %d)", expectSeq, rec.Seq)
		}
		if rec.PrevHash != running {
			return fmt.Errorf("ledger: verify: cadeia quebrada no seq %d", rec.Seq)
		}
		if rec.Hash != rec.computeHash() {
			return fmt.Errorf("ledger: verify: hash divergente no seq %d (ADULTERADO)", rec.Seq)
		}
		running = rec.Hash
	}

	if running != l.lastHash {
		return fmt.Errorf("ledger: verify: cabeça da cadeia diverge da memória")
	}
	return nil
}

// Stats expõe métricas observáveis do caderno.
type Stats struct {
	Keys        int
	Seq         uint64
	WALBytes    int64
	SnapshotSeq uint64
	CompactErr  error
}

// Stats devolve um instantâneo das métricas.
func (l *Ledger) Stats() Stats {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return Stats{
		Keys:        len(l.cells),
		Seq:         l.seq,
		WALBytes:    l.walSize,
		SnapshotSeq: l.snapSeq,
		CompactErr:  l.compactErr,
	}
}

// Close sincroniza (se preciso) e fecha o caderno.
func (l *Ledger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.syncPolicy != SyncEvery {
		if err := l.wal.Sync(); err != nil {
			return err
		}
	}
	return l.wal.Close()
}

// ---------------------------------------------------------------------------
// Internals
// ---------------------------------------------------------------------------

// append grava um registro no WAL como frame [4-byte len][json]\n e faz o
// fsync conforme a política. A compactação automática é acionada pelo chamador
// via maybeCompactLocked, DEPOIS de o registro ser aplicado ao estado — assim o
// snapshot que dispara a compactação já contém o efeito do registro.
func (l *Ledger) append(rec Record) error {
	data, err := json.Marshal(recordToWire(rec))
	if err != nil {
		return fmt.Errorf("ledger: serializar registro: %w", err)
	}

	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(data)))
	if _, err := l.wal.Write(lenBuf[:]); err != nil {
		return err
	}
	if _, err := l.wal.Write(data); err != nil {
		return err
	}
	if _, err := l.wal.Write([]byte{'\n'}); err != nil {
		return err
	}
	l.walSize += int64(4 + len(data) + 1)

	if l.syncPolicy == SyncEvery {
		if err := l.wal.Sync(); err != nil {
			return err
		}
	}
	return nil
}

// maybeCompactLocked dispara a compactação quando o log atinge o teto
// (best-effort, nunca fatal: a correção não depende dela).
func (l *Ledger) maybeCompactLocked() {
	if l.maxLogBytes > 0 && l.walSize >= l.maxLogBytes {
		if err := l.compactLocked(); err != nil {
			l.compactErr = err // não-fatal: o WAL continua correto e durável
		}
	}
}

// apply atualiza o estado vivo (memtable + índice invertido) a partir de um
// registro já persistido.
func (l *Ledger) apply(rec Record) {
	old := l.cells[rec.Key]
	oldValue := old.Value

	var newValue []byte
	switch rec.Op {
	case OpPut:
		newValue = rec.Value
		l.cells[rec.Key] = Cell{Value: append([]byte(nil), rec.Value...), Version: rec.Version}
	case OpDel:
		delete(l.cells, rec.Key)
	}
	l.reindex(rec.Key, oldValue, newValue)
}

// reindex remove os tokens do valor antigo e adiciona os do novo (P7 mantido
// incrementalmente).
func (l *Ledger) reindex(key string, oldValue, newValue []byte) {
	oldTokens := tokensFor(key, oldValue)
	newTokens := tokensFor(key, newValue)
	for t := range oldTokens {
		if _, keep := newTokens[t]; keep {
			continue
		}
		l.removeInv(t, key)
	}
	for t := range newTokens {
		l.addInv(t, key)
	}
}

func (l *Ledger) addInv(tok, key string) {
	m := l.inv[tok]
	if m == nil {
		m = map[string]struct{}{}
		l.inv[tok] = m
	}
	m[key] = struct{}{}
}

func (l *Ledger) removeInv(tok, key string) {
	if m := l.inv[tok]; m != nil {
		delete(m, key)
		if len(m) == 0 {
			delete(l.inv, tok)
		}
	}
}

// compactLocked grava snapshot atômico e trunca o WAL (chamado com lock).
func (l *Ledger) compactLocked() error {
	root := snapshotHash(l.seq, l.lastHash, l.cells)
	snap := Snapshot{
		Seq:      l.seq,
		PrevHash: hex.EncodeToString(l.lastHash[:]),
		RootHash: hex.EncodeToString(root[:]),
		State:    cloneCells(l.cells),
	}

	data, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("ledger: serializar snapshot: %w", err)
	}

	tmp := filepath.Join(l.dir, "snapshot.json.tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("ledger: gravar snapshot: %w", err)
	}
	// fsync do arquivo antes do rename (durabilidade do checkpoint).
	f, err := os.OpenFile(tmp, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("ledger: abrir snapshot para sync: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("ledger: sync snapshot: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("ledger: fechar snapshot: %w", err)
	}
	if err := os.Rename(tmp, filepath.Join(l.dir, "snapshot.json")); err != nil {
		return fmt.Errorf("ledger: renomear snapshot: %w", err)
	}
	// fsync do diretório para tornar o rename durável (POSIX).
	d, err := os.Open(l.dir)
	if err != nil {
		return fmt.Errorf("ledger: abrir dir para sync: %w", err)
	}
	// Directory fsync is a POSIX durability guarantee; on Windows
	// FlushFileBuffers on a directory handle returns ERROR_ACCESS_DENIED, and
	// NTFS rename semantics are already durable — skip it. The Close below
	// still runs on every platform.
	if runtime.GOOS != "windows" {
		if err := d.Sync(); err != nil {
			d.Close()
			return fmt.Errorf("ledger: sync dir: %w", err)
		}
	}
	if err := d.Close(); err != nil {
		return fmt.Errorf("ledger: fechar dir: %w", err)
	}

	// Truncate via os.Truncate (path-based) instead of l.wal.Truncate: on
	// Windows a handle opened with O_APPEND only carries FILE_APPEND_DATA (no
	// GENERIC_WRITE), so SetEndOfFile fails with access denied and every
	// compaction breaks. os.Truncate opens a fresh handle with GENERIC_WRITE;
	// on POSIX both forms are equivalent.
	if err := os.Truncate(l.wal.Name(), 0); err != nil {
		return fmt.Errorf("ledger: truncar WAL: %w", err)
	}
	if _, err := l.wal.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("ledger: seek WAL: %w", err)
	}
	l.walSize = 0
	l.snapSeq = l.seq
	l.snapRoot = root
	l.lastHash = root
	return nil
}

// loadSnapshot carrega o snapshot do disco (se existir) e reconstrói o estado.
func (l *Ledger) loadSnapshot() error {
	path := filepath.Join(l.dir, "snapshot.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("ledger: ler snapshot: %w", err)
	}

	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("ledger: parsear snapshot: %w", err)
	}

	prev, err := hex.DecodeString(s.PrevHash)
	if err != nil || len(prev) != 32 {
		return fmt.Errorf("ledger: snapshot prev inválido")
	}
	var prevHash [32]byte
	copy(prevHash[:], prev)

	root := snapshotHash(s.Seq, prevHash, s.State)
	if hex.EncodeToString(root[:]) != s.RootHash {
		return fmt.Errorf("ledger: snapshot ADULTERADO (raiz não confere)")
	}

	for k, c := range s.State {
		l.cells[k] = c
		l.reindex(k, nil, c.Value)
	}
	l.snapSeq = s.Seq
	l.snapRoot = root
	l.lastHash = root
	l.seq = s.Seq
	return nil
}

// replay relê o WAL do disco, verifica a cadeia de hash e reconstrói o estado
// vivo. running começa na raiz do snapshot; cada registro deve encadear.
func (l *Ledger) replay(br *bufio.Reader) error {
	running := l.snapRoot
	for {
		data, err := readFrame(br)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("ler frame: %w", err)
		}

		var w wireRecord
		if err := json.Unmarshal(data, &w); err != nil {
			return fmt.Errorf("decodificar registro: %w", err)
		}
		rec, err := wireToRecord(w)
		if err != nil {
			return err
		}

		if rec.Seq <= l.seq {
			// Registro já coberto pelo snapshot. Acontece após um crash na
			// janela entre escrever o snapshot e truncar o WAL: o snapshot é
			// mais novo, mas o WAL ainda não foi truncado. Pular em vez de
			// falhar torna o replay idempotente — sem isto o ledger se recusa
			// a abrir ("seq fora de ordem") e exige recuperação manual.
			continue
		}
		if rec.Seq != l.seq+1 {
			return fmt.Errorf("seq fora de ordem (esperava %d, veio %d)", l.seq+1, rec.Seq)
		}
		if rec.PrevHash != running {
			return fmt.Errorf("cadeia quebrada no seq %d", rec.Seq)
		}
		if rec.Hash != rec.computeHash() {
			return fmt.Errorf("hash divergente no seq %d (ADULTERADO)", rec.Seq)
		}

		l.apply(rec)
		l.seq = rec.Seq
		running = rec.Hash
	}
	l.lastHash = running
	return nil
}

// readFrame lê um frame [4-byte len][payload]\n do WAL. Devolve io.EOF apenas
// quando o arquivo terminou exatamente na fronteira de um frame.
func readFrame(br *bufio.Reader) ([]byte, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(br, lenBuf[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(lenBuf[:])
	if n > maxFrameSize {
		return nil, fmt.Errorf("frame grande demais (%d bytes) — WAL corrompido?", n)
	}
	buf := make([]byte, n+1)
	if _, err := io.ReadFull(br, buf); err != nil {
		return nil, err
	}
	if buf[n] != '\n' {
		return nil, fmt.Errorf("frame malformado (falta newline)")
	}
	return buf[:n], nil
}

func cloneCells(in map[string]Cell) map[string]Cell {
	out := make(map[string]Cell, len(in))
	for k, c := range in {
		out[k] = Cell{Value: append([]byte(nil), c.Value...), Version: c.Version}
	}
	return out
}

// tokensFor devolve o conjunto de tokens (chave + valor), minúsculos.
func tokensFor(key string, value []byte) map[string]struct{} {
	set := map[string]struct{}{}
	for _, t := range tokenize(key) {
		set[t] = struct{}{}
	}
	for _, t := range tokenize(string(value)) {
		set[t] = struct{}{}
	}
	return set
}

// tokenize quebra s em tokens alfanuméricos, minúsculos (índice invertido).
func tokenize(s string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.ToLower(string(cur)))
			cur = cur[:0]
		}
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur = append(cur, r)
		} else {
			flush()
		}
	}
	flush()
	return out
}
