// Package asset implements the Cosca Asset Registry (§2 do manifesto
// Creative/Scientific/Media) — Fase 1, etapa 1.2.
//
// Princípios (do Blueprint P1 + §2):
//   - Content-addressable storage: o HASH sha256 do conteúdo é a identidade
//     do asset. Assets iguais = mesma identidade = nunca duplicados.
//   - Não-destrutivo: ORIGINAL + OPS. Derivatives (thumbnails, transcodes,
//     resized) derivam do hash e nunca sobrescrevem o original.
//   - Provenance: cada asset registra Source (de onde veio) para §33/§35.
//   - Metadata rica: formato, codec, dimensões, duração, exif...
package asset

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Type é o tipo canônico de asset (§2 do manifesto).
type Type string

// Tipos suportados.
const (
	TypeImage    Type = "image"
	TypeVideo    Type = "video"
	TypeAudio    Type = "audio"
	Type3D       Type = "3d"
	TypeFont     Type = "font"
	TypeText     Type = "text"
	TypeData     Type = "data"
	TypeModel    Type = "model"
	TypeMaterial Type = "material"
	TypeScript   Type = "script"
	TypeDocument Type = "document"
)

// AllTypes lista os tipos válidos.
var AllTypes = []Type{
	TypeImage, TypeVideo, TypeAudio, Type3D, TypeFont, TypeText,
	TypeData, TypeModel, TypeMaterial, TypeScript, TypeDocument,
}

// Valid reports se o tipo é canônico.
func (t Type) Valid() bool {
	for _, a := range AllTypes {
		if a == t {
			return true
		}
	}
	return false
}

// Asset é um asset registrado (immutable por identidade de conteúdo).
type Asset struct {
	// ID é a identidade content-addressable: sha256 hex do conteúdo.
	ID string `yaml:"id" json:"id"`
	// Type é o tipo canônico.
	Type Type `yaml:"type" json:"type"`
	// Hash é o sha256 do conteúdo (igual a ID; mantido para clareza).
	Hash string `yaml:"hash" json:"hash"`
	// Size em bytes.
	Size int64 `yaml:"size" json:"size"`
	// Metadata rica: format, codec, width, height, duration, exif...
	Metadata map[string]any `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	// Dependencies são IDs de outros assets usados por este.
	Dependencies []string `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	// Version do asset (semver ou hash derivado).
	Version string `yaml:"version" json:"version"`
	// Source registra a proveniência (§33/§35): caminho original, URL, prompt...
	Source string `yaml:"source,omitempty" json:"source,omitempty"`
	// Preview é o caminho (relativo ao assets dir) do derivative de preview.
	Preview string `yaml:"preview,omitempty" json:"preview,omitempty"`
	// Derivatives são caminhos de derivados (thumbnails, transcodes...).
	Derivatives []string `yaml:"derivatives,omitempty" json:"derivatives,omitempty"`
	// AddedAt é o timestamp de registro (UTC RFC3339).
	AddedAt string `yaml:"added_at" json:"added_at"`
}

