package docs

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/shevanio/shevanio-ai/v2/internal/catalog"
	"github.com/shevanio/shevanio-ai/v2/internal/model"
)

type ProductGuideAudit struct{ Findings []string }

func (a ProductGuideAudit) HasFindings() bool { return len(a.Findings) != 0 }

func (a ProductGuideAudit) String() string { return strings.Join(a.Findings, "\n") }

var (
	linkRE = regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)
	flagRE = regexp.MustCompile(`--[a-z][a-z0-9-]*`)
	cmdRE  = regexp.MustCompile(`^  ([a-z][a-z0-9-]+)(?: +([^ ]+))?`)
	ggaRE  = regexp.MustCompile(`(?i)(^|[^a-z0-9])gga([^a-z0-9]|$)`)
)

// AuditProductGuide derives the current inventory from the catalog and CLI
// help sources, then checks README coverage, maintained links, and active docs.
func AuditProductGuide(root string) (ProductGuideAudit, error) {
	var audit ProductGuideAudit
	readme, err := read(root, "README.md")
	if err != nil {
		return audit, err
	}
	haystack := strings.ToLower(readme)
	need := func(term, label string) {
		if !strings.Contains(haystack, strings.ToLower(term)) {
			audit.Findings = append(audit.Findings, fmt.Sprintf("missing coverage: %s (%s)", label, term))
		}
	}
	for _, agent := range catalog.AllAgents() {
		need(string(agent.ID), "agent "+agent.Name)
	}
	for _, component := range catalog.MVPComponents() {
		need(string(component.ID), "component "+component.Name)
	}
	for _, skill := range catalog.MVPSkills() {
		need(skill.Name, "skill "+skill.Name)
	}
	for _, preset := range []model.PresetID{model.PresetMinimal, model.PresetEcosystemOnly, model.PresetFullGentleman, model.PresetCustom} {
		need(string(preset), "preset "+string(preset))
	}

	help, err := read(root, filepath.Join("internal", "app", "help.go"))
	if err != nil {
		return audit, err
	}
	for _, command := range commands(help) {
		need(command, "command "+command)
	}
	for _, name := range []string{"PrintInstallHelp", "PrintSyncHelp"} {
		path := filepath.Join("internal", "cli", "install.go")
		if name == "PrintSyncHelp" {
			path = filepath.Join("internal", "cli", "sync.go")
		}
		source, readErr := read(root, path)
		if readErr != nil {
			return audit, readErr
		}
		for _, flag := range unique(flagRE.FindAllString(functionBody(source, name), -1)) {
			need(flag, "flag "+flag)
		}
	}

	for _, path := range maintainedDocs(root) {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return audit, readErr
		}
		if ggaRE.Match(content) {
			audit.Findings = append(audit.Findings, "retired GGA guidance: "+relative(root, path))
		}
		for _, match := range linkRE.FindAllStringSubmatch(string(content), -1) {
			if target := localTarget(match[1]); target != "" {
				candidate := filepath.Join(filepath.Dir(path), filepath.FromSlash(target))
				if _, statErr := os.Stat(candidate); statErr != nil {
					audit.Findings = append(audit.Findings, fmt.Sprintf("broken link: %s -> %s", relative(root, path), target))
				}
			}
		}
	}
	audit.Findings = unique(audit.Findings)
	return audit, nil
}

func maintainedDocs(root string) []string {
	var paths []string
	for _, pattern := range []string{"docs/*.md", "docs/codebase/*.md", "docs/architecture/*.md"} {
		matches, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern)))
		paths = append(paths, matches...)
	}
	paths = append(paths, filepath.Join(root, "README.md"))
	legacy := filepath.Join(root, "docs", "architecture", "rdd-backlog-disposition.md")
	filtered := paths[:0]
	for _, path := range paths {
		if path != legacy {
			filtered = append(filtered, path)
		}
	}
	sort.Strings(filtered)
	return filtered
}

func commands(help string) []string {
	var result []string
	inSection := false
	scanner := bufio.NewScanner(strings.NewReader(help))
	for scanner.Scan() {
		line := strings.TrimPrefix(scanner.Text(), "\t")
		if line == "COMMANDS" || line == "COMPATIBILITY COMMANDS" {
			inSection = true
			continue
		}
		if inSection && line == "FLAGS" {
			inSection = false
		}
		if !inSection {
			continue
		}
		match := cmdRE.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		command := match[1]
		if (command == "review" || command == "skill-registry") && match[2] != "" && !strings.HasPrefix(match[2], "[") && !strings.HasPrefix(match[2], "<") {
			command += " " + match[2]
		}
		result = append(result, command)
	}
	return unique(result)
}

func functionBody(source, name string) string {
	start := strings.Index(source, "func "+name+"(")
	if start < 0 {
		return ""
	}
	body := source[start:]
	if next := strings.Index(body, "\nfunc "); next >= 0 {
		body = body[:next]
	}
	return body
}

func localTarget(raw string) string {
	target := strings.Trim(strings.TrimSpace(raw), "<>")
	if target == "" || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "//") || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
		return ""
	}
	for _, separator := range []string{"#", "?"} {
		if index := strings.Index(target, separator); index >= 0 {
			target = target[:index]
		}
	}
	return strings.TrimSpace(target)
}

func read(root, path string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, path))
	return string(data), err
}

func relative(root, path string) string {
	result, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(result)
}

func unique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
