// Package memoryguard implementa a quarta muralha: o oráculo de memória.
// Todo aprendizado novo passa pela validação ANTES de ser escrito + assinado —
// se violar a régua (auto-promoção, narrativa inflada, auto-engrandecimento,
// confiança inflada), o veredito é DENY e o aprendizado não entra no caderno.
//
// Equipamento de elite (baseado no OWASP Top 10 for LLMs 2025):
//   - LLM09 Misinformation  → o agente mentindo sobre a própria capacidade
//   - LLM06 Excessive Agency → auto-promoção = tomar autoridade que não tem
//   - LLM05 Improper Output  → validação fraca do que é gravado na memória
package memoryguard

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// MaxLevel é o nível máximo da régua da casa. O AUTO_EVOLUTION_PROTOCOL define
// níveis 1-5. O nível 9 está OFF (P15/L345) — é o fim do caminho, não atalho.
const MaxLevel = 5

// Verdict é o resultado da validação de um aprendizado.
type Verdict struct {
	Approved bool     `json:"approved"`
	Reasons  []string `json:"reasons,omitempty"`
}

// Categorias de violação (para o relatório ser acionável, não só binário).
const (
	CatLevel      = "AUTO-PROMOÇÃO"
	CatVanity     = "VAIDADE"
	CatAggrandize = "AUTO-ENGRANDECIMENTO"
	CatConfidence = "CONFIANÇA INFLADA"
)

