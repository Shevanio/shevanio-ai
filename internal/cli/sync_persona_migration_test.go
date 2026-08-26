package cli

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/shevanio/shevanio-ai/v2/internal/model"
	"github.com/shevanio/shevanio-ai/v2/internal/state"
)

var errStateUnreadableForTest = errors.New("state unreadable")

func TestMigratePersistedPersonaAliasRewritesStateOnce(t *testing.T) {
	var buf bytes.Buffer
	previous := personaNoticeWriter
	personaNoticeWriter = &buf
	defer func() { personaNoticeWriter = previous }()

	homeDir := t.TempDir()
	persisted := state.InstallState{Persona: string(model.PersonaGentlemanNeutralArtifacts)}
	if err := state.Write(homeDir, persisted); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	if err := migratePersistedPersonaAlias(homeDir, &persisted, nil); err != nil {
		t.Fatalf("migratePersistedPersonaAlias() error = %v", err)
	}

	reread, err := state.Read(homeDir)
	if err != nil {
		t.Fatalf("re-read state: %v", err)
	}
	if reread.Persona != string(model.PersonaNeutral) {
		t.Fatalf("persisted persona = %q, want %q", reread.Persona, model.PersonaNeutral)
	}
	if !strings.Contains(buf.String(), personaAliasRemapNotice) {
		t.Fatalf("notice not printed; got %q", buf.String())
	}

	// Second run: state already neutral — no notice, no rewrite.
	buf.Reset()
	if err := migratePersistedPersonaAlias(homeDir, &reread, nil); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("second run printed %q, want silence", buf.String())
	}
}

func TestCanonicalPersonaStateMigration(t *testing.T) {
	profile := model.Profile{Name: "review-safe"}
	assignments := map[string]model.ModelAssignment{
		"sdd-apply": {ProviderID: "openai", ModelID: "gpt-5.6"},
	}
	tests := []struct {
		name        string
		selection   model.Selection
		persisted   state.InstallState
		wantPersona model.PersonaID
		wantPreset  model.PresetID
		wantMode    model.SDDModeID
	}{
		{
			name:        "empty managed defaults converge",
			selection:   model.Selection{},
			persisted:   state.InstallState{},
			wantPersona: model.PersonaShevanio,
			wantPreset:  model.PresetFullShevanio,
		},
		{
			name: "legacy selection converges without losing policy",
			selection: model.Selection{
				Profiles:         []model.Profile{profile},
				ModelAssignments: assignments,
				Persona:          model.PersonaNeutral,
			},
			persisted: state.InstallState{
				SelectionConfigured: true,
				Preset:              model.PresetFullGentleman,
				Persona:             "Gentleman",
				SDDMode:             model.SDDModeMulti,
			},
			wantPersona: model.PersonaNeutral,
			wantPreset:  model.PresetFullShevanio,
			wantMode:    model.SDDModeMulti,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selection := tt.selection
			RestorePersistedSelection(&selection, tt.persisted, SyncFlags{})
			applyResolvedPersona(&selection, tt.persisted.Persona)
			if selection.Persona != tt.wantPersona || selection.Preset != tt.wantPreset || selection.SDDMode != tt.wantMode {
				t.Fatalf("selection identity/state = %+v, want %q/%q/%q", selection, tt.wantPersona, tt.wantPreset, tt.wantMode)
			}
			if !reflect.DeepEqual(selection.Profiles, tt.selection.Profiles) || !reflect.DeepEqual(selection.ModelAssignments, tt.selection.ModelAssignments) {
				t.Fatalf("selection policy fields changed: %+v/%v", selection.Profiles, selection.ModelAssignments)
			}
		})
	}
}
func TestMigratePersistedPersonaAliasSkipsUnreadableState(t *testing.T) {
	persisted := state.InstallState{Persona: string(model.PersonaGentlemanNeutralArtifacts)}
	if err := migratePersistedPersonaAlias(t.TempDir(), &persisted, errStateUnreadableForTest); err != nil {
		t.Fatalf("migrate with read error must be a no-op, got %v", err)
	}
}

// TestApplyResolvedPersonaAliasResolution covers only what this change owns:
// a persisted legacy alias resolves to its canonical identity, valid persisted
// personas are honored or canonicalized, an explicit selection wins over persisted state, and the
// documented missing-field compatibility default still applies to state files
// written before persona persistence existed.
//
// Resolution of unknown or unreadable persisted state is deliberately NOT
// asserted here. That policy belongs to issue #1677, which makes it fail
// closed; pinning today's fail-open behavior would codify a contract this
// change does not intend to define.
func TestApplyResolvedPersonaAliasResolution(t *testing.T) {
	cases := []struct {
		name      string
		selection model.Selection
		persisted string
		want      model.PersonaID
	}{
		{name: "persisted legacy alias resolves to neutral", persisted: string(model.PersonaGentlemanNeutralArtifacts), want: model.PersonaNeutral},
		{name: "persisted gentleman alias resolves to shevanio", persisted: string(model.PersonaGentleman), want: model.PersonaShevanio},
		{name: "persisted neutral is honored", persisted: string(model.PersonaNeutral), want: model.PersonaNeutral},
		{name: "explicit selection wins over persisted alias", selection: model.Selection{Persona: model.PersonaGentleman}, persisted: string(model.PersonaGentlemanNeutralArtifacts), want: model.PersonaGentleman},
		{name: "missing persona field uses the canonical managed default", persisted: "", want: model.PersonaShevanio},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			selection := tc.selection
			applyResolvedPersona(&selection, tc.persisted)
			if selection.Persona != tc.want {
				t.Fatalf("applyResolvedPersona(persisted=%q) = %q, want %q", tc.persisted, selection.Persona, tc.want)
			}
		})
	}
}

// TestRunSyncWithSelectionMigratesAliasOnNoOpSync pins the early-return path:
// a sync that discovers zero agents must still migrate a persisted legacy
// alias, otherwise the one-time migration never fires for those users.
func TestRunSyncWithSelectionMigratesAliasOnNoOpSync(t *testing.T) {
	var buf bytes.Buffer
	previous := personaNoticeWriter
	personaNoticeWriter = &buf
	defer func() { personaNoticeWriter = previous }()

	homeDir := t.TempDir()
	if err := state.Write(homeDir, state.InstallState{Persona: string(model.PersonaGentlemanNeutralArtifacts)}); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	result, err := RunSyncWithSelection(homeDir, model.Selection{})
	if err != nil {
		t.Fatalf("RunSyncWithSelection() error = %v", err)
	}
	if !result.NoOp {
		t.Fatal("zero-agent sync must report NoOp")
	}

	reread, err := state.Read(homeDir)
	if err != nil {
		t.Fatalf("re-read state: %v", err)
	}
	if reread.Persona != string(model.PersonaNeutral) {
		t.Fatalf("persisted persona = %q, want %q after no-op sync", reread.Persona, model.PersonaNeutral)
	}
	if !strings.Contains(buf.String(), personaAliasRemapNotice) {
		t.Fatalf("notice not printed on no-op sync; got %q", buf.String())
	}
}
