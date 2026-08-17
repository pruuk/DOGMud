# U8 Unified Action-Cost Admission Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Route every U8 combat action through one quote/commit admission contract, make voluntary actions refuse atomically while life-preserving actions resolve skill-less when short, and give quell and defy complete data-driven defence narration.

**Architecture:** Extend the U7 `internal/costs` registry with explicit fixed-skill versus equipped-combat-skill metadata. `internal/characters` owns a non-mutating, single-use cost quote and full/partial commit against the real pool and fractional carry. Action packages decide when an otherwise-valid action asks for admission; `combat.RunContest` remains pure resolution. Shared player/mob action functions own mechanical charging, while player wrappers alone render private refusal or shortage text.

**Tech Stack:** Go, YAML world/config data, Python balance model, Go table-driven tests, DOGMud playtest harness.

**Spec:** [`docs/superpowers/specs/2026-08-17-u8-unified-action-cost-admission-design.md`](../specs/2026-08-17-u8-unified-action-cost-admission-design.md)

## Global Constraints

- The approved spec is authoritative. Do not reopen D1-D11 while implementing.
- Use `combat.RunContest` for every opposed roll. Do not add a direct production caller to `contest.Run`, `contest.AgainstDifficulty`, or deprecated dice contest helpers.
- Cost is always `costs.Calc`: config base x encumbrance when physical x inverse governing skill x documented modifier, with the existing product clamp.
- Affordability reads the current pool, never `EffectivePoolMax`, and never subtracts reservation again.
- Full-payment refusal mutates no pool, fractional carry, cooldown, ammunition, item, round, awareness state, buff, grapple state, or effect.
- Partial payment writes off an unpaid whole charge; it never creates debt reclaimed by a later action.
- `Short` removes only the governing skill contribution from the named opposed score. It does not alter damage, swing count, resource multipliers, mitigation, crit/fumble rules, contest floors, or progression hooks.
- Shared action functions charge both players and mobs exactly once. Only player-facing wrappers emit private cost text.
- Mob `charge` remains an alias of trip and mob `howl` remains an alias of taunt. `ExecuteDrainArea` remains an uncharged boss ability. Surprise attack and mutation dead-code cleanup remain out of scope.
- Every balance number is selected by Task 1's model before it is added to Go defaults or shipped YAML. Do not guess a literal in `internal/`.
- Keep touched package `context.md` files, helpfiles, config comments, roadmap, and patch notes current in the same slice.
- Because this plan adds player-facing messages, the final gate is an adversarial in-game playtest, not merely tests and a clean boot.

---

## Shared names this plan uses

These names are fixed for the implementation so later tasks do not invent parallel seams.

```go
// internal/costs/action.go
type SkillSource uint8

const (
	SkillNone SkillSource = iota
	SkillFixed
	SkillEquippedCombat
)

type Spec struct {
	Skill       skills.SkillTag
	SkillSource SkillSource
	Physical    bool
}
```

```go
// internal/characters/pools.go
type CostPolicy uint8

const (
	CostFullOrRefuse CostPolicy = iota
	CostPartial
)

type CostStatus uint8

const (
	CostNoCharge CostStatus = iota
	CostPaid
	CostPartiallyPaid
	CostRefused
)

type ActionCostRequest struct {
	Action   costs.Action
	Pool     Pool
	Base     float64
	Modifier float64
	Units    int
}

type CostCommitResult struct {
	Status  CostStatus
	Pool    Pool
	Charged int
}

func (r CostCommitResult) Short() bool {
	return r.Status == CostPartiallyPaid
}
```

`CostQuote` is exported as a value that can be passed between packages, but it contains only a pointer to an unexported quote state. That state records the owner character, pool/carry snapshot, calculated amount, and a consumed bit. Callers may inspect only `Affordable()`; only `Character.CommitCost` may interpret the snapshot or mark it consumed. This makes a zero-whole quote single-use too, even though committing it changes no pool state.

The new shared config bases are intentionally few:

| Knob | Actions |
|---|---|
| `ShootBaseStaminaCost` | shoot |
| `ReloadBaseStaminaCost` | reload |
| `SpecialMoveBaseStaminaCost` | bash, trip, kick, grapple initiation, hamstring, rake, maul, pounce, gore, drain, throttle, throw |
| `SneakBaseStaminaCost` | sneak |
| `RhetoricActionBaseConvictionCost` | taunt, rally, warcry |
| `GrappleStaminaCostPerRound` | grapple maintenance, retaining the controller/controlled role multipliers |

Sharing the special-move and rhetoric bases is deliberate: these actions share cadence and should not acquire duplicate tuning knobs without evidence that their resource pressure must diverge.

---

### Task 1: Extend the balance model and select the shipped bases

**Files:**
- Modify: `tools/balance/unified_resolution_model.py`
- Create: `docs/superpowers/plans/2026-08-17-u8-cost-model-evidence.md`
- Test: `tools/balance/unified_resolution_model.py` (its built-in assertions and exit status)

- [ ] **Step 1: Add the failing U8 acceptance assertions**

Add named character/load fixtures for novice, mid-skill, veteran, and the live-character bands already represented by the model. Add empty, typical, knee, and capacity loads. Add an `ordinary_swing_cost(character, load, skill)` reference and fail until U8 candidate bases exist.

The model must assert all of these, not merely print them:

```python
assert special_move_cost > ordinary_swing_cost
assert special_move_cost <= ordinary_swing_cost * 4
assert reload_cost < shoot_cost
assert controlled_grapple_cost > controller_grapple_cost
assert all(cost <= product_clamp_bound for cost in laden_novice_costs)
```

Calculate the comparisons for the same character, load, and governing skill. Four swings means four individual swings, not four multi-swing rounds.

