package cli

// voice_chat_action.go — PERCEPÇÃO POR AÇÃO (sob demanda).
//
// O Don pediu: o COSCa NÃO deve processar a visão a cada frame o tempo todo
// (pesa o sistema). Quando o Don PEDE (por comando falado), o COSCa usa as
// FERRAMENTAS de percepção. Este arquivo é a ponte: (1) reconhece a AÇÃO no
// texto transcrito pelo STT ("olha a tela" / "grava 30s e interpreta"), (2)
// dispara a ferramenta (internal/visionact), e (3) entrega o RESULTADO ao
// cérebro (deliberação) para responder em PT-BR.
//
// Sem build tag: compila na build padrão (e nos testes) sem STT/TTS. A
// integração ao loop de voz real está em voice_chat_sherpa.go.
//
// Reconhecimento (intenção, em PT-BR, sem acentos — STT nem sempre acentua):
//   - "olha a tela" / "o que você vê" / "vê isso"  → LookAtScreen
//   - "grava" / "registra" / "<N> segundos" / "interpreta" → RecordAndInterpret

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/visionact"
)

// ──────────────────────────────────────────────────────────────
// Ação de percepção (intent do Don)
// ──────────────────────────────────────────────────────────────

// perceptionActionKind enumera as ações de percepção que o COSCa pode executar.
type perceptionActionKind int

const (
	// actionNone significa "nenhuma ação de percepção" → comportamento normal.
	actionNone perceptionActionKind = iota
	// actionLookAtScreen → captura 1 frame e interpreta AGORA.
	actionLookAtScreen
	// actionRecordAndInterpret → grava N segundos e interpreta a mudança.
	actionRecordAndInterpret
)

// perceptionAction é a intenção parseada + a duração da gravação (se aplicável).
type perceptionAction struct {
	kind     perceptionActionKind
	duration time.Duration
}

// recordVerbs são os gatilhos fortes de gravação/interpretação. São REGEX (a
// texto normalizado, minúsculo, sem acentos).
var recordVerbs = []string{
	`\bgrava\b`, `\bgrave\b`, `\bgravar\b`,
	`\bregistra\b`, `\bregistre\b`, `\bregistrar\b`,
	`\binterpreta\b`, `\binterprete\b`, `\binterpretar\b`,
}

// lookPatterns são os gatilhos de "olha a tela agora". REGEX sobre texto
// normalizado (minúsculo, sem acentos).
var lookPatterns = []string{
	`\bolha\b`, `\bolhe\b`, `\bolhar\b`, `\bveja\b`, `\bvejo\b`,
	`o que voce ve`, `o que voce esta vendo`, `o que esta vendo`,
	`que esta vendo`, `o que esta na tela`, `o que tem na tela`,
	`ve a tela`, `ve essa tela`, `veja a tela`, `veja isso`, `ve isso`,
	`olha a tela`, `olha essa tela`, `olhar a tela`, `olha a tela toda`,
}

// reDuration extrai "<N> segundos" / "<N>s" de um texto (para a duração da
// gravação). Regex sobre texto normalizado.
var reDuration = regexp.MustCompile(`(\d+)\s*(?:segundos|segundo|s)\b`)

// parsePerceptionAction inspeciona a fala do Don e reconhece a intenção de
// percepção. Prioridade:
//  1. verbo de gravação (grava/registra/interpreta…) → RecordAndInterpret;
//  2. verbo/frase de olhar (olha/vê…/o que você vê) → LookAtScreen;
//  3. senão → actionNone (comportamento normal).
//
// A duração da gravação é extraída de "<N> segundos"; se ausente, usa o padrão
// (visionact.DefaultRecordDuration = 30s).
func parsePerceptionAction(utterance string) perceptionAction {
	text := normalizePT(strings.ToLower(strings.TrimSpace(utterance)))
	if text == "" {
		return perceptionAction{kind: actionNone}
	}

	// 1. Verbo de gravação é o sinal mais forte.
	if hasAnyPattern(text, recordVerbs) {
		d, _ := recordDuration(text)
		if d <= 0 {
			d = visionact.DefaultRecordDuration
		}
		return perceptionAction{kind: actionRecordAndInterpret, duration: d}
	}

	// 2. Olhar a tela agora.
	if hasAnyPattern(text, lookPatterns) {
		return perceptionAction{kind: actionLookAtScreen}
	}

	return perceptionAction{kind: actionNone}
}

