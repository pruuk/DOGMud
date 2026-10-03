# companionai Context

## Purpose

`internal/companionai` is the seam between the engine and
`modules/aicompanion`, the module that drives bonded AI companions.
`internal/` never imports `modules/`, so the engine calls the functions here
and the implementations are installed at boot.

## API

```go
type AskFunc func(userId int, mobInstanceId int, text string) bool
type RespawnFunc func(userId int, mobId int) int
type IdleFunc func(mobInstanceId int) bool
type RejoinFunc func(userId int) bool
type SnapshotFunc func(userId int) bool
type NpcAskFunc func(ownerUserId int, mobInstanceId int, text string, authorized bool) bool
type BondedFunc func(mobInstanceId int) bool
type HoldFunc func(userId int, mobInstanceId int) bool

func SetAskHandler(f AskFunc)
func RouteAsk(userId int, mobInstanceId int, text string) bool

type ShowFunc func(userId int, mobInstanceId int, name string, description string, madeBy string, handMade bool) bool
func SetShowHandler(f ShowFunc)
func RouteShow(userId int, mobInstanceId int, name string, description string, madeBy string, handMade bool) bool

type BaubleSearchFunc func(mobInstanceId int) (ownerUserId int)
func SetBaubleSearcher(f BaubleSearchFunc)
func BaubleSearchFor(mobInstanceId int) int
type SearchedFunc func(mobInstanceId int, found []string)
func SetSearchedHandler(f SearchedFunc)
func RouteSearched(mobInstanceId int, found []string)
type BaubleFoundFunc func(mobInstanceId int, name string, pocketed bool)
func SetBaubleFoundHandler(f BaubleFoundFunc)
func RouteBaubleFound(mobInstanceId int, name string, pocketed bool)

func SetRespawner(f RespawnFunc)
func RespawnBonded(userId int, mobId int) int

func SetIdleHandler(f IdleFunc)
func RouteIdle(mobInstanceId int) bool

func SetRejoiner(f RejoinFunc)
func Rejoin(userId int) bool

func SetSnapshotter(f SnapshotFunc)
func Snapshot(userId int) bool

func SetNpcAsker(f NpcAskFunc)
func AskNpc(ownerUserId int, mobInstanceId int, text string, authorized bool) bool

func SetBondedCheck(f BondedFunc)
func IsBondedCompanion(mobInstanceId int) bool
func DrivesBonded(mobId int) bool
func SetDrivesCheck(f DrivesFunc)

func SetHolder(f HoldFunc)
func HoldPosition(userId int, mobInstanceId int) bool

// relay.go: the player's own-key relay (player keys, tier 2)
type RelaySendFunc func(userId int, module string, payload []byte) bool
type RelayInboundFunc func(userId int, command string, payload []byte)
type RelayPageFunc func(w http.ResponseWriter, r *http.Request) bool

func SetRelaySender(f RelaySendFunc)
func SendRelay(userId int, module string, payload []byte) bool

func SetRelayInbound(f RelayInboundFunc)
func RelayInbound(userId int, command string, payload []byte)

func SetRelayPage(f RelayPageFunc, origin ...func() string)
func ServeRelayPage(w http.ResponseWriter, r *http.Request) bool
func RelayOrigin() string
```

- `RouteAsk` is called by `internal/usercommands/ask.go` before the normal
  companion and NPC paths. The aicompanion module installs the handler and
  claims asks aimed at companions it drives.
- `BaubleSearchFor` and `RouteSearched` are called by
  `internal/mobcommands/search.go` (search.go here): whether this companion's
  search also rolls for a bauble, and for which owner (0 for no roll, always
  0 with no module), and what the search turned up, in plain words.
  `RouteBaubleFound` is called by `internal/actions/search_bauble.go` when a
  companion's find is worked free (`BaubleDelivery.ByMobInstanceId`):
  `pocketed` is false when she was carrying too much and it was left on
  the ground. `name` is the MODEL-SAFE name (`items.Item.ModelName`), never
  the finder's view, since it reaches a prompt; the root
  `bauble_finder_view_guard_test.go` lists it as a call beyond the reader.
- `RouteShow` is called by `internal/usercommands/show.go` after a player
  shows an item to a mob, with the item's model-safe name and description
  (`items.Item.ModelName`, `ModelDescription`, never player-written bauble
  text), its maker's mark, and whether it was crafted at all (`handMade`). The aicompanion module claims shows aimed at a
  companion waiting in the Waystone Hollow, who weighs what they are shown;
  the item never leaves the player.
