# Messaging M7 slice 1: the refusal package Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Every subagent brief carries these rules:** edit files with the Edit and Write tools only (no `sed -i`, no Python `open(path, 'w')`, no `>` into a tracked file; `~/.claude/hooks/guard.py` blocks them); `git add` named paths only; never `--no-verify`; no em or en dashes in prose or player text; every `gh` command carries `--repo pruuk/DOGMud`.

**Goal:** Give the most repeated refusals one home (`internal/refusal`), one category (`CategoryRefusal`) and one wording each, and stop a new copy appearing with a phrase guard.

**Architecture:** A new Go package holds one helper per refusal situation. Each helper takes a `Recipient` (anything with `SendText`, so both `*users.UserRecord` and `actions.Actor`) plus keywords, and sends one line on the new `messaging.CategoryRefusal`, which renders exactly as `CategorySystem` does today (same colour value, every normalize stage, no wrap). Call sites in `internal/` and `modules/` switch to the helpers; a root guard fails on any of the retired phrases in a Go string literal outside the package.

**Tech Stack:** Go, `go/ast` for the guard, testify.

**Spec:** `docs/superpowers/specs/completed/2026-10-14-messaging-m7-refusals-design.md` (owner approved 2026-10-15). This is slice 1 of 4 from the spec's "Suggested slicing".

---

## Facts verified against source (`origin/master` `72b1e9edf`, 2026-10-15)

| # | Fact | Source |
|---|---|---|
| 1 | Category enum ends `CategoryCraft`, `CategoryCraftSeen`, then `categoryMax`; new values are appended before `categoryMax` | `internal/messaging/messaging.go:116-128` |
| 2 | `String()` cases end `case CategoryCraftSeen: return "craft-seen"`, then `return "Unknown"` | `messaging.go:259-264` |
| 3 | `TestCategoryStringRoundTrip` walks every value and fails on `"Unknown"` or a duplicate | `internal/messaging/messaging_test.go:11-23` |
| 4 | `shouldWrap` is an allowlist; `TestShouldWrapMatchesPinnedAllowlist` pins exactly 48 admitted; `TestMixedAndPreformattedCategoriesNeverWrap` lists never-wrap categories | `pipeline.go:121-156`, `pipeline_test.go:118-160` |
| 5 | `skipStages` has no case for System, so it gets every stage (default 0) | `internal/messaging/normalize.go:25-45` |
| 6 | `applyCategoryColor` paints with the alias named by `String()`; dogmud aliases `system: 8`, `error: 9`, `craft: 8` | `pipeline.go:170-175`, `_datafiles/world/dogmud/ansi-aliases.yaml:260,261,287` |
| 7 | The `default` world's `ansi-aliases.yaml` defines none of `system`, `craft`, `craft-seen`; only dogmud's does | `grep -n "system\|craft" _datafiles/world/*/ansi-aliases.yaml` |
| 8 | Prior art for a new category's tests: `category_craft_test.go` (`TestCategoryCraftKeepsSystemTreatment`: same `skipStages`, no verbosity suppresses) | `internal/messaging/category_craft_test.go:62-77` |
| 9 | `Verbosity.Suppresses` reads two maps; a category in neither is never suppressed | `internal/messaging/verbosity.go:91-98` |
| 10 | `actions.Actor` has `SendText(cat messaging.Category, msg string)`; `(*users.UserRecord).SendText` has the same signature; `MobActor.SendText` is a no-op | `internal/actions/actor.go:25`, `internal/users/userrecord.go:485`, `actor_mob.go:43` |
| 11 | `internal/util` and `internal/mudlog` import no package that imports `users` or `actions` (deps: copyover, mudlog, term, util) | `go list -deps ./internal/util ./internal/mudlog` |
| 12 | `util.EscapeAnsiTags` neutralises `<ansi` in player text; no exported helper strips `<ansi>` tags (`util.ansiTagRegex` is unexported; `util.StripANSI` strips ESC codes only) | `internal/util/ansiescape.go`, `util.go:80,237,1053` |
| 13 | `mudlog.Error(msg string, args ...any)` | `internal/mudlog/mudlog.go:98` |
| 14 | `AimNothingLine` = "You can't see anything to aim at.", `AimNotHereLine` = "You don't see them here." in `actions/sight_aim.go:19-27`; 2 production uses outside the file (`attack.go:113`, `pet.go:62`), 27 test uses in 6 test files | `grep -rn "AimNotHereLine\|AimNothingLine"` |
| 15 | `AimBySight`/`aimAtSight` refusals are sent on `CategorySystem` at 14 sites: `cast_admission.go:56` (`actor`), `melee_target.go:176`, `ask.go:96`, `attack.go:108`, `consider.go:33`, `give.go:85`, `party.go:185`, `report.go:67`, `show.go:46`, `skill.skullduggery.shadow.go:56`, `talk.go:45`, `target.go:64`, `aicompanion/commands.go:638`, `follow/follow.go:434`; plant and steal (`skill.skullduggery.plant.go:96`, `steal.go:86`) send `aimRefusal` later in the function; `give.go:443` only tests it | grep `-A3` of each call |
| 16 | `target.go:81,106,132`, `report.go:76` and `skill.cast.go:246` echo a creature name the player typed (`You don't see '%s' here.`), which #454 forbids | read at those lines |
| 17 | Creature not-here without echo: `melee_target.go:189`, `consider.go:42`; "Can't find that to pet." `pet.go:68,74` | read |
| 18 | Thing not-here: room containers `plant.go:351`, `steal.go:534`; bauble `steal.go:867`; floor `get.go:80` (echoes), `get.go:737` (echoes); corpse `loot.go:49,163` (echo), `assess.go:29` ("those remains") | read |
| 19 | Left for slice 3 (they name the container, and `get_corpse_sight_test.go:47` pins the corpse wording): `get.go:355,457,541,800` | read |
| 20 | Not-carrying sites (held item, typed name in scope): `actions/drink.go:208` (`rest`), `actions/plant.go:85` (`opts.ItemNoun`), `actions/sell.go:180` (`itemName`), `appraise.go:49` (`rest`), `drop.go:125` (`itemName`), `drop.go:141` (`rest`), `eat.go:38`, `equip.go:102`, `give.go:67` (`giveWhat`), `guild.go:593` (`name`), `offer.go:39`, `put.go:94` (no single name: `""`), `read.go:46`, `show.go:39` (`objectName`), `skill.disenchant.go:40`, `stash.go:18`, `storage.go:154,186` (`itemName`), `throw.go:213`, `use.go:78`, `admin.redescribe.go:37`, `admin.rename.go:35`, `modules/auctions/auctions.go:388`, `modules/cleanup/cleanup.go:71` (all `rest` unless named) | scan of string literals, read at each line |
| 21 | Not in this slice (gold, keys, quests, slots, arms, abilities, storage withdraw): `drop.go:42,84`, `give.go:55`, `put.go:99`, `bank.go`, `share.go`, `mail.go:157`, `lock.go`, `unlock.go`, `hint.go`, `storage.go:250,264,279`, `enchant_slot.go:30`, `salvage.go:43`, `shoot.go:118`, `sort.go:23`, `wear_slot.go:279`, `worn.go:474`, `mutation_helpers.go:81`, `skill.cast.go:122`, `buy.go` | same scan |
| 22 | "You have no target!" at 10 sites: `drain.go:45`, `gore.go:45`, `grapple.go:49`, `kick.go:47`, `maul.go:45`, `pounce.go:52`, `rake.go:45`, `taunt.go:51`, `throttle.go:57`, `trip.go:53`; `NoTarget` means "no aggro target, or the target is gone" | `combat_bash.go:40` |
| 23 | Cooldown line at 16 literals: `consider.go:26` (const `considerCooldownText`, sent `:55`), `mutation_helpers.go:99`, and `bash, drain, gore, grapple, kick, maul, pounce, rake, rally, taunt, throttle, throw, trip, warcry` .go (lines 53, 41, 41, 44, 43, 41, 48, 41, 34, 46, 53, 239, 48, 36); plus `attack.go:443` `surpriseMeleeDeniedText` (sent on `CategorySurpriseAttack` at `:472`, pinned verbatim at `attack_ambush_denial_test.go:281-283`). The timer is SHARED by every special move | `mutation_helpers.go:96` "shared special-move cooldown" |
| 24 | "You are not in a guild." at 15 sites in `guild.go` (93 with a hint, then 175, 211, 263, 329, 363, 400, 447, 524, 553, 583, 612, 647, 709, 743) | scan |
| 25 | "You have no companion travelling with you." at `modules/aicompanion/commands.go:526,543,560,620,678,721` | scan |
| 26 | Party leader refusals: `party.go:176` (invite), `:560` (disband), `:593` (kick), `:624` (promote), `:657` (loot: "Only the party leader can set the loot mode.") | enclosing funcs |
| 27 | "You can't do that while in combat!": `actions/defuse.go:71`, `plant.go:54`, `shadow.go:69`, `steal.go:103`, `skill.skullduggery.sneak.go:60` | scan |
| 28 | "Something went wrong" literals: `give.go:108,130,156,218,276,319`, `party.go:160,219,237,269,285`, `show.go:61`, `character.go:273`, `actions/salvage.go:330` (already `CategoryError`), `admin.room.go:55` (an `fmt.Errorf` whose text is sent), `behaviortree/actions_mob.go:511,523` (mob `say`; 511 already logs, 523 does not) | scan, read |
| 29 | Hint house style: 39 Go literals write a command as `Try <ansi fg="command">...</ansi>`, 3 as `Try '...'` (2 of them admin weather output); `AimShapesHint` uses the ansi form | literal scan, `sight_aim.go:96` |
| 30 | Tests that pin text this slice changes: `consider_test.go:123` ("moment to recover"), `attack_ambush_denial_test.go:281-283`; `cast_sight_test.go:138` and `consider_multiword_test.go:81` pin "You don't see them here.", which does not change | grep of `_test.go` |
| 31 | Root guard prior art: `go_prose_dash_guard_test.go` walks `internal` and `modules`, skips `_test.go`, parses with `go/parser`, checks `*ast.BasicLit` strings, and fails if it scanned fewer than 100 files | file |
| 32 | Command test fixture: `aimScene(t, aimFull|aimShapes|aimDark)` seeds room 1 with Aliceia (user 1), Bobrick (user 2), Skeleton (mob 100); `aimTold(1)` drains with tags stripped | `internal/usercommands/sight_aim_commands_test.go:36-70` |
| 34 | Dry run of Task 10's guard against this master: 117 hits, every one a site in Tasks 4 to 8 (not here 17 plus `sight_aim.go:27`, not carrying 24, no target and cooldown 27, membership and in combat 31, failures 17); `get.go:355,457,541,800`, gold, key and quest lines produce none; `TestRefusalPhraseGuardCanFail` passes | guard copied to the repo root, run, removed |
| 33 | Commands are registered with backtick keys, e.g. `` `inventory`: {Inventory, true, true, false} `` | `internal/usercommands/usercommands.go:139` |

