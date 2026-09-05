// Package parser parses Cosca entity definitions from Markdown files.
// It extracts agents (AGENT_DNA format), skills, prompts, workflows,
// templates, providers, plugins, and ADRs with their metadata and relationships.
package parser

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/CoscaAI/cosca/internal/markdown"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// EntityType enumerates Cosca entity types.
type EntityType string

// EntityAgent represents an agent entity type.
const (
	EntityAgent         EntityType = "agent"
	EntitySkill         EntityType = "skill"
	EntityPrompt        EntityType = "prompt"
	EntityWorkflow      EntityType = "workflow"
	EntityTemplate      EntityType = "template"
	EntityProvider      EntityType = "provider"
	EntityPlugin        EntityType = "plugin"
	EntityADR           EntityType = "adr"
	EntityCapability    EntityType = "capability"
	EntityMemoryRecord  EntityType = "memory_record"
	EntityPattern       EntityType = "pattern"
	EntityPlaybook      EntityType = "playbook"
	EntityRunbook       EntityType = "runbook"
	EntityIncident      EntityType = "incident"
	EntityBenchmark     EntityType = "benchmark"
	EntityReferenceArch EntityType = "reference_architecture"
	EntityConfig        EntityType = "config"
)

// RelationshipType enumerates relationship types between entities.
type RelationshipType string

// RelDependsOn represents a depends-on relationship between entities.
const (
	RelDependsOn  RelationshipType = "depends_on"
	RelExtends    RelationshipType = "extends"
	RelImplements RelationshipType = "implements"
	RelInvokes    RelationshipType = "invokes"
	RelReferences RelationshipType = "references"
	RelDefines    RelationshipType = "defines"
	RelContains   RelationshipType = "contains"
	RelRelatedTo  RelationshipType = "related_to"
	RelImports    RelationshipType = "imports"
	RelReportsTo  RelationshipType = "reports_to"
	RelTriggers   RelationshipType = "triggers"
	RelDocuments  RelationshipType = "documents"
	RelSupersedes RelationshipType = "supersedes"
	RelConsumes   RelationshipType = "consumes"
	RelProduces   RelationshipType = "produces"
)

