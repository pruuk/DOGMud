# U8: unified action-cost admission

**Created:** 2026-08-17
**Slice:** U8 of the unified-resolution arc
**Depends on:** U7 unified costs and U7b reservation ceiling
**Roadmap:** [`UNIFIED_RESOLUTION_ROADMAP.md`](../../roadmaps/UNIFIED_RESOLUTION_ROADMAP.md)
**Supersedes for U8:** the insufficient-resource and reservation wording in
[`2026-08-12-unified-cost-and-harm-design.md`](2026-08-12-unified-cost-and-harm-design.md)

---

## 1. Why this slice exists

U7 built one cost formula and moved attacks, five defences, movement and flee
onto it. U7b made the usable pool safe by capping reservation. The remaining
combat-action surface is still inconsistent: ranged attacks, reloads, social
moves, sneak, grapple initiation and the special-move family are free, while
grapple maintenance uses a private integer formula.

U8 finishes the seam. The objective is not merely to add debits. It is to make
action admission one explicit contract:

1. quote the existing U7 formula without mutation;
2. choose the settled full-payment or life-preserving policy;
3. commit the exact quote through the existing fractional-carry rules; and
4. resolve the action through the existing contest or deterministic path.

`combat.RunContest` remains the one opposed-roll entry point. Cost admission
does not become a second contest implementation and the contest core does not
gain mutable character, cooldown or inventory responsibilities.

---

## 2. Settled decisions

| # | Decision |
|---|---|
| D1 | Extend the existing U7 registry and `costs.Calc`; do not create another cost formula or profile engine. |
| D2 | Introduce a shared, non-mutating quote followed by an explicit full or partial commit. |
| D3 | Voluntary actions pay in full or refuse before consuming cooldown, item, ammunition, round, transition or effect. |
| D4 | Autoattack, defence, flee and grapple maintenance remain available when exhausted; they pay partially and omit their skill term when short. |
| D5 | Reload is a physical Ranged Combat action and costs stamina. |
| D6 | Grapple initiation is full-pay; grapple maintenance is partial-pay and joins the U7 formula. |
| D7 | Only the opposed-roll score loses skill. Damage scaling, crits, fumbles, floors, mitigation and progression remain unchanged. |
| D8 | Where both player and NPC entry points exist, their wrappers use the same action entry, quote and commit behavior. U8 does not invent new NPC commands solely for parity. |
| D9 | Relevant helpfiles and every touched package's `context.md` ship current in U8 rather than waiting for U11. |
| D10 | Surprise attack remains in U10; mutation-active dead-code cleanup is a recorded follow-up, not U8 scope. |

---

## 3. Action and payment matrix

### 3.1 Existing cost surfaces whose shortage behavior U8 completes

| Action | Pool | Governing skill | Physical | If short |
|---|---|---|---:|---|
| Armed autoattack | Stamina | equipped combat skill | yes | partial-pay; all planned swings resolve without the skill term |
| Unarmed autoattack | Stamina | Unarmed Combat | yes | partial-pay; all planned swings resolve without the skill term |
| Dodge | Stamina | Unarmed Combat | yes | partial-pay; defend without the skill term |
| Parry | Stamina | Weapon Combat | yes | partial-pay; defend without the skill term |
| Block | Stamina | Weapon Combat | yes | partial-pay; defend without the skill term |
| Quell | Conviction | Spellcasting | no | partial-pay; defend without the skill term |
| Defy | Conviction | Rhetoric | no | partial-pay; defend without the skill term |
| Flee | Stamina | Skullduggery | yes | partial-pay; blocker contests omit the skill term |

Quell and defy already charge flat Conviction bases through U7. U8 does not add
a second charge or restore the old incoming-cost-fraction proposal.

### 3.2 New U8 cost surfaces

| Action | Pool | Governing skill | Physical | If short |
|---|---|---|---:|---|
| Shoot | Stamina | Ranged Combat | yes | refuse |
| Reload | Stamina | Ranged Combat | yes | refuse before ammunition or cooldown |
| Bash | Stamina | Weapon Combat | yes | refuse |
| Trip | Stamina | Unarmed Combat | yes | refuse |
| Kick | Stamina | Unarmed Combat | yes | refuse |
| Grapple initiation | Stamina | Unarmed Combat | yes | refuse |
| Hamstring | Stamina | Unarmed Combat | yes | refuse |
| Rake | Stamina | Unarmed Combat | yes | refuse |
| Maul | Stamina | Unarmed Combat | yes | refuse |
| Pounce | Stamina | Unarmed Combat | yes | refuse |
| Gore | Stamina | Unarmed Combat | yes | refuse |
| Drain | Stamina | Unarmed Combat | yes | refuse |
| Throttle | Stamina | Unarmed Combat | yes | refuse |
| Throw | Stamina | Skullduggery | yes | refuse before item use or cooldown |
| Sneak | Stamina | Skullduggery | yes | refuse before concealment begins |
| Taunt | Conviction | Rhetoric | no | refuse |
| Rally | Conviction | Rhetoric | no | refuse before buff application |
| Warcry | Conviction | Rhetoric | no | refuse before buff application |

