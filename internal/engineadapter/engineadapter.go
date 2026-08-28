// Package engineadapter é a abstração de CORPO (ADR-024 §2.3) — o target de
// engine que materializa um WorldSpec. É DISTINTA do adapter de ferramenta
// (worldmodel.Adapter / CLIP-SAM-Whisper): esta é o corpotarget (Unreal/Roblox/Blender).
//
// O COSCA emite o WorldSpec (o "mundo que quero representar"); o adapter resolve
// como materializar no corpo. Fina, aditiva, stdlib-only.
package engineadapter

import (
	"context"

	"github.com/CoscaAI/cosca/internal/worldspec"
)

// ExecutionHandle é o controle de um mundo materializado (runtime).
type ExecutionHandle struct {
	Target  string
	Summary string
	Files   []string
	// WorldHash é o hash de conteúdo do WorldSpec materializado (versionamento I5).
	WorldHash string
}

// EngineAdapter materializa um WorldSpec num corpo/engine.
type EngineAdapter interface {
	// Target devolve o nome do corpo ("unreal" | "roblox" | "blender").
	Target() string
	// Materialize materializa o WorldSpec, devolvendo um handle de execução.
	Materialize(ctx context.Context, spec worldspec.WorldSpec) (ExecutionHandle, error)
}
