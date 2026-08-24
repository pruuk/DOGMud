package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// ChannelSocial was unreachable in counterPoolFor because taunt, the only
// social attack, short-circuits its defy-crit into a counter-taunt at the call
// site. U10c made charm a social SPELL, and fireSpellCounterTier has no such
// carve-out, so a defy-crit against a charm now arrives here.
//
// Without a case it would fall through to the physical melee pool and a mob
// would answer a charm with a sword-swing narration.
func TestCounterPoolFor_SocialUsesTheDefyPool(t *testing.T) {
	if got := counterPoolFor(ChannelSocial); got != items.DefenseCounterDefy {
		t.Errorf("counterPoolFor(ChannelSocial) = %v, want %v (not the physical fallthrough)",
			got, items.DefenseCounterDefy)
	}
}

func TestCounterPoolFor_OtherChannelsUnchanged(t *testing.T) {
	cases := map[AttackChannel]items.DefenseType{
		ChannelRanged:        items.DefenseCounterRanged,
		ChannelSpellPhysical: items.DefenseCounterQuell,
		ChannelSpellMental:   items.DefenseCounterQuell,
		ChannelMelee:         items.DefenseCounterMelee,
	}
	for channel, want := range cases {
		if got := counterPoolFor(channel); got != want {
			t.Errorf("counterPoolFor(%v) = %v, want %v", channel, got, want)
		}
	}
}
