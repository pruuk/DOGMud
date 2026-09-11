# Follow-up Slice A: Names in the Dark Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A player who cannot see is never told a name, cannot aim a spell at what they cannot see, and cannot name a hidden creature they do not perceive.

**Architecture:** `internal/messaging` gains a participant sight predicate and a name-hiding routine; the M2 narration seam (`SendTrio`) uses them to render each role for its own reader. The counter, crit and spell lines move onto that seam. `characters.Character.Perceives` becomes the one hidden-creature rule, read by the room listing and by a viewer-aware `Room.FindByNameSeenBy` that every player command passes. `actions.InitiateCast` refuses targeted casts without sight, and a player's `search` ends a found hider's hiding.

**Tech Stack:** Go 1.x, `testify` (`assert`, `require`), `go/ast` root guards, the playtest harness (`playtestrun`, `mudagent`).

**Spec:** `docs/superpowers/specs/2026-09-11-followup-slice-a-names-in-dark-design.md` (owner rulings 1 to 10).

---

## Facts verified against source, 2026-09-11

Read from branch `feature/followup-slice-a-names-in-dark` at `451cc3a03` (master `62b326bfe` plus the spec). The spec's own fact table covers the behaviour; these are the facts the tasks below depend on.

| Fact | Evidence |
|---|---|
| `Audience` literals: 30 in 26 files (salvage 1, mobcommands 15, usercommands 14) | grep `messaging\.Audience{`, non-test |
| `m2FrozenFiles` hashes every STRING LITERAL in 25 special-move files; adding identifier fields changes nothing, adding `""` would | `messaging_surface_guard_test.go:1403-1450` |
| `TestM2RoutingIsFrozen` freezes `SendText`/`SendTextVisual` calls in the same 25 files, not Audience literals | `m2_routing_guard_test.go:70-96`, `:280` |
| The narration registry has no rows for the cross-cast, counter or crit sites; complete three-role events are not candidates | `messaging_surface_guard_test.go:1004-1024`; grep of registry keys |
| Root test package is `main` | `m2_routing_guard_test.go:1` |
| 56 non-test lookup calls (`FindByName`, `ResolveTargetActor`, `FindAttackTarget`) with enclosing functions, listed in Task 10 | AST inventory script, 2026-09-11 |
| `findPlayerByName` / `findMobByName` are also called by `respawn_targeting_test.go:221` and `stale_mob_ids_test.go:36`, `:100` | grep |
| `FindAttackTarget` is called by 5 tests in `internal/actions/combat_test.go:639-712` | grep |
| `playerExcludeIds` has two other callers, so it stays | `NewRound_DoCombat_unified.go:467`, `:603` |
| `ExecuteCounter` fills messages through `fillCounterMessages(&result, defender, attacker)` only | `internal/combat/counter.go:128` |
| `genericDefenceTriad` renders `You withstand %s's %s.` / `%s withstands your %s.` / `%s withstands %s's %s.` when the pool is not loaded | `internal/combat/defence_multiplier.go:380-389` |
| Hooks test fixture: users 1 `Aliceia` and 2 `Bobrick`, mob 100 `Skeleton`, all in room 1 (biome `city`); biomes `city`, `cave`, `default` seeded | `internal/hooks/hooks_test.go:37-175` |
| Hooks test helpers: `darken(t, roomId)`, `drainPlain(uid)`, `seedNarrationBuffs()` (with `heatEyesBuffId`, `nightEyesBuffId`), `countContaining` (case-insensitive), `spellContestAttackWin()`, `attackWinContest(t)`, `pinCounterTierKnobs(t, pct)`, `vbLandingResult()` | `narration_testhelpers_test.go`, `spell_collapse_test.go:35`, `counter_tier_test.go:30`, `:165` |
| `users.NewTestUser(id, username, charName, connId)` builds `Buffs`, `Awareness`, `Position`, `CombatPhase` | `internal/users/test_helpers.go:56-80` |
| A hidden fixture is made with `Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason)` then `ResolveConcealment(true, reason)` | `internal/actions/search_stealth_test.go:23-33` |
| `Room.GetVisibility` reads the biome registry: `DarkArea` gives 0, `LitArea` keeps at least 1; an unseeded biome falls back to `default` | `internal/rooms/rooms.go:144-176`, `:2860-2875`; `biomes.go:50` |
| `(c *Character) AddBuff(id, perma) error`; `Buffs.AddBuff(id, perma) bool` | `predicates_test.go:95`; `buff_room_text_test.go:77` |
| `CooldownReady(tag) bool` is the read-only cooldown query; casts take the `special-move` key | `internal/characters/cooldowns.go:76`; `internal/actions/cast.go:316` |
| No test uses a player searcher: the player branch of `Search` needs `rooms.GetDetails` with a real user | `internal/actions/search_test.go`; `roomdetails.go:42` |
| `util.GetMatchNumber("shape")` is `("shape", 1)`; `"2.shape"` is 2; `"shape#2"` is 2; `"all.shape"` is -1 | `internal/util/util.go:315-350` |
| `findPlayerByName` resolves `@<userId>` and `findMobByName` resolves `#<instanceId>`, each refusing the other prefix | `rooms.go:1922-1945`, `:1986-2000` |
| `m2-actor` knows Heal and Conviction Surge with spellcasting 22; `m2-witness` has no spellcasting; `mid` has skullduggery 8 | `tools/playtest/profiles/*.yaml` |
| Profiles carry buffs as `buffs: list: - buffid: N` (`buffs.Buffs.List`, `buffs.Buff` yaml tags) | `internal/buffs/buffs.go:14-44`; `characters/character.go:145` |

## How every task runs

- Work on `feature/followup-slice-a-names-in-dark`. Named paths only in `git add`.
- Each task ends with, in order:
  1. `gofmt -l internal/ modules/` prints nothing.
  2. `go build ./...` succeeds.
  3. The task's package tests pass.
  4. `go test .` passes from the repo root, because the root guards read the files every task touches.
- **Sabotage probe.** Each task names one deliberate break that still compiles. Make it, confirm the named test fails for the named reason, then undo it with the Edit tool. Never undo with `git checkout -- <path>`: that stages the revert.
- Execution follows the owner's established style: the executor implements and commits each task, and a blind adversarial reviewer in an isolated worktree reviews behind each commit.
- Commit messages end with the two trailer lines:
  `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>` and
  `Claude-Session: https://claude.ai/code/session_01J9ARXi13pxi92SKumuvpum`.

---

### Task 1: Participant sight

**Files:**
- Modify: `internal/messaging/predicates.go` (append after `CanSeeShapes`)
- Test: `internal/messaging/participant_sight_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/messaging/participant_sight_test.go`:

```go
package messaging

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
)

// sightLight is a RoomVisibility with a fixed light level: 0 dark, 1 lit.
type sightLight int

func (l sightLight) GetVisibility() int { return int(l) }

const (
	sightInfraredBuffId = 9101
	sightNightBuffId    = 9102
	sightSleepBuffId    = 9103
)

// sightChar returns a fresh character carrying the given test flags. The three
// flag buffs are seeded once per test, so applying one never replaces another.
func sightChar(t *testing.T, flags ...buffs.Flag) *characters.Character {
	t.Helper()
	t.Cleanup(buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		sightInfraredBuffId: {BuffId: sightInfraredBuffId, Name: "Test Infrared", Flags: []buffs.Flag{buffs.InfraredVision}},
		sightNightBuffId:    {BuffId: sightNightBuffId, Name: "Test Night", Flags: []buffs.Flag{buffs.NightVision}},
		sightSleepBuffId:    {BuffId: sightSleepBuffId, Name: "Test Sleep", Flags: []buffs.Flag{buffs.Sleeping}},
	}))
	c := newChar(t)
	ids := map[buffs.Flag]int{
		buffs.InfraredVision: sightInfraredBuffId,
		buffs.NightVision:    sightNightBuffId,
		buffs.Sleeping:       sightSleepBuffId,
	}
	for _, f := range flags {
		if err := c.AddBuff(ids[f], true); err != nil {
			t.Fatalf("applying %s: %v", f, err)
		}
	}
	return c
}

func TestParticipantSight(t *testing.T) {
	cases := []struct {
		name  string
		light sightLight
		flags []buffs.Flag
		blind bool
		want  SightDecision
	}{
		{name: "lit room", light: 1, want: SightFull},
		{name: "dark room", light: 0, want: SightNone},
		{name: "dark with night vision", light: 0, flags: []buffs.Flag{buffs.NightVision}, want: SightFull},
		{name: "dark with infrared", light: 0, flags: []buffs.Flag{buffs.InfraredVision}, want: SightShapes},
		{name: "blinded in a lit room", light: 1, blind: true, want: SightNone},
		{name: "blinded with infrared in the dark", light: 0, flags: []buffs.Flag{buffs.InfraredVision}, blind: true, want: SightNone},
		// Sleep is NOT a factor: a sleeper struck in a lit room is told what hit them.
		{name: "sleeping in a lit room", light: 1, flags: []buffs.Flag{buffs.Sleeping}, want: SightFull},
		{name: "sleeping with infrared in the dark", light: 0, flags: []buffs.Flag{buffs.Sleeping, buffs.InfraredVision}, want: SightShapes},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := sightChar(t, tc.flags...)
			if tc.blind {
				setBlinded(t, c)
			}
			if got := ParticipantSight(c, tc.light); got != tc.want {
				t.Fatalf("ParticipantSight = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParticipantSight_NilObserverSeesFully(t *testing.T) {
	if got := ParticipantSight(nil, sightLight(0)); got != SightFull {
		t.Fatalf("nil observer = %v, want SightFull, matching the other predicates", got)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/messaging/ -run TestParticipantSight`
Expected: FAIL to compile, `undefined: ParticipantSight`.

- [ ] **Step 3: Write the implementation**

Append to `internal/messaging/predicates.go`, after `CanSeeShapes`:

```go
// ParticipantSight is what a party to an event makes out of the other party:
// the one acting on them, or the one they act on. messaging.SendTrio uses it to
// hide a name from its reader, and actions.InitiateCast uses it to refuse a
// cast at something the caster cannot see.
//
// It differs from CanSeeClearly in one deliberate way: SLEEP IS NOT A FACTOR.
// A sleeper struck in a lit room must be told what hit them, the reason
// CanSeeSightImpairedOnly exists and the melee darkness rewrite uses it
// (hooks/NewRound_DoCombat_unified.go). Observers who are not a party keep
// CanSeeClearly and CanSeeShapes, so a sleeper still receives no room lines.
//
// Full when darkness and blindness allow clear sight; shapes when the observer
// is not blinded and has infrared; none otherwise. A nil observer sees fully,
// matching the other predicates.
func ParticipantSight(observer *characters.Character, room RoomVisibility) SightDecision {
	if CanSeeSightImpairedOnly(observer, room) {
		return SightFull
	}
	if observer.Perception != nil && observer.Perception.State() == perception.Blinded {
		return SightNone
	}
	if observer.HasFlagFromAnySource(buffs.InfraredVision) {
		return SightShapes
	}
	return SightNone
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/messaging/`
Expected: PASS.

- [ ] **Step 5: Sabotage probe**

Change `if CanSeeSightImpairedOnly(observer, room) {` to `if CanSeeClearly(observer, room) {`. Run `go test ./internal/messaging/ -run TestParticipantSight`. Expected: FAIL on `sleeping in a lit room` (got SightNone). Undo with Edit.

- [ ] **Step 6: Gates and commit**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/messaging/ && go test .
git add internal/messaging/predicates.go internal/messaging/participant_sight_test.go
git commit -m "feat(messaging): ParticipantSight, sight for a party to an event" -m "Darkness and blindness decide it; sleep does not, so a sleeper struck in a lit room is still told what hit them. Shapes for infrared." -m "Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>" -m "Claude-Session: https://claude.ai/code/session_01J9ARXi13pxi92SKumuvpum"
```

---

### Task 2: Hiding names

**Files:**
- Create: `internal/messaging/hidenames.go`
- Test: `internal/messaging/hidenames_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/messaging/hidenames_test.go`:

```go
package messaging

import "testing"

