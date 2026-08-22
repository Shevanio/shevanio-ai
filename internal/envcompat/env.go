package envcompat

import (
	"fmt"
	"os"
)

const (
	currentPrefix = "SHEVANIO_AI_"
	legacyPrefix  = "GENTLE_AI_"
)

// supportedSuffixes is intentionally explicit. Removed or unknown legacy
// switches must not silently regain authority in the new runtime.
var supportedSuffixes = []string{
	"AUTHORITY_FIRST_TERMINAL_PROCEDURE",
	"CHANNEL",
	"CLAUDE_REVIEW_CONTEXT",
	"CODEX_REVIEWER_LOOPBACK_BASE_URL",
	"ENGRAM_SETUP_MODE",
	"ENGRAM_SETUP_STRICT",
	"INSTALL_SCOPE",
	"NO_SELF_UPDATE",
	"OPENCODE_BACKGROUND_SUBAGENTS",
	"PI_BACKGROUND_SUBAGENTS",
	"RDD_NEW_LINEAGE",
	"RDD_SHADOW",
	"REVIEW_BINDING",
	"REVIEW_CONTEXT",
	"REVIEW_CONTEXT_END",
	"REVIEW_INSTRUCTION",
	"REVIEW_NAME_STATUS",
	"REVIEW_NUMSTAT",
	"REVIEW_PATCH",
	"REVIEW_PROVIDER_MATERIALIZATION",
	"REVIEW_PROVIDER_TASK",
	"REVIEW_RESULT_SCHEMA",
	"RUNTIME_AGENT_ID",
	"SDD_STATUS_ENGRAM",
	"SELF_UPDATE_DONE",
	"YES",
}

// ApplyLegacyFallbacks imports supported legacy values only when the canonical
// variable is absent. Canonical variables win even when explicitly empty.
func ApplyLegacyFallbacks() error {
	for _, suffix := range supportedSuffixes {
		current := currentPrefix + suffix
		if _, exists := os.LookupEnv(current); exists {
			continue
		}
		legacy := legacyPrefix + suffix
		value, exists := os.LookupEnv(legacy)
		if !exists {
			continue
		}
		if err := os.Setenv(current, value); err != nil {
			return fmt.Errorf("import legacy environment variable %s: %w", legacy, err)
		}
	}
	return nil
}
