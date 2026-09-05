//
// Tests for the Acquisition Budget (internal/acquisition/budget.go).
//
// Cobre:
//   - Defaults do Don (Fontes: 8, Arquivos: 100, Rede: 20MB, Tempo: 30s, IA: 8k)
//   - NewAcquisitionTracker assume defaults com orçamento zero-valued
//   - Record* acumula cada dimensão
//   - Exceeded por dimensão + WhichExceeded (nomes pt-BR)
//   - Summary (dentro + mensagem de confiança quando excedido)
//   - Client.WithTracker: fetch contra httptest registra bytes/fontes/arquivos;
//     tracker estourado → erro; tracker nil → comportamento inalterado
//   - Regressão: aquisição sem tracker roda exatamente como antes
//

package acquisition

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// Defaults
// =============================================================================

func TestDefaultAcquisitionBudget(t *testing.T) {
	b := DefaultAcquisitionBudget()
	if b.MaxSources != 8 {
		t.Errorf("MaxSources = %d, want 8", b.MaxSources)
	}
	if b.MaxFiles != 100 {
		t.Errorf("MaxFiles = %d, want 100", b.MaxFiles)
	}
	if b.MaxNetworkMB != 20 {
		t.Errorf("MaxNetworkMB = %d, want 20", b.MaxNetworkMB)
	}
	if b.MaxTime != 30*time.Second {
		t.Errorf("MaxTime = %v, want 30s", b.MaxTime)
	}
	if b.MaxAITokens != 8000 {
		t.Errorf("MaxAITokens = %d, want 8000", b.MaxAITokens)
	}
}

func TestNewAcquisitionTracker_ZeroBudgetDefaults(t *testing.T) {
	tracker := NewAcquisitionTracker(AcquisitionBudget{})
	if tracker.budget.MaxSources != 8 {
		t.Error("zero-valued budget deveria assumir MaxSources=8")
	}
	if tracker.budget.MaxNetworkMB != 20 {
		t.Error("zero-valued budget deveria assumir MaxNetworkMB=20")
	}
	// Um orçamento parcialmente preenchido NÃO recebe defaults.
	partial := NewAcquisitionTracker(AcquisitionBudget{MaxSources: 3})
	if partial.budget.MaxSources != 3 {
		t.Error("orçamento parcial não deveria ser sobrescrito")
	}
	if partial.budget.MaxFiles != 0 {
		t.Error("orçamento parcial deveria manter as demais dimensões zero")
	}
}

// =============================================================================
// Record* acumula
// =============================================================================

func TestAcquisitionTracker_RecordAccumulates(t *testing.T) {
	tracker := NewAcquisitionTracker(DefaultAcquisitionBudget())
	tracker.RecordSource()
	tracker.RecordSource()
	tracker.RecordSource()
	tracker.RecordFile()
	tracker.RecordFile()
	tracker.RecordBytes(100)
	tracker.RecordBytes(50)
	tracker.RecordBytes(-1) // negativos ignorados
	tracker.RecordDuration(2 * time.Second)
	tracker.RecordDuration(500 * time.Millisecond)
	tracker.RecordAITokens(1500)
	tracker.RecordAITokens(-10) // negativos ignorados

	s := tracker.Spent()
	if s.Sources != 3 {
		t.Errorf("Sources = %d, want 3", s.Sources)
	}
	if s.Files != 2 {
		t.Errorf("Files = %d, want 2", s.Files)
	}
	if s.Bytes != 150 {
		t.Errorf("Bytes = %d, want 150", s.Bytes)
	}
	if s.Duration != 2500*time.Millisecond {
		t.Errorf("Duration = %v, want 2.5s", s.Duration)
	}
	if s.AITokens != 1500 {
		t.Errorf("AITokens = %d, want 1500", s.AITokens)
	}
}

// =============================================================================
// Exceeded per dimension + WhichExceeded
// =============================================================================

