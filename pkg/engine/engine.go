// Package engine é o bridge PÚBLICO das engines do Cosca Engine (Fase 1).
//
// As engines vivem em internal/ (inacessível a outros módulos por regra do
// Go). Este package re-exporta as funções/registries essenciais para que
// consumidores externos (ex.: cosca-code, o Cosca Editor) possam integrar
// as engines in-process sem violar a barreira de internal.
package engine

import (
	"context"

	"github.com/CoscaAI/cosca/internal/asset"
	"github.com/CoscaAI/cosca/internal/gameengine"
	"github.com/CoscaAI/cosca/internal/media"
	"github.com/CoscaAI/cosca/internal/nodegraph"
	"github.com/CoscaAI/cosca/internal/project"
	"github.com/CoscaAI/cosca/internal/provenance"
	"github.com/CoscaAI/cosca/internal/sciengine"
	"github.com/CoscaAI/cosca/internal/tdengine"
)

// Tipos re-exportados (aliases) para consumidores externos.

// Asset é um asset registrado (§2).
type Asset = asset.Asset

// AssetType é o tipo canônico de asset.
type AssetType = asset.Type

// AssetRegistry é o registry content-addressable (§2).
type AssetRegistry = asset.Registry

// ProjectManifest é o Project Manifest (§34).
type ProjectManifest = project.Manifest

// ProjectType é o tipo de projeto/modo do editor (§6/§34).
type ProjectType = project.ProjectType

// Node é um nó do node graph (§21).
type Node = nodegraph.Node

// Graph é um node graph serializável (§21).
type Graph = nodegraph.Graph

// NodeType identifica o tipo de operação de um nó.
type NodeType = nodegraph.NodeType

// Claim é uma afirmação científica (§32).
type Claim = provenance.Claim

// Generation é provenance criativa (§33).
type Generation = provenance.Generation

// LicenseRecord é o registro de licença (§35).
type LicenseRecord = provenance.LicenseRecord

// ProvenanceRegistry é o registry de provenance (§32-35).
type ProvenanceRegistry = provenance.Registry

// Constantes de tipo de asset (§2).
const (
	TypeImage    = asset.TypeImage
	TypeVideo    = asset.TypeVideo
	TypeAudio    = asset.TypeAudio
	Type3D       = asset.Type3D
	TypeFont     = asset.TypeFont
	TypeText     = asset.TypeText
	TypeData     = asset.TypeData
	TypeModel    = asset.TypeModel
	TypeMaterial = asset.TypeMaterial
	TypeScript   = asset.TypeScript
	TypeDocument = asset.TypeDocument
)

// Tipos de projeto (§6/§34).
const (
	TypeEditor       = project.TypeEditor
	TypeImageProj    = project.TypeImage
	TypeCinema       = project.TypeCinema
	TypeMusic        = project.TypeMusic
	TypeGame         = project.TypeGame
	TypeScientific   = project.TypeScientific
	Type3DProj       = project.Type3D
	TypeAnimation    = project.TypeAnimation
	TypeDocumentProj = project.TypeDocument
	TypeLab          = project.TypeLab
)

// OpenAssetRegistry abre (ou cria) o registry de assets de um projeto (§2).
func OpenAssetRegistry(projectRoot string) (*AssetRegistry, error) {
	return asset.Open(projectRoot)
}

// OpenProvenance abre (ou cria) o registry de provenance (§32-35).
func OpenProvenance(projectRoot string) (*ProvenanceRegistry, error) {
	return provenance.Open(projectRoot)
}

// ReadProjectManifest lê o Project Manifest de um projeto (§34).
func ReadProjectManifest(projectRoot string) (*ProjectManifest, error) {
	return project.Read(projectRoot)
}

// NewProjectManifest cria um manifest novo (§34).
func NewProjectManifest(name string, typ ProjectType) (*ProjectManifest, error) {
	return project.New(name, typ)
}

// UnmarshalGraph desserializa JSON em um node graph (§21).
func UnmarshalGraph(data []byte) (*Graph, error) {
	return nodegraph.Unmarshal(data)
}

