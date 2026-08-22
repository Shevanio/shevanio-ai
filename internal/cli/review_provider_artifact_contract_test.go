package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/shevanio/shevanio-ai/v2/internal/reviewtransaction"
)

func TestReviewProviderArtifactV1ContractsArePinned(t *testing.T) {
	root := filepath.Join("..", "..", "contracts", "review-integration", "v1")
	want := map[string]string{
		"fixtures/capabilities-v1.4.fixture.json": "745c1d8508ae371c8292839e933b8edf4a3c05b09c19871e6593e10cc6318535",
		"fixtures/start.fixture.json":             "3963cf5997e26864087e87ebd704302bd2b17ab4e419cc353bec515748b31c0f",
		// issue #2659: start-v2/status-v2 embed a freshly minted target_identity,
		// and the purified identity domain legitimately changed that hash for
		// every new snapshot. Deliberate, not drift.
		"fixtures/start-v2.fixture.json":         "322268ab298a1e54a957187242964d5e1abbd9b6434403ed7f52e5f425a169c3",
		"fixtures/status.fixture.json":           "cabdec8e925c65663b11f10d424563531f5e608d1daf01f5aa4cc8d4fc5c6e8a",
		"fixtures/status-v2.fixture.json":        "8b19bef08fe870707823f6c7a52d929ec7375ab678b050e56d58b504e9c8f65a",
		"fixtures/status-ambiguous.fixture.json": "53f7bf00053dfd4379b7ab9c05a4bf41e8ca2c6047927c75a019775ffe4c4ff4",
		"fixtures/status-corrupted.fixture.json": "f1d2d6c323fd810ea72df27b68a5d02f1336547eca3986b17817ee74edf3ce25",
		"fixtures/status-recover.fixture.json":   "ab8eaf51acedc824a6b1f5d53e1c3a53f4497be171a7fb5870b4a62f9bc90aa9",
		"fixtures/status-unrelated.fixture.json": "cd4204848b40e2d255bd8a9b76b7b2e366db3d07b2c6c305a3bf9c2882efc136",
		"schemas/admitted-result.schema.json":    "6c5b6c8cc3da1813b021da64f9b518ed6e48771c8911b361e920a8da3bd73f92",
		"schemas/artifact-subject.schema.json":   "ab5419a1e2ae6f794caa7d839b93914577c0a8d33cb26288f175354cf8879fb4",
		"schemas/capabilities-v1.4.schema.json":  "4cecdcf0a336737d351b578ab630ea4fd1a23f072d83b130a2322c517e9da140",
		"schemas/result-artifact.schema.json":    "9497399e082cfa71e3ede9b2f7262a8063c5e9314cd2e6d27153fb088a35d444",
		"schemas/start.schema.json":              "068c5a9b7d67364575fdbd84dc4dcc28b8a22bc7948ed79040783dd3a6ff2e25",
		"schemas/start-v2.schema.json":           "7536780c098b5dbb60e6d2e6bab289b1d669840b6b048e05d371a8b4283b52ce",
		"schemas/status.schema.json":             "965a18a0f5c4f7b36b9f7ccecec5e4476597e19be74b3d80b4dd7e9d816bd1aa",
		"schemas/status-v2.schema.json":          "34001defcbd57c12ea7636331419809c2518b0d408d3da986ad2bb311fda8d80",
	}
	for name, expected := range want {
		payload, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(payload)
		if actual := hex.EncodeToString(digest[:]); actual != expected {
			t.Fatalf("%s digest = %s, want %s", name, actual, expected)
		}
	}
}

