// Package aitask implements the AI Task Engine (§4 do manifesto
// Creative/Scientific/Media) — Fase 1, etapa 1.3.
//
// Princípio P3 do Blueprint: "Centralize a definição, não a execução".
// Este package NORMALIZA as 18 tarefas canônicas do manifesto em um enum
// único com metadados (modalidades, hardware, executores candidatos) — a
// espinha dorsal sobre a qual Model Registry (1.4) e Node Graph (1.6) vão
// plugar executores. Nada aqui invoca um modelo; apenas define o contrato.
package aitask

import (
	"fmt"
	"sort"
	"strings"
)

// Type é o identificador canônico de uma tarefa de IA (§4).
type Type string

// As 18 tarefas canônicas do manifesto §4.
const (
	// TextGeneration — geração de texto (LLM).
	TextGeneration Type = "text_generation"
	// ImageGeneration — geração de imagem (diffusion/DiT).
	ImageGeneration Type = "image_generation"
	// VideoGeneration — geração de vídeo.
	VideoGeneration Type = "video_generation"
	// AudioGeneration — geração de áudio (música, sons, efeitos).
	AudioGeneration Type = "audio_generation"
	// SpeechToText — transcrição de fala (ASR).
	SpeechToText Type = "speech_to_text"
	// TextToSpeech — síntese de voz (TTS).
	TextToSpeech Type = "text_to_speech"
	// ImageAnalysis — análise/descrição de imagem (VLM).
	ImageAnalysis Type = "image_analysis"
	// VideoAnalysis — análise/descrição de vídeo.
	VideoAnalysis Type = "video_analysis"
	// AudioAnalysis — análise/descrição de áudio.
	AudioAnalysis Type = "audio_analysis"
	// OCR — extração de texto de documentos/imagens.
	OCR Type = "ocr"
	// Segmentation — segmentação de imagem (SAM2, rembg).
	Segmentation Type = "segmentation"
	// Upscale — super-resolução (Real-ESRGAN).
	Upscale Type = "upscale"
	// Restoration — restauração de mídia (denoise, deblur, colorize).
	Restoration Type = "restoration"
	// Translation — tradução de texto/fala.
	Translation Type = "translation"
	// Embedding — vetores de embedding (texto/imagem).
	Embedding Type = "embedding"
	// Classification — classificação (texto/imagem/áudio).
	Classification Type = "classification"
	// Detection — detecção de objetos/regiões em imagem/vídeo.
	Detection Type = "detection"
	// Simulation — simulação (científica/física/agentes).
	Simulation Type = "simulation"
)

// AllTypes lista as 18 tarefas canônicas em ordem estável.
var AllTypes = []Type{
	TextGeneration, ImageGeneration, VideoGeneration, AudioGeneration,
	SpeechToText, TextToSpeech, ImageAnalysis, VideoAnalysis, AudioAnalysis,
	OCR, Segmentation, Upscale, Restoration, Translation, Embedding,
	Classification, Detection, Simulation,
}

// Valid reports se o tipo é canônico.
func (t Type) Valid() bool {
	for _, a := range AllTypes {
		if a == t {
			return true
		}
	}
	return false
}

// TypesList devolve a lista legível de tarefas.
func TypesList() string {
	parts := make([]string, len(AllTypes))
	for i, t := range AllTypes {
		parts[i] = string(t)
	}
	return strings.Join(parts, ", ")
}

// Modality é uma modalidade de entrada/saída.
type Modality string

// Modalidades suportadas.
const (
	ModText   Modality = "text"
	ModImage  Modality = "image"
	ModVideo  Modality = "video"
	ModAudio  Modality = "audio"
	ModData   Modality = "data"
	ModVector Modality = "vector"
)

// HardwareClass classifica onde a tarefa prefere rodar (§17/§28).
type HardwareClass string

// Classes de hardware.
const (
	// HWCpu — roda bem em CPU.
	HWCpu HardwareClass = "cpu"
	// HWGPU — aceleração GPU fortemente recomendada (ROCm/Vulkan/OpenCL).
	HWGPU HardwareClass = "gpu"
	// HWAny — flexível (CPU ou GPU conforme disponibilidade).
	HWAny HardwareClass = "any"
	// HWRemote — tipicamente servida por API remota.
	HWRemote HardwareClass = "remote"
)