- [ ] **Step 2: Run the model and verify the new gate fails**

Run: `python tools/balance/unified_resolution_model.py`

Expected: non-zero exit identifying missing U8 bases or an unsatisfied U8 constraint; existing U7 tables must still render before the failure.

- [ ] **Step 3: Model action cadence and combined cycles**

Add tables for:

- shoot plus reload cycles across current ranged weapon capacities;
- cooldown-spaced special moves alongside ordinary autoattacks;
- taunt, rally, and warcry against current Conviction max, regen, and reservation bands;
- controller and controlled grapple timelines for at least ten rounds; and
- affordable, exhausted, and recovered transitions for every pool.

Enumerate candidate bases in this exact set:

```python
U8_CANDIDATES = (0.5, 0.75, 1.0, 1.25, 1.5, 2.0, 2.5, 3.0, 3.5, 4.0)
```

Select the simplest candidate satisfying every assertion. For grapple maintenance, model the existing `GrappleControllerCostMultiplier` and `GrappleControlledCostMultiplier` before `costs.Calc`; if the current base 5 fails the combined-pressure constraints, retune it from the same candidate set and record the behavior change explicitly.

- [ ] **Step 4: Record the evidence and chosen values**

Create `docs/superpowers/plans/2026-08-17-u8-cost-model-evidence.md` containing:

- the exact shipped value for all six knobs in the table above;
- per-fixture cost in ordinary-swing equivalents;
- ranged cycle and rhetoric recovery tables;
- ten-round grapple pool traces for both roles; and
- a short rationale for each selected value and every rejected boundary candidate.

No later task may use a value absent from this evidence file.

- [ ] **Step 5: Run and commit the model gate**

Run: `python tools/balance/unified_resolution_model.py`

Expected: exit 0, all U7 and U8 assertions pass, and the printed numbers match the evidence file.

Commit:

```bash
git add tools/balance/unified_resolution_model.py docs/superpowers/plans/2026-08-17-u8-cost-model-evidence.md
git commit -m "test: model U8 action cost pressure"
```

---

### Task 2: Build the action registry and quote/commit contract

**Files:**
- Modify: `internal/costs/action.go`
- Create: `internal/costs/action_test.go`
- Modify: `internal/characters/pools.go`
- Modify: `internal/characters/pools_test.go`
- Modify: `internal/characters/cooldowns.go`
- Create: `internal/characters/action_cost_test.go`
- Modify: `internal/configs/config.balance.go`
- Modify: `internal/configs/config.balance.combat.go`
- Modify: `_datafiles/config.yaml`
- Modify: `internal/costs/context.md`
- Modify: `internal/characters/context.md`
- Modify: `internal/configs/context.md`

- [ ] **Step 1: Write registry tests for the complete matrix**

Use one table covering all existing and new actions. Pin action name, physical flag, and skill source. Pin fixed skills for dodge/parry/block/quell/defy/flee and every new U8 action; pin `SkillEquippedCombat` for attack. The new constants are:

```go
ActionShoot, ActionReload, ActionBash, ActionTrip, ActionKick,
ActionGrapple, ActionGrappleMaintain, ActionHamstring, ActionRake,
ActionMaul, ActionPounce, ActionGore, ActionDrain, ActionThrottle,
ActionThrow, ActionSneak, ActionTaunt, ActionRally, ActionWarcry
```

Run: `go test ./internal/costs -run 'TestSpecFor' -v`

Expected: FAIL because the constants and `SkillSource` metadata do not exist.

- [ ] **Step 2: Replace `HasSkill` with explicit skill selection**

Implement `SkillSource`, update existing registry entries, add every U8 entry, and update `costs.Calc` callers to derive `HasSkill` from `spec.SkillSource != SkillNone`. Do not teach the pure `costs` package how to read a character.

- [ ] **Step 3: Write quote/commit tests before implementation**

Create `internal/characters/action_cost_test.go` with focused tests that prove:

- `QuoteActionCost` changes neither pool nor `costCarry`;
- fixed skill and equipped combat skill select the correct rank;
- physical quotes change with load and mental quotes do not;
- `Units: 4` is exactly four times the unrounded one-unit amount;
- full payment updates carry and pool exactly once;
- full refusal changes neither carry nor pool;
- partial payment charges only the available whole amount and reports `CostPartiallyPaid`;
- a whole due of zero reports `CostNoCharge`, not short;
- non-positive and non-finite base/modifier inputs bank nothing;
- a stale quote refuses without charging a different amount; and
- a quote committed by another character or committed twice refuses; and
- reservation is not subtracted from the current pool a second time.

Run: `go test ./internal/characters -run 'Test(ActionCost|QuoteActionCost|CommitCost)' -v`

Expected: FAIL because the quote types and methods do not exist.

- [ ] **Step 4: Implement the single-use quote**

Implement:

```go
func (c *Character) QuoteActionCost(req ActionCostRequest) CostQuote
func (q CostQuote) Affordable() bool
func (c *Character) CommitCost(q CostQuote, policy CostPolicy) CostCommitResult
```

`QuoteActionCost` resolves the registry spec, selects the fixed skill or `GetCombatSkillLevel`, calls `costs.Calc` once, multiplies its unrounded result by `Units`, and creates an unexported state containing `owner: c`, the current pool/carry snapshot, and `consumed: false`. `Units <= 0` yields a no-charge quote. `CommitCost` first rejects a nil state, a different owner, a consumed quote, or changed pool/carry values. It marks an accepted quote consumed before applying its policy. Full refusal leaves pool and carry untouched. Partial commit stores the fractional remainder, charges `min(wholeDue, currentPool)`, and discards the unpaid whole portion.

