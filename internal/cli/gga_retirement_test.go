package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shevanio/shevanio-ai/v2/internal/backup"
	"github.com/shevanio/shevanio-ai/v2/internal/catalog"
	"github.com/shevanio/shevanio-ai/v2/internal/model"
	"github.com/shevanio/shevanio-ai/v2/internal/planner"
	"github.com/shevanio/shevanio-ai/v2/internal/reviewtransaction"
	"github.com/shevanio/shevanio-ai/v2/internal/state"
	"github.com/shevanio/shevanio-ai/v2/internal/statestore"
	"github.com/shevanio/shevanio-ai/v2/internal/update"
)

func TestRetiredGGAIsAbsentFromActiveSurfaces(t *testing.T) {
	for _, component := range catalog.MVPComponents() {
		if component.ID == legacyGGAComponentID {
			t.Fatalf("active catalog still exposes retired component: %#v", component)
		}
	}
	for _, preset := range []model.PresetID{model.PresetFullGentleman, model.PresetEcosystemOnly} {
		for _, component := range model.ComponentsForPreset(preset, model.PersonaGentleman) {
			if component == legacyGGAComponentID {
				t.Fatalf("preset %q still exposes retired component", preset)
			}
		}
	}
	if planner.MVPGraph().Has(legacyGGAComponentID) {
		t.Fatal("planner still exposes retired component")
	}
	selection := BuildSyncSelection(SyncFlags{}, []model.AgentID{model.AgentOpenCode})
	for _, component := range selection.Components {
		if component == legacyGGAComponentID {
			t.Fatal("sync selection still exposes retired component")
		}
	}
	for _, tool := range requiredDoctorTools(nil) {
		if tool == string(legacyGGAComponentID) {
			t.Fatal("doctor still checks retired GGA binary")
		}
	}
	for _, tool := range update.Tools {
		if tool.Name == string(legacyGGAComponentID) {
			t.Fatal("update registry still exposes retired GGA tool")
		}
	}
}

func TestRunInstallRejectsRetiredGGAWithoutCommandsOrStateMutation(t *testing.T) {
	home := ggaTestHome(t)
	must(t, state.Write(home, state.InstallState{Components: []model.ComponentID{"unknown"}}))

	originalHome := osUserHomeDir
	originalCommand := runCommand
	t.Cleanup(func() {
		osUserHomeDir = originalHome
		runCommand = originalCommand
	})
	osUserHomeDir = func() (string, error) { return home, nil }
	var commands []string
	runCommand = func(name string, args ...string) error {
		commands = append(commands, name+" "+strings.Join(args, " "))
		return nil
	}

	_, err := RunInstall([]string{"--agent", "opencode", "--component", string(legacyGGAComponentID)}, macOSDetectionResult())
	if err == nil || err.Error() != `unsupported component "gga"` {
		t.Fatalf("RunInstall() error = %v, want retired-component error", err)
	}
	if len(commands) != 0 {
		t.Fatalf("retired component triggered commands: %v", commands)
	}
	got, readErr := state.Read(home)
	if readErr != nil || len(got.Components) != 1 || got.Components[0] != "unknown" {
		t.Fatalf("retired component mutated state: %#v, err = %v", got, readErr)
	}
}

func TestMigrateLegacyGGASuccessBacksUpCleansParentsAndStaysDormant(t *testing.T) {
	home := ggaTestHome(t)
	must(t, state.Write(home, state.InstallState{Components: []model.ComponentID{legacyGGAComponentID, "gga-extra", "unknown", legacyGGAComponentID}}))
	writeOwnedGGAFiles(t, home)
	must(t, os.MkdirAll(filepath.Join(legacyGGARootDir(home), "lib"), 0o755))
	if result, err := MigrateLegacyGGA(home, true); err != nil || result.Changed {
		t.Fatalf("registered migration = %#v, err = %v", result, err)
	}
	requirePath(t, legacyGGAConfigPath(home), "")
	result, err := MigrateLegacyGGA(home, false)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.BackupID == "" || len(result.PreservedPaths) != 0 {
		t.Fatalf("migration result = %#v", result)
	}
	got, err := state.Read(home)
	if err != nil || len(got.Components) != 2 || got.Components[0] != "gga-extra" || got.Components[1] != "unknown" {
		t.Fatalf("state = %#v, err = %v", got, err)
	}
	requirePath(t, legacyGGAConfigPath(home), "absent")
	requirePath(t, legacyGGARootDir(home), "absent")
	backupDir := filepath.Join(home, ".shevanio-ai", "backups", result.BackupID)
	manifest, err := backup.ReadManifest(filepath.Join(backupDir, backup.ManifestFilename))
	if err != nil || manifest.FileCount != 1 || !manifest.Compressed {
		t.Fatalf("backup manifest = %#v, err = %v", manifest, err)
	}
	requirePath(t, filepath.Join(backupDir, backup.ArchiveFilename), "")
}

