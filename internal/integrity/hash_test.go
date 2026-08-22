package integrity

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestHashBytesKnownVectors(t *testing.T) {
	cases := []struct {
		name string
		algo HashAlgo
		in   string
		want string
	}{
		{
			name: "sha256 empty",
			algo: HashSHA256,
			in:   "",
			want: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name: "sha256 abc",
			algo: HashSHA256,
			in:   "abc",
			want: "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		},
		{
			name: "blake3 empty",
			algo: HashBLAKE3,
			in:   "",
			want: "af1349b9f5f9a1a6a0404dea36dcc9499bcb25c9adc112b7cc9a93cae41f3262",
		},
		{
			name: "blake3 abc",
			algo: HashBLAKE3,
			in:   "abc",
			want: "6437b3ac38465133ffb63b75273a8db548c558465d79db03fd359c6cd5bd9d85",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := HashBytes(tc.algo, []byte(tc.in)); got != tc.want {
				t.Errorf("HashBytes(%s, %q) = %s, want %s", tc.algo, tc.in, got, tc.want)
			}
		})
	}
}

func TestHashFileMatchesHashBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.txt")
	if err := os.WriteFile(path, []byte("the quick brown fox\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	for _, algo := range []HashAlgo{HashSHA256, HashBLAKE3} {
		fromFile, err := HashFile(algo, path)
		if err != nil {
			t.Fatalf("HashFile(%s): %v", algo, err)
		}
		if fromFile != HashBytes(algo, data) {
			t.Errorf("HashFile(%s) = %s, want %s", algo, fromFile, HashBytes(algo, data))
		}
	}

	// sha256File routes through the abstraction and stays equivalent.
	legacy, err := sha256File(path)
	if err != nil {
		t.Fatalf("sha256File: %v", err)
	}
	if legacy != HashBytes(HashSHA256, data) {
		t.Errorf("sha256File = %s, want %s", legacy, HashBytes(HashSHA256, data))
	}
}

// appendLegacySHA256GitBlock appends a git-anchored block in the OLD on-disk
// format: sha256 file hashes, sha256 manifest/block hash and NO HASH_ALGO
// field. This simulates a historical pre-upgrade block.
func appendLegacySHA256GitBlock(t *testing.T, root string) {
	t.Helper()
	embedDir := filepath.Join(root, embedRelPath)
	manifest, err := scanEmbed(root, embedDir, HashSHA256)
	if err != nil {
		t.Fatalf("scan embed: %v", err)
	}
	manifestHash, manifestJSON, err := manifestHashAndJSON(manifest, HashSHA256)
	if err != nil {
		t.Fatalf("manifest hash: %v", err)
	}

	head := gitHead(t, root)
	tree, err := gitTreeHash(root, embedRelPath)
	if err != nil {
		t.Fatalf("git tree: %v", err)
	}
	author, _ := gitAuthor(root)

	chainPath := filepath.Join(root, ".cosca", "family_chain.dat")
	prevHash, blockNumber := chainState(chainPath)
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000000Z")

	// NOTE: deliberately no HASH_ALGO line — this is the legacy format.
	signedLines := []string{
		"PREV: " + prevHash,
		"TIME: " + timestamp,
		"FILES: " + strconv.Itoa(len(manifest)),
		"MANIFEST_HASH: " + manifestHash,
		"GIT_COMMIT: " + head,
		"GIT_TREE: " + tree,
		"GIT_AUTHOR: " + author,
		"GIT_ANCHORED: true",
		"---",
		string(manifestJSON),
	}
	signedContent := strings.Join(signedLines, "\n")
	signedContent = strings.TrimRight(signedContent, "\n")
	hashHex := HashBytes(HashSHA256, []byte(signedContent))

	blockLines := []string{
		"======= BLOCK " + strconv.Itoa(blockNumber) + " =======",
		"# Hash: " + hashHex,
		"# Time: " + timestamp,
		"# Files: " + strconv.Itoa(len(manifest)),
		"",
		"SIGNATURE: GIT-ANCHORED",
		"SIGNER: git-anchor",
	}
	blockLines = append(blockLines, signedLines...)
	blockText := strings.Join(blockLines, "\n") + "\n"

	if err := appendChain(chainPath, blockText); err != nil {
		t.Fatalf("append chain: %v", err)
	}
}

