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
// still a contest, not a different mechanism -- that unification is what lets
// avoidance.go be deleted in Task 12.
//
// NOT WIRED YET. Melee still hands runBestOfAllDefense its own equipment-derived
// defSeq (characters.GetDefenseSequence), and the non-physical channels still go
// through TrySpellDeflection / TryStoicResolve in avoidance.go. Task 12 does the
// wiring; this is the data it will consume. Before wiring quell or defy, read the
// resource-cost note on characters.GetDefenseStaminaCost -- charging them through
// the stamina path would make them FREE.
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
