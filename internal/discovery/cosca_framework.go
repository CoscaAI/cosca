// Package discovery provides workspace and Cosca framework discovery.
package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/CoscaAI/cosca/internal/agents"
	"github.com/CoscaAI/cosca/internal/prompts"
	"github.com/CoscaAI/cosca/internal/skills"
	"github.com/CoscaAI/cosca/internal/templates"
	"github.com/CoscaAI/cosca/internal/workflows"
	"gopkg.in/yaml.v3"
)

// AOSFramework represents the path to the Cosca framework directory
// and provides methods to discover all framework resources.
type AOSFramework struct {
	Root string
}

// frontmatter holds parsed YAML frontmatter fields common to all resource types.
type frontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Version     string `yaml:"version"`
	Type        string `yaml:"type"`
	Category    string `yaml:"category"`

	// Agent-specific fields
	Role       string `yaml:"role"`
	Mission    string `yaml:"mission"`
	Status     string `yaml:"status"`
	Department string `yaml:"department"`
	ReportsTo  string `yaml:"reports_to"`

	// Workflow-specific fields
	Steps int `yaml:"steps"`

	// Template content
	Content string `yaml:"content"`
}

// NewAOSFramework creates a new Cosca framework reader by finding the
// framework directory. It checks these locations in order:
//
//  1. $COSCA_HOME environment variable
//  2. ~/.config/opencode/cosca/
//  3. ~/.cosca/
//  4. ./.cosca/ in current directory
//  5. /etc/cosca/
func NewAOSFramework() (*AOSFramework, error) {
	// 1. Check $COSCA_HOME environment variable
	if envPath := os.Getenv("COSCA_HOME"); envPath != "" {
		if info, err := os.Stat(envPath); err == nil && info.IsDir() {
			return &AOSFramework{Root: envPath}, nil
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}

	// 2. Check ~/.config/opencode/cosca/
	if homeDir != "" {
		path := filepath.Join(homeDir, ".config", "opencode", "cosca")
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return &AOSFramework{Root: path}, nil
		}
	}

	// 3. Check ~/.cosca/
	if homeDir != "" {
		path := filepath.Join(homeDir, ".cosca")
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return &AOSFramework{Root: path}, nil
		}
	}

	// 4. Check ./.cosca/ in current directory
	if cwd, err := os.Getwd(); err == nil {
		path := filepath.Join(cwd, ".cosca")
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return &AOSFramework{Root: path}, nil
		}
	}

	// 5. Check /etc/cosca/
	if info, err := os.Stat("/etc/cosca"); err == nil && info.IsDir() {
		return &AOSFramework{Root: "/etc/cosca"}, nil
	}

	return nil, fmt.Errorf("cosca framework not found: checked $COSCA_HOME, ~/.config/opencode/cosca/, ~/.cosca/, ./.cosca/, /etc/cosca/")
}

// DiscoverSkills scans the Cosca framework skills directory recursively
// and returns all skills found in .md files.
func (a *AOSFramework) DiscoverSkills() ([]skills.Skill, error) {
	skillsDir := filepath.Join(a.Root, "skills")
	return a.discoverSkills(skillsDir)
}

func (a *AOSFramework) discoverSkills(dir string) ([]skills.Skill, error) {
	var result []skills.Skill

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return result, err
	}

	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			// Recurse into subdirectories
			subSkills, err := a.discoverSkills(fullPath)
			if err != nil {
				continue
			}
			result = append(result, subSkills...)
			continue
		}

		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		// Skip catalog files
		if strings.EqualFold(entry.Name(), "SKILLS_CATALOG.md") {
			continue
		}

		skill, err := a.parseSkillFile(fullPath)
		if err != nil {
			continue
		}
		result = append(result, *skill)
	}

	return result, nil
}

// parseSkillFile reads a single .md skill file and extracts its metadata.
func (a *AOSFramework) parseSkillFile(path string) (*skills.Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading skill file %s: %w", path, err)
	}

	content := string(data)
	fm := parseFrontmatter(content)
	instructions := extractInstructions(content)

	// Build the skill. Frontmatter fields take precedence, then fallback
	// to inline metadata, then to the file name.
	skill := &skills.Skill{
		Name:         firstNonEmpty(fm.Name, extractTitle(content), nameFromPath(path)),
		Description:  firstNonEmpty(fm.Description, extractDescription(content), ""),
		Version:      firstNonEmpty(fm.Version, extractVersion(content), "1.0.0"),
		Category:     firstNonEmpty(fm.Category, extractCategory(content), inferCategoryFromPath(path)),
		Instructions: instructions,
		Tools:        parseToolsFromContent(content),
	}

	return skill, nil
}

