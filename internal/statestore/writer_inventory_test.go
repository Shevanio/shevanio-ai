package statestore

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"
)

const stateImport, storeImport = "github.com/shevanio/shevanio-ai/v2/internal/state", "github.com/shevanio/shevanio-ai/v2/internal/statestore"

func TestU1322WriterInventoryDetectsEveryRawProductionWriter(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	entries := approvedWriterEntries()
	if err := validateWriterInventory(root, entries); err != nil {
		t.Fatalf("behavior mismatch: TestU1322WriterInventoryDetectsEveryRawProductionWriter")
	}
	fixture := t.TempDir()
	for i := range entries {
		entries[i].path = "fixture.go"
	}
	writeSyntheticWriterFixture(t, fixture, entries)
	if err := validateWriterInventory(fixture, entries); err == nil {
		t.Fatalf("behavior mismatch: TestU1322WriterInventoryDetectsEveryRawProductionWriter")
	}
}

type writerEntry struct{ path, function, call string }

func approvedWriterEntries() []writerEntry {
	return []writerEntry{
		{"internal/update/cooldown.go", "CheckAllWithCooldown", "statestore.Mutate"}, {"internal/app/selfupdate.go", "selfUpdate", "statestore.Mutate"},
		{"internal/app/app.go", "RunArgs", "statestore.Mutate"}, {"internal/app/app.go", "tuiExecuteWithBackground", "statestore.Mutate"}, {"internal/app/app.go", "persistAssignments", "statestore.Mutate"},
		{"internal/tui/model.go", "startUpgradeSync", "statestore.Mutate"}, {"internal/cli/review_mode.go", "writeGlobalRDDMode", "statestore.Mutate"}, {"internal/components/uninstall/service.go", "updateStateAfterUninstall", "statestore.Mutate"},
		{"internal/cli/sync.go", "migratePersistedPersonaAlias", "statestore.Mutate"}, {"internal/cli/run.go", "persistInstallState", "statestore.Mutate"}, {"internal/cli/sync.go", "persistSyncManagedAssetStateWithBackground", "statestore.Mutate"}, {"internal/cli/gga_retirement.go", "MigrateLegacyGGA", "withInstallStateLock"},
	}
}
func validateWriterInventory(root string, want []writerEntry) error {
	got, err := scanWriterInventory(root)
	if err != nil {
		return err
	}
	sortWriterEntries(got)
	sortWriterEntries(want)
	if !slices.Equal(got, want) {
		return os.ErrInvalid
	}
	return nil
}
func scanWriterInventory(root string) (found []writerEntry, result error) {
	result = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "internal/state/") || strings.HasPrefix(rel, "internal/statestore/") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		aliases := map[string]string{}
		for _, imp := range file.Imports {
			importPath := imp.Path.Value[1 : len(imp.Path.Value)-1]
			name := filepath.Base(importPath)
			if imp.Name != nil {
				name = imp.Name.Name
			}
			aliases[name] = importPath
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			hasHelper := false
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				if call, ok := node.(*ast.CallExpr); ok {
					if helper, ok := call.Fun.(*ast.Ident); ok && helper.Name == "withInstallStateLock" {
						hasHelper = true
						return false
					}
				}
				return true
			})
			kinds := map[string]bool{}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				if call, ok := node.(*ast.CallExpr); ok {
					kind := writerCall(call, aliases)
					if kind != "" && !(strings.HasPrefix(kind, "state.") && hasHelper) {
						kinds[kind] = true
					}
				}
				return true
			})
			for kind := range kinds {
				found = append(found, writerEntry{rel, fn.Name.Name, kind})
			}
		}
		return nil
	})
	sortWriterEntries(found)
	return found, result
}
func writerCall(call *ast.CallExpr, aliases map[string]string) string {
	if fn, ok := call.Fun.(*ast.Ident); ok {
		if fn.Name == "withInstallStateLock" {
			return fn.Name
		}
	}
	fn, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	x, ok := fn.X.(*ast.Ident)
	if !ok {
		return ""
	}
	if aliases[x.Name] == stateImport && (fn.Sel.Name == "Write" || fn.Sel.Name == "WriteReconciled") {
		return "state." + fn.Sel.Name
	}
	if aliases[x.Name] == storeImport && (fn.Sel.Name == "WithLock" || fn.Sel.Name == "Mutate") {
		return "statestore." + fn.Sel.Name
	}
	return ""
}
func sortWriterEntries(entries []writerEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].path+"\x00"+entries[i].function+"\x00"+entries[i].call < entries[j].path+"\x00"+entries[j].function+"\x00"+entries[j].call
	})
}
func writeSyntheticWriterFixture(t *testing.T, root string, entries []writerEntry) {
	source := "package fixture\nimport statestore \"" + storeImport + "\"\nimport st \"" + stateImport + "\"\n"
	for _, entry := range entries {
		source += "func " + entry.function + "(){" + entry.call + "(\"\", nil)}\n"
	}
	source += "func unlistedWriter(){ st.Write(\"\", st.InstallState{}) }\n"
	if err := os.WriteFile(filepath.Join(root, "fixture.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
}
