package contracts

import (
	"fmt"
	"reflect"
)

// ValidationError describes a single invariant violation found during
// registry validation.
type ValidationError struct {
	Method  string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Method != "" {
		return fmt.Sprintf("method %q: %s", e.Method, e.Message)
	}
	return e.Message
}

// ValidateRegistry enforces the versioned contract invariants. It fails fast
// on the first violation with a precise message. Structural checks run first
// (invariants 1 and 4); schema additivity/breaking (2 and 3) run in a second
// pass so structural errors always surface before schema complaints.
//
//  1. Structural integrity: latestMinor must be the highest installed minor,
//     every version must match its declared method/major/minor, and upgrade
//     paths must chain from the previous installed version.
//  2. Minor additivity: within a major, a minor bump must not remove or
//     change existing request/response fields.
//  3. Major breaking: a major bump must change request or response payload
//     shape, otherwise it "could have shipped as a minor".
//  4. Downgrade/degrades: downgrade paths target an older major from the
//     latest minor; non-floor methods declare a well-formed degrade.
func ValidateRegistry(r *Registry, floorMethods []string) error {
	if r == nil {
		return &ValidationError{Message: "registry is nil"}
	}

	// Pass 1: structural invariants per method.
	for method, m := range r.Methods {
		if err := validateMethodStructure(method, m); err != nil {
			return err
		}
	}

	// Pass 2: schema compatibility (additivity for minors, breaking for majors).
	for method, m := range r.Methods {
		if err := validateSchemaCompatibility(method, m); err != nil {
			return err
		}
	}

	// Pass 3: degrade strategies for non-floor methods.
	if err := validateDegrades(r, floorMethods); err != nil {
		return err
	}

	return nil
}

// validateMethodStructure enforces invariant 1.
func validateMethodStructure(method string, m MethodRegistry) error {
	majors := sortedMajorKeys(m)

	// A method must have at least one major.
	if len(majors) == 0 {
		return &ValidationError{Method: method, Message: "no major versions declared"}
	}

	var prev *SchemaVersion

	for _, major := range majors {
		line, ok := m.MajorLines[major]
		if !ok {
			return &ValidationError{Method: method, Message: fmt.Sprintf("major line %d missing", major)}
		}

		// latestMinor must point at an installed version.
		if _, ok := line.Versions[line.LatestMinor]; !ok {
			return &ValidationError{Method: method, Message: fmt.Sprintf("latestMinor %d is not installed for major %d", line.LatestMinor, major)}
		}

		// latestMinor must be the highest installed minor.
		if highest := highestInstalledMinor(line); highest != line.LatestMinor {
			return &ValidationError{Method: method, Message: fmt.Sprintf("latestMinor %d for major %d must be the highest installed minor %d", line.LatestMinor, major, highest)}
		}

		minors := sortedMinorKeys(line)
		for _, minor := range minors {
			entry := line.Versions[minor]
			c := entry.Contract

			if c.Method != method {
				return &ValidationError{Method: method, Message: fmt.Sprintf("contract method %q does not match registry method", c.Method)}
			}
			if c.SchemaVersion.Major != major {
				return &ValidationError{Method: method, Message: fmt.Sprintf("contract for minor %d must declare major %d", minor, major)}
			}
			if c.SchemaVersion.Minor != minor {
				return &ValidationError{Method: method, Message: fmt.Sprintf("contract for major %d must declare minor %d", major, minor)}
			}

			if prev == nil {
				if entry.UpgradeFromPrevious != nil {
					return &ValidationError{Method: method, Message: fmt.Sprintf("version %d.%d cannot define an upgrade path without a previous installed version", major, minor)}
				}
			} else {
				if entry.UpgradeFromPrevious == nil {
					return &ValidationError{Method: method, Message: fmt.Sprintf("version %d.%d must define an upgrade path from %s", major, minor, prev)}
				}
				up := entry.UpgradeFromPrevious
				if up.From.Major != prev.Major || up.From.Minor != prev.Minor {
					return &ValidationError{Method: method, Message: fmt.Sprintf("upgrade path for %d.%d must start at previous installed version %s", major, minor, prev)}
				}
				if up.To.Major != major || up.To.Minor != minor {
					return &ValidationError{Method: method, Message: fmt.Sprintf("upgrade path for version %d.%d must end at itself", major, minor)}
				}
				if up.UpgradeRequest == nil {
					return &ValidationError{Method: method, Message: fmt.Sprintf("upgrade path %s→%s must declare UpgradeRequest", up.From, up.To)}
				}
				if up.UpgradeResponse == nil {
					return &ValidationError{Method: method, Message: fmt.Sprintf("upgrade path %s→%s must declare UpgradeResponse", up.From, up.To)}
				}
			}

			v := SchemaVersion{Major: major, Minor: minor}
			prev = &v
		}

		// Downgrade paths must target older majors, from latest minor to
		// their latest minor.
		for targetMajor, dp := range line.DowngradePathsFromLatest {
			targetLine, ok := m.MajorLines[targetMajor]
			if !ok {
				return &ValidationError{Method: method, Message: fmt.Sprintf("downgrade path of major %d targets undefined major %d", major, targetMajor)}
			}
			if targetMajor >= major {
				return &ValidationError{Method: method, Message: fmt.Sprintf("downgrade path of major %d must target an older major than %d", major, major)}
			}
			if dp.From.Major != major || dp.From.Minor != line.LatestMinor {
				return &ValidationError{Method: method, Message: fmt.Sprintf("downgrade path of major %d must start at latest minor %d", major, line.LatestMinor)}
			}
			if dp.To.Major != targetMajor || dp.To.Minor != targetLine.LatestMinor {
				return &ValidationError{Method: method, Message: fmt.Sprintf("downgrade path of major %d to major %d must end at latest minor %d of that major", major, targetMajor, targetLine.LatestMinor)}
			}
			if dp.DowngradeRequest == nil {
				return &ValidationError{Method: method, Message: fmt.Sprintf("downgrade path %s→%s must declare DowngradeRequest", dp.From, dp.To)}
			}
			if dp.DowngradeResponse == nil {
				return &ValidationError{Method: method, Message: fmt.Sprintf("downgrade path %s→%s must declare DowngradeResponse", dp.From, dp.To)}
			}
		}
	}

	return nil
}

