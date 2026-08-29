package update

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shevanio/shevanio-ai/v2/internal/reviewtransaction"
	"github.com/shevanio/shevanio-ai/v2/internal/state"
	"github.com/shevanio/shevanio-ai/v2/internal/statestore"
	"github.com/shevanio/shevanio-ai/v2/internal/system"
)

func TestCheckAllWithCooldown_BridgeContentionLeavesStateUntouched(t *testing.T) {
	home := t.TempDir()
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-7 * time.Hour)
	if err := state.Write(home, state.InstallState{InstalledAgents: []string{"claude-code"}, LastUpdateCheck: &stale}); err != nil {
		t.Fatal(err)
	}
	locked, err := reviewtransaction.AcquireAuthorityFileLock(state.Path(home) + ".lock")
	if err != nil {
		t.Fatal(err)
	}

	checkCalls := 0
	CheckAllWithCooldown(context.Background(), "1.0.0", system.PlatformProfile{}, home, 6*time.Hour,
		func() time.Time { return now },
		func(context.Context, string, system.PlatformProfile) []UpdateResult {
			checkCalls++
			return []UpdateResult{{Tool: ToolInfo{Name: "shevanio-ai"}, Status: UpToDate}}
		},
	)

	if err := locked.Release(); err != nil {
		t.Fatal(err)
	}
	after, err := state.Read(home)
	if err != nil {
		t.Fatal(err)
	}
	if checkCalls != 1 || after.LastUpdateCheck == nil || !after.LastUpdateCheck.Equal(stale) || len(after.InstalledAgents) != 1 {
		t.Fatalf("behavior mismatch: TestCheckAllWithCooldown_BridgeContentionLeavesStateUntouched")
	}
}

func TestCheckAllWithCooldown_MutateConcurrentRetryPreservesDistinctFields(t *testing.T) {
	home := t.TempDir()
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-7 * time.Hour)
	if err := state.Write(home, state.InstallState{LastUpdateCheck: &stale}); err != nil {
		t.Fatal(err)
	}

	competingStarted := make(chan struct{})
	releaseCompeting := make(chan struct{})
	competingDone := make(chan error, 1)
	go func() {
		_, err := statestore.Mutate(home, func(current *state.InstallState) error {
			close(competingStarted)
			<-releaseCompeting
			current.BackgroundIntent = "competing-writer"
			return nil
		})
		competingDone <- err
	}()
	<-competingStarted

	checkResults := []UpdateResult{{Tool: ToolInfo{Name: "shevanio-ai"}, Status: UpToDate}}
	results := CheckAllWithCooldown(context.Background(), "1.0.0", system.PlatformProfile{}, home, 6*time.Hour,
		func() time.Time { return now },
		func(context.Context, string, system.PlatformProfile) []UpdateResult {
			return checkResults
		},
	)
	if len(results) != 1 || results[0].Status != UpToDate {
		t.Fatalf("cooldown result = %v, want one successful result", results)
	}

	whileContended, err := state.Read(home)
	if err != nil {
		t.Fatal(err)
	}
	if whileContended.LastUpdateCheck == nil || !whileContended.LastUpdateCheck.Equal(stale) || whileContended.BackgroundIntent != "" {
		t.Fatalf("contended write changed state: %+v", whileContended)
	}

	close(releaseCompeting)
	if err := <-competingDone; err != nil {
		t.Fatal(err)
	}

	CheckAllWithCooldown(context.Background(), "1.0.0", system.PlatformProfile{}, home, 6*time.Hour,
		func() time.Time { return now },
		func(context.Context, string, system.PlatformProfile) []UpdateResult {
			return checkResults
		},
	)

	afterRetry, err := state.Read(home)
	if err != nil {
		t.Fatal(err)
	}
	if afterRetry.LastUpdateCheck == nil || !afterRetry.LastUpdateCheck.Equal(now) || afterRetry.BackgroundIntent != "competing-writer" {
		t.Fatalf("retry did not preserve both fields: %+v", afterRetry)
	}
}

