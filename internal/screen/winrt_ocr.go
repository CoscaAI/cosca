package screen

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
)

// WinRTOCR é o motor de OCR nativo do Windows (Windows.Media.Ocr), acessado via
// um subprocesso PowerShell que interage com o WinRT. É 100% nativo (nenhum
// Tesseract, nenhum binário externo além do powershell.exe que já existe no
// Windows), sem custo, e reconhece os idiomas do perfil do usuário (pt-BR).
//
// O motor é plugável atrás da interface OCRProvider: o cosca screen não sabe
// (nem precisa saber) se o OCR é WinRT, Tesseract ou outro.
type WinRTOCR struct {
	// powershell é o caminho do powershell.exe (override para testes).
	powershell string
	// scriptPath é o caminho do script PowerShell de OCR (override para testes).
	scriptPath string
}

// NewWinRTOCR cria o motor OCR nativo do Windows.
func NewWinRTOCR() *WinRTOCR {
	return &WinRTOCR{powershell: "powershell"}
}

// Recognize lê o texto da imagem via WinRT OCR e preenche as regiões.
func (w *WinRTOCR) Recognize(ctx context.Context, img image.Image, regions []Region) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Salva a imagem em PNG temporário.
	tmp, err := os.CreateTemp("", "cosca-ocr-*.png")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := png.Encode(tmp, img); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	// Roda o PowerShell que faz o OCR e devolve JSON. O script é escrito em um
	// arquivo temporário .ps1 e executado com -File — mais confiável que passar
	// a string longa multi-linha via -Command (que o Windows pode quebrar no
	// escape). Passa o caminho da imagem via env.
	scriptFile, err := os.CreateTemp("", "cosca-ocr-*.ps1")
	if err != nil {
		return err
	}
	scriptPath := scriptFile.Name()
	defer os.Remove(scriptPath)
	if _, err := scriptFile.WriteString(winRTOCRScript); err != nil {
		scriptFile.Close()
		return err
	}
	if err := scriptFile.Close(); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, w.powershell, "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
	cmd.Env = append(os.Environ(), "COSCA_OCR_IMG="+tmp.Name())
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("winrt ocr: %w", err)
	}

	// Parse do JSON: linhas de texto com bounding boxes.
	var lines []ocrLine
	if err := json.Unmarshal(out, &lines); err != nil {
		return fmt.Errorf("winrt ocr: parse: %w", err)
	}
	return applyOCRToRegions(lines, regions)
}

