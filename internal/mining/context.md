# internal/mining

The data and per-room vein state behind mining (wilderness trades; design
and numbers in `docs/economy/mining.md`). It mirrors `internal/timber`.

## Files

- **mining.go**: `Ore` (id, name, item a load gives, tier 1..4, `MinPick`
  1..4, note), `Gem` (item, weight, `MinPick`), `Weighted`, `Data`, `World`
  (item, zone and room existence checks), `Parse` (strict YAML; refuses
  unknown ores in pools, bad tiers and pick tiers, a zone both excluded and
  pooled), `LoadDataFiles` (boot; a missing file means nothing is
  mineable), `Install`, `GetOre`, `AllOres`, `Gems`, `Pool`,
  `IsMineableBiome`, `Zones`, `PickGem`. `DataFileName` is `mining.yaml`.
- **vein.go**: `Vein` (ore, loads left, max, last change, worked out),
  `LoadVein` and `SaveVein` (room long-term data keys `mining.*`, plain
  values that survive the instance save), `Refill`, `RoundsToNextLoad`,
  `Dig`, `PickOre` (weighted, with a neighbour bonus so seams run together),
  `NewVein`. `Store` is satisfied by `*rooms.Room`.

## Rules

- `Pool(roomId, zone, biome)`: a room pool wins; otherwise an excluded zone
  has nothing, a biome without a pool has nothing, and a zone pool replaces
  the biome pool.
- The package knows nothing of items, characters or rolls. The job, the
  tool and the messages are in `internal/actions/mine.go` and
  `internal/usercommands/mine.go`; the roll is `gather.JobMine`.

## Tests

`mining_test.go` pins pool precedence, refusal of bad data, the gem table's
pick gate and a vein's dig and refill cycle. The repo-root
`TestMiningContent` (wilderness_trades_content_test.go) pins the shipped
file against items and recipes.
