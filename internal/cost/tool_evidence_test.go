package cost

import "testing"

// TestApplyToolEvidence_WriteFile gera artifact_value para escrita de arquivo.
func TestApplyToolEvidence_WriteFile(t *testing.T) {
	var r Record
	r.ApplyToolEvidence("write_file", "arquivo criado", true)
	if r.ArtifactValue != 1 || r.TaskProgress != 1 {
		t.Fatalf("esperava artifact=1/task=1, got artifact=%v task=%v", r.ArtifactValue, r.TaskProgress)
	}
}

// TestApplyToolEvidence_BuildCommand gera evidence_gain para build/test.
func TestApplyToolEvidence_BuildCommand(t *testing.T) {
	var r Record
	r.ApplyToolEvidence("execute_command", "build succeeded", true)
	if r.EvidenceGain != 1 || r.TaskProgress != 1 {
		t.Fatalf("esperava evidence=1/task=1, got evidence=%v task=%v", r.EvidenceGain, r.TaskProgress)
	}
}

// TestApplyToolEvidence_GenericCommand nao infla: ls/cat/pwd nao sao evidencia.
func TestApplyToolEvidence_GenericCommand(t *testing.T) {
	var r Record
	r.ApplyToolEvidence("execute_command", "file1\nfile2\n", true)
	if r.EvidenceGain != 0 || r.TaskProgress != 0 {
		t.Fatalf("comando generico nao deveria gerar evidencia, got evidence=%v task=%v", r.EvidenceGain, r.TaskProgress)
	}
}

// TestApplyToolEvidence_FailedWrite nao infla: write falho nao gera artifact.
func TestApplyToolEvidence_FailedWrite(t *testing.T) {
	var r Record
	r.ApplyToolEvidence("write_file", "erro ao escrever", false)
	if r.ArtifactValue != 0 {
		t.Fatalf("write falho nao deveria gerar artifact, got %d", r.ArtifactValue)
	}
}
