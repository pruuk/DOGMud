# Sight gates wrap-up (#382)

**Status:** APPROVED by the owner, 2026-10-09. Owner calls: arrivals keep their behaviour, and names typed in the dark are gated.
**Parent:** `specs/completed/2026-10-09-sight-gates-followups-design.md` (shipped in #452, master
`b1c1829ec`). Its verification playtest (run `7ab93bb8f9c52e52`) closed #446, #447, #448, #216
and #444, and filed #453, #454, #455 and #456. Still open under the epic: #242, #251, #449 and
#451. This slice fixes all eight in one PR and gives #251 a deterministic test and a playtest
case that makes the scan branch run. #382 itself shows closed: PR #439's closing keyword shut it
on 2026-10-08. This PR says "Refs #382", and the epic gets a closing comment when the playtest
passes.

**Rulings that bind this work:** R4 disruptions are heard (sight, then sound); R7 the death
broadcast stays global; R8 a weapon is named only at full sight; the owner rule of 2026-10-05,
a line announcing a change is judged against the state before it (`Room.VisualSnapshot`,
`SendTextVisualToSnapshot`); 0 light is natural dark; one hide at a time, the stronger wins (#444).
New, 2026-10-09: a mover is seen by the light they carry on their own way in and out. A person
entering with a light is seen as they walk in (arrivals keep being judged after the move), and
a person leaving with a light is seen as they walk out (departures are judged before it). The
2026-10-05 rule is about a change to the room's light, such as a darkness put on. It does not
govern a mover's own arrival or departure line.

## Facts verified against source (master `b1c1829ec`)

| Fact | Where |
|---|---|
| Condition narration renders the holder with `GetCharacterName(true)` after the record lands, so the new `hidden` (or `lit`) adjective is in the name | `hooks/Condition_ApplyConditions.go:144`, `:155-157`; trigger `NewRound_UserRoundTick.go:289`; end `NewTurn_PruneConditions.go:46` |
| `GetCharacterName(true)` is `getFormattedName(0, "username").String()`: the tag, then the adjective span, the quest star and " and <pet>" | `characters/formattedname.go:164-169`, `:72-103`, `:195` |
| 29 production lines call `GetCharacterName(true)`; every one feeds narration (conditions, spell and craft token contexts, quest, sleep, arrest, disenchant) | grep, excluding `_test.go` and one comment in `messaging/anonymize.go:11` |
| Mob holders go through `mobDisplayName` (`GetMobNameIndexed`), which carries the same span | `Condition_ApplyConditions.go:157`; `NewRound_MobRoundTick.go:290`; `NewTurn_PruneConditions.go:104-106`; `NewRound_DoCombat_helpers.go:398-404` |
| `messaging.StripNameAdjectives` removes the span and keeps the tag; one caller today | `messaging/hidenames_tagged.go:45`; `combat/defence_multiplier.go:302-303` |
| `textutil` cannot import `messaging`: `messaging` imports `conditions`, which imports `textutil` | `messaging/band.go` and three more; `conditions/narration.go:45-70` |
| `FindByNameSeenBy` filters by `Perceives` only (hidden), never the viewer's sight band | `rooms/rooms.go:2050`, `:2134`, `:2199`; `characters/character.go:965-973` |
| Only `cast` applies the sight band to a typed name: none refuses, shapes takes `shape`/`N.shape` and hints, `castFigures` rewrites a shape to `@id`/`#id` | `actions/cast_admission.go:40-90`, `:108-140`; caller `actions/cast.go:99` |
| `attack <name>`, the 11 melee specials, `give` and `shoot` resolve through `FindByNameSeenBy` with no band check | `actions/combat_attack.go:119`; `actions/melee_target.go:172`; `usercommands/give.go:73`, `:387`; `usercommands/shoot.go:693-699`; `actions/combat_fire.go:189-194` |
| No sight gate in `StageMeleeTarget`, so the playtest's "kick ordel" refusal at shapes is not explained by source | `actions/melee_target.go:158-180` |
| `give` lines print raw names to giver, recipient and room; a quest NPC's return line prints `ctx.MobName` | `give.go:90-97`, `:207-210`, `:234-237`; `behaviortree/actions_quest.go:175` |
| Minimap tag built per symbol from the lower-cased legend name; biome "Deep Water" puts a space in `fg="map-deep water"` | `usercommands/look.go:700-704`; `skill.map.go:176-179`; `biomes/water.yaml`; `templates/maps/map.template:20` |
| `CategoryMobEmote` skips every normalize stage, end punctuation included; of the single-line quoted mob emotes in dogmud, 736 end without a full stop and 51 with one | `messaging/normalize.go:25-47`, `:198`; grep of `world/dogmud/mobs` |
| `claws.yaml` puts a singular verb after `{itemname}` on 8 lines; a mob's natural "claws" is plural | `combat-messages/claws.yaml:173, 230, 247, 305, 329, 353, 403, 425` |
| A mutator's description modifier is added after the description is wrapped | `rooms/roomdetails.go:110`, `:139` (wrap) vs `:185-215` (append) |
| The crit banner is `*** ` + line + ` ***`; the wrapper breaks at any ASCII space | `combat/combat_helpers.go:1802-1806`; `messaging/wrap.go:145` |
| Prompt and GMCP enemies say "an unseen foe" at shapes and at none; combat lines say "a figure" / "something" | `users/userrecord.prompt.go:539`; `gmcp/gmcp.Char.go:506`; `messaging/hidenames.go:30-41` |
| Quit line sent after `RemovePlayer`, through `SendTextVisual` | `hooks/PlayerDespawn_HandleLeave.go:150-152` |
| Four departure room lines go out after the mover has left (player `go` twice, mob forced `go`, `RelocateMob`); arrivals likewise | `usercommands/go.go:265`, `:308-321`; `mobcommands/go.go:76-92`; `actions/relocate_mob.go:98-115` |
| `SendTextVisualToSnapshot` has no sound half; `SendTextVisualWithAudio` has no snapshot | `rooms/rooms.go:419`, `:490` |
| Shroud hide re-entered on load by `reconcileShroudHide` in `Validate`, before the permanent rebuild | `characters/shroud_hide.go:188-200`; `validate.go:696`, `:720-721` |
| On load Awareness is fresh and Visible, so the rebuild finds no source for record 9 and removes it | `validate.go:605-606`; `characters/conditions.go:356-358`, `:376-381` |
| `sneaking` lives in `MiscData`, which is saved | `characters/character.go:324`; `actions/sneak.go:177` |
| Mob fold TargetGone only for a dead or missing target; a target who walked out is not gone | `hooks/combat_shared_helpers.go:661-688` |
| A mob whose user target left is released before any fold step; the fold step runs only in combat | `hooks/NewRound_IdleMobs.go:64-71`; `NewRound_DoCombat.go:326-331` |
| A mob fold that completes with every target gone resolves nothing and says nothing | `hooks/spell_resolution.go:646-655` |
| `sendMobSpellFailed(mob, room, "fizzles")` sends sight then `SoundSpellSputtersOut` | `NewRound_DoCombat_helpers.go:515-519`, `:847-849` |
| `actTryScan` promotes the first hostile sighting; every player is hostile | `behaviortree/actions_scout.go:23-77`, `:263-269` |
| Scan sight tests cover mob sightings and a hidden player; none drives `actTryScan`, none has a visible player in a dark next room | `actions/scan_mob_sight_test.go:48`, `:71`, `:89`; grep `actTryScan` in tests: 0 |
| Scout tree: the in-room hidden search (branch 4) runs before the scan (branch 5) | `behaviors/archetypes/scout.yaml` |
| Goblin Scout 217: scout tree, spawns hidden (`[9]`), species goblin with Night Vision (29), no heat sight | `mobs/ironwind_steppe/217-goblin_scout.yaml`; `species/5-goblin.yaml` |
| Night Vision 29 shifts the window by 18; blind below 25 (HEAD `config.yaml`), so a goblin is blind under light 7 | `conditions/29-night_vision.yaml`; `config.yaml:952`; `characters/vision.go:17-27` |
| Admin spawn echoes: item, mob, container, gold; admin `online` adds id, zone and room columns | `admin.item.go:128`; `admin.mob.go:205`; `admin.spawn.go:51`, `:77`; `online.go:87-93` |

## Fixes

- **H1 Narration names carry no tags (#453).** `GetCharacterName(true)` returns the identity tag
  alone: no adjective span, quest star or pet. All 29 callers are narration, so one change
  covers them. The three mob-holder sites wrap `mobDisplayName` in `StripNameAdjectives`. `look`
  and the rosters do not use either path and keep their adjectives.
- **H2 A name resolves only at full sight (#454).** Lift `admitCastAim`'s sight switch into
  `actions/sight_aim.go`. The shape helpers are renamed for general use and the hint takes the
  verb. At full sight, names resolve. At shapes, only `shape`, `N.shape` and the caster's own
  foe resolve; a typed name gets the hint. With no sight, nothing resolves. `cast` keeps its
  behaviour on the shared code. It also applies to: `attack <name>`, `StageMeleeTarget`,
  `give`, `shoot` and `fire` (judged by `scanReach` into the target's room), `show`, `steal`,
  `plant`, `consider` and `follow`. Owner call 2 adds `talk`, `ask`, `party invite` and `rep`:
  typing a name there also confirms that the person is present. Player actors only, since mobs
  act on shapes (D8). The give
  lines and the quest NPC's return line hide names at each reader's sight with `HideNames`, as
  `attack.go:192-203` does.
- **H3 Departures judged before the move (#456).** The quit line takes `VisualSnapshot` before
  `RemovePlayer`. It is sent with `SendTextVisualToSnapshot`, and readers who did not perceive
  the quitter are excluded. The four departure lines take a snapshot before the move and use a
  new `SendTextVisualWithAudioToSnapshot`, the sound twin. Arrivals are unchanged (Owner call 1).
- **H4 The sneak hide survives a reload (#451).** `reconcileSneakHide` runs beside
  `reconcileShroudHide`. A Visible holder of a live record 9 re-enters through
  `hideForStealthRecord(9)`, so the rebuild keeps the 9. The saved `sneaking` flag then matches.
- **H5 A mob fold is never dropped silently (#242).** When `IdleMobs` releases a mob whose
  target left, a fold in progress ends first. A new `fizzleMobFold` is shared with the
  TargetGone branch. It clears the casting, records the concentration failure and calls
  `sendMobSpellFailed(..., "fizzles")`, sight then sound. A mob fold that completes with no
  target left says the same line.
- **H6 Scout verified (#251).** No code change unless the tests below fail.
- **H7 Copy (#455, #449).**
  - Map tags: one shared builder for `look` and `map`, slugging the legend name (spaces become
    hyphens), applied rune by rune in one pass. Aliases are added for multi-word biomes, and
    `map.template` uses the same slug.
  - `CategoryMobEmote` keeps only the end-punctuation stage. Player-typed emotes stay exempt.
  - The 8 `claws.yaml` lines are reworded so the verb does not agree with `{itemname}`.
  - Description modifiers are applied before the wrap.
  - A U+00A0 joins the closing `***` of a crit banner to the last word.
  - Prompt and GMCP enemies use `UnseenNoun` of the reader's sight: "a figure" or "something".
  - Admin echoes become "You wave your hands and X appears.", with room lines to match. Admin
    `online` drops the Title column.

## Out of scope

Room prose dashes (#248). Crit disarm stays the accepted known limit (playtest-fixes spec).
Arrival lines, by Owner call 1.

## Owner calls (answered 2026-10-09)

1. **Arrivals: keep as is.** A person entering with a light is seen as they walk in. In the
   owner's words: "Logically, a person entering with a light would be seen as they walk in."
2. **Names typed in the dark: gated.** `talk <npc>`, `ask <npc> about`, `party invite <player>`
   and `rep <player>` (whisper your health report) resolve their target by name among the room's
   occupants. Today that confirms someone is present even when you see nothing. They follow H2.

## Testing and close

Each fix gets a failing test first:
- H1: `hooks` start, trigger and end lines for a lit, hidden and pet-owning holder.
- H2: `actions/cast_sight_test.go` still passes. A new `actions/sight_aim_test.go` covers
  attack, kick, give and steal at each band, including a hint, a `shape` hit and a no-sight
  refusal. Give lines are checked at shapes.
- H3: quit with a lit torch at night: the watcher reads the name. Same for `go` and `RelocateMob`.
- H4: `characters/shroud_hide_test.go` sibling: save while sneak-hidden, reload, still Hidden,
  and no end line.
- H5: `hooks/spell_channel_sight_test.go`: target walks out mid-fold, and a sighted, a shapes
  and a blinded reader each get their line. A fold completing with no target does the same.
- H6: new `behaviortree/actions_scout_sight_test.go` drives `actTryScan`. A visible player in a
  lit next room is promoted; this control proves the test can fail. A visible unlit player in
  a pitch-dark next room fails with no SoftTarget. So does a sneak-hidden player in a lit room.
- H7: `messaging` wrap and normalize tests, a map-tag test for "Deep Water", a roomdetails
  wrap test, prompt and GMCP at shapes, and an 80-column check on the admin echoes.

One PR. The playtest re-runs #453 to #456, #451, #449 and a mob mid-fold walkout. Then the
#251 case. Spawn a Goblin Scout in a lit room with no player in it, beside a pitch-dark room
holding an unlit player (Night Vision cannot lift 0) and a lit room holding a sneak-hidden
player. Watch for five minutes: the scout must not move toward either. Then the dark-room
player lights a torch, and the scout must track them within five minutes. That is the control.
If no leak is found, #242, #251, #449, #451 and #453 to #456 close, and #382 gets its closing
comment.
