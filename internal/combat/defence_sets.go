package combat

import "github.com/GoMudEngine/GoMud/internal/characters"

// AttackChannel names an attack type. The applicable defence set is a property
// of the channel, which is the whole reason this is data rather than a filter
// function scattered across the resolvers.
type AttackChannel string

const (
	ChannelMelee         AttackChannel = "melee"
	ChannelRanged        AttackChannel = "ranged"
	ChannelSpellPhysical AttackChannel = "spell-physical"
	ChannelSpellMental   AttackChannel = "spell-mental"
	ChannelSocial        AttackChannel = "social"
)

// DefenceSetFor returns the defences that apply to a channel.
//
// Adding a defence to a channel is one row here and nothing else, which is the
// point of the design. Parry is deliberately excluded from ranged and physical
// spells -- you cannot parry a bolt. Dodge is REUSED for physical spells; there
// is no separate physical-spell defence.
//
// quell (Wil + spellcasting x SkillWeight) answers mental spells; defy
// (Wil + rhetoric x SkillWeight) answers social attacks. A set of size one is
// still a contest, not a different mechanism -- that unification is what let
// avoidance.go be deleted in Task 12.
//
// WIRED, but not everywhere. ResolveChannelDefence consumes this table for the
// three non-melee channels: the five spell sites in internal/hooks and the taunt
// site in internal/actions. MELEE DOES NOT. runBestOfAllDefense still builds its
// own equipment-derived defSeq from characters.GetDefenseSequence, so adding a
// defence to the ChannelMelee row here changes nothing on its own.
//
// Two things a new row must carry with it. It needs an arm in
// characters.GetDefenseScore, or it enters every contest at 0 and always loses
// (TestDefenceSetForReturnsKnownDefenceNames is the guard). And it needs a row
// in characters.DefensePool if it is not paid in stamina, or the pair charges
// the wrong pool.
func DefenceSetFor(channel AttackChannel) []string {
	switch channel {
	case ChannelMelee:
		return []string{characters.DefenseDodge, characters.DefenseParry, characters.DefenseBlock}
	case ChannelRanged, ChannelSpellPhysical:
		return []string{characters.DefenseDodge, characters.DefenseBlock}
	case ChannelSpellMental:
		return []string{characters.DefenseQuell}
	case ChannelSocial:
		return []string{characters.DefenseDefy}
	default:
		return nil
	}
}
