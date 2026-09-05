// Package modelreg implements the Model Registry (§18 do manifesto
// Creative/Scientific/Media) — Fase 1, etapa 1.4.
//
// Diferente do catálogo models.dev (internal/models — context/pricing de
// LLMs), o Model Registry registra MODELOS CONCRETOS usados pelas 18 tarefas
// do AI Task Engine (1.3): formato, quantização, requisito de VRAM,
// capabilities (quais tasks executa) e licença (§35).
//
// Registro por projeto: <projeto>/.cosca/models/index.yaml.
// Um modelo é identificado por (provider, id, version) — chave única.
package modelreg

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/CoscaAI/cosca/internal/aitask"
)

// Format é o formato de serialização do modelo.
type Format string

// Formatos suportados.
const (
	FormatSafetensors Format = "safetensors"
	FormatGGUF        Format = "gguf"
	FormatONNX        Format = "onnx"
	FormatTorch       Format = "torch"
	FormatTensorRT    Format = "tensorrt"
	FormatUnknown     Format = "unknown"
)

// Valid reports se o formato é conhecido.
func (f Format) Valid() bool {
	switch f {
	case FormatSafetensors, FormatGGUF, FormatONNX, FormatTorch, FormatTensorRT, FormatUnknown:
		return true
	}
	return false
}

// Quantization é a precisão de quantização.
type Quantization string

// Quantizações comuns.
const (
	QFP16 Quantization = "fp16"
	QFP32 Quantization = "fp32"
	QInt8 Quantization = "int8"
	QInt4 Quantization = "int4"
	QNone Quantization = "none"
)

// Valid reports se a quantização é conhecida.
func (q Quantization) Valid() bool {
	switch q {
	case QFP16, QFP32, QInt8, QInt4, QNone:
		return true
	}
	return false
}

// Kind classifica local vs remoto.
type Kind string

// Kinds de modelo.
const (
	KindLocal  Kind = "local"
	KindRemote Kind = "remote"
)

// Model é um modelo concreto registrado (§18).
type Model struct {
	// ID canônico, ex: "whisper-large-v3".
	ID string `yaml:"id" json:"id"`
	// Provider, ex: "openai", "local", "ollama".
	Provider string `yaml:"provider" json:"provider"`
	// Version, ex: "large-v3", "6.0", "2.0.78".
	Version string `yaml:"version" json:"version"`
	// Format de serialização.
	Format Format `yaml:"format" json:"format"`
	// Quantization.
	Quantization Quantization `yaml:"quantization" json:"quantization"`
	// VRAM em bytes (0 = desconhecido).
	VRAM int64 `yaml:"vram" json:"vram"`
	// Capabilities: quais tarefas §4 este modelo executa.
	Capabilities []aitask.Type `yaml:"capabilities" json:"capabilities"`
	// Kind: local (roda na máquina) ou remote (API).
	Kind Kind `yaml:"kind" json:"kind"`
	// License (§35) — separada da licença do código.
	License string `yaml:"license" json:"license"`
	// Source: de onde veio (URL, caminho, registry) — provenance §33/§35.
	Source string `yaml:"source,omitempty" json:"source,omitempty"`
	// AddedAt timestamp UTC.
	AddedAt string `yaml:"added_at" json:"added_at"`
}

// Key devolve a chave única (provider/id/version).
func (m *Model) Key() string { return m.Provider + "/" + m.ID + "/" + m.Version }

// Supports reports se o modelo executa a tarefa dada.
func (m *Model) Supports(t aitask.Type) bool {
	for _, c := range m.Capabilities {
		if c == t {
			return true
		}
	}
	return false
}

