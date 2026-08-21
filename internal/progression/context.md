# internal/progression

## Purpose

`internal/progression` is the pure event layer of the unified contest arc
(U9): given the plain facts of one resolved contest, it says which
progression events that contest implies, for whom, at what multiplier.

It deliberately does NOT fire anything. It holds no `*characters.Character`,
no room, and reads no config; multipliers arrive as arguments (`Bonuses`).
That is what makes the matrix table-testable with plain values, and it
matters because a Go test binary never loads `_datafiles/config.yaml` -- a
package that read balance config here would be tested against Go defaults
instead of shipped values.

It also does NOT decide WHEN progression fires. Callers populate an `Outcome`
only for the sides that earned an ordinary event under that call site's
existing rules, and only set `Exceptional`/`Floored` when their own crit,
fumble and floor signals say so. Adding or removing a firing condition is a
caller-side decision, not this package's.

`characters.ApplyProgression` (`internal/characters/progression.go`) is the
single place `[]Event` gets applied to a real character. Callers in
`internal/combat` and `internal/hooks` are guarded against firing
progression any other way by
`internal/progression/seam_guard_test.go`, an AST test modelled on the U5b
pool-mutation guard.

## Files

- `event.go` -- `Side`, `Class`, `Event`, `Outcome`, `Exceptional`,
  `Classify`, `Bonuses`, `OrdinaryEvents`, `BonusEvents`, `EventsForContest`.
- `event_test.go` -- table tests over the matrix.
- `seam_guard_test.go` -- AST guard: fails if `internal/combat` or
  `internal/hooks` (outside an explicit allow-list) calls a progression
  primitive (`OnSkillUse`, `CheckStatProgression`, `OnCritReceived`, ...)
  directly instead of going through `characters.ApplyProgression`.

## Core types

```go
type Side uint8

const (
	SideAttacker Side = iota
	SideDefender
)

type Class uint8

const (
	ClassOrdinary Class = iota // tracks the use counter, progresses normally
	ClassCrit                  // the party who landed the crit
	ClassFumble                // the party who fumbled
	ClassObserved              // the party who received/witnessed the event
)

func (c Class) IsBonus() bool // true for everything except ClassOrdinary

type Event struct {
	Side       Side
	Skill      string
	Stat       string
	Class      Class
	Multiplier float64
}

type Outcome struct {
	AttackerSkill string
	AttackerStat  string
	DefenderSkill string
	DefenderStat  string

	// ToughenStat: the stat a defender trains for RECEIVING a crit
	// (vitality/willpower/charisma). Falls back to DefenderStat when empty.
	ToughenStat string

	// Exceptional: the ONE exceptional result this contest produced.
	Exceptional Exceptional

	// Floored: a contest floor changed the outcome. Floored contests award
	// ordinary events but never bonuses.
	Floored bool
}

type Exceptional uint8

const (
	ExcNone Exceptional = iota
	ExcAttackCrit
	ExcAttackFumble
	ExcDefenceCrit
	ExcDefenceFumble
)

type Bonuses struct {
	Doing     float64 // CritProgressionBonus
	Observing float64 // ObservedCritProgressionBonus
}
```

## Public API

Classifying a contest's crit/fumble signals into one row:

```go
func Classify(attackCrit, defenceCrit, attackFumble, defenceFumble bool) Exceptional
```

Turning an `Outcome` into events:

```go
// One event per side whose Skill or Stat is populated. No bonus events.
func OrdinaryEvents(o Outcome) []Event

// The crit/fumble tier only: the pair of events the ONE exceptional result
// implies, or nil if Floored or ExcNone.
func BonusEvents(o Outcome, b Bonuses) []Event

// OrdinaryEvents followed by BonusEvents, for callers that want both.
func EventsForContest(o Outcome, b Bonuses) []Event
```

## Gotchas

- **Bonus events must not track a use count, EXCEPT `ClassObserved`.** The use
  count becomes a virtual rank in `characters.CheckSkillProgression` /
  `CheckStatProgression`, and `CalculateProgressionChance` is monotonically
  DECREASING in rank -- so tracking a crit would punish critting. The
  `ClassObserved` party has no achievement to punish, and for them tracking
  the toughening stat is the ONLY thing that moves some stats' rank at all
  (nothing else calls `OnStatUse("vitality", ...)` in production). This split
  is enforced in `characters.applyBonusProgression`, not in this package --
  `progression` only labels the class; the caller decides what tracking a
  class implies.
- **An empty `Skill` or `Stat` means no roll of that kind.** Never pass an
  empty name downstream: `CheckSkillProgression("")` still takes the roll and
  a success banners no skill at all.
- **This package decides what an event CARRIES, never WHEN it fires.**
  Callers own firing conditions (whether a contest happened, whether it was
  floored, which side crit). Do not add a "should this fire" check here.
- **`Bonuses` are arguments, not config reads.** A Go test binary never loads
  `_datafiles/config.yaml`; a config-reading core here would be tested
  against zero values instead of the shipped `CritProgressionBonus` /
  `ObservedCritProgressionBonus`.
- **`Outcome.Exceptional` is one enum, not four booleans.** Four independent
  flags would let a caller pay four bonus events for one contest, or pay a
  fumble bonus to the side that won. `Classify` documents the precedence
  (crits cannot collide by construction since crit is margin-derived; a
  fumble CAN collide with a crit since fumble is self-relative, and the crit
  wins because it is the outcome the game narrated).
- **`OrdinaryEvents` and `BonusEvents` are separate on purpose.** Melee awards
  its defender's ordinary event once per round through
  `combat.AwardDefenceProgression`, but evaluates the bonus tier from the
  same `Outcome`; calling `EventsForContest` there would award the defender a
  second ordinary event per weapon hit. Making the caller filter a combined
  slice is how that bug gets written, so the package does the split instead.
- **A floored `Outcome` never pays a bonus**, even if `Exceptional` is set. A
  floor overrode the dice; an exceptional event that did not really happen
  teaches nobody.

## Dependencies

None (`event.go` imports nothing from this codebase). This is a leaf package
by design.

## Consumers

- `internal/characters` (`progression.go`) -- `ApplyProgression` is the sole
  applier of `[]Event`; `ToughenStatFor` supplies the toughening-stat mapping
  callers put into `Outcome.ToughenStat`.
- `internal/combat` (`defence_multiplier.go`) -- `DefenceSkillAndStat` and
  `AwardDefenceProgression` fill `Outcome` for the five defences;
  `awardChannelDefenceBonus` classifies the channel contest and calls
  `BonusEvents`.
- `internal/hooks` (`NewRound_DoCombat_unified.go`,
  `NewRound_DoCombat_helpers.go`, `spell_resolution.go`) -- melee's unarmed
  fallback, the quadrant-flavoured stat-gain emitter, and the player/mob
  magical-crit branches build an `Outcome` and call `ApplyProgression`.
- `internal/actions` (`combat_taunt.go`) -- the conviction/social crit site,
  routed the same way as the spell sites for consistency (not covered by the
  AST guard, which only walks `internal/combat` and `internal/hooks`).
