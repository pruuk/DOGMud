# Baubles hardening and fallback corpus: design

Date: 2026-09-28. Owner: the DOGMud owner. Built on PR #175 (FinalTwist's
baubles) at head `f9fc5d017`, on branch `fix/baubles-hardening`, worktree
`C:/tmp/pr175-ours`. Revision 2, after four blind adversarial reviews (fact
check, security completeness, S5 migration, corpus and S3).

FinalTwist is fixing a separate list on the same PR: lint, the pickpocket
walk-out, catalog prune and writes outside the lock, moderation before the
chat call, a real item beating a bauble only on an equal or stronger match,
the household guard moving into `actions.GetItemFromFloor`,
`combat.SkillMultiplier` for `BaubleSkillFactor`, and the pickpocket reveal
on a `NewTurn` listener with naming at the reveal. Where this design touches
the same code (S3 and `moderate`), the interface section says how the two
compose.

## Facts verified against source

Read at `f9fc5d017` unless marked master. Revision 2 corrects four rows the
fact-check review found partly right.

| Fact | Where |
|---|---|
| `server set <key> <value>` calls `configs.SetVal` directly and never checks `Locked`; only the `server config` menu does (master bug) | `internal/usercommands/admin.server.go:110-113` vs `:323, :366, :386` |
| `SetVal` ignores `Locked`; `ErrLockedConfig` is declared and used nowhere | `internal/configs/configs.go:40, 267, 377` |
| `SetVal` resolves keys through `FindFullPath`, whose table includes bare suffix keys (`baseurl`, `apikey`) that collide across sections | `configs.go:505-521, 577` |
| `isEditAllowed` matches `Locked` by lowercase prefix | `admin.server.go:416-428` |
| Shipped `Locked`: `FilePaths`, `Server.CurrentVersion`, `Server.NextRoomId`, `Server.Seed`, `Server.OnLoginCommands`, `Server.BannedNames` | `_datafiles/config.yaml:99-105` |
| `/viewconfig` is served by the unauthenticated `/` handler and renders `AllConfigData` with an exclusion list that does not cover any API key (master) | `internal/web/web.go:308`, `_datafiles/html/public/viewconfig.html:11` |
| The config dump walks module maps, so `Modules.aicompanion.APIKey` appears in it | `configs.go:169-223` |
| `AllConfigData` feeds the boot log, `server set` listing, `server config`, and `/viewconfig` | `main.go:247`, `admin.server.go:55, 320, 338, 392`, `viewconfig.html:11` |
| `ConfigSecret` prints `*** REDACTED ***` | `internal/configs/config_types.go:96` |
| `APIFramework.APIKey` is a plain `ConfigString`; `Validate` rebuilds it as one | `internal/configs/config.apiframework.go:25, 54` |
| The allowlist accepts any `*.openai.com` or `*.azure.com` unless `AllowCustomEndpoint` | `internal/apiframework/settings.go:32` |
| Empty `APIFramework` settings fall back to `Modules.aicompanion` `APIKeyEnv`, `APIKey`, `BaseURL`, `AllowCustomEndpoint` | `settings.go:208-241` |
| `RelayOrigin` accepts any https host but the game's own, and is config-settable | `modules/aicompanion/tiers.go:43`, `config.yaml:2580` |
| `moderate` accepts an unmoderated player-key reply when there is no server key, the breaker is open (`:248-262`), or the moderation call fails (`:267-268`); `ModerateOutput: false` skips it for all | `modules/baubles/generate.go:248-268` |
| Moderation input is Name and Description only; `appraise` shows `Material` | `modules/baubles/generate.go:266`, `internal/usercommands/appraise.go:118` |
| Records carry `Moderated` and `PlayerKey` | `internal/baubles/record.go:84-85` |
| `ApplyRegenerated` never updates `PlayerKey`; the regen request includes `rec.Name` | `internal/baubles/admin.go:207-234`, `internal/actions/bauble_admin.go:62` |
| `Edit` keeps `Moderated`; `Restore` sets a retired record back to `ready` | `internal/baubles/admin.go:107, 134-195` |
| `RecentNames` selects zone and `GeneratorOpenAI`, ignoring `PlayerKey` | `internal/baubles/generate.go:157-176` |
| Reply value is clamped to the tier (`ClampValue`); `RollValue` exists | `internal/baubles/reply.go:89`, `tiers.go:99, 107` |
| `cleanLine` strips `unicode.IsControl` only; whitespace regex is ASCII `\s`; name and description lengths are bytes | `internal/baubles/validate.go:31, 60-64, 87, 93` |
| `items.AuthoredKeyword` reads a snapshot built by `rebuildAuthoredKeywords`, which already excludes item 900 | `internal/items/itemspec.go:546-576` |
| `CleanReply` runs off the mud lock | `modules/baubles/generate.go:75` |
| `ledger.reserve` checks only the global limit; `Hold` has no user field | `internal/apiframework/budget.go:47-51, 142-160` |
| `SaveBudget` copies `ByConsumer` and `CallsBy` under the lock | `budget.go:286-301` |
| Two `Reserve` callers: the companion through `Books.Reserve`, baubles through the package function; `viaServer` has no finder id | `modules/aicompanion/models.go:567`, `modules/baubles/generate.go:170, 188` |
| Companion per-user state: `ownerTokens`, `strangerTokens`, `strangersFor`, rolled by the module's own `rollDay`, persisted in the companion's save and restored by `loadBudget` | `modules/aicompanion/aicompanion.go:172-174, 403-414`, `models.go:208-265, 657-704` |
| A passer-by call on the owner's own key checks and charges per-user allowances but never the server ledger | `modules/aicompanion/tiers.go:251-296`, `internal/apiframework/relay.go:411-412` |
| Relay settlement clamps returned tokens to 0..held | `modules/aicompanion/tiers.go:296` |
| 61 companion test references read or write the three maps or `budgetDay` | `aicompanion_test.go` (24), `tiers_test.go` (23), `money_test.go` (14) |
| A naming reserves about 2,500 to 2,600 tokens (double with `RetryTransient`) | `internal/apiframework/wire.go:211-219`, `modules/baubles/generate.go:188` |
| Relay reply bodies up to 1 MiB; `CleanReply` errors quote the name and are logged at Warn | `internal/apiframework/relay.go:78`, `validate.go:88`, `internal/baubles/generate.go:131, 136` |
| `DecodeChat` keeps 300 bytes of provider error text | `internal/apiframework/wire.go:167` |
| Four naming slots are shared by server and relay calls | `modules/baubles/generate.go:36-44` |
| `rollChance` is flat; the chance is computed then the window roll is taken | `internal/baubles/find.go:145-153, 214-221` |
| `searchBaubleRoll` is a package-level function variable | `internal/actions/search_bauble.go:55-57` |
| The sight guard watches `RunContest`, `contest.AgainstDifficulty` and four crafting rolls; a package-level initialiser is always reported | `sight_penalty_guard_test.go:40-41, 67-69, 156, 308-322` |
| Every other find-like roll pays `messaging.SightMult` | `forage.go:106`, `salvage.go:102`, `track.go:129`, `search.go:158`, `shop_sight.go:57` |
| `SightScoreMultiplier` floors at 0; shipped caps 0.80 dark and bright | `internal/messaging/sight_mult.go:18-24`, `config.yaml:925, 931` |
| Bare-name `SendTextVisual` observer lines: new `search.go:164`, `look.go:525`; master `search.go:170`, `look.go:460, 567, 572` | as listed |
| `SendTextVisualHidingNames(cat, txt, names []string, exclude...)` | `internal/rooms/rooms.go:276` |
| `GenericTrinket` has four call sites; `mint.go:74` runs only in tests; biome is available at all four through `Place` | `search_bauble.go:237`, `steal_pocket.go:261`, `baubles/generate.go:118`, `baubles/mint.go:74` |
| `Mint` marks only `GeneratorOpenAI` as `ready`; stats print openai and local only | `internal/baubles/mint.go:81-84`, `internal/usercommands/admin.bauble.go:326` |
| `TooBigFor(reply, source)` refuses over-weight and not-pocket-sized names | `internal/baubles/weight.go:55-83` |
| The catalog loader reads only `catalog-*.yaml` in `baubles/` | `internal/baubles/store.go:28, 106` |
| `gossip.Pool` is a plain map; the fallback chain lives in its caller | `internal/gossip/gossip.go:92`, `internal/hooks/MobIdle_HandleIdleMobs.go:438, 502-505` |
| `golang.org/x/text` is already a dependency | `go.mod:8` |
| 24 biomes; `water` and `ether` ship at 0% | `_datafiles/world/dogmud/biomes/`, `config.yaml:1418-1442` |

