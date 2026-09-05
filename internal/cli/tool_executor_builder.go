package cli

// tool_executor_builder.go — thin wrapper sobre internal/toolrun (o montador
// do executor de ferramentas CANÔNICO).
//
// Opção B (Etapa 3): o COSCA tem UM executor de ferramentas (chat/executor)
// montado num ÚNICO ponto (internal/toolrun). Estes helpers do cli apenas
// delegam — o cli e o bootstrap usam a MESMA montagem.

import (
	"github.com/CoscaAI/cosca/internal/chat/sandbox"
	"github.com/CoscaAI/cosca/internal/orchestration"
	"github.com/CoscaAI/cosca/internal/toolrun"
)

// buildCanonicalToolRunner monta o executor canônico (registry real + sandbox
// gate default + permission) e devolve o ToolRunner para o orchestration.
func buildCanonicalToolRunner(workspace, coscaDir string) orchestration.ToolRunner {
	return toolrun.Build(toolrun.Config{Workspace: workspace, CoscaDir: coscaDir})
}

// buildCanonicalToolRunnerWithGate é a variante com sandbox gate explícito
// (ex.: o terminal roda FORA da jail e precisa do sandbox per-command).
func buildCanonicalToolRunnerWithGate(workspace, coscaDir string, gate *sandbox.Gate) orchestration.ToolRunner {
	return toolrun.Build(toolrun.Config{Workspace: workspace, CoscaDir: coscaDir, Gate: gate})
}
