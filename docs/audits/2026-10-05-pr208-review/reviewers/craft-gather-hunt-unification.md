# PR #208 review: craft gather hunt, unification lens

Blind reviewer `craft_gather_hunt:unification`, run 2026-10-05 against PR head `3ce674451`. Findings were then checked by independent verifiers (three for critical and high, one otherwise). Severity is the verifiers' median. Back to the [synthesized report](../README.md).

## Reviewer summary

Unification review of wilderness trades phases 0 to 3 (a4f372715..46766d13b), judged at the PR head. I found five places where existing mechanisms were bypassed or duplicated, plus a few smaller hardcoded balance numbers.

1. **Two corpse-yield systems, one of them unguarded (high).** Player carcass work is now species/mob `harvest:` tables plus `gather.Roll`. Mobs and companions still use the flat `crafting.LookupCorpseSalvageForMob` table, now with a new `speciesCorpseSalvage` fallback map that covers the same species. That old path ignores the new `Corpse.Skinned`/`Butchered`/`HarvestedParts` state. So one carcass yields twice, and the second set of goods is ungraded and never spoils.
2. **A second spoilage clock beside the existing aging system.** `items.SpoilAfter` and `Item.IsSpoiled`/`Freshness` sit next to `items.AgingThresholds` and `GetAgingPhase`. Both run off `Item.CraftedRound`. Eat, look, inventory and the GMCP item editor only know the old one.
3. **The pricing baseline is bypassed for player sells.** Player sells of RestockQty==0 entries now use a new `WalkInBuyPrice` curve instead of `shops.PricingBaseline`/`CalcBuyPrice`. Every NPC valuation path (craftdecision, shop_upgrade, aicompanion economy, list) still prices the same entries on the scarcity curve.
4. **Grade pricing is in one sell path only.** `shops.GradedValue` lives only inside `EvaluateBuyRules`. Fence sales, the legacy `Mob.GetSellPrice` merchants, `offer` and `appraise` still price from `spec.Value`. A crude pelt fenced pays 60% of full value, against 20% at an honest shop.
5. **A hand-rolled rare-part chance roll.** It uses Perception/100 with hardcoded 0.02/0.9 clamps instead of `contest.AgainstDifficulty`, which is the documented one path for "notice it" checks.

**Mutations that no longer reach carcass or crafted-quality work.** Provident Hands (`salvageScoreWithMutations`) no longer applies to animal carcasses, because the player salvage command now turns them away. Faithwrought's "comes out finer" only feeds `CraftSkill`, while the new `Item.Quality` grade (which sets value and tool tier) ignores it.

**Done right:**
- `gather.Roll` resolves through `crafting.RunSalvageContest`, so SalvageFloor and the shared contest crit/margin rules are honoured, and the sight ramp is applied once via `messaging.SightMult`.
- Starting a carcass job is gated on `messaging.CanSeeClearly`.
- Every gathering, tool, grade, staleness and walk-in knob is declared in `config.balance.go` with a `validateGathering` default and shipped in `config.yaml`. I checked this key by key.
- Skill awards go through `actor.AwardResolved` with `CandidateFor(skills.Salvage)`.
- Carcass jobs reuse the Salvaging activity with the same ItemUuid-prefix plus MiscData round-key pattern as corpse salvage.
- `ToolSatisfied` copies the `StationSatisfied` single-rule pattern across craft, list, status and storage pull.
- `species.Size` is reused, corpse identity uses MobId+RoundCreated as salvage does, and `Corpse.Staleness` uses the same `CorpseDecayTime` period as `Corpse.Update`.
- `SameStack` was extended for grade and clock rather than a parallel stacking rule.
- Narration goes through `messaging.SendTrio`.
- Validation panics at boot, on the existing `ValidateSpeciesConditionIds` pattern.
- The craft-output grade reuses `GatherGradeStepSigma`.

## Coverage

Read in full at PR head: internal/gather/gather.go, internal/gather/tools.go, internal/actions/harvest.go, internal/usercommands/carcass.go, internal/items/quality.go, internal/items/spoilage.go, internal/hooks/spoilage.go, internal/species/harvest.go, internal/mobs/harvest.go, items/tools.go (at 46766d13b), and config.balance.gathering.go (first 90 lines). Read the range diffs for actions/salvage.go, usercommands/salvage.go, crafting/corpse_salvage.go, crafting/crafting.go, NewRound_UserRoundTick.go, actions/craft.go, usercommands/craft.go, shops/buyrules.go (plus head lines 76-130), shopinventory.go, economy/buckets.go, items/items.go, stacking.go, itemspec.go, rooms/corpse.go, look.go, forager/forage_core.go, aicompanion/finds.go, mobs.go, mobs/save.go, mutations/graph.go, skills/skills.go, and the guard tests. Baseline comparisons read: contest/contest.go, crafting/difficulty.go (CraftScore, RunCraftContest/RunSalvageContest), shops/pricing.go PricingBaseline, items/aging.go, eat.go aging check, mobs.GetSellPrice, FencePrice/sell_stolen, offer.go, appraise.go value line, StationSatisfied, CraftQualityLevel, the Faithwrought mutation, salvageScoreWithMutations, gmcp.Item.go reqToSpec, and the mob and forager salvage call sites. Verified that every gathering knob named in config.balance.go is present in the committed config.yaml. Not reviewed: the ~200 content YAML/help template files, recipe YAMLs, harvest/gather/quality/spoilage tests, wilderness_trades_content_test.go, docs, narration golden file, internal/hooks/chrysifier_homunculus.go and the main.go wiring, beyond confirming that validation is called. Later-PR files that touch these areas (rooms/spoilage.go, chop/mine resolvers, gear wear/repair, scrap) were seen only incidentally and were not reviewed in depth.