Keep `ApplyCostFloat` and `ApplyCostFloatOrRefuse` as compatibility wrappers until all old callers are intentionally migrated; implement them in terms of the same private fractional calculation so the two paths cannot drift.

- [ ] **Step 5: Add a read-only cooldown query**

Add:

```go
func (c *Character) CooldownReady(trackingTag string) bool {
	return c.Cooldowns == nil || c.Cooldowns[trackingTag] <= 0
}
```

Test that it neither initializes nor prunes the map. `TryCooldown` remains the consuming call and is used only after successful admission.

- [ ] **Step 6: Add the model-selected config bases**

Add the five new fields named in “Shared names this plan uses” as `ConfigFloat`, and convert the existing `GrappleStaminaCostPerRound` from `ConfigInt` to `ConfigFloat` so Task 1 may select a fractional candidate without a code literal or rounding seam. Validate positive finite defaults in `config.balance.combat.go`, and put the exact Task 1 values plus adjacent explanatory comments in `_datafiles/config.yaml`. Add a table test proving missing/invalid values default and shipped YAML decodes to the evidence values.

- [ ] **Step 7: Run and commit the foundation**

Run:

```bash
go test ./internal/costs ./internal/characters ./internal/configs
gofmt -w internal/costs/action.go internal/costs/action_test.go internal/characters/pools.go internal/characters/pools_test.go internal/characters/cooldowns.go internal/characters/action_cost_test.go internal/configs/config.balance.go internal/configs/config.balance.combat.go
```

Expected: all tests pass; `gofmt -l` on the named Go files prints nothing.

Commit:

```bash
git add internal/costs internal/characters internal/configs _datafiles/config.yaml
git commit -m "feat: add unified action cost admission"
```

---

### Task 3: Add shared action admission and refusal results

**Files:**
- Create: `internal/actions/action_cost.go`
- Create: `internal/actions/action_cost_test.go`
- Modify: `internal/actions/actor.go`
- Modify: `internal/actions/context.md`
- Modify: `internal/usercommands/context.md`
- Modify: `internal/mobcommands/context.md`

- [ ] **Step 1: Write the shared action helper tests**

Pin that `admitFullCost(actor, action, pool, base)` delegates to `QuoteActionCost` and then calls `CommitCost(quote, CostFullOrRefuse)`, charges player and mob actors identically, and never emits text itself.

Add these helpers:

```go
func admitFullCost(actor Actor, action costs.Action, pool characters.Pool, base float64) characters.CostCommitResult
func costRefusalText(result characters.CostCommitResult) string
```

`costRefusalText` returns a stamina-specific “too spent” sentence or a conviction-specific “cannot muster the resolve” sentence, without numbers. Shared result structs embed `Cost characters.CostCommitResult`; user wrappers render refusal when `Cost.Status == characters.CostRefused`, while mob wrappers remain silent.

- [ ] **Step 2: Prove wrapper parity and no double charge**

Add one player and one mob actor test using the same character state and base. Assert equal charge and exactly one carry mutation. Add an AST/table guard in `internal/actions/action_cost_test.go` listing the U8 shared action functions and rejecting direct calls to `ApplyCost`, `ApplyCostPartial`, `ApplyCostFloat`, or `ApplyCostFloatOrRefuse` inside them.

- [ ] **Step 3: Run and commit**

Run: `go test ./internal/actions ./internal/usercommands ./internal/mobcommands`

Expected: PASS.

Commit:

```bash
git add internal/actions internal/usercommands/context.md internal/mobcommands/context.md
git commit -m "refactor: centralize shared action admission"
```

---

### Task 4: Admit the physical special-move family atomically

**Files:**
- Modify: `internal/actions/combat_bash.go`
- Modify: `internal/actions/combat_trip.go`
- Modify: `internal/actions/combat_kick.go`
- Modify: `internal/actions/combat_grapple.go`
- Modify: `internal/actions/combat_hamstring.go`
- Modify: `internal/actions/combat_rake.go`
- Modify: `internal/actions/combat_maul.go`
- Modify: `internal/actions/combat_pounce.go`
- Modify: `internal/actions/combat_gore.go`
- Modify: `internal/actions/combat_drain.go`
- Modify: `internal/actions/combat_throttle.go`
- Modify: `internal/actions/combat_test.go`
- Modify: `internal/actions/combat_kick_raptor_test.go`
- Modify: `internal/actions/combat_rake_test.go`
- Modify: `internal/actions/combat_maul_test.go`
- Modify: `internal/actions/combat_pounce_test.go`
- Modify: `internal/actions/combat_gore_test.go`
- Modify: `internal/actions/combat_drain_test.go`
- Modify: `internal/actions/combat_throttle_test.go`
- Modify: `internal/usercommands/bash.go`
- Modify: `internal/usercommands/trip.go`
- Modify: `internal/usercommands/kick.go`
- Modify: `internal/usercommands/grapple.go`
- Modify: `internal/usercommands/rake.go`
- Modify: `internal/usercommands/maul.go`
- Modify: `internal/usercommands/pounce.go`
- Modify: `internal/usercommands/gore.go`
- Modify: `internal/usercommands/drain.go`
- Modify: `internal/usercommands/throttle.go`
- Modify: `internal/mobcommands/bash.go`
- Modify: `internal/mobcommands/charge.go`
- Modify: `internal/mobcommands/trip.go`
- Modify: `internal/mobcommands/kick.go`
- Modify: `internal/mobcommands/grapple.go`
- Modify: `internal/mobcommands/hamstring.go`
- Modify: `internal/mobcommands/rake.go`
- Modify: `internal/mobcommands/maul.go`
- Modify: `internal/mobcommands/pounce.go`
- Modify: `internal/mobcommands/gore.go`
- Modify: `internal/mobcommands/drain.go`
- Modify: `internal/mobcommands/throttle.go`
- Modify: `internal/actions/command_readiness_drift_test.go`

