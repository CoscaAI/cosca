// Package provenance implementa a Integridade e Provenance do ecossistema
// Creative/Scientific/Media (Fase 1, etapa 1.10).
//
// Cobre §32-§35 do manifesto:
//   - §32 SCIENTIFIC INTEGRITY: nunca inventar resultados. Separar
//     OBSERVED / CALCULATED / SIMULATED / GENERATED / HYPOTHESIS. Toda
//     afirmação científica tem SOURCE/DATA/METHOD/ASSUMPTION/CONFIDENCE.
//   - §33 CREATIVE INTEGRITY: para mídia generativa registrar
//     MODEL/PROMPT/SEED/PARAMETERS/SOURCE ASSETS/PROCESSING/EDIT HISTORY —
//     permitir REPRODUZIR o resultado.
//   - §35 LICENSE/PROVENANCE: nunca incorporar código de terceiros sem
//     verificar licença. Registrar SOURCE/LICENSE/VERSION/MODIFICATIONS/
//     ATTRIBUTION. Separar COSCA CODE / THIRD PARTY / MODEL / USER ASSET.
package provenance

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

// =============================================================================
// §32 — Integridade científica
// =============================================================================

// ClaimKind classifica a natureza de uma afirmação (§32 — nunca inventar).
type ClaimKind string

// Kinds de afirmação científica.
const (
	// Observed — medido/diretamente observado.
	Observed ClaimKind = "observed"
	// Calculated — derivado por cálculo determinístico.
	Calculated ClaimKind = "calculated"
	// Simulated — resultado de simulação.
	Simulated ClaimKind = "simulated"
	// Generated — produzido por IA/modelo generativo.
	Generated ClaimKind = "generated"
	// Hypothesis — hipótese, ainda não verificada.
	Hypothesis ClaimKind = "hypothesis"
)

// Valid reports se o kind é canônico.
func (k ClaimKind) Valid() bool {
	switch k {
	case Observed, Calculated, Simulated, Generated, Hypothesis:
		return true
	}
	return false
}

// Claim é uma afirmação científica com provenance completa (§32).
type Claim struct {
	// ID da afirmação.
	ID string `yaml:"id" json:"id"`
	// Statement da afirmação.
	Statement string `yaml:"statement" json:"statement"`
	// Kind classifica observed/calculated/simulated/generated/hypothesis.
	Kind ClaimKind `yaml:"kind" json:"kind"`
	// Source de onde a afirmação veio.
	Source string `yaml:"source,omitempty" json:"source,omitempty"`
	// Data referencia os dados usados (asset IDs).
	Data []string `yaml:"data,omitempty" json:"data,omitempty"`
	// Method descreve o método usado.
	Method string `yaml:"method,omitempty" json:"method,omitempty"`
	// Assumptions explícitas.
	Assumptions []string `yaml:"assumptions,omitempty" json:"assumptions,omitempty"`
	// Confidence 0.0-1.0.
	Confidence float64 `yaml:"confidence" json:"confidence"`
	// CreatedAt timestamp UTC.
	CreatedAt string `yaml:"created_at" json:"created_at"`
}

// =============================================================================
// §33 — Integridade criativa
// =============================================================================

