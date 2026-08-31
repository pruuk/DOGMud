# U5b-2: The Named Behaviour Changes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Route the last eight direct pool-mutation sites onto the U5a primitives, removing the seven retained health floors and the defence affordability gate, so that an exhausted actor still participates in combat and health consistently stores overkill.

**Architecture:** U5b-1 routed every site that could move without changing behaviour. U5b-2 moves the remainder, each of which *does* change behaviour. Every one is already named in `pool_mutation_guard_test.go` with a `"U5b-2 ..."` exemption reason; this chunk removes each exemption as it routes the site, so the guard turning green is the completion signal. Standing rule 1 (provable no-op) was explicitly released for U5 by the user on 2026-08-13, so behaviour changes are in scope, but **only at the named sites below**, each called out individually in the PR body.

**Tech Stack:** Go 1.25, `internal/characters` pool primitives (`ApplyCost`, `ApplyCostPartial`, `CanAfford`, `ApplyHarm`, `ApplyRestore`), `internal/configs` balance knobs, root `pool_mutation_guard_test.go` AST guard.

> **This plan was adversarially reviewed on 2026-08-13 by three independent reviewers** (behavioural-equivalence, plan-executability, scope/design) and amended. Corrections that overturned a stated fact are marked **[CORRECTED]** inline so a reader who saw the first draft is not misled. Where two reviewers disagreed, the code was read and the winner is recorded.

---

## Verified state before starting (re-verified against master `9b9775e40`, 2026-08-13, post-review)

| Site | File:line | Current behaviour | Change |
|---|---|---|---|
| 1a | `internal/actions/surprise_attack.go:299-305` | `if targetChar.Health < 0 { = 0 }` | delete floor |
| 1b | `internal/combat/skill_moves.go:98-104` | `if p.Defender.Health < 0 { = 0 }` | delete floor |
| 1c | `internal/hooks/combat_shared_helpers.go:238-244` | riposte, `if attacker.Health < 0 { = 0 }` | delete floor |
| 1d | `internal/hooks/NewRound_AutoHeal.go:380-386` | mob poison DoT, `if Health < 1 { = 0 }` | delete floor |
| 1e | `internal/hooks/NewRound_AutoHeal.go:398-404` | mob bleed DoT, `if Health < 1 { = 0 }` | delete floor |
| 1f | `internal/usercommands/throw.go:173-179` | grenade self-fumble, `if Health < 0 { = 0 }` | delete floor |
| 1g | `internal/usercommands/throw.go:225-231` | grenade on mob, `if Health < 0 { = 0 }` | delete floor |
| 2 | `internal/combat/combat_helpers.go:560-563` | unaffordable defence `continue`s out of the candidate set | delete the gate |
| 3 | `internal/characters/resources.go:143-154` | `DeductDefenseStamina` full-or-refuse | `ApplyCostPartial`, then delete the function |
| 4 | `internal/mobcommands/cast.go:116` | `mob.Character.Conviction -= firstRoundCost`, **no affordability check anywhere** | add a guard above the cast prose |
| 5a | `internal/usercommands/flee.go:32-46` | hardcoded `10`, `DeductStamina` refuses | new config knob + `ApplyCostPartial` |
| 5b | `internal/usercommands/stand.go:47-69` | `StandMinStamina` gates, `StandStaminaCost` charges, manual clamp | `CanAfford` + `ApplyCostPartial` |
| 6 | `internal/hooks/combat_shared_helpers.go:567-579` | fold-cast upkeep, `char.Conviction -= roundCost` | `ApplyCost` (refusal preserved) |
| 7 | `internal/actions/mutation_helpers.go:107,116` | special-move preamble gate + charge | `ApplyCost` with the cooldown rollback preserved |

**Site 7 is not in the arc memory note's "six changes" list.** The guard carries it as `"U5b-2: cooldown-rollback ordering"`. Pure routing (special moves stay in the refuse column), so it goes first as a warm-up.

**Line numbers are anchors, not addresses.** Every step below also quotes the code verbatim. If a line number has drifted, match on the quoted text; do not trust the number.

**Verified pool-write census.** Every U5b-2-exempt file's *only* direct pool writes are the sites above, so each exemption deletes outright. The one survivor is `internal/characters/resources.go`, which still holds `Heal`'s two writes (`:195`, `:197`); that exemption stays and only its **reason string** narrows to U5c.

---

## What is actually changing, stated precisely

### Site 2: today's exhausted defender is NOT auto-hit **[CORRECTED]**

The first draft of this plan claimed an exhausted defender "takes an unopposed auto-hit". **That is false**, and the false claim had propagated into the commit message, the PR body, `context.md` and the player-facing patch notes.

What actually happens today: with every defence unaffordable, the entry list is empty, `contest.Run` reports `Contested=false`, and `best.margin` is set to `math.Inf(-1)`. In `resolveDefenseOutcomeCore`, `if best.margin > 0` (`combat_helpers.go:925`) is therefore false, so control falls through to the last-resort branch at `:939-952`, where `MinDefenseChance` — **shipped 0.15** (`_datafiles/config.yaml:465`) — gives a flat 15% save. `defType == ""` there, so the message always says "dodge" regardless of what the character can actually do. There is even a regression test guarding this exact path, whose comment reads *"guards against Stage 37.4 bug where defense floor was bypassed when defender had 0 stamina."*

The true before/after:

| | Exhausted defender |
|---|---|
| **Today** | dropped from the contest; flat 15% floor save; **no defence-crit possible** (`crit_floor.go:114` returns early when `defenseType == ""`); the save is always narrated as a dodge |
| **After U5b-2** | rolls a real contest, can win on merit, can defence-crit, and the correct defence type is named |

**It also bites in a narrow band.** Shipped defence costs are dodge `int(2×0.9)` = **1**, parry `int(4×0.9)` = **3**, block `int(5×0.9)` = **4**. So the gate only excluded a defence at 0-3 stamina. "Biggest live-feel change in the arc" is still defensible, but state it as *at 0-3 stamina*.

### The exhaustion gap: deliberate, disclosed, and NOT to be tuned against

Cost spec §3.4 treats gate-removal and skill-term-stripping as **one** change: *"An unaffordable defence is NOT skipped... It must instead roll without its skill term."* This chunk ships the first half only; the second is U8.

`GetDefenseScore` (`internal/characters/combat.go:310-341`) has **no resource term** — it is Dex + skill + gear, plus positional and condition multipliers. `ResourceMultiplier`'s callers are all attack-side (`calcSwingCount`, `buildDamageParams`, `calcAttackScore`, `ExecuteTaunt`, `calcSpellDamageForCharacter`). So between U5b-2 and U8:

> **A defender at 0 stamina defends exactly as well as a full-stamina defender, for free.**

That is *more generous than either endpoint*. The user's decision (2026-08-13) is to **ship it and disclose it loudly** rather than defer Task 8 or front-load U8's penalty. This must appear in the PR body **and** in the playtest brief, because Task 15 will otherwise calibrate "how does exhaustion feel" against a state the arc intends to remove.

### The prone/stand death spiral does not exist **[CORRECTED]**

The first draft, the U5a plan (`2026-08-13-u5a-cost-harm-foundation.md:1177-1182`) and the arc memory note all claim a 300-max character knocked prone is stuck ~45 rounds, unable to stand, with flee as the only exit. **All three are wrong.** Verified in code:

- **`AttemptRecovery` (`internal/characters/skills.go:47-99`) is a FREE roll**, costing no stamina, called unconditionally for every prone or supine player at `internal/hooks/NewRound_UserRoundTick.go:134` (and for mobs at `NewRound_MobRoundTick.go:169`). Chance is `25 + 20·ln(dex/25)`, capped 90 — at DEX 100 that is **~53% per round**.
- It gates only on `MinRecoveryRounds`, which every production knockdown site sets to **1 or 2** (`combat/grapple.go:214`, `combat/skill_moves.go:123,128`, `combat/submission_outcome.go:121`, `hooks/spell_resolution.go:522,1418`). Expected time prone at baseline is **~4 rounds**.
- The 45-round figure was additionally 3× wrong on its own terms: all stamina regen lives in `AutoHeal`, gated `if evt.RoundNumber%3 != 0 { return }` (`NewRound_AutoHeal.go:33-35`).
- Prone is mild at shipped values: `ProneDodgePenalty: 0.93`, `ProneParryPenalty: 0.93`, `ProneBlockPenalty: 0.95`, `ProneVulnerabilityMultiplier: 1.05` (config.yaml:544-555, the last annotated *"Tuned 39.2: lowered from 1.15"*).

**Flee stays Partial**, but on the correct justification: `go.go:73-86` refuses all movement while in combat, so **flee is the only player-initiated disengage**, and refusing it at 0 stamina leaves no alternative action that changes the situation. Do not write the spiral claim into any comment, commit, doc or patch note.

One related fact worth a line in the PR: flee succeeds via a direct `rooms.MoveToRoom` (`NewRound_DoCombat_helpers.go:563-585`) that bypasses `go.go`'s in-combat refusal, but never transitions to standing — so a fleeing character **arrives in the new room still prone**.

### Removing the health floors is observable, at more sites than first stated **[CORRECTED]**

Death is unaffected: all 57 health comparisons in `internal/` and `modules/` are `< 1`, `<= 0`, `> 0` or `>= 1` — **none is `== 0`** — so a negative value passes every gate.

The first draft claimed site 1f (grenade self-fumble) was the only newly player-visible case. It is not. `SendText` queues a message and message senders fire `RedrawPrompt` **without** `OnlyIfChanged` (`internal/hooks/RedrawPrompt_SendRedraw.go:38`), so site 1b (reached from eleven `internal/actions/combat_*.go` entry points, any of which can have a player defender) and site 1a also re-render a player's prompt. And sites 1d/1e/1g unfloor **mob** health, which ships raw to clients.

Task 9 therefore clamps at **every** display surface, not just the prompt.

## The insufficient-resource rule (user, 2026-08-13)

An exhausted actor still **acts**, losing the skill term. Death from exhaustion was tried in this game and players hated it. The line is **whether a meaningful alternative action remains** — not volition, and not "uses a cooldown"; both framings were tried and are provably false.

| Partial charge (`ApplyCostPartial`) | Refuse (`ApplyCost`) |
|---|---|
| auto-attack, dodge/parry/block, grapple upkeep, **flee** | movement, **stand**, **spellcasting**, **mutation special moves**, taunt/rally/warcry, special moves |

Applying the skill-term penalty is **U8**. `CostResult.Short` exists so U8 has something to read; U5b-2 discards it at every site.

## Explicitly OUT of scope

- **U8's skill-term penalty.** See the exhaustion gap above. Disclosed, not fixed here.
- **The player/mob cast asymmetry.** The player path (`skill.cast.go:126`) gates on the *full* cost then pays **zero** up front; the mob path pays a first-round slice. Task 4 gives the mob path the guard it never had but does **not** unify them. Note the direction: today mob = *no floor at all*, after this chunk mob = *must hold ≥ Cost/Folds*, player = *must hold the full cost*. The mob becomes **stricter than today** and still laxer than a player, so this chunk narrows the asymmetry rather than widening it. Unification is a U7 cost-model decision.
- **`Heal()`'s three production callers** — `internal/actions/combat_drain.go:126`, `:281`, `internal/hooks/item_procs.go:99`. They are real pool mutations routed past the primitives through a file-exempt helper, so the guard cannot see them. They die with `Heal` in **U5c**. Say so in the PR: a green guard means *every U5b-2 site is routed*, not *every pool write is*.
- **`actions/search.go` ×6, `actions/track.go:121`, `forager/forage_core.go:126`** — flat `dice.RollStat(x); if roll.Value >= 135.0`. Still unassigned to any chunk after surviving U4 and U5.
- **Buff applier attribution.** `buffs.Buff` has no applier field, so DoT deaths stay anonymous. U5c inherits this.
- **`contest.AgainstDifficulty`** — zero production callers, guarded-and-unused.