- [ ] **Step 1: Add one table-driven admission contract test**

The table must invoke all eleven shared actions and assert for each:

1. an invalid target/body/equipment/state returns before quoting;
2. an active `special-move` cooldown returns before quoting;
3. an otherwise-valid unaffordable action returns `CostRefused`;
4. refusal preserves cooldown and all action-specific state;
5. an affordable miss/fumble pays once and consumes the cooldown; and
6. higher governing skill or lower load reduces the quote through U7.

For grapple initiation, additionally assert no grapple state exists after refusal. For drain, assert `ExecuteDrainArea` does not acquire a cost. For mob aliases, assert `charge` charges only through `ExecuteTrip` and `howl` is not touched in this task.

Run: `go test ./internal/actions -run 'TestSpecialMove.*Admission' -v`

Expected: FAIL because the actions still consume cooldown before admission and are free.

- [ ] **Step 2: Normalize ordering in every shared action**

For each action, perform all read-only readiness and target/anatomy/immunity checks, then `CooldownReady`, then `admitFullCost` with `SpecialMoveBaseStaminaCost`, then `TryCooldown`, then resolution/effect/round consumption. A successful quote followed by `TryCooldown` returning false is a programming error in this synchronous path; return without effect and cover it with the stale-state test rather than charging again.

Embed the cost result in each existing result struct rather than adding unrelated `TooExhausted` booleans. Preserve all current progression behavior after execution.

- [ ] **Step 3: Update wrappers and alias guards**

Player wrappers render the shared stamina refusal text. Mob wrappers consume the same result without private messaging. Extend `command_readiness_drift_test.go` so readiness and execution agree on cooldown, target, anatomy, and grapple-state gates before cost admission.

- [ ] **Step 4: Run and commit**

Run:

```bash
go test ./internal/actions ./internal/usercommands ./internal/mobcommands
go test ./... -run 'Test(Charge|Howl|DrainArea).*Charge' -count=1
```

Expected: PASS; the alias and mutation/boss paths show no second charge.

Commit:

```bash
git add internal/actions internal/usercommands internal/mobcommands
git commit -m "feat: price physical special moves"
```

---

### Task 5: Admit shoot and reload before ammunition, round, or cooldown mutation

**Files:**
- Modify: `internal/actions/combat_fire.go`
- Modify: `internal/actions/combat_fire_test.go`
- Modify: `internal/actions/combat_reload.go`
- Modify: `internal/actions/combat_reload_test.go`
- Modify: `internal/usercommands/shoot.go`
- Modify: `internal/usercommands/shoot_test.go`
- Modify: `internal/usercommands/reload.go`
- Modify: `internal/mobcommands/shoot.go`
- Modify: `internal/mobcommands/reload.go`

- [ ] **Step 1: Write shoot-plus-reload cycle regressions**

For shoot, assert a refused action leaves `weapon.Loaded`, cooldown, round state, ammunition inventory, and target health unchanged. Assert an affordable miss still unloads, pays once, and consumes the round. For reload, assert refusal leaves the ammunition bundle count, `Loaded`, and cooldown unchanged; success consumes exactly one projectile, pays once, loads, and consumes cooldown.

Drive both player and mob wrappers and assert the same mechanical deltas. Assert the combined successful shoot-plus-reload charge equals the two independent quotes and agrees with Task 1's evidence fixture.

- [ ] **Step 2: Reorder `ExecuteFire`**

Complete target parse, visibility, weapon, loaded-state, ammunition compatibility, charm/non-combatant, and line-of-fire checks first. Check cooldown availability without consuming it. Commit `ActionShoot` / Stamina / `ShootBaseStaminaCost`. Only then set `Loaded = false`, consume the cooldown/round, and call the existing ranged resolver.

- [ ] **Step 3: Reorder `ExecuteReload`**

Complete busy, weapon, loaded-state, compatible-ammunition presence, and cooldown-availability checks first. Commit `ActionReload` / Stamina / `ReloadBaseStaminaCost`. Only then consume the projectile, set `Loaded`, and consume the cooldown. Do not move reload into usercommands; both actor types must use this path.

- [ ] **Step 4: Run and commit**

Run: `go test ./internal/actions ./internal/usercommands ./internal/mobcommands -run 'Test(Fire|Shoot|Reload)' -count=1`

Expected: PASS.

Commit:

```bash
git add internal/actions/combat_fire.go internal/actions/combat_fire_test.go internal/actions/combat_reload.go internal/actions/combat_reload_test.go internal/usercommands/shoot.go internal/usercommands/shoot_test.go internal/usercommands/reload.go internal/mobcommands/shoot.go internal/mobcommands/reload.go
git commit -m "feat: price ranged fire and reload"
```

---

### Task 6: Admit throw and sneak before item or awareness mutation

**Files:**
- Modify: `internal/usercommands/throw.go`
- Modify: `internal/usercommands/throw_test.go`
- Modify: `internal/actions/sneak.go`
- Modify: `internal/actions/sneak_test.go`
- Modify: `internal/usercommands/skill.skullduggery.sneak.go`
- Modify: `internal/mobcommands/sneak.go`

- [ ] **Step 1: Write preservation tests**

Throw refusal must preserve the exact item instance, stack quantity, special-move cooldown, target health, and round state. Sneak refusal must leave awareness out of Concealing/Hidden, preserve cooldown/round state, and fire no progression. Affordable failed attempts pay and retain existing failure/progression semantics.

Run: `go test ./internal/actions ./internal/usercommands ./internal/mobcommands -run 'Test(Throw|Sneak).*Cost' -v`

Expected: FAIL because throw consumes cooldown before item resolution and sneak transitions before cost admission.

