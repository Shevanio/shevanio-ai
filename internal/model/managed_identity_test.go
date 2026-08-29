package model

import "testing"

func TestManagedIdentityAliases(t *testing.T) {
	if got, want := CanonicalManagedIdentity, (ManagedIdentity{
		Actor:    "shevanio-orchestrator",
		Persona:  PersonaShevanio,
		Display:  "Shevanio",
		Resource: "Shevanio",
		Preset:   PresetFullShevanio,
	}); got != want {
		t.Fatalf("canonical identity = %#v, want %#v", got, want)
	}

	tests := []struct {
		name      string
		input     string
		want      PersonaID
		wantClass IdentityClass
	}{
		{name: "persona canonical id", input: "shevanio", want: PersonaShevanio, wantClass: IdentityCanonicalManaged},
		{name: "persona canonical display", input: "Shevanio", want: PersonaShevanio, wantClass: IdentityCanonicalManaged},
		{name: "persona legacy lowercase", input: "gentleman", want: PersonaShevanio, wantClass: IdentityLegacyManaged},
		{name: "persona legacy display", input: "Gentleman", want: PersonaShevanio, wantClass: IdentityLegacyManaged},
		{name: "persona near miss", input: "SHEVANIO", want: PersonaID("SHEVANIO"), wantClass: IdentityUnknown},
		{name: "persona near miss legacy", input: "GENTLEMAN", want: PersonaID("GENTLEMAN"), wantClass: IdentityUnknown},
		{name: "persona unknown", input: "custom-owner", want: PersonaID("custom-owner"), wantClass: IdentityUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotClass := NormalizePersonaRead(tt.input)
			if got != tt.want || gotClass != tt.wantClass {
				t.Fatalf("NormalizePersonaRead(%q) = %q, %q; want %q, %q", tt.input, got, gotClass, tt.want, tt.wantClass)
			}
		})
	}

	orchestratorTests := []struct {
		name      string
		input     string
		want      string
		wantClass IdentityClass
	}{
		{name: "orchestrator canonical", input: "shevanio-orchestrator", want: "shevanio-orchestrator", wantClass: IdentityCanonicalManaged},
		{name: "orchestrator legacy gentle", input: "gentle-orchestrator", want: "shevanio-orchestrator", wantClass: IdentityLegacyManaged},
		{name: "orchestrator legacy sdd", input: "sdd-orchestrator", want: "shevanio-orchestrator", wantClass: IdentityLegacyManaged},
		{name: "orchestrator near miss", input: "Gentle-Orchestrator", want: "Gentle-Orchestrator", wantClass: IdentityUnknown},
		{name: "orchestrator near miss sdd", input: "Sdd-Orchestrator", want: "Sdd-Orchestrator", wantClass: IdentityUnknown},
		{name: "orchestrator unknown", input: "user-orchestrator", want: "user-orchestrator", wantClass: IdentityUnknown},
	}
	for _, tt := range orchestratorTests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotClass := NormalizeOrchestratorRead(tt.input)
			if got != tt.want || gotClass != tt.wantClass {
				t.Fatalf("NormalizeOrchestratorRead(%q) = %q, %q; want %q, %q", tt.input, got, gotClass, tt.want, tt.wantClass)
			}
		})
	}

	presetTests := []struct {
		name      string
		input     string
		want      PresetID
		wantClass IdentityClass
	}{
		{name: "preset canonical", input: "full-shevanio", want: PresetFullShevanio, wantClass: IdentityCanonicalManaged},
		{name: "preset legacy", input: "full-gentleman", want: PresetFullShevanio, wantClass: IdentityLegacyManaged},
		{name: "preset near miss", input: "FULL-GENTLEMAN", want: PresetID("FULL-GENTLEMAN"), wantClass: IdentityUnknown},
		{name: "preset unknown", input: "custom-preset", want: PresetID("custom-preset"), wantClass: IdentityUnknown},
	}
	for _, tt := range presetTests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotClass := NormalizePresetRead(tt.input)
			if got != tt.want || gotClass != tt.wantClass {
				t.Fatalf("NormalizePresetRead(%q) = %q, %q; want %q, %q", tt.input, got, gotClass, tt.want, tt.wantClass)
			}
		})
	}
}

func TestManagedIdentityNearMissesRemainUnknown(t *testing.T) {
	if got, class := NormalizePersonaRead("Shevanio "); got != PersonaID("Shevanio ") || class != IdentityUnknown {
		t.Fatalf("persona near miss = %q, %q", got, class)
	}
	if got, class := NormalizeOrchestratorRead("sdd-Orchestrator"); got != "sdd-Orchestrator" || class != IdentityUnknown {
		t.Fatalf("orchestrator near miss = %q, %q", got, class)
	}
	if got, class := NormalizePresetRead(""); got != PresetID("") || class != IdentityUnknown {
		t.Fatalf("empty preset = %q, %q", got, class)
	}
}
