package integrity

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// runGit runs a git command in root with a fixed identity.
func runGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@cosca.example",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@cosca.example",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// isolateUserHome redirects the OS user-home lookup to a throwaway directory so
// kernelKeyDir (~/.config/cosca/keys) resolves inside the test sandbox and never
// touches a real production key. On Windows os.UserHomeDir() reads USERPROFILE
// (HOME is ignored), so both must be set.
func isolateUserHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	return home
}

// setupGitRepo builds a temp repo with an embed dir and a versioned kernel
// PUBLIC key, committed as a genesis commit. The private key is generated in an
// ephemeral dir OUTSIDE the workspace (M1: the private key must never live
// inside the jail); git-anchor tests only need the public key at the workspace
// + versioned locations so integrity.Check passes.
func setupGitRepo(t *testing.T) string {
	t.Helper()
	// Isolate HOME/USERPROFILE so kernelKeyDir cannot pick up a real production
	// key (e.g. ~/.config/cosca/keys).
	isolateUserHome(t)
	root := t.TempDir()

	// Generate the keypair outside the workspace; only the public key is copied
	// into the repo. The machine-bound private key blob stays in ephemeral dir.
	keyDir := filepath.Join(t.TempDir(), "keys")
	if _, _, err := GenerateKeyPair(keyDir); err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	pubData, err := os.ReadFile(filepath.Join(keyDir, "kernel_public.key"))
	if err != nil {
		t.Fatalf("read public key: %v", err)
	}

	embed := filepath.Join(root, "internal", "embed", "cosca", "memory")
	if err := os.MkdirAll(embed, 0755); err != nil {
		t.Fatalf("mkdir embed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(embed, "test.md"), []byte("hello cosca\n"), 0644); err != nil {
		t.Fatalf("write test.md: %v", err)
	}
	// Version the embedded public key (as in production) — integrity.Check is
	// fail-closed: an absent embedded key is a broken contract, not a skip.
	embKey := filepath.Join(root, "internal", "embed", "cosca", "keys", "kernel_public.key")
	if err := os.MkdirAll(filepath.Dir(embKey), 0755); err != nil {
		t.Fatalf("mkdir embed keys: %v", err)
	}
	if err := os.WriteFile(embKey, pubData, 0644); err != nil {
		t.Fatalf("write embedded public key: %v", err)
	}
	// The ACTIVE copy integrity.Check reads lives in the workspace too.
	if err := os.MkdirAll(filepath.Join(root, ".cosca", "keys"), 0700); err != nil {
		t.Fatalf("mkdir workspace keys: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".cosca", "keys", "kernel_public.key"), pubData, 0600); err != nil {
		t.Fatalf("write workspace public key: %v", err)
	}
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "genesis")
	return root
}

func gitHead(t *testing.T, root string) string {
	t.Helper()
	return runGit(t, root, "rev-parse", "HEAD")
}

func lastBlock(t *testing.T, root string) *Block {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".cosca", "family_chain.dat"))
	if err != nil {
		t.Fatalf("read chain: %v", err)
	}
	blocks, err := parseBlocks(string(data))
	if err != nil {
		t.Fatalf("parse chain: %v", err)
	}
	return blocks[len(blocks)-1]
}

func errorsContain(errs []string, needle string) bool {
	for _, e := range errs {
		if strings.Contains(e, needle) {
			return true
		}
	}
	return false
}

func TestSignAutoInGitRepo(t *testing.T) {
	root := setupGitRepo(t)
	head := gitHead(t, root)

	res, err := SignAuto(root)
	if err != nil {
		t.Fatalf("SignAuto: %v", err)
	}
	if !res.Anchored {
		t.Error("expected Anchored=true")
	}
	if res.BlockNumber != 1 {
		t.Errorf("expected block number 1, got %d", res.BlockNumber)
	}

	b := lastBlock(t, root)
	if !b.GitAnchored {
		t.Error("expected GIT_ANCHORED=true")
	}
	if b.GitCommit != head {
		t.Errorf("GIT_COMMIT %q != HEAD %q", b.GitCommit, head)
	}
	if b.GitTree == "" {
		t.Error("expected GIT_TREE to be set")
	}
	if b.Signer != "git-anchor" {
		t.Errorf("expected SIGNER=git-anchor, got %q", b.Signer)
	}
	if b.Signature != "GIT-ANCHORED" {
		t.Errorf("expected SIGNATURE=GIT-ANCHORED, got %q", b.Signature)
	}
	if len(b.Manifest) == 0 {
		t.Error("expected manifest entries")
	}
}

