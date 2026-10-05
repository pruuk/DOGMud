# roomlife Context

## Purpose

The engine side of generated ambient events. When a room is due to show one
of its set ambient lines (the room's `IdleMessages`, else its zone's, from
`hooks.UserRoundTick`), `Modules.roomlife.Chance` percent of the time (10) a
model writes a fresh event instead, seen or heard, from the room's text, the
time of day, the NPCs and things there and the place's own set lines (for
voice). The model side is `modules/roomlife`, which installs the
`Generator`; with none installed every ambient line is the set one.

Only ever on a player's own key, for an awake player in the room who left
"Make the world livelier" ticked (`apiframework.PurposeRoomLife`).

## Files

- **roomlife.go**: `Generator` (`Chance`, `Reserve`, `Generate`),
  `SetGenerator`, `Result` (`Kind`, `Text`, `KeyholderOnly`),
  `MaxGenerateTime`, `TryReplace`, `Pending`; the delivery (`startDelivery`,
  `run`, `finish`), `awake`, `seesClearly`, and the test seams `roll`,
  `shuffle`, `canSee`.
- **request.go**: `Request`, `Snapshot`, `KindSeen`, `KindHeard`,
  `ReplySchemaName` (`room_event`), `ReplySchema`, `ParseReply`,
  `CleanResult`, `ErrUnusable`, `MinTextRunes`, `MaxTextRunes`.
- **place.go**: `Place` (`Chance`, `Setting`, `Timeless`), `PlaceHook`,
  `SetPlaceHook`. A subsystem that owns rooms somewhere other than the usual
  world (`modules/rifts`) says so: its `Chance` replaces the generator's for
  that room (only while the generator's own is above zero, so turning
  generation off still turns it off), `Setting` goes into the request
  (`Request.Setting`) and `Timeless` drops `TimeOfDay`.

## Light: the existing rails

- **Who is asked.** Only an awake player (no `conditions.Sleeping`). Whether
  they see the room clearly (`messaging.CanSeeClearly`: light, blindness)
  goes into the request as `CanSee`; when false, only a heard event is
  accepted (`CleanResult` refuses a seen one).
- **Who reads it.** `finish` uses the room's own senders, the ones every
  ambient line uses: a seen event and the set-line fallback through
  `Room.SendTextVisual` (sight-gated per reader: nothing in the dark, a
  shapes render at shapes); a heard event through `Room.SendText`, the audio
  channel, so a "You hear..." line carries in the dark as other sound lines
  do. A `KeyholderOnly` event goes the same way with every other player
  excluded, and only while the keyholder is there.
- Tests prove both channels against real room lighting and fail when either
  is swapped (`TestInTheDarkOnlyASoundAndItIsHeard`,
  `TestASeenEventIsJudgedPerReader`,
  `TestInTheDarkASeenEventIsRefusedAndNothingIsSeen`).

## Gotchas

- **No player data.** Players are counted, never named; floor items by
  `items.Item.ModelName`.
- **Output is untrusted.** `CleanResult`: printable ASCII without markup,
  12 to 300 characters; a heard event must begin "You hear" and a seen one
  never says "you" (the event happens around the players, never to them).
  Dashes become commas.
- One event per room at a time. Any failure shows the set line; a room left
  empty shows nothing.
- `ReplySchemaName` must be in `LIVELY_SCHEMAS` in relay.js
  (`modules/roomlife` `TestTheRelayKnowsTheSchema`).

## Dependencies

`internal/baubles` (`PlainText`), `internal/conditions`, `internal/gametime`,
`internal/messaging`, `internal/mobs`, `internal/mudlog`, `internal/rooms`,
`internal/users`, `internal/util`. Imported by `internal/hooks` and
`modules/roomlife`.