## Wording (owner review: approve with the plan)

Every line is under 80 visible columns at the longest echo (24 runes plus `...`).

| Helper | Line | Replaces |
|---|---|---|
| `NotHereCreatureLine` | You don't see them here. | unchanged text; now also `target`, `report`, `cast`, `pet` (no echo) |
| `NotHereThingLine` | You don't see that here. | unchanged text; now also `get`, `loot`, `assess` (no echo) |
| `NothingToAimLine` | You can't see anything to aim at. | unchanged |
| `NotCarrying(u, "rope")` | You aren't carrying 'rope'. Try **inventory**. | "You don't have a "rope" to drink." and 22 siblings |
| `NotCarrying(u, "")` | You aren't carrying that. Try **inventory**. | "You don't have that item.", "You don't seem to be carrying that." |
| `NoTarget(u, "kick")` | You have no target. Try **kick &lt;name&gt;**. | "You have no target!" |
| `Cooldown(u, "kick")` | You need a moment to recover before you can kick. | "...before attempting another special move." |
| ambush | No ambush ready. You need a moment to recover before you can ambush. | the trimmed ambush line |
| `WhileFighting(u, "steal")` | You can't steal while fighting. | "You can't do that while in combat!" |
| `NotInGuild(u)` | You aren't in a guild. Try **guild create &lt;TAG&gt; &lt;name&gt;**. | "You are not in a guild." |
| `NoCompanion(u)` | You have no companion travelling with you. | unchanged text |
| `NotPartyLeader(u, "set the loot mode")` | Only the party leader can set the loot mode. | "You are not the leader of your party." |
| `Failed(u, "", err)` | That didn't work. The error has been logged. | "Something went wrong." |
| `Failed(u, "salvage", err)` | That salvage didn't work. The error has been logged. | "Something went wrong with your salvage attempt." |
| portal mob, 511 | say The planes resist me. Your gold is returned. | "say Something went wrong. The planes resist me..." |
| portal mob, 523 | say The way will not open. Your gold is returned. | "say Something went wrong. Your gold is returned." |

