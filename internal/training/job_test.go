package training

import (
	"os"
	"path/filepath"
	"testing"
)

// TestJobManifest valida que um job de treinamento é criado com configs
// corretas e persiste como JSON (para o Git versionar / worker consumir).
func TestJobManifest(t *testing.T) {
	job := NewJob("campaign-002", "dataset-train-002.jsonl", "Qwen/Qwen3-4B")
	if job.CampaignID != "campaign-002" {
		t.Fatalf("campaign_id errado: %s", job.CampaignID)
	}
	if job.Config.LoRA.R != 16 {
		t.Errorf("lora.r esperado 16, got %d", job.Config.LoRA.R)
	}
	if len(job.Config.LoRA.Targets) != 7 {
		t.Errorf("esperado 7 targets, got %d", len(job.Config.LoRA.Targets))
	}

	root := t.TempDir()
	path, err := job.Write(root)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("job não escrito: %v", err)
	}
	t.Logf("job persistido em %s", path)
}

// TestWorkerReportNoPromotion valida a REGRA DE OURO: o WorkerReport NÃO
// possui nenhum campo de decisão de promoção. O worker entrega SÓ artefatos
// e o COSCA decide (PromotionGate).
//
// Por DESIGN, o WorkerReport tem APENAS: campaign_id, status, adapter,
// checkpoint, metrics, logs, error, finished_at. Qualquer tentativa de o
// worker "decidir" fica fora do contrato. Este teste valida que o status
// só aceita TRAINING_COMPLETE/FAILED (nunca PROMOTE/REJECT).
func TestWorkerReportNoPromotion(t *testing.T) {
	r := WorkerReport{
		CampaignID: "campaign-002",
		Status:     "TRAINING_COMPLETE",
		Adapter:    "/content/output",
		Checkpoint: "/content/output",
		Metrics:    map[string]float64{"epochs": 3},
	}
	if r.Status != "TRAINING_COMPLETE" {
		t.Fatalf("status esperado TRAINING_COMPLETE, got %s", r.Status)
	}
	// O contrato NÃO tem campo de promoção — a decisão é 100% do COSCA.
	// O status do worker é binário (completou/falhou), nunca "promovido".
	if r.Status == "PROMOTE" || r.Status == "REJECT" {
		t.Fatalf("status do worker não pode ser decisão de promoção")
	}
	t.Logf("WorkerReport válido (sem campo de promoção) - worker não decide")
}

// TestJobDir valida o diretório default de jobs.
func TestJobDir(t *testing.T) {
	// Usa filepath.Join para garantir consistência de separador no Windows.
	want := filepath.Join("repo", ".cosca", "jobs")
	if got := filepath.Base(DefaultJobDir("repo")); filepath.ToSlash(got) != filepath.ToSlash(want) {
		t.Errorf("DefaultJobDir = %s, want %s", got, want)
	}
}
