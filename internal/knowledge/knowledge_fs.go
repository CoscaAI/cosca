package knowledge

// KnowledgeFilesystem (Fase C, ADR-030 §3.3) — Snapshot copy-on-write + refs.
//
// O modelo replica o padrão CAS do Asset Registry (internal/asset): cada
// unidade de conhecimento é um OBJETO IMUTÁVEL content-addressable, cuja
// identidade é o sha256 do conteúdo (K:<hash>). O MESMO conteúdo adicionado
// 2× é deduplicado — nunca duplicado.
//
// Um SNAPSHOT copy-on-write apenas REFERENCIA um conjunto de hashes de
// objetos: criar um snapshot NÃO duplica os objetos existentes (os snapshots
// compartilham os mesmos blobs). É o "git-like" do ADR-030:
//
//	objects/               blobs content-addressable (K:<hash>)
//	objects/index.yaml     índice de metadados por hash
//	snapshots/<id>.yaml    snapshot copy-on-write (lista de hashes)
//	refs/HEAD              ref ativo (nome de ref OU snapshot id "detached")
//	refs/<name>            ref nomeado → snapshot id
//
// Estrutura-alvo do ADR-030 §2.0 (objects/ + snapshots/ + refs/) — aditiva ao
// manifest.yaml/lock.yaml que já vivem em .cosca/knowledge/. NADA na raiz.
//
// Os objetos são as fontes declarativas de conhecimento (laws.json,
// hall-of-fame.json, manifest.yaml, lock.yaml, acquired/, packages/) — o "DNA"
// que o ADR-029 tornou versionável. Este pacote dá a eles a genealogia, o
// checkout e o diff cognitivo.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Diretórios relativos a <projectRoot>/.cosca/knowledge.
const (
	knowledgeFSObjects    = "objects"
	knowledgeFSSnapshots  = "snapshots"
	knowledgeFSRefs       = "refs"
	knowledgeFSObjectList = "index.yaml"
	knowledgeFSHead       = "HEAD"
	knowledgeFSMainRef    = "main"

	// snapshotPrefix é o prefixo dos ids de snapshot: ks_<yyyy_mm_dd>_<seq>.
	snapshotPrefix = "ks_"
)

// Object é um objeto de conhecimento imutável content-addressable. A
// identidade (Hash) é o sha256 do conteúdo — K:<hash>. O mesmo conteúdo nunca
// é duplicado; a Classe e o Path são metadados imutáveis por hash.
type Object struct {
	// Hash é o sha256 hex do conteúdo (identidade content-addressable).
	Hash string `yaml:"hash" json:"hash"`
	// Class é a classe epistêmica (FACT/EVIDENCE/INFERENCE/DECISION/...).
	Class KnowledgeEpistemic `yaml:"class" json:"class"`
	// Path é a proveniência relativa ao projeto (ex: knowledge/laws.json).
	Path string `yaml:"path,omitempty" json:"path,omitempty"`
	// Size em bytes do conteúdo.
	Size int64 `yaml:"size" json:"size"`
	// CreatedAt é o timestamp de registro (UTC RFC3339).
	CreatedAt string `yaml:"created_at" json:"created_at"`
}

// ValidClass devolve true quando a classe está no vocabulário epistêmico
// (reusa as 7 classes de epistemic_class.go — não duplica o vocabulário).
func (o *Object) ValidClass() bool { return o.Class.Valid() }

// CoWSnapshot é um snapshot copy-on-write: referencia um conjunto de hashes de
// objetos. Criar um snapshot NÃO duplica os objetos — a criação é apenas a
// escrita de uma lista de hashes (barata).
type CoWSnapshot struct {
	// ID é o identificador: ks_<yyyy_mm_dd>_<seq>.
	ID string `yaml:"id" json:"id"`
	// Name é o ref nomeado que foi avançado para este snapshot (ex: main).
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	// CreatedAt (UTC RFC3339).
	CreatedAt string `yaml:"created_at" json:"created_at"`
	// Parent é o id do snapshot anterior (HEAD antes deste), vazio no primeiro.
	Parent string `yaml:"parent,omitempty" json:"parent,omitempty"`
	// Objects é a lista CRUZADA de hashes content-addressable reachable.
	Objects []string `yaml:"objects" json:"objects"`
}