// BuildGraph monta um grafo a partir de nós e valida (§21).
func BuildGraph(name string, nodes []*Node) (*Graph, error) {
	return nodegraph.Build(name, nodes)
}

// GraphSignature calcula a assinatura recursiva de um nó (cache key §23).
func GraphSignature(g *Graph, nodeID string) (string, error) {
	return g.Signature(nodeID)
}

// GraphTopoOrder devolve os nós em ordem topológica (§21).
func GraphTopoOrder(g *Graph) ([]*Node, error) {
	return g.TopoOrder()
}

// Executor executa um nó do grafo (ponte para a Fase 3 — executores reais
// como o mediaexec do editor implementam este contrato).
type Executor interface {
	Run(ctx context.Context, node *Node, input map[string]any) (any, error)
}

// ExecutorFunc adapta uma função a Executor.
type ExecutorFunc func(ctx context.Context, node *Node, input map[string]any) (any, error)

// Run implementa Executor.
func (f ExecutorFunc) Run(ctx context.Context, node *Node, input map[string]any) (any, error) {
	return f(ctx, node, input)
}

// Cache armazena resultados por assinatura de nó (§23).
type Cache = nodegraph.Cache

// NewCache cria um cache de assinaturas (§23).
func NewCache() *Cache { return nodegraph.NewCache() }

// RunStats descreve o resultado de uma execução de grafo (§22/§23).
type RunStats = nodegraph.Stats

// RunOptions configura a execução de um grafo.
type RunOptions = nodegraph.RunOptions

// GraphRun executa o grafo em ordem topológica com cache por assinatura.
func GraphRun(ctx context.Context, g *Graph, exec Executor, opts *RunOptions) (*RunStats, error) {
	return g.Run(ctx, exec, opts)
}

// =============================================================================
// Media Engine (§16) — vídeo/áudio via ffmpeg
// =============================================================================

// MediaInfo é o resultado do probe de um arquivo de mídia (§16).
type MediaInfo = media.Info

// MediaStream descreve um stream de mídia.
type MediaStream = media.Stream

// MediaFormat descreve o container.
type MediaFormat = media.Format

// MediaPipeOptions configura um pipeline ffmpeg.
type MediaPipeOptions = media.PipeOptions

// ProbeMedia analisa um arquivo de mídia via ffprobe (§16).
func ProbeMedia(ctx context.Context, path string) (*MediaInfo, error) {
	return media.Probe(ctx, path)
}

// ValidateMedia verifica integridade de um arquivo de mídia (§16).
func ValidateMedia(ctx context.Context, path string) (*MediaInfo, error) {
	return media.Validate(ctx, path)
}

// MediaExtractAudio extrai a trilha de áudio (wav/mp3/aac).
func MediaExtractAudio(ctx context.Context, in, out string, o MediaPipeOptions) error {
	return media.ExtractAudio(ctx, in, out, o)
}

// MediaTranscode re-encoda um arquivo (crf por qualidade §22).
func MediaTranscode(ctx context.Context, in, out string, o MediaPipeOptions) error {
	return media.Transcode(ctx, in, out, o)
}

// MediaExtractFrame extrai um frame em um instante t.
func MediaExtractFrame(ctx context.Context, in, out, t string, o MediaPipeOptions) error {
	return media.ExtractFrame(ctx, in, out, t, o)
}

// AudioPipeOptions configura um pipeline de áudio (§9).
type AudioPipeOptions = media.AudioPipeOptions

// MediaConvertAudio converte áudio (formato/codec + volume/sample-rate/canais).
func MediaConvertAudio(ctx context.Context, in, out string, o AudioPipeOptions) error {
	return media.ConvertAudio(ctx, in, out, o)
}

// =============================================================================
// 3D Engine (§13) — import/inspeção de modelos (OBJ/glTF) em Go puro
// =============================================================================

// MeshInfo resume um modelo 3D importado (§13).
type MeshInfo = tdengine.MeshInfo

// TDFormat identifica o formato 3D.
type TDFormat = tdengine.Format

// Formatos 3D suportados.
const (
	TDObject   = tdengine.FormatOBJ
	TDGLTF     = tdengine.FormatGLTF
	TDGLB      = tdengine.FormatGLB
	TDUnknown  = tdengine.FormatUnknown
)