Aliases do not get duplicate actions or charges: mob `charge` delegates to trip
and mob `howl` delegates to taunt.

Submission attempts and opportunistic grapple reversals are outcomes inside the
existing grapple lifecycle, not new voluntary actions. They receive no separate
admission charge; grapple maintenance already prices that round.

### 3.3 Grapple maintenance

Grapple upkeep becomes a registered physical Unarmed Combat action. Both sides
quote and partially commit before the drift contest. Either side may therefore
lose its own skill term independently.

The existing controller/controlled asymmetry stays load-bearing:

```
controller base = GrappleStaminaCostPerRound x GrappleControllerCostMultiplier
controlled base = GrappleStaminaCostPerRound x GrappleControlledCostMultiplier
```

That role multiplier modifies the base **before** `costs.Calc`. Base sits
outside `CostTotalMultiplierMax`, preserving the shipped 2:1 controlled-side
premium at every load. Passing the role multiplier as the action modifier would
let the product clamp collapse the distinction under heavy encumbrance.

Grapple's existing stamina-depletion and encumbrance effectiveness penalties
remain. U8 additionally makes load raise maintenance cost and makes an unpaid
whole charge remove the Unarmed Combat term. This cumulative pressure must be
modelled before choosing the shipped base.

---

## 4. Architecture

### 4.1 Package ownership

`internal/costs` remains a pure leaf package. It owns:

- action names and registry specifications;
- fixed versus equipped-combat skill selection metadata;
- physical classification; and
- the existing base x encumbrance x inverse-skill x modifier calculation.

`internal/characters` owns resource state. It creates and commits quotes because
only `Character` can consistently see current pools and per-pool fractional
carry.

`internal/combat`, `internal/actions`, `internal/hooks` and
`internal/usercommands` decide **when** an otherwise-valid action requests its
quote. They do not reproduce the cost formula or mutate pools directly.

`combat.RunContest` receives final scores. It never quotes, charges, checks
cooldowns or mutates inventory.

### 4.2 Cost quote

A quote is an immediate, single-use resolution object, not a reservation held
across rounds. Conceptually it records:

- action and pool;
- calculated fractional amount;
- whole amount due after the pool's current fractional carry;
- affordability against the current pool; and
- enough carry state to commit exactly what was quoted.

Quoting mutates nothing. The implementation must share one internal calculation
between quote and commit so affordability cannot drift from the charged amount.
If pool or carry state changes incompatibly before commit, commit revalidates or
rejects the stale quote rather than silently charging a different cost.

### 4.3 Commit policies

| Commit | Contract |
|---|---|
| Full or refuse | Affordable: update carry and pay the whole amount. Unaffordable: mutate neither carry nor pool. |
| Partial | Update carry, charge what is available, write off the unpaid whole portion and report `Short`. |

The quote/commit seam preserves the shipped primitives' edge cases:

- sub-one-point costs accumulate;
- refusal does not bank debt;
- partial-payment debt is never reclaimed later;
- a floored whole charge of zero is not short;
- non-positive or non-finite inputs are harmless and bank nothing; and
- a cost never drives a pool below zero.

Affordability reads the current pool directly. It must **never** subtract
reservation or use `EffectivePoolMax`: U7 already reserve-clamps the current
pool, so another subtraction would double-count reservation.

### 4.4 Structured status

Callers consume a shared cost result rather than growing unrelated booleans:

- paid;
- partially paid;
- refused; or
- no whole charge due after fractional carry.

The status names the relevant pool but exposes no player-visible numbers.
Player and NPC wrappers consume the same mechanical status; only player wrappers
emit private explanatory text.

---

## 5. Ordering

### 5.1 Voluntary actions

1. Complete read-only validity checks: busy state, target, equipment, body
   parts, immunity, ammunition/item presence, existing buff/state and cooldown
   availability.
2. Quote the cost.
3. Attempt full commit.
4. On refusal, return with no cooldown, ammunition, item use, combat round,
   awareness transition or effect consumed.
