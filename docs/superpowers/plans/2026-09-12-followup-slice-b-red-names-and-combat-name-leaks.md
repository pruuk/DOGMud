# Follow-up slice B: red names and combat name leaks, implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Names stop rendering red for everyone, and the two combat lines that still named an unseen combatant (recoil, retarget) go quiet about who.

**Architecture:** No new concepts. The `aggro` suffix gains the precondition it always needed (a real viewer). The recoil lines move onto `messaging.SendTrio` exactly as slice A moved the crit lines (`internal/hooks/crit_effect_trio.go` is the template), and the seam learns to swallow the adjective span that follows a formatted name. The retarget notice gets one builder that hides the name by the reader's sight, shared by its two call sites.

**Tech Stack:** Go, `internal/characters`, `internal/messaging`, `internal/hooks`, testify.

Spec: `docs/superpowers/specs/2026-09-12-followup-slice-b-red-names-and-combat-name-leaks-design.md`

---

## Read this first

- **Test binaries never load `config.yaml`**; nothing here depends on a balance knob, but do not add one.
- **A failing test must be seen failing.** Every task runs its test red before the fix. If a test passes before the fix, the test is wrong, not the fix unnecessary.
- **The hooks fixture** (`seedAllRegistries()` in `internal/hooks/hooks_test.go:37`): users 1 "Aliceia" and 2 "Bobrick" in room 1, mob instance 100 "Skeleton" in room 1. `darken(t, 1)` makes room 1 an unlit cave and asserts it. `drainPlain(userId)` returns a user's queued lines tag-stripped; `plainText(line)` strips one line; `countContaining(lines, needle)` counts matches. Infrared for a user: `users.GetByUserId(2).Character.Buffs.AddBuff(heatEyesBuffId, true)` after `restore := seedNarrationBuffs(); defer restore()`.
- **Assign a nil `messaging.Recipient` as the interface, never a typed nil pointer** (see the comment on `messaging.Audience`).
- Commit messages end with `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>`. Never `git add -A`; name the paths.

## File structure

| File | Change |
|---|---|
| `internal/characters/formattedname.go` | `getFormattedName`: aggro needs `viewingUserId > 0` |
| `internal/characters/formattedname_test.go` | Add the viewer test |
| `internal/messaging/anonymize_test.go` | Fix a comment that documents the old bug as fact |
| `internal/messaging/hidenames_tagged.go` | Swallow one trailing adjective span with the identity tag |
| `internal/messaging/hidenames_test.go` | Add the adjective-span case |
| `internal/hooks/NewRound_DoCombat_unified.go` | `emitReturnDamageText` through `SendTrio`; `emitRetargetMessage` uses the builder |
| `internal/hooks/return_damage_trio_test.go` | Create: recoil tests |
| `internal/hooks/combat_retarget.go` | Add `retargetNotice` |
| `internal/hooks/combat_retarget_notice_test.go` | Create: retarget tests |
| `internal/hooks/NewRound_DoCombat.go` | The validate-aggro retarget uses the builder |
| `internal/characters/context.md`, `internal/messaging/context.md`, `internal/hooks/context.md` | Document the three changes |
| `docs/PATCH_NOTES.md` | Player-facing entry |
| `tools/playtest/goals/2026-09-12-slice-b-dark-recoil-and-plain-names.yaml` | Create: the playtest lane |

---

### Task 1: The aggro suffix needs a real viewer

**Files:**
- Modify: `internal/characters/formattedname.go` (`getFormattedName`, the `else if` choosing `aggro`)
- Test: `internal/characters/formattedname_test.go`
- Modify: `internal/messaging/anonymize_test.go:71-76` (comment only)

- [ ] **Step 1: Write the failing test**

Append to `internal/characters/formattedname_test.go`:

```go
// The aggro suffix means "this character is fighting YOU". Viewer 0 is a room
// broadcast with no single reader, and a character with no target (or a mob
// target) reports target UserId 0, so the old equality test painted nearly
// every name red in every viewer-0 line.
func TestGetFormattedName_AggroNeedsARealViewer(t *testing.T) {
	util.SetRoundCountForTest(100)
	defer util.ResetRoundCountForTest()

	// Health above zero, or the dead suffix wins before aggro is considered.
	idle := &Character{Name: "Grix", Health: 10}
	assert.Equal(t, "", idle.GetMobName(0).Suffix, "no target, viewer 0: not aggro")
	assert.NotContains(t, idle.GetCharacterName(true), "-aggro")

	vsMob := &Character{Name: "Grix", Health: 10}
	vsMob.SetAggro(0, 55, DefaultAttack, 0)
	assert.Equal(t, "", vsMob.GetMobName(0).Suffix, "mob target, viewer 0: not aggro")

	vsPlayer := &Character{Name: "Grix", Health: 10}
	vsPlayer.SetAggro(7, 0, DefaultAttack, 0)
	assert.Equal(t, "aggro", vsPlayer.GetMobName(7).Suffix, "the player it is fighting sees aggro")
	assert.Equal(t, "", vsPlayer.GetMobName(8).Suffix, "another player does not")
	assert.Equal(t, "", vsPlayer.GetMobName(0).Suffix, "a room broadcast does not")
}
```

Add `"github.com/GoMudEngine/GoMud/internal/util"` to the imports. `SetAggro(userId, mobInstanceId, attackType, rounds)` is how `taunt_hold_test.go:21` sets a target.

- [ ] **Step 2: Run it red**

Run: `go test ./internal/characters/ -run TestGetFormattedName_AggroNeedsARealViewer -v`
Expected: FAIL on the first assertion (`Suffix` is `"aggro"` for the idle character).

- [ ] **Step 3: Fix the primitive**

In `getFormattedName`, replace

```go
	} else if c.CurrentCombatTarget().UserId == viewingUserId {
		f.Suffix = `aggro`
	}
```

with

```go
	} else if target := c.CurrentCombatTarget(); viewingUserId > 0 && target.UserId == viewingUserId {
		// aggro means "fighting YOU". Viewer 0 is a broadcast with no single
		// reader, and a character with no target reports UserId 0, so without
		// the viewer check every idle character rendered red to everyone.
		f.Suffix = `aggro`
	}
```

- [ ] **Step 4: Run it green, then the package**

Run: `go test ./internal/characters/ -run TestGetFormattedName_AggroNeedsARealViewer -v` then `go test ./internal/characters/`
Expected: PASS, PASS.

- [ ] **Step 5: Correct the comment in `anonymize_test.go`**

Lines 71-76 say `GetCharacterName(true)` "renders `username-aggro` for any character not fighting a player, so the suffixed form is the COMMON one". Replace that sentence with: "Until slice B, `GetCharacterName(true)` rendered `username-aggro` for any character not fighting a player, so the suffixed form was the COMMON one in room text; it is now the form a player reads for a foe fighting them." The test itself is unchanged.

- [ ] **Step 6: Commit**

