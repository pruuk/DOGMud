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
  (what the item layer shows, including the retired text). `KeptToFinder()`
  is `PlayerKey && !Moderated`: DERIVED, never stored, so records named
  before slice H are covered. Such a record's `View` is the generic
  trinket (`genericName`, `genericNameSimple`, `genericDescriptionFor(id)`,
  stable per id) with its own text in `BaubleView.Finder` for
  `FoundByUserId` (none when that is 0); retired text wins over both.
  `View` sets `BaubleView.PlayerText` for any `PlayerKey` record, so no
  model prompt carries it (`items.Item.ModelName`).
  `MaterialFor(viewerUserId)` is the material for the finder alone. Never
  promotable (slice C takes only server-key moderated text). `Shelvable()`
  is the one rule for whether a sold or won bauble goes on a shop's
  secondhand shelf: worth more than the cheap tier and not retired. Shared
  by the player-sale path (`internal/actions`) and the auction win path
  (`modules/auctions`), so there is exactly one place to change it.
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
  index of return credits backs `ReturnCredits`. `persistShardPruning`
  writes a shard without the records a predicate marks and only then takes
  them out of memory (persist before publish); a shard write that fails
  leaves the shard dirty and prunes nothing from it that sweep, and the
  count of such failures is what `SweepStatus.ShardErrors` reports.
  `Record.prunableAt(now, keep)`:
  at least `minUnseenSweeps` (2) complete sweeps in a row found nothing
  pointing at the record AND `KeepDuration()` (`Balance.BaubleCatalogKeepDays`,
  30, at least 7 because the sales stats read a week) has passed since
  `Record.lastEvidence` (found, stolen, recognised, returned, sold, vanished,
  or last seen by a sweep); a record with a return credit is never pruned.
  `Load` and `SaveAll` do not prune: only a successful sweep does.
