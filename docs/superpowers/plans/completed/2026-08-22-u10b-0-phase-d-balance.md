# U10b-0 Phase D: Progression Balance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Retune every stat and skill progression multiplier so an hour of
concerted effort on any track yields roughly comparable advancement, and close
the three faucets that make that impossible today.

**Architecture:** Three code changes remove or gate degenerate faucets
(`look`/`consider` ungated perception, per-unit bartering awards, uncapped
conjure), one adds a faucet the design implies but never wired up (observer-side
hidden detection), and one config+Go change applies the solved multipliers.
Tests pinning the old values are repaired, then an adversarial playtest closes
the phase.

**Tech Stack:** Go, `_datafiles/config.yaml`, `internal/skills`,
`internal/actions`, `internal/usercommands`, `tools/balance/*.py`.

**Spec:** `.../2026-08-21-u10b-0-progression-rank-from-training-design.md`
sections 13.3, 13.4, 14.2. **Phase index:** `2026-08-21-u10b-0-README.md`.

**Branch:** `feature/u10b-0-phase-d-balance`, already cut from master.

---

## Revision history

- **Revision 1** — failed blind adversarial review (arithmetic errors).
- **Revision 2** — failed a second blind review. Root cause both times: uses/hour
  were **asserted as free constants** when they are **derived** from cooldown
  keys, gates, equipment exclusivity and the `SkillPrimaryStats` mapping.
- **Revision 3 (this)** — every rate is measured, computed closed-form, or an
  explicit owner ruling, and labelled. Inputs:
  `docs/superpowers/specs/completed/2026-08-22-progression-faucet-census.md`.

## Where the numbers come from

| input | value | source |
|---|---|---|
| clean-hit rate | **0.5752** | 96,723 events / 272 runs, `_datafiles/logs/combat-analytics.jsonl`. **Pre-arc** (ends 2026-08-16); owner accepted with the caveat |
| defence share | dodge 77.0 / parry 15.1 / block 8.0 | same dataset; renormalised to dodge/parry for a no-shield build |
| forage success | >97% by Search rank ~20 | `tools/balance/forage_rate.py`, closed form |
| engagement | combat 10%, gather 100%, craft/salvage/barter 40% | owner ruling — crafting is gather-**then**-craft |
| targets | 3 pts/hr combat, 4 other, at rank 25 | owner ruling |
| difficulty bonus | craft 1.4724, spell 1.2780, manifestation 1.3393 | **means**, not medians — `chance` is linear in the bonus |

**Framing (owner):** *"roughly balance on time given a concerted effort to grind
X or Y."* Each track is solved assuming the player **concentrates** on it. That
is what resolves the shared-cooldown problem: `"special-move"` is one 4-round key
across **eighteen** verbs, so a rhetoric-grinder and a caster spend the *same*
22.5 uses/hour, not two separate budgets.

Regenerate with `python tools/balance/u10b_solve_v3.py`. **Do not transcribe by
hand.**

## Solved multipliers

| track | uses/hr | shipped | **solved** | change |
|---|---|---|---|---|
| weapon-combat | 88.5 | 0.23 | **1.27** | 5.5x |
| unarmed-combat | 162.3 | 0.23 | **0.69** | 3.0x |
| ranged-combat | 22.5 | 0.5 * | **4.98** | 10.0x |
| spellcasting | 22.5 | 0.63 | **3.90** | 6.2x |
| rhetoric | 22.5 | 0.58 | **4.98** | 8.6x |
| manifestation | 25.0 | 0.38 | **4.46** | 11.7x |
| skullduggery | 180.0 | 2.0 * | **0.83** | 0.4x |
| search | 150.0 | 2.0 * | **1.00** | 0.5x |
| bartering | 72.0 | 2.0 | **2.07** | 1.0x |
| salvage | 72.0 | 2.0 | **2.07** | 1.0x |
| the six crafts | 72.0 | 3.5 | **1.41** | 0.4x |
| strength | 104.8 | 0.20 | **0.48** | 2.4x |
| dexterity | 430.8 | 0.15 | **0.12** | 0.8x |
| perception | 162.0 | 1.00 | **0.41** | 0.4x |
| willpower | 22.5 | 1.00 | **2.21** | 2.2x |
| charisma | 94.5 | 0.22 | **0.70** | 3.2x |

