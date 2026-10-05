# Baubles Slice D: Shelf Resale Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A player's sale of an average or rare, non-retired bauble to a living-economy shop puts it on that shop's secondhand shelf instead of destroying it; `list` shows the shelf per viewer, `buy` sells from it by position, a hot bauble waits in a capped backroom until it cools, a buyback returns the record to its unsold status, and the economy dashboard types a fence's shop `fence`.

**Architecture:** The shelf is the existing `ShopInventory.AffixedStock`. Each entry gains a wall-clock `AddedAt` and a `HoldUntil`; `internal/shops/shelf.go` adds one clock (`ShelfNow`) and the listed/held helpers that `list`, `buy` and the sale all share. The listed cap (`ShopAffixedStockCap`, 8 to 12, moved into SHOP ECONOMY with a `config.yaml` key) is enforced lazily on add, `list` and `buy`. `buy` selects by index through a new `util.FindMatchIndexIn`. The bauble catalog gains `MarkBought`, `ShelfHoldUntil` and a shared `unsoldStatus`; `ShopSnapshot` gains a `Fence` flag and `Type()`.

**Tech Stack:** Go 1.25 (`go.mod`), `gopkg.in/yaml.v2` 2.4.0 for shop files and snapshots (honours `IsZero` for `omitempty`), testify, Git Bash for git, PowerShell for the boot check, Docker for `-race`.

**Spec:** `docs/superpowers/specs/2026-09-30-baubles-shelf-resale-design.md` (owner "lgtm" 2026-09-30, eight rulings, facts F1 to F51, test list 1 to 15).

## Where and when

- **Worktree:** `C:/tmp/dogmud-baubles-d-impl` (Git Bash path `/c/tmp/dogmud-baubles-d-impl`). Every path below is relative to it.
- **Branch:** `feature/bauble-shelf-resale`, created fresh from `origin/master` AFTER the docs PR that carries this plan (branch `docs/baubles-slice-d-spec`) merges.
- **BASE:** the `origin/master` SHA the branch is cut from, written to `C:/tmp/dogmud-baubles-d-impl.base` in Task 0 (this plan names that file, which is what keeps it through the `C:/tmp` sweep; delete it when the branch merges).
- **One PR** (Task 19): about 53 files (39 code and test files, counted in the dry run, and 14 docs) and under 3,500 lines, far under the 300-file / 20,000-line limits. CI minutes are exhausted until 10-01, so the local gate in Task 18 is the gate, and its output goes into the PR body.
- **Nothing is deployed.** The owner runs deploys. Claude prepares and merges.
- Throwaway output (test logs, review notes, the PR body draft) goes in the session scratchpad, never `C:/tmp`. The only `C:/tmp` entries this plan creates are the worktree, its `.base` file, the playtest worktree (Task 17) and the boot-check worktree (Task 18), each removed by the task that made it.

## Facts verified against source

Read at HEAD `25b7d34d0` of `docs/baubles-slice-d-spec`, whose code is byte-identical to `origin/master` `6b6ff7ddf` (`git diff --stat origin/master HEAD` lists only the spec and `docs/README.md`), on 2026-09-30. Balance values are from `git show HEAD:_datafiles/config.yaml`. Task 0 re-checks every Edit anchor against the new BASE.

| # | Fact | Where |
|---|------|-------|
| P1 | `AffixedStockEntry{Item items.Item "item"; Price int "price"; AddedRound uint64 "added_round,omitempty"}` | `internal/shops/shopinventory.go:72-76` |
| P2 | `AddAffixedStock(item items.Item, price, cap int)` appends with `AddedRound: util.GetRoundCount()` and drops index 0 while `len > cap`; `RemoveAffixedStock(idx int) (items.Item, bool)` | `shopinventory.go:141-152`, `:155-162` |
| P3 | `shopinventory.go` imports `slices`, `economy`, `items`, `util`; no `time` | `shopinventory.go:3-9` |
| P4 | Shop files marshal with `gopkg.in/yaml.v2` v2.4.0 (`persistence.go:16`, `go.mod:27`), whose `omitempty` calls `IsZero()` (`yaml.go:424` in the module cache) | `internal/shops/persistence.go` |
| P5 | `SaveShop(zone, mobId, roomId) error` needs the shop cached; the path is `<DataFiles>/shops/<zone sanitized>/<mob>-room<room>.yaml`; `GetShopInventory` loads from disk on a cache miss, nil when there is no file; `RegisterShop` writes nothing unless it migrates `CraftSupport` | `persistence.go:31-37, 42-67, 74-110, 151-183` |
| P6 | `AddAffixedStock` callers: `internal/actions/sell.go:393` (cap from `:392`), `modules/auctions/npc_buyers.go:297` (cap from `:296`), `internal/actions/buy.go:639` (rollback, cap 0). Test callers: `shopinventory_test.go:459, 460, 477, 495`, `internal/actions/buy_test.go:29`. `bauble_sweep_test.go:92` builds an `AffixedStockEntry` literal with named fields | repo grep |
| P7 | `npc_buyers.go` imports `configs`, `items`, `mudlog`, `shops`, `uuid` (no stdlib group); `modules/auctions/auctions.go` already imports `internal/baubles`, so the package can | `npc_buyers.go:3-9` |
| P8 | `baubles` imports `items` (not the reverse), so `baubles.ShelfHoldUntil(itm items.Item, ...)` adds no cycle; `shops` imports neither `baubles` nor `actions` | `go list -f '{{.Imports}}'` |
| P9 | `ShopAffixedStockCap ConfigInt yaml:"ShopAffixedStockCap"` sits in the `LOOT` block with comment "(default 8)"; SHOP ECONOMY ends its bartering pair with `BarterMaxBonus` at `:885`; no `config.yaml` key (grep finds none); default `if b.ShopAffixedStockCap <= 0 { b.ShopAffixedStockCap = 8 }` in `validateMisc`; `BarterMaxBonus` default in `validateShops` | `internal/configs/config.balance.go:950, 885`; `config.balance.misc.go:312-313`; `config.balance.shops.go:44-45` |
| P10 | `config.yaml` SHOP ECONOMY: `BarterMaxBonus: 0.15          # Max fractional sell-price bonus from bartering` is the section's last line, followed by a blank line and `# ── BAUBLES` | `config.yaml:1435` |
| P11 | In a fresh worktree `git ls-files -v _datafiles/config.yaml` prints `H` (no skip-worktree bit); the main checkout's copy carries `S` | this worktree |
| P12 | Test binaries never read `config.yaml`: `configData` starts as `newUnloadedConfig()` and `GetBalanceConfig` validates it lazily (`ensureConfigValidated`), so an unpinned test sees Go defaults. `SetConfigForTest(t, c)` swaps the whole config and restores it on cleanup, without re-validating | `internal/configs/configs.go:24, 287-300, 496-500`; `testing_support.go:33-44`; `config.balance.go:1318-1325` |
| P13 | Shipped: `BaubleStolenHeatHours: 72` (`config.yaml:1519`), `BaubleFenceBuyPct: 60` (`:1530`), `BaubleFenceGroups: [fence]` (`:1531`), `BaubleCheapMinValue: 1`, `BaubleCheapMaxValue: 6`, `BaubleAverageMinValue: 10` (`:1546-1548`), `ShopBuyRatio: 0.50` (`:1418`), `BarterMaxDiscount: 0.15` (`:1434`), `BaublesEnabled: false` (`:1440`), `BaubleSearchChancePct: 1` (`:1444`), `BaubleBiomeChancePct:` (`:1445`), `BaubleRollsPerWindow: 2` (`:1474`), `BaublePickpocketChancePct: 50` (`:1486`), tier weights 70/25/5 (`:1491-1493`). Go defaults equal them (72, 60, `fence`, 1/6/10) | `config.yaml`; `config.balance.baubles.go:43-51, 73` |
| P14 | `baubles.TierCheap.Range()` reads the validated ladder (`BaubleCheapMinValue`/`BaubleCheapMaxValue`, reset as a whole when inconsistent) | `internal/baubles/tiers.go:45-53`; `config.balance.baubles.go:253-265` |
| P15 | `sellBaubleToMerchant` (`sell_bauble.go:198-257`) prices via `baubleOfferFor`, draws merchant gold only for a player seller (`:217-228`), then at `:240-247` saves a living shop with the comment "The bauble is not stocked: it leaves the world", then `baubles.MarkSold` at `:248` | `internal/actions/sell_bauble.go` |
| P16 | `baubleOfferFor(item, shopInv, fence, zone)` `:108-138`: switch fence+stolen, fence, not-a-buyer, `HotIn` (`baubleNowForSale()`), default; then the living-shop reserve check starting `if shopInv != nil {` / `ratio := float64(configs.GetBalanceConfig().ShopGoldReserveRatio)` `:128-129` | `sell_bauble.go` |
| P17 | Refusal consts block `:40-46` (`baubleSayUnknown`, `baubleSayNotBuyer`, `baubleSayCantAfford`, `baubleSayHot`); header comment `:18-25` ends "never added to stock, and the record marked sold." | `sell_bauble.go` |
| P18 | `var baubleNowForSale = time.Now` `:61`; test `pinStolenClock` swaps `baubleNowForSale` and `stolenNow` | `sell_bauble.go:61`; `stolen_bauble_test.go:39-46` |
| P19 | `sell.go:317` comment "Baubles (docs/baubles): catalog-priced, never stocked. See sell_bauble.go."; `sell.go` imports no `baubles` | `internal/actions/sell.go` |
| P20 | `tryPurchaseFromInventory` `buy.go:514-709`: `invEntry{entry, item, plainName, price, affixedIdx}` `:517-523`; stock loop `:535-564`; affixed loop over every `AffixedStock` entry `:566-583`; `util.FindMatchIn` `:585`; no-match "Any interest" say `:589-599`; first-`plainName` loop `:601-610`; encumbrance gate `:612-619`; affixed purchase `:624-660` with rollback `AddAffixedStock(bought, matched.price, 0)` `:639`, `SaveShop` `:652-654`, buyer line `bought.DisplayName()` `:655`, room line `:657`; stock-path `SaveShop` `:695-697` | `internal/actions/buy.go` |
| P21 | `buy.go` imports neither `baubles` nor `time`; `Actor.GetUserId()` is 0 for a `MobActor` | `buy.go:3-21`; `actor_mob.go:63-65` |
| P22 | `util.FindMatchIn(searchName, items...) (match, closeMatch string)` `util.go:353-405`, over `GetMatchNumber` (`:322-351`, `N.item`, `item#N`) and `stringMatch` (`:422-442`, `NormalizeForMatch` both sides, prefix pass then contains pass) | `internal/util/util.go` |
| P23 | `util_test.go:548-575` `TestFindMatchIn` cases; `matchnormalize_test.go` apostrophe cases | `internal/util` |
| P24 | Viewer-aware accessors `GetSpecFor`, `DisplayNameFor`, `NameFor`, `LongDescriptionFor` (pointer receivers); `NameFor` returns `Name()` for a non-bauble; `IsBauble()` is `Bauble != ""`; a finder-only record (`PlayerKey && !Moderated`) shows `Trinket` to everyone but `FoundByUserId` | `internal/items/bauble_viewer.go:13-46`; `bauble.go:67-69`; `internal/baubles/record.go:144-181` |
| P25 | `finderViewSites` rows are `"path\|Func": {calls, why}` (plain function name, or `Type.Method`); an unlisted reference or a count mismatch or a stale row fails `TestFinderViewReachesOnlyItsReader`; a `SendText` passes when its receiver and the viewer argument share a root identifier. No row today for `buy.go` or `list.go` | `bauble_finder_view_guard_test.go:71-82, 352-403` |
| P26 | `List` (`list.go:26-121`): for a mob with a `ShopInventory` it builds stock (`:59-67`) and says "I have nothing to sell right now, but check again later." when `renderMobMerchantListing` returns false (`:68-70`); `renderShopTable(user, title, colorPattern, sellerName, sellerTag, headers, rows, helpText)` `:433-438` sends to `user` alone; items table sorted by the Qty string `:454-456`. `list.go` imports no `configs`, `mudlog` or `time` | `internal/usercommands/list.go` |
| P27 | `usercommands` tests never register a templates filesystem, so `templates.Process("tables/shoplist", ...)` renders empty there: a table's text cannot be asserted through `List` (the comment in `list_sight_test.go:92-107`). `listSightRoom(t, "city")` seeds room 8410, merchant mob 841 (instance 8412, `HomeRoomId` 8410, zone `TestZone`, one `Character.Shop` item so `HasShop`), shopper user 8411, comfortably lit | `internal/usercommands/list_sight_test.go:37-89` |
| P28 | `admin.bauble.go:248` prints `sold:` only when `rec.Status == baubles.StatusSold`; `:218` the `[status]` header; `SalesSince` at `:292-293, 390-391` | `internal/usercommands/admin.bauble.go` |
| P29 | `Restore` `admin.go:111-128` sets `StatusFallback`, then `StatusReady` when `r.Generator.Named()`, then `StatusSold` when `SoldValue > 0`, only from `retired`; `Generator.Named()` is openai or corpus | `internal/baubles/admin.go`; `record.go:42-45` |
| P30 | `MarkSold` `sales.go:20-30`; `SalesSince` `:35-45` tests `r.Status == StatusSold && !r.SoldAt.Before(t)` at `:39`; the "every record is sellable" comment `:13-16`; `StatusSold` comment "the item is gone" `record.go:17` | `internal/baubles` |
| P31 | `Record.Hot(now)`, `StolenGoods()`, `HotIn(zone, now)`, `HeatDuration()`, `ItemIsHotIn` `theft.go:88-151`; `MarkRecognized` begins at `:153` | `internal/baubles/theft.go` |
| P32 | `baubles` tests: `withCatalog(t)` seeds carrier 900 and a temp catalog (`catalog_test.go:28-52`); `seedRecord` (`admin_test.go:8-18`); `setBaubleConfig` (`find_test.go:12-17`); package `TestMain` sets the logger (`catalog_test.go:17`) | `internal/baubles` |
| P33 | `actions` tests: `seedBaubleSale`, `newBauble` (tier average, any value, weight 0.6) `sell_bauble_test.go:20-62`; `seedSellRoom` (room 1, `TestZone`, lamp 60), `seedSellMerchant` (template 2, instance 301, `HomeRoomId` 1), `newSellerActor` (player user 1 or mob) `sell_test.go:88-172`; `stolenTestNow`, `stolenBauble` `stolen_bauble_test.go:35-55`; the package sets up the logger (`actions_test.go:16`) | `internal/actions` |
| P34 | `TestSell_Bauble_LivingShopByCraftSupport` asserts `assert.Len(t, si.AffixedStock, 0, "baubles are not resold like affixed loot")` at `:191` after the comment "A general store does, from its own gold, and shelves nothing." | `sell_bauble_test.go:160-192` |
| P35 | `ShopSnapshot.CraftSupport` (`yaml:"craft_support" json:"craft_support"`) `snapshot.go:65`; `captureShops` `capture.go:42-111`; `mobs.GetMobSpec(MobId) *Mob` returns a copy or nil; `(*Mob).IsFence()` nil-safe | `internal/economy/health`; `internal/mobs/mobs.go:803-811, 999-1013` |
| P36 | `PerCraftSupportScores` keys `buckets[s.CraftSupport]` `scoring.go:151-155`, doc `:135-139`; `ShopScoreRow.CraftSupport: s.CraftSupport` `:825` in `ScoreWithConfig`; page groups on `s.craft_support \|\| "(uncategorized)"` `index.html:314`, per-shop cell `row.CraftSupport` `:351` | `scoring.go`; `_datafiles/html/admin/economy/index.html` |
| P37 | `health` tests: package `health_test` has `TestMain` (logger) and `testScoringCfg`; `mobs.SeedMobsForTest(specs, instances) func()` | `capture_test.go:58-61`; `scoring_test.go`; `internal/mobs/test_helpers.go:8-62` |
| P38 | Fences: 104 Fence Dealer Siv (Thornwall City, `craft_support: general`, room 475 Back Alley, East); Jeweler Tess 108 (jewelcrafting, room 482) | `_datafiles/world/dogmud/mobs/thornwall_city/104-fence_dealer_siv.yaml`, `108-jeweler_tess.yaml`; rooms `475.yaml`, `482.yaml` |
| P39 | Every `context.md` and help template here is CRLF in the working tree (`core.autocrlf=true`); Go files are LF in the index. Edit anchors below are single-line or LF-agnostic | `git ls-files --eol` |
| P40 | `tools/context_md_audit.py` baseline at HEAD: 14 packages with 27 phantom symbols; none in `shops`, `baubles`, `economy/health`, `actions`, `usercommands`, `util` or `modules/auctions`; `internal/configs` already has 2 | run at HEAD |
| P41 | New names are unused: `FindMatchIndexIn`, `ShelfNow`, `ListedIndexes`, `HeldCount`, `EnforceAffixedCap`, `RestoreAffixedStock`, `ShelfHoldUntil`, `MarkBought`, `unsoldStatus`, `baubleShelvable`, `baubleSayBackroomFull`, `buildShelfRows`, `renderShelfListing` (grep over `internal`, `modules`, root) | repo grep |
| P42 | Root guards present: `TestFinderViewReachesOnlyItsReader`, `TestFinderViewGuardCatchesALeak`, `TestEveryCreatureLookupDeclaresItsViewer`, `TestLivingStateWritesAreDurable`, `TestNoHandRolledTempRename`, `TestItemWalkersVisitEveryItemField`, `TestEveryItemHolderIsASweepRootOrTransient`, `TestBaubleSweepSourcesMatchTheGuardedRoots`, `TestBaubleSweepReadsEveryStoreFromDisk`, `TestEveryTextSurfaceIsRegistered`, `TestNoRawEventsMessageOutsidePipeline`, `TestSmoke_NoNewSilentlyIgnoredYAMLKeys`, `TestSmoke_ServerBootsCleanWithRealData` (`DOGMUD_BOOT_SMOKE=1`) | repo root `*_test.go` |
| P43 | `golangci-lint` at `~/go/bin/golangci-lint`; `.golangci.yml` enables govet, staticcheck (SA only), errcheck (not on tests), ineffassign, unconvert; `compose.test.yml` service `test` | repo root |
| P44 | The playtest harness is present at `../gomud-playtest-harness`; profile `mid` starts at room 462 (Thornwall City) with 500 gold and `search: 15` | `tools/playtest/profiles/mid.yaml` |
| P45 | `Buy` parses a leading integer followed by a space as a quantity (`buy 5 iron ingot`); `2.trinket` has no space and reaches the matcher, where `GetMatchNumber` reads it as "the second trinket" | `internal/actions/buy.go:296-303`; `util.go:322-351` |
| P46 | The AI companion's `browseShops` returns `shopListing{... Wares []ware}` "as a player's `list` would show it", stock rows only, sorted by name; `rememberShop` keys `Mind.Shops[mob].Wares` by `ItemId`; `describeListing` refs are keyed by `ItemId`. Callers: `browseAction` (`economy.go:225`), `check_wares` (`tools.go:149`), arrival (`travel.go:210`) | `modules/aicompanion/economy.go:60-166` |
| P47 | The companion names every item it tells the model about with `items.Item.ModelName()`, which returns the carrier's own name for a bauble whose text a player's key wrote (`playerTextCarrier`), finder-only or moderated, and `Name()` otherwise | `internal/items/bauble_model.go:11-16`; `modules/aicompanion/perception.go:184-213`, `scene.go:149, 278, 288` |
| P48 | Docs that still say sold baubles are never resold: `docs/baubles/implementation-plan.md:162` ("Never stocked or resold"), `:179-181` (Phase 2 exit check), `:877` ("slice D (the owner's), not here"), `:967` (open question 6); `templates/admincommands/help/command.bauble.template:128` ("Sold baubles leave the world."); `internal/baubles/context.md:361, 456` and `internal/baubles/sweep.go:46-50` (a sold record reaches a merchant again only after a crash) | files named |
| P49 | `internal/shops/context.md` "Pricing Config Knobs" lists `BarterMaxDiscount` and `BarterMaxBonus` as dead (hard-coded `0.15`), and a Gotcha repeats it; both are read today (`buy.go:533`, `sell.go:346`) | `internal/shops/context.md:103-142, 292-297` |
| P50 | The economy page groups a shop with no craft_support under `"(uncategorized)"` and then reads `d.scores.PerCraftSupport["(uncategorized)"]`, a key `PerCraftSupportScores` never writes (it uses `""`), so that row always shows no score | `index.html:314, 324`; `scoring.go:151` |

