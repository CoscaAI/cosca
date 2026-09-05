package contracts

import (
	"sort"
)

// sortedKeys returns the keys of a string-keyed map in sorted order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sortedMajorKeys returns the major keys of a method registry in ascending
// order.
func sortedMajorKeys(m MethodRegistry) []int {
	keys := make([]int, 0, len(m.MajorLines))
	for k := range m.MajorLines {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

// sortedMinorKeys returns the installed minor keys of a major line in
// ascending order.
func sortedMinorKeys(line MajorLine) []int {
	keys := make([]int, 0, len(line.Versions))
	for k := range line.Versions {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

// highestInstalledMinor returns the highest installed minor within a line,
// or -1 if the line has no versions.
func highestInstalledMinor(line MajorLine) int {
	highest := -1
	for minor := range line.Versions {
		if minor > highest {
			highest = minor
		}
	}
	return highest
}

// latestContract returns the contract of the latest minor of the given major.
// Callers must have validated that the major line exists.
func latestContract(m MethodRegistry, major int) (RpcContract, bool) {
	line, ok := m.MajorLines[major]
	if !ok {
		return RpcContract{}, false
	}
	entry, ok := line.Versions[line.LatestMinor]
	if !ok {
		return RpcContract{}, false
	}
	return entry.Contract, true
}

// previousInstalledVersion walks majors in ascending order and returns the
// schema version of the last installed contract before `major`, or nil when
// `major` is the first installed major.
func previousInstalledVersion(m MethodRegistry, major int) *SchemaVersion {
	majors := sortedMajorKeys(m)
	var prev *SchemaVersion
	for _, maj := range majors {
		if maj >= major {
			break
		}
		line := m.MajorLines[maj]
		if c, ok := line.Versions[line.LatestMinor]; ok {
			v := c.Contract.SchemaVersion
			prev = &v
		}
	}
	return prev
}
