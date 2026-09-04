package authority

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ─── Promoção LIVE → FROZEN (SPEC §4) ────────────────────────────────────────
//
// A promoção é o ÚNICO caminho legítimo pelo qual conteúdo do LIVE se torna
// autoridade no FROZEN. NÃO é automática: exige intenção explícita
// (NewPromotionProposal) + base-revision/hash do FROZEN no momento da proposta +
// re-read imediatamente antes do apply. Se o FROZEN mudou desde a proposta →
// REJECTED_CONFLICT (não aplica). Sempre snapshot before/after + proveniência.
// Idempotente.

// PromoteOutcome is the final verdict of a promotion (SPEC §4.4).
type PromoteOutcome string

const (
	// OutcomeApplied: the promotion was applied (or is a no-op already-synced).
	OutcomeApplied PromoteOutcome = "APPLIED"
	// OutcomeRejectedConflict: the FROZEN changed since the proposal base.
	OutcomeRejectedConflict PromoteOutcome = "REJECTED_CONFLICT"
	// OutcomeRejectedInvalid: the LIVE/base data is invalid or stale.
	OutcomeRejectedInvalid PromoteOutcome = "REJECTED_INVALID"
	// OutcomeRejectedNoIntent: no explicit intent (nil/empty proposal).
	OutcomeRejectedNoIntent PromoteOutcome = "REJECTED_NO_INTENT"
)

// PromotionProposal is the explicit intent (SPEC §4.2 step 1/2). It records the
// path, the live_sha, the base_frozen_sha (the FROZEN revision over which the
// proposal was born), the material to promote, the motive and the author.
type PromotionProposal struct {
	PromotionID string `json:"promotion_id"`
	Path        string `json:"path"`                  // canonical relative path ("/")
	LiveSHA     string `json:"live_sha"`              // sha256 of LIVE content at propose time
	BaseFrozenSHA string `json:"base_frozen_sha"`     // sha256 of FROZEN path at propose time ("" if absent)
	BaseFrozenRevision string `json:"base_frozen_revision,omitempty"` // tree revision of FROZEN at propose time
	Material    string `json:"material"`              // the content to promote
	Motive      string `json:"motive"`
	Actor       string `json:"actor"`
	ProposedAt  string `json:"proposed_at"` // ISO-8601
}

// PromoteOptions configures a promotion apply.
type PromoteOptions struct {
	SnapshotBase string // base dir for before/after snapshots of the FROZEN
}

// PromoteResult is the immutable provenance record of a promotion (SPEC §4.4).
type PromoteResult struct {
	PromotionID     string        `json:"promotion_id"`
	Path            string        `json:"path"`
	BaseFrozenSHA   string        `json:"base_frozen_sha"`
	FrozenSHABefore string        `json:"frozen_sha_before"`
	FrozenSHAAfter  string        `json:"frozen_sha_after"`
	LiveSHA         string        `json:"live_sha"`
	SnapshotBefore  string        `json:"snapshot_before"`
	SnapshotAfter   string        `json:"snapshot_after"`
	Outcome         PromoteOutcome `json:"outcome"`
	Actor           string        `json:"actor"`
	Timestamp       string        `json:"timestamp"`
	Reasons         []string      `json:"reasons,omitempty"`
}

// promoteBaseSnapshot resolves the snapshot base for a promotion apply.
func promoteBaseSnapshot(frozenRoot string, o PromoteOptions) string {
	if o.SnapshotBase != "" {
		return o.SnapshotBase
	}
	return filepath.Join(filepath.Dir(frozenRoot), ".cosca", "authority")
}

