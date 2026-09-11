# Messaging M3 Item 5a: Narration Defects Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix six live narration defects in buff, spell and quest text on the paths as they exist today, without moving any store onto the narration core.

**Architecture:** Every change stays on its existing delivery path. Buff and quest room lines move from `Room.SendText` (audio, never sight-gated) to the visual channel. The mob buff tick learns to send trigger text. Four spell cases gain the self-cast shape that `case "shield"` already has. Quest `room_text` gains token substitution, a `{source}` rewrite of 21 authored lines, and a startup check that fails the boot on a line that breaks the convention. One data line is also fixed so the playtest can stage darkness at all: the Cat's Eye Draught's night-vision flag is misspelled and grants nothing.

**Tech Stack:** Go 1.25, testify, `gopkg.in/yaml.v2`, the hooks test fixture `seedAllRegistries`, `events.DrainQueuedMessagesForTest`, the `playtestrun` harness.

**Spec:** [`docs/superpowers/specs/2026-09-11-messaging-m3-item5a-narration-defects-design.md`](../specs/2026-09-11-messaging-m3-item5a-narration-defects-design.md)

---

## 🔴 Read this before Task 1

1. **Every test is seen RED for the right reason before the code changes.** A test that passes before its fix proves nothing. Where a task has a sabotage probe, the sabotage must COMPILE (`go vet` clean) before its red run counts.
2. **The goldens do not move.** 5a changes no message store's content. `git diff --stat master -- internal/narration/testdata/stores/` must be empty at the end. Never run `-update`.
3. **Delivered lines keep their ANSI tags.** A name arrives as `<ansi fg="username">Aliceia</ansi>'s`, so `"Aliceia's"` never matches raw text. Assert through the `plainText` / `drainPlain` helpers from Task 1.
4. **`countContaining` ALREADY EXISTS** in `internal/hooks/combat_verbosity_wiring_test.go:81`, and it is **case-insensitive**. Reuse it. Defining it again is a `redeclared in this block` build failure.
5. **Fixture room 1 is LIT; room 2 is NOT dark either.** Room 1 is `biome: city`. Room 2 has no biome, which resolves to the lit default. To darken a room in a test, set `room.Biome = "cave"`: the fixture seeds `cave` with `DarkArea: true`, and a dark biome is visibility 0 at any hour of game time. The `darken` helper does this and asserts it.
6. **`buffs.SeedBuffsForTest` REPLACES the whole buff registry.** Seed with `defer restore()` AFTER `defer cleanup()`, so the restore runs first (defers are LIFO). Never use `t.Cleanup` for it: that runs after the fixture cleanup and would reinstall a stale registry.
7. **Task order is load-bearing.** The startup check (Task 5) exists before the data rewrite (Task 6), and is wired into boot only inside Task 6, AFTER the rewrite. Wired earlier, boot and `hooks/Quest_HandleQuestUpdate_bridge_test.go:76` (which calls `questengine.LoadDataFiles()`) both fail on today's data.
8. **Every task gate also runs `go test .`** (added in execution). The root package holds `TestNarrationSitesMatchViewpointAudit`, which fails on any new narration format string that misses a viewpoint without a registry row. Task 1's hooks-only gate missed it and left `go test ./...` red.
9. **Git:** named paths only, never `git add -A` or `git add .`. Every `gh` command carries `--repo pruuk/DOGMud`. `grep -c` exits 1 on zero matches, so run every "expect zero" check standalone.

## Facts this plan relies on, verified 2026-09-11 against master `e1e0fed74`

| Fact | Evidence |
|---|---|
| Fixture: users 1 `Aliceia` and 2 `Bobrick` in room 1 and registered as occupants; mob instance 100 `Skeleton` in room 1 with `Buffs: buffs.New()`; buffs 100 and 101 seeded | `internal/hooks/hooks_test.go:37-149` |
| Fixture biomes: `city` (lit), `cave` (`DarkArea: true`), `default` (lit) | `hooks_test.go:152-175` |
| `countContaining(texts []string, needle string) int`, case-insensitive, already defined | `internal/hooks/combat_verbosity_wiring_test.go:80-90` |
| `GetVisibility`: starts at 2, minus 1 at night, minus 2 for a dark biome (floored at 0), plus 1 for a lit biome (capped at 2); sight needs at least 1 | `internal/rooms/rooms.go:144-175`; `internal/messaging/predicates.go` `roomIsLit` |
| `CanSeeClearly`: false if blinded or sleeping, true in a lit room, else `HasFlagFromAnySource(NightVision)`; `CanSeeShapes` adds `InfraredVision` | `predicates.go:24-109` |
| `HasFlagFromAnySource` reads `Buffs.HasFlag(flag, false)`, then mutations | `internal/characters/buffs.go:27-32` |
| `buffs.NightVision` is the string `nightvision`; `buffs.InfraredVision` is `infraredvision` | `internal/buffs/buffspec.go:62-63` |
| `Room.SendTextVisual` renders per recipient and queues an `events.Message`; `DrainQueuedMessagesForTest(userId)` drains exactly those | `rooms.go:307-337`; `internal/events/events.go:338` |
| `messaging.Anonymize` swaps a tagged name for `<ansi fg="combat-anon">a figure</ansi>` | `internal/messaging/anonymize.go:12-23` |
| `SeedBuffsForTest(map[int]*BuffSpec) func()` swaps the map and returns a restore | `internal/buffs/test_helpers.go:6` |
| `Buffs.AddBuff(buffId int, isPermanent bool) bool` sets `TriggersLeft = TriggerCount` and indexes the spec's flags | `internal/buffs/buffs.go:220-258` |
| `Buffs.Trigger` skips any spec with `RoundInterval < 1`, increments `RoundCounter`, fires when `RoundCounter % RoundInterval == 0` | `buffs.go:260-300` |
| `Buffs.Prune` removes a buff whose `Expired()` (`TriggersLeft <= TriggersLeftExpired`, which is 0) | `buffs.go:10`, `:35-36`, `:331-360` |
| `ApplyBuffs(events.Buff{UserId, MobInstanceId, BuffId})` sends start text only on a first application | `internal/hooks/Buff_ApplyBuffs.go:21-112`; `internal/events/eventtypes.go:20-25` |
| `UserRoundTick` walks rooms with players, then every player, with no gate before the buff trigger block; the trigger block skips an `Expired()` buff | `internal/hooks/NewRound_UserRoundTick.go:148-294` |
| `PruneBuffs` walks rooms with players, `GetPlayers(rooms.FindBuffed)` (a player with `len(Buffs.List) > 0`), then every mob instance | `internal/hooks/NewTurn_PruneBuffs.go:18-121`; `rooms.go:1720` |
| `tickMobBuffs(mob *mobs.Mob, mobInstanceId int)` never sends trigger text | `internal/hooks/NewRound_MobRoundTick.go:218-261` |
| `mobDisplayName(mob, room, viewingUserId)` returns `GetMobNameIndexed(...).String()`, which renders `<ansi fg="mobname...">`; **CORRECTED in execution:** `GetCharacterName(true)` does NOT always render `username`. It renders `username-aggro` for any character not fighting a player (viewer 0 matches an empty combat target) and `username-dead` when dead; `messaging.Anonymize` accepted neither until `7399e914b` | `internal/hooks/NewRound_DoCombat_helpers.go:390`; `internal/characters/formattedname.go:50-72`, `:151-170` |
| `applyPlayerEffect(user, target *users.UserRecord, room *rooms.Room, spellData *spells.SpellData, magnitude int, out combat.ChannelDefenceResult)`; `spellContestAttackWin()` is `{DamageMultiplier: 1}` | `internal/hooks/spell_resolution.go:949`; `internal/hooks/spell_collapse_test.go:35` |
| `resolveSpell` fills `HelpArea` targets from every room player, caster included; a help spell with no `TargetDefenseType` is applied uncontested | `spell_resolution.go:108-123`, `:153-178` |
| `textutil.TokenContext{SourceName, SourcePlainName, TargetName, TargetPlainName}`; `SubstituteTokens`; `ValidateTokens(text) []string` returns `"unknown token: {x}"` for anything outside the four known tokens | `internal/textutil/tokens.go:9-56` |
| `questengine.QuestDef = quests.Quest` (`QuestId int`, `Triggers`); `TriggerDef{Event, Room, Noun, Verb, Conditions, Actions}`; `ActionDef.RoomText` | `internal/questengine/types.go`; `internal/quests/quests.go:57-63`; `internal/quests/triggers.go:11-49` |
| `RegisterQuest` needs only `QuestId` and `Triggers` | `internal/questengine/engine.go:36-47` |
| Quest YAML and buff YAML load with `gopkg.in/yaml.v2`; `BuffSpec.BuffId` has no tag, so it reads `buffid` | `internal/fileloader/fileloader.go:18`; `buffspec.go:99` |
| 22 quest `room_text` lines ship; 21 lack `{source}`, quest 77's has it | `grep room_text: _datafiles/world/dogmud/quests/` |
| Bridge tests that load real quest data are all named `TestHandleQuestUpdate_*` | `internal/hooks/Quest_HandleQuestUpdate_bridge_test.go:98`, `:131`, `:176` |
| 🐛 **The Cat's Eye Draught grants NO night vision.** Buff 65 lists `night-vision`; the constant is `nightvision`; nothing normalises buff flags and `BuffSpec.Validate` checks only tokens and tick fields. It is the only DOGMud buff naming night vision | `_datafiles/world/dogmud/buffs/65-cats_eye_draught.yaml:9`; `buffspec.go:207-240` |
| Of 18 distinct flags in shipped buffs, 2 match no declared constant: `night-vision` (buff 65) and `poison-immunity` (buff 64 Stone Stomach, which no Go code reads) | comparison against the 32 constants in `buffspec.go:30-89` |
| Playtest content: spell `heal` is named **Mend Flesh**; `chrysalis-glow` applies buff 1 Illumination (`start_room_text: "A warm glow surrounds {source_plain}."`); `conviction-armor` applies buff 38 (`"{source} is encased in a shimmering layer of conviction."`); `cleansing-wave` is `helparea` purge. None has a `target_defense_type` | `_datafiles/world/dogmud/spells/*.yaml`; `buffs/1-illumination.yaml`; `buffs/38-conviction_armor.yaml` |
| `veteran` knows chrysalis-glow, conviction-armor, cleansing-wave; `m2-witness` knows heal and has no night vision | `tools/playtest/profiles/veteran.yaml`; `m2-witness.yaml` |
| Room 5343 The Reliquary is `fort` (lit); 6397 The Records Archive and 3101 Cave Mouth are `cave` | room YAML; `_datafiles/world/dogmud/biomes/fort.yaml` |
| Quest 76: `room_interact`, room 5343, noun `disc`, `missing_item: 40167`. Quest 77: room 6397, noun `records`, `missing: ["77-end"]` | `quests/76-the_disc.yaml:41-54`; `quests/77-the_truth.yaml:23-28` |
| `playtestrun scenario --checkout <abs> --scenario <path> [--wall-clock]`; `tools/playtest/cmd/agentbridge` exists | `cmd/playtestrun/main.go:99-103` |

## Corrections to the spec found while planning

- **The Cat's Eye Draught does not work.** The spec's darkness lanes rely on the witness drinking it to gain night vision. Its buff misspells the flag, so it grants nothing. This plan fixes the one line (Task 7) and guards it with a test, because the lanes cannot run otherwise and the item's own description promises the effect. **Owner, flag this if you would rather it ship separately.** Validating buff flag names at load, which would also catch buff 64's dead `poison-immunity`, is filed, not done.
- **The D1 playtest actor.** The spec names `m2-actor` casting Conviction Surge. That buff (26) has no `start_room_text`, so there is nothing for darkness to silence. This plan uses **`veteran`**, who knows Chrysalis Glow (buff 1) and Conviction Armor (buff 38), both with room lines, plus Cleansing Wave. The witness stays **`m2-witness`**.
- **Three short runs, not one.** The lanes live in rooms 5343, 6397 and 3101, which are far apart. Separate runs also give fresh characters, which the once-per-character quest triggers need.
- **The dark quest lane alternates who triggers.** Quest 77's trigger fires once per character. The witness triggers it first while the actor is unsighted (D2), then drinks the draught and watches the actor trigger it (D5).
- **Casting at a player you cannot see: checked, not relied on.** Owner, 2026-09-11: check whether it works; if it does, the fix is a follow-up. The code says it does: `actions.InitiateCast` resolves a named `HelpSingle` or `HarmSingle` target with `room.FindByName`, which calls `GetPlayers(FindAll)`, and nothing on that path consults sight or room lighting (`internal/actions/cast.go:108`, `:230`; `internal/rooms/rooms.go:1864-1950`, `:1651-1661`). Lane C confirms it in play with a dedicated step. No other lane step depends on the answer: the control uses Cleansing Wave (an area spell) and the sighted step uses a self-cast.
- **Self-cast lines keep the crit tag.** The spec's wording table omits `critTag`. Each self line appends it, so a crit on yourself is not lost. It is empty on an ordinary cast.

