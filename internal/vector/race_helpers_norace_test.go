//go:build !race

package vector

// testingRace reports whether the test binary was built with -race.
// This file is compiled when the race build tag is NOT active, so
// testingRace returns false in normal builds.
func testingRace() bool { return false }
