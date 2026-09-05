package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

// writeProjectExamples gera os artefatos de INÍCIO do projeto conforme o tipo
// (§31 — "um prompt pode virar um projeto completo"). Cada tipo ganha um
// "kit" de exemplo que o editor já abre pronto:
//
//	game       → level.json (cena ECS §10)
//	scientific → experiment.json (experimento §11)
//	cinema     → workflow.json (pipeline §8)
//	image      → workflow.json (pipeline §7)
//	music      → workflow.json (pipeline §9)
//	3d         → scene.obj (modelo §13)
//
// Estes arquivos são o "molde" que a IA preenche (§31): o Don pede em
// linguagem natural, o agente gera o conteúdo real no mesmo schema.
func writeProjectExamples(path, projectType string) error {
	switch projectType {
	case "game":
		return writeFile(filepath.Join(path, "level.json"), gameSceneExample)
	case "scientific":
		return writeFile(filepath.Join(path, "experiment.json"), sciExperimentExample)
	case "cinema", "image", "music":
		return writeFile(filepath.Join(path, "workflow.json"), mediaWorkflowExample)
	case "3d":
		return writeFile(filepath.Join(path, "scene.obj"), objExample)
	}
	return nil
}

func writeFile(path, content string) error {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// gameSceneExample: cena ECS do COSCA GAME (§10) — player+enemy+coin.
const gameSceneExample = `{
  "name": "level-1",
  "width": 800,
  "height": 600,
  "entities": [
    {
      "id": "player",
      "name": "player",
      "components": [
        {"type": "transform", "params": {"x": 0, "y": 0}},
        {"type": "physics", "params": {"gravity": true}},
        {"type": "render", "params": {"sprite": "hero.png"}},
        {"type": "input"}
      ]
    },
    {
      "id": "enemy",
      "name": "enemy_1",
      "components": [
        {"type": "transform"},
        {"type": "ai"},
        {"type": "health", "params": {"hp": 100}}
      ]
    },
    {
      "id": "coin",
      "name": "coin_1",
      "components": [
        {"type": "transform", "params": {"x": 400, "y": 300}},
        {"type": "score", "params": {"value": 10}}
      ]
    }
  ]
}
`

// sciExperimentExample: experimento do COSCA SCIENTIFIC (§11).
const sciExperimentExample = `{
  "id": "exp-baseline",
  "name": "baseline",
  "input": ["dataset-001"],
  "parameters": {"expr": "constant", "value": 0.72},
  "code_version": "main",
  "environment": "cosca-runtime",
  "result": 0.72,
  "kind": "calculated",
  "metrics": {"result": 0.72}
}
`

// mediaWorkflowExample: pipeline do §21 (video/image/audio).
const mediaWorkflowExample = `{
  "name": "media-pipeline",
  "nodes": [
    {"id": "load", "type": "load_video", "params": {"path": "intro.mp4"}},
    {"id": "audio", "type": "extract_audio", "params": {"format": "wav"}, "inputs": ["load"]},
    {"id": "frame", "type": "frame", "params": {"time": "00:00:01"}, "inputs": ["load"]},
    {"id": "out", "type": "transcode", "params": {"quality": "draft"}, "inputs": ["load"]}
  ]
}
`

// objExample: modelo 3D de exemplo (§13) — cubo.
const objExample = `# Cubo de exemplo (COSCA 3D §13)
# 8 vértices · 6 faces · gerado pelo Cosca Engine
v -1 -1 -1
v 1 -1 -1
v 1 1 -1
v -1 1 -1
v -1 -1 1
v 1 -1 1
v 1 1 1
v -1 1 1
usemtl cosca-default
f 1 2 3 4
f 5 6 7 8
f 1 5 8 4
f 2 6 7 3
f 1 2 6 5
f 4 3 7 8
`
