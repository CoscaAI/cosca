package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewValidates(t *testing.T) {
	if _, err := New("", TypeImage); err == nil {
		t.Fatal("expected error for empty name")
	}
	if _, err := New("ok", ProjectType("nope")); err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestNewDefaults(t *testing.T) {
	m, err := New("acme", TypeCinema)
	if err != nil {
		t.Fatal(err)
	}
	if m.Type != TypeCinema.String() {
		t.Fatalf("type = %q, want %q", m.Type, TypeCinema)
	}
	if m.Version != "0.1.0" {
		t.Fatalf("version = %q, want 0.1.0", m.Version)
	}
	if m.RenderSettings["quality"] != "draft" {
		t.Fatalf("render quality = %v, want draft", m.RenderSettings["quality"])
	}
	if !m.IsType(TypeCinema) {
		t.Fatal("IsType(cinema) should be true")
	}
}

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	m, err := New("hornfit", TypeGame)
	if err != nil {
		t.Fatal(err)
	}
	m.Models = []ModelRef{{ID: "whisper", Provider: "local", Version: "large-v3"}}
	m.Dependencies = []DependencyRef{{ID: "ffmpeg", Version: "6.0", License: "GPL"}}
	if err := m.Write(dir); err != nil {
		t.Fatal(err)
	}

	got, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "hornfit" || got.Type != TypeGame.String() {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	if len(got.Models) != 1 || got.Models[0].ID != "whisper" {
		t.Fatalf("models mismatch: %+v", got.Models)
	}
	if len(got.Dependencies) != 1 || got.Dependencies[0].License != "GPL" {
		t.Fatalf("deps mismatch: %+v", got.Dependencies)
	}
}

func TestReadMissing(t *testing.T) {
	dir := t.TempDir()
	if _, err := Read(dir); !os.IsNotExist(err) {
		t.Fatalf("expected ErrNotExist, got %v", err)
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		manifest Manifest
		wantErr bool
	}{
		{"ok", Manifest{Name: "x", Type: "lab"}, false},
		{"empty name", Manifest{Type: "lab"}, true},
		{"empty type", Manifest{Name: "x"}, true},
		{"bad type", Manifest{Name: "x", Type: "misc"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.manifest.Validate()
			if (err != nil) != c.wantErr {
				t.Fatalf("Validate() err = %v, wantErr %v", err, c.wantErr)
			}
		})
	}
}

func TestAllTypesValid(t *testing.T) {
	for _, typ := range AllProjectTypes {
		if !typ.Valid() {
			t.Fatalf("type %q should be valid", typ)
		}
	}
	if ProjectType("bogus").Valid() {
		t.Fatal("bogus type should be invalid")
	}
}

func TestPath(t *testing.T) {
	m, _ := New("x", TypeDocument)
	got := m.Path(filepath.Join("a", "b"))
	want := filepath.Join("a", "b", ".cosca", "project.yaml")
	if got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}