// ocrLine é uma linha de texto reconhecida, com sua caixa e confiança.
type ocrLine struct {
	Text       string `json:"text"`
	Left       int    `json:"left"`
	Top        int    `json:"top"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Confidence float32 `json:"confidence"`
}

// applyOCRToRegions associa cada linha OCR à região de texto correspondente
// (por sobreposição de bounding box). Regiões sem linha candidata ficam sem
// texto (degradação graciosa).
func applyOCRToRegions(lines []ocrLine, regions []Region) error {
	for i := range regions {
		if regions[i].Kind != RegionText {
			continue
		}
		// Encontra a linha OCR que mais se sobrepõe à região.
		best := ""
		bestConf := float32(0)
		bestOverlap := 0
		for _, ln := range lines {
			overlap := rectOverlap(regions[i].BBox, Rect{X: ln.Left, Y: ln.Top, W: ln.Width, H: ln.Height})
			if overlap > bestOverlap || (overlap > 0 && ln.Confidence > bestConf) {
				if overlap > 0 {
					bestOverlap = overlap
					best = ln.Text
					bestConf = ln.Confidence
				}
			}
		}
		regions[i].Text = best
		regions[i].TextConfidence = bestConf
	}
	return nil
}

// rectOverlap devolve a área de interseção entre dois retângulos.
func rectOverlap(a, b Rect) int {
	x := minInt(a.X+a.W, b.X+b.W) - maxInt(a.X, b.X)
	y := minInt(a.Y+a.H, b.Y+b.H) - maxInt(a.Y, b.Y)
	if x <= 0 || y <= 0 {
		return 0
	}
	return x * y
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// winRTOCRScript é o script PowerShell que faz o OCR nativo e emite JSON.
// Recebe o caminho da imagem via $env:COSCA_OCR_IMG.
// NOTA: é uma string interpretada (\"...\") porque precisa conter o backtick de
// aridade genérica do .NET ('IAsyncOperation`1') — impossível num raw string.
var winRTOCRScript = "$ErrorActionPreference = 'Stop'\n" +
	"Add-Type -AssemblyName System.Runtime.WindowsRuntime\n" +
	"\n" +
	"$null = [Windows.Media.Ocr.OcrEngine,Windows.Foundation,ContentType=WindowsRuntime]\n" +
	"$null = [Windows.Media.Ocr.OcrResult,Windows.Foundation,ContentType=WindowsRuntime]\n" +
	"$null = [Windows.Graphics.Imaging.BitmapDecoder,Windows.Graphics.Imaging,ContentType=WindowsRuntime]\n" +
	"$null = [Windows.Storage.StorageFile,Windows.Storage,ContentType=WindowsRuntime]\n" +
	"$null = [Windows.Storage.Streams.IRandomAccessStream,Windows.Storage.Streams,ContentType=WindowsRuntime]\n" +
	"\n" +
	"$asTaskGeneric = ([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object {\n" +
	"    $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and\n" +
	"    $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation`1'\n" +
	"})[0]\n" +
	"function Await($WinRtTask, $ResultType) {\n" +
	"    $asTask = $asTaskGeneric.MakeGenericMethod($ResultType)\n" +
	"    $netTask = $asTask.Invoke($null, @($WinRtTask))\n" +
	"    $netTask.Wait(-1) | Out-Null\n" +
	"    $netTask.Result\n" +
	"}\n" +
	"\n" +
	"$pngPath = $env:COSCA_OCR_IMG\n" +
	"$file = Await ([Windows.Storage.StorageFile]::GetFileFromPathAsync($pngPath)) ([Windows.Storage.StorageFile])\n" +
	"$stream = Await ($file.OpenAsync([Windows.Storage.FileAccessMode]::Read)) ([Windows.Storage.Streams.IRandomAccessStream])\n" +
	"$decoder = Await ([Windows.Graphics.Imaging.BitmapDecoder]::CreateAsync($stream)) ([Windows.Graphics.Imaging.BitmapDecoder])\n" +
	"$soft = Await ($decoder.GetSoftwareBitmapAsync()) ([Windows.Graphics.Imaging.SoftwareBitmap])\n" +
	"\n" +
	"$engine = [Windows.Media.Ocr.OcrEngine]::TryCreateFromUserProfileLanguages()\n" +
	"$result = Await ($engine.RecognizeAsync($soft)) ([Windows.Media.Ocr.OcrResult])\n" +
	"\n" +
	"$lines = @()\n" +
	"foreach ($line in $result.Lines) {\n" +
	"    if ($line.Text.Trim() -eq '') { continue }\n" +
	"    $minX = [double]::MaxValue; $minY = [double]::MaxValue\n" +
	"    $maxX = -1.0; $maxY = -1.0\n" +
	"    foreach ($wd in $line.Words) {\n" +
	"        $r = $wd.BoundingRect\n" +
	"        $x = [double]$r.X; $y = [double]$r.Y\n" +
	"        $w = [double]$r.Width; $h = [double]$r.Height\n" +
	"        if ($x -lt $minX) { $minX = $x }\n" +
	"        if ($y -lt $minY) { $minY = $y }\n" +
	"        if (($x + $w) -gt $maxX) { $maxX = $x + $w }\n" +
	"        if (($y + $h) -gt $maxY) { $maxY = $y + $h }\n" +
	"    }\n" +
	"    if ($maxX -lt 0) { $maxX = 0; $maxY = 0 }\n" +
	"    $lines += @{ text = $line.Text; left = [int]$minX; top = [int]$minY; width = [int]($maxX - $minX); height = [int]($maxY - $minY); confidence = 1.0 }\n" +
	"}\n" +
	"$stream.Dispose()\n" +
	"$lines | ConvertTo-Json -Compress\n"