// DiscoverWorkflows scans the Cosca framework workflows directory
// and returns all workflows found in .md files.
func (a *AOSFramework) DiscoverWorkflows() ([]workflows.Workflow, error) {
	workflowsDir := filepath.Join(a.Root, "workflows")
	var result []workflows.Workflow

	entries, err := os.ReadDir(workflowsDir)
	if err != nil {
		return result, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		fullPath := filepath.Join(workflowsDir, entry.Name())
		wf, err := a.parseWorkflowFile(fullPath)
		if err != nil {
			continue
		}
		result = append(result, *wf)
	}

	return result, nil
}

// parseWorkflowFile reads a single .md workflow file and extracts its metadata.
func (a *AOSFramework) parseWorkflowFile(path string) (*workflows.Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading workflow file %s: %w", path, err)
	}

	content := string(data)
	fm := parseFrontmatter(content)

	steps := parseWorkflowSteps(content)
	stepCount := len(steps)
	if fm.Steps > 0 {
		stepCount = fm.Steps
	}

	wf := &workflows.Workflow{
		Name:        firstNonEmpty(fm.Name, extractTitle(content), nameFromPath(path)),
		Description: firstNonEmpty(fm.Description, extractDescription(content), ""),
		Version:     firstNonEmpty(fm.Version, extractVersion(content), "1.0.0"),
		Status:      firstNonEmpty(fm.Status, extractStatus(content), "active"),
		Enabled:     true,
		Steps:       stepCount,
		StepList:    steps,
		Inputs:      parseWorkflowIO(content, "INPUTS"),
		Outputs:     parseWorkflowIO(content, "OUTPUTS"),
	}

	return wf, nil
}

// DiscoverAgents scans the Cosca framework departments directory
// looking for SKILL.md files in each department subdirectory.
func (a *AOSFramework) DiscoverAgents() ([]agents.Agent, error) {
	deptsDir := filepath.Join(a.Root, "departments")
	var result []agents.Agent

	entries, err := os.ReadDir(deptsDir)
	if err != nil {
		return result, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(deptsDir, entry.Name(), "SKILL.md")
		agent, err := a.parseAgentFile(skillPath, entry.Name())
		if err != nil {
			continue
		}
		result = append(result, *agent)
	}

	return result, nil
}

// parseAgentFile reads a SKILL.md file from a department directory
// and returns an Agent. The department name is used as the agent name
// if no name is found in frontmatter.
func (a *AOSFramework) parseAgentFile(path string, deptName string) (*agents.Agent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading agent file %s: %w", path, err)
	}

	content := string(data)
	fm := parseFrontmatter(content)

	agent := &agents.Agent{
		Name:             firstNonEmpty(fm.Name, extractTitle(content), deptName),
		Role:             firstNonEmpty(fm.Role, extractRole(content), deptName),
		Mission:          firstNonEmpty(fm.Mission, extractPurpose(content), ""),
		Status:           firstNonEmpty(fm.Status, extractStatus(content), "active"),
		Version:          firstNonEmpty(fm.Version, extractVersion(content), "1.0.0"),
		Department:       firstNonEmpty(fm.Department, deptName, ""),
		ReportsTo:        firstNonEmpty(fm.ReportsTo, extractReportsTo(content), ""),
		Capabilities:     parseCapabilities(content),
		Tools:            parseAgentTools(content),
		Responsibilities: parseResponsibilities(content),
	}

	return agent, nil
}

// DiscoverTemplates scans the Cosca framework templates directory
// and returns all templates found in .md files.
func (a *AOSFramework) DiscoverTemplates() ([]templates.Template, error) {
	templatesDir := filepath.Join(a.Root, "templates")
	return a.discoverTemplates(templatesDir)
}

func (a *AOSFramework) discoverTemplates(dir string) ([]templates.Template, error) {
	var result []templates.Template

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return result, err
	}

	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			subTemplates, err := a.discoverTemplates(fullPath)
			if err != nil {
				continue
			}
			result = append(result, subTemplates...)
			continue
		}

		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		tmpl, err := a.parseTemplateFile(fullPath)
		if err != nil {
			continue
		}
		result = append(result, *tmpl)
	}

	return result, nil
}

