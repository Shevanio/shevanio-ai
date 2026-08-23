package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/shevanio/shevanio-ai/v2/internal/assets"
	"github.com/shevanio/shevanio-ai/v2/internal/backup"
	"github.com/shevanio/shevanio-ai/v2/internal/components/gga"
	"github.com/shevanio/shevanio-ai/v2/internal/model"
	"github.com/shevanio/shevanio-ai/v2/internal/reviewtransaction"
	"github.com/shevanio/shevanio-ai/v2/internal/state"
)

func TestMigrateLegacyGGASuccessBacksUpCleansParentsAndStaysDormant(t *testing.T) {
	home := t.TempDir()
	must(t, state.Write(home, state.InstallState{Components: []model.ComponentID{model.ComponentGGA, "gga-extra", "unknown", model.ComponentGGA}}))
	writeOwnedGGAFiles(t, home)
	if result, err := MigrateLegacyGGA(home, true); err != nil || result.Changed {
		t.Fatalf("registered migration = %#v, err = %v", result, err)
	}
	requirePath(t, gga.ConfigPath(home), "")
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
	for _, path := range []string{gga.ConfigPath(home), gga.AgentsTemplatePath(home), gga.RuntimePRModePath(home), gga.RuntimePS1Path(home), gga.RuntimeCMDPath(home)} {
		requirePath(t, path, "absent")
	}
	for _, path := range []string{filepath.Join(home, ".config", "gga"), filepath.Join(home, ".local", "share", "gga", "lib"), filepath.Join(home, ".local", "share", "gga")} {
		requirePath(t, path, "absent")
	}
	backupDir := filepath.Join(home, ".shevanio-ai", "backups", result.BackupID)
	manifest, err := backup.ReadManifest(filepath.Join(backupDir, backup.ManifestFilename))
	if err != nil || manifest.FileCount != 5 || !manifest.Compressed {
		t.Fatalf("backup manifest = %#v, err = %v", manifest, err)
	}
	requirePath(t, filepath.Join(backupDir, backup.ArchiveFilename), "")
}

func TestMigrateLegacyGGAPreservesUnownedCandidatesAndExternalState(t *testing.T) {
	home := migrationHome(t)
	configPath := gga.ConfigPath(home)
	mustWrite(t, configPath, []byte("user edited"), 0o644)
	must(t, os.Remove(gga.RuntimePRModePath(home)))
	target := filepath.Join(home, "external-pr-mode.sh")
	mustWrite(t, target, []byte("external"), 0o755)
	must(t, os.Symlink(target, gga.RuntimePRModePath(home)))
	must(t, os.Remove(gga.RuntimeCMDPath(home)))
	must(t, os.MkdirAll(gga.RuntimeCMDPath(home), 0o755))
	protected := map[string][]byte{
		filepath.Join(home, "bin", "external-tool"): []byte("binary"), filepath.Join(home, ".local", "share", "homebrew", "Cellar", "gga", "1", "bin", "gga"): []byte("package"),
		filepath.Join(home, ".brew", "taps", "vendor", "tap"): []byte("tap"), filepath.Join(home, ".profile"): []byte("PATH=$HOME/bin:$PATH\n"),
		filepath.Join(home, "repo", ".git", "hooks", "pre-commit"): []byte("hook"), filepath.Join(home, ".config", "gga", "user-notes"): []byte("unknown"), target: []byte("external"),
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
	requirePath(t, gga.RuntimePRModePath(home), "symlink")
	requirePath(t, gga.RuntimeCMDPath(home), "dir")
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
			var held *reviewtransaction.AuthorityFileLock
			if tc.lock {
				held, err = reviewtransaction.AcquireAuthorityFileLock(installStateLockPath(home))
				if err != nil {
					t.Fatal(err)
				}
				defer held.Release()
			}
			original, calls := ggaStateWriteFn, 0
			if tc.retry {
				ggaStateWriteFn = func(home string, persisted state.InstallState) error {
					calls++
					if calls == 1 {
						return errors.New("forced persistence failure")
					}
					return state.WriteReconciled(home, persisted)
				}
				defer func() { ggaStateWriteFn = original }()
			}
			if _, err := MigrateLegacyGGA(home, false); err == nil {
				t.Fatal("migration unexpectedly succeeded")
			}
			if tc.retry {
				failed, err := state.Read(home)
				if err != nil || len(failed.Components) != 2 || failed.Components[0] != model.ComponentGGA {
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
			requirePath(t, gga.ConfigPath(home), "")
		})
	}
}

func migrationHome(t *testing.T) string {
	home := t.TempDir()
	must(t, state.Write(home, state.InstallState{Components: []model.ComponentID{model.ComponentGGA, "unknown"}}))
	writeOwnedGGAFiles(t, home)
	return home
}
func writeOwnedGGAFiles(t *testing.T, home string) {
	writeFiles(t, map[string][]byte{gga.ConfigPath(home): gga.BuildConfig("claude"), gga.AgentsTemplatePath(home): readAsset(t, "gga/AGENTS.md"), gga.RuntimePRModePath(home): readAsset(t, "gga/pr_mode.sh"), gga.RuntimePS1Path(home): readAsset(t, "gga/gga.ps1"), gga.RuntimeCMDPath(home): readAsset(t, "gga/gga.cmd")})
}
func writeFiles(t *testing.T, files map[string][]byte) {
	for path, content := range files {
		mustWrite(t, path, content, 0o755)
	}
}
func readAsset(t *testing.T, name string) []byte {
	content, err := assets.Read(name)
	if err != nil {
		t.Fatal(err)
	}
	return []byte(content)
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
