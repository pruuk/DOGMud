# baubles Context

## Purpose

`internal/baubles` is the engine side of bauble loot: small, non-usable,
non-wearable objects that exist only to be sold for gold, found by `search`
(and later by theft) and named by a language model from the room they were
found in. Plan and phases: `docs/baubles/implementation-plan.md`.

Every bauble is ONE item id (`items.BaubleItemId`, 900, file
`items/other-0/900-curious_trinket.yaml`) plus `Item.Bauble`, the id of a
record in this package's catalog. The catalog is the bauble database: name,
description, material, tier, value, weight, where it was found, whether it
was stolen, and how its text was generated.

## Files

- **tiers.go**: the value ladder. Three fixed tiers, cheapest first.
- **weight.go**: weight bounds in pounds and the scale given to the model.
- **reply.go**: the strict JSON schema for a generation reply, parsing, and
  the numeric limits.
- **record.go**: `Record`, its statuses, sources and generators, and `View`
  (what the item layer shows, including the retired text).
- **store.go**: the on-disk format: shards of `ShardSize` records
  (`catalog-NNNN.yaml`; ids start at 1, so shard 0 is B0000001 to
  B0000500, `shardOf`) plus `meta.yaml` with the next id. A record read from
  a shard it does not belong in (a catalog written before shards began at
  id 1) loses to the copy in its own shard, and `Load` rewrites both shards
  so it ends up in its own only.
- **catalog.go**: the in-memory catalog, write-through to disk, and the
  resolver installed into `internal/items`. Disk writes (`persistShard`,
  `persistMeta`) snapshot under the read lock and write outside it, ordered
  by a separate write mutex, so `Get` never waits on the disk. A per-user
  index of return credits backs `ReturnCredits`. `Prune(now)` drops records
  gone (sold or vanished: `Record.goneAt`) longer than `KeepDuration()`
  (`Balance.BaubleCatalogKeepDays`, 30, at least 7 because the sales stats
  read a week), except any with a return credit; a retired record goes only once it too is
  sold or vanished, since until then the bauble can still be in a pack. It runs at `Load` and at
  every `SaveAll` and rewrites only the `catalog-*` shards, so
  `corpus.promoted.yaml` survives.
- **fallback.go**: `GenericTrinket`, what every find is when the model does
  not name it: "Trinket", a simple description, value and weight at random
  within the tier.
- **generate.go**: the generator seam (`SetGenerator`, `CurrentGenerator`),
  `GenRequest`, `GenResult`, `Generate`, `RecentNames`.
- **validate.go**: `CleanReply` (the text checks the schema cannot make) and
  `PlainText`.
- **mint.go**: `Place`, `NewPlace`, `MintOpts`, `Mint`.
- **sales.go**: `MarkSold`, `SalesSince`.
- **goods.go**: stolen goods that are not baubles, a merchant chest's
  (`internal/merchantchests`), which carry their theft on the item
  (`items.Item.StolenAt`, `StolenZone`, `StolenBy`, `StolenFromMob`) instead
  of a record, on the same heat rules: `GoodsHot` (stolen within
  `HeatDuration`), `GoodsHotIn` (hot, and in the `HeatArea` of the zone they
  were taken in). `ItemIsHotIn` answers for them too, so storage and the
  auction house refuse them where they are hot with no code of their own.