**Dry run.** Before this plan was committed, the code blocks of Tasks 1 to 13 were applied mechanically, by their exact old texts, to a scratch worktree at HEAD `25b7d34d0`: every anchor matched once, `gofmt` was clean after the `gofmt -w` steps, and `go build ./...`, `go vet`, the tests of every touched package and the full root package passed. The dry run found two defects, fixed above: a bare mob fixture cannot carry a bauble (Task 9 now gives it strength), and a one-line helper gofmt rewrites (Task 11). After the plan review the dry run was repeated for every task the revisions changed (2, 6, 8a, 8b, 10, 11, 12, 13, 13b, and Task 16's `sweep.go` comment), on top of Tasks 1 to 13b, and each revised null probe was applied, confirmed to compile and to go red for its named reason, and reverted.

### Where the code differs from the spec

None of these changes a design decision; each is recorded so a reviewer does not re-derive it.

1. **Line ranges.** Spec F21, F23 and F24 quote `buy.go` 584-609, 611-617 and 628-664; at HEAD they are 585-610, 612-619 and 624-660 (P20). Every other spec line number checked matches.
2. **Cheap-tier bound.** The spec writes `rec.Value > int(BaubleCheapMaxValue)`. The plan reads the same knob through `baubles.TierCheap.Range().Max` (P14), the accessor every other tier check uses, which also sees the ladder's own consistency reset.
3. **`invEntry.plainName` becomes dead** once `buy` selects by index; Task 12 deletes the field instead of leaving a written-never-read value.
4. **Parameter name.** `EnforceAffixedCap` and `AddAffixedStock` name the cap `limit`, not `cap`, so new code does not shadow the builtin.
5. **Test clock helper.** `pinStolenClock` ("sets every clock the stolen-bauble code reads") also pins `shops.ShelfNow` from Task 9 on, so the existing fence tests keep one consistent clock.
6. **`list` table text is not assertable in `usercommands` tests** (P27). Tests assert `buildShelfRows`, `renderShelfListing`'s return and the shop's saved state; the rendered "Secondhand goods by Siv" table is checked by the playtest (Task 17).
7. **Colour of the new table title.** The spec names the title, not its colour pattern; the plan uses `cyan`, the pattern of the `Items available` table beside it, and `help list` names it in cyan too.
8. **Stale docs beyond the spec's list** (P48, P49): the spec's section 11 names the `context.md` files and help; the plan also corrects `docs/baubles/implementation-plan.md` (and adds Phase 6f), the admin `bauble` help, two `internal/baubles/context.md` lines, the `sweep.go` comment, and the two barter knobs `internal/shops/context.md` wrongly calls dead.
9. **A pre-existing dashboard bug on the line Task 8b edits** (P50) is fixed there, with a static page check.

### Revisions after the plan review (2026-09-30)

Two blind reviews found no Critical issue. Each item below was applied and the affected tasks re-run in the dry run.

- **Two controller decisions (design changes, made by the controller, not by this plan):**
  - The AI companion's `browse` shows the shelf too (new Task 13b). The controller asked for the generic view, with a finder-only bauble reading "Trinket". The plan uses the companion's existing rule instead, `ModelName()` (P47): a finder-only bauble reads as its carrier, "Curious Trinket", and so does a moderated player-key bauble, whose real name `Name()` would have shown the model. That is stricter than asked, and it is the mechanism the module already uses everywhere else; flagged for the controller in the hand-off.
  - An honest shop with a full backroom says `baubleSayNoRoom` ("I'm afraid I've no room for more of those right now."), a fence keeps `baubleSayBackroomFull` (Task 10, both tested).
- **Null probes that did not compile** now go red for the named reason: Task 8a (`tmpl != nil && false`), Task 9 probe 3 (`ok && rec.Id != ""`), Task 11 probes 2 and 3 (`if false && ...`, `if err := error(nil); ...`), Task 12 probe 2 (`!shopSaved && false`), Task 13 probe 2 (`_ = baubles.StatusSold`).
- **Task 0 Step 1** checks the plan on master with `git cat-file -e` (a merge commit's `show --stat` lists no files). Step 2 also checks every docs anchor Task 16 edits and the Task 10, 11 and 13b code anchors.
- **Task 2** runs `gofmt -w` over all four Go files it edits.
- **Expected outputs corrected:** Task 4's probe message, Task 5's build and vet lines (the build stops at `internal/actions`; vet reports `:478`, `:496`, `buy_test.go:30`), Task 9's lowercase testify wording, Task 0's audit lines.
- **New assertions:** the "nothing to sell" say fires only when both tables are empty (Task 11, probe `&&` to `||`); `TestMarkBoughtReturnsTheUnsoldStatus` seeds a never-sold record so dropping `!r.SoldAt.IsZero()` goes red (Task 6 probe 2b); the snapshot test checks the JSON tag too; the shelf-order test uses three rows whose order differs from any name or price sort.
- **Docs:** Task 16 fixes a code fence and covers P48 and P49; help contrasts `buy 2.trinket` with `buy 2 trinket` (P45); PATCH_NOTES says fences shelve too; the `FindMatchIndexIn` equivalence note now states the one corner where it differs.
- **Playtest:** `ShopAffixedStockCap: 2` makes eviction and the backroom refusal reachable; the goals text names Jeweler Tess and Fence Dealer Siv in the Back Alley, East; the plan states what only unit tests cover (a hold ending, finder-only views, the honest "no room" line).
- **Gate and PR:** the race run adds `internal/configs`, `modules/auctions` and `modules/aicompanion`; the size estimate is about 53 files; Task 19 uses the executing session's scratchpad, deletes the local branch after the worktree, and the PR body's deploy notes warn that rolling back past this PR makes backroom hot baubles buyable.

## File structure

| File | Change | Responsibility |
|---|---|---|
| `internal/util/util.go`, `internal/util/findmatchindex_test.go` | modify, create | `FindMatchIndexIn`; `FindMatchIn` wraps it |
| `internal/configs/config.balance.go`, `config.balance.misc.go`, `config.balance.shops.go`, `config.balance.shops_test.go`, `_datafiles/config.yaml` | modify | `ShopAffixedStockCap` into SHOP ECONOMY, default and shipped 12 |
| `internal/shops/shopinventory.go` | modify | `AddedAt`, `HoldUntil`; `AddAffixedStock` signature |
| `internal/shops/shelf.go`, `internal/shops/shelf_test.go` | create | `ShelfNow`, `Held`, `ListedAt`, `HeldCount`, `ListedIndexes`, `EnforceAffixedCap`, `RestoreAffixedStock` |
| `internal/shops/shopinventory_test.go` | modify | new `AddAffixedStock` signature |
| `internal/baubles/theft.go`, `theft_test.go` | modify | `ShelfHoldUntil` |
| `internal/baubles/admin.go`, `sales.go`, `record.go`, `sales_test.go` (create) | modify | `unsoldStatus`, `MarkBought`, `SalesSince` by `SoldAt`, comments |
| `internal/usercommands/admin.bauble.go`, `admin.bauble_test.go` | modify | `bauble show` prints `last sold:` when `SoldValue > 0` |
| `internal/economy/health/snapshot.go`, `capture.go`, `scoring.go`, `fence_type_test.go` (create), `_datafiles/html/admin/economy/index.html` | modify | `Fence`, `Type()`, grouping |
| `internal/actions/sell_bauble.go`, `sell.go`, `sell_bauble_test.go`, `stolen_bauble_test.go` | modify | shelving, backroom refusal, comments, tests |
| `modules/auctions/npc_buyers.go` | modify | new `AddAffixedStock` call shape |
| `internal/actions/buy.go`, `buy_test.go`, `buy_shelf_test.go` (create) | modify | shelf order, held exclusion, index selection, lazy trim, rollback, buyer's view, `MarkBought` |
| `internal/usercommands/list.go`, `list_shelf_test.go` (create) | modify | lazy trim, "Secondhand goods" table |
| `modules/aicompanion/economy.go`, `economy_shelf_test.go` (create) | modify | `browse` shows the shelf in the model's view (controller decision) |
| `internal/economy/health/economy_page_test.go` (create) | create | static check of the dashboard's grouping and score lookup |
| `bauble_finder_view_guard_test.go` | modify | two `finderViewSites` rows |
| eight `context.md` files, `docs/baubles/implementation-plan.md`, the `sweep.go` comment, three player help templates, the admin `bauble` help, `docs/PATCH_NOTES.md` | modify | docs (Task 16) |

## Spec coverage map

| Spec | Tasks |
|---|---|
| 1 Shelf entry and helpers | 3, 4, 5 |
| 2 Selling (shelving, backroom, comments) | 9, 10 |
| 3 `list` | 11 |
| 4 `buy` | 5 (rollback), 12, 13 |
| 5 Record | 6, 7, 13 |
| 6 Dashboard | 8a, 8b |
| 7 Config | 2 |
| 8 Concurrency | 14 review checklist (no new lock; every new mutation sits in a command or the sale, under the mud lock) |
| 9 Persistence | 3 (round trip, old files), 11 and 12 (lazy trim saved) |
| 10 Other callers | 5 |
| 11 Docs and help | 16 (and comments in 6, 8b, 9) |
| Review decision: companion `browse` | 13b |
| Review decision: honest backroom line | 10 |
| 12 Tests 1 to 15 | 1: 11, 12. 2: 3, 12. 3: 3. 4: 3, 11. 5: 5 (add), 11 (list), 12 (buy). 6: 10. 7: 9. 8: 9. 9: 12. 10: 1. 11: 11, 13. 12: 6, 7, 13. 13: 8a, 8b. 14: 3 (and 5 for the rollback call). 15: 9, 18 |
| Rulings 1 to 8 | 1: 9 (retired destroyed). 2: 6 (`unsoldStatus`). 3: 6 (no `BoughtAt`, `Restore` unchanged). 4: 13. 5: 9 (`baubleShelvable`). 6: 10. 7: 9. 8: 6, 13 (no guard added against give-back; the return credit path is untouched) |

## Task list and PR grouping

All tasks land in one PR on `feature/bauble-shelf-resale`.

| Task | Title | Files | Subagent |
|---|---|---|---|
| 0 | Preconditions and anchors | none | sonnet |
| 1 | `util.FindMatchIndexIn` | 2 | sonnet |
| 2 | `ShopAffixedStockCap` into SHOP ECONOMY at 12 | 5 (a knob move the compiler does not enforce; kept together so the default and the key never disagree) | sonnet |
| 3 | Shelf entry fields and helpers | 3 | sonnet |
| 4 | `baubles.ShelfHoldUntil` | 2 | sonnet |
| 5 | `AddAffixedStock` signature and its callers | 6 (compiler-forced) | sonnet |
| 6 | `unsoldStatus`, `MarkBought`, `SalesSince` | 4 | sonnet |
| 7 | `bauble show` keeps the last sale | 2 | sonnet |
| 8a | `ShopSnapshot.Fence` and `Type()` | 3 | sonnet |
| 8b | Dashboard groups by `Type()` | 3 | sonnet |
| 9 | A player's shelvable sale goes on the shelf | 4 | opus |
| 10 | The backroom refusal | 2 | opus |
| 11 | `list` shows the shelf | 3 | opus |
| 12 | `buy` from the shelf by position | 2 | opus |
| 13 | `buy` in the buyer's own view; buyback | 3 | opus |
| 13b | The AI companion's `browse` shows the shelf | 2 | opus |
| 14 | Checkpoint: foundations review (after Task 8b) | none | opus |
| 15 | Code review of the whole branch | none | opus |
| 16 | Docs | 17 (docs only, plus one Go comment) | sonnet (load `dogmud-player-copy`) |
| 17 | Adversarial playtest | none committed unless fixes | opus (controller) |
| 18 | Local gate | none | sonnet |
| 19 | PR and merge | none | sonnet |

Execution order: 0, 1, 2, 3, 4, 5, 6, 7, 8a, 8b, 14, 9, 10, 11, 12, 13, 13b, 15, 16, 17, 18, 19.

Every task's test gate includes `go test . -count=1` at the repo root (`dogmud-writing-tests`: root guards are line- and function-keyed and go red in packages nobody thought they touched).

---

### Task 0: Preconditions and re-verification

**Files:** none.

- [ ] **Step 1: The docs PR has merged; cut the branch and record BASE**

```bash
git -C "/c/Users/Calabe Davis/workspace/DOGMud" fetch origin
git -C "/c/Users/Calabe Davis/workspace/DOGMud" log --oneline -3 origin/master
git -C "/c/Users/Calabe Davis/workspace/DOGMud" cat-file -e origin/master:docs/superpowers/plans/2026-09-30-baubles-shelf-resale.md; echo "plan-on-master-exit=$?"
git -C "/c/Users/Calabe Davis/workspace/DOGMud" log --oneline -1 origin/master -- docs/superpowers/plans/2026-09-30-baubles-shelf-resale.md
```
Expected: `plan-on-master-exit=0`, and the log names the commit that last changed this plan (reachable from `origin/master`, so the docs PR has merged). A non-zero exit: STOP, the branch must come from master after the docs PR. (`git show --stat` on a merge commit prints no file list, so it cannot answer this.)

```bash
ls /c/tmp/dogmud-baubles-d-impl 2>/dev/null | head -1
```
Expected: no output (exit 2). If the directory exists, STOP and ask.

```bash
git -C "/c/Users/Calabe Davis/workspace/DOGMud" worktree add -b feature/bauble-shelf-resale /c/tmp/dogmud-baubles-d-impl origin/master
cd /c/tmp/dogmud-baubles-d-impl && git rev-parse HEAD > /c/tmp/dogmud-baubles-d-impl.base && cat /c/tmp/dogmud-baubles-d-impl.base && git ls-files -v _datafiles/config.yaml
```
Expected: a SHA, then `H _datafiles/config.yaml` (no skip-worktree bit in a fresh worktree, P11: edit and commit `config.yaml` here normally, and check `git diff` shows only the intended lines).

- [ ] **Step 2: Every Edit anchor exists exactly once**

```bash
cd /c/tmp/dogmud-baubles-d-impl && while IFS= read -r line; do f="${line%% ::: *}"; s="${line#* ::: }"; printf '%s  %s  %s\n' "$(grep -cF -- "$s" "$f")" "$f" "$s"; done <<'EOF'
internal/util/util.go ::: func FindMatchIn(searchName string, items ...string) (match string, closeMatch string) {
internal/configs/config.balance.go ::: ShopAffixedStockCap ConfigInt   `yaml:"ShopAffixedStockCap"` // Max per-instance affixed items a shop resells before evicting oldest (default 8)
internal/configs/config.balance.go ::: BarterMaxBonus              ConfigFloat `yaml:"BarterMaxBonus,omitempty"`           // Max fractional sell-price bonus a player can get via bartering (default 0.15)
internal/configs/config.balance.misc.go ::: b.ShopAffixedStockCap = 8
internal/configs/config.balance.shops.go ::: b.BarterMaxBonus = 0.15
_datafiles/config.yaml ::: BarterMaxBonus: 0.15          # Max fractional sell-price bonus from bartering
internal/configs/config.balance.shops_test.go ::: import "testing"
internal/shops/shopinventory.go ::: AddedRound uint64     `yaml:"added_round,omitempty"` // for age-based clutter eviction
internal/shops/shopinventory.go ::: func (si *ShopInventory) AddAffixedStock(item items.Item, price, cap int) {
internal/shops/shopinventory_test.go ::: si.AddAffixedStock(a, 200, 100)
internal/shops/shopinventory_test.go ::: si.AddAffixedStock(b, 150, 100)
internal/shops/shopinventory_test.go ::: Spec: &items.ItemSpec{Value: 100}}, 50, 3)
internal/shops/shopinventory_test.go ::: Spec: &items.ItemSpec{Value: 400, PhysicalMitigation: 5}}, 200, 8)
internal/actions/sell.go ::: shopInv.AddAffixedStock(item, item.GetSpec().Value, c)
internal/actions/sell.go ::: // Baubles (docs/baubles): catalog-priced, never stocked. See sell_bauble.go.
internal/actions/sell.go ::: "github.com/GoMudEngine/GoMud/internal/characters"
modules/auctions/npc_buyers.go ::: s.bound.AddAffixedStock(item, item.GetSpec().Value, c)
modules/auctions/npc_buyers.go ::: "github.com/GoMudEngine/GoMud/internal/configs"
internal/actions/buy.go ::: shopInv.AddAffixedStock(bought, matched.price, 0) // roll back on carry failure
internal/actions/buy_test.go ::: Spec: &items.ItemSpec{Value: 400, Name: "iron sword", NameSimple: "sword", Type: items.Weapon}}, 400, 8)
internal/actions/buy.go ::: match, closeMatch := util.FindMatchIn(request, itemNames...)
internal/actions/buy.go ::: // Per-instance affixed resale stock (Stage 3): unique bought-back gear,
internal/actions/buy.go ::: bought.DisplayName()))
internal/actions/buy.go ::: plainName  string
internal/actions/buy.go ::: cfg := shops.PricingConfigFromBalance()
internal/baubles/admin.go ::: r.Status = StatusFallback
internal/baubles/sales.go ::: if r.Status == StatusSold && !r.SoldAt.Before(t) {
internal/baubles/sales.go ::: // Every record is sellable, a sold one included: a sold record only reaches a
internal/baubles/record.go ::: // sold to a merchant; the item is gone
internal/baubles/theft.go ::: // MarkRecognized records that the bauble's owner recognised it on someone.
internal/usercommands/admin.bauble.go ::: if rec.Status == baubles.StatusSold {
internal/economy/health/capture.go ::: Name:             lookupShopMobName(inv.MobId, inv.RoomId),
internal/economy/health/snapshot.go ::: CraftSupport     string          `yaml:"craft_support"      json:"craft_support"`
internal/economy/health/snapshot.go ::: // StockSnapshot is a per-item entry.
internal/economy/health/scoring.go ::: b, exists := buckets[s.CraftSupport]
internal/economy/health/scoring.go ::: buckets[s.CraftSupport] = b
internal/economy/health/scoring.go ::: CraftSupport:    s.CraftSupport,
internal/economy/health/scoring.go ::: // roll into key "", shown in the UI as "(uncategorized)". Startup
_datafiles/html/admin/economy/index.html ::: var disc = s.craft_support || "(uncategorized)";
internal/actions/sell_bauble.go ::: // The bauble is not stocked: it leaves the world, and its gold is the
internal/actions/sell_bauble.go ::: // own branch, like affixed loot does: priced from the catalog value times the
internal/actions/sell_bauble.go ::: baubleSayHot        = 
internal/actions/sell_bauble.go ::: ratio := float64(configs.GetBalanceConfig().ShopGoldReserveRatio)
internal/actions/stolen_bauble_test.go ::: origSale, origStolen := baubleNowForSale, stolenNow
internal/actions/sell_bauble_test.go ::: assert.Len(t, si.AffixedStock, 0, "baubles are not resold like affixed loot")
internal/actions/sell_bauble_test.go ::: // A general store does, from its own gold, and shelves nothing.
internal/usercommands/list.go ::: if !renderMobMerchantListing(user, stock, mob.Character.Name) {
internal/usercommands/list.go ::: "github.com/GoMudEngine/GoMud/internal/conditions"
internal/usercommands/list.go ::: "github.com/GoMudEngine/GoMud/internal/mobs"
internal/usercommands/list.go ::: // partitionShopStock splits shop stock into four categories: items, mercs, conditions, pets.
bauble_finder_view_guard_test.go ::: "internal/usercommands/inventory.go|Inventory":
bauble_finder_view_guard_test.go ::: "internal/actions/search_bauble.go|BaubleDelivery.deliver":
internal/actions/context.md ::: - Never stocked, never resold: the item leaves the world, the record is
_datafiles/html/admin/economy/index.html ::: var score = d.scores.PerCraftSupport[disc] || 0;
modules/aicompanion/economy.go ::: if inv := shops.GetShopInventory(m.Zone, int(m.MobId), m.HomeRoomId); inv != nil {
modules/aicompanion/economy.go ::: sort.Slice(l.Wares, func(i, j int) bool { return l.Wares[i].Name < l.Wares[j].Name })
modules/aicompanion/economy.go ::: wr := &WareRecord{Name: w.Name, Price: w.Price, Qty: w.Qty, SeenUnix: nowUnix}
modules/aicompanion/economy.go ::: parts = append(parts, fmt.Sprintf(`%s%s for %d gold`, ref, w.Name, w.Price))
modules/aicompanion/economy.go ::: Qty    int
internal/shops/context.md ::: constants; `StockEvent` depletion/refill event type.
internal/shops/context.md ::: items a shop holds; stock entries are counts of an item id.
internal/shops/context.md ::: value and the Go default are identical, so this fallback never actually
internal/shops/context.md ::: shop's gold pool held back before it will buy from a seller.
internal/shops/context.md ::: - **`BarterMaxDiscount`**: shipped `0.15`, Go default `0.15`. The buy-side
internal/shops/context.md ::: function) and never reads this field.
internal/shops/context.md ::: - **`BarterMaxDiscount` and `BarterMaxBonus` look tunable and are not.**
internal/baubles/context.md ::: - **sales.go**: `MarkSold`, `SalesSince`.
internal/baubles/context.md ::: goods; a theft clears it).
internal/baubles/context.md ::: seam (`SetPromptPreview`, `PreviewPrompt`) and `LooksLikeId`.
internal/baubles/context.md ::: func MarkSold(id string, gold int, sellerUserId int) bool
internal/baubles/context.md ::: pruner and it fails closed. A sold record held again (a crash rolled the
internal/baubles/context.md ::: record is sellable, a sold one included: it only reaches a merchant again
internal/baubles/sweep.go ::: // evidence would erase real ones.
internal/economy/health/context.md ::: ordinary loot) and rolls into the "(uncategorized)" key.
internal/usercommands/context.md ::: - **Trading**: `buy`, `sell`, `list`, `offer`, `appraise` - Commerce mechanics
internal/util/context.md ::: func FindMatchIn(searchName string, items ...string) (match, closeMatch string)
internal/util/context.md ::: `FindMatchIn` returns **two** results
modules/auctions/context.md ::: that has been spending cannot keep bidding, and outbid gold is refunded.
modules/aicompanion/context.md ::: - **economy.go**: `browse` (priced as `list` prices), shop and price
docs/baubles/implementation-plan.md ::: living-economy reserve are respected. Never stocked or resold; the record
docs/baubles/implementation-plan.md ::: `sell bauble` at a general store pay 5 to 8 gold and leave nothing on the
docs/baubles/implementation-plan.md ::: Resale of bought baubles is slice D (the owner's), not here.
docs/baubles/implementation-plan.md ::: 6. Sold baubles destroyed (recommended) or resold as curios.
docs/baubles/implementation-plan.md ::: ### Phase 7: Optional
_datafiles/world/dogmud/templates/admincommands/help/command.bauble.template ::: ShopBuyRatio; other shops refuse. Sold baubles leave the world.
_datafiles/world/dogmud/templates/help/sell.template ::: <ansi fg="command">sell all bauble</ansi> sells them all.
_datafiles/world/dogmud/templates/help/sell.template ::: three days are out, anyone who buys trinkets will take it.
_datafiles/world/dogmud/templates/help/list.template ::: This would list whatever the merchant is carrying.
_datafiles/world/dogmud/templates/help/buy.template ::: run out of gold or cannot carry any more.
docs/PATCH_NOTES.md ::: # DOGMud Patch Notes
EOF
```
Expected: every line starts with `1`. A `0` or `2`: master moved; re-read that file, fix the anchor in this plan (a doc-only commit on the branch), and carry on.

- [ ] **Step 3: New names are still free (expect-zero, run standalone)**

`grep -c` exits 1 on zero matches, so this runs on its own, not in an `&&` chain:

```bash
cd /c/tmp/dogmud-baubles-d-impl && grep -rnE "FindMatchIndexIn|ShelfNow|ListedIndexes|HeldCount|EnforceAffixedCap|RestoreAffixedStock|ShelfHoldUntil|MarkBought|unsoldStatus|baubleShelvable|baubleSayBackroomFull|buildShelfRows|renderShelfListing" --include=*.go internal modules *.go
```
Expected: no output.

- [ ] **Step 4: Baselines**

```bash
cd /c/tmp/dogmud-baubles-d-impl && go build ./... && go test . ./internal/util/ ./internal/configs/ ./internal/shops/ ./internal/baubles/ ./internal/actions/ ./internal/usercommands/ ./internal/economy/health/ ./modules/auctions/ -count=1 2>&1 | tail -12
python tools/context_md_audit.py 2>&1 | grep -E "^packages|^total"
```
Expected: every package `ok`; the audit prints three lines, `packages checked:                142`, `packages with phantom symbols:   14` and `total phantom symbols:           27` (P40). Record these numbers; Task 16 must not raise them.

No commit.

---

### Task 1: `util.FindMatchIndexIn`

**Files:**
- Modify: `internal/util/util.go:353-405`
- Create: `internal/util/findmatchindex_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/util/findmatchindex_test.go`:

```go
package util

import "testing"

// FindMatchIndexIn is FindMatchIn by position (baubles slice D): the same
// algorithm, so a shop's two rows with one name are told apart by index and
// `buy 2.trinket` takes the second. The cases are TestFindMatchIn's.
func TestFindMatchIndexIn_SameAnswersAsFindMatchIn(t *testing.T) {
	items := []string{"SWORD", "SHINING SWORD", "SHIELD", "BIG HELM", "HELMET", "GEM"}
	cases := []struct {
		search          string
		match, closeIdx int
	}{
		{"", -1, -1},
		{"SWORD", 0, 0},
		{"sword#2", -1, 1},
		{"HELM", -1, 4},
		{"G", -1, 5},
		{"iel", -1, 2},
		{"helm#2", -1, 4},
	}
	for _, tc := range cases {
		m, c := FindMatchIndexIn(tc.search, items...)
		if m != tc.match || c != tc.closeIdx {
			t.Errorf("FindMatchIndexIn(%q) = (%d, %d), want (%d, %d)", tc.search, m, c, tc.match, tc.closeIdx)
		}
		wantM, wantC := "", ""
		if tc.match >= 0 {
			wantM = items[tc.match]
		}
		if tc.closeIdx >= 0 {
			wantC = items[tc.closeIdx]
		}
		if gm, gc := FindMatchIn(tc.search, items...); gm != wantM || gc != wantC {
			t.Errorf("FindMatchIn(%q) = (%q, %q), want (%q, %q): the two must agree", tc.search, gm, gc, wantM, wantC)
		}
	}
}

// Two entries with one name: the index says which one.
func TestFindMatchIndexIn_TellsSameNamesApart(t *testing.T) {
	names := []string{"Iron Sword", "Trinket", "Trinket"}
	for search, want := range map[string][2]int{
		"trinket":   {1, 1},
		"2.trinket": {2, 2},
		"trinket#2": {2, 2},
		"3.trinket": {-1, -1},
		"trink":     {-1, 1},
		"2.trink":   {-1, 2},
	} {
		m, c := FindMatchIndexIn(search, names...)
		if m != want[0] || c != want[1] {
			t.Errorf("FindMatchIndexIn(%q) = (%d, %d), want (%d, %d)", search, m, c, want[0], want[1])
		}
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/util/ -run 'TestFindMatchIndexIn' -count=1`
Expected: FAIL, build error `undefined: FindMatchIndexIn`.

- [ ] **Step 3: Implement**

With the Edit tool, in `internal/util/util.go` replace the whole function (old text, lines 353-405):

```go
func FindMatchIn(searchName string, items ...string) (match string, closeMatch string) {

	if searchName == `` {
		return ``, `` // No match
	}

	searchName, searchNumber := GetMatchNumber(searchName)

	var matchCt int = 0
	var closeMatchCt int = 0

	for _, i := range items {

		part, full := stringMatch(searchName, i, false)

		if part {
			closeMatchCt++
			if closeMatchCt == searchNumber {
				closeMatch = i
			}
		}

		if full {
			matchCt++
			if matchCt == searchNumber {
				match = i
				break
			}
		}

	}

	// If no "starts with" or "exact" matches are found, try and find the first item that contain the supplied name
	// Note: Can't have an exact match if there was never a close match
	if len(closeMatch) == 0 {
		closeMatchCt = 0
		for _, i := range items {
			part, _ := stringMatch(searchName, i, true)

			if part {
				closeMatchCt++
				if closeMatchCt == searchNumber {
					closeMatch = i
					break
				}
			}

		}

	}

	return match, closeMatch
}
```

with:

```go
// FindMatchIn returns the full match and the close match for searchName
// among items, by name: FindMatchIndexIn's answer, named. Either is "" when
// there is none.
func FindMatchIn(searchName string, items ...string) (match string, closeMatch string) {
	matchIdx, closeIdx := FindMatchIndexIn(searchName, items...)
	if matchIdx >= 0 {
		match = items[matchIdx]
	}
	if closeIdx >= 0 {
		closeMatch = items[closeIdx]
	}
	return match, closeMatch
}

// FindMatchIndexIn is the one matching algorithm, by position: the index in
// items of the full match and of the close match, -1 for none. Two entries
// with the same name are told apart this way (`buy 2.trinket` takes the
// second of two trinkets, baubles slice D). FindMatchIn wraps it.
func FindMatchIndexIn(searchName string, items ...string) (match int, closeMatch int) {
	match, closeMatch = -1, -1
	if searchName == `` {
		return match, closeMatch // No match
	}

	searchName, searchNumber := GetMatchNumber(searchName)

	matchCt, closeMatchCt := 0, 0

	for idx, i := range items {

		part, full := stringMatch(searchName, i, false)

		if part {
			closeMatchCt++
			if closeMatchCt == searchNumber {
				closeMatch = idx
			}
		}

		if full {
			matchCt++
			if matchCt == searchNumber {
				match = idx
				break
			}
		}

	}

	// If no "starts with" or "exact" matches are found, try and find the first item that contain the supplied name
	// Note: Can't have an exact match if there was never a close match
	if closeMatch < 0 {
		closeMatchCt = 0
		for idx, i := range items {
			part, _ := stringMatch(searchName, i, true)

			if part {
				closeMatchCt++
				if closeMatchCt == searchNumber {
					closeMatch = idx
					break
				}
			}

		}

	}

	return match, closeMatch
}
```

(`closeMatch < 0` equals the old `len(closeMatch) == 0` except in one corner: a search that is empty after `GetMatchNumber` and normalizing (`#2`, or `'`) prefix-matches every item, an empty-string item included, and if that empty item is the Nth close match the old code saw an empty `closeMatch` and ran the contains pass again, where the new code keeps the index. Only malformed data offers an empty item name (buy skips `ItemId 0` rows, and a spec with no name is a content error), so in practice no caller sees a difference; `TestFindMatchIn` and the apostrophe tests pass unchanged.)

- [ ] **Step 4: Run to see it pass, with every existing matcher test**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/util/ -count=1 && go build ./... && go test . -count=1 2>&1 | tail -2`
Expected: `ok` for `internal/util` (including `TestFindMatchIn`, `TestFindMatchIn_ApostropheInsensitive`, `TestFindMatchIn_PrefixStillWorks`), build clean, root `ok`.

- [ ] **Step 5: Null probes**

1. In `FindMatchIndexIn` change `match = idx` to `match = 0`. Run `go test ./internal/util/ -run TestFindMatchIndexIn -count=1`. Expected: FAIL naming `FindMatchIndexIn("2.trinket") = (0, 2), want (2, 2)`. Restore.
2. In `FindMatchIn` change `closeMatch = items[closeIdx]` to `closeMatch = items[0]`. Run `go test ./internal/util/ -run 'TestFindMatchIn$' -count=1`. Expected: FAIL naming `FindMatchIn("HELM")`. Restore.

Re-run Step 4: green.

- [ ] **Step 6: Commit**

```bash
cd /c/tmp/dogmud-baubles-d-impl && git add internal/util/util.go internal/util/findmatchindex_test.go && git commit -F - <<'EOF'
feat(util): FindMatchIndexIn, FindMatchIn by position

The one matching algorithm now returns indexes; FindMatchIn wraps it.
A shop's two rows with one name can be told apart (buy 2.trinket),
which name matching alone cannot do.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: `ShopAffixedStockCap` into SHOP ECONOMY at 12

**Files:**
- Modify: `internal/configs/config.balance.go:885, 950`
- Modify: `internal/configs/config.balance.misc.go:312-314`
- Modify: `internal/configs/config.balance.shops.go:44-46`
- Modify: `internal/configs/config.balance.shops_test.go:3`
- Modify: `_datafiles/config.yaml:1435`

- [ ] **Step 1: Write the failing tests**

In `internal/configs/config.balance.shops_test.go` replace `import "testing"` with:

```go
import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"
)
```

and append:

```go
// The shelf cap (baubles slice D) is a SHOP ECONOMY knob: default 12, an
// explicit value kept.
func TestValidateShops_ShopAffixedStockCapDefault(t *testing.T) {
	b := &Balance{}
	b.validateShops()
	if int(b.ShopAffixedStockCap) != 12 {
		t.Errorf("ShopAffixedStockCap default = %d, want 12", int(b.ShopAffixedStockCap))
	}
	b = &Balance{ShopAffixedStockCap: 5}
	b.validateShops()
	if int(b.ShopAffixedStockCap) != 5 {
		t.Errorf("ShopAffixedStockCap = %d, want 5 (explicit value preserved)", int(b.ShopAffixedStockCap))
	}
}

// The shipped config names the knob, at the Go default, so the live value is
// visible in config.yaml rather than silently the default.
func TestShopAffixedStockCapShipsAtTwelve(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "_datafiles", "config.yaml"))
	if err != nil {
		t.Fatalf("read shipped config: %v", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("decode shipped config: %v", err)
	}
	if int(cfg.Balance.ShopAffixedStockCap) != 12 {
		t.Errorf("shipped ShopAffixedStockCap = %d, want 12 (0 means the key is missing)", int(cfg.Balance.ShopAffixedStockCap))
	}
}
```

- [ ] **Step 2: Run to see them fail**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/configs/ -run 'ShopAffixedStockCap' -count=1`
Expected: FAIL with `ShopAffixedStockCap default = 0, want 12` and `shipped ShopAffixedStockCap = 0, want 12 (0 means the key is missing)`.

- [ ] **Step 3: Move the field, the default, and add the key**

`internal/configs/config.balance.go`: delete the line

```go
	ShopAffixedStockCap ConfigInt   `yaml:"ShopAffixedStockCap"` // Max per-instance affixed items a shop resells before evicting oldest (default 8)
```

and after the line

```go
	BarterMaxBonus              ConfigFloat `yaml:"BarterMaxBonus,omitempty"`           // Max fractional sell-price bonus a player can get via bartering (default 0.15)
```

insert

```go
	ShopAffixedStockCap         ConfigInt   `yaml:"ShopAffixedStockCap"`                // Max listed entries on a shop's secondhand shelf (AffixedStock: bought-back affixed gear, average and rare baubles); over it the one listed earliest goes. Also the most hot baubles a shop holds out of sight (default 12)
```

After all four Go edits below, run `gofmt -w internal/configs/config.balance.go internal/configs/config.balance.misc.go internal/configs/config.balance.shops.go internal/configs/config.balance.shops_test.go` (it realigns the `LOOT` block now that its longest name left, and tidies the blank line the misc deletion leaves).

`internal/configs/config.balance.misc.go`: delete

```go
	if b.ShopAffixedStockCap <= 0 {
		b.ShopAffixedStockCap = 8
	}
```

`internal/configs/config.balance.shops.go`: after

```go
	if b.BarterMaxBonus <= 0 {
		b.BarterMaxBonus = 0.15
	}
```

insert

```go

	// ── SECONDHAND SHELF (baubles slice D) ───────────────────────────────────
	if b.ShopAffixedStockCap <= 0 {
		b.ShopAffixedStockCap = 12
	}
```

`_datafiles/config.yaml` (Edit tool, in this worktree directly: no skip-worktree bit here, P11): after

```yaml
  BarterMaxBonus: 0.15          # Max fractional sell-price bonus from bartering
```

insert

```yaml
  # The secondhand shelf (AffixedStock): gear and average or rare baubles a
  # shop bought from players and resells. At most this many are listed; over
  # it, the one listed earliest goes. A bauble still hot when shelved waits
  # out of sight until it cools, and a shop holds at most this many of those
  # too, refusing more hot goods until some cool.
  ShopAffixedStockCap: 12
```

- [ ] **Step 4: Run to see them pass; check the diff**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/configs/ -count=1 && go build ./... && go test . -count=1 2>&1 | tail -2 && git diff --stat && git diff _datafiles/config.yaml`
Expected: `ok` twice; the stat lists exactly the five files; the `config.yaml` diff is the six added lines and nothing else (a line ending or a stray local value in that diff means STOP: rebuild the file from `git show HEAD:_datafiles/config.yaml` and re-add the six lines).

- [ ] **Step 5: Null probes**

1. Set the new default to `8`. Run `go test ./internal/configs/ -run ShopAffixedStockCapDefault -count=1`. Expected: FAIL `default = 8, want 12`. Restore.
2. Change the yaml line to `ShopAffixedStockCap: 8`. Run `go test ./internal/configs/ -run ShipsAtTwelve -count=1`. Expected: FAIL `shipped ShopAffixedStockCap = 8, want 12`. Restore; re-run Step 4.

- [ ] **Step 6: Commit**

```bash
cd /c/tmp/dogmud-baubles-d-impl && git add internal/configs/config.balance.go internal/configs/config.balance.misc.go internal/configs/config.balance.shops.go internal/configs/config.balance.shops_test.go _datafiles/config.yaml && git commit -F - <<'EOF'
feat(config): ShopAffixedStockCap in SHOP ECONOMY, 12, shipped

The shelf cap moves from the LOOT block to SHOP ECONOMY in Go and
gains a config.yaml key; default and shipped value rise from 8 to 12
(baubles slice D spec, section 7).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: Shelf entry fields and helpers

**Files:**
- Modify: `internal/shops/shopinventory.go:3-9, 72-76`
- Create: `internal/shops/shelf.go`
- Create: `internal/shops/shelf_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/shops/shelf_test.go`:

```go
package shops

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/items"
	"gopkg.in/yaml.v2"
)

// The secondhand shelf (baubles slice D): held entries, the listed cap and
// its eviction order, the rollback reinsert, and the saved times.

var shelfT0 = time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

// shelfEntry is a piece of affixed gear shelved at addedAt and held until
// holdUntil (zero: listed at once). Its ItemId names it in assertions.
func shelfEntry(id int, addedAt, holdUntil time.Time) AffixedStockEntry {
	return AffixedStockEntry{
		Item:      items.Item{ItemId: id, Affixed: true, Spec: &items.ItemSpec{Value: 100, Name: "piece"}},
		Price:     100,
		AddedAt:   addedAt,
		HoldUntil: holdUntil,
	}
}

func shelfIds(si *ShopInventory) []int {
	out := []int{}
	for _, e := range si.AffixedStock {
		out = append(out, e.Item.ItemId)
	}
	return out
}

// Spec tests 1 to 3: a held entry is not listed, counts toward HeldCount
// only, and is never evicted; the listed cap counts listed entries only. A
// hold that ends lists the entry.
func TestShelf_HeldEntriesAreNotListedNorEvicted(t *testing.T) {
	now := shelfT0.Add(time.Hour)
	si := &ShopInventory{AffixedStock: []AffixedStockEntry{
		shelfEntry(1, shelfT0, time.Time{}),
		shelfEntry(2, shelfT0, shelfT0.Add(72*time.Hour)),
		shelfEntry(3, shelfT0.Add(time.Minute), time.Time{}),
		shelfEntry(4, shelfT0, shelfT0.Add(24*time.Hour)),
	}}

	if got := si.ListedIndexes(now); !reflect.DeepEqual(got, []int{0, 2}) {
		t.Fatalf("listed: got %v, want [0 2] (the held ones are out of sight)", got)
	}
	if got := si.HeldCount(now); got != 2 {
		t.Fatalf("held: got %d, want 2", got)
	}
	if removed := si.EnforceAffixedCap(0, now); removed != 0 || len(si.AffixedStock) != 4 {
		t.Fatalf("a cap of 0 removes nothing: removed %d, left %v", removed, shelfIds(si))
	}
	if removed := si.EnforceAffixedCap(1, now); removed != 1 {
		t.Fatalf("cap 1 over two listed entries: removed %d, want 1", removed)
	}
	if got := shelfIds(si); !reflect.DeepEqual(got, []int{2, 3, 4}) {
		t.Fatalf("the earlier listed entry goes and both held ones stay: got %v, want [2 3 4]", got)
	}
	if removed := si.EnforceAffixedCap(1, now); removed != 0 {
		t.Fatalf("at the cap nothing more goes: removed %d", removed)
	}

	// Spec test 2: item 4's hold ends; it lists, after item 3, in slice order.
	later := shelfT0.Add(24 * time.Hour)
	if got := si.ListedIndexes(later); !reflect.DeepEqual(got, []int{1, 2}) {
		t.Fatalf("hold over: got %v, want [1 2]", got)
	}
	if si.AffixedStock[2].Held(later) || !si.AffixedStock[2].Held(later.Add(-time.Second)) {
		t.Fatal("held strictly before HoldUntil, listed from it on")
	}
}

// Spec test 4: over the cap, the entry with the earliest ListedAt goes. An
// entry shelved first but held until later outlives an unheld entry shelved
// after it. Entries saved before AddedAt existed (both times zero) list
// earliest, and a tie goes to the lower index.
func TestShelf_EvictsTheEntryListedEarliest(t *testing.T) {
	si := &ShopInventory{AffixedStock: []AffixedStockEntry{
		shelfEntry(1, shelfT0, shelfT0.Add(2*time.Hour)), // shelved first, listed at T0+2h
		shelfEntry(2, shelfT0.Add(time.Minute), time.Time{}),
		shelfEntry(3, shelfT0.Add(2*time.Minute), time.Time{}),
	}}
	if removed := si.EnforceAffixedCap(2, shelfT0.Add(3*time.Hour)); removed != 1 {
		t.Fatalf("removed %d, want 1", removed)
	}
	if got := shelfIds(si); !reflect.DeepEqual(got, []int{1, 3}) {
		t.Fatalf("item 2 listed earliest and goes, item 1 was held and stays: got %v, want [1 3]", got)
	}

	old := &ShopInventory{AffixedStock: []AffixedStockEntry{
		shelfEntry(7, time.Time{}, time.Time{}),
		shelfEntry(8, time.Time{}, time.Time{}),
		shelfEntry(9, shelfT0, time.Time{}),
	}}
	old.EnforceAffixedCap(2, shelfT0)
	if got := shelfIds(old); !reflect.DeepEqual(got, []int{8, 9}) {
		t.Fatalf("pre-change entries list earliest, ties to the lower index: got %v, want [8 9]", got)
	}
}

// Spec test 14: RestoreAffixedStock puts an entry back at its index with its
// fields unchanged; an index out of range is clamped to the ends.
func TestShelf_RestoreAffixedStockPutsTheEntryBackUnchanged(t *testing.T) {
	a := shelfEntry(1, shelfT0, time.Time{})
	b := shelfEntry(2, shelfT0.Add(time.Minute), shelfT0.Add(time.Hour))
	b.Price, b.AddedRound = 37, 99
	c := shelfEntry(3, shelfT0, time.Time{})
	si := &ShopInventory{AffixedStock: []AffixedStockEntry{a, b, c}}

	if _, ok := si.RemoveAffixedStock(1); !ok {
		t.Fatal("remove")
	}
	si.RestoreAffixedStock(1, b)
	if got := shelfIds(si); !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatalf("back in its place: got %v", got)
	}
	got := si.AffixedStock[1]
	if got.Price != 37 || got.AddedRound != 99 || !got.AddedAt.Equal(b.AddedAt) || !got.HoldUntil.Equal(b.HoldUntil) {
		t.Fatalf("fields changed: %+v", got)
	}

	si.RestoreAffixedStock(99, shelfEntry(4, shelfT0, time.Time{}))
	si.RestoreAffixedStock(-1, shelfEntry(5, shelfT0, time.Time{}))
	if got := shelfIds(si); !reflect.DeepEqual(got, []int{5, 1, 2, 3, 4}) {
		t.Fatalf("out-of-range indexes clamp to the ends: got %v", got)
	}
}

