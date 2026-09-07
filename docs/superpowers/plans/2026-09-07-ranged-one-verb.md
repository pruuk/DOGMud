# Ranged: One Verb, One Action — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `fire <target> [direction]` resolves the shot and chambers the next
round as one action, burning one special-move cooldown whether or not the shooter
was sneaking. The player-facing `reload` retires, taking a two-way cooldown
collision with it.

**Architecture:** Task 1 gives the shared special-move cooldown a single owner:
one helper in `internal/actions`, consumed by all 56 call sites that currently
hand-roll the magic string. Everything after it becomes small. The ranged fold
then lands in `actions.ExecuteFire`, the single seam both
`internal/usercommands/shoot.go` and `internal/mobcommands/shoot.go` already call,
so player and mob parity is structural rather than a second implementation.
`ExecuteReload` stops being a player action and becomes an internal chambering
step. The command registry flips `fire` to canonical with `shoot` as its alias,
and the quest-notify string moves with it so the two cannot drift.

**Owner direction, 2026-09-07:** *"Just have a shared helper for managing the
cooldown that gets called by spells, special attacks, fire/reload, grapple,
etc."* That is Task 1, and it is why the ranged tasks are as small as they are:
the question "should the ambush be denied when the timer is spent" stops being a
ranged decision and becomes one rule in one place.

**Tech Stack:** Go 1.x, `gopkg.in/yaml.v2` data files, Go text/template help
pages, the project's `actions`/`combat` seam packages.

**Source spec:** `docs/superpowers/specs/2026-09-07-ranged-one-verb-design.md`

---

## Facts verified against source, 2026-09-07

Read from the files at plan-writing time. Trust this table over any older note.

| Fact | Evidence |
|---|---|
| `fire` is ALREADY an alias of `shoot` | `_datafiles/world/dogmud/keywords.yaml:318` -> `shoot: ['fire']` |
| Registry lines to flip | `usercommands/usercommands.go:203` `` `shoot`: {Shoot, false, true, false} `` and `:181` `` `reload`: {Reload, false, true, false} ``; `mobcommands/mobcommands.go:89` `"shoot"` and `:77` `"reload"` |
| An ORDINARY shot does NOT burn the special-move cooldown today; RELOAD does | `combat_fire.go:89-95` doc comment; fire's only `TryCooldown` is `:277`, gated on `surpriseShot`; `combat_reload.go:133` |
| Reload also GATES on that timer being ready | `combat_reload.go:86` |
| 🔴 So an ambush strands the weapon: it claims the timer, then reload refuses on it for `SpecialMoveCooldown` rounds | derived from the two rows above; `SpecialMoveCooldown: 4` (`config.yaml:747`) |
| Costs are separate and both real | `costs.ActionShoot` / `costs.ActionReload` (`internal/costs/action.go:33-34`); `ShootBaseStaminaCost: 2`, `ReloadBaseStaminaCost: 1` (`config.yaml:812-813`) |
| `ExecuteReload` does careful work that must be preserved wholesale | `combat_reload.go:96-147`: cost admission, then re-find the bundle by identity (`Equals`) rather than stale index, `BundleEmptied`, and a full rollback if the cooldown claim fails |
| `ReloadResult` fields | `WeaponName`, `AmmoTag`, `AmmoName`, `BundleEmptied`, `Cost`, `Loaded`, `NoWeapon`, `AlreadyLoaded`, `NoAmmo`, `OnCooldown`, `Crafting` |
| `FireResult` already carries `NotLoaded`, `SurpriseOnCooldown`, `Revealed`, `IsSneaking`, `Cost`, `Executed` | `combat_fire.go` FireResult struct |
| The quest notify hardcodes the string regardless of what was typed | `usercommands/shoot.go:207-211` sends `Command: "shoot"` |
| **Three** quests key on these commands | `50-first_shot.yaml:82` (`command: reload`) and `:95` (`command: shoot`, `room: 5351`); `51-across_the_canyon.yaml:80` (`command: shoot`, `room: 5354`); `59-range_practice.yaml:54` (`command: shoot`, `room: 5354`) |
| A dialogue node gates the quest-50 turn-in on BOTH tokens | `_datafiles/world/dogmud/dialogue/pothole_coulee/9161.yaml:144` -> `questRequired: ["50-reload", "50-shoot"]` |
| Quest 50 exists to teach the reload loop; its own reward text says so | `50-first_shot.yaml:13-14`, `:70` (*"shoot, then reload, shoot, then reload"*) |
| ✅ **`sneak` is available to EVERY character from creation.** `initAllSkills()` seeds every skill at rank 1 and `ensureAllSkills()` floors existing saves at 1 | `internal/characters/character.go:443-451`; `sneak` gates on `Skullduggery >= 1` (`skill.skullduggery.sneak.go:23-26`). No quest grants skullduggery because none needs to. **This is why quest 50 can teach the ambush.** |
| Attack messages key on weapon SUBTYPE, and all 8 ranged weapons are `subtype: shooting` | `combat_helpers.go:1577` (`displaySubtype := ws.weaponSubType`), `:1621`; `attack_messages.go:157` |
| Three ammo tags exist | `arrows` (Training Bow, Hunting Bow, Ironhorn Warbow), `bolts` (Hand Crossbow, Arbalest), `shot` (Sling, Primitive Pistol, Relic Sidearm) |
| The archer tree only DECIDES to reload; it does not implement it | `behaviortree/actions_archer.go` -> `unloadedRangedWeapon`, `hasMatchingAmmo` |
| File sizes, for splitting judgement | `usercommands/reload.go` 131 lines, `mobcommands/reload.go` 30, `actions/combat_reload.go` 147 |

---

## File Structure

**Go — modified**

| File | Responsibility |
|---|---|
| **56 call sites across ~20 files** | Migrated onto the Task 1 cooldown helper: the 16 combat verbs, `throw.go`, the readiness checks, `mutation_helpers.go`, `mobcommands/cast.go`, and (cycle permitting) `combat/ai.go` |
| `internal/actions/combat_reload.go` | `ExecuteReload` becomes internal `chamberNextRound`; keeps all bundle/rollback logic |
| `internal/actions/combat_fire.go` | Calls the chambering step after the shot; owns the single cooldown claim |
| `internal/usercommands/shoot.go` | Renamed command entry `Fire`; speaks the recovery line; notify string becomes `"fire"` |
| `internal/usercommands/usercommands.go` | Registry: `fire` canonical, `reload` player half removed |
| `internal/mobcommands/shoot.go` | Mob entry renamed to `Fire` |
| `internal/mobcommands/mobcommands.go` | Registry: `fire` canonical, `reload` removed |
| `internal/behaviortree/actions_archer.go` | Reload decision collapses to an ammo check |

**Go — deleted**

| File | Why |
|---|---|
| `internal/usercommands/reload.go` | Player half retires; the admin half moves to `admin.reload.go` |
| `internal/mobcommands/reload.go` | Mobs no longer reload as a separate act |

**Go — created**

| File | Responsibility |
|---|---|
| `internal/actions/special_move_cooldown.go` | The single owner of the shared special-move timer: tag constant, `SpecialMoveReady`, `ClaimSpecialMove`, `ReleaseSpecialMove` |
| `internal/actions/special_move_cooldown_test.go` | Atomic claim, configured duration (kills the `"1 rounds"` outlier), release |
| `internal/usercommands/admin.reload.go` | The surviving admin data-file reload, no longer sharing a file with a player action |
| `internal/actions/combat_fire_fold_test.go` | The fold: one cooldown, weapon left loaded, mob parity, both collisions dead |
| `internal/usercommands/ranged_quest_contract_test.go` | Pins the Go notify string to the quest YAML triggers |

**Data — modified**

