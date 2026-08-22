//go:build windows

package integrity

import (
	"fmt"
	"runtime"

	"golang.org/x/sys/windows"
)

// applyPrivateKeyACL applies a REAL, restrictive DACL (M2) to the private key
// file so that ONLY the current user can read/modify it. It:
//
//   - grants GENERIC_ALL to the current user's SID,
//   - protects the DACL from inheritance (PROTECTED_DACL_SECURITY_INFORMATION),
//     so no inherited Everyone/Users ACE leaks in from the parent directory.
//
// The 0600 mode bits are a best-effort hint on Windows; this DACL is the actual
// enforcement. It is a MUST for the machine-bound key, since the DPAPI blob
// alone is not enough if any other principal can read the file and replay it to
// the same user context.
func applyPrivateKeyACL(path string) error {
	// A pseudo token for the current process that does not need to be closed,
	// used only to read the owner SID for the DACL.
	token := windows.GetCurrentProcessToken()

	user, err := token.GetTokenUser()
	if err != nil {
		return fmt.Errorf("get token user: %w", err)
	}

	// The TRUSTEE carries a raw pointer to the SID; it must be pinned for the
	// lifetime of the TrusteeValue (see x/sys windows security_windows.go).
	var pinner runtime.Pinner
	pinner.Pin(user.User.Sid)
	defer pinner.Unpin()

	access := []windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.GRANT_ACCESS,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid),
		},
	}}

	acl, err := windows.ACLFromEntries(access, nil)
	if err != nil {
		return fmt.Errorf("build dacl: %w", err)
	}

	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		acl,
		nil,
	); err != nil {
		return fmt.Errorf("set restrictive dacl: %w", err)
	}
	return nil
}
