// Package vision gives the Cosca Kernel eyes.
//
// Two backends:
//   - "llava": a local multimodal VLM (Ollama) that understands the SCENE
//     (objects, layout, state) — the real "sight", not just text.
//   - "tesseract": classic OCR (image → text). Kept as a lightweight fallback.
//
// Both run 100% local. The Kernel gets semantic vision via llava and can
// fall back to OCR when a raw text dump is all that's needed.
package vision

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Options configures a single vision pass.
type Options struct {
	// Backend selects the engine: "" or "auto" (try llava, then tesseract),
	// "llava" (VLM), or "tesseract" (OCR).
	Backend string
	// Lang is the tesseract language (e.g. "eng", "por", "eng+por").
	Lang string
	// PSM is the tesseract page segmentation mode (0-13).
	PSM int
	// Prompt is the instruction for the llava VLM. Empty = a default scene
	// description prompt.
	Prompt string
}

// Backends.
const (
	BackendAuto     = "auto"
	BackendLlava    = "llava"
	BackendTesseract = "tesseract"
)

// Ollama endpoint / model. Overridable for tests.
var (
	OllamaURL   = "http://localhost:11434"
	OllamaModel = "llava"
	tesseractBin = "tesseract"
)

const defaultScenePrompt = "Descreva a imagem em poucas linhas: objetos principais, layout e estado (ex.: personagem diante de casa, plantacao crescendo). Seja objetivo."

// Describe reads an image file and returns a text description of what it
// contains — the Kernel's "sight".
//
// DEFAULT = DETERMINÍSTICO (tesseract OCR), sem depender de VLM externa. A
// visão funciona 100% local/offline. O backend "llava" (VLM via Ollama) é
// OPT-IN: só é usado quando explicitamente pedido via Options.Backend="llava".
// Nunca é o default — a família não depende de serviço externo para enxergar.
func Describe(path string, opts Options) (Result, error) {
	if _, err := os.Stat(path); err != nil {
		return Result{}, fmt.Errorf("image not found: %w", err)
	}

	backend := opts.Backend
	switch backend {
	case BackendLlava:
		// Opt-in explícito: o Don pediu a VLM (Ollama+llava). Se indisponível,
		// NÃO degrada para tesseract silenciosamente — honestidade: reporta erro.
		return describeLlava(path, opts)
	default:
		// "" | "auto" | "tesseract" → determinístico (OCR). Sem VLM externa.
		backend = BackendTesseract
	}

	return describeTesseract(path, opts)
}

// OllamaResponse is the shape of /api/generate.
type OllamaResponse struct {
	Response string `json:"response"`
	Error    string `json:"error"`
}

// describeLlava runs the local VLM (Ollama) and returns a semantic description.
func describeLlava(path string, opts Options) (Result, error) {
	img, err := os.ReadFile(path)
	if err != nil {
		return Result{}, fmt.Errorf("read image: %w", err)
	}
	b64 := base64.StdEncoding.EncodeToString(img)
	prompt := opts.Prompt
	if strings.TrimSpace(prompt) == "" {
		prompt = defaultScenePrompt
	}

	payload, _ := json.Marshal(map[string]any{
		"model":  OllamaModel,
		"prompt": prompt,
		"images": []string{b64},
		"stream": false,
	})

	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Post(OllamaURL+"/api/generate", "application/json", bytes.NewReader(payload))
	if err != nil {
		return Result{}, fmt.Errorf("llava request failed: %w", err)
	}
	defer resp.Body.Close()

	var out OllamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Result{}, fmt.Errorf("llava decode failed: %w", err)
	}
	if out.Error != "" {
		return Result{}, fmt.Errorf("llava error: %s", out.Error)
	}

	info, _ := os.Stat(path)
	return Result{
		Path:      path,
		SizeBytes: info.Size(),
		Text:      strings.TrimSpace(out.Response),
	}, nil
}

// describeTesseract runs OCR via the local tesseract binary.
func describeTesseract(path string, opts Options) (Result, error) {
	args := []string{path, "stdout", "-l", defaultLang(opts.Lang)}
	if opts.PSM != 0 {
		args = append(args, "--psm", fmt.Sprintf("%d", opts.PSM))
	}

	cmd := exec.Command(tesseractBin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return Result{}, fmt.Errorf("tesseract failed: %v (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	info, _ := os.Stat(path)
	return Result{
		Path:      path,
		SizeBytes: info.Size(),
		Text:      strings.TrimSpace(stdout.String()),
	}, nil
}

// Result is the outcome of a vision pass (VLM description or OCR text).
type Result struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	Text      string `json:"text"`
}

// IsEmpty reports whether the vision pass found no readable content.
func (r Result) IsEmpty() bool {
	return strings.TrimSpace(r.Text) == ""
}

// Summary returns a compact one-line description of the result.
func (r Result) Summary() string {
	if r.IsEmpty() {
		return fmt.Sprintf("imagem %s (%s): nenhum texto detectado",
			filepath.Base(r.Path), formatBytes(r.SizeBytes))
	}
	return fmt.Sprintf("imagem %s (%s): %d caracteres OCR",
		filepath.Base(r.Path), formatBytes(r.SizeBytes), len(r.Text))
}

func defaultLang(lang string) string {
	if strings.TrimSpace(lang) == "" {
		return "eng"
	}
	return lang
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
