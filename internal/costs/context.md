# internal/costs

## Purpose

Prices every action in the game through one formula:
`cost = base x encumbrance(actor) x skill(actor) x modifier(action)`. A registry
table says, per action, which skill governs it and whether encumbrance applies;
`Calc` composes the multipliers and clamps their product.

It deliberately does **not** charge anything. Nothing here reads or writes a
`Character` or touches a resource pool. `characters.QuoteActionCost` resolves
the registry metadata into calculator input, and `characters.CommitCost` owns
affordability, fractional carry, and full-versus-partial policy. This package
answers "what does this cost", full stop, which is what lets it stay a
config-only leaf.

## Files

- `cost.go` — `Input`, `Calc`. The composition and the product clamp.
- `action.go` — `Action`, `Spec`, the registry, `SpecFor`.
- `skill.go` — `SkillCostMultiplier`, the inverse-skill curve.
- `encumbrance.go` — `EncumbranceMultiplier`, the carried-weight curve.
- `action_test.go`, `cost_test.go`, `skill_test.go`, `encumbrance_test.go` —
  table tests. `action_test.go` pins the complete action matrix. The
  bench-figure case in `cost_test.go` pins three real characters' swing costs so
  the tuning bench and the code cannot drift apart silently.

## Core types

```go
// cost.go
type Input struct {
	Base      float64 // flat config base for the action
	Carried   float64 // actor's carried weight
	Capacity  float64 // actor's carry capacity
	Physical  bool    // physical actions take the encumbrance multiplier
	SkillRank int     // rank in the action's governing skill
	HasSkill  bool    // false for actions with no associated skill
	Modifier  float64 // per-action tuning knob
}

// action.go
type Action string

const (
	ActionAttack Action = `attack`
	ActionDodge  Action = `dodge`
	ActionParry  Action = `parry`
	ActionBlock  Action = `block`
	ActionMove   Action = `move`
	ActionFlee   Action = `flee`

	// Paid in CONVICTION, and both registered Physical: false.
	ActionQuell Action = `quell`
	ActionDefy  Action = `defy`
	// U8 also registers shoot, reload, every special move, grapple initiation
	// and maintenance, throw, sneak, taunt, rally, and warcry.
)

type SkillSource uint8

const (
	SkillNone SkillSource = iota
	SkillFixed
	SkillEquippedCombat
)

type Spec struct {
	Skill       skills.SkillTag // governing skill for SkillFixed actions
	SkillSource SkillSource
	Physical    bool // physical actions take the encumbrance multiplier
}
```

## Public API

Composing a cost:

```go
func Calc(in Input) float64
```

Looking up what an action is:

```go
func SpecFor(a Action) Spec
```

The individual curves, if you need one on its own:

```go
func SkillCostMultiplier(rank int) float64
func EncumbranceMultiplier(carried, capacity float64) float64
```

Typical call shape — the caller assembles `Input` from the actor plus the
registry, and hands the result to the pool:

```go
spec := costs.SpecFor(costs.ActionDodge)
raw := costs.Calc(costs.Input{
	Base:      float64(bal.DefenceBaseStaminaCost), // ONE shared base for all three physical defences
	Carried:   carried,
	Capacity:  capacity,
	Physical:  spec.Physical,
	SkillRank: rank, // actor's rank in spec.Skill
	HasSkill:  spec.SkillSource != costs.SkillNone,
	Modifier:  float64(bal.DodgeCostModifier), // 1.25; parry 1.10, block 1.15
})
```

There are no per-defence base knobs. `DodgeBaseStaminaCost`,
`ParryBaseStaminaCost` and `BlockBaseStaminaCost` were deleted in U7 Task 6
along with `characters.GetDefenseStaminaCost`; the three defences share
`DefenceBaseStaminaCost` and differ only by their `*CostModifier`. Quell and
defy use their own bases (`QuellBaseConvictionCost` / `DefyBaseConvictionCost`)
with a neutral `Modifier: 1.0`.

`Modifier` is skipped when it is `<= 0`, so a caller that leaves it at the zero
value gets a neutral 1.0 rather than a free action.

