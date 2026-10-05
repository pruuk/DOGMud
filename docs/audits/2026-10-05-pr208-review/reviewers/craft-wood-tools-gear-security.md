# PR #208 review: craft wood tools gear, security lens

Blind reviewer `craft_wood_tools_gear:security`, run 2026-10-05 against PR head `3ce674451`. Findings were then checked by independent verifiers (three for critical and high, one otherwise). Severity is the verifiers' median. Back to the [synthesized report](../README.md).

## Reviewer summary

Security review of wilderness trades phases 4-5 (46766d13b..52cfbeda4), judged as the code stands at the PR head. I found no critical or high issues. Every exploit path I checked is closed at the head:
- Moving away from a chop no longer lets the job finish in another room: the job is tied to the room where it started (later commit 3ce674451 added SalvagingData.RoomId and JobLeftBehind).
- Two players chopping the last tree: ResolveChop reloads the stand, so only one gets it.
- Rift rooms cannot be chopped for fresh stands.
- Tool wear cannot be dodged: the wear lands on the tool that was rolled, matched by ItemId and UUID, and BestTool skips broken tools.
- Overriding a graded item does not bake the grade in twice: affixgen now builds from GetRawSpec, and enchantments start from the template or override.
- Shops never put iron-or-better tools back on the shelf. This holds on the sell, caravan, crafter and reconcile paths.
- reconcileShop cannot be used to refill stock: no code ever removes a template stock entry, so a re-seed happens only once per newly added entry.
- Crafted furniture goes through the existing owner check in housing.UseItem and the per-room container cap.
- Wood is stamped only on items whose spec says they carry wood, and recovery is capped by validation.
- Messages that name the chopper respect the observer's sight (SendTrio hides the name).
- Ungraded shop inputs cap a craft at one grade above standard (capUngradedGradable), so buying shop materials does not give a high-grade output.
Two low findings remain.

## Coverage

I read the following in full at the PR head:
- internal/actions/chop.go
- internal/usercommands/chop.go
- internal/timber/stand.go, timber.go and wood.go
- internal/items/tools.go and grade_effects.go
- internal/housing/crafted.go
- internal/gather/tools.go, including WearTool at the head
- internal/actions/gradable.go, from the later review-fix commit

From the range diff I read:
- NewRound_UserRoundTick (the chop/craft branches and the JobLeftBehind guard at the head)
- toolwear.go, craft.go (RecipeGrade/CraftWood/StampWood), harvest.go (MinTool, wear) and carcass.go
- sell.go (NeverResold) and shops persistence.go (reconcileShop), shopinventory.go, craftdecision.go and caravan visit.go
- combat_fire.go and combat_reload.go (LoadedWood, arrow recovery)
- forage.go and forage_core.go (sickle extra draws), rooms/spoilage.go, species/harvest.go
- characters/validate.go (the carpentry to woodwork migration, including nil-map safety), skills.go, items.go (GetSpec/GetRawSpec, the new fields), stacking.go and main.go (timber load)

I checked how the code is used:
- Override builders use the raw spec: enchantments ApplyTier/StripEnchantment and affixgen.
- Housing ownership is checked in UseItem/placeFurnishing/useContainerDeed, along with the per-room container cap.
- Stock entries are never removed (the reseed-refill check).
- Moving away while chopping is caught by the head's JobLeftBehind.
- Instanced zone biomes against the timber biomes.
- The authored arrow recovery values.
- Leftover carpentry references.

Not covered in depth:
- Whether any value-creation loop remains in tailoring and other gear recipes whose inputs never carry a grade, so they get no input cap and can reach pristine (4.0x value). The linen tunic sample (value 4 from about 11 gold of inputs) was not profitable, and I did not do a full table-wide profit analysis.
- Data races from reconcileShop editing the cached Stock slice in place outside shopCacheMu. The CraftSupport fix already did this before the PR, and I did not confirm that any reader runs outside the game loop.
- Content YAML (items, mobs, recipes), the docs, the tests, the context.md files.
- Repair, gear crit-wear and the mining files, which belong to later commits outside this piece's range.

## Findings (2)

<a id="f112"></a>
### F112 [low] RoomStand only blocks rift rooms; any other per-instance room in a timber biome would grow a fresh full stand for every instance

`internal/actions/chop.go:39` · status **confirmed** · reported as low

RoomStand refuses rooms carrying the `rift_run` temp key and nothing else. Zone instances (rooms.CreateZoneInstanceWithOpts -> CreateEphemeralZone) clone the zone's rooms into new rooms with new ids. Their long-term data starts out without a stand, so the first `chop` seeds a full one. If an instanced zone is ever authored with a forest, dense_forest or swamp biome, a player can buy entry, fell the whole stand, leave, and do it again. A tier-4 grove would then pay out without limit. I checked today's content: the four instanced zones (crash_site_interior, instance_arena, instance_jail_cell, instance_planar_oasis) use the interior, dungeon and ether biomes, so this is latent, not live. RoomVein in mine.go has the same gap.

