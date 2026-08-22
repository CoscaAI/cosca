package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ProblemClass classifies what a case measures:
//
//	reproduction — a known solution the agent is expected to reach (class A)
//	discovery    — a solution the author knows but hides from the agent (class B)
//	open         — a solution even the author does not know (class C)
//	existence    — determine whether a solution exists, or prove impossibility (D)
type ProblemClass string

const (
	ClassReproduction ProblemClass = "reproduction"
	ClassDiscovery    ProblemClass = "discovery"
	ClassOpen         ProblemClass = "open"
	ClassExistence    ProblemClass = "existence"
)

// Oracle configures the oracle-based evaluation mode of a case. When nil, the
// case runs in static mode (the current behaviour): verify commands run once at
// the end, no oracle is queryable during the search.
type Oracle struct {
	// Mode is static (default), interactive (agent queries the oracle), or
	// existence (agent must construct a solution OR prove impossibility).
	Mode string `yaml:"mode"`
	// Feedback is the granularity of the oracle's answer:
	// valid_invalid | pass_fail | property | constraint.
	Feedback string `yaml:"feedback"`
	// MaxSubmissions is the query budget (0 = unlimited).
	MaxSubmissions int `yaml:"max_submissions"`
	// PublicProperties are the properties the agent IS allowed to see (they do
	// not reveal the solution). Optional.
	PublicProperties []string `yaml:"public_properties"`
}

// Feedback constants for Oracle.Feedback.
const (
	FeedbackValidInvalid = "valid_invalid"
	FeedbackPassFail     = "pass_fail"
	FeedbackProperty     = "property"
	FeedbackConstraint   = "constraint"
)

// SecretCase holds the held-out evaluation material for a case: the secret
// verify commands and (optionally) the reference solution and acceptance
// criteria. It lives OUTSIDE the public suite, in
// .cosca/evals/secrets/<case-id>.yaml (0600, never committed, never embedded).
// Only the oracle reads it — the agent under test never sees it.
type SecretCase struct {
	Case               string   `yaml:"case"`
	ReferenceSolution  string   `yaml:"reference_solution"`
	SecretVerify       []string `yaml:"secret_verify"`
	AcceptanceCriteria []string `yaml:"acceptance_criteria"`
	// Feedback overrides the oracle's answer granularity (default valid_invalid).
	Feedback string `yaml:"feedback"`
	// VerifyProof is a command that validates an impossibility proof in
	// existence mode (e.g. a proof checker). Optional.
	VerifyProof string `yaml:"verify_proof"`
}

// secretsDirFor returns the secrets directory for a project root.
func secretsDirFor(dir string) string {
	return filepath.Join(dir, ".cosca", "evals", "secrets")
}

// LoadSecret loads the secret case for id from dir's secrets store. It returns
// a nil SecretCase (no error) when no secret exists, so a suite can mix secret
// and non-secret cases.
func LoadSecret(dir, id string) (*SecretCase, error) {
	path := filepath.Join(secretsDirFor(dir), id+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read secret %q: %w", id, err)
	}
	var sc SecretCase
	if err := yaml.Unmarshal(data, &sc); err != nil {
		return nil, fmt.Errorf("parse secret %q: %w", id, err)
	}
	if sc.Case == "" {
		sc.Case = id
	}
	return &sc, nil
}

// OracleVerdict is the minimal response the oracle returns to a submission. It
// deliberately omits how to fix the candidate: that would leak the secret.
type OracleVerdict struct {
	// Valid reports whether all secret verifies passed.
	Valid bool `json:"valid"`
	// Signal is the human-readable minimal answer (e.g. "VALID", "INVALID", or
	// "t1=PASS t2=FAIL" for pass_fail feedback).
	Signal string `json:"signal"`
	// PerCommand holds the anonymous per-command results for pass_fail (and
	// property/constraint) feedback. Empty for valid_invalid.
	PerCommand []VerifyResult `json:"per_command,omitempty"`
}

