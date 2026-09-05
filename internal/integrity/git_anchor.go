package integrity

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// embedRelPath is the git-relative path of the embedded knowledge directory.
const embedRelPath = "internal/embed/cosca"

// SignAuto appends a git-anchored block to the family chain WITHOUT requiring
// the kernel passphrase. The cryptographic proof is the git commit hash: the
// GIT_COMMIT/GIT_TREE fields are embedded in the signed content (covered by the
// manifest hash chain) and the repository's own SHA-1 chaining + remote
// witness substitutes for the Ed25519 signature. The SIGNATURE field is set to
// the literal marker "GIT-ANCHORED".
func SignAuto(coscaRoot string) (*SignResult, error) {
	// Verify we're inside a git repository.
	if _, err := gitCmd(coscaRoot, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("git-anchor: not a git repository (or git unavailable): %w", err)
	}

	// Resolve the anchor commit + embed tree at HEAD.
	head, err := gitHeadCommit(coscaRoot)
	if err != nil {
		return nil, fmt.Errorf("git-anchor: cannot resolve HEAD commit: %w", err)
	}
	tree, err := gitTreeHash(coscaRoot, embedRelPath)
	if err != nil {
		return nil, fmt.Errorf("git-anchor: cannot resolve embed tree at HEAD (is internal/embed/cosca committed?): %w", err)
	}
	author, err := gitAuthor(coscaRoot)
	if err != nil {
		author = "unknown"
	}

	chainPath := filepath.Join(coscaRoot, ".cosca", "family_chain.dat")
	embedDir := filepath.Join(coscaRoot, embedRelPath)

	manifest, err := scanEmbed(coscaRoot, embedDir, HashBLAKE3)
	if err != nil {
		return nil, fmt.Errorf("scan embed: %w", err)
	}

	manifestHash, manifestJSON, err := manifestHashAndJSON(manifest, HashBLAKE3)
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}

	prevHash, blockNumber := chainState(chainPath)

	now := time.Now().UTC()
	timestamp := now.Format("2006-01-02T15:04:05.000000Z")

	// Build the signed content — git fields live INSIDE the signed section
	// so they are covered by the manifest hash chain.
	signedLines := []string{
		fmt.Sprintf("PREV: %s", prevHash),
		fmt.Sprintf("TIME: %s", timestamp),
		fmt.Sprintf("FILES: %d", len(manifest)),
		fmt.Sprintf("HASH_ALGO: %s", HashBLAKE3),
		fmt.Sprintf("MANIFEST_HASH: %s", manifestHash),
		fmt.Sprintf("GIT_COMMIT: %s", head),
		fmt.Sprintf("GIT_TREE: %s", tree),
		fmt.Sprintf("GIT_AUTHOR: %s", author),
		"GIT_ANCHORED: true",
		"---",
		string(manifestJSON),
	}
	signedContent := strings.Join(signedLines, "\n")
	signedContent = strings.TrimRight(signedContent, "\n")

	hashHex := HashBytes(HashBLAKE3, []byte(signedContent))

	blockLines := []string{
		fmt.Sprintf("======= BLOCK %d =======", blockNumber),
		fmt.Sprintf("# Hash: %s", hashHex),
		fmt.Sprintf("# Time: %s", timestamp),
		fmt.Sprintf("# Files: %d", len(manifest)),
		"",
		"SIGNATURE: GIT-ANCHORED",
		"SIGNER: git-anchor",
	}
	blockLines = append(blockLines, signedLines...)
	blockText := strings.Join(blockLines, "\n") + "\n"

	if err := appendChain(chainPath, blockText); err != nil {
		return nil, fmt.Errorf("write chain: %w", err)
	}

	return &SignResult{
		BlockNumber: blockNumber,
		BlockHash:   hashHex,
		FilesSigned: len(manifest),
		PrevHash:    prevHash,
		Anchored:    true,
	}, nil
}

// gitCmd runs a git command in the repository rooted at `root` and returns
// its trimmed combined output. A missing git binary is reported with a clear
// error instead of a raw exec failure.
func gitCmd(root string, args ...string) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("git not found on PATH (git-anchored chain requires git)")
	}
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat")
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if text != "" {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), text)
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return text, nil
}

// gitHeadCommit returns the full hash of the current HEAD commit.
func gitHeadCommit(root string) (string, error) {
	return gitCmd(root, "rev-parse", "HEAD")
}

// gitTreeHash returns the tree hash of `path` at HEAD.
func gitTreeHash(root, path string) (string, error) {
	return gitCmd(root, "rev-parse", "HEAD:"+path)
}