// New cria um Asset a partir do conteúdo já lido. O ID é o sha256.
func New(data []byte, typ Type, source string) (*Asset, error) {
	if !typ.Valid() {
		return nil, fmt.Errorf("invalid asset type %q (valid: %s)", typ, TypesList())
	}
	sum := sha256.Sum256(data)
	id := hex.EncodeToString(sum[:])
	return &Asset{
		ID:      id,
		Type:    typ,
		Hash:    id,
		Size:    int64(len(data)),
		Version: "1",
		Source:  source,
		AddedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// IsType reports se o asset é do tipo dado.
func (a *Asset) IsType(t Type) bool { return a.Type == t }

// TypesList devolve a lista legível de tipos válidos.
func TypesList() string {
	parts := make([]string, len(AllTypes))
	for i, t := range AllTypes {
		parts[i] = string(t)
	}
	return strings.Join(parts, ", ")
}

// =============================================================================
// Registry — armazenamento content-addressable
// =============================================================================

// Registry armazena assets em <projectRoot>/.cosca/assets/:
//
//	.coscra/assets/index.yaml          — índice (lista de assets)
//	.coscra/assets/objects/<id>        — blobs content-addressable
//	.coscra/assets/derivatives/<id>/…  — derivatives (nunca tocam o original)
type Registry struct {
	root string
	dir  string // .cosca/assets
	// objects é o mapa id → asset (carregado do índice).
	objects map[string]*Asset
}

// FileName é o nome do índice de assets.
const FileName = "index.yaml"

// DefaultDir é o caminho do registry dentro de um projeto.
const DefaultDir = ".cosca/assets"

// Open abre (ou cria) o registry de um projeto.
func Open(projectRoot string) (*Registry, error) {
	dir := filepath.Join(projectRoot, DefaultDir)
	r := &Registry{
		root:    projectRoot,
		dir:     dir,
		objects: make(map[string]*Asset),
	}
	if err := os.MkdirAll(filepath.Join(dir, "objects"), 0o755); err != nil {
		return nil, fmt.Errorf("create assets objects dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "derivatives"), 0o755); err != nil {
		return nil, fmt.Errorf("create assets derivatives dir: %w", err)
	}
	if err := r.load(); err != nil {
		return nil, err
	}
	return r, nil
}

// Dir devolve o caminho do registry.
func (r *Registry) Dir() string { return r.dir }

// ObjectsDir devolve o diretório de blobs content-addressable.
func (r *Registry) ObjectsDir() string { return filepath.Join(r.dir, "objects") }

// DerivativesDir devolve o diretório de derivatives.
func (r *Registry) DerivativesDir() string { return filepath.Join(r.dir, "derivatives") }

func (r *Registry) indexPath() string { return filepath.Join(r.dir, FileName) }

func (r *Registry) load() error {
	data, err := os.ReadFile(r.indexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil // registry novo, índice vazio
		}
		return fmt.Errorf("read assets index: %w", err)
	}
	var assets []*Asset
	if err := yaml.Unmarshal(data, &assets); err != nil {
		return fmt.Errorf("parse assets index: %w", err)
	}
	for _, a := range assets {
		if a != nil && a.ID != "" {
			r.objects[a.ID] = a
		}
	}
	return nil
}

func (r *Registry) save() error {
	assets := make([]*Asset, 0, len(r.objects))
	for _, a := range r.objects {
		assets = append(assets, a)
	}
	sort.Slice(assets, func(i, j int) bool { return assets[i].ID < assets[j].ID })
	data, err := yaml.Marshal(assets)
	if err != nil {
		return fmt.Errorf("marshal assets index: %w", err)
	}
	tmp := r.indexPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write assets index tmp: %w", err)
	}
	return os.Rename(tmp, r.indexPath())
}

// objectPath devolve o caminho do blob de um id.
func (r *Registry) objectPath(id string) string {
	return filepath.Join(r.dir, "objects", id)
}

// Add registra um asset a partir de bytes. Content-addressable: se o mesmo
// conteúdo já existe, devolve o asset existente (nunca duplica).
func (r *Registry) Add(data []byte, typ Type, source string) (*Asset, error) {
	a, err := New(data, typ, source)
	if err != nil {
		return nil, err
	}
	// Dedup por identidade de conteúdo.
	if existing, ok := r.objects[a.ID]; ok {
		return existing, nil
	}
	if err := os.WriteFile(r.objectPath(a.ID), data, 0o644); err != nil {
		return nil, fmt.Errorf("write asset object: %w", err)
	}
	r.objects[a.ID] = a
	if err := r.save(); err != nil {
		return nil, err
	}
	return a, nil
}

// AddFile registra um asset a partir de um arquivo.
func (r *Registry) AddFile(path string, typ Type, source string) (*Asset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if source == "" {
		source = path
	}
	return r.Add(data, typ, source)
}

// Get devolve o asset pelo ID (hash).
func (r *Registry) Get(id string) (*Asset, bool) {
	a, ok := r.objects[id]
	return a, ok
}

// ReadObject devolve os bytes do blob de um asset.
func (r *Registry) ReadObject(id string) ([]byte, error) {
	if _, ok := r.objects[id]; !ok {
		return nil, fmt.Errorf("asset %q not found", id)
	}
	return os.ReadFile(r.objectPath(id))
}

// Has reports se o conteúdo (hash) já está registrado.
func (r *Registry) Has(id string) bool { _, ok := r.objects[id]; return ok }

// List devolve todos os assets ordenados por ID.
func (r *Registry) List() []*Asset {
	assets := make([]*Asset, 0, len(r.objects))
	for _, a := range r.objects {
		assets = append(assets, a)
	}
	sort.Slice(assets, func(i, j int) bool { return assets[i].ID < assets[j].ID })
	return assets
}

// Count devolve o número de assets registrados.
func (r *Registry) Count() int { return len(r.objects) }

// Remove remove um asset e seu blob. Derivatives são preservados no diretório
// (edição não-destrutiva) — apenas o vínculo é removido.
func (r *Registry) Remove(id string) error {
	if _, ok := r.objects[id]; !ok {
		return fmt.Errorf("asset %q not found", id)
	}
	delete(r.objects, id)
	if err := os.Remove(r.objectPath(id)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove asset object: %w", err)
	}
	return r.save()
}

// CopyTo copia o blob de um asset para um destino (para pipelines que precisam
// materializar o arquivo). Não toca o original.
func (r *Registry) CopyTo(id, dest string) error {
	src := r.objectPath(id)
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open asset object: %w", err)
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy asset: %w", err)
	}
	return nil
}
