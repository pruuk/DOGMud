package mobs

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

// TestItemDropChance_YAMLAbsentVersusZero pins #585: an authored
// `itemdropchance: 0` loads as a set 0 (worn items never drop), an absent key
// loads as unset (DefaultItemDropChance, every worn item drops), and a
// positive value loads as itself. Before the fix the field was a plain int, so
// 0 and absent were the same value.
func TestItemDropChance_YAMLAbsentVersusZero(t *testing.T) {
	cases := []struct {
		name    string
		yamlSrc string
		wantSet bool
		want    int
	}{
		{"explicit zero is never", "mobid: 1\nitemdropchance: 0\n", true, 0},
		{"absent key is the default", "mobid: 2\n", false, DefaultItemDropChance},
		{"positive value is kept", "mobid: 3\nitemdropchance: 25\n", true, 25},
	}
	for _, c := range cases {
		var m Mob
		if err := yaml.Unmarshal([]byte(c.yamlSrc), &m); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if (m.ItemDropChance != nil) != c.wantSet {
			t.Fatalf("%s: ItemDropChance set = %v, want %v", c.name, m.ItemDropChance != nil, c.wantSet)
		}
		if got := m.EquippedDropChance(); got != c.want {
			t.Fatalf("%s: EquippedDropChance() = %d, want %d", c.name, got, c.want)
		}
	}
	if DefaultItemDropChance != 100 {
		t.Fatalf("an absent itemdropchance must keep the pre-#585 behaviour (every worn item drops), got %d", DefaultItemDropChance)
	}
}

// TestItemDropChance_SaveRoundTrip proves a template save keeps the
// distinction: an unset chance writes no key (so it reloads as the default)
// and a set 0 writes `itemdropchance: 0` (so it reloads as never). The old
// untagged int wrote `itemdropchance: 0` for every mob, absent or not.
func TestItemDropChance_SaveRoundTrip(t *testing.T) {
	unset, err := yaml.Marshal(Mob{MobId: 1})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(unset), "itemdropchance") {
		t.Fatalf("an unset itemdropchance must not be written, got:\n%s", unset)
	}
	zero := 0
	set, err := yaml.Marshal(Mob{MobId: 2, ItemDropChance: &zero})
	if err != nil {
		t.Fatal(err)
	}
	var back Mob
	if err := yaml.Unmarshal(set, &back); err != nil {
		t.Fatal(err)
	}
	if back.ItemDropChance == nil || back.EquippedDropChance() != 0 {
		t.Fatalf("a set 0 must survive a save and reload as never, got set=%v chance=%d from:\n%s",
			back.ItemDropChance != nil, back.EquippedDropChance(), set)
	}
}

// TestItemDropChance_ShippedZeroLoadsAsNever is the data guard for #585: every
// shipped mob file that authors `itemdropchance: 0` must load as a set 0
// (never), not as absent. It reads the real mob YAML under _datafiles/world.
func TestItemDropChance_ShippedZeroLoadsAsNever(t *testing.T) {
	root := filepath.Join(repoRootForTest(t), "_datafiles", "world")
	zeroKey := regexp.MustCompile(`(?m)^itemdropchance:\s*0\s*$`)
	checked := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		if !strings.Contains(filepath.ToSlash(path), "/mobs/") || strings.Contains(filepath.ToSlash(path), "/mobs.instances/") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !zeroKey.Match(raw) {
			return nil
		}
		var m Mob
		if err := yaml.Unmarshal(raw, &m); err != nil {
			return err
		}
		if m.ItemDropChance == nil || m.EquippedDropChance() != 0 {
			t.Errorf("%s authors itemdropchance: 0 but loads as set=%v chance=%d (want a set 0, never)",
				path, m.ItemDropChance != nil, m.EquippedDropChance())
		}
		checked++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// The walk must be able to find something, or a green run proves nothing.
	if checked < 100 {
		t.Fatalf("found only %d mob files with itemdropchance: 0 under %s; the walk is not reading the shipped data", checked, root)
	}
}
