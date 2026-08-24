package modlink

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// canonicalTokens normalizes a string into a sequence of canonical word tokens
// for precise, deterministic trigger matching:
//
//  1. lowercase;
//  2. Unicode NFD-normalize and strip diacritical marks (á→a, ç→c, ü→u …);
//  3. replace every non-letter, non-digit rune with a space (punctuation,
//     symbols, separators collapse);
//  4. split on whitespace.
//
// The result is a whole-word, case- and accent-insensitive token stream. Two
// forms of the same trigger ("árvore", "Árvore", "arvore", "ár-vore→arvore")
// canonicalize identically, which raises recall within a domain without ever
// sacrificing precision (see containsAll).
func canonicalTokens(s string) []string {
	s = strings.ToLower(s)
	s = norm.NFD.String(s)

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.Is(unicode.Mn, r) {
			continue // drop combining diacritical marks
		}
		b.WriteRune(r)
	}
	s = b.String()

	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return ' '
	}, s)

	return strings.Fields(s)
}

// containsAll reports whether every trigger token appears as a whole word in
// the query token set. It is the precise, no-false-positive match: all trigger
// words must be present. A single common word can never satisfy a multi-word
// trigger, and no substring/semantic/prefix comparison is performed — this is
// what keeps domains isolated and makes the resolver deterministic.
func containsAll(queryTokens, triggerTokens []string) bool {
	if len(triggerTokens) == 0 {
		return false
	}
	if len(queryTokens) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(queryTokens))
	for _, t := range queryTokens {
		set[t] = struct{}{}
	}
	for _, t := range triggerTokens {
		if _, ok := set[t]; !ok {
			return false
		}
	}
	return true
}

// dedup removes duplicate tokens while preserving first-seen order. Used at
// construction to keep trigger token sets canonical and host the match fast.
func dedup(tokens []string) []string {
	if len(tokens) < 2 {
		return tokens
	}
	seen := make(map[string]struct{}, len(tokens))
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}
