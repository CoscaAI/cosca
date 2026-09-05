package integrity

import (
	"strings"
	"testing"
)

func TestHashAlgoValid(t *testing.T) {
	if !HashAlgo("blake3").Valid() || !HashAlgo("sha256").Valid() {
		t.Fatal("supported algos must be valid")
	}
	if HashAlgo("md5").Valid() || HashAlgo("").Valid() {
		t.Fatal("unsupported algos must be invalid")
	}
}

func TestHashBytes(t *testing.T) {
	data := []byte("hello")
	b3 := HashBytes("blake3", data)
	sha := HashBytes("sha256", data)
	if b3 == sha || len(b3) != 64 || len(sha) != 64 {
		t.Fatalf("hashes: blake3=%q sha256=%q", b3, sha)
	}
	// Determinístico.
	if HashBytes("blake3", data) != b3 {
		t.Fatal("hash must be deterministic")
	}
}

func TestRemoveJSONSpaces(t *testing.T) {
	cases := []struct{ in, want string }{
		{`{"a": 1, "b": 2}`, `{"a":1,"b":2}`},
		{`{"a":1}`, `{"a":1}`},
		// Espaços dentro de STRINGS são preservados.
		{`{"msg": "hello world", "n": 3}`, `{"msg":"hello world","n":3}`},
		// String com dois-pontos e vírgula dentro.
		{`{"url": "http://x:8080/a,b", "n": 1}`, `{"url":"http://x:8080/a,b","n":1}`},
		// Vazio.
		{"", ""},
	}
	for _, tc := range cases {
		if got := removeJSONSpaces(tc.in); got != tc.want {
			t.Errorf("removeJSONSpaces(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRemoveJSONSpacesKeyRequirement(t *testing.T) {
	// A normalização deve ser idempotente (aplicar duas vezes = uma vez).
	a := `{ "a" : 1 , "b" : "x y" }`
	once := removeJSONSpaces(a)
	twice := removeJSONSpaces(once)
	if once != twice {
		t.Fatalf("normalization not idempotent: %q vs %q", once, twice)
	}
}

func TestGitAnchorStatus(t *testing.T) {
	// Numa pasta que não é repo git, reporta indisponível.
	s := GitAnchorStatus(t.TempDir())
	if !strings.Contains(s, "unavailable") {
		t.Fatalf("GitAnchorStatus(non-repo) = %q, want 'unavailable'", s)
	}
}