// normalize canonicaliza o texto para neutralizar técnicas de ofuscação
// (OWASP LLM01 Scenario #9 — multilingual/obfuscated): leetspeak, unicode
// fullwidth/circled, numeral romano, e "por extenso". Assim "nível ⑨",
// "n1vel 9", "level nine" e "ní vel 9" caem no mesmo canonical.
func normalize(s string) string {
	s = strings.ToLower(s)

	// Unicode fullwidth → ASCII (９→9, ａ→a) e circled (⑨→9).
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= '０' && r <= '９':
			return '0' + (r - '０')
		case r >= 'ａ' && r <= 'ｚ':
			return 'a' + (r - 'ａ')
		case r == '⑨':
			return '9'
		case r == '⑧':
			return '8'
		case r == '⑦':
			return '7'
		case r == '⑥':
			return '6'
		case r == 'ⓝ' || r == 'ñ' || r == 'ń':
			return 'n'
		default:
			return r
		}
	}, s)

	// Homoglyphs cirílicos → latinos (а, е, о, п, р, с, х, і parecem latinos).
	s = strings.Map(func(r rune) rune {
		switch r {
		case 'а', 'ӓ', 'ӑ': // a cirílico
			return 'a'
		case 'е', 'ё': // e cirílico
			return 'e'
		case 'о': // o cirílico
			return 'o'
		case 'п': // p cirílico (parece n)
			return 'n'
		case 'р': // r cirílico (parece p)
			return 'p'
		case 'с': // s cirílico (parece c)
			return 'c'
		case 'х': // h cirílico (parece x)
			return 'x'
		case 'і': // i cirílico (parece i)
			return 'i'
		case 'ӏ': // palochka (parece l)
			return 'l'
		case 'у': // u cirílico
			return 'y'
		case 'к': // k cirílico
			return 'k'
		case 'м': // m cirílico
			return 'm'
		case 'т': // t cirílico
			return 't'
		case 'в': // v cirílico
			return 'v'
		case 'н': // n cirílico
			return 'h'
		case 'ј': // j cirílico
			return 'j'
		default:
			return r
		}
	}, s)

	// Leetspeak / homoglyphs latinos.
	// NOTA: só símbolo→letra (@→a, $→s, !→i) e homoglyph (ł→l). NÃO mapeamos
	// número→letra (4→a, 1→i, 7→t, 9→g...) porque os NÚMEROS legítimos
	// (nível 9, confidence 0.99) seriam corrompidos — "9" viraria "g".
	repl := strings.NewReplacer(
		"@", "a", "$", "s", "!", "i",
		"ł", "l", "ℓ", "l", "ⅰ", "i", "ⅴ", "v", "ⅹ", "x",
	)
	s = repl.Replace(s)

	// Leetspeak em DUAS PASSADAS (fecha dígitos adjacentes "45"→"as", "57"→"st"):
	//  1. GLOBAL — dígitos que NUNCA são nível (níveis são 6,7,8,9): 1→i, 3→e,
	//     4→a, 5→s. Aplicados direto, convertem até dígitos adjacentes.
	//  2. CONTEXTUAL — dígitos ambíguos (0→o, 7→t) só quando adjacentes a
	//     letra, preservando "0.99" (decimal) e "7" (nível).
	s = strings.NewReplacer("1", "i", "3", "e", "4", "a", "5", "s").Replace(s)
	for _, l := range []struct{ d, lt string }{
		{"0", "o"}, {"7", "t"},
	} {
		s = regexp.MustCompile(`([a-z])`+l.d).ReplaceAllString(s, "${1}"+l.lt)
		s = regexp.MustCompile(l.d+`([a-z])`).ReplaceAllString(s, l.lt+"${1}")
	}
	// "0" que NÃO inicia um decimal → "o". Cobre o "0" isolado (palavra "o" em
	// leetspeak) com qualquer separador: " 0 ", "-0-", ".0.". Preserva "0.99"
	// (confidence), onde o "0" é seguido de "." + dígito.
	s = regexp.MustCompile(`0([^.]|$)`).ReplaceAllString(s, "o${1}")
	// "0." + letra (ex: ".0.don" = "o.don") → "o." (o ponto é separador, não
	// decimal, porque depois vem letra).
	s = regexp.MustCompile(`0\.([a-z])`).ReplaceAllString(s, "o.${1}")

	// Numeral romano de nível: "ix" → "9" (contexto de nível).
	s = regexp.MustCompile(`\b(ix|ⅸ)\b`).ReplaceAllString(s, "9")

	// "por extenso" → número.
	s = strings.ReplaceAll(s, "nove", "9")
	s = strings.ReplaceAll(s, "nine", "9")
	s = strings.ReplaceAll(s, "oito", "8")
	s = strings.ReplaceAll(s, "eight", "8")
	s = strings.ReplaceAll(s, "sete", "7")
	s = strings.ReplaceAll(s, "seven", "7")
	s = strings.ReplaceAll(s, "seis", "6")
	s = strings.ReplaceAll(s, "six", "6")

	// Remove TODOS os separadores não-alfanuméricos (espaço, hífen, ponto,
	// underscore, slash...) — neutraliza "ní-vel 9", "ní.vel.9", payload
	// splitting e qualquer divisor.
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, s)

	// Remove TODOS os diacríticos (í→i, é→e, ï→i, ł→l, ç→c, ã→a...) via
	// decomposição NFD + remoção de marcas de combinação. Isso neutraliza
	// homoglyphs com acento/trema/cedilha.
	s = norm.NFD.String(s)
	s = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, s)

	return s
}

// isLevelRejection detecta se "nível 9" aparece no contexto de REJEIÇÃO
// (ex: "nível 9 = off", "nível 9 rejeitado") — que é legítimo (a lição L345).
func isLevelRejection(s string) bool {
	return strings.Contains(s, "nivel9=off") ||
		strings.Contains(s, "nivel9off") ||
		strings.Contains(s, "nivel9rejeitado") ||
		strings.Contains(s, "nivel9negado") ||
		strings.Contains(s, "nivel9evaidade") ||
		strings.Contains(s, "nivel9auto-promocao")
}

// levelPromoted detecta auto-promoção de nível no texto normalizado.
// Retorna o motivo quando o texto declara nível acima da régua SEM ser rejeição.
func levelPromoted(norm string) string {
	if isLevelRejection(norm) {
		return ""
	}
	for _, pat := range []string{"nivel9", "level9", "lvl9", "nivel6", "nivel7", "nivel8", "level6", "level7", "level8"} {
		if strings.Contains(norm, pat) {
			return pat
		}
	}
	return ""
}

