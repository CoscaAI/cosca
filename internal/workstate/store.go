package workstate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// =============================================================================
// Tipos de disco — manifest + base (checkpoint) + delta (O(k)).
// =============================================================================

// ManifestFilename é o nome do arquivo de manifest (fonte de verdade do snapshot).
const ManifestFilename = "manifest.json"

// baseFile é o conteúdo de um arquivo de checkpoint (estado COMMITADO completo).
type baseFile struct {
	Generation uint64            `json:"generation"`
	CreatedAt  time.Time         `json:"created_at"`
	Entries    map[string]string `json:"entries"`
}

// deltaFile é o conteúdo de um arquivo de delta (só o que mudou: k entries).
type deltaFile struct {
	Generation uint64            `json:"generation"`
	From       uint64            `json:"from"`
	CreatedAt  time.Time         `json:"created_at"`
	Sets       map[string]string `json:"sets,omitempty"`
	Deletes    []string          `json:"deletes,omitempty"`
}

// EntryMeta descreve uma entrada no manifest: nome, tamanho e onde ela vive.
type EntryMeta struct {
	Name       string `json:"name"`
	Size       int    `json:"size"`
	Generation uint64 `json:"generation"`
	InBase     bool   `json:"in_base,omitempty"`
	InDelta    bool   `json:"in_delta,omitempty"`
	Deleted    bool   `json:"deleted,omitempty"`
}

// Manifest descreve um snapshot persistido (o "o que existe" + onde está).
type Manifest struct {
	ID         string               `json:"id"`
	Generation uint64               `json:"generation"`
	CreatedAt  time.Time            `json:"created_at"`
	BaseFile   string               `json:"base_file,omitempty"`
	DeltaFile  string               `json:"delta_file,omitempty"`
	Entries    map[string]EntryMeta `json:"entries"`
	TotalBytes int                  `json:"total_bytes"`
	DeltaBytes int                  `json:"delta_bytes"`
}

// Snapshot é o result de um save/checkpoint — metadados, não o payload.
type Snapshot struct {
	Name       string    `json:"name"`
	Generation uint64    `json:"generation"`
	CreatedAt  time.Time `json:"created_at"`
	Entries    int       `json:"entries"`
	TotalBytes int       `json:"total_bytes"`
	IsCheckpoint bool    `json:"is_checkpoint"`
}

// OpStats são contadores observáveis de operações por entrada — permitem
// verificar que snapshot/restore do delta é O(k), não O(estado total).
type OpStats struct {
	SetOps          uint64 `json:"set_ops"`
	GetOps          uint64 `json:"get_ops"`
	SaveCount       uint64 `json:"save_count"`
	LoadCount       uint64 `json:"load_count"`
	CheckpointCount uint64 `json:"checkpoint_count"`
	DeltaUploaded   uint64 `json:"delta_uploaded"` // entries gravadas no último save
	BaseRead        uint64 `json:"base_read"`      // entries lidas do base no último load
	DeltaApplied    uint64 `json:"delta_applied"`  // entries do delta aplicadas no último load
}

// =============================================================================
// Store — persistência durável do working-state.
// =============================================================================

// Store persiste o working-state do agente em disco com:
//   - escrita atômica (tmp + os.Rename, ver atomicWrite);
//   - manifest JSON descrevendo o snapshot;
//   - delta incremental O(k): checkpoint (base completo) + saves que gravam só
//     o que mudou (k), nunca o estado inteiro.
//
// Tolerante a detach/reattach: o Load() reconstrói base+delta; as entradas
// em andamento (dirty) sobrevivem porque o delta é persistido a cada Save.
//
// COMPLEXIDADE REAL (honesta):
//   - Save() = O(k) para serializar/gravar o delta + O(n) para materializar o
//     manifest das entries correntes (o manifest lista o estado inteiro; o
//     payload gravado no delta é só O(k)).
//   - Load() = O(n) para ler o base UMA vez + O(k) para reaplear o delta.
//   - Checkpoint() = O(k) para o fold + O(n) para gravar o base completo.
type Store struct {
	mu   sync.Mutex
	dir  string
	caps Caps

	// state é o working-state em memória que o Store possui (owns).
	state *State
	gen   uint64
	stats OpStats

	// identidade amigável do snapshot (ex: "agent-task-42").
	name string

	// hooks de teste (versões injetáveis de operações de arquivo) — permite
	// simular falha de escrita/rename de forma determinística e cross-platform.
	rename   func(old, new string) error // default os.Rename
	newFile  func() (*os.File, error)    // default os.CreateTemp no dir
}

