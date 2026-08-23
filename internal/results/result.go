// Package results — envelope uniforme de resultado para toda interação
// ferramenta/agente (paridade D6, ADR-011 Security, Bloco 2).
//
// Hoje a camada `internal/adapter` retorna `(*T, error)`: um error Go nativo não
// carrega `degraded`/`exitCode`, e `nil` parece sucesso sem sinalizar que a
// operação rodou "degradada". O envelope resolve isso: todo resultado atravessa
// a fronteira como `Result`, preservando sucesso, dados, degradação e código de
// saída de forma estruturada.
//
// INVARIANTE (paridade D6): `Success` é SEMPRE derivado de `exitCode == 0`.
//
//	OK(data)       → Success=true,  ExitCode=0
//	Fail(n, ...)   → Success=false, ExitCode=n (n != 0)
//	Degraded(...)  → Success=true,  ExitCode=0, Degraded=true
//
// `Success=true` + `Degraded=true` é um estado válido: "funcionou, mas
// degradado" — não é um error, e `WasError()`/`Err()` o tratam como sucesso.
package results

import "fmt"

// Result é o envelope uniforme de resultado (paridade D6).
type Result struct {
	// Success reporta se a operação foi bem-sucedida. SEMPRE igual a
	// exitCode == 0 (paridade D6).
	Success bool `json:"success"`
	// Data é o payload de sucesso (omitido quando vazio).
	Data any `json:"data,omitempty"`
	// Degraded sinaliza que a operação funcionou, mas em modo degradado
	// (ex.: fallback de provedor, serviço parcial). Válido apenas com
	// Success=true e ExitCode=0.
	Degraded bool `json:"degraded"`
	// ExitCode é o código de saída da operação. 0 = sucesso; não-zero = falha.
	ExitCode int `json:"exitCode"`
	// Reason é a explicação humana (motivo de falha ou de degradação).
	Reason string `json:"reason,omitempty"`
}

// OK constrói um resultado bem-sucedido com exitCode 0 e o payload informado.
func OK(data any) Result {
	return Result{Success: true, Data: data, ExitCode: 0}
}

// Fail constrói um resultado fracassado com um código de saída não-zero.
//
// Para preservar o invariante (Success == exitCode == 0), um exitCode 0 é
// normalizado para 1: uma falha jamais deve parecer sucesso (code 0) na borda.
func Fail(exitCode int, reason string) Result {
	if exitCode == 0 {
		exitCode = 1
	}
	return Result{Success: false, ExitCode: exitCode, Reason: reason}
}

// Degraded constrói um resultado que "funcionou, mas degradado": Success=true,
// Degraded=true e exitCode 0. Não é um error — a operação teve efeito.
func Degraded(reason string) Result {
	return Result{Success: true, Degraded: true, ExitCode: 0, Reason: reason}
}

// WasError reporta se o resultado representa uma falha (Success=false).
func (r Result) WasError() bool {
	return !r.Success
}

// Err converte o resultado em um error para a fronteira. Retorna nil quando o
// resultado é bem-sucedido — inclusive degradado (Success=true). Como um error
// Go nativo não carrega exitCode/degraded, este é o ponto de conversão para
// assinaturas `(*T, error)` legadas (o envelope é aditivo, não substitui).
func (r Result) Err() error {
	if r.Success {
		return nil
	}
	return &ResultError{Result: r}
}

// ResultError é um error que preserva o envelope Result original, permitindo
// resgatar exitCode/reason na fronteira sem análise de string.
type ResultError struct {
	Result Result
}

// Error implementa error (fronteira): o erro Go perde a estrutura, então
// codifica exitCode e reason no texto.
func (e *ResultError) Error() string {
	code := e.Result.ExitCode
	reason := e.Result.Reason
	if reason == "" {
		reason = "falha desconhecida"
	}
	return fmt.Sprintf("results: exitCode %d: %s", code, reason)
}