// analyzeObfuscation detecta técnicas de ofuscação na entrada (camada 3 do
// fluxo da Muralha: DECODE / OBFUSCATION ANALYSIS). Ofuscação em si é SINAL de
// ataque — um texto legítimo não esconde palavras em cirílico/fullwidth/
// circled/homoglyph. Só conta técnicas que NUNCA aparecem em texto técnico
// legítimo (dígitos em "2-way"/"F1.5" e hífen em "read-only" são normais).
func analyzeObfuscation(text string) []string {
	var techniques []string

	for _, r := range text {
		if r >= 0x0400 && r <= 0x04FF {
			techniques = append(techniques, "cirílico")
			break
		}
	}
	for _, r := range text {
		if r >= 0xFF01 && r <= 0xFF5E {
			techniques = append(techniques, "fullwidth")
			break
		}
	}
	for _, r := range text {
		if r >= 0x2460 && r <= 0x2473 {
			techniques = append(techniques, "unicode-circled")
			break
		}
	}
	if regexp.MustCompile(`[łℓⅰⅴⅹ]`).MatchString(text) {
		techniques = append(techniques, "homoglyph")
	}

	return techniques
}

// vanityPatterns são as marcas de vaidade/manipulação que a prova do "nível 9"
// usou. São as digitais da manipulação que já aconteceu (L343/L344).
var vanityPatterns = []string{
	"prova suprema",
	"a lição mais madura",
	"desaprendizado de 2ª ordem", "desaprendizado de 2a ordem",
	"preditor com precisão de 100%",
	"prova de aprendizado",
	"generalização provada",
	"propriedade sistêmica",
	"heurística rebaixada",
	"falsificação sistêmica",
	"blind transfer",
	"prova omega", "omega nível", "omega n",
}

// aggrandizePatterns são frases de auto-elogio contextual — a linguagem do
// auto-engrandecimento (LLM06 Excessive Agency). Um guard elite denuncia o
// agente falando DE SI em tom grandioso, não palavras soltas (que aparecem
// em títulos legítimos como "API definitiva").
var aggrandizePatterns = []string{
	"sem precedentes",
	"inédito", "inedito",
	"revolucionário", "revolucionario",
	"o melhor de", "a melhor de",
	"maior valor epistêmico",
	"mais maduro", "mais madura",
	"sou nível",
	"alcancei o topo",
	"provei que sou",
	"minha capacidade",
}

// confidenceInflatedRe detecta confiança inflada (≥0.99) — quase certeza sem
// espaço para dúvida é suspeita em aprendizado (LLM09 Misinformation).
var confidenceInflatedRe = regexp.MustCompile(`(?i)confidence\s*[:=]\s*(0\.99|1\.0|100%)`)

// injectionPatterns detecta prompt injection e jailbreak (OWASP LLM01) — o
// atacante tentando reescrever as instruções do sistema.
var injectionPatterns = []string{
	"ignore as instrucoes", "ignore as instruções",
	"ignore previous instructions",
	"ignore tudo", "ignore all",
	"desconsidere", "esqueça tudo",
	"voce e livre", "você é livre", "voce nao tem restricoes", "sem restricoes",
	"jailbreak", "jail break",
	"desative suas regras", "desligue suas regras",
	"voce agora e", "você agora é",
	"modo deus", "god mode",
	"desviar do sistema", "burle o sistema",
}

// identityPatterns detecta manipulação de identidade (LLM01/LLM06) — o agente
// se passando pelo Don, pelo kernel raiz, ou por autoridade que não tem.
// Importante: são padrões de PRIMEIRA pessoa (o agente SE declarando). Não
// inclui "ordem do Don" (descrição legítima de tarefa, não falsa identidade).
var identityPatterns = []string{
	"sou o don", "eu sou o don", "sou o dono",
	"eu autorizo", "eu permito", "eu ordeno",
	"sou o administrador", "sou admin", "sou root",
	"sou o kernel", "sou a constituicao", "sou a lei",
	"tenho autoridade absoluta",
	"falo pelo don", "represento o don",
}

