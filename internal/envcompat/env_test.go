package envcompat

import (
	"os"
	"testing"
)

func TestApplyLegacyFallbacks(t *testing.T) {
	for _, tt := range []struct {
		name        string
		current     *string
		legacy      string
		want        string
		wantPresent bool
	}{
		{name: "legacy fallback", legacy: "beta", want: "beta", wantPresent: true},
		{name: "canonical wins", current: stringPointer("stable"), legacy: "beta", want: "stable", wantPresent: true},
		{name: "explicit empty canonical wins", current: stringPointer(""), legacy: "beta", want: "", wantPresent: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			currentName := currentPrefix + "CHANNEL"
			legacyName := legacyPrefix + "CHANNEL"
			unsetForTest(t, currentName)
			unsetForTest(t, legacyName)
			if tt.current != nil {
				t.Setenv(currentName, *tt.current)
			}
			t.Setenv(legacyName, tt.legacy)
			if err := ApplyLegacyFallbacks(); err != nil {
				t.Fatal(err)
			}
			got, present := os.LookupEnv(currentName)
			if got != tt.want || present != tt.wantPresent {
				t.Fatalf("%s = %q, %t; want %q, %t", currentName, got, present, tt.want, tt.wantPresent)
			}
		})
	}
}

func TestApplyLegacyFallbacksDoesNotRestoreRemovedVariable(t *testing.T) {
	const (
		current = "SHEVANIO_AI_CONFIRM_UPDATE"
		legacy  = "GENTLE_AI_CONFIRM_UPDATE"
	)
	unsetForTest(t, current)
	t.Setenv(legacy, "1")
	if err := ApplyLegacyFallbacks(); err != nil {
		t.Fatal(err)
	}
	if _, exists := os.LookupEnv(current); exists {
		t.Fatalf("removed variable %s was restored", current)
	}
}

func stringPointer(value string) *string { return &value }

func unsetForTest(t *testing.T, name string) {
	t.Helper()
	value, exists := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if exists {
			_ = os.Setenv(name, value)
		} else {
			_ = os.Unsetenv(name)
		}
	})
}
