# Merchants: night light audit and seven new shops (design record)

This is the approved proposal, kept verbatim below the rulings, with the owner's rulings of
2026-09-30 and a note of what was built. Where a ruling and the proposal differ, the ruling wins.
Branch `feature/merchant-shops-and-lights`, from `origin/master` at `66b529e49`.

## Owner rulings (2026-09-30)

1. **Shopkeeper lights follow the 5b ruling.** Street and stall keepers whose shop is too dark
   to trade at night while they are awake carry an Oil Lantern (40038) in the `light` slot,
   Fence Dealer Siv (104) included. Keepers inside buildings authored with outdoor light
   (inns, taverns, mills, smithies) get room light instead: `skylight: 0.15`, `lamp: 50`. The
   Travelers' Inn (6252) counts as a room. The lighting day-cycle golden is re-recorded
   deliberately, and the commit states which rooms moved.
2. **Rane (9588)** gets an Oil Lantern. Room 6443 keeps its `lamp: 38`.
3. **Saved instances:** the owner deletes mob and room instance saves at every deploy, so no
   code fallback for a restored empty light slot (section 2.6) is built. This goes in the PR's
   deploy notes.
4. **The seven merchants** become real shops with partial stock from existing items only.
   Missing stock (ale, scholarly books, a Thornwall cloth bolt) is a follow-up; room rental,
   milling and "information" are mechanics and out of scope. The proposal's housekeeping
   applies (`noncombat_shopkeeper`, `maxwander: 0`, groups, a valid `craft_support`). Whisper
   does NOT join the fence group.
5. **Midday glare** from lanterns in open-sky rooms is accepted.

## As built

- **Lanterns (19 existing shops plus two of the seven):** 97, 98, 104, 250, 9128, 9174, 9191,
  9215, 9303, 9333, 9412, 9421, 9428, 9429, 9492, 9505, 9531, 9588, 9607, and 273 Whisper and
  348 Miller Bram.
- **Room light (11 rooms, 12 keepers):** 423 (88, and 85 in the evening), 424 (85), 4045 (278),
  5245 (9116), 5378 (9171, 9172), 5448 (9197), 5449 (9196), 6114 (9423), 6116 (9424), 6252
  (9486), 6280 (9502). `biome:` is unchanged. Both lighting goldens were re-recorded: the
  day-cycle diff is exactly these 11 rooms in all 12 sections. Night, dawn and dusk rise to
  50; noon falls to 54, 57 and 59 (midwinter, equinox, midsummer) because the roof now cuts
  the sky.
- **Pinned to the counter (`maxwander: 1` to `0`):** 9172 Sly Tam, 9209 A Market Hawker and
  9215 A River-Road Smuggler. A wandering keeper with no light of his own walked into dark
  rooms at night and refused trade there (Tam into 5375 to 5377 and 5379, the Hawker into
  5474). 9174 Old Mabbot keeps `maxwander: 4`: he is a road peddler, and he carries a lantern.
- **Stock**, bare item ids:
  - 102: 40007, 40002, 20052, 20022, 20023, 40103, 40102, 40105, 40038, 20096.
  - 88: 30021, 40103, 40102, 40105, 20096, 40038.
  - 273: 33, 36, 20097.
  - 278: 30023.
  - 348: 40076, 40151.
  - 9309: 40126, 40125, 40152, 40017.
  - 9509: 40137.
- **Differences from the proposal, all from the rulings:** 40145 A Bolt of Homespun is left off
  102 and 40134 Mug of Ale is left off 278 (their text names the Confluence; follow-ups). 88
  and 278 get room light, not a lantern. 348 keeps the lantern the proposal gave it: its
  watermill (4135) is `city_backstreet`, and the proposal lists it with the street keepers.
- **Tests:** `shop_night_trade_guard_test.go` (every shop keeper, awake in its shop at a
  sunless hour on midwinter, the equinox or midsummer, can trade with a bare human; red on
  master with the 29 keepers of section 2.2) and `merchant_shops_content_test.go` (the seven
  have valid shop blocks and `shops.ValidateShopMobTags` passes).

---

# The approved proposal

Read from `origin/master` at `66b529e49` (fetched 2026-09-30) through `git archive` into the
scratchpad. No repo file was edited. The scripts that produced every number below are in the
scratchpad: `audit.py`, `dark.py`, `items_idx.py`, `s.py`.

