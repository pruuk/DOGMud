# Crafting Package Context

## Overview

The `internal/crafting` package implements the data-driven recipe and
crafting framework (Stage 13.1), the corpse-salvage lookup table
(cooking supply chain, chunk 5.4), and the enchanting-mat salvage
mapping (enchanting supply chain, chunk 5.4). It is a pure-logic
package: it owns no persistent state and emits no side effects —
callers in `internal/usercommands/` and `internal/hooks/` drive the
actual item transfers and player messaging.

Three distinct systems live here:

1. **Recipe crafting** — players and NPC crafters combine tagged
   ingredients at optional stations to produce output items.
2. **Corpse salvage** — salvaging a mob corpse yields materials
   determined by the mob's group membership (rodent → wild-hare-meat;
   animal → raw-meat; humanoid → cloth/leather).
3. **Enchant salvage mapping** — salvaging a spoiled/decayed potion
   yields enchanting materials; the tier of materials is keyed by the
   source potion's alchemy-recipe `skill_minimum`.

The corpse-salvage system feeds the cooking supply chain: foragers
salvage animal/rodent corpses, the yielded meat flows into forager
lockboxes, and `BackfillVendorFromChests` (in `internal/forager/`)
drains those chests into cook-vendor stock.

## Key Components

### Core Files

- **crafting.go** — `RecipeSpec` struct + `fileloader.Loadable`
  implementation; `LoadRecipes`, `GetRecipe`, `GetRecipesForSkill`,
  ingredient/output access helpers.
- **validation.go** — `ValidateRecipe` checks ingredient tags against
  the item registry; called at load time and by integration tests.
- **difficulty.go** — the craft/salvage CONTEST (U10b-1b): `CraftScore`,
  `CraftDifficulty`, `CraftPrimaryStat`, `DearestMaterialTier`,
  `SalvageDifficulty`, `FallbackSalvageDifficulty`, and the two floor
  seams `RunCraftContest` / `RunSalvageContest`.
- **salvage.go** — `CalcSalvageRounds` (duration from ingredient gold
  value), `RollSalvageReturns` / `RollSalvageReturnsFromSpec` (per-unit
  CONTEST over an ingredient list).
- **corpse_salvage.go** — static `corpseSalvageTable` + public
  `LookupCorpseSalvage(groups []string) []items.SalvageReturn`.
- **enchant_salvage_map.go** — `EnchantSalvageYield` /
  `EnchantSalvageYieldWith`; tiered potion→enchanting-mat mapping
  keyed by recipe `skill_minimum` (see below).
- **crafting_test.go** — recipe-load + GetRecipe unit tests.
- **salvage_test.go** — CalcSalvageRounds / RollSalvageReturns unit tests.
- **difficulty_test.go** — the anchor, the mastery curve against the
  spec's table, tier determinism, and the SkillWeight/CraftSkillMinWeight
  coupling guard.
- **corpse_salvage_test.go** — LookupCorpseSalvage unit tests
  covering each table entry + no-match + first-entry-wins ordering.
- **enchant_salvage_map_test.go** — `EnchantSalvageYieldWith` table-
  driven tests covering each band boundary, roll hits/misses, the
  band-4 all-miss binding-paste floor, and the `qtyBonus` path.
- **validation_test.go** — ValidateRecipe unit tests.
- **integration_crafting_test.go** — end-to-end recipe-load
  integration test (loads YAML from disk).
- **test_main_test.go** — test-binary setup (data-file path
  initialization).

## Key Functions

### Recipe Crafting

- **`LoadRecipes(dir string) error`** — walks `dir` recursively,
  loads all `*.yaml` recipe files, validates each, populates the
  in-process registry. Called from `main.go` at startup.
- **`GetRecipe(id string) (*RecipeSpec, bool)`** — look up a recipe
  by its string id.
- **`GetRecipesForSkill(skill string) []*RecipeSpec`** — return all
  recipes belonging to a skill tag (e.g., `"cooking"`), sorted by
  name for stable UI ordering.

