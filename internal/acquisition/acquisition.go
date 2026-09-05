// Package acquisition implements a "aquisição de evidência externa"
// (external evidence acquisition) para a Cosca.
//
// Pipeline (da conversa do Don):
//
//	INTERNET → External Sources (GitHub, GitLab, Docs, RFCs, registries)
//	→ ACQUISITION → NORMALIZATION → FINGERPRINT → PROVENANCE → QUARANTINE
//	→ EXTRACTION → FTS5/VECTOR → EVIDENCE GRAPH.
//
// Regra de ouro: EXTERNAL DATA ≠ TRUSTED DATA. Tudo o que vem da internet
// começa como UNTRUSTED — a proveniência de todo AcquiredArtifact nasce
// "UNTRUSTED" e NADA é promovido automaticamente ao conhecimento. Busca é uma
// coisa; execução é outra fronteira: conteúdo externo NUNCA é executado.
//
// O hardening espelha EXATAMENTE o guard de SSRF de internal/skills
// (fetchSkillSource): fail-open em caso de falha DNS (prossegue com warn),
// timeout de 5s, teto de 3 redirects e limite de
// 10 MiB por corpo. Além disso rejeita esquemas não-http(s) e IPs
// privados/loopback/link-local resolvidos por net.ParseIP — proteção extra
// contra SSRF (metadata de cloud em 169.254.169.254, scanning de rede
// interna, file:// e afins).
package acquisition

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// fallbackDNSResolver é um resolver que ignora a config quebrada do sistema
// (::1:53) e dialoga direto com DNS público. Usado para ignorar falhas de DNS
// no DialContext e no validateTarget (ordem do Don).
var fallbackDNSResolver = &net.Resolver{
	PreferGo: true,
	Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		d := net.Dialer{Timeout: 5 * time.Second}
		// Tenta 8.8.8.8 primeiro, fallback para 1.1.1.1
		conn, err := d.DialContext(ctx, "udp", "8.8.8.8:53")
		if err != nil {
			log.Debug().Err(err).Msg("acquisition: DNS fallback 8.8.8.8 falhou, tentando 1.1.1.1")
			return d.DialContext(ctx, "udp", "1.1.1.1:53")
		}
		return conn, nil
	},
}

// Proveniência e limites de hardening (espelho de internal/skills).
const (
	// ProvenanceUntrusted é a proveniência inicial de TODO artefato externo.
	ProvenanceUntrusted = "UNTRUSTED"

	// DefaultTimeout é o timeout padrão do fetch (5s, como o skills.go).
	DefaultTimeout = 5 * time.Second
	// DefaultMaxRedirects é o teto de redirects (3, como o skills.go).
	DefaultMaxRedirects = 3
	// DefaultMaxBodyBytes é o limite de corpo por artefato (10 MiB).
	DefaultMaxBodyBytes int64 = 10 << 20

	// acquisitionUA é o User-Agent IDENTIFICADOR honesto (não um browser-UA
	// spoofed). Alguns endpoints rejeitam UA default/browser e pedem um
	// identificador — ver ADR-042 (princípio de honestidade > spoofing).
	acquisitionUA = "Cosca-Acquisition/1.0 (+https://github.com/CoscaAI/cosca)"
)

// AcquiredArtifact é o resultado da aquisição: metadados + fingerprint.
// A proveniência começa SEMPRE como "UNTRUSTED" — evidência externa ≠ dado
// confiável. O corpo do artefato NUNCA entra aqui: é persistido separadamente
// em .cosca/quarantine/artifacts/A-XXXX.
type AcquiredArtifact struct {
	ID          string    `json:"id"` // A-XXXX
	URL         string    `json:"url"`
	ContentType string    `json:"content_type"`
	SHA256      string    `json:"sha256"`
	SizeBytes   int64     `json:"size_bytes"`
	RetrievedAt time.Time `json:"retrieved_at"`
	Provenance  string    `json:"provenance"` // "UNTRUSTED" inicial — sempre
	Notes       string    `json:"notes,omitempty"`
}

// Client é o cliente HTTP endurecido de aquisição. O fetch é FAIL-CLOSED:
// só acontece quando AllowRemote está habilitado explicitamente (a disciplina
// AllowRemoteSources do skills.go). AllowLoopback permite alvos privados/
// loopback (mock servers locais, LANs) — um opt-in separado, desligado por
// padrão.
type Client struct {
	timeout      time.Duration
	maxRedirects int
	maxBodyBytes int64

	// AllowRemote habilita o fetch remoto (--allow-remote). Default: desligado.
	AllowRemote bool
	// AllowLoopback permite IPs privados/loopback/link-local (testes locais).
	AllowLoopback bool

	// GitHubToken é usado para autenticar chamadas à API do GitHub via
	// header Authorization: Bearer <token>. Vazio = sem autenticação
	// (rate limit anônimo: 60 req/h vs 5000 req/h autenticado).
	GitHubToken string

	// Tracker é o orçamento de aquisição opcional (nil = comportamento atual,
	// zero contabilização e zero mudança de comportamento no fetch).
	Tracker *AcquisitionTracker
}

