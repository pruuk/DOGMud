# Sight Gates Close-out Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close epic #382: every remaining line, payload and lookup that names an actor in the dark follows the reader's sight, a no-sight observer contests a sneak by ear, and the copy and jail fixes from the lighting playtests ship.

**Architecture:** No new name machinery. Each leak is routed through an existing sight-aware path (`SendSeen`, `SendTextVisual` / `sendVisualRoomText`, `SendTextVisualHidingNames`, `HideNames`, `NameAt` / `corpseNameFor`, `ShopSightRefusal`, `ResolveLook`). Two shared helpers are added where two call sites must not drift: `scanReach` (player scan text and mob scan) and the sneak observer score and notice helpers (the `sneak` command and the sneaking arrival). One balance knob, `SneakHearingMult` (0.75).

**Tech Stack:** Go, YAML content under `_datafiles/world/dogmud`, GMCP module, repo-root guard tests.

**Spec:** `docs/superpowers/specs/2026-10-08-sight-gates-closeout-design.md` (owner approved 2026-10-08; amended the same day with the dry-run findings listed in its "Dry-run amendments" section).

---

## How to run this plan

- **Two PRs, two branches, both from `origin/master`.**
  - **PR 1 (code):** Groups A, B, C, D, in that order, then Task P1 (gate). Branch `feat/sight-gates-closeout`.
  - **PR 2 (content and copy):** Group E, Tasks E1 to E9. Branch `feat/sight-gates-closeout-copy`. Independent of PR 1; may run in parallel in its own worktree.
  - **After both merge:** Task P2, the closing adversarial dark-room playtest.
- **Dry run.** Every section was dry-run on `f50400bf7` (2026-10-08): each test was written, seen to FAIL on master with the output quoted under its step, then seen to PASS. Each section was dry-run on its own, NOT stacked on the others. So:
  - 🪤 **Locate every edit by its quoted `old_string`, never by line number.** Line numbers are master's; earlier tasks shift them. Files edited by more than one PR 1 task: `internal/usercommands/attack.go` (A2, A3), `internal/usercommands/look.go` (C2/C3, D5), `internal/actions/sneak.go` and `internal/actions/move.go` (B2, B3), the `messaging_surface_guard_test.go` registry (A5 in PR 1, E6 in PR 2, both name the "Your concentration shatters" key; whichever merges second rebases that key), the repo-root guard tests (A5, B2, D5).
  - If a quoted "expected FAIL" output differs only in a line number, that is the stacking, not a defect.
- **Gate for every task:** the package's tests AND the repo root (`go test . -count=1`); the root holds the guard tests (`messaging_surface_guard_test.go`, `sight_penalty_guard_test.go`, the bauble and transient-holder guards). `gofmt -l` on touched files before each commit.
- **Commits:** named paths only, never `git add -A` or `git add .`. Every message ends with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- **`_datafiles/config.yaml`** carries skip-worktree. Task B1 says how to commit it; follow it exactly.
- **PR bodies** use `Refs #N`, never a closing keyword (a closing keyword in prose auto-closes at merge). Issues close by hand after the P2 playtest confirms.
- **No em or en dashes** in prose or player text; player text within 80 columns.
- **Never kill a server by name or port.** Kill only a PID you started (see `dogmud-playtesting`).

## Task index

| Task | Issue | PR |
|---|---|---|
| A1 Anonymize keeps a possessive inside the name tag | #246 | 1 |
| A2 attack hides the target's name at the reader's sight | #214 | 1 |
| A3 attack says why it found no one | #254 | 1 |
| A4 the dark-room fight fallback says "close by" | #216 | 1 |
| A5 spell-channel lines follow sight; disruptions are heard | #242 | 1 |
| B1 `SneakHearingMult` balance knob | #333 | 1 |
| B2 no-sight observers roll by ear; observer notice follows sight | #333, #215 | 1 |
| B3 the sneaker learns its spotter only as far as it can see | #215 | 1 |
| B4 a hidden actor does not emote | #274 | 1 |
| B9 a mob's sayto line follows sight | found in dry run | 1 |
| C1 mob scan follows sight through one reach rule | #251 | 1 |
| C2 corpse lookups take the viewer (signature only) | #435 | 1 |
| C3 below clear sight only the word "corpse" matches | #435 | 1 |
| C4 the decay line follows each reader's sight | #276 | 1 |
| C5 `loot pass` names the member at the looter's sight | #435 | 1 |
| D1 GMCP Room.Info follows the viewer's sight | #252 | 1 |
| D2 the fog map adds only rooms the player can see | #252 | 1 |
| D3 GMCP say follows each listener's sight and deafen | #252 | 1 |
| D4 appraise and offer refuse in the dark | #272 | 1 |
| D5 look at your own gear by touch in the dark | #218 | 1 |
| D6 context.md for group D | | 1 |
| P1 PR 1 gate | | 1 |
| E1 to E9 content and copy | #298, #260, #219, #409 | 2 |
| P2 closing adversarial dark-room playtest | #382 | after both |

---

## Group A: combat and spell (#246, #214, #254, #216, #242)

Dry-run on origin/master `f50400bf7`, 2026-10-08: every test below was written
first, seen to FAIL on unfixed code with the output quoted, then seen to PASS.
After all five tasks: `go build ./...` clean, `go test ./internal/... ./modules/...`
all ok, root `go test .` ok (after the registry edit in A5).

**Gate for every task (dogmud-writing-tests):** run the package tests AND the
repo root, `go test . -count=1`. The root holds `messaging_surface_guard_test.go`,
whose `TestNarrationSitesMatchViewpointAudit` keys on narration literals per file.
Run `gofmt -l` on touched files before each commit.

🪤 **gofmt rewrites `''` in a Go doc comment to a curly quote.** Do not write the
YAML form `{actor}''s` inside any `//` comment; describe it in words.

🪤 Line numbers below are master's. Tasks A2 and A3 both edit `attack.go`, so
locate every edit by its quoted `old_string`, not by number.

Spec deltas found in the dry run (all applied below):
- #214 has two more raw-name lines than the spec lists: the non-combatant /
  attack-immune refusal (`attack.go:179`, `You can't attack ... %s.`) and the PvP
  target's own "prepares to fight you!" (`attack.go:345`), judged at the TARGET's sight.
- #242: the player prone and grapple breaks (`helpers.go:511`, `:521`) used
  `sendVisualRoomText` (sight-only). For mob and player casters to read alike, all
  three player breaks move to the shared sound-backed sender, not only `:1079`.
  `sendVisualElseAudible` gains an `excludeUserIds ...int` so the caster is skipped.
- #242: the focus-shift line names two people (mob and new target), so it uses
  `room.SendTextVisualHidingNames` with both names, matching `target.go:209`.
- Moving the player break room line into a helper drops two entries from the
  viewpoint registry (the walk no longer pairs the actor line with a room line in
  the same function). The guard says to remove stale entries after checking; both
  entries are marked "Not part of the audit", so no audit doc changes.

---

### Task A1: Anonymize keeps a possessive written inside the name tag (#246)

**Files:**
- Modify: `internal/messaging/anonymize.go:21-23` (pattern), `:49-56` (loop)
- Create: `internal/messaging/anonymize_possessive_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/messaging/anonymize_possessive_test.go`:

```go
package messaging

import "testing"

// #246: the combat templates write the possessive INSIDE the name tag, so a
// rendered line reads <ansi fg="username">Calabe's</ansi> (about 275 lines
// across _datafiles/world/*/combat-messages). Anonymize replaced the whole tag body,
// so a shapes reader read "A figure Iron Longsword delivers..." with the 's
// gone. The possessive belongs to the sentence, not the name: keep it.
func TestAnonymizeKeepsAPossessiveInsideTheTag(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			"username at the start",
			`<ansi fg="username">Calabe's</ansi> <ansi fg="item">Iron Longsword</ansi> lands`,
			`<ansi fg="combat-anon">A figure</ansi>'s <ansi fg="item">Iron Longsword</ansi> lands`,
		},
		{
			"mobname mid sentence",
			`You dodge <ansi fg="mobname">Thornwall Thug's</ansi> swing`,
			`You dodge <ansi fg="combat-anon">a figure</ansi>'s swing`,
		},
		{
			"curly apostrophe",
			`<ansi fg="mobname-dup2">Thug #2’s</ansi> blade`,
			`<ansi fg="combat-anon">A figure</ansi>’s blade`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Anonymize(tc.in); got != tc.want {
				t.Fatalf("possessive lost or mangled:\n got %q\nwant %q", got, tc.want)
			}
		})
	}
}

