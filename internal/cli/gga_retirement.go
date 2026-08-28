package cli

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/shevanio/shevanio-ai/v2/internal/backup"
	"github.com/shevanio/shevanio-ai/v2/internal/model"
	"github.com/shevanio/shevanio-ai/v2/internal/reviewtransaction"
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
	dirs      map[string]*os.FileMode
}

var (
	ggaRetirementNow         = time.Now
	ggaRetirementSnapshotter = func() backup.Snapshotter { return backup.NewSnapshotter() }
	ggaLeaseCommitFn         = func(lease *statestore.Lease, next state.InstallState) (statestore.Result, error) {
		return lease.Commit(next)
	}
	ggaRemoveOwnedFilesFn     = removeOwnedGGAFiles
	ggaRetirementDurabilityFn = durableGGABackup
	ggaRestoreServiceFn       = func(homeDir string, manifest backup.Manifest) error {
		return (backup.RestoreService{Roots: []string{homeDir}}).Restore(manifest)
	}
	ggaSyncFn = syncGGA
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
	if len(filtered) == len(persisted.Components) {
		return result, nil
	}
	evidence, ok, evidenceErr := loadGGARetirementEvidence(homeDir)
	if ok {
		result.BackupID, result.RecoveryPaths = evidence.manifest.ID, evidence.paths()
		if evidenceErr != nil {
			return result, unknownGGARetirement(&result, evidence, evidenceErr)
		}
		retired, verifyErr := ggaRetired(homeDir, evidence)
		if verifyErr != nil {
			return result, unknownGGARetirement(&result, evidence, verifyErr)
		}
		if !retired {
			owned, preserved, retryErr := findOwnedGGAFiles(homeDir)
			if retryErr != nil {
				return result, unknownGGARetirement(&result, evidence, retryErr)
			}
			result.PreservedPaths = preserved
			preserved, retryErr = ggaRemoveOwnedFilesFn(owned)
			result.PreservedPaths = append(result.PreservedPaths, preserved...)
			if retryErr != nil {
				return result, recoverGGA(&result, homeDir, evidence, retryErr)
			}
			if retryErr = removeEmptyGGADirectories(homeDir); retryErr != nil {
				return result, recoverGGA(&result, homeDir, evidence, retryErr)
			}
			retired, retryErr = ggaRetired(homeDir, evidence)
			if retryErr != nil || !retired {
				return result, unknownGGARetirement(&result, evidence, retryErr)
			}
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
		return result, errors.New("GGA retirement state publication was not committed") // refusal:by-design operator-knowledge: an absent commit outcome cannot be classified as success.
	}
	owned, preserved, err := findOwnedGGAFiles(homeDir)
	if err != nil {
		return result, err
	}
	result.PreservedPaths = preserved
	if len(owned) > 0 {
		evidence.dirs = make(map[string]*os.FileMode, len(ggaDirs(homeDir)))
		for _, dir := range ggaDirs(homeDir) {
			if info, statErr := os.Stat(dir); statErr == nil {
				mode := info.Mode().Perm()
				evidence.dirs[dir] = &mode
			} else if errors.Is(statErr, os.ErrNotExist) {
				evidence.dirs[dir] = nil
			} else {
				return result, statErr
			}
		}
		manifest, err := backupLegacyGGAFiles(homeDir, owned)
		if err != nil {
			result.Outcome = statestore.Uncommitted
			return result, err
		}
		evidence.backupDir, evidence.manifest = manifest.RootDir, manifest
		result.BackupID = manifest.ID
		stored, err := backup.ReadManifest(filepath.Join(manifest.RootDir, backup.ManifestFilename))
		if err != nil {
			return result, unknownGGARetirement(&result, evidence, err)
		}
		evidence.manifest = stored
		if err := verifyGGARetirementEvidence(homeDir, evidence, owned, true); err != nil {
			return result, unknownGGARetirement(&result, evidence, err)
		}
		preserved, err := ggaRemoveOwnedFilesFn(owned)
		result.PreservedPaths = append(result.PreservedPaths, preserved...)
		if err != nil {
			return result, recoverGGA(&result, homeDir, evidence, err)
		}
		if err := removeEmptyGGADirectories(homeDir); err != nil {
			return result, recoverGGA(&result, homeDir, evidence, err)
		}
		retired, verifyErr := ggaRetired(homeDir, evidence)
		if verifyErr != nil {
			return result, recoverGGA(&result, homeDir, evidence, verifyErr)
		}
		if !retired {
			return result, recoverGGA(&result, homeDir, evidence, ggaRefusal("verify GGA retirement"))
		}
	}
	next := persisted
	next.Components = filtered
	committed, commitErr := ggaLeaseCommitFn(lease, next)
	if committed.Outcome != statestore.Committed {
		if commitErr == nil {
			commitErr = errors.New("GGA retirement state publication was not committed") // refusal:by-design operator-knowledge: an absent commit outcome cannot be classified as success.
		}
		if len(owned) > 0 {
			return result, recoverGGA(&result, homeDir, evidence, fmt.Errorf("persist retired GGA state: %w", commitErr))
		}
		result.Outcome = statestore.Uncommitted
		return result, fmt.Errorf("persist retired GGA state: %w", commitErr)
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
		if err := ggaSyncFn(filepath.Join(manifest.RootDir, name), false); err != nil {
			return err
		}
	}
	if err := ggaSyncFn(manifest.RootDir, true); err != nil {
		return err
	}
	if err := ggaSyncFn(filepath.Dir(manifest.RootDir), true); err != nil {
		return err
	}
	got, err := backup.ReadManifest(filepath.Join(manifest.RootDir, backup.ManifestFilename))
	if err != nil {
		return err
	}
	if got.Source != backup.BackupSourceRetireGGA {
		return fmt.Errorf("GGA retirement manifest source mismatch") // refusal:by-design operator-knowledge: a tampered manifest cannot authorize removal.
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
		return errors.New("GGA retirement backup inventory mismatch") // refusal:by-design operator-knowledge: an unverifiable archive inventory cannot be safely removed.
	}
	return nil
}
func syncGGA(path string, directory bool) error {
	if directory {
		// Unsupported handles are a lower durability claim; SyncReviewDirectory propagates real failures.
		return reviewtransaction.SyncReviewDirectory(path)
	}
	flags := os.O_RDWR
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
func loadGGARetirementEvidence(homeDir string) (ggaRecoveryEvidence, bool, error) {
	root := filepath.Join(homeDir, ".shevanio-ai", "backups")
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ggaRecoveryEvidence{}, false, nil
		}
		return ggaRecoveryEvidence{backupDir: root}, true, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() > entries[j].Name() })
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "retire-gga-") {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		manifest, err := backup.ReadManifest(filepath.Join(dir, backup.ManifestFilename))
		return ggaRecoveryEvidence{backupDir: dir, manifest: manifest}, true, err
	}
	return ggaRecoveryEvidence{}, false, nil
}
func ggaRetired(homeDir string, evidence ggaRecoveryEvidence) (bool, error) {
	if err := verifyGGARetirementEvidence(homeDir, evidence, nil, false); err != nil {
		return false, err
	}
	owned, _, err := findOwnedGGAFiles(homeDir)
	return len(owned) == 0, err
}
func verifyGGARetirementEvidence(homeDir string, evidence ggaRecoveryEvidence, files []ggaManagedFile, live bool) error {
	manifest := evidence.manifest
	if manifest.Source != backup.BackupSourceRetireGGA || filepath.Clean(manifest.RootDir) != filepath.Clean(evidence.backupDir) || !manifest.Compressed || manifest.FileCount != len(manifest.Entries) {
		return ggaRefusal("GGA retirement manifest archive metadata mismatch")
	}
	allowed := make(map[string]ggaManagedFile)
	for _, candidate := range legacyGGAManagedFiles(homeDir) {
		allowed[candidate.path] = candidate
	}
	for _, entry := range manifest.Entries {
		if _, ok := allowed[entry.OriginalPath]; !ok || !entry.Existed || entry.Mode&^uint32(os.ModePerm) != 0 || entry.SnapshotPath == "." || filepath.IsAbs(entry.SnapshotPath) || filepath.VolumeName(entry.SnapshotPath) != "" || strings.Contains(entry.SnapshotPath, "\\") || path.Clean(entry.SnapshotPath) != entry.SnapshotPath || strings.Contains(entry.SnapshotPath, ":") {
			return ggaRefusal("GGA retirement manifest entry mismatch")
		}
	}
	archive, err := inspectGGAArchive(manifest, allowed)
	if err != nil {
		return err
	}
	if len(archive) != len(manifest.Entries) || ggaCompositeChecksum(archive) != manifest.Checksum {
		return ggaRefusal("GGA retirement archive inventory or checksum mismatch")
	}
	if !live {
		return nil
	}
	var livePaths []string
	for _, entry := range manifest.Entries {
		info, err := os.Lstat(entry.OriginalPath)
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("GGA retirement live path mismatch %q: %w", entry.OriginalPath, err)
		}
		data, err := os.ReadFile(entry.OriginalPath)
		if err != nil {
			return err
		}
		if !bytes.Equal(data, archive[entry.OriginalPath]) || info.Mode().Perm() != os.FileMode(entry.Mode).Perm() {
			return ggaRefusal("GGA retirement live snapshot mismatch")
		}
		livePaths = append(livePaths, entry.OriginalPath)
	}
	checksum, err := backup.ComputeChecksum(livePaths)
	if err != nil || checksum != manifest.Checksum {
		return ggaRefusal("GGA retirement live checksum mismatch")
	}
	return nil
}
func inspectGGAArchive(manifest backup.Manifest, allowed map[string]ggaManagedFile) (snapshots map[string][]byte, err error) {
	tempDir, err := os.MkdirTemp("", "shevanio-ai-gga-verify-*")
	if err != nil {
		return nil, err
	}
	defer func() {
		if cleanupErr := os.RemoveAll(tempDir); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("cleanup GGA verification directory: %w", cleanupErr))
		}
	}()
	extracted, err := backup.ExtractArchive(filepath.Join(manifest.RootDir, backup.ArchiveFilename), tempDir)
	if err != nil {
		return nil, err
	}
	bySnapshot := make(map[string]backup.ManifestEntry, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		bySnapshot[entry.SnapshotPath] = entry
	}
	snapshots = make(map[string][]byte, len(extracted))
	for _, extractedEntry := range extracted {
		entry, ok := bySnapshot[extractedEntry.RelPath]
		_, duplicate := snapshots[entry.OriginalPath]
		if !ok || duplicate {
			return nil, ggaRefusal("GGA retirement extracted inventory mismatch")
		}
		info, err := os.Lstat(extractedEntry.SourcePath)
		if err != nil || !info.Mode().IsRegular() {
			return nil, ggaRefusal("GGA retirement extracted file type mismatch")
		}
		data, err := os.ReadFile(extractedEntry.SourcePath)
		if err != nil {
			return nil, err
		}
		fingerprint := fmt.Sprintf("%x", sha256.Sum256(data))
		candidate := allowed[entry.OriginalPath]
		if _, ok := candidate.fingerprints[fingerprint]; !ok || info.Mode().Perm() != os.FileMode(entry.Mode).Perm() {
			return nil, ggaRefusal("GGA retirement extracted snapshot mismatch")
		}
		snapshots[entry.OriginalPath] = data
	}
	return snapshots, nil
}
func ggaCompositeChecksum(snapshots map[string][]byte) string {
	paths := make([]string, 0, len(snapshots))
	for originalPath := range snapshots {
		paths = append(paths, originalPath)
	}
	sort.Strings(paths)
	var content strings.Builder
	for _, originalPath := range paths {
		sum := sha256.Sum256(snapshots[originalPath])
		fmt.Fprintf(&content, "%s:%x\n", originalPath, sum)
	}
	sum := sha256.Sum256([]byte(content.String()))
	return fmt.Sprintf("%x", sum)
}
func unknownGGARetirement(result *GGARetirementResult, evidence ggaRecoveryEvidence, err error) error {
	if err == nil {
		err = ggaRefusal("GGA retirement proof failed")
	}
	result.RecoveryPaths, result.Outcome = evidence.paths(), statestore.Unknown
	return &GGARetirementRecoveryError{Outcome: result.Outcome, RecoveryPaths: result.RecoveryPaths, Err: err}
}
func ggaRefusal(message string) error {
	return fmt.Errorf("%s", message) // refusal:by-design operator-knowledge: semantic recovery evidence requires a code-level correction before mutation.
}
func restoreGGASnapshot(homeDir string, evidence ggaRecoveryEvidence) error {
	if err := verifyGGARetirementEvidence(homeDir, evidence, nil, false); err != nil {
		return err
	}
	if len(evidence.dirs) != len(ggaDirs(homeDir)) {
		return ggaRefusal("GGA recovery directory evidence incomplete")
	}
	manifest := evidence.manifest
	manifest.Entries = append([]backup.ManifestEntry(nil), manifest.Entries...)
	for i := range manifest.Entries {
		manifest.Entries[i].Mode &= uint32(os.ModePerm)
	}
	if err := ggaRestoreServiceFn(homeDir, manifest); err != nil {
		return err
	}
	for _, dir := range ggaDirs(homeDir) {
		mode := evidence.dirs[dir]
		if mode != nil {
			if err := os.MkdirAll(dir, mode.Perm()); err != nil {
				return err
			}
			if err := os.Chmod(dir, mode.Perm()); err != nil {
				return err
			}
		}
		info, err := os.Stat(dir)
		if mode != nil && (err != nil || info.Mode().Perm() != mode.Perm()) {
			return ggaRefusal("GGA recovery directory mismatch")
		}
		if mode == nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("GGA recovery directory absence mismatch: %w", err)
		}
	}
	return verifyGGARetirementEvidence(homeDir, evidence, nil, true)
}
func recoverGGA(result *GGARetirementResult, homeDir string, evidence ggaRecoveryEvidence, primary error) error {
	result.RecoveryPaths = evidence.paths()
	if recoveryErr := restoreGGASnapshot(homeDir, evidence); recoveryErr != nil {
		result.Outcome = statestore.Unknown
		return errors.Join(primary, &GGARetirementRecoveryError{Outcome: result.Outcome, RecoveryPaths: result.RecoveryPaths, Err: recoveryErr})
	}
	result.Outcome = statestore.Uncommitted
	return primary
}
