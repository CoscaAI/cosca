// Package training é o contrato de TREINAMENTO REMOTO do COSCA (V1).
//
// O professor desenhou a evolução: o Colab (ou qualquer GPU) vira um WORKER
// DESCARTÁVEL que recebe um JOB, treina e devolve artefatos. O COSCA é quem
// valida — NUNCA o worker decide promoção.
//
//   COSCA → Git Manager → publica JOB (imutável) → Colab (worker) treina
//     → retorna artefatos → COSCA valida → Golden Gate → PromotionGate.
//
// REGRA DE OURO (professor): o worker NUNCA decide promoção. Ele só entrega
// TRAINING_COMPLETE (adapter, checkpoint, metrics, logs, provenance). O
// COSCA valida e decide (PromotionGate). Mesmo que o treino remoto erre,
// não ganha acesso ao "portão da casa".
package training

import (
	"encoding/json"
	"os"
	"time"
)

// ─── Job Manifest ───────────────────────────────────────────────────────────

// Job é o contrato imutável de um job de treinamento remoto. É publicado no
// Git (via Git Manager) e consumido pelo worker. O worker treina EXATAMENTE
// o que o manifesto descreve — referência imutável.
type Job struct {
	// CampaignID identifica a campanha (ex.: "campaign-002").
	CampaignID string `json:"campaign_id"`

	// Dataset é o caminho/ref do dataset de treino (ex.: "dataset-train-002.jsonl").
	Dataset string `json:"dataset"`

	// Base é o modelo base (ex.: "Qwen/Qwen3-4B").
	Base string `json:"base"`

	// Config é a configuração de treino.
	Config TrainConfig `json:"config"`

	// Commands são os passos que o worker executa (determinístico).
	Commands []string `json:"commands"`

	// CreatedAt registra quando o job foi criado (proveniência).
	CreatedAt time.Time `json:"created_at"`

	// Revision é o commit do Git que publicou o job (referência imutável).
	Revision string `json:"revision,omitempty"`
}

// TrainConfig é a configuração de um job de treino LoRA.
type TrainConfig struct {
	LoRA        LoRAConfig `json:"lora"`
	Epochs      int        `json:"epochs"`
	LearningRate float64   `json:"learning_rate"`
	MaxSeqLen   int        `json:"max_seq_len"`
	QuantMethod string     `json:"quant_method,omitempty"` // ex.: "q4_k_m"
}

// LoRAConfig é a configuração do adaptador LoRA.
type LoRAConfig struct {
	R       int      `json:"r"`
	Alpha   int      `json:"alpha"`
	Targets []string `json:"targets"`
}

// ─── Resultado do worker ────────────────────────────────────────────────────

// WorkerReport é o que o worker RETORNA após treinar. IMPORTANTE: o worker
// NUNCA decide promoção — só entrega o resultado e metadados; o COSCA valida.
type WorkerReport struct {
	CampaignID string    `json:"campaign_id"`
	Status     string    `json:"status"` // "TRAINING_COMPLETE" | "FAILED"
	Adapter    string    `json:"adapter,omitempty"`    // caminho/ref do adaptador
	Checkpoint string    `json:"checkpoint,omitempty"` // caminho/ref do checkpoint
	Metrics    map[string]float64 `json:"metrics,omitempty"` // loss, etc.
	Logs       string    `json:"logs,omitempty"`       // path do log
	Error      string    `json:"error,omitempty"`
	FinishedAt time.Time `json:"finished_at"`
}

// ─── Persistência do job ────────────────────────────────────────────────────

// jobDir é o diretório onde os jobs ficam versionados (.cosca/jobs).
const jobDir = ".cosca/jobs"

// DefaultJobDir retorna o diretório de jobs do COSCA.
func DefaultJobDir(root string) string {
	if root == "" {
		root = "."
	}
	return root + "/" + jobDir
}

// NewJob cria um job de treinamento com configs padrão (campanha 002).
func NewJob(campaignID, dataset, base string) Job {
	return Job{
		CampaignID: campaignID,
		Dataset:    dataset,
		Base:       base,
		Config: TrainConfig{
			LoRA: LoRAConfig{
				R:       16,
				Alpha:   32,
				Targets: []string{"q_proj", "k_proj", "v_proj", "o_proj", "gate_proj", "up_proj", "down_proj"},
			},
			Epochs:      3,
			LearningRate: 2e-4,
			MaxSeqLen:    8192,
			QuantMethod:  "q4_k_m",
		},
		CreatedAt: time.Now().UTC(),
	}
}

// Write persiste o job como JSON no diretório de jobs (para o Git versionar).
func (j Job) Write(root string) (string, error) {
	dir := DefaultJobDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := dir + "/" + j.CampaignID + ".json"
	b, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
