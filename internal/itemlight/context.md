# internal/itemlight

The light and darkness that **fixtures** give their rooms (lighting 5e, item
behaviour slice 1, spec X6 and Rule 10). A leaf: it imports only `uuid`.

## What it is for

A fixture is an item fixed to a room's floor (`ItemSpec.Fixture`, `light` or
`darkness`): the North Gate's arch lantern (item 55, room 4111), the Rift
Stone (item 56, room 5000). Its behaviour tree writes its output here through
`set_light` and `pulse_light` (`internal/behaviortree/actions_item_light.go`),
and `internal/rooms` reads every lit fixture of a room as one term each when it
composes the room's light (`composeLightExcluding` in `lighting.go`).

It exists because `behaviortree` imports `rooms`, so `rooms` cannot read tree
state. This package sits below both. A worn light never comes here: its light
lives on its holder's condition record.

## Surface

| Symbol | Purpose |
|---|---|
| `type Kind`, `Light`, `Darkness` | Which combine an output joins |
| `Set(roomId int, id uuid.UUID, kind Kind, value float64) bool` | Record a fixture's output; reports a change. Non-finite or negative records it unlit |
| `Get(roomId int, id uuid.UUID) (float64, bool)` | A fixture's output (`-Inf` when unlit) and whether one is recorded |
| `Lit(roomId int, id uuid.UUID) bool` | Recorded and giving light (the room look's "is lit") |
| `Clear(roomId int, id uuid.UUID)` | Drop one output: the item left the floor (`Room.RemoveItem`) |
| `ClearRoom(roomId int)` | Drop a room's outputs: it left memory (`removeRoomFromMemory`, the item tick) |
| `Retain(roomId int, keep map[uuid.UUID]bool)` | Drop outputs whose item is no longer on the floor (the item tick, every round) |
| `Terms(roomId int) (light, dark []float64)` | A room's lit outputs, UUID order, for the two combines |
| `ResetForTest() func()` | Empty every room; returns a restore |

## Traps

- **In memory only, keyed by item UUID.** A UUID is minted on every load, so a
  restart or a room reload starts empty. The room's first visit evaluates its
  fixtures at once (`items.OnRoomHolderIndexed`, set by `internal/hooks`), so
  nobody reads a fixture room dark for a round.
- **Unlit is recorded, not absent.** `Set` with `-Inf` keeps the fixture known
  so the look can say "unlit"; `Terms` skips it.
- **Fixtures never trim** and are never reset by a bearer (owner ruling R3).
  A carried adjustable light trims against them like any other term.

## Who uses it

`internal/behaviortree` writes; `internal/rooms` reads and clears;
`internal/hooks` (the item tick) retains and clears; `internal/usercommands`
(`look`) reads `Lit`.
