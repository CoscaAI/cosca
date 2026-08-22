// Package project implements the Cosca Project Manifest (§34 do manifesto
// Creative/Scientific/Media) and the Project Engine (Fase 1, etapa 1.1).
//
// O manifesto de PROJETO (project.yaml) é distinto do manifest.yaml da
// identidade do framework (que alimenta o license gate F2). project.yaml
// descreve o PRODUTO: nome, versão, tipo, modelos, assets, workflows,
// dependências, render settings, plugins e licenças — tudo que um projeto do
// ecossistema criativo precisa declarar para ser reproduzível (§33/§35).
package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ProjectType é o tipo canônico de produto (§34/§42 do manifesto).
type ProjectType string

// Tipos de produto suportados pelo ecossistema.
const (
	TypeEditor     ProjectType = "editor"
	TypeImage      ProjectType = "image"
	TypeCinema     ProjectType = "cinema"
	TypeMusic      ProjectType = "music"
	TypeGame       ProjectType = "game"
	TypeScientific ProjectType = "scientific"
	Type3D         ProjectType = "3d"
	TypeAnimation  ProjectType = "animation"
	TypeDocument   ProjectType = "document"
	TypeLab        ProjectType = "lab"
)

// AllProjectTypes lista todos os tipos válidos, para validação e ajuda.
var AllProjectTypes = []ProjectType{
	TypeEditor, TypeImage, TypeCinema, TypeMusic, TypeGame,
	TypeScientific, Type3D, TypeAnimation, TypeDocument, TypeLab,
}

// String implementa Stringer.
func (t ProjectType) String() string { return string(t) }

// Valid reports se o tipo é um dos canônicos.
func (t ProjectType) Valid() bool {
	for _, a := range AllProjectTypes {
		if a == t {
			return true
		}
	}
	return false
}

// ModelRef referencia um modelo no Model Registry (§18/§3).
type ModelRef struct {
	ID       string `yaml:"id" json:"id"`
	Provider string `yaml:"provider,omitempty" json:"provider,omitempty"`
	Version  string `yaml:"version,omitempty" json:"version,omitempty"`
}

// DependencyRef referencia uma dependência de terceiros (§35).
type DependencyRef struct {
	ID      string `yaml:"id" json:"id"`
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
	License string `yaml:"license,omitempty" json:"license,omitempty"`
}

// Manifest é o Project Manifest completo (§34).
//
// É serializável em YAML (arquivo) e JSON (API/CLI --json).
type Manifest struct {
	// Identity — campos estáveis da identidade do projeto.
	Name    string `yaml:"name" json:"name"`
	Version string `yaml:"version" json:"version"`
	Type    string `yaml:"type" json:"type"`

	// EngineVersion registra qual versão do Cosca gerou o manifesto.
	EngineVersion string `yaml:"engine_version" json:"engine_version"`

	// Modelos usados pelo projeto (§18).
	Models []ModelRef `yaml:"models,omitempty" json:"models,omitempty"`

	// Assets referenciados (§2) — IDs content-addressable quando aplicável.
	Assets []string `yaml:"assets,omitempty" json:"assets,omitempty"`

	// Workflows do projeto (§5) — IDs do workflow engine.
	Workflows []string `yaml:"workflows,omitempty" json:"workflows,omitempty"`

	// Dependências de terceiros com licença (§35).
	Dependencies []DependencyRef `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`

	// RenderSettings guarda parâmetros do Render Engine (§22): preview/draft/final.
	RenderSettings map[string]any `yaml:"render_settings,omitempty" json:"render_settings,omitempty"`

	// Plugins ativos (§20).
	Plugins []string `yaml:"plugins,omitempty" json:"plugins,omitempty"`

	// Licenses registra provenance (§35): source/license/version/modifications.
	Licenses []DependencyRef `yaml:"licenses,omitempty" json:"licenses,omitempty"`

	// Timestamp de criação/atualização.
	Timestamp string `yaml:"timestamp" json:"timestamp"`
}

// FileName é o nome canônico do manifesto de projeto.
const FileName = "project.yaml"

// SchemaVersion é a versão do schema do manifesto de projeto.
const SchemaVersion = "1.0.0"

// DefaultEngineVersion reporta a versão do engine embutida no manifesto.
// Substituída em build time; fallback para "dev".
var DefaultEngineVersion = "dev"

// New cria um manifesto com valores default e timestamp atual.
func New(name string, typ ProjectType) (*Manifest, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("project name is required")
	}
	if !typ.Valid() {
		return nil, fmt.Errorf("invalid project type %q (valid: %s)", typ, typesList())
	}
	return &Manifest{
		Name:          name,
		Version:       "0.1.0",
		Type:          typ.String(),
		EngineVersion: DefaultEngineVersion,
		RenderSettings: map[string]any{
			"quality": "draft", // preview < draft < final (§22)
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// Path devolve o caminho do manifesto dentro de um projeto (.cosca/project.yaml).
func (m *Manifest) Path(projectRoot string) string {
	return filepath.Join(projectRoot, ".cosca", FileName)
}

// Write persiste o manifesto em <projectRoot>/.cosca/project.yaml.
func (m *Manifest) Write(projectRoot string) error {
	if err := os.MkdirAll(filepath.Join(projectRoot, ".cosca"), 0o755); err != nil {
		return fmt.Errorf("create .cosca: %w", err)
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal project manifest: %w", err)
	}
	path := m.Path(projectRoot)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// Read carrega e valida o manifesto de <projectRoot>. Retorna os.ErrNotExist
// se o projeto ainda não tem manifest.
func Read(projectRoot string) (*Manifest, error) {
	path := filepath.Join(projectRoot, ".cosca", FileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("validate %s: %w", path, err)
	}
	return &m, nil
}

// Validate verifica os invariantes do manifesto.
func (m *Manifest) Validate() error {
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("manifest name is required")
	}
	if m.Type == "" {
		return fmt.Errorf("manifest type is required")
	}
	if !ProjectType(m.Type).Valid() {
		return fmt.Errorf("invalid project type %q (valid: %s)", m.Type, typesList())
	}
	return nil
}

// IsType reports se o manifesto é do tipo dado.
func (m *Manifest) IsType(t ProjectType) bool { return ProjectType(m.Type) == t }

func typesList() string {
	parts := make([]string, len(AllProjectTypes))
	for i, t := range AllProjectTypes {
		parts[i] = string(t)
	}
	return strings.Join(parts, ", ")
}

// TypesList devolve a lista legível de tipos válidos (para ajuda/erros).
func TypesList() string { return typesList() }
