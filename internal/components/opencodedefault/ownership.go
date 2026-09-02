package opencodedefault

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/shevanio/shevanio-ai/v2/internal/components/filemerge"
	"github.com/shevanio/shevanio-ai/v2/internal/components/mutationjournal"
	"github.com/shevanio/shevanio-ai/v2/internal/model"
)

const (
	schema           = "shevanio-ai.opencode-default-agent"
	legacyVersion    = 1
	version          = 2
	maxOwnershipSize = 1 << 20
)

var (
	profileScopePattern = regexp.MustCompile(`^profile/[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
	fingerprintPattern  = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

var ManagedAgent = model.CanonicalManagedIdentity.Actor

type ownership struct {
	Schema          string                    `json:"schema"`
	Version         int                       `json:"version"`
	State           string                    `json:"state"`
	PreviousState   string                    `json:"previous_state"`
	PreviousDefault string                    `json:"previous_default,omitempty"`
	Actors          map[string]actorOwnership `json:"actors"`
}
type legacyOwnership struct {
	Schema          string `json:"schema"`
	Version         int    `json:"version"`
	State           string `json:"state"`
	PreviousState   string `json:"previous_state"`
	PreviousDefault string `json:"previous_default,omitempty"`
}
type actorOwnership struct {
	Scope       string `json:"scope"`
	Fingerprint string `json:"fingerprint"`
}
type fieldValue struct {
	present bool
	value   string
}
type InstallPlan struct {
	settingsPath                      string
	owned                             *ownership
	observed, desired                 map[string]actorOwnership
	observedScopes                    map[string]bool
	expectedSettings, expectedOwner   []byte
	settingsExist, ownershipFileExist bool
}
type UninstallPlan struct {
	settingsPath                   string
	settingsExist, ownershipExists bool
	settingsRaw, ownershipRaw      []byte
	current                        fieldValue
	owned                          *ownership
}

func OwnershipPath(settingsPath string) string {
	return filepath.Join(filepath.Dir(settingsPath), ".shevanio-ai-default-agent.json")
}
func PrepareInstall(settingsPath string) (*InstallPlan, error) {
	_, raw, exists, _, err := readSettings(settingsPath)
	if err != nil {
		return nil, err
	}
	owned, ownerRaw, err := readOwnershipRaw(OwnershipPath(settingsPath))
	if err != nil {
		return nil, err
	}
	return &InstallPlan{settingsPath: settingsPath, owned: owned, expectedSettings: raw, settingsExist: exists, expectedOwner: ownerRaw, ownershipFileExist: owned != nil}, nil
}
func (p *InstallPlan) Apply() (bool, error) {
	root, raw, exists, current, err := readSettings(p.settingsPath)
	if err != nil {
		return false, err
	}
	if !sameSnapshot(raw, exists, p.expectedSettings, p.settingsExist) {
		return false, fmt.Errorf("OpenCode ownership transaction conflict: settings changed after observation")
	}
	if err := verifySnapshot(OwnershipPath(p.settingsPath), p.expectedOwner, p.ownershipFileExist); err != nil {
		return false, err
	}
	actors := map[string]actorOwnership{}
	if p.owned != nil {
		for actor, record := range p.owned.Actors {
			actors[actor] = record
		}
	}
	agents, _ := root["agent"].(map[string]any)
	for actor, record := range actors {
		if !p.observedScopes[record.Scope] {
			continue
		}
		if _, wanted := p.desired[actor]; wanted {
			continue
		}
		delete(actors, actor)
		fingerprint, exact := actorFingerprint(agents[actor])
		if exact && fingerprint == record.Fingerprint {
			delete(agents, actor)
		}
	}
	owned := p.owned
	_, currentClass := model.NormalizeOrchestratorRead(current.value)
	if owned == nil || !current.present || currentClass == model.IdentityUnknown {
		owned = newOwnership(current)
	}
	owned.Actors = actors
	for actor, record := range p.observed {
		owned.Actors[actor] = record
	}
	root["default_agent"] = model.CanonicalManagedIdentity.Actor
	settings := encode(root)
	metadata := encode(owned)
	ownerPath := OwnershipPath(p.settingsPath)
	ownerRaw, _ := os.ReadFile(ownerPath)
	changed := !bytes.Equal(raw, settings) || !bytes.Equal(ownerRaw, metadata)
	if err := writePair(p.settingsPath, settings, true, ownerPath, metadata, true); err != nil {
		return false, err
	}
	return changed, nil
}

// ObserveOverlay records exact actor projections created or changed by this plan.
// Existing identical actors and merged supersets remain unowned.
func (p *InstallPlan) ObserveOverlay(scope string, before, overlay, merged []byte) error {
	if scope != "base" && !profileScopePattern.MatchString(scope) {
		return fmt.Errorf("invalid OpenCode actor ownership scope %q", scope)
	}
	normalized, err := filemerge.MergeJSONObjects(nil, overlay)
	if err != nil {
		return fmt.Errorf("normalize OpenCode actor overlay: %w", err)
	}
	generatedActors, err := actorObjects(normalized)
	if err != nil {
		return err
	}
	beforeActors, err := actorObjects(before)
	if err != nil {
		return err
	}
	mergedActors, err := actorObjects(merged)
	if err != nil {
		return err
	}
	if p.observedScopes == nil {
		p.observedScopes = map[string]bool{}
		p.desired = map[string]actorOwnership{}
	}
	p.observedScopes[scope] = true
	p.expectedSettings, p.settingsExist = append([]byte(nil), merged...), true
	for actor, generated := range generatedActors {
		record := actorOwnership{Scope: scope, Fingerprint: actorFingerprintMust(generated)}
		p.desired[actor] = record
		actual, exists := mergedActors[actor]
		if !exists || !jsonEqual(actual, generated) {
			continue
		}
		if previous, existed := beforeActors[actor]; existed && jsonEqual(previous, actual) {
			continue
		}
		if p.observed == nil {
			p.observed = map[string]actorOwnership{}
		}
		p.observed[actor] = record
	}
	return nil
}
func PrepareUninstall(settingsPath string) (*UninstallPlan, error) {
	_, raw, exists, current, err := readSettings(settingsPath)
	if err != nil {
		return nil, err
	}
	owned, ownerRaw, err := readOwnershipRaw(OwnershipPath(settingsPath))
	if err != nil {
		return nil, err
	}
	return &UninstallPlan{settingsPath: settingsPath, settingsExist: exists, settingsRaw: raw, current: current, owned: owned, ownershipRaw: ownerRaw, ownershipExists: owned != nil}, nil
}
func (p *UninstallPlan) Apply(cleaned []byte, settingsExist bool) (changed, removed bool, err error) {
	_, currentRaw, currentExists, _, err := readSettings(p.settingsPath)
	if err != nil {
		return false, false, err
	}
	// A trusted earlier uninstall rewrite may have removed an emptied settings
	// file. With no settings left there is no actor deletion to authorize, but
	// the stale sidecar must still be removed.
	settingsAlreadyRemoved := !currentExists && !settingsExist
	if !sameSnapshot(currentRaw, currentExists, p.settingsRaw, p.settingsExist) && !settingsAlreadyRemoved {
		return false, false, fmt.Errorf("OpenCode ownership transaction conflict: settings changed after preparation")
	}
	if err := verifySnapshot(OwnershipPath(p.settingsPath), p.ownershipRaw, p.ownershipExists); err != nil {
		return false, false, err
	}
	root := map[string]any{}
	if settingsExist {
		if root, err = filemerge.UnmarshalJSONObject(cleaned); err != nil {
			return false, false, fmt.Errorf("parse cleaned OpenCode settings: %w", err)
		}
	}
	if p.owned != nil {
		agents, _ := root["agent"].(map[string]any)
		for actor, record := range p.owned.Actors {
			if fingerprint, exact := actorFingerprint(agents[actor]); exact && fingerprint == record.Fingerprint {
				delete(agents, actor)
			}
		}
	}
	_, currentClass := model.NormalizeOrchestratorRead(p.current.value)
	ownedDefault := currentClass == model.IdentityCanonicalManaged || (currentClass == model.IdentityLegacyManaged && p.owned != nil)
	if p.settingsExist && p.current.present && ownedDefault {
		if p.owned == nil || p.owned.PreviousState == "absent" {
			delete(root, "default_agent")
		} else {
			root["default_agent"] = p.owned.PreviousDefault
		}
	}
	settingsExist = settingsExist && len(root) > 0
	var settings []byte
	if settingsExist {
		settings = encode(root)
	}
	changed = currentExists != settingsExist || (settingsExist && !bytes.Equal(currentRaw, settings)) || p.owned != nil
	if err := writePair(p.settingsPath, settings, settingsExist, OwnershipPath(p.settingsPath), nil, false); err != nil {
		return false, false, err
	}
	return changed, currentExists && !settingsExist, nil
}

func readSettings(path string) (map[string]any, []byte, bool, fieldValue, error) {
	raw, err := readRegular(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil, false, fieldValue{}, nil
	}
	if err != nil {
		return nil, nil, false, fieldValue{}, fmt.Errorf("read OpenCode settings %q: %w", path, err)
	}
	root, err := filemerge.UnmarshalJSONObject(raw)
	if err != nil {
		return nil, nil, false, fieldValue{}, fmt.Errorf("parse OpenCode settings %q: %w", path, err)
	}
	current, err := defaultField(root)
	if err != nil {
		return nil, nil, false, fieldValue{}, err
	}
	return root, raw, true, current, nil
}
func readOwnershipRaw(path string) (*ownership, []byte, error) {
	raw, err := readRegular(path)
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read OpenCode default ownership %q: %w", path, err)
	}
	if len(raw) > maxOwnershipSize {
		return nil, nil, fmt.Errorf("read OpenCode default ownership %q: sidecar exceeds %d bytes", path, maxOwnershipSize)
	}
	var envelope struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, nil, fmt.Errorf("decode OpenCode default ownership %q: %w", path, err)
	}
	if envelope.Version == legacyVersion {
		var legacy legacyOwnership
		if err := decodeOwnership(raw, &legacy); err != nil || !validOwnershipHeader(legacy.Schema, legacy.Version, legacy.State, legacy.PreviousState, legacy.PreviousDefault, legacyVersion) {
			return nil, nil, fmt.Errorf("invalid OpenCode default ownership %q", path)
		}
		return &ownership{Schema: legacy.Schema, Version: version, State: legacy.State, PreviousState: legacy.PreviousState, PreviousDefault: legacy.PreviousDefault, Actors: map[string]actorOwnership{}}, raw, nil
	}
	if envelope.Version != version {
		return nil, nil, fmt.Errorf("invalid OpenCode default ownership %q", path)
	}
	var owned ownership
	if err := decodeOwnership(raw, &owned); err != nil {
		return nil, nil, fmt.Errorf("decode OpenCode default ownership %q: %w", path, err)
	}
	if !validOwnershipHeader(owned.Schema, owned.Version, owned.State, owned.PreviousState, owned.PreviousDefault, version) || owned.Actors == nil {
		return nil, nil, fmt.Errorf("invalid OpenCode default ownership %q", path)
	}
	for actor, record := range owned.Actors {
		if actor == "" || record.Scope != "base" && !profileScopePattern.MatchString(record.Scope) || !fingerprintPattern.MatchString(record.Fingerprint) {
			return nil, nil, fmt.Errorf("invalid OpenCode default ownership %q", path)
		}
	}
	return &owned, raw, nil
}

func decodeOwnership(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("trailing data")
	}
	return nil
}
func validOwnershipHeader(gotSchema string, gotVersion int, state, previousState, previousDefault string, wantVersion int) bool {
	validPrevious := previousState == "absent" && previousDefault == "" || previousState == "value"
	return gotSchema == schema && gotVersion == wantVersion && state == "managed" && validPrevious
}
func readRegular(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	return os.ReadFile(path)
}
func defaultField(root map[string]any) (fieldValue, error) {
	value, exists := root["default_agent"]
	if !exists {
		return fieldValue{}, nil
	}
	text, ok := value.(string)
	if !ok {
		return fieldValue{}, fmt.Errorf("OpenCode default_agent must be a string")
	}
	return fieldValue{present: true, value: text}, nil
}
func newOwnership(previous fieldValue) *ownership {
	owned := &ownership{Schema: schema, Version: version, State: "managed", PreviousState: "absent", Actors: map[string]actorOwnership{}}
	if previous.present {
		owned.PreviousState = "value"
		owned.PreviousDefault = previous.value
	}
	return owned
}

func actorObjects(raw []byte) (map[string]map[string]any, error) {
	root, err := filemerge.UnmarshalJSONObject(raw)
	if err != nil {
		return nil, fmt.Errorf("parse OpenCode actor projection: %w", err)
	}
	result := map[string]map[string]any{}
	agents, _ := root["agent"].(map[string]any)
	for actor, value := range agents {
		if object, ok := value.(map[string]any); ok {
			result[actor] = object
		}
	}
	return result, nil
}
func jsonEqual(left, right any) bool {
	leftJSON, _ := json.Marshal(left)
	rightJSON, _ := json.Marshal(right)
	return bytes.Equal(leftJSON, rightJSON)
}
func actorFingerprint(value any) (string, bool) {
	object, ok := value.(map[string]any)
	if !ok {
		return "", false
	}
	return actorFingerprintMust(object), true
}
func actorFingerprintMust(object map[string]any) string {
	canonical, _ := json.Marshal(object)
	sum := sha256.Sum256(canonical)
	return fmt.Sprintf("sha256:%x", sum)
}
func sameSnapshot(got []byte, gotExists bool, want []byte, wantExists bool) bool {
	return gotExists == wantExists && (!gotExists || bytes.Equal(got, want))
}
func verifySnapshot(path string, want []byte, wantExists bool) error {
	got, err := readRegular(path)
	if os.IsNotExist(err) {
		if !wantExists {
			return nil
		}
	} else if err != nil {
		return fmt.Errorf("verify OpenCode ownership transaction %q: %w", path, err)
	} else if wantExists && bytes.Equal(got, want) {
		return nil
	}
	return fmt.Errorf("OpenCode ownership transaction conflict: %q changed after preparation", path)
}
func encode(value any) []byte {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(err) // Values are either decoded JSON or the fixed ownership struct.
	}
	return append(raw, '\n')
}
func writePair(settingsPath string, settings []byte, keepSettings bool, ownerPath string, owner []byte, keepOwner bool) error {
	journal := mutationjournal.New(filepath.Dir(settingsPath))
	mutations := []pairMutation{
		{path: settingsPath, data: settings, keep: keepSettings, context: "update OpenCode settings"},
		{path: ownerPath, data: owner, keep: keepOwner, context: "update OpenCode default ownership"},
	}
	return applyPair(journal, mutations, mutatePair)
}

type pairMutation struct {
	path    string
	data    []byte
	keep    bool
	context string
}

type pairMutator func(*mutationjournal.Journal, pairMutation) error

func applyPair(journal *mutationjournal.Journal, mutations []pairMutation, mutate pairMutator) error {
	for _, mutation := range mutations {
		if err := journal.Capture(mutation.path); err != nil {
			return err
		}
	}
	for _, mutation := range mutations {
		if err := mutate(journal, mutation); err != nil {
			return fmt.Errorf("%s: %w (rollback: %v)", mutation.context, err, journal.Restore())
		}
	}
	return nil
}

func mutatePair(journal *mutationjournal.Journal, mutation pairMutation) error {
	if mutation.keep {
		_, err := journal.WriteWithMode(mutation.path, mutation.data, 0o644)
		return err
	}
	_, err := journal.Remove(mutation.path)
	return err
}