## Owner rulings

Brainstorm, 2026-09-28:

1. Security-critical fixes are ours; gameplay fixes are FinalTwist's.
2. The bauble search chance pays the sight ramp through `SightMult`.
3. Text named on a player's key is moderated or refused.
4. Per-user API allowances move into `apiframework` and the companion
   migrates onto them. Re-confirmed after the S5 review: migrate in full.
5. Corpus keyed by biome group, about 210 seed entries.
6. Corpus in two layers: tracked seed, living-state overlay.
7. Seed first, then promote server-key moderated prod names; promotion also
   comes to /build later (web builder rework arc).
8. Corpus values stored, clamped into the tier at use; each pool's mean is
   pinned near its tier midpoint by a test. A known name implying its price
   is accepted.
9. Security locks are a hard-coded Go list enforced in `SetVal`.
10. Player-key text: ASCII allowlist plus mandatory moderation; other
    players still see it.

Plan review round, 2026-09-28 (after FinalTwist's `e711ee9de`):

11. Delivery: PR #175 merges first, after FinalTwist's fix round, with no
    deploy (everything ships off). Then M, H, S5 and C each go to master as
    their own PR, branched fresh from master. Our commits never rebase or
    push onto his branch. The spec and plans go to master as a docs-only
    PR. Every gate checks `git diff --shortstat` stays under 20k lines and
    300 files (the CI lint inversion).
12. S5 defaults accepted: clamp usage only for relayed counts (server-key
    overage is still charged); refund owner-less key-0 holds; a share of 0
    means the default (companion 100%, baubles 25%), -1 or 100 means no
    cap; `baubles.finder` is charged even on the finder's own key; the
    companion keeps writing its own allowance file as a backup, and a
    quarantined `budget.yaml` re-seeds from it.
13. `ModerateOutput` and `ModerationModel` (companion and baubles) join the
    hard-locked list.
14. Revised the same day: the five fences with no shop become real
    shopkeepers like Siv (a `shop:` block, non-combatant), so they pay from
    persisted shop gold with the normal restock and cannot be attacked or
    robbed. That conversion is FinalTwist's fix. Resale is OURS (owner,
    same day), in slice D: fences resell bought baubles through
    `ShopInventory.AffixedStock` with a new `HoldUntil` (stolen goods:
    `StolenAt + HeatDuration()`, checked with `Record.Hot`, so they list
    only once cold everywhere; honest goods: zero). Held entries are
    neither listed nor sold, and eviction age starts at listing.
