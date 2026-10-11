# Loose issues sweep 3 (2026-10-10)

Four issues in one branch (`fix/loose-issues-sweep-3`, from master 592dfee5f):
ASCII forms for glyphs that carry meaning (the #253 follow-up), a condition of
its own for Blood Boil (#249), two commands missing from the help index
(#236), and the arrest text order plus the stale arrest (#241). None of it
touches the files reserved for #465 (`internal/combat` damage pipeline files,
`config.balance*.go`, the combat hooks and actions, `throw.go`,
`templatesfunctions.go`, status templates, `config.yaml`).

Owner calls 2026-10-10: the ASCII forms for ☺ ☹ ⚷ → … ± × ↑ ↓, with ☺ `P`
and ⚷ `K` (`@` and `L` collide); the ten biome letters as proposed; the NPC,
Player and Mob map markers turn on at map level 4; Blood Boil gets its own
condition, Neural Toxin keeps Poisoned; the `bleeding` flag reads "bleeding"
for Blood Boil and combat bleeds alike; Purge Affliction and Cleansing Wave
still cure Blood Boil; investigate the stale arrest, and a declaration lapses
when the player walks off during the grace.

## Facts verified against source (master 592dfee5f)

| Issue | Fact | Where |
|---|---|---|
| #253f | `unicodeToAscii` is `map[rune]string`, so one glyph may become several characters; `ConvertToAscii` runs once per recipient on every write, map and legend included | `internal/util/util.go:1135,1250`, `internal/connections/connectiondetails.go:315-316` |
| #253f | None of ☺ ☹ ⚷ → … ± × ↑ ↓ is in the table; ▲ ▼ already map to `^` `v`; em and en dash to `-` | `util.go:1277,1294` |
| #253f | Map markers: `@` You, `☺` Party Member, `☹` Friend (a charmed mob); `⚷` is `mapper.LockedSymbol`, legend "Locked" | `internal/usercommands/skill.map.go:159,163,168`, `internal/mapper/mapper.go:20` |
| #253f | The NPC, Player and Mob markers (`☺`, `mobMapSymbol`) sit under `skillLevel > 4`, but `skillLevel` tops out at 4, so they never draw; `help map` still lists them | `skill.map.go:42-49,136-150`, `templates/help/map.template:19-23` |
| #253f | `L` is already a room symbol (the Loom); `@` is You; `&`, `P`, `K` are unused by rooms, biomes and legend overrides in the live world | `rooms/thornwall_city/480.yaml:10`, `biomes/*.yaml`, `keywords.yaml:380-389` |
| #253f | Ten biome symbols have no ASCII form and reach an ASCII client as Unicode: ⌬ ♠ ∴ ⩕ ⁖ ≋ ⌇ 🕸 ♨ ♣ | `biomes/*.yaml` `symbol:`, `util.go:1250-1298` |
| #253f | The legend is keyed by the original rune and printed with `%c`, so it converts with the map; two glyphs that convert to one letter print two legend rows | `skill.map.go:175`, `templates/maps/map.template:17-22` |
| #253f | → in text: skill banner, admin devtool and relationship output, help combat, map and set-prompt; none inside a padded column except where every row shifts alike | `internal/banner/banner.go:61`, `admin.devtool.go:124-126`, `admin.relationship.go:93,205` |
| #253f | … ends five truncations; three sit in `%-Ns` padded admin columns, where `...` would push a cut row two columns wide; `admin.locate.go` cuts by byte, which can split a rune | `admin.bounty.go:342`, `admin.fact.go:333`, `admin.locate.go:130,138`, `petitions.go:94` |
| #253f | Lock grid cells are five-wide strings `"  ↑  "` / `"  ↓  "`, so single-character `^` / `v` keep the grid aligned | `internal/usercommands/picklock.go:340,342` |
| #253f | ± appears only in `Damage.String()` (no padded caller); × only in admin devtool output and help | `internal/items/itemspec.go:403`, `admin.devtool.go:126` |
| #249 | `applySpellDot` applies `conditions.ConditionIdPoisoned` (121) at :355 through the silent character door, then narrates with its own Go trio at :360-368 | `internal/hooks/spell_effects.go:327-370` |
| #249 | Only `blood-boil.yaml` and `neural-toxin.yaml` carry `effect_type: dot` | `_datafiles/world/dogmud/spells/*.yaml:15` |
| #249 | Spells already name conditions with `condition_ids` (`[]int`); 29 spells use it | `internal/spells/spells.go:50` |
| #249 | Condition ids in use: 0-3, 5-7, 9, 15, 24-142; 135-142 are taken by M6 slice 1; 143 is free | `_datafiles/world/dogmud/conditions/` |
| #249 | `GetAdjectives` adds `poisoned` for the Poison flag and nothing for the `bleeding` flag | `internal/characters/description.go:166-168`, `internal/conditions/conditionspec.go:92-93` |
| #249 | A dot kill with no Poisoned or Bleeding record falls back to the tick cause, which reads the `bleeding` flag as "bleeding out" | `internal/hooks/Death_PlayerAnnouncement.go:198-203`, `internal/hooks/tick_cause.go:30` |
| #249 | Purge Affliction and Cleansing Wave cancel Poison-flag records only | `internal/hooks/spell_purgeaffliction.go:101`, `spell_help_effects.go:331` |
| #249 | `narrateConditionStart(spec, evt, holder, snap, hideOnHidden, unseenBy)` is the one sender of start lines per audience; the event hook takes `unseenBy` before the add | `internal/hooks/condition_cast_lines.go:110`, `Condition_ApplyConditions.go:97,168` |
| #249 | Spell guard: `conditionIdReaders = {"condition","shield","heal"}`; any listed effect type's conditions must author all three start lines and not be silent-start | `spell_condition_data_guard_test.go:79,251-280` |
| #249 | Line key `"internal/hooks/spell_effects.go\|355"`; a moved call fails as both unpardoned and stale | `condition_apply_path_guard_test.go:219,443-456` |
| #249 | Observer lines naming `{actor}`/`{actee}` need a file entry in `observerIdentityGuardContentSafeViaCode` (30 and 41 are there) | `shipped_narration_data_guard_test.go:1005,1075,1082` |
| #236 | `help` builds its index from `keywords.yaml`; `trade` and `tutorial` are registered, have help files, and are the only non-admin commands with a help file missing from the index | `internal/usercommands/help.go:49`, `usercommands.go:76,232`, `keywords.yaml:5-160` |
| #236 | `chat` is listed under communication, `newbie` under general | `keywords.yaml:47,156` |
| #236 | `trade.template` still carries an em dash (line 4, not 3) | `templates/help/trade.template:4` |
| #236 | PATH_TO_1.0 "Help coverage" is at line 313 (was 303) | `docs/PATH_TO_1.0.md:313` |
| #241 | Declaration and warn line are clean ("No more moving along. You're under arrest. Come quietly." / "Move along. You're not welcome here."); no dash in any justice string | `internal/justice/enforce.go:202,218-219` |
| #241 | The seizure line wraps to the reader's width (#430), tested | `internal/justice/arrest.go:416-424`, `arrest_test.go:1014` |
| #241 | Condition 88 lands at :395; its clang line is sent at :407-415, before the seizure line at :416-424; both are `SendText`, delivered in call order | `arrest.go:395,407-424` |
| #241 | Line keys `arrest.go\|395` (arrest) and `arrest.go\|645` (login re-apply) | `condition_apply_path_guard_test.go:151,154`, `arrest.go:644-645` |
| #241 | Each guard stamps `justice_arrest_pending_<uid>` on its OWN MiscData, declares when it has none, and hauls on any later sighting once the stamp is 3 rounds old (`ArrestResistGraceRounds`, unset in config.yaml) | `enforce.go:209-231,100-107`, `config.balance.mobs.go:247-248` |
| #241 | Pending stamps never expire (the prune skips them) and only the hauling guard clears its own | `enforce.go:118,230` |
| #241 | Guards wander (`maxwander` 1 or 2) | `mobs/thornwall_city/106-city_guard.yaml:9` |

## Fixes

### #253 follow-up: ASCII forms for meaningful glyphs

All forms go into `unicodeToAscii`, the existing door; no source swap is
needed because every map marker becomes one character.

- Map markers: ☺ → `P` and ⚷ → `K` (owner call; `@` is You and `L` is the
  Loom), ☹ → `&`.
- The NPC, Player and Mob markers move from `skillLevel > 4` to
  `skillLevel >= 4`, so a level-4 map draws them as `help map` already says
  (owner call). Test: a level-4 map shows a nearby mob and NPC.
- Text: → `->`, … `...`, ± `+/-`, × `x`, ↑ `^`, ↓ `v`.
- The three padded admin truncations (`bountyTruncate`, the fact list, the
  two cuts in `admin.locate.go`) end in `...` in source and cut three runes
  short, so the columns hold in both modes; `admin.locate.go` cuts by rune.
  `petitions.go` keeps `…`, which ends its line.
- Biome symbols widen this (a found flaw): each of the ten gets a one-letter
  form so an ASCII map has no Unicode left (owner approved): Cave ⌬ `O`, Dense Forest ♠ `F`, Forest ♣ `f`, Ether ∴ `:`,
  Mountains ⩕ `M`, Plains ⁖ `.`, River ≋ `=`, Sewer ⌇ `s`, Spiderweb 🕸 `X`,
  Swamp ♨ `w`. None collides with a room symbol, a code marker or a legend
  override.
- `help map` adds the locked-exit marker and an ASCII column for each marker.
  `help charset` says map markers become letters and points to `help map`.
- Test: table rows in `util_test.go` for every new entry; a map test that an
  ASCII map line keeps its rune count; the lock grid renders aligned.

### #249: Blood Boil lands its own condition

- New condition **143 Boiling Blood**, `_datafiles/world/dogmud/conditions/143-boiling_blood.yaml`:
  121's shape (`triggerrate: 1 round`, `tick_pool: health`,
  `tick_from_magnitude: true`), flag `bleeding` (not `poison`, not
  `silent-start`), and start lines per audience naming the caster. No Go
  constant: the id lives in YAML.
- `blood-boil.yaml` gains `condition_ids: [143]`, the field spells already
  use. `applySpellDot` reads `ConditionIds[0]` and falls back to
  `ConditionIdPoisoned` when the list is empty, so Neural Toxin is unchanged.
- Narration: when the DoT's condition `NarratesCastStart(false)`,
  `applySpellDot` sends its start through `narrateConditionStart` (holder
  party and `conditionLineUnseenBy` read before the add, `CasterCrit` from the
  cast) instead of its Go trio. Otherwise the trio stays, so Neural Toxin
  reads exactly as today. A hidden mob caster reads "Something's spell ..."
  through the same path the other 20 condition spells use.
- Adjective: `GetAdjectives` adds `bleeding` for the `bleeding` flag, the way
  it adds `poisoned` for Poison. This also tags combat bleeds (122), which is
  true (owner call).
- Purge Affliction and Cleansing Wave keep curing Blood Boil (owner call):
  they cancel condition 143 as well as Poison-flag records, so what players
  can do about Blood Boil is unchanged. Combat bleeds stay uncured, as today.
- Consequences: poison immunity no longer refuses Blood Boil; a Blood Boil
  death reads "bleeding out" through the tick cause.
- Guards:
  - `spell_condition_data_guard_test.go`: add `"dot"` to
    `conditionIdReaders`, so 143 must author all three start lines and not
    be silent-start; add a check that a dot spell lists at most one id.
  - `condition_apply_path_guard_test.go`: the id read adds lines above :355,
    so re-key `spell_effects.go|355` to the call's new line, and reword its
    reason (the spell narrates through the condition's own start lines, or
    its trio for a silent-start record).
  - `shipped_narration_data_guard_test.go`: add
    `"conditions/143-boiling_blood.yaml": true` beside 30 and 41.
  - `condition_notice_guard_test.go` is met by the authored `start_actee`
    and `end_actee`.
- Tests: Blood Boil lands 143 and not 121; Neural Toxin lands 121 with its
  trio unchanged; the target reads "bleeding", not "poisoned"; each audience
  reads its start line once; a hidden mob caster's line starts "Something's";
  a dot kill on Blood Boil credits the caster.

**Draft copy for the owner to review** (80 columns, no numbers):

```yaml
conditionid: 143
name: Boiling Blood
description: Blood seething in your veins, dealing damage over time.
start_actor: "You set the blood in {actee}'s veins boiling."
start_actee: "{actor}'s spell sets your blood boiling in your veins!"
start_observer: "{actor}'s spell sets {actee}'s blood boiling."
trigger_actee: '<ansi fg="red">Your blood boils and seeps from your skin!</ansi>'
end_actee: "Your blood cools at last and stops seeping."
flags:
  - bleeding
```

### #236: trade and tutorial in the help index

- `trade` under `communication`, beside `chat`; `tutorial` under `general`,
  beside `newbie`.
- `trade.template:4`: "trade channel, for buying, selling and looking for
  goods."
- Test in `helpfile_completeness_test.go`: every non-admin registered
  command with a help file is listed in the `keywords.yaml` help index or is
  a help alias, so the next one cannot slip.
- PATH_TO_1.0 "Help coverage" stays open: it is broader than the index.

### #241 (a): the seizure line first

Swap the two blocks at `arrest.go:407-415` and `:416-424`, so the player
reads "A guard seizes you..." and then "The cell door clangs shut...". The
swap keeps the line count, and both blocks sit below :395 and far above
:645, so both guard keys stay valid unchanged. New test: the seizure line
precedes the clang line. The other checklist items are fixed on master (see
the facts) and the issue closes with this.

### #241 (b): the stale arrest is a real defect

Every guard in the room declares on its own stamp, so three guards print
three declarations in one round. Nothing holds the player, and the stamps
never expire, so any of those guards that later shares a room with the
player hauls them at once, anywhere, with no new declaration. That is
"announced, then arrested 27 rounds later somewhere else."

Proposed fix, in `enforce.go` only: the pending stamp moves from each guard
to the player (`MiscData`, as `keyJailUntilRound` already is), holding the
round and the room. A guard declares only when the player holds no live
stamp, so one guard speaks; any guard in that room hauls once the grace has
run; a stamp from another room, or older than the grace plus a short window,
is dropped and a new sighting declares afresh. The haul clears the stamp.
A player who walks off during the grace lets the declaration lapse; the next
sighting declares again (owner call). Tests: three guards in a room give one
declaration; a guard met later in another room declares afresh rather than
hauling; staying through the grace is still hauled.

## Left out

#465 and its files; M7 wording (#264, #261, #283); #440 (own branch, PR
#474).

## Testing

Package tests for util, usercommands, mapper, hooks, conditions, characters,
spells and justice; the root guards; the pre-push gate; an isolated boot; a
short playtest: Blood Boil and Neural Toxin on a mob and a player, an ASCII
map and lock grid, `help`, and an arrest with three guards.
