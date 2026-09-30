# Housing Context

## Purpose

`internal/housing` owns persistent player housing: the authored lodging
buildings, the living-state record of who owns which rooms, the landlord's
list (homes, extension deeds, redecorating vouchers, guest keys, container
and strongbox deeds), the use of those items, what lies on a lodging's floors
and in its containers, and the routing that sends each lodger through a
building's one shared door to their own rooms.

Four buildings ship, one per city and one in the wilds, and they run
independently: a player may own one home in each (one per ACCOUNT per
building), each gated on its own city's standing (the wild one on none), with
its own rooms, deeds, guests, floors and containers.

- **The Back Court lodgings** (`back_court_lodgings`), New Plymouth Common.
  Hobb Pennock (mob 9801), a bored letting clerk for Crewe Lettings, sits in
  Pennock's Alley (room 5625, west of the Back Court 5615). Standing:
  `np_commonfolk` Warm. Items 55-59; units 6470-6569 (zone New Plymouth
  Lodgings, plane 14).
- **The Quillhouse** (`quillhouse`), the Confluence. Aubric Sallow (mob 9802),
  under-clerk to Madam Orla Pardew, sits in Quill Court (room 6570, west off
  Hall Lane 6144 behind the Municipal Hall). Standing: `margin` Warm (The
  Margin Notation, +15, is enough). Items 60-64; units 6571-6670 (zone
  Confluence Lodgings, plane 15).
- **The Burrows** (`the_burrows`), Thornwall. Brannoc Tull (mob 9803), Torvan
  Cresk's rude doorman, sits at the Burrow Mouth (room 6671, east off the
  smugglers' escape shaft 499 past Torvan's operations room, in the tunnels
  under the city; the route passes the smugglers' lookout 247). Standing:
  `thornwall_citizens` Warm (one city quest, +15, is enough). Items 65-69;
  units 6672-6771 (zone Thornwall Burrows, plane 16).
- **The Hollow Oak** (`hollow_oak`), in the wilds of the South Road between
  Thornwall and the Confluence. Old Brock (mob 9804), a talking badger, keeps
  the ledger on a stump beneath an enormous oak (room 6773), reached east from
  the Shepherd's Reach 6045 by the Combe Track 6772; the Lake & Ladle 6044,
  the Shepherd's Reach and the Long Furlong 6046 all point to the tree. No
  standing check at all (no faction), and every price is a quarter of a
  city's. Items 70-74; units 6774-6873 (zone Hollow Oak, plane 17).

**Every housing payment is a gold sink.** Gold is only ever deducted
(`chargeGold`) and never credited anywhere; nothing is refunded. Every item a
landlord sells is registered with `items.SetNeverBought` (`registerNeverBought`,
on every load), and the sell action refuses it at its one chokepoint
(`actions.sellOneToMerchant`), so no merchant pays for it whatever its value.
The item loader gives every item a value (an authored 0 is replaced by an
automatic one), so value alone never protected them. Boot validation refuses
a landlord with a shop (he buys nothing) or with gold of his own (nothing to
steal or loot), and a housing item that is salable or has vendor categories.
Players can still give, trade or auction unbound housing items to each other;
that moves gold between players, never out of the sink.

Each landlord sells from a list (prices as authored; the three city
buildings ship the numbers below, and the Hollow Oak a quarter of each: 125,
deeds from 375, 125, 25, 63 and 125):

- **Home**, 500 gold, to players the Common Quarter vouches for
  (`np_commonfolk` standing Warm or better; each building names its own
  faction). One per account.
- **Room Extension Deed** (item 55; Quillhouse 60), priced at 3 times everything the lodger
  has paid for rooms so far: 1500 after the home, then 6000, then 24000.
  Bound to the buying account. Used inside the lodging toward a direction, it
  knocks a new room through (`use deed north`, or `use deed` for a menu).
  One unused deed at a time; a lost one is replaced free.
- **Redecorating Voucher** (item 56; 61), 500 gold. Used in a lodging room, it
  replaces that room's description with text the lodger writes.
- **Guest Key** (item 57; 62), 100 gold, made out to the buyer's house. Given to
  a friend and used at the door once, it adds them to the house's guest list
  (up to 5). The owner revokes with `house revoke <name>`.