// parseTemplateFile reads a single .md template file.
func (a *AOSFramework) parseTemplateFile(path string) (*templates.Template, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading template file %s: %w", path, err)
	}

	content := string(data)
	fm := parseFrontmatter(content)

	tmpl := &templates.Template{
		Name:        firstNonEmpty(fm.Name, extractTitle(content), nameFromPath(path)),
		Description: firstNonEmpty(fm.Description, extractDescription(content), ""),
		Type:        firstNonEmpty(fm.Type, extractTemplateType(content), inferCategoryFromPath(path)),
		Version:     firstNonEmpty(fm.Version, extractVersion(content), "1.0.0"),
		Content:     content,
	}

	return tmpl, nil
}

// DiscoverPrompts scans the Cosca framework prompts directory
// and returns all prompts found. Returns empty slice if no prompts directory exists.
func (a *AOSFramework) DiscoverPrompts() ([]prompts.Prompt, error) {
	promptsDir := filepath.Join(a.Root, "prompts")
	var result []prompts.Prompt

	entries, err := os.ReadDir(promptsDir)
	if err != nil {
		return result, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		fullPath := filepath.Join(promptsDir, entry.Name())
		p, err := a.parsePromptFile(fullPath)
		if err != nil {
			continue
		}
		result = append(result, *p)
	}

	return result, nil
}

// parsePromptFile reads a single .md prompt file.
func (a *AOSFramework) parsePromptFile(path string) (*prompts.Prompt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading prompt file %s: %w", path, err)
	}

	content := string(data)
	fm := parseFrontmatter(content)

	p := &prompts.Prompt{
		Name:        firstNonEmpty(fm.Name, extractTitle(content), nameFromPath(path)),
		Description: firstNonEmpty(fm.Description, extractDescription(content), ""),
		Version:     firstNonEmpty(fm.Version, extractVersion(content), "1.0.0"),
		Category:    firstNonEmpty(fm.Category, extractCategory(content), "general"),
		Template:    content,
		Parameters:  parsePromptParameters(content),
	}

	return p, nil
}

// ---------------------------------------------------------------------------
// Frontmatter parsing
// ---------------------------------------------------------------------------

// parseFrontmatter extracts YAML frontmatter (content between --- markers)
// from a markdown file and returns the parsed fields.
func parseFrontmatter(content string) frontmatter {
	var fm frontmatter

	content = strings.TrimSpace(content)

	// Must start with ---
	if !strings.HasPrefix(content, "---") {
		return fm
	}

	// Find the closing ---
	rest := content[3:]
	endIdx := strings.Index(rest, "\n---")
	if endIdx < 0 {
		return fm
	}

	yamlBlock := rest[:endIdx]
	if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err != nil {
		return frontmatter{}
	}

	return fm
}

// ---------------------------------------------------------------------------
// Inline metadata extractors (for Cosca markdown convention)
// ---------------------------------------------------------------------------

// extractTitle extracts the first H1 heading (# TITLE) from markdown content.
// Handles two patterns:
//   - "# TYPE: Name" (e.g. "# WORKFLOW: code-review") → "code-review"
//   - "# Name TYPE" (e.g. "# UNIT TESTING SKILL") → "UNIT TESTING"
//   - "# Name" (e.g. "# AI CHIEF") → "AI CHIEF"
func extractTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			title := strings.TrimPrefix(trimmed, "# ")
			title = strings.TrimSpace(title)

			// Pattern 1: "TYPE: Name"
			if idx := strings.Index(title, ":"); idx >= 0 {
				// Check if prefix before colon is a known type keyword
				prefix := strings.TrimSpace(title[:idx])
				knownTypes := map[string]bool{
					"WORKFLOW": true, "TEMPLATE": true, "SKILL": true,
					"AGENT": true, "PROMPT": true,
				}
				if knownTypes[strings.ToUpper(prefix)] {
					return strings.TrimSpace(title[idx+1:])
				}
			}

			// Pattern 2: "Name TYPE" — remove known type suffixes
			suffixes := []string{" SKILL", " AGENT", " WORKFLOW", " TEMPLATE", " PROMPT"}
			for _, suffix := range suffixes {
				if len(title) >= len(suffix) &&
					strings.EqualFold(title[len(title)-len(suffix):], suffix) {
					return strings.TrimSpace(title[:len(title)-len(suffix)])
				}
			}

			return title
		}
	}
	return ""
}

