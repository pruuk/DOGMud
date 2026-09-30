# Housing Context

## Purpose

`internal/housing` owns persistent player housing: the authored lodging
buildings, the living-state record of who owns which room, the landlord
purchase, and the routing that sends each lodger through a building's one
shared door to their own room.

Phase 1 (2026-09-29) ships one building, the Back Court lodgings in New
Plymouth Common: Hobb Pennock (mob 9801) in Pennock's Alley (room 5625,
west of the Back Court 5615) lets 24 blank single-room units (6470-6493,
zone New Plymouth Lodgings) for 500 gold to players the Common Quarter
vouches for (`np_commonfolk` standing Warm or better). Customization,
bigger houses, furniture and visiting are later phases; the data model
already carries a list of rooms per house so they need no migration.

## Two kinds of data

**Buildings are authored content.** One YAML per building in
`<DataFiles>/housing_buildings/<building_id>.yaml`, git-tracked, loaded with
`fileloader.LoadAllFlatFiles`. A bad file PANICS at boot like any authored
content, including world checks: the door room exists, its door exit exists
and is LOCKED, the landlord mob and faction exist, every unit room exists,
leads back to the door room through an exit of the same name, belongs to one
building only, and spawns no mobs.

**Houses are living state.** One YAML per house in
`<DataFiles>/housing/<building_id>/<entry_room_id>.yaml`. Gitignored, in
`provisioning/Dockerfile.dockerignore`, kept on the production droplet, and
never touched by the instance-save wipe (`make clean-instances` only removes
`rooms.instances/` and `mobs.instances/`). It follows the living-state
contract (`internal/util/livingstate.go`):

- writes go through `util.Save` (`saveHouse`);
- reads go through `util.ReadLivingState`, so absent is not corrupt;
- an unreadable or unparseable file is moved aside by
  `util.QuarantineCorrupt`, logged at ERROR, and never deleted;
- a purchase writes the house BEFORE it takes gold or touches the registry.

A file is named by its entry room, not its owner, so a file that cannot be
read still says which room it guarded. That room is **held**: never sold and
never entered (except by staff) until someone repairs the file. A readable
file that disagrees with the world (unknown building, a room that is not a
unit, a room already owned, a second house for the same owner) is left in
place and its rooms held. `HeldRooms` lists them.

## Files

- **housing.go**: `Building`, `Tier`, `House`, `ParseRepTier`, building
  validation.
- **registry.go**: the in-memory registry, `LoadDataFiles`, building world
  validation, lookups.
- **persistence.go**: house files: `saveHouse`, `loadHouses`, quarantine.
- **door.go**: `RegisterRoomHooks`, `RouteDoor`, `GuardEntry`.
- **purchase.go**: `Purchase`, `PurchaseResult`.
- **terms.go**: `DescribeTerms`, the landlord's price-and-eligibility answer.

## Public API

```go
func LoadDataFiles()          // buildings, then houses; safe to call again on reload
func RegisterRoomHooks()      // once at boot (main.go)

func GetBuilding(buildingId string) (Building, bool)
func BuildingForUnit(roomId int) (Building, bool)
func IsUnitRoom(roomId int) bool
func HouseOf(userId int, buildingId string) (House, bool)
func HouseForRoom(roomId int) (House, bool)
func AllHouses() []House
func VacantUnits(buildingId string) []int
func HeldRooms() map[int]string

func Purchase(user *users.UserRecord, say func(string), buildingId string, tierId string) PurchaseResult
func DescribeTerms(user *users.UserRecord, say func(string), buildingId string, tierId string) bool

func RouteDoor(userId int, fromRoomId int, exitName string) (rooms.ExitRoute, bool)
func GuardEntry(userId int, toRoomId int) (bool, int, string)
func ParseRepTier(name string) (opinions.Tier, bool)
```

Test hooks: `SetDataDirForTest`, `SetBuildingsDirForTest`,
`SetRepTierForTest`, `ResetForTest`, `AddBuildingForTest`.

## How the door works