// validateSchemaCompatibility enforces invariants 2 and 3.
func validateSchemaCompatibility(method string, m MethodRegistry) error {
	majors := sortedMajorKeys(m)

	for _, major := range majors {
		line := m.MajorLines[major]
		minors := sortedMinorKeys(line)

		// Minor bumps must be additive (invariant 2).
		for i := 1; i < len(minors); i++ {
			prevEntry := line.Versions[minors[i-1]]
			currEntry := line.Versions[minors[i]]

			if reqV := findNonAdditive(prevEntry.Contract.Request, currEntry.Contract.Request); reqV != "" {
				return &ValidationError{Method: method, Message: fmt.Sprintf("minor %d.%d request %s from %d.%d", major, minors[i], reqV, major, minors[i-1])}
			}
			if respV := findNonAdditive(prevEntry.Contract.Response, currEntry.Contract.Response); respV != "" {
				return &ValidationError{Method: method, Message: fmt.Sprintf("minor %d.%d response %s from %d.%d", major, minors[i], respV, major, minors[i-1])}
			}
		}
	}

	// Major bumps must be breaking (invariant 3).
	for i := 1; i < len(majors); i++ {
		prevMajor := majors[i-1]
		currMajor := majors[i]

		prevLatest, ok := latestContract(m, prevMajor)
		if !ok {
			return &ValidationError{Method: method, Message: fmt.Sprintf("cannot resolve latest contract of major %d", prevMajor)}
		}
		currLatest, ok := latestContract(m, currMajor)
		if !ok {
			return &ValidationError{Method: method, Message: fmt.Sprintf("cannot resolve latest contract of major %d", currMajor)}
		}

		reqChanged := !payloadEquivalent(prevLatest.Request, currLatest.Request)
		respChanged := !payloadEquivalent(prevLatest.Response, currLatest.Response)

		if !reqChanged && !respChanged {
			return &ValidationError{Method: method, Message: fmt.Sprintf("major bump %d -> %d is not a breaking change (could have shipped as a minor)", prevMajor, currMajor)}
		}
	}

	return nil
}