15. Player-key allowlist (decided in review): `cleanLine` folds curly
    quotes, en and em dashes and the ellipsis to ASCII before the check;
    the system prompt states the allowed characters (PromptVersion 5); an
    allowlist refusal falls back to the server route or the corpus and
    does not feed the player's breaker. `NameSimple` joins the moderated
    and allowlisted fields.

## Slice D: fence resale and the economy dashboard

Depends on FinalTwist turning every fence into a real shopkeeper (ruling
14). Two parts, both ours:

1. **Resale with a heat hold.** Today `GetSellPrice` refuses baubles and a
   sold bauble is never shelved (`internal/mobs/mobs.go:1039`). A fence
   instead puts a bought bauble on `ShopInventory.AffixedStock`
   (`internal/shops/shopinventory.go:72-93`, the existing unique-item
   resale shelf) with a new `HoldUntil time.Time`: `StolenAt +
   HeatDuration()` for stolen goods (`Record.Hot`, so cold everywhere
   before it lists), zero for honest goods. Shop listings and purchases
   skip held entries; the age-based eviction clock starts at listing. The
   catalog record's status and sale bookkeeping must stay consistent when a
   sold bauble is bought back off a fence's shelf (resolved in the plan).
2. **Dashboard.** Fences appear in the snapshot's `Shops` section once they
   are shops. Slice D adds a fence marker to `ShopSnapshot` (from the
   `fence` group), held versus listed bauble counts, and a fence filter or
   panel on `_datafiles/html/admin/economy/index.html`.

