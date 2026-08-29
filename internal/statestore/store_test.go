package statestore

import (
	"bytes"
	"errors"
	"github.com/shevanio/shevanio-ai/v2/internal/reviewtransaction"
	"github.com/shevanio/shevanio-ai/v2/internal/state"
	"os"
	"path/filepath"
	"testing"
)

var errDurability, errPublication, errRelease = errors.New("durability failure"), errors.New("publication failure"), errors.New("release failure")

func must(t *testing.T, e error) {
	if e != nil {
		t.Fatalf("setup failure: %v", e)
	}
}
func raw(t *testing.T, h string, b []byte, m os.FileMode) {
	p := state.Path(h)
	must(t, os.MkdirAll(filepath.Dir(p), 0755))
	must(t, os.WriteFile(p, b, m))
}
func require(t *testing.T, ok bool, msg string) {
	if !ok {
		t.Errorf("behavior mismatch: %s", msg)
	}
}
func TestMutateStateSourceAndValidation(t *testing.T) {
	for _, x := range []struct {
		n           string
		b           []byte
		legacy, bad bool
	}{{"absent", nil, false, false}, {"canonical", []byte(`{"persona":"old"}`), false, false}, {"legacy", []byte(`{"persona":"old"}`), true, false}, {"malformed", []byte("{bad"), false, true}, {"newer", []byte(`{"schema_version":2}`), false, true}} {
		h := t.TempDir()
		lp := filepath.Join(h, ".gentle-ai", "state.json")
		if x.b != nil {
			if x.legacy {
				must(t, os.MkdirAll(filepath.Dir(lp), 0755))
				must(t, os.WriteFile(lp, x.b, 0644))
			} else {
				raw(t, h, x.b, 0644)
			}
		}
		r, e := Mutate(h, func(s *state.InstallState) error { s.Persona = "new"; return nil })
		ok := (!x.bad && r.Outcome == Committed && e == nil) || (x.bad && r.Outcome == Uncommitted && e != nil)
		require(t, ok, "state source or validation")
	}
}
func TestMutateCallbackAndFieldPreservation(t *testing.T) {
	h := t.TempDir()
	want := errors.New("callback failure")
	r, e := Mutate(h, func(*state.InstallState) error { return want })
	require(t, r.Outcome == Uncommitted && errors.Is(e, want), "callback failure outcome")
	h = t.TempDir()
	raw(t, h, []byte(`{"future":true}`), 0644)
	r, e = Mutate(h, func(s *state.InstallState) error { s.Persona = "new"; return nil })
	b, re := os.ReadFile(state.Path(h))
	require(t, r.Outcome == Committed && e == nil && re == nil && bytes.Contains(b, []byte(`"future"`)), "unknown field preservation")
}
func TestMutatePublicationAndReleaseOutcomes(t *testing.T) {
	for _, x := range []struct {
		n      string
		setup  func(string)
		want   Outcome
		target error
	}{{"no-op", func(h string) { must(t, state.Write(h, state.InstallState{Persona: "same"})) }, Committed, nil}, {"desired-visible", func(string) {
		writeState = func(h string, s state.InstallState) error { must(t, state.Write(h, s)); return errDurability }
	}, Committed, errDurability}, {"new-release", func(string) { releaseLock = func(*reviewtransaction.AuthorityFileLock) error { return errRelease } }, Committed, errRelease}, {"old-release", func(string) {
		writeState = func(string, state.InstallState) error { return errPublication }
		releaseLock = func(*reviewtransaction.AuthorityFileLock) error { return errRelease }
	}, Uncommitted, errRelease}} {
		t.Run(x.n, func(t *testing.T) {
			ow, or := writeState, releaseLock
			defer func() { writeState, releaseLock = ow, or }()
			h := t.TempDir()
			x.setup(h)
			r, e := Mutate(h, func(s *state.InstallState) error { s.Persona = "new"; return nil })
			require(t, r.Outcome == x.want && ((x.target == nil && e == nil) || errors.Is(e, x.target)), "publication or release outcome")
		})
	}
}
func TestMutateRollbackOutcomes(t *testing.T) {
	for _, x := range []struct {
		n                       string
		exists, exact, mismatch bool
		restore                 error
	}{{"existing bytes and mode", true, true, false, nil}, {"new file absence", false, true, false, nil}, {"restore failure", true, false, false, errPublication}, {"restore mismatch", true, false, true, nil}} {
		t.Run(x.n, func(t *testing.T) {
			h := t.TempDir()
			p := state.Path(h)
			old := []byte(`{"installed_agents":["old"]}`)
			if x.exists {
				raw(t, h, old, 0600)
			}
			ow, or := writeState, restoreState
			defer func() { writeState, restoreState = ow, or }()
			writeState = func(h string, s state.InstallState) error {
				must(t, state.Write(h, state.InstallState{Persona: "wrong"}))
				return errPublication
			}
			restoreState = func(h string, s fileSnapshot) error {
				if x.restore != nil {
					return x.restore
				}
				if x.mismatch {
					return os.WriteFile(p, []byte("wrong"), 0644)
				}
				return restoreSnapshotDefault(h, s)
			}
			r, e := Mutate(h, func(s *state.InstallState) error { s.Persona = "new"; return nil })
			want := Uncommitted
			if !x.exact {
				want = Unknown
			}
			require(t, r.Outcome == want && e != nil, "rollback outcome")
			if x.exact {
				got, re := snapshotFile(p)
				require(t, re == nil && got.exists == x.exists && (!x.exists || bytes.Equal(got.data, old) && got.mode.Perm() == 0600), "exact bytes and mode rollback")
			}
		})
	}
}
func TestMutateContentionAndReentry(t *testing.T) {
	for _, x := range []struct {
		n    string
		call func(string, Mutator) (Result, error)
	}{{"Mutate", Mutate}, {"WithLock", WithLock}} {
		t.Run(x.n, func(t *testing.T) { contentionCase(t, x.call) })
	}
	t.Run("retry preserves distinct fields", func(t *testing.T) {
		h := t.TempDir()
		must(t, state.Write(h, state.InstallState{Persona: "kept"}))
		l, e := reviewtransaction.AcquireAuthorityFileLock(state.Path(h) + ".lock")
		must(t, e)
		_, e = Mutate(h, func(*state.InstallState) error { return nil })
		require(t, errors.Is(e, reviewtransaction.ErrStoreLockContended), "retry contention")
		must(t, l.Release())
		r, e := Mutate(h, func(s *state.InstallState) error { s.InstalledAgents = []string{"agent"}; return nil })
		s, re := state.Read(h)
		require(t, r.Outcome == Committed && e == nil && re == nil && s.Persona == "kept" && len(s.InstalledAgents) == 1, "retry preserves distinct fields")
	})
	t.Run("callback reentry", func(t *testing.T) {
		h := t.TempDir()
		r, e := Mutate(h, func(*state.InstallState) error {
			inner, ie := Mutate(h, func(*state.InstallState) error { return nil })
			require(t, errors.Is(ie, reviewtransaction.ErrStoreLockContended) && inner.Outcome == Uncommitted, "callback reentry")
			return nil
		})
		require(t, r.Outcome == Committed && e == nil, "callback reentry")
	})
}
func contentionCase(t *testing.T, call func(string, Mutator) (Result, error)) {
	h := t.TempDir()
	l, e := reviewtransaction.AcquireAuthorityFileLock(state.Path(h) + ".lock")
	must(t, e)
	defer l.Release()
	called := false
	r, e := call(h, func(*state.InstallState) error { called = true; return nil })
	require(t, errors.Is(e, reviewtransaction.ErrStoreLockContended) && r.Outcome == Uncommitted && !called, "contention refusal")
}