---

## File Structure

**Modified:** `internal/configs/config.balance.go`, `config.balance.combat.go`, `_datafiles/config.yaml`, `internal/actions/mutation_helpers.go`, `internal/hooks/combat_shared_helpers.go`, `internal/mobcommands/cast.go`, `internal/usercommands/stand.go`, `internal/usercommands/flee.go`, `internal/characters/resources.go`, `internal/characters/pools.go`, `internal/combat/combat_helpers.go`, `internal/combat/regression_test.go`, `internal/actions/surprise_attack.go`, `internal/combat/skill_moves.go`, `internal/hooks/NewRound_AutoHeal.go`, `internal/usercommands/throw.go`, `internal/users/userrecord.prompt.go`, `internal/templates/templatesfunctions.go`, `modules/gmcp/gmcp.Char.go`, `modules/playtest/beacons.go`, `pool_mutation_guard_test.go`, `internal/characters/context.md`, `internal/combat/context.md`, `docs/PATCH_NOTES.md`, `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`

**Created:** `internal/configs/config.balance.flee_test.go`, `internal/combat/defense_affordability_test.go`, `internal/usercommands/flee_cost_test.go`, `internal/hooks/health_overkill_test.go`, `internal/characters/display_health_test.go`, `tools/playtest/goals/2026-08-13-u5b2-exhaustion.yaml`

---

### Task 0: Branch and baseline

- [ ] **Step 1: Confirm the base commit**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git rev-parse --short HEAD
```

Expected: `9b9775e40`. If it differs, re-verify the site table before continuing.

- [ ] **Step 2: Learn the guard's REAL test name**

```bash
grep -n "^func Test" pool_mutation_guard_test.go
```

Expected: `TestPoolMutationGoesThroughThePrimitives` at line 156.

> **[CORRECTED]** The first draft of this plan invoked `TestNoDirectPoolWrites`, which **does not exist**. `go test -run` with a non-matching pattern exits **0** and prints `ok ... [no tests to run]`, so every guard checkpoint would have silently passed without running the guard. Throughout this plan the guard command is:
> ```bash
> go test . -run TestPoolMutationGoesThroughThePrimitives -v 2>&1 | tail -5
> ```

- [ ] **Step 3: Run it now to record the baseline**

Expected: PASS. The guard is green today because every U5b-2 site is exempted. It must still be PASS at the end, with the exemptions gone.

- [ ] **Step 4: Preserve the local `config.yaml` overrides before touching skip-worktree**

**[CORRECTED]** `_datafiles/config.yaml` has `skip-worktree` set **and its working copy differs from `HEAD` in four unrelated hunks**: `HttpPort: 8090` (vs 80), `LogToFile: true` (vs false), `LogLevel: "info"` (vs "debug"), and an extra `Playtest:` block. Committing the file as-is would leak all four to master — precisely the 2026-08-11 incident. Task 1 handles this with a checkout-then-restore dance; back the file up first.

```bash
cp _datafiles/config.yaml /c/tmp/config.local.bak
git update-index --no-skip-worktree _datafiles/config.yaml
git ls-files -v _datafiles/config.yaml
```

Expected: the line now starts with `H`, not `S`. Verify the backup is real before continuing:

```bash
grep -c "HttpPort: 8090" /c/tmp/config.local.bak    # want 1
```

- [ ] **Step 5: Create the branch**

```bash
git checkout -b feature/u5b2-named-behaviour-changes
```

**⚠️ `git add -A` is a trap in this working tree.** Four items live here permanently: `_datafiles/world/dogmud/rooms/thornwall_city/473.yaml` (modified), `ADVERSARIAL_CODE_REVIEW_2026-08-07.md`, `tools/playtest/goals/scenarios/harness-sanity-duo/`, `tools/playtest/scenarios/harness-sanity-duo.yaml` (untracked). **Every commit stages explicit paths.** `git add -p` is interactive and unusable here.

---

### Task 1: Add the `FleeStaminaCost` balance knob

Standing rule 2: no balance number inside `internal/`. `flee.go` hardcodes `10`.

**Files:** `internal/configs/config.balance.go`, `internal/configs/config.balance.combat.go`, `_datafiles/config.yaml`, create `internal/configs/config.balance.flee_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/configs/config.balance.flee_test.go`:

```go
package configs

import "testing"

// The flee stamina cost was a hardcoded 10 in usercommands/flee.go until U5b-2.
// Standing rule 2 of the unified-resolution arc: no balance number lives inside
// internal/. Pin the default so a future edit to flee.go cannot quietly
// reintroduce a literal.
func TestFleeStaminaCost_DefaultsToTheOldHardcodedValue(t *testing.T) {
	b := Balance{}
	b.Validate()
	if int(b.FleeStaminaCost) != 10 {
		t.Fatalf("FleeStaminaCost default = %v, want 10 (the value flee.go hardcoded pre-U5b-2)", int(b.FleeStaminaCost))
	}
}

// A zero or negative cost would make flee free, which is a balance decision
// nobody made. Validation must reject it back to the default.
func TestFleeStaminaCost_RejectsNonPositive(t *testing.T) {
	b := Balance{FleeStaminaCost: -5}
	b.Validate()
	if int(b.FleeStaminaCost) != 10 {
		t.Fatalf("FleeStaminaCost after validating -5 = %v, want 10", int(b.FleeStaminaCost))
	}
}
```

`Balance.Validate()` is exported at `config.balance.go:740` and fans out to `validateCombat()` (`config.balance.combat.go:6`), where step 3 adds the default. `ConfigInt` is `int`. `Balance{}` is addressable, so the pointer receiver is fine.

- [ ] **Step 2: Run it**

```bash
go test ./internal/configs/ -run TestFleeStaminaCost -v
```

Expected: FAIL to **compile**, `b.FleeStaminaCost undefined`.

- [ ] **Step 3: Add the field, default, and validation**

In `internal/configs/config.balance.go`, after the `FlightFleeStaminaMult` line (`:149`):

```go
	FleeStaminaCost         ConfigInt   `yaml:"FleeStaminaCost"`         // Base stamina charged for breaking off to flee (default 10). Charged PARTIALLY: an exhausted character still gets to flee. U7 may fold this into NonHarmContestBaseCost.
```

In `internal/configs/config.balance.combat.go`, after the `FlightFleeStaminaMult` block (`:88-90`):

```go
	if b.FleeStaminaCost <= 0 {
		b.FleeStaminaCost = 10
	}
```

- [ ] **Step 4: Add the shipped value to config.yaml**

**[CORRECTED]** The first draft said to anchor next to `FlightFleeStaminaMult` — **that key does not appear in `config.yaml` at all** (it runs on its Go default). Anchor on the nearest shipped stamina knobs instead: `StandStaminaCost` (`:569`) / `StandMinStamina` (`:572`).

Add immediately after the `StandMinStamina` line:

```yaml
  # FleeStaminaCost: base stamina charged for breaking off to flee. Charged
  # partially, never refused: fleeing is the only player-initiated disengage
  # while in combat, so refusing it at low stamina would leave no alternative.
  FleeStaminaCost: 10
```

Ship it at the old hardcoded value so this task alone is a provable no-op.

- [ ] **Step 5: Run the test**

```bash
go test ./internal/configs/ -run TestFleeStaminaCost -v
```

Expected: PASS, both cases.

- [ ] **Step 6: Commit the config change WITHOUT the four local overrides**

**[CORRECTED]** This is the step that would otherwise leak `HttpPort`, `LogToFile`, `LogLevel` and the `Playtest:` block to master.

```bash
# Save the current state (pristine base + your new knob is what we want to commit)
cp _datafiles/config.yaml /c/tmp/config.withknob.bak

# Reset to the committed version, then re-add ONLY the new knob
git checkout HEAD -- _datafiles/config.yaml
```

Now re-add just the `FleeStaminaCost` block from Step 4 to the freshly-checked-out file, then:

```bash
git add internal/configs/config.balance.go internal/configs/config.balance.combat.go \
        internal/configs/config.balance.flee_test.go _datafiles/config.yaml
git diff --cached -- _datafiles/config.yaml
```

**Expected: exactly four added lines (the comment and the knob) and nothing else.** If `HttpPort`, `LogToFile`, `LogLevel` or `Playtest` appears in that diff, stop and redo this step.

```bash
git commit -m "$(cat <<'EOF'
feat(config): add FleeStaminaCost, replacing flee.go's hardcoded 10 (U5b-2)

Standing rule 2 of the unified-resolution arc: no balance number lives inside
internal/. flee.go has carried a literal 10 since it was written. Ships at 10 so
this commit alone changes nothing; task 6 routes the call site.

Flagged for U7: the cost spec reserves NonHarmContestBaseCost for "flee, sneak
and similar", so this is knowingly a two-chunk knob.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 7: Restore the local overrides on top**

```bash
cp /c/tmp/config.local.bak _datafiles/config.yaml
```

Then re-add the `FleeStaminaCost` block to the restored file (it is now committed but the restored local copy predates it). Confirm:

```bash
grep -c "FleeStaminaCost" _datafiles/config.yaml   # want 1
grep -c "HttpPort: 8090" _datafiles/config.yaml    # want 1 (local override preserved)
git status --short _datafiles/config.yaml          # will show ' M' until Task 12 re-sets skip-worktree
```

---

### Task 2: Route the mutation special-move preamble (site 7)

Special moves are in the **refuse** column, so this is pure routing. The subtlety the guard's exemption names is ordering: the cooldown is consumed by `Cooldowns.Try` *before* the affordability check and must be rolled back if the actor cannot pay.

**Files:** `internal/actions/mutation_helpers.go`, `pool_mutation_guard_test.go`

- [ ] **Step 1: Read the current code**

```bash
sed -n '95,120p' internal/actions/mutation_helpers.go
```

Confirm Gate 3 (`Cooldowns.Try`) precedes Gate 4 (`if char.Stamina < staminaCost`), and that `char.Stamina -= staminaCost` follows at `:116`.

- [ ] **Step 2: Replace the gate-then-charge pair with one `ApplyCost`**

```go
	// Gate 4: stamina cost. Special moves REFUSE when unaffordable -- the actor
	// keeps a meaningful alternative (they can still auto-attack), so declining
	// costs them a beat rather than their participation. ApplyCost pays in full
	// or takes nothing, which is why the cooldown rollback below is still
	// correct: a refused attempt has spent no stamina either.
	if !char.ApplyCost(characters.PoolStamina, staminaCost) {
		// Cooldown was consumed by the Try call above; roll it back so the
		// actor isn't punished with a cooldown for a failed attempt.
		delete(char.Cooldowns, "special-move")
		if actor.IsPlayer() {
			actor.SendText(messaging.CategorySystem, "You're too exhausted!")
		}
		return preambleResult{BlockReason: "low-stamina"}
	}
```

Delete the now-dead `char.Stamina -= staminaCost` line that followed.