// New valida e cria um modelo.
func New(id, provider, version string, format Format, quant Quantization, vram int64, kind Kind, license string, caps []aitask.Type) (*Model, error) {
	id = strings.TrimSpace(id)
	provider = strings.TrimSpace(provider)
	version = strings.TrimSpace(version)
	if id == "" || provider == "" || version == "" {
		return nil, fmt.Errorf("model id, provider and version are required")
	}
	if !format.Valid() {
		return nil, fmt.Errorf("invalid format %q", format)
	}
	if !quant.Valid() {
		return nil, fmt.Errorf("invalid quantization %q", quant)
	}
	if kind != KindLocal && kind != KindRemote {
		return nil, fmt.Errorf("invalid kind %q (valid: local, remote)", kind)
	}
	if len(caps) == 0 {
		return nil, fmt.Errorf("model must declare at least one capability (task)")
	}
	for _, c := range caps {
		if !c.Valid() {
			return nil, fmt.Errorf("invalid capability %q", c)
		}
	}
	return &Model{
		ID: id, Provider: provider, Version: version,
		Format: format, Quantization: quant, VRAM: vram,
		Kind: kind, License: license, Capabilities: caps,
		AddedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// =============================================================================
// Registry
// =============================================================================

// Registry armazena modelos em <projectRoot>/.cosca/models/index.yaml.
type Registry struct {
	root string
	dir  string
	byKey map[string]*Model
}

// DefaultDir é o diretório do registry dentro de um projeto.
const DefaultDir = ".cosca/models"

// Open abre (ou cria) o registry de um projeto.
func Open(projectRoot string) (*Registry, error) {
	dir := filepath.Join(projectRoot, DefaultDir)
	r := &Registry{root: projectRoot, dir: dir, byKey: make(map[string]*Model)}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create models dir: %w", err)
	}
	if err := r.load(); err != nil {
		return nil, err
	}
	return r, nil
}

// Dir devolve o caminho do registry.
func (r *Registry) Dir() string { return r.dir }

func (r *Registry) indexPath() string { return filepath.Join(r.dir, "index.yaml") }

func (r *Registry) load() error {
	data, err := os.ReadFile(r.indexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read models index: %w", err)
	}
	var models []*Model
	if err := yaml.Unmarshal(data, &models); err != nil {
		return fmt.Errorf("parse models index: %w", err)
	}
	for _, m := range models {
		if m != nil && m.Key() != "/" {
			r.byKey[m.Key()] = m
		}
	}
	return nil
}

func (r *Registry) save() error {
	models := make([]*Model, 0, len(r.byKey))
	for _, m := range r.byKey {
		models = append(models, m)
	}
	sort.Slice(models, func(i, j int) bool { return models[i].Key() < models[j].Key() })
	data, err := yaml.Marshal(models)
	if err != nil {
		return fmt.Errorf("marshal models index: %w", err)
	}
	tmp := r.indexPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write models index tmp: %w", err)
	}
	return os.Rename(tmp, r.indexPath())
}

// Register insere ou atualiza um modelo (chave provider/id/version).
func (r *Registry) Register(m *Model) error {
	if m == nil {
		return fmt.Errorf("nil model")
	}
	r.byKey[m.Key()] = m
	return r.save()
}

// Get devolve um modelo por chave (provider/id/version).
func (r *Registry) Get(key string) (*Model, bool) {
	m, ok := r.byKey[key]
	return m, ok
}

// GetByID devolve todos os modelos com o ID dado (qualquer provider/versão).
func (r *Registry) GetByID(id string) []*Model {
	var out []*Model
	for _, m := range r.byKey {
		if m.ID == id {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out
}

// FindForTask devolve modelos que executam a tarefa dada (§18 liga ao §4).
func (r *Registry) FindForTask(t aitask.Type) []*Model {
	var out []*Model
	for _, m := range r.byKey {
		if m.Supports(t) {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out
}

// List devolve todos os modelos ordenados por chave.
func (r *Registry) List() []*Model {
	out := make([]*Model, 0, len(r.byKey))
	for _, m := range r.byKey {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out
}

// Count devolve o número de modelos registrados.
func (r *Registry) Count() int { return len(r.byKey) }

// Remove remove um modelo pela chave.
func (r *Registry) Remove(key string) error {
	if _, ok := r.byKey[key]; !ok {
		return fmt.Errorf("model %q not found", key)
	}
	delete(r.byKey, key)
	return r.save()
}
