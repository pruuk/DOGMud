package guards

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"gopkg.in/yaml.v2"
)

func shippedTopicKeys(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("_datafiles", "world", "dogmud", "dialogue", "*", "*.yaml"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("found no dialogue files (err %v): the guard cannot run", err)
	}
	var keys []string
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		var df dialogue.DialogueFile
		if err := yaml.UnmarshalStrict(data, &df); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		for _, tp := range df.Topics {
			keys = append(keys, dialogue.HeardKey(df.MobId, tp.Key))
		}
	}
	sort.Strings(keys)
	return keys
}

// TestTopicKeyManifest: saved characters hold heard keys, so a shipped key
// may leave content only through a "# retired" or "# renamed" line, and a
// new key must be added to the manifest.
func TestTopicKeyManifest(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("_datafiles", "world", "dogmud", "dialogue", "topic-keys.txt"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	m, err := dialogue.ParseManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	missing, unlisted := m.Diff(shippedTopicKeys(t))
	for _, k := range missing {
		t.Errorf("topic key %s is gone from content: add it under \"# retired\" or \"# renamed\" (old -> new)", k)
	}
	for _, k := range unlisted {
		t.Errorf("topic key %s is not in topic-keys.txt: append it", k)
	}
}

// TestTopicKeyManifest_GuardCanFail proves the guard sees a removed key.
func TestTopicKeyManifest_GuardCanFail(t *testing.T) {
	keys := shippedTopicKeys(t)
	m := dialogue.Manifest{Keys: append([]string{"0.removed_on_purpose"}, keys...)}
	missing, _ := m.Diff(keys)
	if len(missing) != 1 || missing[0] != "0.removed_on_purpose" {
		t.Fatalf("guard did not see the removed key: %v", missing)
	}
}