| File | Responsibility |
|---|---|
| `_datafiles/world/dogmud/keywords.yaml` | Alias flip; help topic rename |
| `_datafiles/world/dogmud/combat-messages/shooting.yaml` | Three recovery lines keyed on ammo tag |
| `_datafiles/world/dogmud/quests/50-first_shot.yaml` | Reload beat removed; second beat teaches the ambush |
| `_datafiles/world/dogmud/quests/51-across_the_canyon.yaml` | Trigger `shoot` -> `fire` |
| `_datafiles/world/dogmud/quests/59-range_practice.yaml` | Trigger `shoot` -> `fire` |
| `_datafiles/world/dogmud/dialogue/pothole_coulee/9161.yaml` | Turn-in gate drops `50-reload` |
| `_datafiles/world/dogmud/templates/help/shoot.template` | Becomes the `fire` page |
| `_datafiles/world/dogmud/templates/help/reload.template` | Admin-only |
| `_datafiles/world/dogmud/templates/help/ranged-combat.template` | Describes one action |
| `docs/PATCH_NOTES.md` | Dated player-facing entry |

---

## Task 1: One owner for the special-move cooldown

**Do this before anything else.** It is a pure refactor with no behaviour change,
and it makes every later task small.

### Why

`grep -rn '"special-move"' --include=*.go internal/` returns **56 non-test call
sites**, each hand-typing the magic string, in **five different idioms**:

| idiom | sites |
|---|---|
| `CooldownReady(...)` then, lines later, `TryCooldown(..., fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown))` | 16 verbs: bash, drain, gore, grapple, hamstring, kick, maul, pounce, rake, rally, reload, taunt, throttle, trip, warcry, throw |
| `GetCooldown(...) > 0` | `action_readiness.go:67,136`, `command_readiness.go:39`, `action_cast_best_in_category.go:45` |
| `char.Cooldowns.Try(...)` raw | `combat_helpers.go:75`, `mutation_helpers.go:99` |
| `_, exists := char.Cooldowns[...]` raw map read | **12 sites** in `combat/ai.go` |
| `delete(char.Cooldowns, ...)` raw | `mutation_helpers.go:115`, `mobcommands/cast.go:109` |

🔴 **There is already a live inconsistency in there.** `combat_helpers.go:75`
hardcodes `"1 rounds"` instead of reading `cfg.SpecialMoveCooldown`, so one path
runs a 1-round cooldown while all 16 verbs run the configured 4. Nothing flags
it, because there is nothing to flag it against.

🔴 **The check-then-claim split is the shape that produced the ranged bug.**
Reload *checked* the timer at `combat_reload.go:86` and *claimed* it at `:133`,
47 lines apart. That distance is where "reload denies the ambush" hid.

**Files:**
- Create: `internal/actions/special_move_cooldown.go`
- Create: `internal/actions/special_move_cooldown_test.go`
- Modify: the 56 call sites listed above

- [ ] **Step 1: Read the underlying API before designing on top of it**

```bash
sed -n '60,100p' internal/characters/cooldowns.go
```

Note the existing contract, which the helper wraps rather than replaces:
`Cooldowns` is a `map[string]int`; `CooldownReady(tag) bool` is a pure read that
does not allocate; `TryCooldown(tag, "N rounds") bool` returns **true when the
cooldown was free and has now been claimed**, false when it was already running.

- [ ] **Step 2: Write the failing test**

Create `internal/actions/special_move_cooldown_test.go`:

```go
package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
)

// TestSpecialMoveCooldownClaimIsAtomic pins the property the old idiom could not
// offer: asking and taking are ONE call. Sixteen verbs used to call
// CooldownReady first and TryCooldown many lines later, and reload's two halves
// sat 47 lines apart -- which is exactly where "reload denies the ambush" hid.
func TestSpecialMoveCooldownClaimIsAtomic(t *testing.T) {
	c := &characters.Character{}

	if !SpecialMoveReady(c) {
		t.Fatal("a fresh character must have the special-move cooldown free")
	}
	if !ClaimSpecialMove(c) {
		t.Fatal("the first claim must succeed")
	}
	if SpecialMoveReady(c) {
		t.Error("the cooldown must read as busy after a claim")
	}
	if ClaimSpecialMove(c) {
		t.Error("a second claim must fail while the cooldown is running")
	}
}

// TestSpecialMoveCooldownUsesTheConfiguredDuration pins the outlier out of
// existence. combat_helpers.go hardcoded "1 rounds" while sixteen verbs used the
// configured value, so one path ran a quarter of the intended cooldown and
// nothing anywhere could notice.
func TestSpecialMoveCooldownUsesTheConfiguredDuration(t *testing.T) {
	c := &characters.Character{}
	ClaimSpecialMove(c)

	if got := c.GetCooldown(SpecialMoveCooldownTag); got <= 1 {
		t.Errorf("cooldown = %d rounds, want the configured SpecialMoveCooldown (>1); "+
			"a value of 1 means the hardcoded outlier came back", got)
	}
}

// TestReleaseSpecialMoveClearsIt covers the refund path two call sites need when
// an action aborts after claiming.
func TestReleaseSpecialMoveClearsIt(t *testing.T) {
	c := &characters.Character{}
	ClaimSpecialMove(c)
	ReleaseSpecialMove(c)

	if !SpecialMoveReady(c) {
		t.Error("Release must return the cooldown to free")
	}
}
```

- [ ] **Step 3: Run it to verify it fails**

Run: `go test ./internal/actions/ -run TestSpecialMove -v`

Expected: compile error, `undefined: SpecialMoveReady`. That is the correct
first failure.

- [ ] **Step 4: Write the helper**

Create `internal/actions/special_move_cooldown.go`:

```go
package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
)

// SpecialMoveCooldownTag is the one place this key is spelled. It was typed by
// hand at 56 call sites before, which is how combat_helpers.go came to run a
// hardcoded "1 rounds" against everyone else's configured value with nothing
// able to notice.
const SpecialMoveCooldownTag = "special-move"

// SpecialMoveReady reports whether the shared special-move cooldown is free.
//
// ⚠️ Use this ONLY to decide what to offer or display -- an AI picking a move,
// a readiness listing. To actually take the cooldown, call ClaimSpecialMove and
// branch on its return. Asking here and claiming later is the split that hid
// the ranged bug: reload asked 47 lines before it took, and in that gap the
// ambush it was about to deny looked available.
func SpecialMoveReady(char *characters.Character) bool {
	return char.CooldownReady(SpecialMoveCooldownTag)
}

// ClaimSpecialMove takes the shared special-move cooldown, returning true when
// it was free and is now held. Asking and taking are one call on purpose.
//
// The duration always comes from Balance.SpecialMoveCooldown. Do not pass a
// literal: a hardcoded "1 rounds" in one path is precisely the drift this
// function exists to make impossible.
func ClaimSpecialMove(char *characters.Character) bool {
	cfg := configs.GetBalanceConfig()
	return char.TryCooldown(SpecialMoveCooldownTag,
		fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown))
}

// ReleaseSpecialMove returns the cooldown to free. It exists for actions that
// claim and then abort, so the player is not charged for a move that did not
// happen. It is NOT a way to dodge the cooldown.
func ReleaseSpecialMove(char *characters.Character) {
	delete(char.Cooldowns, SpecialMoveCooldownTag)
}
```

- [ ] **Step 5: Run it to verify it passes**

Run: `go test ./internal/actions/ -run TestSpecialMove -v`
Expected: PASS, three tests.

- [ ] **Step 6: Migrate the 16 check-then-claim verbs**

For each of `combat_bash.go`, `combat_drain.go`, `combat_gore.go`,
`combat_grapple.go`, `combat_hamstring.go`, `combat_kick.go`, `combat_maul.go`,
`combat_pounce.go`, `combat_rake.go`, `combat_rally.go`, `combat_taunt.go`,
`combat_throttle.go`, `combat_trip.go`, `combat_warcry.go` (and
`usercommands/throw.go`), the pattern is:

```go
	if !char.CooldownReady("special-move") {
		... refusal ...
	}
	... work ...
	if !char.TryCooldown("special-move", fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown)) {
		... rollback ...
	}
```

⚠️ **Keep both halves where they are.** The early check produces a refusal
message *before* any cost is admitted, and the late claim is where rollback
lives. Collapsing them changes behaviour, and this task must not. Replace only
the expressions:

```go
	if !SpecialMoveReady(char) {
		... refusal, unchanged ...
	}
	... work, unchanged ...
	if !ClaimSpecialMove(char) {
		... rollback, unchanged ...
	}
```

In `usercommands/throw.go` the calls read `user.Character`; pass
`user.Character` to the helpers and add the `actions` import if it is missing.

- [ ] **Step 7: Migrate the read-only sites**

`action_readiness.go:67,136`, `command_readiness.go:39` and
`behaviortree/action_cast_best_in_category.go:45` use
`GetCooldown("special-move") > 0`, which is the negation of ready:

```go
	if !SpecialMoveReady(char) {
```

The 12 sites in `combat/ai.go` use `_, exists := char.Cooldowns["special-move"]`.

⚠️ **That idiom is subtly different and you must not blind-swap it.** It is true
whenever the KEY EXISTS, including at value 0, whereas `SpecialMoveReady` is true
when the value is `<= 0`. A pruned-but-present key reads "on cooldown" to the old
code and "ready" to the helper. Check whether `Cooldowns.Prune()` deletes expired
keys or zeroes them:

```bash
sed -n '/func (cd Cooldowns) Prune/,/^}/p' internal/characters/cooldowns.go
```

If Prune **deletes**, the two are equivalent and you may swap. If it **zeroes**,
swapping silently makes mobs use special moves more often. In that case leave
`combat/ai.go` alone, and note in your report that it needs its own decision.

⚠️ **`internal/combat` may not import `internal/actions`.** Check for an import
cycle before touching those sites:

```bash
go list -deps ./internal/actions | grep -c "GoMud/internal/combat"
```

A non-zero count means `actions` already depends on `combat`, so `combat` cannot
import `actions`. If so, leave `combat/ai.go` on the raw idiom and say so in your
report; do not invent a third home for the constant in this task.

- [ ] **Step 8: Migrate the raw sites**

`combat_helpers.go:75` currently reads:

```go
	return !char.Cooldowns.Try("special-move", "1 rounds")
```

🔴 **This is the outlier.** Replacing it with `ClaimSpecialMove` changes the
cooldown it applies from 1 round to the configured 4. **That is a real behaviour
change**, and it is the one exception to this task being a pure refactor. Make it
deliberately, in its own commit, and say so in the message. Read the function it
sits in first to see what it gates.

`mutation_helpers.go:99` uses `Cooldowns.Try` with a computed duration string;
`:115` and `mobcommands/cast.go:109` use raw `delete`. Move the deletes to
`ReleaseSpecialMove`. If the mutation path deliberately uses a duration other
than the configured one, leave its `Try` call alone and add a comment saying why
it is not on the helper.

- [ ] **Step 9: Prove no hand-rolled sites remain where they should not**

```bash
grep -rn '"special-move"' --include=*.go internal/ | grep -v _test.go | \
  grep -v "special_move_cooldown.go"
```

Expected: only the sites you deliberately left (documented in your report), and
nothing else. Every remaining line needs a comment saying why it is exempt.

- [ ] **Step 10: Verify no behaviour changed**

```bash
go build ./... && go test ./...
gofmt -l internal/ modules/
```

Expected: PASS across the board. This is a refactor: **a failing test here means
you changed behaviour**, and the fix is to restore the old semantics at that
site, not to update the test. The one exception is `combat_helpers.go:75`, whose
1-to-4-round change may legitimately move a test; if so, say which and why.

- [ ] **Step 11: Commit**

```bash
git add internal/actions/special_move_cooldown.go internal/actions/special_move_cooldown_test.go
git commit -F - <<'MSG'
refactor(cooldown): one owner for the shared special-move timer

The key "special-move" was typed by hand at 56 call sites in five different
idioms: check-then-claim across sixteen verbs, a bare GetCooldown > 0, a raw
Cooldowns.Try, twelve raw map reads in combat/ai.go, and two raw deletes.

Nothing held them to the same rule, and they had already drifted:
combat_helpers.go ran a hardcoded "1 rounds" while every verb ran the
configured four, and no test could see it because there was nothing to compare
against.

The check-then-claim split is also the shape that produced the ranged bug this
work exists to fix. Reload asked whether the timer was free at line 86 and took
it at line 133, and the ambush it would deny lived in the gap between. Asking
and taking are one call now.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG

git add -u internal/
git commit -F - <<'MSG'
refactor(cooldown): move every special-move call site onto the helper

Mechanical migration, no behaviour change intended. The early readiness check
and the late claim stay exactly where they are in each verb: the check produces
a refusal before any cost is admitted, and the claim is where rollback lives.
Only the expressions change.

Sites deliberately left on the raw idiom are commented in place with the reason.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
```

---

## Task 2: Fold chambering into firing

The mechanical core of the ranged change. Everything after this is naming and
content.

**Files:**
- Modify: `internal/actions/combat_reload.go`
- Modify: `internal/actions/combat_fire.go`
- Test: `internal/actions/combat_fire_fold_test.go` (create)

- [ ] **Step 1: Read the two functions before touching either**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
sed -n '45,147p' internal/actions/combat_reload.go
sed -n '255,300p' internal/actions/combat_fire.go
```

You need `ExecuteReload`'s exact shape in mind: it admits cost, then **re-finds
the ammo bundle by identity** (`Equals`) rather than trusting the earlier index,
decrements `Uses`, sets `BundleEmptied`, sets `weapon.Loaded = true`, and then
claims the cooldown with a **full rollback** if the claim fails. All of that is
being preserved, not rewritten.

- [ ] **Step 2: Write the failing test**

Create `internal/actions/combat_fire_fold_test.go`. Follow the fixture style of
the existing `internal/actions/combat_fire_surprise_test.go` — read it first and
copy how it builds an actor, a target and a loaded weapon, rather than inventing
a harness:

```bash
sed -n '1,60p' internal/actions/combat_fire_surprise_test.go
```

Then write these three tests, substituting that file's real fixture helpers for
`newFireFixture` below:

```go
package actions

import "testing"

// TestFireChambersTheNextRound pins the whole point of the change: one command
// leaves the weapon ready. Before this, firing unloaded the weapon and the
// player had to spend a second command on `reload`.
func TestFireChambersTheNextRound(t *testing.T) {
	f := newFireFixture(t) // loaded weapon, one full ammo bundle, live target

	res := ExecuteFire(f.actor, f.targetName)

	if !res.Executed {
		t.Fatalf("shot did not execute: %+v", res)
	}
	if !f.weapon().Loaded {
		t.Error("weapon is empty after firing; the shot must chamber the next round")
	}
}

// TestFireBurnsExactlyOneSpecialMoveCooldown pins the cadence promise. A shot
// plus a reload cost ONE special-move burn before this change (spent by the
// reload); folding must not make ranged cheaper, and must not double-charge.
func TestFireBurnsExactlyOneSpecialMoveCooldown(t *testing.T) {
	f := newFireFixture(t)

	if !f.char().CooldownReady("special-move") {
		t.Fatal("fixture must start with the timer free")
	}

	ExecuteFire(f.actor, f.targetName)

	if f.char().CooldownReady("special-move") {
		t.Error("firing must claim the special-move cooldown")
	}
}