- **sweep.go**: the catalog sweep. `RegisterLiveSource(name, LiveWalk)`
  (the main package registers `users`, `rooms`, `mobs`, `shops`, `guilds` in
  `bauble_sweep.go`; `modules/auctions` registers `auctions`).
  `ExpectLiveSources(names...)` declares every store a sweep must see;
  a sweep run with no expectation declared, or with a declared name not
  registered (a module not built in, or the sweeper started before the main
  package's sources), fails closed before it walks anything, so a store the
  sweep cannot see is never read as empty (`ExpectedLiveSourceNames` lists
  what is declared). `runSweep`
  walks every live source under `util.LockMud`, then `scanDisk` off the lock,
  then `applySweep`: a referenced record gets `LastSeenAt=now` (moved forward
  only: a clock stepped back between sweeps cannot shorten the keep window),
  `UnseenSweeps=0`, any other one more unseen sweep, and each
  changed shard is written without its prunable records. Any error (no live
  source registered, an expected source missing, a source that panics, an
  unreadable file, a file that names a bauble and does not parse) fails
  closed: nothing is applied. `StartSweeper` runs it at boot and
  every `SweepInterval()` (`Balance.BaubleSweepHours`, 6); `StopSweeper` at
  shutdown. `LastSweep()` feeds `bauble status`, including any shard write
  failures (`ShardErrors`).
- **sweep_disk.go**: `DiskRefs(root, now)`: every `.yaml` and `.plugin.dat`
  under DataFiles except `baubles/` and `economy/snapshots/`; a file is
  parsed (`yaml.Node`) only if it has a `bauble` key. A symlinked file is
  followed to what it points at; a symlinked directory is not followed and
  fails the sweep closed instead (it could loop back on itself, or hide a
  save the scan would otherwise miss), and so does a dangling symlink and a
  `bauble` key whose value is not a scalar (a map or a list, a shape no
  writer produces). On a room file's floor
  (the top-level `items` list in `rooms.instances/`) a find untaken past
  `UntakenLimit()` is not a reference; `rooms.LoadRoomInstance` removes such
  finds on load. Stash and container finds always count.
- **fallback.go**: `GenericTrinket`, the last resort when neither the model
  nor the corpus names a find: "Trinket", a simple description, value and
  weight at random within the tier.
- **corpus.go**: the fallback corpus. `CorpusEntry`, `PromotedEntry`,
  `CorpusReport`, `LoadCorpus`, `LoadCorpusFrom`, `ReloadCorpus` (the seed
  `<DataFiles>/bauble-corpus.yaml` and the overlay
  `<DataFiles>/baubles/corpus.promoted.yaml`), `Fallback`, `FallbackFor`,
  `GroupOf`, `CorpusCounts`, `ClearCorpusForTest`.
- **corpus_admin.go**: `Promote` and its `ErrPromote*` refusals,
  `RemoveCorpusEntry`, `CorpusKeys`, `CorpusList`, `ExportPromoted`,
  `ErrOverlayBroken`, `ErrCorpusCleanup`, and the overlay clean-up that
  `Retire`, `Edit` and `ApplyRegenerated` call.
- **generate.go**: the generator seam (`SetGenerator`, `CurrentGenerator`),
  `GenRequest`, `GenResult`, `Generate`, `RecentNames`, `RecentFallbackNames`.
  `Generate` refuses a `PlayerKey` result that fails `CheckPlayerKeyText`, or
  is neither `Moderated` nor `FinderOnly` with a finder, and any
  `FinderOnly` result that is not `PlayerKey`; `RecentNames` skips
  `PlayerKey` records. A
  generator error that is a ledger refusal (`apiframework.RefusedBy`: a
  spent day, share or finder allowance) is logged by `noteRefusal`, naming
  the counter, at most once a minute (the AI companion's
  `logBudgetRefusal` pattern; `TestRefusedFindsAreLoggedOnceAMinute`);
  every other error is logged each time.
- **validate.go**: `CleanReply` (the text checks the schema cannot make:
  NFKC, curly quotes, en and em dashes and the ellipsis folded to ASCII
  (`typographyFold`), invisible and format characters dropped (`cleanRune`),
  rune lengths, link-shaped text refused (`linkRE`), errors quoting at most
  60 runes (`quoteShort`)) and `PlainText`, which is `cleanLine`, so the room
  text in a prompt is folded the same way. An authored item's whole name is
  refused (`items.AuthoredName`).
- **playerkey.go**: `CheckPlayerKeyText`, the plain-text allowlist for text a
  player's own key wrote, on the CLEANED name, keyword, description and
  material: ASCII letters, space, `' " - , . ! ?`, and every run of periods
  followed by a space, a `"` or the end.
- **mint.go**: `Place`, `NewPlace`, `MintOpts`, `Mint`. `Mint` rolls a
  `PlayerKey` find's value with `tier.RollValue`, keeping the key's
  proposal in `ValueProposed`. `ApplyRegenerated` takes the new result's
  `PlayerKey` (a regen is always server-key).
- **sales.go**: `MarkSold`, `MarkBought` (a buyback off a shop's shelf
  returns a sold record to `unsoldStatus`; any other status is left, so a
  retired one stays retired; the sale fields are kept), `SalesSince`
  (counts by `SoldAt` alone, so a buyback does not erase a sale).
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
  goods; a theft clears it). `ShelfHoldUntil(itm, now)` (slice D) is
  `StolenAt + HeatDuration()` while the record is `Hot` anywhere, else zero:
  how long a shelved bauble is held out of sight.