## Findings (7)

<a id="f049"></a>
### F049 [medium] Mob/companion corpse salvage still runs the old flat table on carcasses that now have harvest tables, ignoring Skinned/Butchered state

`internal/actions/salvage.go:140` · status **confirmed** · reported as high

Phase 2 made `skin`/`butcher`/`harvest` (species/mob `harvest:` tables + gather.Roll) the way to work an animal carcass, and usercommands/salvage.go:163 turns PLAYERS away from salvaging any corpse whose CarcassTable is non-empty. The shared action actions.salvageCorpse, used by mob `salvage` (mobcommands/salvage.go:42), forager behaviour-tree mobs (behaviortree/actions_forager_verbs.go:51) and the AI companion butcher pastime (aicompanion/finds.go:134), was not routed through the new path. It still picks any non-loot corpse whose `LookupCorpseSalvageForMob` is non-empty. In the same range the contributor added a parallel per-species yield table, `speciesCorpseSalvage` (crafting/corpse_salvage.go:59), for canine/bear/boar/deer/feline/mustelid/horse/bird/raptor/rodent. Every one of those species also got a `harvest:` table in this range. So the codebase now has two per-species carcass yield definitions that can drift, and the one serving mobs and companions ignores `Corpse.Skinned`, `Butchered` and `HarvestedParts`. Its items also come out of storeRecovered (salvage.go:408), which stamps no Quality and no CraftedRound, so they never spoil.

**Failure scenario.** A player skins a wolf (gets the pelt; corpse.Skinned=true, corpse stays because it is not Spent). Their AI companion's butcher pastime (or a forager mob) then runs `salvage <mobId>:<round>` on the same body. salvageCorpse finds it eligible via speciesCorpseSalvage["canine"] and yields raw meat + 2 leather strips + sinew, then removes the corpse. The hide is taken twice and the butcher section never pays the graded/spoiling path. The companion's raw meat has CraftedRound 0, so it never rots, while the player's butchered meat rots in 2 days. A forager mob can likewise destroy a carcass mid-way through a player's 4 to 6 round skin job (salvageCorpse checks HasLoot, not the in-progress job or LootAllowed).

**Existing mechanism.** The PR's own species/mob harvest table (mobs.ResolveHarvest + actions.ResolveHarvest) should be the single carcass-yield path. Alternatively, actions.salvageCorpse should refuse a corpse whose actions.CarcassTable is non-empty, just as usercommands/salvage.go now does.

