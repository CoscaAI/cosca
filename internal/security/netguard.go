// netguard.go — Guard de REDE fail-closed (anti-SSRF) para o COSCA.
//
// Diamante minerado do PinchTab (internal/netguard) + lição do professor: um
// orquestrador que deixa agentes navegarem/fazerem fetch precisa de defesa em
// camadas contra SSRF — impedir que um agente acesse a rede interna (metadata
// service, loopback, link-local, CGNAT) de um bucket público.
//
// Primitivas reutilizáveis (determinísticas, stdlib-only):
//   - IsPublicIP:         true se o IP é público (fora de private/loopback/
//                         link-local/multicast/unspecified/CGNAT/benchmark).
//   - DecodeEmbeddedIPv4: decodifica IPv4 embutido em IPv6 de transição
//                         (6to4 2002::/16 e NAT64 64:ff9b::/96) — o detalhe que
//                         fecha SSRF via cloud metadata service, sem banir
//                         IPv4 público legítimo.
//   - ValidatePublicIP:   valida um IP de host contra SSRF (endpoint de URL).
//
// Fail-closed: um IP que NÃO é comprovadamente público é tratado como privado
// (rejeitado) — nunca "talvez seja público".
package security

import (
	"fmt"
	"net/netip"
)

// blockedPrefixes são faixas que o netip.IsPrivate/IsLoopback/IsLinkLocal/...
// NÃO cobrem e que representam rede interna/ambiguidade (CGNAT + benchmark).
// Banir por prefixo é o equivalente do PinchTab netguard.blockedPrefixes.
var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"), // CGNAT (RFC 6598)
	netip.MustParsePrefix("198.18.0.0/15"), // benchmark (RFC 2544)
}

// IsPublicIP devolve true se addr é um endereço IP comprovadamente público.
// Fail-closed: qualquer faixa não-pública conhecida → false. Endereços IPv6 de
// transição (6to4/NAT64) são DECODIFICADOS (não banidos) para não quebrar
// IPv4 público legítimo embutido — mas o IP público resultante é revalidado.
func IsPublicIP(addr netip.Addr) bool {
	// Decodifica IPv4 embutido em IPv6 de transição (6to4/NAT64). Um maper
	// não é "privado" por si só; decodificamos para ver o alvo real (ssrf).
	if decoded, ok := DecodeEmbeddedIPv4(addr); ok {
		return IsPublicIP(decoded)
	}

	if !addr.IsValid() {
		return false
	}
	// Normaliza IPv4-mapped IPv6 (::ffff:a.b.c.d) para o IPv4 real.
	if addr.Is4In6() {
		addr = addr.Unmap()
	}
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified() {
		return false
	}
	// CGNAT + benchmark (não cobertos por netip.IsPrivate).
	for _, p := range blockedPrefixes {
		if p.Contains(addr) {
			return false
		}
	}
	return true
}

// DecodeEmbeddedIPv4 decodifica um IPv4 embutido em um endereço IPv6 de
// transição (6to4 2002::/16 ou NAT64 64:ff9b::/96). Devolve (addr, true).
// Endereços não-de-transição → (zero, false). O motivo de decodificar (em vez
// de banir o prefixo): 64:ff9b::a9fe:a9fe é o cloud metadata service; como o
// wrapper é global-scoped, IsPrivate/IsLoopback não pegam — mas o ALVO real é
// privado. Decodificar permite (a) detectar SSRF e (b) não banir IPv4 público
// legítimo que passe por esses prefixos.
func DecodeEmbeddedIPv4(addr netip.Addr) (netip.Addr, bool) {
	if !addr.IsValid() {
		return netip.Addr{}, false
	}
	b := addr.As16()

	// 6to4: 2002:VVWW:XXYY::/48 → IPv4 = VV.WW.XX.YY.
	if b[0] == 0x20 && b[1] == 0x02 {
		return netip.AddrFrom4([4]byte{b[2], b[3], b[4], b[5]}), true
	}
	// NAT64 (well-known prefix 64:ff9b::/96): os últimos 4 bytes são o IPv4.
	if b[0] == 0x00 && b[1] == 0x64 && b[2] == 0xff && b[3] == 0x9b &&
		b[4] == 0 && b[5] == 0 && b[6] == 0 && b[7] == 0 &&
		b[8] == 0 && b[9] == 0 && b[10] == 0 && b[11] == 0 {
		return netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]}), true
	}
	return netip.Addr{}, false
}

// ValidatePublicIP valida um IP de host contra SSRF. Devolve nil se o IP é
// público; erro descritivo (fail-closed) se for privado/interno/ambiguo.
func ValidatePublicIP(ip netip.Addr) error {
	if !ip.IsValid() {
		return fmt.Errorf("ssrf guard: endereço IP inválido")
	}
	if !IsPublicIP(ip) {
		// Informa o alvo decodificado quando aplicável (torna o bloqueio
		// instrutivo, não silencioso).
		if decoded, ok := DecodeEmbeddedIPv4(ip); ok {
			return fmt.Errorf("ssrf guard: IP %s (IPv4 embutido em transição, alvo %s) não é público — acesso à rede interna bloqueado", ip, decoded)
		}
		return fmt.Errorf("ssrf guard: IP %s não é público — acesso à rede interna/metadata bloqueado", ip)
	}
	return nil
}
