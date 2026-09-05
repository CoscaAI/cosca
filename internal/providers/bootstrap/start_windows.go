//go:build windows

package bootstrap

import (
	"os"
	"os/exec"
	"syscall"
)

// detachedProcess (0x00000008) is CREATE_PROCESS_DETACHED from the Windows
// SDK. It is not exported by the syscall package, so it is defined locally.
const detachedProcess = 0x00000008

// startDetachedProcess starts name in the background, detached from the
// calling process (DETACHED_PROCESS, new process group, no console window),
// with output discarded. It survives the bootstrap process exiting.
//
// REVERSIBLE: the started process is `ollama serve`; kill it with
// `taskkill /PID <pid>` (the pid is surfaced in the report evidence) or via
// Task Manager.
func startDetachedProcess(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	// DETACHED_PROCESS: the child gets no console and is not tied to ours.
	// CREATE_NEW_PROCESS_GROUP keeps Ctrl+C in our console from reaching it.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | detachedProcess,
	}
	nul, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer nul.Close()
	cmd.Stdin = nul
	cmd.Stdout = nul
	cmd.Stderr = nul
	if err := cmd.Start(); err != nil {
		return err
	}
	// Release resources; the child keeps running detached.
	return cmd.Process.Release()
}
