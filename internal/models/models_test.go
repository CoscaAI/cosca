package models

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// sampleAPI is a small, realistic fixture mirroring the current models.dev
// api.json shape (provider-keyed) plus a legacy entry to exercise the
// tolerant parser (context_length / pricing.* as decimal strings).
const sampleAPI = `{
  "deepseek": {
    "id": "deepseek",
    "name": "DeepSeek",
    "api": "https://api.deepseek.com/v1",
    "models": {
      "deepseek-v4-flash": {
        "id": "deepseek-v4-flash",
        "name": "DeepSeek V4 Flash",
        "limit": {"context": 1000000, "output": 384000},
        "cost": {"input": 0.14, "output": 0.28, "cache_read": 0.0028},
        "modalities": {"input": ["text"], "output": ["text"]}
      }
    }
  },
  "openai": {
    "id": "openai",
    "name": "OpenAI",
    "models": {
      "gpt-4o": {
        "id": "gpt-4o",
        "name": "GPT-4o",
        "limit": {"context": 128000},
        "cost": {"input": 2.5, "output": 10, "cache_read": 1.25, "cache_write": 3.75},
        "modalities": {"input": ["text", "image"], "output": ["text"]}
      }
    }
  },
  "legacy-provider": {
    "id": "legacy-provider",
    "name": "Legacy",
    "models": {
      "legacy-model": {
        "id": "legacy-provider/legacy-model",
        "name": "Legacy Model",
        "context_length": 8192,
        "pricing": {"prompt": "0.00003", "completion": "0.00006"},
        "architecture": {"input_modalities": ["text"], "output_modalities": ["text"]},
        "supported_parameters": ["temperature", "top_p"],
        "top_provider": "legacy-provider"
      }
    }
  }
}`

func TestParseSampleAPI(t *testing.T) {
	models, err := parseCatalog([]byte(sampleAPI))
	if err != nil {
		t.Fatalf("parseCatalog: %v", err)
	}

	m, ok := models["deepseek/deepseek-v4-flash"]
	if !ok {
		t.Fatalf("expected deepseek/deepseek-v4-flash in catalog, got %d entries", len(models))
	}
	if m.ContextLength != 1000000 {
		t.Errorf("ContextLength = %d, want 1000000", m.ContextLength)
	}
	if m.Pricing.Prompt != 0.14 {
		t.Errorf("Pricing.Prompt = %v, want 0.14", m.Pricing.Prompt)
	}
	if m.Pricing.Completion != 0.28 {
		t.Errorf("Pricing.Completion = %v, want 0.28", m.Pricing.Completion)
	}
	if m.Pricing.InputCacheRead != 0.0028 {
		t.Errorf("Pricing.InputCacheRead = %v, want 0.0028", m.Pricing.InputCacheRead)
	}
	if len(m.InputModalities) != 1 || m.InputModalities[0] != "text" {
		t.Errorf("InputModalities = %v, want [text]", m.InputModalities)
	}
	if m.Provider != "DeepSeek" {
		t.Errorf("Provider = %q, want DeepSeek", m.Provider)
	}

	o, ok := models["openai/gpt-4o"]
	if !ok {
		t.Fatal("expected openai/gpt-4o in catalog")
	}
	if o.Pricing.InputCacheWrite != 3.75 {
		t.Errorf("InputCacheWrite = %v, want 3.75", o.Pricing.InputCacheWrite)
	}
	if len(o.InputModalities) != 2 || o.InputModalities[1] != "image" {
		t.Errorf("InputModalities = %v, want [text image]", o.InputModalities)
	}

	// Legacy entry: canonical ID from the declared id, string pricing parsed.
	l, ok := models["legacy-provider/legacy-model"]
	if !ok {
		t.Fatal("expected legacy-provider/legacy-model in catalog")
	}
	if l.ContextLength != 8192 {
		t.Errorf("legacy ContextLength = %d, want 8192", l.ContextLength)
	}
	if l.Pricing.Prompt != 0.00003 {
		t.Errorf("legacy Pricing.Prompt = %v, want 0.00003", l.Pricing.Prompt)
	}
	if l.Pricing.Completion != 0.00006 {
		t.Errorf("legacy Pricing.Completion = %v, want 0.00006", l.Pricing.Completion)
	}
	if len(l.SupportedParameters) != 2 {
		t.Errorf("SupportedParameters = %v, want 2 entries", l.SupportedParameters)
	}
	if l.Provider != "legacy-provider" {
		t.Errorf("legacy Provider = %q, want legacy-provider", l.Provider)
	}
}

func TestSaveAndLoadCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.json")

	c := NewCache(path)
	c.UpdatedAt = time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	c.Models["deepseek/deepseek-v4-flash"] = Model{
		ID:            "deepseek/deepseek-v4-flash",
		Name:          "DeepSeek V4 Flash",
		ContextLength: 1000000,
		Provider:      "DeepSeek",
	}

	if err := SaveCache(path, c); err != nil {
		t.Fatalf("SaveCache: %v", err)
	}

	// 0600 is a POSIX permission bit; chmod is a no-op on Windows, so the
	// assertion only applies on unix.
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat cache: %v", err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("cache mode = %o, want 600", info.Mode().Perm())
		}
	}

	loaded, err := LoadCache(path)
	if err != nil {
		t.Fatalf("LoadCache: %v", err)
	}
	if len(loaded.Models) != 1 {
		t.Fatalf("loaded %d models, want 1", len(loaded.Models))
	}
	m, ok := loaded.Models["deepseek/deepseek-v4-flash"]
	if !ok {
		t.Fatal("missing model after round-trip")
	}
	if m.ContextLength != 1000000 || m.Name != "DeepSeek V4 Flash" {
		t.Errorf("round-trip mismatch: %+v", m)
	}
	if !loaded.UpdatedAt.Equal(c.UpdatedAt) {
		t.Errorf("UpdatedAt = %v, want %v", loaded.UpdatedAt, c.UpdatedAt)
	}
}

func TestLoadCacheMissing(t *testing.T) {
	c, err := LoadCache(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("LoadCache(missing) error = %v, want nil", err)
	}
	if len(c.Models) != 0 {
		t.Errorf("expected empty cache, got %d models", len(c.Models))
	}
}

func writeTestCache(t *testing.T, models map[string]Model) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "models.json")
	c := NewCache(path)
	c.UpdatedAt = time.Now().UTC()
	c.Models = models
	if err := SaveCache(path, c); err != nil {
		t.Fatalf("SaveCache: %v", err)
	}
	return path
}

func TestLookupExact(t *testing.T) {
	path := writeTestCache(t, map[string]Model{
		"deepseek/deepseek-v4-flash": {
			ID:            "deepseek/deepseek-v4-flash",
			Name:          "DeepSeek V4 Flash",
			ContextLength: 1000000,
		},
	})

	m, ok := Lookup(path, "deepseek/deepseek-v4-flash")
	if !ok {
		t.Fatal("exact lookup failed")
	}
	if m.ContextLength != 1000000 {
		t.Errorf("ContextLength = %d, want 1000000", m.ContextLength)
	}

	if cl := ContextWindowFor(path, "deepseek/deepseek-v4-flash"); cl != 1000000 {
		t.Errorf("ContextWindowFor = %d, want 1000000", cl)
	}
}

func TestLookupFuzzy(t *testing.T) {
	path := writeTestCache(t, map[string]Model{
		"deepseek/deepseek-v4-flash": {
			ID:            "deepseek/deepseek-v4-flash",
			Name:          "DeepSeek V4 Flash",
			ContextLength: 1000000,
		},
	})

	// Bare model name should match the suffix of the full ID.
	m, ok := Lookup(path, "deepseek-v4-flash")
	if !ok {
		t.Fatal("suffix lookup failed for bare model name")
	}
	if m.ID != "deepseek/deepseek-v4-flash" {
		t.Errorf("suffix lookup returned %q", m.ID)
	}

	// Case-insensitive name match.
	m, ok = Lookup(path, "deepseek v4 flash")
	if !ok || m.ID != "deepseek/deepseek-v4-flash" {
		t.Errorf("name lookup = %+v, ok=%v", m, ok)
	}
}

