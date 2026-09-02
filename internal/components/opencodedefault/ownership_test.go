package opencodedefault

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/shevanio/shevanio-ai/v2/internal/components/mutationjournal"
	"github.com/shevanio/shevanio-ai/v2/internal/model"
)

func check(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func read(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	check(t, err)
	return data
}
func writeFileMode(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	check(t, os.WriteFile(path, data, mode))
	check(t, os.Chmod(path, mode))
}
func requireFile(t *testing.T, path string, want []byte, mode os.FileMode) {
	t.Helper()
	if got := read(t, path); !bytes.Equal(got, want) {
		t.Fatalf("%s bytes = %q, want %q", path, got, want)
	}
	info, err := os.Stat(path)
	check(t, err)
	if got := info.Mode().Perm(); got != mode {
		t.Fatalf("%s mode = %#o, want %#o", path, got, mode)
	}
}
func requireAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("%s exists or cannot be inspected: %v", path, err)
	}
}
func failAfterSecondPairMutation(want error) pairMutator {
	calls := 0
	return func(journal *mutationjournal.Journal, mutation pairMutation) error {
		if err := mutatePair(journal, mutation); err != nil {
			return err
		}
		calls++
		if calls == 2 {
			return want
		}
		return nil
	}
}
func TestOwnershipLifecycle(t *testing.T) {
	settings := filepath.Join(t.TempDir(), "opencode.json")
	write := func(body string) { check(t, os.WriteFile(settings, []byte(body), 0o644)) }
	var installChanged bool
	install := func() {
		plan, err := PrepareInstall(settings)
		check(t, err)
		installChanged, err = plan.Apply()
		check(t, err)
	}
	var uninstallChanged bool
	uninstall := func() {
		plan, err := PrepareUninstall(settings)
		check(t, err)
		raw, err := os.ReadFile(settings)
		var applyErr error
		uninstallChanged, _, applyErr = plan.Apply(raw, err == nil)
		check(t, applyErr)
	}
	wantDefault := func(want string) {
		got := read(t, settings)
		if !bytes.Contains(got, []byte(`"default_agent": "`+want+`"`)) {
			t.Fatalf("default %q not restored: %s", want, got)
		}
	}
	write(`{"default_agent":"build","agent":{"shevanio-orchestrator":{}},"profile":true}`)
	install()
	if !installChanged {
		t.Fatal("first install reported no change")
	}
	wantDefault("shevanio-orchestrator")
	install()
	if installChanged {
		t.Fatal("idempotent install reported a change")
	}
	uninstall()
	if !uninstallChanged {
		t.Fatal("first uninstall reported no change")
	}
	wantDefault("build")
	uninstall()
	if uninstallChanged {
		t.Fatal("idempotent uninstall reported a change")
	}
	wantDefault("build")
	write(`{"default_agent":"plan","profile":true}`)
	install()
	uninstall()
	wantDefault("plan")
	install()
	check(t, os.Remove(settings))
	write(`{"default_agent":"gentle-orchestrator","profile":true}`)
	install()
	uninstall()
	wantDefault("plan")
	install()
	check(t, os.Remove(settings))
	uninstall()
	if _, err := os.Stat(OwnershipPath(settings)); !os.IsNotExist(err) {
		t.Fatalf("stale ownership remains: %v", err)
	}
	install()
	uninstall()
	if _, err := os.Stat(settings); !os.IsNotExist(err) {
		t.Fatalf("fresh absence was not restored: %v", err)
	}
	write(`{"default_agent":"build"}`)
	before := read(t, settings)
	check(t, os.WriteFile(OwnershipPath(settings), []byte(`{"schema":"wrong"}`), 0o644))
	if _, err := PrepareUninstall(settings); err == nil {
		t.Fatal("malformed ownership was accepted")
	}
	if after := read(t, settings); !bytes.Equal(before, after) {
		t.Fatalf("settings changed: %q", after)
	}
}

func TestInstallPreservesValidOwnershipWithoutManagedActorEntry(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "opencode.json")
	ownerPath := OwnershipPath(settings)
	settingsRaw := encode(map[string]any{"default_agent": model.CanonicalManagedIdentity.Actor, "profile": true})
	ownerRaw := encode(newOwnership(fieldValue{present: true, value: "build"}))
	writeFileMode(t, settings, settingsRaw, 0o600)
	writeFileMode(t, ownerPath, ownerRaw, 0o640)

	plan, err := PrepareInstall(settings)
	check(t, err)
	changed, err := plan.Apply()
	check(t, err)
	if changed {
		t.Fatal("install recaptured valid ownership when the actor entry was absent")
	}
	requireFile(t, settings, settingsRaw, 0o600)
	requireFile(t, ownerPath, ownerRaw, 0o640)
}

func TestInstallPreservesValidOwnershipDuringLegacyAliasMigration(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "opencode.json")
	ownerPath := OwnershipPath(settings)
	settingsRaw := encode(map[string]any{"default_agent": "gentle-orchestrator", "profile": true})
	ownerRaw := encode(newOwnership(fieldValue{present: true, value: "build"}))
	writeFileMode(t, settings, settingsRaw, 0o600)
	writeFileMode(t, ownerPath, ownerRaw, 0o640)

	plan, err := PrepareInstall(settings)
	check(t, err)
	changed, err := plan.Apply()
	check(t, err)
	if !changed {
		t.Fatal("legacy default migration reported no change")
	}
	wantSettings := encode(map[string]any{"default_agent": model.CanonicalManagedIdentity.Actor, "profile": true})
	requireFile(t, settings, wantSettings, 0o600)
	requireFile(t, ownerPath, ownerRaw, 0o640)
}

