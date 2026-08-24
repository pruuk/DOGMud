# U10c Charm Redesign Implementation Plan (v2)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make charm a companion with a clock — it stops running a private contest and instead consumes the seam contest it was already running, its duration is bought with the margin of victory, and when the bond ends mid-session the creature turns on the caster.

**Architecture:** Charm declares `target_defense_type: social`, which routes the cast it *already makes* through `ResolveChannelAttack(ChannelSocial)`. The charm effect moves into `applyMobEffect`'s switch, where that contest's result is already in scope, and the private `RunContest` in `resolveCharmSpell` is deleted. Net: **one fewer contest than today**, not one more.

**Tech Stack:** Go — `internal/combat` (seam), `internal/hooks` (spell resolution, round tick), `internal/characters` (companion state), `internal/mobs` (instance saves), `internal/configs` (balance knobs); YAML data files and ANSI help templates.

**Spec:** `docs/superpowers/specs/2026-08-24-u10c-charm-redesign-design.md` — **read sections 11, 12 and 13 FIRST.** They supersede 1-10, section 2.5 is known false, and section 13 supersedes 11.3.2.

---

## v1 was reviewed and rejected. Read this before Task 1.

Three blind adversarial reviewers each made the same finding their number one. v1 of this plan was built on a false premise and would have shipped a silent defect. The facts below are all verified; do not re-derive them, and do not revert to v1's shape.

1. **Charm ALREADY resolves through the seam.** `spellAttackChannel`
   (`spell_resolution.go:1076-1081`) maps an **absent** `target_defense_type` to
   `ChannelSpellMental` — absent is the DEFAULT, not an escape. The mob-target
   loop (`:129-143`) calls `resolveAgainstMob` unconditionally; only the
   *player* loop has an effect-type guard. `spellAttackSideFor` already builds
   `Cha + manifestation × SkillWeight`.
   **v1 proposed adding a second `ResolveChannelAttack` call. Do not.**

2. **`applyMobEffect` already receives the contest result.** Its signature ends
   `out combat.ChannelDefenceResult` (`spell_resolution.go:811`) and it
   switches on `spellData.EffectType` (`:822`). Charm currently falls to
   `default`, which narrates and returns 0 — the verdict is thrown away, and
   then the post-loop `resolveCharmSpell` (`:226-234`) runs a private contest
   to decide the real outcome. **That is the actual defect.**

3. **`AttackSide.score()` computes the score itself** —
   `(Stat + SkillRank*SkillWeight) * Mult`, reading `SkillWeight` from config.
   The old ×25 is not corrected so much as made inexpressible.

4. **The seam does not surface the opposed margin on an attack WIN.**
   `NormalizedDefenceMargin` is assigned *below* `if res.Success { return out }`,
   so it is zero exactly when charm needs it. Task 1 fixes this.

5. **The margin sign is a documented footgun.** `contest.Result.Margin` is
   ATTACK-positive; `bestDefenseResult.margin` is DEFENCE-positive. The contest
   package's docs say mixing them "compiles cleanly and silently puts crit on
   the losing side."

6. **Rounds-at-zero means two different things.** Logout
   (`PlayerDespawn_HandleLeave.go:136-142`) and link-dead
   (`users.go:347-352`, on the **tutorial Guide mob**) both signal by setting
   `RoundsRemaining = 0` — the same state a natural expiry produces. Spec 13.4.

7. **Five other systems set `Charmed`** — summons
   (`companion_summon.go:107`), brood spawns
   (`manifester_companions.go:98`), the homunculus
   (`chrysifier_homunculus.go:160`), `befriend`
   (`mobcommands/befriend.go:51`), and behaviour-tree companions
   (`behaviortree/actions_mob.go:123`). **None may ever produce a grudge.**

8. **The reservation is applied during `RecalculateStats()`**
   (`validate.go:261-284`). `RemoveCompanion` alone does not release it —
   v1 said it did, and that was wrong.

9. **`contest_site_guard_test.go` names U10c as the owner of two allowlist
   rows** (`:76-77`) and asserts both directions. Deleting the contest sites
   without deleting the rows turns `internal/combat` red.

---

## File Structure

| File | Responsibility | Change |
|---|---|---|
| `internal/combat/defence_multiplier.go` | The seam | Modify — add `AttackerNormalizedMargin` |
| `internal/combat/channel_attacker_margin_test.go` | Sign, zeroing, scaling of the new field | Create |
| `internal/combat/contest_site_guard_test.go` | Arc-wide contest-site allowlist | Modify — remove two rows, add a legacy-literal entry |
| `internal/configs/config.balance.go` | Field declarations | Modify — two `ConfigInt` |
| `internal/configs/config.balance.mobs.go` | Defaulting | Modify |
| `internal/hooks/charm_duration.go` | Pure margin→rounds | Create |
| `internal/hooks/charm_duration_test.go` | Monotonic, clamped, floored-takes-Min | Create |
| `internal/hooks/spell_resolution.go` | Routing + effect dispatch | Modify — `social` channel case, `case "charm":`, delete the post-loop block |
| `internal/hooks/charm_spell.go` | The charm effect | Modify — delete the private contest; become the effect arm |
| `internal/hooks/charm_effect_test.go` | Cast-path behaviour, single-contest guard | Create |
| `internal/hooks/NewRound_MobRoundTick.go` | Expiry | Modify — delete ladder, promote grudge, gate it |
| `internal/hooks/charm_expiry_test.go` | Grudge, source-type, logout, absent caster | Create |
| `internal/mobs/instance_save.go` | Instance persistence | Modify — `EverCharmed` guard |
| `internal/mobs/charm_immune_shops_test.go` | Shopkeeper regression guard | Create |
| `internal/characters/companions.go` | `CompanionInfo` | Modify — delete `CharmDuration`, `CharmRerolls` |
| `_datafiles/world/dogmud/spells/charm.yaml` | Spell data + description | Modify — add `target_defense_type: social`, rewrite description |
| `_datafiles/world/dogmud/templates/help/charm.template` | Player help | Modify |
| `_datafiles/config.yaml` | Shipped knobs | Modify — **via the `git show HEAD:` blob** |
| `internal/combat/context.md`, `internal/hooks/context.md` | Package docs | Modify |

---

### Task 1: Surface the attack-win margin on the seam