// TestCheckAllWithCooldown_FreshCacheSkipsNetwork verifies that when
// LastUpdateCheck is recent (elapsed < TTL) no GitHub call is made and the
// cached/empty result is returned immediately.
func TestCheckAllWithCooldown_FreshCacheSkipsNetwork(t *testing.T) {
	home := t.TempDir()
	profile := system.PlatformProfile{OS: "darwin", PackageManager: "brew"}

	// Write state with a LastUpdateCheck 1 minute ago (well within 6h TTL).
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-1 * time.Minute)
	s := state.InstallState{
		InstalledAgents: []string{"claude-code"},
		LastUpdateCheck: &recent,
	}
	if err := state.Write(home, s); err != nil {
		t.Fatalf("state.Write() error = %v", err)
	}

	checkCalled := 0
	stubCheckAll := func(_ context.Context, _ string, _ system.PlatformProfile) []UpdateResult {
		checkCalled++
		return []UpdateResult{{Status: UpdateAvailable}}
	}

	results := CheckAllWithCooldown(context.Background(), "1.0.0", profile, home, 6*time.Hour,
		func() time.Time { return now },
		stubCheckAll,
	)

	if checkCalled != 0 {
		t.Errorf("network check called %d times, want 0 (cache is fresh)", checkCalled)
	}
	if len(results) != 0 {
		t.Errorf("results = %v, want empty (fresh cache returns nil/empty)", results)
	}
}

// TestCheckAllWithCooldown_StaleCacheRefreshes verifies that when
// LastUpdateCheck is older than the TTL the network check fires and on success
// LastUpdateCheck is updated in state.
func TestCheckAllWithCooldown_StaleCacheRefreshes(t *testing.T) {
	home := t.TempDir()
	profile := system.PlatformProfile{OS: "darwin", PackageManager: "brew"}

	// Write state with LastUpdateCheck 7 hours ago (> 6h TTL).
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-7 * time.Hour)
	s := state.InstallState{
		InstalledAgents: []string{"claude-code"},
		LastUpdateCheck: &stale,
	}
	if err := state.Write(home, s); err != nil {
		t.Fatalf("state.Write() error = %v", err)
	}

	stubResults := []UpdateResult{{Tool: ToolInfo{Name: "shevanio-ai"}, Status: UpToDate}}
	checkCalled := 0
	stubCheckAll := func(_ context.Context, _ string, _ system.PlatformProfile) []UpdateResult {
		checkCalled++
		return stubResults
	}

	results := CheckAllWithCooldown(context.Background(), "1.0.0", profile, home, 6*time.Hour,
		func() time.Time { return now },
		stubCheckAll,
	)

	if checkCalled != 1 {
		t.Errorf("network check called %d times, want 1 (cache is stale)", checkCalled)
	}
	if len(results) != len(stubResults) {
		t.Errorf("results len = %d, want %d", len(results), len(stubResults))
	}

	// Verify LastUpdateCheck was updated to now on success.
	updated, err := state.Read(home)
	if err != nil {
		t.Fatalf("state.Read() error = %v", err)
	}
	if updated.LastUpdateCheck == nil || !updated.LastUpdateCheck.Equal(now) {
		t.Errorf("LastUpdateCheck after stale refresh = %v, want %v", updated.LastUpdateCheck, now)
	}
}

// TestCheckAllWithCooldown_MissingCache first-run always checks.
func TestCheckAllWithCooldown_MissingCache(t *testing.T) {
	home := t.TempDir()
	profile := system.PlatformProfile{OS: "darwin", PackageManager: "brew"}

	// No state file — first-run scenario.
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)

	checkCalled := 0
	stubCheckAll := func(_ context.Context, _ string, _ system.PlatformProfile) []UpdateResult {
		checkCalled++
		return nil
	}

	CheckAllWithCooldown(context.Background(), "1.0.0", profile, home, 6*time.Hour,
		func() time.Time { return now },
		stubCheckAll,
	)

	if checkCalled != 1 {
		t.Errorf("network check called %d times on first run, want 1", checkCalled)
	}
}

