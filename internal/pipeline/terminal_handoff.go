package pipeline

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type HandoffCoordinator struct {
	store  *HandoffStore
	chains map[string]*HandoffChain
	mu     sync.RWMutex
}

type HandoffChain struct {
	TaskID      string
	Handoffs    []*HandoffArtifact
	Current     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	mu          sync.RWMutex
}

func NewHandoffCoordinator(store *HandoffStore) *HandoffCoordinator {
	return &HandoffCoordinator{
		store:  store,
		chains: make(map[string]*HandoffChain),
	}
}

func (h *HandoffCoordinator) PrepareHandoff(fromAgent, toAgent string, task *TaskNode, context *GeneralContext) *HandoffArtifact {
	artifact := &HandoffArtifact{
		ID:        NewHandoffID(),
		FromAgent: fromAgent,
		ToAgent:   toAgent,
		CreatedAt: time.Now().UTC(),
	}

	if task != nil {
		artifact.Objective = task.Description
		artifact.Files = task.InputFiles
		artifact.Tests = task.OutputFiles
		if task.Result != nil {
			if task.Result.Error != "" {
				artifact.Errors = append(artifact.Errors, task.Result.Error)
			}
		}
	}

	if context != nil {
		artifact.Decisions = context.decisions
		artifact.KnowledgeUsed = context.knowledgeUsed
	}

	return artifact
}

func (h *HandoffCoordinator) ExecuteHandoff(ctx context.Context, artifact *HandoffArtifact) error {
	if artifact == nil {
		return fmt.Errorf("handoff: artifact is nil")
	}
	if artifact.ToAgent == "" {
		return fmt.Errorf("handoff: no target agent specified")
	}
	if artifact.FromAgent == artifact.ToAgent {
		return fmt.Errorf("handoff: source and target agent are the same (%s)", artifact.FromAgent)
	}

	if artifact.ID == "" {
		artifact.ID = NewHandoffID()
	}
	if artifact.CreatedAt.IsZero() {
		artifact.CreatedAt = time.Now().UTC()
	}

	if h.store != nil {
		if err := h.store.Save(artifact); err != nil {
			return fmt.Errorf("handoff: persist artifact: %w", err)
		}
	}

	if taskID := h.extractTaskID(artifact); taskID != "" {
		h.mu.Lock()
		chain, exists := h.chains[taskID]
		if !exists {
			chain = &HandoffChain{
				TaskID:    taskID,
				Handoffs:  make([]*HandoffArtifact, 0),
				Current:   artifact.FromAgent,
				CreatedAt: time.Now().UTC(),
			}
			h.chains[taskID] = chain
		}
		h.mu.Unlock()

		chain.mu.Lock()
		chain.Handoffs = append(chain.Handoffs, artifact)
		chain.Current = artifact.ToAgent
		chain.UpdatedAt = time.Now().UTC()
		chain.mu.Unlock()
	}

	return nil
}

func (h *HandoffCoordinator) GetChain(taskID string) *HandoffChain {
	h.mu.RLock()
	defer h.mu.RUnlock()
	chain, exists := h.chains[taskID]
	if !exists {
		return nil
	}
	return chain
}

func (h *HandoffCoordinator) ChainTrace(taskID string) string {
	chain := h.GetChain(taskID)
	if chain == nil {
		return fmt.Sprintf("No handoff chain found for task %s", taskID)
	}

	chain.mu.RLock()
	defer chain.mu.RUnlock()

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Handoff Chain: %s\n", taskID))
	b.WriteString(fmt.Sprintf("Created: %s | Last: %s | Current: %s\n",
		chain.CreatedAt.Format(time.RFC3339),
		chain.UpdatedAt.Format(time.RFC3339),
		chain.Current))
	b.WriteString(fmt.Sprintf("%d handoffs:\n", len(chain.Handoffs)))

	for i, art := range chain.Handoffs {
		b.WriteString(fmt.Sprintf("  %d. %s → %s [%s]",
			i+1, art.FromAgent, art.ToAgent, art.ID))
		if art.Objective != "" {
			b.WriteString(fmt.Sprintf(" | %s", art.Objective))
		}
		b.WriteString(fmt.Sprintf(" | %s\n", art.CreatedAt.Format(time.RFC3339)))
	}

	return b.String()
}

func (h *HandoffCoordinator) AllChains() []*HandoffChain {
	h.mu.RLock()
	defer h.mu.RUnlock()

	chains := make([]*HandoffChain, 0, len(h.chains))
	for _, c := range h.chains {
		chains = append(chains, c)
	}
	return chains
}

func (h *HandoffCoordinator) extractTaskID(artifact *HandoffArtifact) string {
	if artifact.Files != nil {
		for _, f := range artifact.Files {
			if strings.HasPrefix(f, "task:") {
				return strings.TrimPrefix(f, "task:")
			}
		}
	}
	return ""
}

func (hc *HandoffChain) History() []*HandoffArtifact {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	cp := make([]*HandoffArtifact, len(hc.Handoffs))
	copy(cp, hc.Handoffs)
	return cp
}

func (hc *HandoffChain) Len() int {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return len(hc.Handoffs)
}

func (hc *HandoffChain) CurrentAgent() string {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.Current
}

func (hc *HandoffChain) AgentSequence() []string {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	seen := make(map[string]bool)
	var seq []string
	for _, art := range hc.Handoffs {
		if !seen[art.FromAgent] {
			seq = append(seq, art.FromAgent)
			seen[art.FromAgent] = true
		}
		if !seen[art.ToAgent] {
			seq = append(seq, art.ToAgent)
			seen[art.ToAgent] = true
		}
	}
	return seq
}
