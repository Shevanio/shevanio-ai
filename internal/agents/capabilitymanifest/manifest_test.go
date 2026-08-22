package capabilitymanifest

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/shevanio/shevanio-ai/v2/internal/model"
)

func TestCanonicalImplementationRoutingBoundaries(t *testing.T) {
	t.Parallel()

	got := CanonicalImplementationRouting()
	want := ImplementationRoutingFacts{
		DirectInline: DirectInlineFacts{
			MinUnderstandingFiles:                    1,
			MaxUnderstandingFiles:                    3,
			MaxMechanicalWriteFiles:                  1,
			MechanicalWriteMustBeAlreadyUnderstood:   true,
			MechanicalWriteMustNotRequireResearch:    true,
			MechanicalWriteMustNotHaveOpenDesignWork: true,
		},
		DelegatedDirect: DelegatedDirectFacts{
			MappingMinUnderstandingFiles:  4,
			WriterMinNonTrivialFiles:      2,
			DelegateWhenReadPreparesWrite: true,
			DelegateWhenBroadResearch:     true,
		},
		SDD: SDDProposalFacts{
			ProposeWhenSubstantialOrAmbiguous:     true,
			DurableArtifactsMustReduceUncertainty: true,
			SelectionPolicy:                       SDDSelectionExplicitRequestOrAcceptedProposal,
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CanonicalImplementationRouting() = %#v, want %#v", got, want)
	}
}

func TestManifestRejectsWeakenedRoutingFacts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		weaken func(*AgentCapabilityManifest)
	}{
		{
			name: "direct understanding starts below one file",
			weaken: func(manifest *AgentCapabilityManifest) {
				manifest.ImplementationRouting.DirectInline.MinUnderstandingFiles = 0
			},
		},
		{
			name: "direct understanding exceeds three files",
			weaken: func(manifest *AgentCapabilityManifest) {
				manifest.ImplementationRouting.DirectInline.MaxUnderstandingFiles = 4
			},
		},
		{
			name: "mapping starts after four files",
			weaken: func(manifest *AgentCapabilityManifest) {
				manifest.ImplementationRouting.DelegatedDirect.MappingMinUnderstandingFiles = 5
			},
		},
		{
			name: "writer starts after two non-trivial files",
			weaken: func(manifest *AgentCapabilityManifest) {
				manifest.ImplementationRouting.DelegatedDirect.WriterMinNonTrivialFiles = 3
			},
		},
		{
			name: "read preparing write no longer delegates",
			weaken: func(manifest *AgentCapabilityManifest) {
				manifest.ImplementationRouting.DelegatedDirect.DelegateWhenReadPreparesWrite = false
			},
		},
		{
			name: "broad research no longer delegates",
			weaken: func(manifest *AgentCapabilityManifest) {
				manifest.ImplementationRouting.DelegatedDirect.DelegateWhenBroadResearch = false
			},
		},
		{
			name: "substantial ambiguity no longer proposes SDD",
			weaken: func(manifest *AgentCapabilityManifest) {
				manifest.ImplementationRouting.SDD.ProposeWhenSubstantialOrAmbiguous = false
			},
		},
		{
			name: "SDD proposal need not reduce durable uncertainty",
			weaken: func(manifest *AgentCapabilityManifest) {
				manifest.ImplementationRouting.SDD.DurableArtifactsMustReduceUncertainty = false
			},
		},
		{
			name: "SDD selection bypasses explicit consent",
			weaken: func(manifest *AgentCapabilityManifest) {
				manifest.ImplementationRouting.SDD.SelectionPolicy = "automatic"
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			manifest := MustForAgent(model.AgentClaudeCode)
			test.weaken(&manifest)
			if err := manifest.Validate(); err == nil {
				t.Fatal("Validate() = nil, want non-canonical routing rejection")
			}
		})
	}
}

