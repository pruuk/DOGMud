package dialogue

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Manifest is the committed list of shipped topic keys. Saved characters
// point at these keys (HeardTopics), so a key may leave content only through
// a retired or renamed line.
type Manifest struct {
	Keys    []string
	Retired []string
	Renamed map[string]string // old -> new
}

// ParseManifest reads the manifest text: keys, then optional "# retired"
// and "# renamed" sections ("old -> new"). Other comment lines are ignored.
func ParseManifest(data []byte) (Manifest, error) {
	m := Manifest{Renamed: map[string]string{}}
	section := "keys"
	for n, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "":
			continue
		case line == "# retired":
			section = "retired"
			continue
		case line == "# renamed":
			section = "renamed"
			continue
		case strings.HasPrefix(line, "#"):
			continue
		}
		switch section {
		case "keys":
			m.Keys = append(m.Keys, line)
		case "retired":
			m.Retired = append(m.Retired, line)
		case "renamed":
			old, nu, ok := strings.Cut(line, "->")
			if !ok || strings.TrimSpace(old) == "" || strings.TrimSpace(nu) == "" {
				return m, fmt.Errorf("topic-keys.txt line %d: a rename is \"old -> new\": %q", n+1, line)
			}
			m.Renamed[strings.TrimSpace(old)] = strings.TrimSpace(nu)
		}
	}
	sort.Strings(m.Keys)
	return m, nil
}

// Diff compares content keys with the manifest: missing are listed keys gone
// from content with no retired or renamed line; unlisted are content keys
// the manifest does not list yet.
func (m Manifest) Diff(content []string) (missing, unlisted []string) {
	have := map[string]bool{}
	for _, k := range content {
		have[k] = true
	}
	listed := map[string]bool{}
	for _, k := range m.Keys {
		listed[k] = true
	}
	excused := map[string]bool{}
	for _, k := range m.Retired {
		excused[k] = true
	}
	for k := range m.Renamed {
		excused[k] = true
	}
	for _, k := range m.Keys {
		if !have[k] && !excused[k] {
			missing = append(missing, k)
		}
	}
	for _, k := range content {
		if !listed[k] {
			unlisted = append(unlisted, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(unlisted)
	return missing, unlisted
}

// HeardRenames reads the shipped manifest's renames for the load migration.
// A missing or unreadable manifest renames nothing.
func HeardRenames() map[string]string {
	path := util.FilePath(string(configs.GetFilePathsConfig().DataFiles) + `/dialogue/topic-keys.txt`)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	m, err := ParseManifest(data)
	if err != nil {
		return nil
	}
	return m.Renamed
}