// A name that merely ends in s keeps no stray apostrophe, and a possessive
// written OUTSIDE the tag (the other template shape) is untouched.
func TestAnonymizeNonPossessiveShapesUnchanged(t *testing.T) {
	tests := []struct{ in, want string }{
		{`<ansi fg="mobname">Silas</ansi> snarls`, `<ansi fg="combat-anon">A figure</ansi> snarls`},
		{`<ansi fg="username">Calabe</ansi>'s blade`, `<ansi fg="combat-anon">A figure</ansi>'s blade`},
	}
	for _, tc := range tests {
		if got := Anonymize(tc.in); got != tc.want {
			t.Fatalf("got %q want %q", got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/messaging/ -run "TestAnonymizeKeepsAPossessive|TestAnonymizeNonPossessive" -count=1`

Expected (observed in the dry run): FAIL, the three possessive cases lose the `'s`:
```
--- FAIL: TestAnonymizeKeepsAPossessiveInsideTheTag (0.00s)
     got "<ansi fg=\"combat-anon\">A figure</ansi> <ansi fg=\"item\">Iron Longsword</ansi> lands"
    want "<ansi fg=\"combat-anon\">A figure</ansi>'s <ansi fg=\"item\">Iron Longsword</ansi> lands"
```
`TestAnonymizeNonPossessiveShapesUnchanged` passes on master; it guards the fix.

- [ ] **Step 3: Implement**

In `internal/messaging/anonymize.go`, replace the pattern:

```go
var nameTagPattern = regexp.MustCompile(
	`<ansi fg="((?:username|mobname)(?:-[A-Za-z0-9_-]+)?|petname)">[^<]+</ansi>(?:` + adjectiveSpanBody + `)?`,
)
```

with (the comment block goes at the end of the existing doc comment above it):

```go
//
// The `poss` group (#246) captures a possessive written INSIDE the tag, the
// shape about 275 combat template lines use (the YAML puts the doubled
// quote of {actor}'s before the closing tag). The body
// is lazy so the optional group gets the 's rather than the body swallowing
// it; Anonymize re-emits it after the figure word.
var nameTagPattern = regexp.MustCompile(
	`<ansi fg="((?:username|mobname)(?:-[A-Za-z0-9_-]+)?|petname)">[^<]+?(?P<poss>'s|’s)?</ansi>(?:` + adjectiveSpanBody + `)?`,
)

var nameTagPossIdx = nameTagPattern.SubexpIndex("poss")
```

and the loop in `Anonymize`:

```go
	for _, loc := range nameTagPattern.FindAllStringIndex(text, -1) {
		out = append(out, text[last:loc[0]]...)
		word := "a figure"
		if atSentenceStart(text, loc[0]) {
			word = "A figure"
		}
		out = append(out, `<ansi fg="combat-anon">`+word+`</ansi>`...)
		last = loc[1]
	}
```

with:

```go
	for _, loc := range nameTagPattern.FindAllStringSubmatchIndex(text, -1) {
		out = append(out, text[last:loc[0]]...)
		word := "a figure"
		if atSentenceStart(text, loc[0]) {
			word = "A figure"
		}
		out = append(out, `<ansi fg="combat-anon">`+word+`</ansi>`...)
		if ps := loc[2*nameTagPossIdx]; ps >= 0 {
			out = append(out, text[ps:loc[2*nameTagPossIdx+1]]...)
		}
		last = loc[1]
	}
```

`nameTagPattern` has no other users (`grep -rn nameTagPattern internal/` shows only
anonymize.go).

- [ ] **Step 4: Run tests and see them pass**

Run: `go test ./internal/messaging/ -count=1` then `go test . -count=1`
Expected: `ok  github.com/GoMudEngine/GoMud/internal/messaging` and `ok  github.com/GoMudEngine/GoMud`.

- [ ] **Step 5: Null probe**

Temporarily delete the three `if ps := ...` lines; rerun Step 2's command; confirm
`TestAnonymizeKeepsAPossessiveInsideTheTag` fails naming the lost `'s`. Restore.

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/messaging/
git add internal/messaging/anonymize.go internal/messaging/anonymize_possessive_test.go
git commit -m "fix(messaging): Anonymize keeps a possessive inside the name tag (#246)

A shapes reader read \"A figure Iron Longsword\" because the combat
templates put the 's inside the name tag and Anonymize replaced the
whole body. The pattern now captures the possessive and re-emits it.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task A2: attack hides the target's name at the reader's sight (#214)

**Files:**
- Modify: `internal/usercommands/attack.go:172-181` (companion and can't-attack refusals), `:250-253` (mob engagement line), `:338-349` (PvP lines)
- Create: `internal/usercommands/attack_sight_names_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/usercommands/attack_sight_names_test.go`:

```go
package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #214: attack named its target to an attacker who could not see it. The
// room's "prepares to fight" line was already sight-gated; the attacker's own
// lines were raw SendText with the mob's display name, so in pitch dark the
// attacker read "You prepare to enter into mortal combat with Skeleton." The
// same raw name rode the companion refusal and the can't-attack refusal, and
// the PvP lines named each side to the other at any sight.

// darkenTestRoom1 turns the seeded room 1 (city, lamp 60) pitch dark for a
// normal-sighted reader: the seeded cave biome has no sky light, and with no
// lamp nothing else lights it.
func darkenTestRoom1(t *testing.T) {
	t.Helper()
	_, room := getTestUserAndRoom(t)
	room.Biome = "cave"
	room.Lamp = nil
	require.Equal(t, 0, room.LightLevel(), "fixture room must be pitch dark")
}

func TestAttack_PitchDark_DoesNotNameTheMobToTheAttacker(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	darkenTestRoom1(t)
	user, room := getTestUserAndRoom(t)
	user.Character.EndAggro()
	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Attack("skeleton", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	require.Contains(t, out, "mortal combat", "the engagement line must still be sent: %q", out)
	require.NotContains(t, strings.ToLower(out), "skeleton",
		"an attacker who sees nothing must not learn the target's name: %q", out)
}

func TestAttack_PitchDark_CompanionRefusalDoesNotNameTheMob(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	darkenTestRoom1(t)
	user, room := getTestUserAndRoom(t)
	user.Character.EndAggro()

	// Charm the skeleton to user 2, so user 1's attack is refused as
	// someone else's companion.
	m := mobs.GetInstance(100)
	require.NotNil(t, m)
	m.Character.Charmed = nil
	other := users.GetByUserId(2)
	require.NotNil(t, other)
	m.Character.Charm(other.UserId, -1, "")
	require.Equal(t, mobs.HarmBlockedCompanion, mobs.CheckPlayerHarm(m), "fixture: the skeleton must be a companion")
	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Attack("skeleton", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	require.Contains(t, out, "companion", "the refusal must still be sent: %q", out)
	require.NotContains(t, strings.ToLower(out), "skeleton",
		"an attacker who sees nothing must not learn the companion's name: %q", out)
}

func TestAttack_Lit_StillNamesTheMob(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)
	user.Character.EndAggro()
	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Attack("skeleton", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	require.Contains(t, out, "Skeleton", "a sighted attacker reads the name: %q", out)
}
```

- [ ] **Step 2: Run them and see the two dark tests fail**

Run: `go test ./internal/usercommands/ -run "TestAttack_PitchDark|TestAttack_Lit_StillNames" -count=1`

Expected (observed):
```
--- FAIL: TestAttack_PitchDark_DoesNotNameTheMobToTheAttacker
    ... "You prepare to enter into mortal combat with <ansi fg=\"mobname\">Skeleton</ansi>." should not contain "skeleton"
--- FAIL: TestAttack_PitchDark_CompanionRefusalDoesNotNameTheMob
    ... "<ansi fg=\"mobname\">Skeleton</ansi> <ansi fg=\"black-bold\">(charmed)</ansi> is someone's companion!" should not contain "skeleton"
```
`TestAttack_Lit_StillNamesTheMob` passes (the guard against over-hiding).

- [ ] **Step 3: Implement**

In `internal/usercommands/attack.go`, replace:

```go
			switch mobs.CheckPlayerHarm(m) {
			case mobs.HarmBlockedCompanion:
				user.SendText(messaging.CategorySystem, fmt.Sprintf(`%s is someone's companion!`, mName))
				return true, nil
			case mobs.HarmBlockedNonCombatant, mobs.HarmBlockedAttackImmune:
				user.SendText(messaging.CategorySystem, fmt.Sprintf(`You can't attack <ansi fg="mobname">%s</ansi>.`, m.Character.Name))
```

with:

```go
			// #214: the attacker's own lines ride raw SendText, which
			// bypasses the sight gate, so each hides the target's name at
			// the attacker's sight here. Wrap the Sprintf in place: the
			// viewpoint audit keys on the literal (see target.go).
			attackerSight := messaging.ParticipantSight(user.Character, room)

			switch mobs.CheckPlayerHarm(m) {
			case mobs.HarmBlockedCompanion:
				user.SendText(messaging.CategorySystem, messaging.HideNames(
					fmt.Sprintf(`%s is someone's companion!`, mName),
					[]string{m.Character.Name}, attackerSight))
				return true, nil
			case mobs.HarmBlockedNonCombatant, mobs.HarmBlockedAttackImmune:
				user.SendText(messaging.CategorySystem, messaging.HideNames(
					fmt.Sprintf(`You can't attack <ansi fg="mobname">%s</ansi>.`, m.Character.Name),
					[]string{m.Character.Name}, attackerSight))
```

Replace:

```go
				user.SendText(messaging.CategoryHitMelee,
					fmt.Sprintf(`You prepare to enter into mortal combat with %s.`, mName),
				)
```

with:

```go
				user.SendText(messaging.CategoryHitMelee, messaging.HideNames(
					fmt.Sprintf(`You prepare to enter into mortal combat with %s.`, mName),
					[]string{m.Character.Name}, attackerSight))
```

Replace (PvP branch):

```go
			user.SendText(messaging.CategoryHitMelee,
				fmt.Sprintf(`You prepare to enter into mortal combat with <ansi fg="username">%s</ansi>.`, p.Character.Name),
			)

			sendMeleeAmbushDenial(user, pvpAmbushDenied)

			if !isSneaking {

				p.SendText(messaging.CategoryHitMelee,
					fmt.Sprintf(`<ansi fg="username">%s</ansi> prepares to fight you!`, user.Character.Name),
				)
```

with:

```go
			// #214: each side's own line hides the other's name at the
			// READER's sight, as target.go's shift-focus lines do.
			user.SendText(messaging.CategoryHitMelee, messaging.HideNames(
				fmt.Sprintf(`You prepare to enter into mortal combat with <ansi fg="username">%s</ansi>.`, p.Character.Name),
				[]string{p.Character.Name},
				messaging.ParticipantSight(user.Character, room)))

			sendMeleeAmbushDenial(user, pvpAmbushDenied)

			if !isSneaking {

				p.SendText(messaging.CategoryHitMelee, messaging.HideNames(
					fmt.Sprintf(`<ansi fg="username">%s</ansi> prepares to fight you!`, user.Character.Name),
					[]string{user.Character.Name},
					messaging.ParticipantSight(p.Character, room)))
```

HideNames removes the `(charmed)` adjective span with the tag; observed dark
output: `Something is someone's companion!` and
`You prepare to enter into mortal combat with something.`

- [ ] **Step 4: Run tests and see them pass**

Run: `go test ./internal/usercommands/ -count=1` then `go test . -count=1`
Expected: `ok  .../internal/usercommands` and `ok  github.com/GoMudEngine/GoMud`
(the viewpoint audit still matches: the literals are unchanged, only wrapped).

- [ ] **Step 5: Null probe**

Change `attackerSight` in the engagement line back to `messaging.SightFull`;
rerun Step 2; confirm `TestAttack_PitchDark_DoesNotNameTheMobToTheAttacker` fails
on "skeleton". Restore.

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/usercommands/
git add internal/usercommands/attack.go internal/usercommands/attack_sight_names_test.go
git commit -m "fix(attack): hide the target's name at the reader's sight (#214)

The attacker's engagement line and the companion and can't-attack
refusals named the mob in pitch dark; the PvP lines named each side
to the other at any sight. Each now goes through HideNames at its
reader's sight, as target already does.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task A3: attack says why it found no one (#254)

**Files:**
- Modify: `internal/usercommands/attack.go:20` (new consts above `func Attack`), `:100-103` (not-found block), `:166`, `:289` (stale-id lines)
- Modify: `internal/usercommands/attack_stale_target_test.go:103`
- Create: `internal/usercommands/attack_not_found_test.go`
- Modify: `docs/superpowers/audits/messaging-m6-content-ledger.md` row 17 (status cell)

- [ ] **Step 1: Write the failing tests**

Create `internal/usercommands/attack_not_found_test.go`:

```go
package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/stretchr/testify/require"
)

// #254: "You attack the darkness!" covered two different failures. A bare
// `attack` with nobody to pick up reads one line; a typed name that matches
// nothing reads the owner's line (2026-08-15) and never echoes the name, so a
// hider's presence and a dark room give nothing away.
func TestAttack_NoArgumentAndNoFoe_SaysNothingToAttack(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)
	user.Character.EndAggro()
	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Attack("", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	require.Contains(t, out, "There is nothing here to attack.")
	require.NotContains(t, out, "darkness")
}

func TestAttack_UnmatchedName_SaysNothingByThatName(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)
	user.Character.EndAggro()
	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Attack("zzyzx", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	require.Contains(t, out, "Nothing by that name is in this room.")
	require.NotContains(t, out, "zzyzx", "the typed name is never echoed")
	require.NotContains(t, out, "darkness")
}
```

- [ ] **Step 2: Run them and see them fail**

Run: `go test ./internal/usercommands/ -run "TestAttack_NoArgument|TestAttack_UnmatchedName" -count=1`

Expected (observed):
```
--- FAIL: TestAttack_NoArgumentAndNoFoe_SaysNothingToAttack
    Error: "<ansi fg=\"system\">You attack the darkness!</ansi>\n" does not contain "There is nothing here to attack."
--- FAIL: TestAttack_UnmatchedName_SaysNothingByThatName
    Error: "<ansi fg=\"system\">You attack the darkness!</ansi>\n" does not contain "Nothing by that name is in this room."
```

- [ ] **Step 3: Implement**

In `internal/usercommands/attack.go`, directly above `func Attack(`, add:

```go
// attackNothingHereLine answers a bare `attack` with no foe to pick up;
// attackNoSuchNameLine answers a name that resolves to no one (owner,
// 2026-08-15). A resolved id whose record is gone reads the second line too.
const (
	attackNothingHereLine = `There is nothing here to attack.`
	attackNoSuchNameLine  = `Nothing by that name is in this room.`
)
```

Replace:

```go
	if attackMobInstanceId == 0 && attackPlayerId == 0 {
		user.SendText(messaging.CategorySystem, "You attack the darkness!")
		return true, nil
	}
```

with:

```go
	// #254: two failures, two lines. Neither echoes the typed name, so an
	// unperceived hider and a name that matches nothing read the same.
	if attackMobInstanceId == 0 && attackPlayerId == 0 {
		if rest == `` {
			user.SendText(messaging.CategorySystem, attackNothingHereLine)
		} else {
			user.SendText(messaging.CategorySystem, attackNoSuchNameLine)
		}
		return true, nil
	}
```

Replace both stale-id lines (one under `if m == nil {`, one under `if p == nil {`):

```go
			user.SendText(messaging.CategorySystem, `You don't see them here.`)
```

with:

```go
			user.SendText(messaging.CategorySystem, attackNoSuchNameLine)
```

In `internal/usercommands/attack_stale_target_test.go`, replace:

```go
	assert.True(t, strings.Contains(strings.Join(msgs, "\n"), "don't see them"),
```

with:

```go
	assert.True(t, strings.Contains(strings.Join(msgs, "\n"), attackNoSuchNameLine),
```

`You don't see them here.` stays in `cast_admission.go`, `melee_target.go` and
`consider.go`: converging the wider not-found family is the follow-up the spec
puts out of scope.

In `docs/superpowers/audits/messaging-m6-content-ledger.md`, row 17 (the
"You attack the darkness!" row), change its final status cell from `open` to
`fixed (#254, sight-gates close-out)`. Use the Edit tool anchored on the row's
unique text `Filed, not fixed.` through the end of the row.

- [ ] **Step 4: Run tests and see them pass**

Run: `go test ./internal/usercommands/ -count=1` then `go test . -count=1`
Expected: both `ok`.

- [ ] **Step 5: Null probe**

Swap the two `SendText` lines inside the `if rest == ""` / `else` block; rerun Step 2; both tests fail
naming the wrong line. Restore.

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/usercommands/
git add internal/usercommands/attack.go internal/usercommands/attack_not_found_test.go internal/usercommands/attack_stale_target_test.go docs/superpowers/audits/messaging-m6-content-ledger.md
git commit -m "fix(attack): say why attack found no one (#254)

\"You attack the darkness!\" covered a bare attack with no foe and a
name that matched nothing. They now read \"There is nothing here to
attack.\" and the owner's \"Nothing by that name is in this room.\";
the typed name is never echoed. attack's stale-id lines join the
second. M6 ledger row 17 marked fixed.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task A4: the dark-room fight fallback says "close by" (#216)

**Files:**
- Modify: `internal/hooks/NewRound_DoCombat_helpers.go:435`
- Modify: `internal/hooks/dark_room_fallback_sight_test.go:47-48`
- Re-record: `internal/hooks/testdata/darkness_narration.golden` (54 lines)

- [ ] **Step 1: Write the failing test**

In `internal/hooks/dark_room_fallback_sight_test.go`, replace:

```go
func TestDarkRoomCombatFallback_FollowsSight(t *testing.T) {
	const needle = "sounds of fighting"
```

with:

```go
// #216: every caller passes the fight's OWN room, so the blind reader is in
// the room with the fight. "nearby" told them it was somewhere else.
func TestDarkRoomCombatFallback_SaysCloseByNotNearby(t *testing.T) {
	room := seedFallbackRoom(t, 0, nightEyesConditionId)
	sendDarkRoomCombatFallback(room)
	got := drainPlain(1)
	require.Equal(t, 1, countContaining(got, "You hear fighting close by."), "%v", got)
	require.Zero(t, countContaining(got, "nearby"), "%v", got)
}

func TestDarkRoomCombatFallback_FollowsSight(t *testing.T) {
	const needle = "You hear fighting"
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/hooks/ -run TestDarkRoomCombatFallback -count=1`
Expected (observed): FAIL in `TestDarkRoomCombatFallback_SaysCloseByNotNearby`
(`Not equal`, line 53) and in both `FollowsSight` subtests (the new needle).

- [ ] **Step 3: Implement**

In `sendDarkRoomCombatFallback`, replace:

```go
			u.SendText(messaging.CategoryDefault, `<ansi fg="yellow">You hear the sounds of fighting nearby.</ansi>`)
```

with:

```go
			// #216: every caller passes the fight's own room, so the
			// reader is IN the fight's room; "nearby" said otherwise.
			u.SendText(messaging.CategoryDefault, `<ansi fg="yellow">You hear fighting close by.</ansi>`)
```

- [ ] **Step 4: Re-record the golden and run**

Run: `go test ./internal/hooks/ -run TestDarknessNarrationGolden -update-darkness -count=1`
Expected: `ok`. Then `git diff --stat -- internal/hooks/testdata/` shows
`darkness_narration.golden | 108 +++---` (54 lines, every one a `spectator =>`
line changing "the sounds of fighting nearby" to "fighting close by"; confirm
with `git diff -- internal/hooks/testdata/ | grep "^[-+]" | grep -v "close by\|nearby"`
printing only the two file header lines).

Run: `go test ./internal/hooks/ -count=1` then `go test . -count=1`. Expected: both `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/hooks/NewRound_DoCombat_helpers.go internal/hooks/dark_room_fallback_sight_test.go internal/hooks/testdata/darkness_narration.golden
git commit -m "fix(combat): a blind reader in the fight's room hears it close by (#216)

The dark-room fallback only ever goes to the fight's own room, so
\"nearby\" told the reader the fight was elsewhere. Golden re-recorded.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task A5: spell-channel lines follow sight; disruptions are heard (#242)

**Files:**
- Modify: `internal/hooks/NewRound_DoCombat_helpers.go`: `sendVisualElseAudible` (:446) gains `excludeUserIds`; new senders after it; call sites in `handlePlayerFoldCasting` (:507-522), `handleMobFoldCasting` (:728-751, :829-831), `handlePlayerConcentrationBreak` (:1078-1081), `handleMobTargetSwitch` (:1219-1222)
- Modify: `messaging_surface_guard_test.go:1279-1280` (remove two stale registry entries)
- Create: `internal/hooks/spell_channel_sight_test.go`

This task extracts the six lines into named senders FIRST with their old
behaviour, so the new tests fail on behaviour rather than on a missing symbol,
then routes the senders.

- [ ] **Step 1: Extract the senders with the OLD behaviour (pure refactor)**

In `internal/hooks/NewRound_DoCombat_helpers.go`, directly above
`// castingTargetChar returns the first target character from a CastingData, or nil.`, add:

```go
// The sound lines a reader who sees nothing gets for a spell-channel
// disruption (#242, owner ruling R4). Mob and player casters share them.
const (
	spellChantBreaksOffSound = `Someone's chant breaks off.`
	spellSputtersOutSound    = `A half-formed spell sputters out.`
)

// sendMobConcentrationBroke narrates a mob caster's broken concentration.
func sendMobConcentrationBroke(mob *mobs.Mob, room *rooms.Room) {
	room.SendText(messaging.CategorySpellDisruption, fmt.Sprintf(
		`%s's concentration breaks.`, mobDisplayName(mob, room, 0)))
}

// sendMobSpellFailed narrates a mob's spell that fizzles (target gone) or
// falters (not enough conviction); verb is "fizzles" or "falters".
func sendMobSpellFailed(mob *mobs.Mob, room *rooms.Room, verb string) {
	room.SendText(messaging.CategorySpellDisruption, fmt.Sprintf(
		`%s's spell %s.`, mobDisplayName(mob, room, 0), verb))
}

// sendMobWeaving narrates a mob still holding its fold.
func sendMobWeaving(mob *mobs.Mob, room *rooms.Room) {
	room.SendText(messaging.CategorySpellFold, fmt.Sprintf(
		`%s weaves magic with focused intent.`, mobDisplayName(mob, room, 0)))
}

// sendPlayerConcentrationBroke narrates a player caster's broken
// concentration to the rest of the room; the caster reads its own line.
func sendPlayerConcentrationBroke(caster *users.UserRecord, room *rooms.Room) {
	if room == nil {
		return
	}
	room.SendText(messaging.CategorySpellDisruption, fmt.Sprintf(
		`<ansi fg="username">%s</ansi>'s concentration breaks.`, caster.Character.Name), caster.UserId)
}

// sendMobShiftsFocus narrates a mob switching its attack to a new player.
func sendMobShiftsFocus(mob *mobs.Mob, room *rooms.Room, newTarget *users.UserRecord) {
	room.SendText(messaging.CategoryMobEmote,
		fmt.Sprintf("%s shifts focus to <ansi fg=\"username\">%s</ansi>!", mobDisplayName(mob, room, 0), newTarget.Character.Name),
	)
}
```

Then point every call site at them. In `handlePlayerFoldCasting`, in BOTH the
`case result.ProneBroke:` and `case result.GrappleBroke:` arms, replace:

```go
		room := rooms.LoadRoom(user.Character.RoomId)
		if room != nil {
			sendVisualRoomText(room, messaging.CategorySpellDisruption, fmt.Sprintf(
				`<ansi fg="username">%s</ansi>'s concentration breaks.`, user.Character.Name), user.UserId)
		}
```

with:

```go
		sendPlayerConcentrationBroke(user, rooms.LoadRoom(user.Character.RoomId))
```

(The arms differ in their preceding `user.SendText` line; make two Edits, each
including that arm's own `user.SendText(...)` line in `old_string` for uniqueness.
Leave the em dash in the grapple line: it is #219, PR 2.)

In `handleMobFoldCasting`, replace each pair:

```go
		mobRoom.SendText(messaging.CategorySpellDisruption, fmt.Sprintf(
			`%s's concentration breaks.`, mobDisplayName(mob, mobRoom, 0)))
```
(twice, ProneBroke and GrappleBroke arms) with `sendMobConcentrationBroke(mob, mobRoom)`;

```go
		mobRoom.SendText(messaging.CategorySpellDisruption, fmt.Sprintf(
			`%s's spell fizzles.`, mobDisplayName(mob, mobRoom, 0)))
```
with `sendMobSpellFailed(mob, mobRoom, "fizzles")`;

```go
		mobRoom.SendText(messaging.CategorySpellDisruption, fmt.Sprintf(
			`%s's spell falters.`, mobDisplayName(mob, mobRoom, 0)))
```
with `sendMobSpellFailed(mob, mobRoom, "falters")`;

```go
		mobRoom.SendText(messaging.CategorySpellFold, fmt.Sprintf(
			`%s weaves magic with focused intent.`, mobDisplayName(mob, mobRoom, 0)))
```
with `sendMobWeaving(mob, mobRoom)`.

In `handlePlayerConcentrationBreak`, replace:

```go
		defRoom.SendText(messaging.CategorySpellDisruption, fmt.Sprintf(
			`<ansi fg="username">%s</ansi>'s concentration breaks.`,
			defUser.Character.Name), defUser.UserId)
```

with `sendPlayerConcentrationBroke(defUser, defRoom)`.

In `handleMobTargetSwitch`, replace:

```go
			mobRoom.SendText(messaging.CategoryMobEmote,
				fmt.Sprintf("%s shifts focus to <ansi fg=\"username\">%s</ansi>!", mobDisplayName(mob, mobRoom, 0), newTarget.Character.Name),
			)
```

with `sendMobShiftsFocus(mob, mobRoom, newTarget)`.

Run: `go build ./... && go test ./internal/hooks/ -count=1`
Expected: `ok` (the player prone and grapple breaks now go through plain SendText
for one step; Step 5 replaces it).

- [ ] **Step 2: Write the failing tests**

Create `internal/hooks/spell_channel_sight_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #242: a mob's spell-channel lines went out on plain Room.SendText, the
// unfiltered audio channel, so a shapes-only or blind observer read the
// caster's name. Owner ruling R4 (2026-10-08): the disruptions (concentration
// breaks, fizzles, falters) have a sound line for a reader who sees nothing;
// the quiet weave and focus-shift lines are sight-only.
//
// seedFallbackRoom (dark_room_fallback_sight_test.go) puts users 1 and 2 in
// cave room 2 at a pinned lamp and gives user 1 the named eyes. At lamp 10
// user 1 with heat eyes reads shapes and user 2 reads nothing; at lamp 60
// both read faces.

func spellChannelMob(t *testing.T) *mobs.Mob {
	t.Helper()
	m := mobs.GetInstance(100)
	require.NotNil(t, m)
	return m
}

func TestMobSpellDisruption_ShapesReadAFigure_BlindHearTheSound(t *testing.T) {
	cases := []struct {
		name  string
		send  func(*mobs.Mob, *rooms.Room)
		seen  string
		sound string
	}{
		{"concentration breaks", sendMobConcentrationBroke, "concentration breaks", spellChantBreaksOffSound},
		{"spell fizzles", func(m *mobs.Mob, r *rooms.Room) { sendMobSpellFailed(m, r, "fizzles") }, "spell fizzles", spellSputtersOutSound},
		{"spell falters", func(m *mobs.Mob, r *rooms.Room) { sendMobSpellFailed(m, r, "falters") }, "spell falters", spellSputtersOutSound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			room := seedFallbackRoom(t, 10, heatEyesConditionId)
			require.Equal(t, messaging.SightShapes, messaging.ParticipantSight(users.GetByUserId(1).Character, room))
			require.Equal(t, messaging.SightNone, messaging.ParticipantSight(users.GetByUserId(2).Character, room))

			tc.send(spellChannelMob(t), room)

			shapes, blind := drainPlain(1), drainPlain(2)
			require.Equal(t, 1, countContaining(shapes, tc.seen), "shapes reader sees it: %v", shapes)
			require.Zero(t, countContaining(shapes, "Skeleton"), "but not whose: %v", shapes)
			require.Zero(t, countContaining(shapes, tc.sound))
			require.Equal(t, 1, countContaining(blind, tc.sound), "blind reader hears it: %v", blind)
			require.Zero(t, countContaining(blind, "Skeleton"), "%v", blind)
		})
	}
}

func TestMobSpellChannel_WeaveIsSightOnly(t *testing.T) {
	room := seedFallbackRoom(t, 10, heatEyesConditionId)
	sendMobWeaving(spellChannelMob(t), room)

	shapes, blind := drainPlain(1), drainPlain(2)
	require.Equal(t, 1, countContaining(shapes, "weaves magic"), "%v", shapes)
	require.Zero(t, countContaining(shapes, "Skeleton"), "%v", shapes)
	require.Empty(t, blind, "a reader who sees nothing gets nothing for a quiet weave")
}

func TestMobSpellChannel_LitRoomNamesTheCaster(t *testing.T) {
	room := seedFallbackRoom(t, 60, heatEyesConditionId)
	m := spellChannelMob(t)
	sendMobConcentrationBroke(m, room)
	sendMobWeaving(m, room)
	got := drainPlain(2)
	require.Equal(t, 1, countContaining(got, "Skeleton's concentration breaks."), "%v", got)
	require.Equal(t, 1, countContaining(got, "Skeleton weaves magic"), "%v", got)
}

// The player caster's break (prone, grapple, and the pain of a hit) gets the
// same treatment as the mob's, so mob and player casters read alike. The
// hit path used plain Room.SendText and named the caster to everyone.
func TestPlayerConcentrationBroke_FollowsTheObserversSight(t *testing.T) {
	t.Run("shapes reads a figure", func(t *testing.T) {
		room := seedFallbackRoom(t, 10, heatEyesConditionId)
		caster := users.GetByUserId(2)
		sendPlayerConcentrationBroke(caster, room)
		got := drainPlain(1)
		require.Equal(t, 1, countContaining(got, "concentration breaks"), "%v", got)
		require.Zero(t, countContaining(got, caster.Character.Name), "%v", got)
		require.Empty(t, drainPlain(2), "the caster reads its own line, not the room's")
	})

	t.Run("sees nothing, hears the chant break off", func(t *testing.T) {
		room := seedFallbackRoom(t, 0, nightEyesConditionId)
		caster := users.GetByUserId(2)
		sendPlayerConcentrationBroke(caster, room)
		got := drainPlain(1)
		require.Equal(t, 1, countContaining(got, spellChantBreaksOffSound), "%v", got)
		require.Zero(t, countContaining(got, caster.Character.Name), "%v", got)
	})
}

func TestMobShiftsFocus_IsSightOnlyAndHidesBothNames(t *testing.T) {
	room := seedFallbackRoom(t, 10, heatEyesConditionId)
	target := users.GetByUserId(2)
	sendMobShiftsFocus(spellChannelMob(t), room, target)

	shapes, blind := drainPlain(1), drainPlain(2)
	require.Equal(t, 1, countContaining(shapes, "shifts focus"), "%v", shapes)
	require.Zero(t, countContaining(shapes, "Skeleton"), "%v", shapes)
	require.Zero(t, countContaining(shapes, target.Character.Name), "%v", shapes)
	require.Empty(t, blind, "a reader who sees nothing gets nothing for a focus shift")
}
```

- [ ] **Step 3: Run them and see them fail**

Run: `go test ./internal/hooks/ -run "TestMobSpell|TestPlayerConcentrationBroke|TestMobShiftsFocus" -count=1`

Expected (observed): five failures, every one a leaked name; the lit test passes:
```
--- FAIL: TestMobSpellDisruption_ShapesReadAFigure_BlindHearTheSound/concentration_breaks
    Messages: but not whose: [Skeleton's concentration breaks.]
--- FAIL: .../spell_fizzles      Messages: but not whose: [Skeleton's spell fizzles.]
--- FAIL: .../spell_falters      Messages: but not whose: [Skeleton's spell falters.]
--- FAIL: TestMobSpellChannel_WeaveIsSightOnly   Messages: [Skeleton weaves magic with focused intent.]
--- FAIL: TestPlayerConcentrationBroke_FollowsTheObserversSight/shapes_reads_a_figure   Messages: [Bobrick's concentration breaks.]
--- FAIL: .../sees_nothing,_hears_the_chant_break_off   Messages: [Bobrick's concentration breaks.]
--- FAIL: TestMobShiftsFocus_IsSightOnlyAndHidesBothNames   Messages: [Skeleton shifts focus to Bobrick!]
```

- [ ] **Step 4: Give `sendVisualElseAudible` an exclude list**

Replace:

```go
// skipped. Every player in the room reads exactly one of the two.
func sendVisualElseAudible(room *rooms.Room, cat messaging.Category, visualMsg, soundMsg string) {
	if room == nil {
		return
	}
	room.SendTextVisual(cat, visualMsg)
	for _, uid := range room.GetPlayers() {
		u := users.GetByUserId(uid)
```

with:

```go
// skipped. Every player in the room reads exactly one of the two, except
// excludeUserIds, who read neither (a caster reads its own line).
func sendVisualElseAudible(room *rooms.Room, cat messaging.Category, visualMsg, soundMsg string, excludeUserIds ...int) {
	if room == nil {
		return
	}
	room.SendTextVisual(cat, visualMsg, excludeUserIds...)
	for _, uid := range room.GetPlayers() {
		if isExcludedUser(uid, excludeUserIds) {
			continue
		}
		u := users.GetByUserId(uid)
```

The one other caller (`Death_MobBroadcast.go:55`) passes no excludes and is unchanged.

- [ ] **Step 5: Route the senders**

Replace the five sender bodies added in Step 1 (from `// sendMobConcentrationBroke narrates`
through the end of `sendMobShiftsFocus`) with:

```go
// sendMobConcentrationBroke narrates a mob caster's broken concentration:
// seen by sight (a figure at shapes), heard by a reader who sees nothing.
// It used plain Room.SendText, the unfiltered channel, which named the
// caster to everyone (#242).
func sendMobConcentrationBroke(mob *mobs.Mob, room *rooms.Room) {
	sendVisualElseAudible(room, messaging.CategorySpellDisruption, fmt.Sprintf(
		`%s's concentration breaks.`, mobDisplayName(mob, room, 0)),
		spellChantBreaksOffSound)
}

// sendMobSpellFailed narrates a mob's spell that fizzles (target gone) or
// falters (not enough conviction); verb is "fizzles" or "falters".
func sendMobSpellFailed(mob *mobs.Mob, room *rooms.Room, verb string) {
	sendVisualElseAudible(room, messaging.CategorySpellDisruption, fmt.Sprintf(
		`%s's spell %s.`, mobDisplayName(mob, room, 0), verb),
		spellSputtersOutSound)
}

// sendMobWeaving narrates a mob still holding its fold. Sight only: a quiet
// weave makes no sound (owner ruling R4).
func sendMobWeaving(mob *mobs.Mob, room *rooms.Room) {
	sendVisualRoomText(room, messaging.CategorySpellFold, fmt.Sprintf(
		`%s weaves magic with focused intent.`, mobDisplayName(mob, room, 0)))
}

// sendPlayerConcentrationBroke narrates a player caster's broken
// concentration to the rest of the room, as sendMobConcentrationBroke does
// for a mob: seen by sight, heard by a reader who sees nothing. The caster
// reads its own line. The pain-of-a-hit path used plain Room.SendText and
// named the caster to everyone (#242).
func sendPlayerConcentrationBroke(caster *users.UserRecord, room *rooms.Room) {
	sendVisualElseAudible(room, messaging.CategorySpellDisruption, fmt.Sprintf(
		`<ansi fg="username">%s</ansi>'s concentration breaks.`, caster.Character.Name),
		spellChantBreaksOffSound, caster.UserId)
}

// sendMobShiftsFocus narrates a mob switching its attack to a new player.
// Sight only (owner ruling R4); each reader sees both names at its own
// sight, as target.go's player shift-focus line does.
func sendMobShiftsFocus(mob *mobs.Mob, room *rooms.Room, newTarget *users.UserRecord) {
	room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
		fmt.Sprintf("%s shifts focus to <ansi fg=\"username\">%s</ansi>!", mobDisplayName(mob, room, 0), newTarget.Character.Name),
		[]string{mob.Character.Name, newTarget.Character.Name},
	)
}
```

(`sendVisualElseAudible` returns on a nil room, so `sendPlayerConcentrationBroke`
needs no nil check of its own.)

- [ ] **Step 6: Run the package; see it pass**

Run: `go vet ./internal/hooks/ && go test ./internal/hooks/ -count=1`
Expected: `ok  github.com/GoMudEngine/GoMud/internal/hooks` (the darkness golden is
unaffected).

- [ ] **Step 7: Run the root; remove the two stale registry entries**

Run: `go test . -count=1`
Expected (observed): FAIL
```
--- FAIL: TestNarrationSitesMatchViewpointAudit
    messaging_surface_guard_test.go:1500: 2 narrationViewpointRegistry entr(y/ies) are no longer found incomplete by this guard's walk:
      hooks/NewRound_DoCombat_helpers.go|<ansi fg="red">You lose your concentration as you hit the ground!</ansi>
      hooks/NewRound_DoCombat_helpers.go|<ansi fg="red">Your concentration shatters — you cannot hold the fold while grap
```
Both entries are marked "Not part of the audit" (no audit doc to update). With
the Edit tool, delete exactly those two lines from `narrationViewpointRegistry` in
`messaging_surface_guard_test.go` (master lines 1279-1280, keys beginning
`"hooks/NewRound_DoCombat_helpers.go|<ansi fg=\"red\">You lose your concentration`
and `"hooks/NewRound_DoCombat_helpers.go|<ansi fg=\"red\">Your concentration shatters`).
Then `gofmt -w messaging_surface_guard_test.go` (it realigns the map's values
column, about 28 lines; whitespace only).

Run: `go test . -count=1` then `go build ./... && go test ./internal/... ./modules/... -count=1`
Expected: `ok  github.com/GoMudEngine/GoMud`; every package `ok` (observed).

- [ ] **Step 8: Null probe**

In `sendMobConcentrationBroke`, change `sendVisualElseAudible(...)` back to
`room.SendText(messaging.CategorySpellDisruption, fmt.Sprintf(...))`; rerun Step 3's
command; confirm the `concentration_breaks` subtest fails on "Skeleton". Restore.

- [ ] **Step 9: Commit**

```bash
gofmt -l internal/hooks/ messaging_surface_guard_test.go
git add internal/hooks/NewRound_DoCombat_helpers.go internal/hooks/spell_channel_sight_test.go messaging_surface_guard_test.go
git commit -m "fix(combat): spell-channel lines follow sight; disruptions are heard (#242)

Mob concentration breaks, fizzles and falters, the weave line, the
mob focus shift and the player break on a hit all went out on the
unfiltered channel and named the caster to shapes and blind readers.
They now go through the sight pipeline. Per owner ruling R4 the
disruptions carry a sound line for a reader who sees nothing
(\"Someone's chant breaks off.\", \"A half-formed spell sputters
out.\"); the weave and focus shift are sight-only. Player prone and
grapple breaks share the mob's sender so both casters read alike.
Two viewpoint-registry entries the walk no longer finds are removed.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

## Group B, part 1: the sneak contest by ear, sneak names, hidden emotes (#333, #215, #274)

Dry-run on `origin/master` `f50400bf7`, 2026-10-08: every test below was written first, run red on
master with the output quoted under its step, then implemented and run green. `go build ./...`
clean; `go test . ./internal/actions/ ./internal/usercommands/ ./internal/configs/
./internal/mobcommands/ ./internal/behaviortree/ ./internal/hooks/` all `ok` at the end of B4.

**Scope note (verified in source, beyond the spec text):** the sneak contest runs in TWO places
with the same defect: `actions.Sneak` (`internal/actions/sneak.go:131-177`) and the sneaking
arrival roll `sneakerSpotted` (`internal/actions/move.go:293-330`, notice line `:308-309`
"%s slips into the room but you notice them."). Both get the hearing roll and the sight-aware
notice, through one helper, so the two cannot drift (CLAUDE.md: finish sibling paths).

**Wording note:** the spec lists the sneaker's tiers as name / "a figure" / "Something notices
you.". The line is rendered through `messaging.HideNames` (the spec's own rule: no new name
machinery), so at SightNone it reads "You try to blend into the shadows but something notices
you." The tier word is the same; the sentence keeps its opening.

**Root guard:** `sight_penalty_guard_test.go` (`TestEveryRollSiteAppliesTheSightPenalty`) goes
red when the observer score moves into a helper. B2 registers the helper with the guard; do not
exempt the four roll sites.

Shared test helpers defined in B2 (`newDarkDetectWorld`, `hearLines`, `hearTag`, `hearSuperCond`,
`hearInfraCond`) are used again in B3. Do B1, B2, B3, B4 in order.

---

### Task B1: `SneakHearingMult` balance knob (#333)

**Files:**
- Modify: `internal/configs/config.balance.go:297` (add field after `SneakModNoLightLitRoom`)
- Modify: `internal/configs/config.balance.combat.go:283-285` (validator after `SneakModNoLightLitRoom`)
- Modify: `_datafiles/config.yaml` (HEAD blob line 903, after `SneakModNoLightLitRoom: 0.9`)
- Create: `internal/configs/config_sneak_hearing_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/configs/config_sneak_hearing_test.go`:

```go
package configs

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

// Sight gates close-out (#333, owner 2026-10-08): an observer who sees
// nothing gets a hearing roll against a sneak, at SneakHearingMult of its
// detection score. The owner set 0.75: opposed rolls make a lower value a
// severe penalty.

// An absent key reads 0 and must take the default, so test binaries (which
// never load config.yaml) see the shipped tuning.
func TestSneakHearingMult_AbsentTakesTheDefault(t *testing.T) {
	b := Balance{}
	b.Validate()
	if b.SneakHearingMult != 0.75 {
		t.Fatalf("SneakHearingMult = %v after Validate on an empty Balance, want 0.75", b.SneakHearingMult)
	}
}

// Zero, a negative and anything above 1 (the ear beating the eye) are
// rejected like an absent key.
func TestSneakHearingMult_OutOfRangeTakesTheDefault(t *testing.T) {
	for _, v := range []ConfigFloat{-0.5, 0, 1.5} {
		b := Balance{SneakHearingMult: v}
		b.Validate()
		if b.SneakHearingMult != 0.75 {
			t.Errorf("SneakHearingMult %v validated to %v, want the 0.75 default", v, b.SneakHearingMult)
		}
	}
}

// A legal value is kept, so the guard is not simply overwriting everything.
func TestSneakHearingMult_InRangeIsKept(t *testing.T) {
	for _, v := range []ConfigFloat{0.5, 1} {
		b := Balance{SneakHearingMult: v}
		b.Validate()
		if b.SneakHearingMult != v {
			t.Errorf("SneakHearingMult %v validated to %v, want it kept", v, b.SneakHearingMult)
		}
	}
}

// The shipped config.yaml names the key and ships the owner's 0.75. An
// absent key would still read 0.75 through the default, but the owner tunes
// this in config.yaml, so it must be there to tune.
func TestSneakHearingMult_ShippedConfigShips075(t *testing.T) {
	src := shippedConfigSource(t)
	for _, line := range bytes.Split(src, []byte("\n")) {
		trimmed := strings.TrimSpace(string(line))
		if !strings.HasPrefix(trimmed, "SneakHearingMult:") {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(trimmed, "SneakHearingMult:"))
		if i := strings.Index(val, "#"); i >= 0 {
			val = strings.TrimSpace(val[:i])
		}
		got, err := strconv.ParseFloat(val, 64)
		if err != nil {
			t.Fatalf("SneakHearingMult value %q does not parse: %v", val, err)
		}
		if got != 0.75 {
			t.Fatalf("config.yaml ships SneakHearingMult %v, want 0.75", got)
		}
		return
	}
	t.Fatal("config.yaml does not name SneakHearingMult")
}
```

(`shippedConfigSource` is the existing anchored reader in
`internal/configs/config_fumble_outpays_win_test.go:18`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/configs/ -run SneakHearingMult`
Expected: build failure, observed in the dry run:
```
internal\configs\config_sneak_hearing_test.go:29:16: unknown field SneakHearingMult in struct literal of type Balance
FAIL	github.com/GoMudEngine/GoMud/internal/configs [build failed]
```

- [ ] **Step 3: Declare the field and its validator**

In `internal/configs/config.balance.go`, directly after the `SneakModNoLightLitRoom` line (:297):

```go
	SneakHearingMult            ConfigFloat `yaml:"SneakHearingMult"`            // Detection multiplier for an observer who sees nothing and can only hear a sneaker; superhearing skips it (default 0.75)
```

In `internal/configs/config.balance.combat.go`, directly after the
`if b.SneakModNoLightLitRoom <= 0 { ... }` block (:283-285):

```go
	// An observer who sees nothing hears a sneaker at this fraction of its
	// detection score (sight gates close-out, #333, owner 2026-10-08). Above
	// 1 the ear would beat the eye, so it is rejected like an absent key.
	if !(b.SneakHearingMult > 0) || b.SneakHearingMult > 1 {
		b.SneakHearingMult = 0.75
	}
```

(`!(x > 0)` rather than `x <= 0` so a NaN from YAML also falls back, the house pattern at
`config.balance.combat.go:248`.)

Run: `go test ./internal/configs/ -run SneakHearingMult -v`
Expected: the three `Balance` tests PASS; `TestSneakHearingMult_ShippedConfigShips075` FAILS with
`config.yaml does not name SneakHearingMult` (observed).

- [ ] **Step 4: Add the key to config.yaml**

`_datafiles/config.yaml` carries skip-worktree in the owner's main checkout. First check this
worktree: `git ls-files -v _datafiles/config.yaml`.
- `H` (a fresh worktree, the usual case): edit the file with the Edit tool and stage it normally.
- `S`: build the change from the blob instead: `git show HEAD:_datafiles/config.yaml >
  "$TMP/config.head.yaml"`, apply the same Edit to that copy, then
  `git update-index --cacheinfo 100644,$(git hash-object -w "$TMP/config.head.yaml"),_datafiles/config.yaml`,
  then `git update-index --skip-worktree _datafiles/config.yaml` (cacheinfo clears the bit) and
  confirm `git ls-files -v _datafiles/config.yaml` prints `S`.

The edit: after the line `  SneakModNoLightLitRoom: 0.9       # Alert-observer modifier: dark sneaker in lit room`
insert:

```yaml
  # SneakHearingMult: an observer who sees nothing (pitch dark, blinded) can
  #   still HEAR a sneak or a sneaking arrival, at this fraction of its
  #   detection score (Perception + Search). The superhearing condition flag
  #   skips it. Opposed rolls make a low value a severe penalty, so the owner
  #   started it at 0.75 (sight gates close-out, #333, 2026-10-08). Range
  #   (0, 1]; anything else falls back to 0.75.
  SneakHearingMult: 0.75
```

After merge, apply the same insert to the owner's local disk copy of `_datafiles/config.yaml`
in the main checkout (Edit tool; it is skip-worktree, so git will not show it), or
`TestSneakHearingMult_ShippedConfigShips075` fails in that checkout.

- [ ] **Step 5: Run tests to verify they pass, and prove the guard can fail**

Run: `go test ./internal/configs/`
Expected: `ok  	github.com/GoMudEngine/GoMud/internal/configs` (observed 0.351s).

Null probe: change `|| b.SneakHearingMult > 1 {` to `|| b.SneakHearingMult > 2 {`, run
`go test ./internal/configs/ -run SneakHearingMult`. Observed:
```
--- FAIL: TestSneakHearingMult_OutOfRangeTakesTheDefault (0.00s)
    config_sneak_hearing_test.go:32: SneakHearingMult 1.5 validated to 1.5, want the 0.75 default
```
Restore `> 1` and rerun: `ok`.

- [ ] **Step 6: Commit**

```bash
git add internal/configs/config.balance.go internal/configs/config.balance.combat.go internal/configs/config_sneak_hearing_test.go _datafiles/config.yaml
git commit -m "feat(config): SneakHearingMult, the no-sight observer's ear (#333)

Owner ruling 2026-10-08: an observer who sees nothing gets a hearing roll
against a sneak, at 0.75 of its detection score. Range (0, 1]; anything
else falls back to 0.75.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task B2: observers who see nothing roll by ear; the observer's notice follows its sight (#333, #215 observer half)

**Files:**
- Modify: `internal/actions/skill_helpers.go:3-11` (import), `:87-92` (`CalcDetectionScore`; add `CalcHearingScore`, `detectionBase`)
- Modify: `internal/actions/sneak.go:36` (add `sneakObserverScore`, `sneakNoticeLine` above `MobIsSneaking`), `:144-151` (player loop), `:168-169` (mob loop)
- Modify: `internal/actions/move.go:305-310` (player loop of `sneakerSpotted`), `:322-323` (mob loop)
- Modify: `sight_penalty_guard_test.go:82-90` (register the helper)
- Modify: `internal/actions/context.md:724-731` (sneak roll and success/failure bullets)
- Create: `internal/actions/sneak_hearing_test.go`

- [ ] **Step 1: Write the failing behavioural tests**

Create `internal/actions/sneak_hearing_test.go` (the two unit tests that name the new functions
are added in Step 4, so this step's red is behavioural, not a compile error):

```go
package actions

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// Sight gates close-out, #333 and #215 (owner 2026-10-08). An observer who
// sees nothing gets a HEARING roll against a sneak, at SneakHearingMult of
// its detection score; the superhearing flag skips the multiplier. A notice
// names the sneaker only as far as the observer's eyes allow: the name at
// clear sight, "a figure" at shapes, and at none only that someone was
// heard. The same contest runs in two places, the sneak command (Sneak) and
// a sneaking arrival (EntryDetection), and both follow the rule.

const (
	hearSuperCond = 9631
	hearInfraCond = 9632
)

var hearTag = regexp.MustCompile(`<[^>]*>`)

// newDarkDetectWorld is newDetectWorld with the lamp out: a plain-eyed
// observer in it sees nothing (SightNone). mult pins SneakHearingMult.
func newDarkDetectWorld(t *testing.T, mult float64) *detectWorld {
	t.Helper()
	pinDetectionKnobs(t)
	c := configs.GetConfig()
	c.Balance.SneakHearingMult = configs.ConfigFloat(mult)
	configs.SetConfigForTest(t, c)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		hearSuperCond: {ConditionId: hearSuperCond, Name: "Test Sharp Ears", RoundInterval: 1, TriggerCount: 50,
			Flags: []conditions.Flag{conditions.SuperHearing}},
		hearInfraCond: {ConditionId: hearInfraCond, Name: "Test Heat Eyes", RoundInterval: 1, TriggerCount: 50,
			Flags:   []conditions.Flag{conditions.InfraredVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}}},
	}))
	dest := &rooms.Room{RoomId: 9800, Zone: "MoveDetect", SkyLight: rooms.SkyLightPtr(0), Lamp: rooms.LampPtr(0)}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{9800: dest}, map[string]*rooms.ZoneConfig{}))
	return &detectWorld{dest: dest}
}

// hearLines is what uid's client prints, tags stripped.
func hearLines(uid int) []string {
	var out []string
	for _, m := range events.DrainQueuedMessageEventsForTest(uid) {
		out = append(out, strings.TrimSpace(hearTag.ReplaceAllString(m.Text, "")))
	}
	return out
}

// A sharp observer in the dark no longer spots a sneaker by sight: with the
// knob pinned near zero its ear cannot win. On master it rolled at the sight
// ramp's 0.80 floor and won.
func TestSneak_BlindObserverRollsByEar(t *testing.T) {
	w := newDarkDetectWorld(t, 0.01)
	_, obs := w.place(t, "player", 9811, "Watcher")
	obs.Stats.Perception.ValueAdj = 1000
	actor, mc := w.place(t, "player", 9810, "Sneak")
	mc.Stats.Dexterity.ValueAdj = 100

	got := Sneak(actor)

	require.True(t, got.Success, "a blind observer at 1000 x 0.01 cannot hear a sneaker at 100")
	require.Empty(t, hearLines(9811))
}

// Superhearing skips the multiplier, so the same sharp observer hears it.
func TestSneak_SuperhearingObserverStillHears(t *testing.T) {
	w := newDarkDetectWorld(t, 0.01)
	_, obs := w.place(t, "player", 9811, "Watcher")
	obs.Stats.Perception.ValueAdj = 1000
	require.True(t, obs.Conditions.AddCondition(hearSuperCond, true))
	actor, mc := w.place(t, "player", 9810, "Sneak")
	mc.Stats.Dexterity.ValueAdj = 0
	events.DrainQueuedMessageEventsForTest(9811)

	got := Sneak(actor)

	require.False(t, got.Success)
	require.Equal(t, []string{"You hear someone trying to move quietly."}, hearLines(9811),
		"heard, never named")
}

// The observer's notice follows its sight, for both contests.
func TestSneakNotice_FollowsObserverSight(t *testing.T) {
	cases := []struct {
		name  string
		infra bool
		want  string
	}{
		{"blind observer hears", false, "You hear someone trying to move quietly."},
		{"heat-sighted observer sees a figure", true, "A figure tries to hide but you notice them."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newDarkDetectWorld(t, 0.75)
			_, obs := w.place(t, "player", 9811, "Watcher")
			obs.Stats.Perception.ValueAdj = 1000
			if c.infra {
				require.True(t, obs.Conditions.AddCondition(hearInfraCond, true))
			}
			actor, mc := w.place(t, "player", 9810, "Sneak")
			mc.Stats.Dexterity.ValueAdj = 0
			events.DrainQueuedMessageEventsForTest(9811)

			require.False(t, Sneak(actor).Success)
			require.Equal(t, []string{c.want}, hearLines(9811))
		})
	}
}

func TestEntryDetectionNotice_FollowsObserverSight(t *testing.T) {
	cases := []struct {
		name  string
		infra bool
		want  string
	}{
		{"blind observer hears", false, "You hear someone trying to move quietly."},
		{"heat-sighted observer sees a figure", true, "A figure slips into the room but you notice them."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newDarkDetectWorld(t, 0.75)
			_, obs := w.place(t, "player", 9811, "Watcher")
			obs.Stats.Perception.ValueAdj = 1000
			if c.infra {
				require.True(t, obs.Conditions.AddCondition(hearInfraCond, true))
			}
			mover, mc := w.place(t, "player", 9810, "Sneak")
			mc.Stats.Dexterity.ValueAdj = 0
			hideForMove(t, mc)
			mc.SetMiscData(`sneaking`, true)
			events.DrainQueuedMessageEventsForTest(9811)

			got := EntryDetection(mover, w.dest, true)

			require.False(t, got.StillSneaking)
			require.Equal(t, []string{c.want}, hearLines(9811))
		})
	}
}

// A sneaking arrival meets the same ear: a sharp blind observer with the knob
// near zero does not catch it.
func TestEntryDetection_BlindObserverRollsByEar(t *testing.T) {
	w := newDarkDetectWorld(t, 0.01)
	_, obs := w.place(t, "player", 9811, "Watcher")
	obs.Stats.Perception.ValueAdj = 1000
	mover, mc := w.place(t, "player", 9810, "Sneak")
	mc.Stats.Dexterity.ValueAdj = 100
	hideForMove(t, mc)
	mc.SetMiscData(`sneaking`, true)

	got := EntryDetection(mover, w.dest, true)

	require.True(t, got.StillSneaking)
}
```

(`pinDetectionKnobs`, `detectWorld`, `place` and `hideForMove` are the existing fixtures in
`internal/actions/move_detection_test.go:20-89`.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/actions/ -run "TestSneak_BlindObserverRollsByEar|TestSneak_SuperhearingObserverStillHears|TestSneakNotice_FollowsObserverSight|TestEntryDetectionNotice_FollowsObserverSight|TestEntryDetection_BlindObserverRollsByEar"`
Expected, observed on master:
```
--- FAIL: TestSneak_BlindObserverRollsByEar (0.00s)
        	Messages:   	a blind observer at 1000 x 0.01 cannot hear a sneaker at 100
--- FAIL: TestSneak_SuperhearingObserverStillHears (0.00s)
        	            	actual  : []string{"Sneak tries to hide but you notice them."}
    --- FAIL: TestSneakNotice_FollowsObserverSight/heat-sighted_observer_sees_a_figure (0.00s)
        	            	actual  : []string{"Sneak tries to hide but you notice them."}
    --- FAIL: TestEntryDetectionNotice_FollowsObserverSight/blind_observer_hears (0.00s)
        	            	actual  : []string{"Sneak slips into the room but you notice them."}
--- FAIL: TestEntryDetection_BlindObserverRollsByEar (0.00s)
FAIL
```

- [ ] **Step 3: Implement the hearing score and the shared observer helpers**

In `internal/actions/skill_helpers.go`, add the import after `combat`:

```go
	"github.com/GoMudEngine/GoMud/internal/conditions"
```

Replace `CalcDetectionScore`'s body (:87-92) and add two functions after it:

```go
func CalcDetectionScore(c *characters.Character, room messaging.RoomVisibility) float64 {
	return detectionBase(c) * messaging.SightMult(c, room)
}

// CalcHearingScore is CalcDetectionScore for an observer who sees nothing
// (sight gates close-out, #333, owner 2026-10-08): no sight ramp, since there
// is no sight to price, but Balance.SneakHearingMult instead. The superhearing
// condition flag skips that multiplier. Only the sneak contests ask for it,
// through sneakObserverScore.
func CalcHearingScore(c *characters.Character) float64 {
	base := detectionBase(c)
	if c.HasConditionFlag(conditions.SuperHearing) {
		return base
	}
	return base * float64(configs.GetBalanceConfig().SneakHearingMult)
}

// detectionBase is an observer's detection before any sense prices it:
// Perception + rank(search)*SkillWeight.
func detectionBase(c *characters.Character) float64 {
	return float64(c.Stats.Perception.ValueAdj) +
		float64(c.GetSkillLevel(skills.Search))*
			float64(configs.GetBalanceConfig().SkillWeight)
}
```

In `internal/actions/sneak.go`, insert directly above `// MobIsSneaking derives` (:36):

```go
// sneakObserverScore is an observer's side of a sneak contest, the sneak
// command's and a sneaking arrival's alike, and the sight it was priced at.
// An observer who sees nothing hears for the sneaker (CalcHearingScore); one
// who makes out anything looks (CalcDetectionScore). Sight gates close-out,
// #333, owner 2026-10-08.
func sneakObserverScore(observer *characters.Character, room messaging.RoomVisibility) (float64, messaging.SightDecision) {
	sight := messaging.ParticipantSight(observer, room)
	if sight == messaging.SightNone {
		return CalcHearingScore(observer), sight
	}
	return CalcDetectionScore(observer, room), sight
}

// sneakNoticeLine is what an observer who wins a sneak contest reads. seen
// is the line with the sneaker's name tagged (moverName); the name follows
// the observer's sight (HideNames), and an observer who sees nothing only
// heard someone (#215, #333).
func sneakNoticeLine(seen, sneakerName string, sight messaging.SightDecision) string {
	if sight == messaging.SightNone {
		return `You hear someone trying to move quietly.`
	}
	return messaging.HideNames(seen, []string{sneakerName}, sight)
}
```

In `Sneak`'s player loop (:144-151) replace:

```go
		observerScore := CalcDetectionScore(observer.Character, room)
		rollHappened = true
		success := combat.RunContest(sneakScore, []contest.Entry{{Score: observerScore}}).Success
		if !success {
			// Notify the observing player.
			observer.SendText(messaging.CategorySystem,
				`<ansi fg="username">`+actor.GetName()+`</ansi> tries to hide but you notice them.`,
			)
```

with:

```go
		observerScore, sight := sneakObserverScore(observer.Character, room)
		rollHappened = true
		success := combat.RunContest(sneakScore, []contest.Entry{{Score: observerScore}}).Success
		if !success {
			// Notify the observing player, as far as its sight allows.
			observer.SendText(messaging.CategorySystem, sneakNoticeLine(
				moverName(actor)+` tries to hide but you notice them.`, actor.GetName(), sight))
```

(`moverName`, `move.go:226`, tags a mob sneaker `mobname`; the old line tagged every sneaker
`username`.)

In `Sneak`'s mob loop (:169) replace `observerScore := CalcDetectionScore(&m.Character, room)`
with `observerScore, _ := sneakObserverScore(&m.Character, room)`.

In `internal/actions/move.go` `sneakerSpotted`, player loop (:306-310) replace:

```go
		observerScore := CalcDetectionScore(p.Character, dest)
		if !combat.RunContest(sneakScore, []contest.Entry{{Score: observerScore}}).Success {
			NewUserActor(p).SendText(messaging.CategorySystem, fmt.Sprintf(
				`%s slips into the room but you notice them.`, moverName(mover)))
			return true
		}
```

with:

```go
		observerScore, sight := sneakObserverScore(p.Character, dest)
		if !combat.RunContest(sneakScore, []contest.Entry{{Score: observerScore}}).Success {
			NewUserActor(p).SendText(messaging.CategorySystem, sneakNoticeLine(
				moverName(mover)+` slips into the room but you notice them.`, mover.GetName(), sight))
			return true
		}
```

and in its mob loop (:323) replace `observerScore := CalcDetectionScore(&m.Character, dest)`
with `observerScore, _ := sneakObserverScore(&m.Character, dest)`. (`fmt` stays imported in
`move.go`; other lines use it.)

- [ ] **Step 4: Add the unit tests for the two new functions**

In `internal/actions/sneak_hearing_test.go`, add `"github.com/GoMudEngine/GoMud/internal/messaging"`
to the imports, and insert above `// A sharp observer in the dark no longer spots`:

```go
func TestCalcHearingScore_PaysTheKnobUnlessSuperhearing(t *testing.T) {
	w := newDarkDetectWorld(t, 0.75)
	_, obs := w.place(t, "player", 9811, "Watcher")
	obs.Stats.Perception.ValueAdj = 200

	require.InDelta(t, 150.0, CalcHearingScore(obs), 0.001, "Perception 200 at 0.75")

	require.True(t, obs.Conditions.AddCondition(hearSuperCond, true))
	require.InDelta(t, 200.0, CalcHearingScore(obs), 0.001, "superhearing skips the multiplier")
}

// sneakObserverScore prices an observer by ear exactly when it sees nothing.
func TestSneakObserverScore_EarOnlyAtSightNone(t *testing.T) {
	w := newDarkDetectWorld(t, 0.5)
	_, blind := w.place(t, "player", 9811, "Watcher")
	blind.Stats.Perception.ValueAdj = 200
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(blind, w.dest))

	score, sight := sneakObserverScore(blind, w.dest)
	require.Equal(t, messaging.SightNone, sight)
	require.InDelta(t, CalcHearingScore(blind), score, 0.001)

	_, heat := w.place(t, "player", 9812, "Seer")
	heat.Stats.Perception.ValueAdj = 200
	require.True(t, heat.Conditions.AddCondition(hearInfraCond, true))
	score, sight = sneakObserverScore(heat, w.dest)
	require.Equal(t, messaging.SightShapes, sight)
	require.InDelta(t, CalcDetectionScore(heat, w.dest), score, 0.001)
}
```

- [ ] **Step 5: Register the helper with the root sight guard**

Without this, `go test .` fails (observed):
```
--- FAIL: TestEveryRollSiteAppliesTheSightPenalty (0.11s)
    sight_penalty_guard_test.go:447: roll sites with no sight penalty:
          internal/actions/move.go:307 in sneakerSpotted: combat.RunContest
          internal/actions/move.go:324 in sneakerSpotted: combat.RunContest
          internal/actions/sneak.go:190 in Sneak: combat.RunContest
          internal/actions/sneak.go:214 in Sneak: combat.RunContest
```
In `sight_penalty_guard_test.go`, `sightPenaltyHelpers` (:82-90), add after `"stealVictimScore": true,`:

```go
	// The sneak contests' observer side (Sneak, sneakerSpotted). It prices an
	// observer who sees anything through CalcDetectionScore; one who sees
	// nothing has no sight to price and rolls by ear instead (#333, owner
	// 2026-10-08).
	"sneakObserverScore": true,
```

- [ ] **Step 6: Update `internal/actions/context.md`**

In the sneak section (:724-731) replace:

```
  Each observer uses effective Perception plus the Search skill multiplier.
  Resolution flows through `combat.RunContest`.
- **Success/failure:** Success resolves Concealing to Hidden, queues the Hidden
  condition mirror, sets the `sneaking` misc key, and returns `Success`. The first
  observer who wins resolves the actor back to Visible and populates
  `SpottedByName`. `RollHappened` distinguishes a contested attempt from an
  empty-room success.
```

with:

```
  Each observer uses effective Perception plus the Search skill multiplier,
  priced by `sneakObserverScore`: by sight (`CalcDetectionScore`) when it
  makes out anything, by ear (`CalcHearingScore`, `Balance.SneakHearingMult`,
  skipped for the superhearing flag) when it sees nothing (#333, owner
  2026-10-08). A sneaking arrival (`sneakerSpotted`) prices observers the same
  way. Resolution flows through `combat.RunContest`.
- **Success/failure:** Success resolves Concealing to Hidden, queues the Hidden
  condition mirror, sets the `sneaking` misc key, and returns `Success`. The first
  observer who wins resolves the actor back to Visible and populates
  `SpottedBy` (the observer's character, not a name). The observer's notice
  (`sneakNoticeLine`) names the sneaker only as far as the observer sees, and
  says only "You hear someone trying to move quietly." when it sees nothing;
  the sneaker's line (`SpottedLine`) names the spotter only as far as the
  sneaker sees (#215). `RollHappened` distinguishes a contested attempt from
  an empty-room success.
```

(`SpottedBy` and `SpottedLine` land in B3; B2 and B3 ship in the same PR, and this text is
written once here so B3 does not touch it again.)

- [ ] **Step 7: Run tests to verify they pass, and prove the branch can fail**

Run: `go build ./... && go test ./internal/actions/ -run "Hearing|SneakObserverScore|BlindObserver|Superhearing|SneakNotice|EntryDetectionNotice" -v`
Expected (observed): all PASS, including both subtests of each table.

Run: `go test . ./internal/actions/`
Expected (observed): `ok  	github.com/GoMudEngine/GoMud` and `ok  	github.com/GoMudEngine/GoMud/internal/actions`.

Null probe: in `sneakObserverScore` change `return CalcHearingScore(observer), sight` to
`return CalcDetectionScore(observer, room), sight`, run
`go test ./internal/actions/ -run "BlindObserver|SneakObserverScore"`. Observed:
```
--- FAIL: TestSneakObserverScore_EarOnlyAtSightNone (0.00s)
--- FAIL: TestSneak_BlindObserverRollsByEar (0.00s)
--- FAIL: TestEntryDetection_BlindObserverRollsByEar (0.00s)
```
Restore and rerun: `ok`.

Also run: `python -I tools/context_md_audit.py` and confirm nothing it prints names
`internal/actions` (observed: no actions entries).

- [ ] **Step 8: Commit**

```bash
git add internal/actions/skill_helpers.go internal/actions/sneak.go internal/actions/move.go internal/actions/sneak_hearing_test.go internal/actions/context.md sight_penalty_guard_test.go
git commit -m "feat(stealth): an observer who sees nothing hears for a sneak (#333, #215)

Owner ruling 2026-10-08: an observer at SightNone rolls by ear,
Perception + Search at SneakHearingMult, superhearing exempt. Its notice
says only that it heard someone; a sighted observer's notice names the
sneaker as far as its sight allows. The sneak command and a sneaking
arrival share sneakObserverScore and sneakNoticeLine, so the two contests
agree. The root sight guard learns the helper.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task B3: the sneaker learns its spotter only as far as it can see (#215 sneaker half)

**Files:**
- Modify: `internal/actions/sneak.go:22-24` (`SneakResult.SpottedByName` becomes `SpottedBy`), `:64` (doc), `:155`, `:176` (returns); add `SpottedLine` above `// MobIsSneaking derives`
- Modify: `internal/usercommands/skill.skullduggery.sneak.go:63-71`
- Modify: `internal/actions/move_detection_own_pets_test.go:73`, `:90`
- Create: `internal/actions/sneak_spotted_line_test.go`
- Create: `internal/usercommands/skill_skullduggery_sneak_sight_test.go`

- [ ] **Step 1: Write the failing command-level test**

Create `internal/usercommands/skill_skullduggery_sneak_sight_test.go`:

```go
package usercommands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Sight gates close-out, #215: a sneaker spotted in pitch dark by an observer
// who heard them is not told the observer's name ("... but Sil Vantage
// notices you." in the 2026-09-30 playtest). The name follows the sneaker's
// own sight: here, none.
func TestSneakCommand_SpotterNotNamedInPitchDark(t *testing.T) {
	const superCond = 9941
	c := configs.GetConfig()
	c.Balance.ContestFloor = 0
	c.Balance.ContestGapSaturation = 0
	configs.SetConfigForTest(t, c)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		superCond: {ConditionId: superCond, Name: "Test Sharp Ears", RoundInterval: 1, TriggerCount: 50,
			Flags: []conditions.Flag{conditions.SuperHearing}},
	}))

	room := &rooms.Room{RoomId: 9940, Zone: "SneakSight", SkyLight: rooms.SkyLightPtr(0), Lamp: rooms.LampPtr(0)}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{9940: room}, map[string]*rooms.ZoneConfig{}))

	sneaker := users.NewTestUser(9942, "sneak", "Sneak", 0)
	sneaker.Character.Skills = map[string]int{string(skills.Skullduggery): 1}
	sneaker.Character.Stats.Dexterity.ValueAdj = 0
	watcher := users.NewTestUser(9943, "watcher", "Watcher", 0)
	watcher.Character.Stats.Perception.ValueAdj = 1000
	require.True(t, watcher.Character.Conditions.AddCondition(superCond, true))
	for _, u := range []*users.UserRecord{sneaker, watcher} {
		u.Character.RoomId = room.RoomId
	}
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9942: sneaker, 9943: watcher}))
	room.AddPlayer(9942)
	room.AddPlayer(9943)
	events.DrainQueuedMessagesForTest(9942)

	handled, err := Sneak("", sneaker, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	out := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(
		strings.Join(events.DrainQueuedMessagesForTest(9942), "\n"), "")
	require.Contains(t, out, "You try to blend into the shadows but something notices you.")
	require.NotContains(t, out, "Watcher", "the sneaker sees nothing, so learns no name")
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/usercommands/ -run TestSneakCommand_SpotterNotNamedInPitchDark`
Expected, observed on master (note the player spotter also wears the mob colour):
```
--- FAIL: TestSneakCommand_SpotterNotNamedInPitchDark (0.00s)
        	Error:      	"<ansi fg=\"system\">You try to blend into the shadows but <ansi fg=\"mobname\">Watcher</ansi> notices you.</ansi>\n" does not contain "something notices you."
FAIL
```

- [ ] **Step 3: Write the unit tests for `SpottedLine`**

Create `internal/actions/sneak_spotted_line_test.go`:

```go
package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// Sight gates close-out, #215: a failed sneak tells the sneaker who spotted
// them only as far as the SNEAKER's own sight allows. Sneak carries the
// spotter's identity (SpottedBy), not a bare name, and SpottedLine renders it:
// the name at clear sight, "a figure" at shapes, "something" when the sneaker
// sees nothing.
func TestSpottedLine_FollowsSneakerSight(t *testing.T) {
	cases := []struct {
		name  string
		lamp  int
		infra bool
		want  string
	}{
		{"lit room names the spotter", 80, false,
			"You try to blend into the shadows but Watcher notices you."},
		{"heat sight in the dark sees a figure", 0, true,
			"You try to blend into the shadows but a figure notices you."},
		{"pitch dark sees nothing", 0, false,
			"You try to blend into the shadows but something notices you."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newDarkDetectWorld(t, 0.75)
			w.dest.Lamp = rooms.LampPtr(c.lamp)
			_, obs := w.place(t, "player", 9811, "Watcher")
			obs.Stats.Perception.ValueAdj = 1000
			require.True(t, obs.Conditions.AddCondition(hearSuperCond, true), "hears even where it cannot see")
			actor, mc := w.place(t, "player", 9810, "Sneak")
			mc.Stats.Dexterity.ValueAdj = 0
			if c.infra {
				require.True(t, mc.Conditions.AddCondition(hearInfraCond, true))
			}

			got := Sneak(actor)

			require.False(t, got.Success)
			require.NotNil(t, got.SpottedBy)
			require.Equal(t, "Watcher", got.SpottedBy.Name)
			line := SpottedLine(mc, w.dest, got.SpottedBy)
			require.Equal(t, c.want, hearTag.ReplaceAllString(line, ""))
		})
	}
}

// A spotter the sneaker cannot perceive (hidden, no see-hidden) is never
// named, even in a lit room.
func TestSpottedLine_HiddenSpotterIsNotNamed(t *testing.T) {
	w := newDarkDetectWorld(t, 0.75)
	w.dest.Lamp = rooms.LampPtr(80)
	_, obs := w.place(t, "player", 9811, "Watcher")
	hideForMove(t, obs)
	_, mc := w.place(t, "player", 9810, "Sneak")
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(mc, w.dest))

	line := SpottedLine(mc, w.dest, obs)

	require.Equal(t, "You try to blend into the shadows but something notices you.",
		hearTag.ReplaceAllString(line, ""))
}
```

Run: `go test ./internal/actions/ -run SpottedLine`
Expected: build failure (observed): `got.SpottedBy undefined (type SneakResult has no field or
method SpottedBy)` and `undefined: SpottedLine`.

- [ ] **Step 4: Implement**

In `internal/actions/sneak.go`, replace the field (:22-24):

```go
	// SpottedByName is the name of the observer who detected the actor.
	// Empty when Success is true.
	SpottedByName string
```

with:

```go
	// SpottedBy is the observer who detected the actor; nil when Success is
	// true. It is the observer itself, not a name, so the caller words it at
	// the sneaker's own sight (SpottedLine, #215).
	SpottedBy *characters.Character
```

In `Sneak`'s doc comment (:64) change `SpottedByName is set.` to `SpottedBy is set.`

Replace the two returns:
- `:155` `return SneakResult{Cost: cost, SpottedByName: observer.Character.Name, RollHappened: true}`
  becomes `return SneakResult{Cost: cost, SpottedBy: observer.Character, RollHappened: true}`
- `:176` `return SneakResult{Cost: cost, SpottedByName: m.Character.Name, RollHappened: true}`
  becomes `return SneakResult{Cost: cost, SpottedBy: &m.Character, RollHappened: true}`

Insert above `// MobIsSneaking derives` (after B2's helpers):

```go
// SpottedLine is what a sneaker reads when spotter catches the attempt, the
// spotter named only as far as the SNEAKER's own sight allows (#215): the
// name at clear sight, "a figure" at shapes, "something" when the sneaker
// sees nothing or cannot perceive the spotter (a hidden spotter is never
// named).
func SpottedLine(sneaker *characters.Character, room messaging.RoomVisibility, spotter *characters.Character) string {
	tag := `mobname`
	if spotter.GetUserId() > 0 {
		tag = `username`
	}
	line := `You try to blend into the shadows but <ansi fg="` + tag + `">` +
		spotter.Name + `</ansi> notices you.`
	sight := messaging.ParticipantSight(sneaker, room)
	if !sneaker.Perceives(spotter) {
		sight = messaging.SightNone
	}
	return messaging.HideNames(line, []string{spotter.Name}, sight)
}
```

In `internal/usercommands/skill.skullduggery.sneak.go` (:63-71) replace:

```go
	case result.SpottedByName != "":
		// Apply failure cooldown so the player can't spam sneak
		if cfg.SneakFailCooldown > 0 {
			user.Character.TryCooldown(sneakCooldownKey,
				fmt.Sprintf(`%d rounds`, cfg.SneakFailCooldown))
		}
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`You try to blend into the shadows but <ansi fg="mobname">%s</ansi> notices you.`,
			result.SpottedByName))
```

with:

```go
	case result.SpottedBy != nil:
		// Apply failure cooldown so the player can't spam sneak
		if cfg.SneakFailCooldown > 0 {
			user.Character.TryCooldown(sneakCooldownKey,
				fmt.Sprintf(`%d rounds`, cfg.SneakFailCooldown))
		}
		// The spotter is named only as far as the sneaker can see (#215).
		user.SendText(messaging.CategorySystem,
			actions.SpottedLine(user.Character, room, result.SpottedBy))
```

In `internal/actions/move_detection_own_pets_test.go`:
- `:73` `require.Empty(t, got.SpottedByName, "your own pet never notices you")` becomes
  `require.Nil(t, got.SpottedBy, "your own pet never notices you")`
- `:90` `require.Equal(t, "Rocky", got.SpottedByName)` becomes
  ```go
  	require.NotNil(t, got.SpottedBy)
  	require.Equal(t, "Rocky", got.SpottedBy.Name)
  ```

Run `grep -rn "SpottedByName" --include=*.go .` and expect no output (observed: none; the only
callers were the user command and the pet tests; no mob command reads it).

- [ ] **Step 5: Run tests to verify they pass**

Run: `go build ./... && go test ./internal/actions/ -run "SpottedLine|OwnPet|OtherPlayersPet" -v`
Expected (observed): PASS for `TestSpottedLine_FollowsSneakerSight` (3 subtests),
`TestSpottedLine_HiddenSpotterIsNotNamed`, `TestSneak_OwnPetDoesNotNoticePlayer`,
`TestSneak_OtherPlayersPetStillNoticesPlayer`, `TestEntryDetection_OwnPetDoesNotSpotSneakingPlayer`,
`TestEntryDetection_OtherPlayersPetStillSpotsSneaker`.

Run: `go test ./internal/usercommands/ -run TestSneakCommand_SpotterNotNamedInPitchDark`
Expected (observed): `ok`.

- [ ] **Step 6: Commit**

```bash
git add internal/actions/sneak.go internal/actions/sneak_spotted_line_test.go internal/actions/move_detection_own_pets_test.go internal/usercommands/skill.skullduggery.sneak.go internal/usercommands/skill_skullduggery_sneak_sight_test.go
git commit -m "fix(stealth): name a sneak's spotter only as far as the sneaker sees (#215)

SneakResult carries the spotter (SpottedBy), not its name, and
SpottedLine words it at the sneaker's own sight: the name, a figure, or
something; a hidden spotter is never named. A player spotter is tagged
username, not mobname.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task B4: a hidden actor does not emote (#274)

**Files:**
- Modify: `internal/actions/room_lines.go:27-37` (`SendSeen`)
- Modify: `internal/usercommands/emote.go:26-30`, `:38`, `:61-68` (call `hiddenEmoteNote`; add it)
- Modify: `internal/actions/context.md:601-606` (`SendSeen` bullet)
- Create: `internal/actions/send_seen_hidden_test.go`
- Create: `internal/usercommands/emote_hidden_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/actions/send_seen_hidden_test.go` (uses the 5b speech scene in
`internal/actions/speech_sight_test.go:58-190`):

```go
package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// Sight gates close-out, #274 (owner R3, 2026-10-08): a hidden actor does not
// emote. A hidden mob's behaviour-tree greet ("A Liveried Footman pays you
// little mind.") gave it away; the same path carries a hidden player's emote.
// Silence for every listener, not an anonymous line.
func TestSendSeen_HiddenActorReachesNoOne(t *testing.T) {
	cases := []struct {
		name   string
		player bool
		cat    messaging.Category
		line   string
	}{
		{"hidden player", true, messaging.CategoryEmote,
			FormatEmoteText("Kesh", "waves.", "username")},
		{"hidden mob", false, messaging.CategoryMobEmote,
			FormatMobEmoteText("Grel", "pays you little mind.")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sc := newSpeechScene(t)
			hideForMove(t, sc.player.Character)
			hideForMove(t, &sc.mob.Character)
			checkSpeech(t, sc, c.player, func(a Actor) { SendSeen(a, c.cat, c.line, false) },
				speechWant{})
		})
	}
}
```

Create `internal/usercommands/emote_hidden_test.go`:

```go
package usercommands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Sight gates close-out, #274 (owner R3, 2026-10-08): a hidden player's emote
// reaches no one, and the player is told why, after their own echo. Every
// form: empty, alias, free text, and the @ form that has no echo.
func TestEmote_HiddenPlayerIsToldNoOneSees(t *testing.T) {
	const note = "No one sees it; you are hidden."
	tags := regexp.MustCompile(`<[^>]*>`)
	cases := []struct {
		name, rest, echo string
	}{
		{"empty", "", "You emote."},
		{"alias", "beam", "You Emote: Hider beams with pride."},
		{"free text", "waves.", "You Emote: Hider waves."},
		{"at form", "@waves.", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			room := &rooms.Room{RoomId: 9950, Zone: "EmoteHidden", SkyLight: rooms.SkyLightPtr(0), Lamp: rooms.LampPtr(80)}
			t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{9950: room}, map[string]*rooms.ZoneConfig{}))
			hider := users.NewTestUser(9951, "hider", "Hider", 0)
			watcher := users.NewTestUser(9952, "watcher", "Watcher", 0)
			for _, u := range []*users.UserRecord{hider, watcher} {
				u.Character.RoomId = room.RoomId
			}
			t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9951: hider, 9952: watcher}))
			room.AddPlayer(9951)
			room.AddPlayer(9952)
			reason := state.TransitionReason{Trigger: "emote_hidden_test"}
			require.NoError(t, hider.Character.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason))
			hider.Character.Awareness.ResolveConcealment(true, reason)
			require.True(t, hider.Character.IsHidden())
			events.DrainQueuedMessagesForTest(9951)
			events.DrainQueuedMessagesForTest(9952)
			events.DrainQueuedRoomMessagesForTest(9950)

			handled, err := Emote(c.rest, hider, room, 0)
			require.True(t, handled)
			require.NoError(t, err)

			var own []string
			for _, line := range events.DrainQueuedMessagesForTest(9951) {
				own = append(own, strings.TrimSpace(tags.ReplaceAllString(line, "")))
			}
			want := []string{note}
			if c.echo != "" {
				want = []string{c.echo, note}
			}
			require.Equal(t, want, own)
			require.Empty(t, events.DrainQueuedMessagesForTest(9952), "a hidden emote reaches no one")
			require.Empty(t, events.DrainQueuedRoomMessagesForTest(9950), "nothing is queued to the room")
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/actions/ -run TestSendSeen_HiddenActorReachesNoOne`
Expected, observed on master:
```
    --- FAIL: TestSendSeen_HiddenActorReachesNoOne/hidden_player (0.00s)
            	Error:      	Should be empty, but was [Kesh waves.]
            	Messages:   	clear should read nothing
    --- FAIL: TestSendSeen_HiddenActorReachesNoOne/hidden_mob (0.00s)
            	Error:      	Should be empty, but was [Grel pays you little mind.]
```

Run: `go test ./internal/usercommands/ -run TestEmote_HiddenPlayerIsToldNoOneSees`
Expected: FAIL in all four subtests; the dry run (taken after Step 3's `SendSeen` guard, so only
the note was missing) showed e.g.
```
    --- FAIL: TestEmote_HiddenPlayerIsToldNoOneSees/free_text (0.00s)
            	expected: []string{"You Emote: Hider waves.", "No one sees it; you are hidden."}
            	actual  : []string{"You Emote: Hider waves."}
    --- FAIL: TestEmote_HiddenPlayerIsToldNoOneSees/at_form (0.00s)
            	expected: []string{"No one sees it; you are hidden."}
            	actual  : []string(nil)
```
On untouched master the watcher assertion fails as well.

- [ ] **Step 3: Silence a hidden actor in `SendSeen`**

In `internal/actions/room_lines.go`, `SendSeen` (:32-37): extend the doc comment and add the
guard after the nil-room check, so the function reads:

```go
// (owner ruling 6).
//
// A hidden actor's emote reaches no one (#274, owner R3, 2026-10-08): an
// emote would give its hider away, so it is silence rather than an anonymous
// line. The player emote command tells its own hider why.
func SendSeen(actor Actor, cat messaging.Category, text string, chatter bool) {
	room := actor.GetRoom()
	if room == nil {
		return
	}
	if c := actor.GetCharacter(); c != nil && c.IsHidden() {
		return
	}
```

(The rest of the body is unchanged. `SendHeard` is NOT changed: a rally or warcry is a sound,
heard whoever makes it. Speech keeps `sendSpoken`, which already handles a still-hidden speaker.)

Run: `go test ./internal/actions/ -run TestSendSeen`
Expected (observed): `ok`.

- [ ] **Step 4: Tell a hidden player why**

In `internal/usercommands/emote.go`, call `hiddenEmoteNote(user)` immediately after each of the
three `actions.SendSeen(...)` calls (empty form :26-30, before `return true, nil`; alias :38,
before `events.AddToQueue`; free text :61-64, before `events.AddToQueue`), and add at the end of
the file:

```go
// hiddenEmoteNote tells a hidden player that their emote reached no one.
// actions.SendSeen sends nothing for a hidden actor (#274, owner R3,
// 2026-10-08), and without this line the silence would be a mystery.
func hiddenEmoteNote(user *users.UserRecord) {
	if user.Character.IsHidden() {
		user.SendText(messaging.CategoryEmote, `No one sees it; you are hidden.`)
	}
}
```

(`events.Emote` is still queued for a hidden player; its only listener is the AI companion
module (`modules/aicompanion/aicompanion.go:242`), the hider's own ally. Unchanged on purpose.)

- [ ] **Step 5: Update `internal/actions/context.md`**

In the `SendSeen` bullet (:601-606), after `` `Room.SendTextVisualHidingNames` unfiltered (owner ruling 6).``
append:

```
  A hidden
  actor's emote reaches no one (#274, owner R3, 2026-10-08): silence, not an
  anonymous line. The player `emote` command tells its own hider so.
```

- [ ] **Step 6: Run tests to verify they pass, and run the gate**

Run: `go test ./internal/usercommands/ -run TestEmote_HiddenPlayerIsToldNoOneSees`
Expected (observed): `ok`.

Null probe: delete the `if c := actor.GetCharacter(); ...` guard, run
`go test ./internal/actions/ -run TestSendSeen_HiddenActorReachesNoOne`, confirm the two
subtests fail with `clear should read nothing`; restore.

Gate (observed all `ok` in the dry run):
```
gofmt -l internal/ sight_penalty_guard_test.go
go vet ./internal/actions/ ./internal/usercommands/ ./internal/configs/ ./internal/mobcommands/
go test . ./internal/actions/ ./internal/usercommands/ ./internal/configs/ ./internal/mobcommands/ ./internal/behaviortree/ ./internal/hooks/
```
(`gofmt -l` prints nothing. Observed timings: root 19s, actions 100s, usercommands 2.4s.)

- [ ] **Step 7: Commit**

```bash
git add internal/actions/room_lines.go internal/actions/send_seen_hidden_test.go internal/actions/context.md internal/usercommands/emote.go internal/usercommands/emote_hidden_test.go
git commit -m "fix(sight): a hidden actor does not emote (#274)

Owner ruling R3, 2026-10-08: SendSeen sends nothing for a hidden actor,
so a hidden mob's greet no longer gives it away. The spec applies the
ruling to players too: a hidden player's emote reaches no one and they
read \"No one sees it; you are hidden.\" after their echo.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task B9: Mob sayto, sayto-only and replyto follow each listener's sight

Found by the group D dry run. A mob's addressed speech (`sayto`, `replyto`) sends its room
line on plain `room.SendText`, naming both the mob and the addressee to listeners who see
only shapes or nothing; the addressee's own line ("Skeleton says to you") names the mob to a
blinded addressee. Same treatment NPC `say` already gets (`actions.sendSpoken`, 5b ruling 3):
the words always arrive, names follow sight, and NPC speech is never deafen-filtered (ruling
6). The hidden-mob branches already say "someone" and are unchanged.

**Files:**
- Modify: `internal/mobcommands/sayto.go` (imports :3-13; sites :50-51, :68, :107, :147-148, :158 on master `f50400bf7`)
- Create: `internal/mobcommands/sayto_sight_test.go`

No guard registers `sayto.go` (`speech_wrapper_guard_test.go` lists say, shout, rally, warcry,
emote only), so no allowlist moves.

- [ ] **Step 1: Write the failing test**

Create `internal/mobcommands/sayto_sight_test.go`. It reuses `mobSpeechRoom` and
`mobSpeechHeard` from `speech_sight_test.go` (Aliceia, user 1, sees clearly; Bobrick, user 2,
is blinded; the speaker is mob 100 "Skeleton"; mob 200 "Merchant" is in the room).

```go
package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// A mob speaking to someone in particular is speech like any other (sight
// gates 5b ruling 3): every listener hears the words, and each name in the
// line, the speaker's and the addressee's, follows that listener's sight.
// mobSpeechRoom blinds Bobrick (user 2); Aliceia (user 1) sees clearly.

func TestMobSayTo_BlindedBystanderHearsNoNames(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)

	_, err := SayTo("alice hello there", mob, room)
	require.NoError(t, err)
	require.Equal(t, []string{`Skeleton says to you, "hello there"`}, mobSpeechHeard(1))
	require.Equal(t, []string{`Someone says to someone, "hello there"`}, mobSpeechHeard(2))
}

func TestMobSayTo_BlindedAddresseeHearsNoSpeakerName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)

	_, err := SayTo("bob hello there", mob, room)
	require.NoError(t, err)
	require.Equal(t, []string{`Someone says to you, "hello there"`}, mobSpeechHeard(2))
	require.Equal(t, []string{`Skeleton says to Bobrick, "hello there"`}, mobSpeechHeard(1))
}

func TestMobSayTo_MobAddresseeHiddenFromTheBlind(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)

	_, err := SayTo("merchant hello there", mob, room)
	require.NoError(t, err)
	require.Equal(t, []string{`Skeleton says to Merchant, "hello there"`}, mobSpeechHeard(1))
	require.Equal(t, []string{`Someone says to someone, "hello there"`}, mobSpeechHeard(2))
}

func TestMobSayToOnly_BlindedAddresseeHearsNoSpeakerName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)

	_, err := SayToOnly("bob a secret", mob, room)
	require.NoError(t, err)
	require.Equal(t, []string{`Someone says to you, "a secret"`}, mobSpeechHeard(2))
	require.Empty(t, mobSpeechHeard(1), "sayto-only reaches the addressee alone")
}

func TestMobReplyTo_BlindedListenersHearNoNames(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)

	_, err := ReplyTo("alice some reply", mob, room)
	require.NoError(t, err)
	require.Equal(t, []string{`Skeleton replies to you, "some reply"`}, mobSpeechHeard(1))
	require.Equal(t, []string{`Someone replies to someone, "some reply"`}, mobSpeechHeard(2))

	_, err = ReplyTo("bob some reply", mob, room)
	require.NoError(t, err)
	require.Equal(t, []string{`Someone replies to you, "some reply"`}, mobSpeechHeard(2))

	_, err = ReplyTo("merchant some reply", mob, room)
	require.NoError(t, err)
	require.Equal(t, []string{`Someone replies to someone, "some reply"`}, mobSpeechHeard(2))
}

// A shapes-only listener reads "a figure" for both parties.
func TestMobSayTo_ShapesBystanderReadsFigures(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)
	bob := users.GetByUserId(2)
	bob.Character.Perception = characters.New().Perception
	room.Lamp = rooms.LampPtr(35)
	require.Equal(t, messaging.SightShapes, messaging.ParticipantSight(bob.Character, room))
	events.DrainQueuedMessagesForTest(2)

	_, err := SayTo("alice hello there", mob, room)
	require.NoError(t, err)
	require.Equal(t, []string{`A figure says to a figure, "hello there"`}, mobSpeechHeard(2))
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/mobcommands/ -run 'TestMobSayTo|TestMobSayToOnly|TestMobReplyTo'`

Expected (observed on `f50400bf7`): all six FAIL, e.g.

```
--- FAIL: TestMobSayTo_BlindedBystanderHearsNoNames
    expected: []string{"Someone says to someone, \"hello there\""}
    actual  : []string{"Skeleton says to Aliceia, \"hello there\""}
--- FAIL: TestMobSayTo_BlindedAddresseeHearsNoSpeakerName
    expected: []string{"Someone says to you, \"hello there\""}
    actual  : []string{"Skeleton says to you, \"hello there\""}
--- FAIL: TestMobSayTo_MobAddresseeHiddenFromTheBlind
    actual  : []string{"Skeleton says to Merchant, \"hello there\""}
--- FAIL: TestMobSayToOnly_BlindedAddresseeHearsNoSpeakerName
    actual  : []string{"Skeleton says to you, \"a secret\""}
--- FAIL: TestMobReplyTo_BlindedListenersHearNoNames
    actual  : []string{"Skeleton replies to Aliceia, \"some reply\""}
--- FAIL: TestMobSayTo_ShapesBystanderReadsFigures
    actual  : []string{"Skeleton says to Aliceia, \"hello there\""}
FAIL	github.com/GoMudEngine/GoMud/internal/mobcommands
```

- [ ] **Step 3: Add the two senders to `internal/mobcommands/sayto.go`**

Add `"github.com/GoMudEngine/GoMud/internal/users"` to the import block (between `rooms` and
`util`), and add right after the import block:

```go
// sayToRoom sends a mob's addressed speech line (sayto, replyto) to everyone
// else in room. Like NPC say (actions.sendSpoken), every listener hears the
// words, and each name in the line, the speaker's and the addressee's, reads
// "a figure" at shapes and "someone" when the listener sees nothing (sight
// gates 5b ruling 3). Authored NPC speech, so never deafen-filtered (ruling 6).
func sayToRoom(room *rooms.Room, line string, names []string, excludeUserIds ...int) {
	room.SendTextHidingNames(messaging.CategorySpeech, line, names, messaging.HideSpeakerNames, excludeUserIds...)
}

// sayToUser sends the addressee its own line, the speaker's name hidden at the
// addressee's sight: a blinded player spoken to hears "Someone says to you".
func sayToUser(toUser *users.UserRecord, room *rooms.Room, line, speaker string) {
	toUser.SendText(messaging.CategorySpeech,
		messaging.HideSpeakerNames(line, []string{speaker}, messaging.ParticipantSight(toUser.Character, room)))
}
```

- [ ] **Step 4: Route the six sends through them**

In `SayTo`, player addressee, not hidden (master :50-51), replace the two lines with:

```go
			sayToUser(toUser, room, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> says to you, "<ansi fg="saytext-mob">%s</ansi>"`, mob.Character.Name, rest), mob.Character.Name)
			sayToRoom(room, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> says to <ansi fg="username">%s</ansi>, "<ansi fg="saytext-mob">%s</ansi>"`, mob.Character.Name, toUser.Character.Name, rest),
				[]string{mob.Character.Name, toUser.Character.Name}, toUser.UserId)
```

In `SayTo`, mob addressee (master :68), replace the `room.SendText(...)` line with:

```go
			sayToRoom(room, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> says to <ansi fg="mobname">%s</ansi>, "<ansi fg="saytext-mob">%s</ansi>"`, mob.Character.Name, toMob.Character.Name, rest),
				[]string{mob.Character.Name, toMob.Character.Name})
```

In `SayToOnly`, the not-hidden branch (master :107), replace the `toUser.SendText(...)` line with:

```go
		sayToUser(toUser, room, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> says to you, "<ansi fg="saytext-mob">%s</ansi>"`, mob.Character.Name, rest), mob.Character.Name)
```

In `ReplyTo`, player addressee, not hidden (master :147-148), replace the two lines with:

```go
			sayToUser(toUser, room, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> replies to you, "<ansi fg="saytext-mob">%s</ansi>"`, mob.Character.Name, rest), mob.Character.Name)
			sayToRoom(room, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> replies to <ansi fg="username">%s</ansi>, "<ansi fg="saytext-mob">%s</ansi>"`, mob.Character.Name, toUser.Character.Name, rest),
				[]string{mob.Character.Name, toUser.Character.Name}, toUser.UserId)
```

In `ReplyTo`, mob addressee (master :158), replace the `room.SendText(...)` line with:

```go
			sayToRoom(room, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> replies to <ansi fg="mobname">%s</ansi>, "<ansi fg="saytext-mob">%s</ansi>"`, mob.Character.Name, toMob.Character.Name, rest),
				[]string{mob.Character.Name, toMob.Character.Name})