// NewStore abre/cria um Store de working-state num diretório. Não lê nada
// do disco — use Load() para restaurar um snapshot existente.
func NewStore(dir string, caps Caps) (*Store, error) {
	if dir == "" {
		return nil, errors.New("workstate: store dir is empty")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("workstate: mkdir %s: %w", dir, err)
	}
	return &Store{
		dir:     dir,
		caps:    caps,
		state:   NewState(caps),
		gen:     0,
		rename:  os.Rename,
		name:    filepath.Base(dir),
		newFile: func() (*os.File, error) { return os.CreateTemp(dir, ".ws-tmp-*") },
	}, nil
}

// SetName define um nome amigável para o snapshot (também aparece no manifest ID).
func (s *Store) SetName(name string) { s.name = name }

// Name devolve o nome amigável atual.
func (s *Store) Name() string { return s.name }

// Set grava/atualiza uma entrada no working-state em memória.
func (s *Store) Set(name, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Set(name, value)
	s.stats.SetOps++
}

// Get lê uma entrada do working-state em memória.
func (s *Store) Get(name string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats.GetOps++
	return s.state.Get(name)
}

// Has informa se a entrada existe.
func (s *Store) Has(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.Has(name)
}

// Delete remove uma entrada (tombstone no delta).
func (s *Store) Delete(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Delete(name)
}

// State devolve o working-state em memória (para acesso avançado/puro).
func (s *Store) State() *State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// Len devolve o número de entradas correntes no working-state em memória.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.Len()
}

// Stats devolve uma cópia dos contadores de operações.
func (s *Store) Stats() OpStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stats
}

// Generation devolve a geração corrente do snapshot persistido.
func (s *Store) Generation() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gen
}

// =============================================================================
// Escrita atômica — tmp + os.Rename (nunca escrita parcial).
// =============================================================================

// atomicWrite grava `data` no destino de forma atômica: escreve num arquivo
// temporário no MESMO diretório e faz rename por cima do destino. Se qualquer
// passo falhar, o destino permanece INTACTO.
func (s *Store) atomicWrite(dest string, data []byte) error {
	dir := filepath.Dir(dest)
	f, err := s.newFile()
	if err != nil {
		if rerr := os.MkdirAll(dir, 0o755); rerr != nil {
			return fmt.Errorf("workstate: mkdir for atomic write: %w", rerr)
		}
		var merr error
		f, merr = os.CreateTemp(dir, ".ws-tmp-*")
		if merr != nil {
			return fmt.Errorf("workstate: create tmp: %w", merr)
		}
	}
	tmpName := f.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		cleanup()
		return fmt.Errorf("workstate: write tmp: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		cleanup()
		return fmt.Errorf("workstate: sync tmp: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return fmt.Errorf("workstate: close tmp: %w", err)
	}

	rename := s.rename
	if rename == nil {
		rename = os.Rename
	}
	if err := rename(tmpName, dest); err != nil {
		cleanup()
		return fmt.Errorf("workstate: replace %s: %w", dest, err)
	}
	return nil
}

// =============================================================================
// Save / Checkpoint / Load — o ciclo do snapshot.
// =============================================================================

// Save persiste o working-state como um DELTA incremental O(k): grava no disco
// SÓ as entradas que mudaram desde o último checkpoint (o delta em memória), mais
// um manifest JSON que lista o estado corrente. Não mexe no base/checkpoint —
// detach/reattach preserva o trabalho em andamento porque o delta é persistido.
// Complexidade: O(k) para o payload do delta + O(n) para materializar o manifest.
func (s *Store) Save() (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	gen := s.gen + 1
	s.gen = gen

	// Monta o delta (somente o que mudou — O(k)).
	sets, deletes := s.state.DeltaSnapshot()
	delta := deltaFile{
		Generation: gen,
		From:       gen - 1,
		CreatedAt:  time.Now().UTC(),
		Sets:       sets,
		Deletes:    deletes,
	}
	deltaBytes := 0
	for k, v := range sets {
		deltaBytes += len(k) + len(v)
	}
	data, err := json.Marshal(delta)
	if err != nil {
		return Snapshot{}, fmt.Errorf("workstate: marshal delta: %w", err)
	}
	deltaName := fmt.Sprintf("delta-%d.json", gen)
	if err := s.atomicWrite(filepath.Join(s.dir, deltaName), data); err != nil {
		s.gen-- // falhou: geração não avançou
		return Snapshot{}, err
	}

	// Monta o manifest descrevendo o snapshot.
	entries := s.state.Entries()
	totalBytes := 0
	meta := make(map[string]EntryMeta, len(entries))
	for name, val := range entries {
		meta[name] = EntryMeta{
			Name:       name,
			Size:       len(val),
			Generation: gen,
			InBase:     s.state.touched[name] == false,
			InDelta:    s.state.touched[name],
		}
		totalBytes += len(val)
	}
	// reflexo dos deletes no manifest (entrada sumiu de vista).
	for _, name := range deletes {
		if _, ok := meta[name]; !ok {
			meta[name] = EntryMeta{Name: name, Generation: gen, Deleted: true}
		}
	}

	// Preserva o BaseFile do manifest anterior (o checkpoint vigente). Sem isto,
	// um cold-Load() veria base vazia e perderia as entradas commitadas na base.
	prevMan, _ := s.readManifest()
	baseFile := ""
	if prevMan != nil {
		baseFile = prevMan.BaseFile
	}

	manifest := Manifest{
		ID:         snapshotID(s.name, gen),
		Generation: gen,
		CreatedAt:  time.Now().UTC(),
		BaseFile:   baseFile,
		DeltaFile:  deltaName,
		Entries:    meta,
		TotalBytes: totalBytes,
		DeltaBytes: deltaBytes,
	}
	mdata, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Snapshot{}, fmt.Errorf("workstate: marshal manifest: %w", err)
	}
	if err := s.atomicWrite(filepath.Join(s.dir, ManifestFilename), mdata); err != nil {
		// manifest não avançou: o snapshot "anterior" continua íntegro (o
		// delta-<gen>.json órfão é inofensivo). Geração não avança.
		s.gen--
		return Snapshot{}, err
	}

	// Deltas anteriores (mesma base) foram superseded pelo delta corrente.
	s.pruneStaleDeltas(gen)

	s.stats.SaveCount++
	s.stats.DeltaUploaded = uint64(len(sets) + len(deletes))
	return Snapshot{
		Name:         s.name,
		Generation:   gen,
		CreatedAt:    manifest.CreatedAt,
		Entries:      len(entries),
		TotalBytes:   totalBytes,
		IsCheckpoint: false,
	}, nil
}

