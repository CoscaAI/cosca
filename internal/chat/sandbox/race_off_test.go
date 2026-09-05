//go:build !race

package sandbox

// testingRace reports whether the test binary was built with -race. Under the
// non-race build the race build tag is absent, so this returns false.
func testingRace() bool { return false }