func TestUninstallRestoresOwnedLegacyDefault(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "opencode.json")
	ownerPath := OwnershipPath(settings)
	settingsRaw := encode(map[string]any{"default_agent": "sdd-orchestrator", "profile": true})
	writeFileMode(t, settings, settingsRaw, 0o600)
	writeFileMode(t, ownerPath, encode(newOwnership(fieldValue{present: true, value: "build"})), 0o640)

	plan, err := PrepareUninstall(settings)
	check(t, err)
	changed, removed, err := plan.Apply(settingsRaw, true)
	check(t, err)
	if !changed || removed {
		t.Fatalf("uninstall result = changed %t, removed %t", changed, removed)
	}
	wantSettings := encode(map[string]any{"default_agent": "build", "profile": true})
	requireFile(t, settings, wantSettings, 0o600)
	requireAbsent(t, ownerPath)
}

func TestPairWriteFailureRollsBackAndFreshInstallRetries(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "opencode.json")
	ownerPath := OwnershipPath(settings)
	settingsBefore := []byte("{\"default_agent\":\"build\",\"profile\":true}\n")
	settingsAfter := encode(map[string]any{"default_agent": model.CanonicalManagedIdentity.Actor, "profile": true})
	ownerAfter := encode(newOwnership(fieldValue{present: true, value: "build"}))
	writeFileMode(t, settings, settingsBefore, 0o600)

	wantErr := errors.New("fail after second pair write")
	err := applyPair(mutationjournal.New(dir), []pairMutation{
		{path: settings, data: settingsAfter, keep: true, context: "update OpenCode settings"},
		{path: ownerPath, data: ownerAfter, keep: true, context: "update OpenCode default ownership"},
	}, failAfterSecondPairMutation(wantErr))
	if !errors.Is(err, wantErr) {
		t.Fatalf("pair write error = %v, want %v", err, wantErr)
	}
	requireFile(t, settings, settingsBefore, 0o600)
	requireAbsent(t, ownerPath)

	plan, err := PrepareInstall(settings)
	check(t, err)
	changed, err := plan.Apply()
	check(t, err)
	if !changed {
		t.Fatal("fresh install retry reported no change")
	}
	requireFile(t, settings, settingsAfter, 0o600)
	requireFile(t, ownerPath, ownerAfter, 0o644)
}

func TestPairRemoveFailureRollsBackAndFreshUninstallRetries(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "opencode.json")
	ownerPath := OwnershipPath(settings)
	settingsBefore := encode(map[string]any{"default_agent": model.CanonicalManagedIdentity.Actor})
	ownerBefore := encode(newOwnership(fieldValue{}))
	writeFileMode(t, settings, settingsBefore, 0o600)
	writeFileMode(t, ownerPath, ownerBefore, 0o640)

	wantErr := errors.New("fail after second pair remove")
	err := applyPair(mutationjournal.New(dir), []pairMutation{
		{path: settings, context: "update OpenCode settings"},
		{path: ownerPath, context: "update OpenCode default ownership"},
	}, failAfterSecondPairMutation(wantErr))
	if !errors.Is(err, wantErr) {
		t.Fatalf("pair remove error = %v, want %v", err, wantErr)
	}
	requireFile(t, settings, settingsBefore, 0o600)
	requireFile(t, ownerPath, ownerBefore, 0o640)

	plan, err := PrepareUninstall(settings)
	check(t, err)
	changed, removed, err := plan.Apply(nil, false)
	check(t, err)
	if !changed || !removed {
		t.Fatalf("fresh uninstall retry = changed %t, removed %t", changed, removed)
	}
	requireAbsent(t, settings)
	requireAbsent(t, ownerPath)
}

func TestUninstallWithoutOwnershipHandlesDefaultAgent(t *testing.T) {
	for _, tt := range []struct {
		name         string
		defaultAgent string
		wantPresent  bool
	}{
		{name: "managed default is removed", defaultAgent: "shevanio-orchestrator"},
		{name: "legacy gentle default is preserved", defaultAgent: "gentle-orchestrator", wantPresent: true},
		{name: "legacy sdd default is preserved", defaultAgent: "sdd-orchestrator", wantPresent: true},
		{name: "user default is preserved", defaultAgent: "build", wantPresent: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			settings := filepath.Join(t.TempDir(), "opencode.json")
			original := `{"default_agent":"` + tt.defaultAgent + `","agent":{"shevanio-orchestrator":{}},"profile":true}`
			writeFileMode(t, settings, []byte(original), 0o600)
			plan, err := PrepareUninstall(settings)
			check(t, err)

			cleaned := []byte(`{"default_agent":"` + tt.defaultAgent + `","profile":true}`)
			_, _, err = plan.Apply(cleaned, true)
			check(t, err)

			want := map[string]any{"profile": true}
			if tt.wantPresent {
				want["default_agent"] = tt.defaultAgent
			}
			requireFile(t, settings, encode(want), 0o600)
		})
	}
}
