package voice

// ── Síntese concatenativa (synth.go) ────────────────────────────────────
// A Voz Cosca é concatenativa: o banco guarda DÍFONOS (transições
// f1→f2), não fonemas isolados — diretriz do Professor (diphone >
// phoneme: corta a robótica dos cortes artificiais).
//
// Pipeline: []Phoneme + ProsodyPlan → busca do dífono no banco →
// concatenação com crossfade → aplicação de duração (resample) e
// energia (envelope) → WAV/PCM.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	SampleRate = 24000
	fadeSamples = 90 // 3.75ms crossfade
)

// Banco: dífonos em memória (lazy).
type Banco struct {
	dir   string
	dif   map[string][]float32
	cache map[string][]float32
}

// AbrirBanco carrega o índice de dífonos.
func AbrirBanco(dir string) (*Banco, error) {
	b := &Banco{dir: dir, dif: map[string][]float32{}, cache: map[string][]float32{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".wav") {
			continue
		}
		smp, err := lerWAV16(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		b.dif[strings.TrimSuffix(e.Name(), ".wav")] = smp
	}
	if len(b.dif) == 0 {
		return nil, errors.New("banco de dífonos vazio em " + dir)
	}
	return b, nil
}

func (b *Banco) Tem(f1, f2 string) bool {
	_, ok := b.dif[f1+"_"+f2]
	return ok
}

func (b *Banco) Cobertura() int { return len(b.dif) }

func (b *Banco) DífonosDisponiveis() []string {
	out := make([]string, 0, len(b.dif))
	for k := range b.dif {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// lerWAV16: WAV PCM16 mono → []float32.
func lerWAV16(path string) ([]float32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 44 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, errors.New("não é WAV")
	}
	pos := 12
	var fmtChunk, dataStart, dataLen int
	for pos+8 <= len(data) {
		id := string(data[pos : pos+4])
		sz := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		if id == "fmt " {
			fmtChunk = pos + 8
		}
		if id == "data" {
			dataStart = pos + 8
			dataLen = sz
			break
		}
		pos += 8 + sz + (sz & 1)
	}
	if fmtChunk == 0 || dataStart == 0 {
		return nil, errors.New("chunks ausentes")
	}
	channels := int(binary.LittleEndian.Uint16(data[fmtChunk+2 : fmtChunk+4]))
	bits := int(binary.LittleEndian.Uint16(data[fmtChunk+14 : fmtChunk+16]))
	if channels != 1 || bits != 16 {
		return nil, fmt.Errorf("formato não suportado: ch=%d bits=%d", channels, bits)
	}
	n := dataLen / 2
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		s := int16(binary.LittleEndian.Uint16(data[dataStart+i*2 : dataStart+i*2+2]))
		out[i] = float32(s) / 32768.0
	}
	return out, nil
}

// ── Síntese ─────────────────────────────────────────────────────────────

// Sintetizar: pipeline completo — G2P + prosódia + concatenação.
// texto original é usado para o planner; fonemas opcionais para cache.
func (b *Banco) Sintetizar(texto string, estilo VoiceStyle) ([]float32, error) {
	fonemas := G2P(texto)
	plano := PlanejarProsodia(texto, fonemas, estilo)

	// monta sequência de dífonos com base no plano
	var out []float32
	var prev Symbol
	prev = ""
	prevDur := 0.0
	for i, f := range fonemas {
		if f.Symbol == " " {
			// pausa: silêncio com duração do planner
			pausaMs := plano.Duration[i]
			n := int(pausaMs / 1000 * SampleRate)
			out = append(out, make([]float32, n)...)
			prev = ""
			continue
		}
		if prev != "" {
			if smp, ok := b.dif[prev+"_"+f.Symbol]; ok {
				// aplica energia da transição (média dos dois fonemas)
				en := float32((plano.Energy[i-1] + plano.Energy[i]) / 2)
				if en > 0 {
					smp = aplicarEnergia(smp, en)
				}
				// aplica duração do fonema anterior (resample)
				if prevDur > 0 {
					smp = resample(smp, prevDur)
				}
				out = append(out, smp...)
			} else {
				out = append(out, pulsoCurto()...)
			}
		}
		prev = f.Symbol
		prevDur = plano.Duration[i]
	}
	if len(out) == 0 {
		return nil, errors.New("síntese vazia")
	}
	out = crossfadeGlobal(out, fadeSamples)
	return out, nil
}

type Symbol = string

// aplicarEnergia: escala a amplitude (0..1).
func aplicarEnergia(smp []float32, ganho float32) []float32 {
	out := make([]float32, len(smp))
	for i, s := range smp {
		out[i] = s * ganho
	}
	return out
}

// resample: ajusta a duração de um dífono (linear — PSOLA fica para a
// próxima iteração; o esqueleto prosódico já está certo).
func resample(smp []float32, durMs float64) []float32 {
	atualMs := float64(len(smp)) / SampleRate * 1000
	if durMs <= 0 || atualMs <= 0 {
		return smp
	}
	ratio := durMs / atualMs
	if math.Abs(ratio-1.0) < 0.05 {
		return smp
	}
	target := int(float64(len(smp)) * ratio)
	if target <= 0 {
		return smp
	}
	out := make([]float32, target)
	for i := 0; i < target; i++ {
		src := float64(i) * float64(len(smp)-1) / float64(target-1)
		lo := int(src)
		hi := lo + 1
		if hi >= len(smp) {
			hi = len(smp) - 1
		}
		frac := float32(src - float64(lo))
		out[i] = smp[lo]*(1-frac) + smp[hi]*frac
	}
	return out
}

// pulsoCurto: clique de 5ms para dífono ausente (fail-soft audível).
func pulsoCurto() []float32 {
	n := SampleRate / 200
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		env := float32(math.Sin(float64(i) / float64(n) * math.Pi))
		out[i] = 0.08 * env * env
	}
	return out
}

