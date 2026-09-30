# Housing Context

## Purpose

`internal/housing` owns persistent player housing: the authored lodging
buildings, the living-state record of who owns which rooms, the landlord's
list (homes, extension deeds, redecorating vouchers), the use of those
items, and the routing that sends each lodger through a building's one
shared door to their own rooms.

One building ships, the Back Court lodgings in New Plymouth Common. Hobb
Pennock (mob 9801), a bored letting clerk for Crewe Lettings, sits in
Pennock's Alley (room 5625, west of the Back Court 5615) and sells from a
list:

- **Home**, 500 gold, to players the Common Quarter vouches for
  (`np_commonfolk` standing Warm or better). One per account.
- **Room Extension Deed** (item 55), priced at 3 times everything the lodger
  has paid for rooms so far: 1500 after the home, then 6000, then 24000.
  Bound to the buying account. Used inside the lodging toward a direction, it
  knocks a new room through (`use deed north`, or `use deed` for a menu).
- **Redecorating Voucher** (item 56), 500 gold. Used in a lodging room, it
  replaces that room's description with text the lodger writes.
- **Guest Key** (item 57), 100 gold, made out to the buyer's house. Given to
  a friend and used at the door once, it adds them to the house's guest list
  (up to 5). The owner revokes with `house revoke <name>`.

Homes and extensions come from one pool of 100 blank unit rooms
(6470-6569, zone New Plymouth Lodgings). A house holds at most 8 rooms.

## Two kinds of data

**Buildings are authored content.** One YAML per building in
`<DataFiles>/housing_buildings/<building_id>.yaml`, git-tracked, loaded with
`fileloader.LoadAllFlatFiles`. A bad file PANICS at boot like any authored
content, including world checks against room TEMPLATES (not live rooms,
which carry overlays after a reload): the door room exists, its door exit
exists and is LOCKED, the landlord mob, faction and both items exist, every
unit room exists, leads back to the door room through an exit of the same
name, belongs to one building only, spawns no mobs, and has no authored
north/south/east/west/up/down exit (extensions place those).

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
- every change (purchase, deed bought, deed used, voucher used) builds a new
  record with `House.clone`, writes it, and only then publishes it to the
  registry and takes gold or items.

A house records its rooms (`RoomIds`, the first is the entry), the doorways
between them (`Links`, one entry per doorway, the way back implied), the
owner's descriptions by room (`Descriptions`) and what was paid for rooms
(`RoomsPaid`; houses from before extensions have it 0 and `Spent` falls back
to `PricePaid`), and the guests let in with keys (`Guests`, by account, with
the character name that used the key for lists and revoking).

A file is named by its entry room, not its owner, so a file that cannot be
read still says which room it guarded. That room is **held**: never sold and
never entered (except by staff) until someone repairs the file. A readable
file that disagrees with the world (unknown building, a room that is not a
unit, a room already owned, a second house for the same owner, a doorway
outside the house or two doorways in one direction, or a guest who is the
owner or listed twice: `checkLinks`) is left in place and its rooms held. `HeldRooms` lists them.

## Files

- **housing.go**: `Building`, `Tier`, `House`, `RoomLink`, `ParseRepTier`,
  `ParseDirection`, building validation, doorway geometry (`exitsOf`,
  `offsets`, `checkLinks`).
- **registry.go**: the in-memory registry, `LoadDataFiles`, building world
  validation, lookups, unit coordinates.
- **persistence.go**: house files: `saveHouse`, `loadHouses`, quarantine.
- **door.go**: `RegisterRoomHooks`, `RouteDoor`, `GuardEntry`.
- **overlay.go**: `ApplyOverlay`, laying a house over its rooms.
- **offers.go**: the landlord's list: `Offers`, `MatchOffer`, `Buy`.
- **purchase.go**: `Purchase` (a home), `PurchaseResult`.
- **use_items.go**: `UseItem` (deeds and vouchers).
- **terms.go**: `DescribeTerms`, the landlord's answer in words.
- **guests.go**: guest keys and the guest list: `Revoke`, `Leave`,
  `HousesOwnedBy`, `GuestOf`, `BuildingForDoor`, ejection.

## Public API

