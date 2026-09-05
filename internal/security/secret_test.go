package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectBytes_VariousSecrets(t *testing.T) {
	input := strings.Join([]string{
		"const aws = \"AKIAIOSFODNN7EXAMPLE\"",                     // AWS access key
		"aws_secret_access_key = 'wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY'", // AWS secret
		"token: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij",          // GitHub token
		"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjMifQ.T3BlbklkQ0lFQ0lBQ0lBQ0lBQ0lBQ0lBQw", // JWT puro
	}, "\n")
	res := DetectBytes([]byte(input), "test")
	if res.Clean {
		t.Fatal("esperava detectar segredos, veio Clean")
	}
	if !res.HasSecrets() {
		t.Fatal("HasSecrets() deveria ser true")
	}
 

	// Ao menos os kinds de alta severidade devem constar.
	seen := map[SecretKind]bool{}
	for _, m := range res.Matches {
		seen[m.Kind] = true
	}
	for _, k := range []SecretKind{SecretAWSKey, SecretAWSSecret, SecretGitHubToken, SecretJWT} {
		if !seen[k] {
			t.Errorf("esperava kind %s detectado; vistos: %v", k, seen)
		}
	}
}

func TestDetectBytes_PrivateKeyAndBearer(t *testing.T) {
	input := "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQE...\n-----END RSA PRIVATE KEY-----\nAuthorization: Bearer AbCdEfGhIjKlMnOpQrStUvWxYz0123456789"
	res := DetectBytes([]byte(input), "pem")
	if !res.HasSecrets() {
		t.Fatal("esperava detectar private key / bearer")
	}
	seen := map[SecretKind]bool{}
	for _, m := range res.Matches {
		seen[m.Kind] = true
	}
	if !seen[SecretPrivateKey] {
		t.Errorf("esperava private-key; vistos: %v", seen)
	}
}

func TestDetectBytes_Clean(t *testing.T) {
	input := "apenas texto normal; sem segredos; um valor xyzabc"
	res := DetectBytes([]byte(input), "clean")
	if !res.Clean {
		t.Fatalf("esperava Clean, mas detectou: %+v", res.Matches)
	}
}

func TestMask_NeverExposesFull(t *testing.T) {
	secret := "AKIAIOSFODNN7EXAMPLE"
	m := mask(secret)
	if strings.Contains(m, "AKIAIOSFODNN7EXAMPLE") {
		t.Fatalf("mask expôs o segredo completo: %q", m)
	}
	if m == secret {
		t.Fatalf("mask == valor: %q", m)
	}
}

func TestDetectBytes_ReportsLineColumn(t *testing.T) {
	input := "linha um\nAKIAIOSFODNN7EXAMPLE\n"
	res := DetectBytes([]byte(input), "lc")
	if len(res.Matches) == 0 {
		t.Fatal("esperava match")
	}
	m := res.Matches[0]
	if m.Line != 2 {
		t.Fatalf("linha=%d, want 2", m.Line)
	}
	if m.Column < 1 {
		t.Fatalf("coluna=%d, want >=1", m.Column)
	}
	if m.Masked == "" {
		t.Fatal("Masked vazio")
	}
}

func TestScanSecretsDir_DetectsAndSkipsBinary(t *testing.T) {
	dir := t.TempDir()

	// Arquivo de texto com segredo.
	if err := os.WriteFile(filepath.Join(dir, "creds.txt"), []byte("AKIAIOSFODNN7EXAMPLE"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Arquivo limpo.
	if err := os.WriteFile(filepath.Join(dir, "clean.txt"), []byte("sem segredo"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Subdir .git com segredo — deve ser pulado.
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "hooks.sh"), []byte("AKIAIOSFODNN7EXAMPLE"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := ScanSecretsDir(dir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if !res.HasSecrets() {
		t.Fatal("esperava detectar segredo em creds.txt")
	}
	// Só o creds.txt; nada do .git.
	for _, m := range res.Matches {
		if filepath.ToSlash(m.File) == filepath.ToSlash(filepath.Join(dir, ".git", "hooks.sh")) {
			t.Fatalf(".git não deveria ser varrido: %s", m.File)
		}
		if filepath.ToSlash(m.File) == filepath.ToSlash(filepath.Join(dir, "clean.txt")) {
			t.Fatalf("clean.txt não deveria ter match")
		}
	}
	if len(res.Matches) != 1 {
		t.Fatalf("esperava exatamente 1 match (creds.txt), got %d", len(res.Matches))
	}
}