#### The three ingredient questions, and which one to ask

All three share one tag matcher (`componentTagOf`) and answer in recipe
order, so they never disagree about WHICH tag is short. They differ only in
what they are allowed to count. `componentTagOf` gives hot stolen goods
(a merchant chest's, `baubles.GoodsHot`, read through the `stolenNow`
clock) no tag at all, so no count, selection, storage plan or consumption
uses one while it is hot: crafting it into clean output would shed its
heat. Cooled, it is an ordinary material.

- **`HasIngredients(inv, componentInv, recipe) (bool, string)`** counts
  only what the actor CARRIES. This is what `actions.InitiateCraft` asks,
  and it has to be: `InitiateCraft` is shared with mobs, which have no
  storage. Its `MissingTag` is therefore a carried-only answer and must not
  be printed to a player as-is.
- **`PlanStoragePull(recipe, inv, componentInv, storage) ([]items.Item, bool)`**
  gives the exact storage items to pull, and whether pulling them makes the
  recipe craftable. **All-or-nothing by owner ruling**: if storage cannot
  cover the whole shortfall it returns `(nil, false)` and nothing moves,
  including the part it could have covered. Do not change that. Its
  completeness loop ranges a MAP, which is fine for a boolean and cannot
  pick a name.
- **`HasIngredientsWithStorage(inv, componentInv, storage, recipe) (bool, string)`**
  is `HasIngredients` with storage counted too, returning the first tag in
  RECIPE order that neither the actor nor storage can supply. 🐛 **Every
  player-facing "you are missing X" must come from this one.** `craft
  setting` shipped to prod saying "You are missing: copper-wire." to a
  player with 39 in the bank, because the all-or-nothing pull moved nothing
  and the refusal fell back to the carried-only answer, whose first short
  tag was the one they had. Player command paths recompute with it in
  `internal/usercommands/craft.go` (`storageAwareMissingTag`,
  `recipeStatus`), which is where `user.ItemStorage` is known.

`planAgainstStorage` is the single private traversal behind the latter two,
so the shortfall is computed in exactly one place.

### Salvage Math

🔴 **`CalcSalvageChance` and `CalcSuccessChance` were DELETED by U10b-1b.**
Craft and salvage are contests now, not flat percentages. The knobs that
fed them (`SalvageMinChance`, `SalvageMaxChance`, `SalvageSoftCap`,
`CraftingBaseSuccessChance`, `CraftingSkillBonusPerLevel`,
`CraftingMin/MaxSuccessChance`) still exist in config and are still
validated, but **decide nothing** — do not pin them in a test expecting
an outcome.

- **`CraftScore(stat, skillLevel)`** — `stat + skill × SkillWeight`, the
  standard composition. `stat` is the DISCIPLINE'S primary, which varies:
  blacksmithing STRENGTH, alchemy/cooking/enchanting PERCEPTION,
  tailoring/jewelcrafting DEXTERITY. Use `CraftPrimaryStat(recipe)`.
- **`CraftDifficulty(skillMinimum, materialTierMult)`** —
  `(CraftBaseDifficulty + skillMinimum × CraftSkillMinWeight) × tierMult`.
  ⚠️ The 50/50 anchor holds only while `SkillWeight == CraftSkillMinWeight`;
  there is a guard test for that coupling.
- **`DearestMaterialTier(consumed []items.Item)`** — max `MaterialTier`
  over the CONCRETE items being spent. 🔴 Never resolve by
  `component_tag`: `items.FindSpecByComponentTag` iterates a Go map and
  four items share the tag `bottle`.
- **`SelectIngredients(inv, componentInv, recipe)`** (crafting.go) — the
  items `ConsumeIngredients` would take, in the same order (component bag
  first). That ordering IS the bottle tiebreak.
