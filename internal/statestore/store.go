package statestore

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/shevanio/shevanio-ai/v2/internal/reviewtransaction"
	"github.com/shevanio/shevanio-ai/v2/internal/state"
)

type Outcome string

const (
	Uncommitted Outcome = "uncommitted"
	Committed   Outcome = "committed"
	Unknown     Outcome = "unknown"
)

type Result struct{ Outcome Outcome }
type Mutator func(*state.InstallState) error
type leaseLifecycle uint8

const (
	leaseActive leaseLifecycle = iota
	leaseTerminal
	leaseReleased
)

type Lease struct {
	homeDir    string
	lock       *reviewtransaction.AuthorityFileLock
	preimage   fileSnapshot
	lifecycle  leaseLifecycle
	releaseErr error
}

var ErrNilLease = errors.New("statestore: nil lease")

type LeaseLifecycleError struct {
	Operation string
	State     leaseLifecycle
}

func (err *LeaseLifecycleError) Error() string {
	return "statestore: lease " + err.Operation + " refused outside active lifecycle"
}

func Begin(homeDir string) (*Lease, error) {
	lock, err := reviewtransaction.AcquireAuthorityFileLock(state.Path(homeDir) + ".lock")
	if err != nil {
		return nil, err
	}
	preimage, err := snapshotFile(state.Path(homeDir))
	if err != nil {
		_ = releaseLock(lock)
		return nil, err
	}
	return &Lease{homeDir: homeDir, lock: lock, preimage: preimage}, nil
}

func (lease *Lease) Current() (state.InstallState, error) {
	if err := lease.requireActive("current"); err != nil {
		return state.InstallState{}, err
	}
	current, err := state.Read(lease.homeDir)
	if errors.Is(err, os.ErrNotExist) {
		return state.InstallState{}, nil
	}
	if err != nil {
		return state.InstallState{}, err
	}
	return cloneState(current)
}

func (lease *Lease) Commit(next state.InstallState) (Result, error) {
	if err := lease.requireActive("commit"); err != nil {
		return Result{Outcome: Uncommitted}, err
	}
	result, err := lease.commit(next)
	if err != nil && result.Outcome != Committed {
		restored, restoreErr := lease.Restore()
		if restoreErr != nil {
			return restored, errors.Join(err, restoreErr)
		}
		return restored, err
	}
	lease.lifecycle = leaseTerminal
	return result, err
}

func (lease *Lease) Restore() (Result, error) {
	if err := lease.requireActive("restore"); err != nil {
		return Result{Outcome: Uncommitted}, err
	}
	result, err := lease.restore()
	lease.lifecycle = leaseTerminal
	return result, err
}

func (lease *Lease) Release() error {
	if lease == nil {
		return nil
	}
	if lease.lifecycle == leaseReleased {
		return lease.releaseErr
	}
	lease.lifecycle = leaseReleased
	lease.releaseErr = releaseLock(lease.lock)
	lease.lock = nil
	return lease.releaseErr
}

func (lease *Lease) requireActive(operation string) error {
	if lease == nil {
		return ErrNilLease
	}
	if lease.lifecycle != leaseActive {
		return &LeaseLifecycleError{Operation: operation, State: lease.lifecycle}
	}
	return nil
}

func (lease *Lease) commit(next state.InstallState) (Result, error) {
	desired, err := marshalState(next)
	if err != nil {
		return Result{Outcome: Uncommitted}, err
	}
	if lease.preimage.exists && bytes.Equal(lease.preimage.data, desired) {
		return Result{Outcome: Committed}, nil
	}
	if err := writeLeaseState(lease.homeDir, next); err != nil {
		if readBack(state.Path(lease.homeDir), desired) == nil {
			return Result{Outcome: Committed}, err
		}
		return Result{Outcome: Uncommitted}, err
	}
	if err := readBack(state.Path(lease.homeDir), desired); err != nil {
		return Result{Outcome: Uncommitted}, err
	}
	return Result{Outcome: Committed}, nil
}

func (lease *Lease) restore() (Result, error) {
	if err := restoreState(lease.homeDir, lease.preimage); err != nil {
		return Result{Outcome: Unknown}, err
	}
	got, err := snapshotFile(state.Path(lease.homeDir))
	if err != nil || !sameSnapshot(got, lease.preimage) {
		if err == nil {
			err = errors.New("statestore: restoration mismatch")
		}
		return Result{Outcome: Unknown}, err
	}
	return Result{Outcome: Uncommitted}, nil
}

func cloneState(current state.InstallState) (state.InstallState, error) {
	data, err := marshalState(current)
	if err != nil {
		return state.InstallState{}, err
	}
	var clone state.InstallState
	if err := json.Unmarshal(data, &clone); err != nil {
		return state.InstallState{}, err
	}
	return clone, nil
}

type fileSnapshot struct {
	exists bool
	data   []byte
	mode   os.FileMode
}

var writeState = state.Write
var writeLeaseState = state.WriteReconciled
var restoreState = restoreSnapshotDefault
var releaseLock = func(lock *reviewtransaction.AuthorityFileLock) error { return lock.Release() }

func Mutate(homeDir string, mutate Mutator) (Result, error) { return mutateWithLock(homeDir, mutate) }

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
func sameSnapshot(left, right fileSnapshot) bool {
	return left.exists == right.exists && (!left.exists || (bytes.Equal(left.data, right.data) && left.mode.Perm() == right.mode.Perm()))
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
