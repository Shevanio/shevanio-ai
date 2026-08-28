package cli

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/shevanio/shevanio-ai/v2/internal/backup"
	"github.com/shevanio/shevanio-ai/v2/internal/model"
	"github.com/shevanio/shevanio-ai/v2/internal/state"
	"github.com/shevanio/shevanio-ai/v2/internal/statestore"
)

const legacyGGAComponentID model.ComponentID = "gga"

type GGARetirementResult struct {
	Changed                       bool
	Outcome                       statestore.Outcome
	BackupID                      string
	PreservedPaths, RecoveryPaths []string
}
type GGARetirementRecoveryError struct {
	Outcome       statestore.Outcome
	RecoveryPaths []string
	Err           error
}

func (err *GGARetirementRecoveryError) Error() string {
	return fmt.Sprintf("GGA retirement recovery: %v", err.Err)
}
func (err *GGARetirementRecoveryError) Unwrap() error { return err.Err }

type ggaRecoveryEvidence struct {
	backupDir string
	manifest  backup.Manifest
	dirs      map[string]os.FileMode
}

var (
	ggaRetirementNow         = time.Now
	ggaRetirementSnapshotter = func() backup.Snapshotter { return backup.NewSnapshotter() }
	ggaLeaseCommitFn         = func(lease *statestore.Lease, next state.InstallState) (statestore.Result, error) {
		return lease.Commit(next)
	}
	ggaRemoveOwnedFilesFn     = removeOwnedGGAFiles
	ggaRetirementDurabilityFn = durableGGABackup
	ggaRetirementRestoreFn    = restoreGGASnapshot
)

func MigrateLegacyGGA(homeDir string, ggaRegistered bool) (result GGARetirementResult, err error) {
	if ggaRegistered {
		return GGARetirementResult{}, nil
	}
	lease, err := statestore.Begin(homeDir)
	if err != nil {
		return result, fmt.Errorf("acquire install state lock: %w", err)
	}
	defer func() {
		if releaseErr := lease.Release(); releaseErr != nil {
			err = errors.Join(err, fmt.Errorf("release install state lock: %w", releaseErr))
		}
	}()
	persisted, err := lease.Current()
	if err != nil {
		return result, fmt.Errorf("read install state: %w", err)
	}
	filtered := filterLegacyGGAComponents(persisted.Components)
	evidence, ok := loadGGARetirementEvidence(homeDir)
	if ok && ggaRetired(evidence) {
		result.BackupID, result.RecoveryPaths = evidence.manifest.ID, evidence.paths()
		if len(filtered) == len(persisted.Components) {
			result.Outcome = statestore.Committed
			return result, nil
		}
		next := persisted
		next.Components = filtered
		committed, commitErr := ggaLeaseCommitFn(lease, next)
		if committed.Outcome == statestore.Committed {
			result.Changed, result.Outcome = true, statestore.Committed
			return result, commitErr
		}
		if commitErr != nil {
			return result, recoverGGA(&result, homeDir, evidence, commitErr)
		}
		result.Outcome = statestore.Unknown
		// refusal:by-design operator-knowledge: an absent commit outcome cannot be classified as success.
		return result, errors.New("GGA retirement state publication was not committed")
	}
	if len(filtered) == len(persisted.Components) {
		return result, nil
	}
	owned, preserved, err := findOwnedGGAFiles(homeDir)
	if err != nil {
		return result, err
	}
	result.PreservedPaths = preserved
	if len(owned) > 0 {
		evidence.dirs = map[string]os.FileMode{}
		for _, dir := range ggaDirs(homeDir) {
			if info, err := os.Stat(dir); err == nil {
				evidence.dirs[dir] = info.Mode().Perm()
			}
		}
		manifest, err := backupLegacyGGAFiles(homeDir, owned)
		if err != nil {
			result.Outcome = statestore.Uncommitted
			return result, err
		}
		evidence.backupDir, evidence.manifest = manifest.RootDir, manifest
		result.BackupID = manifest.ID
		preserved, err := ggaRemoveOwnedFilesFn(owned)
		result.PreservedPaths = append(result.PreservedPaths, preserved...)
		if err != nil {
			return result, recoverGGA(&result, homeDir, evidence, err)
		}
		if err := removeEmptyGGADirectories(homeDir); err != nil {
			return result, recoverGGA(&result, homeDir, evidence, err)
		}
	}
	next := persisted
	next.Components = filtered
	committed, commitErr := ggaLeaseCommitFn(lease, next)
	if committed.Outcome != statestore.Committed {
		if commitErr == nil {
			// refusal:by-design operator-knowledge: an absent commit outcome cannot be classified as success.
			commitErr = errors.New("GGA retirement state publication was not committed")
		}
		if len(owned) > 0 {
			return result, recoverGGA(&result, homeDir, evidence, fmt.Errorf("persist retired GGA state: %w", commitErr))
		}
		result.Outcome = statestore.Uncommitted
		return result, fmt.Errorf("persist retired GGA state: %w", commitErr)
	}
	if len(owned) > 0 && !ggaRetired(evidence) {
		result.RecoveryPaths, result.Outcome = evidence.paths(), statestore.Unknown
		// refusal:by-design operator-knowledge: external retirement cannot be proven safe.
		return result, &GGARetirementRecoveryError{Outcome: result.Outcome, RecoveryPaths: result.RecoveryPaths, Err: errors.New("verify GGA retirement")}
	}
	result.Changed, result.Outcome = true, statestore.Committed
	return result, commitErr
}
func filterLegacyGGAComponents(components []model.ComponentID) []model.ComponentID {
	filtered := make([]model.ComponentID, 0, len(components))
	for _, component := range components {
		if component != legacyGGAComponentID {
			filtered = append(filtered, component)
		}
	}
	return filtered
}

