package evals

import (
	"math"
	"math/rand/v2"
)

// Statistical metrics for the eval harness, adapted (design only) from
// OpenAI's evals framework (MIT): get_accuracy, get_bootstrap_accuracy_std,
// get_confusion_matrix, compute_matthew_corr, compute_precision,
// compute_recall, compute_f_score, compute_averaged_f_score, and the
// HigherIsBetter flag. All math is pure Go — no numpy dependency.

const (
	// DefaultMetricThreshold is the per-case reward at or above which a case
	// outcome counts as "positive" for the confusion matrix when it did not
	// explicitly pass (default reward >= 0.5, mirroring OpenAI evals' use of
	// a 0.5 classification cut-off).
	DefaultMetricThreshold = 0.5
	// DefaultBootstrapSamples is the number of bootstrap resamples used for
	// BootstrapStd, matching OpenAI evals' 1000-sample bootstrap.
	DefaultBootstrapSamples = 1000
	// DefaultBootstrapSeed is the fixed seed for BootstrapStd, keeping the
	// bootstrap std reproducible across runs for a given suite.
	DefaultBootstrapSeed uint64 = 20260811
)

// ConfusionMatrix for binary outcomes, where "positive" is a case that
// passed (or reached the reward threshold). Every benchmark case is treated
// as an expected positive: TP counts cases that achieved the threshold, FN
// counts cases that were expected to pass but did not. FP and TN are zero by
// construction unless a caller builds the matrix differently.
type ConfusionMatrix struct {
	TP, FP, TN, FN int
}

// ComputeConfusionMatrix classifies each result as positive when the case
// passed or its reward reached threshold (defaulting to
// DefaultMetricThreshold), and negative otherwise.
func ComputeConfusionMatrix(results []CaseResult, threshold float64) ConfusionMatrix {
	if threshold <= 0 {
		threshold = DefaultMetricThreshold
	}
	cm := ConfusionMatrix{}
	for _, cr := range results {
		if cr.Status == StatusPassed || cr.Reward >= threshold {
			cm.TP++
		} else {
			cm.FN++
		}
	}
	return cm
}

// Accuracy returns (tp+tn)/total, or 0 when total is 0.
func Accuracy(tp, tn, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(tp+tn) / float64(total)
}

// Precision returns tp/(tp+fp), or 0 when the denominator is 0.
func Precision(tp, fp int) float64 {
	denom := tp + fp
	if denom <= 0 {
		return 0
	}
	return float64(tp) / float64(denom)
}

// Recall returns tp/(tp+fn), or 0 when the denominator is 0.
func Recall(tp, fn int) float64 {
	denom := tp + fn
	if denom <= 0 {
		return 0
	}
	return float64(tp) / float64(denom)
}

// FScore returns the beta-weighted F-score
// (1+beta^2)*precision*recall/(beta^2*precision+recall). beta=1 yields the
// harmonic F1. It returns 0 when precision and recall are both 0.
func FScore(tp, fp, fn int, beta float64) float64 {
	p := Precision(tp, fp)
	r := Recall(tp, fn)
	denom := beta*beta*p + r
	if denom <= 0 {
		return 0
	}
	return (1 + beta*beta) * p * r / denom
}

// MatthewCorr returns the standard Matthews correlation coefficient
// (TP*TN - FP*FN) / sqrt((TP+FP)(TP+FN)(TN+FP)(TN+FN)), clamped to 0 when
// the denominator vanishes (no meaningful correlation is computable).
func MatthewCorr(tp, fp, tn, fn int) float64 {
	denom := math.Sqrt(float64(tp+fp) * float64(tp+fn) * float64(tn+fp) * float64(tn+fn))
	if denom == 0 {
		return 0
	}
	return (float64(tp*tn) - float64(fp*fn)) / denom
}

// BootstrapStd computes the bootstrap standard deviation of the mean reward
// over numSamples resamples (default DefaultBootstrapSamples, matching
// OpenAI evals' 1000-sample bootstrap). Each resample draws len(rewards)
// samples with replacement from the per-case rewards (0.0-1.0), computes the
// mean, and the function returns the std of those means. Identical rewards
// yield a std of 0. A local generator seeded deterministically
// (DefaultBootstrapSeed) makes the result reproducible for a given suite.
func BootstrapStd(rewards []float64, numSamples int) float64 {
	return BootstrapStdSeed(rewards, numSamples, DefaultBootstrapSeed)
}

// BootstrapStdSeed is BootstrapStd with an explicit seed for the local
// generator. The same seed always reproduces the same std for the same
// rewards slice.
func BootstrapStdSeed(rewards []float64, numSamples int, seed uint64) float64 {
	if len(rewards) == 0 {
		return 0
	}
	if numSamples <= 0 {
		numSamples = DefaultBootstrapSamples
	}
	// math/rand/v2 with a PCG source is deterministic per seed and does not
	// touch the global RNG.
	rng := rand.New(rand.NewPCG(seed, seed))
	n := len(rewards)

	means := make([]float64, 0, numSamples)
	for i := 0; i < numSamples; i++ {
		var sum float64
		for j := 0; j < n; j++ {
			sum += rewards[rng.IntN(n)]
		}
		means = append(means, sum/float64(n))
	}

	var m float64
	for _, x := range means {
		m += x
	}
	m /= float64(numSamples)

	var sq float64
	for _, x := range means {
		d := x - m
		sq += d * d
	}
	return math.Sqrt(sq / float64(numSamples))
}

// computeMetrics derives the EvalMetrics for a finished report: the
// confusion matrix from case statuses (passed = positive), the aggregate
// metrics, and the bootstrap std of the per-case rewards.
func computeMetrics(report *Report) {
	cm := ComputeConfusionMatrix(report.Cases, DefaultMetricThreshold)
	total := len(report.Cases)

	rewards := make([]float64, 0, total)
	for _, cr := range report.Cases {
		rewards = append(rewards, cr.Reward)
	}

	report.Metrics = EvalMetrics{
		Accuracy:       Accuracy(cm.TP, cm.TN, total),
		Precision:      Precision(cm.TP, cm.FP),
		Recall:         Recall(cm.TP, cm.FN),
		F1:             FScore(cm.TP, cm.FP, cm.FN, 1.0),
		MatthewsCorr:   MatthewCorr(cm.TP, cm.FP, cm.TN, cm.FN),
		BootstrapStd:   BootstrapStd(rewards, DefaultBootstrapSamples),
		HigherIsBetter: true,
	}
}
