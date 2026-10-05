# PR #208 review: craft gather hunt, correctness lens

Blind reviewer `craft_gather_hunt:correctness`, run 2026-10-05 against PR head `3ce674451`. Findings were then checked by independent verifiers (three for critical and high, one otherwise). Severity is the verifiers' median. Back to the [synthesized report](../README.md).

## Reviewer summary

I reviewed the wilderness-trades phase 0-3 code as it stands at PR head 3ce674451, reading it for correctness. I found no panics, nil dereferences or races reachable from player input. Boot validation of harvest tables matches the sibling validators. All the new gathering knobs are present in the shipped config.yaml.

There are four real problems:
1. **Corpse targeting (high).** The carcass commands find a corpse with room.FindCorpse, which only ever returns the newest corpse of a given name. The job then finishes on the first corpse whose mob id and creation round match, which can be a different body. In practice, once the newest wolf of a pack is skinned but not also butchered and emptied of loot, the older wolves cannot be skinned at all. Two wolves killed in the same round can also have the job land on the wrong body.
2. **Corpse salvage bypasses the new flags (medium).** The player `salvage` command now refuses any corpse with a harvest table. The salvage path that mobs and companions use (actions.salvageCorpse) checks neither that table nor the skinned/butchered/parts-taken flags. A skinned or part-cut carcass can therefore be salvaged again for meat, leather and sinew. That meat never spoils, and foragers or companions can remove a corpse while a player is still mid-job on it.
3. **Storage is not a cold store (medium).** The spoilage clock runs on the absolute round number, so goods keep rotting in bank storage and while the player is offline. The code comment claims storage is a cold store. Banked meat is thrown out within 10 rounds of being withdrawn.
4. **Second spoilage mechanism (medium).** Spoilage was added as a new SpoilAfter clock beside the existing items.Aging mechanism, which already provides fresh-to-spoiled phases on the same CraftedRound stamp.

One low finding: several balance numbers are hardcoded in Go rather than read from config.

## Coverage

Read in full at PR head (3ce674451): internal/actions/harvest.go, internal/usercommands/carcass.go, internal/rooms/corpse.go, internal/mobs/harvest.go, internal/species/harvest.go, internal/gather/gather.go and gather/tools.go, internal/items/spoilage.go, quality.go and tools.go, internal/hooks/spoilage.go, internal/crafting/corpse_salvage.go.

Read the diffs for: the Salvaging and Crafting branches of NewRound_UserRoundTick, actions/craft.go, actions/salvage.go, usercommands/salvage.go, forager/forage_core.go, crafting.go (OutputCount, SalvageIngredients, tool validation), shops/buyrules.go, sell.go (stock update), buy.go (pricing), items stacking.go, items.go and itemspec.go, mobs.go, species.go, and the main.go validators. Also read config.balance.gathering.go and checked those knobs against `git show HEAD:_datafiles/config.yaml`.

Checked for the shop buy/sell arbitrage loop and found it already fixed at head: walk-in buy prices are capped by CalcBuyPrice(current+1). The ceil(1 * (1 + barter)) rounding that turns a value-1 item into a profit is pre-existing at c696c117a, so it is not reported.

Not read or only skimmed: the content YAML (species and mob harvest tables, the processing recipes, the roughly 200 help templates). I relied on the boot validators for those, and checked only the spoil_after values (all of the form "N days", which parse). Also not read: harvest_test.go, gather_test.go, wilderness_trades_content_test.go, the narration golden files, modules/aicompanion beyond finds.go, and the later-phase files (timber, mining, repair, grade_effects) beyond what they touch here.

One accidental write: a single shell command redirected a scratch listing to /tmp/x. Nothing in the repo or worktree was modified.

## Findings (5)

<a id="f043"></a>
### F043 [medium] Mob and companion corpse salvage ignores harvest tables and the Skinned/Butchered/HarvestedParts flags, so carcasses can be worked twice

