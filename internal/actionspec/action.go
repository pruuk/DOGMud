package actionspec

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
	ActionFlee   Action = `flee`

	// The two non-physical defences. Both are paid in CONVICTION rather than
	// stamina, and both are registered Physical: false so they never take the
	// encumbrance multiplier -- a heavy pack does not make it harder to put down
	// a working or to refuse an insult.
	ActionQuell Action = `quell`
	ActionDefy  Action = `defy`

	ActionShoot           Action = `shoot`
	ActionReload          Action = `reload`
	ActionBash            Action = `bash`
	ActionTrip            Action = `trip`
	ActionKick            Action = `kick`
	ActionGrapple         Action = `grapple`
	ActionGrappleMaintain Action = `grapple-maintain`
	ActionHamstring       Action = `hamstring`
	ActionRake            Action = `rake`
	ActionMaul            Action = `maul`
	ActionPounce          Action = `pounce`
	ActionGore            Action = `gore`
	ActionDrain           Action = `drain`
	ActionThrottle        Action = `throttle`
	ActionThrow           Action = `throw`
	ActionSneak           Action = `sneak`
	ActionTaunt           Action = `taunt`
	ActionRally           Action = `rally`
	ActionWarcry          Action = `warcry`
)

// SkillSource says how a caller resolves the skill rank used to price an
// action. Fixed actions use Spec.Skill; equipped-combat actions ask the actor
// which combat skill their current weapon uses.
type SkillSource uint8

const (
	SkillNone SkillSource = iota
	SkillFixed
	SkillEquippedCombat
)

// Spec is what the registry knows about an action: which skill discounts it,
// and whether encumbrance applies.
type Spec struct {
	Skill       skills.SkillTag // governing skill for SkillFixed actions
	SkillSource SkillSource
	Physical    bool // physical actions take the encumbrance multiplier
}

// registry maps each priced action to its spec. Package-level and read-only
// after init; nothing mutates it at runtime.
var registry = map[Action]Spec{
	ActionAttack: {SkillSource: SkillEquippedCombat, Physical: true},
	ActionDodge:  {Skill: skills.UnarmedCombat, SkillSource: SkillFixed, Physical: true},
	ActionParry:  {Skill: skills.WeaponCombat, SkillSource: SkillFixed, Physical: true},
	ActionBlock:  {Skill: skills.WeaponCombat, SkillSource: SkillFixed, Physical: true},
	ActionMove:   {Skill: skills.Search, SkillSource: SkillFixed, Physical: true},

	// Flee is physical, and SKULLDUGGERY governs it. That is not an invented
	// pairing to fill the field: combat.ResolveFleeBlockers already rolls the
	// fleer's Dex + Skullduggery x 25 against each blocker, so skullduggery is
	// the skill the game itself says breaking off is made of, and the registry
	// only makes the cost agree with the roll. Encumbrance applies for the
	// obvious reason: a full pack is exactly what you cannot sprint away in,
	// and before this entry escaping with a third more than your capacity on
	// your back cost precisely what escaping empty-handed cost.
	ActionFlee: {Skill: skills.Skullduggery, SkillSource: SkillFixed, Physical: true},

	// Physical: false is load-bearing, not an oversight. Quell answers a mental
	// spell and defy answers a social attack; charging either an encumbrance
	// premium would price a caster's saving throw off their backpack.
	ActionQuell: {Skill: skills.Spellcasting, SkillSource: SkillFixed},
	ActionDefy:  {Skill: skills.Rhetoric, SkillSource: SkillFixed},

	ActionShoot:           {Skill: skills.RangedCombat, SkillSource: SkillFixed, Physical: true},
	ActionReload:          {Skill: skills.RangedCombat, SkillSource: SkillFixed, Physical: true},
	ActionBash:            {Skill: skills.WeaponCombat, SkillSource: SkillFixed, Physical: true},
	ActionTrip:            {Skill: skills.UnarmedCombat, SkillSource: SkillFixed, Physical: true},
	ActionKick:            {Skill: skills.UnarmedCombat, SkillSource: SkillFixed, Physical: true},
	ActionGrapple:         {Skill: skills.UnarmedCombat, SkillSource: SkillFixed, Physical: true},
	ActionGrappleMaintain: {Skill: skills.UnarmedCombat, SkillSource: SkillFixed, Physical: true},
	ActionHamstring:       {Skill: skills.UnarmedCombat, SkillSource: SkillFixed, Physical: true},
	ActionRake:            {Skill: skills.UnarmedCombat, SkillSource: SkillFixed, Physical: true},
	ActionMaul:            {Skill: skills.UnarmedCombat, SkillSource: SkillFixed, Physical: true},
	ActionPounce:          {Skill: skills.UnarmedCombat, SkillSource: SkillFixed, Physical: true},
	ActionGore:            {Skill: skills.UnarmedCombat, SkillSource: SkillFixed, Physical: true},
	ActionDrain:           {Skill: skills.UnarmedCombat, SkillSource: SkillFixed, Physical: true},
	ActionThrottle:        {Skill: skills.UnarmedCombat, SkillSource: SkillFixed, Physical: true},
	ActionThrow:           {Skill: skills.Skullduggery, SkillSource: SkillFixed, Physical: true},
	ActionSneak:           {Skill: skills.Skullduggery, SkillSource: SkillFixed, Physical: true},
	ActionTaunt:           {Skill: skills.Rhetoric, SkillSource: SkillFixed},
	ActionRally:           {Skill: skills.Rhetoric, SkillSource: SkillFixed},
	ActionWarcry:          {Skill: skills.Rhetoric, SkillSource: SkillFixed},
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
