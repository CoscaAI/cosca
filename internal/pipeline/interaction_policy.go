package pipeline

import "strings"

// OpLevel defines the risk/impact level of an operation.
type OpLevel int

const (
	OpRead        OpLevel = iota // read-only operations
	OpWrite                      // safe writes (new files, formatting)
	OpModify                     // modify existing files
	OpDangerous                  // delete, rename, move
	OpSystem                     // system-level changes (packages, configs)
	OpIrreversible               // cannot be undone (rm -rf, drop table, etc.)
)

func (o OpLevel) String() string {
	switch o {
	case OpRead:
		return "read"
	case OpWrite:
		return "write"
	case OpModify:
		return "modify"
	case OpDangerous:
		return "dangerous"
	case OpSystem:
		return "system"
	case OpIrreversible:
		return "irreversible"
	default:
		return "unknown"
	}
}

type InteractionPolicy struct {
	AutoApproveRead      bool
	AutoApproveSafeWrite bool
	ConfirmDangerous     bool
	ConfirmSystem        bool
	BlockIrreversible    bool

	AmbiguityThreshold    float64
	HighRiskThreshold     OpLevel
	MaxAutonomousAttempts int
}

type InteractionDecision struct {
	ShouldAsk  bool
	Reason     string
	Question   string
	Options    []string
	AutoChoice string
}

func NewDefaultPolicy() *InteractionPolicy {
	return &InteractionPolicy{
		AutoApproveRead:      true,
		AutoApproveSafeWrite: true,
		ConfirmDangerous:     true,
		ConfirmSystem:        true,
		BlockIrreversible:    true,

		AmbiguityThreshold:    0.70,
		HighRiskThreshold:     OpSystem,
		MaxAutonomousAttempts: 3,
	}
}

func (p *InteractionPolicy) Evaluate(op OpLevel, confidence float64, recoveryAttempts int) *InteractionDecision {
	if p.BlockIrreversible && op >= OpIrreversible {
		return &InteractionDecision{
			ShouldAsk: true,
			Reason:    "operation is irreversible",
			Question:  "This operation cannot be undone. Are you sure you want to proceed?",
			Options:   []string{"proceed", "abort"},
		}
	}

	if p.ConfirmSystem && op >= OpSystem {
		return &InteractionDecision{
			ShouldAsk: true,
			Reason:    "operation affects system-level configuration",
			Question:  "This will make system-level changes. Continue?",
			Options:   []string{"continue", "skip"},
		}
	}

	if p.ConfirmDangerous && op >= OpDangerous {
		return &InteractionDecision{
			ShouldAsk: true,
			Reason:    "operation is potentially destructive",
			Question:  "This will modify or remove existing files. Proceed?",
			Options:   []string{"yes", "no"},
		}
	}

	if op >= p.HighRiskThreshold && confidence < p.AmbiguityThreshold {
		return &InteractionDecision{
			ShouldAsk: true,
			Reason:    "high risk operation with low confidence",
			Question:  "This is a high-risk operation and confidence is low. Continue?",
			Options:   []string{"continue", "abort"},
		}
	}

	if recoveryAttempts >= p.MaxAutonomousAttempts {
		return &InteractionDecision{
			ShouldAsk: true,
			Reason:    "maximum autonomous recovery attempts exceeded",
			Question:  "Recovery has failed multiple times. Would you like to intervene?",
			Options:   []string{"retry", "skip", "abort"},
		}
	}

	if confidence < p.AmbiguityThreshold && op >= OpModify {
		return &InteractionDecision{
			ShouldAsk: true,
			Reason:    "low confidence on modification operation",
			Question:  "The agent is uncertain about this change. Would you like to review?",
			Options:   []string{"approve", "reject"},
		}
	}

	if op == OpRead || (op == OpWrite && p.AutoApproveSafeWrite) {
		return &InteractionDecision{
			ShouldAsk: false,
			AutoChoice: "proceed",
		}
	}

	return &InteractionDecision{
		ShouldAsk: false,
		AutoChoice: "proceed",
	}
}

func (p *InteractionPolicy) ShouldAskHuman(task *TaskNode, context *GeneralContext) *InteractionDecision {
	if task == nil {
		return &InteractionDecision{
			ShouldAsk: false,
			AutoChoice: "proceed",
		}
	}

	op := classifyTaskOp(task)
	confidence := 0.85
	recoveryAttempts := 0

	if task.Result != nil && !task.Result.Success && task.Result.Error != "" {
		recoveryAttempts = 1
	}

	if context != nil {
		_ = context.TraceID()
	}

	return p.Evaluate(op, confidence, recoveryAttempts)
}

func classifyTaskOp(task *TaskNode) OpLevel {
	if task == nil {
		return OpRead
	}

	desc := task.Description

	dangerousKeywords := []string{"delete", "drop", "remove", "rm ", "truncate", "purge"}
	for _, kw := range dangerousKeywords {
		if containsWord(desc, kw) {
			return OpDangerous
		}
	}

	systemKeywords := []string{"deploy", "install", "upgrade", "migrate", "provision"}
	for _, kw := range systemKeywords {
		if containsWord(desc, kw) {
			return OpSystem
		}
	}

	writeKeywords := []string{"create", "add", "generate", "write", "build", "init", "scaffold"}
	for _, kw := range writeKeywords {
		if containsWord(desc, kw) {
			return OpWrite
		}
	}

	modifyKeywords := []string{"update", "change", "modify", "refactor", "fix", "patch", "edit"}
	for _, kw := range modifyKeywords {
		if containsWord(desc, kw) {
			return OpModify
		}
	}

	readKeywords := []string{"read", "list", "show", "view", "get", "fetch", "search", "find", "check", "test", "lint", "inspect", "analyze"}
	for _, kw := range readKeywords {
		if containsWord(desc, kw) {
			return OpRead
		}
	}

	return OpModify
}

func containsWord(s, word string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, strings.ToLower(word))
}
