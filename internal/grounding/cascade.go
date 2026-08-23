package grounding

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
)

// Env vars do kill-switch de fidelity (paridade com MCE_RERANKER_ENABLED —
// precedência env > config > default, aqui só env é implementado pois não há
// config de grounding ainda).
const (
	// EnvGroundingEnabled liga a cascata de gates (default false —
	// comportamento atual, zero regressão).
	EnvGroundingEnabled = "COSCA_GROUNDING_ENABLED"
	// EnvGroundingThreshold sobrepõe o piso de fidelidade (default 0.60).
	EnvGroundingThreshold = "COSCA_GROUNDING_THRESHOLD"
	// EnvGroundingHHEM liga a fase 2 (NLI local); stub por ora.
	EnvGroundingHHEM = "COSCA_GROUNDING_HHEM_ENABLED"
)

// ErrGateNotWired é o erro devolvido pelo HHEMGate (fase 2): o NLI local
// cross-encoder (~400MB ONNX) não entra na fatia 1. Escolher o HHEMGate
// hoje é um erro explícito, não um silent-fail.
var ErrGateNotWired = errors.New("grounding: HHEM NLI gate não está ligado (fase 2)")

// CascadeVerdict é o resultado de um gate da cascata de fidelidade: o
// veredito mais o relatório por claim para auditoria reversível até os chunks.
type CascadeVerdict struct {
	// Verdict é block | flag | deliver.
	Verdict Verdict
	// Report é o relatório de fidelidade (nil quando o gate não calculou).
	Report *FidelityReport
	// GaterID identifica qual gate produziu o veredito ("heuristic" |
	// "hhem" | "passthrough").
	GaterID string
}

// Gater é a interface da cascata de fidelidade. A fatia 1 tem apenas o
// HeuristicGater; o HHEMGate é o gate condicional da fase 2.
type Gater interface {
	// Verify devolve o veredito da cascata para os claims extraídos da
	// resposta, contra os chunks recuperados.
	Verify(ctx context.Context, query string, claims []Claim, chunks []SourceChunk) (CascadeVerdict, error)
}

// Config é a configuração do gate de fidelidade lida do ambiente.
type Config struct {
	// Enabled liga a cascata. false = comportamento atual (deliver, sem gate).
	Enabled bool
	// Threshold é o piso de fidelidade (default DefaultFaithfulnessThreshold).
	Threshold float64
	// HHEMEnabled é a fase 2 (não implementada; stub).
	HHEMEnabled bool
}

// DefaultConfig devolve a configuração default (disabled). É o "modo atual".
func DefaultConfig() Config {
	return Config{Enabled: false, Threshold: DefaultFaithfulnessThreshold, HHEMEnabled: false}
}

// EnvConfig devolve a configuração a partir do ambiente (kill-switch C3): env
// ausente/inválido cai no default.
func EnvConfig() Config {
	cfg := DefaultConfig()
	if v := strings.TrimSpace(os.Getenv(EnvGroundingEnabled)); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Enabled = b
		}
	}
	if v := strings.TrimSpace(os.Getenv(EnvGroundingThreshold)); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			cfg.Threshold = f
		}
	}
	if v := strings.TrimSpace(os.Getenv(EnvGroundingHHEM)); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.HHEMEnabled = b
		}
	}
	return cfg
}

// HeuristicGater implementa a cascata heurística de fidelidade (C4/self-RAG
// zero-LLM): verifica os claims por token-overlap + bônus de n-grama/número
// via VerifyClaim e devolve o veredito do FidelityReport.
type HeuristicGater struct {
	// Enabled liga o gate. false → sempre deliver (fail-open, modo atual).
	Enabled bool
	// Threshold é o piso de fidelidade (default DefaultFaithfulnessThreshold).
	Threshold float64
	// VerifyOpts são as opções da verificação por claim.
	VerifyOpts VerifyOptions
}

// NewHeuristicGater cria um HeuristicGater a partir de um Config.
func NewHeuristicGater(cfg Config) *HeuristicGater {
	return &HeuristicGater{
		Enabled:    cfg.Enabled,
		Threshold:  cfg.Threshold,
		VerifyOpts: VerifyOptions{Threshold: cfg.Threshold},
	}
}

// Verify implementa Gater. Quando desligado, devolve deliver (fail-open) sem
// calcular nada. Quando ligado, verifica os claims e devolve o veredito.
func (g *HeuristicGater) Verify(ctx context.Context, query string, claims []Claim, chunks []SourceChunk) (CascadeVerdict, error) {
	if g == nil || !g.Enabled {
		return CascadeVerdict{Verdict: VerdictDeliver, GaterID: "heuristic"}, nil
	}
	if len(chunks) == 0 {
		rep := FidelityReport{Faithfulness: -1, Total: 0, Threshold: g.Threshold}
		return CascadeVerdict{Verdict: rep.Verdict(), Report: &rep, GaterID: "heuristic"}, nil
	}

	opts := g.VerifyOpts
	if opts.Threshold <= 0 {
		opts.Threshold = g.Threshold
	}

	verdicts := make([]ClaimVerdict, 0, len(claims))
	supported := 0
	for _, c := range claims {
		v := VerifyClaim(query, c, chunks, opts)
		verdicts = append(verdicts, v)
		if v.Supported {
			supported++
		}
	}

	rep := FidelityReport{Claims: verdicts, Threshold: opts.Threshold}
	total := len(verdicts)
	rep.Total = total
	if total > 0 {
		rep.Supported = supported
		rep.Faithfulness = round4(float64(supported) / float64(total))
	} else {
		rep.Faithfulness = -1
	}

	return CascadeVerdict{Verdict: rep.Verdict(), Report: &rep, GaterID: "heuristic"}, nil
}

// HHEMGate é o stub da fase 2 (NLI local cross-encoder). Não está ligado;
// qualquer chamada devolve ErrGateNotWired — o ADR §6 coloca HHEM como phaser
// 2 condicionada a evidência de resíduo de infidelidade.
type HHEMGate struct{}

// Verify devolve sempre o erro de "não ligado" — nunca um veredito silencioso.
func (HHEMGate) Verify(context.Context, string, []Claim, []SourceChunk) (CascadeVerdict, error) {
	return CascadeVerdict{}, ErrGateNotWired
}

// Choose devolve o gate default da cascata. É SEMPRE heuristic-only na fatia
// 1 (nunca escolhe o HHEM), lendo o kill-switch de ambiente. Um gate
// desligado (COSCA_GROUNDING_ENABLED=false) é um *HeuristicGater com
// Enabled=false — fail-open, comportamento atual.
func Choose() Gater {
	cfg := EnvConfig()
	if cfg.HHEMEnabled {
		return HHEMGate{}
	}
	return NewHeuristicGater(cfg)
}

// WithConfig devolve o gate escolhido a partir de um Config explícito
// (útil para injeção em testes e para o chamador).
func WithConfig(cfg Config) Gater {
	if cfg.HHEMEnabled {
		return HHEMGate{}
	}
	return NewHeuristicGater(cfg)
}