**Suggested fix.** Put the CarcassTable gate inside actions.salvageCorpse, where every actor passes, instead of only in the player command. Give mobs and companions a mob-side skin/butcher route through actions.ResolveHarvest (it already takes an Actor), and delete speciesCorpseSalvage once nothing reaches it.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
actions/salvage.go:140 `if len(crafting.LookupCorpseSalvageForMob(mobSpec.Groups, mobSpec.Character.SpeciesId)) > 0 { target = c ...` (no Skinned/Butchered/CarcassTable check); salvage.go:181-183 rolls returns then `room.RemoveCorpse(target)`. usercommands/salvage.go:163 `if table, _, ok := actions.CarcassTable(corpse); ok && !table.Empty() { ... skin/butcher instead ...}` gates only the player command. crafting/corpse_salvage.go:59 `var speciesCorpseSalvage = map[string][]items.SalvageReturn{ "canine": corpseSalvageTable[1].Returns, "deer": ..., "bird": ...}`. `git grep -ln '^harvest:' species/*.yaml` lists 2-canine, 3-bear, 6-boar, 7-deer, 9-raptor, 10-rodent, 11-feline, 24-mustelid, 25-bird, 26-horse among others. modules/aicompanion/finds.go:134 `returns := crafting.LookupCorpseSalvageForMob(spec.Groups, spec.Character.SpeciesId)`.
```

- **confirmed** (medium): I read the code and the claim holds. The PR added a carcass gate only to the player `salvage` command: usercommands/salvage.go:163 refuses a corpse when `actions.CarcassTable(corpse)` returns a non-empty table. The shared `actions.salvageCorpse` (internal/actions/salvage.go:116-183) has no such gate. It picks any non-prunable mob corpse for which `crafting.LookupCorpseSalvageForMob` returns something. It never looks at `Skinned`, `Butchered`, `HarvestedParts` or `CarcassTable`. It rolls the flat returns and calls `room.RemoveCorpse(target)`.  Three callers reach that path, and none of them adds a harvest-state check: - mobcommands/salvage.go, which handles the bonded-companion `salvage <mobId>:<round>` form. - behaviortree/actions_forager_verbs.go actTrySalvage (first eligible corpse). - aicompanion/finds.go `butcherable`, which checks HasLoot, LootAllowed and LookupCorpseSalvageForMob only.  The PR also added `speciesCorpseSalvage` (crafting/corpse_salvage.go) for rodent, canine, bear, boar, deer, feline, mustelid, horse, bird and raptor. Every one of those species now also has a `harvest:` block in _datafiles/world/dogmud/species. That gives two per-species carcass yield definitions.  The Skinned flag only matters to the harvest path. `Skinned=true` with `Butchered=false` leaves the corpse in the room (`corpse.Spent()` needs both), so a skinned wolf stays eligible for mob/companion salvage, which pays leather strips again and destroys the carcass.  `storeRecovered` (salvage.go:408-426) sets no Quality or CraftedRound. The harvest path sets both (harvest.go:417-418), so spoilage and grade differ between the two paths.  Severity is lowered to medium for three reasons: 1. The mob path ignoring corpse state predates this PR for group-matched corpses such as the `animal` group; the PR widened it through the species fallback and added the player-only gate, creating the inconsistency. 2. The double yield is leather strips and sinew from the old table, not a second pelt. 3. The companion route needs the owner's own kill (LootAllowed).  The mid-job destruction by a forager mob is plausible from the code (no job lock is checked), but I did not trace the skin activity's corpse lookup on completion.

- **confirmed** (medium): Nothing outside the player command stops this. In the PR, actions.salvageCorpse (internal/actions/salvage.go:121-183) picks a corpse in only these cases: it is not Prunable, MobId>0, it optionally matches the given mob and round, and LookupCorpseSalvageForMob returns something. HasLoot is the only check after that. It never reads Corpse.Skinned, Butchered or HarvestedParts, and it never calls actions.CarcassTable or ResolveHarvest. The only carcass-table gate is in the player path, usercommands.startCorpseSalvage (usercommands/salvage.go:160-168). I grepped modules/, internal/mobcommands and internal/behaviortree for Skinned, Butchered, CarcassTable and ResolveHarvest and got no hits. So the mob salvage command, forager mobs and the companion check `butcherable` (aicompanion/finds.go, which looks only at Prunable, HasLoot, LootAllowed and LookupCorpseSalvageForMob) all bypass the harvest model. The PR also added speciesCorpseSalvage (crafting/corpse_salvage.go), a second per-species yield table. It covers canine, bear, boar, deer, feline, mustelid, horse, bird, raptor and rodent, and every one of those species also gets a `harvest:` table in _datafiles/world/dogmud/species. That makes two carcass-yield definitions for the same species. A skinned carcass is left in the room until Spent() (Skinned && Butchered, harvest.go:442), so a companion or forager can still salvage a half-worked carcass and get the leather back a second time. Salvage output comes from storeRecovered via items.New and gets no Quality or CraftedRound stamp, while harvest.go:417-418 stamps both. I lowered the severity from high to medium for three reasons. The double yield is small (one leather and meat bundle). It needs a companion or a forager mob in the room. The companion path is still bounded by LootAllowed for the owner. It is a real unification and consistency defect that allows a modest exploit, not data loss or a security problem.

- **confirmed** (medium): The defect is real, but "high" overstates it. actions.salvageCorpse (internal/actions/salvage.go:116-183) chooses a corpse using only Prunable, MobId>0, the optional MobId:Round filter, a non-empty LookupCorpseSalvageForMob and HasLoot. It never checks Skinned, Butchered, HarvestedParts or actions.CarcassTable. Only the player command (usercommands/salvage.go:163) sends carcasses that have a table to skin/butcher. The PR added three things: the species fallback (speciesCorpseSalvage, crafting/corpse_salvage.go), the companion butcher pastime (modules/aicompanion/finds.go is a new file and the baseline has no 'butcher' in modules/aicompanion), and the mob 'salvage <id>:<round>' targeting. So the PR itself created a second per-species yield path, and that path ignores its own carcass state. Every species named in speciesCorpseSalvage (rodent, canine, bear, boar, deer, feline, mustelid, horse, bird, raptor) has a `harvest:` table, so all of them are affected. This is an inconsistency inside the PR's own unification work and should be fixed (for example, salvageCorpse refusing a corpse whose CarcassTable is non-empty, or at least one that is Skinned/Butchered).  Why I lowered it to medium: (1) The companion butcher path (finds.go butcherable) requires LootAllowed(ownerUserId) and !HasLoot. So the double dip mostly lets the same owner's companion take the hide twice on that owner's own kill. That is a mild economy exploit, not cross-player harm or a security issue. (2) The non-spoiling raw meat is a balance issue, not a data or crash defect. (3) The forager-mob case does exist. actTrySalvage in behaviortree/actions_forager_verbs.go takes the first eligible corpse, and forager.yaml wires try_salvage. salvageCorpse does not check LootAllowed or an in-progress job, so a forager can remove a carcass while a player is partway through skinning it. But the forager-takes-the-corpse behaviour itself predates the PR. The PR widened it through the species fallback (116 more creatures become eligible) instead of creating it. No crash, persistence corruption or privilege issue is involved.

</details>

<a id="f050"></a>
### F050 [medium] Salvage and craft-quality mutations do not reach the new carcass and grade paths

`internal/gather/gather.go:127` · status **confirmed** · reported as medium

gather.Roll scores carcass jobs with skills.Salvage, but builds its score fresh rather than going through actions.salvageScoreWithMutations (Provident Hands: mutations.GetSalvageYieldBonus). Before this PR, every animal corpse was worked through actions.Salvage, which applied that bonus. Players are now turned away from salvage on any carcass with a harvest table (17 species), so the Provident Hands bonus silently stops applying to the main thing players salvaged. On the craft side, Faithwrought (craft_quality_bonus: 'it comes out finer') feeds only Item.CraftSkill via Character.CraftQualityLevel. The new Item.Quality grade, which now sets sell value (GradedValue) and tool tier (EffectiveToolTier), is computed in gather.CraftGradeOutput from the contest margin alone, so the mutation never reaches it. The result is two parallel notions of crafted quality (CraftSkill and Quality), with the mutation wired only to the one that does not drive value.

**Failure scenario.** A Chrysifier with Provident Hands skins a deer. Before the PR, `salvage deer` got score*(1+bonus). Now `salvage deer` is refused and `skin deer` rolls without the bonus. A Faithwrought smith forges a steel knife: CraftSkill is lifted 15%, but its Quality (which decides whether it works as masterwork and what it sells for) is unaffected.

**Existing mechanism.** actions.salvageScoreWithMutations (internal/actions/salvage.go) and Character.CraftQualityLevel / mutations.GetCraftQualityBonus (internal/characters/chrysifier.go)

**Suggested fix.** Apply salvageScoreWithMutations (or a gather-side equivalent fed from the same mutation getter) in gather.Roll for Salvage-skill jobs, and let GetCraftQualityBonus nudge the CraftGradeOutput margin. Alternatively, record that these mutations deliberately do not apply and update their descriptions.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
gather.go:127 `StatAvg: (float64(c.GetStatValue(job.StatA)) + float64(c.GetStatValue(job.StatB))) / 2,` with Score (gather.go:146) `(in.StatAvg*mult + float64(in.SkillLevel)*float64(b.GatherSkillWeight)) * sight` and no mutation term. actions/salvage.go:28 `func salvageScoreWithMutations(char, base) { if bonus := mutations.GetSalvageYieldBonus(char.Mutations); bonus > 0 { return base * (1.0 + bonus) } }`. usercommands/salvage.go:163 refuses carcasses. characters/chrysifier.go:27 CraftQualityLevel feeds only `newItem.CraftSkill`. faithwrought.yaml pros `craft_quality_bonus: 0.15`.
```

