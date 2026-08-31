package orchestration

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/deliberate"
	"github.com/CoscaAI/cosca/internal/shadow"
)

// ─── DeliberateConfig ────────────────────────────────────────────────────────
//
// DeliberateConfig configures the Kernel-First Deliberation stage (ADR-032).
// It is a feature flag: when Enabled is false (the default, fail-closed), the
// orchestration flow is EXACTLY the current one — the stage is a no-op and the
// pipeline context passes through unchanged.
type DeliberateConfig struct {
	// Enabled is the feature flag. Default false (fail-closed / LEI DO COFRE):
	// when false, the deliberation stage never runs and the legacy path is
	// preserved bit-for-bit.
	Enabled bool

	// ShadowMode is the observability flag (ADR-033). When true (and Enabled is
	// false), the deliberation stage runs in SHADOW mode: it computes the same
	// verdict but NEVER changes the requested response — it only records the
	// counterfactual decision (ShadowTrace) in `.cosca/shadow/`. Default false.
	ShadowMode bool

	// EmitThreshold is the confidence at or above which the Kernel responds
	// WITHOUT the LLM (EmitOK). Default 0.70 (deliberate A4).
	EmitThreshold float64

	// ReservationThreshold is the confidence at or above which the Kernel
	// calls the LLM but attaches reservations (EmitWithReservations). Below it
	// the Kernel escalates. Default 0.50 (deliberate A4).
	ReservationThreshold float64

	// MaxEvidence bounds the top-N evidence entries in the clean context.
	// Default 5.
	MaxEvidence int

	// MaxCharsPerEvidence truncates each evidence entry in the clean context.
	// Default 300.
	MaxCharsPerEvidence int

	// MinScore is the minimum evidence score for a result to become a
	// Position. Default 0.50.
	MinScore float64

	// Weights are the A3 convergence weights. When zero-valued, the deliberate
	// defaults are used (0.30/0.25/0.25/0.20).
	Weights deliberate.ConvergenceWeights
}

// DefaultDeliberateConfig returns the fail-closed default: disabled, with the
// deliberate A4 thresholds. Enabled=false preserves the current behaviour.
func DefaultDeliberateConfig() DeliberateConfig {
	return DeliberateConfig{
		Enabled:              false,
		ShadowMode:           false,
		EmitThreshold:        deliberate.DefaultEmitThreshold,
		ReservationThreshold: 0.50,
		MaxEvidence:          5,
		MaxCharsPerEvidence:  300,
		MinScore:             0.50,
		Weights:              deliberate.DefaultConvergenceWeights(),
	}
}

// normalizeDeliberateConfig fills any zero-valued field with its default so the
// deliberator always has deterministic, safe thresholds.
func normalizeDeliberateConfig(cfg DeliberateConfig) DeliberateConfig {
	def := DefaultDeliberateConfig()
	if cfg.EmitThreshold <= 0 {
		cfg.EmitThreshold = def.EmitThreshold
	}
	if cfg.ReservationThreshold <= 0 {
		cfg.ReservationThreshold = def.ReservationThreshold
	}
	if cfg.MaxEvidence <= 0 {
		cfg.MaxEvidence = def.MaxEvidence
	}
	if cfg.MaxCharsPerEvidence <= 0 {
		cfg.MaxCharsPerEvidence = def.MaxCharsPerEvidence
	}
	if cfg.MinScore <= 0 {
		cfg.MinScore = def.MinScore
	}
	if cfg.Weights == (deliberate.ConvergenceWeights{}) {
		cfg.Weights = deliberate.DefaultConvergenceWeights()
	}
	return cfg
}