**Bold** is the command colour (`<ansi fg="command">`), the codebase's style (fact 29). Owner ruling 2026-10-15: hints use the command colour, not the single quotes spec section 3 named.

## File structure

| File | Responsibility |
|---|---|
| Create `internal/refusal/refusal.go` | `Recipient`, `send`, `echo`, `hint` |
| Create `internal/refusal/nothere.go` | sight-safe not-here lines |
| Create `internal/refusal/carrying.go` | `NotCarrying` |
| Create `internal/refusal/combat.go` | `NoTarget`, `Cooldown`, `WhileFighting` |
| Create `internal/refusal/membership.go` | `NotInGuild`, `NoCompanion`, `NotPartyLeader` |
| Create `internal/refusal/failed.go` | `Failed` |
| Create `internal/refusal/refusal_test.go` | unit tests |
| Create `internal/refusal/context.md` | package doc |
| Create `internal/messaging/category_refusal_test.go` | category treatment |
| Create `refusal_phrase_guard_test.go` (repo root) | phrase guard |
| Create `internal/usercommands/refusal_echo_sight_test.go` | no echo through sight |
| Modify | the call sites in facts 14 to 28, `messaging.go`, `pipeline_test.go`, `ansi-aliases.yaml`, three `context.md`, `docs/README.md`, `docs/PATCH_NOTES.md` |

---

### Task 0: Worktree

- [ ] **Step 1:** After PR #543 (spec and this plan) merges:

```bash
cd "/c/Users/Calabe Davis/workspace/DOGMud" && git fetch -q origin
git worktree add -b feature/m7-slice1-refusals C:/tmp/dogmud-m7s1 origin/master
```

Record `C:/tmp/dogmud-m7s1` in the session handoff (it keeps the tmp sweep off it). All later paths are relative to it.

### Task 1: `CategoryRefusal`

**Files:** Modify `internal/messaging/messaging.go:116-128` and `:259-264`, `_datafiles/world/dogmud/ansi-aliases.yaml:292`, `internal/messaging/pipeline_test.go`. Create `internal/messaging/category_refusal_test.go`.

- [ ] **Step 1: Write the failing test** `internal/messaging/category_refusal_test.go`:

```go
package messaging

import "testing"

// TestCategoryRefusalKeepsSystemTreatment pins M7's promise (#371): moving a
// refusal off CategorySystem changes nothing a player sees. Same alias
// value (checked in the data test below), same normalisation, never wrapped,
// never suppressed.
func TestCategoryRefusalKeepsSystemTreatment(t *testing.T) {
	if CategoryRefusal.String() != "refusal" {
		t.Fatalf("CategoryRefusal.String() = %q, want refusal", CategoryRefusal.String())
	}
	if skipStages(CategoryRefusal) != skipStages(CategorySystem) {
		t.Error("CategoryRefusal must normalise exactly as CategorySystem")
	}
	if shouldWrap(CategoryRefusal) {
		t.Error("CategoryRefusal must not wrap: CategorySystem never did")
	}
	for _, v := range []Verbosity{VerbosityFull, VerbosityMedium, VerbosityLight} {
		if v.Suppresses(CategoryRefusal) {
			t.Errorf("verbosity %v suppresses CategoryRefusal; no tier may", v)
		}
	}
}

// TestCategoryRefusalRendersAsSystem renders one refusal on both categories
// and compares them with the colour tag's name taken out.
func TestCategoryRefusalRendersAsSystem(t *testing.T) {
	text := "You aren't carrying 'rope'."
	render := func(c Category) string {
		return RenderForRecipient(RenderInput{Category: c, Text: text, Channel: ChannelAudio, LineWidth: 80})
	}
	sys, ref := render(CategorySystem), render(CategoryRefusal)
	if want := `fg="refusal"`; !strings.Contains(ref, want) {
		t.Fatalf("refusal render %q lacks %s", ref, want)
	}
	if strings.ReplaceAll(ref, `fg="refusal"`, `fg="system"`) != sys {
		t.Errorf("render differs beyond the alias name:\nsystem  %q\nrefusal %q", sys, ref)
	}
}
```

with the import block `import (\n\t"strings"\n\t"testing"\n)`.

- [ ] **Step 2: Run** `go test ./internal/messaging/ -run 'CategoryRefusal' -count=1`. Expected: FAIL, `undefined: CategoryRefusal`.

- [ ] **Step 3: Implement.** In `messaging.go`, after `CategoryCraftSeen` and before the `categoryMax` comment:

```go
	// Refusals and prompts (messaging M7, #371): every line internal/refusal
	// sends. Appended so no existing value shifts. Treated exactly as
	// CategorySystem, where these lines rode (every normalize stage, never
	// wrapped, alias refusal at the same value as system), so the move
	// changes nothing a player sees until the owner recolours it.
	CategoryRefusal
```

In `String()` after the `CategoryCraftSeen` case:

```go
	case CategoryRefusal:
		return "refusal"
```

In `ansi-aliases.yaml`, after the `craft-seen: 144` block:

```yaml

# Refusals and prompts (messaging M7, #371). Same value as `system`, the
# category these lines rode, so moving them changes nothing a player sees.
  refusal: 8
```

In `pipeline_test.go`, add to the `neverWrap` map in `TestMixedAndPreformattedCategoriesNeverWrap`:

```go
		CategoryRefusal:         "refusals rode CategorySystem, which never wraps (M7, #371)",
```

- [ ] **Step 4: Run** `go test ./internal/messaging/ -count=1`. Expected: PASS (the round-trip test now covers `refusal`; the wrap pin stays at 48).

- [ ] **Step 5: Commit**

```bash
git add internal/messaging/messaging.go internal/messaging/category_refusal_test.go internal/messaging/pipeline_test.go _datafiles/world/dogmud/ansi-aliases.yaml
git commit -F- <<'EOF'
feat(messaging): CategoryRefusal, treated as CategorySystem (#371)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

### Task 2: Package core

**Files:** Create `internal/refusal/refusal.go`, `internal/refusal/refusal_test.go`.

- [ ] **Step 1: Write the failing test** `internal/refusal/refusal_test.go`:

```go
package refusal

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sent struct {
	cat  messaging.Category
	text string
}

// fakeRecipient records what a helper tells it.
type fakeRecipient struct{ got []sent }