`*` = **no `config.yaml` entry today.** These must be **ADDED**, not edited.

**Why `unarmed-combat` lands below `weapon-combat`.** Measured, the offhand fist
earns 1.83x the uses the main weapon does: 4 swings to the longsword's 2 (so
`P(entry clean)` is 0.967 against 0.820, because `CleanHit` is OR-aggregated
across a weapon's swings), plus it collects the dodge award, which is 83.6% of
all defences. Equal multipliers make the empty offhand strictly dominant. This
was a blind-review finding; measurement confirmed it.

**Two rows depend on changes in this plan.** `bartering` 2.07 assumes Task 3
lands; without it bartering is unbounded in time and no multiplier is correct.
`manifestation` 4.46 assumes Task 4 lands; without it manifestation runs at
225/hr and 4.46 over-rewards it ~9x.

---

## File Structure

| file | responsibility |
|---|---|
| `internal/usercommands/look.go` | remove the ungated perception award |
| `internal/actions/consider.go` | remove the ungated perception award |
| `internal/usercommands/go.go` | **add** a Search award on observer-side hidden detection |
| `internal/actions/buy.go`, `sell.go` | bartering: award per transaction, not per unit |
| `internal/actions/cast.go` | conjure gets its own cooldown key |
| `internal/configs/config.balance.go` + `.spells.go` | declare + default `ConjureCooldown` |
| `_datafiles/config.yaml` | the solved multipliers (**skip-worktree — see Task 6**) |
| `internal/skills/skills.go` | the Go-side shadow map, which must match |
| `internal/characters/progression_test.go`, `progression_rank_test.go` | tests pinning old values |

---

### Task 1: Unhook `look` and `consider` from perception

**Why:** The only stat faucets in the game with **no cooldown and no gate**. At
one command per second that is 3,600 perception uses/hour against forage's 150
ceiling — 24x. Confirmed live: the first `consider` issued in the measurement
session printed `STATISTIC INCREASED / perception` immediately.

Perception keeps six feeders via `SkillPrimaryStats`: `ranged-combat`, `search`,
`alchemy`, `cooking`, `enchanting`, `salvage`.

**Files:**
- Modify: `internal/usercommands/look.go:84-85`
- Modify: `internal/actions/consider.go:26-27`
- Create: `internal/actions/consider_no_progression_test.go`

- [ ] **Step 1: Remove the award in `look.go`**

Delete both lines:

```go
		// Track perception use when examining a target
		user.Character.OnStatUse("perception", user.UserId)
```

- [ ] **Step 2: Remove the award in `consider.go`**

From:

```go
func Consider(actor Actor, target Actor) ConsiderResult {
	actor.OnStatUse("perception")

	selfChar := actor.GetCharacter()
```

to:

```go
func Consider(actor Actor, target Actor) ConsiderResult {
	selfChar := actor.GetCharacter()
```

- [ ] **Step 3: Fix the now-false docstring**

Replace the trailing sentence of the doc comment above `Consider`. From:

```go
// is a no-op (existing actor abstraction), so the math runs
// silently. Triggers OnStatUse("perception") on the actor.
```

to:

```go
// is a no-op (existing actor abstraction), so the math runs
// silently.
//
// Deliberately awards NO progression. look and consider were the only stat
// faucets with no cooldown and no gate -- roughly 3,600 perception uses/hour
// against forage's 150 ceiling. Perception is now trained by search/forage,
// the perception crafts, salvage and ranged-combat via SkillPrimaryStats.
// Do not re-add a progression call here.
```

- [ ] **Step 4: Add the regression test**

Create `internal/actions/consider_no_progression_test.go`:

```go
package actions

import (
	"os"
	"strings"
	"testing"
)

// Consider must not award progression. look/consider were the only ungated
// stat faucets in the game (U10b-0 Phase D Task 1); re-adding a progression
// call here reopens a ~24x perception exploit against forage's ceiling.
func TestConsider_AwardsNoProgression(t *testing.T) {
	src, err := os.ReadFile("consider.go")
	if err != nil {
		t.Fatalf("read consider.go: %v", err)
	}
	for _, banned := range []string{"OnStatUse", "OnSkillUse", "CheckStatProgression"} {
		if strings.Contains(string(src), banned) {
			t.Errorf("consider.go calls %s; Phase D removed the ungated perception "+
				"faucet and it must not return", banned)
		}
	}
}
```