// DeliberateConfigFromConfig maps the YAML-facing deliberation section
// (internal/config) into the engine's DeliberateConfig. Fail-closed (LEI DO
// COFRE): Enabled is only true when explicitly set in the config — an absent
// section yields the engine default (disabled, DefaultDeliberateConfig).
// Thresholds are forwarded as-is; normalizeDeliberateConfig fills any zero
// with the A4 default when the deliberator runs.
func DeliberateConfigFromConfig(cfg config.DeliberationConfig) DeliberateConfig {
	return DeliberateConfig{
		Enabled:              cfg.Enabled,
		ShadowMode:           cfg.ShadowMode,
		EmitThreshold:        cfg.EmitThreshold,
		ReservationThreshold: cfg.ReservationThreshold,
		MaxEvidence:          cfg.MaxEvidence,
		MaxCharsPerEvidence:  cfg.MaxCharsPerEvidence,
		MinScore:             cfg.MinScore,
		Weights:              deliberate.DefaultConvergenceWeights(),
	}
}

// ─── DeliberationTrace ───────────────────────────────────────────────────────
//
// DeliberationTrace is the audit trail of the Kernel-First Deliberation
// (ADR-032 §3.6). It answers "why did the Kernel decide?" — the verdict, the
// arithmetic convergence, the confidence breakdown, the positions and the
// evidence IDs that fed the decision. It is deterministic and serializable.
type DeliberationTrace struct {
	Verdict      deliberate.Emit
	Convergence  float64
	Confidence   deliberate.ConfidenceBreakdown
	Positions    []deliberate.Position
	Adjustments  []deliberate.Adjustment
	EvidenceIDs  []string
	Deterministic bool // true = the Kernel responded WITHOUT the LLM (EmitOK)
	Timestamp    time.Time
}

// ─── Deliberator ─────────────────────────────────────────────────────────────
//
// Deliberator is the deterministic Kernel-First Deliberation stage (ADR-032).
// It consumes the evidence the pipeline already collected (knowledge results,
// memory results) and runs the deliberate arithmetic gates — convergence,
// confidence breakdown, emit verdict — with ZERO LLM. It never blocks the
// legacy path: on any error it returns a zero trace and the caller falls
// through to the Executor unchanged (fail-closed).
type Deliberator struct {
	config DeliberateConfig
}

// NewDeliberator creates a Deliberator with normalized, fail-closed config.
func NewDeliberator(cfg DeliberateConfig) *Deliberator {
	return &Deliberator{config: normalizeDeliberateConfig(cfg)}
}

// collectPositions builds deliberate.Position values from the evidence the
// pipeline already collected (reuse, not duplication). "Zero achismo": a
// Position is only created with non-empty EvidenceIDs — no evidence, no
// Position, and the Kernel concludes "uncertain" (escalates to the LLM).
func (d *Deliberator) collectPositions(data PipelineData) []deliberate.Position {
	var positions []deliberate.Position

	// Knowledge results → recommendation stance (the "what to do" evidence).
	if kr := data.KnowledgeResults; kr != nil {
		for i := range kr.Results {
			r := &kr.Results[i]
			if r.Score < d.config.MinScore {
				continue
			}
			claim := r.Snippet
			if claim == "" {
				claim = truncateString(r.Content, d.config.MaxCharsPerEvidence)
			}
			if claim == "" {
				continue
			}
			positions = append(positions, deliberate.NewPosition(
				"ev:"+r.ID,
				claim,
				data.ResolvedAgent,
				deliberate.DimensionRecommendation,
				[]string{r.ID},
			))
		}
	}

	// Memory results → premises stance (the factual/rationale evidence).
	for _, m := range data.MemoryResults {
		if m.Content == "" {
			continue
		}
		positions = append(positions, deliberate.NewPosition(
			"ev:"+m.ID,
			truncateString(m.Content, d.config.MaxCharsPerEvidence),
			data.ResolvedAgent,
			deliberate.DimensionPremises,
			[]string{m.ID},
		))
	}

	return positions
}