**Failure scenario.** An operator adds an instanced forest zone, for example a dungeon whose rooms use biome dense_forest. A player opens the instance, chops every room down to stumps, lets the instance be cleaned up, opens a new one and finds every stand full again. The regrowth limit never applies.

**Existing mechanism.** rooms.(*Room).IsEphemeral() in internal/rooms/rooms.go:157

**Suggested fix.** In RoomStand, also return false when room.IsEphemeral(). Make the same change in RoomVein so the two gathering stands stay consistent. Alternatively, have timber.Parse/LoadDataFiles refuse a zone pool that names an instanced zone.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
chop.go:39: `if room.GetTempData(`rift_run`) != nil { return ... false }` is the only instancing guard. rooms/rooms.go:157 already defines `func (r *Room) IsEphemeral() bool`, and the guard does not use it. instances.go:361 calls `CreateEphemeralZone(zoneName)`. A grep of the zone-config.yaml files with `instanced: true` gives 4 zones, with biomes interior, interior, dungeon and ether. The timber.yaml biomes are forest, dense_forest and swamp.
```

- **confirmed** (low): The claim holds as written, and the reviewer correctly calls it latent. RoomStand (internal/actions/chop.go:39) has only one instancing guard: it rejects rooms where GetTempData(`rift_run`) is set. It never calls room.IsEphemeral(), which is defined at internal/rooms/rooms.go:157 (RoomId >= ephemeralRoomIdMinimum). Zone instances are built by CreateEphemeralRoomIds (internal/rooms/ephemeral.go:56). That function calls LoadRoomTemplate(roomId) for each room, gives the copy a new ephemeral RoomId, and adds it to memory. Each copy therefore starts with the template's data and carries no rift_run temp key. If the template rooms never stored a stand, timber.LoadStand returns ok=false on the copy, and RoomStand seeds a fresh full stand with NewStand. Each new instance repeats this, so the regrowth limit never applies across instances. MineVein in mine.go:39 uses the same rift_run-only guard. It is not live today. The four zones with `instanced: true` use only these room biomes: crash_site_interior (31 rooms, all interior), instance_arena (5, all interior), instance_jail_cell (1, dungeon) and instance_planar_oasis (3, ether). None of those zones appears in the zones: list of timber.yaml, whose biome pools are forest, dense_forest and swamp. That makes timber.Pool return nothing, so no stand is reachable in current content. Low severity fits: it is a hardening gap that a future content change would turn into unlimited timber, and the fix is to use the existing IsEphemeral() guard.

</details>

<a id="f113"></a>
### F113 [low] Rare-species identification in `survey` can be re-rolled for free, any number of times, even in combat

`internal/actions/chop.go:119` · status **confirmed** · reported as low

SpeciesKnown runs a new contest.AgainstDifficulty roll every time it is called, and SurveyTrees calls it on every `survey`. The command is registered as allowed in combat (usercommands.go:201, `{Survey, false, true, false}`) and costs nothing: no stamina, no cooldown, no activity. The Perception/Search check that is meant to gate naming a tier 3-4 wood (yew, walnut, ironwood) therefore only ever needs to succeed once. Chop also re-rolls it (usercommands/chop.go:63) to decide which 'axe too poor' message to show.

**Failure scenario.** A player with low Perception stands in a Fernway South room seeded with ironwood and types `survey` repeatedly. Within a few tries the roll succeeds and they get the species name, its note and the exact required axe tier. The Perception gate is no gate. The impact is information only; no items or gold change hands.

**Suggested fix.** Remember a successful identification per character per stand (for example a temp-data key on the user keyed by room id and stand species), or charge a short cooldown on a failed survey.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
chop.go:119-121: `score := CalcSearchScore(char) * messaging.SightMult(char, actor.GetRoom()); return contest.AgainstDifficulty(score, 100+15*float64(sp.Tier-2)).Success`. SurveyTrees at chop.go:144 calls `known := SpeciesKnown(actor, sp)` with no memo of an earlier result. usercommands.go:201 registers `survey` as `{Survey, false, true, false}`, which allows it in combat.
```

- **confirmed** (low): The claim holds as written. SpeciesKnown (internal/actions/chop.go:118-128) returns true for tier 1 and 2 woods. For tier 3 and up it makes a fresh contest.AgainstDifficulty roll on every call, and nothing stores the result. SurveyTrees (chop.go:154) calls it on every survey. A success shows sp.Name and sp.Note, plus the exact required axe tier through AxeTooPoor if the player's axe is too weak. The Survey handler (usercommands/chop.go:22-29) checks only CanSeeClearly. It has no IsFree check, no cooldown, no stamina cost and no activity transition. The registration is `survey: {Survey, false, true, false}` and the CommandAccess field order is Func, AllowedWhenDowned, AllowedInCombat, AdminOnly (usercommands.go:34-39), so survey is allowed in combat and blocked when downed. Chop (usercommands/chop.go:63) also rerolls SpeciesKnown to pick its "axe too poor" message, but that branch returns before any activity starts, so it is free too. Repeating the command therefore beats the Perception/Search gate. The only effect is information: the species name, its note and the needed axe tier. No item or gold moves, so the severity stays low. Being usable in combat adds little, since the roll is free anywhere.

</details>
