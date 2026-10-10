# Loose Issues Sweep 3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the glyphs that carry meaning an ASCII form and turn on the level-4 map markers (the #253 follow-up), give Blood Boil its own condition (#249), put `trade` and `tutorial` in the help index (#236), and fix the arrest line order and the stale arrest (#241), in one branch.

**Spec:** `docs/superpowers/specs/2026-10-10-loose-issues-sweep-3-design.md` (owner approved 2026-10-10).

**Architecture:** One task per fix, each touching its own files, then a gate task. No new packages and no new non-test Go files. Behavioural changes get a test that fails first; copy and YAML changes are checked by a guard, a build or a boot.

**Tech Stack:** Go, testify, Go `text/template` world templates, YAML content.

**Worktree:** `C:\Users\Calabe Davis\workspace\DOGMud\.claude\worktrees\loose-issues-sweep-3`, branch `fix/loose-issues-sweep-3` (from master 592dfee5f; spec commit d4d8cd3c8). Run every command from there. Never `cd` to, or run git against, the main checkout `C:\Users\Calabe Davis\workspace\DOGMud` or the other worktree `...\loose-issues-sweep`.

---

## Ground rules for every task

- **Do not touch these files** (another session's #465 is editing them): in `internal/combat/`: `damage_pipeline.go`, `crit_damage.go`, `combat_helpers.go`, `calculations.go`, `skill_moves.go`, `reflect_damage.go`; `internal/configs/config.balance*.go`; in `internal/hooks/`: `combat_shared_helpers.go`, `NewRound_DoCombat_unified.go`; in `internal/actions/`: `combat_taunt.go`, `combat_counter.go`; `internal/usercommands/throw.go`, `throw_seam_test.go`; `internal/templates/templatesfunctions.go`; the status templates; `_datafiles/config.yaml`. No task here needs one. If a step seems to, stop and report it as a blocker.
- No new balance knob. The arrest window is derived from `ArrestResistGraceRounds` (Task 10).
- The line-keyed root guard `condition_apply_path_guard_test.go` keys `internal/hooks/spell_effects.go|355` (Task 5 re-keys it) and `internal/justice/arrest.go|395` and `|645` (Task 9 keeps both lines where they are). Nothing else this plan edits is line-keyed.
- Stage named paths only. Never `git add -A` or `git add .`.
- Never edit a file with a Python read-modify-write. Use the Edit tool (or Write for a whole-file replacement after a Read). Templates and YAML are CRLF in the working tree and LF in the index (`core.autocrlf=true`); an Edit that leaves some lines LF is normalised on `git add`.
- No em dashes or en dashes in player text, comments, docs or commit messages.
- Player text: 80 columns, no raw numbers for damage, healing, armor, chances or durations.
- New tests restore every piece of shared state they change (users, user 1's party, round count, registries, room contents, MiscData, package seams) with `t.Cleanup`. Register `seedAllRegistries()` with `t.Cleanup`, not `defer`, in any test that also registers a `t.Cleanup` seed: a `defer` runs before every `t.Cleanup`, so a later seed's restore would put the test registry back. The usercommands package already has a shuffle-order problem (#440); do not add to it.
- Every commit message ends with a blank line, then:
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`
- Test command form: `go test ./internal/<pkg>/ -run '<Regex>' -count=1`.

## Facts verified against source (worktree at d4d8cd3c8; master at 9798f6d02)

Master moved one merge past the spec's base (592dfee5f -> 9798f6d02, PR #473): it added `docs/superpowers/specs/2026-10-10-mitigation-curve-design.md` and one row in `docs/README.md`. No other file this plan touches changed. Task 11 merges master first; the README row is the only possible conflict (keep both rows).

| Issue | Fact | Where |
|---|---|---|
| #253f | `ConvertToAscii` at :1135, `unicodeToAscii` (`map[rune]string`) at :1250; the `// Map / directional` rows are :1276-1278, the `'≈'` row :1278. Applied per recipient at `connectiondetails.go:316` | `internal/util/util.go` |
| #253f | `TestConvertToAscii` at :1187; the `{"unmapped high rune passthrough", ...}` row is :1228 | `internal/util/util_test.go` |
| #253f | None of ☺ ☹ ⚷ → … ± × ↑ ↓ ⌬ ♠ ♣ ∴ ⩕ ⁖ ≋ ⌇ 🕸 ♨ is in the table (scan of the table keys) | `util.go:1250-1298` |
| #253f | `mobMapSymbol(asciiMode bool) rune` (:31-36); `skillLevel` tops out at 4 (:42-49); three `if skillLevel > 4` blocks: :98 (screen-size map), :136-150 (the Mob, NPC and Player markers), :171 (empty). Party Member ☺ :163, Friend ☹ :159, You @ :168 | `internal/usercommands/skill.map.go` |
| #253f | `mapper.Config` keeps `symbolOverrides map[int]SymbolOverride` unexported; `SymbolOverride{Symbol rune; Legend string}`; `OverrideSymbol(roomId, symbol, legend)` is the only accessor | `internal/mapper/mapper.config.go:3-22` |
| #253f | Mapper glyphs: `defaultMapSymbol '•'`, `SecretSymbol '?'`, `LockedSymbol '⚷'` (:17-21); exit glyphs live in `posDeltas` (`positionDelta.arrow`) | `internal/mapper/mapper.go:17-21,41,195-200` |
| #253f | `maptag_test.go` scans `OverrideSymbol\([^`\n]*` + a backtick legend in `../usercommands/*.go`; it needs "Party Member" and "Secret" to be found | `internal/mapper/maptag_test.go:79-105` |
| #253f | Test fixture: `seedAllRegistries()` seeds room 1 (users 1, 2, mob 100 AutoAggro) and an empty room 2; `rooms.MarkRoomOccupancy`; `Room.AddMob` updates `roomsWithMobs` (rooms.go:1354) | `internal/usercommands/usercommands_test.go:107-238`, `internal/rooms/test_helpers.go:27-36` |
| #253f | Biome symbols (dogmud): Cave ⌬, Dense Forest ♠, Forest ♣, Ether ∴, Mountains ⩕, Plains ⁖, River ≋, Sewer ⌇, Spiderweb 🕸 (`"\U0001F578"`), Swamp ♨; the rest are mapped already (• ▼ ⌂ ❄ ≈) or ASCII. The default world shares eight of them | `_datafiles/world/{dogmud,default}/biomes/*.yaml` |
| #253f | Room `mapsymbol`s in dogmud: h T W R L J E C + * (all ASCII); `L` is the Loom (thornwall_city/480.yaml:10). Legend overrides `$ ★ G ✗ % ?` (+ zone `!`). The default (upstream sample) world's rooms use ♜ (35 rooms, no ASCII form) and the letters `& K P X M`, so the new forms collide there; only dogmud is live | `rooms/**`, `keywords.yaml:380-389`, `_datafiles/world/default/rooms/**` |
| #253f | ≋ also draws water in five splash templates; it reads `=` there too after Task 3 | `templates/generic/splash/*.template` |
| #253f | Truncations: `bountyTruncate` (admin.bounty.go:336-343, callers :145-146 in `%-22s`/`%-15s`), `factTruncate` (admin.fact.go:325-334, callers :112,115,116 in padded columns and :264), admin.locate.go:130,138 cut BY BYTE (`roomTitle[0:23]`) into `%-24s`; `petitionSnippet` (petitions.go:94) ends its line | as listed |
| #253f | `fmt` pads `%-Ns` by rune count, so `…` (one rune) holds a padded column in UTF-8 and `...` must replace three runes to hold it in ASCII | Go `fmt` |
| #253f | Lock grid cells `"  ↑  "` / `"  ↓  "` | `internal/usercommands/picklock.go:340,342` |
| #253f | `help map` has 12 em dashes and a U+2212 minus (web section) and lists ☺ ☠ ☹ with no ASCII form and no locked-exit marker; `help charset` says nothing about the map | `templates/help/map.template:35-79`, `charset.template` |
| #253f | `map wide` never changes anything: the screen-size block also sits under `skillLevel > 4` (:98). Not changed here (owner decision, see Notes) | `skill.map.go:98` |
| #249 | `applySpellDot` (:326-370) applies `conditions.ConditionIdPoisoned` at :355 with `tc.AddConditionMagnitudeBy(..., "spell", c.casterRef())`, then `commitHarmfulSpellAggro`, then its own trio (:360-368). File imports have no `events` | `internal/hooks/spell_effects.go:1-24,326-370` |
| #249 | Existing pattern for "the spell's own condition, else a default": `spellHealConditionId`, `spellWardConditionId` (`ConditionIds[0]` or a constant) | `internal/hooks/spell_help_effects.go:271-317` |
| #249 | `spellConditionNarratesStart(c, id)`: spec exists, target does not hold it, `spec.NarratesCastStart(c.selfCast())` | `spell_help_effects.go:141-146` |
| #249 | `narrateConditionStart(spec, evt events.Condition, holder conditionParty, snap rooms.VisualSnapshot, hideOnHidden bool, unseenBy []int)`; `conditionPartyOf(state.ActorRef) (conditionParty, bool)`; `conditionLineUnseenBy(*rooms.Room, *characters.Character) []int`. The apply hook reads `unseenBy` before the add (Condition_ApplyConditions.go:97) | `internal/hooks/condition_cast_lines.go:57-200`, `Condition_ApplyConditions.go:97,168,245` |
| #249 | `events.Condition` fields: `UserId, MobInstanceId, ConditionId, Source, ..., Caster state.ActorRef, CasterCrit bool` | `internal/events/eventtypes.go:21-52` |
| #249 | Only `blood-boil.yaml` and `neural-toxin.yaml` have `effect_type: dot`; 29 spells carry `condition_ids:` | `_datafiles/world/dogmud/spells/` |
| #249 | 121 Poisoned is `poison` + `silent-start` (so Neural Toxin keeps its trio); 122 Bleeding is `bleeding` + `silent-start` + `stacking`. Ids in use 0-3, 5-7, 9, 15, 24-142; 143 is free on master | `conditions/121-*.yaml`, `122-*.yaml`, `ls conditions/` |
| #249 | A condition's filename must be `<id>-<ConvertForFilename(name)>.yaml`; `triggercount` is required or the record is born expired (`TestNoShippedConditionIsBornDead`) | `internal/conditions/conditionspec.go:449-452`, `born_dead_guard_test.go` |
| #249 | `GetAdjectives` adds `poisoned` for the Poison flag (:166-168), nothing for Bleeding. `adjectiveStyles` styles `poisoned` (purple) | `internal/characters/description.go:143-172`, `formattedname.go:24-33` |
| #249 | Purge Affliction (:101) and Cleansing Wave's `applySpellPurge` (:331) both call `CancelConditionsWithFlag(conditions.Poison)`. A cancel only EXPIRES a record; `HasCondition` stays true until the prune, `GetConditions` filters expired ones | `internal/hooks/spell_purgeaffliction.go:101`, `spell_help_effects.go:331`, `purge_target_test.go:72-82` |
| #249 | `Character.RemoveCondition(id)` expires one record (end line at the prune) | `internal/characters/conditions.go:291-296` |
| #249 | No antidote removes 121 (Minor Antidote removes 39, 40, 78); only the two purges cure Blood Boil today | `conditions/47-minor_antidote.yaml:13-16` |
| #249 | Guards: `conditionIdReaders = {"condition","shield","heal"}` (:79), used at :90 and :258; key `"internal/hooks/spell_effects.go|355"` (:219); `observerIdentityGuardContentSafeViaCode` lists 30 and 41 (:1075,:1082) | root `spell_condition_data_guard_test.go`, `condition_apply_path_guard_test.go`, `shipped_narration_data_guard_test.go` |
| #249 | Test helpers in package hooks: `seedAllRegistries`, `seedCastObserver(t)` (user 3 Orin in room 1), `drainPlain(uid)`, `countContaining`, `pinSpellContest(t)`, `spellContestAttackWin()`, `newSpellEffectCtx(casterChar, casterActor, targetActor, room, spell, magnitude, out)`, `litRoomOneWithHiddenSkeleton(t)`, `requireUnnamed`, `tickMobConditions(mob, id)`, `resolvePurgeAffliction`, `purgeTarget` | `internal/hooks/*_test.go` |
| #249 | Line normalisation capitalises a sentence start (stage `stageCapitalize`) for `CategoryConditionApply`, so a hidden caster's `{actor}'s spell` reads "Something's spell" | `internal/messaging/normalize.go:14-55`, `pipeline.go:57` |
| #236 | `help` lists `keywords.GetAllHelpTopicInfo()`; a scan of all 192 `userCommands` finds exactly two non-admin commands with a help file and no index entry: `trade` (usercommands.go:76) and `tutorial` (:232) | `internal/usercommands/help.go:49`, `keywords.yaml:5-160` |
| #236 | `chat` is under `communication` (:47), `newbie` under `general` (:156); no help alias is `trade` or `tutorial` | `keywords.yaml` |
| #236 | `trade.template:4` carries the em dash | `templates/help/trade.template:4` |
| #241 | `if u := users.GetByUserId(userId); u != nil {` at :407; the condition-88 block :408-415 and the seizure block (comment + send) :416-424 are nine lines each, so swapping them keeps :395 and :645 | `internal/justice/arrest.go:395,407-424,645` |
| #241 | Each guard stamps `justice_arrest_pending_<uid>` on its own MiscData and declares when it has none (:209-231); `pruneStaleWarnStamps` skips those keys (:114-130), so they never expire | `internal/justice/enforce.go` |
| #241 | `ArrestResistGraceRounds` is unset in config.yaml; Go default 3 (`config.balance.mobs.go:247-248`, off-limits); `arrestGraceRounds()` reads it | `enforce.go:100-107`, `git show HEAD:_datafiles/config.yaml` |
| #241 | Guards run `RunGuardEnforcement` every round from `NewRound_MobRoundTick.go:135` | as listed |
| #241 | `factions.FactionsForMob` needs loaded faction definitions, and `factions` has no exported test seed; justice's other reads are package seams (`openFactionBountyFn`, `cellRoomFn`, `guardSayFn`, `executeArrestFn`) | `internal/factions/factions.go:150-161`, `internal/justice/justice.go:28-83` |
| #241 | Mob MiscData never reaches disk: `MobInstanceData` persists only `plan:` keys (`collectPlanState`), and copyover carries no mob state. Old per-guard stamps vanish on the next restart, so no migration is needed | `internal/mobs/instance_save.go:25-55,336-355` |
| #241 | Player MiscData persists in the user save and round-trips YAML numbers; `miscDataRound` reads uint64/int64/int/float64 | `enforce.go:69-85` |

---

### Task 1: #253 follow-up, ASCII forms for map markers and text glyphs

**Model: haiku**

**Files:**
- Modify: `internal/util/util.go` (after the `'≈'` row, :1278)
- Modify: `internal/util/util_test.go` (rows in `TestConvertToAscii`)
- Modify: `internal/util/context.md`

- [ ] **Step 1: Write the failing rows**

In `TestConvertToAscii`, directly before the `{"unmapped high rune passthrough", "café", "café"},` row, add:

```go
		// #253 follow-up: glyphs that carry meaning get an ASCII form. A map
		// marker becomes one letter, so a map cell keeps its width.
		{"map markers", "@☺☹⚷", "@P&K"},
		{"arrow", "Novice → Adept", "Novice -> Adept"},
		{"ellipsis", "a long name…", "a long name..."},
		{"plus or minus", "~12 ±3", "~12 +/-3"},
		{"times", "rate 1.50×", "rate 1.50x"},
		{"lock grid cells keep their width", "  ↑  |  ↓  ", "  ^  |  v  "},
```

- [ ] **Step 2: Run it and confirm the new rows fail**

Run: `go test ./internal/util/ -run TestConvertToAscii -count=1`
Expected: the six new subtests FAIL; every old row PASSES.

- [ ] **Step 3: Add the table rows**

In `unicodeToAscii`, directly after the line

```go
	'≈': "~", '⌂': "#", '◆': "*", '●': "o", '○': "o",
```

add:

```go
	// Map markers that carry meaning (#253 follow-up): one letter each, so a
	// cell keeps its width. P and K, not @ and L: @ is You and L is the
	// Loom's room symbol (owner call 2026-10-10).
	'☺': "P", '☹': "&", '⚷': "K",
	// Text glyphs that carry meaning (#253 follow-up): the skill banner and
	// admin arrows, truncation ellipses, a weapon's damage spread, admin
	// multipliers, and the lock grid's up and down pins.
	'→': "->", '…': "...", '±': "+/-", '×': "x", '↑': "^", '↓': "v",
```

Run `gofmt -w internal/util/util.go`.

- [ ] **Step 4: Document it**

In `internal/util/context.md`, after the sentence ending `dropping them would glue two words together (#253).`, add:

```
Glyphs that carry meaning get a visible form (#253 follow-up): the map
markers `☺ ☹ ⚷` become `P & K`, `→` becomes `->`, `…` `...`, `±` `+/-`,
`×` `x`, and `↑ ↓` `^ v`. A map cell must convert to exactly one ASCII
character or its row shifts; `internal/mapper`'s
`TestMapGlyphsConvertToOneAsciiCharacter` holds every biome and mapper glyph
to that.
```

(The mapper test lands in Task 3; the sentence is true once the branch is complete.)

- [ ] **Step 5: Run the util tests and the ASCII readers**

Run: `go test ./internal/util/ ./internal/users/ ./internal/connections/ -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/util/util.go internal/util/util_test.go internal/util/context.md
git commit -m "fix(ascii): map markers, arrows, ellipses and pins get ASCII forms (#253 follow-up)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: #253 follow-up, padded admin columns end a cut in `...`

**Model: haiku**

**Files:**
- Modify: `internal/usercommands/admin.bounty.go:336-343`
- Modify: `internal/usercommands/admin.fact.go:325-334`
- Modify: `internal/usercommands/admin.locate.go:128-139`
- Create: `internal/usercommands/admin_truncate_test.go`

- [ ] **Step 1: Write the failing test**

```go
package usercommands

import (
	"testing"
	"unicode/utf8"

	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
)

// #253 follow-up: a padded admin column cut its text with "…", one rune, so
// the column held in UTF-8 but an ASCII client, which reads "...", saw the
// row pushed two columns wide. The cut now ends in "..." itself, three runes
// short, and holds in both charsets.
func TestAdminColumnTruncation_EndsInThreeDotsAndHoldsTheWidth(t *testing.T) {
	for name, cut := range map[string]func(string, int) string{
		"bountyTruncate": bountyTruncate,
		"factTruncate":   factTruncate,
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, "short", cut("short", 10), "text that fits is untouched")
			assert.Equal(t, "exactlyten", cut("exactlyten", 10), "text at the width is untouched")

			got := cut("abcdefghijklmnop", 10)
			assert.Equal(t, "abcdefg...", got)
			assert.Equal(t, 10, utf8.RuneCountInString(got))
			assert.Equal(t, got, util.ConvertToAscii(got), "the cut is plain ASCII")

			// Cut by rune, never inside one.
			got = cut("ÄÖÜäöüßÄÖÜäöüß", 10)
			assert.True(t, utf8.ValidString(got))
			assert.Equal(t, 10, utf8.RuneCountInString(got))

			assert.Equal(t, "abc", cut("abcdefgh", 3), "a width of three or less has no room for dots")
		})
	}
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test ./internal/usercommands/ -run TestAdminColumnTruncation -count=1`
Expected: FAIL (the cuts end in `…`).

- [ ] **Step 3: Change the two helpers**

In `admin.bounty.go`, replace:

```go
// bountyTruncate shortens s to at most maxLen runes, appending "…" if trimmed.
func bountyTruncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}
```

with:

```go
// bountyTruncate shortens s to at most maxLen runes, ending a cut in "...".
// ASCII dots, not "…": the column is padded by rune count, and an ASCII
// client reads "…" as three characters, which pushed a cut row two columns
// wide (#253 follow-up).
func bountyTruncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}
```

In `admin.fact.go`, replace:

```go
// factTruncate shortens s to at most n runes, appending "…" if trimmed.
// Named with a fact prefix to avoid collision with similarly-named
// helpers in other admin command files in this package.
func factTruncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}
```

with:

```go
// factTruncate shortens s to at most n runes, ending a cut in "..." (ASCII,
// so a padded column holds in either charset; #253 follow-up). Named with a
// fact prefix to avoid collision with similarly-named helpers in other admin
// command files in this package.
func factTruncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n <= 3 {
		return string(runes[:n])
	}
	return string(runes[:n-3]) + "..."
}
```

- [ ] **Step 4: Cut `admin locate` by rune**

In `admin.locate.go`, replace:

```go
				// trunacte room.Title to only 20 chars
				roomTitle := room.Title
				if len(roomTitle) > 24 {
					roomTitle = roomTitle[0:23] + `…`
				}

				mobName := mob.Character.Name
				if mob.Character.IsInCombat() {
					mobName = `*` + mobName
				}
				if len(mobName) > 24 {
					mobName = mobName[0:23] + `…`
				}