// collectAdjustments derives typed confidence adjustments from the collected
// positions (ADR-032 §3.2). Currently it docks for unresolved contradictions
// between positions in the same dimension. Corroboration and critic
// adjustments are reserved for the enrichment phase (ADR-032 Fase 2).
func (d *Deliberator) collectAdjustments(positions []deliberate.Position) []deliberate.Adjustment {
	var adjustments []deliberate.Adjustment
	if hasContradiction(positions) {
		adjustments = append(adjustments, deliberate.UnresolvedContradictionAdjustment())
	}
	return adjustments
}

// Deliberate runs the deterministic Kernel-First Deliberation on the pipeline
// context. It returns the audit trail; it does NOT mutate the context — the
// caller (Engine.Execute) applies the verdict (EmitOK → deterministic
// response; otherwise → clean context for the LLM).
func (d *Deliberator) Deliberate(ctx context.Context, pc PipelineContext) (DeliberationTrace, error) {
	logger := log.Ctx(ctx).With().Str("stage", "deliberation").Str("request_id", pc.RequestID).Logger()

	positions := d.collectPositions(pc.Data)
	adjustments := d.collectAdjustments(positions)

	convergence, _ := deliberate.ComputeConvergence(positions, d.config.Weights)
	breakdown := deliberate.ComputeConfidence(convergence, adjustments)
	verdict := evaluateEmitWithThresholds(breakdown, d.config)

	trace := DeliberationTrace{
		Verdict:       verdict,
		Convergence:   convergence,
		Confidence:    breakdown,
		Positions:     positions,
		Adjustments:   adjustments,
		EvidenceIDs:   collectEvidenceIDs(positions),
		Deterministic: verdict == deliberate.EmitOK,
		Timestamp:     time.Now(),
	}

	logger.Info().
		Str("verdict", string(verdict)).
		Float64("convergence", convergence).
		Float64("confidence", breakdown.Final).
		Int("positions", len(positions)).
		Int("evidence", len(trace.EvidenceIDs)).
		Msg("deliberation: kernel decided")

	return trace, nil
}

// evaluateEmitWithThresholds applies the configured Emit/Reservation
// thresholds to a confidence breakdown. It defaults to the deliberate A4
// thresholds (0.70 / 0.50) when the config is unset, matching EvaluateEmit.
func evaluateEmitWithThresholds(b deliberate.ConfidenceBreakdown, cfg DeliberateConfig) deliberate.Emit {
	emit := cfg.EmitThreshold
	if emit <= 0 {
		emit = deliberate.DefaultEmitThreshold
	}
	res := cfg.ReservationThreshold
	if res <= 0 {
		res = 0.50
	}
	switch {
	case b.Final >= emit:
		return deliberate.EmitOK
	case b.Final >= res:
		return deliberate.EmitWithReservations
	default:
		return deliberate.Escalate
	}
}