// TestAmbushNoLongerStrandsAnEmptyWeapon pins the collision nobody had written
// down. The surprise shot claimed the special-move timer, and reload GATED on
// that same timer being ready, so a successful ambush left the shooter unable to
// reload for SpecialMoveCooldown rounds. Chambering is part of the shot now, so
// the ambush cannot strand the weapon.
func TestAmbushNoLongerStrandsAnEmptyWeapon(t *testing.T) {
	f := newFireFixture(t)
	f.setSneaking(true)

	res := ExecuteFire(f.actor, f.targetName)

	if !res.Executed {
		t.Fatalf("ambush did not execute: %+v", res)
	}
	if !f.weapon().Loaded {
		t.Error("weapon is empty after an ambush; the ambush must not strand it")
	}
}
```

Then add the mob-parity test. Owner requirement: *"mob and player needs to have
parity. This can't be just a player only fix."* Nothing else in this plan proves
a mob gets the fold, and parity by shared seam is a claim worth pinning rather
than assuming. Follow the table-driven player/mob shape already used in
`internal/actions/action_cost_test.go:170-181`:

```go
// TestFireFoldsForMobsToo pins the owner's parity requirement. Both wrappers
// call ExecuteFire, so the fold reaches mobs by construction -- but "by
// construction" is exactly the kind of claim that stops being true when someone
// adds a wrapper-level shortcut, and a mob that cannot chamber simply stops
// shooting with nothing in any log.
func TestFireFoldsForMobsToo(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(t *testing.T) fireFixture
	}{
		{"player", newFireFixture},
		{"mob", newMobFireFixture},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := tc.build(t)

			res := ExecuteFire(f.actor, f.targetName)

			if !res.Executed {
				t.Fatalf("shot did not execute: %+v", res)
			}
			if !f.weapon().Loaded {
				t.Error("weapon is empty after firing; the fold must apply to this actor too")
			}
			if f.char().CooldownReady("special-move") {
				t.Error("firing must claim the special-move cooldown for this actor too")
			}
		})
	}
}
```

`newMobFireFixture` is the same fixture with `&MobActor{Mob: &mobs.Mob{
InstanceId: 22, Character: mobCharacter}}` in place of the `UserActor`. Build it
alongside `newFireFixture` in this file.

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/actions/ -run 'TestFireChambers|TestFireBurnsExactly|TestAmbushNoLonger|TestFireFoldsForMobs' -v`

Expected: `TestFireChambersTheNextRound` and `TestAmbushNoLongerStrandsAnEmptyWeapon`
FAIL on "weapon is empty after firing", because firing still unloads. If they
pass at this stage, the fixture is not actually firing — stop and report.

- [ ] **Step 4: Convert `ExecuteReload` into an internal step**

In `internal/actions/combat_reload.go`, rename `ExecuteReload` to
`chamberNextRound` and make it unexported. **Remove only the two cooldown
lines**, leaving every other line untouched:

- delete the read-only gate at `:86`:

```go
	if !char.CooldownReady("special-move") {
		return ReloadResult{WeaponName: weapon.DisplayName(), OnCooldown: true}
	}
```

- delete the claim at `:133` **and its rollback block**, so the tail reads:

```go
	weapon.Loaded = true
	result.Loaded = true
	return result
```

Put this comment above `chamberNextRound`:

```go
// chamberNextRound readies the next projectile. It is an internal STEP OF
// FIRING, not an action a player can take: `fire` resolves the shot and then
// calls this, so the two together are one action costing one special-move
// cooldown.
//
// ⚠️ IT MUST NOT TOUCH THE SPECIAL-MOVE COOLDOWN. It used to both gate on that
// timer and claim it, which produced a collision in BOTH directions: a recent
// reload denied the ambush that needed the timer, and a successful ambush then
// left the weapon unreloadable for the cooldown's length. ExecuteFire owns the
// single claim now. Re-adding a claim here re-creates both bugs.
//
// The bundle is deliberately re-found by identity (Equals) after cost admission
// rather than by the index found before it: admission calls through the actor
// seam, which a test double or a future synchronous hook can use to invalidate
// the inventory state underneath us.
```

Keep `OnCooldown` on `ReloadResult` for now; Task 3 removes it once nothing sets
it.

- [ ] **Step 5: Call it from `ExecuteFire`, and claim the cooldown once**

In `internal/actions/combat_fire.go`, find the surprise-shot cooldown claim at
`:277`:

```go
	surpriseShot := !crossRoom && result.IsSneaking
	if surpriseShot && !char.TryCooldown("special-move",
		fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown)) {
		// DENIED, and the player must be told (Task 14 speaks the line).
		surpriseShot = false
		result.SurpriseOnCooldown = true
	}
```

Replace it with an unconditional claim:

```go
	// ONE claim per shot, sneaking or not, through the Task 1 helper. Before the
	// fold, an ordinary shot claimed nothing and the separate `reload` claimed
	// instead, so a shot-plus-reload cycle already cost exactly one burn. It
	// still does: this is a command-count change, not a rate change.
	//
	// The claim does not gate the ambush. A shot from stealth is a surprise
	// strike because the shooter was hidden, full stop. The old code refused it
	// when the timer was already spent, which in practice meant refusing it
	// whenever the player had reloaded -- exactly the sequence needed to have a
	// loaded weapon to ambush with. SurpriseOnCooldown therefore has no way to
	// be set any more and is removed in the same change.
	surpriseShot := !crossRoom && result.IsSneaking
	ClaimSpecialMove(char)
```

🔴 **`surpriseShot` no longer depends on the cooldown, and that is the decision
being made here.** Being genuinely hidden is the only condition.

The old refusal existed to stop ambush-spam, but the shared timer was the wrong
instrument: a reload spent it, and you must reload to have anything to ambush
with. What actually limits ambushes is stealth itself — firing reveals you, and
`sneak` refuses while you are in combat (`usercommands.go:209`), so you get one
ambush per engagement no matter what the timer says.

The claim still matters: it is what the shot costs against the shared budget
`throw`, the mutations and every combat verb draw on.

**Do not "restore" the refusal** by branching on `ClaimSpecialMove`'s return
here. That re-creates the denial this work exists to remove.

Then, after the shot resolves and the weapon is unloaded, chamber the next round.
Put it immediately after the existing unload so the two read as one motion, and
carry the result out on `FireResult` for the wrapper to narrate:

```go
	// Chamber the next round as part of the same action. A failure here is NOT
	// a failed shot: the shot already landed. The weapon is simply left empty
	// and the next `fire` reports it.
	result.Chambered = chamberNextRound(actor)
```

Add to `FireResult`:

```go
	// Chambered reports the auto-reload that follows every shot. Its NoAmmo
	// flag is how "you are out" reaches the player; its BundleEmptied flag is
	// how "that was your last one" does. A shot with Chambered.Loaded false is
	// still a HIT -- the shot resolved before the chambering was attempted.
	Chambered ReloadResult
```

- [ ] **Step 6: Delete the now-unreachable `SurpriseOnCooldown`**

Nothing can set it. Remove the field from `FireResult` and let the compiler find
its readers; each one is a message the player can no longer receive.

⚠️ **`usercommands/shoot.go` speaks it** via `surpriseShotDeniedText` (around
`:237-250`). Delete that constant and its branch too. Its own comment explains
that the case was common *because* reload burned the timer — that cause is gone.

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test ./internal/actions/ -run 'TestFireChambers|TestFireBurnsExactly|TestAmbushNoLonger' -v`

Expected: PASS, three tests.

- [ ] **Step 8: Prove each test can fail**

This project has been bitten three times by tests that passed for the wrong
reason. One at a time, sabotage and confirm RED naming the right thing:

1. comment out `result.Chambered = chamberNextRound(actor)` -> the chamber tests go red
2. comment out the `char.TryCooldown(...)` line -> the cooldown test goes red

Restore after each and confirm green. **Paste all outputs in your report.**

- [ ] **Step 9: Run the package and build**

```bash
go build ./... && go test ./internal/actions/
gofmt -l internal/ modules/
```

Expected: build clean, tests pass, gofmt silent. `go build` will fail in
`usercommands`/`mobcommands` until Task 3 — that is expected; fix only what is
inside `internal/actions` here and note the remaining breakage in your report.

- [ ] **Step 10: Commit**

```bash
git add internal/actions/
git commit -F - <<'MSG'
feat(ranged): firing chambers the next round, and claims one cooldown