- [ ] **Step 5: Verify**

```bash
go build ./... && go test ./internal/actions/... ./internal/usercommands/...
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/usercommands/look.go internal/actions/consider.go \
        internal/actions/consider_no_progression_test.go
git commit -m "fix(progression): unhook look and consider from perception"
```

---

### Task 2: Award Search on observer-side hidden detection

**Why:** The owner's replacement faucet. `go.go:609` and `go.go:629` already run
a full opposed contest when you enter a room holding a hidden player or mob, and
**award nothing today**. Naturally rate-limited: needs a hidden actor, fires at
most once per room entry per hidden actor.

Award the **Search skill**, not raw perception. `search`'s own description is
*"Finding hidden things -- and foraging the wild for resources"*, and
`SkillPrimaryStats["search"] == "perception"`, so the perception roll happens
automatically and consistently with forage.

**Files:**
- Modify: `internal/usercommands/go.go` (both detection branches)

- [ ] **Step 1: Award on detecting a hidden player**

In the hidden-player branch, inside `if success {`, after the `user.SendText(...)`
that reports the notice:

```go
						// Winning the observer side of a hidden-detection contest
						// trains Search (primary stat perception). Opportunity-gated:
						// requires a hidden actor, at most once per room entry.
						user.Character.OnSkillUse(string(skills.Search), user.UserId)
```

- [ ] **Step 2: Award on detecting a hidden mob**

In the hidden-mob branch, inside `if success {`, after the `destRoom.SendText(...)`:

```go
						user.Character.OnSkillUse(string(skills.Search), user.UserId)
```

- [ ] **Step 3: Confirm the import**

`go.go` already uses `skills.Search` at line 388, so no import change is needed:

```bash
grep -n 'internal/skills' internal/usercommands/go.go
```
Expected: one import line.

- [ ] **Step 4: Verify**

```bash
gofmt -l internal/ modules/   # must print nothing
go build ./... && go test ./internal/usercommands/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/usercommands/go.go
git commit -m "feat(progression): train Search on observer-side hidden detection"
```

---

### Task 3: Bartering awards per transaction, not per unit

**Why:** `buy.go` and `sell.go` award **inside the per-unit loop** with no
cooldown, so `sell all` on a 200-item stack fires 200 uses from one command.
Bartering is therefore not time-bound and **no uses/hour can be fitted to it**.
The solved 2.07 assumes this is fixed.

**Files:**
- Modify: `internal/actions/buy.go` (`postSuccessBookkeeping` + 3 call sites)
- Modify: `internal/actions/sell.go:376-378`
- Create: `internal/actions/bartering_per_transaction_test.go`

- [ ] **Step 1: Gate the buy award**

From:

```go
func postSuccessBookkeeping(buyer Actor, shopMob *mobs.Mob, shopUser *users.UserRecord) {
	buyer.OnSkillUse("bartering")
```

to:

```go
// awardProgression is true only for the FIRST unit of a multi-buy. Bartering
// used to award per unit with no cooldown, so `buy 200 x` fired 200 rolls from
// one command and no uses/hour could be fitted to it (Phase D Task 3).
func postSuccessBookkeeping(buyer Actor, shopMob *mobs.Mob, shopUser *users.UserRecord,
	awardProgression bool) {
	if awardProgression {
		buyer.OnSkillUse("bartering")
	}
```

- [ ] **Step 2: Update the three call sites**

All three sit inside the `for purchased < quantity` loop:

```go
				postSuccessBookkeeping(buyer, nil, shopUser, purchased == 0)
```
```go
					postSuccessBookkeeping(buyer, shopMob, nil, purchased == 0)
```

Apply the same to the third. **Read the surrounding loop** — if the counter is
named differently in scope, use the expression that is true only on the first
iteration. Do not guess.

- [ ] **Step 3: Gate the sell award**

In `sell.go`, inside `for out.Sold < quantity`, change:

```go
	// Progression.
	seller.OnSkillUse(string(skills.Bartering))
	mob.Character.OnStatUse("charisma", 0)
```

to:

```go
	// Progression. FIRST unit only -- awarding per unit made `sell all` on a
	// 200-item stack fire 200 rolls from one command (Phase D Task 3).
	if out.Sold == 0 {
		seller.OnSkillUse(string(skills.Bartering))
		mob.Character.OnStatUse("charisma", 0)
	}
```

