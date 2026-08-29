# internal/targeting

## Purpose

The single seam for choosing and committing combat targets, for players and
mobs alike. Two verbs, deliberately separate: `Select` answers "who should I
fight?" and has no combat consequence; `Commit` enters combat with a selected
target. The codebase discovered that distinction three times independently
(behaviortree's `SoftTarget`, `StageMeleeTarget`'s deferred engagement, and the
taunt hold) before it had a name for it.

It deliberately does **not** own target storage. In U12a `Commit`, `CommitAfter`,
`Release` and `CommitTaunt` all delegate to `characters.SetAggro` /
`EndAggro`, so the `Aggro`/`CombatPhase` dual-write and every guard stay exactly
where they were and nothing observable changes. It also does not decide *when*
a mob wants to fight; that stays with the behaviour trees.

Shipped as slice U12a of the unified-targeting arc. U12b sweeps the remaining
~86 write sites onto this package; U12c collapses the two stores and deletes
`characters.Aggro`.

## Files

- `doc.go` — package doc, and the two layering rules in prose.
- `commit.go` — `Reason`, `Commit`, `CommitAfter`, `Release`, `CommitTaunt`,
  `ReasonForAggroType`, and the single private `aggroTypeFor` translation.
- `engagement.go` — `Engagement`, `EngagementOf`, `IsAimedAt`,
  `ConsumeOpeningStrike`.
- `select.go` — `Kind`, `Criteria`, `Scope`, `Select` and the two strategies.
- `score.go` — the injected power-score function and `SetPowerScoreFn`.
- `imports_guard_test.go` — fails if this package ever depends on
  `internal/combat`.

## Core types

```go
type Reason int // ReasonAttack, ReasonSurprise, ReasonRetaliate, ReasonTaunt, ReasonDisengage

type Engagement struct {
	Phase          combatphase.State // STORED
	Target         state.ActorRef    // STORED
	OpeningUnspent bool              // STORED
	Casting        bool              // DERIVED from the activity machine
	Ranged         bool              // DERIVED from the equipped weapon subtype
	SpellTargets   []state.ActorRef  // STORED, and NOT in Target
}

type Kind int // RandomPlayer, WeakestHatedMob

type Criteria struct {
	Kind       Kind
	RatioBelow float64 // WeakestHatedMob only; 0 means 1.0
}

type Scope struct {
	Room              *rooms.Room
	Self              *characters.Character
	SelfMobInstanceId int // 0 for players
}
```

## Public API

Selection, no combat consequence:

```go
func Select(c Criteria, s Scope) (state.ActorRef, bool)
func SetPowerScoreFn(fn func(c characters.Character) float64)
```

Commitment:

```go
func Commit(c *characters.Character, ref state.ActorRef, r Reason)
func CommitAfter(c *characters.Character, ref state.ActorRef, r Reason, waitRounds int)
func CommitTaunt(c *characters.Character, ref state.ActorRef, holdRounds int)
func Release(c *characters.Character, r Reason)
func ReasonForAggroType(t characters.AggroType) Reason
```

Query:

```go
func EngagementOf(c *characters.Character) Engagement
func (e Engagement) IsAimedAt(ref state.ActorRef) bool
func ConsumeOpeningStrike(c *characters.Character) bool
```

## Gotchas

- **`EngagementOf` is pure; `ConsumeOpeningStrike` is the only side effect,
  and it should have exactly one caller.** Today `calculateCombat`
  (`internal/combat/combat.go:406-410`) reads `Aggro.Type` and demotes it in the
  same breath, which is why U10d had to add `AttackResult.WasSurpriseAttack` to
  carry the fact past the read. If the query ever consumes, every caller asking
  "is this an ambush?" spends the ambush. `TestEngagementOf_IsPure` guards this.

- **`Engagement.Target` is ZERO for a spell cast. Use `IsAimedAt`, not
  `Target`.** `characters.SetCast` (`internal/characters/spells.go:208`) builds
  `Aggro{Type: SpellCast, SpellInfo: ...}` and never writes `UserId` or
  `MobInstanceId`; the real targets live in `SpellInfo`, which is why
  `characters.IsAggro` has a dedicated `SpellCast` branch. Reading only `Target`
  reports "no target" for a mob mid-cast that `IsAggro` reports as aggro'd. An
  adversarial review caught exactly this in the first draft.

