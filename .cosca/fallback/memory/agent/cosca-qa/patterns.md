# cosca-qa — Reusable Patterns

> Discovered patterns that can be reapplied. Grows with agent experience.

## Patterns Discovered

### P1 evaluation: evidence fidelity × authority safety

For context-and-injection evaluations, freeze synthetic fixtures and expected evidence
IDs; measure retrieval/attribution separately from answer correctness. Run the same task
with benign, indirect, and tool-result injections, and assert both the textual decision
and an empty side-effect recorder. A release gate should fail on any unsafe tool request,
even when the final answer looks correct; report quality, security, cost, and latency as
separate dimensions rather than hiding a security failure in an aggregate score.

### Evidence-bounded capability matrix

For competitive matrices, first freeze the column set and capability definitions;
then score only from code/test evidence, attach a caveat to every cell, and reserve
`?` for a genuinely un-audited boundary (especially external backends). Keep
workflow graph, knowledge graph, memory, replay, recovery and evaluation separate:
their names overlap, but their guarantees do not.