func TestLookupMissing(t *testing.T) {
	path := writeTestCache(t, map[string]Model{
		"deepseek/deepseek-v4-flash": {ID: "deepseek/deepseek-v4-flash"},
	})

	if _, ok := Lookup(path, "gpt-99"); ok {
		t.Error("Lookup(gpt-99) = true, want false")
	}
	if _, ok := Lookup(path, "deepseek/other-model"); ok {
		t.Error("Lookup(deepseek/other-model) = true, want false")
	}
	if _, ok := Lookup(path, ""); ok {
		t.Error("Lookup(empty) = true, want false")
	}
	if cl := ContextWindowFor(path, "gpt-99"); cl != 0 {
		t.Errorf("ContextWindowFor(unknown) = %d, want 0", cl)
	}
}

func TestLookupMissingCache(t *testing.T) {
	// Missing cache file must not error or panic.
	if _, ok := Lookup(filepath.Join(t.TempDir(), "missing.json"), "deepseek-v4-flash"); ok {
		t.Error("Lookup on missing cache = true, want false")
	}
	// Corrupt cache must also degrade to "not found".
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := Lookup(bad, "deepseek-v4-flash"); ok {
		t.Error("Lookup on corrupt cache = true, want false")
	}
}

func TestSyncFromAPIOffline(t *testing.T) {
	// A bad URL must return an error — and must not panic or write a cache.
	dest := filepath.Join(t.TempDir(), "models.json")
	_, err := SyncFromAPI(context.Background(), "http://127.0.0.1:1/nope", dest)
	if err == nil {
		t.Fatal("SyncFromAPI(bad url) = nil error, want error")
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Error("bad-URL sync must not create the cache file")
	}

	// Malformed success payload must also return an error, not panic.
	_, err = SyncFromAPI(context.Background(), "http://127.0.0.1:1/", dest)
	if err == nil {
		t.Error("SyncFromAPI(unreachable) = nil error, want error")
	}
}

