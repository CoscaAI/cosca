package provenance

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddClaimValidates(t *testing.T) {
	r, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// Válida.
	c := Claim{ID: "c1", Statement: "O modelo alcançou 0.92 de precisão", Kind: Calculated, Confidence: 0.9}
	if err := r.AddClaim(c); err != nil {
		t.Fatal(err)
	}
	// Kind inválido.
	if err := r.AddClaim(Claim{ID: "c2", Statement: "x", Kind: ClaimKind("magic"), Confidence: 0.5}); err == nil {
		t.Fatal("expected invalid kind error")
	}
	// Confidence fora de faixa.
	if err := r.AddClaim(Claim{ID: "c3", Statement: "x", Kind: Observed, Confidence: 1.5}); err == nil {
		t.Fatal("expected confidence error")
	}
}

func TestAddGenerationValidates(t *testing.T) {
	r, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	g := Generation{
		AssetID: "abc123", Model: "stable-diffusion", Prompt: "uma cidade futurista",
		Seed: 42, SourceAssets: []string{"asset1"}, Processing: []string{"upscale"},
	}
	if err := r.AddGeneration(g); err != nil {
		t.Fatal(err)
	}
	// Sem asset_id.
	if err := r.AddGeneration(Generation{Model: "x"}); err == nil {
		t.Fatal("expected asset_id error")
	}
	// Sem model.
	if err := r.AddGeneration(Generation{AssetID: "x"}); err == nil {
		t.Fatal("expected model error")
	}
}

func TestAddLicenseValidates(t *testing.T) {
	r, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	l := LicenseRecord{Name: "ffmpeg", Source: SourceThirdParty, License: "GPL-2.0", Version: "6.0", Compatible: false}
	if err := r.AddLicense(l); err != nil {
		t.Fatal(err)
	}
	if err := r.AddLicense(LicenseRecord{Name: "x", Source: LicenseSource("bogus"), License: "MIT"}); err == nil {
		t.Fatal("expected invalid source error")
	}
	if err := r.AddLicense(LicenseRecord{Name: "y", License: ""}); err == nil {
		t.Fatal("expected license id error")
	}
}

func TestPersistence(t *testing.T) {
	root := t.TempDir()
	r, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	_ = r.AddClaim(Claim{ID: "c1", Statement: "medido", Kind: Observed, Confidence: 0.95})
	_ = r.AddGeneration(Generation{AssetID: "g1", Model: "whisper", Prompt: "transcrever"})
	_ = r.AddLicense(LicenseRecord{Name: "numpy", Source: SourceThirdParty, License: "BSD-3-Clause", Compatible: true})

	r2, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(r2.Claims) != 1 || r2.Claims[0].Kind != Observed {
		t.Fatalf("claims mismatch: %+v", r2.Claims)
	}
	if len(r2.Generations) != 1 || r2.Generations[0].Seed != 0 {
		t.Fatalf("generations mismatch: %+v", r2.Generations)
	}
	if len(r2.Licenses) != 1 || r2.Licenses[0].License != "BSD-3-Clause" {
		t.Fatalf("licenses mismatch: %+v", r2.Licenses)
	}
	if _, err := os.Stat(filepath.Join(root, DefaultDir, FileName)); err != nil {
		t.Fatalf("provenance file missing: %v", err)
	}
}

func TestClaimByID(t *testing.T) {
	r, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_ = r.AddClaim(Claim{ID: "c1", Statement: "s", Kind: Simulated, Confidence: 0.5})
	c, ok := r.ClaimByID("c1")
	if !ok || c.Statement != "s" {
		t.Fatalf("ClaimByID = %+v ok=%v", c, ok)
	}
	if _, ok := r.ClaimByID("missing"); ok {
		t.Fatal("missing claim should not be found")
	}
}

func TestAllKindsAndSourcesValid(t *testing.T) {
	for _, k := range []ClaimKind{Observed, Calculated, Simulated, Generated, Hypothesis} {
		if !k.Valid() {
			t.Fatalf("claim kind %q should be valid", k)
		}
	}
	if ClaimKind("bogus").Valid() {
		t.Fatal("bogus kind should be invalid")
	}
	for _, s := range []LicenseSource{SourceCoscaCode, SourceThirdParty, SourceModel, SourceUserAsset} {
		if !s.Valid() {
			t.Fatalf("license source %q should be valid", s)
		}
	}
	if LicenseSource("bogus").Valid() {
		t.Fatal("bogus source should be invalid")
	}
}
