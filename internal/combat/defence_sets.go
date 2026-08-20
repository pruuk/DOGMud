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
// WIRED EVERYWHERE, through DefenceEntriesFor. Since U6b Task 2 this table is
// consumed only via DefenceEntriesFor, which intersects it with the defender's
// equipment gate: melee's runBestOfAllDefense set and ResolveChannelDefence's
// channel sets both come from that one builder. Adding a defence to a row here
// reaches every consumer of that channel, subject to the equipment gate below
// (dodge, quell and defy are ungated; parry and block are equipment-gated).
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

// DefenceEntryOpts carries the situational filters the entry builder applies.
type DefenceEntryOpts struct {
	// ThirdPartyVsGrappler mirrors the melee-only filterDefensesForThirdParty
	// behaviour (a bystander swinging into a grapple faces a reduced set).
	ThirdPartyVsGrappler bool
}

// equipmentGatedMeleeDefences reproduces, branch for branch, the equipment
// gate that lived in characters.GetDefenseSequence (deleted by U6b Task 2 —
// this is its only surviving copy). The branch ORDER is load-bearing:
//
//   - IsUnarmedStyle() first: bare hands and wielded Fist/Claws weapons get
//     dodge only — no parry even though "armed", and no block even with a
//     shield equipped. A shield-without-weapon defender also lands here
//     (no weapon => unarmed style) and gets dodge only.
//   - IsDualWielding() next: TWO parry entries (two blades, two chances) and
//     no block — this branch returns before the shield check.
//   - weapon + HasShield(): parry and block. HasShield() includes species
//     NaturalBash, so an armed earth elemental blocks with no shield item;
//     do NOT tighten this to BestBlockRating() > 0.
//   - weapon alone: parry.
func equipmentGatedMeleeDefences(c *characters.Character) []string {
	defenses := []string{characters.DefenseDodge}

	if c.IsUnarmedStyle() {
		return defenses
	}

	if c.IsDualWielding() {
		return append(defenses, characters.DefenseParry, characters.DefenseParry)
	}

	if c.Equipment.Weapon.ItemId > 0 && c.HasShield() {
		return append(defenses, characters.DefenseParry, characters.DefenseBlock)
	}

	if c.Equipment.Weapon.ItemId > 0 {
		return append(defenses, characters.DefenseParry)
	}

	return defenses
}

// thirdPartyGrappleDefences is the set-reduction half of the third-party
// grapple rule: an entangled defender attacked by a bystander keeps only
// block. filterDefensesForThirdParty (melee) layers the vulnerability
// messaging on top of this same rule.
func thirdPartyGrappleDefences(defSeq []string) []string {
	filtered := []string{}
	for _, def := range defSeq {
		if def == characters.DefenseBlock {
			filtered = append(filtered, def)
		}
	}
	return filtered
}

// DefenceEntriesFor is THE defence-set NAME builder for every channel, melee
// included. It intersects DefenceSetFor's channel table with the equipment
// gate copied verbatim from characters.GetDefenseSequence (which U6b Task 2
// deleted):
//
//   - parry: wielded weapon AND !IsUnarmedStyle() — knuckle/claw fighters
//     never parry; appears TWICE when dual-wielding (two blades, two chances)
//   - block: wielded weapon AND HasShield() — which includes species
//     NaturalBash, so an earth elemental blocks with no shield item; do NOT
//     gate on BestBlockRating()
//   - dodge, quell and defy: always available on their channels
//
// It returns NAMES ONLY. Scoring stays with the consumer: melee's candidate
// loop keeps its situational penalties/quoting/bookkeeping; the channel seam
// keeps GetDefenseScoreFor x defenceEffectiveness, and gains the prone
// penalties there (before U6b a prone defender dodged a bolt at full score
// while dodging a sword at penalty).
func DefenceEntriesFor(channel AttackChannel, defender *characters.Character, opts DefenceEntryOpts) []string {
	if defender == nil {
		return nil
	}

	gated := equipmentGatedMeleeDefences(defender)

	entries := []string{}
	for _, name := range DefenceSetFor(channel) {
		switch name {
		case characters.DefenseDodge, characters.DefenseParry, characters.DefenseBlock:
			// Physical defences: keep the gated multiplicity (dual-wield
			// contributes two parry entries).
			for i := 0; i < countDefenceName(gated, name); i++ {
				entries = append(entries, name)
			}
		default:
			// quell and defy are not equipment-gated.
			entries = append(entries, name)
		}
	}

	if opts.ThirdPartyVsGrappler {
		entries = thirdPartyGrappleDefences(entries)
	}

	return entries
}

func countDefenceName(set []string, name string) int {
	n := 0
	for _, s := range set {
		if s == name {
			n++
		}
	}
	return n
}
