package skilleval

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

// DefaultNeutralScore is the score used when a judge verdict cannot be parsed.
// A neutral default means a single malformed judge response can never crash or
// bias the eval loop — it simply reads as "no signal".
const DefaultNeutralScore = 0.5

// ErrScorerNotWired indicates a scorer whose provider backend is not yet
// connected. See LLMJudgeScorer.
var ErrScorerNotWired = errors.New("skilleval: scorer provider not wired")

// errNoJudgeJSON is the sentinel returned by parseJudgeJSON when no parseable
// JSON could be recovered from an LLM judge response.
var errNoJudgeJSON = errors.New("skilleval: no parseable judge JSON")

// Scorer grades an output against a rubric and reports the resulting
// GradingResult. Implementations must never abort the eval loop on a transient
// or malformed judge output; they should degrade to a neutral result.
type Scorer interface {
	Grade(ctx context.Context, rubric []string, output string) (GradingResult, error)
}

// StaticScorer is a deterministic scorer used for tests and fixtures. It
// grades an output by checking whether each rubric condition string appears in
// the output, ignoring case. Passed counts the conditions present and Score is
// the clamped pass-rate; it never returns an error.
type StaticScorer struct{}

// Grade grades output against rubric by substring matching (case-insensitive).
func (StaticScorer) Grade(_ context.Context, rubric []string, output string) (GradingResult, error) {
	res := GradingResult{Total: len(rubric)}
	if res.Total > 0 {
		res.Feedback = make([]string, 0, res.Total)
	}

	lower := strings.ToLower(output)
	for _, cond := range rubric {
		if strings.Contains(lower, strings.ToLower(cond)) {
			res.Passed++
			res.Feedback = append(res.Feedback, "pass")
			continue
		}
		res.Feedback = append(res.Feedback, "fail")
	}

	res.Score = PassRate(res.Passed, res.Total)
	return res, nil
}

// PassRate returns passed/total, clamped to [0,1]. A non-positive total yields 0.
func PassRate(passed, total int) float64 {
	if total <= 0 {
		return 0
	}
	return clampScore(float64(passed) / float64(total))
}

// LLMJudgeScorer grades an output against a rubric using a judge model from
// the Cosca Model Registry (internal/modelreg). The judge's JSON verdict is
// parsed defensively (see parseJudgeScore).
//
// FATIA 1 (this increment): only the contract and the defensive parsing are
// established here. The model-registry provider is not wired yet, so Grade
// returns ErrScorerNotWired. Wiring the provider — invoking the judge and
// threading the raw verdict through parseJudgeScore — lands in a later
// increment.
type LLMJudgeScorer struct{}

// Grade returns ErrScorerNotWired because the LLM provider is not yet
// connected. It is a placeholder that locks the Scorer contract; it does not
// invoke any model.
func (LLMJudgeScorer) Grade(context.Context, []string, string) (GradingResult, error) {
	return GradingResult{}, ErrScorerNotWired
}

// judgeResponse is the loose JSON shape expected from an LLM judge. The
// "score" field is a pointer so an explicit score of 0 (all conditions failed)
// is distinguishable from an absent score.
type judgeResponse struct {
	Score    *float64 `json:"score"`
	Passed   int      `json:"passed"`
	Total    int      `json:"total"`
	Feedback []string `json:"feedback"`
}

// clampScore clamps v into [0,1]. Values above 1 become 1 and values below 0
// become 0.
func clampScore(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// extractBalancedJSON scans raw for the first balanced JSON object or array,
// respecting quotes so braces/backets inside string literals (e.g. "{edge}")
// are not counted. It returns the extracted object (including its outer
// delimiter) or "" when no balanced object is found. This lets a judge embed
// JSON inside natural-language prose.
func extractBalancedJSON(raw string) string {
	if raw == "" {
		return ""
	}

	startO := strings.IndexByte(raw, '{')
	startA := strings.IndexByte(raw, '[')
	var start int
	switch {
	case startO < 0 && startA < 0:
		return ""
	case startO < 0:
		start = startA
	case startA < 0:
		start = startO
	case startA < startO:
		start = startA
	default:
		start = startO
	}

	opener := raw[start]
	closer := byte('}')
	if opener == '[' {
		closer = ']'
	}

	depth := 0
	inStr := false
	escaped := false
	for i := start; i < len(raw); i++ {
		c := raw[i]
		if inStr {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case opener:
			depth++
		case closer:
			depth--
			if depth == 0 {
				return raw[start : i+1]
			}
		}
	}
	return ""
}

// parseJudgeJSON attempts to parse a judge verdict from raw. It first tries a
// direct JSON unmarshal of the whole response and, on failure, falls back to
// extracting a balanced JSON object/array via brace-counting (tolerating
// leading/trailing prose). It returns errNoJudgeJSON only when nothing
// parseable is found — never for malformed-but-recoverable content.
func parseJudgeJSON(raw string) (judgeResponse, error) {
	var jr judgeResponse
	if raw == "" {
		return jr, errNoJudgeJSON
	}
	if err := json.Unmarshal([]byte(raw), &jr); err == nil {
		return jr, nil
	}
	if obj := extractBalancedJSON(raw); obj != "" {
		if err := json.Unmarshal([]byte(obj), &jr); err == nil {
			return jr, nil
		}
	}
	return jr, errNoJudgeJSON
}

// resolveJudgeScore derives the final [0,1] score from a judge response. An
// explicit "score" field (including an explicit 0) takes precedence; otherwise
// the pass-rate derived from "passed"/"total" is used. When neither is
// present it falls back to the neutral default.
func resolveJudgeScore(jr judgeResponse) float64 {
	if jr.Score != nil {
		return clampScore(*jr.Score)
	}
	if jr.Total > 0 {
		return clampScore(float64(jr.Passed) / float64(jr.Total))
	}
	return DefaultNeutralScore
}

// parseJudgeScore extracts a clamped score in [0,1] from a judge's raw
// response. It is deliberately defensive: valid JSON is handled directly,
// embedded objects are recovered via brace-counting, and any parse failure
// falls back to the neutral default. It never returns an error — a malformed
// judge verdict must never abort the eval loop.
func parseJudgeScore(raw string) float64 {
	jr, err := parseJudgeJSON(raw)
	if err != nil {
		return DefaultNeutralScore
	}
	return resolveJudgeScore(jr)
}