- **confirmed** (medium): Both halves of the claim hold up in the code. Salvage half: gather.Roll (gather.go:110-142) builds Inputs from stats, skill, tool tier and sight, and Score (gather.go:146-157) has no mutation term. JobSkin and JobButcher use skills.Salvage (gather.go:51-58). The only caller of salvageScoreWithMutations is actions.Salvage (salvage.go:100). At baseline c696c117a, usercommands/salvage.go had no CarcassTable or harvest refusal, and crafting/corpse_salvage.go mapped the generic "animal" group to raw meat, so animal corpses were salvaged with the Provident Hands score bonus. The PR adds a refusal in startCorpseSalvage for any corpse whose CarcassTable is non-empty and sends the player to skin and butcher, which do not apply the bonus. Craft half: craft.go:254-256 sets newItem.Quality from RecipeGrade, which wraps gather.CraftGradeOutput (contest margin, inputs and tool only). CraftQualityLevel (Faithwrought) feeds only newItem.CraftSkill, and the async hook path at NewRound_UserRoundTick.go:736-739 does the same. GradedValue and EffectiveToolTier read Quality. So the mutation never reaches the grade that drives value and tool tier. This is a real unification gap: a mutation's effect silently drops on the paths players now use. It does not crash or corrupt anything, so medium fits.

</details>

<a id="f051"></a>
### F051 [medium] Raw-goods spoilage is a second clock parallel to the existing item aging system

