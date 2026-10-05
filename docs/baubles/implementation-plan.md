# Baubles: AI-named loot from `search`

Status: Phases 0 to 6a written in one cumulative patch, built and tested
(full suite, race detector on the touched packages, JS tests). Rebased onto
the owner's DOGMud master at `cf5af4c55` (see section 6).

A player who uses `search` has a small chance to turn up a **bauble**: a
non-usable, non-wearable object that exists only to be sold. When one is
found, an OpenAI model names it, describes it, weighs it and prices it from
the room and region it was found in. There is exactly one item file (the
carrier, id 900); every bauble is that id plus a catalog record, and the
catalog (a separate store, not item files) holds the name, description,
weight, value, where it was found, and whether it was stolen. With the model
off or unreachable the game behaves the same, with plainer local names.

## 1. Fixed rules

### 1.1 Value ladder (deterministic)

| Tier | Gold |
|------|------|
| cheap | 1 to 6 |
| average | 10 to 15 |
| rare | 40 to 200 |

- The game picks the tier BEFORE generation. How it picks is Phase 4 work
  (see section 4). The model then picks a value inside that range, and code
  clamps the answer (`baubles.ApplyLimits`). The model never picks the tier.
- The gaps (7 to 9, 16 to 39) are intentional: a price tells the player the
  tier.
- An unknown or missing tier is always treated as cheap.
- Written in Phase 0 as `internal/baubles/tiers.go`, pinned by
  `TestTierRangesAreTheSpec`. Phase 3 lifts it into `config.yaml` with the
  other bauble knobs, because balance numbers belong there; Phase 0 does not
  touch `config.yaml` (skip-worktree, and local edits in the working copy).

### 1.2 Weight (model decides)

The model judges weight from what the object is and what it is made of: a
child's wooden toy is a fraction of a pound, a large vase several pounds.
The prompt gives it a scale (`baubles.WeightGuidance`: tiny, small, medium,
large, with examples) so weights are consistent with each other and with
the game's existing items. Code only clamps: 0.1 to 25 lb, rounded to 0.1;
a missing or nonsensical weight becomes 0.5 lb. Weight feeds encumbrance
exactly like any other item, through the GetSpec overlay (Phase 1).

### 1.3 Roll

- The chance per roll depends on where the search is and who searches
  (Phase 5b): by room biome (buildings 5%, streets 2 to 2.5%, roads 1%,
  wilderness 0.25%, deep water never), raised by search skill up to double.
  Two rolls per room per window, window of one hour. Rolls are shared by
  everyone in the room by default.
- Mobs never roll. A FIND trains search (a won search); a roll that finds
  nothing awards nothing. A closed window says nothing, and "You find
  nothing of interest." is unchanged, so search does not become an oracle.

## 2. What the codebase gives and constrains

