package deliberate

// Cross-review voting (ADR-011 A5 core): SELF-VOTE IS FORBIDDEN. Each reviewer
// escorts the others, never themself. VoteCross validates the votes and
// excludes the decision owner from the cross-review pool.

import (
	"errors"
	"fmt"
)

// ErrSelfVote is returned by VoteCross when a reviewer votes on their own
// target (Target == Reviewer). Self-votes break the adversarial property and
// are always rejected (A5).
var ErrSelfVote = errors.New("deliberate: self-vote is forbidden (Reviewer == Target)")

// VoteCross validates a batch of cross-reviews and returns the pool of valid
// votes for a decision owner.
//
// Rules (deterministic, zero-LLM):
//   - Any review with Target == Reviewer (a self-vote) makes the whole batch
//     invalid: it returns ErrSelfVote and NO partial result.
//   - Reviews whose Reviewer is the decision `owner` are excluded from the
//     returned pool: the owner does not cross-vote on their own decision (the
//     one who proposes does not validate the proposal).
//   - Reviews must not be nil/empty-Reviewer; those are rejected with an error.
//
// The returned slice preserves the original order of the accepted reviews.
func VoteCross(reviews []Review, owner string) ([]Review, error) {
	if len(reviews) == 0 {
		return []Review{}, nil
	}

	accepted := make([]Review, 0, len(reviews))
	for _, r := range reviews {
		if r.SelfVote() {
			return nil, fmt.Errorf("%w (reviewer %q voted on target %q)", ErrSelfVote, r.Reviewer, r.Target)
		}
		if r.Reviewer == "" || r.Target == "" {
			return nil, fmt.Errorf("deliberate: review with empty reviewer or target (reviewer=%q target=%q)", r.Reviewer, r.Target)
		}
		if r.Reviewer == owner {
			// The owner does not cross-vote on their own decision.
			continue
		}
		accepted = append(accepted, r)
	}

	return accepted, nil
}
