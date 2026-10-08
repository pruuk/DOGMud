# Sight gates close-out (#382): the last names that leak in the dark

**Status:** APPROVED by the owner, 2026-10-08. The R3 extension to player emotes stands (owner approved the spec with it flagged).
**Epic:** #382. **Parents:** `specs/completed/2026-09-29-sight-and-gates-parity-design.md`
(slices 5a/5b) and `specs/completed/2026-09-22-graded-room-lighting-design.md`. Lighting arc
#372 closed 2026-10-07; this slice clears what is left under #382 and ends with the adversarial
dark-room playtest that closes it.

**Issues in scope:** #214, #242, #246, #216, #254 (A) · #333, #215, #274, #251 (B) ·
#276 remainder, #435 (C) · #252, #272, #218 (D) · #298, #260, #219, #409 (E).
**Out of scope:** #228 (its code exists only on PR #208), #315 (owner: inside the quest redo,
#375), `help glow` (moves to #234), room-prose em dashes (#248), the repo-wide not-found
wording sweep (follow-up issue).

## Facts verified against source (master `f50400bf7`, 2026-10-08)

| Fact | Where |
|---|---|
| `attack` builds `mName` with `GetMobNameIndexed(user.UserId, dupIdx)`, no sight check | `internal/usercommands/attack.go:172` |
| Raw name in "mortal combat with %s" (mob) and "%s is someone's companion!" | `attack.go:252`, `attack.go:176` |
| Raw player name in "mortal combat with %s" (player target) | `attack.go:339` |
| "You attack the darkness!" covers empty arg and unmatched name; two other sites say "You don't see them here." | `attack.go:101`, `:166`, `:289` |
| Pattern to copy: `HideNames` wrapping the Sprintf in place (a registry test keys on the literal) | `internal/usercommands/target.go:205`, `:216`, `:223` |
| Mob concentration breaks / fizzles / falters go out on plain `mobRoom.SendText` | `internal/hooks/NewRound_DoCombat_helpers.go:730`, `:736`, `:741`, `:749` |
| Mob "weaves magic" and "shifts focus to %s" on plain `SendText` | `NewRound_DoCombat_helpers.go:830`, `:1220` |
| Player concentration break on hit uses plain `defRoom.SendText` with the raw name | `NewRound_DoCombat_helpers.go:1079` |
| Player prone/grapple breaks already use `sendVisualRoomText` | `NewRound_DoCombat_helpers.go:511`, `:521` |
| `sendVisualRoomText`, `sendVisualElseAudible(room, cat, visualMsg, soundMsg)` exist | `NewRound_DoCombat_helpers.go:402`, `:444` |
| "You hear the sounds of fighting nearby." sent in the fight's own room | `NewRound_DoCombat_helpers.go:425-435` |
| `nameTagPattern` covers username, mobname, petname; `Anonymize` replaces the whole tag body | `internal/messaging/anonymize.go:21`, `:32` |
| `HideNames`, `HideSpeakerNames`, `UnseenFigure` | `internal/messaging/hidenames.go:55`, `:94`, `:39` |
| `ParticipantSight`, `SeesThroughExit`, `CanSeeShapes` | `internal/messaging/predicates.go:56`, `:96`, `:208` |
| `SendHeard`, `SendSeen`, `sendSpoken(..., stillHidden)` | `internal/actions/room_lines.go:19`, `:32`, `:51` |
| Sneak observer loops roll `CalcDetectionScore` for every non-ally, no sight check | `internal/actions/sneak.go:131-177` |
| Observer notice line names the actor raw; `SpottedByName` is the raw observer name | `sneak.go:150`, `:155`, `:176` |
| Sneaker reads `SpottedByName` | `internal/usercommands/skill.skullduggery.sneak.go:63-71` |
| `CalcDetectionScore = (Perception + Search*SkillWeight) * SightMult` | `internal/actions/skill_helpers.go:87` |
| `SightMult` floors at `DarknessCombatPenalty`, shipped 0.80 | `internal/messaging/sight_mult.go:32`; `config.yaml` (HEAD blob) line 933 |
| `SuperHearing` condition flag exists; only `world/default` grants it today (no DOGMud content) | `internal/conditions/conditionspec.go:66`; `_datafiles/world/default/conditions/28-superior_hearing.yaml` |
| Mob emotes go through `SendSeen` with no hidden check | `internal/mobcommands/emote.go:25`, `:36` |
| Scan structured loop: mobs skip hidden, players do not; no sight check for mob callers | `internal/actions/scan.go:83-107` |
| Player scan text decides reach with `SeesThroughExit` / `SensesHeatThroughExit`, lists via `listedOccupants` | `scan.go:143-176`, `scan.go:215` |
| `mobCanSee` = SightFull or SightShapes (ruling D8) | `internal/behaviortree/sight.go:36` |
| Scout promotes first sighting to `SoftTarget` | `internal/behaviortree/actions_scout.go:44-66` |
| Corpse dust lines on plain `r.SendText` with the raw name | `internal/rooms/rooms.go:205`, `:208` |
| `FindCorpse(searchName)` / `FindCorpseIndex(searchName)` take no viewer; 8 callers | `rooms.go:1502`, `:1562`; callers listed in C |
| Parser scope carries the user | `internal/parser/parser.go:35` (`Scope.User`) |
| Room light override field `lamp:` | e.g. `_datafiles/world/dogmud/rooms/greenford/6280.yaml` (#207) |
| `corpseNameFor(user, room, corpse)` is the sight-aware corpse name | `internal/usercommands/loot.go:63` |
| `SendTextHidingNames(cat, txt, names, hide, ...)` | `rooms.go:301` |
| GMCP `Room.Info` copies `room.Description` with no sight check | `modules/gmcp/gmcp.Room.go:358` |
| GMCP say fans out to every player in the room, no Deafened or sight check | `modules/gmcp/gmcp.Comm.go:66-84` |
| Fog map has its own per-character set, written at exactly one site | `internal/characters/character.go:675` (`MarkRoomVisited`); `internal/usercommands/go.go:289` |
| `ShopSightRefusal` / `ShopSightRefusalText` used by buy, sell, list only | `internal/actions/shop_sight.go:20`, `:28` |
| `appraise` names the merchant raw | `internal/usercommands/appraise.go:86`, `:109` |
| `look` refuses at `SightNone` before own-gear lookup | `internal/usercommands/look.go:51`, `:292`; `internal/actions/look.go:57` |
| Jail cell has `biome: dungeon`, no light; arrest line names no command; `fine` names `payfine` | `rooms/instance_jail_cell/5107.yaml:4`; `internal/justice/arrest.go:419`; `internal/usercommands/jail.go:47` |
| `equip` wearable line has no light mention; `eq` is an `inventory` alias | `internal/usercommands/equip.go:162`; `_datafiles/world/dogmud/keywords.yaml:171` |
| `say` has no empty guard | `internal/usercommands/say.go:16` |
| Player-facing em dashes | `stand.go:37`, `eat.go:22`, `NewRound_DoCombat_helpers.go:518`, `:527`, `:530`, `:534` |
| "item(s)"; bare `get all` on an empty floor says nothing | `internal/usercommands/get.go:84`, `:133`, `:229-250` |
| "faces are clear again" / "make out faces again" | `narration/light-notices/carried.yaml:21-22` |

## Owner rulings (2026-10-08)

- **R1 (#333):** an observer at `SightNone` gets a hearing roll only.
- **R2:** `SneakHearingMult` starts at **0.75**.
- **R3 (#274):** a hidden actor does not emote. Silence, not an anonymous line.
- **R4 (#242):** disruptions (breaks, fizzles, falters) have a sound line; the weave and
  focus-shift lines are sight-only.
- **R5 (#252):** in scope; the map keeps rooms already seen and adds none while the player sees
  nothing.
- **R6 (#254, 2026-08-15):** an unmatched name reads "Nothing by that name is in this room."
- Defaults the owner did not object to: #216 reads "You hear fighting close by."; the sneaker's
  view of whoever spotted them follows the sneaker's own sight (name, "a figure", "something").

## The rule

Every line that names an actor goes through an existing sight-aware path: `SendSeen`,
`SendTextVisual` / `sendVisualRoomText`, `HideNames`, `SendTextHidingNames`, or
`corpseNameFor`. No new name machinery.

## A. Combat and spell

- **#214:** wrap `attack.go:176`, `:252` and the player-target `:339` in `messaging.HideNames`
  at `ParticipantSight(user.Character, room)`, in place, as `target.go:205` does.
- **#242:** `:830` (weaves) and `:1220` (shifts focus) go through `sendVisualRoomText`.
  `:730`, `:736`, `:741`, `:749` go through `sendVisualElseAudible` with sound lines:
  "Someone's chant breaks off." (breaks), "A half-formed spell sputters out." (fizzles,
  falters). The player break at `:1079` moves to `sendVisualElseAudible` with the same sound
  line, so mob and player read alike.
- **#246:** `Anonymize` keeps a trailing `'s` found inside a name tag and re-emits it after
  the figure word. Covers every combat template; no YAML edits.
- **#216:** the `:435` line becomes "You hear fighting close by." Goldens re-recorded.
- **#254:** empty `attack` reads "There is nothing here to attack."; an unmatched name reads
  the R6 line, and `:166`, `:289` converge on it. The typed name is never echoed.

## B. Stealth and mob side

- **#333:** in both observer loops, an observer whose `ParticipantSight` of the room is
  `SightNone` scores `(Perception + Search*SkillWeight) * SneakHearingMult` instead of
  `CalcDetectionScore` (its `SightMult` is meaningless without sight). An observer with the
  `SuperHearing` flag skips the multiplier. New balance knob `SneakHearingMult`
  (`ConfigFloat`, default 0.75) declared in `config.balance.go` and set in `config.yaml`.
  A hearing notice tells the observer "You hear someone trying to move quietly." and never
  names the sneaker.
- **#215:** a sighted observer's notice line goes through `HideNames` at that observer's
  sight. `SneakResult` carries the spotter's identity instead of a bare name, and the sneaker's
  line is rendered at the sneaker's own sight of the spotter: the name, "a figure", or
  "Something notices you." at `SightNone`.
- **#274:** `SendSeen` returns without sending to the room when the actor is hidden. Its
  callers are mob `emote` (`mobcommands/emote.go:25`, `:36`) and player `emote`
  (`usercommands/emote.go:26`, `:38`, `:61`). Today a hidden player's emote names them to the
  whole room. Under R3 it reaches no one; the player still reads their own echo, followed by
  "No one sees it; you are hidden." so the silence is not a mystery. **Owner check:** R3 was
  asked about mobs; this applies it to players too. `SendHeard` is unchanged (a sound is
  heard whoever makes it); speech keeps its own `sendSpoken` path.
- **#251:** extract the player scan's reach decision (`scan.go:143-155`) into one helper,
  `scanReach(viewer, room, adjRoom) SightDecision`. The structured loop fills each sighting
  from `listedOccupants(viewer, adjRoom, self)` when the reach is not `SightNone`, so hidden
  players drop out and darkness blocks both directions. Under D8 a mob acts on shapes. The
  player text keeps rendering names or figures from the same helper. Scout behaviour changes;
  the playtest covers it.

## C. Corpses

- **#276:** the dust lines at `rooms.go:205`, `:208` go out per listener with the corpse name
  hidden by sight (`SendTextHidingNames`). `nameTagPattern` gains `user-corpse` and
  `mob-corpse`.
- **#435:** `FindCorpse` and `FindCorpseIndex` take the viewer; below `SightFull` only the
  bare word "corpse" matches a player corpse. The eight callers pass it: `look.go:444`,
  `get.go:183`, `get.go:749`, `assess.go:27`, `salvage.go:40`, `loot.go:47`, `loot.go:161`,
  and the parser's `corpseAdapter` (`internal/parser/adapters.go:44`, viewer from
  `Scope.User`). `loot pass` names the next member through `HideNames`.

## D. Other surfaces

- **#252 Room.Info:** at `SightNone` the payload blanks description, exits and both rosters;
  below `SightFull` roster names become the shapes figure. `Room.Info.Contents` follows.
- **#252 Comm say:** skip `Deafened` recipients; the `Name` field is rendered per recipient at
  their sight of the speaker, the same tiers the text lane uses.
- **#252 map:** `go.go:289` marks the room visited only when the mover's sight there is not
  `SightNone`. Already-seen rooms stay. Note for the deploy: rooms walked in the dark before
  this ships stay on old saves' maps.
- **#272:** `appraise` and `offer` call `ShopSightRefusal` first and print
  `ShopSightRefusalText`, as `sell` does.
- **#218:** `ResolveLook` gains an own-gear-by-touch result at `SightNone`: when `lookAt`
  names an item the looker wears or carries, the answer is "You run your hands over your
  <item>." with the item's name and no description (items carry no touch text, and the
  description is what sight reads). Rooms, exits and creatures keep the refusal.

## E. Content and copy (PR 2)

- **#298:** cell 5107 gets a `lamp:` override (the room field #207 used on 27 rooms, e.g.
  `rooms/greenford/6280.yaml`) at a dim value, chosen in the plan from the band table so a
  normal-sighted prisoner makes out shapes and can `look`; the arrest line at
  `arrest.go:419` adds "Type fine to see what you owe."; condition 88's `start_actee` names
  `fine` the same way.
- **#260:** equipping a light source adds a second line ("It casts light around you.");
  empty `say` refuses ("Say what?"); `equipment` joins the `inventory` aliases.
- **#219:** the six em dashes above become commas, colons or full stops.
- **#409:** empty `get all` replies "There is nothing here to pick up."; "item(s)" becomes
  singular or plural by count; the two `lighter_faces` variants drop "again".

## Delivery

- **PR 1 (A to D):** one branch, one commit per issue, full local gate.
- **PR 2 (E):** independent, may run in parallel.
- **Playtest:** after both merge, an adversarial dark-room playtest (pitch dark, shapes,
  blind, deafened, a sneaker, a scout mob, a corpse, a shop, GMCP on). #382 closes when it
  finds no name leak.

## Testing

Every change gets a test shown able to fail before the fix. Wording changes re-record their
goldens (#216, #242). `SneakHearingMult` gets a test that reads the shipped `config.yaml`
value, since test binaries load Go defaults. #251 gets a scan test for a hidden player and for
both dark directions, plus the playtest. GMCP gets payload tests at each sight tier and for a
deafened recipient.

## Dry-run amendments (2026-10-08)

The plan was dry-run on `f50400bf7` before it was written. These findings override the
sections above where they differ; the plan carries them.

- **A, #214:** two more raw-name lines join the fix: the non-combatant refusal
  (`attack.go:179`) and the PvP target's own "prepares to fight you!" (`attack.go:345`), the
  latter judged at the target's sight.
- **A, #242:** the player prone and grapple breaks (`helpers.go:511`, `:521`) also move to the
  sound-backed sender, so mob and player casters read alike. `sendVisualElseAudible` gains an
  exclude list. The focus-shift line names two people and uses `SendTextVisualHidingNames`.
- **B, #333 and #215:** the same contest runs a second time for a sneaking arrival
  (`sneakerSpotted`, `internal/actions/move.go:293-330`, "slips into the room but you notice
  them."). Both share one score helper and one notice helper. The sneaker's line at
  `SightNone` keeps its opening: "You try to blend into the shadows but something notices
  you."
- **B, new:** a mob's `sayto` line goes to the room on plain `SendText`
  (`mobcommands/sayto.go:50`, `:68`) and names both parties in the dark. It is fixed in the
  same PR.
- **C, #276:** `nameTagPattern` does NOT learn the corpse tags. Corpse lines carry "corpse of
  <name>" inside the tag and `Anonymize` runs before `HideNames`, so the change would turn
  "loots the corpse of Deadric" into "loots the A figure" and break #428. The decay line goes
  out through `SendTextVisualHidingNames` instead; its wording becomes "The corpse of Deadric
  crumbles to dust." for every reader, matching every other corpse line, and a reader who sees
  nothing gets no line.
- **C, #435:** the bare-word rule covers mob corpses too; `NameAt` already hides both kinds
  below clear sight. `FindCorpse` becomes a wrapper over `FindCorpseIndex`.
- **D, #252:** at `SightNone` Room.Info also drops the title, items and containers, as the
  text `look` refuses them. A light-band change in the room re-sends Room.Info and the map, so
  lighting a torch updates the panel and maps the room. A hidden mob's `sayto` also sent its
  real name over GMCP; the say rework covers it (new `Communication.SpeakerHidden`).
- **D, #218:** implemented as a new `LookOwnGearByTouch` result in `actions.ResolveLook`; a mob
  looker gets nothing from it.
- **E, #298:** the lamp is `lamp: 35` (shapes band 25 to 49 under shipped `LightBlindBelow: 25`,
  `LightDimBelow: 50`), and it goes on the two static fallback cells 5105 and 5106 as well as
  5107. The `fine` hint goes on the arrest line and the Jailed condition's `description`, not
  `start_actee`, which prints back to back with the arrest line.
- **E, #219 and #409:** a seventh dash (`jail.go:42`) and the mirror "item(s)" in `drop all`
  (`drop.go:127`) are fixed too.
