package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	yamlv2 "gopkg.in/yaml.v2"
)

// keeperMobFields is the slice of a mob file this guard reads.
type keeperMobFields struct {
	MobId        int      `yaml:"mobid"`
	Crafter      bool     `yaml:"crafter"`
	NonCombatant bool     `yaml:"non_combatant"`
	Groups       []string `yaml:"groups"`
	Character    struct {
		Name string        `yaml:"name"`
		Shop []interface{} `yaml:"shop"`
	} `yaml:"character"`
}

// TestShopkeepersAndInnkeepersAreNonCombatant keeps every shopkeeper, crafter
// and innkeeper out of reach of player harm (#366, owner ruling 2026-10-13):
// a mob that stocks a shop, crafts for one, or is in the innkeeper group must
// carry non_combatant, which mobs.HarmAuthorization reads for attacks, theft
// and harm spells. Other quest-givers are not covered here; their protection
// comes with the quest and dialogue rework (#375).
func TestShopkeepersAndInnkeepersAreNonCombatant(t *testing.T) {
	root := shippedWorldRoot + "/mobs"
	checked := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		var m keeperMobFields
		if uerr := yamlv2.Unmarshal(data, &m); uerr != nil {
			t.Errorf("%s: parse: %v", path, uerr)
			return nil
		}
		innkeeper := false
		for _, g := range m.Groups {
			if g == "innkeeper" {
				innkeeper = true
			}
		}
		if len(m.Character.Shop) == 0 && !m.Crafter && !innkeeper {
			return nil
		}
		checked++
		if !m.NonCombatant {
			t.Errorf("%s: mob %d (%s) keeps a shop, crafts or keeps an inn but is not non_combatant: true",
				filepath.ToSlash(path), m.MobId, m.Character.Name)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	// Proves the walk reached the data: over a hundred shop mobs ship.
	if checked < 100 {
		t.Fatalf("checked %d keeper mobs, want at least 100; the walk or the field names are broken", checked)
	}
}