// Generation registra a provenance de um asset gerado (§33 — reproduzível).
type Generation struct {
	// AssetID do resultado (no Asset Registry).
	AssetID string `yaml:"asset_id" json:"asset_id"`
	// Model usado (ID no Model Registry).
	Model string `yaml:"model" json:"model"`
	// Prompt usado.
	Prompt string `yaml:"prompt" json:"prompt"`
	// Seed de determinismo (reproduzibilidade §22/§33).
	Seed int64 `yaml:"seed" json:"seed"`
	// Parameters completos (sampling, steps, CFG...).
	Parameters map[string]any `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	// SourceAssets — assets de origem usados.
	SourceAssets []string `yaml:"source_assets,omitempty" json:"source_assets,omitempty"`
	// Processing — pipeline/etapas aplicadas (grafo/operações).
	Processing []string `yaml:"processing,omitempty" json:"processing,omitempty"`
	// EditHistory — histórico de edições não-destrutivas (§33).
	EditHistory []EditEntry `yaml:"edit_history,omitempty" json:"edit_history,omitempty"`
	// CreatedAt timestamp UTC.
	CreatedAt string `yaml:"created_at" json:"created_at"`
}

// EditEntry é uma edição registrada (ORIGINAL + OPS).
type EditEntry struct {
	Op   string `yaml:"op" json:"op"`
	When string `yaml:"when" json:"when"`
}

// =============================================================================
// §35 — Licenças e provenance de código
// =============================================================================

// LicenseSource classifica a origem de uma dependência (§35).
type LicenseSource string

// Origem das dependências.
const (
	// SourceCoscaCode — código próprio do Cosca.
	SourceCoscaCode LicenseSource = "cosca-code"
	// SourceThirdParty — biblioteca/código de terceiros.
	SourceThirdParty LicenseSource = "third-party"
	// SourceModel — modelo de IA.
	SourceModel LicenseSource = "model"
	// SourceUserAsset — asset do usuário.
	SourceUserAsset LicenseSource = "user-asset"
)

// Valid reports se a origem é canônica.
func (s LicenseSource) Valid() bool {
	switch s {
	case SourceCoscaCode, SourceThirdParty, SourceModel, SourceUserAsset:
		return true
	}
	return false
}

// LicenseRecord registra a licença de uma dependência (§35).
type LicenseRecord struct {
	// Name da dependência.
	Name string `yaml:"name" json:"name"`
	// Source — cosca-code | third-party | model | user-asset.
	Source LicenseSource `yaml:"source" json:"source"`
	// License — SPDX id ou descrição (MIT, Apache-2.0, GPL-3.0, AGPL...).
	License string `yaml:"license" json:"license"`
	// Version da dependência.
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
	// Modifications feitas ao original.
	Modifications string `yaml:"modifications,omitempty" json:"modifications,omitempty"`
	// Attribution exigida.
	Attribution string `yaml:"attribution,omitempty" json:"attribution,omitempty"`
	// Compatible reports se a licença é compatível com o núcleo proprietário.
	Compatible bool `yaml:"compatible" json:"compatible"`
	// AddedAt timestamp UTC.
	AddedAt string `yaml:"added_at" json:"added_at"`
}

// =============================================================================
// Registry
// =============================================================================

// Registry armazena provenance em <projectRoot>/.cosca/provenance.yaml.
type Registry struct {
	root        string
	path        string
	Claims      []Claim           `yaml:"claims,omitempty" json:"claims,omitempty"`
	Generations []Generation      `yaml:"generations,omitempty" json:"generations,omitempty"`
	Licenses    []LicenseRecord   `yaml:"licenses,omitempty" json:"licenses,omitempty"`
}

// DefaultDir é o diretório do registry dentro de um projeto.
const DefaultDir = ".cosca"

// FileName do registro de provenance.
const FileName = "provenance.yaml"

// Open abre (ou cria) o registry de um projeto.
func Open(projectRoot string) (*Registry, error) {
	path := filepath.Join(projectRoot, DefaultDir, FileName)
	r := &Registry{root: projectRoot, path: path}
	if err := os.MkdirAll(filepath.Join(projectRoot, DefaultDir), 0o755); err != nil {
		return nil, fmt.Errorf("create .cosca: %w", err)
	}
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, r); err != nil {
			return nil, fmt.Errorf("parse provenance: %w", err)
		}
	}
	return r, nil
}

// Path devolve o caminho do arquivo.
func (r *Registry) Path() string { return r.path }

// save persiste o registro.
func (r *Registry) save() error {
	data, err := yaml.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshal provenance: %w", err)
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write provenance tmp: %w", err)
	}
	return os.Rename(tmp, r.path)
}

// AddClaim registra uma afirmação científica (§32) e salva.
func (r *Registry) AddClaim(c Claim) error {
	if c.ID == "" || c.Statement == "" {
		return fmt.Errorf("claim id and statement are required")
	}
	if !c.Kind.Valid() {
		return fmt.Errorf("invalid claim kind %q (valid: observed, calculated, simulated, generated, hypothesis)", c.Kind)
	}
	if c.Confidence < 0 || c.Confidence > 1 {
		return fmt.Errorf("confidence must be 0.0-1.0, got %v", c.Confidence)
	}
	if c.CreatedAt == "" {
		c.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	r.Claims = append(r.Claims, c)
	return r.save()
}

// AddGeneration registra a provenance de uma geração (§33) e salva.
func (r *Registry) AddGeneration(g Generation) error {
	if g.AssetID == "" {
		return fmt.Errorf("generation asset_id is required")
	}
	if g.Model == "" {
		return fmt.Errorf("generation model is required")
	}
	if g.CreatedAt == "" {
		g.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	r.Generations = append(r.Generations, g)
	return r.save()
}

// AddLicense registra a licença de uma dependência (§35) e salva.
func (r *Registry) AddLicense(l LicenseRecord) error {
	if l.Name == "" || l.License == "" {
		return fmt.Errorf("license name and license id are required")
	}
	if !l.Source.Valid() {
		return fmt.Errorf("invalid license source %q (valid: cosca-code, third-party, model, user-asset)", l.Source)
	}
	if l.AddedAt == "" {
		l.AddedAt = time.Now().UTC().Format(time.RFC3339)
	}
	r.Licenses = append(r.Licenses, l)
	return r.save()
}

// ClaimByID devolve uma afirmação por ID.
func (r *Registry) ClaimByID(id string) (Claim, bool) {
	for _, c := range r.Claims {
		if c.ID == id {
			return c, true
		}
	}
	return Claim{}, false
}

// SortedClaims devolve as afirmações ordenadas por timestamp.
func (r *Registry) SortedClaims() []Claim {
	out := append([]Claim(nil), r.Claims...)
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt < out[j].CreatedAt })
	return out
}

// SortedGenerations devolve as gerações ordenadas por timestamp.
func (r *Registry) SortedGenerations() []Generation {
	out := append([]Generation(nil), r.Generations...)
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt < out[j].CreatedAt })
	return out
}

// SortedLicenses devolve as licenças ordenadas por nome.
func (r *Registry) SortedLicenses() []LicenseRecord {
	out := append([]LicenseRecord(nil), r.Licenses...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