- **admin.go**: `CatalogStats`, `Retire`, `Restore`, `Edit` (hand edits,
  checked like a model's answer), `ApplyRegenerated`, the prompt-preview
  seam (`SetPromptPreview`, `PreviewPrompt`) and `LooksLikeId`.
  `Record.unsoldStatus` (ready when `Generator.Named()`, else fallback) is
  `Restore`'s rule, shared with `MarkBought`.
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
type GenResult struct { /* Reply, Generator, Model, PromptVersion, Tokens, Moderated, PlayerKey, FinderOnly */ }
type GeneratorFunc func(ctx context.Context, req GenRequest) (GenResult, error)
func SetGenerator(fn GeneratorFunc, info func() GeneratorInfo)
func CurrentGenerator() (GeneratorInfo, bool)
func Generate(ctx context.Context, req GenRequest, randn func(n int) int) GenResult // blocks; never fails
func RecentNames(zone string, n int) []string

type Record struct { /* see record.go */ }
func (r Record) View() items.BaubleView
func (r Record) Shelvable() bool

func Load() error        // boot, after items.LoadDataFiles()
func SaveAll()           // shutdown and copyover; retries failed writes
func SetDirForTest(dir string)
func Create(r Record) (Record, error)
func Get(id string) (Record, bool)
func Update(id string, change func(r *Record)) (Record, bool)
func Count() int
func Recent(n int) []Record
func KeepDuration() time.Duration

type LiveWalk func(visit func(*items.Item))
func ExpectLiveSources(names ...string)
func ExpectedLiveSourceNames() []string
func RegisterLiveSource(name string, walk LiveWalk)
func LiveSourceNames() []string
type SweepStatus struct { /* At, OK, Err, Skipped, Records, Referenced, Pruned, ShardErrors, Files, Parsed, Live, Disk */ }
func RunSweep(now time.Time) SweepStatus // takes the mud lock: never call holding it
func LastSweep() SweepStatus
func SweepInterval() time.Duration
func StartSweeper()
func StopSweeper()
func DiskRefs(root string, now time.Time) (refs map[string]bool, files int, parsed int, err error)

type Place struct{ RoomId int; Zone, Region, Biome string }
func NewPlace(roomId int, zone string, region string, biome string) Place
type MintOpts struct{ Source Source; Place Place; FinderUserId int; Tier ValueTier; FoundIn string; Result *GenResult; Randn func(n int) int }
func Mint(o MintOpts) (items.Item, Record, error)

func MarkSold(id string, gold int, sellerUserId int) bool
func MarkBought(id string, buyerUserId int) bool
func ShelfHoldUntil(itm items.Item, now time.Time) time.Time
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
func Edit(id string, field string, value string, admin string) (Record, int, error) // EditFields; int: corpus entries removed
func ApplyRegenerated(id string, res GenResult, admin string, randn func(n int) int) (Record, int, error)
func SetPromptPreview(f PromptPreview)
func PreviewPrompt(req GenRequest) ([]string, bool)
func LooksLikeId(s string) bool
func SalesSince(t time.Time) (count int, gold int)

const GeneratorCorpus Generator = `corpus`
func (g Generator) Named() bool // openai or corpus: a ready record
func RecentFallbackNames(zone string, n int) []string
// Record.HandEdited: set only by Edit, cleared by ApplyRegenerated; Promote refuses it

type CorpusEntry struct{ Name, NameSimple, Description, Material string; WeightLbs float64; Value int }
type PromotedEntry struct{ CorpusEntry; FromRecord, Zone, Biome, Model string; PromptVersion int; PromotedAt time.Time }
type CorpusReport struct{ Seed, Promoted int; Skipped []string; SeedErr error; SeedKept bool; Quarantined string; OverlayBroken bool }
func LoadCorpus() CorpusReport // boot and data reload, after items and Load
func LoadCorpusFrom(seedPath, overlayPath string) CorpusReport
func ReloadCorpus() CorpusReport
func ClearCorpusForTest()
func Fallback(place Place, tier ValueTier, source Source, recent []string, randn func(n int) int) GenResult
func FallbackFor(req GenRequest, randn func(n int) int) GenResult
func GroupOf(biome string) (string, bool)
func CorpusCounts() (seed int, promoted int)

func Promote(id string) (string, error) // ErrNoCorpus, ErrOverlayBroken, ErrNoRecord, ErrPromote*
func RemoveCorpusEntry(key, which string) (PromotedEntry, error) // which: the entry's name or its FromRecord id
type CorpusKeyCount struct{ Key string; Seed, Promoted, Unused int }
func CorpusKeys() []CorpusKeyCount
type CorpusListing struct{ Known bool; Seed []CorpusEntry; Promoted []PromotedEntry; Unused map[int]string }
func CorpusList(key string) CorpusListing
func ExportPromoted() (string, error)

type FindOpts struct{ Place Place; UserId int; SkillFactor float64; SightPenalty float64; Feature string; Household bool; Randn func(n int) int; Now time.Time }
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
- `FindOpts.SightPenalty` (0 is none, clamped to 0..1 by `clampUnit`)
  multiplies the chance by `1 - SightPenalty` after the nothing-here check
  and before the window roll, so a search in the dark spends its roll as
  one in the light does; callers pass `1 - messaging.SightMult`.
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
  book...): its text would name that thing, so it falls back to the
  corpus's pocket pool instead of being a clamped strongbox.
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
- A record lives as long as something points at it. The sweep is the only
  pruner and it fails closed. A record on a shop's shelf is sold and still
  referenced (`WalkItems` visits `AffixedStock`), so it is kept; a buyback
  makes it unsold (`MarkBought`). A sold record held again (a crash rolled
  the seller back) is seen and kept, and its sale is left as it was: every
  record is sellable, and a save on disk can lag a real sale by one
  autosave. `TestItemWalkersVisitEveryItemField` and
  `TestEveryItemHolderIsASweepRootOrTransient` (repo root) fail when a store
  of items is not walked; a new store needs a `WalkItems`, a live source in
  `bauble_sweep.go` and a root in `item_walker_guard_test.go`.