- **`RunCraftContest` / `RunSalvageContest`** — the ONLY readers of
  `CraftFloor` / `SalvageFloor`. Call these, never `contest.RunWithFloors`
  directly, so a site cannot be handed the wrong floor. `SalvageFloor`
  ships at 0.15 in `_datafiles/config.yaml`, matching its Go default
  (`config.balance.shops.go`'s validator self-heals any `<= 0` value back
  to 0.15). This is the ONE knob that actually tunes salvage's mercy
  band today; see the Gotchas section below for the three that do not.
- **`SalvageDifficulty(itemId, tierMult)`** — the item's own craft
  difficulty, read from the recipe that PRODUCED the item being taken
  apart (`GetRecipeByOutputItemId`); `ok=false` when it has no recipe.
  The live caller (`actions.salvageItem`) always passes `tierMult=1.0`
  (neutral), deliberately: the material tier would have to come from the
  recipe's own ingredients, which do not exist yet at salvage time.
- **`FallbackSalvageDifficulty()`**: prices something with no recipe at
  all, a corpse, or an item carrying `salvage_returns`. Returns
  `CraftBaseDifficulty` untouched. Deliberately untuned; the corpse path
  is the only one that is reachable today (see `SalvageReturns` gotcha
  below).
- **`CalcSalvageRounds(totalGoldValue, goldPerRound, maxRounds)`** —
  duration = `max(1, min(maxRounds, goldValue / goldPerRound))`.
- **`RollSalvageReturns(ingredients, score, difficulty)`** /
  **`RollSalvageReturnsFromSpec(returns, score, difficulty)`** — one
  `RunSalvageContest` per unit of every ingredient (or every tagged
  `SalvageReturn`); returns only recovered items. The two exist because a
  recipe's ingredients and an item's `SalvageReturns` are different
  slice types; both feed the same contest.

