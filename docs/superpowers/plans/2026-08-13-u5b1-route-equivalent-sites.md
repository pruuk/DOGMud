# U5b-1 — Route the Provably-Equivalent Pool Sites: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Route every pool mutation whose behaviour is provably unchanged onto
the U5a primitives, delete the clamp blocks the primitives now own, and declare
the guard exemptions. **No gameplay delta.** The five real behaviour changes are
deliberately held back for U5b-2.

**Architecture:** The seam is **behaviour-neutral vs behaviour-changing**, not by
file or by primitive. Splitting by category would scatter the behaviour changes
across both PRs, so neither could merge as "provably equivalent" and neither
would have a clean control to playtest against.

**Tech Stack:** Go, `internal/characters`, `internal/hooks`, `internal/actions`,
`internal/combat`, `internal/usercommands`, `internal/mobcommands`,
`internal/behaviortree`.

---

## The two traps that make this chunk dangerous

Both are silent: they compile, pass every test, pass a boot test, and surface
weeks later in playtest.

### TRAP 1 — `Heal()` is a HARM path. Do not wrap it over `ApplyRestore`.

`buffs.ComputeTickAmount` (`internal/buffs/tick.go:36-38`) returns **`-amount`**
when `percent < 0`. `Heal` (`resources.go:188`) does a bare `c.Health += hp` with
only an upper clamp, so a negative argument is unfloored damage.

Two production sites feed it signed values:
`NewRound_UserRoundTick.go:209` and `NewRound_MobRoundTick.go:198`.

`ApplyRestore` **returns 0 and changes nothing for non-positive input.** So:

```go
func (c *Character) Heal(hp int) int { return c.ApplyRestore(PoolHealth, hp) }
```

would **silently delete every health damage-over-time buff in the game.** Nothing
fails to compile. No test covers a negative-percent health tick. The symptom is
"poison stopped working", noticed much later.

**Therefore: fix the two signed call sites FIRST (Task 3), as their own commit.
`Heal` itself is not touched in this chunk.**

### TRAP 2 — `ApplyHealthChange` cancels combat buffs on death-crossing.

`resources.go:174-178`: when `newHealth < 0` it calls `c.CancelCombatBuffs()`,
which calls `CancelBuffsWithFlag(CancelIfCombat)`, which calls **`Validate(true)`**
and therefore `RecalculateStats`. Verified through the whole chain.

`ApplyHarm` deliberately does none of that. Routing `combat.go`'s **8** call
sites straight to `ApplyHarm` would drop the on-death combat-buff cancel for
every melee kill in the game.

**Therefore: `ApplyHealthChange` survives as a named wrapper (Task 4). Its 8 call
sites are NOT touched.**

---

## Held back for U5b-2 — do NOT do these here

Every one is a real gameplay change. If you find yourself editing these, stop.

1. `combat_helpers.go:562-564` — the defence affordability gate that `continue`s
   unaffordable defences out of the best-of-N candidate set.
2. `DeductDefenseStamina` full-or-refuse → partial.
3. `mobcommands/cast.go:116` — gains a refusal it has never had.
4. **All SEVEN health floors.** `NewRound_AutoHeal.go` mob DoT (2), plus
   `surprise_attack.go`, `skill_moves.go`, `combat_shared_helpers.go` riposte,
   and both `throw.go` sites (5).

   > **CORRECTED MID-EXECUTION 2026-08-13.** Revision 1 of this plan held back
   > only the two mob DoT floors and told Task 2 to delete the other five. That
   > was inconsistent: all seven are structurally identical, and deleting any of
   > them is **not** behaviour-neutral. Health then stores overkill instead of 0,
   > and that is observable -- GMCP ships `Character.Health` raw
   > (`gmcp.Char.go:536`) and the prompt prints it raw
   > (`userrecord.prompt.go:337,340`), so a player who fumbles a grenade onto
   > themselves can briefly see a negative number in their own prompt before the
   > next round tick kills them. Death itself is unaffected; every gate tests
   > `< 1` or `<= 0`.
   >
   > The five sites are **routed** to `ApplyHarm` in U5b-1 but **keep their
   > floor**, each carrying a `NOTE(U5b-2)`. U5b-2 removes all seven together as
   > one named, playtested change.