```

with:

```go
				// Both cells are padded to 24 runes. Cut by rune (a byte cut
				// could split one) and end in ASCII dots, so the column holds
				// for a UTF-8 and an ASCII client alike (#253 follow-up).
				roomTitle := room.Title
				if r := []rune(roomTitle); len(r) > 24 {
					roomTitle = string(r[:21]) + `...`
				}

				mobName := mob.Character.Name
				if mob.Character.IsInCombat() {
					mobName = `*` + mobName
				}
				if r := []rune(mobName); len(r) > 24 {
					mobName = string(r[:21]) + `...`
				}
```

`petitions.go:94` keeps `…`: its snippet ends the line, so nothing after it shifts.

- [ ] **Step 5: Run the tests and the admin suites**

Run: `go test ./internal/usercommands/ -run 'TestAdminColumnTruncation|Bounty|Fact|Locate' -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/usercommands/admin.bounty.go internal/usercommands/admin.fact.go internal/usercommands/admin.locate.go internal/usercommands/admin_truncate_test.go
git commit -m "fix(admin): padded columns end a cut in three dots, cut by rune (#253 follow-up)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: #253 follow-up, biome letters, `help map` and `help charset`

**Model: sonnet**

**Files:**
- Modify: `internal/util/util.go` (after Task 1's marker rows)
- Modify: `internal/util/util_test.go`
- Create: `internal/mapper/ascii_glyph_test.go`
- Modify: `internal/usercommands/skill.map_symbol_test.go`
- Modify: `_datafiles/world/dogmud/templates/help/map.template` (whole file)
- Modify: `_datafiles/world/dogmud/templates/help/charset.template`

- [ ] **Step 1: Write the failing tests**

Create `internal/mapper/ascii_glyph_test.go`:

```go
package mapper

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// #253 follow-up: an ASCII-mode map is converted cell by cell with the rest
// of the line, so a glyph that converts to nothing, or to several
// characters, shifts its row and breaks the frame. Every glyph a map can
// draw must convert to exactly one ASCII character: each biome symbol in
// both worlds, each room mapsymbol in the live dogmud world, the mapper's
// own symbols and its exit lines. (The command's markers are held to it in
// usercommands.) The default world's rooms are upstream sample content and
// use ♜, which has no ASCII form; they are not scanned.
func TestMapGlyphsConvertToOneAsciiCharacter(t *testing.T) {
	glyphs := map[string]string{
		"default symbol": string(defaultMapSymbol),
		"secret":         string(SecretSymbol),
		"locked":         string(LockedSymbol),
	}
	for dir, d := range posDeltas {
		glyphs["exit "+dir] = string(d.arrow)
	}
	for _, world := range []string{"default", "dogmud"} {
		root := filepath.Join("..", "..", "_datafiles", "world", world)

		biomes, err := filepath.Glob(filepath.Join(root, "biomes", "*.yaml"))
		require.NoError(t, err)
		require.NotEmpty(t, biomes, world)
		for _, f := range biomes {
			b, err := os.ReadFile(f)
			require.NoError(t, err)
			var biome struct {
				Symbol string `yaml:"symbol"`
			}
			require.NoError(t, yaml.Unmarshal(b, &biome), f)
			if biome.Symbol != "" {
				glyphs[world+" biome "+filepath.Base(f)] = biome.Symbol
			}
		}

		if world != "dogmud" {
			continue
		}
		roomFiles, err := filepath.Glob(filepath.Join(root, "rooms", "*", "*.yaml"))
		require.NoError(t, err)
		for _, f := range roomFiles {
			b, err := os.ReadFile(f)
			require.NoError(t, err)
			if !strings.Contains(string(b), "mapsymbol:") {
				continue
			}
			var room struct {
				MapSymbol string `yaml:"mapsymbol"`
			}
			require.NoError(t, yaml.Unmarshal(b, &room), f)
			if room.MapSymbol != "" {
				glyphs[world+" room "+filepath.Base(f)] = room.MapSymbol
			}
		}
	}
	// The scan must reach the glyphs it guards.
	require.Contains(t, glyphs, "dogmud biome cave.yaml")
	require.Contains(t, glyphs, "dogmud room 480.yaml")

	for name, g := range glyphs {
		got := util.ConvertToAscii(g)
		assert.True(t, len(got) == 1 && got[0] < 0x80,
			"%s: %q converts to %q, not one ASCII character", name, g, got)
	}
}
```

Append to `internal/usercommands/skill.map_symbol_test.go`:

```go
// #253 follow-up: the markers the map command draws convert to one ASCII
// character each, so an ASCII map cell keeps its width.
func TestMapCodeMarkersConvertToOneAsciiCharacter(t *testing.T) {
	for name, r := range map[string]rune{
		"you": '@', "player, npc, party member": '☺', "friend": '☹', "mob": mobMapSymbol(true),
	} {
		got := util.ConvertToAscii(string(r))
		if len(got) != 1 || got[0] >= 0x80 {
			t.Errorf("%s: %q converts to %q, not one ASCII character", name, r, got)
		}
	}
}
```

In `TestConvertToAscii`, directly after Task 1's `{"lock grid cells keep their width", ...}` row, add:

```go
		{"biome glyphs", "⌬♠♣∴⩕⁖≋⌇🕸♨", "OFf:M.=sXw"},
```

- [ ] **Step 2: Run them and confirm they fail**

Run: `go test ./internal/mapper/ -run TestMapGlyphsConvertToOneAsciiCharacter -count=1`
Expected: FAIL naming the ten biome files (cave, dense_forest, forest, ether, mountains, plains, river, sewer, spiderweb, swamp; the default world's copies too). The locked symbol and every exit line PASS (Task 1). If a ROOM mapsymbol fails, stop and report it: the spec found none.

Run: `go test ./internal/util/ -run 'TestConvertToAscii/biome' -count=1`
Expected: FAIL

Run: `go test ./internal/usercommands/ -run TestMapCodeMarkersConvertToOneAsciiCharacter -count=1`
Expected: PASS already (Task 1 added ☺ and ☹); it stays as the guard.

- [ ] **Step 3: Add the biome rows**

In `unicodeToAscii`, directly after Task 1's `'☺': "P", '☹': "&", '⚷': "K",` row, add:

```go
	// Biome glyphs (#253 follow-up, owner approved 2026-10-10): one letter
	// each, none shared with a room mapsymbol, a map marker or a legend
	// override. Cave, Dense Forest, Forest, Ether, Mountains, Plains, River,
	// Sewer, Spiderweb, Swamp. ≋ also draws water in the splash art.
	'⌬': "O", '♠': "F", '♣': "f", '∴': ":", '⩕': "M",
	'⁖': ".", '≋': "=", '⌇': "s", '🕸': "X", '♨': "w",
```

Run `gofmt -w internal/util/util.go`.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/mapper/ ./internal/util/ -count=1`
Expected: PASS

- [ ] **Step 5: Rewrite `help map`**

Read `_datafiles/world/dogmud/templates/help/map.template`, then replace the whole file with (no em dashes, the minus sign is a hyphen, an ASCII column, the locked-exit marker, and a `help charset` link):

```
<ansi fg="black-bold">.:</ansi> <ansi fg="magenta">Help for </ansi><ansi fg="skill">map</ansi> (skill)

<ansi fg="yellow">The in-game ASCII map:</ansi>

The <ansi fg="skill">map</ansi> command draws a text map of the area centred on your
character. Each room appears as a glyph; connecting exits are drawn
as lines between them.

Your position and what you can see scale with your Perception stat.
Higher Perception expands the visible range and reveals more detail.
Biome glyphs mark terrain types (forest, dungeon, road, etc.), and
any room with a custom map symbol uses that symbol instead. A legend
below the map explains every glyph that appears.

Hidden areas and secret passages may not appear on the map.

<ansi fg="yellow">Map symbols:</ansi>

  UTF-8  ASCII
  <ansi fg="white">@</ansi>      <ansi fg="white">@</ansi>      You (your current room)
  <ansi fg="white">☺</ansi>      <ansi fg="white">P</ansi>      A party member, and with keen Perception
                a player or a peaceful NPC
  <ansi fg="white">☠</ansi>      <ansi fg="white">!</ansi>      A hostile or fighting mob (keen Perception)
  <ansi fg="white">☹</ansi>      <ansi fg="white">&</ansi>      A friend (a charmed mob travelling with
                a party member)
  <ansi fg="white">⚷</ansi>      <ansi fg="white">K</ansi>      A locked exit
  <ansi fg="white">?</ansi>      <ansi fg="white">?</ansi>      A secret passage you have found

With <ansi fg="command">set charset</ansi> on ASCII the map draws the second column,
and terrain glyphs become letters too. The legend names each one.

<ansi fg="yellow">Usage:</ansi>

  <ansi fg="skill">map</ansi>        Display a map of the nearby area.
  <ansi fg="skill">map wide</ansi>   Display a zoomed-out overview
                 (requires advanced mapping skill /
                 high Perception).

<ansi fg="yellow">The web client map:</ansi>

Players using the browser-based web client also have a graphical
"leather map" panel: an aged-parchment map that fills in as you
explore the world (fog of war). Rooms you have not yet visited are
hidden; rooms further from you appear slightly dimmed.

<ansi fg="yellow">Your position:</ansi>
  Your current room is the raised tile at the centre of the map.
  The view follows you as you move. Hover any visible room tile
  to see its name in a tooltip.

<ansi fg="yellow">Markers on rooms:</ansi>
  <ansi fg="white">$</ansi>   Bank: deposit and withdraw gold here
  <ansi fg="white">S</ansi>   Shop: a merchant or vendor
  <ansi fg="white">T</ansi>   Trainer: a skill instructor or master
  <ansi fg="white">▢</ansi>   Storage: a storeroom or money-pouch
  A small figure on a room means a party member is standing there.

<ansi fg="yellow">Vertical connections (stairs):</ansi>
  <ansi fg="white">▲</ansi>   Stairs up to the floor above
  <ansi fg="white">▼</ansi>   Stairs down to the floor below
  The map shows one floor (Z-level) at a time. Walk to a staircase
  and use it to change levels; the map will update to show the
  new floor.

<ansi fg="yellow">Paths between rooms:</ansi>
  The lines connecting rooms change style to show terrain type:
  - Solid line: paved road or corridor
  - Dashed line: forest trail, marsh path, or open ground
  - Dashed blue line: river, waterway, or canal
  - Rocky or jagged: mountain ridge or cliff path
  - Long dashed: highway or long-distance connection

<ansi fg="yellow">Special exits:</ansi>
  A small door icon (with the path turning red beyond it) means a
  locked exit: you need a key or the right skill to pass.

  A faint dotted path ending in a <ansi fg="white">?</ansi> is a secret passage. It
  appears only once you have discovered it.

  An arrowhead on a path marks a one-way passage: you can travel
  in one direction only.

  A small archway indicates a gate or zone threshold.

  A short dashed stub pointing off the edge of a room tile is an
  unexplored exit: the area exists but you have not been there
  yet. When the exit leads to a different zone it is labelled
  with an arrow and the zone name (e.g. "→ Ironwind Steppe").

<ansi fg="yellow">Map controls (bottom-left corner of the panel):</ansi>
  <ansi fg="command">fit</ansi>   Zoom to fit the entire known map in the panel.
  <ansi fg="command">ctr</ansi>   Re-centre the view on your current room.
  <ansi fg="command">-</ansi> / <ansi fg="command">+</ansi>  Zoom out / zoom in one step.

<ansi fg="yellow">Related:</ansi>
  <ansi fg="command">help map-examples</ansi>
  <ansi fg="command">help perception</ansi>
  <ansi fg="command">help charset</ansi>
```

The `map wide` lines are left as they were; see the Notes at the end of this plan.

- [ ] **Step 6: Point `help charset` at the map**

In `charset.template`, replace:

```
  your client shows garbled characters in tables, status bars,
  or the message of the day.
```

with:

```
  your client shows garbled characters in tables, status bars,
  or the message of the day. Map markers and terrain glyphs
  become letters, so a map keeps its shape; <ansi fg="command">help map</ansi>
  lists them.
```

- [ ] **Step 7: Check the copy and run the help suites**

Run: `grep -n "[—–−]" _datafiles/world/dogmud/templates/help/map.template _datafiles/world/dogmud/templates/help/charset.template` (want no output; a no-match grep exits 1, so run it on its own)
Read both files once against `dogmud-player-copy`: no line over 80 visible columns.
Run: `go test ./internal/templates/ ./internal/usercommands/ -run 'Help|U8' -count=1`
Expected: PASS (the cross-reference guard resolves `help charset`; the link graph grows by one).

- [ ] **Step 8: Commit**

```bash
git add internal/util/util.go internal/util/util_test.go internal/mapper/ascii_glyph_test.go internal/usercommands/skill.map_symbol_test.go _datafiles/world/dogmud/templates/help/map.template _datafiles/world/dogmud/templates/help/charset.template
git commit -m "fix(map): biome glyphs become letters on an ASCII map; help map lists the ASCII forms (#253 follow-up)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: #253 follow-up, a level-4 map draws the NPC, Player and Mob markers

**Model: sonnet**

**Files:**
- Modify: `internal/mapper/mapper.config.go`
- Modify: `internal/usercommands/skill.map.go:136-150`
- Modify: `internal/usercommands/skill.map_symbol_test.go`
- Modify: `internal/mapper/context.md`

- [ ] **Step 1: Add the read accessor**

In `internal/mapper/mapper.config.go`, after `OverrideSymbol`, add:

```go
// SymbolOverrideAt reports the symbol and legend forced on roomId, if any.
func (c *Config) SymbolOverrideAt(roomId int) (SymbolOverride, bool) {
	o, ok := c.symbolOverrides[roomId]
	return o, ok
}
```

- [ ] **Step 2: Write the failing test**

Append to `internal/usercommands/skill.map_symbol_test.go` (add imports `internal/conditions`, `internal/mapper`, `internal/mobs`, `internal/rooms`, and testify `assert` and `require`):

```go
// #253 follow-up: the NPC, Player and Mob markers sat under skillLevel > 4,
// which no Perception tier reaches, so no map drew them although `help map`
// lists them (owner call 2026-10-10: level 4).
func TestMapOccupantMarkers_DrawnFromLevelFour(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	// Room 1 holds Aliceia, Bobrick and the hostile Skeleton; a peaceful
	// Merchant joins room 2.
	merchant := &mobs.Mob{MobId: 2, InstanceId: 9253, HomeRoomId: 2}
	merchant.Character.Name = "Merchant"
	merchant.Character.Conditions = conditions.New()
	t.Cleanup(mobs.SeedMobsForTest(nil, map[int]*mobs.Mob{100: mobs.GetInstance(100), 9253: merchant}))
	rooms.LoadRoom(2).AddMob(9253)

	for _, level := range []int{1, 2, 3} {
		c := mapper.Config{}
		addOccupantMarkers(&c, level, false)
		_, npc := c.SymbolOverrideAt(2)
		_, player := c.SymbolOverrideAt(1)
		assert.False(t, npc || player, "a level-%d map draws no occupant marker", level)
	}

	c := mapper.Config{}
	addOccupantMarkers(&c, 4, false)
	npc, ok := c.SymbolOverrideAt(2)
	require.True(t, ok, "a level-4 map marks the room holding the Merchant")
	assert.Equal(t, mapper.SymbolOverride{Symbol: '☺', Legend: "NPC"}, npc)
	player, ok := c.SymbolOverrideAt(1)
	require.True(t, ok, "a level-4 map marks the room holding players")
	assert.Equal(t, mapper.SymbolOverride{Symbol: '☺', Legend: "Player"}, player,
		"players are marked last, so they win over the Skeleton in the same room")
}
```

- [ ] **Step 3: Run it and confirm it fails**

Run: `go test ./internal/usercommands/ -run TestMapOccupantMarkers -count=1`
Expected: build FAIL, `undefined: addOccupantMarkers`.

- [ ] **Step 4: Extract the markers and turn them on at level 4**

In `skill.map.go`, replace:

```go
	if skillLevel > 4 {
		for _, rid := range rooms.GetRoomsWithMobs() {
			if roomInfo := rooms.LoadRoom(rid); roomInfo != nil {
				if len(roomInfo.GetMobs(rooms.FindFighting|rooms.FindHostile)) > 0 {
					c.OverrideSymbol(rid, mobMapSymbol(user.AsciiMode), `Mob`)
				} else {
					c.OverrideSymbol(rid, '☺', `NPC`)
				}
			}
		}

		for _, rid := range rooms.GetRoomsWithPlayers() {
			c.OverrideSymbol(rid, '☺', `Player`)
		}
	}
```

with:

```go
	addOccupantMarkers(&c, skillLevel, user.AsciiMode)
```

and add, below `mobMapSymbol`:

```go
// occupantMarkerLevel is the map level that draws the NPC, Player and Mob
// markers. They sat under skillLevel > 4, which no Perception tier reaches,
// so no map drew them although `help map` lists them (#253 follow-up; owner
// call 2026-10-10).
const occupantMarkerLevel = 4

// addOccupantMarkers marks, on a map of occupantMarkerLevel or higher, every
// room holding mobs (Mob when one is hostile or fighting, NPC otherwise) and
// every room holding players (Player). Players are marked last, so a room
// holding both reads Player; the party, friend and You markers the caller
// adds afterwards override all three.
func addOccupantMarkers(c *mapper.Config, skillLevel int, asciiMode bool) {
	if skillLevel < occupantMarkerLevel {
		return
	}
	for _, rid := range rooms.GetRoomsWithMobs() {
		if roomInfo := rooms.LoadRoom(rid); roomInfo != nil {
			if len(roomInfo.GetMobs(rooms.FindFighting|rooms.FindHostile)) > 0 {
				c.OverrideSymbol(rid, mobMapSymbol(asciiMode), `Mob`)
			} else {
				c.OverrideSymbol(rid, '☺', `NPC`)
			}
		}
	}
	for _, rid := range rooms.GetRoomsWithPlayers() {
		c.OverrideSymbol(rid, '☺', `Player`)
	}
}
```

Leave the other two `skillLevel > 4` blocks (the screen-size block at :98 and the empty one at :171) as they are (see Notes).

- [ ] **Step 5: Document the accessor and the markers**

In `internal/mapper/context.md`, in the `### Rendering` code block, after `func (c *Config) OverrideSymbol(roomId int, symbol rune, legend string)` add:

```go
func (c *Config) SymbolOverrideAt(roomId int) (SymbolOverride, bool)
```

and replace the ASCII legend table:

```
| Symbol | Meaning              |
|--------|----------------------|
| `@`    | You (current room)   |
| `☺`    | Player / Party / NPC |
| `☠`    | Hostile mob          |
| `☹`    | Friendly NPC         |
| *(biome/mapsymbol)* | Room terrain glyph |
```

with:

```
| Symbol | ASCII | Meaning |
|--------|-------|---------|
| `@`    | `@`   | You (current room) |
| `☺`    | `P`   | Party member; from map level 4, a player or a peaceful NPC |
| `☠`    | `!`   | Hostile or fighting mob, from map level 4 (`mobMapSymbol`) |
| `☹`    | `&`   | Friend (a party member's charmed mob) |
| `⚷`    | `K`   | Locked exit (`LockedSymbol`) |
| *(biome/mapsymbol)* | one letter | Room terrain glyph |

Every glyph a map draws must convert to one ASCII character
(`TestMapGlyphsConvertToOneAsciiCharacter`); the ASCII forms live in
`util.unicodeToAscii`, except the mob cell, which `mobMapSymbol` picks at
the source because `☠` is dropped elsewhere.
```

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/usercommands/ -run 'TestMap' -count=1`
Run: `go test ./internal/mapper/ -count=1`
Expected: PASS (`TestEveryMultiWordLegendHasSlugAlias` still finds "Party Member" through the unchanged `OverrideSymbol` call).

- [ ] **Step 7: Commit**

```bash
git add internal/mapper/mapper.config.go internal/mapper/context.md internal/usercommands/skill.map.go internal/usercommands/skill.map_symbol_test.go
git commit -m "fix(map): a level-4 map draws the NPC, player and mob markers (#253 follow-up)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: #249 Blood Boil lands its own condition and narrates through it

**Model: sonnet**

**Files:**
- Create: `_datafiles/world/dogmud/conditions/143-boiling_blood.yaml`
- Modify: `_datafiles/world/dogmud/spells/blood-boil.yaml`
- Modify: `_datafiles/world/dogmud/templates/help/blood-boil.template`
- Modify: `internal/hooks/spell_effects.go` (imports, `applySpellDot`, new `spellDotConditionId`)
- Modify: `internal/spells/spells.go:50` (field comment)
- Create: `internal/hooks/blood_boil_test.go`
- Modify: root `spell_condition_data_guard_test.go`, `condition_apply_path_guard_test.go`, `shipped_narration_data_guard_test.go`
- Modify: `internal/hooks/context.md`, `internal/spells/context.md`

- [ ] **Step 1: Write the failing tests**

Create `internal/hooks/blood_boil_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// boilingBloodId is Blood Boil's own condition (#249), the id its spell YAML
// names in condition_ids. The spec below mirrors
// _datafiles/world/dogmud/conditions/143-boiling_blood.yaml.
const boilingBloodId = 143

// seedBoilingBlood replaces the condition registry with 143 and adds the
// shipped records (121 Poisoned, 122 Bleeding) on top. Call it after
// t.Cleanup(seedAllRegistries()) so the restores run in reverse.
func seedBoilingBlood(t *testing.T) {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		boilingBloodId: {ConditionId: boilingBloodId, Name: "Boiling Blood",
			TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10,
			Flags:    []conditions.Flag{conditions.Bleeding},
			TickPool: "health", TickFromMagnitude: true,
			StartActorText:  "You set the blood in {actee}'s veins boiling.",
			StartUserText:   "{actor}'s spell sets your blood boiling in your veins!",
			StartRoomText:   "{actor}'s spell sets {actee}'s blood boiling.",
			TriggerUserText: `<ansi fg="red">Your blood boils and seeps from your skin!</ansi>`,
			EndUserText:     "Your blood cools at last and stops seeping."},
	}))
	t.Cleanup(conditions.SeedConditionRecordsForTest())
}

func bloodBoilTestSpell() *spells.SpellData {
	return &spells.SpellData{SpellId: "blood-boil", Name: "Blood Boil",
		AttackType: combatvocab.AttackSpell, DamageType: combatvocab.DamagePhysical, Targeting: combatvocab.TargetSingle,
		EffectType: "dot", EffectMagnitude: 40, BaseFolds: 6, ConditionIds: []int{boilingBloodId}}
}

// Neural Toxin names no condition, so it keeps 121 Poisoned and its trio.
func neuralToxinTestSpell() *spells.SpellData {
	return &spells.SpellData{SpellId: "neural-toxin", Name: "Neural Toxin",
		AttackType: combatvocab.AttackSpell, DamageType: combatvocab.DamageMental, Targeting: combatvocab.TargetSingle,
		EffectType: "dot", EffectMagnitude: 30, BaseFolds: 5}
}

// castOnBobrick has Aliceia (1) cast spell on Bobrick (2) in lit room 1, with
// Orin (3) watching, and returns what each of the three read.
func castOnBobrick(t *testing.T, spell *spells.SpellData) (caster, holder, room []string) {
	t.Helper()
	seedCastObserver(t)
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90) // pin fully lit
	drainCastParties()
	u1, u2, r := users.GetByUserId(1), users.GetByUserId(2), rooms.LoadRoom(1)
	applySpellEffect(newSpellEffectCtx(u1.Character, actions.NewUserActorInRoom(u1, r),
		actions.NewUserActorInRoom(u2, r), r, spell, 10, spellContestAttackWin()))
	return drainPlain(1), drainPlain(2), drainPlain(3)
}

// #249: Blood Boil landed 121 Poisoned, so it read as poison everywhere. It
// lands its own 143, and each audience reads that record's start line once,
// in place of the spell's generic "afflicts" trio.
func TestBloodBoil_LandsBoilingBloodAndTellsEachAudienceOnce(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	seedBoilingBlood(t)

	caster, holder, room := castOnBobrick(t, bloodBoilTestSpell())

	target := users.GetByUserId(2).Character
	recs := target.GetConditions(boilingBloodId)
	require.Len(t, recs, 1, "Blood Boil lands Boiling Blood")
	assert.Less(t, recs[0].TickAmount, 0, "and it harms")
	assert.Empty(t, target.GetConditions(conditions.ConditionIdPoisoned), "not Poisoned")

	assert.Equal(t, 1, countContaining(caster, "You set the blood in Bobrick's veins boiling."), "%v", caster)
	assert.Equal(t, 1, countContaining(holder, "Aliceia's spell sets your blood boiling in your veins!"), "%v", holder)
	assert.Equal(t, 1, countContaining(room, "Aliceia's spell sets Bobrick's blood boiling."), "%v", room)
	for _, lines := range [][]string{caster, holder, room} {
		assert.Zero(t, countContaining(lines, "afflicts"), "the generic trio is not sent too: %v", lines)
	}
}

// Neural Toxin names no condition: it lands 121 and reads exactly as before.
func TestNeuralToxin_StillLandsPoisonedWithItsOwnLines(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	seedBoilingBlood(t)

	caster, holder, room := castOnBobrick(t, neuralToxinTestSpell())

	target := users.GetByUserId(2).Character
	require.Len(t, target.GetConditions(conditions.ConditionIdPoisoned), 1)
	assert.Empty(t, target.GetConditions(boilingBloodId))
	assert.Equal(t, 1, countContaining(caster, "Your Neural Toxin afflicts Bobrick!"), "%v", caster)
	assert.Equal(t, 1, countContaining(holder, "Aliceia's Neural Toxin afflicts you!"), "%v", holder)
	assert.Equal(t, 1, countContaining(room, "Aliceia's Neural Toxin afflicts Bobrick!"), "%v", room)
}

// A hidden mob's Blood Boil still reads "Something's spell ..." to its
// victim, as every other condition spell's result line does (owner ruling
// 2026-10-10: a hidden caster's result lines keep "Something's").
func TestBloodBoil_AHiddenMobCasterReadsSomething(t *testing.T) {
	m := litRoomOneWithHiddenSkeleton(t)
	seedBoilingBlood(t)
	pinSpellContest(t)
	drainPlain(2)

	resolveMobSpellAgainstPlayer(m, users.GetByUserId(2), rooms.LoadRoom(1), bloodBoilTestSpell(), combat.AttackSide{}, 10)

	requireUnnamed(t, drainPlain(2), "Something's spell sets your blood boiling in your veins!")
}

// A dot kill on Blood Boil credits its caster (#240 still holds on 143).
func TestBloodBoil_ALethalTickCreditsTheCaster(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	seedBoilingBlood(t)
	u1, r := users.GetByUserId(1), rooms.LoadRoom(1)
	mob := mobs.GetInstance(100)

	applySpellEffect(newSpellEffectCtx(u1.Character, actions.NewUserActorInRoom(u1, r),
		actions.NewMobActorInRoom(mob, r), r, bloodBoilTestSpell(), 80, spellContestAttackWin()))
	require.Len(t, mob.Character.GetConditions(boilingBloodId), 1)
	events.DrainQueuedCharacterDiedForTest()

	tickMobConditions(mob, 100)

	died := events.DrainQueuedCharacterDiedForTest()
	require.Len(t, died, 1)
	assert.Equal(t, 1, died[0].KillerUserId, "the caster is the killer")
	assert.Positive(t, mob.Character.PlayerDamage[1], "and is credited with the harm")
}
```

- [ ] **Step 2: Run them and confirm they fail**

Run: `go test ./internal/hooks/ -run 'TestBloodBoil_|TestNeuralToxin_' -count=1`
Expected: the three `TestBloodBoil_` tests FAIL (Blood Boil lands 121 and reads "afflicts"); `TestNeuralToxin_StillLandsPoisonedWithItsOwnLines` PASSES. If the hidden-caster test fails because the line names the Skeleton, stop and report it: that would mean a harmful cast reveals the caster before its result line, which every condition spell would share.

- [ ] **Step 3: Read the dot's condition from the spell**

In `internal/hooks/spell_effects.go`, add `"github.com/GoMudEngine/GoMud/internal/events"` to the imports (between `configs` and `messaging`). Directly above `// applySpellDot is the one damage-over-time applier`, add:

```go
// spellDotConditionId is the record a damage-over-time spell lands: its one
// condition_ids entry (Blood Boil's 143 Boiling Blood, #249), or 121
// Poisoned for a spell that names none (Neural Toxin). The root guard
// TestDotSpellsNameAtMostOneCondition holds the shipped dots to one id.
func spellDotConditionId(spell *spells.SpellData) int {
	if len(spell.ConditionIds) > 0 {
		return spell.ConditionIds[0]
	}
	return conditions.ConditionIdPoisoned
}
```

In `applySpellDot`, replace from

```go
	// Condition 121 ticks every round, so dotDuration is the trigger count.
```

through the end of the function (the closing `return 0` and `}` after the `SendTrio`) with:

```go
	// The dot's record ticks every round, so dotDuration is the trigger count.
	// The record's negative magnitude is the harm per tick, floored at one.
	dotAmount := c.magnitude
	if dotAmount < 1 {
		dotAmount = 1
	}
	dotId := spellDotConditionId(c.spell)
	// Names are read BEFORE the condition lands: the add below can add the
	// "poisoned" or "bleeding" adjective to the target's rendered name, and
	// SendTrio's redaction must see the exact string the line prints.
	casterName, targetName := c.casterName(), c.targetName()
	casterHidden, targetHidden := c.hiddenFromRoom()
	// A record that tells its own start, one line per audience (Blood Boil's
	// 143), is narrated by it, as a condition spell's is (owner ruling R11);
	// a silent-start record (121, Neural Toxin's) keeps the trio below.
	// Everything the start line is judged by is read before the add, as the
	// apply hook reads it: the refresh test, the holder's names, and who
	// could not make the holder out (#249).
	holder, holderKnown := conditionPartyOf(c.targetRef())
	narrated := holderKnown && spellConditionNarratesStart(c, dotId)
	unseenBy := conditionLineUnseenBy(c.room, tc)
	// The character door, on purpose: an immune target refuses the record,
	// and nothing that did not happen may be narrated.
	afflicted := tc.AddConditionMagnitudeBy(dotId, dotDuration, -float64(dotAmount), "spell", c.casterRef()) == nil
	if afflicted && narrated {
		// Told before the fight is committed, so the line is judged against
		// the room as the spell found it.
		ref := c.targetRef()
		narrateConditionStart(conditions.GetConditionSpec(dotId), events.Condition{
			UserId: ref.UserId, MobInstanceId: ref.MobInstanceId, ConditionId: dotId,
			Source: "spell", Caster: c.casterRef(), CasterCrit: c.out.AttackerCrit,
		}, holder, nil, false, unseenBy)
	}
	commitHarmfulSpellAggro(c, fresh)
	if !afflicted || narrated {
		return 0
	}
	messaging.SendTrio(messaging.Trio{
		Actor: messaging.Say(c.category(), fmt.Sprintf(
			`Your %s afflicts %s!%s`, c.spell.Name, targetName, c.critTag())),
		Actee: messaging.Say(c.category(), fmt.Sprintf(
			`%s's <ansi fg="cyan">%s</ansi> afflicts you!%s`, casterName, c.spell.Name, c.critTag())),
		Observer: messaging.Say(c.category(), fmt.Sprintf(
			`%s's <ansi fg="cyan">%s</ansi> afflicts %s!`,
			roomName(casterHidden, casterName), c.spell.Name, roomName(targetHidden, targetName))),
	}, spellAudience(c.casterUser(), casterName, c.targetUser(), targetName, c.room))
	return 0
}
```

Also update `applySpellDot`'s doc comment: after `so a dot kill counts for them.` add the line `// The record is the spell's own (spellDotConditionId, #249).`

