# PR #208 review: craft wood tools gear, correctness lens

Blind reviewer `craft_wood_tools_gear:correctness`, run 2026-10-05 against PR head `3ce674451`. Findings were then checked by independent verifiers (three for critical and high, one otherwise). Severity is the verifiers' median. Back to the [synthesized report](../README.md).

## Reviewer summary

I read the timber package, chop and survey, tool wear and the tool ladder, grade effects in GetSpec, bow and arrow wood traits, harvest min_tool trophies, sickle foraging, crafted furniture, room spoilage, shop reconcile and the carpentry-to-woodwork migration, all at the PR head and scoped by 46766d13b..52cfbeda4. I found no critical or high-severity bug. Regrow and felling arithmetic and stand persistence hold up. Nothing in the main paths dereferences nil. Enchant and affix overrides build from GetRawSpec, so a grade is never applied twice; I checked every assignment of the form `.Spec = &`. The wood traits stay on the weapon pointer from reload to fire. What I did find is five medium or low issues: balance numbers hardcoded in Go rather than in config, timber.yaml biome keys that are never validated, chop revealing a tree's species that the survey hides, migration code for a skill and recipe that never shipped to master, and extra cost added to the item.GetSpec hot path.

## Coverage

I read in full at the PR head: internal/timber/{timber,stand,wood}.go, internal/actions/chop.go, internal/usercommands/chop.go, internal/gather/tools.go, gather.go (Roll, gradeFrom, CraftGradeOutput), internal/items/tools.go, grade_effects.go, and the items.go GetSpec/GetRawSpec diff. I also read: actions/craft.go (ToolSatisfied through JobLeftBehind, plus the instant path), toolwear.go, harvest.go (planHarvest and ResolveHarvest), the forage.go and forage_core.go diffs, the combat_fire.go and combat_reload.go diffs, the stacking.go diff, the shops persistence/reconcile, sell.go, craftdecision.go and caravan diffs, housing crafted.go and use_items.go, rooms/spoilage.go, the characters/validate.go migration, the config.balance knobs and validators, the main.go loader wiring, and the UserRoundTick salvage and crafting branches. I checked that every `.Spec = &` writer (enchantments, affixgen, AddWornCondition, Rename) builds from the raw spec, so grades cannot compound, and that MigrateDetunedBow reads Spec directly. I checked timber.yaml against the zone-config names and biome files. I did not deeply review: the per-item YAML for trophies and hunter gear, or the woodwork recipes beyond five samples; I relied on wilderness_trades_content_test for load validity and did not verify it. I also did not review the later-commit code for crit or shot wear (WearBowOnShot, CritWearWeapon) or repair, which belongs to c45ffe974 rather than this range, the mining package, or the pricing balance of graded gear (economy lens).

## Findings (5)

<a id="f052"></a>
### F052 [medium] Wilderness-trades balance numbers hardcoded in Go instead of Balance config

`internal/actions/chop.go:28` · status **confirmed** · reported as medium

Several tuning numbers in this piece are Go literals, even though the same piece adds about 40 Balance knobs next to them: (1) chopBarkChance = 40, the bark drop percentage (chop.go:28, used at :274); (2) the species-recognition difficulty 100+15*(tier-2) in SpeciesKnown (chop.go:127); (3) the branch yield 1+util.Rand(2) per felling (chop.go:272); (4) toolGradeDurability, the 0.75/1.25/1.5/2.0 durability multipliers by grade (items/tools.go:103-112); (5) WornFraction 0.60 and BadlyWornFraction 0.85 (items/tools.go:233-234), which decide when GearWornMult and GearBadlyWornMult take effect; and (6) the plus or minus one tier shift for pristine and crude in EffectiveToolTier. The config.yaml comment even names the grade-durability numbers ('scaled by the instance grade (crude 0.75 .. pristine 2.0)'), but no knob controls them. The project rule is that balance numbers come from config.yaml and internal/configs/config.balance.go.

**Failure scenario.** The owner wants rare bark to drop less often, worn-gear penalties to start later, or pristine tools to last 1.5x instead of 2x. None of that can be done with a config edit; each needs a code change and a deploy, which is exactly what the Balance contract is meant to prevent. Some of these numbers also sit beside knobs that look as though they control them (ToolDurability* is configurable but the grade scaling on it is not), so someone retuning will reasonably think they changed something they did not.

**Existing mechanism.** internal/configs/config.balance.go Balance struct plus config.balance.gathering.go validators (the same files this PR already extends with Timber*, ToolDurability*, GearWornMult and similar)