5. `usercommands/flee.go` and `usercommands/stand.go` cost semantics.

---

## Ground truth, verified 2026-08-13 against master `19c5a5c97`

**107 production pool writes exist**, not the 88 a naive grep finds -- filtering
out `*Max` hides 19 `X = XMax.Value`-shaped spawn/respawn/admin/revive writes.

**31 clamp blocks / 101 physical lines** get deleted, not the ~68 an earlier
estimate suggested.

`modules/` never mutates a pool. Verified.

---

### Task 0: Branch

- [ ] **Step 1**

```bash
git checkout master && git pull origin master
git checkout -b feature/u5b1-route-equivalent-sites
git branch --show-current
```

---

### Task 1: Route the 13 `ApplyRestore` sites

All are unambiguously positive restores with a max clamp the primitive now owns.

**Pattern.** Replace:

```go
	x.Conviction += refund
	if x.Conviction > x.ConvictionMax.Value {
		x.Conviction = x.ConvictionMax.Value
	}
```

with:

```go
	x.ApplyRestore(characters.PoolConviction, refund)
```

(Inside `internal/characters` itself, drop the package qualifier.)

**Files and lines:**

| File | Line | Statement | Clamp to delete |
|---|---|---|---|
| `actions/cast_interrupt.go` | 24 | `target.Conviction += unspent / 2` | 25-27 |
| `behaviortree/actions_combat.go` | 252 | `mob.Character.Conviction += refund` | 253-255 |
| `mobcommands/cancel.go` | 26 | `mob.Character.Conviction += refund` | 27-29 |
| `usercommands/cancel.go` | 30 | `user.Character.Conviction += refund` | 31-33 |
| `hooks/item_procs.go` | 181 | `owner.Conviction += amt` | 182-184 |
| `hooks/NewRound_AutoHeal.go` | 249 | `user.Character.Stamina += staminaRegen` | 250-252 |
| `hooks/NewRound_AutoHeal.go` | 256 | `user.Character.Conviction += convictionRegen` | 257-259 |
| `hooks/NewRound_AutoHeal.go` | 327 | `mob.Character.Health += hpAmt` | 328-330 |
| `hooks/NewRound_AutoHeal.go` | 339 | `mob.Character.Health += hpAmt` | 340-342 |
| `hooks/NewRound_AutoHeal.go` | 356 | `mob.Character.Stamina += spAmt` | 357-359 |
| `hooks/NewRound_AutoHeal.go` | 366 | `mob.Character.Conviction += cpAmt` | 367-369 |
| `hooks/NewRound_DoCombat_unified.go` | 404 | `atkChar.Health += healAmt` | 405-407 |

> Line numbers drift as you edit. **Work bottom-up within each file** and locate
> by the surrounding code, not the number.

**Two sites need more than the pattern:**

- [ ] **`NewRound_DoCombat_unified.go:404` (lifesteal)** -- `healDesc` at `:411`
  currently uses the requested `healAmt`. It must use the APPLIED return, or the
  message overstates healing when the attacker is near full health:

```go
	healed := atkChar.ApplyRestore(characters.PoolHealth, healAmt)
```
  then use `healed` in the message at `:411`.

- [ ] **`hooks/item_procs.go:180-184` is a TRANSFER, not a restore.** `amt` is
  pre-clamped to `other.Conviction` at `:174-176` so drain and gain are equal
  today. Under the primitives the two can differ -- an owner at max CP absorbs
  less than was drained -- which would create or destroy conviction. Keep the
  pre-clamp, route BOTH halves, and feed the harm's applied return into the
  restore:

```go
	drained := other.ApplyHarm(characters.PoolConviction, amt, ownerRef)
	owner.ApplyRestore(characters.PoolConviction, drained)
```

- [ ] **Verify the post-restore reads still work.** `NewRound_AutoHeal.go:266-271`
  and `:374-385` call `OnRegenTick(c.Health, healthMax, ...)` reading all three
  pools immediately after these writes. That is depletion-sensitive stat
  progression. Do not reorder.

- [ ] **Build, test, commit**

```bash
go build ./... && go test ./internal/... 2>&1 | grep -v "^ok" | head
gofmt -l internal/
git add -A && git commit -m "refactor(pools): route the thirteen restore sites onto ApplyRestore (U5b-1)"
```

---