`internal/items/spoilage.go:21` · status **confirmed** · reported as medium

The baseline already had per-item decay: ItemSpec.Aging (items.AgingThresholds with SpoilRounds), items.GetAgingPhase/PhaseSpoiled, and CalcEffectiveAgingSpeed, clocked from Item.CraftedRound. It is used for food in eat.go ('food has no bottle'), by shops.isPotionDeclining in buyrules, and by the inventory/look/salvage/throw displays, and is surfaced in the GMCP item editor. The PR adds a separate ItemSpec.SpoilAfter (a game-time period string) with its own Item.Spoils/SpoilRound/IsSpoiled/Freshness/FreshnessValueMultiplier, also clocked from CraftedRound. It gets its own sweep (hooks/spoilage.go), its own buy refusal and price slide, and its own SameStack rule. None of the existing aging consumers know about SpoilAfter, and none of the new spoilage consumers know about Aging.

**Failure scenario.** An admin uses the GMCP item editor (modules/gmcp/gmcp.Item.go:249) to give raw meat spoil_rounds. It does nothing to harvested-meat rot, which only reads spoil_after, and the editor cannot see or edit spoil_after. Raw meat with SpoilAfter shows no age text in look/inventory, which only render the aging phase. Shops now price decay two ways: the potion path via isPotionDeclining (phase-based) and the raw-goods path via FreshnessValueMultiplier (a hardcoded 0.5 to 1.0 linear slide). Retuning one leaves the other behind.

**Existing mechanism.** items.AgingThresholds / items.GetAgingPhase / PhaseSpoiled (internal/items/aging.go), with the CraftedRound clock already used by eat.go and shops.isPotionDeclining