- **Container Deed** (item 58; 63), 250 gold. Used in a lodging room with one
  word (`use container deed mug`, or a prompt asks), it places a container
  of that name there. Anyone let in can look in, put into and get from it.
- **Strongbox Deed** (item 59; 64), 500 gold. The same, but only the owner (and
  the owner's companions) can open what it places. Each ROOM holds at most 6
  containers of both kinds (`max_containers_per_room`); the house as a whole
  has no limit.

Anything left on a lodging's floor, and in its containers, is kept in the
house file and survives restarts, crashes and a wipe of `rooms.instances/`.

Each building's homes and extensions come from its own pool of 100 blank
unit rooms. A house holds at most 8 rooms. Every item belongs to one
building: a deed, voucher or container deed works only in a lodging of the
building that sold it, and a guest key only at its door.

## Two kinds of data

**Buildings are authored content.** One YAML per building in
`<DataFiles>/housing_buildings/<building_id>.yaml`, git-tracked, loaded with
`fileloader.LoadAllFlatFiles`. A bad file PANICS at boot like any authored
content, including world checks against room TEMPLATES (not live rooms,
which carry overlays after a reload): the door room exists, its door exit
exists and is LOCKED, the landlord mob, the faction (if any) and all five items exist, every
unit room exists, leads back to the door room through an exit of the same
name, belongs to one building only, spawns no mobs, and has no authored
north/south/east/west/up/down exit (extensions place those). A building also
carries the words the shared code speaks with, all required: `proprietor`
("the Widow"), `vouched_by` ("the Common Quarter"), `standing_hint`,
`location` (where its door is) and `outside_text` (what a guest put out of a
lodging sees). A tier must be exactly one room.

**Standing is optional.** A building with no `faction` (and so no
`min_rep_tier`, which is invalid without one) checks no standing:
`Building.ChecksStanding` is false, anybody who can pay is welcomed
(`welcomes`), `vouched_by` and `standing_hint` are not required, and the
terms end with the `terms.open` line instead of `terms.ready` or
`terms.not_vouched`. The Hollow Oak is the one such building.

**Each landlord has his own voice.** Everything a landlord says through say
comes from `voice.go`: `defaultLines` holds every line (Hobb's words, the
default for any landlord), keyed (`terms.pitch`, `home.welcome`,
`deed.sold`, ...), and a building's `voice:` map overrides any of them.
The note beside an available home on the list is a line too
(`list.home_note`). Lines use `{placeholders}`: the building's own words (`{proprietor}`,
`{Proprietor}`, `{vouched_by}`, `{Vouched_by}`, `{standing_hint}`, `{name}`,
`{Name}`, `{door}`, `{Door}`) and each key's own (`linePlaceholders`: `{price}`,
`{tier}`, `{what}`, ...). `Building.Line(key, name, value, ...)` renders a
line. `Validate` rejects an unknown key, a placeholder the key does not
provide, an empty line and a semicolon. A new spoken line must go into
`defaultLines` and be said through `Line`, or `TestVoice_EveryLineIsConsistentAndUsed`
fails. Brannoc (the Burrows) and Old Brock (the Hollow Oak) override nearly every
line; Aubric (the Quillhouse) overrides a few.

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
- every change (purchase, deed bought, deed used, voucher used, key used,
  container placed, contents captured) builds a new record with
  `House.clone`, writes it, and only then publishes it to the registry and
  takes gold or items; the player is then saved at once (`saveUser`), so a
  crash cannot give back what was spent while the house keeps what it bought.

A house records its rooms (`RoomIds`, the first is the entry), the doorways
between them (`Links`, one entry per doorway, the way back implied), the
owner's descriptions by room (`Descriptions`) and what was paid for rooms
(`RoomsPaid`; houses from before extensions have it 0 and `Spent` falls back
to `PricePaid`), the extension deeds sold (`DeedsIssued`; filled in from
`RoomsPaid` for older files by `normalizeDeeds`), the guests let in with keys
(`Guests`, by account, with the character name that used the key for lists
and revoking), the containers placed by deeds WITH their contents
(`Containers`: room, name, `owner_only`, items, gold), and what lies on each
room's floor (`Floors`: room, items, stash, gold).

