// Package execpolicy implements a declarative command execution policy engine
// adapted from OpenAI's Codex CLI execpolicy (Apache-2.0).
//
// A policy is a set of ordered prefix rules. Each rule carries a token pattern,
// a decision (allow / prompt / forbidden), and optional example invocations
// (match / not_match) that are validated when the policy is loaded.
//
// Policies are loaded from YAML and are purely additive: with no policy
// configured, command execution keeps its existing behavior.
package execpolicy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// Decision is the action applied to a command that matches a rule.
type Decision string

const (
	// Allow permits the command to run.
	Allow Decision = "allow"
	// Prompt flags the command for human approval.
	Prompt Decision = "prompt"
	// Forbidden blocks the command outright.
	Forbidden Decision = "forbidden"
)

// DecisionOrDefault returns the rule's decision, defaulting to Allow when
// unset (the Codex default).
func (r *Rule) DecisionOrDefault() Decision {
	if r.Decision == "" {
		return Allow
	}
	return r.Decision
}

// UnmarshalYAML normalizes a decision token to lowercase so policy authors may
// write "ALLOW", "Allow", or "allow".
func (d *Decision) UnmarshalYAML(value *yaml.Node) error {
	*d = Decision(strings.ToLower(strings.TrimSpace(value.Value)))
	return nil
}

// Pattern is a sequence of token positions. Each element holds the set of
// alternative tokens accepted at that position; a single alternative is the
// common case. In YAML both forms are accepted:
//
//	pattern: ["git", "reset", "--hard"]
//	pattern: [["git"], ["reset"], ["--hard"]]
//
// A list element (e.g. [["rm", "del"], "-rf"]) expresses alternatives at that
// position, matching "rm -rf" or "del -rf".
type Pattern [][]string

// UnmarshalYAML accepts either a flat list of string tokens or a nested list
// where each inner list is the alternative set for that position.
func (p *Pattern) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.SequenceNode {
		return fmt.Errorf("execpolicy: pattern must be a list, got %s", kindName(value.Kind))
	}
	out := make(Pattern, 0, len(value.Content))
	for _, item := range value.Content {
		switch item.Kind {
		case yaml.ScalarNode:
			out = append(out, []string{item.Value})
		case yaml.SequenceNode:
			alts := make([]string, 0, len(item.Content))
			for _, alt := range item.Content {
				if alt.Kind != yaml.ScalarNode {
					return fmt.Errorf("execpolicy: pattern alternative must be a string, got %s", kindName(alt.Kind))
				}
				alts = append(alts, alt.Value)
			}
			if len(alts) == 0 {
				return fmt.Errorf("execpolicy: pattern alternative list must not be empty")
			}
			out = append(out, alts)
		default:
			return fmt.Errorf("execpolicy: pattern element must be a string or a list of strings, got %s", kindName(item.Kind))
		}
	}
	*p = out
	return nil
}

// Rule is a single policy rule. Rules are evaluated in order and the first
// matching rule wins.
type Rule struct {
	// Pattern is the ordered token pattern. An element that is a list of
	// strings means alternatives at that position.
	Pattern Pattern `yaml:"pattern"`

	// Decision is allow, prompt, or forbidden. Defaults to allow.
	Decision Decision `yaml:"decision"`

	// Justification is a human-readable reason attached to the decision.
	Justification string `yaml:"justification"`

	// Match holds example invocations that MUST match this rule; validated
	// at load time.
	Match [][]string `yaml:"match"`

	// NotMatch holds example invocations that MUST NOT match this rule;
	// validated at load time.
	NotMatch [][]string `yaml:"not_match"`
}

// Policy is an execution policy: an ordered list of rules plus host-executable
// resolution behavior.
type Policy struct {
	// Version is the policy schema version (informational).
	Version string

	// ResolveHostExecutables enables basename fallback: a full path such as
	// /usr/bin/git matches a rule whose first token is "git" when no rule
	// matched the exact full path.
	ResolveHostExecutables bool

	// Rules are evaluated in order; the first matching rule wins.
	Rules []Rule
}

// yamlPolicy is the on-disk schema for a policy file.
type yamlPolicy struct {
	Version                string `yaml:"version"`
	ResolveHostExecutables bool   `yaml:"resolve_host_executables"`
	Rules                  []Rule `yaml:"rules"`
}

// Load parses and validates a YAML policy file.
func Load(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("execpolicy: read policy file %s: %w", path, err)
	}
	p, err := LoadYAML(data)
	if err != nil {
		return nil, fmt.Errorf("execpolicy: load %s: %w", path, err)
	}
	return p, nil
}

