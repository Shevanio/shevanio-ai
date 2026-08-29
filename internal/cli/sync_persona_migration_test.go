package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/shevanio/shevanio-ai/v2/internal/model"
	"github.com/shevanio/shevanio-ai/v2/internal/state"
)

// TestApplyResolvedPersonaAliasResolution covers only what this change owns:
// a persisted legacy alias resolves to neutral, valid persisted personas are
// honored unchanged, an explicit selection wins over persisted state, and the
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
		{name: "persisted gentleman is honored", persisted: string(model.PersonaGentleman), want: model.PersonaGentleman},
		{name: "persisted neutral is honored", persisted: string(model.PersonaNeutral), want: model.PersonaNeutral},
		{name: "explicit selection wins over persisted alias", selection: model.Selection{Persona: model.PersonaGentleman}, persisted: string(model.PersonaGentlemanNeutralArtifacts), want: model.PersonaGentleman},
		{name: "missing persona field uses the documented compatibility default", persisted: "", want: model.PersonaNeutral},
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

// TestRunSyncWithSelectionPublishesAliasOnZeroAgentNoOp pins the early-return path:
// a sync that discovers zero agents must still migrate a persisted legacy alias.
func TestRunSyncWithSelectionPublishesAliasOnZeroAgentNoOp(t *testing.T) {
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
	if !bytes.Equal(gotLegacy, legacy) || legacyInfo.Mode().Perm() != 0o600 {
		t.Fatal("alias-only publication changed legacy fallback")
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
