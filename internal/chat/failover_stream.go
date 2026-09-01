package chat

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
)

// defaultMaxStreamReconnects caps how many times failoverStream reconnects to a
// fallback provider over the lifetime of a single logical stream. The cap keeps
// a pathological provider (or a cascade of failures) from turning one Recv call
// into an unbounded reconnect loop while still tolerating a full chain of
// mid-stream failures (primary + two fallbacks).
const defaultMaxStreamReconnects = 2

// failoverStream wraps a ChatStream and transparently reconnects to the next
// available provider when the active stream dies mid-flight (Recv error).
// This provides resilience against providers that fail after streaming has
// started — the client sees the reconnect as a brief interruption instead of
// a dead stream. Reconnect is attempted at most maxReconnects times.
type failoverStream struct {
	ctx           context.Context
	messages      []Message
	opts          ChatOptions
	providers     []ChatProvider // ordered: [primary, fallbacks...]
	current       ChatStream
	providerIdx   int
	maxReconnects int
	reconnects    int
	chunks        int
	sawContent    bool
}

// newFailoverStream wraps stream (opened from providers[idx]) so that
// mid-stream failures transparently reconnect to the next provider in the
// ordered chain.
func newFailoverStream(
	ctx context.Context,
	messages []Message,
	opts ChatOptions,
	providers []ChatProvider,
	idx int,
	stream ChatStream,
) *failoverStream {
	return &failoverStream{
		ctx:           ctx,
		messages:      messages,
		opts:          opts,
		providers:     providers,
		current:       stream,
		providerIdx:   idx,
		maxReconnects: defaultMaxStreamReconnects,
	}
}

// Recv returns the next chunk from the active stream. When the active stream
// dies with an error after having delivered content, it transparently
// reconnects to the next available provider and returns the first chunk of the
// new stream instead of surfacing the error. The ChatStream contract is
// preserved: a normal end is signaled by a chunk with a non-empty FinishReason
// and no error, never by a reconnect.
func (s *failoverStream) Recv() (*ChatStreamChunk, error) {
	for {
		chunk, err := s.current.Recv()
		if err != nil {
			// The active stream died. Reconnect only when the failure is
			// mid-flight — i.e. content was already delivered and we still
			// have reconnect budget. A stream that dies before producing
			// anything is an open failure and surfaces as an error.
			if !s.sawContent || s.reconnects >= s.maxReconnects || !s.reconnect() {
				return nil, fmt.Errorf(
					"stream failed after %d chunks and %d reconnects: %w",
					s.chunks, s.reconnects, err,
				)
			}
			// Reconnected to a fallback provider — loop and read the first
			// chunk from the new stream.
			continue
		}

		if chunk != nil {
			s.chunks++
			if streamChunkHasContent(chunk) {
				s.sawContent = true
			}
			return chunk, nil
		}

		// CONTRATO: um ChatStream NUNCA devolve (nil, nil). Um chunk nil sem
		// erro é um provider degenerado (ex.: fail-closed sem credenciais que
		// "abre" mas não produz). Em vez de propagar o nil (que panica no
		// consumidor — incidente run.go:831), trata como stream morto antes
		// de entregar conteúdo: reconneta ao próximo provider ou falha limpa.
		if !s.reconnect() {
			return nil, fmt.Errorf(
				"stream produced no content and no fallback available (chunks=%d reconnects=%d)",
				s.chunks, s.reconnects,
			)
		}
		// Reconnected — loop and read from the new stream.
		s.sawContent = false
		continue
	}
}

// reconnect tries to open a stream on the next available provider, advancing
// through the remaining providers in order. It returns true when a stream
// opens successfully. Providers are tried only once (no wrap-around): a
// provider that already failed to open is unlikely to succeed moments later,
// and cycling back to it risks an unbounded loop. The maxReconnects cap bounds
// total reconnects across the stream's lifetime.
func (s *failoverStream) reconnect() bool {
	for i := s.providerIdx + 1; i < len(s.providers); i++ {
		provider := s.providers[i]
		stream, err := provider.ChatStream(s.ctx, s.messages, s.opts)
		if err != nil {
			log.Warn().Err(err).Str("provider", provider.Name()).
				Msg("chat stream fallback provider failed to open")
			continue
		}
		s.current = stream
		s.providerIdx = i
		s.reconnects++
		log.Warn().Str("provider", provider.Name()).Int("reconnect", s.reconnects).
			Msg("chat stream reconnecting to fallback provider")
		return true
	}
	return false
}

// Close terminates the active stream. Any stream replaced by a reconnect is
// already dead (it produced a Recv error), so only the current one is closed.
func (s *failoverStream) Close() error {
	if s.current == nil {
		return nil
	}
	return s.current.Close()
}

// streamChunkHasContent reports whether a chunk carries actual model output —
// content deltas or tool call deltas. Role-only and finish-reason chunks do
// not count: a stream that fails before emitting any output is treated as an
// open failure rather than a mid-stream interruption.
func streamChunkHasContent(chunk *ChatStreamChunk) bool {
	for _, choice := range chunk.Choices {
		if choice.Delta.Content != "" || len(choice.Delta.ToolCalls) > 0 {
			return true
		}
	}
	return false
}