- **theft.go**: `Theft`, `MarkStolen` (a household's bauble taken),
  `MarkHousehold`, `MarkVanished` (left untaken too long), `UntakenLimit`;
  after a theft (Phase 6c): `Record.Hot` (stolen within
  `BaubleStolenHeatHours` and not returned since: when the owner may
  recognise it), `Record.HotIn` (hot, and in the heat area of the theft's
  zone, `StolenZone`: where it cannot be sold or stored), `HeatArea` (a
  zone's area: its `BaubleHeatAreas` group, else itself), `HeatDuration`,
  `ItemIsHotIn`, `Record.RecognizedSinceTheft`, `MarkRecognized`,
  `MarkReturned` (cools it; the first credited return is kept for good,
  with its game round in `ReturnCreditRound`), `ReturnCredits(userId,
  faction, sinceRound)` (a thief's credited returns per faction since a
  round, the oldest open catch's: they set the next return's share and are
  capped by the catches it has cost them), `Record.StolenGoods` (stolen and
  not given back since: what a fence pays its premium for), `MarkGiven`
  and `Record.GivenTo` (a player gave it to a mob that does not own it,
  `GivenToMob`; picked from that mob's pocket it is not the mob's stolen
  goods; a theft clears it).
- **admin.go**: `CatalogStats`, `Retire`, `Restore`, `Edit` (hand edits,
  checked like a model's answer), `ApplyRegenerated`, the prompt-preview
  seam (`SetPromptPreview`, `PreviewPrompt`) and `LooksLikeId`.
- **window.go**: the per-room roll windows and per-feature search claims (in
  memory), `FeatureSearchable`, `ClaimFeatureSearch`, `WindowState`,
  `ResetWindow`.
- **find.go**: `RollFind` (the search roll: window, chance, tier; no minting),
  `PickTier`, `ZoneExcluded`, `RevealDelay`, and the settings read from
  `configs.Balance.Bauble*`.

## API

```go
type ValueTier string // TierCheap, TierAverage, TierRare
type ValueRange struct{ Min, Max int }

func Tiers() []ValueTier
func ParseTier(s string) (ValueTier, bool)
func (t ValueTier) Valid() bool
func (t ValueTier) Range() ValueRange
func (t ValueTier) ClampValue(v int) int
func (t ValueTier) RollValue(randn func(n int) int) int
func (t ValueTier) PromptLine() string
func (r ValueRange) Contains(v int) bool
func (r ValueRange) Clamp(v int) int

func ClampWeight(w float64) float64 // MinWeightLbs, MaxWeightLbs, DefaultWeightLbs
// WeightGuidance is the weight scale sent to the model.

type Reply struct { /* name, name_simple, description, material, weight_lbs, value */ }
type Limited struct { /* Reply, Tier, ProposedValue, ProposedWeight */ }

func ReplySchema() map[string]any
func ParseReply(content string) (Reply, error)
func ApplyLimits(r Reply, t ValueTier) Limited
func GenericTrinket(tier ValueTier, randn func(n int) int) Reply
func CleanReply(r Reply) (Reply, error) // ErrUnusableReply
func PlainText(s string) string

type GenRequest struct { /* Tier, Source, Place, RoomTitle, RoomDescription, RoomNouns, Container, ContainerDescription, TimeOfDay, RecentNames, FinderUserId, Victim */ }
type GenResult struct { /* Reply, Generator, Model, PromptVersion, Tokens, Moderated */ }
type GeneratorFunc func(ctx context.Context, req GenRequest) (GenResult, error)
func SetGenerator(fn GeneratorFunc, info func() GeneratorInfo)
func CurrentGenerator() (GeneratorInfo, bool)
func Generate(ctx context.Context, req GenRequest, randn func(n int) int) GenResult // blocks; never fails
func RecentNames(zone string, n int) []string

type Record struct { /* see record.go */ }
func (r Record) View() items.BaubleView

func Load() error        // boot, after items.LoadDataFiles()
func SaveAll()           // shutdown and copyover; retries failed writes
func SetDirForTest(dir string)
func Create(r Record) (Record, error)
func Get(id string) (Record, bool)
func Update(id string, change func(r *Record)) (Record, bool)
func Count() int
func Recent(n int) []Record
func KeepDuration() time.Duration
func Prune(now time.Time) int

type Place struct{ RoomId int; Zone, Region, Biome string }
func NewPlace(roomId int, zone string, region string, biome string) Place
type MintOpts struct{ Source Source; Place Place; FinderUserId int; Tier ValueTier; FoundIn string; Result *GenResult; Randn func(n int) int }
func Mint(o MintOpts) (items.Item, Record, error)

func MarkSold(id string, gold int, sellerUserId int) bool
type Theft struct{ ByUserId, RoomId, FromMob int; FromName, Faction, Zone string }
func MarkStolen(id string, t Theft, at time.Time) bool
func MarkHousehold(id string) bool
func MarkVanished(id string, at time.Time) bool
func UntakenLimit() time.Duration
func HeatDuration() time.Duration
func (r Record) Hot(now time.Time) bool
func (r Record) RecognizedSinceTheft() bool
func (r Record) HotIn(zone string, now time.Time) bool
func HeatArea(zone string) string
func ItemIsHotIn(itm items.Item, zone string, now time.Time) bool // baubles, and merchant-chest goods via GoodsHotIn
func GoodsHot(itm items.Item, now time.Time) bool
func GoodsHotIn(itm items.Item, zone string, now time.Time) bool
func MarkRecognized(id string, byUserId int, at time.Time) bool
func MarkReturned(id string, byUserId int, credited []string, at time.Time) bool
func ReturnCredits(userId int, faction string, sinceRound uint64) int
func (r Record) StolenGoods() bool
func MarkGiven(id string, mobId int) bool
func (r Record) GivenTo(mobId int) bool

func CatalogStats() Stats
func Retire(id string, admin string) error
func Restore(id string, admin string) error
func Edit(id string, field string, value string, admin string) (Record, error) // EditFields
func ApplyRegenerated(id string, res GenResult, admin string) (Record, error)
func SetPromptPreview(f PromptPreview)
func PreviewPrompt(req GenRequest) ([]string, bool)
func LooksLikeId(s string) bool
func SalesSince(t time.Time) (count int, gold int)

type FindOpts struct{ Place Place; UserId int; SkillFactor float64; Feature string; Household bool; Randn func(n int) int; Now time.Time }
func RollFind(o FindOpts) (tier ValueTier, found bool)
func BaseChance(biome string) float64
func ChanceFor(biome string, skillFactor float64) float64
func RevealDelay() time.Duration
func PickTier(randn func(n int) int) ValueTier
func PickPocketTier(randn func(n int) int) ValueTier
func ZoneExcluded(zone string) bool
func WindowState(roomId int, userId int, now time.Time) (used int, allowed int, reopens time.Time, open bool)
func FeatureWindowState(roomId int, userId int, feature string, now time.Time) (used int, allowed int, reopens time.Time, open bool)
func FeatureSearchable(roomId int, userId int, feature string, now time.Time) (ok bool, byYou bool, reopens time.Time)
func ClaimFeatureSearch(roomId int, userId int, feature string, now time.Time) bool
func ResetWindow(roomId int)
```

## Rules

- The ladder defaults to cheap 1 to 6, average 10 to 15, rare 40 to 200
  gold, with intentional gaps. It is read from `config.yaml`
  (`Balance.Bauble*Value`); configs validates it as a whole and resets all
  six numbers if the tiers overlap or invert. `TestTierRangesAreTheSpec` pins
  the defaults; `configs.TestBaubleShippedConfigMatchesDefaults` pins the
  shipped file to them.
- The game chooses the tier before generation; the model only picks a
  value inside it, and `ApplyLimits` clamps whatever comes back. Three sets
  of weights (cheap/average/rare): a find nobody keeps uses
  `BaubleTierWeight*` (70/25/5, `PickTier`); a household's find
  (`FindOpts.Household`: searched indoors with one of the household about,
  decided by the caller at the search) uses `BaubleHouseholdTierWeight*`
  (45/40/15); a bauble made for a pickpocket uses
  `BaublePickpocketTierWeight*` (50/40/10, `PickPocketTier`). A find that
  has to be stolen leans richer (`configs.TestBaubleStolenFindsLeanRicher`
  pins the defaults to that).
- Search rolls: the chance per roll is `ChanceFor(biome, skillFactor)`:
  `BaubleBiomeChancePct[biome]` (or `BaubleSearchChancePct`, 1%, for an
  unlisted biome) times `1 + BaubleSkillMaxBonus x skillFactor`, capped at
  100. Buildings 5%, streets 2 to 2.5%, wilderness 0.25%, deep water 0 (no
  window opened, no roll spent). Rolled to one part in a million.
  `BaubleRollsPerWindow` (2) rolls per room per `BaubleWindowMinutes` (60,
  real time, from the window's
  first roll), shared by the room unless `BaubleWindowPerPlayer`. Every roll
  spends the window, found or not.
- Feature windows: a feature (`search bookshelf`) gives ONE bauble roll per
  `BaubleFeatureWindowMinutes` (60), shared by the room unless
  `BaubleWindowPerPlayer`. The limit is on the bauble roll only: the search
  itself always happens in full (quests rely on `search <noun>`). The caller
  checks `FeatureSearchable` and claims with `ClaimFeatureSearch` before the
  roll, so the claim holds even where no bauble can be found; `RollFind` with
  `Feature` set spends no window of its own. A feature's roll never spends the
  room's rolls; once it is claimed, a repeat search of that feature takes the
  room's roll instead, as a plain search would.
- Off by default: `Balance.BaublesEnabled` (false) makes `RollFind` find
  nothing, silently, everywhere. `ResetWindow` clears a room's feature windows
  with its own. Each window stores its own end time, so the prune sweep is
  right for both lengths.
- Record fields for what happened after the find: `FoundIn` (the feature
  searched), `Household` (left for the household), `Stolen*` (set by
  `MarkStolen`: by whom, from which room, which resident was watching, their
  faction) and `VanishedAt` (left untaken for `BaubleUntakenHours`, 24;
  `UntakenLimit`). `CatalogStats` counts households, stolen and vanished, and
  leaves vanished records out of `Unsold`.
- The model decides weight from what the object is (a toy is light, a large
  vase heavy); code only clamps it to 0.1 to 25 lb, rounded to 0.1.
- Pickpocketed finds (`SourcePickpocket`, internal/actions/steal_pocket.go)
  are pocket-sized: `MaxWeightFor` is `Balance.BaublePickpocketMaxWeight`
  (1 lb) for them, applied by `ApplyLimitsFor`/`ClampWeightFor` in `Mint`,
  `Edit` and `ApplyRegenerated`; and `Generate` refuses a naming the model
  itself weighed over that, or whose name is a thing no pocket holds
  (`TooBigFor`, `notPocketSized`: urn, vase, candlestick, lantern, jug,
  book...): its text would name that thing, so it is a generic trinket
  instead of a clamped strongbox.
- An unknown or missing tier is always treated as cheap, never as dearer.
- One record per bauble, never shared, never reused. Ids are `B` plus seven
  digits and only ever go up; the ids of a shard that could not be read are
  skipped so no two objects ever share one, and the meta file is written at
  once so a reboot does not forget it.
- Every change is written through to disk before the call returns (records
  change rarely: a find, a naming, a sale). A NEW record whose write fails is
  not created at all (`Create` takes it back out of memory and returns the
  error; `Mint` hands out no item, and the find "crumbles away"), so no item
  can ever point at a record a crash would lose. A failed write of an
  existing record is logged, kept in memory, and retried by `SaveAll`.

## Gotchas

- **Admin edits go through `CleanReply` too**, so an admin cannot put
  markup, digits in a name, or a real item's keyword on a bauble by hand
  either. Value stays inside the tier; `ApplyRegenerated` refuses a generic
  answer so a failed regeneration never wipes a model name.
- **Locking.** The catalog has its own `sync.RWMutex` because the resolver
  runs inside `Item.GetSpec`, which is read from more than the game loop.
  It never calls out while holding it. `Update`'s change function runs under
  that lock: keep it to field assignments.
- **No pending records, no placeholder text.** A find is named FIRST (off
  the mud lock, `Generate`) and minted only when its text is final. Nothing
  is ever shown as "unexamined".
- **`Generate` blocks.** Call it only on a goroutine without the mud lock;
  mint and deliver under the lock afterwards (`actions/search_bauble.go`).
  It never fails: no generator, an error, a timeout or text that fails
  `CleanReply` all give a generic trinket.
- **No API key means generic trinkets, always.** `modules/baubles` installs a
  generator only when it is enabled and finds a key.
- **Keywords.** `CleanReply` refuses a keyword that a real item answers to:
  the fixed `reservedNouns` (key, sword, potion, token, ring...) and every
  loaded item's own keyword and every word of its name (so a bauble keyed
  "silver" can never take `get silver` from a real Silver Dagger;
  `items.AuthoredKeyword`, read
  through the `authoredKeyword` variable), so a bauble can never hijack
  `get key`, `drink potion` or `get lantern`. It falls back to the last word
  of the bauble's own name, then its other words (last to first, four
  letters or more), else `trinket`; players can still use any word of the
  name. Many common trinket nouns and adjectives are real items' words
  (locket, pendant, box, candle, silver, iron, leather...), so a fair share
  of finds are keyed "trinket"; the tie-break in `items.FindMatchIn` is the
  second guard. Natural finds (prompt version 3) add
  bone, stone, shell, tooth, geode, amber, pearl, claw, fang and crystal:
  the Reckoning Bone (quest 58), the weighted and river stones and several
  crafting materials answer to them. The prompt asks for a specific noun
  instead (quartz, agate, fossil, antler, jawbone).
- **Selling lives in `internal/actions/sell_bauble.go`**, not here. Every
  record is sellable, a sold one included: it only reaches a merchant again
  if a crash lost the seller's save after the sale was recorded.
- **Boot only.** `Load` runs on first boot, not on a data reload
  (`loadAllDataFiles(isReload)`): the catalog is runtime state.
- **The carrier must exist.** `Mint` refuses (`ErrNoCarrier`) and creates no
  record when item 900 is not loaded.
- **Runtime data.** `<DataFiles>/baubles/` is gitignored (with a `.gitkeep`)
  and skipped by the messaging surface guard and its Python twin, like
  `warehouses`.
- **Windows are in memory**, not in the room's temp data: rooms unload when
  nobody is near, which would reset a room-held window. A restart reopening
  every window is harmless. The map is swept of expired windows once it
  passes 2048 entries.
- **`TryFind` says nothing about why it found nothing.** The caller must keep
  a closed window, an excluded zone and a failed roll indistinguishable.
- `ParseReply` refuses unknown fields and fractional values. That is what
  sends a malformed reply down the fallback path instead of into the world.

## Dependencies

`configs`, `items`, `mudlog`, `util`; `gopkg.in/yaml.v3`. No network code.

## Consumers

- `main.go` (`Load` at boot, `SaveAll` at shutdown), `copyover.go`
  (`SaveAll`).
- `internal/usercommands/admin.bauble.go` (`bauble spawn|show|list`),
  `appraise.go` (free bauble appraisal).
- `internal/actions/sell_bauble.go` (`Get`, `Sellable`, `MarkSold`).
- `internal/actions/search_bauble.go` (`RollFind`, `Generate`, `Mint`,
  `RevealDelay`, `RecentNames`).
- `modules/baubles` (`SetGenerator`, `ReplySchema`, `ParseReply`,
  `PromptLine`, `WeightGuidance`, `PlainText`).
