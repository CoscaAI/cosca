package integrity

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SignResult holds the result of a signing operation.
type SignResult struct {
	BlockNumber int
	BlockHash   string
	FilesSigned int
	PrevHash    string
	Anchored    bool // true when the block is git-anchored (no Ed25519 signature)
}

// kernelKeyDir resolve o diretório da CHAVE PRIVADA do kernel — SEMPRE fora do
// workspace: ~/.config/cosca/keys. Não existe mais um fallback para
// <coscaRoot>/.cosca/keys (M1): a chave privada jamais pode viver dentro da
// jaula ou do diretório de trabalho, senão um agente preso a leria e forjaria a
// chain. A chave PÚBLICA permanece no workspace (.cosca/keys/kernel_public.key),
// onde integrity.Check a lê.
//
// Retorna "" se o home do usuário não puder ser resolvido — o chamador deve
// tratar esse caso como "identidade indisponível".
func kernelKeyDir(_ string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".config", "cosca", "keys")
}

// Sign scans the embed directory, builds a new manifest, and appends a signed
// block to the family chain. The key is loaded and DECRYPTED machine-bound via
// DPAPI (no passphrase), so only the same user on the same machine can sign.
// Subagents without the machine-bound key cannot sign; a different machine or
// user fails to unprotect the key and is denied.
func Sign(coscaRoot string) (*SignResult, error) {
	keysDir := kernelKeyDir(coscaRoot)
	if keysDir == "" {
		return nil, fmt.Errorf("cannot resolve user config directory for kernel key")
	}
	privKeyPath := filepath.Join(keysDir, "kernel_private.key")
	chainPath := filepath.Join(coscaRoot, ".cosca", "family_chain.dat")
	embedDir := filepath.Join(coscaRoot, "internal", "embed", "cosca")

	// Load private key (machine-bound unprotect; no passphrase)
	privKey, err := loadPrivateKey(privKeyPath)
	if err != nil {
		return nil, fmt.Errorf("cannot load private key (only the kernel on this machine can sign): %w", err)
	}

	// Scan embed directory for all files
	manifest, err := scanEmbed(coscaRoot, embedDir, HashBLAKE3)
	if err != nil {
		return nil, fmt.Errorf("scan embed: %w", err)
	}

	// Compute manifest hash
	manifestHash, manifestJSON, err := manifestHashAndJSON(manifest, HashBLAKE3)
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}

	// Determine previous block hash and next block number
	prevHash, blockNumber := chainState(chainPath)

	// Build the signed content (everything from PREV line onwards)
	now := time.Now().UTC()
	timestamp := now.Format("2006-01-02T15:04:05.000000Z")

	signedLines := []string{
		fmt.Sprintf("PREV: %s", prevHash),
		fmt.Sprintf("TIME: %s", timestamp),
		fmt.Sprintf("FILES: %d", len(manifest)),
		fmt.Sprintf("HASH_ALGO: %s", HashBLAKE3),
		fmt.Sprintf("MANIFEST_HASH: %s", manifestHash),
		"---",
		string(manifestJSON),
	}
	signedContent := strings.Join(signedLines, "\n")
	signedContent = strings.TrimRight(signedContent, "\n")

	// Hash the signed content (BLAKE3), then sign the HEX STRING (matches Python convention)
	hashHex := HashBytes(HashBLAKE3, []byte(signedContent))

	sig := ed25519.Sign(privKey, []byte(hashHex))
	sigB64 := base64.StdEncoding.EncodeToString(sig)

	// Build the complete block
	blockHash := hashHex
	blockLines := []string{
		fmt.Sprintf("======= BLOCK %d =======", blockNumber),
		fmt.Sprintf("# Hash: %s", blockHash),
		fmt.Sprintf("# Time: %s", timestamp),
		fmt.Sprintf("# Files: %d", len(manifest)),
		"",
		fmt.Sprintf("SIGNATURE: %s", sigB64),
		"SIGNER: cosca-kernel",
	}
	blockLines = append(blockLines, signedLines...)
	blockText := strings.Join(blockLines, "\n") + "\n"

	// Append to chain file (or create new one)
	if err := appendChain(chainPath, blockText); err != nil {
		return nil, fmt.Errorf("write chain: %w", err)
	}

	return &SignResult{
		BlockNumber: blockNumber,
		BlockHash:   blockHash,
		FilesSigned: len(manifest),
		PrevHash:    prevHash,
	}, nil
}

