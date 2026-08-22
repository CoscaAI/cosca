package discovery

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

// CoscaInfo contains all discovered Cosca installation info.
type CoscaInfo struct {
	InstallPath    string         `json:"install_path" yaml:"install_path"`
	Version        string         `json:"version,omitempty" yaml:"version,omitempty"`
	Capabilities   []string       `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
	Agents         []AgentInfo    `json:"agents,omitempty" yaml:"agents,omitempty"`
	Skills         []SkillInfo    `json:"skills,omitempty" yaml:"skills,omitempty"`
	Workflows      []WorkflowInfo `json:"workflows,omitempty" yaml:"workflows,omitempty"`
	Templates      []TemplateInfo `json:"templates,omitempty" yaml:"templates,omitempty"`
	HasConfig      bool           `json:"has_config" yaml:"has_config"`
	ConfigPath     string         `json:"config_path,omitempty" yaml:"config_path,omitempty"`
	IsProjectCosca bool           `json:"is_project_cosca" yaml:"is_project_cosca"`
}

// AgentInfo describes an Cosca agent.
type AgentInfo struct {
	Name        string `json:"name" yaml:"name"`
	Type        string `json:"type" yaml:"type"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Path        string `json:"path" yaml:"path"`
}

// SkillInfo describes an Cosca skill.
type SkillInfo struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Path        string `json:"path" yaml:"path"`
}

// WorkflowInfo describes an Cosca workflow.
type WorkflowInfo struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Path        string `json:"path" yaml:"path"`
	Steps       int    `json:"steps,omitempty" yaml:"steps,omitempty"`
}

// TemplateInfo describes an Cosca template.
type TemplateInfo struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Path        string `json:"path" yaml:"path"`
}

// CoscaConfig represents the cosca.config.yaml structure.
type CoscaConfig struct {
	Version      string                 `yaml:"version"`
	Capabilities []string               `yaml:"capabilities"`
	Agents       []AgentConfig          `yaml:"agents"`
	Skills       []SkillConfig          `yaml:"skills"`
	Workflows    []WorkflowConfig       `yaml:"workflows"`
	Settings     map[string]interface{} `yaml:"settings"`
}

// AgentConfig configures an Cosca agent.
type AgentConfig struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
	Enabled     bool   `yaml:"enabled"`
}

// SkillConfig configures an Cosca skill.
type SkillConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Enabled     bool   `yaml:"enabled"`
}

// WorkflowConfig configures an Cosca workflow.
type WorkflowConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Enabled     bool   `yaml:"enabled"`
}

// DetectCosca finds the Cosca global installation and project configuration.
func DetectCosca(_ context.Context, logger zerolog.Logger) (*CoscaInfo, error) {
	info := &CoscaInfo{}

	// Look for Cosca global installation
	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Warn().Err(err).Msg("cannot determine home directory")
		return info, nil
	}

	// Check global Cosca path (~/.config/opencode/cosca)
	globalCoscaPath := filepath.Join(homeDir, ".config", "opencode", "cosca")
	if _, err := os.Stat(globalCoscaPath); err == nil {
		info.InstallPath = globalCoscaPath
		info.IsProjectCosca = false
		logger.Debug().Str("path", globalCoscaPath).Msg("found global Cosca installation")
	}

	// Check legacy global path (~/.cosca)
	legacyPath := filepath.Join(homeDir, ".cosca")
	if _, err := os.Stat(legacyPath); err == nil && info.InstallPath == "" {
		info.InstallPath = legacyPath
		info.IsProjectCosca = false
	}

	// If global not found, check project-level .cosca
	if info.InstallPath == "" {
		projectCoscaPath := ".cosca"
		if _, err := os.Stat(projectCoscaPath); err == nil {
			absPath, err := filepath.Abs(projectCoscaPath)
			if err == nil {
				info.InstallPath = absPath
			} else {
				info.InstallPath = projectCoscaPath
			}
			info.IsProjectCosca = true
		}
	}

	if info.InstallPath == "" {
		logger.Debug().Msg("no Cosca installation found")
		return info, nil
	}

	// Read config
	configPaths := []string{
		filepath.Join(info.InstallPath, "cosca.config.yaml"),
		filepath.Join(info.InstallPath, "cosca.config.yml"),
		filepath.Join(info.InstallPath, "config.yaml"),
		filepath.Join(info.InstallPath, "config.yml"),
	}

	for _, cfgPath := range configPaths {
		if data, err := os.ReadFile(cfgPath); err == nil {
			info.HasConfig = true
			info.ConfigPath = cfgPath

			var cfg CoscaConfig
			if err := yaml.Unmarshal(data, &cfg); err == nil {
				info.Version = cfg.Version
				info.Capabilities = cfg.Capabilities

				// Map agents
				for _, a := range cfg.Agents {
					info.Agents = append(info.Agents, AgentInfo{
						Name:        a.Name,
						Type:        a.Type,
						Description: a.Description,
						Path:        filepath.Join(info.InstallPath, "agents", a.Name),
					})
				}

				// Map skills
				for _, s := range cfg.Skills {
					info.Skills = append(info.Skills, SkillInfo{
						Name:        s.Name,
						Description: s.Description,
						Path:        filepath.Join(info.InstallPath, "skills", s.Name),
					})
				}

				// Map workflows
				for _, w := range cfg.Workflows {
					info.Workflows = append(info.Workflows, WorkflowInfo{
						Name:        w.Name,
						Description: w.Description,
						Path:        filepath.Join(info.InstallPath, "workflows", w.Name),
					})
				}
			}
			break
		}
	}

	// If no config, scan directories for agents/skills/workflows/templates
	if !info.HasConfig {
		scanAOSDirectories(info)
	}

	logger.Debug().
		Str("path", info.InstallPath).
		Str("version", info.Version).
		Int("agents", len(info.Agents)).
		Int("skills", len(info.Skills)).
		Int("workflows", len(info.Workflows)).
		Msg("Cosca discovery complete")

	return info, nil
}

// scanAOSDirectories scans the Cosca installation for agents, skills, workflows, and templates.
func scanAOSDirectories(info *CoscaInfo) {
	basePath := info.InstallPath

	// Scan agents directory
	agentsPath := filepath.Join(basePath, "agents")
	if entries, err := os.ReadDir(agentsPath); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml") {
				name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
				info.Agents = append(info.Agents, AgentInfo{
					Name: name,
					Path: filepath.Join(agentsPath, entry.Name()),
				})
			}
		}
	}

	// Scan skills directory
	skillsPath := filepath.Join(basePath, "skills")
	if entries, err := os.ReadDir(skillsPath); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml") || strings.HasSuffix(entry.Name(), ".md") {
				name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
				info.Skills = append(info.Skills, SkillInfo{
					Name: name,
					Path: filepath.Join(skillsPath, entry.Name()),
				})
			}
		}
	}

	// Scan workflows directory
	workflowsPath := filepath.Join(basePath, "workflows")
	if entries, err := os.ReadDir(workflowsPath); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml") {
				name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
				info.Workflows = append(info.Workflows, WorkflowInfo{
					Name: name,
					Path: filepath.Join(workflowsPath, entry.Name()),
				})
			}
		}
	}

	// Scan templates directory
	templatesPath := filepath.Join(basePath, "templates")
	if entries, err := os.ReadDir(templatesPath); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml") || strings.HasSuffix(entry.Name(), ".md") {
				name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
				info.Templates = append(info.Templates, TemplateInfo{
					Name: name,
					Path: filepath.Join(templatesPath, entry.Name()),
				})
			}
		}
	}
}
