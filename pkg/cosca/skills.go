package cosca

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/CoscaAI/cosca/internal/safe"
)

// =============================================================================
// Types
// =============================================================================

// Skill represents a skill that an agent can use to perform specialised tasks.
type Skill struct {
	// Name is the unique skill identifier.
	Name string `json:"name"`
	// Description explains what the skill does.
	Description string `json:"description"`
	// Version is the skill's semantic version.
	Version string `json:"version"`
	// Category is the skill's classification category.
	Category string `json:"category"`
	// Instructions contain the skill's operational instructions.
	Instructions string `json:"instructions,omitempty"`
	// Tools lists the tools provided by this skill.
	Tools []SkillTool `json:"tools,omitempty"`
	// Source is the origin of the skill (file path, URL, etc.).
	Source string `json:"source,omitempty"`
}

// SkillTool represents a tool defined within a skill.
type SkillTool struct {
	// Name is the tool's name.
	Name string `json:"name"`
	// Description explains what the tool does.
	Description string `json:"description"`
}

// InstallSkillRequest is the request body for installing a skill.
type installSkillRequest struct {
	Source string `json:"source"`
}

// =============================================================================
// SkillsSDK
// =============================================================================

// SkillsSDK provides methods for discovering, inspecting, and installing
// Cosca skills. Skills define specialised capabilities that agents can use
// to perform their tasks.
type SkillsSDK struct {
	client *Client
}

// List returns all available skills registered in the runtime.
func (s *SkillsSDK) List() ([]Skill, error) {
	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		"/v1/skills",
		nil,
	)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, s.client.decodeError(resp)
	}

	var skills []Skill
	if err := json.NewDecoder(resp.Body).Decode(&skills); err != nil {
		return nil, fmt.Errorf("failed to decode skills list: %w", err)
	}

	return skills, nil
}

// Search finds skills matching the given query string. The search is
// case-insensitive and matches against name, description, and category
// fields.
func (s *SkillsSDK) Search(query string) ([]Skill, error) {
	if query == "" {
		return nil, fmt.Errorf("search query is required")
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		fmt.Sprintf("/v1/skills/search?q=%s", query),
		nil,
	)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, s.client.decodeError(resp)
	}

	var skills []Skill
	if err := json.NewDecoder(resp.Body).Decode(&skills); err != nil {
		return nil, fmt.Errorf("failed to decode skills search results: %w", err)
	}

	return skills, nil
}

// Get returns a single skill by name (case-insensitive). Returns an error
// if no skill matches the given name.
func (s *SkillsSDK) Get(name string) (*Skill, error) {
	if name == "" {
		return nil, fmt.Errorf("skill name is required")
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		fmt.Sprintf("/v1/skills/%s", name),
		nil,
	)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, s.client.decodeError(resp)
	}

	var skill Skill
	if err := json.NewDecoder(resp.Body).Decode(&skill); err != nil {
		return nil, fmt.Errorf("failed to decode skill: %w", err)
	}

	return &skill, nil
}

// Install installs a skill from a source (file path or URL). The skill
// is parsed, registered, and persisted for future use. Returns the
// installed skill on success.
func (s *SkillsSDK) Install(name string, source string) (*Skill, error) {
	if name == "" {
		return nil, fmt.Errorf("skill name is required")
	}
	if source == "" {
		return nil, fmt.Errorf("source is required for installation")
	}

	body := installSkillRequest{Source: source}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal install request: %w", err)
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodPost,
		fmt.Sprintf("/v1/skills/%s/install", name),
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, s.client.decodeError(resp)
	}

	var skill Skill
	if err := json.NewDecoder(resp.Body).Decode(&skill); err != nil {
		return nil, fmt.Errorf("failed to decode installed skill: %w", err)
	}

	return &skill, nil
}