Firing unloaded the weapon, so every shot cost two commands. Worse, the two
commands fought over one timer: reload claimed the special-move cooldown that
the ambush opener needs, so the natural reload-sneak-shoot order denied the
ambush it was setting up.

The reverse held too and was never written down anywhere: reload also GATED on
that timer being ready, so a successful ambush claimed it and left the shooter
unable to reload for four rounds.

ExecuteReload becomes chamberNextRound, an internal step of firing that must
never touch the cooldown, and ExecuteFire claims it exactly once per shot
whether or not the shooter was sneaking. All of the reload's careful work is
preserved: cost admission, re-finding the bundle by identity after admission,
BundleEmptied, and the empty-bundle removal.

Cadence is unchanged on purpose. A shot-plus-reload cycle already cost one
burn, spent by the reload; it still costs one. This is a command-count change,
not a rate change.

FireResult.SurpriseOnCooldown is deleted because nothing can set it any more:
the ambush cannot be denied by a timer a reload spent, since there is no
separate reload.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
```

---

## Task 3: Retire the player `reload`, rename the command to `fire`

**Files:**
- Delete: `internal/usercommands/reload.go`, `internal/mobcommands/reload.go`
- Create: `internal/usercommands/admin.reload.go`
- Modify: `internal/usercommands/shoot.go`, `internal/usercommands/usercommands.go`
- Modify: `internal/mobcommands/shoot.go`, `internal/mobcommands/mobcommands.go`
- Modify: `internal/actions/combat_reload.go` (drop `OnCooldown`)

- [ ] **Step 1: Move the admin half to its own file**

`internal/usercommands/reload.go:34-64` holds the admin data-file reload
(`items`, `biomes`, `translations`, `mapcache`, `help`), gated by
`user.HasRolePermission("reload", true)`. Create
`internal/usercommands/admin.reload.go` containing **only** that switch, as a
`Reload` function with the same signature, minus the fall-through to the player
path:

```go
// Reload reloads a server data file in place. This is the ADMIN command only.
//
// It used to share its name and file with a player-facing ranged-weapon reload,
// separated by a role check. That player command is gone: firing now chambers
// the next round itself, so `reload` means exactly one thing again.
func Reload(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
```

Keep the existing subcommand switch body verbatim. Replace the old default
fall-through with the existing unknown-subcommand message.

- [ ] **Step 2: Delete both reload command files**

```bash
git rm internal/usercommands/reload.go internal/mobcommands/reload.go
```

- [ ] **Step 3: Rename the command functions to `Fire`**

In `internal/usercommands/shoot.go`, rename `func Shoot(` to `func Fire(`. In
`internal/mobcommands/shoot.go`, do the same. Leave the file names as
`shoot.go` — renaming files is churn the compiler does not need, and the
registry is the source of truth for command names.

- [ ] **Step 4: Flip the registries**

`internal/usercommands/usercommands.go`: change `:203` from

```go
		`shoot`:           {Shoot, false, true, false},
```

to

```go
		`fire`:            {Fire, false, true, false},
```

and change `:181` from

```go
		`reload`:          {Reload, false, true, false},     // All: ranged-weapon reload; admin subcommands reload data files
```

to

```go
		`reload`:          {Reload, true, true, false},      // Admin only: reload a server data file. The player half retired when firing began chambering its own next round.
```

⚠️ **Check the struct's field order before flipping that second boolean.** Read
the type at the top of `usercommands.go` and set the admin/permission field, not
whichever field happens to be second. If the struct has no admin flag, leave the
booleans as they are — `Reload` already checks `HasRolePermission` internally,
and that check is what actually gates it.

`internal/mobcommands/mobcommands.go`: change `:89` `"shoot": {Shoot, false},`
to `"fire": {Fire, false},` and **delete** `:77` `"reload": {Reload, false},`.

- [ ] **Step 5: Make `shoot` the alias**

In `_datafiles/world/dogmud/keywords.yaml:318`, change

```yaml
  shoot:              ['fire']
```

to

```yaml
  fire:               ['shoot']
```

- [ ] **Step 6: Move the quest notify string**

In `internal/usercommands/shoot.go:207-211`, change `Command: "shoot"` to
`Command: "fire"`, and update the comment above it to say `fire`. Add:

```go
	// ⚠️ This string is the quest contract, and it is HARDCODED rather than
	// taken from what the player typed -- `shoot` is an alias and still
	// arrives here. Quests 50, 51 and 59 gate on it. If you change it, change
	// their `command:` triggers in the same commit or the beats stop firing
	// silently.
```

- [ ] **Step 7: Drop the dead `OnCooldown` field**

In `internal/actions/combat_reload.go`, remove `OnCooldown` from `ReloadResult`.
Nothing sets it after Task 2. Let the compiler find any reader.

- [ ] **Step 8: Simplify the archer tree**

In `internal/behaviortree/actions_archer.go`, the reload decision is now only an
ammo question. Delete `unloadedRangedWeapon` and keep `hasMatchingAmmo`, updating
its doc comment:

```go
// hasMatchingAmmo reports whether the character carries an ammo bundle whose
// AmmoTag matches the supplied weapon ammo tag. Firing chambers its own next
// round, so the tree no longer decides WHETHER to reload -- only whether the
// mob has anything left to shoot.
```

Let the compiler point at the node that called `unloadedRangedWeapon` and remove
that branch.

- [ ] **Step 9: Build, and fix what the compiler finds**

```bash
go build ./...
```

Every error is a caller of a thing that no longer exists. Expected sites: the
deleted `Reload` player wrappers, `SurpriseOnCooldown` readers, `OnCooldown`
readers, and `unloadedRangedWeapon`. Fix each by deletion, not by re-adding the
removed concept.

- [ ] **Step 10: Run the affected packages**

```bash
go test ./internal/actions/ ./internal/usercommands/ ./internal/mobcommands/ ./internal/behaviortree/
gofmt -l internal/ modules/
```

Expected: PASS, gofmt silent. Existing shoot/reload tests will need their calls
renamed to `Fire`; that is a rename, not a behaviour change. If any test asserts
that reload is a player command, delete that test and say so in your report.

- [ ] **Step 11: Commit**

```bash
git add -u internal/ _datafiles/world/dogmud/keywords.yaml
git add internal/usercommands/admin.reload.go
git commit -F - <<'MSG'
feat(ranged): `fire` is the command; the player `reload` retires

`fire` was already an alias of `shoot`, so this is a registry flip rather than
new plumbing: `fire` becomes canonical and `shoot` becomes its alias, in both
the player and mob registries, so existing habits and mob YAML keep working.

The player-facing reload is gone, since firing chambers its own next round. The
admin data-file reload survives and moves to its own file: one command was
doing two unrelated jobs separated by a role check, and retiring the player
half un-overloads the verb as a side effect.

The quest notify string moves to "fire" in the same commit as the quest
triggers that depend on it. It is hardcoded rather than read from what the
player typed, so it cannot follow a rename on its own, and quests 50, 51 and 59
gate on it.

The archer behaviour tree loses its reload decision and keeps only its ammo
check. FireResult.SurpriseOnCooldown and ReloadResult.OnCooldown are gone: with
one claim per shot and no separate reload, nothing can set either.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
```

---

## Task 4: The recovery line

**Files:**
- Modify: `_datafiles/world/dogmud/combat-messages/shooting.yaml`
- Modify: `internal/usercommands/shoot.go`

- [ ] **Step 1: Read the file's existing shape**

```bash
sed -n '1,40p' _datafiles/world/dogmud/combat-messages/shooting.yaml
```

Note the token vocabulary in the header comment (`{itemname}`, `{source}`,
`{target}`, ...) and the `optionid` / `options` nesting. Match it.

- [ ] **Step 2: Add the three recovery lines**

Append a `recovery:` block under `options:`, keyed by ammo tag:

```yaml
  # Recovery lines for the chambering that follows every shot. Keyed on the
  # weapon's ammo tag, which is the only discriminator available: all eight
  # ranged weapons share `subtype: shooting`. That is FINER-grained than the
  # attack messages above, which key on subtype alone, so a sling and a firearm
  # already share every attack line in this file.
  recovery:
    arrows:
      toattacker:
        - 'You draw and nock another shaft in one motion.'
    bolts:
      toattacker:
        - 'You crank the string back and seat another bolt.'
    shot:
      toattacker:
        - 'You settle another shot into the cradle.'
```

⚠️ The `shot` line covers a sling and two firearms, so it is worded to fit
both. Do not make it sling-specific.

- [ ] **Step 3: Speak it after the shot**

In `internal/usercommands/shoot.go`, after the shot's own narration and before
the counter dispatch, send the recovery line when `result.Chambered.Loaded` is
true, and the out-of-ammo line when it is not:

```go
	// The recovery reads as part of the same motion as the shot, so it follows
	// the shot's own narration immediately and before any counter.
	if result.Chambered.Loaded {
		user.SendText(messaging.CategoryCombat,
			items.GetRecoveryMessage(result.Chambered.AmmoTag))
	} else if result.Chambered.NoAmmo {
		user.SendText(messaging.CategorySystem,
			`You reach for another and find nothing. You are out of ammunition.`)
	} else if result.Chambered.BundleEmptied {
		user.SendText(messaging.CategorySystem,
			`That was the last of them.`)
	}
```

Add the `GetRecoveryMessage(ammoTag string) string` lookup to
`internal/items/attack_messages.go`, following `GetAttackMessage`'s existing
shape and returning a safe empty string for an unknown tag.

⚠️ **`BundleEmptied` and `NoAmmo` are different.** `BundleEmptied` means this
shot consumed the last round *and the weapon is now loaded with it*; `NoAmmo`
means there was nothing to chamber. The order above matters: a successful
chamber that emptied the bundle should say both, so check `Loaded` first.

- [ ] **Step 4: Verify by rendering, not by reasoning**

Build and run the server, equip a sling with a pouch of shot, and fire until the
pouch is empty. Confirm the recovery line appears once per shot, that "that was
the last of them" appears exactly once, and that the next `fire` reports no
ammunition.

```bash
go build -o dogmud.exe . && ./dogmud.exe
```

Connect on telnet 33333. **Do not use `go run .`** — a fixed binary path keeps
one Windows Firewall rule valid.

- [ ] **Step 5: Commit**

```bash
git add _datafiles/world/dogmud/combat-messages/shooting.yaml internal/usercommands/shoot.go internal/items/attack_messages.go
git commit -F - <<'MSG'
feat(ranged): the shot and its recovery read as one motion

A folded action that narrated only the shot would leave the player unsure the
weapon had reloaded, and out-of-ammo would arrive as a surprise. Each shot now
ends with a short recovery line, and running dry says so.

The lines are keyed on ammo tag, which is finer-grained than the attack
messages beside them: those key on weapon subtype, and all eight ranged weapons
are `subtype: shooting`, so a sling and a relic sidearm already share every
attack line in this same file. The `shot` line is worded to fit both a sling
and a firearm for that reason.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
```

---

## Task 5: Quests 50, 51 and 59

Content, and the highest risk of a silent break: a quest whose trigger no longer
matches simply stops granting, with nothing in the log.

**Files:**
- Modify: `_datafiles/world/dogmud/quests/50-first_shot.yaml`
- Modify: `_datafiles/world/dogmud/quests/51-across_the_canyon.yaml`
- Modify: `_datafiles/world/dogmud/quests/59-range_practice.yaml`
- Modify: `_datafiles/world/dogmud/dialogue/pothole_coulee/9161.yaml`

- [ ] **Step 1: The easy two first**

In `51-across_the_canyon.yaml:80` and `59-range_practice.yaml:54`, change
`command: shoot` to `command: fire`. Change nothing else in either file.

- [ ] **Step 2: Rewrite quest 50's triggers**

Delete the whole `RELOAD` trigger block (`50-first_shot.yaml:80-91`). Replace the
`SHOOT` block with two shot beats:

```yaml
  # ── FIRST SHOT: loose at the practice butt down-range (5351) ───────────
  - event: command
    command: fire
    room: 5351
    conditions:
      has: ["50-start"]
      missing: ["50-shoot"]
    actions:
      - grant: "50-shoot"
      - send_text: >-
          The stone leaves the sling and strikes the straw with a solid
          thud. Your hand has already found another and settled it in the
          cradle. Iden calls down: "Now do it again, but from where it
          cannot see you. Type sneak first."

  # ── AMBUSH: the same shot again, but from cover ───────────────────────
  # Fires on the `sneak` command rather than on `fire`, so the beat is granted
  # for LEARNING TO HIDE. The ambush itself is then the payoff the player sees
  # in the damage, not another token to chase.
  - event: command
    command: sneak
    room: 5351
    conditions:
      has: ["50-shoot"]
      missing: ["50-ambush"]
    actions:
      - grant: "50-ambush"
      - send_text: >-
          You settle out of sight behind the butts. The sling is already
          loaded, because it loaded itself. Fire from here and see what a
          shot is worth when nothing is expecting it.
```

✅ **Every character can sneak from creation.** `characters.New()` calls
`initAllSkills()`, which seeds **every** skill at rank 1
(`internal/characters/character.go:443-451`), and `ensureAllSkills()` floors
existing saves at 1 during `Validate()`. `sneak` gates on `Skullduggery >= 1`
(`skill.skullduggery.sneak.go:23-26`), so it is available immediately and no
quest needs to grant it.

Owner, 2026-09-07: *"Everyone starts with skullduggery. We just haven't
encouraged or taught players to use it."* That is the reason this beat is worth
spending: the mechanic is available to every player from their first minute and
nothing currently points at it.

⚠️ **The beat triggers on `sneak`, not on the second `fire`.** Gating it on a
shot taken while hidden would require the quest engine to know the shot was an
ambush, which the `command` event does not carry. Granting on the hide keeps the
trigger inside what the engine can already see, and the ambush damage is its own
reward.

- [ ] **Step 3: Rewrite quest 50's steps and reward text**

Replace the `reload` step with a `second` step and rewrite the prose so nothing
instructs the player to reload. The `start` hint currently reads:

```yaml
    hint: >-
      Type ask iden shot to get ammunition. Type equip sling to ready it,
      and reload to chamber a stone. Then go east to the Long Terrace and
      type shoot butt to drop the practice butt. Return west to Marksman
      Iden -- type ask iden done.
```

It becomes:

```yaml
    hint: >-
      Type ask iden shot to get ammunition. Type equip sling to ready it.
      Then go east to the Long Terrace and type fire butt. Sneak and fire
      once more to see what a shot from cover is worth. Return west to
      Marksman Iden -- type ask iden done.
```

Replace the `reload` step entirely:

```yaml
  - id: ambush
    map_target: 5351
    description: >-
      You put a stone into the butt and the sling readied itself. Iden wants
      you to try it once more from cover, where the target does not know you
      are there.
    hint: >-
      Type sneak on the Long Terrace to slip out of sight, then fire butt
      again and watch what the shot is worth. Then report to Marksman Iden
      -- type ask iden done.
```

And the `shoot` step's description and hint, which currently tell the player a
ranged weapon is empty after every shot, become:

```yaml
  - id: shoot
    map_target: 5350
    description: >-
      You put a stone into the butt, and the sling readied itself without
      being told. Try it once more from cover.
    hint: >-
      Type sneak on the Long Terrace, then fire butt again. Report to
      Marksman Iden afterward -- type ask iden done.
```

The reward `playermessage` teaches the retired loop in as many words
(*"shoot, then reload, shoot, then reload"*). Replace that sentence:

```yaml
  playermessage: >-
    Iden watches the stone bury itself in the straw and gives a single nod.
    "There. A sling readies itself while your eye is still on the mark -- so
    the only thing you have to think about is the mark. The eye is the thing
    that matters now. Not the arm -- the eye. Keep the sling and the shot;
    they are yours." She tips her head north, toward the canyon mouth. "Bryn
    holds the slot below the raider bluffs. He will show you the next part --
    how to put a shot into something a whole room away, before it ever knows
    you are there."
```

Also update the token-flow comment block at the top of the file (`:1-17`) so it
describes `50-shoot` and `50-ambush` rather than `50-reload`, and no longer says
the quest teaches reloading.

- [ ] **Step 4: Move the turn-in gate**

`_datafiles/world/dogmud/dialogue/pothole_coulee/9161.yaml:144` reads:

```yaml
      questRequired: ["50-reload", "50-shoot"]
```

Change it to:

```yaml
      questRequired: ["50-shoot", "50-ambush"]
```

⚠️ **If this is missed, quest 50 becomes uncompletable** — the turn-in waits on
a token nothing grants any more.

- [ ] **Step 5: Verify no reference to the dead token survives**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
grep -rn "50-reload" _datafiles/ docs/ || echo "  clean"
grep -rn "command: shoot" _datafiles/world/dogmud/quests/ || echo "  no shoot triggers left"
```

Expected: both clean. Then confirm every quest file still parses:

```bash
python -c "
import glob, yaml
for f in sorted(glob.glob('_datafiles/world/dogmud/quests/*.yaml')):
    yaml.safe_load(open(f, encoding='utf-8'))
print('all quest files parse')
"
```

- [ ] **Step 6: Guard the notify contract with a test**

The notify string is hardcoded in Go and the triggers live in YAML, so the two
can drift apart silently — a quest that stops granting logs nothing. This is the
same silent-data-contract family as the mob `items:` key that left 17 mobs empty.

First extract the string to a named constant in `internal/usercommands/shoot.go`,
replacing the literal from Task 3 Step 6:

```go
// questNotifyCommand is the command name the quest engine is told about when a
// shot resolves. It is HARDCODED rather than taken from what the player typed,
// because `shoot` is an alias and still arrives here. Quests 50, 51 and 59 gate
// their beats on this exact string; ranged_quest_contract_test.go fails if the
// YAML and this constant ever disagree.
const questNotifyCommand = "fire"
```

Then create `internal/usercommands/ranged_quest_contract_test.go`:

```go
package usercommands

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gopkg.in/yaml.v2"
)

// TestRangedQuestTriggersMatchTheNotifyConstant pins the contract between the
// Go notify string and the quest YAML that gates on it. Nothing else connects
// them: the notify is hardcoded, so a rename on either side leaves the other
// pointing at a command that never fires, and a quest that silently stops
// granting produces no error anywhere.
func TestRangedQuestTriggersMatchTheNotifyConstant(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))

	// The ranged spoke: First Shot, Across the Canyon, Range Practice.
	for _, name := range []string{
		"50-first_shot.yaml",
		"51-across_the_canyon.yaml",
		"59-range_practice.yaml",
	} {
		path := filepath.Join(root, "_datafiles", "world", "dogmud", "quests", name)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		var q struct {
			Triggers []struct {
				Event   string `yaml:"event"`
				Command string `yaml:"command"`
			} `yaml:"triggers"`
		}
		if err := yaml.Unmarshal(raw, &q); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}

		found := 0
		for _, tr := range q.Triggers {
			if tr.Event != "command" || tr.Command == "" {
				continue
			}
			found++
			if tr.Command != questNotifyCommand {
				t.Errorf("%s: trigger command %q does not match questNotifyCommand %q",
					name, tr.Command, questNotifyCommand)
			}
		}
		if found == 0 {
			t.Errorf("%s: no command triggers found -- this test would pass vacuously", name)
		}
	}
}
```

⚠️ The `found == 0` check is deliberate. Without it the test passes on a file
whose triggers were renamed to something the loop never inspects, which is the
exact failure it exists to catch.

- [ ] **Step 7: Prove the guard can fail**

Temporarily change one trigger in `51-across_the_canyon.yaml` back to
`command: shoot`, run the test, and confirm it goes RED naming that file. Restore
and confirm green. Paste both outputs.

Run: `go test ./internal/usercommands/ -run TestRangedQuestTriggers -v`

- [ ] **Step 8: Commit**

```bash
git add _datafiles/world/dogmud/quests/ _datafiles/world/dogmud/dialogue/pothole_coulee/9161.yaml internal/usercommands/shoot.go internal/usercommands/ranged_quest_contract_test.go
git commit -F - <<'MSG'
content: quest 50 teaches one verb; 51 and 59 follow the rename

Quest 50 existed to teach the reload loop, in as many words: "shoot, then
reload, shoot, then reload." That loop is gone, so the tutorial was teaching a
command that no longer exists.

The first beat teaches the verb. The second teaches the ambush, which is the
part of ranged worth knowing and which nothing currently points at: every
character can already sneak, because initAllSkills seeds every skill at rank 1
on creation and ensureAllSkills floors older saves at 1, so `sneak`'s
Skullduggery gate has always been satisfied for everyone. No quest grants
skullduggery because none needs to.

The ambush beat triggers on `sneak` rather than on a second shot. Gating it on
"a shot taken while hidden" would need the quest engine to know the shot was an
ambush, which the command event does not carry; granting on the hide keeps the
trigger inside what the engine can already see, and the ambush damage is its own
reward.

The reload step, its token, and every instruction to reload are removed,
including from Iden's parting speech.

Iden's turn-in gate moves from 50-reload to the new token. Missing that would
have left the quest uncompletable, waiting on a token nothing grants.

Quests 51 and 59 only needed their command triggers moved to `fire`, in the
same commit as the notify string they depend on.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
```

---

## Task 6: Help pages

⚠️ **Help templates parse LAZILY and template function names resolve at PARSE
time**, so a typo passes both `go build` and boot and reaches a player as
`[TEMPLATE ERROR]`. Every edit here is verified by RENDERING the page.

**Files:**
- Modify: `_datafiles/world/dogmud/templates/help/shoot.template`
- Modify: `_datafiles/world/dogmud/templates/help/reload.template`
- Modify: `_datafiles/world/dogmud/templates/help/ranged-combat.template`
- Modify: `_datafiles/world/dogmud/keywords.yaml`

- [ ] **Step 1: Read all three pages first**

```bash
cat _datafiles/world/dogmud/templates/help/shoot.template
cat _datafiles/world/dogmud/templates/help/reload.template
cat _datafiles/world/dogmud/templates/help/ranged-combat.template
grep -n "shoot\|reload\|ranged" _datafiles/world/dogmud/keywords.yaml
```

- [ ] **Step 2: Rename the shoot page to fire**

```bash
git mv _datafiles/world/dogmud/templates/help/shoot.template \
       _datafiles/world/dogmud/templates/help/fire.template
```

Rewrite its body so it describes one action: `fire <target>` in the same room,
`fire <target> <direction>` into an adjacent one, that the weapon readies itself
afterward, and that running out of ammunition is what stops you rather than a
separate reload step. Keep the existing `<ansi>` styling and the page's heading
shape. **No raw numbers, no em or en dashes, wrap under 80 visible columns.**

- [ ] **Step 3: Make the reload page admin-only**

`reload.template` becomes a short page describing only the data-file reload
subcommands, with a line pointing players at `help fire`:

```
  If you came here looking to reload a weapon: you do not need to. Firing
  readies the next round by itself. See <ansi fg="command">help fire</ansi>.
```

- [ ] **Step 4: Rewrite ranged-combat**

Remove every instruction to reload from `ranged-combat.template` and describe the
single action. Cross-link `help fire`.

- [ ] **Step 5: Move the help topics and aliases**

In `keywords.yaml`, rename the `shoot` help topic entry to `fire` in whatever
category list holds it, keeping the list alphabetical. Confirm the alias flip
from Task 3 Step 5 is present.

- [ ] **Step 6: Render every page rather than trusting the boot**

Start the server and run each of: `help fire`, `help shoot`, `help reload`,
`help ranged-combat`, `help`.

Expected: `help fire` and `help shoot` render the same page; no page shows
`[TEMPLATE ERROR]`; `help` lists `fire`; the reload page reads as admin-only.

⚠️ A boot test will **not** catch a template typo. Only rendering will.

- [ ] **Step 7: Commit**

```bash
git add _datafiles/world/dogmud/templates/help/ _datafiles/world/dogmud/keywords.yaml
git commit -F - <<'MSG'
docs(help): one verb, one action

The shoot page becomes the fire page and describes a single action: fire in the
room, fire into an adjacent one, and the weapon readies itself. The reload page
keeps only the admin data-file subcommands and points players at help fire.
ranged-combat no longer instructs anyone to reload.

Every edited page was checked by rendering it in game. Help templates parse
lazily, so a typo passes build and boot and reaches a player as a template
error.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
```

---

## Task 7: Full verification, patch notes, and the playtest gate

- [ ] **Step 1: Format, build, full suite**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
gofmt -l internal/ modules/     # must print NOTHING
go build ./...
go test ./...
```

Expected: PASS. Two recorded flakes may appear and are NOT regressions:
`TestCheckConcentrationBreak_ProgressionFiresOnEveryResolvedContest` ("fixture
never held") and any single combat miss, both from a ~2.3% self-relative attack
fumble. Re-run before investigating either.

- [ ] **Step 2: Wipe instance saves and boot-test in an isolated worktree**

The server must be DOWN for the rooms wipe.

```bash
rm -rf _datafiles/world/dogmud/mobs.instances _datafiles/world/dogmud/rooms.instances
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

**Exit code 124 is the SUCCESS case.** Do not grep for the bare word `panic` —
`GamePlay.MapConsistencyEnforce` legitimately has the *value* `panic`. Clean up
with `git worktree remove --force C:/tmp/dogmud-boot-check`; if Windows holds a
lock, `rm -rf` then `git worktree prune`.

- [ ] **Step 3: Confirm the mob side in play**

Parity was the owner's explicit requirement and no unit test proves a mob
narrates correctly. Find an archer mob (`9167-bluff_marksman`,
`9168-raider_sharpshooter`), let it shoot at you, and confirm it fires repeatedly
without a reload turn and does not lock up when its ammo runs out.

- [ ] **Step 4: Write the patch notes entry**

Add to the top of `docs/PATCH_NOTES.md`, below the heading and above the current
first entry. Player-facing framing, no raw numbers, no em dashes:

```markdown
## 2026-09-07: Ranged weapons ready themselves now

Shooting used to take two commands. You fired, the weapon went empty, and you
typed reload before you could fire again. In a fight that meant half your
attention went on housekeeping, and the worst of it was that reloading and
setting up an ambush drew from the same well: if you reloaded and then tried to
sneak up on something, the ambush you were setting up quietly refused to happen.
It also worked the other way. Land a perfect surprise shot and you could not
reload for a while afterward, which is the exact moment you most wanted to.

Fire is now the whole thing. You shoot, and your hands find the next round on
their own, and the weapon is ready. One command, and the ambush works whenever
you have set one up. If you run dry, the game tells you so rather than leaving
you to find out when nothing happens.

Marksman Iden teaches the new way, and will not send you off reciting the old
one.
```

- [ ] **Step 5: Commit the patch notes**

```bash
git add docs/PATCH_NOTES.md
git commit -F - <<'MSG'
docs: patch notes for the ranged one-verb change

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
```

- [ ] **Step 6: Adversarial playtest — REQUIRED, not optional**

This task authors player-facing content (help pages, quest dialogue, combat
messaging), so the project's Content Playtest-Review Gate applies. Boot-clean
verifies the system, never the experience.

⚠️ **Seed the character.** Two recorded constraints will otherwise eat the run:
Thornwall merchants sleep on NPC schedules, and a fresh character cannot walk to
the Pothole Coulee firing range inside a 25 minute budget at three commands per
round. Write a goals file with `ephemeral.profile` set to a stocked character
placed at the range (5350/5351) rather than `creation_flow: true`.

Run it with an explicitly critical mandate and drive:

- `fire butt`, twice, reading every line: does the recovery read as one motion
  with the shot, or as two events?
- Quest 50 end to end, including Iden's turn-in.
- Firing until the pouch is empty: is running dry clear before it happens?
- `help fire`, `help shoot`, `help reload`, `help ranged-combat`, `help`.
- An archer mob shooting at you.

Fix what it finds, re-run if needed, and only then hand it to the owner.

- [ ] **Step 7: Ship through a PR**

```bash
git push -u origin HEAD
gh pr create --repo pruuk/DOGMud --base master --head "$(git branch --show-current)" --fill
gh pr checks <n> --repo pruuk/DOGMud --watch
```

⚠️ Always pass `--repo pruuk/DOGMud`. This repo is a fork and `gh` defaults to
the **upstream parent**. A green check is not proof on its own; confirm with
`gh run view <id> --repo pruuk/DOGMud --log-failed` before merging, then:

```bash
gh pr merge <n> --repo pruuk/DOGMud --merge --delete-branch
```

Use `--merge`, not `--squash`. **Do not deploy** — the owner runs deploys.

---

## Done when

0. The shared special-move cooldown has exactly one owner, every call site that
   can use it does, and the hardcoded `"1 rounds"` outlier is gone — Task 1.
1. `fire <target> [direction]` resolves a shot and chambers the next round in one
   action, for players and mobs alike, through `actions.ExecuteFire` — Task 2.
2. Exactly one special-move cooldown is burned per shot, sneaking or not, pinned
   by `TestFireBurnsExactlyOneSpecialMoveCooldown` — Task 2.
3. Reload can no longer deny an ambush, and an ambush can no longer strand an
   empty weapon, the latter pinned by `TestAmbushNoLongerStrandsAnEmptyWeapon` —
   Task 2.
4. The player `reload` command is gone; the admin one still works from its own
   file — Task 3.
5. `fire` is canonical, `shoot` is its alias, in both registries — Task 3.
6. The shot and its recovery read as one motion, and running dry says so —
   Task 4.
7. Quests 50, 51 and 59 fire their beats; quest 50 teaches the verb then the ambush, and its
   turn-in gate matches the tokens it grants — Task 5. The Go notify string and
   the YAML triggers are pinned to each other by
   `TestRangedQuestTriggersMatchTheNotifyConstant` — Task 5.
7b. Mob parity is pinned by `TestFireFoldsForMobsToo`, not only observed in play
   — Task 2.
8. Help describes one verb, verified by rendering rather than by booting —
   Task 6.
9. Boot clean, full suite green, mob parity confirmed in play, and the content
   playtest gate run against a seeded character at the range — Task 7.

## Out of scope

- Retiring `Item.Loaded`. It still distinguishes a fresh weapon from a chambered
  one and carries the out-of-ammo signal.
- Per-weapon flavour beyond the three ammo tags. Attack messages already lump a
  sling in with a firearm; splitting them is one change to the message system
  covering shot and recovery together.
- Renaming the admin `reload` verb. Retiring the player half already removes the
  ambiguity.
- Touching `throw`. It shares the special-move cooldown and is deliberately
  unchanged.
- Retuning any ranged damage knob. `SurpriseRangedStrikeMultiplier` (0.5) and
  `RangedUnengagedDamageMultiplier` (2.75) keep their shipped values.
