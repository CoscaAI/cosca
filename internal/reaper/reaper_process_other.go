//go:build !unix && !windows

package reaper

import "errors"

// This platform (neither POSIX nor Windows) cannot probe liveness or read the
// command line of a PID. The process reaper fails closed: procAlive returns
// false so the entry is reported "missing" and nothing is ever terminated by
// this package on such platforms. Kept conservative by design.

// procAlive conservatively reports dead; the process reaper then no-ops.
func procAlive(pid int) bool { return false }

// procCommandLine is unsupported; the reaper skips if it reached this point.
func procCommandLine(pid int) (string, error) {
	return "", errors.New("reaper: command-line introspection unsupported on this platform")
}

// procStop is unsupported.
func procStop(pid int) error { return errors.New("reaper: process stop unsupported on this platform") }

// procKill is unsupported.
func procKill(pid int) error { return errors.New("reaper: process kill unsupported on this platform") }