```

After this, `grep -n "room.SendText" internal/mobcommands/sayto.go` prints nothing.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `gofmt -l internal/mobcommands/` (expect no output), then
`go build ./...` (expect no output), then
`go test ./internal/mobcommands/ -run 'TestMobSayTo|TestMobSayToOnly|TestMobReplyTo|TestSayTo|TestReplyTo'`

Expected: `ok  	github.com/GoMudEngine/GoMud/internal/mobcommands`

- [ ] **Step 6: Null probe, then restore**

In `sayToUser`, temporarily replace
`messaging.HideSpeakerNames(line, []string{speaker}, messaging.ParticipantSight(toUser.Character, room)))`
with `line)`. Rerun the Step 5 test command.

Expected (observed in the dry run): exactly the three addressee tests go red:

```
--- FAIL: TestMobSayTo_BlindedAddresseeHearsNoSpeakerName
--- FAIL: TestMobSayToOnly_BlindedAddresseeHearsNoSpeakerName
--- FAIL: TestMobReplyTo_BlindedListenersHearNoNames
```

Restore the line and rerun; expect `ok`. (Reverting `sayToRoom` to a plain `room.SendText` is
the master state Step 2 already showed red for the bystander tests.)

- [ ] **Step 7: Package and root gate**

Run: `go vet ./internal/mobcommands/` (expect no output), then
`go test ./internal/mobcommands/ ./internal/actions/ ./internal/rooms/ ./internal/messaging/`
and `go test .` at the repo root.

Expected (observed in the dry run):

```
ok  	github.com/GoMudEngine/GoMud/internal/mobcommands
ok  	github.com/GoMudEngine/GoMud/internal/actions
ok  	github.com/GoMudEngine/GoMud/internal/rooms
ok  	github.com/GoMudEngine/GoMud/internal/messaging
ok  	github.com/GoMudEngine/GoMud
```

- [ ] **Step 8: Commit**

```bash
git add internal/mobcommands/sayto.go internal/mobcommands/sayto_sight_test.go
git commit -m "fix(mobcommands): mob sayto and replyto hide names by each listener's sight (#382)