Planned once his change lands.

## Slice M: config locks and redaction (on master first)

These are master bugs, independent of the PR. Slice M lands on master as its
own PR; slice H rebases onto it.

**M1. `SetVal` enforces locks.** `SetVal` resolves the key with
`FindFullPath`, then refuses with `ErrLockedConfig` when the RESOLVED path
matches either the `Server.Locked` list (prefix, lowercase, as
`isEditAllowed` does) or a new Go constant list `configs.hardLocked`.
`isEditAllowed` delegates to the same function. `Server.Locked` itself is
hard-locked. The hard list (exact paths, lowercase compare):

- `APIFramework.APIKey`, `.APIKeyEnv`, `.BaseURL`, `.AllowCustomEndpoint`
- `Modules.aicompanion.APIKey`, `.APIKeyEnv`, `.BaseURL`,
  `.AllowCustomEndpoint`, `.RelayOrigin`, `.PlayerKeys`, `.Model`,
  `.FastModel`, `.DeepModel`
- `FilePaths.WebDomain`, `Server.Locked`

Slice H adds `Modules.baubles.Model`, `.MaxCompletionTokens`,
`.MaxConcurrent`, `.UsePlayerKeys`. Budget knobs stay tunable in game.
Module config setters that write through `SetVal` inherit the check; the
plan greps every `SetVal` caller.

**M2. One redacted view of the config.** New `Config.DisplayConfigData`
returns `AllConfigData` with values replaced by `*** REDACTED ***` for any
`ConfigSecret` and any leaf whose last path element matches
`(?i)^(apikey|secret|password)$`. `AllConfigData` stays raw because
`FindFullPath` types come from it. The boot log (moved out of `main()` into
a testable function), both `server` listings and `/viewconfig` switch to
`DisplayConfigData`. `/viewconfig` also excludes `apiframework*` and
`modules*`. A guard test fails if any template, or any Go file outside
`internal/configs`, calls `AllConfigData` except the call sites that use it
only to look up a key's type, each allowlisted by name with its reason.

**Tests.** `server set Server.Seed 1`, `server set seed 1` (suffix key) and
`server set Server.Locked x` all refused; a sentinel key set in both key
locations never appears in the boot-log function's output, either server
listing, or a rendered `/viewconfig`.

## Slice H: hardening (on the PR)

### S1. The key is a secret

`APIFramework.APIKey` becomes `ConfigSecret`, including in `Validate`.
`key_guard_test.go` gains cases for the typed key and the module-map key
through M2's display path.

### S2. The key only goes to OpenAI

`endpointAllowed` accepts exactly `api.openai.com` and hosts ending
`.openai.azure.com`, unless `AllowCustomEndpoint`. The endpoint, key and
model settings are in M1's hard list, so no admin can change them in game.
`DecodeChat` scrubs `sk-\S+` (and the masked `sk-proj-\S*` form) from the
300 bytes it keeps, on both server and relay routes.

### S3. Player-key text

- **Pre-check, not post-refusal.** `name()` skips the player route unless
  `ModerateOutput` is on, the server has a key, and the provider breaker is
  closed; then the find goes to the server route or the corpus. This sits
  before the route choice at `generate.go:108` and composes with
  FinalTwist's "moderate before paying" change, which reorders the server
  route after this check. Any moderation failure after a player-key reply
  (including the call failing at `:267`) refuses the reply.