// ObjectDiffChange é o tipo de mudança no diff cognitivo.
type ObjectDiffChange string

const (
	// DiffAdded — objeto presente no snapshot B mas não no A (+).
	DiffAdded ObjectDiffChange = "added"
	// DiffRemoved — objeto presente no snapshot A mas não no B (-).
	DiffRemoved ObjectDiffChange = "removed"
	// DiffChanged — mesmo path com conteúdo (hash) diferente (~).
	DiffChanged ObjectDiffChange = "changed"
)

// IsValid devolve true para um tipo de mudança canônico.
func (c ObjectDiffChange) IsValid() bool {
	switch c {
	case DiffAdded, DiffRemoved, DiffChanged:
		return true
	default:
		return false
	}
}

// ObjectDiff é uma linha do diff cognitivo entre dois snapshots.
type ObjectDiff struct {
	// Change é o tipo de mudança (added/removed/changed).
	Change ObjectDiffChange `yaml:"change" json:"change"`
	// Class é a classe epistêmica do objeto.
	Class KnowledgeEpistemic `yaml:"class" json:"class"`
	// Path é a proveniência do objeto (posição no diff cognitivo).
	Path string `yaml:"path,omitempty" json:"path,omitempty"`
	// Hash é o sha256 do objeto afetado.
	Hash string `yaml:"hash" json:"hash"`
	// Icon é o símbolo de exibição (+, ~, -).
	Icon string `yaml:"-" json:"-"`
}

// icon devolve o símbolo do diff cognitivo.
func (d ObjectDiff) icon() string {
	switch d.Change {
	case DiffAdded:
		return "+"
	case DiffRemoved:
		return "-"
	case DiffChanged:
		return "~"
	default:
		return "?"
	}
}

// KnowledgeFS é a store content-addressable de conhecimento (Fase C).
type KnowledgeFS struct {
	root string // <projectRoot>/.cosca/knowledge
	// objects é o índice hash → Object (carregado de objects/index.yaml).
	objects map[string]*Object
}

// NewKnowledgeFS abre (ou cria) a store de conhecimento de um projeto.
func NewKnowledgeFS(projectRoot string) (*KnowledgeFS, error) {
	dir := filepath.Join(projectRoot, ".cosca", "knowledge")
	if err := os.MkdirAll(filepath.Join(dir, knowledgeFSObjects), 0o700); err != nil {
		return nil, fmt.Errorf("knowledge fs: criar %s: %w", knowledgeFSObjects, err)
	}
	if err := os.MkdirAll(filepath.Join(dir, knowledgeFSSnapshots), 0o700); err != nil {
		return nil, fmt.Errorf("knowledge fs: criar %s: %w", knowledgeFSSnapshots, err)
	}
	if err := os.MkdirAll(filepath.Join(dir, knowledgeFSRefs), 0o700); err != nil {
		return nil, fmt.Errorf("knowledge fs: criar %s: %w", knowledgeFSRefs, err)
	}
	fs := &KnowledgeFS{root: dir, objects: make(map[string]*Object)}
	if err := fs.loadObjects(); err != nil {
		return nil, err
	}
	return fs, nil
}

// Dir devolve o diretório raiz da store (.cosca/knowledge).
func (fs *KnowledgeFS) Dir() string { return fs.root }

// objectIndexPath devolve o caminho do índice de objetos.
func (fs *KnowledgeFS) objectIndexPath() string {
	return filepath.Join(fs.root, knowledgeFSObjects, knowledgeFSObjectList)
}

// objectBlobPath devolve o caminho do blob de um hash.
func (fs *KnowledgeFS) objectBlobPath(hash string) string {
	return filepath.Join(fs.root, knowledgeFSObjects, hash)
}

// snapshotPath devolve o caminho do arquivo de snapshot.
func (fs *KnowledgeFS) snapshotPath(id string) string {
	return filepath.Join(fs.root, knowledgeFSSnapshots, id+".yaml")
}