type ggaManagedFile struct {
	path         string
	fingerprints map[string]struct{}
}

func legacyGGAManagedFiles(homeDir string) []ggaManagedFile {
	return []ggaManagedFile{
		{legacyGGAConfigPath(homeDir), map[string]struct{}{"98ddd3a5af86dddcd5abaf247d2269d3c8d3efbd0cc3b6fcc37185d3c51de296": {}, "cdce9c0a004c313bb2ef5286873db74476e1a7707a061b877a456daeb9804487": {}, "878768d613def5adb784557fb0be93a4543da7ba86e4747611c079737beebf38": {}, "c8dc7e4f1ccd67402b78e4b7d72b066c5c39786c08fd9f2a49ed40650eed056a": {}}},
		{filepath.Join(legacyGGARootDir(homeDir), "AGENTS.md"), map[string]struct{}{"b8d688b95e7d26ad51d4cdbbe9f226fe8056a0f806f1e459fe66589aa060b3a4": {}}},
		{filepath.Join(legacyGGARootDir(homeDir), "lib", "pr_mode.sh"), map[string]struct{}{"a4335d2167ba3530cf56c7a328f0a5ad18921a8896217f1263b4d497f5fdb9fb": {}}},
		{filepath.Join(homeDir, "bin", "gga.ps1"), map[string]struct{}{"3b354091415658e97200e5ccf0bd1abb77c39e7d3dff7d80e9def78a56507abd": {}}},
		{filepath.Join(homeDir, "bin", "gga.cmd"), map[string]struct{}{"2d266e6e48fe77ea3365b5aee343148b7a735686caa94aae60e06cf7fa87648c": {}}},
	}
}

func legacyGGARootDir(homeDir string) string {
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "gga")
		}
		return filepath.Join(homeDir, "AppData", "Roaming", "gga")
	}
	return filepath.Join(homeDir, ".config", "gga")
}