- **Allowlist.** Player-key Name, Description and Material are limited to
  ASCII letters, space, and `' - , . ! ?`, with a period only before a space
  or at the end. Anything else refuses the reply.
- **Material is moderated** with Name and Description, for every route.
- **Value.** For a player-key find `Mint` ignores the reply's value and rolls
  `tier.RollValue`.
- **Tokens.** `viaPlayer` passes the relay's token count through
  `Charged(..., relayed=true)` before it reaches the record or statistics.
- **Slots.** Relay calls take a per-user slot (one in flight per finder), not
  one of the shared server slots.
- **Recent names and regen.** `RecentNames` skips `PlayerKey` records.
  `BaubleRequestForRecord` omits the name of a `PlayerKey` record.
  `ApplyRegenerated` sets `PlayerKey` from the new result.
- **Authored names.** `CleanReply` refuses a name equal to any authored item
  name after normalising both: NFKC (`golang.org/x/text/unicode/norm`),
  lowercase, every `unicode.IsSpace` run collapsed to one space. The name set
  is built inside `rebuildAuthoredKeywords` in the same atomic snapshot,
  read through `items.AuthoredName(name) bool`.
- **Logs.** Validation errors quote at most 60 runes of the offending text.

### S4. Output cleaning (every route)

`cleanLine` maps every `unicode.Zs` to ASCII space and drops `unicode.Cf`,
`Co`, `Cs`, `Mn`, U+2028, U+2029 and the Hangul fillers U+115F, U+1160,
U+3164, U+FFA0, after NFKC. `CleanReply` refuses, case-insensitively, text
containing `://`, `www.`, or a token matching
`[a-z0-9-]+\.[a-z]{2,6}(/|\b)`. Name and description lengths are counted in
runes. A table test lists each code point the v10 review showed surviving,
an OSC sequence and NBSP. Cyrillic homoglyphs are real letters and pass
S4; the defence is S3's ASCII allowlist for player-key text and moderation
for server-key text, and a test pins that a homoglyph name on a player key
is refused.

### S5. Allowances in `apiframework`, with the companion migrated

Ruling 4 stands. The review found the single-key `Reserve` cannot express
the companion's rules, so the ledger API grows to fit them.

**API.**

- `Charge{Dim string, UserId int}` names one per-user allowance.
  Dimensions: `companion.owner`, `companion.stranger`,
  `companion.strangersfor`, `baubles.finder`.
- `Reserve(consumer string, tokens int, spendServer bool, charges ...Charge) (Hold, error)`
  checks, under the one ledger lock and all or nothing:
  1. if `spendServer`, the global daily limit and the consumer's share;
  2. each charge against its dimension's daily allowance.
  It then adds the tokens to every checked counter. `spendServer` false is
  the relay path: allowances are charged, the server budget is not.
- `Hold` records consumer, tokens, day, `spendServer` and the charges.
- `Settle(h, used, failed)` adjusts every counter the hold touched. `used` is
  clamped to 0..`h.Tokens` (the relay clamp, now for every caller). Every
  counter floors at 0. A hold from an earlier day follows the ledger's
  existing rule for all counters.
- `Allowance(dim, userId) (spent, limit int)` answers read-only checks
  (`ownerBudgetLeft`, `strangerMayAsk`, the admin line, the runtime log).
- `SeedAllowance(dim, day, userId, tokens)` seeds a day's per-user spend,
  like `SeedTokens` does for consumer totals.

**Day.** The ledger's clock is the only clock. The companion's `rollDay`
and `budgetDay` go; tests move day with the ledger's `SetClockForTest`.

**Persistence.** `budget.yaml` gains `by_user: {"<dim>:<userId>": n}`,
copied deep in `SaveBudget` under the lock. Corruption keeps today's
quarantine behaviour; the day's per-user spends restart with it, which is
the same loss the file already accepts for totals. On the first boot after
deploy, the companion's `loadBudget` seeds today's `Owners`, `Strangers` and
`StrangersFor` through `SeedAllowance` once (marked in the ledger by day),
then stops writing them.

