// Package integrity verifies the Cosca family blockchain integrity before
// the engine starts. It checks Ed25519 signatures and file hashes against
// the on-chain manifest. If the chain is broken, startup is blocked.
package integrity

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ChainInfo holds the verification result.
type ChainInfo struct {
	Valid    bool
	Blocks   int
	Files    int
	Errors   []string
	Warnings []string
}

// Block represents one signed block in the chain.
type Block struct {
	Number   int
	Hash     string
	PrevHash string
	Time     string // UTC timestamp from the block
	Files    int
	Manifest []FileEntry

	// HashAlgo is the content-addressing algorithm used for the manifest file
	// hashes, the manifest hash and the block content hash. Blocks without a
	// HASH_ALGO field default to sha256 (legacy format).
	HashAlgo HashAlgo

	// Git anchoring (git-anchored blocks only).
	GitCommit   string // full hash of the HEAD commit containing this embed state
	GitTree     string // tree hash of internal/embed/cosca at that commit
	GitAuthor   string // author of the commit
	GitAnchored bool   // true when SIGNATURE == "GIT-ANCHORED" (no Ed25519 key)
	Signer      string // "cosca-kernel" (Ed25519) or "git-anchor"
	Signature   string // base64 Ed25519 signature, or the literal "GIT-ANCHORED"
}

// FileEntry is one tracked file.
type FileEntry struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
	Size int64  `json:"size"`
}

// Check loads the family chain and public key, then verifies every block
// signature and every file hash. Returns nil error if everything is valid,
// or a descriptive error with the list of problems.
func Check(coscaRoot string) (*ChainInfo, error) {
	info := &ChainInfo{Valid: true}

	chainPath := filepath.Join(coscaRoot, ".cosca", "family_chain.dat")
	pubKeyPath := filepath.Join(coscaRoot, ".cosca", "keys", "kernel_public.key")

	// Load chain FIRST — if chain doesn't exist, skip (no keys needed).
	// Fresh clone = no chain = no integrity check required.
	chainData, err := os.ReadFile(chainPath)
	if err != nil {
		if os.IsNotExist(err) {
			info.Warnings = append(info.Warnings, "family_chain.dat not found — skipping integrity check (first run?)")
			return info, nil
		}
		return nil, fmt.Errorf("integrity: cannot read chain: %w", err)
	}

	// Chain exists → public key is REQUIRED. Missing key with existing chain = broken setup.
	pubKey, err := loadPublicKey(pubKeyPath)
	if err != nil {
		return nil, fmt.Errorf("integrity: chain exists but public key is missing — run cosca-check --init: %w", err)
	}

	// Verify against embedded (versioned) public key.
	// Prevents key replacement attacks: subagent deletes keys + re-inits with own key.
	// Fail-closed: an unreadable/missing embedded key is a broken contract (the
	// key is versioned in git), NOT a reason to skip the anti-hijack comparison.
	embeddedKeyPath := filepath.Join(coscaRoot, "internal", "embed", "cosca", "keys", "kernel_public.key")
	embeddedKey, embErr := loadPublicKey(embeddedKeyPath)
	if embErr != nil {
		return nil, fmt.Errorf("integrity: falha ao carregar key embutida (contrato violado): %w", embErr)
	}
	if !ed25519.PublicKey(pubKey).Equal(embeddedKey) {
		return nil, fmt.Errorf("integrity: PUBLIC KEY MISMATCH — the active key differs from the versioned key in git. Chain may have been hijacked. Restore from git: cp internal/embed/cosca/keys/kernel_public.key .cosca/keys/")
	}

	blocks, err := parseBlocks(string(chainData))
	if err != nil {
		return nil, fmt.Errorf("integrity: cannot parse chain: %w", err)
	}

	info.Blocks = len(blocks)
	if len(blocks) > 0 {
		info.Files = blocks[len(blocks)-1].Files
	}

	// Verify chain: all signatures must be valid.
	// Only the LATEST block's file hashes are checked against current files
	// (historical blocks are audit trail, not current state).
	latestBlock := blocks[len(blocks)-1]

	for _, block := range blocks {
		isLatest := block.Number == latestBlock.Number

		// ALL blocks: verify integrity proof — git anchor or Ed25519 signature
		if block.GitAnchored {
			warnings, gErr := verifyGitAnchor(coscaRoot, block, isLatest)
			if gErr != nil {
				info.Errors = append(info.Errors, gErr.Error())
				info.Valid = false
				continue
			}
			info.Warnings = append(info.Warnings, warnings...)
		} else {
			if err := verifyBlock(block, pubKey); err != nil {
				info.Errors = append(info.Errors,
					fmt.Sprintf("Block %d: INVALID SIGNATURE — %v", block.Number, err))
				info.Valid = false
				continue
			}
		}

		// Only LATEST block: verify file hashes against current disk state
		if !isLatest {
			continue
		}

		info.Errors = append(info.Errors, verifyFileHashes(coscaRoot, block, fileHashWorkers)...)
	}

	if len(info.Errors) > 0 {
		info.Valid = false
	}

	return info, nil
}

