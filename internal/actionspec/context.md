# internal/actionspec

## Purpose

`internal/actionspec` is the single registry of every action the game prices:
for each `Action`, which skill governs it, how that skill is resolved, and
whether encumbrance applies. It moved out of `internal/costs` in U9 so that
the cost calculator and the progression layer read ONE table -- both ask "which
skill/stat does this action exercise", and two tables answering that question
would drift the first time someone added an action to only one of them.

It deliberately does NOT compute a cost, roll a contest, or charge a pool.
This package only answers "what does this action name mean", full stop. Cost
composition lives in `internal/costs.Calc`; progression application lives in
`internal/characters.ApplyProgression`.

`internal/costs` re-exports every symbol here as type aliases and constants
so no pre-U9 cost call site needed to change; new code should import this
package directly.

## Files

- `action.go` -- `Action`, `SkillSource`, `Spec`, the registry, `SpecFor`,
  `StatFor`.
- `action_test.go` -- table tests, including `TestEveryRegisteredActionResolvesAStat`.

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

	// Stat OVERRIDES the stat this action exercises for progression. Empty
	// (every registered action today) means the skill's primary stat. Exists
	// for the two cases that diverge: a spell declaring its own primarystat,
	// and the toughening stat awarded for a crit RECEIVED (vitality /
	// willpower / charisma), which is deliberately not the stat that fed the
	// defence score.
	Stat string
}
```

## Public API

```go
// SpecFor returns the registry entry for an action. An UNREGISTERED action
// returns the zero Spec -- no skill, not physical -- rather than panicking.
func SpecFor(a Action) Spec

// StatFor returns the stat an action exercises: the Spec's Stat override if
// set, otherwise the governing skill's primary stat (skills.GetSkillPrimaryStat).
// Returns "" for a Spec with neither -- callers MUST treat that as "no stat
// roll", never pass it downstream.
func StatFor(s Spec) string
```

## Gotchas

- **An unregistered `Action` returns the zero `Spec`, it does not panic.**
  That is the deliberate failure mode: a forgotten registry row shows up as a
  flat base cost with no skill discount and no encumbrance, which is
  findable, rather than a crash mid combat round.
- **`StatFor` returning `""` is not inert -- it is a contract.**
  `CheckStatProgression("")` still takes the roll and a success sends a
  levelup banner naming no stat at all. Every caller must check for empty and
  skip the stat roll, never pass the empty string on.
- **`SkillEquippedCombat` actions have no fixed `Spec.Skill`.** `ActionAttack`
  is the only one registered this way; its skill comes from the actor's
  current weapon at call time (`characters.GetCombatSkillTag`), not from this
  table. `StatFor` does not special-case this -- callers resolving an
  equipped-combat action's stat must resolve the skill themselves first, then
  look up its primary stat, rather than calling `StatFor` on the registry
  `Spec` directly.
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
  `SpecFor` as aliases so pre-U9 call sites are unchanged. Does NOT re-export
  `StatFor`; a caller that needs it must import `internal/actionspec`
  directly.
- `internal/characters` -- registered-action cost quoting reads `SpecFor` via
  `internal/costs`.
- **`StatFor` has zero production callers as of U9.** It ships ahead of its
  consumer, the same way `spells.SpellData.PrimaryStat` did before this same
  arc made it required -- do not delete it for being unused; a later U-slice
  is expected to route action-based progression (the ~93 non-contest call
  sites the `internal/progression` seam guard names as deliberately
  untouched: craft, salvage, forage, search, steal, and the rest) through it.
  Today's contest paths resolve their trained skill/stat their own way:
  melee/defence through `combat.DefenceSkillAndStat`, spells through
  `spells.SpellData.PrimaryStat` and `CasterStatValue`.
