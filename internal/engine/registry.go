// Package engine implements the core Agent Engine — the central loop that
// orchestrates context building, LLM routing, tool execution, subagent spawning,
// session management, and auto-compaction for the Cosca Chat CLI.
package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// ─── Agent Definition ──────────────────────────────────────────────────────────

// AgentDef represents a parsed agent definition from Markdown with YAML frontmatter.
type AgentDef struct {
	// Name is the agent's logical identifier (e.g. "cosca-architecture").
	Name string

	// DisplayName is a human-readable title (e.g. "Architecture Chief").
	DisplayName string

	// Description is a brief summary of the agent's role and responsibilities.
	Description string

	// Capabilities is a list of keyword tags used for routing (e.g. ["architecture", "design"]).
	Capabilities []string

	// SystemPrompt is the full system prompt extracted from the Markdown body.
	SystemPrompt string

	// FilePath is the absolute path to the .md file this definition was loaded from.
	FilePath string

	// Parent is the parent agent name in the Cosca hierarchy (e.g. "cosca-cto").
	Parent string

	// Temperature is the default LLM temperature override for this agent. 0 means no override.
	Temperature float64
}

// agentFrontmatter maps the YAML frontmatter keys in Cosca agent Markdown files.
// Supports both the new spec format (name, display_name) and the legacy format (agent, type).
type agentFrontmatter struct {
	Agent        string   `yaml:"agent"`
	Name         string   `yaml:"name"`
	DisplayName  string   `yaml:"display_name"`
	Description  string   `yaml:"description"`
	Capabilities []string `yaml:"capabilities"`
	Parent       string   `yaml:"parent"`
	Temperature  float64  `yaml:"temperature"`
	Type         string   `yaml:"type"`
	Version      string   `yaml:"version"`
}

// ─── Agent Registry ────────────────────────────────────────────────────────────

// AgentRegistry loads and manages Cosca agents defined in Markdown files with
// YAML frontmatter. Agents may be loaded from multiple directories; the last
// definition for a given name wins (allows project-level overrides).
type AgentRegistry struct {
	agents map[string]*AgentDef
	mu     sync.RWMutex
}

// NewAgentRegistry creates an AgentRegistry and loads agent definitions from each
// of the provided directories. Directories that do not exist or are unreadable are
// silently skipped to support optional agent directories.
func NewAgentRegistry(agentDirs ...string) *AgentRegistry {
	r := &AgentRegistry{
		agents: make(map[string]*AgentDef),
	}
	for _, dir := range agentDirs {
		_ = r.LoadDirectory(dir) // best-effort per directory
	}
	return r
}

// LoadDefault loads agents from the standard Cosca agent directories:
//   - .cosca/framework/agents/
//   - .cosca/agents/
//
// Returns an error only if no agents could be loaded from any location.
func (r *AgentRegistry) LoadDefault() error {
	locations := []string{
		".cosca/framework/agents",
		".cosca/agents",
	}
	var errs []string
	for _, loc := range locations {
		if err := r.LoadDirectory(loc); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", loc, err))
		}
	}
	if len(r.agents) == 0 {
		return fmt.Errorf("no agents loaded from any location: %s", strings.Join(errs, "; "))
	}
	return nil
}

// LoadDirectory scans a directory recursively for *.md files and attempts to parse
// each as an agent definition with YAML frontmatter. Files without valid frontmatter
// containing the required name field are silently skipped.
func (r *AgentRegistry) LoadDirectory(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("cannot access %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("cannot read directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			// Recurse into subdirectories (e.g. cosca-backend/ containing PROMPT.md)
			_ = r.LoadDirectory(path)
		} else if strings.HasSuffix(entry.Name(), ".md") {
			_ = r.parseAgentFile(path)
		}
	}

	return nil
}

// parseAgentFile attempts to parse a single Markdown file as an agent definition.
// The file must start with --- YAML frontmatter --- followed by the system prompt body.
func (r *AgentRegistry) parseAgentFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	content := string(data)

	// Find YAML frontmatter between --- delimiters.
	if !strings.HasPrefix(content, "---") {
		return fmt.Errorf("no frontmatter delimiters in %s", filePath)
	}

	rest := content[3:]
	endIdx := strings.Index(rest, "\n---")
	if endIdx < 0 {
		return fmt.Errorf("unclosed frontmatter in %s", filePath)
	}

	yamlBlock := rest[:endIdx]

	// Body starts after the closing --- + optional newline
	bodyStart := endIdx + 4 // skip \n---
	if bodyStart < len(rest) && rest[bodyStart] == '\n' {
		bodyStart++ // skip the trailing newline after ---
	} else if bodyStart < len(rest) && rest[bodyStart] == '\r' {
		bodyStart += 2
	}
	bodyStart += 3 // account for original offset (content = "---" + rest)

	// Parse YAML frontmatter.
	var fm agentFrontmatter
	if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err != nil {
		return fmt.Errorf("failed to parse frontmatter in %s: %w", filePath, err)
	}

	// Determine agent name (support both "name" and legacy "agent" keys).
	name := fm.Name
	if name == "" {
		name = fm.Agent
	}
	if name == "" {
		return fmt.Errorf("no agent name in frontmatter of %s", filePath)
	}

	body := strings.TrimSpace(content[bodyStart:])

	def := &AgentDef{
		Name:         name,
		DisplayName:  fm.DisplayName,
		Description:  fm.Description,
		Capabilities: fm.Capabilities,
		SystemPrompt: body,
		FilePath:     filePath,
		Parent:       fm.Parent,
		Temperature:  fm.Temperature,
	}

	r.mu.Lock()
	r.agents[name] = def
	r.mu.Unlock()

	return nil
}

// Get returns the agent definition for the given name, or nil if not found.
// Lookup is case-sensitive (agent names use kebab-case, e.g. "cosca-backend").
func (r *AgentRegistry) Get(name string) *AgentDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.agents[name]
}

// FindByCapability returns all agents that declare the given capability keyword.
// Matching is case-insensitive. Returns an empty slice if no agents match.
func (r *AgentRegistry) FindByCapability(capability string) []*AgentDef {
	r.mu.RLock()
	defer r.mu.RUnlock()

	capLower := strings.ToLower(capability)
	var result []*AgentDef
	for _, def := range r.agents {
		for _, c := range def.Capabilities {
			if strings.EqualFold(c, capLower) {
				result = append(result, def)
				break
			}
		}
	}
	return result
}

// List returns a snapshot of all currently loaded agent definitions.
// The order is non-deterministic.
func (r *AgentRegistry) List() []*AgentDef {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*AgentDef, 0, len(r.agents))
	for _, def := range r.agents {
		result = append(result, def)
	}
	return result
}
