# Shops Package Context

## Overview

The `internal/shops` package implements the living-economy shopkeeper system:
persistent shop state (stock levels, merchant gold), dynamic pricing, restock
cadence, and buy/sell rule evaluation. Shop state survives server restarts and
reflects real NPC and player activity over time.

This package is the storage and pricing tier. Behavior that drives stock
change (forager deliveries, NPC sells, player purchases) lives in
`internal/behaviortree/`, `internal/forager/`, and `internal/actions/`.

## Key Components

### Core Files
- **shopinventory.go**: `ShopInventory` struct + `StockEntry`; `GetStock`,
  `AddStock`, `AddStockAtRound` stock-mutation helpers; `CraftSupport` tag
  constants; `StockEvent` depletion/refill event type.
- **persistence.go**: Disk I/O (`GetShopInventory`, `SaveShop`); in-memory
  shop cache; prewarm helpers.
- **pricing.go**: Dynamic pricing (`GetSellPrice`), `PricingConfig`,
  `PricingConfigFromBalance`; barter helpers.
- **buyrules.go**: `EvaluateBuyRules` — merchant acceptance and price
  negotiation when a seller brings an item to the shop.
- **restock_cadence.go**: Per-tier restock logic; `LastRestockByTier`.
- **overstock_decay.go**: `TickOverstockDecay` / `TickOverstockDecayWith` —
  time-based drain of unsold above-baseline stock; now returns
  `[]DecayedUnit` so callers can act on what was removed (chunk 5.4).
- **enchant_reserve.go**: `AddToReserve` / `ReservePool` / `DrainReserve`
  — global in-memory pool of enchanting mats produced by alchemy-vendor
  potion decay; drawn by enchanters neediest-gap-first (chunk 5.4).
- **stock_transfers.go**: `SelectStockTransfers` — shared neediest-gap-first
  allocator; used by both the forager chest backfill and the enchanter
  reserve draw (chunk 5.4).
- **effective_max_stock.go**: `EffectiveMaxStock` — caps MaxStock when the
  shop is below the gold reserve threshold.
- **craftdecision.go**: `ShouldCraftNow` — evaluates whether the shopkeeper's
  crafter NPC should fire this tick.
- **validation.go**: `ValidateShopMobTags` — startup panic if any shop-bearing
  mob is missing a `craft_support` tag.

## Key Functions

### Stock Management
- **`GetStock(itemId int) *StockEntry`**: Returns the live entry for an item,
  or nil if not stocked.
- **`AddStock(itemId int, qty int)`**: Increases current stock up to MaxStock;
  no depletion-event tracking.
- **`AddStockAtRound(itemId int, qty int, round uint64)`**: Round-aware variant.
  When the item was at 0 before this call, pushes a completed `StockEvent`
  into `StockEvents` history and clears `CurrentDepletion`. Pass `round = 0`
  to skip event tracking.

### Pricing
- **`GetSellPrice(item items.Item) int`**: Base buy-back price for legacy shop
  path (no `ShopInventory`).
- **`PricingConfigFromBalance() PricingConfig`**: Builds a `PricingConfig`
  snapshot from the current balance config; used by both pricing and buy-rules.
  Falls back to `DefaultPricingConfig()` for any field that reads as zero.
- **`PricingBaseline(entry *StockEntry, cfg PricingConfig) int`**: The single
  normalizer (scarcity-curve denominator) used by every pricing path, both
  player buy/sell and the NPC craft/salvage/gear-upgrade decisions. Returns
  `entry.RestockQty` when it's set (`>0`), otherwise `cfg.DefaultBaselineQty`
  (the fallback for crafted/caravan-delivered entries with `RestockQty == 0`).
  Always call this rather than inlining `MaxStock/2`: that old estimate
  priced low-volume `RestockQty == 0` goods near the ceiling and diverged
  from player-facing pricing.
- **`ScarcityMultiplier(current, restockQty int, cfg PricingConfig) float64`**:
  The dynamic-pricing core. Computes `ratio = current / restockQty` and maps
  it onto `[PriceFloor, PriceCeiling]`, shipped **0.25x to 5.0x** (see Config
  Knobs below). At `ratio <= 0` (out of stock) returns `PriceCeiling`; at
  `ratio >= AbundanceThreshold` (fully restocked, or better) returns
  `PriceFloor`; in between it follows an inverse-quadratic curve
  (`PriceFloor + (PriceCeiling - PriceFloor) * (1 - ratio/AbundanceThreshold)^2`),
  so price climbs sharply as stock nears zero rather than sliding down
  linearly. **Abundance is per item, not absolute**: because `ratio` divides
  by `restockQty` (obtained via `PricingBaseline`, above), a low-volume item
  (`restockQty = 1`) hits the price floor at 3 units on hand, while a
  high-volume item (`restockQty = 50`) needs 150 units on hand for the same
  floor price. `ShopAbundanceThreshold` is a multiple of each item's own
  restock quantity, never a fixed stock count.