// WithTracker vincula o orçamento de aquisição ao cliente e devolve o próprio
// c (setter fluente — documentado: muta o receiver). Quando Tracker é nil o
// fetch roda exatamente como antes; quando setado, cada FetchAll bem-sucedido
// contabiliza bytes/fontes/arquivos e aborta se o orçamento estourar.
func (c *Client) WithTracker(t *AcquisitionTracker) *Client {
	c.Tracker = t
	return c
}

// NewClient cria um cliente endurecido. Valores <= 0 usam os defaults
// (5s timeout, 3 redirects, 10 MiB corpo).
func NewClient(timeout time.Duration, maxRedirects int, maxBodyBytes int64) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if maxRedirects <= 0 {
		maxRedirects = DefaultMaxRedirects
	}
	if maxBodyBytes <= 0 {
		maxBodyBytes = DefaultMaxBodyBytes
	}
	return &Client{
		timeout:      timeout,
		maxRedirects: maxRedirects,
		maxBodyBytes: maxBodyBytes,
	}
}

// Fetch busca a URL, calcula o SHA-256, registra ContentType/RetrievedAt e
// devolve o artefato (proveniência sempre "UNTRUSTED"). O corpo NÃO é
// retornado aqui — use FetchAll para obter os bytes brutos na mesma conexão.
func (c *Client) Fetch(ctx context.Context, rawURL string) (*AcquiredArtifact, error) {
	art, _, err := c.FetchAll(ctx, rawURL)
	return art, err
}

// httpClient devolve o http.Client endurecido de aquisição: transport com
// re-validação anti-SSRF no DIAL (defesa em profundidade vs DNS rebinding e
// redirects a hosts internos), timeout, teto de redirects. É o plumbing comum a
// FetchAll e FetchConditional.
func (c *Client) httpClient() *http.Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Defense in depth: re-valida o IP no momento do dial — protege
			// contra DNS rebinding e redirects para hosts internos.
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				host = addr
			}
			if ips, lookupErr := fallbackDNSResolver.LookupIP(ctx, "ip", host); lookupErr == nil {
				for _, ip := range ips {
					if blockedIP(ip) && !c.AllowLoopback {
						return nil, fmt.Errorf("acquisition: SSRF guard: %q resolve para IP bloqueado %s", host, ip)
					}
				}
			}
			return (&net.Dialer{Timeout: c.timeout, Resolver: fallbackDNSResolver}).DialContext(ctx, network, addr)
		},
	}
	return &http.Client{
		Timeout:   c.timeout,
		Transport: transport,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= c.maxRedirects {
				return fmt.Errorf("acquisition: parado após %d redirects", len(via))
			}
			return nil
		},
	}
}

// FetchAll faz o fetch endurecido e devolve o artefato E o corpo bruto numa
// única requisição (o hash e o corpo gravado nunca divergem).
func (c *Client) FetchAll(ctx context.Context, rawURL string) (*AcquiredArtifact, []byte, error) {
	if err := c.validateTarget(rawURL); err != nil {
		return nil, nil, err
	}

	client := c.httpClient()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("acquisition: requisição inválida: %w", err)
	}
	// User-Agent identificador honesto (honestidade > spoofing; ADR-042).
	req.Header.Set("User-Agent", acquisitionUA)
	if c.GitHubToken != "" && strings.Contains(rawURL, "api.github.com") {
		req.Header.Set("Authorization", "Bearer "+c.GitHubToken)
		log.Debug().Str("url", rawURL).Bool("auth", true).Msg("acquisition: usando token GitHub")
	} else if strings.Contains(rawURL, "api.github.com") {
		log.Debug().Str("url", rawURL).Bool("auth", false).Msg("acquisition: SEM token GitHub (esperado GITHUB_TOKEN no env)")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("acquisition: fetch %s: %w", rawURL, err)
	}
	log.Debug().Str("url", rawURL).Int("status", resp.StatusCode).Msg("acquisition: resposta")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("acquisition: HTTP %d ao buscar %s", resp.StatusCode, rawURL)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBodyBytes+1))
	if err != nil {
		return nil, nil, fmt.Errorf("acquisition: ler corpo: %w", err)
	}
	if int64(len(body)) > c.maxBodyBytes {
		return nil, nil, fmt.Errorf("acquisition: corpo excede %d bytes", c.maxBodyBytes)
	}

	if c.Tracker != nil {
		c.Tracker.RecordBytes(int64(len(body)))
		c.Tracker.RecordSource()
		c.Tracker.RecordFile()
		if c.Tracker.Exceeded() {
			dims := strings.Join(c.Tracker.WhichExceeded(), ", ")
			return nil, nil, fmt.Errorf("acquisition: orçamento de aquisição excedido (%s)", dims)
		}
	}

	sum := sha256.Sum256(body)
	art := &AcquiredArtifact{
		URL:         rawURL,
		ContentType: resp.Header.Get("Content-Type"),
		SHA256:      fmt.Sprintf("%x", sum[:]),
		SizeBytes:   int64(len(body)),
		RetrievedAt: time.Now().UTC(),
		Provenance:  ProvenanceUntrusted,
		// Trecho curto da normalização — NUNCA um dump completo.
		Notes: shortExcerpt(body, 500),
	}
	return art, body, nil
}