func TestReviewProviderArtifactV20ContractsArePinned(t *testing.T) {
	root := filepath.Join("..", "..", "contracts", "review-integration", "v2")
	want := map[string]string{
		"fixtures/capabilities.fixture.json": "9cb8dae15023d5d9ff4bbd3b9e11ee9426376312ecf5b1c34cb7128995b683d1",
		"fixtures/consent.fixture.json":      "d5b6a0aff860b167fe08085ea1d96fb62c9c6eeb11c0a90ae278faa7d40ead7f",
		"fixtures/status.fixture.json":       "e2289c86b0ac798f7ecd40a9d962b28c680e28a783338fe6f014ccd2cafe32ec",
		"schemas/capabilities.schema.json":   "43f381dde3669281b8c85c42d7bd1e1ed1681ad17d52eea599af760ae33648fc",
		"schemas/consent.schema.json":        "bda1b04619e284e8df47e8b4f8d7c4130819af7b48e50d3fcbce4c3b4b8b8c5d",
		"schemas/status.schema.json":         "5ee1171879ad59f4d8ed82343ea1c1f905ec9e27350f391a2336fb944471a344",
	}
	for name, expected := range want {
		payload, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(payload)
		if actual := hex.EncodeToString(digest[:]); actual != expected {
			t.Fatalf("%s digest = %s, want %s", name, actual, expected)
		}
	}
}

func TestReviewProviderArtifactV21ContractsArePinned(t *testing.T) {
	root := filepath.Join("..", "..", "contracts", "review-integration", "v2")
	want := map[string]string{
		"fixtures/capabilities-v2.1.fixture.json": "d5fad1d401b8df62d18e2274222062804294efca704dc12b5429e557fc4bac62",
		// issue #2659: consent-v3 embeds a freshly minted target_identity;
		// the purified identity domain legitimately changed that hash.
		// Deliberate, not drift.
		"fixtures/consent-v3.fixture.json":      "45a96c22fdc93b67ed2a4ef135a4d4e7d731c6551b7113453bc96cbc1fd9c44b",
		"schemas/capabilities-v2.1.schema.json": "4fd757c7bf69bfb6c4ad90cd4975e29e4e33b69704950086cbfbc4db3be59485",
		// Cross-lane battery conformance fix: the schema pinned the choice
		// invocations to `--agent claude-code`, but the live emitter omits the
		// agent token when the caller declared no runtime (the pinned fixture
		// itself carries no --agent), and #2676 binds the declared runtime
		// (claude-code, opencode, codex) when there is one. The schema now
		// follows the emitter. The agent enum also admits "pi": the Pi host
		// relay drives consent with its own declared runtime identity, which
		// the emitter legitimately publishes once the relay handshake is
		// declared. Deliberate, not drift.
		"schemas/consent-v3.schema.json": "e182f62cc749093e13138f3c5dbd1a4eff261533c02aae8c315953a310e786a5",
		"schemas/status.schema.json":     "5ee1171879ad59f4d8ed82343ea1c1f905ec9e27350f391a2336fb944471a344",
	}
	for name, expected := range want {
		payload, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(payload)
		if actual := hex.EncodeToString(digest[:]); actual != expected {
			t.Fatalf("%s digest = %s, want %s", name, actual, expected)
		}
	}
}

func TestReviewProviderArtifactV25StatusContractsArePinned(t *testing.T) {
	root := filepath.Join("..", "..", "contracts", "review-integration", "v2")
	want := map[string]string{
		"fixtures/status-v5.fixture.json": "831b9d6b6771ebd053f270cad451765f024fd8a0327da8b20398278cb3bd22af",
		// Cross-lane battery conformance fix: live negotiated STATUS publishes
		// the top-level repository_context reference (review_status_contract.go's
		// ReviewTargetStatusResult, populated since the recovered-units merges),
		// which the schema did not admit; and targeted_validation_required also
		// arrives as the provider-task (external.run_provider_role) and pi
		// host-relay (review.capture-validation) shapes, which the schema
		// rejected as missing the generic submission; and the negotiated-route
		// disposition preview (ReviewRepairDispositionProviderInputs) is real
		// optional emitter output the strict schema must admit; and the pi
		// host-relay materialize path renders the reviewer_result collect
		// input with a capture-result submission descriptor, which the
		// submission oneOf and the no-submission allOf rule both rejected.
		// Deliberate, not drift.
		"schemas/status-v5.schema.json": "da0e0c7c5d696b08bcc82e33f30943ebe2bdd9a25857f29e60d020a238e5748a",
	}
	for name, expected := range want {
		payload, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(payload)
		if actual := hex.EncodeToString(digest[:]); actual != expected {
			t.Fatalf("%s digest = %s, want %s", name, actual, expected)
		}
	}
}