- **Transport.** Since Phase 5f there is one mechanism for every feature
  that calls a model: `internal/apiframework` (wire format, transport with
  the consent door, the server key, one daily budget, one breaker, and a
  registry through which a player's relay key can be lent). The AI companion
  and baubles both use it. The earlier `internal/openaiclient` (Phases 0 to
  5e) is gone; see Phase 5f.
- **Item identity.** `items.Item.GetSpec()` is the single read point for
  name, description, value and weight, so a bauble resolves its fields from
  the catalog there. No spec snapshot is stored on the item (unlike affixed
  loot), so save files stay small and admin edits in the catalog show
  everywhere at once.
- **Three silent breakages to prevent.** `SameStack` would stack all
  baubles into one line (same item id, no Spec); `actions.Sell` and
  `mobs.GetSellPrice` price and restock by item id; search's
  `FoundAnything()` must list every new tier.
- **Region.** `rooms.ZoneConfig.Region` where set, else the zone name.
- **Persistence.** Durable writes only (`util.Save`), runtime data
  gitignored like `users` and `shops`. No SQL driver in `go.mod`; the catalog
  starts as sharded files behind a `Store` interface, SQLite optional later.

## 3. Data model

One item file: `_datafiles/world/dogmud/items/other-0/900-curious_trinket.yaml`
(`type: object`, `subtype: mundane`, name "Curious Trinket", keyword
`trinket`; every bauble also answers to `bauble`). The loader requires the
file name to be `<id>-<name>.yaml` from the item's own name, so it cannot be
called `900-bauble.yaml`. It is `not_salable` until Phase 2.

Item instance field: `Bauble string yaml:"bauble,omitempty"`, the record id.

Catalog record (`internal/baubles`, Phase 1):

```text
Id, Status (ready | fallback | sold | retired)
Name, NameSimple, Description, Material
Tier (cheap | average | rare), Value, ValueProposed
WeightLbs, WeightProposed
Source (search | pickpocket | burglary | admin)
RoomId, Zone, Region, Biome, FoundByUserId, FoundRound, FoundAt
Stolen, StolenFromMob, StolenFromName, StolenFaction, StolenAt
Generator (openai | local | admin), Model, PromptVersion, Tokens,
Moderated, EditedBy, SoldAt, SoldValue
```

Store: `_datafiles/world/dogmud/baubles/catalog-NNNN.yaml`, 500 records per
shard, plus `meta.yaml` (next id). Write-through: every change rewrites its
shard at once through `util.Save`; a failed write is retried by `SaveAll` at
shutdown and copyover. A corrupt shard is renamed aside and its ids are
never reissued. Ids are `B0000001`, `B0000002`, ...

Model reply (written in Phase 0, `baubles.ReplySchema`, strict JSON):

```text
name, name_simple, description, material, weight_lbs (number), value (integer)
```

## 4. Phases

### Phase 0: Transport, value ladder, weight rules (written; revised on rebase; transport superseded by Phase 5f)

- `internal/openaiclient`: `Call` (one retry on 429, 5xx or no answer),
  `Moderate`, `ListModels`, `EndpointAllowed`, `ResolveKey`, `Breaker`, and
  the wire types, first copied from `modules/aicompanion/openai.go`.
- As first written, `modules/aicompanion/openai.go` became aliases over it.
  The rebase onto the owner's master DROPPED that: the companion's transport
  now carries the consent door and relay route and is left exactly as the
  owner wrote it. The patch no longer touches `modules/aicompanion` at all.
- `internal/baubles`: the value ladder, weight bounds and scale, the reply
  schema, `ParseReply` and `ApplyLimits`.
- Exit: `go test ./internal/apiframework/... ./internal/baubles/...`;
  `python tools/context_md_audit.py` clean.

### Phase 1: Carrier item and catalog-backed display (written)

- Carrier file; `Item.Bauble`; resolver seam in `internal/items`
  (`SetBaubleResolver`) so `items` never imports `baubles`.
- GetSpec overlay: Name, NameSimple, Description, Value, **Weight**.
- `NameMatch` accepts `bauble` and `trinket`; `SameStack` never stacks
  baubles.
- Catalog, sharded store, boot load, flush on autosave and shutdown.
- Local fallback namer by biome; value from `Tier.RollValue`; weight from a
  per-object default in the word list.
- Admin `bauble spawn [cheap|average|rare]`, `bauble show <id>`,
  `bauble list [n]`, with its help template.
- Boot load (first boot only, not on data reload) after
  `items.LoadDataFiles()`; `SaveAll` at shutdown and copyover.
- Runtime dir gitignored with `.gitkeep`; skipped by the messaging surface
  guard and `tools/messaging_surface_audit.py`.
- As written: the catalog is write-through rather than flushed on the
  autosave cadence (records change rarely, and this guarantees a record is on
  disk before any save file can point at it). `Mint(MintOpts)` is the single
  way to make a bauble; Phase 3 search and Phase 6 theft call it.
- Exit: spawned baubles keep unique names and weights across a restart;
  `go test ./internal/items/... ./internal/baubles/...
  ./internal/usercommands/...` and the root guards green.

### Phase 2: Selling and appraisal (written)

- `internal/actions/sell_bauble.go`: the bauble branch of
  `sellOneToMerchant`, and `baubleOfferFor` for `resolveMerchant`. Price =
  catalog value × `ShopBuyRatio`, rounded up, min 1 (the affixed-loot
  spread: no scarcity curve, no barter bonus). Merchant gold and the
  living-economy reserve are respected. The record is marked sold. Unknown
  records are refused with a spoken line. (Since slice D, Phase 6f, an
  average or rare bauble a player sells to a living shop is shelved for
  resale; the rest are destroyed.)
- Who buys: living-economy shops with `craft_support` general or
  jewelcrafting, and every legacy merchant. (Owner question 4, answered
  provisionally; the list moves to `config.yaml` in Phase 3.)
- `mobs.GetSellPrice` returns 0 for any bauble, so no ItemId-keyed path can
  price or shelve the carrier. The carrier stays `not_salable`.
- `sell all bauble`: each sale is named by the item actually sold, and
  `SellResult.Mixed` makes the command say "You sell 3 items" instead of
  pluralising one bauble's name.
- `offer` quotes the bauble price. `appraise` is free for baubles and shows
  description, material, weight, worth, region found, and this merchant's
  price. Stolen status is never shown (Phase 6).
- Telemetry: `baubles.MarkSold` logs each sale; `baubles.SalesSince` feeds
  24h and 7-day totals at the top of `bauble list`.
- Player help for `sell` and `appraise`, admin help for `bauble`, updated.
- Exit: `go test ./internal/actions/... ./internal/baubles/...`; in game,
  `bauble spawn average`, `get bauble`, `appraise bauble` and
  `sell bauble` at a general store pay 5 to 8 gold and (before slice D)
  left nothing on the shelf. Since Phase 6f the store shelves it.

### Phase 3: Search integration (local names only) (written)

- Config: a BAUBLES block in the Balance section of `config.yaml` (placed in
  the shop economy section, clear of the owner's local edits), with Go
  defaults in `internal/configs/config.balance.baubles.go`:
  `BaubleSearchDisabled` (false), `BaubleSearchChancePct` (3),
  `BaubleRollsPerWindow` (2), `BaubleWindowMinutes` (60, real),
  `BaubleWindowPerPlayer` (false), `BaubleExcludedZones` ([]), tier weights
  (70/25/5), the six ladder numbers (1-6, 10-15, 40-200) and
  `BaubleBuyerCraftSupports` (general, jewelcrafting).
  `TestBaubleShippedConfigMatchesDefaults` pins the file to the defaults.
  Zero means "default" (an absent key decodes as zero); the switch is
  `BaubleSearchDisabled`. A broken ladder resets all six numbers.
- The ladder (`tiers.go`) and the buyer list (`sell_bauble.go`) now read the
  config.
- `baubles.TryFind`: switch, excluded zone, roll window, chance (in
  hundredths of a percent), tier by weight, `Mint`. Windows are kept in
  memory in the baubles package, NOT in room temp data as first planned:
  rooms unload when nobody is near, which would reset a room-held window.
- Tier 4 in `actions/search.go` (`search_bauble.go`): players only; never in
  instance/ephemeral rooms, banks, storage or character rooms; never sets
  `rolledAgainstSomething`; in `FoundAnything()` but not in the new
  `foundByContest()` that feeds the award, so a lucky bauble cannot turn a
  lost search into a won one. Finds go to the backpack, or the floor when
  the player cannot carry them. Failures are silent.
- Admin: `bauble window [reset]` shows or reopens the room's window.
- Tier choice moved here from Phase 4 (search mints now, so it needs a tier).
- Exit: `go test ./internal/configs/... ./internal/baubles/...
  ./internal/actions/...`; in game, `bauble window reset` then `search`
  repeatedly: at most 2 rolls, then nothing for an hour.

### Phase 4: OpenAI generation (`modules/baubles`) (written)

Owner rulings for this phase:

- **No "unexamined" state.** A find is named first and handed over only when
  its text is final. The search itself takes longer instead: the player is
  told they are working something loose, and the bauble arrives when the
  API call has gone through (or failed).
- **No API key means a generic "Trinket"** for every find: a simple
  description, a random weight (0.1 to 0.8 lb) and a random value inside the
  tier. The same generic trinket is the fallback for every failure (module
  off, over budget, breaker open, timeout, API error, refusal, moderation
  flag or failed check, text that fails validation).

As written:

- `internal/baubles`: `RollFind` replaces `TryFind` (roll only, no mint);
  `Generate` (blocking; never fails) behind a generator seam
  (`SetGenerator`, installed by the module); `CleanReply` (text checks the
  schema cannot make: lengths, digits, markup, keyword collisions with real
  items); `GenericTrinket` replaces the biome word-list namer; `Mint` takes
  the finished `GenResult`; `RecentNames` for the prompt. The `pending`
  status and its placeholder text are gone.
- `internal/actions/search_bauble.go`: on a find, "Something glints among
  the clutter. You set about working it loose..." then a `BaubleDelivery`
  goroutine: `Generate` off the lock, wait out the rest of
  `BaubleRevealSeconds` (so generic and model-named finds arrive at the same
  pace), then `util.LockMud()` once to mint and deliver (pack, feet, or the
  room where it was found if the finder logged off). `BaubleRequest` copies
  authored room text only.
- Config: `Balance.BaubleRevealSeconds` (3, max 30). `Modules.baubles` block
  (see `modules/baubles/context.md`): enabled by default, key from
  `OPENAI_API_KEY`, then `APIKey`, then the aicompanion key; gpt-5-nano,
  minimal reasoning, 15 s timeout, 300k tokens a day, 4 at once, breaker
  3 failures / 300 s, moderation on and failing closed.
- `modules/baubles`: prompt (world tone from Gaius, what a bauble may and may
  not be, the tier's value rule, `WeightGuidance`, names to avoid), the call
  through `openaiclient`, budget reserved up front and settled, breaker,
  moderation. Registered in `modules/all-modules.go`.
- Admin: `bauble status` (what names baubles, today's calls and tokens,
  breaker); `bauble spawn [tier]` now runs the same delivery path as search.
- Worst-case wait for a find: `TimeoutSeconds` (15 s by default; retries off)
  plus moderation, hard-capped at 30 s by `baubles.MaxGenerateTime`.
- A find in flight when the server stops or copyovers is lost (nothing was
  written yet); logged, harmless.
- Exit: `go test ./internal/baubles/... ./internal/actions/...
  ./modules/baubles/... ./internal/configs/...`; in game with no key,
  `bauble spawn` delivers a "Trinket" after about 3 s; with a key,
  `bauble status` reports openai and `bauble spawn rare` in a themed room
  delivers a fitting named object.

### Phase 5: Admin tools, and naming baubles in commands (written)

- `bauble status | stats | list | show | prompt | spawn | edit | regen |
  retire | restore | window` (`internal/usercommands/admin.bauble.go`, help
  in `admincommands/help/command.bauble.template`). Every subcommand taking a
  record accepts an id or the name of a bauble in the admin's pack or on the
  floor.
- `edit <bauble> <field> <text>`: name, keyword, desc, material, value,
  weight, tier. Checked like a model reply (`CleanReply`), value kept in the
  tier, `EditedBy` recorded. The field word splits target from text, so
  multi-word targets work.
- `regen`: asks the model again in the background, avoiding the current
  name; keeps provenance and theft fields; refuses to replace a model name
  with a generic trinket when the call fails.
- `retire` / `restore`: withdraw or bring back a record's text; value kept.
- `prompt`: the exact messages that would name the record now (the module
  installs the renderer even without a key).
- `stats`: status, tier and namer counts, unsold value, sales for 24 h and
  7 days, tokens spent, top regions.
- **Owner request: players must be able to name any bauble.** Names are the
  model's, so `items.NameMatch` gains word matching for baubles: any word of
  the name, or several in order, is a full match (`get doll`, `look childs
  doll`, `appraise child doll` for "Small Child's Doll"); possessives and
  hyphens are handled; leading `a`/`the` dropped. Every command resolves
  items through `FindMatchIn`, so get, drop, look, appraise, sell, give and
  `2.doll` all agree.
- `look` gains a floor-item branch (last, so it never shadows exits, nouns,
  creatures or corpses), for a bauble left on the ground.
- Bug found and fixed: the floor listing grouped items by ItemId, so several
  baubles on the ground showed as one name "(x3)". It now keys on the
  bauble id too.
- Player help `item-names` explains how to refer to found trinkets.
- Exit: `go test ./internal/items/... ./internal/baubles/...
  ./internal/actions/... ./internal/usercommands/...`; in game,
  `bauble spawn`, then `get doll` / `look at doll` / `appraise doll` on
  whatever arrives, and `bauble edit`, `retire`, `restore` on it.

### Phase 5b: Location odds, skill, and training (written)

Owner rulings (2026-09-26): finds train search; the chance grows with search
skill; the chance depends on the place (buildings about 5%, streets about 2%,
wilderness 0.25%), set for every biome in the world.

- **Chance by biome** (`Balance.BaubleBiomeChancePct`, a table in
  `config.yaml`; Go defaults in `config.balance.baubles.go`, pinned together
  by `TestBaubleShippedConfigMatchesDefaults`). Percent per roll, with the
  number of dogmud rooms in each biome at the time of writing:

  | Biome | % | Rooms | Why |
  |---|---|---|---|
  | interior | 5 | 257 | buildings people use (owner's anchor) |
  | fort | 4 | 8 | fortified dwellings |
  | ruins | 3.5 | 14 | abandoned buildings |
  | sewer | 3 | 23 | what washes down the drains |
  | city_backstreet | 2.5 | 105 | alleys and yards |
  | city_thoroughfare | 2 | 98 | main streets (owner's anchor), busy and swept |
  | dungeon | 2 | 44 | built places underground |
  | road, shore | 1 | 5, 14 | travellers drop things; things wash up |
  | farmland, cave | 0.75 | 99, 77 | homesteads; someone sheltered here |
  | land, river | 0.5 | 220, 8 | open ground (steppe, valleys, some road zones) |
  | forest, dense_forest, plains, swamp, cliffs, mountains, desert, snow, spiderweb | 0.25 | 239 | wilderness (owner's anchor) |
  | water, ether | 0 | 95 | deep water; not a place in the world |

  A biome not in the table (or a room with none) uses
  `BaubleSearchChancePct`, now 1% and no longer the flat chance for
  everywhere. A table given in config is used as given, so an operator can
  list only some biomes. A listed 0 never finds, and a room at 0 spends no
  roll and opens no window.
- **Search skill** (`Balance.BaubleSkillMaxBonus`, default 1.0): chance =
  biome chance x (1 + bonus x skill factor), where the skill factor is the
  square root of search rank over `SkillSoftCap` (the same curve as
  `combat.SkillMultiplier`, so early ranks count most). A searcher at the
  soft cap doubles every chance: 10% indoors, 0.5% in the wild. Scaling is
  proportional, so skill never makes the wilderness as rich as a town.
- **Finds train search.** `actions.awardSearch` pays the search's one award
  as a win when a bauble is found, even in a room with no contest. A roll
  that finds nothing pays nothing: every room offers rolls, so counting a
  miss would make every room the secret-exit progression farm. Rate: at most
  2 rolls per room per hour at 0.25 to 10%, too rare to farm.
- Rolls are cut to one part in a million so a skilled wilderness chance
  (0.375%, say) is rolled as written.
- Admin: `bauble status` lists the biome table; `bauble window` shows the
  chance where the admin stands, base and with their own search skill.
- Tests: the actions test binary's default roll never finds (so tests about
  hidden exits cannot flake on a 1% find); the bauble tests stub it.
- Exit: `go test ./internal/configs/... ./internal/baubles/...
  ./internal/actions/...`; in game, `bauble window` in a house, a street
  and a forest shows about 5%, 2% and 0.25% base.

### Phase 5c: Targeted search of room features (written)

Owner ruling (2026-09-26): `search` searches the room; `search XXYY` checks
whether XXYY is a feature of the room and, if so, searches that feature.
Feature searches do not count towards the room's 2 rolls.

- **What a feature is** (`actions.FindSearchFeature`): a room noun (the
  same lookup `look <noun>` uses, aliases and plurals included), a hidden noun
  the player has already discovered, or a container they can see, in that
  order. Leading articles and prepositions are dropped, so `search under the
  table` and `search in the barrel` work.
- **Anti-oracle**: an undiscovered hidden noun or container never matches,
  and gets the same "You see no such thing here to search." as nonsense.
  Naming nothing spends no cooldown.
- **What it rolls**: one bauble roll, never from the room's window. It
  shares the `search` cooldown. (Phase 5d: each feature can be searched only
  once per `BaubleFeatureWindowMinutes`.)
  It rolls no contested tier (secret exits, hidden nouns, stashes, hiders
  stay the room search's), since otherwise a feature search would be the
  room search with free extra rolls.
- **Rate**: rooms typically have 2 to 4 features (the richest, Stillwater
  4103, has 11), so a room and its features offer about 4 to 6 rolls an hour
  instead of 2. If feature finds prove too generous, lengthen
  `BaubleFeatureWindowMinutes` or lower the biome table.
- **Prompt** (`PromptVersion` 2): the request names what was searched and
  sends its authored description (`searched_description`, capped at 400
  bytes), so a find in the hearth suits the hearth. Still authored text only.
- **Progression**: a find is a won search, a miss pays nothing (same rule as
  the room's bauble roll).
- **Quests**: the `search` command notification still fires for every form
  of search (quests 49 and 58 in room 5343).
- Admin: `bauble window` lists every feature of the room (hidden ones
  included) with its own window; `bauble window reset` reopens them all.
- Exit: `go test ./internal/baubles/... ./internal/actions/...`; in game,
  `search hearth` twice within the hour rolls once, and `search` in the same
  room still has its 2 rolls.

### Phase 5d: Households, spots, once an hour, and untaken finds (written)

Owner rulings (2026-09-26):

1. **A feature can be searched once every 60 minutes.**
   `Balance.BaubleFeatureWindowMinutes` (60) replaces the Phase 5c rolls
   knob. The search itself claims the feature (`baubles.ClaimFeatureSearch`),
   whatever it turns up, even in a room that offers no baubles. A repeat
   inside the hour is refused ("You have already searched the bookshelf
   closely..." or, when another player searched it, "The bookshelf has been
   searched through recently...") and costs no cooldown. Shared by the room
   unless `BaubleWindowPerPlayer`.
2. **Indoors with an NPC about, a find stays in the room.** The room's biome
   must be `Indoor` (interior, fort, dungeon, sewer, cave, spiderweb, ether)
   and a resident present when the find is delivered: a person (the player
   species, a shopkeeper, or a faction member), awake, able to see, not a
   monster that attacks on sight, not a companion. The find is left on the
   feature searched, belongs to that household (`Item.BaubleHousehold`), and
   the finder is told so, naming the resident.
3. **Taking it is stealing.** `get <name>` of a household's bauble
   (`actions.TakeHouseholdBauble`): nobody about, taken (still marked
   stolen); a resident about and the thief not sneaking, caught at once;
   sneaking, an opposed skullduggery contest against the most watchful
   resident, the same terms as stealing from a container (hidden bonus
   included). Caught uses the steal system's consequences (`thiefCaught`,
   shared with `steal <npc>`): revealed, crime against the resident's
   factions with reputation, bounty and witnesses, and the resident attacks
   unless it cannot be fought. The bauble stays. The record keeps who stole
   it, from which room and under whose eye. `get all` never takes one by
   accident.
4. **Found on a feature, shown on it**: "Child's Small Doll (on the
   bookshelf)" in the ground listing and "You look at the Child's Small Doll
   on the bookshelf:" on `look`; "beside the chest" for a container, since
   the find lies in the room, not inside it.
5. **Untaken finds vanish after 24 hours** (`Balance.BaubleUntakenHours`).
   Any find left lying (a household's, too heavy to carry, finder gone)
   carries the time it was left; the room sweeps it on its round tick and
   before showing its floor to a visitor, and the record gets `VanishedAt`.
   Carrying a bauble clears all of this (`Character.StoreItem`), so a bauble
   a player owns and drops is never swept.

Choices made without a ruling (easy to change):

- A thief not sneaking is caught with no roll at all. `get all` skips
  household baubles so that is never an accident.
- No skullduggery rank is required to try (stealing from an NPC needs rank
  2); an untrained thief simply loses most contests.
- The household check happens when the find is delivered, a few seconds
  after the search, in the room where it was found.

Exit: `go test ./internal/items/... ./internal/rooms/... ./internal/baubles/...
./internal/actions/...`; in game, `search <feature>` in a shop with its
keeper present leaves the find on the feature, `get` it in plain sight gets
you caught, `sneak` then `get` rolls, and `search <feature>` again within
the hour is refused.

### Phase 5e: Natural finds in the wild (written)

Owner idea (2026-09-26): where the room description shows only nature, a
find can be a natural object.

- `PromptVersion` 3 tells the model: when the place shows only nature and no
  sign of people, their homes or roads, the find is usually a natural
  curiosity a collector, trader or jeweller would pay a little for, suited to
  the terrain (a quartz crystal or geode in a cave, a rough gem in rocky
  ground, a fossil in a cliff, an agate from a riverbed, a shell on a shore,
  amber in old woods, an antler, tooth or piece of bone in forest or steppe).
  Rarer tiers are finer (a clear crystal, a real gemstone). Never food, herbs,
  ore, hide, timber or other crafting material. Where people have been (a
  camp, ruins, a track) a lost personal item is still likely.
- The model judges it from the room text and terrain already sent; no new
  data leaves the game.
- Keywords: general natural words collide with real items (the Reckoning
  Bone from quest 58, the weighted stones, crafting materials such as the
  Stillwater Black Pearl and Old White's Fang), so bone, stone, shell, tooth,
  geode, amber, pearl, claw, fang and crystal join the reserved keywords, and
  the prompt asks for a specific noun (quartz, agate, fossil, antler,
  jawbone). A name may still contain the word ("Weathered Fox Jawbone").
- Cost: the system prompt grows by about 150 tokens per call.
- With no API key finds stay generic trinkets everywhere, as before.

### Phase 5f: Owner review (written)

The repo owner's seven review points (2026-09-26), and the rulings taken on
them:

1. **One mechanism, one budget.** `internal/openaiclient` is replaced by
   `internal/apiframework`, which the AI companion now uses too (it was this
   fork's own addition, so no separate package is kept for it). One wire
   format (`Chat`, `DecodeChat`, `Charged`), one transport (`Post`, which
   refuses a request classed as carrying player data unless the caller's
   consent door, `Admit`, passes it), one server key in a new top-level
   `APIFramework:` config section (every setting it leaves empty is read
   from the companion's old place and defaults, so a server's un-updated
   config.yaml behaves exactly as before, key from `OPENAI_API_KEY`
   included), one
   daily token budget with a per-feature breakdown
   (`ConsumerCompanion`, `ConsumerBaubles`), kept on disk in
   `<DataFiles>/apiframework/budget.yaml` and seeded once from the
   companion's old saved total, and one breaker for the server key. The
   companion keeps its consent ledger, per-owner and passer-by caps, per-owner
   relay breakers and relay; every one of its tests passes on the framework.
2. **Quests use `search <noun>`.** A feature search is now a FULL room search
   (every contested tier) plus the feature's own bauble roll. The hourly limit
   is on that bauble roll only: nothing is refused, and a repeat takes the
   room's roll instead.
3. **Name matching.** A bauble's word match is only partial; a bauble is a
   full match only for its exact name or `bauble`/`trinket`. With no `N.`
   given, `FindMatchIn` prefers a real item to a bauble among partial and
   contains matches (`get shield` takes the shield, not "Shield-Maiden's
   Brooch").
4. **Docker.** `provisioning/Dockerfile.dockerignore` and `.gitignore` leave
   out the living catalog (`_datafiles/**/baubles/*`, keeping `.gitkeep`) and
   the budget (`_datafiles/**/apiframework`), like the other living state.
   Keep both directories on the server across deploys.
5. **Off by default.** `Balance.BaublesEnabled: false` (no finds) and
   `Modules.baubles.Enabled: false` (no naming). Switched on, a find is named
   on the finder's OWN key when they tick "Also name things I find while
   searching" on the web client's Companion key page (the relay is lent for
   `PurposeFinds` only, and `relay.js` accepts the bauble schema only from a
   key with the box ticked; `UsePlayerKeys`), else on the server key, else it
   is a generic trinket. Nothing of the player's is ever in the request.
6. **A test printed a key.** Fixed, and guarded: `Endpoint` redacts its key
   in every print verb, the three packages' tests clear `OPENAI_API_KEY`
   and fix the server settings, and
   `internal/apiframework/key_guard_test.go` reads the test files of the
   framework, the companion and the baubles module and fails any test that
   could print a key (with a probe proving it catches the original line).
7. **`get all` in a home.** Taking a household's bauble moved to `steal`,
   with the steal checks (skullduggery rank 2, the cooldown, the container
   theft's observer contest, shared as `stealObserverPass`). `get` refuses it
   and `get all` skips it, each saying "steal <word>", so no pickup can start
   a crime by accident.

Guards touched: the combat contest-site allowlist key moved from
`stealFromContainer` to the shared `stealObserverPass`; the progression seam
guard's sale site is one helper, `saleProgression`, used by `sell.go` and
`sell_bauble.go`; four new narration sites are registered in
`narrationViewpointRegistry` (all verdictCorrect).

Regression review of the move (2026-09-27, an independent reviewer
comparing the companion before and after). Found and fixed:

- A config.yaml from before the section (a server's, never updated by a
  patch) lost the key: the companion always defaulted `APIKeyEnv` to
  `OPENAI_API_KEY` and the shipped file never named one. `Server` now reads
  every setting the section leaves empty from `Modules.aicompanion`, then
  the companion's old defaults, with the environment variable winning as
  before; a custom `BaseURL`, `AllowCustomEndpoint`, `DailyTokenBudget` (0
  there is still no cap) and the breaker are honoured the same way.
- `aicompanion status` says "unlimited" again for no cap; `aicompanion
  models` shows both breakers (server key, and a player's own key).
- Bauble outcomes on a finder's key are counted against that key's breaker
  (also their companion's) by the companion's own rule; an unusable bauble
  reply is not counted at all.
- The old budget file still records the companion's share of the day, so a
  rollback to the code before the framework does not hand out a fresh day.
- `Server` reads only the companion's config block, not every module's
  (about 8 times cheaper; it runs under the mud lock).
- Test isolation: with the budget and breaker shared, a call one companion
  test left in flight could settle into the next test's figures (a flaky
  failure under `-shuffle` and load). `apiframework.Books` lets a caller hold
  its own set; on a server the companion uses the shared one, and in its
  tests every module gets its own.

A second pass by the same reviewer found two more, also fixed:

- The shipped `APIFramework` values were filled in, so a server taking the
  new `config.yaml` while keeping its settings under `Modules.aicompanion`
  (or in `config-overrides.yaml`) would silently lose them. They ship empty;
  the effective defaults are unchanged.
- The bauble goroutine read the config off the game loop, where
  `configs.SetVal` writes it without a lock (a possible fatal map race).
  `Server()` is now a snapshot, refreshed on the game loop
  (`RefreshServer`: at load, every round, in admin views).
- Also: an old breaker of 0 keeps its old floors (1 error, 5 seconds), and
  `bauble status` shows "unlimited" for no cap.

Unchanged by design: one budget for the server key, shared with baubles;
a bauble request takes one of the relay page's slots (2 at once, 30 a
minute) in the finder's browser.

### Phase 5g: Second outside analysis (written)

An analysis of the patch (2026-09-27) raised thirteen points; the owner
ruled that baubles may spend the companion's share of the one budget (its
item 4, left as is) and asked for the rest fixed:

- **Breakers in two levels** (`apiframework` breaker.go). The provider
  breaker, shared, counts only failures that say the provider or key is
  unwell (`ProviderFailure`: no answer, timeout, 401, 403, 408, 429, 5xx;
  `DecodeChat` now returns a `StatusError`). Each feature has its own
  breaker for everything else. A bauble model the provider refuses (400,
  404) pauses baubles, never the companion.
- **One outcome per logical call.** A bauble find records once after its
  retry, not per attempt; the companion keeps one ticket across its retry
  and tool rounds.
- **Half-open, one probe.** After a cooldown exactly one caller is let
  through (`Allow` returns a probing `Ticket`); its success closes the
  breaker, its failure reopens it; `Release` or a 60-second expiry frees a
  probe that never reports. Leave is taken immediately before a request
  leaves; readiness checks only peek (`Blocked`).
- **The player's relay breaker per purpose.** Finds on a player's key feed a
  finds breaker only; the companion's is never touched
  (`relayTable.findsResult`).
- **AllowCustomEndpoint** is a string, so an explicit "false" in
  `APIFramework` overrides an old `Modules.aicompanion` true.
- **Moderation** of a player-key find (hardening, 2026-09-28/29): moderated
  whenever the server can (a flag or a failed check refuses on either key,
  and every check feeds the moderation breaker, not the naming breaker);
  when it cannot (no server key, moderation off, a breaker open), the find
  is FINDER-ONLY: its finder reads it, everyone else sees a plain Trinket.
  Player-key text must also pass `baubles.CheckPlayerKeyText` after curly
  quotes and dashes are folded, and text that does not goes to the
  server's key without counting against the player's.
- **The companion settles the ledger's own hold** (`reserveFor`,
  `settleHeld`), not one rebuilt from its own day, which could differ from
  the ledger's around the UTC midnight.
- **Catalog durability.** A new record whose write fails is not created and
  no item is handed out (the find crumbles away).
- **Copyover and shutdown** finish every find still on its way
  (`actions.FlushBaubleDeliveries`, under the lock, before saving): named if
  the naming came back, else a generic trinket.
- **Nowhere to put it**: a finder offline from a room that cannot be loaded
  gets nothing minted, so no ghost record.
- **`get all <name>`** skips this household's baubles (said once each) and
  takes the rest, instead of stopping at the first.
- **Keywords** are kept off every loaded item's keyword and head noun
  (`items.AuthoredKeyword`), falling back to the name's own last word, then
  "trinket"; `FindMatchIn` prefers a real item on full matches too.
- **Shards** start at id 1 (shard 0 is B0000001 to B0000500); a catalog in
  the old layout is read with each record's own shard winning and rewritten
  into place at load.

Each fix has a test, and each test was checked to fail with the fix
reverted.

An independent review of those fixes found more, also fixed and tested:
the keyword check read the live items map off the lock (a possible fatal
map race; now a snapshot rebuilt by every writer); a 403 that only refuses
one model counted as the provider failing (now `ModelRefusal`); a slow
probe could be joined by a second (probe lifetime five minutes, and every
companion goroutine releases its ticket last, a panic included); a bauble
timeout was released unjudged (now it counts; only a cancel is released);
unusable bauble replies were never counted (now the outcome is reported
after parsing and checking); a find could slip past the copyover flush
before its goroutine started (now tracked first); the lost-shard id bump
was not saved (now the meta file is written at load); a held-back call
showed as an error in `aicompanion status`; and about one bauble keyword in
four fell to "trinket" (now the name's other words are tried first). A
second pass found that fallback could pick an adjective a real item uses
("silver" taking `get silver` from a Silver Dagger), so every word of every
loaded item's name is now protected, not only keywords and head nouns.

### Phase 6a: Pickpocketing (written)

Owner request (2026-09-28). A player's `steal <npc>`:

- The contest is rolled at once; the player sees only "You attempt to pick
  X's pocket...". The outcome is revealed after a pause of
  `StealPocketSeconds` (3) at Dexterity 100, scaled by 100/Dexterity (the
  speed stat: quicker hands are faster), between `StealPocketMinSeconds`
  (1.5) and `StealPocketMaxSeconds` (6).
- On a success the mark's own bauble is taken; a mark carrying none turns
  one up `BaublePickpocketChancePct` (50) of the time, and its naming is
  asked for at that moment, so the pause pays for the model call. A naming
  still out when the pause ends is waited for up to
  `BaublePickpocketGraceSecs` (5), then given up (a generic trinket).
- The bauble is handed over with the rest of the loot, in the ordinary
  success line ("You successfully steal 12 gold and a Tarnished Brass
  Thimble from ..."), and recorded as stolen from the mark.
- Pocket-sized: the prompt says so (`taken_from`, `size_rule`,
  `PromptVersion` 4), the weight is clamped to `BaublePickpocketMaxWeight`
  (1 lb), and a naming the model itself weighed at over twice that is
  refused (`baubles.TooBigFor`).
- The thief leaving, going offline or fighting, or the mark going, loses
  the chance: nothing taken, nothing caught; a bauble already named stays
  in the mark's pocket for the next attempt. Never in a companion's or
  former companion's pocket (its name may be a player's). Copyover and
  shutdown reveal every pause at once (`FlushPocketAttempts`). A mob's
  steal is at once, as before.
- From the independent review: skullduggery is awarded at the reveal, not
  the roll (a skill-up line would give the roll away, and a thief who
  walked off to dodge being caught would still have trained); one
  pickpocket at a time per thief; the reveal also checks the thief is not
  under attack and tells them when the mark has gone; a naming weighed over the pocket
  limit, or naming a thing no pocket holds (an urn, a candlestick), is
  refused, not clamped.
- Economy note: pickpockets have no per-room or per-NPC ration, unlike
  search. At the shipped 60-second steal cooldown and a 50% chance per
  success on a mark without a bauble, a practised thief can turn up
  roughly one bauble a minute at most (fewer with failures), each a model
  call. `BaublePickpocketChancePct` and `StealCooldown` are the dials.
- Key order, by owner decision after review: a pickpocket's bauble is named
  like every find, the thief's own key first (when they allowed finds on
  the key page), then the server's key, else a generic trinket. The naming
  starts at the moment of success, so a thief reading their browser's
  network traffic can learn the outcome before the reveal; accepted. An
  audit of every model call in the patch found the rest follows that order:
  the companion (`route`), search and household finds, and natural finds.
  Kept as deliberate exceptions: moderation of a find named on a player's
  key uses the server's key (free, protects shared world text); the
  companion's model list probe uses the server's key; an admin's `bauble
  regenerate` uses the server's key.

### Phase 6b: Stolen finds are worth more (written, after PR #175)

Owner ruling (2026-09-28): a find that has to be stolen should more often
be valuable: a household's find (indoors with an NPC about) more than one
picked up, and a pickpocketed bauble more than one found on the ground.

- Two more sets of tier weights in the BAUBLES block, with Go defaults and
  the same validation as the search set (`configs.validateTierWeights`: a
  negative takes its default, all zero takes all three, a single zero
  switches a tier off):

  | Set | Cheap | Average | Rare | Expected gold* |
  |---|---|---|---|---|
  | search (`BaubleTierWeight*`, unchanged) | 70 | 25 | 5 | about 12 |
  | household (`BaubleHouseholdTierWeight*`) | 45 | 40 | 15 | about 25 |
  | pickpocket (`BaublePickpocketTierWeight*`) | 50 | 40 | 10 | about 19 |

  *At the middle of each tier's range on the default ladder (3.5, 12.5,
  120 gold); rare finds carry most of the value.
- Household finds lean richest because they are rationed (a room's two
  rolls an hour, a feature once an hour) and still have to be stolen past
  the household. Pickpocketing can repeat about once a minute (the steal
  cooldown), so it is lifted less.
- **When a find is a household's.** Its tier is chosen when the roll
  succeeds, before naming starts, so the household check now also happens
  at the search (`actions.householdFind`, the same `HouseholdResident` test)
  and is passed to the roll (`baubles.FindOpts.Household`). A find rolled
  as a household's stays the household's at delivery even if nobody of the
  household is about by then (`BaubleDelivery.Household`): the finder is
  told "It belongs to this household, so you leave it where it lies.", and
  taking it is still `steal` (uncontested with nobody watching). So a
  richer find always has to be stolen. A find rolled with nobody about that
  finds a resident at delivery is the household's as before, at the search
  weights.
- A mob's own carried bauble keeps whatever tier it was made at.
- Tests: `TestStolenFindsUseTheirOwnWeights`,
  `TestSearch_HouseholdFindsRollAsTheHouseholds`,
  `TestSearch_AHouseholdsFindStaysTheirsWhenTheyStepOut`,
  `TestPickpocketBaubleTakesThePickpocketWeights`,
  `TestBaubleStolenTierWeightsAreValidatedApart`,
  `TestBaubleStolenFindsLeanRicher`; each checked to fail with its piece of
  the change reverted.

### Phase 6c: Stolen goods, fences and owners (written, after PR #175)

Owner rulings (2026-09-28): fences buy stolen baubles at 60% of value so
thieves seek them out; heat lasts three real days (first a week), and only
in the area the bauble was stolen in, so a thief can sell it to an honest
merchant in another town; banks will not take a hot bauble; the NPC robbed may recognise its bauble on the thief; a thief who
returns what they took earns back reputation, three returns equal to one
catch.

- **Heat.** A stolen bauble is hot for `BaubleStolenHeatHours` (72) after
  its latest theft, unless given back since (`baubles.Record.Hot`). Stolen
  again, hot again. For selling, storing and listing it is hot only in the
  heat area of the theft (`Record.HotIn`): the theft's zone, recorded as
  `StolenZone` (pickpocket and household thefts pass the room's zone), or
  the group of zones in `BaubleHeatAreas` that zone belongs to. New
  Plymouth's eight inner zones ship as one area ("New Plymouth"); its
  outskirts, and every other town (one zone each), are their own. A record
  whose theft zone is unknown is hot everywhere while hot. Merchants judge
  by the zone of the room they trade in, storage by the storage room's,
  the auction house by the lister's room. The owner's recognition follows
  the owner, not the area: it is time-only (`Hot`).
- **Honest merchants** refuse a hot bauble ("That was stolen, and not long
  ago. I won't touch it. Try someone less particular..."), and buy a
  cooled one at the honest price (`ShopBuyRatio`, 50%). Before this phase a
  stolen bauble sold anywhere at full rate, which the richer stolen-find
  odds of Phase 6b made worse.
- **Fences.** A merchant in one of `BaubleFenceGroups` (default `fence`;
  Fence Dealer Siv in Thornwall, mob 104, now carries the group) buys every
  bauble: any stolen one, hot or cold, at `BaubleFenceBuyPct` (60%) of its
  value, rounded up, better than an honest merchant pays for anything; an
  honest one at the honest price. The owner asked for all merchants to
  refuse while hot rather than only the victim's faction, so fences are
  the one outlet.
- **Banks.** The bank itself holds only gold; items go into `storage` (the
  Thornwall Bank's vault is a storage room). Every `storage add` path
  refuses a hot bauble. The auction house refuses one too, since listing it
  would launder what merchants refuse (`auctionRefusesStolen`).
- **Recognition.** On any move into a room (a `RoomChange` listener), an
  owner in the room may recognise its hot bauble on a player: the mob
  template it was lifted from, or for a household's bauble taken with
  nobody watching, any of that household at home. Awake, alive, not a
  companion. A contest: the owner's noticing score (`stealVictimScore`,
  with their own sight) against the carrier's steal score (Dexterity,
  skullduggery at SkillWeight, the hidden bonus). Recognised, the owner
  says so and it is a catch in the act (`thiefCaught`: crime, reputation,
  bounty, the owner attacks unless it cannot fight). Once per theft.
  Only the thief is ever accused: anyone else carrying it is left alone, so
  a thief can neither frame a bystander nor spend the recognition on a
  friend. The owner is the mob TEMPLATE (instance ids do not survive a
  respawn), so any instance of a common template recognises it.
- **Returns.** `give <bauble> <owner>` returns it: it cools, and the owner
  is glad (the existing gift-to-opinion seeder also fires). Given by its
  thief, each of the owner's factions credits a share of the catch: the
  catch is `CrimeRepDeltaTheft` (default -5, applied per faction by
  `thiefCaught`) split into `BaubleReturnsPerCatch` (3, at least 2) parts,
  the remainder spread so every three returns total exactly one catch (1,
  2, 2). No single return is worth a catch. A return earns back only
  reputation actually lost: a faction credits at most three returns for
  every OPEN theft its crimes log holds against the thief, so a thief who
  is never caught gains nothing, stealing just to return cannot farm
  reputation, and a sentence served (which resolves the crimes and
  restores reputation) leaves nothing more to earn back. Known edge: a
  catch at the reputation floor costs nothing (the bump is clamped) yet
  counts until resolved. A bauble earns credit once ever. A returned bauble is no
  longer stolen goods: a fence pays the honest price for it.
- **Walking in to return it** can get the thief recognised first (the
  owner looks on every entry); the help says so. Sneaking in helps.
- New files: `internal/actions/stolen_bauble.go`,
  `internal/actions/stolen_bauble_test.go`, `internal/baubles/theft_test.go`,
  `internal/hooks/RoomChange_StolenBaubleRecognition.go`,
  `internal/hooks/RoomChange_StolenBaubleRecognition_test.go`,
  `internal/usercommands/stolen_bauble_test.go`.
- Guards: `stolenRecognitionRoll` is on the contest-site allowlist, and
  pays the owner's sight ramp inside itself (the sight-penalty guard).
- An independent review found, and this phase fixed: returns paying a thief
  who was never caught; recognition of a non-thief carrier (framing, and
  spending the recognition on an alt); any passer-by in a house counting as
  the household; the fence premium outliving a return; a returns-per-catch
  of 1 making a return equal a catch; `storage add` stopping at a hot match
  instead of storing a cool one of the same name (and saying two
  contradictory things); a trinket-shy shop giving the stolen-goods line;
  and a cold stolen bauble going to an honest merchant ahead of a fence in
  the same room.
- A second review pass found, and this phase fixed: `storage add @handle`
  (an explicit item pick bypasses the name filter) storing a hot bauble;
  returns earning back reputation a served sentence had already restored.
- Left as they are (noted): a hot bauble can still be dropped, stashed or
  mailed to wait out its three days; heat follows the record wherever it
  goes. Since only
  the thief is ever accused, a friend can carry it past the owner safely;
  it stays hot, so merchants, storage and the auction house still refuse
  it.
- Every test was checked to fail with its piece of the change reverted,
  except the one line in the `auction` command that calls
  `auctionRefusesStolen` (the command runs through an interactive prompt;
  the check itself is tested).
- **Fence roster** (owner request: every town has a fence in it or near
  it). Each got the `fence` group and one idle line hinting at the trade:

  | Town | Fence | Where | Shop |
  |---|---|---|---|
  | Thornwall City | Fence Dealer Siv (104) | in the city | yes |
  | New Plymouth | Ysolde (9323) | Common, in the city | yes |
  | New Plymouth, Kilnreach Works | Mother Coyle (9213), A Market Hawker (9209), A River-Road Smuggler (9215) | the Outskirts, next to both | yes |
  | Stillwater | Sly Tam (9172) | North Road North, next door | yes |
  | Hartcharn | Wick Orrel (9185), of the Tap and Trough | in town | yes |
  | The Confluence, Greenford | Varro the Importer (9428) | the Confluence; Greenford two zones away | yes |
  | Ashwick, Watchers Crossing | Peddler Malk (250) | Marches Spur Road, next to both | yes |
  | Pothole Coulee | Thornwall City's fences | two zones away | |

  The Tri-Rivers towns (Greenford, the Confluence) had no shady character
  at all; Varro, an importer on the quay, was the closest fit.
- **Every fence keeps a shop** (owner ruling 14, 2026-09-28, replacing the
  first cut's stash-paying go-betweens). Ysolde, the Smuggler, Tam and Malk
  each have a `shop:` block (lockpicks, a disarm kit, torches) and pay from
  persisted shop gold with the normal restock. They carry no
  `craft_support` (owner ruling, 2026-09-29): with `general` they would buy
  any vendor loot (`shops.vendorAcceptsAny`) and drain the gold a fence
  needs for baubles. `shops.ValidateShopMobTags` lets a fence omit it
  (`mobs.Mob.IsFence`). Fences that were traders before (Siv, Mother Coyle,
  the Hawker, Wick Orrel, Varro) keep the `craft_support` they had. All are `non_combatant: true`, so they cannot be
  attacked or robbed. Shop gold: Malk 2000; Ysolde, the Smuggler and Tam
  1000. One departure from the ruling, for a reason found in the world
  files:
  - Tam's `behavior_archetype` goes from `thief` to `noncombat_shopkeeper`:
    a thief archetype picks players' pockets, and a non-combatant one could
    do it with no answer (no attack, no stealing back).

  Torvan Cresk (249) is not a fence (owner ruling, 2026-09-29): quest 14
  (The Undertow) has players fight him for the strongbox key, so he stays
  as he was before PR #175. Thornwall City is covered by Siv.

  `TestEveryTownHasAFenceNearby` checks every fence has a shop and is
  `non_combatant`, that the four fence-only shops (`fenceOnlyShops`) have
  no `craft_support` and the other fences do, and that Torvan is not a
  fence. `TestStolenBauble_AFenceShopRefusesOrdinaryLootButBuysBaubles`
  checks such a shop refuses an ordinary item it stocks and still buys a
  bauble.
  Resale of bought baubles is slice D: Phase 6f.

### Phase 6d: The owner's fix round on PR #175 (written)

The owner's review list, less items 2, 4, 6 and 8 (the owner's to do).

- **Lint (1).** The two `TransitionToRevealing` results in `steal.go` are
  discarded with `_ =`.
- **Catalog pruning and locking (3).** Superseded by the catalog sweep
  (`internal/baubles/sweep.go`), now the only pruner: `Load` and `SaveAll`
  prune nothing. The sweep runs at boot and then every
  `Balance.BaubleSweepHours`, collects every bauble id any item still
  points at (the live world and every save file under DataFiles), and
  prunes a record only after two sweeps in a row found nothing pointing at
  it AND its latest evidence is older than `KeepDuration()`
  (`Balance.BaubleCatalogKeepDays`, 30, at least 7 since the sales stats
  read a week). A record with a return credit (`ReturnCreditAt`) is never
  pruned, since the credit window still counts it. A sweep that goes wrong
  anywhere applies nothing. It rewrites only the `catalog-*` shards, so
  the corpus overlay survives. Shard and meta writes happen outside the lock
  `Get` takes (a separate write mutex orders them), so a read never waits
  on the disk. `ReturnCredits` reads a per-user index instead of scanning
  the catalog.
- **Matching (5).** `items.FindMatchIn` ranks candidates by match strength
  (exact name, whole words, word start, substring); a real item beats a
  bauble only on an equal or stronger match, so `button` finds the
  Tarnished Copper Button over the Buttoned Leather Vest. Corrected
  2026-09-29 (owner ruling): the ranking no longer re-orders real items
  against each other. The real items are chosen among themselves by the
  old list-order rule, as if no bauble were there, and only that choice is
  weighed against the best bauble (`findMatchWithBaubles`). A household's
  bauble never beats a real item on a partial match (`candle` finds a
  Candlestick, not a household Stub of Candle).
- **Search skill (7).** `BaubleSkillFactor` reuses
  `combat.SkillMultiplier`, rescaled to 0..1.
- **Recognition sight (9).** An owner who cannot make out shapes
  (`messaging.CanSeeShapes`: blind, or a dark room) recognises nothing; one
  who sees only shapes recognises the bauble but names nobody ("a
  figure"), following the witnessing tiers.
- **Return window (10).** A return earns credit only against catches still
  open, and only returns made since the oldest of them count
  (`Record.ReturnCreditRound`, compared with the crimes' rounds).
- **Best offer (11).** A bauble goes to the best offer the merchant can
  actually pay (shop gold, or purse for a legacy merchant); `sell N` picks
  the buyer again for each bauble.
- **Gifts (12).** A bauble a player gives a mob that does not own it is
  marked (`Record.GivenToMob`, `baubles.MarkGiven`); picked back out of
  that mob's pocket it is not the mob's stolen goods, so no fence premium
  and no heat. A theft clears the mark.
- **Fences are shopkeepers (13).** See the roster above (Tam is
  explained there). The stash code
  (`stashFence`, `BaubleBuyersInRoom`) is gone; bauble sales, `offer` and
  `appraise` ask the room's merchants only.

### Phase 6e: Fallback corpus (hardening slice C) (written)

Design: `docs/superpowers/specs/completed/2026-09-28-baubles-hardening-and-corpus-design.md`,
slice C. Plan: `docs/superpowers/plans/completed/2026-09-28-slice-c-bauble-corpus.md`.

- **No key no longer means "Trinket".** A find no model names takes
  hand-written text from the fallback corpus, chosen by where it was found:
  the biome and its group (dwelling, street, underground, ruins, waterside,
  wild) and the tier, `pocket` for pickpocketed finds, then the tier alone.
  A generic "Trinket" is left only for when nothing fits.
- Two layers: the tracked seed `_datafiles/world/dogmud/bauble-corpus.yaml`
  (234 entries, reviewed by the owner) and the living-state overlay
  `_datafiles/world/dogmud/baubles/corpus.promoted.yaml`, filled by
  `bauble promote <bauble>` from moderated model names made on the
  server's key and never edited by hand. Retiring, editing or regenerating
  a record takes its promoted text out again.
- Values are clamped into the tier when used; each seed pool's average sits
  near its tier's midpoint, so a corpus find pays what a generic one did.
  A known name hinting at its price is accepted (owner ruling 8).
- Admin: `bauble promote`, `bauble corpus list|remove|reload|export`. The
  logic is `baubles.Promote` and `baubles.RemoveCorpusEntry`, so the /build
  queue (web builder rework arc) can call the same functions.

### Phase 6f: Shelf resale (slice D) (written)

Design: `docs/superpowers/specs/completed/2026-09-30-baubles-shelf-resale-design.md`.
Plan: `docs/superpowers/plans/completed/2026-09-30-baubles-shelf-resale.md`.

- **Sold baubles can come back.** A player's sale of an average or rare,
  non-retired bauble to a living-economy shop puts it on the shop's
  secondhand shelf (`AffixedStock`) at its catalog value. A mob's sale, a
  legacy merchant, a cheap bauble (so a dozen value-1 trinkets cannot evict
  shelved gear) and a retired one are still destroyed.
- **Hot goods wait in the back room.** A bauble hot anywhere when shelved
  is held out of sight until `StolenAt + HeatDuration()`. A shop holds at
  most `ShopAffixedStockCap` of those and refuses more: a fence in its own
  voice, an honest shop with a plain "no room".
- **`list` and `buy`.** `list` shows a "Secondhand goods" table in shelf
  order, per viewer (a finder-only bauble reads Trinket to others). `buy`
  selects by position (`buy 2.trinket`), matches a bauble in the buyer's own
  view, and a buyback returns the record to its unsold status
  (`MarkBought`); `SalesSince` counts by `SoldAt`, so the sale still counts.
- **Cap.** `ShopAffixedStockCap` rises from 8 to 12 and gains a
  `config.yaml` key in SHOP ECONOMY; over it the entry listed earliest goes.
- **Elsewhere.** The dashboard types a fence's shop `fence`; the AI
  companion's `browse` shows the shelf in the model's view.

### Phase 7: Optional

Pre-generated pool per region, identify-on-appraise, collectors and region
quests, achievements, auctions, companion awareness, SQLite store, web admin.
(Search-rank scaling of the chance is done: Phase 5b.)

## 5. Open questions

1. Room-shared or per-player roll windows.
2. Real or game hour.
3. Tier weights (the 70/25/5 above is a placeholder).
4. Which merchants buy baubles.
5. Model and daily token cap.
6. Sold baubles destroyed (recommended) or resold as curios. Answered by
   slice D (Phase 6f): average and rare ones a player sells to a living
   shop are resold; the rest are destroyed.

## 6. Patch process

One cumulative patch, regenerated after each phase. Phases 0 to 5 were
written against the FinalTwist fork at `a381b9abe`, then rebased onto the
owner's DOGMud master of 2026-09-25 (the `DOGMud-master.zip` snapshot), and
finally onto master at `cf5af4c55` (lighting plan 5b: the steal observer
helper takes the watcher's sight ramp, and `TestStealPaysTheThiefsEyes`
puts its mark in the thief's room, which the pickpocket reveal requires).
That patch shipped as PR #175. Changes after it (Phase 6b on) go in a new
cumulative patch against the PR's head, `f57565f6c`
(`dogmud-post-pr175.patch`). Phase 6c is its own patch
(`dogmud-stolen-goods.patch`), applied on top of that one:

```text
git apply --check dogmud-baubles.patch
git apply dogmud-baubles.patch
gofmt -l internal/apiframework internal/baubles internal/items internal/usercommands internal/actions internal/configs modules/baubles *.go
go build ./...
go test ./internal/apiframework/... ./internal/baubles/... ./internal/items/... ./internal/actions/... ./internal/usercommands/... ./internal/configs/... ./modules/baubles/... ./modules/aicompanion/...
go test ./internal/rooms/... -run 'NoDogmudRoomOrZoneDefaultsToCityBiome'
go test . -run 'Durable|TempRename|Messaging|Surface|Boot'
golangci-lint run --new-from-rev=HEAD   # CI gates new findings only
```

### Rebase onto the owner's master (what changed and why)

(Historical. Phase 5f later replaced `internal/openaiclient` with
`internal/apiframework` and moved the companion onto it, consent door
intact, so the notes below on the companion being untouched and on
`UseCompanionKey` no longer hold.)

- **`modules/aicompanion` is untouched.** The owner rebuilt its transport:
  every request passes a per-player consent door (`admit`), can route through
  the owner's own key in their browser (relay, tier 2) or the server key
  (tier 3), and is billed at worst case when sent with no usage reported.
  Wrapping that in `internal/openaiclient` would have bypassed the door, so
  the Phase 0 wrapper hunks were dropped and the companion keeps its own
  code. Baubles carry no player data, so they sit outside the consent
  ledger by design; `internal/openaiclient` documents that it must never
  carry player data.
- **Lessons carried over** from the owner's transport into
  `internal/openaiclient` and `modules/baubles`: a sent request with no usage
  is billed at prompt estimate plus a whole answer (`Result.Sent`,
  `Result.Estimated`, `neverConnected`); a call already given up on is never
  sent; response bodies closed errcheck-clean; and the daily budget keeps
  in-flight reservations across the UTC rollover (`outstanding`), as the
  companion's `rollDay` does.
- **Key resolution is unchanged** in the companion (`APIKeyEnv`, then
  `APIKey`), so `UseCompanionKey` still reads `aicompanion.APIKeyEnv` and
  `aicompanion.APIKey`. That is the server's key (the companion's tier 3),
  never a player's relay key.
- **The `city` biome is gone** (split into `city_thoroughfare`,
  `city_backstreet` and others). Nothing in the patch maps biomes any more
  (the generic trinket replaced the biome word lists); the prompt sends the
  biome id with underscores as spaces.
- Everything else the patch touches (items, actions, usercommands, configs,
  shops, mobs, rooms, plugins, util, main.go, copyover.go, config.yaml,
  guard tests) either did not change or changed in unrelated code; the patch
  applies to all of it as written. `docs/README.md` changed, so its row was
  re-added by hand.

The patch never touches `_datafiles/config.yaml` until Phase 3, and then
only by a small, clearly marked block.