```go
func LoadDataFiles()          // buildings, then houses, then overlays; safe to call again on reload
func RegisterRoomHooks()      // once at boot (main.go)

func GetBuilding(buildingId string) (Building, bool)
func BuildingForUnit(roomId int) (Building, bool)
func LandlordBuildings(mobId int) []Building
func IsUnitRoom(roomId int) bool
func IsHousingItem(itemId int) bool
func HouseOf(userId int, buildingId string) (House, bool)
func HouseForRoom(roomId int) (House, bool)
func HostsOf(userId int, buildingId string) []House   // houses where userId is a guest
func HousesOwnedBy(userId int) []House
func GuestOf(userId int) []House
func BuildingForDoor(roomId int) (Building, bool)
func Revoke(ownerId int, name string) (Guest, error)
func Leave(userId int, ownerName string) (House, error)
func AllHouses() []House
func VacantUnits(buildingId string) []int
func HeldRooms() map[int]string

func Offers(user *users.UserRecord, buildingId string, tierId string) []Offer
func MatchOffer(request string, ownsHome bool) (string, bool)   // OfferHome, OfferExtension, OfferRedecorate, OfferGuestKey
func Buy(user *users.UserRecord, say func(string), buildingId string, tierId string, key string)
func Purchase(user *users.UserRecord, say func(string), buildingId string, tierId string) PurchaseResult
func DescribeTerms(user *users.UserRecord, say func(string), buildingId string, tierId string) bool
func UseItem(user *users.UserRecord, room *rooms.Room, itm items.Item, args string, rest string) bool

func RouteDoor(userId int, fromRoomId int, exitName string) (rooms.ExitRoute, bool)
func GuardEntry(userId int, toRoomId int) (bool, int, string)
func ApplyOverlay(r *rooms.Room)
func ParseRepTier(name string) (opinions.Tier, bool)
func ParseDirection(s string) (string, bool)
```

Test hooks: `SetDataDirForTest`, `SetBuildingsDirForTest`,
`SetRepTierForTest`, `ResetForTest`, `AddBuildingForTest`.

## How the door works

The door is a real authored exit (`door` in room 5625) with a lock of
difficulty 255. It points back at its own room only because an exit needs a
destination; nobody ever arrives by it.

`RegisterRoomHooks` installs four hooks in `internal/rooms`
(`routing_hooks.go`), which cannot import this package:

- `rooms.SetExitRouter(RouteDoor)`: `usercommands.Go` asks the router before
  moving. The router lists every lodging the player may enter: their own
  (labelled `home`) and every house that has them as a guest (labelled by
  owner name). One goes straight there, with the authored lock skipped;
  several come back as `ExitRoute.Choices` and `Go` asks with a menu
  (`Whose lodging? [home/Alice/cancel]`, prefix answers work), or takes a
  destination pre-picked by the `visit <name>` command; none is refused. Everything else in
  `Go` (combat, stamina, sneaking, companions, quest `room_enter`, visited
  rooms, narration) runs unchanged. `unlock` ("open door") delegates to `Go`,
  `picklock` refuses before the lockpicks check, and `look door` shows the
  room's `door` noun instead of peering through.
- `rooms.SetEntryGuard(GuardEntry)`: `rooms.MoveToRoom` refuses to place a
  player in a unit room of a house they neither own nor are a guest of,
  however they arrive. Staff
  (`users.RoleAdmin`) may enter any unit. A refused LOGIN placement is
  redirected to the building's door room.
