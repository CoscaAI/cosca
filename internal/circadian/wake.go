// Package circadian implements the Cosca Operational Rest Cycle (ORC).
//
// This file implements the wake ritual: the six-step sequence the engine runs
// when it returns from the ORC window to the awake state. The ritual reloads
// kernel memory, validates the knowledge base, reads the constitution, syncs
// the clock, renders the summary and finally accepts commands again.
//
// The ritual never aborts: a failing step is recorded with Status "error" and
// the ritual continues, mirroring the ORC pipeline's isolation guarantees.
package circadian

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver for the integrity check

	"github.com/CoscaAI/cosca/internal/kernel"
)

// WakeStep is a single step of the wake ritual. Status is one of "ok",
// "skipped" or "error".
type WakeStep struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

// WakeResult is the outcome of a wake ritual run.
type WakeResult struct {
	MemoryStats        map[string]int `json:"memory_stats"`
	KnowledgeValid     bool           `json:"knowledge_valid"`
	ConstitutionLoaded bool           `json:"constitution_loaded"`
	ClockSynced        bool           `json:"clock_synced"`
	Summary            string         `json:"summary"`
	At                 time.Time      `json:"at"`
	Steps              []WakeStep     `json:"steps"`
}

// Wake runs the six-step wake ritual against the given .cosca directory:
//
//  1. load_memory       — open kernel memory (knowledge.db) and collect stats
//  2. validate_knowledge — PRAGMA integrity_check over the knowledge base
//  3. read_constitution — verify the constitutional principles (P1-P9, canonical)
//  4. sync_clock        — record the current time and the previous state
//  5. show_summary      — render the ritual summary line
//  6. accept_commands   — transition the engine to the awake state
//
// Every step is isolated: a failure marks the step as "error" and the ritual
// continues. The returned error is only non-nil when the context is cancelled.
func (e *Engine) Wake(ctx context.Context, coscaDir string) (*WakeResult, error) {
	result := &WakeResult{
		MemoryStats: map[string]int{},
		At:          time.Now(),
	}

	// Step 1: load_memory — open the compiled knowledge base.
	dbPath := filepath.Join(coscaDir, "knowledge.db")
	mem, err := kernel.OpenMemory(dbPath)
	if err != nil {
		result.addStep("load_memory", "error", "open memory: "+err.Error())
	} else {
		stats, statsErr := mem.Stats(ctx)
		_ = mem.Close()
		if statsErr != nil {
			result.addStep("load_memory", "error", "read stats: "+statsErr.Error())
		} else {
			result.MemoryStats = stats
			result.addStep("load_memory", "ok", fmt.Sprintf(
				"loaded: %d documents, %d chunks, %d learnings, %d knowledge entries",
				stats["documents"], stats["chunks"], stats["learnings"], stats["knowledge_entries"],
			))
		}
	}

	// Step 2: validate_knowledge — run the SQLite integrity check.
	result.KnowledgeValid = false
	if fi, statErr := os.Stat(dbPath); statErr != nil || fi.Size() == 0 {
		result.addStep("validate_knowledge", "error", "knowledge.db not found: "+dbPath)
	} else {
		db, openErr := sql.Open("sqlite", dbPath)
		if openErr != nil {
			result.addStep("validate_knowledge", "error", "open db: "+openErr.Error())
		} else {
			var integrity string
			scanErr := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity)
			_ = db.Close()
			switch {
			case scanErr != nil:
				result.addStep("validate_knowledge", "error", "integrity check failed: "+scanErr.Error())
			case !strings.EqualFold(strings.TrimSpace(integrity), "ok"):
				result.addStep("validate_knowledge", "error", "integrity check result: "+integrity)
			default:
				result.KnowledgeValid = true
				result.addStep("validate_knowledge", "ok", "PRAGMA integrity_check: ok")
			}
		}
	}

	// Step 3: read_constitution — verify the canonical constitutional principles
	// (P1-P9, que inclui a amenda do Contrato de Autoridade: P9 — Integridade do LIVE).
	result.ConstitutionLoaded = len(kernel.Constitution) == kernel.ExpectedConstitutionPrinciples
	if result.ConstitutionLoaded {
		result.addStep("read_constitution", "ok",
			fmt.Sprintf("P1-P%d loaded (%d principles)", kernel.ExpectedConstitutionPrinciples, len(kernel.Constitution)))
	} else {
		result.addStep("read_constitution", "error",
			fmt.Sprintf("expected %d constitutional principles, got %d",
				kernel.ExpectedConstitutionPrinciples, len(kernel.Constitution)))
	}

	// Step 4: sync_clock — record the time and the previous engine state.
	prevState := e.State()
	result.ClockSynced = true
	result.addStep("sync_clock", "ok", fmt.Sprintf(
		"clock synced at %s (previous state: %s)",
		result.At.UTC().Format(time.RFC3339), prevState,
	))

	// Step 5: show_summary — render the ritual summary line.
	result.Summary = result.renderSummary()
	result.addStep("show_summary", "ok", result.Summary)

	// Step 6: accept_commands — return the engine to the awake state.
	switch {
	case e.State() == StateAwake:
		result.addStep("accept_commands", "ok", "engine already awake, accepting commands")
	case e.TransitionTo(StateAwake, "wake ritual complete") != nil:
		result.addStep("accept_commands", "error", "transition to awake failed")
	default:
		result.addStep("accept_commands", "ok", "engine awake, accepting commands")
	}

	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	return result, nil
}

// addStep appends a step to the ritual result.
func (r *WakeResult) addStep(name, status, detail string) {
	r.Steps = append(r.Steps, WakeStep{Name: name, Status: status, Detail: detail})
}

// renderSummary builds the one-line ritual summary, e.g.
// "ORC retomado: 459 docs, 8370 chunks, P1-P8 carregada, knowledge OK".
func (r *WakeResult) renderSummary() string {
	constitution := "P1-P8 indisponível"
	if r.ConstitutionLoaded {
		constitution = "P1-P8 carregada"
	}
	knowledge := "corrupt"
	if r.KnowledgeValid {
		knowledge = "OK"
	}
	return fmt.Sprintf("ORC retomado: %d docs, %d chunks, %s, knowledge %s",
		r.MemoryStats["documents"], r.MemoryStats["chunks"], constitution, knowledge)
}