- **Fallback corpus** (slice C of the 2026-09-28 hardening design). A find
  no model names takes its text from the corpus (`FallbackFor`, called by
  `Generate`, `Mint`, `FlushBaubleDeliveries` and the pickpocket reveal):
  one pool merging the promoted overlay and the seed at `<biome>-<tier>` and
  `<group>-<tier>` (an empty or unmapped biome skips both), then the bare
  `<tier>` only when that pool is empty. A pickpocketed find uses
  `pocket-<tier>` then `<tier>`, filtered by `TooBigFor`. Names the zone
  found lately (`RecentFallbackNames`) are avoided; when every entry is
  recent the least recent is taken, never a broader key. Values are clamped
  into the tier and weights limited (`ApplyLimitsFor`) at use. Nothing
  fits: a generic trinket. Records say `Generator` `corpus` and `Model`
  `corpus:<key>`, and are `ready`.
- **Promotion.** `Promote` copies a model name (`GeneratorOpenAI`, not
  `PlayerKey`, `Moderated`, not `HandEdited`, not retired; sold is fine)
  into the overlay under its exact `<biome>-<tier>` or `pocket-<tier>` key,
  with provenance, unless its pool (the key and, for a biome, its group's
  key) already has an entry by that name. `HandEdited` is set only by
  `Edit` and cleared by `ApplyRegenerated`; `EditedBy` does not bar
  promotion, because `Retire`, `Restore` and regen set it without writing
  text. `Retire`, `Edit` and `ApplyRegenerated` remove the record's overlay
  entries (the last two return how many; `ErrCorpusCleanup` means the
  record changed but the entries could not be removed). `RemoveCorpusEntry`
  takes an entry's name or record id, never a position. The /build queue
  will call the same functions.
- **A broken overlay is never written.** When the overlay cannot be read
  and cannot be quarantined either, the pool is marked broken and every
  overlay writer returns `ErrOverlayBroken` until a reload succeeds (a save
  would replace entries the pool never saw). A reload whose seed cannot be
  read keeps the seed already in use (`SeedKept`).

## Gotchas

- **Finder-only text is fail-safe, not routed.** The catalog's
  viewer-agnostic view of a finder-only record IS the generic trinket, so
  every render path shows "Trinket" unless it asks the item layer for one
  viewer's view (`items.Item.GetSpecFor` and kin), which only the
  single-reader functions listed in the repo-root
  `bauble_finder_view_guard_test.go` do, each with a pinned call count.
  Never read a record's `Name`, `Description` or `Material` for display
  outside the admin command; use the item accessors or `MaterialFor`.
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
  `CleanReply` all fall back through `FallbackFor` (the corpus, else a
  generic trinket).
- **No API key means corpus finds.** `modules/baubles` installs a generator
  only when it is enabled and finds a key; without one every find comes
  from the fallback corpus, and a generic trinket only when the corpus has
  nothing that fits (an empty corpus behaves exactly as before).
