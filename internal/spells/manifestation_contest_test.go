package spells

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/stats"
)

// TestContestingManifestationSpellResolvesWithItsDeclaredStat is the
// INVERSION of the retired manifestation_channel_guard_test.go. That guard
// forbade manifestation spells from declaring a contested target_defense_type
// because the deleted channel score helper hardcoded Willpower + Spellcasting, so a
// contesting manifestation spell would have attacked with the wrong stat and
// skill. U6b Task 4 deleted that constraint's reason: the resolver now builds
// a combat.AttackSide from the spell's own primarystat (CasterStatValue) and
// its school's governing skill, so a contesting manifestation spell is
// legitimate authored content.
//
// This is a loader-level proof: a manifestation spell that declares a
// contested defense type passes Validate (the load gate), and CasterStatValue
// resolves to the DECLARED stat's value — the value the hit contest's
// AttackSide is built from — not to the old hardcoded Willpower.
func TestContestingManifestationSpellResolvesWithItsDeclaredStat(t *testing.T) {
	sd := &SpellData{
		SpellId:           "test-charisma-lash",
		Name:              "Charisma Lash",
		Type:              HarmSingle,
		EffectType:        "damage",
		TargetDefenseType: "mental", // contested — the retired guard forbade this
		PrimaryStat:       "charisma",
		Schools:           []string{SchoolManifestation},
		BaseFolds:         2,
		EffectMagnitude:   10,
	}

	if err := sd.Validate(); err != nil {
		t.Fatalf("a contesting manifestation spell must now load cleanly, got: %v", err)
	}

	var st stats.Statistics
	st.Charisma.ValueAdj = 140
	st.Willpower.ValueAdj = 60

	if got := sd.CasterStatValue(st); got != 140 {
		t.Errorf("CasterStatValue = %d, want 140 (the declared charisma) -- "+
			"the hit contest's AttackSide must be built from the spell's primarystat, not hardcoded Willpower", got)
	}
}