`internal/actions/salvage.go:119` · status **confirmed** · reported as medium

The player `salvage` command now refuses any corpse with a harvest table and sends the player to skin/butcher (usercommands/salvage.go:160-167). actions.salvageCorpse is the path used by the forager behaviour-tree action (try_salvage), mobcommands.Salvage and the AI companion's butcher pastime (finds.go butcherable). It still accepts any corpse that LookupCorpseSalvageForMob covers. This range widened that lookup to species such as canine, bear, deer and boar. The path never consults CarcassTable, Skinned, Butchered or HarvestedParts. So a carcass a player has skinned, or cut rare parts from, can still be salvaged in full by a companion for raw-meat, two leather strips and sinew. The salvaged goods carry no CraftedRound stamp, so that raw meat never spoils, unlike butchered meat. A forager NPC can also salvage-and-remove a corpse while a player is mid-job on it, and the player then gets 'The carcass you were working on is gone.'

**Failure scenario.** A player kills a wolf, loots it and runs `skin wolf` (gets the pelt; corpse Skinned=true, kept because it is not butchered). Their AI companion idles and butcherHere picks the wolf: it has no loot, the owner may loot it, and the canine species fallback yields raw-meat. She issues `salvage <mobId>:<round>`, and salvageCorpse rolls raw-meat + leather-strip x2 + sinew and removes the corpse. One carcass has given both the skin section and a full salvage. That meat is never on a spoilage clock.

**Existing mechanism.** actions.CarcassTable / SectionEntries are the gate the player path already uses; the same check belongs in salvageCorpse so all three salvage callers agree.

