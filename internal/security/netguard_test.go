package security

import (
	"net/netip"
	"testing"
)

func TestIsPublicIP_PrivateBlocked(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", false},   // loopback
		{"10.0.0.1", false},    // private
		{"192.168.1.10", false}, // private
		{"169.254.169.254", false}, // link-local (cloud metadata)
		{"0.0.0.0", false},     // unspecified
		{"100.64.0.1", false},  // CGNAT
		{"198.18.0.1", false},  // benchmark
		{"::1", false},         // loopback IPv6
		{"8.8.8.8", true},      // público
		{"1.1.1.1", true},      // público
		{"223.5.5.5", true},    // público
	}
	for _, c := range cases {
		got := IsPublicIP(netip.MustParseAddr(c.ip))
		if got != c.want {
			t.Errorf("IsPublicIP(%s) = %v, esperava %v", c.ip, got, c.want)
		}
	}
}

// TestDecodeEmbeddedIPv4_NAT64_SixTo4 valida o diamante anti-SSRF: decodificar
// IPv4 embutido em IPv6 de transição. 64:ff9b::a9fe:a9fe = 169.254.169.254
// (cloud metadata service) — endereço que, sem decodificação, passa como
// "não privado" (o wrapper é global-scoped).
func TestDecodeEmbeddedIPv4_NAT64_SixTo4(t *testing.T) {
	// NAT64 well-known prefix 64:ff9b::/96: 64:ff9b::a9fe:a9fe -> 169.254.169.254
	nat64 := netip.MustParseAddr("64:ff9b::a9fe:a9fe")
	decoded, ok := DecodeEmbeddedIPv4(nat64)
	if !ok {
		t.Fatal("NAT64 deveria decodificar")
	}
	if decoded.String() != "169.254.169.254" {
		t.Fatalf("NAT64 decode = %s, esperava 169.254.169.254", decoded)
	}
	// 6to4: 2002:0808:0808:: -> 8.8.8.8
	six4 := netip.MustParseAddr("2002:0808:0808::")
	if d2, ok2 := DecodeEmbeddedIPv4(six4); !ok2 || d2.String() != "8.8.8.8" {
		t.Fatalf("6to4 decode = %v/%v, esperava 8.8.8.8", d2, ok2)
	}
}

// TestValidatePublicIP_MetadataServiceBlocked é o caso de uso crítico: o COSCA
// deve bloquear um agente de acessar 169.254.169.254 (cloud metadata) mesmo via
// NAT64 — fail-closed e instrutivo.
func TestValidatePublicIP_MetadataServiceBlocked(t *testing.T) {
	err := ValidatePublicIP(netip.MustParseAddr("169.254.169.254"))
	if err == nil {
		t.Fatal("metadata service deveria ser bloqueado")
	}
	// Via NAT64 (o wrapper global-scoped enganaria o IsPrivate).
	err2 := ValidatePublicIP(netip.MustParseAddr("64:ff9b::a9fe:a9fe"))
	if err2 == nil {
		t.Fatal("metadata service via NAT64 deveria ser bloqueado")
	}
	if err2 == nil || err2.Error() == "" {
		t.Fatal("erro deveria ser instrutivo (não vazio)")
	}
}

// TestIsPublicIP_ReservedIPv6Blocked valida os ranges IPv6 reservados que o
// hardening (ADR-041) acrescenta: documentação, site-local e NAT64 local-use.
func TestIsPublicIP_ReservedIPv6Blocked(t *testing.T) {
	cases := []string{
		"2001:db8::1",      // documentação (RFC 3849)
		"fec0::1",          // site-local (deprecado)
		"64:ff9b:1::a9fe:a9fe", // NAT64 local-use (RFC 8215)
	}
	for _, c := range cases {
		if IsPublicIP(netip.MustParseAddr(c)) {
			t.Errorf("IsPublicIP(%s) = true, esperava false (range reservado)", c)
		}
	}
}

// TestCheckHostname valida a blocklist de hostnames de metadata por NOME
// (defesa em profundidade, antes do DNS). Ver ADR-041.
func TestCheckHostname(t *testing.T) {
	blocked := []string{
		"localhost",
		"node.localhost",
		"foo.local",
		"bar.internal",
		"host.docker.internal",
		"metadata.google.internal",
	}
	for _, h := range blocked {
		if err := CheckHostname(h); err == nil {
			t.Errorf("CheckHostname(%q) deveria rejeitar", h)
		}
	}
	allowed := []string{
		"example.com",
		"www.example.com",
		"api.github.com",
		"8.8.8.8",
		"sub.domain.example.org",
	}
	for _, h := range allowed {
		if err := CheckHostname(h); err != nil {
			t.Errorf("CheckHostname(%q) = %v, esperava nil", h, err)
		}
	}
}