func TestMigrateLegacyGGAPreservesUnownedCandidatesAndExternalState(t *testing.T) {
	home := migrationHome(t)
	managed := legacyGGAManagedFiles(home)
	configPath := managed[0].path
	mustWrite(t, configPath, []byte("user edited"), 0o644)
	target := filepath.Join(home, "external-pr-mode.sh")
	mustWrite(t, target, []byte("external"), 0o755)
	must(t, os.MkdirAll(filepath.Dir(managed[2].path), 0o755))
	must(t, os.Symlink(target, managed[2].path))
	must(t, os.MkdirAll(managed[4].path, 0o755))
	protected := map[string][]byte{
		filepath.Join(home, "bin", "external-tool"):                                            []byte("binary"),
		filepath.Join(home, ".local", "share", "homebrew", "Cellar", "gga", "1", "bin", "gga"): []byte("package"),
		filepath.Join(home, ".brew", "taps", "vendor", "tap"):                                  []byte("tap"),
		filepath.Join(home, ".profile"):                                                        []byte("PATH=$HOME/bin:$PATH\n"),
		filepath.Join(home, "repo", ".git", "hooks", "pre-commit"):                             []byte("hook"),
		filepath.Join(home, ".config", "gga", "user-notes"):                                    []byte("unknown"),
		target: []byte("external"),
	}
	writeFiles(t, protected)
	first, err := MigrateLegacyGGA(home, false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := MigrateLegacyGGA(home, false)
	if err != nil || second.Changed || second.BackupID != "" {
		t.Fatalf("repeat migration = %#v, err = %v", second, err)
	}
	if len(first.PreservedPaths) != 3 {
		t.Fatalf("preserved paths = %#v", first.PreservedPaths)
	}
	for path, want := range protected {
		requirePath(t, path, string(want))
	}
	requirePath(t, configPath, "user edited")
	requirePath(t, managed[2].path, "symlink")
	requirePath(t, managed[4].path, "dir")
}

func TestMigrateLegacyGGAFailuresLeaveStateAndFilesUnchanged(t *testing.T) {
	cases := []struct {
		name  string
		lock  bool
		setup func(*testing.T, string)
		retry bool
	}{
		{"corrupt state", false, func(t *testing.T, home string) { mustWrite(t, state.Path(home), []byte("{"), 0o644) }, false},
		{"backup failure", false, func(t *testing.T, home string) {
			mustWrite(t, filepath.Join(home, ".shevanio-ai", "backups"), []byte("file"), 0o644)
		}, false},
		{"lock failure", true, nil, false},
		{"persistence retry", false, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := migrationHome(t)
			if tc.setup != nil {
				tc.setup(t, home)
			}
			before, err := os.ReadFile(state.Path(home))
			if err != nil {
				t.Fatal(err)
			}
			if tc.lock {
				held, lockErr := reviewtransaction.AcquireAuthorityFileLock(state.Path(home) + ".lock")
				if lockErr != nil {
					t.Fatal(lockErr)
				}
				defer held.Release()
			}
			original, calls := ggaLeaseCommitFn, 0
			if tc.retry {
				ggaLeaseCommitFn = func(lease *statestore.Lease, persisted state.InstallState) (statestore.Result, error) {
					calls++
					if calls == 1 {
						return statestore.Result{Outcome: statestore.Uncommitted}, errors.New("forced persistence failure")
					}
					return lease.Commit(persisted)
				}
				defer func() { ggaLeaseCommitFn = original }()
			}
			if _, err := MigrateLegacyGGA(home, false); err == nil {
				t.Fatal("migration unexpectedly succeeded")
			}
			if tc.retry {
				failed, err := state.Read(home)
				if err != nil || len(failed.Components) != 2 || failed.Components[0] != legacyGGAComponentID {
					t.Fatalf("state after failure = %#v, err = %v", failed, err)
				}
				result, err := MigrateLegacyGGA(home, false)
				if err != nil || !result.Changed || calls != 2 {
					t.Fatalf("retry result = %#v, err = %v, calls = %d", result, err, calls)
				}
				return
			}
			after, err := os.ReadFile(state.Path(home))
			if err != nil || string(after) != string(before) {
				t.Fatalf("state changed: %v", err)
			}
			requirePath(t, legacyGGAConfigPath(home), "")
		})
	}
}