// LoadYAML parses and validates a YAML policy document.
func LoadYAML(data []byte) (*Policy, error) {
	var yp yamlPolicy
	if err := yaml.Unmarshal(data, &yp); err != nil {
		return nil, fmt.Errorf("execpolicy: parse policy YAML: %w", err)
	}
	p := &Policy{
		Version:                yp.Version,
		ResolveHostExecutables: yp.ResolveHostExecutables,
		Rules:                  yp.Rules,
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return p, nil
}

// Evaluate checks a tokenized command against the policy rules. It returns the
// decision of the first matching rule together with its justification, or Allow
// with an empty justification when no rule matches.
//
// When ResolveHostExecutables is enabled and the first token is an absolute
// path, a second pass falls back to its basename (e.g. /usr/bin/git -> git) if
// no rule matched the exact invocation.
func (p *Policy) Evaluate(argv []string) (Decision, string) {
	if p == nil || len(p.Rules) == 0 || len(argv) == 0 {
		return Allow, ""
	}
	if d, j, ok := p.matchRules(argv); ok {
		return d, j
	}
	if p.ResolveHostExecutables && filepath.IsAbs(argv[0]) {
		alt := append([]string(nil), argv...)
		alt[0] = filepath.Base(argv[0])
		if d, j, ok := p.matchRules(alt); ok {
			return d, j
		}
	}
	return Allow, ""
}

// matchRules returns the decision and justification of the first rule whose
// pattern matches argv.
func (p *Policy) matchRules(argv []string) (Decision, string, bool) {
	for i := range p.Rules {
		r := &p.Rules[i]
		if r.matches(argv) {
			return r.DecisionOrDefault(), r.Justification, true
		}
	}
	return Allow, "", false
}

// matches reports whether the rule's pattern matches argv as a prefix: every
// pattern position must be present in argv, and each argv token must be one of
// the alternatives at that position. Longer argv (extra trailing tokens) still
// matches, so a forbidden rule on "git reset --hard" also blocks
// "git reset --hard --force".
func (r *Rule) matches(argv []string) bool {
	if len(r.Pattern) == 0 || len(argv) < len(r.Pattern) {
		return false
	}
	for i, alts := range r.Pattern {
		if len(alts) == 0 || !containsToken(alts, argv[i]) {
			return false
		}
	}
	return true
}

// Validate checks that every rule has a non-empty pattern, a valid decision,
// and that every Match example matches the rule while every NotMatch example
// does not. It returns an error listing all violations. Load and LoadYAML call
// this before returning a policy.
func (p *Policy) Validate() error {
	if p == nil {
		return errors.New("execpolicy: nil policy")
	}
	var errs []string
	for i := range p.Rules {
		r := &p.Rules[i]
		if len(r.Pattern) == 0 {
			errs = append(errs, fmt.Sprintf("rule %d: pattern must not be empty", i))
			continue
		}
		switch r.DecisionOrDefault() {
		case Allow, Prompt, Forbidden:
		default:
			errs = append(errs, fmt.Sprintf("rule %d: invalid decision %q (want allow, prompt, or forbidden)", i, string(r.Decision)))
		}
		for j, ex := range r.Match {
			if !p.ruleMatchesExample(r, ex) {
				errs = append(errs, fmt.Sprintf("rule %d: match example %d %v does not match the rule pattern", i, j, ex))
			}
		}
		for j, ex := range r.NotMatch {
			if p.ruleMatchesExample(r, ex) {
				errs = append(errs, fmt.Sprintf("rule %d: not_match example %d %v unexpectedly matches the rule pattern", i, j, ex))
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("execpolicy validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// ruleMatchesExample reports whether an example invocation matches the rule,
// applying the same basename fallback used by Evaluate.
func (p *Policy) ruleMatchesExample(r *Rule, argv []string) bool {
	if r.matches(argv) {
		return true
	}
	if p != nil && p.ResolveHostExecutables && len(argv) > 0 && filepath.IsAbs(argv[0]) {
		alt := append([]string(nil), argv...)
		alt[0] = filepath.Base(argv[0])
		return r.matches(alt)
	}
	return false
}

// Tokenize splits a shell command string into tokens using shell-like rules:
// whitespace separation, single/double quote handling, and backslash escapes.
// Shell operators (|, &&, ;, ...) are treated as ordinary tokens; the pipeline
// is intentionally not interpreted.
func Tokenize(command string) []string {
	var tokens []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			tokens = append(tokens, string(cur))
			cur = cur[:0]
		}
	}
	runes := []rune(command)
	i := 0
	for i < len(runes) {
		c := runes[i]
		switch {
		case unicode.IsSpace(c):
			flush()
			i++
		case c == '\'':
			i++
			for i < len(runes) && runes[i] != '\'' {
				cur = append(cur, runes[i])
				i++
			}
			i++ // skip closing quote
		case c == '"':
			i++
			for i < len(runes) && runes[i] != '"' {
				if runes[i] == '\\' && i+1 < len(runes) {
					i++
					cur = append(cur, runes[i])
					i++
					continue
				}
				cur = append(cur, runes[i])
				i++
			}
			i++ // skip closing quote
		case c == '\\':
			if i+1 < len(runes) {
				cur = append(cur, runes[i+1])
				i += 2
			} else {
				i++
			}
		default:
			cur = append(cur, c)
			i++
		}
	}
	flush()
	return tokens
}

func containsToken(alts []string, tok string) bool {
	for _, a := range alts {
		if a == tok {
			return true
		}
	}
	return false
}

func kindName(k yaml.Kind) string {
	switch k {
	case yaml.DocumentNode:
		return "document"
	case yaml.SequenceNode:
		return "list"
	case yaml.MappingNode:
		return "mapping"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.AliasNode:
		return "alias"
	default:
		return "unknown"
	}
}
