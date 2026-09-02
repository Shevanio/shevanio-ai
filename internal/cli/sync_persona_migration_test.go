package cli

import (
	"bytes"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/shevanio/shevanio-ai/v2/internal/model"
	"github.com/shevanio/shevanio-ai/v2/internal/state"
)

// TestApplyResolvedPersonaAliasResolution covers only what this change owns:
// persisted managed aliases resolve to canonical values, custom personas remain
// unchanged, an explicit selection wins over persisted state, and the
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
		{name: "persisted legacy managed ID resolves to shevanio", persisted: string(model.PersonaGentleman), want: model.PersonaShevanio},
		{name: "persisted legacy managed display resolves to shevanio", persisted: "Gentleman", want: model.PersonaShevanio},
		{name: "persisted neutral is honored", persisted: string(model.PersonaNeutral), want: model.PersonaNeutral},
		{name: "persisted custom is honored", persisted: string(model.PersonaCustom), want: model.PersonaCustom},
		{name: "explicit managed alias wins and normalizes", selection: model.Selection{Persona: model.PersonaGentleman}, persisted: string(model.PersonaNeutral), want: model.PersonaShevanio},
		{name: "explicit unknown is preserved", selection: model.Selection{Persona: "user-persona"}, persisted: string(model.PersonaNeutral), want: "user-persona"},
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

func TestRestorePersistedSelectionIdentityBoundary(t *testing.T) {
	cases := []struct {
		name        string
		persisted   state.InstallState
		wantPersona model.PersonaID
		wantPreset  model.PresetID
	}{
		{name: "legacy fallback", wantPersona: model.PersonaShevanio, wantPreset: model.PresetFullShevanio},
		{name: "legacy managed values", persisted: state.InstallState{SelectionConfigured: true, Persona: "Gentleman", Preset: model.PresetFullGentleman}, wantPersona: model.PersonaShevanio, wantPreset: model.PresetFullShevanio},
		{name: "custom values", persisted: state.InstallState{SelectionConfigured: true, Persona: "custom", Preset: model.PresetCustom}, wantPersona: model.PersonaCustom, wantPreset: model.PresetCustom},
		{name: "unknown preset", persisted: state.InstallState{SelectionConfigured: true, Persona: "neutral", Preset: "user-preset"}, wantPersona: model.PersonaNeutral, wantPreset: "user-preset"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			selection := model.Selection{}
			RestorePersistedSelection(&selection, tt.persisted, SyncFlags{})
			applyResolvedPersona(&selection, tt.persisted.Persona)
			if selection.Persona != tt.wantPersona || selection.Preset != tt.wantPreset {
				t.Fatalf("selection identity = %q/%q, want %q/%q", selection.Persona, selection.Preset, tt.wantPersona, tt.wantPreset)
			}
		})
	}
}

// TestRunSyncWithSelectionPublishesAliasesOnZeroAgentNoOp pins the early-return
// path: a sync that discovers zero agents must still converge managed aliases.
func TestRunSyncWithSelectionPublishesAliasesOnZeroAgentNoOp(t *testing.T) {
	var buf bytes.Buffer
	previous := personaNoticeWriter
	personaNoticeWriter = &buf
	defer func() { personaNoticeWriter = previous }()

	homeDir := t.TempDir()
	if err := state.Write(homeDir, state.InstallState{Persona: string(model.PersonaGentlemanNeutralArtifacts), InstalledBinaryVersion: "old", ManagedAssetDigest: "old"}); err != nil {
		t.Fatalf("seed state: %v", err)
	}
	legacyPath := homeDir + "/.gentle-ai/state.json"
	legacy := []byte(`{"persona":"legacy-only","unknown":true}
`)
	mustWrite(t, legacyPath, legacy, 0o600)
	result, err := RunSyncWithSelection(homeDir, model.Selection{})
	if err != nil {
		t.Fatalf("RunSyncWithSelection() error = %v", err)
	}
	if !result.NoOp {
		t.Fatal("zero-agent sync must report NoOp")
	}
	reread := mustPersonaState(t, homeDir)
	if reread.Persona != string(model.PersonaNeutral) {
		t.Fatalf("persisted persona = %q, want %q after no-op sync", reread.Persona, model.PersonaNeutral)
	}
	if reread.InstalledBinaryVersion != "old" || reread.ManagedAssetDigest != "old" || strings.Count(buf.String(), personaAliasRemapNotice) != 1 {
		t.Fatalf("alias-only publication changed provenance or notice count: state=%#v notice=%q", reread, buf.String())
	}
	gotLegacy := mustPersonaFile(t, legacyPath)
	legacyInfo, err := os.Stat(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotLegacy, legacy) {
		t.Fatal("alias-only publication changed legacy fallback bytes")
	}
	if runtime.GOOS != "windows" && legacyInfo.Mode().Perm() != 0o600 {
		t.Fatal("alias-only publication changed legacy fallback mode")
	}

	managed := t.TempDir()
	canonicalAssignment := state.ModelAssignmentState{ProviderID: "canonical", ModelID: "wins"}
	if err := state.Write(managed, state.InstallState{
		Persona: "Gentleman", Preset: model.PresetFullGentleman,
		ModelAssignments: map[string]state.ModelAssignmentState{
			model.CanonicalManagedIdentity.Actor: canonicalAssignment,
			"sdd-orchestrator":                   {ProviderID: "legacy", ModelID: "remove"},
			"sdd-orchestrator-team":              {ProviderID: "custom", ModelID: "keep"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	result, err = RunSyncWithSelection(managed, model.Selection{})
	converged := mustPersonaState(t, managed)
	if err != nil || !result.NoOp || converged.Persona != string(model.PersonaShevanio) || converged.Preset != model.PresetFullShevanio || converged.ModelAssignments[model.CanonicalManagedIdentity.Actor] != canonicalAssignment {
		t.Fatalf("managed alias convergence = result=%#v state=%#v err=%v", result, converged, err)
	}
	if _, exists := converged.ModelAssignments["sdd-orchestrator"]; exists || converged.ModelAssignments["sdd-orchestrator-team"].ModelID != "keep" {
		t.Fatalf("managed alias convergence changed custom or retained legacy assignment: %#v", converged.ModelAssignments)
	}

	noAlias := t.TempDir()
	if err := state.Write(noAlias, state.InstallState{Persona: string(model.PersonaNeutral)}); err != nil {
		t.Fatal(err)
	}
	noAliasBefore, noAliasMode := personaStateSnapshot(t, noAlias)
	buf.Reset()
	result, err = RunSyncWithSelection(noAlias, model.Selection{})
	if err != nil || !result.NoOp {
		t.Fatalf("true no-op = result=%#v err=%v", result, err)
	}
	noAliasAfter, gotNoAliasMode := personaStateSnapshot(t, noAlias)
	if !bytes.Equal(noAliasAfter, noAliasBefore) || gotNoAliasMode != noAliasMode || buf.Len() != 0 {
		t.Fatal("true no-op wrote state or emitted a notice")
	}
}
