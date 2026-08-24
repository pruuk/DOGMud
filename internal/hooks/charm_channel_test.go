package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

// Charm declares target_defense_type: social so the cast it ALREADY makes is
// answered by defy instead of quell. An absent value must keep meaning
// ChannelSpellMental -- it is the default for every unclassified spell, not an
// escape from routing, and reading it as one is what produced two rejected
// plans for this slice.
func TestSpellAttackChannel_Routing(t *testing.T) {
	cases := []struct {
		name string
		tdt  string
		want combat.AttackChannel
	}{
		{"social routes to social", "social", combat.ChannelSocial},
		{"physical routes to physical", "physical", combat.ChannelSpellPhysical},
		{"absent stays mental", "", combat.ChannelSpellMental},
		{"unknown stays mental", "nonsense", combat.ChannelSpellMental},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := spellAttackChannel(&spells.SpellData{TargetDefenseType: c.tdt}); got != c.want {
				t.Errorf("spellAttackChannel(%q) = %v, want %v", c.tdt, got, c.want)
			}
		})
	}

	if got := spellAttackChannel(nil); got != combat.ChannelSpellMental {
		t.Errorf("spellAttackChannel(nil) = %v, want ChannelSpellMental", got)
	}
}
