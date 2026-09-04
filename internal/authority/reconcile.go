package authority

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ─── Reconciliação FROZEN → LIVE (SPEC §3) ──────────────────────────────────
//
// Materializa de forma SEGURA o LIVE a partir do FROZEN: aditiva, idempotente,
// NÃO-destrutiva. NUNCA copy-all destrutivo; NUNCA deleta exclusivos do FROZEN;
// preserva conteúdo divergente do LIVE como evidência (candidato a promoção,
// §4). Backup/snapshot antes; dry-run obrigatório (PlanReconcile) e --apply só
// com dry-run prévio + flag explícita (ApplyReconcile).

// ReconcileAction is a single planned or performed action (SPEC §6.2 audit).
type ReconcileAction struct {
	Path   string `json:"path"`
	State  State  `json:"state"`
	Op     string `json:"op"` // create|update|skip|preserve|evidence
	Detail string `json:"detail,omitempty"`
}

// ReconcileResult reports what the reconciliation did (or would do) — FACT.
type ReconcileResult struct {
	FrozenRoot     string            `json:"frozen_root"`
	LiveRoot       string            `json:"live_root"`
	DryRun         bool              `json:"dry_run"`
	Applied        bool              `json:"applied"`
	Actions        []ReconcileAction `json:"actions"`
	SnapshotBefore string            `json:"snapshot_before,omitempty"`
	EvidenceDir    string            `json:"evidence_dir,omitempty"`
	ReconciledAt   string            `json:"reconciled_at"`
	Outcome        string            `json:"outcome"` // dry-run|applied|no-change|fail-closed
}

// ReconcileOptions configures an apply.
type ReconcileOptions struct {
	FrozenRoot   string
	LiveRoot     string
	SnapshotBase string // base dir for the before/after snapshots + evidence
	EvidenceBase string // base dir for copied divergent LIVE evidence
}

// defaultReconcileDirs resolves SnapshotBase/EvidenceBase defaults kept outside
// the LIVE tree, so the LIVE root can be restored exactly (rollback) and the
// evidence never pollutes the surface.
func defaultReconcileDirs(liveRoot string, o ReconcileOptions) (snapshotBase, evidenceBase string) {
	snapshotBase = o.SnapshotBase
	if snapshotBase == "" {
		snapshotBase = filepath.Join(filepath.Dir(liveRoot), ".cosca", "authority")
	}
	evidenceBase = o.EvidenceBase
	if evidenceBase == "" {
		evidenceBase = filepath.Join(snapshotBase, "evidence")
	}
	return snapshotBase, evidenceBase
}

// planActions builds the deterministic dry-run action list from a drift report.
func planActions(rep *DriftReport) []ReconcileAction {
	acts := make([]ReconcileAction, 0, len(rep.Entries))
	for _, e := range rep.Entries {
		switch e.State {
		case StateMatch:
			acts = append(acts, ReconcileAction{Path: e.Path, State: e.State, Op: "skip",
				Detail: "já em sincronia — nada a fazer"})
		case StateFrozenNewer:
			acts = append(acts, ReconcileAction{Path: e.Path, State: e.State, Op: "update",
				Detail: "FROZEN é autoridade — atualizar o LIVE a partir do FROZEN"})
		case StateLiveNewer:
			acts = append(acts, ReconcileAction{Path: e.Path, State: e.State, Op: "evidence",
				Detail: "LIVE mais novo — NÃO sobrescrever; preservar como evidência (candidato a promoção §4)"})
		case StateOnlyFrozen:
			acts = append(acts, ReconcileAction{Path: e.Path, State: e.State, Op: "preserve",
				Detail: "exclusivo do FROZEN — preservar; nunca deletar; não copiar ao LIVE por padrão (G5)"})
		case StateOnlyLive:
			acts = append(acts, ReconcileAction{Path: e.Path, State: e.State, Op: "evidence",
				Detail: "só no LIVE — registrar como evidência de evolução a promover (§2.2)"})
		}
	}
	return acts
}

// PlanReconcile is the MANDATORY dry-run: it computes exactly the set of
// actions (create/update/skip/preserve/evidence) WITHOUT writing anything.
// Without a prior plan (dry-run) no apply is permitted.
func PlanReconcile(frozenRoot, liveRoot string) (*ReconcileResult, error) {
	rep, err := RunDrift(frozenRoot, liveRoot)
	if err != nil {
		return nil, err
	}
	_, evidenceBase := defaultReconcileDirs(liveRoot, ReconcileOptions{})
	return &ReconcileResult{
		FrozenRoot:  frozenRoot,
		LiveRoot:    liveRoot,
		DryRun:      true,
		Applied:     false,
		Actions:     planActions(rep),
		EvidenceDir: evidenceBase,
		ReconciledAt: time.Now().UTC().Format(time.RFC3339Nano),
		Outcome:     "dry-run",
	}, nil
}