func (f *fakeRecipient) SendText(cat messaging.Category, msg string) {
	f.got = append(f.got, sent{cat, msg})
}

// one asserts a helper sent exactly one line on cat and returns it.
func one(t *testing.T, f *fakeRecipient, cat messaging.Category) string {
	t.Helper()
	require.Len(t, f.got, 1, "a refusal is exactly one line")
	assert.Equal(t, cat, f.got[0].cat)
	return f.got[0].text
}

func TestEchoTrimsStripsAndCaps(t *testing.T) {
	assert.Equal(t, "rope", echo("  rope "))
	assert.Equal(t, "red rope", echo(`<ansi fg="red">red rope</ansi>`))
	assert.NotContains(t, echo(`<ansi fg="red"`), "<ansi", "a half tag is escaped")
	long := strings.Repeat("x", 40)
	assert.Equal(t, strings.Repeat("x", echoMax)+"...", echo(long))
	assert.Equal(t, "", echo("   "))
}

func TestHintNamesACommand(t *testing.T) {
	assert.Equal(t, ` Try <ansi fg="command">kick <name></ansi>.`, hint("kick <name>"))
}

// fits80 is the player-copy width rule for every refusal line.
func fits80(t *testing.T, line string) {
	t.Helper()
	assert.LessOrEqual(t, util.VisibleWidth(line), 80, "%q", line)
}
```

- [ ] **Step 2: Run** `go test ./internal/refusal/ -count=1`. Expected: FAIL, package has no non-test files / `undefined: echo`.

- [ ] **Step 3: Implement** `internal/refusal/refusal.go`:

```go
// Package refusal holds one helper per refusal situation (messaging M7,
// #371): the line a player reads when a command cannot do what they asked.
// Each helper sends exactly one line on messaging.CategoryRefusal and has a
// ...Text sibling (or a ...Line constant) returning it. See context.md.
package refusal

import (
	"regexp"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Recipient is whoever a refusal is told to: *users.UserRecord and every
// actions.Actor satisfy it (a mob actor's SendText does nothing).
type Recipient interface {
	SendText(cat messaging.Category, msg string)
}

// echoMax is the most runes of a player's own input a line repeats, so the
// longest echo still fits 80 columns.
const echoMax = 24

var ansiTag = regexp.MustCompile(`</?ansi[^>]*>`)

func send(to Recipient, line string) {
	to.SendText(messaging.CategoryRefusal, line)
}

// echo is typed as a refusal may repeat it: tags stripped, trimmed, capped at
// echoMax runes, and any leftover markup escaped, so typed input cannot style
// the line. Only names the player holds or knows are echoed (spec section 2).
func echo(typed string) string {
	s := strings.TrimSpace(ansiTag.ReplaceAllString(typed, ``))
	if r := []rune(s); len(r) > echoMax {
		s = strings.TrimSpace(string(r[:echoMax])) + `...`
	}
	return util.EscapeAnsiTags(s)
}

// hint is the tail naming a command the player can type next.
func hint(command string) string {
	return ` Try <ansi fg="command">` + command + `</ansi>.`
}
```

- [ ] **Step 4: Run** `go test ./internal/refusal/ -count=1`. Expected: PASS.

- [ ] **Step 5: Commit** `internal/refusal/refusal.go internal/refusal/refusal_test.go` with message `feat(refusal): package core, echo and hint (#371)` and the co-author trailer (heredoc as in Task 1).

### Task 3: The families

**Files:** Create `nothere.go`, `carrying.go`, `combat.go`, `membership.go`, `failed.go` in `internal/refusal/`; append to `refusal_test.go`.

- [ ] **Step 1: Append the failing tests** to `internal/refusal/refusal_test.go`:

```go
func TestNotHereLinesNeverEcho(t *testing.T) {
	// The sight-gated helpers take no name at all: a creature that is there
	// but unseen and a name that matches nothing cannot read differently (#454).
	for line, send := range map[string]func(Recipient){
		NotHereCreatureLine: NotHereCreature,
		NotHereThingLine:    NotHereThing,
		NothingToAimLine:    NothingToAim,
	} {
		f := &fakeRecipient{}
		send(f)
		assert.Equal(t, line, one(t, f, messaging.CategoryRefusal))
	}
	assert.Equal(t, "You don't see them here.", NotHereCreatureLine)
	assert.Equal(t, "You don't see that here.", NotHereThingLine)
	assert.Equal(t, "You can't see anything to aim at.", NothingToAimLine)
}

func TestNotCarrying(t *testing.T) {
	f := &fakeRecipient{}
	NotCarrying(f, " rope ")
	assert.Equal(t, `You aren't carrying 'rope'. Try <ansi fg="command">inventory</ansi>.`,
		one(t, f, messaging.CategoryRefusal))
	assert.Equal(t, `You aren't carrying that. Try <ansi fg="command">inventory</ansi>.`, NotCarryingText(""))
	fits80(t, NotCarryingText(strings.Repeat("w", 60)))
}

func TestCombatFamily(t *testing.T) {
	f := &fakeRecipient{}
	NoTarget(f, "kick")
	assert.Equal(t, `You have no target. Try <ansi fg="command">kick <name></ansi>.`, one(t, f, messaging.CategoryRefusal))

	f = &fakeRecipient{}
	Cooldown(f, "kick")
	assert.Equal(t, "You need a moment to recover before you can kick.", one(t, f, messaging.CategoryRefusal))

	f = &fakeRecipient{}
	WhileFighting(f, "steal")
	assert.Equal(t, "You can't steal while fighting.", one(t, f, messaging.CategoryRefusal))

	// The player-copy rule: a cooldown never names a duration.
	assert.NotRegexp(t, `[0-9]`, CooldownText("kick"))
	fits80(t, "No ambush ready. "+CooldownText("ambush"))
}

func TestMembershipFamily(t *testing.T) {
	f := &fakeRecipient{}
	NotInGuild(f)
	line := one(t, f, messaging.CategoryRefusal)
	assert.Equal(t, `You aren't in a guild. Try <ansi fg="command">guild create <TAG> <name></ansi>.`, line)
	fits80(t, line)

	f = &fakeRecipient{}
	NoCompanion(f)
	assert.Equal(t, "You have no companion travelling with you.", one(t, f, messaging.CategoryRefusal))

	f = &fakeRecipient{}
	NotPartyLeader(f, "set the loot mode")
	assert.Equal(t, "Only the party leader can set the loot mode.", one(t, f, messaging.CategoryRefusal))
	assert.Equal(t, "Only the party leader can do that.", NotPartyLeaderText(""))
}