- **Two corpus layers, two rules.** The seed is authored content: a broken
  file logs ERROR and the corpus runs without it (CI:
  `TestShippedCorpusSeed`, which loads items first). The overlay is living
  state: `util.ReadLivingState`, quarantine on corruption, `util.Save`,
  persist before publish; an overlay entry that fails its checks is kept on
  every save and never used. The overlay sits in the catalog's directory,
  which is safe because the catalog loader reads only `catalog-*` files.
- **Load order.** `LoadCorpus` runs after `items.LoadDataFiles` (entries are
  checked against authored item names, `CleanReply`) and after `Load`, and
  also on a data reload, unlike the catalog.
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
  record is sellable, a sold one included. A record reaches a merchant
  again after a buyback off a shop's shelf (`MarkBought` makes it unsold
  first); one still marked sold does so only if a crash lost the seller's
  save after the sale was recorded.
- **Boot only.** `Load` runs on first boot, not on a data reload
  (`loadAllDataFiles(isReload)`): the catalog is runtime state.
- **The carrier must exist.** `Mint` refuses (`ErrNoCarrier`) and creates no
  record when item 900 is not loaded.
- **Runtime data.** `<DataFiles>/baubles/` is gitignored (with a `.gitkeep`)
  and skipped by the messaging surface guard and its Python twin, like
  `warehouses`.
- **The corpus overlay is decoded whole and strictly** (`corpus.go`,
  `decodeStrict` with `KnownFields`). One type error (a word where
  `weight_lbs` wants a number) or one unknown field (a typo such as
  `valeu:`) anywhere in `baubles/corpus.promoted.yaml` fails the decode of
  the WHOLE document, not just that entry: the overlay is quarantined
  (`util.QuarantineCorrupt`, its bytes kept aside unchanged for recovery)
  and restarts empty. Only an entry that parses but fails `checkEntry` is
  skipped on its own, and that one is kept and saved again. Fix a hand
  edit from the quarantined copy, then reload (`ReloadCorpus`).
- **Windows are in memory**, not in the room's temp data: rooms unload when
  nobody is near, which would reset a room-held window. A restart reopening
  every window is harmless. The map is swept of expired windows once it
  passes 2048 entries.
- **`TryFind` says nothing about why it found nothing.** The caller must keep
  a closed window, an excluded zone and a failed roll indistinguishable.
- `ParseReply` refuses unknown fields and fractional values. That is what
  sends a malformed reply down the fallback path instead of into the world.
- **`linkRE` is a heuristic.** It misses a top-level domain longer than six
  letters and a domain written with U+3002 (NFKC keeps it); server-key text
  is moderated, and player-key text is held to the ASCII allowlist, which
  refuses both. It also refuses a missing-space typo such as `horse.Its` on
  every route; that find falls back like any unusable reply. Accepted by the
  owner, 2026-09-28.

## Dependencies

`apiframework` (only `RefusedBy`, to recognise a refusal), `configs`,
`items`, `mudlog`, `util`; `gopkg.in/yaml.v3`. No network code.

## Consumers

- `main.go` (`Load` at boot, `LoadCorpus` at boot and data reload,
  `StartSweeper` before Server Ready, `StopSweeper` then `SaveAll` at
  shutdown), `bauble_sweep.go` (`RegisterLiveSource`), `copyover.go`
  (`SaveAll`; the sweeper is not stopped there, see the comment).
- `modules/auctions` (`RegisterLiveSource` for the auction house).
- `internal/usercommands/admin.bauble.go`
  (`bauble spawn|show|list|promote|corpus`; `bauble status` reads
  `LastSweep`, `SweepInterval`), `appraise.go` (free bauble appraisal).
- `internal/actions/sell_bauble.go` (`Get`, `MarkSold`).
- `internal/actions/search_bauble.go` (`RollFind`, `Generate`, `Mint`,
  `RevealDelay`, `RecentNames`).
- `internal/actions/search_bauble.go` and `steal_pocket.go` (`FallbackFor`).
- `modules/baubles` (`SetGenerator`, `ReplySchema`, `ParseReply`,
  `PromptLine`, `WeightGuidance`, `PlainText`).
