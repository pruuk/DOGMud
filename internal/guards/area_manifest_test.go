package guards

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/areas"
	"github.com/GoMudEngine/GoMud/internal/manifest"
	"gopkg.in/yaml.v2"
)

func shippedAreaKeys(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("_datafiles", "world", "dogmud", "areas", "*.yaml"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("found no area files (err %v): the guard cannot run", err)
	}
	var keys []string
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		var a areas.Area
		if err := yaml.UnmarshalStrict(data, &a); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if want := strings.TrimSuffix(filepath.Base(p), ".yaml"); a.Key != want {
			t.Errorf("%s: key %q is not the file name", p, a.Key)
		}
		keys = append(keys, a.Key)
	}
	sort.Strings(keys)
	return keys
}

// TestAreaKeyManifest: quest triggers name area keys, so a shipped key may
// leave content only through a "# retired" or "# renamed" line, and a new
// key must be added to the manifest.
func TestAreaKeyManifest(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("_datafiles", "world", "dogmud", "areas", "area-keys.txt"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	m, err := manifest.Parse("area-keys.txt", data)
	if err != nil {
		t.Fatal(err)
	}
	missing, unlisted := m.Diff(shippedAreaKeys(t))
	for _, k := range missing {
		t.Errorf("area key %s is gone from content: add it under \"# retired\" or \"# renamed\" (old -> new)", k)
	}
	for _, k := range unlisted {
		t.Errorf("area key %s is not in area-keys.txt: append it", k)
	}
}

// TestAreaKeyManifest_GuardCanFail proves the guard sees a removed key.
func TestAreaKeyManifest_GuardCanFail(t *testing.T) {
	keys := shippedAreaKeys(t)
	m := manifest.Manifest{Keys: append([]string{"removed_on_purpose"}, keys...)}
	missing, _ := m.Diff(keys)
	if len(missing) != 1 || missing[0] != "removed_on_purpose" {
		t.Fatalf("guard did not see the removed key: %v", missing)
	}
}
