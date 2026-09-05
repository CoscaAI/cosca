//go:build windows && (amd64 || arm64)

package mic

// This file implements native microphone capture on Windows via the OS-builtin
// winmm/WaveIn API — pure syscalls (syscall.NewLazyDLL("winmm.dll")), NO
// external native dependency (no PortAudio, no pkg-config, no Python). winmm
// ships with every Windows install, so COSCA stays sovereign.
//
// It opens the default (or indexed) input device as mono 16-bit PCM, drains the
// capture buffers in a background goroutine, converts int16 PCM → normalized
// float32 [-1,1], and emits `Chunk` values on a channel (non-blocking: a slow
// consumer drops, never backpressures the OS buffer).
//
// The 8-byte-pointer struct layouts (WAVEHDR / WAVEINCAPS) are the canonical
// 64-bit Windows layout, so this file is constrained to windows/amd64 + arm64;
// other Windows arches degrade via capture_other.go.

import (
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// ---- winmm constants ----
const (
	waveFormatPCM = 0x0001
	callBackNull  = 0x00000000 // no device notification: we poll WHDR_DONE flags
	waveMapper    = ^uint32(0) // default device
	whdrDone      = 0x00000001
	mmsysNoError  = 0
)

// ---- lazy DLLs ----
var (
	mm = syscall.NewLazyDLL("winmm.dll")

	procWaveInOpen            = mm.NewProc("waveInOpen")
	procWaveInClose           = mm.NewProc("waveInClose")
	procWaveInPrepareHeader   = mm.NewProc("waveInPrepareHeader")
	procWaveInUnprepareHeader = mm.NewProc("waveInUnprepareHeader")
	procWaveInAddBuffer       = mm.NewProc("waveInAddBuffer")
	procWaveInStart           = mm.NewProc("waveInStart")
	procWaveInReset           = mm.NewProc("waveInReset")
	procWaveInGetNumDevs      = mm.NewProc("waveInGetNumDevs")
	procWaveInGetDevCapsW     = mm.NewProc("waveInGetDevCapsW") // Unicode variant (exports are waveInGetDevCapsA/W)
)

// waveFormatEx mirrors WAVEFORMATEX.
type waveFormatEx struct {
	wFormatTag      uint16
	nChannels       uint16
	nSamplesPerSec  uint32
	nAvgBytesPerSec uint32
	nBlockAlign     uint16
	wBitsPerSample  uint16
	cbSize          uint16
}

// waveHdr mirrors WAVEHDR (64-bit layout).
type waveHdr struct {
	lpData          uintptr
	dwBufferLength  uint32
	dwBytesRecorded uint32
	dwUser          uintptr
	dwFlags         uint32
	dwLoops         uint32
	lpNext          uintptr
	reserved        uintptr
}

// waveInCaps mirrors WAVEINCAPS (64-bit layout).
type waveInCaps struct {
	wMid           uint16
	wPid           uint16
	vDriverVersion uint32
	szPname        [32]uint16
	dwFormats      uint32
	wChannels      uint16
	wReserved1     uint16
}

func mmErr(tag string, r1 uintptr) error {
	if r1 != mmsysNoError {
		return fmt.Errorf("mic: %s failed (mmr=%#x)", tag, r1)
	}
	return nil
}

// winmmSource is a Source backed by waveIn capture. It runs a goroutine that
// drains OS buffers and emits float32 chunks.
type winmmSource struct {
	cfg    Config
	chunks chan Chunk
	done   chan struct{}
	wg     sync.WaitGroup
	once   sync.Once

	hwi uintptr
	buf [][]byte
	hdr []waveHdr
}

func newCapture(cfg Config) (Source, error) {
	// cfg is already resolved by NewCapture.
	blockAlign := cfg.Channels * 16 / 8
	chunkBytes := cfg.SampleRate * blockAlign * cfg.ChunkMS / 1000
	if chunkBytes <= 0 {
		return nil, fmt.Errorf("mic: invalid chunk size (sample_rate=%d chunk_ms=%d)", cfg.SampleRate, cfg.ChunkMS)
	}

	devIdx := cfg.Device
	if devIdx == 0 {
		devIdx = -1
	}

	fx := waveFormatEx{
		wFormatTag:      waveFormatPCM,
		nChannels:       uint16(cfg.Channels),
		nSamplesPerSec:  uint32(cfg.SampleRate),
		nAvgBytesPerSec: uint32(cfg.SampleRate * blockAlign),
		nBlockAlign:     uint16(blockAlign),
		wBitsPerSample:  16,
	}

	// CALLBACK_NULL: no device notification — we poll WHDR_DONE on each buffer.
	var hwi uintptr
	r, _, _ := procWaveInOpen.Call(
		uintptr(unsafe.Pointer(&hwi)),
		uintptr(uint32(devIdx)),
		uintptr(unsafe.Pointer(&fx)),
		0, // dwCallback
		0, // dwInstance
		callBackNull,
	)
	if err := mmErr("waveInOpen", r); err != nil {
		if r == 2 || r == 1 { // MMSYSERR_BADDEVICEID / MMSYSERR_NODRIVER
			return nil, fmt.Errorf("%w: waveInOpen rejected device %d", ErrNoDevice, devIdx)
		}
		return nil, fmt.Errorf("%w: %v", ErrOpenFailed, err)
	}

	// Allocate a pool of buffers (8x ~= 800ms of latency tolerance) so the
	// device never underruns between re-arms.
	const nBuf = 8
	s := &winmmSource{
		cfg:    cfg,
		chunks: make(chan Chunk, 32),
		done:   make(chan struct{}),
		hwi:    hwi,
		buf:    make([][]byte, nBuf),
		hdr:    make([]waveHdr, nBuf),
	}

	for i := 0; i < nBuf; i++ {
		s.buf[i] = make([]byte, chunkBytes)
		s.hdr[i] = waveHdr{
			lpData:          uintptr(unsafe.Pointer(&s.buf[i][0])),
			dwBufferLength:  uint32(chunkBytes),
		}
		if r, _, _ := procWaveInPrepareHeader.Call(hwi, uintptr(unsafe.Pointer(&s.hdr[i])), unsafe.Sizeof(s.hdr[i])); r != mmsysNoError {
			procWaveInClose.Call(hwi)
			return nil, fmt.Errorf("mic: waveInPrepareHeader %d failed (mmr=%#x)", i, r)
		}
		if r, _, _ := procWaveInAddBuffer.Call(hwi, uintptr(unsafe.Pointer(&s.hdr[i])), unsafe.Sizeof(s.hdr[i])); r != mmsysNoError {
			procWaveInClose.Call(hwi)
			return nil, fmt.Errorf("mic: waveInAddBuffer %d failed (mmr=%#x)", i, r)
		}
	}

	if r, _, _ := procWaveInStart.Call(hwi); r != mmsysNoError {
		procWaveInReset.Call(hwi)
		procWaveInClose.Call(hwi)
		return nil, fmt.Errorf("mic: waveInStart failed (mmr=%#x)", r)
	}

	s.wg.Add(1)
	go s.run()
	return s, nil
}

// run drains the capture buffers and emits float32 chunks. Non-blocking sends.
func (w *winmmSource) run() {
	defer w.wg.Done()
	// Unprepare/close happen in Close(); this goroutine only drains.
	for {
		select {
		case <-w.done:
			return
		default:
		}

		// Drain every buffer whose WHDR_DONE flag is set, re-arming it for the
		// next capture cycle. Then poll again after a short sleep — this is
		// device-agnostic (no reliance on driver event signalling quirks).
		drained := false
		for i := range w.hdr {
			if w.hdr[i].dwFlags&whdrDone == 0 {
				continue
			}
			drained = true
			samples := w.convert(i)
			chunk := Chunk{Samples: samples, SampleRate: w.cfg.SampleRate}
			select {
			case w.chunks <- chunk:
			case <-w.done:
				return
			default:
				// slow consumer → drop this chunk (never backpressure the OS)
			}
			// Re-arm the buffer for the next capture cycle. IMPORTANT: clear
			// only WHDR_DONE — clearing ALL flags would wipe WHDR_PREPARED,
			// making waveInAddBuffer fail silently on the re-arm (and the
			// device would then stop capturing after the first pool fills).
			w.hdr[i].dwFlags &^= whdrDone
			w.hdr[i].dwBytesRecorded = 0
			procWaveInAddBuffer.Call(w.hwi, uintptr(unsafe.Pointer(&w.hdr[i])), unsafe.Sizeof(w.hdr[i]))
		}

		if !drained {
			// Nothing completed this pass: sleep briefly to avoid a busy loop,
			// then re-check. A 10ms poll keeps latency well under one chunk.
			select {
			case <-w.done:
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
}

// convert turns the int16 PCM bytes of buffer i into normalized float32 [-1,1].
func (w *winmmSource) convert(i int) []float32 {
	recorded := int(w.hdr[i].dwBytesRecorded)
	if recorded <= 0 {
		return nil
	}
	samples := recorded / 2
	out := make([]float32, samples)
	b := w.buf[i]
	for s := 0; s < samples; s++ {
		v := int16(uint16(b[s*2]) | uint16(b[s*2+1])<<8)
		out[s] = float32(v) / 32768.0
	}
	return out
}

func (w *winmmSource) Chunks() <-chan Chunk { return w.chunks }

func (w *winmmSource) Close() error {
	var err error
	w.once.Do(func() {
		close(w.done)
		// Stop the capture and unblock the drain goroutine.
		procWaveInReset.Call(w.hwi)
		w.wg.Wait()
		for i := range w.hdr {
			procWaveInUnprepareHeader.Call(w.hwi, uintptr(unsafe.Pointer(&w.hdr[i])), unsafe.Sizeof(w.hdr[i]))
		}
		procWaveInClose.Call(w.hwi)
		close(w.chunks)
	})
	return err
}

// ListDevices enumerates the available input (capture) audio devices on the
// system. Used by `cosca voice` diagnostics and to pick a device index.
func ListDevices() ([]DeviceInfo, error) {
	n, _, _ := procWaveInGetNumDevs.Call()
	if n == 0 {
		return nil, ErrNoDevice
	}
	devs := make([]DeviceInfo, 0, n)
	var caps waveInCaps
	for i := uint32(0); i < uint32(n); i++ {
		if r, _, _ := procWaveInGetDevCapsW.Call(uintptr(i), uintptr(unsafe.Pointer(&caps)), unsafe.Sizeof(caps)); r != mmsysNoError {
			continue
		}
		name := utf16ToString(caps.szPname[:])
		devs = append(devs, DeviceInfo{
			Index:       int(i),
			Name:        name,
			MaxChannels: int(caps.wChannels),
		})
	}
	return devs, nil
}

func utf16ToString(s []uint16) string {
	for i, v := range s {
		if v == 0 {
			s = s[:i]
			break
		}
	}
	return syscall.UTF16ToString(s)
}