// Checkpoint sela o delta no base e persiste o ESTADO COMPLETO como um
// checkpoint (base-<gen>.json). A partir daqui o próximo Save volta a ser só
// delta O(k). Complexidade: O(k) para o fold + O(n) para gravar o base completo.
//
// Consistência (fail-safe I2): o fold do delta na memória só acontece DEPOIS
// de o arquivo base e o manifest terem sido gravados com sucesso. Se qualquer
// um falhar, o working-state em memória permanece com o delta intacto — nada se
// perde, e o disco continua apontando para o snapshot anterior.
func (s *Store) Checkpoint() (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	gen := s.gen + 1

	// Estado COMMITADO candidato = visão atual (base + delta − deletes).
	// Não muta o state: usamos a visão fusionada para calcular o arquivo base.
	folded := s.state.Entries()

	bfile := baseFile{Generation: gen, CreatedAt: time.Now().UTC(), Entries: folded}
	data, err := json.Marshal(bfile)
	if err != nil {
		return Snapshot{}, fmt.Errorf("workstate: marshal base: %w", err)
	}
	baseName := fmt.Sprintf("base-%d.json", gen)
	if err := s.atomicWrite(filepath.Join(s.dir, baseName), data); err != nil {
		return Snapshot{}, err
	}

	// Manifest aponta para o base (e limpa delta).
	manifest := Manifest{
		ID:         snapshotID(s.name, gen),
		Generation: gen,
		CreatedAt:  time.Now().UTC(),
		BaseFile:   baseName,
		Entries:    make(map[string]EntryMeta, len(folded)),
		TotalBytes: 0,
	}
	total := 0
	for name, val := range folded {
		manifest.Entries[name] = EntryMeta{Name: name, Size: len(val), Generation: gen, InBase: true}
		total += len(val)
	}
	manifest.TotalBytes = total
	mdata, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Snapshot{}, fmt.Errorf("workstate: marshal manifest: %w", err)
	}
	if err := s.atomicWrite(filepath.Join(s.dir, ManifestFilename), mdata); err != nil {
		return Snapshot{}, err
	}

	// Commit do disco OK → agora sela o fold na memória (O(edits)).
	s.state.Checkpoint()
	s.gen = gen

	// Deltas anteriores ficam órfãos (o base agora é a verdade) — remove.
	s.pruneStaleDeltas(gen)

	s.stats.CheckpointCount++
	s.stats.DeltaUploaded = 0
	return Snapshot{
		Name:         s.name,
		Generation:   gen,
		CreatedAt:    manifest.CreatedAt,
		Entries:      len(folded),
		TotalBytes:   total,
		IsCheckpoint: true,
	}, nil
}

