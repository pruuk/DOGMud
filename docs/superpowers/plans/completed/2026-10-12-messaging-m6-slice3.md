# Messaging M6 Slice 3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close #370 ledger rows 3, 4, 6, 14, 15, 20, 23, 25, 35: speech and silence decide how a taunt reads, a wordless pool for beasts and `howl`, a body line after a death, and five small pool or copy fixes.

**Architecture:** A species `speech` field and a `silenced` condition flag feed one check, `characters.SpeechState`. The shared `actions.ExecuteTaunt` refuses a silenced taunter before any cost. Mob `taunt` and `howl` share one narration flow that takes a taunt pool name (`rhetoric` or the new `wordless`). The corpse line is emitted where a corpse is created. Pools stay in YAML; root guards from slice 2 keep them clean.

**Tech Stack:** Go, YAML stores via `internal/fileloader`, `internal/narration`, goldens in `internal/narration/testdata/stores/`.

**Spec:** `docs/superpowers/specs/2026-10-12-messaging-m6-slice3-design.md`

---

## Facts verified against source (master after #498, 2026-10-12)

| Fact | Where |
|---|---|
| `characters` already reads species through `species.GetSpecies(c.SpeciesId)` (nil when unknown) | `internal/characters/anatomy.go:9-19` |
| `species.Species` fields include `NaturalBash bool \`yaml:"naturalbash,omitempty"\``, `BodyParts []string`; 42 species files | `internal/species/species.go:40-60`, `_datafiles/world/dogmud/species/` |
| Condition flags are `conditions.Flag` constants; a condition carrying a flag outside `conditions.AllFlags` fails validation | `internal/conditions/conditionspec.go:29-110,443` |
| `Character.HasFlagFromAnySource(flag)` checks conditions AND mutation flags | `internal/characters/conditions.go:26-31` |
| `actions.ExecuteTaunt(actor Actor) TauntResult` is shared by player `taunt`, mob `taunt` and mob `howl`; it checks `IsActing`, target, cooldown, then pays cost via `admitFullCost` | `internal/actions/combat_taunt.go:91-130` |
| `TauntResult` fields: `Cost, Target, Executed, OnCooldown, NoTarget, Hit, Crit, Fumble, Defence, DmgDesc, DmgDescToTarget, AggroPulled, Crafting` | `internal/actions/combat_taunt.go:19-80` |
| Player `taunt` refusal branches print a `CategorySystem` line per `TauntResult` flag | `internal/usercommands/taunt.go:28-48` |
| Mob `taunt` switches on Fumble / Hit (crit) / miss, calls `sendMobTauntTriad(intensity, ...)`, falls back to legacy literals when it returns false, sends an aggro-pull line | `internal/mobcommands/taunt.go:50-104,107-160` |
| Mob `howl` has the same switch with six hand-rolled literals and its own aggro-pull line ("turns from its prey and snarls at") | `internal/mobcommands/howl.go:15-85` |
| `combat.GetTauntTriad(intensity, source, target, sourceType, targetType, damageDesc string, picker ...narration.Picker)` hardcodes `tauntMessages["rhetoric"]`; the loader keys every file in `taunt-messages/` by `optionid`; 3 non-test callers | `internal/combat/taunt_messages.go:107,147-148` |
| `rhetoric.yaml` bands `hit`, `miss`, `critical`, `fumble`; index-paired `actor`/`actee`/`observer`; tokens `{actor} {actee} {actortype} {acteetype} {damage}` | `_datafiles/world/dogmud/taunt-messages/rhetoric.yaml` |
| Mob death line via `sendVisualElseAudible(room, CategoryDeath, deathMsg, soundMsg)`; `sendVisualElseAudible` = `SendTextVisual` + `SendTextUnsighted` | `internal/hooks/Death_MobBroadcast.go:51-55`, `NewRound_DoCombat_helpers.go:488-494` |
| Mob corpse: `dropMobLootAndSetCorpse(m, room)`, corpse only when `config.Death.CorpsesEnabled` (`:80`); player corpse: `wirePlayerCorpse(c)` | `internal/hooks/Death_MobLoot.go:27,80`, `internal/hooks/Death_PlayerCorpse.go:16,24` |
| Room senders: `SendTextVisual`, `SendTextVisualHidingNames`, `SendTextVisualToUser`, `SendTextUnsighted` | `internal/rooms/rooms.go:281,288,566,600` |
| `cast_started` holds 3 variants with `{spell}` | `_datafiles/world/dogmud/casting-messages.yaml:6-9` |
| `deflectedSwingLines(defense combatvocab.Defence, sourceName, targetName, dmgDesc string) (toAttacker, toDefender items.ItemMessage)` | `internal/combat/combat_helpers.go:1578-1600` |
| Defence store types: `DefenseMessageGroup`, `DefenseIntensity`, `DefenseOptions`, `Validate`, `RenderDefenseMessage` | `internal/items/defensive_messages.go:57-150` |
| Mob craft `Initiated` branch sends fixed "begins working on something."; `result.Recipe` is in scope (used by the `ImmediateComplete` branch) | `internal/mobcommands/craft.go:55-67` |
| `IronwindEvent-Regional` (3 lines) and `IronwindEvent-Global` (2 lines), unreachable | `_datafiles/world/dogmud/gossip_templates.yaml:102-109` |
| Seven fact descriptions start with Boats, Drowned, Taxes, Wolves, Bandits, Folk, Water | `_datafiles/world/dogmud/facts.yaml:11,35,59,83,99,108,125` |
| `counter-parry.yaml`: 45 lines, 22 to 24 blade-shaped | that file |