// TestCheckAllWithCooldown_FailedCheckDoesNotAdvanceTimestamp verifies that
// when the underlying check returns an error-flagged result (CheckFailed), the
// LastUpdateCheck timestamp is NOT updated, so the next launch retries.
func TestCheckAllWithCooldown_FailedCheckDoesNotAdvanceTimestamp(t *testing.T) {
	home := t.TempDir()
	profile := system.PlatformProfile{OS: "darwin", PackageManager: "brew"}

	// Stale cache — will attempt a refresh.
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-7 * time.Hour)
	s := state.InstallState{
		InstalledAgents: []string{"claude-code"},
		LastUpdateCheck: &stale,
	}
	if err := state.Write(home, s); err != nil {
		t.Fatalf("state.Write() error = %v", err)
	}

	// Return a failed-check result (all tools failed).
	failedResults := []UpdateResult{
		{Tool: ToolInfo{Name: "shevanio-ai"}, Status: CheckFailed},
	}
	stubCheckAll := func(_ context.Context, _ string, _ system.PlatformProfile) []UpdateResult {
		return failedResults
	}

	CheckAllWithCooldown(context.Background(), "1.0.0", profile, home, 6*time.Hour,
		func() time.Time { return now },
		stubCheckAll,
	)

	// LastUpdateCheck must NOT have advanced.
	updated, err := state.Read(home)
	if err != nil {
		t.Fatalf("state.Read() error = %v", err)
	}
	if updated.LastUpdateCheck != nil && updated.LastUpdateCheck.Equal(now) {
		t.Error("LastUpdateCheck was advanced after a failed check — must only advance on success")
	}
	// It should remain the original stale value.
	if updated.LastUpdateCheck == nil || !updated.LastUpdateCheck.Equal(stale) {
		t.Errorf("LastUpdateCheck = %v, want original stale %v", updated.LastUpdateCheck, stale)
	}
}

// TestCheckAllWithCooldown_EmptyHomeDirAlwaysChecks verifies that when homeDir
// is "" (home resolution failed), the check always runs and no state file is
// written anywhere.
func TestCheckAllWithCooldown_EmptyHomeDirAlwaysChecks(t *testing.T) {
	profile := system.PlatformProfile{OS: "darwin", PackageManager: "brew"}
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)

	// Ensure the CWD-relative state directory does not pre-exist from a
	// previous (buggy) run, so we can detect a fresh write unambiguously.
	cwd, _ := os.Getwd()
	stateInCWD := filepath.Join(cwd, ".shevanio-ai", "state.json")
	_ = os.Remove(stateInCWD)
	_ = os.Remove(filepath.Join(cwd, ".shevanio-ai"))

	checkCalled := 0
	stubCheckAll := func(_ context.Context, _ string, _ system.PlatformProfile) []UpdateResult {
		checkCalled++
		return []UpdateResult{{Tool: ToolInfo{Name: "shevanio-ai"}, Status: UpToDate}}
	}

	results := CheckAllWithCooldown(context.Background(), "1.0.0", profile, "", 6*time.Hour,
		func() time.Time { return now },
		stubCheckAll,
	)

	if checkCalled != 1 {
		t.Errorf("network check called %d times with empty homeDir, want 1 (always-check)", checkCalled)
	}
	if len(results) == 0 {
		t.Error("expected non-empty results from the check, got empty")
	}

	// Verify no state file was written to CWD or any relative path.
	if _, err := os.Stat(stateInCWD); err == nil {
		t.Errorf("state file was written to CWD (%s) — must not write when homeDir is empty", stateInCWD)
	}
}

