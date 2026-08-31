package tts

import (
	"sync"

	"github.com/rs/zerolog"
)

// ──────────────────────────────────────────────────────────────
// Speaker: the "falar" primitive (percebe → interpreta → FALA)
// ──────────────────────────────────────────────────────────────

// Speaker wraps a TTS Engine and exposes the high-level primitives the rest of
// the COSCA uses to make the agent speak. It degrades gracefully: with a
// disabled engine (default build) it returns ErrDisabled instead of panicking.
// Reproduction is secondary — the primary deliverable is the synthesized PCM or
// a .wav file the consumer/player reproduces.
type Speaker struct {
	eng    Engine
	cfg    Config
	logger zerolog.Logger

	mu     sync.Mutex
	closed bool
	// output WAV dir, if set, for SpeakToFile default path naming.
	outputDir string
}

// NewSpeaker wraps an engine (possibly the disabled one) into a Speaker.
func NewSpeaker(eng Engine, cfg Config, logger zerolog.Logger) *Speaker {
	if logger.Warn().Enabled() && eng == nil {
		logger.Warn().Msg("tts speaker: nil engine — synthesis will fail")
	}
	return &Speaker{eng: eng, cfg: cfg, logger: logger}
}

// Enabled reports whether the underlying engine is the real native TTS.
func (s *Speaker) Enabled() bool { return s.eng != nil && Enabled() }

// Open opens the engine. Idempotent.
func (s *Speaker) Open() error {
	if s.eng == nil {
		return ErrDisabled
	}
	return s.eng.Open()
}

// Close closes the engine. Idempotent.
func (s *Speaker) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.eng == nil {
		return nil
	}
	return s.eng.Close()
}

// Synthesize turns text into PCM (mono, [-1, 1]) at the engine's sample rate.
// It is the core "the COSCA speaks" call. sid selects the speaker, speed the
// rate (1.0 normal; <1 faster, >1 slower). sid/speed defaults come from Config.
func (s *Speaker) Synthesize(text string, sid int, speed float64) (*SynthesizeResult, error) {
	if s.eng == nil {
		return nil, ErrDisabled
	}
	if sid < 0 {
		sid = s.cfg.Sid
	}
	if speed <= 0 {
		speed = s.cfg.Speed
	}
	return s.eng.Synthesize(text, sid, speed)
}

// SpeakToFile synthesizes text into a .wav file at path. It is the portable
// output: write a WAV and hand it to a player / the consumer. Returns the
// synthesis result (including the duration).
func (s *Speaker) SpeakToFile(text, path string, sid int, speed float64) (*SynthesizeResult, error) {
	res, err := s.Synthesize(text, sid, speed)
	if err != nil {
		return nil, err
	}
	if len(res.Samples) == 0 {
		return res, nil // empty utterance
	}
	if err := WriteWAV16File(path, res.Samples, res.SampleRate); err != nil {
		return nil, err
	}
	s.logger.Info().
		Str("path", path).
		Int("sample_rate", res.SampleRate).
		Int("samples", len(res.Samples)).
		Dur("duration", res.Duration).
		Msg("tts speaker: speech synthesized to wav")
	return res, nil
}

// ──────────────────────────────────────────────────────────────
// SynthesisQueue: async queue — feed text, get PCM/WAV out. Enables a
// non-blocking "COSCA speaks continuously" loop (e.g. from a live bus).
// ──────────────────────────────────────────────────────────────

// SpeakRequest is one queued utterance.
type SpeakRequest struct {
	Text  string  `json:"text"`
	Path  string  `json:"path,omitempty"` // if set, write a .wav for the output
	Sid   int     `json:"sid,omitempty"`
	Speed float64 `json:"speed,omitempty"`
}

// SynthesisQueue consumes SpeakRequest entries on a buffered channel, runs the
// (blocking) synthesis in a single worker goroutine so callers never wait, and
// emits the results on Done. Close stops the worker.
type SynthesisQueue struct {
	spk  *Speaker
	in   chan SpeakRequest
	done chan *SynthesizeResult
	err  chan error
	stop chan struct{}
	wg   sync.WaitGroup
	once sync.Once
}

// NewSynthesisQueue returns a started queue. The caller should Close it.
func NewSynthesisQueue(spk *Speaker, buffer int) *SynthesisQueue {
	if buffer < 1 {
		buffer = 32
	}
	q := &SynthesisQueue{
		spk:  spk,
		in:   make(chan SpeakRequest, buffer),
		done: make(chan *SynthesizeResult, buffer),
		err:  make(chan error, buffer),
		stop: make(chan struct{}),
	}
	q.wg.Add(1)
	go q.run()
	return q
}

// Enqueue submits an utterance. It never blocks (a full buffer drops the item
// and reports the drop via the error channel).
func (q *SynthesisQueue) Enqueue(req SpeakRequest) {
	select {
	case q.in <- req:
	case <-q.stop:
	default:
		select {
		case q.err <- ErrDisabled:
		default:
		}
	}
}

// Done returns the channel of successful synthesis results.
func (q *SynthesisQueue) Done() <-chan *SynthesizeResult { return q.done }

// Errors returns the channel of synthesis failures (non-fatal).
func (q *SynthesisQueue) Errors() <-chan error { return q.err }

// Close stops the worker and drains. Idempotent.
func (q *SynthesisQueue) Close() {
	q.once.Do(func() {
		close(q.stop)
		q.wg.Wait()
		close(q.done)
		close(q.err)
	})
}

func (q *SynthesisQueue) run() {
	defer q.wg.Done()
	for {
		select {
		case <-q.stop:
			return
		case req := <-q.in:
			var res *SynthesizeResult
			var err error
			if req.Path != "" {
				res, err = q.spk.SpeakToFile(req.Text, req.Path, req.Sid, req.Speed)
			} else {
				res, err = q.spk.Synthesize(req.Text, req.Sid, req.Speed)
			}
			if err != nil {
				select {
				case q.err <- err:
				case <-q.stop:
					return
				}
				continue
			}
			select {
			case q.done <- res:
			case <-q.stop:
				return
			}
		}
	}
}
