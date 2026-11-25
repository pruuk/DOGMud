# buttons: Package Documentation

## Overview

The server side of the touch controls' buttons (#781, touch arc spec 2; the
first slice of the button protocol in `docs/roadmaps/TOUCH_ARC_ROADMAP.md`).
The client never builds a command: the server gives each button an opaque
id, the client sends back only the id, and the server turns it into the line
the player would have typed. Plan 3a widens this to every button.

## Movement ids

- `FixedIds()`: the six T directions' ids (`m1` north, `m2` south, `m3`
  east, `m4` west, `m5` up, `m6` down). The same in every room and for every
  player (owner ruling 8); the only ids the dark offers. `Room.Info` sends
  them as `moves`.
- `MovementIds(userId, room, rooms.KnownExits(...))`: each known exit's id
  by name; a T direction gets its fixed id, any other exit a per-room id
  (`x` and 8 random hex digits from `util.RandomHex`) issued to that player
  for that room. A resend in the same room keeps the ids of exits still
  known; each call replaces the player's whole set, so an exit no longer
  known loses its id. The same exit names in the same room keep the held
  set as it is and return a copy of it (`Room.Info`'s round watcher calls
  this every round). A nil room issues nothing and leaves the ids held
  as they were.
- `Resolve(userId, roomId, id)`: a fixed id is its bare direction
  (`north`) in any room; a per-room id is `go <exit>` only in the room it
  was issued for. `ok` false is a stale press (ruling 7); the caller sends
  `refusal.StaleButton` and re-sends `Room.Info`. The room check matters for
  a press that lands after a move but before the next `Room.Info`: the ids
  held are still the old room's.
- `Forget(userId)`: drop everything (despawn).

## Dark marks

`RecordDarkMark(userId, roomId, direction, MarkWall|MarkLocked)` notes a T
direction tried in the dark (ruling 4) and reports whether anything changed;
`usercommands.Go` calls it, so typed and tapped tries both mark.
`DarkMarks(userId, roomId, dark)` returns them and clears them once the
player is elsewhere or the room is not dark to them; `ClearDarkMarks(userId)`
on a room change. `Room.Info` sends them as `darkmarks`.

## State

In memory, guarded by one mutex, per session: not living state
(`dogmud-persistence`). A restart or a logout forgets ids and marks; the
next `Room.Info` issues fresh ones.

## Callers

`modules/gmcp` (`gmcp.Room.go` builds the ids and marks into `Room.Info`,
`gmcp.ButtonPress.go` resolves presses, `gmcp.RoomExitsWatch.go` forgets on
despawn and clears marks on a room change); `internal/usercommands/go.go`
(dark marks). These callers arrive in plan 2a Tasks 9 to 12.
