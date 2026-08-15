# internal/costs

## Purpose

Prices every action in the game through one formula:
`cost = base x encumbrance(actor) x skill(actor) x modifier(action)`. A registry
table says, per action, which skill governs it and whether encumbrance applies;
`Calc` composes the multipliers and clamps their product.

It deliberately does **not** charge anything. Nothing here reads or writes a
`Character`, touches a resource pool, or decides whether an actor can afford
what it just priced. Deduction is `characters.ApplyCostFloat` (which banks the
sub-integer remainder); refusal policy lives at the call site. This package
answers "what does this cost", full stop, which is what lets it stay a
config-only leaf.

## Files

- `cost.go` — `Input`, `Calc`. The composition and the product clamp.
- `action.go` — `Action`, `Spec`, the registry, `SpecFor`.
- `skill.go` — `SkillCostMultiplier`, the inverse-skill curve.
- `encumbrance.go` — `EncumbranceMultiplier`, the carried-weight curve.
- `cost_test.go`, `skill_test.go`, `encumbrance_test.go` — table tests. The
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
)

type Spec struct {
	Skill    skills.SkillTag // governing skill; meaningless unless HasSkill
	HasSkill bool            // false for actions with no associated skill
	Physical bool            // physical actions take the encumbrance multiplier
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
	Base:      float64(bal.DodgeBaseStaminaCost),
	Carried:   carried,
	Capacity:  capacity,
	Physical:  spec.Physical,
	SkillRank: rank,          // actor's rank in spec.Skill
	HasSkill:  spec.HasSkill,
	Modifier:  1.25, // per-action knob; Task 6 adds the config field for it
})
```

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

None yet as of this task — the package is built ahead of its call sites. The
U7 arc wires it in from `internal/combat` (per-swing attack, the five defence
costs) and from movement. The registry is the seam: ranged, taunt, rally,
warcry, the thirteen currently-free special moves, grapple initiation and sneak
each become a registry entry plus a config base, with no change at their call
sites.
