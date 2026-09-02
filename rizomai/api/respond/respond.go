// Package respond centraliza a escrita de respostas HTTP conforme o contrato
// (ADR-005): envelope de sucesso {data}, paginação {data,page,limit,total} e
// envelope de erro {code,error,details}. Pacote folha (só stdlib) para ser
// usado por handlers e middlewares sem risco de ciclo de imports.
package respond

import (
	"encoding/json"
	"net/http"
)

// Data escreve 200/2xx com envelope { "data": ... }.
func Data(w http.ResponseWriter, status int, data any) {
	write(w, status, map[string]any{"data": data})
}

// List escreve o envelope paginado { data, page, limit, total }.
func List(w http.ResponseWriter, status int, data any, page, limit, total int) {
	write(w, status, map[string]any{
		"data":  data,
		"page":  page,
		"limit": limit,
		"total": total,
	})
}

// Error escreve o envelope de erro { code, error, details } (todo 4xx/5xx).
func Error(w http.ResponseWriter, status int, code, msg string, details any) {
	if details == nil {
		details = map[string]any{}
	}
	write(w, status, map[string]any{
		"code":    code,
		"error":   msg,
		"details": details,
	})
}

func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