## File Structure

| File | Responsibility |
|---|---|
| `internal/hooks/narration_testhelpers_test.go` | **New.** Tag stripping, draining, buff seeding and room darkening for the hooks tests in this plan |
| `internal/hooks/selfcast_wording_test.go` | **New.** D3 tests |
| `internal/hooks/hooks_test.go` | **Modify.** Delete the three assertion-free self-cast tests |
| `internal/hooks/spell_resolution.go` | **Modify.** D3 self-cast shape in `purge`, `heal`, `buff`, `default:` |
| `internal/hooks/buff_room_text_test.go` | **New.** D1 and D4 tests |
| `internal/hooks/Buff_ApplyBuffs.go` | **Modify.** D1: visual send, mob tag |
| `internal/hooks/NewRound_UserRoundTick.go` | **Modify.** D1: visual send |
| `internal/hooks/NewTurn_PruneBuffs.go` | **Modify.** D1: visual send for player and mob; mob tag |
| `internal/hooks/NewRound_MobRoundTick.go` | **Modify.** D4: mob trigger text |
| `internal/hooks/quest_room_text_test.go` | **New.** D2 and D5 tests |
| `internal/questengine/bridge.go` | **Modify.** D2 and D5: substitute, send visual |
| `internal/questengine/loader.go` | **Modify.** `ValidateAllRoomText`, `roomTextProblems`, wired into `LoadDataFiles` |
| `internal/questengine/roomtext_validation_test.go` | **New.** Rule tests, panic tests, shipped-data test |
| 13 files under `_datafiles/world/dogmud/quests/` | **Modify.** D6: 21 lines gain `{source} ` |
| `internal/buffs/shipped_flags_test.go` | **New.** The draught grants night vision |
| `_datafiles/world/dogmud/buffs/65-cats_eye_draught.yaml` | **Modify.** `night-vision` becomes `nightvision` |
| `tools/playtest/profiles/m2-witness.yaml` | **Modify.** Carry one Cat's Eye Draught |
| `tools/playtest/scenarios/m3-item5a-{lit,quest-dark,buff-dark}.yaml` | **New.** Playtest scenarios |
| `tools/playtest/goals/scenarios/m3-item5a-{lit,quest-dark,buff-dark}/{actor,witness}.yaml` | **New.** Six goals files |
| `internal/questengine/context.md` | **Modify.** `RoomText` behaviour and the startup check |
| `docs/PATCH_NOTES.md` | **Modify.** Player-facing entry |

---

## Task 1: Test helpers, then D3 self-cast wording

**Files:**
- Create: `internal/hooks/narration_testhelpers_test.go`
- Create: `internal/hooks/selfcast_wording_test.go`
- Modify: `internal/hooks/hooks_test.go` (delete three functions)
- Modify: `internal/hooks/spell_resolution.go:1001-1016`, `:1033-1046`, `:1075-1091`, `:1121-1124`

- [ ] **Step 1: Write the shared test helpers**

Create `internal/hooks/narration_testhelpers_test.go`:

```go
package hooks

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// Counting lines uses countContaining, which already lives in
// combat_verbosity_wiring_test.go and matches case-insensitively.

// narrationTagPattern matches one ANSI tag. Delivered lines KEEP their tags, so
// a name arrives as `<ansi fg="username">Aliceia</ansi>'s` and a plain
// substring such as "Aliceia's" never matches raw text.
var narrationTagPattern = regexp.MustCompile(`<[^>]*>`)

// plainText strips ANSI tags and surrounding whitespace from a delivered line.
func plainText(line string) string {
	return strings.TrimSpace(narrationTagPattern.ReplaceAllString(line, ""))
}

// drainPlain drains a user's queued messages and returns them tag-stripped.
func drainPlain(userId int) []string {
	raw := events.DrainQueuedMessagesForTest(userId)
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		out = append(out, plainText(line))
	}
	return out
}

// Buff ids for narration tests. Chosen well clear of the fixture's 100 and 101.
const (
	glowBuffId      = 7001 // start_room_text
	shiverBuffId    = 7002 // trigger_room_text, fires every round
	fadeBuffId      = 7003 // end_room_text
	nightEyesBuffId = 7004 // grants NightVision; RoundInterval 0, so it never ticks
	heatEyesBuffId  = 7005 // grants InfraredVision; RoundInterval 0, so it never ticks
)

// seedNarrationBuffs installs the narration test buffs and returns the restore
// func. Call it AFTER `defer cleanup()` and `defer` its result, so it restores
// before the fixture does: SeedBuffsForTest replaces the whole registry.
func seedNarrationBuffs() func() {
	return buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		glowBuffId: {BuffId: glowBuffId, Name: "Test Glow", RoundInterval: 5, TriggerCount: 3,
			StartRoomText: "{source} glows."},
		shiverBuffId: {BuffId: shiverBuffId, Name: "Test Shiver", RoundInterval: 1, TriggerCount: 3,
			TriggerRoomText: "{source} shivers."},
		fadeBuffId: {BuffId: fadeBuffId, Name: "Test Fade", RoundInterval: 5, TriggerCount: 3,
			EndRoomText: "{source} fades."},
		nightEyesBuffId: {BuffId: nightEyesBuffId, Name: "Test Night Eyes",
			Flags: []buffs.Flag{buffs.NightVision}},
		heatEyesBuffId: {BuffId: heatEyesBuffId, Name: "Test Heat Eyes",
			Flags: []buffs.Flag{buffs.InfraredVision}},
	})
}

// darken turns a fixture room into an unlit cave. The fixture seeds `cave` as
// DarkArea, and GetVisibility reads the biome registry, so setting the field is
// enough. Asserts the room really is unlit, so a lane cannot pass by accident
// in a lit room.
func darken(t *testing.T, roomId int) {
	t.Helper()
	room := rooms.LoadRoom(roomId)
	require.NotNil(t, room)
	room.Biome = "cave"
	require.Zero(t, room.GetVisibility(), "room %d must actually be unlit", roomId)
}
```

- [ ] **Step 2: Confirm the helpers compile with no name collision**

Run: `go vet ./internal/hooks/`
Expected: no output. A `redeclared in this block` error means an existing test already defines one of these names; rename the new one.

- [ ] **Step 3: Write the failing D3 tests**

Create `internal/hooks/selfcast_wording_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
)

// These replace TestApplyPlayerEffect_PurgeSelf, _HealSelf and _BuffSelf,
// which called applyPlayerEffect(u, u, ...) and asserted NOTHING, so they could
// never have caught that a self-caster was told about themselves twice and in
// the third person, or that the room read "Aliceia's Heal envelops Aliceia".
//
// A help spell with no target is a self-cast by default (actions/cast.go:227),
// and an area spell puts the caster in its own target list, so this is the
// common path, not an edge case.

func TestSelfCastPurge_OneLineToCaster_RoomNamesCasterOnce(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	u := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	drainPlain(1)
	drainPlain(2)

	u.Character.AddCondition(characters.ConditionPoisoned, 10, 5.0, "test")
	spell := &spells.SpellData{SpellId: "cleansing-wave", Name: "Cleansing Wave", EffectType: "purge"}
	applyPlayerEffect(u, u, room, spell, 10, spellContestAttackWin())

	caster, observer := drainPlain(1), drainPlain(2)
	assert.Equal(t, 1, countContaining(caster, "You purge the afflictions from your body."))
	assert.Equal(t, 0, countContaining(caster, "cleanses Aliceia"),
		"a self-caster must not be told about themselves in the third person")
	assert.Equal(t, 1, countContaining(observer, "Cleansing Wave cleanses Aliceia of afflictions."))
	assert.Equal(t, 0, countContaining(observer, "Aliceia's Cleansing Wave cleanses Aliceia"))
}

func TestSelfCastHeal_OneLineToCaster_RoomNamesCasterOnce(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	u := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	drainPlain(1)
	drainPlain(2)

	spell := &spells.SpellData{SpellId: "heal", Name: "Heal", EffectType: "heal", EffectMagnitude: 3}
	applyPlayerEffect(u, u, room, spell, 3, spellContestAttackWin())

	caster, observer := drainPlain(1), drainPlain(2)
	assert.Equal(t, 1, countContaining(caster, "A warm glow of healing magic envelops you."))
	assert.Equal(t, 0, countContaining(caster, "restorative magic around Aliceia"))
	assert.Equal(t, 1, countContaining(observer, "Aliceia channels restorative magic."))
	assert.Equal(t, 0, countContaining(observer, "envelops Aliceia in healing light"))
}

func TestSelfCastBuff_OneLineToCaster_RoomNamesCasterOnce(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	u := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	drainPlain(1)
	drainPlain(2)

	spell := &spells.SpellData{SpellId: "bless", Name: "Bless", EffectType: "buff", BuffIds: []int{100}}
	applyPlayerEffect(u, u, room, spell, 0, spellContestAttackWin())

	caster, observer := drainPlain(1), drainPlain(2)
	assert.Equal(t, 1, countContaining(caster, "Your Bless takes effect."))
	assert.Equal(t, 0, countContaining(caster, "takes effect on Aliceia"))
	// "Bless settles over Aliceia." is a substring of the old broken line, so
	// the second assertion is the one that proves the fix.
	assert.Equal(t, 1, countContaining(observer, "Bless settles over Aliceia."))
	assert.Equal(t, 0, countContaining(observer, "Aliceia's Bless settles over Aliceia"))
}

func TestSelfCastDefault_NamesNoOneInTheThirdPerson(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	u := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	drainPlain(1)

	spell := &spells.SpellData{SpellId: "curiosity", Name: "Curiosity", EffectType: "curiosity"}
	applyPlayerEffect(u, u, room, spell, 0, spellContestAttackWin())

	caster := drainPlain(1)
	assert.Equal(t, 1, countContaining(caster, "Your Curiosity takes effect."))
	assert.Equal(t, 0, countContaining(caster, "takes effect on Aliceia"))
}

// TestCrossCast_WordingUnchanged is a regression guard, not a red test: casting
// on someone else must keep today's lines exactly.
func TestCrossCast_WordingUnchanged(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	caster := users.GetByUserId(1)
	target := users.GetByUserId(2)
	room := rooms.LoadRoom(1)

	cases := []struct {
		spell      *spells.SpellData
		casterLine string
		targetLine string
	}{
		{&spells.SpellData{SpellId: "purge", Name: "Purge", EffectType: "purge"},
			"Your Purge cleanses Bobrick of afflictions.", "Aliceia's Purge purges the toxins from your body."},
		{&spells.SpellData{SpellId: "heal", Name: "Heal", EffectType: "heal", EffectMagnitude: 3},
			"You weave restorative magic around Bobrick.", "Aliceia's Heal envelops you in healing energy."},
		{&spells.SpellData{SpellId: "bless", Name: "Bless", EffectType: "buff", BuffIds: []int{100}},
			"Your Bless takes effect on Bobrick!", "Aliceia's Bless takes effect on you!"},
	}
	for _, c := range cases {
		drainPlain(1)
		drainPlain(2)
		applyPlayerEffect(caster, target, room, c.spell, 3, spellContestAttackWin())
		assert.Equal(t, 1, countContaining(drainPlain(1), c.casterLine), c.spell.Name)
		assert.Equal(t, 1, countContaining(drainPlain(2), c.targetLine), c.spell.Name)
	}
}

// TestAreaHeal_CasterIsTheirOwnTarget drives the real HelpArea fill, which puts
// every room player in the target list, caster included.
func TestAreaHeal_CasterIsTheirOwnTarget(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	u := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	drainPlain(1)
	drainPlain(2)

	spell := &spells.SpellData{SpellId: "mass-mend", Name: "Mass Mend", Type: spells.HelpArea,
		EffectType: "heal", EffectMagnitude: 3}
	resolveSpell(u, activity.CastingData{SpellId: "mass-mend"}, spell, room)

	caster, observer := drainPlain(1), drainPlain(2)
	assert.Equal(t, 1, countContaining(caster, "A warm glow of healing magic envelops you."))
	assert.Equal(t, 0, countContaining(caster, "restorative magic around Aliceia"))
	assert.Equal(t, 1, countContaining(caster, "restorative magic around Bobrick"))
	assert.Equal(t, 1, countContaining(observer, "Aliceia's Mass Mend envelops you in healing energy."))
	assert.Equal(t, 1, countContaining(observer, "Aliceia channels restorative magic."))
}