// refPath devolve o caminho de um ref (HEAD ou nome).
func (fs *KnowledgeFS) refPath(name string) string {
	return filepath.Join(fs.root, knowledgeFSRefs, name)
}

// isSnapshotID devolve true quando s tem a forma ks_<data>_<seq>.
func isSnapshotID(s string) bool { return strings.HasPrefix(s, snapshotPrefix) }

// loadObjects carrega o índice de objetos (objects/index.yaml). Ausente => vazio.
func (fs *KnowledgeFS) loadObjects() error {
	data, err := os.ReadFile(fs.objectIndexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("knowledge fs: ler índice de objetos: %w", err)
	}
	var objs []*Object
	if err := yaml.Unmarshal(data, &objs); err != nil {
		return fmt.Errorf("knowledge fs: parsear índice de objetos: %w", err)
	}
	for _, o := range objs {
		if o != nil && o.Hash != "" {
			fs.objects[o.Hash] = o
		}
	}
	return nil
}

// saveObjects persiste o índice de objetos (tmp + rename).
func (fs *KnowledgeFS) saveObjects() error {
	objs := make([]*Object, 0, len(fs.objects))
	for _, o := range fs.objects {
		objs = append(objs, o)
	}
	sort.Slice(objs, func(i, j int) bool { return objs[i].Hash < objs[j].Hash })
	data, err := yaml.Marshal(objs)
	if err != nil {
		return fmt.Errorf("knowledge fs: serializar índice de objetos: %w", err)
	}
	target := fs.objectIndexPath()
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("knowledge fs: escrever índice tmp: %w", err)
	}
	return os.Rename(tmp, target)
}

func hashContent(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// Add registra um objeto de conhecimento a partir de bytes. Content-addressable:
// se o mesmo conteúdo já existe, devolve o objeto existente (nunca duplica).
func (fs *KnowledgeFS) Add(content []byte, class KnowledgeEpistemic, path string) (*Object, error) {
	hash := hashContent(content)
	if o, ok := fs.objects[hash]; ok {
		return o, nil
	}
	obj := &Object{
		Hash:      hash,
		Class:     class,
		Path:      path,
		Size:      int64(len(content)),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := os.WriteFile(fs.objectBlobPath(hash), content, 0o600); err != nil {
		return nil, fmt.Errorf("knowledge fs: escrever objeto %s: %w", hash, err)
	}
	fs.objects[hash] = obj
	if err := fs.saveObjects(); err != nil {
		return nil, err
	}
	return obj, nil
}

// AddFile registra um objeto a partir de um arquivo do disco.
func (fs *KnowledgeFS) AddFile(absPath string, class KnowledgeEpistemic, relPath string) (*Object, error) {
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("knowledge fs: ler %s: %w", absPath, err)
	}
	if relPath == "" {
		relPath = absPath
	}
	return fs.Add(content, class, relPath)
}

// Object devolve o metadado de um objeto pelo hash.
func (fs *KnowledgeFS) Object(hash string) (*Object, bool) {
	o, ok := fs.objects[hash]
	return o, ok
}

// Has reports se o conteúdo (hash) já está registrado.
func (fs *KnowledgeFS) Has(hash string) bool { _, ok := fs.objects[hash]; return ok }

// Objects devolve todos os objetos registrados, ordenados por hash.
func (fs *KnowledgeFS) Objects() []*Object {
	objs := make([]*Object, 0, len(fs.objects))
	for _, o := range fs.objects {
		objs = append(objs, o)
	}
	sort.Slice(objs, func(i, j int) bool { return objs[i].Hash < objs[j].Hash })
	return objs
}

// ObjectsCount devolve o número de objetos registrados.
func (fs *KnowledgeFS) ObjectsCount() int { return len(fs.objects) }

// Snapshot copia o estado atual para um snapshot copy-on-write.
//
// `name` é o ref nomeado a avançar para este snapshot (default "main"); se o
// ref não existe, é criado. `hashes` é o conjunto de hashes reachable do
// snapshot (os objetos que representam o estado atual). Se `hashes` for nil,
// snapshota TODOS os objetos registrados no índice.
//
// Como o snapshot apenas referencia hashes, criar um snapshot NUNCA duplica os
// objetos dos snapshots anteriores (CoW).
func (fs *KnowledgeFS) Snapshot(name string, hashes []string) (*CoWSnapshot, error) {
	if name == "" {
		name = knowledgeFSMainRef
	}

	// Resolve o conjunto de hashes (o estado atual).
	set := hashes
	if set == nil {
		set = make([]string, 0, len(fs.objects))
		for h := range fs.objects {
			set = append(set, h)
		}
	}
	set = dedupSorted(set)

	// Parent = snapshot atualmente ativo (se houver).
	parent := ""
	if cur, err := fs.ResolveHEAD(); err == nil {
		parent = cur.ID
	}

	id := fs.nextSnapshotID()

	snap := &CoWSnapshot{
		ID:        id,
		Name:      name,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Parent:    parent,
		Objects:   set,
	}
	if err := fs.writeSnapshot(snap); err != nil {
		return nil, err
	}

	// Avança o ref nomeado e o HEAD para ele.
	if err := fs.SetRef(name, id); err != nil {
		return nil, err
	}
	if err := fs.setHEAD(name); err != nil {
		return nil, err
	}
	return snap, nil
}

// SetRef define/atualiza o ref nomeado `name` → snapshot id.
func (fs *KnowledgeFS) SetRef(name, id string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("knowledge fs: ref vazio")
	}
	if !isSnapshotID(id) {
		return fmt.Errorf("knowledge fs: %q não é um id de snapshot (esperado ks_...)", id)
	}
	// Cria o ref no diretório de refs (0777 na base; o repo não intromete).
	refDir := filepath.Join(fs.root, knowledgeFSRefs)
	if err := os.MkdirAll(refDir, 0o700); err != nil {
		return fmt.Errorf("knowledge fs: criar refs dir: %w", err)
	}
	return atomicWrite(fs.refPath(name), []byte(id), 0o600)
}

