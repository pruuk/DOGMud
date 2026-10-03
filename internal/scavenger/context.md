# Scavenger Package Context

## Overview

The city scavengers, the loot goblin's replacement (2026-09-30). One NPC per
city, and one per New Plymouth district, walks to random rooms of its city
and picks up the litter it finds. Its haul sits in its inventory, where a
pickpocket can take it back (`actions.Steal` draws a random carried item and
most of its gold), and is cleared at each real-world day boundary. The rest
of the world is kept tidy by the daily floor decay in
`internal/rooms/floor_decay.go`, which skips every room a scavenger keeps.

This package holds the authored profiles, the resolved room pools and the
pure pieces of the walk. The behavior-tree action that drives a scavenger
each idle tick is `scavenger_step`
(`internal/behaviortree/actions_scavenger.go`), used by the archetype
`behaviors/archetypes/scavenger.yaml`.

## Files

- **scavenger.go**: `Profile`, the loader (`LoadDataFiles`, `Parse`,
  `resolvePool`), the registry (`ProfileFor`, `All`, `IsPatrolledRoom`,
  `AnchorRooms`) and test hooks (`SetProfilesForTest`,
  `Profile.SetPoolForTest`).
- **walk.go**: `Walk` (the in-progress walk kept in the mob's TempData),
  `StepDelay`, `PickTarget`, `Walk.NoteArrival`, `Line`, and the constants
  `MaxFailedSteps` (2) and `RetryAfterNoPath` (10 s).

## Data

`_datafiles/world/dogmud/scavengers.yaml` (authored, committed), a list under
`scavengers:`. Each profile names its mob, the zones and biomes its pool is
drawn from (`DefaultBiomes`: `city_thoroughfare`, `city_backstreet`,
`interior`), or an explicit `include_rooms` list, rooms to exclude, its home
room, and three sets of lines in its own voice (`{actor}`, `{item}`,
`{gold}` placeholders, each checked at load).

Pools are resolved at boot by `LoadDataFiles(World)`. `World` injects the
rooms and mapper lookups (`main.go` wires them), so `Parse` is testable
without a world on disk. Unknown mobs or zones, a home room outside its own
pool, a duplicate mob, a missing placeholder, an unknown YAML key, or a pool
under two rooms fail the boot like any broken content. Rooms with no path
from the home room are dropped with a Warn naming them. Rooms `World.Private`
reports (main.go wires `housing.IsUnitRoom`) are dropped from every pool, and
a private home room fails the load: no scavenger ever enters a player home.

`LoadDataFiles` installs the set atomically; main.go then registers
`IsPatrolledRoom` with `rooms.SetFloorDecayExempt` and prepares
`AnchorRooms()` so every scavenger spawns at boot.

## The mob side

A scavenger's mob YAML carries `behavior_archetype: scavenger`, the group
`scavenger` (which makes it essential in `mobs.IsEssential`, so its room is
never unloaded and its walk and haul survive), `maxwander: -1`, and is NOT
non-combatant or attack-immune: `mobs.CheckPlayerHarm` would refuse the
pickpocket otherwise. Its home room's spawninfo spawns it.

Scavenger mob ids: 9820 to 9834 (9801 to 9804 are the housing landlords).

## Invariants

- Scavengers and the floor decay share one definition of litter,
  `rooms.Room.FloorItemIsLitter`. Never give a scavenger its own rule: an
  item the decay spares (a room's own prop, a quest item, an untaken
  bauble) must be one a scavenger spares too.
- Every pickup goes through `actions.TakeFloorItem`, the player's gates.
- `scavenger_step` owns every idle tick of a scavenger (always Success), so
  the legacy wander, the displaced-home pull and the goal planner never
  fight its walk.
- Pools are the floor-decay exemption, with every housing unit room. A
  room added to a pool stops decaying; one removed starts. A player home is
  never in a pool and never decays.