- **`CalcSellPrice(baseValue, current, restockQty int, cfg PricingConfig) int`**:
  What the NPC charges a player to buy an item:
  `ceil(baseValue * ScarcityMultiplier)`, floored at 1.
- **`CalcBuyPrice(baseValue, current, restockQty int, cfg PricingConfig) int`**:
  What the NPC offers a player for an item:
  `ceil(baseValue * BuyRatio * ScarcityMultiplier)`, floored at 1.
- **`EvaluateBuyRules(item, si, crafterSkill, buysGeneral, cfg, worn) BuyOffer`**:
  Determines whether the merchant will buy an item and at what price. Returns
  `offer.Price > 0` when the merchant accepts.
- **`ApplyBarterSellDiscount(price int, discount float64) int`**: Applies a
  caller-supplied fractional discount to a sell price (buyer side).
- **`ApplyBarterBuyBonus(price int, bonus float64) int`**: Applies a
  caller-supplied fractional bonus to a buy price (seller side). Neither
  barter function reads a config knob itself; see Gotchas.

### Pricing Config Knobs
Declared in `internal/configs/config.balance.go`; defaulted/validated in
`internal/configs/config.balance.shops.go`; shipped values live in
`_datafiles/config.yaml`. `PricingConfigFromBalance()` only reads a knob's
config value when it is non-zero, otherwise it keeps
`DefaultPricingConfig()`'s value. For all five live knobs below, the shipped
value and the Go default are identical, so this fallback never actually
triggers in production today.

**Live** (read by `pricing.go` / `buyrules.go`):
- **`ShopBuyRatio`**: shipped `0.50`, Go default `0.50`. Feeds
  `PricingConfig.BuyRatio`, the multiplier in `CalcBuyPrice` above.
- **`ShopPriceFloor`**: shipped `0.25`, Go default `0.25`. The scarcity
  floor (overstocked items sell cheapest here).
- **`ShopPriceCeiling`**: shipped `5.0`, Go default `5.0`. The scarcity
  ceiling (out-of-stock items are priced highest here).
- **`ShopAbundanceThreshold`**: shipped `3.0`, Go default `3.0`. The
  `current/restockQty` ratio at which `ScarcityMultiplier` reaches
  `PriceFloor`; see the per-item normalization note above.
- **`ShopGoldReserveRatio`**: shipped `0.50`, Go default `0.50`. Read
  directly (not through `PricingConfig`) by `buyrules.go`'s
  `EvaluateBuyRules` and by `modules/auctions/npc_buyers.go`. Fraction of a
  shop's gold pool held back before it will buy from a seller.

**Dead** (declared, defaulted, validated, shipped with an explanatory
comment in `config.yaml`, but consumed by nothing outside
`internal/configs` itself):
- **`ShopMaterialReserve`**: shipped `1`, Go default `1`. Its comment
  claims "units of each material a crafter mob reserves before selling";
  no code reads the field, so a crafter mob does not actually reserve
  anything against this knob.
- **`BarterMaxDiscount`**: shipped `0.15`, Go default `0.15`. The buy-side
  barter cap that actually ships is a **hard-coded literal** `0.15` in
  `internal/actions/buy.go` (`discount := skill/50.0 * 0.15`); this config
  field is never read. The shipped value only *looks* load-bearing because
  it happens to match the hard-coded number.
- **`BarterMaxBonus`**: shipped `0.15`, Go default `0.15`. Same story on
  the sell side: `internal/actions/sell.go` hard-codes
  `bonus := skill/50.0 * 0.15` (then re-clamps to `0.15` in the same
  function) and never reads this field.

### Overstock Decay (chunk 5.4)
- **`TickOverstockDecay(si *ShopInventory, round uint64) []DecayedUnit`**:
  Reads balance config and delegates to `TickOverstockDecayWith`. Returns
  the set of items and quantities removed this sweep so callers can act on
  them (e.g. convert decayed potions to enchanting mats).