// setHEAD aponta o HEAD para `target` (nome de ref ou id de snapshot detached).
func (fs *KnowledgeFS) setHEAD(target string) error {
	return atomicWrite(fs.refPath(knowledgeFSHead), []byte(target), 0o600)
}

// ReadHEAD devolve o valor bruto do HEAD (nome de ref ou id de snapshot).
func (fs *KnowledgeFS) ReadHEAD() (string, error) {
	data, err := os.ReadFile(fs.refPath(knowledgeFSHead))
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("knowledge fs: nenhum HEAD (rode 'cosca knowledge snapshot' primeiro)")
		}
		return "", fmt.Errorf("knowledge fs: ler HEAD: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// RefValue devolve o snapshot id de um ref nomeado.
func (fs *KnowledgeFS) RefValue(name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("knowledge fs: ref vazio")
	}
	if isSnapshotID(name) {
		return name, nil
	}
	data, err := os.ReadFile(fs.refPath(name))
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("knowledge fs: ref %q não encontrado", name)
		}
		return "", fmt.Errorf("knowledge fs: ler ref %q: %w", name, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// Refs devolve os nomes de refs nomeados (excluindo HEAD), ordenados.
func (fs *KnowledgeFS) Refs() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(fs.root, knowledgeFSRefs))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("knowledge fs: ler refs dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || e.Name() == knowledgeFSHead {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}

// GetSnapshot carrega um snapshot pelo id.
func (fs *KnowledgeFS) GetSnapshot(id string) (*CoWSnapshot, error) {
	if !isSnapshotID(id) {
		return nil, fmt.Errorf("knowledge fs: %q não é um id de snapshot (ks_...)", id)
	}
	data, err := os.ReadFile(fs.snapshotPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("knowledge fs: snapshot %q não encontrado", id)
		}
		return nil, fmt.Errorf("knowledge fs: ler snapshot %q: %w", id, err)
	}
	var s CoWSnapshot
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("knowledge fs: parsear snapshot %q: %w", id, err)
	}
	if s.ID == "" {
		s.ID = id
	}
	return &s, nil
}