### Task 2: Route the 20 source-available `ApplyHarm` sites

Each has an actor in hand. Build the ref inline:

```go
	src := state.ActorRef{UserId: attacker.GetUserId(), MobInstanceId: attacker.MobInstanceId}
```

`Character.MobInstanceId` is at `character.go:257`; `GetUserId()` at `:446`. For a
self-inflicted effect the source is the character itself.

| File | Line | Statement | Clamp | Source |
|---|---|---|---|---|
| `actions/combat_taunt.go` | 145 | `char.Conviction -= selfDmg` | 146-148 | self |
| `actions/combat_taunt.go` | 224 | `target.Char.Conviction -= dmg` | 225-227 | `actor` |
| `actions/surprise_attack.go` | 296 | `targetChar.Health -= dmg` | 297-299 | `actor` |
| `combat/skill_moves.go` | 94 | `p.Defender.Health -= baseDamage` | 95-97 | `p.Attacker` |
| `hooks/combat_shared_helpers.go` | 220 | `attacker.Health -= dmg` | 221-223 | `defender` |
| `hooks/NewRound_DoCombat_unified.go` | 358 | `defChar.Health -= bonusDmg` | none | `atk` |
| `hooks/NewRound_DoCombat_unified.go` | 389 | `atkChar.Health -= returnDmg` | none | `def` |
| `hooks/spell_resolution.go` | 283 | `user.Character.Health -= backfireDmg` | none | self |
| `hooks/spell_resolution.go` | 412 | `mob.Character.Health -= dmg` | none | `casterChar` |
| `hooks/spell_resolution.go` | 511 | `mob.Character.Health -= dmg` | none | `casterChar` |
| `hooks/spell_resolution.go` | 712 | `user.Character.Health -= backfireDmg` | none | self |
| `hooks/spell_resolution.go` | 792 | `target.Character.Health -= dmg` | none | `user` |
| `hooks/spell_resolution.go` | 1273 | `caster.Character.Health -= dmg` | none | self |
| `hooks/spell_resolution.go` | 1294 | `caster.Character.Health -= dmg` | none | self |
| `hooks/spell_resolution.go` | 1345 | `target.Character.Health -= dmg` | none | `caster` |
| `hooks/spell_resolution.go` | 1410 | `target.Character.Health -= dmg` | none | `caster` |
| `usercommands/inventory.go` | 64 | `user.Character.Health -= dmg` | none | self |
| `usercommands/throw.go` | 170 | `user.Character.Health -= dmg` | 171-173 | self |
| `usercommands/throw.go` | 217 | `mob.Character.Health -= dmg` | 218-220 | `user` |

Plus the anonymous-source harm sites, which pass `state.ActorRef{}`:

| File | Line | Statement | Clamp |
|---|---|---|---|
| `hooks/NewRound_AutoHeal.go` | 81 | `user.Character.Health -= dmg` (toxicity) | none |
| `hooks/NewRound_AutoHeal.go` | 217 | `user.Character.Health -= poisonDmg` | none |
| `hooks/NewRound_AutoHeal.go` | 229 | `user.Character.Health -= bleedDmg` | none |
| `hooks/pinnacle_tick.go` | 175 | `c.Health -= drain` | **KEEP** `:166-168` |

> **`pinnacle_tick.go:166-168` is NOT a pool floor.** It is
> `if c.Health-drain < 1 { drain = c.Health - 1 }` -- a **design floor at 1**
> ensuring sentient-item hunger cannot kill. Keep it; route only the write.

> **DoT sites cannot supply a source.** `buffs.Buff` has no applier field, so
> poison and bleed pass `state.ActorRef{}` and stay anonymous. That is a known
> gap, recorded in the roadmap, not solvable here.

**Three sites keep a result struct in sync -- use the APPLIED return:**

- [ ] **`NewRound_DoCombat_unified.go:358-359`** -- `res.DamageToTarget += bonusDmg`
  feeds `TrackPlayerDamage`, aggro and analytics. Must become:

```go
	res.DamageToTarget += defChar.ApplyHarm(characters.PoolHealth, bonusDmg, atkRef)
```

- [ ] **`hooks/combat_shared_helpers.go:220`** -- `result.RiposteDamage = dmg` at
  `:225` must use the applied return.