// ExecutorHint sugere implementações candidatas (não é dependência — é
// conhecimento do terreno, pesquisa da Fase 0). A execução real pluga via
// Model Registry (1.4).
type ExecutorHint struct {
	// Name do executor/modelo candidato.
	Name string `json:"name" yaml:"name"`
	// Kind: "local" (ROCM/CPU) ou "api" (remoto).
	Kind string `json:"kind" yaml:"kind"`
	// Notes de licença/compatibilidade (gfx1030, AGPL, ...).
	Notes string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

// Task descreve uma tarefa canônica.
type Task struct {
	Type         Type            `json:"type" yaml:"type"`
	Input        Modality        `json:"input" yaml:"input"`
	Output       Modality        `json:"output" yaml:"output"`
	Hardware     HardwareClass   `json:"hardware" yaml:"hardware"`
	Description  string          `json:"description" yaml:"description"`
	Executors    []ExecutorHint  `json:"executors" yaml:"executors"`
	RequiresModel bool           `json:"requires_model" yaml:"requires_model"`
}

// Catalog é o registro canônico das 18 tarefas.
var Catalog = map[Type]Task{
	TextGeneration: {
		Type: TextGeneration, Input: ModText, Output: ModText, Hardware: HWRemote,
		Description: "Geração de texto via LLM (chat, escrita, código, raciocínio).",
		Executors: []ExecutorHint{
			{Name: "deepseek-v4-flash", Kind: "api", Notes: "provider padrão da família"},
			{Name: "llama.cpp", Kind: "local", Notes: "GGUF, CPU/ROCm"},
			{Name: "ollama", Kind: "local", Notes: "servidor local"},
		},
		RequiresModel: true,
	},
	ImageGeneration: {
		Type: ImageGeneration, Input: ModText, Output: ModImage, Hardware: HWGPU,
		Description: "Geração de imagem a partir de texto (diffusion).",
		Executors: []ExecutorHint{
			{Name: "stable-diffusion", Kind: "local", Notes: "PyTorch-ROCm gfx1030"},
			{Name: "diffusers", Kind: "local", Notes: "HF pipeline"},
			{Name: "flux", Kind: "api", Notes: "remoto"},
		},
		RequiresModel: true,
	},
	VideoGeneration: {
		Type: VideoGeneration, Input: ModText, Output: ModVideo, Hardware: HWGPU,
		Description: "Geração de vídeo a partir de texto/imagens.",
		Executors: []ExecutorHint{
			{Name: "wan/rife-based", Kind: "local", Notes: "PyTorch-ROCm, ainda experimental"},
			{Name: "veo", Kind: "api", Notes: "remoto"},
		},
		RequiresModel: true,
	},
	AudioGeneration: {
		Type: AudioGeneration, Input: ModText, Output: ModAudio, Hardware: HWAny,
		Description: "Geração de áudio/música/sons.",
		Executors: []ExecutorHint{
			{Name: "audiocraft", Kind: "local", Notes: "Meta, PyTorch"},
			{Name: "piper", Kind: "local", Notes: "TTS leve, CPU"},
		},
		RequiresModel: true,
	},
	SpeechToText: {
		Type: SpeechToText, Input: ModAudio, Output: ModText, Hardware: HWAny,
		Description: "Transcrição de fala (ASR) — pipeline de legendas.",
		Executors: []ExecutorHint{
			{Name: "whisper", Kind: "local", Notes: "openai/whisper, PyTorch-ROCm"},
			{Name: "faster-whisper", Kind: "local", Notes: "CTranslate2, CPU eficiente"},
			{Name: "whisper.cpp", Kind: "local", Notes: "C++, CPU/Vulkan"},
		},
		RequiresModel: true,
	},
	TextToSpeech: {
		Type: TextToSpeech, Input: ModText, Output: ModAudio, Hardware: HWCpu,
		Description: "Síntese de voz (TTS) — narração.",
		Executors: []ExecutorHint{
			{Name: "piper", Kind: "local", Notes: "leve, CPU, MIT"},
		},
		RequiresModel: true,
	},
	ImageAnalysis: {
		Type: ImageAnalysis, Input: ModImage, Output: ModText, Hardware: HWRemote,
		Description: "Análise/descrição de imagem (VLM).",
		Executors: []ExecutorHint{
			{Name: "vlm-remoto", Kind: "api", Notes: "gpt-4o/claude vision"},
		},
		RequiresModel: true,
	},
	VideoAnalysis: {
		Type: VideoAnalysis, Input: ModVideo, Output: ModText, Hardware: HWRemote,
		Description: "Análise de vídeo (frames + áudio + contexto).",
		Executors: []ExecutorHint{
			{Name: "ffmpeg+whisper+vlm", Kind: "local", Notes: "pipeline próprio"},
		},
		RequiresModel: true,
	},
	AudioAnalysis: {
		Type: AudioAnalysis, Input: ModAudio, Output: ModText, Hardware: HWAny,
		Description: "Análise de áudio (eventos, tonalidade, fala).",
		Executors: []ExecutorHint{
			{Name: "librosa", Kind: "local", Notes: "features, sem modelo"},
			{Name: "whisper", Kind: "local", Notes: "para fala"},
		},
		RequiresModel: false,
	},
	OCR: {
		Type: OCR, Input: ModImage, Output: ModText, Hardware: HWCpu,
		Description: "Extração de texto de imagens/documentos.",
		Executors: []ExecutorHint{
			{Name: "tesseract", Kind: "local", Notes: "já instalado, leve"},
			{Name: "paddleocr", Kind: "local", Notes: "SOTA, mais pesado"},
		},
		RequiresModel: false,
	},
	Segmentation: {
		Type: Segmentation, Input: ModImage, Output: ModImage, Hardware: HWGPU,
		Description: "Segmentação de imagem (fundo/objetos).",
		Executors: []ExecutorHint{
			{Name: "rembg", Kind: "local", Notes: "já instalado (cosca-media)"},
			{Name: "sam2", Kind: "local", Notes: "segment anything, PyTorch"},
		},
		RequiresModel: true,
	},
	Upscale: {
		Type: Upscale, Input: ModImage, Output: ModImage, Hardware: HWGPU,
		Description: "Super-resolução de imagem.",
		Executors: []ExecutorHint{
			{Name: "realesrgan", Kind: "local", Notes: "PyTorch-ROCm"},
		},
		RequiresModel: true,
	},
	Restoration: {
		Type: Restoration, Input: ModImage, Output: ModImage, Hardware: HWAny,
		Description: "Restauração (denoise, deblur, colorize, reparo).",
		Executors: []ExecutorHint{
			{Name: "realesrgan", Kind: "local", Notes: "também faz restauração"},
		},
		RequiresModel: true,
	},
	Translation: {
		Type: Translation, Input: ModText, Output: ModText, Hardware: HWRemote,
		Description: "Tradução de texto (e fala via ASR+TTS).",
		Executors: []ExecutorHint{
			{Name: "llm-remoto", Kind: "api", Notes: "qualquer LLM"},
		},
		RequiresModel: true,
	},
	Embedding: {
		Type: Embedding, Input: ModText, Output: ModVector, Hardware: HWCpu,
		Description: "Vetores de embedding (texto/imagem) para busca semântica.",
		Executors: []ExecutorHint{
			{Name: "tf-idf", Kind: "local", Notes: "fallback offline, já integrado"},
			{Name: "ollama", Kind: "local", Notes: "nomic-embed-text"},
		},
		RequiresModel: false,
	},
	Classification: {
		Type: Classification, Input: ModText, Output: ModText, Hardware: HWAny,
		Description: "Classificação (texto/imagem/áudio em categorias).",
		Executors: []ExecutorHint{
			{Name: "llm-remoto", Kind: "api", Notes: "zero-shot"},
		},
		RequiresModel: true,
	},
	Detection: {
		Type: Detection, Input: ModImage, Output: ModImage, Hardware: HWGPU,
		Description: "Detecção de objetos/regiões em imagem/vídeo.",
		Executors: []ExecutorHint{
			{Name: "grounding-dino", Kind: "local", Notes: "open-vocabulary"},
			{Name: "yolo", Kind: "local", Notes: "clássico, rápido"},
		},
		RequiresModel: true,
	},
	Simulation: {
		Type: Simulation, Input: ModData, Output: ModData, Hardware: HWCpu,
		Description: "Simulação (científica, física, agentes) — sem modelo de IA necessariamente.",
		Executors: []ExecutorHint{
			{Name: "numpy/scipy", Kind: "local", Notes: "núcleo numérico"},
		},
		RequiresModel: false,
	},
}

// Get devolve a descrição de uma tarefa. Panic-free: false se inválida.
func Get(t Type) (Task, bool) {
	task, ok := Catalog[t]
	return task, ok
}

// MustGet devolve a tarefa; panic se inválida (uso em tempo de inicialização).
func MustGet(t Type) Task {
	task, ok := Catalog[t]
	if !ok {
		panic(fmt.Sprintf("aitask: unknown task type %q", t))
	}
	return task
}

// SortedTypes devolve as 18 tarefas ordenadas por nome (exibição estável).
func SortedTypes() []Type {
	types := append([]Type(nil), AllTypes...)
	sort.Slice(types, func(i, j int) bool { return types[i] < types[j] })
	return types
}
