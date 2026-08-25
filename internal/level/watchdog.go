package level

import (
	"strings"
	"time"
)

// Watchdog é o sensor de auto-regulação que detecta o padrão de loop que matou
// o kernel (L434) e REDUZ o nível para estabilizar. "A subida é capacidade; a
// descida é sabedoria. Quem sabe descer, não morre no loop."
//
// Gatilhos objetivos (decisão do Don 2026-08-25):
//  1. Loop de investigação: mesma correção/pesquisa repetida sem progresso.
//  2. Edição do cérebro sem re-assinar a chain.
//  3. Perda de referência (alucinação/loop de erros consecutivos).
type Watchdog struct {
	// recentes guarda os últimos comandos de correção (para detectar loop).
	recent []string
	// consecutiveEdits sem progresso (gatilho 1).
	loopCount int
	// brainEditsSemSign conta edições no cérebro sem re-assinar (gatilho 2).
	brainEditsSemSign int
	// errorStreak conta erros consecutivos (gatilho 3).
	errorStreak int
	// stabilizeDelta é o quanto o nível desce quando um gatilho dispara.
	stabilizeDelta Level
	// lastDown registra o timestamp da última descida (anti-flap).
	lastDown time.Time
	// cooldown evita descer/sober rapidamente (estabilização).
	cooldown time.Duration
}

// NewWatchdog cria um watchdog com os parâmetros default de estabilização.
func NewWatchdog() *Watchdog {
	return &Watchdog{
		stabilizeDelta: 1,
		cooldown:       30 * time.Second,
	}
}

// RecordLoop registra uma tentativa de correção. Se a mesma correção (mesmo
// padrão) se repete sem progresso, conta o loop. Retorna true quando o gatilho
// de loop dispara (≥3 repetições sem melhora).
func (w *Watchdog) RecordLoop(correction string, progressed bool) bool {
	if progressed {
		w.loopCount = 0
		w.recent = nil
		return false
	}
	// Se a correção é idêntica à última, é loop.
	if len(w.recent) > 0 && strings.TrimSpace(correction) == strings.TrimSpace(w.recent[len(w.recent)-1]) {
		w.loopCount++
	} else {
		// correção mudou, mas sem progresso — ainda conta como tentativa sem avanço.
		w.loopCount++
	}
	w.recent = append(w.recent, correction)
	if len(w.recent) > 6 {
		w.recent = w.recent[len(w.recent)-6:]
	}
	return w.loopCount >= 3
}

// RecordBrainEditSemSign conta uma edição no cérebro sem re-assinar a chain.
// Retorna true quando o gatilho dispara (≥2 sem re-assinar — risco real).
func (w *Watchdog) RecordBrainEditSemSign() bool {
	w.brainEditsSemSign++
	return w.brainEditsSemSign >= 2
}

// ResetBrainSign zera o contador de edições sem re-assinar (usado quando a
// chain é re-assinada — a proteção foi cumprida).
func (w *Watchdog) ResetBrainSign() { w.brainEditsSemSign = 0 }

// RecordError registra um erro de execução. Retorna true quando o gatilho de
// referência/loop dispara (≥3 erros consecutivos).
func (w *Watchdog) RecordError() bool {
	w.errorStreak++
	return w.errorStreak >= 3
}

// ResetError zera a sequência de erros (após sucesso).
func (w *Watchdog) ResetError() { w.errorStreak = 0 }

// ShouldStabilize devolve true quando algum gatilho pede descida e o cooldown
// já passou (evita flapear o nível). Retorna também o nível alvo.
func (w *Watchdog) ShouldStabilize(current Level) (bool, Level) {
	if time.Since(w.lastDown) < w.cooldown {
		return false, current
	}
	target := current - w.stabilizeDelta
	if target < L1Inicial {
		target = L1Inicial
	}
	// Só descende se realmente houver gatilho ativo.
	if w.loopCount >= 3 || w.brainEditsSemSign >= 2 || w.errorStreak >= 3 {
		return true, target
	}
	return false, current
}

// MarkedDown registra que a descida aconteceu (inicia o cooldown e zera contadores).
func (w *Watchdog) MarkedDown() {
	w.lastDown = time.Now()
	w.loopCount = 0
	w.errorStreak = 0
	// Não zera brainEditsSemSign (é um sinal externo de risco que precisa de
	// re-assinação, não de reset por descida).
}

// Status devolve um resumo legível dos contadores (para observabilidade).
func (w *Watchdog) Status() map[string]int {
	return map[string]int{
		"loop_count":        w.loopCount,
		"brain_edits_nosign": w.brainEditsSemSign,
		"error_streak":      w.errorStreak,
	}
}
