// Package platform define a camada de conectores por rede (ADR-004 §regras).
//
// Os tipos base (interfaces Publisher/OAuthProvider, Credentials, Token,
// PublishResult, Error) vivem em platform/types para evitar o ciclo
// platform → conector → platform; este pacote os re-exporta para o resto do
// código, e o Registry (registry.go) instancia os conectores.
//
// Implementação apenas com net/http (sem SDKs externos), contra as specs
// oficiais das plataformas.
package platform

import "github.com/rizomai/rizomai/internal/platform/types"

// Re-exports dos tipos base (API pública do pacote).
type (
	Credentials   = types.Credentials
	PublishResult = types.PublishResult
	Publisher     = types.Publisher
	Token         = types.Token
	OAuthProvider = types.OAuthProvider
	Error         = types.Error
)
