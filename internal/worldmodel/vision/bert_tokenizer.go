package vision

import (
	_ "embed"
	"strings"
	"unicode"
)

// ──────────────────────────────────────────────────────────────
// Minimal WordPiece BERT tokenizer (pure Go — no Python/tokenizers)
// ──────────────────────────────────────────────────────────────
//
// GroundingDINO uses a BERT-base text encoder (beit/bert-base-uncased) to turn
// the text-prompt into token embeddings. The exported ONNX graph requires
// `input_ids`, `token_type_ids` and `attention_mask` (plus the image
// `pixel_values` and the `pixel_mask`). This file implements a faithful-enough
// WordPiece tokenizer so COSCA can build those tensors entirely in Go — no
// huggingface `tokenizers` library, no subprocess.

//go:embed bert_vocab.txt
var bertVocabData string

// bertTokenizer wraps the embedded vocab and the special-token id map.
type bertTokenizer struct {
	vocab map[string]int
	ids   []string

	clsID int
	sepID int
	padID int
	unkID int
	maskID int
}

// Default BERT special-token ids (bert-base-uncased):
//  [PAD]=0 [UNK]=100 [CLS]=101 [SEP]=102 [MASK]=103
const (
	bertCLS   = 101
	bertSEP   = 102
	bertPAD   = 0
	bertUNK   = 100
	bertMASK  = 103
	bertMaxCL = 256 // exported graph pads text to 256 tokens
)

var globalBERT *bertTokenizer

// getBERT returns the lazily-built, process-wide BERT tokenizer.
func getBERT() *bertTokenizer {
	if globalBERT != nil {
		return globalBERT
	}
	t := &bertTokenizer{vocab: make(map[string]int, 30522)}
	lines := strings.Split(bertVocabData, "\n")
	for id, line := range lines {
		tok := strings.TrimRight(line, "\r")
		if tok == "" {
			continue
		}
		t.vocab[tok] = id
	}
	// Rebuild id→token slice of the exact vocab length.
	maxID := 0
	for _, id := range t.vocab {
		if id > maxID {
			maxID = id
		}
	}
	t.ids = make([]string, maxID+1)
	t.clsID = bertCLS
	t.sepID = bertSEP
	t.padID = bertPAD
	t.unkID = bertUNK
	t.maskID = bertMASK
	// Resolve special-token ids from the vocab when present, else default.
	if v, ok := t.vocab["[CLS]"]; ok {
		t.clsID = v
	}
	if v, ok := t.vocab["[SEP]"]; ok {
		t.sepID = v
	}
	if v, ok := t.vocab["[PAD]"]; ok {
		t.padID = v
	}
	if v, ok := t.vocab["[UNK]"]; ok {
		t.unkID = v
	}
	if v, ok := t.vocab["[MASK]"]; ok {
		t.maskID = v
	}
	globalBERT = t
	return t
}

// bertToken is a subword token together with its owning phrase index, and the
// original character span in the cleaned prompt (used to map tokens back to
// phrases for label decoding).
type bertToken struct {
	token  string
	phrase int // index into the prompt phrase list; -1 for separators/special
	start  int
	end    int
}

// cleanText lowercases, strips accents (NFD→drop combining marks) and collapses
// whitespace, matching BERT's pre-tokenizer for the uncased model.
func cleanText(s string) string {
	s = strings.ToLower(s)
	// Slight accent stripping: normalize to NFD and drop combining marks (\u0300-\u036f).
	s = stripAccents(s)
	// Collapse whitespace runs to a single space.
	s = strings.Join(strings.Fields(s), " ")
	return s
}