// SubmitToOracle runs the secret_verify commands against dir and returns the
// minimal signal dictated by feedback. It is the black-box verifier: the caller
// (the agent) learns only what feedback allows, never the commands themselves.
func SubmitToOracle(ctx context.Context, dir string, secret *SecretCase, feedback string, timeout time.Duration) (OracleVerdict, error) {
	if secret == nil || len(secret.SecretVerify) == 0 {
		return OracleVerdict{Valid: true, Signal: "VALID"}, nil
	}
	if feedback == "" {
		feedback = FeedbackValidInvalid
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	results := RunVerifyCommands(ctx, dir, secret.SecretVerify, timeout)
	allOK := true
	for _, vr := range results {
		if !vr.OK {
			allOK = false
			break
		}
	}

	verdict := OracleVerdict{Valid: allOK}
	switch feedback {
	case FeedbackValidInvalid:
		if allOK {
			verdict.Signal = "VALID"
		} else {
			verdict.Signal = "INVALID"
		}
	case FeedbackPassFail, FeedbackProperty, FeedbackConstraint:
		// Anonymous per-command PASS/FAIL. Property/constraint map here for now;
		// a future phase maps commands to named public_properties. The command
		// string and output tail are stripped so the secret never leaks.
		parts := make([]string, 0, len(results))
		for i, vr := range results {
			verdict.PerCommand = append(verdict.PerCommand, VerifyResult{OK: vr.OK})
			state := "PASS"
			if !vr.OK {
				state = "FAIL"
			}
			parts = append(parts, fmt.Sprintf("t%d=%s", i+1, state))
		}
		verdict.Signal = strings.Join(parts, " ")
	default:
		if allOK {
			verdict.Signal = "VALID"
		} else {
			verdict.Signal = "INVALID"
		}
	}
	return verdict, nil
}

// Submission represents one candidate the agent sent to the oracle. It is the
// reliable unit of "hypothesis": each submission is a candidate the agent
// believed might be valid.
type Submission struct {
	Index  int          `json:"index"`
	At     time.Time    `json:"at"`
	Valid  bool         `json:"valid"`
	Signal string       `json:"signal"`
	// Regressed is true when a previously-valid candidate became invalid (the
	// agent broke a working solution).
	Regressed bool `json:"regressed,omitempty"`
}

// DiscoveryMetrics captures the PROCESS of discovery, not just the outcome. It
// answers "how did the agent get there?" rather than "did it pass?".
type DiscoveryMetrics struct {
	TimeToFirstHypothesis string  `json:"time_to_first_hypothesis"`
	NumHypotheses         int     `json:"num_hypotheses"`
	NumDiscarded          int     `json:"num_discarded"`
	DiscardRate           float64 `json:"discard_rate"`
	TimeToSolution        string  `json:"time_to_solution"`
	Regressions           int     `json:"regressions"`
	HumanInterventions    int     `json:"human_interventions"`
	// DiscoveryEfficiency is the master metric: reward per unit of resource
	// consumed (hypotheses + experiments + 1). Two agents solving the same
	// problem differ by how efficiently they explore.
	DiscoveryEfficiency float64 `json:"discovery_efficiency"`
}

// SubmissionLog is an append-only record of the agent's oracle submissions for
// one case. It lives in .cosca/evals/submissions/<case-id>.jsonl and is the raw
// material from which DiscoveryMetrics is computed. Appending is atomic
// (single line, newline-delimited) so concurrent agent submissions never
// corrupt the file.
type SubmissionLog struct {
	dir    string
	caseID string
}

// NewSubmissionLog returns a log rooted at dir for the given case.
func NewSubmissionLog(dir, caseID string) *SubmissionLog {
	return &SubmissionLog{dir: dir, caseID: caseID}
}

func (l *SubmissionLog) path() string {
	return filepath.Join(l.dir, ".cosca", "evals", "submissions", l.caseID+".jsonl")
}

// Append records one submission. It returns the submission's 1-based index.
func (l *SubmissionLog) Append(s Submission) (int, error) {
	prev, _ := l.Load()
	s.Index = len(prev) + 1
	path := l.path()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return 0, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	data, _ := json.Marshal(s)
	if _, err := f.Write(append(data, '\n')); err != nil {
		return 0, err
	}
	return s.Index, nil
}

// Load returns all submissions for the case in order. Missing/empty log yields
// nil, nil.
func (l *SubmissionLog) Load() ([]Submission, error) {
	data, err := os.ReadFile(l.path())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var subs []Submission
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var s Submission
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			return nil, fmt.Errorf("parse submission: %w", err)
		}
		subs = append(subs, s)
	}
	return subs, nil
}

// ComputeDiscoveryMetrics derives DiscoveryMetrics from the submissions,
// experiment count and reward. It implements the "quality of the process"
// measurement: discovery_efficiency = reward / (hypotheses + experiments + 1).
func ComputeDiscoveryMetrics(subs []Submission, experiments int, reward float64, start time.Time) DiscoveryMetrics {
	m := DiscoveryMetrics{HumanInterventions: 0}
	if len(subs) == 0 {
		return m
	}

	m.NumHypotheses = len(subs)
	first := subs[0].At
	m.TimeToFirstHypothesis = first.Sub(start).Round(time.Millisecond).String()

	validSeen := false
	for _, s := range subs {
		if !s.Valid {
			m.NumDiscarded++
		}
		if s.Regressed {
			m.Regressions++
		}
		if !validSeen && s.Valid {
			validSeen = true
			m.TimeToSolution = s.At.Sub(start).Round(time.Millisecond).String()
		}
	}

	if m.NumHypotheses > 0 {
		m.DiscardRate = float64(m.NumDiscarded) / float64(m.NumHypotheses)
	}
	m.DiscoveryEfficiency = reward / float64(m.NumHypotheses+experiments+1)
	return m
}