- [ ] **`combat/skill_moves.go:94`** -- `result.Damage` is set at `:79`, BEFORE the
  write. Leave that alone; it is pre-set, not synced.

- [ ] **Build, test, commit**

```bash
go build ./... && go test ./internal/... 2>&1 | grep -v "^ok" | head
gofmt -l internal/
git add -A && git commit -m "refactor(pools): route the harm sites onto ApplyHarm (U5b-1)"
```

---

### Task 3: Split the four signed buff-tick branches — THE LOAD-BEARING TASK

**Read Trap 1 again before starting.** These four `+=` statements are **signed**:
`ComputeTickAmount` returns negative for `TickPercent < 0`, and the surrounding
`if > Max ... else if < 0` clamps exist precisely because both directions are
live. Mapping `+=` to `ApplyRestore` deletes every stamina and conviction DoT in
the game.

| File | Line | Statement | Clamp |
|---|---|---|---|
| `hooks/NewRound_MobRoundTick.go` | 200 | `mob.Character.Stamina += buff.TickAmount` | 201-205 |
| `hooks/NewRound_MobRoundTick.go` | 207 | `mob.Character.Conviction += buff.TickAmount` | 208-212 |
| `hooks/NewRound_UserRoundTick.go` | 211 | `user.Character.Stamina += tickAmt` | 212-216 |
| `hooks/NewRound_UserRoundTick.go` | 218 | `user.Character.Conviction += tickAmt` | 219-223 |

- [ ] **Step 1: Split each on sign**

```go
	// tickAmt is SIGNED: buffs.ComputeTickAmount returns a negative value for
	// TickPercent < 0, so this is a damage-over-time delivery path as well as a
	// regen one. Routing it to ApplyRestore alone would silently delete every
	// stamina DoT buff, because ApplyRestore no-ops on non-positive input.
	if tickAmt > 0 {
		user.Character.ApplyRestore(characters.PoolStamina, tickAmt)
	} else if tickAmt < 0 {
		user.Character.ApplyHarm(characters.PoolStamina, -tickAmt, state.ActorRef{})
	}
```

Note `-tickAmt` -- `ApplyHarm` takes a POSITIVE amount.

- [ ] **Step 2: Do the same for the two `case "health":` branches**

`NewRound_UserRoundTick.go:209` and `NewRound_MobRoundTick.go:198` pass the same
signed value into `Heal()`. They have the identical latent bug. Split them the
same way and stop calling `Heal` at these two sites.

**Do NOT change `Heal` itself.** Its other 4 callers are genuinely positive.
`Heal` becomes a wrapper in U5c, once these two are gone.

- [ ] **Step 3: Prove the DoT path still works**

Add to `internal/hooks/` a test asserting a negative-percent tick still reduces
the pool. If the package has no suitable harness, assert at the primitive level
in `internal/characters/pools_test.go` instead:

```go
// TestSignedTickSplit_NegativeStillHarms guards the U5b-1 trap: a signed tick
// amount routed only through ApplyRestore would silently delete every
// damage-over-time buff, because ApplyRestore no-ops on non-positive input.
func TestSignedTickSplit_NegativeStillHarms(t *testing.T) {
	c := poolChar(10, 10, 10)
	tick := -4
	if tick > 0 {
		c.ApplyRestore(PoolStamina, tick)
	} else if tick < 0 {
		c.ApplyHarm(PoolStamina, -tick, state.ActorRef{})
	}
	if c.Stamina != 6 {
		t.Errorf("negative tick: stamina %d, want 6 -- the DoT path is broken", c.Stamina)
	}
	if got := c.ApplyRestore(PoolStamina, -4); got != 0 {
		t.Errorf("ApplyRestore must no-op on negative input, got %d -- if this "+
			"changes, the sign split above is no longer load-bearing", got)
	}
}
```

- [ ] **Step 4: Build, test, commit**

```bash
go build ./... && go test ./internal/... 2>&1 | grep -v "^ok" | head
gofmt -l internal/
git add -A && git commit -m "fix(pools): split the four signed buff ticks into harm and restore (U5b-1)"
```

---

### Task 4: Route the equivalent cost helpers

Only the two that are **exactly** equivalent. The rest are U5b-2.

- [ ] **Step 1: `DeductStamina` (`resources.go:30-36`) → `ApplyCost`**

