# Follow-up slice F: mobs perceive darkness

Slice F of the messaging M3 item 5a follow-ups, the sibling of slice A. Slice A
made **players** unable to name or aim at what they cannot see. Mobs were
deliberately left untouched, because restoring their sight before restoring
their night vision would have left cave dwellers unable to fight in their own
caves. This slice closes that asymmetry in one go.

## Facts verified against source, 2026-09-11

| Fact | Evidence |
|---|---|
| Buff 29 (Night Vision) is **absent from dogmud** | Only `_datafiles/world/default/buffs/29-night_vision.yaml` exists; there is no `dogmud/buffs/29-*` |
| Its flag is spelled correctly and is really consumed | The file says `flags: [nightvision]`; `buffs/buffspec.go:62` declares `NightVision Flag = "nightvision"`; `messaging/predicates.go` `CanSeeClearly` ends by returning `observer.HasFlagFromAnySource(buffs.NightVision)` |
| Mobs already compute sight in combat | `combat/combat.go:147-148` and `:197-198` build `combatContext` from `messaging.CanSeeSightImpairedOnly(&mob.Character, room)` |
| The penalty at stake | `Balance.DarknessCombatPenalty` is **0.80**, applied to attack (`combat_helpers.go:564`) AND defence (`:761`); shipped value at `config.yaml:863` |
| **8** species reference buff 29, not 2 | `2-canine` 20 mobs, `8-serpent` 19, `9-raptor` 10, `17-arachnid` 6, `5-goblin` 5, `11-feline` 4, `24-mustelid` 2, `4-troll` 1, so **67 mobs** of the 641 carrying a `speciesid` |
| No player is affected | Six of the eight say `selectable: false`; `4-troll` and `5-goblin` omit the key and `species/species.go:46 Selectable bool` zero-values to false |
| **Most dark-room spawns have no night vision** | Of **141** spawn entries in dark-biome rooms, **115 belong to species without buff 29**, leaving **17 distinct hostile mobs** that would stop initiating. They include `228` Stone Beetle Queen (sp 12) and `229` Windscour Wyrm (sp 23), and `272` Chrysalis Phantom (sp 1), an ambusher |
| **A light source cancels the darkness for everyone in the room** | `rooms/rooms.go:182-189`: `GetVisibility()` adds +1 when `GetMobs(FindHasLight)` or `GetPlayers(FindHasLight)` is non-empty. `FindHasLight` is satisfied by `HasFlagFromAnySource(buffs.EmitsLight)` (`rooms.go:1648`, `:1744`) |
| The light chain is real end to end | `buffs.EmitsLight = "lightsource"` (`buffspec.go:60`); buff 1 "Illumination" carries `flags: [lightsource]`; spell `chrysalis-glow` (alias `glow`) is `effect_type: buff` with `buff_ids: [1]` |
| **Every player starts with the light spell, and so does every mob** | `characters/character.go:360` `StarterSpells` lists `chrysalis-glow` as the "light source for caves", granted at creation **and to every mob at `Validate()`** (actor parity, 2026-07-10) |
| Blast radius of the restore is bounded | 121 rooms carry a dark biome (`cave` or `dungeon`); at least 14 of them spawn one of the 8 species. **A floor, not a precise count**: the scan reads room spawn lists only, so wander, patrols and schedules are not counted |
| Species buff ids are **never validated** | `species.Validate()` (`species/species.go:105`) checks name, description and size, recalculates stats and handles damage dice. It never looks at `BuffIds` |
| The guard has a precedent to mirror | `species.ValidateBodyPartTags(mutationIdExists func(id string) bool)` (`species/species.go:322`) panics at boot on an unknown id, taking an injected checker |
| Boot order supports the guard | `main.go:1635 buffs.LoadDataFiles()`, `:1640 species.LoadDataFiles()`, `:1691 mutations.ValidateBodyPartTags()`, `:1693 species.ValidateBodyPartTags(mutations.HasSpec)` |
| `buffs` has no existence checker yet | It exposes `GetBuffSpec(int) *BuffSpec` (`buffspec.go:163`) and `GetAllBuffIds()` (`:175`). `mutations.HasSpec(id string) bool` (`mutations.go:756`) is the shape to copy |
| **Mob decision code ignores light entirely** | Proven with a control: `IsHidden` or `Perceives` matches **14** times across `internal/behaviortree` and `internal/mobs`, while `CanSee`, `roomIsLit`, `NightVision`, `DarkArea` or `IsLit` matches **0**. The pattern was capable of matching and found nothing |
| `aiprofile` is **not** a sight surface | It is a mob YAML field (`mobs/mobs.go:115 AIProfile string`). It selects combat move preferences, not targets. There is no `internal/aiprofile` package |
| Mob attack has one hook | `mobcommands/attack.go:42` calls `actions.FindAttackTarget(rest, room, 0, mob.InstanceId, nil)` with a **nil viewer**, registered in the slice A lookup guard as a deliberate mob exception |
| Mob casting has one hook | `mobcommands/cast.go:43` calls `actions.InitiateCast`, the same entry players use; `actions/cast_admission.go:47` early-returns when the actor is not a player or the room is nil |
| Scout detection is the pre-slice-A pattern | `behaviortree/conditions_scout.go:21 condRoomHasHiddenEntity` loops `GetPlayers()` and `GetMobs()` and returns Success on any raw `IsHidden()`, with no perception rule. Used by 2 trees |