- **This package must never import `internal/combat`.** `internal/combat` is
  itself a `Commit` call site (`combat.go:409`, migrated in U12b), so the import
  would be a cycle. The power score is injected; `internal/combat` registers
  itself in an `init()` (`internal/combat/targeting_wiring.go`) rather than
  `main.go`, because `main.go` is not linked into any test binary and a
  main-only registration left every test exercising predation silently failing
  closed. `TestTargetingDoesNotImportCombat` enforces the direction.

- **`Select` fails closed.** No registered score function, no room, an unknown
  `Kind`, or nothing matching all return `ok=false`. Callers must treat that as
  a normal outcome. Picking arbitrarily would silently change which mob gets
  eaten.

- **`Criteria.RatioBelow` is used RAW and is not defaulted here.** The
  behaviour tree resolves the default itself with
  `getFloatParam(params, "ratio_below", 1.0)`. Defaulting a second time in this
  package would flip an authored `ratio_below: 0` from "predation disabled"
  (nothing is strictly below zero) to "engage anyone weaker". A Go caller
  constructing `Criteria{Kind: WeakestHatedMob}` without setting it therefore
  selects nobody, which is the safe direction and matches the code this
  replaced.

- **`internal/behaviortree`'s predation tests depend on a sibling file's
  import.** `actions_combat.go` no longer imports `internal/combat`, so the
  `init()` that registers the score function only runs because *other* files in
  that package (`actions_mob.go`, `conditions_combat.go`) still import it. This
  fails loudly rather than silently — the weakest-mob tests flip to `Failure` —
  but it is easy to break unknowingly while refactoring those files. If it
  breaks, register explicitly in the test rather than re-adding a `combat`
  import to `actions_combat.go`.

- **`CommitTaunt` sets the hold BEFORE committing.** Reversing those two lines
  makes every taunt silently no-op against an existing hold, because the
  taunt-hold gate would still be pointing at the previous taunter. Relatedly,
  `ReasonTaunt` maps to `characters.DefaultAttack` in `aggroTypeFor`: the gate
  pins only `DefaultAttack`/`Shooting`/`SurpriseAttack`, so a taunt committing
  as anything else could not hold its own target.

- **`Commit` refuses a zero `ActorRef`.** Committing to nobody would set an
  engagement with no target that every downstream consumer then has to defend
  against.

- **`internal/characters` can never import this package**, because this package
  imports it. That constrains where targeting *logic* may live; it is not a
  licence for `characters` to keep committing. `ForceTauntAggro` moved here as
  `CommitTaunt` and `characters` kept only the lock state, so the seam has **no
  exemptions**.

- **`Reason` is a fact about a moment, not about a state.** It says why a
  commit happened. What kind of engagement resulted is `Engagement`'s job.
  Conflating them is how `Aggro.Type` ended up being demoted mid-round.

- **Callers must not name `characters.SurpriseAttack` themselves.** Pass
  `actions.EngageAggroType`'s verdict through `ReasonForAggroType`. The ambush
  parity guard bans the direct reference, because deriving the surprise type
  locally skips the shared special-move cooldown the other engagement paths pay.

- **`user.Character` is already a `*Character`; `mob.Character` is a value.**
  Player call sites pass `user.Character`, mob call sites pass
  `&mob.Character`.

## Dependencies

`internal/characters`, `internal/items`, `internal/mobs`, `internal/rooms`,
`internal/state`, `internal/state/combatphase`, `internal/util`.

Not `internal/combat`, by rule and by test.

## Consumers

`internal/behaviortree` (`actAttack`, `target_random_player_in_room`,
`target_weakest_mob_in_room`), `internal/actions` (`StageMeleeTarget`,
`combat_taunt.go`), `internal/hooks` (`pinnacle_tick.go`), and
`internal/combat` for the `init()` registration only.