**Files:** Modify `internal/combat/defence_multiplier.go`; create `internal/combat/channel_attacker_margin_test.go`.

Additive change to shared combat code. Must not alter any existing field's behaviour.

- [ ] **Step 1: Find the real test fixture first**

v1 invented a helper that does not exist. The two real fixtures in package `combat` are:

```bash
grep -n "func defenceFixture\|func defenceAdmissionCharacters" internal/combat/*_test.go
```

`defenceFixture(defenderStamina int)` (`defense_affordability_test.go:19`) and `defenceAdmissionCharacters()` (`defence_admission_test.go:53`). **Neither takes a `*testing.T`.** Use `defenceAdmissionCharacters()` — it is the only one that gives the defender a rhetoric rank, which `ChannelSocial` needs since defy is the sole entry in that set.

- [ ] **Step 2: Write the failing test**

```go
package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
)

// AttackerNormalizedMargin is the attack-positive twin of
// NormalizedDefenceMargin. They populate on OPPOSITE paths and neither
// substitutes for the other; contest.Result.Margin's own docs warn that mixing
// the two conventions compiles cleanly and inverts the outcome.
func TestAttackerNormalizedMargin_PopulatedOnlyOnAttackWin(t *testing.T) {
	attacker, defender := defenceAdmissionCharacters()

	restore := SetChannelAttackContestRunnerForTest(
		func(atk float64, entries []contest.Entry) contest.Result {
			name := ""
			if len(entries) > 0 {
				name = entries[0].Name
			}
			return contest.Result{
				Success: true, Contested: true, Winner: name,
				Margin:      30,
				AttackRoll:  dice.RollResult{StdDev: 10},
				DefenseRoll: dice.RollResult{StdDev: 10},
			}
		})
	defer restore()

	out := ResolveChannelAttack(ChannelSocial, AttackSide{Stat: 100}, attacker, defender)

	want := 30.0 / (10.0 * math.Sqrt2)
	if math.Abs(out.AttackerNormalizedMargin-want) > 1e-9 {
		t.Errorf("AttackerNormalizedMargin = %v, want %v", out.AttackerNormalizedMargin, want)
	}
	if out.AttackerNormalizedMargin <= 0 {
		t.Error("a decisive ATTACK win must be POSITIVE; the sign is inverted")
	}
	if out.NormalizedDefenceMargin != 0 {
		t.Errorf("NormalizedDefenceMargin = %v, want 0 on an attack win", out.NormalizedDefenceMargin)
	}
}

func TestAttackerNormalizedMargin_ZeroWhenDefenceWon(t *testing.T) {
	attacker, defender := defenceAdmissionCharacters()

	restore := SetChannelAttackContestRunnerForTest(
		func(atk float64, entries []contest.Entry) contest.Result {
			name := ""
			if len(entries) > 0 {
				name = entries[0].Name
			}
			return contest.Result{
				Success: false, Contested: true, Winner: name,
				Margin:      -30,
				AttackRoll:  dice.RollResult{StdDev: 10},
				DefenseRoll: dice.RollResult{StdDev: 10},
			}
		})
	defer restore()

	out := ResolveChannelAttack(ChannelSocial, AttackSide{Stat: 100}, attacker, defender)
	if out.AttackerNormalizedMargin != 0 {
		t.Errorf("AttackerNormalizedMargin = %v, want 0 when the defence won", out.AttackerNormalizedMargin)
	}
}

// A floored outcome stamps a +-1 SENTINEL margin, not a roll. Reporting it
// would let a mercy-granted win read as dominance.
func TestAttackerNormalizedMargin_ZeroWhenFloored(t *testing.T) {
	attacker, defender := defenceAdmissionCharacters()

	restore := SetChannelAttackContestRunnerForTest(
		func(atk float64, entries []contest.Entry) contest.Result {
			name := ""
			if len(entries) > 0 {
				name = entries[0].Name
			}
			return contest.Result{
				Success: true, Contested: true, Floored: true, Winner: name,
				Margin:      1,
				AttackRoll:  dice.RollResult{StdDev: 10},
				DefenseRoll: dice.RollResult{StdDev: 10},
			}
		})
	defer restore()

	out := ResolveChannelAttack(ChannelSocial, AttackSide{Stat: 100}, attacker, defender)
	if out.AttackerNormalizedMargin != 0 {
		t.Errorf("AttackerNormalizedMargin = %v, want 0 on a floored win", out.AttackerNormalizedMargin)
	}
}
```

- [ ] **Step 3: Run to verify it fails**

```bash
go test ./internal/combat/ -run TestAttackerNormalizedMargin -v
```

Expected: FAIL — `out.AttackerNormalizedMargin undefined`.

- [ ] **Step 4: Add the field**

Beneath `NormalizedDefenceMargin` in `ChannelDefenceResult`:

```go
	// AttackerNormalizedMargin is the ATTACK-POSITIVE opposed margin, populated
	// only on the res.Success return. NormalizedDefenceMargin is its
	// defence-positive counterpart, populated only when the defence won, so
	// neither substitutes for the other and neither is meaningful on the
	// other's path. contest.Result.Margin's docs record why that matters:
	// mixing the conventions compiles cleanly and inverts the outcome.
	//
	// Zero on a floored win — the margin is then a +-1 sentinel, not a roll.
	//
	// ALSO ZERO on three other attack-win exits that never reach that return:
	// an empty defence set, an uncontested roll, and the ForceCrit forced win.
	// A consumer that must distinguish "no margin" from "zero margin" needs
	// more than this field.
	//
	// Added by U10c for charm's duration; the gap is general, not charm-shaped.
	AttackerNormalizedMargin float64
```

- [ ] **Step 5: Populate it — at line 386, NOT 424**

```bash
grep -n "if res.Success {" internal/combat/defence_multiplier.go
```

**This returns TWO matches.** `:386` is inside `resolveChannelAttackWithRunner` and returns `out` — that is the one. `:424` is inside `defenceDamageMultiplier`, returns `1.0`, and has no `out` in scope; patching it is a compile error.

```go
	if res.Success {
		if !res.Floored && res.DefenseRoll.StdDev > 0 {
			out.AttackerNormalizedMargin = res.Margin / (res.DefenseRoll.StdDev * math.Sqrt2)
		}
		return out
	}
```

- [ ] **Step 6: Verify, and prove nothing else changed**