// recordDuration extrai a duração em segundos de um texto ("30 segundos",
// "30s", "1 minuto e 30 segundos" → 30s). Retorna (0,false) quando não há uma
// duração reconhecível.
func recordDuration(text string) (time.Duration, bool) {
	m := reDuration.FindStringSubmatch(text)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n <= 0 {
		return 0, false
	}
	// Limite de segurança: uma gravação sensata não passa de 10 minutos.
	if n > 600 {
		n = 600
	}
	return time.Duration(n) * time.Second, true
}

// hasAnyPattern reports whether any of the regexps matches `text`.
func hasAnyPattern(text string, patterns []string) bool {
	for _, p := range patterns {
		if re := regexp.MustCompile(p); re.MatchString(text) {
			return true
		}
	}
	return false
}

// normalizePT remove acentos e normaliza para minúsculo ASCII, para que o
// reconhecimento funcione mesmo quando o STT omite/erra os acentos PT-BR.
func normalizePT(s string) string {
	replacer := strings.NewReplacer(
		"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
		"é", "e", "è", "e", "ê", "e", "ë", "e",
		"í", "i", "ì", "i", "î", "i", "ï", "i",
		"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
		"ú", "u", "ù", "u", "û", "u", "ü", "u",
		"ç", "c",
	)
	return replacer.Replace(strings.ToLower(s))
}

// ──────────────────────────────────────────────────────────────
// buildActionEngine — o engine de ferramentas de percepção (produção)
// ──────────────────────────────────────────────────────────────

// buildActionEngine monta o engine de percepção POR AÇÃO (não o loop contínuo).
// Usa o captor de tela real + a pipeline de visão nativa Go ONNX (memoizada, a
// do leak fix). É o que o voice chat usa para "olha a tela" / "grava e
// interpreta" — sem nenhum loop de fundo.
func buildActionEngine(_ config.PerceptionConfig, logger zerolog.Logger) *visionact.Engine {
	return visionact.New(visionact.WithLogger(logger))
}

// ──────────────────────────────────────────────────────────────
// runPerceptionAction — dispara a ferramenta e delibera
// ──────────────────────────────────────────────────────────────

// runPerceptionAction executa a ação de percepção reconhecida, monta o contexto
// perceptual do RESULTADO (o Observation.SummaryText() ou o relatório de
// mudança) e entrega ao cérebro para a resposta inteligente em PT-BR.
//
// Degradação graciosa (nunca quebra o loop):
//   - ferramenta falha (captura/visão) → responde com fallback honesto + erro
//     (logado pelo chamador);
//   - sem cérebro / modelo falha → responde com o template da ferramenta.
func runPerceptionAction(ctx context.Context, act perceptionAction, engine *visionact.Engine,
	utterance string, mem *memoryManagerAdapter, logger zerolog.Logger) (string, error) {

	memoryHints := buildMemoryHints(ctx, mem, utterance, 4)

	switch act.kind {
	case actionLookAtScreen:
		obs, err := engine.LookAtScreen(ctx)
		if err != nil {
			logger.Warn().Err(err).Msg("voice chat: LookAtScreen degraded")
			return respondWithTool("", utterance), err
		}
		perceptionText := ""
		if obs != nil {
			perceptionText = sanitizePerceptionContext(obs.SummaryText())
		}
		return respondWithDeliberationTool(ctx, perceptionText, utterance, memoryHints)

	case actionRecordAndInterpret:
		report, err := engine.RecordAndInterpret(ctx, act.duration)
		if err != nil {
			logger.Warn().Err(err).Msg("voice chat: RecordAndInterpret degraded")
			return respondWithTool("", utterance), err
		}
		return respondWithDeliberationTool(ctx, report, utterance, memoryHints)

	default:
		return respondWithTool("", utterance), nil
	}
}

// describeAction devolve uma descrição legível da ação reconhecida (para o
// usuário ver o que o COSCa entendeu).
func describeAction(act perceptionAction) string {
	switch act.kind {
	case actionLookAtScreen:
		return "olhar a tela (LookAtScreen)"
	case actionRecordAndInterpret:
		return fmt.Sprintf("gravar %.0fs e interpretar (RecordAndInterpret)", act.duration.Seconds())
	default:
		return "nenhuma ação de percepção"
	}
}