**Config.** Typed scalar knobs, no dotted map keys:
`APIFramework.CompanionSharePercent`, `APIFramework.BaublesSharePercent`
(25), and the per-user limits where each feature's other knobs live:
`Modules.aicompanion.DailyTokensPerCompanion`, `StrangerDailyTokens`,
`StrangerTokensPerOwner` (unchanged names and values), and
`Modules.baubles.DailyTokensPerUser` (20,000, about 8 to 10 namings). A
share with no global cap (`DailyTokenBudget` 0 or negative) is no limit.

**Callers.** Baubles thread `FinderUserId` into `viaServer` and `viaPlayer`
and charge `baubles.finder`. A call with no finder (admin `regen`) charges
no per-user dimension and counts under the baubles share. Companion call
sites map one to one: owner call on server key = `spendServer` true,
`companion.owner`; passer-by on server key = true, `companion.stranger` and
`companion.strangersfor`; passer-by on the owner's key = false, the same two.
The existing rule that a server-key stranger call never charges the owner's
own allowance is kept by not passing `companion.owner`.

**Tests.** First, a rule table extracted from the current companion code and
tests (every charge, check, settle and day rule), reviewed before any code
moves. The 61 test references are rewritten to assert the same rules
through `Allowance` and the ledger clock; no rule in the table may lose a
test. New: two-charge all-or-nothing refusal; relay reserve leaves the
server total unchanged; settle clamp and floor; cross-midnight settle; a
`SaveBudget` race test under `-race`; first-boot seeding; baubles over its
share and over its per-user limit fall back.

**Review.** S5 gets its own review pass before joining the slice.

### U2. The bauble search pays the sight ramp

`FindOpts` gains `SightPenalty float64` (0 means none, so a zero-value
`FindOpts` behaves as today). `RollFind` multiplies the chance by
`1 - SightPenalty` after `chanceFor` and before the window roll, so a
search in the dark still spends a window roll exactly as a search in the
light does. The post-sight chance is logged with the find.
`searchForBauble` and the feature roll set it from
`1 - messaging.SightMult(char, room)`. The sight guard adds
`"actions": {"searchBaubleRoll": true}` and exempts the `var searchBaubleRoll`
initialiser; its probe drops `SightMult` from `searchForBauble` and must go
red.

### U4. Names hidden by sight

`search.go:164, 170` and `look.go:460, 525, 567, 572` move to
`SendTextVisualHidingNames` with the actor's name in `names`. Four of the six
are master's own lines; fixed here because the new ones copied them.

## Slice C: fallback corpus

### Seam

`baubles.Fallback(place Place, tier ValueTier, source Source, recent []string, randn func(int) int) GenResult`
replaces all four `GenericTrinket` call sites. With nothing in the corpus it
returns `GenericTrinket`.

Corpus results carry `Generator: corpus`, `Model: "corpus:<key>"` (so a bad
entry can be traced and removed), `Moderated: false`, `PlayerKey: false`.
`Mint` marks corpus records `ready`. `record.go`'s status comment, the stats
line (`admin.bauble.go:326`) and the pickpocket `named=` log learn the new
generator. No record migration: `corpus` is a new value only.

### Files

- Seed, tracked: `_datafiles/world/dogmud/bauble-corpus.yaml` with
  `groups:` (biome to group) and `entries:` (key to a list of
  `{name, name_simple, description, material, weight_lbs, value}`).
- Overlay, living state: `_datafiles/world/dogmud/baubles/corpus.promoted.yaml`.
  Same entry shape plus provenance: `from_record` (informational; it dangles
  once the catalog prunes the record), `zone`, `biome`, `model`,
  `prompt_version`, `promoted_at`. Covered by the existing `baubles/` ignore
  rules; the catalog loader only reads `catalog-*`, and FinalTwist's prune
  must keep that filter (a test asserts the overlay survives a prune).

