# Merchant Chests Context

## Purpose

`internal/merchantchests` keeps a locked chest in every merchant's shop and
restocks it on a timer. A chest is an ordinary room container
(`rooms.Container`), so `picklock`, `look`, `get`, `put` and `steal` already
work on it. This package owns what makes one a merchant's chest: which
merchant and room it belongs to, its name and description, a lock whose pins
scale with the merchant's average stock value, and the restock.

The theft side lives elsewhere: `actions/merchant_chest.go`
(`WatchMerchantChest`, the sleeping-merchant Perception factor),
`usercommands/merchant_chest.go` (the once-per-lock-cycle take memo) and
`actions/sell_stolen.go` and `actions/stolen_bauble.go` (stolen goods on
the stolen-bauble rules: hot for three days in the area of the theft,
recognised on the thief by the merchant and that area's guards, a fence's
cut anywhere).

## Data

`DataFiles/merchant_chests.yaml`, read strictly (`yaml.UnmarshalStrict`, so
an unknown key is a boot panic, not a silently ignored value):

- `settings:` every tunable (`Settings`): `restock_interval` and
  `relock_interval` (gametime periods), `gold_value_ratio`, `gold_spread`,
  `items_min`, `items_max`, `lookout_mobs` (town law-keepers outside the
  `guard` group who recognise hot chest goods on a thief in their own heat
  area, as a guard does; `IsLookout`, checked to exist by `CheckWorld`), the
  lock formula (`lock_base`,
  `lock_per_doubling`, `lock_min`, `lock_max`), the Perception formula
  (`perception_base`, `perception_per_doubling`) and
  `sleeping_perception_mult`.
- `generic:` the generic chest (`Chest`): the name and description any
  chest that leaves them empty falls back to.
- `chests:` one `Chest` per merchant and room: `mobid`, `roomid`, `name`
  (one word; it is typed as a container noun and forms the lock id
  `<roomid>-<name>`), `description`, and optional `difficulty` (pins, 2-32;
  0 derives them from stock value). A merchant that trades from two rooms
  has one entry per room.

The roster, with every merchant's average stock value, Perception before
and after, chest and pins, is `docs/economy/merchant_chests.md`.

## Formulas

With `d = log2(1 + avg)`, where `avg` is `AverageStockValue(mobId)`, the
plain mean of `ItemSpec.Value` over the distinct item ids in the merchant
template's `shop:` list (`StockItemIds`):

- `LockDifficulty(s, avg)` = `lock_base + lock_per_doubling x d`, rounded,
  clamped to `[lock_min, lock_max]`.
- `TargetPerception(s, avg)` = `perception_base + perception_per_doubling x
  d`, rounded. Merchant templates carry this as `stats.perception.base`;
  `CheckWorld` warns at boot about any that drift.
- `GoldFor(s, avg, roll01)` = `avg x gold_value_ratio x (1 + spread)`, with
  `roll01` in [0,1] mapped onto `[-gold_spread, +gold_spread]`, at least 1.

## API

```go
func Load()
func Validate(c Catalog) error
func CheckWorld(spawnsIn func(mobId, roomId int) bool) (errs []string, warnings []string)
func EnsureAll()
func Tick(roundNumber uint64)
func Restock(room *rooms.Room, ch Chest, now uint64)
func MarkOpened(roomId int, containerName string, now uint64)

func Current() Settings
func All() []Chest
func ChestAt(roomId int, containerName string) (Chest, bool)
func OwnerIn(room *rooms.Room, ch Chest) *mobs.Mob
func SleepingPerceptionMult() float64
func IsLookout(mobId int) bool

func StockItemIds(mobId int) []int
func AverageStockValue(mobId int) float64
func LockDifficulty(s Settings, avg float64) int
func TargetPerception(s Settings, avg float64) int
func GoldFor(s Settings, avg float64, roll01 float64) int

// test helper
func SetForTest(c Catalog) func()
```

## Lifecycle

- Boot and `/reload` (`main.go`, after the shop-room eager spawn): `Load`,
  `CheckWorld` (errors: unknown merchant, no shop list, merchant does not
  spawn in the room; panic at boot, logged on reload; Perception drift is a
  warning), then `EnsureAll`.
- `EnsureAll` places every chest (`syncContainer`: the catalog's lock pins,
  relock interval and description win over a saved room instance; contents
  and the open or shut state are left alone) and reads each chest's restock
  record into an in-memory clock (`chestClock`). It restocks NOTHING: at
  boot `loadAllDataFiles` runs before `util.LoadRoundCount`, so the round it
  would see is `RoundCountMinimum`, not the saved one.
- `Tick` runs from the `MerchantChestRestock` NewRound hook, on the live
  round. It looks every `tickEveryRounds` rounds and loads only the rooms
  whose chests need work. A chest is due when never stocked, when its time
  has come, or when its record is from the future (a reset round counter;
  `isDue`), so the first look after boot stocks every new chest.
- Relocking: `gamelock.Lock.IsLocked` never relocks an opened lock (it
  measures `RelockInterval` from the current round, which never comes due;
  a known engine bug, left alone because fixing it relocks every door and
  quest-opened exit in the world). So `picklock` calls `MarkOpened` when it
  opens a merchant chest, and `Tick` locks it again with `SetLocked` once
  `relock_interval` has passed, which also rotates the combination. A
  `/reload` keeps the open timers.
- `Restock` clears the chest, sets gold (`GoldFor`), draws `items_min` to
  `items_max` goods without repeats from `StockItemIds`, stamps each
  `StolenFrom` (the merchant's name) and `StolenFromMob` (its template),
  which makes it the merchant's goods; taking it out later starts its heat
  (`items.Item.MarkTaken`). It then locks the chest with `SetLocked`,
  which rotates the combination so a thief's keyring solution stops
  working. It records the round in the room's long-term data
  (`merchantchest:<name>:restocked`), saved with the room instance, so a
  reboot restocks only the chests that are due.

## Gotchas

- Renaming or removing a chest in the catalog does not remove the old
  container from a room instance already saved with it. Clear that room's
  instance, or remove the container by hand.
- A merchant's live Perception is its template base plus its random
  `statpool` share at spawn. Mob instance saves (`mobs.MobInstanceData`)
  hold only training gains, so a changed base takes effect at the next
  boot without clearing `mobs.instances`.
- Chest gold and goods are not the shop's: restocking mints them, and a
  theft does not drain the merchant's shop inventory or gold.
- A restock clears the chest, so anything a player `put` in it is gone, and
  it rotates the combination under a thief who is mid-pick.
- A builder `SaveRoomTemplate` on a merchant room writes the live chest,
  stolen-marked goods and all, into the room template. It is harmless (the
  next restock clears it and `syncContainer` retunes the lock) but noisy in
  the authored YAML; strip the container from the template by hand.