A file is named by its entry room, not its owner, so a file that cannot be
read still says which room it guarded. That room is **held**: never sold and
never entered (except by staff) until someone repairs the file. Which OTHER
rooms it owned is unknown, so the whole building is **frozen** (`Frozen`): it
sells no home and builds no extension, since any unit that looks vacant may
hold that lodger's things. The quarantined file (`<entry>.yaml.corrupt-<time>`)
stays in the building's folder, and every load treats it as an open case
(`quarantinedEntry`): the hold and the freeze last across restarts until
staff put a repaired `<entry>.yaml` back AND move the `.corrupt` file out of
the folder, then reload. A readable file that disagrees with the world (unknown
building, a room that is not a unit, a room listed twice or already owned, a
second house for the same owner, a doorway outside the house or two doorways
in one direction, a room no doorway reaches or two rooms in one place, a
guest who is the owner or listed twice, a container outside the house, badly
named or named twice in one room, or a floor recorded twice or outside the
house: `checkLinks`) is left in place and all its rooms held, and its owner
is not sold another home meanwhile (`heldOwners`). `HeldRooms` lists them.

## Files

- **housing.go**: `Building`, `Tier`, `House`, `RoomLink`, `HouseContainer`,
  `ParseRepTier`, `ParseDirection`, building validation, doorway geometry
  (`exitsOf`, `offsets`, `checkLinks`), `House.WalkItems`.
- **registry.go**: the in-memory registry, `LoadDataFiles`, building world
  validation, lookups, unit coordinates.
- **persistence.go**: house files: `saveHouse`, `loadHouses`, quarantine.
- **door.go**: `RegisterRoomHooks`, `RouteDoor`, `GuardEntry`.
- **overlay.go**: `ApplyOverlay` (layout, containers and floor, on load) and
  `applyLayout` (layout only, for live rooms).
- **offers.go**: the landlord's list: `Offers`, `MatchOffer`, `Buy`.
- **purchase.go**: `Purchase` (a home), `PurchaseResult`.
- **use_items.go**: `UseItem` (deeds and vouchers).
- **terms.go**: `DescribeTerms`, the landlord's answer in words.
- **voice.go**: `defaultLines`, `Building.Line`, voice validation.
- **guests.go**: guest keys and the guest list: `Revoke`, `Leave`,
  `HousesOwnedBy`, `GuestOf`, `BuildingForDoor`, ejection.
- **containers.go**: container and strongbox deeds (`useContainerDeed`),
  loading and capturing floors and contents (`hydrateContainers`,
  `hydrateFloor`, `Capture`, `captureHouseAt`, `captureAllLoaded`,
  `AfterUserCommand`, `AfterMobCommand`, `CaptureOnSave`), and strongboxes
  (`OpenStrongboxesForUser`, `OpenStrongboxesForMob`).

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
func Revoke(ownerId int, name string) (Guest, []string, error)     // from every home of the owner's; returns building names
func Leave(userId int, ownerName string) (House, []string, error)  // from every home of that owner's
func AllBuildings() []Building
func LandlordName(b Building) string
func ItemIssuedHere(itemId int, roomId int) bool   // item sold by the building roomId (unit or door) belongs to
func AllHouses() []House
func VacantUnits(buildingId string) []int
func HeldRooms() map[int]string
func Frozen(buildingId string) (string, bool)          // why a building sells no rooms
func (h House) Outstanding(b Building) int            // extension deeds sold, not yet used

func Offers(user *users.UserRecord, buildingId string, tierId string) []Offer
func MatchOffer(request string, ownsHome bool) (string, bool)   // OfferHome, OfferExtension, OfferRedecorate, OfferGuestKey, OfferContainer, OfferStrongbox
func Buy(user *users.UserRecord, say func(string), buildingId string, tierId string, key string)
func Purchase(user *users.UserRecord, say func(string), buildingId string, tierId string) PurchaseResult
func DescribeTerms(user *users.UserRecord, say func(string), buildingId string, tierId string) bool
func UseItem(user *users.UserRecord, room *rooms.Room, itm items.Item, args string, rest string) bool

func RouteDoor(userId int, fromRoomId int, exitName string) (rooms.ExitRoute, bool)
func GuardEntry(userId int, toRoomId int) (bool, int, string)
func ApplyOverlay(r *rooms.Room)