// AddedAt and HoldUntil survive the shop save (yaml.v2, as SaveShop uses),
// zero times are omitted, and a shop file written before they existed loads
// with both zero: listed.
func TestShelf_TimesRoundTripAndOldFilesLoadListed(t *testing.T) {
	in := ShopInventory{AffixedStock: []AffixedStockEntry{shelfEntry(5, shelfT0, shelfT0.Add(72*time.Hour))}}
	data, err := yaml.Marshal(&in)
	if err != nil {
		t.Fatal(err)
	}
	var out ShopInventory
	if err := yaml.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	e := out.AffixedStock[0]
	if !e.AddedAt.Equal(shelfT0) || !e.HoldUntil.Equal(shelfT0.Add(72*time.Hour)) {
		t.Fatalf("times lost in the round trip: %+v\n%s", e, data)
	}

	listed := ShopInventory{AffixedStock: []AffixedStockEntry{shelfEntry(6, time.Time{}, time.Time{})}}
	raw, err := yaml.Marshal(&listed)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "hold_until") || strings.Contains(string(raw), "added_at") {
		t.Fatalf("zero times must be omitted:\n%s", raw)
	}

	old := "gold: 10\naffixed_stock:\n- item:\n    itemid: 5\n  price: 7\n  added_round: 3\n"
	var loaded ShopInventory
	if err := yaml.Unmarshal([]byte(old), &loaded); err != nil {
		t.Fatal(err)
	}
	le := loaded.AffixedStock[0]
	if !le.AddedAt.IsZero() || !le.HoldUntil.IsZero() || le.Held(shelfT0) || len(loaded.ListedIndexes(shelfT0)) != 1 {
		t.Fatalf("an old entry loads listed with zero times: %+v", le)
	}
}
```

- [ ] **Step 2: Run to see them fail**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/shops/ -run TestShelf_ -count=1`
Expected: FAIL, build errors `unknown field AddedAt in struct literal of type AffixedStockEntry` and `si.ListedIndexes undefined`.

- [ ] **Step 3: Implement**

`internal/shops/shopinventory.go`: replace

```go
	"slices"

	"github.com/GoMudEngine/GoMud/internal/economy"
```

with

```go
	"slices"
	"time"

	"github.com/GoMudEngine/GoMud/internal/economy"
```

Replace

```go
	AddedRound uint64     `yaml:"added_round,omitempty"` // for age-based clutter eviction
}
```

with

```go
	AddedRound uint64     `yaml:"added_round,omitempty"` // game round when shelved (kept for the record; eviction reads ListedAt)
	AddedAt    time.Time  `yaml:"added_at,omitempty"`    // wall clock when shelved: heat is real time, rounds stop while the server is down
	HoldUntil  time.Time  `yaml:"hold_until,omitempty"`  // held out of sight until then (a bauble hot when shelved); zero: listed at once
}
```

Create `internal/shops/shelf.go`:

```go
package shops

import (
	"slices"
	"time"
)

// The secondhand shelf (baubles slice D). AffixedStock holds unique items a
// shop bought from players and resells: affix-scaled gear, and average or
// rare baubles. An entry is LISTED (list shows it, buy sells it) unless it
// is HELD: a bauble still hot when shelved waits out of sight until
// HoldUntil. Listed entries are capped (Balance.ShopAffixedStockCap) and
// the cap is enforced lazily, on an add, on list and on buy. Held entries
// never count against that cap and are never evicted; a shop refuses a new
// hot bauble once it holds that many (internal/actions baubleOfferFor).
//
// Every mutation happens in a command or a sale, under the mud lock, like
// the rest of ShopInventory (it has no lock of its own).

// ShelfNow is the one clock the shelf reads: list, buy, the sale that
// shelves an item and the held-count refusal all call it, so a test that
// sets it sees one consistent held state. A variable for tests.
var ShelfNow = time.Now

// Held reports whether the entry is still out of sight at now.
func (e AffixedStockEntry) Held(now time.Time) bool {
	return now.Before(e.HoldUntil)
}

// ListedAt is when the entry went on show: the end of its hold, or when it
// was shelved. An entry saved before these fields existed has both zero and
// so counts as listed earliest.
func (e AffixedStockEntry) ListedAt() time.Time {
	if !e.HoldUntil.IsZero() {
		return e.HoldUntil
	}
	return e.AddedAt
}

// HeldCount is how many entries are held at now.
func (si *ShopInventory) HeldCount(now time.Time) int {
	n := 0
	for _, e := range si.AffixedStock {
		if e.Held(now) {
			n++
		}
	}
	return n
}

// ListedIndexes returns the AffixedStock indexes of the entries listed at
// now, in slice order. This is THE shelf order: list renders it unsorted and
// buy counts `buy 2.name` in it.
func (si *ShopInventory) ListedIndexes(now time.Time) []int {
	out := make([]int, 0, len(si.AffixedStock))
	for i, e := range si.AffixedStock {
		if !e.Held(now) {
			out = append(out, i)
		}
	}
	return out
}

// EnforceAffixedCap removes listed entries while more than limit are listed
// at now, the one listed earliest (ListedAt) first, ties to the lower index,
// and returns how many it removed. limit <= 0 removes nothing. Held entries
// are never removed. A removed item is gone; a bauble's record keeps the
// sale that shelved it. A caller that changes a living shop saves it.
func (si *ShopInventory) EnforceAffixedCap(limit int, now time.Time) int {
	if limit <= 0 {
		return 0
	}
	removed := 0
	for {
		listed := si.ListedIndexes(now)
		if len(listed) <= limit {
			return removed
		}
		oldest := listed[0]
		for _, idx := range listed[1:] {
			if si.AffixedStock[idx].ListedAt().Before(si.AffixedStock[oldest].ListedAt()) {
				oldest = idx
			}
		}
		si.AffixedStock = slices.Delete(si.AffixedStock, oldest, oldest+1)
		removed++
	}
}

// RestoreAffixedStock puts an entry back at idx (clamped to the list) with
// its price, round and times unchanged: buy's rollback when the buyer cannot
// take the item after all.
func (si *ShopInventory) RestoreAffixedStock(idx int, e AffixedStockEntry) {
	idx = max(0, min(idx, len(si.AffixedStock)))
	si.AffixedStock = slices.Insert(si.AffixedStock, idx, e)
}
```

Run `gofmt -w internal/shops/shopinventory.go internal/shops/shelf.go internal/shops/shelf_test.go`.

- [ ] **Step 4: Run to see them pass**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/shops/ -count=1 && go build ./... && go test . -count=1 2>&1 | tail -2`
Expected: `ok` (all existing shop tests too, including `TestAffixedStock_*`), build clean, root `ok` (`TestItemWalkersVisitEveryItemField` included: the new fields hold no items).

- [ ] **Step 5: Null probes**

1. Make `Held` return `false`. Run `go test ./internal/shops/ -run TestShelf_HeldEntriesAreNotListedNorEvicted -count=1`. Expected: FAIL `listed: got [0 1 2 3], want [0 2]`. Restore.
2. In `EnforceAffixedCap` delete the inner `for _, idx := range listed[1:]` loop (always evict `listed[0]`). Run `go test ./internal/shops/ -run TestShelf_EvictsTheEntryListedEarliest -count=1`. Expected: FAIL `got [2 3], want [1 3]`. Restore.
3. In `RestoreAffixedStock` drop the clamp line. Run the restore test. Expected: a panic `slice bounds out of range` in `TestShelf_RestoreAffixedStockPutsTheEntryBackUnchanged`. Restore; re-run Step 4.

- [ ] **Step 6: Commit**

```bash
cd /c/tmp/dogmud-baubles-d-impl && git add internal/shops/shopinventory.go internal/shops/shelf.go internal/shops/shelf_test.go && git commit -F - <<'EOF'
feat(shops): shelf entry times and listed/held helpers

AffixedStockEntry gains AddedAt and HoldUntil (omitempty: old shop
files load listed). shelf.go adds the ShelfNow clock, Held, ListedAt,
HeldCount, ListedIndexes, EnforceAffixedCap (earliest-listed first,
held never evicted) and RestoreAffixedStock.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 4: `baubles.ShelfHoldUntil`

**Files:**
- Modify: `internal/baubles/theft.go` (before `// MarkRecognized records ...`, `:153`)
- Modify: `internal/baubles/theft_test.go` (append)

- [ ] **Step 1: Write the failing test**

Append to `internal/baubles/theft_test.go`:

```go
// A bauble shelved while hot anywhere (Hot, not HotIn: its buyer could carry
// it back into the theft's area) is held until its heat ends, StolenAt plus
// the heat; anything else is listed at once (baubles slice D).
func TestShelfHoldUntilIsTheEndOfTheHeat(t *testing.T) {
	SetDirForTest(t.TempDir())
	t.Cleanup(func() { items.SetBaubleResolver(nil) })
	setBaubleConfig(t, func(b *configs.Balance) { b.BaubleStolenHeatHours = 72 })

	rec, err := Create(Record{Name: "Bone Dice", NameSimple: "dice", Tier: TierAverage, Value: 12, Status: StatusReady})
	if err != nil {
		t.Fatal(err)
	}
	it := items.Item{ItemId: items.BaubleItemId, Bauble: rec.Id}
	t0 := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)

	if !ShelfHoldUntil(it, t0).IsZero() {
		t.Fatal("an honest bauble is listed at once")
	}
	MarkStolen(rec.Id, Theft{ByUserId: 1, FromMob: 2, Zone: "Thornwall City"}, t0)
	if got := ShelfHoldUntil(it, t0.Add(time.Hour)); !got.Equal(t0.Add(72 * time.Hour)) {
		t.Fatalf("hot: held until %v, want %v", got, t0.Add(72*time.Hour))
	}
	if !ShelfHoldUntil(it, t0.Add(72*time.Hour)).IsZero() {
		t.Fatal("cold once the heat is out")
	}
	MarkReturned(rec.Id, 1, nil, t0.Add(2*time.Hour))
	if !ShelfHoldUntil(it, t0.Add(3*time.Hour)).IsZero() {
		t.Fatal("given back, it is not hot, so not held")
	}
	if !ShelfHoldUntil(items.Item{ItemId: 5}, t0).IsZero() {
		t.Fatal("anything that is not a bauble is listed at once")
	}
}
```

- [ ] **Step 2: Run to see it fail**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/baubles/ -run TestShelfHoldUntilIsTheEndOfTheHeat -count=1`
Expected: FAIL, build error `undefined: ShelfHoldUntil`.

- [ ] **Step 3: Implement**

In `internal/baubles/theft.go`, before the line `// MarkRecognized records that the bauble's owner recognised it on someone.` insert:

```go
// ShelfHoldUntil is when a bauble a shop shelves at now may first be shown
// for sale (baubles slice D): the end of its heat, StolenAt plus
// HeatDuration, while it is hot anywhere. Hot, not HotIn: the shop may be
// outside the theft's heat area, but its buyer could carry it back there,
// where its owner would know it. Zero (listed at once) for a bauble that is
// not hot, one with no record, and anything that is not a bauble. The hold
// is fixed when shelved; a later change to the heat does not move it.
func ShelfHoldUntil(itm items.Item, now time.Time) time.Time {
	if !itm.IsBauble() {
		return time.Time{}
	}
	rec, ok := Get(itm.Bauble)
	if !ok || !rec.Hot(now) {
		return time.Time{}
	}
	return rec.StolenAt.Add(HeatDuration())
}

```

- [ ] **Step 4: Run to see it pass**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/baubles/ -count=1 && go test . -count=1 2>&1 | tail -2`
Expected: `ok`, root `ok`.

- [ ] **Step 5: Null probe**

Delete `|| !rec.Hot(now)` from the guard. Run the test. Expected: FAIL `an honest bauble is listed at once` (its zero `StolenAt` plus the heat is not zero). Restore; re-run Step 4.

- [ ] **Step 6: Commit**

```bash
cd /c/tmp/dogmud-baubles-d-impl && git add internal/baubles/theft.go internal/baubles/theft_test.go && git commit -F - <<'EOF'
feat(baubles): ShelfHoldUntil, the end of a shelved bauble's heat

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 5: `AddAffixedStock` signature and its callers

**Files:**
- Modify: `internal/shops/shopinventory.go:139-152`
- Modify: `internal/shops/shopinventory_test.go:459, 460, 477, 495`; `internal/shops/shelf_test.go` (append)
- Modify: `internal/actions/sell.go:7, 391-393`
- Modify: `modules/auctions/npc_buyers.go:4, 296-297`
- Modify: `internal/actions/buy.go:633-639`
- Modify: `internal/actions/buy_test.go:4, 29-30`

The signature change is deliberate (spec section 1): the compiler lists every caller.

- [ ] **Step 1: Write the failing test**

Append to `internal/shops/shelf_test.go`:

```go
// Spec test 5, on an add: AddAffixedStock stamps the wall clock and the
// hold, then trims the LISTED entries to the cap, earliest listed first; a
// held add is never counted against the listed cap.
func TestShelf_AddStampsTimesAndTrimsTheListedEntries(t *testing.T) {
	si := &ShopInventory{AffixedStock: []AffixedStockEntry{
		shelfEntry(1, shelfT0, shelfT0.Add(time.Hour)),
		shelfEntry(2, shelfT0.Add(time.Minute), time.Time{}),
	}}
	now := shelfT0.Add(2 * time.Hour)

	si.AddAffixedStock(shelfEntry(3, time.Time{}, time.Time{}).Item, 55, 2, time.Time{}, now)
	if got := shelfIds(si); !reflect.DeepEqual(got, []int{1, 3}) {
		t.Fatalf("item 1's hold ended at T0+1h, after item 2 was shelved, so item 2 goes: got %v, want [1 3]", got)
	}
	last := si.AffixedStock[1]
	if last.Price != 55 || !last.AddedAt.Equal(now) || !last.HoldUntil.IsZero() {
		t.Fatalf("added entry: %+v", last)
	}

	si.AddAffixedStock(shelfEntry(4, time.Time{}, time.Time{}).Item, 60, 2, now.Add(72*time.Hour), now)
	if got := shelfIds(si); !reflect.DeepEqual(got, []int{1, 3, 4}) {
		t.Fatalf("a held add evicts nothing: got %v, want [1 3 4]", got)
	}
	if !si.AffixedStock[2].Held(now) {
		t.Fatal("the new entry is held")
	}
}
```

- [ ] **Step 2: Run to see it fail**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/shops/ -run TestShelf_Add -count=1`
Expected: FAIL, build error `too many arguments in call to si.AddAffixedStock`.

- [ ] **Step 3: Change the signature**

`internal/shops/shopinventory.go`: replace

```go
// AddAffixedStock appends a bought-back affixed item at relist price, evicting
// the oldest entry (FIFO) when the list is at cap. cap <= 0 means no cap.
func (si *ShopInventory) AddAffixedStock(item items.Item, price, cap int) {
	si.AffixedStock = append(si.AffixedStock, AffixedStockEntry{
		Item:       item,
		Price:      price,
		AddedRound: util.GetRoundCount(),
	})
	if cap > 0 {
		for len(si.AffixedStock) > cap {
			si.AffixedStock = si.AffixedStock[1:] // drop oldest
		}
	}
}
```

with

```go
// AddAffixedStock shelves an item at its relist price: appended with the
// round, the wall clock (AddedAt: now) and the end of any hold (holdUntil,
// zero to list it at once), then EnforceAffixedCap(limit, now) trims the
// listed entries (limit <= 0: no cap). It does not enforce the held cap:
// a sale refuses a hot bauble a full backroom cannot take before it gets
// here (internal/actions baubleOfferFor).
func (si *ShopInventory) AddAffixedStock(item items.Item, price, limit int, holdUntil, now time.Time) {
	si.AffixedStock = append(si.AffixedStock, AffixedStockEntry{
		Item:       item,
		Price:      price,
		AddedRound: util.GetRoundCount(),
		AddedAt:    now,
		HoldUntil:  holdUntil,
	})
	si.EnforceAffixedCap(limit, now)
}
```

- [ ] **Step 4: Let the compiler list the callers, then fix each**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go build ./... 2>&1; go vet ./internal/shops/ ./internal/actions/ 2>&1 | grep -E "not enough arguments|too many" `
Expected: `go build ./...` stops at the first failing package, `internal/actions` (`buy.go:639`, `sell.go:393`); once those are fixed a rerun reports `modules/auctions/npc_buyers.go:297`. Vet reports the test callers at the line of each call's closing argument: `shopinventory_test.go:459`, `:460`, `:478`, `:496` and `internal/actions/buy_test.go:30`. Re-run the build and vet after each fix until both are clean. Any site not listed here: STOP and add it to this task.

`internal/shops/shopinventory_test.go`:
- `si.AddAffixedStock(a, 200, 100)` becomes `si.AddAffixedStock(a, 200, 100, time.Time{}, shelfT0)`
- `si.AddAffixedStock(b, 150, 100)` becomes `si.AddAffixedStock(b, 150, 100, time.Time{}, shelfT0)`
- `Spec: &items.ItemSpec{Value: 100}}, 50, 3)` becomes `Spec: &items.ItemSpec{Value: 100}}, 50, 3, time.Time{}, shelfT0)` (same `now` each time: ties go to the lower index, so the FIFO expectation `[102 103 104]` holds)
- `Spec: &items.ItemSpec{Value: 400, PhysicalMitigation: 5}}, 200, 8)` becomes `Spec: &items.ItemSpec{Value: 400, PhysicalMitigation: 5}}, 200, 8, time.Time{}, shelfT0)`
- and in its import block replace `	"testing"` with `	"testing"` + newline + `	"time"`:

```go
import (
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/items"
```

`internal/actions/sell.go`: replace

```go
			c := int(configs.GetBalanceConfig().ShopAffixedStockCap)
			shopInv.AddAffixedStock(item, item.GetSpec().Value, c)
```

with

```go
			c := int(configs.GetBalanceConfig().ShopAffixedStockCap)
			now := shops.ShelfNow()
			shopInv.AddAffixedStock(item, item.GetSpec().Value, c, baubles.ShelfHoldUntil(item, now), now)
```

and its import `	"github.com/GoMudEngine/GoMud/internal/characters"` with

```go
	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/characters"
```

(`ShelfHoldUntil` is always zero for affixed loot; the call keeps one shape at every shelving site, spec section 10.)

`modules/auctions/npc_buyers.go`: replace

```go
	c := int(configs.GetBalanceConfig().ShopAffixedStockCap)
	s.bound.AddAffixedStock(item, item.GetSpec().Value, c)
```

with

```go
	c := int(configs.GetBalanceConfig().ShopAffixedStockCap)
	now := shops.ShelfNow()
	s.bound.AddAffixedStock(item, item.GetSpec().Value, c, baubles.ShelfHoldUntil(item, now), now)
```

and `	"github.com/GoMudEngine/GoMud/internal/configs"` (the first import) with

```go
	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/configs"
```

`internal/actions/buy.go`: replace

```go
		bought, ok := shopInv.RemoveAffixedStock(matched.affixedIdx)
		if !ok {
			return BuyResult{Reason: BuyReasonOutOfStock}
		}
		bought.UUID = items.NewItemUUID()
		if !char.StoreItem(bought) {
			shopInv.AddAffixedStock(bought, matched.price, 0) // roll back on carry failure
```

with

