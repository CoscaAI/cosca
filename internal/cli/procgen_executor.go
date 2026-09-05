package cli

import (
	"context"
	"fmt"

	"github.com/CoscaAI/cosca/internal/media"
	"github.com/CoscaAI/cosca/internal/nodegraph"
	"github.com/CoscaAI/cosca/internal/procgen"
)

// dispatchExecutor é o executor combinado do CLI: roda nós procedurais via
// procgen.Registry() (famílias Noise/Math/Pattern) e nós de mídia via
// media.Executor; tipos desconhecidos passam direto (type:id) — executores de
// tasks/models/gpu plugam nas próximas etapas. É o executor default de
// `cosca render` e `cosca ngraph run`.
type dispatchExecutor struct {
	procgen map[nodegraph.NodeType]nodegraph.Executor
}

// mediaExec é o executor de mídia compartilhado (ffmpeg/ffprobe).
var mediaExec = media.Executor{}

// newDispatchExecutor monta o executor combinado. O MetricsRecorder é
// opcional: quando dado, é injetado nos nós procedurais (embrião do
// Performance Memory — toda operação registra uma OpMetric).
func newDispatchExecutor(metrics *procgen.MetricsRecorder) *dispatchExecutor {
	registry := procgen.Registry()
	if metrics != nil {
		for _, e := range registry {
			if ms, ok := e.(procgen.MetricsSetter); ok {
				ms.SetMetrics(metrics)
			}
		}
	}
	return &dispatchExecutor{procgen: registry}
}

// Run implementa nodegraph.Executor.
func (d *dispatchExecutor) Run(ctx context.Context, node *nodegraph.Node, input map[string]any) (any, error) {
	if d != nil {
		if exec, ok := d.procgen[node.Type]; ok {
			return exec.Run(ctx, node, input)
		}
	}
	switch node.Type {
	case media.NodeLoad, media.NodeProbe, media.NodeExtractAudio, media.NodeTranscode,
		media.NodeExtractFrame, media.NodeConvertAudio, media.NodeValidate:
		return mediaExec.Run(ctx, node, input)
	default:
		return fmt.Sprintf("%s:%s", node.Type, node.ID), nil
	}
}