- [ ] **Step 4: Write a real regression test**

Create `internal/actions/bartering_per_transaction_test.go` asserting that a
multi-unit buy and a multi-unit sell each award bartering **once**. Use the shop
fixture `buy_test.go` already provides. If no usable fixture exists, assert
structurally with `go/ast` that both award sites sit inside a first-iteration
guard, mirroring the AST-assertion style in `combat_reload_test.go`.

**Do not ship a `t.Skip`.**

- [ ] **Step 5: Verify**

```bash
go build ./... && go test ./internal/actions/...
```

- [ ] **Step 6: Commit**

```bash
git add internal/actions/buy.go internal/actions/sell.go \
        internal/actions/bartering_per_transaction_test.go
git commit -m "fix(progression): bartering awards per transaction, not per unit"
```

---

### Task 4: Give conjure its own cooldown key

**Why:** Measured, the cheapest manifestation path (`conjure-water`: 30 CP,
difficulty 15, `waitrounds` 2, **no corpse, no target**) runs at the shared
`special-move` ceiling of **225/hr** even outside a sanctuary, standing still, at
~100% engagement. `dismiss` has no cooldown and costs no slot, so summon/dismiss
is free. Manifestation is currently one of the *easiest* tracks to grind.

Owner ruling: **a cast cooldown on its own key**, sized so conjuring runs at
about the raise+assess rate (~2 manifestation uses per corpse, so corpse-bound =
kill rate). A separate key means conjuring does not consume a combat slot.

**The exact value needs the kill-rate measurement the failed session owed**, so
it ships as a **config knob**, not a literal. Default **36 rounds** (25 casts/hr,
midpoint of the 18–36/hr band implied by 5–10 round fights at 10% engagement).

**Files:**
- Modify: `internal/configs/config.balance.go`
- Modify: `internal/configs/config.balance.spells.go`
- Modify: `internal/actions/cast.go:279`
- Modify: `internal/actions/command_readiness_drift_test.go`

- [ ] **Step 1: Declare the knob**

In `internal/configs/config.balance.go`, beside the other spell knobs:

```go
	ConjureCooldown                ConfigInt   `yaml:"ConjureCooldown"`                // Rounds between conjure/summon casts, on their own key (default 36)
```

- [ ] **Step 2: Default it**

In `internal/configs/config.balance.spells.go`:

```go
	// `<= 0`, not `< 0`: a pacing floor, not an off-switch. Conjure spells need
	// no corpse and no target, so with no cooldown they run at the shared
	// special-move ceiling (225/hr) standing still, which made manifestation the
	// easiest track in the game to grind.
	if b.ConjureCooldown <= 0 {
		b.ConjureCooldown = 36
	}
```

- [ ] **Step 3: Verify the spell accessors BEFORE writing Step 4**

```bash
grep -n "Schools\|SummonRequiresCorpse" internal/spells/*.go
```

The YAML fields are `schools:` (a list) and `summon_requires_corpse:`. Use the
**real** Go names this prints. Do **not** invent an accessor.

- [ ] **Step 4: Branch the cooldown in the cast path**

`internal/actions/cast.go:279` currently reads:

```go
		if !char.TryCooldown(`special-move`, fmt.Sprintf(`%d rounds`, cfg.SpecialMoveCooldown)) {
```

Replace with (substituting the accessor names from Step 3):

```go
		// Conjure/summon spells need no corpse and no target, so they cannot
		// share the combat special-move budget -- that let them run at 225/hr
		// standing still. They take their own, much longer key. Raise spells are
		// excluded: they are already corpse-bound and waitrounds-bound.
		cooldownKey := `special-move`
		cooldownRounds := int(cfg.SpecialMoveCooldown)
		if spellData != nil && spellHasSchool(spellData, spells.SchoolManifestation) &&
			!spellData.SummonRequiresCorpse {
			cooldownKey = `conjure`
			cooldownRounds = int(cfg.ConjureCooldown)
		}
		if !char.TryCooldown(cooldownKey, fmt.Sprintf(`%d rounds`, cooldownRounds)) {
```

- [ ] **Step 5: Update the cooldown-drift guard test**