// extractDescription extracts the Description section content.
func extractDescription(content string) string {
	lines := strings.Split(content, "\n")
	inDesc := false
	var descParts []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## Description") || strings.HasPrefix(trimmed, "## DESCRIPTION") {
			inDesc = true
			continue
		}
		if inDesc {
			// Stop at next heading or empty line followed by table
			if strings.HasPrefix(trimmed, "## ") {
				break
			}
			if strings.HasPrefix(trimmed, "|") || strings.HasPrefix(trimmed, "## Inputs") || strings.HasPrefix(trimmed, "#") {
				break
			}
			if trimmed != "" {
				descParts = append(descParts, trimmed)
			}
		}
	}
	return strings.Join(descParts, " ")
}

// extractVersion extracts the version from inline metadata like:
// > **Version**: 1.0.0
func extractVersion(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if idx := strings.Index(trimmed, "**Version**"); idx >= 0 {
			rest := trimmed[idx+len("**Version**"):]
			rest = strings.Trim(rest, ": |")
			// Split on | or space to get just the version
			parts := strings.Fields(rest)
			if len(parts) > 0 {
				return strings.TrimSpace(parts[0])
			}
		}
	}
	return ""
}

// extractStatus extracts the status from inline metadata.
func extractStatus(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if idx := strings.Index(trimmed, "**Status**"); idx >= 0 {
			rest := trimmed[idx+len("**Status**"):]
			rest = strings.Trim(rest, ": |")
			parts := strings.Fields(rest)
			if len(parts) > 0 {
				return strings.TrimSpace(parts[0])
			}
		}
	}
	return "active"
}

// extractCategory extracts the category from inline metadata.
func extractCategory(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if idx := strings.Index(trimmed, "**Category**"); idx >= 0 {
			rest := trimmed[idx+len("**Category**"):]
			rest = strings.Trim(rest, ": |")
			parts := strings.Fields(rest)
			if len(parts) > 0 {
				return strings.TrimSpace(parts[0])
			}
		}
	}
	return ""
}

// extractPurpose extracts the PURPOSE section from agent SKILL.md files.
func extractPurpose(content string) string {
	lines := strings.Split(content, "\n")
	inPurpose := false
	var parts []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## PURPOSE") {
			inPurpose = true
			continue
		}
		if inPurpose {
			if strings.HasPrefix(trimmed, "## ") {
				break
			}
			if trimmed != "" {
				parts = append(parts, trimmed)
			}
		}
	}
	return strings.Join(parts, " ")
}

// extractRole extracts the role from an agent file (e.g., "AI Chief").
func extractRole(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		// Look for the title like "# AI CHIEF"
		if strings.HasPrefix(trimmed, "# ") {
			role := strings.TrimPrefix(trimmed, "# ")
			return strings.TrimSpace(role)
		}
	}
	return ""
}

// extractReportsTo extracts the Reports To field from agent files.
func extractReportsTo(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "**Reports To**") || strings.Contains(trimmed, "- **Reports To**:") {
			idx := strings.Index(trimmed, "**Reports To**")
			rest := trimmed[idx+len("**Reports To**"):]
			rest = strings.Trim(rest, ": |")
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// extractTemplateType extracts the template type from TEMPLATE: line.
func extractTemplateType(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# TEMPLATE:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# TEMPLATE:"))
		}
	}
	return ""
}

// extractInstructions extracts the full body of a skill file as instructions.
func extractInstructions(content string) string {
	return content
}

// ---------------------------------------------------------------------------
// Structured content parsers
// ---------------------------------------------------------------------------

// parseToolsFromContent extracts tools referenced in a skill file's Inputs table.
// Table format: | Name/Input | Type/Required | Description | ...
// The first column is always the tool/input name.
func parseToolsFromContent(content string) []skills.Tool {
	var tools []skills.Tool
	seen := make(map[string]bool)

	lines := strings.Split(content, "\n")
	inInputs := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## Inputs") || strings.HasPrefix(trimmed, "## INPUTS") {
			inInputs = true
			continue
		}
		if inInputs {
			if strings.HasPrefix(trimmed, "## ") {
				break
			}
			// Parse table rows: | name | required | description |
			if strings.HasPrefix(trimmed, "|") {
				parts := splitPipe(trimmed)
				if len(parts) < 2 {
					continue
				}
				name := strings.TrimSpace(parts[0])
				desc := ""
				if len(parts) >= 4 {
					desc = strings.TrimSpace(parts[3])
				} else if len(parts) >= 3 {
					desc = strings.TrimSpace(parts[2])
				}
				if name != "" && !isHeaderRow(name, parts) && !isSeparatorRow(parts) && !seen[name] {
					tools = append(tools, skills.Tool{
						Name:        name,
						Description: desc,
					})
					seen[name] = true
				}
			}
		}
	}
	return tools
}