func TestAreaPurge_CasterIsTheirOwnTarget(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	u := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	drainPlain(1)
	drainPlain(2)

	spell := &spells.SpellData{SpellId: "cleansing-wave", Name: "Cleansing Wave", Type: spells.HelpArea,
		EffectType: "purge"}
	resolveSpell(u, activity.CastingData{SpellId: "cleansing-wave"}, spell, room)

	caster, observer := drainPlain(1), drainPlain(2)
	assert.Equal(t, 1, countContaining(caster, "You purge the afflictions from your body."))
	assert.Equal(t, 0, countContaining(caster, "cleanses Aliceia"))
	assert.Equal(t, 1, countContaining(caster, "Your Cleansing Wave cleanses Bobrick of afflictions."))
	assert.Equal(t, 1, countContaining(observer, "Cleansing Wave cleanses Aliceia of afflictions."))
	assert.Equal(t, 1, countContaining(observer, "purges the toxins from your body."))
}
```

- [ ] **Step 4: Run them and confirm they fail for the right reason**

Run: `go test ./internal/hooks/ -run 'TestSelfCast|TestCrossCast|TestAreaHeal|TestAreaPurge' -v 2>&1 | grep -E "^--- (PASS|FAIL)|Error:"`
Expected: `TestCrossCast_WordingUnchanged` PASSES (it is the guard). Every other test FAILS, each on an assertion naming the old third-person or doubled line, for example a count of 1 where 0 was expected for `"cleanses Aliceia"`.

- [ ] **Step 5: Delete the three assertion-free tests from `hooks_test.go`**

Delete these three functions in full, each from its `func` line to its closing `}`:
`TestApplyPlayerEffect_PurgeSelf` (`hooks_test.go:2411`), `TestApplyPlayerEffect_HealSelf` (`:2442`), `TestApplyPlayerEffect_BuffSelf` (`:2489`). Leave the neighbouring cross-cast, crit and shield tests alone.

Run (standalone): `grep -cE "func TestApplyPlayerEffect_(PurgeSelf|HealSelf|BuffSelf)\(" internal/hooks/hooks_test.go`
Expected: `0`.

- [ ] **Step 6: Implement the `purge` self-cast shape**

In `internal/hooks/spell_resolution.go`, replace:

```go
	case "purge":
		target.Character.CancelBuffsWithFlag(buffs.Poison)
		target.Character.RemoveCondition(characters.ConditionPoisoned)
		user.SendText(messaging.CategorySpellVital, fmt.Sprintf(
			`<ansi fg="green">Your %s cleanses <ansi fg="username">%s</ansi> of afflictions.%s</ansi>`,
			spellData.Name, target.Character.Name, critTag))
		if target.UserId != user.UserId {
			target.SendText(messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="green"><ansi fg="username">%s</ansi>'s %s purges the toxins from your body.</ansi>`,
				user.Character.Name, spellData.Name))
		} else {
			target.SendText(messaging.CategorySpellVital, `<ansi fg="green">You purge the afflictions from your body.</ansi>`)
		}
		sendVisualRoomText(room, messaging.CategorySpellVital, fmt.Sprintf(
			`<ansi fg="username">%s</ansi>'s <ansi fg="cyan">%s</ansi> cleanses <ansi fg="username">%s</ansi>.`,
			user.Character.Name, spellData.Name, target.Character.Name), user.UserId, target.UserId)
```

with:

```go
	case "purge":
		target.Character.CancelBuffsWithFlag(buffs.Poison)
		target.Character.RemoveCondition(characters.ConditionPoisoned)
		if target.UserId != user.UserId {
			user.SendText(messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="green">Your %s cleanses <ansi fg="username">%s</ansi> of afflictions.%s</ansi>`,
				spellData.Name, target.Character.Name, critTag))
			target.SendText(messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="green"><ansi fg="username">%s</ansi>'s %s purges the toxins from your body.</ansi>`,
				user.Character.Name, spellData.Name))
			sendVisualRoomText(room, messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="username">%s</ansi>'s <ansi fg="cyan">%s</ansi> cleanses <ansi fg="username">%s</ansi>.`,
				user.Character.Name, spellData.Name, target.Character.Name), user.UserId, target.UserId)
		} else {
			// SELF-CAST: one line to the caster and one to the room, naming them
			// once, the shape case "shield" below already has. An area spell
			// puts the caster in its own target list (resolveSpell), so every
			// Cleansing Wave reaches this branch, not only a deliberate self-cast.
			// critTag stays on the caster's line so a crit on yourself is not lost.
			user.SendText(messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="green">You purge the afflictions from your body.%s</ansi>`, critTag))
			sendVisualRoomText(room, messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="cyan">%s</ansi> cleanses <ansi fg="username">%s</ansi> of afflictions.`,
				spellData.Name, target.Character.Name), user.UserId)
		}
```

- [ ] **Step 7: Implement the `heal` self-cast shape**

Replace:

```go
		target.Character.AddCondition(characters.ConditionRegen, durationRounds, regenMult, "heal spell")
		user.SendText(messaging.CategorySpellVital, fmt.Sprintf(
			`<ansi fg="green">You weave restorative magic around <ansi fg="username">%s</ansi>.%s</ansi>`,
			target.Character.Name, critTag))
		if target.UserId != user.UserId {
			target.SendText(messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="green"><ansi fg="username">%s</ansi>'s %s envelops you in healing energy. Your wounds begin to mend.</ansi>`,
				user.Character.Name, spellData.Name))
		} else {
			target.SendText(messaging.CategorySpellVital, `<ansi fg="green">A warm glow of healing magic envelops you. Your wounds begin to mend.</ansi>`)
		}
		sendVisualRoomText(room, messaging.CategorySpellVital, fmt.Sprintf(
			`<ansi fg="username">%s</ansi>'s <ansi fg="cyan">%s</ansi> envelops <ansi fg="username">%s</ansi> in healing light.`,
			user.Character.Name, spellData.Name, target.Character.Name), user.UserId, target.UserId)
```

with:

```go
		target.Character.AddCondition(characters.ConditionRegen, durationRounds, regenMult, "heal spell")
		if target.UserId != user.UserId {
			user.SendText(messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="green">You weave restorative magic around <ansi fg="username">%s</ansi>.%s</ansi>`,
				target.Character.Name, critTag))
			target.SendText(messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="green"><ansi fg="username">%s</ansi>'s %s envelops you in healing energy. Your wounds begin to mend.</ansi>`,
				user.Character.Name, spellData.Name))
			sendVisualRoomText(room, messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="username">%s</ansi>'s <ansi fg="cyan">%s</ansi> envelops <ansi fg="username">%s</ansi> in healing light.`,
				user.Character.Name, spellData.Name, target.Character.Name), user.UserId, target.UserId)
		} else {
			// SELF-CAST: see case "purge". The room line reuses the wording
			// applyMobSelfEffect already uses for a mob healing itself.
			user.SendText(messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="green">A warm glow of healing magic envelops you. Your wounds begin to mend.%s</ansi>`, critTag))
			sendVisualRoomText(room, messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="username">%s</ansi> channels restorative magic.`,
				user.Character.Name), user.UserId)
		}
```

- [ ] **Step 8: Implement the `buff` self-cast shape**

Replace:

```go
		user.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
			`Your %s takes effect on <ansi fg="username">%s</ansi>!%s`,
			spellData.Name, target.Character.Name, critTag))
		if target.UserId != user.UserId {
			target.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
				`<ansi fg="username">%s</ansi>'s %s takes effect on you!`,
				user.Character.Name, spellData.Name))
		}
		// M1 audit defect: this case told the caster and the target and left
		// the room out, while its sibling `case "heal":` above broadcasts. A
		// spell visibly taking hold on someone is not a private exchange.
		// Shape and exclusions mirror the heal line; the category follows this
		// case's own two lines rather than heal's, because a buff is not
		// necessarily vital magic.
		sendVisualRoomText(room, spellSchoolCategory(spellData), fmt.Sprintf(
			`<ansi fg="username">%s</ansi>'s <ansi fg="cyan">%s</ansi> settles over <ansi fg="username">%s</ansi>.`,
			user.Character.Name, spellData.Name, target.Character.Name), user.UserId, target.UserId)
```

with:

```go
		// M1 audit defect: this case told the caster and the target and left
		// the room out, while its sibling `case "heal":` above broadcasts. A
		// spell visibly taking hold on someone is not a private exchange.
		// Shape and exclusions mirror the heal line; the category follows this
		// case's own two lines rather than heal's, because a buff is not
		// necessarily vital magic.
		//
		// KNOWN AND DEFERRED: the buff's own start text ALSO narrates this
		// moment to the target and the room, through the event AddBuff queues
		// above, so a buff with authored start text reaches each audience
		// twice. The messaging arc's M6 merges them into one line per audience.
		// See docs/superpowers/specs/2026-09-11-messaging-m3-item5a-narration-defects-design.md.
		if target.UserId != user.UserId {
			user.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
				`Your %s takes effect on <ansi fg="username">%s</ansi>!%s`,
				spellData.Name, target.Character.Name, critTag))
			target.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
				`<ansi fg="username">%s</ansi>'s %s takes effect on you!`,
				user.Character.Name, spellData.Name))
			sendVisualRoomText(room, spellSchoolCategory(spellData), fmt.Sprintf(
				`<ansi fg="username">%s</ansi>'s <ansi fg="cyan">%s</ansi> settles over <ansi fg="username">%s</ansi>.`,
				user.Character.Name, spellData.Name, target.Character.Name), user.UserId, target.UserId)
		} else {
			// SELF-CAST: see case "purge". The caster line stays, reworded,
			// rather than being dropped: a buff with no authored start text
			// would otherwise leave a self-caster reading nothing at all.
			user.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
				`Your %s takes effect.%s`, spellData.Name, critTag))
			sendVisualRoomText(room, spellSchoolCategory(spellData), fmt.Sprintf(
				`<ansi fg="cyan">%s</ansi> settles over <ansi fg="username">%s</ansi>.`,
				spellData.Name, target.Character.Name), user.UserId)
		}
```

- [ ] **Step 9: Implement the `default:` self-cast wording**

Replace:

```go
	default:
		user.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
			`Your %s takes effect on <ansi fg="username">%s</ansi>.`,
			spellData.Name, target.Character.Name))
```

with:

```go
	default:
		if target.UserId == user.UserId {
			user.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
				`Your %s takes effect.`, spellData.Name))
		} else {
			user.SendText(spellSchoolCategory(spellData), fmt.Sprintf(
				`Your %s takes effect on <ansi fg="username">%s</ansi>.`,
				spellData.Name, target.Character.Name))
		}
```

- [ ] **Step 10: Run the tests and confirm they pass**

Run: `go test ./internal/hooks/ -run 'TestSelfCast|TestCrossCast|TestAreaHeal|TestAreaPurge' -v 2>&1 | grep -E "^--- (PASS|FAIL)|^(ok|FAIL)"`
Expected: every test PASSES.

- [ ] **Step 11: Prove the heal test can fail**

Temporarily change the self-cast heal room line's format string from `` `<ansi fg="username">%s</ansi> channels restorative magic.` `` to `` `<ansi fg="username">%s</ansi>'s Heal envelops <ansi fg="username">%s</ansi> in healing light.` `` and its argument list from `user.Character.Name` to `user.Character.Name, target.Character.Name`.

Run: `go vet ./internal/hooks/ && go test ./internal/hooks/ -run TestSelfCastHeal 2>&1 | grep -E "FAIL|Error:"`
Expected: `go vet` prints nothing (the sabotage compiles), and the test FAILS on `"Aliceia channels restorative magic."`. Revert the sabotage and re-run to green.

- [ ] **Step 12: Run the whole hooks package and format**

Run: `gofmt -l internal/hooks/ && go test ./internal/hooks/ 2>&1 | tail -3`
Expected: `gofmt` prints nothing; `ok  github.com/GoMudEngine/GoMud/internal/hooks`.

- [ ] **Step 13: Commit**

```bash
git add internal/hooks/narration_testhelpers_test.go internal/hooks/selfcast_wording_test.go internal/hooks/hooks_test.go internal/hooks/spell_resolution.go
git commit -m "fix(hooks): a self-cast gets one line and the room names the caster once"
```

---

## Task 2: D1, buff room lines go visual