// fileHashWorkers is the concurrency limit for the per-file hash verification
// worker pool. Signature verification stays sequential (few blocks, cheap);
// only the (potentially thousands of) file hashes are parallelized.
const fileHashWorkers = 8

// verifyFileHashes checks every file in the block manifest against disk state
// using the block's HashAlgo. The per-file hashing is distributed across a
// worker pool (concurrency <= fileHashWorkers); results are collected and
// sorted by path so the returned error list is deterministic regardless of
// goroutine completion order.
func verifyFileHashes(root string, block *Block, workers int) []string {
	if workers < 1 {
		workers = 1
	}

	type fileErr struct {
		msg string
	}

	jobs := make(chan FileEntry, len(block.Manifest))
	results := make(chan fileErr, len(block.Manifest))

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for entry := range jobs {
				fpath := filepath.Join(root, entry.Path)
				if _, statErr := os.Stat(fpath); os.IsNotExist(statErr) {
					results <- fileErr{fmt.Sprintf("FILE MISSING — %s", entry.Path)}
					continue
				}

				actualHash, err := HashFile(block.HashAlgo, fpath)
				if err != nil {
					results <- fileErr{fmt.Sprintf("CANNOT READ — %s: %v", entry.Path, err)}
					continue
				}

				if actualHash != entry.Hash {
					results <- fileErr{fmt.Sprintf("TAMPERED — %s", entry.Path)}
				}
			}
		}()
	}

	for _, entry := range block.Manifest {
		jobs <- entry
	}
	close(jobs)
	wg.Wait()
	close(results)

	// Deterministic ordering: sort by path (all errors share the same block).
	var errs []fileErr
	for r := range results {
		errs = append(errs, r)
	}
	sort.Slice(errs, func(i, j int) bool { return errs[i].msg < errs[j].msg })

	out := make([]string, 0, len(errs))
	for _, r := range errs {
		out = append(out, fmt.Sprintf("Block %d: %s", block.Number, r.msg))
	}
	return out
}

// loadPublicKey reads an Ed25519 public key from a PEM file.
func loadPublicKey(path string) (ed25519.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM")
	}

	// Try PKIX/SPKI format first (what cryptography library outputs)
	pub, err := parsePKIXPublicKey(block.Bytes)
	if err == nil {
		return pub, nil
	}

	return nil, fmt.Errorf("unsupported key format")
}

// parsePKIXPublicKey extracts the raw Ed25519 key from a SubjectPublicKeyInfo.
func parsePKIXPublicKey(der []byte) (ed25519.PublicKey, error) {
	// Ed25519 OID: 1.3.101.112
	// SPKI format: SEQUENCE { SEQUENCE { OID }, BIT STRING }
	// We look for the raw 32-byte key at the end
	if len(der) < 44 {
		return nil, fmt.Errorf("DER too short")
	}
	// The raw key is the last 32 bytes
	return ed25519.PublicKey(der[len(der)-32:]), nil
}