func TestAcquisitionTracker_ExceededPerDimension(t *testing.T) {
	b := DefaultAcquisitionBudget()

	cases := []struct {
		name  string
		setup func(t *AcquisitionTracker)
		want  []string
	}{
		{"fontes", func(tr *AcquisitionTracker) { tr.RecordSource() }, nil},
		{"fontes", func(tr *AcquisitionTracker) {
			for i := 0; i < 9; i++ {
				tr.RecordSource()
			}
		}, []string{"fontes"}},
		{"arquivos", func(tr *AcquisitionTracker) {
			for i := 0; i < 101; i++ {
				tr.RecordFile()
			}
		}, []string{"arquivos"}},
		{"rede", func(tr *AcquisitionTracker) { tr.RecordBytes(b.MaxNetworkMB<<20 + 1) }, []string{"rede"}},
		{"rede no limite", func(tr *AcquisitionTracker) { tr.RecordBytes(b.MaxNetworkMB << 20) }, nil},
		{"tempo", func(tr *AcquisitionTracker) { tr.RecordDuration(b.MaxTime + time.Second) }, []string{"tempo"}},
		{"tempo no limite", func(tr *AcquisitionTracker) { tr.RecordDuration(b.MaxTime) }, nil},
		{"tokens IA", func(tr *AcquisitionTracker) { tr.RecordAITokens(b.MaxAITokens + 1) }, []string{"tokens IA"}},
		{"múltiplas", func(tr *AcquisitionTracker) {
			for i := 0; i < 9; i++ {
				tr.RecordSource()
			}
			tr.RecordBytes(b.MaxNetworkMB<<20 + 1)
			tr.RecordAITokens(b.MaxAITokens + 1)
		}, []string{"fontes", "rede", "tokens IA"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tr := NewAcquisitionTracker(b)
			tc.setup(tr)

			wantExceeded := len(tc.want) > 0
			if tr.Exceeded() != wantExceeded {
				t.Errorf("Exceeded() = %v, want %v (spent %+v)", tr.Exceeded(), wantExceeded, tr.Spent())
			}
			got := tr.WhichExceeded()
			if len(got) != len(tc.want) {
				t.Fatalf("WhichExceeded() = %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("WhichExceeded()[%d] = %q, want %q (got %v)", i, got[i], tc.want[i], got)
				}
			}
		})
	}
}

// =============================================================================
// Summary
// =============================================================================

func TestAcquisitionTracker_Summary_Within(t *testing.T) {
	tr := NewAcquisitionTracker(DefaultAcquisitionBudget())
	tr.RecordSource()
	tr.RecordFile()
	tr.RecordBytes(3 << 20)
	tr.RecordDuration(1500 * time.Millisecond)
	tr.RecordAITokens(2000)

	summary := tr.Summary()
	for _, want := range []string{
		"Fontes: 1/8",
		"Arquivos: 1/100",
		"Rede: 3/20MB",
		"Tempo: 1s/30s",
		"IA: 2/8k",
	} {
		if !strings.Contains(summary, want) {
			t.Errorf("Summary missing %q: %q", want, summary)
		}
	}
	if strings.Contains(summary, "Não consegui validar") {
		t.Errorf("Summary dentro do orçamento não deveria avisar: %q", summary)
	}
}

func TestAcquisitionTracker_Summary_Exceeded(t *testing.T) {
	tr := NewAcquisitionTracker(DefaultAcquisitionBudget())
	for i := 0; i < 9; i++ {
		tr.RecordSource()
	}

	summary := tr.Summary()
	if !strings.Contains(summary, "Fontes: 9/8") {
		t.Errorf("Summary deveria mostrar o estouro de fontes: %q", summary)
	}
	if !strings.Contains(summary, "Não consegui validar com confiança dentro do orçamento. Preciso da sua decisão.") {
		t.Errorf("Summary excedido deveria pedir decisão do Don: %q", summary)
	}
}

// =============================================================================
// Client.WithTracker — integração no fetch
// =============================================================================