**How a salvage actually resolves (`internal/actions/salvage.go`).**
`usercommands.Salvage` (player `salvage <item>` command) starts a
multi-round `activity.Salvaging` state and resolves the roll on the final
tick by calling `actions.Salvage(actor, opts)`, the single entry point
both the player path and mob idle-salvage paths share. It composes the
salvager's SCORE once, up front: `CraftScore(perception,
salvageSkillLevel)` (salvage's primary stat is perception, per
`skills.SkillPrimaryStats`), scaled by the Provident Hands mutation via
`salvageScoreWithMutations`. It then dispatches to `salvageItem` (by
item UUID) or `salvageCorpse` (room corpse), each of which resolves its
own DIFFICULTY (`SalvageDifficulty` for a recipe-backed item,
`FallbackSalvageDifficulty` for a corpse or a `salvage_returns` item) and
rolls `RunSalvageContest(score, difficulty)` once per unit via
`RollSalvageReturns` / `RollSalvageReturnsFromSpec`. `storeRecovered`
then creates and stores whatever came back. A spoiled or declining
potion is a separate branch entirely: it skips the contest and calls
`EnchantSalvageYield` instead (see Enchant Salvage Mapping below).

### Enchant Salvage Mapping

- **`EnchantSalvageYieldWith(skillMin int, roll func() float64, qtyBonus int, b EnchantSalvageBands) []RecipeIngredient`**
  — pure, testable core. Maps a potion's recipe `skill_minimum` to a
  slice of `RecipeIngredient` (item tags + quantities) using four
  bands:

  | Band | `skillMin` threshold | Guaranteed output | Possible extras |
  |------|----------------------|-------------------|-----------------|
  | 1    | < `Band2Min` (10)    | binding-paste ×(1+bonus) | — |
  | 2    | ≥ 10                 | binding-paste ×(1+bonus) | chrysalis-setting (25%) |
  | 3    | ≥ 18                 | binding-paste ×(1+bonus) | chrysalis-setting (35%), mutation-catalyst (12%) |
  | 4    | ≥ 28                 | binding-paste fallback if all miss | mutation-catalyst (40%), chrysalis-setting (30%), chrysalis-core (8%) |

  Band 4 rolls each rare mat independently; if every roll misses the
  function still returns one binding-paste (the guaranteed floor).
  `qtyBonus` (0+) increases the binding-paste quantity in bands 1-3;
  pass 0 when the NPC decay path calls it, non-zero for a skilled
  player salvage bonus.

- **`EnchantSalvageYield(potionItemId int, roll func() float64, qtyBonus int) []RecipeIngredient`**
  — live wrapper. Resolves the potion item ID to its alchemy-recipe
  `skill_minimum` via `GetRecipeByOutputItemId`; falls back to 0
  (band 1) for unknown/no-recipe potions. Reads band thresholds and
  percentages from `configs.GetBalanceConfig()`. Delegates to
  `EnchantSalvageYieldWith`.

  Config knobs (all in `Balance`, with defaults):
  - `EnchantSalvageBand2Min` (10), `Band3Min` (18), `Band4Min` (28)
  - `EnchantSalvageBand2SettingPct` (25)
  - `EnchantSalvageBand3SettingPct` (35), `Band3CatalystPct` (12)
  - `EnchantSalvageBand4CatalystPct` (40), `Band4SettingPct` (30),
    `Band4CorePct` (8)

  **Shared callers.** The same function drives two paths:
  1. *Player spoiled-potion salvage* — `usercommands/salvage.go` calls
     it when the target corpse is a potion item.
  2. *NPC alchemy-decay loop* — the shop restock hook
     (`hooks/MobIdle_HandleIdleMobs.go`) iterates `DecayedUnit` slices
     returned by `shops.TickOverstockDecay`; for each decayed potion
     it calls `EnchantSalvageYield` and routes the resulting mats into
     the `shops.AddToReserve` global pool. Enchanters then draw from
     that pool on their idle tick via `shops.SelectStockTransfers`.

### Corpse Salvage

- **`LookupCorpseSalvage(groups []string) []items.SalvageReturn`** —
  scans `corpseSalvageTable` in declaration order; returns the
  `Returns` slice of the first entry whose `Group` string appears in
  the mob's groups list. Returns `nil` when no entry matches (e.g.,
  elementals, chrysalis mobs).

  Table order (specific before broad):

  | Group | Yields |
  |-------|--------|
  | `rodent` | wild-hare-meat ×1, leather-strip ×1 |
  | `animal` | raw-meat ×1, leather-strip ×2, sinew ×1 |
  | `humanoid` | cloth-strip ×2, leather-strip ×1 |

  To add a new mob category, prepend its entry before `animal` if it
  should take priority over the generic-animal row, or append after
  `humanoid` for unmatchable fallbacks. The `rodent` entry sits before
  `animal` so small-game mobs with both groups match the narrow row.

## Narration (M3 item 6)

A recipe is a narration store. Read its text only through the door in
`narration.go`; `store_text_fields_guard_test.go` fails on a read of
`SuccessMessage`, `FailureMessage`, `SuccessRoomMessage` or
`FailureRoomMessage` anywhere else.

- `Phase`: `PhaseSuccess`, `PhaseFailure`.
- `(*RecipeSpec).Narration(p) narration.Variants`: `success_actor` /
  `failure_actor` are the Actor (the crafter); `success_observer` /
  `failure_observer` are the Observer (the room). No Actee: a craft has no
  second party. Enchanting another player's gear would add one. M4b-1 renamed
  all four from `success_message` / `success_room_message` and their failure
  siblings: the outcome half stays, the role half is now the canonical
  vocabulary every narration store shares.
- `(*RecipeSpec).Narrate(p, textutil.TokenContext) narration.Roles`: renders
  through `textutil.Narrate`, crafter as the actor (`{actor}`,
  `{actor_plain}`).
- `(*RecipeSpec).MobRoomLine(p, mobName, fallback) string`: a mob crafter's
  room line, the authored Observer line or the fallback.
- `(RecipeSpec).ValidateNarrationTokens() []string`: unknown `{token}`
  spellings across all four narration fields, via `textutil.ValidateTokens`.
- `Validate` refuses: an empty `success_actor` or `failure_actor`; a
  whitespace-only line; a room message without `{actor}`; an unknown token
  (M4a, matching conditions and spells). The loader panics on the error, so a
  typo cannot ship and render raw to the player.

Room messages are empty in all 126 shipped recipes until M6 authors them; a
player craft with none sends nothing to the room.

## Global State

- **`recipeRegistry map[string]*RecipeSpec`** — in-process map keyed
  by `RecipeSpec.RecipeId`; populated at startup by `LoadRecipes`.
  Read-only after load; no mutex needed.
- **`corpseSalvageTable []corpseSalvageEntry`** — package-level
  slice; declared statically in `corpse_salvage.go`, never mutated
  at runtime.
- **`EnchantSalvageBands`** — plain Go struct (no package-level
  state); constructed on each call to `EnchantSalvageYield` from the
  live balance config. No mutex needed; all state is stack-local.

## Data Structure Design

### RecipeSpec (YAML)

```yaml
id: grilled-meat
name: Grilled Meat
skill: cooking
skill_minimum: 0
station: ""          # "" = no station required
time_rounds: 2
ingredients:
  - item_tag: raw-meat
    quantity: 1
