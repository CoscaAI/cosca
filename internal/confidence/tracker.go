// Package confidence provides an Agent Confidence Tracker that monitors
// and queries agent confidence scores across domains. It serves as the
// single source of truth for agent reliability metrics in the Cosca platform.
//
// Confidence scores are calibrated from the Capability Confidence Model
// (Wave 2) and are updated as agents evolve through usage.
package confidence

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/rs/zerolog"
)

// Domain represents a functional area that agents operate in.
// Each agent is assigned to one or more domains based on their capabilities.
type Domain string

// Predefined domains matching Cosca agent specializations.
const (
	DomainQualityAssurance   Domain = "quality_assurance"
	DomainSecurityCompliance Domain = "security_compliance"
	DomainOperations         Domain = "operations"
	DomainPerformance        Domain = "performance"
	DomainCodeHealth         Domain = "code_health"
	DomainGovernance         Domain = "governance"
)

// AgentScore represents an agent's confidence result for a specific domain.
type AgentScore struct {
	Name       string  `json:"name"`
	Domain     Domain  `json:"domain"`
	Confidence float64 `json:"confidence"`
}

// DomainSummary provides aggregated confidence statistics for a domain.
type DomainSummary struct {
	Domain            Domain  `json:"domain"`
	AverageConfidence float64 `json:"average_confidence"`
	AgentCount        int     `json:"agent_count"`
}

// Tracker is the Agent Confidence Tracker. It maintains a registry of agent
// confidence profiles and exposes query methods for monitoring agent reliability.
//
// Thread safety is guaranteed via a read-write mutex. All exported methods
// are safe for concurrent use.
type Tracker struct {
	mu      sync.RWMutex
	log     zerolog.Logger
	agents  map[string]*agentProfile
	domains map[Domain][]string // maps domain → agent names
}

// agentProfile holds the internal confidence data for a single agent.
type agentProfile struct {
	Name       string
	Confidence float64
	Domains    []Domain
}

// NewTracker creates a new Agent Confidence Tracker preloaded with the
// Capability Confidence Model (Wave 2) agent scores.
//
// The tracker initializes with 10 agents activated during Wave 2 with
// their calibrated confidence scores. These scores represent the initial
// confidence model and are updated as agents gain operational history.
func NewTracker(log zerolog.Logger) *Tracker {
	t := &Tracker{
		log:     log,
		agents:  make(map[string]*agentProfile),
		domains: make(map[Domain][]string),
	}
	t.loadWave2Profiles()
	return t
}

// loadWave2Profiles registers the 10 Wave 2 agents with their calibrated
// confidence scores and domain assignments.
func (t *Tracker) loadWave2Profiles() {
	profiles := []agentProfile{
		{Name: "cosca-qa", Confidence: 0.50, Domains: []Domain{DomainQualityAssurance}},
		{Name: "cosca-governance", Confidence: 0.45, Domains: []Domain{DomainGovernance, DomainSecurityCompliance}},
		{Name: "cosca-technical-debt", Confidence: 0.50, Domains: []Domain{DomainCodeHealth}},
		{Name: "cosca-critic", Confidence: 0.40, Domains: []Domain{DomainGovernance}},
		{Name: "cosca-compliance", Confidence: 0.55, Domains: []Domain{DomainSecurityCompliance}},
		{Name: "cosca-testing", Confidence: 0.40, Domains: []Domain{DomainQualityAssurance}},
		{Name: "cosca-performance", Confidence: 0.60, Domains: []Domain{DomainPerformance}},
		{Name: "cosca-devops", Confidence: 0.75, Domains: []Domain{DomainOperations}},
		{Name: "cosca-review", Confidence: 0.45, Domains: []Domain{DomainQualityAssurance, DomainCodeHealth}},
		{Name: "cosca-monitoring", Confidence: 0.72, Domains: []Domain{DomainOperations}},
	}

	for _, p := range profiles {
		t.agents[p.Name] = &agentProfile{
			Name:       p.Name,
			Confidence: p.Confidence,
			Domains:    p.Domains,
		}
		for _, d := range p.Domains {
			t.domains[d] = append(t.domains[d], p.Name)
		}
	}

	t.log.Info().
		Int("agents", len(t.agents)).
		Int("domains", len(t.domains)).
		Msg("confidence tracker initialized with Wave 2 profiles")
}

// GetConfidence returns the confidence score for a specific agent in a
// specific domain. The agent name is matched case-insensitively.
//
// Returns an error if the agent is not registered or does not operate in
// the requested domain.
func (t *Tracker) GetConfidence(agentName string, domain Domain) (float64, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	profile, err := t.lookupAgent(agentName)
	if err != nil {
		return 0, err
	}

	for _, d := range profile.Domains {
		if d == domain {
			t.log.Debug().
				Str("agent", profile.Name).
				Str("domain", string(domain)).
				Float64("confidence", profile.Confidence).
				Msg("confidence retrieved")
			return profile.Confidence, nil
		}
	}

	return 0, fmt.Errorf("agent %q does not operate in domain %q", agentName, domain)
}