// InitChain generates a new keypair and creates the genesis block.
// The new keypair is machine-bound (DPAPI) — no passphrase to remember.
// REFUSES to overwrite an existing chain — subagents cannot hijack the chain
// by deleting keys and re-initializing. If a chain already exists, it must be
// manually removed (requires filesystem access).
func InitChain(coscaRoot string) (*SignResult, error) {
	keysDir := kernelKeyDir(coscaRoot)
	if keysDir == "" {
		return nil, fmt.Errorf("cannot resolve user config directory for kernel key")
	}
	privPath := filepath.Join(keysDir, "kernel_private.key")
	chainPath := filepath.Join(coscaRoot, ".cosca", "family_chain.dat")

	// Refuse to overwrite an existing chain
	if existing, err := os.ReadFile(chainPath); err == nil && len(existing) > 0 {
		if strings.Contains(string(existing), "======= BLOCK ") {
			return nil, fmt.Errorf("chain already exists — will not overwrite. Delete %s manually to re-initialize", chainPath)
		}
	}

	// Refuse to overwrite if a versioned public key exists in git (immutable identity)
	embeddedKeyPath := filepath.Join(coscaRoot, "internal", "embed", "cosca", "keys", "kernel_public.key")
	if _, err := os.Stat(embeddedKeyPath); err == nil {
		// A versioned key exists — this is the canonical identity. Don't allow overwrite.
		// Subagents may have deleted .cosca/keys/ but the git version is immutable.
		return nil, fmt.Errorf("versioned public key exists at %s — chain identity is already established. Delete this file from git to reset identity (requires commit)", embeddedKeyPath)
	}

	// Only generate keys if they don't exist
	if _, err := os.Stat(privPath); os.IsNotExist(err) {
		if _, _, err := GenerateKeyPair(keysDir); err != nil {
			return nil, fmt.Errorf("generate keys: %w", err)
		}
		fmt.Fprintf(os.Stderr, "✅ Ed25519 key pair generated in %s (machine-bound)\n", keysDir)

		// Copy public key to versioned location (git-tracked, immutable identity)
		pubPath := filepath.Join(keysDir, "kernel_public.key")
		pubData, _ := os.ReadFile(pubPath)
		os.MkdirAll(filepath.Dir(embeddedKeyPath), 0755)
		os.WriteFile(embeddedKeyPath, pubData, 0644)
		fmt.Fprintf(os.Stderr, "✅ Public key versioned at %s (commit to git)\n", embeddedKeyPath)

		// Public key ALSO stays in the workspace (.cosca/keys) so
		// integrity.Check can read it; the PRIVATE key never enters the
		// jail workspace.
		wsKeysDir := filepath.Join(coscaRoot, ".cosca", "keys")
		os.MkdirAll(wsKeysDir, 0700)
		os.WriteFile(filepath.Join(wsKeysDir, "kernel_public.key"), pubData, 0600)
	} else {
		fmt.Fprintf(os.Stderr, "ℹ️  Keys already exist — using existing key pair\n")
	}

	// Sign the genesis block
	return Sign(coscaRoot)
}

// SignAfterLearning re-signs the chain after a learning was registered.
// Called automatically by the kernel after every stage 7-8 evolution step.
// Machine-bound (no passphrase): it prefers full Ed25519 signing via the
// DPAPI-bound key (same machine + user). If signing fails (e.g. the key is
// missing for this user/machine, or it is still in the legacy passphrase
// format), it falls back to the keyless git-anchored path (SignAuto). On
// failure to fall back it logs but does NOT block — the chain will detect the
// breach on next startup and alert.
func SignAfterLearning(coscaRoot string) *SignResult {
	result, err := Sign(coscaRoot)
	if err == nil {
		fmt.Fprintf(os.Stderr, "🔐 Chain re-signed (Ed25519, machine-bound) — block %d (%d files)\n", result.BlockNumber, result.FilesSigned)
		return result
	}
	fmt.Fprintf(os.Stderr, "⚠️  Ed25519 re-sign failed (%v) — falling back to git anchor\n", err)

	res, err := SignAuto(coscaRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Auto re-sign failed: %v\n", err)
		fmt.Fprintf(os.Stderr, "   Chain will be broken on next startup.\n")
		fmt.Fprintf(os.Stderr, "   Run: cosca-check --sign-auto\n")
		return nil
	}
	fmt.Fprintf(os.Stderr, "🔐 Chain re-signed (git-anchored) — block %d (%d files)\n", res.BlockNumber, res.FilesSigned)
	return res
}

