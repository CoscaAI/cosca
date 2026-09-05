// Package adapter — envelope uniforme na fronteira (paridade D6, ADR-011
// Security, Bloco 2).
//
// As assinaturas legadas dos adapters retornam `(*T, error)`. Um error Go
// nativo não carrega `degraded`/`exitCode`, e `nil` parece sucesso sem
// sinalizar que a operação rodou "degradada". Este arquivo expõe uma conversão
// **aditiva** para o envelope `results.Result`: os métodos novos (sufixo
// `Result`) retornam `*results.Result`, e as funções `toResult`/`fromResult`
// fazem a ponte na fronteira sem duplicar lógica.
//
// INVARIANTE (paridade D6): `Success` é SEMPRE `exitCode == 0`. `Degraded=true`
// apenas com `Success=true` e `exitCode=0` ("funcionou, mas degradado").
package adapter

import (
	"errors"
	"fmt"

	"github.com/CoscaAI/cosca/internal/results"
)

// adapterResult converte o resultado de uma operação `(T, error)` no envelope.
//	- err != nil          → results.Fail(exitCode 1, err.Error())
//	- degradedReason != "" → results.Degraded(reason) com Data preservado
//	- caso contrário      → results.OK(data)
func adapterResult[T any](data T, err error, degradedReason string) *results.Result {
	if err != nil {
		r := results.Fail(1, err.Error())
		return &r
	}
	if degradedReason != "" {
		r := results.Degraded(degradedReason)
		r.Data = data
		return &r
	}
	r := results.OK(data)
	return &r
}

// toResult é a forma pública de converter `(T, error)` → envelope. Delega para
// adapterResult com degradação vazia (comum para consumidores que não têm um
// motivo de degradação a declarar).
func toResult[T any](data T, err error) *results.Result {
	return adapterResult(data, err, "")
}

// fromResult converte um envelope `results.Result` de volta para `(T, error)`,
// a assinatura legada que continua compilando. Um resultado degradado
// (Success=true, Degraded=true) NÃO é error — volta como (data, nil), pois a
// operação teve efeito.
func fromResult[T any](r *results.Result, zero T) (T, error) {
	if r == nil {
		return zero, errors.New("results: nil result")
	}
	if r.WasError() {
		return zero, r.Err()
	}
	data, ok := r.Data.(T)
	if !ok {
		// Payload ausente (Data nil) é aceitável para tipos de valor/slice
		// vazios: retorna zero sem erro.
		return zero, nil
	}
	return data, nil
}

// toResultError é um atalho para construir um envelope de falha a partir de um
// error simples em contextos onde não há payload.
func toResultError(err error) *results.Result {
	if err == nil {
		r := results.OK(nil)
		return &r
	}
	r := results.Fail(1, err.Error())
	return &r
}

// degradeResult é um atalho para marcar um payload como degradado (útil em
// testes e em pontos que querem apenas declarar o motivo).
func degradeResult[T any](data T, reason string) *results.Result {
	return adapterResult(data, nil, reason)
}

// describeResultError devolve uma descrição estável de um result para logging
// (ex.: "degraded: motivo" / "ok" / "exitCode 3: motivo").
func describeResult(r *results.Result) string {
	if r == nil {
		return "nil"
	}
	if r.WasError() {
		return fmt.Sprintf("exitCode %d: %s", r.ExitCode, r.Reason)
	}
	if r.Degraded {
		return "degraded: " + r.Reason
	}
	return "ok"
}