// validateDegrades enforces invariant 4 for non-floor methods.
func validateDegrades(r *Registry, floorMethods []string) error {
	floor := map[string]bool{}
	for _, name := range floorMethods {
		floor[name] = true
	}

	for method, m := range r.Methods {
		if floor[method] {
			continue
		}

		if m.Degrade == nil {
			return &ValidationError{Method: method, Message: "non-floor method must declare a degrade strategy"}
		}

		switch m.Degrade.Strategy {
		case DegradeUnsupported:
			// No further requirements.
		case DegradeFallback:
			fb := m.Degrade.Fallback
			if fb == nil {
				return &ValidationError{Method: method, Message: "fallback degrade must declare a Fallback target"}
			}
			if !floor[fb.ToMethod] {
				return &ValidationError{Method: method, Message: fmt.Sprintf("fallback degrade must target a floor method, got %q", fb.ToMethod)}
			}
			// The targeted floor version must exist.
			target := r.Methods[fb.ToMethod]
			line, ok := target.MajorLines[fb.ToVersion.Major]
			if !ok {
				return &ValidationError{Method: method, Message: fmt.Sprintf("fallback degrade targets missing major %d on floor method %q", fb.ToVersion.Major, fb.ToMethod)}
			}
			if _, ok := line.Versions[fb.ToVersion.Minor]; !ok {
				return &ValidationError{Method: method, Message: fmt.Sprintf("fallback degrade targets missing version %s on floor method %q", fb.ToVersion, fb.ToMethod)}
			}
			if fb.AdaptRequest == nil || fb.AdaptResponse == nil {
				return &ValidationError{Method: method, Message: "fallback degrade must declare AdaptRequest and AdaptResponse"}
			}
		default:
			return &ValidationError{Method: method, Message: fmt.Sprintf("unknown degrade strategy %q", m.Degrade.Strategy)}
		}
	}

	return nil
}

// ---- Payload compatibility (structural) ---- //

// payloadEquivalent reports whether two payload shapes are structurally
// equivalent: same type kind, and for structs same exported field names with
// equal types. nil and *struct{} (empty) are treated as equivalent.
func payloadEquivalent(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ta := derefType(reflect.TypeOf(a))
	tb := derefType(reflect.TypeOf(b))
	if ta != tb {
		return false
	}
	if ta.Kind() != reflect.Struct {
		return ta == tb
	}
	return fieldSignatures(ta) == fieldSignatures(tb)
}

// findNonAdditive returns a description of the first field removed or changed
// in `curr` relative to `prev`, or "" if the change is purely additive.
// A field added in curr is additive (OK); a field that existed in prev and is
// gone or has a different type in curr is a violation.
func findNonAdditive(prev, curr any) string {
	if prev == nil {
		return "" // adding the first payload shape is always fine
	}
	// If both are nil or equal, no change.
	if curr == nil {
		return "removed (payload became empty)"
	}

	prevT := derefType(reflect.TypeOf(prev))
	currT := derefType(reflect.TypeOf(curr))

	if prevT.Kind() == reflect.Struct && currT.Kind() == reflect.Struct {
		prevFields := exportedFieldMap(prevT)
		currFields := exportedFieldMap(currT)

		for name, prevFT := range prevFields {
			currFT, ok := currFields[name]
			if !ok {
				return fmt.Sprintf("removed field %q", name)
			}
			if currFT != prevFT {
				return fmt.Sprintf("changed field %q type %s -> %s", name, prevFT, currFT)
			}
		}
		return ""
	}

	// Non-struct payloads: any type change counts as non-additive.
	if prevT != currT {
		return fmt.Sprintf("changed payload type %s -> %s", prevT, currT)
	}
	return ""
}

// exportedFieldMap returns exported field name -> type for a struct type.
func exportedFieldMap(t reflect.Type) map[string]reflect.Type {
	out := map[string]reflect.Type{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath == "" { // exported
			out[f.Name] = f.Type
		}
	}
	return out
}

// fieldSignatures returns a stable string of the struct's exported field
// signatures for equality comparison.
func fieldSignatures(t reflect.Type) string {
	fields := exportedFieldMap(t)
	out := ""
	// Sort for determinism.
	for _, name := range sortedKeys(fields) {
		out += name + ":" + fields[name].String() + ";"
	}
	return out
}

// derefType unwraps pointers to their element type.
func derefType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}
