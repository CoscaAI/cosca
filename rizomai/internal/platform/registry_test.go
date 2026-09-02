// Testes da fábrica de conectores (registry) — 13 plataformas.
package platform

import (
	"testing"

	"github.com/rizomai/rizomai/internal/domain"
)

func TestRegistryPublisher(t *testing.T) {
	reg := NewRegistry(Config{})

	all := []struct {
		p    domain.Platform
		name string
	}{
		{domain.PlatformX, "x"},
		{domain.PlatformLinkedIn, "linkedin"},
		{domain.PlatformTelegram, "telegram"},
		{domain.PlatformInstagram, "instagram"},
		{domain.PlatformFacebook, "facebook"},
		{domain.PlatformThreads, "threads"},
		{domain.PlatformYouTube, "youtube"},
		{domain.PlatformTikTok, "tiktok"},
		{domain.PlatformBluesky, "bluesky"},
		{domain.PlatformReddit, "reddit"},
		{domain.PlatformPinterest, "pinterest"},
		{domain.PlatformSnapchat, "snapchat"},
		{domain.PlatformGoogleBusiness, "googlebusiness"},
	}

	if len(all) != 13 {
		t.Fatalf("catálogo deveria ter 13 plataformas, tem %d", len(all))
	}
	for _, c := range all {
		pub, err := reg.Publisher(c.p)
		if err != nil {
			t.Fatalf("Publisher(%s): %v", c.p, err)
		}
		if pub.Name() != c.name {
			t.Errorf("Publisher(%s).Name() = %q, esperado %q", c.p, pub.Name(), c.name)
		}
	}

	if _, err := reg.Publisher("whatsapp"); err == nil {
		t.Error("plataforma desconhecida deveria retornar erro")
	}
}

func TestRegistryOAuth(t *testing.T) {
	reg := NewRegistry(Config{})

	// OAuth de navegador: todas exceto telegram/bluesky/reddit.
	for _, p := range []domain.Platform{
		domain.PlatformX, domain.PlatformLinkedIn, domain.PlatformInstagram,
		domain.PlatformFacebook, domain.PlatformThreads, domain.PlatformYouTube,
		domain.PlatformTikTok, domain.PlatformPinterest, domain.PlatformSnapchat,
		domain.PlatformGoogleBusiness,
	} {
		oa, err := reg.OAuth(p)
		if err != nil {
			t.Fatalf("OAuth(%s): %v", p, err)
		}
		if oa == nil {
			t.Fatalf("OAuth(%s) = nil", p)
		}
	}

	// Sem browser OAuth (credentials).
	for _, p := range []domain.Platform{domain.PlatformTelegram, domain.PlatformBluesky, domain.PlatformReddit} {
		if _, err := reg.OAuth(p); err == nil {
			t.Errorf("%s deveria retornar erro em OAuth() (usam credentials)", p)
		}
	}
}

func TestCredentialPlatforms(t *testing.T) {
	got := CredentialPlatforms()
	if len(got) != 3 {
		t.Fatalf("esperava 3 plataformas de credentials, veio %d", len(got))
	}
}