## Working rules for every task

- Worktree `C:/tmp/dogmud-m6s3` on branch `feat/m6-slice3` (create in Task 0; named here so the tmp sweep keeps it).
- Edit tool only; no `sed -i`, no Python that writes files. `git add` named paths only.
- Every gate runs the touched packages AND `go test .`; world-data tasks also run the FULL `go test ./...` and the YAML shape check (`project-m6-slice2-lessons-2026-10-12`).
- Player copy: plain ESL-clear English, at most 80 visible columns with tokens filled, no numbers, no em/en dash, no spaced hyphen or double hyphen as a dash, a space between words and tags, singular "they" for an actor (never "its" for a player-usable line). Never put `: ` inside an unquoted YAML value.
- Null-probe every new test.
- Commits end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

---

### Task 0: Worktree and baseline

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud" && git fetch origin
git worktree add -b feat/m6-slice3 C:/tmp/dogmud-m6s3 origin/master
cd C:/tmp/dogmud-m6s3 && go build ./... && go test . ./internal/combat/... ./internal/actions/... ./internal/mobcommands/... ./internal/usercommands/... ./internal/species/... ./internal/characters/... -count=1
```
Expected: all `ok`.

---

### Task 1: Species `speech` field

**Files:** Modify `internal/species/species.go`; modify the speechless species YAML files; create `internal/species/speech_test.go`; update `internal/species/context.md`.

- [ ] **Step 1: Failing test** `internal/species/speech_test.go`:

```go
package species

import (
	"testing"

	"gopkg.in/yaml.v2"
)

func TestSpeechDefaultsToTrueWhenAbsent(t *testing.T) {
	var s Species
	if err := yaml.Unmarshal([]byte("speciesid: 900\nname: test\n"), &s); err != nil {
		t.Fatal(err)
	}
	if !s.CanSpeak() {
		t.Fatal("a species with no speech key must be able to speak")
	}
}