func TestPricingParse(t *testing.T) {
	cases := map[string]float64{
		"0.00003":   0.00003,
		"0":         0,
		"1.5":       1.5,
		" 0.0028 ":  0.0028,
		"0.0000001": 0.0000001,
	}
	for in, want := range cases {
		got, err := parsePrice(in)
		if err != nil {
			t.Errorf("parsePrice(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("parsePrice(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseCatalogFlat(t *testing.T) {
	// models.json layout (provider-agnostic, flat, no cost).
	flat := `{
	  "deepseek/deepseek-v4-flash": {
	    "id": "deepseek/deepseek-v4-flash",
	    "name": "DeepSeek V4 Flash",
	    "limit": {"context": 1000000},
	    "modalities": {"input": ["text"], "output": ["text"]}
	  }
	}`
	models, err := parseCatalog([]byte(flat))
	if err != nil {
		t.Fatalf("parseCatalog(flat): %v", err)
	}
	m, ok := models["deepseek/deepseek-v4-flash"]
	if !ok {
		t.Fatal("flat catalog missing model")
	}
	if m.ContextLength != 1000000 {
		t.Errorf("ContextLength = %d, want 1000000", m.ContextLength)
	}
	if m.Provider != "deepseek" {
		t.Errorf("Provider = %q, want deepseek", m.Provider)
	}
}

func TestParseCatalogCatalogJSON(t *testing.T) {
	// catalog.json layout: {"models": {...}, "providers": {...}}.
	catalog := `{
	  "models": {
	    "deepseek/deepseek-v4-flash": {
	      "id": "deepseek/deepseek-v4-flash",
	      "name": "DeepSeek V4 Flash",
	      "limit": {"context": 1000000}
	    }
	  },
	  "providers": {"deepseek": {"name": "DeepSeek"}}
	}`
	models, err := parseCatalog([]byte(catalog))
	if err != nil {
		t.Fatalf("parseCatalog(catalog.json): %v", err)
	}
	if _, ok := models["deepseek/deepseek-v4-flash"]; !ok {
		t.Fatal("catalog.json missing model")
	}
}

func TestCacheJSONShape(t *testing.T) {
	// The cache file must round-trip through JSON with the documented fields.
	c := NewCache("")
	c.Models["deepseek/deepseek-v4-flash"] = Model{
		ID:            "deepseek/deepseek-v4-flash",
		Name:          "DeepSeek V4 Flash",
		ContextLength: 1000000,
		Provider:      "DeepSeek",
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		UpdatedAt time.Time        `json:"updated_at"`
		Models    map[string]Model `json:"models"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("cache JSON shape invalid: %v", err)
	}
	if decoded.Models["deepseek/deepseek-v4-flash"].ContextLength != 1000000 {
		t.Error("cache JSON missing context_length")
	}
}

func TestParseCatalogCanonicalOwnerWins(t *testing.T) {
	// Two providers host the same canonical ID; the provider whose key owns
	// the ID prefix must win, not a reseller alias.
	const doc = `{
	  "reseller": {
	    "name": "Reseller Inc",
	    "models": {
	      "deepseek-v4-flash": {
	        "id": "deepseek/deepseek-v4-flash",
	        "name": "DeepSeek V4 Flash (resold)",
	        "limit": {"context": 999}
	      }
	    }
	  },
	  "deepseek": {
	    "name": "DeepSeek",
	    "models": {
	      "deepseek-v4-flash": {
	        "id": "deepseek-v4-flash",
	        "name": "DeepSeek V4 Flash",
	        "limit": {"context": 1000000}
	      }
	    }
	  }
	}`
	models, err := parseCatalog([]byte(doc))
	if err != nil {
		t.Fatalf("parseCatalog: %v", err)
	}
	m, ok := models["deepseek/deepseek-v4-flash"]
	if !ok {
		t.Fatal("missing canonical model")
	}
	if m.ContextLength != 1000000 {
		t.Errorf("ContextLength = %d, want 1000000 (canonical owner must win)", m.ContextLength)
	}
	if m.Name != "DeepSeek V4 Flash" {
		t.Errorf("Name = %q, want DeepSeek V4 Flash", m.Name)
	}
	if m.Provider != "DeepSeek" {
		t.Errorf("Provider = %q, want DeepSeek", m.Provider)
	}
}

func TestLookupFuzzyDeterministic(t *testing.T) {
	// Multiple providers host the same bare model name; the lookup must be
	// deterministic and prefer the canonical provider.
	path := writeTestCache(t, map[string]Model{
		"azure/deepseek-v4-flash":    {ID: "azure/deepseek-v4-flash", Name: "DeepSeek V4 Flash", ContextLength: 1000000, Provider: "Azure"},
		"deepseek/deepseek-v4-flash": {ID: "deepseek/deepseek-v4-flash", Name: "DeepSeek V4 Flash", ContextLength: 1000000, Provider: "DeepSeek"},
		"reseller/deepseek-v4-flash": {ID: "reseller/deepseek-v4-flash", Name: "DeepSeek V4 Flash", ContextLength: 999, Provider: "Reseller Inc"},
	})

	m, ok := Lookup(path, "deepseek-v4-flash")
	if !ok {
		t.Fatal("suffix lookup failed")
	}
	if m.ID != "deepseek/deepseek-v4-flash" {
		t.Errorf("fuzzy lookup = %q, want canonical deepseek/deepseek-v4-flash", m.ID)
	}

	// Same result across repeated calls.
	for i := 0; i < 10; i++ {
		got, _ := Lookup(path, "deepseek-v4-flash")
		if got.ID != m.ID {
			t.Fatalf("non-deterministic lookup: %q vs %q", got.ID, m.ID)
		}
	}
}
