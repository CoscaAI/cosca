package adapter

import (
	"github.com/CoscaAI/cosca/internal/agents"
	"github.com/CoscaAI/cosca/internal/orchestration"
	"github.com/CoscaAI/cosca/internal/results"
	"github.com/CoscaAI/cosca/internal/skills"
)

// AgentResolverAdapter adapts agents.Manager to the
// orchestration.AgentResolver port interface. It is the single shared
// implementation used by both the CLI (`cosca run` / `cosca chat`) and the
// REST API (`POST /v1/run`) so both entry points route through the same
// orchestration engine with identical agent resolution behaviour.
type AgentResolverAdapter struct {
	mgr *agents.Manager
	// degradedReason, quando não vazio, marca as operações bem-sucedidas como
	// Degraded (paridade D6).
	degradedReason string
}

// NewAgentResolverAdapter creates a new AgentResolverAdapter backed by the
// given agent manager.
func NewAgentResolverAdapter(mgr *agents.Manager) *AgentResolverAdapter {
	return &AgentResolverAdapter{mgr: mgr}
}

// SetDegraded marca o adapter como degradado com o motivo informado.
func (a *AgentResolverAdapter) SetDegraded(reason string) {
	a.degradedReason = reason
}

// Get returns an agent by name as an orchestration.AgentInfo.
func (a *AgentResolverAdapter) Get(name string) (*orchestration.AgentInfo, error) {
	agent, err := a.mgr.Get(name)
	if err != nil {
		return nil, err
	}
	return &orchestration.AgentInfo{
		Name:            agent.Name,
		Role:            agent.Role,
		Department:      agent.Department,
		Description:     agent.Description,
		Capabilities:    agentCapabilityNames(agent),
		Responsibilities: agent.Responsibilities,
	}, nil
}

// GetResult é a forma com envelope (paridade D6) de Get. Aditivo.
func (a *AgentResolverAdapter) GetResult(name string) *results.Result {
	data, err := a.Get(name)
	return adapterResult(data, err, a.degradedReason)
}

// Search finds agents matching a query string.
func (a *AgentResolverAdapter) Search(query string) ([]orchestration.AgentInfo, error) {
	results, err := a.mgr.Search(query)
	if err != nil {
		return nil, err
	}
	infos := make([]orchestration.AgentInfo, 0, len(results))
	for i := range results {
		r := &results[i]
		infos = append(infos, orchestration.AgentInfo{
			Name:            r.Name,
			Role:            r.Role,
			Department:      r.Department,
			Description:     r.Description,
			Capabilities:    agentCapabilityNames(r),
			Responsibilities: r.Responsibilities,
		})
	}
	return infos, nil
}

// SearchResult é a forma com envelope (paridade D6) de Search. Aditivo.
func (a *AgentResolverAdapter) SearchResult(query string) *results.Result {
	data, err := a.Search(query)
	return adapterResult(data, err, a.degradedReason)
}

// Compile-time check: AgentResolverAdapter implements AgentResolver.
var _ orchestration.AgentResolver = (*AgentResolverAdapter)(nil)

// agentCapabilityNames extrai o nome de cada capability de um agents.Agent,
// retornando nil quando o agente nao tem capabilities declaradas (evita
// slices vazios que seriam serializados como [] em vez de omitidos).
func agentCapabilityNames(a *agents.Agent) []string {
	if a == nil || len(a.Capabilities) == 0 {
		return nil
	}
	names := make([]string, 0, len(a.Capabilities))
	for _, c := range a.Capabilities {
		if c.Name != "" {
			names = append(names, c.Name)
		}
	}
	if len(names) == 0 {
		return nil
	}
	return names
}

// SkillResolverAdapter adapts skills.Manager to the
// orchestration.SkillResolver port interface. Shared between the CLI and the
// REST API so both entry points use identical skill resolution behaviour.
type SkillResolverAdapter struct {
	mgr *skills.Manager
	// degradedReason, quando não vazio, marca as operações bem-sucedidas como
	// Degraded (paridade D6).
	degradedReason string
}

// NewSkillResolverAdapter creates a new SkillResolverAdapter backed by the
// given skill manager.
func NewSkillResolverAdapter(mgr *skills.Manager) *SkillResolverAdapter {
	return &SkillResolverAdapter{mgr: mgr}
}

// SetDegraded marca o adapter como degradado com o motivo informado.
func (a *SkillResolverAdapter) SetDegraded(reason string) {
	a.degradedReason = reason
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

// GetResult é a forma com envelope (paridade D6) de Get. Aditivo.
func (a *SkillResolverAdapter) GetResult(name string) *results.Result {
	data, err := a.Get(name)
	return adapterResult(data, err, a.degradedReason)
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

// SearchResult é a forma com envelope (paridade D6) de Search. Aditivo.
func (a *SkillResolverAdapter) SearchResult(query string) *results.Result {
	data, err := a.Search(query)
	return adapterResult(data, err, a.degradedReason)
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
