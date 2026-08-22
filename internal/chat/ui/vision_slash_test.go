package ui

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRenderVision_ReadsImage verifica o fluxo /ver de ponta a ponta:
// imagem → OCR → texto apresentado na TUI.
func TestRenderVision_ReadsImage(t *testing.T) {
	if _, err := exec.LookPath("tesseract"); err != nil {
		t.Skipf("tesseract not found: %v", err)
	}
	dir := t.TempDir()
	img := filepath.Join(dir, "painel.jpg")
	// Cria imagem com texto via Pillow.
	script := "from PIL import Image, ImageDraw; " +
		"img=Image.new('RGB',(600,120),'white'); " +
		"d=ImageDraw.Draw(img); " +
		"d.text((20,40), 'PAINEL INVERSOR CFW10', fill='black'); " +
		"img.save('" + img + "')"
	if out, err := exec.Command("python3", "-c", script).CombinedOutput(); err != nil {
		t.Skipf("pillow unavailable: %v %s", err, out)
	}

	out := renderVision(img)
	if strings.Contains(out, "Não consegui enxergar") {
		t.Fatalf("renderVision falhou: %s", out)
	}
	if !strings.Contains(out, "INVERSOR") {
		t.Errorf("output deve conter o texto OCR, got: %s", out)
	}
	if !strings.Contains(out, "👁") {
		t.Errorf("output deve ter o indicador visual: %s", out)
	}
	t.Logf("output: %s", out)
}

// TestRenderVision_MissingFile verifica erro amigável para arquivo inexistente.
func TestRenderVision_MissingFile(t *testing.T) {
	out := renderVision("/nonexistent/img.png")
	if !strings.Contains(out, "Não consegui enxergar") {
		t.Errorf("expected friendly error, got: %s", out)
	}
}

// TestRenderVision_NoArg verifica a mensagem de uso.
func TestRenderVision_NoArg(t *testing.T) {
	out := renderSlashResult(Model{}, "/ver")
	if !strings.Contains(out, "Uso: /ver") {
		t.Errorf("expected usage message, got: %s", out)
	}
}