```go
		if matched.affixedIdx >= len(shopInv.AffixedStock) {
			return BuyResult{Reason: BuyReasonOutOfStock}
		}
		shelved := shopInv.AffixedStock[matched.affixedIdx] // the whole entry, for a rollback
		bought, ok := shopInv.RemoveAffixedStock(matched.affixedIdx)
		if !ok {
			return BuyResult{Reason: BuyReasonOutOfStock}
		}
		bought.UUID = items.NewItemUUID()
		if !char.StoreItem(bought) {
			// Roll back on carry failure: the same entry in the same place,
			// its relist price and listing time unchanged (a re-add would
			// stamp a new time at the barter-discounted price). Defensive:
			// the encumbrance gate above refuses first.
			shopInv.RestoreAffixedStock(matched.affixedIdx, shelved)
```

`internal/actions/buy_test.go`: replace `	"testing"` (first import line) with

```go
	"testing"
	"time"
```

and `Spec: &items.ItemSpec{Value: 400, Name: "iron sword", NameSimple: "sword", Type: items.Weapon}}, 400, 8)` with `Spec: &items.ItemSpec{Value: 400, Name: "iron sword", NameSimple: "sword", Type: items.Weapon}}, 400, 8, time.Time{}, time.Now())`.

Run `gofmt -w` on every file this task touched.

- [ ] **Step 5: Run to see everything pass**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go build ./... && go vet ./internal/shops/ ./internal/actions/ ./modules/auctions/ && go test ./internal/shops/ ./internal/actions/ ./modules/auctions/ . -count=1 2>&1 | tail -5`
Expected: no vet output; four `ok` lines (`TestBuy_AffixedStockItem`, `TestAffixedStock_*`, `TestShelf_Add...` included).

- [ ] **Step 6: Null probe**

Delete `si.EnforceAffixedCap(limit, now)` from `AddAffixedStock`. Run `go test ./internal/shops/ -run 'TestShelf_Add|TestAffixedStock_CapEvictsOldest' -count=1`. Expected: FAIL in both (`got [1 2 3], want [1 3]`; `cap 3: want 3 entries, got 5`). Restore; re-run Step 5.

- [ ] **Step 7: Commit**

```bash
cd /c/tmp/dogmud-baubles-d-impl && git add internal/shops/shopinventory.go internal/shops/shopinventory_test.go internal/shops/shelf_test.go internal/actions/sell.go internal/actions/buy.go internal/actions/buy_test.go modules/auctions/npc_buyers.go && git commit -F - <<'EOF'
feat(shops): AddAffixedStock takes a hold and a clock; buy restores in place

AddAffixedStock(item, price, limit, holdUntil, now) stamps AddedAt and
HoldUntil and trims listed entries through EnforceAffixedCap. The sell
and auction callers pass baubles.ShelfHoldUntil (zero for gear). buy's
carry-failure rollback puts the whole entry back with
RestoreAffixedStock instead of re-adding it at the discounted price.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 6: `unsoldStatus`, `MarkBought`, `SalesSince`

**Files:**
- Modify: `internal/baubles/admin.go:111-128`
- Modify: `internal/baubles/sales.go:9-45`
- Modify: `internal/baubles/record.go:17`
- Create: `internal/baubles/sales_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/baubles/sales_test.go`:

```go
package baubles

import (
	"testing"
	"time"
)

// A buyback off a shop's shelf (baubles slice D) returns a sold record to
// its unsold status by Restore's rule (ruling 2), keeps the sale on the
// record, and leaves a record an admin retired meanwhile retired. The sale
// still counts in SalesSince.
func TestMarkBoughtReturnsTheUnsoldStatus(t *testing.T) {
	withCatalog(t)
	named := seedRecord(t, Record{Name: `Bone Dice`, NameSimple: `dice`, Tier: TierAverage, Value: 12, Status: StatusReady, Generator: GeneratorOpenAI})
	generic := seedRecord(t, Record{Name: `Trinket`, NameSimple: `trinket`, Tier: TierAverage, Value: 11, Status: StatusFallback, Generator: GeneratorLocal})
	retired := seedRecord(t, Record{Name: `Rude Name`, Tier: TierAverage, Value: 13, Status: StatusReady, Generator: GeneratorOpenAI})
	seedRecord(t, Record{Name: `Tin Cup`, NameSimple: `cup`, Tier: TierAverage, Value: 10, Status: StatusReady, Generator: GeneratorOpenAI}) // never sold: its SoldAt is zero

	start := time.Now().UTC().Add(-time.Second)
	for _, r := range []Record{named, generic, retired} {
		if !MarkSold(r.Id, 6, 42) {
			t.Fatalf("sell %s", r.Id)
		}
	}
	if err := Retire(retired.Id, `Admin`); err != nil {
		t.Fatal(err)
	}

	for id, want := range map[string]Status{named.Id: StatusReady, generic.Id: StatusFallback, retired.Id: StatusRetired} {
		if !MarkBought(id, 7) {
			t.Fatalf("buy back %s", id)
		}
		got, _ := Get(id)
		if got.Status != want {
			t.Errorf("%s bought back: status %s, want %s", id, got.Status, want)
		}
		if got.SoldValue != 6 || got.SoldAt.IsZero() {
			t.Errorf("%s: the sale stays on the record: %+v", id, got)
		}
	}
	if MarkBought(`B0000404`, 7) {
		t.Fatal("no such record")
	}
	if n, gold := SalesSince(start); n != 3 || gold != 18 {
		t.Fatalf("a buyback does not erase a sale: %d sales, %d gold, want 3 and 18", n, gold)
	}
	if n, _ := SalesSince(time.Time{}); n != 3 {
		t.Fatalf("a record never sold (the tin cup) is not a sale: %d, want 3", n)
	}
}

// unsoldStatus is Restore's rule, shared (ruling 2).
func TestUnsoldStatusIsRestoresRule(t *testing.T) {
	for g, want := range map[Generator]Status{
		GeneratorOpenAI: StatusReady, GeneratorCorpus: StatusReady,
		GeneratorLocal: StatusFallback, GeneratorAdmin: StatusFallback, ``: StatusFallback,
	} {
		if got := (Record{Generator: g}).unsoldStatus(); got != want {
			t.Errorf("generator %q: %s, want %s", g, got, want)
		}
	}
}
```

(`withCatalog` gives a fresh catalog holding these four records. `SalesSince(time.Time{})` counts the tin cup, never sold, unless the `SoldAt.IsZero()` test is there: a zero `SoldAt` is not before a zero `t`.)

- [ ] **Step 2: Run to see it fail**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/baubles/ -run 'TestMarkBought|TestUnsoldStatus' -count=1`
Expected: FAIL, build errors `undefined: MarkBought` and `unsoldStatus undefined`.

- [ ] **Step 3: Implement**

`internal/baubles/admin.go`, in `Restore`, replace

```go
		r.Status = StatusFallback
		if r.Generator.Named() {
			r.Status = StatusReady
		}
		if r.SoldValue > 0 {
```

with

```go
		r.Status = r.unsoldStatus()
		if r.SoldValue > 0 {
```

and add `unsoldStatus` right after `Restore` by replacing (unique: only `Restore` ends just above that comment)

```go
	return nil
}

// EditFields are the fields an admin may change by hand.
```

with

```go
	return nil
}

// unsoldStatus is the status of a record not sold, or bought back: ready
// when the model or the corpus named it, fallback for a generic trinket.
// Restore and MarkBought share it (baubles slice D, ruling 2).
func (r Record) unsoldStatus() Status {
	if r.Generator.Named() {
		return StatusReady
	}
	return StatusFallback
}

