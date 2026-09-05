//go:build !windows

package integrity

// applyPrivateKeyACL is a no-op on platforms that use POSIX mode bits. The
// private key file is created with 0600 (owner read/write only) by
// os.WriteFile, which — unlike Windows — is the actual enforcement mechanism on
// Linux/Darwin. The stronger DACL restrictions only matter on Windows
// (see acl_windows.go), which is where the real DPAPI machine-binding lives.
func applyPrivateKeyACL(path string) error {
	return nil
}