func TestFailedTellsPlainlyOnError(t *testing.T) {
	f := &fakeRecipient{}
	Failed(f, "", assert.AnError, "userId", 7)
	assert.Equal(t, "That didn't work. The error has been logged.", one(t, f, messaging.CategoryError))
	assert.Equal(t, "That salvage didn't work. The error has been logged.", FailedText("salvage"))
	assert.NotContains(t, FailedText(""), assert.AnError.Error(), "the cause goes to the log, not the player")
}
```

- [ ] **Step 2: Run** `go test ./internal/refusal/ -count=1`. Expected: FAIL, undefined helpers.

- [ ] **Step 3: Implement.**

`internal/refusal/nothere.go`:

```go
package refusal

// The sight-safe not-here lines (spec section 2, #454). They never echo:
// a name the player can only find by looking reads the same whether it is
// there unseen or matches nothing. Callers that return a line instead of
// sending it (actions.AimBySight) use the constants.
const (
	// NotHereCreatureLine answers a creature name the player cannot pick out.
	NotHereCreatureLine = `You don't see them here.`
	// NotHereThingLine answers a thing in the room (floor, room container,
	// corpse) the player cannot pick out.
	NotHereThingLine = `You don't see that here.`
	// NothingToAimLine answers a player who sees nothing and named a shape,
	// or named nothing at all.
	NothingToAimLine = `You can't see anything to aim at.`
)

func NotHereCreature(to Recipient) { send(to, NotHereCreatureLine) }
func NotHereThing(to Recipient)    { send(to, NotHereThingLine) }
func NothingToAim(to Recipient)    { send(to, NothingToAimLine) }
```

`internal/refusal/carrying.go`:

```go
package refusal

// NotCarryingText answers a name the player looked for in what they carry.
// It echoes typed (the player's own belongings, spec section 2); an empty
// typed reads "that".
func NotCarryingText(typed string) string {
	what := `that`
	if e := echo(typed); e != `` {
		what = `'` + e + `'`
	}
	return `You aren't carrying ` + what + `.` + hint(`inventory`)
}

func NotCarrying(to Recipient, typed string) { send(to, NotCarryingText(typed)) }
```

`internal/refusal/combat.go`:

```go
package refusal

// NoTargetText answers a combat move with no target; verb is the command.
func NoTargetText(verb string) string {
	return `You have no target.` + hint(verb+` <name>`)
}

// CooldownText answers the shared special-move timer. doing names what the
// player tried ("kick", "consider anyone"); never a duration.
func CooldownText(doing string) string {
	return `You need a moment to recover before you can ` + doing + `.`
}

// WhileFightingText answers an action combat forbids.
func WhileFightingText(doing string) string {
	return `You can't ` + doing + ` while fighting.`
}

func NoTarget(to Recipient, verb string)       { send(to, NoTargetText(verb)) }
func Cooldown(to Recipient, doing string)      { send(to, CooldownText(doing)) }
func WhileFighting(to Recipient, doing string) { send(to, WhileFightingText(doing)) }
```

`internal/refusal/membership.go`:

```go
package refusal

// NoCompanionLine answers an AI companion command with no companion.
const NoCompanionLine = `You have no companion travelling with you.`

func NotInGuildText() string {
	return `You aren't in a guild.` + hint(`guild create <TAG> <name>`)
}

// NotPartyLeaderText answers a leader-only party command; action completes
// "Only the party leader can ...", empty reads "do that".
func NotPartyLeaderText(action string) string {
	if action == `` {
		action = `do that`
	}
	return `Only the party leader can ` + action + `.`
}

func NotInGuild(to Recipient)                    { send(to, NotInGuildText()) }
func NoCompanion(to Recipient)                   { send(to, NoCompanionLine) }
func NotPartyLeader(to Recipient, action string) { send(to, NotPartyLeaderText(action)) }
```

`internal/refusal/failed.go`:

```go
package refusal