**Suggested fix.** Add Balance knobs (for example TimberBarkChance, TimberRecogniseBase/Step, TimberBranchMax, ToolGradeDurability{Crude,Fine,Superb,Pristine}, GearWornAt/GearBadlyWornAt), give them validator defaults, and add them to _datafiles/config.yaml.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
chop.go:28 `const chopBarkChance = 40`; chop.go:127 `return contest.AgainstDifficulty(score, 100+15*float64(sp.Tier-2)).Success`; chop.go:272 `give(branch.ItemId, 1+util.Rand(2))`; items/tools.go:103-112 `func toolGradeDurability(q Quality) float64 { ... return 0.75 ... return 2.0`; items/tools.go:233-234 `WornFraction = 0.60` / `BadlyWornFraction = 0.85`. config.balance.go comment: 'scaled by the instance grade (crude 0.75 .. pristine 2.0)', and no matching field exists.
```

- **confirmed** (medium): I checked all six cited literals in the PR head, and each one is as described. tools.go and chop.go are new files in this PR (tools.go is not present at c696c117a), so none of these numbers are older code being carried over. In the same PR, the Balance struct gains Timber*, ToolDurability*, RareToolMult* and Gear*Mult knobs right next to them, but none of those knobs control the cited values.  The species-recognition literal makes the case stronger than the claim does. The PR adds a TimberTierDifficulty knob (default 15, shipped as 15 in config.yaml) and uses it for the chop difficulty at chop.go:92. SpeciesKnown at chop.go:127 hardcodes its own 15 per tier instead. An owner who retunes TimberTierDifficulty would change the felling difficulty but not the recognition difficulty, which is the misleading near-miss knob the claim describes.  The config.balance.go:650 comment does name 'crude 0.75 .. pristine 2.0', and no field holds those values. This breaks the project's documented rule that balance numbers come from config.yaml. It is not a runtime correctness bug, so medium (tunability and consistency) is appropriate.

</details>

<a id="f108"></a>
### F108 [low] Dead carpentry/carpenters-workbench migration for names that never shipped; reconcileShop now overwrites any differing saved CraftSupport

`internal/characters/validate.go:323` · status **confirmed** · reported as low

validateSkillMigrations renames the 'carpentry' skill and the 'carpenters-workbench' recipe, and reconcileShop was widened from 'fill an empty CraftSupport' to 'overwrite whenever it differs from the template', with 'carpentry became woodwork' as the justification. Neither the carpentry skill nor the carpentry recipes exist at the merge base c696c117a; they appear only in this PR's own earlier phase. So no production save or shop file can contain them. The character migration runs on every load forever and does nothing, and the shop change alters a long-standing rule for a case that cannot arise in production.

**Failure scenario.** No player-facing failure today. The cost is maintenance: a permanent per-load migration for names that never shipped, and a changed reconcile rule that will silently overwrite any future runtime or admin-set CraftSupport on a saved shop the next time the shop registers.

**Suggested fix.** Squash the rename inside the PR so the migration block is unnecessary, and keep the narrower 'only when empty' CraftSupport rule unless there is a shipped case it is needed for.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
`git show c696c117a:internal/skills/skills.go | grep -i carp` returns nothing; `git ls-tree c696c117a _datafiles/world/dogmud/recipes/` has no carpentry dir. validate.go:323-345 carpentry and carpenters-workbench block. shops/persistence.go reconcileShop: `if template.CraftSupport != "" && inv.CraftSupport != template.CraftSupport { inv.CraftSupport = template.CraftSupport` (previously `inv.CraftSupport == "" && template.CraftSupport != ""`).
```

- **confirmed** (low): The facts in the claim hold. The merge base of the PR head and master is c696c117a. `git show c696c117a:internal/skills/skills.go | grep -i carp` returns nothing, and no file path at c696c117a contains "carp". The carpentry skill and recipes first appear in e42c5f520 (2026-10-03) and 2ac23d595, which are this PR's own commits. No branch other than the detached PR checkout contains them. So no production save or shop file can hold "carpentry" or "carpenters-workbench".  The migration block at validate.go:323-345 therefore never fires on a production save. The one real exception is a local save made while play-testing this branch.  The reconcileShop rule did change. The baseline (persistence.go:80 and :126 at c696c117a) only filled an empty CraftSupport. The head (persistence.go:162) overwrites whenever the template's value is non-empty and differs from the saved one. Its comment gives "carpentry became woodwork" as the reason.  Two things temper the severity. First, nothing at runtime sets ShopInventory.CraftSupport except this reconcile; a grep for non-test assignments finds only persistence.go:163. That means the claimed overwrite of a "runtime or admin-set" value is hypothetical today. Second, the template is the authored source of truth (mobs/crafter.go:106 sets it from mob.ShopCraftSupport), so following the template is a defensible rule in its own right.  Net effect: a dead migration that costs a few map lookups and a rule change justified by a case that never shipped. These are maintenance issues with no player-facing failure, which fits the "low" rating.

</details>

<a id="f109"></a>
### F109 [low] Item.GetSpec now copies the 568-field Balance config several times per call for graded or worn gear

`internal/items/items.go:346` · status **confirmed** · reported as low

GetSpec is called about 246 times in non-test code, about 58 of them in combat and characters. It now runs applyGrade, which calls GearGradeMults and so GetBalanceConfig, and, when Wear > 0, applyCondition, which calls ConditionMult. ConditionMult reaches WearFraction, then Durability, then ToolDurability or GearDurability; each of those copies the raw ItemSpec, may call GetBalanceConfig, and ConditionMult then calls GetBalanceConfig again. Each GetBalanceConfig takes two RLocks and returns the Balance struct, which has 568 yaml fields, by value. Every weapon and armour piece a player crafts is now always graded (RecipeGrade alwaysGraded), so this cost lands on ordinary equipped gear on every combat read.

**Failure scenario.** In a busy fight on the 1-CPU production droplet, each swing and each mitigation and weight read for crafted gear makes several multi-KB struct copies and lock round trips that did not happen before. This is not a correctness failure, but a hot-path cost multiplied by player count.

**Suggested fix.** Read the few grade and wear knobs once (a cached snapshot refreshed on config reload, or pass the multipliers in), and compute Durability once per GetSpec rather than through WearFraction and ConditionMult separately.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
items.go GetSpec: `spec = applyGrade(spec, i.Quality); spec = applyBowWood(spec, i.Wood); if i.Wear > 0 { spec = applyCondition(spec, i.ConditionMult()) }`. grade_effects.go GearGradeMults: `b := configs.GetBalanceConfig()`. tools.go ConditionMult: `f := i.WearFraction() ... b := configs.GetBalanceConfig()`. GetBalanceConfig returns `configData.Balance` by value under RLock. `awk '/^type Balance struct/,/^}/' ... | grep -c yaml:` gives 568.
```

- **confirmed** (low): The call chain is accurate as written. In the PR head, Item.GetSpec (internal/items/items.go:346) now always runs applyGrade for non-bauble items. applyGrade calls GearGradeMults, which calls configs.GetBalanceConfig() once, but only when the quality is graded and not Standard and the item is a gear type. When Wear > 0, GetSpec also calls i.ConditionMult(). That goes through WearFraction, then Durability, then ToolDurability (one GetRawSpec copy), then GearDurability (another GetRawSpec copy, plus a GetBalanceConfig when spec.Durability <= 0). ConditionMult then calls GetBalanceConfig again. GetBalanceConfig (config.balance.go:1494) runs ensureConfigValidated (one RLock), then takes a second RLock and returns the Balance struct by value. So a graded, worn weapon pays up to 3 Balance copies, about 5 lock round trips and 3 extra ItemSpec copies per GetSpec. The baseline paid none of these. The struct also holds maps and slices, but copying it only copies their headers, so the cost is a flat memcpy of a few KB.  The impact is real but very small, and the reviewer already admits it is not a correctness failure. Each copy costs on the order of 100ns, and MUD combat rounds are seconds apart. Calling GetBalanceConfig per read is also the established pattern here: 82 non-test calls in internal/combat and internal/characters already copy this struct. The one real point is that worn gear, which the PR now produces, takes the longer path, and that path is easy to cache (call GetBalanceConfig once and pass it down, or compute ConditionMult from a single raw spec). Treat it as a low or nit performance cleanup, not a defect.

</details>

<a id="f110"></a>
### F110 [low] timber.yaml biome keys are never validated; a typo silently makes a whole biome unchoppable

`internal/timber/timber.go:144` · status **confirmed** · reported as medium

Parse checks species ids, weights, log and bark items, and zone names (ZoneExists), but it never checks that a key under `biomes:` is a real biome id. RoomStand and Pool look rooms up by exact biome string. A misspelt or renamed biome is accepted at boot, and every room of the intended biome then answers 'There is no timber worth cutting here'. Zone pools depend on the biome key too: Pool returns nil unless current.biomes[biome] exists, so a bad biome key also silently disables the zone overrides for those rooms. The sibling loader validates its zone references with a boot panic, so biome references are the one unchecked link.

**Failure scenario.** A content author writes `dense-forest:` (hyphen) or a biome is later renamed in biomes/*.yaml. Boot succeeds. Every dense_forest room in the world, including the Fernway South ironwood rooms, becomes unchoppable, and nothing in the logs says so. Only a playtest would catch it.

**Existing mechanism.** rooms.GetBiome(id) (info, ok), the biome registry already used by Room.GetBiome

**Suggested fix.** Add `BiomeExists func(string) bool` to timber.World, wire it in main.go as `func(b string) bool { _, ok := rooms.GetBiome(b); return ok }`, and fail Parse on an unknown biome key, as zones already do.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
timber.go:144-148 `for biome, pool := range d.biomes { if err := check(`biome `+biome, pool); err != nil {...} }` checks only pool contents. timber.go:150-155 does check zones: `if w.ZoneExists != nil && !w.ZoneExists(zone) { return nil, fmt.Errorf("zone %q does not exist", zone) }`. World (timber.go:79-82) has no BiomeExists. main.go wires only ItemExists and ZoneExists. Pool(): `if _, ok := current.biomes[biome]; !ok { return nil }`.
```

- **confirmed** (low): The code does what the claim says. Parse (internal/timber/timber.go:144-148) checks only the contents of each biome pool, never whether the biome key is a real biome. World has only ItemExists and ZoneExists. main.go:1906-1909 wires only those two. Pool() and IsChoppable() look the biome up by exact string, and chop.go:42-48 reads room.Biome directly on purpose. So a biome key that does not match the registry loads at boot with no error and leaves those rooms unchoppable. The registry check the claim points to does exist: rooms.GetBiome in internal/rooms/biomes.go:201.  The claim overstates how silent this is. TestTimberContent (wilderness_trades_content_test.go) asserts that timber.IsChoppable returns true for `forest`, `dense_forest` and `swamp`. Those are exactly the three biome keys in the shipped timber.yaml. So the claim's example typo (`dense-forest`) on a shipped key would fail the content test before merge. It would not reach production unnoticed.  Two cases are still uncaught: - A biome renamed in the registry and the room files, while timber.yaml keeps the old key. The test only checks timber's own map, not the registry. - A new fourth biome key added with a typo.  It is a real gap in validation consistency, since zones get a boot panic and biomes do not. But the test narrows it, so the severity drops from medium to low.

</details>

<a id="f111"></a>
### F111 [low] `chop <word>` and the chop narration reveal a species that SpeciesKnown hides from survey

`internal/usercommands/chop.go:46` · status **confirmed** · reported as low

Survey hides rare species (tier 3 and up) behind a Perception and Search roll (SpeciesKnown). Chop then checks the player's word against the real species name, and its mismatch reply echoes the word back. chopWordMatches also accepts any substring, so `chop a` matches 'black walnut'. In the same piece, ResolveChop's AxeTooPoor message and its success and failure narration print sp.Name with no SpeciesKnown check, while the Chop command takes care to hide the name in its own axe-too-poor message. The recognition gate is therefore cosmetic and the two code paths disagree.

**Failure scenario.** A surveyor fails the recognition roll and reads 'an unfamiliar hardwood'. They type `chop yew`, which either starts a job (it is yew) or replies 'There's no yew here'. Trying `chop yew`, `chop walnut` and `chop ironwood` identifies the tree with no roll. Alternatively, if their best axe is too poor at resolve time, ResolveChop prints 'The ironwood is too hard for your axe', naming the species the Chop command avoided naming.

**Existing mechanism.** actions.SpeciesKnown (same piece)

**Suggested fix.** Match only generic words (or the species name only when SpeciesKnown is true), drop the bare substring match, and use the same known or unknown wording in ResolveChop's AxeTooPoor message and narration.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
usercommands/chop.go:46 `if want := ...; want != `` && !chopWordMatches(want, sp.Name) { ... `There's no %s here...`, want`; chop.go:97 `return word == species || strings.HasPrefix(species, word) || strings.Contains(species, word)`; usercommands/chop.go:62-67 hides the name when !SpeciesKnown; actions/chop.go ResolveChop calls `AxeTooPoor(sp)` and narrates `The %s creaks, leans...` with sp.Name and no SpeciesKnown check.
```

- **confirmed** (low): The claim holds as written. In usercommands/chop.go:46, Chop compares the player's word against the real sp.Name before any SpeciesKnown check. On a mismatch it echoes the word back ("There's no %s here"). On a match it starts the job. Either way the player learns whether a guessed species is right. chopWordMatches (line 97) also accepts any substring, so very short words match.  The two code paths do disagree. Chop deliberately falls back to a nameless axe-too-poor message when SpeciesKnown fails (lines 61-67). ResolveChop in actions/chop.go has no such guard: line 232 calls AxeTooPoor(sp), which prints sp.Name, and lines 284-289 narrate success and failure with sp.Name. The failure line ("Your axe keeps glancing off the %s") names the species even when the player gets no logs.  Two things keep the severity low: 1. SpeciesKnown is a fresh, unsticky roll on every survey (actions/chop.go:118-127, contest.AgainstDifficulty with no memo), so typing `survey trees` again already gets past the gate with no guessing. 2. A successful fell hands over sp.LogItemId items, whose names presumably identify the wood anyway.  ResolveChop's AxeTooPoor branch only fires if the best axe changed mid-job, because Chop already refuses a too-poor axe before the job starts. So that path is an edge case. The failure narration path is reached normally.  The gate is cosmetic, and the inconsistency is real but minor.

</details>
