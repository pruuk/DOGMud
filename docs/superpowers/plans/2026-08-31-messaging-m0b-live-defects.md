# Messaging M0b — Live Defects: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop two defects that are silently degrading play today, before the
snapshot harness freezes them as the baseline.

**Architecture:** Two small, independent production changes. The first gives
`combat.RenderChannelDefenceMessages` a generic fallback so a missing message
pool degrades to plain text instead of silence, copying a pattern that already
exists in `internal/combat/counter.go`. The second teaches the messaging sight
predicates about `buffs.Sleeping` so a sleeping player stops receiving visual
broadcasts. Both are written so the full three-way perception verdict planned for
M2/M4 remains straightforward.

**Tech Stack:** Go, standard `testing`.

**Spec:** [`2026-08-31-messaging-unification-design.md`](../specs/2026-08-31-messaging-unification-design.md), stage M0b.

---

## Facts verified against source, 2026-08-31

| Fact | Evidence |
|---|---|
| Six sites gate on `if triad.ToRoom == ""`, but only **four go silent** | `internal/combat/counter.go:190` and `:249` fall back to `fillGenericCounterMessages` / `buildGenericCounterTauntMessages` |
| The four silent ones all call **one function** | `combat.RenderChannelDefenceMessages` (`internal/combat/defence_multiplier.go:286`), from `hooks/spell_resolution.go:483`, `mobcommands/taunt.go:108`, and both copies of `skill_move_defence.go` |
| `counter.go` is unaffected by a fix there | it calls `items.RenderDefenseMessage` **directly**, not the channel wrapper |
| The empty triad originates one level down | `items.RenderDefenseMessage` returns `DefenseMessageTriad{}` on a nil pool, a missing intensity band, or audience lists of unequal length (`internal/items/defensive_messages.go:107`) |
| The pool lookup is a raw string cast | `items.DefenseType(out.DefenceType)` at `defence_multiplier.go:290` compiles for any string and yields nil for one with no pool |
| The fallback pattern to copy | `fillGenericCounterMessages` (`internal/combat/counter.go:206`) builds all three audience strings with `fmt.Sprintf` and a damage description |
| Sleep is a buff flag | `buffs.Sleeping Flag = "sleeping"` (`internal/buffs/buffspec.go:59`) |
| The pipeline never consults it | grep for `Sleeping` under `internal/messaging/` and in `internal/rooms/rooms.go` returns **nothing** |
| The sight predicates to change | `messaging.CanSeeClearly` (`internal/messaging/predicates.go:24`) and `CanSeeShapes` (`:44`) |
| Audio is unaffected by design | `Room.SendText` bypasses the sight gate; wake-on-shout lives in `shout.go` and must keep working |

---

## Task 1: A missing message pool degrades to generic text, never silence

**Files:**
- Modify: `internal/combat/defence_multiplier.go`
- Test: `internal/combat/defence_multiplier_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestRenderChannelDefenceMessages_UnknownPoolFallsBackToGenericText(t *testing.T) {
	// A defence type with no authored pool. Today this yields an empty triad,
	// and every caller's `if triad.ToRoom == ""` guard then discards the
	// attacker and defender lines too -- the whole event goes silent while the
	// mechanics resolve normally.
	out := ChannelDefenceResult{
		Defended:                 true,
		DefenceType:              "no-such-defence-pool",
		NormalizedDefenceMargin:  0.6,
	}
	triad := RenderChannelDefenceMessages(out,
		ChannelDefenceIdentities{Attacker: "Grimwald", Defender: "Meirok"},
		"searing bolt")

	if triad.ToRoom == "" || triad.ToAttacker == "" || triad.ToDefender == "" {
		t.Fatalf("unknown pool must fall back to generic text, got %+v", triad)
	}
	for name, msg := range map[string]string{
		"ToRoom":     string(triad.ToRoom),
		"ToAttacker": string(triad.ToAttacker),
		"ToDefender": string(triad.ToDefender),
	} {
		if !strings.Contains(msg, "searing bolt") {
			t.Errorf("%s should name the attack, got %q", name, msg)
		}
	}
	if !strings.Contains(string(triad.ToRoom), "Meirok") ||
		!strings.Contains(string(triad.ToRoom), "Grimwald") {
		t.Errorf("room line should name both actors, got %q", triad.ToRoom)
	}
}

func TestRenderChannelDefenceMessages_UndefendedStaysEmpty(t *testing.T) {
	// The attack WON. There is no defence to narrate, and the fallback must not
	// invent one. This is the direction a careless fix breaks.
	triad := RenderChannelDefenceMessages(
		ChannelDefenceResult{Defended: false},
		ChannelDefenceIdentities{Attacker: "Grimwald", Defender: "Meirok"},
		"searing bolt")
	if triad.ToRoom != "" || triad.ToAttacker != "" || triad.ToDefender != "" {
		t.Fatalf("an undefended attack must render nothing, got %+v", triad)
	}
}
```

- [ ] **Step 2: Run it and watch the first test fail**

Run: `go test ./internal/combat/ -run TestRenderChannelDefenceMessages -v`

