package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// ── Typed Tool Contract (OpenHands Pattern #2) ──────────────────────────
//
// Every tool has a typed contract: Input (Action) → Output (Observation).
// The JSON Schema is auto-generated from Go structs — the LLM receives
// the exact contract, preventing "prompt drift" between description and
// implementation.
//
// OpenHands Pattern:
//   Action (Pydantic model) → schema → LLM
//   LLM response → Observation (validated) → tool execution

// ToolContract defines a typed tool with schema generation.
type ToolContract[Input, Output any] interface {
	// Name returns the unique tool identifier.
	Name() string

	// Description explains what the tool does (for LLM context).
	Description() string

	// Schema returns the JSON Schema for the Input type.
	// Auto-generated from the Go struct tags.
	Schema() map[string]interface{}

	// Validate checks the input before execution.
	Validate(input Input) error

	// Execute runs the tool with validated input.
	Execute(ctx context.Context, input Input) (Output, error)
}

// ── Base contract implementation ─────────────────────────────────────────

// BaseTool provides a default implementation of ToolContract.
// Embed this in concrete tools and override Execute.
type BaseTool[Input, Output any] struct {
	name        string
	description string
}

// NewBaseTool creates a BaseTool with name and description.
func NewBaseTool[Input, Output any](name, description string) *BaseTool[Input, Output] {
	return &BaseTool[Input, Output]{name: name, description: description}
}

func (t *BaseTool[Input, Output]) Name() string        { return t.name }
func (t *BaseTool[Input, Output]) Description() string { return t.description }

// Schema generates a JSON Schema from the Input type's Go struct tags.
// Supports: json tags, required fields, enum values.
func (t *BaseTool[Input, Output]) Schema() map[string]interface{} {
	var input Input
	return generateSchema(reflect.TypeOf(input))
}

// Validate performs basic validation on the input.
func (t *BaseTool[Input, Output]) Validate(input Input) error {
	// Basic: ensure required fields are non-zero
	v := reflect.ValueOf(input)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return fmt.Errorf("tool %q: input is nil", t.name)
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil // only validate structs
	}

	tp := v.Type()
	for i := 0; i < tp.NumField(); i++ {
		field := tp.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" {
			continue
		}
		parts := strings.Split(jsonTag, ",")
		name := parts[0]
		if name == "-" {
			continue
		}

		// Check for "required" in validate tag or json tag
		isRequired := false
		for _, part := range parts {
			if part == "required" {
				isRequired = true
				break
			}
		}
		validateTag := field.Tag.Get("validate")
		if strings.Contains(validateTag, "required") {
			isRequired = true
		}

		if isRequired && v.Field(i).IsZero() {
			return fmt.Errorf("tool %q: required field %q is empty", t.name, name)
		}
	}

	return nil
}

// ── Schema Generation ────────────────────────────────────────────────────

func generateSchema(t reflect.Type) map[string]interface{} {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": make(map[string]interface{}),
	}

	if t.Kind() != reflect.Struct {
		schema["type"] = goKindToJSONType(t.Kind())
		return schema
	}

	props := schema["properties"].(map[string]interface{})
	required := []string{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		parts := strings.Split(jsonTag, ",")
		name := parts[0]

		prop := map[string]interface{}{
			"type":        goKindToJSONType(field.Type.Kind()),
			"description": field.Name,
		}

		// Handle nested structs
		if field.Type.Kind() == reflect.Struct {
			prop = generateSchema(field.Type)
		}

		props[name] = prop

		// Check if required
		for _, part := range parts {
			if part == "required" {
				required = append(required, name)
				break
			}
		}
		validateTag := field.Tag.Get("validate")
		if strings.Contains(validateTag, "required") {
			required = append(required, name)
		}

		// Enum values
		if enumTag := field.Tag.Get("enum"); enumTag != "" {
			prop["enum"] = strings.Split(enumTag, ",")
		}
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

func goKindToJSONType(k reflect.Kind) string {
	switch k {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Map, reflect.Struct, reflect.Ptr:
		return "object"
	default:
		return "string"
	}
}

// ── Example built-in tools ───────────────────────────────────────────────

// PlanCreateInput is the contract for creating a plan.
type PlanCreateInput struct {
	Prompt     string `json:"prompt" validate:"required"`
	Agent      string `json:"agent,omitempty"`
	MaxTasks   int    `json:"max_tasks,omitempty"`
	AutoApprove bool  `json:"auto_approve,omitempty"`
}

// PlanCreateOutput is the result of plan creation.
type PlanCreateOutput struct {
	PlanID      string `json:"plan_id"`
	TasksTotal  int    `json:"tasks_total"`
	EstimateMin int    `json:"estimate_min"`
}

// BuildCheckInput is the contract for build verification.
type BuildCheckInput struct {
	Path    string `json:"path" validate:"required"`
	Target  string `json:"target" enum:"build,test,vet"`
	Timeout int    `json:"timeout_sec,omitempty"`
}

// BuildCheckOutput is the result of build verification.
type BuildCheckOutput struct {
	Success  bool   `json:"success"`
	Output   string `json:"output"`
	Duration int64  `json:"duration_ms"`
}

// SchemaToJSON converts a schema map to JSON string for LLM context.
func SchemaToJSON(schema map[string]interface{}) string {
	data, _ := json.MarshalIndent(schema, "", "  ")
	return string(data)
}

// ToolRegistry manages typed tool contracts.
type ToolRegistry struct {
	tools map[string]interface{} // map[name] → ToolContract[any, any]
}

// NewToolRegistry creates an empty tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]interface{})}
}

// Register adds a tool contract to the registry.
func (r *ToolRegistry) Register(tool interface{}) {
	// Extract name via reflection
	v := reflect.ValueOf(tool)
	m := v.MethodByName("Name")
	if !m.IsValid() {
		return
	}
	name := m.Call(nil)[0].String()
	r.tools[name] = tool
}

// List returns all registered tool names.
func (r *ToolRegistry) List() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}
