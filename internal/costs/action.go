package costs

import "github.com/GoMudEngine/GoMud/internal/skills"

// The action registry is the seam the rest of the U7 arc wires into. One table
// says, for every action the game prices, which skill governs it and whether it
// is physical; Calc reads nothing else about the action. That is deliberate:
// ranged attacks, taunt, rally, warcry, the thirteen special moves that are
// currently free, grapple initiation and sneak all become a registry entry here
// plus a config base, with NO new plumbing at their call sites. Without this
// table each of those would need its own bespoke cost expression inline, which
// is exactly the scattering U7 exists to undo.
//
// Adding an action is therefore two edits and no logic: a constant plus a
// registry row.

// Action names one priced action.
type Action string

const (
	ActionAttack Action = `attack`
	ActionDodge  Action = `dodge`
	ActionParry  Action = `parry`
	ActionBlock  Action = `block`
	ActionMove   Action = `move`
)

// Spec is what the registry knows about an action: which skill discounts it,
// and whether encumbrance applies.
type Spec struct {
	Skill    skills.SkillTag // governing skill; meaningless unless HasSkill
	HasSkill bool            // false for actions with no associated skill
	Physical bool            // physical actions take the encumbrance multiplier
}

// registry maps each priced action to its spec. Package-level and read-only
// after init; nothing mutates it at runtime.
var registry = map[Action]Spec{
	ActionAttack: {Skill: skills.WeaponCombat, HasSkill: true, Physical: true},
	ActionDodge:  {Skill: skills.UnarmedCombat, HasSkill: true, Physical: true},
	ActionParry:  {Skill: skills.WeaponCombat, HasSkill: true, Physical: true},
	ActionBlock:  {Skill: skills.WeaponCombat, HasSkill: true, Physical: true},
	ActionMove:   {Skill: skills.Search, HasSkill: true, Physical: true},
}

// SpecFor returns the registry entry for an action.
//
// An UNREGISTERED action returns the zero Spec — no skill, not physical —
// rather than panicking. That is a deliberate failure mode: a caller who adds a
// new action and forgets its registry row gets a flat base cost, which is
// wrong-but-playable, instead of a panic in the middle of a combat round. The
// mistake shows up as a cost that never varies with skill or load, which is
// findable; a crash mid-fight is not an acceptable price for a missing table
// row.
func SpecFor(a Action) Spec {
	return registry[a]
}
