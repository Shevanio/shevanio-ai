package statestore

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/shevanio/shevanio-ai/v2/internal/reviewtransaction"
	"github.com/shevanio/shevanio-ai/v2/internal/state"
	"os"
	"path/filepath"
)

type Outcome string

const (
	Uncommitted Outcome = "uncommitted"
	Committed   Outcome = "committed"
	Unknown     Outcome = "unknown"
)

type Result struct{ Outcome Outcome }
type Mutator func(*state.InstallState) error
type fileSnapshot struct {
	exists bool
	data   []byte
	mode   os.FileMode
}

var writeState = state.Write
var restoreState = restoreSnapshotDefault
var releaseLock = func(lock *reviewtransaction.AuthorityFileLock) error { return lock.Release() }

func Mutate(homeDir string, mutate Mutator) (Result, error)   { return mutateWithLock(homeDir, mutate) }
func WithLock(homeDir string, mutate Mutator) (Result, error) { return mutateWithLock(homeDir, mutate) }

func mutateWithLock(homeDir string, mutate Mutator) (Result, error) {
	lock, err := reviewtransaction.AcquireAuthorityFileLock(state.Path(homeDir) + ".lock")
	if err != nil {
		return Result{Outcome: Uncommitted}, err
	}
	result, operationErr := mutateLocked(homeDir, mutate)
	releaseErr := releaseLock(lock)
	if releaseErr != nil {
		operationErr = errors.Join(operationErr, releaseErr)
	}
	return result, operationErr
}
func mutateLocked(homeDir string, mutate Mutator) (Result, error) {
	path := state.Path(homeDir)
	snapshot, err := snapshotFile(path)
	if err != nil {
		return Result{Outcome: Uncommitted}, err
	}
	current, err := state.Read(homeDir)
	if errors.Is(err, os.ErrNotExist) {
		current = state.InstallState{}
	} else if err != nil {
		return Result{Outcome: Uncommitted}, err
	}
	if err := mutate(&current); err != nil {
		return Result{Outcome: Uncommitted}, err
	}
	desired, err := marshalState(current)
	if err != nil {
		return Result{Outcome: Uncommitted}, err
	}
	if snapshot.exists && bytes.Equal(snapshot.data, desired) {
		return Result{Outcome: Committed}, nil
	}
	if err := writeState(homeDir, current); err != nil {
		if readBack(path, desired) == nil {
			return Result{Outcome: Committed}, err
		}
		return restoreAfterFailure(homeDir, snapshot, err)
	}
	if err := readBack(path, desired); err != nil {
		return restoreAfterFailure(homeDir, snapshot, err)
	}
	return Result{Outcome: Committed}, nil
}
func restoreAfterFailure(homeDir string, snapshot fileSnapshot, publicationErr error) (Result, error) {
	if err := restoreState(homeDir, snapshot); err != nil {
		return Result{Outcome: Unknown}, errors.Join(publicationErr, err)
	}
	got, err := snapshotFile(state.Path(homeDir))
	if err != nil || got.exists != snapshot.exists || (got.exists && (!bytes.Equal(got.data, snapshot.data) || got.mode.Perm() != snapshot.mode.Perm())) {
		return Result{Outcome: Unknown}, errors.Join(publicationErr, err)
	}
	return Result{Outcome: Uncommitted}, publicationErr
}
func snapshotFile(path string) (fileSnapshot, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return fileSnapshot{}, nil
	}
	if err != nil {
		return fileSnapshot{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fileSnapshot{}, err
	}
	return fileSnapshot{exists: true, data: append([]byte(nil), data...), mode: info.Mode().Perm()}, nil
}
func restoreSnapshotDefault(homeDir string, snapshot fileSnapshot) error {
	path := state.Path(homeDir)
	if !snapshot.exists {
		err := os.Remove(path)
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, snapshot.data, snapshot.mode.Perm()); err != nil {
		return err
	}
	return os.Chmod(path, snapshot.mode.Perm())
}
func readBack(path string, want []byte) error {
	got, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(got, want) {
		return errors.New("statestore: publication mismatch")
	}
	return nil
}
func marshalState(s state.InstallState) ([]byte, error) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