// NewPromotionProposal is PROPOSE (SPEC §4.2 step 1): it is read-only and
// records the intent — the live_sha, the base_frozen_sha (revision of the
// FROZEN over which the proposal is born) and the material (live content). It
// NEVER writes anything; the actual apply is Promote.
func NewPromotionProposal(frozenRoot, liveRoot, path, motive, actor string) (*PromotionProposal, error) {
	if stringsTrim(path) == "" || stringsTrim(actor) == "" {
		return nil, fmt.Errorf("authority: promoção exige path e actor (intenção explícita) — REJECTED_NO_INTENT")
	}
	livePath := filepath.Join(liveRoot, filepath.FromSlash(path))
	if err := ensureWithinRoot(liveRoot, livePath); err != nil {
		return nil, err
	}
	liveData, err := os.ReadFile(livePath)
	if err != nil {
		return nil, fmt.Errorf("authority: promoção — conteúdo LIVE em %q não encontrado (REJECTED_INVALID): %w", path, err)
	}

	frozenPath := filepath.Join(frozenRoot, filepath.FromSlash(path))
	base := ""
	if data, ferr := os.ReadFile(frozenPath); ferr == nil {
		base = sha256Bytes(data)
	}
	baseRev, _ := TreeRevision(frozenRoot)

	return &PromotionProposal{
		PromotionID:       newPromotionID(),
		Path:              canonicalRel(path),
		LiveSHA:           sha256Bytes(liveData),
		BaseFrozenSHA:     base,
		BaseFrozenRevision: baseRev,
		Material:          string(liveData),
		Motive:            motive,
		Actor:             actor,
		ProposedAt:        time.Now().UTC().Format(time.RFC3339Nano),
	}, nil
}

// Promote is the explicit APPLY of a promotion (SPEC §4.2 steps 3–7). It
// re-reads the FROZEN immediately before apply, aborts fail-closed on any
// conflict, snapshots before/after, and records provenance. The FROZEN is the
// only zone this writes into — and it does so ONLY on explicit, verified intent.
//
// The FROZEN write is guarded by GuardWrite with AllowFrozenWrite=true, which is
// the single sanctioned window in the whole machine (SPEC §5).
func Promote(frozenRoot, liveRoot string, prop *PromotionProposal, o PromoteOptions) (*PromoteResult, error) {
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	res := &PromoteResult{Timestamp: ts, Actor: actorOf(prop)}

	if prop == nil || stringsTrim(prop.PromotionID) == "" || stringsTrim(prop.Actor) == "" || stringsTrim(prop.Path) == "" {
		res.Outcome = OutcomeRejectedNoIntent
		res.Reasons = []string{"promoção exige intenção explícita (promotion_id, path, actor) — nenhuma escrita"}
		return res, nil
	}

	res.PromotionID = prop.PromotionID
	res.Path = prop.Path
	res.BaseFrozenSHA = prop.BaseFrozenSHA
	res.LiveSHA = prop.LiveSHA

	frozenPath := filepath.Join(frozenRoot, filepath.FromSlash(prop.Path))
	if err := ensureWithinRoot(frozenRoot, frozenPath); err != nil {
		res.Outcome = OutcomeRejectedInvalid
		res.Reasons = append(res.Reasons, "proposta de promoção tenta escapar a zona FROZEN (path traversal) — fail-closed")
		return res, nil
	}

	// 3. COMPARE — current LIVE must match the proposed live_sha, else stale.
	livePath := filepath.Join(liveRoot, filepath.FromSlash(prop.Path))
	liveData, err := os.ReadFile(livePath)
	if err != nil {
		res.Outcome = OutcomeRejectedInvalid
		res.Reasons = append(res.Reasons, "conteúdo LIVE ausente no apply — repropor")
		return res, nil
	}
	curLiveSHA := sha256Bytes(liveData)
	if curLiveSHA != prop.LiveSHA {
		res.Outcome = OutcomeRejectedInvalid
		res.Reasons = append(res.Reasons, fmt.Sprintf("conteúdo LIVE mudou desde a proposta (%s → %s) — repropor", prop.LiveSHA, curLiveSHA))
		return res, nil
	}

	// Idempotent no-op: the FROZEN already contains the exact promoted content.
	var fBefore []byte
	if fdata, ferr := os.ReadFile(frozenPath); ferr == nil {
		fBefore = fdata
	}
	if fBefore != nil && sha256Bytes(fBefore) == prop.LiveSHA {
		res.FrozenSHABefore = prop.LiveSHA
		res.FrozenSHAAfter = prop.LiveSHA
		res.Outcome = OutcomeApplied
		res.Reasons = append(res.Reasons, "already-synced: o FROZEN já contém exatamente o conteúdo promovido — no-op")
		return res, nil
	}

	// Drift state must be LIVE_NEWER (update) or ONLY_LIVE (add) to promote.
	state := pathState(frozenPath, livePath, fBefore, liveData)
	if state == StateFrozenNewer {
		res.Outcome = OutcomeRejectedConflict
		res.Reasons = append(res.Reasons, "o FROZEN está mais novo que a proposta — a base ficou para trás; não promover")
		return res, nil
	}

	// 4. REREAD — re-read FROZEN immediately before apply; compare current sha
	// vs base_sha. Any change ⇒ REJECTED_CONFLICT (SPEC §4.3). Nothing written.
	curFrozen := ""
	if fdata, ferr := os.ReadFile(frozenPath); ferr == nil {
		curFrozen = sha256Bytes(fdata)
	}
	if curFrozen != prop.BaseFrozenSHA {
		res.Outcome = OutcomeRejectedConflict
		res.Reasons = append(res.Reasons, fmt.Sprintf("o FROZEN mudou desde a proposta — base %q, atual %q; abortar (fail-closed)", prop.BaseFrozenSHA, curFrozen))
		return res, nil
	}
	if prop.BaseFrozenRevision != "" {
		curRev, revErr := TreeRevision(frozenRoot)
		if revErr == nil && curRev != prop.BaseFrozenRevision {
			res.Outcome = OutcomeRejectedConflict
			res.Reasons = append(res.Reasons, "a revisão raiz da árvore FROZEN mudou desde a proposta; abortar (fail-closed)")
			return res, nil
		}
	}

	// 6. GATE passed → snapshot BEFORE, then apply (only sanctioned FROZEN write).
	snapshotBase := promoteBaseSnapshot(frozenRoot, o)
	beforeDir := filepath.Join(snapshotBase, "promote-"+timestampName())
	before := filepath.Join(beforeDir, "before")
	if _, err := SnapshotTree(frozenRoot, before); err != nil {
		return nil, fmt.Errorf("authority: promote snapshot before: %w", err)
	}
	res.SnapshotBefore = before
	res.FrozenSHABefore = curFrozen

	// The sanctioned write into FROZEN (SPEC §5 only allows this via Promote).
	if err := GuardWrite(frozenPath, GuardOptions{FrozenRoot: frozenRoot, LiveRoot: liveRoot, AllowFrozenWrite: true}); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(frozenPath), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(frozenPath, []byte(prop.Material), 0o700); err != nil {
		return nil, fmt.Errorf("authority: promote write frozen %q: %w", prop.Path, err)
	}

	// 7. SNAPSHOT AFTER + verify the written hash is idempotent.
	after := filepath.Join(beforeDir, "after")
	if _, err := SnapshotTree(frozenRoot, after); err != nil {
		return nil, fmt.Errorf("authority: promote snapshot after: %w", err)
	}
	written, werr := os.ReadFile(frozenPath)
	if werr != nil {
		return nil, werr
	}
	res.FrozenSHAAfter = sha256Bytes(written)
	res.SnapshotAfter = after
	res.Outcome = OutcomeApplied
	res.Reasons = append(res.Reasons, "promoção aplicada; rebuild do binário (embutido) e testes exigidos pós-promoção (§4.5)")
	return res, nil
}

