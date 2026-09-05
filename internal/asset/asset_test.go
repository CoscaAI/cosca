package asset

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestNewHashesContent(t *testing.T) {
	data := []byte("hello asset")
	a, err := New(data, TypeImage, "test.png")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	want := hex.EncodeToString(sum[:])
	if a.ID != want {
		t.Fatalf("ID = %q, want %q", a.ID, want)
	}
	if a.Hash != want {
		t.Fatalf("Hash = %q, want %q", a.Hash, want)
	}
	if a.Size != int64(len(data)) {
		t.Fatalf("Size = %d, want %d", a.Size, len(data))
	}
	if a.Type != TypeImage {
		t.Fatalf("Type = %q, want image", a.Type)
	}
	if !a.IsType(TypeImage) {
		t.Fatal("IsType(image) should be true")
	}
}

func TestNewInvalidType(t *testing.T) {
	if _, err := New([]byte("x"), Type("bogus"), ""); err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestAddDeduplicates(t *testing.T) {
	r, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("same content")

	a1, err := r.Add(data, TypeText, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	a2, err := r.Add(data, TypeText, "b.txt")
	if err != nil {
		t.Fatal(err)
	}
	if a1.ID != a2.ID {
		t.Fatalf("dedup failed: %q != %q", a1.ID, a2.ID)
	}
	if r.Count() != 1 {
		t.Fatalf("Count = %d, want 1", r.Count())
	}
	// O source do segundo não sobrescreve — retorna o existente.
	if a1.Source != "a.txt" {
		t.Fatalf("existing source = %q, want a.txt", a1.Source)
	}
}

func TestAddFileAndGet(t *testing.T) {
	root := t.TempDir()
	r, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}

	src := filepath.Join(root, "logo.png")
	if err := os.WriteFile(src, []byte("png-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	a, err := r.AddFile(src, TypeImage, "")
	if err != nil {
		t.Fatal(err)
	}
	if a.Source != src {
		t.Fatalf("Source = %q, want %q", a.Source, src)
	}

	got, ok := r.Get(a.ID)
	if !ok {
		t.Fatal("Get should find the asset")
	}
	if got.Type != TypeImage {
		t.Fatalf("type = %q, want image", got.Type)
	}

	data, err := r.ReadObject(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "png-bytes" {
		t.Fatalf("object content = %q, want png-bytes", data)
	}

	// O blob content-addressable existe no disco.
	if _, err := os.Stat(r.objectPath(a.ID)); err != nil {
		t.Fatalf("object file missing: %v", err)
	}
}

func TestPersistenceAcrossReopen(t *testing.T) {
	root := t.TempDir()
	r, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Add([]byte("persist me"), TypeScript, "run.sh"); err != nil {
		t.Fatal(err)
	}

	// Reabre o registry — índice lido do disco.
	r2, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Count() != 1 {
		t.Fatalf("after reopen Count = %d, want 1", r2.Count())
	}
	assets := r2.List()
	if assets[0].Type != TypeScript {
		t.Fatalf("type = %q, want script", assets[0].Type)
	}
}

func TestRemove(t *testing.T) {
	r, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a, err := r.Add([]byte("temp"), TypeData, "")
	if err != nil {
		t.Fatal(err)
	}

	if err := r.Remove(a.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Get(a.ID); ok {
		t.Fatal("asset should be removed")
	}
	if r.Count() != 0 {
		t.Fatalf("Count = %d, want 0", r.Count())
	}

	if err := r.Remove(a.ID); err == nil {
		t.Fatal("expected error removing missing asset")
	}
}

func TestCopyTo(t *testing.T) {
	root := t.TempDir()
	r, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	a, err := r.Add([]byte("copy me"), TypeDocument, "")
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(root, "out.pdf")
	if err := r.CopyTo(a.ID, dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "copy me" {
		t.Fatalf("copied content = %q, want copy me", got)
	}
}

func TestAllTypesValid(t *testing.T) {
	for _, typ := range AllTypes {
		if !typ.Valid() {
			t.Fatalf("type %q should be valid", typ)
		}
	}
	if Type("nope").Valid() {
		t.Fatal("bogus type should be invalid")
	}
}