Expected: `UnknownPoolFallsBackToGenericText` FAILS on the empty triad.
`UndefendedStaysEmpty` PASSES already, which is what makes it a real guard on
the fix rather than a restatement of it.

- [ ] **Step 3: Add the fallback**

In `internal/combat/defence_multiplier.go`, replace the body of
`RenderChannelDefenceMessages` after the `!out.Defended` early return:

```go
	triad := items.RenderDefenseMessage(items.DefenseType(out.DefenceType),
		out.DefensiveCrit, out.NormalizedDefenceMargin, map[items.TokenName]string{
			items.TokenAttacker: identities.Attacker,
			items.TokenDefender: identities.Defender,
			items.TokenAttack:   attack,
			items.TokenWeapon:   attack,
		}, indexOverride...)

	// A defence HAPPENED, so it must be narrated. An empty triad here means the
	// pool lookup failed -- a missing pool, a missing intensity band, or
	// audience lists of unequal length -- and callers gate on triad.ToRoom, so
	// returning empty silences the event for the attacker and the defender too,
	// not just the room. The mechanics still resolve, so the player sees a
	// spell simply stop happening. Degrade to plain text instead, the way
	// counter.go already does with fillGenericCounterMessages.
	if triad.ToRoom == "" {
		logMissingDefencePool(out.DefenceType)
		return genericDefenceTriad(identities, attack)
	}
	return triad
}

// genericDefenceTriad is the last-resort narration for a defence whose authored
// pool could not be resolved. Deliberately plain: it names who, whom and what,
// and nothing else. It exists so a data gap costs flavour, never silence.
func genericDefenceTriad(identities ChannelDefenceIdentities, attack string) items.DefenseMessageTriad {
	return items.DefenseMessageTriad{
		ToDefender: items.ItemMessage(fmt.Sprintf(
			`You turn aside %s's %s.`, identities.Attacker, attack)),
		ToAttacker: items.ItemMessage(fmt.Sprintf(
			`%s turns aside your %s.`, identities.Defender, attack)),
		ToRoom: items.ItemMessage(fmt.Sprintf(
			`%s turns aside %s's %s.`, identities.Defender, identities.Attacker, attack)),
	}
}
```

Add `"fmt"` to the imports if it is not already present.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/combat/ -run TestRenderChannelDefenceMessages -v`

Expected: both PASS.

- [ ] **Step 5: Run the whole combat package**

Run: `go test ./internal/combat/`

Expected: PASS. If an existing test asserted that an unknown pool produces
silence, that test encoded the bug — read it, and say so rather than deleting it
quietly.

- [ ] **Step 6: Commit**

```bash
git add internal/combat/defence_multiplier.go internal/combat/defence_multiplier_test.go
git commit -m "fix(combat): a missing defence message pool degrades to text, not silence"
```

---

## Task 2: Make the missing pool visible to developers

A data gap should not be silent to us either. Right now nothing anywhere reports
that a pool lookup failed.

**Files:**
- Modify: `internal/combat/defence_multiplier.go`

- [ ] **Step 1: Find the logging idiom this repo uses**

Run: `grep -rn "mudlog\." --include=*.go internal/combat/ | head -5`

Use whatever that shows. Do not introduce a different logging package.

- [ ] **Step 2: Implement `logMissingDefencePool`, logging each type once**

```go
// missingDefencePools remembers which defence types have already been reported,
// so a pool that is missing during a long fight logs once rather than on every
// swing. Guarded because combat rounds run concurrently with other work.
var (
	missingDefencePoolsMu sync.Mutex
	missingDefencePools   = map[string]bool{}
)

// logMissingDefencePool reports an unresolvable defence message pool exactly
// once per defence type per process. The lookup is a raw string cast
// (items.DefenseType(out.DefenceType)), so a renamed or unauthored type fails
// silently at runtime -- this is the only signal that it happened.
func logMissingDefencePool(defenceType string) {
	missingDefencePoolsMu.Lock()
	defer missingDefencePoolsMu.Unlock()
	if missingDefencePools[defenceType] {
		return
	}
	missingDefencePools[defenceType] = true
	mudlog.Warn("defence messages", "missing pool", defenceType,
		"effect", "falling back to generic narration")
}
```

Adjust the `mudlog` call to match the signature the grep in Step 1 revealed.
Add `"sync"` to the imports.

- [ ] **Step 3: Verify it compiles and the tests still pass**

Run: `go build ./... && go test ./internal/combat/ -run TestRenderChannelDefenceMessages`

Expected: both clean.

- [ ] **Step 4: Commit**

```bash
git add internal/combat/defence_multiplier.go
git commit -m "feat(combat): log an unresolvable defence message pool once per type"
```

---

## Task 3: A sleeping player stops receiving visual broadcasts