In `internal/spells/spells.go:50`, change the `ConditionIds` field comment from `// Condition IDs to apply (for "condition" effect type)` to `// Condition IDs to apply: every id for "condition", the one ward or heal for "shield"/"heal", the one record for "dot"`. Run `gofmt -w internal/spells/spells.go internal/hooks/spell_effects.go`.

- [ ] **Step 4: Ship the condition and point the spell at it**

Create `_datafiles/world/dogmud/conditions/143-boiling_blood.yaml`:

```yaml
conditionid: 143
name: Boiling Blood
description: Blood seething in your veins, dealing damage over time.
triggerrate: 1 round
triggercount: 10
tick_pool: health
tick_from_magnitude: true
start_actor: "You set the blood in {actee}'s veins boiling."
start_actee: "{actor}'s spell sets your blood boiling in your veins!"
start_observer: "{actor}'s spell sets {actee}'s blood boiling."
trigger_actee: '<ansi fg="red">Your blood boils and seeps from your skin!</ansi>'
end_actee: "Your blood cools at last and stops seeping."
flags:
  - bleeding
```

In `_datafiles/world/dogmud/spells/blood-boil.yaml`, after `effect_type: dot`, add the line `condition_ids: [143]`.

In `_datafiles/world/dogmud/templates/help/blood-boil.template`, replace:

```
  <ansi fg="yellow">Defense:     </ansi> Physical — quick reflexes can dodge it; armor only
                 softens what lands
  <ansi fg="yellow">Effect:      </ansi> Damage over time: the boiling blood turns toxic for several rounds
```

with:

```
  <ansi fg="yellow">Defense:     </ansi> Physical. Quick reflexes can dodge it; armor only
                 softens what lands
  <ansi fg="yellow">Effect:      </ansi> Damage over time: the target bleeds for several
                 rounds. Purge Affliction or Cleansing Wave ends it
```

(Task 7 makes the last sentence true for Cleansing Wave and Purge Affliction alike; both cure it today through 121's poison flag, so it is true at every commit.)

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/hooks/ -run 'TestBloodBoil_|TestNeuralToxin_|Dot|HiddenMob_Spell|ConditionNotice|ConditionCast' -count=1`
Expected: PASS

- [ ] **Step 6: Update the three root guards**

`spell_condition_data_guard_test.go`: replace

```go
// conditionIdReaders are the effect types whose applier reads condition_ids
// (internal/hooks: applySpellConditionEffect, applySpellShield,
// applySpellHeal). On any other effect type the list is dead data that
// promises the player something the spell never does: Chrysalis Cocoon's
// [52] and Mass Mend's [33] were exactly that.
var conditionIdReaders = []string{"condition", "shield", "heal"}
```

with

```go
// conditionIdReaders are the effect types whose applier reads condition_ids
// (internal/hooks: applySpellConditionEffect, applySpellShield,
// applySpellHeal, and applySpellDot through spellDotConditionId, #249). On
// any other effect type the list is dead data that promises the player
// something the spell never does: Chrysalis Cocoon's [52] and Mass Mend's
// [33] were exactly that. A dot's named record must tell its own start, as
// every other reader's must (TestSpellConditionsNarrateTheirOwnStart).
var conditionIdReaders = []string{"condition", "shield", "heal", "dot"}
```

and append:

```go
// A dot spell lands one record (spellDotConditionId reads ConditionIds[0]),
// so a second id would be dead data (#249).
func TestDotSpellsNameAtMostOneCondition(t *testing.T) {
	all := shippedSpells(t)
	dots := 0
	for _, id := range sortedSpellIds(all) {
		s := all[id]
		if s.EffectType != "dot" {
			continue
		}
		dots++
		if len(s.ConditionIds) > 1 {
			t.Errorf("%s: a dot spell lands one record, names %v", id, s.ConditionIds)
		}
	}
	// Blood Boil and Neural Toxin; a guard that found none would pass vacuously.
	if dots < 2 {
		t.Fatalf("only %d dot spells found, want at least 2", dots)
	}
}
```

`shipped_narration_data_guard_test.go`: directly after the line `"conditions/41-mind_fog.yaml":               true,` add:

```go
	// Blood Boil's own record (#249), narrated by the same start sender
	// (applySpellDot through narrateConditionStart).
	"conditions/143-boiling_blood.yaml": true,
```

`condition_apply_path_guard_test.go`: find the new line of the dot's add with
`grep -n "AddConditionMagnitudeBy(dotId" internal/hooks/spell_effects.go` (call it N), then replace

```go
	"internal/hooks/spell_effects.go|355": "former combat condition (spell dot): silent-start record, the spell narrates the affliction; must apply synchronously so the refusal is known to the narrator",
```

with

```go
	"internal/hooks/spell_effects.go|N": "spell dot: applies synchronously so an immune target's refusal is known before anything is narrated; the spell then tells the record's own start lines (narrateConditionStart, Blood Boil's 143) or, for a silent-start record (121), its own trio",
```

(N as the number), and at the end of the comment block above it change `when the dot began to carry its caster and creditMobHarm landed above it) ──────` to `when the dot began to carry its caster and creditMobHarm landed above it; re-keyed loose issues sweep 3 when the dot began to read its record from condition_ids, #249) ──────`.