// EditFields are the fields an admin may change by hand.
```

`internal/baubles/sales.go`: replace

```go
// Every record is sellable, a sold one included: a sold record only reaches a
// merchant again if a crash lost the seller's save after the sale was
// recorded, and refusing it would leave the player holding an object nobody
// will ever buy.
```

with

```go
// Every record is sellable, a sold one included. A record goes back into a
// pack two ways: bought back off a shop's shelf (MarkBought, baubles slice
// D), which makes it unsold again, and a crash that lost the seller's save
// after the sale was recorded, the only way a record still marked sold
// reaches a merchant again. Refusing that one would leave the player holding
// an object nobody will ever buy.
```

Replace

```go
// SalesSince counts the baubles sold at or after t and the gold paid for
// them. Used by the admin command to watch how much gold baubles put into
// the economy.
func SalesSince(t time.Time) (count int, gold int) {
	cat.mu.RLock()
	defer cat.mu.RUnlock()
	for _, r := range cat.records {
		if r.Status == StatusSold && !r.SoldAt.Before(t) {
```

with

```go
// MarkBought records that a bauble was bought back off a shop's shelf
// (baubles slice D): a sold record returns to its unsold status
// (unsoldStatus, Restore's rule). A record in any other status is left as
// it is, so one an admin retired while it sat on the shelf stays retired.
// SoldAt, SoldValue and every theft field are kept: the sale happened. It
// returns false when there is no such record.
func MarkBought(id string, buyerUserId int) bool {
	rec, ok := Update(id, func(r *Record) {
		if r.Status == StatusSold {
			r.Status = r.unsoldStatus()
		}
	})
	if ok {
		mudlog.Info(`baubles`, `action`, `bought`, `id`, id, `status`, string(rec.Status), `buyerUserId`, buyerUserId)
	}
	return ok
}

// SalesSince counts the baubles sold at or after t and the gold paid for
// them. Used by the admin command to watch how much gold baubles put into
// the economy. It reads SoldAt alone, so a sale still counts after a
// buyback; a record sold twice keeps only its latest SoldAt and SoldValue
// and counts once, at that sale.
func SalesSince(t time.Time) (count int, gold int) {
	cat.mu.RLock()
	defer cat.mu.RUnlock()
	for _, r := range cat.records {
		if !r.SoldAt.IsZero() && !r.SoldAt.Before(t) {
```

`internal/baubles/record.go`: replace `// sold to a merchant; the item is gone` with `// sold to a merchant; destroyed, or on its shelf`. Run `gofmt -w internal/baubles/record.go internal/baubles/admin.go internal/baubles/sales.go internal/baubles/sales_test.go`.

- [ ] **Step 4: Run to see it pass**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/baubles/ -count=1 && go build ./... && go test . -count=1 2>&1 | tail -2`
Expected: `ok` (`TestRetireAndRestore`, `TestRestoreReturnsACorpusRecordToReady`, `TestMarkSold` pass through `unsoldStatus` unchanged), build clean, root `ok`.

- [ ] **Step 5: Null probes**

1. In `MarkBought` drop the `if r.Status == StatusSold` guard (always `r.Status = r.unsoldStatus()`). Run `go test ./internal/baubles/ -run TestMarkBought -count=1`. Expected: FAIL `bought back: status ready, want retired`. Restore.
2. Restore `SalesSince`'s old condition `r.Status == StatusSold && !r.SoldAt.Before(t)`. Expected: FAIL `a buyback does not erase a sale: 0 sales`. Restore.
2b. Drop only `!r.SoldAt.IsZero() && ` from the new condition. Expected: FAIL `a record never sold (the tin cup) is not a sale: 4, want 3`. Restore.
3. Make `unsoldStatus` return `StatusReady` always. Expected: FAIL in both tests, naming `GeneratorLocal`/the generic record, and `TestRetireAndRestore` still passes (it covers only a named record). Restore; re-run Step 4.

- [ ] **Step 6: Commit**

```bash
cd /c/tmp/dogmud-baubles-d-impl && git add internal/baubles/admin.go internal/baubles/sales.go internal/baubles/record.go internal/baubles/sales_test.go && git commit -F - <<'EOF'
feat(baubles): MarkBought and a shared unsoldStatus; sales counted by SoldAt

A buyback off a shelf returns a sold record to Restore's unsold status
(ruling 2) and leaves a retired one retired. SalesSince reads SoldAt
alone, so a buyback does not erase a sale.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 7: `bauble show` keeps the last sale

**Files:**
- Modify: `internal/usercommands/admin.bauble.go:248-250`
- Modify: `internal/usercommands/admin.bauble_test.go` (append)

- [ ] **Step 1: Write the failing test**

Append to `internal/usercommands/admin.bauble_test.go`:

```go
// bauble show keeps a bauble's last sale in view after a buyback put it back
// in a pack (baubles slice D): the header shows its status now, the sold
// line what it last sold for.
func TestAdminBauble_ShowKeepsTheLastSaleAfterABuyback(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	baubles.SetDirForTest(t.TempDir())
	defer items.SetBaubleResolver(nil)
	admin, room := getTestUserAndRoom(t)

	rec, err := baubles.Create(baubles.Record{Name: "Bone Dice", NameSimple: "dice", Tier: baubles.TierAverage,
		Value: 12, WeightLbs: 0.2, Description: "A pair of yellowed bone dice.", Status: baubles.StatusReady,
		Generator: baubles.GeneratorOpenAI})
	require.NoError(t, err)
	require.True(t, baubles.MarkSold(rec.Id, 6, 7))
	require.True(t, baubles.MarkBought(rec.Id, 8))

	out := adminSaid(t, "show "+rec.Id, admin, room)
	assert.Contains(t, out, "[ready]", "the header shows where it is now")
	assert.Contains(t, out, "last sold: 6 gold", "the sale stays visible")
}
```

- [ ] **Step 2: Run to see it fail**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/usercommands/ -run TestAdminBauble_ShowKeepsTheLastSale -count=1`
Expected: FAIL `... does not contain "last sold: 6 gold"`.

- [ ] **Step 3: Implement**

In `internal/usercommands/admin.bauble.go` replace

```go
	if rec.Status == baubles.StatusSold {
		fmt.Fprintf(&b, "  sold:        %d gold, %s\r\n", rec.SoldValue, rec.SoldAt.Format(`2006-01-02 15:04 MST`))
	}
```

with

```go
	// The last sale stays visible after a buyback puts the record back in a
	// pack (baubles slice D); the [status] header says where it is now.
	if rec.SoldValue > 0 {
		fmt.Fprintf(&b, "  last sold:   %d gold, %s\r\n", rec.SoldValue, rec.SoldAt.Format(`2006-01-02 15:04 MST`))
	}
```

- [ ] **Step 4: Run to see it pass**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/usercommands/ -run 'TestAdminBauble' -count=1 && go test . -count=1 2>&1 | tail -2`
Expected: `ok`, root `ok`.

- [ ] **Step 5: Null probe**

Change the condition back to `rec.Status == baubles.StatusSold`. Run the Step 2 command. Expected: FAIL on `last sold: 6 gold`. Restore; re-run Step 4.

- [ ] **Step 6: Commit**

```bash
cd /c/tmp/dogmud-baubles-d-impl && git add internal/usercommands/admin.bauble.go internal/usercommands/admin.bauble_test.go && git commit -F - <<'EOF'
feat(admin): bauble show prints the last sale whenever there was one

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 8a: `ShopSnapshot.Fence` and `Type()`

**Files:**
- Modify: `internal/economy/health/snapshot.go:65` and before `// StockSnapshot is a per-item entry.`
- Modify: `internal/economy/health/capture.go:57-58`
- Create: `internal/economy/health/fence_type_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/economy/health/fence_type_test.go`:

```go
package health_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/economy/health"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"gopkg.in/yaml.v2"
)

// A fence's shop is typed "fence" on the dashboard (baubles slice D), from
// its mob template, while its snapshot keeps its real craft_support.
func TestCaptureSnapshot_FenceShopIsTypedFence(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Balance.BaubleFenceGroups = configs.ConfigSliceString{"fence"}
	configs.SetConfigForTest(t, cfg)
	shops.ClearCache()
	t.Cleanup(shops.ClearCache)
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{
		104: {MobId: 104, Groups: []string{"fence"}, Character: characters.Character{Name: "Fence Dealer Siv"}},
		108: {MobId: 108, Character: characters.Character{Name: "Jeweler Tess"}},
	}, map[int]*mobs.Mob{}))
	shops.RegisterShop("testzone", 104, 475, shops.ShopInventory{Gold: 500, StartingGold: 500, CraftSupport: shops.CraftSupportGeneral})
	shops.RegisterShop("testzone", 108, 482, shops.ShopInventory{Gold: 500, StartingGold: 500, CraftSupport: shops.CraftSupportJewelcrafting})

	byMob := map[int]health.ShopSnapshot{}
	for _, s := range health.CaptureSnapshot().Shops {
		byMob[s.MobId] = s
	}
	fence, tess := byMob[104], byMob[108]
	if !fence.Fence || fence.Type() != "fence" || fence.CraftSupport != "general" {
		t.Errorf("fence: Fence=%v Type=%q CraftSupport=%q, want true, fence, general", fence.Fence, fence.Type(), fence.CraftSupport)
	}
	if tess.Fence || tess.Type() != "jewelcrafting" {
		t.Errorf("jeweller: Fence=%v Type=%q, want false, jewelcrafting", tess.Fence, tess.Type())
	}
}

// A snapshot saved before Fence existed has no fence key: it decodes with
// Fence false and Type() is its craft_support; a false Fence writes no key.
func TestShopSnapshot_PreFenceYAMLTypesByCraftSupport(t *testing.T) {
	var s health.ShopSnapshot
	if err := yaml.Unmarshal([]byte("zone: thornwall\nmob_id: 104\ncraft_support: general\n"), &s); err != nil {
		t.Fatal(err)
	}
	if s.Fence || s.Type() != "general" {
		t.Errorf("old snapshot: Fence=%v Type=%q, want false, general", s.Fence, s.Type())
	}
	out, err := yaml.Marshal(health.ShopSnapshot{CraftSupport: "general"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "fence") {
		t.Errorf("omitempty keeps a non-fence snapshot unchanged:\n%s", out)
	}
	// The dashboard reads the JSON (s.fence in index.html).
	js, err := json.Marshal(health.ShopSnapshot{CraftSupport: "general"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(js), `"fence"`) {
		t.Errorf("json omitempty: a non-fence snapshot carries no fence key: %s", js)
	}
	js, err = json.Marshal(health.ShopSnapshot{CraftSupport: "general", Fence: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), `"fence":true`) {
		t.Errorf("json: a fence's snapshot says so as fence: %s", js)
	}
}
```

- [ ] **Step 2: Run to see them fail**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/economy/health/ -run 'FenceShopIsTypedFence|PreFenceYAML' -count=1`
Expected: FAIL, build errors `fence.Fence undefined` and `s.Type undefined`.

- [ ] **Step 3: Implement**

`internal/economy/health/snapshot.go`: after the line

```go
	CraftSupport     string          `yaml:"craft_support"      json:"craft_support"`
```

insert

```go
	Fence            bool            `yaml:"fence,omitempty"    json:"fence,omitempty"` // a fence's shop (its mob template's IsFence); see Type
```

and before `// StockSnapshot is a per-item entry.` insert

```go
// Type is the kind of shop the dashboard groups by: "fence" for a fence's
// shop (baubles slice D), else its craft_support. A snapshot saved before
// Fence existed decodes with it false and falls back to CraftSupport, and a
// fence's real craft_support stays in history.
func (s ShopSnapshot) Type() string {
	if s.Fence {
		return "fence"
	}
	return s.CraftSupport
}

```

`internal/economy/health/capture.go`: replace

```go
			Name:             lookupShopMobName(inv.MobId, inv.RoomId),
		}
```

with

```go
			Name:             lookupShopMobName(inv.MobId, inv.RoomId),
		}
		// A fence's shop is typed "fence" on the dashboard. The template
		// decides (always loaded at boot): an instance's groups can change.
		tmpl := mobs.GetMobSpec(mobs.MobId(inv.MobId))
		ss.Fence = tmpl != nil && tmpl.IsFence()
```

Run `gofmt -w internal/economy/health/snapshot.go internal/economy/health/capture.go internal/economy/health/fence_type_test.go`.

- [ ] **Step 4: Run to see them pass**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/economy/health/ -count=1 && go test . -count=1 2>&1 | tail -2`
Expected: `ok` (the existing `TestSnapshot_YAMLRoundTrip`, `TestCaptureSnapshot_Shops`, `TestLookupShopMobName_FallsBackToTemplate` too), root `ok`.

- [ ] **Step 5: Null probe**

Replace `ss.Fence = tmpl != nil && tmpl.IsFence()` with `ss.Fence = tmpl != nil && false` (so `tmpl` stays used and the file compiles). Run the Step 2 command. Expected: FAIL `fence: Fence=false Type="general"`. Restore; re-run Step 4.

- [ ] **Step 6: Commit**

```bash
cd /c/tmp/dogmud-baubles-d-impl && git add internal/economy/health/snapshot.go internal/economy/health/capture.go internal/economy/health/fence_type_test.go && git commit -F - <<'EOF'
feat(economy): ShopSnapshot.Fence from the mob template, and Type()

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 8b: The dashboard groups by `Type()`

**Files:**
- Modify: `internal/economy/health/scoring.go:135-155, 825`
- Modify: `_datafiles/html/admin/economy/index.html:314, 327`
- Modify: `internal/economy/health/fence_type_test.go` (append)
- Create: `internal/economy/health/economy_page_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/economy/health/economy_page_test.go`:

```go
package health_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The admin economy page groups shops by Type() and looks each group's
// score up by the key PerCraftSupportScores uses, labelling the empty key
// only when it renders it (baubles slice D). The page is JavaScript with no
// harness of its own, so this reads its source: a static check that the
// grouping key and the score lookup agree.
func TestEconomyPage_GroupsByTypeAndLooksUpScoresByKey(t *testing.T) {
	_, here, _, _ := runtime.Caller(0)
	page, err := os.ReadFile(filepath.Join(filepath.Dir(here), "..", "..", "..", "_datafiles", "html", "admin", "economy", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(page)
	for _, want := range []string{
		`var disc = s.fence ? "fence" : (s.craft_support || "");`,
		`var score = d.scores.PerCraftSupport[disc] || 0;`,
		`disc || "(uncategorized)",`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("index.html lacks %q", want)
		}
	}
	if strings.Contains(src, `s.craft_support || "(uncategorized)"`) {
		t.Error(`grouping under "(uncategorized)" looks the score up under a key PerCraftSupportScores never uses ("")`)
	}
}
```

and

Append to `internal/economy/health/fence_type_test.go`:

```go
// Fences group under "fence" whatever their craft_support, in the rollup
// and on the per-shop rows (baubles slice D).
func TestScore_FenceIsItsOwnType(t *testing.T) {
	snap := health.Snapshot{Shops: []health.ShopSnapshot{
		{CraftSupport: "general", Fence: true, Stock: []health.StockSnapshot{{RestockQty: 1, Current: 2, Max: 10}}}, // 20
		{CraftSupport: "general", Stock: []health.StockSnapshot{{RestockQty: 1, Current: 6, Max: 10}}},              // 60
	}}
	scores := health.PerCraftSupportScores(snap)
	if scores["fence"] < 19.99 || scores["fence"] > 20.01 || scores["general"] < 59.99 || scores["general"] > 60.01 {
		t.Errorf("rollup: %v, want fence 20 and general 60", scores)
	}
	rows := health.ScoreWithConfig(&snap, nil, testScoringCfg).PerShop
	if rows[0].CraftSupport != "fence" || rows[1].CraftSupport != "general" {
		t.Errorf("per-shop types: %q %q, want fence general", rows[0].CraftSupport, rows[1].CraftSupport)
	}
}
```

- [ ] **Step 2: Run to see it fail**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/economy/health/ -run 'TestScore_FenceIsItsOwnType|TestEconomyPage_' -count=1`
Expected: FAIL `rollup: map[general:40], want fence 20 and general 60`, `per-shop types: "general" "general"`, and the page check's `index.html lacks "var disc = s.fence ...` plus its `(uncategorized)` error.

- [ ] **Step 3: Implement**

`internal/economy/health/scoring.go`: replace

```go
// PerCraftSupportScores returns mean per-shop score grouped by the
// craft discipline each shop supports. Shops with empty CraftSupport
// roll into key "", shown in the UI as "(uncategorized)". Startup
// validation allows that only for a fence's shop, which buys no
// ordinary loot (shops.ValidateShopMobTags).
```

with

```go
// PerCraftSupportScores returns mean per-shop score grouped by each shop's
// Type(): "fence" for a fence's shop (baubles slice D), else the craft
// discipline it supports. A shop with an empty CraftSupport that is not
// flagged a fence (a snapshot saved before Fence existed) rolls into key
// "", shown in the UI as "(uncategorized)".
```

Replace `		b, exists := buckets[s.CraftSupport]` with `		b, exists := buckets[s.Type()]`, `			buckets[s.CraftSupport] = b` with `			buckets[s.Type()] = b`, and `			CraftSupport:    s.CraftSupport,` with `			CraftSupport:    s.Type(),`.

`_datafiles/html/admin/economy/index.html`: group by the scoring key and label the empty key when rendering, which also fixes a pre-existing bug on this line: the page grouped a shop with no craft_support under `"(uncategorized)"` and then looked its score up under that label, a key `PerCraftSupportScores` never uses (it keys such shops `""`), so that row always showed no score. Replace

```js
            var disc = s.craft_support || "(uncategorized)";
```

with

```js
            var disc = s.fence ? "fence" : (s.craft_support || "");
```

and replace

```js
                disc,
                shops.length,
```

with

```js
                disc || "(uncategorized)",
                shops.length,
```

(`var score = d.scores.PerCraftSupport[disc] || 0;` between them is unchanged and now looks up the real key.)

- [ ] **Step 4: Run to see it pass**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/economy/health/ -count=1 && go test . -count=1 2>&1 | tail -2`
Expected: `ok` (the existing `TestScore_PerCraftSupport_*` and the new page check too), root `ok`.

- [ ] **Step 5: Null probe**

1. Put `buckets[s.CraftSupport]` back in both places. Run the Step 2 command. Expected: FAIL on the rollup. Restore.
2. Put `|| "(uncategorized)"` back into the page's `var disc` line. Expected: FAIL in `TestEconomyPage_GroupsByTypeAndLooksUpScoresByKey`. Restore; re-run Step 4.

- [ ] **Step 6: Commit**

```bash
cd /c/tmp/dogmud-baubles-d-impl && git add internal/economy/health/scoring.go internal/economy/health/fence_type_test.go internal/economy/health/economy_page_test.go _datafiles/html/admin/economy/index.html && git commit -F - <<'EOF'
feat(economy): the dashboard groups fences under "fence"

The page now groups by the scoring key and labels the empty key when it
renders, so a shop with no craft_support finally shows its score.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 14: Checkpoint: foundations review (runs after Task 8b)

**Files:** none unless the review finds something.

- [ ] **Step 1: Dispatch a reviewer (opus)** with `superpowers:requesting-code-review`, the diff `git -C /c/tmp/dogmud-baubles-d-impl diff $(cat /c/tmp/dogmud-baubles-d-impl.base)..HEAD`, the spec path, and this checklist: `FindMatchIndexIn` is behaviour-identical to the old `FindMatchIn` for every input; `EnforceAffixedCap` never removes a held entry and terminates; `RestoreAffixedStock` clamps; the `config.yaml` diff is exactly six lines; `validateMisc` no longer defaults the cap and `validateShops` does; `MarkBought` never changes a non-sold status; `SalesSince` excludes a zero `SoldAt`; `Restore` behaviour unchanged; `Fence` is `omitempty` in yaml and json; every Go default quoted in a comment matches `config.yaml`.
- [ ] **Step 2: Fix findings** each with a failing test first where it is behaviour, then re-run `go test ./internal/util/ ./internal/configs/ ./internal/shops/ ./internal/baubles/ ./internal/usercommands/ ./internal/economy/health/ ./internal/actions/ ./modules/auctions/ . -count=1`. Commit fixes as `fix(<pkg>): <what> (review)`.

---

### Task 9: A player's shelvable sale goes on the shelf

**Files:**
- Modify: `internal/actions/sell_bauble.go:18-25, 240-247`, and add `baubleShelvable`
- Modify: `internal/actions/sell.go:317`
- Modify: `internal/actions/sell_bauble_test.go:185-191` and append
- Modify: `internal/actions/stolen_bauble_test.go:39-46` and append

- [ ] **Step 1: Write the failing tests**

In `internal/actions/stolen_bauble_test.go` replace `pinStolenClock`'s body lines

```go
	origSale, origStolen := baubleNowForSale, stolenNow
	baubleNowForSale = func() time.Time { return now }
	stolenNow = func() time.Time { return now }
	t.Cleanup(func() { baubleNowForSale, stolenNow = origSale, origStolen })
```

with

```go
	origSale, origStolen, origShelf := baubleNowForSale, stolenNow, shops.ShelfNow
	baubleNowForSale = func() time.Time { return now }
	stolenNow = func() time.Time { return now }
	shops.ShelfNow = func() time.Time { return now }
	t.Cleanup(func() { baubleNowForSale, stolenNow, shops.ShelfNow = origSale, origStolen, origShelf })
```

and append:

```go
// A hot bauble a player sells to a fence's living shop is shelved but held
// out of sight until its heat ends everywhere: StolenAt plus
// BaubleStolenHeatHours (baubles slice D, spec test 8).
func TestStolenBauble_AHotOneIsShelvedHeldUntilItCools(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 0)()
	pinStolenClock(t, stolenTestNow)
	cfg := configs.GetConfig()
	cfg.Balance.BaubleStolenHeatHours = 72
	configs.SetConfigForTest(t, cfg)
	merchantInstance().Groups = []string{`fence`}

	shops.ClearCache()
	_ = shops.RemoveShopFile("TestZone", 2, 1)
	defer shops.RemoveShopFile("TestZone", 2, 1)
	defer shops.ClearCache()
	si := shops.RegisterShop("TestZone", 2, 1, shops.ShopInventory{Gold: 1000, StartingGold: 1000, CraftSupport: shops.CraftSupportGeneral})

	stolenAt := stolenTestNow.Add(-time.Hour)
	hot := stolenBauble(t, "Tarnished Brass Thimble", "thimble", 12, 99, 1, stolenAt)
	seller := newSellerActor(t, true)
	require.True(t, seller.GetCharacter().StoreItem(hot))
	res := Sell(seller, SellOptions{ItemName: "thimble", Quantity: 1})
	require.Equal(t, 1, res.Sold, "res=%+v", res)

	require.Len(t, si.AffixedStock, 1, "a player's sale of an average bauble is shelved")
	e := si.AffixedStock[0]
	assert.Equal(t, hot.Bauble, e.Item.Bauble)
	assert.Equal(t, 12, e.Price, "relisted at its catalog value")
	assert.True(t, e.HoldUntil.Equal(stolenAt.Add(72*time.Hour)), "held until the heat ends: %v", e.HoldUntil)
	assert.True(t, e.AddedAt.Equal(stolenTestNow), "shelved on the shelf clock: %v", e.AddedAt)
	assert.Equal(t, 1, si.HeldCount(stolenTestNow))
	assert.Empty(t, si.ListedIndexes(stolenTestNow), "not listed while held")
}
```

In `internal/actions/sell_bauble_test.go` replace

```go
	// A general store does, from its own gold, and shelves nothing.
```

with

```go
	// A general store does, from its own gold, and puts it on its shelf at
	// its catalog value (baubles slice D).
```

and

```go
	assert.Len(t, si.AffixedStock, 0, "baubles are not resold like affixed loot")
```

with

```go
	require.Len(t, si.AffixedStock, 1, "an average bauble a player sells is shelved")
	assert.Equal(t, b.Bauble, si.AffixedStock[0].Item.Bauble)
	assert.Equal(t, 12, si.AffixedStock[0].Price, "relisted at its catalog value")
	assert.True(t, si.AffixedStock[0].HoldUntil.IsZero(), "an honest bauble is listed at once")
```

and append:

```go
// Only a player's sale of a shelvable bauble to a living shop shelves it
// (baubles slice D, spec test 7): a mob's sale (ruling 7: the shop paid
// nothing), a cheap bauble (ruling 5) and a retired one (ruling 1) are
// destroyed, and each record is still marked sold.
func TestSell_Bauble_OnlyAPlayersShelvableSaleIsShelved(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 0)()

	shops.ClearCache()
	_ = shops.RemoveShopFile("TestZone", 2, 1)
	defer shops.RemoveShopFile("TestZone", 2, 1)
	defer shops.ClearCache()
	si := shops.RegisterShop("TestZone", 2, 1, shops.ShopInventory{Gold: 1000, CraftSupport: shops.CraftSupportGeneral})

	mob := newSellerActor(t, false)
	mob.GetCharacter().Stats.Strength.ValueAdj = 100 // a bare mob fixture has no carrying capacity
	fromMob := newBauble(t, "Painted Wooden Horse", "horse", 12, baubles.StatusReady)
	require.True(t, mob.GetCharacter().StoreItem(fromMob))
	res := Sell(mob, SellOptions{ItemName: "horse", Quantity: 1})
	require.Equal(t, 1, res.Sold, "res=%+v", res)

	player := newSellerActor(t, true)
	cheap := newBauble(t, "Chipped Clay Cup", "cup", 4, baubles.StatusReady)
	retired := newBauble(t, "Rude Carving", "carving", 12, baubles.StatusRetired)
	require.True(t, player.GetCharacter().StoreItem(cheap))
	require.True(t, player.GetCharacter().StoreItem(retired))
	require.Equal(t, 1, Sell(player, SellOptions{ItemName: "cup", Quantity: 1}).Sold)
	require.Equal(t, 1, Sell(player, SellOptions{ItemName: "trinket", Quantity: 1}).Sold, "a retired bauble reads Trinket")

	assert.Empty(t, si.AffixedStock, "none of the three is shelved")
	for _, it := range []items.Item{fromMob, cheap, retired} {
		rec, _ := baubles.Get(it.Bauble)
		assert.Equal(t, baubles.StatusSold, rec.Status, "%s: the sale is recorded", it.Bauble)
	}
}
```

- [ ] **Step 2: Run to see them fail**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/actions/ -run 'TestSell_Bauble_LivingShopByCraftSupport|TestStolenBauble_AHotOneIsShelvedHeldUntilItCools|TestSell_Bauble_OnlyAPlayersShelvableSaleIsShelved' -count=1`
Expected: FAIL in the first two with `should have 1 item(s), but has 0` (testify's lowercase wording) ("an average bauble a player sells is shelved" / "a player's sale of an average bauble is shelved"). `TestSell_Bauble_OnlyAPlayersShelvableSaleIsShelved` PASSES now (nothing is shelved yet); it is proven by the null probes in Step 5.

- [ ] **Step 3: Implement**

`internal/actions/sell_bauble.go`: replace the header lines

```go
// own branch, like affixed loot does: priced from the catalog value times the
// shop buy ratio, never added to stock, and the record marked sold.
```

with

```go
// own branch, like affixed loot does: priced from the catalog value times the
// shop buy ratio, and the record marked sold. A player's sale of an average
// or rare bauble (baubleShelvable) to a living-economy shop puts it on that
// shop's secondhand shelf (AffixedStock) at its catalog value, held out of
// sight while it is hot (baubles.ShelfHoldUntil); every other sale destroys
// it (baubles slice D).
```

After `baubleShopBuys` (before `// Merchant lines for bauble refusals.`) insert:

```go
// baubleShelvable reports whether a sold bauble goes on the shop's shelf
// rather than leaving the world (baubles slice D): worth more than the cheap
// tier (owner ruling 5, so a dozen value-1 trinkets cannot evict shelved
// gear, and the bauble Bartering loop stays closed), and not retired (ruling
// 1: its withdrawn text would be listed once MarkSold sets it sold).
func baubleShelvable(rec baubles.Record) bool {
	return rec.Status != baubles.StatusRetired && rec.Value > baubles.TierCheap.Range().Max
}

```

Replace

```go
	// The bauble is not stocked: it leaves the world, and its gold is the
	// only trace. The shop's gold changed, so a living-economy shop is saved.
	if shopInv != nil {
		shopInv.BuysCount++
```

with

```go
	// A player's sale of a shelvable bauble to a living-economy shop puts it
	// on the shelf at its catalog value (the affixed-loot rule), held out of
	// sight while it is hot. A mob's sale never does (ruling 7: the shop paid
	// nothing), nor does a legacy merchant's, a cheap bauble or a retired
	// one: those leave the world and the gold is the only trace. The shop
	// changed either way, so a living-economy shop is saved. The record is
	// marked sold after the save, for every sale.
	if shopInv != nil {
		if seller.IsPlayer() {
			if rec, ok := baubles.Get(item.Bauble); ok && baubleShelvable(rec) {
				now := shops.ShelfNow()
				shopInv.AddAffixedStock(item, item.GetSpec().Value,
					int(configs.GetBalanceConfig().ShopAffixedStockCap),
					baubles.ShelfHoldUntil(item, now), now)
			}
		}
		shopInv.BuysCount++
```

`internal/actions/sell.go`: replace

```go
	// Baubles (docs/baubles): catalog-priced, never stocked. See sell_bauble.go.
```

with

```go
	// Baubles (docs/baubles): catalog-priced; a player's average or rare one
	// goes on a living shop's shelf, the rest leave the world. See
	// sell_bauble.go.
```

- [ ] **Step 4: Run to see them pass, with every sale test**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/actions/ -run 'TestSell|TestStolenBauble|TestBuy' -count=1 && go test ./internal/actions/ -count=1 && go test . -count=1 2>&1 | tail -2`
Expected: `ok` three times (spec test 15, `TestSell_Bauble_ASoldRecordSellsAgain`, included; the existing fence living-shop tests now shelve a held entry and still pass).

- [ ] **Step 5: Null probes**

1. Pass `time.Time{}` instead of `baubles.ShelfHoldUntil(item, now)`. Run `go test ./internal/actions/ -run TestStolenBauble_AHotOneIsShelvedHeldUntilItCools -count=1`. Expected: FAIL `held until the heat ends`. Restore.
2. Remove the `if seller.IsPlayer()` wrapper (keep its body). Run `go test ./internal/actions/ -run TestSell_Bauble_OnlyAPlayersShelvableSaleIsShelved -count=1`. Expected: FAIL `none of the three is shelved` (the mob's horse is on the shelf). Restore.
3. Replace `ok && baubleShelvable(rec)` with `ok && rec.Id != ""` (every record; `rec` stays used). Same test. Expected: FAIL `none of the three is shelved` (the cup and the retired trinket). Restore; re-run Step 4.

- [ ] **Step 6: Commit**

```bash
cd /c/tmp/dogmud-baubles-d-impl && git add internal/actions/sell_bauble.go internal/actions/sell.go internal/actions/sell_bauble_test.go internal/actions/stolen_bauble_test.go && git commit -F - <<'EOF'
feat(sell): a player's average or rare bauble goes on the shop's shelf

A player's sale of a shelvable bauble (above the cheap tier, not
retired) to a living-economy shop shelves it at catalog value, held
until its heat ends when hot. Mob sales, legacy merchants, cheap and
retired baubles still destroy it (rulings 1, 5, 7). pinStolenClock
now pins shops.ShelfNow too.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 10: The backroom refusal

**Files:**
- Modify: `internal/actions/sell_bauble.go:40-46` and before `:128`
- Modify: `internal/actions/stolen_bauble_test.go` (append)

- [ ] **Step 1: Write the failing test**

Append to `internal/actions/stolen_bauble_test.go`:

```go
// A shop's backroom holds at most ShopAffixedStockCap hot baubles (owner
// ruling 6, spec test 6). Full, it turns away another hot shelvable one in
// the fence's voice, as an interest refusal (not Broke), and still buys a
// cold one (listed at once) and a cheap hot one (destroyed, never held).
func TestStolenBauble_AFullBackroomRefusesHotGoods(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 0)()
	pinStolenClock(t, stolenTestNow)
	cfg := configs.GetConfig()
	cfg.Balance.ShopAffixedStockCap = 2
	cfg.Balance.BaubleStolenHeatHours = 72
	configs.SetConfigForTest(t, cfg)
	merchantInstance().Groups = []string{`fence`}

	shops.ClearCache()
	_ = shops.RemoveShopFile("TestZone", 2, 1)
	defer shops.RemoveShopFile("TestZone", 2, 1)
	defer shops.ClearCache()
	si := shops.RegisterShop("TestZone", 2, 1, shops.ShopInventory{Gold: 1000, StartingGold: 1000, CraftSupport: shops.CraftSupportGeneral})
	later := stolenTestNow.Add(24 * time.Hour)
	for _, name := range []string{"held one", "held two"} {
		si.AffixedStock = append(si.AffixedStock, shops.AffixedStockEntry{
			Item:  items.Item{ItemId: sellTestItemId, Affixed: true, Spec: &items.ItemSpec{Name: name, Value: 10}},
			Price: 10, AddedAt: stolenTestNow, HoldUntil: later,
		})
	}

	hot := stolenBauble(t, "Tarnished Brass Thimble", "thimble", 12, 99, 1, stolenTestNow.Add(-time.Hour))
	offer := BaubleOfferFrom(hot, merchantInstance())
	assert.Equal(t, 0, offer.Price)
	assert.Equal(t, baubleSayBackroomFull, offer.Refusal)
	assert.False(t, offer.Broke, "an interest refusal: the next merchant in the room is tried")

	seller := newSellerActor(t, true)
	char := seller.GetCharacter()
	require.True(t, char.StoreItem(hot))
	res := Sell(seller, SellOptions{ItemName: "thimble", Quantity: 1})
	assert.Equal(t, 0, res.Sold, "res=%+v", res)
	assert.Len(t, si.AffixedStock, 2)

	cold := stolenBauble(t, "Bone Dice", "dice", 12, 99, 1, stolenTestNow.Add(-30*24*time.Hour))
	cheapHot := stolenBauble(t, "Brass Button", "button", 4, 99, 1, stolenTestNow.Add(-time.Hour))
	require.True(t, char.StoreItem(cold))
	require.True(t, char.StoreItem(cheapHot))
	require.Equal(t, 1, Sell(seller, SellOptions{ItemName: "dice", Quantity: 1}).Sold, "a cold one still sells")
	require.Equal(t, 1, Sell(seller, SellOptions{ItemName: "button", Quantity: 1}).Sold, "a cheap hot one still sells")
	require.Len(t, si.AffixedStock, 3, "the cold one is shelved; the cheap hot one is not shelved at all")
	assert.Equal(t, cold.Bauble, si.AffixedStock[2].Item.Bauble)
	assert.Equal(t, 2, si.HeldCount(stolenTestNow), "the backroom is unchanged")
}

// An honest shop buys a bauble that is hot only in another heat area (not
// HotIn here) and holds it, so its backroom can fill too. Full, it refuses
// the next one in an honest voice, never the fence's line about hot goods
// (controller decision, plan review 2026-09-30).
func TestStolenBauble_AnHonestShopWithAFullBackroomHasNoRoom(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 0)()
	pinStolenClock(t, stolenTestNow)
	cfg := configs.GetConfig()
	cfg.Balance.ShopAffixedStockCap = 2
	cfg.Balance.BaubleStolenHeatHours = 72
	configs.SetConfigForTest(t, cfg)

	shops.ClearCache()
	_ = shops.RemoveShopFile("TestZone", 2, 1)
	defer shops.RemoveShopFile("TestZone", 2, 1)
	defer shops.ClearCache()
	si := shops.RegisterShop("TestZone", 2, 1, shops.ShopInventory{Gold: 1000, StartingGold: 1000, CraftSupport: shops.CraftSupportGeneral})
	held := shops.AffixedStockEntry{
		Item:  items.Item{ItemId: sellTestItemId, Affixed: true, Spec: &items.ItemSpec{Name: "held", Value: 10}},
		Price: 10, AddedAt: stolenTestNow, HoldUntil: stolenTestNow.Add(24 * time.Hour),
	}
	si.AffixedStock = []shops.AffixedStockEntry{held, held}

	elsewhere := newBauble(t, "Bone Dice", "dice", 12, baubles.StatusReady)
	require.True(t, baubles.MarkStolen(elsewhere.Bauble, baubles.Theft{ByUserId: 1, RoomId: 1, Zone: "Faraway", FromMob: 99, FromName: "Merchant"}, stolenTestNow.Add(-time.Hour)))

	offer := BaubleOfferFrom(elsewhere, merchantInstance())
	assert.Equal(t, 0, offer.Price)
	assert.Equal(t, baubleSayNoRoom, offer.Refusal, "an honest shop does not talk about moving hot goods")
	assert.False(t, offer.Broke)

	si.AffixedStock = si.AffixedStock[:1]
	assert.Equal(t, 6, BaubleOfferFrom(elsewhere, merchantInstance()).Price, "with room, it buys a bauble that is hot only elsewhere")
}
```

- [ ] **Step 2: Run to see it fail**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/actions/ -run 'TestStolenBauble_AFullBackroomRefusesHotGoods|TestStolenBauble_AnHonestShopWithAFullBackroomHasNoRoom' -count=1`
Expected: FAIL, build errors `undefined: baubleSayBackroomFull` and `undefined: baubleSayNoRoom`.

- [ ] **Step 3: Implement**

`internal/actions/sell_bauble.go`: replace the const block

```go
const (
	baubleSayUnknown    = "I'm afraid I don't buy those."
	baubleSayNotBuyer   = "I'm not interested in trinkets. Try a general store or a jeweller."
	baubleSayCantAfford = "I can't afford that right now."
	baubleSayHot        = "That was stolen, and not long ago. I won't touch it. Try someone less particular about where things come from."
)
```

with

```go
const (
	baubleSayUnknown      = "I'm afraid I don't buy those."
	baubleSayNotBuyer     = "I'm not interested in trinkets. Try a general store or a jeweller."
	baubleSayCantAfford   = "I can't afford that right now."
	baubleSayHot          = "That was stolen, and not long ago. I won't touch it. Try someone less particular about where things come from."
	baubleSayBackroomFull = "I can't move any more hot goods right now. Come back once some of what I'm sitting on has cooled."
	baubleSayNoRoom       = "I'm afraid I've no room for more of those right now."
)
```

(`baubleSayBackroomFull` is a fence's line; `baubleSayNoRoom` is an honest shop's, in the voice of `baubleSayUnknown`.)

Replace

```go
	if shopInv != nil {
		ratio := float64(configs.GetBalanceConfig().ShopGoldReserveRatio)
```

with

```go
	// The backroom (owner ruling 6, baubles slice D): a shop holds at most
	// ShopAffixedStockCap baubles that were hot when shelved. A full one
	// refuses another hot, shelvable bauble from any seller, so the offer is
	// one rule for every caller; it is an interest refusal, so
	// bestBaubleMerchant tries the next merchant, and offer and appraise
	// show the line. Held entries count here only, never against the listed
	// cap. The check and the add run in one sale under the mud lock. An
	// honest shop fills its backroom too (a bauble hot only in another heat
	// area), and says so in an honest voice; a fence talks about hot goods.
	if shopInv != nil && baubleShelvable(rec) {
		now := shops.ShelfNow()
		if rec.Hot(now) && shopInv.HeldCount(now) >= int(configs.GetBalanceConfig().ShopAffixedStockCap) {
			if fence {
				return BaubleOffer{Refusal: baubleSayBackroomFull}
			}
			return BaubleOffer{Refusal: baubleSayNoRoom}
		}
	}

	if shopInv != nil {
		ratio := float64(configs.GetBalanceConfig().ShopGoldReserveRatio)
```

- [ ] **Step 4: Run to see it pass**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/actions/ -count=1 && go test . -count=1 2>&1 | tail -2`
Expected: `ok`, root `ok`.

- [ ] **Step 5: Null probes**

1. Change `>=` to `>`. Run the Step 2 command. Expected: FAIL in both tests on the price assertion (testify prints `expected: 0` then `actual  : 8` for the fence, `actual  : 6` for the honest shop). Restore.
2. Replace `rec.Hot(now)` with `true`. Expected: FAIL `a cold one still sells`. Restore.
3. Delete the `if fence { ... }` block (every shop says `baubleSayNoRoom`). Expected: FAIL in `TestStolenBauble_AFullBackroomRefusesHotGoods` on the refusal line. Restore.
4. Make both branches return `baubleSayBackroomFull`. Expected: FAIL `an honest shop does not talk about moving hot goods`. Restore; re-run Step 4.

- [ ] **Step 6: Commit**

```bash
cd /c/tmp/dogmud-baubles-d-impl && git add internal/actions/sell_bauble.go internal/actions/stolen_bauble_test.go && git commit -F - <<'EOF'
feat(sell): a full backroom refuses more hot baubles

A shop already holding ShopAffixedStockCap hot baubles refuses another
hot shelvable one (ruling 6), as an interest refusal so the next
merchant is tried; cold and cheap ones still sell. A fence says so in
its own voice, an honest shop in a plain one.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 11: `list` shows the shelf

**Files:**
- Modify: `internal/usercommands/list.go:3-24, 58-70`, new functions before `// partitionShopStock ...`
- Create: `internal/usercommands/list_shelf_test.go`
- Modify: `bauble_finder_view_guard_test.go:75`

- [ ] **Step 1: Write the failing tests**

Create `internal/usercommands/list_shelf_test.go`:

```go
package usercommands

import (
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The secondhand shelf in `list` (baubles slice D). The table's text cannot
// be read here (no templates filesystem in this test binary, see
// list_sight_test.go), so these check the rows, whether a table is shown,
// and the shop's saved state.

var shelfListNow = time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

// shelfListFixture is listSightRoom's lit shop (merchant 841, room 8410,
// shopper 8411) with a registered living shop, the shelf clock pinned at
// shelfListNow, the cap pinned, and DataFiles and the catalog in temp dirs.
func shelfListFixture(t *testing.T, limit int) (*users.UserRecord, *rooms.Room, *shops.ShopInventory) {
	t.Helper()
	user, room := listSightRoom(t, "city")
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(t.TempDir())
	cfg.Balance.ShopAffixedStockCap = configs.ConfigInt(limit)
	configs.SetConfigForTest(t, cfg)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		84001: {ItemId: 84001, Name: "tin cup", Type: items.Object, Value: 50},
		items.BaubleItemId: {ItemId: items.BaubleItemId, Name: "Curious Trinket", NameSimple: "trinket",
			Type: items.Object, Subtype: items.Mundane, Weight: 0.2, Value: 1, NotSalable: true},
	}))
	baubles.SetDirForTest(t.TempDir())
	t.Cleanup(func() { items.SetBaubleResolver(nil) })
	orig := shops.ShelfNow
	shops.ShelfNow = func() time.Time { return shelfListNow }
	t.Cleanup(func() { shops.ShelfNow = orig })
	shops.ClearCache()
	t.Cleanup(shops.ClearCache)
	si := shops.RegisterShop("TestZone", 841, 8410, shops.ShopInventory{Gold: 100, StartingGold: 100, CraftSupport: shops.CraftSupportGeneral})
	return user, room, si
}

// shelfGear is affixed gear on the shelf.
func shelfGear(name string, price int, addedAt, holdUntil time.Time) shops.AffixedStockEntry {
	return shops.AffixedStockEntry{
		Item:  items.Item{ItemId: 84001, Affixed: true, Spec: &items.ItemSpec{ItemId: 84001, Name: name, Type: items.Object, Value: price}},
		Price: price, AddedAt: addedAt, HoldUntil: holdUntil,
	}
}

func plainRow(row []string) string {
	return listSightTag.ReplaceAllString(strings.Join(row, " | "), "")
}

// Spec test 1 (list half): the shelf lists in shelf order, unsorted, without
// the held entry, at each entry's own price; with only held entries nothing
// is shown, so the "nothing to sell" line can fire.
func TestListShelf_ShowsListedEntriesInShelfOrder(t *testing.T) {
	user, _, si := shelfListFixture(t, 12)
	// Shelf order Zinc, Amber, Copper differs from every sort a renderer
	// might slip in: by name (Amber, Copper, Zinc), by price ascending
	// (Amber 10, Copper 20, Zinc 30) and by price descending (Zinc, Copper,
	// Amber).
	si.AffixedStock = []shops.AffixedStockEntry{
		shelfGear("Zinc Ring", 30, shelfListNow.Add(-3*time.Hour), time.Time{}),
		shelfGear("Held Torc", 90, shelfListNow.Add(-2*time.Hour), shelfListNow.Add(time.Hour)),
		shelfGear("Amber Brooch", 10, shelfListNow.Add(-time.Hour), time.Time{}),
		shelfGear("Copper Pin", 20, shelfListNow.Add(-time.Minute), time.Time{}),
	}

	rows := buildShelfRows(si, user.UserId, shelfListNow)
	require.Len(t, rows, 3, "the held entry is out of sight")
	for i, want := range []struct{ name, price string }{{"Zinc Ring", "30"}, {"Amber Brooch", "10"}, {"Copper Pin", "20"}} {
		assert.Contains(t, plainRow(rows[i]), want.name, "row %d: shelf order, not sorted by name or price", i)
		assert.Equal(t, want.price, rows[i][2], "row %d: its own price", i)
	}
	for _, r := range rows {
		assert.NotContains(t, plainRow(r), "Held Torc")
		assert.Len(t, r, 3, "Name, Type, Price")
	}
	assert.True(t, renderShelfListing(user, si, "Keeper", shelfListNow))

	si.AffixedStock = si.AffixedStock[1:2] // only the held one
	assert.False(t, renderShelfListing(user, si, "Keeper", shelfListNow), "nothing listed: no table, so list may say it has nothing")
}

// The "nothing to sell" say fires only when both tables are empty: a shop
// with no stock but a listed shelf says nothing, and one whose shelf holds
// only held entries says it.
func TestListShelf_NothingToSellOnlyWhenBothTablesAreEmpty(t *testing.T) {
	user, room, si := shelfListFixture(t, 12)
	si.AffixedStock = []shops.AffixedStockEntry{shelfGear("Zinc Ring", 30, shelfListNow, time.Time{})}
	events.DrainQueuedInputsForTest(8412)
	_, err := List("", user, room, 0)
	require.NoError(t, err)
	for _, in := range events.DrainQueuedInputsForTest(8412) {
		assert.NotContains(t, in, "nothing to sell", "a listed shelf is something to sell")
	}

	si.AffixedStock = []shops.AffixedStockEntry{shelfGear("Held Torc", 90, shelfListNow, shelfListNow.Add(time.Hour))}
	_, err = List("", user, room, 0)
	require.NoError(t, err)
	assert.Contains(t, strings.Join(events.DrainQueuedInputsForTest(8412), "\n"), "nothing to sell", "only held entries: nothing to show")
}

// Spec tests 4 and 5 (list half): list enforces the cap lazily once a hold
// has ended, evicting the entry listed earliest (not the one shelved first),
// and saves the trimmed shop.
func TestListShelf_EnforcesTheCapLazilyAndSaves(t *testing.T) {
	user, room, si := shelfListFixture(t, 2)
	si.AffixedStock = []shops.AffixedStockEntry{
		shelfGear("Old Torc", 50, shelfListNow.Add(-80*time.Hour), shelfListNow.Add(-time.Hour)), // shelved first, listed an hour ago
		shelfGear("Bone Ring", 40, shelfListNow.Add(-10*time.Hour), time.Time{}),
		shelfGear("Jet Pin", 30, shelfListNow.Add(-5*time.Hour), time.Time{}),
	}

	handled, err := List("", user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	names := func(inv *shops.ShopInventory) []string {
		out := []string{}
		for _, e := range inv.AffixedStock {
			out = append(out, e.Item.GetSpec().Name)
		}
		return out
	}
	assert.Equal(t, []string{"Old Torc", "Jet Pin"}, names(si), "Bone Ring listed earliest and goes")

	shops.ClearCache()
	reloaded := shops.GetShopInventory("TestZone", 841, 8410)
	require.NotNil(t, reloaded, "the trim was saved")
	assert.Equal(t, []string{"Old Torc", "Jet Pin"}, names(reloaded))
}

// Spec test 11 (list half): a finder-only bauble on the shelf reads its own
// name to its finder and Trinket to anyone else.
func TestListShelf_AFinderOnlyBaubleReadsByViewer(t *testing.T) {
	user, _, si := shelfListFixture(t, 12)
	rec, err := baubles.Create(baubles.Record{Name: "Painted Wooden Horse", NameSimple: "horse", Tier: baubles.TierAverage,
		Value: 12, WeightLbs: 0.5, Description: "A child's toy horse, its red paint flaking.", Status: baubles.StatusReady,
		PlayerKey: true, Moderated: false, FoundByUserId: user.UserId})
	require.NoError(t, err)
	horse := items.New(items.BaubleItemId)
	horse.Bauble = rec.Id
	si.AffixedStock = []shops.AffixedStockEntry{{Item: horse, Price: 12, AddedAt: shelfListNow}}

	mine := buildShelfRows(si, user.UserId, shelfListNow)
	theirs := buildShelfRows(si, user.UserId+1000, shelfListNow)
	require.Len(t, mine, 1)
	require.Len(t, theirs, 1)
	assert.Contains(t, plainRow(mine[0]), "Painted Wooden Horse", "the finder reads their own name")
	assert.Contains(t, plainRow(theirs[0]), "Trinket")
	assert.NotContains(t, plainRow(theirs[0]), "Horse", "nobody else reads the hidden words")
	assert.Equal(t, "12", theirs[0][2], "the price is the same in both views")
}
```

- [ ] **Step 2: Run to see them fail**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/usercommands/ -run TestListShelf_ -count=1`
Expected: FAIL, build errors `undefined: buildShelfRows` and `undefined: renderShelfListing`.

- [ ] **Step 3: Implement**

`internal/usercommands/list.go` imports: replace

```go
	"strconv"

	"github.com/GoMudEngine/GoMud/internal/actions"
```

with

```go
	"strconv"
	"time"

	"github.com/GoMudEngine/GoMud/internal/actions"
```

replace `	"github.com/GoMudEngine/GoMud/internal/conditions"` with

```go
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
```

and `	"github.com/GoMudEngine/GoMud/internal/mobs"` with

```go
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
```

In `List`, replace

```go
		shopInv := shops.GetShopInventory(mob.Zone, int(mob.MobId), mob.HomeRoomId)
		if shopInv != nil {
			stock := buildShopStockFromInventory(shopInv, user)
```

with

```go
		shopInv := shops.GetShopInventory(mob.Zone, int(mob.MobId), mob.HomeRoomId)
		if shopInv != nil {
			// The shelf cap is enforced lazily (baubles slice D): a hold that
			// ended since the last trade can push the listed entries over it.
			now := shops.ShelfNow()
			if shopInv.EnforceAffixedCap(int(configs.GetBalanceConfig().ShopAffixedStockCap), now) > 0 {
				if err := shops.SaveShop(shopInv.Zone, shopInv.MobId, shopInv.RoomId); err != nil {
					mudlog.Error("LIST", "msg", "SaveShop failed", "error", err)
				}
			}
			stock := buildShopStockFromInventory(shopInv, user)
```

and

```go
			if !renderMobMerchantListing(user, stock, mob.Character.Name) {
				mob.Command(`say I have nothing to sell right now, but check again later.`)
			}
		} else {
```

with

```go
			listedStock := renderMobMerchantListing(user, stock, mob.Character.Name)
			listedShelf := renderShelfListing(user, shopInv, mob.Character.Name, now)
			if !listedStock && !listedShelf {
				mob.Command(`say I have nothing to sell right now, but check again later.`)
			}
		} else {
```

Before `// partitionShopStock splits shop stock into four categories: items, mercs, conditions, pets.` insert:

```go
// buildShelfRows builds the "Secondhand goods" rows (baubles slice D): the
// entries of si.ListedIndexes(now), in that order and unsorted, since buy
// counts `buy 2.name` in the same order. A held entry is not shown. Name is
// the viewer's own view (DisplayNameFor): a finder-only bauble reads Trinket
// to everyone but its finder. Price is the relist price before barter, like
// the stock table's.
func buildShelfRows(si *shops.ShopInventory, viewerUserId int, now time.Time) [][]string {
	rows := [][]string{}
	for _, idx := range si.ListedIndexes(now) {
		e := &si.AffixedStock[idx]
		rows = append(rows, []string{
			e.Item.DisplayNameFor(viewerUserId),
			string(e.Item.GetSpec().Type),
			strconv.Itoa(e.Price),
		})
	}
	return rows
}

// renderShelfListing sends the lister the merchant's secondhand shelf as its
// own table, rendered "Secondhand goods by <merchant>". Returns false when
// nothing on it is listed.
func renderShelfListing(user *users.UserRecord, si *shops.ShopInventory, sellerName string, now time.Time) bool {
	rows := buildShelfRows(si, user.UserId, now)
	if len(rows) == 0 {
		return false
	}
	renderShopTable(user, `Secondhand goods`, `cyan`, sellerName, `mobname`, []string{"Name", "Type", "Price"}, rows,
		`To buy something, type: <ansi fg="command">buy [name]</ansi>`)
	return true
}

```

Run `gofmt -w internal/usercommands/list.go internal/usercommands/list_shelf_test.go`.

- [ ] **Step 4: Run the package tests, then the guard (expect the guard to fail)**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/usercommands/ -count=1 && go test . -run TestFinderViewReachesOnlyItsReader -count=1`
Expected: `ok` for `internal/usercommands`; the root test FAILS: `internal/usercommands/list.go|buildShelfRows: makes 1 finder-view reference(s), and is not in finderViewSites`.

- [ ] **Step 5: Register the site**

In `bauble_finder_view_guard_test.go` replace

```go
	"internal/usercommands/inventory.go|Inventory":             {2, "the player's own inventory listing"},
```

with

```go
	"internal/usercommands/inventory.go|Inventory":             {2, "the player's own inventory listing"},
	"internal/usercommands/list.go|buildShelfRows":             {1, "the lister's own shop listing; renderShopTable sends it to that user alone"},
```

Run `gofmt -w bauble_finder_view_guard_test.go`, then `go test . -count=1 2>&1 | tail -2`. Expected: `ok`.

- [ ] **Step 6: Null probes**

0. Change `if !listedStock && !listedShelf {` to `if !listedStock || !listedShelf {`. Run `go test ./internal/usercommands/ -run TestListShelf_NothingToSellOnlyWhenBothTablesAreEmpty -count=1`. Expected: FAIL `a listed shelf is something to sell`. Restore.
1. In `buildShelfRows` use `e.Item.DisplayName()`. Run `go test ./internal/usercommands/ -run TestListShelf_AFinderOnlyBaubleReadsByViewer -count=1` (Expected: FAIL `the finder reads their own name`) and `go test . -run TestFinderViewReachesOnlyItsReader -count=1` (Expected: FAIL `in finderViewSites but reads no finder view (stale)`). Restore.
2. Change `if shopInv.EnforceAffixedCap(` to `if false && shopInv.EnforceAffixedCap(` (the imports stay used). Run `go test ./internal/usercommands/ -run TestListShelf_EnforcesTheCapLazilyAndSaves -count=1`. Expected: FAIL `Bone Ring listed earliest and goes`. Restore.
3. Keep the trim, and in place of the save write `if err := error(nil); err != nil {` (so `mudlog` stays used and nothing is written). Same test. Expected: FAIL `the trim was saved` (reloaded is nil). Restore; re-run Steps 4 and 5.

- [ ] **Step 7: Commit**

```bash
cd /c/tmp/dogmud-baubles-d-impl && git add internal/usercommands/list.go internal/usercommands/list_shelf_test.go bauble_finder_view_guard_test.go && git commit -F - <<'EOF'
feat(list): a "Secondhand goods" table for each shop's shelf

list shows a living shop's listed shelf entries as their own table,
in shelf order, per viewer (a finder-only bauble reads Trinket to
others), enforces the listed cap lazily and saves the trim. The
"nothing to sell" line fires only when both tables are empty. Sold-on
affixed gear is finally visible too.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 12: `buy` from the shelf by position

**Files:**
- Modify: `internal/actions/buy.go:514-610, 652-654, 695-697`
- Create: `internal/actions/buy_shelf_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/actions/buy_shelf_test.go`:

```go
package actions

import (
	"fmt"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Buying off the secondhand shelf (baubles slice D).

// shelfBuyFixture seeds the bauble sale harness (carrier, catalog, room 1),
// pins the shelf clock at now and the cap at limit, points DataFiles at a
// temp dir, and registers a living shop (TestZone, template 2, room 1) with
// an empty shelf.
func shelfBuyFixture(t *testing.T, limit int, now time.Time) *shops.ShopInventory {
	t.Helper()
	seedBaubleSale(t)
	t.Cleanup(seedSellRoom(t))
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(t.TempDir())
	cfg.Balance.ShopAffixedStockCap = configs.ConfigInt(limit)
	configs.SetConfigForTest(t, cfg)
	orig := shops.ShelfNow
	shops.ShelfNow = func() time.Time { return now }
	t.Cleanup(func() { shops.ShelfNow = orig })
	shops.ClearCache()
	t.Cleanup(shops.ClearCache)
	return shops.RegisterShop("TestZone", 2, 1, shops.ShopInventory{Gold: 1000, StartingGold: 1000, CraftSupport: shops.CraftSupportGeneral})
}

// shelfBuyer is a player standing in room 1 with gold and the strength to
// carry anything on a shelf; no Bartering, so prices are undiscounted.
func shelfBuyer(t *testing.T, userId int, gold int) *UserActor {
	t.Helper()
	room := rooms.LoadRoom(1)
	require.NotNil(t, room)
	u := users.NewTestUser(userId, fmt.Sprintf("buyer%d", userId), fmt.Sprintf("Buyer%d", userId), uint64(userId))
	u.Character.RoomId = 1
	u.Character.Gold = gold
	u.Character.Conditions = conditions.New()
	u.Character.Stats.Strength.ValueAdj = 100
	return &UserActor{User: u, Room: room}
}

// Spec test 9: two Trinket rows at different prices with a held Trinket
// between them. `buy 2.trinket` takes and charges the second LISTED row and
// removes its own AffixedStock index; `buy trinket` then takes the first.
func TestBuy_Shelf_SameNameRowsAreChosenByPosition(t *testing.T) {
	now := stolenTestNow
	si := shelfBuyFixture(t, 12, now)
	first := newBauble(t, "Trinket", "trinket", 12, baubles.StatusFallback)
	held := newBauble(t, "Trinket", "trinket", 13, baubles.StatusFallback)
	second := newBauble(t, "Trinket", "trinket", 14, baubles.StatusFallback)
	si.AffixedStock = []shops.AffixedStockEntry{
		{Item: first, Price: 12, AddedAt: now.Add(-3 * time.Hour)},
		{Item: held, Price: 13, AddedAt: now.Add(-2 * time.Hour), HoldUntil: now.Add(time.Hour)},
		{Item: second, Price: 14, AddedAt: now.Add(-time.Hour)},
	}
	buyer := shelfBuyer(t, 1, 100)

	res := tryPurchaseFromInventory(buyer, "2.trinket", nil, si)
	require.True(t, res.Success, "res=%+v", res)
	assert.Equal(t, 86, buyer.GetCharacter().Gold, "charged the second listed row's own price")
	require.Len(t, si.AffixedStock, 2)
	assert.Equal(t, first.Bauble, si.AffixedStock[0].Item.Bauble, "the first row stays")
	assert.Equal(t, held.Bauble, si.AffixedStock[1].Item.Bauble, "the held entry between them is never counted")

	res = tryPurchaseFromInventory(buyer, "trinket", nil, si)
	require.True(t, res.Success, "res=%+v", res)
	assert.Equal(t, 74, buyer.GetCharacter().Gold, "then the first row, at its own price")
	require.Len(t, si.AffixedStock, 1)
	assert.Equal(t, held.Bauble, si.AffixedStock[0].Item.Bauble)
}

// Spec tests 1 and 2 (buy half): a held entry cannot be bought by its name;
// once its hold ends (the shelf clock moves on) it can.
func TestBuy_Shelf_AHeldEntryIsNotForSaleUntilItsHoldEnds(t *testing.T) {
	now := stolenTestNow
	si := shelfBuyFixture(t, 12, now)
	dice := newBauble(t, "Bone Dice", "dice", 12, baubles.StatusReady)
	si.AffixedStock = []shops.AffixedStockEntry{{Item: dice, Price: 12, AddedAt: now, HoldUntil: now.Add(time.Hour)}}
	buyer := shelfBuyer(t, 1, 100)

	res := tryPurchaseFromInventory(buyer, "bone dice", nil, si)
	assert.Equal(t, BuyReasonNoMatch, res.Reason, "held: no row answers to its name")
	assert.Len(t, si.AffixedStock, 1)

	shops.ShelfNow = func() time.Time { return now.Add(time.Hour) }
	res = tryPurchaseFromInventory(buyer, "bone dice", nil, si)
	require.True(t, res.Success, "the hold is over: res=%+v", res)
	assert.Empty(t, si.AffixedStock)
}

// Spec test 5 (buy half): buy enforces the cap lazily once a hold has ended,
// evicting the entry listed earliest, and saves the trim even when nothing
// is bought.
func TestBuy_Shelf_EnforcesTheCapLazilyAndSavesTheTrim(t *testing.T) {
	now := stolenTestNow
	si := shelfBuyFixture(t, 1, now)
	gear := func(name string) items.Item {
		return items.Item{ItemId: sellTestItemId, Affixed: true, Spec: &items.ItemSpec{ItemId: sellTestItemId, Name: name, Type: items.Weapon, Value: 50}}
	}
	si.AffixedStock = []shops.AffixedStockEntry{
		{Item: gear("keen torc"), Price: 50, AddedAt: now.Add(-80 * time.Hour), HoldUntil: now.Add(-time.Hour)},
		{Item: gear("warding ring"), Price: 50, AddedAt: now.Add(-10 * time.Hour)},
	}
	buyer := shelfBuyer(t, 1, 0)

	res := tryPurchaseFromInventory(buyer, "nothing like this", nil, si)
	assert.Equal(t, BuyReasonNoMatch, res.Reason)
	require.Len(t, si.AffixedStock, 1, "over the cap once the hold ended: trimmed on buy")
	assert.Equal(t, "keen torc", si.AffixedStock[0].Item.GetSpec().Name, "the one listed earliest went, not the one shelved first")

	shops.ClearCache()
	reloaded := shops.GetShopInventory("TestZone", 2, 1)
	require.NotNil(t, reloaded, "the trim was saved though nothing was bought")
	assert.Len(t, reloaded.AffixedStock, 1)
}
```

- [ ] **Step 2: Run to see them fail**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/actions/ -run TestBuy_Shelf_ -count=1`
Expected: FAIL. `SameNameRowsAreChosenByPosition`: `charged the second listed row's own price` (expected 86, actual 88: the name match takes the first `Trinket`). `AHeldEntryIsNotForSaleUntilItsHoldEnds`: `held: no row answers to its name`. `EnforcesTheCapLazilyAndSavesTheTrim`: `trimmed on buy` (2 entries).

- [ ] **Step 3: Implement**

In `internal/actions/buy.go` replace

```go
func tryPurchaseFromInventory(buyer Actor, request string, shopMob *mobs.Mob, shopInv *shops.ShopInventory) BuyResult {
	cfg := shops.PricingConfigFromBalance()

	type invEntry struct {
		entry      *shops.StockEntry
		item       items.Item
		plainName  string
		price      int
		affixedIdx int // index into shopInv.AffixedStock, or -1 for base-ItemId stock
	}
```

with

```go
func tryPurchaseFromInventory(buyer Actor, request string, shopMob *mobs.Mob, shopInv *shops.ShopInventory) BuyResult {
	cfg := shops.PricingConfigFromBalance()

	// The shelf cap is enforced lazily (baubles slice D): a hold that ended
	// since the last trade can push the listed entries over it. The trim is
	// saved before returning unless a purchase below saves the shop anyway.
	now := shops.ShelfNow()
	shopSaved := false
	if shopInv.EnforceAffixedCap(int(configs.GetBalanceConfig().ShopAffixedStockCap), now) > 0 {
		defer func() {
			if !shopSaved {
				if err := shops.SaveShop(shopInv.Zone, shopInv.MobId, shopInv.RoomId); err != nil {
					mudlog.Error("PURCHASE", "msg", "SaveShop failed", "error", err)
				}
			}
		}()
	}

	type invEntry struct {
		entry      *shops.StockEntry
		item       items.Item
		price      int
		affixedIdx int // index into shopInv.AffixedStock, or -1 for base-ItemId stock
	}
```

Replace

```go
		available = append(available, invEntry{
			entry:      entry,
			item:       itm,
			plainName:  spec.Name,
			price:      basePrice,
			affixedIdx: -1,
		})
```

with

```go
		available = append(available, invEntry{
			entry:      entry,
			item:       itm,
			price:      basePrice,
			affixedIdx: -1,
		})
```

Replace

```go
	// Per-instance affixed resale stock (Stage 3): unique bought-back gear,
	// priced at its stored relist price (AffixValue x 1.0), less any barter.
	for i := range shopInv.AffixedStock {
		e := &shopInv.AffixedStock[i]
		spec := e.Item.GetSpec()
		price := e.Price
		if discount := barterDiscount(char, buyer.GetRoom(), barterMaxDiscount); discount > 0 {
			price = shops.ApplyBarterSellDiscount(price, discount)
		}
		available = append(available, invEntry{
			item:       e.Item,
			plainName:  spec.Name,
			price:      price,
			affixedIdx: i,
		})
		itemNames = append(itemNames, spec.Name)
		itemNamesFancy = append(itemNamesFancy, e.Item.DisplayName())
	}

	match, closeMatch := util.FindMatchIn(request, itemNames...)
	if match == "" {
		match = closeMatch
	}
	if match == "" {
```

with

```go
	// The secondhand shelf (Stage 3; baubles slice D): bought-back affixed
	// gear and baubles, in ListedIndexes order, the order list shows, each at
	// its stored relist price less any barter. A held entry (a bauble still
	// hot when shelved) is not offered, so no name, no match and no "Any
	// interest" line can reach it. affixedIdx is the raw AffixedStock index,
	// not the row's position among the listed ones.
	for _, idx := range shopInv.ListedIndexes(now) {
		e := &shopInv.AffixedStock[idx]
		name := e.Item.GetSpec().Name
		price := e.Price
		if discount := barterDiscount(char, buyer.GetRoom(), barterMaxDiscount); discount > 0 {
			price = shops.ApplyBarterSellDiscount(price, discount)
		}
		available = append(available, invEntry{
			item:       e.Item,
			price:      price,
			affixedIdx: idx,
		})
		itemNames = append(itemNames, name)
		itemNamesFancy = append(itemNamesFancy, e.Item.DisplayName())
	}

	// Select by position, not by name (baubles slice D): two rows can share
	// a name (two shelved trinkets), and `buy 2.trinket` must take the second.
	// Counted in available order: stock first, then the shelf in list order.
	// A full match anywhere outranks a close match, as FindMatchIn rules.
	pick, closePick := util.FindMatchIndexIn(request, itemNames...)
	if pick < 0 {
		pick = closePick
	}
	if pick < 0 {
```

Replace

```go
	var matched *invEntry
	for i := range available {
		if available[i].plainName == match {
			matched = &available[i]
			break
		}
	}
	if matched == nil {
		return BuyResult{Reason: BuyReasonNoMatch}
	}
```

with

```go
	matched := &available[pick]
```

Replace (the affixed purchase's save, two-tab indent)

```go
		if err := shops.SaveShop(shopInv.Zone, shopInv.MobId, shopInv.RoomId); err != nil {
			mudlog.Error("PURCHASE", "msg", "SaveShop failed", "error", err)
		}
		buyer.SendText(
```

with

```go
		if err := shops.SaveShop(shopInv.Zone, shopInv.MobId, shopInv.RoomId); err != nil {
			mudlog.Error("PURCHASE", "msg", "SaveShop failed", "error", err)
		}
		shopSaved = true
		buyer.SendText(
```

(the old text ends mid-line at `buyer.SendText(`; the rest of that line is unchanged). And (the stock purchase's save, one-tab indent)

```go
	if err := shops.SaveShop(shopInv.Zone, shopInv.MobId, shopInv.RoomId); err != nil {
		mudlog.Error("PURCHASE", "msg", "SaveShop failed", "error", err)
	}

	tradeInString :=
```

with

```go
	if err := shops.SaveShop(shopInv.Zone, shopInv.MobId, shopInv.RoomId); err != nil {
		mudlog.Error("PURCHASE", "msg", "SaveShop failed", "error", err)
	}
	shopSaved = true

	tradeInString :=
```

Run `gofmt -w internal/actions/buy.go internal/actions/buy_shelf_test.go`, then `grep -n "plainName" internal/actions/buy.go` (expect-zero, standalone). Expected: no output.

- [ ] **Step 4: Run to see them pass, with every buy test**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go vet ./internal/actions/ && go test ./internal/actions/ -count=1 && go test . -count=1 2>&1 | tail -2`
Expected: no vet output, `ok` (`TestBuy_AffixedStockItem` and every existing buy test), root `ok`.

- [ ] **Step 5: Null probes**

1. Replace `for _, idx := range shopInv.ListedIndexes(now) {` with `for idx := range shopInv.AffixedStock {`. Run `go test ./internal/actions/ -run TestBuy_Shelf_AHeldEntryIsNotForSaleUntilItsHoldEnds -count=1`. Expected: FAIL `held: no row answers to its name`. Restore.
2. Replace `if !shopSaved {` with `if !shopSaved && false {` (`shopSaved` stays read). Run `go test ./internal/actions/ -run TestBuy_Shelf_EnforcesTheCapLazilyAndSavesTheTrim -count=1`. Expected: FAIL `the trim was saved though nothing was bought`. Restore.
3. Put name matching back: replace `matched := &available[pick]` with

   ```go
   	for i := range itemNames {
   		if itemNames[i] == itemNames[pick] {
   			pick = i
   			break
   		}
   	}
   	matched := &available[pick]
   ```

   Run `go test ./internal/actions/ -run TestBuy_Shelf_SameNameRowsAreChosenByPosition -count=1`. Expected: FAIL `charged the second listed row's own price` (88, the first `Trinket`). Restore; re-run Step 4.

- [ ] **Step 6: Commit**

```bash
cd /c/tmp/dogmud-baubles-d-impl && git add internal/actions/buy.go internal/actions/buy_shelf_test.go && git commit -F - <<'EOF'
feat(buy): buy off the shelf by position, never a held entry

buy offers the shelf in ListedIndexes order (list's order), skips held
entries entirely, selects by index through util.FindMatchIndexIn so
buy 2.trinket takes the second of two, enforces the listed cap lazily
and saves the trim when no purchase does.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 13: `buy` in the buyer's own view; buyback

**Files:**
- Modify: `internal/actions/buy.go` (imports, the shelf loop's `name`, the buyer line, after the affixed save)
- Modify: `internal/actions/buy_shelf_test.go` (append; add `events` and `strings` imports)
- Modify: `bauble_finder_view_guard_test.go:72`

- [ ] **Step 1: Write the failing tests**

In `internal/actions/buy_shelf_test.go` replace the import lines

```go
	"fmt"
	"testing"
	"time"
```

with

```go
	"fmt"
	"strings"
	"testing"
	"time"
```

and `	"github.com/GoMudEngine/GoMud/internal/configs"` with

```go
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
```

Append:

```go
// Spec test 11 (buy half) and ruling 4: a finder-only bauble on a shelf is
// matched and named in the buyer's own view. Its finder buys it by its own
// name and reads that name on the purchase line; anyone else buys a Trinket,
// and the hidden words match nothing for them.
func TestBuy_Shelf_AFinderOnlyBaubleIsBoughtInTheBuyersOwnView(t *testing.T) {
	now := stolenTestNow
	si := shelfBuyFixture(t, 12, now)
	const finderId, otherId = 1, 2
	rec, err := baubles.Create(baubles.Record{Name: "Painted Wooden Horse", NameSimple: "horse", Tier: baubles.TierAverage,
		Value: 12, WeightLbs: 0.6, Description: "A child's toy horse, its red paint flaking.", Status: baubles.StatusReady,
		Source: baubles.SourceSearch, PlayerKey: true, Moderated: false, FoundByUserId: finderId})
	require.NoError(t, err)
	shelve := func() {
		si.AffixedStock = []shops.AffixedStockEntry{{Item: items.Item{ItemId: items.BaubleItemId, Bauble: rec.Id}, Price: 12, AddedAt: now}}
	}

	other := shelfBuyer(t, otherId, 100)
	shelve()
	res := tryPurchaseFromInventory(other, "painted wooden horse", nil, si)
	assert.Equal(t, BuyReasonNoMatch, res.Reason, "the hidden words match nothing for anyone but the finder")
	events.DrainQueuedMessagesForTest(otherId)
	res = tryPurchaseFromInventory(other, "trinket", nil, si)
	require.True(t, res.Success, "res=%+v", res)
	out := strings.Join(events.DrainQueuedMessagesForTest(otherId), "\n")
	assert.Contains(t, out, "Trinket")
	assert.NotContains(t, out, "Horse")

	finder := shelfBuyer(t, finderId, 100)
	shelve()
	events.DrainQueuedMessagesForTest(finderId)
	res = tryPurchaseFromInventory(finder, "painted wooden horse", nil, si)
	require.True(t, res.Success, "the finder buys it by its own name: res=%+v", res)
	out = strings.Join(events.DrainQueuedMessagesForTest(finderId), "\n")
	assert.Contains(t, out, "You buy the")
	assert.Contains(t, out, "Painted Wooden Horse", "the purchase line shows the finder their own name")
}

// Spec test 12 (buy half): bought back off the shelf, a sold record returns
// to its unsold status (ruling 2), named to ready and generic to fallback,
// and its sale still counts.
func TestBuy_Shelf_ABuybackReturnsTheRecordToItsUnsoldStatus(t *testing.T) {
	now := stolenTestNow
	si := shelfBuyFixture(t, 12, now)
	named := newBauble(t, "Bone Dice", "dice", 12, baubles.StatusReady)
	baubles.Update(named.Bauble, func(r *baubles.Record) { r.Generator = baubles.GeneratorCorpus })
	generic := newBauble(t, "Brass Thimble", "thimble", 11, baubles.StatusFallback)
	since := time.Now().UTC().Add(-time.Second)
	require.True(t, baubles.MarkSold(named.Bauble, 6, 9))
	require.True(t, baubles.MarkSold(generic.Bauble, 6, 9))
	si.AffixedStock = []shops.AffixedStockEntry{{Item: named, Price: 12, AddedAt: now}, {Item: generic, Price: 11, AddedAt: now}}
	buyer := shelfBuyer(t, 1, 100)

	require.True(t, tryPurchaseFromInventory(buyer, "bone dice", nil, si).Success)
	require.True(t, tryPurchaseFromInventory(buyer, "brass thimble", nil, si).Success)
	r1, _ := baubles.Get(named.Bauble)
	r2, _ := baubles.Get(generic.Bauble)
	assert.Equal(t, baubles.StatusReady, r1.Status, "a named record is ready again")
	assert.Equal(t, baubles.StatusFallback, r2.Status, "a generic one is fallback again")
	n, _ := baubles.SalesSince(since)
	assert.Equal(t, 2, n, "a buyback does not erase the sale")
}
```

- [ ] **Step 2: Run to see them fail**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/actions/ -run 'TestBuy_Shelf_AFinderOnly|TestBuy_Shelf_ABuyback' -count=1`
Expected: FAIL: `the finder buys it by its own name` (every buyer sees `Trinket` today) and `a named record is ready again` (status stays `sold`).

- [ ] **Step 3: Implement**

`internal/actions/buy.go` imports: replace `	"github.com/GoMudEngine/GoMud/internal/characters"` with

```go
	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/characters"
```

In the shelf loop replace

```go
		name := e.Item.GetSpec().Name
		price := e.Price
```

with

```go
		name := e.Item.GetSpec().Name
		if e.Item.IsBauble() {
			// The buyer's own view, the one list showed them (owner ruling
			// 4): a finder-only bauble is Trinket to everyone but its finder,
			// so only its finder's own name holds its hidden words and nobody
			// else can confirm them by typing. A mob buyer (user 0) gets the
			// generic view.
			name = e.Item.NameFor(buyer.GetUserId())
		}
		price := e.Price
```

Replace

```go
		shopSaved = true
		buyer.SendText(messaging.CategoryLoot, fmt.Sprintf(`You buy the <ansi fg="itemname">%s</ansi>.`, bought.DisplayName()))
```

with

```go
		shopSaved = true
		if bought.IsBauble() {
			// Back in a pack: its record returns to its unsold status
			// (baubles slice D, ruling 2).
			baubles.MarkBought(bought.Bauble, buyer.GetUserId())
		}
		buyer.SendText(messaging.CategoryLoot, fmt.Sprintf(`You buy the <ansi fg="itemname">%s</ansi>.`, bought.DisplayNameFor(buyer.GetUserId())))
```

(The room line below keeps `bought.DisplayName()`.)

- [ ] **Step 4: Run the package, then the guard (expect the guard to fail)**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./internal/actions/ -count=1 && go test . -run TestFinderViewReachesOnlyItsReader -count=1`
Expected: `ok` for `internal/actions`; root FAILS: `internal/actions/buy.go|tryPurchaseFromInventory: makes 2 finder-view reference(s), and is not in finderViewSites`.

- [ ] **Step 5: Register the site**

In `bauble_finder_view_guard_test.go` replace

```go
	"internal/actions/search_bauble.go|BaubleDelivery.deliver":
```

with

```go
	"internal/actions/buy.go|tryPurchaseFromInventory":         {2, "the buyer's own view of a shelf bauble: a match key compared with what the buyer typed, and the buyer's own purchase line (buyer.SendText)"},
	"internal/actions/search_bauble.go|BaubleDelivery.deliver":
```

(the old text ends at the key; the rest of that line is unchanged). Run `gofmt -w bauble_finder_view_guard_test.go internal/actions/buy.go`, then `go test . -count=1 2>&1 | tail -2`. Expected: `ok`.

- [ ] **Step 6: Null probes**

1. Replace `name = e.Item.NameFor(buyer.GetUserId())` with `name = e.Item.GetSpec().Name`. Run the Step 2 command (Expected: FAIL `the finder buys it by its own name`) and `go test . -run TestFinderViewReachesOnlyItsReader -count=1` (Expected: FAIL `makes 1 finder-view reference(s), finderViewSites says 2`). Restore.
2. Replace the `baubles.MarkBought(bought.Bauble, buyer.GetUserId())` call with `_ = baubles.StatusSold` (the import stays used). Run `go test ./internal/actions/ -run TestBuy_Shelf_ABuyback -count=1`. Expected: FAIL `a named record is ready again`. Restore.
3. Put `bought.DisplayName()` back in the buyer's line. Run `go test ./internal/actions/ -run TestBuy_Shelf_AFinderOnly -count=1`. Expected: FAIL `the purchase line shows the finder their own name`. Restore; re-run Steps 4 and 5.

- [ ] **Step 7: Commit**

```bash
cd /c/tmp/dogmud-baubles-d-impl && git add internal/actions/buy.go internal/actions/buy_shelf_test.go bauble_finder_view_guard_test.go && git commit -F - <<'EOF'
feat(buy): shelf baubles in the buyer's own view; a buyback unsells

buy matches a shelf bauble by the name list showed that buyer
(NameFor, ruling 4) and names it on the buyer's own purchase line
(DisplayNameFor); the guard gains the site. A bauble bought back
calls baubles.MarkBought.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 13b: The AI companion's `browse` shows the shelf

**Files:**
- Modify: `modules/aicompanion/economy.go:60-65, 75-122, 125-150, 152-166`
- Create: `modules/aicompanion/economy_shelf_test.go`

Controller decision (plan review, 2026-09-30): `browseShops` mirrors `list` ("as a player's `list` would show it"), so it shows the shelf too, and it must not use the finder view, because these names reach the model. The companion already names every item it tells the model about with `items.Item.ModelName()` (`perception.go`, `scene.go`, `actions.go`), which shows the carrier (`Curious Trinket`) for any bauble whose text a player's key wrote, finder-only or moderated. The plan uses that existing rule rather than the generic `Name()`: `Name()` would give a finder-only bauble's generic `Trinket` but a moderated player-key bauble's real name, which the module's rule (spec S3 of slice H) keeps from the model. Shelf rows are shown in `describeListing` and never written to shop memory: `rememberShop` keys wares by `ItemId`, which a shelf row shares with regular stock (and every bauble is item 900), so remembering them would overwrite real prices.

- [ ] **Step 1: Write the failing test**

Create `modules/aicompanion/economy_shelf_test.go`:

```go
package aicompanion

import (
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
)

// browse mirrors list (baubles slice D): the companion sees a shop's listed
// secondhand shelf after its stock, in shelf order, never a held entry. The
// names reach the model, so they are ModelName: a bauble whose text a
// player's key wrote reads as its carrier, finder-only or moderated. Shelf
// rows are never remembered: each is one of a kind, and they share ItemIds
// with regular stock (every bauble is item 900).
func TestBrowseShopsShowsTheShelfInTheModelsView(t *testing.T) {
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		items.BaubleItemId: {ItemId: items.BaubleItemId, Name: `Curious Trinket`, NameSimple: `trinket`, Type: items.Object, Subtype: items.Mundane, Value: 1},
		96001:              {ItemId: 96001, Name: `iron sword`, Type: items.Weapon, Value: 100},
	}))
	baubles.SetDirForTest(t.TempDir())
	t.Cleanup(func() { items.SetBaubleResolver(nil) })
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(t.TempDir())
	configs.SetConfigForTest(t, cfg)

	room := &rooms.Room{RoomId: 9601, Zone: `TestZone`, Title: `Shop`}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{9601: room},
		map[string]*rooms.ZoneConfig{`TestZone`: {Name: `TestZone`, RoomId: 9601, RoomIds: map[int]struct{}{9601: {}}}}))
	keeper := &mobs.Mob{MobId: 961, InstanceId: 9602, HomeRoomId: 9601, Zone: `TestZone`,
		Character: characters.Character{Name: `Keeper`, RoomId: 9601, Conditions: conditions.New(),
			Shop: characters.Shop{{ItemId: 96001, Price: 100}}}}
	keeper.Character.HealthMax.Value, keeper.Character.Health = 100, 100
	mobs.SetInstanceForTest(9602, keeper)
	t.Cleanup(func() { mobs.SetInstanceForTest(9602, nil) })
	room.AddMob(9602)

	shops.ClearCache()
	t.Cleanup(shops.ClearCache)
	si := shops.RegisterShop(`TestZone`, 961, 9601, shops.ShopInventory{Gold: 100, CraftSupport: shops.CraftSupportGeneral,
		Stock: []shops.StockEntry{{ItemId: 96001, RestockQty: 1, MaxStock: 2, Current: 1}}})

	bauble := func(name string, moderated bool) items.Item {
		rec, err := baubles.Create(baubles.Record{Name: name, NameSimple: `horse`, Tier: baubles.TierAverage, Value: 12,
			WeightLbs: 0.5, Description: `A toy horse.`, Status: baubles.StatusReady, PlayerKey: true, Moderated: moderated, FoundByUserId: 7})
		if err != nil {
			t.Fatal(err)
		}
		it := items.New(items.BaubleItemId)
		it.Bauble = rec.Id
		return it
	}
	now := time.Now()
	si.AffixedStock = []shops.AffixedStockEntry{
		{Item: bauble(`Painted Wooden Horse`, false), Price: 12, AddedAt: now},                           // finder-only
		{Item: bauble(`Carved Oak Horse`, true), Price: 14, AddedAt: now, HoldUntil: now.Add(time.Hour)}, // held
		{Item: bauble(`Glass Horse`, true), Price: 15, AddedAt: now},                                     // moderated player-key text
	}

	listings := browseShops(room)
	if len(listings) != 1 {
		t.Fatalf("one open merchant: %+v", listings)
	}
	wares := listings[0].Wares
	var shelf []ware
	for _, w := range wares {
		if w.Secondhand {
			shelf = append(shelf, w)
		}
	}
	if len(shelf) != 2 {
		t.Fatalf("the two listed shelf rows, not the held one: %+v", shelf)
	}
	for _, w := range shelf {
		if w.Name != `Curious Trinket` {
			t.Errorf("a bauble a player's key wrote reads as its carrier to the model: %+v", w)
		}
	}
	if shelf[0].Price != 12 || shelf[1].Price != 15 {
		t.Errorf("shelf order, each at its own price: %+v", shelf)
	}
	if !wares[len(wares)-1].Secondhand || wares[0].Secondhand {
		t.Errorf("the stock first, then the shelf, as list shows them: %+v", wares)
	}

	text := describeListing(listings[0], nil)
	for _, hidden := range []string{`Painted`, `Glass`, `Carved`} {
		if strings.Contains(text, hidden) {
			t.Errorf("the model never reads a player's text (%s): %q", hidden, text)
		}
	}
	if !strings.Contains(text, `Curious Trinket for 12 gold (secondhand)`) {
		t.Errorf("listing wording: %q", text)
	}

	mind := &Mind{}
	mind.rememberShop(listings[0], 9601, now.Unix())
	if w := mind.Shops[961].Wares[items.BaubleItemId]; w != nil {
		t.Errorf("shelf rows are not remembered: %+v", w)
	}
	if mind.Shops[961].Wares[96001] == nil {
		t.Error("the stock is remembered as before")
	}
}
```

- [ ] **Step 2: Run to see it fail**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go test ./modules/aicompanion/ -run TestBrowseShopsShowsTheShelfInTheModelsView -count=1`
Expected: FAIL, build error `w.Secondhand undefined (type ware has no field or method Secondhand)`.

- [ ] **Step 3: Implement**

In `modules/aicompanion/economy.go` replace

```go
type ware struct {
	ItemId int
	Name   string
	Price  int
	Qty    int
}
```

with

```go
type ware struct {
	ItemId     int
	Name       string
	Price      int
	Qty        int
	Secondhand bool // a one-of-a-kind shelf row (baubles slice D): shown, never remembered
}
```

Replace

```go
		if inv := shops.GetShopInventory(m.Zone, int(m.MobId), m.HomeRoomId); inv != nil {
```

with

```go
		inv := shops.GetShopInventory(m.Zone, int(m.MobId), m.HomeRoomId)
		if inv != nil {
```

Replace

```go
		sort.Slice(l.Wares, func(i, j int) bool { return l.Wares[i].Name < l.Wares[j].Name })
		out = append(out, l)
```

with

```go
		sort.Slice(l.Wares, func(i, j int) bool { return l.Wares[i].Name < l.Wares[j].Name })
		// The secondhand shelf, after the stock as list shows it (baubles
		// slice D), in shelf order, never a held entry. ModelName, never the
		// finder's view: these names reach the model, so a bauble whose text
		// a player's key wrote reads as its carrier. Read only: the lazy cap
		// trim is left to list and buy.
		if inv != nil {
			for _, idx := range inv.ListedIndexes(shops.ShelfNow()) {
				e := &inv.AffixedStock[idx]
				l.Wares = append(l.Wares, ware{ItemId: e.Item.ItemId, Name: e.Item.ModelName(), Price: e.Price, Qty: 1, Secondhand: true})
			}
		}
		out = append(out, l)
```

In `rememberShop` replace

```go
	for _, w := range l.Wares {
		wr := &WareRecord{Name: w.Name, Price: w.Price, Qty: w.Qty, SeenUnix: nowUnix}
```

with

```go
	for _, w := range l.Wares {
		if w.Secondhand {
			continue // one of a kind, and its ItemId is shared: never a price to remember
		}
		wr := &WareRecord{Name: w.Name, Price: w.Price, Qty: w.Qty, SeenUnix: nowUnix}
```

In `describeListing` replace

```go
	for _, w := range l.Wares {
		ref := ``
```

with

```go
	for _, w := range l.Wares {
		if w.Secondhand {
			// No ref: refs are keyed by ItemId, which a shelf row shares.
			parts = append(parts, fmt.Sprintf(`%s for %d gold (secondhand)`, w.Name, w.Price))
			continue
		}
		ref := ``
```

Run `gofmt -w modules/aicompanion/economy.go modules/aicompanion/economy_shelf_test.go`.

- [ ] **Step 4: Run to see it pass**

Run: `cd /c/tmp/dogmud-baubles-d-impl && go vet ./modules/aicompanion/ && go test ./modules/aicompanion/ -count=1 2>&1 | tail -3 && go test . -count=1 2>&1 | tail -2`
Expected: no vet output, `ok` (`TestShopMemory` too), root `ok`.

- [ ] **Step 5: Null probes**

1. Use `e.Item.Name()` instead of `e.Item.ModelName()`. Run the Step 2 command. Expected: FAIL `a bauble a player's key wrote reads as its carrier to the model` for the Glass Horse (a finder-only one would read `Trinket`). Restore.
2. Delete the `if w.Secondhand { continue }` in `rememberShop`. Expected: FAIL `shelf rows are not remembered`. Restore.
3. Replace `for _, idx := range inv.ListedIndexes(shops.ShelfNow()) {` with `for idx := range inv.AffixedStock {`. Expected: FAIL `the two listed shelf rows, not the held one`. Restore; re-run Step 4.

- [ ] **Step 6: Commit**

```bash
cd /c/tmp/dogmud-baubles-d-impl && git add modules/aicompanion/economy.go modules/aicompanion/economy_shelf_test.go && git commit -F - <<'EOF'
feat(aicompanion): browse shows a shop's secondhand shelf

browseShops mirrors list: the listed shelf rows follow the stock, in
shelf order, named with ModelName so no player-written bauble text
reaches the model. They are described but never remembered, since a
shelf row shares its ItemId with regular stock.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 15: Code review of the whole branch

**Files:** none unless the review finds something.

- [ ] **Step 1: Dispatch a reviewer (opus)** with `superpowers:requesting-code-review`: the spec, this plan, and `git diff $(cat /c/tmp/dogmud-baubles-d-impl.base)..HEAD`. Checklist:
  - Every one of the eight rulings is implemented where the coverage map says.
  - No path shows a held entry: `list` (rows), `buy` (names, fancy names, the "Any interest" line), `offer`/`appraise` (they read offers only).
  - Every `SaveShop` after a shelf mutation: the sale (existing save), `list`'s trim, `buy`'s trim (deferred) and purchase.
  - Locking (spec section 8): no new lock; `MarkBought` and `ShelfHoldUntil` take `cat.mu` briefly and are called with no shop or cache lock held.
  - `bestBaubleMerchant` still moves past a backroom refusal (it is not `Broke`).
  - The Bartering loop stays closed for baubles (cheap ones never shelved).
  - No raw magic number: every cap reads `ShopAffixedStockCap`, every heat reads `HeatDuration()`, the cheap bound reads `TierCheap.Range()`.
  - The guard rows' counts and reasons are true.
  - A fence's backroom refusal and an honest shop's are the right lines for the right shop, and both are interest refusals.
  - The companion's `browse` never names a bauble in any view but `ModelName`, never shows a held entry, and never writes a shelf row into shop memory.
- [ ] **Step 2: Fix findings** with a failing test first for behaviour; commit as `fix(<pkg>): <what> (review)`. Re-run `go test ./internal/shops/ ./internal/baubles/ ./internal/actions/ ./internal/usercommands/ ./internal/economy/health/ ./internal/util/ ./internal/configs/ ./modules/auctions/ . -count=1`.

---

### Task 16: Docs

**Files:**
- Modify: `internal/shops/context.md`, `internal/baubles/context.md`, `internal/economy/health/context.md`, `internal/actions/context.md`, `internal/usercommands/context.md`, `internal/util/context.md`, `modules/auctions/context.md`, `modules/aicompanion/context.md`
- Modify: `internal/baubles/sweep.go` (comment only), `docs/baubles/implementation-plan.md`
- Modify: `_datafiles/world/dogmud/templates/help/sell.template`, `buy.template`, `list.template`, `_datafiles/world/dogmud/templates/admincommands/help/command.bauble.template`
- Modify: `docs/PATCH_NOTES.md`

Load `dogmud-player-copy` first (80-column visible width, no raw numbers for durations, ESL-clear). No em or en dashes anywhere. These files are CRLF in the working tree (P39): use the Edit tool only, never a Python read-modify-write.

- [ ] **Step 1: `internal/shops/context.md`**

Replace the line `  constants; `StockEvent` depletion/refill event type.` (the end of the Core Files `shopinventory.go` bullet) with itself followed by a new bullet:

```markdown
  constants; `StockEvent` depletion/refill event type.
- **shelf.go** (baubles slice D): the secondhand shelf over `AffixedStock`.
  `ShelfNow` (the one clock list, buy and the sale read), `(AffixedStockEntry)
  Held(now)` and `ListedAt()`, `(*ShopInventory) HeldCount(now)`,
  `ListedIndexes(now)` (THE shelf order), `EnforceAffixedCap(limit, now)`
  (earliest `ListedAt` first, held never evicted, lazy: on add, `list`,
  `buy`) and `RestoreAffixedStock(idx, e)` (buy's rollback).
```

At the end of the file (after the `## WalkItems (walk_items.go)` section) append:

```markdown

## The secondhand shelf (shelf.go)

`AffixedStock` holds unique items a shop bought from players and resells:
affix-scaled gear and, since baubles slice D, average and rare baubles a
player sold to a living shop. `AffixedStockEntry` carries `AddedRound`
(kept for the record), `AddedAt` (wall clock when shelved) and `HoldUntil`
(zero: listed at once). Both times are `omitempty`, so a shop file written
before them loads listed and earliest in eviction order.

An entry is held while `now < HoldUntil` (a bauble still hot when shelved,
`baubles.ShelfHoldUntil`): `list` does not show it and `buy` does not offer
it. `AddAffixedStock(item, price, limit, holdUntil, now)` appends and trims
the listed entries to `Balance.ShopAffixedStockCap` (12, SHOP ECONOMY);
held entries never count against that cap and are never evicted, and a
shop refuses another hot bauble once it holds that many
(`internal/actions` `baubleOfferFor`). Every mutation runs in a command or a
sale under the mud lock; a caller that changes a living shop saves it.
```

In the Pricing Config Knobs section, add the cap and correct two stale bullets (a flaw found here: `BarterMaxDiscount` and `BarterMaxBonus` are read by `buy.go:533` and `sell.go:346` since the lighting work, so they are live, not dead). Replace

```markdown
value and the Go default are identical, so this fallback never actually
triggers in production today.
```

with

```markdown
value and the Go default are identical, so this fallback never actually
triggers in production today. The same holds for every live knob below.
```

Replace

```markdown
  `EvaluateBuyRules` and by `modules/auctions/npc_buyers.go`. Fraction of a
  shop's gold pool held back before it will buy from a seller.
```

with

```markdown
  `EvaluateBuyRules` and by `modules/auctions/npc_buyers.go`. Fraction of a
  shop's gold pool held back before it will buy from a seller.
- **`BarterMaxDiscount`**: shipped `0.15`, Go default `0.15`. Read by
  `internal/actions` `tryPurchaseFromInventory` (through `barterDiscount`,
  which also folds in sight): the buy-side cap at Bartering 50.
- **`BarterMaxBonus`**: shipped `0.15`, Go default `0.15`. Read by
  `sellOneToMerchant`: the sell-side cap at Bartering 50.
- **`ShopAffixedStockCap`**: shipped `12`, Go default `12`
  (`config.balance.shops.go`). Not a price: the most entries the secondhand
  shelf lists, and the most hot baubles a shop holds out of sight (see
  "The secondhand shelf" below).
```

and delete the two stale Dead bullets, from the line `- **`BarterMaxDiscount`**: shipped `0.15`, Go default `0.15`. The buy-side` through the line `  function) and never reads this field.` (the eleven lines after the `ShopMaterialReserve` bullet), leaving `ShopMaterialReserve` as the only Dead knob. In Gotchas, replace

```markdown
- **`BarterMaxDiscount` and `BarterMaxBonus` look tunable and are not.**
  Both the buy-side and sell-side barter caps are hard-coded `0.15` literals
  in `internal/actions/buy.go` and `internal/actions/sell.go`; editing these
  two `config.yaml` knobs changes nothing at runtime. Same dead pattern as
  the salvage knobs elsewhere in the codebase: check for a consumer before
  trusting a knob's comment.
```

with

```markdown
- **Check for a consumer before trusting a knob's comment.**
  `ShopMaterialReserve` is declared, shipped and read by nothing. (The
  barter caps `BarterMaxDiscount` and `BarterMaxBonus` were once hard-coded
  literals too; `buy.go` and `sell.go` read the knobs now.)
```

- [ ] **Step 2: `internal/baubles/context.md`**

Replace the line

```markdown
- **sales.go**: `MarkSold`, `SalesSince`.
```

with

```markdown
- **sales.go**: `MarkSold`, `MarkBought` (a buyback off a shop's shelf
  returns a sold record to `unsoldStatus`; any other status is left, so a
  retired one stays retired; the sale fields are kept), `SalesSince`
  (counts by `SoldAt` alone, so a buyback does not erase a sale).
```

Replace

```markdown
  goods; a theft clears it).
- **admin.go**: `CatalogStats`, `Retire`, `Restore`, `Edit` (hand edits,
  checked like a model's answer), `ApplyRegenerated`, the prompt-preview
  seam (`SetPromptPreview`, `PreviewPrompt`) and `LooksLikeId`.
```

with

```markdown
  goods; a theft clears it). `ShelfHoldUntil(itm, now)` (slice D) is
  `StolenAt + HeatDuration()` while the record is `Hot` anywhere, else zero:
  how long a shelved bauble is held out of sight.
- **admin.go**: `CatalogStats`, `Retire`, `Restore`, `Edit` (hand edits,
  checked like a model's answer), `ApplyRegenerated`, the prompt-preview
  seam (`SetPromptPreview`, `PreviewPrompt`) and `LooksLikeId`.
  `Record.unsoldStatus` (ready when `Generator.Named()`, else fallback) is
  `Restore`'s rule, shared with `MarkBought`.
```

In the API listing replace

```go
func MarkSold(id string, gold int, sellerUserId int) bool
```

with

```go
func MarkSold(id string, gold int, sellerUserId int) bool
func MarkBought(id string, buyerUserId int) bool
func ShelfHoldUntil(itm items.Item, now time.Time) time.Time
```

Two places still say a sold record reaches a merchant again only after a crash. Replace (`:360-361`)

```markdown
- A record lives as long as something points at it. The sweep is the only
  pruner and it fails closed. A sold record held again (a crash rolled the
  seller back) is seen and kept, and its sale is left as it was: every
```

with

```markdown
- A record lives as long as something points at it. The sweep is the only
  pruner and it fails closed. A record on a shop's shelf is sold and still
  referenced (`WalkItems` visits `AffixedStock`), so it is kept; a buyback
  makes it unsold (`MarkBought`). A sold record held again (a crash rolled
  the seller back) is seen and kept, and its sale is left as it was: every
```

and (`:455-456`)

```markdown
- **Selling lives in `internal/actions/sell_bauble.go`**, not here. Every
  record is sellable, a sold one included: it only reaches a merchant again
  if a crash lost the seller's save after the sale was recorded.
```

with

```markdown
- **Selling lives in `internal/actions/sell_bauble.go`**, not here. Every
  record is sellable, a sold one included. A record reaches a merchant
  again after a buyback off a shop's shelf (`MarkBought` makes it unsold
  first); one still marked sold does so only if a crash lost the seller's
  save after the sale was recorded.
```

- [ ] **Step 3: `internal/economy/health/context.md`**

Replace

```markdown
- **`PerCraftSupportScores`** — whether each crafting trade can actually buy
  its inputs. A fence's shop may carry no craft_support (it buys no
  ordinary loot) and rolls into the "(uncategorized)" key.
```

with

```markdown
- **`PerCraftSupportScores`**: whether each crafting trade can actually buy
  its inputs, grouped by `ShopSnapshot.Type()`: "fence" for a fence's shop
  (`ShopSnapshot.Fence`, set in `captureShops` from the mob template's
  `IsFence`), else its craft_support. A snapshot saved before `Fence`
  existed decodes with it false and groups by craft_support; an empty one
  rolls into the "(uncategorized)" key. The per-shop rows and the admin
  page group the same way.
```

(This replaces the old bullet's em dash too.)

- [ ] **Step 4: `internal/actions/context.md`**

Replace the two lines

```markdown
- Never stocked, never resold: the item leaves the world, the record is
  marked sold (`baubles.MarkSold`), a living-economy shop is saved.
```

with

```markdown
- The shelf (slice D): a player's sale of an average or rare, non-retired
  bauble (`baubleShelvable`) to a living-economy shop puts it on the shop's
  secondhand shelf (`AffixedStock`) at its catalog value, held out of sight
  until `baubles.ShelfHoldUntil` while it is hot. A mob's sale, a legacy
  merchant, a cheap or a retired bauble still leaves the world. Every sale
  marks the record sold (`baubles.MarkSold`) and saves a living shop. A shop
  holding `ShopAffixedStockCap` hot baubles refuses another shelvable hot
  one (`baubleSayBackroomFull`, an interest refusal, so the next merchant is
  tried). `buy` (`tryPurchaseFromInventory`) offers the shelf in
  `ListedIndexes` order, never a held entry, selects by position
  (`util.FindMatchIndexIn`, so `buy 2.trinket` takes the second), names a
  bauble in the buyer's own view (`NameFor`, `DisplayNameFor` on the
  buyer's line), trims the listed cap lazily, rolls back with
  `RestoreAffixedStock`, and calls `baubles.MarkBought` on a buyback.
```

- [ ] **Step 5: `internal/usercommands/context.md`**

Replace

```markdown
- **Trading**: `buy`, `sell`, `list`, `offer`, `appraise` - Commerce mechanics
```

with

```markdown
- **Trading**: `buy`, `sell`, `list`, `offer`, `appraise` - Commerce mechanics.
  `list` shows a living shop's secondhand shelf as a second table,
  "Secondhand goods" (`renderShelfListing` over `buildShelfRows`: the
  listed entries in shelf order, each in the lister's own view, a
  finder-view site in the root guard), trims the listed cap lazily and
  saves; "nothing to sell" fires only when both tables are empty.
```

- [ ] **Step 6: `internal/util/context.md`**

Replace

```go
func FindMatchIn(searchName string, items ...string) (match, closeMatch string)
```

with

```go
func FindMatchIn(searchName string, items ...string) (match, closeMatch string)
func FindMatchIndexIn(searchName string, items ...string) (match, closeMatch int) // -1 for none
```

and after the paragraph that begins "`FindMatchIn` returns **two** results" add a paragraph: "`FindMatchIndexIn` is the same algorithm by position (`FindMatchIn` wraps it); use it when two entries can share a name and the caller must know which one matched, as `buy` does for shelf rows."

- [ ] **Step 7: `modules/auctions/context.md`**

Replace

```markdown
`NpcWallet` is a real balance: `CanAfford`/`Spend`/`Refund`/`Regen`. A buyer
that has been spending cannot keep bidding, and outbid gold is refunded.
```

with

```markdown
`NpcWallet` is a real balance: `CanAfford`/`Spend`/`Refund`/`Regen`. A buyer
that has been spending cannot keep bidding, and outbid gold is refunded.

A lot the shopkeeper buyer wins goes onto its bound shop's secondhand shelf
(`shops.AddAffixedStock` with `baubles.ShelfHoldUntil`, which is zero today:
the shopkeeper never wins a bauble, since `EvaluateBuyRules` refuses the
carrier), capped by `Balance.ShopAffixedStockCap`.
```

- [ ] **Step 7b: `modules/aicompanion/context.md`**

Replace

```markdown
- **economy.go**: `browse` (priced as `list` prices), shop and price
  memory, the money rules for buying.
```

with

```markdown
- **economy.go**: `browse` (priced as `list` prices), shop and price
  memory, the money rules for buying. `browseShops` also returns a shop's
  listed secondhand shelf (baubles slice D), after its stock and in shelf
  order, as `ware.Secondhand` rows named with `ModelName` (the carrier for
  a bauble a player's key wrote; never the finder's view). They are shown
  in `describeListing` and never remembered: each is one of a kind and
  shares its `ItemId` with regular stock.
```

- [ ] **Step 7c: `internal/baubles/sweep.go` comment**

Replace

```go
// is left as sold: every record is sellable already (sales.go), a save file
// on disk can lag a sale by one autosave, and rewriting the sale on that
// evidence would erase real ones.
```

with

```go
// is left as sold: every record is sellable already (sales.go), a save file
// on disk can lag a sale by one autosave, and rewriting the sale on that
// evidence would erase real ones. A bauble on a shop's secondhand shelf
// (baubles slice D) is sold and still referenced (the shops source walks
// AffixedStock), so it stays; a buyback makes it unsold (MarkBought).
```

- [ ] **Step 7d: `docs/baubles/implementation-plan.md`**

Replace (`:161-163`)

```markdown
  spread: no scarcity curve, no barter bonus). Merchant gold and the
  living-economy reserve are respected. Never stocked or resold; the record
  is marked sold. Unknown records are refused with a spoken line.
```

with

```markdown
  spread: no scarcity curve, no barter bonus). Merchant gold and the
  living-economy reserve are respected. The record is marked sold. Unknown
  records are refused with a spoken line. (Since slice D, Phase 6f, an
  average or rare bauble a player sells to a living shop is shelved for
  resale; the rest are destroyed.)
```

Replace (`:179-181`)

```markdown
  `bauble spawn average`, `get bauble`, `appraise bauble` and
  `sell bauble` at a general store pay 5 to 8 gold and leave nothing on the
  shelf.
```

with

```markdown
  `bauble spawn average`, `get bauble`, `appraise bauble` and
  `sell bauble` at a general store pay 5 to 8 gold and (before slice D)
  left nothing on the shelf. Since Phase 6f the store shelves it.
```

Replace (`:877`)

```markdown
  Resale of bought baubles is slice D (the owner's), not here.
```

with

```markdown
  Resale of bought baubles is slice D: Phase 6f.
```

Replace (`:967`)

```markdown
6. Sold baubles destroyed (recommended) or resold as curios.
```

with

```markdown
6. Sold baubles destroyed (recommended) or resold as curios. Answered by
   slice D (Phase 6f): average and rare ones a player sells to a living
   shop are resold; the rest are destroyed.
```

And before `### Phase 7: Optional` insert:

```markdown
### Phase 6f: Shelf resale (slice D) (written)

Design: `docs/superpowers/specs/2026-09-30-baubles-shelf-resale-design.md`.
Plan: `docs/superpowers/plans/2026-09-30-baubles-shelf-resale.md`.

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
```


`sell.template`: after the line `<ansi fg="command">sell all bauble</ansi> sells them all.` insert

```
A shop puts a trinket of some worth on its shelf of secondhand
goods, where anyone can buy it. A cheap one it throws away.
```

and after `three days are out, anyone who buys trinkets will take it.` insert

```

A stolen trinket a shop buys while it is still hot waits in its
back room until it cools, and only then goes on the shelf. A
back room holds only so much: a shop with a full one turns away
hot goods until some of what it holds has cooled.
```

`list.template`: after `  This would list whatever the merchant is carrying.` insert

```

A shop that buys from players also lists, below its own goods, a
table of <ansi fg="cyan">Secondhand goods</ansi>: gear and trinkets it
bought and will sell again. Each is one of a kind.
```

`buy.template`: after `run out of gold or cannot carry any more.` insert

```

Secondhand goods are bought by the name <ansi fg="command">list</ansi> shows you.
When two share a name, <ansi fg="command">buy 2.trinket</ansi> buys the second one
listed. Mind the dot: <ansi fg="command">buy 2 trinket</ansi>, with a space,
buys two trinkets. The word <ansi fg="command">bauble</ansi>, which
<ansi fg="command">sell</ansi> accepts for any trinket, does not work here.
```

(`Buy` reads a leading number followed by a space as a quantity, `buy.go:296-303`; `2.trinket` has no space, so it reaches the matcher as "the second trinket".)

`_datafiles/world/dogmud/templates/admincommands/help/command.bauble.template`: replace

```
    ShopBuyRatio; other shops refuse. Sold baubles leave the world.
```

with

```
    ShopBuyRatio; other shops refuse. An average or rare bauble a
    player sells to a living shop goes on its secondhand shelf (held
    out of sight while hot); the rest leave the world. Buying one back
    makes its record unsold again; bauble show keeps the last sale.
```

- [ ] **Step 9: `docs/PATCH_NOTES.md`**

After `# DOGMud Patch Notes` and its blank line, insert (use the merge day's date if it is not 2026-10-01):

```markdown
## 2026-10-01: Secondhand shelves

- A general store or jeweller that buys a trinket of some worth from you
  now puts it on a shelf of secondhand goods instead of throwing it away.
  Cheap trinkets are still thrown away.
- `list` shows each shop's secondhand shelf as its own table, below the
  shop's usual goods. Gear you sold to a shop shows there too, so you no
  longer have to guess its name to buy it back.
- Buy from the shelf by the name `list` shows you. When two things share a
  name, `buy 2.trinket` buys the second one listed.
- Fences keep a shelf too: what a fence buys from you goes on its shelf
  like any shop's, stolen goods included once they have cooled.
- A stolen trinket a shop buys while it is still hot waits out of sight in
  its back room until it has cooled, and only then goes on the shelf. A
  shop with a full back room turns away more hot goods for a while.
- Mind the dot when buying: `buy 2.trinket` buys the second trinket
  listed, while `buy 2 trinket` buys two trinkets.
- A shop keeps up to twelve secondhand pieces on show. When it has too
  many, the one that has been on show longest goes.

```

- [ ] **Step 10: Check**

```bash
cd /c/tmp/dogmud-baubles-d-impl && python tools/context_md_audit.py 2>&1 | grep -E "^packages|^total|^internal/(shops|baubles|economy|actions|usercommands|util)|^modules/(auctions|aicompanion)"
```
Expected: `packages checked:` (the Task 0 count), `packages with phantom symbols:   14`, `total phantom symbols:           27` (Task 0 baseline), and no line for any touched package.

Run the dash check standalone (expect-zero; old lines elsewhere in these files are not this branch's to change, so only added lines are checked):

```bash
cd /c/tmp/dogmud-baubles-d-impl && git diff $(cat /c/tmp/dogmud-baubles-d-impl.base) -- . | grep -nP "^\+.*(\x{2014}|\x{2013})"
```
Expected: no output.

Run: `go test ./internal/devtools/ ./internal/templates/ . -count=1 2>&1 | tail -3`. Expected: `ok`.

- [ ] **Step 11: Commit**

```bash
cd /c/tmp/dogmud-baubles-d-impl && gofmt -l internal/baubles/sweep.go && go build ./internal/baubles/ && git add internal/shops/context.md internal/baubles/context.md internal/economy/health/context.md internal/actions/context.md internal/usercommands/context.md internal/util/context.md modules/auctions/context.md modules/aicompanion/context.md internal/baubles/sweep.go docs/baubles/implementation-plan.md _datafiles/world/dogmud/templates/help/sell.template _datafiles/world/dogmud/templates/help/buy.template _datafiles/world/dogmud/templates/help/list.template _datafiles/world/dogmud/templates/admincommands/help/command.bauble.template docs/PATCH_NOTES.md && git commit -F - <<'EOF'
docs(baubles): the secondhand shelf in context, help and patch notes

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 17: Adversarial playtest (controller)

**Files:** none committed, unless the triage produces fixes.

`list`, `buy` and `sell` output changes, so the branch ends with an in-game adversarial review.

- [ ] **Step 1: Load `dogmud-playtesting`** and follow it: the harness location (`../gomud-playtest-harness`, P44), local runs need `--checkout` and an `ephemeral:` goals file, never kill the owner's server (kill by PID only).

- [ ] **Step 2: A throwaway checkout with finds switched on**

```bash
ls /c/tmp/dogmud-baubles-d-playtest 2>/dev/null | head -1
```
Expected: no output. Then:

```bash
git -C /c/tmp/dogmud-baubles-d-impl worktree add --detach /c/tmp/dogmud-baubles-d-playtest HEAD
```

In `C:/tmp/dogmud-baubles-d-playtest/_datafiles/config.yaml` (never committed), with the Edit tool: `BaublesEnabled: true`, `BaubleSearchChancePct: 100`, every non-zero value under `BaubleBiomeChancePct` to `100`, `BaubleRollsPerWindow: 20`, `BaublePickpocketChancePct: 100`, and the three tier-weight triples (`BaubleTierWeight*`, `BaubleHouseholdTierWeight*`, `BaublePickpocketTierWeight*`) to cheap `20`, average `60`, rare `20`, so most finds are shelvable and some cheap ones test the throw-away, and `ShopAffixedStockCap: 2`, so a tester reaches both the listed-cap eviction (a third sale at one shop pushes the earliest listed entry off) and the backroom refusal (a third hot trinket at the fence) within the budget, and the refusal copy gets read in game. Leave `Modules.baubles.Enabled` as shipped, so finds are corpus finds.

**What this playtest cannot reach, and what covers it instead.** A hold ends 72 real hours after a theft, against a 45-minute budget, so no tester sees a held entry come off hold; `TestShelf_HeldEntriesAreNotListedNorEvicted`, `TestBuy_Shelf_AHeldEntryIsNotForSaleUntilItsHoldEnds` and `TestListShelf_EnforcesTheCapLazilyAndSaves` cover it. Corpus finds are not written by a player's key, so no finder-only bauble exists here; the per-viewer paths are covered by `TestListShelf_AFinderOnlyBaubleReadsByViewer`, `TestBuy_Shelf_AFinderOnlyBaubleIsBoughtInTheBuyersOwnView` and `TestBrowseShopsShowsTheShelfInTheModelsView`. The honest shop's "no room" line needs a bauble hot only in another heat area, which Thornwall alone does not give; `TestStolenBauble_AnHonestShopWithAFullBackroomHasNoRoom` covers it.

- [ ] **Step 3: Goals file** at `C:/tmp/dogmud-baubles-d-playtest/tools/playtest/goals/2026-09-30-bauble-shelf.yaml`:

```yaml
# Bauble slice D playtest: shops now keep the trinkets players sell them
# on a "Secondhand goods" shelf, and hold stolen ones out of sight until
# they cool. Finds are switched on at a very high rate in this checkout,
# and each shop lists at most two secondhand pieces and holds at most two
# hot ones.
ephemeral:
  profile: mid
  start_room: 462
  budgets:
    wall_clock: 45m

goals:
  - >-
    You start in Thornwall City. Search indoors and out around the city
    until you carry at least eight trinkets. Look at each and note its name
    and what appraise says it is worth.
  - >-
    Find Jeweler Tess, who keeps a jeweller's shop in Thornwall City. Run
    list before selling anything. Sell your trinkets to her one at a time,
    and run list after each sale. Report what appears in the Secondhand
    goods table, in what order, at what price, and whether a cheap trinket
    ever appears there. After the third sale, report which piece left the
    table. Report any table that looks wrong, repeats, or is empty when it
    should not be.
  - >-
    Buy trinkets back from the Secondhand goods table by the name list
    shows. When two share a name, try buy 2.<name>, then compare it with
    buy 2 <name> (with a space). Report whether you got the one you meant,
    how many you got, and whether the price you paid matches the list. Try
    buy with a word from a trinket's description that is not in its listed
    name, and with the word bauble, and report what happens.
  - >-
    Pickpocket townspeople with steal until you hold at least three stolen
    trinkets. Try to sell one to Jeweler Tess. Then find Fence Dealer Siv,
    who trades in the Back Alley, East of Thornwall City (a back alley
    off the city's streets; ask around if you cannot find it), and sell
    your stolen trinkets to him one at a time. Run list after each sale.
    Report whether a stolen trinket shows on his shelf (it should stay out
    of sight for now), what he says when he will not take one more, and
    whether anything he says reads oddly.
  - >-
    Run help list, help buy and help sell. Report anything that is unclear,
    that contradicts what you saw, or that a player whose first language is
    not English would struggle with.
```

- [ ] **Step 4: Run** (the skill gives the exact form):

```
/playtest local --checkout C:/tmp/dogmud-baubles-d-playtest bug-finder 2026-09-30-bauble-shelf.yaml
```

- [ ] **Step 5: Triage.** Read the whole report. Code defects: a failing test first, then the fix, in `C:/tmp/dogmud-baubles-d-impl`, committed as `fix(<pkg>): <what> (playtest)`. Copy defects in help or patch notes: fix and commit as `docs(baubles): <what> (playtest)`. Re-run the playtest if anything beyond wording changed. Extract the findings to memory (reports are gitignored). Confirm `playtestrun stop` really removed its container (`docker ps -a` shows no leftover playtest container; if one remains, `docker rm -f <that id>`). Then:

```bash
cd /c/tmp/dogmud-baubles-d-playtest && git status --short
git -C /c/tmp/dogmud-baubles-d-impl worktree remove --force /c/tmp/dogmud-baubles-d-playtest
```

Expected before removal: only `_datafiles/config.yaml` and the goals file differ.

---

### Task 18: Local gate

**Files:** none, unless a check fails. CI is out of minutes until 10-01; this is the gate and its output goes into the PR body. Save each step's output as `bauble-shelf-gate.txt` in the executing session's own scratchpad directory (the `$SCRATCH` of Task 19).

- [ ] **Step 1: gofmt on committed blobs**

```bash
cd /c/tmp/dogmud-baubles-d-impl && BASE=$(cat /c/tmp/dogmud-baubles-d-impl.base) && for f in $(git diff --name-only $BASE..HEAD -- '*.go'); do out=$(git show HEAD:"$f" | gofmt -l); [ -n "$out" ] && echo "UNFORMATTED: $f"; done; echo done
```
Expected: only `done`.

- [ ] **Step 2: Size and go.mod**

```bash
cd /c/tmp/dogmud-baubles-d-impl && BASE=$(cat /c/tmp/dogmud-baubles-d-impl.base) && git diff --shortstat $BASE..HEAD && git diff --exit-code $BASE..HEAD -- go.mod go.sum; echo "gomod-exit=$?"
```
Expected: about 53 files and under 3,500 lines (far under 300 files and 20,000 lines); `gomod-exit=0`.

- [ ] **Step 3: Build, vet, full test suite**

```bash
cd /c/tmp/dogmud-baubles-d-impl && go build ./... && go vet ./... && go test ./... -count=1 2>&1 | grep -v "^ok\|no test files" | tail -30
```
Expected (timeout 20 minutes): no vet output and nothing printed by the last command (no `FAIL`). A failure in a package this branch did not touch: re-run that package on `origin/master` in a detached worktree before deciding it is not ours.

- [ ] **Step 4: The root guards this branch leans on, by name**

```bash
cd /c/tmp/dogmud-baubles-d-impl && go test . -count=1 -run "TestFinderViewReachesOnlyItsReader|TestFinderViewGuardCatchesALeak|TestEveryCreatureLookupDeclaresItsViewer|TestLivingStateWritesAreDurable|TestNoHandRolledTempRename|TestItemWalkersVisitEveryItemField|TestEveryItemHolderIsASweepRootOrTransient|TestBaubleSweepSourcesMatchTheGuardedRoots|TestBaubleSweepReadsEveryStoreFromDisk|TestEveryTextSurfaceIsRegistered|TestNoRawEventsMessageOutsidePipeline|TestSmoke_NoNewSilentlyIgnoredYAMLKeys" -v 2>&1 | grep -E "^(--- FAIL|--- PASS|FAIL|ok)"
```
Expected: twelve `--- PASS` lines and `ok`. A missing name has been renamed on master: find it with `grep -n "^func Test" *_test.go`.

- [ ] **Step 5: Boot smoke in the test process**

```bash
cd /c/tmp/dogmud-baubles-d-impl && DOGMUD_BOOT_SMOKE=1 go test . -count=1 -run TestSmoke_ServerBootsCleanWithRealData -timeout 600s 2>&1 | tail -5
```
Expected: `ok`. (`internal/rooms`' two zone lifecycle tests fail on Windows only under this variable; this run does not touch that package.)

- [ ] **Step 6: Lint**

```bash
cd /c/tmp/dogmud-baubles-d-impl && golangci-lint run --new-from-merge-base=$(cat /c/tmp/dogmud-baubles-d-impl.base)
```
Expected: `0 issues.`

- [ ] **Step 7: Race, in the Linux test container**

```bash
cd /c/tmp/dogmud-baubles-d-impl && docker compose -f compose.test.yml run --build --rm test go test -race -count=1 ./internal/shops/ ./internal/baubles/ ./internal/actions/ ./internal/usercommands/ ./internal/economy/health/ ./internal/util/ ./internal/configs/ ./modules/auctions/ ./modules/aicompanion/ 2>&1 | tail -12
```
Expected: nine `ok` lines, no `WARNING: DATA RACE`.

- [ ] **Step 8: A real boot on private ports, stopped by its own PID**

Check the fixed boot path is free (each command exits 1 when empty; run them standalone):
```bash
git -C /c/tmp/dogmud-baubles-d-impl worktree list | grep -F "dogmud-boot-check"
```
```bash
ls /c/tmp/dogmud-boot-check 2>/dev/null | head -3
```
Expected: no output from either; if the directory exists, STOP and ask.

Build (Bash):
```bash
git -C /c/tmp/dogmud-baubles-d-impl worktree add --detach /c/tmp/dogmud-boot-check HEAD && cd /c/tmp/dogmud-boot-check && go build -o boot-check.exe . && cat > /c/tmp/dogmud-boot-check.overrides.yaml <<'EOF'
Network:
  TelnetPort: [33533]
  LocalPort: 9899
  HttpPort: 8391
  HttpsPort: 0
  AIPort: 0
EOF
echo built
```

Run (PowerShell; never ports 33333, 44444, 9999, 80 or 55555; kill only this PID):
```powershell
$env:CONFIG_PATH = 'C:\tmp\dogmud-boot-check.overrides.yaml'
$p = Start-Process -FilePath 'C:\tmp\dogmud-boot-check\boot-check.exe' -WorkingDirectory 'C:\tmp\dogmud-boot-check' -RedirectStandardOutput 'C:\tmp\dogmud-boot-check.log' -RedirectStandardError 'C:\tmp\dogmud-boot-check.err' -WindowStyle Hidden -PassThru
$deadline = (Get-Date).AddSeconds(180)
while ((Get-Date) -lt $deadline -and -not $p.HasExited -and -not (Select-String -Path 'C:\tmp\dogmud-boot-check.log','C:\tmp\dogmud-boot-check.err' -Pattern 'Server Ready' -Quiet)) { Start-Sleep -Seconds 2 }
"pid=$($p.Id) exited=$($p.HasExited)"
Select-String -Path 'C:\tmp\dogmud-boot-check.log','C:\tmp\dogmud-boot-check.err' -Pattern 'Server Ready|status=listening|Starting http server|^panic:|goroutine [0-9]+ \[running\]|runtime error|bind:' | Select-Object -First 20
if (-not $p.HasExited) { Stop-Process -Id $p.Id -Force -Confirm:$false }
Remove-Item Env:\CONFIG_PATH
```
Expected: `exited=False`, `Server Ready`, `Telnet status=listening port=33533`, the http server on 8391, and no `panic:`, `goroutine ... [running]`, `runtime error` or `bind:` line. Never grep the bare word `panic` (`GamePlay.MapConsistencyEnforce: panic` is a real config value).

Clean up (PowerShell), then (Bash) `git -C /c/tmp/dogmud-baubles-d-impl worktree prune`:
```powershell
Remove-Item -Recurse -Force 'C:\tmp\dogmud-boot-check'; Remove-Item -Force 'C:\tmp\dogmud-boot-check.overrides.yaml','C:\tmp\dogmud-boot-check.log','C:\tmp\dogmud-boot-check.err'
```

- [ ] **Step 9: Docs and tree**

```bash
cd /c/tmp/dogmud-baubles-d-impl && BASE=$(cat /c/tmp/dogmud-baubles-d-impl.base) && git diff --name-only $BASE..HEAD | sed -n 's#^\(internal/economy/health\|internal/[^/]*\|modules/[^/]*\)/.*#\1#p' | sort -u; echo ---; git diff --name-only $BASE..HEAD -- '*context.md' '_datafiles/world/dogmud/templates/help/' docs/PATCH_NOTES.md; git status --short
```
Expected: every package in the first list has its `context.md` in the second (`modules/aicompanion` included), except `internal/configs` (a knob move; its file table is unchanged); the three player help templates, `docs/PATCH_NOTES.md` and (from a separate `git diff --name-only $BASE..HEAD -- docs/baubles _datafiles/world/dogmud/templates/admincommands`) `docs/baubles/implementation-plan.md` and `command.bauble.template` listed; a clean tree.

---

### Task 19: PR and merge

- [ ] **Step 1: Write the PR body** as `bauble-shelf-pr.md` in the EXECUTING session's own scratchpad directory (the one its system prompt names; never a path copied from this plan, whose planning session is gone). Set it once in Git Bash: `SCRATCH='<that directory>'`. The body holds a summary (shelving rules and the eight rulings; the "Secondhand goods" table; `buy` by position and in the buyer's own view; the backroom; `MarkBought`, `SalesSince`, `bauble show`; the dashboard `fence` type; `ShopAffixedStockCap` 8 to 12 with a new `config.yaml` key), the spec and plan paths, the code-vs-spec notes and the review revisions from this plan (the two controller decisions called out), a "Local gate (CI out of minutes until 10-01)" section pasting `bauble-shelf-gate.txt`, the playtest findings, a "Deploy notes" section ("No deploy from this PR; the owner deploys." and "Rollback: a build from before this PR still loads shop files written after it, but ignores `hold_until`, so every hot bauble waiting in a backroom would become listed and buyable at once. Roll back only with that accepted, or after the holds have run out (72 hours after the last shelved theft)."), and the closing line `🤖 Generated with [Claude Code](https://claude.com/claude-code)`.

- [ ] **Step 2: Push and open the PR on the fork**

```bash
git -C /c/tmp/dogmud-baubles-d-impl push -u origin feature/bauble-shelf-resale
gh pr create --repo pruuk/DOGMud --base master --head feature/bauble-shelf-resale --title "feat(baubles): slice D, shelf resale" --body-file "$SCRATCH/bauble-shelf-pr.md"
```
Read the URL `gh` prints and confirm it says `pruuk/DOGMud`.

- [ ] **Step 3: Merge**

```bash
gh pr merge <number> --repo pruuk/DOGMud --merge --delete-branch
```
If GitHub refuses because required checks did not pass (no CI minutes), merge with `--admin` only if the owner has authorised merging past red CI on local gate evidence for this PR; otherwise stop and ask.

- [ ] **Step 4: Hand-off (no deploy)**

Report the PR number and merge SHA; that nothing was deployed; that the owner's main checkout `_datafiles/config.yaml` (skip-worktree `S`) lacks `ShopAffixedStockCap` until the EOD re-sync from the HEAD blob (the Go default 12 equals the shipped value, so nothing behaves differently meanwhile); then, once `gh pr view <number> --repo pruuk/DOGMud --json state` says `MERGED`: `git -C "/c/Users/Calabe Davis/workspace/DOGMud" worktree remove /c/tmp/dogmud-baubles-d-impl`, `git -C "/c/Users/Calabe Davis/workspace/DOGMud" branch -D feature/bauble-shelf-resale` (the local branch; `--delete-branch` removed the remote one, and `-D` because the local master may not have fetched the merge yet), and `rm /c/tmp/dogmud-baubles-d-impl.base`.

---

## Self-review

**Spec coverage.** Section 1 (entry fields, one clock, `Held`, `ListedAt`, `HeldCount`, `ListedIndexes`, the new `AddAffixedStock`, `EnforceAffixedCap`, `RestoreAffixedStock`, `ShelfHoldUntil` global not local): Tasks 3, 4, 5. Section 2 (`baubleShelvable`, player-only, inside the living-shop block before `SaveShop`, `MarkSold` after for every sale; the backroom refusal after the existing refusals and before the reserve, interest not `Broke`, any seller; comments): Tasks 9, 10. Section 3 (lazy trim and save, the second unsorted table titled `Secondhand goods`, Name/Type/Price, `DisplayNameFor`, price before barter, the say only when both are empty, `buildShelfRows` guard row with the spec's exact reason): Task 11. Section 4 (trim first, stock then shelf in `ListedIndexes` order, held absent from names and fancy names, raw `affixedIdx`, `NameFor` for bauble rows only, `FindMatchIndexIn` with `FindMatchIn` as a wrapper, the first-name loop replaced, keyword parity not added and documented in help, copy-then-`RestoreAffixedStock` rollback, `DisplayNameFor` on the buyer's line, `MarkBought`, save the trim when no purchase saved; guard row with the spec's exact reason): Tasks 1, 5, 12, 13, 16. Section 5 (`unsoldStatus` in `admin.go` used by `Restore`, `MarkBought` with its log, `SalesSince` by `SoldAt`, `bauble show` "last sold:", no new status, comments on `sales.go` and `record.go:17`): Tasks 6, 7. Section 6 (`Fence` yaml and json omitempty from the template, `Type()`, scoring and row, the page): Tasks 8a, 8b. Section 7: Task 2. Section 8: Tasks 10 (comment) and 15 (checklist). Section 9: Task 3 round trip, Tasks 11 and 12 saves; no migration. Section 10: Task 5. Section 11: Task 16 (all six `context.md` files the spec names, plus `modules/auctions/context.md` for the touched package). Section 12, tests 1 to 15: see the coverage map; the spec's "no seam for the rollback branch" holds (Task 3 tests `RestoreAffixedStock` directly). Section 13 risks: no task changes them; the review checklist re-reads the Bartering loop and the finder-only price note. Rulings 1 to 8: see the coverage map.

**Placeholder scan.** Every code step carries its code and its exact old text; Task 0 Step 2 greps every Edit anchor at BASE. The values filled at execution are run results (BASE, the PR number, gate output) and the patch-note date if the merge is not on 2026-10-01. The Task 16 `context.md` anchors were grepped at HEAD too (each occurs once).

**Type consistency.** `FindMatchIndexIn(searchName string, items ...string) (match int, closeMatch int)` (Tasks 1, 12). `ShelfNow func() time.Time` (Tasks 3, 9, 10, 11, 12 and test helpers). `(AffixedStockEntry) Held(now time.Time) bool`, `ListedAt() time.Time`; `(*ShopInventory) HeldCount(now time.Time) int`, `ListedIndexes(now time.Time) []int`, `EnforceAffixedCap(limit int, now time.Time) int`, `RestoreAffixedStock(idx int, e AffixedStockEntry)`, `AddAffixedStock(item items.Item, price, limit int, holdUntil, now time.Time)` (Tasks 3, 5, 9, 10, 11, 12). `baubles.ShelfHoldUntil(itm items.Item, now time.Time) time.Time` (Tasks 4, 5, 9). `baubles.MarkBought(id string, buyerUserId int) bool`, `(Record) unsoldStatus() Status` (Tasks 6, 7, 13). `baubleShelvable(rec baubles.Record) bool`, `baubleSayBackroomFull` (Tasks 9, 10). `buildShelfRows(si *shops.ShopInventory, viewerUserId int, now time.Time) [][]string`, `renderShelfListing(user *users.UserRecord, si *shops.ShopInventory, sellerName string, now time.Time) bool` (Task 11). `ShopSnapshot.Fence bool`, `(ShopSnapshot) Type() string` (Tasks 8a, 8b). Test helpers `shelfEntry`, `shelfIds` (Tasks 3, 5), `shelfListFixture`, `shelfGear`, `plainRow` (Task 11), `shelfBuyFixture`, `shelfBuyer` (Tasks 12, 13) are each defined before use. `invEntry.plainName` is removed in Task 12 and read nowhere after. Added by the review revisions: `baubleSayNoRoom` (Task 10, beside `baubleSayBackroomFull`); `ware.Secondhand bool` (Task 13b, read by `rememberShop` and `describeListing`); test files `economy_page_test.go` (8b) and `economy_shelf_test.go` (13b), each with its own imports so no task leaves an unused import behind.

**Review revisions coverage.** Every item of the 2026-09-30 plan review maps to a change listed under "Revisions after the plan review" near the top; the re-run dry run applied Tasks 1 to 13b and the `sweep.go` comment (39 code and test files), passed `go build ./...`, `go vet`, the tests of every touched package, `internal/actions` and the root package, and all 17 revised or new null probes compiled and went red for their named reasons.