import (
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// FailedText is what a player reads when the server failed them. action is
// a noun for what failed ("salvage"), or empty.
func FailedText(action string) string {
	if action == `` {
		return `That didn't work. The error has been logged.`
	}
	return `That ` + action + ` didn't work. The error has been logged.`
}

// Failed logs err with its cause and logArgs (pass "userId", id), then tells
// the player plainly on CategoryError (spec section 4). It is the one helper
// that does not send on CategoryRefusal: this is the server's fault, not a
// refusal of what the player asked.
func Failed(to Recipient, action string, err error, logArgs ...any) {
	mudlog.Error(`refusal.Failed`, append([]any{`action`, action, `error`, err}, logArgs...)...)
	to.SendText(messaging.CategoryError, FailedText(action))
}
```

- [ ] **Step 4: Run** `go test ./internal/refusal/ -count=1` then `go vet ./internal/refusal/`. Expected: PASS, no vet output.

- [ ] **Step 5: Commit** the five new files and the test with `feat(refusal): not here, carrying, combat, membership and failure families (#371)`.

### Task 4: Not-here sites

**Files:** `internal/actions/sight_aim.go`, `cast_admission.go`, `melee_target.go`, `plant.go`, `steal.go`; `internal/usercommands/` `attack.go`, `pet.go`, `ask.go`, `consider.go`, `give.go`, `party.go`, `report.go`, `show.go`, `skill.skullduggery.shadow.go`, `skill.skullduggery.plant.go`, `skill.skullduggery.steal.go`, `talk.go`, `target.go`, `skill.cast.go`, `get.go`, `loot.go`, `assess.go`; `modules/aicompanion/commands.go`; `modules/follow/follow.go`; and the six test files in fact 14.

- [ ] **Step 1: Retire the actions constants.** In `sight_aim.go` delete `AimNothingLine` and `AimNotHereLine` from the const block (keep `aimShapesLine`, `aimHintWidth`) and replace each use in the file with `refusal.NothingToAimLine` / `refusal.NotHereCreatureLine`; import `github.com/GoMudEngine/GoMud/internal/refusal`. Then `grep -rln "AimNotHereLine\|AimNothingLine" --include=*.go internal modules` lists every consumer (8 files, fact 14); replace `actions.AimNotHereLine` with `refusal.NotHereCreatureLine` and `actions.AimNothingLine` with `refusal.NothingToAimLine` in production and test files (fact 14), adding the import and dropping an `actions` import left unused.

- [ ] **Step 2: Move every AimBySight refusal send to the new category.** At each site in fact 15, change `user.SendText(messaging.CategorySystem, refusal)` (or `actor.`, or the plant/steal `aimRefusal` send) to `messaging.CategoryRefusal`. `attack.go:113` becomes `refusal.NothingToAim(user)`; `pet.go:62` becomes `refusal.NotHereCreature(user)`. Where a local variable is named `refusal`, rename it `aimRefusal` in that function so the package name is not shadowed.

- [ ] **Step 3: Convert the literal lines** (one call each, no other change at the site):

| Site | Becomes |
|---|---|
| `actions/melee_target.go:189`, `usercommands/consider.go:42`, `target.go:81,106,132`, `report.go:76`, `pet.go:68,74` | `refusal.NotHereCreature(user)` |
| `skill.cast.go:246` | `refusal.NotHereCreature(user)` (the red tag goes; the category colours it) |
| `actions/plant.go:351`, `actions/steal.go:534,867` | `refusal.NotHereThing(actor)` |
| `get.go:80`, `get.go:737` (the `else` branch only), `loot.go:49,163`, `assess.go:29` | `refusal.NotHereThing(user)` |

Remove any `fmt` import left unused.

- [ ] **Step 4: Run** `go build ./... && go test ./internal/actions/ ./internal/usercommands/ ./modules/follow/ ./modules/aicompanion/ -count=1`. Expected: PASS. A failure that pins old echoed text is a test asserting the #454 leak: change it to the fixed line and say so in the commit body.

- [ ] **Step 5: Commit** the touched files by name: `refactor(refusal): not-here lines through internal/refusal, no echo of looked-for names (#371, #454)`.

### Task 5: Not-carrying sites

**Files:** every file in fact 20.

- [ ] **Step 1: Convert.** Each line in fact 20 becomes one call, with the typed name named in the fact (default `rest`):

```go
refusal.NotCarrying(user, rest)        // usercommands
refusal.NotCarrying(actor, rest)       // actions/drink.go
refusal.NotCarrying(actor, opts.ItemNoun) // actions/plant.go:85
refusal.NotCarrying(seller, itemName)  // actions/sell.go:180, inside its IsPlayer check
refusal.NotCarrying(user, "")          // put.go:94
```

`give.go:67` loses its usage tail (`give.go` still prints usage when the command is malformed). `storage.go:154,186` lose `term.CRLFStr`; drop the import if unused. `modules/cleanup/cleanup.go:71` moves from `CategoryError` to the helper's category: it is a refusal, not a server failure.

- [ ] **Step 2: Run** `go build ./... && go test ./internal/actions/ ./internal/usercommands/ ./modules/... -count=1`. Expected: PASS (fact 30: no test pins these lines; `storage_remove_slot_test.go` checks `NotContains "You don't have a"`, still true).

- [ ] **Step 3: Commit** by name: `refactor(refusal): not-carrying lines echo the name and point at inventory (#371)`.

### Task 6: No target and cooldown

**Files:** fact 22 and 23 files, `internal/actions/consider_test.go`, `internal/usercommands/attack_ambush_denial_test.go`.

- [ ] **Step 1: No target.** In each fact 22 file, `user.SendText(messaging.CategorySystem, "You have no target!")` becomes `refusal.NoTarget(user, "<verb>")` with the file's command: `drain`, `gore`, `grapple`, `kick`, `maul`, `pounce`, `rake`, `taunt`, `throttle`, `trip`.

- [ ] **Step 2: Cooldown.** In each special-move file, the cooldown line becomes `refusal.Cooldown(user, "<verb>")` with the file's command (`bash`, `drain`, `gore`, `grapple`, `kick`, `maul`, `pounce`, `rake`, `rally`, `taunt`, `throttle`, `throw`, `trip`, `warcry`). `mutation_helpers.go:99` becomes `refusal.Cooldown(actor, "use that ability")`. In `consider.go` delete `considerCooldownText` and send `refusal.Cooldown(actor, "consider anyone")`; in `consider_test.go:123` change the probe to `"before you can consider anyone"`.

- [ ] **Step 3: Ambush.** `attack.go:443` moves out of the `const` block into a `var`:

```go
// surpriseMeleeDeniedText is the refusal a melee ambusher sees when the
// shared special-move timer was already claimed (comment above kept).
var surpriseMeleeDeniedText = `No ambush ready. ` + refusal.CooldownText(`ambush`)
```

It stays on `CategorySurpriseAttack` at `:472` (slice 3 decides the combat-category refusals). In `attack_ambush_denial_test.go:281-283` replace the pinned literal with `No ambush ready. You need a moment to recover before you can ambush.` and the message with "the melee refusal uses the shared cooldown wording".

- [ ] **Step 4: Run** `go build ./... && go test ./internal/actions/ ./internal/usercommands/ -count=1`. Expected: PASS.

- [ ] **Step 5: Commit** by name: `refactor(refusal): no-target and cooldown lines name the move (#371)`.

### Task 7: Membership and while fighting

- [ ] **Step 1:** `guild.go` (fact 24, all 15 including 93) becomes `refusal.NotInGuild(user)`. `aicompanion/commands.go` (fact 25) becomes `refusal.NoCompanion(user)`. `party.go`: `:176` `refusal.NotPartyLeader(user, "invite others")`, `:560` `"disband the party"`, `:593` `"remove members"`, `:624` `"promote a member"`, `:657` `"set the loot mode"`.

- [ ] **Step 2:** fact 27: `defuse.go:71` `refusal.WhileFighting(actor, "defuse a trap")`, `plant.go:54` `"plant anything"`, `shadow.go:69` `"shadow anyone"`, `steal.go:103` `"steal"`, `skill.skullduggery.sneak.go:60` `refusal.WhileFighting(user, "sneak")`.

- [ ] **Step 3: Run** `go build ./... && go test ./internal/actions/ ./internal/usercommands/ ./modules/aicompanion/ -count=1`. Expected: PASS.

- [ ] **Step 4: Commit** by name: `refactor(refusal): guild, companion, party leader and in-combat lines (#371)`.

### Task 8: Failures

- [ ] **Step 1:** Each fact 28 line becomes `refusal.Failed(user, "", <err>, "userId", user.UserId)`:

| Site | `<err>` |
|---|---|
| `give.go:108,218` | `result.Err` |
| `give.go:156,319` | `err` |
| `give.go:130` | `errors.New("give to a player: neither an item nor gold")` |
| `give.go:276` | `errors.New("give to a mob: neither an item nor gold")` |
| `party.go:160` | `errors.New("party create failed")` |
| `party.go:219` | `errors.New("party invite: no party after create")` |
| `party.go:237` | `errors.New("party invite failed")` |
| `party.go:269` | `errors.New("party accept failed")` |
| `party.go:285` | `errors.New("party decline failed")` |
| `show.go:61` | `errors.New("show: found item has no id")` |
| `character.go:273` | `errors.New("SwapToAlt failed")` (keep `user.ClearPrompt()` after it) |

`actions/salvage.go:330` becomes `refusal.Failed(actor, "salvage", fmt.Errorf("no item spec for item %d", itemId), "userId", actor.GetUserId())` inside the existing `IsPlayer` check.

- [ ] **Step 2:** `admin.room.go:55`: `fmt.Errorf("room %d did not load from its template", liveRoom.RoomId)` (admin output; slice 4 moves it to `CategoryAdmin`).

- [ ] **Step 3:** `behaviortree/actions_mob.go:511` becomes `mob.Command("say The planes resist me. Your gold is returned.")`; `:523` becomes `mob.Command("say The way will not open. Your gold is returned.")` with `mudlog.Error("actOpenInstancePortal", "zone", templateZone, "userId", user.UserId, "error", "room did not load", "roomId", ctx.RoomId)` before it.

- [ ] **Step 4: Run** `go build ./... && go test ./internal/usercommands/ ./internal/actions/ ./internal/behaviortree/ -count=1`. Expected: PASS.

- [ ] **Step 5: Commit** by name: `refactor(refusal): server failures log their cause and say so plainly (#371)`.

### Task 9: No echo through sight

**Files:** Create `internal/usercommands/refusal_echo_sight_test.go`.

- [ ] **Step 1: Write the test:**

```go
package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/stretchr/testify/assert"
)

// M7 spec section 2 (#454): a creature that is there but unseen and a name
// that matches nothing read the same, and neither echoes what was typed.
// Each command is driven in the dark at the Skeleton (there) and at a name
// that matches nothing, and under full light at the name that matches nothing.
func TestRefusalNeverEchoesThroughSight(t *testing.T) {
	cmds := map[string]func(string) string{
		"target": func(rest string) string {
			user, room := aimScene(t, aimDark)
			_, _ = Target(rest, user, room, events.EventFlag(0))
			return aimTold(1)
		},
		"consider": func(rest string) string {
			user, room := aimScene(t, aimDark)
			_, _ = Consider(rest, user, room, events.EventFlag(0))
			return aimTold(1)
		},
	}
	lit := map[string]func(string) string{
		"target": func(rest string) string {
			user, room := aimScene(t, aimFull)
			_, _ = Target(rest, user, room, events.EventFlag(0))
			return aimTold(1)
		},
		"consider": func(rest string) string {
			user, room := aimScene(t, aimFull)
			_, _ = Consider(rest, user, room, events.EventFlag(0))
			return aimTold(1)
		},
	}
	for name, run := range cmds {
		there, nothing := run("skeleton"), run("zzyzx")
		assert.Equal(t, there, nothing, "%s: an unseen creature and no match must read the same", name)
		assert.Contains(t, there, refusal.NotHereCreatureLine, name)
		assert.NotContains(t, strings.ToLower(there), "skeleton", name)
		litNothing := lit[name]("zzyzx")
		assert.Contains(t, litNothing, refusal.NotHereCreatureLine, "%s in light", name)
		assert.NotContains(t, litNothing, "zzyzx", "%s echoed a name the player looked for", name)
	}
}
```

If `Target` needs a current combat target to reach its lookup (as `TestTargetSight_NoSightResolvesNothing` sets up with `targeting.Commit`), add that setup inside both `target` closures, copied from that test.

- [ ] **Step 2: Prove it can fail.** Temporarily change `target.go:81` back to `user.SendText(messaging.CategorySystem, fmt.Sprintf("You don't see '%s' here.", rest))`, run `go test ./internal/usercommands/ -run TestRefusalNeverEchoesThroughSight -count=1`, expect FAIL on "echoed a name", then revert that one line.

- [ ] **Step 3: Run** the test again. Expected: PASS.

- [ ] **Step 4: Commit** the test file: `test(refusal): sight-gated refusals never echo (#371, #454)`.

### Task 10: The phrase guard

**Files:** Create `refusal_phrase_guard_test.go` (repo root, `package main`).

- [ ] **Step 1: Write the guard:**

```go
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// Messaging M7 (#371): a refusal lives in internal/refusal. A Go string
// literal anywhere else in internal/ or modules/ that carries one of these
// phrases is a copy the helpers replaced. A hit is converted to the helper,
// or, if it is not that refusal, the phrase here is narrowed; never
// allow-listed by file. Each phrase's probe proves the pattern can match.
// Later slices add rows.
var refusalPhrases = []struct {
	family string
	re     *regexp.Regexp
	probe  string
}{
	{"not here", regexp.MustCompile(`You don't see (them|that|those [a-z]+) here`), "You don't see those remains here."},
	{"not here, echoed", regexp.MustCompile(`You don't see ['"]?%s['"]? here`), `You don't see "%s" here.`},
	{"not here, floor", regexp.MustCompile(`You don't see (any|a|an) "?%s"? (to pick up|to loot|around)`), "You don't see a %s around."},
	{"not here, verb", regexp.MustCompile(`Can't find that to `), "Can't find that to pet."},
	{"not carrying", regexp.MustCompile(`You don't have (a|an|any) "?%s"? (to |that can )`), `You don't have a "%s" to drink.`},
	{"not carrying, bare", regexp.MustCompile(`You don't have (that|that item)\.`), "You don't have that item."},
	{"not carrying, inventory", regexp.MustCompile(`You don't have "%s" in your inventory`), `You don't have "%s" in your inventory.`},
	{"not carrying, seem", regexp.MustCompile(`You don't seem to be carrying`), "You don't seem to be carrying that."},
	{"no target", regexp.MustCompile(`You have no target`), "You have no target!"},
	{"cooldown", regexp.MustCompile(`moment to recover before`), "You need a moment to recover before attempting another special move."},
	{"guild", regexp.MustCompile(`You are not in a guild`), "You are not in a guild."},
	{"companion", regexp.MustCompile(`no companion travelling with you`), "You have no companion travelling with you."},
	{"party leader", regexp.MustCompile(`not the leader of your party|Only the party leader can`), "You are not the leader of your party."},
	{"in combat", regexp.MustCompile(`can't do that while in combat`), "You can't do that while in combat!"},
	{"failure", regexp.MustCompile(`Something went wrong`), "Something went wrong."},
}

// refusalPhraseHits reports every literal in src that carries a phrase.
func refusalPhraseHits(t *testing.T, rel string, src []byte) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, src, 0)
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	var hits []string
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			s = lit.Value
		}
		for _, p := range refusalPhrases {
			if p.re.MatchString(s) {
				hits = append(hits, rel+":"+strconv.Itoa(fset.Position(lit.Pos()).Line)+": "+p.family+": "+s)
			}
		}
		return true
	})
	return hits
}