func Capture(r *rooms.Room) bool                            // live floor and containers -> house record, saved if changed
func CaptureOnSave(r *rooms.Room)                           // the rooms.RoomSaveHook
func AfterUserCommand(userId int, roomBefore int)           // world.go, after every player command
func AfterMobCommand(mobInstanceId int, roomBefore int)     // world.go, after every mob command
func OpenStrongboxesForUser(userId int, roomId int) func()  // usercommands.TryCommand; call the result when done
func OpenStrongboxesForMob(mobInstanceId int, roomId int) func() // mobcommands.TryCommand
func (h *House) WalkItems(visit func(*items.Item))          // the bauble sweep's housing source
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

`ApplyOverlay` also makes the room's containers exactly the house's
(`hydrateContainers`, strongboxes sealed), contents included, and puts the
recorded floor into the room (`hydrateFloor`). A room with no recorded floor
(a house written before floors were recorded) keeps what its instance save
held until its first capture adopts it. A vacant unit loses any containers
but keeps its floor. Runtime changes to a house (a deed or voucher used)
re-lay live rooms with `applyLayout`, which leaves containers and floors
alone: their live state is newer than the record until the next capture. A
room newly added to the house, and every room of a newly bought home, is
recorded with an empty floor and gets the full overlay, so it starts empty
whatever an old instance save or tenancy left in it.

Boot loads every room (validation, the shop prewarm) before any house is
known, so `LoadDataFiles` lays every house over its loaded rooms afterwards
(`applyAllOverlays`). On a data reload it first captures every loaded house
room (`captureAllLoaded`); a room whose capture could not be written gets the
layout only, so the reload never replaces newer live state with an older
file.

## The landlord's list

The shop system prices an item once for everybody; an extension costs a
multiple of what THIS lodger has paid, so the list is built per player by
`Offers` and rendered by the `list` command in the shop table style
(`internal/usercommands/housing_shop.go`). `buy home|deed|voucher|key|container|strongbox` (also
`buy room`, which means a home to someone without one and an extension to an
owner) goes to `Buy` before the ordinary shop; both refuse below the faces
light band like every shop (`actions.ShopSightRefusal`). The alley carries a
room `lamp: 52` (its lantern) so this works at night. `ask hobb ...` reaches
the same `Buy` through the behavior-tree action `buy_housing`, and
`housing_terms` explains the list in words.

- **Home** (`Purchase`): refuses unknown building or tier, an existing home in
  this building (one per ACCOUNT, shared by alts), a held house of the same
  owner, a frozen building, standing below `min_rep_tier`, carried gold plus
  bank below the price, and no vacant unit beyond those promised to unused
  extension deeds. A tier is exactly one room (`Validate`); more come from
  deeds, which lay the doorways.
- **Extension deed** (`buyExtension`): needs a home, a building not frozen,
  room below `max_rooms`, the price, and a vacant unit beyond those already
  promised to every lodger's unused deeds (`outstandingLocked`), so no deed is
  ever sold that could not be used. It writes `RoomsPaid += price` and
  `DeedsIssued++` BEFORE the deed is handed over, so an unused deed still
  raises the next price. While a lodger has an unused deed (`Outstanding`) no
  other is sold: if they carry it they are told to use it, and if they do not
  (lost, stored, junked) a replacement is handed over free. A deed is
  honoured only while one is outstanding, so of two copies only the first
  used builds a room. The deed carries `items.Item.BoundUserId` and only works
  for that account.
- **Voucher** (`buyRedecorate`): needs a home and the flat price. Unbound.
- **Container and strongbox deeds** (`buyContainerDeed`): need a home, the
  flat price, and a free corner: the rooms' remaining per-room space
  (`containerSpace`) must exceed the container and strongbox deeds the buyer
  already carries. Unbound, since they only work in their holder's own
  lodging. `MatchOffer` reads the strongbox
  words (strongbox, lockable, locked, lock, private, safe) first, then the
  container words (container, box, storage), then the rest, so "container
  deed" is never read as an extension deed. The list's footer is built from
  the offer keys.