// respondWithTool é o template de FALLBACK quando a ferramenta de percepção não
// tem cérebro (ou o modelo falha/degradou). É honesto: se a ferramenta não
// retornou nada, diz que não conseguiu perceber; senão, relata o que percebeu.
func respondWithTool(toolContext, utterance string) string {
	ctxText := strings.TrimSpace(toolContext)
	u := strings.TrimSpace(utterance)

	if ctxText == "" {
		if u != "" {
			return fmt.Sprintf("Não consegui perceber nada agora. Você pediu: %q.", u)
		}
		return "Não consegui perceber nada agora."
	}
	if u != "" {
		return fmt.Sprintf("%s Você pediu: %q.", strings.TrimSpace(ctxText), u)
	}
	return ctxText
}

// shouldRespond decide se o COSCa deve responder a uma fala finalizada. Ele
// evita o COSCa responder a CADA som/ruído captado (loop infinito): só responde
// quando a fala é uma PERGUNTA, uma AÇÃO de percepção, ou UM ENDEREÇAMENTO
// ("cosca..."). Palavras soltas ou ruído são ignoradas.
//
// Regras (em ordem):
//  1. Muito curto (<5 caracteres ou <2 palavras) → ruído, ignora.
//  2. Contém pergunta ("o que", "como", "por que", "?"...) → responde.
//  3. Contém ação de percepção ("olha", "grava", "veja", "vê") → responde.
//  4. Contém endereçamento ("cosca", "você pode", "consegue") → responde.
//  5. Caso contrário → ignora (fala solta sem pedido).
func shouldRespond(u string) bool {
	u = strings.TrimSpace(strings.ToLower(u))
	if u == "" {
		return false
	}
	words := strings.Fields(u)
	if len(words) < 2 {
		return false
	}
	if len(u) < 5 {
		return false
	}
	// Ações de percepção.
	if strings.Contains(u, "olha") || strings.Contains(u, "olhe") ||
		strings.Contains(u, "veja") || strings.Contains(u, " ve ") ||
		strings.Contains(u, "grava") || strings.Contains(u, "grave") ||
		strings.Contains(u, "registra") || strings.Contains(u, "registre") {
		return true
	}
	// Endereçamento.
	if strings.Contains(u, "cosca") || strings.Contains(u, "robo") {
		return true
	}
	// Pergunta.
	for _, q := range []string{"o que", "que que", "como", "por que", "porque", "qual", "onde",
		"voce", "você", "consegue", "pode", "diz", "me diz", "ve", "viu", "está", "esta "} {
		if strings.Contains(u, q) {
			return true
		}
	}
	if strings.Contains(u, "?") {
		return true
	}
	return false
}

// sanitizePerceptionContext limpa o relatório de visão antes de entregar ao
// cérebro. Remove os avisos de degradação ("N degradation warning(s): ...") —
// que são técnicos e levam o modelo a responder "configuração de modelos
// degradada" em vez de DESCREVER a tela. O que interessa ao cérebro são as
// entidades detectadas; a saúde do pipeline não deve virar desculpa.
func sanitizePerceptionContext(summary string) string {
	if summary == "" {
		return ""
	}
	// Remove a partir de "degradation warning" até o fim da linha que termina
	// com ";" ou fim da string (o bloco de avisos vem no final).
	idx := strings.Index(summary, "degradation warning")
	if idx >= 0 {
		// Corta da linha que contém "degradation". Conserva tudo antes dela.
		lineStart := strings.LastIndex(summary[:idx], "\n")
		if lineStart >= 0 {
			summary = strings.TrimSpace(summary[:lineStart])
		} else {
			summary = strings.TrimSpace(summary[:idx])
		}
	}
	// Garante um fallback honesto se a limpeza deixou vazio.
	if strings.TrimSpace(summary) == "" {
		return "Tela analisada."
	}
	// Uma detecção vazia é resultado válido (a tela pode não ter os elementos
	// esperados) — o cérebro deve descrever, não reclamar.
	if strings.Contains(summary, "0 entity") {
		return "Tela analisada. Nenhum objeto destacado detectado — descreva como a tela parece estar."
	}
	return summary
}
