// Package evidence define a epistemologia de observação do COSCA.
//
// PRINCÍPIO minerado (Osiris, MIT — src/lib/sherlock.ts) sob ADR-037. NENHUM
// código do repositório-fonte foi importado; apenas o princípio foi reimplementado
// como primitiva Go idiomática. Ver ADR-037.
//
// A primitiva ObservationState é a classe epistêmica de primeira classe da
// Evidence Layer. Ideia-central, transformada em lei:
//
//   - NÃO confundir "ausência de evidência" com "evidência de ausência".
//   - Um instrumento que foi BLOQUEADO (recusou observar) ou INCONCLUSIVE
//     (calibração suspeita) NÃO estabeleceu ausência — só os observou sem proveito.
//   - O estado é SIMÉTRICO: aplica-se ao modelo observando o mundo E ao avaliador
//     observando o modelo (um rótulo de eval também é uma observation).
//
// Diferente de:
//   - level.Verdict  — decisão de AUTORIZAÇÃO de uma ação (allow/deny).
//   - CapabilityStatus — estado de uma CAPACIDADE (medida/verificada/stale...).
//
// ObservationState registra o que se SABE sobre o MUNDO a partir de uma
// observação, e se essa observação foi CONFIÁVEL.
package evidence

import (
	"fmt"
	"strings"
)

// ObservationState é o estado epistêmico de uma observação externa.
type ObservationState string

const (
	// ObservationFound: o referente existe/funcionou E foi observado com confiança.
	ObservationFound ObservationState = "found"
	// ObservationNotFound: AUSÊNCIA PROVADA — verificamos e a ausência foi
	// demonstrada. É a única afirmação de ausência válida.
	ObservationNotFound ObservationState = "not_found"
	// ObservationBlocked: DESCONHECIDO — o instrumento RECUSOU observar
	// (401/403/407/429/451/503). NÃO é ausência.
	ObservationBlocked ObservationState = "blocked"
	// ObservationInconclusive: NÃO-CONFIÁVEL — o instrumento reportou, mas sua
	// calibração está suspeita (soft-404: também "encontra" um controle ausente).
	ObservationInconclusive ObservationState = "inconclusive"
	// ObservationSkipped: a observação não foi tentada (fora de escopo).
	ObservationSkipped ObservationState = "skipped"
	// ObservationError: falha com motivo EXPLÍCITO (nunca implícito).
	ObservationError ObservationState = "error"
)

// String devolve o estado como string canônica.
func (s ObservationState) String() string { return string(s) }

// Valid devolve true para os 6 estados canônicos.
func (s ObservationState) Valid() bool {
	switch s {
	case ObservationFound, ObservationNotFound, ObservationBlocked,
		ObservationInconclusive, ObservationSkipped, ObservationError:
		return true
	}
	return false
}

// IsAbsence devolve true APENAS para ausência PROVADA (ObservationNotFound).
// Lema central do ADR-037: Blocked/Inconclusive NUNCA são ausência.
func (s ObservationState) IsAbsence() bool { return s == ObservationNotFound }

// IsUncertain devolve true quando o instrumento NÃO conseguiu estabelecer o
// estado do mundo: recusou (Blocked) ou calibração suspeita (Inconclusive).
func (s ObservationState) IsUncertain() bool {
	return s == ObservationBlocked || s == ObservationInconclusive
}

// IsConfident devolve true quando a observação é CONFIÁVEL (estabeleceu o estado
// do mundo por observação direta).
func (s ObservationState) IsConfident() bool {
	return s == ObservationFound || s == ObservationNotFound
}

// IsKnown devolve true quando o estado do mundo foi estabelecido (encontrado ou
// não-encontrado) — o oposto do "incognoscível agora". Blocked/Inconclusive são
// incognoscíveis; Skipped/Error são lacunas de observação.
func (s ObservationState) IsKnown() bool {
	return s.IsConfident()
}

// Observation carrega o resultado de uma observação: estado epistêmico + motivo
// explícito (nunca implícito) + evidência que a suporta.
type Observation struct {
	State ObservationState
	// Reason é o motivo explícito (obrigatório para ObservationError; útil para
	// Blocked/Inconclusive/Skipped). Nunca implícito.
	Reason string
	// Evidence é a proveniência/evidência que suporta a observação (opcional).
	Evidence string
}

// Validate verifica a integridade da Observation: estado canônico e, para
// ObservationError, motivo explícito presente (não-engolimento implícito).
func (o Observation) Validate() error {
	if !o.State.Valid() {
		return fmt.Errorf("evidence: estado de observação inválido %q", o.State)
	}
	if o.State == ObservationError && strings.TrimSpace(o.Reason) == "" {
		return fmt.Errorf("evidence: ObservationError exige Reason explícito (nunca implícito)")
	}
	return nil
}

// Calibrate aplica o princípio dos DOIS controles independentes (ADR-037).
// controlsReported é quantos controles ALEATÓRIOS (garantidamente ausentes) o
// detector também "encontrou". Se QUALQUER um deles disparou, o detector é um
// soft-404 → a observação Found vira Inconclusive (não confiável). Dois controles
// robustecem: um único pode escapar quando esse controle é rejeitado pelo site.
func Calibrate(primary ObservationState, controlsReported int) ObservationState {
	if primary == ObservationFound && controlsReported > 0 {
		return ObservationInconclusive
	}
	return primary
}