func TestRefusalPhrasesLiveInThePackage(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Dir(here)
	scanned := 0
	for _, top := range []string{"internal", "modules"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if strings.HasPrefix(rel, "internal/refusal/") {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanned++
			for _, h := range refusalPhraseHits(t, rel, src) {
				t.Errorf("%s (use internal/refusal)", h)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned < 100 {
		t.Fatalf("scanned only %d Go files; the walk is broken", scanned)
	}
}

// TestRefusalPhraseGuardCanFail runs the scanner over a source file holding
// every probe and requires one hit per phrase.
func TestRefusalPhraseGuardCanFail(t *testing.T) {
	var b strings.Builder
	b.WriteString("package probe\n\nvar probes = []string{\n")
	for _, p := range refusalPhrases {
		if !p.re.MatchString(p.probe) {
			t.Errorf("%s: probe %q does not match its own pattern", p.family, p.probe)
		}
		b.WriteString("\t" + strconv.Quote(p.probe) + ",\n")
	}
	b.WriteString("}\n")
	hits := refusalPhraseHits(t, "probe.go", []byte(b.String()))
	if len(hits) < len(refusalPhrases) {
		t.Errorf("scanner found %d hits for %d probes:\n%s", len(hits), len(refusalPhrases), strings.Join(hits, "\n"))
	}
}
```

- [ ] **Step 2: Run** `go test -run 'TestRefusalPhrase' -count=1 .`. Expected: PASS. Any hit is a site Tasks 4 to 8 missed: convert it (or, if the line is not that refusal, narrow the phrase and record why in the commit body), never skip the file.

- [ ] **Step 3: Probe the walk.** Temporarily put `user.SendText(messaging.CategorySystem, "You have no target!")` back at `kick.go:47`; run; expect FAIL naming `internal/usercommands/kick.go`; revert that line.

- [ ] **Step 4: Commit** `refusal_phrase_guard_test.go`: `test: refusal phrase guard with probes (#371)`.

### Task 11: Docs

- [ ] **Step 1:** Create `internal/refusal/context.md`: purpose (spec section 1), the `Recipient` interface, the echo rule (sight-gated helpers take no name; `echo` strips, trims, caps at 24 runes, escapes), hints in the command colour, one table of helpers with their lines (copy the Wording table), `Failed` on `CategoryError` with its log, the phrase guard and how to add a row, and what is left for slices 2 to 4. Verify every symbol named with `Select-String -Path internal\refusal\*.go -Pattern '^(func|type|const|var)\s'`.
- [ ] **Step 2:** `internal/messaging/context.md`: add a `CategoryRefusal` paragraph after the `CategoryCraftSeen` one (appended value, treated as System, alias `refusal: 8`, carries every `internal/refusal` line) and change the line near `:533` that says refusals stay on `CategorySystem` to say they moved to `CategoryRefusal` in M7. `internal/actions/context.md:693`: the sight refusals are now `refusal.NothingToAimLine` and `refusal.NotHereCreatureLine`.
- [ ] **Step 3:** `docs/README.md`: add a row for `../internal/refusal/context.md` beside the `../tools/context_md_audit.py` row (CLAUDE.md: new files go in the index).
- [ ] **Step 4:** `docs/PATCH_NOTES.md`: dated entry, player-facing, no numbers: "Refusals read the same everywhere. When you name something you aren't carrying, the game repeats what you typed and points you at your inventory. Special moves name what you're waiting to do. Targeting a creature you can't see no longer repeats the name you typed."
- [ ] **Step 5:** `python tools/context_md_audit.py` reports nothing for `internal/refusal`, `internal/messaging`, `internal/actions`. Commit the docs by name: `docs: M7 slice 1 refusal package (#371)`.

### Task 12: Gate, boot, playtest, PR

- [ ] **Step 1:** The `dogmud-shipping` pre-push gate in order: `gofmt -l internal/ modules/` prints nothing; `go build ./...`; `go test ./internal/refusal/ ./internal/messaging/ ./internal/actions/ ./internal/usercommands/ ./internal/behaviortree/ ./modules/... -count=1`; `go test -count=1 .` (root guards); `golangci-lint run --new-from-merge-base=origin/master` reports `0 issues`.
- [ ] **Step 2:** Boot check per `dogmud-shipping` (detached worktree, private ports, `Server Ready` 1, panics 0).
- [ ] **Step 3:** Playtest per `dogmud-playtesting` (spec Testing): in light and dark, `target`, `consider` and `kick` a missing name and an unseen creature; `drop`, `eat`, `give` something not carried; two special moves back to back; `guild info` outside a guild; `party disband` as a non-leader. Every line reads as the Wording table, none echoes a looked-for name, none passes 80 columns. File findings as issues under #371.
- [ ] **Step 4:** Push and open the PR with `--repo pruuk/DOGMud`, body naming slice 1 of #371 (no closing keyword: #371 stays open for slices 2 to 4), the Wording table, and the test and playtest evidence.

## Self-review notes

- Spec coverage for slice 1: package and `context.md` (Tasks 2, 3, 11), `CategoryRefusal` (1), echo rule and its test (2, 9), hints (2, 3), families with most repeats (3 to 7), `Failed` and the `actions_mob.go` check (8: both reach a player as mob speech, so they are reworded, not turned into logs), phrase guard with probes (10), boot and playtest (12). Deferred by the spec's slicing: `Prompt`, `WhileBusy`, `TargetAsleep`, `Cost`, `NotWearing` (no slice 1 site), `CategoryUsage`, `CategoryAdmin`, narration out of System, the tag guard.
- Departure from the spec's helper names: `NotHereCreature` and siblings have `...Line` constants instead of `...Text` functions, because `AimBySight` returns its line rather than sending it.
- Hint style ruled 2026-10-15 (owner): the command colour, as the codebase already writes hints (fact 29). `TestHintNamesACommand` pins it.