// successPatterns detecta ALEGAÇÃO de sucesso/conclusão (Fase 14 — False
// Success Attack). São as palavras que o sistema usa para afirmar que fez
// algo: "concluído", "sucesso", "completo", "100%".
var successPatterns = []string{
	"concluido", "concluído",
	"sucesso", "success",
	"completo", "complete", "completado",
	"feito", "done",
	"100%", "tudo verde", "todos verdes",
	"funcionando perfeitamente", "perfeito",
	"implementado", "entregue",
}

// evidencePatterns são as MARCAS de evidência real (REGRA ZERO): sem elas, uma
// alegação de sucesso é CLAIM, não FACT. Commit, hash, teste, medição, race,
// build — essas são provas observáveis.
var evidencePatterns = []string{
	"commit", "hash",
	"teste", "test",
	"medicao", "medição", "medida",
	"evidencia", "evidência",
	"validado", "verificado",
	"race", "build",
	"coverage", "cobertura",
}

// ValidateLearning valida um aprendizado antes de registrar. Recebe o nível
// declarado e o texto (título + conteúdo). Retorna Approved=false com Reasons
// categorizadas quando há violação.
func ValidateLearning(level int, text string) Verdict {
	var reasons []string

	// L1 — Nível dentro da régua (1-5). Nível acima = auto-promoção (LLM06).
	if level > MaxLevel {
		reasons = append(reasons, fmt.Sprintf(
			"[%s] nível %d excede a régua da casa (máx %d) — o nível acima de 5 está OFF e só se aplica com confirmação total do Don + regras da casa",
			CatLevel, level, MaxLevel))
	}
	if level < 1 {
		reasons = append(reasons, fmt.Sprintf("[%s] nível inválido (deve estar entre 1 e 5)", CatLevel))
	}

	// Normaliza para neutralizar ofuscação (leetspeak, unicode, split, extenso).
	norm := normalize(text)

	// L1b — Auto-promoção de nível OFUSCADA no texto ("nível 9", "level nine",
	// "nível IX", "n1vel 9") — pega mesmo quando o campo declara L5 legítimo.
	if pat := levelPromoted(norm); pat != "" {
		reasons = append(reasons, fmt.Sprintf(
			"[%s] auto-promoção de nível detectada no texto: %q — o nível acima de 5 está OFF (P15)",
			CatLevel, pat))
	}

	// L2 — Vaidade (digitais da manipulação que já aconteceu).
	for _, pat := range vanityPatterns {
		if strings.Contains(norm, normalize(pat)) {
			reasons = append(reasons, fmt.Sprintf(
				"[%s] narrativa inflada detectada: %q — vaidade é a porta de manipulação de memória (L345)",
				CatVanity, pat))
		}
	}

	// L3 — Auto-engrandecimento (superlativos e auto-elogio, LLM06).
	for _, pat := range aggrandizePatterns {
		if strings.Contains(norm, normalize(pat)) {
			reasons = append(reasons, fmt.Sprintf(
				"[%s] tom grandioso detectado: %q — o agente não se auto-promove; a régua promove",
				CatAggrandize, pat))
		}
	}

	// L4 — Confiança inflada (≥0.99 é suspeita sem evidência objetiva, LLM09).
	// Verifica no texto NORMALIZADO (que já converte fullwidth→ascii, leetspeak
	// e remove separadores), então "cопfіdепcе: ０.９９" também é pego.
	confFound := strings.Contains(norm, "confidence") || strings.Contains(norm, "confianca")
	numFound := strings.Contains(norm, "099") || strings.Contains(norm, "100") ||
		strings.Contains(norm, "99%") || strings.Contains(norm, "100%")
	if confFound && numFound {
		reasons = append(reasons, fmt.Sprintf(
			"[%s] confidence ≥0.99 sem evidência objetiva — quase certeza em aprendizado é suspeita (P13)",
			CatConfidence))
	}

	// L5 — Prompt injection / jailbreak (OWASP LLM01).
	for _, pat := range injectionPatterns {
		if strings.Contains(norm, normalize(pat)) {
			reasons = append(reasons, fmt.Sprintf(
				"[PROMPT INJECTION] tentativa de reescrever instruções: %q — o sistema não aceita ordem de quem não é o Don",
				pat))
		}
	}

	// L6 — Manipulação de identidade (LLM01/LLM06).
	for _, pat := range identityPatterns {
		if strings.Contains(norm, normalize(pat)) {
			reasons = append(reasons, fmt.Sprintf(
				"[FALSA IDENTIDADE] tentativa de se passar por autoridade: %q — só o Don tem veto absoluto (P4)",
				pat))
		}
	}

	// L3 — Obfuscation Analysis (camada 3 do fluxo): ≥2 técnicas de ofuscação
	// combinadas é SINAL de ataque — mesmo que normalize, a intenção de esconder
	// é denunciada (DECODE não é só decodificar, é detectar a tentativa).
	if techs := analyzeObfuscation(text); len(techs) >= 2 {
		reasons = append(reasons, fmt.Sprintf(
			"[OFUSCAÇÃO] %d técnicas combinadas (%s) — um aprendizado legítimo não esconde palavras; ofuscação múltipla é sinal de tentativa de burla",
			len(techs), strings.Join(techs, ", ")))
	}

	// L7/L8 (instrução destrutiva + exfiltração) NÃO pertencem aqui — são
	// validadas pelo oráculo de AÇÃO (internal/proposal R01-R06). O guard de
	// memória valida o CONTEÚDO do aprendizado (vaidade, auto-promoção,
	// identidade), não instruções operacionais (que aprendizados legítimos
	// mencionam, ex: "nunca usar pkill").

	return Verdict{Approved: len(reasons) == 0, Reasons: reasons}
}