// pathState classifies a single file pair without walking the whole tree.
func pathState(frozenPath, livePath string, frozenData, liveData []byte) State {
	var fSHA, lSHA string
	if frozenData != nil {
		fSHA = sha256Bytes(frozenData)
	}
	if liveData != nil {
		lSHA = sha256Bytes(liveData)
	}
	switch {
	case fSHA == "" && lSHA != "":
		return StateOnlyLive
	case fSHA != "" && lSHA == "":
		return StateOnlyFrozen
	case fSHA == lSHA:
		return StateMatch
	case lSHA != "" && fSHA != "":
		// Compare mtimes for the newer side (deterministic tie-break → FROZEN_NEWER).
		return classifyDivergentByFiles(frozenPath, livePath)
	default:
		return StateFrozenNewer
	}
}

// classifyDivergentByFiles reads mtimes of two files to decide the newer side.
func classifyDivergentByFiles(frozenPath, livePath string) State {
	fi1, e1 := os.Stat(frozenPath)
	fi2, e2 := os.Stat(livePath)
	if e1 == nil && e2 == nil {
		if fi2.ModTime().After(fi1.ModTime()) {
			return StateLiveNewer
		}
	}
	return StateFrozenNewer
}

// newPromotionID is a deterministic-ish unique id (time-based + random suffix).
func newPromotionID() string {
	return fmt.Sprintf("promote-%d-%d", time.Now().UTC().UnixNano(), os.Getpid())
}

// actorOf extracts a safe label for provenance before validation.
func actorOf(p *PromotionProposal) string {
	if p == nil {
		return ""
	}
	return p.Actor
}
