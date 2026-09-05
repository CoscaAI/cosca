package skilleval

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand/v2"
)

// EvalSplit is the deterministic partition of a skill-eval dataset into three
// disjoint sets: Train (fitness), Val (validation) and Holdout (regression).
// The three sets are disjoint and their union is exactly the input dataset, so
// a case never appears in more than one set — and crucially the holdout never
// leaks into the train set.
//
// FATIA 3.1 (bounded): this establishes the split that underlies the
// benchmarks-as-gates phase. Fitness (did the skill improve) is measured on
// Train, while regression ("did it break the rest") is measured ONLY on the
// dedicated Holdout via RegressionGate (see regression.go). Keeping the holdout
// disjoint from train is the anti-overfit guarantee: a variant that overfits the
// train set can be caught in the holdout without contaminating the fitness
// estimate.
type EvalSplit struct {
	Train, Val, Holdout []SkillCase
}

const (
	// DefaultSplitRatio is the default fraction of a dataset reserved for the
	// Train split. The complement is split evenly between Val and Holdout, so
	// the default is 0.5 / 0.25 / 0.25 — the Hermes-style anti-overfit split.
	DefaultSplitRatio = 0.5
	// MinSplitCases is the smallest dataset a split can accept. A dataset below
	// this is rejected because it cannot yield a useful holdout (the case count
	// is the only signal the regression gate has; 2 cases cannot separate a
	// train-fitness signal from a holdout-regression signal).
	MinSplitCases = 3
)

// SplitEval partitions `cases` into Train / Val / Holdout given a single `ratio`
// that is the Train fraction. The remaining (1-ratio) is divided evenly between
// Val and Holdout, so ratio=0.5 yields the default 50/25/25 split.
//
// Determinism and stability:
//   - The shuffle is seeded by a combination of `seed` and a hash of the skill
//     name, so the same (skill, seed) pair always reproduces the exact same
//     partition (reproducible) while different skills partition differently even
//     with the same numeric seed (the hash varies the stream per skill).
//   - The randomness uses math/rand/v2 with a PCG source, the same
//     deterministic, cross-platform primitive established in
//     internal/evals.BootstrapStdSeed; it never touches the global RNG.
//
// Validation:
//   - `ratio` must be strictly in (0,1): an out-of-range ratio is rejected
//     because it would starve one of the three sets. Note that Train+Val+Holdout
//     always sums to 1.0 by construction (ratio + (1-ratio)), and SplitEval
//     asserts that invariant before building the result.
//   - len(cases) must be at least MinSplitCases (3), otherwise the split is
//     rejected because it cannot host a useful holdout.
//   - The split sizes are allocated so that Train+Val+Holdout == len(cases)
//     exactly and, when the dataset allows it, Val and Holdout each hold at
//     least one case.
func SplitEval(skill string, cases []SkillCase, ratio float64, seed int64) (*EvalSplit, error) {
	if ratio <= 0 || ratio >= 1 || math.IsNaN(ratio) {
		return nil, fmt.Errorf("skilleval: split ratio %.4f must be strictly in (0,1)", ratio)
	}
	if len(cases) < MinSplitCases {
		return nil, fmt.Errorf("skilleval: split needs at least %d cases for a useful holdout, got %d", MinSplitCases, len(cases))
	}

	nTrain, nVal, nHold := splitSizes(len(cases), ratio)
	// Train / Val / Holdout sum to 1.0 by construction (ratio + (1-ratio) = 1),
	// but assert the invariant explicitly so the caller sees no drift.
	if float64(nTrain+nVal+nHold) != float64(len(cases)) {
		return nil, fmt.Errorf("skilleval: split sizes %d/%d/%d do not cover %d cases", nTrain, nVal, nHold, len(cases))
	}

	// Deterministic shuffle: seed a PCG source with the skill hash and the user
	// seed, so the partition is reproducible per (skill, seed) and different
	// skills get different streams. The permutation is a bijection over
	// [0, n), so reading contiguous ranges covers every case exactly once.
	rng := rand.New(rand.NewPCG(splitSeed(skill), uint64(seed)))
	order := rng.Perm(len(cases))

	split := &EvalSplit{
		Train:   make([]SkillCase, 0, nTrain),
		Val:     make([]SkillCase, 0, nVal),
		Holdout: make([]SkillCase, 0, nHold),
	}
	for i, pos := range order {
		switch {
		case i < nTrain:
			split.Train = append(split.Train, cases[pos])
		case i < nTrain+nVal:
			split.Val = append(split.Val, cases[pos])
		default:
			split.Holdout = append(split.Holdout, cases[pos])
		}
	}

	// Defensive: the sets must be disjoint and covering. Any violation here is a
	// bug in splitSizes / the shuffle, so fail loudly rather than hand back a
	// corrupted partition that could silently leak holdout cases into train.
	if !disjointCovers(split, len(cases)) {
		return nil, fmt.Errorf("skilleval: split for skill %q is not a covering partition", skill)
	}
	return split, nil
}

// splitSeed derives a stable, skill-aware hash that decorrelates the split
// stream across skills. It uses FNV-1a over the skill name, the same cheap,
// deterministic primitive used elsewhere in the codebase for identity-derived
// hashes. The result is a uint64 that is mixed with the caller's int64 seed
// (converted to uint64) when seeding the PCG source.
func splitSeed(skill string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(skill))
	return h.Sum64()
}

// splitSizes allocates the three split sizes from n and the train fraction.
//
// It rounds the train fraction and splits the remainder evenly between Val and
// Holdout, then guarantees a non-empty Val and Holdout when the dataset is large
// enough to support them (stealing from train if necessary) so the split always
// yields a usable validation and regression shard. The sizes always sum to n.
func splitSizes(n int, ratio float64) (nTrain, nVal, nHold int) {
	if n <= 0 {
		return 0, 0, 0
	}
	nTrain = int(math.Round(ratio * float64(n)))
	if nTrain > n {
		nTrain = n
	}
	if nTrain < 0 {
		nTrain = 0
	}
	rem := n - nTrain
	nHold = rem / 2
	nVal = rem - nHold

	// Guarantee a non-empty holdout, then a non-empty val, when the dataset is
	// large enough. Stealing from the larger train shard keeps the partition
	// covering while preserving a useful regression/validation signal.
	if n >= MinSplitCases {
		if nHold < 1 && nVal >= 1 {
			nVal--
			nHold++
		}
		if nVal < 1 && nTrain >= 1 {
			nTrain--
			nVal++
		}
	}
	if nTrain < 0 {
		nTrain = 0
	}
	return nTrain, nVal, nHold
}

// disjointCovers reports whether the split's three sets are pairwise disjoint
// and together hold every one of the total cases exactly once. It compares
// case IDs, so it only relies on identity rather than structural equality.
func disjointCovers(split *EvalSplit, total int) bool {
	if split == nil {
		return false
	}
	if len(split.Train)+len(split.Val)+len(split.Holdout) != total {
		return false
	}
	seen := make(map[string]struct{}, total)
	for _, group := range [][]SkillCase{split.Train, split.Val, split.Holdout} {
		for _, c := range group {
			if _, dup := seen[c.ID]; dup {
				return false
			}
			seen[c.ID] = struct{}{}
		}
	}
	return len(seen) == total
}
