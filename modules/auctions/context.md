# Auctions Module Context

## Purpose

`modules/auctions` is the global auction house. Players list items; other
players **and a panel of NPC buyers** bid against each other in real gold. It
also receives lots seized from unpaid storage, which is how abandoned goods
re-enter the economy.

The NPC bidders are what make the house feel alive on a small player
population — a listing with no human interest still finds a price.

## Files

- **auctions.go** — the module, `AuctionManager`, `AuctionItem`, the `auction`
  command, persistence, and the round handler.
- **npc_buyers.go** — the `NpcBuyer` interface, `NpcWallet`, and each buyer
  archetype.
- **files/datafiles/templates/auctions/** — the four broadcasts and their 42
  and 30 forms (see Broadcasts).
- **history_table_test.go** — `auction history` at 30, 42 and 80, led by
  the Item, and its goldens in `testdata/`.

## Types

```go
type AuctionItem struct     { /* item, seller, bids, buyout, reserve, end round */ }
type PastAuctionItem struct { /* completed record */ }
type SeizedLot struct       { /* storage-seizure origin */ }
type AuctionManager struct  { /* active + history */ }

type AuctionUpdate struct   { /* event payload; Type() == "AuctionUpdate" */ }
```

## NPC buyers

```go
type NpcBuyer interface {
    Name() string
    Interested(item items.Item) bool
    MaxBid(item items.Item) int
    CanAfford(n int) bool
    Spend(n int)
    Refund(n int)
    Wallet() *NpcWallet
    Flavor() string
}
```

Archetypes — collector, craftsperson, adventurer, merchants' guild, crown
assessor — differ in what they want (`Interested`) and what they will pay
(`MaxBid`). `Flavor()` supplies the reason shown to players ("for their
collection"), which is what stops NPC bids reading as a faceless bot.

`NpcWallet` is a real balance: `CanAfford`/`Spend`/`Refund`/`Regen`. A buyer
that has been spending cannot keep bidding, and outbid gold is refunded.

A lot the shopkeeper buyer wins goes onto its bound shop's secondhand shelf
(`Receive`, baubles slice D): `shops.AddAffixedStock` with
`baubles.ShelfHoldUntil`, which is zero today since the shopkeeper never
wins a bauble (`EvaluateBuyRules` refuses the carrier). A won item that
would be held is not shelved when the shop's `BackroomFull` returns true
(the rule the bauble sale's offer refuses by): it leaves the world instead,
as a legacy shop's purchase would. A bauble is further gated by
`baubles.Record.Shelvable` (owner ruling 5): a cheap or retired one leaves
the world instead of shelving, same rule the player-sale path in
`internal/actions` applies; a non-bauble item is unaffected. The shop still
paid for it either way, capped by `Balance.ShopAffixedStockCap`.

## Broadcasts and their forms

The four broadcasts (`auctions/auction-start`, `-update`, `-bid`, `-end`)
live in `files/datafiles/templates/auctions/`, each with a `.w42` and a
`.w30` form (plan 3b-3): a full-width star rule, the word rule (star and
space, so 41 and 29 wide), and every line inside the word rule. The 80
templates are unchanged.

`broadcastAuction(fname, auction, readers)` sends one to every reader in
`onlineReaders()` who has not turned auctions off, checking the opt-out
before rendering, and renders it once per distinct (screen reader, width)
among them through `renderAuction` (`templates.ProcessForWidth`, which picks
the form by width without measuring: spec ruling 11). A bid reaches each
reader at their own width; it once went out rendered at the bidder's.
`broadcast_test.go` holds the per-view rendering. The `auction` command with
no arguments shows the update through `templates.Process`, as a screen the
player asked for.

`auction history` is a `tables/generic` table whose two-line form leads
with the Item (`WithLead(1)`, plan 3b-3 ruling 6). `history_table_test.go`
fits it at 30, 42 and 80 with the longest shipped item name and pins its
bordered and plain-column forms (`testdata/auction_history_*.golden`,
recorded from master; `-update-auction-tables` rewrites them).

## Gotchas

- **Player money is BANK-ONLY, never carried gold.** `Bid` checks and debits
  `Character.Bank`, `refundUser` refunds to `Character.Bank`, and the seller
  payout settles to `Character.Bank`. `Bid` owns the entire money path
  (check, escrow, refund the previous bidder, emit `BankChange`), so a caller
  must not add its own balance check or debit around it. The `auction bid`
  command handler used to do both against `Character.Gold`: it locked out anyone
  whose money was banked, and on success charged the winner twice out of two
  different pools. Since refunds only return to the bank, the carried-gold half
  was destroyed. `bank_only_test.go` now fails any production file in this
  module that touches `Character.Gold`.
- **Wallets regenerate; they are not infinite.** A test that lists many lots
  quickly will see NPC interest dry up. That is the design, not a bug.
- **`Refund` must be called on every outbid path.** A missed refund silently
  drains an NPC's wallet over days of uptime.
- **`tooTrivialToAuction` filters junk** before it ever reaches the house. If a
  listing is refused, check there first.
- **`auctionRefusesStolen` turns away a stolen bauble hot where it is
  listed from** (docs/baubles Phase 6c: `baubles.ItemIsHotIn` with the
  lister's room zone, read through the `auctionClock` clock), so the house
  cannot launder what the area's honest merchants refuse; listed from
  another town it is just a trinket.
- **`restoreNpcBinding` re-attaches buyers after load** — a persisted auction
  holds a buyer id, not a live object. Anything reconstructing auctions must
  call it or NPC bids stop mid-auction.
- **`reserveFrom(buyout, pct)` derives the reserve from the buyout.** Changing
  the percentage retroactively changes nothing already listed.
- **Auction state is persisted by the module itself** (`load`/`save`), not by
  the engine's world save.

## Dependencies

`plugins`, `events`, `items`, `users`, `rooms`, `mudlog`, `configs`,
`templates`, `messaging`.

## Consumers

Registered as a plugin; drives the `auction` command, the storage-seizure
handler, and the Discord auction mirror.

## WalkItems and the bauble sweep

`(*AuctionManager).WalkItems` (walk_items.go) walks the lot on the block and
the seized lots; past auctions keep names only. `init` registers it with
`baubles.RegisterLiveSource("auctions", ...)`, so the bauble catalog sweep
sees items held by the auction house between saves.