func TestEveryManifestKeepsWorkRoutingDormantAndHashesCanonically(t *testing.T) {
	t.Parallel()

	const wantRoutingDigest = "sha256:5f69a973e6e1a30a14647a392ea918145ea7b54e793000150bc764a3b34ca5ea"
	// Digests pin the four providers with an enforceable fresh-reviewer
	// boundary: Claude Code's generated reviewer has no live tools, OpenCode
	// relays one ordinary task through Go-owned admission, Codex's provider
	// subprocess reaches the same contract, and gentle-pi's host relay
	// forwards the Go-issued opaque task to a fresh locked-down pi
	// subprocess (gentle-pi#311, shevanio-ai#3249).
	wantManifestDigests := map[model.AgentID]string{
		model.AgentAntigravity: "sha256:3bcc241d9c5911aca6cc05394e91aa612dd9d0bfaf39d9fbd6964c89e0c8314f",
		model.AgentClaudeCode:  "sha256:5e5c7bbad4cf3fcd1e94d93e462b5f8e3b0797c568bd5a6c00daa806440bf30d",
		model.AgentCodex:       "sha256:ff9c33a5683b4087aeae16dc80393386cd9d0b9a9e098b7dc36d935387e57615",
		model.AgentCursor:      "sha256:59a9ab4b411889c6262173a877e89d42e3f52eaadf6fd783a1cb2f48e8eeff9e",
		model.AgentGeminiCLI:   "sha256:6576a11eaee1fd28e84a7e0b8729e49b132c4632549fdb891ae85f7fde77643e",
		model.AgentHermes:      "sha256:a5e23cc16dbba8528b970d9370348cf184c9795e5f8f9bc1ca0d446a321219f1",
		model.AgentKilocode:    "sha256:8f927a9bf1aa21e207ce38a1ddf87334a9dfedfcb92aaee8eac57d3f958fbb56",
		model.AgentKimi:        "sha256:2d3f8b5133633adc29959d4695f35a1291bb5f03c950da41af951157b97e3f9e",
		model.AgentKiroIDE:     "sha256:385a963c6e7bb90e6fde8ce69104ad19d2d57a45612c391a9a917cda4a63c46b",
		model.AgentOpenClaw:    "sha256:11c685af81f348f5f22e860e6951a5ae78dc59fc8e7af654d9694c8fc13bc93f",
		model.AgentOpenCode:    "sha256:176775053a061748a2a55bc1bd93d2181ceaf7c651b6b8c97c47ec1de10ce347",
		// Pi row updated by shevanio-ai#3249: review transport and immutable
		// reviewer execution flip to Advertised behind gentle-pi's host relay.
		// Deliberate, not drift.
		model.AgentPi:            "sha256:bef2c4fbaffaa9682fa5d3c0f7a761030ba1e6a030f7c19890972fc3aa87bf60",
		model.AgentQwenCode:      "sha256:d67bc1a8f2d6dd5cf5e3444e73adc3dfd2946320d3381618ea70191936f90916",
		model.AgentTrae:          "sha256:af8d5656dccedea846b5c79c48d8a822a7b46bff893f203145223e5ee09ebe2c",
		model.AgentVSCodeCopilot: "sha256:fe4676d2c9fddf1af24cf0530f91a7da713da5b819db8a03caf7f8187a688633",
		model.AgentWindsurf:      "sha256:ae3a0fbc2f6846f799795800758b7cef8111918e45a191ac25bc5eeffc2e3469",
	}

	for agent, wantDigest := range wantManifestDigests {
		agent := agent
		wantDigest := wantDigest
		t.Run(string(agent), func(t *testing.T) {
			t.Parallel()

			manifest := MustForAgent(agent)
			if err := manifest.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if manifest.Contracts.WorkRoutingV1.Exposure != ContractExposureDormant {
				t.Fatalf("work-routing exposure = %q, want %q", manifest.Contracts.WorkRoutingV1.Exposure, ContractExposureDormant)
			}
			if manifest.Advertises(ContractWorkRoutingV1) {
				t.Fatal("work-routing must remain unadvertised before final activation")
			}
			wantImmutableExecutor := agent == model.AgentClaudeCode || agent == model.AgentOpenCode || agent == model.AgentCodex || agent == model.AgentPi
			if got := manifest.Advertises(ContractImmutableReviewExecutorV1); got != wantImmutableExecutor {
				t.Fatalf("immutable reviewer execution advertised = %t, want %t", got, wantImmutableExecutor)
			}
			wantExposure := ContractExposureDormant
			if wantImmutableExecutor {
				wantExposure = ContractExposureAdvertised
			}
			if got := manifest.Contracts.ImmutableReviewExecutorV1.Exposure; got != wantExposure {
				t.Fatalf("immutable reviewer execution exposure = %q, want %q", got, wantExposure)
			}

			payload, err := manifest.CanonicalJSON()
			if err != nil {
				t.Fatalf("CanonicalJSON() error = %v", err)
			}
			var roundTrip AgentCapabilityManifest
			if err := json.Unmarshal(payload, &roundTrip); err != nil {
				t.Fatalf("Unmarshal(CanonicalJSON()) error = %v", err)
			}
			if roundTrip != manifest {
				t.Fatalf("canonical JSON round trip = %#v, want %#v", roundTrip, manifest)
			}

			gotDigest, err := roundTrip.Digest()
			if err != nil {
				t.Fatalf("Digest() error = %v", err)
			}
			if gotDigest != wantDigest {
				t.Fatalf("Digest() = %q, want %q", gotDigest, wantDigest)
			}

			gotRoutingDigest, err := manifest.RoutingDigest()
			if err != nil {
				t.Fatalf("RoutingDigest() error = %v", err)
			}
			if gotRoutingDigest != wantRoutingDigest {
				t.Fatalf("RoutingDigest() = %q, want %q", gotRoutingDigest, wantRoutingDigest)
			}
		})
	}
}

