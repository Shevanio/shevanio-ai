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
	home := t.TempDir()
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
	home := t.TempDir()
	must(t, state.Write(home, state.InstallState{Components: []model.ComponentID{legacyGGAComponentID, "gga-extra", "unknown", legacyGGAComponentID}}))
	writeOwnedGGAFiles(t, home)
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

func migrationHome(t *testing.T) string {
	home := t.TempDir()
	must(t, state.Write(home, state.InstallState{Components: []model.ComponentID{legacyGGAComponentID, "unknown"}}))
	writeOwnedGGAFiles(t, home)
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