- **`TickOverstockDecayWith(si, round, isComponent, decayRounds, decayQty) []DecayedUnit`**:
  Testable core. For each `StockEntry` whose `Current > RestockQty` and whose
  `LastGrewRound` is older than `decayRounds`, removes `decayQty` units (never
  below `RestockQty`), then re-stamps `LastGrewRound` to pace subsequent
  decays. Crafting materials (`is_component`) are always skipped.
  Returns a `[]DecayedUnit` — one entry per affected `StockEntry`.

  **Key semantics:**
  - `RestockQty` is the baseline. Items with `RestockQty 0` (NPC-dumped /
    backfilled entries) drain fully to 0 when unsold.
  - The grace period (`decayRounds`) is measured from `LastGrewRound`, so
    items that are actively receiving forager deliveries decay more slowly
    than items that have been sitting since the last restock.
  - Crafting materials are excluded so supply for crafting recipes is never
    silently eroded by the decay pass.
  - The caller in `hooks/MobIdle_HandleIdleMobs.go` inspects each
    `DecayedUnit`: if `ItemSpec.Type == items.Potion`, it calls
    `crafting.EnchantSalvageYield` and feeds the resulting mats into
    `AddToReserve`.

  Config knobs:
  - `ShopOverstockDecayRounds` (default 21600) — rounds between decay fires
    per entry.
  - `ShopOverstockDecayQty` (default 1) — units removed per fire.

### Global Enchant-Mat Reserve (chunk 5.4)

The enchant reserve is a virtual analog of the forager chests: alchemy
vendors (Ilsa's, Voss's) accumulate enchanting mats here as their
unsold potions decay; enchanting-supply vendors (Vael, future
enchanters with `craft_support: "enchanting"`) draw from it to fill
their own stock gaps.

- **`AddToReserve(matItemId, qty int)`**: Adds `qty` units of
  `matItemId` to the global reserve. No-op for zero ID or qty. Called
  by the shop-restock hook after each decayed-potion conversion.
- **`ReservePool() map[int]int`**: Returns a snapshot copy of the
  current reserve (safe to iterate and mutate without holding the
  lock). Called by the enchanter draw path to build the input pool for
  `SelectStockTransfers`.
- **`DrainReserve(matItemId, qty int)`**: Removes up to `qty` units of
  `matItemId`; safe when `qty` exceeds the available balance (clamps
  to 0). Called after `SelectStockTransfers` confirms how many units
  to transfer.

  **Persistence:** the reserve is in-memory only (v1). A server restart
  re-accumulates from ongoing potion decay; no bootstrapping is needed
  because alchemy shops decay slowly and continuously.

  **Thread safety:** all three functions acquire `enchantReserveMu`.

### Stock Transfer Allocator (chunk 5.4)

- **`SelectStockTransfers(shopInv *ShopInventory, pool map[int]int) map[int]int`**:
  Pure function. Given a shop's current inventory and a pool of
  available item IDs → quantities, returns a transfer plan (item ID →
  qty to move) that fills the shop's biggest stock gaps first.

  Algorithm: compute `gap = MaxStock - Current` for every stocked item
  that appears in the pool; sort gaps descending (ties broken by item
  ID); greedily assign from pool until each item's gap or pool supply
  is exhausted. Only items the shop already carries (existing
  `StockEntry`) are eligible — the allocator never introduces new
  item IDs.

  **Shared by two call sites:**
  1. `internal/forager/chest_backfill.go` — `BackfillVendorFromChests`
     passes a pool built from all forager chest contents.
  2. `internal/hooks/MobIdle_HandleIdleMobs.go` — the enchanter draw
     path passes `ReservePool()` as the pool and calls `DrainReserve`
     for each entry in the result.

  Lifted from the forager-internal `selectBackfillTransfers` so both
  paths share one tested, deterministic allocator.

### `StockEntry.LastGrewRound`
Set by `AddStockAtRound` whenever stock increases (forager delivery,
backfill, restock tick, or player sale). Read by `TickOverstockDecayWith` to
enforce the grace period. Zero means "never grown"; a zero `LastGrewRound`
satisfies the decay condition immediately.

## Global State

### Shop Cache
- **`shopCache`**: `map[string]*ShopInventory` keyed by
  `"{zone}/{mobId}-room{roomId}"`. Populated lazily by `GetShopInventory`,
  persisted by `SaveShop`.
- **`shopCacheMu`**: `sync.RWMutex` protecting the cache.

### Enchant Reserve
- **`enchantReserve map[int]int`** — in-memory map of item ID → qty
  for enchanting mats waiting to be drawn by enchanters.
- **`enchantReserveMu sync.Mutex`** — guards `enchantReserve`.
  Both fields are package-private; callers use the `AddToReserve` /
  `ReservePool` / `DrainReserve` API.
- **`ResetEnchantReserveForTest()`** — test helper; clears the reserve
  between test cases so state doesn't leak across sub-tests.

### Persistence
Shop state lives at
`_datafiles/world/dogmud/shops/{zone}/{mobId}-room{roomId}.yaml`. This
directory is **not** wiped by the instance-save cleanup SOP — it is
persistent living-economy state. Deleting a file resets that merchant to
template defaults (500g starting gold, base stock levels).