// Entity represents a parsed Cosca entity.
type Entity struct {
	ID            string                 `json:"id"`
	Type          EntityType             `json:"type"`
	Name          string                 `json:"name"`
	Path          string                 `json:"path"`
	Version       string                 `json:"version,omitempty"`
	Status        string                 `json:"status,omitempty"`
	Owner         string                 `json:"owner,omitempty"`
	Description   string                 `json:"description,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Relationships []Relationship         `json:"relationships,omitempty"`
	RawContent    string                 `json:"-"`
	Frontmatter   map[string]interface{} `json:"-"`
}

// Relationship represents a relationship between two entities.
type Relationship struct {
	SourceID   string                 `json:"source_id"`
	SourceType EntityType             `json:"source_type"`
	TargetID   string                 `json:"target_id"`
	TargetType EntityType             `json:"target_type"`
	Type       RelationshipType       `json:"type"`
	Weight     float64                `json:"weight,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// EntityParser parses Cosca entities from Markdown content.
type EntityParser struct {
	markdownParser *markdown.Parser
}

// NewEntityParser creates a new entity parser.
func NewEntityParser() *EntityParser {
	return &EntityParser{
		markdownParser: markdown.NewParser(),
	}
}

// ParseFile parses a file path and returns all entities found within.
func (ep *EntityParser) ParseFile(path string, content string) ([]Entity, error) {
	doc, err := ep.markdownParser.Parse(path, content)
	if err != nil {
		return nil, fmt.Errorf("parse markdown: %w", err)
	}

	// Detect entity type from path and content
	entityType := ep.detectEntityType(path, doc)

	var entities []Entity

	switch entityType {
	case EntityAgent:
		entity, err := ep.parseAgent(path, content, doc)
		if err != nil {
			return nil, fmt.Errorf("parse agent: %w", err)
		}
		if entity != nil {
			entities = append(entities, *entity)
		}

	case EntitySkill:
		entity, err := ep.parseSkill(path, content, doc)
		if err != nil {
			return nil, fmt.Errorf("parse skill: %w", err)
		}
		if entity != nil {
			entities = append(entities, *entity)
		}

	case EntityWorkflow:
		entity, err := ep.parseWorkflow(path, content, doc)
		if err != nil {
			return nil, fmt.Errorf("parse workflow: %w", err)
		}
		if entity != nil {
			entities = append(entities, *entity)
		}

	case EntityTemplate:
		entity, err := ep.parseTemplate(path, content, doc)
		if err != nil {
			return nil, fmt.Errorf("parse template: %w", err)
		}
		if entity != nil {
			entities = append(entities, *entity)
		}

	case EntityProvider:
		entity, err := ep.parseProvider(path, content, doc)
		if err != nil {
			return nil, fmt.Errorf("parse provider: %w", err)
		}
		if entity != nil {
			entities = append(entities, *entity)
		}

	case EntityPlugin:
		entity, err := ep.parsePlugin(path, content, doc)
		if err != nil {
			return nil, fmt.Errorf("parse plugin: %w", err)
		}
		if entity != nil {
			entities = append(entities, *entity)
		}

	case EntityADR:
		entity, err := ep.parseADR(path, content, doc)
		if err != nil {
			return nil, fmt.Errorf("parse adr: %w", err)
		}
		if entity != nil {
			entities = append(entities, *entity)
		}

	case EntityCapability:
		entity, err := ep.parseCapability(path, content, doc)
		if err != nil {
			return nil, fmt.Errorf("parse capability: %w", err)
		}
		if entity != nil {
			entities = append(entities, *entity)
		}

	default:
		// Generic entity
		entity := ep.parseGeneric(path, content, doc, entityType)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	// Extract relationships from content references
	relationships := ep.extractRelationships(content, entities)
	for i := range entities {
		entities[i].Relationships = append(entities[i].Relationships, relationships...)
	}

	return entities, nil
}

// detectEntityType determines the entity type from file path and content.
func (ep *EntityParser) detectEntityType(path string, doc *markdown.Document) EntityType {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	lowerPath := strings.ToLower(path)
	lowerDir := strings.ToLower(dir)
	lowerBase := strings.ToLower(base)

	// Check frontmatter for explicit type
	if doc != nil {
		if t, ok := doc.Frontmatter.Data["type"]; ok {
			if typeStr, ok := t.(string); ok {
				return EntityType(typeStr)
			}
		}
	}

	// Agent detection: AGENT_DNA format or in departments/
	if strings.Contains(lowerPath, "agent_dna") || strings.Contains(lowerBase, "dna") {
		return EntityAgent
	}
	if strings.Contains(lowerDir, "department") || strings.Contains(lowerDir, "council") || strings.Contains(lowerDir, "chief") {
		// Check if it has AGENT DNA fields
		if doc != nil && len(doc.Headings) >= 23 {
			return EntityAgent
		}
	}

	// Skill detection
	if strings.Contains(lowerDir, "skills") && strings.Contains(lowerBase, "skill") {
		return EntitySkill
	}
	if strings.Contains(lowerPath, "skill.md") {
		return EntitySkill
	}

	// Workflow detection
	if strings.Contains(lowerDir, "workflow") {
		return EntityWorkflow
	}

	// Template detection
	if strings.Contains(lowerDir, "template") {
		return EntityTemplate
	}

	// Provider detection
	if strings.Contains(lowerDir, "provider") {
		return EntityProvider
	}

	// Plugin detection
	if strings.Contains(lowerDir, "plugin") {
		return EntityPlugin
	}

	// ADR detection
	if strings.Contains(lowerPath, "adr") || strings.Contains(lowerPath, "decision") {
		return EntityADR
	}

	// Capability detection
	if strings.Contains(lowerDir, "capabilit") {
		return EntityCapability
	}

	// Pattern detection
	if strings.Contains(lowerDir, "pattern") {
		return EntityPattern
	}

	// Playbook/runbook detection
	if strings.Contains(lowerBase, "playbook") {
		return EntityPlaybook
	}
	if strings.Contains(lowerBase, "runbook") {
		return EntityRunbook
	}

	// Incident detection
	if strings.Contains(lowerDir, "incident") {
		return EntityIncident
	}

	// Benchmark
	if strings.Contains(lowerBase, "benchmark") {
		return EntityBenchmark
	}

	// Config
	if strings.HasSuffix(lowerBase, ".yaml") || strings.HasSuffix(lowerBase, ".json") || strings.HasSuffix(lowerBase, ".toml") {
		return EntityConfig
	}

	return EntitySkill // default for markdown files in the Cosca tree
}

// parseAgent parses an Cosca agent in AGENT_DNA format (23 fields).
func (ep *EntityParser) parseAgent(path string, content string, doc *markdown.Document) (*Entity, error) {
	entity := &Entity{
		ID:       uuid.New().String(),
		Type:     EntityAgent,
		Path:     path,
		Metadata: make(map[string]interface{}),
	}

	// Extract agent name from first heading
	if doc != nil && doc.Title != "" {
		// Title format: "AGENT: agent-name" or "# AGENT NAME"
		title := doc.Title
		title = strings.TrimPrefix(title, "AGENT:")
		title = strings.TrimPrefix(title, "AGENT")
		entity.Name = strings.TrimSpace(title)
	}

	if entity.Name == "" {
		entity.Name = filepath.Base(filepath.Dir(path))
	}

	// Extract metadata from frontmatter
	if doc != nil && len(doc.Frontmatter.Data) > 0 {
		for k, v := range doc.Frontmatter.Data {
			entity.Metadata[k] = v
		}
	}

	// Extract version from heading content
	versionRe := regexp.MustCompile(`\*\*Version\*\*:\s*([\d.]+)`)
	if m := versionRe.FindStringSubmatch(content); len(m) > 1 {
		entity.Version = m[1]
		entity.Metadata["version"] = m[1]
	}

	// Extract status
	statusRe := regexp.MustCompile(`\*\*Status\*\*:\s*(\w+)`)
	if m := statusRe.FindStringSubmatch(content); len(m) > 1 {
		entity.Status = m[1]
		entity.Metadata["status"] = m[1]
	}

	// Extract owner
	ownerRe := regexp.MustCompile(`\*\*Owner\*\*:\s*(.+)`)
	if m := ownerRe.FindStringSubmatch(content); len(m) > 1 {
		entity.Owner = strings.TrimSpace(m[1])
		entity.Metadata["owner"] = entity.Owner
	}

	// Extract role from section
	entity.Description = extractSection(content, "## 1. ROLE")

	// Extract responsibilities
	responsibilities := extractListAfterHeading(content, "## 3. RESPONSIBILITIES")
	if len(responsibilities) > 0 {
		entity.Metadata["responsibilities"] = responsibilities
	}

	// Extract dependencies
	deps := extractTableAfterHeading(content, "## 7. DEPENDENCIES")
	if len(deps) > 0 {
		entity.Metadata["dependencies"] = deps
	}

	// Extract capabilities/tools
	tools := extractTableAfterHeading(content, "## 6. TOOLS")
	if len(tools) > 0 {
		entity.Metadata["tools"] = tools
	}

	// Extract inputs
	inputs := extractTableAfterHeading(content, "## 4. INPUTS")
	if len(inputs) > 0 {
		entity.Metadata["inputs"] = inputs
	}

	// Extract outputs
	outputs := extractTableAfterHeading(content, "## 5. OUTPUTS")
	if len(outputs) > 0 {
		entity.Metadata["outputs"] = outputs
	}

	return entity, nil
}

// parseSkill parses a skill definition (SKILL.md format).
func (ep *EntityParser) parseSkill(path string, content string, doc *markdown.Document) (*Entity, error) {
	entity := &Entity{
		ID:       uuid.New().String(),
		Type:     EntitySkill,
		Path:     path,
		Metadata: make(map[string]interface{}),
	}

	if doc != nil && doc.Title != "" {
		entity.Name = doc.Title
	}

	// Extract metadata from version line
	versionRe := regexp.MustCompile(`\*\*Version\*\*:\s*([\d.]+)`)
	if m := versionRe.FindStringSubmatch(content); len(m) > 1 {
		entity.Version = m[1]
	}
	statusRe := regexp.MustCompile(`\*\*Status\*\*:\s*(\w+)`)
	if m := statusRe.FindStringSubmatch(content); len(m) > 1 {
		entity.Status = m[1]
	}
	ownerRe := regexp.MustCompile(`\*\*Owner\*\*:\s*(.+)`)
	if m := ownerRe.FindStringSubmatch(content); len(m) > 1 {
		entity.Owner = strings.TrimSpace(m[1])
	}

	entity.Description = extractSection(content, "## PURPOSE")

	// Extract category from path
	dir := filepath.Dir(path)
	if dir != "." {
		entity.Metadata["category"] = filepath.Base(dir)
	}

	// Extract scope
	entity.Metadata["scope"] = extractSection(content, "## SCOPE")

	// Extract dependencies
	deps := extractTableAfterHeading(content, "## DEPENDENCIES")
	if len(deps) > 0 {
		entity.Metadata["dependencies"] = deps
	}

	return entity, nil
}

// parseWorkflow parses a workflow definition.
func (ep *EntityParser) parseWorkflow(path string, content string, doc *markdown.Document) (*Entity, error) {
	entity := &Entity{
		ID:       uuid.New().String(),
		Type:     EntityWorkflow,
		Path:     path,
		Metadata: make(map[string]interface{}),
	}

	if doc != nil {
		entity.Name = doc.Title
	}

	versionRe := regexp.MustCompile(`\*\*Version\*\*:\s*([\d.]+)`)
	if m := versionRe.FindStringSubmatch(content); len(m) > 1 {
		entity.Version = m[1]
	}

	// Extract category
	catRe := regexp.MustCompile(`\*\*Category\*\*:\s*(\w+)`)
	if m := catRe.FindStringSubmatch(content); len(m) > 1 {
		entity.Metadata["category"] = m[1]
	}

	// Extract steps
	steps := extractSteps(content)
	if len(steps) > 0 {
		entity.Metadata["steps"] = steps
	}

	entity.Description = extractSection(content, "## OBJECTIVE")

	// Extract dependencies
	deps := extractTableAfterHeading(content, "## DEPENDENCIES")
	if len(deps) > 0 {
		entity.Metadata["dependencies"] = deps
	}

	// Extract inputs
	inputs := extractTableAfterHeading(content, "## INPUTS")
	if len(inputs) > 0 {
		entity.Metadata["inputs"] = inputs
	}

	// Extract outputs
	outputs := extractTableAfterHeading(content, "## OUTPUTS")
	if len(outputs) > 0 {
		entity.Metadata["outputs"] = outputs
	}

	return entity, nil
}

// parseTemplate parses a template definition.
func (ep *EntityParser) parseTemplate(path string, content string, doc *markdown.Document) (*Entity, error) {
	entity := &Entity{
		ID:       uuid.New().String(),
		Type:     EntityTemplate,
		Path:     path,
		Metadata: make(map[string]interface{}),
	}

	if doc != nil {
		entity.Name = doc.Title
	}

	versionRe := regexp.MustCompile(`\*\*Version\*\*:\s*([\d.]+)`)
	if m := versionRe.FindStringSubmatch(content); len(m) > 1 {
		entity.Version = m[1]
	}

	entity.Description = extractSection(content, "## DOMAIN")

	// Extract stack
	stack := extractTableAfterHeading(content, "## RECOMMENDED STACK")
	if len(stack) > 0 {
		entity.Metadata["stack"] = stack
	}

	// Extract features
	features := extractListAfterHeading(content, "## KEY FEATURES")
	if len(features) > 0 {
		entity.Metadata["features"] = features
	}

	return entity, nil
}

// parseProvider parses a provider definition.
func (ep *EntityParser) parseProvider(path string, content string, doc *markdown.Document) (*Entity, error) {
	entity := &Entity{
		ID:       uuid.New().String(),
		Type:     EntityProvider,
		Path:     path,
		Metadata: make(map[string]interface{}),
	}

	if doc != nil {
		entity.Name = doc.Title
	}

	// Try to parse YAML provider config
	var providerConfig map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &providerConfig); err == nil {
		if id, ok := providerConfig["id"]; ok {
			entity.Name = fmt.Sprintf("%v", id)
		}
		if name, ok := providerConfig["name"]; ok {
			entity.Metadata["display_name"] = name
		}
		if models, ok := providerConfig["models"]; ok {
			entity.Metadata["models"] = models
		}
		if caps, ok := providerConfig["capabilities"]; ok {
			entity.Metadata["capabilities"] = caps
		}
	}

	return entity, nil
}

// parsePlugin parses a plugin definition.
func (ep *EntityParser) parsePlugin(path string, content string, doc *markdown.Document) (*Entity, error) {
	entity := &Entity{
		ID:       uuid.New().String(),
		Type:     EntityPlugin,
		Path:     path,
		Metadata: make(map[string]interface{}),
	}

	if doc != nil {
		entity.Name = doc.Title
	}

	// Try to parse JSON plugin manifest
	var manifest map[string]interface{}
	if err := json.Unmarshal([]byte(content), &manifest); err == nil {
		if id, ok := manifest["id"]; ok {
			entity.Name = fmt.Sprintf("%v", id)
		}
		if v, ok := manifest["version"]; ok {
			entity.Version = fmt.Sprintf("%v", v)
		}
		if desc, ok := manifest["description"]; ok {
			entity.Description = fmt.Sprintf("%v", desc)
		}
		if perms, ok := manifest["permissions"]; ok {
			entity.Metadata["permissions"] = perms
		}
		if lifecycle, ok := manifest["lifecycle"]; ok {
			entity.Metadata["lifecycle"] = lifecycle
		}
	}

	return entity, nil
}

// parseADR parses an Architecture Decision Record.
func (ep *EntityParser) parseADR(path string, content string, doc *markdown.Document) (*Entity, error) {
	entity := &Entity{
		ID:       uuid.New().String(),
		Type:     EntityADR,
		Path:     path,
		Metadata: make(map[string]interface{}),
	}

	if doc != nil {
		entity.Name = doc.Title
	}

	// ADR specific fields
	entity.Metadata["status"] = extractSection(content, "## Status")
	entity.Metadata["context"] = extractSection(content, "## Context")
	entity.Metadata["decision"] = extractSection(content, "## Decision")
	entity.Metadata["consequences"] = extractSection(content, "## Consequences")

	// Extract ADR number from path
	re := regexp.MustCompile(`(\d{3,4})`)
	if m := re.FindStringSubmatch(filepath.Base(path)); len(m) > 1 {
		entity.Metadata["adr_number"] = m[1]
	}

	return entity, nil
}

// parseCapability parses a capability definition.
func (ep *EntityParser) parseCapability(path string, content string, doc *markdown.Document) (*Entity, error) {
	entity := &Entity{
		ID:       uuid.New().String(),
		Type:     EntityCapability,
		Path:     path,
		Metadata: make(map[string]interface{}),
	}

	if doc != nil {
		entity.Name = doc.Title
	}

	// Try YAML parsing
	var capData map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &capData); err == nil {
		if id, ok := capData["id"]; ok {
			entity.Metadata["capability_id"] = id
		}
		if cat, ok := capData["category"]; ok {
			entity.Metadata["category"] = cat
		}
		if deps, ok := capData["dependencies"]; ok {
			entity.Metadata["dependencies"] = deps
		}
	}

	return entity, nil
}