Body becomes `return c.ApplyCost(PoolStamina, amount)`. Delete the clamp at
`:33-35`. Behaviour identical: both refuse when short, both take nothing.

Keep the function and its 3 callers -- this is a body swap, not a call-site
migration.

- [ ] **Step 2: `DeductAttackStamina` (`resources.go:113-128`) → `ApplyCostPartial`**

Body becomes:

```go
	cost := c.GetAttackStaminaCost()
	return c.ApplyCostPartial(PoolStamina, cost).Charged
```

Delete the partial-spend branch. Behaviour identical: charges what it can,
returns the actual amount. All 4 callers (`combat.go:56,134,213,262`) discard
the return.

- [ ] **Step 3: `ApplyHealthChange` (`resources.go:171-186`) → wrapper**

**Read Trap 2.** Keep the function, keep all 8 call sites, keep the
`CancelCombatBuffs` behaviour. Rewrite the body to delegate the arithmetic while
preserving the side effect:

```go
func (c *Character) ApplyHealthChange(healthChange int) int {
	oldHealth := c.Health

	var applied int
	if healthChange < 0 {
		applied = -c.ApplyHarm(PoolHealth, -healthChange, state.ActorRef{})
	} else {
		applied = c.ApplyRestore(PoolHealth, healthChange)
	}

	// Preserved from the pre-U5b implementation and load-bearing: crossing below
	// zero cancels combat-scoped buffs, which reaches Validate(true) and a full
	// stat recalculation. Eight melee call sites depend on it. ApplyHarm
	// deliberately does not do this, so it stays here.
	if c.Health < 0 {
		c.CancelCombatBuffs()
	}

	_ = oldHealth
	return applied
}
```

> Check the ordering against the original: the old code called
> `CancelCombatBuffs()` **before** writing `c.Health`. Verify whether
> `CancelCombatBuffs` reads `c.Health`; if it does, preserve the original order.
> If it does not, after-write is equivalent and clearer. **Confirm before
> shipping** -- do not assume.

- [ ] **Step 4: Build, test, commit**

```bash
go build ./... && go test ./internal/... 2>&1 | grep -v "^ok" | head
gofmt -l internal/
git add -A && git commit -m "refactor(characters): route the equivalent cost helpers onto the primitives (U5b-1)"
```

---

### Task 5: Grapple upkeep

`Position_GrappleTick.go:733-740` floors both sides at 0. Its own docstring at
`:725-728` says: *"Cost can drive stamina to 0; the character keeps grappling."*
That is `ApplyCostPartial`'s contract verbatim -- the cleanest site in the set.

- [ ] **Step 1: Route both sides**

```go
	controller.ApplyCostPartial(characters.PoolStamina, ctrlCost)
	controlled.ApplyCostPartial(characters.PoolStamina, cdCost)
```

Delete the clamps at `:734-736` and `:738-740`.

- [ ] **Step 2: Preserve the post-deduction reads.** `:336-337` calls
  `fireStaminaWarningIfLow` on both participants after the upkeep. Do not reorder.

- [ ] **Step 3: Build, test, commit**

```bash
go build ./... && go test ./internal/... 2>&1 | grep -v "^ok" | head
git add -A && git commit -m "refactor(hooks): grapple upkeep uses ApplyCostPartial (U5b-1)"
```

---

### Task 6: The guard, with five exemption classes

The guard can only pass once everything above is routed, so it goes last.

- [ ] **Step 1: Create `pool_mutation_guard_test.go` at the repo root**

`package main`. Walk the AST of every production `.go` file under `internal/` and
`modules/`, and fail any **assignment** whose left-hand side is a selector ending
in `.Health`, `.Stamina` or `.Conviction` (excluding `*Max`), unless the file is
exempt.

Model it on the existing `contest_floor_guard_test.go` and
`floor_pair_guard_test.go`: a `map[string]string` of repo-relative path to
written reason, matched by file OR directory prefix.

**The five exemption classes, each with its reason:**