- `RouteIdle` is called by `internal/hooks/MobIdle_HandleIdleMobs.go` right
  after the sleeping check. The aicompanion module claims the idle tick of
  every companion it drives, so floor-loot grabs, behaviour-tree idle and
  canned idle emotes never compete with it.
- `Snapshot` is called by the aicompanion module every few rounds and after
  any trade or pickup, so the bonded companion's gear, gold and progression
  are in the owner's user record when the engine autosaves or shuts down.
  (The engine itself snapshots companions only at logout, so a restart
  would otherwise lose a session's purchases and loot.) `internal/hooks`
  installs `SnapshotBondedCompanion`.
- `Rejoin` is called by the aicompanion module when a companion that went
  off on an errand has no known way back to its owner. `internal/hooks`
  installs `RejoinBondedCompanions`, which reuses `TransportCompanions`, so
  the return looks exactly like following.
- `RespawnBonded` is called by the aicompanion module when a fallen bonded
  companion has recovered. `internal/hooks` installs the implementation
  (`RespawnBondedCompanion`), because it owns `applyCompanionState`.
- `HoldPosition` is called by `internal/hooks` `TransportCompanions` once
  per companion it is about to move, with that companion's mob instance.
  The aicompanion module installs `holdFollow`, which holds only the bonded
  companion it drives (its owner sneaking, or it walking in on foot a
  moment later); every other companion of the same owner follows as before.
- `DrivesBonded(mobId)` is called by `internal/usercommands/dismiss.go`
  with the companion's mob template id. It is true only while a drives
  check is installed (`SetDrivesCheck`, which the aicompanion module does
  only when switched on) and that check says it drives this companion (it
  has the companion's profile). `dismiss` refuses a bonded companion that
  is driven, and lets the owner part with one peacefully that is not. It
  asks by template rather than live instance so a fallen companion, or one
  not yet taken up at login, is still driven.
- `IsBondedCompanion` is also asked by `internal/hooks/mob_area_harm.go`:
  a bonded companion's area harm spell, when it resolves, spares whatever
  her owner (`GetCharmedUserId`) could not harm, by the engine's own
  `mobs.CheckPlayerHarm`, `(*rooms.Room).CanPvp` and party check.

- **The relay seams** carry a companion's model request to the owner's own
  browser and the provider's reply back, for a player running their
  companion on their own key (spec
  `docs/superpowers/specs/completed/2026-09-25-aicompanion-player-keys-design.md`).
  `modules/gmcp` installs the sender (`gmcp.Relay.go`), which returns false
  when the player has no GMCP-negotiated connection; it forwards inbound
  `Companion.Relay.Response`, `.Ready` and `.Gone` to `RelayInbound`. The
  aicompanion module installs the inbound handler and the page (player-keys
  plan Tasks 9 and 11; until then nothing installs them and both are inert).
  `internal/web` `serveTemplate` asks `ServeRelayPage` before anything else,
  so the module alone answers its relay host, and sets a
  `Content-Security-Policy` with `frame-src` = `RelayOrigin()` on the game
  page (`webclient-pure.html`) only. `SetRelayPage(nil)` removes the page
  and its origin together.

## Gotchas

- **Every entry point is nil-safe.** With nothing installed each returns
  "not handled", so the engine behaves as before when the module is absent.
- **Callers hold the mud lock, except for the relay seams.** The seams in
  `companionai.go` run on the game loop; the installed functions must never
  take `util.LockMud()` themselves. The relay seams do NOT: `RelayInbound`
  runs on the player's connection goroutine, `ServeRelayPage` and
  `RelayOrigin` on web request goroutines, and `SendRelay` wherever the
  module makes its model call. So they are stored atomically, and an
  installed inbound handler must touch no game state.
- **No engine imports.** This package imports only the standard library
  (`net/http`, `sync/atomic`) so anything can call it without creating an
  import cycle.

## Consumers

`internal/web` (the relay page and the game page CSP), `modules/gmcp`
(installs the relay sender, forwards relay messages),
`internal/usercommands` (ask, dismiss), `internal/hooks` (installs the
respawner, asks the follow hold), `internal/actions` and `internal/seeders`
(the bonded check), `modules/aicompanion` (installs the ask handler, the
holder and the bonded check, calls the respawner).