- [ ] **Step 2: Reorder throw**

Resolve target, item, ownership, throwable validity, and cooldown availability read-only. Commit `ActionThrow` / Stamina / `SpecialMoveBaseStaminaCost`, then consume cooldown, call `UseItem`, consume the round, and resolve the contest. Do not add a second shared-action wrapper solely for throw; its only live implementation remains the player command.

- [ ] **Step 3: Reorder shared sneak**

In `actions.Sneak`, complete actor/state/room and cooldown checks, commit `ActionSneak` / Stamina / `SneakBaseStaminaCost`, then call `TransitionToConcealing` and run existing contests. Return the structured cost to both wrappers; only the user wrapper prints refusal.

- [ ] **Step 4: Run and commit**

Run: `go test ./internal/actions ./internal/usercommands ./internal/mobcommands -run 'Test(Throw|Sneak)' -count=1`

Expected: PASS.

Commit:

```bash
git add internal/usercommands/throw.go internal/usercommands/throw_test.go internal/actions/sneak.go internal/actions/sneak_test.go internal/usercommands/skill.skullduggery.sneak.go internal/mobcommands/sneak.go
git commit -m "feat: admit throw and sneak costs atomically"
```

---

### Task 7: Admit taunt, rally, and warcry before social effects

**Files:**
- Modify: `internal/actions/combat_taunt.go`
- Modify: `internal/actions/combat_rally.go`
- Modify: `internal/actions/combat_warcry.go`
- Modify: `internal/actions/command_readiness_drift_test.go`
- Modify: `internal/actions/contest_sign_taunt_test.go`
- Modify: `internal/actions/rhetoric_progression_test.go`
- Modify: `internal/usercommands/taunt.go`
- Modify: `internal/usercommands/rally.go`
- Modify: `internal/usercommands/warcry.go`
- Modify: `internal/mobcommands/taunt.go`
- Modify: `internal/mobcommands/howl.go`
- Modify: `internal/mobcommands/rally.go`
- Modify: `internal/mobcommands/warcry.go`

- [ ] **Step 1: Write Conviction admission tests**

Assert unaffordable taunt creates no aggro/conviction harm, unaffordable rally applies no buff, and unaffordable warcry applies no room buffs. All three preserve cooldown and round state. Affordable failed taunt still pays; affordable rally/warcry pay once before applying effects. Encumbrance must not change any quote, while Rhetoric rank must.

- [ ] **Step 2: Reorder the shared actions**

After read-only validity and `CooldownReady`, commit `ActionTaunt`, `ActionRally`, or `ActionWarcry` against Conviction with `RhetoricActionBaseConvictionCost`. Only then consume the special-move cooldown and apply contest/buffs/progression. Mob `howl` delegates to `ExecuteTaunt` and must not quote independently.

- [ ] **Step 3: Run and commit**

Run: `go test ./internal/actions ./internal/usercommands ./internal/mobcommands -run 'Test(Taunt|Rally|Warcry|Howl)' -count=1`

Expected: PASS.

Commit:

```bash
git add internal/actions internal/usercommands internal/mobcommands
git commit -m "feat: price rhetoric actions"
```

---

### Task 8: Plan and charge autoattack before hit resolution

**Files:**
- Modify: `internal/combat/combat.go`
- Modify: `internal/combat/combat_helpers.go`
- Modify: `internal/combat/attack_cost.go`
- Modify: `internal/combat/attack_cost_test.go`
- Create: `internal/combat/attack_admission_test.go`

- [ ] **Step 1: Write the exhausted multi-swing regression**

Build a deterministic attacker with multiple planned swings and too little Stamina for the aggregate quote. Assert:

- every planned swing is still attempted;
- Stamina reaches zero but never negative;
- the cost result is short once for the round;
- hit scores omit only `GetCombatSkillLevel()*SkillWeight`;
- swing count and weapon damage still use the combat skill exactly as before; and
- the player shortage message is emitted once, not once per swing.

Add an affordable control showing identical hit-score inputs and damage to pre-U8 behavior except for admission timing.

- [ ] **Step 2: Extract a pre-resolution attack plan**

Create an internal `attackPlan` holding the prepared `[]weaponSetup` and total swing count. Move `collectAttackWeapons`, `buildWeaponSetup`, and `calcSwingCount` use ahead of hit resolution so the aggregate quote is known before the first roll. `calculateCombat` consumes this plan rather than recalculating it.

Add `omitAttackSkill bool` to `combatContext`. In `calcAttackScore`, skip only the equipped combat-skill addend when it is true. Do not thread the flag into damage parameters or swing-count calculation.

- [ ] **Step 3: Replace post-resolution charging**

Quote `ActionAttack` with `Units: plan.totalSwings`, partially commit once, set `omitAttackSkill` from `result.Short()`, then resolve the plan. Delete or reduce `ChargeAttackCost` to the new admission wrapper; no wrapper may charge after `calculateCombat` returns.

- [ ] **Step 4: Run and commit**

Run: `go test ./internal/combat -run 'Test(Attack|Autoattack).*Cost|Test.*Swing' -count=1`

Expected: PASS.

Commit:

```bash
git add internal/combat
git commit -m "feat: resolve short autoattacks without skill"
```

---

### Task 9: Quote all defences, commit only the winner, and expose channel outcomes

**Files:**
- Modify: `internal/characters/combat.go`
- Create: `internal/characters/defense_score_test.go`
- Modify: `internal/combat/combat_helpers.go`
- Modify: `internal/combat/defence_multiplier.go`
- Modify: `internal/combat/contest_sign_test.go`
- Create: `internal/combat/defence_admission_test.go`
- Modify: `internal/actions/combat_taunt.go`
- Modify: `internal/hooks/spell_resolution.go`