// gitAuthor returns "Name <email>" of the HEAD commit.
func gitAuthor(root string) (string, error) {
	return gitCmd(root, "log", "-1", "--format=%an <%ae>")
}

// gitCommitExists verifies that a commit hash exists in git history.
func gitCommitExists(root, commit string) error {
	_, err := gitCmd(root, "cat-file", "-e", commit)
	return err
}

// gitEmbedStatus returns porcelain status of the embed directory ("" = clean).
func gitEmbedStatus(root string) (string, error) {
	return gitCmd(root, "status", "--porcelain", "--", embedRelPath)
}

// verifyGitAnchor validates a git-anchored block against the repository.
// For the LATEST block it pins HEAD/embed-tree to the anchor (a moved commit
// or rewritten embed tree is a breach). For historical blocks it proves the
// commit still exists (a rewritten/rebased history is a breach). Warnings are
// returned for legitimately-dirty working trees (learning registration dirties
// the embed until it is committed).
func verifyGitAnchor(root string, block *Block, isLatest bool) ([]string, error) {
	var warnings []string

	if !isLatest {
		if err := gitCommitExists(root, block.GitCommit); err != nil {
			return nil, fmt.Errorf("Block %d: GIT COMMIT REWRITTEN/REBASED — %s no longer exists in git history", block.Number, block.GitCommit)
		}
		if block.GitTree != "" {
			treeAt, err := gitCmd(root, "rev-parse", block.GitCommit+":"+embedRelPath)
			if err != nil {
				return nil, fmt.Errorf("Block %d: GIT TREE VERIFY FAILED — cannot resolve embed tree at %s: %v", block.Number, block.GitCommit, err)
			}
			if treeAt != block.GitTree {
				return nil, fmt.Errorf("Block %d: GIT TREE MISMATCH — embed tree at %s was rewritten", block.Number, block.GitCommit)
			}
		}
		return warnings, nil
	}

	head, err := gitHeadCommit(root)
	if err != nil {
		return nil, fmt.Errorf("Block %d: cannot resolve git HEAD: %v", block.Number, err)
	}
	if head != block.GitCommit {
		return nil, fmt.Errorf("Block %d: GIT COMMIT MISMATCH — embed changed without re-signing (anchored to %s, HEAD is %s). Run: cosca-check --sign-auto", block.Number, block.GitCommit, head)
	}

	tree, err := gitTreeHash(root, embedRelPath)
	if err != nil {
		return nil, fmt.Errorf("Block %d: cannot resolve embed tree at HEAD: %v", block.Number, err)
	}
	if tree != block.GitTree {
		return nil, fmt.Errorf("Block %d: GIT TREE MISMATCH — embed tree at HEAD (%s) differs from anchored tree (%s)", block.Number, tree, block.GitTree)
	}

	if status, statusErr := gitEmbedStatus(root); statusErr == nil && status != "" {
		warnings = append(warnings, fmt.Sprintf("Block %d: git-anchored but internal/embed/cosca has uncommitted changes — commit and re-sign, or confirm the manifest covers them", block.Number))
	}
	return warnings, nil
}

// GitAnchorStatus returns a short human-readable summary of the git anchoring
// state of the latest chain block, used by the watchdog.
func GitAnchorStatus(root string) string {
	head, err := gitHeadCommit(root)
	if err != nil {
		return "git-anchor: unavailable (not a git repo or git missing)"
	}
	short := head
	if len(head) > 8 {
		short = head[:8]
	}

	status, _ := gitEmbedStatus(root)
	dirty := status != ""

	chainPath := filepath.Join(root, ".cosca", "family_chain.dat")
	if data, readErr := os.ReadFile(chainPath); readErr == nil {
		if blocks, parseErr := parseBlocks(string(data)); parseErr == nil && len(blocks) > 0 {
			latest := blocks[len(blocks)-1]
			if latest.GitAnchored {
				shortAnchor := latest.GitCommit
				if len(shortAnchor) > 8 {
					shortAnchor = shortAnchor[:8]
				}
				if latest.GitCommit != head {
					return fmt.Sprintf("git-anchor: DIVERGED — latest block anchored to %s, HEAD is %s (re-sign required)", shortAnchor, short)
				}
				if dirty {
					return fmt.Sprintf("git-anchor: %s (embed has uncommitted changes)", short)
				}
				return fmt.Sprintf("git-anchor: %s (embed clean)", short)
			}
			return "git-anchor: latest block is Ed25519-signed (no anchor)"
		}
	}
	if dirty {
		return fmt.Sprintf("git-anchor: %s (embed has uncommitted changes, no chain yet)", short)
	}
	return fmt.Sprintf("git-anchor: %s", short)
}
