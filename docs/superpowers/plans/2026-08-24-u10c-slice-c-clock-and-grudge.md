# U10c Slice C — The Clock And The Grudge

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Charm stops being permanent. The bond lasts a span bought with the margin of victory, and when it runs out mid-session the creature turns on the caster.

**Depends on Slice B** (PR #65). Charm must already resolve off one `ChannelSocial` contest before a duration can be read from that contest's margin.

**Spec:** `docs/superpowers/specs/2026-08-24-u10c-charm-redesign-design.md` — sections 11-14 supersede 1-10; 13 supersedes 11.3.2.

**Also folded in:** `dismiss` gains the same presence gate the expiry path uses (Task 8), so the two exits from a charmed bond cannot disagree about the anti-grief rule.

**Out of scope, and belongs to Slice D:** the `EverCharmed` instance-save guard, the shopkeeper regression test, the spell description and helpfile, patch notes, the playtest gate.

---

## Verified against `957899587` (slice B branch tip) while writing

Do not re-derive these. Do verify anything you are about to edit.

| Fact | Evidence |
|---|---|
| The dead ladder is `NewRound_MobRoundTick.go:394` through **`:455`**; `:456` closes `tickMobCharmState` | read directly |
| The expiry block starts at `:375` (`// Charm expiry cleanup.`) | read directly |
| `tickMobCharmDuration` is in the **idle** lane (`:113`); `tickMobCharmState` is at `:154`, **below** the `if !active` gate at `:142` | read directly |
| Logout runs `saveCompanionState(user)` **first**, which destroys charmed **companion** instances; the `Charmed.Expire()` loop after it only reaches charmed mobs that are **not** companions | `PlayerDespawn_HandleLeave.go:134-142` |
| `publishReleasedReservation` is **package-private to `internal/usercommands`** — hooks cannot call it and must queue `events.CharacterVitalsChanged{UserId:}` directly | `dismiss.go:39-41` |
| `dismiss` sets the grudge **unconditionally, with no room check**, after an early return for player-crafted companions | `dismiss.go:96-126` |
| A `ForceCrit` win returns **above** where `AttackerNormalizedMargin` is assigned, so it reads **zero** | pinned by `TestAttackerNormalizedMargin_ZeroOnForcedCritWin_KNOWN` |
| `CharmPermanent = -1`; `tickMobCharmDuration` decrements only `> 0`, and expiry gates on `== 0`, so `-1` is inert on both sides | `charminfo.go:9`, `NewRound_MobRoundTick.go:196-200` |

---

### Task 1: DESIGN GATE — what a sleeping creature buys

**Decide before writing code. Task 4 is unsafe without it.**

`resolveAgainstMob` sets `side.ForceCrit = combat.SleepingForceCrit(...)`. A
forced-crit win returns before `AttackerNormalizedMargin` is assigned, so it is
**zero** — and `charmDurationFor(0)` returns the **minimum**.

Left alone, **charming a sleeping creature buys the shortest possible bond**,
while a scrappy contested win against the same creature awake can buy the
longest. NPC schedules make sleeping routine (`activity: sleeping`), so this is
a common case, not a corner.

- [ ] Verify it yourself:

```bash
grep -n "SleepingForceCrit" internal/hooks/spell_resolution.go internal/combat/*.go
sed -n '/if side.ForceCrit {/,/^	}/p' internal/combat/defence_multiplier.go
go test ./internal/combat/ -run TestAttackerNormalizedMargin_ZeroOnForcedCritWin -v
```

- [ ] Choose and record in the spec as section 15 **before** coding:

**Recommended: treat a forced crit as maximally decisive, at the charm call
site.** A sleeping victim is the most decisive charm available, so it should buy
the ceiling. One branch in the charm arm, no shared-code change:

```go
	margin := out.AttackerNormalizedMargin
	if out.AttackerCrit && margin == 0 {
		// A forced crit (sleeping victim) returns from the seam above the
		// margin assignment, so it reads zero -- the MINIMUM -- for what is
		// the most decisive charm in the game. Read it as the ceiling instead.
		margin = charmMarginCeiling
	}
```

Alternative: populate the margin on the seam's ForceCrit path. That is a
shared-code change affecting every channel and needs its own justification;
Slice A deliberately left it zero and documented why.

Whichever is chosen, **the `_KNOWN` test in `internal/combat` must be updated or
deliberately left standing**, and this plan's Task 4 test must pin the outcome.

---

### Task 2: The duration knobs

**Files:** `internal/configs/config.balance.go`, `internal/configs/config.balance.mobs.go`, `_datafiles/config.yaml`.

- [ ] **Step 1: Declare**, beside `CompanionReserveDefault`:

```go
	CharmDurationMinRounds ConfigInt `yaml:"CharmDurationMinRounds"` // Rounds a barely-won charm holds, and what a floored win takes (default 30)
	CharmDurationMaxRounds ConfigInt `yaml:"CharmDurationMaxRounds"` // Rounds a charm won by two sigma or better holds (default 450)
```

- [ ] **Step 2: Default**, in `config.balance.mobs.go` after that block:

```go
	if b.CharmDurationMinRounds < 1 {
		// ~3.4 minutes at RoundSeconds 4. What a scraped win buys, and what a
		// FLOORED win buys -- a mercy-granted success is not a dominant one.
		b.CharmDurationMinRounds = 30
	}
	if b.CharmDurationMaxRounds < 1 {
		// ~30 minutes. Deliberately short enough that a bond usually begins and
		// ends inside one session, which is what makes spec 13's
		// destroy-on-logout rule an edge case rather than a strategy.
		b.CharmDurationMaxRounds = 450
	}
```

**Do not** justify 450 by the old ladder's `50 + Cha/2 + manifestation*3`. That
was the *interval between re-rolls*, re-granted on each success — it never meant
"how long a charm lasts", and repeating it would give the tuning a false
pedigree.

- [ ] **Step 3:** `go test ./internal/configs/ -count=1`.

- [ ] **Step 4: Ship the values from the committed blob.** `config.yaml` has
      `skip-worktree` and the working copy carries dev overrides.

```bash
git show HEAD:_datafiles/config.yaml > /tmp/cfg.yaml
# add the two keys near the companion knobs
python -c "import yaml;yaml.safe_load(open(r'<WINDOWS PATH TO /tmp/cfg.yaml>',encoding='utf-8'));print('parses')"
H=$(git hash-object -w /tmp/cfg.yaml)
git update-index --cacheinfo 100644,$H,_datafiles/config.yaml
git diff --cached _datafiles/config.yaml          # MUST show only the two added lines
git update-index --skip-worktree _datafiles/config.yaml
git ls-files -v _datafiles/config.yaml            # MUST print "S"
```

**Parse the same path you hashed** — a previous version of this plan validated a
different file than it staged. `--cacheinfo` clears skip-worktree; restoring it
is not optional.

**Also add the two keys to the on-disk file by hand**, or the boot test in
Task 11 exercises the Go defaults rather than the shipped values.

- [ ] **Step 5: Commit.**

---

### Task 3: The duration function

**Files:** create `internal/hooks/charm_duration.go` and `charm_duration_test.go`.

- [ ] **Step 1: Write the failing tests**

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

// A bigger margin must NEVER buy a shorter bond. This is what catches an
// inverted margin sign, which the contest package warns compiles cleanly.
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

// Assert the RELATIONSHIP, not the shipped tuning. Hardcoding 240 would break
// on any retune while testing nothing about the curve's shape.
func TestCharmDurationFor_MidMarginIsMidRange(t *testing.T) {
	charmDurationTestConfig(t)
	lo, hi := charmDurationFor(0), charmDurationFor(2.0)
	if got, want := charmDurationFor(1.0), lo+(hi-lo)/2; got != want {
		t.Errorf("margin 1.0 = %d, want the midpoint %d", got, want)
	}
}

// CharmPermanent is -1 and means never expires. This must never return it or 0,
// either of which makes the bond permanent or instant.
func TestCharmDurationFor_NeverReturnsSentinelOrZero(t *testing.T) {
	charmDurationTestConfig(t)
	for _, m := range []float64{-100, -1, 0, 0.001, 1, 100} {
		if got := charmDurationFor(m); got < 1 {
			t.Errorf("margin %v returned %d; must always be >= 1", m, got)
		}
	}
}
```

- [ ] **Step 2: Run — expect `undefined: charmDurationFor`.**

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
// for any caster past manifestation 10. Calling this "the crit threshold" would
// be false for exactly the casters who use charm most.
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
// A floored win arrives as margin 0 and takes Min, which is correct -- a
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

- [ ] **Step 4: Run, commit.**

---

### Task 4: Give the bond its clock

**Files:** `internal/hooks/charm_spell.go`; test alongside.

- [ ] **Step 1: Write the failing tests**

```go
// A dominant charm holds far longer than a scraped one. Slice B shipped charm
// permanent (99999); this is where the clock arrives.
func TestApplyMobEffectCharm_DurationRidesTheMargin(t *testing.T) {
	// drive the arm twice with different AttackerNormalizedMargin and assert
	// mob.Character.Charmed.RoundsRemaining differs in the right DIRECTION,
	// and that neither is 99999 or CharmPermanent.
}

// Task 1's ruling. A sleeping victim forces the crit, which returns from the
// seam above the margin assignment and therefore reads zero -- the minimum --
// for the most decisive charm in the game.
func TestApplyMobEffectCharm_ForcedCritBuysTheCeiling(t *testing.T) {
	// out := ChannelDefenceResult{AttackerCrit: true, AttackerNormalizedMargin: 0}
	// assert RoundsRemaining == CharmDurationMaxRounds
}
```

- [ ] **Step 2: Replace the permanent charm**

In the charm arm, swap `targetMob.Character.Charm(user.UserId, 99999, "")` for
the margin-derived duration, applying Task 1's ForceCrit ruling.

**Do not substitute `characters.CharmPermanent`.** That sentinel means "never
expires" and would restore exactly the bug this slice removes.

- [ ] **Step 3: Run, commit.**

---

### Task 5: Delete the ladder

**Files:** `internal/hooks/NewRound_MobRoundTick.go`.

- [ ] **Step 1:** Remove `:394` through **`:455`** — the whole
      `if mob.Character.IsCharmed() { ... }` block introduced by
      `// Re-roll contested Charisma vs Willpower on CharmDuration tick.`
      `:456` is `tickMobCharmState`'s own closing brace; do not take it.

That deletes the periodic re-roll, the duplicated `Charisma + manifestation*25`,
the `CharmRerolls` effectiveness decay, and the "control is slipping" warnings.

- [ ] **Step 2: Fix the orphaned imports.** `combat`, `contest` and `skills` are
      used **nowhere else** in this file and will fail the build. Let the
      compiler confirm rather than trusting this list.

- [ ] **Step 3:** `go build ./... && go test ./internal/hooks/ -count=1`. Commit.

---

### Task 6: Expiry becomes the grudge — gated THREE ways

**Files:** `internal/hooks/NewRound_MobRoundTick.go`; create `charm_expiry_test.go`.

This is the task both previous plan versions got wrong. Every gate below exists
because something concrete goes wrong without it.

- [ ] **Step 1: Write the failing tests — all five**

```go
func TestCharmExpiry_PresentOwner_ProducesTheGrudge(t *testing.T) { /* ... */ }

// The reservation is applied during RecalculateStats, NOT by RemoveCompanion.
// Assert a DERIVED value (EffectivePoolMax / usable conviction), never
// GetPoolReservation -- that helper recomputes from the live companion slice,
// so it returns 0 the moment the entry is dropped and would pass even if
// RecalculateStats were never called.
func TestCharmExpiry_ReleasesTheReservation(t *testing.T) { /* ... */ }

// Five other systems set Charmed: summons, brood spawns, the homunculus,
// befriend, behaviour-tree companions. A conjured creature turning on its
// summoner would be a bug, not a mechanic.
func TestCharmExpiry_SummonedCompanionNeverGrudges(t *testing.T) { /* ... */ }

// Spec 3.10 anti-grief.
func TestCharmExpiry_AbsentOwner_NoGrudge(t *testing.T) { /* ... */ }

// CharmPermanent is -1 and must be inert on both the decrement and the gate.
func TestCharmExpiry_PermanentSentinelNeverExpires(t *testing.T) { /* ... */ }
```

- [ ] **Step 2: Implement the three gates**

```go
	// 1. ONLY a charmed companion grudges.
	comp := charmedUser.Character.GetCompanionByInstanceId(mob.InstanceId)
	wasCharmed := comp != nil && comp.SourceType == characters.CompanionCharmed

	// 2. The owner must be PRESENT to receive it. A bond lapsing while they are
	//    elsewhere must not create a creature that hunts them across zones.
	present := charmedUser.Character.RoomId == mob.Character.RoomId

	// 3. The owner must not be LEAVING. Logout and link-dead both signal by
	//    setting RoundsRemaining to 0 -- the same state a natural expiry
	//    produces -- and link-dead does it to the tutorial Guide mob. Firing a
	//    grudge at a player who is disconnecting is indefensible.
```

For gate 3, **verify how "leaving" is detectable before choosing a mechanism**:

```bash
sed -n '130,145p' internal/hooks/PlayerDespawn_HandleLeave.go
sed -n '340,356p' internal/users/users.go
grep -rn "zombie" internal/users/*.go | head -5
```

Note what Slice C's planning already found: at logout `saveCompanionState` runs
**first** and destroys charmed *companion* instances, so the `Charmed.Expire()`
loop after it only reaches charmed mobs that are **not** companions. Confirm
whether that makes gate 3 reachable for a companion at all — if it is not, say
so in a comment rather than adding a guard against nothing. **Link-dead is a
separate path and must still be checked.**

- [ ] **Step 2b: Use `dismiss`'s bookkeeping ORDER, not your own**

`dismiss` is the other place a charmed bond ends, and it got the ordering right
for a reason it records at `dismiss.go:91-93`:

> *"Remove the companion record before doing anything that might trigger
> room-wide combat logic."*

Setting aggro before `RemoveCompanion` can re-enter combat resolution while the
pet is half-removed — still on `Companions`, no longer charmed. Follow the same
sequence here:

```go
	mob.Character.RemoveCharm()                       // 1. break the link
	charmedUser.Character.TrackCharmed(mob.InstanceId, false) // 2. untrack
	charmedUser.Character.RemoveCompanion(mob.InstanceId)     // 3. drop the record
	charmedUser.Character.RecalculateStats()                  // 4. free the reserve
	events.AddToQueue(events.CharacterVitalsChanged{UserId: charmedUser.UserId})
	// 5. ONLY NOW may aggro be set.
```

- [ ] **Step 3: Release the reservation properly**

`RemoveCompanion` is not enough. Call `RecalculateStats()`, then queue the
vitals refresh directly — `publishReleasedReservation` is private to
`internal/usercommands`:

```go
	charmedUser.Character.RecalculateStats()
	events.AddToQueue(events.CharacterVitalsChanged{UserId: charmedUser.UserId})
```

Without the second line the conviction bar stays stale at exactly the moment it
matters: the player is now being attacked.

- [ ] **Step 4: Carry the ladder's copy verbatim.** Both lines were written for
      this moment and never fired:

- to the owner: `%s breaks free of your control!`
- to the room: `%s snarls and turns on %s!`

- [ ] **Step 5: Run, commit.**

---

### Task 7: The lane split — decide and document

`tickMobCharmDuration` runs in the **idle** lane (every round, every zone);
`tickMobCharmState` runs only for **active** zones. A bond that lapses in a cold
zone therefore parks at `RoundsRemaining == 0` and the expiry fires when a
player next makes that zone active.

- [ ] **Step 1: Judge whether this is actually wrong.** The grudge already
      requires the owner to be present, so a deferred fire means "it turns on
      you when you come back" — which is arguably what the rule says. What is
      genuinely wrong is that the bond *appears* to persist: the creature stays
      charmed, stays a companion, and keeps reserving conviction, for an
      unbounded time.

- [ ] **Step 2: Choose.**

**Recommended: move `tickMobCharmState` into the idle lane**, so a bond ends on
schedule everywhere. Note the consequence in a comment: `ExpiredCommand`
execution (e.g. `CharmExpiredRevert`, despawns) then also runs in unpopulated
zones, for every charm setter and not just charm.

The alternative — a "lapsed this round" flag — needs new state that nothing else
wants, since `RoundsRemaining` parks at 0 rather than going negative.

- [ ] **Step 3:** Whatever is chosen, write it down at the site. Commit.

---

### Task 8: Give `dismiss` the same room check — belt and suspenders

**Files:** `internal/usercommands/dismiss.go`; test alongside.

`dismiss` is the third way a charmed bond ends, and it is the odd one out. After
this slice:

| Exit | Grudge? | Creature survives? | Room check? |
|---|---|---|---|
| `dismiss` | yes | yes | **no** |
| expiry (Task 6) | yes | yes | **yes** |
| logout | no | no | n/a |

`dismiss.go` sets `SetAggro(user.UserId, ...)` unconditionally. A player who
dismisses a charmed creature they are not standing next to creates exactly the
cross-zone hunter that spec 3.10's presence gate exists to prevent — the rule
Task 6 implements carefully is violated by the command next door.

**Owner ruling 2026-08-24: this is very hard to reach in practice** — companions
follow their owner closely — **but add the guard anyway. Belt and suspenders.**
Do not spend effort trying to construct the scenario in game; the point is that
the two exits should not disagree about a rule, not that anyone is hitting it.

- [ ] **Step 1: Write the failing test**

```go
// The expiry path gates its grudge on the owner being present, because a
// hostile creature with patrol and pathto behaviour can otherwise follow a
// player across zones. dismiss must not be a way around that rule.
//
// Reaching this in play is very hard -- companions follow closely -- so this
// is a guard, not a bug fix. It exists so the two exits from a charmed bond
// cannot disagree about the same anti-grief rule.
func TestDismiss_AbsentCharmedCreatureDoesNotGetAggro(t *testing.T) {
	// Seed a charmed companion whose Character.RoomId differs from the
	// dismissing user's. Call Dismiss. Assert:
	//   - the companion record is removed (dismiss still works)
	//   - the reservation is released
	//   - mob.Character.Aggro is nil
}

// The normal case must keep working: dismiss a creature you are standing next
// to and it turns on you.
func TestDismiss_PresentCharmedCreatureStillTurnsOnYou(t *testing.T) { /* ... */ }
```

- [ ] **Step 2: Add the guard**

Wrap the aggro and its two messages, leaving every other step unchanged:

```go
	// Charmed wild creature — the bond-break is a betrayal; it turns hostile.
	//
	// Only if the owner is THERE to receive it. A creature dismissed from
	// another room would otherwise acquire aggro it can carry across zones via
	// patrol and pathto, which is the griefing shape spec 3.10 rules out for
	// the expiry path. Reaching this is very hard in practice -- companions
	// follow closely -- so it is a guard rather than a fix, and it exists so
	// the two exits from a charmed bond cannot disagree about the same rule.
	if mob.Character.RoomId == user.Character.RoomId {
		mob.Character.SetAggro(user.UserId, 0, characters.DefaultAttack)
		// ... the two existing messages, unchanged ...
	}
```

**Leave the room broadcast where it is** — the room should still see the
dismissal even when the creature is elsewhere; check which of the three
messages is the room one before wrapping.

- [ ] **Step 3:** `go test ./internal/usercommands/ -count=1`. Commit.

---

### Task 9: Delete the dead companion fields

**Files:** `internal/characters/companions.go`.

- [ ] **Step 1: Confirm nothing reads them.** Use a pattern that does **not**
      also match the live helper `tickMobCharmDuration`:

```bash
grep -rnE "[^a-zA-Z]CharmDuration\b|[^a-zA-Z]CharmRerolls\b" --include=*.go internal/ modules/
```

A bare `grep CharmDuration` matches `tickMobCharmDuration` four times and will
send you chasing a false positive.

- [ ] **Step 2:** Confirm no save carries them:

```bash
grep -rl "charm_duration\|charm_rerolls" _datafiles/world/dogmud/users/ 2>/dev/null
```

Expect nothing — both are `omitempty` and were never assigned.

- [ ] **Step 3:** Delete `CharmDuration` and `CharmRerolls` from `CompanionInfo`.
      Build, test, commit.

---

### Task 10: Retire the second allowlist row

**Files:** `internal/combat/contest_site_guard_test.go`.

- [ ] Slice B left `"internal/hooks/NewRound_MobRoundTick.go:tickMobCharmState"`
      in place, owned by "U10c slice C". Task 5 deletes that site, so the row
      goes now. The guard asserts both directions and will fail with
      `stale allowlist entry` otherwise — it caught exactly this in Slice B.

- [ ] Consider adding `NewRound_MobRoundTick.go` to `legacyLiteralFiles` once its
      `* 25.0` is gone. **Check the reader's behaviour on a missing file first**
      (`sed -n '370,382p'`): it `t.Fatalf`s, so a listed file may never be
      deleted later.

- [ ] `go test ./internal/combat/ -count=1`. Commit.

---

### Task 11: Gates

- [ ] `gofmt -l internal/ modules/` prints nothing.
- [ ] `go build ./...`; tests for `combat`, `hooks`, `characters`, `configs`,
      `actions`, `usercommands`, `mobcommands`.
- [ ] `golangci-lint` — no **new** finding on a touched file.
- [ ] Boot in an isolated detached worktree on non-default ports. `Server Ready`
      = 1, panic patterns = 0. **Exit 124 is success.** Never grep the bare word
      `panic`.
- [ ] **No patch notes yet.** The player-facing story lands whole in Slice D,
      with the copy.
- [ ] PR with `--repo pruuk/DOGMud`. Confirm each job ran with zero annotations.
      Merge `--merge`.

---

## For whoever executes this

Slice B's in-game verification was **not** completed — granting charm needs
discovery or a save edit, and the save edit hit a forced password-change flow.
So no one has yet watched a charm resolve in a live game since the rewrite.

That does not block this slice, but it means **Slice D's playtest gate is
carrying more weight than usual**. Do not let it be skipped.

If a `grep` or `sed` finds something this document did not predict, **stop and
report**. Two prior plans for this work were rejected for exactly one such
unpredicted fact each, and both sounded most confident precisely where they were
wrong.