```bash
git add internal/characters/formattedname.go internal/characters/formattedname_test.go internal/messaging/anonymize_test.go
git commit -m "fix(names): the aggro colour needs a real viewer" -m "getFormattedName compared the combat target's UserId with the viewer id, and both are 0 for a room broadcast about a character not fighting a player, so nearly every name rendered username-aggro (bright red). The suffix now requires a viewer." -m "Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 2: The seam swallows the adjective span behind a hidden name

> **Correction after review (2026-09-12):** the first version of this task used `\([^<]*\)`, which never matches a real adjective because `CompileAdjectiveSwaps` colour-patterns every rune into its own tag. The regex and test below are the corrected ones; the test must carry the shipped nested-tag shape.

**Files:**
- Modify: `internal/messaging/hidenames_tagged.go`
- Test: `internal/messaging/hidenames_test.go`

`characters.FormattedName.String` prints `<ansi fg="mobname">Skeleton</ansi> <ansi fg="black-bold">(dead)</ansi>`. `hideTaggedName` replaces the identity tag and leaves the adjectives, so an unsighted reader would read "something (dead)".

- [ ] **Step 1: Write the failing test**

Append to `internal/messaging/hidenames_test.go`:

```go
// FormattedName.String prints a character's adjectives in a black-bold span
// right after the identity tag. Hiding the name and leaving "(dead)" or
// "(♥friend)" behind tells an unsighted reader what they could not see.
func TestHideNames_AdjectiveSpanGoesWithTheTag(t *testing.T) {
	const anon = "<ansi fg=\"combat-anon\">"
	cases := []struct {
		name  string
		text  string
		sight SightDecision
		want  string
	}{
		{
			name:  "one adjective",
			text:  "You recoil from striking <ansi fg=\"mobname\">Skeleton</ansi> <ansi fg=\"black-bold\">(dead)</ansi>!",
			sight: SightNone,
			want:  "You recoil from striking " + anon + "something</ansi>!",
		},
		{
			name:  "adjectives after a duplicate index",
			text:  "<ansi fg=\"mobname-dup2\">Skeleton #2</ansi> <ansi fg=\"black-bold\">(♥friend|hidden)</ansi> recoils.",
			sight: SightShapes,
			want:  anon + "A figure</ansi> recoils.",
		},
		{
			name:  "a black-bold span that is not adjectives stays",
			text:  "<ansi fg=\"mobname\">Skeleton</ansi> <ansi fg=\"black-bold\">hisses</ansi>.",
			sight: SightNone,
			want:  anon + "Something</ansi> <ansi fg=\"black-bold\">hisses</ansi>.",
		},
		{
			name:  "clear sight leaves everything",
			text:  "<ansi fg=\"mobname\">Skeleton</ansi> <ansi fg=\"black-bold\">(dead)</ansi> recoils.",
			sight: SightFull,
			want:  "<ansi fg=\"mobname\">Skeleton</ansi> <ansi fg=\"black-bold\">(dead)</ansi> recoils.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := HideNames(tc.text, []string{"skeleton"}, tc.sight); got != tc.want {
				t.Fatalf("HideNames =\n  %q\nwant\n  %q", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run it red**

Run: `go test ./internal/messaging/ -run TestHideNames_AdjectiveSpanGoesWithTheTag -v`
Expected: FAIL on "one adjective" and "adjectives after a duplicate index" (the span remains); the other two pass already.

- [ ] **Step 3: Implement**

In `internal/messaging/hidenames_tagged.go` add, next to `dupIndexSuffix`:

```go
// adjectiveSpan matches the adjective list FormattedName.String prints right
// after an identity tag: a space, then a black-bold span holding a
// parenthesised list such as "(dead)" or "(♥friend|hidden)". CompileAdjectiveSwaps
// colour-patterns each adjective RUNE BY RUNE, so in production the body is
// nested single-rune tags, which the pattern admits. It goes with the name it
// describes; "something (dead)" would tell a blind reader what they could not
// see.
var adjectiveSpan = regexp.MustCompile(`^ <ansi fg="black-bold">\((?:[^<]|<ansi fg="[^"]*">[^<]*</ansi>)*\)</ansi>`)
```

In `hideTaggedName`, change `last = m[1]` to:

```go
		last = m[1]
		if adj := adjectiveSpan.FindStringIndex(text[last:]); adj != nil {
			last += adj[1]
		}
```

- [ ] **Step 4: Run it green, then the package**

Run: `go test ./internal/messaging/ -run TestHideNames -v` then `go test ./internal/messaging/`
Expected: PASS, PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/messaging/hidenames_tagged.go internal/messaging/hidenames_test.go
git commit -m "fix(messaging): a hidden name takes its adjectives with it" -m "FormattedName prints adjectives in a black-bold span after the identity tag. HideNames now removes that span along with the tag, so an unsighted reader reads something, not something (dead)." -m "Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 3: Recoil lines through the seam

**Files:**
- Modify: `internal/hooks/NewRound_DoCombat_unified.go` (`emitReturnDamageText`, lines 432-484)
- Create: `internal/hooks/return_damage_trio_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/hooks/return_damage_trio_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Slice F's playtest read "You recoil from striking Stone Beetle Queen (dead)!"
// in an unlit cave while every other line said "something". The three recoil
// lines were direct SendText calls that never reached the seam.

func TestRecoil_AttackerInTheDarkIsNotToldTheDefendersName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	drainPlain(1)

	atk := actions.NewUserActorInRoom(users.GetByUserId(1), room)
	def := actions.NewMobActorInRoom(mobs.GetInstance(100), room)
	emitReturnDamageText(atk, def, 5)

	attacker := drainPlain(1)
	assert.Equal(t, 1, countContaining(attacker, "You recoil from striking something!"))
	assert.Equal(t, 0, countContaining(attacker, "Skeleton"))
}

func TestRecoil_DefenderInTheDarkIsNotToldTheAttackersName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	drainPlain(1)

	atk := actions.NewMobActorInRoom(mobs.GetInstance(100), room)
	def := actions.NewUserActorInRoom(users.GetByUserId(1), room)
	emitReturnDamageText(atk, def, 5)

	defender := drainPlain(1)
	assert.Equal(t, 1, countContaining(defender, "Something recoils from striking you!"))
	assert.Equal(t, 0, countContaining(defender, "Skeleton"))
}

func TestRecoil_RoomLineIsVisualAndHidesNames(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	drainPlain(2)

	atk := actions.NewUserActorInRoom(users.GetByUserId(1), room)
	def := actions.NewMobActorInRoom(mobs.GetInstance(100), room)
	emitReturnDamageText(atk, def, 5)
	assert.Equal(t, 0, countContaining(drainPlain(2), "recoils"),
		"an observer who cannot see must not be told about the recoil at all")

	require.True(t, users.GetByUserId(2).Character.Buffs.AddBuff(heatEyesBuffId, true))
	emitReturnDamageText(atk, def, 5)
	observer := drainPlain(2)
	assert.Equal(t, 1, countContaining(observer, "A figure recoils from striking a figure!"))
	assert.Equal(t, 0, countContaining(observer, "Aliceia"))
	assert.Equal(t, 0, countContaining(observer, "Skeleton"))
}

func TestRecoil_LitRoomNamesBothAndSparesTheParticipantsTheRoomLine(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	room := rooms.LoadRoom(1)
	drainPlain(1)
	drainPlain(2)

	atk := actions.NewUserActorInRoom(users.GetByUserId(1), room)
	def := actions.NewMobActorInRoom(mobs.GetInstance(100), room)
	emitReturnDamageText(atk, def, 5)

	attacker := drainPlain(1)
	assert.Equal(t, 1, countContaining(attacker, "You recoil from striking Skeleton!"))
	assert.Equal(t, 0, countContaining(attacker, "Aliceia recoils"), "the room line must not reach the attacker")
	assert.Equal(t, 1, countContaining(drainPlain(2), "Aliceia recoils from striking Skeleton!"))
}
```

- [ ] **Step 2: Run them red**

Run: `go test ./internal/hooks/ -run TestRecoil -v`
Expected: the two dark participant tests FAIL (the name is present). The room test's second half FAILS ("A figure" absent; the raw name reaches infrared). The lit test may already pass.

- [ ] **Step 3: Rewrite `emitReturnDamageText`**

Replace the body from `excludes := playerExcludeIds(atk, def)` to the end of the function with:

```go
	// Through the seam, as the crit lines are (sendCritEffectTrio): the
	// attacker recoils, so they are the Actor and the defender the Actee. A
	// reader who cannot see the other party reads "something", the room line
	// is visual, and the participants are excluded from it.
	var atkRecipient, defRecipient messaging.Recipient
	if atk.IsPlayer() {
		atkRecipient = atk
	}
	if def.IsPlayer() {
		defRecipient = def
	}
	messaging.SendTrio(messaging.Trio{
		Actor: messaging.Say(messaging.CategoryHitMelee, fmt.Sprintf(
			`<ansi fg="red">You recoil from striking %s! (%s)</ansi>`, defToken, dmgDesc)),
		Actee: messaging.Say(messaging.CategoryHitMelee, fmt.Sprintf(
			`<ansi fg="red">%s recoils from striking you! (%s)</ansi>`, atkToken, dmgDesc)),
		Observer: messaging.Say(messaging.CategoryHitMelee, fmt.Sprintf(
			`<ansi fg="red">%s recoils from striking %s! (%s)</ansi>`, atkToken, defToken, dmgDesc)),
	}, messaging.Audience{
		Actor:     atkRecipient,
		ActorId:   atk.GetUserId(),
		ActorName: atkChar.Name,
		Actee:     defRecipient,
		ActeeId:   def.GetUserId(),
		ActeeName: defChar.Name,
		Room:      atkRoom,
	})
}
```

Update the doc comment above the function: it now says the lines go through `messaging.SendTrio` and why. Keep the token construction above unchanged. Check `grep -rn "playerExcludeIds(\|sendVisualRoomText(" internal/hooks --include=*.go | grep -v _test`; leave both helpers if anything else still calls them, delete either that lost its last caller.

- [ ] **Step 4: Run green, then the root guards**

Run: `go test ./internal/hooks/ -run "TestRecoil|TestSweep|TestCritDispatch" -v`, then `go test ./internal/hooks/`, then `go test . -run "TestNarrationSitesMatchViewpointAudit|TestM2RoutingIsFrozen|TestEveryTrioLiteralNamesAllThreeRoles|TestM2LiteralsAreFrozen|TestEveryTextSurfaceIsRegistered"`.
Expected: all PASS. If a root guard fails, read its message: it says exactly which row or literal it wants. Add the row it names in the same commit; never loosen the guard.

- [ ] **Step 5: Commit**

```bash
git add internal/hooks/NewRound_DoCombat_unified.go internal/hooks/return_damage_trio_test.go
git commit -m "fix(combat): recoil lines stop naming combatants in the dark" -m "The three return-damage lines were direct SendText calls, so a blind attacker read the defender's name and adjectives. They go through SendTrio now, like the crit lines." -m "Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 4: One retarget notice, hidden by the reader's sight

**Files:**
- Modify: `internal/hooks/combat_retarget.go` (add `retargetNotice` at the end of the file)
- Modify: `internal/hooks/NewRound_DoCombat_unified.go` (`emitRetargetMessage`, lines 997-1016)
- Modify: `internal/hooks/NewRound_DoCombat.go:132-141`
- Create: `internal/hooks/combat_retarget_notice_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/hooks/combat_retarget_notice_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// "You turn your attention to X!" printed X regardless of whether the reader
// could see. RetargetOrEnd picks whoever is already attacking you, so the
// notice stands in the dark; only the name is hidden.

func TestRetargetNotice_LitRoomNamesTheTarget(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	room := rooms.LoadRoom(1)

	line, ok := retargetNotice(room, 1, state.ActorRef{MobInstanceId: 100})
	require.True(t, ok)
	assert.Equal(t, "You turn your attention to Skeleton!", plainText(line))

	line, ok = retargetNotice(room, 1, state.ActorRef{UserId: 2})
	require.True(t, ok)
	assert.Equal(t, "You turn your attention to Bobrick!", plainText(line))
}

func TestRetargetNotice_DarkRoomHidesTheTarget(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	darken(t, 1)
	room := rooms.LoadRoom(1)

	line, ok := retargetNotice(room, 1, state.ActorRef{MobInstanceId: 100})
	require.True(t, ok)
	assert.Equal(t, "You turn your attention to something!", plainText(line))

	line, ok = retargetNotice(room, 1, state.ActorRef{UserId: 2})
	require.True(t, ok)
	assert.Equal(t, "You turn your attention to something!", plainText(line))
}

func TestRetargetNotice_UnresolvedTargetSaysNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	room := rooms.LoadRoom(1)

	_, ok := retargetNotice(room, 1, state.ActorRef{MobInstanceId: 999})
	assert.False(t, ok)
	_, ok = retargetNotice(room, 1, state.ActorRef{})
	assert.False(t, ok)
}
```

- [ ] **Step 2: Run them red**

Run: `go test ./internal/hooks/ -run TestRetargetNotice -v`
Expected: compile FAIL, `undefined: retargetNotice`.

- [ ] **Step 3: Add the builder**

Append to `internal/hooks/combat_retarget.go` (add `"fmt"`, `"github.com/GoMudEngine/GoMud/internal/messaging"` and `"github.com/GoMudEngine/GoMud/internal/state"` to its imports if absent; `mobs`, `rooms` and `users` are already imported there):

```go
// retargetNotice builds the "You turn your attention to X!" line for a player
// RetargetOrEnd just gave a new target, with X hidden by that player's sight
// in room: "something" for a reader who cannot see, "a figure" for infrared.
// ok is false when the target no longer resolves, in which case say nothing.
//
// The notice is not suppressed in the dark. RetargetOrEnd picks whoever is
// already attacking the reader, and melee in the dark still swings, so the
// honest line is that their attention turned to something.
func retargetNotice(room *rooms.Room, userId int, target state.ActorRef) (string, bool) {
	var name, line string
	if mob := mobs.GetInstance(target.MobInstanceId); target.MobInstanceId > 0 && mob != nil {
		name = mob.Character.Name
		line = fmt.Sprintf(`You turn your attention to <ansi fg="mobname">%s</ansi>!`, name)
	} else if u := users.GetByUserId(target.UserId); target.UserId > 0 && u != nil {
		name = u.Character.Name
		line = fmt.Sprintf(`You turn your attention to <ansi fg="username">%s</ansi>!`, name)
	} else {
		return "", false
	}
	if room == nil {
		return line, true
	}
	return messaging.HideNames(line, []string{name}, room.ParticipantSight(userId)), true
}
```

- [ ] **Step 4: Use it at both call sites**

`emitRetargetMessage` in `NewRound_DoCombat_unified.go` becomes:

```go
// emitRetargetMessage sends the "You turn your attention to..." message
// to a player attacker who was just retargeted by RetargetOrEnd. The name
// is hidden by the attacker's sight (retargetNotice).
func emitRetargetMessage(atk actions.Actor) {
	if !atk.IsPlayer() {
		return
	}
	atkChar := atk.GetCharacter()
	if !atkChar.IsInCombat() {
		return
	}
	if line, ok := retargetNotice(atk.GetRoom(), atk.GetUserId(), atkChar.CurrentCombatTarget()); ok {
		atk.SendText(messaging.CategorySystem, line)
	}
}
```

In `NewRound_DoCombat.go`, replace the block

```go
				if RetargetOrEnd(user.Character, uRoom, user.UserId, 0) {
					retargeted := user.Character.CurrentCombatTarget()
					if mob := mobs.GetInstance(retargeted.MobInstanceId); mob != nil {
						user.SendText(messaging.CategorySystem, fmt.Sprintf("You turn your attention to <ansi fg=\"mobname\">%s</ansi>!", mob.Character.Name))
					} else if defUser := users.GetByUserId(retargeted.UserId); defUser != nil {
						user.SendText(messaging.CategorySystem, fmt.Sprintf("You turn your attention to <ansi fg=\"username\">%s</ansi>!", defUser.Character.Name))
					}
				}
```

with

```go
				if RetargetOrEnd(user.Character, uRoom, user.UserId, 0) {
					if line, ok := retargetNotice(uRoom, user.UserId, user.Character.CurrentCombatTarget()); ok {
						user.SendText(messaging.CategorySystem, line)
					}
				}
```

Run `go build ./...`; if `fmt`, `mobs` or `users` become unused in `NewRound_DoCombat.go`, drop the import.

- [ ] **Step 5: Run green, then the root guards**

Run: `go test ./internal/hooks/ -run TestRetargetNotice -v`, `go test ./internal/hooks/`, then `go test . -run "TestNarrationSitesMatchViewpointAudit|TestM2RoutingIsFrozen|TestEveryTrioLiteralNamesAllThreeRoles|TestM2LiteralsAreFrozen|TestEveryTextSurfaceIsRegistered"`.
Expected: all PASS. A guard that names a missing row gets that row, in this commit.

- [ ] **Step 6: Commit**

```bash
git add internal/hooks/combat_retarget.go internal/hooks/combat_retarget_notice_test.go internal/hooks/NewRound_DoCombat_unified.go internal/hooks/NewRound_DoCombat.go
git commit -m "fix(combat): the retarget notice hides a target you cannot see" -m "Both copies of 'You turn your attention to X!' were direct SendText calls naming X regardless of sight. One builder now serves both call sites and hides the name by the reader's ParticipantSight." -m "Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 5: Docs, patch notes and the ship gates

**Files:**
- Modify: `internal/characters/context.md`, `internal/messaging/context.md`, `internal/hooks/context.md`
- Modify: `docs/PATCH_NOTES.md`

- [ ] **Step 1: `internal/characters/context.md`**

Find the paragraph describing `GetMobName` / `getFormattedName` / the `aggro` suffix (`grep -n "aggro\|getFormattedName\|GetCharacterName" internal/characters/context.md`). Add one sentence: "The `aggro` suffix requires a real viewer (`viewingUserId > 0`) whose id matches the character's combat target; viewer-0 renders (`GetCharacterName(true)`, room broadcasts) never carry it."

- [ ] **Step 2: `internal/messaging/context.md`**

At the `HideNames` entry (line 84), append: "When the whole identity tag is hidden, one directly following adjective span (`` <ansi fg="black-bold">(...)</ansi>``, as `FormattedName.String` prints it) goes with it, so a caller may pass formatted names through the seam."

- [ ] **Step 3: `internal/hooks/context.md`**

In the "Names in the dark" paragraph (line 121), add the recoil lines (`emitReturnDamageText`) to the list that goes through `messaging.SendTrio`, and add: "The retarget notice (`retargetNotice` in `combat_retarget.go`, used by both `DoCombat` and `emitRetargetMessage`) hides the new target's name by the reader's sight."

- [ ] **Step 4: `docs/PATCH_NOTES.md`**

Insert above the `## 2026-09-11: What you cannot see, you cannot name` entry, keeping the 80-column wrap and no raw numbers:

```markdown
## 2026-09-12: Names are only red when they should be

A name shown in red means that creature is fighting you. For a long while,
almost every name in the room was red whether it was fighting you or not,
and a bystander's name was red to everyone. Now a name turns red only for the
player it is actually attacking. Everyone else sees it in its ordinary colour.

Two combat lines were also still naming people you could not see. If you
struck something in the dark that hurt to hit, you were told exactly what it
was, and when the thing you were fighting fell and another took its place, you
were told its name too. Both now say "something" until you can see.
```

- [ ] **Step 5: Gates**

Run, each standalone:

```bash
gofmt -l internal/
go build ./...
go test ./... 2>&1 | grep -v "^ok" | grep -v "no test files"
golangci-lint run --new-from-rev=master ./...
```

Expected: gofmt lists no file, build clean, no FAIL lines, lint 0 issues.

- [ ] **Step 6: Commit**

```bash
git add internal/characters/context.md internal/messaging/context.md internal/hooks/context.md docs/PATCH_NOTES.md
git commit -m "docs: red names, recoil and retarget in the dark" -m "Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 6: Playtest lane

**Files:**
- Create: `tools/playtest/goals/2026-09-12-slice-b-dark-recoil-and-plain-names.yaml`

Run per the `dogmud-playtesting` skill (ephemeral goals file, `--checkout` of this branch). The lane: veteran in room 3112, no light. `attack queen` in the dark and fight until the queen dies or the budget ends, quoting every line containing "recoil": each must read "something", never "Stone Beetle Queen". Then cast `chrysalis-glow`, `look`, and quote the "Also here" line verbatim with its colour codes if the harness exposes raw output; if it does not, record that colour could not be observed. Extract findings to memory after the run (reports are gitignored).

- [ ] **Step 1: Write the goals file, run it, read `.run/<id>/` for the transcript, fix anything it finds, re-run.**
- [ ] **Step 2: Commit the goals file.**

```bash
git add tools/playtest/goals/2026-09-12-slice-b-dark-recoil-and-plain-names.yaml
git commit -m "test(playtest): slice B lane, recoil in the dark and plain names" -m "Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

## Self-review

- Spec defect 1 -> Task 1. Defect 2 -> Tasks 2 and 3 (the adjective span is what makes 3 clean). Defect 3 -> Task 4. Gates and docs -> Tasks 5 and 6.
- `retargetNotice(room *rooms.Room, userId int, target state.ActorRef) (string, bool)` is spelled the same in Task 4's builder, both call sites and the tests.
- `emitReturnDamageText(atk, def actions.Actor, returnDmg int)` keeps its signature, so its one caller at `:402` is untouched.
