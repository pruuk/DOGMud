# Baubles slice D: shelf resale (design)

Date: 2026-09-30. Owner-approved design, specced against `origin/master`
6b6ff7ddf. Parent spec:
`docs/superpowers/specs/2026-09-28-baubles-hardening-and-corpus-design.md`
(slice D row). Revised after two blind reviews. The owner's eight rulings
(2026-09-30) are folded into the sections below and recorded under "Rulings"
at the end.

## Facts verified against source

Every row was read from the tree at 6b6ff7ddf on 2026-09-30. Balance values
are from `_datafiles/config.yaml` (`git show HEAD:`).

| # | Fact | Where |
|---|------|-------|
| F1 | `sellBaubleToMerchant(seller, item, room, mob, shopInv, awardProgression)` prices via `baubleOfferFor`, removes the item, saves a living shop, then calls `baubles.MarkSold(item.Bauble, sellValue, seller.GetUserId())`. Nothing is stocked; the comment says "it leaves the world" | `internal/actions/sell_bauble.go:198-257`, comment `:240-241`, `MarkSold` `:248` |
| F2 | `baubleOfferFor(item, shopInv, fence, zone)`: fence and `StolenGoods()` pays `FencePrice`; fence otherwise pays `BaublePrice`; a living shop not in `BaubleBuyerCraftSupports` refuses; `HotIn(zone, now)` refuses; else `BaublePrice`. Reserve check for living shops | `sell_bauble.go:108-138` |
| F3 | Test clock for heat at a sale: `var baubleNowForSale = time.Now` | `sell_bauble.go:61` |
| F4 | Only a player seller draws down merchant gold; a mob's sale mints its payout (`char.Gold += sellValue` for any seller). A legacy merchant (nil `shopInv`) pays from `mob.Character.Gold` | `sell_bauble.go:214-230`, `:143-148` |
| F5 | `(*Mob).GetSellPrice` returns 0 for a bauble | `internal/mobs/mobs.go:1093-1095` |
| F6 | `(*Mob).IsFence` matches `m.Groups` against `BaubleFenceGroups` | `mobs.go:1000-1013` |
| F7 | `BaubleFenceGroups: [fence]`, `BaubleFenceBuyPct: 60`, `BaubleStolenHeatHours: 72`, `BaubleBuyerCraftSupports: [general, jewelcrafting]`, `ShopBuyRatio: 0.50`, `BarterMaxDiscount: 0.15`, `BaubleCheapMinValue: 1`, `BaubleCheapMaxValue: 6`, `BaubleAverageMinValue: 10` | `config.yaml:1531, 1530, 1519, 1554-1556, 1418, 1434, 1546, 1547, 1548`; Go field `BaubleCheapMaxValue` `internal/configs/config.balance.go:914` |
| F8 | Nine fence mobs: 104 (general), 250 (none), 9172 (none), 9185 (general), 9209 (general), 9213 (general), 9215 (none), 9323 (none), 9428 (cooking). Each has shop entries | `_datafiles/world/dogmud/mobs/**` `groups:` and `craft_support:` |
| F9 | Every mob with any `Character.Shop` entry, or a crafter with materials or recipes, gets a `ShopInventory` via `RegisterMobShop` | `internal/mobs/crafter.go:44-107` |
| F10 | `ValidateShopMobTags` lets only a fence omit `craft_support` | `internal/shops/validation.go:53-56` |
| F11 | `AffixedStockEntry{Item, Price, AddedRound uint64 "added_round,omitempty"}` | `internal/shops/shopinventory.go:72-76` |
| F12 | `AddAffixedStock(item, price, cap int)` appends with `AddedRound: util.GetRoundCount()` and drops index 0 while `len > cap` (cap <= 0 means none) | `shopinventory.go:141-152` |
| F13 | `RemoveAffixedStock(idx) (items.Item, bool)` | `shopinventory.go:155-162` |
| F14 | `AddedRound` is written at `:145` and read nowhere | repo-wide grep for `AddedRound` |
| F15 | `ShopAffixedStockCap` is NOT in `config.yaml`; the live value is the Go default 8. Its Go field sits in the `LOOT` block and its default in the misc `LOOT` defaults, while `config.yaml` has no balance `LOOT` section; the shop knobs live in `SHOP ECONOMY` in both files | `config.balance.go:947-950` (field), `:868-885` (`SHOP ECONOMY`, `BarterMaxBonus` `:885`); `config.balance.misc.go:305, 312-313`; `config.balance.shops.go:44-45` (`BarterMaxBonus` default); `config.yaml:1417-1435` |
| F16 | `AddAffixedStock` callers: sell of affixed loot `sell.go:392-393` (price `item.GetSpec().Value`), auction win `modules/auctions/npc_buyers.go:296-297`, buy rollback `buy.go:639` (cap 0, barter-discounted `matched.price`). Test callers: `shopinventory_test.go:459, 460, 477, 495`, `buy_test.go:29` | grep |
| F17 | The auction shopkeeper bids only where `EvaluateBuyRules` offers; that refuses an item whose spec has no `VendorCategories`, and carrier item 900 has none, so auctions never shelve a bauble today | `npc_buyers.go:216-237`, `internal/shops/buyrules.go:50-52`, `_datafiles/world/*/items/*/900-curious_trinket.yaml` |
| F18 | `list`: `buildShopStockFromInventory` reads only `shopInv.Stock`; `AffixedStock` is never shown. Stock rows are sorted by the Qty column string. An empty listing makes the mob `say I have nothing to sell` | `internal/usercommands/list.go:125-149`, sort `:454-456`, say `:68-70` |
| F19 | `renderShopTable` titles a table `"%s by %s"` (title, seller name) and sends it to the lister alone with `user.SendText` | `list.go:433-438`, format `:434` |
| F20 | `tryPurchaseFromInventory` builds `available` as `shopInv.Stock` rows then every `AffixedStock` entry (`affixedIdx: i`), matched by `e.Item.GetSpec().Name`, fancy names `e.Item.DisplayName()`, price `e.Price` less barter | `internal/actions/buy.go:535-582` |
| F21 | `util.FindMatchIn(request, itemNames...)` returns the matched NAME (`N.name` picks the Nth full or close match via `GetMatchNumber`); `buy` then takes the FIRST `available` entry whose `plainName` equals it, so two entries with one name always resolve to the first | `internal/util/util.go:322-337, 353-404`; `buy.go:584-609` |
| F22 | No match: the mob says "Any interest in this X?" naming a random fancy name to the room (`shopMob.Command`) | `buy.go:588-597` |
| F23 | Encumbrance gate refuses when carried plus the item's weight exceeds `CarryCapacity()`, before any side effect | `buy.go:611-617` |
| F24 | Affixed purchase: gold check, `RemoveAffixedStock`, fresh UUID, `StoreItem`, rollback `AddAffixedStock(bought, matched.price, 0)` on failure, `SaveShop`, "You buy the X" (`DisplayName()`) to the buyer, room line with `DisplayName()` | `buy.go:628-664` |
| F25 | `StoreItem` fails only for `ItemId < 1` or when the new weight exceeds twice `CarryCapacity()` | `internal/characters/inventory.go:169-187` |
| F26 | For a bauble, `GetSpec()` resolves through the catalog: `Value` is the record's `Value`; name and text are the generic view (viewer 0) | `internal/items/items.go:329-341`, `internal/items/bauble.go:76-105` |
| F27 | Viewer-aware accessors: `GetSpecFor`, `DisplayNameFor`, `NameFor`, `LongDescriptionFor` (items), `MaterialFor` (baubles). A finder-only record shows `Trinket` to everyone else; `Value` is the same in both views | `internal/items/bauble_viewer.go:13-46`, `internal/baubles/record.go:155-181`, `fallback.go:16` |
| F28 | Generic trinkets (`GenericTrinket`, `fallback` status) are also named `Trinket` | `internal/baubles/fallback.go:16, 36-49` |
| F29 | `NameMatch` is viewer-agnostic on purpose: a finder-only bauble's hidden words never match, "for its finder either", so a typed word cannot confirm hidden text. Every bauble also answers to `bauble` and `trinket` in `NameMatch` | `internal/items/items.go:605-645` (comment `:638-641`), `bauble.go:19-21` |
| F30 | Root guard lists every finder-view call site as `"path\|Func"` with a reference count; a reference in an unlisted function fails; sends beyond one reader are refused; a `SendText` passes when its receiver and the accessor's viewer argument hang from the same root identifier | `bauble_finder_view_guard_test.go:28-47, 71-82`, naming `lookup_viewer_guard_test.go:132` |
| F31 | Heat: `HeatDuration()`, `Record.Hot(now)` (stolen goods and `now < StolenAt + HeatDuration()`), `StolenGoods()`, `HotIn(zone, now)` (only in the theft's heat area), `HeatArea`, `ItemIsHotIn(itm, zone, now)` | `internal/baubles/theft.go:88-151` |
| F32 | Statuses `ready`, `fallback`, `sold`, `retired`; the `sold` comment says "the item is gone" | `record.go:14-19`, `:17` |
| F33 | `MarkSold` sets `Status=sold`, `SoldAt=now UTC`, `SoldValue` | `internal/baubles/sales.go:20-30` |
| F34 | `SalesSince(t)` counts `Status == sold && !SoldAt.Before(t)` | `sales.go:35-45` (check at `:39`) |
| F35 | "Every record is sellable, a sold one included" rule and its crash rationale | `sales.go:13-16`; test `TestSell_Bauble_ASoldRecordSellsAgain` `sell_bauble_test.go:233` |
| F36 | Status readers: `CatalogStats` unsold count `admin.go:70`; `Restore` `admin.go:113-121`; `ApplyRegenerated` `admin.go:262`; retired checks `record.go:163`, `corpus.go:380`, `corpus_admin.go:139`; admin command `admin.bauble.go:218` (show header), `:248` (show prints the sold line only when `Status == sold`), `:297` (list row), `:395-396` (stats). `:292-293` and `:390-391` are `SalesSince` calls | grep for `Status` and `SalesSince` |
| F37 | `View()` shows the real text unless `Status == retired` | `record.go:163-168` |
| F38 | `Restore` picks the unsold status inline: `fallback`, or `ready` when `r.Generator.Named()`; then `sold` when `SoldValue > 0` | `internal/baubles/admin.go:111-128` (rule `:116-119`) |
| F39 | A return credits its thief once per bauble: giver is `StolenByUserId`, `ReturnCreditAt` zero, and the owner's faction holds catches against them | `internal/actions/stolen_bauble.go:279-318`, `theft.go:166-180` |
| F40 | Sweep: live source `shops` walks `shops.AllShops()` with `WalkItems`, which visits every `AffixedStock` item; the disk scan reads `shops/` files too | `bauble_sweep.go:49-52`, `internal/shops/walk_items.go:7-13`, `bauble_sweep_test.go:92` |
| F41 | Commands run in `processInput` (`world.go:962`, `TryCommand` `:1040`), reached from the `Input` listener inside `EventLoop`, which runs under `util.LockMud()` (`world.go:864-866`). The auction `NewRound` listener (`auctions.go:72`) runs there too | `world.go` |
| F42 | `ShopInventory` has no lock of its own; `shopCacheMu` guards only the cache map. `SaveShop` marshals the live struct | `internal/shops/persistence.go:19-22, 151-183` |
| F43 | Catalog: `cat.mu` taken briefly, never held across a call out; disk writes outside it. Sweep takes the mud lock for the live walk, then `cat.mu` in `applySweep` | `catalog.go:20-27`, `sweep.go:212, 228-229, 59-63` |
| F44 | `ShopSnapshot` has `CraftSupport` (yaml and json `craft_support`); `captureShops` covers every cached shop; `lookupShopMobName` falls back to `mobs.GetMobSpec` | `internal/economy/health/snapshot.go:60-88`, `capture.go:42-111, 484-499` |
| F45 | `CraftSupport` consumers: `PerCraftSupportScores` keys on it (`scoring.go:151`; doc `:135-139` says empty means a fence), `ShopScoreRow.CraftSupport` (`scoring.go:825`); page groups `s.craft_support \|\| "(uncategorized)"` (`index.html:314`), score lookup `PerCraftSupport[disc]` (`:324`), per-shop cell `row.CraftSupport` with a dash fallback (`:351`) | files named |
| F46 | Test to invert: `assert.Len(t, si.AffixedStock, 0, "baubles are not resold like affixed loot")` in `TestSell_Bauble_LivingShopByCraftSupport` (player seller, value 12) | `sell_bauble_test.go:160-192`, line `:191` |
| F47 | Bartering is awarded once per `sell` command and once per `buy` command. The award path has no per-trade or time-based limit on ordinary events; its chance is keyed on skill level. The only per-round claims are `claimBonusProgression` (bonus events) and `DriftFromCombat` | `sell.go:292-295, 474-477`; `buy.go:431-435, 809-822`; `internal/characters/progression_award_resolved.go:32-82`; `progression.go:115-140, 178-216, 824, 884-935` |
| F48 | Torch 20096 has value 1; both the shop's buy offer and its sale price floor at 1 | `_datafiles/world/dogmud/items/armor-20000/light/20096-torch.yaml:1-10`; `buyrules.go:73-81`; `internal/shops/pricing.go:96-113` |
| F49 | Refusal lines, merchant voice: "I'm afraid I don't buy those.", "I'm not interested in trinkets. Try a general store or a jeweller.", "I can't afford that right now.", "That was stolen, and not long ago. I won't touch it. Try someone less particular about where things come from." | `sell_bauble.go:41-46` |
| F50 | Stale comments this change contradicts: `sell.go:317` ("catalog-priced, never stocked"), `record.go:17`, `scoring.go:135-139`, `config.balance.go:950` ("default 8"), `sell_bauble.go:18-25, 240-241`, `sales.go:13-16`; `internal/actions/context.md:1369` ("Never stocked, never resold") | files named |
| F51 | Player help: `sell.template:17-20` (baubles bought by general stores and jewellers; `sell all bauble`), `buy.template`, `list.template` | `_datafiles/world/dogmud/templates/help/` |

Brief corrections: `ApplyRegenerated` (`admin.go:262`) also reads status;
auctions shelve no bauble today (F17); the cap knob has no `config.yaml` key
(F15); the `admin.bauble.go` line numbers are as in F36.

## Summary

A player's sale of an average or rare, non-retired bauble to a living shop
puts it on the shelf (`AffixedStock`) at catalog value instead of
destroying it. `list` shows the shelf, per viewer. A bauble still hot when
shelved is held in the backroom until its heat ends; the backroom holds at
most as many as the shelf cap, and a full one refuses another hot bauble.
The listed cap rises from 8 to 12. Buying one back returns its record to its
unsold status. The dashboard types a fence's shop as `fence`.

## 1. Shelf entry and helpers (`internal/shops`)

`AffixedStockEntry` gains two fields; `AddedRound` stays as is.

```go
AddedAt   time.Time `yaml:"added_at,omitempty"`   // wall clock when shelved
HoldUntil time.Time `yaml:"hold_until,omitempty"` // zero: listed at once
```

Why a wall-clock `AddedAt`: heat is real time (`StolenAt`, F31) and
`AddedRound` is game rounds (F12), which do not advance while the server is
down, so the two cannot be ordered against each other.

One clock seam for everything the shelf does: `var ShelfNow = time.Now` in
`internal/shops`. `list`, `buy`, the sale's shelving and the held-cap
refusal all read it, so a test that sets it sees one consistent held state.
`baubleNowForSale` keeps its existing job (the `HotIn` refusal).

- `(e AffixedStockEntry) Held(now time.Time) bool` is `now.Before(e.HoldUntil)`.
- `(e AffixedStockEntry) ListedAt() time.Time` is `HoldUntil` when non-zero,
  else `AddedAt`. Entries saved before this change have both zero and so
  sort earliest.
- `(si *ShopInventory) HeldCount(now time.Time) int` counts held entries.
- `(si *ShopInventory) ListedIndexes(now time.Time) []int` returns the raw
  `AffixedStock` indexes of the non-held entries in slice order. This is
  THE shelf order: `list` renders it unsorted and `buy` matches in it.
- `AddAffixedStock(item items.Item, price, cap int, holdUntil, now time.Time)`
  appends `{Item, Price, AddedRound: util.GetRoundCount(), AddedAt: now,
  HoldUntil: holdUntil}` and then calls `EnforceAffixedCap(cap, now)`. The
  signature change is deliberate: the compiler lists every caller (F16). It
  does not enforce the held cap; the sale refuses first (section 2).
- `EnforceAffixedCap(cap int, now time.Time) int`: while the count of
  non-held entries exceeds `cap`, remove the non-held entry with the
  earliest `ListedAt()`, ties to the lowest index. Returns how many it
  removed. `cap <= 0` removes nothing. Held entries are never removed. A
  removed item is gone; a bauble's record keeps the sale that shelved it.
- `RestoreAffixedStock(idx int, e AffixedStockEntry)` reinserts an entry at
  `idx` (clamped to the length), for `buy`'s rollback.

`baubles.ShelfHoldUntil(itm items.Item, now time.Time) time.Time`, beside
`ItemIsHotIn` (F31): for a bauble whose record is `Hot(now)`,
`rec.StolenAt.Add(HeatDuration())`; otherwise zero. Non-baubles always get
zero.

Why the hold is global (`Hot`), not local (`HotIn`): the shop that bought it
may be outside the theft's heat area, but its buyer can carry it anywhere,
including back into that area, where its owner would recognise it and
honest merchants would refuse it. Holding it until it is cold everywhere
means nobody buys a hot item off a shelf.

## 2. Selling (`internal/actions/sell_bauble.go`)

`baubleShelvable(rec baubles.Record) bool` is `rec.Status !=
baubles.StatusRetired && rec.Value > int(BaubleCheapMaxValue)` (F7: above 6,
so average and rare only).

A bauble goes on the shelf only when all hold: the seller is a player
(ruling 7; a mob's sale mints its payout and the shop pays nothing, F4), the
merchant has a `ShopInventory`, and `baubleShelvable(rec)`. Inside the
existing `if shopInv != nil` block, before `SaveShop`:

```go
if seller.IsPlayer() {
    if rec, ok := baubles.Get(item.Bauble); ok && baubleShelvable(rec) {
        now := shops.ShelfNow()
        shopInv.AddAffixedStock(item, item.GetSpec().Value,
            int(configs.GetBalanceConfig().ShopAffixedStockCap),
            baubles.ShelfHoldUntil(item, now), now)
    }
}
```

Every other sale destroys the bauble as today: a legacy merchant, a mob
seller, a cheap bauble (ruling 5) and a retired one (ruling 1, whose
withdrawn text would otherwise be listed once `MarkSold` sets it sold, F33,
F37). Price is the catalog value (F26), the `GetSpec().Value` rule affixed
loot uses (F16). `MarkSold` is unchanged and runs after the save for every
sale.

Backroom cap (ruling 6). In `baubleOfferFor`, after the existing refusals
and before the reserve check: when `shopInv != nil`, `baubleShelvable(rec)`,
`rec.Hot(now)` and `shopInv.HeldCount(now) >= ShopAffixedStockCap` (with
`now = shops.ShelfNow()`), refuse with

```go
baubleSayBackroomFull = "I can't move any more hot goods right now. Come back once some of what I'm sitting on has cooled."
```

It is an interest refusal, not `Broke`, so `bestBaubleMerchant` moves on to
the next merchant and `offer` and `appraise` show it. It does not ask
whether the seller is a player: a mob selling a shelvable hot bauble to a
full backroom is refused too, which keeps the offer one rule for every
caller. Held entries count against this cap only, never the listed cap.

The comments in F50 (`sell_bauble.go:18-25, 240-241`, `sell.go:317`) are
rewritten to say which baubles are shelved.

## 3. `list` (`internal/usercommands/list.go`)

For every mob merchant with a `ShopInventory`:

1. `now := shops.ShelfNow()`; `EnforceAffixedCap(cap, now)`; if it removed
   anything, `shops.SaveShop`.
2. Render the regular stock as today.
3. Render the entries of `ListedIndexes(now)` in that order, unsorted, as a
   second table titled `Secondhand goods` (rendered by F19's format as
   "Secondhand goods by Siv"), columns Name, Type, Price, help line
   `To buy something, type: buy [name]`. Name is
   `e.Item.DisplayNameFor(user.UserId)` (F27), so a finder-only bauble reads
   `Trinket` to everyone but its finder. Price is `e.Price` (before barter,
   like the stock table).
4. The "nothing to sell" say (F18) fires only when both tables are empty.

The rows are built in `buildShelfRows(si *shops.ShopInventory, viewerUserId
int, now time.Time) [][]string`, registered in `finderViewSites` (F30):
`"internal/usercommands/list.go|buildShelfRows": {1, "the lister's own shop
listing; renderShopTable sends it to that user alone"}`.

This also fixes the existing gap where sold-on affixed loot could be bought
only by guessing its name (F18, F20).

## 4. `buy` (`internal/actions/buy.go`)

In `tryPurchaseFromInventory`:

- First: `now := shops.ShelfNow()`; `EnforceAffixedCap(cap, now)`; remember
  whether it removed anything.
- `available` is built as today from `shopInv.Stock`, then from the shelf in
  `ListedIndexes(now)` order. Held entries are absent from `available`,
  `itemNames` and `itemNamesFancy`. Each shelf row's `affixedIdx` is its raw
  `AffixedStock` index, not its position among listed rows.
- Name of a shelf bauble row (ruling 4): `e.Item.NameFor(buyer.GetUserId())`
  (F27), the view `list` showed that buyer. A mob buyer has user id 0 and
  gets the generic view. Every other row keeps `GetSpec().Name`. This
  differs on purpose from `NameMatch` (F29), which never matches hidden
  words: here only the finder's own view contains them, so nobody else can
  confirm them by typing.
- Selection by index. New `util.FindMatchIndexIn(searchName string, items
  ...string) (match int, closeMatch int)` runs `FindMatchIn`'s exact
  algorithm (F21) and returns positions, `-1` for none; `FindMatchIn` is
  rewritten as a wrapper over it so there is one algorithm. `buy` uses the
  index to pick `available[i]` directly, replacing the first-name loop
  (`buy.go:601-606`). So `buy trinket` takes the first matching row and
  `buy 2.trinket` the second, counted in `available` order: regular stock
  first (in `shopInv.Stock` order), then the shelf in `list` order. A full
  match anywhere outranks a close match, as `FindMatchIn` already rules.
  The price charged is the chosen row's own price.
- Keyword parity with `sell all bauble` (F51) is not added: `FindMatchIn`
  matches one name per row, and a shelf row is bought by the name `list`
  shows. `help buy` says so (section 11).
- A request that matches only a held bauble takes the existing no-match
  path (F22) or the existing close-match rule, and the mob's "Any interest
  in this X?" can never name a held item.
- Affixed purchase (F24): copy the entry before `RemoveAffixedStock`; on
  `StoreItem` failure, `RestoreAffixedStock(idx, copy)`, replacing the
  `AddAffixedStock(bought, matched.price, 0)` rollback, which stamped a new
  listing time and relisted at the barter-discounted price.
- Buyer's line: "You buy the X" uses `bought.DisplayNameFor(buyer.GetUserId())`
  in `buyer.SendText`, which passes the guard's receiver rule (F30); the
  room line keeps `DisplayName()`. With the `NameFor` above, the guard row
  is `"internal/actions/buy.go|tryPurchaseFromInventory": {2, "the buyer's
  own view of a shelf bauble: a match key compared with what the buyer
  typed, and the buyer's own purchase line (buyer.SendText)"}`.
- After a successful bauble purchase: `baubles.MarkBought(bought.Bauble,
  buyer.GetUserId())` (section 5).
- If `EnforceAffixedCap` removed anything and no purchase saved the shop,
  save it before returning.

## 5. Record (`internal/baubles`)

- New `(r Record) unsoldStatus() Status` in `admin.go`: `StatusReady` when
  `r.Generator.Named()`, else `StatusFallback`. `Restore` (F38) calls it in
  place of its inline lines `:116-119`, so the rule lives once (ruling 2).
- New `MarkBought(id string, buyerUserId int) bool` in `sales.go`: `Update`
  that sets `Status = r.unsoldStatus()` when the status is `sold` and
  changes nothing otherwise, so a record an admin retired while it sat on
  the shelf stays retired. Logs `action bought` like `MarkSold`. `SoldAt`,
  `SoldValue` and every theft field are kept.
- A stolen bauble bought back is cold (its hold outlasted `Hot`) but still
  `StolenGoods()`: honest shops buy it anywhere and a fence pays
  `FencePrice`, 60% (F2, F7).
- Give-back after a buyback is intended (ruling 8): the thief fences it,
  waits out the hold, buys it back and gives it to its owner for the
  once-per-bauble return credit (F39). That costs 40% of value before
  barter (sold at 60%, bought at full value), 25% at the full 0.15 barter
  discount, plus the wait.
- `SalesSince` counts `!r.SoldAt.IsZero() && !r.SoldAt.Before(t)` and drops
  the status test (F34), so a buyback does not erase a past sale. A record
  sold twice holds only its latest `SoldAt` and `SoldValue`, so it counts
  once, at its latest sale.
- Admin `bauble show` (F36, `admin.bauble.go:248`) prints its sold line when
  `SoldValue > 0` instead of when the status is sold, worded "last sold:",
  so the sale history stays visible after a buyback. The `[status]` header
  (`:218`) shows the current status.
- No new status (owner ruling).
- The "a sold record can sell again" rule (F35) stays; the code path is
  unchanged. Its comment is rewritten: a record now also returns to a pack
  legitimately, through a buyback, and the crash case is the only way one
  still marked sold does. `record.go:17` ("the item is gone") becomes "sold
  to a merchant; destroyed, or on its shelf".

## 6. Dashboard (`internal/economy/health`, admin page)

`ShopSnapshot` gains `Fence bool` with `yaml:"fence,omitempty"
json:"fence,omitempty"`, set in `captureShops` from the mob template:
`t := mobs.GetMobSpec(mobs.MobId(inv.MobId)); ss.Fence = t != nil &&
t.IsFence()` (the template is always loaded, F44; instance groups can gain
bounty tags). `CraftSupport` keeps the shop's real tag.

A flag rather than a derived type string: saved snapshots decode with
`Fence` false and fall back to `CraftSupport`, and the real `craft_support`
of fences 104 and 9428 (F8) stays in history.

`func (s ShopSnapshot) Type() string` returns `"fence"` when `Fence`, else
`CraftSupport`. `PerCraftSupportScores` keys on `s.Type()` (`scoring.go:151`,
doc `:135-139` rewritten) and `ShopScoreRow.CraftSupport` takes `s.Type()`
(`:825`). The page groups on `s.fence ? "fence" : (s.craft_support ||
"(uncategorized)")` (`index.html:314`); the per-shop cell already reads
`row.CraftSupport`. Nothing else on the page changes.

## 7. Config

One section in both files, `SHOP ECONOMY`:

- `ShopAffixedStockCap: 12` is added to `config.yaml` after
  `BarterMaxBonus` (`:1435`), commented as the cap on listed shelf entries,
  and separately on held ones.
- The Go field moves from the `LOOT` block (`config.balance.go:950`) to
  `SHOP ECONOMY` after `BarterMaxBonus` (`:885`), with its comment saying
  default 12; its default moves from `config.balance.misc.go:312-313` to
  `config.balance.shops.go` beside `BarterMaxBonus` (`:44-45`), as 12.
- Commit `config.yaml` from the `git show HEAD:` blob (CLAUDE.md tripwire).

## 8. Concurrency and locks

`list`, `buy`, `sell` and the auction win all mutate a `ShopInventory` from
the event loop under the mud lock (F41); the sweep's live walk takes the
same lock (F43), so a bauble moving shelf to pack is seen in one of the two.
`SaveShop` marshals the struct and so relies on the same lock (F42); the new
saves in `list` and `buy` sit inside it. The held-cap check and the add run
in one sale under that lock, so the count cannot change between them.
`MarkBought` and `ShelfHoldUntil` take `cat.mu` briefly and never while
holding a shop or cache lock, the existing order (mud lock, then `cat.mu`,
F43).

## 9. Persistence and migration

Shop files are living state. `AddedAt` and `HoldUntil` are `omitempty`, so
an old file loads with both zero: listed, and earliest in eviction order.
No migration: the zero values are the correct meaning, no bauble is on any
shelf before this ships (F1, F17), and nothing is renamed or removed.
Snapshots: section 6.

## 10. Other `AddAffixedStock` callers

- `sell.go:393` (affixed loot): passes `baubles.ShelfHoldUntil(item, now)`
  (always zero there) and `now = shops.ShelfNow()`; the new cap rule, 12
  listed.
- `npc_buyers.go:297` (auction win): same call shape. Auctions never win a
  bauble today (F17).
- `buy.go:639`: replaced by `RestoreAffixedStock` (section 4).
- Tests edited for the new signature, not just rerun:
  `shopinventory_test.go:459, 460, 477, 495` and `buy_test.go:29`.

## 11. Docs and help

- Help, `_datafiles/world/dogmud/templates/help/`: `sell.template` (average
  and rare baubles sold to a shop go on its shelf; a fence with a full
  backroom turns hot goods away), `list.template` (the secondhand table),
  `buy.template` (secondhand goods are bought by the name `list` shows,
  `2.name` for the second of two).
- Comments in F50.
- `internal/shops/context.md` (entry fields, new methods, clock, caps),
  `internal/baubles/context.md` (`MarkBought`, `unsoldStatus`,
  `ShelfHoldUntil`, `SalesSince`), `internal/economy/health/context.md`
  (`Fence`, `Type()`), `internal/actions/context.md` (`:1369`, the shelf,
  the backroom refusal, the buyback), `internal/usercommands/context.md`
  (`list` shows the shelf), `internal/util/context.md` (`FindMatchIn` is
  documented at `:95-100, 218`; add `FindMatchIndexIn`).

## 12. Tests to pin

1. A held entry is absent from `list` and cannot be bought by its name.
2. A hold expires (set `shops.ShelfNow`) and the entry lists and sells.
3. The listed cap counts non-held entries only; eviction never touches a
   held entry.
4. Over the cap, the entry with the earliest `ListedAt()` goes: an entry
   shelved first but held until later outlives an unheld entry shelved
   after it.
5. Enforcement after a hold expires happens lazily on `list`, on `buy`, and
   on an add.
6. Backroom full: a fence holding `ShopAffixedStockCap` entries gives the
   backroom refusal for a shelvable hot bauble, and still buys a cold one
   and a cheap hot one (destroyed, not held).
7. Only shelvable player sales shelve: a mob seller, a cheap bauble and a
   retired bauble each leave the shelf empty and the record sold.
8. Invert F46: the general store's sale leaves one shelf entry at catalog
   value; a hot bauble's entry carries `HoldUntil = StolenAt + 72h`.
9. Same-name selection: two `Trinket` rows at different prices; `buy
   trinket` takes and charges the first in `list` order, `buy 2.trinket`
   the second, and each removes its own `AffixedStock` index, including
   when a held entry sits between them.
10. `FindMatchIndexIn` and `FindMatchIn` agree on every existing
    `FindMatchIn` test case.
11. Finder view: `list` shows a finder-only bauble's own name to its finder
    and `Trinket` to another player; `buy` matches the finder's own name for
    the finder and `Trinket` for anyone else; the purchase line shows the
    finder their own name; the guard passes with the two new rows and fails
    without either.
12. A buyback sets a named record ready and a generic one fallback, leaves
    a retired one retired, and `SalesSince` still counts the sale; `bauble
    show` still prints the last sale; `TestRetireAndRestore` passes through
    `unsoldStatus`.
13. Dashboard: `captureShops` sets `Fence` for a fence template; `Type()`
    and `PerCraftSupportScores` report `fence`; a pre-change snapshot YAML
    (no `fence` key) decodes with `Type()` equal to its `craft_support`.
14. `RestoreAffixedStock` reinserts an entry at its index with its fields
    unchanged. The `buy` rollback branch itself is defensive and not
    reachable through real code: the encumbrance gate (F23) refuses first,
    and `StoreItem` fails only above twice capacity or for `ItemId < 1`
    (F25). No seam is added for it.
15. Existing `TestSell_Bauble_ASoldRecordSellsAgain` still passes.

No sweep test is added: this change does not alter what the sweep walks
(F40 already covers every `AffixedStock` item on master). Tests load Go
defaults, not `config.yaml` (dogmud-writing-tests), so cap tests pass the
cap explicitly.

## 13. Risks and accepted limitations

- Duplication on crash: the shop is saved at the sale, the seller's save
  later. A crash between leaves the bauble in the restored pack and on the
  shelf, two items on one record.
- Gold on crash: `buy` saves the shop (gold in, entry out) before the buyer
  autosaves. A crash between restores the buyer's gold without the item,
  so the shop's gold is created and the item lost. Both crash cases are
  pre-existing for affixed loot (F16, F24).
- The Bartering loop is closed for baubles: a cheap bauble is not shelved
  (ruling 5), and an average one (value 10 or more, F7) loses at least 3 gold
  per round trip: bought back for at least 9 after the 0.15 barter discount,
  sold for at most 6 (a fence's 60%). The same loop already exists for value-1
  shop stock such as the torch (F48): bought and sold back at 1 gold each,
  one Bartering award per command, with no repeat limit (F47).
  Pre-existing and out of scope.
- A finder-only bauble shows others `Trinket` at its real price. With cheap
  baubles off the shelf, a `Trinket` priced 10 or more reveals an average or
  rare record whose text is kept to its finder or that is a generic trinket
  (F28). Accepted: value was never part of the kept text (F27), and the
  words stay hidden.
- Accepted limitation (ruling 3): retiring and then restoring a bauble that
  was bought back labels it `sold` while a player holds it, because
  `Restore` still reads `SoldValue > 0` (F38). No `BoughtAt` field.
- Stale price: `Price` is fixed at shelving, so an admin `Edit` of value or
  a regen does not reprice a shelved bauble. Affixed loot behaves the same.
- `HoldUntil` is fixed at shelving; a later change to
  `BaubleStolenHeatHours` does not move existing holds.
- `buy N.name` counting runs over stock rows in `shopInv.Stock` order, while
  `list` sorts stock rows by quantity (F18). For shelf rows the orders
  agree; the stock mismatch is pre-existing.
- The general-store discipline score changes when four fences (104, 9185,
  9209, 9213) leave the `general` group for `fence`.

## Rulings

Owner rulings of 2026-09-30.

1. **Retired baubles are not shelved.** Selling one destroys it as today;
   `MarkSold` is unchanged (section 2).
2. **Buyback status uses `Restore`'s rule** through one shared helper,
   `unsoldStatus` (section 5).
3. **`Restore` after a buyback shows `sold`: accepted.** No `BoughtAt`
   field (section 13).
4. **`buy` matches a shelf bauble by the buyer's own view,** the view
   `list` used, registered with the finder-view guard (section 4).
5. **Cheap tier is not shelved.** Only baubles valued above
   `BaubleCheapMaxValue` (6) go on a shelf; cheap ones are destroyed on sale.
   This stops twelve value-1 trinkets evicting shelved gear and closes the
   bauble Bartering loop (sections 2, 13).
6. **The backroom is capped at the shelf cap (12).** A shop holding that
   many refuses another hot bauble. A fence says so in its own voice
   (`baubleSayBackroomFull`, "I can't move any more hot goods right now.
   Come back once some of what I'm sitting on has cooled."); an honest shop
   refuses in a neutral voice of its own (`baubleSayNoRoom`, "I'm afraid
   I've no room for more of those right now."), added by controller
   decision during plan review, since an honest merchant would not talk
   about hot goods. Held entries count against the held cap, not the listed
   cap (section 2).
7. **Only a player's sale shelves a bauble.** A mob's sale destroys it,
   because the shop never pays a mob (section 2).
8. **Give-back after a buyback is intended.** It costs about 40% of value
   plus the hold (section 5).

## Out of scope

Salvaging baubles into materials, heat stepping by zone, the torch
Bartering loop, and any other dashboard change (no panel, no shelf counts).