// ParseOBJ importa um modelo .obj (vértices/faces/normais/materiais).
func ParseOBJ(path string) (*MeshInfo, error) {
	return tdengine.ParseOBJ(path)
}

// ParseGLTF importa um modelo .gltf (JSON) — GLB detectado com suporte parcial.
func ParseGLTF(path string) (*MeshInfo, error) {
	return tdengine.ParseGLTF(path)
}

// DetectTDFormat infere o formato 3D pela extensão.
func DetectTDFormat(path string) TDFormat {
	return tdengine.DetectFormat(path)
}

// =============================================================================
// Game Engine (§10) — ECS (scene/entity/component/system) em Go puro
// =============================================================================

// GameComponentType identifica um componente (§10).
type GameComponentType = gameengine.ComponentType

// GameComponent é um componente declarativo.
type GameComponent = gameengine.Component

// GameEntity é uma entidade da cena.
type GameEntity = gameengine.Entity

// GameScene é uma cena do jogo (nível/mapa).
type GameScene = gameengine.Scene

// GameSystem processa entidades (padrão ECS).
type GameSystem = gameengine.System

// Componentes de jogo (§10).
const (
	GameTransform   = gameengine.ComponentTransform
	GamePhysics     = gameengine.ComponentPhysics
	GameRender      = gameengine.ComponentRender
	GameAnimation   = gameengine.ComponentAnimation
	GameAudio       = gameengine.ComponentAudio
	GameInput       = gameengine.ComponentInput
	GameAI          = gameengine.ComponentAI
	GameCollider    = gameengine.ComponentCollider
	GameHealth      = gameengine.ComponentHealth
	GameScore       = gameengine.ComponentScore
)

// NewGameScene cria uma cena vazia.
func NewGameScene(name string) *GameScene { return &gameengine.Scene{Name: name} }

// GameSceneAddEntity adiciona uma entidade validada.
func GameSceneAddEntity(s *GameScene, e *GameEntity) error { return s.AddEntity(e) }

// GameSceneEntitiesWith devolve entidades com um componente.
func GameSceneEntitiesWith(s *GameScene, t GameComponentType) []*GameEntity {
	return s.EntitiesWith(t)
}

// UnmarshalGameScene desserializa uma cena JSON.
func UnmarshalGameScene(data []byte) (*GameScene, error) {
	return gameengine.UnmarshalScene(data)
}

// GameComponentsList devolve os componentes válidos.
func GameComponentsList() string { return gameengine.ComponentsList() }

// =============================================================================
// Scientific Engine (§11/§12) — experimento reprodutível + COSCA LAB
// =============================================================================

// Experiment é um experimento reprodutível (§11).
type Experiment = sciengine.Experiment

// ResultKind classifica o resultado (§32).
type ResultKind = sciengine.ResultKind

// ExperimentRegistry persiste experimentos (§12).
type ExperimentRegistry = sciengine.Registry

// Kinds de resultado científico (§32).
const (
	SciObserved   = sciengine.ResultObserved
	SciCalculated = sciengine.ResultCalculated
	SciSimulated  = sciengine.ResultSimulated
	SciGenerated  = sciengine.ResultGenerated
	SciHypothesis = sciengine.ResultHypothesis
)

// OpenExperiments abre o registry de experimentos de um projeto (§12).
func OpenExperiments(projectRoot string) (*ExperimentRegistry, error) {
	return sciengine.Open(projectRoot)
}

// NewExperiment cria um experimento validado.
func NewExperiment(id, name string, kind ResultKind) (*Experiment, error) {
	return sciengine.New(id, name, kind)
}

// ExperimentAdd registra um experimento.
func ExperimentAdd(r *ExperimentRegistry, e *Experiment) error { return r.Add(e) }

// ExperimentBest devolve o melhor por métrica.
func ExperimentBest(r *ExperimentRegistry, metric string) (*Experiment, bool) {
	return r.Best(metric)
}

// AssetTypesList devolve os tipos de asset válidos.
func AssetTypesList() string { return asset.TypesList() }

// ProjectTypesList devolve os tipos de projeto válidos.
func ProjectTypesList() string { return project.TypesList() }
