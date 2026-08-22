// Package tdengine implementa a 3D Engine (§13 do manifesto
// Creative/Scientific/Media) — Fase 6, etapa 6.0.
//
// Objetivo: importar, validar e inspecionar modelos 3D (OBJ, glTF) com
// parsers PUROS em Go (sem libs externas — Regra L211: nada de lib sem
// chamar o Don). OBJ é texto simples; glTF é JSON estruturado. A engine
// extrai geometria, materiais e estrutura de cena para o editor e o node
// graph (§13: MODELING → MATERIALS → LIGHTING → CAMERAS → SCENE → RENDER).
package tdengine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Format identifica o formato 3D.
type Format string

// Formatos suportados.
const (
	FormatOBJ  Format = "obj"
	FormatGLTF Format = "gltf"
	FormatGLB  Format = "glb"
	FormatUnknown Format = "unknown"
)

// DetectFormat infere o formato pela extensão (sem abrir).
func DetectFormat(path string) Format {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".obj":
		return FormatOBJ
	case ".gltf":
		return FormatGLTF
	case ".glb":
		return FormatGLB
	default:
		return FormatUnknown
	}
}

// Vertex é um vértice 3D.
type Vertex struct {
	X, Y, Z float64
}

// Face é uma face poligonal (índices 1-based do OBJ, convertidos).
type Face struct {
	// Vertices são os índices (0-based) dos vértices da face.
	Vertices []int
}

// MeshInfo resume um modelo 3D importado.
type MeshInfo struct {
	Path      string   `json:"path"`
	Format    Format   `json:"format"`
	Vertices  int      `json:"vertices"`
	Faces     int      `json:"faces"`
	Normals   int      `json:"normals"`
	Materials []string `json:"materials,omitempty"`
	// VertexCount reporta se o formato é totalmente suportado (glb: parcial).
	Supported bool `json:"supported"`
	// Warnings do parser (ex.: recursos não suportados ignorados).
	Warnings []string `json:"warnings,omitempty"`
}

// =============================================================================
// Parser OBJ
// =============================================================================

// ParseOBJ lê um arquivo .obj e extrai a estrutura básica. Objetos e grupos
// são ignorados na geometria (mas vértices/faces/normais/materiais contam).
func ParseOBJ(path string) (*MeshInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("3d: %w", err)
	}
	info := &MeshInfo{
		Path:      path,
		Format:    FormatOBJ,
		Supported: true,
	}
	lines := strings.Split(string(data), "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		switch fields[0] {
		case "v": // vértice geométrico
			if len(fields) >= 4 {
				info.Vertices++
			}
		case "vn": // normal
			info.Normals++
		case "f": // face
			// Suporta v, v/vt, v/vt/vn, v//vn
			verts := make([]int, 0, len(fields)-1)
			for _, tok := range fields[1:] {
				idxStr := tok
				if i := strings.Index(tok, "/"); i >= 0 {
					idxStr = tok[:i]
				}
				n, err := strconv.Atoi(idxStr)
				if err != nil {
					continue
				}
				// OBJ é 1-based; negativo = relativo ao fim.
				if n > 0 {
					verts = append(verts, n-1)
				} else if n < 0 {
					verts = append(verts, info.Vertices+n)
				}
			}
			if len(verts) >= 3 {
				info.Faces++
			}
		case "usemtl": // material
			if len(fields) >= 2 {
				info.Materials = append(info.Materials, fields[1])
			}
		}
	}
	if info.Vertices == 0 {
		info.Supported = false
		info.Warnings = append(info.Warnings, "arquivo sem vértices 'v' — pode ser vazio ou formato inesperado")
	}
	return info, nil
}

// jsonUnmarshal é um wrapper de json.Unmarshal para o parser glTF.
func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// =============================================================================
// Parser glTF (JSON)
// =============================================================================

// glTFJSON modela a estrutura mínima de um .gltf.
type glTFJSON struct {
	Scenes []struct {
		Name string `json:"name"`
		Nodes []int `json:"nodes"`
	} `json:"scenes"`
	Meshes []struct {
		Name  string `json:"name"`
		Primitives []map[string]any `json:"primitives"`
	} `json:"meshes"`
	Materials []struct {
		Name string `json:"name"`
	} `json:"materials"`
}

// ParseGLTF lê um arquivo .gltf (JSON) e extrai a estrutura de cena.
// GLB (binário) é detectado mas não parseado (suporte parcial).
func ParseGLTF(path string) (*MeshInfo, error) {
	format := DetectFormat(path)
	if format == FormatGLB {
		return &MeshInfo{
			Path: path, Format: FormatGLB, Supported: false,
			Warnings: []string{"GLB binário — estrutura extraída por assinatura, geometria requer parser binário (futuro)"},
		}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("3d: %w", err)
	}
	var g glTFJSON
	if err := jsonUnmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("3d: parse gltf: %w", err)
	}
	info := &MeshInfo{Path: path, Format: FormatGLTF, Supported: true}
	for _, m := range g.Meshes {
		info.Faces += len(m.Primitives)
	}
	for _, mat := range g.Materials {
		if mat.Name != "" {
			info.Materials = append(info.Materials, mat.Name)
		}
	}
	if len(g.Scenes) > 0 && g.Scenes[0].Name != "" {
		// Materializa o nome da cena em Materials? Não — adiciona warning não.
		// Deixamos apenas a contagem; o nome da cena é metadado extra.
	}
	return info, nil
}