// TestCheckAllWithCooldown_EmptyHomeDirNoStateRead verifies that when homeDir
// is "", the cooldown skip does not engage (no state.Read), so the check runs
// even when a real state file might exist somewhere.
func TestCheckAllWithCooldown_EmptyHomeDirNoStateRead(t *testing.T) {
	profile := system.PlatformProfile{OS: "darwin", PackageManager: "brew"}
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-1 * time.Minute)

	checkCalled := 0
	stubCheckAll := func(_ context.Context, _ string, _ system.PlatformProfile) []UpdateResult {
		checkCalled++
		return nil
	}

	// Even if we pretend there's a "recent" timestamp, with empty homeDir we
	// never read state so the cooldown cannot engage — check must run.
	_ = recent // intentionally unused: we cannot write to homeDir="" safely

	CheckAllWithCooldown(context.Background(), "1.0.0", profile, "", 6*time.Hour,
		func() time.Time { return now },
		stubCheckAll,
	)

	if checkCalled != 1 {
		t.Errorf("network check called %d times, want 1 — empty homeDir disables cooldown read", checkCalled)
	}
}

// TestCheckAllWithCooldown_FutureTimestampAlwaysChecks verifies that a future
// LastUpdateCheck (clock skew) does not cause the cooldown to skip the check.
// Negative elapsed must be treated as needing a check.
func TestCheckAllWithCooldown_FutureTimestampAlwaysChecks(t *testing.T) {
	home := t.TempDir()
	profile := system.PlatformProfile{OS: "darwin", PackageManager: "brew"}

	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	// LastUpdateCheck is 1 hour IN THE FUTURE relative to now.
	future := now.Add(1 * time.Hour)
	s := state.InstallState{
		InstalledAgents: []string{"claude-code"},
		LastUpdateCheck: &future,
	}
	if err := state.Write(home, s); err != nil {
		t.Fatalf("state.Write() error = %v", err)
	}

	checkCalled := 0
	stubCheckAll := func(_ context.Context, _ string, _ system.PlatformProfile) []UpdateResult {
		checkCalled++
		return nil
	}

	CheckAllWithCooldown(context.Background(), "1.0.0", profile, home, 6*time.Hour,
		func() time.Time { return now },
		stubCheckAll,
	)

	if checkCalled != 1 {
		t.Errorf("network check called %d times with future timestamp, want 1 (must not skip on negative elapsed)", checkCalled)
	}
}

// TestCheckAllWithCooldown_NonMissingReadErrorSkipsWrite verifies that when the
// state file exists but produces a non-missing read error (e.g. corrupt JSON),
// the timestamp write is skipped — existing state must never be clobbered.
func TestCheckAllWithCooldown_NonMissingReadErrorSkipsWrite(t *testing.T) {
	home := t.TempDir()
	profile := system.PlatformProfile{OS: "darwin", PackageManager: "brew"}
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)

	// Write a corrupt (non-parseable) state file so state.Read returns a
	// non-missing error (file exists but JSON is invalid).
	stateDir := filepath.Join(home, ".shevanio-ai")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	corruptPath := filepath.Join(stateDir, "state.json")
	if err := os.WriteFile(corruptPath, []byte("NOT VALID JSON"), 0o644); err != nil {
		t.Fatalf("WriteFile corrupt: %v", err)
	}

	stubCheckAll := func(_ context.Context, _ string, _ system.PlatformProfile) []UpdateResult {
		// Return a successful result — checkSucceeded will be true.
		return []UpdateResult{{Tool: ToolInfo{Name: "shevanio-ai"}, Status: UpToDate}}
	}

	// stale first read: corrupt file → read error → always-check (skip cooldown).
	// After check, re-read for write: corrupt → non-missing error → must skip write.
	CheckAllWithCooldown(context.Background(), "1.0.0", profile, home, 6*time.Hour,
		func() time.Time { return now },
		stubCheckAll,
	)

	// The corrupt file must still be corrupt — not overwritten with valid JSON.
	data, err := os.ReadFile(corruptPath)
	if err != nil {
		t.Fatalf("ReadFile after cooldown: %v", err)
	}
	if string(data) != "NOT VALID JSON" {
		t.Errorf("state file was overwritten; got %q, want original corrupt content", string(data))
	}
}
