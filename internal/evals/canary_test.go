package evals

import "testing"

func TestStripCanaryHTML(t *testing.T) {
	in := "<!-- canary: deadbeef -->\n\nBuild the thing.\n"
	want := "Build the thing."
	if got := StripCanary(in); got != want {
		t.Errorf("StripCanary(%q) = %q, want %q", in, got, want)
	}
}

func TestStripCanaryHash(t *testing.T) {
	in := "# canary: 1234abcd\n\nBuild the thing.\n"
	want := "Build the thing."
	if got := StripCanary(in); got != want {
		t.Errorf("StripCanary(%q) = %q, want %q", in, got, want)
	}
}

func TestStripCanaryHashCaseInsensitive(t *testing.T) {
	in := "# CANARY: 9999\nBuild the thing."
	want := "Build the thing."
	if got := StripCanary(in); got != want {
		t.Errorf("StripCanary(%q) = %q, want %q", in, got, want)
	}
}

func TestStripCanaryMultipleAndBlankLines(t *testing.T) {
	in := "<!-- canary: a -->\n# canary: b\n\n\n\nBuild the thing."
	want := "Build the thing."
	if got := StripCanary(in); got != want {
		t.Errorf("StripCanary(%q) = %q, want %q", in, got, want)
	}
}

func TestStripCanaryLeavesNormalTextIntact(t *testing.T) {
	in := "Build the thing.\nSecond line."
	if got := StripCanary(in); got != in {
		t.Errorf("StripCanary(%q) = %q, want unchanged", in, got)
	}
}

func TestStripCanaryLeavesMiddleHashComments(t *testing.T) {
	// A non-canary hash comment in the middle of the prompt is not a canary
	// marker and must survive (only "canary" comment lines are stripped).
	in := "Context paragraph.\n\n# note: keep this hint\n\nDo the thing."
	want := "Context paragraph.\n\n# note: keep this hint\n\nDo the thing."
	if got := StripCanary(in); got != want {
		t.Errorf("StripCanary(%q) = %q, want %q", in, got, want)
	}
}

func TestStripCanaryMiddleMarker(t *testing.T) {
	in := "Intro line.\n\n# canary: mid-123\n\nMore instructions.\n"
	want := "Intro line.\n\nMore instructions."
	if got := StripCanary(in); got != want {
		t.Errorf("StripCanary(%q) = %q, want %q", in, got, want)
	}
}

func TestStripCanaryEmpty(t *testing.T) {
	for _, in := range []string{"", "\n", "\n\n"} {
		if got := StripCanary(in); got != "" {
			t.Errorf("StripCanary(%q) = %q, want empty", in, got)
		}
	}
}
