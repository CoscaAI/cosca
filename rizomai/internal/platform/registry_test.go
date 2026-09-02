// Testes da fábrica de conectores (registry).
package platform

import (
	"testing"

	"github.com/rizomai/rizomai/internal/domain"
)

func TestRegistryPublisher(t *testing.T) {
	reg := NewRegistry(Config{})

	want := map[domain.Platform]string{
		domain.PlatformX:        "x",
		domain.PlatformLinkedIn: "linkedin",
		domain.PlatformTelegram: "telegram",
	}
	for p, name := range want {
		pub, err := reg.Publisher(p)
		if err != nil {
			t.Fatalf("Publisher(%s): %v", p, err)
		}
		if pub.Name() != name {
			t.Errorf("Publisher(%s).Name() = %q, esperado %q", p, pub.Name(), name)
		}
	}

	if _, err := reg.Publisher("instagram"); err == nil {
		t.Error("plataforma desconhecida deveria retornar erro")
	}
}

func TestRegistryOAuth(t *testing.T) {
	reg := NewRegistry(Config{})

	for _, p := range []domain.Platform{domain.PlatformX, domain.PlatformLinkedIn} {
		oa, err := reg.OAuth(p)
		if err != nil {
			t.Fatalf("OAuth(%s): %v", p, err)
		}
		if oa == nil {
			t.Fatalf("OAuth(%s) = nil", p)
		}
	}

	// Telegram não usa OAuth (bot token).
	if _, err := reg.OAuth(domain.PlatformTelegram); err == nil {
		t.Error("Telegram deveria retornar erro em OAuth() (sem OAuth)")
	}
}