// parseWorkflowSteps extracts workflow steps from a workflow file.
func parseWorkflowSteps(content string) []workflows.Step {
	var steps []workflows.Step

	lines := strings.Split(content, "\n")
	inSteps := false
	ended := false
	var currentStep *workflows.Step
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Start of steps section
		if strings.HasPrefix(trimmed, "## STEPS") {
			inSteps = true
			continue
		}

		if !inSteps {
			continue
		}

		// End of steps section — encountered another ## heading
		if strings.HasPrefix(trimmed, "## ") && !strings.HasPrefix(trimmed, "###") {
			if currentStep != nil {
				steps = append(steps, *currentStep)
			}
			ended = true
			break
		}

		// Step heading: "### Step N: Name"
		if strings.HasPrefix(trimmed, "### Step ") {
			if currentStep != nil {
				steps = append(steps, *currentStep)
			}
			currentStep = &workflows.Step{
				Name: strings.TrimPrefix(trimmed, "### Step "),
			}
			continue
		}

		// Parse step metadata
		if currentStep != nil {
			if strings.Contains(trimmed, "- **Chief**:") {
				// Format: "- **Chief**: Provider Chief" — split on "**:"
				// (the closing bold marker) to isolate the value.
				parts := strings.SplitN(trimmed, "**:", 2)
				if len(parts) >= 2 {
					currentStep.Agent = strings.TrimSpace(parts[1])
				}
			}
			if strings.Contains(trimmed, "**Description**:") {
				parts := strings.SplitN(trimmed, "**:", 2)
				if len(parts) >= 2 {
					currentStep.Description = strings.TrimSpace(parts[1])
				}
			}
		}
	}

	// Only append the trailing step when the section was not terminated by a
	// "## " heading (which already appended it) — otherwise the last step
	// would be duplicated.
	if currentStep != nil && !ended {
		steps = append(steps, *currentStep)
	}

	return steps
}

// parseWorkflowIO extracts inputs or outputs tables from a workflow file.
// Handles both 3-column and 4-column table formats.
// Standard format: | Name | Type | (Required) | Description |
// The first column (index 0) is always the name.
func parseWorkflowIO(content string, section string) []workflows.IO {
	var ios []workflows.IO

	lines := strings.Split(content, "\n")
	inSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## "+section) {
			inSection = true
			continue
		}
		if inSection {
			if strings.HasPrefix(trimmed, "## ") {
				break
			}
			if strings.HasPrefix(trimmed, "|") {
				parts := splitPipe(trimmed)

				// Need at least 2 columns: Name, Type
				if len(parts) < 2 {
					continue
				}

				name := strings.TrimSpace(parts[0])
				typ := strings.TrimSpace(parts[1])

				// Skip header rows and separator rows
				if isHeaderRow(name, parts) || isSeparatorRow(parts) {
					continue
				}

				io := workflows.IO{
					Name: name,
					Type: typ,
				}

				// 4-column format: Name, Type, Required, Description.
				// The required flag lives in column index 2 (parts[2]);
				// parts[3] is the description.
				if len(parts) >= 4 {
					required := strings.TrimSpace(parts[2])
					io.Required = strings.EqualFold(required, "yes") || strings.EqualFold(required, "true")
				}

				ios = append(ios, io)
			}
		}
	}

	return ios
}

// parseCapabilities extracts capabilities from an agent SKILL.md file.
func parseCapabilities(content string) []agents.Capability {
	var caps []agents.Capability

	// Look for bullet points in the PURPOSE or description section
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") && !strings.HasPrefix(trimmed, "--") {
			text := strings.TrimPrefix(trimmed, "- ")
			// Skip metadata lines
			if strings.Contains(text, "**") || strings.Contains(text, ":") && !strings.HasPrefix(text, "**") {
				continue
			}
			caps = append(caps, agents.Capability{
				Name:        text,
				Description: text,
			})
		}
	}

	return caps
}