func legacyGGAConfigPath(homeDir string) string {
	return filepath.Join(legacyGGARootDir(homeDir), "config")
}
func findOwnedGGAFiles(homeDir string) ([]ggaManagedFile, []string, error) {
	var owned []ggaManagedFile
	var preservedPaths []string
	for _, candidate := range legacyGGAManagedFiles(homeDir) {
		exists, isOwned, err := inspectGGACandidate(candidate)
		if err != nil {
			return nil, nil, err
		}
		if !exists {
			continue
		}
		if !isOwned {
			preservedPaths = append(preservedPaths, candidate.path)
			continue
		}
		owned = append(owned, candidate)
	}
	return owned, preservedPaths, nil
}
func backupLegacyGGAFiles(homeDir string, files []ggaManagedFile) (backup.Manifest, error) {
	root := filepath.Join(homeDir, ".shevanio-ai", "backups")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return backup.Manifest{}, fmt.Errorf("create GGA retirement backup directory: %w", err)
	}
	id := "retire-gga-" + ggaRetirementNow().UTC().Format("20060102150405.000000000")
	dir := filepath.Join(root, id)
	if err := os.Mkdir(dir, 0o755); err != nil {
		return backup.Manifest{}, fmt.Errorf("create GGA retirement backup directory: %w", err)
	}
	paths := make([]string, len(files))
	for i, file := range files {
		paths[i] = file.path
	}
	manifest, err := ggaRetirementSnapshotter().Create(dir, paths)
	if err != nil {
		_ = os.RemoveAll(dir)
		return backup.Manifest{}, fmt.Errorf("create GGA retirement backup: %w", err)
	}
	manifest.Source = backup.BackupSourceRetireGGA
	if err := backup.WriteManifest(filepath.Join(dir, backup.ManifestFilename), manifest); err != nil {
		_ = os.RemoveAll(dir)
		return backup.Manifest{}, fmt.Errorf("annotate GGA retirement backup: %w", err)
	}
	if err := ggaRetirementDurabilityFn(homeDir, files, manifest); err != nil {
		_ = os.RemoveAll(dir)
		return backup.Manifest{}, fmt.Errorf("publish GGA retirement backup: %w", err)
	}
	return manifest, nil
}
func durableGGABackup(_ string, files []ggaManagedFile, manifest backup.Manifest) error {
	for _, name := range []string{backup.ArchiveFilename, backup.ManifestFilename} {
		if err := syncGGA(filepath.Join(manifest.RootDir, name), false); err != nil {
			return err
		}
	}
	if err := syncGGA(manifest.RootDir, true); err != nil {
		return err
	}
	got, err := backup.ReadManifest(filepath.Join(manifest.RootDir, backup.ManifestFilename))
	if err != nil {
		return err
	}
	if got.Source != backup.BackupSourceRetireGGA {
		// refusal:by-design operator-knowledge: a tampered manifest cannot authorize removal.
		return fmt.Errorf("GGA retirement manifest source mismatch")
	}
	want := map[string]bool{}
	for _, file := range files {
		want[file.path] = true
	}
	for _, entry := range got.Entries {
		if entry.Existed {
			delete(want, entry.OriginalPath)
		}
	}
	if len(want) != 0 || len(got.Entries) != len(files) {
		// refusal:by-design operator-knowledge: an unverifiable archive inventory cannot be safely removed.
		return errors.New("GGA retirement backup inventory mismatch")
	}
	return nil
}
func syncGGA(path string, directory bool) error {
	flags := os.O_RDWR
	if directory {
		flags = os.O_RDONLY
	}
	f, err := os.OpenFile(path, flags, 0)
	if err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	err = f.Close()
	if err != nil {
		return err
	}
	if directory {
		return nil
	}
	_, err = os.ReadFile(path)
	return err
}
func removeOwnedGGAFiles(files []ggaManagedFile) ([]string, error) {
	var preserved []string
	for _, file := range files {
		exists, isOwned, err := inspectGGACandidate(file)
		if err != nil {
			return preserved, err
		}
		if !exists {
			continue
		}
		if !isOwned {
			preserved = append(preserved, file.path)
			continue
		}
		if err := os.Remove(file.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return preserved, fmt.Errorf("remove owned GGA path %q: %w", file.path, err)
		}
	}
	return preserved, nil
}
func inspectGGACandidate(candidate ggaManagedFile) (bool, bool, error) {
	info, err := os.Lstat(candidate.path)
	if errors.Is(err, os.ErrNotExist) {
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("inspect legacy GGA path %q: %w", candidate.path, err)
	}
	if !info.Mode().IsRegular() {
		return true, false, nil
	}
	data, err := os.ReadFile(candidate.path)
	if err != nil {
		return false, false, fmt.Errorf("fingerprint legacy GGA path %q: %w", candidate.path, err)
	}
	fingerprint := fmt.Sprintf("%x", sha256.Sum256(data))
	_, owned := candidate.fingerprints[fingerprint]
	return true, owned, nil
}
func removeEmptyGGADirectories(homeDir string) error {
	for _, dir := range []string{
		filepath.Join(homeDir, ".config", "gga"),
		filepath.Join(homeDir, ".local", "share", "gga", "lib"),
		filepath.Join(homeDir, ".local", "share", "gga"),
	} {
		info, err := os.Lstat(dir)
		if errors.Is(err, os.ErrNotExist) || err != nil || !info.IsDir() {
			continue
		}
		if err := os.Remove(dir); err != nil && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, syscall.ENOTEMPTY) {
			return fmt.Errorf("remove empty GGA directory %q: %w", dir, err)
		}
	}
	return nil
}
func (e ggaRecoveryEvidence) paths() []string {
	return []string{e.backupDir, filepath.Join(e.backupDir, backup.ArchiveFilename), filepath.Join(e.backupDir, backup.ManifestFilename)}
}
func ggaDirs(homeDir string) []string {
	return []string{filepath.Join(homeDir, ".config", "gga"), filepath.Join(homeDir, ".local", "share", "gga", "lib"), filepath.Join(homeDir, ".local", "share", "gga")}
}
func loadGGARetirementEvidence(homeDir string) (ggaRecoveryEvidence, bool) {
	root := filepath.Join(homeDir, ".shevanio-ai", "backups")
	entries, err := os.ReadDir(root)
	if err != nil {
		return ggaRecoveryEvidence{}, false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		manifest, err := backup.ReadManifest(filepath.Join(dir, backup.ManifestFilename))
		if err == nil && manifest.Source == backup.BackupSourceRetireGGA && manifest.RootDir == dir {
			return ggaRecoveryEvidence{backupDir: dir, manifest: manifest}, true
		}
	}
	return ggaRecoveryEvidence{}, false
}
func ggaRetired(evidence ggaRecoveryEvidence) bool {
	for _, entry := range evidence.manifest.Entries {
		if entry.Existed {
			if _, err := os.Lstat(entry.OriginalPath); !errors.Is(err, os.ErrNotExist) {
				return false
			}
		}
	}
	return true
}
func restoreGGASnapshot(homeDir string, evidence ggaRecoveryEvidence) error {
	if err := (backup.RestoreService{Roots: []string{homeDir}}).Restore(evidence.manifest); err != nil {
		return err
	}
	for _, dir := range ggaDirs(homeDir) {
		mode, exists := evidence.dirs[dir]
		if exists {
			if err := os.MkdirAll(dir, mode.Perm()); err != nil {
				return err
			}
			if err := os.Chmod(dir, mode.Perm()); err != nil {
				return err
			}
		}
		info, err := os.Stat(dir)
		if exists && (err != nil || info.Mode().Perm() != mode.Perm()) {
			// refusal:by-design operator-knowledge: recovery metadata drift requires operator review.
			return errors.New("GGA recovery directory mismatch")
		}
		if !exists && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("GGA recovery directory absence mismatch: %w", err)
		}
	}
	return nil
}
func recoverGGA(result *GGARetirementResult, homeDir string, evidence ggaRecoveryEvidence, primary error) error {
	result.RecoveryPaths = evidence.paths()
	if recoveryErr := ggaRetirementRestoreFn(homeDir, evidence); recoveryErr != nil {
		result.Outcome = statestore.Unknown
		return errors.Join(primary, &GGARetirementRecoveryError{Outcome: result.Outcome, RecoveryPaths: result.RecoveryPaths, Err: recoveryErr})
	}
	result.Outcome = statestore.Uncommitted
	return primary
}
