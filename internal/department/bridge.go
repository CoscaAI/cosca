package department

import "strings"

// AgentDepartmentMap maps the 55 agent types to their departments.
// This bridges the gap between the department system (6 hardcoded) and
// the 55+ agent definitions.
var AgentDepartmentMap = map[string]string{
	"cosca-kernel":                          "kernel",
	"cosca-ceo":                             "executive",
	"cosca-cto":                             "executive",
	"cosca-product":                         "executive",
	"cosca-governance":                      "executive",
	"cosca-security":                        "security",
	"cosca-compliance":                      "security",
	"cosca-backend":                         "developer",
	"cosca-frontend":                        "developer",
	"cosca-api":                             "developer",
	"cosca-database":                        "developer",
	"cosca-mobile":                          "developer",
	"cosca-cli":                             "developer",
	"cosca-sdk":                             "developer",
	"cosca-testing":                         "developer",
	"cosca-qa":                              "developer",
	"cosca-review":                          "developer",
	"cosca-documentation":                   "developer",
	"cosca-workflow":                        "developer",
	"cosca-integrations":                    "developer",
	"cosca-plugin":                          "developer",
	"cosca-provider":                        "developer",
	"cosca-technical-debt":                  "developer",
	"cosca-migration":                       "developer",
	"cosca-bootstrap":                       "developer",
	"cosca-uiux":                            "developer",
	"cosca-messaging":                       "developer",
	"cosca-release":                         "developer",
	"cosca-critic":                          "developer",
	"cosca-paradigm":                        "developer",
	"cosca-evolution":                       "developer",
	"cosca-architecture":                    "developer",
	"cosca-cache":                           "developer",
	"cosca-automation":                      "developer",
	"cosca-specialist-backend-api":          "developer",
	"cosca-specialist-backend-service":      "developer",
	"cosca-specialist-database-sql":         "developer",
	"cosca-specialist-documentation-writer": "developer",
	"cosca-specialist-frontend-component":   "developer",
	"cosca-specialist-review-code":          "developer",
	"cosca-specialist-testing-unit":         "developer",
	"cosca-specialist-testing-integration":  "developer",
	"cosca-specialist-testing-e2e":          "developer",
	"cosca-ai":                              "research",
	"cosca-analytics":                       "research",
	"cosca-discovery":                       "research",
	"cosca-context":                         "research",
	"cosca-memory":                          "kernel",
	"cosca-semantic-memory":                 "kernel",
	"cosca-devops":                          "operations",
	"cosca-infrastructure":                  "operations",
	"cosca-monitoring":                      "operations",
	"cosca-performance":                     "operations",
	"cosca-platform":                        "operations",
	"cosca-runtime":                         "operations",
}

// MapAgentToDepartment returns the department for an agent name.
func MapAgentToDepartment(agentName string) string {
	if dept, ok := AgentDepartmentMap[agentName]; ok {
		return dept
	}
	for prefix, dept := range AgentDepartmentMap {
		if strings.HasPrefix(agentName, prefix) {
			return dept
		}
	}
	return "developer"
}
