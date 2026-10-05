# Mining

Mining is a wilderness trade built like lumberjacking: go out with a pick,
dig ore, haul it to a forge, then sell it or smelt it and make something.
It uses no skill. A miner's Strength and Vitality and the quality of the
pick decide how it goes, the way felling works.

Code: `internal/mining` (ores, gems, pools, veins), `internal/actions/mine.go`
(`RoomVein`, `Prospect`, `ResolveMine`), `internal/usercommands/mine.go`
(`prospect`, `mine`), `gather.JobMine`, `items.ToolPick`. Data:
`_datafiles/world/dogmud/mining.yaml`. Knobs: Balance `Mining*` in
`config.yaml`.

## How rooms are tagged

Rooms are tagged by **biome**, not by their description. Every room file
carries a `biome:` (or inherits its zone's default), and the mining pools are
keyed on it, the same way timber is:

- `cave` (84 rooms), `mountains` (26) and `cliffs` (98) hold ore by default.
- A **zone pool** replaces the biome pool in that zone, so the Eastern
  Highlands carry gold and the Labyrinth carries tin.
- A **room pool** replaces both. The four drifts of the Pothole Coulee
  basalt-iron mine (5255 to 5258) carry the richest silver and gold, and
  monsters guard them.
- **Rift rooms** have no ore: they are rebuilt every run, so a vein there
  would be fresh each day. `actions.RoomVein` refuses any room with the
  `rift_run` temp key. Rifts have their own ore through forage.
- **Excluded zones** have no ore even in cave rooms. Thornwall City's 26
  cave rooms are drains, cellars and a lair, and the one cave room on the
  Marches Spur Road is a back room.

What is mineable, then: Pothole Coulee (15 cave, 9 mountain, 44 cliff rooms,
plus the mine), the Ironwind Steppe (16 cave, 31 cliff), the Labyrinth of Low
Tunnels (20 cave), the Eastern Highlands (10 mountain, 20 cliff), Cascade
Pass Road (7 mountain, 2 cliff), the Stillwater sea caves (6) and one cliff at
Kilnreach. About 180 rooms.

## The commands

- `prospect` reads the vein: which ore, how much is left, and when a
  worked-out seam will be worth digging again. Coal, copper, tin and iron
  anyone can name. Tier 3 ores need Perception 105 and gold 110 (times the
  sight ramp). This is a stat check with no roll and no skill.
- `mine [ore]` is a timed job on the Salvaging activity (`mine:<room>`) of
  `MiningRoundsBase` (4) plus one round per tier above 1, shortened by the
  pick's speed. It refuses without a pick and when the pick is below the
  ore's `min_pick`.

## The roll

`gather.JobMine` scores avg(Strength, Vitality) times the pick multiplier,
with no skill, against `GatherBaseDifficulty - MiningEase + (tier - 1) *
MiningTierDifficulty`: 95 for copper, 110 for iron, 125 for silver and 140
for gold. The margin sets the grade, and the pick's tier caps it. A success
digs one load from the vein and yields one ore, plus one per 50 Strength
above 100, plus one for a fine job, up to `MiningMaxOre` (3). The pick wears
on every attempt.

A success also finds a gem with chance `MiningGemChance` (0.04) times
Perception/100 times the pick's rare-find multiplier, clamped to 0.5% to
25%. That is about 4% with an iron pick at Perception 100, and about 8%
with a masterwork one. The gem is drawn from the gem table, minus the gems
the pick is too poor to take whole.

## Ores and gems

| Ore | Item | Tier | Pick | Where |
|---|---|---|---|---|
| coal | coal dust 40020 | 1 | any | everywhere |
| copper | copper ore 40260 | 1 | any | everywhere, most in the Low Tunnels and on cliffs |
| tin | tin ore 40261 | 1 | any | cliffs, Low Tunnels, Steppe, Cascade, Stillwater |
| iron | iron ore 40236 | 2 | any | everywhere |
| lake-iron | lake-iron nodule 40059 | 3 | iron | Stillwater sea caves |
| basalt-iron | basalt-iron ore 40069 | 3 | iron | Pothole Coulee and its mine |
| silver | silver ore 40262 | 3 | iron | caves, mountains, the Highlands, the mine drifts |
| gold | gold ore 40263 | 4 | steel | Eastern Highlands, Cascade Pass, the deep drifts |

Gems: polished stone (weight 6), gem dust (4), raw gem (3, iron pick) and
flawless gem (1, steel pick; new 40269).

Veins hold `MiningVeinMin` to `MiningVeinMax` (4 to 8) loads, refill one per
`MiningRegrowRounds` (1800, two game days), and are re-rolled toward their
neighbours' ore when they refill from empty. They are stored in the room's
long-term data under `mining.*`.

## Picks

| Pick | Tier | Source |
|---|---|---|
| Rough Pick 10073 | crude | Mine Foreman Dagna (Pothole Coulee Mine Mouth, new 9842), Smith Rusk, Smith Brindle |
| Miner's Pick 10074 | iron | blacksmithing 5: 2 iron ingots, a plank |
| Steel Pick 10075 | steel, speed 1.15 | blacksmithing 18: 2 steel ingots, a hardwood board, a leather strip |
| Masterwork Pick 10076 | masterwork, speed 1.3 | blacksmithing 50: 2 crucible steel, an ironwood haft, a leather strip |

Picks follow the tool rules: only crude ones are sold, forged ones are
graded and wear out, and shops never resell them.

## Smelting and drawing

Blacksmithing at a forge: `smelt-iron-ore` (0), `smelt-copper-ore` (0),
`smelt-tin-ore` (0), `alloy-bronze` (6: 2 copper and 1 tin ingot make 2
bronze), `smelt-silver-ore` (12, with coal), `smelt-gold-ore` (22, with coal),
and the existing `steel-ingot` and `crucible-steel`. Jewelcrafting at a
jeweler's bench: `draw-copper-wire` (0, makes 3), `draw-silver-wire` (10,
makes 4), `draw-gold-wire` (24, makes 2), and `flawless-gem-ring` (50). Ore
grade carries into the ingot and from the ingot into the gear (one grade
above the worst input, `gather.CraftGrade`). New items: copper, tin, silver
and gold ore (40260 to 40263), copper, tin, bronze, silver and gold ingots
(40264 to 40268).

