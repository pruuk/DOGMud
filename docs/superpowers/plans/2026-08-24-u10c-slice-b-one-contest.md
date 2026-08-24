# U10c Slice B — One Contest, On The Right Channel

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Charm stops running a private contest on top of the one its cast already runs, and the contest it keeps is answered by defy instead of quell.

**Scope:** This slice does **not** give charm a duration. Charm stays permanent when it lands. That is deliberate — see "Why this is a slice" below.

**Spec:** `docs/superpowers/specs/2026-08-24-u10c-charm-redesign-design.md` — sections 11, 12 and 13 supersede 1-10; 13 supersedes 11.3.2.

---

## Why this is a slice, and not a task in a bigger plan

Two full ten-task plans for U10c were written and both were rejected by blind
adversarial review — v1 for a false premise about the seam, v2 for three
confident claims about existing code that were wrong (the wrong hook cited for
logout, a function that has no target parameter, and prose prescribing a guard
its own code omitted).

This project already knew better. `2026-08-21-u10b-0-README.md`:

> "Later phases are planned after their predecessor lands, not before —
> planning six phases in detail up front is what produced the three failed
> versions."

So: one slice, planned against the code as it is today, small enough that every
claim in it was verified while writing it.

**Slice A is MERGED** (`9b8fa2d51`): `ChannelDefenceResult.AttackerNormalizedMargin`.

**After this slice, still to plan:**
- **Slice C** — the clock and the grudge (duration from margin, expiry, the
  `CompanionCharmed` gate, the online-and-present gate, the lane split).
- **Slice D** — the guards (`EverCharmed` instance-save, shop regression test),
  player copy, patch notes, playtest gate, ship.

Do not start C or D from this document.

---

## Verified facts (checked while writing, on master `9b8fa2d51`)

1. **Charm already runs a seam contest.** `spellAttackChannel`
   (`spell_resolution.go:1076-1081`) maps an **absent** `target_defense_type`
   to `ChannelSpellMental`; the mob loop (`:129-143`) calls `resolveAgainstMob`
   unconditionally. Its verdict is then discarded in
   `applyMobEffect_default` and a private `RunContest` in `resolveCharmSpell`
   decides the outcome.
2. **`applyMobEffect` already receives that verdict.** Signature ends
   `out combat.ChannelDefenceResult` (`:811`); it switches on
   `spellData.EffectType` (`:822`); `critTag` and `mName` are in scope (`:812`,
   `:820`).
3. **`applyMobEffect` is called with `user == nil`** from
   `resolveMobSpellAgainstMob` (`:1359`, `:1374`). Its docstring says so and
   every sibling arm guards. Mob-cast charm is currently excluded by
   `behaviortree/action_cast_best_in_category.go:181`, so this is latent — but
   the arm must guard anyway.
4. **`spellAttackSideFor(spellData, casterChar)`** (`:282`) takes **no target**
   and is called once per cast at `:83`, *above* target selection. The
   in-combat penalty cannot live there.
5. **`resolveAgainstMob`** (`:328`) is the only place with both the target and
   the `side` in scope before `runSpellChannelAttack` at `:332`.
6. **`SituationalAttackMult` returns a flat 1.0 for `ChannelSocial`**
   (`internal/combat/situational.go`, switch covers melee and ranged only).
   `spellAttackSideFor:302` already applies it. Charm does not "gain" it.
7. **`counter.go:139-141` documents that `ChannelSocial` never reaches
   `ExecuteCounter`**, because taunt short-circuits its defy-crit into
   `executeCounterTaunt` at the call site (`actions/combat_counter.go:103-129`).
   `fireSpellCounterTier` (`hooks/counter_tier.go:33-42`) has no such
   carve-out.
8. **The player-target branch shortcuts on `TargetDefenseType == ""`**
   (`:161-164`). Declaring `social` moves charm to `resolveAgainstPlayer`,
   which runs a real contest — and `applyPlayerEffect` has no charm arm.
