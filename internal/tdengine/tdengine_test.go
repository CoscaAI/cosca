package tdengine

import (
	"os"
	"path/filepath"
	"testing"
)

const cubeOBJ = `# Cube
v -1 -1 -1
v 1 -1 -1
v 1 1 -1
v -1 1 -1
v -1 -1 1
v 1 -1 1
v 1 1 1
v -1 1 1
vn 0 0 -1
vn 0 0 1
usemtl default
f 1 2 3 4
f 5 6 7 8
f 1/1 5/1 8/1 4/1
f 2//1 6//1 7//1 3//1
`

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseOBJ(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "cube.obj", cubeOBJ)

	info, err := ParseOBJ(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Format != FormatOBJ {
		t.Fatalf("format = %v, want obj", info.Format)
	}
	if info.Vertices != 8 {
		t.Fatalf("vertices = %d, want 8", info.Vertices)
	}
	if info.Normals != 2 {
		t.Fatalf("normals = %d, want 2", info.Normals)
	}
	if info.Faces != 4 {
		t.Fatalf("faces = %d, want 4", info.Faces)
	}
	if len(info.Materials) != 1 || info.Materials[0] != "default" {
		t.Fatalf("materials = %v", info.Materials)
	}
	if !info.Supported {
		t.Fatal("cube should be supported")
	}
}

func TestParseOBJ_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "empty.obj", "# comentário apenas\n")
	info, err := ParseOBJ(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Vertices != 0 || info.Supported {
		t.Fatalf("empty obj: %+v", info)
	}
	if len(info.Warnings) == 0 {
		t.Fatal("expected warning for empty obj")
	}
}

func TestParseOBJ_MissingFile(t *testing.T) {
	if _, err := ParseOBJ("/nonexistent/x.obj"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestParseGLTF(t *testing.T) {
	dir := t.TempDir()
	content := `{
  "scenes": [{"name": "Cena", "nodes": [0]}],
  "meshes": [{"name": "Cube", "primitives": [{"mode": 4}, {"mode": 4}]}],
  "materials": [{"name": "Mat_A"}, {"name": "Mat_B"}]
}`
	path := writeFile(t, dir, "cube.gltf", content)

	info, err := ParseGLTF(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Format != FormatGLTF {
		t.Fatalf("format = %v", info.Format)
	}
	if info.Faces != 2 {
		t.Fatalf("faces (primitives) = %d, want 2", info.Faces)
	}
	if len(info.Materials) != 2 {
		t.Fatalf("materials = %v", info.Materials)
	}
}

func TestParseGLB_Partial(t *testing.T) {
	dir := t.TempDir()
	// GLB começa com magic 0x46546C67 = "glTF".
	path := writeFile(t, dir, "model.glb", "\x67\x6C\x54\x46")
	info, err := ParseGLTF(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Format != FormatGLB {
		t.Fatalf("format = %v, want glb", info.Format)
	}
	if info.Supported {
		t.Fatal("glb should be partial support")
	}
}

func TestDetectFormat(t *testing.T) {
	cases := map[string]Format{
		"a.obj":  FormatOBJ,
		"b.gltf": FormatGLTF,
		"c.glb":  FormatGLB,
		"d.txt":  FormatUnknown,
	}
	for name, want := range cases {
		if got := DetectFormat(name); got != want {
			t.Fatalf("DetectFormat(%s) = %v, want %v", name, got, want)
		}
	}
}
