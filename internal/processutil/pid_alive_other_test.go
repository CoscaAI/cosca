//go:build !windows && !unix

package processutil

// pidAlive reports whether a process with the given PID is still running. On
// platforms without a supported liveness probe we conservatively report dead so
// the orphan test cannot false-positive.
func pidAlive(pid int) bool {
	return false
}
