package screens

import (
	"strings"
	"testing"

	"github.com/shevanio/shevanio-ai/v2/internal/model"
)

func TestPersonaOptionsExcludeGentlemanNeutralArtifacts(t *testing.T) {
	for _, option := range PersonaOptions() {
		if option == model.PersonaGentlemanNeutralArtifacts || option == model.PersonaGentleman {
			t.Fatalf("PersonaOptions() still offers a legacy alias %q", option)
		}
	}
}

// TestPersonaDescriptionsNeverReuseNeutral pins the invariant issue #833 is
// actually about: "neutral" named two unrelated axes at once, artifact language
// and conversational tone, so the selector could not be read unambiguously.
// Asserting the negative space is what keeps that ambiguity from returning; a
// positive assertion against the description constants cannot, because it
// passes for any wording those constants happen to hold, including a
// reintroduced "neutral".
func TestPersonaDescriptionsNeverReuseNeutral(t *testing.T) {
	for persona, description := range personaDescriptions {
		for _, field := range strings.Fields(strings.ToLower(description)) {
			if strings.Trim(field, ".,;:()") == "neutral" {
				t.Fatalf("persona %q description reuses the ambiguous word %q: %q", persona, "neutral", description)
			}
		}
	}
}

// TestPersonaDescriptionsSeparateToneFromArtifactLanguage asserts the two axes
// stay distinguishable: only the Gentleman personas claim a regional
// conversational tone, and every managed persona states that technical
// artifacts are English. Collapsing the descriptions into each other, or giving
// the neutral persona a regional tone, fails here.
func TestPersonaDescriptionsSeparateToneFromArtifactLanguage(t *testing.T) {
	managed := []model.PersonaID{
		model.PersonaShevanio,
		model.PersonaGentlemanNeutralArtifacts,
		model.PersonaNeutral,
	}
	for _, persona := range managed {
		description, ok := personaDescriptions[persona]
		if !ok {
			t.Fatalf("persona %q has no description", persona)
		}
		if !strings.Contains(strings.ToLower(description), "english technical artifacts") {
			t.Fatalf("persona %q must state that technical artifacts are English: %q", persona, description)
		}
		mentionsVoseo := strings.Contains(strings.ToLower(description), "voseo")
		isGentleman := false
		if mentionsVoseo != isGentleman {
			t.Fatalf("persona %q voseo claim = %v, want %v (only Gentleman personas carry a regional tone): %q",
				persona, mentionsVoseo, isGentleman, description)
		}
	}

	if personaDescriptions[model.PersonaNeutral] == personaDescriptions[model.PersonaGentlemanNeutralArtifacts] {
		t.Fatal("the neutral persona and its legacy alias must stay distinguishable in the review label")
	}
}

// TestRenderPersonaShowsEveryManagedDescription keeps the selector itself
// honest: each managed persona's description must actually reach the rendered
// screen.
func TestRenderPersonaShowsEveryManagedDescription(t *testing.T) {
	for _, persona := range []model.PersonaID{
		model.PersonaShevanio,
		model.PersonaNeutral,
	} {
		out := RenderPersona(persona, 0)
		if !strings.Contains(out, personaDescriptions[persona]) {
			t.Fatalf("RenderPersona(%q) does not show its description %q; output:\n%s",
				persona, personaDescriptions[persona], out)
		}
	}
}

// TestReviewPersonaLabelKeepsThePersonaID guards the confirm-before-write
// screen: the reader must be able to see the exact persona value that will be
func TestReviewPersonaLabelKeepsThePersonaID(t *testing.T) {
	cases := []struct {
		input model.PersonaID
		want  model.PersonaID
	}{
		{input: model.PersonaGentleman, want: model.PersonaShevanio},
		{input: model.PersonaGentlemanNeutralArtifacts, want: model.PersonaGentlemanNeutralArtifacts},
		{input: model.PersonaNeutral, want: model.PersonaNeutral},
	}
	for _, tt := range cases {
		label := reviewPersonaLabel(tt.input)
		if !strings.Contains(label, string(tt.want)) {
			t.Fatalf("reviewPersonaLabel(%q) = %q, must contain effective persona ID %q", tt.input, label, tt.want)
		}
		if !strings.Contains(label, personaDescriptions[tt.want]) {
			t.Fatalf("reviewPersonaLabel(%q) = %q, must contain effective description", tt.input, label)
		}
	}
}
