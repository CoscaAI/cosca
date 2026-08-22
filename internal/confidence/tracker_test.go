package confidence

import (
	"sort"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nopLogger returns a logger that discards all output.
func nopLogger() zerolog.Logger {
	return zerolog.Nop()
}

// ---------------------------------------------------------------------------
// NewTracker — initialization
// ---------------------------------------------------------------------------

func TestNewTracker_InitializesWithWave2Profiles(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	agents := tr.ListAgents()
	assert.Len(t, agents, 10, "Wave 2 should have exactly 10 agents")

	expectedAgents := []string{
		"cosca-compliance",
		"cosca-critic",
		"cosca-devops",
		"cosca-governance",
		"cosca-monitoring",
		"cosca-performance",
		"cosca-qa",
		"cosca-review",
		"cosca-technical-debt",
		"cosca-testing",
	}
	assert.Equal(t, expectedAgents, agents)

	domains := tr.ListDomains()
	assert.Len(t, domains, 6)
}

// ---------------------------------------------------------------------------
// GetConfidence
// ---------------------------------------------------------------------------

func TestGetConfidence_ExactMatch(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	tests := []struct {
		agent  string
		domain Domain
		want   float64
	}{
		{"cosca-qa", DomainQualityAssurance, 0.50},
		{"cosca-devops", DomainOperations, 0.75},
		{"cosca-performance", DomainPerformance, 0.60},
		{"cosca-compliance", DomainSecurityCompliance, 0.55},
		{"cosca-technical-debt", DomainCodeHealth, 0.50},
		{"cosca-monitoring", DomainOperations, 0.72},
		{"cosca-critic", DomainGovernance, 0.40},
		{"cosca-governance", DomainGovernance, 0.45},
		{"cosca-testing", DomainQualityAssurance, 0.40},
		{"cosca-review", DomainQualityAssurance, 0.45},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.agent+"/"+string(tt.domain), func(t *testing.T) {
			t.Parallel()

			conf, err := tr.GetConfidence(tt.agent, tt.domain)
			require.NoError(t, err)
			assert.Equal(t, tt.want, conf)
		})
	}
}

func TestGetConfidence_CaseInsensitive(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	conf, err := tr.GetConfidence("COSCA-DEVOPS", DomainOperations)
	require.NoError(t, err)
	assert.Equal(t, 0.75, conf)

	conf, err = tr.GetConfidence("Cosca-Qa", DomainQualityAssurance)
	require.NoError(t, err)
	assert.Equal(t, 0.50, conf)
}