// parseGeneric creates a generic entity for unrecognized file types.
func (ep *EntityParser) parseGeneric(path string, content string, doc *markdown.Document, entityType EntityType) *Entity {
	entity := &Entity{
		ID:       uuid.New().String(),
		Type:     entityType,
		Path:     path,
		Metadata: make(map[string]interface{}),
	}

	if doc != nil {
		if doc.Title != "" {
			entity.Name = doc.Title
		} else {
			entity.Name = filepath.Base(path)
		}
		entity.Description = extractFirstParagraph(content)
	}

	// Copy frontmatter to metadata
	if doc != nil && len(doc.Frontmatter.Data) > 0 {
		for k, v := range doc.Frontmatter.Data {
			entity.Metadata[k] = v
		}
	}

	return entity
}

// extractRelationships finds relationships between entities from content.
func (ep *EntityParser) extractRelationships(content string, _ []Entity) []Relationship {
	var rels []Relationship

	// Find cross-references in Markdown: [text](../path)
	re := regexp.MustCompile(`\[([^\]]*)\]\(\.\./([^)]+)\)`)
	matches := re.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		refPath := match[2]
		rel := Relationship{
			Type:     RelReferences,
			Weight:   1.0,
			Metadata: map[string]interface{}{"link_text": match[1], "target_path": refPath},
		}
		rels = append(rels, rel)
	}

	// Find depends_on, extends, implements in frontmatter or content
	depRe := regexp.MustCompile(`(?i)(depends_on|extends|implements|invokes|reports_to)[:\s]+([^\n]+)`)
	depMatches := depRe.FindAllStringSubmatch(content, -1)
	for _, match := range depMatches {
		if len(match) < 3 {
			continue
		}
		relType := RelationshipType(strings.ToLower(match[1]))
		target := strings.TrimSpace(match[2])
		rel := Relationship{
			Type:     relType,
			Weight:   1.0,
			Metadata: map[string]interface{}{"target_name": target},
		}
		rels = append(rels, rel)
	}

	return rels
}

