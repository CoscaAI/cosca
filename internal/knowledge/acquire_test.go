package knowledge

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildAcquireURL(t *testing.T) {
	pkg := &KnowledgePackage{Repository: "prisma/prisma"}
	url := buildAcquireURL(pkg)
	if url != "https://api.github.com/repos/prisma/prisma/readme" {
		t.Fatalf("url = %q", url)
	}
	// Com branch.
	url2 := buildAcquireURLWithBranch(pkg, "main")
	if url2 != "https://api.github.com/repos/prisma/prisma/readme?ref=main" {
		t.Fatalf("url with branch = %q", url2)
	}
	// Repositório vazio/malformado → "".
	if got := buildAcquireURL(&KnowledgePackage{}); got != "" {
		t.Fatalf("empty repo url = %q", got)
	}
	if got := buildAcquireURL(&KnowledgePackage{Repository: "no-slash"}); got != "" {
		t.Fatalf("no-slash url = %q", got)
	}
}

func TestQuoteTags(t *testing.T) {
	got := quoteTags([]string{"go", "framework"})
	if len(got) != 2 || got[0] != `"go"` || got[1] != `"framework"` {
		t.Fatalf("quoteTags = %v", got)
	}
}

func TestDecodeGitHubReadme(t *testing.T) {
	// Válido: base64.
	raw := []byte("# Prisma\nDocs")
	resp := []byte(`{"content":"` + base64.StdEncoding.EncodeToString(raw) + `","encoding":"base64"}`)
	decoded, ok := decodeGitHubReadme(resp)
	if !ok || string(decoded) != string(raw) {
		t.Fatalf("decode = %q, %v", decoded, ok)
	}

	// Base64 com newlines (GitHub costuma quebrar linhas): o newline real
	// dentro do content vem ESCAPADO no JSON ("\n"), que o Unmarshal converte.
	b64 := base64.StdEncoding.EncodeToString(raw)
	resp2 := []byte(`{"content":"` + b64[:10] + `\n` + b64[10:] + `","encoding":"base64"}`)
	if _, ok := decodeGitHubReadme(resp2); !ok {
		t.Fatal("base64 com newline deve decodificar")
	}

	// JSON inválido → false.
	if _, ok := decodeGitHubReadme([]byte("not json")); ok {
		t.Fatal("invalid json must fail")
	}
	// Encoding errado → false.
	if _, ok := decodeGitHubReadme([]byte(`{"content":"aGk=","encoding":"utf8"}`)); ok {
		t.Fatal("non-base64 encoding must fail")
	}
	// Content vazio → false.
	if _, ok := decodeGitHubReadme([]byte(`{"content":"","encoding":"base64"}`)); ok {
		t.Fatal("empty content must fail")
	}
	// Base64 inválido → false.
	if _, ok := decodeGitHubReadme([]byte(`{"content":"!!!","encoding":"base64"}`)); ok {
		t.Fatal("invalid base64 must fail")
	}
}

func TestAcquiredKnowledgeDir(t *testing.T) {
	got := acquiredKnowledgeDir("/base")
	if got != filepath.Join("/base", "knowledge", "acquired") {
		t.Fatalf("dir = %q", got)
	}
}

func TestWriteAcquiredMarkdown(t *testing.T) {
	kbDir := t.TempDir()
	path, hash, err := writeAcquiredMarkdown(kbDir, "prisma", "typescript", "https://x", []byte("# body"))
	if err != nil {
		t.Fatalf("writeAcquiredMarkdown: %v", err)
	}
	if !strings.HasSuffix(path, "docs.md") {
		t.Fatalf("path = %q", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "title:") || !strings.Contains(content, "tags:") || !strings.Contains(content, "# prisma — Official Documentation") {
		t.Fatalf("markdown incompleto: %q", content)
	}
	if len(hash) != 64 {
		t.Fatalf("hash = %q", hash)
	}

	// Body gigante → truncado a 500 KiB.
	big := make([]byte, 600<<10)
	for i := range big {
		big[i] = 'a'
	}
	path2, _, err := writeAcquiredMarkdown(kbDir, "big-pkg", "go", "https://x", big)
	if err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(path2)
	if !strings.Contains(string(data2), "Content truncated") {
		t.Fatal("big body must be truncated with notice")
	}
}