**Files:**
- Create: `internal/hooks/buff_room_text_test.go`
- Modify: `internal/hooks/Buff_ApplyBuffs.go:88-107`
- Modify: `internal/hooks/NewRound_UserRoundTick.go:288`
- Modify: `internal/hooks/NewTurn_PruneBuffs.go:53`, `:103-110`

- [ ] **Step 1: Write the failing D1 tests**

Create `internal/hooks/buff_room_text_test.go`:

```go
package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Buff room lines describe what the room SEES ("A warm glow surrounds Alice"),
// but went out on the audio channel, which is never sight-gated, so blind and
// unsighted observers received them. M2 fixed the same defect for
// cast_room_text; these three buff phases were never touched.

// expire sets a buff's remaining triggers to the pruning threshold, so the next
// PruneBuffs removes it and sends its end text. Deterministic, unlike counting
// ticks.
func expire(t *testing.T, list []*buffs.Buff, buffId int) {
	t.Helper()
	for _, b := range list {
		if b.BuffId == buffId {
			b.TriggersLeft = buffs.TriggersLeftExpired
			return
		}
	}
	t.Fatalf("buff %d not found to expire", buffId)
}

// rawLineContaining returns the first raw (still tagged) line whose plain text
// contains want, or "" if none does.
func rawLineContaining(raw []string, want string) string {
	for _, line := range raw {
		if strings.Contains(plainText(line), want) {
			return line
		}
	}
	return ""
}

func TestBuffStartRoomText_SightedObserverSeesIt(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	drainPlain(2)

	assert.Equal(t, events.Continue, ApplyBuffs(events.Buff{UserId: 1, BuffId: glowBuffId}))
	assert.Equal(t, 1, countContaining(drainPlain(2), "Aliceia glows."))
}

func TestBuffStartRoomText_UnsightedObserverInTheDarkGetsNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	darken(t, 1)
	drainPlain(2)

	ApplyBuffs(events.Buff{UserId: 1, BuffId: glowBuffId})
	assert.Equal(t, 0, countContaining(drainPlain(2), "glows."),
		"an observer who cannot see must not be told what a buff looks like")
}

func TestBuffStartRoomText_NightVisionSeesItInTheDark(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	darken(t, 1)
	require.True(t, users.GetByUserId(2).Character.Buffs.AddBuff(nightEyesBuffId, true))
	drainPlain(2)

	ApplyBuffs(events.Buff{UserId: 1, BuffId: glowBuffId})
	assert.Equal(t, 1, countContaining(drainPlain(2), "Aliceia glows."))
}

func TestBuffStartRoomText_MobHolderUsesTheMobTag(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	events.DrainQueuedMessagesForTest(2)

	ApplyBuffs(events.Buff{MobInstanceId: 100, BuffId: glowBuffId})
	line := rawLineContaining(events.DrainQueuedMessagesForTest(2), "Skeleton glows.")
	require.NotEmpty(t, line, "the observer must receive the mob's start text")
	assert.Contains(t, line, `fg="mobname`)
	assert.NotContains(t, line, `fg="username">Skeleton`,
		"a mob holder was tagged with the player colour")
}

func TestBuffTriggerRoomText_UnsightedObserverInTheDarkGetsNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	darken(t, 1)
	require.True(t, users.GetByUserId(1).Character.Buffs.AddBuff(shiverBuffId, false))
	drainPlain(2)

	UserRoundTick(events.NewRound{RoundNumber: 1})
	assert.Equal(t, 0, countContaining(drainPlain(2), "shivers."))
}

func TestBuffTriggerRoomText_SightedObserverSeesIt(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	require.True(t, users.GetByUserId(1).Character.Buffs.AddBuff(shiverBuffId, false))
	drainPlain(2)

	UserRoundTick(events.NewRound{RoundNumber: 1})
	assert.Equal(t, 1, countContaining(drainPlain(2), "Aliceia shivers."))
}

func TestBuffEndRoomText_UnsightedObserverInTheDarkGetsNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	darken(t, 1)
	holder := users.GetByUserId(1)
	require.True(t, holder.Character.Buffs.AddBuff(fadeBuffId, false))
	expire(t, holder.Character.Buffs.List, fadeBuffId)
	drainPlain(2)

	PruneBuffs(events.NewTurn{TurnNumber: 1})
	assert.Equal(t, 0, countContaining(drainPlain(2), "fades."))
}

func TestBuffEndRoomText_SightedObserverSeesIt(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	holder := users.GetByUserId(1)
	require.True(t, holder.Character.Buffs.AddBuff(fadeBuffId, false))
	expire(t, holder.Character.Buffs.List, fadeBuffId)
	drainPlain(2)

	PruneBuffs(events.NewTurn{TurnNumber: 1})
	assert.Equal(t, 1, countContaining(drainPlain(2), "Aliceia fades."))
}

func TestBuffEndRoomText_MobHolderIsVisualAndUsesTheMobTag(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	mob := mobs.GetInstance(100)
	require.True(t, mob.Character.Buffs.AddBuff(fadeBuffId, false))
	expire(t, mob.Character.Buffs.List, fadeBuffId)
	events.DrainQueuedMessagesForTest(2)

	PruneBuffs(events.NewTurn{TurnNumber: 1})
	line := rawLineContaining(events.DrainQueuedMessagesForTest(2), "Skeleton fades.")
	require.NotEmpty(t, line)
	assert.Contains(t, line, `fg="mobname`)

	// And the same line is gated by sight.
	require.True(t, mob.Character.Buffs.AddBuff(fadeBuffId, false))
	expire(t, mob.Character.Buffs.List, fadeBuffId)
	darken(t, 1)
	drainPlain(2)
	PruneBuffs(events.NewTurn{TurnNumber: 2})
	assert.Equal(t, 0, countContaining(drainPlain(2), "fades."))
}
```

- [ ] **Step 2: Run them and confirm the right ones fail**

Run: `go test ./internal/hooks/ -run 'TestBuffStartRoomText|TestBuffTriggerRoomText|TestBuffEndRoomText' -v 2>&1 | grep -E "^--- (PASS|FAIL)"`
Expected FAIL: every `..._UnsightedObserverInTheDarkGetsNothing`, `TestBuffStartRoomText_MobHolderUsesTheMobTag`, and `TestBuffEndRoomText_MobHolderIsVisualAndUsesTheMobTag`.
Expected PASS (they are controls): every `..._SightedObserverSeesIt` and `TestBuffStartRoomText_NightVisionSeesItInTheDark`.

- [ ] **Step 3: Start text, mob tag and visual send**

In `internal/hooks/Buff_ApplyBuffs.go`, replace:

```go
		} else if evt.MobInstanceId != 0 {
			if m := mobs.GetInstance(evt.MobInstanceId); m != nil {
				charName = m.Character.GetCharacterName(true)
				charPlainName = m.Character.GetCharacterName(false)
				roomId = m.Character.RoomId
			}
		}
```

with:

```go
		} else if evt.MobInstanceId != 0 {
			if m := mobs.GetInstance(evt.MobInstanceId); m != nil {
				// The mob tag, not the player one. GetCharacterName(true) tags
				// every name `username`, so a mob holder rendered in the player
				// colour. mobDisplayName is what the spell code already uses.
				charName = m.Character.GetCharacterName(true)
				if r := rooms.LoadRoom(m.Character.RoomId); r != nil {
					charName = mobDisplayName(m, r, 0)
				}
				charPlainName = m.Character.GetCharacterName(false)
				roomId = m.Character.RoomId
			}
		}
```

Then replace:

```go
				RoomSendFunc: func(msg string, skip ...int) {
					if r := rooms.LoadRoom(roomId); r != nil {
						r.SendText(messaging.CategoryBuffApply, msg, skip...)
					}
				},
```

with:

```go
				// Visual, not audio. Start text describes what the room SEES
				// ("A warm glow surrounds Alice"), and Room.SendText is never
				// sight-gated, so it reached blind and unsighted observers. M2
				// fixed the same defect for cast_room_text.
				RoomSendFunc: func(msg string, skip ...int) {
					if r := rooms.LoadRoom(roomId); r != nil {
						r.SendTextVisual(messaging.CategoryBuffApply, msg, skip...)
					}
				},
```

- [ ] **Step 4: Trigger text, visual send**

In `internal/hooks/NewRound_UserRoundTick.go`, the call `r.SendText(messaging.CategoryBuffApply, msg, skip...)` occurs exactly once (`:288`). Replace that line with:

```go
										// Visual: see Buff_ApplyBuffs.go start text.
										r.SendTextVisual(messaging.CategoryBuffApply, msg, skip...)
```

- [ ] **Step 5: End text, visual send for both holders, mob tag**

In `internal/hooks/NewTurn_PruneBuffs.go`, `r.SendText(messaging.CategoryBuffExpire, msg, skip...)` occurs exactly twice (`:53` player, `:110` mob). Replace BOTH with:

```go
r.SendTextVisual(messaging.CategoryBuffExpire, msg, skip...)
```

(keeping each line's existing indentation). Then, in the mob branch, replace:

```go
					tCtx := textutil.TokenContext{
						SourceName:      mob.Character.GetCharacterName(true),
						SourcePlainName: mob.Character.GetCharacterName(false),
					}
```

with:

```go
					// The mob tag, not the player one: see Buff_ApplyBuffs.go.
					sourceName := mob.Character.GetCharacterName(true)
					if r := rooms.LoadRoom(mob.Character.RoomId); r != nil {
						sourceName = mobDisplayName(mob, r, 0)
					}
					tCtx := textutil.TokenContext{
						SourceName:      sourceName,
						SourcePlainName: mob.Character.GetCharacterName(false),
					}
```

Run (standalone): `grep -nE "(^|[^a-z])r\.SendText\(messaging\.CategoryBuff" internal/hooks/Buff_ApplyBuffs.go internal/hooks/NewRound_UserRoundTick.go internal/hooks/NewTurn_PruneBuffs.go`
Expected: no output. The personal `user.SendText(messaging.CategoryBuff...)` lines are correct and remain. The `[^a-z]` guard matters: a bare `r\.SendText` also matches the `r` at the end of `user`.

- [ ] **Step 6: Run the tests and confirm they pass**

Run: `go test ./internal/hooks/ -run 'TestBuffStartRoomText|TestBuffTriggerRoomText|TestBuffEndRoomText' -v 2>&1 | grep -E "^--- (PASS|FAIL)|^(ok|FAIL)"`
Expected: all PASS.

- [ ] **Step 7: Prove the darkness test can fail**

Temporarily revert `Buff_ApplyBuffs.go`'s `r.SendTextVisual(messaging.CategoryBuffApply, ...)` back to `r.SendText(...)`.
Run: `go vet ./internal/hooks/ && go test ./internal/hooks/ -run TestBuffStartRoomText_UnsightedObserverInTheDarkGetsNothing 2>&1 | grep -E "FAIL|Error:"`
Expected: `go vet` clean, test FAILS. Revert and re-run to green.

- [ ] **Step 8: Package tests and format**

Run: `gofmt -l internal/hooks/ && go test ./internal/hooks/ 2>&1 | tail -3`
Expected: no gofmt output; `ok`.

- [ ] **Step 9: Commit**

```bash
git add internal/hooks/buff_room_text_test.go internal/hooks/Buff_ApplyBuffs.go internal/hooks/NewRound_UserRoundTick.go internal/hooks/NewTurn_PruneBuffs.go
git commit -m "fix(hooks): buff room lines go out on the visual channel"
```

---

## Task 3: D4, mob buff trigger text

**Files:**
- Modify: `internal/hooks/NewRound_MobRoundTick.go` (import block; `tickMobBuffs` at `:218-261`)
- Test: `internal/hooks/buff_room_text_test.go` (append)

- [ ] **Step 1: Write the failing test**

Append to `internal/hooks/buff_room_text_test.go`:

```go
// TestMobBuffTriggerRoomText is the D4 guard. The player round tick has always
// sent a triggered buff's trigger_room_text; tickMobBuffs never did, so a mob
// holding a trigger-text buff showed nothing. No mob holder of the 7 shipped
// trigger-text buffs could be staged in a playtest, so this is its only check.
func TestMobBuffTriggerRoomText(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	mob := mobs.GetInstance(100)
	require.True(t, mob.Character.Buffs.AddBuff(shiverBuffId, false))
	events.DrainQueuedMessagesForTest(2)

	tickMobBuffs(mob, 100)
	line := rawLineContaining(events.DrainQueuedMessagesForTest(2), "Skeleton shivers.")
	require.NotEmpty(t, line, "a sighted observer must see the mob's trigger text")
	assert.Contains(t, line, `fg="mobname`)

	// Sight-gated like every other buff room line.
	darken(t, 1)
	drainPlain(2)
	tickMobBuffs(mob, 100)
	assert.Equal(t, 0, countContaining(drainPlain(2), "shivers."))
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test ./internal/hooks/ -run TestMobBuffTriggerRoomText -v 2>&1 | grep -E "^--- (PASS|FAIL)|Error"`
Expected: FAIL at `require.NotEmpty` ("a sighted observer must see the mob's trigger text").

- [ ] **Step 3: Add the `textutil` import**

In `internal/hooks/NewRound_MobRoundTick.go`, replace:

```go
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
```

with:

```go
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/textutil"
	"github.com/GoMudEngine/GoMud/internal/users"
```

- [ ] **Step 4: Send trigger text from the mob tick**

In `tickMobBuffs`, replace:

```go
			triggeredBuffIds = append(triggeredBuffIds, buff.BuffId)
		}
		events.AddToQueue(events.BuffsTriggered{MobInstanceId: mobInstanceId, BuffIds: triggeredBuffIds})