`command_readiness_drift_test.go` asserts exact
`TryCooldown("special-move", ...)` call positions per file. `cast.go` now uses a
variable key, so that assertion fails. Update the expectation for `cast.go`
**only**, with a comment saying why it is exempt.

- [ ] **Step 6: Add the knob to `config.yaml`**

(This file has skip-worktree — commit it in Task 6, not here.)

```yaml
  # ConjureCooldown: rounds between conjure/summon casts. Their OWN key, not the
  # shared special-move budget. Conjure needs no corpse and no target, so an
  # unthrottled conjure runs at 225/hr standing still. 36 rounds paces it to
  # roughly the raise+assess rate. Retune after the kill-rate measurement.
  ConjureCooldown: 36
```

- [ ] **Step 7: Verify**

```bash
gofmt -l internal/ modules/
go build ./... && go test ./internal/actions/... ./internal/configs/...
```

- [ ] **Step 8: Commit**

```bash
git add internal/configs/config.balance.go internal/configs/config.balance.spells.go \
        internal/actions/cast.go internal/actions/command_readiness_drift_test.go
git commit -m "feat(balance): conjure gets its own cooldown key"
```

---

### Task 5: Apply the solved multipliers to the Go map

**Why the Go map matters:** `GetSkillProgressionMultiplier` returns `(0, false)`
on a config miss, meaning *"use the hardcoded default"*, and
`skills.GetProgressionMultiplier` falls back to `SkillProgressionMultipliers` in
`internal/skills/skills.go`. **Every test binary** takes that path, because tests
never load `config.yaml`. Stats have no such shadow —
`GetStatProgressionMultiplier` returns 1.0 on a miss.

The two maps **disagree today** (Go `WeaponCombat: 0.3` vs config `0.23`). Bring
them into line rather than preserving the split.

**Files:**
- Modify: `internal/skills/skills.go:354-378`

- [ ] **Step 1: Replace the map**

```go
// Solved on measured play-time rates, U10b-0 Phase D revision 3. Regenerate with
// `python tools/balance/u10b_solve_v3.py`; do not hand-edit.
// MUST stay in sync with SkillProgressionMultipliers in _datafiles/config.yaml
// -- this map is what every test binary sees, since tests never load config.
var SkillProgressionMultipliers = map[SkillTag]float64{
	// Melee. unarmed sits BELOW weapon deliberately: the offhand fist takes 4
	// swings to a longsword's 2 and collects the dodge award (83.6% of all
	// defences), so it earns 1.83x the uses. Equal values make the empty
	// offhand strictly dominant.
	WeaponCombat:  1.27,
	UnarmedCombat: 0.69,
	// These three share ONE 4-round "special-move" key with 15 other verbs, so a
	// concerted grinder gets only ~22.5 uses/hour.
	RangedCombat:  4.98,
	Spellcasting:  3.90,
	Rhetoric:      4.98,
	// Assumes the Phase D conjure cooldown is in place. Without it manifestation
	// runs at 225/hr and this over-rewards it ~9x.
	Manifestation: 4.46,
	// Utility. Assumes bartering awards per transaction, not per unit.
	Search:        1.00,
	Bartering:     2.07,
	Skullduggery:  0.83,
	Salvage:       2.07,
	// Crafts, at 40% engagement (gather THEN craft).
	Blacksmithing: 1.41,
	Alchemy:       1.41,
	Tailoring:     1.41,
	Cooking:       1.41,
	Jewelcrafting: 1.41,
	Enchanting:    1.41,
}
```

- [ ] **Step 2: Verify the build**

```bash
gofmt -l internal/ modules/
go build ./...
```
Tests are repaired in Task 7; expect failures there until then.

- [ ] **Step 3: Commit**

```bash
git add internal/skills/skills.go
git commit -m "balance(progression): solved skill multipliers in the Go map"
```

---

### Task 6: Apply the solved multipliers to `config.yaml`

**⚠️ `_datafiles/config.yaml` carries `skip-worktree`.** `git add` fails with a
misleading "sparse-checkout" error, and the on-disk copy silently diverges from
the committed blob in **both** directions. It was found a full commit behind on
2026-08-22, missing 14 keys including both crit knobs.

**Files:**
- Modify: `_datafiles/config.yaml`

- [ ] **Step 1: Clear the skip-worktree bit and check for drift**