Callers pass literals 8 and 10 (`mutation_venom_coat.go:13`, `mutation_cocoon.go:15`), both positive, so `ApplyCost` is exactly equivalent to the old pair. (Those two literals are themselves standing-rule-2 violations left alone by this chunk — note it in the PR rather than silently passing over it.)

- [ ] **Step 3: Add the `characters` import**

`internal/actions/mutation_helpers.go` does **not** currently import it. Add `"github.com/GoMudEngine/GoMud/internal/characters"`.

- [ ] **Step 4: Remove the guard exemption**

Delete `pool_mutation_guard_test.go:95`:

```go
	"internal/actions/mutation_helpers.go": "U5b-2: cooldown-rollback ordering",
```

- [ ] **Step 5: Build, test, verify the guard**

```bash
go build ./... && go test ./internal/actions/ && go test . -run TestPoolMutationGoesThroughThePrimitives -v 2>&1 | tail -5
```

Expected: all PASS. A guard failure naming `mutation_helpers.go` means a write was missed — find it, do not re-add the exemption.

- [ ] **Step 6: Commit**

```bash
git add internal/actions/mutation_helpers.go pool_mutation_guard_test.go
git commit -m "$(cat <<'EOF'
refactor(actions): route the special-move preamble onto ApplyCost (U5b-2)

Special moves stay in the refuse column, so this is behaviour-neutral. ApplyCost
refuses without charging, which collapses the separate read and subtraction into
one call and leaves the cooldown rollback correct: a refused attempt has spent
no stamina.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Route the fold-cast upkeep cost (site 6)

Spellcasting is in the **refuse** column and this site already refuses. Behaviour-neutral routing.

**Files:** `internal/hooks/combat_shared_helpers.go`

- [ ] **Step 1: Read the current code**

```bash
sed -n '564,582p' internal/hooks/combat_shared_helpers.go
```

Confirm the gate is `if roundCost > 0 && char.Conviction < roundCost {` (`:567`) and the charge is `char.Conviction -= roundCost` (`:579`).

- [ ] **Step 2: Replace with `ApplyCost`**

```go
	// Upkeep REFUSES when unaffordable: the cast collapses. ApplyCost pays in
	// full or takes nothing, so a broken concentration never also drains the
	// pool. The roundCost > 0 guard is now redundant -- ApplyCost treats a
	// non-positive amount as free and succeeds -- but it is kept so a zero-cost
	// fold cannot be read as a refusal path.
	//
	// Categorisation note: the refuse column's usual rationale ("the actor keeps
	// every other action") does NOT hold here. handlePlayerFoldCasting returns
	// true on every branch, so a caster who cannot pay loses the conviction
	// already sunk into prior folds, the spell, AND their round. That shape is
	// inherited, not introduced here; filed for U7/U8.
	if roundCost > 0 && !char.ApplyCost(characters.PoolConviction, roundCost) {
		clearCastingActivity(char, activity.TriggerConcentrationBreak)
		return FoldRoundResult{
			InsufficientConviction: true,
			FoldDelta:              foldDelta,
			ConvictionCost:         roundCost,
			SpellData:              spellData,
			CastingData:            cs,
		}
	}

	// Conviction was charged by the ApplyCost above. Advance folds via the
	// Activity machine.
```

Delete the `char.Conviction -= roundCost` line.

**Verified safe:** `calcFoldConvictionCost` (`:369-378`) returns `0` or `>= 1`, never negative, so the short-circuit is exactly equivalent to the old unconditional `-=`. Nothing between the old gate (`:567`) and the old subtraction (`:579`) reads `char.Conviction`.

- [ ] **Step 3: Confirm the charge moved and nothing else reads the pool**

```bash
sed -n '564,585p' internal/hooks/combat_shared_helpers.go | grep -n "Conviction"
```

**[CORRECTED]** Expected output is three lines: the `ApplyCost` call, `InsufficientConviction: true,` and `ConvictionCost: roundCost,`. (The first draft wrongly expected `AdvanceCastingFolds(foldDelta, roundCost)` to match — it contains no `Conviction` substring.) There must be **no** bare `char.Conviction` read or write.

- [ ] **Step 4: Build and test**

```bash
go build ./... && go test ./internal/hooks/ -run "Fold|Cast|Concentration" -v 2>&1 | tail -20
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/hooks/combat_shared_helpers.go
git commit -m "$(cat <<'EOF'
refactor(hooks): fold-cast upkeep uses ApplyCost (U5b-2)

Spellcasting is in the refuse column and this site already refused, so the
change is behaviour-neutral. Charging inside the gate means a broken
concentration can never also drain the pool.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: Give the mob cast path the affordability guard it never had (site 4)

A mob can begin a cast at 0 conviction today and go negative. There is no affordability check anywhere on this path — `actions.InitiateCast`'s own docstring says the conviction pre-check is "intentionally left to the caller," and no caller does it.

**Three ordering constraints, all load-bearing:**
1. The charge must happen **before** `TransitionToCasting` (`cast.go:130`), or the FSM and the ledger diverge.
2. The guard must fire **before** the YAML cast-text block (`cast.go:83-109`), or a mob that cannot afford the spell still emits `spellInfo.CastRoomText` to the room. **[CORRECTED]** The first draft cited the string `"begins weaving a spell"` as what would leak — that literal is at `:142-143`, *after* the transition, so it was never the risk. The ordering requirement stands; the example was wrong.
3. `InitiateCast` has **already consumed the shared special-move cooldown** by the time the guard runs (see Step 2).

**Files:** `internal/mobcommands/cast.go`, `pool_mutation_guard_test.go`

- [ ] **Step 1: Read the structure**

```bash
sed -n '79,145p' internal/mobcommands/cast.go
```

Confirm: `spellInfo := result.SpellInfo` at **:81**, YAML cast-text block **:83-109**, cost block **:111-116**, `castData` **:119-129**, `TransitionToCasting` **:130**.

- [ ] **Step 2: Confirm the cooldown hazard, then read `InitiateCast` for other side effects**

```bash
sed -n '262,275p' internal/actions/cast.go
```

**[CORRECTED — this was missed in the first draft.]** Expected: `InitiateCast` calls `char.TryCooldown("special-move", ...)` and returns `OnCooldown: true` on failure, all gated on `if !spellInfo.IgnoreMoveCooldown`. So by the time the new guard runs, the mob has already burned the slot that `bash`/`kick`/`trip` share — which directly undercuts the "the behaviour tree picks another action next tick" justification. The refusal branch must roll it back, exactly as Task 2 does.

Then read the rest of `InitiateCast` and confirm nothing else it does before returning `Initiated: true` needs unwinding (a spell cooldown, a reserved CastingState slot, a target lock). Record what you find in the commit message.

- [ ] **Step 3: Insert the guard immediately after `spellInfo := result.SpellInfo` (`:81`)**

```go
	// First-round conviction slice -- the mob pays a portion up front.
	//
	// U5b-2: this path had NO affordability check of any kind, so a mob could
	// begin a cast at zero conviction and drive the pool negative. Spellcasting
	// is in the refuse column, so ApplyCost is correct.
	//
	// Placement is load-bearing on both sides. It is ABOVE the YAML cast-text
	// block so a mob that cannot afford the spell never narrates it to the room,
	// and it is ABOVE TransitionToCasting so the ledger cannot lead the FSM.
	//
	// InitiateCast has already consumed the shared special-move cooldown (see
	// actions/cast.go, gated on !IgnoreMoveCooldown), so a refusal here must roll
	// it back or a broke mob silently blocks bash/kick/trip for the cooldown
	// duration.
	//
	// Note the asymmetry with the player path, which is NOT unified here:
	// usercommands/skill.cast.go:126 gates on the FULL cost and then pays zero
	// up front. Reconciling the two is a U7 cost-model decision. This chunk
	// moves the mob from "no floor at all" to "must hold at least one slice",
	// which narrows the gap rather than widening it.
	firstRoundCost := spellInfo.Cost / result.FoldsNeeded
	if firstRoundCost < 1 {
		firstRoundCost = 1
	}
	if !mob.Character.ApplyCost(characters.PoolConviction, firstRoundCost) {
		if !spellInfo.IgnoreMoveCooldown {
			delete(mob.Character.Cooldowns, "special-move")
		}
		mudlog.Debug("mob.Cast",
			"mob", mob.Character.Name,
			"requested_spell", spellName,
			"reason", "insufficient conviction")
		return true, nil
	}
```

- [ ] **Step 4: Delete the old cost block (`:111-116`)**

```go
	// First-round conviction slice — mob pays a portion up-front.
	firstRoundCost := spellInfo.Cost / result.FoldsNeeded
	if firstRoundCost < 1 {
		firstRoundCost = 1
	}
	mob.Character.Conviction -= firstRoundCost
```

`firstRoundCost` is still referenced by `castData.ConvictionSpent`; the new block defines it earlier in the same function scope.

- [ ] **Step 5: Refund on a failed transition**

`TransitionToCasting` can still fail. The charge now precedes it, so:

```go
	); err != nil {
		// Mob can't start cast — likely busy. Silent failure;
		// btree will pick another action next tick.
		//
		// U5b-2: the first-round slice was charged above (it has to be, so the
		// ledger cannot lead the FSM). A failed transition means no cast
		// happened, so give it back.
		mob.Character.ApplyRestore(characters.PoolConviction, firstRoundCost)
		return true, nil
	}
```

**Verified:** this error branch is the only early return between the new charge point and the transition, so one refund is sufficient.

- [ ] **Step 6: Confirm imports**

`characters` is already imported (`cast.go:8`); `mudlog` already imported (`:11`). No change expected — confirm with `go build`.

- [ ] **Step 7: Remove the guard exemption**

Delete `pool_mutation_guard_test.go:94`:

```go
	"internal/mobcommands/cast.go":         "U5b-2: mob cast gains a guard",
```

- [ ] **Step 8: Build and verify**

```bash
go build ./... && go test ./internal/mobcommands/ && go test . -run TestPoolMutationGoesThroughThePrimitives -v 2>&1 | tail -5
```

Expected: all PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/mobcommands/cast.go pool_mutation_guard_test.go
git commit -m "$(cat <<'EOF'
fix(mobcommands): guard the mob cast conviction debit (U5b-2)

This path had no affordability check of any kind -- InitiateCast's docstring
says the conviction pre-check is "intentionally left to the caller" and no
caller did it -- so a mob could begin a cast at zero conviction and drive the
pool negative.

Placement is load-bearing on three sides: above the YAML cast text so an
unaffordable spell is never narrated, above TransitionToCasting so the ledger
cannot lead the FSM, and rolling back the shared special-move cooldown that
InitiateCast has already consumed by this point (otherwise a broke mob silently
blocks bash/kick/trip). The transition can still fail, so that branch refunds.

The player path's opposite asymmetry (gate on the full cost, pay zero up front)
is deliberately left alone; unifying the two is a U7 decision. This change moves
the mob from "no floor at all" to "must hold at least one slice", narrowing the
gap rather than widening it.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: Route `stand`'s two-knob gate (site 5b)

`stand` is in the **refuse** column. Two distinct knobs, deliberately not collapsed: `StandMinStamina` **gates**, `StandStaminaCost` **charges**, both fractions of max, both shipping 0.15 (`config.yaml:569,572`).

**Files:** `internal/usercommands/stand.go`, `pool_mutation_guard_test.go`

- [ ] **Step 1: Note the ordering hazard**

**[CORRECTED]** The first draft argued that if a retune made `StandStaminaCost > StandMinStamina`, refusing was "the safer of the two". **It is the worse of the two.** The FSM transition fires at `:59`, *before* the charge at `:66`, so a refusal there means the player **stands for free** — an exploit shape. Use `ApplyCostPartial` for the charge: the `CanAfford` gate still expresses the requirement, and at shipped values (where the two knobs are equal) it is a bit-for-bit no-op.

- [ ] **Step 2: Replace the gate**

```go
	// Check if player has enough stamina.
	//
	// U5b-2: stand REFUSES, and it is deliberately not a single ApplyCost.
	// StandMinStamina gates and StandStaminaCost charges; they are independent
	// knobs that merely happen to ship at the same fraction today. Collapsing
	// them into one call would weld a tuning coincidence into code.
	if !user.Character.CanAfford(characters.PoolStamina, minStamina) {
		needed := minStamina - user.Character.Stamina
		user.SendText(messaging.CategorySystem, fmt.Sprintf("You're too exhausted to stand! (need %d more stamina)", needed))
		return true, nil
	}
```

- [ ] **Step 3: Replace the charge (`:65-69`)**

```go
	// Charge stamina. Partial, not full-or-refuse: the FSM transition above has
	// already fired, so a refusal here would let the player stand for FREE. That
	// is unreachable at shipped values (both knobs are 0.15, so the amounts are
	// identical), but it is the correct shape if they are ever tuned apart.
	// ApplyCostPartial also makes the old manual sub-zero clamp redundant.
	_ = user.Character.ApplyCostPartial(characters.PoolStamina, staminaCost)
```

Delete:

```go
	user.Character.Stamina -= staminaCost
	if user.Character.Stamina < 0 {
		user.Character.Stamina = 0
	}
```

- [ ] **Step 4: Add the `characters` import**

`internal/usercommands/stand.go` does not currently import it. Add it.

- [ ] **Step 5: Note the raw-number message for follow-up**

`"You're too exhausted to stand! (need %d more stamina)"` leaks a raw number, against the project's player-copy rule. **Leave it** — changing player copy is not this chunk's scope — but list it in the PR body as a known, deliberately deferred item so it is not mistaken for an oversight.

- [ ] **Step 6: Remove the guard exemption**

Delete `pool_mutation_guard_test.go:93`:

```go
	"internal/usercommands/stand.go":       "U5b-2: stand's two-knob gate",
```

- [ ] **Step 7: Build and verify**

```bash
go build ./... && go test ./internal/usercommands/ 2>&1 | tail -10 && go test . -run TestPoolMutationGoesThroughThePrimitives -v 2>&1 | tail -5
```

Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/usercommands/stand.go pool_mutation_guard_test.go
git commit -m "$(cat <<'EOF'
refactor(usercommands): route stand onto CanAfford plus ApplyCostPartial (U5b-2)

Deliberately two calls, not one. StandMinStamina gates and StandStaminaCost
charges; they are independent knobs that merely happen to ship at the same
fraction, and collapsing them would weld that coincidence into code.

The charge is PARTIAL rather than full-or-refuse because the FSM transition
fires before it: a refusal would let the player stand for free. Unreachable at
shipped values, correct at any retune. The manual sub-zero clamp goes away
because the primitive cannot drive a pool below zero.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: Flee charges partially instead of refusing (site 5a)

**This is a behaviour change.** Flee is in the **partial** column.

> **[CORRECTED] Do not use the prone/stand death-spiral justification.** It is false (see the header section): `AttemptRecovery` is a free roll firing every round at ~53% for a DEX 100 character. The correct justification is that **`go.go:73-86` refuses all movement while in combat, so flee is the only player-initiated disengage** — refusing it at 0 stamina leaves no alternative action that changes the situation.

The "You're too exhausted to flee!" message becomes unreachable and must be **deleted** (standing rule 4).

**Files:** `internal/usercommands/flee.go`, create `internal/usercommands/flee_cost_test.go`

- [ ] **Step 1: Confirm the skullduggery term exists**

```bash
grep -rn "fleeScore\|Skullduggery" internal/combat/flee*.go internal/usercommands/flee.go | cat
```

Expected: a `Dex + Skullduggery * 25`-shaped expression in `internal/combat/flee.go`. Confirmed present by two reviewers — U8 has something to strip.

- [ ] **Step 2: Write the contract test**

Create `internal/usercommands/flee_cost_test.go`:

```go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
)

// U5b-2: flee moved from refuse to partial charge. An exhausted character MUST
// still get to flee: go.go refuses all movement while in combat, so fleeing is
// the only player-initiated disengage. Refusing it at zero stamina would leave
// no alternative action that changes the character's situation.
func TestFleeCost_ExhaustedCharacterIsChargedPartiallyNotRefused(t *testing.T) {
	c := characters.New()
	c.StaminaMax.Base = 100
	c.StaminaMax.Recalculate()
	c.Stamina = 3

	cost := int(configs.GetBalanceConfig().FleeStaminaCost)
	if cost <= 3 {
		t.Fatalf("test fixture assumes FleeStaminaCost (%d) exceeds the 3 stamina on hand", cost)
	}

	res := c.ApplyCostPartial(characters.PoolStamina, cost)

	if res.Charged != 3 {
		t.Errorf("charged = %d, want 3 (everything that was there)", res.Charged)
	}
	if !res.Short {
		t.Error("Short = false, want true -- U8 reads this to strip the skill term")
	}
	if c.Stamina != 0 {
		t.Errorf("stamina after a partial flee charge = %d, want 0", c.Stamina)
	}
}
```

- [ ] **Step 3: Run it**

```bash
go test ./internal/usercommands/ -run TestFleeCost -v
```

Expected: PASS immediately. This pins the *primitive contract* Task 6 depends on; it is a guard, not a red-green cycle, because `ApplyCostPartial` already exists from U5a. `Flee()` itself takes a `*users.UserRecord` and a `*rooms.Room` and has no unit-testable seam — do not fabricate one. The behaviour change is verified by reading the code in Step 6 and by playtest in Task 15.

- [ ] **Step 4: Make the change (`flee.go:32-46`)**

Replace:

```go
	if !user.Character.IsDisengaging() {
		// Fleeing costs stamina — a flyer breaks away far more easily (Winged Flight).
		fleeStaminaCost := 10
		if mutations.IsFlying(user.Character.Mutations) {
			fleeStaminaCost = int(float64(fleeStaminaCost) * float64(configs.GetBalanceConfig().FlightFleeStaminaMult))
			if fleeStaminaCost < 1 {
				fleeStaminaCost = 1
			}
		}
		if !user.Character.DeductStamina(fleeStaminaCost) {
			user.SendText(messaging.CategorySystem, `You're too exhausted to flee! You need to stand and fight.`)
			return true, nil
		}

		user.SendText(messaging.CategorySystem, `You attempt to flee...`)