`TestMiningContent` checks that every metal any recipe uses can be reached
from mined ore.

## Metals in gear

A smith can make a dagger, short sword, buckler or helm in copper, bronze,
silver or gold. Iron and steel are the existing pieces. The variants are
authored from the iron piece with these multipliers:

| Metal | Damage | Speed | Protection | Weight | Value | Extra | Skill |
|---|---|---|---|---|---|---|---|
| copper | 0.85 | 1.00 | 0.80 | 1.05 | 1.2 | | 0 to 8 |
| bronze | 0.95 | 1.00 | 0.95 | 1.10 | 1.6 | | 4 to 12 |
| silver | 0.85 | 1.05 | 0.85 | 1.10 | 6 | +2 magical protection on armour | 14 to 22 |
| gold | 0.70 | 0.90 | 0.60 | 1.50 | 15 | +2 Charisma | 24 to 32 |

Value is also at least 1.3 times the materials. A Gold Buckler blocks 9
against iron's 15, protects 5 against 8, weighs 9 against 6, and is worth 418
against 12. Items: daggers 10077 to 10080, short swords 10081 to 10084,
bucklers 20125 to 20128, helms 20129 to 20132, the flawless gem ring 20133.
Gear grades apply on top (`help craft`).

## Not done

- Forage still turns up iron ore, basalt-iron and coal in mountains and
  caves, and lake-iron in water. Mining is the reliable source; forage is a
  lucky find.
- Obelisk Glass is still rift-only.
- No mining hazards (cave-ins, bad air) and no NPC miners.