### Where mobs scan the room with no sight filter

| Site | Function | Trees | Gate? |
|---|---|---|---|
| `behaviortree/conditions_player.go:192` | `condMultipleEnemies` | 9 | yes |
| `behaviortree/conditions_player.go:115` | `condPlayersInRoom` | 3, all hostile (`archetypes/ambusher`, `254-bandit_leader`, `272-chrysalis_phantom`) | yes |
| `behaviortree/actions_party.go:235` | `engageHostilePlayerInRoom` | party engage | yes |
| `behaviortree/actions_archer.go:138` | `archerMeleeEngaged` | archer | **no** |
| `behaviortree/conditions_scout.go:21` | `condRoomHasHiddenEntity` | 2 | yes, through `Perceives` |
| `behaviortree/actions_mob.go:342` | `actSweepCompanions` | 1 (`9587-hull_sweeper`) | **no** |
| `behaviortree/conditions_player.go:137` | `condPlayerInRoomMissingQuest` | 3 (Dewey 9491, Cleric Hadwen 9100, room 6467) | **no** |
| `behaviortree/conditions_player.go:165` | `condPlayerInRoomHasQuest` | 1 | **no** |

Four sites are deliberately left ungated. The two quest-giver conditions,
because gating them would stop NPCs offering quests in an unlit room and would
regress the newbie chain. And `actSweepCompanions` and `archerMeleeEngaged`, because neither
selects a target. The sweep relocates every player-in-room's companions to
another room via `hooks.PushCompanionsToRoom`, iterating players only to find
whose companions to move; the archer check reports that a fight ALREADY exists
(its own doc calls it "pinned in melee", and its only caller decides whether to
kite or hold a reload). Gating either would break a hull sweeper in the dark, or
leave a blind archer shooting point-blank instead of kiting, for no
perception-related reason.

## Owner rulings (2026-09-11)

1. **All 8 species get night vision restored** (67 mobs). The species data has
   declared `buffids: [29]` all along and it silently did nothing, so this
   repairs a broken reference rather than granting a new power. No player is
   affected.
2. **The species buff-id guard ships in this slice.** It is the same defect
   class that hid this for months.
3. **Scout hidden-detection routes through `Perceives`**, the rule slice A
   introduced, so mobs and players cannot disagree about who is visible.
4. **A mob that cannot see picks no NEW target by sight, and fights already
   under way continue** at the existing 0.80 penalty. Nothing mid-combat
   changes, and a cave ambush still works once joined.
5. **The quest-giver conditions stay ungated** ("fine for now").
6. **A short playtest confirms the cave bosses still attack**, run up into the
   caves east and north of Thornwall.

## The design

**One predicate, already shared.** Mob sight uses
`messaging.CanSeeSightImpairedOnly(&mob.Character, room)`, exactly what combat
already calls. Nothing new is invented, and restoring buff 29 flows through it
automatically, because that predicate's last line reads the `NightVision` flag.

**Perception of a creature uses `characters.Character.Perceives`**, the rule
slice A added for the room listing and every player lookup. Scout detection and
mob target selection both route through it, so "can this creature be picked"
has exactly one answer in the codebase.

Four changes:

1. **Restore buff 29 into dogmud.** Copy
   `world/default/buffs/29-night_vision.yaml` to
   `world/dogmud/buffs/29-night_vision.yaml`. The flag spelling is already
   verified against `buffspec.go:62`, so this cannot ship inert the way the
   Cat's Eye Draught did when its flag was misspelled.
2. **Gate the four hostile scan sites** on the acting mob's sight, leaving the
   two quest-giver conditions and the companion sweep alone. A mob that cannot
   see the room finds no candidates and does not engage.
3. **Give the two single hooks a viewer.** `mobcommands/attack.go:42` passes
   `&mob.Character` instead of `nil`, and `cast_admission.go` stops
   early-returning for mobs, so a mob's targeted cast needs sight the way a
   player's does. The slice A lookup registry entry for the mob caller changes
   from a deliberate exception to a filtered call, which the root guard then
   enforces.
4. **Add the boot guard.** `buffs.HasSpec(id int) bool` mirroring
   `mutations.HasSpec`, and
   `species.ValidateSpeciesBuffIds(buffIdExists func(int) bool)` mirroring
   `ValidateBodyPartTags`, called from `main.go` immediately after line 1693. It
   panics at boot on a species referencing a buff that does not exist, which is
   exactly what would have caught this on the day it broke.

## Testing

Unit tests per surface, each with a sabotage probe proven to turn its test red
before it is trusted, per the standing rule that a null probe must be shown
capable of failing.

- A mob in a dark room with no night vision picks no target; the same mob
  holding buff 29 does.
- **A mob in a dark room where someone carries a light DOES pick a target.**
  This is the case that keeps the cave bosses working, and it is the one most
  easily broken by gating on biome instead of on `GetVisibility()`.
- A mob already engaged keeps fighting when the light goes out.
- A quest giver still offers its quest in an unlit room, the regression the two
  ungated conditions exist to prevent.
- A hull sweeper still relocates companions in the dark.
- A blind archer still knows it is pinned in melee, so it kites rather than
  shooting point-blank.
- A blind archer still knows it is pinned in melee and kites.
- A scout does not detect a hider it cannot perceive, and does with see-hidden.
- The boot guard panics on a species referencing a missing buff id, proven red
  by pointing a fixture species at a nonexistent id.

### Playtest lane: the Ironwind cave bosses

One short run, on the `veteran` profile (it carries `chrysalis-glow: 64`, so it
can light a cave; a fresh character also starts with the spell at rank 1).

- Room **3112 Beetle Queen's Chamber** (`biome: cave`) holds `228` Stone Beetle
  Queen, and room **3114 Wyrm's Den** (`biome: cave`) holds `229` Windscour
  Wyrm. Both are hostile, and **neither species has night vision**, so they are
  exactly the encounters this slice could break.
- Walk in normally, with the glow up. **Both must still attack.** That is the
  pass condition.
- Then let the glow lapse and re-enter. The expected result is that they do not
  initiate, while attacking one still starts a fight that runs to its end.
  Record what the room reads like, because this is the new player-visible
  behaviour and the owner has not seen it yet.

## Risks

| Risk | Answer |
|---|---|
| **82% of dark-room spawns lack night vision, so cave bosses stop attacking** | They only go passive when **nobody in the room carries light**, because `GetVisibility()` adds +1 for any light-bearing player or mob. Every player starts with `chrysalis-glow`, and so does every mob. Normal play is unchanged; the playtest lane above exists to prove it |
| A player sneaks in unlit and the boss never fights | Symmetric with slice A, where that same player cannot target anything either, and the mob still fights back once attacked. Treated as a stealth option, not a defect |
| 67 mobs gaining a 25% effective uplift in the dark unbalances encounters | It applies only in dark rooms, and the uplift is the removal of a penalty the data always said should not apply |
| The ungated sites look inconsistent | Deliberate and recorded above with reasons. Quest giving and companion relocation are not targeting |
| `condPlayersInRoom` is generic and could gain a peaceful caller later | The root lookup guard cannot see behaviour-tree conditions, so the plan adds a test naming the three hostile trees; a future peaceful caller then shows up as a failing expectation |

## Out of scope

- Mob narration. Mobs do not read text, so slice A's name hiding has no mob side.
- The other slices: B (red names), C (buff expiry notice) and D+E (Purge
  Affliction targeting and the buff flag name guard) stay separate.
- The `hints` to `tips` rename, designed separately and deferred to M3 item 7.
