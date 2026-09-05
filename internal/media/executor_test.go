package media

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/nodegraph"
)

func n(id string, typ nodegraph.NodeType, inputs []string, params map[string]any) *nodegraph.Node {
	return &nodegraph.Node{ID: id, Type: typ, Inputs: inputs, Params: params}
}

func TestExecutorLoadReturnsPath(t *testing.T) {
	ex := Executor{}
	got, err := ex.Run(context.Background(), n("load", NodeLoad, nil, map[string]any{"path": "video.mp4"}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "video.mp4" {
		t.Fatalf("load output = %v, want video.mp4", got)
	}
}

func TestExecutorUnknownType(t *testing.T) {
	ex := Executor{}
	_, err := ex.Run(context.Background(), n("x", "nope", nil, nil), nil)
	if err == nil {
		t.Fatal("want error for unknown node type")
	}
	if !strings.Contains(err.Error(), "unknown node type") {
		t.Fatalf("err = %v, want unknown node type", err)
	}
}

func TestExecutorMissingInputOutput(t *testing.T) {
	ex := Executor{}
	// Nó com input declarado mas sem saída disponível.
	_, err := ex.Run(context.Background(), n("seg", NodeTranscode, []string{"load"}, nil), nil)
	if err == nil || !strings.Contains(err.Error(), "has no output") {
		t.Fatalf("err = %v, want 'has no output'", err)
	}
	// Nó de processo sem params["out"].
	_, err = ex.Run(context.Background(), n("seg", NodeTranscode, nil, map[string]any{"path": "a.mp4"}), nil)
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("err = %v, want 'out is required'", err)
	}
}

func TestExecutorInputWrongType(t *testing.T) {
	ex := Executor{}
	node := n("seg", NodeProbe, []string{"load"}, nil)
	_, err := ex.Run(context.Background(), node, map[string]any{"load": 42})
	if err == nil || !strings.Contains(err.Error(), "want string path") {
		t.Fatalf("err = %v, want 'want string path'", err)
	}
}

// TestExecutorEndToEnd roda o pipeline §21 (VIDEO → EXTRACT_AUDIO) como grafo.
func TestExecutorEndToEnd(t *testing.T) {
	hasFFmpeg(t)
	dir := t.TempDir()
	video := makeTestVideo(t, dir)
	audio := filepath.Join(dir, "out.wav")

	g, err := nodegraph.Build("extract", []*nodegraph.Node{
		n("load", NodeLoad, nil, map[string]any{"path": video}),
		n("audio", NodeExtractAudio, []string{"load"}, map[string]any{"out": audio}),
		n("check", NodeValidate, []string{"audio"}, nil),
	})
	if err != nil {
		t.Fatal(err)
	}

	stats, err := g.Run(context.Background(), Executor{}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if stats.Executed != 3 {
		t.Fatalf("Executed = %d, want 3", stats.Executed)
	}
	// O nó extract_audio devolve o caminho de saída.
	if stats.Results["audio"] != audio {
		t.Fatalf("audio result = %v, want %s", stats.Results["audio"], audio)
	}
	// O nó validate devolve *Info com stream de áudio.
	info, ok := stats.Results["check"].(*Info)
	if !ok {
		t.Fatalf("check result = %T, want *Info", stats.Results["check"])
	}
	if !info.HasAudio {
		t.Fatal("validated output should have an audio stream")
	}
}

// TestExecutorGraphWithCache prova que o cache por assinatura (§23) evita
// re-executar mídia quando nada mudou.
func TestExecutorGraphWithCache(t *testing.T) {
	hasFFmpeg(t)
	dir := t.TempDir()
	video := makeTestVideo(t, dir)
	audio := filepath.Join(dir, "out.wav")

	g, err := nodegraph.Build("extract", []*nodegraph.Node{
		n("load", NodeLoad, nil, map[string]any{"path": video}),
		n("audio", NodeExtractAudio, []string{"load"}, map[string]any{"out": audio}),
	})
	if err != nil {
		t.Fatal(err)
	}

	cache := nodegraph.NewCache()
	first, err := g.Run(context.Background(), Executor{}, &nodegraph.RunOptions{Cache: cache})
	if err != nil {
		t.Fatal(err)
	}
	if first.Executed != 2 || first.CachedHits != 0 {
		t.Fatalf("first run: Executed=%d CachedHits=%d, want 2/0", first.Executed, first.CachedHits)
	}

	// Segunda execução: tudo em cache — zero execução real.
	second, err := g.Run(context.Background(), Executor{}, &nodegraph.RunOptions{Cache: cache})
	if err != nil {
		t.Fatal(err)
	}
	if second.Executed != 0 || second.CachedHits != 2 {
		t.Fatalf("second run: Executed=%d CachedHits=%d, want 0/2", second.Executed, second.CachedHits)
	}
}