## Data Structure Design

### StockEntry
```go
type StockEntry struct {
    ItemId        int    `yaml:"item_id"`
    RestockQty    int    `yaml:"restock_qty"`               // supply-cart quantity per restock (0 = NPC/forager only)
    MaxStock      int    `yaml:"max_stock"`                 // hard cap on accumulation
    Current       int    `yaml:"current"`                   // persisted live count
    LastGrewRound uint64 `yaml:"last_grew_round,omitempty"` // drives overstock decay grace period
}
```

### ShopInventory (abbreviated)
```go
type ShopInventory struct {
    Gold                   int
    StartingGold           int
    LastRestock            uint64
    Stock                  []StockEntry
    KnownRecipes           []string
    CraftSupport           string
    LastRestockByTier      map[int]uint64
    SalesCount             int
    BuysCount              int
    RestockCount           int
    ConsumedByCrafterCount int
    StockEvents            map[int][]StockEvent
    CurrentDepletion       map[int]uint64
    // Location fields (not persisted)
    Zone   string
    MobId  int
    RoomId int
}
```

## Gotchas

- **`BarterMaxDiscount` and `BarterMaxBonus` look tunable and are not.**
  Both the buy-side and sell-side barter caps are hard-coded `0.15` literals
  in `internal/actions/buy.go` and `internal/actions/sell.go`; editing these
  two `config.yaml` knobs changes nothing at runtime. Same dead pattern as
  the salvage knobs elsewhere in the codebase: check for a consumer before
  trusting a knob's comment.
- **`ShopAbundanceThreshold` is a ratio of `restockQty`, not an absolute
  stock count.** Two items can both be "abundant" (priced at `PriceFloor`)
  at wildly different absolute `Current` values if their `RestockQty`
  differs. When tuning perceived scarcity for a specific item, check its
  `restock_qty` first: the threshold config knob affects every item
  uniformly, but the stock level it takes effect at does not.
- **`ScarcityMultiplier`'s curve is inverse-quadratic, not linear.** Price
  rises slowly while stock is still comfortably above the abundance
  threshold and accelerates sharply only in the last stretch toward zero.
  A linear mental model of the 0.25x-5.0x range will underestimate price
  spikes on nearly-depleted stock.
- **`PricingConfigFromBalance()` silently keeps the Go default for any
  knob that reads as zero or less**, rather than erroring. This is
  currently inert for all five live knobs (shipped values equal their Go
  defaults), but a future `config.yaml` edit that zeroes one of them would
  fail closed to the default instead of failing loudly.

## Integration Notes

### Stock Contributors
- **`internal/behaviortree/`** — `TickOverstockDecay` is called from the
  shop restock hook each server tick.
- **`internal/forager/vendor_sell.go`** — `SellToVendor` transfers items
  directly into `StockEntry.Current` (free supply handoff, no gold).
- **`internal/forager/chest_backfill.go`** — `BackfillVendorFromChests`
  tops off vendors from forager lockboxes (also free, no gold).
- **`internal/actions/sell.go`** — player and mob sales use
  `EvaluateBuyRules` + `AddStockAtRound`; player sells draw down
  `ShopInventory.Gold`, mob sells do not (see `actions` package context.md).

### Consumers
- **`internal/usercommands/buy.go`**, **`internal/mobcommands/buy.go`**,
  **`internal/actions/buy.go`** — purchase flow reads pricing + stock.
- **`internal/economy/`** — economy dashboard reads `ShopInventory` for
  throughput and health scoring.
- **`internal/hooks/`** — restock cadence hook fires `TickOverstockDecay`
  each tick.

## Testing Notes

- **overstock_decay_test.go**: Covers baseline-clamping, crafting-material
  exclusion, grace-period gating, pacing (re-stamp after decay), nil/zero-qty
  guards, and the `[]DecayedUnit` return slice (content + length).
- **enchant_reserve_test.go**: Covers `AddToReserve` accumulation,
  `DrainReserve` partial and over-drain, and zero-ID/zero-qty no-op guards.
  Each test calls `ResetEnchantReserveForTest()` in setup to isolate state.
- **stock_transfers_test.go**: Covers the neediest-gap-first ordering,
  pool-cap clamping, items-not-in-shop exclusion, and empty-pool / empty-shop
  edge cases.
- **pricing_test.go**, **buyrules_test.go**, **restock_cadence_test.go**,
  **effective_max_stock_test.go**: Unit coverage for each sub-system.
- `TickOverstockDecayWith` injects `isComponent`, `decayRounds`, and
  `decayQty` so tests don't need loaded item specs or a live config stack.
