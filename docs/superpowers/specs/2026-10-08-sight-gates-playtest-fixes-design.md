# Sight gates close-out: playtest fixes (#382)

**Status:** APPROVED by the owner, 2026-10-08.
**Parent:** `specs/2026-10-08-sight-gates-closeout-design.md` (approved; shipped in #441 and
#443, master `f508db20b`). Its closing playtest (task P2, run `6b3b017287df1ded`) found the
defects below. #382 closes when a re-run of the failed and unrun cases finds no leak.

## Owner rulings (2026-10-08, after the playtest)

- **R7:** the server-wide "*** X has DIED! (killed by Y) ***" broadcast stays global and
  unchanged. It is an event feed, not something seen in the room.
- **R8:** combat lines show the weapon's name only at full sight. At shapes and at no sight it
  reads generically ("their weapon").

## Facts verified against source (master `f508db20b`)

| Fact | Where |
|---|---|
| Only `RemoveCondition` flips Perception Blinded to Sighted | `internal/characters/conditions.go:204-208` |
| Natural expiry prunes the record and calls `Validate`, never touching Perception | `internal/hooks/NewTurn_PruneConditions.go:39`, `:66`; mob branch `:96` |
| Other removals skip the flip too: respawn prune, `CancelConditionsWithFlag` | `internal/hooks/Life_Cascades.go:117`; `characters/conditions.go:33-35` |
| `Perception` is `yaml:"-"`; `Validate` builds a fresh Sighted machine when nil, so a relog cures blindness even while the condition is live | `character.go:244`; `validate.go:634-639` |
| `HasAnyBlindSource()` | `internal/characters/sight.go:24` |
| Weapon name rendered once for every reader: `ws.weaponName = weapon.DisplayName()` into `TokenItemName` | `internal/combat/combat_helpers.go:404`, `:1670`; `combat.go:271`, `:315` |
| Parry "sweep aside the fumbled {attack}" filled from `GetSpec().Name` | `defense-messages/parry.yaml:106`; `combat_helpers.go:1361-1369` |
| Per-reader hiding in combat: participants `hideIdentitiesInPersonalLines`, spectators `drainSpectatorLines`; `nameTagPattern` matches username, mobname, petname only | `combat.go:743-754`; `hooks/combat_verbosity.go:349`; `messaging/anonymize.go:27-29` |
| In DOGMud YAML 1403 of 1454 `{itemname}` and 35 of 114 `{attack}` sit inside `fg="item"` | grep of `_datafiles/world/dogmud` |
| Flee breaks go out on `SendTextVisual`, no sound | `mobcommands/flee.go:36-38`; `usercommands/usercommands.go:415-417` |
| Close-out's heard disruption helpers are unexported in `hooks` | `hooks/NewRound_DoCombat_helpers.go:481-501`, `:523-553` |
| `Room.SendTextVisualWithAudio` exists | `internal/rooms/rooms.go:490` |
| Other visual-only disruptions: boss interrupt, throw interrupt, throttle interrupt | `hooks/spell_effects.go:519-520`; `usercommands/throw.go:383`; `usercommands/throttle.go:99-100` |
| Player fizzle, falter and bleed-out break send no room line; the mob versions do | `NewRound_DoCombat_helpers.go:596-600`, `:625-627`, `:632-634` vs `:839`, `:846` |
| `sneakerSpotted` returns only a bool; the mover gets no line; condition 9 has no `end_actee` | `actions/move.go:293`, `:209-212`; `conditions/9-hidden.yaml` |
| `SpottedLine(sneaker, room, spotter)` words the `sneak` command's line | `actions/sneak.go:66-78` |
| `IsHidden()` reads only the Awareness machine; spawn conditions add record 9 without entering Hidden; the mirror runs Awareness to condition only | `character.go:891-895`; `characters/conditions.go:237-283`; `hooks/Awareness_Cascades.go:44-61` |
| 9 DOGMud mobs spawn with condition 9 | grep `conditionids:.*\b9\b` |
| 15 flavour lines put an article before a `mobname` tag ("A a figure") | grep of `_datafiles/world/dogmud`, e.g. `stillwater/4121.yaml:66` |
| Mob aggro line lacks a full stop | `mobcommands/attack.go:103`, `:153` (player: `usercommands/attack.go:284`, `:379`) |
| "On the Ground" is `CategoryRoomDescription`, deliberately unwrapped; the roster wraps via `RenderRoster` | `messaging/pipeline.go:103-104`; `usercommands/look.go:863`; `actions/search.go:306`; `actions/roster.go:19-25` |
| `time` prints day or night from `gd.Night` (geometric sunset) | `modules/time/time.go:64-72`; `gametime/celestial.go:90`; `LampsLit()` `:128` |
| `Zone.Map` always adds the current room, whatever the sight | `modules/gmcp/gmcp.Zone.go:93-94` |

## Fixes

- **F1 Blindness ends.** `Character.Validate` reconciles Perception with the record: a blind
  source and Sighted goes to Blinded; no source and Blinded goes to Sighted. Every removal path
  and every load already calls `Validate`, so expiry, death, respawn, purge and relog all agree.
  Covers condition 77 too. Closes the relog cure.
- **F2 Weapons generic below full sight (R8).** A tag-aware pass in `messaging`, in the style
  of `Anonymize`, turns `<ansi fg="item">...</ansi>` into "weapon". Applied only on combat paths:
  `hideIdentitiesInPersonalLines` (each side at its own sight) and the spectator drain. Untagged
  `{weapon}` and `{attack}` tokens in combat and move templates get the item tag. An unarmed
  name ("fists") is not an item and stays. Articles are tested ("a weapon", never "an weapon").
- **F3 Every disruption is heard (R4).** The two sound lines move to exported `messaging`
  constants. Flee, boss, throw and throttle interrupts use `SendTextVisualWithAudio`. Player
  fizzle, falter and bleed-out break gain the room lines the mob versions have, sight then sound.
  Flee converges on the shared wording.
- **F4 A sneaker spotted on arrival is told.** `sneakerSpotted` returns the spotter;
  `SpottedLine` takes the opening, and a player mover reads "You slip into the room but
  something notices you." (spotter at the mover's sight, as the `sneak` command does).
- **F5 A mob spawned hidden is hidden.** `AddCondition` of a spec with the Hidden flag enters
  Awareness Hidden (`TransitionToConcealing`, `ResolveConcealment(true)`), guarded against the
  cascade's own re-add.
- **F6 Copy.** Drop the `mobname` tag from the 15 flavour lines. Full stop on the mob aggro
  line. "On the Ground" wraps to 80 through a ground renderer like `RenderRoster`, at both
  sites. `time` reads "dusk" (afternoon) or "dawn" (morning) while `LampsLit()` and not
  `Night`, with a final full stop.
- **F7 The map stays dark.** `Zone.Map` adds the current room only when the player's
  `ParticipantSight` is not `SightNone`.

**Out of scope:** R7's broadcast; admin `zap` naming the admin; the `fine` reply width (#250).

## Testing and close

Each fix gets a failing test first (models: `perception/integration_test.go`,
`combat/darkness_identity_hiding_test.go`, `hooks/spell_channel_sight_test.go`,
`actions/sneak_spotted_line_test.go`, `hooks/hidden_mob_room_lines_test.go`,
`actions/roster_test.go`, `gmcp/gmcp.RoomSight_test.go`). One PR. Then a re-run playtest of
the failed cases plus those not run before: a fizzle, a break on hit, `loot pass`, a hidden
spawned mob's emote, blindness expiring. No leak closes the 18 issues and #382.
