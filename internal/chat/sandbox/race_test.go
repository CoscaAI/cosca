//go:build race

package sandbox

// testingRace reports whether the test binary was built with -race. This file
// is only compiled when the race build tag is active (go test -race sets it),
// so testingRace returns true exactly under the race detector.
func testingRace() bool { return true }