Everything below is for a normal human observer (species 1 declares no vision flag,
`species/1-*.yaml:32 disabledslots: []`, no nightvision or infra key). A player who carries
their own light, or has night vision, passes the gate on their own.

---

## 0. Facts verified against source

| Fact | Value | Source |
|---|---|---|
| Dark-trade gate | refuses when `LightBand(c, room) < BandFaces` | `internal/actions/shop_sight.go:20-24` |
| Band order | Dark, Shapes, Faces, Dazzled | `internal/messaging/band.go:21-26` |
| Faces edge | `light >= dimBelow - strength` | `internal/messaging/window.go:40-43` |
| `LightDimBelow` (faces edge) | **50**, Go default; key absent from `config.yaml` (comment says so at `config.yaml:944`) | `internal/configs/config.balance.lighting.go:27-31` |
| `LightBlindBelow` | 25, Go default, absent | same |
| `LightDazzleAbove` | **75**, shipped | `_datafiles/config.yaml:946` |
| `LightDoublingStep` | 8, Go default, absent | `config.balance.lighting.go:116-117` |
| Night sky | sun is Absent when sin(alt) <= 0; moon runs `LightStarlight` 10 (all new) to `LightMoonsFull` 35 (all full), Go defaults, absent | `internal/gametime/celestial.go:113-115`, `:142-158`; `config.balance.lighting.go:160-165` |
| Room light | `Combine(step, Attenuate(sky, fraction), lamp, every carried light)` | `internal/rooms/lighting.go:97-146` |
| Carried light counts mobs | loops `r.mobs` then `r.players` | `internal/rooms/lighting.go:163-172` |
| Room overrides | `skylight:` and `lamp:` on the room beat the biome | `internal/rooms/rooms.go:101-102`, `lighting.go:198-218` |
| Biome light defaults | backstreet 0.95/35, thoroughfare 0.95/52, interior 0.15/50, cave 0.0/none, fort 0.35/none, dungeon 0.0/none; river, farmland, water, plains, land, shore 1.0/none | `biomes/*.yaml` (`city_backstreet.yaml:9-10`, `city_thoroughfare.yaml:9-10`, `interior.yaml:7-8`, `cave.yaml:5`, `fort.yaml:5`) |
| Light items | Candle 40077 -> cond 124 (38); Oil Lantern 40038 -> 125 (52); Torch 20096 -> 126 (56); Hooded Lantern 20097 -> 127 (54, flag `adjustable`) | `conditions/124..127-*.yaml:8`, `127-hooded_lantern_light.yaml:10` |
| Mob light slot | `Worn.Light` yaml key `light` | `internal/characters/worn.go:34` |
| Mob equipment and shop YAML path | `character.equipment`, `character.shop` | `internal/characters/character.go:131,148` |
| Shop entry fields | `itemid, quantity, quantitymax, price, ...` | `internal/characters/shop.go:18-27` |
| Mobs wearing a light today | **0** of 642 parsed mob files (62 have some equipment) | `audit.py` over `mobs/**` |
| Shop mobs today | **101**, all `non_combatant: true`; 96 `noncombat_shopkeeper`, 4 questgiver, 1 passive | `audit.py` |
| `craft_support` rule | required on any `HasShop()` or crafter mob unless `IsFence()`; must be one of 7 values | `internal/shops/validation.go:49-63`, `shopinventory.go:29-37` |
| Fence groups | `BaubleFenceGroups: [fence]` | `_datafiles/config.yaml:1537`, `internal/mobs/mobs.go:1000` |
| Shop seeding | each `shop:` item gets `RestockQty 5`, `MaxStock = rarity_tier x stock_multiplier x ShopMaxStockMultiplier`, else 20; gold floor 500 | `internal/mobs/crafter.go:59,73-78,88-89`; `internal/shops/effective_max_stock.go:28-41`; `config.yaml:1431` (2.0) |
| Sleep gate runs before sight gate | `list`, `buy`, `sell` | `internal/usercommands/list.go:34,40`; `buy.go:22`; `sell.go:23`; `internal/actions/sleeping_target.go:78-98` |

---

## 1. Decision 1: Siv wears an Oil Lantern

Siv (104) stands in room 475, `biome: city_backstreet` (`rooms/thornwall_city/475.yaml:10`,
spawn at `:27-28`), no schedule, so awake at all hours. Night level 36 (new moons) to 43 (full),
below faces. With the lantern: 55 to 56. Verified form: the slot key is `light`
(`worn.go:34`) under `character.equipment`, the same shape as his weapon
(`mobs/thornwall_city/104-fence_dealer_siv.yaml:39-41`):