func TestGetConfidence_AgentNotFound(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	_, err := tr.GetConfidence("nonexistent", DomainOperations)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetConfidence_AgentNotInDomain(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	// cosca-devops is registered for operations, not quality_assurance.
	_, err := tr.GetConfidence("cosca-devops", DomainQualityAssurance)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not operate in domain")
}

func TestGetConfidence_MultiDomainAgent(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	// cosca-review operates in quality_assurance AND code_health.
	conf, err := tr.GetConfidence("cosca-review", DomainQualityAssurance)
	require.NoError(t, err)
	assert.Equal(t, 0.45, conf)

	conf, err = tr.GetConfidence("cosca-review", DomainCodeHealth)
	require.NoError(t, err)
	assert.Equal(t, 0.45, conf)

	// cosca-governance is in governance AND security_compliance.
	conf, err = tr.GetConfidence("cosca-governance", DomainGovernance)
	require.NoError(t, err)
	assert.Equal(t, 0.45, conf)

	conf, err = tr.GetConfidence("cosca-governance", DomainSecurityCompliance)
	require.NoError(t, err)
	assert.Equal(t, 0.45, conf)
}

// ---------------------------------------------------------------------------
// GetAverageConfidence
// ---------------------------------------------------------------------------

func TestGetAverageConfidence_WithWave2Data(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	avg := tr.GetAverageConfidence()
	// (0.50 + 0.45 + 0.50 + 0.40 + 0.55 + 0.40 + 0.60 + 0.75 + 0.45 + 0.72) / 10 = 5.32 / 10 = 0.532
	assert.InEpsilon(t, 0.532, avg, 0.001)
}

func TestGetAverageConfidence_EmptyTracker(t *testing.T) {
	t.Parallel()

	tr := &Tracker{
		log:    nopLogger(),
		agents: make(map[string]*agentProfile),
	}

	avg := tr.GetAverageConfidence()
	assert.Equal(t, 0.0, avg)
}

// ---------------------------------------------------------------------------
// GetTopPerformers
// ---------------------------------------------------------------------------

func TestGetTopPerformers_AllAgentsInDomain(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	// DomainQualityAssurance has: cosca-qa (0.50), cosca-testing (0.40), cosca-review (0.45)
	performers := tr.GetTopPerformers(DomainQualityAssurance, 0)
	assert.Len(t, performers, 3)

	// Should be sorted by confidence descending.
	expected := []struct {
		name       string
		confidence float64
	}{
		{"cosca-qa", 0.50},
		{"cosca-review", 0.45},
		{"cosca-testing", 0.40},
	}

	for i, exp := range expected {
		assert.Equal(t, exp.name, performers[i].Name)
		assert.Equal(t, exp.confidence, performers[i].Confidence)
		assert.Equal(t, DomainQualityAssurance, performers[i].Domain)
	}
}

func TestGetTopPerformers_WithLimit(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	// DomainQualityAssurance has 3 agents; limit to 2.
	performers := tr.GetTopPerformers(DomainQualityAssurance, 2)
	assert.Len(t, performers, 2)
	assert.Equal(t, "cosca-qa", performers[0].Name)
	assert.Equal(t, "cosca-review", performers[1].Name)
}

func TestGetTopPerformers_LimitGreaterThanAvailable(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	// DomainPerformance has only 1 agent; limit to 5.
	performers := tr.GetTopPerformers(DomainPerformance, 5)
	assert.Len(t, performers, 1)
	assert.Equal(t, "cosca-performance", performers[0].Name)
}

func TestGetTopPerformers_UnknownDomain(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	performers := tr.GetTopPerformers("unknown_domain", 10)
	assert.Empty(t, performers)
}

func TestGetTopPerformers_StableSortingOnTies(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	// Add agents with same confidence to test tie-breaking.
	tr.Register("agent-alpha", 0.60, []Domain{"test_domain"})
	tr.Register("agent-beta", 0.60, []Domain{"test_domain"})
	tr.Register("agent-gamma", 0.80, []Domain{"test_domain"})

	performers := tr.GetTopPerformers("test_domain", 0)
	require.Len(t, performers, 3)

	assert.Equal(t, "agent-gamma", performers[0].Name, "highest confidence first")
	// Ties sorted alphabetically.
	assert.Equal(t, "agent-alpha", performers[1].Name)
	assert.Equal(t, "agent-beta", performers[2].Name)
}

// ---------------------------------------------------------------------------
// GetDomainSummary
// ---------------------------------------------------------------------------

func TestGetDomainSummary_SingleAgentDomain(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	// DomainPerformance has only cosca-performance (0.60).
	summary, err := tr.GetDomainSummary(DomainPerformance)
	require.NoError(t, err)
	assert.Equal(t, DomainPerformance, summary.Domain)
	assert.Equal(t, 0.60, summary.AverageConfidence)
	assert.Equal(t, 1, summary.AgentCount)
}

func TestGetDomainSummary_MultiAgentDomain(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	// DomainOperations: cosca-devops (0.75) + cosca-monitoring (0.72) → avg 0.735, 2 agents.
	summary, err := tr.GetDomainSummary(DomainOperations)
	require.NoError(t, err)
	assert.Equal(t, DomainOperations, summary.Domain)
	assert.InEpsilon(t, 0.735, summary.AverageConfidence, 0.001)
	assert.Equal(t, 2, summary.AgentCount)
}

func TestGetDomainSummary_QualityAssurance(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	// DomainQualityAssurance: qa (0.50), testing (0.40), review (0.45) → avg = 1.35/3 = 0.45.
	summary, err := tr.GetDomainSummary(DomainQualityAssurance)
	require.NoError(t, err)
	assert.InEpsilon(t, 0.45, summary.AverageConfidence, 0.001)
	assert.Equal(t, 3, summary.AgentCount)
}

func TestGetDomainSummary_UnknownDomain(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	_, err := tr.GetDomainSummary("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no registered agents")
}

// ---------------------------------------------------------------------------
// GetAgentDomains
// ---------------------------------------------------------------------------

func TestGetAgentDomains_SingleDomainAgent(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	domains, err := tr.GetAgentDomains("cosca-devops")
	require.NoError(t, err)
	assert.Len(t, domains, 1)
	assert.Equal(t, DomainOperations, domains[0])
}

func TestGetAgentDomains_MultiDomainAgent(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	domains, err := tr.GetAgentDomains("cosca-review")
	require.NoError(t, err)
	assert.Len(t, domains, 2)
	assert.Contains(t, domains, DomainQualityAssurance)
	assert.Contains(t, domains, DomainCodeHealth)
}

func TestGetAgentDomains_NotFound(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	_, err := tr.GetAgentDomains("no-such-agent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ---------------------------------------------------------------------------
// ListAgents
// ---------------------------------------------------------------------------

func TestListAgents_Sorted(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	agents := tr.ListAgents()
	assert.True(t, sort.StringsAreSorted(agents), "agents should be sorted alphabetically")
}

// ---------------------------------------------------------------------------
// ListDomains
// ---------------------------------------------------------------------------

func TestListDomains_AllSixDomainsPresent(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	domains := tr.ListDomains()
	assert.Len(t, domains, 6)

	expected := []Domain{
		DomainCodeHealth,
		DomainGovernance,
		DomainOperations,
		DomainPerformance,
		DomainQualityAssurance,
		DomainSecurityCompliance,
	}
	assert.Equal(t, expected, domains)
}

// ---------------------------------------------------------------------------
// Register — adding/updating agents
// ---------------------------------------------------------------------------

func TestRegister_AddNewAgent(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	tr.Register("cosca-newbie", 0.35, []Domain{DomainCodeHealth})

	conf, err := tr.GetConfidence("cosca-newbie", DomainCodeHealth)
	require.NoError(t, err)
	assert.Equal(t, 0.35, conf)

	avg := tr.GetAverageConfidence()
	// 11 agents now: original sum 5.32 + 0.35 = 5.67 / 11 ≈ 0.51545
	assert.InEpsilon(t, 5.67/11.0, avg, 0.001)

	// Verify it appears in top performers.
	performers := tr.GetTopPerformers(DomainCodeHealth, 0)
	assert.GreaterOrEqual(t, len(performers), 3) // review, technical-debt, newbie
}

func TestRegister_UpdateExistingAgent(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	// Initially cosca-qa has 0.50 in quality_assurance.
	conf, err := tr.GetConfidence("cosca-qa", DomainQualityAssurance)
	require.NoError(t, err)
	assert.Equal(t, 0.50, conf)

	// Update with new confidence and additional domains.
	tr.Register("cosca-qa", 0.85, []Domain{DomainQualityAssurance, DomainCodeHealth})

	conf, err = tr.GetConfidence("cosca-qa", DomainQualityAssurance)
	require.NoError(t, err)
	assert.Equal(t, 0.85, conf)

	// Should now also be in code_health.
	conf, err = tr.GetConfidence("cosca-qa", DomainCodeHealth)
	require.NoError(t, err)
	assert.Equal(t, 0.85, conf)

	// Old domain registrations should be gone if not re-declared.
	// Since we used only quality_assurance and code_health, operations should not include it.
	// (qa wasn't in operations before either, so this is more about correctness of cleanup.)
}

func TestRegister_ClampingConfidence(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	// Below range: clamp to 0.0.
	tr.Register("agent-low", -0.5, []Domain{DomainPerformance})
	conf, err := tr.GetConfidence("agent-low", DomainPerformance)
	require.NoError(t, err)
	assert.Equal(t, 0.0, conf)

	// Above range: clamp to 1.0.
	tr.Register("agent-high", 1.5, []Domain{DomainPerformance})
	conf, err = tr.GetConfidence("agent-high", DomainPerformance)
	require.NoError(t, err)
	assert.Equal(t, 1.0, conf)
}

func TestRegister_RemovesOldDomains(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	// cosca-governance is initially in governance and security_compliance.
	domains, _ := tr.GetAgentDomains("cosca-governance")
	assert.Contains(t, domains, DomainGovernance)
	assert.Contains(t, domains, DomainSecurityCompliance)

	// Re-register with only governance.
	tr.Register("cosca-governance", 0.50, []Domain{DomainGovernance})

	domains, _ = tr.GetAgentDomains("cosca-governance")
	assert.Len(t, domains, 1)
	assert.Equal(t, DomainGovernance, domains[0])

	// Verify security_compliance domain no longer includes governance.
	performers := tr.GetTopPerformers(DomainSecurityCompliance, 0)
	for _, p := range performers {
		assert.NotEqual(t, "cosca-governance", p.Name, "governance should be removed from security_compliance")
	}
}

// ---------------------------------------------------------------------------
// Concurrency safety
// ---------------------------------------------------------------------------

func TestTracker_ConcurrentReads(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	// Launch multiple goroutines that read concurrently.
	done := make(chan struct{})
	for i := 0; i < 20; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				tr.GetConfidence("cosca-devops", DomainOperations)
				tr.GetAverageConfidence()
				tr.GetTopPerformers(DomainQualityAssurance, 3)
				tr.ListAgents()
				tr.ListDomains()
				tr.GetDomainSummary(DomainPerformance)
				tr.GetAgentDomains("cosca-review")
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestTracker_ConcurrentReadWrite(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	done := make(chan struct{})

	// Writers: register agents with incrementing IDs.
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 50; j++ {
				name := "agent-" + string(rune('A'+id)) + "-" + string(rune('0'+j%10))
				tr.Register(name, 0.5+float64(j%10)*0.05, []Domain{DomainPerformance})
			}
			done <- struct{}{}
		}(i)
	}

	// Readers: query while writes are happening.
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				tr.GetAverageConfidence()
				tr.ListAgents()
				tr.ListDomains()
				if performers := tr.GetTopPerformers(DomainPerformance, 5); len(performers) > 0 {
					_ = performers[0].Confidence
				}
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 15; i++ {
		<-done
	}
}

// ---------------------------------------------------------------------------
// Wave 2 data integrity
// ---------------------------------------------------------------------------

func TestWave2Data_AllAgentsPresent(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	expected := map[string]float64{
		"cosca-qa":             0.50,
		"cosca-governance":     0.45,
		"cosca-technical-debt": 0.50,
		"cosca-critic":         0.40,
		"cosca-compliance":     0.55,
		"cosca-testing":        0.40,
		"cosca-performance":    0.60,
		"cosca-devops":         0.75,
		"cosca-review":         0.45,
		"cosca-monitoring":     0.72,
	}

	for agentName, expectedConf := range expected {
		// Find a domain this agent operates in.
		domains, err := tr.GetAgentDomains(agentName)
		require.NoError(t, err)
		require.NotEmpty(t, domains, "agent %q should have at least one domain", agentName)

		conf, err := tr.GetConfidence(agentName, domains[0])
		require.NoError(t, err)
		assert.Equal(t, expectedConf, conf, "agent %q has wrong confidence", agentName)
	}
}

// ---------------------------------------------------------------------------
// Domain coverage
// ---------------------------------------------------------------------------

func TestDomainCoverage_EveryDomainHasAgents(t *testing.T) {
	t.Parallel()

	tr := NewTracker(nopLogger())

	allDomains := tr.ListDomains()
	for _, d := range allDomains {
		summary, err := tr.GetDomainSummary(d)
		require.NoError(t, err, "domain %q should have agents", d)
		assert.Greater(t, summary.AgentCount, 0, "domain %q should have at least 1 agent", d)
		assert.Greater(t, summary.AverageConfidence, 0.0, "domain %q should have positive confidence", d)
	}
}

// ---------------------------------------------------------------------------
// AgentScore struct integrity
// ---------------------------------------------------------------------------

func TestAgentScore_Fields(t *testing.T) {
	t.Parallel()

	score := AgentScore{
		Name:       "cosca-qa",
		Domain:     DomainQualityAssurance,
		Confidence: 0.50,
	}

	assert.Equal(t, "cosca-qa", score.Name)
	assert.Equal(t, DomainQualityAssurance, score.Domain)
	assert.Equal(t, 0.50, score.Confidence)
}

// ---------------------------------------------------------------------------
// DomainSummary struct integrity
// ---------------------------------------------------------------------------

func TestDomainSummary_Fields(t *testing.T) {
	t.Parallel()

	summary := DomainSummary{
		Domain:            DomainOperations,
		AverageConfidence: 0.735,
		AgentCount:        2,
	}

	assert.Equal(t, DomainOperations, summary.Domain)
	assert.Equal(t, 0.735, summary.AverageConfidence)
	assert.Equal(t, 2, summary.AgentCount)
}
