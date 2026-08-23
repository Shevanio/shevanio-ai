package cli

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/shevanio/shevanio-ai/v2/internal/backup"
	"github.com/shevanio/shevanio-ai/v2/internal/components/gga"
	"github.com/shevanio/shevanio-ai/v2/internal/model"
	"github.com/shevanio/shevanio-ai/v2/internal/state"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type GGARetirementResult struct {
	Changed        bool
	BackupID       string
	PreservedPaths []string
}

var (
	ggaRetirementNow         = time.Now
	ggaRetirementSnapshotter = func() backup.Snapshotter { return backup.NewSnapshotter() }
	ggaStateWriteFn          = state.WriteReconciled
)

func MigrateLegacyGGA(homeDir string, ggaRegistered bool) (GGARetirementResult, error) {
	if ggaRegistered {
		return GGARetirementResult{}, nil
	}
	var result GGARetirementResult
	err := withInstallStateLock(homeDir, func() error {
		persisted, err := state.Read(homeDir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("read install state: %w", err)
		}
		filtered := filterLegacyGGAComponents(persisted.Components)
		if len(filtered) == len(persisted.Components) {
			return nil
		}
		owned, preserved, err := findOwnedGGAFiles(homeDir)
		if err != nil {
			return err
		}
		result.PreservedPaths = preserved
		if len(owned) > 0 {
			manifest, err := backupLegacyGGAFiles(homeDir, owned)
			if err != nil {
				return err
			}
			result.BackupID = manifest.ID
			preserved, err := removeOwnedGGAFiles(owned)
			result.PreservedPaths = append(result.PreservedPaths, preserved...)
			if err != nil {
				return err
			}
			if err := removeEmptyGGADirectories(homeDir); err != nil {
				return err
			}
		}
		persisted.Components = filtered
		if err := ggaStateWriteFn(homeDir, persisted); err != nil {
			return fmt.Errorf("persist retired GGA state: %w", err)
		}
		result.Changed = true
		return nil
	})
	return result, err
}
func filterLegacyGGAComponents(components []model.ComponentID) []model.ComponentID {
	filtered := make([]model.ComponentID, 0, len(components))
	for _, component := range components {
		if component != model.ComponentGGA {
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
		{gga.ConfigPath(homeDir), map[string]struct{}{"98ddd3a5af86dddcd5abaf247d2269d3c8d3efbd0cc3b6fcc37185d3c51de296": {}, "cdce9c0a004c313bb2ef5286873db74476e1a7707a061b877a456daeb9804487": {}, "878768d613def5adb784557fb0be93a4543da7ba86e4747611c079737beebf38": {}, "c8dc7e4f1ccd67402b78e4b7d72b066c5c39786c08fd9f2a49ed40650eed056a": {}}},
		{gga.AgentsTemplatePath(homeDir), map[string]struct{}{"b8d688b95e7d26ad51d4cdbbe9f226fe8056a0f806f1e459fe66589aa060b3a4": {}}}, {gga.RuntimePRModePath(homeDir), map[string]struct{}{"a4335d2167ba3530cf56c7a328f0a5ad18921a8896217f1263b4d497f5fdb9fb": {}}},
		{gga.RuntimePS1Path(homeDir), map[string]struct{}{"3b354091415658e97200e5ccf0bd1abb77c39e7d3dff7d80e9def78a56507abd": {}}}, {gga.RuntimeCMDPath(homeDir), map[string]struct{}{"2d266e6e48fe77ea3365b5aee343148b7a735686caa94aae60e06cf7fa87648c": {}}},
	}
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
	return manifest, nil
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