```bash
go test ./internal/combat/ -run TestAttackerNormalizedMargin -v
go test ./internal/combat/ ./internal/hooks/ ./internal/actions/ -count=1
```

Expected: the three new tests PASS; everything else `ok`. Any other failure means the return was altered rather than extended.

- [ ] **Step 7: Commit**

```bash
git add internal/combat/defence_multiplier.go internal/combat/channel_attacker_margin_test.go
git commit -m "feat(combat): surface the attack-positive margin on a channel win

NormalizedDefenceMargin is assigned below the 'if res.Success { return out }'
early return, so the opposed margin was zero exactly when the ATTACKER won.
Any effect scaling with how decisively an attack landed had nothing to read.

Additive, with the same guards the neighbouring DefenseRollZScore uses. The
doc comment names the three other attack-win exits that also leave it zero, so
a future consumer is not misled by the common case."
```

---

### Task 2: The duration knobs

**Files:** Modify `internal/configs/config.balance.go`, `internal/configs/config.balance.mobs.go`, `_datafiles/config.yaml`.

- [ ] **Step 1: Declare the fields** — beside `CompanionReserveDefault` in `config.balance.go`:

```go
	CharmDurationMinRounds ConfigInt `yaml:"CharmDurationMinRounds"` // Rounds a barely-won charm holds, and what a floored win takes (default 30)
	CharmDurationMaxRounds ConfigInt `yaml:"CharmDurationMaxRounds"` // Rounds a charm won by two sigma or better holds (default 450)
```

- [ ] **Step 2: Default them** — in `config.balance.mobs.go`, after the `CompanionReserveDefault` block:

```go
	if b.CharmDurationMinRounds < 1 {
		// ~3.4 minutes at RoundSeconds 4. What a scraped win buys, and what a
		// FLOORED win buys, since a mercy-granted success is not a dominant one.
		b.CharmDurationMinRounds = 30
	}
	if b.CharmDurationMaxRounds < 1 {
		// ~30 minutes. Deliberately short enough that a bond usually begins and
		// ends inside one session, which is what makes spec 13's
		// destroy-on-logout rule an edge case rather than a strategy.
		b.CharmDurationMaxRounds = 450
	}
```

**Do not** justify 450 by the old ladder's `50 + Cha/2 + manifestation*3`. Review found that number was the *interval between re-rolls*, re-granted on each success — it never meant "how long a charm lasts", and repeating that claim in a code comment would give the tuning a false pedigree.

- [ ] **Step 3: Verify defaults load**

```bash
go test ./internal/configs/ -count=1
```

- [ ] **Step 4: Ship the values via the committed blob**

`_datafiles/config.yaml` has `skip-worktree` and the working copy carries dev overrides that must not ship.

```bash
git show HEAD:_datafiles/config.yaml > /tmp/cfg.yaml
# add next to the companion knobs:
#   CharmDurationMinRounds: 30
#   CharmDurationMaxRounds: 450
python -c "import yaml;yaml.safe_load(open(r'C:\Users\CALABE~1\AppData\Local\Temp\cfg.yaml',encoding='utf-8'));print('parses')"
H=$(git hash-object -w /tmp/cfg.yaml)
git update-index --cacheinfo 100644,$H,_datafiles/config.yaml
git diff --cached _datafiles/config.yaml            # MUST show only the two added lines
git update-index --skip-worktree _datafiles/config.yaml
git ls-files -v _datafiles/config.yaml              # MUST print "S"
```

`--cacheinfo` **clears** skip-worktree; restoring it is not optional.

**Also copy the two keys into the on-disk file by hand.** Task 10's boot test copies the working file, so without this the boot exercises the Go defaults rather than the shipped values.

- [ ] **Step 5: Commit**

```bash
git add internal/configs/
git commit -m "feat(balance): charm duration knobs

Min 30 rounds is a scraped or floored win; Max 450 is a two-sigma win. The
ceiling is deliberately inside a typical session, which is what makes the
destroy-on-logout rule an edge case rather than a strategy."
```

---

### Task 3: The duration function

**Files:** Create `internal/hooks/charm_duration.go` and `internal/hooks/charm_duration_test.go`.

- [ ] **Step 1: Write the failing test**

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

func charmDurationTestConfig(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.CharmDurationMinRounds = 30
	cfg.Balance.CharmDurationMaxRounds = 450
	configs.SetConfigForTest(t, cfg)
}

// A bigger margin must NEVER buy a shorter bond. This is the test that catches
// an inverted margin sign, so assert the direction, not just the endpoints.
func TestCharmDurationFor_IsMonotonicInMargin(t *testing.T) {
	charmDurationTestConfig(t)

	prev := 0
	for _, m := range []float64{0, 0.1, 0.25, 0.5, 1.0, 1.5, 1.9} {
		got := charmDurationFor(m)
		if got < prev {
			t.Errorf("margin %v gave %d rounds, less than a smaller margin's %d", m, got, prev)
		}
		prev = got
	}
}

func TestCharmDurationFor_ClampsBothEnds(t *testing.T) {
	charmDurationTestConfig(t)

	if got := charmDurationFor(0); got != 30 {
		t.Errorf("margin 0 = %d, want the 30 minimum", got)
	}
	// A floored win reports margin 0 and must take exactly Min.
	if got := charmDurationFor(-5); got != 30 {
		t.Errorf("negative margin = %d, want the 30 minimum", got)
	}
	if got := charmDurationFor(2.0); got != 450 {
		t.Errorf("two sigma = %d, want the 450 maximum", got)
	}
	if got := charmDurationFor(50); got != 450 {
		t.Errorf("absurd margin = %d, want the 450 maximum", got)
	}
}

// Assert the RELATIONSHIP, not the shipped tuning: a two-sigma-halfway win sits
// halfway up the range. Hardcoding 240 would break on any retune while
// testing nothing about the curve.
func TestCharmDurationFor_MidMarginIsMidRange(t *testing.T) {
	charmDurationTestConfig(t)

	min, max := charmDurationFor(0), charmDurationFor(2.0)
	want := min + (max-min)/2
	if got := charmDurationFor(1.0); got != want {
		t.Errorf("margin 1.0 = %d, want the midpoint %d", got, want)
	}
}