9. **The post-loop charm block is `:226-233`** (`:226` is the comment, `:233`
   the closing brace). It ignores the loop's dead/absent/protected filters and
   always fires on `TargetMobInstanceIds[0]`.
10. **`isSummonOrCharm` suppresses the no-targets fallback** (`:171-174`), so
    moving charm into the loop means a target that left the room produces
    **no message at all** unless one is added.
11. **`charm_spell.go` imports** `fmt, math, characters, combat, contest,
    messaging, mobs, rooms, skills, users` — **not `spells`**.
12. **`resolveCharmSpell`'s body holds five things worth keeping** beyond the
    contest: the `AddCompanion` failure branch (`:93-96`), the loop clearing
    other companions' aggro toward the new pet (`:100-111`), the caster's
    `EndAggro` (`:113-116`), the success narration (`:118-123`), and the room
    failure line (`:132-134`).

---

### Task 1: Decide and record what happens to charm cast at a PLAYER

**This is a design gate, not code. Do it first — Task 3 is unsafe without it.**

Fact 8: declaring `target_defense_type: social` moves a player-targeted charm
from an uncontested shortcut onto a real `ChannelSocial` contest that charges
the victim conviction for defy and awards them rhetoric — for an effect that
does nothing, because `applyPlayerEffect` has no charm arm.

The helpfile says *"Charm cannot be used on other players"*
(`charm.template:30`) and **no code enforces it**.

- [ ] Verify the claim yourself:

```bash
grep -rn '"charm"' internal/ --include=*.go
grep -n "TargetDefenseType == \"\"" internal/hooks/spell_resolution.go
sed -n '99,116p' internal/actions/cast.go
```

- [ ] Choose, and write the choice into the spec as section 14 before coding:

**Recommended: enforce what the helpfile already promises.** Reject a
player target for `EffectType == "charm"` at cast time, with a clear refusal.
That makes the helpfile true, costs one guard, and removes the whole branch
from this slice's blast radius.

The alternative — letting charm work on players — is a PvP design decision far
outside U10c and must not be taken by accident.

- [ ] Commit the spec change before writing code.

---

### Task 2: Teach the channel router about `social`

**Files:** `internal/hooks/spell_resolution.go`; test in `internal/hooks/`.

- [ ] **Step 1: Write the failing test**

```go
func TestSpellAttackChannel_SocialRoutesToChannelSocial(t *testing.T) {
	cases := []struct {
		name string
		tdt  string
		want combat.AttackChannel
	}{
		{"social", "social", combat.ChannelSocial},
		{"physical", "physical", combat.ChannelSpellPhysical},
		{"absent stays mental", "", combat.ChannelSpellMental},
		{"unknown stays mental", "nonsense", combat.ChannelSpellMental},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := spellAttackChannel(&spells.SpellData{TargetDefenseType: c.tdt}); got != c.want {
				t.Errorf("spellAttackChannel(%q) = %v, want %v", c.tdt, got, c.want)
			}
		})
	}
	// A nil SpellData must not panic; the current implementation guards.
	if got := spellAttackChannel(nil); got != combat.ChannelSpellMental {
		t.Errorf("spellAttackChannel(nil) = %v, want ChannelSpellMental", got)
	}
}
```

- [ ] **Step 2: Run it — the `social` case fails, the other three pass.**

- [ ] **Step 3: Add the case**

```go
	case "social":
		// Charm is an act of social domination whose attack side is already
		// Charisma, so defy answers it rather than quell. Declaring the channel
		// in data is what lets charm stop hand-rolling its own contest.
		//
		// NOTE for anyone adding a second social spell: this channel also
		// reaches fireSpellCounterTier, which combat/counter.go documents as
		// unreachable for ChannelSocial. See Task 4.
		return combat.ChannelSocial
```

- [ ] **Step 4: Run, then commit.**

---

### Task 3: Move the charm effect into `applyMobEffect`

**Files:** `internal/hooks/spell_resolution.go`, `internal/hooks/charm_spell.go`; test in `internal/hooks/`.

- [ ] **Step 1: Read the whole body being replaced — all of it**