```go
var poolWriteExemptions = map[string]string{
	// Owns the primitives; setPool is the single sanctioned writer.
	"internal/characters/pools.go": "defines the pool primitives",

	// THE CLAMP LAYER. validatePoolClamps, the reservation clamp and the
	// enchant-withdrawal shrink all write pools directly to enforce invariants.
	// Routing them through the primitives would be circular.
	"internal/characters/validate.go": "implements the pool clamps themselves",

	// Construction and spawn-time absolute sets, which run in a defined order
	// relative to Validate() and cannot be expressed as cost, harm or restore.
	"internal/characters/character.go":  "character construction and full-restore",
	"internal/characters/overrides.go":  "ApplyMobOverrides spawn-time set",
	"internal/mobs/mobs.go":             "mob spawn",
	"internal/hooks/PlayerSpawn_HandleJoin.go": "companion re-summon",

	// Respawn sets pools to a FRACTION of max, which is a reduction for anyone
	// above it. No primitive expresses an absolute set to a computed value.
	"internal/hooks/Life_Cascades.go": "respawn sets pools to 5% of max",

	// Admin commands deliberately bypass cost and harm semantics.
	"internal/usercommands/admin.zap.go": "admin set-to-1",
	"internal/usercommands/admin.mob.go": "admin mob heal",
	"internal/usercommands/admin.paz.go": "admin restore",
	"internal/mobcommands/suicide.go":    "ReviveOnDeath full restore",
	"internal/usercommands/suicide.go":   "ReviveOnDeath full restore",

	// A test fixture living in a non-_test.go file, so it compiles into the
	// shipping binary and the walker sees it.
	"internal/users/test_helpers.go": "test fixture in a non-_test.go file",
}
```

- [ ] **Step 2: Run it. Expect failures, and treat each as a finding.**

Anything reported that is NOT in the list above is a site this plan missed. **Do
not add it to the exemption list to make the test quiet.** Report it -- it is
either a routing this plan forgot or a U5b-2 site that should not be here yet.

**The U5b-2 sites WILL be reported.** Add each with the reason
`"U5b-2 routes this; it is a deliberate behaviour change"` so the guard is green
here and U5b-2 removes the exemptions as it routes them:

```go
	// --- Temporary. U5b-2 routes each of these and removes the entry. ---
	"internal/combat/combat_helpers.go":     "U5b-2: defence affordability gate",
	"internal/usercommands/flee.go":         "U5b-2: flee cost semantics",
	"internal/usercommands/stand.go":        "U5b-2: stand's two-knob gate",
	"internal/mobcommands/cast.go":          "U5b-2: mob cast gains a guard",
	"internal/actions/mutation_helpers.go":  "U5b-2: cooldown-rollback ordering",
	// The seven retained health floors. Routed to ApplyHarm in U5b-1 but still
	// flooring, each with a NOTE(U5b-2) at the site. Removing them is
	// observable via GMCP and the prompt, so it is a playtested change.
	"internal/hooks/NewRound_AutoHeal.go":        "U5b-2: mob DoT health floors",
	"internal/actions/surprise_attack.go":        "U5b-2: retained health floor",
	"internal/combat/skill_moves.go":             "U5b-2: retained health floor",
	"internal/hooks/combat_shared_helpers.go":    "U5b-2: retained health floor + fold-upkeep cost",
	"internal/usercommands/throw.go":             "U5b-2: retained health floors",
```

Also note `combat_shared_helpers.go:558` (`char.Conviction -= roundCost`, the
fold-cast upkeep) is a **cost** site guarded at `:546`, so it belongs to U5b-2's
cost work, not here. Its exemption above covers it.

- [ ] **Step 3: Prove it bites**

Temporarily add `user.Character.Stamina -= 1` to a non-exempt file, confirm the
guard FAILS naming that file and line, then revert and confirm it passes.

- [ ] **Step 4: Commit**

```bash
git add pool_mutation_guard_test.go
git commit -m "test(pools): guard against direct pool mutation, with five exemption classes (U5b-1)"
```

---

### Task 7: Documentation and verification

- [ ] **Step 1: Update `internal/characters/context.md`**

Add to the `#### Gotchas` block under `### Resource Pools`:

