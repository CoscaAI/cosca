package cli

import "github.com/CoscaAI/cosca/internal/embed"

// materializeFallback copies the canonical framework trees from the embedded
// filesystem into <root>/.cosca/fallback/. See embed.MaterializeFallback for
// details; this thin wrapper keeps call sites in init/sync readable.
func materializeFallback(root string) error {
	return embed.MaterializeFallback(root)
}
