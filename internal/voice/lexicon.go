package voice

// ── Léxico: camada de CONHECIMENTO, não o algoritmo ─────────────────────
// (Diretriz do Professor) O léxico guarda pronúncias verificadas do
// vocabulário da casa. Quando bate, a evidência é LEXICON. O G2P regra
// NUNCA é substituído pelo léxico silenciosamente — a origem é sempre
// declarada. Ex.: "cosca" → k 'o S k a (LEXICON).

import "strings"

// lexEntry: pronúncia verificada em SAMPA (sem o apóstrofo como token;
// a tônica é marcada com Stress=true no fonema).
type lexEntry struct {
	fonemas []Phoneme
}

// lexico: o vocabulário da família + irregularidades comuns do PB.
// Formato: SAMPA com "'" antes da vogal tônica (convenção da casa).
var lexico = map[string]lexEntry{
	"você":         {tokens("v o 's e")},
	"vocês":        {tokens("v o 's e S")},
	"também":       {tokens("t a~ 'b e~")},
	"aqui":         {tokens("a 'k i")},
	"ali":          {tokens("a 'l i")},
	"está":         {tokens("e S 't a")},
	"estão":        {tokens("e S 't a~ w")},
	"não":          {tokens("n 'a~ w")},
	"muito":        {tokens("m 'u~ j t u")},
	"senhor":       {tokens("s e~ 'J o R")},
	"senhora":      {tokens("s e~ 'J o R a")},
	"chefe":        {tokens("S 'e f i")},
	"chef":         {tokens("S 'e f")},
	"cosca":        {tokens("k 'o S k a")},
	"kernel":       {tokens("k 'e R n e w")},
	"protocolo":    {tokens("p r o t o 'k o l u")},
	"conhecimento": {tokens("k o J e s i 'm e~ t u")},
	"família":      {tokens("f a 'm i l i a")},
	"protege":      {tokens("p r o 't E Z i")},
	"acima":        {tokens("a 's i m a")},
	"tudo":         {tokens("t 'u d u")},
	"obrigado":     {tokens("o b r i 'g a d u")},
	"obrigada":     {tokens("o b r i 'g a d a")},
	"guarda":       {tokens("g 'w a R d a")},
	"carro":        {tokens("k 'a R u")},
	"casa":         {tokens("k 'a z a")},
	"ordem":        {tokens("O 'R d e~")},
	"estamos":      {tokens("e S 't a m u S")},
	"sabedoria":    {tokens("s a b e d o 'r i a")},
	"lealdade":     {tokens("l e a w 'd a d i")},
	"silêncio":     {tokens("s i 'l e~ s i u")},
	"respeito":     {tokens("R e S 'p e j t u")},
	"justiça":      {tokens("Z u S 't i s a")},
	"força":        {tokens("f 'o R s a")},
	"coragem":      {tokens("k o 'r a Z e~")},
	"verdade":      {tokens("v e R 'd a d i")},
	"confiança":    {tokens("k o~ f i 'a~ s a")},
	"amigo":        {tokens("a 'm i g u")},
	"capo":         {tokens("k 'a p u")},
	"capos":        {tokens("k 'a p u S")},
	"consigliere":  {tokens("k o~ s i 'l i E R i")},
	"don":          {tokens("d 'o~")},
	"voz":          {tokens("v 'O S")},
	// monossílabos átonos (pronúncia reduzida do PB: e→i, o→u)
	"o": {tokens("u")}, "a": {tokens("a")}, "os": {tokens("u S")},
	"as": {tokens("a S")}, "e": {tokens("i")}, "de": {tokens("d i")},
	"em": {tokens("e~")}, "com": {tokens("k o~")}, "sem": {tokens("s e~")},
	"por": {tokens("p u R")}, "que": {tokens("k i")}, "se": {tokens("s i")},
	"um": {tokens("u~")}, "uns": {tokens("u~ S")}, "no": {tokens("n u")},
	"na": {tokens("n a")}, "nos": {tokens("n u S")}, "nas": {tokens("n a S")},
	"do": {tokens("d u")}, "da": {tokens("d a")}, "dos": {tokens("d u S")},
	"das": {tokens("d a S")}, "ao": {tokens("a~ w")}, "aos": {tokens("a~ w S")},
	"me": {tokens("m i")}, "te": {tokens("t i")}, "lhe": {tokens("L i")},
	"lhes": {tokens("L i S")}, "vos": {tokens("v u S")}, "à": {tokens("a")},
	"às": {tokens("a S")},
}

// tokens: converte "k 'o S k a" → []Phoneme com Stress e Nasal marcados.
// Stress só é marcado em VOGAL (correção: 'l em "silêncio" não pode
// carregar a tônica — a vogal mais próxima à frente assume).
func tokens(sampa string) []Phoneme {
	raw := strings.Fields(sampa)
	out := make([]Phoneme, len(raw))
	stressIdx := -1
	for i, tok := range raw {
		stress := strings.HasPrefix(tok, "'")
		if stress {
			tok = tok[1:]
		}
		nasal := strings.HasSuffix(tok, "~")
		sym := strings.TrimSuffix(tok, "~")
		out[i] = Phoneme{
			Symbol: sym,
			Stress: stress,
			Nasal:  nasal,
			Dur:    durClass(sym, nasal, stress),
			Evid:   EvidenceLexicon,
		}
		if stress {
			stressIdx = i
		}
	}
	// se a marca caiu em consoante, move para a vogal mais próxima
	if stressIdx >= 0 && !out[stressIdx].IsVowel() {
		for j := stressIdx + 1; j < len(out); j++ {
			if out[j].IsVowel() {
				out[stressIdx].Stress = false
				out[j].Stress = true
				out[j].Dur = durClass(out[j].Symbol, out[j].Nasal, true)
				stressIdx = j
				break
			}
		}
	}
	return out
}

// Consultar: retorna a pronúncia lexical, ou false.
func Consultar(palavra string) ([]Phoneme, bool) {
	normalizada := strings.ToLower(strings.Trim(palavra, ".,;:!?()\"'-"))
	ent, ok := lexico[normalizada]
	if !ok {
		return nil, false
	}
	// cópia defensiva
	out := make([]Phoneme, len(ent.fonemas))
	copy(out, ent.fonemas)
	return out, true
}

// durClass: classe de duração a partir do símbolo SAMPA.
func durClass(sym string, nasal, stress bool) DurClass {
	if nasal {
		return DurNasal
	}
	switch sym {
	case "a", "e", "i", "o", "u", "E", "O":
		if stress {
			return DurVowelLong
		}
		return DurVowelShort
	case "p", "b", "t", "d", "k", "g":
		return DurStop
	case "f", "v", "s", "z", "S", "Z":
		return DurFricative
	case "m", "n", "J":
		return DurNasalCons
	case "l", "L", "r", "R":
		return DurLiquid
	case "j", "w":
		return DurGlide
	}
	return DurPause
}