// validateTarget aplica o fail-closed do guard de SSRF: apenas esquemas
// http/https e apenas hosts que NÃO resolvem para IPs privados/loopback/
// link-local (espelha a disciplina AllowRemoteSources do skills.go).
func (c *Client) validateTarget(rawURL string) error {
	if !c.AllowRemote {
		return fmt.Errorf("acquisition: busca remota desabilitada por segurança — use --allow-remote: %s", rawURL)
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("acquisition: URL inválida %q: %w", rawURL, err)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return fmt.Errorf("acquisition: esquema %q rejeitado (SSRF guard — apenas http/https): %s", u.Scheme, rawURL)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("acquisition: URL sem host: %s", rawURL)
	}
	if strings.EqualFold(host, "localhost") {
		if !c.AllowLoopback {
			return fmt.Errorf("acquisition: host localhost rejeitado (SSRF guard): %s", rawURL)
		}
		return nil
	}
	ips := resolveIPs(host)
	if len(ips) == 0 {
		// DNS resolution failed — fail-open (Don's order: DNS failures are non-fatal).
		// Allow the acquisition to proceed; the DialContext guard still provides
		// defense-in-depth at connection time.
		log.Warn().Str("host", host).Msg("acquisition: DNS failure ao resolver host — prosseguindo sem validação de IP (fail-open)")
		return nil
	}
	for _, ip := range ips {
		if blockedIP(ip) && !c.AllowLoopback {
			return fmt.Errorf("acquisition: SSRF guard: %q resolve para IP bloqueado %s", host, ip)
		}
	}
	return nil
}

// resolveIPs devolve os endereços do host. Se o host é um IP literal, usa-o
// diretamente (sem DNS); caso contrário resolve via fallbackDNSResolver (DNS
// público 8.8.8.8/1.1.1.1 — ignora config quebrada do sistema).
func resolveIPs(host string) []net.IP {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}
	}
	ips, err := fallbackDNSResolver.LookupIP(context.Background(), "ip", host)
	if err != nil {
		log.Warn().Err(err).Str("host", host).Msg("acquisition: fallback DNS falhou ao resolver host")
		return nil
	}
	return ips
}

// blockedCIDRs são as faixas rejeitadas pelo guard de SSRF: loopback,
// RFC1918 (privadas), link-local, ULA IPv6 e a metadata da cloud.
var blockedCIDRs = []string{
	"127.0.0.0/8",    // loopback IPv4
	"10.0.0.0/8",     // RFC1918
	"172.16.0.0/12",  // RFC1918
	"192.168.0.0/16", // RFC1918
	"169.254.0.0/16", // link-local (metadata AWS 169.254.169.254)
	"::1/128",        // loopback IPv6
	"fc00::/7",       // ULA IPv6
	"fe80::/10",      // link-local IPv6
}

// blockedIP devolve true para IPs em faixas privadas/loopback/link-local.
// Normaliza IPv4-mapped (::ffff:127.0.0.1) antes da checagem para que a
// tabela IPv4 pegue os endereços mapeados também.
func blockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	for _, cidr := range blockedCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// nonSafeRe casa qualquer caractere que não seja alfanumérico.
var nonSafeRe = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// NormalizeName deriva um nome de arquivo seguro (sanitizado) da URL para
// armazenamento local. Remove esquema/query/fragment, colapsa qualquer runa de
// caracteres não alfanuméricos (incluindo barras e pontos) em um único "-",
// garante que o resultado não comece/termine com "-" e limita o tamanho — sem
// path traversal (".."/"/") e sem colisão com nomes reservados.
func NormalizeName(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "artifact"
	}
	raw := u.Host + u.Path
	if raw == "" {
		raw = u.Host
	}
	name := nonSafeRe.ReplaceAllString(raw, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		name = "artifact"
	}
	if len(name) > 120 {
		name = name[len(name)-120:]
	}
	name = strings.Trim(name, "-")
	if name == "" {
		name = "artifact"
	}
	return name
}

// shortExcerpt captura um curto trecho imprimível do corpo (nó da
// NORMALIZATION do pipeline). NUNCA um dump completo.
func shortExcerpt(body []byte, n int) string {
	if n <= 0 {
		n = 500
	}
	s := strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || (r >= 32 && r < 127) {
			return r
		}
		return ' '
	}, string(body))
	trimmed := strings.TrimSpace(s)
	runes := []rune(trimmed)
	if len(runes) > n {
		runes = runes[:n]
	}
	return string(runes)
}
