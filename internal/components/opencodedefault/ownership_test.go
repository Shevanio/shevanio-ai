package opencodedefault

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shevanio/shevanio-ai/v2/internal/components/filemerge"
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

func TestV1OwnershipUpgradesWithoutAdoptingActors(t *testing.T) {
	settings := filepath.Join(t.TempDir(), "opencode.json")
	check(t, os.WriteFile(settings, []byte(`{"default_agent":"shevanio-orchestrator"}`), 0o644))
	check(t, os.WriteFile(OwnershipPath(settings), []byte(`{"schema":"shevanio-ai.opencode-default-agent","version":1,"state":"managed","previous_state":"value","previous_default":"build"}`), 0o644))
	plan, err := PrepareInstall(settings)
	check(t, err)
	if len(plan.owned.Actors) != 0 || plan.owned.PreviousDefault != "build" {
		t.Fatalf("v1 upgrade = %#v, want previous default and zero actors", plan.owned)
	}
	_, err = plan.Apply()
	check(t, err)
	upgraded, err := readOwnership(OwnershipPath(settings))
	check(t, err)
	if upgraded.Version != 2 || upgraded.PreviousDefault != "build" || len(upgraded.Actors) != 0 {
		t.Fatalf("persisted upgrade = %#v", upgraded)
	}
}

func TestV2OwnershipStrictValidation(t *testing.T) {
	valid := encode(&ownership{Schema: schema, Version: version, State: "managed", PreviousState: "absent", Actors: map[string]actorOwnership{
		"shevanio-orchestrator": {Scope: "base", Fingerprint: "sha256:" + strings.Repeat("a", 64)},
		"sdd-apply-cheap":       {Scope: "profile/cheap", Fingerprint: "sha256:" + strings.Repeat("b", 64)},
	}})
	path := filepath.Join(t.TempDir(), "owner.json")
	check(t, os.WriteFile(path, valid, 0o644))
	got, err := readOwnership(path)
	check(t, err)
	if roundTrip := encode(got); !bytes.Equal(roundTrip, valid) {
		t.Fatalf("v2 round trip = %s, want %s", roundTrip, valid)
	}
	for _, tt := range []struct{ name, old, replacement string }{
		{"unknown version", `"version": 2`, `"version": 3`},
		{"invalid scope", `"scope": "base"`, `"scope": "profile/Bad"`},
		{"invalid fingerprint", `sha256:` + strings.Repeat("a", 64), `sha256:short`},
		{"unknown field", `"actors":`, `"unexpected":true,"actors":`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			check(t, os.WriteFile(path, bytes.Replace(valid, []byte(tt.old), []byte(tt.replacement), 1), 0o644))
			if _, err := readOwnership(path); err == nil {
				t.Fatal("invalid v2 ownership was accepted")
			}
		})
	}
	check(t, os.WriteFile(path, append(valid, []byte(`{}`)...), 0o644))
	if _, err := readOwnership(path); err == nil {
		t.Fatal("trailing JSON was accepted")
	}
}

func TestOwnershipInputRefusals(t *testing.T) {
	dir := t.TempDir()
	t.Run("directory", func(t *testing.T) {
		if _, err := readOwnership(dir); err == nil {
			t.Fatal("directory ownership input was accepted")
		}
	})
	t.Run("symlink", func(t *testing.T) {
		target, link := filepath.Join(dir, "target"), filepath.Join(dir, "link")
		check(t, os.WriteFile(target, encode(newOwnership(fieldValue{})), 0o644))
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if _, err := readOwnership(link); err == nil {
			t.Fatal("symlink ownership input was accepted")
		}
	})
	t.Run("oversized", func(t *testing.T) {
		path := filepath.Join(dir, "oversized")
		check(t, os.WriteFile(path, bytes.Repeat([]byte("x"), maxOwnershipSize+1), 0o644))
		if _, err := readOwnership(path); err == nil {
			t.Fatal("oversized ownership input was accepted")
		}
	})
}

func TestObserveOverlayRecordsOnlyExactCurrentWrites(t *testing.T) {
	before := []byte(`{"agent":{"sdd-init":{"mode":"subagent"},"sdd-verify":{"mode":"subagent","user":true}}}`)
	overlay := []byte(`{"agent":{"shevanio-orchestrator":{"mode":"primary","tools":{"__replace__":{"read":true}}},"sdd-init":{"mode":"subagent"},"sdd-verify":{"mode":"subagent"}}}`)
	merged, err := filemerge.MergeJSONObjects(before, overlay)
	check(t, err)
	plan := &InstallPlan{}
	check(t, plan.ObserveOverlay("base", before, overlay, merged))
	if len(plan.observed) != 1 {
		t.Fatalf("base observations = %#v, want only the created actor", plan.observed)
	}
	wantFingerprint := fingerprint(t, map[string]any{"mode": "primary", "tools": map[string]any{"read": true}})
	if got := plan.observed["shevanio-orchestrator"]; got.Scope != "base" || got.Fingerprint != wantFingerprint {
		t.Fatalf("base ownership = %#v, want base/%s", got, wantFingerprint)
	}
	profileOverlay := []byte(`{"agent":{"shevanio-orchestrator-cheap":{"mode":"primary"}}}`)
	profileMerged, err := filemerge.MergeJSONObjects(merged, profileOverlay)
	check(t, err)
	check(t, plan.ObserveOverlay("profile/cheap", merged, profileOverlay, profileMerged))
	if got := plan.observed["shevanio-orchestrator-cheap"]; got.Scope != "profile/cheap" || got.Fingerprint != fingerprint(t, map[string]any{"mode": "primary"}) {
		t.Fatalf("profile ownership = %#v", got)
	}
}

func fingerprint(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	check(t, err)
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("sha256:%x", sum)
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
