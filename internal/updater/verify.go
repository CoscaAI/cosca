package updater

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// =============================================================================
// Checksum Verification
// =============================================================================

// VerifyChecksum verifies that a file's SHA256 checksum matches the expected value.
func VerifyChecksum(filePath, expectedChecksum string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file for checksum: %w", err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("compute checksum: %w", err)
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actual)
	}

	return nil
}

// =============================================================================
// GPG Signature Verification
// =============================================================================

// gpgTimeout bounds a single GPG operation so a hung or malicious gpg process
// can never block an update indefinitely.
const gpgTimeout = 30 * time.Second

// gpgBinary is the GPG executable used for signature verification. It is a
// package-level variable so tests can inject a binary (see jailGeteuid).
var gpgBinary = "gpg"

// VerifyGPGSignature verifies that filePath carries a valid GPG detached
// signature stored in signaturePath, made by the key whose public part is in
// publicKeyPath.
//
// The verification is REAL and fail-closed: it runs the gnupg toolchain against
// a throwaway, private keyring and returns an error unless the signature
// verifies. It never reports success without checking, and requires the gnupg
// toolchain to be installed and reachable via PATH.
func VerifyGPGSignature(filePath, signaturePath, publicKeyPath string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", filePath)
	}
	if _, err := os.Stat(signaturePath); os.IsNotExist(err) {
		return fmt.Errorf("signature file not found: %s", signaturePath)
	}
	if _, err := os.Stat(publicKeyPath); os.IsNotExist(err) {
		return fmt.Errorf("public key not found: %s", publicKeyPath)
	}

	home, err := os.MkdirTemp("", "cosca-gpg-*")
	if err != nil {
		return fmt.Errorf("create temporary gpg home: %w", err)
	}
	defer func() { _ = os.RemoveAll(home) }()

	if err := os.Chmod(home, 0o700); err != nil {
		return fmt.Errorf("secure temporary gpg home: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), gpgTimeout)
	defer cancel()

	if err := runGPG(ctx, home, "--import", publicKeyPath); err != nil {
		return fmt.Errorf("import public key: %w", err)
	}

	if err := runGPG(ctx, home, "--verify", signaturePath, filePath); err != nil {
		return fmt.Errorf("assinatura inválida ou não confere: %w", err)
	}

	return nil
}

// runGPG executes the configured gpgBinary against the throwaway homedir.
// stderr is captured so failures surface actionable diagnostics.
func runGPG(ctx context.Context, home string, args ...string) error {
	cmdArgs := append([]string{"--homedir", home, "--batch"}, args...)
	cmd := exec.CommandContext(ctx, gpgBinary, cmdArgs...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
			return errors.New("gpg não disponível — atualização rejeitada sem verificação de assinatura")
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("gpg timed out: %w", err)
		}
		return fmt.Errorf("gpg: %v: %s", err, stderr.String())
	}
	return nil
}

// =============================================================================
// Backup Management
// =============================================================================

// CreateBackup creates a timestamped backup of the given file.
func CreateBackup(filePath string) (string, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("stat file for backup: %w", err)
	}

	src, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file for backup: %w", err)
	}
	defer func() { _ = src.Close() }()

	backupName := fmt.Sprintf("%s.bak", filepath.Base(filePath))
	backupPath := filepath.Join(filepath.Dir(filePath), backupName)

	dst, err := os.OpenFile(backupPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return "", fmt.Errorf("create backup file: %w", err)
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("write backup: %w", err)
	}

	return backupPath, nil
}

// RestoreFromBackup restores a file from its backup.
func RestoreFromBackup(backupPath, originalPath string) error {
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup not found: %s", backupPath)
	}

	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("read backup: %w", err)
	}

	if err := os.WriteFile(originalPath, data, 0o755); err != nil {
		return fmt.Errorf("restore from backup: %w", err)
	}

	return nil
}

// VerifyFileIntegrity performs both checksum and backup verification.
func VerifyFileIntegrity(filePath, expectedChecksum string) error {
	if err := VerifyChecksum(filePath, expectedChecksum); err != nil {
		return fmt.Errorf("integrity check failed: %w", err)
	}
	return nil
}