output:
  item_id: 40060
  quantity: 1
success_actor: "You grill the meat over the fire."
failure_actor: ""
```

Optional enchanting fields: `target_type`, `enchant_type`.

### corpseSalvageEntry (Go struct, not YAML)

```go
type corpseSalvageEntry struct {
    Group   string
    Returns []items.SalvageReturn  // {ItemTag string, Quantity int}
}
```

## Gotchas

- **`SalvageMinChance`, `SalvageMaxChance`, `SalvageSoftCap` are DEAD
  knobs.** They are declared in `config.balance.go`, defaulted in
  `config.balance.misc.go`, shipped in `_datafiles/config.yaml`
  (0.15 / 0.85 / 50), and read by NOTHING else in the tree: grep the repo
  for any of the three names outside those two `config.balance.*.go`
  files and it comes back empty. U10b-1b replaced the sqrt-curve chance
  they fed with the `RunSalvageContest` contest above, and the knobs were
  left behind rather than deleted. Tuning any of the three has zero
  effect on live salvage; the one knob that does anything is
  `SalvageFloor` (see Salvage Math above). This is the same shape as the
  `MobStatCap` / `MobSkillCap` trap documented in the
  `dogmud-progression-model` skill: a legacy knob that is still declared,
  still validated, and still looks plausible to a reader, but enforces
  nothing.
- **The item (or corpse) is consumed whether or not anything is
  recovered.** `salvageItem` calls `char.RemoveItem(targetItem)`
  unconditionally, before checking whether `recovered` is non-empty
  (`internal/actions/salvage.go`, "Always destroy the item"); `salvageCorpse`
  calls `room.RemoveCorpse(target)` the same way. The comment at the
  corpse site puts it plainly: destruction is the COST of the attempt,
  not its outcome. A salvage that returns nothing still trains the skill
  (one `AwardResolved` per command, win or lose) and still destroys the
  target.
- **`salvage_returns` entries silently do nothing if `item_tag` does not
  match a real `component_tag`.** Recovery resolves the tag through
  `items.FindSpecByComponentTag`; `storeRecovered` skips a `nil` spec
  with no warning. A typo'd tag is a content bug that produces no error,
  only a player who recovers less than the recipe promised.
- **The `salvage_returns` item path is UNREACHABLE today, not merely
  untested.** Zero items in `_datafiles/world/dogmud/items/` carry
  `salvage_returns`, so `SalvageDifficulty`'s `ok=false` branch (an item
  with no recipe) and `salvageItem`'s `RollSalvageReturnsFromSpec` call
  never actually fire; every live item salvage goes through the
  recipe-reverse-lookup branch instead. This is content-shaped, not
  code-shaped: the corpse path already exercises
  `RollSalvageReturnsFromSpec` and `FallbackSalvageDifficulty` every day
  via `LookupCorpseSalvage`, so only the item-tagged variant is dormant,
  waiting for content to give it something to do.

## Integration Notes

**Consumers of recipe crafting:**
- `internal/usercommands/craft.go` — player `craft` command; calls
  `GetRecipe`, resolves ingredients from backpack + equipped items,
  drives the multi-round activity, calls `RollSalvageReturns` for
  salvage recipes.
- `internal/hooks/TickMobCraft*.go` — NPC crafter tick; calls
  `GetRecipesForSkill` to pick recipes, manufactures items into the
  NPC's shop stock.

**Consumers of corpse salvage:**
- `internal/usercommands/salvage.go` — player `salvage <corpse>`
  command; calls `LookupCorpseSalvage(mob.Groups)`, then
  `RollSalvageReturns`.
- `internal/actions/salvage.go` — shared `actions.Salvage` called by
  both player and mob salvage paths; same lookup chain.

**Consumers of enchant salvage mapping:**
- `internal/usercommands/salvage.go` — when the salvaged target is a
  potion item, calls `EnchantSalvageYield` to determine mat returns.
- `internal/hooks/MobIdle_HandleIdleMobs.go` — the shop restock hook
  iterates `[]shops.DecayedUnit` returned by `shops.TickOverstockDecay`
  for alchemy-vendor mobs (Ilsa's, Voss's); for each decayed potion it
  calls `EnchantSalvageYield` and feeds results into
  `shops.AddToReserve`.

**Upstream dependencies:**
- `internal/items` — `SalvageReturn`, `ItemSpec`, `component_tag`
  resolution.
- `internal/fileloader` — generic YAML walker used by `LoadRecipes`.
- `internal/configs` — balance knobs read by callers (not by this
  package directly).

## Testing Notes

- All pure-math functions (`CalcSalvageRounds`, the difficulty.go helpers,
  `RollSalvageReturns`) are covered by table-driven unit tests in
  `salvage_test.go`.
- `LookupCorpseSalvage` tests in `corpse_salvage_test.go` cover each
  table row, the no-match case, nil/empty input, and the
  first-entry-wins ordering guarantee (a mob with both `animal` and
  `humanoid` groups gets the `animal` row; a mob with both `rodent`
  and `animal` groups gets the `rodent` row).
- `integration_crafting_test.go` loads YAML from disk — requires
  the test binary to be run from the repo root or with the data path
  initialized by `test_main_test.go`.
- Adding a new `corpseSalvageTable` entry requires a matching test
  case in `corpse_salvage_test.go`.
- `enchant_salvage_map_test.go` covers all four band boundaries, roll
  hit vs miss for each probabilistic mat, the band-4 all-miss
  binding-paste floor, and `qtyBonus` stacking. Use
  `EnchantSalvageYieldWith` (inject a deterministic roll) for new
  tests; never call the live `EnchantSalvageYield` in unit tests
  (requires a loaded balance config).

## Output quantity

`output.quantity` is honoured on every craft path (players, the shared
action, NPC crafters) through `RecipeSpec.OutputCount` (at least 1).
Salvage gives back one unit's share (`RecipeSpec.SalvageIngredients`): a
recipe that makes n gives each unit ingredient/n, the remainder as a chance,
so craft-then-salvage cannot multiply a material (three Obelisk Lenses from
one Obelisk Glass; three chain links from their bar).

## Obelisk glass armour

`obelisk-lens` (jewelcrafting 35): 1 Obelisk Glass (40234, foraged from rift
seams) → 3 Obelisk Lenses (40235). Lens-scaled versions of the top steel
pieces (blacksmithing, forge), each the original recipe plus lenses:
`glass-scale-hauberk` (45; chainmail vest + 24 lenses → 20099),
`glass-scale-helm` (48; masterwork plate helm + 12 → 20100),
`glass-facet-buckler` (42; steel buckler + 15 → 20101). Harder, a third of
the weight, escape modifier 0.5, and `return_damage` 5/3/4 (12% for the
set, physical, mitigated by the attacker's armour).
