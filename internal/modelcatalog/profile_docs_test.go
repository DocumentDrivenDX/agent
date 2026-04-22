package modelcatalog

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func TestRoutingProfileDocsMatchCatalog(t *testing.T) {
	catalog, err := Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}

	doc := readRoutingProfilesDoc(t)
	docProfiles := profileSections(doc)
	if len(docProfiles) == 0 {
		t.Fatal("docs/routing/profiles.md has no profile sections")
	}

	catalogProfiles := make(map[string]struct{})
	for _, profile := range catalog.Profiles() {
		catalogProfiles[profile.Name] = struct{}{}
		if _, ok := docProfiles[profile.Name]; !ok {
			t.Errorf("catalog profile %q missing from docs/routing/profiles.md", profile.Name)
		}
	}
	for profile := range docProfiles {
		if _, ok := catalogProfiles[profile]; !ok {
			t.Errorf("docs/routing/profiles.md names profile %q that is absent from catalog", profile)
		}
	}
}

func TestRoutingProfileDocsIncludeRequiredFields(t *testing.T) {
	docProfiles := profileSections(readRoutingProfilesDoc(t))
	required := []string{
		"- Name:",
		"- Intent:",
		"- ProviderPreference:",
		"- Expected cost class:",
		"- Example use case:",
		"- Hard constraints:",
	}
	for name, section := range docProfiles {
		for _, field := range required {
			if !strings.Contains(section, field) {
				t.Errorf("profile %q docs missing required field %q", name, field)
			}
		}
	}
}

func readRoutingProfilesDoc(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	data, err := os.ReadFile(filepath.Join(repoRoot, "docs", "routing", "profiles.md"))
	if err != nil {
		t.Fatalf("read docs/routing/profiles.md: %v", err)
	}
	return string(data)
}

func profileSections(doc string) map[string]string {
	headingRE := regexp.MustCompile("(?m)^### `([^`]+)`\\s*$")
	matches := headingRE.FindAllStringSubmatchIndex(doc, -1)
	out := make(map[string]string, len(matches))
	for i, match := range matches {
		name := doc[match[2]:match[3]]
		start := match[1]
		end := len(doc)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		out[name] = doc[start:end]
	}
	return out
}
