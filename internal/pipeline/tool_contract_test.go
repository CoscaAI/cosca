package pipeline

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// readFileTool is a concrete ToolContract for testing schema generation.
type readFileTool struct {
	*BaseTool[ReadFileInput, ReadFileOutput]
}

type ReadFileInput struct {
	Path string `json:"path" validate:"required"`
	Max  int    `json:"max,omitempty"`
}

type ReadFileOutput struct {
	Content string `json:"content"`
}

func newReadFileTool() *readFileTool {
	return &readFileTool{BaseTool: NewBaseTool[ReadFileInput, ReadFileOutput]("read_file", "Reads a file")}
}

func (t *readFileTool) Execute(ctx context.Context, in ReadFileInput) (ReadFileOutput, error) {
	return ReadFileOutput{Content: "content of " + in.Path}, nil
}

func TestBaseToolNameDescription(t *testing.T) {
	tool := newReadFileTool()
	if tool.Name() != "read_file" || tool.Description() != "Reads a file" {
		t.Fatalf("name/desc: %q / %q", tool.Name(), tool.Description())
	}
}

func TestToolSchemaGeneration(t *testing.T) {
	tool := newReadFileTool()
	schema := tool.Schema()

	if schema["type"] != "object" {
		t.Fatalf("schema type = %v", schema["type"])
	}
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("no properties: %+v", schema)
	}
	if _, ok := props["path"]; !ok {
		t.Fatal("missing path property")
	}
	if _, ok := props["max"]; !ok {
		t.Fatal("missing max property")
	}
	// required from validate tag.
	req, ok := schema["required"].([]string)
	if !ok || len(req) != 1 || req[0] != "path" {
		t.Fatalf("required = %v", schema["required"])
	}
	// Non-struct type → primitive schema.
	s := generateSchema(reflect.TypeOf(int(0)))
	if s["type"] != "number" {
		t.Fatalf("int schema = %v", s)
	}
}

func TestToolValidate(t *testing.T) {
	tool := newReadFileTool()

	// Missing required field → error.
	if err := tool.Validate(ReadFileInput{}); err == nil || !strings.Contains(err.Error(), `"path"`) {
		t.Fatalf("validate empty: %v", err)
	}

	// Populated required field → ok.
	if err := tool.Validate(ReadFileInput{Path: "/tmp/x"}); err != nil {
		t.Fatalf("validate filled: %v", err)
	}

	// Pointer-typed nil input → error.
	type ptrInput struct {
		Path string `json:"path" validate:"required"`
	}
	ptrTool := NewBaseTool[*ptrInput, ReadFileOutput]("pt", "d")
	if err := ptrTool.Validate(nil); err == nil || !strings.Contains(err.Error(), "nil") {
		t.Fatalf("validate nil ptr: %v", err)
	}

	// Non-struct input → no validation.
	base2 := NewBaseTool[string, string]("t", "d")
	if err := base2.Validate("just a string"); err != nil {
		t.Fatalf("validate non-struct: %v", err)
	}
}

func TestSchemaToJSON(t *testing.T) {
	tool := newReadFileTool()
	out := SchemaToJSON(tool.Schema())
	if !strings.Contains(out, `"path"`) || !strings.Contains(out, `"required"`) {
		t.Fatalf("json schema: %s", out)
	}
}

func TestToolRegistry(t *testing.T) {
	reg := NewToolRegistry()
	if reg.List() == nil {
		t.Fatal("List must return empty (non-nil) slice")
	}
	reg.Register(newReadFileTool())
	names := reg.List()
	if len(names) != 1 || names[0] != "read_file" {
		t.Fatalf("names = %v", names)
	}
	// Registering a non-tool is ignored.
	reg.Register(42)
	if len(reg.List()) != 1 {
		t.Fatalf("non-tool registered: %v", reg.List())
	}
}

func TestGoKindToJSONType(t *testing.T) {
	if goKindToJSONType(reflect.String) != "string" {
		t.Fatal("string kind")
	}
	if goKindToJSONType(reflect.Bool) != "boolean" {
		t.Fatal("bool kind")
	}
	if goKindToJSONType(reflect.Int) != "number" {
		t.Fatal("int kind")
	}
	if goKindToJSONType(reflect.Slice) != "array" {
		t.Fatal("slice kind")
	}
	if goKindToJSONType(reflect.Map) != "object" {
		t.Fatal("map kind")
	}
}
