# PR #208: Ideas Worth Salvaging

Survey taken 2026-10-12 of the general improvement concepts in FinalTwist's
bundled [PR #208](https://github.com/pruuk/DOGMud/pull/208), in case it is
never finished and we pick ideas out of it ourselves.

**Status: on hold.** FinalTwist has time to come back and finish #208 against
the blocking groups in [`README.md`](README.md). Nothing here is planned work
yet. If we do salvage, we expect to take a smaller portion than this table
lists; it is a menu, not a roadmap.

## Sources and scope

- Read from the PR head `3ce674451` (base `c696c117a`): its `docs/PATCH_NOTES.md`
  entries and the `context.md` of each package it adds.
- Watch-outs come from our review in this folder; finding ids link through
  [`findings-index.md`](findings-index.md).
- Checked against `origin/master` on 2026-10-12: none of the packages
  `internal/housing`, `merchantchests`, `scavenger`, `gather`, `mining`,
  `timber`, `rifts` or `lively` exist, items carry no durability or grade,
  and the loot goblin (`internal/hooks/NewRound_SpawnLootGoblin.go`) is
  still live.
- **Left out on purpose:** the API features (NPC idle lines, room ambience,
  look-detail, model-written rift rooms, the Hollow companion courtship).
  The owner prefers model-free designs.

Size is a rough guess at the porting effort: S is a small PR, M a few PRs,
L an arc of its own, XL a large arc.

## The table

| # | Concept | What it adds | Why it might be worth it | Watch-outs (from our review) | Size |
|---|---|---|---|---|---|
| 1 | **Player housing** | Lodgings in New Plymouth, the Confluence and Thornwall (each gated on city standing) and a cheap one in the wilds. Homes, room extension deeds, redecorate vouchers, guest keys, container and strongbox deeds | Reviewers rated it the best-reused system in the PR: `util.Save`, living-state loading, the existing lock system. A real gold sink | Every command in a house compares the whole house; floors have no cap; units never return to the pool (F018, F019, F020). Redecorate text is player-written | L |
| 2 | **Home furnishings** | Bed deed (rest twice as fast), crafting station deed, woodwork furniture set up with `use` | Gives housing a purpose beyond storage | The bed bonus is a flat value, not a knob (F072, F076) | S, after #1 |
| 3 | **City scavengers and floor decay** | One scavenger NPC per city (one per New Plymouth district) picks up litter, can be pickpocketed, empties daily. Outside cities, floor items slowly decay | Replaces the world-wide loot goblin sweep with something that fits the setting | Decay cannot be turned off because 0 is replaced with a default (F080, F083, F085); a dropped honest bauble becomes stolen goods (F025); quadratic idle work (F082) | M |
| 4 | **Merchant chests (theft)** | A locked chest in each shop; lock and alertness scale with stock value; a sleeping merchant notices less; goods are hot for three days, a fence buys anywhere | Good thief gameplay | Adds a second stolen-goods model, guard registry and relock clock beside the bauble steal rules (F070, F015, F016); should be rebuilt on the steal path. Companion `put` overflow, cooldown skip (B6) | M |
| 5 | **Material grades** | Crude to pristine on materials and crafted gear, changing damage, speed and protection, not only price. Shop goods count as standard | Makes crafting skill and gathering produce better gear | The foundation for #6 to #10 | M |
| 6 | **Gear wear and repair** | Crits wear weapons and armour, bows per arrow, tools per job. Broken gear is kept, not lost. Mend it yourself if you could craft it, or pay a merchant of its trade | Gold sink and a lasting reason to keep crafters | Go defaults of 0 mean no wear without the config keys (B5); broken affixed gear sells at full price; repair skips the shop sight and sleep gates (B6) | M |
| 7 | **Tool tiers and a shared gathering roll** | A stat-led gather roll; the tool multiplies the score and its tier caps the grade. Shops sell only crude tools, better ones are forged | One mechanism behind every gathering trade | Several numbers hardcoded rather than in the balance config (F052, F104, F105, F107) | M |
| 8 | **Hunting and carcass processing** | `skin`, `butcher`, `harvest <part>`; per-species part tables; rare parts gated by Perception and tool; spoilage; tanning, smoking, bowstrings, fur gear | Hunting becomes a whole way to play | Spoilage is a second clock beside item aging (F051); companion salvage pays twice (B6); bank storage bypasses spoilage (F044, F048) | L |
| 9 | **Lumberjacking and woodwork** | Regrowing tree stands, `survey trees`, wood species that change bows and arrows; carpentry renamed woodwork | Strong flavour, feeds #2 | Should share one mechanism with #10 | M |
| 10 | **Mining, prospecting and smelting** | Ore instead of finished ingots, refilling veins, gems, copper, bronze, silver and gold gear variants | Every smith input can come from players | `internal/mining` is a near copy of `internal/timber` (F057); build one resource-node mechanism for both | M |
| 11 | **Rifts (micro-dungeons)** | Daily-moving portal sites; dungeons built a room at a time from an authored room bank and torn down behind you; locked exits needing keys; a lens memory puzzle; a pursuer if you flee; a harsh death forfeit; lore fragments | The most novel idea in the PR, and it stands without the model-written rooms | Rift doors can lead into unrelated rooms (B4); a disconnect dodges the forfeit (F040); crash duplication (F095, F096); sight-gate leaks (F042, F102) | XL |
| 12 | **Companion combat archetypes** | A behaviour-tree archetype per companion (archer, guardian, and so on) and a roster of six | The archetype trees do not need the API | Tangled with the API companion work, so costly to separate | M |
| 13 | **Species corrections** | 32 creatures moved to the right species, plus new `bird` and `horse` species; hunting and alliance run on species | Cheap, data only, fixes odd hunting and ally behaviour | None noted | S |
| 14 | **Armour weights** | About half of all armour weighed nothing; now weighted by material | Cheap data fix | Balance check for starter kits | S |
| 15 | **Shop pricing fixes** | A shop never pays more than its resale justifies; repeated sales of one item pay less; shops remember recent scrap | Closes buy-and-sell-back loops | Check against `shops.PricingBaseline` | S |
| 16 | **Jobs bound to their room** | A craft, mine or chop job ends if you walk away | Small and sensible | Needs the job system from #7 to #10 | S |
| 17 | **Loose bug fixes** | Blindness not clearing when its condition expires; multi-output recipes giving one item; `steal` reaching into locked containers; `look in` on any container; punctuation in `keyword_match` | Possibly live bugs on master | Confirm each on master before porting | S each |

## If we salvage

A suggested order, cheapest and safest first:

1. The data and bug rows: #13, #14, #15, #17.
2. Housing, #1 then #2. The cleanest code in the PR.
3. Scavengers, #3, to retire the loot goblin.
4. The trades as one arc: #5 grades, #7 tools, #8 hunting, #9 and #10
   unified, #6 wear and repair, #16.
5. Merchant chests, #4, only once rebased on the existing steal model.
6. Rifts, #11, last: the largest piece with the worst open defects.

Any port is a DOGMud port of FinalTwist's idea and should credit #208.
