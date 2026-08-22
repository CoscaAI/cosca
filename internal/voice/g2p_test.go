package voice

import (
	"strings"
	"testing"
)

// sampa: converte []Phoneme → string SAMPA, pausa = espaço duplo.
func sampa(fonemas []Phoneme) string {
	var b strings.Builder
	for _, f := range fonemas {
		if f.Symbol == " " {
			b.WriteString(" ") // pausa + o espaço do próximo fonema = separação
			continue
		}
		b.WriteString(f.SAMPA())
		b.WriteString(" ")
	}
	return strings.TrimSpace(b.String())
}

func TestG2P(t *testing.T) {
	cases := []struct {
		texto string
		want  string
	}{
		// léxico (evidência LEXICON)
		{"não", "n 'a~ w"},
		{"você", "v o s 'e"},
		{"carro", "k 'a R u"},
		{"cosca", "k 'o S k a"},
		{"conhecimento", "k o J e s i m 'e~ t u"},
		// regras FACT
		{"mesa", "m 'e z a"}, // c+a→k não aplica; s intervocálico→z
		{"chefe", "S 'e f i"},
		{"guarda", "g w 'a R d a"},
		// nasalização CONSUMINDO m/n (correção do Professor)
		{"campo", "k 'a~ p u"},
		{"bem", "b 'e~"},
		{"banana", "b a n 'a n a"}, // m/n + vogal → consoante nasal normal
		// tonicidade por acento gráfico (café → tônica no é, não no a)
		{"café", "k a f 'E"},
		{"silêncio", "s i l 'e~ s i u"},
		// oxítona por regra
		{"amor", "a m 'o r"},
		// paroxítona
		{"mesa", "m 'e z a"},
		// redução de monossílabos átonos
		{"de", "d i"},
		{"o", "u"},
		// x contextual: exame → z
		{"exame", "e z 'a m i"},
		// frase completa
		{"O Cosca protege o conhecimento acima de tudo, chef.",
			"u  k 'o S k a  p r o t 'E Z i  u  k o J e s i m 'e~ t u  a s 'i m a  d i  t 'u d u  S 'e f"},
	}
	for _, c := range cases {
		got := sampa(G2P(c.texto))
		if got != c.want {
			t.Errorf("G2P(%q) = %q, want %q", c.texto, got, c.want)
		}
	}
}

func TestG2PEvidencia(t *testing.T) {
	// léxico → LEXICON
	for _, f := range G2P("cosca") {
		if f.Evid != EvidenceLexicon && f.Symbol != " " {
			t.Errorf("cosca: esperava LEXICON, veio %s", f.Evid)
		}
	}
	// regra certa → FACT (mesa: m, e, z, a — regras)
	fonemas := G2P("mesa")
	for _, f := range fonemas {
		if f.Evid != EvidenceFact {
			t.Errorf("mesa: esperava FACT, veio %s", f.Evid)
		}
	}
	// regra contextual → INFERRED (x em xícara)
	fonemas = G2P("xícara")
	achouX := false
	for _, f := range fonemas {
		if f.Symbol == "S" && f.Evid == EvidenceInferred {
			achouX = true
		}
	}
	if !achouX {
		t.Errorf("xícara: esperava S INFERRED")
	}
}

func TestProsodyPlanner(t *testing.T) {
	fonemas := G2P("O Cosca protege o conhecimento.")
	plano := PlanejarProsodia("O Cosca protege o conhecimento.", fonemas, StyleCoscaExecutive)
	if len(plano.Contour) != len(fonemas) {
		t.Fatalf("contour len %d != fonemas %d", len(plano.Contour), len(fonemas))
	}
	if len(plano.Duration) != len(fonemas) {
		t.Fatalf("duration len %d != fonemas %d", len(plano.Duration), len(fonemas))
	}
	// pergunta deve TERMINAR com pitch mais alto que afirmação
	fp := G2P("O Cosca vai falar?")
	pq := PlanejarProsodia("O Cosca vai falar?", fp, StyleCoscaExecutive)
	fa := G2P("O Cosca vai falar.")
	af := PlanejarProsodia("O Cosca vai falar.", fa, StyleCoscaExecutive)
	// pega o contorno do último fonema de VOGAL (não os separadores)
	lastVowel := func(fonemas []Phoneme, p ProsodyPlan) float64 {
		for i := len(fonemas) - 1; i >= 0; i-- {
			if fonemas[i].IsVowel() {
				return p.Contour[i]
			}
		}
		return 0
	}
	if lastVowel(fp, pq) <= lastVowel(fa, af) {
		t.Errorf("pergunta deve subir no fim: pergunta=%v afirmação=%v", lastVowel(fp, pq), lastVowel(fa, af))
	}
}