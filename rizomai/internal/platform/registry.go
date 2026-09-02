package platform

import (
	"fmt"

	"github.com/rizomai/rizomai/internal/domain"
	"github.com/rizomai/rizomai/internal/platform/linkedin"
	"github.com/rizomai/rizomai/internal/platform/telegram"
	"github.com/rizomai/rizomai/internal/platform/x"
)

// Config reúne as credenciais de APP (via env — nunca em código).
type Config struct {
	X        x.Config
	LinkedIn linkedin.Config
	Telegram telegram.Config
}

// Registry é a fábrica de publishers por plataforma.
type Registry struct {
	x        *x.Client
	linkedin *linkedin.Client
	telegram *telegram.Client
}

// NewRegistry instancia os conectores com as credenciais de app.
func NewRegistry(cfg Config) *Registry {
	return &Registry{
		x:        x.New(cfg.X),
		linkedin: linkedin.New(cfg.LinkedIn),
		telegram: telegram.New(cfg.Telegram),
	}
}

// Publisher devolve o conector da plataforma.
func (r *Registry) Publisher(p domain.Platform) (Publisher, error) {
	switch p {
	case domain.PlatformX:
		return r.x, nil
	case domain.PlatformLinkedIn:
		return r.linkedin, nil
	case domain.PlatformTelegram:
		return r.telegram, nil
	default:
		return nil, fmt.Errorf("plataforma não suportada: %s", p)
	}
}

// OAuth devolve o conector se ele tiver fluxo OAuth (X/LinkedIn).
func (r *Registry) OAuth(p domain.Platform) (OAuthProvider, error) {
	pub, err := r.Publisher(p)
	if err != nil {
		return nil, err
	}
	oa, ok := pub.(OAuthProvider)
	if !ok {
		return nil, fmt.Errorf("plataforma %s não usa OAuth (bot token/credenciais)", p)
	}
	return oa, nil
}