```

with:

```go
			// Trigger text. The player round tick has always sent it; this mob
			// tick never did, so a mob holding a trigger-text buff showed
			// nothing. Room line only, because a mob has no client. Visual,
			// because the text describes what the room sees. An expired buff is
			// skipped, matching the player tick. Same shape as the mob branch of
			// PruneBuffs, so the line gets the same buff colour.
			if !buff.Expired() {
				if trigSpec := buffs.GetBuffSpec(buff.BuffId); trigSpec != nil && trigSpec.TriggerRoomText != "" {
					if room := rooms.LoadRoom(mob.Character.RoomId); room != nil {
						tCtx := textutil.TokenContext{
							SourceName:      mobDisplayName(mob, room, 0),
							SourcePlainName: mob.Character.GetCharacterName(false),
						}
						cfg := textutil.SendTextConfig{
							RoomSendFunc: func(msg string, skip ...int) {
								room.SendTextVisual(messaging.CategoryBuffApply, msg, skip...)
							},
						}
						textutil.SendPhaseText("", trigSpec.TriggerRoomText, tCtx, "cyan", cfg)
					}
				}
			}
			triggeredBuffIds = append(triggeredBuffIds, buff.BuffId)
		}
		events.AddToQueue(events.BuffsTriggered{MobInstanceId: mobInstanceId, BuffIds: triggeredBuffIds})
```

- [ ] **Step 5: Run the test and confirm it passes**

Run: `go test ./internal/hooks/ -run TestMobBuffTriggerRoomText -v 2>&1 | grep -E "^--- (PASS|FAIL)"`
Expected: PASS.

- [ ] **Step 6: Package tests, format, commit**

Run: `gofmt -l internal/hooks/ && go test ./internal/hooks/ 2>&1 | tail -3`
Expected: no gofmt output; `ok`.

```bash
git add internal/hooks/NewRound_MobRoundTick.go internal/hooks/buff_room_text_test.go
git commit -m "fix(hooks): a mob holding a trigger-text buff shows it"
```

---

## Task 4: D2 and D5, quest room text is substituted and visual

**Files:**
- Create: `internal/hooks/quest_room_text_test.go`
- Modify: `internal/questengine/bridge.go` (import block; `RoomText` at `:204-212`)

- [ ] **Step 1: Write the failing tests**

Create `internal/hooks/quest_room_text_test.go`:

```go
package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Quest room_text went out RAW on the audio channel: no token substitution, so
// quest 77 showed players a literal {source}; and no sight gate, so a blind
// observer still read "unlocks the strongbox". The behaviour tree reads the same
// key and already did both correctly (behaviortree/actions_dialogue.go).

func TestQuestRoomText_NamesThePlayerToASightedObserver(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	drainPlain(1)
	drainPlain(2)

	questengine.NewGameBridge(users.GetByUserId(1), 1).RoomText("{source} unlocks the strongbox.")

	observer := drainPlain(2)
	assert.Equal(t, 1, countContaining(observer, "Aliceia unlocks the strongbox."))
	assert.Equal(t, 0, countContaining(observer, "{source}"), "a literal token reached a player")
	assert.Equal(t, 0, countContaining(drainPlain(1), "unlocks the strongbox"),
		"the acting player is excluded from their own room line")
}

func TestQuestRoomText_UnsightedObserverInTheDarkGetsNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	darken(t, 1)
	drainPlain(2)

	questengine.NewGameBridge(users.GetByUserId(1), 1).RoomText("{source} unlocks the strongbox.")
	assert.Equal(t, 0, countContaining(drainPlain(2), "unlocks the strongbox"))
}

// TestQuestRoomText_InfraredObserverSeesAFigure proves the name carries the tag
// messaging.Anonymize strips. An untagged name would reach an infrared-only
// observer in full.
func TestQuestRoomText_InfraredObserverSeesAFigure(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	darken(t, 1)
	require.True(t, users.GetByUserId(2).Character.Buffs.AddBuff(heatEyesBuffId, true))
	drainPlain(2)

	questengine.NewGameBridge(users.GetByUserId(1), 1).RoomText("{source} unlocks the strongbox.")

	observer := drainPlain(2)
	require.Equal(t, 1, countContaining(observer, "unlocks the strongbox"))
	for _, line := range observer {
		if strings.Contains(line, "unlocks the strongbox") {
			assert.NotContains(t, line, "Aliceia", "an infrared-only observer read the name")
			// The pipeline capitalises a sentence-initial placeholder.
			assert.Contains(t, strings.ToLower(line), "a figure")
		}
	}
}
```

- [ ] **Step 2: Run them and confirm they fail**

Run: `go test ./internal/hooks/ -run TestQuestRoomText -v 2>&1 | grep -E "^--- (PASS|FAIL)|Error:"`
Expected: all three FAIL. The first on `"Aliceia unlocks the strongbox."` (the raw line reads `{source} unlocks...`), the second because the audio line still arrives, the third because the raw line has no tagged name to anonymize.

- [ ] **Step 3: Add the `textutil` import**

In `internal/questengine/bridge.go`, replace:

```go
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
```

with:

```go
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/textutil"
	"github.com/GoMudEngine/GoMud/internal/users"
```

`textutil` imports nothing internal, so there is no cycle.

- [ ] **Step 4: Substitute and send visual**

Replace:

```go
// RoomText sends a message to everyone in the room except the triggering player.
func (b *GameBridge) RoomText(text string) {
	room := rooms.LoadRoom(b.roomId)
	if room == nil {
		mudlog.Error("GameBridge.RoomText", "error", fmt.Sprintf("room %d not found", b.roomId))
		return
	}
	room.SendText(messaging.CategoryNPCDialogue, text, b.user.UserId)
}
```

with:

```go
// RoomText sends a message to everyone in the room except the triggering player.
//
// It substitutes {source} with the player's TAGGED name, then sends on the
// VISUAL channel, and both matter. The line describes something the room
// watches the player do ("{source} unlocks the strongbox"), so an observer who
// cannot see must not receive it, and Room.SendText is never sight-gated. And
// the name must carry its `username` tag, because messaging.Anonymize strips
// only tagged names: an untagged one would reach an infrared-only observer in
// full.
//
// The behaviour tree already delivered its own room_text this way
// (behaviortree/actions_dialogue.go). Until 2026-09-11 this path sent the text
// raw on the audio channel, so quest 77 showed players a literal {source}.
// ValidateAllRoomText (loader.go) is what keeps every quest line naming {source}.
func (b *GameBridge) RoomText(text string) {
	room := rooms.LoadRoom(b.roomId)
	if room == nil {
		mudlog.Error("GameBridge.RoomText", "error", fmt.Sprintf("room %d not found", b.roomId))
		return
	}
	rendered := textutil.SubstituteTokens(text, textutil.TokenContext{
		SourceName:      b.user.Character.GetCharacterName(true),
		SourcePlainName: b.user.Character.GetCharacterName(false),
	})
	room.SendTextVisual(messaging.CategoryNPCDialogue, rendered, b.user.UserId)
}
```

- [ ] **Step 5: Run the tests and confirm they pass**

Run: `go test ./internal/hooks/ -run TestQuestRoomText -v 2>&1 | grep -E "^--- (PASS|FAIL)"`
Expected: all PASS.

- [ ] **Step 6: Prove the infrared test can fail**

Temporarily change `SourceName: b.user.Character.GetCharacterName(true),` to `SourceName: b.user.Character.GetCharacterName(false),` (untagged).
Run: `go vet ./internal/questengine/ && go test ./internal/hooks/ -run TestQuestRoomText_InfraredObserverSeesAFigure 2>&1 | grep -E "FAIL|Error:"`
Expected: `go vet` clean; test FAILS with `an infrared-only observer read the name`. Revert and re-run to green.

- [ ] **Step 7: Package tests, format, commit**

Run: `gofmt -l internal/hooks/ internal/questengine/ && go test ./internal/hooks/ ./internal/questengine/ 2>&1 | tail -4`
Expected: no gofmt output; both `ok`.

```bash
git add internal/hooks/quest_room_text_test.go internal/questengine/bridge.go
git commit -m "fix(questengine): quest room text names the player and needs sight"
```

---

## Task 5: The startup check (NOT wired into boot yet)

**Files:**
- Modify: `internal/questengine/loader.go` (import block; new functions at the end)
- Create: `internal/questengine/roomtext_validation_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/questengine/roomtext_validation_test.go`:

```go
package questengine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoomTextProblems(t *testing.T) {
	cases := []struct {
		name        string
		text        string
		wantProblem bool
		mentions    string
	}{
		{"names the player", "{source} unlocks the strongbox.", false, ""},
		{"subjectless fragment", "unlocks the strongbox.", true, "{source}"},
		{"uses target", "{source} glares at {target}.", true, "{target}"},
		{"uses target_plain", "{source} glares at {target_plain}.", true, "{target_plain}"},
		{"uses source_plain", "{source_plain} unlocks the strongbox.", true, "{source_plain}"},
		{"unknown token", "{source} opens {thing}.", true, "{thing}"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			problems := roomTextProblems(c.text)
			if !c.wantProblem {
				assert.Empty(t, problems)
				return
			}
			assert.NotEmpty(t, problems)
			assert.True(t, strings.Contains(strings.Join(problems, " | "), c.mentions),
				"a problem should mention %s, got %v", c.mentions, problems)
		})
	}
}

// withEngine swaps in a fresh engine holding one quest whose single trigger has
// one room_text action, and restores the previous engine afterwards.
func withEngine(t *testing.T, roomText string) {
	t.Helper()
	prev := globalEngine
	t.Cleanup(func() { globalEngine = prev })
	globalEngine = NewEngine()
	globalEngine.RegisterQuest(&QuestDef{
		QuestId: 90001,
		Triggers: []TriggerDef{{
			Event:   "room_interact",
			Actions: []ActionDef{{RoomText: roomText}},
		}},
	})
}

func TestValidateAllRoomText_PanicsAtStartupOnABadLine(t *testing.T) {
	withEngine(t, "unlocks the strongbox.")
	assert.Panics(t, ValidateAllRoomText,
		"a quest line that names no one must stop the boot, not fail silently in play")
}

func TestValidateAllRoomText_AcceptsAGoodLine(t *testing.T) {
	withEngine(t, "{source} unlocks the strongbox.")
	assert.NotPanics(t, ValidateAllRoomText)
}
```

- [ ] **Step 2: Run them and confirm they fail to compile**

Run: `go test ./internal/questengine/ -run 'TestRoomTextProblems|TestValidateAllRoomText' 2>&1 | head -4`
Expected: build failure, `undefined: roomTextProblems` and `undefined: ValidateAllRoomText`.

- [ ] **Step 3: Add the imports**

In `internal/questengine/loader.go`, replace:

```go
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"gopkg.in/yaml.v2"
```

with:

```go
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/textutil"
	"gopkg.in/yaml.v2"
```

- [ ] **Step 4: Implement the check**

Append to the end of `internal/questengine/loader.go`:

```go
// ValidateAllRoomText enforces the one convention quest room_text uses, and
// panics at startup on a line that breaks it, the same way ValidateAllFlags
// does for flag references.
//
// Every quest room_text is something the room watches the triggering player
// do, so it must name them with {source}. Until 2026-09-11 twenty-one lines
// were written as subjectless fragments, so the room read "unlocks the
// strongbox" with no one doing it, and one used {source} that nothing filled
// in. A line that fails here would otherwise fail silently in play.
func ValidateAllRoomText() {
	var problems []string
	for _, q := range globalEngine.quests {
		for i, t := range q.Triggers {
			for j, a := range t.Actions {
				if a.RoomText == "" {
					continue
				}
				prefix := fmt.Sprintf("quest %d trigger %d action %d room_text", q.QuestId, i, j)
				for _, p := range roomTextProblems(a.RoomText) {
					problems = append(problems, prefix+": "+p)
				}
			}
		}
	}

	if len(problems) > 0 {
		// The engine stores quests in a map; sort so the report is stable.
		sort.Strings(problems)
		panic(fmt.Sprintf("Quest room_text validation failed (%d errors):\n  %s", len(problems), strings.Join(problems, "\n  ")))
	}

	mudlog.Info("ValidateAllRoomText()", "msg", "all quest room_text validated")
}

