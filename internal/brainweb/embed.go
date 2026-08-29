package brainweb

import "embed"

// WebFS embute os assets estáticos do visualizador (index.html, JS, CSS,
// three.js self-hostado). Self-hostado (não CDN) por decisão de segurança:
// a CSP do servidor REST usa script-src 'self' — um CDN externo seria bloqueado.
//
//go:embed web
var WebFS embed.FS
