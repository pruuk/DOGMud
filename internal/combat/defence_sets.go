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
// equipment gate: melee's runBestOfAllDefense set and ResolveChannelAttack's
// channel sets both come from that one builder. Adding a defence to a row here
// reaches every consumer of that channel, subject to the equipment gate below
// (dodge, quell and defy are ungated; parry and block are equipment-gated).
//
// THREE things a new row must carry with it. It needs an arm in
// characters.GetDefenseScore, or it enters every contest at 0 and always loses
// (TestDefenceSetForReturnsKnownDefenceNames is the guard). It needs a row
// in characters.DefensePool if it is not paid in stamina, or the pair charges
// the wrong pool.
//
// And it needs a row in DefenceSkillAndStat, which is the one whose absence
// fails SILENTLY and WIDELY. Without it that defence maps to ("", ""), so
// hooks.bestSwingDefence builds an award-nothing Candidate for it -- and if
// that candidate happens to roll highest, progression.BestOf reports false and
// the defender's ENTIRE ROUND trains nothing, the real dodge and parry
// candidates in the same slice included. Not a compile error, not a panic, and
// invisible in combat text. Unreachable today only because every shipped row
// here has a mapping.
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
// this is its only surviving copy).
//
// ⚠️ PER-SLOT, NOT A MAIN-HAND LADDER. Until 2026-08-30 this was a four-branch
// ladder and every branch read Equipment.Weapon or Offhand, so an extra arm
// could not contribute a defence at all:
//
//   - IsUnarmedStyle() ran FIRST and reads the MAIN HAND ONLY, so claws in hand
//     one suppressed parry AND block across all six arms.
//   - IsDualWielding() reads Weapon+Offhand only and returned EARLY, so two
//     weapons hid a shield in arm three.
//   - the parry count was hardcoded at two, so arms three through six could
//     never add one.
//   - HasShield() was the only part that scanned every arm, and exactly one
//     branch could reach it.
//
// A player with the extra-arms mutation and a tower shield on their third arm
// was getting dodge and nothing else.
//
// The rule is now derived from what each arm actually HOLDS: one parry entry
// per parry-capable armed hand, plus block if ANY arm holds a shield. That
// reproduces every two-handed case the ladder produced — no weapon to dodge,
// weapon to parry, two weapons to parry+parry, weapon+shield to parry+block —
// and lets the extra-arms mutation do what it visibly promises.
//
// HasShield() still includes species NaturalBash, so an earth elemental blocks
// with no shield item; do NOT tighten it to BestBlockRating() > 0.
//
// ⚠️ INTENDED BEHAVIOUR CHANGE: an unarmed or claw fighter HOLDING A SHIELD now
// blocks, where the ladder gave them dodge alone. Two EMPTY hands are unchanged
// (no weapon, no shield, so dodge only), which is the build
// internal/skills/skills.go solves WeaponCombat 1.34 against.
func equipmentGatedMeleeDefences(c *characters.Character) []string {
	defenses := []string{characters.DefenseDodge}

	for i := 0; i < c.ParryCapableArmCount(); i++ {
		defenses = append(defenses, characters.DefenseParry)
	}

	if c.HasShield() {
		defenses = append(defenses, characters.DefenseBlock)
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
