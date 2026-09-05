// Package media — Executor de nós (§16/§21 do manifesto Creative/Scientific/Media).
//
// O Executor implementa nodegraph.Executor e é a PONTE entre o Node Graph
// (etapa 1.6) e a Media Engine (etapa 1.8): cada operação de mídia vira um nó
// que RODA de verdade. Antes deste arquivo, o render usava executor
// pass-through; aqui os tipos de nó (load, probe, extract_audio, transcode,
// extract_frame, convert_audio, validate) executam as funções reais.
//
// Convenção de fluxo (princípio P1 do Blueprint — grafo = dados puros):
//   - nó `load` devolve o caminho (params["path"]) — é a FONTE.
//   - nós de processo devolvem o caminho de SAÍDA (params["out"]) — encadeiam.
//   - nós `probe`/`validate` devolvem *Info (metadados) — terminais.
//
// A entrada de um nó é resolvida assim: se o nó tem Inputs, usa a saída do
// primeiro input (deve ser string = caminho); senão, usa params["path"].
package media

import (
	"context"
	"fmt"

	"github.com/CoscaAI/cosca/internal/nodegraph"
)

// Tipos de nó de mídia (contrato do §21).
const (
	NodeLoad         nodegraph.NodeType = "load"
	NodeProbe        nodegraph.NodeType = "probe"
	NodeExtractAudio nodegraph.NodeType = "extract_audio"
	NodeTranscode    nodegraph.NodeType = "transcode"
	NodeExtractFrame nodegraph.NodeType = "extract_frame"
	NodeConvertAudio nodegraph.NodeType = "convert_audio"
	NodeValidate     nodegraph.NodeType = "validate"
)

// Executor executa nós de mídia via ffmpeg/ffprobe. É stateless e reutilizável
// — o estado fica no node graph (dados puros) e no cache por assinatura (§23).
type Executor struct{}

// Run implementa nodegraph.Executor.
func (Executor) Run(ctx context.Context, node *nodegraph.Node, input map[string]any) (any, error) {
	in, err := resolveInputPath(node, input)
	if err != nil {
		return nil, err
	}

	switch node.Type {
	case NodeLoad:
		// Fonte: devolve o caminho para os nós seguintes.
		return in, nil

	case NodeProbe:
		return Probe(ctx, in)

	case NodeValidate:
		return Validate(ctx, in)

	case NodeExtractAudio:
		out, err := requiredOut(node)
		if err != nil {
			return nil, err
		}
		return out, ExtractAudio(ctx, in, out, pipeOptions(node.Params))

	case NodeTranscode:
		out, err := requiredOut(node)
		if err != nil {
			return nil, err
		}
		return out, Transcode(ctx, in, out, pipeOptions(node.Params))

	case NodeExtractFrame:
		out, err := requiredOut(node)
		if err != nil {
			return nil, err
		}
		t := strParam(node.Params, "t")
		if t == "" {
			t = "0"
		}
		return out, ExtractFrame(ctx, in, out, t, pipeOptions(node.Params))

	case NodeConvertAudio:
		out, err := requiredOut(node)
		if err != nil {
			return nil, err
		}
		return out, ConvertAudio(ctx, in, out, audioPipeOptions(node.Params))

	default:
		return nil, fmt.Errorf("media: unknown node type %q", node.Type)
	}
}

// resolveInputPath devolve o caminho de entrada de um nó: a saída do primeiro
// input (se houver inputs) ou params["path"]. Garante que o valor é string.
func resolveInputPath(node *nodegraph.Node, input map[string]any) (string, error) {
	if len(node.Inputs) > 0 {
		srcID := node.Inputs[0]
		v, ok := input[srcID]
		if !ok {
			return "", fmt.Errorf("media: node %q: input %q has no output", node.ID, srcID)
		}
		s, ok := v.(string)
		if !ok {
			return "", fmt.Errorf("media: node %q: input %q output is %T, want string path", node.ID, srcID, v)
		}
		return s, nil
	}
	return strParam(node.Params, "path"), nil
}

// requiredOut devolve params["out"] ou erro — todo nó de processo precisa de
// um destino (§21: PROCESS → ... → EXPORT).
func requiredOut(node *nodegraph.Node) (string, error) {
	out := strParam(node.Params, "out")
	if out == "" {
		return "", fmt.Errorf("media: node %q (%s): param %q is required", node.ID, node.Type, "out")
	}
	return out, nil
}

// =============================================================================
// Parsing de params (map[string]any → structs tipados)
// =============================================================================

func strParam(params map[string]any, key string) string {
	if params == nil {
		return ""
	}
	v, ok := params[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func boolParam(params map[string]any, key string) bool {
	if params == nil {
		return false
	}
	v, ok := params[key]
	if !ok {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true" || t == "1"
	}
	return false
}

// strSliceParam aceita []string ou []any (JSON desserializa como []any).
func strSliceParam(params map[string]any, key string) []string {
	if params == nil {
		return nil
	}
	v, ok := params[key]
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func pipeOptions(params map[string]any) PipeOptions {
	return PipeOptions{
		HWAccel:   boolParam(params, "hwaccel"),
		Quality:   strParam(params, "quality"),
		ExtraArgs: strSliceParam(params, "extra_args"),
	}
}

func audioPipeOptions(params map[string]any) AudioPipeOptions {
	return AudioPipeOptions{
		Volume:     strParam(params, "volume"),
		SampleRate: strParam(params, "sample_rate"),
		Channels:   strParam(params, "channels"),
	}
}