```bash
git update-index --no-skip-worktree _datafiles/config.yaml
git diff --stat _datafiles/config.yaml
```
Expected: **no diff output**. If anything prints, the disk copy drifted again —
reconcile against `git show HEAD:_datafiles/config.yaml` **before** editing, and
preserve the intentional local overrides (`HttpPort`, `LogLevel`) and the
uncommitted `Playtest:` block.

- [ ] **Step 2: Set the skill multipliers**

Under `Balance.SkillProgressionMultipliers`:

```yaml
    weapon-combat: 1.27
    unarmed-combat: 0.69
    ranged-combat: 4.98      # NEW KEY -- absent today; Go map was authoritative
    spellcasting: 3.90
    rhetoric: 4.98
    manifestation: 4.46
    search: 1.00             # NEW KEY
    skullduggery: 0.83       # NEW KEY
    bartering: 2.07
    salvage: 2.07
    blacksmithing: 1.41
    alchemy: 1.41
    tailoring: 1.41
    cooking: 1.41
    jewelcrafting: 1.41
    enchanting: 1.41
```

- [ ] **Step 3: Set the stat multipliers, and add `ConjureCooldown`**

Under `Balance.StatProgressionMultipliers`:

```yaml
    strength: 0.48
    dexterity: 0.12
    perception: 0.41
    willpower: 2.21
    charisma: 0.70
    vitality: 4.5            # UNCHANGED -- see note
```

Also add the `ConjureCooldown: 36` block from Task 4 Step 6.

**`vitality` is deliberately not retuned.** Its faucets are the regen damper and
the crit-toughen path, neither a uses/hour track, so the solver has no row for
it. It reads ~52%/use on the branch and needs its own slice. This is a stated
gap, not an oversight.

- [ ] **Step 4: Commit, then restore the bit**

```bash
git add _datafiles/config.yaml
git commit -m "balance(progression): solved multipliers in config.yaml"
git update-index --skip-worktree _datafiles/config.yaml
git ls-files -v _datafiles/config.yaml   # expect a leading "S"
```

- [ ] **Step 5: Confirm both sources agree**