// GetAverageConfidence returns the arithmetic mean of all registered agents'
// confidence scores. Returns 0.0 when no agents are registered.
func (t *Tracker) GetAverageConfidence() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.agents) == 0 {
		return 0.0
	}

	var total float64
	for _, p := range t.agents {
		total += p.Confidence
	}
	return total / float64(len(t.agents))
}

// GetTopPerformers returns the highest-confidence agents for a given domain,
// ordered from highest to lowest confidence. The limit parameter caps the
// number of results returned. Pass limit <= 0 to return all agents in the domain.
//
// Returns an empty slice if no agents are registered for the domain.
func (t *Tracker) GetTopPerformers(domain Domain, limit int) []AgentScore {
	t.mu.RLock()
	defer t.mu.RUnlock()

	agentNames, ok := t.domains[domain]
	if !ok || len(agentNames) == 0 {
		return []AgentScore{}
	}

	scores := make([]AgentScore, 0, len(agentNames))
	for _, name := range agentNames {
		profile, ok := t.agents[name]
		if !ok {
			continue
		}
		scores = append(scores, AgentScore{
			Name:       profile.Name,
			Domain:     domain,
			Confidence: profile.Confidence,
		})
	}

	// Sort by confidence descending, then by name ascending for stability.
	sort.Slice(scores, func(i, j int) bool {
		if scores[i].Confidence != scores[j].Confidence {
			return scores[i].Confidence > scores[j].Confidence
		}
		return scores[i].Name < scores[j].Name
	})

	if limit > 0 && limit < len(scores) {
		scores = scores[:limit]
	}

	return scores
}

// GetDomainSummary returns aggregated confidence statistics for a domain,
// including the average confidence and number of agents operating in it.
// Returns an error if the domain has no registered agents.
func (t *Tracker) GetDomainSummary(domain Domain) (*DomainSummary, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	agentNames, ok := t.domains[domain]
	if !ok || len(agentNames) == 0 {
		return nil, fmt.Errorf("domain %q has no registered agents", domain)
	}

	var total float64
	count := 0
	for _, name := range agentNames {
		if profile, ok := t.agents[name]; ok {
			total += profile.Confidence
			count++
		}
	}

	if count == 0 {
		return nil, fmt.Errorf("domain %q has no active agents", domain)
	}

	return &DomainSummary{
		Domain:            domain,
		AverageConfidence: total / float64(count),
		AgentCount:        count,
	}, nil
}

// GetAgentDomains returns all domains that a specific agent operates in.
// The agent name is matched case-insensitively.
// Returns an error if the agent is not registered.
func (t *Tracker) GetAgentDomains(agentName string) ([]Domain, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	profile, err := t.lookupAgent(agentName)
	if err != nil {
		return nil, err
	}

	domains := make([]Domain, len(profile.Domains))
	copy(domains, profile.Domains)
	return domains, nil
}

// ListAgents returns the names of all registered agents.
func (t *Tracker) ListAgents() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	names := make([]string, 0, len(t.agents))
	for name := range t.agents {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ListDomains returns all registered domains.
func (t *Tracker) ListDomains() []Domain {
	t.mu.RLock()
	defer t.mu.RUnlock()

	domains := make([]Domain, 0, len(t.domains))
	for d := range t.domains {
		domains = append(domains, d)
	}
	sort.Slice(domains, func(i, j int) bool {
		return string(domains[i]) < string(domains[j])
	})
	return domains
}

// lookupAgent finds an agent profile by name (case-insensitive). Caller must
// hold at least a read lock.
func (t *Tracker) lookupAgent(name string) (*agentProfile, error) {
	// Exact match first.
	if p, ok := t.agents[name]; ok {
		return p, nil
	}

	// Case-insensitive fallback.
	lower := strings.ToLower(name)
	for key, p := range t.agents {
		if strings.ToLower(key) == lower {
			return p, nil
		}
	}

	return nil, fmt.Errorf("agent %q not found in confidence tracker", name)
}

// Register adds or updates an agent's confidence profile. This allows the
// tracker to be extended with results from new Capability Confidence Model
// waves or programmatic updates from the monitoring system.
func (t *Tracker) Register(name string, confidence float64, domains []Domain) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Validate confidence range.
	if confidence < 0.0 {
		confidence = 0.0
	}
	if confidence > 1.0 {
		confidence = 1.0
	}

	// Remove old domain registrations for this agent.
	for d, names := range t.domains {
		filtered := make([]string, 0, len(names))
		for _, n := range names {
			if !strings.EqualFold(n, name) {
				filtered = append(filtered, n)
			}
		}
		if len(filtered) == 0 {
			delete(t.domains, d)
		} else {
			t.domains[d] = filtered
		}
	}

	// Register new domains.
	normalizedDomains := make([]Domain, len(domains))
	copy(normalizedDomains, domains)

	t.agents[name] = &agentProfile{
		Name:       name,
		Confidence: confidence,
		Domains:    normalizedDomains,
	}

	for _, d := range normalizedDomains {
		t.domains[d] = append(t.domains[d], name)
	}

	t.log.Info().
		Str("agent", name).
		Float64("confidence", confidence).
		Int("domains", len(normalizedDomains)).
		Msg("agent registered in confidence tracker")
}