// roomTextProblems returns every way text breaks the quest room_text
// convention, or nil. Split out so the rules are testable without an engine.
//
// It is stricter than buffs and spells, which only WARN on an unknown token.
// A new rule with no shipped violations can fail at boot at no cost; upgrading
// buffs and spells would change what is allowed to boot, so that is filed.
func roomTextProblems(text string) []string {
	var problems []string
	if !strings.Contains(text, "{source}") {
		problems = append(problems, "must name the acting player with {source}; the room is watching them act")
	}
	for _, token := range []string{"{target}", "{target_plain}"} {
		if strings.Contains(text, token) {
			problems = append(problems, token+" is not available: a quest has no target, so it would render empty")
		}
	}
	if strings.Contains(text, "{source_plain}") {
		problems = append(problems, "{source_plain} is an untagged name that cannot be anonymized in the dark; use {source}")
	}
	problems = append(problems, textutil.ValidateTokens(text)...)
	return problems
}
```

- [ ] **Step 5: Run the tests and confirm they pass**

Run: `go test ./internal/questengine/ -run 'TestRoomTextProblems|TestValidateAllRoomText' -v 2>&1 | grep -E "^(---|    ---) (PASS|FAIL)|^(ok|FAIL)"`
Expected: all PASS.

- [ ] **Step 6: Confirm the check is NOT wired into boot yet**

Run: `grep -c "ValidateAllRoomText()" internal/questengine/loader.go`
Expected: `2`, the function's own declaration and its `mudlog.Info` line; neither is a call (corrected in execution). The call check that matters is `grep -nE "^\s+ValidateAllRoomText\(\)" internal/questengine/loader.go`, which must print nothing. Wiring it now would fail boot on today's shipped data; that happens in Task 6 after the rewrite.

- [ ] **Step 7: Package tests, format, commit**

Run: `gofmt -l internal/questengine/ && go test ./internal/questengine/ 2>&1 | tail -3`
Expected: no gofmt output; `ok`.

```bash
git add internal/questengine/loader.go internal/questengine/roomtext_validation_test.go
git commit -m "feat(questengine): a startup check for quest room text"
```

---

## Task 6: D6, rewrite the 21 quest lines and wire the check into boot

**Files:**
- Modify: 13 quest files under `_datafiles/world/dogmud/quests/`
- Modify: `internal/questengine/loader.go` (`LoadDataFiles`)
- Test: `internal/questengine/roomtext_validation_test.go` (append)

- [ ] **Step 1: Write the failing shipped-data test**

Append to `internal/questengine/roomtext_validation_test.go`:

```go
// TestShippedQuestRoomTextFollowsConvention reads the real quest files. It is
// the data half of the check: ValidateAllRoomText stops a boot, this stops a
// merge.
func TestShippedQuestRoomTextFollowsConvention(t *testing.T) {
	files, err := filepath.Glob("../../_datafiles/world/dogmud/quests/*.yaml")
	require.NoError(t, err)
	require.NotEmpty(t, files, "the glob must find the quest files, or this test proves nothing")

	checked := 0
	for _, f := range files {
		data, err := os.ReadFile(f)
		require.NoError(t, err)
		var q QuestDef
		require.NoError(t, yaml.Unmarshal(data, &q), f)
		for i, tr := range q.Triggers {
			for j, a := range tr.Actions {
				if a.RoomText == "" {
					continue
				}
				checked++
				assert.Empty(t, roomTextProblems(a.RoomText),
					"%s trigger %d action %d: %q", filepath.Base(f), i, j, a.RoomText)
			}
		}
	}
	assert.Equal(t, 22, checked,
		"22 quest room_text lines ship; a different count means the inventory moved, so re-read this test")
}
```

Then replace this file's import block:

```go
import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)
```

with:

```go
import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)
```

- [ ] **Step 2: Run it and confirm it fails on exactly 21 lines**

Run: `go test ./internal/questengine/ -run TestShippedQuestRoomTextFollowsConvention 2>&1 | grep -c "must name the acting player"`
Expected: `21`. Quest 77 is the 22nd line and already passes. A count of 0 means the test did not parse the files; stop and find out why.

- [ ] **Step 3: Rewrite the 21 lines**

Each line gains `{source} ` at the start of its quoted text. Run exactly these commands:

```bash
Q=_datafiles/world/dogmud/quests
sed -i '118s/room_text: "/room_text: "{source} /' $Q/14-the_undertow.yaml
sed -i '71s/room_text: "/room_text: "{source} /'  $Q/63-dock_rat.yaml
sed -i '78s/room_text: "/room_text: "{source} /'  $Q/65-the_street_sweepers_secret.yaml
sed -i '69s/room_text: "/room_text: "{source} /'  $Q/68-the_cooperage_circle.yaml
sed -i '103s/room_text: "/room_text: "{source} /' $Q/68-the_cooperage_circle.yaml
sed -i '86s/room_text: "/room_text: "{source} /'  $Q/69-the_gallery_cipher.yaml
sed -i '72s/room_text: "/room_text: "{source} /'  $Q/70-the_pre_founding_web.yaml
sed -i '91s/room_text: "/room_text: "{source} /'  $Q/71-the_tribute.yaml
sed -i '58s/room_text: "/room_text: "{source} /'  $Q/72-the_water_dispute.yaml
sed -i '76s/room_text: "/room_text: "{source} /'  $Q/73-the_margin_notation.yaml
sed -i '101s/room_text: "/room_text: "{source} /' $Q/73-the_margin_notation.yaml
sed -i '127s/room_text: "/room_text: "{source} /' $Q/73-the_margin_notation.yaml
sed -i '96s/room_text: "/room_text: "{source} /'  $Q/74-the_undercroft.yaml
sed -i '120s/room_text: "/room_text: "{source} /' $Q/74-the_undercroft.yaml
sed -i '145s/room_text: "/room_text: "{source} /' $Q/74-the_undercroft.yaml
sed -i '177s/room_text: "/room_text: "{source} /' $Q/74-the_undercroft.yaml
sed -i '73s/room_text: "/room_text: "{source} /'  $Q/75-the_surveyors_report.yaml
sed -i '107s/room_text: "/room_text: "{source} /' $Q/75-the_surveyors_report.yaml
sed -i '54s/room_text: "/room_text: "{source} /'  $Q/76-the_disc.yaml
sed -i '85s/room_text: "/room_text: "{source} /'  $Q/76-the_disc.yaml
sed -i '127s/room_text: "/room_text: "{source} /' $Q/76-the_disc.yaml
```

- [ ] **Step 4: Verify the rewrite landed exactly once per line**

Run (standalone): `grep -rn 'room_text: "{source} {source}' _datafiles/world/dogmud/quests/`
Expected: no output (no doubled token).

Run: `grep -rh 'room_text: "{source} ' _datafiles/world/dogmud/quests/ | wc -l`
Expected: `22` (21 rewritten plus quest 77).

Run: `git diff --stat -- _datafiles/world/dogmud/quests/`
Expected: 12 files changed, 21 insertions, 21 deletions. (Corrected in execution: the 21 lines live in 12 files, not 13.)

- [ ] **Step 5: Run the data test and confirm it passes**

Run: `go test ./internal/questengine/ -run TestShippedQuestRoomTextFollowsConvention -v 2>&1 | grep -E "^--- (PASS|FAIL)"`
Expected: PASS.

- [ ] **Step 6: Wire the check into boot**

In `internal/questengine/loader.go`, replace:

```go
	ValidateAllFlags()

	mudlog.Info("questengine.LoadDataFiles()", "loadedCount", len(globalEngine.quests), "Time Taken", time.Since(start))
```

with:

```go
	ValidateAllFlags()
	ValidateAllRoomText()

	mudlog.Info("questengine.LoadDataFiles()", "loadedCount", len(globalEngine.quests), "Time Taken", time.Since(start))
```

- [ ] **Step 7: Run the tests that load real quest data**

Run: `go test ./internal/hooks/ -run TestHandleQuestUpdate -v 2>&1 | grep -E "^--- (PASS|FAIL)|panic"`
Expected: every `TestHandleQuestUpdate_*` test PASSES and no `panic` appears. These call `questengine.LoadDataFiles()`, so they now run the startup check on shipped data.

- [ ] **Step 8: Commit**

```bash
git add internal/questengine/loader.go internal/questengine/roomtext_validation_test.go _datafiles/world/dogmud/quests/14-the_undertow.yaml _datafiles/world/dogmud/quests/63-dock_rat.yaml _datafiles/world/dogmud/quests/65-the_street_sweepers_secret.yaml _datafiles/world/dogmud/quests/68-the_cooperage_circle.yaml _datafiles/world/dogmud/quests/69-the_gallery_cipher.yaml _datafiles/world/dogmud/quests/70-the_pre_founding_web.yaml _datafiles/world/dogmud/quests/71-the_tribute.yaml _datafiles/world/dogmud/quests/72-the_water_dispute.yaml _datafiles/world/dogmud/quests/73-the_margin_notation.yaml _datafiles/world/dogmud/quests/74-the_undercroft.yaml _datafiles/world/dogmud/quests/75-the_surveyors_report.yaml _datafiles/world/dogmud/quests/76-the_disc.yaml
git commit -m "fix(quests): every quest room line names who acted, checked at startup"
```

- [ ] **Step 9: Boot the committed tree against the shipped data**

The worktree checks out HEAD, so this runs against the commit from Step 8.

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1; echo "exit=$?"
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log
grep -c "Server Ready" boot.log
grep -c "all quest room_text validated" boot.log
```

Expected: `exit=124` (the timeout fired because the server stayed up), `0` panic lines, `1` Server Ready, `1` validation line. Do not grep for the bare word `panic`: the config value `MapConsistencyEnforce: panic` produces false hits.

Clean up:

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git worktree remove --force C:/tmp/dogmud-boot-check || (powershell -Command "Remove-Item -Recurse -Force C:\tmp\dogmud-boot-check"; git worktree prune)
```

---

## Task 7: The Cat's Eye Draught grants night vision

The darkness lanes in Task 8 need a witness who can see in the dark. The only DOGMud buff meant to grant that misspells its flag, so it grants nothing today.

**Files:**
- Create: `internal/buffs/shipped_flags_test.go`
- Modify: `_datafiles/world/dogmud/buffs/65-cats_eye_draught.yaml:9`

- [ ] **Step 1: Write the failing test**

Create `internal/buffs/shipped_flags_test.go`:

```go
package buffs

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// TestCatsEyeDraughtGrantsNightVision guards a flag spelling. The draught's
// buff listed `night-vision`, but the constant every sight check reads is
// `nightvision`, and nothing normalises or validates buff flag names at load.
// So the draught granted no night vision at all, while its item description
// promised that "the darkness becomes transparent".
func TestCatsEyeDraughtGrantsNightVision(t *testing.T) {
	data, err := os.ReadFile("../../_datafiles/world/dogmud/buffs/65-cats_eye_draught.yaml")
	require.NoError(t, err)
	var spec BuffSpec
	require.NoError(t, yaml.Unmarshal(data, &spec))
	require.Equal(t, 65, spec.BuffId, "read the wrong file, so this test proves nothing")
	assert.Contains(t, spec.Flags, NightVision)
}
```

- [ ] **Step 2: Run it and confirm it fails for the right reason**

Run: `go test ./internal/buffs/ -run TestCatsEyeDraughtGrantsNightVision -v 2>&1 | grep -E "^--- (PASS|FAIL)|does not contain|Error:"`
Expected: FAIL on `assert.Contains`, reporting a flag list holding `night-vision`. A failure on the `BuffId` check instead means the file did not parse; stop and find out why.

- [ ] **Step 3: Fix the flag**

```bash
sed -i 's/^  - night-vision$/  - nightvision/' _datafiles/world/dogmud/buffs/65-cats_eye_draught.yaml
```

Run: `git diff -- _datafiles/world/dogmud/buffs/65-cats_eye_draught.yaml`
Expected: exactly one line changed, `-  - night-vision` to `+  - nightvision`.

- [ ] **Step 4: Run the test and confirm it passes**

Run: `go test ./internal/buffs/ 2>&1 | tail -2`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/buffs/shipped_flags_test.go _datafiles/world/dogmud/buffs/65-cats_eye_draught.yaml
git commit -m "fix(buffs): the Cat's Eye Draught grants night vision"
```