- [ ] **Step 1: Split defence score construction without changing the full score**

Add:

```go
func (c *Character) GetDefenseScoreFor(defenseType string, includeSkill bool) float64

func (c *Character) GetDefenseScore(defenseType string) float64 {
	return c.GetDefenseScoreFor(defenseType, true)
}
```

Move only the dodge/parry/block/quell/defy governing-skill addend behind `includeSkill`; all stats, equipment, prone, visibility, resource, and effectiveness multipliers stay where they are. Test that `includeSkill=true` exactly equals the legacy method and false differs by only that term.

- [ ] **Step 2: Write best-of-all admission tests**

Provide mixed-affordability candidates and deterministic contest results. Assert all eligible defences enter, each candidate includes skill iff its own quote is affordable, only `Result.Winner` commits, losing short candidates neither charge nor message nor progress, and a short winner does all three once. Assert dodge/parry/block use Stamina and quell/defy use Conviction.

- [ ] **Step 3: Implement quote-before-contest in melee**

Pair each contest entry with its `CostQuote`. Build its score with `GetDefenseScoreFor(type, quote.Affordable())`. After `RunContest`, partially commit only the winner and store `CostCommitResult` in `bestDefenseResult`. Keep winner progression and narration after that commit.

- [ ] **Step 4: Return a structured channel-defence result**

Replace the scalar return with:

```go
type ChannelDefenceResult struct {
	DamageMultiplier float64
	DefenceType      string
	DefenseZScore    float64
	Defended         bool
	DefensiveCrit    bool
	Cost             characters.CostCommitResult
}
```

`ResolveChannelDefence` follows the same quote-all/commit-winner algorithm. Update every spell and taunt caller to read `.DamageMultiplier`. Keep the existing attack/defence margin sign and mitigation curve tests unchanged; extend them to assert the structured fields.

- [ ] **Step 5: Run and commit**

Run:

```bash
go test ./internal/characters -run 'TestGetDefenseScore' -count=1
go test ./internal/combat ./internal/actions ./internal/hooks -run 'Test.*Defen|Test.*Quell|Test.*Defy|Test.*Taunt|Test.*Spell' -count=1
```

Expected: PASS; statistical contest-sign tests retain their existing bounds.

Commit:

```bash
git add internal/characters internal/combat internal/actions/combat_taunt.go internal/hooks/spell_resolution.go
git commit -m "feat: resolve short defences without skill"
```

---

### Task 10: Make flee partial-pay and skill-less when short

**Files:**
- Modify: `internal/usercommands/flee.go`
- Modify: `internal/usercommands/flee_cost_test.go`
- Modify: `internal/combat/flee.go`
- Modify: `internal/combat/flee_test.go`
- Modify: `internal/hooks/NewRound_DoCombat_helpers.go`

- [ ] **Step 1: Write flee admission and blocker tests**

Assert a short flee charges remaining Stamina, still transitions to disengaging, and resolves every blocker using Dexterity plus prone penalty but no Skullduggery. Assert an affordable flee includes Skullduggery. Assert flying modifies the quoted base before commit and the shortage message appears once per flee command, never once per blocker.

- [ ] **Step 2: Carry the short state into blocker resolution**

In the command, quote `ActionFlee` using `FleeStaminaCost` with the existing flight modifier and partially commit. Pass `includeSkill := !costResult.Short()` to:

```go
func ResolveFleeBlockers(fleer *characters.Character, room *rooms.Room, includeSkill bool) *FleeBlocker
```

Remove only the Skullduggery term when false. Preserve blocker Unarmed Combat, prone penalty, opponent ordering, and `RunContest` calls.

- [ ] **Step 3: Run and commit**

Run: `go test ./internal/usercommands ./internal/combat ./internal/hooks -run 'Test.*Flee' -count=1`

Expected: PASS.

Commit:

```bash
git add internal/usercommands/flee.go internal/usercommands/flee_cost_test.go internal/combat/flee.go internal/combat/flee_test.go internal/hooks/NewRound_DoCombat_helpers.go
git commit -m "feat: resolve short flee attempts without skill"
```

---

### Task 11: Fold grapple maintenance into unified partial admission

**Files:**
- Modify: `internal/hooks/Position_GrappleTick.go`
- Modify: `internal/hooks/Position_GrappleTick_test.go`
- Create: `internal/hooks/grapple_cost_test.go`
- Modify: `internal/hooks/context.md`
- Modify: `internal/grapplemessaging/context.md`

- [ ] **Step 1: Write role and escape-round regressions**

Assert controller and controlled quote independently, controlled cost remains greater at empty/typical/knee/capacity loads, either participant can lose only its own Unarmed Combat term, both pay on the round an escape resolves, and no pool goes negative. Assert existing stamina depletion and encumbrance effectiveness multipliers remain in the score even when skill is omitted.

- [ ] **Step 2: Replace the private integer debit**

For each participant, calculate the role-adjusted base first:

```go
base := float64(cfg.GrappleStaminaCostPerRound) * roleMultiplier
```

Quote `ActionGrappleMaintain` / Stamina / `base`, partially commit, then call `grappleScore(character, role, cfg, !result.Short())`. Do this for both sides before `RunContest`. Delete `applyGrappleStaminaCost` and any `math.Round` path it used.

- [ ] **Step 3: Add one shortage message per participant per round**

Use grapple messaging's existing audience route to tell a player participant that trained control is slipping. Do not repeat on submission attempts, reversals, observers, or targets; NPCs receive no invisible private text.

- [ ] **Step 4: Run and commit**

Run: `go test ./internal/hooks ./internal/grapplemessaging -run 'Test.*Grapple' -count=1`