- `rooms.SetPrivateRoomCheck(IsUnitRoom)`: the loot goblin never picks a unit
  room, and `rooms.SaveRoomTemplate` refuses one (the builder would otherwise
  write a lodger's exits and text into git-tracked content).
- `rooms.SetRoomOverlay(ApplyOverlay)`: see below.

Mobs never consult the router; the lock keeps them out of the door, and unit
rooms have no inbound exits except a house's own doorways.

## The overlay

Exits, title, description, nouns and coordinates are `instance:"skip"` room
fields: an instance save never writes them and every load restores them from
the template. So the house record is their only source of truth, and
`ApplyOverlay` lays it over a unit room every time the room is built from
disk (`rooms.LoadRoomInstance` calls the hook last) and whenever the house
changes while the room is loaded. For a room of a house it:

- adds a doorway exit for every link touching the room;
- for any room but the entry: removes the door back to the alley and its
  noun, and uses the building's `extension_title` and `extension_description`;
- uses the owner's own description, if they wrote one;
- places the room at the entry room's authored coordinates plus its offset
  along the doorways, so the map draws the house as its doorways say.

It is idempotent and never loads a room (entry coordinates are captured from
the templates at load, `unitCoords`). Vacant units are left as their template
made them. After a deed is used, the house's map is rebuilt from both the
entry and the new room (`events.RebuildMap`). The lodgings zone is
`non_cartesian`, and each house is its own crawl component (the `door` exit
has no map direction), so houses never collide with each other on the map.

Unit rooms are ordinary authored rooms, so their floors persist through the
normal room instance save and are already walked by the bauble sweep. The
local smoke-test instance wipe clears those floors in development, as it does
every room's.

## The landlord's list

The shop system prices an item once for everybody; an extension costs a
multiple of what THIS lodger has paid, so the list is built per player by
`Offers` and rendered by the `list` command in the shop table style
(`internal/usercommands/housing_shop.go`). `buy home|deed|voucher` (also
`buy room`, which means a home to someone without one and an extension to an
owner) goes to `Buy` before the ordinary shop; both refuse below the faces
light band like every shop (`actions.ShopSightRefusal`). The alley carries a
room `lamp: 52` (its lantern) so this works at night. `ask hobb ...` reaches
the same `Buy` through the behavior-tree action `buy_housing`, and
`housing_terms` explains the list in words.

- **Home** (`Purchase`): refuses unknown building or tier, an existing home in
  this building (one per ACCOUNT, shared by alts), standing below
  `min_rep_tier`, carried gold plus bank below the price, no vacant unit.
- **Extension deed** (`buyExtension`): needs a home, room below `max_rooms`,
  a vacant unit, and the price. It writes `RoomsPaid += price` BEFORE the
  deed is handed over, so an unused deed still raises the next price and
  deeds cannot be stockpiled cheaply. The deed carries
  `items.Item.BoundUserId` and only works for that account.
- **Voucher** (`buyRedecorate`): needs a home and the flat price. Unbound.

Gold is always carried first, then the bank; `events.EquipmentChange` and
`events.ItemOwnership` are queued.

## Using the items

`usercommands.Use` hands `use <words>` to `UseItem` when a leading run of
words names a housing item in the backpack (unless the whole text already
names an ordinary item). Both items only work inside the user's own house.

- **Deed**: `use deed <direction>` builds straight away; `use deed` alone
  opens a prompt listing only the free directions (no exit that way, no room
  of the house in that cell) plus cancel. Refusals (wrong account, wrong
  place, bad or taken direction, a cell already holding one of the house's
  rooms, room cap, no vacant unit) never use up the deed.
- **Voucher**: `use voucher <text>` or `use voucher` then a prompt for the
  text, then a yes/no confirmation with a preview. The text is cleaned
  (`util.EscapeAnsiTags`, whitespace folded) and must be
  `DescriptionMinLen` to `DescriptionMaxLen` characters. A later voucher in
  the same room overwrites the earlier text. The voucher is re-checked in the
  backpack after the confirmation.

## Guests

- **Buying** (`buyGuestKey`): needs a home and fewer than `max_guests`
  guests. The key carries `items.Item.HouseKeyOwner` (the buyer's account).
  Nothing is recorded until the key is used, so an unused key costs nothing.
- **Using** (`useGuestKey`, from `UseItem` before the in-house check): only at
  the building's door room, not by the owner, not a second time by an existing
  guest, and not past `max_guests` (checked again at the lock, so keys bought
  early cannot overfill a house). The guest is written to the house, then the
  key is spent; the owner is told if online. Every refusal keeps the key.
- **Access** is the house record alone: owner or guest, by account. A guest
  walks every room, arrives at the entry room, and leaves by the door. Guests
  cannot use deeds or vouchers there (owner-only), and cannot invite anyone.
- **Revoking** (`Revoke`, `house revoke <name>`, whole name or unique prefix)
  and **leaving** (`Leave`, `house leave <owner>`) write the house first, then
  put the player outside the building's door if they are online inside
  (`ejectIfInside`). A player offline inside is redirected at login by
  `GuardEntry`.
- Commands live in `internal/usercommands/house.go`: `house` (own lodging,
  guests, lodgings you may visit), `house revoke`, `house leave`, and `visit`.

## Adding capacity or a building

Run `tools/housing_units.py` to generate blank unit rooms (it refuses to
overwrite and continues x after the highest existing unit, so authored cells
never repeat), then list the new ids in the building's `unit_rooms`. A new
building also needs a door exit in a street room (locked, no map direction),
a landlord mob with a tree calling the two actions, its two items, and a
`housing_buildings/<id>.yaml`. Record new rooms in the lighting goldens
(`-update-lighting-parity`, `-update-lighting-daycycle`) and confirm the diff
only adds them.

## Gotchas

- **A house record holds no items.** `TestHouseHoldsNoItemsYet` fails the day
  a `House` field reaches an `items.Item`. When it does, register the store
  with the bauble sweep (a `WalkItems`, a live source in `bauble_sweep.go`,
  a root in `item_walker_guard_test.go`) or baubles kept there are pruned.
- **Keep the door exit locked and without a `mapdirection`.** The lock is the
  only thing stopping mobs; a map direction would make the mapper draw a
  self-loop and join every house into one crawl.
- **Never author a direction exit in a unit template.** Validation panics on
  one, because the overlay owns those exits.
- **Don't edit a lodger's room in the builder.** `SaveRoomTemplate` refuses
  and logs; the builder change stays in memory until the room reloads.
- **A deed from one building used in another** is refused. Items are shared
  between buildings only if a future building lists the same item ids.
- **Holding is not deletion.** A held room stays held until its file is
  fixed or removed by hand and the server reloads.

## Dependencies

`configs`, `events`, `exit`, `factions`, `fileloader`, `items`, `messaging`,
`mobs`, `mudlog`, `opinions`, `rooms`, `term`, `users`, `util`, plus YAML.

## Consumers

`main.go` (boot), `internal/behaviortree` (`buy_housing`, `housing_terms`),
`internal/usercommands` (`list`, `buy`, `use`, via `housing_shop.go`; `house`
and `visit`, via `house.go`), and
through the room hooks `internal/rooms` and `internal/usercommands`.