// TestReviewProviderArtifactConformanceSchemasArePinned pins the schemas the
// cross-lane battery conformance work first published: the delivery gate
// result (shevanio-ai.review-gate-result/v1) and the OpenCode provider-role
// capture acknowledgement (shevanio-ai.opencode-review-provider-role/v1). Both
// envelopes already shipped on the wire; only their published schemas are new.
func TestReviewProviderArtifactConformanceSchemasArePinned(t *testing.T) {
	root := filepath.Join("..", "..", "contracts", "review-integration", "v2")
	want := map[string]string{
		"schemas/gate-result.schema.json":            "cfa5ca058d7ff4238f1461c33374b908cf272081c4578dbb47cd8edbde04436e",
		"schemas/opencode-provider-role.schema.json": "9082804bd12099cae4018be5e93da8bae78d839604fe888f12fdb2d6eb55bf32",
	}
	for name, expected := range want {
		payload, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(payload)
		if actual := hex.EncodeToString(digest[:]); actual != expected {
			t.Fatalf("%s digest = %s, want %s", name, actual, expected)
		}
	}
}

func TestReviewProviderArtifactSchemasAreStrictAndBound(t *testing.T) {
	root := filepath.Join("..", "..", "contracts", "review-integration", "v1", "schemas")
	tests := []struct {
		name string
		id   string
	}{
		{name: "artifact-subject.schema.json", id: "https://shevanio-ai.dev/contracts/review-integration/v1/schemas/artifact-subject.schema.json"},
		{name: "admitted-result.schema.json", id: "https://shevanio-ai.dev/contracts/review-integration/v1/schemas/admitted-result.schema.json"},
		{name: "correction-plan-request.schema.json", id: reviewtransaction.CorrectionPlanRequestSchemaID},
		{name: "result-artifact-v2.schema.json", id: "https://shevanio-ai.dev/contracts/review-integration/v1/schemas/result-artifact-v2.schema.json"},
		{name: "start-v2.schema.json", id: ReviewIntegrationStartSchemaIDV2},
		{name: "status-v2.schema.json", id: ReviewIntegrationStatusSchemaIDV2},
		{name: "authority-repair-assessment.schema.json", id: reviewtransaction.AuthorityRepairAssessmentSchemaID},
		{name: "repair.schema.json", id: ReviewIntegrationRepairSchemaID},
	}
	documents := make(map[string]map[string]any, len(tests))
	for _, tt := range tests {
		payload, err := os.ReadFile(filepath.Join(root, tt.name))
		if err != nil {
			t.Fatal(err)
		}
		var schema map[string]any
		if err := json.Unmarshal(payload, &schema); err != nil {
			t.Fatal(err)
		}
		if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" || schema["$id"] != tt.id || schema["additionalProperties"] != false {
			t.Fatalf("%s header = %#v", tt.name, schema)
		}
		documents[tt.name] = schema
	}

	artifact := documents["result-artifact-v2.schema.json"]
	artifactRequired := schemaStringArray(t, artifact["required"])
	for _, field := range []string{"subject_hash", "admission_decision"} {
		if !slices.Contains(artifactRequired, field) {
			t.Fatalf("result artifact v2 omits required %q: %v", field, artifactRequired)
		}
	}
	if artifact["oneOf"] == nil {
		t.Fatal("result artifact v2 does not require exactly one provider-owned locator")
	}

	start := documents["start-v2.schema.json"]
	if !slices.Contains(schemaStringArray(t, start["required"]), "artifact_subjects") {
		t.Fatal("START v2 does not require provider-owned artifact subjects")
	}
	riskCodes := start["$defs"].(map[string]any)["risk_reason"].(map[string]any)["properties"].(map[string]any)["code"].(map[string]any)["enum"]
	codes := schemaStringArray(t, riskCodes)
	for _, code := range []string{string(reviewtransaction.RiskReasonProcessBoundary), string(reviewtransaction.RiskReasonProcessScanLimit)} {
		if !slices.Contains(codes, code) {
			t.Fatalf("START v2 rejects runtime risk reason %q: %v", code, codes)
		}
	}
	startStates := schemaStringArray(t, start["properties"].(map[string]any)["state"].(map[string]any)["enum"])
	for _, state := range []string{string(reviewtransaction.StateCorrectionRequired), string(reviewtransaction.StateValidating)} {
		if !slices.Contains(startStates, state) {
			t.Fatalf("START v2 rejects valid compact state %q: %v", state, startStates)
		}
	}

	status := documents["status-v2.schema.json"]
	transitionArtifact := status["$defs"].(map[string]any)["transition_artifact"].(map[string]any)
	transitionRequired := schemaStringArray(t, transitionArtifact["required"])
	for _, field := range []string{"subject_hash", "admission_decision"} {
		if !slices.Contains(transitionRequired, field) {
			t.Fatalf("status v2 transition artifact omits %q: %v", field, transitionRequired)
		}
	}
	properties := transitionArtifact["properties"].(map[string]any)
	if properties["schema"].(map[string]any)["const"] != reviewResultArtifactSchema ||
		properties["admission_decision"].(map[string]any)["const"] != string(reviewtransaction.ArtifactAdmissionCompleted) {
		t.Fatalf("status v2 artifact identity = %#v", properties)
	}
	transitionInput := status["$defs"].(map[string]any)["transition_input"].(map[string]any)
	inputRules := transitionInput["allOf"].([]any)
	captureRule := inputRules[1].(map[string]any)
	captureThen := captureRule["then"].(map[string]any)
	for _, field := range []string{"artifact_subject", "candidate_diff", "changed_path_manifest"} {
		if !slices.Contains(schemaStringArray(t, captureThen["required"]), field) {
			t.Fatalf("legacy status v2 capture input omits required frozen context %q: %#v", field, captureThen)
		}
	}
	inputProperties := transitionInput["properties"].(map[string]any)
	if inputProperties["artifact_subject"].(map[string]any)["$ref"] != "artifact-subject.schema.json" ||
		inputProperties["candidate_diff"] == nil || inputProperties["base_tree"] != nil || inputProperties["candidate_tree"] != nil ||
		inputProperties["changed_path_manifest"].(map[string]any)["type"] != "array" {
		t.Fatalf("legacy status v2 capture input frozen context = %#v", inputProperties)
	}

	v2Root := filepath.Join("..", "..", "contracts", "review-integration", "v2", "schemas")
	v2Schemas := []struct {
		name string
		id   string
	}{
		{name: "artifact-subject.schema.json", id: "https://shevanio-ai.dev/contracts/review-integration/v2/schemas/artifact-subject.schema.json"},
		{name: "admitted-result.schema.json", id: "https://shevanio-ai.dev/contracts/review-integration/v2/schemas/admitted-result.schema.json"},
		{name: "start.schema.json", id: ReviewIntegrationStartSchemaID},
		{name: "status.schema.json", id: ReviewIntegrationStatusSchemaIDV3},
		{name: "status-v4.schema.json", id: ReviewIntegrationStatusSchemaIDV4},
		{name: "status-v5.schema.json", id: ReviewIntegrationStatusSchemaIDV5},
		{name: "capabilities.schema.json", id: ReviewIntegrationCapabilitiesSchemaIDV2},
		{name: "capabilities-v2.1.schema.json", id: ReviewIntegrationCapabilitiesSchemaIDV21},
		{name: "capabilities-v2.2.schema.json", id: ReviewIntegrationCapabilitiesSchemaIDV22},
		{name: "consent.schema.json", id: ReviewIntegrationConsentSchemaIDV2},
		{name: "consent-v3.schema.json", id: ReviewIntegrationConsentSchemaIDV3},
		{name: "failure.schema.json", id: ReviewIntegrationFailureSchemaIDV2},
		{name: "operation.schema.json", id: ReviewIntegrationOperationSchemaIDV2},
		{name: "repair.schema.json", id: ReviewIntegrationRepairSchemaIDV2},
		{name: "gate-result.schema.json", id: "https://shevanio-ai.dev/contracts/review-integration/v2/schemas/gate-result.schema.json"},
		{name: "opencode-provider-role.schema.json", id: "https://shevanio-ai.dev/contracts/review-integration/v2/schemas/opencode-provider-role.schema.json"},
	}
	v2Documents := make(map[string]map[string]any, len(v2Schemas))
	for _, tt := range v2Schemas {
		payload, err := os.ReadFile(filepath.Join(v2Root, tt.name))
		if err != nil {
			t.Fatal(err)
		}
		var schema map[string]any
		if err := json.Unmarshal(payload, &schema); err != nil {
			t.Fatal(err)
		}
		if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" || schema["$id"] != tt.id || schema["additionalProperties"] != false {
			t.Fatalf("v2 %s header = %#v", tt.name, schema)
		}
		v2Documents[tt.name] = schema
	}
	v2Input := v2Documents["status.schema.json"]["$defs"].(map[string]any)["transition_input"].(map[string]any)
	v2CaptureThen := v2Input["allOf"].([]any)[1].(map[string]any)["then"].(map[string]any)
	for _, field := range []string{"artifact_subject", "base_tree", "candidate_tree", "changed_path_manifest"} {
		if !slices.Contains(schemaStringArray(t, v2CaptureThen["required"]), field) {
			t.Fatalf("native Git status capture input omits %q: %#v", field, v2CaptureThen)
		}
	}
	v2Properties := v2Input["properties"].(map[string]any)
	if v2Properties["candidate_diff"] != nil || v2Properties["base_tree"] == nil || v2Properties["candidate_tree"] == nil {
		t.Fatalf("native Git status capture input = %#v", v2Properties)
	}
}

