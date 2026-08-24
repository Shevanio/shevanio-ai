package docs

import (
	"path/filepath"
	"testing"
)

func TestCurrentProductGuideAudit(t *testing.T) {
	t.Parallel()

	report, err := AuditProductGuide(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("AuditProductGuide() error = %v", err)
	}
	if report.HasFindings() {
		t.Fatalf("current product guide audit failed:\n%s", report)
	}
}

func TestLocalLinkTarget(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
		want string
	}{
		{name: "relative document with anchor", raw: "docs/usage.md#sync", want: "docs/usage.md"},
		{name: "same document anchor", raw: "#command-reference", want: ""},
		{name: "external URL", raw: "https://example.com/guide", want: ""},
		{name: "angle bracket path", raw: "<docs/rollback.md>", want: "docs/rollback.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := localTarget(tc.raw); got != tc.want {
				t.Fatalf("localTarget(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestExtractCommandsKeepsSubcommandsAndCompatibilityPaths(t *testing.T) {
	t.Parallel()

	help := "\tCOMMANDS\n  install      Configure\n  review start [flags]\n\tCOMPATIBILITY COMMANDS\n  review-start --cwd <repo>\n\tFLAGS\n"
	want := []string{"install", "review start", "review-start"}
	got := commands(help)
	if len(got) != len(want) {
		t.Fatalf("commands() = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("commands()[%d] = %q, want %q", index, got[index], want[index])
		}
	}
}
