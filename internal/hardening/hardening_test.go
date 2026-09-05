package hardening

import (
	"os"
	"reflect"
	"testing"
)

func TestRemoveDangerousEnv(t *testing.T) {
	environ := []string{
		"PATH=/usr/bin:/bin",
		"HOME=/home/user",
		"LD_PRELOAD=/tmp/libevil.so",
		"LD_LIBRARY_PATH=/tmp/evil",
		"DYLD_INSERT_LIBRARIES=/tmp/libevil.dylib",
		"DYLD_LIBRARY_PATH=/tmp/evil",
		"OPENAI_API_KEY=sk-test",
	}
	got := RemoveDangerousEnv(environ)
	want := []string{
		"PATH=/usr/bin:/bin",
		"HOME=/home/user",
		"OPENAI_API_KEY=sk-test",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RemoveDangerousEnv:\n got %v\nwant %v", got, want)
	}
	// The input must not be modified.
	if len(environ) != 7 {
		t.Errorf("input environ modified: len = %d, want 7", len(environ))
	}
}

func TestRemoveDangerousEnvPreservesSimilarNames(t *testing.T) {
	environ := []string{
		"MY_LD_PRELOAD=harmless",
		"LD_PRELOAD2=x",
		"NOT_DYLD_LIBRARY_PATH=y",
		"DYDYLD_LIBRARY_PATH=z",
	}
	got := RemoveDangerousEnv(environ)
	if len(got) != len(environ) {
		t.Errorf("RemoveDangerousEnv dropped non-dangerous vars: %v", got)
	}
}

func TestRemoveDangerousEnvEmptyAndNil(t *testing.T) {
	if got := RemoveDangerousEnv(nil); got == nil || len(got) != 0 {
		t.Errorf("RemoveDangerousEnv(nil) = %v, want empty non-nil slice", got)
	}
	if got := RemoveDangerousEnv([]string{}); len(got) != 0 {
		t.Errorf("RemoveDangerousEnv([]) = %v, want empty", got)
	}
}

func TestPreMainDoesNotPanic(t *testing.T) {
	// PreMain must never panic or fail, on any platform, and must not remove
	// benign variables.
	benign := []string{"PATH", "HOME", "OPENAI_API_KEY", "COSCA_JAILED"}
	orig := make(map[string]string)
	for _, k := range benign {
		if v, ok := os.LookupEnv(k); ok {
			orig[k] = v
		}
	}

	PreMain()

	for k := range orig {
		if v, ok := os.LookupEnv(k); !ok || v != orig[k] {
			t.Errorf("PreMain removed benign env var %q (was %q)", k, orig[k])
		}
	}
}

func TestPreMainRemovesDangerousEnv(t *testing.T) {
	_ = os.Setenv("LD_PRELOAD", "/tmp/libevil.so")
	_ = os.Setenv("LD_LIBRARY_PATH", "/tmp/evil")
	defer os.Unsetenv("LD_PRELOAD")
	defer os.Unsetenv("LD_LIBRARY_PATH")

	PreMain()

	if v, ok := os.LookupEnv("LD_PRELOAD"); ok {
		t.Errorf("LD_PRELOAD still set after PreMain: %q", v)
	}
	if v, ok := os.LookupEnv("LD_LIBRARY_PATH"); ok {
		t.Errorf("LD_LIBRARY_PATH still set after PreMain: %q", v)
	}
}