```markdown
- **`Heal()` is a HARM path at two call sites.** `buffs.ComputeTickAmount`
  returns a negative value for `TickPercent < 0`. Do NOT make `Heal` a thin
  wrapper over `ApplyRestore` -- `ApplyRestore` no-ops on non-positive input, so
  that would silently delete every health damage-over-time buff. U5b-1 split the
  two signed call sites; U5c retires `Heal`.
- **`ApplyHealthChange` is a wrapper, not a legacy path.** It owns the
  `CancelCombatBuffs` on crossing below zero, which reaches `Validate(true)` and
  a full stat recalculation, and 8 melee call sites depend on it. `ApplyHarm`
  deliberately does not do this.
- **Direct pool writes are guarded.** `pool_mutation_guard_test.go` at the repo
  root fails any production assignment to `.Health`/`.Stamina`/`.Conviction`
  outside five declared exemption classes: the primitives themselves, the clamp
  layer, construction/spawn, admin commands, and a test fixture that compiles
  into the binary.
```

- [ ] **Step 2: Audit, scoped**

```bash
python tools/context_md_audit.py internal/characters
```

- [ ] **Step 3: Full verification**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/... 2>&1 | grep -v "^ok" | head -20
go test . -run 'TestOpposedContestsAreFloored|TestMigratedSitesKeepTheirFloorPair|TestPoolMutation' -v
```

- [ ] **Step 4: Confirm the behaviour changes were NOT made**

```bash
git diff master --stat -- internal/combat/combat_helpers.go internal/usercommands/flee.go \
    internal/usercommands/stand.go internal/mobcommands/cast.go internal/actions/mutation_helpers.go
```

Expected: **no output.** Those files belong to U5b-2. If any appears, a
behaviour change leaked into the no-op PR.

Also confirm the mob DoT floors survive:

```bash
grep -n "Health < 1" internal/hooks/NewRound_AutoHeal.go
```

Expected: still present around `:396` and `:410`.

- [ ] **Step 5: Boot test on NON-DEFAULT ports**

```bash
git worktree add --detach C:/tmp/dogmud-u5b1-boot HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-u5b1-boot/_datafiles/config.yaml
```

Edit the worktree copy: `TelnetPort: [43333]`, `LocalPort: 19999`,
`HttpPort: 18090`, `AIPort: 15555`, `Logging.LogToFile: false`.

```bash
cd C:/tmp/dogmud-u5b1-boot && timeout 150 go run . > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log  # want 0
grep -c "Server Ready" boot.log                                         # want 1
grep -icE "bind:|address already in use" boot.log                       # want 0
```

**Exit 124 is success.** Do not grep for the bare word `panic`.

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git worktree remove --force C:/tmp/dogmud-u5b1-boot || rm -rf C:/tmp/dogmud-u5b1-boot
git worktree prune
```

- [ ] **Step 6: Patch notes**

`docs/PATCH_NOTES.md` is newest-first, `## YYYY-MM-DD: Title`, short prose, no
raw numbers, no en or em dashes. U5b-1 changes nothing a player can see.

- [ ] **Step 7: Commit**

```bash
git add internal/characters/context.md docs/PATCH_NOTES.md
git commit -m "docs(pools): document the Heal harm path and the mutation guard (U5b-1)"
```

---

## Ship it

```bash
git push -u origin feature/u5b1-route-equivalent-sites
gh pr create --repo pruuk/DOGMud --base master --head feature/u5b1-route-equivalent-sites --fill
gh pr checks <PR-number> --repo pruuk/DOGMud --watch
gh pr merge  <PR-number> --repo pruuk/DOGMud --merge --delete-branch
```

**Always `--repo pruuk/DOGMud`.** Use `--merge`, not `--squash`.

**No playtest needed for this PR** -- it is behaviour-neutral by construction.
U5b-2 is the one that gets the adversarial playtest run.

**Do not propose a deploy.** The arc is under a deploy gate until U0-U11.

---

## Definition of done

1. All 13 restore sites, ~23 harm sites and the grapple upkeep route through the
   primitives; the four signed buff ticks are split on sign.
2. `Heal` is **unchanged**, and its two signed call sites no longer call it.
3. `ApplyHealthChange` keeps its 8 call sites and its `CancelCombatBuffs`.
4. 31 clamp blocks deleted; `validate.go`'s clamps untouched.
5. `pool_mutation_guard_test.go` passes with five declared exemption classes and
   was proven to bite.
6. **None of the five U5b-2 files was modified** -- verified by diff.
7. A test pins that a negative tick still harms.
8. `context.md` accurate; audit clean; boot test clean on non-default ports;
   `gofmt -l internal/ modules/` silent.