- [ ] **Step 7: Run the root guards**

Run: `go test . -run 'TestSpellCondition|TestDotSpells|TestShieldAndHeal|TestConditionApplyPath|TestObserverIdentity|TestEveryDogmudCondition|TestShippedConditionKeysBind' -count=1`
Run: `go test ./internal/conditions/ -count=1` (the born-dead guard reads 143)
Expected: PASS. If `TestConditionApplyPath...` names a stale key, re-read N; if it names another `spell_effects.go` line, a new call was added by mistake.

- [ ] **Step 8: Document it**

`internal/hooks/context.md`, replace:

```
  passes that rounds figure straight to
  `AddConditionMagnitudeBy(conditions.ConditionIdPoisoned, dotDuration, ..., c.casterRef())`: record 121 ticks
  every round (slice 1b; it was every third round before). See
```

with:

```
  passes that rounds figure straight to
  `AddConditionMagnitudeBy(spellDotConditionId(spell), dotDuration, ..., c.casterRef())`:
  the spell's one `condition_ids` entry (Blood Boil's 143 Boiling Blood) or
  121 Poisoned (Neural Toxin) ticks every round (slice 1b; it was every
  third round before). See
```

and replace:

```
before the harm, as melee does with `TrackPlayerDamage`; the dot does not
(its ticks harm anonymously, a filed follow-up). A dot's duration reads the
```

with:

```
before the harm, as melee does with `TrackPlayerDamage`; the dot's record
carries its caster, so each tick credits them (`creditMobHarm`, #240). A dot
whose record tells its own start (Blood Boil's 143) narrates through
`narrateConditionStart`, with everything read before the record lands and
the lines sent before the fight is committed; a silent-start record (121)
keeps the spell's "afflicts" trio (#249). A dot's duration reads the
```

`internal/spells/context.md`, replace the row

```
| `dot` | Applies ConditionPoisoned; ticks for EffectDuration cycles (each cycle = 3 rounds in AutoHeal) |
```

with

```
| `dot` | Applies the spell's one `condition_ids` record (Blood Boil: 143 Boiling Blood), or 121 Poisoned when it names none (Neural Toxin); duration from the caster (`applySpellDot`, `spellDotConditionId`) |
```

- [ ] **Step 9: Commit**

```bash
git add _datafiles/world/dogmud/conditions/143-boiling_blood.yaml _datafiles/world/dogmud/spells/blood-boil.yaml _datafiles/world/dogmud/templates/help/blood-boil.template internal/hooks/spell_effects.go internal/hooks/blood_boil_test.go internal/hooks/context.md internal/spells/spells.go internal/spells/context.md spell_condition_data_guard_test.go condition_apply_path_guard_test.go shipped_narration_data_guard_test.go
git commit -m "fix(spells): Blood Boil lands its own Boiling Blood and tells its own start (#249)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: #249 a bleed shows as "bleeding"

**Model: haiku**

**Files:**
- Modify: `internal/characters/description.go:166-168`
- Modify: `internal/characters/formattedname.go:24-33`
- Create: `internal/characters/adjectives_bleeding_test.go`
- Modify: `internal/hooks/blood_boil_test.go`
- Modify: `internal/characters/context.md:845`

- [ ] **Step 1: Write the failing tests**

Create `internal/characters/adjectives_bleeding_test.go`:

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #249: a poison shows as "poisoned" beside a name and a bleed showed as
// nothing. The bleeding flag reads "bleeding": Blood Boil's 143 and a combat
// bleed (122) alike (owner call 2026-10-10).
func TestGetAdjectives_BleedingFlagReadsBleeding(t *testing.T) {
	t.Cleanup(conditions.SeedConditionRecordsForTest())
	c := New()
	require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdBleeding, 4, -3, "test"))

	adj := c.GetAdjectives()
	assert.Contains(t, adj, "bleeding")
	assert.NotContains(t, adj, "poisoned")
}
```

Append to `internal/hooks/blood_boil_test.go`:

```go
// The target of Blood Boil shows as bleeding, not poisoned (#249).
func TestBloodBoil_TheTargetReadsBleedingNotPoisoned(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	seedBoilingBlood(t)

	castOnBobrick(t, bloodBoilTestSpell())

	adj := users.GetByUserId(2).Character.GetAdjectives()
	assert.Contains(t, adj, "bleeding")
	assert.NotContains(t, adj, "poisoned")
}
```

- [ ] **Step 2: Run them and confirm they fail**

Run: `go test ./internal/characters/ -run TestGetAdjectives_Bleeding -count=1`
Run: `go test ./internal/hooks/ -run TestBloodBoil_TheTargetReadsBleeding -count=1`
Expected: both FAIL (no "bleeding"). If `New()` panics in the characters test, build the character as `internal/hooks/conditions_pin_test.go` does (`c := &Character{}; c.Conditions.Validate(true)`) and say so in the report.

- [ ] **Step 3: Add the adjective and its style**

In `description.go`, directly after

```go
	if c.HasConditionFlag(conditions.Poison) {
		retAdjectives = append(retAdjectives, `poisoned`)
	}
```

add:

```go
	// A bleed shows as a poison does (#249): Blood Boil's Boiling Blood and
	// a combat bleed both carry the flag.
	if c.HasConditionFlag(conditions.Bleeding) {
		retAdjectives = append(retAdjectives, `bleeding`)
	}
```

In `formattedname.go`'s `adjectiveStyles`, after the `poisoned` row add:

```go
		`bleeding`: {`bleeding`, `bleed`, `red`},  // Is a wound or Blood Boil bleeding them (#249)?
```

Run `gofmt -w internal/characters/formattedname.go internal/characters/description.go`.

In `internal/characters/context.md:845`, change `(sleeping, charmed, poisoned, prone, etc.)` to `(sleeping, charmed, poisoned, bleeding, prone, etc.)`.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/characters/ -count=1`
Run: `go test ./internal/hooks/ -run 'TestBloodBoil_|Purge|Poison|Bleed|Death' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/characters/description.go internal/characters/formattedname.go internal/characters/adjectives_bleeding_test.go internal/characters/context.md internal/hooks/blood_boil_test.go
git commit -m "fix(characters): a bleeding character shows as bleeding (#249)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: #249 Purge Affliction and Cleansing Wave still cure Blood Boil

**Model: sonnet**

Mechanism, chosen against the codebase: the two purges already share one rule (`CancelConditionsWithFlag(conditions.Poison)`); this widens that rule to "every poison, and every record a damage-over-time spell lands", read from the spell registry through Task 5's `spellDotConditionId`. No Go constant for 143 (the id lives in YAML, spec), no new flag, and a combat bleed (122), which no spell lands, stays uncured (owner call).

**Files:**
- Modify: `internal/hooks/spell_purgeaffliction.go` (imports, new `purgeAfflictions`, :101)
- Modify: `internal/hooks/spell_help_effects.go:319-331` (`applySpellPurge`)
- Modify: `internal/hooks/blood_boil_test.go`
- Modify: `_datafiles/world/dogmud/templates/help/cleansing-wave.template:18`
- Modify: `internal/hooks/context.md`, `internal/spells/context.md`

- [ ] **Step 1: Write the failing test**

Append to `internal/hooks/blood_boil_test.go`:

```go
// Purge Affliction and Cleansing Wave cured Blood Boil through 121's poison
// flag. Blood Boil's 143 is a bleed, so both purges cure it by name of the
// spells' own records, and a combat bleed (122) stays (owner call
// 2026-10-10).
func TestPurge_CuresBoilingBloodButNotACombatBleed(t *testing.T) {
	cases := map[string]func(caster, target *users.UserRecord, room *rooms.Room){
		"purge affliction": func(caster, target *users.UserRecord, room *rooms.Room) {
			resolvePurgeAffliction(caster, room, purgeTarget{char: target.Character, user: target, name: target.Character.Name})
		},
		"cleansing wave": func(caster, target *users.UserRecord, room *rooms.Room) {
			wave := &spells.SpellData{SpellId: "cleansing-wave", Name: "Cleansing Wave", EffectType: "purge"}
			applySpellEffect(newSpellEffectCtx(caster.Character, actions.NewUserActorInRoom(caster, room),
				actions.NewUserActorInRoom(target, room), room, wave, 10, spellContestAttackWin()))
		},
	}
	for name, purge := range cases {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(seedAllRegistries())
			seedBoilingBlood(t)
			t.Cleanup(spells.SeedSpellsForTest(map[string]*spells.SpellData{
				"blood-boil": bloodBoilTestSpell(), "neural-toxin": neuralToxinTestSpell(),
			}))
			caster, target, room := users.GetByUserId(1), users.GetByUserId(2), rooms.LoadRoom(1)
			require.NoError(t, target.Character.AddConditionMagnitude(boilingBloodId, 5, -4, "spell"))
			require.NoError(t, target.Character.AddConditionMagnitude(conditions.ConditionIdPoisoned, 5, -4, "spell"))
			require.NoError(t, target.Character.AddConditionMagnitude(conditions.ConditionIdBleeding, 5, -4, "rake"))

			purge(caster, target, room)

			// GetConditions, not HasCondition: a cure only expires a record
			// and leaves it for the round's prune.
			assert.Empty(t, target.Character.GetConditions(boilingBloodId), "Blood Boil is cured")
			assert.Empty(t, target.Character.GetConditions(conditions.ConditionIdPoisoned), "poison is cured, as before")
			assert.Len(t, target.Character.GetConditions(conditions.ConditionIdBleeding), 1, "a combat bleed is not")
		})
	}
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test ./internal/hooks/ -run TestPurge_CuresBoilingBlood -count=1`
Expected: both subtests FAIL on "Blood Boil is cured"; the other two assertions PASS.

- [ ] **Step 3: One cure for both purges**

In `spell_purgeaffliction.go`, add `"github.com/GoMudEngine/GoMud/internal/spells"` to the imports, and above `resolvePurgeAffliction` add:

```go
// purgeAfflictions ends what Purge Affliction and Cleansing Wave cure: every
// poison, and every record a damage-over-time spell lands (Blood Boil's
// Boiling Blood, #249). The cure follows the spells, read from the spell
// registry through spellDotConditionId, so a new dot spell's record is
// curable the day it ships, and a combat bleed (122), which no spell lands,
// stays uncured as before (owner call 2026-10-10).
func purgeAfflictions(ch *characters.Character) {
	ch.CancelConditionsWithFlag(conditions.Poison)
	for _, s := range spells.GetAllSpells() {
		if s.EffectType != "dot" {
			continue
		}
		// GetConditions skips a record already expired, by the poison
		// cancel above (121) or an earlier dot spell naming the same one.
		if id := spellDotConditionId(s); len(ch.GetConditions(id)) > 0 {
			ch.RemoveCondition(id)
		}
	}
}
```

Replace `target.char.CancelConditionsWithFlag(conditions.Poison)` (:101) with `purgeAfflictions(target.char)`, and in its doc comment change `narrates the purge and cancels poison on the target` to `narrates the purge and cures the target (purgeAfflictions)`.

In `spell_help_effects.go`'s `applySpellPurge`, replace `c.targetChar().CancelConditionsWithFlag(conditions.Poison)` with `purgeAfflictions(c.targetChar())`, and in its doc comment change `it cancels every poison on the target.` to `it cures the target (purgeAfflictions: every poison and every record a dot spell lands).`

Run `gofmt -l internal/hooks/` (want no output) and `go build ./internal/hooks/`.

- [ ] **Step 4: Copy and docs**

`cleansing-wave.template:18`: change `Purges poisons from all allies in the room` to `Purges poisons and Blood Boil from all allies in the room`.

`internal/hooks/context.md`, replace:

```
condition), `applySpellShield` (the spell's own ward) and `applySpellPurge`
(cancels every poison). `applySpellDefaultEffect` serves an effect with no
```

with:

```
condition), `applySpellShield` (the spell's own ward) and `applySpellPurge`
(`purgeAfflictions`, shared with Purge Affliction: every poison and every
record a dot spell lands, never a combat bleed, #249).
`applySpellDefaultEffect` serves an effect with no
```

`internal/spells/context.md`, replace the row

```
| `purge` | Removes poison conditions and ConditionPoisoned from target(s) |
```

with

```
| `purge` | Cures poisons and every record a `dot` spell lands (`purgeAfflictions`), not a combat bleed |
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/hooks/ -run 'Purge|Cleans|TestBloodBoil_|SelfCast|AreaPurge|HiddenMob_Spell' -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/hooks/spell_purgeaffliction.go internal/hooks/spell_help_effects.go internal/hooks/blood_boil_test.go internal/hooks/context.md internal/spells/context.md _datafiles/world/dogmud/templates/help/cleansing-wave.template
git commit -m "fix(spells): the purges cure every record a dot spell lands, so Blood Boil still yields to them (#249)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: #236 `trade` and `tutorial` in the help index

**Model: haiku**

**Files:**
- Modify: `_datafiles/world/dogmud/keywords.yaml:47,156`
- Modify: `_datafiles/world/dogmud/templates/help/trade.template:4`
- Modify: `internal/usercommands/helpfile_completeness_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/usercommands/helpfile_completeness_test.go` (add `"gopkg.in/yaml.v2"` to the imports):

```go
// TestHelpFileCompleteness_IndexListsEveryCommand: `help` lists only what
// keywords.yaml indexes, so a command with a help file but no index entry is
// found only by a player who already knows its name. trade and tutorial were
// (#236). Every non-admin command with a help file is listed, directly,
// through the help file it shares (commandHelpAliases), or as a help alias.
func TestHelpFileCompleteness_IndexListsEveryCommand(t *testing.T) {
	root := helpDataRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "keywords.yaml"))
	if err != nil {
		t.Fatalf("reading keywords.yaml: %v", err)
	}
	var kw struct {
		Help        map[string]map[string][]string `yaml:"help"`
		HelpAliases map[string][]string            `yaml:"help-aliases"`
	}
	if err := yaml.Unmarshal(raw, &kw); err != nil {
		t.Fatalf("parsing keywords.yaml: %v", err)
	}
	indexed := map[string]bool{}
	for _, categories := range kw.Help {
		for _, list := range categories {
			for _, c := range list {
				indexed[strings.ToLower(c)] = true
			}
		}
	}
	aliasOf := map[string]string{}
	for topic, list := range kw.HelpAliases {
		for _, a := range list {
			aliasOf[strings.ToLower(a)] = strings.ToLower(topic)
		}
	}
	if len(indexed) < 100 {
		t.Fatalf("read only %d indexed topics; the keywords.yaml shape changed", len(indexed))
	}

	userHelpDir := filepath.Join(root, "templates", "help")
	var unlisted []string
	checked := 0
	for name, info := range userCommands {
		if info.AdminOnly || commandHelpSkip[name] {
			continue
		}
		target := name
		if alias, ok := commandHelpAliases[name]; ok {
			target = alias
		}
		if !helpFileExistsAt(filepath.Join(userHelpDir, target+".template"), filepath.Join(userHelpDir, target+".md")) {
			continue // TestHelpFileCompleteness_Commands reports a missing file
		}
		checked++
		if indexed[name] || indexed[target] || (aliasOf[name] != "" && indexed[aliasOf[name]]) {
			continue
		}
		unlisted = append(unlisted, name)
	}
	if checked < 100 {
		t.Fatalf("checked only %d commands; the scan cannot see the commands it guards", checked)
	}
	if len(unlisted) > 0 {
		sort.Strings(unlisted)
		t.Errorf("commands with a help file that `help` never lists (%d): %s\n"+
			"Add each under a category in _datafiles/world/dogmud/keywords.yaml.",
			len(unlisted), strings.Join(unlisted, ", "))
	}
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test ./internal/usercommands/ -run TestHelpFileCompleteness_IndexListsEveryCommand -count=1`
Expected: FAIL naming exactly `trade, tutorial`.

- [ ] **Step 3: Index them**

In `keywords.yaml`, under `communication:` add `      - trade` directly after `      - chat`; under `general:` add `      - tutorial` directly after `      - newbie`.

In `trade.template`, replace the line

```
<ansi fg="yellow">trade</ansi> channel — for buying, selling, and looking for
```

with

```
<ansi fg="yellow">trade</ansi> channel, for buying, selling and looking for
```

- [ ] **Step 4: Run the help suites**

Run: `go test ./internal/usercommands/ -run 'Help' -count=1`
Run: `go test ./internal/keywords/ ./internal/templates/ -count=1`
Expected: PASS

`docs/PATH_TO_1.0.md:313` "Help coverage" stays open (it is broader than the index); no edit.

- [ ] **Step 5: Commit**

```bash
git add _datafiles/world/dogmud/keywords.yaml _datafiles/world/dogmud/templates/help/trade.template internal/usercommands/helpfile_completeness_test.go
git commit -m "fix(help): trade and tutorial are in the help index, and a test keeps every command there (#236)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: #241 (a) the seizure line comes before the cell door

**Model: haiku**

**Files:**
- Modify: `internal/justice/arrest.go:408-424`
- Modify: `internal/justice/arrest_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/justice/arrest_test.go`:

```go
// #241: the player read "The cell door clangs shut behind you." and only
// then "A guard seizes you...": the door before the arrest. The seizure
// comes first.
func TestExecuteArrest_SeizureLinePrecedesTheCellDoor(t *testing.T) {
	const doorLine = "The cell door clangs shut behind you."
	const arrestedUserId = 8803

	origCell, origMove, origDecay, origNow := cellRoomFn, aMoveFn, aDecayFn, bNowFn
	t.Cleanup(func() {
		cellRoomFn, aMoveFn, aDecayFn, bNowFn = origCell, origMove, origDecay, origNow
	})
	aMoveFn = func(userId int, toRoomId int, isSpawn ...bool) error { return nil }
	cellRoomFn = func(faction string) int { return 5106 }
	aDecayFn = func() int { return 5 }
	bNowFn = func() uint64 { return 100 }

	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		jailedConditionId: {ConditionId: jailedConditionId, Name: "Jailed",
			Flags: []conditions.Flag{conditions.SilentStart}, TriggerCount: 1, RoundInterval: 1,
			StartUserText: doorLine},
	}))
	u := users.NewTestUser(arrestedUserId, "jailbird3", "Jailbird", 0)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{arrestedUserId: u}))
	events.DrainQueuedMessagesForTest(arrestedUserId)

	if ok := ExecuteArrest(u.Character, arrestedUserId, "stillwater_guards", false); !ok {
		t.Fatalf("ExecuteArrest should succeed when the faction has a cell")
	}

	seize, door := -1, -1
	msgs := events.DrainQueuedMessagesForTest(arrestedUserId)
	for i, msg := range msgs {
		if seize < 0 && strings.Contains(msg, "A guard seizes you") {
			seize = i
		}
		if door < 0 && strings.Contains(msg, doorLine) {
			door = i
		}
	}
	if seize < 0 || door < 0 {
		t.Fatalf("want both the seizure and the door line, got %q", msgs)
	}
	if seize > door {
		t.Errorf("the cell door (line %d) clanged before the guard seized the player (line %d)", door, seize)
	}
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test ./internal/justice/ -run TestExecuteArrest_SeizureLinePrecedes -count=1`
Expected: FAIL ("the cell door ... clanged before the guard seized the player").

- [ ] **Step 3: Swap the two nine-line blocks**

In `arrest.go`, replace:

```go
	if u := users.GetByUserId(userId); u != nil {
		if spec := conditions.GetConditionSpec(jailedConditionId); spec != nil {
			line := spec.AuthoredStartLine(
				u.Character.GetCharacterName(true),
				u.Character.GetCharacterName(false))
			if line != "" {
				u.SendText(messaging.CategoryConditionApply, line)
			}
		}
		// CategorySystem is never wrapped by the pipeline (it also carries
		// tables), so this line wraps itself to the reader's width; unwrapped
		// it ran past 100 columns (#430). It names `fine`, which names
		// `payfine`: a new prisoner otherwise had to guess the way out (#298).
		u.SendText(messaging.CategorySystem, messaging.WrapAnsi(
			fmt.Sprintf("A guard seizes you and hauls you to the holding cell. "+
				"You have been placed under arrest by the %s. "+
				`Type <ansi fg="command">fine</ansi> to see what you owe.`, factionName),
			u.GetLineWidth()))
	} else {
```

with:

```go
	if u := users.GetByUserId(userId); u != nil {
		// The seizure comes first, then the cell door (#241). CategorySystem
		// is never wrapped by the pipeline (it carries tables), so this line
		// wraps itself to the reader's width (#430). It names `fine`, which
		// names `payfine`, so a new prisoner sees the way out (#298).
		u.SendText(messaging.CategorySystem, messaging.WrapAnsi(
			fmt.Sprintf("A guard seizes you and hauls you to the holding cell. "+
				"You have been placed under arrest by the %s. "+
				`Type <ansi fg="command">fine</ansi> to see what you owe.`, factionName),
			u.GetLineWidth()))
		if spec := conditions.GetConditionSpec(jailedConditionId); spec != nil {
			line := spec.AuthoredStartLine(
				u.Character.GetCharacterName(true),
				u.Character.GetCharacterName(false))
			if line != "" {
				u.SendText(messaging.CategoryConditionApply, line)
			}
		}
	} else {
```

The line count is unchanged, so `arrest.go|395` and `|645` stay put.

- [ ] **Step 4: Run the tests and the line-keyed guard**

Run: `go test ./internal/justice/ -count=1`
Run: `go test . -run TestConditionApplyPath -count=1`
Expected: PASS (no stale key: `grep -n "AddConditionScaled(jailedConditionId" internal/justice/arrest.go` still prints 395 and 645).

- [ ] **Step 5: Commit**

```bash
git add internal/justice/arrest.go internal/justice/arrest_test.go
git commit -m "fix(justice): the guard seizes you before the cell door shuts (#241)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 10: #241 (b) one arrest stamp, on the player, that lapses

**Model: sonnet**

The stamp moves from each guard's MiscData to the player's, as two keys (round and room), so one guard declares however many share the room. A stamp holds only in the room it was made in, and only for the grace plus a window as long again (`2 * arrestGraceRounds()`, derived from `ArrestResistGraceRounds`; no new knob). A stamp that does not hold is dropped and the sighting declares afresh. Any guard in that room hauls once the grace has run, and the haul clears it. Old per-guard `justice_arrest_pending_<uid>` keys need no migration: mob MiscData is never saved (facts table), so they vanish at the next restart, and nothing reads them after this task.

**Files:**
- Modify: `internal/justice/enforce.go`
- Create: `internal/justice/arrest_stamp_test.go`
- Modify: `internal/justice/context.md`

- [ ] **Step 1: Add the seam and the keys (no behaviour change)**

In `enforce.go`, after the `executeArrestFn` var, add:

```go
// guardFactionsFn seam: the factions a guard enforces for. Tests override,
// because factions.FactionsForMob needs loaded faction definitions.
var guardFactionsFn = factions.FactionsForMob

// Player MiscData keys for a declared arrest (#241). The stamp lives on the
// PLAYER, not on each guard, so one declaration is heard however many guards
// share the room, and it holds only where and while it was made
// (liveArrestStamp).
const (
	keyArrestPendingRound = "justice_arrest_pending_round"
	keyArrestPendingRoom  = "justice_arrest_pending_room"
)
```

In `RunGuardEnforcement`, change `guardFactions := factions.FactionsForMob(mob)` to `guardFactions := guardFactionsFn(mob)`.

Run: `go build ./internal/justice/ && go test ./internal/justice/ -count=1` (expect PASS).

- [ ] **Step 2: Write the failing tests**

Create `internal/justice/arrest_stamp_test.go`:

```go
package justice

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const (
	stampTestUserId = 8841
	stampTestRoomA  = 9841
	stampTestRoomB  = 9842
	stampTestRound  = uint64(1000)
)

// arrestStampScene is a wanted player (surrender policy) in room A with
// three guards of one cell-owning faction, every justice seam pinned. It
// counts the declarations the guards speak and records whom they haul.
type arrestStampScene struct {
	player       *users.UserRecord
	roomA, roomB *rooms.Room
	guards       []*mobs.Mob
	declared     int
	hauled       []int
}

func newArrestStampScene(t *testing.T) *arrestStampScene {
	t.Helper()
	s := &arrestStampScene{}

	u := users.NewTestUser(stampTestUserId, "wanted", "Wanted", 0)
	u.Character.ArrestPolicy = characters.ArrestSurrender
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{stampTestUserId: u}))
	s.player = u

	s.roomA = &rooms.Room{RoomId: stampTestRoomA}
	s.roomB = &rooms.Room{RoomId: stampTestRoomB}
	s.roomA.AddPlayer(stampTestUserId)
	t.Cleanup(func() {
		s.roomA.RemovePlayer(stampTestUserId)
		s.roomB.RemovePlayer(stampTestUserId)
	})

	for i := 0; i < 3; i++ {
		g := &mobs.Mob{MobId: 1, InstanceId: 8850 + i, Groups: []string{"guard", "test_guards"}}
		g.Character.Name = "Guard"
		s.guards = append(s.guards, g)
	}

	origFactions, origBounty, origCell := guardFactionsFn, openFactionBountyFn, cellRoomFn
	origSay, origArrest := guardSayFn, executeArrestFn
	t.Cleanup(func() {
		guardFactionsFn, openFactionBountyFn, cellRoomFn = origFactions, origBounty, origCell
		guardSayFn, executeArrestFn = origSay, origArrest
	})
	guardFactionsFn = func(*mobs.Mob) []string { return []string{"test_guards"} }
	openFactionBountyFn = func(int, map[string]bool) bool { return true } // wanted: SeverityArrest
	cellRoomFn = func(string) int { return 5106 }
	guardSayFn = func(_ *rooms.Room, _ *mobs.Mob, line string) {
		if strings.Contains(line, "under arrest") {
			s.declared++
		}
	}
	executeArrestFn = func(_ *characters.Character, userId int, _ string, _ bool) bool {
		s.hauled = append(s.hauled, userId)
		return true
	}
	return s
}

// moveToB walks the player from room A to room B.
func (s *arrestStampScene) moveToB() {
	s.roomA.RemovePlayer(stampTestUserId)
	s.roomB.AddPlayer(stampTestUserId)
}

func (s *arrestStampScene) stamp() (round, room uint64, ok bool) {
	round, okRound := miscDataRound(s.player.Character.MiscData, keyArrestPendingRound)
	room, okRoom := miscDataRound(s.player.Character.MiscData, keyArrestPendingRoom)
	return round, room, okRound && okRoom
}

// #241: every guard in the room stamped its own MiscData and declared on its
// own, so three guards spoke three declarations in one round. The stamp is
// the player's: the first guard declares, the others see it and wait.
func TestArrestStamp_ThreeGuardsDeclareOnce(t *testing.T) {
	s := newArrestStampScene(t)
	for _, g := range s.guards {
		RunGuardEnforcement(g, s.roomA, stampTestRound)
	}
	if s.declared != 1 {
		t.Fatalf("three guards declared %d times in one round, want 1", s.declared)
	}
	if round, room, ok := s.stamp(); !ok || round != stampTestRound || room != stampTestRoomA {
		t.Fatalf("player stamp = round %d room %d (ok %v), want round %d room %d", round, room, ok, stampTestRound, stampTestRoomA)
	}
	for _, g := range s.guards {
		for k := range g.Character.MiscData {
			if strings.HasPrefix(k, "justice_arrest_pending") {
				t.Errorf("guard %d carries %s; the stamp belongs on the player", g.InstanceId, k)
			}
		}
	}

	for _, g := range s.guards {
		RunGuardEnforcement(g, s.roomA, stampTestRound+1)
	}
	if s.declared != 1 || len(s.hauled) != 0 {
		t.Fatalf("within the grace: %d declarations, %d hauls; want 1 and 0", s.declared, len(s.hauled))
	}
}

// Staying put through the grace is still an arrest: any guard in the room
// hauls once it has run, and the haul clears the stamp.
func TestArrestStamp_StayingThroughTheGraceIsHauled(t *testing.T) {
	s := newArrestStampScene(t)
	RunGuardEnforcement(s.guards[0], s.roomA, stampTestRound)
	RunGuardEnforcement(s.guards[1], s.roomA, stampTestRound+arrestGraceRounds())

	if len(s.hauled) != 1 || s.hauled[0] != stampTestUserId {
		t.Fatalf("hauled %v, want the player once", s.hauled)
	}
	if s.declared != 1 {
		t.Errorf("declared %d times, want 1", s.declared)
	}
	if _, _, ok := s.stamp(); ok {
		t.Errorf("the haul must clear the stamp")
	}
}

// The defect from the issue: declared, the player walked off, and the same
// guard met later somewhere else hauled them at once with no new word. A
// declaration holds only in its room: elsewhere the guard declares afresh.
func TestArrestStamp_AGuardElsewhereDeclaresAfresh(t *testing.T) {
	s := newArrestStampScene(t)
	RunGuardEnforcement(s.guards[0], s.roomA, stampTestRound)
	s.moveToB()
	later := stampTestRound + arrestGraceRounds() + 1
	RunGuardEnforcement(s.guards[0], s.roomB, later)

	if len(s.hauled) != 0 {
		t.Fatalf("hauled %v in another room with no new declaration", s.hauled)
	}
	if s.declared != 2 {
		t.Fatalf("declared %d times, want a fresh declaration (2)", s.declared)
	}
	if round, room, ok := s.stamp(); !ok || round != later || room != stampTestRoomB {
		t.Fatalf("player stamp = round %d room %d (ok %v), want round %d room %d", round, room, ok, later, stampTestRoomB)
	}
}

// A declaration in the same room lapses too, once it is older than the
// grace plus a window as long again: the guard declares afresh.
func TestArrestStamp_AnOldDeclarationLapses(t *testing.T) {
	s := newArrestStampScene(t)
	RunGuardEnforcement(s.guards[0], s.roomA, stampTestRound)
	RunGuardEnforcement(s.guards[0], s.roomA, stampTestRound+2*arrestGraceRounds()+1)

	if len(s.hauled) != 0 || s.declared != 2 {
		t.Fatalf("%d hauls and %d declarations; want 0 and 2", len(s.hauled), s.declared)
	}
}
```

- [ ] **Step 3: Run them and confirm they fail**

Run: `go test ./internal/justice/ -run TestArrestStamp_ -count=1`
Expected: all four FAIL on the old per-guard stamp (three declarations; a second guard declares instead of hauling; the same guard hauls in room B; the old stamp hauls).

- [ ] **Step 4: Stamp the player**

In `enforce.go`, after the `arrestGraceRounds` function, add:

```go
// arrestStampLapseRounds is how long a declaration holds: the grace, then as
// long again for a guard in the room to make the haul. Derived from the one
// knob, ArrestResistGraceRounds, rather than a second (#241).
func arrestStampLapseRounds() uint64 { return 2 * arrestGraceRounds() }

// liveArrestStamp reads the player's declared arrest and reports whether it
// still holds in roomId at nowRound: made in this room and no older than
// arrestStampLapseRounds. A stamp that does not hold is cleared, so this
// sighting declares afresh: a player who walks off during the grace, or is
// next seen long after, hears the declaration again before any haul (owner
// call 2026-10-10). A player who leaves and comes back to the same room
// inside the window still holds it.
func liveArrestStamp(player *characters.Character, roomId int, nowRound uint64) (uint64, bool) {
	round, ok := miscDataRound(player.MiscData, keyArrestPendingRound)
	if !ok {
		return 0, false
	}
	stampRoom, roomOk := miscDataRound(player.MiscData, keyArrestPendingRoom)
	if roomOk && stampRoom == uint64(roomId) && nowRound >= round && nowRound-round <= arrestStampLapseRounds() {
		return round, true
	}
	clearArrestStamp(player)
	return 0, false
}

