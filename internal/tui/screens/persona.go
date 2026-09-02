package screens

import (
	"strings"

	"github.com/shevanio/shevanio-ai/v2/internal/model"
	"github.com/shevanio/shevanio-ai/v2/internal/tui/styles"
)

func PersonaOptions() []model.PersonaID {
	return []model.PersonaID{model.CanonicalManagedIdentity.Persona, model.PersonaNeutral, model.PersonaCustom}
}

var personaDescriptions = map[model.PersonaID]string{
	model.CanonicalManagedIdentity.Persona: "Mentor conversation; English technical artifacts",
	// The legacy alias is remapped at normalization time and no longer offered
	// in the picker; the entry stays so the review screen can label persisted
	// state that has not been migrated yet.
	model.PersonaGentlemanNeutralArtifacts: "No regional conversation tone; English technical artifacts (legacy alias, remapped)",
	model.PersonaNeutral:                   "No regional conversation tone; English technical artifacts",
	model.PersonaCustom:                    "Do not install a managed persona; choose themes/logo on the next screens",
}

var personaLabels = map[model.PersonaID]string{
	model.CanonicalManagedIdentity.Persona: model.CanonicalManagedIdentity.Display,
}

func RenderPersona(selected model.PersonaID, cursor int) string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Choose your Persona"))
	b.WriteString("\n\n")
	b.WriteString(styles.SubtextStyle.Render(model.CanonicalManagedIdentity.Display + " teaches before it solves."))
	b.WriteString("\n\n")

	for idx, persona := range PersonaOptions() {
		isSelected := persona == selected
		focused := idx == cursor
		label := personaLabels[persona]
		if label == "" {
			label = string(persona)
		}
		b.WriteString(renderRadio(label, isSelected, focused))
		b.WriteString(styles.SubtextStyle.Render("    " + personaDescriptions[persona]))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(renderOptions([]string{"Back"}, cursor-len(PersonaOptions())))
	b.WriteString("\n")
	b.WriteString(styles.HelpStyle.Render("j/k: navigate • enter: select • esc: back"))

	return b.String()
}