// ListSnapshots devolve todos os snapshots, ordenados por id (ascendente).
func (fs *KnowledgeFS) ListSnapshots() ([]CoWSnapshot, error) {
	entries, err := os.ReadDir(filepath.Join(fs.root, knowledgeFSSnapshots))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("knowledge fs: ler snapshots dir: %w", err)
	}
	var snaps []CoWSnapshot
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".yaml")
		s, err := fs.GetSnapshot(id)
		if err != nil {
			return nil, err
		}
		snaps = append(snaps, *s)
	}
	sort.Slice(snaps, func(i, j int) bool { return snaps[i].ID < snaps[j].ID })
	return snaps, nil
}

func (fs *KnowledgeFS) writeSnapshot(s *CoWSnapshot) error {
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("knowledge fs: serializar snapshot %s: %w", s.ID, err)
	}
	snapDir := filepath.Join(fs.root, knowledgeFSSnapshots)
	if err := os.MkdirAll(snapDir, 0o700); err != nil {
		return fmt.Errorf("knowledge fs: criar snapshots dir: %w", err)
	}
	target := fs.snapshotPath(s.ID)
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("knowledge fs: escrever snapshot tmp: %w", err)
	}
	return os.Rename(tmp, target)
}

// nextSnapshotID gera ks_<yyyy_mm_dd>_<seq> com seq incremental (nunca colide).
func (fs *KnowledgeFS) nextSnapshotID() string {
	date := time.Now().UTC().Format("20060102")
	prefix := snapshotPrefix + date + "_"
	max := 0
	entries, _ := os.ReadDir(filepath.Join(fs.root, knowledgeFSSnapshots))
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".yaml")
		if strings.HasPrefix(name, prefix) {
			if n := parseSeq(name, prefix); n > max {
				max = n
			}
		}
	}
	return fmt.Sprintf("%s%03d", prefix, max+1)
}

func parseSeq(id, prefix string) int {
	rest := strings.TrimPrefix(id, prefix)
	var seq int
	_, _ = fmt.Sscanf(rest, "%d", &seq)
	return seq
}

// ResolveHEAD resolve o HEAD para o snapshot ativo. Aceita HEAD como nome de
// ref (main/experimental...) ou id de snapshot (detached).
func (fs *KnowledgeFS) ResolveHEAD() (*CoWSnapshot, error) {
	head, err := fs.ReadHEAD()
	if err != nil {
		return nil, err
	}
	id, err := fs.RefValue(head)
	if err != nil {
		return nil, fmt.Errorf("knowledge fs: HEAD -> %q: %w", head, err)
	}
	return fs.GetSnapshot(id)
}

// Checkout troca o HEAD para apontar para um snapshot existente.
//
// `target` pode ser um ref nomeado (main/experimental) ou um id de snapshot
// (ks_...). Um ref nomeado mantém o HEAD simbólico; um id de snapshot deixa o
// HEAD detached (como o git).
func (fs *KnowledgeFS) Checkout(target string) (*CoWSnapshot, error) {
	if strings.TrimSpace(target) == "" {
		return nil, fmt.Errorf("knowledge fs: alvo de checkout vazio")
	}
	if isSnapshotID(target) {
		// detached: valida que o snapshot existe.
		snap, err := fs.GetSnapshot(target)
		if err != nil {
			return nil, err
		}
		if err := fs.setHEAD(target); err != nil {
			return nil, err
		}
		return snap, nil
	}
	id, err := fs.RefValue(target)
	if err != nil {
		return nil, err
	}
	snap, err := fs.GetSnapshot(id)
	if err != nil {
		return nil, err
	}
	if err := fs.setHEAD(target); err != nil {
		return nil, err
	}
	return snap, nil
}

// resolveSnapshot resolve um alvo (ref nomeado ou id) para um snapshot.
// Um id (ks_...) resolve direto; um nome de ref resolve via refs/<name>.
func (fs *KnowledgeFS) resolveSnapshot(target string) (*CoWSnapshot, error) {
	if isSnapshotID(target) {
		return fs.GetSnapshot(target)
	}
	id, err := fs.RefValue(target)
	if err != nil {
		return nil, err
	}
	return fs.GetSnapshot(id)
}