func TestMigrateLegacyGGAPreservesPartialPathsOnRemovalError(t *testing.T) {
	home := migrationHome(t)
	original := ggaRemoveOwnedFilesFn
	t.Cleanup(func() { ggaRemoveOwnedFilesFn = original })
	sentinel := errors.New("sentinel removal failure")
	preservedPath := filepath.Join(home, ".config", "gga", "user-owned")
	ggaRemoveOwnedFilesFn = func([]ggaManagedFile) ([]string, error) {
		return []string{preservedPath}, sentinel
	}

	result, err := MigrateLegacyGGA(home, false)
	if !errors.Is(err, sentinel) {
		t.Fatalf("behavior mismatch: TestMigrateLegacyGGAPreservesPartialPathsOnRemovalError: error = %v", err)
	}
	if len(result.PreservedPaths) != 1 || result.PreservedPaths[0] != preservedPath {
		t.Fatalf("behavior mismatch: TestMigrateLegacyGGAPreservesPartialPathsOnRemovalError: preserved paths = %#v", result.PreservedPaths)
	}
}

func TestGGARetirementDurableArchiveManifest(t *testing.T) {
	home := migrationHome(t)
	var synced []string
	originalSync, originalRemove, originalDurability := ggaSyncFn, ggaRemoveOwnedFilesFn, ggaRetirementDurabilityFn
	t.Cleanup(func() {
		ggaSyncFn, ggaRemoveOwnedFilesFn, ggaRetirementDurabilityFn = originalSync, originalRemove, originalDurability
	})
	ggaSyncFn = func(path string, _ bool) error { synced = append(synced, path); return nil }
	ggaRemoveOwnedFilesFn = func(files []ggaManagedFile) ([]string, error) {
		if len(synced) != 4 || filepath.Base(synced[0]) != backup.ArchiveFilename || filepath.Base(synced[1]) != backup.ManifestFilename || synced[2] != filepath.Dir(synced[0]) || synced[3] != filepath.Dir(synced[2]) {
			return nil, errors.New("backup durability was incomplete before removal")
		}
		return removeOwnedGGAFiles(files)
	}
	if result, err := MigrateLegacyGGA(home, false); err != nil || result.Outcome != statestore.Committed {
		t.Fatalf("retirement result = %#v, err = %v", result, err)
	}
	sentinel := errors.New("archive durability failure")
	ggaRetirementDurabilityFn = func(string, []ggaManagedFile, backup.Manifest) error { return sentinel }
	result, err := MigrateLegacyGGA(migrationHome(t), false)
	if err == nil || !errors.Is(err, sentinel) || result.Outcome != statestore.Uncommitted || result.Changed {
		t.Fatalf("behavior mismatch: TestGGARetirementDurableArchiveManifest: %#v %v", result, err)
	}
	semanticHome := migrationHome(t)
	ggaRetirementDurabilityFn = func(homeDir string, files []ggaManagedFile, manifest backup.Manifest) error {
		if err := originalDurability(homeDir, files, manifest); err != nil {
			return err
		}
		manifest.Checksum = strings.Repeat("0", 64)
		return backup.WriteManifest(filepath.Join(manifest.RootDir, backup.ManifestFilename), manifest)
	}
	result, err = MigrateLegacyGGA(semanticHome, false)
	if err == nil || result.Outcome != statestore.Unknown || result.Changed {
		t.Fatalf("behavior mismatch: TestGGARetirementDurableArchiveManifest: %#v %v", result, err)
	}
	requirePath(t, legacyGGAConfigPath(semanticHome), string(legacyGGAConfig()))
}
func TestGGARetirementCompensationAndExactRestore(t *testing.T) {
	home := migrationHome(t)
	path := legacyGGAConfigPath(home)
	must(t, os.Chmod(filepath.Dir(path), 0o700))
	primary := errors.New("state publication failure")
	setGGARecoveryFailure(t, primary, nil)
	result, err := MigrateLegacyGGA(home, false)
	if err == nil || !errors.Is(err, primary) || result.Outcome != statestore.Unknown {
		t.Fatalf("behavior mismatch: %s: %#v %v", t.Name(), result, err)
	}
	requirePath(t, path, "absent")
}
func TestGGARetirementUnknownRetainsEvidence(t *testing.T) {
	home := migrationHome(t)
	root := filepath.Join(home, ".shevanio-ai", "backups")
	candidate := filepath.Join(root, "retire-gga-99999999999999.000000000")
	must(t, os.MkdirAll(candidate, 0o755))
	manifestPath := filepath.Join(candidate, backup.ManifestFilename)
	mustWrite(t, manifestPath, []byte("{"), 0o644)
	before, err := os.ReadFile(state.Path(home))
	if err != nil {
		t.Fatal(err)
	}
	result, err := MigrateLegacyGGA(home, false)
	var recoveryErr *GGARetirementRecoveryError
	wantPaths := []string{candidate, filepath.Join(candidate, backup.ArchiveFilename), manifestPath}
	if err == nil || !errors.As(err, &recoveryErr) || result.Outcome != statestore.Unknown || strings.Join(result.RecoveryPaths, "\x00") != strings.Join(wantPaths, "\x00") {
		t.Fatalf("behavior mismatch: TestGGARetirementUnknownRetainsEvidence: %#v %v", result, err)
	}
	after, readErr := os.ReadFile(state.Path(home))
	if readErr != nil || string(after) != string(before) {
		t.Fatalf("behavior mismatch: TestGGARetirementUnknownRetainsEvidence: state changed: %v", readErr)
	}
	requirePath(t, legacyGGAConfigPath(home), string(legacyGGAConfig()))
}
func TestGGARetirementCombinedResultAndRecoveryError(t *testing.T) {
	primary, recovery := errors.New("primary retirement failure"), errors.New("recovery failure")
	setGGARecoveryFailure(t, primary, recovery)
	result, err := MigrateLegacyGGA(migrationHome(t), false)
	var recoveryErr *GGARetirementRecoveryError
	if err == nil || !errors.As(err, &recoveryErr) || !errors.Is(err, primary) || !errors.Is(err, recovery) || result.Outcome != statestore.Unknown || recoveryErr.Outcome != statestore.Unknown || len(result.RecoveryPaths) == 0 {
		t.Fatalf("behavior mismatch: %s: result = %#v, err = %v", t.Name(), result, err)
	}
	if _, e := os.Stat(result.RecoveryPaths[len(result.RecoveryPaths)-1]); e != nil {
		t.Fail()
	}
}
func TestGGARetirementRetryFromEvidence(t *testing.T) {
	home := migrationHome(t)
	primary, recovery := errors.New("first publication failure"), errors.New("first restoration failure")
	originalCommit, originalRestore, originalRemove := ggaLeaseCommitFn, ggaRestoreServiceFn, ggaRemoveOwnedFilesFn
	t.Cleanup(func() {
		ggaLeaseCommitFn, ggaRestoreServiceFn, ggaRemoveOwnedFilesFn = originalCommit, originalRestore, originalRemove
	})
	removeCalls := 0
	ggaRemoveOwnedFilesFn = func(files []ggaManagedFile) ([]string, error) { removeCalls++; return removeOwnedGGAFiles(files) }
	firstAttempt := true
	ggaLeaseCommitFn = func(lease *statestore.Lease, persisted state.InstallState) (statestore.Result, error) {
		if firstAttempt {
			firstAttempt = false
			return statestore.Result{Outcome: statestore.Uncommitted}, primary
		}
		return lease.Commit(persisted)
	}
	ggaRestoreServiceFn = func(string, backup.Manifest) error { return recovery }
	first, firstErr := MigrateLegacyGGA(home, false)
	if firstErr == nil || first.Outcome != statestore.Unknown || len(first.RecoveryPaths) == 0 {
		t.Fatalf("behavior mismatch: %s: first result = %#v, err = %v", t.Name(), first, firstErr)
	}
	second, secondErr := MigrateLegacyGGA(home, false)
	if secondErr != nil || second.Outcome != statestore.Committed || !second.Changed || removeCalls != 1 {
		t.Fatalf("behavior mismatch: %s: second result = %#v, err = %v, removeCalls = %d", t.Name(), second, secondErr, removeCalls)
	}
}
func setGGARecoveryFailure(t *testing.T, primary, recovery error) {
	originalCommit, originalRestore := ggaLeaseCommitFn, ggaRestoreServiceFn
	t.Cleanup(func() { ggaLeaseCommitFn, ggaRestoreServiceFn = originalCommit, originalRestore })
	ggaLeaseCommitFn = func(*statestore.Lease, state.InstallState) (statestore.Result, error) {
		return statestore.Result{Outcome: statestore.Uncommitted}, primary
	}
	ggaRestoreServiceFn = func(string, backup.Manifest) error { return recovery }
}