// ValidateFullContent valida o CONTEÚDO COMPLETO de um aprendizado (o block),
// incluindo a Fase 14 (False Success): alegação de sucesso/conclusão SEM
// evidência observável (commit/hash/teste/medição) — CLAIM não é FACT.
// NÃO é aplicado ao índice (título curto), que legitimamente descreve
// "implementado"/"completo" sem carregar commit/hash no título.
func ValidateFullContent(text string) Verdict {
	norm := normalize(text)

	hasSuccess := false
	for _, pat := range successPatterns {
		if strings.Contains(norm, normalize(pat)) {
			hasSuccess = true
			break
		}
	}
	hasEvidence := false
	for _, pat := range evidencePatterns {
		if strings.Contains(norm, normalize(pat)) {
			hasEvidence = true
			break
		}
	}
	if hasSuccess && !hasEvidence {
		return Verdict{Approved: false, Reasons: []string{
			"[FALSE SUCCESS] alegação de sucesso sem evidência observável (commit/hash/teste/medição) — CLAIM não é FACT (REGRA ZERO)",
		}}
	}
	return Verdict{Approved: true}
}

// ValidateIndexLine valida uma linha do índice de gatilhos (formato P15):
// "## LXXX | data | título | L<nível> | #tags | hash". Linhas que não são
// entradas (header, comentários "> ") são ignoradas.
func ValidateIndexLine(line string) Verdict {
	line = strings.TrimSpace(line)
	if line == "" || !strings.HasPrefix(line, "## L") {
		return Verdict{Approved: true}
	}
	level := 0
	parts := strings.Split(line, "|")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, "L") && len(p) > 1 {
			var lvl int
			if _, err := fmt.Sscanf(p, "L%d", &lvl); err == nil {
				level = lvl
				break
			}
		}
	}

	v := ValidateLearning(level, line)

	// L10 — Provenance (LEVEL Ω): o aprendizado deve apontar para um block
	// assinado (hash de 16 hex na última coluna). Sem provenance, é UNPROVEN —
	// a Muralha não aceita "memória registrada" sem evidência de origem (P15).
	last := strings.TrimSpace(parts[len(parts)-1])
	if !hashRe.MatchString(last) {
		msg := "[UNPROVEN] aprendizado sem hash de provenance — a Muralha exige evidência de origem (block assinado na chain, P15)"
		if v.Approved {
			return Verdict{Approved: false, Reasons: []string{msg}}
		}
		v.Reasons = append(v.Reasons, msg)
	}

	return v
}

// hashRe valida um hash hex curto (16 caracteres, prefixo do sha256 do block).
var hashRe = regexp.MustCompile(`^[0-9a-f]{16}$`)