```bash
sed -n '25,140p' internal/hooks/charm_spell.go
```

v2 of this plan read only to `:120` and silently dropped the room failure
message at `:132-134`. Read to the end of the function.

- [ ] **Step 2: Write the failing tests**

```go
// One cast, ONE contest. Before this slice a charm ran two: the cast's
// ChannelSpellMental contest, whose verdict was discarded, and a private
// RunContest in resolveCharmSpell.
func TestCharm_RunsExactlyOneChannelContest(t *testing.T) { /* count runner calls across a full cast */ }

// applyMobEffect is called with a nil user from resolveMobSpellAgainstMob.
// Every sibling arm guards; this one must too, even though no mob casts charm
// today.
func TestApplyMobEffectCharm_NilUserDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("applyMobEffect_charm panicked on a nil user: %v", r)
		}
	}()
	_ = applyMobEffect_charm(nil, testMob(t), testRoom(t), charmSpellData(t),
		combat.ChannelDefenceResult{}, "Something")
}

// A defended charm must narrate to the ROOM, not just the caster. The old
// path did (via applyMobEffect_default plus resolveCharmSpell's room line);
// losing it would make a failed 36-fold cast invisible to bystanders.
func TestCharm_DefendedNarratesToRoom(t *testing.T) { /* assert room text sent */ }
```

- [ ] **Step 3: Write `applyMobEffect_charm`**

Signature must match how `applyMobEffect` will call it, using names in scope
there (`user`, `mob`, `room`, `spellData`, `out`, `critTag`, `mName`):

```go
func applyMobEffect_charm(user *users.UserRecord, mob *mobs.Mob, room *rooms.Room,
	spellData *spells.SpellData, out combat.ChannelDefenceResult, critTag, mName string) int
```

**Required, each for a reason found in review:**

- **Guard `user == nil` first** and return 0. Fact 3.
- **Add the `spells` import** to `charm_spell.go`. Fact 11. If `spellData` ends
  up unused, drop the parameter instead of importing.
- **Translate, do not "preserve verbatim".** The old body names `targetMob` and
  `targetName`; the new one has `mob` and `mName`. Every carried line needs
  renaming, and `charm_spell.go:95`'s `return false` becomes `return 0`.
- **Carry all five items from fact 12**, not the four v2 listed. The
  companion-aggro loop is the one most easily lost, and losing it leaves your
  other companions attacking your new one.
- **Narrate a defended outcome through the shared path**, as every sibling arm
  does:
  ```go
  if out.Defended {
      sendSpellChannelDefenceMessages(room, spellSchoolCategory(spellData), out,
          spellDefenceIdentity(user.Character, user, room), mName, spellData.Name, user, nil)
      return 0
  }
  ```
  Verify that signature before using it: `grep -n "func sendSpellChannelDefenceMessages" -A 4 internal/hooks/spell_resolution.go`.
- **Use `critTag`** in the success line, as sibling arms do, or deliberately
  decide charm shows no crit tag and say why in a comment.
- **Do not check `out.AttackerFumble`.** `resolveAgainstMob` returns at `:349`
  on a fumble, before `applyMobEffect`, so that operand is dead on this path.

- [ ] **Step 4: Add the switch arm** in `applyMobEffect`, before `default`:

```go
	case "charm":
		return applyMobEffect_charm(user, mob, room, spellData, out, critTag, mName)
```

- [ ] **Step 5: Delete the post-loop block** at `spell_resolution.go:226-233`
      (verify the range first — v2 had it as `:226-234`, one line long).

- [ ] **Step 6: Restore feedback for a target that left**

Fact 10. Moving into the loop means a target that died, left the room, or
gained protection now yields **no message at all**, after a 36-fold channel and
120 conviction. Add a fizzle line for `EffectType == "charm"` when the loop
resolved zero targets — near the `isSummonOrCharm` suppression at `:171-174`,
which exists to stop the *generic* no-target message.

This is a real pre-existing bug being fixed, not just preserved: today charm
succeeds against a mob that has already left the room. Say so in the commit.

