package adapter

import (
	"github.com/CoscaAI/cosca/internal/agents"
	"github.com/CoscaAI/cosca/internal/orchestration"
	"github.com/CoscaAI/cosca/internal/skills"
)

// AgentResolverAdapter adapts agents.Manager to the
// orchestration.AgentResolver port interface. It is the single shared
// implementation used by both the CLI (`cosca run` / `cosca chat`) and the
// REST API (`POST /v1/run`) so both entry points route through the same
// orchestration engine with identical agent resolution behaviour.
type AgentResolverAdapter struct {
	mgr *agents.Manager
}

// NewAgentResolverAdapter creates a new AgentResolverAdapter backed by the
// given agent manager.
func NewAgentResolverAdapter(mgr *agents.Manager) *AgentResolverAdapter {
	return &AgentResolverAdapter{mgr: mgr}
}

// Get returns an agent by name as an orchestration.AgentInfo.
func (a *AgentResolverAdapter) Get(name string) (*orchestration.AgentInfo, error) {
	agent, err := a.mgr.Get(name)
	if err != nil {
		return nil, err
	}
	return &orchestration.AgentInfo{
		Name:        agent.Name,
		Role:        agent.Role,
		Department:  agent.Department,
		Description: agent.Description,
	}, nil
}

// Search finds agents matching a query string.
func (a *AgentResolverAdapter) Search(query string) ([]orchestration.AgentInfo, error) {
	results, err := a.mgr.Search(query)
	if err != nil {
		return nil, err
	}
	infos := make([]orchestration.AgentInfo, 0, len(results))
	for _, r := range results {
		infos = append(infos, orchestration.AgentInfo{
			Name:        r.Name,
			Role:        r.Role,
			Department:  r.Department,
			Description: r.Description,
		})
	}
	return infos, nil
}

// Compile-time check: AgentResolverAdapter implements AgentResolver.
var _ orchestration.AgentResolver = (*AgentResolverAdapter)(nil)

// SkillResolverAdapter adapts skills.Manager to the
// orchestration.SkillResolver port interface. Shared between the CLI and the
// REST API so both entry points use identical skill resolution behaviour.
type SkillResolverAdapter struct {
	mgr *skills.Manager
}

// NewSkillResolverAdapter creates a new SkillResolverAdapter backed by the
// given skill manager.
func NewSkillResolverAdapter(mgr *skills.Manager) *SkillResolverAdapter {
	return &SkillResolverAdapter{mgr: mgr}
}

// Get returns a skill by name as an orchestration.SkillInfo.
func (a *SkillResolverAdapter) Get(name string) (*orchestration.SkillInfo, error) {
	s, err := a.mgr.Get(name)
	if err != nil {
		return nil, err
	}
	return &orchestration.SkillInfo{
		Name:        s.Name,
		Description: s.Description,
		Category:    s.Category,
	}, nil
}

// Search finds skills matching a query string.
func (a *SkillResolverAdapter) Search(query string) ([]orchestration.SkillInfo, error) {
	results, err := a.mgr.Search(query)
	if err != nil {
		return nil, err
	}
	infos := make([]orchestration.SkillInfo, 0, len(results))
	for _, r := range results {
		infos = append(infos, orchestration.SkillInfo{
			Name:        r.Name,
			Description: r.Description,
			Category:    r.Category,
		})
	}
	return infos, nil
}

// List returns all available skills.
func (a *SkillResolverAdapter) List() ([]orchestration.SkillInfo, error) {
	results := a.mgr.List()
	infos := make([]orchestration.SkillInfo, 0, len(results))
	for _, r := range results {
		infos = append(infos, orchestration.SkillInfo{
			Name:        r.Name,
			Description: r.Description,
			Category:    r.Category,
		})
	}
	return infos, nil
}

// Compile-time check: SkillResolverAdapter implements SkillResolver.
var _ orchestration.SkillResolver = (*SkillResolverAdapter)(nil)
