package platform

import (
	"fmt"

	"github.com/rizomai/rizomai/internal/domain"
	"github.com/rizomai/rizomai/internal/platform/bluesky"
	"github.com/rizomai/rizomai/internal/platform/facebook"
	"github.com/rizomai/rizomai/internal/platform/googlebusiness"
	"github.com/rizomai/rizomai/internal/platform/instagram"
	"github.com/rizomai/rizomai/internal/platform/linkedin"
	"github.com/rizomai/rizomai/internal/platform/pinterest"
	"github.com/rizomai/rizomai/internal/platform/reddit"
	"github.com/rizomai/rizomai/internal/platform/snapchat"
	"github.com/rizomai/rizomai/internal/platform/telegram"
	"github.com/rizomai/rizomai/internal/platform/threads"
	"github.com/rizomai/rizomai/internal/platform/tiktok"
	"github.com/rizomai/rizomai/internal/platform/x"
	"github.com/rizomai/rizomai/internal/platform/youtube"
)

// Config reúne as credenciais de APP (via env — nunca em código).
type Config struct {
	X              x.Config
	LinkedIn       linkedin.Config
	Telegram       telegram.Config
	Instagram      instagram.Config
	Facebook       facebook.Config
	Threads        threads.Config
	YouTube        youtube.Config
	TikTok         tiktok.Config
	Bluesky        bluesky.Config
	Reddit         reddit.Config
	Pinterest      pinterest.Config
	Snapchat       snapchat.Config
	GoogleBusiness googlebusiness.Config
}

// Registry é a fábrica de publishers por plataforma.
type Registry struct {
	x              *x.Client
	linkedin       *linkedin.Client
	telegram       *telegram.Client
	instagram      *instagram.Client
	facebook       *facebook.Client
	threads        *threads.Client
	youtube        *youtube.Client
	tiktok         *tiktok.Client
	bluesky        *bluesky.Client
	reddit         *reddit.Client
	pinterest      *pinterest.Client
	snapchat       *snapchat.Client
	googlebusiness *googlebusiness.Client
}

// NewRegistry instancia os conectores com as credenciais de app.
func NewRegistry(cfg Config) *Registry {
	return &Registry{
		x:              x.New(cfg.X),
		linkedin:       linkedin.New(cfg.LinkedIn),
		telegram:       telegram.New(cfg.Telegram),
		instagram:      instagram.New(cfg.Instagram),
		facebook:       facebook.New(cfg.Facebook),
		threads:        threads.New(cfg.Threads),
		youtube:        youtube.New(cfg.YouTube),
		tiktok:         tiktok.New(cfg.TikTok),
		bluesky:        bluesky.New(cfg.Bluesky),
		reddit:         reddit.New(cfg.Reddit),
		pinterest:      pinterest.New(cfg.Pinterest),
		snapchat:       snapchat.New(cfg.Snapchat),
		googlebusiness: googlebusiness.New(cfg.GoogleBusiness),
	}
}

// Publisher devolve o conector da plataforma (13 redes — 13 conectores).
func (r *Registry) Publisher(p domain.Platform) (Publisher, error) {
	switch p {
	case domain.PlatformX:
		return r.x, nil
	case domain.PlatformLinkedIn:
		return r.linkedin, nil
	case domain.PlatformTelegram:
		return r.telegram, nil
	case domain.PlatformInstagram:
		return r.instagram, nil
	case domain.PlatformFacebook:
		return r.facebook, nil
	case domain.PlatformThreads:
		return r.threads, nil
	case domain.PlatformYouTube:
		return r.youtube, nil
	case domain.PlatformTikTok:
		return r.tiktok, nil
	case domain.PlatformBluesky:
		return r.bluesky, nil
	case domain.PlatformReddit:
		return r.reddit, nil
	case domain.PlatformPinterest:
		return r.pinterest, nil
	case domain.PlatformSnapchat:
		return r.snapchat, nil
	case domain.PlatformGoogleBusiness:
		return r.googlebusiness, nil
	default:
		return nil, fmt.Errorf("plataforma não suportada: %s", p)
	}
}

// OAuth devolve o conector se ele tiver fluxo OAuth de navegador.
// Telegram (bot token), Bluesky (app password) e Reddit (script flow)
// NÃO usam browser OAuth — usam credentials (ADR-006 §4).
func (r *Registry) OAuth(p domain.Platform) (OAuthProvider, error) {
	pub, err := r.Publisher(p)
	if err != nil {
		return nil, err
	}
	oa, ok := pub.(OAuthProvider)
	if !ok {
		return nil, fmt.Errorf("plataforma %s não usa OAuth de navegador (bot token/app password/script flow)", p)
	}
	return oa, nil
}

// CredentialPlatforms devolve as plataformas conectadas via credentials direto.
func CredentialPlatforms() []domain.Platform {
	return []domain.Platform{domain.PlatformTelegram, domain.PlatformBluesky, domain.PlatformReddit}
}