// collectEvidenceIDs flattens the evidence IDs of all positions, deduplicated
// and sorted for a deterministic trace.
func collectEvidenceIDs(positions []deliberate.Position) []string {
	seen := map[string]struct{}{}
	var ids []string
	for _, p := range positions {
		for _, id := range p.EvidenceIDs {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

// ─── Shadow mapping (ADR-033 §3.3) ────────────────────────────────────────────

// toShadowTrace mapeia um DeliberationTrace + PipelineContext para a observação
// contrafactual do Cognitive Shadow Mode. A taxonomia segue a regra "zero
// achismo": sem posição efetiva → RETRIEVAL_INSUFFICIENT (o Kernel teria
// escalado). A decisão NUNCA é aplicada — é apenas o que o Kernel TERIA feito.
func toShadowTrace(pc PipelineContext, trace DeliberationTrace, elapsed time.Duration) shadow.ShadowTrace {
	effective := effectivePositionCount(trace.Positions)
	decision := shadow.Classify(trace.Verdict, effective)
	st := shadow.ShadowTrace{
		RequestID:     pc.RequestID,
		Agent:         pc.Data.ResolvedAgent,
		Decision:      decision,
		WouldEscalate: decision.WillEscalate(),
		Confidence:    trace.Confidence.Final,
		Convergence:   trace.Convergence,
		EvidenceIDs:   trace.EvidenceIDs,
		Positions:     effective,
		Reason:        shadowReason(trace),
		Breakdown:     trace.Confidence.Breakdown,
		DurationMs:    elapsed.Milliseconds(),
		At:            time.Now(),
	}
	if decision == shadow.EMIT_OK {
		st.WouldRespond = BuildDeterministicResponse(pc.Data, trace)
	}
	return st
}

// effectivePositionCount conta as posições EFETIVAS (Position.Effective():
// substanciada + com EvidenceIDs) — o 'zero achismo' do ADR-033 §3.3.
func effectivePositionCount(positions []deliberate.Position) int {
	n := 0
	for _, p := range positions {
		if p.Effective() {
			n++
		}
	}
	return n
}

// shadowReason produz o motivo humano-legível da decisão contrafactual.
func shadowReason(trace DeliberationTrace) string {
	if effectivePositionCount(trace.Positions) == 0 {
		return "no substantiated evidence (retrieval insufficient)"
	}
	if hasContradiction(trace.Positions) {
		return "conflicting evidence"
	}
	switch trace.Verdict {
	case deliberate.EmitOK:
		return "converged; kernel would answer without LLM"
	case deliberate.EmitWithReservations:
		return "partial convergence; kernel would call LLM with reservations"
	default:
		return "insufficient convergence; kernel would escalate"
	}
}

// hasContradiction reports whether two effective positions in the same
// dimension assert different stances (an unresolved contradiction).
func hasContradiction(positions []deliberate.Position) bool {
	byDim := map[deliberate.Dimension]map[string]bool{}
	for _, p := range positions {
		if !p.Effective() {
			continue
		}
		if byDim[p.Dimension] == nil {
			byDim[p.Dimension] = map[string]bool{}
		}
		byDim[p.Dimension][strings.ToLower(strings.TrimSpace(p.Claim))] = true
	}
	for _, stances := range byDim {
		if len(stances) > 1 {
			return true
		}
	}
	return false
}

// ─── Clean Context (ADR-032 §3.4) ────────────────────────────────────────────
//
// BuildCleanContext produces the deterministic, structured block handed to the
// LLM when the Kernel is uncertain (EmitWithReservations / Escalate). It is
// NOT a dump of the whole context — only positions, evidence (top-N,
// truncated), hypotheses with scores, contradictions, and an explicit question
// to the specialist. Pure and testable without an LLM.
func BuildCleanContext(data PipelineData, trace DeliberationTrace, cfg DeliberateConfig) string {
	cfg = normalizeDeliberateConfig(cfg)

	var b strings.Builder
	b.WriteString("── DELIBERAÇÃO DO KERNEL (determinístico, zero-LLM) ──\n")
	fmt.Fprintf(&b, "Kernel verdict: %s (confiança %.2f)\n", trace.Verdict, trace.Confidence.Final)

	// POSITIONS
	b.WriteString("\nPOSITIONS (evidências da casa):\n")
	if len(trace.Positions) == 0 {
		b.WriteString("  (nenhuma posição substantiada — o Kernel não tem evidência suficiente)\n")
	} else {
		for _, p := range trace.Positions {
			ev := "sem evidência"
			if len(p.EvidenceIDs) > 0 {
				ev = strings.Join(p.EvidenceIDs, ",")
			}
			fmt.Fprintf(&b, "  [%s] %s: %q — evidência: %s\n", p.ID, p.Dimension, truncateString(p.Claim, cfg.MaxCharsPerEvidence), ev)
		}
	}

	// EVIDÊNCIAS (top-N, truncated)
	b.WriteString("\nEVIDÊNCIAS (traceáveis, top-N):\n")
	evidences := collectEvidenceEntries(data, trace, cfg)
	if len(evidences) == 0 {
		b.WriteString("  (nenhuma evidência traceável)\n")
	} else {
		for _, e := range evidences {
			fmt.Fprintf(&b, "  %s — %s — %q (score %.2f)\n", e.ID, e.Source, truncateString(e.Text, cfg.MaxCharsPerEvidence), e.Score)
		}
	}

	// HIPÓTESES (scores)
	b.WriteString("\nHIPÓTESES (scores):\n")
	hypotheses := computeHypotheses(trace.Positions)
	if len(hypotheses) == 0 {
		b.WriteString("  (nenhuma hipótese derivada)\n")
	} else {
		for _, h := range hypotheses {
			label := "baixa"
			if h.Score >= cfg.ReservationThreshold {
				label = "média"
			}
			if h.Score >= cfg.EmitThreshold {
				label = "alta"
			}
			fmt.Fprintf(&b, "  H%d: %q — convergência %.2f — %s\n", h.Index, truncateString(h.Claim, cfg.MaxCharsPerEvidence), h.Score, label)
		}
	}

	// CONTRADIÇÕES
	b.WriteString("\nCONTRADIÇÕES (não resolvidas):\n")
	contradictions := findContradictions(trace.Positions)
	if len(contradictions) == 0 {
		b.WriteString("  (nenhuma contradição detectada)\n")
	} else {
		for _, c := range contradictions {
			fmt.Fprintf(&b, "  %s vs %s: %q contradiz %q\n", c.A, c.B, truncateString(c.ClaimA, cfg.MaxCharsPerEvidence), truncateString(c.ClaimB, cfg.MaxCharsPerEvidence))
		}
	}

	// PERGUNTA AO ESPECIALISTA
	b.WriteString("\nPERGUNTA AO ESPECIALISTA:\n")
	fmt.Fprintf(&b, "  O Kernel não converge (confiança %.2f). ", trace.Confidence.Final)
	if len(contradictions) > 0 {
		b.WriteString("Resolva a contradição e recomende entre as hipóteses acima, citando as evidências.")
	} else {
		b.WriteString("Recomende entre as hipóteses acima, citando as evidências.")
	}
	b.WriteString("\n── FIM DA DELIBERAÇÃO ──\n")

	return b.String()
}

// evidenceEntry is one traceable evidence rendered in the clean context.
type evidenceEntry struct {
	ID     string
	Source string
	Text   string
	Score  float64
}

// collectEvidenceEntries gathers the top-N evidence (by score) referenced by
// the trace's positions, from the knowledge and memory results already in the
// pipeline data. Deterministic ordering: score desc, then ID asc.
func collectEvidenceEntries(data PipelineData, trace DeliberationTrace, cfg DeliberateConfig) []evidenceEntry {
	// Build an ID → score lookup from the positions' evidence IDs.
	want := map[string]bool{}
	for _, id := range trace.EvidenceIDs {
		want[id] = true
	}

	var entries []evidenceEntry
	if kr := data.KnowledgeResults; kr != nil {
		for i := range kr.Results {
			r := &kr.Results[i]
			if !want[r.ID] {
				continue
			}
			text := r.Snippet
			if text == "" {
				text = r.Content
			}
			entries = append(entries, evidenceEntry{
				ID:     r.ID,
				Source: "knowledge.db",
				Text:   text,
				Score:  r.Score,
			})
		}
	}
	for _, m := range data.MemoryResults {
		if !want[m.ID] {
			continue
		}
		entries = append(entries, evidenceEntry{
			ID:     m.ID,
			Source: "memory",
			Text:   m.Content,
			Score:  0.5, // memory records carry no score in the projection
		})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Score != entries[j].Score {
			return entries[i].Score > entries[j].Score
		}
		return entries[i].ID < entries[j].ID
	})
	if len(entries) > cfg.MaxEvidence {
		entries = entries[:cfg.MaxEvidence]
	}
	return entries
}

// hypothesis is one derived stance with its convergence score.
type hypothesis struct {
	Index int
	Claim string
	Score float64
}

// computeHypotheses derives the candidate stances (hypotheses) from the
// recommendation-dimension positions, scoring each by its share of the
// recommendation weight. Deterministic: sorted by score desc, then claim asc.
func computeHypotheses(positions []deliberate.Position) []hypothesis {
	// Group effective recommendation positions by stance.
	type acc struct {
		claim string
		count int
	}
	groups := map[string]*acc{}
	var order []string
	for _, p := range positions {
		if p.Dimension != deliberate.DimensionRecommendation || !p.Effective() {
			continue
		}
		s := strings.ToLower(strings.TrimSpace(p.Claim))
		if groups[s] == nil {
			groups[s] = &acc{claim: p.Claim}
			order = append(order, s)
		}
		groups[s].count++
	}

	var out []hypothesis
	for _, s := range order {
		g := groups[s]
		// Score = share of the recommendation weight held by this stance.
		score := float64(g.count) / float64(len(order))
		out = append(out, hypothesis{Claim: g.claim, Score: score})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Claim < out[j].Claim
	})
	for i := range out {
		out[i].Index = i + 1
	}
	return out
}

// contradiction is a pair of positions in the same dimension with conflicting
// stances.
type contradiction struct {
	A, B   string
	ClaimA string
	ClaimB string
}

// findContradictions detects pairs of effective positions in the same
// dimension asserting different stances. Deterministic ordering.
func findContradictions(positions []deliberate.Position) []contradiction {
	byDim := map[deliberate.Dimension][]deliberate.Position{}
	for _, p := range positions {
		if !p.Effective() {
			continue
		}
		byDim[p.Dimension] = append(byDim[p.Dimension], p)
	}

	var out []contradiction
	for _, ps := range byDim {
		for i := 0; i < len(ps); i++ {
			for j := i + 1; j < len(ps); j++ {
				a, b := ps[i], ps[j]
				if strings.ToLower(strings.TrimSpace(a.Claim)) == strings.ToLower(strings.TrimSpace(b.Claim)) {
					continue
				}
				out = append(out, contradiction{A: a.ID, B: b.ID, ClaimA: a.Claim, ClaimB: b.Claim})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].A != out[j].A {
			return out[i].A < out[j].A
		}
		return out[i].B < out[j].B
	})
	return out
}

// ─── Deterministic Response (ADR-032 §3.3) ───────────────────────────────────
//
// BuildDeterministicResponse assembles the Kernel's answer WITHOUT the LLM
// when the gate decides EmitOK. It reuses the format of the executor's
// deterministicResponse heuristic (knowledge-only), but is driven by the
// deliberation's positions/evidence.
func BuildDeterministicResponse(data PipelineData, trace DeliberationTrace) string {
	var b strings.Builder
	b.WriteString("Conhecimento da casa (resposta determinística do kernel):\n")

	// Prefer the recommendation positions (the "what to do" stance).
	shown := 0
	for _, p := range trace.Positions {
		if p.Dimension != deliberate.DimensionRecommendation {
			continue
		}
		if shown >= 5 {
			break
		}
		claim := p.Claim
		if claim == "" {
			continue
		}
		b.WriteString("• " + claim + "\n")
		shown++
	}

	// Fall back to any position if no recommendation stance exists.
	if shown == 0 {
		for _, p := range trace.Positions {
			if shown >= 5 {
				break
			}
			if p.Claim == "" {
				continue
			}
			b.WriteString("• " + p.Claim + "\n")
			shown++
		}
	}

	if shown == 0 {
		return ""
	}
	if len(trace.EvidenceIDs) > 0 {
		fmt.Fprintf(&b, "\nEvidências: %s\n", strings.Join(trace.EvidenceIDs, ", "))
	}
	return b.String()
}