func TestNewBlockIsBLAKE3AndVerifies(t *testing.T) {
	root := setupGitRepo(t)

	res, err := SignAuto(root)
	if err != nil {
		t.Fatalf("SignAuto: %v", err)
	}
	if res.BlockNumber != 1 {
		t.Fatalf("expected block 1, got %d", res.BlockNumber)
	}

	b := lastBlock(t, root)
	if b.HashAlgo != HashBLAKE3 {
		t.Errorf("expected HASH_ALGO blake3, got %q", b.HashAlgo)
	}
	if len(b.Hash) != 64 {
		t.Errorf("expected 64-hex-char block hash, got %d chars", len(b.Hash))
	}
	// Every manifest entry must be a blake3 digest (all 32-byte/64-hex).
	for _, e := range b.Manifest {
		if len(e.Hash) != 64 {
			t.Errorf("manifest entry %s has non-blake3 hash %q", e.Path, e.Hash)
		}
	}

	info, err := Check(root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !info.Valid {
		t.Fatalf("blake3 block should verify: %v", info.Errors)
	}
}

func TestLegacySHA256BlockVerifies(t *testing.T) {
	root := setupGitRepo(t)
	appendLegacySHA256GitBlock(t, root)

	b := lastBlock(t, root)
	if b.HashAlgo != HashSHA256 {
		t.Errorf("expected default HASH_ALGO sha256 for legacy block, got %q", b.HashAlgo)
	}
	// The raw chain must not contain the new field for legacy blocks.
	data, err := os.ReadFile(filepath.Join(root, ".cosca", "family_chain.dat"))
	if err != nil {
		t.Fatalf("read chain: %v", err)
	}
	if strings.Contains(string(data), "HASH_ALGO") {
		t.Error("legacy block unexpectedly contains HASH_ALGO field")
	}

	info, err := Check(root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !info.Valid {
		t.Fatalf("legacy sha256 block should verify after upgrade: %v", info.Errors)
	}
}

func TestMixedSHA256AndBLAKE3Chain(t *testing.T) {
	root := setupGitRepo(t)

	// Block 1: legacy sha256 git-anchored block.
	appendLegacySHA256GitBlock(t, root)

	// Block 2: new blake3 git-anchored block over the same embed state.
	if _, err := SignAuto(root); err != nil {
		t.Fatalf("SignAuto: %v", err)
	}

	blocks, err := parseBlocks(readChain(t, root))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].HashAlgo != HashSHA256 {
		t.Errorf("block 1 expected sha256, got %q", blocks[0].HashAlgo)
	}
	if blocks[1].HashAlgo != HashBLAKE3 {
		t.Errorf("block 2 expected blake3, got %q", blocks[1].HashAlgo)
	}

	info, err := Check(root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !info.Valid {
		t.Fatalf("mixed sha256+blake3 chain should verify: %v", info.Errors)
	}
}

func TestTamperedBLAKE3BlockDetected(t *testing.T) {
	root := setupGitRepo(t)
	if _, err := SignAuto(root); err != nil {
		t.Fatalf("SignAuto: %v", err)
	}

	p := filepath.Join(root, "internal", "embed", "cosca", "memory", "test.md")
	if err := os.WriteFile(p, []byte("tampered blake3 content!\n"), 0644); err != nil {
		t.Fatalf("tamper: %v", err)
	}

	info, err := Check(root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if info.Valid {
		t.Fatal("expected chain invalid after tampering blake3 block")
	}
	if !errorsContain(info.Errors, "TAMPERED") {
		t.Errorf("expected TAMPERED error, got %v", info.Errors)
	}
}

func TestParallelVerifyMatchesSequential(t *testing.T) {
	root := t.TempDir()
	contents := map[string]string{
		"a.txt": "aaaa",
		"b.txt": "bbbb",
		"c.txt": "cccc",
		"d.txt": "dddd",
		"e.txt": "eeee",
	}
	for name, content := range contents {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	for _, algo := range []HashAlgo{HashSHA256, HashBLAKE3} {
		manifest := []FileEntry{
			{Path: "a.txt", Hash: HashBytes(algo, []byte("aaaa"))},
			{Path: "b.txt", Hash: HashBytes(algo, []byte("bbbb"))},
			{Path: "c.txt", Hash: HashBytes(algo, []byte("WRONG"))},
			{Path: "d.txt", Hash: HashBytes(algo, []byte("dddd"))},
			{Path: "e.txt", Hash: HashBytes(algo, []byte("eeef"))},
			{Path: "missing.txt", Hash: HashBytes(algo, []byte("gone"))},
		}
		block := &Block{Number: 1, HashAlgo: algo, Manifest: manifest}

		seq := verifyFileHashes(root, block, 1)
		par := verifyFileHashes(root, block, fileHashWorkers)

		if !reflect.DeepEqual(seq, par) {
			t.Errorf("algo %s: parallel != sequential\n seq=%v\n par=%v", algo, seq, par)
		}

		joined := strings.Join(par, "\n")
		if !strings.Contains(joined, "TAMPERED — c.txt") ||
			!strings.Contains(joined, "TAMPERED — e.txt") ||
			!strings.Contains(joined, "FILE MISSING — missing.txt") {
			t.Errorf("algo %s: missing expected errors in: %v", algo, par)
		}
		if strings.Contains(joined, "a.txt") && !strings.Contains(joined, "TAMPERED — a.txt") {
			t.Errorf("algo %s: a.txt should have no error, got %v", algo, par)
		}
	}
}

func readChain(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".cosca", "family_chain.dat"))
	if err != nil {
		t.Fatalf("read chain: %v", err)
	}
	return string(data)
}