// parseAgentTools extracts agent tools from an agent file.
func parseAgentTools(content string) []agents.Tool {
	var tools []agents.Tool
	seen := make(map[string]bool)

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Look for delegation lines like "- Backend API endpoints → Backend Chief"
		if strings.Contains(trimmed, "→") || strings.Contains(trimmed, "->") {
			parts := strings.Split(trimmed, "→")
			if len(parts) < 2 {
				parts = strings.Split(trimmed, "->")
			}
			if len(parts) >= 2 {
				name := strings.TrimSpace(strings.TrimLeft(parts[0], "- "))
				category := "delegation"
				purpose := strings.TrimSpace(parts[1])
				if !seen[name] {
					tools = append(tools, agents.Tool{
						Name:     name,
						Category: category,
						Purpose:  purpose,
					})
					seen[name] = true
				}
			}
		}
	}

	return tools
}

// parseResponsibilities extracts responsibility items from an agent file.
func parseResponsibilities(content string) []string {
	var responsibilities []string

	lines := strings.Split(content, "\n")
	inResp := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## RESPONSIBILITIES") || strings.HasPrefix(trimmed, "## RESPONSIBILITY") {
			inResp = true
			continue
		}
		if inResp {
			if strings.HasPrefix(trimmed, "## ") {
				break
			}
			if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "1.") || strings.HasPrefix(trimmed, "2.") || strings.HasPrefix(trimmed, "3.") || strings.HasPrefix(trimmed, "4.") || strings.HasPrefix(trimmed, "5.") || strings.HasPrefix(trimmed, "6.") || strings.HasPrefix(trimmed, "7.") || strings.HasPrefix(trimmed, "8.") || strings.HasPrefix(trimmed, "9.") || strings.HasPrefix(trimmed, "10.") {
				text := strings.TrimSpace(strings.TrimLeft(trimmed, "- 1234567890."))
				if text != "" {
					responsibilities = append(responsibilities, text)
				}
			}
		}
	}

	return responsibilities
}

// parsePromptParameters extracts parameters from a prompt file.
func parsePromptParameters(content string) []prompts.Parameter {
	var params []prompts.Parameter

	lines := strings.Split(content, "\n")
	inInputs := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## Inputs") || strings.HasPrefix(trimmed, "## INPUTS") || strings.HasPrefix(trimmed, "## Parameters") {
			inInputs = true
			continue
		}
		if inInputs {
			if strings.HasPrefix(trimmed, "## ") {
				break
			}
			if strings.HasPrefix(trimmed, "|") {
				parts := splitPipe(trimmed)
				// Standard table: | Name | Type | Default | Description |
				// The first column (index 0) is always the name, consistent
				// with parseToolsFromContent and parseWorkflowIO. A previous
				// implementation used index 1, which captured header cells
				// ("Type", "Default") as parameters.
				if len(parts) >= 2 && !isSeparatorRow(parts) {
					name := strings.TrimSpace(parts[0])
					typ := strings.TrimSpace(parts[1])
					if name != "" && !isHeaderRow(name, parts) {
						p := prompts.Parameter{
							Name: name,
							Type: typ,
						}
						if len(parts) >= 4 {
							p.Default = strings.TrimSpace(parts[3])
						} else if len(parts) >= 3 {
							p.Default = strings.TrimSpace(parts[2])
						}
						params = append(params, p)
					}
				}
			}
		}
	}

	return params
}

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

// firstNonEmpty returns the first non-empty string from the list.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// nameFromPath extracts the file name (without extension) from a path.
func nameFromPath(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

// inferCategoryFromPath infers a category from the parent directory name.
func inferCategoryFromPath(path string) string {
	dir := filepath.Dir(path)
	parent := filepath.Base(dir)
	if parent != "." && parent != "/" && parent != "skills" && parent != "templates" && parent != "workflows" && parent != "prompts" && parent != "departments" {
		return parent
	}
	return "general"
}

// isHeaderRow checks if a table row is a header text row.
func isHeaderRow(firstCol string, _ []string) bool {
	lower := strings.ToLower(firstCol)
	return lower == "name" || lower == "input" || lower == "output"
}

// isSeparatorRow checks if a table row is a separator (e.g. |---|---|).
// A separator row consists only of dashes (possibly with colons for alignment).
func isSeparatorRow(parts []string) bool {
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" && !isAllDashes(p) {
			return false
		}
	}
	return len(parts) > 0
}

// isAllDashes checks if a string consists only of dashes and optional colons.
func isAllDashes(s string) bool {
	for _, r := range s {
		if r != '-' && r != ':' {
			return false
		}
	}
	return len(s) > 0
}

// splitPipe splits a pipe-delimited table row, handling leading/trailing pipes.
func splitPipe(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")

	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
