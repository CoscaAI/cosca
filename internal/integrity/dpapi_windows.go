//go:build windows

package integrity

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// protectMachineKey protects a byte string with the Windows Data Protection API
// (DPAPI), binding it to the CURRENT USER on the CURRENT MACHINE
// (CryptProtectData default scope). Binding is implicit and non-spoofable: the
// blob can only be unprotected by the same user profile on the same machine, so
// it does not rely on a remembered secret (no passphrase/war phrase needed).
//
// The optional `entropy` argument is application-specific auxiliary data folded
// into the protection key; it is NOT a secret here (it lived in the binary) but
// acts as a domain separator so that only this build can unprotect its blobs.
// It must be identical between protect and unprotect calls. See machineKeyEntropy.
//
// CRYPTPROTECT_UI_FORBIDDEN (0x1) ensures no UI prompt is shown — the operation
// must be silent (never ask for a password/biometric on the fly).
func protectMachineKey(data []byte, entropy []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("protectMachineKey: empty input")
	}

	var dataIn windows.DataBlob
	dataIn.Size = uint32(len(data))
	dataIn.Data = &data[0]

	var entropyIn *windows.DataBlob
	if len(entropy) > 0 {
		entropyIn = &windows.DataBlob{Size: uint32(len(entropy)), Data: &entropy[0]}
	}

	var dataOut windows.DataBlob
	if err := windows.CryptProtectData(&dataIn, nil, entropyIn, 0, nil,
		windows.CRYPTPROTECT_UI_FORBIDDEN, &dataOut); err != nil {
		return nil, fmt.Errorf("protectMachineKey: CryptProtectData failed (not a Windows machine/user?): %w", err)
	}
	// The output blob is allocated by DPAPI and MUST be freed with LocalFree.
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(dataOut.Data)))

	out := make([]byte, dataOut.Size)
	copy(out, unsafe.Slice(dataOut.Data, dataOut.Size))
	return out, nil
}

// unprotectMachineKey reverses protectMachineKey. It only succeeds for the same
// user profile on the same machine that created the blob. A wrong machine, a
// different user, or a tampered blob returns an error (identity denied).
func unprotectMachineKey(blob []byte, entropy []byte) ([]byte, error) {
	if len(blob) == 0 {
		return nil, fmt.Errorf("unprotectMachineKey: empty input")
	}

	var dataIn windows.DataBlob
	dataIn.Size = uint32(len(blob))
	dataIn.Data = &blob[0]

	var entropyIn *windows.DataBlob
	if len(entropy) > 0 {
		entropyIn = &windows.DataBlob{Size: uint32(len(entropy)), Data: &entropy[0]}
	}

	var dataOut windows.DataBlob
	if err := windows.CryptUnprotectData(&dataIn, nil, entropyIn, 0, nil,
		windows.CRYPTPROTECT_UI_FORBIDDEN, &dataOut); err != nil {
		return nil, fmt.Errorf("unprotectMachineKey: CryptUnprotectData failed (wrong machine, user or tampered blob): %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(dataOut.Data)))

	out := make([]byte, dataOut.Size)
	copy(out, unsafe.Slice(dataOut.Data, dataOut.Size))
	return out, nil
}
