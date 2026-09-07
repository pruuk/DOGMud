package mobs

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

// TestMobYAML_NoUnknownTopLevelKeys guards against the class of bug that shipped
// 17 mobs carrying nothing: their YAML declared `items:` at column 0, but
// mobs.Mob has no Items field, so yaml.Unmarshal silently discarded the key.
// Nothing failed at load time, nothing failed at spawn time, and the only
// symptom was an empty corpse — a mob that quietly carried nothing.
//
// An unknown top-level key in a mob YAML binds to no field and is dropped
// without a word. This test derives the legal top-level key set by reflection
// over mobs.Mob (rather than hardcoding it) so a field added or retagged later
// cannot silently drift from what this guard allows, then walks every mob YAML
// in the world data tree and fails loudly, listing every offending file and
// key, if any top-level key falls outside that set.
//
// Deliberately TOP-LEVEL only: `level:` is nested under `character:` in 611
// files as a legacy field, so a blanket strict-unmarshal guard would trip on
// that in bulk and be useless as a gate. This test only walks the outermost
// map.
func TestMobYAML_NoUnknownTopLevelKeys(t *testing.T) {
	validKeys := validTopLevelYAMLKeys(t, reflect.TypeOf(Mob{}))

	root := repoRootForTest(t)
	mobsDir := filepath.Join(root, "_datafiles", "world", "dogmud", "mobs")

	var offenses []string

	err := filepath.Walk(mobsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".yaml") {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}

		var top map[string]any
		if unmarshalErr := yaml.Unmarshal(data, &top); unmarshalErr != nil {
			// Malformed YAML is a different failure mode; not this guard's job.
			return nil
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}

		keys := make([]string, 0, len(top))
		for k := range top {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			if !validKeys[k] {
				offenses = append(offenses, fmt.Sprintf("%s: unknown top-level key %q", filepath.ToSlash(rel), k))
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk %s: %v", mobsDir, err)
	}

	if len(offenses) > 0 {
		sort.Strings(offenses)
		t.Fatalf("mob YAML files contain top-level keys that bind to no field on mobs.Mob "+
			"(silently discarded by yaml.Unmarshal):\n%s", strings.Join(offenses, "\n"))
	}
}

// validTopLevelYAMLKeys derives the set of yaml keys mobs.Mob will actually
// bind at the top level, by reflecting over its exported fields.
func validTopLevelYAMLKeys(t *testing.T, mobType reflect.Type) map[string]bool {
	t.Helper()

	valid := make(map[string]bool)

	for i := 0; i < mobType.NumField(); i++ {
		field := mobType.Field(i)

		// Unexported fields are never settable by yaml.Unmarshal.
		if field.PkgPath != "" {
			continue
		}

		tag, hasTag := field.Tag.Lookup("yaml")
		if !hasTag {
			valid[strings.ToLower(field.Name)] = true
			continue
		}

		name := strings.SplitN(tag, ",", 2)[0]
		if name == "-" {
			// Explicitly excluded from YAML (e.g. runtime-only fields).
			continue
		}
		if name == "" {
			// yaml.v2 falls back to the lowercased field name when the tag
			// omits a name (e.g. `yaml:",omitempty"`).
			name = strings.ToLower(field.Name)
		}
		valid[name] = true
	}

	return valid
}