A mob's addressed speech sent its room line on plain room.SendText, naming
the mob and its addressee to listeners who see only shapes or nothing, and
told a blinded addressee who spoke. Route the room lines through
SendTextHidingNames with HideSpeakerNames, as NPC say does (5b ruling 3),
and hide the speaker in the addressee's own line at the addressee's sight.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

## Section C: mob scan (#251), corpse matching (#435), corpse decay line (#276)

Dry-run on origin/master `f50400bf7`, 2026-10-08. Every test below was seen
failing on unfixed code and passing after the change; full `go build ./...`,
`go vet ./...`, `go test ./...` and root `go test .` green at the end.

**Spec corrections found in the dry run (apply these, not the spec text):**

1. **#276: do NOT add `user-corpse` / `mob-corpse` to `nameTagPattern`.** It
   contradicts the #428 design recorded in `internal/rooms/context.md`
   ("A mob-corpse or user-corpse tag is not an identity tag, so `Anonymize`
   never hides a corpse name; pass the name"). Corpse lines put
   `ObservedName()` ("corpse of <name>") inside the corpse tag, and
   `rooms.deliverVisual` runs `Anonymize` before `HideNames`, so a corpse tag in
   `nameTagPattern` would turn "loots the corpse of Deadric" into "loots the A
   figure" on every existing #428 line (`loot.go:143`, `get.go:431`, `:468`,
   `look.go:470`). The decay line instead joins the #428 shape (Task C4).
2. **#276 decay line goes through `SendTextVisualHidingNames`, not
   `SendTextHidingNames`.** Every other corpse line is visual
   (`loot.go:142`, `look.go:470`); a corpse crumbling is seen, not heard. A
   reader who sees nothing now reads no decay line. Full-sight wording changes
   from "A Deadric corpse crumbles to dust." to "The corpse of Deadric crumbles
   to dust." (the `ObservedName` form every #428 observer line uses).
3. **#435 applies to mob corpses too, not only player corpses.** `Corpse.NameAt`
   hides mob and player corpse names alike below clear sight (#428), so a name
   match on either kind would confirm what the hidden lines withhold.
4. **#251:** no player consumer reads the structured `ScanResult`
   (`grep -rn "\.Sightings"`: only `behaviortree/actions_scout.go` and tests),
   so the structured result follows sight for every actor and the player text
   now renders from it. One computation, not two.
5. `FindCorpse` and `FindCorpseIndex` held byte-identical matching logic;
   `FindCorpse` becomes a three-line wrapper over `FindCorpseIndex`.

**Conflict note for the plan assembler:** group A's #246 edits
`internal/messaging/anonymize.go` (`nameTagPattern`). This section no longer
touches that file (correction 1), so there is no overlap.

---

### Task C1: Mob scan follows sight through one reach rule (#251)

**Files:**
- Create: `internal/actions/scan_mob_sight_test.go`
- Modify: `internal/actions/scan.go:46-50` (doc), `:83-108` (structured loop), `:129-168` (text loop), insert `scanReach` above `listedOccupants` (`:201`)
- Modify: `internal/actions/context.md:506-508`, `:950-956`

- [ ] **Step 1: Write the failing test**

Create `internal/actions/scan_mob_sight_test.go`:

```go
package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// #251: the structured ScanResult, which scout behaviour trees act on, follows
// the scanner's sight by the same rule the player's scan text uses
// (scanReach, then listedOccupants). A mob scanning out of a dark room, or
// into one, sees nobody; a hidden player is never a sighting. Under lighting
// 5d ruling D8 a mob acts on a shape, so a dim next room still counts.
const (
	scanMobHereId  = 9530
	scanMobThereId = 9531
	scanMobScoutId = 9532
	scanMobSelfId  = 9533
)

func scanMobSightings(t *testing.T, hereLamp, thereLamp int) ScanResult {
	t.Helper()
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", SkyLight: rooms.SkyLightPtr(0.0)},
	}))
	here := &rooms.Room{RoomId: scanMobHereId, Zone: "ScanMob", Biome: "cave", Lamp: rooms.LampPtr(hereLamp),
		Exits: map[string]exit.RoomExit{"north": {RoomId: scanMobThereId}}}
	there := &rooms.Room{RoomId: scanMobThereId, Zone: "ScanMob", Biome: "cave", Lamp: rooms.LampPtr(thereLamp),
		Exits: map[string]exit.RoomExit{"south": {RoomId: scanMobHereId}}}
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{scanMobHereId: here, scanMobThereId: there},
		map[string]*rooms.ZoneConfig{"ScanMob": {Name: "ScanMob", RoomId: scanMobHereId,
			RoomIds: map[int]struct{}{scanMobHereId: {}, scanMobThereId: {}}}},
	))
	scout := newScanTestMob(scanMobScoutId, "Midroad Scout", scanMobThereId)
	mobs.SetInstanceForTest(scanMobScoutId, scout)
	t.Cleanup(func() { mobs.SetInstanceForTest(scanMobScoutId, nil) })
	there.AddMob(scanMobScoutId)

	actor := newScanMobActor("Scanner", here, scanMobSelfId)
	return Scan(actor, ScanOptions{HostileOnly: true})
}

func TestScan_MobSightingsFollowTheScannersSight(t *testing.T) {
	for _, c := range []struct {
		name        string
		here, there int
		wantMobs    int
	}{
		{"both rooms bright: a sighting", 90, 90, 1},
		{"next room dim: a shape still counts (D8)", 90, 35, 1},
		{"next room dark: nobody", 90, -50, 0},
		{"own room too dark to see out: nobody", 10, 90, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			result := scanMobSightings(t, c.here, c.there)
			require.Len(t, result.Sightings, 1, "the exit is still listed")
			require.Len(t, result.Sightings[0].Mobs, c.wantMobs)
			require.Empty(t, result.Sightings[0].Players)
		})
	}
}

// The structured result lists whom the roster would list: no hidden player,
// no hidden mob, no stale listing. Fixture from scanOccupantScene: north holds
// the Midroad Scout, a hidden mob, a stale mob listing and the hidden Kesh.
func TestScan_MobSightingsSkipHiddenAndStale(t *testing.T) {
	actor, _ := scanOccupantScene(t, 90, 90)
	actor.isPlayer, actor.userId, actor.mobInstId = false, 0, scanMobSelfId
	actor.char.Conditions = conditions.New() // no infra, no see-hidden

	result := Scan(actor, ScanOptions{HostileOnly: true})
	require.Len(t, result.Sightings, 1)
	require.Empty(t, result.Sightings[0].Players, "a hidden player is never a scout's sighting")
	require.Len(t, result.Sightings[0].Mobs, 1)
	require.Equal(t, "Midroad Scout", result.Sightings[0].Mobs[0].Name)
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/actions/ -run 'TestScan_MobSightings' -count=1`
Expected (observed in dry run): FAIL with
`next_room_dark:_nobody ... "[{9532 Midroad Scout true}]" should have 0 item(s), but has 1`,
the same for `own_room_too_dark_to_see_out:_nobody`, and
`TestScan_MobSightingsSkipHiddenAndStale ... Should be empty, but was [{9483 Kesh false}]`.

- [ ] **Step 3: Add `scanReach`**

In `internal/actions/scan.go`, insert directly above the `// listedOccupants is every creature in room that the viewer's room roster` comment:

```go
// scanReach is how well a scanner standing in here makes out the room next
// through an exit, and whether anything reaches through at all. Light first:
// when the scanner sees out of here (messaging.SeesThroughExit), the answer is
// their sight in next. When the light here refuses, heat may still show next's
// occupants as shapes (lighting plan 6, owner ruling O6); it never upgrades a
// view the light grants. Neither: SightNone, nothing reaches. A nil next room
// reaches nothing. It is the one rule behind both halves of Scan: the player's
// text and the structured sightings a scout mob acts on (#251).
func scanReach(viewer *characters.Character, here, next *rooms.Room) (sight messaging.SightDecision, reaches bool) {
	if next == nil {
		return messaging.SightNone, false
	}
	if messaging.SeesThroughExit(viewer, here) {
		return messaging.ParticipantSight(viewer, next), true
	}
	if messaging.SensesHeatThroughExit(viewer, here, next) {
		return messaging.SightShapes, true
	}
	return messaging.SightNone, false
}
```

- [ ] **Step 4: Fill the structured sighting from the roster at the scanner's reach**

In `Scan`, replace the two occupant loops (the `for _, mobInstId := range adjRoom.GetMobs(rooms.FindAll) {` block and the `for _, pId := range adjRoom.GetPlayers(rooms.FindAll) {` block, master lines 83-108) with:

```go
		// Whom the scanner makes out there follows the scanner's sight, by
		// the rule the player's text below uses (#251): nobody when neither
		// light nor heat reaches, and only those the roster would list
		// (listedOccupants), so a hidden player is never a scout's sighting.
		// A shape counts: a mob acts on a figure it can make out (lighting 5d
		// ruling D8), and the player text, not this list, hides the names.
		if sight, _ := scanReach(actor.GetCharacter(), room, adjRoom); sight != messaging.SightNone {
			listedMobs, listedPlayers := listedOccupants(actor.GetCharacter(), adjRoom, actor.GetUserId())
			for _, m := range listedMobs {
				sighting.Mobs = append(sighting.Mobs, ScanEntity{
					Id:    m.InstanceId,
					Name:  m.Character.Name,
					IsMob: true,
				})
			}
			for _, u := range listedPlayers {
				sighting.Players = append(sighting.Players, ScanEntity{
					Id:    u.UserId,
					Name:  u.Character.Name,
					IsMob: false,
				})
			}
		}
```

(keep the following `result.Sightings = append(result.Sightings, sighting)` line).

- [ ] **Step 5: Render the player text from the structured sighting**

In the `if actor.IsPlayer() {` block, replace from the comment line
`// (listedOccupants), the same one look's heat uses. The` through the end of
the `for _, u := range listedPlayers {` parts loop with:

```go
			// (listedOccupants), the same one look's heat uses. The
			// structured result already holds exactly those occupants, by
			// the same scanReach (#251), so the text reads it.
			//
			// The next room's title shows only when the light here sees out
			// (#428). When it refuses, by heat-only figures or nothing at
			// all, the line names the direction alone, as look <exit> names
			// no room under heat.
			//
			// A locked exit (#427) reads as look reads it: locked whenever
			// light or heat would reach through it, with nobody listed; too
			// dark when neither would.
			sight, reaches := scanReach(viewer, room, rooms.LoadRoom(s.RoomId))
			dirLabel := fmt.Sprintf(`<ansi fg="exit">%s</ansi>`, s.ExitName)
			if s.Locked && reaches {
				actor.SendText(messaging.CategorySystem,
					fmt.Sprintf(`  %s: the exit is locked`, dirLabel))
				continue
			}
			parts := []string{}
			for _, m := range s.Mobs {
				parts = append(parts,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi>`, m.Name))
			}
			for _, u := range s.Players {
				parts = append(parts,
					fmt.Sprintf(`<ansi fg="username">%s</ansi>`, u.Name))
			}