Gold is always carried first, then the bank; `events.EquipmentChange` and
`events.ItemOwnership` are queued. Asking the landlord (`buy_housing`) only
sells on a plain request: after lead-ins ("please", "I'd like to") the first
word is buy or purchase, nothing negates it (not, don't, never, no,
nothing...), and it names an offer (`isPlainPurchase`, `MatchOffer`).
Anything else that mentions buying, a question above all ("how much to buy
a room?"), is answered with the terms, which say what to type.

## Using the items

`usercommands.Use` hands `use <words>` to `UseItem` when a leading run of
words names a housing item in the backpack (unless the whole text already
names an ordinary item), trying the LONGEST run first, so `use container deed
mug` is the container deed named mug, and preferring an item this player may
use (not bound to another account). Every item but the guest key only works
inside the user's own house.

- **Deed**: `use deed <direction>` builds straight away; `use deed` alone
  opens a prompt listing only the free directions (no exit that way, no room
  of the house in that cell) plus cancel. Refusals (wrong account, wrong
  place, a deed already honoured, a frozen building, bad or taken direction,
  a cell already holding one of the house's rooms, room cap, no vacant unit)
  never use up the deed.
- **Voucher**: `use voucher <text>` or `use voucher` then a prompt for the
  text, then a yes/no confirmation with a preview. The text is cleaned
  (`util.EscapeAnsiTags`, whitespace folded) and must be
  `DescriptionMinLen` to `DescriptionMaxLen` characters. A later voucher in
  the same room overwrites the earlier text. The voucher is re-checked in the
  backpack after the confirmation.

## Containers

- **Placing** (`useContainerDeed`): `use container deed <word>` or `use
  strongbox deed <word>`; with no word, a prompt asks for one. The name is one
  word of 2 to 16 lower-case letters (upper case is folded), not reserved
  (`reservedContainerNames`: directions, door, all, gold, ...), and not an
  exit, noun or container already in the room, and the room must hold fewer
  than `max_containers_per_room`. Checked again under the registry lock with
  the house re-read, then written, then the deed spent and the empty
  container added to the live room. Refusals keep the deed.
- **Using**: an ordinary `rooms.Container` in the live room, so `look in`,
  `open`, `put`, `get` and `remove ... from` work as on any container
  (`look` strips at/in/into/inside and the/my; `open`, an alias of `unlock`,
  shows an unlocked container's contents in a unit room; `remove X from Y`
  is a get when Y is a container here that the player can see).
- **Where floors and contents live**: in the house record, never only in the
  room's instance save. `ApplyOverlay` copies them in on load; `Capture`
  copies a changed room back and saves it, comparing items as the YAML the
  file would hold, so any change to a saved field counts and nothing unsaved
  does. world.go calls `AfterUserCommand` after every player command and
  `AfterMobCommand` after every mob command, each with the room the actor
  stood in before the command; they capture every loaded room of that house
  and of the house the actor is in now (`captureHouseAt`), so dropping
  something and walking out in one line, or something landing in the next
  room, is caught. When a player's command changed anything, the player is
  saved too (`saveUser`), so an item is never both on the floor and still in
  their saved pack after a crash. The room save hook (`CaptureOnSave`, via
  `rooms.SetRoomSaveHook`) is the backstop on every autosave, on
  `SaveAllRooms` (shutdown, copyover) and when a room is unloaded, for
  anything that landed without a command (a corpse rotting). Item UUIDs are
  not written to disk, so `loadOneHouse` mints them (`Item.Validate`) as the
  room and user loaders do.
- **Strongboxes** are sealed (`rooms.Container.Sealed`) and kept under a lock
  (`rooms.SealedLockDifficulty`) at all times, so every engine path that
  honours container locks (look, get and `get all <container>`, put, use,
  tab completion, the AI companion) refuses them, whatever word a player
  uses for the name, without saying what or how much is inside. The
  paths that could force a lock refuse a sealed one outright: picklock,
  unlock with a key (players and mobs), steal and plant
  (`Container.IsSealedShut`). The owner's own command, and a command of a
  mob charmed by the owner, unlocks the strongboxes of the room the actor
  stands in for that one command: `usercommands.TryCommand` defers
  `OpenStrongboxesForUser` right after alias expansion, and
  `mobcommands.TryCommand` defers `OpenStrongboxesForMob`. The game loop
  runs one command at a time, so nobody else acts while one is open. A
  nested command finds them open already and leaves the shutting to the
  outer one. Staff may open them too.
- **Baubles**: private rooms (all lodgings) never offer bauble searches
  (`actions.baubleRoomAllowed`), as character rooms do not: a bought, safe
  room with many searchable containers would be a farm. A found bauble
  dropped in a lodging is an ordinary item, and a find left lying in a
  private room from before never expires (`rooms.removeUntakenBaubles`).

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
  and **leaving** (`Leave`, `house leave <owner>`) cover every lodging of
  that owner's in every city, and write each house first, then
  put the player outside the building's door if they are online inside
  (`ejectIfInside`). A player offline inside is redirected at login by
  `GuardEntry`.
- Commands live in `internal/usercommands/house.go`: `house` (own lodging,
  guests, lodgings you may visit), `house revoke`, `house leave`, and `visit`.

## Adding capacity or a building

Run `tools/housing_units.py` to generate blank unit rooms (it refuses to
overwrite and continues x after the highest existing unit, so authored cells
never repeat), then list the new ids in the building's `unit_rooms`. The
zone folder must be the zone name in lower case with underscores, or the
rooms will not load. A building's units read like it through `--template`
(`tools/housing_unit_templates/`). A new building also needs: a door room
with a locked door exit (no map direction), a `lamp` so the list can be read
at night, and an exit into it from a street; a new plane for its units; a
landlord mob with a tree calling the two actions for its building id; its
own five items (so none is honoured in another building); and a
`housing_buildings/<id>.yaml` with the wording fields. The landlord needs
`gold: 0` and no shop. The Quillhouse and the Burrows are worked examples of
adding a city. Record new rooms in the lighting goldens
(`-update-lighting-parity`, `-update-lighting-daycycle`) and confirm the diff
only adds them.

## Gotchas

- **A house record holds items** (floors and container contents). It is a
  bauble sweep store: `House.WalkItems`, the `housing` live source in `bauble_sweep.go`
  (walking `AllHouses`), a root in `item_walker_guard_test.go`, and entries
  for `houses`, `roomHouse` and `ownerHouse` in `itemStoreVars`
  (`bauble_sweep_guard_test.go`). A new item-holding field must be reached by
  `WalkItems` or `TestItemWalkersVisitEveryItemField` fails.
- **Never write a unit room's containers or floor from outside housing
  except through ordinary commands.** The record is the source of truth: the
  next load replaces the room's with the record's, and `Capture` puts back
  any recorded container found missing (sealed again if a strongbox).
- **Refresh live rooms with `applyLayout`, not `ApplyOverlay`.** The full
  overlay reloads containers and floor from the record and would throw away
  anything changed since the last capture.
- **A strongbox is only as safe as the lock checks.** A new command that
  reads or moves a room container's contents must honour
  `Container.Lock.IsLocked()` (as get, put, look and use do) or refuse
  `Container.IsSealedShut()`. Never unlock a sealed container anywhere but
  `openStrongboxes`.
- **Holding and freezing are not deletion.** A held room stays held, and a
  frozen building sells nothing, until the file is fixed or removed by hand
  and the data reloaded.
- **Keep the door exit locked and without a `mapdirection`.** The lock is the
  only thing stopping mobs; a map direction would make the mapper draw a
  self-loop and join every house into one crawl.
- **Never author a direction exit in a unit template.** Validation panics on
  one, because the overlay owns those exits.
- **Don't edit a lodger's room in the builder.** `SaveRoomTemplate` refuses
  and logs; the builder change stays in memory until the room reloads.
- **A deed from one building used in another** is refused. Items are shared
  between buildings only if a future building lists the same item ids.

## Dependencies

`configs`, `events`, `exit`, `factions`, `fileloader`, `gamelock`, `items`,
`messaging`, `mobs`, `mudlog`, `opinions`, `rooms`, `term`, `users`, `util`,
plus YAML.

## Consumers

`main.go` (boot), `world.go` (`AfterUserCommand`, `AfterMobCommand`),
`bauble_sweep.go` (the `housing` live source), `internal/behaviortree`
(`buy_housing`, `housing_terms`), `internal/usercommands` (`list`, `buy`,
`use`, via `housing_shop.go`; `house` and `visit`, via `house.go`; the
strongbox window in `TryCommand`; `unlock` for `open`), `internal/mobcommands`
(the strongbox window), and through the room hooks `internal/rooms` and
`internal/usercommands`. `internal/rooms` (`Container.Sealed`) and
`internal/actions` (steal, plant, `baubleRoomAllowed`) honour strongboxes and
private rooms without importing housing.
