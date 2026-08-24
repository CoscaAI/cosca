package worlddebug

import (
	"sort"
)

// Asset Browser — semantic asset catalog.
//
// The professor's point: do NOT resolve "tree_001.uasset" by filename. Query
// by semantics: (species, context, age, scale). The resolver takes a World
// Entity requirement and ranks candidate assets semantically.
//
//   World Entity → "urban mature tree" → Asset Query → candidate ranking → tree_urban_mature_04

// AssetDescriptor is a semantic catalog entry.
type AssetDescriptor struct {
	AssetID     string   `json:"asset_id"`
	Category    string   `json:"category"`   // "tree", "building", "road"
	Species     []string `json:"species"`    // for vegetation
	Contexts    []string `json:"contexts"`   // "forest", "urban", "standalone"
	Ages        []string `json:"ages"`       // "young", "mature", "old"
	ScaleRange  [2]float64 `json:"scale_range"` // min/max meters
	FoliageDensity float64 `json:"foliage_density"`
}

// AssetQuery specifies the semantic requirements for an asset.
type AssetQuery struct {
	Category    string   `json:"category"`
	Species     []string `json:"species"`
	Contexts    []string `json:"contexts"`
	Ages        []string `json:"ages"`
	MaxScale    float64  `json:"max_scale"`
}

// AssetMatch is a ranked candidate.
type AssetMatch struct {
	AssetID  string  `json:"asset_id"`
	Score    float64 `json:"score"`
	Reasons  []string `json:"reasons"`
}

// Browser queries a semantic asset catalog.
type Browser struct {
	catalog []AssetDescriptor
}

// NewBrowser creates a Browser with the given catalog.
func NewBrowser(catalog []AssetDescriptor) *Browser {
	return &Browser{catalog: catalog}
}

// Query ranks catalog assets against the requirement.
func (b *Browser) Query(q AssetQuery) []AssetMatch {
	var matches []AssetMatch
	for _, a := range b.catalog {
		if a.Category != q.Category {
			continue
		}
		score, reasons := scoreAsset(a, q)
		if score > 0 {
			matches = append(matches, AssetMatch{AssetID: a.AssetID, Score: score, Reasons: reasons})
		}
	}
	// Sort by score descending.
	sort.Slice(matches, func(i, j int) bool { return matches[i].Score > matches[j].Score })
	return matches
}

// scoreAsset computes a match score for an asset against a query.
func scoreAsset(a AssetDescriptor, q AssetQuery) (float64, []string) {
	var score float64
	var reasons []string

	// Species overlap (up to 2 points).
	if len(q.Species) > 0 {
		for _, s := range a.Species {
			if contains(q.Species, s) {
				score += 2
				reasons = append(reasons, "species "+s+" compatible")
				break
			}
		}
	}
	// Context overlap (up to 2 points).
	if len(q.Contexts) > 0 {
		for _, c := range a.Contexts {
			if contains(q.Contexts, c) {
				score += 1.5
				reasons = append(reasons, "context "+c+" compatible")
				break
			}
		}
	}
	// Age overlap (up to 1 point).
	if len(q.Ages) > 0 {
		for _, g := range a.Ages {
			if contains(q.Ages, g) {
				score += 1
				reasons = append(reasons, "age "+g+" compatible")
				break
			}
		}
	}
	// Scale range compatible (1 point).
	if q.MaxScale > 0 && a.ScaleRange[1] >= q.MaxScale {
		score += 1
		reasons = append(reasons, "scale range compatible")
	}

	return score, reasons
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// DefaultTreeCatalog builds a semantic catalog for vegetation.
// This shows the professor's intent: query by species/context/age, not filename.
func DefaultTreeCatalog() []AssetDescriptor {
	return []AssetDescriptor{
		{AssetID: "tree_forest_oak_mature", Category: "tree", Species: []string{"oak"}, Contexts: []string{"forest"}, Ages: []string{"mature"}, ScaleRange: [2]float64{8, 15}},
		{AssetID: "tree_urban_oak_mature", Category: "tree", Species: []string{"oak"}, Contexts: []string{"urban"}, Ages: []string{"mature"}, ScaleRange: [2]float64{6, 10}},
		{AssetID: "tree_urban_pine_young", Category: "tree", Species: []string{"pine"}, Contexts: []string{"urban"}, Ages: []string{"young"}, ScaleRange: [2]float64{2, 5}},
		{AssetID: "tree_forest_birch_old", Category: "tree", Species: []string{"birch"}, Contexts: []string{"forest"}, Ages: []string{"old"}, ScaleRange: [2]float64{10, 20}},
		{AssetID: "tree_standalone_pine_mature", Category: "tree", Species: []string{"pine"}, Contexts: []string{"standalone"}, Ages: []string{"mature"}, ScaleRange: [2]float64{5, 10}},
	}
}