```

Keep `viewer := actor.GetCharacter()` and `seesOut := messaging.SeesThroughExit(viewer, room)` above the loop (`seesOut` still picks the label). Keep the `switch sight {` figure/none block and everything after it unchanged.

Update the `Scan` doc comment's first sentence to:

```go
// Scan walks each visible (non-secret) exit from the actor's room, loads
// the adjacent room, and lists the mobs and players the actor makes out in
// each (scanReach, then listedOccupants: nobody through darkness, never a
// hidden creature the actor does not perceive).
```

- [ ] **Step 6: Run the tests**

Run: `go build ./... && go test ./internal/actions/ -run TestScan -count=1`
Expected: `ok  github.com/GoMudEngine/GoMud/internal/actions`
Then: `go test ./internal/actions/ ./internal/behaviortree/ ./internal/mobcommands/ ./internal/usercommands/ -count=1`
Expected (dry run): all four `ok`.

- [ ] **Step 7: Update `internal/actions/context.md`**

Replace

```
stale listing never. Scan's player-facing name list uses it too; its
structured `ScanResult` (for mob callers) still lists every mob not
`IsHidden`. Sight
```

with

```
stale listing never. Scan's structured `ScanResult` (for mob callers) and
its player-facing list both read it, behind one reach rule, `scanReach`
(#251). Sight
```

Replace the `**Structured result:**` bullet in the `### Scan` section with:

```
- **Structured result (#251):** follows the scanner's sight.
  `scanReach(viewer, here, next)` is the one reach rule: the scanner's
  `ParticipantSight` in the next room when they see out of this one
  (`SeesThroughExit`), shapes by heat when only `SensesHeatThroughExit`
  reaches, else `SightNone`. At anything but `SightNone`, `Mobs` and
  `Players` are `listedOccupants` (the roster's rule: no hidden creature the
  scanner does not `Perceives`, no stale listing). A shape counts, so a scout
  mob acts on a figure (lighting 5d ruling D8); names stay in the struct and
  the player text hides them below clear sight.
```

and in the next bullet change "one line per exit, following the player's sight (see the look section above)" to "one line per exit, rendered from the structured sighting at the same `scanReach` (see the look section above)".

- [ ] **Step 8: Commit**

```bash
git add internal/actions/scan.go internal/actions/scan_mob_sight_test.go internal/actions/context.md
git commit -m "fix(scan): mob sightings follow the scanner's sight (#251)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task C2: Corpse lookups take the viewer (signature only, #435)

No behaviour change in this task: the parameter is threaded through and
unused, so the compiler enumerates every caller (dogmud-refactoring).

**Files:**
- Modify: `internal/rooms/rooms.go:1502-1613` (`FindCorpse`, `FindCorpseIndex`)
- Modify: `internal/parser/adapters.go:40-48`
- Modify: `internal/usercommands/assess.go:27`, `get.go:183`, `get.go:749`, `look.go:444`, `loot.go:47`, `loot.go:161`, `salvage.go:40`
- Modify tests: `internal/rooms/corpse_ordering_test.go`, `internal/rooms/rooms_test.go`

- [ ] **Step 1: Change the signatures**

In `internal/rooms/rooms.go`, replace the whole `func (r *Room) FindCorpse(searchName string) (Corpse, bool) {` function and the four-line comment plus signature line of `FindCorpseIndex` that follow it with:

```go
// FindCorpse is FindCorpseIndex returning a value copy of the corpse. A caller
// that changes the corpse (loot, ownership) must use FindCorpseIndex and work
// on r.Corpses[idx], since a copy silently drops the mutation.
func (r *Room) FindCorpse(searchName string, viewer *characters.Character) (Corpse, bool) {
	idx := r.FindCorpseIndex(searchName, viewer)
	if idx < 0 {
		return Corpse{}, false
	}
	return r.Corpses[idx], true
}

// FindCorpseIndex returns the slice index of the non-prunable corpse
// searchName names (or -1), newest first. Callers mutate r.Corpses[idx] in
// place via a pointer.
func (r *Room) FindCorpseIndex(searchName string, viewer *characters.Character) int {
```

Leave the body of `FindCorpseIndex` unchanged. (The deleted `FindCorpse` body was a line-for-line copy of `FindCorpseIndex`'s matching, returning by value.) `rooms` already imports `characters` and `messaging`.

- [ ] **Step 2: Let the compiler list the callers**

Run: `go build ./...`
Expected (dry run): `internal\parser\adapters.go:44:32: not enough arguments in call to s.Room.FindCorpseIndex`, then after fixing it, seven errors in `internal\usercommands` (`assess.go:27`, `get.go:183`, `get.go:749`, `look.go:444`, `loot.go:47`, `loot.go:161`, `salvage.go:40`).

- [ ] **Step 3: Fix the parser caller**

In `internal/parser/adapters.go` `corpseAdapter`, replace `idx := s.Room.FindCorpseIndex(candidate)` with:

```go
	// The corpse is matched at the looker's sight (#435): below clear sight
	// only the word "corpse" reaches one.
	var viewer *characters.Character
	if s.User != nil {
		viewer = s.User.Character
	}
	idx := s.Room.FindCorpseIndex(candidate, viewer)
```

(`characters` is already imported there.)

- [ ] **Step 4: Fix the seven usercommands callers**

Each passes `user.Character`:

- `assess.go:27`: `corpse, found := room.FindCorpse(rest, user.Character)`
- `get.go:183`: `if cIdx := room.FindCorpseIndex(args[len(args)-1], user.Character); cIdx >= 0 {`
- `get.go:749`: `if _, corpseFound := room.FindCorpse(rest, user.Character); corpseFound {`
- `look.go:444`: `if corpse, corpseFound := room.FindCorpse(rest, user.Character); corpseFound {`
- `loot.go:47`: `corpseIdx := room.FindCorpseIndex(rest, user.Character)`
- `loot.go:161`: `corpseIdx := room.FindCorpseIndex(rest, user.Character)`
- `salvage.go:40`: `corpse, corpseFound := room.FindCorpse(rest, user.Character)`

- [ ] **Step 5: Pass a nil viewer in the existing rooms tests**

In `internal/rooms/corpse_ordering_test.go` (lines 18, 26, 41) and `internal/rooms/rooms_test.go` (lines 1068, 1073, 1078, 1083, 1833, 2187, 2191, 2195), add `, nil` as the second argument to every `FindCorpse("...")` / `FindCorpseIndex("...")` call, e.g. `r.FindCorpse("goblin corpse", nil)`.

- [ ] **Step 6: Build, vet and test**

Run: `go build ./... && go vet ./... && go test ./internal/rooms/ ./internal/parser/ ./internal/usercommands/ -count=1`
Expected: build and vet silent; three `ok` lines.

- [ ] **Step 7: Commit**

```bash
git add internal/rooms/rooms.go internal/rooms/corpse_ordering_test.go internal/rooms/rooms_test.go internal/parser/adapters.go internal/usercommands/assess.go internal/usercommands/get.go internal/usercommands/look.go internal/usercommands/loot.go internal/usercommands/salvage.go
git commit -m "refactor(rooms): corpse lookups take the viewer; FindCorpse wraps FindCorpseIndex (#435)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task C3: Below clear sight only the word "corpse" matches (#435)

**Files:**
- Create: `internal/rooms/corpse_sight_match_test.go`
- Modify: `internal/rooms/rooms.go` (`FindCorpseIndex`, top of body)
- Modify: `internal/parser/adapters_test.go` (new test, `messaging` import)
- Modify: `internal/usercommands/assess_disclosure_test.go:56`, `internal/usercommands/look_corpse_observer_hiding_test.go:42`
- Modify comments: `internal/usercommands/look.go:459-461`, `internal/usercommands/assess.go:71-72`
- Modify: `internal/rooms/context.md:95-96`

- [ ] **Step 1: Write the failing rooms test**

Create `internal/rooms/corpse_sight_match_test.go`:

```go
package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// #435: every corpse line hides the dead one's name below clear sight (#428),
// so name matching must too. A viewer who cannot see clearly reaches a corpse
// by the word "corpse" alone, newest first and countable ("2.corpse"); a name
// that would confirm whose it is matches nothing. Clear sight matches by name
// as before. Fixture (detailsCorpseRoom): City Beggar, City Beggar, Deadric,
// oldest to newest, in a dark cave.
func TestFindCorpse_BelowClearSightMatchesOnlyTheWordCorpse(t *testing.T) {
	for _, c := range []struct {
		name  string
		infra bool
		want  messaging.SightDecision
	}{
		{"sees nothing", false, messaging.SightNone},
		{"shapes only", true, messaging.SightShapes},
	} {
		t.Run(c.name, func(t *testing.T) {
			r, viewer := detailsCorpseRoom(t)
			if c.infra && !viewer.Character.Conditions.AddCondition(sightTestInfraredConditionId, true) {
				t.Fatal("precondition: the viewer should now carry infrared")
			}
			if got := messaging.ParticipantSight(viewer.Character, r); got != c.want {
				t.Fatalf("precondition: viewer sight = %v, want %v", got, c.want)
			}
			for _, named := range []string{"deadric corpse", "deadric", "city beggar corpse", "beggar"} {
				if idx := r.FindCorpseIndex(named, viewer.Character); idx != -1 {
					t.Errorf("FindCorpseIndex(%q) = %d at %v, want -1: the name confirms whose corpse it is", named, idx, c.want)
				}
				if _, ok := r.FindCorpse(named, viewer.Character); ok {
					t.Errorf("FindCorpse(%q) matched at %v; the name confirms whose corpse it is", named, c.want)
				}
			}
			if idx := r.FindCorpseIndex("corpse", viewer.Character); idx != 2 {
				t.Errorf(`FindCorpseIndex("corpse") = %d, want 2 (the newest)`, idx)
			}
			if idx := r.FindCorpseIndex("2.corpse", viewer.Character); idx != 1 {
				t.Errorf(`FindCorpseIndex("2.corpse") = %d, want 1 (the second newest)`, idx)
			}
			if got, ok := r.FindCorpse("corpse", viewer.Character); !ok || got.Character.Name != "Deadric" {
				t.Errorf(`FindCorpse("corpse") = %q, %v; want the newest, Deadric`, got.Character.Name, ok)
			}
		})
	}
}

func TestFindCorpse_ClearSightMatchesByName(t *testing.T) {
	r, viewer := detailsCorpseRoom(t)
	lamp := 90
	r.Lamp = &lamp
	if got := messaging.ParticipantSight(viewer.Character, r); got != messaging.SightFull {
		t.Fatalf("precondition: viewer sight = %v, want SightFull", got)
	}
	if idx := r.FindCorpseIndex("deadric corpse", viewer.Character); idx != 2 {
		t.Errorf(`FindCorpseIndex("deadric corpse") = %d, want 2`, idx)
	}
	if idx := r.FindCorpseIndex("beggar", viewer.Character); idx != 1 {
		t.Errorf(`FindCorpseIndex("beggar") = %d, want 1 (the newest beggar)`, idx)
	}
	if got, ok := r.FindCorpse("city beggar corpse", viewer.Character); !ok || got.Character.Name != "City Beggar" {
		t.Errorf(`FindCorpse("city beggar corpse") = %q, %v`, got.Character.Name, ok)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/rooms/ -run TestFindCorpse -count=1`
Expected (dry run): FAIL in both subtests of `TestFindCorpse_BelowClearSightMatchesOnlyTheWordCorpse`, e.g.
`FindCorpseIndex("deadric corpse") = 2 at 2, want -1: the name confirms whose corpse it is` and
`FindCorpseIndex("2.corpse") = -1, want 1 (the second newest)` (today's name dedup makes `2.corpse` unreachable). `TestFindCorpse_ClearSightMatchesByName` passes already.

- [ ] **Step 3: Gate the match on the viewer's sight**

In `internal/rooms/rooms.go`, replace the `FindCorpseIndex` doc comment and signature from Task C2 with this, keeping the existing name-matching body below the new block:

```go
// FindCorpseIndex returns the slice index of the non-prunable corpse
// searchName names (or -1), newest first. Callers mutate r.Corpses[idx] in
// place via a pointer.
//
// The match follows viewer's sight in this room (#435). Every corpse line
// hides the dead one's name below clear sight (Corpse.NameAt, #428), so a
// name that matched would confirm whose corpse it is: below SightFull each
// corpse, mob or player, answers only to the word "corpse", newest first and
// countable ("2.corpse"). A nil viewer (a system lookup, a test) matches by
// name as clear sight does.
func (r *Room) FindCorpseIndex(searchName string, viewer *characters.Character) int {

	if viewer != nil && messaging.ParticipantSight(viewer, r) != messaging.SightFull {
		candidates := []string{}
		indexes := []int{}
		for idx := len(r.Corpses) - 1; idx >= 0; idx-- {
			if r.Corpses[idx].Prunable {
				continue
			}
			candidates = append(candidates, `corpse`)
			indexes = append(indexes, idx)
		}
		match, closeMatch := util.FindMatchIndexIn(searchName, candidates...)
		if match < 0 {
			match = closeMatch
		}
		if match < 0 {
			return -1
		}
		return indexes[match]
	}
```

- [ ] **Step 4: Run it to verify it passes, then prove it can fail**

Run: `go test ./internal/rooms/ -run 'TestFindCorpse|TestRoom_FindCorpse|TestGetDetails' -count=1`
Expected: `ok  github.com/GoMudEngine/GoMud/internal/rooms`

Null probe: temporarily change `!= messaging.SightFull` to `== messaging.SightNone` in the new block and rerun `-run TestFindCorpse_Below`. Expected (dry run): `--- FAIL: TestFindCorpse_BelowClearSightMatchesOnlyTheWordCorpse/shapes_only`. Restore and rerun: `ok`.

- [ ] **Step 5: Parser test**

In `internal/parser/adapters_test.go` add `"github.com/GoMudEngine/GoMud/internal/messaging"` to the imports (after `items`) and insert before `func TestFloorItemAdapter`:

```go
// #435: the parser's corpse lookup matches at the looker's sight. In the
// dark only the word "corpse" reaches the remains; by name, only in light.
func TestCorpseAdapter_FollowsTheLookersSight(t *testing.T) {
	s, cleanup := seedParserTest(t)
	defer cleanup()
	s.Room.Corpses = []rooms.Corpse{{
		MobId:     1,
		Character: characters.Character{Name: "Skeleton"},
	}}

	s.Room.Lamp = rooms.LampPtr(-50)
	require.NotEqual(t, messaging.SightFull, messaging.ParticipantSight(s.User.Character, s.Room), "precondition: dark")
	_, ok := corpseAdapter(s, "skeleton corpse")
	assert.False(t, ok, "in the dark the name must not confirm whose corpse it is")
	m, ok := corpseAdapter(s, "corpse")
	require.True(t, ok, "the word corpse still reaches it")
	assert.Equal(t, 0, m.CorpseIdx)

	s.Room.Lamp = rooms.LampPtr(90)
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(s.User.Character, s.Room), "precondition: lit")
	_, ok = corpseAdapter(s, "skeleton corpse")
	assert.True(t, ok, "in light the name matches")
}
```

(The parser fixture's user reads shapes, not none, at lamp -50, hence `NotEqual SightFull`.)

Run: `go test ./internal/parser/ -count=1` → `ok`.
Null probe: in `corpseAdapter` pass `nil` instead of `viewer` (add `_ = viewer` so it compiles) and rerun `-run TestCorpseAdapter_Follows`. Expected (dry run): FAIL `in the dark the name must not confirm whose corpse it is`. Restore.

- [ ] **Step 6: Fix the two usercommands fixtures the gate now rightly changes**

Run: `go test ./internal/usercommands/ -count=1`
Expected before this step (dry run): FAIL in `TestAssess_DisclosesReservationBand`, `TestAssess_FlagsUnaffordableReservation`, `TestAssess_PlayerCorpseAdvertisesNoRaisableForms` (their room is unlit and they assess "goblin" by name) and `TestLookCorpse_ShapesOnlyObserverDoesNotReadTheDeadPlayersName` (a shapes looker typed "deadric corpse").

`internal/usercommands/assess_disclosure_test.go`, replace `room := &rooms.Room{RoomId: 999901, Corpses: []rooms.Corpse{corpse}}` with:

```go
	// Lit, so the assessor sees clearly and reaches the corpse by name: below
	// clear sight only the word "corpse" matches one (#435).
	room := &rooms.Room{RoomId: 999901, Lamp: rooms.LampPtr(90), Corpses: []rooms.Corpse{corpse}}
```

`internal/usercommands/look_corpse_observer_hiding_test.go`, replace `handled, err := Look("deadric corpse", looker, room, 0)` with:

```go
	// A shapes-only looker reaches the corpse by the word "corpse" alone; its
	// name would confirm whose it is (#435).
	handled, err := Look("corpse", looker, room, 0)
```

Run: `go test ./internal/usercommands/ -count=1` → `ok`.

- [ ] **Step 7: Correct the two comments that described the old matching**

`internal/usercommands/look.go`, in the self-line comment, replace

```go
			// FindCorpse matches the word "corpse" by substring, so a
			// shapes-only looker reaches any corpse and must not be told whose
			// it is, nor shown a description that would say.
```

with

```go
			// below clear sight FindCorpse matches only the word "corpse"
			// (#435), so a shapes-only looker reaches a corpse without naming
			// it and must not be told whose it is, nor shown a description
			// that would say.
```

`internal/usercommands/assess.go`, replace

```go
	// The remains are named only at clear sight (#428 review): FindCorpse
	// matches the word "corpse", so a shapes-only player reaches any corpse.
```

with

```go
	// The remains are named only at clear sight (#428 review): below it
	// FindCorpse matches only the word "corpse" (#435), so a shapes-only
	// player reaches a corpse without learning whose it is.
```

- [ ] **Step 8: Document it in `internal/rooms/context.md`**

After the bullet ending "tag, so `Anonymize` never hides a corpse name; pass the name." add:

```
- **Corpse matching by sight (#435):** `FindCorpseIndex(searchName, viewer)`
  and its value-copy twin `FindCorpse(searchName, viewer)` match at the
  viewer's `ParticipantSight` in the room. At clear sight (or a nil viewer, a
  system lookup) by name, newest first, as before. Below it every corpse, mob
  or player, answers only to the word "corpse", newest first and countable
  (`2.corpse`), so typing a name cannot confirm whose corpse it is.
```

- [ ] **Step 9: Gate and commit**

Run: `go build ./... && go vet ./... && go test ./internal/rooms/ ./internal/parser/ ./internal/usercommands/ -count=1 && go test . -count=1`
Expected: four `ok` lines.

```bash
git add internal/rooms/rooms.go internal/rooms/corpse_sight_match_test.go internal/rooms/context.md internal/parser/adapters_test.go internal/usercommands/assess.go internal/usercommands/assess_disclosure_test.go internal/usercommands/look.go internal/usercommands/look_corpse_observer_hiding_test.go
git commit -m "fix(rooms): below clear sight a corpse answers only to the word corpse (#435)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task C4: The decay line follows each reader's sight (#276)

**Files:**
- Create: `internal/rooms/corpse_decay_sight_test.go`
- Modify: `internal/rooms/rooms.go:204-209` (inside `UpdateCorpses`)
- Modify: `internal/rooms/context.md` (bullet after the C3 one)

- [ ] **Step 1: Write the failing test**

Create `internal/rooms/corpse_decay_sight_test.go`:

```go
package rooms

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// #276: the decay line named the dead one to the whole room on the audio
// channel, whatever each reader could see. It now goes out like every other
// corpse line (#428): ObservedName through SendTextVisualHidingNames with the
// dead one's name hidden. Fixture (sightTestRoom): an unlit cave holding
// Aliceia (7411), Bobrick (7412) and Ordel (7413).
func decayOneCorpse(t *testing.T, r *Room, c Corpse) {
	t.Helper()
	if err := configs.AddOverlayOverrides(map[string]any{"GamePlay.Death.CorpsesEnabled": true}); err != nil {
		t.Fatalf("enable corpses: %v", err)
	}
	t.Cleanup(func() {
		configs.AddOverlayOverrides(map[string]any{"GamePlay.Death.CorpsesEnabled": false})
	})
	c.RoundCreated = 1
	c.Prunable = true
	r.Corpses = []Corpse{c}
	r.UpdateCorpses(999999999)
	if len(r.Corpses) != 0 {
		t.Fatalf("precondition: the corpse should have decayed, %d remain", len(r.Corpses))
	}
}

func TestUpdateCorpses_DecayLineFollowsEachReadersSight(t *testing.T) {
	for _, c := range []struct {
		name   string
		corpse Corpse
		clear  string
	}{
		{"player corpse", Corpse{UserId: 777}, "The corpse of Deadric crumbles to dust."},
		{"mob corpse", Corpse{MobId: 12}, "The corpse of Deadric crumbles to dust."},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := sightTestRoom(t, "cave")
			if !users.GetByUserId(7413).Character.Conditions.AddCondition(sightTestInfraredConditionId, true) {
				t.Fatal("precondition: Ordel should now carry infrared")
			}
			c.corpse.Character.Name = "Deadric"
			decayOneCorpse(t, r, c.corpse)

			shapes := sightTestPlain(events.DrainQueuedMessagesForTest(7413))
			if len(shapes) != 1 || shapes[0] != "The corpse of a figure crumbles to dust." {
				t.Errorf("shapes reader read %q, want exactly the hidden decay line", shapes)
			}
			if blind := events.DrainQueuedMessagesForTest(7412); len(blind) != 0 {
				t.Errorf("a reader who sees nothing read %q", blind)
			}

			lit := sightTestRoom(t, "cave")
			lamp := 90
			lit.Lamp = &lamp
			decayOneCorpse(t, lit, c.corpse)
			clear := sightTestPlain(events.DrainQueuedMessagesForTest(7411))
			if len(clear) != 1 || clear[0] != c.clear {
				t.Errorf("clear reader read %q, want %q", clear, c.clear)
			}
			if strings.Contains(strings.Join(shapes, " "), "Deadric") {
				t.Errorf("the shapes reader learned the name: %q", shapes)
			}
		})
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/rooms/ -run TestUpdateCorpses_DecayLine -count=1`
Expected (dry run), for both subtests:
`shapes reader read ["A Deadric corpse crumbles to dust."], want exactly the hidden decay line`,
`a reader who sees nothing read ["<ansi fg=\"room-description\">A <ansi fg=\"user-corpse\">Deadric corpse</ansi> crumbles to dust.</ansi>\n"]`,
`clear reader read ["A Deadric corpse crumbles to dust."], want "The corpse of Deadric crumbles to dust."`.

- [ ] **Step 3: Send the decay line the #428 way**

In `UpdateCorpses`, replace

```go
			if corpse.MobId > 0 {
				r.SendText(messaging.CategoryRoomDescription, fmt.Sprintf(`A <ansi fg="mob-corpse">%s</ansi> crumbles to dust.`, corpse.DisplayName()))
			}
			if corpse.UserId > 0 {
				r.SendText(messaging.CategoryRoomDescription, fmt.Sprintf(`A <ansi fg="user-corpse">%s corpse</ansi> crumbles to dust.`, corpse.Character.Name))
			}
```

with

```go
			// Seen, not heard, and named per reader (#276), the shape of
			// every corpse line since #428: ObservedName with the dead one's
			// name hidden, so a shapes reader reads "The corpse of a figure
			// crumbles to dust." and a reader who sees nothing reads nothing.
			// A corpse tag is not an identity tag, so Anonymize alone would
			// leave the name.
			if corpse.MobId > 0 || corpse.UserId > 0 {
				corpseColor := `mob-corpse`
				if corpse.UserId > 0 {
					corpseColor = `user-corpse`
				}
				r.SendTextVisualHidingNames(messaging.CategoryRoomDescription,
					fmt.Sprintf(`The <ansi fg="%s">%s</ansi> crumbles to dust.`, corpseColor, corpse.ObservedName()),
					[]string{corpse.Character.Name})
			}
```

- [ ] **Step 4: Run it to verify it passes**

Run: `go test ./internal/rooms/ -run TestUpdateCorpses -count=1`
Expected: `ok  github.com/GoMudEngine/GoMud/internal/rooms` (includes the existing `TestUpdateCorpses_DropsLootOnDecay`).

- [ ] **Step 5: Document it**

In `internal/rooms/context.md`, after the C3 "Corpse matching by sight" bullet, add:

```
- **Corpse decay line (#276):** "The corpse of <name> crumbles to dust." goes
  out through `SendTextVisualHidingNames` with `ObservedName()` and the dead
  one's name hidden, the same shape as every other corpse line: a shapes
  reader reads "The corpse of a figure crumbles to dust.", a reader who sees
  nothing reads nothing.
```

- [ ] **Step 6: Commit**

```bash
git add internal/rooms/rooms.go internal/rooms/corpse_decay_sight_test.go internal/rooms/context.md
git commit -m "fix(rooms): the corpse decay line hides the dead one's name by sight (#276)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task C5: `loot pass` names the member at the looter's sight (#435)

**Files:**
- Create: `internal/usercommands/loot_pass_sight_test.go`
- Modify: `internal/usercommands/loot.go:233-235` (end of `lootPassCorpse`)

- [ ] **Step 1: Write the failing test**

Create `internal/usercommands/loot_pass_sight_test.go`:

```go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #435: `loot pass` named the party member the share goes to, whatever the
// looter could see. The name now follows the looter's sight, as the corpse's
// own name already did (corpseNameFor). Aliceia (user 1) passes her
// round-robin share to Bobrick (user 2) in room 1.
func lootPassScene(t *testing.T) *rooms.Room {
	t.Helper()
	cleanup := seedAllRegistries()
	t.Cleanup(cleanup)
	restoreCond := seedCraftHidingCondition()
	t.Cleanup(restoreCond)

	p := parties.New(1)
	require.NotNil(t, p)
	t.Cleanup(p.Disband)
	require.True(t, p.InvitePlayer(2))
	require.True(t, p.AcceptInvite(2))

	room := rooms.LoadRoom(1)
	origCorpses := room.Corpses
	t.Cleanup(func() { room.Corpses = origCorpses })
	sword := items.New(10001)
	corpse := rooms.Corpse{
		MobId:           1,
		OwnerUserIds:    []int{1, 2},
		LootMode:        "roundrobin",
		RoundOwnedUntil: 1 << 62,
		RRAssignee:      map[string]int{sword.UUID.String(): 1},
	}
	corpse.Character.Name = "Skeleton"
	corpse.Loot.AddItem(sword)
	room.Corpses = []rooms.Corpse{corpse}
	craftPlainLines(1)
	return room
}

func TestLootPass_ShapesOnlyLooterDoesNotReadTheMembersName(t *testing.T) {
	room := lootPassScene(t)
	darkenCraftRoom(t, 1)
	looter := users.GetByUserId(1)
	require.True(t, looter.Character.Conditions.AddCondition(craftHidingInfraredConditionId, true))

	handled, err := Loot("pass corpse", looter, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	lines := craftPlainLines(1)
	require.Equal(t, []string{"You pass your share of the corpse of a figure to a figure."}, lines,
		"a shapes-only looter must not read whom the share goes to")
}

func TestLootPass_ClearSightNamesTheMember(t *testing.T) {
	room := lootPassScene(t)
	room.Lamp = rooms.LampPtr(90)

	handled, err := Loot("pass corpse", users.GetByUserId(1), room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	require.Equal(t, []string{"You pass your share of the Skeleton corpse to Bobrick."}, craftPlainLines(1))
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/usercommands/ -run TestLootPass -count=1`
Expected (dry run): FAIL
`expected: []string{"You pass your share of the corpse of a figure to a figure."}`
`actual  : []string{"You pass your share of the corpse of a figure to Bobrick."}`.
`TestLootPass_ClearSightNamesTheMember` passes already (it pins the lit wording).

- [ ] **Step 3: Hide the member's name at the looter's sight**

In `lootPassCorpse`, replace

```go
	user.SendText(messaging.CategorySystem,
		fmt.Sprintf(`You pass your share of the %s to <ansi fg="username">%s</ansi>.`, corpseNameFor(user, room, corpse), nextName),
	)
```

with

```go
	// Whom the share goes to follows the looter's sight, as the corpse's
	// name does (#435): a shapes-only looter passes it "to a figure".
	user.SendText(messaging.CategorySystem, messaging.HideNames(
		fmt.Sprintf(`You pass your share of the %s to <ansi fg="username">%s</ansi>.`, corpseNameFor(user, room, corpse), nextName),
		[]string{nextName}, messaging.ParticipantSight(user.Character, room)),
	)
```

- [ ] **Step 4: Run it to verify it passes**

Run: `go test ./internal/usercommands/ -run TestLootPass -count=1`
Expected: `ok  github.com/GoMudEngine/GoMud/internal/usercommands`

- [ ] **Step 5: Section gate**

Run: `go build ./... && go vet ./... && go test ./... -count=1 && go test . -count=1`
Expected (dry run): every package `ok`, root `ok  github.com/GoMudEngine/GoMud`.

- [ ] **Step 6: Commit**

```bash
git add internal/usercommands/loot.go internal/usercommands/loot_pass_sight_test.go
git commit -m "fix(loot): loot pass names the member at the looter's sight (#435)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

**Closes on merge (put in the PR body, not a commit):** #251, #276 (with the
remainder done here; GMCP room contents carry no corpse list,
`grep -n -i corpse modules/gmcp/gmcp.Room.go` is empty), #435.

---

## Group D: other surfaces (#252, #272, #218)

Dry run 2026-10-08 on `origin/master` `f50400bf7`: every task below was run in a scratch
worktree in this order (test first, seen red, fix, seen green), then discarded. Observed
output is quoted under each step. Six tasks, six commits.

**Facts this group adds to the spec's table (read from source in the dry run):**

| Fact | Where |
|---|---|
| `GetRoomNode` builds all Room.Info lanes; the sub-module requests (`Room.Info.Contents.Players` etc.) return early from the same function | `modules/gmcp/gmcp.Room.go:204-465` |
| Hidden occupants already skipped via `HasConditionFlag(conditions.Hidden)` | `gmcp.Room.go:288`, `:316` |
| Room.Info is pushed on `RoomChange` only; nothing re-pushes it when light changes | `gmcp.Room.go:97-151` |
| `events.SightBandChanged{UserId}` is queued by `lightnotice` when a player's band changes; `gmcp.Char.go:123` answers it with `Char.Sight` only | `internal/lightnotice/tracker.go:276`; `internal/events/eventtypes.go:444` |
| GMCP tests that build a mapper must call `mudlog.SetupLogger(nil, "", "", false)` or `mapper.Start` panics on a nil logger | observed in the dry run; precedent `modules/gmcp/gmcp.Relay_test.go:28` |
| `Communication` has no hidden flag; mob `sayto` while hidden queues `CommType: "say"` with `TargetUserId` and the real name, and `onComm` ignores `TargetUserId` for say, so every player in the room got the hidden mob's name over GMCP | `internal/mobcommands/sayto.go:41`, `:110`; `modules/gmcp/gmcp.Comm.go:66-86` |
| `speakerNoun` (unexported) is the "a figure" / "someone" word for speakers | `internal/messaging/hidenames.go:77` |
| `Character.FindItem(name) (items.Item, string, bool)` searches backpack, worn, bandolier and component bag | `internal/characters/inventory.go:464` |
| Mob look's silent-refusal case list | `internal/mobcommands/look.go:44` |
| Root guards: `finderViewSites` counts `DisplayNameFor` calls per function (`look.go|Look` is 2); `transientItemHolders` must list any struct holding an `items.Item` | `bauble_finder_view_guard_test.go:78`; `bauble_sweep_guard_test.go:38` |

**Spec corrections found in the dry run (handled below, owner already approved the intent):**
- Room.Info at `SightNone` also blanks the title (`Name`), items and containers, not only the
  description, exits and rosters: `look` refuses the whole room in the dark, and a title or a
  floor item would leak the same way. Id, area, environment, symbol and coordinates stay so the
  client can place the player on the map they hold.
- Lighting a torch inside a room must refresh Room.Info and map the room, or the client keeps
  the dark payload until the next move. Task D2 answers `SightBandChanged`.
- Hidden mob `sayto` leaked the name over GMCP to the whole room. Task D3 fixes it with the
  same change (it is the same delivery code).

---

### Task D1: GMCP Room.Info follows the viewer's sight (#252)

**Files:**
- Modify: `modules/gmcp/gmcp.Room.go:3-16` (imports), `:204-465` (`GetRoomNode`), new helper after `GetRoomNode`
- Test: `modules/gmcp/gmcp.RoomSight_test.go` (new)

- [ ] **Step 1: Write the failing tests**

Create `modules/gmcp/gmcp.RoomSight_test.go`:

```go
package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// roomSightFixture seeds one sky-less room lit by exactly lamp (0 is pitch
// dark, 30 shapes, 60 faces), with an exit, a floor item, a mob (Grave
// Wight) and a second player (Kesh), and returns the viewer standing in it.
func roomSightFixture(t *testing.T, lamp int) *users.UserRecord {
	t.Helper()
	mudlog.SetupLogger(nil, ``, ``, false) // the mapper logs when it builds a zone
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"default": {BiomeId: "default"},
	}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		999981: {ItemId: 999981, Name: "Grey Pebble", Type: items.Object},
	}))
	zero := 0.0
	room := &rooms.Room{RoomId: 9720, Zone: "SightZone", Biome: "default", SkyLight: &zero,
		Title: "Cellar", Description: "A low cellar smelling of damp stone.",
		Exits: map[string]exit.RoomExit{"north": {RoomId: 9721}},
		Items: []items.Item{items.New(999981)}}
	if lamp > 0 {
		room.Lamp = rooms.LampPtr(lamp)
	}
	north := &rooms.Room{RoomId: 9721, Zone: "SightZone", Biome: "default"}
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{9720: room, 9721: north},
		map[string]*rooms.ZoneConfig{"SightZone": {Name: "SightZone", RoomId: 9720,
			RoomIds: map[int]struct{}{9720: {}, 9721: {}}}},
	))

	wight := &mobs.Mob{MobId: 1, InstanceId: 721, Zone: "SightZone"}
	wight.Character.Name = "Grave Wight"
	wight.Character.RoomId = 9720
	t.Cleanup(mobs.SeedMobsForTest(
		map[int]*mobs.Mob{1: {MobId: 1, Zone: "SightZone"}},
		map[int]*mobs.Mob{721: wight},
	))
	room.AddMob(721)

	viewer := users.NewTestUser(9722, "viewer", "Viewer", 97722)
	viewer.Character.RoomId = 9720
	kesh := users.NewTestUser(9723, "kesh", "Kesh", 97723)
	kesh.Character.RoomId = 9720
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9722: viewer, 9723: kesh}))
	room.AddPlayer(9722)
	room.AddPlayer(9723)
	return viewer
}

func roomInfoFor(t *testing.T, viewer *users.UserRecord) GMCPRoomModule_Payload {
	t.Helper()
	data, name := (&GMCPRoomModule{}).GetRoomNode(viewer, `Room.Info`)
	require.Equal(t, `Room.Info`, name)
	p, ok := data.(GMCPRoomModule_Payload)
	require.True(t, ok, "Room.Info payload is %T", data)
	return p
}

// #252: a viewer who sees nothing gets the room's identity (id, area,
// coordinates, so the map can place them) and nothing the light would show:
// no title, description, exits, items or occupants. Empty, not omitted, so a
// client merging payloads drops the last sighted reading (see Char.Enemies).
func TestRoomInfo_DarkViewerGetsNoSightedDetail(t *testing.T) {
	p := roomInfoFor(t, roomSightFixture(t, 0))
	require.Equal(t, 9720, p.Id)
	require.Equal(t, ``, p.Name)
	require.Equal(t, ``, p.Description)
	require.Empty(t, p.Exits)
	require.Empty(t, p.ExitsV2)
	require.NotNil(t, p.Exits, "exits must be an empty map, not omitted")
	require.Empty(t, p.Contents.Items)
	require.Empty(t, p.Contents.Players)
	require.Empty(t, p.Contents.Npcs)
}

// #252: at shapes the room reads (title, description, exits, items) but every
// occupant is the anonymous figure with no id or adjectives, as the room
// roster shows them.
func TestRoomInfo_ShapesViewerGetsFiguresNotNames(t *testing.T) {
	p := roomInfoFor(t, roomSightFixture(t, 30))
	require.Equal(t, `Cellar`, p.Name)
	require.NotEmpty(t, p.Description)
	require.Contains(t, p.Exits, `north`)
	require.Len(t, p.Contents.Items, 1)
	require.Len(t, p.Contents.Players, 1)
	require.Len(t, p.Contents.Npcs, 1)
	for _, c := range append(p.Contents.Players, p.Contents.Npcs...) {
		require.Equal(t, `a figure`, c.Name)
		require.Equal(t, ``, c.Id)
		require.Empty(t, c.Adjectives)
	}
}

// Clear sight is unchanged: names and ids as before.
func TestRoomInfo_SightedViewerGetsNames(t *testing.T) {
	p := roomInfoFor(t, roomSightFixture(t, 60))
	require.Equal(t, `Cellar`, p.Name)
	require.Len(t, p.Contents.Players, 1)
	require.Equal(t, `Kesh`, p.Contents.Players[0].Name)
	require.NotEqual(t, ``, p.Contents.Players[0].Id)
	require.Len(t, p.Contents.Npcs, 1)
	require.Equal(t, `Grave Wight`, p.Contents.Npcs[0].Name)
}

// The sub-module requests take the same gate: a client asking for only the
// players list in the dark gets an empty list.
func TestRoomInfoContentsPlayers_DarkViewerGetsNone(t *testing.T) {
	viewer := roomSightFixture(t, 0)
	data, _ := (&GMCPRoomModule{}).GetRoomNode(viewer, `Room.Info.Contents.Players`)
	got, ok := data.([]GMCPRoomModule_Payload_Contents_Character)
	require.True(t, ok, "payload is %T", data)
	require.Empty(t, got)
}
```

(The `events` import is used by Task D2's tests, appended to this file. If you run D1 alone,
drop it until D2.)

- [ ] **Step 2: Run, confirm red**

Run: `go test ./modules/gmcp/ -run "TestRoomInfo" 2>&1 | grep -v "^time="`
Expected (observed): `TestRoomInfo_DarkViewerGetsNoSightedDetail` fails (`expected: "" actual: "Cellar"`),
`TestRoomInfo_ShapesViewerGetsFiguresNotNames` fails (`expected: "a figure" actual: "Kesh"`),
`TestRoomInfoContentsPlayers_DarkViewerGetsNone` fails (`Should be empty, but was [{@9723 Kesh [] false false}]`).
`TestRoomInfo_SightedViewerGetsNames` passes (the regression guard).

- [ ] **Step 3: Implement**

In `modules/gmcp/gmcp.Room.go` imports add `"github.com/GoMudEngine/GoMud/internal/messaging"`
after `.../internal/mapper`.

In `GetRoomNode`, right after `payload := GMCPRoomModule_Payload{}`:

```go
	// The payload shows what the room text shows (#252): a viewer who sees
	// nothing gets no title, description, exits, items or occupants; a viewer
	// who sees shapes gets every occupant as the roster's anonymous figure.
	// One verdict per call: it depends only on the viewer and the room.
	sight := messaging.ParticipantSight(user.Character, room)
	seesNothing := sight == messaging.SightNone
```

At the top of each of the four loops (`for name, container := range room.Containers {`,
`for _, itm := range room.Items {`, `for _, uId := range room.GetPlayers() {` in the Players
block, `for _, mIId := range room.GetMobs() {`) add as the first statement:

```go
			if seesNothing {
				break
			}
```

In the Players loop, after the `conditions.Hidden` skip and before the existing `append`:

```go
			if sight == messaging.SightShapes {
				payload.Contents.Players = append(payload.Contents.Players, unseenOccupant(u.Character.IsInCombat()))
				continue
			}
```

In the Npcs loop, after the `conditions.Hidden` skip and before `c := GMCPRoomModule_Payload_Contents_Character{`:

```go
			if sight == messaging.SightShapes {
				payload.Contents.Npcs = append(payload.Contents.Npcs, unseenOccupant(mob.Character.IsInCombat()))
				continue
			}
```

Replace the basic-details lines:

```go
		// Basic details. The id, area, environment and coordinates stay in the
		// dark: they place the player on a map they already hold.
		payload.Id = room.RoomId
		if !seesNothing {
			payload.Name = room.Title
			payload.Description = room.Description
		}
		payload.Area = room.Zone
```

At the top of `for exitName, exitInfo := range room.Exits {`:

```go
			// Empty maps, not omitted, in the dark, so a client merging
			// payloads drops the last sighted exits (the Char.Enemies rule).
			if seesNothing {
				break
			}
```

Add after `GetRoomNode`, before `wantsGMCPPayload`:

```go
// unseenOccupant is a roster entry for a creature the viewer makes out only as
// a shape: the room roster's "a figure" (messaging.UnseenNoun), no id and no
// adjectives, since either would name it. Whether it is fighting stays: the
// motion is what a shape shows.
func unseenOccupant(inCombat bool) GMCPRoomModule_Payload_Contents_Character {
	return GMCPRoomModule_Payload_Contents_Character{
		Name:       messaging.UnseenNoun(messaging.SightShapes),
		Adjectives: []string{},
		Aggro:      inCombat,
	}
}
```

- [ ] **Step 4: Run, confirm green, including the wire freeze**

Run: `go test ./modules/gmcp/ 2>&1 | grep -v "^time=" | tail -3`
Expected (observed): `ok  	github.com/GoMudEngine/GoMud/modules/gmcp`. Blanking values does not
change field names, so `TestWireFreeze_GMCPJSONFieldNames` stays green.

- [ ] **Step 5: Commit**

```bash
git add modules/gmcp/gmcp.Room.go modules/gmcp/gmcp.RoomSight_test.go
git commit -m "fix(gmcp): Room.Info follows the viewer's sight (#252)

In the dark the payload carries no title, description, exits, items or
occupants; at shapes every occupant is 'a figure' with no id or adjectives.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task D2: the fog map adds only rooms the player can see; light refreshes Room.Info (#252, R5)

**Files:**
- Create: `internal/actions/map_visit.go`, `internal/actions/map_visit_test.go`
- Modify: `internal/usercommands/go.go:284-289` (the `MarkRoomVisited` call)
- Modify: `modules/gmcp/gmcp.Room.go` (import `internal/actions`; register and add `sightBandChangedHandler`)
- Test: append to `modules/gmcp/gmcp.RoomSight_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/actions/map_visit_test.go`:

```go
package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// mapVisitRoom seeds one sky-less room lit by exactly lamp (0 is pitch dark)
// and a character standing in it.
func mapVisitRoom(t *testing.T, lamp int) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	zero := 0.0
	room := &rooms.Room{RoomId: 9730, Zone: "MapZone", SkyLight: &zero}
	if lamp > 0 {
		room.Lamp = rooms.LampPtr(lamp)
	}
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{9730: room},
		map[string]*rooms.ZoneConfig{"MapZone": {Name: "MapZone", RoomId: 9730, RoomIds: map[int]struct{}{9730: {}}}},
	))
	u := users.NewTestUser(9731, "mapper", "Mapper", 97731)
	u.Character.RoomId = 9730
	return u, room
}

// #252, R5: a room walked through in pitch dark does not join the map.
func TestMarkRoomMappedIfSeen_DarkRoomStaysOffTheMap(t *testing.T) {
	u, room := mapVisitRoom(t, 0)
	require.False(t, MarkRoomMappedIfSeen(u.Character, room))
	require.False(t, u.Character.HasVisitedRoom("MapZone", 9730))
}

// A room the character makes out, even as shapes, joins the map.
func TestMarkRoomMappedIfSeen_ShapesRoomJoinsTheMap(t *testing.T) {
	u, room := mapVisitRoom(t, 30)
	require.True(t, MarkRoomMappedIfSeen(u.Character, room))
	require.True(t, u.Character.HasVisitedRoom("MapZone", 9730))
}

// A room already on the map stays there when the character returns blind.
func TestMarkRoomMappedIfSeen_DarkReturnKeepsAMappedRoom(t *testing.T) {
	u, room := mapVisitRoom(t, 0)
	u.Character.MarkRoomVisited("MapZone", 9730)
	require.False(t, MarkRoomMappedIfSeen(u.Character, room))
	require.True(t, u.Character.HasVisitedRoom("MapZone", 9730))
}
```

Append to `modules/gmcp/gmcp.RoomSight_test.go`:

```go
// A band change maps the room the player now makes out (#252, R5): lighting a
// torch in a room entered in the dark puts it on the map.
func TestRoomSightBandChanged_MapsTheRoomOnceSeen(t *testing.T) {
	viewer := roomSightFixture(t, 30)
	require.False(t, viewer.Character.HasVisitedRoom("SightZone", 9720))
	(&GMCPRoomModule{}).sightBandChangedHandler(events.SightBandChanged{UserId: viewer.UserId})
	require.True(t, viewer.Character.HasVisitedRoom("SightZone", 9720))
}

// Still dark: the band changed but nothing is seen, so nothing is mapped.
func TestRoomSightBandChanged_DarkMapsNothing(t *testing.T) {
	viewer := roomSightFixture(t, 0)
	(&GMCPRoomModule{}).sightBandChangedHandler(events.SightBandChanged{UserId: viewer.UserId})
	require.False(t, viewer.Character.HasVisitedRoom("SightZone", 9720))
}
```

- [ ] **Step 2: Run, confirm red**

Run: `go test ./internal/actions/ -run TestMarkRoomMappedIfSeen` and `go test ./modules/gmcp/ -run TestRoomSightBandChanged`
Expected: build failure, `undefined: MarkRoomMappedIfSeen` / `sightBandChangedHandler`.
(Dry run null probe, after Step 3: replacing the `SightNone` check with an impossible value
turned `..._DarkRoomStaysOffTheMap` and `..._DarkReturnKeepsAMappedRoom` red; disabling the
`MarkRoomMappedIfSeen` call in the handler turned `..._MapsTheRoomOnceSeen` red.)

- [ ] **Step 3: Implement**

Create `internal/actions/map_visit.go`:

```go
package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// MarkRoomMappedIfSeen records room on the character's fog-of-war map
// (characters.Character.MarkRoomVisited, the Zone.Map set) when the character
// makes out anything there, and reports whether it did. A room walked through
// in the dark stays off the map until the character sees it (#252, owner
// ruling R5); rooms already on the map stay.
//
// The template id goes on the map, never an ephemeral instance id: the raw
// instance id (e.g. 1000000000) is unauthorable and would leak onto the
// Zone.Map snapshot. rooms.OriginalRoomId returns a normal room's own id.
func MarkRoomMappedIfSeen(c *characters.Character, room *rooms.Room) bool {
	if c == nil || room == nil {
		return false
	}
	if messaging.ParticipantSight(c, room) == messaging.SightNone {
		return false
	}
	templateId, _ := rooms.OriginalRoomId(room.RoomId)
	c.MarkRoomVisited(room.Zone, templateId)
	return true
}
```

In `internal/usercommands/go.go`, replace the comment block and call at `:284-289`
(`// Record this room as visited for fog-of-war web map. ...` through
`user.Character.MarkRoomVisited(destRoom.Zone, matchRoom)`) with:

```go
			// Record this room on the fog-of-war web map, by its template id,
			// only if the player can make anything out there: a room walked
			// through in the dark stays off the map (#252, ruling R5).
			actions.MarkRoomMappedIfSeen(user.Character, destRoom)
```

(`matchRoom` stays: the quest `room_enter` notify above still uses it.)

In `modules/gmcp/gmcp.Room.go` add import `"github.com/GoMudEngine/GoMud/internal/actions"`
(gmcp already imports actions in `gmcp.CharOp.go`, so no cycle), register in `init()` after
the `GMCPRoomUpdate` listener:

```go
	events.RegisterListener(events.SightBandChanged{}, g.sightBandChangedHandler)
```

and add after `init()`:

```go
// sightBandChangedHandler re-sends Room.Info when a player's light band
// changes, since the payload follows sight (#252): lighting a torch in a dark
// room fills in the room, and losing the light empties it. A room the player
// now makes out joins their map, and the map is re-sent, so a room entered in
// the dark is mapped the moment it is seen.
func (g *GMCPRoomModule) sightBandChangedHandler(e events.Event) events.ListenerReturn {

	evt, typeOk := e.(events.SightBandChanged)
	if !typeOk || evt.UserId == 0 {
		return events.Continue
	}

	user := users.GetByUserId(evt.UserId)
	if user == nil {
		return events.Continue
	}

	if room := rooms.LoadRoom(user.Character.RoomId); room != nil {
		if actions.MarkRoomMappedIfSeen(user.Character, room) {
			events.AddToQueue(GMCPZoneUpdate{UserId: evt.UserId})
		}
	}

	events.AddToQueue(GMCPRoomUpdate{
		UserId:     evt.UserId,
		Identifier: `Room.Info`,
	})

	return events.Continue
}
```

- [ ] **Step 4: Run, confirm green**

Run: `go test ./internal/actions/ -run TestMarkRoomMappedIfSeen` then `go test ./modules/gmcp/ ./internal/usercommands/ 2>&1 | grep -v "^time=" | tail -3`
Expected (observed): all `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/actions/map_visit.go internal/actions/map_visit_test.go internal/usercommands/go.go modules/gmcp/gmcp.Room.go modules/gmcp/gmcp.RoomSight_test.go
git commit -m "fix(gmcp): the map adds only rooms the player can see; light refreshes Room.Info (#252)

Owner ruling R5: rooms already on the map stay, a room walked in the dark
joins it once seen. A light band change re-sends Room.Info and the map.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task D3: GMCP say follows each listener's sight and the deafen filter (#252)

**Files:**
- Modify: `internal/messaging/hidenames.go:73-98` (export `speakerNoun` as `SpeakerNoun`)
- Modify: `internal/events/eventtypes.go:152-159` (`Communication.SpeakerHidden`)
- Modify: `internal/actions/say.go:43-49`, `internal/mobcommands/sayto.go:41-47`, `:110-116`
- Modify: `modules/gmcp/gmcp.Comm.go` (imports, say branch, new `sayDeliveries`)
- Test: `modules/gmcp/gmcp.Comm_test.go` (new)

- [ ] **Step 1: Write the failing tests**

Create `modules/gmcp/gmcp.Comm_test.go`:

```go
package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// sayFixture seeds one sky-less room lit by exactly lamp (0 is pitch dark, 30
// shapes, 60 faces) holding the speaker Kesh (9741), a listener Ana (9742), a
// deafened listener Bo (9743) and a mob, the Crier (instance 741).
func sayFixture(t *testing.T, lamp int) {
	t.Helper()
	zero := 0.0
	room := &rooms.Room{RoomId: 9740, Zone: "SayZone", SkyLight: &zero}
	if lamp > 0 {
		room.Lamp = rooms.LampPtr(lamp)
	}
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{9740: room},
		map[string]*rooms.ZoneConfig{"SayZone": {Name: "SayZone", RoomId: 9740, RoomIds: map[int]struct{}{9740: {}}}},
	))
	crier := &mobs.Mob{MobId: 1, InstanceId: 741, Zone: "SayZone"}
	crier.Character.Name = "Crier"
	crier.Character.RoomId = 9740
	t.Cleanup(mobs.SeedMobsForTest(
		map[int]*mobs.Mob{1: {MobId: 1, Zone: "SayZone"}},
		map[int]*mobs.Mob{741: crier},
	))
	room.AddMob(741)

	kesh := users.NewTestUser(9741, "kesh", "Kesh", 97741)
	ana := users.NewTestUser(9742, "ana", "Ana", 97742)
	bo := users.NewTestUser(9743, "bo", "Bo", 97743)
	bo.Deafened = true
	for _, u := range []*users.UserRecord{kesh, ana, bo} {
		u.Character.RoomId = 9740
	}
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9741: kesh, 9742: ana, 9743: bo}))
	room.AddPlayer(9741)
	room.AddPlayer(9742)
	room.AddPlayer(9743)
}

// sendersByUser runs sayDeliveries and maps each recipient to the Sender it reads.
func sendersByUser(evt events.Communication) map[int]string {
	got := map[int]string{}
	for _, d := range sayDeliveries(evt, GMCPCommModule_Payload{Channel: `say`, Sender: evt.Name}) {
		got[d.UserId] = d.Payload.Sender
	}
	return got
}

func keshSays() events.Communication {
	return events.Communication{SourceUserId: 9741, CommType: `say`, Name: `Kesh`, Message: `hello`}
}

// #252: the sender follows each listener's sight, as the text lane does; the
// deafened listener gets no player chatter; the speaker reads their own name.
func TestSayDeliveries_SenderFollowsListenerSight(t *testing.T) {
	cases := []struct {
		lamp int
		want string
	}{
		{60, `Kesh`},
		{30, `A figure`},
		{0, `Someone`},
	}
	for _, c := range cases {
		sayFixture(t, c.lamp)
		got := sendersByUser(keshSays())
		require.Equal(t, map[int]string{9741: `Kesh`, 9742: c.want}, got, "lamp %d", c.lamp)
	}
}

// A speaker still hidden after speaking is "Someone" to every listener, even
// in full light; the speaker still reads their own name.
func TestSayDeliveries_HiddenSpeakerIsSomeone(t *testing.T) {
	sayFixture(t, 60)
	evt := keshSays()
	evt.SpeakerHidden = true
	got := sendersByUser(evt)
	require.Equal(t, map[int]string{9741: `Kesh`, 9742: `Someone`}, got)
}

// An NPC's words are authored, so they reach a deafened listener too (ruling 6).
func TestSayDeliveries_MobSpeechReachesTheDeafened(t *testing.T) {
	sayFixture(t, 60)
	got := sendersByUser(events.Communication{SourceMobInstanceId: 741, CommType: `say`, Name: `Crier`, Message: `hear ye`})
	require.Equal(t, map[int]string{9741: `Crier`, 9742: `Crier`, 9743: `Crier`}, got)
}

// A mob's sayto to one player reaches only that player, not the room.
func TestSayDeliveries_TargetedSayReachesOnlyTheTarget(t *testing.T) {
	sayFixture(t, 60)
	got := sendersByUser(events.Communication{SourceMobInstanceId: 741, TargetUserId: 9742,
		CommType: `say`, Name: `Crier`, Message: `psst`, SpeakerHidden: true})
	require.Equal(t, map[int]string{9742: `Someone`}, got)
}
```

- [ ] **Step 2: Run, confirm red**

Run: `go test ./modules/gmcp/ -run TestSayDeliveries`
Expected: build failure, `undefined: sayDeliveries` and `unknown field SpeakerHidden`.
(Dry run null probe, after Step 3: disabling the per-listener block and the `TargetUserId`
branch turned `..._SenderFollowsListenerSight`, `..._HiddenSpeakerIsSomeone` and
`..._TargetedSayReachesOnlyTheTarget` red. `..._MobSpeechReachesTheDeafened` is a guard on
ruling 6 and is meant to stay green under that probe.)

- [ ] **Step 3: Implement**

`internal/messaging/hidenames.go`: rename `speakerNoun` to `SpeakerNoun` (three occurrences:
the doc comment line 73, the `func` line 77, the call in `HideSpeakerNames` line 98). The doc
comment becomes `// SpeakerNoun is what a listener at d calls ...`, otherwise unchanged.

`internal/events/eventtypes.go`, in `Communication` after `Message string`:

```go
	SpeakerHidden       bool // a say spoken while still hidden: no listener but the speaker learns who (#252)
```

`internal/actions/say.go`, in the `events.Communication{...}` literal after `Message: text,`:

```go
		SpeakerHidden:       isSneaking,
```

`internal/mobcommands/sayto.go`: in the `if isSneaking {` branch's `events.Communication{...}`
(line 41, the one with `TargetUserId: toUser.UserId`) add `SpeakerHidden: true,` after
`Message: rest,`; in `SayToOnly`'s literal (line 110) add `SpeakerHidden: isSneaking,` after
`Message: rest,`.

`modules/gmcp/gmcp.Comm.go` imports become:

```go
import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/channels"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/plugins"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/ansitags"
)
```

Replace the whole `if evt.CommType == `say` { ... }` branch body (lines 66-86, the room
lookup and `sendToUserIds = append([]int{}, room.GetPlayers()...)`) with:

```go
	if evt.CommType == `say` {

		// Say is room speech and follows each listener, as the text lane does
		// (actions.sendSpoken): its own payload per listener.
		for _, d := range sayDeliveries(evt, payload) {
			events.AddToQueue(GMCPOut{
				UserId:  d.UserId,
				Module:  `Comm.Channel`,
				Payload: d.Payload,
			})
		}
		return events.Continue

	} else if evt.CommType == `party` {
```

Add before `type GMCPCommModule_Payload struct`:

```go
// commDelivery is one listener's copy of a Comm.Channel payload.
type commDelivery struct {
	UserId  int
	Payload GMCPCommModule_Payload
}

// sayDeliveries is who receives a say over GMCP and what each reads, matching
// the text lane (actions.sendSpoken, #252):
//
//   - A say aimed at one player (a mob's sayto, TargetUserId) reaches only
//     that player and the speaker; any other say reaches everyone in the
//     speaker's room.
//   - A player's words are chatter: a deafened listener does not receive
//     them. An NPC's are authored and reach a deafened listener (ruling 6).
//   - The sender reads by the listener's sight of the room: the name at clear
//     sight, "A figure" at shapes, "Someone" when they see nothing, and
//     "Someone" for everyone when the speaker spoke still hidden. The speaker
//     always reads their own name.
func sayDeliveries(evt events.Communication, base GMCPCommModule_Payload) []commDelivery {

	var room *rooms.Room
	if evt.SourceUserId > 0 {
		if user := users.GetByUserId(evt.SourceUserId); user != nil {
			room = rooms.LoadRoom(user.Character.RoomId)
		}
	}
	if evt.SourceMobInstanceId > 0 {
		if mob := mobs.GetInstance(evt.SourceMobInstanceId); mob != nil {
			room = rooms.LoadRoom(mob.Character.RoomId)
		}
	}
	if room == nil {
		return nil
	}

	listenerIds := room.GetPlayers()
	if evt.TargetUserId > 0 {
		listenerIds = []int{evt.TargetUserId}
	}

	deliveries := []commDelivery{}
	for _, uid := range listenerIds {
		u := users.GetByUserId(uid)
		if u == nil {
			continue
		}

		p := base
		if uid != evt.SourceUserId {
			if evt.SourceUserId > 0 && u.Deafened {
				continue
			}
			d := messaging.ParticipantSight(u.Character, room)
			if evt.SpeakerHidden {
				d = messaging.SightNone
			}
			if d != messaging.SightFull {
				noun := messaging.SpeakerNoun(d) // "a figure" or "someone", ASCII
				p.Sender = strings.ToUpper(noun[:1]) + noun[1:]
			}
		}
		deliveries = append(deliveries, commDelivery{UserId: uid, Payload: p})
	}
	return deliveries
}
```

(A mob speaker's own id is never a player id, so for a mob `uid != evt.SourceUserId` is always
true and every listener is judged; `SourceUserId > 0` keeps the deafen filter to player
chatter. The comment on the targeted case says "and the speaker": for a mob there is none.)

- [ ] **Step 4: Run, confirm green**

Run: `go build ./... && go test ./modules/gmcp/ ./internal/messaging/ ./internal/events/ ./internal/actions/ ./internal/mobcommands/ 2>&1 | grep -v "^time=" | tail -6`
Expected (observed): all `ok` (`internal/actions` takes about 75 to 100 s).

- [ ] **Step 5: Commit**

```bash
git add internal/messaging/hidenames.go internal/events/eventtypes.go internal/actions/say.go internal/mobcommands/sayto.go modules/gmcp/gmcp.Comm.go modules/gmcp/gmcp.Comm_test.go
git commit -m "fix(gmcp): say follows each listener's sight and the deafen filter (#252)

GMCP Comm say now matches the text lane: name, 'A figure' or 'Someone' by
the listener's sight, 'Someone' for a speaker still hidden, no player
chatter to the deafened. A mob's sayto reaches only its target; it used
to send a hidden mob's name to the whole room.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task D4: appraise and offer refuse in the dark (#272)

**Files:**
- Modify: `internal/usercommands/appraise.go:18-20`, `internal/usercommands/offer.go:28-30`
- Test: `internal/usercommands/appraise_offer_sight_test.go` (new; reuses `listSightRoom` and `listSightPlain` from `list_sight_test.go`)

- [ ] **Step 1: Write the failing tests**

```go
package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/require"
)

// #272: appraise and offer are dealing, so they take the shop sight gate that
// list, buy and sell already take (lighting plan 5b): below the faces band
// they refuse with the one shared line, name no merchant and take no gold.
// The fixture is list_sight_test.go's listSightRoom (merchant "Keeper").

func TestAppraise_DarkRoom_RefusesAndNamesNoMerchant(t *testing.T) {
	for _, biome := range []string{"cave", "shapes"} {
		user, room := listSightRoom(t, biome)
		user.Character.StoreItem(items.New(84001))
		user.Character.Gold = 100

		handled, err := Appraise("tin cup", user, room, 0)
		require.NoError(t, err)
		require.True(t, handled)

		lines := listSightPlain(events.DrainQueuedMessagesForTest(8411))
		require.Equal(t, []string{actions.ShopSightRefusalText}, lines, "biome %s", biome)
		require.Equal(t, 100, user.Character.Gold, "biome %s: no fee in the dark", biome)
	}
}

func TestOffer_DarkRoom_Refuses(t *testing.T) {
	for _, biome := range []string{"cave", "shapes"} {
		user, room := listSightRoom(t, biome)
		user.Character.StoreItem(items.New(84001))

		handled, err := Offer("tin cup", user, room, 0)
		require.NoError(t, err)
		require.True(t, handled)

		lines := listSightPlain(events.DrainQueuedMessagesForTest(8411))
		require.Equal(t, []string{actions.ShopSightRefusalText}, lines, "biome %s", biome)
	}
}

// Control: in a lit shop both verbs deal as before and never refuse.
func TestAppraiseOffer_LitRoom_Deal(t *testing.T) {
	user, room := listSightRoom(t, "city")
	user.Character.StoreItem(items.New(84001))
	user.Character.Gold = 100

	_, err := Appraise("tin cup", user, room, 0)
	require.NoError(t, err)
	_, err = Offer("tin cup", user, room, 0)
	require.NoError(t, err)

	joined := strings.Join(listSightPlain(events.DrainQueuedMessagesForTest(8411)), "\n")
	require.NotContains(t, joined, actions.ShopSightRefusalText)
	require.Contains(t, joined, "Keeper")
}
```

- [ ] **Step 2: Run, confirm red**

Run: `go test ./internal/usercommands/ -run "TestAppraise_|TestOffer_|TestAppraiseOffer_" 2>&1 | grep -v "^time="`
Expected (observed on master): `TestAppraise_DarkRoom_RefusesAndNamesNoMerchant` fails with
`actual: []string{"You give Keeper 20 gold to appraise tin cup.", ...}` (the #272 leak, and
the fee taken in the dark); `TestOffer_DarkRoom_Refuses` fails with `actual: []string{}`.
The lit control passes.

- [ ] **Step 3: Implement**

`internal/usercommands/appraise.go`, first statement of `Appraise`:

```go
	// Below the faces band you can't make out the goods (lighting plan 5b,
	// #272): appraising is dealing, as list, buy and sell are.
	if actions.ShopSightRefusal(user.Character, room) {
		user.SendText(messaging.CategorySystem, actions.ShopSightRefusalText)
		return true, nil
	}
```

`internal/usercommands/offer.go`, first statement of `Offer`:

```go
	// Below the faces band you can't make out the goods (lighting plan 5b,
	// #272): asking for an offer is dealing, as list, buy and sell are.
	if actions.ShopSightRefusal(user.Character, room) {
		user.SendText(messaging.CategorySystem, actions.ShopSightRefusalText)
		return true, nil
	}
```

Both files already import `actions` and `messaging`. The refusal comes before the merchant
lookup, so in the dark it does not reveal whether a merchant is present.

- [ ] **Step 4: Run, confirm green**

Run: same command as Step 2. Expected (observed): `ok  	github.com/GoMudEngine/GoMud/internal/usercommands`.

- [ ] **Step 5: Commit**

```bash
git add internal/usercommands/appraise.go internal/usercommands/offer.go internal/usercommands/appraise_offer_sight_test.go
git commit -m "fix(shops): appraise and offer refuse in the dark like sell (#272)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task D5: look at your own gear by touch in the dark (#218)

The rule lives in `actions.ResolveLook`, per its contract ("Every sight rule of look lives in
actions.ResolveLook", `usercommands/look.go:44-48`), as a new `LookKind`; each wrapper words it.

**Files:**
- Modify: `internal/actions/look.go` (import `items`; `LookOwnGearByTouch`; `LookResolution.TouchItem`; the `SightNone` branch)
- Modify: `internal/usercommands/look.go:50-51`, `internal/mobcommands/look.go:44`
- Modify (root guards): `bauble_finder_view_guard_test.go:78`, `bauble_sweep_guard_test.go:38-45`
- Test: `internal/usercommands/look_own_gear_touch_test.go` (new; reuses `seedDarknessGateRoom`, `runGate`, `tooDarkToSeeLine`, `blindLookLine`)

- [ ] **Step 1: Write the failing tests**

```go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/stretchr/testify/require"
)

// #218: your own gear needs no light. A looker who sees nothing, in the dark
// or blinded, still knows by touch what they carry: look names it, without
// its description (which sight reads). Anything else keeps the refusal.

const touchGearItemId = 99218

func seedTouchGear(t *testing.T, lamp int) (func(string) string, *characters.Character) {
	t.Helper()
	user, room := seedDarknessGateRoom(t, lamp)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		touchGearItemId: {ItemId: touchGearItemId, Name: "Oak Staff", Type: items.Weapon,
			Description: "A staff carved with running hares."},
	}))
	require.True(t, user.Character.StoreItem(items.New(touchGearItemId)))
	look := func(what string) string {
		return runGate(t, user, func() (bool, error) { return Look(what, user, room, 0) })
	}
	return look, user.Character
}

func TestLook_OwnGearInTheDarkIsKnownByTouch(t *testing.T) {
	look, _ := seedTouchGear(t, 0)
	out := look("staff")
	require.Contains(t, out, "You run your hands over your")
	require.Contains(t, out, "Oak Staff")
	require.NotContains(t, out, "running hares", "the description is what sight reads")
	require.NotContains(t, out, tooDarkToSeeLine)
}

func TestLook_OwnGearWhileBlindedIsKnownByTouch(t *testing.T) {
	look, char := seedTouchGear(t, 90)
	orig := char.Perception
	t.Cleanup(func() { char.Perception = orig })
	char.Perception = characters.New().Perception
	require.NoError(t, char.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}))

	out := look("staff")
	require.Contains(t, out, "You run your hands over your")
	require.NotContains(t, out, blindLookLine)
}

// Something that is not yours keeps the refusal in the dark.
func TestLook_NotYourGearInTheDarkStillRefuses(t *testing.T) {
	look, _ := seedTouchGear(t, 0)
	out := look("lantern")
	require.Contains(t, out, tooDarkToSeeLine)
	require.NotContains(t, out, "You run your hands over your")
}
```

- [ ] **Step 2: Run, confirm red**

Run: `go test ./internal/usercommands/ -run "TestLook_OwnGear|TestLook_NotYourGear" 2>&1 | grep -E "^(--- FAIL|ok|FAIL)"`
Expected (observed with the ResolveLook branch disabled):
`--- FAIL: TestLook_OwnGearInTheDarkIsKnownByTouch`, `--- FAIL: TestLook_OwnGearWhileBlindedIsKnownByTouch`.
`TestLook_NotYourGearInTheDarkStillRefuses` passes (guards the refusal).

- [ ] **Step 3: Implement**

`internal/actions/look.go`: add import `"github.com/GoMudEngine/GoMud/internal/items"`. Append
to the `LookKind` const block, after `LookOther` (appended, so no existing value renumbers):

```go
	LookOwnGearByTouch              // sees nothing, but names an item it wears or carries: known by touch (#218)
```

Add to `LookResolution` after `PetUserId int`:

```go
	// TouchItem is the worn or carried item a looker who sees nothing found
	// by touch (LookOwnGearByTouch).
	TouchItem items.Item
```

At the top of `if res.Sight == messaging.SightNone {` in `ResolveLook`, before the
`// The cause, told apart ...` comment:

```go
		// Your own gear needs no light: a looker who sees nothing still
		// knows by touch what it wears or carries (#218). Only the item, by
		// name: its description is what sight reads.
		if lookAt != `` {
			if item, _, found := char.FindItem(lookAt); found {
				res.Kind, res.TouchItem = LookOwnGearByTouch, item
				return res
			}
		}
```

Then `gofmt -w internal/actions/look.go` (the const block's comment column widens).

`internal/usercommands/look.go`, right after `res := actions.ResolveLook(...)`:

```go
	if res.Kind == actions.LookOwnGearByTouch {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`You run your hands over your <ansi fg="item">%s</ansi>.`, res.TouchItem.DisplayNameFor(user.UserId)))
		return true, nil
	}
```

`internal/mobcommands/look.go:44`, a mob is silent on it as on every refusal:

```go
	case actions.LookBlind, actions.LookTooDark, actions.LookOwnGearByTouch, actions.LookExitTooDark, actions.LookExitLocked, actions.LookExitShapes:
```

Root guards (both fire on this change; observed in the dry run):

`bauble_finder_view_guard_test.go:78`:

```go
	"internal/usercommands/look.go|Look":                       {3, "what the looker reads about an item they carry, by sight or by touch in the dark (#218); the room lines beside them keep DisplayName"},
```

`bauble_sweep_guard_test.go`, in `transientItemHolders` after the `GiveItemResult` line:

```go
	`internal/actions.LookResolution`:            `a look's result, alive for one call (TouchItem, #218)`,
```

- [ ] **Step 4: Run, confirm green, root included**

Run: `go test . ./internal/usercommands/ ./internal/mobcommands/ ./internal/actions/ 2>&1 | grep -v "^time=" | tail -5`
Expected (observed): all `ok`. Before the two guard edits the root package failed with
`internal/usercommands/look.go|Look: makes 3 finder-view reference(s), finderViewSites says 2`
and `internal/actions.LookResolution holds an items.Item but no bauble sweep root reaches it`.

- [ ] **Step 5: Commit**

```bash
git add internal/actions/look.go internal/usercommands/look.go internal/mobcommands/look.go internal/usercommands/look_own_gear_touch_test.go bauble_finder_view_guard_test.go bauble_sweep_guard_test.go
git commit -m "fix(look): your own gear is known by touch in the dark (#218)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task D6: context.md for group D

**Files:** `modules/gmcp/context.md`, `internal/actions/context.md`, `internal/messaging/context.md`, `internal/events/context.md`

- [ ] **Step 1: `modules/gmcp/context.md`**, add a section after "Char.Sight: the player's light band":

```markdown
## Room.Info and Comm say follow sight (sight gates close-out, #252, 2026-10-08)

`GetRoomNode` (`gmcp.Room.go`) reads `messaging.ParticipantSight(user.Character, room)` once per
call, for every lane including the sub-module requests:

- **SightNone:** `name` and `description` are empty; `exits` and `exitsv2` are empty maps;
  `Contents.Items`, `Containers`, `Players` and `Npcs` are empty lists. Empty, not omitted, the
  Char.Enemies rule, so a merging client drops the last sighted reading. `num`, `area`,
  `environment`, `symbol`, `coords` and `details` stay: they place the player on a map they hold.
- **SightShapes:** the room reads, but every occupant is `unseenOccupant`: name `a figure`
  (`messaging.UnseenNoun`), empty `id` and `adjectives`, `aggro` kept.
- **Re-push on light:** `sightBandChangedHandler` (`gmcp.Room.go`) answers
  `events.SightBandChanged` with Room.Info, and, when the player now makes out the room,
  `actions.MarkRoomMappedIfSeen` plus a `GMCPZoneUpdate`. `gmcp.Char.go` answers the same event
  with Char.Sight; both listeners run.
- **Map (owner ruling R5):** `go.go` maps a room only through `actions.MarkRoomMappedIfSeen`, so
  a room walked in the dark stays off `Zone.Map` until seen; rooms already mapped stay.

`Comm.Channel` say goes through `sayDeliveries` (`gmcp.Comm.go`), one payload per listener,
matching the text lane (`actions.sendSpoken`): `sender` is the name at clear sight, `A figure`
at shapes, `Someone` at none or when `events.Communication.SpeakerHidden`; a player speaker's
words skip deafened listeners, an NPC's do not (ruling 6); a say with `TargetUserId` reaches
only that player. Other channels are unchanged.
```

- [ ] **Step 2: `internal/actions/context.md`**: in the `look.go` paragraph, change the
`LookKind` list to end `` `LookExitShapes`, `LookOther`, `LookOwnGearByTouch` `` and add after the
`#364` sentence: ``At `SightNone` a `lookAt` naming an item the looker wears or carries
(`Character.FindItem`) resolves to `LookOwnGearByTouch` with `LookResolution.TouchItem` (#218):
the player reads "You run your hands over your <item>." (name only), the mob is silent.``
Add a line for the new file: ``**`map_visit.go`** (#252): `MarkRoomMappedIfSeen(c, room) bool`
puts a room's template id on the fog-of-war map only when `ParticipantSight` is not
`SightNone`; the one writer of `Character.MarkRoomVisited` outside tests (`go.go`, and the GMCP
band-change handler).``

- [ ] **Step 3: `internal/messaging/context.md`**: after the `HideSpeakerNames` bullet add
``- `SpeakerNoun(d SightDecision) string`: the word `HideSpeakerNames` substitutes, "a figure"
at shapes and "someone" otherwise; exported for GMCP say's `sender` (#252).``

- [ ] **Step 4: `internal/events/context.md`**: in the `Communication` struct block add
`SpeakerHidden bool // a say spoken while still hidden (#252)` after `Message string`.

- [ ] **Step 5: Verify and commit**

Run: `python tools/context_md_audit.py`
Expected: no phantom symbols reported for the four files.

```bash
git add modules/gmcp/context.md internal/actions/context.md internal/messaging/context.md internal/events/context.md
git commit -m "docs(context): Room.Info and say follow sight, map only what is seen, look by touch (#252, #218)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

**Group D gate (dry run observed):** `go build ./...` clean; `go vet` on gmcp, actions,
usercommands, mobcommands clean; `go test . ./modules/gmcp/ ./internal/actions/ ./internal/usercommands/ ./internal/mobcommands/ ./internal/messaging/ ./internal/events/` all `ok`.

**Playtest notes for the closing playtest:** the web client receives an empty room `name` in
the dark; check it renders sensibly (the dry run did not exercise `webclient-pure.html`).
Light a torch inside a room entered in the dark and confirm the room panel fills in and the
map gains the room.

**Out of scope, seen in passing:** mob `sayto` (not hidden) sends
`<mob> says to <player>, "..."` through plain `room.SendText` (`internal/mobcommands/sayto.go:50`,
`:68`), so the text lane names both to shapes and dark observers. Same leak class as #382;
the group B or the closing playtest owner should pick it up.

---

## PR 1 gate and closing playtest

### Task P1: PR 1 gate

**Files:** none new.

- [ ] **Step 1: Full build, vet and suite, root package included**

```
go build ./...
go vet ./...
go test ./... -count=1 2>&1 | grep -v "^time=" | grep -E "^(--- FAIL|FAIL|panic)"
```
Expected: the third command prints nothing. On Windows the two `internal/rooms` zone lifecycle tests fail only under `DOGMUD_BOOT_SMOKE=1`; see `dogmud-writing-tests`.

- [ ] **Step 2: Race pass on the packages this PR touched**

```
go test -race ./internal/actions/... ./internal/messaging/... ./internal/rooms/... ./internal/usercommands/... ./internal/hooks/... ./modules/gmcp/... -count=1
```
Expected: all `ok`.

- [ ] **Step 3: context.md audit**

```
python tools/context_md_audit.py
```
Expected: no phantom symbols reported for `internal/actions`, `internal/messaging`, `internal/rooms`, `internal/events`, `modules/gmcp`.

- [ ] **Step 4: Boot check**

Follow `dogmud-shipping`'s detached-worktree boot check. The server must boot without a panic and with `SneakHearingMult` read from `config.yaml` (no validator warning). Kill only the PID you started.

- [ ] **Step 5: Code review**

Run `superpowers:requesting-code-review` over the branch diff against `origin/master`. Fix every Critical and Important finding in the branch before opening the PR.

- [ ] **Step 6: Push and open PR 1**

```
git push -u origin feat/sight-gates-closeout
gh pr create --repo pruuk/DOGMud --base master --head feat/sight-gates-closeout --title "fix(sight): sight-gates close-out, names in the dark (#382)"
```
Body: one line per task with its issue as `Refs #N` (never a closing keyword), the spec's dry-run amendments, the balance note (`SneakHearingMult` 0.75: a no-sight observer rolls `(Perception + Search*SkillWeight) * 0.75` instead of the old `* 0.80` sight floor; superhearing skips it), the deploy note (maps on old saves keep rooms walked in the dark before this ships), and the merge note with PR 2 (`NewRound_DoCombat_helpers.go`, `messaging_surface_guard_test.go`: different hunks and keys, textual rebase at most). End with the Claude Code line.

### Task P2: closing adversarial dark-room playtest (#382)

Runs after PR 1 and PR 2 are both merged. Load `dogmud-playtesting` first; it covers the harness location, the ephemeral goals file, `--checkout`, and never killing the owner's server.

- [ ] **Step 1: Goals file** (ephemeral, scratchpad). One goal per surface, each run at three sights: pitch dark (no light, no infravision), shapes (dim, 25 to 49), and blind (Blinded condition), plus one deafened listener:
  1. `attack <mob>` and `attack <player>`, bare `attack`, `attack nosuchname`.
  2. A mob caster's channel, a broken concentration, a fizzle; a player caster's broken concentration.
  3. A sneaker in the room and a sneaker arriving; observers at each sight; the sneaker's own line.
  4. A hidden mob with a greet and idle emote; a hidden player's `emote`.
  5. A scout mob next to a dark room and a hidden player.
  6. A player corpse: `look corpse`, `get all corpse`, `loot pass`, the corpse decaying.
  7. GMCP on: Room.Info in the dark and at shapes, `say` from a hidden speaker, a deafened listener, the map after walking a dark room then lighting a torch.
  8. `appraise` and `offer` in a dark shop; `look <own worn item>` in the dark.
  9. A mob `sayto` in the dark.
  10. Arrest into a holding cell: the cell is dim, `look` works, the arrest line names `fine`.
- [ ] **Step 2: Run** with `--checkout` on merged master. A combat fixture must survive several rounds or the run comes back partial.
- [ ] **Step 3: Triage.** Any name, roster or detail a reader below clear sight learns is a leak. File each as a GitHub issue on `pruuk/DOGMud` (search first; reports are gitignored). Fix leaks in a follow-up PR before closing.
- [ ] **Step 4: Close.** When a run finds no leak: close #214, #242, #246, #216, #254, #333, #215, #274, #251, #276, #435, #252, #272, #218, #298, #260, #219, #409 by hand with a comment naming the merge commit, tick the #382 checklist, and close #382.

---

## PR 2: Group E, content and copy (#298, #260, #219, #409)

Ships as its own PR on its own branch from `origin/master`, independent of PR 1.
Every task below was dry-run on `f50400bf7`: each new test was seen to FAIL on
master with the output quoted, then PASS after the change, and the full
`go build ./...` plus `go test ./...` was green at the end.

**Merge note with PR 1 (group A).** `internal/hooks/NewRound_DoCombat_helpers.go`
is edited by both PRs. This PR touches ONLY four string literals inside
`handlePlayerFoldCasting` (master lines 518, 527, 530, 534); group A touches the
mob sites (730 to 749, 830, 1220) and the player break at 1079. The hunks do not
overlap. Both PRs also edit `messaging_surface_guard_test.go`'s
`narrationViewpointRegistry`, on different keys (this PR rekeys the line-1280
"Your concentration shatters" entry, plus the `equip.go`, `get.go` and
`drop.go` entries). Whichever merges second rebases; expect at most a textual
conflict in that map, never a semantic one.

**Spec deviations, decided in the dry run:**
- #298 widened: the two static fallback cells (5105 Thornwall, 5106
  Stillwater, `holding_cell_room` in `factions/*.yaml`) are as dark by night as
  5107, so they get the same lamp. Their prose said "the only light comes from"
  a slit, so the prose now names the lamp too.
- #298: the "Type fine to see what you owe." line goes on the arrest line and
  the Jailed condition's `description:` (what `conditions` shows later), NOT on
  `start_actee`. `ExecuteArrest` prints `start_actee` and the arrest line back
  to back, so the spec's "both" would print the hint twice in a row.
- #219 widened by one: `internal/usercommands/jail.go:42` ("all but served —")
  is a seventh dash in a line this PR's area touches.
- #409 widened: `drop all <name>` (`internal/usercommands/drop.go:127`) is the
  mirror of the `get all` sweep and had the same "item(s)".

---

### Task E1: Holding cells carry a dim lamp (#298)

**Lamp value: 35.** Evidence: shipped `LightBlindBelow: 25`, `LightDimBelow: 50`
(`_datafiles/config.yaml` lines 945 to 946), so a normal observer reads shapes
at 25 to 49. The `dungeon` biome has `skylight: 0.0` and no lamp
(`biomes/dungeon.yaml`), so 5107's level is the lamp alone: 35. 35 is the
backstreet lamp that "is meant to leave it at shapes by night"
(`lighting_street_lamp_no_dip_test.go` header). The static cells (skylight 0.1)
read 35 at night and 36 at noon. Null probes run in the dry run: `lamp: 60`
fails the faces edge (`cell reads 60, at or above LightDimBelow 50`), `lamp: 20`
fails the floor (`reads 20, below LightBlindBelow 25`).

Instanced cells copy the lamp: `CreateEphemeralRoomIds`
(`internal/rooms/ephemeral.go:56`) clones each room with `LoadRoomTemplate`, a
fresh read of the YAML. `instance:"skip"` on `Room.Lamp` only keeps instance
saves from overriding it.

**Files:**
- Create: `jail_cell_content_test.go` (repo root, package `main`)
- Modify: `_datafiles/world/dogmud/rooms/instance_jail_cell/5107.yaml`
- Modify: `_datafiles/world/dogmud/rooms/stillwater/5106.yaml`
- Modify: `_datafiles/world/dogmud/rooms/thornwall_city/5105.yaml`
- Re-record: `testdata/lighting_parity.golden`, `testdata/lighting_daycycle.golden`, `testdata/lighting_balance_spread.golden`

- [ ] **Step 1: Write the failing test**

Create `jail_cell_content_test.go`:

```go
package main

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// TestJailedConditionNamesTheFineCommand is the second half of #298: the
// arrest line names `fine` once, at arrest. A prisoner who has scrolled past
// it finds the Jailed condition in `conditions`, whose description is the
// shipped condition 88's, so that description names `fine` too.
func TestJailedConditionNamesTheFineCommand(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	t.Cleanup(conditions.SeedConditionsForTest(nil))
	conditions.LoadDataFiles()

	spec := conditions.GetConditionSpec(88)
	if spec == nil || spec.Name != "Jailed" {
		t.Fatalf("condition 88 is %+v, want the shipped Jailed condition", spec)
	}
	if !spec.Listed() {
		t.Fatalf("Jailed is not listed, so `conditions` never shows its description")
	}
	desc := strings.Join(strings.Fields(spec.Description), " ")
	if !strings.Contains(desc, "Type fine to see what you owe.") {
		t.Errorf("Jailed description does not name the fine command: %q", desc)
	}
}

// TestJailCellsAreNeverPitchDark is the guard on #298: a prisoner could not
// even look around the cell, because every holding cell was a dungeon room
// with no lamp. 5107 is the template each arrest clones into a per-prisoner
// instance (internal/justice/arrest.go aCreateCellFn); 5105 and 5106 are the
// factions' static cells (holding_cell_room in factions/*.yaml), used when
// the instance cannot be made. Each carries a dim lamp of its own now.
//
// A normal-sighted prisoner must make out shapes (at or above
// LightBlindBelow) at every hour. The instanced cell has no sky at all and is
// also held below LightDimBelow: a cell is a dim room, not a lit one. The
// static cells keep their barred-slit sky (skylight 0.1), so only the floor
// is asserted there. The walk tries both sky extremes and both street-lamp
// states.
func TestJailCellsAreNeverPitchDark(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	// The real shipped config: a bare test binary uses Go default lighting
	// edges and walks the `default` fixture world.
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	rooms.LoadBiomeDataFiles()
	rooms.LoadDataFiles()

	lc := configs.GetLightingConfig()
	cells := []struct {
		roomId  int
		dimOnly bool
	}{
		{5107, true},  // instanced template: no sky, the lamp alone
		{5106, false}, // Stillwater static cell
		{5105, false}, // Thornwall static cell
	}
	for _, c := range cells {
		cell := rooms.LoadRoom(c.roomId)
		if cell == nil {
			t.Fatalf("room %d did not load: the walk is not seeing the world", c.roomId)
		}
		if cell.Lamp == nil {
			t.Errorf("room %d (%s) has no lamp: a prisoner reads pitch dark by night", cell.RoomId, cell.Title)
			continue
		}
		for _, celestial := range []float64{0, 100} {
			for _, lampsLit := range []bool{false, true} {
				level := cell.LightTermsAtForTest(celestial, lampsLit, 1).Level
				if level < lc.BlindBelow {
					t.Errorf("room %d celestial %v lampsLit %v: reads %d, below LightBlindBelow %d (pitch dark)",
						c.roomId, celestial, lampsLit, level, lc.BlindBelow)
				}
				if c.dimOnly && level >= lc.DimBelow {
					t.Errorf("room %d celestial %v lampsLit %v: reads %d, at or above LightDimBelow %d (faces, too bright for a cell)",
						c.roomId, celestial, lampsLit, level, lc.DimBelow)
				}
			}
		}
	}
}
```

(`TestJailedConditionNamesTheFineCommand` is Task E2's; it lives in this file
because it loads the same shipped data. Write the whole file now; that test
stays red until E2.)

- [ ] **Step 2: Run it to verify it fails**

Run: `go test -run "TestJailCellsAreNeverPitchDark" -count=1 . 2>&1 | grep -E "jail_cell_content|^(ok|FAIL)"`
Expected (dry run, before any YAML edit):
```
    jail_cell_content_test.go:...: room 5107 (A Holding Cell) has no lamp: a prisoner reads pitch dark by night
    jail_cell_content_test.go:...: room 5106 (Holding Cell) has no lamp: a prisoner reads pitch dark by night
    jail_cell_content_test.go:...: room 5105 (Holding Cell) has no lamp: a prisoner reads pitch dark by night
FAIL
```

- [ ] **Step 3: Give 5107 a lamp and say so in its prose**

In `_datafiles/world/dogmud/rooms/instance_jail_cell/5107.yaml`, add `lamp: 35`
under `biome: dungeon`, add one description sentence, and add a `lamp` noun.
The whole file becomes:

```yaml
roomid: 5107
zone: Instance Jail Cell
title: A Holding Cell
biome: dungeon
lamp: 35
description: >-
  Four close walls of cold, mortared stone press in around a single
  iron-strapped door with no handle on this side. A narrow pallet, a tin
  cup, and a barred slit too high to see through are the only furnishings.
  A small oil lamp gutters in a caged niche above the door, out of reach.
  There is no way out but the law's mercy and the slow passage of time.
nouns:
  door: A heavy iron-strapped door, barred from the far side.
  lamp: A small oil lamp behind an iron cage, set high above the door. It
    gives just enough light to make out the walls, and no more.
  pallet: A thin straw pallet against the wall.
```

(Keep any keys below `pallet:` in the real file unchanged; the dry run showed
none after it besides what is listed. Read the file first.)

- [ ] **Step 4: Give 5106 a lamp; its prose said the slit was "the only light"**

In `_datafiles/world/dogmud/rooms/stillwater/5106.yaml`, replace:
```yaml
  joints, and the only light comes from a narrow
  <ansi fg="itemname">window</ansi> slit set high near the ceiling, too
  small for anything but a wrist. A low wooden
```
with:
```yaml
  joints. A <ansi fg="itemname">lamp</ansi> hung outside the bars keeps
  the cell in a dim glow, and a narrow <ansi fg="itemname">window</ansi>
  slit set high near the ceiling is too small for anything but a wrist.
  A low wooden
```
Replace:
```yaml
biome: dungeon
skylight: 0.1
```
with:
```yaml
biome: dungeon
skylight: 0.1
lamp: 35
```
And replace:
```yaml
nouns:
  window: A slit of a window high in the stone
```
with:
```yaml
nouns:
  lamp: A small oil lamp on a hook outside the bars, kept low. It gives
    enough light to see the walls by, and the guards refill it each morning.
  window: A slit of a window high in the stone
```

- [ ] **Step 5: Give 5105 a lamp**

In `_datafiles/world/dogmud/rooms/thornwall_city/5105.yaml`, replace:
```yaml
  slit near the ceiling grudgingly admits whatever light the hour allows. The
  air is cool and close, smelling of damp stone and old straw. Footsteps and
```
with:
```yaml
  slit near the ceiling grudgingly admits whatever light the hour allows. A
  lamp on the steps outside the bars throws a dim glow over it all. The air
  is cool and close, smelling of damp stone and old straw. Footsteps and
```
Replace:
```yaml
biome: dungeon
skylight: 0.1
```
with:
```yaml
biome: dungeon
skylight: 0.1
lamp: 35
```
And replace:
```yaml
nouns:
  bars: Thick iron bars
```
with:
```yaml
nouns:
  lamp: A shuttered oil lamp on the steps beyond the bars, turned low. It
    lights the cell just enough to see the walls by, and no more.
  bars: Thick iron bars
```

Check every new noun line for an unquoted `: ` inside the text (the YAML colon
trap, `dogmud-authoring-content`): there is none.

- [ ] **Step 6: Run the test to verify it passes**

Run: `go test -run "TestJailCellsAreNeverPitchDark" -count=1 -v . 2>&1 | grep -E "^(--- |ok|FAIL)"`
Expected:
```
--- PASS: TestJailCellsAreNeverPitchDark (0.38s)
ok  	github.com/GoMudEngine/GoMud
```

- [ ] **Step 7: Null-probe both edges, then restore**

Set 5107 to `lamp: 60`, run Step 6's command: expect
`room 5107 celestial 0 lampsLit false: reads 60, at or above LightDimBelow 50`.
Set it to `lamp: 20`: expect `reads 20, below LightBlindBelow 25 (pitch dark)`.
Restore `lamp: 35` and confirm PASS again.

- [ ] **Step 8: Re-record the three lighting goldens**

The cells' light moved, so the three whole-world lighting goldens move with it.
Run first without the flags and confirm exactly these three fail:
`go test -count=1 . 2>&1 | grep -E "^--- FAIL"` expect
`TestLightingBalanceSpread`, `TestLightingDayCycleAcrossSampleRounds`,
`TestLightingParityAcrossEveryShippedRoom` (plus `TestJailedConditionNamesTheFineCommand`, which is E2's).

Then:
```
go test . -run "TestLightingParityAcrossEveryShippedRoom|TestLightingDayCycleAcrossSampleRounds|TestLightingBalanceSpread" -update-lighting-parity -update-lighting-daycycle -update-lighting-balance-spread -count=1
git diff -U0 -- testdata/ | grep "^[-+]room" | grep -v "room 510[567] "
```
Expected: the second command prints nothing (every changed `room` line is 5105,
5106 or 5107). Dry-run diffs: daycycle `room 5107 light=0` to `light=35`,
`room 5105 light=12` to `light=36`; parity `plain sight=none` to
`plain sight=shapes` for the three cells; balance spread `raw=3.000` to
`raw=35.076` for 5105/5106 and a new `room 5107 override ... level=35` row.

- [ ] **Step 9: Commit**

```bash
git add jail_cell_content_test.go _datafiles/world/dogmud/rooms/instance_jail_cell/5107.yaml _datafiles/world/dogmud/rooms/stillwater/5106.yaml _datafiles/world/dogmud/rooms/thornwall_city/5105.yaml testdata/lighting_parity.golden testdata/lighting_daycycle.golden testdata/lighting_balance_spread.golden
git commit -m "fix(content): holding cells carry a dim lamp so a prisoner can see (#298)

The instanced cell 5107 and the static cells 5105 and 5106 were dungeon
rooms with no lamp, pitch dark by night. Each gets lamp 35 (shapes for a
normal observer, the backstreet value) and prose that names it. Lighting
goldens re-recorded; only the three cells moved.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task E2: The arrest line and the Jailed condition name `fine` (#298)

**Files:**
- Modify: `internal/justice/arrest.go:414-421` (the arrest flavor line in `ExecuteArrest`)
- Modify: `_datafiles/world/dogmud/conditions/88-jailed.yaml:3-4`
- Modify: `condition_apply_path_guard_test.go:150` (line-number allowlist rekey)
- Test: `internal/justice/arrest_test.go` (append), `jail_cell_content_test.go` (written in E1)

- [ ] **Step 1: Write the failing test**

Append to `internal/justice/arrest_test.go` (it already imports `regexp`,
`strings`, `conditions`, `events`, `users`):

```go
// #298: a new prisoner was never told how to buy their way out. The arrest
// line names the `fine` command (which in turn names `payfine`), so the
// player learns the price and the way out from the line they read last.
func TestExecuteArrest_ArrestLineNamesTheFineCommand(t *testing.T) {
	const arrestedUserId = 8803

	origCell := cellRoomFn
	origMove := aMoveFn
	origDecay := aDecayFn
	origNow := bNowFn
	t.Cleanup(func() {
		cellRoomFn = origCell
		aMoveFn = origMove
		aDecayFn = origDecay
		bNowFn = origNow
	})
	aMoveFn = func(userId int, toRoomId int, isSpawn ...bool) error { return nil }
	cellRoomFn = func(faction string) int { return 5106 }
	aDecayFn = func() int { return 5 }
	bNowFn = func() uint64 { return 100 }

	restoreConditions := conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		jailedConditionId: {
			ConditionId:   jailedConditionId,
			Name:          "Jailed",
			Flags:         []conditions.Flag{conditions.SilentStart},
			TriggerCount:  1,
			RoundInterval: 1,
		},
	})
	defer restoreConditions()

	u := users.NewTestUser(arrestedUserId, "jailbird3", "Jailbird", 0)
	u.LineWidth = 80
	restoreUsers := users.SeedUsersForTest(map[int]*users.UserRecord{arrestedUserId: u})
	defer restoreUsers()
	events.DrainQueuedMessagesForTest(arrestedUserId)

	if ok := ExecuteArrest(u.Character, arrestedUserId, "stillwater_guards", false); !ok {
		t.Fatalf("ExecuteArrest should succeed when the faction has a cell")
	}

	tags := regexp.MustCompile(`<[^>]*>`)
	for _, msg := range events.DrainQueuedMessagesForTest(arrestedUserId) {
		if !strings.Contains(msg, "A guard seizes you") {
			continue
		}
		plain := strings.Join(strings.Fields(tags.ReplaceAllString(msg, "")), " ")
		if !strings.Contains(plain, "Type fine to see what you owe.") {
			t.Errorf("the arrest line does not name the fine command: %q", plain)
		}
		if !strings.Contains(msg, `<ansi fg="command">fine</ansi>`) {
			t.Errorf("the fine command is not marked as a command: %q", msg)
		}
		return
	}
	t.Fatalf("the arrested player never read the arrest line")
}
```

- [ ] **Step 2: Run both tests to verify they fail**

Run: `go test -run "TestExecuteArrest_ArrestLineNamesTheFineCommand" -count=1 ./internal/justice/`
Expected:
```
    arrest_test.go:...: the arrest line does not name the fine command: "A guard seizes you and hauls you to the holding cell. You have been placed under arrest by the stillwater_guards."
    arrest_test.go:...: the fine command is not marked as a command: ...
FAIL
```
Run: `go test -run "TestJailedConditionNamesTheFineCommand" -count=1 . 2>&1 | grep -E "jail_cell_content|^(ok|FAIL)"`
Expected:
```
    jail_cell_content_test.go:...: Jailed description does not name the fine command: "You are locked in a holding cell. You cannot leave until your sentence is served or your fine is paid."
FAIL
```

- [ ] **Step 3: Implement**

In `internal/justice/arrest.go`, replace:
```go
		// CategorySystem is never wrapped by the pipeline (it also carries
		// tables), so this two-sentence line wraps itself to the reader's
		// width; unwrapped it ran past 100 columns (#430).
		u.SendText(messaging.CategorySystem, messaging.WrapAnsi(
			fmt.Sprintf("A guard seizes you and hauls you to the holding cell. "+
				"You have been placed under arrest by the %s.", factionName),
			u.GetLineWidth()))
```
with:
```go
		// CategorySystem is never wrapped by the pipeline (it also carries
		// tables), so this line wraps itself to the reader's width; unwrapped
		// it ran past 100 columns (#430). It names `fine`, which names
		// `payfine`: a new prisoner otherwise had to guess the way out (#298).
		u.SendText(messaging.CategorySystem, messaging.WrapAnsi(
			fmt.Sprintf("A guard seizes you and hauls you to the holding cell. "+
				"You have been placed under arrest by the %s. "+
				`Type <ansi fg="command">fine</ansi> to see what you owe.`, factionName),
			u.GetLineWidth()))
```

In `_datafiles/world/dogmud/conditions/88-jailed.yaml`, replace:
```yaml
description: You are locked in a holding cell. You cannot leave until your
  sentence is served or your fine is paid.
```
with:
```yaml
description: You are locked in a holding cell. You cannot leave until your
  sentence is served or your fine is paid. Type fine to see what you owe.
```
Leave `start_actee` alone: `ExecuteArrest` prints it immediately before the
arrest line, so naming `fine` in both would print the hint twice in a row.

- [ ] **Step 4: Rekey the line-number allowlist**

The arrest.go edit adds two lines above `RestoreJailOnLogin`'s condition add,
which moves from line 643 to 645. In `condition_apply_path_guard_test.go:150`
replace `"internal/justice/arrest.go|643":` with
`"internal/justice/arrest.go|645":` (reason text unchanged). Confirm with
`grep -n "AddConditionScaled(jailedConditionId, float64(until-now))" internal/justice/arrest.go`, expect `645:`.

- [ ] **Step 5: Run to verify they pass**

```
go test -count=1 ./internal/justice/
go test -run "TestJailedConditionNamesTheFineCommand|TestPlayerConditionsTravelTheEventPath" -count=1 . 2>&1 | grep -E "^(--- |ok|FAIL)"
```
Expected: `ok  github.com/GoMudEngine/GoMud/internal/justice` (including the
existing #430 test `TestExecuteArrest_ArrestFlavorFitsTheLineWidth`, which still
holds every line to 80 columns with the third sentence added) and `ok` for the
root.

- [ ] **Step 6: Commit**

```bash
git add internal/justice/arrest.go internal/justice/arrest_test.go _datafiles/world/dogmud/conditions/88-jailed.yaml condition_apply_path_guard_test.go
git commit -m "fix(justice): the arrest line and the Jailed condition name fine (#298)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task E3: Putting on a light says it casts light (#260)

**Files:**
- Modify: `internal/conditions/conditionspec.go` (add `AnyLightSource` after `AnyDarknessSource`, master line 267)
- Modify: `internal/usercommands/equip.go:161-164` (the wearable branch)
- Modify: `messaging_surface_guard_test.go:1350` (registry key rekey)
- Create: `internal/usercommands/equip_light_line_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/usercommands/equip_light_line_test.go`:

```go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

const (
	equipLightTestCond  = 9771 // a carried light, literal strength 40
	equipLightTestTorch = 999975
	equipLightTestHat   = 999976
)

// equipLightFixture seeds a torch-shaped light-slot item that sheds light and
// a plain wearable that does not, and returns user 1 holding both.
func equipLightFixture(t *testing.T) *users.UserRecord {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		0: {SpeciesId: 0, Name: "human", Size: species.Medium},
	}))
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		equipLightTestCond: {ConditionId: equipLightTestCond, Name: "Test Torchlight", Secret: true, TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 40}}},
	}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		equipLightTestTorch: {ItemId: equipLightTestTorch, Name: "test torch", NameSimple: "torch",
			Type: items.Light, Subtype: items.Wearable, WornConditionIds: []int{equipLightTestCond}},
		equipLightTestHat: {ItemId: equipLightTestHat, Name: "test hat", NameSimple: "hat",
			Type: items.Head, Subtype: items.Wearable},
	}))
	user := users.GetByUserId(1)
	require.NotNil(t, user)
	user.Character.SpeciesId = 0
	user.Character.Stats.Strength.ValueAdj = 100
	user.Character.StoreItem(items.Item{ItemId: equipLightTestTorch})
	user.Character.StoreItem(items.Item{ItemId: equipLightTestHat})
	return user
}

// #260: `equip torch` said only "You wear your Torch." Putting on something
// that sheds light now says so; putting on anything else does not.
func TestEquippingALightSaysItCastsLight(t *testing.T) {
	user := equipLightFixture(t)
	_, room := getTestUserAndRoom(t)
	events.DrainQueuedMessagesForTest(user.UserId)

	_, err := Equip("test torch", user, room, 0)
	require.NoError(t, err)
	require.Equal(t, equipLightTestTorch, user.Character.Equipment.Light.ItemId, "fixture: the torch must be worn")
	got := hoodTestText(user.UserId)
	require.Contains(t, got, "You wear your")
	require.Contains(t, got, "It casts light around you.")

	_, err = Equip("test hat", user, room, 0)
	require.NoError(t, err)
	require.NotContains(t, hoodTestText(user.UserId), "casts light", "a hat sheds no light")
}

// AnyLightSource is the light twin of AnyDarknessSource: a darkness is never
// a light (lighting plan 5d, ruling D1), and an unknown id is skipped.
func TestAnyLightSource(t *testing.T) {
	equipLightFixture(t)
	require.True(t, conditions.AnyLightSource([]int{equipLightTestCond}))
	require.False(t, conditions.AnyLightSource(nil))
	require.False(t, conditions.AnyLightSource([]int{987654}), "an unknown id is not a light")
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go vet ./internal/usercommands/`
Expected: `equip_light_line_test.go:...: undefined: conditions.AnyLightSource`

- [ ] **Step 3: Add the predicate**

In `internal/conditions/conditionspec.go`, directly after `AnyDarknessSource`:

```go
// AnyLightSource reports whether any of the condition ids names a light
// source: an item whose worn conditions shed light says so as it goes on
// (#260). A darkness is never a light (IsLightSource). Unknown ids are
// skipped.
func AnyLightSource(conditionIds []int) bool {
	for _, id := range conditionIds {
		if spec := GetConditionSpec(id); spec != nil && spec.IsLightSource() {
			return true
		}
	}
	return false
}
```

Run: `go test -run "TestEquippingALightSaysItCastsLight|TestAnyLightSource" -count=1 ./internal/usercommands/`
Expected (the predicate exists, the line does not yet):
```
--- FAIL: TestEquippingALightSaysItCastsLight
        Error: "<ansi fg=\"system\">You wear your <ansi fg=\"item\">Test Torch</ansi>.</ansi>\n\n" does not contain "It casts light around you."
```

- [ ] **Step 4: Say it in the equip line**

In `internal/usercommands/equip.go`, replace:
```go
			} else if result.Item.GetSpec().Subtype == items.Wearable {
				user.SendText(messaging.CategorySystem,
					fmt.Sprintf(`You wear your <ansi fg="item">%s</ansi>.`, result.Item.DisplayName()),
				)
```
with:
```go
			} else if result.Item.GetSpec().Subtype == items.Wearable {
				// A light says it is lit as it goes on (#260); the light
				// notice that may follow speaks of the room, not the item.
				lightNote := ``
				if conditions.AnyLightSource(result.Item.GetSpec().WornConditionIds) {
					lightNote = ` It casts light around you.`
				}
				user.SendText(messaging.CategorySystem,
					fmt.Sprintf(`You wear your <ansi fg="item">%s</ansi>.%s`, result.Item.DisplayName(), lightNote),
				)
```
Keep the Sprintf literal inside the `SendText` call: the narration guard
fingerprints the literal at the call (`TestNarrationSitesMatchViewpointAudit`).
A `wearLine` variable lifted out of the call was tried in the dry run and the
guard lost the site.

- [ ] **Step 5: Rekey the narration registry**

In `messaging_surface_guard_test.go:1350` replace the key
`"usercommands/equip.go|You wear your <ansi fg=\"item\">%s</ansi>."` with
`"usercommands/equip.go|You wear your <ansi fg=\"item\">%s</ansi>.%s"` and append
to its reason text: `; the trailing %s is the #260 light note, still the wearer's own line`.

- [ ] **Step 6: Run to verify it passes**

```
go test -count=1 ./internal/usercommands/ ./internal/conditions/
go test -run "TestNarrationSitesMatchViewpointAudit" -count=1 . 2>&1 | grep -E "^(--- |ok|FAIL)"
```
Expected: `ok` for all three.

- [ ] **Step 7: Commit**

```bash
git add internal/conditions/conditionspec.go internal/usercommands/equip.go internal/usercommands/equip_light_line_test.go messaging_surface_guard_test.go
git commit -m "fix(equip): putting on a light says it casts light (#260)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task E4: An empty `say` is refused (#260)

**Files:**
- Modify: `internal/usercommands/say.go` (guard after the muted check, before `drunkify`)
- Create: `internal/usercommands/say_empty_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/usercommands/say_empty_test.go` (uses `speechWrapperScene`
and `speechWrapperHeard` from `speech_sight_wrapper_test.go`):

```go
package usercommands

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// #260: an empty `say` printed `You say, ""` and spoke a blank line to the
// room. It is refused now, and nobody else hears anything. Whitespace alone
// counts as empty.
func TestSay_EmptyIsRefused(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	for _, rest := range []string{"", "   "} {
		alice, _, room := speechWrapperScene(t)
		handled, err := Say(rest, alice, room, 0)
		require.NoError(t, err)
		require.True(t, handled)
		require.Equal(t, []string{"Say what?"}, speechWrapperHeard(1), "rest %q", rest)
		require.Empty(t, speechWrapperHeard(2), "rest %q: the room heard an empty say", rest)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test -run "TestSay_EmptyIsRefused" -count=1 ./internal/usercommands/`
Expected:
```
        Error:      	Not equal:
        	            	expected: []string{"Say what?"}
        	            	actual  : []string{"You say, \"\""}
FAIL
```

- [ ] **Step 3: Implement**

In `internal/usercommands/say.go`, replace:
```go
	if user.Character.HasConditionFlag(conditions.Drunk) {
		rest = drunkify(rest)
	}
```
with:
```go
	// Nothing to say: refuse rather than speak a blank line to the room
	// (#260).
	if strings.TrimSpace(rest) == `` {
		user.SendText(messaging.CategorySystem, `Say what?`)
		return true, nil
	}

	if user.Character.HasConditionFlag(conditions.Drunk) {
		rest = drunkify(rest)
	}
```
(`strings` is already imported for `drunkify`.)

- [ ] **Step 4: Run to verify it passes**

Run: `go test -run "TestSay" -count=1 ./internal/usercommands/`
Expected: `ok`

- [ ] **Step 5: Commit**

```bash
git add internal/usercommands/say.go internal/usercommands/say_empty_test.go
git commit -m "fix(say): an empty say is refused, not spoken to the room (#260)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task E5: `equipment` is an alias of `inventory` (#260)

**Files:**
- Modify: `_datafiles/world/dogmud/keywords.yaml:319`
- Create: `internal/keywords/keywords_equipment_alias_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/keywords/keywords_equipment_alias_test.go`:

```go
package keywords

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/stretchr/testify/require"
)

// TestEquipmentIsInventory loads the shipped DOGMud keywords.yaml and proves
// `equipment` reaches `inventory`, whose listing shows what is worn (#260: a
// new player typed `equipment` and was told it was not a command, while the
// short `eq` already worked).
func TestEquipmentIsInventory(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)

	origKeywords := loadedKeywords
	defer func() { loadedKeywords = origKeywords }()

	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud"))
	configs.SetConfigForTest(t, cfg)

	LoadAliases()

	require.Equal(t, "inventory", TryCommandAlias("eq"), "fixture: `eq` must already alias inventory")
	require.Equal(t, "inventory", TryCommandAlias("equipment"),
		"the dogmud keywords.yaml must alias `equipment` to `inventory`")
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test -run "TestEquipmentIsInventory" -count=1 ./internal/keywords/`
Expected:
```
        	            	expected: "inventory"
        	            	actual  : "equipment"
FAIL
```

- [ ] **Step 3: Implement**

In `_datafiles/world/dogmud/keywords.yaml:319` replace
`  inventory:          ['i', 'inv', 'eq']` with
`  inventory:          ['i', 'inv', 'eq', 'equipment']`.
(No command or alias named `equipment` exists today; checked with
`grep -rn '"equipment":' internal/usercommands/` and the keywords file.)

- [ ] **Step 4: Run to verify it passes**

Run: `go test -count=1 ./internal/keywords/`
Expected: `ok`

- [ ] **Step 5: Commit**

```bash
git add _datafiles/world/dogmud/keywords.yaml internal/keywords/keywords_equipment_alias_test.go
git commit -m "fix(keywords): equipment is an alias of inventory (#260)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task E6: No dashes in the player lines this PR touches (#219)

**Files:**
- Create: `copy_no_dash_test.go` (repo root, package `main`)
- Modify: `internal/usercommands/stand.go:37`, `internal/usercommands/eat.go:22`, `internal/usercommands/jail.go:42`
- Modify: `internal/hooks/NewRound_DoCombat_helpers.go:518, 527, 530, 534` (string literals ONLY; see the merge note)
- Modify: `messaging_surface_guard_test.go:1280` (registry key rekey)

- [ ] **Step 1: Write the failing guard**

Create `copy_no_dash_test.go`. The file list includes every file this PR edits
that carries player copy, so a later line there cannot bring a dash back:

```go
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// noDashCopyFiles are the source files whose string literals must carry no
// em or en dash (#219, #383): player copy uses commas, colons and full stops.
// Comments may keep dashes. The list is the files the #219 fixes touched plus
// the other files the same copy PR edits, so a line added there later cannot
// bring a dash back. Widen it file by file as the #383 sweep reaches more.
var noDashCopyFiles = []string{
	"internal/usercommands/stand.go",
	"internal/usercommands/eat.go",
	"internal/usercommands/jail.go",
	"internal/usercommands/say.go",
	"internal/usercommands/get.go",
	"internal/usercommands/drop.go",
	"internal/usercommands/equip.go",
	"internal/hooks/NewRound_DoCombat_helpers.go",
}

func TestCopyFilesHaveNoDashesInStringLiterals(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Dir(here)
	fset := token.NewFileSet()
	for _, rel := range noDashCopyFiles {
		file, err := parser.ParseFile(fset, filepath.Join(root, filepath.FromSlash(rel)), nil, 0)
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		scanned := 0
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			scanned++
			if strings.ContainsAny(lit.Value, "—–") {
				t.Errorf("%s: string literal carries a dash: %s", fset.Position(lit.Pos()), lit.Value)
			}
			return true
		})
		if scanned == 0 {
			t.Errorf("%s: scanned no string literals; the walk is not reaching the file", rel)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test -run TestCopyFilesHaveNoDashesInStringLiterals -count=1 . 2>&1 | grep -E "copy_no_dash|string literal|^(ok|FAIL)"`
Expected (dry run, exactly seven hits):
```
internal\usercommands\stand.go:37:43: string literal carries a dash: "You're locked in a grapple — you'll need to break free first."
internal\usercommands\eat.go:22:43: string literal carries a dash: `<ansi fg="red">Your hands are committed to the grapple — you can't reach for that.</ansi>`
internal\usercommands\jail.go:42:43: string literal carries a dash: "Your sentence is all but served — you'll be released shortly."
internal\hooks\NewRound_DoCombat_helpers.go:518:52: ... Your concentration shatters — you cannot hold the fold while grappled!
internal\hooks\NewRound_DoCombat_helpers.go:527:52: ... Your spell fizzles — the target is gone.
internal\hooks\NewRound_DoCombat_helpers.go:530:52: ... The spell dissipates — its data cannot be found.
internal\hooks\NewRound_DoCombat_helpers.go:534:52: ... Your conviction wavers — the fold collapses.
FAIL
```
(If E7 has not run yet, `drop.go` and `get.go` have no dashes in literals
either way; only comments carry them.)

- [ ] **Step 3: Replace each dash (one literal per edit, nothing else on the line)**

| File:line | Old literal text | New literal text |
|---|---|---|
| `stand.go:37` | `You're locked in a grapple — you'll need to break free first.` | `You're locked in a grapple. You'll need to break free first.` |
| `eat.go:22` | `Your hands are committed to the grapple — you can't reach for that.` | `Your hands are committed to the grapple, so you can't reach for that.` |
| `jail.go:42` | `Your sentence is all but served — you'll be released shortly.` | `Your sentence is all but served. You'll be released shortly.` |
| `NewRound_DoCombat_helpers.go:518` | `Your concentration shatters — you cannot hold the fold while grappled!` | `Your concentration shatters. You cannot hold the fold while grappled!` |
| `NewRound_DoCombat_helpers.go:527` | `Your spell fizzles — the target is gone.` | `Your spell fizzles. The target is gone.` |
| `NewRound_DoCombat_helpers.go:530` | `The spell dissipates — its data cannot be found.` | `The spell dissipates. Its data cannot be found.` |
| `NewRound_DoCombat_helpers.go:534` | `Your conviction wavers — the fold collapses.` | `Your conviction wavers, and the fold collapses.` |

Use the Edit tool with the old text as `old_string`; surrounding ANSI tags and
quoting stay exactly as they are.

- [ ] **Step 4: Rekey the narration registry**

The guard fingerprints a site by its literal's first 80 characters. In
`messaging_surface_guard_test.go:1280` replace the key
`"hooks/NewRound_DoCombat_helpers.go|<ansi fg=\"red\">Your concentration shatters — you cannot hold the fold while grap"`
with
`"hooks/NewRound_DoCombat_helpers.go|<ansi fg=\"red\">Your concentration shatters. You cannot hold the fold while grapp"`
(reason text unchanged). The dry run's guard output named exactly that new
fingerprint as unregistered and the old one as stale.

- [ ] **Step 5: Run to verify it passes**

```
go test -run "TestCopyFilesHaveNoDashesInStringLiterals|TestNarrationSitesMatchViewpointAudit" -count=1 . 2>&1 | grep -E "^(--- |ok|FAIL)"
grep -rn "fizzles — the target\|concentration shatters —\|conviction wavers —\|dissipates —\|locked in a grapple —\|committed to the grapple —\|all but served —" --include=*_test.go .
```
Expected: `ok`; the grep prints nothing (no test pins an old literal).

- [ ] **Step 6: Commit**

```bash
git add copy_no_dash_test.go internal/usercommands/stand.go internal/usercommands/eat.go internal/usercommands/jail.go internal/hooks/NewRound_DoCombat_helpers.go messaging_surface_guard_test.go
git commit -m "fix(copy): no dashes in grapple, jail and spell disruption lines (#219)

Adds a literal-scanning guard over the files the copy PR touches.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task E7: `get all` replies on an empty floor; counts say "item" or "items" (#409)

**Files:**
- Modify: `internal/usercommands/get.go:84` (`getAllMatchingFromFloor` summary), `:133` (component bag), `:229-250` (bare `get all` floor sweep); add `itemCount` above `func Get`
- Modify: `internal/usercommands/drop.go:127` (`drop all <name>`, the sweep's mirror)
- Modify: `internal/usercommands/get_sight_gates_test.go:30` (pins the old text)
- Modify: `messaging_surface_guard_test.go:1342` and `:1354` (registry key rekeys)
- Create: `internal/usercommands/get_all_copy_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/usercommands/get_all_copy_test.go` (uses `seedFixtureRoom`,
`sentTo` and `pebbleCmdItemId` from `get_fixture_test.go`):

```go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #409: a bare `get all` on a floor with nothing to take printed nothing at
// all. A fixture is not something to take, so a floor holding only the Arch
// Lantern counts as empty.
func TestGetAllOnAnEmptyFloorSaysSo(t *testing.T) {
	user, room := seedFixtureRoom(t)
	Get("pebble", user, room, 0)
	require.Len(t, user.Character.Items, 1, "fixture: the pebble is taken first")
	sentTo(user)
	room.Gold = 0

	_, err := Get("all", user, room, 0)
	require.NoError(t, err)
	assert.Contains(t, sentTo(user), "There is nothing here to pick up.")
}

// A floor with something on it does not get the empty line.
func TestGetAllWithSomethingToTakeIsNotEmpty(t *testing.T) {
	user, room := seedFixtureRoom(t)
	room.Gold = 0
	_, err := Get("all", user, room, 0)
	require.NoError(t, err)
	assert.NotContains(t, sentTo(user), "nothing here to pick up")
}

// #409: "You pick up 1 item(s)." read machine-made. The count picks the noun.
func TestGetAllNamedCountsItemsInWords(t *testing.T) {
	user, room := seedFixtureRoom(t)
	_, err := Get("all pebble", user, room, 0)
	require.NoError(t, err)
	out := sentTo(user)
	assert.Contains(t, out, "You pick up 1 item.")
	assert.NotContains(t, out, "item(s)")

	two := []items.Item{items.New(pebbleCmdItemId), items.New(pebbleCmdItemId)}
	for _, p := range two {
		room.AddItem(p, false)
	}
	_, err = Get("all pebble", user, room, 0)
	require.NoError(t, err)
	out = sentTo(user)
	assert.Contains(t, out, "You pick up 2 items.")
	assert.NotContains(t, out, "item(s)")
}

// `drop all <name>` is the sweep's mirror and counts the same way.
func TestDropAllNamedCountsItemsInWords(t *testing.T) {
	user, room := seedFixtureRoom(t)
	user.Character.Items = []items.Item{items.New(pebbleCmdItemId)}
	_, err := Drop("all pebble", user, room, 0)
	require.NoError(t, err)
	out := sentTo(user)
	assert.Contains(t, out, "You drop 1 item.")
	assert.NotContains(t, out, "item(s)")

	user.Character.Items = []items.Item{items.New(pebbleCmdItemId), items.New(pebbleCmdItemId)}
	_, err = Drop("all pebble", user, room, 0)
	require.NoError(t, err)
	assert.Contains(t, sentTo(user), "You drop 2 items.")
}

// The component-bag sweep says the same in words.
func TestGetAllBagCountsItemsInWords(t *testing.T) {
	user, room := seedFixtureRoom(t)
	origComp := user.Character.ComponentItems
	t.Cleanup(func() { user.Character.ComponentItems = origComp })

	user.Character.ComponentItems = []items.Item{items.New(pebbleCmdItemId)}
	_, err := Get("all bag", user, room, 0)
	require.NoError(t, err)
	assert.Contains(t, sentTo(user), "You move 1 item from your component bag to your backpack.")

	user.Character.ComponentItems = []items.Item{items.New(pebbleCmdItemId), items.New(pebbleCmdItemId)}
	_, err = Get("all bag", user, room, 0)
	require.NoError(t, err)
	assert.Contains(t, sentTo(user), "You move 2 items from your component bag to your backpack.")
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test -run "TestGetAll|TestDropAll" -count=1 -v ./internal/usercommands/ 2>&1 | grep -E "^(--- |ok|FAIL)|Error:"`
Expected (dry run):
```
        Error: "" does not contain "There is nothing here to pick up."
--- FAIL: TestGetAllOnAnEmptyFloorSaysSo
--- PASS: TestGetAllWithSomethingToTakeIsNotEmpty
        Error: "<ansi fg=\"system\">You pick up 1 item(s).</ansi>\n" does not contain "You pick up 1 item."
--- FAIL: TestGetAllNamedCountsItemsInWords
        Error: "<ansi fg=\"system\">You drop 1 item(s).</ansi>\n" does not contain "You drop 1 item."
--- FAIL: TestDropAllNamedCountsItemsInWords
        Error: "<ansi fg=\"system\">You move 1 item(s) from your component bag to your backpack.</ansi>\n" does not contain ...
--- FAIL: TestGetAllBagCountsItemsInWords
```

- [ ] **Step 3: Add `itemCount` and use it**

In `internal/usercommands/get.go`, directly above `func Get(`:
```go
// itemCount words a count of items for a sweep's summary line: "1 item",
// "2 items" (#409: "item(s)" read machine-made).
func itemCount(n int) string {
	if n == 1 {
		return `1 item`
	}
	return fmt.Sprintf(`%d items`, n)
}
```
Replace `fmt.Sprintf(`You pick up %d item(s).`, picked)` with
`fmt.Sprintf(`You pick up %s.`, itemCount(picked))`.
Replace `fmt.Sprintf(`You move %d item(s) from your component bag to your backpack.`, ct)` with
`fmt.Sprintf(`You move %s from your component bag to your backpack.`, itemCount(ct))`.

In `internal/usercommands/drop.go:127` replace
`fmt.Sprintf(`You drop %d item(s).`, dropped)` with
`fmt.Sprintf(`You drop %s.`, itemCount(dropped))`.

- [ ] **Step 4: Reply on an empty floor**

In `internal/usercommands/get.go`, replace the bare sweep:
```go
		// get all — grab everything from the floor
		if room.Gold > 0 {
			Get(`gold`, user, room, flags)
		}

		if len(room.Items) > 0 {
			iCopies := append([]items.Item{}, room.Items...)

			for _, item := range iCopies {
				// A fixture is part of the room: a sweep passes it without a
				// word (lighting 5e).
				if item.IsFixture() {
					continue
				}
				// Never by accident: see getAllMatchingFromFloor.
				if item.BaubleBelongsTo(room.RoomId) {
					leaveHouseholdBauble(user, item)
					continue
				}
				Get(item.Name(), user, room, flags)
			}
		}

		return true, nil
```
with:
```go
		// get all — grab everything from the floor
		// offered counts what the sweep answered for, so a floor with
		// nothing to take still gets a reply (#409).
		offered := 0
		if room.Gold > 0 {
			offered++
			Get(`gold`, user, room, flags)
		}

		if len(room.Items) > 0 {
			iCopies := append([]items.Item{}, room.Items...)

			for _, item := range iCopies {
				// A fixture is part of the room: a sweep passes it without a
				// word (lighting 5e).
				if item.IsFixture() {
					continue
				}
				offered++
				// Never by accident: see getAllMatchingFromFloor.
				if item.BaubleBelongsTo(room.RoomId) {
					leaveHouseholdBauble(user, item)
					continue
				}
				Get(item.Name(), user, room, flags)
			}
		}

		if offered == 0 {
			user.SendText(messaging.CategorySystem, `There is nothing here to pick up.`)
		}

		return true, nil
```
(The `—` in the first comment line is pre-existing and in a comment, which the
E6 guard allows.)

- [ ] **Step 5: Update the pinned test and the registry**

`internal/usercommands/get_sight_gates_test.go:30`: replace
`assert.Contains(t, out, "You pick up 1 item(s).")` with
`assert.Contains(t, out, "You pick up 1 item.")`.

`messaging_surface_guard_test.go`: replace the key
`"usercommands/drop.go|You drop %d item(s)."` with `"usercommands/drop.go|You drop %s."`
(line 1342) and `"usercommands/get.go|You pick up %d item(s)."` with
`"usercommands/get.go|You pick up %s."` (line 1354). Reason texts unchanged.

- [ ] **Step 6: Run to verify they pass**

```
go test -count=1 ./internal/usercommands/
go test -run "TestNarrationSitesMatchViewpointAudit|TestCopyFilesHaveNoDashesInStringLiterals" -count=1 . 2>&1 | grep -E "^(--- |ok|FAIL)"
```
Expected: `ok` for both.

- [ ] **Step 7: Commit**

```bash
git add internal/usercommands/get.go internal/usercommands/drop.go internal/usercommands/get_all_copy_test.go internal/usercommands/get_sight_gates_test.go messaging_surface_guard_test.go
git commit -m "fix(get): get all answers an empty floor; sweeps count item or items (#409)

drop all <name>, the sweep's mirror, had the same item(s) and is fixed
with it.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task E8: A carried light's first glow never says "again" (#409)

**Files:**
- Modify: `_datafiles/world/dogmud/narration/light-notices/carried.yaml:21-22`
- Modify: `internal/lightnotice/store_test.go` (append)
- Re-record: `internal/narration/testdata/stores/light_notices.golden`

- [ ] **Step 1: Write the failing test**

Append to `internal/lightnotice/store_test.go` (imports `strings` already):

```go
// #409: a lantern lit in a cave the player had just walked into said "you can
// make out faces again", though they never saw faces there. A carried light
// coming up is often the first light a player has in that room, so its
// lighter lines never say "again".
func TestCarriedLighterLinesNeverSayAgain(t *testing.T) {
	t.Cleanup(ResetForTest)
	if err := LoadFrom(shippedDir); err != nil {
		t.Fatalf("shipped light notices refused: %v", err)
	}
	for _, tr := range []Transition{LighterShapes, LighterFaces} {
		for _, indoor := range []bool{false, true} {
			lines := Pool(CauseCarried, tr, indoor)
			if len(lines) == 0 {
				t.Fatalf("carried %s indoor=%v: no lines; the walk tested nothing", tr, indoor)
			}
			for _, l := range lines {
				if strings.Contains(strings.ToLower(l), "again") {
					t.Errorf("carried %s line says again: %q", tr, l)
				}
			}
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test -run TestCarriedLighterLinesNeverSayAgain -count=1 ./internal/lightnotice/`
Expected:
```
    store_test.go:...: carried lighter_faces line says again: "A carried light spreads around you; faces are clear again."
    store_test.go:...: carried lighter_faces line says again: "By the carried light, you can make out faces again."
FAIL
```

- [ ] **Step 3: Implement**

In `_datafiles/world/dogmud/narration/light-notices/carried.yaml`, replace:
```yaml
      - 'A carried light spreads around you; faces are clear again.'
      - 'By the carried light, you can make out faces again.'
```
with:
```yaml
      - 'A carried light spreads around you; faces are clear now.'
      - 'By the carried light, you can make out faces.'
```
Only `carried` changes. `movement`, `lamp`, `weather` and `sky` keep "again":
each of those follows a darker state the player was actually in.

- [ ] **Step 4: Run, then re-record the narration snapshot**

```
go test -count=1 ./internal/lightnotice/
go test -count=1 ./internal/narration/ 2>&1 | grep -E "^(--- |ok|FAIL)"
```
Expected: lightnotice `ok`; narration `--- FAIL: TestSnapshotStores`. Then:
```
go test -count=1 ./internal/narration/ -update
git diff --stat -- internal/narration/testdata/
go test -count=1 ./internal/narration/
```
Expected: only `light_notices.golden | 4 ++--`, then `ok`.

- [ ] **Step 5: Commit**

```bash
git add _datafiles/world/dogmud/narration/light-notices/carried.yaml internal/lightnotice/store_test.go internal/narration/testdata/stores/light_notices.golden
git commit -m "fix(narration): a carried light's first glow does not say again (#409)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task E9: PR 2 gate

- [ ] **Step 1: Full build and suite, root package included**

```
go build ./...
go test ./... 2>&1 | grep -v "^time=" | grep -E "^(--- FAIL|FAIL|panic)"
```
Expected: the second command prints nothing. (Dry run: clean.) On Windows the
two `internal/rooms` zone lifecycle tests fail only under
`DOGMUD_BOOT_SMOKE=1`; see `dogmud-writing-tests`.

- [ ] **Step 2: Boot check**

Follow `dogmud-shipping`'s detached-worktree boot check: the three room files
and condition 88 must load without a panic (YAML colon trap, room reciprocity
untouched). Kill only the PID you started.

- [ ] **Step 3: Copy check**

Every new player line is under 80 columns: "You wear your <item>. It casts
light around you." fits for item names up to 34 characters; the arrest line is
wrapped by `WrapAnsi` and held to 80 by `TestExecuteArrest_ArrestFlavorFitsTheLineWidth`;
the condition description goes through the `conditions` template's
`splitstring ... 58`.

- [ ] **Step 4: Push and open PR 2**

`git push -u origin <branch>`, then
`gh pr create --repo pruuk/DOGMud --base master` with a body listing #298,
#260, #219, #409 as `Refs` (not closing keywords, per the PR-body trap; close
them by hand after merge once the playtest confirms), the spec deviations
above, and the merge note about `NewRound_DoCombat_helpers.go` and
`messaging_surface_guard_test.go`. End the body with the Claude Code line.

**Out of scope, noted for #383:** `(s)` plurals remain in
`hooks/StorageFee_MonthlyCharge.go:67, 130` ("slot(s)"),
`items/items.go:300` ("round(s)") and `usercommands/admin.ai.go:88`; roughly
60 more string lines with dashes remain across `internal/usercommands` and
`internal/hooks` outside the files E6 guards.