## Gotchas

- **`SkillCostMultiplier` runs OPPOSITE to `combat.SkillMultiplier`, and the
  name difference is load-bearing.** `combat.SkillMultiplier(rank int) float64`
  already exists with an identical signature and scales DAMAGE UPWARD on a sqrt
  curve; this one scales COST DOWNWARD on two linear segments. At rank 100 they
  return 3.0 and 0.40. Several U7 call sites live inside `package combat`, where
  an unqualified `SkillMultiplier(...)` resolves to the combat one — a mix-up
  compiles clean, passes vet, and lands as a 7.5x error in the wrong direction.
  Do not rename this back to match, and never introduce a `costs.SkillMultiplier`.

- **The clamp is on the PRODUCT of the multipliers, not on each factor.**
  Clamping factors individually still lets encumbrance 5.0, a rank-0 skill
  penalty 1.10 and a defence premium 1.25 stack to 6.875x. A laden novice priced
  at nearly seven times base cannot pay for anything, and "cannot pay" does not
  read to the player as expensive — it reads as autofail-everything. One ceiling
  on the composed product (`CostTotalMultiplierMax`, default 6.0) is the only
  place that guarantee can live. `Input.Base` sits OUTSIDE the clamp: it is the
  authored price of the action, and capping it would flatten cheap actions
  against expensive ones.

- **Four production callers of `Calc`** (this was "one" through Task 6 and is no
  longer): `characters.GetDefenseCostFloat` (all five defences),
  `characters.GetMovementStaminaCost` (movement), `combat.attackCostPerSwing`
  (one attack swing) and `usercommands.Flee`. Retuning `CostTotalMultiplierMax`
  therefore moves defence, movement, attack and flee prices together. In
  practice it still binds only on defence: movement's worst case is
  `5.0 x 1.10 = 5.5` against the 6.0 ceiling, and attack's and flee's are the
  same, because none of the three carries a per-action premium above 1.0.

- **The clamp is rank-dependent, and only bites novices.** It caps the composed
  multiplier, so the rank the actor holds decides the load at which it starts
  applying. At rank 1 (which is what `characters.New()` seeds every skill to)
  dodge reaches 6.0 above roughly **95.6%** of carry capacity, block above
  **98.3%** and parry above **99.8%** — so at capacity all three converge on the
  same 6.0 and the per-defence modifiers stop being visible. At a realistic
  ~40% load none of them are close (1.735 / 1.597 / 1.527). At the skill cap the
  question does not arise at all: dodge's worst case is
  `5.0 x 0.40 x 1.25 = 2.5`, so **a mastered defender never reaches the clamp at
  any load.** The convergence at capacity is ACCEPTED — a defender hauling their
  own weight in ore paying a flat ceiling for any defence is the intended
  behaviour, not a defect to fix.

- **Leaf rule: never import `internal/characters`.** `characters` reaches this
  package (or will), so the reverse edge is a cycle. That is why `Input` carries
  plain `float64`s instead of an actor: callers extract carried weight, capacity
  and skill rank themselves. Importing `internal/skills` is fine — verified not
  to reach `characters`. Guard with
  `go list -deps ./internal/costs | grep GoMudEngine`.

- **`0` is not a usable value for any `Cost*` knob.** Every one of them is
  validated `<= 0 -> default` in `configs.validateProgression`, so a `0` in
  `config.yaml` is silently replaced, not honoured. This is the opposite of the
  usual project rule that `0` is a legal shipped value (`StaminaPerStrength: 0`).
  A zero here would price actions free or divide by zero, so it is treated as
  "absent". To make an action free, leave it out of the registry or give it a
  `Base` of 0 — not a zeroed multiplier knob.

- **`Physical: false` is the ONLY thing keeping encumbrance off quell and defy,
  and it is load-bearing.** Both are read for real now:
  `characters.GetDefenseCostFloat` passes `spec.Physical` straight into `Input`,
  so flipping either row multiplies a backpack into the price of a caster's
  saving throw. `TestQuellAndDefyPriceThroughTheirRegistryRows` in
  `internal/characters` fails on the flip. Note this was NOT true before U7:
  the rows existed but `GetDefenseCostFloat` returned early for both, so nothing
  read them and flipping one changed nothing.