func TestSpeechFalseIsRead(t *testing.T) {
	var s Species
	if err := yaml.Unmarshal([]byte("speciesid: 901\nname: test\nspeech: false\n"), &s); err != nil {
		t.Fatal(err)
	}
	if s.CanSpeak() {
		t.Fatal("speech: false must make CanSpeak false")
	}
}
```
Check which yaml package `internal/species` uses (`grep -n "yaml" internal/species/*.go`) and import that one.

- [ ] **Step 2:** `go test ./internal/species/ -run TestSpeech -count=1` → FAIL (undefined `CanSpeak`).

- [ ] **Step 3: Implement.** A `bool` with `omitempty` cannot tell absent from false, so use a pointer:

```go
	// Speech is false for a species with no voice (beasts, slimes, elementals,
	// constructs). Absent means it speaks. Read through CanSpeak; a
	// characters.SpeechState check decides how a taunt reads (M6 slice 3).
	Speech *bool `yaml:"speech,omitempty"`
```
and

```go
// CanSpeak reports whether this species has a voice. A species file with no
// speech key speaks.
func (s *Species) CanSpeak() bool {
	return s == nil || s.Speech == nil || *s.Speech
}
```
Check `Species.Save()` still round-trips (a nil pointer omits the key).

- [ ] **Step 4: Data.** Add `speech: false` (top level, beside `tameable:`) to these files in `_datafiles/world/dogmud/species/`: `10-rodent, 11-feline, 12-insectoid, 13-fish, 14-carnivorous_plant, 15-fungal_colony, 16-slime, 17-arachnid, 18-worm, 19-dummy, 20-orb, 21-reptile, 22-bat, 23-aberration, 24-mustelid, 2-canine, 30-skeleton, 31-zombie, 35-flesh_golem, 36-water_elemental, 37-earth_elemental, 38-air_elemental, 39-fire_elemental, 3-bear, 40-magma_elemental, 41-sand_elemental, 42-storm_elemental, 43-ice_elemental, 44-smoke_elemental, 45-automaton, 6-boar, 7-deer, 8-serpent, 9-raptor` (34 files). Leave `0-ghostly_spirit, 1-human, 32-wraith, 33-spectre, 34-vampire, 4-troll, 5-goblin, 99-ascended` untouched. Also check `_datafiles/world/default/species/` exists; if it does, leave it alone (default world).

- [ ] **Step 5:** Add a test that loads the shipped dogmud species dir (anchor with `runtime.Caller`; mirror how other species tests load, if any) and asserts `2-canine` cannot speak and `1-human` can. Run `go test ./internal/species/ . -count=1` → PASS. Null probe: delete `speech: false` from canine, confirm fail, restore.

- [ ] **Step 6:** `internal/species/context.md`: one line for `Speech` / `CanSpeak`.

- [ ] **Step 7: Commit** `feat(species): a species can have no voice (speech: false)`.

---

### Task 2: `silenced` flag and `characters.SpeechState`

**Files:** Modify `internal/conditions/conditionspec.go`; create `internal/characters/speech.go`, `internal/characters/speech_test.go`; update `internal/characters/context.md`, `internal/conditions/context.md`.

- [ ] **Step 1:** Add to the flag constants (beside `Quiet`), and to `conditions.AllFlags`:

```go
	// Silenced: the bearer cannot make a sound. Taunt and howl are refused
	// (characters.SpeechState). Nothing sets it yet; #499 adds the spell.
	Silenced Flag = `silenced`
```
Find `AllFlags` (`grep -n "AllFlags" internal/conditions/*.go`) and add `Silenced`.

- [ ] **Step 2: Failing tests** `internal/characters/speech_test.go`. Use the existing condition test seeding (`conditions.SeedConditionsForTest`, as in `internal/combat/darkness_penalty_verdict_test.go` `verdictObserver`) to apply a condition carrying `conditions.Silenced`, and set `SpeciesId` to a speechless and a speaking species via a seeded species registry (find the species test seam: `grep -rn "func .*ForTest" internal/species/`; if none exists, add `species.SetSpeciesForTest(map[int]*Species) func()` in a `_test`-reachable way, i.e. an exported helper in a non-test file whose name ends in `ForTest`, matching project convention). Cases: speaking species, no flag → `Speaks`; speechless species → `Speechless`; speaking species with the flag → `Silenced`; speechless species with the flag → `Silenced` (silence wins); unknown species id → `Speaks`.

- [ ] **Step 3:** `go test ./internal/characters/ -run TestSpeechState -count=1` → FAIL.

- [ ] **Step 4: Implement** `internal/characters/speech.go`:

```go
package characters

import (
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/species"
)

// SpeechState is whether a character can make itself heard (M6 slice 3).
type SpeechState int

const (
	// Speaks: words work; a taunt reads from the rhetoric pool.
	Speaks SpeechState = iota
	// Speechless: the species has no voice; a taunt reads from the wordless
	// pool (snarls, roars), as a howl does.
	Speechless
	// Silenced: something stops the character making any sound; a taunt or
	// howl is refused. Wins over the species.
	Silenced
)

// SpeechState answers the one question every taunt path asks.
func (c *Character) SpeechState() SpeechState {
	if c.HasFlagFromAnySource(conditions.Silenced) {
		return Silenced
	}
	if sp := species.GetSpecies(c.SpeciesId); sp != nil && !sp.CanSpeak() {
		return Speechless
	}
	return Speaks
}
```

- [ ] **Step 5:** Tests pass; `go test ./internal/characters/... ./internal/conditions/... . -count=1`. Null probe: swap the two `if` blocks, confirm "silence wins" fails, restore.

- [ ] **Step 6:** context.md lines in `internal/characters` (SpeechState) and `internal/conditions` (Silenced).

- [ ] **Step 7: Commit** `feat(characters): SpeechState, and a silenced condition flag`.

---

### Task 3: A silenced taunter is refused before paying

**Files:** Modify `internal/actions/combat_taunt.go`, `internal/usercommands/taunt.go`; tests in `internal/actions/` and `internal/usercommands/`.

- [ ] **Step 1:** Add `Silenced bool` to `TauntResult` (doc: "the taunter cannot make a sound; nothing was paid"). In `ExecuteTaunt`, directly after the `IsActing` check and before target resolution:

```go
	// A silenced taunter cannot shout, snarl or howl (M6 slice 3). Refused
	// before the target, the cooldown and the cost, so nothing is spent.
	if char.SpeechState() == characters.Silenced {
		return TauntResult{Silenced: true}
	}
```

- [ ] **Step 2:** In `usercommands/taunt.go`, after the `Crafting` branch:

```go
	if result.Silenced {
		user.SendText(messaging.CategorySystem, "You cannot make a sound.")
		return true, nil
	}
```
Mob `taunt` and `howl` already `return` when `!result.Executed`, so a silenced mob skips its turn; verify by reading both and keep it that way.

- [ ] **Step 3: Tests.** In `internal/actions`, find existing `ExecuteTaunt` tests (`grep -ln "ExecuteTaunt" internal/actions/*_test.go`) and add one: a silenced actor gets `Silenced: true`, `Executed: false`, and its conviction is unchanged (nothing paid). In `internal/usercommands`, a silenced player's `taunt` prints "You cannot make a sound." (follow `taunt_anonymize_test.go` for fixtures). Null probe: move the silence check below `admitFullCost`, confirm the "nothing paid" assertion fails, restore.

- [ ] **Step 4: Commit** `feat(taunt): a silenced character cannot taunt or howl, and pays nothing`.

---

### Task 4: The wordless pool

**Files:** Create `_datafiles/world/dogmud/taunt-messages/wordless.yaml`; modify `internal/combat/taunt_messages.go`; tests; golden.

- [ ] **Step 1: Pool file.** Same header comment style and structure as `rhetoric.yaml`: `optionid: wordless`, bands `hit`, `miss`, `critical`, `fumble`, each with index-paired `actor`, `actee`, `observer` lists of at least 6 lines. Same tokens; `hit` and `critical` carry ` (<ansi fg="damage">{damage}</ansi>)` like rhetoric's. Content: snarls, roars, growls, hisses, bared teeth, a raised hackle, a low rumble, a piercing shriek. No words, no speech, and no specific body part a species may lack (no fangs, horns, mane); "teeth" and "throat" are acceptable only if the plan reviewer agrees, so prefer sound and posture. `actor` lines are "You ..." (a speechless player, after a future shapeshift). Fumble lines: the challenge fails ("Your snarl breaks into a cough"). Miss lines: the target is unmoved. Read `rhetoric.yaml` first and match its line lengths and colour markup.

- [ ] **Step 2: Loader API.** Add, beside `GetTauntTriad`:

```go
// Taunt pool names: the taunt-messages file a taunt narrates from.
const (
	TauntPoolRhetoric = "rhetoric" // spoken barbs
	TauntPoolWordless = "wordless" // snarls and roars: howl, and any speechless taunter
)

// GetTauntTriadFrom is GetTauntTriad for a named pool.
func GetTauntTriadFrom(pool string, intensity TauntIntensity, source, target, sourceType, targetType, damageDesc string, picker ...narration.Picker) TauntTriad
```
Move the body of `GetTauntTriad` into `GetTauntTriadFrom` (replacing `tauntMessages["rhetoric"]` with `tauntMessages[pool]`) and make `GetTauntTriad` call `GetTauntTriadFrom(TauntPoolRhetoric, ...)`. Check `LoadTauntMessageFiles` requires `rhetoric` to exist; require `wordless` too if it does (and update any test fixture map at `taunt_messages.go:218`).

- [ ] **Step 3: Tests.** In `internal/combat/taunt_messages_test.go`: the shipped store loads both pools; `GetTauntTriadFrom(TauntPoolWordless, ...)` renders all three roles for every band; an unknown pool returns the zero triad. Golden: add a `wordless` section to the taunt golden in `internal/narration/snapshot_test.go` (find the `taunt_messages` subtest; extend its builder to cover every file in the dir, or add a pool), then `go test ./internal/narration/ -run TestSnapshotStores/taunt_messages -update -count=1` and read the diff.

- [ ] **Step 4: Gate** `go test . ./internal/combat/... ./internal/narration/... -count=1` (the root prose guards scan `taunt-messages`? check `proseDashDirs`; if not listed, run the dash and spacing checks on the new file by hand with the slice 2 counter script).

- [ ] **Step 5: Commit** `feat(taunt): a wordless pool of snarls and roars`.

---

### Task 5: Howl and taunt read the right pool

**Files:** Modify `internal/mobcommands/taunt.go`, `internal/mobcommands/howl.go`, `internal/usercommands/taunt.go`; tests in `internal/mobcommands/`, `internal/usercommands/`.

- [ ] **Step 1: One mob flow.** Factor `Taunt`'s body (`internal/mobcommands/taunt.go:21-104`) into

```go
// runMobTaunt is the shared narration flow of a mob taunt and a mob howl:
// the same conviction attack, read from pool. legacy holds the fallback
// literals used only when the store is empty (unit tests that never load
// world data). aggroLine is the room line for an aggro pull.
func runMobTaunt(mob *mobs.Mob, room *rooms.Room, pool string, legacy mobTauntLegacy, aggroLine string) (bool, error)
```
with `mobTauntLegacy` a small struct of the fumble / hit-personal / hit-room / miss-personal / miss-room format strings. `sendMobTauntTriad` gains a `pool string` parameter and calls `combat.GetTauntTriadFrom(pool, ...)`. `Taunt` becomes `runMobTaunt(mob, room, poolFor(&mob.Character), tauntLegacy, "...wheels around and locks onto...")`; `Howl` becomes `runMobTaunt(mob, room, combat.TauntPoolWordless, howlLegacy, "...turns from its prey and snarls at...")` with its current literals as `howlLegacy`. `poolFor(c)` returns `TauntPoolWordless` when `c.SpeechState() == characters.Speechless`, else `TauntPoolRhetoric`. Keep `sendChannelDefenceMessages(..., "taunt"|"howl")` per caller. Behaviour for every other path must be unchanged; read both functions line by line against the result.

- [ ] **Step 2: Player taunt.** In `usercommands/taunt.go` where it calls `combat.GetTauntTriad(...)` (`:203`), use `combat.GetTauntTriadFrom(pool, ...)` with the same `poolFor` rule on `user.Character` (put `poolFor` somewhere both packages can call: a method `(c *Character) TauntPool() string` in `internal/characters/speech.go` returning `"wordless"`/`"rhetoric"` would import combat's names; to avoid an import cycle, return the plain strings there and keep `combat.TauntPool*` equal to them, with a test asserting they match).

- [ ] **Step 3: Tests.** Mob: with the taunt store loaded, a speaking mob's taunt room line is from the rhetoric observer pool; a speechless mob's (species `speech: false`) is from the wordless pool; a howl is always wordless. Player: a speechless player's taunt reads wordless (seed a speechless species on the player). Null probe: make `poolFor` always return rhetoric; confirm the speechless cases fail.

- [ ] **Step 4: Gate** `go test . ./internal/mobcommands/... ./internal/usercommands/... ./internal/combat/... -count=1`.

- [ ] **Step 5: Commit** `feat(taunt): howl and speechless taunters read the wordless pool`.

---

### Task 6: A body where it fell

**Files:** Modify `internal/hooks/Death_MobLoot.go`, `internal/hooks/Death_PlayerCorpse.go`; create `internal/hooks/corpse_line.go` and `internal/hooks/corpse_line_test.go`.

- [ ] **Step 1: Helper** `internal/hooks/corpse_line.go`:

```go
// announceCorpse tells the room a body was left behind, only to readers who
// can see it (M6 slice 3, ledger row 15): a reader who sees the room clearly
// reads the named line, a reader who makes out only shapes reads the
// anonymous one, and a reader who sees nothing reads nothing (the collapse
// sound already told them). Called only where a corpse is created.
func announceCorpse(room *rooms.Room, identity string, plainName string)
```
`identity` is the tagged name (`<ansi fg="mobname">Name</ansi>` or `username`). Lines: `"<identity>'s body lies where it fell."` and `"A body lies where it fell."`. Send per reader by sight: iterate the room's players and use `messaging.ReaderSight(u.Character, room)` (SightFull → named, SightShapes → anonymous, SightNone → skip) with `u.SendText(messaging.CategoryDeath, ...)`, or a room sender that already does per-tier text if one exists (read `rooms.go:281-620`; prefer an existing seam). Exclude nobody (the dead player has no client in the room as a corpse; check `wirePlayerCorpse` to see whether the dead player is still listed in the room and exclude them if so).

- [ ] **Step 2: Wire.** In `dropMobLootAndSetCorpse`, inside the `if config.Death.CorpsesEnabled {` branch after the corpse is added to the room, call `announceCorpse(room, mobIdentity, m.Character.Name)`. In `wirePlayerCorpse`, after the corpse is placed, the same for the player. Check the hook order: `Death_MobBroadcast` ("has died.") runs before `Death_MobLoot`, so the body line follows the death line; confirm by reading how death hooks are registered.

- [ ] **Step 3: Tests** `corpse_line_test.go`, using the hooks package's room fixtures (`seedAllRegistries` / `darken` as in `grapple_personal_sight_test.go`): a full-sight reader gets the named line, a shapes reader the anonymous line, a no-sight reader nothing; plus a test that `dropMobLootAndSetCorpse` with `CorpsesEnabled` false sends no body line. Null probe: send the named line to every reader; confirm the shapes case fails.

- [ ] **Step 4: Gate** `go test . ./internal/hooks/... -count=1`.

- [ ] **Step 5: Commit** `feat(death): a body line follows a death when a corpse is left`.

---

### Task 7: Cast-start variety

**Files:** `_datafiles/world/dogmud/casting-messages.yaml`; golden `casting_messages.golden`.

- [ ] Grow `cast_started` from 3 to 10 variants: same `{spell}` token, same register (folds, threads, focus, the shape of a working). Keep the existing three first. Re-record `go test ./internal/narration/ -run TestSnapshotStores/casting_messages -update -count=1`, read the diff, run `go test . ./internal/spells/... ./internal/narration/... -count=1`. Commit `copy(casting): more ways to begin a cast`.

---

### Task 8: Deflected-swing pools

**Files:** `_datafiles/world/dogmud/defense-messages/{dodge,parry,block}.yaml`; `internal/items/defensive_messages.go`; `internal/combat/combat_helpers.go` (`deflectedSwingLines`); tests; goldens.

- [ ] **Step 1: Shape.** Add to `DefenseMessageGroup` an optional `Deflected *DeflectedPool \`yaml:"deflected,omitempty"\`` with `Actor []string \`yaml:"actor"\`` and `Actee []string \`yaml:"actee"\``; `Validate` requires equal lengths and at least 4 when present. A generic fallback lives in a new `_datafiles/world/dogmud/defense-messages/deflected.yaml` with `optionid: deflected` holding only the `deflected:` section, if the loader accepts a group with no bands; otherwise put the generic pool in `dodge.yaml` under a second key. Decide by reading `DefenseMessageGroup.Validate` and report the choice.
- [ ] **Step 2: Content.** Variant 0 of each pool is today's sentence, written with tokens: actor `{actee} dodges your swing, but you still connect! (<ansi fg="damage">{damage}</ansi>)`, actee `You dodge {actor}''s swing, but it still catches you! (<ansi fg="damage">{damage}</ansi>)`, and likewise parry, block, and the generic "turns ... aside". Add at least three more pairs per pool that still name the defence and that damage landed.
- [ ] **Step 3: Wire.** `deflectedSwingLines` picks one index for the defence's pool (generic when the defence is empty or the pool is missing), renders both roles through `narration.Render` with `{actor}`, `{actee}`, `{damage}`, and returns them; with the store unloaded it returns today's literal text (tests that never load world data). Keep the signature.
- [ ] **Step 4: Tests.** Store load and validation; `deflectedSwingLines` with the store loaded returns a pair from the matching pool (same index both roles) for dodge, parry, block and an empty defence; unloaded returns today's text. Null probe. Goldens: `defense_messages` (and `combat_messages` if it captures these lines) re-recorded and read.
- [ ] **Step 5: Commit** `feat(combat): a deflected swing reads from a pool per defence`.

---

### Task 9: The mob craft line names the item

**Files:** `internal/mobcommands/craft.go`; test.

- [ ] In the `result.Initiated` branch, replace "begins working on something." with "begins working on <item>." where the item is the recipe output's display name from `result.Recipe` (find the field the `ImmediateComplete` branch's `MobRoomLine` uses for the output; wrap it in `<ansi fg="itemname">`). Send through the same visual sender with the mob name hidden as today. If the recipe has no output name, keep "something". Test: an initiated mob craft room line contains the item name. Commit `copy(craft): a mob says what it starts crafting`.

---

### Task 10: Ironwind gossip and fact lines

**Files:** `_datafiles/world/dogmud/gossip_templates.yaml`, `_datafiles/world/dogmud/facts.yaml`; create `internal/gossip/facts_capital_test.go` (or a root guard); goldens (`gossip`).

- [ ] **Step 1:** Delete the `IronwindEvent-Regional` and `IronwindEvent-Global` blocks. Check nothing else references the key (`grep -rn IronwindEvent`).
- [ ] **Step 2:** Rewrite the seven descriptions to start lowercase and read after any lead-in ("They say ...", "Word is, ..."), e.g. `wolves have been prowling out of the Deep Woods, taking livestock from the outlying farms.` Read the three `fact-default` templates first to check every lead-in.
- [ ] **Step 3: Guard test.** Load `facts.yaml` (anchor with `runtime.Caller`); fail on any description whose first word starts with a capital unless it is `A`, `An`, `The` (lowercased at render by `LowerLeadingArticle`) or on an allowlist of proper nouns (`Thornwall`, `Elgar`, `Delia`). Null probe: capitalise one back.
- [ ] **Step 4:** Re-record the gossip golden, read it, full gate including `go test ./...`. Commit `copy(gossip): retire dead Ironwind lines; fact lines read after a lead-in`.

---

### Task 11: Parry counters fit any weapon

**Files:** `_datafiles/world/dogmud/defense-messages/counter-parry.yaml`; golden.

- [ ] Rewrite each line naming a cut, point, thrust or arm so it fits a mace, staff or blade alike ("turn the blow and drive back in", "check the strike and answer with your own"). Keep index pairing across `actor`/`actee`/`observer` and bands. Add a check to an existing guard or a test: no whole-word `cut|cuts|point|thrust|thrusts` in `counter-parry.yaml`. Re-record `defense_messages.golden`, read it. Commit `copy(counters): parry counters fit any weapon`.

---

### Task 12: Docs

- [ ] Ledger rows 3, 4, 6, 14, 15, 20, 23, 25, 35: `done (PR #___, M6 slice 3)` with a short note; fill the number once the PR exists.
- [ ] `docs/README.md` rows for this plan, `taunt-messages/wordless.yaml`, and any new test guard.
- [ ] `docs/PATCH_NOTES.md` Unreleased bullets: beasts snarl and roar instead of trading words; a body line after a kill; more variety in casting and in dodged-but-hit swings; parry counters fit any weapon; gossip reads cleanly.
- [ ] context.md: species, characters, conditions, combat (taunt pools, deflected pools), mobcommands (runMobTaunt), hooks (announceCorpse).
- [ ] Commit `docs: M6 slice 3 ledger, index, patch notes`.

---

### Task 13: Gate, PR, playtest

- [ ] `dogmud-shipping` gate: gofmt, lint (`--new-from-merge-base`), full `go test ./...`, YAML shape check against master for changed data files, isolated boot check.
- [ ] Final whole-branch review (subagent), fixes, then PR with `--repo pruuk/DOGMud`, "Part of #370".
- [ ] Merge after green CI.
- [ ] Playtest (`dogmud-playtesting`, profile `mid`, not `veteran`): a wolf howls; a speechless mob taunts; a kill in light and in dim light shows the body line (and none with no sight); a few casts; a dodged-but-hit swing. File findings as issues.