// planStillValid re-verifies (after a dry-run) that every write/evidence action
// in the plan still matches the CURRENT drift. If the FROZEN changed between
// plan and apply (SPEC §4.3-style conflict, extended to reconcile), the apply is
// aborted fail-closed: nothing is written.
func planStillValid(plan *ReconcileResult, cur *DriftReport) []string {
	var mismatches []string
	curByPath := map[string]State{}
	for _, e := range cur.Entries {
		curByPath[relKey(e.Path)] = e.State
	}
	for _, a := range plan.Actions {
		if a.Op != "update" && a.Op != "evidence" {
			continue
		}
		got, ok := curByPath[relKey(a.Path)]
		if !ok || got != a.State {
			mismatches = append(mismatches, fmt.Sprintf("%s: planejado %s, atual %s", a.Path, a.State, got))
		}
	}
	return mismatches
}

// ApplyReconcile performs the reconciliation. It is idempotent and additive. It
// REFUSES to run without a prior dry-run plan (fail-closed), re-verifies the
// plan against the current state, snapshots the LIVE root BEFORE any write, and
// never writes to FROZEN, never deletes, and never copies ONLY_FROZEN to LIVE.
func ApplyReconcile(frozenRoot, liveRoot string, plan *ReconcileResult, o ReconcileOptions) (*ReconcileResult, error) {
	if plan == nil {
		return nil, fmt.Errorf("authority: reconcile apply recusado sem dry-run prévio — rode PlanReconcile primeiro (fail-closed)")
	}
	snapshotBase, evidenceBase := defaultReconcileDirs(liveRoot, o)

	// 1. Re-verify the plan (dry-run → apply conflict => fail-closed, T7).
	cur, err := RunDrift(frozenRoot, liveRoot)
	if err != nil {
		return nil, err
	}
	if mm := planStillValid(plan, cur); len(mm) > 0 {
		return nil, fmt.Errorf("authority: reconcile falhou fail-closed — o FROZEN mudou entre dry-run e apply; nada foi escrito. Discrepância: %v", mm)
	}

	// 2. Snapshot BEFORE (SPEC §3.3.5 / §6.3). Never destroy the snapshot.
	beforeDir := filepath.Join(snapshotBase, "reconcile-"+timestampName())
	before := filepath.Join(beforeDir, "before")
	if _, err := SnapshotTree(liveRoot, before); err != nil {
		return nil, fmt.Errorf("authority: reconcile snapshot before: %w", err)
	}

	// 3. Apply the write actions (additive, never into FROZEN, never delete).
	result := &ReconcileResult{
		FrozenRoot:     frozenRoot,
		LiveRoot:       liveRoot,
		DryRun:         false,
		Applied:        true,
		Actions:        plan.Actions,
		SnapshotBefore: before,
		EvidenceDir:    evidenceBase,
		ReconciledAt:   time.Now().UTC().Format(time.RFC3339Nano),
		Outcome:        "applied",
	}

	guard := GuardOptions{FrozenRoot: frozenRoot, LiveRoot: liveRoot, AllowFrozenWrite: false}
	for _, a := range plan.Actions {
		liveTarget := filepath.Join(liveRoot, filepath.FromSlash(a.Path))
		frozenTarget := filepath.Join(frozenRoot, filepath.FromSlash(a.Path))

		switch a.Op {
		case "update": // FROZEN_NEWER → LIVE := FROZEN (FROZEN autoridade)
			if err := GuardWrite(liveTarget, guard); err != nil {
				return nil, err
			}
			data, err := os.ReadFile(frozenTarget)
			if err != nil {
				return nil, fmt.Errorf("authority: reconcile update read frozen %q: %w", a.Path, err)
			}
			if err := os.MkdirAll(filepath.Dir(liveTarget), 0o700); err != nil {
				return nil, err
			}
			if err := os.WriteFile(liveTarget, data, 0o700); err != nil {
				return nil, fmt.Errorf("authority: reconcile update write live %q: %w", a.Path, err)
			}
		case "evidence": // LIVE_NEWER / ONLY_LIVE → preserve LIVE, copy to evidence
			if err := GuardWrite(liveTarget, guard); err != nil {
				return nil, err
			}
			// Copy the divergent LIVE content as evidence (never overwrite it).
			if data, err := os.ReadFile(liveTarget); err == nil {
				evid := filepath.Join(evidenceBase, filepath.FromSlash(a.Path))
				if werr := os.MkdirAll(filepath.Dir(evid), 0o700); werr != nil {
					return nil, werr
				}
				if werr := os.WriteFile(evid, data, 0o700); werr != nil {
					return nil, fmt.Errorf("authority: reconcile evidence write %q: %w", a.Path, werr)
				}
			}
		case "preserve", "skip": // ONLY_FROZEN / MATCH → nothing
			// No write. ONLY_FROZEN is never deleted, never copied by default (G5).
		}
	}

	// 4. Mark no-change if nothing needed writing.
	wrote := 0
	for _, a := range plan.Actions {
		if a.Op == "update" || a.Op == "evidence" {
			wrote++
		}
	}
	if wrote == 0 {
		result.Outcome = "no-change"
		result.Applied = false
	}

	return result, nil
}