```

with:

```go
	if !user.Character.IsDisengaging() {
		// Fleeing costs stamina — a flyer breaks away far more easily (Winged Flight).
		bal := configs.GetBalanceConfig()
		fleeStaminaCost := int(bal.FleeStaminaCost)
		if mutations.IsFlying(user.Character.Mutations) {
			fleeStaminaCost = int(float64(fleeStaminaCost) * float64(bal.FlightFleeStaminaMult))
			if fleeStaminaCost < 1 {
				fleeStaminaCost = 1
			}
		}
		// U5b-2: flee charges what it can and NEVER refuses. go.go refuses all
		// movement while in combat, so fleeing is the only player-initiated
		// disengage; refusing it at zero stamina would leave no alternative
		// action that changes the character's situation.
		//
		// The old "You're too exhausted to flee!" message is deleted rather than
		// left unreachable (standing rule 4). U8 reads CostResult.Short to strip
		// the skill term from fleeScore's skullduggery contribution; this chunk
		// discards it.
		_ = user.Character.ApplyCostPartial(characters.PoolStamina, fleeStaminaCost)

		user.SendText(messaging.CategorySystem, `You attempt to flee...`)
```

- [ ] **Step 5: Fix imports**

Add `characters`. `configs` stays used. Confirm with `go build ./internal/usercommands/`.

- [ ] **Step 5b: Route the OTHER `DeductStamina` caller and delete the function**

> **[ADDED during execution, 2026-08-13.]** The first draft said `DeductStamina` "keeps its other caller, so it does not become dead" and left it at that. But its own docstring reads *"Deprecated: use ApplyCost. **U5b routes the remaining callers.**"* — so leaving it alive means the deprecated path survives the chunk that promised to remove it, against standing rule 4, and U6 inherits it. It has exactly two production callers: `flee.go:41` (removed in Step 4) and `go.go:182`.

Movement is in the **refuse** column and `DeductStamina` is already full-or-refuse, so this is exactly equivalent. In `internal/usercommands/go.go:182`:

```go
		if !user.Character.DeductStamina(staminaCost) {
```

becomes:

```go
		// U5b-2: movement REFUSES when unaffordable -- the character keeps every
		// other action, and this is the gate that makes flee the only
		// player-initiated disengage while in combat.
		if !user.Character.ApplyCost(characters.PoolStamina, staminaCost) {
```

Add the `characters` import to `go.go` if absent. Then delete `DeductStamina` from `internal/characters/resources.go:25-31` entirely, and delete its now-uncompilable test `TestCharacter_DeductStamina` (`internal/characters/character_test.go:2129-2180`) — the full-or-refuse contract it covered is already pinned by `TestApplyCost_RefusesAndTakesNothing` and `TestApplyCost_PaysInFullWhenAffordable` in `pools_test.go`.

Verify nothing references it:

```bash
grep -rn "DeductStamina" internal/ modules/ --include=*.go
```

Expected: **no output**.

- [ ] **Step 6: Confirm the message is gone everywhere**

```bash
grep -rn "too exhausted to flee" internal/ _datafiles/ --include=*.go --include=*.yaml --include=*.txt
```

Expected: **no output**. If a helpfile mentions it, update the helpfile in this commit.

- [ ] **Step 7: Build and test**

```bash
go build ./... && go test ./internal/usercommands/ ./internal/combat/ 2>&1 | tail -10
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/usercommands/flee.go internal/usercommands/flee_cost_test.go
git commit -m "$(cat <<'EOF'
feat(usercommands): flee charges partially and never refuses (U5b-2)

NAMED BEHAVIOUR CHANGE. go.go refuses all movement while in combat, so fleeing
is the only player-initiated disengage. Refusing it at zero stamina leaves the
character with no alternative action that changes their situation, which is what
the insufficient-resource rule exists to prevent.

The hardcoded 10 becomes FleeStaminaCost, shipped at 10. The now-unreachable
"You're too exhausted to flee!" message is deleted rather than left as dead
code.

Note for anyone reading the earlier U5a plan: its "prone/stand death spiral"
argument for this assignment is FALSE. AttemptRecovery is a free roll firing
every round at roughly 53% for a baseline character, so a prone character stands
in about four rounds without spending anything. The assignment is right; that
justification was not.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: `DeductDefenseStamina` becomes a partial charge, then dies (site 3)

**Files:** `internal/characters/resources.go`, `internal/combat/combat_helpers.go`, `pool_mutation_guard_test.go`

- [ ] **Step 1: Census the references**

```bash
grep -rn "DeductDefenseStamina" internal/ modules/ --include=*.go
```

**[CORRECTED]** Expected **five** lines, not two: `resources.go:143` (comment), `:147` (definition), `combat_helpers.go:665` (the sole call), and `pool_mutation_guard_test.go:81` + `:92` (comment + exemption string). There is **no** `TestDeductDefenseStamina`, so nothing needs deleting from the test suite.

- [ ] **Step 2: Replace the call site (`combat_helpers.go:663-666`)**

```go
	// Charge stamina only for the winning defence.
	//
	// U5b-2: partial, not full-or-refuse. With the affordability gate above
	// removed, an exhausted defender can now win the contest, so this call must
	// be able to charge what little is there rather than declining and leaving
	// the defence free. U8 reads CostResult.Short to strip the skill term from
	// the defence score; this chunk discards it.
	if best.defenseType != "" {
		_ = targetChar.ApplyCostPartial(characters.PoolStamina,
			targetChar.GetDefenseStaminaCost(best.defenseType))
	}
```

- [ ] **Step 3: Delete the function (`resources.go:143-154`)**

Remove the doc comment and the whole `DeductDefenseStamina` body. `GetDefenseStaminaCost` **stays** — the new call site uses it.

- [ ] **Step 4: Narrow the guard exemption reason (`pool_mutation_guard_test.go:92`)**

`resources.go` still holds `Heal`'s two writes, so the file exemption survives:

```go
	"internal/characters/resources.go":     "U5c retires Heal; its two writes are all that remain here",
```

- [ ] **Step 5: Build and test**

```bash
go build ./... && go test ./internal/characters/ ./internal/combat/ 2>&1 | tail -20
```

Expected: PASS. **Task 7 in isolation is genuinely behaviour-neutral** — the gate is still in place at this commit, so the winner is affordable by construction and `ApplyCostPartial` is indistinguishable from the old function. The order matters: Task 8 alone would leave defence free.

- [ ] **Step 6: Commit**

```bash
git add internal/characters/resources.go internal/combat/combat_helpers.go pool_mutation_guard_test.go
git commit -m "$(cat <<'EOF'
refactor(characters): delete DeductDefenseStamina for ApplyCostPartial (U5b-2)

Defence charges what it can. This is behaviour-neutral on its own: the
affordability gate is still in place at this commit, so the winning defence is
affordable by construction. The next commit removes the gate and makes it live.

For the record, the roadmap's claim that the discarded return value at the call
site was a live bug is wrong. Candidates were gated before the contest, so that
bool was unreachable-false.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 8: Remove the defence affordability gate (site 2)

Read the "Site 2" and "exhaustion gap" sections in the header before starting. The change is real but narrower than the first draft claimed, and it lands without its designed U8 counterpart.

**Files:** `internal/combat/combat_helpers.go`, `internal/combat/regression_test.go`, create `internal/combat/defense_affordability_test.go`

- [ ] **Step 1: Read the existing regression test that this change inverts**

**[CORRECTED — the first draft omitted this file entirely.]**

```bash
sed -n '78,112p' internal/combat/regression_test.go
```

`TestRegression_DefenseFloorAlwaysApplies` asserts, for a 0-stamina defender, `assert.True(math.IsInf(best.margin, -1))` and `assert.Empty(best.defenseType)`. **Task 8 inverts both by construction.** It guards a real Stage 37.4 bug, so it must be **rewritten, not deleted**.

- [ ] **Step 2: Rewrite it to assert the floor still applies THROUGH the contest**

Replace the two assertions and the trailing comment:

```go
	// U5b-2: a 0-stamina defender is no longer dropped from the candidate set.
	// The Stage 37.4 guarantee this test exists for -- "even a totally broke
	// defender still has a chance" -- is now delivered by the contest itself
	// rather than by MinDefenseChance catching an empty result. That is a
	// strictly stronger guarantee: the defender can win on merit and can
	// defence-crit, neither of which the floor path allowed.
	assert.False(t, math.IsInf(best.margin, -1),
		"a 0-stamina defender must now enter the contest, not fall through to the floor")
	assert.NotEmpty(t, best.defenseType,
		"a defence must be selected even at 0 stamina")
	assert.Contains(t, []string{characters.DefenseDodge, characters.DefenseParry}, best.defenseType,
		"the selected defence must come from the requested sequence")
```

Keep the test name and the Stage 37.4 provenance comment at the top, adding a line noting U5b-2 changed the mechanism.

- [ ] **Step 3: Write the new tests**

Create `internal/combat/defense_affordability_test.go`. Package is `combat` (not `combat_test`) — `runBestOfAllDefense` is unexported, matching `combat_helpers_test.go`.

```go
package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
)

// defenceFixture builds an attacker and a defender with known stats and a
// defender stamina pool the caller controls.
//
// ValueAdj is set directly rather than via Base+Recalculate because stat
// compression was removed on 2026-08-02: ValueAdj == Value always, and the
// combat scoring path reads ValueAdj. This matches avoidance_test.go's idiom.
// characters.New() initialises Position, Buffs, Cooldowns and Stats and calls
// Validate(), so the fixture survives the positional checks in the loop.
func defenceFixture(defenderStamina int) (*characters.Character, *characters.Character) {
	attacker := characters.New()
	attacker.Stats.Strength.ValueAdj = 100
	attacker.Stats.Dexterity.ValueAdj = 100

	defender := characters.New()
	defender.Stats.Dexterity.ValueAdj = 100
	defender.StaminaMax.Base = 100
	defender.StaminaMax.Recalculate()
	defender.Stamina = defenderStamina

	return attacker, defender
}

// U5b-2: an exhausted defender must still get to defend.
//
// Before this chunk the affordability gate `continue`d an unaffordable defence
// out of the candidate set. With every defence unaffordable the entry list came
// out empty, contest.Run reported uncontested, and the swing fell through to the
// MinDefenseChance last-resort branch -- a flat 15% save, always narrated as a
// dodge, with no possibility of a defence crit. After this change the defender
// rolls a real contest.
func TestRunBestOfAllDefense_ExhaustedDefenderStillEntersTheContest(t *testing.T) {
	attacker, defender := defenceFixture(0)

	result := &AttackResult{}
	ctx := combatContext{sourceCanSee: true, targetCanSee: true}

	best := runBestOfAllDefense(result, attacker, defender,
		[]string{characters.DefenseDodge}, 100.0, false, ctx)

	if best.defenseType == "" {
		t.Fatal("an exhausted defender was dropped from the candidate set; " +
			"they must enter the contest and be charged partially instead")
	}
	if math.IsInf(best.margin, -1) {
		t.Error("margin is the uncontested sentinel; the contest did not run")
	}
}

// The winner is charged whatever is actually there. A defence must never come
// out free just because the defender could not pay in full.
//
// Uses BLOCK deliberately. Shipped and default dodge cost is int(2 * 0.9) = 1,
// so a 1-stamina fixture could afford it in full and the assertion would be
// vacuous. Block is int(5 * 0.9) = 4.
func TestRunBestOfAllDefense_PartiallyChargesAnExhaustedWinner(t *testing.T) {
	attacker, defender := defenceFixture(1)

	cost := defender.GetDefenseStaminaCost(characters.DefenseBlock)
	if cost <= 1 {
		t.Fatalf("block costs %d; this test needs a cost above the 1 stamina on hand", cost)
	}

	result := &AttackResult{}
	ctx := combatContext{sourceCanSee: true, targetCanSee: true}

	best := runBestOfAllDefense(result, attacker, defender,
		[]string{characters.DefenseBlock}, 100.0, false, ctx)

	if best.defenseType == "" {
		t.Fatal("exhausted defender was dropped from the candidate set")
	}
	if defender.Stamina != 0 {
		t.Errorf("stamina after a partial defence charge = %d, want 0 (charged down to empty, not refused)", defender.Stamina)
	}
}
```

- [ ] **Step 4: Run the new tests to verify they fail**

```bash
go test ./internal/combat/ -run TestRunBestOfAllDefense -v
```

Expected: FAIL on the first with "an exhausted defender was dropped from the candidate set".

- [ ] **Step 5: Delete the gate (`combat_helpers.go:560-563`)**

Replace:

```go
		// Check if defender can afford this defense (don't deduct yet)
		cost := targetChar.GetDefenseStaminaCost(defenseType)
		if targetChar.Stamina < cost {
			continue
		}
```

with:

```go
		// U5b-2: there is deliberately NO affordability gate here.
		//
		// This used to `continue` an unaffordable defence out of the candidate
		// set. With every defence unaffordable the entry list came out empty,
		// contest.Run reported uncontested, and the swing fell through to the
		// MinDefenseChance last resort -- a flat 15% save, always narrated as a
		// dodge, and never able to defence-crit. An exhausted actor still acts;
		// the winning defence is charged partially below.
		//
		// The defender's exhaustion currently costs their defence NOTHING:
		// GetDefenseScore has no resource term, and stripping the skill term is
		// U8. That gap is deliberate and disclosed, not an oversight.
```

Deleting `cost :=` is safe — it has no other use in the loop, so no unused-variable error.

- [ ] **Step 6: Verify the surrounding invariants**

```bash
sed -n '548,562p' internal/combat/combat_helpers.go
```

Confirm `result.DefenseAttempts` (`:555`) and `IncrementDefenseCount()` (`:558`) sit **above** where the gate was, so stance tracking is unchanged. No other code infers a defence from stamina.

- [ ] **Step 7: Run the combat and hooks suites**

```bash
go test ./internal/combat/ ./internal/hooks/ 2>&1 | tail -20
```

Expected: PASS, including the rewritten `TestRegression_DefenseFloorAlwaysApplies`.

Failures in `contest_floors_test.go`, `crit_floor_test.go` or `margin_crit_contest_test.go` would be meaningful — read them before touching either file. Verified not at risk: melee calls `contest.Run`, not `RunWithFloors` (`combat_helpers.go:642`), so the ±1 floor sentinel does not exist on this path; melee floors are applied post-hoc at `:925-952` and never touch margin. The `math.Inf(-1)` sentinel is still reachable whenever `defSeq` is empty, which `filterDefensesForThirdParty` (`:522`) still produces.

- [ ] **Step 8: Commit**

```bash
git add internal/combat/combat_helpers.go internal/combat/defense_affordability_test.go internal/combat/regression_test.go
git commit -m "$(cat <<'EOF'
feat(combat): an exhausted defender rolls a real contest (U5b-2)

NAMED BEHAVIOUR CHANGE, and the largest live-feel change in the arc so far --
though narrower than first described.

The affordability gate continued an unaffordable defence out of the best-of-N
candidate set. With every defence unaffordable the entry list came out empty and
the swing fell through to the MinDefenseChance last resort: a flat 15% save,
always narrated as a dodge, and never able to defence-crit. It was NOT an
unopposed auto-hit. After this change the defender rolls a real contest, can win
on merit, can defence-crit, and the correct defence type is named.

It bites in a narrow band. Shipped defence costs are dodge 1, parry 3, block 4,
so the gate only excluded a defence at 0-3 stamina.

DISCLOSED GAP: the cost spec pairs this with stripping the skill term from an
unaffordable defence, which is U8. GetDefenseScore has no resource term and
ResourceMultiplier's callers are all attack-side, so until U8 lands an exhausted
defender defends exactly as well as a rested one, for free. That is more
generous than either endpoint and is a deliberate, temporary state. Do not tune
against it.

TestRegression_DefenseFloorAlwaysApplies is rewritten rather than deleted: its
Stage 37.4 guarantee is now delivered by the contest itself, which is strictly
stronger than the floor path it used to assert.

Defence attempts and stance counting already happened above the gate, so stance
tracking is unchanged.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 9: Unfloor health and clamp every display surface

**[CORRECTED — merged.]** The first draft split these into two commits. They are one change: unfloor the pool, clamp the display. Splitting them means commit 9 introduces a player-visible regression that commit 10 removes, so anyone bisecting, reverting or cherry-picking commit 9 alone gets negative health on the wire.

**Files:** the seven floor sites, `internal/characters/pools.go`, `internal/users/userrecord.prompt.go`, `internal/templates/templatesfunctions.go`, `modules/gmcp/gmcp.Char.go`, `modules/playtest/beacons.go`, `pool_mutation_guard_test.go`, create `internal/hooks/health_overkill_test.go` and `internal/characters/display_health_test.go`

#### Part A — the clamp helper (do this FIRST, so no commit ever ships an unclamped display)

- [ ] **Step 1: Add one exported helper in `internal/characters/pools.go`**

**[CORRECTED]** The first draft put a private helper in `internal/users` plus an inline `max(0, …)` in gmcp — three clamps in three packages, guaranteeing the next display surface forgets. One helper, used everywhere:

```go
// DisplayHealth returns health clamped for player-facing output.
//
// The pools deliberately store overkill -- U6 reads the magnitude of a killing
// blow, and ApplyHarm floors stamina and conviction at 0 but never health -- so
// a negative value is correct in the model and wrong on the wire.
//
// Every display surface must call this: the prompt, the status template, GMCP
// vitals, GMCP enemies, and the playtest beacon. renderVitalBar and
// targetHealthDesc already clamp internally and do not need it.
func (c *Character) DisplayHealth() int {
	if c.Health < 0 {
		return 0
	}
	return c.Health
}
```

- [ ] **Step 2: Test it**

Create `internal/characters/display_health_test.go`:

```go
package characters

import "testing"

func TestDisplayHealth_ClampsNegativeToZero(t *testing.T) {
	c := New()
	c.Health = -25
	if got := c.DisplayHealth(); got != 0 {
		t.Errorf("DisplayHealth() with Health=-25 = %d, want 0", got)
	}
}

func TestDisplayHealth_PassesThroughNonNegative(t *testing.T) {
	c := New()
	for _, hp := range []int{0, 1, 137} {
		c.Health = hp
		if got := c.DisplayHealth(); got != hp {
			t.Errorf("DisplayHealth() with Health=%d = %d, want %d", hp, got, hp)
		}
	}
}
```

```bash
go test ./internal/characters/ -run TestDisplayHealth -v
```

Expected: PASS.

- [ ] **Step 3: Wrap the nine unsafe prompt reads**

In `internal/users/userrecord.prompt.go`, replace `u.Character.Health` with `u.Character.DisplayHealth()` (and `pet.Character.Health` with `pet.Character.DisplayHealth()`) at exactly these nine:

| Line | Token | Read |
|---|---|---|
| 335 | `{hp}` class | `util.QuantizeTens(u.Character.Health, ...)` |
| 337 | `{hp}` value | `u.Character.Health` |
| 340 | `{hp:-}` value | `u.Character.Health` |
| 343 | `{HP}` **class** | `util.QuantizeTens(u.Character.Health, ...)` |
| 350 | `{hp%}` pct | `float64(u.Character.Health)` |
| 353 | `{hp%}` class | `util.QuantizeTens(u.Character.Health, ...)` |
| 359 | `{hp%:-}` pct | `float64(u.Character.Health)` |
| 441 | pet class | `util.QuantizeTens(pet.Character.Health, ...)` |
| 442 | pet pct | `float64(pet.Character.Health)` |

`{HP}` at `:345` *prints* `HealthMax`, which is never negative, but its **class** on `:343` derives from `Health` and does need wrapping.

Do **not** wrap `:427`, `:496`, `:500`, `:543`, `:546` — those feed `renderVitalBar` (clamps at `:84-85`) and `targetHealthDesc` (returns "dead" for `<= 0` at `:167`).

- [ ] **Step 4: Verify the prompt sweep**

**[CORRECTED]** The first draft's grep was a false pass: seven of the nine unsafe reads have `HealthMax` on the same line, so `grep -v HealthMax` stripped them whether or not they were wrapped. Use a word boundary:

```bash
grep -nE "Character\.Health\b" internal/users/userrecord.prompt.go | grep -v DisplayHealth
```

Expected after the edit: **only** lines 427, 496, 500, 543.

- [ ] **Step 5: Clamp the three remaining display surfaces**

**[CORRECTED — none of these were in the first draft except gmcp.Char.go:536.]**

`modules/gmcp/gmcp.Char.go:536` (own vitals):
```go
			Hp:            user.Character.DisplayHealth(),
```

`modules/gmcp/gmcp.Char.go:454` (enemy list — reachable from sites 1d/1e/1g, which unfloor **mob** health, and rendered during the one-round zombie window):
```go
					Hp:      mob.Character.DisplayHealth(),
```

`modules/playtest/beacons.go:59` (the harness Task 15 uses — an unclamped beacon would make the playtest report negative HP):
```go
			HP:     u.Character.DisplayHealth(),
```

`internal/templates/templatesfunctions.go:171` — the `healthStr` helper backs `status.template:3` and renders raw `%d` plus `util.QuantizeTens(h, hMax)`. It takes an `int`, not a Character, so clamp at the top of the helper:

```go
		"healthStr": func(h int, hMax int, padTo ...int) string {
			// U5b-2: pools store overkill; displays never show it. QuantizeTens
			// has no negative guard, so an unclamped h also yields a malformed
			// ANSI class.
			if h < 0 {
				h = 0
			}
			padding := ``
```

**Verified NOT needed:** `modules/gmcp/gmcp.Party.go` already clamps at all three sites (`:98`, `:291`, `:368` each carry `if hPct < 0 { hPct = 0 }`). One reviewer listed these as gaps; reading the code shows they are safe. `internal/usercommands/companion.go:53` goes through `RenderVitalBar`, which clamps.

#### Part B — remove the seven floors

- [ ] **Step 6: Confirm the count is still seven**

```bash
grep -rn "NOTE(U5b-2)" internal/ --include=*.go | wc -l
```

Expected: `7`. If not, stop — the site table is stale.

- [ ] **Step 7: Write the overkill contract test**

Create `internal/hooks/health_overkill_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
)

// U5b-2 removed the seven retained health floors, so health stores overkill
// consistently at every damage site. U6 reads that magnitude.
//
// This pins the primitive contract all seven sites now rely on: harm floors
// stamina and conviction at zero but deliberately does NOT floor health.
// validate.go carries a matching explicit "No lower Health clamp" comment.
func TestApplyHarm_HealthStoresOverkill(t *testing.T) {
	c := characters.New()
	c.HealthMax.Base = 100
	c.HealthMax.Recalculate()
	c.Health = 10

	applied := c.ApplyHarm(characters.PoolHealth, 35, state.ActorRef{})

	if c.Health != -25 {
		t.Errorf("health after 35 damage to a 10-health character = %d, want -25 (overkill preserved for U6)", c.Health)
	}
	if applied != 35 {
		t.Errorf("applied = %d, want 35 (the full amount, not the amount that fit)", applied)
	}
}

// Every death gate in the game tests `< 1` or `<= 0` -- all 57 health
// comparisons in internal/ and modules/ were audited and none is `== 0` -- so a
// negative value passes all of them. This is the assertion that makes removing
// the floors safe.
func TestApplyHarm_NegativeHealthStillReadsAsDead(t *testing.T) {
	c := characters.New()
	c.HealthMax.Base = 100
	c.HealthMax.Recalculate()
	c.Health = 1

	c.ApplyHarm(characters.PoolHealth, 50, state.ActorRef{})

	if !(c.Health < 1) {
		t.Errorf("health = %d does not satisfy the `< 1` death gate", c.Health)
	}
	if !(c.Health <= 0) {
		t.Errorf("health = %d does not satisfy the `<= 0` death gate", c.Health)
	}
}
```

```bash
go test ./internal/hooks/ -run TestApplyHarm -v
```

Expected: PASS already — `ApplyHarm` has never floored health. This is the standing guarantee the seven deletions rely on.

- [ ] **Step 8: Delete all seven floors**

Each is a seven-line block: a four-line `NOTE(U5b-2)` comment plus a three-line `if`. Delete the whole block at:

- `internal/actions/surprise_attack.go:299-305` (`targetChar.Health`)
- `internal/combat/skill_moves.go:98-104` (`p.Defender.Health`)
- `internal/hooks/combat_shared_helpers.go:238-244` (`attacker.Health`, riposte)
- `internal/hooks/NewRound_AutoHeal.go:380-386` (poison, condition is `< 1`)
- `internal/hooks/NewRound_AutoHeal.go:398-404` (bleed, condition is `< 1`)
- `internal/usercommands/throw.go:173-179` (`user.Character.Health`, self-fumble)
- `internal/usercommands/throw.go:225-231` (`mob.Character.Health`)

The riposte site (`combat_shared_helpers.go:237`) reassigns `dmg` from `ApplyHarm`'s return, which is unfloored for health, so removing the floor changes neither `dmg` nor the damage description.

- [ ] **Step 9: Verify all seven are gone**

**[CORRECTED]** The first draft grepped for `U5b-2` and expected no output — but Tasks 2-8 deliberately *add* comments containing that token. Grep for the marker only:

```bash
grep -rn "NOTE(U5b-2)" internal/ --include=*.go
```

Expected: **no output**.

- [ ] **Step 10: Remove the five guard exemptions**

Delete `pool_mutation_guard_test.go:100-104`:

```go
	"internal/hooks/NewRound_AutoHeal.go":     "U5b-2: mob DoT health floors",
	"internal/actions/surprise_attack.go":     "U5b-2: retained health floor",
	"internal/combat/skill_moves.go":          "U5b-2: retained health floor",
	"internal/hooks/combat_shared_helpers.go": "U5b-2: retained health floor + fold-upkeep cost",
	"internal/usercommands/throw.go":          "U5b-2: retained health floors",
```

**Keep** the `DELIBERATELY ABSENT` block at `:106-113` — it explains why `combat_helpers.go` and `flee.go` are *not* exempt, which is still true and still useful.

- [ ] **Step 11: Build, test, and check the real completion signal**

**[CORRECTED]** `grep -c "U5b-2"` can never reach 0 — the token appears in the guard file's prose at `:37`, `:74`, `:81`, `:96-99`, `:106`, `:108`, `:112`, and Step 10 explicitly keeps some of that. Gate on the exemption **map values** instead:

```bash
go build ./... && go test ./internal/... ./modules/... 2>&1 | grep -v "^ok" | head -20
grep -c '"U5b-2' pool_mutation_guard_test.go
go test . -run TestPoolMutationGoesThroughThePrimitives -v 2>&1 | tail -5
```

Expected: build clean, no test failures, `grep -c` returns **0**, guard PASS. **That grep plus a green guard is the completion signal for the chunk.**

- [ ] **Step 12: Commit**

```bash
git add internal/characters/pools.go internal/characters/display_health_test.go \
        internal/actions/surprise_attack.go internal/combat/skill_moves.go \
        internal/hooks/combat_shared_helpers.go internal/hooks/NewRound_AutoHeal.go \
        internal/usercommands/throw.go internal/hooks/health_overkill_test.go \
        internal/users/userrecord.prompt.go internal/templates/templatesfunctions.go \
        modules/gmcp/gmcp.Char.go modules/playtest/beacons.go pool_mutation_guard_test.go
git commit -m "$(cat <<'EOF'
feat(pools): remove the seven retained health floors, clamp the displays (U5b-2)

NAMED BEHAVIOUR CHANGE. Health now stores overkill at every damage site instead
of at nineteen of twenty-six, which is what U6 reads to size a killing blow.
U5b-1 held these seven back together so this could be one reviewable unit.

Death is unaffected: all 57 health comparisons in internal/ and modules/ are
`< 1`, `<= 0`, `> 0` or `>= 1`, and none is `== 0`, so a negative passes every
gate.

The unfloor and the display clamp ship in ONE commit deliberately. Split, the
first would introduce a player-visible regression that the second removes, so
any bisect or revert landing between them would put negative health on the wire.

The exposure is wider than a naive read suggests, which is why the clamp lives
in one exported Character.DisplayHealth() rather than three inline guards:
message senders fire RedrawPrompt without OnlyIfChanged, so the melee and
skill-move sites re-render a player's prompt too, and three of the seven sites
unfloor MOB health, which ships raw to GMCP enemy lists and the playtest beacon.
util.QuantizeTens has no negative guard, so an unclamped value also produced a
malformed ANSI class.

gmcp.Party.go and renderVitalBar/targetHealthDesc already clamp internally and
are deliberately untouched.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 10: Update `context.md` and the roadmap

Standing rule 3: `context.md` ships in the same PR, never as a follow-up.

**Files:** `internal/characters/context.md`, `internal/combat/context.md`, `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`, and **the plan file itself**

- [ ] **Step 1: Audit the current claims**

```bash
grep -n "DeductDefenseStamina\|DeductStamina\|health floor\|affordability" internal/characters/context.md internal/combat/context.md
```

- [ ] **Step 2: Update `internal/characters/context.md`**

Remove any `DeductDefenseStamina` entry from the Public API section — the function no longer exists. Add `DisplayHealth()`. In Gotchas:

```markdown
- **Health stores overkill; stamina and conviction do not.** `ApplyHarm` floors
  stamina and conviction at 0 and deliberately does NOT floor health, so a
  killing blow leaves a negative value that U6 reads for magnitude.
  `validate.go` carries a matching explicit "No lower Health clamp". Clamping
  belongs at the display layer: call `Character.DisplayHealth()`, never re-add a
  floor here. As of U5b-2 all seven remaining per-site floors are gone, so this
  is uniform.
- **`ApplyCost` vs `ApplyCostPartial` is not a style choice.** Refuse where a
  meaningful alternative action remains (movement, stand, spellcasting, mutation
  special moves); charge partially where refusal would leave the actor helpless
  (auto-attack, dodge/parry/block, grapple upkeep, flee). The split is NOT
  volitional-vs-involuntary and NOT "uses a cooldown"; both framings were tried
  and both are provably false.
- **A green pool-mutation guard does not mean every pool write is routed.**
  `resources.go` is exempt as a FILE, so `Heal()`'s writes are invisible and so
  are its three production callers (`actions/combat_drain.go:126`, `:281`,
  `hooks/item_procs.go:99`). They retire with `Heal` in U5c.
```

- [ ] **Step 3: Update `internal/combat/context.md`**

```markdown
- **`runBestOfAllDefense` has no affordability gate, on purpose.** Every defence
  in the sequence enters the contest regardless of the defender's stamina, and
  only the winner is charged, partially. Re-adding a gate would drop an
  exhausted defender out of the contest and back onto the `MinDefenseChance`
  last resort: a flat 15% save, always narrated as a dodge, never able to
  defence-crit. Defence attempts and stance counting happen above where the gate
  used to be, so they are unaffected either way.
- **Exhaustion currently costs a defender nothing.** `GetDefenseScore` has no
  resource term and every `ResourceMultiplier` caller is attack-side, so between
  U5b-2 and U8 a 0-stamina defender defends exactly as well as a rested one.
  That is a known, temporary, deliberate gap; U8 strips the skill term. Do not
  "fix" it by re-adding a gate.
```

- [ ] **Step 4: Run the context audit, scoped**

```bash
python tools/context_md_audit.py internal/characters
python tools/context_md_audit.py internal/combat
```

Expected: no findings about symbols that do not exist. If it flags `DeductDefenseStamina`, you missed a reference.

- [ ] **Step 5: Correct the roadmap in BOTH places**

**[CORRECTED]** The first draft fixed only `:175`. The same wrong claim is repeated at `:204-205`.

At `:175`, strike "the discarded `DeductDefenseStamina` bool" from the named-behaviour-fixes list and replace it with "the defence affordability gate that dropped exhausted defenders onto the MinDefenseChance last resort".

At `:204-205`, delete the claim *"`DeductDefenseStamina` returns a bool that `combat_helpers.go:665` discards. A defence the character cannot pay for still wins the best-of-N."* — the second sentence is false under the pre-change gate.

Mark U5b complete.

- [ ] **Step 6: Commit, including the plan file**

**[CORRECTED]** The plan file is itself untracked and no task committed it, so Task 12's `git status` check would flag it. It belongs with the docs commit.

```bash
git add internal/characters/context.md internal/combat/context.md \
        docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md \
        docs/superpowers/plans/completed/2026-08-13-u5b2-named-behaviour-changes.md
git commit -m "$(cat <<'EOF'
docs: record the U5b-2 behaviour changes in context.md and the roadmap

Corrects the roadmap in two places (:175 and :204-205), both of which listed the
discarded DeductDefenseStamina bool as a named behaviour fix. It was never a
live bug: candidates were gated before the contest, so the winner was affordable
by construction and that return value was unreachable-false.

Also records the U8 exhaustion gap and the fact that a green pool guard does not
mean every pool write is routed.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 11: Patch notes

Player-facing framing. No raw numbers. **No en dashes or em dashes anywhere**, including `&mdash;`. Plain language for ESL readers.

- [ ] **Step 1: Check the house format and the existing heading**

```bash
head -20 docs/PATCH_NOTES.md
```

**[CORRECTED]** There is already a `## 2026-08-13: The shared way of spending and healing is now the only way` entry from U5b-1. The format is `## DATE: Title`, so the new entry needs its own title rather than a bare duplicate date.

- [ ] **Step 2: Add the entry**

```markdown
## 2026-08-13: Fighting while worn out

Being out of breath no longer takes you out of the fight. If you are too tired
to pay for a dodge, a parry or a block, you now still get to try it, and you pay
whatever you have left. Before this, an exhausted fighter was dropped out of the
exchange entirely and left with only a small last chance to avoid the blow, and
that chance was always described as a dodge no matter what you were actually
good at. Now the right defence is named, and it can turn into a telling one.

Running away works the same way. You can always break off and flee, no matter
how tired you are. While you are in a fight you cannot simply walk out, so this
is the one way to leave, and it should never be closed to you.

Spellcasting creatures now check that they can pay for a spell before they start
weaving it. One could previously begin a spell it had no hope of finishing.
```

> Deliberately **not** mentioned: that exhaustion currently costs a defender nothing. That is a temporary internal gap closing in U8, not a feature to announce.

- [ ] **Step 3: Verify no dashes slipped in**

```bash
grep -n "—\|–\|&mdash;\|&ndash;" docs/PATCH_NOTES.md | head
```

Expected: no hits in the new entry.

- [ ] **Step 4: Commit**

```bash
git add docs/PATCH_NOTES.md
git commit -m "docs: patch notes for U5b-2

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 12: Author the U5b-2 playtest goals file

**[NEW — added on review.]** The first draft pointed Task 15 at `2026-08-03-prepush-sweep.yaml`, a generic 30-minute sweep whose goals are shop economy, crafting exclusivity, recipe dispatch, assess disclosure and help spot-checks. Nothing in it drives an agent toward any U5b-2 change, and the target states (0 stamina in combat, a ~2.3% grenade fumble at low health, a drained mob caster) are individually unlikely from a fresh character inside that budget.

**Files:** create `tools/playtest/goals/2026-08-13-u5b2-exhaustion.yaml`

- [ ] **Step 1: Write the goals file**

```yaml
# U5b-2 targeted adversarial playtest: exhaustion, unfloored health, and the
# mob cast guard. Local runs require `ephemeral:`.
#
# READ THIS FIRST, TESTER: exhaustion currently costs a DEFENDER nothing. The
# skill-term penalty lands in U8. If an out-of-breath character defends just as
# well as a rested one, that is the expected temporary state, not a bug. Report
# how it FEELS, but do not treat it as a defect and do not tune against it.
ephemeral:
  creation_flow: true
  creation_rationale: >
    Fresh character required: the exhaustion and prone paths must be exercised
    on a real progression-appropriate character rather than a seeded veteran,
    whose larger pools would hide the low-stamina band entirely.
  budgets:
    wall_clock: 40m

goals:
  - >-
    Fight to exhaustion. Take a real fight long enough that your stamina
    reaches zero and keep fighting. Defences must still fire. Read every
    combat line: does the defence text name the right thing (block, parry,
    dodge) rather than always saying dodge? Does anything read as though
    you defended for free in a way that feels wrong?
  - >-
    Prone recovery and flee. Get knocked down at low stamina. You should
    stand back up on your own within a few rounds without spending
    anything. Confirm that. Then at zero stamina confirm flee ALWAYS
    works and never refuses. Note whether the fleeing character arrives
    in the next room still prone, and whether that reads as a bug.
  - >-
    Grenade self-fumble. Throw an explosive repeatedly at low health until
    one backfires onto you. The moment it does, read the prompt, the status
    sheet, and any client health bar. None of them may show a negative
    number or a missing or wrong health colour.
  - >-
    Kill mobs with poison, with bleed, and with a thrown explosive. Watch
    the enemy health readout in the web client for the round in which each
    dies. It must never show a negative number.
  - >-
    Drain a spellcasting creature's conviction over a long fight. It must
    stop casting cleanly, must never narrate a spell it cannot cast, and
    must not lock itself out of its other special moves afterwards. Note
    whether caster mobs seem to cast MORE often than you remember.
  - >-
    Player caster runs dry mid-spell. Begin a multi-fold spell with barely
    enough conviction and let it run out. Read what happens to the spell,
    to the conviction already spent, and to your round.
  - >-
    Mutation special move at low stamina. Attempt one you cannot afford.
    It must refuse with a clear message AND must not consume the shared
    special-move cooldown. Immediately try a different special move to
    confirm the slot is still free.
  - >-
    General adversarial pass: anything that reads wrong, renders oddly, or
    contradicts its own help text is a finding.
```

- [ ] **Step 2: Confirm it parses the way its siblings do**

```bash
python -c "import yaml,sys; d=yaml.safe_load(open('tools/playtest/goals/2026-08-13-u5b2-exhaustion.yaml')); print(list(d.keys())); print(len(d['goals']))"
```

Expected: `['ephemeral', 'goals']` and `8`.

- [ ] **Step 3: Commit**

```bash
git add tools/playtest/goals/2026-08-13-u5b2-exhaustion.yaml
git commit -m "test(playtest): targeted U5b-2 exhaustion goals file

The generic prepush sweep drives an agent through shop, craft and help
itineraries and would never reach zero stamina in combat, a grenade self-fumble,
or a drained mob caster. This one seeks those states directly and briefs the
tester that unpenalised exhausted defence is the expected pre-U8 state.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 13: Pre-push gates

- [ ] **Step 1: gofmt — has its own CI gate and has broken a push before**

```bash
gofmt -l internal/ modules/ *.go
```

Expected: **no output**.

> **[CORRECTED during execution, 2026-08-13.]** The SOP's `gofmt -l internal/ modules/` **does not cover repo-root `.go` files**, and `pool_mutation_guard_test.go` lives at the repo root. Deleting one line from its aligned exemption block leaves the survivors over-aligned, so every task that removes an exemption can silently make it gofmt-dirty. This actually happened at Task 2 and was not caught until Task 4. **Run the `*.go` form after any task that edits the exemption map** — Task 9 removes five more lines from that same block. Do not run bare `gofmt -l .`: it floods with pre-existing `vendor/` hits.

- [ ] **Step 2: Full build and test**

```bash
go build ./... && go test ./... 2>&1 | grep -v "^ok" | head -30
```

Expected: no failures. `internal/relationships` may fail to run because Windows Defender quarantines its test binary — expected and pre-existing; let CI run it. `boot_smoke_test.go` skips unless `GOMUD_BOOT_SMOKE` is set, so this will not contend for the user's ports.

- [ ] **Step 3: The completion signal**

```bash
grep -c '"U5b-2' pool_mutation_guard_test.go
go test . -run TestPoolMutationGoesThroughThePrimitives -v 2>&1 | tail -5
```

Expected: `0`, then PASS. If the guard prints `[no tests to run]`, the test name is wrong — it is `TestPoolMutationGoesThroughThePrimitives`.

- [ ] **Step 4: Set `LogToFile: false` for the boot test**

The working copy carries `LogToFile: true` (a local soak override, restored in Task 1 Step 7). The boot worktree needs `false`.

- [ ] **Step 5: Boot test in an isolated detached worktree**

```bash
git worktree add --detach C:/tmp/dogmud-u5b2-boot HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-u5b2-boot/_datafiles/config.yaml
```

Edit `C:/tmp/dogmud-u5b2-boot/_datafiles/config.yaml` to non-default ports so it cannot collide with the user's live server: `TelnetPort: [43333]`, `LocalPort: 19999`, `HttpPort: 18090`, `AIPort: 15555`, and `LogToFile: false` (a fresh worktree has no `_datafiles/logs` and the server exits 1 without it).

```bash
cd C:/tmp/dogmud-u5b2-boot && timeout 180 go run . > boot.log 2>&1
echo "exit=$?"
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

**Exit code 124 is the success case.** Do **not** grep the bare word `panic` — `GamePlay.MapConsistencyEnforce` legitimately has the *value* `panic`.

- [ ] **Step 6: Clean up**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git -C C:/tmp/dogmud-u5b2-boot diff --stat
git worktree remove --force C:/tmp/dogmud-u5b2-boot
```

If Windows holds a lock: `rm -rf C:/tmp/dogmud-u5b2-boot && git worktree prune`.

- [ ] **Step 7: Restore the config.yaml skip-worktree bit**

```bash
git ls-files -v _datafiles/config.yaml     # currently H
git update-index --skip-worktree _datafiles/config.yaml
git ls-files -v _datafiles/config.yaml     # now S
```

- [ ] **Step 8: Confirm only the four permanent items remain**

```bash
git status --short
```

**[CORRECTED]** Expected exactly four (the plan file was committed in Task 10):

```
 M _datafiles/world/dogmud/rooms/thornwall_city/473.yaml
?? ADVERSARIAL_CODE_REVIEW_2026-08-07.md
?? tools/playtest/goals/scenarios/harness-sanity-duo/
?? tools/playtest/scenarios/harness-sanity-duo.yaml
```

If one got swept into a commit, reset it out.

---

### Task 14: Push and open the PR

- [ ] **Step 1: Push**

```bash
git push -u origin feature/u5b2-named-behaviour-changes
```

- [ ] **Step 2: Open the PR — `--repo pruuk/DOGMud` is mandatory**

`gh` defaults to the fork **parent**, `GoMudEngine/GoMud`. A bare `gh pr create` opened a PR against upstream on 2026-08-08 and had to be closed immediately.

```bash
gh pr create --repo pruuk/DOGMud --base master --head feature/u5b2-named-behaviour-changes \
  --title "U5b-2: the named behaviour changes" --body-file - <<'EOF'
Closes the U5b slice. U5b-1 routed every pool site that could move without
changing behaviour; this moves the eight that could not. Standing rule 1 was
released for U5 by the user on 2026-08-13, so behaviour changes are in scope,
but only at the sites named below.

## Named behaviour changes

1. **An exhausted defender rolls a real contest.** The affordability gate
   continued an unaffordable defence out of the best-of-N candidate set, so the
   swing fell through to the `MinDefenseChance` last resort: a flat 15% save,
   always narrated as a dodge, never able to defence-crit. It was **not** an
   unopposed auto-hit. It also bites in a narrow band, since shipped defence
   costs are dodge 1, parry 3, block 4.
2. **Health stores overkill at every damage site.** The seven retained floors
   are gone. Death is unaffected: all 57 health comparisons are `< 1`, `<= 0`,
   `> 0` or `>= 1`, none `== 0`. The display clamp ships in the same commit.
3. **Defence charges partially.** `DeductDefenseStamina` is deleted.
4. **Mob casting is guarded.** That path had no affordability check of any kind.
   The charge is sequenced above the YAML cast text and above
   `TransitionToCasting`, rolls back the shared special-move cooldown that
   `InitiateCast` has already consumed, and refunds on a failed transition.
5. **Flee charges partially and never refuses.** `go.go` refuses all movement in
   combat, so flee is the only player-initiated disengage. The hardcoded 10
   becomes `FleeStaminaCost`, shipped at 10.
6. **Stand keeps its two knobs.** `CanAfford(StandMinStamina)` plus
   `ApplyCostPartial(StandStaminaCost)` — partial because the FSM transition
   fires first, so a refusal would let the player stand for free.

Behaviour-neutral routing: fold-cast upkeep, the mutation special-move preamble.

## ⚠️ Disclosed gap: exhaustion currently costs a defender nothing

Cost spec §3.4 pairs gate-removal with stripping the skill term from an
unaffordable defence. This PR ships the first half; the second is U8.
`GetDefenseScore` has no resource term and every `ResourceMultiplier` caller is
attack-side, so **until U8 lands, a 0-stamina defender defends exactly as well
as a rested one, for free** — more generous than either endpoint. This is a
deliberate, temporary state, taken knowingly. Do not tune against it, and note
that the playtest brief says the same.

## Two corrections to earlier documents

- The roadmap listed "the discarded `DeductDefenseStamina` bool" as a named
  behaviour fix, at both `:175` and `:204-205`. It was never a live bug:
  candidates were gated before the contest, so that return value was
  unreachable-false. Both are corrected here.
- The U5a plan's "prone/stand death spiral" argument for flee=Partial is
  **false**. `AttemptRecovery` is a free roll firing every round at roughly 53%
  for a baseline character, `MinRecoveryRounds` is 1-2 at every production
  knockdown site, and all stamina regen runs only on `RoundNumber%3 == 0`. A
  prone character stands in about four rounds without spending anything. The
  assignment is right; the justification was not, and it is not repeated in any
  comment, commit or patch note here.

## Scope notes

- A green pool guard means every **U5b-2 site** is routed, not every pool write.
  `resources.go` is exempt as a file, so `Heal()` and its three production
  callers (`combat_drain.go:126`, `:281`, `item_procs.go:99`) stay invisible
  until U5c.
- Deliberately left alone: the player/mob cast asymmetry (U7 — note this chunk
  *narrows* it, moving mobs from "no floor at all" to "must hold one slice");
  `stand`'s raw-number exhaustion message (player-copy work, not this chunk);
  the literal stamina costs in `mutation_cocoon.go:15` and
  `mutation_venom_coat.go:13` (standing-rule-2 violations of the same family as
  flee's, not routed here).

## Verification

- `grep -c '"U5b-2' pool_mutation_guard_test.go` returns 0 and
  `TestPoolMutationGoesThroughThePrimitives` passes. That pair is the
  completion signal.
- `gofmt -l internal/ modules/` clean, `go build ./...` clean, `go test ./...`
  clean.
- Boot test in an isolated detached worktree reached `Server Ready` with no
  panics.

## Not yet done

The adversarial in-game playtest, using the targeted goals file added in this
PR. U5b-2 is the slice that gets it, and it gates the merge.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
```

- [ ] **Step 3: Watch the checks, then verify which runs re-ran**

```bash
gh pr checks <n> --repo pruuk/DOGMud --watch
gh run view <id> --repo pruuk/DOGMud --log-failed
```

**A green check is not proof.** `gh pr checks --watch` has returned early before, and the lint gate is `only-new-issues`, so a run can pass while emitting annotations.

- [ ] **Step 4: Do NOT merge yet.** Task 15 is the gate.

---

### Task 15: Adversarial in-game playtest — the merge gate

- [ ] **Step 1: Run the harness against the targeted goals file**

Note the quoted path — it contains a space.

```text
/playtest local --checkout "C:/Users/Calabe Davis/workspace/DOGMud" bug-finder 2026-08-13-u5b2-exhaustion.yaml
```

- [ ] **Step 2: Brief the tester on the expected temporary state**

The goals file carries this at the top, but restate it: **exhaustion costs a defender nothing until U8.** An out-of-breath character defending as well as a rested one is expected, not a defect. Report how it feels; do not file it as a bug and do not tune against it.

- [ ] **Step 3: Read every line of output and report bluntly**

The eight goals cover: exhausted defence, prone recovery plus flee, grenade self-fumble display, mob DoT and grenade deaths in the client, mob caster conviction drain, player caster running dry mid-fold, mutation special move cooldown rollback, and a general pass.

- [ ] **Step 4: Fix what it finds, re-run if needed**

- [ ] **Step 5: Hand to the user for manual playtest**

Do **not** claim U5b-2 done on a clean boot and a green CI run alone.

- [ ] **Step 6: Merge only after the user signs off**

```bash
gh pr merge <n> --repo pruuk/DOGMud --merge --delete-branch
```

`--merge` (a `--no-ff` merge commit), **not** `--squash` — the per-commit messages carry the evidence.

- [ ] **Step 7: Delete the stray `refs/tags/master` if it re-seeds**

```bash
git ls-remote --tags origin | grep master
```

**🚫 Merging is not deploying.** No deploy until the whole arc is done and playtested. Prod stays on `7c64c228c`.

---

## Post-merge follow-ups

**Memory** (`project-unified-contest-resolution-arc.md`):
- U5b-2 → merged, with the PR number and the new master SHA; U5c → next
- **Delete the prone/stand death-spiral entry** — it is false, and it has already propagated into two plans and one memory note
- **Correct the "auto-hit" framing** of the defence gate to the `MinDefenseChance` floor-save reality
- Add: the guard's real test name is `TestPoolMutationGoesThroughThePrimitives`
- Add: `config.yaml`'s working copy carries four permanent local overrides, so any commit touching it needs the checkout-then-restore dance in Task 1 Step 6
- Replace the "▶ START HERE FOR U5b-2" section with a U5c equivalent; keep the traps section verbatim

**Back-correct** `docs/superpowers/plans/completed/2026-08-13-u5a-cost-harm-foundation.md:1177-1182`, the origin of the death-spiral claim.

**File for a future chunk:**
- The U8 exhaustion gap closes the loop opened here — flag it as a hard dependency, not a nice-to-have
- `stand`'s raw-number message; the `mutation_cocoon`/`venom_coat` literals
- The unassigned flat-roll family (`search.go` ×6, `track.go:121`, `forage_core.go:126`) has now survived U4 and U5 unowned