Expected: PASS and Task 1's grapple traces still match.

Commit:

```bash
git add internal/hooks internal/grapplemessaging/context.md
git commit -m "feat: unify grapple maintenance costs"
```

---

### Task 12: Add coordinated quell and defy defence-message pools

**Files:**
- Modify: `internal/items/defensive_messages.go`
- Create: `internal/items/defensive_messages_test.go`
- Create: `_datafiles/world/dogmud/defense-messages/quell.yaml`
- Create: `_datafiles/world/dogmud/defense-messages/defy.yaml`
- Modify: `internal/combat/defence_multiplier.go`
- Create: `internal/combat/channel_defence_messages_test.go`
- Modify: `internal/hooks/spell_resolution.go`
- Modify: `internal/actions/combat_taunt.go`
- Modify: `internal/items/context.md`
- Modify: `internal/combat/context.md`

- [ ] **Step 1: Strengthen loader validation first**

Add `DefenseQuell` and `DefenseDefy`. For every defence file, validation must reject:

- a missing weak, normal, or heavy band;
- fewer than five variants in any audience list;
- an empty defender, attacker, or room list; and
- unequal audience-list lengths within a band.

Test each rejection and a valid five-variant group. Run:

`go test ./internal/items -run 'TestDefenseMessage.*Valid' -v`

Expected: FAIL until validation is implemented.

- [ ] **Step 2: Add one coordinated selector**

Add a renderer that selects one random index from the band and uses that same index for `ToDefender`, `ToAttacker`, and `ToRoom`. Do not call `MessageOptions.Get()` independently three times. Apply token replacement after selection and return one triad to the caller.

Test with uniquely numbered audience strings so all three rendered messages prove they used the same index.

- [ ] **Step 3: Author the two YAML files**

Each file has weak, normal, and heavy bands, each with at least five coordinated audience triads. Weak copy describes a narrow or partial response without claiming the effect vanished. Normal copy describes a clean defence. Heavy copy is valid for defensive-crit full negation. Use existing message tokens and no raw mechanics.

- [ ] **Step 4: Route live spell and social outcomes**

Use `ChannelDefenceResult` to select quell for mental-spell defence and defy for social defence, render exactly one coordinated triad, and send it through the existing defender/attacker/room audience routes. Remove the replaced hardcoded one-off messages. For AoE spells, narrate once per actual target outcome and never add a duplicate shortage line to observers.

Add integration tests proving partial mitigation selects weak/normal-compatible wording without “fully negated” claims, defensive crit selects heavy full-negation wording, and all three audiences receive the matching variant.

- [ ] **Step 5: Run and commit**

Run:

```bash
go test ./internal/items ./internal/combat ./internal/actions ./internal/hooks -run 'Test.*(DefenseMessage|Quell|Defy|ChannelDefence)' -count=1
go test ./internal/items ./internal/actions ./internal/hooks
```

Expected: PASS; both YAML files load through the real data loader.

Commit:

```bash
git add internal/items internal/combat internal/hooks/spell_resolution.go internal/actions/combat_taunt.go _datafiles/world/dogmud/defense-messages
git commit -m "feat: add quell and defy defence narration"
```

---

### Task 13: Finish player messaging, help, package context, and roadmap records

**Files:**
- Modify: `_datafiles/world/dogmud/templates/help/combat.template`
- Modify: `_datafiles/world/dogmud/templates/help/stamina.template`
- Modify: `_datafiles/world/dogmud/templates/help/ranged-combat.template`
- Modify: `_datafiles/world/dogmud/templates/help/shoot.template`
- Modify: `_datafiles/world/dogmud/templates/help/reload.template`
- Modify: `_datafiles/world/dogmud/templates/help/grapple.template`
- Modify: `_datafiles/world/dogmud/templates/help/sneak.template`
- Modify: `_datafiles/world/dogmud/templates/help/throw.template`
- Modify: `_datafiles/world/dogmud/templates/help/taunt.template`
- Modify: `_datafiles/world/dogmud/templates/help/rally.template`
- Modify: `_datafiles/world/dogmud/templates/help/warcry.template`
- Modify: `_datafiles/world/dogmud/templates/help/quell.template`
- Modify: `_datafiles/world/dogmud/templates/help/defy.template`
- Modify: `_datafiles/world/dogmud/templates/help/weapon-combat.template`
- Modify: `_datafiles/world/dogmud/templates/help/unarmed-combat.template`
- Modify: touched `internal/*/context.md` files named in Tasks 2-12
- Modify: `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`
- Modify: `docs/roadmaps/CURRENT_BACKLOG.md`
- Modify: `docs/PATCH_NOTES.md`

- [ ] **Step 1: Add refusal and shortage text tests**

Use output capture to assert one concise, pool-aware line for each voluntary refusal and one non-failure shortage line for autoattack, winning defence, flee, and grapple maintenance. Assert no raw point values, percentages, skill ranks, or modifiers appear; losing defence candidates and NPC actors emit no private line.

- [ ] **Step 2: Update helpfiles**

Explain which actions consume Stamina or Conviction, that load and governing skill affect price, that voluntary actions require full payment, and that autoattack/defence/flee/grapple maintenance remain possible in a desperate skill-less form. Mention reload as physical exertion and grapple's two-sided upkeep. Keep prose player-facing and omit numeric tuning.

- [ ] **Step 3: Audit package context files**

Update `costs`, `characters`, `configs`, `actions`, `usercommands`, `mobcommands`, `combat`, `hooks`, `items`, and `grapplemessaging` context files where touched. Remove stale references to post-resolution attack charging, flat grapple upkeep, affordability-gated defences, and hardcoded quell/defy narration.

Run:

```bash
rg -n "ChargeAttackCost|applyGrappleStaminaCost|too exhausted to flee|hardcoded.*(quell|defy)" internal --glob 'context.md'
```

Expected: no stale behavioral claims.

- [ ] **Step 4: Update project records**

Mark U8 implementation complete only after Task 14 passes. Until then, update the roadmap row with the implementation branch state, keep both recorded follow-ups (mutation-active dead-code audit and unified combat messaging) in `CURRENT_BACKLOG.md`, and add a dated, player-facing `PATCH_NOTES.md` entry with no raw numbers and no em dashes.

- [ ] **Step 5: Run and commit documentation checks**

Run: `go test ./internal/templates ./internal/usercommands ./internal/actions`

Expected: help templates load and touched messaging tests pass.

Commit:

```bash
git add _datafiles/world/dogmud/templates/help internal docs/roadmaps docs/PATCH_NOTES.md
git commit -m "docs: explain unified action admission"
```

---

### Task 14: Run recurrence guards, full verification, boot, and adversarial playtest

**Files:**
- Modify if defects are found: implementation/test/content files from Tasks 1-13
- Modify after the gate passes: `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`
- Modify after the gate passes: `docs/PATCH_NOTES.md`

- [ ] **Step 1: Run focused recurrence guards**

Run:

```bash
go test ./internal/costs ./internal/characters ./internal/configs
go test ./internal/combat ./internal/actions ./internal/hooks ./internal/items
go test ./internal/usercommands ./internal/mobcommands ./internal/grapplemessaging
go test ./... -run 'Test.*(Cost|Admission|Short|Flee|Grapple|Quell|Defy|Floor|Contest)' -count=1
python tools/balance/unified_resolution_model.py
```

Expected: all pass. The model output matches the committed evidence table.

- [ ] **Step 2: Search for bypasses and stale paths**

Run:

```bash
rg -n "ApplyCost(Float|FloatOrRefuse|Partial)?\(" internal/actions internal/combat internal/hooks internal/usercommands internal/mobcommands
rg -n "contest\.(Run|AgainstDifficulty)|dice\.(OpposedRollStat|OpposedRollStatWithFloors|OpposedRoll)" internal --glob '*.go'
rg -n "Cooldowns\.Try|TryCooldown" internal/actions internal/usercommands/throw.go
```

Review every result. Expected: no U8 action bypasses `QuoteActionCost`/`CommitCost`; no new opposed-roll bypass exists; every consuming cooldown call follows successful full admission.

- [ ] **Step 3: Run formatting, build, and the complete test suite**

Run:

```bash
gofmt -l internal/ modules/
go build ./...
go test ./...
```

Expected: `gofmt` prints nothing; build and tests exit 0.

- [ ] **Step 4: Run the isolated boot check**

Follow the repository Pre-Push SOP exactly: create a detached worktree at `C:/tmp/dogmud-boot-check`, copy `_datafiles/config.yaml`, build a fixed `boot-check.exe`, run it for up to 180 seconds, and inspect the log.

Expected: timeout exit 124, exactly one `Server Ready`, and zero matches for `^panic:|goroutine [0-9]+ \[running\]|runtime error`. Remove the temporary worktree afterward.

- [ ] **Step 5: Run the mandatory adversarial content playtest**

Invoke the `source-command-playtest` skill and use an isolated checkout/environment. Give the agent an explicitly critical, confused-player mandate. Exercise:

- every voluntary family when affordable, too depleted, and recovered;
- player and available NPC equivalents;
- multi-swing exhausted autoattack and one-message-per-round behavior;
- all five defences, including a losing short candidate and a winning short candidate;
- flee against multiple blockers;
- grapple initiation refusal, both maintenance roles, and an escape round;
- shoot/reload ammunition preservation;
- throw item preservation;
- sneak concealment preservation;
- rally/warcry buff preservation;
- weak, normal, and heavy quell and defy narration from defender, attacker, and room viewpoints; and
- repeated-action text for duplication, contradiction, awkward voice, or claims that a partial defence fully erased an effect.

Boot-clean and YAML parsing are not substitutes for this step. Fix every defect, add a regression where feasible, and rerun the affected scenario until clean.

- [ ] **Step 6: Final review and commit**

Run `git diff --check`, inspect `git status --short`, and confirm only U8 files plus the approved plan/evidence are staged. Preserve `.agents/`, `AGENTS.md`, and unrelated user changes.

After every gate passes, mark U8 done in the roadmap, finalize patch notes, and commit:

```bash
git add docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md docs/PATCH_NOTES.md
git commit -m "feat: complete unified action cost admission"
```

If the playtest required code, test, YAML, help, or model corrections after their earlier task commits, stage those reviewed U8 paths explicitly before this commit. Do not use a broad `git add internal`, `git add _datafiles`, or `git add docs`, because the shared workspace may contain unrelated user work.

Do not claim completion until the final command outputs and playtest transcript have been inspected in the current session.

---

## Spec coverage checklist

- Quote/commit ownership and all four statuses: Tasks 2-3.
- Full/refuse voluntary matrix and ordering: Tasks 4-7.
- Autoattack aggregate partial payment and skill stripping: Task 8.
- Best-of-all physical, magical, and social defence semantics: Task 9.
- Flee partial payment and skill stripping: Task 10.
- Grapple maintenance role pricing and independent shortage: Task 11.
- Quell/defy validation, content, coordinated selection, and live routing: Task 12.
- Helpfiles, context, config comments, patch notes, roadmap, and backlog follow-ups: Task 13.
- Model evidence, recurrence guards, clean boot, and adversarial in-game review: Tasks 1 and 14.
- Out-of-scope protections for surprise attack, mutation cleanup, boss drain, aliases, U9 progression, and broad messaging unification: Global Constraints plus Tasks 4 and 13.
