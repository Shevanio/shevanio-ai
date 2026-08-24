package cli

import (
	"testing"

	"github.com/shevanio/shevanio-ai/v2/internal/model"
	"github.com/shevanio/shevanio-ai/v2/internal/planner"
	"github.com/shevanio/shevanio-ai/v2/internal/verify"
)

func TestWithPostInstallNotesDoesNotChangeUnrelatedComponents(t *testing.T) {
	// Set GOBIN and PATH to the same directory so that withGoInstallPathNote
	// detects that GOBIN is already in PATH and does not append a guidance note.
	gobin := "/usr/local/bin"
	t.Setenv("GOBIN", gobin)
	t.Setenv("PATH", gobin)

	report := verify.Report{Ready: true, FinalNote: "You're ready."}
	resolved := planner.ResolvedPlan{OrderedComponents: []model.ComponentID{model.ComponentEngram}}

	updated := withPostInstallNotes(report, resolved)
	if updated.FinalNote != report.FinalNote {
		t.Fatalf("FinalNote changed unexpectedly: %q", updated.FinalNote)
	}
}
