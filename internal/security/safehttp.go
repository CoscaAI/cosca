// safehttp.go — Cliente HTTP com revalidação anti-SSRF hop-by-hop.
//
// PRINCÍPIO minerado (Osiris, MIT — src/lib/ssrf-guard.ts) reimplementado segundo
// os contratos do COSCA. Ver ADR-041. NENHUM código do repositório-fonte foi
// importado; apenas a ideia foi portada para uma primitiva Go idiomática.
//
// O que isto fecha: um fetch seguro não é só "a URL inicial é pública". O caminho
// real é URL → DNS → IP → redirect → DNS → IP → destino final. Se o redirect for
// seguido automaticamente e o destino for privado, um guard que validou apenas a
// primeira URL não protege o destino final. NewSafeClient devolve um http.Client
// cuja CheckRedirect revalida CADA salto (hostname de metadata + re-resolução +
// todas as IPs públicas), falha-fechado, com teto de redirects e só http/https.
package security

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"time"
)

// NewSafeClient devolve um http.Client que revalida cada redirect contra SSRF.
// A validação da URL INICIAL permanece a cargo do chamador (ex.: handleWeb, que
// já resolve e valida antes do dial) — este client garante os SALTOS seguintes.
//
// timeout: limite global do client. maxRedirects: teto de saltos permitidos.
// Ambos com default aplicado quando <= 0.
func NewSafeClient(timeout time.Duration, maxRedirects int) *http.Client {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	if maxRedirects <= 0 {
		maxRedirects = 3
	}
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("ssrf guard: redirect excedeu %d saltos", maxRedirects)
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("ssrf guard: redirect para protocolo %q bloqueado", req.URL.Scheme)
			}
			return validateURLHost(req.URL.Hostname())
		},
	}
}

// validateURLHost valida o host de um alvo de rede contra SSRF: rejeita hostname
// de metadata por nome, re-resolve o host e valida TODAS as IPs como públicas.
// Fail-closed: qualquer falha de DNS/validação bloqueia o alvo.
func validateURLHost(host string) error {
	if err := CheckHostname(host); err != nil {
		return err
	}
	ips, err := net.DefaultResolver.LookupIP(context.Background(), "ip", host)
	if err != nil || len(ips) == 0 {
		return fmt.Errorf("ssrf guard: falha ao resolver host %q: %w", host, err)
	}
	for _, ip := range ips {
		addr, ok := netip.AddrFromSlice(ip)
		if !ok {
			return fmt.Errorf("ssrf guard: IP %v inválido", ip)
		}
		if err := ValidatePublicIP(addr); err != nil {
			return err
		}
	}
	return nil
}