// chainState returns the previous block hash and the next block number
// by inspecting the tail of the chain file. Genesis defaults are used when
// the chain does not exist yet.
func chainState(chainPath string) (prevHash string, blockNumber int) {
	prevHash = "0000000000000000000000000000000000000000000000000000000000000000"
	blockNumber = 1

	if existing, err := os.ReadFile(chainPath); err == nil && len(existing) > 0 {
		blocks, parseErr := parseBlocks(string(existing))
		if parseErr == nil && len(blocks) > 0 {
			lastBlock := blocks[len(blocks)-1]
			blockNumber = lastBlock.Number + 1
			if lastBlock.Hash != "" {
				prevHash = lastBlock.Hash
			}
		}
	}
	return prevHash, blockNumber
}

// manifestHashAndJSON returns the hash (per the given algorithm) of the
// compact manifest JSON and the JSON bytes themselves.
func manifestHashAndJSON(manifest []FileEntry, algo HashAlgo) (string, []byte, error) {
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return "", nil, err
	}
	return HashBytes(algo, manifestJSON), manifestJSON, nil
}

// appendChain appends a block to the chain file, creating it if necessary.
func appendChain(chainPath, blockText string) error {
	existingChain, _ := os.ReadFile(chainPath)
	var newChain []byte
	if len(existingChain) > 0 {
		newChain = append(existingChain, []byte("\n"+blockText)...)
	} else {
		newChain = []byte(blockText)
	}
	return os.WriteFile(chainPath, newChain, 0664)
}

// scanEmbed walks the embed directory and returns a sorted manifest of file
// entries, hashed with the given algorithm.
func scanEmbed(coscaRoot, embedDir string, algo HashAlgo) ([]FileEntry, error) {
	var entries []FileEntry

	err := filepath.Walk(embedDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(coscaRoot, path)
		if err != nil {
			return err
		}

		hash, err := HashFile(algo, path)
		if err != nil {
			return fmt.Errorf("hash %s: %w", path, err)
		}

		entries = append(entries, FileEntry{
			Path: relPath,
			Hash: hash,
			Size: info.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Sort by path for deterministic ordering
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	return entries, nil
}

// loadPrivateKey reads and unprotects an Ed25519 private key.
//
// MACHINE-BOUND (primary): the file holds a blob produced by protectMachineKey
// (DPAPI on Windows). Unprotecting requires the same machine + user; a different
// machine/user or a tampered blob yields an error.
//
// LEGACY: a file in the old "COSCA ENCRYPTED PRIVATE KEY" passphrase armored
// format cannot be recovered anymore, because there is no passphrase in the new
// contract. It returns a clear error pointing to `rekey` as the migration path
// (fresh machine-bound identity).
func loadPrivateKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("private key file is empty")
	}

	// Legacy passphrase-encrypted armored format — no passphrase is available.
	if isEncryptedKey(data) {
		return nil, fmt.Errorf("private key is in legacy passphrase-encrypted format — run a rekey to migrate to machine-bound protection")
	}

	// Primary format: machine-bound (DPAPI) protected blob.
	raw, err := unprotectPrivateKey(data)
	if err != nil {
		return nil, fmt.Errorf("unprotect machine-bound private key (wrong machine or user?): %w", err)
	}

	key, err := x509.ParsePKCS8PrivateKey(raw)
	if err != nil {
		return nil, fmt.Errorf("parse PKCS#8 private key: %w", err)
	}

	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not Ed25519")
	}
	return priv, nil
}