// clearArrestStamp drops the player's declared arrest.
func clearArrestStamp(player *characters.Character) {
	player.SetMiscData(keyArrestPendingRound, nil)
	player.SetMiscData(keyArrestPendingRoom, nil)
}
```

In `RunGuardEnforcement`'s `case SeverityArrest:` block, replace:

```go
			resist := user.Character.ArrestPolicy == characters.ArrestResist
			pendingKey := fmt.Sprintf("justice_arrest_pending_%d", uid)
			pendingRound, pending := miscDataRound(mob.Character.MiscData, pendingKey)
			switch resolveArrest(resist, pending, pendingRound, nowRound, arrestGraceRounds()) {
```

with:

```go
			resist := user.Character.ArrestPolicy == characters.ArrestResist
			pendingRound, pending := liveArrestStamp(user.Character, room.RoomId, nowRound)
			switch resolveArrest(resist, pending, pendingRound, nowRound, arrestGraceRounds()) {
```

replace:

```go
				guardSayFn(room, mob,
					"No more moving along. You're under arrest. Come quietly.")
				mob.Character.SetMiscData(pendingKey, nowRound)
```

with:

```go
				guardSayFn(room, mob,
					"No more moving along. You're under arrest. Come quietly.")
				user.Character.SetMiscData(keyArrestPendingRound, nowRound)
				user.Character.SetMiscData(keyArrestPendingRoom, room.RoomId)
```

and replace:

```go
				executeArrestFn(user.Character, uid, faction, false)
				mob.Character.SetMiscData(pendingKey, nil)
```

with:

```go
				executeArrestFn(user.Character, uid, faction, false)
				clearArrestStamp(user.Character)
```

In the `faction == ""` branch, change the comment `leave the pending stamp so the declaration line isn't lost.` to `leave the player's stamp; it lapses on its own (liveArrestStamp).`

In `pruneStaleWarnStamps`'s doc comment, replace `// Leaves justice_arrest_pending_* (self-cleaning on haul) and all other keys.` with `// Leaves every other key alone (an arrest stamp lives on the player, #241).`

Run `gofmt -w internal/justice/enforce.go` and `go vet ./internal/justice/`.

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/justice/ -count=1`
Expected: PASS (`TestPruneStaleWarnStamps` still passes: it checks only that the prune leaves other keys alone).

- [ ] **Step 6: Document it**

In `internal/justice/context.md`, replace the three surrender rows:

```
| Surrender — first sight | `guardSayFn` warning + stamp `justice_arrest_pending_<uid>` in guard MiscData |
| Surrender — within `ArrestResistGraceRounds` | No-op (waiting) |
| Surrender — grace expired | Call `executeArrestFn` (→ `ExecuteArrest`), clear pending stamp |
```

with:

```
| Surrender, no live stamp | `guardSayFn` declaration; stamp `justice_arrest_pending_round` and `justice_arrest_pending_room` on the PLAYER's MiscData, so one guard speaks however many share the room (#241) |
| Surrender, within `ArrestResistGraceRounds` | No-op (waiting) |
| Surrender, grace run, same room | Any guard there calls `executeArrestFn` (→ `ExecuteArrest`) and clears the stamp |
| Surrender, stamp from another room or older than twice the grace | `liveArrestStamp` drops it; this sighting declares afresh |
```

After the `**`executeArrestFn` seam**` paragraph add:

```
**`guardFactionsFn` seam**: `factions.FactionsForMob`, which needs loaded
faction definitions; tests override it.
```

Replace:

```
accumulate on guard MiscData indefinitely. Arrest-pending stamps
(`justice_arrest_pending_*`) are self-cleaning on haul and are left alone.
```

with:

```
accumulate on guard MiscData indefinitely. Arrest stamps live on the
player, not the guard, and lapse on their own (`liveArrestStamp`, #241).
```

In the flow diagram, replace `            ├─ !pending     → guardSayFn + stamp pending key` with `            ├─ !pending     → guardSayFn + stamp the player (round, room)`, and directly above the `└─ resolveArrest(...)` line add `       └─ liveArrestStamp(player, room, now)  (another room or past 2x grace: dropped)`.

- [ ] **Step 7: Commit**

```bash
git add internal/justice/enforce.go internal/justice/arrest_stamp_test.go internal/justice/context.md
git commit -m "fix(justice): one arrest declaration per player, and it lapses when they walk off (#241)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 11: Gate, patch notes, PR

**Model: sonnet**

**Files:**
- Modify: `docs/PATCH_NOTES.md`

- [ ] **Step 1: Bring in master**

Run: `git fetch origin && git merge origin/master` (9798f6d02 or later). If `docs/README.md` conflicts, keep both rows. If any file this plan touched changed on master, re-run that task's tests after the merge.

- [ ] **Step 2: gofmt, build, lint**

Load the `dogmud-shipping` skill. Then, from the worktree:

Run: `gofmt -l internal/ modules/` (expect no output; also `gofmt -l *.go` for the root guards)
Run: `go build ./...`
Run: `~/go/bin/golangci-lint run --new-from-merge-base=origin/master` (expect `0 issues`; `golangci-lint --version` should match CI's v2.12.2 in `.github/workflows/validate.yml:43`)

- [ ] **Step 3: Touched-package tests**

Run: `go test ./internal/util/ ./internal/usercommands/ ./internal/mapper/ ./internal/hooks/ ./internal/conditions/ ./internal/characters/ ./internal/spells/ ./internal/justice/ ./internal/keywords/ ./internal/templates/ ./internal/users/ ./internal/connections/ -count=1`
Run: `go test . -count=1`
Expected: PASS. A failure in a test this plan did not touch: check it on a clean detached worktree of master (never `git stash`; the stash is shared across worktrees) before treating it as ours.

- [ ] **Step 4: Shuffle the new tests only**

Run: `go test ./internal/usercommands/ -run 'TestAdminColumnTruncation|TestMapCodeMarkers|TestMapOccupantMarkers|TestHelpFileCompleteness_IndexListsEveryCommand' -shuffle=on -count=3`
Run: `go test ./internal/hooks/ -run 'TestBloodBoil_|TestNeuralToxin_|TestPurge_CuresBoilingBlood' -shuffle=on -count=3`
Run: `go test ./internal/justice/ -run 'TestArrestStamp_|TestExecuteArrest_SeizureLinePrecedes' -shuffle=on -count=3`
Run: `go test ./internal/characters/ -run TestGetAdjectives_Bleeding -shuffle=on -count=3`
Run: `go test ./internal/mapper/ -run TestMapGlyphsConvertToOneAsciiCharacter -shuffle=on -count=3`
Run: `go test ./internal/util/ -run TestConvertToAscii -shuffle=on -count=3`
Expected: PASS every time. (No new mobcommands or items tests in this plan.) A failure under shuffle is a missing `t.Cleanup` in the new test: fix the test, not the order.

- [ ] **Step 5: Isolated boot**

`C:/tmp/dogmud-boot-check-loose3` is the boot worktree; it is removed in this step. The committed `config.yaml` is used as is.

```bash
git worktree add --detach C:/tmp/dogmud-boot-check-loose3 HEAD
cat > C:/tmp/dogmud-boot-check-loose3/boot-overrides.yaml <<'EOF'
Network.TelnetPort: [33340]
Network.LocalPort: 9994
Network.HttpPort: 8100
Network.HttpsPort: 0
Network.AIPort: 0
EOF
go build -C C:/tmp/dogmud-boot-check-loose3 -o boot-check.exe .
```

Start it hidden and keep its PID (PowerShell):

```powershell
$env:CONFIG_PATH = 'C:\tmp\dogmud-boot-check-loose3\boot-overrides.yaml'; $env:LOG_NOCOLOR = '1'
$p = Start-Process -FilePath 'C:\tmp\dogmud-boot-check-loose3\boot-check.exe' -WorkingDirectory 'C:\tmp\dogmud-boot-check-loose3' -RedirectStandardOutput 'C:\tmp\dogmud-boot-check-loose3\boot.log' -RedirectStandardError 'C:\tmp\dogmud-boot-check-loose3\boot.err' -WindowStyle Hidden -PassThru
$p.Id
```

Wait (Monitor with an until-loop, not `sleep`) until `boot.log` contains `Server Ready` or a panic, at most 180 seconds. Then:

Run: `grep -c "Server Ready" C:/tmp/dogmud-boot-check-loose3/boot.log` (want 1)
Run: `grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" C:/tmp/dogmud-boot-check-loose3/boot.log C:/tmp/dogmud-boot-check-loose3/boot.err` (want 0 for each; a zero count exits 1, so run it on its own)
Run: `grep -iE "143|boiling|blood-boil|keywords" C:/tmp/dogmud-boot-check-loose3/boot.log | grep -iE "error|warn|fail"` (want no output; run on its own)

Stop it by that PID only (never by name or port): `Stop-Process -Id <PID> -Force`. Then `git worktree remove --force C:/tmp/dogmud-boot-check-loose3`; if Windows still holds the exe, `Remove-Item -Recurse -Force 'C:\tmp\dogmud-boot-check-loose3'` and then `git worktree prune`.

- [ ] **Step 6: Patch notes**

Add this section to `docs/PATCH_NOTES.md` directly above `## 2026-10-10: More loose ends` (below the `## Unreleased: ...` section):

```markdown
## 2026-10-10: Still more loose ends

- Blood Boil now makes its target bleed instead of poisoning them. The
  target, the caster and anyone watching each read their own line as it
  lands. A stomach hardened against poison no longer keeps it out, and
  Purge Affliction and Cleansing Wave still end it.
- Anyone who is bleeding, from Blood Boil or a wound taken in a fight,
  now shows as bleeding beside their name.
- `help` now lists `trade` and `tutorial`.
- When you are arrested, you read that a guard has seized you before you
  hear the cell door shut.
- A guard's call to arrest you is spoken once, however many guards are in
  the room. If you walk away before they take you, the call lapses, and
  the next guard who sees you says it again before taking you. Guards no
  longer seize you somewhere else, long after the fact, without a word.
- With the ASCII character set on, the map draws letters for its markers
  and its terrain, and the legend names each one. Arrows, trailing dots
  and similar marks now come through as plain text.
- A keen eye now spots players, peaceful folk and hostile creatures on the
  map, as `help map` always said.
```

Read it once against `dogmud-player-copy`: 80 columns, no raw numbers, no em or en dashes (`grep -n "[—–]" docs/PATCH_NOTES.md | head -5` must show nothing in the new section).

```bash
git add docs/PATCH_NOTES.md
git commit -m "docs(patch-notes): loose issues sweep 3" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 7: PR**

Write the body to a file in the session scratchpad (not `C:/tmp`):

```markdown
Four fixes clear of the files reserved for the mitigation work.

Fixes #249
Fixes #236
Fixes #241

Also the follow-up to issue 253 (closed): ASCII forms for the glyphs that carry meaning (map markers, biome glyphs, arrows, ellipses, the lock grid pins), padded admin columns that cut in three dots, and the NPC, player and mob map markers turned on at map level 4 as `help map` promised.

Notes:
- #249: Blood Boil lands its own condition 143 Boiling Blood (`condition_ids` on the spell; `applySpellDot` falls back to 121 so Neural Toxin is unchanged) and narrates through the condition's start lines. The `bleeding` flag now shows as an adjective, for combat bleeds too. Purge Affliction and Cleansing Wave cure every record a dot spell lands (`purgeAfflictions`), so Blood Boil still yields to them; a combat bleed does not.
- #241: the arrest stamp moved from each guard to the player (round and room) and lapses after twice the grace, derived from `ArrestResistGraceRounds`; no new knob. Old per-guard stamps live only in memory and vanish at the restart.
- `map wide` still does nothing at any Perception (its sizing sits under the same unreachable level); left for an owner call.

Left out on purpose: the mitigation work (issue 465, another branch), issues 264, 261 and 283 (M7 wording), 440 (own branch).

Gate: gofmt clean, build, golangci-lint 0 issues, touched-package tests, new tests under -shuffle=on, isolated boot reached Server Ready.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
```

Only #249, #236 and #241 carry a closing keyword; 253 and the left-out issues are named without `#` so nothing closes or links by accident.

```bash
git push -u origin fix/loose-issues-sweep-3
gh pr create --repo pruuk/DOGMud --base master --head fix/loose-issues-sweep-3 --title "Loose issues sweep 3: Blood Boil, help index, arrests, ASCII glyphs" --body-file <scratchpad body file>
```

Read the URL `gh` prints and confirm it says `pruuk/DOGMud`. Then `gh pr checks <n> --repo pruuk/DOGMud --watch`, and confirm with `gh run list --repo pruuk/DOGMud --branch fix/loose-issues-sweep-3` that every expected workflow ran. The owner merges and deploys.

---

## Notes for the owner (decisions and found flaws)

1. **`map wide` is dead.** The screen-size block in `skill.map.go:98` sits under the same unreachable `skillLevel > 4` as the markers did, and nothing reads `rest == "wide"` for sizing, so `map wide` draws the ordinary map at every Perception while `help map` says it needs high Perception. This plan leaves it alone: turning it on at level 4 changes map size for every keen player. Owner call: turn it on at level 4, or drop the line from `help map`.
2. **Purge mechanism.** The spec says the purges "cancel condition 143". To avoid a Go constant for a YAML id, `purgeAfflictions` cures every record a `dot` spell lands, read from the spell registry. Same behaviour today; a future dot spell's record is curable automatically. A dot spell that named 122 would make the purges cure combat bleeds; none does.
3. **Narration order.** Blood Boil's start lines go out before the fight is committed (judged against the room as the spell found it); Neural Toxin's trio keeps its old place after the commit.
4. **The bleeding adjective is styled** like `poisoned`: red, short form `bleed`. A small call made in Task 6.
5. **Shared ☺ legend.** Now that NPC and Player markers draw, the legend row for ☺ (P) names whichever of Party Member, NPC or Player was placed first; the three share one glyph by design.
6. **Same-room return.** A player who leaves and comes back to the room where they were declared, inside twice the grace, is still held by that declaration. Tighter would need a "last seen" key; not added.
7. **≋ in splash art** also becomes `=` in ASCII mode (it was raw Unicode there before).
7a. **The default world** (upstream sample content, not live) draws ♜ on 35 rooms, which has no ASCII form, and uses `& K P X M` as room symbols, which the new forms share. The spec's collision check covered the live dogmud world only, which is right for DOGMud; the glyph test scans dogmud rooms only.
8. **Not touched, noted:** `factionTruncate` and `opinionTruncate` (admin.faction.go, admin.opinion.go) cut by byte, which can split a rune; they already end in an ASCII `~`. The `🛡` glyph in `help combat` and in `combat_shared_helpers.go` (off-limits) has no ASCII form.