func TestHideNames(t *testing.T) {
	const anon = `<ansi fg="combat-anon">`
	cases := []struct {
		name  string
		text  string
		names []string
		sight SightDecision
		want  string
	}{
		{
			name: "clear sight changes nothing", sight: SightFull,
			text: "You kick Bobrick!", names: []string{"Bobrick"},
			want: "You kick Bobrick!",
		},
		{
			name: "shapes, bare name mid-sentence", sight: SightShapes,
			text: "You kick Bobrick!", names: []string{"Bobrick"},
			want: "You kick " + anon + "a figure</ansi>!",
		},
		{
			name: "no sight, bare name at the start", sight: SightNone,
			text: "Bobrick kicks you!", names: []string{"Bobrick"},
			want: anon + "Something</ansi> kicks you!",
		},
		{
			name: "a whole identity tag goes with the name, possessive kept", sight: SightNone,
			text:  `<ansi fg="green"><ansi fg="username">Aliceia</ansi>'s Heal envelops you.</ansi>`,
			names: []string{"Aliceia"},
			want:  `<ansi fg="green">` + anon + `Something</ansi>'s Heal envelops you.</ansi>`,
		},
		{
			name: "a suffixed mob tag with a duplicate index goes whole", sight: SightShapes,
			text: `You hit <ansi fg="mobname-dup2">Rat #2</ansi>.`, names: []string{"Rat"},
			want: "You hit " + anon + "a figure</ansi>.",
		},
		{
			name: "capitalized after a sentence end, through tags", sight: SightNone,
			text:  `<ansi fg="cyan-bold">⚡ SWEEP!</ansi> Kesh dodges and sweeps Bobrick to the ground!`,
			names: []string{"Kesh", "Bobrick"},
			want:  `<ansi fg="cyan-bold">⚡ SWEEP!</ansi> ` + anon + "Something</ansi> dodges and sweeps " + anon + "something</ansi> to the ground!",
		},
		{
			name: "whole words only", sight: SightNone,
			text: "Keshara greets Kesh.", names: []string{"Kesh"},
			want: "Keshara greets " + anon + "something</ansi>.",
		},
		{
			name: "longest name first", sight: SightNone,
			text: "Kesh Vane waves.", names: []string{"Kesh", "Kesh Vane"},
			want: anon + "Something</ansi> waves.",
		},
		{
			name: "empty names are ignored", sight: SightNone,
			text: "Hello there.", names: []string{NoName, ""},
			want: "Hello there.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := HideNames(tc.text, tc.names, tc.sight); got != tc.want {
				t.Fatalf("HideNames =\n  %q\nwant\n  %q", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/messaging/ -run TestHideNames`
Expected: FAIL to compile, `undefined: HideNames` and `undefined: NoName`.

- [ ] **Step 3: Write the implementation**

Create `internal/messaging/hidenames.go`:

```go
package messaging

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// NoName marks a side of an Audience with nobody on it: a salvaged corpse, a
// thrown item landing in a crowd. It is the empty string, spelled out so the
// root Audience guard can tell a considered absence from a forgotten name, the
// same idea as NoLine.
const NoName = ""

// identityOpenTag matches a player, mob or pet name's opening tag at the very
// end of the text before a name. The same aliases Anonymize recognises.
var identityOpenTag = regexp.MustCompile(`<ansi fg="(?:(?:username|mobname)(?:-[A-Za-z0-9_-]+)?|petname)">$`)

// identityCloseAfterName matches what may follow a name inside its identity
// tag: an optional duplicate index (" #2", see characters.FormattedName) and
// the closing tag.
var identityCloseAfterName = regexp.MustCompile(`^(?: #\d+)?</ansi>`)

// HideNames replaces each of names in text with what a reader who cannot make
// that party out perceives: "a figure" when they see shapes only, "something"
// when they see nothing. Clear sight returns text unchanged.
//
// A name matches as an EXACT substring, longest name first, and only as a whole
// word: "Kesh" does not match inside "Keshara". When a match is the whole
// content of an identity tag, the tag goes with it, so the output never nests
// a combat-anon tag inside a name tag. The replacement is capitalized at the
// start of a sentence, looking through ANSI tags, so "⚡ SWEEP! Kesh dodges"
// reads "⚡ SWEEP! Something dodges".
func HideNames(text string, names []string, d SightDecision) string {
	if d == SightFull || text == "" {
		return text
	}
	word := "something"
	if d == SightShapes {
		word = "a figure"
	}
	ordered := make([]string, 0, len(names))
	for _, n := range names {
		if n != NoName {
			ordered = append(ordered, n)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool { return len(ordered[i]) > len(ordered[j]) })
	for _, name := range ordered {
		text = hideOneName(text, name, word)
	}
	return text
}

func hideOneName(text, name, word string) string {
	from := 0
	for from <= len(text) {
		idx := strings.Index(text[from:], name)
		if idx < 0 {
			return text
		}
		start := from + idx
		end := start + len(name)
		if !standsAsWord(text, start, end, name) {
			_, size := utf8.DecodeRuneInString(text[start:])
			from = start + size
			continue
		}
		cutStart, cutEnd := start, end
		if open := identityOpenTag.FindStringIndex(text[:start]); open != nil {
			if closing := identityCloseAfterName.FindStringIndex(text[end:]); closing != nil {
				cutStart, cutEnd = open[0], end+closing[1]
			}
		}
		shown := word
		if atSentenceStart(text[:cutStart]) {
			shown = strings.ToUpper(word[:1]) + word[1:]
		}
		replacement := `<ansi fg="combat-anon">` + shown + `</ansi>`
		text = text[:cutStart] + replacement + text[cutEnd:]
		from = cutStart + len(replacement)
	}
	return text
}

// standsAsWord reports whether the match at [start, end) is a whole word: a
// name that begins or ends with a letter or digit may not touch another one.
func standsAsWord(text string, start, end int, name string) bool {
	first, _ := utf8.DecodeRuneInString(name)
	if isNameRune(first) && start > 0 {
		prev, _ := utf8.DecodeLastRuneInString(text[:start])
		if isNameRune(prev) {
			return false
		}
	}
	last, _ := utf8.DecodeLastRuneInString(name)
	if isNameRune(last) && end < len(text) {
		next, _ := utf8.DecodeRuneInString(text[end:])
		if isNameRune(next) {
			return false
		}
	}
	return true
}

func isNameRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// atSentenceStart reports whether text placed after prefix begins a sentence:
// only tags and spaces before it, or a sentence end followed by a space.
func atSentenceStart(prefix string) bool {
	plain := ansiTagPattern.ReplaceAllString(prefix, "")
	trimmed := strings.TrimRight(plain, " ")
	if trimmed == "" {
		return true
	}
	if len(trimmed) == len(plain) {
		return false
	}
	switch trimmed[len(trimmed)-1] {
	case '.', '!', '?':
		return true
	}
	return false
}
```

`ansiTagPattern` is the existing `<ansi[^>]*>|</ansi>` in `internal/messaging/wrap.go:11`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/messaging/`
Expected: PASS.

- [ ] **Step 5: Sabotage probe**

In `hideOneName`, replace `if !standsAsWord(text, start, end, name) {` with `if false && !standsAsWord(text, start, end, name) {`. Run `go test ./internal/messaging/ -run TestHideNames`. Expected: FAIL on `whole words only`. Undo with Edit.

- [ ] **Step 6: Gates and commit**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/messaging/ && go test .
git add internal/messaging/hidenames.go internal/messaging/hidenames_test.go
git commit -m "feat(messaging): HideNames and NoName" -m "Swaps a name for a figure or something by the reader's sight: exact whole-word match, longest first, a whole identity tag replaced with its name, capitalized at a sentence start." -m "Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>" -m "Claude-Session: https://claude.ai/code/session_01J9ARXi13pxi92SKumuvpum"
```

---
### Task 3: The seam hides names per viewer

**Files:**
- Modify: `internal/messaging/trio.go` (`Broadcaster`, `Audience`, `SendTrio`)
- Modify: `internal/rooms/rooms.go:307-365` (`SendTextVisual`, `SendTextVisualAsLit`, `sendTextVisualJudgedBy`; add two methods)
- Test: `internal/messaging/trio_test.go` (update the fake, add tests)
- Test: `internal/rooms/participant_sight_test.go` (create)

- [ ] **Step 1: Update the fake and write the failing seam tests**

In `internal/messaging/trio_test.go`, replace the `fakeBroadcaster` type and its method:

```go
type fakeBroadcaster struct {
	calls int
	cat   Category
	text  string
	names []string
	excl  []int
	sight map[int]SightDecision
}

func (f *fakeBroadcaster) SendTextVisualHidingNames(cat Category, txt string, names []string, excludeUserIds ...int) {
	f.calls++
	f.cat = cat
	f.text = txt
	f.names = append([]string(nil), names...)
	f.excl = append([]int(nil), excludeUserIds...)
}

func (f *fakeBroadcaster) ParticipantSight(userId int) SightDecision {
	if d, ok := f.sight[userId]; ok {
		return d
	}
	return SightFull
}
```

Append to the same file:

```go
func TestSendTrioHidesTheOtherPartyByEachReadersOwnSight(t *testing.T) {
	actor, actee := &fakeRecipient{}, &fakeRecipient{}
	room := &fakeBroadcaster{sight: map[int]SightDecision{7: SightShapes, 9: SightNone}}
	SendTrio(Trio{
		Actor:    Say(CategorySystem, `You kick <ansi fg="mobname">Bobrick</ansi>!`),
		Actee:    Say(CategorySystem, `<ansi fg="username">Aliceia</ansi> kicks you!`),
		Observer: Say(CategoryKick, `Aliceia kicks Bobrick!`),
	}, Audience{
		Actor: actor, ActorId: 7, ActorName: "Aliceia",
		Actee: actee, ActeeId: 9, ActeeName: "Bobrick",
		Room: room,
	})

	if got, want := actor.texts[0], `You kick <ansi fg="combat-anon">a figure</ansi>!`; got != want {
		t.Errorf("actor (shapes) read %q, want %q", got, want)
	}
	if got, want := actee.texts[0], `<ansi fg="combat-anon">Something</ansi> kicks you!`; got != want {
		t.Errorf("actee (no sight) read %q, want %q", got, want)
	}
	if len(room.names) != 2 || room.names[0] != "Aliceia" || room.names[1] != "Bobrick" {
		t.Errorf("room was handed names %v, want [Aliceia Bobrick]", room.names)
	}
	if room.text != `Aliceia kicks Bobrick!` {
		t.Errorf("the observer line is hidden per observer by the Broadcaster, not before it: got %q", room.text)
	}
}

func TestSendTrioWithoutNamesIsUnchangedInTheDark(t *testing.T) {
	actor := &fakeRecipient{}
	room := &fakeBroadcaster{sight: map[int]SightDecision{7: SightNone}}
	SendTrio(Trio{
		Actor:    Say(CategorySystem, "You kick Bobrick!"),
		Actee:    NoLine,
		Observer: NoLine,
	}, Audience{Actor: actor, ActorId: 7, Room: room})

	if actor.texts[0] != "You kick Bobrick!" {
		t.Fatalf("a caller that names no one must get its text untouched, got %q", actor.texts[0])
	}
}

func TestSendTrioNilRoomHidesNothing(t *testing.T) {
	actor := &fakeRecipient{}
	SendTrio(Trio{
		Actor:    Say(CategorySystem, "You kick Bobrick!"),
		Actee:    NoLine,
		Observer: NoLine,
	}, Audience{Actor: actor, ActorId: 7, ActeeName: "Bobrick"})

	if actor.texts[0] != "You kick Bobrick!" {
		t.Fatalf("with no room there is no light to judge, got %q", actor.texts[0])
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/messaging/`
Expected: FAIL to compile: `unknown field ActorName in struct literal`, and `*fakeBroadcaster does not implement Broadcaster` is NOT reported yet (the interface still names `SendTextVisual`), so the first error is the unknown field.

- [ ] **Step 3: Implement the seam**

In `internal/messaging/trio.go`, replace the `Broadcaster` declaration:

```go
// Broadcaster is a room that can deliver an event's observer line and judge a
// participant's sight. Satisfied by *rooms.Room.
type Broadcaster interface {
	// SendTextVisualHidingNames broadcasts a sight-gated line, excluding some
	// user ids. An observer who makes out shapes only reads each of names as
	// "a figure".
	SendTextVisualHidingNames(cat Category, txt string, names []string, excludeUserIds ...int)
	// ParticipantSight is ParticipantSight for the user in this room.
	ParticipantSight(userId int) SightDecision
}
```

Replace the `Audience` struct (keep its doc comment above it):

```go
type Audience struct {
	Actor   Recipient
	ActorId int
	// ActorName and ActeeName are the names exactly as the lines print them,
	// plain or inside an identity tag. SendTrio hides each from a reader who
	// cannot see that party: "a figure" for infrared, "something" otherwise.
	// Write NoName for a side with nobody on it; the root guard requires both.
	ActorName string
	Actee     Recipient
	ActeeId   int
	ActeeName string
	Room      Broadcaster
}
```

Replace `SendTrio` (and the paragraph of its doc comment that says it "does not render, choose, band or tokenise anything"):

```go
// SendTrio delivers one narrated event to everyone entitled to it.
//
// A line is delivered only if it has BOTH text and a recipient; either half
// being absent is a correct, silent skip. The room broadcast always excludes
// the actor and the actee, so a caller can no longer get the exclusion list
// wrong by hand.
//
// Each role is rendered for its own reader. The actor's line hides ActeeName
// and the actee's line hides ActorName, judged by that reader's
// ParticipantSight; the observer line hides both, judged per observer by the
// room. It chooses, bands and tokenises nothing: callers still pass finished
// strings.
func SendTrio(t Trio, aud Audience) {
	if aud.Actor != nil && t.Actor.Text != "" {
		aud.Actor.SendText(t.Actor.Cat, hideForReader(aud, aud.ActorId, t.Actor.Text, aud.ActeeName))
	}
	if aud.Actee != nil && t.Actee.Text != "" {
		aud.Actee.SendText(t.Actee.Cat, hideForReader(aud, aud.ActeeId, t.Actee.Text, aud.ActorName))
	}
	if aud.Room != nil && t.Observer.Text != "" {
		aud.Room.SendTextVisualHidingNames(t.Observer.Cat, t.Observer.Text,
			[]string{aud.ActorName, aud.ActeeName}, trioExclusions(aud)...)
	}
}

// hideForReader hides the other party's name from one participant by that
// participant's sight in the room. With no room there is no light to judge.
func hideForReader(aud Audience, readerId int, text, otherName string) string {
	if aud.Room == nil || otherName == NoName {
		return text
	}
	return HideNames(text, []string{otherName}, aud.Room.ParticipantSight(readerId))
}
```

- [ ] **Step 4: Implement the room side**

In `internal/rooms/rooms.go`, change `SendTextVisual` and `SendTextVisualAsLit` to pass no names:

```go
func (r *Room) SendTextVisual(cat messaging.Category, txt string, excludeUserIds ...int) {
	r.sendTextVisualJudgedBy(r, cat, txt, nil, excludeUserIds...)
}
```

```go
func (r *Room) SendTextVisualAsLit(cat messaging.Category, txt string, excludeUserIds ...int) {
	r.sendTextVisualJudgedBy(litRoom{}, cat, txt, nil, excludeUserIds...)
}
```

Change the signature of `sendTextVisualJudgedBy` and hide names for shapes-only observers:

```go
// sendTextVisualJudgedBy is SendTextVisual with the lighting it judges sight
// against passed in, so SendTextVisualAsLit shares one delivery path. names,
// when given, are hidden from an observer who makes out shapes only, including
// bare names Anonymize cannot see.
func (r *Room) sendTextVisualJudgedBy(lighting messaging.RoomVisibility, cat messaging.Category, txt string, names []string, excludeUserIds ...int) {
	for _, uid := range r.GetPlayers() {
		if excluded(uid, excludeUserIds) {
			continue
		}
		u := users.GetByUserId(uid)
		if u == nil {
			continue
		}
		decision := messaging.SightNone
		switch {
		case messaging.CanSeeClearly(u.Character, lighting):
			decision = messaging.SightFull
		case messaging.CanSeeShapes(u.Character, lighting):
			decision = messaging.SightShapes
		}
		text := txt
		if decision == messaging.SightShapes && len(names) > 0 {
			text = messaging.HideNames(txt, names, messaging.SightShapes)
		}
		rendered := messaging.RenderForRecipient(messaging.RenderInput{
			Category:      cat,
			Text:          text,
			Channel:       messaging.ChannelVisual,
			SightDecision: decision,
			LineWidth:     u.GetLineWidth(),
		})
		if rendered == "" {
			continue
		}
		events.AddToQueue(events.Message{
			UserId: u.UserId,
			Text:   rendered + "\n",
		})
	}
}

// SendTextVisualHidingNames is SendTextVisual for a line that names the
// parties to an event: an observer who makes out shapes only reads each of
// names as "a figure". It is the observer half of messaging.SendTrio.
func (r *Room) SendTextVisualHidingNames(cat messaging.Category, txt string, names []string, excludeUserIds ...int) {
	r.sendTextVisualJudgedBy(r, cat, txt, names, excludeUserIds...)
}

// ParticipantSight is messaging.ParticipantSight for a user in this room. An
// unknown user sees fully: there is nobody to hide anything from.
func (r *Room) ParticipantSight(userId int) messaging.SightDecision {
	u := users.GetByUserId(userId)
	if u == nil {
		return messaging.SightFull
	}
	return messaging.ParticipantSight(u.Character, r)
}
```

Confirm no other caller of the old signature: `grep -n "sendTextVisualJudgedBy(" internal/rooms/*.go` shows only the three calls above and the definition.

- [ ] **Step 5: Write the room test**

Create `internal/rooms/participant_sight_test.go`:

```go
package rooms

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const sightTestInfraredBuffId = 7401

var sightTestTag = regexp.MustCompile(`<[^>]*>`)

func sightTestPlain(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, strings.TrimSpace(sightTestTag.ReplaceAllString(l, "")))
	}
	return out
}

// sightTestRoom seeds three players in one room of the given biome: 7411
// Aliceia and 7412 Bobrick are the parties, 7413 Ordel watches.
func sightTestRoom(t *testing.T, biome string) *Room {
	t.Helper()
	t.Cleanup(SeedBiomesForTest(map[string]*BiomeInfo{
		"cave":    {BiomeId: "cave", DarkArea: true},
		"city":    {BiomeId: "city", LitArea: true},
		"default": {BiomeId: "default", LitArea: true},
	}))
	t.Cleanup(buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		sightTestInfraredBuffId: {BuffId: sightTestInfraredBuffId, Name: "Test Heat Eyes", Flags: []buffs.Flag{buffs.InfraredVision}},
	}))
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		7411: users.NewTestUser(7411, "aliceia", "Aliceia", 97411),
		7412: users.NewTestUser(7412, "bobrick", "Bobrick", 97412),
		7413: users.NewTestUser(7413, "ordel", "Ordel", 97413),
	}))
	r := &Room{RoomId: 7410, Biome: biome}
	for _, id := range []int{7411, 7412, 7413} {
		r.AddPlayer(id)
		events.DrainQueuedMessagesForTest(id)
	}
	return r
}

func TestSendTextVisualHidingNames_InfraredObserverReadsFigures(t *testing.T) {
	r := sightTestRoom(t, "cave")
	if !users.GetByUserId(7413).Character.Buffs.AddBuff(sightTestInfraredBuffId, true) {
		t.Fatal("precondition: the observer should now carry infrared")
	}
	r.SendTextVisualHidingNames(messaging.CategoryKick, "Aliceia kicks Bobrick!",
		[]string{"Aliceia", "Bobrick"}, 7411, 7412)

	got := sightTestPlain(events.DrainQueuedMessagesForTest(7413))
	if len(got) != 1 || got[0] != "A figure kicks a figure!" {
		t.Fatalf("infrared observer read %q, want one line %q", got, "A figure kicks a figure!")
	}
}

func TestSendTextVisualHidingNames_UnsightedObserverGetsNothing(t *testing.T) {
	r := sightTestRoom(t, "cave")
	r.SendTextVisualHidingNames(messaging.CategoryKick, "Aliceia kicks Bobrick!",
		[]string{"Aliceia", "Bobrick"}, 7411, 7412)

	if got := events.DrainQueuedMessagesForTest(7413); len(got) != 0 {
		t.Fatalf("an observer who cannot see got %q", got)
	}
}

func TestSendTextVisualHidingNames_LitObserverReadsTheNames(t *testing.T) {
	r := sightTestRoom(t, "city")
	r.SendTextVisualHidingNames(messaging.CategoryKick, "Aliceia kicks Bobrick!",
		[]string{"Aliceia", "Bobrick"}, 7411, 7412)

	got := sightTestPlain(events.DrainQueuedMessagesForTest(7413))
	if len(got) != 1 || got[0] != "Aliceia kicks Bobrick!" {
		t.Fatalf("lit observer read %q, want the names", got)
	}
}

func TestRoomParticipantSight_JudgesTheUserInThisRoom(t *testing.T) {
	r := sightTestRoom(t, "cave")
	users.GetByUserId(7412).Character.Buffs.AddBuff(sightTestInfraredBuffId, true)

	if d := r.ParticipantSight(7411); d != messaging.SightNone {
		t.Errorf("no vision in a cave = %v, want SightNone", d)
	}
	if d := r.ParticipantSight(7412); d != messaging.SightShapes {
		t.Errorf("infrared in a cave = %v, want SightShapes", d)
	}
	if d := r.ParticipantSight(99999); d != messaging.SightFull {
		t.Errorf("unknown user = %v, want SightFull", d)
	}
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/messaging/ ./internal/rooms/`
Expected: PASS.

- [ ] **Step 7: Sabotage probe**

In `SendTrio`, swap `aud.ActeeName` and `aud.ActorName` in the two `hideForReader` calls. Run `go test ./internal/messaging/ -run TestSendTrioHides`. Expected: FAIL, the actor reads the unhidden `Bobrick`. Undo with Edit.

- [ ] **Step 8: Gates and commit**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/messaging/ ./internal/rooms/ && go test .
git add internal/messaging/trio.go internal/messaging/trio_test.go internal/rooms/rooms.go internal/rooms/participant_sight_test.go
git commit -m "feat(messaging): SendTrio hides names per reader" -m "Audience carries ActorName and ActeeName. Each personal line hides the other party by its reader's participant sight; the observer line hides both for shapes-only observers through Room.SendTextVisualHidingNames. No names given, or a lit room, changes nothing." -m "Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>" -m "Claude-Session: https://claude.ai/code/session_01J9ARXi13pxi92SKumuvpum"
```

---

### Task 4: Every Audience names both parties

**Files:**
- Modify: `m2_routing_guard_test.go` (`TestEveryAudienceLiteralPairsIdsWithRecipients`)
- Modify: 30 literals in `internal/actions/salvage.go`, `internal/mobcommands/{bash,charge,drain,gore,grapple,hamstring,kick,maul,pounce,rake,shoot,throttle,trip}.go`, `internal/usercommands/{bash,drain,gore,grapple,kick,maul,pounce,rake,shoot,throttle,throw,trip}.go`

- [ ] **Step 1: Extend the guard (the failing test)**

In `m2_routing_guard_test.go`, find this loop inside `TestEveryAudienceLiteralPairsIdsWithRecipients`:

```go
				for recipient, id := range pairs {
					if named[recipient] && !named[id] {
						bad = append(bad, filepath.ToSlash(path)+":"+
							strconv.Itoa(fset.Position(cl.Pos()).Line)+
							"  names "+recipient+" but not "+id)
					}
				}
```

Add directly after it:

```go
				// Every literal names both parties, even a side with nobody on
				// it (messaging.NoName): SendTrio can hide a name from a reader
				// in the dark only if it is told the name.
				for _, name := range []string{"ActorName", "ActeeName"} {
					if !named[name] {
						bad = append(bad, filepath.ToSlash(path)+":"+
							strconv.Itoa(fset.Position(cl.Pos()).Line)+
							"  does not name "+name)
					}
				}
```

Replace the failure message:

```go
		t.Errorf("%d messaging.Audience literal(s) name a recipient without its id:\n  %s\n\n"+
			"SendTrio builds the room broadcast's exclusion list from ActorId and "+
			"ActeeId. Omit one and that person receives the third-person room line "+
			"on top of their own personal line.",
			len(bad), strings.Join(bad, "\n  "))
```

with:

```go
		t.Errorf("%d messaging.Audience literal(s) are incomplete:\n  %s\n\n"+
			"SendTrio builds the room broadcast's exclusion list from ActorId and "+
			"ActeeId. Omit one and that person receives the third-person room line "+
			"on top of their own personal line.\n\n"+
			"It hides ActorName and ActeeName from a reader who cannot see that "+
			"party. Omit one and a player in the dark is told a name. Write "+
			"messaging.NoName for a side with nobody on it.",
			len(bad), strings.Join(bad, "\n  "))
```

- [ ] **Step 2: Run the guard to verify it fails**

Run: `go test . -run TestEveryAudienceLiteralPairsIdsWithRecipients`
Expected: FAIL listing 30 literals, each `does not name ActorName` and `does not name ActeeName`.

- [ ] **Step 3: Add the names, mob side**

Each name is the exact string the file's sentences print. Every mob file below declares `mobName := mob.Character.Name` and a `target` with `.Name`.

In each of `internal/mobcommands/bash.go`, `drain.go`, `gore.go`, `grapple.go`, `hamstring.go`, `kick.go`, `maul.go`, `pounce.go`, `rake.go`, `throttle.go`, and in `charge.go:174` (`narrateChargeWhiffOnProne`), replace every

```go
	aud := messaging.Audience{
		Actee:   acteeRecipient,
		ActeeId: target.UserId,
		Room:    room,
	}
```

with

```go
	aud := messaging.Audience{
		ActorName: mobName,
		Actee:     acteeRecipient,
		ActeeId:   target.UserId,
		ActeeName: target.Name,
		Room:      room,
	}
```

In `internal/mobcommands/trip.go` the same literal appears twice (`:66` and `:268`, `narrateTripWhiffOnProne`); use Edit with `replace_all: true`. Both functions declare `mobName` and `target`.

In `internal/mobcommands/charge.go:66` the literal uses `ActeeId: targetPlayerId,`; replace it with:

```go
	aud := messaging.Audience{
		ActorName: mobName,
		Actee:     acteeRecipient,
		ActeeId:   targetPlayerId,
		ActeeName: targetName,
		Room:      room,
	}
```

In `internal/mobcommands/shoot.go:112`:

```go
	aud := messaging.Audience{
		ActorName: mob.Character.Name,
		Actee:     acteeRecipient,
		ActeeId:   result.TargetUserId,
		ActeeName: result.TargetName,
		Room:      room,
	}
```

and at `:172` replace `messaging.Audience{ActeeId: result.TargetUserId, Room: tr}` with:

```go
messaging.Audience{ActorName: mob.Character.Name, ActeeId: result.TargetUserId, ActeeName: result.TargetName, Room: tr}
```

The shot lines wrap these names in `mobname` / `username` tags; `HideNames` replaces a whole tag whose content is the name.

- [ ] **Step 4: Add the names, player side**

In each of `internal/usercommands/drain.go`, `gore.go`, `kick.go`, `maul.go`, `pounce.go`, `rake.go`, `throttle.go` (each declares `targetName := res.Target.Name`), replace

```go
	aud := messaging.Audience{
		Actor:   user,
		ActorId: user.UserId,
		Actee:   acteeRecipient,
		ActeeId: res.Target.UserId,
		Room:    room,
	}
```

with

```go
	aud := messaging.Audience{
		Actor:     user,
		ActorId:   user.UserId,
		ActorName: user.Character.Name,
		Actee:     acteeRecipient,
		ActeeId:   res.Target.UserId,
		ActeeName: targetName,
		Room:      room,
	}
```

`internal/usercommands/bash.go:67` (sentences print `target.Name`):

```go
	aud := messaging.Audience{
		Actor:     user,
		ActorId:   user.UserId,
		ActorName: user.Character.Name,
		Actee:     acteeRecipient,
		ActeeId:   target.UserId,
		ActeeName: target.Name,
		Room:      room,
	}
```

`internal/usercommands/grapple.go:101` and `internal/usercommands/trip.go:74` (each declares `targetName := target.Name` and uses `ActeeId: targetPlayerId`):

```go
	aud := messaging.Audience{
		Actor:     user,
		ActorId:   user.UserId,
		ActorName: user.Character.Name,
		Actee:     acteeRecipient,
		ActeeId:   targetPlayerId,
		ActeeName: targetName,
		Room:      room,
	}
```

`internal/usercommands/shoot.go:452`:

```go
	aud := messaging.Audience{
		Actor:     user,
		ActorId:   user.UserId,
		ActorName: user.Character.Name,
		Actee:     acteeRecipient,
		ActeeId:   result.TargetUserId,
		ActeeName: result.TargetName,
		Room:      room,
	}
```

and at `:546` replace `messaging.Audience{ActeeId: result.TargetUserId, Room: tr}` with:

```go
messaging.Audience{ActorName: user.Character.Name, ActeeId: result.TargetUserId, ActeeName: result.TargetName, Room: tr}
```

`internal/usercommands/throw.go:288` (the item lands in the fray; nobody is the actee):

```go
	aud := messaging.Audience{
		Actor:     user,
		ActorId:   user.UserId,
		ActorName: user.Character.Name,
		ActeeName: messaging.NoName,
		Room:      room,
	}
```

`internal/actions/salvage.go:243` (the actee is a corpse):

```go
	}, messaging.Audience{
		Actor:     actor,
		ActorId:   actor.GetUserId(),
		ActorName: actor.GetName(),
		ActeeName: messaging.NoName,
		Room:      room,
	})
```

`messaging.NoName` is an identifier, not a string literal, so `m2FrozenFiles` fingerprints do not move.

- [ ] **Step 5: Run the guards and the packages**

```bash
gofmt -w internal/actions/salvage.go internal/mobcommands/ internal/usercommands/
go build ./...
go test . -run 'TestEveryAudienceLiteralPairsIdsWithRecipients|TestM2LiteralsAreFrozen|TestM2RoutingIsFrozen|TestNarrationSitesMatchViewpointAudit'
go test ./internal/actions/ ./internal/mobcommands/ ./internal/usercommands/
```

Expected: all PASS. If `TestM2LiteralsAreFrozen` fails, a string literal was added to a frozen file: find it with `git diff -U0 -- internal/mobcommands internal/usercommands | grep '^+.*"'` and replace it with an identifier.

- [ ] **Step 6: Sabotage probe**

Delete `ActeeName: targetName,` from `internal/usercommands/kick.go`. Run `go test . -run TestEveryAudienceLiteralPairsIdsWithRecipients`. Expected: FAIL naming `internal/usercommands/kick.go:234  does not name ActeeName`. Undo with Edit.

- [ ] **Step 7: Gates and commit**

```bash
gofmt -l internal/ modules/
go build ./...
go test .
git add m2_routing_guard_test.go internal/actions/salvage.go internal/mobcommands/bash.go internal/mobcommands/charge.go internal/mobcommands/drain.go internal/mobcommands/gore.go internal/mobcommands/grapple.go internal/mobcommands/hamstring.go internal/mobcommands/kick.go internal/mobcommands/maul.go internal/mobcommands/pounce.go internal/mobcommands/rake.go internal/mobcommands/shoot.go internal/mobcommands/throttle.go internal/mobcommands/trip.go internal/usercommands/bash.go internal/usercommands/drain.go internal/usercommands/gore.go internal/usercommands/grapple.go internal/usercommands/kick.go internal/usercommands/maul.go internal/usercommands/pounce.go internal/usercommands/rake.go internal/usercommands/shoot.go internal/usercommands/throttle.go internal/usercommands/throw.go internal/usercommands/trip.go
git commit -m "feat(messaging): every Audience names both parties" -m "All 30 literals pass ActorName and ActeeName as their sentences print them, NoName where a side is empty, and the root Audience guard now requires both." -m "Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>" -m "Claude-Session: https://claude.ai/code/session_01J9ARXi13pxi92SKumuvpum"
```

---
### Task 5: Counter lines on the seam

**Files:**
- Modify: `internal/combat/counter.go` (`CounterResult` fields; `ExecuteCounter`)
- Modify: `internal/actions/combat_counter.go:71-91` (`DispatchCounterMessages`; add `SendCounterTrio`)
- Modify: `internal/hooks/counter_tier.go:33-60` (`fireSpellCounterTier`)
- Test: `internal/actions/counter_trio_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/actions/counter_trio_test.go`:

```go
package actions

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const counterTrioInfraredBuffId = 7501

var counterTrioTag = regexp.MustCompile(`<[^>]*>`)

func counterTrioPlain(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, strings.TrimSpace(counterTrioTag.ReplaceAllString(l, "")))
	}
	return out
}

// counterTrioRoom seeds 7511 Aliceia (the counterer), 7512 Bobrick (countered)
// and 7513 Ordel (watching) in one room of the given biome.
func counterTrioRoom(t *testing.T, biome string) *rooms.Room {
	t.Helper()
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave":    {BiomeId: "cave", DarkArea: true},
		"city":    {BiomeId: "city", LitArea: true},
		"default": {BiomeId: "default", LitArea: true},
	}))
	t.Cleanup(buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		counterTrioInfraredBuffId: {BuffId: counterTrioInfraredBuffId, Name: "Test Heat Eyes", Flags: []buffs.Flag{buffs.InfraredVision}},
	}))
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		7511: users.NewTestUser(7511, "aliceia", "Aliceia", 97511),
		7512: users.NewTestUser(7512, "bobrick", "Bobrick", 97512),
		7513: users.NewTestUser(7513, "ordel", "Ordel", 97513),
	}))
	room := &rooms.Room{RoomId: 7510, Biome: biome}
	for _, id := range []int{7511, 7512, 7513} {
		room.AddPlayer(id)
		events.DrainQueuedMessagesForTest(id)
	}
	return room
}

func counterTrioResult() combat.CounterResult {
	return combat.CounterResult{
		Countered:       true,
		CountererUserId: 7511,
		CountererName:   "Aliceia",
		CounteredName:   "Bobrick",
		DefenderMsg:     "You strike back at Bobrick!",
		AttackerMsg:     "Aliceia turns your attack into a strike of their own!",
		RoomMsg:         "Aliceia strikes back at Bobrick!",
	}
}

func TestSendCounterTrio_InTheDarkNobodyIsNamed(t *testing.T) {
	room := counterTrioRoom(t, "cave")
	SendCounterTrio(room, counterTrioResult(), users.GetByUserId(7512), 7512)

	assert.Equal(t, []string{"You strike back at something!"},
		counterTrioPlain(events.DrainQueuedMessagesForTest(7511)))
	assert.Equal(t, []string{"Something turns your attack into a strike of their own!"},
		counterTrioPlain(events.DrainQueuedMessagesForTest(7512)))
	assert.Empty(t, events.DrainQueuedMessagesForTest(7513),
		"an observer who cannot see gets no counter line")
}

func TestSendCounterTrio_InfraredObserverReadsFigures(t *testing.T) {
	room := counterTrioRoom(t, "cave")
	require.True(t, users.GetByUserId(7513).Character.Buffs.AddBuff(counterTrioInfraredBuffId, true))
	SendCounterTrio(room, counterTrioResult(), users.GetByUserId(7512), 7512)

	assert.Equal(t, []string{"A figure strikes back at a figure!"},
		counterTrioPlain(events.DrainQueuedMessagesForTest(7513)))
}

func TestSendCounterTrio_LitRoomIsUnchanged(t *testing.T) {
	room := counterTrioRoom(t, "city")
	SendCounterTrio(room, counterTrioResult(), users.GetByUserId(7512), 7512)

	assert.Equal(t, []string{"You strike back at Bobrick!"},
		counterTrioPlain(events.DrainQueuedMessagesForTest(7511)))
	assert.Equal(t, []string{"Aliceia strikes back at Bobrick!"},
		counterTrioPlain(events.DrainQueuedMessagesForTest(7513)))
}

func TestSendCounterTrio_NotCounteredSendsNothing(t *testing.T) {
	room := counterTrioRoom(t, "city")
	res := counterTrioResult()
	res.Countered = false
	SendCounterTrio(room, res, users.GetByUserId(7512), 7512)

	for _, id := range []int{7511, 7512, 7513} {
		assert.Empty(t, events.DrainQueuedMessagesForTest(id))
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/actions/ -run TestSendCounterTrio`
Expected: FAIL to compile: `unknown field CountererName`, `undefined: SendCounterTrio`.

- [ ] **Step 3: Carry the names out of the counter**

In `internal/combat/counter.go`, add to `CounterResult` after `CountererUserId int`:

```go
	// CountererName and CounteredName are the plain names the narration below
	// is built from, so the dispatchers can hand them to messaging.SendTrio,
	// which hides a name from a reader who cannot see that party.
	CountererName string
	CounteredName string
```

In `ExecuteCounter`, directly after `result.CountererUserId = defender.GetUserId()`:

```go
	result.CountererName = defender.Name
	result.CounteredName = attacker.Name
```

- [ ] **Step 4: One dispatch through the seam**

In `internal/actions/combat_counter.go`, replace the body of `DispatchCounterMessages` and add `SendCounterTrio` after it:

```go
func DispatchCounterMessages(actor Actor, res combat.CounterResult) {
	var countered messaging.Recipient
	if actor.IsPlayer() {
		countered = actor
	}
	SendCounterTrio(actor.GetRoom(), res, countered, actor.GetUserId())
}

// SendCounterTrio delivers one counter's narration through messaging.SendTrio:
// the counterer's line, the countered party's line and the room's. A reader who
// cannot see the other party reads "something" in place of their name, and the
// room line reaches only observers who can see. The counterer is looked up
// from res.CountererUserId; countered is nil for a mob. Both the skill-move
// exits (DispatchCounterMessages) and the spell exits
// (hooks.fireSpellCounterTier) come through here.
func SendCounterTrio(room *rooms.Room, res combat.CounterResult, countered messaging.Recipient, counteredUserId int) {
	if !res.Countered {
		return
	}
	var counterer messaging.Recipient
	if res.CountererUserId > 0 {
		if u := users.GetByUserId(res.CountererUserId); u != nil {
			counterer = u
		}
	}
	aud := messaging.Audience{
		Actor:     counterer,
		ActorId:   res.CountererUserId,
		ActorName: res.CountererName,
		Actee:     countered,
		ActeeId:   counteredUserId,
		ActeeName: res.CounteredName,
	}
	if room != nil {
		aud.Room = room
	}
	messaging.SendTrio(messaging.Trio{
		Actor:    messaging.Say(messaging.CategoryHitMelee, res.DefenderMsg),
		Actee:    messaging.Say(messaging.CategoryHitMelee, res.AttackerMsg),
		Observer: messaging.Say(messaging.CategoryHitMelee, res.RoomMsg),
	}, aud)
}
```

Add `"github.com/GoMudEngine/GoMud/internal/rooms"` to the file's imports if it is not already there (`messaging` and `users` already are).

One deliberate change: the room line now always excludes both parties. Before, the countered player was excluded only when `AttackerMsg` was non-empty.

- [ ] **Step 5: The spell exit uses the same dispatch**

In `internal/hooks/counter_tier.go`, replace everything in `fireSpellCounterTier` from `if defenderUser != nil && res.DefenderMsg != "" {` down to (and including) the closing brace of the `if res.RoomMsg != "" {` block with:

```go
	var countered messaging.Recipient
	counteredId := 0
	if casterUser != nil {
		countered = casterUser
		counteredId = casterUser.UserId
	}
	actions.SendCounterTrio(room, res, countered, counteredId)
```

`res.CountererUserId` is `defender.GetUserId()`, the same player `defenderUser` names. Add `"github.com/GoMudEngine/GoMud/internal/actions"` to the imports. `defenderUser` stays in the signature; callers are unchanged.

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/actions/ ./internal/hooks/ ./internal/combat/`
Expected: PASS, including the existing `counter_tier_test.go`.

- [ ] **Step 7: Sabotage probe**

In `SendCounterTrio`, change `ActeeName: res.CounteredName,` to `ActeeName: messaging.NoName,`. Run `go test ./internal/actions/ -run TestSendCounterTrio_InTheDark`. Expected: FAIL, the counterer reads `You strike back at Bobrick!`. Undo with Edit.

- [ ] **Step 8: Gates and commit**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/actions/ ./internal/hooks/ ./internal/combat/ && go test .
git add internal/combat/counter.go internal/actions/combat_counter.go internal/actions/counter_trio_test.go internal/hooks/counter_tier.go
git commit -m "feat(combat): counter narration goes through the seam" -m "CounterResult carries both names; DispatchCounterMessages and the spell counter exit share SendCounterTrio, so a counter in the dark names nobody and its room line reaches only those who can see." -m "Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>" -m "Claude-Session: https://claude.ai/code/session_01J9ARXi13pxi92SKumuvpum"
```

---

### Task 6: Crit effect lines on the seam (the SWEEP leak)

**Files:**
- Modify: `internal/hooks/NewRound_DoCombat_unified.go:554-564` (crit message routing in `dispatchCritAndMessaging`)
- Modify: `internal/hooks/combat_shared_helpers.go` (add `sendCritEffectTrio` after `applyCritEffects`)
- Test: `internal/hooks/crit_effect_trio_test.go` (create)

- [ ] **Step 1: Write the failing tests**

Create `internal/hooks/crit_effect_trio_test.go`:

```go
package hooks

import (
	"fmt"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sweepPrefix = `<ansi fg="cyan-bold">⚡ SWEEP!</ansi> `

// sweepCrit builds the three SWEEP lines exactly as applyCritEffects words
// them (combat_shared_helpers.go), with defender first and attacker second.
func sweepCrit(defender, attacker string) CritEffectResult {
	return CritEffectResult{
		DefenderMsg: sweepPrefix + `You dodge and sweep their legs out! They crash to the ground!`,
		AttackerMsg: fmt.Sprintf(sweepPrefix+`%s dodges and sweeps your legs! You crash to the ground!`, defender),
		RoomMsg:     fmt.Sprintf(sweepPrefix+`%s dodges and sweeps %s to the ground!`, defender, attacker),
	}
}

func TestSweep_AttackerInTheDarkIsNotToldTheDefendersName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	drainPlain(1)
	drainPlain(2)

	atk := actions.NewUserActorInRoom(users.GetByUserId(2), room)
	def := actions.NewUserActorInRoom(users.GetByUserId(1), room)
	sendCritEffectTrio(atk, def, room, sweepCrit("Aliceia", "Bobrick"))

	attacker := drainPlain(2)
	assert.Equal(t, 1, countContaining(attacker, "SWEEP! Something dodges and sweeps your legs!"))
	assert.Equal(t, 0, countContaining(attacker, "Aliceia"))
}

func TestSweep_RoomLineIsVisual(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	drainPlain(2)

	atk := actions.NewMobActorInRoom(mobs.GetInstance(100), room)
	def := actions.NewUserActorInRoom(users.GetByUserId(1), room)
	sendCritEffectTrio(atk, def, room, sweepCrit("Aliceia", "Skeleton"))
	assert.Equal(t, 0, countContaining(drainPlain(2), "SWEEP"),
		"an observer who cannot see must not be told about the sweep at all")

	require.True(t, users.GetByUserId(2).Character.Buffs.AddBuff(heatEyesBuffId, true))
	sendCritEffectTrio(atk, def, room, sweepCrit("Aliceia", "Skeleton"))
	observer := drainPlain(2)
	assert.Equal(t, 1, countContaining(observer, "SWEEP! A figure dodges and sweeps a figure to the ground!"))
	assert.Equal(t, 0, countContaining(observer, "Aliceia"))
	assert.Equal(t, 0, countContaining(observer, "Skeleton"))
}

func TestSweep_LitRoomNamesBoth(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	room := rooms.LoadRoom(1)
	drainPlain(2)

	atk := actions.NewMobActorInRoom(mobs.GetInstance(100), room)
	def := actions.NewUserActorInRoom(users.GetByUserId(1), room)
	sendCritEffectTrio(atk, def, room, sweepCrit("Aliceia", "Skeleton"))
	assert.Equal(t, 1, countContaining(drainPlain(2), "Aliceia dodges and sweeps Skeleton to the ground!"))
}

// The wiring: dispatchCritAndMessaging must route crit effects through the
// seam. A parry crit fires riposte with no contest (counter_tier_test.go), so
// this is deterministic. Before the fix the room line went out on the audio
// channel and reached the unsighted observer.
func TestCritDispatch_RiposteRoomLineSparesAnObserverInTheDark(t *testing.T) {
	pinCounterTierKnobs(t, 0.5)
	cleanup := seedAllRegistries()
	defer cleanup()
	roundTallies = newCombatTallies()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	drainPlain(2)

	atk := actions.NewMobActorInRoom(mobs.GetInstance(100), room)
	def := actions.NewUserActorInRoom(users.GetByUserId(1), room)
	res := vbLandingResult()
	res.ParryCritDetected = true
	dispatchCritAndMessaging(atk, def, res)

	assert.Equal(t, 0, countContaining(drainPlain(2), "RIPOSTE"))
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/hooks/ -run 'TestSweep|TestCritDispatch_Riposte'`
Expected: FAIL to compile, `undefined: sendCritEffectTrio`. (After Step 3 alone, `TestCritDispatch_RiposteRoomLineSparesAnObserverInTheDark` still fails until Step 4.)

- [ ] **Step 3: Write the helper**

In `internal/hooks/combat_shared_helpers.go`, directly after `applyCritEffects`, add:

```go
// sendCritEffectTrio delivers a crit effect's lines (riposte, sweep, shield
// slam) through messaging.SendTrio. The DEFENDER performs the effect, so the
// defender is the Actor and the original attacker the Actee.
//
// A reader who cannot see the other combatant reads "something", and the room
// line is visual. The SWEEP room line used to go out on the audio channel and
// named both combatants to players standing in the dark.
func sendCritEffectTrio(atk, def actions.Actor, room *rooms.Room, crit CritEffectResult) {
	var atkRecipient, defRecipient messaging.Recipient
	if atk.IsPlayer() {
		atkRecipient = atk
	}
	if def.IsPlayer() {
		defRecipient = def
	}
	aud := messaging.Audience{
		Actor:     defRecipient,
		ActorId:   def.GetUserId(),
		ActorName: def.GetCharacter().Name,
		Actee:     atkRecipient,
		ActeeId:   atk.GetUserId(),
		ActeeName: atk.GetCharacter().Name,
	}
	if room != nil {
		aud.Room = room
	}
	messaging.SendTrio(messaging.Trio{
		Actor:    messaging.Say(messaging.CategoryHitMelee, crit.DefenderMsg),
		Actee:    messaging.Say(messaging.CategoryHitMelee, crit.AttackerMsg),
		Observer: messaging.Say(messaging.CategoryHitMelee, crit.RoomMsg),
	}, aud)
}
```

Add `"github.com/GoMudEngine/GoMud/internal/actions"` and `"github.com/GoMudEngine/GoMud/internal/messaging"` to that file's imports if missing (`rooms` is already imported: `applyCritEffects` takes a `*rooms.Room`).

- [ ] **Step 4: Route the dispatch through it**

In `internal/hooks/NewRound_DoCombat_unified.go`, replace:

```go
	// Crit message routing — Divergence #1.
	if critResult.AttackerMsg != `` && atk.IsPlayer() {
		atk.SendText(messaging.CategoryHitMelee, critResult.AttackerMsg)
	}
	if critResult.DefenderMsg != `` && def.IsPlayer() {
		def.SendText(messaging.CategoryHitMelee, critResult.DefenderMsg)
	}
	if critResult.RoomMsg != `` && atkRoom != nil {
		// Crit effects (riposte / sweep / bash) are melee follow-ups.
		atkRoom.SendText(messaging.CategoryHitMelee, critResult.RoomMsg, playerExcludeIds(atk, def)...)
	}
```

with:

```go
	// Crit message routing. Through the seam, so a reader who cannot see the
	// other combatant reads "something" and the room line is sight-gated.
	sendCritEffectTrio(atk, def, atkRoom, critResult)
```

`playerExcludeIds` stays: it has two other callers (`:467`, `:603`).

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/hooks/`
Expected: PASS, including `combat_verbosity_wiring_test.go` and `counter_tier_test.go`.

- [ ] **Step 6: Sabotage probe**

Put the old `atkRoom.SendText(messaging.CategoryHitMelee, critResult.RoomMsg, playerExcludeIds(atk, def)...)` back on the line after `sendCritEffectTrio(...)`. Run `go test ./internal/hooks/ -run TestCritDispatch_Riposte`. Expected: FAIL, the observer receives the RIPOSTE line. Undo with Edit.

- [ ] **Step 7: Gates and commit**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/hooks/ && go test .
git add internal/hooks/combat_shared_helpers.go internal/hooks/NewRound_DoCombat_unified.go internal/hooks/crit_effect_trio_test.go
git commit -m "fix(combat): crit effect lines stop naming combatants in the dark" -m "Riposte, sweep and shield slam go through SendTrio. The room line was audio, so SWEEP named both combatants to players who could not see; it is now visual and names are hidden per reader." -m "Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>" -m "Claude-Session: https://claude.ai/code/session_01J9ARXi13pxi92SKumuvpum"
```

---
### Task 7: Spell lines on the seam

**Files:**
- Create: `internal/hooks/spell_audience.go`
- Modify: `internal/hooks/spell_resolution.go` (`applyPlayerEffect` cross-cast lines; `sendSpellChannelDefenceMessages`; `resolveMobSpellAgainstPlayer`)
- Modify: `internal/hooks/spell_purgeaffliction.go:21-32`
- Test: `internal/hooks/spell_names_in_dark_test.go` (create)

- [ ] **Step 1: Write the failing tests**

Create `internal/hooks/spell_names_in_dark_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Aliceia (1) casts on Bobrick (2); both stand in room 1.

func castHealOnBobrick(t *testing.T) (caster, target []string) {
	t.Helper()
	spell := &spells.SpellData{SpellId: "heal", Name: "Heal", EffectType: "heal", EffectMagnitude: 3}
	applyPlayerEffect(users.GetByUserId(1), users.GetByUserId(2), rooms.LoadRoom(1), spell, 3, spellContestAttackWin())
	return drainPlain(1), drainPlain(2)
}

func TestCrossCastHeal_InTheDarkNobodyIsNamed(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	darken(t, 1)
	drainPlain(1)
	drainPlain(2)

	caster, target := castHealOnBobrick(t)
	assert.Equal(t, 1, countContaining(target, "Something's Heal envelops you in healing energy."))
	assert.Equal(t, 0, countContaining(target, "Aliceia"))
	assert.Equal(t, 1, countContaining(caster, "You weave restorative magic around something."))
	assert.Equal(t, 0, countContaining(caster, "Bobrick"))
}

func TestCrossCastHeal_InfraredTargetReadsAFigure(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	darken(t, 1)
	require.True(t, users.GetByUserId(2).Character.Buffs.AddBuff(heatEyesBuffId, true))
	drainPlain(1)
	drainPlain(2)

	_, target := castHealOnBobrick(t)
	assert.Equal(t, 1, countContaining(target, "A figure's Heal envelops you in healing energy."))
}

func TestCrossCastHeal_LitRoomIsUnchanged(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	drainPlain(1)
	drainPlain(2)

	caster, target := castHealOnBobrick(t)
	assert.Equal(t, 1, countContaining(target, "Aliceia's Heal envelops you in healing energy."))
	assert.Equal(t, 1, countContaining(caster, "You weave restorative magic around Bobrick."))
}

func TestCrossCastDamage_TargetInTheDarkReadsSomething(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	darken(t, 1)
	drainPlain(2)

	spell := &spells.SpellData{SpellId: "sparks", Name: "Sparks", EffectType: "damage", DamageMultiplier: 0.8, BaseFolds: 4}
	applyPlayerEffect(users.GetByUserId(1), users.GetByUserId(2), rooms.LoadRoom(1), spell, 10, spellContestAttackWin())
	target := drainPlain(2)
	assert.Equal(t, 1, countContaining(target, "Something's Sparks strikes you!"))
	assert.Equal(t, 0, countContaining(target, "Aliceia"))
}

func TestSpellDefence_DefenderInTheDarkIsToldWithTheAttackerHidden(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	darken(t, 1)
	drainPlain(1)
	drainPlain(2)

	out := combat.ChannelDefenceResult{Defended: true, DefenceType: "dodge"}
	sendSpellChannelDefenceMessages(rooms.LoadRoom(1), messaging.CategorySpellVital, out,
		"Aliceia", "Bobrick", "Hex", users.GetByUserId(1), users.GetByUserId(2))

	defender := drainPlain(2)
	assert.Equal(t, 1, countContaining(defender, "You withstand something's Hex."),
		"a defender who cannot see used to be told nothing at all")
	assert.Equal(t, 0, countContaining(defender, "Aliceia"))
	assert.Equal(t, 1, countContaining(drainPlain(1), "Something withstands your Hex."))
}

func TestMobCastOnPlayer_TargetInTheDarkReadsSomething(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	darken(t, 1)
	drainPlain(2)
	original := runSpellChannelAttack
	runSpellChannelAttack = func(combat.AttackChannel, combat.AttackSide, *characters.Character, *characters.Character) combat.ChannelDefenceResult {
		return spellContestAttackWin()
	}
	t.Cleanup(func() { runSpellChannelAttack = original })

	spell := &spells.SpellData{SpellId: "test-hex", Name: "Hex", Type: spells.HarmSingle}
	resolveMobSpellAgainstPlayer(mobs.GetInstance(100), users.GetByUserId(2), rooms.LoadRoom(1), spell, combat.AttackSide{}, 10)

	target := drainPlain(2)
	assert.Equal(t, 1, countContaining(target, "Something's Hex takes effect on you."))
	assert.Equal(t, 0, countContaining(target, "Skeleton"))
}

func TestPurgeAffliction_CrossCastInTheDarkNamesNobody(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	darken(t, 1)
	drainPlain(1)
	drainPlain(2)

	resolvePurgeAffliction(users.GetByUserId(1), users.GetByUserId(2))
	assert.Equal(t, 1, countContaining(drainPlain(2), "Something purges the afflictions from your body."))
	assert.Equal(t, 1, countContaining(drainPlain(1), "You direct purging energy towards something."))
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/hooks/ -run 'TestCrossCast|TestSpellDefence_|TestMobCastOnPlayer_|TestPurgeAffliction_CrossCast'`
Expected: FAIL. The dark cases read the names (`Aliceia's Heal envelops you`); the defence case finds no defender line at all. The lit case passes already, which is the point: it must keep passing.

- [ ] **Step 3: The audience helper**

Create `internal/hooks/spell_audience.go`:

```go
package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// spellAudience is the messaging.Audience for a spell between two parties.
// Either user may be nil (a mob caster or a mob target). actorName and
// acteeName must be the exact strings the lines print.
//
// A nil *users.UserRecord is never stored in the Recipient fields: a typed nil
// would make the interface non-nil, and SendTrio would call through it.
func spellAudience(caster *users.UserRecord, actorName string, target *users.UserRecord, acteeName string, room *rooms.Room) messaging.Audience {
	aud := messaging.Audience{
		ActorName: actorName,
		ActeeName: acteeName,
	}
	if caster != nil {
		aud.Actor = caster
		aud.ActorId = caster.UserId
	}
	if target != nil {
		aud.Actee = target
		aud.ActeeId = target.UserId
	}
	if room != nil {
		aud.Room = room
	}
	return aud
}
```

The Audience guard (Task 4) accepts this literal: it names no recipient without its id, and it names both `ActorName` and `ActeeName`.

- [ ] **Step 4: `applyPlayerEffect` cross-casts**

In `internal/hooks/spell_resolution.go`, inside `applyPlayerEffect`:

**damage**: replace the whole `if !out.Defended { ... }` block that follows `dmgDesc := ...` with:

```go
		if !out.Defended {
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.Say(spellSchoolCategory(spellData), fmt.Sprintf(
					`Your %s strikes `+
						`<ansi fg="username">%s</ansi>! `+
						`(<ansi fg="damage">%s</ansi>)%s`,
					spellData.Name, target.Character.Name, dmgDesc, critTag)),
				Actee: messaging.Say(spellSchoolCategory(spellData), fmt.Sprintf(
					`<ansi fg="red"><ansi fg="username">%s</ansi>'s `+
						`%s strikes you! `+
						`(<ansi fg="damage">%s</ansi>)</ansi>`,
					user.Character.Name, spellData.Name,
					combat.GetDamageDescription(dmg, target.Character.HealthMax.Value))),
				Observer: messaging.Say(spellSchoolCategory(spellData), fmt.Sprintf(
					`<ansi fg="username">%s</ansi>'s <ansi fg="cyan">%s</ansi> strikes `+
						`<ansi fg="username">%s</ansi>!`,
					user.Character.Name, spellData.Name, target.Character.Name)),
			}, spellAudience(user, user.Character.Name, target, target.Character.Name, room))
		}
```

**purge**: replace the `if target.UserId != user.UserId { ... }` branch (keep the `else` self-cast branch exactly as it is) with:

```go
		if target.UserId != user.UserId {
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.Say(messaging.CategorySpellVital, fmt.Sprintf(
					`<ansi fg="green">Your %s cleanses <ansi fg="username">%s</ansi> of afflictions.%s</ansi>`,
					spellData.Name, target.Character.Name, critTag)),
				Actee: messaging.Say(messaging.CategorySpellVital, fmt.Sprintf(
					`<ansi fg="green"><ansi fg="username">%s</ansi>'s %s purges the toxins from your body.</ansi>`,
					user.Character.Name, spellData.Name)),
				Observer: messaging.Say(messaging.CategorySpellVital, fmt.Sprintf(
					`<ansi fg="username">%s</ansi>'s <ansi fg="cyan">%s</ansi> cleanses <ansi fg="username">%s</ansi>.`,
					user.Character.Name, spellData.Name, target.Character.Name)),
			}, spellAudience(user, user.Character.Name, target, target.Character.Name, room))
		} else {
```

**heal**: replace the cross-cast branch the same way:

```go
		if target.UserId != user.UserId {
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.Say(messaging.CategorySpellVital, fmt.Sprintf(
					`<ansi fg="green">You weave restorative magic around <ansi fg="username">%s</ansi>.%s</ansi>`,
					target.Character.Name, critTag)),
				Actee: messaging.Say(messaging.CategorySpellVital, fmt.Sprintf(
					`<ansi fg="green"><ansi fg="username">%s</ansi>'s %s envelops you in healing energy. Your wounds begin to mend.</ansi>`,
					user.Character.Name, spellData.Name)),
				Observer: messaging.Say(messaging.CategorySpellVital, fmt.Sprintf(
					`<ansi fg="username">%s</ansi>'s <ansi fg="cyan">%s</ansi> envelops <ansi fg="username">%s</ansi> in healing light.`,
					user.Character.Name, spellData.Name, target.Character.Name)),
			}, spellAudience(user, user.Character.Name, target, target.Character.Name, room))
		} else {
```

**buff**: replace the cross-cast branch:

```go
		if target.UserId != user.UserId {
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.Say(spellSchoolCategory(spellData), fmt.Sprintf(
					`Your %s takes effect on <ansi fg="username">%s</ansi>!%s`,
					spellData.Name, target.Character.Name, critTag)),
				Actee: messaging.Say(spellSchoolCategory(spellData), fmt.Sprintf(
					`<ansi fg="username">%s</ansi>'s %s takes effect on you!`,
					user.Character.Name, spellData.Name)),
				Observer: messaging.Say(spellSchoolCategory(spellData), fmt.Sprintf(
					`<ansi fg="username">%s</ansi>'s <ansi fg="cyan">%s</ansi> settles over <ansi fg="username">%s</ansi>.`,
					user.Character.Name, spellData.Name, target.Character.Name)),
			}, spellAudience(user, user.Character.Name, target, target.Character.Name, room))
		} else {
```

**shield**: replace from `target.SendText(spellSchoolCategory(spellData), `A shimmering magical barrier forms around you, bolstering your defenses.`)` through the `sendVisualRoomText(... A shimmering barrier surrounds ...)` call with:

```go
		if target.UserId != user.UserId {
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.Say(spellSchoolCategory(spellData), fmt.Sprintf(
					`A shimmering magical barrier forms around <ansi fg="username">%s</ansi>, bolstering their defenses.`,
					target.Character.Name)),
				Actee: messaging.Say(spellSchoolCategory(spellData),
					`A shimmering magical barrier forms around you, bolstering your defenses.`),
				Observer: messaging.Say(spellSchoolCategory(spellData), fmt.Sprintf(
					`A shimmering barrier surrounds <ansi fg="username">%s</ansi>.`, target.Character.Name)),
			}, spellAudience(user, user.Character.Name, target, target.Character.Name, room))
		} else {
			target.SendText(spellSchoolCategory(spellData), `A shimmering magical barrier forms around you, bolstering your defenses.`)
			sendVisualRoomText(room, spellSchoolCategory(spellData), fmt.Sprintf(
				`A shimmering barrier surrounds <ansi fg="username">%s</ansi>.`, target.Character.Name), target.UserId)
		}
```

One deliberate change: on a cross-cast shield the caster no longer also receives the room line. It excluded only the target before; the seam excludes both parties, as every other cross-cast already did.

**default**: replace the `else` branch that sends `Your %s takes effect on ...` with:

```go
		} else {
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.Say(spellSchoolCategory(spellData), fmt.Sprintf(
					`Your %s takes effect on <ansi fg="username">%s</ansi>.`,
					spellData.Name, target.Character.Name)),
				Actee:    messaging.NoLine,
				Observer: messaging.NoLine,
			}, spellAudience(user, user.Character.Name, target, target.Character.Name, room))
		}
```

Self-cast branches are untouched.

- [ ] **Step 5: `sendSpellChannelDefenceMessages`**

Replace everything in that function after `if triad.ToRoom == "" { return }` with:

```go
	messaging.SendTrio(messaging.Trio{
		Actor:    messaging.Say(category, string(triad.ToAttacker)),
		Actee:    messaging.Say(category, string(triad.ToDefender)),
		Observer: messaging.Say(category, string(triad.ToRoom)),
	}, spellAudience(attackerUser, attackerName, defenderUser, defenderName, room))
```

The identities passed in are the strings the triad printed, so the names match exactly. A participant who cannot see now reads the line with the other party hidden, where `SendTextVisualToUser` used to drop it.

- [ ] **Step 6: `resolveMobSpellAgainstPlayer`**

Every player-facing line here gets `spellAudience(nil, caster.Character.Name, target, target.Character.Name, room)`.

**damage**: replace the `if !out.Defended { ... }` block with:

```go
		if !out.Defended {
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.NoLine,
				Actee: messaging.Say(spellSchoolCategory(spellData), fmt.Sprintf(
					`<ansi fg="mobname">%s</ansi>'s <ansi fg="cyan">%s</ansi> `+
						`strikes you! (<ansi fg="damage">%s</ansi>)%s`,
					caster.Character.Name, spellData.Name,
					combat.GetDamageDescription(dmg, target.Character.HealthMax.Value), critTag)),
				Observer: messaging.Say(spellSchoolCategory(spellData), fmt.Sprintf(
					`<ansi fg="mobname">%s</ansi>'s <ansi fg="cyan">%s</ansi> strikes `+
						`<ansi fg="username">%s</ansi>!`,
					caster.Character.Name, spellData.Name, target.Character.Name)),
			}, spellAudience(nil, caster.Character.Name, target, target.Character.Name, room))
		}
```

**dot**: replace the `target.SendText(... afflicts you!%s ...)` and following `sendVisualRoomText(... afflicts ...)` with:

```go
		messaging.SendTrio(messaging.Trio{
			Actor: messaging.NoLine,
			Actee: messaging.Say(spellSchoolCategory(spellData), fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi>'s <ansi fg="cyan">%s</ansi> afflicts you!%s`,
				caster.Character.Name, spellData.Name, critTag)),
			Observer: messaging.Say(spellSchoolCategory(spellData), fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi>'s <ansi fg="cyan">%s</ansi> afflicts <ansi fg="username">%s</ansi>!`,
				caster.Character.Name, spellData.Name, target.Character.Name)),
		}, spellAudience(nil, caster.Character.Name, target, target.Character.Name, room))
```

**knockdown**: replace the `if knocked { ... } else if !out.Defended { ... }` pair with:

```go
		if knocked {
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.NoLine,
				Actee: messaging.Say(spellSchoolCategory(spellData), fmt.Sprintf(
					`<ansi fg="mobname">%s</ansi>'s <ansi fg="cyan">%s</ansi> slams you `+
						`to the ground! (<ansi fg="damage">%s</ansi>)%s`,
					caster.Character.Name, spellData.Name,
					combat.GetDamageDescription(dmg, target.Character.HealthMax.Value), critTag)),
				Observer: messaging.Say(spellSchoolCategory(spellData), fmt.Sprintf(
					`<ansi fg="mobname">%s</ansi>'s <ansi fg="cyan">%s</ansi> knocks `+
						`<ansi fg="username">%s</ansi> to the ground!`,
					caster.Character.Name, spellData.Name, target.Character.Name)),
			}, spellAudience(nil, caster.Character.Name, target, target.Character.Name, room))
		} else if !out.Defended {
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.NoLine,
				Actee: messaging.Say(spellSchoolCategory(spellData), fmt.Sprintf(
					`<ansi fg="mobname">%s</ansi>'s <ansi fg="cyan">%s</ansi> strikes you, but you're `+
						`already down. (<ansi fg="damage">%s</ansi>)%s`,
					caster.Character.Name, spellData.Name,
					combat.GetDamageDescription(dmg, target.Character.HealthMax.Value), critTag)),
				Observer: messaging.NoLine,
			}, spellAudience(nil, caster.Character.Name, target, target.Character.Name, room))
		}
```

**buff**: replace the `target.SendText(... takes effect on you!%s ...)` and `sendVisualRoomText(... affects ...)` with:

```go
		messaging.SendTrio(messaging.Trio{
			Actor: messaging.NoLine,
			Actee: messaging.Say(spellSchoolCategory(spellData), fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi>'s <ansi fg="cyan">%s</ansi> takes effect on you!%s`,
				caster.Character.Name, spellData.Name, critTag)),
			Observer: messaging.Say(spellSchoolCategory(spellData), fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi>'s <ansi fg="cyan">%s</ansi> affects <ansi fg="username">%s</ansi>!`,
				caster.Character.Name, spellData.Name, target.Character.Name)),
		}, spellAudience(nil, caster.Character.Name, target, target.Character.Name, room))
```

**default**: replace the `target.SendText(... takes effect on you. ...)` with:

```go
		messaging.SendTrio(messaging.Trio{
			Actor: messaging.NoLine,
			Actee: messaging.Say(spellSchoolCategory(spellData), fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi>'s <ansi fg="cyan">%s</ansi> takes effect on you.`,
				caster.Character.Name, spellData.Name)),
			Observer: messaging.NoLine,
		}, spellAudience(nil, caster.Character.Name, target, target.Character.Name, room))
```

The backfire line (`'s spell backfires!`) is observer-only with no party to hide and stays as it is.

- [ ] **Step 7: Purge Affliction**

In `internal/hooks/spell_purgeaffliction.go`, replace the `if user.UserId != target.UserId { ... }` branch (keep the `else`) with:

```go
	if user.UserId != target.UserId {
		messaging.SendTrio(messaging.Trio{
			Actor: messaging.Say(messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="green">You direct purging energy towards <ansi fg="username">%s</ansi>.</ansi>`,
				target.Character.Name)),
			Actee: messaging.Say(messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="green"><ansi fg="username">%s</ansi> purges the afflictions from your body.</ansi>`,
				user.Character.Name)),
			Observer: messaging.Say(messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="username">%s</ansi> directs purging energy towards <ansi fg="username">%s</ansi>.`,
				user.Character.Name, target.Character.Name)),
		}, spellAudience(user, user.Character.Name, target, target.Character.Name, room))
	} else {
```

`spellAudience` leaves `Room` unset when `rooms.LoadRoom` returned nil, which matches the old `if room != nil` guard.

- [ ] **Step 8: Run the tests to verify they pass**

```bash
go build ./...
go test ./internal/hooks/
go test . -run 'TestNarrationSitesMatchViewpointAudit|TestEveryTrioLiteralNamesAllThreeRoles|TestEveryAudienceLiteralPairsIdsWithRecipients'
```

Expected: PASS. The existing self-cast tests (`selfcast_wording_test.go`) and the 5a narration tests stay green. If `TestNarrationSitesMatchViewpointAudit` reports a registry key as vanished or new, read the site against source before editing the registry, as that guard's doc comment requires; no change to it is expected, because only complete three-role events moved.

- [ ] **Step 9: Sabotage probe**

In `spellAudience`, change `ActorName: actorName,` to `ActorName: messaging.NoName,`. Run `go test ./internal/hooks/ -run TestCrossCastHeal_InTheDark`. Expected: FAIL, the target reads `Aliceia's Heal`. Undo with Edit.

- [ ] **Step 10: Gates and commit**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/hooks/ && go test .
git add internal/hooks/spell_audience.go internal/hooks/spell_resolution.go internal/hooks/spell_purgeaffliction.go internal/hooks/spell_names_in_dark_test.go
git commit -m "fix(spells): spell lines stop naming unseen casters and targets" -m "Cross-cast lines, the spell defence triad, mob casts on players and Purge Affliction go through SendTrio. A target in the dark reads Something's Heal; a defender who could not see is now told they defended; infrared reads a figure." -m "Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>" -m "Claude-Session: https://claude.ai/code/session_01J9ARXi13pxi92SKumuvpum"
```

---
### Task 8: One rule for who you perceive

**Files:**
- Modify: `internal/characters/character.go` (add `Perceives` after `IsHidden`, `:878-883`)
- Modify: `internal/rooms/roomdetails.go:253-257`, `:298-302`, `:316-320`
- Test: `internal/characters/perceives_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/characters/perceives_test.go`:

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
)

const perceivesVeilBuffId = 7301

func perceivesChar(t *testing.T, name string) *Character {
	t.Helper()
	c := New()
	c.Name = name
	c.Awareness = awareness.NewMachine()
	return c
}

// perceivesHide puts c into the Awareness Hidden state, the only thing
// IsHidden reads. Concealing then resolving is the only route into it.
func perceivesHide(t *testing.T, c *Character) {
	t.Helper()
	reason := state.TransitionReason{Trigger: "perceives_test"}
	if err := c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason); err != nil {
		t.Fatalf("concealing: %v", err)
	}
	c.Awareness.ResolveConcealment(true, reason)
	if !c.IsHidden() {
		t.Fatal("precondition: the fixture should now be hidden")
	}
}

func TestPerceives(t *testing.T) {
	t.Run("a creature that is not hidden", func(t *testing.T) {
		if !perceivesChar(t, "Viewer").Perceives(perceivesChar(t, "Kesh")) {
			t.Fatal("anyone perceives a creature that is not hidden")
		}
	})

	t.Run("a hidden creature, no see-hidden", func(t *testing.T) {
		kesh := perceivesChar(t, "Kesh")
		perceivesHide(t, kesh)
		if perceivesChar(t, "Viewer").Perceives(kesh) {
			t.Fatal("a hidden creature must not be perceived without see-hidden")
		}
	})

	t.Run("yourself, while hidden", func(t *testing.T) {
		kesh := perceivesChar(t, "Kesh")
		perceivesHide(t, kesh)
		if !kesh.Perceives(kesh) {
			t.Fatal("a hider always perceives themselves")
		}
	})

	t.Run("see-hidden from a buff, with no pet", func(t *testing.T) {
		t.Cleanup(buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
			perceivesVeilBuffId: {BuffId: perceivesVeilBuffId, Name: "Test Veil", Flags: []buffs.Flag{buffs.SeeHidden}},
		}))
		viewer := perceivesChar(t, "Viewer")
		if err := viewer.AddBuff(perceivesVeilBuffId, true); err != nil {
			t.Fatalf("applying see-hidden: %v", err)
		}
		if viewer.Pet.Exists() {
			t.Fatal("precondition: the viewer must have no pet")
		}
		kesh := perceivesChar(t, "Kesh")
		perceivesHide(t, kesh)
		if !viewer.Perceives(kesh) {
			t.Fatal("see-hidden alone reveals a hidden creature; no pet is involved")
		}
	})

	t.Run("see-hidden from a mutation", func(t *testing.T) {
		t.Cleanup(mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
			"test-eyes": {MutationId: "test-eyes", Name: "Test Eyes",
				Pros: []mutations.MutationEffect{{Type: "flag", Target: string(buffs.SeeHidden), Value: 1}}},
		}))
		viewer := perceivesChar(t, "Viewer")
		viewer.Mutations = map[string]int{"test-eyes": 1}
		kesh := perceivesChar(t, "Kesh")
		perceivesHide(t, kesh)
		if !viewer.Perceives(kesh) {
			t.Fatal("a see-hidden mutation reveals a hidden creature")
		}
	})

	t.Run("nothing", func(t *testing.T) {
		if perceivesChar(t, "Viewer").Perceives(nil) {
			t.Fatal("a nil creature is not perceived")
		}
	})
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/characters/ -run TestPerceives`
Expected: FAIL to compile, `c.Perceives undefined`.

- [ ] **Step 3: Write `Perceives`**

In `internal/characters/character.go`, directly after `IsHidden`:

```go
// Perceives reports whether c can make out other in the same room: other is c,
// other is not hidden, or c has see-hidden from any source (a buff or a
// mutation). It is the one rule for the room listing (rooms/roomdetails.go)
// and for naming a creature (rooms.Room.FindByNameSeenBy), so the two cannot
// disagree.
//
// No pet is involved. The listing used to require a pet as well, a leftover
// from upstream GoMud where see-hidden was a pet power (commit 463a76727).
// No player can own a pet in DOGMud, so that check hid every hidden creature
// from everyone.
func (c *Character) Perceives(other *Character) bool {
	if other == nil {
		return false
	}
	if c == other || !other.IsHidden() {
		return true
	}
	return c.HasFlagFromAnySource(buffs.SeeHidden)
}
```

`character.go` already imports `internal/buffs`.

- [ ] **Step 4: The room listing reads it**

In `internal/rooms/roomdetails.go`, replace the player check:

```go
				if player.Character.IsHidden() { // Don't show them if sneaking or camo
					if !user.Character.Pet.Exists() || !user.Character.HasFlagFromAnySource(buffs.SeeHidden) {
						continue
					}
				}
```

with:

```go
				if !user.Character.Perceives(player.Character) { // sneaking or camo, and no see-hidden
					continue
				}
```

Replace the mob count loop's check:

```go
			if mob.Character.IsHidden() {
				if !user.Character.Pet.Exists() || !user.Character.HasFlagFromAnySource(buffs.SeeHidden) {
					continue
				}
			}
```

with:

```go
			if !user.Character.Perceives(&mob.Character) {
				continue
			}
```

Replace the mob listing loop's check:

```go
			if mob.Character.IsHidden() { // Don't show them if sneaking or camo
				if !user.Character.Pet.Exists() || !user.Character.HasFlagFromAnySource(buffs.SeeHidden) {
					continue
				}
			}
```

with:

```go
			if !user.Character.Perceives(&mob.Character) { // sneaking or camo, and no see-hidden
				continue
			}
```

`roomdetails.go` still uses `buffs.Sleeping`, so its import stays. Confirm: `grep -n "SeeHidden\|Pet.Exists() ||" internal/rooms/roomdetails.go` prints nothing.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go build ./... && go test ./internal/characters/ ./internal/rooms/`
Expected: PASS.

- [ ] **Step 6: Sabotage probe**

In `Perceives`, change the last line to `return c.Pet.Exists() && c.HasFlagFromAnySource(buffs.SeeHidden)`. Run `go test ./internal/characters/ -run TestPerceives`. Expected: FAIL on `see-hidden from a buff, with no pet`. Undo with Edit.

- [ ] **Step 7: Gates and commit**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/characters/ ./internal/rooms/ && go test .
git add internal/characters/character.go internal/characters/perceives_test.go internal/rooms/roomdetails.go
git commit -m "fix(rooms): see-hidden reveals hidden creatures without a pet" -m "Character.Perceives is the one hidden-creature rule, and the three Also here checks read it. The pet requirement was an upstream leftover from when see-hidden was a pet power; no player can own a pet, so Veil Sight, the Cat's Eye Draught and five mutations revealed nothing." -m "Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>" -m "Claude-Session: https://claude.ai/code/session_01J9ARXi13pxi92SKumuvpum"
```

---

### Task 9: A lookup that respects the looker

**Files:**
- Modify: `internal/rooms/rooms.go:1890-2047` (`FindByName`, add `FindByNameSeenBy`, `findPlayerByName`, `findMobByName`; import `internal/characters`)
- Modify: `internal/actions/target_resolution.go` (`ResolveTargetOptions.Viewer`; the lookup call; import `internal/characters`)
- Modify: `internal/rooms/respawn_targeting_test.go:221`, `internal/rooms/stale_mob_ids_test.go:36`, `:100`
- Test: `internal/rooms/find_seen_by_test.go` (create)
- Test: `internal/actions/target_viewer_test.go` (create)

- [ ] **Step 1: Write the failing room test**

Create `internal/rooms/find_seen_by_test.go`:

```go
package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const (
	seenByRoomId        = 7600
	seenByViewerId      = 7601
	seenByHiderId       = 7602
	seenByHiddenGuardId = 7611
	seenByGuardId       = 7612
	seenByVeilBuffId    = 7621
)

func seenByHide(t *testing.T, c *characters.Character) {
	t.Helper()
	reason := state.TransitionReason{Trigger: "find_seen_by_test"}
	if err := c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason); err != nil {
		t.Fatalf("concealing: %v", err)
	}
	c.Awareness.ResolveConcealment(true, reason)
	if !c.IsHidden() {
		t.Fatal("precondition: the fixture should now be hidden")
	}
}

// seenByRoom holds the viewer Aliceia, a hidden player Kesh, and two guards:
// the first hidden, the second not.
func seenByRoom(t *testing.T) (*Room, *characters.Character) {
	t.Helper()
	t.Cleanup(buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		seenByVeilBuffId: {BuffId: seenByVeilBuffId, Name: "Test Veil", Flags: []buffs.Flag{buffs.SeeHidden}},
	}))
	viewer := users.NewTestUser(seenByViewerId, "aliceia", "Aliceia", 97601)
	hider := users.NewTestUser(seenByHiderId, "kesh", "Kesh", 97602)
	seenByHide(t, hider.Character)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		seenByViewerId: viewer,
		seenByHiderId:  hider,
	}))

	r := &Room{RoomId: seenByRoomId}
	r.AddPlayer(seenByViewerId)
	r.AddPlayer(seenByHiderId)
	for _, g := range []struct {
		id     int
		hidden bool
	}{{seenByHiddenGuardId, true}, {seenByGuardId, false}} {
		m := &mobs.Mob{InstanceId: g.id}
		m.Character.Name = "Guard"
		m.Character.RoomId = seenByRoomId
		m.Character.Buffs = buffs.New()
		m.Character.Awareness = awareness.NewMachine()
		if g.hidden {
			seenByHide(t, &m.Character)
		}
		mobs.SetInstanceForTest(g.id, m)
		id := g.id
		t.Cleanup(func() { mobs.SetInstanceForTest(id, nil) })
		r.AddMob(g.id)
	}
	return r, viewer.Character
}

func TestFindByNameSeenBy_HiddenPlayerCannotBeNamed(t *testing.T) {
	r, viewer := seenByRoom(t)
	if pId, _ := r.FindByNameSeenBy(viewer, "kesh"); pId != 0 {
		t.Errorf("a viewer without see-hidden named the hidden player (%d)", pId)
	}
	if pId, _ := r.FindByNameSeenBy(viewer, "@7602"); pId != 0 {
		t.Errorf("@id must respect perception too, got %d", pId)
	}
	if pId, _ := r.FindByName("kesh"); pId != seenByHiderId {
		t.Errorf("FindByName, the unfiltered form for staff and mobs, = %d, want %d", pId, seenByHiderId)
	}
}

func TestFindByNameSeenBy_SeeHiddenNamesThem(t *testing.T) {
	r, viewer := seenByRoom(t)
	if err := viewer.AddBuff(seenByVeilBuffId, true); err != nil {
		t.Fatalf("applying see-hidden: %v", err)
	}
	if pId, _ := r.FindByNameSeenBy(viewer, "kesh"); pId != seenByHiderId {
		t.Errorf("with see-hidden = %d, want %d", pId, seenByHiderId)
	}
	if _, mId := r.FindByNameSeenBy(viewer, "2.guard"); mId != seenByGuardId {
		t.Errorf("with see-hidden, 2.guard = %d, want the second guard %d", mId, seenByGuardId)
	}
}

func TestFindByNameSeenBy_OrdinalsCountOnlyWhatYouPerceive(t *testing.T) {
	r, viewer := seenByRoom(t)
	if _, mId := r.FindByNameSeenBy(viewer, "guard"); mId != seenByGuardId {
		t.Errorf("guard = %d, want the only guard the viewer perceives, %d", mId, seenByGuardId)
	}
	if _, mId := r.FindByNameSeenBy(viewer, "2.guard"); mId != 0 {
		t.Errorf("2.guard = %d, want nothing: the viewer perceives one guard", mId)
	}
	if _, mId := r.FindByNameSeenBy(viewer, "#7611"); mId != 0 {
		t.Errorf("#id must respect perception too, got %d", mId)
	}
}

func TestFindByNameSeenBy_NilViewerIsFindByName(t *testing.T) {
	r, _ := seenByRoom(t)
	for _, name := range []string{"kesh", "guard", "2.guard", "@7602", "#7611"} {
		wantP, wantM := r.FindByName(name)
		gotP, gotM := r.FindByNameSeenBy(nil, name)
		if gotP != wantP || gotM != wantM {
			t.Errorf("%q: nil viewer = (%d, %d), FindByName = (%d, %d)", name, gotP, gotM, wantP, wantM)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/rooms/ -run TestFindByNameSeenBy`
Expected: FAIL to compile, `r.FindByNameSeenBy undefined`.

- [ ] **Step 3: Implement the lookup**

In `internal/rooms/rooms.go`, add `"github.com/GoMudEngine/GoMud/internal/characters"` to the imports and replace `FindByName`:

```go
// FindByName resolves a player and a mob by name with no perception limit.
// Staff tools and mob callers use it; a player command that names a creature
// uses FindByNameSeenBy.
func (r *Room) FindByName(searchName string, findTypes ...FindFlag) (playerId int, mobInstanceId int) {
	return r.FindByNameSeenBy(nil, searchName, findTypes...)
}

// FindByNameSeenBy resolves a player and a mob by name as viewer would: a
// creature viewer does not perceive (characters.Character.Perceives) is skipped
// BEFORE matching, so it cannot be named and does not count toward `2.name` or
// `name#2`. The room listing reads the same rule. A nil viewer is FindByName.
func (r *Room) FindByNameSeenBy(viewer *characters.Character, searchName string, findTypes ...FindFlag) (playerId int, mobInstanceId int) {
	if len(findTypes) < 1 {
		findTypes = []FindFlag{FindAll}
	}
	mobInstanceId, _ = r.findMobByName(viewer, searchName, findTypes...)
	playerId, _ = r.findPlayerByName(viewer, searchName, findTypes...)
	return playerId, mobInstanceId
}
```

In `findPlayerByName`, add the `viewer *characters.Character` first parameter. In the `@` branch, replace:

```go
				if userIdMatch > 0 {
					if uId != userIdMatch {
						continue
					}
					return uId, nil
				}
```

with:

```go
				if userIdMatch > 0 {
					if uId != userIdMatch {
						continue
					}
					if viewer != nil {
						if u := users.GetByUserId(uId); u == nil || !viewer.Perceives(u.Character) {
							return 0, errors.New("user not found")
						}
					}
					return uId, nil
				}
```

In the name loop, directly after the stale-id guard `if u == nil { continue }`, add:

```go
		if viewer != nil && !viewer.Perceives(u.Character) {
			continue
		}
```

In `findMobByName`, add the `viewer *characters.Character` first parameter. In the `#` branch, replace:

```go
				if mobIdMatch > 0 {
					if mId != mobIdMatch {
						continue
					}
					return mId, nil
				}
```

with:

```go
				if mobIdMatch > 0 {
					if mId != mobIdMatch {
						continue
					}
					if viewer != nil {
						if m := mobs.GetInstance(mId); m == nil || !viewer.Perceives(&m.Character) {
							return 0, errors.New("mob not found")
						}
					}
					return mId, nil
				}
```

In the name loop, directly after the stale-id guard `if m == nil { continue }`, add:

```go
		if viewer != nil && !viewer.Perceives(&m.Character) {
			continue
		}
```

Update the three existing test calls to pass `nil`:
- `internal/rooms/respawn_targeting_test.go:221`: `room.findPlayerByName(nil, "tester")`
- `internal/rooms/stale_mob_ids_test.go:36`: `r.findMobByName(nil, "sala")`
- `internal/rooms/stale_mob_ids_test.go:100`: `r.findMobByName(nil, "#1")`

- [ ] **Step 4: Run the room tests to verify they pass**

Run: `go build ./... && go test ./internal/rooms/`
Expected: PASS.

- [ ] **Step 5: Write the failing resolver test**

Create `internal/actions/target_viewer_test.go`:

```go
package actions

import (
	"errors"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func viewerTestHide(t *testing.T, c *characters.Character) {
	t.Helper()
	reason := state.TransitionReason{Trigger: "target_viewer_test"}
	require.NoError(t, c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason))
	c.Awareness.ResolveConcealment(true, reason)
	require.True(t, c.IsHidden())
}

// viewerTestRoom holds Aliceia (7701), who looks, and Kesh (7702), who hides.
func viewerTestRoom(t *testing.T) (*rooms.Room, *users.UserRecord) {
	t.Helper()
	viewer := users.NewTestUser(7701, "aliceia", "Aliceia", 97701)
	hider := users.NewTestUser(7702, "kesh", "Kesh", 97702)
	viewerTestHide(t, hider.Character)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{7701: viewer, 7702: hider}))
	room := &rooms.Room{RoomId: 7700}
	room.AddPlayer(7701)
	room.AddPlayer(7702)
	return room, viewer
}

func TestResolveTargetActor_ViewerCannotNameAHiddenCreature(t *testing.T) {
	room, viewer := viewerTestRoom(t)
	_, err := ResolveTargetActor(room, "kesh", ResolveTargetOptions{Viewer: viewer.Character})
	require.True(t, errors.Is(err, ErrTargetNotFound), "got %v", err)
}

func TestResolveTargetActor_NoViewerStillReachesThem(t *testing.T) {
	room, _ := viewerTestRoom(t)
	target, err := ResolveTargetActor(room, "kesh")
	require.NoError(t, err, "staff tools pass no viewer and must still reach a hidden player")
	require.Equal(t, 7702, target.GetUserId())
}
```

- [ ] **Step 6: Run it to verify it fails**

Run: `go test ./internal/actions/ -run TestResolveTargetActor_`
Expected: FAIL to compile, `unknown field Viewer in struct literal`.

- [ ] **Step 7: Add the option**

In `internal/actions/target_resolution.go`, add `"github.com/GoMudEngine/GoMud/internal/characters"` to the imports, and add to `ResolveTargetOptions` after `ExcludeMobInstanceId int`:

```go
	// Viewer is the character doing the looking. When set, a creature it does
	// not perceive (characters.Character.Perceives) cannot be named. Every
	// player command that names a creature sets it; staff tools and mob
	// callers leave it nil. The root lookup guard enforces the split.
	Viewer *characters.Character
```

Replace `playerId, mobInstanceId := r.FindByName(name, flags...)` with:

```go
	playerId, mobInstanceId := r.FindByNameSeenBy(o.Viewer, name, flags...)
```

- [ ] **Step 8: Run the tests to verify they pass**

Run: `go build ./... && go test ./internal/actions/ ./internal/rooms/`
Expected: PASS.

- [ ] **Step 9: Sabotage probe**

In `findPlayerByName`'s name loop, delete the `if viewer != nil && !viewer.Perceives(u.Character) { continue }` block. Run `go test ./internal/rooms/ ./internal/actions/ -run 'TestFindByNameSeenBy_HiddenPlayer|TestResolveTargetActor_Viewer'`. Expected: both FAIL. Undo with Edit.

- [ ] **Step 10: Gates and commit**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/rooms/ ./internal/actions/ && go test .
git add internal/rooms/rooms.go internal/rooms/find_seen_by_test.go internal/rooms/respawn_targeting_test.go internal/rooms/stale_mob_ids_test.go internal/actions/target_resolution.go internal/actions/target_viewer_test.go
git commit -m "feat(rooms): FindByNameSeenBy and ResolveTargetOptions.Viewer" -m "A creature the viewer does not perceive is skipped before matching, so it cannot be named and does not count toward 2.name. A nil viewer is the old FindByName." -m "Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>" -m "Claude-Session: https://claude.ai/code/session_01J9ARXi13pxi92SKumuvpum"
```

---
### Task 10: Every player command that names a creature passes the viewer

**Files:**
- Create: `lookup_viewer_guard_test.go` (repo root)
- Create: `internal/actions/cast_sight.go` (`castViewer` only; Task 11 extends it)
- Modify: `internal/actions/combat_attack.go` (`FindAttackTarget` signature, pools, named branch; imports)
- Modify: `internal/actions/combat_test.go:639`, `:657`, `:672`, `:693`, `:712`
- Modify: `internal/actions/cast.go:107`, `:186`, `:213`
- Modify: `internal/actions/combat_fire.go:182-187`
- Modify: `internal/actions/melee_target.go:172`
- Modify: `internal/actions/buy.go:319-321`
- Modify: `internal/parser/adapters.go` (`mobAdapter`, `playerAdapter`; import)
- Modify: `internal/usercommands/{attack,ask,consider,give,look,party,report,sell,shoot,show,talk,target}.go`, `skill.skullduggery.{plant,shadow,steal}.go`
- Modify: `internal/mobcommands/attack.go:42`
- Modify: `modules/follow/follow.go:410`
- Test: append to `internal/actions/target_viewer_test.go`

- [ ] **Step 1: Write the lookup registry guard (the failing test)**

Create `lookup_viewer_guard_test.go` in the repo root:

```go
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// lookupEntry records how one function looks creatures up by name: calls that
// pass the looker as a viewer (so a creature they do not perceive cannot be
// named), and calls that do not, with the reason that is right.
type lookupEntry struct {
	viewer int
	plain  int
	why    string
}

const (
	whyMob      = "a mob caller; mobs perceiving hidden creatures is slice F"
	whyStaff    = "a staff tool; it must reach every character, hidden or not"
	whySelf     = "only asks whether the typed name is the looker themselves, whom they always perceive"
	whyUnfilter = "the unfiltered form itself, kept for staff tools and mob callers"
)

// lookupRegistry is every non-test creature lookup in internal/ and modules/,
// keyed "path|function". Follow-up slice A, owner ruling 9: every player
// command that names a creature passes the player as viewer.
var lookupRegistry = map[string]lookupEntry{
	"internal/actions/buy.go|Buy":                                       {viewer: 1, plain: 1, why: whySelf},
	"internal/actions/cast.go|InitiateCast":                             {viewer: 3, plain: 1, why: "the mob help-spell branch; " + whyMob},
	"internal/actions/combat_attack.go|FindAttackTarget":                {viewer: 1},
	"internal/actions/combat_fire.go|ExecuteFire":                       {viewer: 2},
	"internal/actions/melee_target.go|StageMeleeTarget":                 {viewer: 1, plain: 1, why: whySelf},
	"internal/actions/target_resolution.go|ResolveTargetActor":          {viewer: 1},
	"internal/mobcommands/aid.go|Aid":                                   {plain: 1, why: whyMob},
	"internal/mobcommands/attack.go|Attack":                             {plain: 1, why: whyMob},
	"internal/mobcommands/befriend.go|Befriend":                         {plain: 1, why: whyMob},
	"internal/mobcommands/consider.go|Consider":                         {plain: 1, why: whyMob},
	"internal/mobcommands/give.go|Give":                                 {plain: 1, why: whyMob},
	"internal/mobcommands/givequest.go|GiveQuest":                       {plain: 1, why: whyMob},
	"internal/mobcommands/look.go|Look":                                 {plain: 1, why: whyMob},
	"internal/mobcommands/plant.go|Plant":                               {plain: 1, why: whyMob},
	"internal/mobcommands/sayto.go|ReplyTo":                             {plain: 1, why: whyMob},
	"internal/mobcommands/sayto.go|SayTo":                               {plain: 1, why: whyMob},
	"internal/mobcommands/sayto.go|SayToOnly":                           {plain: 1, why: whyMob},
	"internal/mobcommands/shadow.go|Shadow":                             {plain: 1, why: whyMob},
	"internal/mobcommands/show.go|Show":                                 {plain: 1, why: whyMob},
	"internal/mobcommands/steal.go|parseMobStealArgs":                   {plain: 1, why: whyMob},
	"internal/parser/adapters.go|mobAdapter":                            {viewer: 1},
	"internal/parser/adapters.go|playerAdapter":                         {viewer: 1},
	"internal/rooms/rooms.go|Room.FindByName":                           {plain: 1, why: whyUnfilter},
	"internal/usercommands/admin.ai.go|AiFlag":                          {plain: 1, why: whyStaff},
	"internal/usercommands/admin.buff.go|Buff":                          {plain: 1, why: whyStaff},
	"internal/usercommands/admin.command.go|Command":                    {plain: 1, why: whyStaff},
	"internal/usercommands/admin.paz.go|Paz":                            {plain: 1, why: whyStaff},
	"internal/usercommands/admin.skillset.go|Skillset":                  {plain: 1, why: whyStaff},
	"internal/usercommands/admin.zap.go|Zap":                            {plain: 1, why: whyStaff},
	"internal/usercommands/ask.go|Ask":                                  {viewer: 1},
	"internal/usercommands/attack.go|Attack":                            {viewer: 1},
	"internal/usercommands/consider.go|Consider":                        {viewer: 1},
	"internal/usercommands/give.go|Give":                                {viewer: 1},
	"internal/usercommands/give.go|giveTargetResolves":                  {viewer: 1},
	"internal/usercommands/look.go|Look":                                {viewer: 1},
	"internal/usercommands/moderation_target.go|resolveModTarget":       {plain: 1, why: whyStaff},
	"internal/usercommands/party.go|cmdPartyInvite":                     {viewer: 1},
	"internal/usercommands/report.go|Report":                            {viewer: 1},
	"internal/usercommands/sell.go|resolveSellItem":                     {viewer: 1},
	"internal/usercommands/shoot.go|resolveShootTarget":                 {viewer: 2},
	"internal/usercommands/show.go|Show":                                {viewer: 1},
	"internal/usercommands/skill.skullduggery.plant.go|parsePlantArgs":  {viewer: 1},
	"internal/usercommands/skill.skullduggery.shadow.go|Shadow":         {viewer: 1, plain: 1, why: whySelf},
	"internal/usercommands/skill.skullduggery.steal.go|parseStealArgs":  {viewer: 1},
	"internal/usercommands/talk.go|Talk":                                {viewer: 1},
	"internal/usercommands/target.go|Target":                            {viewer: 1, plain: 1, why: whySelf},
	"modules/follow/follow.go|FollowModule.followMobCommand":            {plain: 1, why: whyMob},
	"modules/follow/follow.go|FollowModule.followUserCommand":           {viewer: 1},
}

func lookupIsNil(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "nil"
}

// lookupPassesViewer classifies one lookup call. A ResolveTargetActor call
// passes a viewer when its options literal names Viewer, or when the function
// sets `.Viewer =` on an options variable (buy.go does).
func lookupPassesViewer(call *ast.CallExpr, callee string, body *ast.BlockStmt) bool {
	switch callee {
	case "FindByNameSeenBy":
		return len(call.Args) > 0 && !lookupIsNil(call.Args[0])
	case "FindAttackTarget":
		return len(call.Args) == 5 && !lookupIsNil(call.Args[4])
	case "ResolveTargetActor":
		for _, a := range call.Args {
			if cl, ok := a.(*ast.CompositeLit); ok {
				for _, elt := range cl.Elts {
					if kv, ok := elt.(*ast.KeyValueExpr); ok {
						if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "Viewer" {
							return true
						}
					}
				}
			}
		}
		assigns := false
		ast.Inspect(body, func(n ast.Node) bool {
			if as, ok := n.(*ast.AssignStmt); ok {
				for _, lhs := range as.Lhs {
					if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "Viewer" {
						assigns = true
					}
				}
			}
			return true
		})
		return assigns
	}
	return false
}

func lookupFuncName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	switch t := fd.Recv.List[0].Type.(type) {
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name + "." + fd.Name.Name
		}
	case *ast.Ident:
		return t.Name + "." + fd.Name.Name
	}
	return fd.Name.Name
}

// TestEveryCreatureLookupDeclaresItsViewer fails when a function looks a
// creature up by name and is not in lookupRegistry, when its viewer or plain
// call counts differ from the registry, or when a registry entry is stale.
//
// The mistake it exists for is the one this slice makes easy: adding a player
// command that names a creature and forgetting the viewer, so `look <hidden
// creature>` gives the hider away again. If you are here for a new player
// command, pass the player as Viewer (ResolveTargetOptions.Viewer or
// FindByNameSeenBy). If it is a staff tool or a mob caller, register it as
// plain with the reason.
func TestEveryCreatureLookupDeclaresItsViewer(t *testing.T) {
	found := map[string]lookupEntry{}
	for _, root := range []string{"internal", "modules"} {
		fset := token.NewFileSet()
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return nil
			}
			rel := filepath.ToSlash(path)
			for _, decl := range file.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				key := rel + "|" + lookupFuncName(fd)
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					callee := ""
					switch f := call.Fun.(type) {
					case *ast.SelectorExpr:
						callee = f.Sel.Name
					case *ast.Ident:
						callee = f.Name
					}
					switch callee {
					case "FindByName", "FindByNameSeenBy", "ResolveTargetActor", "FindAttackTarget":
					default:
						return true
					}
					e := found[key]
					if lookupPassesViewer(call, callee, fd.Body) {
						e.viewer++
					} else {
						e.plain++
					}
					found[key] = e
					return true
				})
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s (test must run from the repo root): %v", root, err)
		}
	}
	if len(found) == 0 {
		t.Fatal("no lookups found at all: the walk is broken, not the code")
	}

	var problems []string
	for key, got := range found {
		want, ok := lookupRegistry[key]
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("%s: not in lookupRegistry (viewer %d, plain %d)", key, got.viewer, got.plain))
		case got.viewer != want.viewer || got.plain != want.plain:
			problems = append(problems, fmt.Sprintf("%s: viewer %d plain %d, registry says viewer %d plain %d", key, got.viewer, got.plain, want.viewer, want.plain))
		}
	}
	for key, want := range lookupRegistry {
		if _, ok := found[key]; !ok {
			problems = append(problems, key+": in lookupRegistry but no lookup found there (stale)")
		}
		if want.plain > 0 && want.why == "" {
			problems = append(problems, key+": plain lookups need a reason")
		}
	}
	sort.Strings(problems)
	if len(problems) > 0 {
		t.Errorf("%d creature lookup problem(s):\n  %s\n\n"+
			"A player command that names a creature must pass the player as Viewer, "+
			"or a hidden creature can be named. Staff tools and mob callers are "+
			"registered as plain, with the reason.",
			len(problems), strings.Join(problems, "\n  "))
	}
}
```

- [ ] **Step 2: Run the guard to verify it fails**

Run: `go test . -run TestEveryCreatureLookupDeclaresItsViewer`
Expected: FAIL. Every player entry reports `viewer 0 plain N, registry says viewer N plain 0`, and `internal/rooms/rooms.go|Room.FindByName` matches already (Task 9).

- [ ] **Step 3: `FindAttackTarget` takes the viewer**

In `internal/actions/combat_attack.go`, add `"github.com/GoMudEngine/GoMud/internal/mobs"` and `"github.com/GoMudEngine/GoMud/internal/users"` to the imports. Change the signature to:

```go
func FindAttackTarget(rest string, room *rooms.Room, actorUserId int, actorMobInstanceId int, viewer *characters.Character) AttackTarget {
```

In the three wildcard pools, add a perception skip after each self-exclusion. In the `*` pool's mob loop, after `continue // mob can't target itself` and its closing brace:

```go
				if !attackPerceivesMob(viewer, mobInstanceId) {
					continue
				}
```

In the `*` pool's player loop, after `continue // user can't target themselves` and its closing brace:

```go
				if !attackPerceivesPlayer(viewer, userId) {
					continue
				}
```

Do the same in the `*mob` loop (`attackPerceivesMob(viewer, mobInstanceId)`) and the `*user` loop (`attackPerceivesPlayer(viewer, userId)`).

Replace the named branch's `playerId, mobInstanceId := room.FindByName(rest)` with:

```go
		playerId, mobInstanceId := room.FindByNameSeenBy(viewer, rest)
```

Add after `FindAttackTarget`:

```go
// attackPerceivesMob and attackPerceivesPlayer report whether viewer makes out
// a creature in a wildcard pool. A nil viewer (a mob attacker, until slice F)
// perceives everything, as before.
func attackPerceivesMob(viewer *characters.Character, mobInstanceId int) bool {
	if viewer == nil {
		return true
	}
	m := mobs.GetInstance(mobInstanceId)
	return m != nil && viewer.Perceives(&m.Character)
}

func attackPerceivesPlayer(viewer *characters.Character, userId int) bool {
	if viewer == nil {
		return true
	}
	u := users.GetByUserId(userId)
	return u != nil && viewer.Perceives(u.Character)
}
```

Update the callers:
- `internal/usercommands/attack.go:95`: `t := actions.FindAttackTarget(rest, room, user.UserId, 0, user.Character)`
- `internal/mobcommands/attack.go:42`: `t := actions.FindAttackTarget(rest, room, 0, mob.InstanceId, nil)`
- `internal/actions/combat_test.go`: append `, nil` as the last argument of the five `FindAttackTarget(...)` calls at `:639`, `:657`, `:672`, `:693`, `:712`.

- [ ] **Step 4: Cast, fire, melee staging, buy, parser, follow**

`internal/actions/cast_sight.go` (create):

```go
package actions

import "github.com/GoMudEngine/GoMud/internal/characters"

// castViewer is the character whose perception limits a named cast: the
// caster when a player, nil (no limit) for a mob until slice F.
func castViewer(actor Actor) *characters.Character {
	if !actor.IsPlayer() {
		return nil
	}
	return actor.GetCharacter()
}
```

`internal/actions/cast.go`, HarmSingle, replace:

```go
	case spells.HarmSingle:
		if targetName != `` {
			pId, mId := room.FindByName(targetName)
```

with:

```go
	case spells.HarmSingle:
		if targetName != `` {
			pId, mId := room.FindByNameSeenBy(castViewer(actor), targetName)
```

HarmMulti, replace:

```go
		// and HarmSingle both refused.
		if targetName != `` {
			pId, mId := room.FindByName(targetName)
```

with:

```go
		// and HarmSingle both refused.
		if targetName != `` {
			pId, mId := room.FindByNameSeenBy(castViewer(actor), targetName)
```

HelpSingle, player branch, replace:

```go
			if targetName != `` && targetName != actor.GetName() {
				pId, mId := room.FindByName(targetName)
```

with:

```go
			if targetName != `` && targetName != actor.GetName() {
				pId, mId := room.FindByNameSeenBy(actor.GetCharacter(), targetName)
```

The mob HelpSingle branch (`// Mob HelpSingle: named target or self.`) keeps `room.FindByName`.

`internal/actions/combat_fire.go`: replace:

```go
	targetUserId, targetMobInstanceId := targetRoom.FindByName(strings.Join(targetWords, " "))
	if targetUserId == 0 && targetMobInstanceId == 0 && crossRoom {
		// The trailing word may have been part of the target name after all;
		// retry as a same-room shot using the full argument string.
		crossRoom, exitName, targetRoom = false, "", room
		targetUserId, targetMobInstanceId = room.FindByName(strings.Join(args, " "))
	}
```

with:

```go
	// A player cannot shoot a creature they do not perceive; a mob shooter has
	// no viewer until slice F.
	var viewer *characters.Character
	if actor.IsPlayer() {
		viewer = actor.GetCharacter()
	}
	targetUserId, targetMobInstanceId := targetRoom.FindByNameSeenBy(viewer, strings.Join(targetWords, " "))
	if targetUserId == 0 && targetMobInstanceId == 0 && crossRoom {
		// The trailing word may have been part of the target name after all;
		// retry as a same-room shot using the full argument string.
		crossRoom, exitName, targetRoom = false, "", room
		targetUserId, targetMobInstanceId = room.FindByNameSeenBy(viewer, strings.Join(args, " "))
	}
```

`internal/actions/melee_target.go:172`:

```go
	target, err := ResolveTargetActor(room, rest, ResolveTargetOptions{ExcludeUserId: user.UserId, Viewer: user.Character})
```

`internal/actions/buy.go`: replace:

```go
		if buyer.IsPlayer() {
			exclude.ExcludeUserId = buyer.GetUserId()
		}
```

with:

```go
		if buyer.IsPlayer() {
			exclude.ExcludeUserId = buyer.GetUserId()
			exclude.Viewer = buyer.GetCharacter()
		}
```

`internal/parser/adapters.go`: replace `import "github.com/GoMudEngine/GoMud/internal/items"` with:

```go
import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
)
```

In `mobAdapter` replace `_, mobInstanceId := s.Room.FindByName(candidate)` with `_, mobInstanceId := s.Room.FindByNameSeenBy(scopeViewer(s), candidate)`; in `playerAdapter` replace `playerId, _ := s.Room.FindByName(candidate)` with `playerId, _ := s.Room.FindByNameSeenBy(scopeViewer(s), candidate)`. Add:

```go
// scopeViewer is the character doing the looking, or nil when the scope has no
// user: a creature the user does not perceive cannot be matched.
func scopeViewer(s Scope) *characters.Character {
	if s.User == nil {
		return nil
	}
	return s.User.Character
}
```

`modules/follow/follow.go`: in `followUserCommand`, replace:

```go
	userId, mobInstId := 0, 0
	if len(followTargetName) > 0 {
		userId, mobInstId = room.FindByName(followTargetName)
	}

	followCommandTarget := followId{userId: userId, mobInstanceId: mobInstId}
	followCommandSource := followId{userId: user.UserId}
```

with:

```go
	userId, mobInstId := 0, 0
	if len(followTargetName) > 0 {
		userId, mobInstId = room.FindByNameSeenBy(user.Character, followTargetName)
	}

	followCommandTarget := followId{userId: userId, mobInstanceId: mobInstId}
	followCommandSource := followId{userId: user.UserId}
```

- [ ] **Step 5: The player commands**

Each replacement adds only the viewer:

| File | Replace | With |
|---|---|---|
| `usercommands/look.go:81` | `actions.ResolveTargetActor(room, lookAt)` | `actions.ResolveTargetActor(room, lookAt, actions.ResolveTargetOptions{Viewer: user.Character})` |
| `usercommands/show.go:43` | `actions.ResolveTargetActor(room, targetName)` | `actions.ResolveTargetActor(room, targetName, actions.ResolveTargetOptions{Viewer: user.Character})` |
| `usercommands/talk.go:43` | `actions.ResolveTargetActor(room, searchName)` | `actions.ResolveTargetActor(room, searchName, actions.ResolveTargetOptions{Viewer: user.Character})` |
| `usercommands/ask.go:94` | `actions.ResolveTargetActor(room, searchName)` | `actions.ResolveTargetActor(room, searchName, actions.ResolveTargetOptions{Viewer: user.Character})` |
| `usercommands/party.go:182` | `actions.ResolveTargetActor(room, rest)` | `actions.ResolveTargetActor(room, rest, actions.ResolveTargetOptions{Viewer: user.Character})` |
| `usercommands/report.go:60` | `actions.ResolveTargetActor(room, rest)` | `actions.ResolveTargetActor(room, rest, actions.ResolveTargetOptions{Viewer: user.Character})` |
| `usercommands/give.go:73` | `actions.ResolveTargetActor(room, giveWho)` | `actions.ResolveTargetActor(room, giveWho, actions.ResolveTargetOptions{Viewer: user.Character})` |
| `usercommands/give.go:376` | `room.FindByName(who)` | `room.FindByNameSeenBy(user.Character, who)` |
| `usercommands/sell.go:127` | `room.FindByName(parts[0])` | `room.FindByNameSeenBy(user.Character, parts[0])` |
| `usercommands/skill.skullduggery.plant.go:96` | `actions.ResolveTargetActor(room, targetNoun)` | `actions.ResolveTargetActor(room, targetNoun, actions.ResolveTargetOptions{Viewer: user.Character})` |
| `usercommands/skill.skullduggery.steal.go:84` | `actions.ResolveTargetActor(room, targetNoun)` | `actions.ResolveTargetActor(room, targetNoun, actions.ResolveTargetOptions{Viewer: user.Character})` |
| `usercommands/consider.go:29` | `actions.ResolveTargetOptions{ExcludeUserId: user.UserId})` | `actions.ResolveTargetOptions{ExcludeUserId: user.UserId, Viewer: user.Character})` |

In `usercommands/target.go:62-64` and `usercommands/skill.skullduggery.shadow.go:57-59`, add `Viewer: user.Character,` on the line after `ExcludeUserId: user.UserId,` inside the options literal. Their self checks (`room.FindByName(rest)`, `room.FindByName(strings.ToLower(rest))`) stay as they are.

`usercommands/shoot.go`: change the helper's signature to

```go
func resolveShootTarget(room *rooms.Room, rest string, viewer *characters.Character) (userId, mobInstanceId int, targetRoom *rooms.Room, crossRoom bool) {
```

replace `userId, mobInstanceId = targetRoom.FindByName(strings.Join(targetWords, " "))` with `userId, mobInstanceId = targetRoom.FindByNameSeenBy(viewer, strings.Join(targetWords, " "))`, replace `userId, mobInstanceId = room.FindByName(strings.Join(args, " "))` with `userId, mobInstanceId = room.FindByNameSeenBy(viewer, strings.Join(args, " "))`, and update its only caller at `:52` to `resolveShootTarget(room, rest, user.Character)`.

- [ ] **Step 6: Resolver tests for the attack pools**

Append to `internal/actions/target_viewer_test.go`:

```go
func TestFindAttackTarget_ViewerCannotNameOrDrawAHiddenCreature(t *testing.T) {
	room, viewer := viewerTestRoom(t)

	named := FindAttackTarget("kesh", room, 7701, 0, viewer.Character)
	require.False(t, named.Found, "a hidden player must not be attackable by name")

	pool := FindAttackTarget("*user", room, 7701, 0, viewer.Character)
	require.False(t, pool.Found, "the only other player is hidden, so *user draws nobody")
}

func TestFindAttackTarget_NoViewerIsUnchanged(t *testing.T) {
	room, _ := viewerTestRoom(t)
	named := FindAttackTarget("kesh", room, 7701, 0, nil)
	require.True(t, named.Found)
	require.Equal(t, 7702, named.UserId)
}
```

- [ ] **Step 7: Run everything touched**

```bash
gofmt -w internal/actions/ internal/parser/adapters.go internal/usercommands/ internal/mobcommands/attack.go modules/follow/follow.go
go build ./...
go test ./internal/actions/ ./internal/parser/ ./internal/usercommands/ ./internal/mobcommands/ ./modules/follow/
go test . -run TestEveryCreatureLookupDeclaresItsViewer
```

Expected: all PASS.

- [ ] **Step 8: Sabotage probe**

Remove `, actions.ResolveTargetOptions{Viewer: user.Character}` from `usercommands/look.go:81`. Run `go test . -run TestEveryCreatureLookupDeclaresItsViewer`. Expected: FAIL with `internal/usercommands/look.go|Look: viewer 0 plain 1, registry says viewer 1 plain 0`. Undo with Edit.

- [ ] **Step 9: Gates and commit**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/... ./modules/... && go test .
git add lookup_viewer_guard_test.go internal/actions/cast_sight.go internal/actions/cast.go internal/actions/combat_attack.go internal/actions/combat_test.go internal/actions/combat_fire.go internal/actions/melee_target.go internal/actions/buy.go internal/actions/target_viewer_test.go internal/parser/adapters.go internal/usercommands/attack.go internal/usercommands/ask.go internal/usercommands/consider.go internal/usercommands/give.go internal/usercommands/look.go internal/usercommands/party.go internal/usercommands/report.go internal/usercommands/sell.go internal/usercommands/shoot.go internal/usercommands/show.go internal/usercommands/talk.go internal/usercommands/target.go internal/usercommands/skill.skullduggery.plant.go internal/usercommands/skill.skullduggery.shadow.go internal/usercommands/skill.skullduggery.steal.go internal/mobcommands/attack.go modules/follow/follow.go
git commit -m "fix(targeting): a hidden creature cannot be named by a player who does not perceive it" -m "Every player command that names a creature passes the player as viewer: attack and its wildcard pools, the melee special moves, cast, shoot, target, look, consider, give, show, talk, ask, party, report, buy, sell, follow, plant, steal and shadow. Staff tools and mob callers stay unfiltered. A root registry guard fails on a lookup that does not declare which it is." -m "Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>" -m "Claude-Session: https://claude.ai/code/session_01J9ARXi13pxi92SKumuvpum"
```

---
### Task 11: Targeted casts need sight

**Files:**
- Modify: `internal/actions/cast_sight.go` (extend)
- Modify: `internal/actions/cast.go` (admission after the already-casting check; `resolvePlayerAggroTarget` gains `leaderFallback`)
- Test: `internal/actions/cast_sight_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/actions/cast_sight_test.go`:

```go
package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const castSightInfraredBuffId = 7801

// castSightActor is a PLAYER caster: stubActor with a user id, a name and a
// record of what it was told.
type castSightActor struct {
	*stubActor
	userId int
	name   string
	sent   []string
}

func (a *castSightActor) IsPlayer() bool  { return true }
func (a *castSightActor) GetUserId() int  { return a.userId }
func (a *castSightActor) GetName() string { return a.name }
func (a *castSightActor) SendText(_ messaging.Category, msg string) {
	a.sent = append(a.sent, msg)
}

func (a *castSightActor) told(substr string) bool {
	for _, s := range a.sent {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// castSightScene stands Caster (7811), Witness (7812) and Other (7813) in one
// room of the given biome, and seeds a help and a harm spell.
func castSightScene(t *testing.T, biome string) (*castSightActor, *rooms.Room) {
	t.Helper()
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave":    {BiomeId: "cave", DarkArea: true},
		"city":    {BiomeId: "city", LitArea: true},
		"default": {BiomeId: "default", LitArea: true},
	}))
	t.Cleanup(buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		castSightInfraredBuffId: {BuffId: castSightInfraredBuffId, Name: "Test Infrared", Flags: []buffs.Flag{buffs.InfraredVision}},
	}))
	t.Cleanup(spells.SeedSpellsForTest(map[string]*spells.SpellData{
		"sight-heal": {SpellId: "sight-heal", Name: "Sight Heal", Type: spells.HelpSingle, BaseFolds: 2, Cost: 5},
		"sight-bolt": {SpellId: "sight-bolt", Name: "Sight Bolt", Type: spells.HarmSingle, BaseFolds: 2, Cost: 5},
	}))
	caster := users.NewTestUser(7811, "caster", "Caster", 97811)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		7811: caster,
		7812: users.NewTestUser(7812, "witness", "Witness", 97812),
		7813: users.NewTestUser(7813, "other", "Other", 97813),
	}))
	room := &rooms.Room{RoomId: 7810, Biome: biome}
	room.AddPlayer(7811)
	room.AddPlayer(7812)
	room.AddPlayer(7813)
	actor := &castSightActor{stubActor: newStubActor(caster.Character, room), userId: 7811, name: "Caster"}
	return actor, room
}

func giveCasterInfrared(t *testing.T, a *castSightActor) {
	t.Helper()
	require.NoError(t, a.GetCharacter().AddBuff(castSightInfraredBuffId, true))
}

func requireRefused(t *testing.T, a *castSightActor, r CastResult, line string) {
	t.Helper()
	assert.False(t, r.Initiated)
	assert.True(t, r.NoTarget)
	assert.True(t, r.RefusalExplained, "the refusal is narrated, so the generic line must not follow")
	assert.True(t, a.told(line), "caster was told %q, want a line containing %q", a.sent, line)
	assert.True(t, a.GetCharacter().CooldownReady("special-move"), "a refused cast spends nothing")
}

func TestCastSight_ClearSightNamesAndShapes(t *testing.T) {
	a, _ := castSightScene(t, "city")
	r := InitiateCast(a, "sight-heal", "witness")
	require.True(t, r.Initiated)
	assert.Equal(t, []int{7812}, r.TargetUserIds)

	a2, _ := castSightScene(t, "city")
	r = InitiateCast(a2, "sight-heal", "2.shape")
	require.True(t, r.Initiated)
	assert.Equal(t, []int{7813}, r.TargetUserIds, "figures are the other players in room order")
}

func TestCastSight_ShapesOnlyRefusesANameWithAHint(t *testing.T) {
	a, _ := castSightScene(t, "cave")
	giveCasterInfrared(t, a)
	requireRefused(t, a, InitiateCast(a, "sight-heal", "witness"), "You can only make out shapes here.")
	assert.True(t, a.told("cast sight-heal shape"))
}

func TestCastSight_ShapesOnlyAimsAtAShape(t *testing.T) {
	for _, name := range []string{"shape", "2.shape", "shape#2"} {
		a, _ := castSightScene(t, "cave")
		giveCasterInfrared(t, a)
		r := InitiateCast(a, "sight-heal", name)
		require.True(t, r.Initiated, "%q should initiate", name)
		want := 7812
		if name != "shape" {
			want = 7813
		}
		assert.Equal(t, []int{want}, r.TargetUserIds, "%q", name)
	}
}

func TestCastSight_NoSightRefusesEveryTargetedCast(t *testing.T) {
	a, _ := castSightScene(t, "cave")
	requireRefused(t, a, InitiateCast(a, "sight-heal", "witness"), "You don't see them here.")

	a, _ = castSightScene(t, "cave")
	requireRefused(t, a, InitiateCast(a, "sight-heal", "shape"), "You can't see anything to aim at.")

	a, _ = castSightScene(t, "cave")
	requireRefused(t, a, InitiateCast(a, "sight-bolt", ""), "You can't see anything to aim at.")
}

func TestCastSight_SelfCastNeedsNoSight(t *testing.T) {
	a, _ := castSightScene(t, "cave")
	r := InitiateCast(a, "sight-heal", "")
	require.True(t, r.Initiated)
	assert.Equal(t, []int{7811}, r.TargetUserIds)
}

func TestCastSight_AHiddenCreatureIsNotAFigure(t *testing.T) {
	a, _ := castSightScene(t, "city")
	viewerTestHide(t, users.GetByUserId(7812).Character)
	r := InitiateCast(a, "sight-heal", "shape")
	require.True(t, r.Initiated)
	assert.Equal(t, []int{7813}, r.TargetUserIds, "the hidden witness is skipped")
}

func TestCastSight_MobCasterIsUnaffected(t *testing.T) {
	a, room := castSightScene(t, "cave")
	mobCaster := newStubActor(a.GetCharacter(), room) // IsPlayer false
	r := InitiateCast(mobCaster, "sight-heal", "witness")
	require.True(t, r.Initiated, "mobs perceiving darkness is slice F")
}
```

`viewerTestHide` is the helper from `target_viewer_test.go` (Task 9).

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/actions/ -run TestCastSight`
Expected: FAIL. The dark cases initiate instead of refusing; `2.shape` resolves nobody.

- [ ] **Step 3: Write the admission**

Replace `internal/actions/cast_sight.go` with:

```go
package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// shapeWord is what a player who makes out shapes aims at: `shape`, `2.shape`,
// `shape#2`. No dogmud mob, item, noun or alias uses the word.
const shapeWord = "shape"

// castAim is what a player caster's sight lets a targeted cast aim at.
type castAim struct {
	// targetName is the name to resolve. A shape is rewritten to "@<userId>"
	// or "#<mobInstanceId>", forms FindByName already resolves.
	targetName string
	// ownFoeOnly is set when the caster makes out shapes only: a no-name
	// harmful cast may aim at the caster's own foe, not the party leader's.
	ownFoeOnly bool
}

// castViewer is the character whose perception limits a named cast: the
// caster when a player, nil (no limit) for a mob until slice F.
func castViewer(actor Actor) *characters.Character {
	if !actor.IsPlayer() {
		return nil
	}
	return actor.GetCharacter()
}

// admitCastAim applies follow-up slice A's sight rules to a player's targeted
// cast (harmsingle, harmmulti, helpsingle). It returns refused=true after
// telling the caster why; nothing has been spent at that point.
//
//	clear sight:  names, your foe then your party leader's foe, shapes
//	shapes only:  your own foe, shapes; a typed name gets a hint
//	no sight:     every targeted cast refused
//
// A self-cast (a help spell with no name, or the caster's own name) needs no
// sight. Mob casters, area, help-multi and neutral casts are not affected.
func admitCastAim(actor Actor, spellInfo *spells.SpellData, targetName string) (castAim, bool) {
	aim := castAim{targetName: targetName}
	switch spellInfo.Type {
	case spells.HarmSingle, spells.HarmMulti, spells.HelpSingle:
	default:
		return aim, false
	}
	room := actor.GetRoom()
	if !actor.IsPlayer() || room == nil {
		return aim, false
	}
	if spellInfo.Type == spells.HelpSingle && (targetName == `` || targetName == actor.GetName()) {
		return aim, false
	}

	char := actor.GetCharacter()
	shape := castShapeIndex(targetName)

	switch messaging.ParticipantSight(char, room) {
	case messaging.SightNone:
		if targetName == `` || shape > 0 {
			actor.SendText(messaging.CategorySystem, `You can't see anything to aim at.`)
		} else {
			actor.SendText(messaging.CategorySystem, `You don't see them here.`)
		}
		return aim, true
	case messaging.SightShapes:
		if targetName == `` {
			aim.ownFoeOnly = true
			return aim, false
		}
		if shape == 0 {
			actor.SendText(messaging.CategorySystem, fmt.Sprintf(
				`You can only make out shapes here. Try <ansi fg="command">cast %s shape</ansi> or <ansi fg="command">cast %s 2.shape</ansi>.`,
				spellInfo.SpellId, spellInfo.SpellId))
			return aim, true
		}
	}

	if shape > 0 {
		figures := castFigures(char, actor.GetUserId(), room)
		if shape > len(figures) {
			actor.SendText(messaging.CategorySystem, `You don't see them here.`)
			return aim, true
		}
		aim.targetName = figures[shape-1]
	}
	return aim, false
}

// castShapeIndex is N for `shape`, `N.shape` or `shape#N`, and 0 otherwise.
func castShapeIndex(targetName string) int {
	if targetName == `` {
		return 0
	}
	word, n := util.GetMatchNumber(targetName)
	if word != shapeWord || n < 1 {
		return 0
	}
	return n
}

// castFigures lists what the caster makes out as shapes: the other players
// they perceive, in room order, then the mobs they perceive, in room order.
// Each is a name FindByName resolves: "@<userId>" or "#<mobInstanceId>".
func castFigures(viewer *characters.Character, selfUserId int, room *rooms.Room) []string {
	figures := []string{}
	for _, uid := range room.GetPlayers() {
		if uid == selfUserId {
			continue
		}
		if u := users.GetByUserId(uid); u != nil && viewer.Perceives(u.Character) {
			figures = append(figures, fmt.Sprintf(`@%d`, uid))
		}
	}
	for _, mid := range room.GetMobs() {
		if m := mobs.GetInstance(mid); m != nil && viewer.Perceives(&m.Character) {
			figures = append(figures, fmt.Sprintf(`#%d`, mid))
		}
	}
	return figures
}
```

- [ ] **Step 4: Wire it into `InitiateCast`**

In `internal/actions/cast.go`, directly after:

```go
	// 2. Already casting?
	if char.Activity != nil && char.Activity.IsCasting() {
		return CastResult{SpellInfo: spellInfo, AlreadyCasting: true}
	}
```

add:

```go

	// 2b. Sight (follow-up slice A). A player must see what a targeted cast
	// aims at. The refusal is narrated and nothing has been spent.
	aim, refused := admitCastAim(actor, spellInfo, targetName)
	if refused {
		return CastResult{SpellInfo: spellInfo, NoTarget: true, RefusalExplained: true}
	}
	targetName = aim.targetName
```

Change both calls `pId, mId := resolvePlayerAggroTarget(actor, room)` (HarmSingle and HarmMulti branches) to:

```go
			pId, mId := resolvePlayerAggroTarget(actor, room, !aim.ownFoeOnly)
```

Change `resolvePlayerAggroTarget`:

```go
// resolvePlayerAggroTarget returns the player's current aggro target, falling
// back to the party leader's aggro target if the player has no aggro of their
// own and leaderFallback is set. A caster who makes out shapes only aims at
// their own foe (follow-up slice A), so the fallback is off for them.
// Returns (userId, mobInstanceId) with at most one non-zero.
func resolvePlayerAggroTarget(actor Actor, room *rooms.Room, leaderFallback bool) (int, int) {
	char := actor.GetCharacter()
	if tgt := char.CurrentCombatTarget(); !tgt.IsZero() {
		if tgt.MobInstanceId > 0 {
			return 0, tgt.MobInstanceId
		}
		if tgt.UserId > 0 {
			return tgt.UserId, 0
		}
	}
	if !leaderFallback {
		return 0, 0
	}
```

(the rest of the function, the party leader fallback, is unchanged).

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go build ./... && go test ./internal/actions/ ./internal/usercommands/ ./internal/mobcommands/`
Expected: PASS, including the existing `cast_test.go`, `cast_charm_no_player_target_test.go`, `cast_conjure_cooldown_test.go` and `cast_harm_authorization_test.go`.

- [ ] **Step 6: Sabotage probe**

In `admitCastAim`, change `case messaging.SightNone:` to `case messaging.SightDecision(99):`. Run `go test ./internal/actions/ -run TestCastSight_NoSight`. Expected: FAIL, the casts initiate in the dark. Undo with Edit.

- [ ] **Step 7: Gates and commit**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/actions/ && go test .
git add internal/actions/cast_sight.go internal/actions/cast_sight_test.go internal/actions/cast.go
git commit -m "fix(spells): a targeted cast needs sight" -m "A player cannot cast at what they cannot see. In the dark only self and area casts work; a caster who makes out shapes aims at their own foe or at shape, 2.shape, shape#2, and a typed name gets a hint. Refusals are narrated and spend nothing. Mob casters are unchanged until slice F." -m "Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>" -m "Claude-Session: https://claude.ai/code/session_01J9ARXi13pxi92SKumuvpum"
```

---

### Task 12: A player's search ends a found hider's hiding

**Files:**
- Modify: `internal/actions/search.go` (add `revealSpotted`, `endHidingOnSpot`; call after the Tier 2 hidden-mob block; imports)
- Test: `internal/actions/search_reveal_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/actions/search_reveal_test.go`:

```go
package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRevealSpotted_PlayerSearcherEndsAHiddenMobsHiding(t *testing.T) {
	room := hiddenMobRoom(t, 7901, 7902, 0)
	seeker := newSearchFakeActor("Seeker", room, true, 0)

	revealSpotted(seeker, SearchResult{HiddenMobsFound: []int{7902}}, room)
	require.False(t, mobs.GetInstance(7902).Character.IsHidden(),
		"a player's find drags the mob out of hiding for everyone")
}

func TestRevealSpotted_MobSearcherEndsNothing(t *testing.T) {
	room := hiddenMobRoom(t, 7903, 7904, 0)
	scout := newSearchFakeActor("Scout", room, false, 0)

	revealSpotted(scout, SearchResult{HiddenMobsFound: []int{7904}}, room)
	require.True(t, mobs.GetInstance(7904).Character.IsHidden(),
		"mobs perceiving hidden creatures is slice F")
}

func revealPlayerScene(t *testing.T, biome string) (*rooms.Room, *users.UserRecord) {
	t.Helper()
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave":    {BiomeId: "cave", DarkArea: true},
		"city":    {BiomeId: "city", LitArea: true},
		"default": {BiomeId: "default", LitArea: true},
	}))
	hider := users.NewTestUser(7911, "kesh", "Kesh", 97911)
	viewerTestHide(t, hider.Character)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{7911: hider}))
	room := &rooms.Room{RoomId: 7910, Biome: biome}
	room.AddPlayer(7911)
	events.DrainQueuedMessagesForTest(7911)
	return room, hider
}

func revealTold(lines []string, want string) bool {
	for _, l := range lines {
		if strings.Contains(l, want) {
			return true
		}
	}
	return false
}

func TestRevealSpotted_AHiddenPlayerIsRevealedAndTold(t *testing.T) {
	room, hider := revealPlayerScene(t, "city")
	revealSpotted(newSearchFakeActor("Seeker", room, true, 0), SearchResult{HiddenPlayersFound: []int{7911}}, room)

	require.False(t, hider.Character.IsHidden())
	assert.True(t, revealTold(events.DrainQueuedMessagesForTest(7911), "searches the room and spots you!"))
}

func TestRevealSpotted_AHiderWhoCannotSeeIsNotToldTheName(t *testing.T) {
	room, hider := revealPlayerScene(t, "cave")
	revealSpotted(newSearchFakeActor("Seeker", room, true, 0), SearchResult{HiddenPlayersFound: []int{7911}}, room)

	require.False(t, hider.Character.IsHidden())
	lines := events.DrainQueuedMessagesForTest(7911)
	assert.True(t, revealTold(lines, "Someone searches the room and spots you!"))
	assert.False(t, revealTold(lines, "Seeker"))
}
```

`hiddenMobRoom` and `newSearchFakeActor` are the existing helpers in `search_stealth_test.go` and `search_test.go`; `viewerTestHide` is from Task 9.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/actions/ -run TestRevealSpotted`
Expected: FAIL to compile, `undefined: revealSpotted`.

- [ ] **Step 3: Write the reveal**

In `internal/actions/search.go`, add `"github.com/GoMudEngine/GoMud/internal/state"` and `"github.com/GoMudEngine/GoMud/internal/state/awareness"` to the imports, and add after `spotsHider`:

```go
// revealSpotted ends the hiding of everything a PLAYER's search found, for
// everyone in the room. Owner ruling 10 (follow-up slice A): the same thing
// walking in and spotting a hider already does (usercommands/go.go). Without
// it a hidden creature that never fights, such as Torvan Cresk in quest 14,
// could be found by search but still not named. A mob's search ends nothing;
// mobs perceiving hidden creatures is slice F.
func revealSpotted(actor Actor, found SearchResult, room *rooms.Room) {
	if !actor.IsPlayer() {
		return
	}
	searcher := actor.GetName()
	for _, uid := range found.HiddenPlayersFound {
		if u := users.GetByUserId(uid); u != nil {
			endHidingOnSpot(searcher, u.Character, u, room)
		}
	}
	for _, mid := range found.HiddenMobsFound {
		if m := mobs.GetInstance(mid); m != nil {
			endHidingOnSpot(searcher, &m.Character, nil, room)
		}
	}
}

// endHidingOnSpot drives the hider's Awareness machine out of Hidden, exactly
// as go.go does for a spotted occupant; the Awareness cascade then cancels
// buff 9. A player hider is told, by name only if they can see the searcher.
func endHidingOnSpot(searcher string, hider *characters.Character, hiderUser *users.UserRecord, room *rooms.Room) {
	if hider.Awareness != nil {
		_ = hider.Awareness.TransitionToRevealing(
			state.TransitionReason{Trigger: awareness.TriggerObserverSearch})
	}
	if hiderUser == nil {
		return
	}
	hider.SetMiscData(`sneaking`, nil)
	if messaging.CanSeeClearly(hider, room) {
		hiderUser.SendText(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="username">%s</ansi> searches the room and spots you!`, searcher))
	} else {
		hiderUser.SendText(messaging.CategorySystem, `Someone searches the room and spots you!`)
	}
}
```

In `Search`, directly after the Tier 2 hidden-mob block (the `if actor.IsPlayer() && len(hiddenMobNames) > 0 { ... }` that renders `descriptions/who`) and before `// ── Tier 3`, add:

```go
	revealSpotted(actor, result, room)
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go build ./... && go test ./internal/actions/`
Expected: PASS, including the existing search tests (their actors are mobs or find nothing hidden).

- [ ] **Step 5: Sabotage probe**

In `revealSpotted`, change `if !actor.IsPlayer() {` to `if actor.IsPlayer() {`. Run `go test ./internal/actions/ -run TestRevealSpotted`. Expected: FAIL on both mob tests. Undo with Edit.

- [ ] **Step 6: Gates and commit**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/actions/ && go test .
git add internal/actions/search.go internal/actions/search_reveal_test.go
git commit -m "feat(search): a player's find ends a hider's hiding for everyone" -m "Owner ruling 10. Search already spotted hiders but only told the searcher; now the found creature leaves hiding the way a spotted occupant does on room entry, so a hidden creature that never fights can be found and then named." -m "Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>" -m "Claude-Session: https://claude.ai/code/session_01J9ARXi13pxi92SKumuvpum"
```

---
### Task 13: Cast help names the targets

**Files:**
- Modify: `_datafiles/world/dogmud/templates/help/cast.template`

- [ ] **Step 1: Add the section**

Insert directly before the line `<ansi fg="yellow">━━━ Combat and Concentration ━━━</ansi>`:

```
<ansi fg="yellow">━━━ Targets ━━━</ansi>

  <ansi fg="command">cast heal Kesh</ansi>
  Aim at someone by name.

  <ansi fg="command">cast heal</ansi>
  With no name, a harmful spell aims at your foe and a helpful one at you.

You must be able to see what you aim at. In the dark you can cast only
on yourself or on the whole room. If you can make out shapes but not
faces, aim at your foe, or at a shape: <ansi fg="command">cast heal shape</ansi>,
<ansi fg="command">cast heal 2.shape</ansi>. A creature hiding from you
cannot be named at all until you find it.

```

- [ ] **Step 2: Check and commit**

```bash
grep -c "Targets" _datafiles/world/dogmud/templates/help/cast.template
git add _datafiles/world/dogmud/templates/help/cast.template
git commit -m "docs(help): cast help explains aiming in the dark" -m "Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>" -m "Claude-Session: https://claude.ai/code/session_01J9ARXi13pxi92SKumuvpum"
```

Expected: `1`. The in-game check is Lane 1's first step.

---

### Task 14: Package docs and patch notes

**Files:**
- Modify: `internal/messaging/context.md`, `internal/rooms/context.md`, `internal/actions/context.md`, `internal/characters/context.md`, `internal/hooks/context.md`
- Modify: `docs/PATCH_NOTES.md`

- [ ] **Step 1: `internal/messaging/context.md`**

Replace the `Broadcaster` and `Audience` bullets with:

```markdown
- `Broadcaster`: interface satisfied by `*rooms.Room`:
  `SendTextVisualHidingNames(cat, txt, names, excludeUserIds ...int)` and
  `ParticipantSight(userId int) SightDecision`.
- `Audience`: who is present for one event: `Actor`/`ActorId`/`ActorName`,
  `Actee`/`ActeeId`/`ActeeName`, `Room`. Ids are passed rather than derived
  because `users.UserRecord.UserId` is a FIELD while `actions.Actor` exposes
  `GetUserId()`. The names are exactly as the lines print them; the root guard
  requires both on every literal.
- `NoName`: the empty string, for a side of an Audience with nobody on it.
```

Add to the Functions list, after `CanSeeShapes`:

```markdown
- `ParticipantSight(observer *characters.Character, room RoomVisibility) SightDecision`
  is what a party to an event makes out of the other party. Darkness and
  blindness decide it; sleep does not (a sleeper struck in a lit room is told
  what hit them). Infrared gives `SightShapes`.
- `HideNames(text string, names []string, d SightDecision) string`: replaces
  each name with "a figure" (shapes) or "something" (none): exact whole-word
  match, longest first, a whole identity tag replaced with its name,
  capitalized at a sentence start.
```

Replace the `SendTrio` bullet with:

```markdown
- `SendTrio(t Trio, aud Audience)`: delivers one narrated event to
  everyone entitled to it. A line goes out only if it has BOTH text
  and a recipient. The room broadcast ALWAYS excludes the actor and the
  actee. Each role is rendered for its reader: the actor's line hides
  `ActeeName` and the actee's hides `ActorName` by that reader's
  `ParticipantSight`; the observer line hides both for shapes-only observers.
```

- [ ] **Step 2: `internal/rooms/context.md`**

Append to the "Room text by channel" bullet (after `Blinded and sleeping observers still get nothing from it.`):

```markdown
  `SendTextVisualHidingNames` is `SendTextVisual` for a line that names an
  event's parties: a shapes-only observer reads each name as "a figure". It is
  the observer half of `messaging.SendTrio`; `ParticipantSight(userId)` is the
  other half.
- **Naming a creature**: `FindByNameSeenBy(viewer, name, flags...)` skips
  every creature `viewer` does not perceive (`characters.Character.Perceives`)
  before matching, so a hidden creature cannot be named and does not count
  toward `2.name`. `FindByName` is the unfiltered form for staff tools and mob
  callers. The "Also here" listing (`roomdetails.go`) reads the same
  `Perceives` rule, with no pet requirement.
```

- [ ] **Step 3: `internal/actions/context.md`**

Insert directly before `## Skill Actions`:

```markdown
## Naming and aiming in the dark (follow-up slice A)

- **`ResolveTargetOptions.Viewer`**: every player command that names a creature
  passes the player; a creature they do not perceive cannot be named. Staff
  tools and mob callers pass none. `lookup_viewer_guard_test.go` in the repo
  root registers every lookup as viewer or plain, with the reason.
- **`FindAttackTarget(rest, room, userId, mobId, viewer)`**: the named branch
  and the `*`, `*mob`, `*user` pools skip what `viewer` does not perceive.
- **`InitiateCast`** runs `admitCastAim` (`cast_sight.go`) for a player's
  harmsingle, harmmulti and helpsingle casts: clear sight allows names, foes and
  shapes; shapes only allows the caster's own foe and `shape` / `N.shape` /
  `shape#N` (figures are perceived players then mobs, in room order); no sight
  refuses. Refusals are narrated, set `RefusalExplained`, and spend nothing.
- **`SendCounterTrio(room, res, countered, counteredUserId)`**: the one counter
  dispatch, used by `DispatchCounterMessages` and `hooks.fireSpellCounterTier`.
  It goes through `messaging.SendTrio`, so a counter in the dark names nobody.

```

In the Search section, after `**Messaging:** UserActor receives discovery feedback per tier. MobActor silent.`, add:

```markdown

**Ending hiding:** a player's find ends each found creature's hiding for
everyone (`revealSpotted`), the way a spotted occupant's hiding ends on room
entry in `usercommands/go.go`. A player hider is told, by name only if they can
see the searcher. A mob's find ends nothing (slice F).
```

- [ ] **Step 4: `internal/characters/context.md`**

Under `### Character States and Modifiers`, add a bullet after the `**Buffs integration**` line:

```markdown
- **Perception of hidden creatures** (`character.go`): `Perceives(other)` is
  true for yourself, for anyone not hidden, or when you have see-hidden from any
  source (buff or mutation). No pet is involved. The room listing and
  `rooms.Room.FindByNameSeenBy` both read it.
```

- [ ] **Step 5: `internal/hooks/context.md`**

Insert directly after the `## Combat System Integration` heading line:

```markdown

**Names in the dark.** Crit effect lines (`sendCritEffectTrio`), counter lines
(`actions.SendCounterTrio`) and spell lines between two parties
(`spellAudience` in `spell_audience.go`, used by `applyPlayerEffect`,
`sendSpellChannelDefenceMessages`, `resolveMobSpellAgainstPlayer` and
`resolvePurgeAffliction`) all go through `messaging.SendTrio`, so a reader who
cannot see the other party reads "something", or "a figure" with infrared.
```

- [ ] **Step 6: `docs/PATCH_NOTES.md`**

Insert after `# DOGMud Patch Notes` and its blank line:

```markdown
## 2026-09-11: What you cannot see, you cannot name

In the dark, spells and counterattacks used to tell you exactly who was
involved. If someone healed you, struck you with a spell, or turned your blow
into a sweep while you stood blind, you read their name anyway, and so did
everyone else in the room who could not see either. Now a player who cannot
see reads "something", and one whose eyes only catch warmth reads "a figure".

The same goes for aiming. You can no longer cast a spell at someone you cannot
see. In the dark you can still cast on yourself or on the whole room, and if you
can make out shapes, you can aim at whoever you are fighting or at a shape:
`cast heal shape`, `cast heal 2.shape`. A refused cast costs you nothing.

Hiding means something now. A creature slipping through the shadows is not
listed, and you cannot look at it, talk to it, trade with it or attack it by
name until you find it. Veil Sight, the Cat's Eye Draught and a handful of
mutations were always meant to reveal hidden creatures, and they quietly never
did. Now they do. And a successful search drags whatever you found out of
hiding for everyone.

```

- [ ] **Step 7: Check and commit**

Check that the lines this task added carry no em or en dash. Run it on its own line: `grep -c` exits 1 on zero matches, and zero is the pass.

```bash
git diff -U0 -- docs/PATCH_NOTES.md internal/messaging/context.md internal/rooms/context.md internal/actions/context.md internal/characters/context.md internal/hooks/context.md | grep "^+" | grep -c "—\|–"
```

Expected: `0`. Then:

```bash
python tools/context_md_audit.py
git add docs/PATCH_NOTES.md internal/messaging/context.md internal/rooms/context.md internal/actions/context.md internal/characters/context.md internal/hooks/context.md
git commit -m "docs: names in the dark, package docs and patch notes" -m "Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>" -m "Claude-Session: https://claude.ai/code/session_01J9ARXi13pxi92SKumuvpum"
```

The audit must report no phantom symbols in the five files.

---

### Task 15: Playtest fixtures

Tools and playtest files are not indexed in `docs/README.md`, by convention.

**Files:**
- Create: `tools/playtest/profiles/slice-a-infrared.yaml`
- Create: `tools/playtest/scenarios/slice-a-dark-cave.yaml`
- Create: `tools/playtest/goals/scenarios/slice-a-dark-cave/{caster,witness,infrared}.yaml`
- Create: `tools/playtest/scenarios/slice-a-hidden.yaml`
- Create: `tools/playtest/goals/scenarios/slice-a-hidden/{sneaker,seer,quester}.yaml`

- [ ] **Step 1: The infrared profile**

`tools/playtest/profiles/slice-a-infrared.yaml`:

```yaml
# Follow-up slice A playtest: the INFRARED tester.
#
# Makes out heat shapes in the dark but no faces: buff 85 (InfraredVision) is
# carried as a permanent active buff. Nothing in dogmud content grants infrared,
# so this profile is the only way to stand a shapes-only player in a room. Knows
# Heal, so it can aim at a shape without needing PvP. No night vision, no
# see-hidden.
role: user
username: template-slice-a-infrared
character:
  name: Vessa Thorn
  description: >
    A watcher whose eyes catch warmth where there is no light.
  roomid: 462
  zone: Thornwall City
  speciesid: 1
  stats:
    strength:
      base: 110
    dexterity:
      base: 115
    perception:
      base: 130
    vitality:
      base: 135
    willpower:
      base: 130
    charisma:
      base: 110
  health: 650
  stamina: 500
  conviction: 600
  gold: 300
  skills:
    weapon-combat: 15
    spellcasting: 22
    search: 12
  spellbook:
    heal: 2
  buffs:
    list:
      - buffid: 85
        permabuff: true
  equipment:
    weapon:
      itemid: 10006
    body:
      itemid: 20008
    feet:
      itemid: 20003
```

- [ ] **Step 2: Lane 1 scenario and goals**

`tools/playtest/scenarios/slice-a-dark-cave.yaml`:

```yaml
# Follow-up slice A, lane 1: aiming and names in an UNLIT cave.
#
# Room 3101, Cave Mouth, Ironwind Steppe: cave biome, unlit, the room the M2 and
# 5a darkness lanes used. Do NOT cast a light spell here: it lights the cave and
# voids every later step.
name: slice-a-dark-cave
mode: party
summary: >-
  A caster, a sightless witness and an infrared tester check that casts need
  sight and that nobody is told a name they cannot see.
on_actor_stop: continue
budgets:
  wall_clock: 25m
requires:
  max_connections: 20
roster:
  - id: caster
    personality: bug-finder
    goals: goals/scenarios/slice-a-dark-cave/caster.yaml
  - id: witness
    personality: bug-finder
    goals: goals/scenarios/slice-a-dark-cave/witness.yaml
  - id: infrared
    personality: bug-finder
    goals: goals/scenarios/slice-a-dark-cave/infrared.yaml

group_goals:
  - id: confirm-dark
    do: All three type `look` first.
    verify: Nobody can read a normal room description. If anyone can, the lane is void.
  - id: blind-cast-refused
    do: The caster types `cast heal Ordel` and `cast mind-spike Ordel`.
    verify: Both are refused with "You don't see them here." and the caster's conviction does not drop.
  - id: shape-hint
    do: The infrared tester types `cast heal Ordel`, then `cast heal shape` and `cast heal 2.shape`.
    verify: >-
      The named cast is refused with a hint naming `cast heal shape`. The shape
      casts land on the two other players, one each. Ordel reads "Something's
      Heal envelops you in healing energy." and never a name.
  - id: named-to-nobody
    do: The caster drinks their draught, says they can see, and types `cast heal Ordel`.
    verify: >-
      It lands. Ordel, still unable to see, reads "Something's Heal", not the
      caster's name. The infrared tester reads "a figure" for both parties.
```

`tools/playtest/goals/scenarios/slice-a-dark-cave/caster.yaml`:

```yaml
# Slice A lane 1: the CASTER. You are Sil Vantage in an UNLIT cave with no
# night vision. Ordel Quist and Vessa Thorn are with you. Quote every line
# VERBATIM. 3 commands per round on the AI port; the overflow is dropped.

ephemeral:
  profile: m2-actor
  start_room: 3101
  overlays:
    grant_spells:
      mind-spike: 1
    grant_items: [30047]
  budgets:
    wall_clock: 25m

goals:
  - >-
    Type `look`. If you can read a normal room description, say so at once:
    the lane is void.
  - >-
    Type `score` or `status` and note your conviction. Type `cast heal Ordel`.
    Quote exactly what you are told. THE CORRECT ANSWER IS A REFUSAL: "You don't
    see them here." Then type `cast mind-spike Ordel` and quote the reply, which
    must also be that refusal. Check your conviction did not drop.
  - >-
    Type `cast heal` with NO target. It should start and land on you. Quote it.
  - >-
    Wait for Vessa to finish her shape casts. Then type `drink draught` and say
    when you can see. Type `cast heal Ordel`. It should land. Ask Ordel exactly
    what they read, and ask Vessa what she read.
```

`tools/playtest/goals/scenarios/slice-a-dark-cave/witness.yaml`:

```yaml
# Slice A lane 1: the WITNESS. You are Ordel Quist in an UNLIT cave with no
# special senses. You cannot see, on purpose. Quote every line you receive
# VERBATIM, including anything that mentions a spell.

ephemeral:
  profile: m2-witness
  start_room: 3101
  budgets:
    wall_clock: 25m

goals:
  - >-
    Type `look`. If you can read a normal room description, say so at once.
  - >-
    Stay put and do not drink anything. Every time you receive a line about a
    spell or healing, quote it exactly and say whether it names anyone. A NAME
    IS THE DEFECT: you cannot see who cast it. "Something" is correct.
```

`tools/playtest/goals/scenarios/slice-a-dark-cave/infrared.yaml`:

```yaml
# Slice A lane 1: the INFRARED tester. You are Vessa Thorn in an UNLIT cave.
# Your eyes see heat shapes but no faces. Quote every line VERBATIM.

ephemeral:
  profile: slice-a-infrared
  start_room: 3101
  budgets:
    wall_clock: 25m

goals:
  - >-
    Type `conditions`, then `buffs` if that lists nothing. Quote the list. If
    InfraredVision is NOT active, say so at once and stop: the lane cannot test
    shapes.
  - >-
    Type `cast heal Ordel`. Quote the reply. It MUST be a refusal with a hint
    that mentions `cast heal shape`.
  - >-
    Type `cast heal shape`. When it lands, type `cast heal 2.shape`. Quote both.
    Say who each landed on if you are told. Ask Ordel what they read.
  - >-
    When Sil casts heal on Ordel, quote the line you receive. It should say "a
    figure" for both of them and name nobody.
```

- [ ] **Step 3: Lanes 2 and 3 scenario and goals**

`tools/playtest/scenarios/slice-a-hidden.yaml`:

```yaml
# Follow-up slice A, lanes 2 and 3: hidden creatures.
#
# Lane 2 (room 462, Thornwall City, lit): a sneaker hides from a seer who has
# Veil Sight and NO pet. Lane 3 (room 498, quest 14): a quester without
# see-hidden must find Torvan Cresk with `search` before naming him.
name: slice-a-hidden
mode: party
summary: >-
  Hidden creatures cannot be named until perceived, Veil Sight reveals them
  with no pet, and a search drags Torvan Cresk out of hiding.
on_actor_stop: continue
budgets:
  wall_clock: 30m
requires:
  max_connections: 20
roster:
  - id: sneaker
    personality: bug-finder
    goals: goals/scenarios/slice-a-hidden/sneaker.yaml
  - id: seer
    personality: bug-finder
    goals: goals/scenarios/slice-a-hidden/seer.yaml
  - id: quester
    personality: bug-finder
    goals: goals/scenarios/slice-a-hidden/quester.yaml

group_goals:
  - id: hidden-unnamed
    do: The sneaker sneaks; the seer types `look`, `look <sneaker>`, `consider <sneaker>`, `attack <sneaker>` and `cast veil-sight <sneaker>`.
    verify: The sneaker is not listed and every named command reads as if nobody by that name is there.
  - id: veil-sight-reveals
    do: The seer types `cast veil-sight`, then `look` and `look <sneaker>`.
    verify: The sneaker is listed and can be looked at. The seer has no pet.
  - id: search-drags-out
    do: In room 498 the quester waits until Torvan Cresk is not listed, tries `attack torvan`, then searches until he is found.
    verify: >-
      The attack reads as if nobody is there. After the find lists "Torvan
      Cresk (hiding)", `look` lists him and `attack torvan` starts a fight.
```

`tools/playtest/goals/scenarios/slice-a-hidden/sneaker.yaml`:

```yaml
# Slice A lane 2: the SNEAKER. You are Midroad Scout. Quote every line VERBATIM.

ephemeral:
  profile: mid
  start_room: 462
  budgets:
    wall_clock: 30m

goals:
  - >-
    Type `sneak`. Quote the reply. If you are spotted or it fails, wait a round
    and try again until you are told you are sneaky. Tell Sil Vantage the moment
    you are hidden, then stay put and do nothing else that would end it.
  - >-
    Quote any line that says you were spotted or noticed.
```

`tools/playtest/goals/scenarios/slice-a-hidden/seer.yaml`:

```yaml
# Slice A lane 2: the SEER. You are Sil Vantage. You have no pet. Quote every
# line VERBATIM.

ephemeral:
  profile: m2-actor
  start_room: 462
  overlays:
    grant_spells:
      veil-sight: 1
  budgets:
    wall_clock: 30m

goals:
  - >-
    Type `look` and note who is listed. Wait until Midroad Scout says they are
    hidden.
  - >-
    Type `look`. Midroad Scout must NOT be listed. Then type `look scout`,
    `consider scout`, `attack scout` and `cast veil-sight scout`, one at a time.
    Quote each reply. Each must read as if nobody by that name is here. If the
    attack starts a fight, that is the defect: say so.
  - >-
    Type `cast veil-sight` with no target. When it lands, type `look`. Midroad
    Scout MUST now be listed. Type `look scout` and quote it. Confirm you have
    no pet.
```

`tools/playtest/goals/scenarios/slice-a-hidden/quester.yaml`:

```yaml
# Slice A lane 3: the QUESTER. You are Ordel Quist on step `confront` of The
# Undertow, in Torvan Cresk's operations room. You have no way to see hidden
# creatures. Quote every line VERBATIM.

ephemeral:
  profile: m2-witness
  start_room: 498
  overlays:
    set_quest_tokens: ["14-start", "14-confront"]
  budgets:
    wall_clock: 30m

goals:
  - >-
    Type `look`. If Torvan Cresk is listed, wait a few rounds and `look` again,
    until he is NOT listed (he slips into the shadows on his own). Quote the
    listing each time. If after 15 looks he is always listed, say so and stop.
  - >-
    While he is not listed, type `attack torvan`. Quote the reply. It must read
    as if nobody is there; a fight starting is the defect.
  - >-
    Type `search`. Quote the whole reply. Repeat, waiting for the cooldown,
    until the reply lists "Torvan Cresk (hiding)". Then type `look`: he MUST be
    listed. Type `attack torvan` and confirm the fight starts. You may flee
    after the first exchange.
```

- [ ] **Step 4: Commit**

The harness reads these files when Task 17 launches each scenario; a malformed one surfaces there, before any tester connects.

```bash
git add tools/playtest/profiles/slice-a-infrared.yaml tools/playtest/scenarios/slice-a-dark-cave.yaml tools/playtest/scenarios/slice-a-hidden.yaml tools/playtest/goals/scenarios/slice-a-dark-cave/caster.yaml tools/playtest/goals/scenarios/slice-a-dark-cave/witness.yaml tools/playtest/goals/scenarios/slice-a-dark-cave/infrared.yaml tools/playtest/goals/scenarios/slice-a-hidden/sneaker.yaml tools/playtest/goals/scenarios/slice-a-hidden/seer.yaml tools/playtest/goals/scenarios/slice-a-hidden/quester.yaml
git commit -m "test(playtest): slice A lanes for aiming and names in the dark" -m "Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>" -m "Claude-Session: https://claude.ai/code/session_01J9ARXi13pxi92SKumuvpum"
```


---

### Task 16: Ship gates

- [ ] **Step 1: Local gates**

```bash
gofmt -l internal/ modules/
go vet ./internal/messaging/ ./internal/rooms/ ./internal/actions/ ./internal/hooks/ ./internal/characters/
go build ./...
go test ./...
golangci-lint run --new-from-rev=master
```

Expected: `gofmt` prints nothing, everything else passes. The existing goldens must be byte-identical: `git diff --stat master -- '*.golden' 'testdata/'` shows no changes.

- [ ] **Step 2: Boot check in an isolated worktree**

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log
grep -c "Server Ready" boot.log
```

Expected: exit code 124 from `timeout` (the server stayed up), `0` for the panic count (run that `grep -c` standalone: it exits 1 on zero matches), `1` for Server Ready. Tear down with `git worktree remove --force C:/tmp/dogmud-boot-check`; if Windows holds the exe, `Remove-Item -Recurse -Force C:\tmp\dogmud-boot-check` then `git worktree prune`.

- [ ] **Step 3: Record completion in the spec's README row**

No file change needed if the row already describes the design; confirm `grep -c "followup-slice-a" docs/README.md` is `2` (spec and plan rows).

---

### Task 17: Playtest, then the PR

- [ ] **Step 1: Run both scenarios**

Follow the `dogmud-playtesting` skill. For each of `tools/playtest/scenarios/slice-a-dark-cave.yaml` and `tools/playtest/scenarios/slice-a-hidden.yaml`:

```text
/playtest-scenario --checkout <absolute repo path> tools/playtest/scenarios/<scenario>.yaml
```

After each run, confirm teardown with `docker ps` and remove only `dogmud-playtest-<run_id>-server-1` by exact name with `docker rm -f`. Never touch other containers.

- [ ] **Step 2: Judge the lanes against the spec**

Each group goal passes as written in the scenario. Record, for the owner:
- every awkward swapped-name line a tester quotes (spec risk 1);
- whether the infrared profile's buff 85 was active (Lane 1 goal 1). If it was not, the profile's `buffs` shape does not load and that lane is void; report it rather than editing the profile mid-run.

Playtest reports are gitignored: extract findings into the slice A memory file before the report is cleaned up.

- [ ] **Step 3: Push and open the PR**

```bash
git push -u origin feature/followup-slice-a-names-in-dark
gh pr create --repo pruuk/DOGMud --base master --head feature/followup-slice-a-names-in-dark --title "Follow-up slice A: names in the dark" --body-file <path to a body file written with the Write tool>
gh pr checks <n> --repo pruuk/DOGMud --watch
gh run list --repo pruuk/DOGMud --branch feature/followup-slice-a-names-in-dark
```

The body lists the ten rulings, the behaviour changes players will notice (spec Risks), the playtest results and SWEEP's unit-test-only coverage, and ends with the PR attribution line. Read the URL `gh` prints and confirm it says `pruuk/DOGMud`.

- [ ] **Step 4: Merge only on the owner's word**

When the owner says to merge: `gh pr merge <n> --repo pruuk/DOGMud --merge --delete-branch`. The owner runs the deploy.