func TestCheckPassesGitAnchoredBlock(t *testing.T) {
	root := setupGitRepo(t)
	if _, err := SignAuto(root); err != nil {
		t.Fatalf("SignAuto: %v", err)
	}

	info, err := Check(root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !info.Valid {
		t.Errorf("expected valid chain, errors: %v", info.Errors)
	}
	if info.Blocks != 1 {
		t.Errorf("expected 1 block, got %d", info.Blocks)
	}
}

func TestCheckTamperedEmbed(t *testing.T) {
	root := setupGitRepo(t)
	if _, err := SignAuto(root); err != nil {
		t.Fatalf("SignAuto: %v", err)
	}

	// Modify a tracked embed file without re-signing.
	p := filepath.Join(root, "internal", "embed", "cosca", "memory", "test.md")
	if err := os.WriteFile(p, []byte("tampered!\n"), 0644); err != nil {
		t.Fatalf("tamper: %v", err)
	}

	info, err := Check(root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if info.Valid {
		t.Fatal("expected chain invalid after tampering")
	}
	if !errorsContain(info.Errors, "TAMPERED") {
		t.Errorf("expected TAMPERED error, got %v", info.Errors)
	}
}

func TestCheckGitCommitMismatch(t *testing.T) {
	root := setupGitRepo(t)
	if _, err := SignAuto(root); err != nil {
		t.Fatalf("SignAuto: %v", err)
	}

	// Move HEAD without re-signing.
	p := filepath.Join(root, "internal", "embed", "cosca", "memory", "test.md")
	if err := os.WriteFile(p, []byte("updated\n"), 0644); err != nil {
		t.Fatalf("update: %v", err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "move head")

	info, err := Check(root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if info.Valid {
		t.Fatal("expected chain invalid after HEAD moved without re-signing")
	}
	if !errorsContain(info.Errors, "GIT COMMIT MISMATCH") {
		t.Errorf("expected GIT COMMIT MISMATCH, got %v", info.Errors)
	}
}

func TestCheckGitCommitRewritten(t *testing.T) {
	root := setupGitRepo(t)
	if _, err := SignAuto(root); err != nil {
		t.Fatalf("SignAuto block1: %v", err)
	}

	// Amend the genesis commit (rewrites C1), then prune the old object.
	p := filepath.Join(root, "internal", "embed", "cosca", "memory", "test.md")
	if err := os.WriteFile(p, []byte("v1a\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--amend", "-m", "rewritten genesis")
	runGit(t, root, "reflog", "expire", "--expire=now", "--all")
	runGit(t, root, "gc", "--prune=now")

	// Sign a new block against the rewritten HEAD so block 1 is now historical.
	if _, err := SignAuto(root); err != nil {
		t.Fatalf("SignAuto block2: %v", err)
	}

	info, err := Check(root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if info.Valid {
		t.Fatal("expected chain invalid after history rewrite")
	}
	if !errorsContain(info.Errors, "GIT COMMIT REWRITTEN/REBASED") {
		t.Errorf("expected GIT COMMIT REWRITTEN/REBASED, got %v", info.Errors)
	}
}

func TestSignAutoOutsideGitRepo(t *testing.T) {
	root := t.TempDir()
	_, err := SignAuto(root)
	if err == nil {
		t.Fatal("expected error for non-git directory")
	}
	if !strings.Contains(err.Error(), "git") {
		t.Errorf("expected git-related error, got %v", err)
	}
}

func TestBackwardCompatEd25519Chain(t *testing.T) {
	root := setupGitRepo(t)
	t.Setenv("COSCA_ROOT", root)

	// setupGitRepo pre-arms a keypair (for the git-anchor tests) — remove it so
	// InitChain establishes a FRESH Ed25519 identity, as on a new deployment
	// (fresh identity = no versioned embedded key, no pre-existing workspace key).
	if err := os.RemoveAll(filepath.Join(root, ".cosca", "keys")); err != nil {
		t.Fatalf("remove pre-armed keys: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "internal", "embed", "cosca", "keys", "kernel_public.key")); err != nil {
		t.Fatalf("remove embedded key: %v", err)
	}

	// Ed25519 genesis chain (machine-bound via DPAPI — no passphrase).
	if _, err := InitChain(root); err != nil {
		t.Fatalf("InitChain: %v", err)
	}
	info, err := Check(root)
	if err != nil {
		t.Fatalf("Check genesis: %v", err)
	}
	if !info.Valid {
		t.Fatalf("Ed25519 genesis chain should be valid: %v", info.Errors)
	}

	// Append a git-anchored block on top — mixed chain must stay valid.
	if _, err := SignAuto(root); err != nil {
		t.Fatalf("SignAuto: %v", err)
	}
	info, err = Check(root)
	if err != nil {
		t.Fatalf("Check mixed: %v", err)
	}
	if !info.Valid {
		t.Fatalf("mixed chain should be valid: %v", info.Errors)
	}
	if info.Blocks != 2 {
		t.Errorf("expected 2 blocks, got %d", info.Blocks)
	}
}

func TestSignAfterLearningNoPassphrase(t *testing.T) {
	root := setupGitRepo(t)

	// No machine-bound key in the isolated HOME key dir → Sign falls back to the
	// keyless git-anchor path (the pre-existing behaviour is preserved).
	res := SignAfterLearning(root)
	if res == nil {
		t.Fatal("expected successful auto re-sign without passphrase")
	}
	if !res.Anchored {
		t.Error("expected git-anchored block from SignAfterLearning")
	}
	b := lastBlock(t, root)
	if !b.GitAnchored {
		t.Error("expected GIT_ANCHORED block")
	}
}

func TestCheckEmbeddedKeyFailClosed(t *testing.T) {
	root := setupGitRepo(t)
	if _, err := SignAuto(root); err != nil {
		t.Fatalf("SignAuto: %v", err)
	}
	embKey := filepath.Join(root, "internal", "embed", "cosca", "keys", "kernel_public.key")

	t.Run("missing", func(t *testing.T) {
		if err := os.Remove(embKey); err != nil {
			t.Fatalf("remove embedded key: %v", err)
		}
		_, err := Check(root)
		if err == nil {
			t.Fatal("expected fail-closed error when embedded key is missing")
		}
		if !strings.Contains(err.Error(), "key embutida") {
			t.Fatalf("expected contract-violation error, got: %v", err)
		}
	})

	t.Run("corrupt", func(t *testing.T) {
		if err := os.WriteFile(embKey, []byte("not-a-pem"), 0644); err != nil {
			t.Fatalf("corrupt embedded key: %v", err)
		}
		_, err := Check(root)
		if err == nil {
			t.Fatal("expected fail-closed error when embedded key is corrupt")
		}
		if !strings.Contains(err.Error(), "key embutida") {
			t.Fatalf("expected contract-violation error, got: %v", err)
		}
	})
}

func TestVerifyGitAnchorHistoricalTreeCheck(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")

	// Commit A: embed dir absent in the tree.
	if err := os.WriteFile(filepath.Join(root, "foo.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatalf("write foo: %v", err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "no embed")
	commitA := gitHead(t, root)

	// Commit B: embed dir present in the tree.
	embed := filepath.Join(root, "internal", "embed", "cosca", "memory")
	if err := os.MkdirAll(embed, 0755); err != nil {
		t.Fatalf("mkdir embed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(embed, "test.md"), []byte("hello\n"), 0644); err != nil {
		t.Fatalf("write test.md: %v", err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "with embed")
	commitB := gitHead(t, root)
	treeB := runGit(t, root, "rev-parse", commitB+":internal/embed/cosca")

	// Happy path: commit exists and embed tree resolves and matches.
	block := &Block{Number: 1, GitAnchored: true, GitCommit: commitB, GitTree: treeB}
	if warnings, err := verifyGitAnchor(root, block, false); err != nil || warnings != nil {
		t.Fatalf("expected valid historical block, err=%v warnings=%v", err, warnings)
	}

	// Fail-closed: commit exists but the embed tree cannot be resolved
	// (path absent at that commit) — must block, not skip.
	blockNoTree := &Block{Number: 1, GitAnchored: true, GitCommit: commitA, GitTree: "deadbeef"}
	_, err := verifyGitAnchor(root, blockNoTree, false)
	if err == nil {
		t.Fatal("expected fail-closed error when embed tree cannot be resolved")
	}
	if !strings.Contains(err.Error(), "GIT TREE VERIFY FAILED") {
		t.Fatalf("expected GIT TREE VERIFY FAILED, got: %v", err)
	}
}