**Suggested fix.** Express raw-goods rot through ItemSpec.Aging (a decay-only threshold set: SpoilRounds, plus DecayRounds for the price slide), or extend AgingThresholds with a period form. Then the editor, look/inventory text, eat and shop pricing share one phase function. Move the 0.5 freshness floor to a Balance knob.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
items/itemspec.go:353 `Aging AgingThresholds yaml:"aging,omitempty"` (baseline) vs itemspec.go:366 `SpoilAfter string yaml:"spoil_after,omitempty"` (new). items/spoilage.go:34 `return gametime.GetDate(i.CraftedRound).AddPeriod(i.GetSpec().SpoilAfter)`; spoilage.go:63 `return 0.5 + 0.5*i.Freshness(now)` (hardcoded, no knob). eat.go:50 `if itemSpec.Aging.HasAging() && matchItem.CraftedRound > 0 { ... items.GetAgingPhase(...) == items.PhaseSpoiled` (baseline). `git grep IsSpoiled|Spoils()` at head: only harvest.go, stacking.go, rooms/spoilage.go and shops/buyrules.go. No look/inventory/eat consumer.
```

- **confirmed** (medium): I read the code at the PR head and the claim holds. Before the PR there was already a general per-item decay clock: ItemSpec.Aging (AgingThresholds with SpoilRounds), GetAgingPhase/PhaseSpoiled, run from Item.CraftedRound. It covers more than potions. eat.go uses it to refuse spoiled food ("food has no bottle"), and the 2026-04-09 qol-batch plan already added `aging:` thresholds to food so food could spoil.  The PR adds a second clock, ItemSpec.SpoilAfter, also run from CraftedRound, with its own set of methods: Spoils, SpoilRound, IsSpoiled, Freshness and FreshnessValueMultiplier. Its consumers are separate too: harvest.go, the stacking.go SameStack rule, the rooms/spoilage.go sweep, and the buy refusal and price slide in shops/buyrules.go. No Aging consumer reads SpoilAfter: drink, eat, inventory, look, salvage, throw, NewRound_AutoHeal, isPotionDeclining in buyrules, and the GMCP editor. No spoilage consumer reads Aging.  Two parts of the claim are overstated, but neither changes the verdict: - Raw meat (40014) is subtype mundane, not edible, so eat.go never sees it. - Aging carries potion ideas (ferment and peak potency boosts, bottle multiplier, alchemy-skill descriptions), which gives the PR some reason to want a simpler rule for raw goods.  Even so, the PR does not extend or reuse AgingThresholds, for example by letting SpoilRounds alone drive raw goods. It builds a parallel clock with a hardcoded 0.5 to 1.0 price slide and no balance knob. The GMCP editor still exposes only Aging, and look/inventory still show only the Aging phase, so 10+ pelt and meat items carrying spoil_after rot with no age text and cannot be edited in the editor. This is the inconsistency the unification effort targets, so medium stands.

</details>

<a id="f105"></a>
### F105 [low] Rare-part 'noticed' check is a bespoke probability roll with hardcoded clamps instead of the shared contest path

`internal/actions/harvest.go:248` · status **confirmed** · reported as medium

Whether a gatherer notices a rare part (fang, gland, antler) is a Perception check against a difficulty. The codebase's documented single path for that is contest.AgainstDifficulty ('searching a room, following a trail, foraging ... the alternative, a separate threshold helper, is how the codebase ended up with several unrelated ways to decide the same kind of question'). planHarvest instead computes `base * Perception/100 * RareMult(tier)`, clamps it to a hardcoded 0.02..0.9 band (not Balance knobs, so not retunable from config.yaml), and rolls util.Rand(10000)/10000. It also bypasses the sight ramp: gather.Roll applies SightMult to the job score, but the rare-notice chance uses raw Perception, so a dim room does not make a rare part harder to spot.

**Failure scenario.** Search/track noticing uses contest crit/fumble/margin semantics and the SightMult ramp. Rare-part noticing does not. A player in near-dark (SightMult well below 1) notices a hidden gland at the same rate as in daylight. The 2%/90% bounds cannot be tuned without a code change, against the project rule that balance numbers come from config.yaml.

**Existing mechanism.** contest.AgainstDifficulty (internal/contest/contest.go), or crafting.RunSalvageContest with a sight-ramped perception score as gather.Roll already does

**Suggested fix.** Score perception * SightMult * RareMult(tier) against a Balance-knob difficulty via contest.AgainstDifficulty/RunWithFloors. Make any floor a config knob rather than the 0.02/0.9 literals.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
harvest.go:248 `chance := base * float64(in.Perception) / 100.0 * gather.RareMult(tier)`; :249 `chance = math.Max(0.02, math.Min(0.9, chance))`; :250 `if in.Rand() >= chance { continue }`; :375 `Perception: char.GetStatValue(\`perception\`)` (no SightMult); :380 `Rand: func() float64 { return float64(util.Rand(10000)) / 10000.0 }`. contest/contest.go AgainstDifficulty doc comment quoted above.
```

- **confirmed** (low): The cited code is in the PR head exactly as the claim quotes it. In internal/actions/harvest.go, planHarvest computes `chance := base * Perception/100 * gather.RareMult(tier)`, clamps it with the literals `math.Max(0.02, math.Min(0.9, chance))` and rolls `in.Rand()`. ResolveHarvest supplies `Perception: char.GetStatValue("perception")` with no SightMult and `Rand: util.Rand(10000)/10000`. contest.AgainstDifficulty's doc comment does name foraging and searching as the single path, and warns against separate threshold helpers. So all three parts of the claim hold: the roll is bespoke, the 0.02/0.9 clamps are not config knobs (only GatherRareBaseChance is), and the notice chance has no sight ramp. The PR also adds the same pattern in mine.go:197-198, with different literal clamps (0.005..0.25). That makes it a second copy of the bespoke helper, not a one-off.  I downgrade the severity for two reasons. First, darkness is not entirely ignored. planHarvest only runs when roll.Success is true, and gather.Roll applies SightMult (gather.go:138,152). A dark room therefore lowers whether any parts come off at all, though it does not lower rare-part noticing beyond that. Second, the per-entry `chance` behaves like a loot-table drop rate scaled by Perception, not a contested difficulty, so forcing it through AgainstDifficulty is a design choice rather than a clear bug. The base rate can still be tuned (and turned off) from config. What is real is a consistency and tunability gap: hardcoded clamps against the balance-from-config rule, and a parallel threshold mechanism of the kind the contest doc warns about.

</details>

<a id="f106"></a>
### F106 [low] Material grade only reaches the honest-shop buy path; fence, legacy merchant, offer and appraise still price from spec.Value

`internal/shops/buyrules.go:87` · status **confirmed** · reported as medium

GradedValue (spec.Value * QualityValueMultiplier) is applied only inside shops.EvaluateBuyRules, plus repair.go. The other sell/valuation paths for the same item still read spec.Value raw: actions.sellStolenToFence (sell_stolen.go:126, FencePrice(item.GetSpec().Value)), offer's fence quote (usercommands/offer.go:47), the legacy merchant mobs.Mob.GetSellPrice (mobs.go:1176, value = item.GetSpec().Value), and appraise's 'Worth' line (appraise.go:121). Spoiled-goods refusal is likewise only in EvaluateBuyRules. Grade-based pricing was added as a local step in one path rather than as an item-value seam that all of them call.

**Failure scenario.** QualityValueCrude ships at 0.4 and ShopBuyRatio at 0.5. An honest living-economy shop pays 0.2x value for a crude pelt. If the pelt is stolen, a fence pays FencePrice(spec.Value) = 60% of full value: three times the honest price, and above what a standard pelt fetches honestly. A pristine pelt (4.0x) sold to a legacy GetSellPrice merchant fetches the ungraded price. `appraise` reports the ungraded Worth while `sell` pays the graded one, which is the appraise-vs-sell mismatch already logged in memory.

**Existing mechanism.** There is no single instance-value helper; the right move is to make GradedValue (or an items-level Item.Value()) the one base-value read used by EvaluateBuyRules, FencePrice callers, GetSellPrice and appraise

**Suggested fix.** Lift GradedValue (with the spoilage refusal and wear factor) into an instance-value function and replace the item.GetSpec().Value reads in sell_stolen.go, offer.go, mobs.GetSellPrice and appraise.go with it.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
buyrules.go:87 `value := GradedValue(item)`; `git grep GradedValue|QualityValueMultiplier` returns only buyrules.go, repair.go and quality.go. sell_stolen.go:126 `price := FencePrice(item.GetSpec().Value)`. offer.go:47 `actions.FencePrice(itemSpec.Value)`. mobs.go:1176 `value = item.GetSpec().Value`. appraise.go:121 `Worth: %d gold ... spec.Value`. config.yaml:1815 `QualityValueCrude: 0.4`, :1539 `BaubleFenceBuyPct: 60`.
```

- **confirmed** (low): The structural claim holds: grade-based value is applied only by shops.GradedValue, which only EvaluateBuyRules (buyrules.go:87) and repair.go:107 call. GetSpec does not grade Value either; applyGrade (grade_effects.go:81) scales only damage, speed, mitigation, block and weight. So the fence, legacy-merchant and offer paths really do read the ungraded spec.Value.  Three parts of the claim do not hold up, which is why I lowered the severity: 1. **Appraise is miscited.** appraise.go:121 is inside appraiseBauble, which only runs for baubles. GetSpec returns a bauble's spec ungraded, so baubles never carry a grade. The paid non-bauble appraise path (appraise.go:69-86) prints no Worth from spec.Value. There is no graded appraise-vs-sell mismatch at the cited line. 2. **The fence scenario cannot happen today.** The only source of stolen goods is merchant chests, and merchantchests/restock.go:162 builds them with items.New(pool[j]) and then sets StolenFrom and StolenFromMob. No grade is ever set on them, so a stolen graded pelt cannot exist. "Fence pays 3x the honest price for a stolen crude pelt" does not reproduce. 3. **The legacy GetSellPrice path is narrow.** sell.go:395, sell.go:158 and offer.go:90 only use GetSellPrice when GetShopInventory returns nil, which happens only when no shop file exists on disk. That path can mis-price a graded item, but only at a merchant with no living-economy inventory. Whether any shipped merchant is like that depends on runtime data.  What remains real: the design inconsistency (no single graded instance-value seam) and the latent fence and offer divergence if graded goods ever become stealable. That is a valid unification note of low severity, not a medium defect that occurs today.

</details>

<a id="f107"></a>
### F107 [low] Hardcoded balance numbers in the new harvest/spoilage code despite the config knob convention

`internal/species/harvest.go:93` · status **confirmed** · reported as low

Most gathering numbers were properly made Balance knobs, but several tuning constants were left as Go literals: the body-size yield scale (small x0.5, large x2), the spoiled-goods price floor (FreshnessValueMultiplier 0.5 + 0.5*freshness), the rare-notice clamps (0.02/0.9), and the targeted-harvest +1 grade bump. The size scale in particular duplicates the size axis that already has knobs for difficulty (GatherSizeDifficultyMedium/Large) and duration (GatherJobRoundsSmall/Medium/Large), but not for yield.

**Failure scenario.** The owner retunes carcass economy from config.yaml (the documented way). Yield per body size, the rotten-goods price floor and rare-part bounds cannot be changed without a code change and redeploy, while the neighbouring difficulty and duration knobs can.

**Existing mechanism.** Balance knobs in internal/configs/config.balance.go / config.balance.gathering.go surfaced via _datafiles/config.yaml

**Suggested fix.** Add GatherSizeYieldSmall/Large, SpoilPriceFloor, GatherRareChanceMin/Max (and optionally GatherTargetedGradeBonus) to config.balance.gathering.go, with legal-zero handling as the file already documents, and ship them in config.yaml.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
species/harvest.go:93 `f *= 0.5` / :95 `f *= 2`; items/spoilage.go:63 `return 0.5 + 0.5*i.Freshness(now)`; actions/harvest.go:249 `math.Max(0.02, math.Min(0.9, chance))`; actions/harvest.go:362-363 `if opts.Part != \`\` && grade < items.QualityPristine { grade++ }`.
```

- **confirmed** (low): The claim checks out. All four literals are in the PR head and appear exactly as cited. None of them reads a Balance knob. Meanwhile config.balance.gathering.go does add knobs on the same axes: size difficulty (GatherSizeDifficultyMedium/Large defaulting to 5/15), job rounds by size (GatherJobRoundsSmall/Medium/Large defaulting to 2/4/6), targeted difficulty (GatherTargetedDifficulty) and the rare base chance (GatherRareBaseChance). So yield by body size, the freshness price floor, the rare-notice clamp and the targeted grade bump are the only gathering tunables that cannot be changed from config.yaml. That goes against the dogmud-balance-config skill rule: "Before hardcoding any balance number, check whether a knob already exists". Low severity is right. Nothing is functionally wrong. The cost is lost tunability, and changing these values needs a code change and a redeploy.

</details>

<a id="f134"></a>
### F134 [medium] Player sell path bypasses shops.PricingBaseline for RestockQty==0 entries with a new WalkInBuyPrice curve, diverging from every NPC valuation path

`internal/shops/buyrules.go:98` · status **refuted** · reported as medium

shops.PricingBaseline is documented at baseline as 'the single normalizer ... used by EVERY pricing path, player buy/sell AND the NPC craft / salvage / gear-upgrade decision paths'. It handles RestockQty==0 goods explicitly via DefaultBaselineQty. This range changes EvaluateBuyRules so an entry with RestockQty 0 no longer goes through CalcBuyPrice(PricingBaseline). It goes through a new linear WalkInBuyPrice (value*BuyRatio*(1 - ShopWalkInDevaluePerUnit*current)). A later in-PR fix then caps that with a CalcBuyPrice(PricingBaseline) limit, so two curves now set one price. The NPC paths still value the same RestockQty-0 entries on the scarcity curve: craftdecision.go:42/64/207/228, planners/shop_upgrade.go:62, aicompanion/economy.go:98, and usercommands/list.go:156. That is the player-vs-NPC divergence PricingBaseline was introduced to remove.

**Failure scenario.** A shop holds 2 crafted units (RestockQty 0) of an item. The NPC craft decision (craftdecision.go:42) values its output at CalcSellPrice over PricingBaseline=3, which is the scarcity curve. A player selling the same item is paid WalkInBuyPrice, a flat ~0.5x slide. The shop's 'should I craft this' and 'what will I pay for it' answers now come from different curves. Retuning ShopWalkInDevaluePerUnit moves only the player side, which recreates the drift class the PricingBaseline doc describes.

**Existing mechanism.** shops.PricingBaseline + shops.CalcBuyPrice/ScarcityMultiplier (internal/shops/pricing.go)

**Suggested fix.** If walk-in goods need a gentler slope, change it once inside PricingBaseline/ScarcityMultiplier (e.g. a separate baseline for RestockQty==0) so player sell, list, NPC craft and companion economy all move together. Do not fork the curve in EvaluateBuyRules alone.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
baseline internal/shops/pricing.go:52-58: 'PricingBaseline is the single normalizer ... used by EVERY pricing path ... the baseline for crafted / caravan-delivered RestockQty==0 goods'. head buyrules.go:98 `if entry != nil && entry.RestockQty > 0 { price = CalcBuyPrice(value, entry.Current, PricingBaseline(entry, cfg), cfg) } else { ... price = WalkInBuyPrice(value, current, cfg)` (line 113), then line 118 `if limit := CalcBuyPrice(value, current+1, PricingBaseline(entry, cfg), cfg); price > limit`. `git grep PricingBaseline(` shows craftdecision.go, shop_upgrade.go, aicompanion/economy.go and list.go unchanged.
```

- **refuted** (low): The code facts are accurate. At head, buyrules.go:98 sends RestockQty==0 entries to WalkInBuyPrice, and line 118 caps that price with CalcBuyPrice(value, current+1, PricingBaseline(entry,cfg)). The claimed defect does not hold, though.  1. Every NPC path the claim cites computes the shop's SELL price with CalcSellPrice: craftdecision.go:42/64/207/228, shop_upgrade.go:62, aicompanion/economy.go:98, list.go:156 and buy.go:560. None of them computes what the shop pays a player. The PR changes only the shop's BUY price, so no NPC buy-price path exists for it to diverge from. The failure scenario compares CalcSellPrice on the craft side with the buy price on the player side. Those were always two different functions with different ratios, both before and after the PR.  2. EvaluateBuyRules is the only buy-price path. Its other caller, auctions/npc_buyers.go:224, calls EvaluateBuyRules directly, so the NPC auction buyers get the same walk-in price. NPC and player buy prices therefore agree.  3. What PricingBaseline unified is the scarcity denominator, replacing the old MaxStock/2 estimates per call site. Every remaining scarcity-curve call, including the new cap, still gets its denominator from PricingBaseline. The baseline also already had a flat, non-scarcity buy path for unstocked items (nil entry, value*BuyRatio). The PR extends that same flat path to RestockQty-0 entries and adds a per-unit slide. It does not invent a rival normalizer.  4. "Retuning ShopWalkInDevaluePerUnit moves only the player side" is wrong. No separate NPC buy side exists; the auction buyers go through the same function.  One small real issue remains. The PricingBaseline doc comment in pricing.go:52-58 says it is "used by EVERY pricing path". pricing.go is not in the PR diff, so that comment is now slightly stale for the walk-in buy price, which uses PricingBaseline only as a cap. The shops context.md was updated to describe WalkInBuyPrice and the cap, so the main remaining issue is the stale doc comment, which is low severity. The medium-severity unification defect is refuted.

</details>