func migrationHome(t *testing.T) string {
	home := ggaTestHome(t)
	must(t, state.Write(home, state.InstallState{Components: []model.ComponentID{legacyGGAComponentID, "unknown"}}))
	writeOwnedGGAFiles(t, home)
	return home
}

func ggaTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	return home
}

func writeOwnedGGAFiles(t *testing.T, home string) {
	writeFiles(t, map[string][]byte{legacyGGAConfigPath(home): legacyGGAConfig()})
}

func legacyGGAConfig() []byte {
	return []byte("# Gentleman Guardian Angel Configuration\n# Generated by shevanio-ai\n\n# AI Provider for code review\n# Options: claude, gemini, codex, opencode, ollama:<model>\nPROVIDER=\"claude\"\n\n# File patterns to review (comma-separated globs)\nFILE_PATTERNS=\"*.ts,*.tsx,*.js,*.jsx,*.py,*.go\"\n\n# Patterns to exclude\nEXCLUDE_PATTERNS=\"*.test.*,*.spec.*,*.d.ts,dist/*,build/*,node_modules/*\"\n\n# Rules file\nRULES_FILE=\"AGENTS.md\"\n\n# Strict mode (fail on ambiguous AI responses)\nSTRICT_MODE=\"true\"\n\n# Review timeout in seconds\nTIMEOUT=\"300\"\n")
}

func writeFiles(t *testing.T, files map[string][]byte) {
	for path, content := range files {
		mustWrite(t, path, content, 0o755)
	}
}

func mustWrite(t *testing.T, path string, content []byte, mode os.FileMode) {
	must(t, os.MkdirAll(filepath.Dir(path), 0o755))
	must(t, os.WriteFile(path, content, mode))
}

func must(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func requirePath(t *testing.T, path, want string) {
	info, err := os.Lstat(path)
	switch {
	case want == "absent":
		if !errors.Is(err, os.ErrNotExist) {
			t.Errorf("path %q exists: %v", path, err)
		}
	case err != nil:
		t.Errorf("path %q missing: %v", path, err)
	case want == "dir" && !info.IsDir() || want == "symlink" && info.Mode()&os.ModeSymlink == 0:
		t.Errorf("path %q has wrong type", path)
	case want != "" && want != "dir" && want != "symlink":
		got, readErr := os.ReadFile(path)
		if readErr != nil || string(got) != want {
			t.Errorf("path %q = %q, err = %v", path, got, readErr)
		}
	}
}