**Suggested fix.** In salvageCorpse, skip a corpse whose CarcassTable is non-empty, or at least one already Skinned/Butchered/with HarvestedParts. Then move the companion butcher pastime onto ResolveHarvest (or have it refuse table carcasses), so a carcass is worked through exactly one mechanism.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
salvage.go:119-145 loop: `if c.Prunable {continue}; if c.MobId <= 0 {continue}; ... if len(crafting.LookupCorpseSalvageForMob(mobSpec.Groups, mobSpec.Character.SpeciesId)) > 0 { target = c; found = true; break }` (no Skinned/Butchered/table check).
usercommands/salvage.go:163 `if table, _, ok := actions.CarcassTable(corpse); ok && !table.Empty() { ... skin %s and butcher %s ...; return true, nil }` (player path only).
aicompanion/finds.go:123-145 butcherable uses LookupCorpseSalvageForMob only.
corpse_salvage.go:61-67 adds canine/bear/boar/deer/feline/mustelid/horse to the fallback.
harvest.go:418 `itm.CraftedRound = now` (only the harvest path stamps the spoil clock).
```

- **confirmed** (medium): The claim holds when the code is read as written. In internal/actions/salvage.go:119-145, salvageCorpse picks its target using only Prunable, MobId > 0, the optional MobId:Round filter and LookupCorpseSalvageForMob. It never calls CarcassTable or SectionEntries, and it never reads Skinned, Butchered or HarvestedParts. The only later guard is HasLoot. It then rolls the full species returns and calls RemoveCorpse.  The player path in usercommands/salvage.go:160-167 refuses any carcass with a non-empty harvest table. So the three callers disagree: - the forager try_salvage - mobcommands.Salvage, which parses `salvage id:round` - the AI companion, through butcherable/butcherHere and autonomy `salvage` + salvageCommand  None of those three apply the gate the player path uses.  Skinning sets Skinned=true, and harvest.go only removes the corpse once it is Spent (Skinned && Butchered). A wolf that has only been skinned therefore stays in the room. butcherable then accepts it, because it has no loot, the owner may loot it, and the canine species fallback returns raw-meat. The companion's salvage removes it and gives raw-meat, leather-strip x2 and sinew.  Only the harvest path stamps CraftedRound (harvest.go:418), so meat from salvage has no spoil clock. Canine does carry a species harvest table (dogmud/species/2-canine.yaml), so CarcassTable would be non-empty for the cited case.  The forager's first-eligible salvage can also remove a carcass in the middle of a job, which produces the 'carcass you were working on is gone' message at harvest.go:305. Whether that interleaving actually happens depends on timing at runtime, but nothing in the code prevents it.  I am keeping the severity at medium. The bug lets one carcass be worked twice and yields meat that never spoils. Because of the owner-loot gate, the companion case only benefits the owner's own party. Nothing here is a crash or a security issue.

</details>

<a id="f044"></a>
### F044 [medium] Raw goods keep rotting in bank storage and while the player is offline, though the code says storage is a cold store

`internal/hooks/spoilage.go:23` · status **confirmed** · reported as medium

sweepSpoiledGoods says 'Goods in bank storage are not swept: storage is a cold store.' Spoilage is computed from the absolute round number: SpoilRound = GetDate(CraftedRound).AddPeriod(SpoilAfter), and IsSpoiled(now) is now >= that. Nothing re-stamps or pauses CraftedRound on deposit or withdrawal; the only CraftedRound writers are harvest and crafting. So storage only defers the sweep: an item whose window passed while banked is spoiled the moment it is withdrawn and is thrown out within 10 rounds. The same holds for logging off: the round counter keeps advancing while offline. Crafted potions get explicit offline preservation (pinnacle_tick.go reconcilePreservedPotionRounds); raw goods get none. Spoil windows are 1-5 game days, roughly 1-5 real hours at 900 rounds per game day, so this will be hit routinely.

**Failure scenario.** A hunter butchers a deer (venison, spoil_after a few days), banks the venison to 'keep it cold', and logs off overnight. On login they withdraw it. IsSpoiled(now) is already true, and the next sweep (at most 10 rounds) throws it out with 'The smell gives it away'. The comment promised storage would preserve it.

**Existing mechanism.** hooks/pinnacle_tick.go reconcilePreservedPotionRounds already re-stamps CraftedRound to freeze aging for preserved potions; the same pattern applies to storage deposit/withdraw and login.

**Suggested fix.** Either remove the cold-store claim and document that rot continues in storage and offline, or make it true: record the remaining shelf life on deposit and logout, and re-stamp CraftedRound on withdrawal and login, as the potion preservation code does.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
hooks/spoilage.go:20-23 `// Goods in bank storage are not swept: storage is a cold store.`
items/spoilage.go:30-41 `return gametime.GetDate(i.CraftedRound).AddPeriod(i.GetSpec().SpoilAfter)` / `return r > 0 && now >= r`.
Grep of CraftedRound writers at head shows only harvest.go:418, NewRound_UserRoundTick.go:738 (craft output), migrations.go:107 and the potion-only pinnacle_tick.go:97-121; there is no storage or login re-stamp.
util.SaveRoundCount/LoadRoundCount persist the round counter across reboots (main.go:444, 599).
```

- **confirmed** (medium): The defect reproduces from the code as written. Spoilage is worked out only from the absolute round counter and the stamped CraftedRound, and nothing pauses or re-stamps that stamp while goods sit in storage or the player is offline. The sweep only looks at the backpack (c.Items) and the component bag (c.ComponentItems), so storage just puts off the check. Once an item is withdrawn, IsSpoiled(now) is true straight away, and the next sweep (within 10 rounds) throws it out. This contradicts the comment at hooks/spoilage.go:23 saying storage is a cold store.  Offline rotting is arguably intended, because docs/economy/wilderness-trades.md says goods "rot on the game clock". The storage contradiction is unambiguous, though: nothing in the code makes the cold-store claim true.  The time scale holds up. RoundsPerDay is 900 in config.yaml. The hook's own comment says 10 rounds is 40 seconds, so a game day is about 1 real hour. Spoil windows of 2 to 5 days therefore last 2 to 5 real hours, so an overnight bank deposit is a realistic way to hit this.  The claimed existing mechanism is real. pinnacle_tick.go:104-121 has reconcilePreservedPotionRounds, which re-stamps CraftedRound to keep elapsed time constant. That is the existing pattern a fix would reuse. I kept the severity at medium for the storage case. The offline case alone would be low or by design.

</details>

<a id="f045"></a>
### F045 [medium] Carcass commands can only reach the newest same-name corpse, and the job finishes on a different body than the one picked

`internal/usercommands/carcass.go:75` · status **confirmed** · reported as high

findCarcass resolves the corpse with room.FindCorpse. FindCorpse puts one entry per name into its candidate list, in newest-first order, so only the NEWEST corpse of each name can ever be returned; even `2.wolf` cannot reach the second one. startCarcassJob then rejects that newest corpse when its section is done ('has already been skinned', lines 111-119), so every older same-name carcass in the room is unreachable. A corpse is only removed when it is both skinned and butchered AND has no loot left (actions/harvest.go:442), so skinning alone, or any loot left on it (including items reserved to another player under round-robin), blocks the rest of the pack for good. Separately, the job is keyed by MobId + RoundCreated only. ResolveHarvest (actions/harvest.go:297-303) takes the FIRST (oldest) corpse with that key, while the job was started on the newest. With two of the same mob killed in the same round (an AoE pull), the job lands on the other body: possibly one already worked ('nothing left') or one the player failed LootAllowed for. The aicompanion code (finds.go salvageHits) already documents this twin-key hazard; the carcass code does not handle it.

**Failure scenario.** A player kills three steppe wolves and loots them. `skin wolf` takes the hide from the newest corpse (Skinned=true, not butchered, so it is not removed). `skin wolf` again prints 'The wolf corpse has already been skinned.' The other two wolves can never be skinned unless the player first butchers the newest one, and even that does not help if anything is left in its loot. Twin variant: two wolves die in round R. The player skins one (the oldest copy is marked) and then types `skin wolf`. FindCorpse returns the newest twin, which is unskinned, so the start check passes. ResolveHarvest finds the oldest twin, which is already skinned, and says 'nothing left to skin'. The fresh twin stays unskinnable.

**Existing mechanism.** rooms.Room.FindCorpseIndex and the util.FindMatchIndexIn N.name addressing (baubles slice D) tell same-name entries apart by position; the aicompanion salvageHits guard also exists for the twin key.

**Suggested fix.** Resolve the corpse by index, either with a per-instance corpse identity (a UUID stamped in AddCorpse) or by building the candidate list without deduping names so `2.wolf` works. Make the resolver skip corpses whose section is already done, or key the job by that unique id rather than MobId + RoundCreated.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
carcass.go:75 `corpse, found := room.FindCorpse(name)`; carcass.go:111-117 `if part == `` && len(actions.SectionEntries(table, &corpse, section)) == 0 { ... corpse.Skinned { msg = ... has already been skinned`.
rooms.go:1499-1503 `if c.MobId > 0 { name := c.Character.Name + ` corpse`; if _, ok := mobCorpseLookup[name]; !ok { mobCorpseLookup[name] = idx; mobCorpses = append(mobCorpses, name) } }` (one entry per name, newest first).
harvest.go:298-302 `for i, c := range room.Corpses { if !c.Prunable && c.MobId == opts.MobId && c.RoundCreated == opts.RoundCreated { idx = i; break } }` (oldest first).
carcass.go:131 `ItemUuid: fmt.Sprintf(`%s%s:%d`, actions.HarvestActivityPrefix, section, corpse.MobId)`.
harvest.go:442 `if corpse.Spent() && !corpse.HasLoot() { room.RemoveCorpse(*corpse) }`.
Death_MobLoot.go:86 `RoundCreated: currentRound`.
```

- **confirmed** (medium): The claim holds as written in the code. Room.FindCorpse (rooms.go:1473-1527) walks the corpses newest first and adds only the first index it sees for each "<Name> corpse" string to mobCorpseLookup/mobCorpses. FindMatchIn then runs over that de-duplicated list, so for a given name only the newest non-prunable corpse can ever come back. N.name addressing has nothing to work with because the candidate list has one entry per name. findCarcass (carcass.go:75) and Harvest (carcass.go:53, 61) all go through FindCorpse. In startCarcassJob (carcass.go:111-119), when the newest corpse's section is empty the job is refused with "has already been skinned/butchered", and no other corpse is tried.  The corpse leaves the room only through harvest.go:442-443 (`corpse.Spent() && !corpse.HasLoot()`, where Spent = Skinned && Butchered, corpse.go:58), through salvage, or through decay. So after `skin wolf` on the newest wolf, every older same-name wolf stays unreachable for skinning until the newest one is butchered and has no loot left. Loot left on it, including items reserved to someone else, keeps it there until it decays.  The twin case is real too. The job key is MobId plus RoundCreated (carcass.go:131 and 143). ResolveHarvest (harvest.go:297-303) picks the first matching corpse, which is the oldest, while the job was started on the newest. Two same-MobId kills in one round therefore share a key. After the first skin marks the oldest twin, a second `skin` passes the start check on the newest twin and then fails in ResolveHarvest with "nothing left to skin". That leaves the newest twin unskinnable until it decays.  Severity is lowered from high to medium. There is a workaround in the common case: butcher the newest corpse, so it is removed as long as it has no loot, and then the next one becomes reachable. No data is corrupted and nothing is lost permanently; the corpses decay normally. But in the ordinary multi-kill case the skin-only flow, and the harvest flow, are broken.

- **confirmed** (medium): Nothing elsewhere prevents this. Every carcass entry point (findCarcass, the Harvest phrase probes) goes through room.FindCorpse. FindCorpse, and FindCorpseIndex too, keeps one candidate per name, newest first (`if _, ok := mobCorpseLookup[name]; !ok`). FindMatchIndexIn's N.name counting therefore runs over a list that holds each name only once, so `2.wolf` cannot reach the second same-name corpse.  After `skin wolf` the newest corpse is Skinned but not Butchered. Spent() needs both, so the corpse stays, and a second `skin wolf` resolves to it again and hits the 'has already been skinned' branch at carcass.go:111-117. No guard skips worked corpses.  The twin-key part also holds. The job key uses corpse.MobId, which Death_MobLoot.go:84 sets from m.MobId, the mob TYPE id, so two wolves share it. The key adds only RoundCreated, the same for kills in the same round. ResolveHarvest (harvest.go:297-303) takes the first, oldest match, while the job started on the newest corpse.  I lowered the severity from high to medium for two reasons. The player is not blocked for good in the common case: skinning and butchering the newest corpse while it has no loot removes it (harvest.go:442), and that exposes the next one. The twin variant also needs two same-type kills in one round. Still, 'skin each wolf, then butcher' is the natural flow and it fails, and leftover or round-robin-reserved loot does make the older corpses unreachable.

- **confirmed** (medium): Both halves of the claim hold up in the code. (1) Newest-only reach: rooms.go FindCorpse (1473-1527) walks the corpses newest-first and keeps only the first index for each "<Name> corpse" key. Each name gets one candidate, so FindMatchIn can never return an older same-name corpse; an N.name prefix has nothing to pick between. findCarcass (carcass.go:75) uses it, and startCarcassJob (111-119) then refuses the newest corpse once its section is empty. A corpse leaves the room only when Skinned && Butchered && !HasLoot (harvest.go:442; corpse.go Spent/HasLoot). So `skin wolf` x3 on three wolves skins only one and then answers "already been skinned". (2) Twin key: the job stores only MobId (in ItemUuid) and RoundCreated (HarvestRoundKey). ResolveHarvest (harvest.go:297-302) scans oldest-first and takes the first non-prunable match, while FindCorpse picked the newest, so same-round twins split across two bodies as described. The PR's own aicompanion salvageHits comment documents this hazard; the carcass commands do not guard against it. I lowered the severity from high to medium for three reasons. It is a gameplay and usability defect with no data loss and no security or economy exploit. A workaround exists: butcher the newest corpse and loot it empty, and the next one becomes reachable. The twin case needs two kills of the same mob in one round. Still, packs of same-name mobs are common, so the bug will be hit in normal play of the new feature. Note: FindCorpseIndex has the same one-per-name dedupe, so the 'existing mechanism' that tells same-name corpses apart by position is not in the room corpse lookup. Only the item-side N.name addressing does that.

</details>

<a id="f104"></a>
### F104 [low] Harvest balance numbers hardcoded in Go instead of config

`internal/actions/harvest.go:249` · status **confirmed** · reported as low

Several values that shape harvest outcomes are Go literals rather than Balance knobs: the rare-part notice clamp (0.02..0.9; the config.yaml comment even describes it as part of the tuning), the body-size quantity scale (small x0.5, large x2), the freshness sell-price floor (half price at the point of spoiling) and the half-step 'narrow loss still gives crude' band. This goes against the project rule that balance numbers come from config.yaml; retuning any of them needs a code change.

**Failure scenario.** The owner wants rare parts noticed more reliably by high-Perception characters, or large carcasses to give 3x. Editing config.yaml has no effect because the cap and the multipliers are literals.

**Existing mechanism.** internal/configs/config.balance.gathering.go (validateGathering) already holds the sibling knobs (GatherRareBaseChance, GatherJobRounds*, CorpseStaleGradeAt).

**Suggested fix.** Add GatherRareChanceMin/Max, HarvestQtySmallMult/LargeMult and SpoilSellFloor to Balance with defaults in validateGathering and entries in config.yaml.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
harvest.go:249 `chance = math.Max(0.02, math.Min(0.9, chance))`.
species/harvest.go:91-96 `case Small: f *= 0.5 / case Large: f *= 2`.
items/spoilage.go:63 `return 0.5 + 0.5*i.Freshness(now)`.
gather/gather.go:207 `case !cr.Floored && res.Sigmas > -0.5*step:`.
```

- **confirmed** (low): All four cited literals exist in the PR head exactly as described, and none of them is reachable from config. The sibling knobs for the same formulas do live in config.balance.gathering.go and config.yaml: GatherRareBaseChance, RareToolMultCrude/Iron/Steel/Masterwork, GatherSizeDifficultyMedium/Large, CorpseStaleGradeAt and GatherGradeStepSigma. So the rare-notice cap, the size multipliers on quantity, the freshness price floor and the crude band can only be retuned by changing Go code. The failure scenario holds. One part of the claim is overstated. The config.yaml comment says only "Rare parts: chance * Perception/100". It does not mention the 0.02..0.9 clamp, or the tool multiplier either, so it does not "describe the clamp as part of the tuning". If anything, the comment under-documents the real formula. A probability clamp and the half-step crude band are arguably structural rather than balance numbers. The body-size quantity scale (x0.5 / x2) and the freshness floor (0.5) are plainly balance values, though, and they sit next to existing size and freshness knobs. This is a consistency and maintainability issue with no runtime bug, so severity stays low.

</details>

<a id="f132"></a>
### F132 [medium] Spoilage is a second clock (SpoilAfter) beside the existing items.Aging mechanism

`internal/items/spoilage.go:21` · status **refuted** · reported as medium

The repo already has a per-instance freshness clock for perishables: items.Aging (AgingThresholds with SpoilRounds, PhaseSpoiled), keyed off the same Item.CraftedRound with the same 'CraftedRound==0 never ages' rule. Shops already refuse spoiled items through it (buyrules isPotionDeclining) and eat.go reads it for food. This range adds an independent SpoilAfter/IsSpoiled/Freshness/FreshnessValueMultiplier stack with its own shop gate and its own sweep hook. Two clocks on one field means a spec can carry both, with different meanings. Each new consumer (eat, sell, storage, the offline freeze in pinnacle_tick) has to know to check both, and the existing offline-preservation logic covers only one of them.

**Failure scenario.** A future change gives raw meat an aging block or a cooked-food spoil, or extends potion preservation to all perishables. The shop path checks one clock via isPotionDeclining (potions only) and the other via IsSpoiled. Eat checks only Aging and the sweep checks only SpoilAfter. An item can then be 'spoiled' under one and fresh under the other, and preservation fixes made in pinnacle_tick do not reach SpoilAfter goods.

**Existing mechanism.** internal/items/aging.go (AgingThresholds.SpoilRounds, GetAgingPhase, PhaseSpoiled) and the CraftedRound-based aging consumers in eat.go, drink.go, NewRound_AutoHeal.go and pinnacle_tick.go.

**Suggested fix.** Express raw-goods spoilage as Aging thresholds (spoil_rounds, or a period string converted at load) and route IsSpoiled/Freshness through GetAgingPhase, so one clock serves food, potions and raw goods.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
items/spoilage.go:21-41 (new Spoils/SpoilRound/IsSpoiled on CraftedRound + SpoilAfter); itemspec.go adds `SpoilAfter string yaml:"spoil_after"`.
items/aging.go:11-31 existing `PhaseSpoiled`, `AgingThresholds{... SpoilRounds ...}`, `GetAgingPhase`.
shops/buyrules.go:164-181 existing isPotionDeclining gate, plus the new `if item.IsSpoiled(now) { return BuyOffer{} }` at :84.
```

- **refuted** (none): The facts in the claim are right: PR #208 adds a second freshness clock (SpoilAfter, Spoils, SpoilRound, IsSpoiled, Freshness, FreshnessValueMultiplier in internal/items/spoilage.go). It sits beside items.Aging and uses the same CraftedRound==0 opt-out. The claimed correctness defect does not reproduce from the code as written, though.  1) No spec carries both clocks. Every spec with spoil_after (raw meat, hare meat, pelts and hides in materials-40000) has no aging block. Every spec with spoil_rounds is a consumables-30000 potion or salve.  2) The eat-path divergence cannot happen. 40014 Raw Meat is `subtype: mundane`, not Edible, and eat.go refuses anything that is not Edible before it reads Aging. Separately, the player sweep (hooks/spoilage.go, every 10 rounds) removes IsSpoiled goods from the backpack.  3) The "offline preservation does not reach SpoilAfter goods" point misreads pinnacle_tick.go. tickPreserveContents and reconcilePreservedPotionRounds only advance CraftedRound for c.PotionItems inside a preserves_contents bandolier. They are not a general perishables freeze, so raw goods were never in scope.  4) The shop gate does not conflict either. isPotionDeclining is gated on spec.Aging.HasAging() and IsSpoiled on SpoilAfter, and the two sets of specs do not overlap.  Every failure scenario offered depends on a hypothetical future spec carrying both blocks. As a unification note, reusing Aging would also have been a poor fit. It counts integer rounds rather than game-time periods, is potion-shaped (a potency boost of 1.15 or 1.30 at the fermented and peak phases), and is scaled by bottle and craft skill. Its consumers (look, inventory, throw, salvage) show alchemy phase text. An Aging block with only spoil_rounds set would land in PhaseDeclining at once with a 1.30 multiplier. The parallel mechanism is a defensible design choice, not a present correctness bug. At most it is a low-priority consistency note: a spec validator could reject a spec that sets both aging and spoil_after.

</details>