```yaml
  equipment:
    weapon:
      itemid: 10009
    light:
      itemid: 40038
```

How it lights: spawn calls `mob.Character.Validate(true)` (`internal/mobs/mobs.go:773`), which runs
`reapplyPermanentConditions` (`internal/characters/validate.go:718-720`); that adds every worn
item's `wornconditionids` as permanent (`internal/characters/conditions.go:249-253`), and a
permanent record never expires (`internal/conditions/conditions.go:441-443`). Condition 125 is a
light source, so `carriedLight` adds 52 to room 475 (`lighting.go:163-167`).

---

## 2. Night light audit of every shopkeeper

### 2.1 Light per room class at night

Computed with the engine's formula (Combine and Attenuate, step 8, rounded and clamped as in
`lighting.go:129-145`). "new" is every moon new (10), "full" every moon full (35).

| Room class | Night base (new / full) | + Candle 38 | + Oil Lantern 52 | + Hooded 54 | + Torch 56 |
|---|---|---|---|---|---|
| Open sky, no lamp (river, farmland, water, plains, land, shore) | 10 / 35 | 39 / 45 | **52 / 54** | 54 / 56 | 56 / 58 |
| city_backstreet (0.95, lamp 35) | 36 / 43 | 45 / 49 | **55 / 56** | 56 / 58 | 58 / 59 |
| fort (0.35, no lamp) | -2 / 23 | 38 / 41 | **52 / 53** | 54 / 55 | 56 / 57 |
| Room 6443 (dungeon, room `lamp: 38`) | 38 / 38 | 46 | **55** | 57 | 58 |
| cave (0.0, no lamp) | 0 / 0 | 38 | **52** | 54 | 56 |
| interior (0.15, lamp 50) | **50 / 50** (exactly on the edge) | 54 | 59 | 60 | 61 |
| city_thoroughfare (0.95, lamp 52) | **52 / 54** | 55 | 60 | 61 | 62 |

The candle never reaches faces anywhere that is dark (best case 49, backstreet under full
moons). Interior shops pass at exactly 50: any future raise of `LightDimBelow` refuses every
interior shop at night.

### 2.2 Which shops are dark while the keeper is awake

"Dark" also covers dawn and dusk: a low sun is below 50 in open sky too, so the hours below are
hours when the room reads under faces on at least one day of the year, intersected with hours
the keeper's schedule has them awake in the shop room (`activity` not `sleeping`, same
`target_room`). A keeper with no schedule never sleeps (sleep comes only from a `sleeping`
segment, `internal/hooks/NewRound_IdleMobs_schedule.go:107,117`). "Every day" means dark on
every day of the year.

**29 of the 101 existing shops are below faces while their keeper is awake and in the shop.**
27 of them are dark every night of the year while open; 85 and 97 only at winter dawn or dusk.
Two are dark in daylight too: 9588 (38 at every hour) and 9116 (fort, dark even at winter noon).
Every one of the 29 reaches faces with an Oil Lantern (`dark.py`, "with lantern still dark: []"
for all).