// Diff produz o diff cognitivo entre dois snapshots (A → B).
//
// Cada alvo (refA/refB) pode ser um ref nomeado ou um id de snapshot. O diff
// é por PATHS (a proveniência cognitiva), não por arquivo:
//   - path só em A → removed (-)
//   - path só em B → added (+)
//   - path em ambos mas com hash diferente → changed (~)  [conteúdo mudou]
//   - path em ambos com o mesmo hash → inalterado (não aparece)
// Objetos sem path são comparados por hash (added/removed).
func (fs *KnowledgeFS) Diff(refA, refB string) ([]ObjectDiff, error) {
	a, err := fs.resolveSnapshot(refA)
	if err != nil {
		return nil, fmt.Errorf("knowledge fs: diff A: %w", err)
	}
	b, err := fs.resolveSnapshot(refB)
	if err != nil {
		return nil, fmt.Errorf("knowledge fs: diff B: %w", err)
	}
	return fs.diff(a, b), nil
}

// diffObjects resolve os hashes de um snapshot para os seus metadados.
func (fs *KnowledgeFS) diffObjects(s *CoWSnapshot) (byPath map[string]*Object, byHash map[string]*Object) {
	byPath = make(map[string]*Object, len(s.Objects))
	byHash = make(map[string]*Object, len(s.Objects))
	for _, h := range s.Objects {
		o, ok := fs.objects[h]
		if !ok {
			o = &Object{Hash: h, Class: EpistemicEVIDENCE}
		}
		byHash[h] = o
		if o.Path != "" {
			byPath[o.Path] = o
		}
	}
	return byPath, byHash
}

func (fs *KnowledgeFS) diff(a, b *CoWSnapshot) []ObjectDiff {
	aPath, aHash := fs.diffObjects(a)
	bPath, bHash := fs.diffObjects(b)

	var out []ObjectDiff
	seen := map[string]bool{}

	// Paths em A: removed (só A) ou changed (ambos, hash diff).
	for p, oa := range aPath {
		if ob, ok := bPath[p]; ok {
			if ob.Hash != oa.Hash {
				out = append(out, ObjectDiff{
					Change: DiffChanged, Class: ob.Class, Path: p, Hash: ob.Hash,
				})
			}
		} else {
			out = append(out, ObjectDiff{Change: DiffRemoved, Class: oa.Class, Path: p, Hash: oa.Hash})
		}
		seen[p] = true
	}
	// Paths só em B: added. (Já marcados em seen quando em ambos — pulados.)
	for p, ob := range bPath {
		if seen[p] {
			continue
		}
		if _, ok := aPath[p]; !ok {
			out = append(out, ObjectDiff{Change: DiffAdded, Class: ob.Class, Path: p, Hash: ob.Hash})
			seen[p] = true
		}
	}
	// Objetos SEM path: comparados por hash.
	for h, oa := range aHash {
		if oa.Path != "" {
			continue
		}
		if _, ok := bHash[h]; !ok {
			out = append(out, ObjectDiff{Change: DiffRemoved, Class: oa.Class, Path: oa.Path, Hash: h})
		}
	}
	for h, ob := range bHash {
		if ob.Path != "" {
			continue
		}
		if _, ok := aHash[h]; !ok {
			out = append(out, ObjectDiff{Change: DiffAdded, Class: ob.Class, Path: ob.Path, Hash: h})
		}
	}

	// Ordena: icon (+ ~ -), depois path, depois hash.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Change != out[j].Change {
			return diffRank(out[i].Change) < diffRank(out[j].Change)
		}
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Hash < out[j].Hash
	})
	for i := range out {
		out[i].Icon = out[i].icon()
	}
	return out
}

func diffRank(c ObjectDiffChange) int {
	switch c {
	case DiffAdded:
		return 0
	case DiffChanged:
		return 1
	case DiffRemoved:
		return 2
	default:
		return 3
	}
}

func dedupSorted(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(in))
	for _, s := range in {
		if s != "" {
			set[s] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// atomicWrite escreve `data` em `target` de forma atômica (tmp + rename).
func atomicWrite(target string, data []byte, mode os.FileMode) error {
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return fmt.Errorf("knowledge fs: escrever tmp %q: %w", tmp, err)
	}
	return os.Rename(tmp, target)
}
