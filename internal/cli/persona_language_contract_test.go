package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/shevanio/shevanio-ai/v2/internal/model"
	"github.com/shevanio/shevanio-ai/v2/internal/system"
)

func TestNormalizePersonaRemapsGentlemanNeutralArtifacts(t *testing.T) {
	got, remapped, err := normalizePersona("gentleman-neutral-artifacts")
	if err != nil {
		t.Fatalf("normalizePersona() error = %v", err)
	}
	if got != model.PersonaNeutral {
		t.Fatalf("normalizePersona() = %q, want %q", got, model.PersonaNeutral)
	}
	if !remapped {
		t.Fatal("normalizePersona() remapped = false, want true for the legacy alias")
	}
}

func TestNormalizeManagedIdentityInputs(t *testing.T) {
	personaCases := []struct {
		input string
		want  model.PersonaID
		ok    bool
	}{
		{input: "", want: model.PersonaShevanio, ok: true},
		{input: "shevanio", want: model.PersonaShevanio, ok: true},
		{input: "Shevanio", want: model.PersonaShevanio, ok: true},
		{input: "gentleman", want: model.PersonaShevanio, ok: true},
		{input: "Gentleman", want: model.PersonaShevanio, ok: true},
		{input: "SHEVANIO"},
		{input: "Shevanio "},
	}
	for _, tt := range personaCases {
		t.Run("persona/"+tt.input, func(t *testing.T) {
			got, remapped, err := normalizePersona(tt.input)
			if (err == nil) != tt.ok || got != tt.want || remapped {
				t.Fatalf("normalizePersona(%q) = %q, %v, %v; want %q, false, success=%v", tt.input, got, remapped, err, tt.want, tt.ok)
			}
		})
	}

	presetCases := []struct {
		input string
		want  model.PresetID
		ok    bool
	}{
		{input: "", want: model.PresetFullShevanio, ok: true},
		{input: "full-shevanio", want: model.PresetFullShevanio, ok: true},
		{input: "full-gentleman", want: model.PresetFullShevanio, ok: true},
		{input: "custom", want: model.PresetCustom, ok: true},
		{input: "Full-shevanio"},
		{input: "full-shevanio-extra"},
	}
	for _, tt := range presetCases {
		t.Run("preset/"+tt.input, func(t *testing.T) {
			got, err := normalizePreset(tt.input)
			if (err == nil) != tt.ok || got != tt.want {
				t.Fatalf("normalizePreset(%q) = %q, %v; want %q, success=%v", tt.input, got, err, tt.want, tt.ok)
			}
		})
	}
}

func TestNormalizeInstallFlagsPrintsAliasRemapNotice(t *testing.T) {
	var buf bytes.Buffer
	previous := personaNoticeWriter
	personaNoticeWriter = &buf
	defer func() { personaNoticeWriter = previous }()

	input, err := NormalizeInstallFlags(InstallFlags{Persona: "gentleman-neutral-artifacts"}, system.DetectionResult{})
	if err != nil {
		t.Fatalf("NormalizeInstallFlags() error = %v", err)
	}
	if input.Selection.Persona != model.PersonaNeutral {
		t.Fatalf("Selection.Persona = %q, want %q", input.Selection.Persona, model.PersonaNeutral)
	}
	if !strings.Contains(buf.String(), personaAliasRemapNotice) {
		t.Fatalf("notice not printed; got %q", buf.String())
	}
}