- **A skill discount is not a physical-only privilege.** `SkillFixed` on the
  mental and social rows is deliberate: every action with a governing skill
  takes the inverse-skill multiplier. `Physical` governs encumbrance and
  `SkillSource` governs rank selection; they are independent. Attack alone uses
  `SkillEquippedCombat`; fixed rows read `Spec.Skill`.

- **An unregistered `Action` returns the zero `Spec`, it does not panic.** The
  result is a flat base cost with no skill discount and no encumbrance. That is
  the deliberate failure mode: a forgotten registry row shows up as a cost that
  never varies with skill or load, which is findable, rather than a crash mid
  combat round.

- **Every function reads `configs.GetBalanceConfig()` on each call**, so a
  config reload takes effect immediately and nothing here is safe to cache
  across a reload.

## Dependencies

- `internal/configs` — every curve and the clamp read `GetBalanceConfig()`.
- `internal/skills` — `SkillTag` only, for the registry.

Knobs owned: `CostSkillMultAtZero`, `CostSkillMultAtMid`, `CostSkillMultAtCap`,
`CostSkillMidRank`, `CostSkillCapRank`, `CostEncumbranceKnee`,
`CostEncumbranceKneeMult`, `CostEncumbranceMax`, `CostTotalMultiplierMax`.
Declared in `internal/configs/config.balance.go`, defaulted and validated in
`internal/configs/config.balance.progression.go`.

## Consumers

Three, as of U7 Task 10.

- **Defence** (Task 6): `characters.GetDefenseCostFloat` prices all five
  defences through `Calc`, and `internal/combat` charges the result via
  `Character.ApplyCostFloat` at both defence sites (`runBestOfAllDefense` for
  melee, `ResolveChannelDefence` for the ranged, spell and social channels).
- **Attack** (Task 7): `combat.attackCostPerSwing` prices ONE swing; the
  exported `combat.ChargeAttackCost(attacker, swings)` multiplies by the swings
  actually thrown and charges through `ApplyCostFloat`. The four wrappers in
  `combat/combat.go` call it after `calculateCombat` returns. Note it reads the
  rank off `GetCombatSkillLevel` (weapon-appropriate) rather than the registry's
  nominal `skills.WeaponCombat`.
- **Movement** (Task 8): `characters.GetMovementStaminaCost` folds the terrain
  multiplier into `Base` (terrain is a property of the move, not the actor, and
  `Base` is deliberately outside the clamp), then applies the mutation speed
  modifier, the hidden multiplier and `MovementMaxStaminaCost` after `Calc`
  returns. It returns a FLOAT and `go.go` charges it through
  `Character.ApplyCostFloatOrRefuse`, the all-or-nothing sibling of
  `ApplyCostFloat`: movement banks its remainder like every other priced action
  AND still refuses when unaffordable, which is the U5b-2 gate that makes flee
  the only player-initiated disengage in combat. It used to return an int and
  ceil every move, which flattened a 1.0-to-5.0 encumbrance range into three
  distinct prices; `MovementCostFloor` was deleted with the ceiling, because a
  banked sub-1 charge is not free and any floor at or above 1 re-flattens the
  curve.
- **Flee**: `usercommands.Flee` prices `FleeStaminaCost` as the `Base` through
  the `ActionFlee` row (physical, governed by `skills.Skullduggery`, which is
  the skill `combat.ResolveFleeBlockers` already rolls) and charges it with
  `ApplyCostFloat`. `ApplyCostFloat`, NOT `ApplyCostFloatOrRefuse`: flee must
  never refuse for cost, and `ApplyCostFloat` delegates to `ApplyCostPartial`.
  Flee and movement are deliberate mirror images here.

The registry now includes every U8 cost surface: ranged, taunt, rally, warcry,
the special-move family, grapple initiation and maintenance, throw, and sneak.
Later U8 tasks migrate their call sites onto the quote/commit seam.
