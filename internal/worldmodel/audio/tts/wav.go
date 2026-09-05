package tts

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
)

// WriteWAV16File writes mono 16-bit PCM WAV from normalized samples ([-1, 1]) at
// sampleRate to path. This is the portable "export" path for the speaker: the
// COSCA writes a .wav and a separate player (or the consumer) reproduces it.
// Pure Go — no cgo.
func WriteWAV16File(path string, samples []float32, sampleRate int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := WriteWAV16(f, samples, sampleRate); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// WriteWAV16 writes mono 16-bit PCM WAV from normalized samples ([-1, 1]) at
// sampleRate to w. Pure Go — usable for both files and in-memory buffers (e.g.
// an HTTP `audio/wav` response).
func WriteWAV16(w io.Writer, samples []float32, sampleRate int) error {
	if sampleRate <= 0 {
		return fmt.Errorf("wav: sample_rate must be > 0")
	}
	if len(samples) == 0 {
		return fmt.Errorf("wav: no samples to write")
	}
	// Header (44 bytes) + 16-bit mono PCM body.
	dataSize := len(samples) * 2
	final := make([]byte, 44+dataSize)
	copy(final, wavHeaderPCM16(sampleRate, dataSize))

	buf := final[44:]
	for i, s := range samples {
		v := int16(math.Max(-32768, math.Min(32767, float64(s)*32767)))
		binary.LittleEndian.PutUint16(buf[i*2:i*2+2], uint16(v))
	}

	_, err := w.Write(final)
	return err
}

// EncodeWAV16 returns the mono 16-bit PCM WAV bytes for the given samples. It is
// a convenience wrapper over WriteWAV16 for callers that need an in-memory slice
// (e.g. a REST handler returning `audio/wav`).
func EncodeWAV16(samples []float32, sampleRate int) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := WriteWAV16(buf, samples, sampleRate); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// wavHeaderPCM16 builds the canonical 44-byte RIFF/WAVE header for mono, 16-bit
// PCM audio. dataSize is the payload byte length.
func wavHeaderPCM16(sampleRate, dataSize int) []byte {
	byteRate := sampleRate * 2 // mono × 16-bit (2 bytes) per sample
	h := make([]byte, 44)
	copy(h[0:4], "RIFF")
	binary.LittleEndian.PutUint32(h[4:8], uint32(36+dataSize))
	copy(h[8:12], "WAVE")
	copy(h[12:16], "fmt ")
	binary.LittleEndian.PutUint32(h[16:20], 16) // fmt chunk size (PCM)
	binary.LittleEndian.PutUint16(h[20:22], 1)  // audio format = PCM
	binary.LittleEndian.PutUint16(h[22:24], 1)  // channels = mono
	binary.LittleEndian.PutUint32(h[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(h[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(h[32:34], 2)  // block align (mono×2)
	binary.LittleEndian.PutUint16(h[34:36], 16) // bits per sample
	copy(h[36:40], "data")
	binary.LittleEndian.PutUint32(h[40:44], uint32(dataSize))
	return h
}
