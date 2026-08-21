package spells

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"
)

// TestNoManifestationSpellContestsThroughTheSharedChannel guards the data
// property U9's Step 5a relies on: combat.ChannelAttackScore
// (internal/combat/defence_multiplier.go) hardcodes Willpower and stays that
// way on purpose, because no manifestation-school spell reaches it today.
// The 13 summon/conjure/raise spells declare target_defense_type: none and
// resolve through resolveCompanionSummon (nothing to contest); charm is the
// only manifestation spell that attacks another character, and it resolves
// through its own bespoke path (resolveCharmSpell), never touching the shared
// channel.
//
// The charisma/willpower split is prevented by data, not by code:
// ChannelAttackScore hardcodes Willpower, and no manifestation-school spell
// reaches it. Authoring a contesting manifestation spell breaks that silently,
// so fail loudly here instead. If this fires, either thread primarystat into
// ChannelAttackScore or give the new spell its own resolution path as charm has.
//
// Reads the shipped YAML directly (same technique as
// TestShippedSummonPetMultipliers) rather than going through GetAllSpells(),
// so this test does not depend on LoadSpellFiles() or SeedSpellsForTest()
// having run first.
func TestNoManifestationSpellContestsThroughTheSharedChannel(t *testing.T) {
	entries, err := os.ReadDir(shippedSpellDir)
	if err != nil {
		t.Fatalf("reading %s: %v", shippedSpellDir, err)
	}

	checked := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}

		raw, err := os.ReadFile(filepath.Join(shippedSpellDir, e.Name()))
		if err != nil {
			t.Errorf("%s: %v", e.Name(), err)
			continue
		}

		var sd SpellData
		if err := yaml.Unmarshal(raw, &sd); err != nil {
			t.Errorf("%s: unmarshal: %v", e.Name(), err)
			continue
		}

		if !sd.HasSchool(SchoolManifestation) {
			continue
		}
		if sd.SpellId == "charm" {
			continue // bespoke path, see resolveCharmSpell
		}
		checked++

		if sd.TargetDefenseType != "" && sd.TargetDefenseType != "none" {
			t.Errorf("manifestation spell %q (%s) declares target_defense_type %q; "+
				"it will contest on Willpower through combat.ChannelAttackScore while "+
				"its primarystat says %q",
				sd.SpellId, e.Name(), sd.TargetDefenseType, sd.PrimaryStat)
		}
	}

	if checked == 0 {
		t.Fatal("found 0 non-charm manifestation spells on disk; the guard didn't check anything -- " +
			"has the shipped spell set moved?")
	}
}
