# internal/actionspec

## Purpose

`internal/actionspec` is the single registry of every action the game prices:
for each `Action`, which skill governs it, how that skill is resolved, and
whether encumbrance applies. It moved out of `internal/costs` in U9 to make it
a leaf that anything can read without dragging in the cost calculator.

**It does NOT serve the progression layer**, despite U9 having intended it to.
The progression seam derives its skill and stat names from CONTEXT -- the
weapon that threw the swing, the defence that registered, the spell record
being cast -- and none of those callers has an `Action` in hand to look up. A
`Stat` override field was added here and removed again in the same slice,
unused. If a later slice routes crafting and the other non-contest sites
through the registry, add it back together with its consumer.

It deliberately does NOT compute a cost, roll a contest, or charge a pool.
This package only answers "what does this action name mean", full stop. Cost
composition lives in `internal/costs.Calc`; progression application lives in
`internal/characters.ApplyProgression`.

`internal/costs` re-exports every symbol here as type aliases and constants
so no pre-U9 cost call site needed to change; new code should import this
package directly.

## Files

- `action.go` -- `Action`, `SkillSource`, `Spec`, the registry, `SpecFor`.
- `action_test.go` -- table tests, including
  `TestEveryRegisteredActionSkillHasAPrimaryStat`.

## Core types

```go
type Action string

const (
	ActionAttack Action = `attack`
	ActionDodge  Action = `dodge`
	ActionParry  Action = `parry`
	ActionBlock  Action = `block`
	ActionMove   Action = `move`
	ActionFlee   Action = `flee`

	// Paid in CONVICTION, both registered Physical: false.
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
// action.
type SkillSource uint8

const (
	SkillNone           SkillSource = iota
	SkillFixed                      // uses Spec.Skill directly
	SkillEquippedCombat             // actor's current weapon decides the skill
)

// Spec is what the registry knows about one action.
type Spec struct {
	Skill       skills.SkillTag // governing skill for SkillFixed actions
	SkillSource SkillSource
	Physical    bool // physical actions take the encumbrance multiplier
}
```

## Public API

```go
// SpecFor returns the registry entry for an action. An UNREGISTERED action
// returns the zero Spec -- no skill, not physical -- rather than panicking.
func SpecFor(a Action) Spec
```

## Gotchas

- **An unregistered `Action` returns the zero `Spec`, it does not panic.**
  That is the deliberate failure mode: a forgotten registry row shows up as a
  flat base cost with no skill discount and no encumbrance, which is
  findable, rather than a crash mid combat round.
- **An empty skill or stat name is not inert anywhere in this codebase.**
  `CheckSkillProgression("")` and `CheckStatProgression("")` still take the
  roll, and a success sends a levelup banner naming nothing at all. Any caller
  deriving a name from this registry must check for empty and skip the roll
  rather than passing the empty string on.
- **`SkillEquippedCombat` actions have no fixed `Spec.Skill`.** `ActionAttack`
  is the only one registered this way; its skill comes from the actor's
  current weapon at call time (`characters.GetCombatSkillTag`), not from this
  table. A caller that needs an equipped-combat action's stat must resolve
  the skill from the actor first and then look up its primary stat; the
  registry row cannot answer it.
- **`Physical: false` on `ActionQuell` / `ActionDefy` is load-bearing.**
  Flipping either row multiplies a backpack's weight into the price of a
  caster's saving throw or a social defence. See
  `internal/costs/context.md` for the consumer-side consequence.
- **The registry is package-level and read-only after init.** Nothing
  mutates `registry` at runtime; `SpecFor` is a pure map lookup.

## Dependencies

- `internal/skills` -- `SkillTag`, `GetSkillPrimaryStat`. Leaf-safe: does not
  reach `internal/characters` or `internal/costs`.

## Consumers

- `internal/costs` (`action.go`) -- re-exports every type, constant and
  `SpecFor` as aliases so pre-U9 call sites are unchanged.
- `internal/characters` -- registered-action cost quoting reads `SpecFor` via
  `internal/costs`.