// crossfadeGlobal: fade in/out global (evita cliques nas bordas).
func crossfadeGlobal(x []float32, f int) []float32 {
	n := len(x)
	out := make([]float32, n)
	copy(out, x)
	fadeIn := f
	for i := 0; i < fadeIn && i < n; i++ {
		out[i] *= float32(i) / float32(fadeIn)
	}
	for i := 0; i < f && i < n; i++ {
		out[n-1-i] *= float32(i) / float32(f)
	}
	return out
}

// EscreverWAV16: grava PCM16 mono.
func EscreverWAV16(path string, smp []float32) error {
	n := len(smp)
	raw := make([]byte, 44+n*2)
	copy(raw[0:4], "RIFF")
	binary.LittleEndian.PutUint32(raw[4:8], uint32(36+n*2))
	copy(raw[8:12], "WAVE")
	copy(raw[12:16], "fmt ")
	binary.LittleEndian.PutUint32(raw[16:20], 16)
	binary.LittleEndian.PutUint16(raw[20:22], 1)
	binary.LittleEndian.PutUint16(raw[22:24], 1)
	binary.LittleEndian.PutUint32(raw[24:28], SampleRate)
	binary.LittleEndian.PutUint32(raw[28:32], SampleRate*2)
	binary.LittleEndian.PutUint16(raw[32:34], 2)
	binary.LittleEndian.PutUint16(raw[34:36], 16)
	copy(raw[36:40], "data")
	binary.LittleEndian.PutUint32(raw[40:44], uint32(n*2))
	for i, s := range smp {
		v := int16(math.Max(-1, math.Min(1, float64(s))) * 32767)
		binary.LittleEndian.PutUint16(raw[44+i*2:44+i*2+2], uint16(v))
	}
	return os.WriteFile(path, raw, 0o644)
}

// TocarPCM: envia PCM16 para a saída de áudio (aplay) em streaming.
func TocarPCM(smp []float32) error {
	raw := make([]byte, len(smp)*2)
	for i, s := range smp {
		v := int16(math.Max(-1, math.Min(1, float64(s))) * 32767)
		binary.LittleEndian.PutUint16(raw[i*2:i*2+2], uint16(v))
	}
	cmd := exec.Command("aplay", "-q", "-f", "S16_LE", "-r", "24000", "-c", "1", "-t", "raw", "-")
	cmd.Stdin = bytes.NewReader(raw)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("aplay: %w", err)
	}
	return nil
}