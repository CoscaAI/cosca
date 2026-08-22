package knowledge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultLawsPathAndProjectLawsPath(t *testing.T) {
	// DefaultLawsPath: caminho no home.
	p := DefaultLawsPath()
	if !strings.Contains(p, ".cosca") || !strings.Contains(p, "laws.json") {
		t.Fatalf("DefaultLawsPath = %q", p)
	}
	// ProjectLawsPath: .cosca/knowledge/laws.json relativo ao CWD.
	pp := ProjectLawsPath()
	if !strings.Contains(pp, ".cosca") || !strings.Contains(pp, "laws.json") {
		t.Fatalf("ProjectLawsPath = %q", pp)
	}
}

func TestVerifyItemAndVerifyAll(t *testing.T) {
	e := NewPromotionEngine()
	item := &KnowledgeItem{ID: "K-verify", Title: "x", Evidence: []Evidence{{ID: "E-1"}}, Confidence: 0.9}
	if err := e.Register(item); err != nil {
		t.Fatal(err)
	}

	// VerifyItem: item inexistente → erro.
	if err := e.VerifyItem("ghost"); err == nil {
		t.Fatal("VerifyItem(ghost) must error")
	}
	// VerifyItem válido: incrementa contador.
	if err := e.VerifyItem("K-verify"); err != nil {
		t.Fatal(err)
	}
	got, _ := e.Get("K-verify")
	if got.VerificationCount != 1 || got.LastVerified.IsZero() {
		t.Fatalf("after verify: count=%d", got.VerificationCount)
	}

	// VerifyAll: incrementa todos, best-effort.
	e.VerifyAll()
	got2, _ := e.Get("K-verify")
	if got2.VerificationCount != 2 {
		t.Fatalf("after VerifyAll: count=%d", got2.VerificationCount)
	}
}

func TestListKnowledgeItems(t *testing.T) {
	// Arquivo inexistente → lista vazia, sem erro.
	items, err := ListKnowledgeItems(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil || len(items) != 0 {
		t.Fatalf("missing file: %v, %d", err, len(items))
	}

	// Arquivo válido → itens ordenados por ID.
	e := NewPromotionEngine()
	_ = e.Register(&KnowledgeItem{ID: "K-02", Title: "b"})
	_ = e.Register(&KnowledgeItem{ID: "K-01", Title: "a"})
	path := filepath.Join(t.TempDir(), "laws.json")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	items, err = ListKnowledgeItems(path)
	if err != nil || len(items) != 2 {
		t.Fatalf("list: %v, %d", err, len(items))
	}
	if items[0].ID != "K-01" || items[1].ID != "K-02" {
		t.Fatalf("not sorted: %v", items)
	}
}

func TestRepairOrphanVectors(t *testing.T) {
	// Engine mínimo: RepairOrphanVectors não deve panicar.
	e := newTestEngine(t, filepath.Join(t.TempDir(), "k.db"))
	e.RepairOrphanVectors() // no-op ou reparo; não pode panicar
	if got := e.RootDir(); got == "" {
		t.Fatal("RootDir vazio")
	}
}

func TestFederatedPackageStore(t *testing.T) {
	// PackageStore global: caminho derivado de diretórios conhecidos.
	s, err := NewGlobalPackageStore()
	if err != nil {
		t.Fatalf("NewGlobalPackageStore: %v", err)
	}
	if s.Path() == "" {
		t.Fatal("Path vazio")
	}
	if s.Exists("definitely-not-a-package-xyz") {
		t.Log("Exists retornou true (ambiente pode ter pacotes globais)")
	}
	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	t.Logf("List retornou %d pacotes globais", len(list))
	_ = os.Getenv
}