// TestEveryManifestDigestStaysByteStable pins that exactly 15 non-Pi agent rows
// remain byte-stable when the Pi row is updated. The Pi row change is the only
// expected delta from the capability flip; any other row changing is a regression.
// Digests are pinned to the current rebased baseline (post-rebase onto main);
// Commit 2 flips SystemPrompt for Pi and updates the Pi digest, after which
// both this test and TestEveryManifestKeepsWorkRoutingDormantAndHashesCanonically
// pass together.
func TestEveryManifestDigestStaysByteStable(t *testing.T) {
	t.Parallel()

	// Digests pinned to the current (post-rebase) baseline. These 15 rows must
	// not change when only the Pi row is updated — they are the stability contract.
	wantNonPiDigests := map[model.AgentID]string{
		model.AgentAntigravity: "sha256:3bcc241d9c5911aca6cc05394e91aa612dd9d0bfaf39d9fbd6964c89e0c8314f",
		model.AgentClaudeCode:  "sha256:5e5c7bbad4cf3fcd1e94d93e462b5f8e3b0797c568bd5a6c00daa806440bf30d",
		model.AgentCodex:       "sha256:ff9c33a5683b4087aeae16dc80393386cd9d0b9a9e098b7dc36d935387e57615",
		model.AgentCursor:      "sha256:59a9ab4b411889c6262173a877e89d42e3f52eaadf6fd783a1cb2f48e8eeff9e",
		model.AgentGeminiCLI:   "sha256:6576a11eaee1fd28e84a7e0b8729e49b132c4632549fdb891ae85f7fde77643e",
		model.AgentHermes:      "sha256:a5e23cc16dbba8528b970d9370348cf184c9795e5f8f9bc1ca0d446a321219f1",
		model.AgentKilocode:    "sha256:8f927a9bf1aa21e207ce38a1ddf87334a9dfedfcb92aaee8eac57d3f958fbb56",
		model.AgentKimi:        "sha256:2d3f8b5133633adc29959d4695f35a1291bb5f03c950da41af951157b97e3f9e",
		model.AgentKiroIDE:     "sha256:385a963c6e7bb90e6fde8ce69104ad19d2d57a45612c391a9a917cda4a63c46b",
		model.AgentOpenClaw:    "sha256:11c685af81f348f5f22e860e6951a5ae78dc59fc8e7af654d9694c8fc13bc93f",
		model.AgentOpenCode:    "sha256:176775053a061748a2a55bc1bd93d2181ceaf7c651b6b8c97c47ec1de10ce347",
		// Pi excluded intentionally — it changes in the next commit.
		model.AgentQwenCode:      "sha256:d67bc1a8f2d6dd5cf5e3444e73adc3dfd2946320d3381618ea70191936f90916",
		model.AgentTrae:          "sha256:af8d5656dccedea846b5c79c48d8a822a7b46bff893f203145223e5ee09ebe2c",
		model.AgentVSCodeCopilot: "sha256:fe4676d2c9fddf1af24cf0530f91a7da713da5b819db8a03caf7f8187a688633",
		model.AgentWindsurf:      "sha256:ae3a0fbc2f6846f799795800758b7cef8111918e45a191ac25bc5eeffc2e3469",
	}

	nonPiAgents := make([]model.AgentID, 0, len(wantNonPiDigests))
	for agent := range wantNonPiDigests {
		nonPiAgents = append(nonPiAgents, agent)
	}

	if got := len(nonPiAgents); got != 15 {
		t.Fatalf("want 15 non-Pi agents, got %d", got)
	}

	for _, agent := range nonPiAgents {
		agent := agent
		wantDigest := wantNonPiDigests[agent]
		t.Run(string(agent), func(t *testing.T) {
			t.Parallel()

			manifest := MustForAgent(agent)
			gotDigest, err := manifest.Digest()
			if err != nil {
				t.Fatalf("Digest() error = %v", err)
			}
			if gotDigest != wantDigest {
				t.Fatalf("Digest() = %q, want %q (byte-stable contract)", gotDigest, wantDigest)
			}
		})
	}
}

func TestForAgentRejectsUnknownAgent(t *testing.T) {
	t.Parallel()

	_, err := ForAgent(model.AgentID("unknown"))
	if !errors.Is(err, ErrUnsupportedAgent) {
		t.Fatalf("ForAgent() error = %v, want ErrUnsupportedAgent", err)
	}
}