Groups: `dwelling` (interior, fort), `street` (city_backstreet,
city_thoroughfare), `underground` (sewer, dungeon, cave), `ruins`,
`waterside` (road, shore, river, farmland, land), `wild` (every 0.25% biome),
`pocket` (pickpocket finds). About 10 entries per group and tier.

### Loading and concurrency

The corpus loads after items (it needs the authored-name snapshot), at boot
and on `bauble corpus reload`. The pool is an immutable snapshot behind an
atomic pointer, read off the lock by `Generate`. Promote, remove and reload
run under the mud lock: build the new pool, save it (overlay only), then
swap the pointer. The seed is authored content: a malformed seed fails the
CI validation test and logs ERROR at runtime without crashing. The overlay
follows the living-state contract (`util.ReadLivingState`, quarantine,
`util.Save`, persist before publish); after a quarantine the overlay
restarts empty and the quarantined file stays aside for recovery.

Every entry passes `CleanReply`, the authored-name check, and
`ApplyLimitsFor` weight bounds at load; a failing entry is skipped and
logged. The CI test loads items first and asserts the authored snapshot is
non-empty, so it cannot pass vacuously.

### Lookup

- Search and admin finds: one candidate pool merging the overlay and seed
  entries at `biome-tier` and `group-tier`. Fall to `tier` only when that
  merged pool is empty after filters. An empty or unmapped biome skips the
  biome key and its group.
- Pickpocket finds: the same merge at `pocket-tier`, then `tier`, filtered by
  `TooBigFor(entry, SourcePickpocket)`. If every entry fails the filter (for
  example `BaublePickpocketMaxWeight` lowered below all of them), the find is
  a generic trinket.
- Recent avoidance: `recent` comes from a sibling of `RecentNames` that
  includes corpus records. Prefer entries not in `recent`; if all are recent,
  take the least recent. Recency never falls through to a broader key.
- Value: stored, clamped with `tier.ClampValue`. Weight: `ApplyLimitsFor` at
  use.

### Promotion and admin

- `bauble promote <id>`: `Generator` openai, `PlayerKey` false, `Moderated`
  true, `EditedBy` empty, status anything but `retired` (sold records are
  promotable). Copied under its exact `biome-tier` or `pocket-tier` key with
  provenance.
- `Edit` clears `Moderated`, so edited text is never promotable.
- `Retire` also removes overlay entries whose `from_record` matches.
- `bauble corpus list [key]`, `bauble corpus remove <key> <n>`,
  `bauble corpus reload`, `bauble corpus export` (prints the overlay in seed
  format through `yaml.Marshal`). All admin-only, like `bauble`.
- The logic lives in `baubles.Promote` and `baubles.RemoveCorpusEntry` so the
  future /build queue calls the same functions.

### Seed content

Drafted under the `dogmud-player-copy` rules (80-column wrap, no numbers in
prose, ESL-clear). Pocket entries pass `TooBigFor`. Each pool's mean value
sits near its tier's range midpoint (tested). The owner reviews the file
before merge.

## Docs to update

`internal/baubles/context.md` (Fallback, corpus, Promote, SightPenalty, the
generator, `PlayerKey` in `GenResult`), `modules/baubles/context.md`,
`internal/items/context.md` (`AuthoredName`),
`internal/apiframework/context.md` (the new ledger API),
`modules/aicompanion/context.md` (allowances moved), `internal/configs`
`context.md` (`hardLocked`, `DisplayConfigData`), the `bauble` admin help
template and `baubleUsage` text, the `config.yaml` comment at 2631-2633
(player-key finds are now moderated) and the `Locked` comment at 94-98,
`docs/baubles/implementation-plan.md` (no-key finds come from the corpus),
`docs/aicompanion/settings.md`, and `docs/README.md` for the seed file.

## Out of scope

The /build approval queue (web builder rework arc). U6, the pre-existing
household watcher quirk (its own master PR). Everything on FinalTwist's
list. Requiring `RelayOrigin` to sit under `WebDomain` (the hard lock
covers the attack; the product rule can come later).
