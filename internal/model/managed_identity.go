package model

// ManagedIdentity is the current identity written by Shevanio AI.
type ManagedIdentity struct {
	Actor    string
	Persona  PersonaID
	Display  string
	Resource string
	Preset   PresetID
}

const canonicalOrchestrator = "shevanio-orchestrator"

// CanonicalManagedIdentity is the sole current managed identity.
var CanonicalManagedIdentity = ManagedIdentity{
	Actor:    canonicalOrchestrator,
	Persona:  PersonaShevanio,
	Display:  "Shevanio",
	Resource: "Shevanio",
	Preset:   PresetFullShevanio,
}

// IdentityClass describes how an identity value relates to the managed contract.
type IdentityClass string

const (
	IdentityCanonicalManaged IdentityClass = "canonical-managed"
	IdentityLegacyManaged    IdentityClass = "legacy-managed"
	IdentityUserManaged      IdentityClass = "user-managed"
	IdentityUnknown          IdentityClass = "unknown"
)

// NormalizePersonaRead maps exact canonical and legacy persona inputs to the
// canonical persona ID. Unknown values are preserved without case folding.
func NormalizePersonaRead(value string) (PersonaID, IdentityClass) {
	switch value {
	case string(PersonaShevanio), "Shevanio":
		return PersonaShevanio, IdentityCanonicalManaged
	case string(PersonaGentleman), "Gentleman":
		return PersonaShevanio, IdentityLegacyManaged
	default:
		return PersonaID(value), IdentityUnknown
	}
}

// NormalizeOrchestratorRead maps exact canonical and legacy actor inputs to
// the canonical actor. Unknown values are preserved without case folding.
func NormalizeOrchestratorRead(value string) (string, IdentityClass) {
	switch value {
	case canonicalOrchestrator:
		return canonicalOrchestrator, IdentityCanonicalManaged
	case "gentle-orchestrator", "sdd-orchestrator":
		return canonicalOrchestrator, IdentityLegacyManaged
	default:
		return value, IdentityUnknown
	}
}

// NormalizePresetRead maps exact canonical and legacy preset inputs to the
// canonical preset. Unknown values are preserved without case folding.
func NormalizePresetRead(value string) (PresetID, IdentityClass) {
	switch value {
	case string(PresetFullShevanio):
		return PresetFullShevanio, IdentityCanonicalManaged
	case string(PresetFullGentleman):
		return PresetFullShevanio, IdentityLegacyManaged
	default:
		return PresetID(value), IdentityUnknown
	}
}