The door is a real authored exit (`door` in room 5625) with a lock of
difficulty 255. It points back at its own room only because an exit needs a
destination; nobody ever arrives by it.

`RegisterRoomHooks` installs three hooks in `internal/rooms`
(`routing_hooks.go`), which cannot import this package:

- `rooms.SetExitRouter(RouteDoor)`: `usercommands.Go` asks the router before
  moving. An owner is sent to their own entry room and the authored lock is
  skipped; anyone else gets the refusal and does not move. Everything else in
  `Go` (combat, stamina, sneaking, companions, quest `room_enter`, visited
  rooms, narration) runs unchanged. `unlock` ("open door") delegates to `Go`,
  `picklock` refuses before the lockpicks check, and `look door` shows the
  room's `door` noun instead of peering through. Arriving back in the alley
  reads "enters from the door" because `Go` asks the router which exit would
  have led here.
- `rooms.SetEntryGuard(GuardEntry)`: `rooms.MoveToRoom` refuses to place a
  player in a unit room they do not own, however they arrive (walking,
  recall, a summons, a quest move, admin teleport of someone else). Staff
  (`users.RoleAdmin`) may enter any unit. A refused LOGIN placement is
  redirected to the building's door room, so nobody logs in stranded.
- `rooms.SetPrivateRoomCheck(IsUnitRoom)`: the loot goblin
  (`rooms.GetRoomWithMostItems`) never picks a unit room, owned or vacant.

Mobs never consult the router; the lock keeps them out of the door, and unit
rooms have no inbound exits.

Unit rooms are ordinary authored rooms, so their floors persist through the
normal room instance save and are already walked by the bauble sweep. Note
that the local smoke-test instance wipe clears those floors in development,
as it does every room's.

## The landlord

Hobb Pennock's behavior tree
(`behaviors/new_plymouth_common/9801-hobb_pennock.yaml`) calls two actions
registered in `internal/behaviortree/actions_housing.go`:
`buy_housing` (on `buy`/`purchase`) and `housing_terms` (on room, price and
similar words). Both take `building` and `tier` params and read every
number from the building file, so tree prose cannot drift from the price.

`Purchase` refuses, with the landlord speaking, in this order: unknown
building or tier; already owns a house in this building (one per ACCOUNT,
shared by alts like the bank); standing below `min_rep_tier` with `faction`;
carried gold plus bank below the price; no vacant unit. It then writes the
house, publishes it, takes carried gold first and the bank after (as paying a
fine does), and queues `events.EquipmentChange`.

## Adding capacity or a building

Run `tools/housing_units.py` to generate blank unit rooms (it refuses to
overwrite), then list the new ids in the building's `unit_rooms`. A new
building also needs a door exit in a street room (locked, no map direction),
a landlord mob with a tree calling the two actions, and a
`housing_buildings/<id>.yaml`. Record new rooms in the lighting goldens
(`-update-lighting-parity`, `-update-lighting-daycycle`) and confirm the diff
only adds them.

## Gotchas

- **A house holds no items yet.** `TestHouseHoldsNoItemsYet` fails the day a
  `House` field reaches an `items.Item`. When it does, register the store
  with the bauble sweep (a `WalkItems`, a live source in `bauble_sweep.go`,
  a root in `item_walker_guard_test.go`) or baubles kept at home are pruned.
- **Keep the door exit locked and without a `mapdirection`.** The lock is the
  only thing stopping mobs; a map direction would make the mapper draw a
  self-loop.
- **Unit `door` exits have no map direction either**, which is what keeps
  each unit a one-room map instead of every unit crawling the overworld.
- **Holding is not deletion.** A held room stays held until its file is
  fixed or removed by hand and the server reloads.

## Dependencies

`configs`, `events`, `factions`, `fileloader`, `messaging`, `mobs`,
`mudlog`, `opinions`, `rooms`, `users`, `util`, plus YAML.

## Consumers

`main.go` (boot), `internal/behaviortree` (`buy_housing`, `housing_terms`),
and through the room hooks `internal/rooms` and `internal/usercommands`.
