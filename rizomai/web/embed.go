// Package web embute o dashboard do RIZOMAI (Fase 4) e o serve pelo próprio
// gateway HTTP em "/" — single-binary (ADR-001/ADR-004): zero infra extra,
// sem build de frontend, combina com o monorepo enxuto.
//
// O dashboard é HTML/CSS/JS vanilla (sem framework). O contrato consumido é
// o mesmo openapi/rizomai.yaml (ADR-005: a spec é a fonte da verdade).
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed *
var assets embed.FS

// Handler serve os assets do dashboard com fallback SPA para index.html:
// assets existentes (/css, /js) são servidos direto do filesystem embutido;
// qualquer outro path cai no index.html (hash routing do JS — funciona mesmo
// em navegação direta por URL).
//
// O fallback NÃO passa pelo http.FileServer: o Go redireciona qualquer path
// que termine em "/index.html" para "./" (loop de redirect). Lemos os bytes
// do FS embutido e escrevemos direto.
func Handler() http.Handler {
	sub, err := fs.Sub(assets, ".")
	if err != nil {
		panic(err) // embed quebrado — falha rápido no boot
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Defesa extra: o mux da API já roteia /v1/* e /healthz para os
		// handlers certos antes deste "/" (ServeMux: padrão mais específico
		// vence). Este guard evita vazar assets caso o Handler seja montado
		// em outro lugar por engano.
		if strings.HasPrefix(r.URL.Path, "/v1/") || r.URL.Path == "/healthz" {
			http.NotFound(w, r)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(sub, path); err != nil {
			// Fallback SPA: qualquer rota desconhecida serve o app.
			b, rerr := assets.ReadFile("index.html")
			if rerr != nil {
				http.Error(w, "index.html não encontrado no embed", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(b)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
