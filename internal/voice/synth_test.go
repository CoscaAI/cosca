package voice

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBancoSintetizar(t *testing.T) {
	dir := t.TempDir()
	// banco mínimo: dífonos k_o e o_S
	os.WriteFile(filepath.Join(dir, "k_o.wav"), wav16(300), 0o644)
	os.WriteFile(filepath.Join(dir, "o_S.wav"), wav16(300), 0o644)
	os.WriteFile(filepath.Join(dir, "S_k.wav"), wav16(300), 0o644)

	b, err := AbrirBanco(dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.Cobertura() != 3 {
		t.Fatalf("cobertura = %d, want 3", b.Cobertura())
	}
	if !b.Tem("k", "o") {
		t.Error("Tem(k,o) = false")
	}
	// síntese com prosódia (estilo default)
	out, err := b.Sintetizar("cosca", StyleCoscaExecutive)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("síntese vazia")
	}
	// com dífono ausente: fail-soft (nunca falha)
	out2, err := b.Sintetizar("xyz", StyleCoscaExecutive)
	if err != nil {
		t.Fatal(err)
	}
	if len(out2) == 0 {
		t.Fatal("síntese com ausentes vazia")
	}
}

func TestEscreverWAV16(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.wav")
	smp := []float32{0.1, -0.2, 0.3}
	if err := EscreverWAV16(path, smp); err != nil {
		t.Fatal(err)
	}
	got, err := lerWAV16(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
}

// wav16: gera WAV PCM16 mono com n amostras.
func wav16(n int) []byte {
	raw := make([]byte, 44+n*2)
	copy(raw[0:4], "RIFF")
	raw[4] = byte((36 + n*2) & 0xff)
	copy(raw[8:12], "WAVE")
	copy(raw[12:16], "fmt ")
	raw[16] = 16
	raw[20] = 1
	raw[22] = 1
	raw[24] = 0x80
	raw[25] = 0xbb
	raw[28] = 0x00
	raw[29] = 0xbb
	raw[32] = 2
	raw[34] = 16
	copy(raw[36:40], "data")
	for i := 0; i < n; i++ {
		raw[44+i*2] = byte(i % 256)
	}
	return raw
}