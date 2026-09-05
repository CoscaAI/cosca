package orchestration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNormalizeToolResults_PropagatesToResult valida a cadeia do Caminho A
// (ADR-031): pc.Data.ToolResults (interface{} com []*ToolCallResult) deve
// chegar ao Result.ToolExecutions via buildResult.normalizeToolResults.
// Sem isso, a evidência de artefato (write_file/build) fica cega no record.
func TestNormalizeToolResults_PropagatesToResult(t *testing.T) {
	pc := NewPipelineContext("test-request", "test prompt")
	pc = pc.WithToolResults([]*ToolCallResult{
		{Name: "write_file", Content: "arquivo criado", Error: ""},
		{Name: "execute_command", Content: "build ok", Error: ""},
	})

	e := &Engine{}
	result := e.buildResult(pc, time.Now())

	require.NotNil(t, result, "buildResult nao deve retornar nil")
	require.Len(t, result.ToolExecutions, 2, "deve ter 2 tool executions propagadas")

	names := make([]string, 0, len(result.ToolExecutions))
	for _, te := range result.ToolExecutions {
		names = append(names, te.Name)
	}
	assert.Equal(t, []string{"write_file", "execute_command"}, names)
}

// TestNormalizeToolResults_NilOrEmpty valida que entradas vazias/nil produzem
// um slice nil (não um slice vazio serializado como []).
func TestNormalizeToolResults_NilOrEmpty(t *testing.T) {
	assert.Nil(t, normalizeToolResults(nil), "nil deve virar nil")
	assert.Nil(t, normalizeToolResults([]*ToolCallResult{}), "slice vazio deve virar nil")
	assert.Nil(t, normalizeToolResults("not-a-slice"), "tipo inesperado deve virar nil")
	assert.Nil(t, normalizeToolResults([]*ToolCallResult{nil}), "entrada com nil deve virar nil")
}

// TestNormalizeToolResults_ValueSlice valida o case []ToolCallResult (por valor).
func TestNormalizeToolResults_ValueSlice(t *testing.T) {
	got := normalizeToolResults([]ToolCallResult{{Name: "read_file"}})
	require.Len(t, got, 1)
	assert.Equal(t, "read_file", got[0].Name)
}