// CharmPermanent is -1 and means never expires. This must never return it, or
// 0, either of which makes the bond permanent or instant.
func TestCharmDurationFor_NeverReturnsSentinelOrZero(t *testing.T) {
	charmDurationTestConfig(t)

	for _, m := range []float64{-100, -1, 0, 0.001, 1, 100} {
		if got := charmDurationFor(m); got < 1 {
			t.Errorf("margin %v returned %d; must always be >= 1", m, got)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails** — `go test ./internal/hooks/ -run TestCharmDurationFor -v`. Expected: `undefined: charmDurationFor`.

- [ ] **Step 3: Implement**

```go
package hooks

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// charmMarginCeiling is the normalized margin at which a charm buys its full
// duration: two sigma.
//
// NOT the crit bar, despite the resemblance. combat.CritBarFor subtracts
// CritBarSkillSlope*(atkRank-defRank) and clamps to [1.5, 3.0], and because no
// mob in the game carries an authored rhetoric rank the real bar sits at 1.5
// for any caster past manifestation 10. Calling this "the crit threshold"
// would be false for exactly the casters who use charm most.
const charmMarginCeiling = 2.0

// charmDurationFor converts the ATTACK-POSITIVE normalized margin of the
// winning contest into the rounds a charm holds.
//
//	duration = Min + (Max - Min) * clamp(margin / 2.0, 0, 1)
//
// The player is never told this number, nor how long is left. That uncertainty
// is the mechanic (spec 3.3): a bond you cannot plan around is the whole risk
// of charming something dangerous.
//
// A floored win arrives as margin 0 and takes Min, which is correct — a
// mercy-granted success is not a dominant one.
func charmDurationFor(normalizedMargin float64) int {
	bal := configs.GetBalanceConfig()

	lo := int(bal.CharmDurationMinRounds)
	hi := int(bal.CharmDurationMaxRounds)
	if lo < 1 {
		lo = 1
	}
	if hi < lo {
		hi = lo
	}

	ratio := normalizedMargin / charmMarginCeiling
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	return lo + int(math.Round(float64(hi-lo)*ratio))
}
```

- [ ] **Step 4: Run to verify it passes**, then commit.

```bash
go test ./internal/hooks/ -run TestCharmDurationFor -v
git add internal/hooks/charm_duration.go internal/hooks/charm_duration_test.go
git commit -m "feat(charm): duration is bought with the margin of victory

A floored win arrives as margin 0 and takes Min: a mercy-granted success is not
a dominant one.

The two-sigma ceiling is NOT described as the crit bar. CritBarFor clamps to
1.5 for any caster past manifestation 10, because no mob carries an authored
rhetoric rank, so that framing would be false for exactly the casters who use
charm most."
```

---

### Task 4: Route charm through the contest it already runs

This is the task v1 got backwards. **Nothing here adds a contest; this deletes one.**

**Files:** Modify `_datafiles/world/dogmud/spells/charm.yaml`, `internal/hooks/spell_resolution.go`, `internal/hooks/charm_spell.go`; create `internal/hooks/charm_effect_test.go`.

- [ ] **Step 1: Read the three sites before editing**

```bash
sed -n '1076,1082p' internal/hooks/spell_resolution.go   # spellAttackChannel
sed -n '811,836p'   internal/hooks/spell_resolution.go   # applyMobEffect switch
sed -n '226,234p'   internal/hooks/spell_resolution.go   # the post-loop charm block to DELETE
sed -n '25,120p'    internal/hooks/charm_spell.go        # resolveCharmSpell
```

- [ ] **Step 2: Declare the channel in the spell data**

In `_datafiles/world/dogmud/spells/charm.yaml`, beside the other top-level keys:

```yaml
target_defense_type: social
```

- [ ] **Step 3: Teach `spellAttackChannel` the social channel**

```go
func spellAttackChannel(spellData *spells.SpellData) combat.AttackChannel {
	if spellData == nil {
		return combat.ChannelSpellMental
	}
	switch spellData.TargetDefenseType {
	case "physical":
		return combat.ChannelSpellPhysical
	case "social":
		// Charm is an act of social domination whose attack side is already
		// Charisma, so defy answers it. Declaring the channel in data is what
		// replaces charm's hand-rolled contest.
		return combat.ChannelSocial
	}
	return combat.ChannelSpellMental
}
```

- [ ] **Step 4: Give charm its own effect arm**

In `applyMobEffect`'s switch, before `default`:

```go
	case "charm":
		return applyMobEffect_charm(user, mob, room, spellData, out, mName)
```

- [ ] **Step 5: Rewrite `resolveCharmSpell` as the effect arm**

In `charm_spell.go`, replace `resolveCharmSpell` with a function that **consumes** `out` instead of rolling:

```go
// applyMobEffect_charm binds a creature whose contest has ALREADY been decided.
//
// Charm used to run its own RunContest here, on hand-built scores, on top of
// the ChannelSpellMental contest the cast had already run and thrown away. That
// gave one cast two contests, two defence charges, two progression awards and
// two verdicts free to disagree — the exact shape U6b was written to delete.
// The contest now happens once, in the seam, on ChannelSocial, and this reads
// its result.
//
// Returns 0: charm deals no damage.
func applyMobEffect_charm(user *users.UserRecord, mob *mobs.Mob, room *rooms.Room,
	spellData *spells.SpellData, out combat.ChannelDefenceResult, mName string) int {

	ch := user.Character

	if mob.CharmImmune {
		user.SendText(messaging.CategorySpellMental, `That creature's mind is impervious to charm.`)
		return 0
	}

	// ── Reservation + budget gate ──────────────────────────────────────
	// The reserve is FLAT and does not scale with what you charmed: a sewer rat
	// and an Elemental King tie up the same conviction. That is a deliberate
	// decision (spec 3.8), not an oversight and not a leftover from the
	// pet-multiplier path. Charm is already a risky game — the creature trains
	// while it serves you, keeps the gear you hand it, and turns on you at a
	// moment you cannot predict. A power-scaled price would add bookkeeping
	// without adding tension, so the cost of charming something enormous is the
	// DANGER, not the invoice.
	reserve := ch.CalcCompanionReserve(characters.CompanionReserveBase(0))
	if len(ch.Companions) >= ch.GetMaxCompanions() {
		user.SendText(messaging.CategorySystem,
			`You are already sustaining as many companions as your will can hold.`)
		return 0
	}
	if ch.WouldBreachReservationCap(characters.PoolConviction, reserve) {
		user.SendText(messaging.CategorySystem, ch.ReservationRefusal(characters.PoolConviction, reserve))
		return 0
	}

	// Fumble is resolved BEFORE success: the seam's contract is that a fumbled
	// attack aborts even when the roll won.
	if out.AttackerFumble || out.Defended {
		user.SendText(messaging.CategorySpellMental, fmt.Sprintf(
			`You reach for %s's mind, but its will holds.`, mName))
		return 0
	}

	// The bond has a clock, and the player is never told how long it is.
	rounds := charmDurationFor(out.AttackerNormalizedMargin)
	mob.Character.Charm(user.UserId, rounds, "")
	mob.Character.EndAggro()
	ch.TrackCharmed(mob.InstanceId, true)

	// ... retain the existing CompanionInfo registration, ConvictionReserve:
	// reserve, RecalculateStats() and success messaging from the old
	// resolveCharmSpell body unchanged ...

	return 0
}
```

**Preserve verbatim** from the old body: the `CompanionInfo` construction, `ConvictionReserve: reserve`, the `ch.RecalculateStats()` call, and the success narration.

**Delete entirely**: the attack-score block, the defence-score block, the aggro-penalty block, and the `combat.RunContest` call.

- [ ] **Step 6: Delete the post-loop charm block**

Remove `spell_resolution.go:226-234` (`if !castFumbled && spellData != nil && spellData.EffectType == "charm" { ... }`). The effect now fires inside the target loop.

This also removes the two-contradictory-messages defect: charm no longer falls through `applyMobEffect_default`, so the player stops seeing a resist line and a success line for the same cast.

- [ ] **Step 7: Move the in-combat penalty onto the AttackSide**

The 0.75/0.85 penalties lived in the deleted block. `spellAttackSideFor` builds the side for every spell; charm's penalty composes onto its `Mult` there, alongside `combat.SituationalAttackMult` which every other `ChannelSocial` caller already applies and charm never did.

```bash
sed -n '282,305p' internal/hooks/spell_resolution.go
```

Apply the penalty only for `EffectType == "charm"`, so no other spell changes.

- [ ] **Step 8: Write the single-contest test**

```go
// One cast, ONE contest. v1 of this plan would have run two.
func TestCharm_RunsExactlyOneChannelContest(t *testing.T) {
	charmDurationTestConfig(t)

	calls := 0
	restore := combat.SetChannelAttackContestRunnerForTest(
		func(atk float64, entries []contest.Entry) contest.Result {
			calls++
			name := ""
			if len(entries) > 0 {
				name = entries[0].Name
			}
			return contest.Result{
				Success: true, Contested: true, Winner: name,
				Margin:      20,
				AttackRoll:  dice.RollResult{StdDev: 10},
				DefenseRoll: dice.RollResult{StdDev: 10},
			}
		})
	defer restore()

	castCharmAtTestMob(t) // drive a full charm cast; see helper note below

	if calls != 1 {
		t.Errorf("a charm cast ran %d channel contests, want exactly 1", calls)
	}
}

func TestCharm_RoutesToSocialChannel(t *testing.T) {
	sd := &spells.SpellData{TargetDefenseType: "social"}
	if got := spellAttackChannel(sd); got != combat.ChannelSocial {
		t.Errorf("spellAttackChannel = %v, want ChannelSocial", got)
	}
}
```

For `castCharmAtTestMob`, follow the seeding pattern in the nearest existing `internal/hooks` spell test — find it with:

```bash
grep -rln "resolveSpell\|resolveAgainstMob\|SeedSpellsForTest" internal/hooks/*_test.go | head -5
```

- [ ] **Step 9: Verify and commit**

```bash
gofmt -l internal/ && go build ./... && go test ./internal/hooks/ -count=1
```

Remove imports the deleted contest orphaned — `math` and `contest` in `charm_spell.go` are both likely casualties; `skills` survives only if something still uses it. Let the compiler decide.

```bash
git add internal/hooks/ _datafiles/world/dogmud/spells/charm.yaml
git commit -m "fix(charm): consume the contest instead of running a second one

Charm has resolved through the seam since U6b -- spellAttackChannel maps an
ABSENT target_defense_type to ChannelSpellMental, and the mob-target loop has
no effect-type guard. The seam's verdict was then discarded and a private
RunContest in resolveCharmSpell decided the real outcome.

Charm now declares target_defense_type: social, so the cast it already makes
routes to ChannelSocial and is answered by defy. The effect moves into
applyMobEffect's switch, where the result is already in scope, and the private
contest is deleted. One fewer contest than before, not one more.

This also fixes a live defect: charm printed two contradictory outcome
messages per cast, because the discarded seam contest still narrated through
applyMobEffect_default."
```

---

### Task 5: Expiry — the grudge, correctly gated

**Files:** Modify `internal/hooks/NewRound_MobRoundTick.go`; create `internal/hooks/charm_expiry_test.go`.

The grudge must fire for **charmed companions whose clock ran out**, and for nothing else.

- [ ] **Step 1: Write the failing tests**

```go
package hooks

import "testing"

func TestCharmExpiry_PresentCaster_ProducesTheGrudge(t *testing.T) {
	owner, mob := charmedPairInRoom(t)

	mob.Character.Charmed.RoundsRemaining = 0
	tickMobCharmState(mob)

	if mob.Character.IsCharmed() {
		t.Error("charm must be removed at expiry")
	}
	if owner.Character.GetCompanionByInstanceId(mob.InstanceId) != nil {
		t.Error("companion entry must be removed")
	}
	if mob.Character.Aggro == nil || mob.Character.Aggro.UserId != owner.UserId {
		t.Error("the creature must turn on the caster who was standing there")
	}
}

// The reservation is applied during RecalculateStats, not by RemoveCompanion.
// v1 of this plan asserted otherwise and told the implementer not to fix it.
func TestCharmExpiry_ReleasesTheReservation(t *testing.T) {
	owner, mob := charmedPairInRoom(t)
	before := owner.Character.GetPoolReservation("conviction", owner.Character.ConvictionMax.Value)
	if before <= 0 {
		t.Fatal("precondition: the charmed companion should be reserving conviction")
	}

	mob.Character.Charmed.RoundsRemaining = 0
	tickMobCharmState(mob)

	if after := owner.Character.GetPoolReservation("conviction", owner.Character.ConvictionMax.Value); after != 0 {
		t.Errorf("reservation still %d after the bond ended, want 0", after)
	}
}

// Five other systems set Charmed. None may ever produce a grudge.
func TestCharmExpiry_NonCharmedCompanionNeverGrudges(t *testing.T) {
	owner, mob := summonedPairInRoom(t) // SourceType: CompanionSummoned

	mob.Character.Charmed.RoundsRemaining = 0
	tickMobCharmState(mob)

	if mob.Character.Aggro != nil {
		t.Error("a SUMMONED companion must never turn on its owner")
	}
}

// Logout and link-dead both signal by setting RoundsRemaining to 0 -- the same
// state a natural expiry produces. Reading rounds-at-zero as one thing fires
// the grudge on a player mid-logout, and on the tutorial Guide mob when a
// newcomer's connection drops.
func TestCharmExpiry_OwnerLeaving_NoGrudge(t *testing.T) {
	owner, mob := charmedPairInRoom(t)
	markOwnerLeaving(t, owner) // whatever the despawn path sets; see Step 2

	mob.Character.Charmed.RoundsRemaining = 0
	tickMobCharmState(mob)

	if mob.Character.Aggro != nil {
		t.Error("a bond ended by the owner LEAVING must not produce a grudge")
	}
}

func TestCharmExpiry_AbsentCaster_NoGrudge(t *testing.T) {
	owner, mob := charmedPairInRoom(t)
	owner.Character.RoomId = mob.Character.RoomId + 1

	mob.Character.Charmed.RoundsRemaining = 0
	tickMobCharmState(mob)

	if mob.Character.IsCharmed() {
		t.Error("charm must still be removed")
	}
	if mob.Character.Aggro != nil {
		t.Error("a creature whose bond lapsed while the caster was away must not hunt them")
	}
}

func TestCharmExpiry_PermanentSentinelNeverExpires(t *testing.T) {
	_, mob := charmedPairInRoom(t)
	mob.Character.Charmed.RoundsRemaining = -1

	for i := 0; i < 50; i++ {
		tickMobCharmDuration(mob)
		tickMobCharmState(mob)
	}
	if !mob.Character.IsCharmed() {
		t.Error("a permanent charm expired; the -1 sentinel was mishandled")
	}
}
```

- [ ] **Step 2: Determine how "the owner is leaving" is detectable**

```bash
sed -n '130,155p' internal/hooks/PlayerDespawn_HandleLeave.go
sed -n '344,356p' internal/users/users.go
```

Both call `Charmed.Expire()`. Decide the discriminator and write `markOwnerLeaving` to match it. Options, in order of preference:

1. The despawn path removes the charm outright (`RemoveCharm` + `DestroyInstance`) rather than expiring it, so the tick never sees rounds-at-zero for a leaving owner. **Preferred** — it matches spec 13.1, where logout destroys the creature anyway.
2. Failing that, gate the grudge on the owner being **online and in the room**, so a logging-out or link-dead owner cannot receive one.

- [ ] **Step 3: Run to verify the tests fail** — `go test ./internal/hooks/ -run TestCharmExpiry -v`.

- [ ] **Step 4: Delete the ladder**

Remove the block from `// Re-roll contested Charisma vs Willpower on CharmDuration tick.` at `:394` through **`:455`** — verify the exact end brace; `:456` is `tickMobCharmState`'s own closing brace. v1 said `394-450`, which cuts the block mid-`else`.

That deletes: the periodic re-roll, the duplicated `Charisma + manifestation*25`, the `CharmRerolls` decay, and the "control is slipping" warnings.

**The deletion orphans three imports** — `combat`, `contest` and `skills` are used nowhere else in this file. Remove them or the package will not build.

- [ ] **Step 5: Promote the grudge into the expiry block, gated**

```go
	if mob.Character.IsCharmed() && mob.Character.Charmed.RoundsRemaining == 0 {
		cmd := mob.Character.Charmed.ExpiredCommand
		charmedUserId := mob.Character.RemoveCharm()

		if charmedUserId > 0 {
			if charmedUser := users.GetByUserId(charmedUserId); charmedUser != nil {
				comp := charmedUser.Character.GetCompanionByInstanceId(mob.InstanceId)

				// ONLY a charmed companion grudges. Summons, brood spawns, the
				// homunculus, befriended mobs and behaviour-tree companions all
				// set Charmed too, and a conjured creature turning on its
				// summoner would be a bug, not a mechanic.
				wasCharmed := comp != nil && comp.SourceType == characters.CompanionCharmed

				charmedUser.Character.TrackCharmed(mob.InstanceId, false)
				charmedUser.Character.RemoveCompanion(mob.InstanceId)

				// RemoveCompanion alone does NOT free the conviction: the
				// reservation is applied during RecalculateStats, which sums
				// live companions. dismiss does both halves for the same reason.
				charmedUser.Character.RecalculateStats()

				// The grudge only bites if the owner is there to receive it. A
				// bond lapsing while they are elsewhere -- or while they are
				// logging out -- must not create a creature that hunts them.
				if wasCharmed && charmedUser.Character.RoomId == mob.Character.RoomId {
					charmedUser.SendText(messaging.CategorySpellMental, fmt.Sprintf(
						`<ansi fg="red-bold">%s breaks free of your control!</ansi>`,
						mob.Character.Name))
					if room := rooms.LoadRoom(mob.Character.RoomId); room != nil {
						sendVisualRoomText(room, messaging.CategoryMobEmote, fmt.Sprintf(
							`<ansi fg="red">%s snarls and turns on %s!</ansi>`,
							mob.Character.Name, charmedUser.Character.Name),
							charmedUser.UserId)
					}
					mob.Character.SetAggro(charmedUser.UserId, 0, characters.DefaultAttack)
				}
			}
		}

		if cmd != `` {
			for _, c := range strings.Split(cmd, `;`) {
				if c = strings.TrimSpace(c); len(c) > 0 {
					mob.Command(c)
				}
			}
		}
	}
```

Both messages are carried over verbatim from the deleted ladder. They were written for this moment and never fired.

- [ ] **Step 6: Fix the lane split**

`tickMobCharmDuration` runs in the **idle** lane (`:113`); `tickMobCharmState` runs only for **active** zones (below the `if !active { continue }` gate at `:142`). A bond lapsing in a cold zone therefore parks at zero and fires when a player next enters — inverting the absent-caster rule into an ambush on return.

Move `tickMobCharmState` into the idle lane alongside the decrement, or gate the grudge on the bond having lapsed *this* round. Record which you chose and why in a comment.

- [ ] **Step 7: Verify and commit**

```bash
go test ./internal/hooks/ -run TestCharmExpiry -v
git add internal/hooks/NewRound_MobRoundTick.go internal/hooks/charm_expiry_test.go
git commit -m "feat(charm): expiry is the grudge, gated three ways

The ladder was gated on CompanionInfo.CharmDuration > 0 and the only assignment
to that field lived inside the ladder, so ~60 lines never ran -- including a
second copy of the charm scoring expression.

Its break-free branch was the one good thing in it, so that becomes the
unconditional expiry outcome, messages and all. Three gates keep it honest:
only a CompanionCharmed source grudges (five other systems set Charmed, and a
conjured creature turning on its summoner would be a bug), only a present owner
receives it, and an owner who is leaving never does -- logout and link-dead
both signal by setting rounds to zero, which is the same state a natural expiry
produces.

RecalculateStats is called because RemoveCompanion alone does not free the
reservation; the reservation is applied during recalculation."
```

---

### Task 6: Logout destroys the creature — confirm and pin

Spec 13. This is largely **confirming existing behaviour is preserved**, not building it.

**Files:** Modify `internal/hooks/PlayerDespawn_HandleLeave.go` if Task 5 Step 2 chose option 1; create tests.

- [ ] **Step 1: Write the test**

```go
// Spec 13: logging out destroys a charmed creature. No grudge, nothing left
// behind to grief newcomers, and the gear on it dies with it.
func TestCharmedCreature_DestroyedOnLogout(t *testing.T) {
	owner, mob := charmedPairInRoom(t)
	instId := mob.InstanceId

	simulatePlayerLogout(t, owner)

	if mobs.GetInstance(instId) != nil {
		t.Error("a charmed creature must not survive its owner logging out")
	}
	if len(owner.Character.Companions) != 0 {
		t.Error("the charmed companion record must not persist")
	}
}
```

- [ ] **Step 2: Verify the existing path already does this**

```bash
sed -n '40,63p' internal/hooks/PlayerSpawn_HandleJoin.go
```

It already destroys charmed mobs and strips charmed `CompanionInfo`, with the comment *"Charmed mobs are temporary by nature (borrowed, not created)."* **Preserve that comment** and extend it to record that this is now load-bearing:

```go
	// Remove charmed companions — they don't persist through restart.
	// Charmed mobs are temporary by nature (borrowed, not created).
	//
	// U10c makes this load-bearing rather than incidental. Destroying the
	// creature is what closes the griefing path a surviving grudge would open
	// (walk something large into a town, log out, leave it killing newcomers)
	// and it is why no charm clock needs to persist. Quitting still lets a
	// player dodge the betrayal, but it costs them the creature, the conviction
	// and any gear they gave it — and with a 30-minute ceiling, a bond rarely
	// spans a logout at all.
```

- [ ] **Step 3: Run, then commit.**

---

### Task 7: Guard the item-duplication vector

**Files:** Modify `internal/mobs/instance_save.go`; add a test.

- [ ] **Step 1: Write the failing test**

```go
// Once a bond expires the ex-companion is uncharmed while still wearing the
// equipment its owner handed it. Saving it would bake player gear into a world
// mob permanently: kill, loot, re-charm, repeat.
func TestSaveMobInstance_SkipsEverCharmedMobs(t *testing.T) {
	mob := everCharmedButNotCurrentlyCharmedMob(t)

	if err := SaveMobInstance(mob); err != nil {
		t.Fatalf("SaveMobInstance: %v", err)
	}
	if instanceSaveExists(t, mob) {
		t.Error("an ex-charmed mob must never be written to mobs.instances/")
	}
}
```

- [ ] **Step 2: Extend the skip**

```go
	// Companions live on CompanionInfo, not in mobs.instances/.
	//
	// EverCharmed, not just IsCharmed: once a bond expires the ex-companion is
	// uncharmed while still wearing the equipment its owner handed it. Saving
	// it would bake player gear into a world mob permanently -- kill it, loot
	// it, charm another, repeat. The betrayal stays real in-session (it fights
	// you with your own gear) but nothing reaches disk, so a reboot clears it.
	if mob.Character.IsCharmed() || mob.Character.EverCharmed {
		return nil
	}
```

- [ ] **Step 3: Run, then commit.**

---

### Task 8: Pin the non-combatant protection

Spec 11.3.3. Already true; the deliverable is a regression guard.

**Files:** Create `internal/mobs/charm_immune_shops_test.go`.

- [ ] **Step 1: Write the test**

```go
// Shopkeepers must never be charmable. Verified 2026-08-24: all 97 mobs with a
// shop block carry charm_immune and/or non_combatant, and non_combatant blocks
// charm before it is reached (playerHarmTargetPermitted -> CheckPlayerHarm ->
// HarmBlockedNonCombatant). This pins it so a future shopkeeper authored
// without either flag is caught here rather than by a player charming the
// blacksmith.
func TestEveryShopMobIsCharmProtected(t *testing.T) {
	// Walk the mob data root, parse each YAML, and for every mob with a
	// non-empty shop block assert CharmImmune || IsNonCombatant().
	// Follow the walking pattern in template_training_test.go.
}
```

```bash
grep -n "func TestNoMobTemplateCarriesAuthoredTraining" -A 25 internal/mobs/template_training_test.go
```

- [ ] **Step 2: Run — it must PASS immediately** (the property already holds). Verify it can fail by temporarily removing both flags from one shop mob, then restore.

- [ ] **Step 3: Commit.**

---

### Task 9: Retire the arc's allowlist rows, and guard the seam

**Files:** Modify `internal/combat/contest_site_guard_test.go`; create `internal/hooks/charm_seam_guard_test.go`.

- [ ] **Step 1: Delete the two stale rows**

```bash
grep -n "charm" internal/combat/contest_site_guard_test.go
```

Remove `"internal/hooks/NewRound_MobRoundTick.go:tickMobCharmState"` and `"internal/hooks/charm_spell.go:resolveCharmSpell"` from `contestSiteOwners`. The guard asserts **both** directions plus a vacuity floor, so leaving them turns `internal/combat` red.

- [ ] **Step 2: Add `charm_spell.go` to `legacyLiteralFiles`**

```bash
grep -n "legacyLiteralFiles" -A 16 internal/combat/contest_site_guard_test.go
```

Its `×25` is gone, so the literal guard should now cover it and stop it regrowing.

- [ ] **Step 3: Add the hooks-side guard**

`internal/hooks` guards are AST-based, not text greps — copy that pattern:

```bash
sed -n '20,50p' internal/hooks/channel_defence_routing_test.go
```

Assert that `charm_spell.go` contains **no** `RunContest` call and no `* 25` literal. Do **not** assert it contains `ResolveChannelAttack` — after Task 4 the contest lives in `spell_resolution.go`, not here, and v1's version of this guard would have certified the double-resolution as correct.

- [ ] **Step 4: Run `go test ./internal/combat/ ./internal/hooks/ -count=1`, then commit.**

---

### Task 10: Player-facing copy — REQUIRED FOR COMPLETION

Owner ruling: the slice is not done until both land. Read spec §10 and §11.3.4 first.

**Files:** Modify `_datafiles/world/dogmud/spells/charm.yaml`, `_datafiles/world/dogmud/templates/help/charm.template`.

- [ ] **Step 1: The spell description**

Drop "Stronger creatures resist more fiercely" — spec 3.6 makes it false. Convey that a strong-*willed* creature resists where a strong-*bodied* one may not, and that the bond does not last forever. No numbers.

- [ ] **Step 2: The helpfile**

Read it first: it already describes behaviour that never ran, and this slice largely makes it true. Four lines are now wrong:

| Line | Fix |
|---|---|
| `Defense: Mental (opposed by target's willpower)` | Social channel, answered by defy. Still Willpower-based. |
| "…willpower and **mental fortitude**" | The defending skill is **rhetoric**. |
| `Duration: Scales with charisma and manifestation skill` | Duration comes from **how decisively you won**. |
| "The stronger your charisma… the longer the charm endures." | Replace with the margin framing. |

**One line the spec previously said to preserve is FALSE and must be rewritten:** *"Merchants, powerful named creatures, and certain others are immune."* The 372 `charm_immune` mobs are townsfolk; **no boss carries it.** Say that shopkeepers and their like are beyond reach, without implying the powerful are.

**Add** a line for spec 13: the bond does not survive you leaving the world.

Still true, preserve: charm cannot target players; creatures already in combat resist more strongly; the companion limit and `dismiss`.

**Never state the formula or the rounds remaining.**

- [ ] **Step 3: Mechanical checks — including dashes, which v1 omitted**

```bash
grep -n "—\|–" _datafiles/world/dogmud/templates/help/charm.template   # MUST be empty
awk 'length($0)>80 {print FNR": "length($0)}' _datafiles/world/dogmud/templates/help/charm.template
o=$(grep -o '<ansi ' _datafiles/world/dogmud/templates/help/charm.template | wc -l)
c=$(grep -o '</ansi>' _datafiles/world/dogmud/templates/help/charm.template | wc -l); echo "open=$o close=$c"
python -c "import yaml;yaml.safe_load(open(r'_datafiles/world/dogmud/spells/charm.yaml',encoding='utf-8'));print('yaml ok')"
```

The file currently carries **four em dashes**; project convention forbids them in player copy. Long lines are acceptable only where the excess is ANSI markup.

- [ ] **Step 4: Verify SERVED, not just written**

Boot an isolated worktree on non-default ports, connect over telnet, create a throwaway veteran character, and run `help charm`. Confirm the rendered output no longer claims the duration scales with charisma and manifestation skill. Templates parse per request, so a copy-in and re-fetch needs no rebuild.

- [ ] **Step 5: Commit.**

---

### Task 11: Documentation

**Files:** `internal/combat/context.md`, `internal/hooks/context.md`.

- [ ] Both enumerate the charm `RunContest` sites this slice deletes
      (`combat/context.md:886-889`, `hooks/context.md:1065-1069`). Correct them.
- [ ] `hooks/context.md:1340` says "charmed companions are permanently Active" — false after this slice.
- [ ] `hooks/context.md:1415-1420` documents charm's flat reserve; keep it and add that the flatness is deliberate.
- [ ] Record the new `AttackerNormalizedMargin` field in `combat/context.md`.
- [ ] Commit.

---

### Task 12: Gates and ship

- [ ] **Local gates**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/combat/ ./internal/hooks/ ./internal/characters/ ./internal/configs/ ./internal/mobs/ ./internal/actions/ ./internal/usercommands/ -count=1
GOTMPDIR=C:/gotmp golangci-lint run ./internal/hooks/... ./internal/combat/... ./internal/mobs/...
```

No **new** lint finding on a touched file.

- [ ] **Boot test** — isolated detached worktree, non-default ports, fixed `boot-check.exe`. `Server Ready` = 1, panic patterns = 0. **Exit 124 is success.** Never grep the bare word `panic`.

- [ ] **Patch notes** — dated entry, player framing, no raw numbers, no en/em dashes, 80-char wrap. The story: a charmed creature now serves for a time rather than forever; you are never told how long; when the hold breaks it turns on you; and it does not survive you leaving the world.

- [ ] **Adversarial playtest gate** — required. Drive: casting charm and reading every line; keeping a creature until it turns; `help charm` read as a player would; and confirming a summoned companion never grudges.

- [ ] **PR** — `gh pr create --repo pruuk/DOGMud …`. `--repo` is mandatory; this is a fork and `gh` defaults to the parent. Confirm each job ran with zero annotations, merge with `--merge`.

- [ ] **Roadmap** — mark U10c ✅ with the merge SHA, and record what the arc learned: charm was never outside the seam, it was discarding the seam's verdict and running a second contest.

---

## Notes for whoever executes this

- **No migration.** The owner confirms no veteran character uses charm.
- **U10b is a different, still-unshipped slice.** Do not fold any of it in.
- If a grep finds something this plan did not predict, **stop and report.** v1 of this plan was rejected by blind review for exactly one such unpredicted fact, and every prior slice in this arc was re-planned for the same reason.