5. On payment, consume cooldown and other action resources, perform awareness
   transitions, then resolve the contest or deterministic effect.

A miss or fumble still pays because a valid action was attempted. Cooldown APIs
that currently combine checking and consumption need a read-only admission check
or equivalent ordering; U8 must not pay and then discover the cooldown was busy.

Specific ordering requirements:

- reload refuses before consuming ammunition or setting `Loaded`;
- throw refuses before `UseItem` and before its shared cooldown;
- sneak refuses before `TransitionToConcealing`;
- rally and warcry refuse before buff or condition application; and
- grapple initiation refuses before creating grapple state.

### 5.2 Autoattack

Determine the round's swing plan and aggregate price before hit resolution,
then commit partially. When the aggregate quote is short, every planned swing
still occurs but its hit score omits the equipped combat-skill term. U8 does not
reduce damage skill multipliers on the rare hit that still lands.

The shortage message appears once for the round, never per swing.

### 5.3 Defence

Quote every eligible defence without mutation. Build each candidate score with
its skill term only when that quote is affordable, then run the existing
best-of-all contest. Commit only `contest.Result.Winner`, whether or not the
attack ultimately deals damage.

Do not message, charge or progress a losing candidate merely because its quote
was short. Quell and defy use Conviction; dodge, parry and block use Stamina.

### 5.4 Flee

Quote and partially commit once before blocker contests. A short flee still
resolves every blocker contest, with the fleer's Skullduggery term omitted.

### 5.5 Grapple maintenance

For the once-per-pair round tick:

1. quote and partially commit controller maintenance;
2. quote and partially commit controlled maintenance;
3. build each drift score with or without its own Unarmed Combat term;
4. run the existing contest; and
5. apply the unchanged outcome, transition and messaging rules.

Both sides continue paying on the round an escape resolves, matching current
ordering. No grapple state-machine redesign is part of U8.

---

## 6. Configuration and modelling

Every new action receives a config-owned base. Actions intentionally sharing a
base may receive documented relative modifiers, but U8 does not add modifier
knobs solely to restate neutral `1.0`. Aliases, player wrappers and mob wrappers
never receive duplicate knobs.

Every key has a validated Go fallback and an adjacent shipped-config comment.
No live balance number is hardcoded under `internal/`.

Before values are selected, extend the repository balance model across:

- novice, mid-skill, veteran and live-character skill bands;
- empty, typical, knee and maximum encumbrance;
- current stamina and conviction pools and regeneration;
- action frequency and shared-cooldown cadence;
- combined shoot-plus-reload ranged cycles;
- taunt, rally and warcry against conviction regeneration and reservation;
- multi-round grapple controller and controlled timelines; and
- the combined grapple cost, depletion, encumbrance and skill-strip pressure.

For a cooldown-gated physical maneuver, the target cost is modestly above one
ordinary swing and no greater than four ordinary swings for the **same** typical
character, load and governing skill. Four means four swings, not four full
multi-swing combat rounds.

Acceptance constraints:

- ranged and rhetoric playstyles become resource-bound without being priced
  out;
- reload contributes to ranged fatigue without dominating shoot cost;
- typical-load characters can sustain cooldown-spaced manoeuvres;
- the controlled grappler remains dearer at every modelled load;
- exhaustion remains punishing while life-preserving actions remain available;
  and
- the existing product clamp still protects a laden novice from absurd cost
  multiplication.

Model output and chosen shipped values are evidence attached to the plan or
implementation, not numbers guessed inside Go code.

---

## 7. Skill stripping and progression boundary

`Short` removes only the governing skill term from the applicable opposed-roll
score:

- equipped combat skill from autoattack hit score;
- the selected defence's skill from its candidate score;
- Skullduggery from flee blocker contests; and
- Unarmed Combat from that participant's grapple drift score.

It does not change damage skill multipliers, crit/fumble interpretation,
contest floors, defence sets, mitigation, resource multipliers or grapple
outcome thresholds.

Existing progression hooks remain behavior-compatible. An executed action still
fires whatever progression hook it fires today even when its contest score was
skill-less. U9 owns replacing those side effects with events and distinguishing
attempting, doing and observing. U8 must not partially implement U9.

---

## 8. Player-facing messaging

Voluntary refusal uses concise pool-aware language: physically too spent for a
Stamina action, unable to muster the necessary resolve for a Conviction action.

Partial shortage explains why training did not contribute without claiming the
action automatically failed:

- autoattack: exhausted movement prevents training from coming cleanly through;
- defence: the selected response is desperate rather than practised;
- flee: the character breaks away on instinct rather than technique; and
- grapple: strength and leverage remain while trained control slips.

Rules:

- no raw costs, pool values, modifiers, ranks or percentages;
- one message per action or combat round;
- no messages for losing defence candidates;
- no repetition per swing, blocker, observer, AoE target or grapple target;
- existing combat narration remains authoritative for the actual outcome; and
- NPCs use identical mechanics without invisible player-private messages.

Because U8 authors player-facing copy, the content playtest-review SOP applies.
The plan ends with an explicitly adversarial in-game pass over exhausted,
refused and recovered states.

---

## 9. Documentation in this slice

U8 updates behavior-specific documentation now rather than leaving stale text
for U11:

- affected helpfiles for combat resources, defences, ranged combat, reload,
  sneak, grapple, rhetoric actions and special moves;
- every touched package's `context.md`, expected to include `costs`,
  `characters`, `combat`, `actions`, `hooks`, `usercommands` and `configs` as
  applicable;
- adjacent `_datafiles/config.yaml` comments;
- `docs/PATCH_NOTES.md`;
- this spec and the unified-resolution roadmap; and
- direct help cross-references made inaccurate by U8.

U11 remains the holistic config organization, help registry/category and
arc-closing documentation audit. It should not be asked to reconstruct U8's
behavior after the fact.

---

## 10. Verification

### 10.1 Quote and commit

- quoting mutates neither pool nor carry;
- full commit is atomic;
- refusal mutates neither pool nor carry;
- partial commit reports the exact charge and shortage;
- fractional carry, zero-whole, non-finite and non-positive cases retain their
  current contracts;
- reservation is not subtracted twice; and
- stale quotes cannot silently charge a different amount.

### 10.2 Action integration

Every U8 action has tests for its action name, pool, skill, physical flag,
configured base and payment policy. Physical cost responds to load; mental and
social cost does not. Higher governing skill reduces price through the existing
curve.

Full refusal preserves cooldown, ammunition, thrown item, concealment state,
buffs, grapple state and combat round as applicable. Valid misses and fumbles
still pay. Where both player and NPC entry points exist, they charge exactly
once through the same action path. Aliases charge only through their delegate.

Focused regressions cover shoot-plus-reload, throw preservation, sneak
transition ordering, rally/warcry buffs, grapple initiation and maintenance,
and mutation actives avoiding a second charge.

### 10.3 Combat invariants

- exhausted autoattacks still throw the planned swings without skill;
- every defence remains eligible regardless of affordability;
- only the winner pays and only a short winner messages;
- quell/defy charge Conviction;
- exhausted flee still resolves without Skullduggery;
- either grappler can independently lose its skill term;
- contest floors, crits, fumbles, mitigation and damage stay unchanged; and
- no new production caller bypasses `combat.RunContest`.

### 10.4 Completion gates

- `gofmt -l internal/ modules/` prints nothing;
- build and touched-package/full relevant tests pass;
- config validation and the balance model pass;
- isolated boot reaches `Server Ready` with no runtime panic;
- package context audit finds no stale symbols or behavior;
- affected help text and patch notes are current; and
- adversarial in-game playtest covers every action family at affordable,
  exhausted and recovered states, with special attention to repeated messages.

---

## 11. Explicitly out of scope and follow-up

- Surprise attack remains a U10 design slice.
- U9 owns progression events and primary-stat unification.
- Spell casting cost retuning is separate.
- Grapple transitions, outcome bands and effectiveness curves are unchanged.
- Mutation actives that remain live keep their existing costs.

**Recorded follow-up: mutation-active command dead-code audit.** Inventory the
current command registry, mutation definitions, implementations, tests,
helpfiles and config knobs after the mutation removals. Delete only entries
proven unreachable or obsolete. This is intentionally outside U8 so uncertainty
in that older surface cannot delay the unified action-cost slice.

---

## 12. Done when

1. Every action in section 3 is registered and priced through the U7 formula.
2. One quote/commit contract owns affordability, fractional carry and policy.
3. Voluntary refusal and life-preserving partial payment match the matrix.
4. Autoattack, defence, flee and grapple maintenance omit skill exactly when
   short while staying inside `combat.RunContest`.
5. No failed admission consumes secondary state and no successful attempt is
   free merely because it missed.
6. Player and NPC paths are mechanically identical.
7. Model evidence supports every shipped base.
8. Relevant help, `context.md`, config comments, roadmap and patch notes are
   current.
9. The adversarial playtest gate passes.