**Files:**
- Modify: `internal/messaging/predicates.go`
- Test: `internal/messaging/predicates_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestCanSeeClearly_SleeperSeesNothing(t *testing.T) {
	c := newTestCharacter(t)          // use whatever helper this package already has
	addSleepingBuff(t, c)             // see Step 2 for finding the right helper
	if CanSeeClearly(c, litRoom{}) {
		t.Error("a sleeping character must not see clearly, even in a lit room")
	}
	if CanSeeShapes(c, litRoom{}) {
		t.Error("a sleeping character must not see shapes either")
	}
}

func TestCanSeeClearly_AwakeInLitRoomStillSees(t *testing.T) {
	// The direction a careless fix breaks: everyone stops seeing anything.
	c := newTestCharacter(t)
	if !CanSeeClearly(c, litRoom{}) {
		t.Error("an awake character in a lit room must still see clearly")
	}
}
```

- [ ] **Step 2: Find the package's existing test helpers before writing new ones**

Run: `grep -n "func newTest\|RoomVisibility\|GetVisibility" internal/messaging/predicates_test.go | head`

Reuse what is there. If the package has no character builder, construct one the
way the neighbouring tests do rather than inventing a fixture. To set the flag,
check how other tests apply a buff: `grep -rn "AddBuff" internal/messaging/`.

- [ ] **Step 3: Run and watch the sleeper test fail**

Run: `go test ./internal/messaging/ -run TestCanSeeClearly -v`

Expected: `SleeperSeesNothing` FAILS; `AwakeInLitRoomStillSees` PASSES.

- [ ] **Step 4: Add the sleep check to both predicates**

In `CanSeeClearly`, immediately after the existing Blinded check:

```go
	// Asleep is a perception state, not a movement state. The pipeline never
	// consulted it, so a sleeping player received every visual broadcast in the
	// room -- NPC dialogue, ambient flavour, all of it. Audio is deliberately
	// unaffected: Room.SendText bypasses this gate, so a shout still reaches a
	// sleeper and still wakes them (shout.go owns that).
	if observer.HasBuffFlag(buffs.Sleeping) {
		return false
	}
```

`CanSeeShapes` already returns early when `CanSeeClearly` is true, but it must
not fall through to the infrared branch for a sleeper, so add the same check
there after its own Blinded check.

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/messaging/`

Expected: PASS.

- [ ] **Step 6: Confirm the wake paths still work**

Sleep is enforced elsewhere and those paths must not regress.

Run: `go test ./internal/usercommands/ ./internal/mobcommands/ ./internal/rooms/`

Expected: PASS. If a test fails because a sleeper no longer receives a visual
message it used to, that is the fix working; read the test and decide whether
the message should have been audio all along, and say so.

- [ ] **Step 7: Commit**

```bash
git add internal/messaging/predicates.go internal/messaging/predicates_test.go
git commit -m "fix(messaging): a sleeping character no longer receives visual broadcasts"
```

---

## Task 4: Verification and patch notes

**Files:**
- Modify: `docs/PATCH_NOTES.md`

- [ ] **Step 1: Full build and test**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/combat/ ./internal/messaging/ ./internal/hooks/ ./internal/usercommands/ ./internal/mobcommands/
```

Expected: `gofmt` silent, everything else passing.

- [ ] **Step 2: Add the patch note**

Player-facing framing, no raw numbers, no em dashes, wrapped at 80. Append at
the top of `docs/PATCH_NOTES.md` under the title:

```markdown
## 2026-08-31: Spells stop vanishing, and sleep means sleep

Some spells and defences simply produced no text at all. The effect landed, the
health changed, and nobody was told anything, which made whole abilities look
like they were doing nothing. That happened whenever a message was missing
behind the scenes, and it silenced the line for everyone in the room rather than
just the one that was absent. Missing text now falls back to a plain
description, so you always find out what happened.

Sleeping also means sleeping now. Room chatter and the comings and goings of
people around you no longer reach you while you are asleep. Anything loud enough
to wake you still does.
```

- [ ] **Step 3: Boot test in an isolated detached worktree**

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
echo "exit=$?   (124 means the server stayed up: SUCCESS)"
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

Clean up with `git worktree remove --force C:/tmp/dogmud-boot-check`, then
`rm -rf` and `git worktree prune` if Windows holds a lock.

- [ ] **Step 4: Commit**

```bash
git add docs/PATCH_NOTES.md
git commit -m "docs: patch notes for the M0b live defect fixes"
```

---

## Done when

1. An unresolvable defence pool renders generic text for all three audiences,
   and an undefended attack still renders nothing.
2. A missing pool is logged once per defence type.
3. A sleeping character fails both sight predicates; an awake one in a lit room
   still passes.
4. `gofmt` clean, `go build ./...` clean, the five listed packages pass.
5. Boot test exits 124 with zero panics and one `Server Ready`.
6. `docs/PATCH_NOTES.md` updated.

## Explicitly NOT in M0b

- **The `{source_plain}` anonymizer leak** — owner sequenced it into M5.
- **The crime-in-the-dark gate** — M5, and blocked on the `NightVision`
  decision, since nothing in the game currently grants that flag.
- **The full three-way perception verdict.** Task 3 is a targeted fix inside the
  existing predicates. Consolidating darkness, blindness and sleep into one
  verdict that crime witnessing also reads is M2/M4 work. Do not start it here,
  and do not write Task 3 in a way that makes it harder.
- **The cross-room sniping bug.** Same `CancelCombatBuffs` family, but a
  different defect with its own root cause, and not messaging.
