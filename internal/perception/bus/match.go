package bus

import (
	"time"

	"github.com/CoscaAI/cosca/internal/worldmodel"
	"github.com/CoscaAI/cosca/internal/worldmodel/vision"
)

// ──────────────────────────────────────────────────────────────
// Temporal overlap (sync)
// ──────────────────────────────────────────────────────────────
//
// Two observations are "bound" when their temporal intervals
// [Timestamp, Timestamp+Duration] overlap (or are within `tol` of each other).
// This is the core of the multimodal synchronisation: it relates an audio
// segment (a spoken word/phrase) to the vision frames the agent was perceiving
// at that instant.

// interval returns [start, end] of the observation's temporal extent.
func (o Observation) interval() (start, end MonotonicTime) {
	start = o.Timestamp
	dur := o.Duration
	if o.Payload.Audio != nil {
		// Prefer the declared segment duration; fall back to the estimate.
		if d := o.Payload.Audio.duration(); d > 0 {
			dur = d
		}
	}
	if dur < 0 {
		dur = 0
	}
	end = start + ToMonotonic(dur)
	return start, end
}

// overlap computes the amount of real overlap between two intervals, expanded by
// a tolerance `tol` (so a small gap between an audio segment and a vision frame
// still binds). Returns (overlapDuration, true) when the intervals are within
// tol, else (0, false). With tol=0 this is a strict intersection test.
func overlap(aStart, aEnd, bStart, bEnd MonotonicTime, tol time.Duration) (MonotonicTime, bool) {
	// Normalise so aStart <= bStart.
	if aStart > bStart {
		aStart, aEnd, bStart, bEnd = bStart, bEnd, aStart, aEnd
	}
	gap := bStart - aEnd
	if gap < 0 {
		gap = 0
	}
	if gap > ToMonotonic(tol) {
		return 0, false
	}
	overlapEnd := bEnd
	if aEnd < overlapEnd {
		overlapEnd = aEnd
	}
	ov := overlapEnd - bStart
	if ov < 0 {
		ov = 0
	}
	return ov, true
}

// overlapConfidence converts an overlap duration and the reference (audio)
// duration into a normalised [0,1] confidence. A zero reference duration and a
// positive overlap degenerates to a near-1 confidence (an instantaneous
// reference "covers" its own instant by definition).
func overlapConfidence(overlap, reference MonotonicTime) float64 {
	if reference <= 0 {
		if overlap > 0 {
			return 1.0
		}
		return 0
	}
	c := float64(overlap) / float64(reference)
	if c > 1 {
		c = 1
	}
	if c < 0 {
		c = 0
	}
	return c
}

// match returns a MultiRel binding a single audio observation to the vision
// observations that overlapped it within tol. It returns nil when there is no
// vision overlap (or the observation is not audio).
//
// `window` is the full ring snapshot (already sorted by timestamp). `tol` is
// the overlap tolerance (0 → strict intersection).
func match(audioObs Observation, window []Observation, tol time.Duration) *MultiRel {
	if audioObs.Modality != ModalityAudio {
		return nil
	}
	aStart, aEnd := audioObs.interval()
	aud := audioObs.Payload.Audio
	if aud == nil {
		return nil
	}

	refs := make([]WindowRef, 0, 4)
	var totalOverlap MonotonicTime
	for i := range window {
		v := &window[i]
		if v.Modality != ModalityVision {
			continue
		}
		vStart, vEnd := v.interval()
		ov, ok := overlap(aStart, aEnd, vStart, vEnd, tol)
		if !ok {
			continue
		}
		totalOverlap += ov
		refs = append(refs, WindowRef{
			Sequence:  v.Sequence,
			Timestamp: v.Timestamp,
			Entities:  copyEntities(v.Payload.Vision),
		})
	}
	if len(refs) == 0 {
		return nil
	}

	audDur := aEnd - aStart
	return &MultiRel{
		AudioSeg:   aud.SegmentID,
		TextHint:   aud.textHint(),
		AudioConf:  audioObs.Confidence,
		Overlap:    refs,
		Confidence: overlapConfidence(totalOverlap, audDur),
		Window:     [2]time.Duration{tol, tol},
	}
}

// copyEntities returns a defensive copy of a vision observation's entities.
func copyEntities(v *vision.Observation) []worldmodel.WorldEntity {
	if v == nil {
		return nil
	}
	return append([]worldmodel.WorldEntity(nil), v.Entities...)
}