// parseBlocks extracts blocks from the chain file content.
func parseBlocks(content string) ([]*Block, error) {
	// Split by "======= BLOCK " (handles both header-less and header formats)
	rawParts := strings.Split(content, "======= BLOCK ")
	var partTexts []string
	for _, rp := range rawParts {
		rp = strings.TrimSpace(rp)
		if rp == "" {
			continue
		}
		// Skip parts that are chain headers (start with #)
		if strings.HasPrefix(rp, "#") && !strings.Contains(rp, "\n") {
			continue
		}
		// Remove the "N =======" line at the start (e.g., "1 =======\n" or "2 =======\n\n")
		nl := strings.Index(rp, "\n")
		if nl > 0 && strings.Contains(rp[:nl], "======") {
			rp = rp[nl+1:]
		}
		partTexts = append(partTexts, rp)
	}

	var blocks []*Block
	for i, part := range partTexts {
		block := &Block{Number: i + 1, HashAlgo: HashSHA256}
		lines := strings.Split(part, "\n")

		// Find the separator between header and manifest
		sep := -1
		for j, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "---" {
				sep = j
				break
			}
		}

		if sep < 0 {
			// Try to find the JSON manifest start (starts with [)
			for j, line := range lines {
				if strings.HasPrefix(strings.TrimSpace(line), "[") {
					sep = j
					break
				}
			}
		}

		// Parse header fields
		for j := 0; j < len(lines); j++ {
			line := strings.TrimSpace(lines[j])
			if sep >= 0 && j >= sep && strings.HasPrefix(line, "[") {
				break // reached manifest
			}
			if strings.HasPrefix(line, "# Hash: ") {
				block.Hash = strings.TrimPrefix(line, "# Hash: ")
			}
			if strings.HasPrefix(line, "PREV: ") {
				block.PrevHash = strings.TrimPrefix(line, "PREV: ")
			}
			if strings.HasPrefix(line, "TIME: ") {
				block.Time = strings.TrimPrefix(line, "TIME: ")
			}
			if strings.HasPrefix(line, "FILES: ") {
				fmt.Sscanf(line, "FILES: %d", &block.Files)
			}
			if strings.HasPrefix(line, "HASH_ALGO: ") {
				block.HashAlgo = HashAlgo(strings.TrimSpace(strings.TrimPrefix(line, "HASH_ALGO: ")))
			}
			if strings.HasPrefix(line, "GIT_COMMIT: ") {
				block.GitCommit = strings.TrimPrefix(line, "GIT_COMMIT: ")
			}
			if strings.HasPrefix(line, "GIT_TREE: ") {
				block.GitTree = strings.TrimPrefix(line, "GIT_TREE: ")
			}
			if strings.HasPrefix(line, "GIT_AUTHOR: ") {
				block.GitAuthor = strings.TrimPrefix(line, "GIT_AUTHOR: ")
			}
			if strings.HasPrefix(line, "GIT_ANCHORED: ") {
				block.GitAnchored = strings.TrimSpace(strings.TrimPrefix(line, "GIT_ANCHORED: ")) == "true"
			}
			if strings.HasPrefix(line, "SIGNER: ") {
				block.Signer = strings.TrimPrefix(line, "SIGNER: ")
			}
			if strings.HasPrefix(line, "SIGNATURE: ") {
				block.Signature = strings.TrimPrefix(line, "SIGNATURE: ")
			}
		}

		// Parse manifest JSON
		if sep >= 0 && sep < len(lines) {
			manifestJSON := strings.Join(lines[sep+1:], "\n")
			// Trim trailing content (next block markers)
			if idx := strings.Index(manifestJSON, "\n\n======= BLOCK "); idx > 0 {
				manifestJSON = manifestJSON[:idx]
			}
			manifestJSON = strings.TrimSpace(manifestJSON)
			if manifestJSON != "" {
				if err := json.Unmarshal([]byte(manifestJSON), &block.Manifest); err != nil {
					// Try to recover: find the JSON array
					start := strings.Index(manifestJSON, "[")
					end := strings.LastIndex(manifestJSON, "]")
					if start >= 0 && end > start {
						if err := json.Unmarshal([]byte(manifestJSON[start:end+1]), &block.Manifest); err != nil {
							return nil, fmt.Errorf("block %d: invalid manifest JSON: %w", i, err)
						}
					}
				}
			}
		}

		blocks = append(blocks, block)
	}

	return blocks, nil
}

// verifyBlock checks the Ed25519 signature of a block.
func verifyBlock(block *Block, pubKey ed25519.PublicKey) error {
	// Use the RAW signed content from the chain file.
	// We don't reconstruct — we extract the exact bytes that were signed.
	return verifyBlockFromChain(block, pubKey)
}

