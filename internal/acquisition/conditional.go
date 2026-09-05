// conditional.go — GET condicional (ETag/Last-Modified) para aquisição.
//
// PRINCÍPIO minerado (Osiris, MIT — src/lib/httpJson.ts) reimplementado segundo
// os contratos do COSCA. Ver ADR-042. NENHUM código do repositório-fonte foi
// importado; apenas a ideia foi portada para uma primitiva Go idiomática.
//
// Valor: re-baixar um dump grande que mudou pouco, num poll rápido, para
// descobrir que é byte-idêntico, é o caminho caro de aprender nada. O replay de
// ETag/Last-Modified (If-None-Match / If-Modified-Since) faz o upstream
// responder 304 com corpo vazio — o caso comum vira centenas de bytes. Além
// disso, o conteúdo é decodificado PELO HEADER Content-Encoding (gzip/deflate),
// e um User-Agent identificador honesto é enviado (não um browser-UA spoofed).
package acquisition

import (
	"compress/gzip"
	"compress/zlib"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Validators são os validadores de cache de um recurso (replay no next poll).
type Validators struct {
	ETag         string `json:"etag,omitempty"`
	LastModified string `json:"last_modified,omitempty"`
}

// ConditionalResult é o resultado de um GET condicional.
type ConditionalResult struct {
	// Changed é false quando o upstream respondeu 304 — a cópia local ainda é
	// a corrente. Nesse caso Body é nil.
	Changed bool
	// Body é o corpo decodificado (não-nil apenas quando Changed é true).
	Body []byte
	// Validators são os validadores a guardar para o próximo poll.
	Validators Validators
}

// FetchConditional faz um GET condicional via o cliente endurecido de aquisição
// (SSRF no dial, timeout, teto de redirects, limite de corpo). Replay de EL
// validators v (ETag/Last-Modified) → upstream responde 304 (Changed=false,
// Body=nil, sem erro) quando nada mudou. Em 200, decodifica o corpo pelo
// Content-Encoding e devolve os validators atualizados.
func (c *Client) FetchConditional(ctx context.Context, rawURL string, v Validators) (ConditionalResult, error) {
	if err := c.validateTarget(rawURL); err != nil {
		return ConditionalResult{}, err
	}

	client := c.httpClient()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return ConditionalResult{}, fmt.Errorf("acquisition: requisição inválida: %w", err)
	}
	// User-Agent identificador honesto (honestidade > spoofing; ADR-042).
	req.Header.Set("User-Agent", acquisitionUA)
	if v.ETag != "" {
		req.Header.Set("If-None-Match", v.ETag)
	}
	if v.LastModified != "" {
		req.Header.Set("If-Modified-Since", v.LastModified)
	}

	resp, err := client.Do(req)
	if err != nil {
		return ConditionalResult{}, fmt.Errorf("acquisition: fetch condicional %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	// 304 = resposta VÁLIDA a um request condicional (não um erro). A cópia
	// local continua corrente.
	if resp.StatusCode == http.StatusNotModified {
		return ConditionalResult{
			Changed:   false,
			Body:      nil,
			Validators: validatorsFrom(resp.Header, v),
		}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return ConditionalResult{}, fmt.Errorf("acquisition: HTTP %d ao buscar %s", resp.StatusCode, rawURL)
	}

	body, err := c.readDecodedBody(resp.Body, resp.Header.Get("Content-Encoding"))
	if err != nil {
		return ConditionalResult{}, fmt.Errorf("acquisition: ler corpo: %w", err)
	}
	if int64(len(body)) > c.maxBodyBytes {
		return ConditionalResult{}, fmt.Errorf("acquisition: corpo excede %d bytes", c.maxBodyBytes)
	}

	if c.Tracker != nil {
		c.Tracker.RecordBytes(int64(len(body)))
		c.Tracker.RecordSource()
		c.Tracker.RecordFile()
		if c.Tracker.Exceeded() {
			dims := strings.Join(c.Tracker.WhichExceeded(), ", ")
			return ConditionalResult{}, fmt.Errorf("acquisition: orçamento de aquisição excedido (%s)", dims)
		}
	}

	return ConditionalResult{
		Changed:    true,
		Body:       body,
		Validators: validatorsFrom(resp.Header, v),
	}, nil
}

// validatorsFrom extrai os validators da resposta, caindo de volta para os
// anteriores quando o header não traz um novo (upstream que ignora validators
// ainda funciona — degrada para GET comum com grace).
func validatorsFrom(h http.Header, prev Validators) Validators {
	next := prev
	if etag := h.Get("ETag"); etag != "" {
		next.ETag = etag
	}
	if lm := h.Get("Last-Modified"); lm != "" {
		next.LastModified = lm
	}
	return next
}

// readDecodedBody lê o corpo e decodifica pelo Content-Encoding do HEADER (não
// pelo que foi pedido) — alguns hosts servem JSON pré-comprimido e setam o
// encoding independente do Accept-Encoding. Suporta gzip e deflate (stdlib);
// encodings desconhecidos passam como raw (sem dependência brotli).
func (c *Client) readDecodedBody(body io.Reader, contentEncoding string) ([]byte, error) {
	var r io.Reader = body
	switch strings.ToLower(strings.TrimSpace(contentEncoding)) {
	case "gzip":
		gz, err := gzip.NewReader(body)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		r = gz
	case "deflate":
		zl, err := zlib.NewReader(body)
		if err != nil {
			return nil, err
		}
		defer zl.Close()
		r = zl
	}
	return io.ReadAll(io.LimitReader(r, c.maxBodyBytes+1))
}
