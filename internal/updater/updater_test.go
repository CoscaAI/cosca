package updater

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestChannels(t *testing.T) {
	t.Parallel()

	if ChannelStable != "stable" {
		t.Errorf("ChannelStable = %q", ChannelStable)
	}
	if ChannelBeta != "beta" {
		t.Errorf("ChannelBeta = %q", ChannelBeta)
	}
	if ChannelNightly != "nightly" {
		t.Errorf("ChannelNightly = %q", ChannelNightly)
	}
}

func TestNewUpdater(t *testing.T) {
	t.Parallel()

	u := NewUpdater("1.0.0", "stable")
	if u == nil {
		t.Fatal("NewUpdater returned nil")
	}
	if u.currentVersion != "1.0.0" {
		t.Errorf("currentVersion = %q", u.currentVersion)
	}
	if u.channel != "stable" {
		t.Errorf("channel = %q", u.channel)
	}
}

func TestCheck(t *testing.T) {
	t.Parallel()

	u := NewUpdater("1.0.0", "stable")
	info, err := u.Check()
	if err != nil {
		t.Fatalf("Check error: %v", err)
	}
	if info == nil {
		t.Fatal("info should not be nil")
	}
	if info.CurrentVersion != "1.0.0" {
		t.Errorf("CurrentVersion = %q", info.CurrentVersion)
	}
	if info.UpdateAvailable {
		t.Error("UpdateAvailable should be false (no network)")
	}
}

func TestApplyWithoutCheck(t *testing.T) {
	t.Parallel()

	u := NewUpdater("1.0.0", "stable")
	err := u.Apply()
	if err == nil {
		t.Error("expected error when Apply() called before Check()")
	}
}

func TestApplyNoUpdate(t *testing.T) {
	t.Parallel()

	u := NewUpdater("1.0.0", "stable")
	_, _ = u.Check()
	err := u.Apply()
	if err == nil {
		t.Error("expected error when no update available")
	}
}

func TestCheckForUpdates(t *testing.T) {
	t.Parallel()

	info, err := CheckForUpdates("1.0.0", "owner", "repo")
	if err != nil {
		t.Fatalf("CheckForUpdates error: %v", err)
	}
	if info.CurrentVersion != "1.0.0" {
		t.Errorf("CurrentVersion = %q", info.CurrentVersion)
	}
}

func TestDownloadUpdate(t *testing.T) {
	t.Parallel()

	err := DownloadUpdate("", "checksum123", "/tmp/dest")
	if err == nil {
		t.Error("expected error for empty URL")
	}

	err = DownloadUpdate("https://example.com/update", "checksum", "/tmp/dest")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBinaryName(t *testing.T) {
	t.Parallel()

	name := BinaryName()
	if name == "" {
		t.Error("BinaryName should not be empty")
	}
}

func TestUpdateInfo(t *testing.T) {
	t.Parallel()

	info := UpdateInfo{
		CurrentVersion:  "1.0.0",
		LatestVersion:   "2.0.0",
		UpdateAvailable: true,
		DownloadURL:     "https://example.com/cosca-v2",
		Checksum:        "abc123",
		Changelog:       []string{"Fix bug", "Add feature"},
	}

	if !info.UpdateAvailable {
		t.Error("UpdateAvailable should be true")
	}
	if len(info.Changelog) != 2 {
		t.Errorf("Changelog len = %d", len(info.Changelog))
	}
}

func TestVerifyChecksumEmpty(t *testing.T) {
	t.Parallel()

	// Non-existent file should error
	err := VerifyChecksum("/nonexistent", "hash")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestVerifyGPGSignature(t *testing.T) {
	t.Parallel()

	err := VerifyGPGSignature("/file", "/nonexistent-sig", "/nonexistent-key")
	if err == nil {
		t.Error("expected error for nonexistent signature file")
	}
}

func TestVerifyGPGSignature_NoGPG(t *testing.T) {
	orig := gpgBinary
	gpgBinary = "/nonexistent/gpg"
	t.Cleanup(func() { gpgBinary = orig })

	dir := t.TempDir()
	filePath := filepath.Join(dir, "payload.bin")
	sigPath := filepath.Join(dir, "payload.asc")
	keyPath := filepath.Join(dir, "public.asc")
	for _, p := range []string{filePath, sigPath, keyPath} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatalf("write temp file %s: %v", p, err)
		}
	}

	err := VerifyGPGSignature(filePath, sigPath, keyPath)
	if err == nil {
		t.Fatal("expected error when gpg is unavailable")
	}
	if !strings.Contains(err.Error(), "gpg não disponível") {
		t.Fatalf("error should mention unavailable gpg, got: %v", err)
	}
}

func TestVerifyGPGSignature_RealSignature(t *testing.T) {
	gpgBin := "/usr/bin/gpg"
	if _, err := os.Stat(gpgBin); err != nil {
		t.Skip("gpg binary not found")
	}

	orig := gpgBinary
	gpgBinary = gpgBin
	t.Cleanup(func() { gpgBinary = orig })

	home := t.TempDir()
	if err := os.Chmod(home, 0o700); err != nil {
		t.Fatalf("chmod gpg home: %v", err)
	}

	runGPGTest := func(args ...string) error {
		cmd := exec.Command(gpgBin, append([]string{"--homedir", home, "--batch"}, args...)...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%v: %s", err, stderr.String())
		}
		return nil
	}

	if err := runGPGTest("--passphrase", "", "--quick-gen-key",
		"test <test@cosca.local>", "default", "default", "never"); err != nil {
		t.Skipf("gpg keygen indisponível: %v", err)
	}

	pub, err := exec.Command(gpgBin, "--homedir", home, "--batch",
		"--armor", "--export", "test@cosca.local").Output()
	if err != nil {
		t.Fatalf("export public key: %v", err)
	}
	keyPath := filepath.Join(home, "public.asc")
	if err := os.WriteFile(keyPath, pub, 0o600); err != nil {
		t.Fatalf("write public key: %v", err)
	}

	filePath := filepath.Join(home, "payload.bin")
	if err := os.WriteFile(filePath, []byte("cosca update payload\n"), 0o600); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	sigPath := filepath.Join(home, "payload.asc")
	if err := runGPGTest("--armor", "--detach-sign", "--output", sigPath, filePath); err != nil {
		t.Fatalf("detach-sign: %v", err)
	}

	if err := VerifyGPGSignature(filePath, sigPath, keyPath); err != nil {
		t.Fatalf("expected valid signature to pass, got: %v", err)
	}

	if err := os.WriteFile(filePath, []byte("tampered payload\n"), 0o600); err != nil {
		t.Fatalf("tamper payload: %v", err)
	}
	if err := VerifyGPGSignature(filePath, sigPath, keyPath); err == nil {
		t.Fatal("expected error for tampered payload")
	}
}

func TestCreateBackup(t *testing.T) {
	t.Parallel()

	_, err := CreateBackup("/nonexistent-file")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestRestoreFromBackup(t *testing.T) {
	t.Parallel()

	err := RestoreFromBackup("/nonexistent-backup", "/original")
	if err == nil {
		t.Error("expected error for nonexistent backup")
	}
}

func TestVerifyFileIntegrity(t *testing.T) {
	t.Parallel()

	err := VerifyFileIntegrity("/nonexistent", "hash")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
