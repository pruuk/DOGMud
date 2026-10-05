# Timber Package Context

## Overview

`internal/timber` is the data and the per-room stand state behind
lumberjacking (wilderness trades, phase 4; `docs/economy/wilderness-trades.md`).
The commands live elsewhere: `actions/chop.go` (`RoomStand`, `SurveyTrees`,
`ResolveChop`) and `usercommands/chop.go` (`Chop`, `Survey`).

## Files

- **timber.go**: `Species` (id, name, log item, optional bark item, tier 1..4,
  survey note), `Weighted`, `Data`, `World`, `Parse` (strict YAML; checks
  items, zones, tiers, weights and unknown species), `LoadDataFiles` (called
  from main after items and rooms; a missing file loads nothing), `Install`,
  `GetSpecies`, `AllSpecies`, `IsChoppable`, `Pool`, `Zones`.
- **stand.go**: `Stand` (species, stock, max, updated round, felled flag),
  `Store` (satisfied by `rooms.Room`), `LoadStand`, `SaveStand`,
  `Stand.Regrow`, `Stand.RoundsToNextTree`, `Stand.Fell`, `PickSpecies`,
  `NewStand`.

## Rules

- Data file: `_datafiles/world/dogmud/timber.yaml`. A biome pool makes that
  biome choppable; a zone pool replaces the biome pool in that zone but never
  makes a non-timber biome choppable.
- A stand lives in the room's long-term data under `timber.*` keys as plain
  strings and ints, so it survives the room's instance save. Wiping
  `rooms.instances/` resets stands, which is harmless.
- Regrowth: one tree per `Balance.TimberRegrowRounds` since `Updated`, up to
  `Max`. A stand felled to zero is re-rolled (`PickSpecies`, leaning a quarter
  of the pool weight toward each neighbouring stand's species) when it grows
  back.

## Tests

`timber_test.go` covers parsing and validation, pools, the stand round trip,
felling, regrowth and the re-roll signal, and the neighbour lean.

## Axe tiers (wilderness trades review)

`Species.MinAxe` is the poorest axe tier (as an int, `items.ToolTier` values)
that can fell a species: crude for tiers 1 and 2, iron for tier 3 (yew,
walnut), steel for tier 4 (ironwood).

## Wood traits (wood.go)

`Species.Bow` (`BowTraits`: speed, weight, accuracy multipliers) and
`Species.Arrow` (`ArrowTraits`: damage and accuracy multipliers, recovery
chance), validated by `Parse` (multipliers 0.5..1.5 or omitted, recovery
0..0.9). `BowWood`, `ArrowWood`, `WoodName`, `SpeciesForLog`. Read by
`items.GetSpec` (bow speed and weight) and by `actions.ExecuteFire` and
`chamberNextRound` (accuracy, arrow damage, recovery).