```bash
python - <<'PY'
import re, io, yaml
cfg = yaml.safe_load(io.open('_datafiles/config.yaml', encoding='utf-8'))
cfgm = cfg['Balance']['SkillProgressionMultipliers']
go = io.open('internal/skills/skills.go', encoding='utf-8').read()
block = go.split('SkillProgressionMultipliers = map[SkillTag]float64{')[1].split('\n}')[0]
pairs = dict(re.findall(r'(\w+):\s*([0-9.]+)', block))
tag = dict(re.findall(r'(\w+)\s+SkillTag = `([^`]+)`', go))
bad = 0
for k, v in pairs.items():
    name = tag.get(k)
    if name and abs(float(v) - float(cfgm.get(name, -1))) > 1e-9:
        print('MISMATCH %-16s go=%s config=%s' % (name, v, cfgm.get(name)))
        bad += 1
print('OK, both sources agree' if not bad else '%d mismatches' % bad)
PY
```
Expected: `OK, both sources agree`.

---

### Task 7: Repair the tests that pin the old values

**Files:**
- Modify: `internal/characters/progression_test.go:258-277`
- Modify: `internal/characters/progression_rank_test.go:92-110`

- [ ] **Step 1: Run them and read the failures**

```bash
go test ./internal/characters/... -run 'TestGetProgressionMultiplier|TestStatChance_ReproducesTheDocumentedAnchors' -v
```
Expected: both FAIL. `TestGetProgressionMultiplier` hard-asserts
`0.3/0.3/0.5/2.0/2.0/2.0/1.0`; the anchor test calls `withRepoRoot` (so it loads
the real config) and pins perception at multiplier 1.0.

- [ ] **Step 2: Update the hard-asserted values**

Replace each expected value with the Task 5 figure, and add above the table:

```go
	// Values are solved, not chosen: `python tools/balance/u10b_solve_v3.py`.
	// If this fails after a retune, regenerate rather than editing to fit.
```

- [ ] **Step 3: Re-anchor the perception case**

`TestStatChance_ReproducesTheDocumentedAnchors` pins perception at 1.0, which
Task 6 changes to 0.41. Update the expected chance, keeping the test's
**structure** — it verifies the curve, not the constant.

- [ ] **Step 4: Full suite**

```bash
export GOTMPDIR=C:/gotmp
go test ./... 2>&1 | grep -v '^ok' | head -30
```
`go test ./...` is known to exit 0 with no known failures, so **any** failure
here is real.

- [ ] **Step 5: Commit**

```bash
git add internal/characters/progression_test.go internal/characters/progression_rank_test.go
git commit -m "test(progression): re-anchor multiplier tests to the solved values"
```

---

### Task 8: Adversarial playtest gate

**Required by the Content Playtest-Review Gate SOP.** This phase changes what
every player's advancement feels like; a clean boot proves nothing about it.

- [ ] **Step 1: Pre-push gates**

```bash
gofmt -l internal/ modules/          # nothing
go build ./...
export GOTMPDIR=C:/gotmp && go test ./...
```

- [ ] **Step 2: Update `docs/PATCH_NOTES.md`**

Dated entry, player-facing framing, **no raw numbers, no em dashes**. Cover:
practising a neglected skill advances it noticeably faster; examining and sizing
up creatures no longer sharpens your senses by itself, but spotting something
hidden does; conjured allies need longer between summonings; haggling rewards
the deal rather than the item count.

- [ ] **Step 3: Boot test in an isolated worktree**

Per the pre-push SOP — detached worktree, **shifted ports** so the user's live
server is untouched, build to a fixed `boot-check.exe` path. **Exit code 124 is
the success case.** Never grep the bare word `panic`
(`MapConsistencyEnforce: panic` is a legitimate value).

- [ ] **Step 4: Wipe instance saves before smoke testing**

```bash
rm -rf _datafiles/world/dogmud/mobs.instances/* \
       _datafiles/world/dogmud/rooms.instances/*
```
Do **not** touch `shops/`, `guilds/` or `moderation/`.

- [ ] **Step 5: Run the adversarial playtest**

⚠️ **Do not reuse `mid.yaml` at room 462.** The Phase D measurement session
failed on exactly that: room 462 is an urban hub eight moves from any hostile,
the first matched enemy fled, and the only reachable enemy died in one round.
Author a goals file with a **wilderness start room** and a budget that leaves
time for actual play.

```text
/playtest local --checkout <abs> feel-tester <new-goals-file>.yaml
```

The question is whether advancement *feels* fair across activities.

- [ ] **Step 6: Fix what it finds, re-run if needed, then hand to the user**

---

## Out of scope, filed rather than fixed

- **`vitality` at 4.5** — ~52%/use, needs its own slice (Task 6 Step 3).
- **`charge` records 95 events at a 0.0% hit rate** in the analytics. It is a
  **mob-only** ability (`internal/mobcommands/charge.go`; `divergences.go:177`
  marks it `mob-ai`), so a large attacker/defender skill gap is a plausible
  explanation — every mob is combat skill 1. But note the arithmetic before
  dismissing it: at a true 5% hit rate, P(0 hits in 95 attempts) is 0.8%; it
  needs a true rate near 2% for luck alone to be comfortable. Worth a look at
  whether `charge` reports `Hit` correctly to `RecordSpecialMove` at all —
  `mobcommands/charge.go` contains no `RecordSpecialMove` call, so find who
  records it and with what.
- 🚩 **`TestTaunt_StalePlayerIdInRoom_StillMessages` is FLAKY** (`internal/usercommands`).
  Measured during Task 3: **2 of 4** full-package runs fail on a tree WITHOUT any
  Phase D change, and 1 of 6 with. It passes 3/3 when run alone, so it is
  order-dependent, not logic-dependent. **This invalidates the standing
  assumption that `go test ./...` has no known failures and therefore that any
  failure is real.** Anyone gating on a green suite needs to know this one lies.
  Not a Phase D regression; do not chase it inside this phase.
- **`"Your fists flies wide"`** — subject/verb disagreement in unarmed crit
  narration, found in the measurement session.
- **`playtestrun stop` does not stop the container** — known harness bug,
  re-confirmed; orphans need `docker rm -f`.
- **The `Playtest:` block is live but uncommitted** — 16 Go references, on-disk
  only, so a fresh clone or prod has no Playtest config.
- **The clean-hit rate is pre-arc.** 0.5752 predates U6b/U9/U10 and Phases
  A/B/C. Owner accepted this. A post-arc re-measurement should revisit the
  combat rows.