// verifyBlockFromChain reads the raw chain, extracts block N, reconstructs
// the exact signed payload, and verifies the Ed25519 signature. The content
// hash uses the block's HashAlgo (BLAKE3 for new blocks, SHA-256 for legacy
// blocks that lack the HASH_ALGO field).
func verifyBlockFromChain(block *Block, pubKey ed25519.PublicKey) error {
	blockNum := block.Number
	chainPath := filepath.Join(".cosca", "family_chain.dat")

	// Try multiple possible roots
	roots := []string{
		".",
		os.Getenv("COSCA_ROOT"),
	}
	if cwd, err := os.Getwd(); err == nil {
		roots = append([]string{cwd}, roots...)
	}

	var data []byte
	var err error
	for _, root := range roots {
		data, err = os.ReadFile(filepath.Join(root, chainPath))
		if err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("cannot read chain for signature verification: %w", err)
	}

	content := string(data)
	// Split by the block marker (handles both header-less and with-header)
	rawParts := strings.Split(content, "======= BLOCK ")
	if blockNum >= len(rawParts) {
		return fmt.Errorf("block %d not found in chain (only %d parts)", blockNum, len(rawParts))
	}

	// rawParts[0] may be header or empty, rawParts[1] = block 1 content, etc.
	blockText := rawParts[blockNum]

	// Remove the "# N =======" line from the block text
	nl := strings.Index(blockText, "\n")
	if nl > 0 && strings.Contains(blockText[:nl], "======") {
		blockText = blockText[nl+1:]
	}

	lines := strings.Split(blockText, "\n")

	// Find SIGNATURE and the start of signed content (PREV: line)
	var sigB64 string
	signedStart := -1
	for j, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "SIGNATURE: ") {
			sigB64 = strings.TrimPrefix(trimmed, "SIGNATURE: ")
		}
		if strings.HasPrefix(trimmed, "PREV: ") {
			signedStart = j
		}
	}

	if sigB64 == "" {
		return fmt.Errorf("no signature found in block %d", blockNum)
	}
	if signedStart < 0 {
		return fmt.Errorf("no PREV line found in block %d", blockNum)
	}

	// Reconstruct the exact bytes that were signed
	// This is: PREV: ...\n...\n---\n{manifest}
	signedContent := strings.Join(lines[signedStart:], "\n")

	// Trim trailing newlines exactly like Python's sha256_str does
	signedContent = strings.TrimRight(signedContent, "\n")

	// Hash the content with the block's algorithm, then sign the HEX STRING
	// of the hash (Python: private_key.sign(sha256_str(content).encode()))
	hashHex := HashBytes(block.HashAlgo, []byte(signedContent))

	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}

	if !ed25519.Verify(pubKey, []byte(hashHex), sig) {
		return fmt.Errorf("signature does not match")
	}

	return nil
}

// removeJSONSpaces converts Go's json.Marshal output (with spaces) to
// Python's compact format (no spaces after : and ,).
func removeJSONSpaces(s string) string {
	// Simple approach: remove spaces after : and ,
	var out strings.Builder
	inString := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '"' && (i == 0 || s[i-1] != '\\') {
			inString = !inString
		}
		if !inString && ch == ' ' && i > 0 && (s[i-1] == ':' || s[i-1] == ',') {
			continue
		}
		out.WriteByte(ch)
	}
	return out.String()
}

// VerifyOrFatal runs the integrity check and calls os.Exit(1) with a clear
// message if the chain is broken. This is the function to call at the very
// beginning of main() or Compose().
func VerifyOrFatal(coscaRoot string) {
	info, err := Check(coscaRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n🔴 COSCA INTEGRITY CHECK FAILED\n")
		fmt.Fprintf(os.Stderr, "   Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "   The engine cannot start.\n\n")
		os.Exit(1)
	}

	if !info.Valid {
		fmt.Fprintf(os.Stderr, "\n╔══════════════════════════════════════════╗\n")
		fmt.Fprintf(os.Stderr, "║  🔴 FAMILY CHAIN BREACH DETECTED          ║\n")
		fmt.Fprintf(os.Stderr, "║  Engine startup blocked for safety        ║\n")
		fmt.Fprintf(os.Stderr, "╚══════════════════════════════════════════╝\n\n")
		for _, e := range info.Errors {
			fmt.Fprintf(os.Stderr, "  ❌ %s\n", e)
		}
		fmt.Fprintf(os.Stderr, "\n  Chain has been tampered with.\n")
		fmt.Fprintf(os.Stderr, "   Only the kernel can re-sign after authorized changes.\n")
		fmt.Fprintf(os.Stderr, "   Run: cosca-check --sign\n\n")
		os.Exit(1)
	}

	// All good
	fmt.Fprintf(os.Stderr, "✅ Family chain integrity verified — %d blocks, %d files\n\n", info.Blocks, info.Files)
}