func TestReviewProviderArtifactV2FixturesValidate(t *testing.T) {
	root := filepath.Join("..", "..", "contracts", "review-integration", "v2", "fixtures")
	startPayload, err := os.ReadFile(filepath.Join(root, "start.fixture.json"))
	if err != nil {
		t.Fatal(err)
	}
	var start ReviewIntegrationStartResult
	if err := json.Unmarshal(startPayload, &start); err != nil {
		t.Fatal(err)
	}
	if err := start.Validate(); err != nil {
		t.Fatalf("v2 START fixture: %v", err)
	}
	statusPayload, err := os.ReadFile(filepath.Join(root, "status.fixture.json"))
	if err != nil {
		t.Fatal(err)
	}
	var status ReviewTargetStatusResult
	if err := json.Unmarshal(statusPayload, &status); err != nil {
		t.Fatal(err)
	}
	if err := status.Validate(); err != nil {
		t.Fatalf("v2 STATUS fixture: %v", err)
	}
	v5StatusPayload, err := os.ReadFile(filepath.Join(root, "status-v5.fixture.json"))
	if err != nil {
		t.Fatal(err)
	}
	var v5Status ReviewTargetStatusResult
	if err := json.Unmarshal(v5StatusPayload, &v5Status); err != nil {
		t.Fatal(err)
	}
	if err := v5Status.Validate(); err != nil {
		t.Fatalf("v5 STATUS fixture: %v", err)
	}
	consentPayload, err := os.ReadFile(filepath.Join(root, "consent.fixture.json"))
	if err != nil {
		t.Fatal(err)
	}
	var consent ReviewIntegrationConsentResult
	if err := json.Unmarshal(consentPayload, &consent); err != nil {
		t.Fatal(err)
	}
	if err := consent.Validate(); err != nil {
		t.Fatalf("v2 consent fixture: %v", err)
	}
	consentV3Payload, err := os.ReadFile(filepath.Join(root, "consent-v3.fixture.json"))
	if err != nil {
		t.Fatal(err)
	}
	var consentV3 ReviewIntegrationConsentResult
	if err := json.Unmarshal(consentV3Payload, &consentV3); err != nil {
		t.Fatal(err)
	}
	if err := consentV3.Validate(); err != nil || consentV3.Agent != "claude-code" {
		t.Fatalf("v2.1 consent fixture: %#v, %v", consentV3, err)
	}
}

func schemaStringArray(t *testing.T, value any) []string {
	t.Helper()
	values, ok := value.([]any)
	if !ok {
		t.Fatalf("schema value is not an array: %#v", value)
	}
	result := make([]string, len(values))
	for index, value := range values {
		text, ok := value.(string)
		if !ok {
			t.Fatalf("schema array value is not a string: %#v", value)
		}
		result[index] = text
	}
	return result
}
