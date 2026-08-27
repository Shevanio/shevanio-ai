package statestore

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestU1322WriterInventoryDetectsEveryRawProductionWriter(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	entries := []struct{ path, function, call string }{
		{"internal/update/cooldown.go", "CheckAllWithCooldown", "statestore.WithLock"},
		{"internal/app/selfupdate.go", "selfUpdate", "statestore.WithLock"},
		{"internal/app/app.go", "RunArgs", "statestore.WithLock"},
		{"internal/app/app.go", "tuiExecuteWithBackground", "statestore.WithLock"},
		{"internal/app/app.go", "persistAssignments", "statestore.WithLock"},
		{"internal/tui/model.go", "startUpgradeSync", "statestore.WithLock"},
		{"internal/cli/review_mode.go", "writeGlobalRDDMode", "statestore.WithLock"},
		{"internal/components/uninstall/service.go", "updateStateAfterUninstall", "statestore.WithLock"},
		{"internal/cli/sync.go", "migratePersistedPersonaAlias", "statestore.WithLock"},
		{"internal/cli/run.go", "persistInstallState", "withInstallStateLock"},
		{"internal/cli/sync.go", "persistSyncManagedAssetStateWithBackground", "withInstallStateLock"},
		{"internal/cli/gga_retirement.go", "MigrateLegacyGGA", "withInstallStateLock"},
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(root, entry.path))
		body := functionSource(string(data), entry.function)
		if err != nil || !strings.Contains(body, entry.call) ||
			(entry.call == "statestore.WithLock" && strings.Contains(body, "state.Write")) {
			t.Fatalf("behavior mismatch: TestU1322WriterInventoryDetectsEveryRawProductionWriter")
		}
	}
}

func functionSource(source, name string) string {
	start := strings.Index(source, "func "+name)
	method := strings.Index(source, ") "+name)
	if start < 0 || (method >= 0 && method < start) {
		start = strings.LastIndex(source[:method], "func")
	}
	if start < 0 {
		return ""
	}
	end := strings.Index(source[start+len("func "):], "\nfunc ")
	if end >= 0 {
		return source[start : start+len("func ")+end]
	}
	return source[start:]
}
