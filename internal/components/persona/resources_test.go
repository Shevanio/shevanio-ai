package persona_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/shevanio/shevanio-ai/v2/internal/components/persona"
	"github.com/shevanio/shevanio-ai/v2/internal/model"
)

func TestResourcePlanOutputStylePaths(t *testing.T) {
	dir := t.TempDir()
	shevanio := filepath.Join(dir, "shevanio.md")
	gentleman := filepath.Join(dir, "gentleman.md")
	neutral := filepath.Join(dir, "neutral.md")

	tests := []struct {
		name    string
		persona model.PersonaID
		want    persona.OutputStylePaths
	}{
		{name: "shevanio writes the canonical style", persona: model.PersonaShevanio, want: persona.OutputStylePaths{Write: shevanio, Backup: []string{shevanio, neutral, gentleman}}},
		{
			name:    "legacy gentleman emits the canonical shevanio style",
			persona: model.PersonaGentleman,
			want: persona.OutputStylePaths{
				Write:  shevanio,
				Backup: []string{shevanio, neutral, gentleman},
			},
		},
		{
			name:    "neutral removes canonical and legacy regional styles",
			persona: model.PersonaNeutral,
			want: persona.OutputStylePaths{
				Write:  neutral,
				Backup: []string{shevanio, neutral, gentleman},
				Remove: []string{shevanio, gentleman},
			},
		},
		{
			name:    "legacy neutral alias writes neutral and removes retired gentleman",
			persona: model.PersonaGentlemanNeutralArtifacts,
			want: persona.OutputStylePaths{
				Write:  neutral,
				Backup: []string{shevanio, neutral, gentleman},
				Remove: []string{shevanio, gentleman},
			},
		},
		{
			name:    "custom manages no output styles",
			persona: model.PersonaCustom,
			want:    persona.OutputStylePaths{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := persona.ResourcePlanFor(tt.persona).OutputStylePaths(dir)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ResourcePlanFor(%q).OutputStylePaths() = %#v, want %#v", tt.persona, got, tt.want)
			}
		})
	}
}