| Mob | Name | Room | Room biome, why dark | Awake and dark (every day) | Also dark some days | Setting | Fix |
|---|---|---|---|---|---|---|---|
| 85 | Merchant Brecca | 424 Trading Post | river, open sky | none | 6-8, 15-17 | timber building, shutters open | room light (B) |
| 97 | Blacksmith Kerra | 470 Craftsmen's Quarter, East | backstreet | none | 15-17 | street forge | lantern (A) |
| 98 | Apothecary Voss | 471 Apothecary Lane | backstreet | 19-20 | 6-8, 15-18 | lane | lantern (A) |
| 104 | Fence Dealer Siv | 475 Back Alley, East | backstreet | 0-4, 19-23 | 5-8, 15-18 | alley | **lantern (decided)** |
| 250 | Peddler Malk | 4003 Peddler's Camp | plains | 0-4, 19-23 | 5-8, 15-18 | lean-to camp | lantern (A) |
| 9116 | Smith Rusk | 5245 The Coulee Smithy | fort, no lamp | 0-6, 17-23 | 7-16 | "roofed with split logs" | room light (B) |
| 9128 | Herbalist Birna | 5265 The Sheltered Pool | water | 0-4, 19-23 | 5-8, 15-18 | pool | lantern (A) |
| 9171 | Goodwife Pemberton | 5378 Lake & Ladle, Common Room | farmland | 0-4, 19-23 | 5-8, 15-18 | inn common room, hearth | room light (B) |
| 9172 | Sly Tam | 5378 (same) | farmland | 0-4, 19-23 | 5-8, 15-18 | same room | room light (B) |
| 9174 | Old Mabbot | 5373 Rolling Pasture Road | farmland | 0-4, 19-23 | 5-8, 15-18 | road | lantern (A) |
| 9191 | Tamsin Reed | 5425 Ford Approach | land | 0-4, 19-23 | 5-8, 15-18 | road | lantern (A) |
| 9196 | Brannick Oats | 5449 The Sheaf and Sickle | farmland | 0-4, 19-23 | 5-8, 15-18 | tavern room | room light (B) |
| 9197 | Dawkin the Miller | 5448 The Watermill | farmland | 0-4, 19-23 | 5-8, 15-18 | inside the mill | room light (B) |
| 9215 | A River-Road Smuggler | 5479 The Ford | river | 0-4, 19-23 | 5-8, 15-18 | ford | lantern (A) |
| 9303 | Dunmar Wells | 5505 Warehouse Row | backstreet | 19-20 | 6-8, 15-18 | street | lantern (A) |
| 9333 | Master Halvard | 5709 The Forge Yard | backstreet | 19-20 | 5-8, 15-18 | yard | lantern (A) |
| 9412 | Birrel the Netmender | 6098 Netmender's Row | water | 0-4, 19-23 | 5-8, 15-18 | village stall | lantern (A) |
| 9421 | Pella the Fish-Trader | 6110 The Fish Quay | water | 0-4, 19-23 | 5-8, 15-18 | quay | lantern (A) |
| 9423 | Ferrick the Chandler | 6114 The Chandlery | water | 0-4, 19-23 | 5-8, 15-18 | "squat building with an open front" | room light (B) |
| 9424 | Sybba the Tavern-keeper | 6116 The Quayside Tavern | water | 0-4, 19-23 | 5-8, 15-18 | "Inside, benches" | room light (B) |
| 9428 | Varro the Importer | 6128 The Spice Quay | water | 0-4, 19-23 | 5-8, 15-18 | quay | lantern (A) |
| 9429 | Lenne the Provisioner | 6127 The River Market | water | 0-4, 19-23 | 5-8, 15-18 | market stalls | lantern (A) |
| 9486 | The Innkeeper | 6252 The Travelers' Inn | backstreet | 19-22 | 5-8, 15-18 | inn (text reads as the frontage) | owner call: A or B |
| 9492 | Mistress Odell | 6268 The Wheatside Hamlet | farmland | 19-22 | 5-8, 15-18 | awning stall | lantern (A) |
| 9502 | Maret the Miller | 6280 The Watermill | water | 19-21 | 5-8, 15-18 | "wheel turns outside the east wall" | room light (B) |
| 9505 | Fenn the Fishmonger | 6282 Riverside Row | backstreet | 0-4, 19-23 | 5-8, 15-18 | row of stalls | lantern (A) |
| 9531 | Wren the Ostler | 6319 The Coaching Stable | backstreet | 19-22 | 6-8, 15-18 | open yard | lantern (A) |
| 9588 | Enchanter Rane | 6443 The Chrysalis Workshop | dungeon, `lamp: 38` (`rooms/stillwater/6443.yaml:15`) | **6-21, all day** | | interior, lamp deliberately dim | room lamp to 50, or lantern |
| 9607 | Jeweler Vurl | 6462 The Gem Stall | shore | 19-21 | 6-8, 15-18 | stilted stall | lantern (A) |

Among the seven merchants of decision 3, 88, 273, 278 and 348 are dark too (section 3).

### 2.3 Which light item

Oil Lantern 40038 for every keeper who needs one:

- **Candle 40077 fails** everywhere that is dark (table 2.1).
- **Torch 20096** passes but dazzles more by day (below).
- **Hooded Lantern 20097 is unsafe on a keeper.** Its condition is `adjustable` and trims to
  its bearer's eyes, but only when the bearer arrives in a room: "Nothing calls it on a round
  tick: a room that changes around a standing bearer leaves their light as it was"
  (`internal/rooms/light_trim.go:22-25,37`; mob arrival at `rooms.go:1253`). A static keeper
  spawned at noon trims down and stays trimmed into the night. Only in an always-dark room
  (Whisper's cave) does it stay at full strength, since nothing else lights the room.
- **Daytime cost of any carried light in an open-sky room.** A peak summer sun reads 73 there
  without a lantern. With the Oil Lantern it reaches **75, the dazzle edge**, on about 86 days a
  year between 10:00 and 13:59 (open sky) and 76 days (backstreet); a torch on 126 and 122 days,
  09:00 to 14:59. Dazzle does not refuse trade (the gate is only `< BandFaces`), but it scales
  the bartering discount by `SightMult` (`shop_sight.go:57`) and costs any other sight roll
  made in that room. The lantern is the least-dazzling light that reaches faces.

### 2.4 Does an equipped light shine while the keeper sleeps? Yes. Does it matter? Barely.

- It shines: the record is permanent (`conditions.go:441-443`); `LightNow` returns nothing
  only for expired, hooded or trimmed records, never for sleep (`internal/conditions/light.go:36-39`);
  `carriedLight` adds every mob in the room with no sleep check (`lighting.go:163-167`).
- For trade it does not matter: `ShopClosedForSleep` refuses before the sight gate runs
  (`usercommands/list.go:34` then `:40`; `buy.go:22`; `sell.go:23`).
- Side effects: the room stays lit overnight, so a sneaker there loses the dark
  (`config.yaml:901-903`), and a keeper who sleeps elsewhere carries the light there (97 Kerra
  sleeps in 5101, 9333 Halvard in 5710, 85 Brecca spends 18-22 in 423).
- It also shines by day, which is the dazzle cost of 2.3.

### 2.5 The owner's ruling, and how an equipped light fits it

From `project-lighting-5b-owner-rulings-2026-09-26.md:25`, call 9 ("backstreet shops refuse at
night without a carried light"):

> **follow-up: shopkeepers bring their own light while awake (behaviour), unless their schedule puts them to sleep; a shop in a separate room off the street gets its own light (content)**

| Ruling says | Equipped light does | Gap |
|---|---|---|
| Keeper brings light **while awake** | Always on: awake, asleep, day and night | Shines asleep (harmless to trade, 2.4) and by day (dazzle cost, 2.3) |
| It is **behaviour** | It is content: one YAML line per keeper, no code | No behaviour tree action equips or snuffs a light today |
| **Separate room** gets its own light (content) | Nothing, unless the room is changed | Group B rooms are buildings authored with outdoor biomes |

**Recommendation, simplest correct approach grounded in code:**

1. **Group A (street, lane, yard, quay, stall, road): Oil Lantern in the `light` slot.** No code,
   no golden churn, passes every hour, and the "shines while asleep" gap cannot open a shop
   because the sleep gate runs first. Accept the summer-midday dazzle or treat it as the reason
   to build the behaviour later.
2. **Group B (inside a building): give the room its own light, per the ruling.** Set the room
   overrides `skylight: 0.15` and `lamp: 50` (the interior biome's values) rather than changing
   `biome:`, because the biome also drives weather, map symbol and movement cost. This moves
   `testdata/lighting_daycycle.golden` (the test loads rooms, `lighting_daycycle_golden_test.go:104`)
   and needs a deliberate re-record. If the owner prefers one mechanism for everything, the
   lantern also fixes every Group B keeper.
3. 9588 Rane: the room's `lamp: 38` makes the shop refuse **all day**; raise the room lamp to 50
   (content) or give Rane a lantern.
4. The behaviour version (equip at wake, unequip at sleep or in daylight) stays a follow-up. It
   is the only way to remove both gaps; it is not needed for the gate.

### 2.6 Trap: a saved mob instance hides the new lantern

On spawn, a saved instance replaces the whole equipment block: `if savedInstance.Equipment != nil
{ mob.Character.Equipment = *savedInstance.Equipment }` (`internal/mobs/mobs.go:641-643`), and
this runs before `Validate(true)` (`:773`). Saves carry `Equipment` (`internal/mobs/instance_save.go:160-161`)
whenever the mob has progression, planner state, or gold or equipment that differs from its
template (`instance_save.go:382-400`). Any keeper with an existing `mobs.instances/` file,
locally or on prod, spawns **without** the lantern. Locally the smoke-test instance wipe clears
it. For prod this needs either the owner deleting those keepers' instance files at deploy, or a
small code change that fills an empty restored slot from the template. Decide before shipping.

---

## 3. The seven new shops

### 3.1 Conventions and rules that apply

| Rule | Source | Applies how |
|---|---|---|
| `craft_support` required and valid (unless fence) | `validation.go:49-63`; boot panics on a bad tag (`:31-32`) | every block below sets one |
| All shops are `non_combatant` | 101 of 101 (`audit.py`) | all seven already have it |
| `behavior_archetype: noncombat_shopkeeper` | 96 of 101 | switch 102, 88 (questgiver), 9309, 9509 (passive), 348 (none) |
| `maxwander: 0` | 97 of 101 | 88 has 3, 102 has 1: a wandering keeper leaves the shop room empty |
| `shopkeepers` group = Merchants' Concord faction | `factions/shopkeepers.yaml`; membership by group `internal/factions/factions.go:147-155` | 17 of 101 carry it; add where the keeper is a town trader |
| Gold | floor 500; "specialists set 1000g and generals set 5000g" (`crafter.go:55-57`) | used for the older zones below; newer zones keep low gold like their neighbours |
| Quantities | Every entry becomes `RestockQty 5`, `MaxStock` from rarity tier x 2.0 (else 20) (`crafter.go:73-89`). `quantity`, `quantitymax`, `price` are read only by the legacy catalog when no ShopInventory exists (`actions/buy.go:75-96`); `list` reads the ShopInventory (`usercommands/list.go:62`) | bare `- itemid:` entries, as the newer zones do |
| A shop makes the keeper essential and non-despawning | `mobs.go:1055-1058,1071-1076` | pins 7 more rooms in memory |

### 3.2 Per merchant

**Market Merchant 102** (`mobs/thornwall_city/102-market_merchant.yaml`), room 465 Market Square
Center, `city_thoroughfare`, **lit at night (52 to 54)**. Schedule `thornwall_market_merchant`
awake 06-22 at 465, sleeps 22-06 at 465. Description (`:28-30`): "sells cloth, leather goods,
and small hardware".

```yaml
craft_support: general
behavior_archetype: noncombat_shopkeeper   # was noncombat_questgiver
maxwander: 0                               # was 1
groups: [humanoid, merchant, thornwall_citizens, shopkeepers]
character:
  gold: 5000                               # was 500 (general)
  shop:
    - itemid: 40007    # Cloth Strip           (rt 40 -> max 80)
    - itemid: 40145    # A Bolt of Homespun    (rt 40 -> max 80) see gap
    - itemid: 40002    # Leather Strip         (rt 40 -> max 80)
    - itemid: 20052    # Sturdy Leather Boots  (rt 50 -> max 100)
    - itemid: 20022    # Leather Vest          (rt 40 -> max 80)
    - itemid: 20023    # Leather Leggings      (rt 40 -> max 80)
    - itemid: 40103    # Waterskin             (no rt -> max 20)
    - itemid: 40102    # Coil of Rope          (max 20)
    - itemid: 40105    # Tinderbox             (max 20)
    - itemid: 40038    # Oil Lantern           (max 20)
    - itemid: 20096    # Torch                 (max 20)
```

Gaps: no generic bolt of cloth. 40145's text places it "from the weaver's loom in the Craftsmen's
Row" beside "the dyed river-cloth of the quay" (a Confluence reading; sold by 9475, 9477); 40132
Bolt of River-Cloth is "the valley's madder-red, a staple of the river trade". Keep 40145 only if
the owner accepts the wording. No nails or small tools exist. Leather Satchel 20025 and Leather
Backpack 20054 fit "leather goods" but are sold nowhere today and have tailoring recipes, so
selling them would bypass crafting; left out.

**Traveling Merchant 88** (`mobs/watchers_crossing/88-traveling_merchant.yaml`), room 423 The
Crossing Inn, `biome: river` (open sky), no schedule. **Dark: 0-4 and 19-23 every day**, 5-8 and
15-18 some days. Already in `shopkeepers`. Description promises nothing specific: "contents
hidden beneath oiled canvas" (`:30`). Stock is a travel kit and an owner choice.

```yaml
craft_support: general
behavior_archetype: noncombat_shopkeeper   # was noncombat_questgiver
maxwander: 0                               # was 3
character:
  gold: 5000                               # was 500
  equipment:
    weapon: {itemid: 10020}
    neck: {itemid: 20024}
    light: {itemid: 40038}                 # or room 423 gets its own light (Group B)
  shop:
    - itemid: 30021    # Trail Rations  (rt 50 -> max 100)
    - itemid: 40103    # Waterskin
    - itemid: 40102    # Coil of Rope
    - itemid: 40105    # Tinderbox
    - itemid: 20096    # Torch
    - itemid: 40038    # Oil Lantern
```

Room 423 is an inn common room with a hearth under an outdoor biome: Group B. Merchant Brecca
(85) already spends 18-22 in 423 (`schedules/watchers_crossing/watchers_brecca.yaml`); `list`
shows every merchant in the room (`usercommands/list.go` loops `GetMobs(FindMerchant)`), so the
two lists would appear together then.

**Whisper 273** (`mobs/thornwall_city/273-whisper.yaml`), room 507 The Listening Post, `cave`,
no lamp. **Light 0 at every hour: trade refused day and night.** No schedule. Description
(`:35-36`): "deals in information, tools, and discretion".

```yaml
craft_support: general
character:
  equipment:
    light: {itemid: 40038}     # 52 in the cave. 20097 also works here (see below)
  shop:
    - itemid: 33       # Iron Lockpicks     (sold by 104, 250, 9172, 9215, 9323)
    - itemid: 36       # Basic Disarm Kit   (same sellers)
    - itemid: 20097    # Hooded Lantern     (the discretion tool: "hood" closes it)
```

Gaps: **"information" has no item.** Steel/Master Lockpicks (34, 35) and Reinforced/Precision
Disarm Kits (37, 38) are sold nowhere and come only from recipes
(`recipes/blacksmithing/steel-lockpicks.yaml`, `master-lockpicks.yaml`,
`recipes/jewelcrafting/reinforced-disarm-kit.yaml`, `precision-disarm-kit.yaml`), so they are left
out. Whisper's idle text says "the faint light" (`:14-15`); a candle would match the text but
reads shapes only (38), so it cannot open the shop. Do not add the `fence` group without the
owner: `TestEveryTownHasAFenceNearby` (`internal/actions/stolen_bauble_test.go:902`) then
asserts on it. Room 507 sits behind the locked Chrysalis Den exits (patrol note in
`project-movement-4b-playtest-findings-2026-09-29.md:32-34`); players can still reach it.

**Haral 278** (`mobs/north_road/278-haral.yaml`), room 4045 Common Room, `biome: farmland`,
no schedule. **Dark: 0-4 and 19-23 every day.** Tavern owner; his dialogue says "Ale or stew.
That is what I have." (`dialogue/north_road/278.yaml:31`) and "Room rates are fixed. The ale is
not." (`:48`).

```yaml
craft_support: cooking
groups: [humanoid, shopkeepers]
character:
  gold: 1000                   # was 50 (floors to 500 anyway)
  equipment:
    light: {itemid: 40038}     # or room 4045 gets its own light (Group B)
  shop:
    - itemid: 30023    # Hearty Stew   (rt 50 -> max 100)
    - itemid: 40134    # Mug of Ale    (rt 40 -> max 80) see gap
```

Gaps: **no generic ale.** 40134 is the only ale and reads "a dented pewter mug of the Quayside's
own ale" (sold by 9424 at the Confluence). **Room rental has no item or mechanic.**

**Miller Bram 348** (`mobs/stillwater/348-miller_bram.yaml`), room 4135 The Watermill,
`city_backstreet`. Schedule `bram` awake 05-20 at 4135, sleeps 20-05 at 4135. **Dark: 19 every
day**, 5-8 and 15-18 some days. Description (`:33-34`): sells flour "by the sack" and "will also
mill grain you bring yourself for a modest fee".

```yaml
craft_support: cooking
behavior_archetype: noncombat_shopkeeper   # none today
groups: [humanoid, merchant, stillwater_citizens, shopkeepers]
character:
  gold: 1000                   # was 100
  equipment:
    light: {itemid: 40038}
  shop:
    - itemid: 40076    # Sack of Flour               (no rt -> max 20)
    - itemid: 40151    # A Sheaf of Gleaned Grain    (rt 40 -> max 80), optional
```

Gaps: **the milling service has no mechanic.** 40153 A Sack of Milled Flour is "Stone-ground at
Greenford's watermill", so it is left for Maret (9502). 40076 reads "the milled wealth of the
vale", which is loose for Stillwater but not named.

**Fishmonger 9309** (`mobs/new_plymouth_docks/9309-fishmonger.yaml`), room 5504 The Fish Market,
`city_thoroughfare`, **lit at night (52 to 54)**. No schedule. Description: "split, salt, hang,
done" and drying racks (`:30-31`).

```yaml
craft_support: cooking
behavior_archetype: noncombat_shopkeeper   # was noncombat_passive
character:
  # gold 8 kept: floors to 500, matching 9505 (40) and 9421 (40)
  shop:
    - itemid: 40126    # Fresh River Catch   (rt 40 -> max 80)
    - itemid: 40125    # Smoked River-Fish   (rt 40 -> max 80)
    - itemid: 40152    # A River Trout       (rt 40 -> max 80)
    - itemid: 40017    # Salt Pouch          (rt 50 -> max 100), optional
```

Left out: 40131 Salt-Cured Roe ("a Confluence luxury"). No gap.

**Aldith the Bookseller 9509** (`mobs/greenford/9509-aldith_the_bookseller.yaml`), room 6290 The
Bookshop, `interior`, **lit at night at exactly 50**. Schedule `gf_bookseller` awake 07-22,
sleeps 22-07 at 6290. Room text: "university texts: natural philosophy, survey and measurement,
cartography ... a long run of history".

```yaml
craft_support: general           # precedent: Drunn the Bookseller 9444
behavior_archetype: noncombat_shopkeeper   # was noncombat_passive
character:
  shop:
    - itemid: 40137    # A Confluence History (rt 40 -> max 80)
```

Gaps: **only one book exists.** No natural philosophy, survey, measurement or cartography item;
40160 A Surveyor's Marked Map is `not_salable: true` and a quest reward (`quests/75-the_surveyors_report.yaml:96`).
Her schedule text says she sleeps "in the room above the shop", but `target_room` is 6290 for
sleep too (`schedules/greenford/gf_bookseller.yaml:33-35`).

### 3.3 Dark status summary for the seven

| Mob | Room | Night level | Needs light |
|---|---|---|---|
| 102 | 465 thoroughfare | 52 to 54 | no |
| 88 | 423 river | 10 to 35 | yes (lantern, or room light: Group B) |
| 273 | 507 cave | 0 always | yes, all day (lantern) |
| 278 | 4045 farmland | 10 to 35 | yes (lantern, or room light: Group B) |
| 348 | 4135 backstreet | 36 to 43 | yes (lantern) |
| 9309 | 5504 thoroughfare | 52 to 54 | no |
| 9509 | 6290 interior | 50 (on the edge) | no |

### 3.4 Tests

- **No test pins any of the seven as a non-shop.** Test hits on ids 102, 88 and so on are
  synthetic fixtures, not YAML (for example `internal/actions/aggression_test.go:67`,
  `internal/crimes/crimes_test.go:240`, `internal/mobs/instance_save_archetype_test.go:107`).
- **No test pins a shop count.** Tests that read real mob YAML: `stolen_bauble_test.go:902`
  (only `fence`-group mobs), `internal/mobs/template_training_test.go:26` (a template count in a
  comment). `HasShop()` in tests only on fixtures (`internal/mobs/mobs_test.go:593-598`,
  `internal/caravan/visit_test.go:537-551`).
- **Lighting goldens do not load mobs** (`lighting_daycycle_golden_test.go:104-106`,
  `lighting_parity_golden_test.go:95-97` load rooms, conditions, mutators only), so equipped
  lights do not move them; room `lamp:`/`skylight:` edits do.
- Boot validation: `ValidateShopMobTags` panics at cold boot on a missing or bad
  `craft_support` (`validation.go:31-32`); every block above sets a valid one.

---

## 4. Owner calls

1. Group B (and 88, 278): room light per the ruling, or the lantern everywhere for one mechanism?
2. 9486's Travelers' Inn (6252): its text describes the frontage; street (lantern) or room?
3. 9588 Rane: raise room 6443's `lamp` from 38 to 50, or give Rane a lantern and keep the green gloom?
4. Summer-midday dazzle from lanterns in open-sky rooms: accept, or build the behaviour?
5. Prod instance saves (2.6): delete the affected keepers' instance files at deploy, or code the
   template fallback for an empty restored light slot?
6. Stock gaps with no item: ale (Haral), room rental (Haral), milling service (Bram),
   "information" (Whisper), scholarly books (Aldith), a Thornwall cloth bolt (Market Merchant).
   Accept the partial stock, or author items first?
7. Every change here is player-facing content, so the plan ends with the adversarial playtest
   gate (`dogmud-authoring-content`).