// LocalSnapshot devolve a versão em memória (não persistida) do snapshot.
func (s *Store) LocalSnapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Snapshot{
		Name:       s.name,
		Generation: s.gen,
		CreatedAt:  time.Now().UTC(),
		Entries:    s.state.Len(),
		TotalBytes: s.state.TotalBytes(),
	}
}

// Load restaura o working-state do disco (base + delta). Cold-restore:
// lê o base uma vez O(n) (n = estado commitado) e aplica o delta O(k). As
// entradas em andamento (dirty) são preservadas porque o delta está no disco.
// Não é O(1): o O(n) é o custo mínimo de materializar o estado commitado; o
// valor incremental deste pacote é que a parte do DELTA é O(k), não O(n).
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	man, err := s.readManifest()
	if err != nil {
		return err
	}
	if man == nil {
		// Nenhum snapshot ainda: estado vazio.
		s.state = NewState(s.caps)
		s.gen = 0
		s.stats.LoadCount++
		return nil
	}

	// 1) base (estado commitado) — O(total).
	base := map[string]string{}
	s.gen = man.Generation
	if man.BaseFile != "" {
		bfile, err := s.readBase(man.BaseFile)
		if err != nil {
			return fmt.Errorf("workstate: load base: %w", err)
		}
		base = bfile.Entries
		s.stats.BaseRead = uint64(len(base))
	}

	// 2) delta (só o que mudou) — O(k).
	sets, deletes := map[string]string{}, []string{}
	if man.DeltaFile != "" {
		dfile, err := s.readDelta(man.DeltaFile)
		if err != nil {
			return fmt.Errorf("workstate: load delta: %w", err)
		}
		sets = dfile.Sets
		deletes = dfile.Deletes
		s.stats.DeltaApplied = uint64(len(sets) + len(deletes))
	}

	// Reconstrói o State: base carregado + delta reaplicado (só k entradas).
	st := NewStateWithBase(base, s.caps)
	for k, v := range sets {
		st.Set(k, v)
	}
	for _, k := range deletes {
		st.Delete(k)
	}
	s.state = st
	s.stats.LoadCount++
	return nil
}

// Reset limpa o working-state em memória (para um novo ciclo), sem tocar disco.
func (s *Store) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = NewState(s.caps)
	s.gen = 0
}

// LastManifest devolve o manifest do snapshot mais recente (nil se nenhum).
func (s *Store) LastManifest() (*Manifest, error) {
	return s.readManifest()
}

// =============================================================================
// Leitura / helpers.
// =============================================================================

// readManifest lê o manifest.json (nil,nil se não existir).
func (s *Store) readManifest() (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, ManifestFilename))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("workstate: read manifest: %w", err)
	}
	var man Manifest
	if err := json.Unmarshal(data, &man); err != nil {
		return nil, fmt.Errorf("workstate: parse manifest: %w", err)
	}
	return &man, nil
}

func (s *Store) readBase(name string) (*baseFile, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, name))
	if err != nil {
		return nil, err
	}
	var b baseFile
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("workstate: parse base %s: %w", name, err)
	}
	return &b, nil
}

func (s *Store) readDelta(name string) (*deltaFile, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, name))
	if err != nil {
		return nil, err
	}
	var d deltaFile
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("workstate: parse delta %s: %w", name, err)
	}
	return &d, nil
}

// pruneStaleDeltas remove arquivos de delta com geração < gen (lixo órfão).
func (s *Store) pruneStaleDeltas(curGen uint64) {
	matches, _ := filepath.Glob(filepath.Join(s.dir, "delta-*.json"))
	for _, m := range matches {
		name := filepath.Base(m)
		var g uint64
		if _, err := fmt.Sscanf(name, "delta-%d.json", &g); err == nil && g < curGen {
			_ = os.Remove(m)
		}
	}
}

// snapshotID gera um identificador estável por nome+geração (determinístico).
func snapshotID(name string, gen uint64) string {
	return fmt.Sprintf("%s@%d", name, gen)
}