---

## Task 8: Playtest fixtures

**Files:**
- Modify: `tools/playtest/profiles/m2-witness.yaml`
- Create: `tools/playtest/scenarios/m3-item5a-lit.yaml`, `m3-item5a-quest-dark.yaml`, `m3-item5a-buff-dark.yaml`
- Create: `tools/playtest/goals/scenarios/m3-item5a-lit/actor.yaml`, `witness.yaml`
- Create: `tools/playtest/goals/scenarios/m3-item5a-quest-dark/actor.yaml`, `witness.yaml`
- Create: `tools/playtest/goals/scenarios/m3-item5a-buff-dark/actor.yaml`, `witness.yaml`

- [ ] **Step 1: Give the witness a Cat's Eye Draught**

In `tools/playtest/profiles/m2-witness.yaml`, replace:

```yaml
  items:
    - itemid: 30036
```

with:

```yaml
  items:
    - itemid: 30036
    # Cat's Eye Draught: grants night vision when drunk. Carried for the M3
    # item 5a darkness lanes, where the witness must be unsighted first and
    # sighted second. This profile still has NO night vision until a goal
    # tells the witness to drink it.
    - itemid: 30047
```

- [ ] **Step 2: Confirm the profile still sanitizes**

Run: `go test ./internal/playtestprofiles/ -run TestRepoTemplatesSanitize -v 2>&1 | grep -E "^--- (PASS|FAIL)"`
Expected: PASS.

- [ ] **Step 3: Scenario A, lit room**

Create `tools/playtest/scenarios/m3-item5a-lit.yaml`:

```yaml
# M3 item 5a, lane A: a LIT room. Quest wording (D6) and self-cast wording (D3).
#
# Room 5343, The Reliquary, Pothole Coulee: fort biome, lit. Both players can
# see, so every line that should arrive does. Two things changed:
#   - quest room lines now name who acted ("<name> works something free...")
#   - casting on yourself gives YOU one line, and the room one line naming you once
name: m3-item5a-lit
mode: party
summary: >-
  A veteran triggers a quest action and casts on themselves while a witness
  reads exactly what the room is told.
on_actor_stop: continue
budgets:
  wall_clock: 25m
requires:
  max_connections: 20
roster:
  - id: actor
    personality: bug-finder
    goals: goals/scenarios/m3-item5a-lit/actor.yaml
  - id: witness
    personality: bug-finder
    goals: goals/scenarios/m3-item5a-lit/witness.yaml

group_goals:
  - id: quest-names-actor
    do: Actor types `look disc`. Witness stands in the room.
    verify: >-
      Witness reads a line naming the actor, "Veteran Pathfinder works something
      free of the cracked floor and pockets it." A line with no name in front is
      the defect. It fires once per character, so do it once.
  - id: self-cast-one-line
    do: >-
      Actor casts `cast chrysalis-glow` with no target, then `cast
      cleansing-wave`. Witness casts `cast heal` with no target.
    verify: >-
      The caster is told once, in the second person, never "on <their own
      name>". The room names the caster once, never "<name>'s Mend Flesh
      envelops <name>".
  - id: reads-well
    do: Judge every quest and self-cast line seen as prose.
    verify: Report anything that repeats, reads machine-made, or names someone oddly.
```

- [ ] **Step 4: Scenario A goals**

Create `tools/playtest/goals/scenarios/m3-item5a-lit/actor.yaml`:

```yaml
# M3 item 5a, lane A: the ACTOR. You are Veteran Pathfinder, in a lit room with
# Ordel Quist, who reads what the room is told. Quote every line VERBATIM.
#
# Companions may follow you. They are expected; ignore them.
# 3 commands per round on the AI port; the overflow is dropped.

ephemeral:
  profile: veteran
  start_room: 5343
  budgets:
    wall_clock: 25m

goals:
  - >-
    Confirm Ordel is in the room before anything else. Tell them on the party
    channel when you are about to act.
  - >-
    QUEST LINE. Type `look disc`. Quote what YOU are told. Then ask Ordel to
    quote what THEY read. They should read your name in front of the action:
    "Veteran Pathfinder works something free of the cracked floor and pockets
    it." Do this ONCE only; it will not fire twice for you.
  - >-
    SELF-CAST, BUFF. Type `cast chrysalis-glow` with NO target, so it lands on
    you. If it fizzles, cast it again. Quote every line you receive. You should
    get "Your Chrysalis Glow takes effect." and, separately, "A warm glow
    surrounds you." That second line is the effect's own description and is
    EXPECTED. The defect would be a line telling you it took effect "on Veteran
    Pathfinder".
  - >-
    SELF-CAST, AREA. Type `cast cleansing-wave`. It hits everyone in the room,
    including you. Quote every line. You should get "You purge the afflictions
    from your body." exactly ONCE, plus a separate line about cleansing Ordel.
    Two lines both about purging yourself is the defect.
  - >-
    Ask Ordel to cast `cast heal` on themselves, and quote what you read. It
    should be "Ordel Quist channels restorative magic." It should NOT be
    "Ordel Quist's Mend Flesh envelops Ordel Quist in healing light."
  - Judge every line as writing. Quote anything awkward, repetitive, or odd.
```

Create `tools/playtest/goals/scenarios/m3-item5a-lit/witness.yaml`:

```yaml
# M3 item 5a, lane A: the WITNESS. You are Ordel Quist, standing in a lit room
# with Veteran Pathfinder. You are the instrument: STAND STILL AND READ, and
# quote every line VERBATIM. You are carrying a Cat's Eye Draught; do NOT drink
# it in this lane.
#
# 3 commands per round on the AI port; the overflow is dropped.

ephemeral:
  profile: m2-witness
  start_room: 5343
  budgets:
    wall_clock: 25m

goals:
  - Confirm on the party channel that you are in the same room as Veteran Pathfinder.
  - >-
    When they `look disc`, quote the line you read. It must name them in front
    of the action. A line that starts with a bare verb and no name is the
    defect.
  - >-
    When they `cast chrysalis-glow` on themselves, quote every line. Expect
    "Chrysalis Glow settles over Veteran Pathfinder." and, separately, "A warm
    glow surrounds Veteran Pathfinder." The defect is "Veteran Pathfinder's
    Chrysalis Glow settles over Veteran Pathfinder."
  - >-
    When they `cast cleansing-wave`, quote every line. You are one of its
    targets, so you are told it purged YOU, and you also read a line about it
    cleansing Veteran Pathfinder. That second line should name them ONCE.
  - >-
    Then cast `cast heal` with NO target, so it lands on you. The spell is
    called Mend Flesh. If it fizzles, cast it again. Quote what YOU are told.
    You should get ONE line, "A warm glow of healing magic envelops you. Your
    wounds begin to mend." Two lines about healing yourself is the defect.
  - Judge every line as writing. Quote anything awkward or odd.
```

- [ ] **Step 5: Scenario B, quest in the dark**

Create `tools/playtest/scenarios/m3-item5a-quest-dark.yaml`:

```yaml
# M3 item 5a, lane B: a quest line in the DARK. D2 (sight-gated) and D5 (no
# literal {source}).
#
# Room 6397, The Records Archive, in the crash site: cave biome, genuinely
# unlit. The records trigger fires ONCE PER CHARACTER, so the order below is
# the whole design:
#   1. witness triggers it while the actor cannot see  -> actor gets NOTHING (D2)
#   2. witness drinks a Cat's Eye Draught (night vision)
#   3. actor triggers it while the witness now can see -> witness reads a NAMED
#      line with no literal {source} (D5)
name: m3-item5a-quest-dark
mode: party
summary: >-
  Two players trigger the same quest action in an unlit room, one while the
  other cannot see and one while the other can.
on_actor_stop: continue
budgets:
  wall_clock: 25m
requires:
  max_connections: 20
roster:
  - id: actor
    personality: bug-finder
    goals: goals/scenarios/m3-item5a-quest-dark/actor.yaml
  - id: witness
    personality: bug-finder
    goals: goals/scenarios/m3-item5a-quest-dark/witness.yaml

group_goals:
  - id: confirm-dark
    do: Both type `look` first.
    verify: Neither can see a normal room description. If either can, the lane is void.
  - id: unsighted-gets-nothing
    do: Witness types `look records` while the actor waits.
    verify: The actor receives NOTHING about it. Silence is the PASS.
  - id: sighted-reads-name
    do: Witness types `drink draught`, confirms they can see, then the actor types `look records`.
    verify: >-
      The witness reads a line naming the actor, "Veteran Pathfinder stands
      motionless before the wall of moving light...". A literal "{source}" is
      the defect. If the draught does not let the witness see, report that at
      once: the lane cannot test D5.
```

- [ ] **Step 6: Scenario B goals**

Create `tools/playtest/goals/scenarios/m3-item5a-quest-dark/actor.yaml`:

```yaml
# M3 item 5a, lane B: the ACTOR. You are Veteran Pathfinder in an UNLIT cave
# room. You have no night vision and that is on purpose. Ordel Quist is with
# you. Quote every line VERBATIM. Companions may follow; ignore them.
#
# 3 commands per round on the AI port; the overflow is dropped.

ephemeral:
  profile: veteran
  start_room: 6397
  budgets:
    wall_clock: 25m

goals:
  - >-
    Type `look`. Report what you get. If you can read a normal room
    description, say so at once: the lane is void.
  - >-
    WAIT for Ordel. They will type `look records` first. While they do, report
    EXACTLY what you receive about it. THE CORRECT ANSWER IS NOTHING. Count the
    lines about Ordel's action and state the number, even when it is zero.
  - >-
    Only after Ordel has drunk their draught and says they can see, type `look
    records` yourself, ONCE. Quote what you are told, then ask Ordel what they
    read.
```

Create `tools/playtest/goals/scenarios/m3-item5a-quest-dark/witness.yaml`:

```yaml
# M3 item 5a, lane B: the WITNESS. You are Ordel Quist in an UNLIT cave room.
# You have NO night vision yet. You carry one Cat's Eye Draught. The ORDER of
# your goals is the whole test, so follow it exactly. Quote every line VERBATIM.
#
# 3 commands per round on the AI port; the overflow is dropped.

ephemeral:
  profile: m2-witness
  start_room: 6397
  budgets:
    wall_clock: 25m

goals:
  - >-
    Type `look`. Report what you get. If you can read a normal room
    description, say so at once: the lane is void.
  - >-
    FIRST: type `look records`, ONCE. Quote what you are told. Tell Veteran
    Pathfinder you have done it and ask what they received. They should have
    received nothing.
  - >-
    SECOND: type `drink draught`. Then `look` again and report whether you can
    now see a normal room description. Tell Veteran Pathfinder when you can. If
    you still cannot see, report that loudly.
  - >-
    THIRD: when Veteran Pathfinder types `look records`, quote EXACTLY the line
    you read about them. It must name them: "Veteran Pathfinder stands
    motionless before the wall of moving light, and for a long moment does not
    seem to breathe." If you read the characters "{source}" anywhere, that is
    the defect. Quote it.
```

- [ ] **Step 7: Scenario C, a buff in the dark**

Create `tools/playtest/scenarios/m3-item5a-buff-dark.yaml`:

```yaml
# M3 item 5a, lane C: buff room lines in the DARK (D1).
#
# Room 3101, Cave Mouth, Ironwind Steppe: cave biome, unlit, the room the M2
# darkness lane used. A buff's room line ("A warm glow surrounds <name>")
# describes something seen, so an observer who cannot see must not get it.
# The control matters just as much: a spell landing ON the witness must still
# be told to them.
#
# One step deliberately casts at a player the caster cannot see, to find out
# whether that works (the code says it does). Nothing else depends on it: the
# other casts are self-casts or an area spell.
name: m3-item5a-buff-dark
mode: party
summary: >-
  A veteran casts buffs in an unlit cave while a sightless witness reports
  what they are, and are not, told.
on_actor_stop: continue
budgets:
  wall_clock: 25m
requires:
  max_connections: 20
roster:
  - id: actor
    personality: bug-finder
    goals: goals/scenarios/m3-item5a-buff-dark/actor.yaml
  - id: witness
    personality: bug-finder
    goals: goals/scenarios/m3-item5a-buff-dark/witness.yaml

group_goals:
  - id: confirm-dark
    do: Both type `look` first.
    verify: Neither can see a normal room description. If either can, the lane is void.
  - id: unsighted-gets-nothing
    do: Actor casts `cast chrysalis-glow` on themselves.
    verify: The witness receives NOTHING about the glow. Silence is the PASS.
  - id: still-told-about-yourself
    do: Actor casts `cast cleansing-wave`, which lands on everyone in the room.
    verify: The witness IS told the wave purged them. Silence here is serious.
  - id: cast-at-unseen-target
    do: >-
      Before the witness drinks anything, the actor types `cast conviction-armor
      ordel`, naming a player they cannot see.
    verify: >-
      A CHECK, not a pass or fail for this run. Record exactly what happens. If
      the cast is refused, quote the refusal. If it starts or lands, quote what
      the actor is told and what the witness is told, and say whether the
      witness is told WHO cast it. A cast that works on a player the caster
      cannot see is filed as a follow-up.
  - id: sighted-sees-it
    do: Witness types `drink draught`, then the actor casts `cast conviction-armor` on themselves.
    verify: >-
      The witness now reads "Veteran Pathfinder is encased in a shimmering layer
      of conviction." and a line that Conviction Armor settles over them.
```

- [ ] **Step 8: Scenario C goals**

Create `tools/playtest/goals/scenarios/m3-item5a-buff-dark/actor.yaml`:

```yaml
# M3 item 5a, lane C: the ACTOR. You are Veteran Pathfinder in an UNLIT cave.
# You have no night vision, on purpose. Ordel Quist stands with you and cannot
# see either. Quote every line VERBATIM. Ignore any companions.
#
# 3 commands per round on the AI port; the overflow is dropped.

ephemeral:
  profile: veteran
  start_room: 3101
  budgets:
    wall_clock: 25m

goals:
  - >-
    Type `look`. If you can read a normal room description, say so at once.
  - >-
    Type `cast chrysalis-glow` with NO target. If it fizzles, cast it again.
    Tell Ordel the moment it lands and ask what they received. THE CORRECT
    ANSWER IS NOTHING.
  - >-
    Type `cast cleansing-wave`. It lands on everyone in the room. Ask Ordel what
    they were told. They MUST be told something about the wave purging them.
  - >-
    TARGETING CHECK, while you still cannot see. Type `cast conviction-armor
    ordel`, naming Ordel even though you cannot see them. Quote exactly what you
    are told: whether the cast is refused, starts, or lands. If it fizzles, try
    once more. Ask Ordel what they were told. This step only records what
    happens; neither outcome is a failure of your run.
  - >-
    Wait until Ordel has drunk their draught and says they can see. Then type
    `cast conviction-armor` with NO target. If it fizzles, cast it again. Ask
    what they read now.
```

Create `tools/playtest/goals/scenarios/m3-item5a-buff-dark/witness.yaml`:

```yaml
# M3 item 5a, lane C: the WITNESS. You are Ordel Quist in an UNLIT cave. You
# have NO night vision and carry one Cat's Eye Draught. STAND STILL AND READ.
# Quote every line VERBATIM, and count lines even when the count is zero.
#
# 3 commands per round on the AI port; the overflow is dropped.

ephemeral:
  profile: m2-witness
  start_room: 3101
  budgets:
    wall_clock: 25m

goals:
  - >-
    Type `look`. If you can read a normal room description, say so at once.
  - >-
    When Veteran Pathfinder casts a glow on themselves, report EXACTLY what you
    receive about it. THE CORRECT ANSWER IS NOTHING. Do not treat silence as a
    fault.
  - >-
    When they cast a cleansing wave, you are one of its targets, so you MUST be
    told. Quote it. If you are told nothing, report that loudly: it means too
    much was silenced.
  - >-
    Veteran Pathfinder will then try to cast armor on you by name, though
    neither of you can see. Before drinking anything, quote every line you
    receive about it, and say whether any line names who cast it.
  - >-
    Now type `drink draught`, then `look`, and report whether you can see a
    normal room description. Tell Veteran Pathfinder when you can. If you still
    cannot see, report that loudly.
  - >-
    When they cast conviction armor on themselves, quote every line you read.
    You should now read that they are encased in a shimmering layer of
    conviction.
```

- [ ] **Step 9: Commit**

```bash
git add tools/playtest/profiles/m2-witness.yaml tools/playtest/scenarios/m3-item5a-lit.yaml tools/playtest/scenarios/m3-item5a-quest-dark.yaml tools/playtest/scenarios/m3-item5a-buff-dark.yaml tools/playtest/goals/scenarios/m3-item5a-lit/actor.yaml tools/playtest/goals/scenarios/m3-item5a-lit/witness.yaml tools/playtest/goals/scenarios/m3-item5a-quest-dark/actor.yaml tools/playtest/goals/scenarios/m3-item5a-quest-dark/witness.yaml tools/playtest/goals/scenarios/m3-item5a-buff-dark/actor.yaml tools/playtest/goals/scenarios/m3-item5a-buff-dark/witness.yaml
git commit -m "test(playtest): three lanes for the M3 item 5a narration gate"
```

---

## Task 9: Documentation

**Files:**
- Modify: `internal/questengine/context.md`
- Modify: `docs/PATCH_NOTES.md`

- [ ] **Step 1: Document `RoomText` in the GameBridge section**

In `internal/questengine/context.md`, replace:

```markdown
This is the seam that keeps the evaluator testable: tests supply a fake
`ActionContext`/`PlayerState`, production supplies `GameBridge`.
```

with:

```markdown
This is the seam that keeps the evaluator testable: tests supply a fake
`ActionContext`/`PlayerState`, production supplies `GameBridge`.

`RoomText` substitutes `{source}` with the triggering player's **tagged** name
and sends on the **visual** channel, the way the behaviour tree's own
`room_text` does. Until 2026-09-11 it sent raw text on the audio channel, so an
observer who could not see still read it and quest 77 showed a literal
`{source}`. The tag matters: `messaging.Anonymize` strips only tagged names.
```

- [ ] **Step 2: Add the startup-check gotcha**

In the same file, replace:

```markdown
- **Ephemeral (instance) rooms match their TEMPLATE room id** in `room:`
  triggers, not their runtime id.
```

with:

```markdown
- **Ephemeral (instance) rooms match their TEMPLATE room id** in `room:`
  triggers, not their runtime id.
- **Every quest `room_text` must contain `{source}`, and `ValidateAllRoomText`
  panics at startup if one does not.** The room is watching the player act, so
  the line must say who. It also rejects `{target}` and `{target_plain}` (a
  quest has no target) and `{source_plain}` (an untagged name cannot be
  anonymized in the dark). The rules live in `roomTextProblems`, and
  `TestShippedQuestRoomTextFollowsConvention` checks the shipped files.
```

- [ ] **Step 3: Run the context.md audit**

Run: `python tools/context_md_audit.py 2>&1 | grep -i questengine`
Expected: no findings for `questengine`.

- [ ] **Step 4: Add the patch notes entry**

In `docs/PATCH_NOTES.md`, replace the first line:

```markdown
# DOGMud Patch Notes
```

with:

```markdown
# DOGMud Patch Notes

## 2026-09-11: The dark hides more, and quests say who did what

Some things were being described to you that you had no way of seeing. If a
spell or a lingering effect settled over someone while you stood blind, asleep
or in the dark, you were still told how it looked. You are not anymore. The
same goes for the small moments in quests where you watch another adventurer
search a shelf, read a ledger, or pry something loose.

Those quest moments had a stranger problem too: they never said who was doing
them. You could read "unlocks the strongbox and pulls out a leather journal"
with no one attached to it. They now name the person. One of them showed a
stray scrap of code where a name belonged, and that reads properly now as well.

Casting on yourself reads better. Healing or cleansing yourself used to tell you
about it twice, and told everyone nearby that you had cast the spell on
yourself, naming you twice over. Now you get one line and the room gets one.

A creature suffering from a lingering effect now shows it, the way a person
already did.

And the Cat's Eye Draught finally does what its label says. It was meant to let
you see in the dark, and it quietly never did.
```

- [ ] **Step 5: Check the patch notes wrap and punctuation**

Run (standalone): `awk '/^## 2026-09-09/{exit} length > 80 {print NR": "length}' docs/PATCH_NOTES.md`
Expected: no output.

Run (standalone): `awk '/^## 2026-09-09/{exit} {print}' docs/PATCH_NOTES.md | grep -c "—\|–"`
Expected: `0`.

- [ ] **Step 6: Commit**

```bash
git add internal/questengine/context.md docs/PATCH_NOTES.md
git commit -m "docs: quest room text, the startup check, and the player-facing notes"
```

---

## Task 10: Full verification, the playtest gate, and the PR

- [ ] **Step 1: Format, vet, full suite**

Run: `gofmt -l internal/ modules/ *.go`
Expected: no output.

Run: `go test ./... 2>&1 | grep -E "^(FAIL|---[[:space:]]*FAIL|panic:)"`
Expected: no output. If `internal/playtestrun` fails here but passes with `go test ./internal/playtestrun/ -count=1`, that is known full-suite contention, not a regression; confirm with `go list -deps ./internal/playtestrun/` that no package this plan changed is in its graph.

- [ ] **Step 2: The goldens did not move**

Run: `git diff --stat master -- internal/narration/testdata/stores/`
Expected: no output.

- [ ] **Step 3: Run playtest lane A (lit)**

```bash
go run ./cmd/playtestrun scenario --checkout "C:/Users/Calabe Davis/workspace/DOGMud" --scenario "C:/Users/Calabe Davis/workspace/DOGMud/tools/playtest/scenarios/m3-item5a-lit.yaml" --wall-clock 25m
```

Read the run id from `tools/playtest/.run/<run_id>/session.json` rather than piping the command's output, which buffers until exit. Confirm `dirty: false` and that `commit` equals `git rev-parse HEAD`. Drive both players through `tools/playtest/cmd/agentbridge`, reading both bridges directly so every quoted line is verbatim. Tear down with `go run ./cmd/playtestrun stop --checkout <abs> --run <run_id>`, then check `docker ps` and remove the container by its run id with `docker rm -f dogmud-playtest-<run_id>-server-1` if it survived. Never touch a container you did not start.

- [ ] **Step 4: Run playtest lane B (quest in the dark)**

Same procedure with `tools/playtest/scenarios/m3-item5a-quest-dark.yaml`. The witness's goals must run in their stated order.

- [ ] **Step 5: Run playtest lane C (buff in the dark)**

Same procedure with `tools/playtest/scenarios/m3-item5a-buff-dark.yaml`.

- [ ] **Step 6: Record the findings before the reports disappear**

Playtest reports are gitignored. Write the verbatim lines each lane produced, and every failure or surprise, into the memory file `project-messaging-m3-item5a-design.md` before continuing. State plainly that D4 was covered by unit test only.

- [ ] **Step 7: Push and open the PR**

```bash
git push -u origin feature/messaging-m3-item5a-defects
gh pr create --repo pruuk/DOGMud --base master --head feature/messaging-m3-item5a-defects --title "M3 item 5a: buff, spell and quest narration defects" --body-file <body-file>
```

Read back the URL `gh` prints and confirm it says `pruuk/DOGMud`. Then wait for every workflow with `gh run list --repo pruuk/DOGMud --branch feature/messaging-m3-item5a-defects`, because `gh pr checks --watch` can return before jobs register. Confirm `validate / lint`, `validate / test` and `validate / javascript` all succeed, and that `gh run view <id> --repo pruuk/DOGMud --log-failed` prints nothing. The PR body names the Cat's Eye Draught fix separately, so the owner sees the one gameplay change.

Do not merge without the owner's word.

---

## What this plan deliberately does not do

| Item | Where it belongs |
|---|---|
| Move buffs, spells and quests onto the narration core | 5b |
| The merged "who plus what" line for a spell buff cast on another player, which needs a caster in the buff event, a `{caster}` token and a rewrite of 17 buffs | M6 |
| The doubled narration for a spell buff whose buff has start text | M6, with the merged line |
| The `{source_plain}` anonymizer leak in buff files, including Illumination (buff 1) | M5, owner hold |
| Buffs and spells failing at boot on an unknown token instead of warning | filed |
| Validating buff flag names at load. It would also catch buff 64 Stone Stomach's `poison-immunity`, a flag no code reads, so that buff grants no immunity | filed |
| An area spell producing one room line per target | filed |
| Refusing a cast aimed by name at a player the caster cannot see, if lane C confirms it works | follow-up (owner, 2026-09-11) |
| Reworking how quests are reached and tied to rooms | the future quest arc |