func TestClient_WithTrackerRecordsBytesSourcesFiles(t *testing.T) {
	content := []byte("evidência externa com bytes contáveis")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	}))
	defer srv.Close()

	tr := NewAcquisitionTracker(DefaultAcquisitionBudget())
	c := newTestClient(true).WithTracker(tr)

	art, body, err := c.FetchAll(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FetchAll: %v", err)
	}
	if art.SHA256 == "" || len(body) != len(content) {
		t.Fatalf("fetch com tracker deveria retornar artefato normal: %+v", art)
	}

	s := tr.Spent()
	if s.Sources != 1 {
		t.Errorf("Sources = %d, want 1", s.Sources)
	}
	if s.Files != 1 {
		t.Errorf("Files = %d, want 1", s.Files)
	}
	if s.Bytes != int64(len(content)) {
		t.Errorf("Bytes = %d, want %d", s.Bytes, len(content))
	}
	if tr.Exceeded() {
		t.Errorf("um fetch pequeno não deveria estourar: %v", tr.WhichExceeded())
	}

	// Fetch simples também contabiliza (Fetch delega a FetchAll).
	art2, err := c.Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if art2.SHA256 != art.SHA256 {
		t.Error("hash divergente entre Fetch/FetchAll")
	}
	s = tr.Spent()
	if s.Sources != 2 || s.Files != 2 {
		t.Errorf("após 2 fetches: Sources=%d Files=%d, want 2/2", s.Sources, s.Files)
	}
	if s.Bytes != 2*int64(len(content)) {
		t.Errorf("Bytes = %d, want %d", s.Bytes, 2*int64(len(content)))
	}
}

func TestClient_WithTrackerExceededReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// Orçamento sem rede (0MB) — qualquer byte estoura. Não é zero-valued
	// porque as demais dimensões estão preenchidas (defaults não se aplicam).
	tr := NewAcquisitionTracker(AcquisitionBudget{
		MaxSources:   8,
		MaxFiles:     100,
		MaxNetworkMB: 0,
		MaxTime:      30 * time.Second,
		MaxAITokens:  8000,
	})
	c := newTestClient(true).WithTracker(tr)

	_, _, err := c.FetchAll(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("esperava erro de orçamento excedido")
	}
	if !strings.Contains(err.Error(), "orçamento de aquisição excedido") {
		t.Errorf("erro deveria mencionar o orçamento: %v", err)
	}
	if !strings.Contains(err.Error(), "rede") {
		t.Errorf("erro deveria listar a dimensão estourada (rede): %v", err)
	}
	// O consumo foi contabilizado mesmo abortando.
	s := tr.Spent()
	if s.Sources != 1 || s.Files != 1 || s.Bytes != 2 {
		t.Errorf("consumo deveria ser contabilizado antes do abort: %+v", s)
	}
}

// =============================================================================
// Regressão — tracker nil (comportamento atual inalterado)
// =============================================================================

func TestClient_NilTrackerUnchanged(t *testing.T) {
	content := []byte("hello cosca external evidence\n")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(content)
	}))
	defer srv.Close()

	// Comportamento padrão: fetch normal, sem contabilização.
	c := NewClient(0, 0, 0)
	c.AllowRemote = true
	c.AllowLoopback = true
	art, body, err := c.FetchAll(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FetchAll sem tracker: %v", err)
	}
	if art.SizeBytes != int64(len(content)) || len(body) != len(content) {
		t.Fatalf("fetch sem tracker deveria devolver corpo completo: %+v", art)
	}
	if c.Tracker != nil {
		t.Error("tracker deveria permanecer nil")
	}

	// Com tracker nil explícito, o resultado é idêntico.
	withNil := newTestClient(true).WithTracker(nil)
	if withNil.Tracker != nil {
		t.Error("WithTracker(nil) deveria manter Tracker nil")
	}
	art2, _, err := withNil.FetchAll(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FetchAll com tracker nil: %v", err)
	}
	if art2.SHA256 != art.SHA256 {
		t.Error("hash deveria ser idêntico com tracker nil")
	}
}

// =============================================================================
// compile-time guard
// =============================================================================

var _ = time.Second