// ── Helper functions ──────────────────────────────────────────────────────

// extractSection extracts content under a Markdown heading.
func extractSection(content, heading string) string {
	// Find the heading in the content
	idx := strings.Index(content, heading)
	if idx < 0 {
		return ""
	}
	// Move past the heading line
	start := idx + len(heading)
	if start >= len(content) {
		return ""
	}
	// Find next ## heading or end of string
	nextIdx := strings.Index(content[start:], "\n## ")
	if nextIdx < 0 {
		return strings.TrimSpace(content[start:])
	}
	return strings.TrimSpace(content[start : start+nextIdx])
}
func extractListAfterHeading(content, heading string) []string {
	section := extractSection(content, heading)
	if section == "" {
		return nil
	}
	var items []string
	orderedListRe := regexp.MustCompile(`^\d+\.\s`)
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			items = append(items, strings.TrimLeft(line, "-* "))
		} else if orderedListRe.MatchString(line) {
			items = append(items, orderedListRe.ReplaceAllString(line, ""))
		}
	}
	return items
}

// extractTableAfterHeading extracts a table under a heading.
func extractTableAfterHeading(content, heading string) []map[string]string {
	section := extractSection(content, heading)
	if section == "" {
		return nil
	}

	lines := strings.Split(section, "\n")
	if len(lines) < 2 {
		return nil
	}

	// Parse header line (first line with |)
	headerLine := ""
	startIdx := 0
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "|") {
			headerLine = strings.TrimSpace(line)
			startIdx = i
			break
		}
	}
	if headerLine == "" {
		return nil
	}

	headers := parseTableRow(headerLine)
	if len(headers) == 0 {
		return nil
	}

	var rows []map[string]string
	for j := startIdx + 2; j < len(lines); j++ {
		line := strings.TrimSpace(lines[j])
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := parseTableRow(line)
		if len(cells) == 0 {
			continue
		}

		row := make(map[string]string)
		for k, h := range headers {
			if k < len(cells) {
				row[h] = cells[k]
			}
		}
		rows = append(rows, row)
	}

	return rows
}

// parseTableRow parses a Markdown table row.
func parseTableRow(line string) []string {
	line = strings.Trim(line, "|")
	parts := strings.Split(line, "|")
	cells := make([]string, 0, len(parts))
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}

// extractSteps extracts numbered workflow steps.
func extractSteps(content string) []map[string]string {
	var steps []map[string]string
	lines := strings.Split(content, "\n")
	var currentStep map[string]string
	stepHeader := regexp.MustCompile(`^### Step \d+: (.+)$`)
	for _, line := range lines {
		m := stepHeader.FindStringSubmatch(line)
		if m != nil {
			if currentStep != nil {
				steps = append(steps, currentStep)
			}
			currentStep = map[string]string{
				"name":    strings.TrimSpace(m[1]),
				"details": "",
			}
		} else if currentStep != nil {
			if currentStep["details"] != "" {
				currentStep["details"] += "\n"
			}
			currentStep["details"] += line
		}
	}
	if currentStep != nil {
		steps = append(steps, currentStep)
	}
	return steps
}
func extractFirstParagraph(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, ">") && !strings.HasPrefix(trimmed, "```") {
			return trimmed
		}
	}
	return ""
}