func stripAccents(s string) string {
	var b strings.Builder
	for _, r := range s {
		// Simple explicit mapping for common accented latin chars used by the
		// uncased pipeline; a full NFD pass is overkill here.
		switch r {
		case 'á', 'à', 'â', 'ä', 'ã', 'å':
			b.WriteRune('a')
		case 'é', 'è', 'ê', 'ë':
			b.WriteRune('e')
		case 'í', 'ì', 'î', 'ï':
			b.WriteRune('i')
		case 'ó', 'ò', 'ô', 'ö', 'õ':
			b.WriteRune('o')
		case 'ú', 'ù', 'û', 'ü':
			b.WriteRune('u')
		case 'ç':
			b.WriteRune('c')
		case 'ñ':
			b.WriteRune('n')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// isBERTPunc reports whether a rune is treated as a punctuation boundary by
// BERT's basic tokenizer (it splits these off as their own tokens).
func isBERTPunc(r rune) bool {
	switch r {
	case '!', '"', '#', '$', '%', '&', '\'', '(', ')', '*', '+', ',', '-',
		'.', '/', ':', ';', '<', '=', '>', '?', '@', '[', '\\', ']', '^', '_',
		'`', '{', '|', '}', '~':
		return true
	}
	// Covers the remaining Unicode punctuation categories.
	return unicode.In(r, unicode.Pc, unicode.Pd, unicode.Pe, unicode.Pf,
		unicode.Pi, unicode.Po, unicode.Ps)
}

// basicTokenize splits text into BERT basic tokens (words + punctuation), with
// the char span each token occupies in `text` (necessary to map subwords back
// to their source phrase).
func basicTokenize(text string) []bertToken {
	var toks []bertToken
	runes := []rune(text)
	i := 0
	for i < len(runes) {
		r := runes[i]
		if unicode.IsSpace(r) {
			i++
			continue
		}
		if isBERTPunc(r) {
			toks = append(toks, bertToken{token: string(r), start: i, end: i + 1})
			i++
			continue
		}
		j := i
		for j < len(runes) && !unicode.IsSpace(runes[j]) && !isBERTPunc(runes[j]) {
			j++
		}
		toks = append(toks, bertToken{token: string(runes[i:j]), start: i, end: j})
		i = j
	}
	return toks
}

// wordPiece greedily splits a basic token into WordPiece subwords ("##" for
// continuations). The returned pieces inherit the basic token's span.
func (t *bertTokenizer) wordPiece(basic bertToken) []bertToken {
	word := basic.token
	if word == "" {
		return nil
	}
	if _, ok := t.vocab[word]; ok {
		basic.token = word
		return []bertToken{basic}
	}
	runes := []rune(word)
	var pieces []bertToken
	start := 0
	for start < len(runes) {
		end := len(runes)
		found := false
		var cur string
		for end > start {
			sub := string(runes[start:end])
			prefix := ""
			if start > 0 {
				prefix = "##"
			}
			cur = prefix + sub
			if _, ok := t.vocab[cur]; ok {
				found = true
				break
			}
			end--
		}
		if !found {
			// Whole word unknown → emit [UNK]. We could per-piece it, but a
			// single [UNK] is what the reference tokenizer does for a fully
			// unknown word (per-piece fallback only for partial matches).
			return []bertToken{{token: "[UNK]", phrase: basic.phrase, start: basic.start, end: basic.end}}
		}
		pieces = append(pieces, bertToken{token: cur, phrase: basic.phrase, start: basic.start, end: basic.end})
		start = end
	}
	return pieces
}

// tokenizePhrase turns a single prompt phrase into a sequence of subword
// bertTokens, all tagged with the given phrase index.
func (t *bertTokenizer) tokenizePhrase(phrase string, phraseIdx int) []bertToken {
	clean := cleanText(phrase)
	basics := basicTokenize(clean)
	var out []bertToken
	for _, b := range basics {
		b.phrase = phraseIdx
		out = append(out, t.wordPiece(b)...)
	}
	return out
}

// encodePrompt builds the GroundingDINO text inputs from a list of phrases
// (classes). It returns input_ids, token_type_ids, attention_mask and a
// parallel tokenToPhrase array (index → phrase; -1 for [CLS]/[SEP]/[PAD]).
//
// The sequence is `[CLS] <phrase tokens, each phrase followed by a '.' (period)
// delimiter> [SEP]`, padded with [PAD] up to bertMaxCL tokens (the exported
// graph's text length).
//
// IMPORTANT: GroundingDINO's exported graph has a `NonZero`+`Gather` path
// (nodes /model/NonZero → /model/Transpose → /model/Gather_11) that requires
// AT LEAST THREE "special" text positions — [CLS](101), a '.'(1012) or
// '?'(1029) delimiter, and [SEP](102). A prompt with no '.'/'?' yields only two
// marked positions, so Gather_11 indexes out of bounds (idx=2 over a size-2
// tensor). This is why the reference processor says the text "needs to end with
// a dot"; we therefore always emit a '.' delimiter after every phrase.
func (t *bertTokenizer) encodePrompt(phrases []string) (inputIDs, tokenTypeIDs, attentionMask []int64, tokenToPhrase []int32) {
	ids := []int64{int64(t.clsID)}
	types := []int64{0}
	mask := []int64{1}
	phraseOf := []int32{-1}

	periodID, ok := t.vocab["."]
	if !ok {
		periodID = 1012 // canonical bert-base-uncased '.' token id
	}

	for i, phrase := range phrases {
		for _, tok := range t.tokenizePhrase(phrase, i) {
			id, ok := t.vocab[tok.token]
			if !ok {
				id = t.unkID
			}
			ids = append(ids, int64(id))
			types = append(types, 0)
			mask = append(mask, 1)
			phraseOf = append(phraseOf, int32(i))
		}
		// '.' delimiter after each phrase (phrase-agnostic; satisfies the
		// NonZero/Gather path and separates phrases).
		ids = append(ids, int64(periodID))
		types = append(types, 0)
		mask = append(mask, 1)
		phraseOf = append(phraseOf, -1)
	}

	ids = append(ids, int64(t.sepID))
	types = append(types, 0)
	mask = append(mask, 1)
	phraseOf = append(phraseOf, -1)

	// Truncate to the graph's text budget (keep [CLS] always).
	if len(ids) > bertMaxCL {
		ids = ids[:bertMaxCL]
		types = types[:bertMaxCL]
		mask = mask[:bertMaxCL]
		phraseOf = phraseOf[:bertMaxCL]
	}

	// Pad up to bertMaxCL so the tensors match the exported text length.
	for len(ids) < bertMaxCL {
		ids = append(ids, int64(t.padID))
		types = append(types, 0)
		mask = append(mask, 0)
		phraseOf = append(phraseOf, -1)
	}

	return ids, types, mask, phraseOf
}