- [ ] **Step 7: Delete the private contest** from `charm_spell.go` — the attack
      score, the defence score, the aggro-penalty block, and the `RunContest`
      call. Let the compiler find newly-unused imports (`math` and `contest`
      are the likely casualties; `skills` may survive).

- [ ] **Step 8: Verify and commit**

```bash
gofmt -l internal/ && go build ./... && go test ./internal/hooks/ ./internal/combat/ -count=1
```

---

### Task 4: Handle the counter tier reaching a channel it documents as unreachable

**Files:** `internal/hooks/counter_tier.go` and/or `internal/combat/counter.go`.

Fact 7. Once charm is `ChannelSocial`, a defy-crit against it reaches
`fireSpellCounterTier`, which has no carve-out — so the mob answers a charm
with a **physical melee swing** narrated from `counter-melee.yaml`, and
`counter.go`'s comment becomes false.

- [ ] **Step 1: Confirm the reachability yourself**

```bash
sed -n '130,145p' internal/combat/counter.go
sed -n '30,45p'  internal/hooks/counter_tier.go
sed -n '100,130p' internal/actions/combat_counter.go
```

- [ ] **Step 2: Choose and implement**

Either give `fireSpellCounterTier` the same defy carve-out taunt has (so a
defy-crit against charm counter-*taunts*, using `counter-defy.yaml`), or
suppress the counter tier for `ChannelSocial` spells and record why.

**Do not leave `counter.go`'s comment standing if it becomes false** — that
comment is load-bearing documentation for the next reader.

- [ ] **Step 3: Test that a defy-crit against charm does not produce a melee
      counter-swing. Commit.**

---

### Task 5: Retire the arc's allowlist rows

**Files:** `internal/combat/contest_site_guard_test.go`.

- [ ] **Step 1:**

```bash
grep -n "charm" internal/combat/contest_site_guard_test.go
```

Three hits, not two: `:76` and `:77` are `contestSiteOwners` rows naming U10c
as owner; `:331` is a comment in the guard-3 exemption list.

- [ ] **Step 2: Delete only the row whose site this slice removed.**
      `resolveCharmSpell`'s `RunContest` is gone after Task 3, so
      `"internal/hooks/charm_spell.go:resolveCharmSpell"` goes.
      **`tickMobCharmState`'s row must STAY** — the ladder is Slice C's to
      delete, and removing the row early turns the guard red in the other
      direction.

- [ ] **Step 3: Add `charm_spell.go` to `legacyLiteralFiles`** only if its
      `× 25` is gone. Check the reader's behaviour on a missing file first
      (`sed -n '370,382p' internal/combat/contest_site_guard_test.go`) — it
      `t.Fatalf`s, so a file listed there may not later be deleted.

- [ ] **Step 4: `go test ./internal/combat/ -count=1`. Commit.**

---

### Task 6: Gates

- [ ] `gofmt -l internal/ modules/` prints nothing.
- [ ] `go build ./...`; `go test` for `combat`, `hooks`, `actions`,
      `characters`, `usercommands`, `mobcommands`.
- [ ] `golangci-lint` — no **new** finding on a touched file.
- [ ] Boot test in an isolated detached worktree on non-default ports.
      `Server Ready` = 1, panic patterns = 0. **Exit 124 is success.**
- [ ] **Manual check over telnet, because the message changes are the point:**
      cast charm at a mob and read every line. Confirm you no longer see a
      resist line *and* a success line for the same cast — that contradiction
      is what deleting the discarded contest fixes, and no unit test sees it.
- [ ] **No patch notes for this slice.** It changes plumbing and removes a
      contradiction; the player-facing story lands with Slice C's clock.
- [ ] PR with `--repo pruuk/DOGMud`. Confirm each job ran with zero
      annotations. Merge `--merge`.

---

## For whoever executes this

If a `grep` or `sed` finds something this document did not predict, **stop and
report.** Two prior plans for this slice were rejected for exactly one such
unpredicted fact each, and in both cases the plan sounded most confident
precisely where it was wrong.
