# Baubles slice C: fallback corpus Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When no model names a bauble, its text comes from a hand-written, biome-keyed corpus (tracked seed plus a living-state overlay of promoted model names) instead of the one generic "Trinket".

**Architecture:** A new `internal/baubles/corpus.go` holds an immutable pool behind an atomic pointer, built from `_datafiles/world/dogmud/bauble-corpus.yaml` (seed) and `_datafiles/world/dogmud/baubles/corpus.promoted.yaml` (overlay). `baubles.Fallback` replaces all four `GenericTrinket` call sites and returns a generic trinket only when the pool has nothing that fits. `internal/baubles/corpus_admin.go` holds `Promote` and `RemoveCorpusEntry` so the admin command now, and the /build queue later, call the same functions.

**Tech Stack:** Go 1.25, `gopkg.in/yaml.v3` (already a dependency of `internal/baubles`), the living-state helpers in `internal/util`.

**Source spec:** `docs/superpowers/specs/2026-09-28-baubles-hardening-and-corpus-design.md`, section "Slice C: fallback corpus", its share of "Docs to update", and owner rulings 11 to 15.

**Delivery (owner ruling 11).** PR #175 merges first. Then slices M, H, S5 and C each go to master as their own PR. Slice C branches FRESH from master once H and S5 are merged: worktree `C:\tmp\dogmud-baubles-c`, branch `feature/bauble-corpus`, created from the main checkout (`C:\Users\Calabe Davis\workspace\DOGMud`) by Task 0. `$BASE` is the master commit it branched from: **`b5ac8b0fa`** (origin/master, recorded by Task 0 on 2026-09-29); every diff and size check in this plan is taken from `$BASE`. The main checkout's local `master` is stale (`62f9027a2`) and is not touched, so `git merge-base HEAD master` gives the WRONG commit: every step below uses `BASE=b5ac8b0fa`, and wherever this plan says `master` it means `origin/master` at `b5ac8b0fa`. Our commits never rebase onto, and never push to, FinalTwist's branch. This plan and the spec reach master through the separate docs-only PR, not through this branch.

**Runs after slices H and S5.** Slice H adds `items.AuthoredName`, the S3 and S4 cleaning inside `CleanReply`, and `FindOpts.SightPenalty`; S5 adds the per-feature allowances in `internal/apiframework/budget.go` (`DimBaublesFinder`). Task 0 checks they are on master. This plan never re-implements them; it relies on `CleanReply` refusing an authored item name.

**Subagent models:** sonnet for every code task (judgment about Go and tests), sonnet for the content drafting in Task 11. The controller runs Tasks 0, 11 (merge and owner gate), 13 and 14 itself.

---

## Facts verified against source

First read in `C:\tmp\pr175-ours` at `e711ee9de` (PR #175's head after FinalTwist's fix round) on 2026-09-28. **Re-verified at `b5ac8b0fa`** (origin/master after #175, H1, the sweep, H2, H3 #191 and S5 #193) on 2026-09-29, in `C:\tmp\dogmud-baubles-c`: every line number below is at `b5ac8b0fa`, and every quoted anchor in Tasks 1 to 12 and 14 was grepped against that tree and corrected where it had drifted. Every edit is still located by symbol and exact text, never by line alone.

| Fact | Where |
|---|---|
| `GenericTrinket(tier ValueTier, randn func(n int) int) Reply`; nil randn gives the first description, the tier midpoint and 0.1 lb; weights 0.1 to 0.8 lb. H added `genericDescriptionFor(id string) string`, the stable description everyone but the finder reads for a finder-only record | `internal/baubles/fallback.go:15-22, 32-55, 57-64` |
| Four `GenericTrinket` call sites: `search_bauble.go:237` (in `FlushBaubleDeliveries`), `steal_pocket.go:283` (in `(*pocketAttempt).naming`), `baubles/generate.go:156` (the `generic` closure in `Generate`), `baubles/mint.go:74` (only when `MintOpts.Result` is nil) | grep `GenericTrinket(`, 2026-09-29 |
| Production always passes `Result` to `Mint`, so `mint.go:74` runs only in tests | `search_bauble.go:304-312`, `steal_pocket.go:412-420` |
| `Mint` sets `StatusReady` only for `GeneratorOpenAI`, else `StatusFallback`; copies `Moderated` and `PlayerKey` from the result; a `PlayerKey` result's value is rolled by the server | `internal/baubles/mint.go:81-84, 86-92, 116-117` |
| Generators: `openai`, `local`, `admin`; statuses `ready`, `fallback`, `sold`, `retired` | `internal/baubles/record.go:14-19, 34-38` |
| `Record` has `Moderated`, `PlayerKey`, `EditedBy` (line 106, `yaml:"edited_by,omitempty"`), `Model`, `PromptVersion`, `Biome`, `Zone`, `Source`, `Tier`, and (the sweep) `LastSeenAt`, `UnseenSweeps`; there is no hand-edit flag apart from `EditedBy` | `internal/baubles/record.go:43-124` |
| `(Record).KeptToFinder() bool` is `PlayerKey && !Moderated` (H2): derived, never stored; its comment says such a record is never promotable. `(Record).View()` shows everyone but the finder `genericName` and `genericDescriptionFor(id)` for it, and `Trinket` for a retired record | `internal/baubles/record.go:126-168` |
| `Retire` sets `StatusRetired` and `EditedBy`; `Restore` sets `ready` only for `GeneratorOpenAI`, else `fallback`, and `sold` when `SoldValue > 0`; both set `EditedBy` | `internal/baubles/admin.go:96-124` |
| `Edit(id, field, value, admin string) (Record, error)` runs `CleanReply` and `ApplyLimitsFor`, sets `EditedBy`, never touches `Moderated` | `internal/baubles/admin.go:134-201` |
| `ApplyRegenerated(id string, res GenResult, admin string, randn func(n int) int) (Record, error)` (H2 added `randn`: a `PlayerKey` result's value is rolled, as in `Mint`) refuses any result whose `Generator` is not `GeneratorOpenAI`, sets `Moderated` and `PlayerKey` from the result and `EditedBy` to `admin + " (regen)"` | `internal/baubles/admin.go:203-249` |
| Callers of `Edit`: `internal/usercommands/admin.bauble.go:426` (`baubleEdit`) and six calls in `internal/baubles/admin_test.go` (`TestEdit`, lines 64 to 81). Callers of `ApplyRegenerated`: `internal/actions/bauble_admin.go:86` (`RegenerateBauble`, passing `util.Rand`), `admin_test.go` (`TestApplyRegenerated` twice, `TestApplyRegeneratedSetsPlayerKey`, `TestApplyRegeneratedRollsPlayerKeyValue` twice, the second as `got, err =`) and `generate_test.go:493` (`TestFinderOnlyReachesTheRecordAndRegenClearsIt`); every test call passes `first` | `git grep`, 2026-09-29 |
| `events.DrainQueuedMessagesForTest(userId int) []string` returns a user's queued `SendText` output; usercommands tests already read admin output with it, and `admin.bauble_test.go` already imports `events` | `internal/events/events.go:361`; `cancel_spell_test.go:58`; `admin.bauble_test.go:9, 119-128` |
| `RecentNames(zone string, n int) []string`: zone, `GeneratorOpenAI` and NOT `PlayerKey` (H: a player-key name never reaches another prompt), newest id first, under `cat.mu.RLock` | `internal/baubles/generate.go:214-236` |
| `Generate` is off the mud lock; its `generic` closure is used at four returns (no generator; an error, where a ledger refusal is logged by `noteRefusal` at most once a minute; a `CleanReply` or player-key text failure; `TooBigFor`). `` `result`, `generic trinket` `` appears four times in the file: once in `noteRefusal`, three times in `Generate` | `internal/baubles/generate.go:120-139, 150-212` |
| `GenResult` has `FinderOnly` (H2): player-key text kept to its finder | `internal/baubles/generate.go:56-70` |
| `CleanReply(r Reply) (Reply, error)` folds typography, lowercases `NameSimple` and `Material`, refuses digits in the name, an authored item's name (`items.AuthoredName`, H), a name over 40 runes or outside 1 to 6 words, a description outside 20 to 400, and text that reads as a link; rewrites an unusable keyword (reserved or `items.AuthoredKeyword`) to a word of the name or `trinket`; never touches numbers | `internal/baubles/validate.go:22-30, 117-166` |
| `reservedNouns` includes key, ring, coin, token, gem, bone, stone, shell, pearl, crystal and more | `internal/baubles/validate.go:45-60` |
| `ApplyLimitsFor(r Reply, t ValueTier, s Source) Limited` clamps value (`ClampValue`) and weight (`ClampWeightFor`) | `internal/baubles/reply.go:84-92` |
| `ClampWeight` rounds to a tenth and clamps to `MinWeightLbs` 0.1 to `MaxWeightLbs` 25 (Go constants, not config) | `internal/baubles/weight.go:14-37` |
| `MaxWeightFor(SourcePickpocket)` is `Balance.BaublePickpocketMaxWeight` when `0 < w < 25`; `TooBigFor(r Reply, s Source) bool` is false for every source but pickpocket, true when weight exceeds it or a name word is in `notPocketSized` (urn, vase, lamp, cloak, book, bowl and more) | `internal/baubles/weight.go:41-83` |
| `ValueTier.ClampValue` and `RollValue`; nil randn gives `(Min+Max)/2` | `internal/baubles/tiers.go:99, 107-114` |
| `BaseChance(biome string) float64` reads `BaubleBiomeChancePct`, else `BaubleSearchChancePct` | `internal/baubles/find.go:55-64` |
| Catalog lives at `<DataFiles>/baubles` (`catalogDir`); `Load` is boot only | `internal/baubles/catalog.go:71-81, 83-120`; `main.go:1658-1662` |
| The catalog loader reads only `catalog-*.yaml` (`shardPrefix`), quarantining only those | `internal/baubles/store.go:28, 106, 122-131` |
| The catalog sweep's disk scan walks every `.yaml` under DataFiles except `baubles/` and `economy/snapshots/` (`sweepSkipDirs`), so the overlay is never read by it; the seed at the world root is read but never parsed unless it has a `bauble:` key (`baubleKeyRe`, case-sensitive) | `internal/baubles/sweep_disk.go:21-47, 76-135` |
| `baubles` test binary has `TestMain` (logger set, `BaublesEnabled` true); `withCatalog(t) string` seeds item 900 and a temp catalog; `seedRecord`, `goodReply`, `installGenerator`, `setBaubleConfig`, `first` helpers exist | `catalog_test.go:17-50`, `admin_test.go:8-18`, `generate_test.go:17-32`, `find_test.go:12-18` |
| `util.ReadLivingState(path string) ([]byte, error)` returns `ErrStateAbsent` or `ErrStateCorrupt`; `util.QuarantineCorrupt(path string) (string, error)` fails when the file cannot be stat'ed or renamed, leaving it in place; `util.Save(path string, data []byte, doSafe ...bool) error` | `internal/util/livingstate.go:47-58, 67, 90-113`; `internal/util/util.go:804` |
| Prior art for a living-state YAML reader: `ledger.loadLocked` (read, decode, quarantine on either failure, through `ledger.quarantine`) | `internal/apiframework/budget.go:180-207` |
| `gossip.Load` panics on a bad file; `gossip.Pool(key)` is a plain map read and the fallback chain is in its caller | `internal/gossip/gossip.go:31-60, 92-94`; `internal/hooks/MobIdle_HandleIdleMobs.go:430, 494-505` |
| Boot order: `conditions.LoadDataFiles()` then `items.LoadDataFiles()` then `baubles.Load()` inside `if !isReload` | `main.go:1650-1662` |
| `items.LoadDataFiles` builds the authored snapshot (`rebuildAuthoredKeywords`, item 900 excluded); `items.AuthoredKeyword(word string) bool`; `items.AuthoredName(name string) bool`; `items.SeedItemsForTest(map[int]*ItemSpec) func()` restores the original map | `internal/items/itemspec.go:561, 593, 604, 888-908`; `internal/items/test_helpers.go:6-14` |
| A test that loads real items needs conditions first and `Network.LogoutRounds = 3` | `internal/items/shipped_light_items_test.go:15-24` |
| `Hooded Lantern` is item 20097 | `_datafiles/world/dogmud/items/armor-20000/light/20097-hooded_lantern.yaml:2` |
| 24 biome files; each `biomeid` matches its filename; no `default.yaml`, so a room with no known biome gives `Place.Biome` "" | `_datafiles/world/dogmud/biomes/`; `internal/rooms/biomes.go:201-207`; `rooms.go:2916-2931`; `search_bauble.go:391-403` |
| Shipped chances: interior 5, fort 4, ruins 3.5, sewer 3, city_backstreet 2.5, city_thoroughfare 2, dungeon 2, road 1, shore 1, farmland 0.75, cave 0.75, land 0.5, river 0.5, nine wild biomes 0.25, water 0, ether 0 | `git show HEAD:_datafiles/config.yaml` lines 1445-1469 |
| Go defaults equal the shipped bauble block (so a test binary's `BaseChance` is the shipped one) | `internal/configs/config.balance.baubles_test.go:175-228` (`TestBaubleShippedConfigMatchesDefaults`) |
| Value ladder: cheap 1 to 6, average 10 to 15, rare 40 to 200 | `config.yaml` 1546-1551 |
| `BaublePickpocketMaxWeight: 1.0` | `config.yaml` 1487 |
| Weight is NOT tiered in config: every tier is 0.1 to 25 lb (Go constants), pickpocket 1.0 lb (config) | `weight.go:14-18`, `config.yaml` 1487 |
| Stale "generic trinket" bauble config comments at `b5ac8b0fa`: 1485, 2692 (H's text), 2701, 2705 (H's `MaxConcurrent` form; S5's alternative form did not land), 2707-2708 (S5's `DailyTokensPerUser` comment, which S5 then reworded: it now says `bauble spawn` charges the admin's own allowance and `bauble regen` charges nobody) | `grep -n -i generic _datafiles/config.yaml` at `b5ac8b0fa` |
| Other stale "generic trinket" wording at `b5ac8b0fa` (Tasks 5, 6, 9 and 12 rewrite each, keyed by text): `admin.bauble.go:113, 117, 132` (spawn comment, spawn text, status line), `internal/baubles/context.md:86-88, 312, 358-360`, `mint.go:43, 52`, `generate.go:19, 90, 147` and the four `result` log values (138, 174, 197, 204), `admin.go:205` (`ApplyRegenerated`'s doc), `search_bauble.go:33, 120, 213`, `steal_pocket.go:277-278, 460`, `find.go:245`, `modules/baubles/baubles.go:10-13, 107, 124-125, 199`, `modules/baubles/generate.go:357`, `config.balance.go:908`, `modules/baubles/context.md:13, 94, 110`, `internal/actions/context.md:598-599, 805-806, 830-831, 849`, `modules/context.md:29`, `docs/README.md:196`, `docs/aicompanion/settings.md:358`. Finder-only and retired views that legitimately stay generic (unless the owner rules otherwise): `generate.go:65-69`, `record.go:156-161`, `modules/baubles/generate.go:363`, `modules/baubles/context.md:107-108`, `internal/baubles/context.md:337`, `internal/items/bauble_viewer.go:7`, `internal/items/context.md:395`, `internal/mobs/mobs.go:1093`, `docs/aicompanion/settings.md:353`, `config.yaml:2717` | grep, 2026-09-29 |
| `docs/baubles/implementation-plan.md` has `### Phase 6c: Stolen goods, fences and owners (written, after PR #175)` at line 739, **`### Phase 6d: The owner's fix round on PR #175 (written)` at line 879**, and `### Phase 7: Optional` at line 931. Phase 6d is taken, so the corpus section is **Phase 6e** | `grep -n "^### Phase" docs/baubles/implementation-plan.md` at `b5ac8b0fa` |
| The catalog prune landed with the sweep: `applySweep(now time.Time, refs map[string]bool, keep time.Duration) (referenced int, pruned int, shardErrors int)` (in package `baubles`) removes every record `(Record).prunableAt(now, keep)` allows, through `(*catalog).persistShardPruning(shard int, prune func(r *Record) bool) (int, error)`. A record is prunable only after `minUnseenSweeps` (2) complete sweeps in a row found nothing pointing at it AND `keep` has passed since its `lastEvidence()`; a record with `ReturnCreditAt` set is never pruned. The exported entry point `RunSweep(now time.Time) SweepStatus` also walks live sources and every save file. `window.go`'s `pruneLocked` still sweeps search windows only | `internal/baubles/sweep.go:52-111, 317`; `internal/baubles/catalog.go:276-335, 357-396` |
| Root guards the seed and the overlay meet: `TestEveryTextSurfaceIsRegistered` (`messaging_surface_guard_test.go:554`), `TestNoStringOrDataSaysBuff` (`identifier_word_guard_test.go:379`; it lists files with `git ls-files`, line 391, so an untracked seed is not scanned), `TestLivingStateWritesAreDurable` (`durable_write_guard_test.go:92`) and `TestNoHandRolledTempRename` (line 185) | grep, 2026-09-29 |
| In this worktree `config.yaml` carries `H` (no skip-worktree bit) | `git ls-files -v _datafiles/config.yaml`, 2026-09-29 |
| `_datafiles/**/baubles/*` is gitignored (except `.gitkeep`), so the overlay is ignored and the seed at the world root is not | `.gitignore:24-26`; `git check-ignore -v` |
| The messaging surface guard skips `baubles/`; the corpus keys (`groups`, `entries`, `name_simple`, `weight_lbs`, `from_record`, `promoted_at`, `prompt_version`) carry no text stem, and `description` is already registered | `messaging_surface_guard_test.go:216, 237-256` |
| `TestNoStringOrDataSaysBuff` scans every tracked `_datafiles/**/*.yaml` line: the seed must never spell that word | `identifier_word_guard_test.go:379-440` |
| Admin `bauble` switch, usage fallback text, spawn comment and text, status line, stats "Named:" line | `internal/usercommands/admin.bauble.go:50-75, 79-99, 112-117, 132, 375-376` |
| Pickpocket log `named` is `rec.Generator == baubles.GeneratorOpenAI`, in `resolve`, unchanged in form by H and S5 | `internal/actions/steal_pocket.go:342` |
| `pocketAttempt` has `mu`, `req baubles.GenRequest`, `randn`, `res`; `naming()` returns `(baubles.GenResult, bool)` | `internal/actions/steal_pocket.go:61-94, 275-284` |
| Actions test harness: `stubBaubleSearch` (tier average, in-line delivery), `newSearchFakeActor`, `newSearchTestRoom` (no zone, no biome), `canCarry`, `pinConfigForTest` | `search_bauble_test.go:28-73`; `search_test.go:34, 89-91`; `testsupport_test.go:35-44` |
| Room ids 9540 to 9549 and user ids 7160 to 7169 are unused in `internal/actions` tests | grep, 2026-09-29 |
| `docs/README.md`: the Reference table ends at line 24 (the `worldbuilding/` row); the `baubles/implementation-plan.md` row is line 196 and already names Phase 6d (the owner's fix round) | `docs/README.md` at `b5ac8b0fa` |
| Slice H and S5 symbols are on master: `func AuthoredName(name string) bool` (`internal/items/itemspec.go:604`), `authoredName` in `validate.go:177`, `FindOpts.SightPenalty` in `find.go`, `DimBaublesFinder` in `internal/apiframework/budget.go` | Task 0 Step 1, 2026-09-29 |

### Spec statements that did not survive the check

1. **The spec's seed count leaves out the bare-tier pools.** Lookup falls to the `tier` key, but "about 210 entries: 7 groups times 3 tiers" gives that key nothing, so it could only ever hold promoted entries, and promotion never writes a bare-tier key. It is still reached: by a room whose biome is empty or unknown, and by a pickpocket find whose pocket pool is empty after filtering. This plan adds small `cheap`, `average` and `rare` pools (about 5 each, all pocket-safe), so the seed is about 225 entries.
2. **Restore is a sibling path the spec does not name.** `Restore` (`admin.go:112-118`) marks only `GeneratorOpenAI` records `ready`; a restored corpus record would read `fallback`. Task 1 fixes it with `Mint`.
3. **Fact row 41 is imprecise:** `Restore` sets a retired record to `ready` only for `GeneratorOpenAI`; otherwise `fallback`, or `sold` when it was sold.
4. **`EditedBy` cannot be the promotion test.** `Retire`, `Restore` and `regen` all set `EditedBy`, so under the spec's "no `EditedBy`" rule a record that was ever retired, restored or regenerated could never be promoted, although its text is still the model's. The review ruled: a new `HandEdited` record field, set only by `Edit` (and cleared by `ApplyRegenerated`, whose text is the model's again), is what `Promote` checks. Task 1 adds it; Task 7 tests that regenerated, and retired-then-restored, records are promotable.
5. **A promoted record's text can change after promotion.** `Edit` and `ApplyRegenerated` rewrite the record, so the overlay entry copied from it would keep text that no longer matches anything the admin approved. Task 7 makes both remove the record's overlay entries, as `Retire` does, and report how many.
6. **`Edit` must NOT clear `Moderated` (controller ruling, 2026-09-29, after Task 1 shipped).** The spec's "`Edit` clears `Moderated`" predates H2's `KeptToFinder` (`PlayerKey && !Moderated`): clearing it hides an admin's edit of a moderated player-key record from everyone but its finder. `Edit` sets only `HandEdited`, which is what `Promote` checks (first, before `Moderated`). The corpus review fix commit replaced Task 1's `TestEditClearsModeratedAndMarksHandEdited` with `TestEditKeepsModeratedAndMarksHandEdited`; Task 1's text below is the plan as it shipped then.

### Open design questions from the 2026-09-29 re-verification (for the owner; not decided here)

The plan's steps are unchanged on each of these; they are listed because H2, H3 or S5 changed what the corpus meets.

1. **What others see of a finder-only record.** H2's `Record.View` shows everyone but the finder the generic "Trinket" (`genericName`, `genericDescriptionFor`) for a `KeptToFinder` record, and a retired record shows "Trinket" to all. The plan leaves both generic. Should either draw from the corpus instead?
2. **A ledger refusal now yields a corpus find.** When the day's budget, the baubles share or the finder's `DailyTokensPerUser` allowance (S5) refuses, `Generate` logs it through `noteRefusal` and returns `generic()`, which Task 5 routes to `FallbackFor`. So a finder over their allowance keeps getting hand-written text rather than a Trinket. Task 5 relabels `noteRefusal`'s log value to `fallback`; should that log (and the other three) name the generator actually used (`corpus` or `local`), as Task 6 does for the pickpocket log?
3. **`RecentFallbackNames` counts player-key records.** H made `RecentNames` skip `PlayerKey` records (a player-key name never reaches another prompt). The plan's `RecentFallbackNames` takes every `Named()` record, player-key ones included. It is only compared locally, never sent anywhere; confirm that is intended, or filter `PlayerKey` like `RecentNames`.
4. **Phase number.** `docs/baubles/implementation-plan.md` already has Phase 6d (the owner's fix round on PR #175), so this plan now calls the corpus Phase 6e (Tasks 10 and 12). Confirm the number.

---

## File map

| File | Action | Responsibility |
|---|---|---|
| `internal/baubles/record.go` | Modify | `GeneratorCorpus`, `Generator.Named`, `Record.HandEdited`, status comments |
| `internal/baubles/mint.go` | Modify | fallback through `Fallback`; corpus records are `ready` |
| `internal/baubles/admin.go` | Modify | `Restore` uses `Named`; `Edit` sets `HandEdited` and leaves `Moderated` (ruling 6); `ApplyRegenerated` clears `HandEdited`; `Retire`, `Edit` and `ApplyRegenerated` drop the record's overlay entries (the last two report how many) |
| `internal/actions/bauble_admin.go` | Modify | `RegenerateBauble` takes `ApplyRegenerated`'s new count and tells the admin |
| `internal/baubles/generate.go` | Modify | `generic` closure calls `FallbackFor`; `RecentFallbackNames` |
| `internal/baubles/corpus.go` | Create | types, key parsing, entry checks, loading, the pool, lookup, `Fallback`, `FallbackFor` |
| `internal/baubles/corpus_admin.go` | Create | `Promote`, `RemoveCorpusEntry`, `removePromotedFrom`, listing, export, overlay save |
| `internal/baubles/corpus_test.go` | Create | unit tests for corpus.go and the engine call sites |
| `internal/baubles/corpus_admin_test.go` | Create | promotion, removal, retire, persistence, catalog coexistence |
| `internal/baubles/corpus_seed_test.go` | Create | CI validation of the shipped seed |
| `internal/baubles/admin_test.go`, `generate_test.go` | Modify | Task 1 tests |
| `internal/actions/search_bauble.go` | Modify | `FlushBaubleDeliveries` uses `FallbackFor` |
| `internal/actions/steal_pocket.go` | Modify | `naming` uses `FallbackFor`; the `pickpocket` log names the generator |
| `internal/actions/search_bauble_test.go`, `pickpocket_test.go` | Modify | wiring tests |
| `main.go` | Modify | `baubles.LoadCorpus()` after the catalog |
| `internal/usercommands/admin.bauble.go` | Modify | `promote`, `corpus` subcommands, status, spawn, stats text |
| `internal/usercommands/admin.bauble_test.go` | Modify | admin round trip |
| `_datafiles/world/dogmud/templates/admincommands/help/command.bauble.template` | Modify | help for the new subcommands |
| `_datafiles/world/dogmud/bauble-corpus.yaml` | Create | the seed |
| `_datafiles/config.yaml` | Modify | four stale comments (from the HEAD blob) |
| `internal/baubles/context.md`, `modules/baubles/context.md`, `internal/actions/context.md`, `modules/context.md` | Modify | docs |
| `internal/baubles/find.go`, `internal/baubles/weight.go`, `internal/configs/config.balance.go`, `modules/baubles/baubles.go`, `modules/baubles/generate.go` | Modify | comments, a log value and the status text that still say "generic trinket" (Task 12 Step 9) |
| `docs/baubles/implementation-plan.md` | Modify | Phase 6e section (6d is the owner's fix round) |
| `docs/aicompanion/settings.md` | Modify | the bauble paragraph's last sentence |
| `docs/README.md` | Modify | a row for the seed file; the baubles phase row's wording. No row for this plan: it reaches master through the docs-only PR |

---

### Task 0: Preconditions (controller)

**Files:** none.

- [ ] **Step 1: Bring the main checkout's `master` up to date, and confirm #175, H and S5 are merged.** From the main checkout (never from `C:\tmp\pr175-ours`, which is FinalTwist's PR branch and is never rebased or pushed by us). `git fetch origin master:master` fast-forwards the local `master` without checking it out; it refuses, and changes nothing, if `master` is checked out or has diverged, in which case STOP and ask the controller. Run each grep on its own (a `grep` that finds nothing exits 1 and would stop an `&&` chain):

```bash
cd "/c/Users/Calabe Davis/workspace/DOGMud" && git fetch origin master:master
cd "/c/Users/Calabe Davis/workspace/DOGMud" && git log --oneline -25 master
cd "/c/Users/Calabe Davis/workspace/DOGMud" && git grep -n "func AuthoredName" master -- 'internal/items/*.go'
cd "/c/Users/Calabe Davis/workspace/DOGMud" && git grep -n "SightPenalty" master -- internal/baubles/find.go
cd "/c/Users/Calabe Davis/workspace/DOGMud" && git grep -n "AuthoredName" master -- internal/baubles/validate.go
cd "/c/Users/Calabe Davis/workspace/DOGMud" && git grep -n "DimBaublesFinder" master -- internal/apiframework/budget.go
```

Expected: the log shows the merges of PR #175 and of the slice H and S5 PRs; one `func AuthoredName(name string) bool` hit; at least one `SightPenalty` hit in `find.go` (the `FindOpts` field); at least one `AuthoredName` hit in `validate.go`; at least one `DimBaublesFinder` hit in `budget.go`. If any is missing, STOP: H or S5 is not on master, and this slice must not branch yet (the load checks would be weaker than the spec requires, and the stale-wording edits in Task 12 are written against their text).

- [ ] **Step 2: Create the worktree and record `$BASE`.**

```bash
cd "/c/Users/Calabe Davis/workspace/DOGMud" && git worktree list
cd "/c/Users/Calabe Davis/workspace/DOGMud" && git worktree add -b feature/bauble-corpus C:/tmp/dogmud-baubles-c master
cd /c/tmp/dogmud-baubles-c && BASE=$(git rev-parse HEAD) && echo "BASE=$BASE" && git status --short && git ls-files -v _datafiles/config.yaml
```

Expected: `C:/tmp/dogmud-baubles-c` was not in the first list (if it is, STOP: another session owns it); `BASE=<sha>` equal to `master`; no status output; `H _datafiles/config.yaml` (a fresh worktree has its own index, so no skip-worktree bit). Write the printed sha into the task report: shell variables do not persist between commands, so every later step that uses `$BASE` starts with `BASE=<that sha>`.

**Done 2026-09-29 (Steps 1 and 2).** #175, H1, the sweep, H2, H3 (#191) and S5 (#193) are merged; all four Step 1 greps hit on `origin/master`. The main checkout's local `master` was NOT fast-forwarded (it is stale at `62f9027a2` and is left alone); the worktree `C:/tmp/dogmud-baubles-c` was created on `feature/bauble-corpus` from `origin/master`, and **`BASE=b5ac8b0fa`**. `git merge-base HEAD master` would return the stale `62f9027a2`, so no later step uses it.

- [ ] **Step 3: Every anchor this plan quotes is in the merged code.** The edits below are keyed by exact text, first written against `e711ee9de` and the H and S5 plans, and corrected against `b5ac8b0fa` on 2026-09-29 (the list below is the corrected one). Check each against the tree now:

```bash
cd /c/tmp/dogmud-baubles-c && while IFS= read -r line; do f="${line%% :: *}"; a="${line#* :: }"; n=$(grep -cF -- "$a" "$f" 2>/dev/null); echo "${n:-0}	$f	$a"; done <<'EOF' | sort -n | head -80
internal/baubles/record.go :: StatusReady    Status = `ready`    // named by the model
internal/baubles/record.go :: GeneratorOpenAI Generator = `openai`
internal/baubles/record.go :: EditedBy      string    `yaml:"edited_by,omitempty"`
internal/baubles/mint.go :: if res.Generator == GeneratorOpenAI {
internal/baubles/mint.go :: res := GenResult{Reply: GenericTrinket(tier, randn), Generator: GeneratorLocal}
internal/baubles/mint.go :: // Result is the finished text from Generate. nil makes a generic
internal/baubles/mint.go :: // Randn picks a generic trinket's value and weight: randn(n) returns
internal/baubles/mint.go :: // o.Result, or a generic trinket), and the carrier item pointing at it. The
internal/baubles/admin.go :: if r.Generator == GeneratorOpenAI {
internal/baubles/admin.go :: r.EditedBy = admin + ` (regen)`
internal/baubles/admin.go :: // For a name or description that should not be in the game.
internal/baubles/admin.go :: func Edit(id string, field string, value string, admin string) (Record, error) {
internal/baubles/admin.go :: func ApplyRegenerated(id string, res GenResult, admin string, randn func(n int) int) (Record, error) {
internal/baubles/admin.go :: // the theft fields are kept. It refuses a generic trinket: regenerating is
internal/baubles/admin_test.go :: got, err = ApplyRegenerated(r.Id, GenResult{Reply: high, Generator: GeneratorOpenAI}, `Admin`, first)
internal/baubles/generate_test.go :: got, err := ApplyRegenerated(rec.Id, GenResult{Reply: goodReply(), Generator: GeneratorOpenAI, Moderated: true}, `Admin`, first)
internal/baubles/generate.go :: return GenResult{Reply: GenericTrinket(tier, randn), Generator: GeneratorLocal}
internal/baubles/generate.go :: `result`, `generic trinket`
internal/baubles/generate.go :: // With nothing installed, every bauble is a generic trinket (fallback.go).
internal/baubles/generate.go :: // which every bauble is a generic trinket. info may be nil.
internal/baubles/generate.go :: // generic trinket otherwise. It never fails. randn picks a generic
internal/baubles/find.go :: // named by the model and a generic trinket arrive at the same pace.
internal/baubles/context.md :: book...): its text would name that thing, so it is a generic trinket
internal/baubles/context.md :: **No API key means generic trinkets, always.**
internal/baubles/context.md :: all give a generic trinket.
internal/baubles/context.md :: `GenResult`, `Generate`, `RecentNames`.
internal/baubles/context.md :: func Edit(id string, field string, value string, admin string) (Record, error) // EditFields
internal/baubles/context.md :: func ApplyRegenerated(id string, res GenResult, admin string) (Record, error)
internal/baubles/context.md :: - `internal/usercommands/admin.bauble.go` (`bauble spawn|show|list`;
internal/baubles/context.md :: - `main.go` (`Load` at boot, `StartSweeper` before Server Ready,
internal/actions/search_bauble.go :: res = baubles.GenResult{Reply: baubles.GenericTrinket(p.d.Request.Tier, p.randn), Generator: baubles.GeneratorLocal}
internal/actions/search_bauble.go :: otherwise as the generic trinket it would have
internal/actions/search_bauble.go :: // cannot tell a model-named find from a generic trinket by its timing, and
internal/actions/search_bauble.go :: Randn    func(n int) int // the generic trinket's dice; nil means util.Rand
internal/actions/steal_pocket.go :: return baubles.GenResult{Reply: baubles.GenericTrinket(p.req.Tier, p.randn), Generator: baubles.GeneratorLocal}, false
internal/actions/steal_pocket.go :: // naming is the bauble's naming if it came back, else the generic
internal/actions/steal_pocket.go :: `named`, rec.Generator == baubles.GeneratorOpenAI
internal/actions/steal_pocket.go :: // whose naming is not back is the generic trinket it would have been).
internal/actions/bauble_admin.go :: updated, err := baubles.ApplyRegenerated(id, res, adminName, util.Rand)
internal/actions/context.md :: then given up (a generic
internal/actions/context.md :: generic trinket) WITHOUT the mud lock
internal/actions/context.md :: named if its naming came back, otherwise the generic trinket it would have
internal/actions/context.md :: The minimum wait applies to generic trinkets too
internal/usercommands/admin.bauble.go :: //	bauble status                         what names baubles (model or generic) and the search settings
internal/usercommands/admin.bauble.go :: "  bauble restore <bauble>\r\n"+
internal/usercommands/admin.bauble.go :: // model when one is set up, otherwise a generic trinket), then delivered
internal/usercommands/admin.bauble.go :: how := `a generic trinket (no model is set up)`
internal/usercommands/admin.bauble.go :: Naming: <ansi fg=\"yellow\">generic trinkets</ansi>.
internal/usercommands/admin.bauble.go :: fmt.Fprintf(&b, "  Named:   by model %d, generic %d, admin-edited %d\r\n",
internal/usercommands/admin.bauble.go :: updated, err := baubles.Edit(rec.Id, args[fieldAt], strings.Join(args[fieldAt+1:], ` `), user.Character.Name)
internal/configs/config.balance.go :: waited for before it is a generic trinket (default 5)
modules/baubles/baubles.go :: // Off by default. Switched off, it installs nothing and every bauble is a
modules/baubles/baubles.go :: `naming`, `generic trinkets`, `reason`, `Modules.baubles.Enabled is false`
modules/baubles/baubles.go :: (nothing can moderate them); every other find is a generic trinket.
modules/baubles/baubles.go :: // reports none free. A find beyond them is not queued: it is a generic
modules/baubles/generate.go :: //   - A flag always keeps the text out of the world: a generic trinket.
modules/baubles/context.md :: find with, every find is a generic "Trinket" (value and weight random within
modules/baubles/context.md :: 5. Neither route: `errNoRoute`, a generic trinket.
modules/baubles/context.md :: `ApplyLimits`; any failure anywhere is a generic trinket.
modules/context.md :: without a key every find is a generic trinket |
docs/README.md :: (a generic "Trinket" when no API key is set)
docs/README.md :: fences as real shopkeepers),
docs/aicompanion/settings.md :: or stay generic trinkets when there is none.
docs/baubles/implementation-plan.md :: ### Phase 6d: The owner's fix round on PR #175 (written)
docs/baubles/implementation-plan.md :: ### Phase 7: Optional
_datafiles/world/dogmud/templates/admincommands/help/command.bauble.template :: what names them (a finder's own key, the server key, or generic
_datafiles/world/dogmud/templates/admincommands/help/command.bauble.template :: Records live under
_datafiles/config.yaml ::   # is given up, and the bauble is a generic trinket.
_datafiles/config.yaml ::   # breaker, shared with the AI companion), else it is a generic "Trinket".
_datafiles/config.yaml ::     # trinket. `bauble spawn` charges the admin's own allowance, as a find
_datafiles/config.yaml ::     # keep it short. Failures and timeouts become generic trinkets.
_datafiles/config.yaml ::     MaxConcurrent: 4           # server-key calls at once; more are generic
_datafiles/config.yaml ::     # or their own (about 8 to 10 names). Over it, a find is a generic
main.go :: if err := baubles.Load(); err != nil {
EOF
```

Expected: every count is at least 1 (the list prints lowest first, so a `0` is on top). For each `0`, find the merged text (`grep -n "generic" <file>`, or the symbol), and write it into the step that quotes the old anchor before running that step; record each substitution in the task report. Two known movers: the `steal_pocket.go` `named` log line (FinalTwist's naming-at-reveal fix moves it and may reshape it; Task 6 replaces whatever pickpocket log reports `named`), and the `MaxConcurrent: 4` comment (H and S5 both rewrite it; Task 12 Step 5 covers both forms).

**Run 2026-09-29 at `b5ac8b0fa`.** Four anchors of the original list were at 0 and are corrected above and in the steps that quote them: `config.yaml`'s H naming comment (the `else it is a generic "Trinket".` line now closes the `breaker, shared with the AI companion),` line; Task 12 Step 5), `bauble_admin.go`'s and `admin.go`'s `ApplyRegenerated` (H2 added a `randn` argument; Tasks 1 and 7), and `modules/baubles/baubles.go`'s no-server-key detail (H2 rewrote it for finder-only text; Task 12 Step 9). Every other anchor was 1 or more; `generate.go`'s `` `result`, `generic trinket` `` is 4 (Task 5 notes the fourth, in `noteRefusal`). The anchors added in the re-verification (the `ApplyRegenerated` test callers, the context.md API and Consumers lines, the README Phase 6d clause, the settings.md line, the S5 allowance comment) were grepped at 1 too. The pickpocket log did not move in form (`steal_pocket.go:342`); H's `MaxConcurrent` form landed, S5's did not.

- [ ] **Step 4: Baseline is green.**

```bash
cd /c/tmp/dogmud-baubles-c && go test ./internal/baubles/... ./internal/actions/... ./internal/usercommands/... 2>&1 | tail -5
cd /c/tmp/dogmud-baubles-c && go test . 2>&1 | tail -3
```

Expected: `ok` for each. A red baseline is fixed or reported before Task 1.

**Run 2026-09-29 at `b5ac8b0fa`:** green. `ok` for `internal/baubles` (2.1s), `internal/actions` (56.1s), `internal/usercommands` (1.5s) and the repo root (32.3s).

- [ ] **Step 5: Re-find the four call sites and the pickpocket log** (lines move under FinalTwist's fixes, H and S5):

```bash
cd /c/tmp/dogmud-baubles-c && grep -rn "GenericTrinket(" --include=*.go . | grep -v _test.go
cd /c/tmp/dogmud-baubles-c && grep -n "named" internal/actions/steal_pocket.go
```

Expected: `fallback.go` (definition) plus the four sites; the pickpocket `mudlog.Info` line that logs `named`. If FinalTwist's naming-at-reveal fix moved the pickpocket fallback or reshaped that log line, Task 6 edits whichever `GenericTrinket(` call remains in `steal_pocket.go` and whichever pickpocket log reports `named`; record the line numbers in the task report.

**Run 2026-09-29 at `b5ac8b0fa`:** `fallback.go:36` (definition), `search_bauble.go:237` (`FlushBaubleDeliveries`), `steal_pocket.go:283` (`(*pocketAttempt).naming`), `generate.go:156` (the `generic` closure), `mint.go:74`. The pickpocket log is `steal_pocket.go:342`, in `resolve`, still `` `named`, rec.Generator == baubles.GeneratorOpenAI `` (the other `named` hits, 51, 89, 165, 243, 258, 306 and 371, are the `named` channel and comments).

- [ ] **Step 6: FinalTwist's catalog prune is on master.** His fix round prunes the catalog ("catalog never pruned" in the review of his patch). Search every file of the package, not only `catalog.go` and `store.go`:

```bash
cd /c/tmp/dogmud-baubles-c && grep -n -i "prune" internal/baubles/*.go | grep -v "_test.go" | grep -v "window.go"
```

Expected: at least one function that removes catalog records (read each hit to be sure; `window.go`'s `pruneLocked` sweeps search windows and does not count). Record its name and signature in the task report: Task 7 calls it from `TestOverlaySurvivesTheCatalog`. If nothing is found, report it to the controller now; Task 7 Step 5 and the gate (Task 14) then FAIL rather than pass without it.

**Run 2026-09-29 at `b5ac8b0fa`:** the prune is the catalog sweep's. In `internal/baubles/sweep.go:59`, unexported: `func applySweep(now time.Time, refs map[string]bool, keep time.Duration) (referenced int, pruned int, shardErrors int)`. It marks every record not in `refs` unseen once more and removes those `(Record).prunableAt(now, keep)` allows (at least `minUnseenSweeps`, 2, complete sweeps in a row without a sighting AND `keep` since `lastEvidence()`; never one with `ReturnCreditAt`), writing each touched shard through `(*catalog).persistShardPruning(shard int, prune func(r *Record) bool) (int, error)` (`catalog.go:286`) before dropping the records from memory. `RunSweep(now time.Time) SweepStatus` (`sweep.go:317`) is the exported entry point, but it also walks the live sources and every save file under DataFiles.

---

### Task 1: The corpus generator, and the statuses that follow from it

**Files:**
- Modify: `internal/baubles/record.go` (the `Status` and `Generator` const blocks, `Record.HandEdited`)
- Modify: `internal/baubles/mint.go` (the `status :=` block)
- Modify: `internal/baubles/admin.go` (`Restore`, `Edit`, `ApplyRegenerated`)
- Test: `internal/baubles/admin_test.go`, `internal/baubles/generate_test.go`

- [ ] **Step 1: Write the failing tests.** Append to `internal/baubles/admin_test.go`:

```go
// A corpus record is named text, like a model's: restoring it after a
// retire makes it ready again, not a generic fallback.
func TestRestoreReturnsACorpusRecordToReady(t *testing.T) {
	withCatalog(t)
	r := seedRecord(t, Record{Name: `Chipped Clay Marble`, NameSimple: `marble`, Tier: TierCheap, Value: 2, Status: StatusReady, Generator: GeneratorCorpus})
	if err := Retire(r.Id, `Admin`); err != nil {
		t.Fatal(err)
	}
	if err := Restore(r.Id, `Admin`); err != nil {
		t.Fatal(err)
	}
	if got, _ := Get(r.Id); got.Status != StatusReady {
		t.Fatalf("a restored corpus record is ready, got %s", got.Status)
	}
}

// Edited text was never moderated as written, so an edit clears the flag
// and marks the record HandEdited (which Promote refuses).
func TestEditClearsModeratedAndMarksHandEdited(t *testing.T) {
	withCatalog(t)
	r := seedRecord(t, Record{Name: `Painted Wooden Horse`, NameSimple: `horse`, Tier: TierCheap, Value: 3, WeightLbs: 0.6, Status: StatusReady, Generator: GeneratorOpenAI, Moderated: true})
	got, err := Edit(r.Id, `name`, `Painted Wooden Pony`, `Admin`)
	if err != nil {
		t.Fatal(err)
	}
	if got.Moderated {
		t.Fatal("an edit must clear Moderated")
	}
	if !got.HandEdited {
		t.Fatal("an edit must mark the record HandEdited")
	}
}

// Retire and Restore change no text, so they never mark a record
// HandEdited; a regeneration writes the model's text again and clears it.
func TestOnlyEditMarksHandEdited(t *testing.T) {
	withCatalog(t)
	r := seedRecord(t, Record{Name: `Painted Wooden Horse`, NameSimple: `horse`, Tier: TierCheap, Value: 3, WeightLbs: 0.6, Status: StatusReady, Generator: GeneratorOpenAI, Moderated: true})
	if err := Retire(r.Id, `Admin`); err != nil {
		t.Fatal(err)
	}
	if err := Restore(r.Id, `Admin`); err != nil {
		t.Fatal(err)
	}
	if got, _ := Get(r.Id); got.HandEdited || got.EditedBy == `` {
		t.Fatalf("retire and restore set EditedBy, never HandEdited: %+v", got)
	}
	h := seedRecord(t, Record{Name: `Painted Wooden Pony`, NameSimple: `pony`, Tier: TierCheap, Value: 3, WeightLbs: 0.6, Status: StatusReady, Generator: GeneratorOpenAI, HandEdited: true})
	got, err := ApplyRegenerated(h.Id, GenResult{Reply: goodReply(), Generator: GeneratorOpenAI, Model: `gpt-test`, Moderated: true}, `Admin`, first)
	if err != nil {
		t.Fatal(err)
	}
	if got.HandEdited {
		t.Fatal("regenerated text is the model's again: HandEdited is cleared")
	}
}

// A guard, not a red-first test: ApplyRegenerated already refuses anything
// but GeneratorOpenAI, so this fails only to build until GeneratorCorpus
// exists. It pins that regenerating wants a NEW model name: a corpus answer
// is refused like a generic one, so a failed call never swaps a model name
// for corpus text.
func TestApplyRegeneratedRefusesACorpusAnswer(t *testing.T) {
	withCatalog(t)
	r := seedRecord(t, Record{Name: `Painted Wooden Horse`, NameSimple: `horse`, Tier: TierCheap, Value: 3, Status: StatusReady, Generator: GeneratorOpenAI})
	if _, err := ApplyRegenerated(r.Id, GenResult{Reply: goodReply(), Generator: GeneratorCorpus, Model: `corpus:cheap`}, `Admin`, first); err == nil {
		t.Fatal("a corpus answer must not replace a record's text")
	}
}
```

Append to `internal/baubles/generate_test.go`:

```go
// A find drawn from the corpus is named text: Mint marks it ready and keeps
// its generator and pool.
func TestMintMarksACorpusResultReady(t *testing.T) {
	withCatalog(t)
	res := GenResult{Reply: Reply{Name: `Chipped Clay Marble`, NameSimple: `marble`, Description: `A small clay marble, glazed blue long ago.`, WeightLbs: 0.1, Value: 2}, Generator: GeneratorCorpus, Model: `corpus:street-cheap`}
	_, rec, err := Mint(MintOpts{Source: SourceSearch, Tier: TierCheap, Result: &res})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != StatusReady || rec.Generator != GeneratorCorpus || rec.Model != `corpus:street-cheap` || rec.Moderated || rec.PlayerKey {
		t.Fatalf("corpus record: %+v", rec)
	}
}
```

- [ ] **Step 2: Run to see them fail.**

```bash
cd /c/tmp/dogmud-baubles-c && go test ./internal/baubles/ -run 'TestRestoreReturnsACorpusRecordToReady|TestEditClearsModeratedAndMarksHandEdited|TestOnlyEditMarksHandEdited|TestApplyRegeneratedRefusesACorpusAnswer|TestMintMarksACorpusResultReady' -v 2>&1 | tail -15
```

Expected: build failure, `undefined: GeneratorCorpus` and `unknown field HandEdited`.

- [ ] **Step 3: Implement.** In `internal/baubles/record.go` replace the two status lines:

```go
	StatusReady    Status = `ready`    // named by the model, or drawn from the fallback corpus
	StatusFallback Status = `fallback` // a generic trinket: nothing named it and the corpus had nothing that fit
```

Replace the generator const block:

```go
const (
	GeneratorOpenAI Generator = `openai`
	GeneratorLocal  Generator = `local`  // a generic trinket (fallback.go)
	GeneratorCorpus Generator = `corpus` // drawn from the fallback corpus (corpus.go); Model is "corpus:<key>"
	GeneratorAdmin  Generator = `admin`
)

// Named reports text that is a real name (the model's or the corpus's), as
// opposed to the generic trinket. Named records are ready.
func (g Generator) Named() bool {
	return g == GeneratorOpenAI || g == GeneratorCorpus
}
```

In `internal/baubles/mint.go` replace

```go
	status := StatusFallback
	if res.Generator == GeneratorOpenAI {
		status = StatusReady
	}
```

with

```go
	status := StatusFallback
	if res.Generator.Named() {
		status = StatusReady
	}
```

In `internal/baubles/record.go` replace the line

```go
	EditedBy      string    `yaml:"edited_by,omitempty"`
```

with

```go
	EditedBy      string    `yaml:"edited_by,omitempty"`

	// HandEdited is set by Edit: an admin wrote some of this text, so it is
	// not the model's answer as moderation passed it, and Promote refuses
	// it. ApplyRegenerated clears it (the text is the model's again).
	// Retire, Restore and regen set EditedBy, never this.
	HandEdited bool `yaml:"hand_edited,omitempty"`
```

(The blank line keeps the new field out of the audit block's gofmt alignment. `omitempty` means no existing catalog shard changes and no migration is needed: an absent key reads false.)

In `internal/baubles/admin.go`, inside `Restore`, replace `if r.Generator == GeneratorOpenAI {` with `if r.Generator.Named() {`. Inside `Edit`'s `Update` closure, after `r.EditedBy = admin` add:

```go
		// Hand-written text was never moderated as it now reads.
		r.Moderated = false
		r.HandEdited = true
```

Inside `ApplyRegenerated`'s `Update` closure, after ``r.EditedBy = admin + ` (regen)` `` add:

```go
		// The text is the model's again, as moderation passed it.
		r.HandEdited = false
```

- [ ] **Step 4: Run to see them pass, then the package.**

```bash
cd /c/tmp/dogmud-baubles-c && go test ./internal/baubles/ -run 'TestRestoreReturnsACorpusRecordToReady|TestEditClearsModeratedAndMarksHandEdited|TestOnlyEditMarksHandEdited|TestApplyRegeneratedRefusesACorpusAnswer|TestMintMarksACorpusResultReady' -v 2>&1 | tail -10
cd /c/tmp/dogmud-baubles-c && go test ./internal/baubles/ 2>&1 | tail -3
```

Expected: PASS, then `ok`.

- [ ] **Step 5: Null probes (two).** (a) Delete the `r.Moderated = false` line, rerun `TestEditClearsModeratedAndMarksHandEdited`, confirm it fails with "an edit must clear Moderated", restore. (b) Delete the `r.HandEdited = false` line in `ApplyRegenerated`, rerun `TestOnlyEditMarksHandEdited`, confirm it fails with "HandEdited is cleared", restore. Confirm green.

- [ ] **Step 6: Commit.**

```bash
cd /c/tmp/dogmud-baubles-c && git add internal/baubles/record.go internal/baubles/mint.go internal/baubles/admin.go internal/baubles/admin_test.go internal/baubles/generate_test.go
git commit -F - <<'EOF'
feat(baubles): a corpus generator; corpus records are ready; HandEdited

GeneratorCorpus is text drawn from the fallback corpus. Mint and Restore
treat it as named text (Generator.Named). Edit clears Moderated and sets
the new HandEdited flag, which regen clears; Retire and Restore leave it,
so only hand-written text is kept out of the corpus.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: Corpus types, keys and entry checks

**Files:**
- Create: `internal/baubles/corpus.go`
- Test: `internal/baubles/corpus_test.go`

- [ ] **Step 1: Write the failing tests.** Create `internal/baubles/corpus_test.go`:

```go
package baubles

import (
	"testing"
)

func TestParseCorpusKey(t *testing.T) {
	cases := []struct {
		key    string
		prefix string
		tier   ValueTier
		ok     bool
	}{
		{`interior-cheap`, `interior`, TierCheap, true},
		{`city_backstreet-rare`, `city_backstreet`, TierRare, true},
		{`Pocket-Average`, `pocket`, TierAverage, true},
		{`cheap`, ``, TierCheap, true},
		{`interior`, ``, ``, false},
		{`interior-legendary`, ``, ``, false},
		{`-cheap`, ``, ``, false},
	}
	for _, c := range cases {
		prefix, tier, ok := parseCorpusKey(c.key)
		if prefix != c.prefix || tier != c.tier || ok != c.ok {
			t.Errorf("%q: got (%q, %q, %v), want (%q, %q, %v)", c.key, prefix, tier, ok, c.prefix, c.tier, c.ok)
		}
	}
	if corpusKey(`dwelling`, TierRare) != `dwelling-rare` || corpusKey(``, TierCheap) != `cheap` {
		t.Fatal("corpusKey is parseCorpusKey's inverse")
	}
}

// An entry gets the checks a model's answer gets, plus an exact weight.
func TestCheckEntry(t *testing.T) {
	good := CorpusEntry{Name: `Bent Tin Thimble`, NameSimple: `thimble`, Description: `A tin thimble, pressed a little out of shape.`, Material: `Tin`, WeightLbs: 0.1, Value: 3}
	got, err := checkEntry(good)
	if err != nil {
		t.Fatal(err)
	}
	if got.Material != `tin` || got.NameSimple != `thimble` || got.Value != 3 || got.WeightLbs != 0.1 {
		t.Fatalf("cleaned like a reply, numbers untouched: %+v", got)
	}
	bad := map[string]func(e *CorpusEntry){
		`digits in the name`: func(e *CorpusEntry) { e.Name = `Tin Thimble 2` },
		`too heavy`:          func(e *CorpusEntry) { e.WeightLbs = 30 },
		`not a tenth`:        func(e *CorpusEntry) { e.WeightLbs = 0.15 },
		`no weight`:          func(e *CorpusEntry) { e.WeightLbs = 0 },
		`short description`:  func(e *CorpusEntry) { e.Description = `Tiny.` },
	}
	for name, change := range bad {
		e := good
		change(&e)
		if _, err := checkEntry(e); err == nil {
			t.Errorf("%s: must be refused", name)
		}
	}
}

// A typo in a field name is an error, not a silently empty field.
func TestDecodeStrictRefusesUnknownFields(t *testing.T) {
	var doc seedDoc
	if err := decodeStrict([]byte("entries:\n  cheap:\n    - name: X\n      valeu: 3\n"), &doc); err == nil {
		t.Fatal("an unknown field must be refused")
	}
	if err := decodeStrict([]byte(``), &doc); err != nil {
		t.Fatalf("an empty document is empty, not an error: %v", err)
	}
}
```

- [ ] **Step 2: Run to see it fail.**

```bash
cd /c/tmp/dogmud-baubles-c && go test ./internal/baubles/ -run 'TestParseCorpusKey|TestCheckEntry|TestDecodeStrict' 2>&1 | tail -5
```

Expected: build failure, `undefined: parseCorpusKey`.

- [ ] **Step 3: Implement.** Create `internal/baubles/corpus.go`:

```go
package baubles

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// The fallback corpus (docs/superpowers/specs/2026-09-28-baubles-hardening-
// and-corpus-design.md, slice C): hand-written bauble text, keyed by where a
// find was made, used whenever no model names it. Two layers:
//
//   - the seed, <DataFiles>/bauble-corpus.yaml: tracked, authored content.
//   - the overlay, <DataFiles>/baubles/corpus.promoted.yaml: living state,
//     model names an admin promoted (`bauble promote`, corpus_admin.go).
//
// Both are read into one immutable pool behind an atomic pointer, so
// Fallback reads it off the mud lock. Writers build a new pool, save the
// overlay, and only then swap the pointer (persist before publish).

const (
	seedFileName    = `bauble-corpus.yaml`
	overlayFileName = `corpus.promoted.yaml`
	// pocketPrefix keys the pools for pickpocketed finds: pocket-<tier>.
	pocketPrefix = `pocket`
	// fallbackRecentNames is how many of a zone's newest named finds a
	// fallback avoids repeating.
	fallbackRecentNames = 12
)

// CorpusEntry is one piece of bauble text, in the shape a model answers.
type CorpusEntry struct {
	Name        string  `yaml:"name"`
	NameSimple  string  `yaml:"name_simple"`
	Description string  `yaml:"description"`
	Material    string  `yaml:"material,omitempty"`
	WeightLbs   float64 `yaml:"weight_lbs"`
	Value       int     `yaml:"value"`
}

func (e CorpusEntry) reply() Reply {
	return Reply{Name: e.Name, NameSimple: e.NameSimple, Description: e.Description, Material: e.Material, WeightLbs: e.WeightLbs, Value: e.Value}
}

func entryFrom(r Reply) CorpusEntry {
	return CorpusEntry{Name: r.Name, NameSimple: r.NameSimple, Description: r.Description, Material: r.Material, WeightLbs: r.WeightLbs, Value: r.Value}
}

// PromotedEntry is an overlay entry: the text, plus where it came from.
type PromotedEntry struct {
	CorpusEntry `yaml:",inline"`
	// FromRecord is informational: it dangles once the catalog prunes the
	// record. Retire removes the entries that name it.
	FromRecord    string    `yaml:"from_record"`
	Zone          string    `yaml:"zone,omitempty"`
	Biome         string    `yaml:"biome,omitempty"`
	Model         string    `yaml:"model,omitempty"`
	PromptVersion int       `yaml:"prompt_version,omitempty"`
	PromotedAt    time.Time `yaml:"promoted_at"`
}

// seedDoc is the seed file (and the export format).
type seedDoc struct {
	Groups  map[string]string        `yaml:"groups,omitempty"` // biome -> group
	Entries map[string][]CorpusEntry `yaml:"entries"`
}

// overlayDoc is the overlay file.
type overlayDoc struct {
	Entries map[string][]PromotedEntry `yaml:"entries"`
}

func normKey(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// parseCorpusKey splits "interior-cheap" into ("interior", cheap) and a
// bare tier "cheap" into ("", cheap). Biome ids use underscores, never
// hyphens, so the last hyphen is the split.
func parseCorpusKey(key string) (prefix string, tier ValueTier, ok bool) {
	key = normKey(key)
	if t, ok := ParseTier(key); ok {
		return ``, t, true
	}
	i := strings.LastIndex(key, `-`)
	if i <= 0 {
		return ``, ``, false
	}
	t, ok := ParseTier(key[i+1:])
	if !ok {
		return ``, ``, false
	}
	return key[:i], t, true
}

// corpusKey is parseCorpusKey's inverse.
func corpusKey(prefix string, tier ValueTier) string {
	if prefix == `` {
		return string(tier)
	}
	return prefix + `-` + string(tier)
}

// decodeStrict decodes YAML refusing unknown fields, so a typo in a field
// name fails loudly. An empty document decodes to nothing.
func decodeStrict(data []byte, out any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// checkEntry runs an entry through what a model's answer must pass
// (CleanReply, which also refuses an authored item's name since slice H)
// and requires a weight already inside the bauble bounds, in tenths of a
// pound. It returns the cleaned entry. Values are not checked here: they
// are clamped into the tier when used.
func checkEntry(e CorpusEntry) (CorpusEntry, error) {
	cleaned, err := CleanReply(e.reply())
	if err != nil {
		return CorpusEntry{}, err
	}
	if ClampWeight(e.WeightLbs) != e.WeightLbs {
		return CorpusEntry{}, fmt.Errorf(`%w: weight %v lb is not a tenth of a pound from %.1f to %.1f`, ErrUnusableReply, e.WeightLbs, MinWeightLbs, MaxWeightLbs)
	}
	return entryFrom(cleaned), nil
}
```

- [ ] **Step 4: Run to see them pass.**

```bash
cd /c/tmp/dogmud-baubles-c && go test ./internal/baubles/ -run 'TestParseCorpusKey|TestCheckEntry|TestDecodeStrict' -v 2>&1 | tail -8
```

Expected: three PASS.

- [ ] **Step 5: Null probe.** Change `dec.KnownFields(true)` to `false`, confirm `TestDecodeStrictRefusesUnknownFields` fails with "an unknown field must be refused", restore.

- [ ] **Step 6: Commit.**

```bash
cd /c/tmp/dogmud-baubles-c && git add internal/baubles/corpus.go internal/baubles/corpus_test.go
git commit -F - <<'EOF'
feat(baubles): corpus entry types, pool keys and entry checks

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: Loading the seed and the overlay

**Files:**
- Modify: `internal/baubles/corpus.go`
- Test: `internal/baubles/corpus_test.go`

- [ ] **Step 1: Write the failing tests.** In `internal/baubles/corpus_test.go` replace the import block with

```go
import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/util"
)
```

and append:

```go
// testSeed is a small seed: interior and fort in the dwelling group, ruins
// its own group, a bare cheap pool whose one entry is priced far above the
// tier (the clamp test), and a pocket pool with one entry too heavy for a
// pocket.
const testSeed = `groups:
  interior: dwelling
  fort: dwelling
  ruins: ruins
entries:
  interior-cheap:
    - name: Bent Tin Thimble
      name_simple: thimble
      description: A tin thimble, pressed a little out of shape by a careless heel.
      material: tin
      weight_lbs: 0.1
      value: 3
  dwelling-cheap:
    - name: Chipped Clay Marble
      name_simple: marble
      description: A small clay marble, glazed blue long ago and chipped since.
      material: clay
      weight_lbs: 0.1
      value: 2
  cheap:
    - name: Knotted Twine Bracelet
      name_simple: bracelet
      description: A bracelet of knotted brown twine, frayed where a wrist rubbed it.
      material: twine
      weight_lbs: 0.1
      value: 99
  pocket-cheap:
    - name: Brass Snuff Spoon
      name_simple: spoon
      description: A tiny brass spoon for snuff, its bowl no bigger than a fingernail.
      material: brass
      weight_lbs: 0.1
      value: 2
    - name: Heavy Pewter Tankard
      name_simple: tankard
      description: A pewter tankard with a hinged lid, far too heavy for any pocket.
      material: pewter
      weight_lbs: 1.5
      value: 5
  ruins-average:
    - name: Faded Mosaic Tile
      name_simple: tile
      description: A square tile from an old floor, painted with half of a red bird.
      material: fired clay
      weight_lbs: 0.3
      value: 13
`

const testSpoolOverlay = `entries:
  interior-cheap:
    - name: Painted Wooden Spool
      name_simple: spool
      description: A wooden thread spool painted with a band of faded blue.
      material: wood
      weight_lbs: 0.2
      value: 4
      from_record: B0000007
      zone: ashwick
      biome: interior
      model: gpt-test
      prompt_version: 3
      promoted_at: 2026-09-28T12:00:00Z
`

func writeTestFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}

// withCorpus writes a seed and an overlay (either may be empty, meaning no
// file) to a temp dir and loads them. The corpus is cleared afterwards.
func withCorpus(t *testing.T, seed, overlay string) (seedPath, overlayPath string, rep CorpusReport) {
	t.Helper()
	dir := t.TempDir()
	seedPath = filepath.Join(dir, seedFileName)
	overlayPath = filepath.Join(dir, `baubles`, overlayFileName)
	if seed != `` {
		writeTestFile(t, seedPath, seed)
	}
	if overlay != `` {
		writeTestFile(t, overlayPath, overlay)
	}
	t.Cleanup(ClearCorpusForTest)
	return seedPath, overlayPath, LoadCorpusFrom(seedPath, overlayPath)
}

func TestLoadCorpusReadsBothLayers(t *testing.T) {
	_, _, rep := withCorpus(t, testSeed, testSpoolOverlay)
	if rep.Seed != 6 || rep.Promoted != 1 || len(rep.Skipped) != 0 || rep.SeedErr != nil || rep.Quarantined != `` {
		t.Fatalf("report: %+v", rep)
	}
	if g, ok := GroupOf(`Interior`); !ok || g != `dwelling` {
		t.Fatalf("interior is in dwelling, got %q %v", g, ok)
	}
	if _, ok := GroupOf(`water`); ok {
		t.Fatal("water has no group")
	}
	if seed, promoted := CorpusCounts(); seed != 6 || promoted != 1 {
		t.Fatalf("counts: %d seed, %d promoted", seed, promoted)
	}
}

// Every entry must pass what a model's answer passes; one that fails is
// skipped and reported, and a key no pool can have is skipped whole.
func TestLoadCorpusSkipsWhatAModelAnswerCouldNotPass(t *testing.T) {
	seed := testSeed + `  interior-rare:
    - name: Silver Cup Number 2
      name_simple: cup
      description: A silver cup with a number scratched on its base.
      weight_lbs: 0.5
      value: 90
    - name: Stone Idol Head
      name_simple: idol
      description: A stone head broken from a small idol, far heavier than it looks.
      weight_lbs: 40
      value: 120
    - name: Plain Pewter Cup
      name_simple: cup
      description: A plain pewter drinking cup, dented on one side.
      material: pewter
      weight_lbs: 0.5
      value: 60
  castle-cheap:
    - name: Rusty Castle Nail
      name_simple: nail
      description: A square iron nail, rusted almost through.
      weight_lbs: 0.1
      value: 1
`
	_, _, rep := withCorpus(t, seed, ``)
	if rep.Seed != 7 || len(rep.Skipped) != 3 {
		t.Fatalf("seven usable, three skipped (digits, weight, unknown key): %+v", rep)
	}
}

// A broken seed is logged and the corpus runs without it: the overlay
// still loads (a bare tier key needs no groups) and nothing panics.
func TestMalformedSeedIsLoggedNotFatal(t *testing.T) {
	overlay := `entries:
  cheap:
    - name: Painted Wooden Spool
      name_simple: spool
      description: A wooden thread spool painted with a band of faded blue.
      weight_lbs: 0.2
      value: 4
      from_record: B0000007
      promoted_at: 2026-09-28T12:00:00Z
`
	_, _, rep := withCorpus(t, "entries: [this is not a map\n", overlay)
	if rep.SeedErr == nil || rep.Seed != 0 || rep.Promoted != 1 {
		t.Fatalf("report: %+v", rep)
	}
}

// A corrupt overlay is living state: moved aside (never deleted), logged,
// and the overlay restarts empty. An empty file is empty, not corrupt.
func TestCorruptOverlayIsQuarantined(t *testing.T) {
	seedPath, overlayPath, rep := withCorpus(t, testSeed, "entries: {this is: [not closed\n")
	if rep.Quarantined == `` || rep.Promoted != 0 || rep.Seed != 6 {
		t.Fatalf("report: %+v", rep)
	}
	if _, err := os.Stat(overlayPath); !os.IsNotExist(err) {
		t.Fatalf("the corrupt file is moved aside: %v", err)
	}
	if _, err := os.Stat(rep.Quarantined); err != nil {
		t.Fatalf("the quarantined copy is kept for recovery: %v", err)
	}
	writeTestFile(t, overlayPath, ``)
	if rep := LoadCorpusFrom(seedPath, overlayPath); rep.Quarantined != `` {
		t.Fatalf("an empty overlay must not be quarantined: %+v", rep)
	}
}

func TestNoCorpusFilesIsAnEmptyCorpus(t *testing.T) {
	_, _, rep := withCorpus(t, ``, ``)
	if rep.Seed != 0 || rep.Promoted != 0 || rep.SeedErr != nil || rep.Quarantined != `` {
		t.Fatalf("report: %+v", rep)
	}
}

// A reload that cannot read the seed keeps the seed already in use (a typo
// in a hand edit must not empty every pool); the overlay is still read
// again. A first load has nothing to keep.
func TestReloadWithABrokenSeedKeepsTheSeedInUse(t *testing.T) {
	seedPath, overlayPath, _ := withCorpus(t, testSeed, ``)
	writeTestFile(t, seedPath, "entries: [this is not a map\n")
	writeTestFile(t, overlayPath, testSpoolOverlay)
	rep := LoadCorpusFrom(seedPath, overlayPath)
	if rep.SeedErr == nil || !rep.SeedKept || rep.Seed != 6 || rep.Promoted != 1 {
		t.Fatalf("report: %+v", rep)
	}
	if g, ok := GroupOf(`fort`); !ok || g != `dwelling` {
		t.Fatalf("the kept seed keeps its groups too, got %q %v", g, ok)
	}
	if seed, _ := CorpusCounts(); seed != 6 {
		t.Fatalf("the kept seed is in use: %d", seed)
	}
}

// A corrupt overlay that cannot be moved aside is still where a save would
// write: the pool marks it broken (writers refuse, Task 7) and the next
// reload that can move it aside clears the mark.
func TestAnOverlayThatCannotBeQuarantinedIsBroken(t *testing.T) {
	quarantineOverlay = func(string) (string, error) { return ``, errors.New(`disk says no`) }
	t.Cleanup(func() { quarantineOverlay = util.QuarantineCorrupt })
	corrupt := "entries: {this is: [not closed\n"
	seedPath, overlayPath, rep := withCorpus(t, testSeed, corrupt)
	if !rep.OverlayBroken || rep.Quarantined != `` || rep.Promoted != 0 || rep.Seed != 6 {
		t.Fatalf("report: %+v", rep)
	}
	if data, err := os.ReadFile(overlayPath); err != nil || string(data) != corrupt {
		t.Fatalf("the file is left exactly as it was: %v", err)
	}
	quarantineOverlay = util.QuarantineCorrupt
	if rep := LoadCorpusFrom(seedPath, overlayPath); rep.OverlayBroken || rep.Quarantined == `` {
		t.Fatalf("moved aside on the next reload, and writable again: %+v", rep)
	}
}
```

- [ ] **Step 2: Run to see them fail.**

```bash
cd /c/tmp/dogmud-baubles-c && go test ./internal/baubles/ -run 'TestLoadCorpus|TestMalformedSeed|TestCorruptOverlay|TestNoCorpusFiles|TestReloadWithABrokenSeed|TestAnOverlayThatCannotBeQuarantined' 2>&1 | tail -5
```

Expected: build failure, `undefined: CorpusReport`.

- [ ] **Step 3: Implement.** In `internal/baubles/corpus.go` replace the import block with

```go
import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/util"
	"gopkg.in/yaml.v3"
)
```

and append:

```go
// promotedSlot is one overlay entry as loaded. raw is what is saved, exactly
// as read or promoted; use is the checked text. An entry that fails the
// checks (ok false) is kept and saved again, never used: the overlay is
// living state, and a rewrite must not drop it.
type promotedSlot struct {
	raw PromotedEntry
	use CorpusEntry
	ok  bool
	why string
}

// corpusPool is one immutable snapshot of the corpus.
type corpusPool struct {
	seedPath    string
	overlayPath string
	groups      map[string]string        // biome -> group
	seed        map[string][]CorpusEntry // key -> checked entries
	promoted    map[string][]promotedSlot
	// overlayBroken: the overlay file could not be read and could not be
	// moved aside either, so it still sits where a save would write. Every
	// overlay writer refuses (ErrOverlayBroken) until a reload clears it.
	overlayBroken bool
}

var (
	corpus atomic.Pointer[corpusPool]
	// corpusWriteMu serialises the writers (load, promote, remove, retire,
	// edit, regen). They also run under the mud lock; this keeps tests and
	// any future caller honest. Readers never take it.
	corpusWriteMu sync.Mutex
	// quarantineOverlay moves a corrupt overlay aside. A variable so a test
	// can make it fail.
	quarantineOverlay = util.QuarantineCorrupt
)

// knownPrefix reports a key prefix a pool may have: a biome in groups, a
// group, pocket, or none (a bare tier).
func (p *corpusPool) knownPrefix(prefix string) bool {
	if prefix == `` || prefix == pocketPrefix {
		return true
	}
	if _, ok := p.groups[prefix]; ok {
		return true
	}
	for _, g := range p.groups {
		if g == prefix {
			return true
		}
	}
	return false
}

// CorpusReport is what a load found.
type CorpusReport struct {
	Seed          int      // seed entries in use
	Promoted      int      // overlay entries in use
	Skipped       []string // one line per entry or key not used, and why
	SeedErr       error    // the seed could not be read or parsed
	SeedKept      bool     // SeedErr on a reload: the seed already in use was kept
	Quarantined   string   // where a corrupt overlay was moved
	OverlayBroken bool     // a corrupt overlay could not be moved aside: no writes until a reload
}

// LoadCorpus reads the seed and the overlay of the configured world. Call
// after items.LoadDataFiles (entries are checked against the authored item
// names) and after Load (the overlay sits in the catalog's directory). It
// never fails: a broken seed is logged at ERROR and the corpus runs
// without it (a reload keeps the seed already in use); a corrupt overlay is
// quarantined and starts empty, or, when it cannot be moved aside, is left
// in place and marked broken so nothing overwrites it.
func LoadCorpus() CorpusReport {
	return LoadCorpusFrom(
		util.FilePath(configs.GetFilePathsConfig().DataFiles.String(), `/`, seedFileName),
		util.FilePath(catalogDir(), `/`, overlayFileName),
	)
}

// ReloadCorpus reads the loaded corpus's own files again (the admin
// `bauble corpus reload`), or the configured world's when none is loaded.
func ReloadCorpus() CorpusReport {
	if p := corpus.Load(); p != nil {
		return LoadCorpusFrom(p.seedPath, p.overlayPath)
	}
	return LoadCorpus()
}

// LoadCorpusFrom is LoadCorpus from these two files. Tests use it.
func LoadCorpusFrom(seedPath, overlayPath string) CorpusReport {
	corpusWriteMu.Lock()
	defer corpusWriteMu.Unlock()
	pool, rep := readCorpus(seedPath, overlayPath, corpus.Load())
	corpus.Store(pool)
	for _, s := range rep.Skipped {
		mudlog.Warn(`baubles.LoadCorpus`, `skipped`, s)
	}
	if rep.SeedErr != nil {
		mudlog.Error(`baubles.LoadCorpus`, `seed`, seedPath, `error`, rep.SeedErr, `keptSeedInUse`, rep.SeedKept)
	}
	mudlog.Info(`baubles.LoadCorpus()`, `seed`, rep.Seed, `promoted`, rep.Promoted, `skipped`, len(rep.Skipped))
	return rep
}

// ClearCorpusForTest empties the corpus: every fallback is a generic
// trinket again.
func ClearCorpusForTest() {
	corpus.Store(nil)
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// readCorpus reads both files into a new pool. prev is the pool in use (nil
// at boot): when the seed cannot be read and prev was read from the same
// seed file, prev's groups and seed entries are kept.
func readCorpus(seedPath, overlayPath string, prev *corpusPool) (*corpusPool, CorpusReport) {
	pool := &corpusPool{
		seedPath:    seedPath,
		overlayPath: overlayPath,
		groups:      map[string]string{},
		seed:        map[string][]CorpusEntry{},
		promoted:    map[string][]promotedSlot{},
	}
	var rep CorpusReport
	seedName := filepath.Base(seedPath)

	// The seed: authored content. Missing means a world with no corpus.
	var sd seedDoc
	if data, err := os.ReadFile(seedPath); err == nil {
		if err := decodeStrict(data, &sd); err != nil {
			rep.SeedErr = err
			sd = seedDoc{}
		}
	} else if !os.IsNotExist(err) {
		rep.SeedErr = err
	}
	if rep.SeedErr != nil && prev != nil && prev.seedPath == seedPath {
		// A reload that cannot read the seed keeps the seed in use: a typo
		// in a hand edit must not empty every pool until the next reload.
		// The maps are never written after a pool is built, so sharing them
		// is safe. The overlay below is still read again.
		pool.groups, pool.seed = prev.groups, prev.seed
		for _, list := range prev.seed {
			rep.Seed += len(list)
		}
		rep.SeedKept = true
	}
	for _, biome := range sortedKeys(sd.Groups) {
		b, g := normKey(biome), normKey(sd.Groups[biome])
		if b == `` || g == `` || b == pocketPrefix || g == pocketPrefix {
			rep.Skipped = append(rep.Skipped, fmt.Sprintf(`%s groups %q: %q is not a usable group`, seedName, biome, sd.Groups[biome]))
			continue
		}
		pool.groups[b] = g
	}
	for _, key := range sortedKeys(sd.Entries) {
		k := normKey(key)
		if prefix, _, ok := parseCorpusKey(k); !ok || !pool.knownPrefix(prefix) {
			rep.Skipped = append(rep.Skipped, fmt.Sprintf(`%s %q: not a biome, group, pocket or tier key`, seedName, key))
			continue
		}
		for i, e := range sd.Entries[key] {
			cleaned, err := checkEntry(e)
			if err != nil {
				rep.Skipped = append(rep.Skipped, fmt.Sprintf(`%s %s #%d %q: %v`, seedName, k, i+1, e.Name, err))
				continue
			}
			pool.seed[k] = append(pool.seed[k], cleaned)
			rep.Seed++
		}
	}

	// The overlay: living state. Absent is empty; unreadable or
	// unparseable is quarantined, never treated as absent.
	var od overlayDoc
	raw, err := util.ReadLivingState(overlayPath)
	if err == nil {
		err = decodeStrict(raw, &od)
	}
	if err != nil && !errors.Is(err, util.ErrStateAbsent) {
		moved, qErr := quarantineOverlay(overlayPath)
		rep.Quarantined = moved
		if qErr != nil {
			// The bad file is still where a save would write, and a save
			// would replace entries this pool never saw. Refuse every
			// overlay write until a reload can read it or move it aside.
			pool.overlayBroken = true
			rep.OverlayBroken = true
		}
		mudlog.Error(`baubles.LoadCorpus`, `action`, `quarantined corrupt overlay`, `error`, err, `quarantinedTo`, moved, `quarantineError`, qErr, `overlayBroken`, rep.OverlayBroken)
		od = overlayDoc{}
	}
	for _, key := range sortedKeys(od.Entries) {
		k := normKey(key)
		prefix, _, keyOk := parseCorpusKey(k)
		for i, e := range od.Entries[key] {
			slot := promotedSlot{raw: e}
			if !keyOk || !pool.knownPrefix(prefix) {
				slot.why = `not a biome, group, pocket or tier key`
			} else if cleaned, err := checkEntry(e.CorpusEntry); err != nil {
				slot.why = err.Error()
			} else {
				slot.use, slot.ok = cleaned, true
				rep.Promoted++
			}
			if !slot.ok {
				rep.Skipped = append(rep.Skipped, fmt.Sprintf(`%s %s #%d %q: %s`, overlayFileName, k, i+1, e.Name, slot.why))
			}
			pool.promoted[k] = append(pool.promoted[k], slot)
		}
	}
	return pool, rep
}

// GroupOf is the corpus group of a biome, if it has one.
func GroupOf(biome string) (string, bool) {
	p := corpus.Load()
	if p == nil {
		return ``, false
	}
	g, ok := p.groups[normKey(biome)]
	return g, ok
}

// CorpusCounts is how many seed and promoted entries are in use.
func CorpusCounts() (seed int, promoted int) {
	p := corpus.Load()
	if p == nil {
		return 0, 0
	}
	for _, list := range p.seed {
		seed += len(list)
	}
	for _, slots := range p.promoted {
		for _, s := range slots {
			if s.ok {
				promoted++
			}
		}
	}
	return seed, promoted
}
```

- [ ] **Step 4: Run to see them pass.**

```bash
cd /c/tmp/dogmud-baubles-c && go test ./internal/baubles/ -run 'TestLoadCorpus|TestMalformedSeed|TestCorruptOverlay|TestNoCorpusFiles|TestReloadWithABrokenSeed|TestAnOverlayThatCannotBeQuarantined|TestParseCorpusKey|TestCheckEntry|TestDecodeStrict' -v 2>&1 | tail -14
```

Expected: all PASS.

- [ ] **Step 5: Null probes (three).** One at a time, restoring after each: (a) replace `if err != nil && !errors.Is(err, util.ErrStateAbsent) {` with `if err != nil && errors.Is(err, util.ErrStateCorrupt) {` (the decode failure is then treated as absent); `TestCorruptOverlayIsQuarantined` must fail with "report:". (b) Change `prev.seedPath == seedPath` to `false`; `TestReloadWithABrokenSeedKeepsTheSeedInUse` must fail with "report:". (c) Delete `pool.overlayBroken = true` and `rep.OverlayBroken = true`; `TestAnOverlayThatCannotBeQuarantinedIsBroken` must fail with "report:". Confirm green.

- [ ] **Step 6: Commit.**

```bash
cd /c/tmp/dogmud-baubles-c && git add internal/baubles/corpus.go internal/baubles/corpus_test.go
git commit -F - <<'EOF'
feat(baubles): load the corpus seed and the promoted overlay

The seed is authored content: a broken file is logged at ERROR and the
corpus runs without it, or, on a reload, keeps the seed already in use.
The overlay is living state: read through ReadLivingState, quarantined
when corrupt (marked broken, and never written, when it cannot be moved
aside), and an entry that fails its checks is kept for the next save
rather than dropped.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 4: Lookup, Fallback and recent names

**Files:**
- Modify: `internal/baubles/corpus.go`
- Modify: `internal/baubles/generate.go` (add `RecentFallbackNames` after `RecentNames`)
- Test: `internal/baubles/corpus_test.go`

- [ ] **Step 1: Write the failing tests.** In `internal/baubles/corpus_test.go` add `"github.com/GoMudEngine/GoMud/internal/configs"` to the imports and append:

```go
// Search finds merge the overlay and the seed at biome-tier and
// group-tier into one pool, and fall to the bare tier only when that pool
// is empty. An empty or unmapped biome skips both.
func TestFallbackMergesBiomeAndGroupThenTier(t *testing.T) {
	withCorpus(t, testSeed, testSpoolOverlay)

	seen := map[string]bool{}
	for i := 0; i < 3; i++ {
		drewFrom := 0
		res := Fallback(Place{Biome: `interior`}, TierCheap, SourceSearch, nil, func(n int) int { drewFrom = n; return i % n })
		if drewFrom != 3 {
			t.Fatalf("interior-cheap (overlay and seed) and dwelling-cheap are one pool of three, drew from %d", drewFrom)
		}
		if res.Generator != GeneratorCorpus || res.Moderated || res.PlayerKey {
			t.Fatalf("corpus result: %+v", res)
		}
		seen[res.Reply.Name] = true
	}
	for _, name := range []string{`Painted Wooden Spool`, `Bent Tin Thimble`, `Chipped Clay Marble`} {
		if !seen[name] {
			t.Errorf("%s is in the merged pool", name)
		}
	}

	if res := Fallback(Place{Biome: `fort`}, TierCheap, SourceSearch, nil, first); res.Reply.Name != `Chipped Clay Marble` || res.Model != `corpus:dwelling-cheap` {
		t.Fatalf("fort has no pool of its own: its group's, got %+v", res)
	}
	for _, biome := range []string{`ruins`, ``, `water`} {
		if res := Fallback(Place{Biome: biome}, TierCheap, SourceSearch, nil, first); res.Reply.Name != `Knotted Twine Bracelet` || res.Model != `corpus:cheap` {
			t.Fatalf("biome %q: nothing at biome or group, so the bare tier, got %+v", biome, res)
		}
	}
	// ruins is both a biome and a group: its pool is counted once.
	drewFrom := 0
	res := Fallback(Place{Biome: `ruins`}, TierAverage, SourceSearch, nil, func(n int) int { drewFrom = n; return 0 })
	if res.Reply.Name != `Faded Mosaic Tile` || drewFrom != 0 {
		t.Fatalf("one entry, drawn without a roll: %+v (drew from %d)", res, drewFrom)
	}
}

// Values are stored as written and clamped into the tier when used.
func TestFallbackClampsTheValueIntoTheTier(t *testing.T) {
	withCorpus(t, testSeed, ``)
	if res := Fallback(Place{}, TierCheap, SourceSearch, nil, first); res.Reply.Value != TierCheap.Range().Max {
		t.Fatalf("99 gold in the cheap pool is clamped to the cheap maximum, got %d", res.Reply.Value)
	}
}

// Pickpocketed finds use pocket-tier then tier, and only what fits a
// pocket (TooBigFor). Nothing that fits: a generic trinket.
func TestFallbackPocketFindsFitAPocket(t *testing.T) {
	withCorpus(t, testSeed, ``)
	for i := 0; i < 4; i++ {
		res := Fallback(Place{Biome: `interior`}, TierCheap, SourcePickpocket, nil, func(n int) int { return i % n })
		if res.Reply.Name != `Brass Snuff Spoon` || res.Model != `corpus:pocket-cheap` {
			t.Fatalf("the tankard is too heavy for a pocket; only the spoon: %+v", res)
		}
	}
	if res := Fallback(Place{}, TierAverage, SourcePickpocket, nil, first); res.Generator != GeneratorLocal || res.Reply.Name != `Trinket` {
		t.Fatalf("no pocket-average and no average pool: a generic trinket, got %+v", res)
	}
	setBaubleConfig(t, func(b *configs.Balance) { b.BaublePickpocketMaxWeight = 0.05 })
	if res := Fallback(Place{}, TierCheap, SourcePickpocket, nil, first); res.Generator != GeneratorLocal || res.Reply.Name != `Trinket` {
		t.Fatalf("every entry is over the pocket limit, the bare tier's too: a generic trinket, got %+v", res)
	}
}

// Entries a zone found lately are avoided. When every entry of the pool was
// found lately, the least recent is taken; recency never widens the key.
func TestFallbackAvoidsRecentNamesWithoutFallingThrough(t *testing.T) {
	withCorpus(t, testSeed, ``)
	for i := 0; i < 4; i++ {
		res := Fallback(Place{Biome: `interior`}, TierCheap, SourceSearch, []string{`Bent Tin Thimble`}, func(n int) int { return i % n })
		if res.Reply.Name != `Chipped Clay Marble` {
			t.Fatalf("the only entry not found lately, got %q", res.Reply.Name)
		}
	}
	recent := []string{`Bent Tin Thimble`, `Chipped Clay Marble`} // newest first
	if res := Fallback(Place{Biome: `interior`}, TierCheap, SourceSearch, recent, first); res.Reply.Name != `Chipped Clay Marble` {
		t.Fatalf("all recent: the least recent, never the bare tier's bracelet, got %q", res.Reply.Name)
	}
	recent = []string{`chipped clay marble`, `Bent Tin Thimble`}
	if res := Fallback(Place{Biome: `interior`}, TierCheap, SourceSearch, recent, first); res.Reply.Name != `Bent Tin Thimble` {
		t.Fatalf("names match without case: the thimble is now least recent, got %q", res.Reply.Name)
	}
}

func TestFallbackWithNoCorpusIsAGenericTrinket(t *testing.T) {
	ClearCorpusForTest()
	res := Fallback(Place{Biome: `interior`}, TierAverage, SourceSearch, nil, nil)
	if res.Generator != GeneratorLocal || res.Reply.Name != `Trinket` || res.Reply.Value != TierAverage.RollValue(nil) {
		t.Fatalf("empty corpus: exactly the old generic trinket, got %+v", res)
	}
}

// The names to avoid are the zone's newest model and corpus finds; a
// generic trinket is not a name.
func TestRecentFallbackNamesCountsModelAndCorpusFinds(t *testing.T) {
	withCatalog(t)
	_, _ = Create(Record{Name: `Old Cup`, Zone: `ashwick`, Generator: GeneratorOpenAI})
	_, _ = Create(Record{Name: `Trinket`, Zone: `ashwick`, Generator: GeneratorLocal})
	_, _ = Create(Record{Name: `Bent Tin Thimble`, Zone: `ashwick`, Generator: GeneratorCorpus})
	_, _ = Create(Record{Name: `Elsewhere`, Zone: `thornwall`, Generator: GeneratorCorpus})
	got := RecentFallbackNames(`ashwick`, 5)
	if len(got) != 2 || got[0] != `Bent Tin Thimble` || got[1] != `Old Cup` {
		t.Fatalf("newest first, model and corpus finds in the zone only: %v", got)
	}
}
```

- [ ] **Step 2: Run to see them fail.**

```bash
cd /c/tmp/dogmud-baubles-c && go test ./internal/baubles/ -run 'TestFallback|TestRecentFallbackNames' 2>&1 | tail -5
```

Expected: build failure, `undefined: Fallback`.

- [ ] **Step 3: Implement.** Append to `internal/baubles/corpus.go`:

```go
type corpusCandidate struct {
	key   string
	entry CorpusEntry
}

// candidates is the pool for one find. A search, admin or burglary find
// merges <biome>-<tier> and <group>-<tier> (overlay and seed alike); an
// empty or unmapped biome skips both. A pickpocketed find uses
// pocket-<tier>. Only when that pool is empty after filtering does it fall
// to the bare <tier>. Every entry must pass TooBigFor for the source (a
// no-op for anything but a pickpocket).
func (p *corpusPool) candidates(biome string, tier ValueTier, source Source) []corpusCandidate {
	var keys []string
	if source == SourcePickpocket {
		keys = []string{corpusKey(pocketPrefix, tier)}
	} else if b := normKey(biome); b != `` {
		if g, ok := p.groups[b]; ok {
			keys = append(keys, corpusKey(b, tier))
			if g != b {
				keys = append(keys, corpusKey(g, tier))
			}
		}
	}
	if out := p.collect(keys, source); len(out) > 0 {
		return out
	}
	return p.collect([]string{corpusKey(``, tier)}, source)
}

func (p *corpusPool) collect(keys []string, source Source) []corpusCandidate {
	var out []corpusCandidate
	add := func(key string, e CorpusEntry) {
		if !TooBigFor(e.reply(), source) {
			out = append(out, corpusCandidate{key: key, entry: e})
		}
	}
	for _, k := range keys {
		for _, s := range p.promoted[k] {
			if s.ok {
				add(k, s.use)
			}
		}
		for _, e := range p.seed[k] {
			add(k, e)
		}
	}
	return out
}

// pickIndex draws an index in [0, n): a nil randn, or an answer out of
// range, gives 0, which tests rely on.
func pickIndex(n int, randn func(n int) int) int {
	if randn == nil || n <= 1 {
		return 0
	}
	if v := randn(n); v >= 0 && v < n {
		return v
	}
	return 0
}

// pickCandidate prefers an entry whose name is not in recent (newest
// first), at random. When every entry is recent it takes the one whose
// latest find is oldest. It never looks beyond cands.
func pickCandidate(cands []corpusCandidate, recent []string, randn func(n int) int) corpusCandidate {
	lastSeen := map[string]int{}
	for i, n := range recent {
		k := normKey(n)
		if _, seen := lastSeen[k]; !seen {
			lastSeen[k] = i
		}
	}
	var fresh []corpusCandidate
	for _, c := range cands {
		if _, seen := lastSeen[normKey(c.entry.Name)]; !seen {
			fresh = append(fresh, c)
		}
	}
	if len(fresh) > 0 {
		return fresh[pickIndex(len(fresh), randn)]
	}
	best := cands[0]
	for _, c := range cands[1:] {
		if lastSeen[normKey(c.entry.Name)] > lastSeen[normKey(best.entry.Name)] {
			best = c
		}
	}
	return best
}

// Fallback is a find no model named: an entry from the corpus for where it
// was found, its value clamped into the tier and its weight limited for
// the source, or a generic trinket when the corpus has nothing that fits.
// It never blocks and reads only the snapshot, so it is safe off the mud
// lock. randn(n) returns [0, n); nil picks the first entry (tests).
func Fallback(place Place, tier ValueTier, source Source, recent []string, randn func(n int) int) GenResult {
	if !tier.Valid() {
		tier = TierCheap
	}
	if p := corpus.Load(); p != nil {
		if cands := p.candidates(place.Biome, tier, source); len(cands) > 0 {
			c := pickCandidate(cands, recent, randn)
			limited := ApplyLimitsFor(c.entry.reply(), tier, source)
			return GenResult{Reply: limited.Reply, Generator: GeneratorCorpus, Model: `corpus:` + c.key}
		}
	}
	return GenResult{Reply: GenericTrinket(tier, randn), Generator: GeneratorLocal}
}

// FallbackFor is Fallback for a naming request, avoiding the names of the
// zone's recent finds.
func FallbackFor(req GenRequest, randn func(n int) int) GenResult {
	return Fallback(req.Place, req.Tier, req.Source, RecentFallbackNames(req.Place.Zone, fallbackRecentNames), randn)
}
```

In `internal/baubles/generate.go`, directly after `RecentNames`, add:

```go
// RecentFallbackNames returns up to n names of named finds in the zone
// (model or corpus), newest first: what a corpus fallback avoids
// repeating. A promoted entry shares its model record's name, so both
// count.
func RecentFallbackNames(zone string, n int) []string {
	cat.mu.RLock()
	defer cat.mu.RUnlock()
	recs := make([]*Record, 0, 32)
	for _, r := range cat.records {
		if r.Zone == zone && r.Generator.Named() {
			recs = append(recs, r)
		}
	}
	sort.Slice(recs, func(a, b int) bool { return recs[a].Id > recs[b].Id })
	out := []string{}
	for _, r := range recs {
		if len(out) >= n {
			break
		}
		out = append(out, r.Name)
	}
	return out
}
```

- [ ] **Step 4: Run to see them pass, then the package.**

```bash
cd /c/tmp/dogmud-baubles-c && go test ./internal/baubles/ -run 'TestFallback|TestRecentFallbackNames' -v 2>&1 | tail -12
cd /c/tmp/dogmud-baubles-c && go test ./internal/baubles/ 2>&1 | tail -3
```

Expected: PASS, then `ok`.

- [ ] **Step 5: Null probes (two).** (a) In `candidates`, change `if out := p.collect(keys, source); len(out) > 0 {` to `if out := p.collect(append(keys, corpusKey(``, tier)), source); len(out) > 0 {`; `TestFallbackMergesBiomeAndGroupThenTier` must fail with "one pool of three". Restore. (b) In `pickCandidate`, change `>` to `<` in the least-recent loop; `TestFallbackAvoidsRecentNamesWithoutFallingThrough` must fail with "the least recent". Restore and confirm green.

- [ ] **Step 6: Commit.**

```bash
cd /c/tmp/dogmud-baubles-c && git add internal/baubles/corpus.go internal/baubles/generate.go internal/baubles/corpus_test.go
git commit -F - <<'EOF'
feat(baubles): Fallback draws from the corpus by biome, group, pocket and tier

One pool merges the overlay and the seed at biome-tier and group-tier;
the bare tier only when that is empty. Pickpocket finds are filtered by
TooBigFor. Recent names are avoided without ever widening the key, and
values are clamped into the tier at use.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 5: The engine's two call sites (Generate, Mint)

**Files:**
- Modify: `internal/baubles/generate.go` (the `generic` closure in `Generate`)
- Modify: `internal/baubles/mint.go` (the `res :=` line)
- Test: `internal/baubles/corpus_test.go`

- [ ] **Step 1: Write the failing tests.** In `internal/baubles/corpus_test.go` add `"context"` to the imports (`"errors"` is already there from Task 3) and append:

```go
// Every way Generate gives up (no generator, an error, a pocket-sized
// refusal) now goes to the corpus.
func TestGenerateFallsBackToTheCorpus(t *testing.T) {
	withCatalog(t)
	withCorpus(t, testSeed, ``)
	SetGenerator(nil, nil)
	req := GenRequest{Tier: TierCheap, Source: SourceSearch, Place: Place{Zone: `ashwick`, Biome: `fort`}}
	if res := Generate(context.Background(), req, nil); res.Generator != GeneratorCorpus || res.Reply.Name != `Chipped Clay Marble` {
		t.Fatalf("no generator: the corpus, got %+v", res)
	}

	installGenerator(t, func(ctx context.Context, req GenRequest) (GenResult, error) {
		return GenResult{}, errors.New(`boom`)
	})
	if res := Generate(context.Background(), req, nil); res.Generator != GeneratorCorpus {
		t.Fatalf("a failed call: the corpus, got %+v", res)
	}

	installGenerator(t, func(ctx context.Context, req GenRequest) (GenResult, error) {
		r := goodReply()
		r.Name, r.NameSimple = `Bronze Funeral Urn`, `urn`
		return GenResult{Reply: r}, nil
	})
	pocket := GenRequest{Tier: TierCheap, Source: SourcePickpocket, Place: Place{Zone: `ashwick`, Biome: `fort`}}
	if res := Generate(context.Background(), pocket, nil); res.Generator != GeneratorCorpus || res.Reply.Name != `Brass Snuff Spoon` {
		t.Fatalf("an urn is too big for a pocket: a pocket entry, got %+v", res)
	}
}

// Mint with no result (tests only) draws from the corpus too, and the
// record says so.
func TestMintWithNoResultDrawsFromTheCorpus(t *testing.T) {
	withCatalog(t)
	withCorpus(t, testSeed, ``)
	_, rec, err := Mint(MintOpts{Source: SourceSearch, Place: Place{RoomId: 1, Zone: `ashwick`, Biome: `fort`}, Tier: TierCheap, Randn: first})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Generator != GeneratorCorpus || rec.Status != StatusReady || rec.Name != `Chipped Clay Marble` || rec.Model != `corpus:dwelling-cheap` {
		t.Fatalf("record: %+v", rec)
	}
}
```

- [ ] **Step 2: Run to see them fail.**

```bash
cd /c/tmp/dogmud-baubles-c && go test ./internal/baubles/ -run 'TestGenerateFallsBackToTheCorpus|TestMintWithNoResultDrawsFromTheCorpus' -v 2>&1 | tail -8
```

Expected: FAIL with "no generator: the corpus, got {... Generator:local ...}" and "record: {... Name:Trinket ...}".

- [ ] **Step 3: Implement.** In `internal/baubles/generate.go`, inside `Generate`, replace

```go
	generic := func() GenResult {
		return GenResult{Reply: GenericTrinket(tier, randn), Generator: GeneratorLocal}
	}
```

with

```go
	// A find the model does not name comes from the fallback corpus, or is
	// a generic trinket when the corpus has nothing that fits.
	generic := func() GenResult {
		r := req
		r.Tier = tier
		return FallbackFor(r, randn)
	}
```

and replace every `` `result`, `generic trinket` `` in the file's `mudlog.Warn` calls with `` `result`, `fallback` `` (four occurrences at `b5ac8b0fa`: three in `Generate` and one in `noteRefusal`, H's rate-limited log for a ledger refusal, whose find also returns through `generic()`; use Edit with replace_all on that exact text). In `Generate`'s `TooBigFor` branch also replace the comment line `// clamped to. A generic (small) trinket instead.` with `// clamped to. A fallback instead (the corpus's pocket pool, or a small trinket).`.

In `internal/baubles/mint.go` replace

```go
	res := GenResult{Reply: GenericTrinket(tier, randn), Generator: GeneratorLocal}
	if o.Result != nil {
		res = *o.Result
	}
```

with

```go
	var res GenResult
	if o.Result != nil {
		res = *o.Result
	} else {
		res = Fallback(o.Place, tier, source, RecentFallbackNames(o.Place.Zone, fallbackRecentNames), randn)
	}
```

In the same file, three comments still say every fallback is a generic trinket. In `MintOpts` replace

```go
	// Result is the finished text from Generate. nil makes a generic
	// trinket here. Either way its numbers are clamped to Tier.
```

with

```go
	// Result is the finished text from Generate. nil draws from the
	// fallback corpus here (a generic trinket when it has nothing that
	// fits). Either way its numbers are clamped to Tier.
```

and

```go
	// Randn picks a generic trinket's value and weight: randn(n) returns
	// [0, n). nil means util.Rand. Tests pass a fixed function.
```

with

```go
	// Randn picks the corpus entry, or a generic trinket's value and
	// weight: randn(n) returns [0, n). nil means util.Rand. Tests pass a
	// fixed function.
```

and replace `Mint`'s doc comment

```go
// Mint creates one new bauble: a catalog record with FINAL text (from
// o.Result, or a generic trinket), and the carrier item pointing at it. The
// record is on disk before Mint returns. Naming happens before this, off the
// mud lock (Generate); Mint itself never waits on anything.
```

with

```go
// Mint creates one new bauble: a catalog record with FINAL text (from
// o.Result, or else from the fallback corpus), and the carrier item pointing
// at it. The record is on disk before Mint returns. Naming happens before
// this, off the mud lock (Generate); Mint itself never waits on anything.
```

In `internal/baubles/generate.go`, three comments likewise. Replace `// With nothing installed, every bauble is a generic trinket (fallback.go).` with the two lines

```go
// With nothing installed, every bauble comes from the fallback corpus
// (corpus.go), or is a generic trinket (fallback.go) when nothing fits.
```

replace `// which every bauble is a generic trinket. info may be nil.` (in `SetGenerator`'s doc comment) with `// which every bauble comes from the fallback corpus. info may be nil.`, and replace `Generate`'s doc comment

```go
// Generate names one find. It returns the model's answer when a generator
// is installed and its answer is usable (validated and clamped), and a
// generic trinket otherwise. It never fails. randn picks a generic
// trinket's value and weight: pass util.Rand in production (nil gives the
// deterministic midpoint, for tests).
```

with

```go
// Generate names one find. It returns the model's answer when a generator
// is installed and its answer is usable (validated and clamped), and a
// fallback otherwise (FallbackFor: the corpus, else a generic trinket). It
// never fails. randn picks the fallback: pass util.Rand in production (nil
// gives the first entry, or the deterministic midpoint, for tests).
```

- [ ] **Step 4: Run to see them pass, then the package (existing no-corpus tests must stay green: an empty corpus is exactly the old generic trinket).**

```bash
cd /c/tmp/dogmud-baubles-c && go test ./internal/baubles/ -run 'TestGenerateFallsBackToTheCorpus|TestMintWithNoResultDrawsFromTheCorpus' -v 2>&1 | tail -6
cd /c/tmp/dogmud-baubles-c && go test ./internal/baubles/ 2>&1 | tail -3
```

Expected: PASS, then `ok`.

- [ ] **Step 5: Commit.**

```bash
cd /c/tmp/dogmud-baubles-c && git add internal/baubles/generate.go internal/baubles/mint.go internal/baubles/corpus_test.go
git commit -F - <<'EOF'
feat(baubles): Generate and Mint fall back to the corpus

Their comments now say so too.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 6: The two actions call sites, and the pickpocket log

**Files:**
- Modify: `internal/actions/search_bauble.go` (`FlushBaubleDeliveries`)
- Modify: `internal/actions/steal_pocket.go` (`(*pocketAttempt).naming`, the `pickpocket` log in `resolve`)
- Test: `internal/actions/search_bauble_test.go`, `internal/actions/pickpocket_test.go`

- [ ] **Step 1: Write the failing tests.** In `internal/actions/search_bauble_test.go` add `"os"` and `"path/filepath"` to the imports and append:

```go
// loadTestCorpus gives the baubles package a small fallback corpus for one
// test: bare cheap and average pools (the test rooms have no biome) and a
// pocket pool.
func loadTestCorpus(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	seed := filepath.Join(dir, "bauble-corpus.yaml")
	text := `entries:
  cheap:
    - name: Bent Tin Thimble
      name_simple: thimble
      description: A tin thimble, pressed a little out of shape by a careless heel.
      material: tin
      weight_lbs: 0.1
      value: 3
  average:
    - name: Painted Wooden Spool
      name_simple: spool
      description: A wooden thread spool painted with a band of faded blue.
      material: wood
      weight_lbs: 0.2
      value: 12
  pocket-cheap:
    - name: Brass Snuff Spoon
      name_simple: spoon
      description: A tiny brass spoon for snuff, its bowl no bigger than a fingernail.
      material: brass
      weight_lbs: 0.1
      value: 2
`
	if err := os.WriteFile(seed, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	rep := baubles.LoadCorpusFrom(seed, filepath.Join(dir, "corpus.promoted.yaml"))
	t.Cleanup(baubles.ClearCorpusForTest)
	if rep.Seed != 3 {
		t.Fatalf("test corpus: %+v", rep)
	}
}

func TestSearch_Bauble_NoKeyDrawsFromTheCorpus(t *testing.T) {
	pinConfigForTest(t)
	actor := newSearchFakeActor("Finder", newSearchTestRoom(9540), true, 7160)
	canCarry(actor)
	stubBaubleSearch(t, true, actor) // every roll finds, tier average
	loadTestCorpus(t)

	Search(actor, SearchOptions{})

	itm, has := actor.char.FindInBackpack("spool")
	if !has || !itm.IsBauble() {
		t.Fatalf("the corpus entry is in the pack: %q", actor.sent)
	}
	rec, _ := baubles.Get(itm.Bauble)
	if rec.Name != "Painted Wooden Spool" || rec.Generator != baubles.GeneratorCorpus || rec.Status != baubles.StatusReady || rec.Model != "corpus:average" {
		t.Fatalf("record: %+v", rec)
	}
}

// A find still being named at a flush is finished from the corpus.
func TestFlush_UnnamedFindDrawsFromTheCorpus(t *testing.T) {
	pinConfigForTest(t)
	actor := newSearchFakeActor("Hasty", newSearchTestRoom(9541), true, 7161)
	canCarry(actor)
	stubBaubleSearch(t, true, actor)
	loadTestCorpus(t)
	naming := make(chan struct{})
	baubles.SetGenerator(func(ctx context.Context, req baubles.GenRequest) (baubles.GenResult, error) {
		close(naming)
		<-ctx.Done()
		return baubles.GenResult{}, ctx.Err()
	}, nil)

	d := BaubleDelivery{Request: BaubleRequest(actor.room, baubles.TierCheap, baubles.SourceSearch, ""), UserId: 7161}
	done := make(chan struct{})
	go func() { d.run(false); close(done) }()
	<-naming

	if n := FlushBaubleDeliveries(); n != 1 {
		t.Fatalf("one find finished by the flush, got %d", n)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the delivery goroutine stands down once flushed")
	}
	if _, has := actor.char.FindInBackpack("thimble"); !has {
		t.Fatal("unnamed at the flush: drawn from the corpus")
	}
}
```

Append to `internal/actions/pickpocket_test.go`:

```go
// A pocket bauble whose naming never came back is drawn from the corpus's
// pocket pool.
func TestPickpocketUnnamedBaubleDrawsFromTheCorpus(t *testing.T) {
	loadTestCorpus(t)
	p := &pocketAttempt{
		req:   baubles.GenRequest{Tier: baubles.TierCheap, Source: baubles.SourcePickpocket, Place: baubles.Place{Zone: "nowhere"}},
		randn: func(int) int { return 0 },
	}
	res, named := p.naming()
	if named || res.Generator != baubles.GeneratorCorpus || res.Reply.Name != "Brass Snuff Spoon" {
		t.Fatalf("got %+v (named %v)", res, named)
	}
}
```

- [ ] **Step 2: Run to see them fail.**

```bash
cd /c/tmp/dogmud-baubles-c && go test ./internal/actions/ -run 'TestSearch_Bauble_NoKeyDrawsFromTheCorpus|TestFlush_UnnamedFindDrawsFromTheCorpus|TestPickpocketUnnamedBaubleDrawsFromTheCorpus' -v 2>&1 | tail -12
```

Expected: `TestSearch_Bauble_NoKeyDrawsFromTheCorpus` PASSES already (it goes through `Generate`, wired in Task 5). The other two FAIL: the flush finds no `thimble`, and the pocket attempt returns `Generator:local`. That is the proof that these two sites are independent of `Generate`.

- [ ] **Step 3: Implement.** In `internal/actions/search_bauble.go`, in `FlushBaubleDeliveries`, replace

```go
			res = baubles.GenResult{Reply: baubles.GenericTrinket(p.d.Request.Tier, p.randn), Generator: baubles.GeneratorLocal}
```

with

```go
			res = baubles.FallbackFor(p.d.Request, p.randn)
```

and in the function's doc comment replace the text `otherwise as the generic trinket it would have` with `otherwise from the fallback corpus, as it would have` (the next comment line, "been had the model not answered.", stays). In the same file replace `// cannot tell a model-named find from a generic trinket by its timing, and` with `// cannot tell a model-named find from a fallback one by its timing, and`, and in `BaubleDelivery` replace the field comment `// the generic trinket's dice; nil means util.Rand` with `// the fallback's dice; nil means util.Rand` (comment text only; gofmt alignment is unchanged).

In `internal/actions/steal_pocket.go`, in `naming`, replace

```go
	return baubles.GenResult{Reply: baubles.GenericTrinket(p.req.Tier, p.randn), Generator: baubles.GeneratorLocal}, false
```

with

```go
	return baubles.FallbackFor(p.req, p.randn), false
```

and replace its two comment lines (`// naming is the bauble's naming if it came back, else the generic` and `// trinket it would have been.`) with `// naming is the bauble's naming if it came back, else the corpus` and `// fallback it would have been.`. In `FlushPocketAttempts`'s doc comment replace `// whose naming is not back is the generic trinket it would have been).` with `// whose naming is not back is the corpus fallback it would have been).`.

The pickpocket log: at `b5ac8b0fa` it is this line in `resolve` (`steal_pocket.go:342`, Task 0 Step 5), unchanged in form since `e711ee9de`:

```go
			mudlog.Info(`baubles`, `action`, `pickpocket`, `id`, rec.Id, `mob`, p.mobName, `named`, rec.Generator == baubles.GeneratorOpenAI)
```

Wherever it now is, and whatever else it logs, replace only its `` `named`, rec.Generator == baubles.GeneratorOpenAI `` pair with `` `generator`, string(rec.Generator) ``, so at `b5ac8b0fa` it reads:

```go
			mudlog.Info(`baubles`, `action`, `pickpocket`, `id`, rec.Id, `mob`, p.mobName, `generator`, string(rec.Generator))
```

Then confirm no `named` pair is left (expected: no output):

```bash
cd /c/tmp/dogmud-baubles-c && grep -n '`named`' internal/actions/steal_pocket.go
```

- [ ] **Step 4: Run to see them pass, then the package.**

```bash
cd /c/tmp/dogmud-baubles-c && go test ./internal/actions/ -run 'TestSearch_Bauble|TestFlush_|TestPickpocket' -v 2>&1 | grep -E "^(--- |ok|FAIL)" | tail -30
cd /c/tmp/dogmud-baubles-c && go test ./internal/actions/ 2>&1 | tail -3
```

Expected: every listed test PASS, then `ok`.

- [ ] **Step 5: No `GenericTrinket` call is left outside the corpus.**

```bash
cd /c/tmp/dogmud-baubles-c && grep -rn "GenericTrinket(" --include=*.go . | grep -v _test.go
```

Expected: exactly two lines, the definition in `internal/baubles/fallback.go` and the call in `Fallback` in `internal/baubles/corpus.go`.

- [ ] **Step 6: Commit.**

```bash
cd /c/tmp/dogmud-baubles-c && git add internal/actions/search_bauble.go internal/actions/steal_pocket.go internal/actions/search_bauble_test.go internal/actions/pickpocket_test.go
git commit -F - <<'EOF'
feat(actions): flushed and unnamed pocket finds draw from the bauble corpus

The pickpocket log names the generator (openai, corpus or local) instead
of a named yes/no, and the comments that said "generic trinket" name the
fallback.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 7: Promotion, removal, retire, edit, regen and export

**Files:**
- Create: `internal/baubles/corpus_admin.go`
- Modify: `internal/baubles/admin.go` (`Retire`, `Edit`, `ApplyRegenerated`)
- Modify: `internal/actions/bauble_admin.go` (`RegenerateBauble`), `internal/usercommands/admin.bauble.go` (`baubleEdit`, `baubleRetire`): the callers of the two changed signatures
- Modify: `internal/baubles/admin_test.go` and `internal/baubles/generate_test.go` (call sites of `Edit` and `ApplyRegenerated`)
- Test: `internal/baubles/corpus_admin_test.go`

- [ ] **Step 1: Write the failing tests.** Create `internal/baubles/corpus_admin_test.go`:

```go
package baubles

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/util"
)

// promotable is a record Promote accepts: named by the model on the
// server's key, moderated, never hand-edited, found indoors.
func promotable() Record {
	return Record{
		Name: `Painted Wooden Spool`, NameSimple: `spool`,
		Description: `A wooden thread spool painted with a band of faded blue.`,
		Material:    `wood`, Tier: TierCheap, Value: 4, WeightLbs: 0.2, Status: StatusReady,
		Source: SourceSearch, Zone: `ashwick`, Biome: `interior`, Generator: GeneratorOpenAI,
		Model: `gpt-test`, PromptVersion: 3, Moderated: true,
	}
}

func TestPromoteRefusals(t *testing.T) {
	withCatalog(t)
	withCorpus(t, testSeed, ``)
	cases := []struct {
		name   string
		change func(r *Record)
		want   error
	}{
		{`player key`, func(r *Record) { r.PlayerKey = true }, ErrPromotePlayerKey},
		{`unmoderated`, func(r *Record) { r.Moderated = false }, ErrPromoteUnmoderated},
		{`hand-edited`, func(r *Record) { r.HandEdited = true }, ErrPromoteEdited},
		{`retired`, func(r *Record) { r.Status = StatusRetired }, ErrPromoteRetired},
		{`generic`, func(r *Record) { r.Generator = GeneratorLocal }, ErrPromoteNotModel},
		{`from the corpus`, func(r *Record) { r.Generator = GeneratorCorpus }, ErrPromoteNotModel},
		{`unmapped biome`, func(r *Record) { r.Biome = `water` }, ErrPromoteNoGroup},
		{`no biome`, func(r *Record) { r.Biome = `` }, ErrPromoteNoGroup},
		{`too big for a pocket`, func(r *Record) {
			r.Source, r.Name, r.NameSimple = SourcePickpocket, `Bronze Funeral Urn`, `urn`
		}, ErrPromoteTooBig},
		// interior-cheap's own seed entry, and dwelling-cheap's, which
		// Fallback merges into the same pool.
		{`a name in its pool's seed`, func(r *Record) {
			r.Name, r.NameSimple = `Bent Tin Thimble`, `thimble`
		}, ErrPromoteNameTaken},
		{`a name in its group's seed`, func(r *Record) {
			r.Name, r.NameSimple = `chipped clay marble`, `marble`
		}, ErrPromoteNameTaken},
	}
	for _, c := range cases {
		r := promotable()
		c.change(&r)
		rec := seedRecord(t, r)
		if _, err := Promote(rec.Id); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
	if _, promoted := CorpusCounts(); promoted != 0 {
		t.Fatalf("nothing refused may reach the overlay, got %d", promoted)
	}
	if _, err := Promote(`B9999999`); !errors.Is(err, ErrNoRecord) {
		t.Fatalf("no record: %v", err)
	}
	ClearCorpusForTest()
	if _, err := Promote(seedRecord(t, promotable()).Id); !errors.Is(err, ErrNoCorpus) {
		t.Fatalf("no corpus loaded: %v", err)
	}
}

// A sold find is promotable. It is saved with its provenance, used at once,
// read back on reload, and refused a second time.
func TestPromoteASoldFindAndUseIt(t *testing.T) {
	withCatalog(t)
	seedPath, overlayPath, _ := withCorpus(t, testSeed, ``)
	r := promotable()
	r.Status, r.SoldValue = StatusSold, 3
	rec := seedRecord(t, r)

	key, err := Promote(rec.Id)
	if err != nil || key != `interior-cheap` {
		t.Fatalf("promote: %q %v", key, err)
	}
	data, err := os.ReadFile(overlayPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`from_record: ` + rec.Id, `Painted Wooden Spool`, `model: gpt-test`, `biome: interior`, `promoted_at:`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("the overlay holds %q:\n%s", want, data)
		}
	}
	if _, err := Promote(rec.Id); !errors.Is(err, ErrPromoteDuplicate) {
		t.Fatalf("twice: %v", err)
	}
	if _, err := Promote(seedRecord(t, promotable()).Id); !errors.Is(err, ErrPromoteNameTaken) {
		t.Fatalf("another record with a name already promoted to that pool: %v", err)
	}
	recent := []string{`Bent Tin Thimble`, `Chipped Clay Marble`}
	if res := Fallback(Place{Biome: `interior`}, TierCheap, SourceSearch, recent, first); res.Reply.Name != `Painted Wooden Spool` || res.Model != `corpus:interior-cheap` {
		t.Fatalf("the promoted entry is in the pool at once: %+v", res)
	}
	LoadCorpusFrom(seedPath, overlayPath)
	if _, promoted := CorpusCounts(); promoted != 1 {
		t.Fatalf("read back from disk: %d", promoted)
	}

	p := promotable()
	p.Source, p.Name, p.NameSimple, p.WeightLbs = SourcePickpocket, `Carved Walnut Button`, `button`, 0.1
	if key, err := Promote(seedRecord(t, p).Id); err != nil || key != `pocket-cheap` {
		t.Fatalf("a pickpocketed find goes to the pocket pool: %q %v", key, err)
	}
}

// Persist before publish: a promotion whose save fails changes nothing.
func TestPromoteChangesNothingWhenTheSaveFails(t *testing.T) {
	withCatalog(t)
	dir := t.TempDir()
	seedPath := filepath.Join(dir, seedFileName)
	writeTestFile(t, seedPath, testSeed)
	blocker := filepath.Join(dir, `blocker`)
	writeTestFile(t, blocker, `a file where the overlay's directory should be`)
	LoadCorpusFrom(seedPath, filepath.Join(blocker, overlayFileName))
	t.Cleanup(ClearCorpusForTest)

	if _, err := Promote(seedRecord(t, promotable()).Id); err == nil {
		t.Fatal("a save that cannot be written must fail the promotion")
	}
	if _, promoted := CorpusCounts(); promoted != 0 {
		t.Fatal("and nothing is published in memory either")
	}
}

func TestRetireRemovesItsCorpusEntry(t *testing.T) {
	withCatalog(t)
	_, overlayPath, _ := withCorpus(t, testSeed, ``)
	k := promotable()
	k.Name, k.NameSimple = `Carved Walnut Button`, `button`
	keep := seedRecord(t, k)
	gone := seedRecord(t, promotable())
	for _, id := range []string{keep.Id, gone.Id} {
		if _, err := Promote(id); err != nil {
			t.Fatal(err)
		}
	}
	if err := Retire(gone.Id, `Admin`); err != nil {
		t.Fatal(err)
	}
	l := CorpusList(`interior-cheap`)
	if len(l.Promoted) != 1 || l.Promoted[0].FromRecord != keep.Id {
		t.Fatalf("only the retired record's entry goes: %+v", l.Promoted)
	}
	data, _ := os.ReadFile(overlayPath)
	if strings.Contains(string(data), gone.Id) {
		t.Fatal("and it is gone from disk")
	}
}

// A promoted record's text can change after promotion: an edit or a
// regeneration removes what was promoted from it, and says how much.
func TestEditAndRegenRemoveTheRecordsCorpusEntry(t *testing.T) {
	withCatalog(t)
	_, overlayPath, _ := withCorpus(t, testSeed, ``)
	edited := seedRecord(t, promotable())
	p := promotable()
	p.Name, p.NameSimple = `Carved Walnut Button`, `button`
	regen := seedRecord(t, p)
	for _, id := range []string{edited.Id, regen.Id} {
		if _, err := Promote(id); err != nil {
			t.Fatal(err)
		}
	}

	_, removed, err := Edit(edited.Id, `desc`, `A wooden spool with a band of blue paint, most of it worn away.`, `Admin`)
	if err != nil || removed != 1 {
		t.Fatalf("an edit removes its one promoted entry: %d %v", removed, err)
	}
	_, removed, err = ApplyRegenerated(regen.Id, GenResult{Reply: goodReply(), Generator: GeneratorOpenAI, Model: `gpt-test`, Moderated: true}, `Admin`, first)
	if err != nil || removed != 1 {
		t.Fatalf("a regeneration removes its one promoted entry: %d %v", removed, err)
	}
	if l := CorpusList(`interior-cheap`); len(l.Promoted) != 0 {
		t.Fatalf("both gone from memory: %+v", l.Promoted)
	}
	data, _ := os.ReadFile(overlayPath)
	if strings.Contains(string(data), edited.Id) || strings.Contains(string(data), regen.Id) {
		t.Fatalf("and from disk:\n%s", data)
	}
	if _, removed, err := Edit(edited.Id, `value`, `3`, `Admin`); err != nil || removed != 0 {
		t.Fatalf("a record with nothing promoted removes nothing: %d %v", removed, err)
	}
}

// Promotion looks only at whose hand wrote the text: a regenerated record
// (moderated, on the server's key) and a retired then restored one are
// promotable; a hand-edited one is not.
func TestPromoteLooksAtHandEditedNotEditedBy(t *testing.T) {
	withCatalog(t)
	withCorpus(t, testSeed, ``)

	r := promotable()
	r.Moderated, r.HandEdited = false, true
	regen := seedRecord(t, r)
	reply := goodReply()
	reply.Name, reply.NameSimple = `Painted Clay Owl`, `owl`
	if _, _, err := ApplyRegenerated(regen.Id, GenResult{Reply: reply, Generator: GeneratorOpenAI, Model: `gpt-test`, Moderated: true}, `Admin`, first); err != nil {
		t.Fatal(err)
	}
	if _, err := Promote(regen.Id); err != nil {
		t.Fatalf("freshly regenerated, moderated, server-key text is promotable: %v", err)
	}

	p := promotable()
	p.Name, p.NameSimple = `Carved Walnut Button`, `button`
	restored := seedRecord(t, p)
	if err := Retire(restored.Id, `Admin`); err != nil {
		t.Fatal(err)
	}
	if err := Restore(restored.Id, `Admin`); err != nil {
		t.Fatal(err)
	}
	if _, err := Promote(restored.Id); err != nil {
		t.Fatalf("retired then restored, the text is still the model's: %v", err)
	}

	q := promotable()
	q.Name, q.NameSimple = `Tarnished Brass Thimble`, `thimble`
	edited := seedRecord(t, q)
	if _, _, err := Edit(edited.Id, `material`, `copper`, `Admin`); err != nil {
		t.Fatal(err)
	}
	if _, err := Promote(edited.Id); !errors.Is(err, ErrPromoteEdited) {
		t.Fatalf("hand-edited text is never promoted: %v", err)
	}
}

// Removal names the entry (its name, any case, or the record it came from),
// never a position that shifts as entries come and go.
func TestRemoveCorpusEntry(t *testing.T) {
	withCatalog(t)
	_, overlayPath, _ := withCorpus(t, testSeed, ``)
	rec := seedRecord(t, promotable())
	p := promotable()
	p.Name, p.NameSimple = `Carved Walnut Button`, `button`
	other := seedRecord(t, p)
	for _, id := range []string{rec.Id, other.Id} {
		if _, err := Promote(id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := RemoveCorpusEntry(`interior-cheap`, `Silver Spoon`); err == nil {
		t.Fatal("no entry by that name")
	}
	if _, err := RemoveCorpusEntry(`dwelling-cheap`, `Chipped Clay Marble`); err == nil || !strings.Contains(err.Error(), `seed`) {
		t.Fatalf("seed entries are not removable in game: %v", err)
	}
	removed, err := RemoveCorpusEntry(`Interior-Cheap`, `painted wooden SPOOL`)
	if err != nil || removed.FromRecord != rec.Id {
		t.Fatalf("remove by name: %+v %v", removed, err)
	}
	removed, err = RemoveCorpusEntry(`interior-cheap`, strings.ToLower(other.Id))
	if err != nil || removed.Name != `Carved Walnut Button` {
		t.Fatalf("remove by record id: %+v %v", removed, err)
	}
	if _, promoted := CorpusCounts(); promoted != 0 {
		t.Fatal("removed from memory")
	}
	data, _ := os.ReadFile(overlayPath)
	if strings.Contains(string(data), rec.Id) || strings.Contains(string(data), other.Id) {
		t.Fatal("and from disk")
	}
}

// Two overlay entries with one name (only a hand edit of the file can do
// that) are never removed by guesswork.
func TestRemoveCorpusEntryRefusesAnAmbiguousName(t *testing.T) {
	withCatalog(t)
	twice := `entries:
  interior-cheap:
    - name: Painted Wooden Spool
      name_simple: spool
      description: A wooden thread spool painted with a band of faded blue.
      weight_lbs: 0.2
      value: 4
      from_record: B0000007
      promoted_at: 2026-09-28T12:00:00Z
    - name: Painted Wooden Spool
      name_simple: spool
      description: A wooden thread spool painted with a band of faded blue.
      weight_lbs: 0.2
      value: 4
      from_record: B0000008
      promoted_at: 2026-09-28T12:00:00Z
`
	withCorpus(t, testSeed, twice)
	if _, err := RemoveCorpusEntry(`interior-cheap`, `Painted Wooden Spool`); err == nil {
		t.Fatal("two entries share the name: name the record instead")
	}
	if removed, err := RemoveCorpusEntry(`interior-cheap`, `B0000008`); err != nil || removed.FromRecord != `B0000008` {
		t.Fatalf("by record id: %+v %v", removed, err)
	}
}

// An overlay that could not be read or moved aside is never written: a
// save would replace entries the pool never saw. Retire still retires.
func TestABrokenOverlayRefusesEveryWrite(t *testing.T) {
	withCatalog(t)
	quarantineOverlay = func(string) (string, error) { return ``, errors.New(`disk says no`) }
	t.Cleanup(func() { quarantineOverlay = util.QuarantineCorrupt })
	corrupt := "entries: {this is: [not closed\n"
	_, overlayPath, rep := withCorpus(t, testSeed, corrupt)
	if !rep.OverlayBroken {
		t.Fatalf("fixture: %+v", rep)
	}
	rec := seedRecord(t, promotable())
	if _, err := Promote(rec.Id); !errors.Is(err, ErrOverlayBroken) {
		t.Fatalf("promote: %v", err)
	}
	if _, err := RemoveCorpusEntry(`interior-cheap`, `Painted Wooden Spool`); !errors.Is(err, ErrOverlayBroken) {
		t.Fatalf("remove: %v", err)
	}
	err := Retire(rec.Id, `Admin`)
	if !errors.Is(err, ErrCorpusCleanup) || !errors.Is(err, ErrOverlayBroken) {
		t.Fatalf("retire reports the overlay it could not clean: %v", err)
	}
	if got, _ := Get(rec.Id); got.Status != StatusRetired {
		t.Fatal("and retires the record all the same")
	}
	if data, err := os.ReadFile(overlayPath); err != nil || string(data) != corrupt {
		t.Fatalf("the file is untouched: %v", err)
	}
}

// An overlay entry that fails its checks is not used, but a save keeps it.
func TestAnUnusableOverlayEntryIsKeptOnSave(t *testing.T) {
	withCatalog(t)
	_, overlayPath, rep := withCorpus(t, testSeed, `entries:
  interior-cheap:
    - name: Stone Idol Head
      name_simple: idol
      description: A stone head broken from a small idol, far heavier than it looks.
      weight_lbs: 40
      value: 3
      from_record: B0000009
      promoted_at: 2026-09-28T12:00:00Z
`)
	if rep.Promoted != 0 || len(rep.Skipped) != 1 {
		t.Fatalf("report: %+v", rep)
	}
	rec := seedRecord(t, promotable())
	if _, err := Promote(rec.Id); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(overlayPath)
	if !strings.Contains(string(data), `Stone Idol Head`) || !strings.Contains(string(data), rec.Id) {
		t.Fatalf("both entries are saved:\n%s", data)
	}
	l := CorpusList(`interior-cheap`)
	if len(l.Promoted) != 2 || l.Unused[1] == `` {
		t.Fatalf("listed, with why the first is unused: %+v", l)
	}
}

// The overlay lives in the catalog's directory. The catalog loader reads
// only catalog-* files, so reloading, writing, retrying shards and pruning
// the catalog never touch it. (Step 5 replaces the PRUNE line with a call to
// FinalTwist's catalog prune; the gate fails while the line is there.)
func TestOverlaySurvivesTheCatalog(t *testing.T) {
	dir := withCatalog(t)
	seedPath := filepath.Join(t.TempDir(), seedFileName)
	writeTestFile(t, seedPath, testSeed)
	overlayPath := filepath.Join(dir, overlayFileName)
	LoadCorpusFrom(seedPath, overlayPath)
	t.Cleanup(ClearCorpusForTest)
	if _, err := Promote(seedRecord(t, promotable()).Id); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(overlayPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := loadFrom(dir); err != nil {
		t.Fatal(err)
	}
	if Count() != 1 {
		t.Fatalf("the catalog read its one record and nothing else, got %d", Count())
	}
	_, _ = Create(Record{Name: `Another Find`, Generator: GeneratorLocal})
	SaveAll()
	// PRUNE: call the catalog prune here (applySweep, Step 5).

	after, err := os.ReadFile(overlayPath)
	if err != nil || string(after) != string(before) {
		t.Fatalf("the overlay is untouched: %v", err)
	}
	if aside, _ := filepath.Glob(filepath.Join(dir, overlayFileName+`.corrupt-*`)); len(aside) != 0 {
		t.Fatalf("never quarantined as a shard: %v", aside)
	}
	LoadCorpusFrom(seedPath, overlayPath)
	if _, promoted := CorpusCounts(); promoted != 1 {
		t.Fatal("and still loads")
	}
}

// Export prints the promoted entries in the seed's format (no provenance),
// ready to paste into bauble-corpus.yaml.
func TestExportPromotedIsSeedFormat(t *testing.T) {
	withCatalog(t)
	withCorpus(t, testSeed, ``)
	if _, err := Promote(seedRecord(t, promotable()).Id); err != nil {
		t.Fatal(err)
	}
	out, err := ExportPromoted()
	if err != nil {
		t.Fatal(err)
	}
	var doc seedDoc
	if err := decodeStrict([]byte(out), &doc); err != nil {
		t.Fatalf("parses strictly as a seed: %v\n%s", err, out)
	}
	if l := doc.Entries[`interior-cheap`]; len(l) != 1 || l[0].Name != `Painted Wooden Spool` {
		t.Fatalf("export: %+v", doc)
	}
}
```

- [ ] **Step 2: Run to see them fail.**

```bash
cd /c/tmp/dogmud-baubles-c && go test ./internal/baubles/ -run 'TestPromote|TestRetireRemoves|TestEditAndRegenRemove|TestRemoveCorpusEntry|TestABrokenOverlay|TestAnUnusableOverlay|TestOverlaySurvives|TestExportPromoted' 2>&1 | tail -5
```

Expected: build failure, `undefined: Promote`.

- [ ] **Step 3: Implement.** Create `internal/baubles/corpus_admin.go`:

```go
package baubles

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/GoMudEngine/GoMud/internal/util"
	"gopkg.in/yaml.v3"
)

// Corpus administration: promoting a model's name into the overlay,
// removing one, and what Retire, Edit and ApplyRegenerated need. The admin
// `bauble promote` and `bauble corpus` commands call these, and so will the
// /build queue (web builder rework arc). Callers hold the mud lock;
// corpusWriteMu serialises them as well.

var (
	ErrNoCorpus           = errors.New(`the fallback corpus is not loaded`)
	ErrOverlayBroken      = errors.New(`the promoted file could not be read or set aside, so nothing is written to it until "bauble corpus reload" succeeds`)
	ErrCorpusCleanup      = errors.New(`the record was changed, but the fallback corpus entries promoted from it could not be removed`)
	ErrPromoteNotModel    = errors.New(`only a find named by the model can be promoted`)
	ErrPromotePlayerKey   = errors.New(`it was named on a player's own key`)
	ErrPromoteUnmoderated = errors.New(`its text was never passed by moderation`)
	ErrPromoteEdited      = errors.New(`an admin has changed its text by hand`)
	ErrPromoteRetired     = errors.New(`it is retired`)
	ErrPromoteNoGroup     = errors.New(`it was found in a biome with no corpus group`)
	ErrPromoteTooBig      = errors.New(`it is too big for a pocket`)
	ErrPromoteDuplicate   = errors.New(`it is already in the corpus`)
	ErrPromoteNameTaken   = errors.New(`its pool already has a find by that name`)
)

// promotionKey is the overlay key a record goes under: pocket-<tier> for a
// pickpocketed find, else <biome>-<tier>, and only for a biome with a group
// (lookup skips a biome without one).
func (p *corpusPool) promotionKey(rec Record) (string, error) {
	if rec.Source == SourcePickpocket {
		return corpusKey(pocketPrefix, rec.Tier), nil
	}
	b := normKey(rec.Biome)
	if _, ok := p.groups[b]; b == `` || !ok {
		return ``, ErrPromoteNoGroup
	}
	return corpusKey(b, rec.Tier), nil
}

// mergedKeys is key and every key Fallback merges with it into one pool: a
// biome's key and its group's. A pocket or group key stands alone.
func (p *corpusPool) mergedKeys(key string) []string {
	keys := []string{key}
	if prefix, tier, ok := parseCorpusKey(key); ok {
		if g, ok := p.groups[prefix]; ok && g != prefix {
			keys = append(keys, corpusKey(g, tier))
		}
	}
	return keys
}

// nameInPool reports a seed or overlay entry (used or not) called name in
// the pool key belongs to.
func (p *corpusPool) nameInPool(key, name string) bool {
	n := normKey(name)
	for _, k := range p.mergedKeys(key) {
		for _, e := range p.seed[k] {
			if normKey(e.Name) == n {
				return true
			}
		}
		for _, s := range p.promoted[k] {
			if normKey(s.raw.Name) == n {
				return true
			}
		}
	}
	return false
}

// withPromoted is a copy of the pool with change applied to a copy of its
// overlay. The pool itself is never changed: readers may hold it.
func (p *corpusPool) withPromoted(change func(m map[string][]promotedSlot)) *corpusPool {
	next := *p
	next.promoted = make(map[string][]promotedSlot, len(p.promoted))
	for k, v := range p.promoted {
		next.promoted[k] = append([]promotedSlot(nil), v...)
	}
	change(next.promoted)
	for k, v := range next.promoted {
		if len(v) == 0 {
			delete(next.promoted, k)
		}
	}
	return &next
}

// saveOverlay writes every overlay entry, used or not, through util.Save.
func saveOverlay(path string, promoted map[string][]promotedSlot) error {
	doc := overlayDoc{Entries: map[string][]PromotedEntry{}}
	for k, slots := range promoted {
		for _, s := range slots {
			doc.Entries[k] = append(doc.Entries[k], s.raw)
		}
	}
	data, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return util.Save(path, data)
}

// Promote copies a record's text into the overlay, under its exact
// <biome>-<tier> or pocket-<tier> key, with its provenance. Only text the
// model wrote on the server's key and moderation passed, never hand-edited
// (HandEdited; EditedBy alone is fine, since Retire, Restore and regen set
// it without writing text) and not retired (a sold record is fine), and
// only a name its pool does not already have. It returns the key.
func Promote(id string) (string, error) {
	corpusWriteMu.Lock()
	defer corpusWriteMu.Unlock()
	p := corpus.Load()
	if p == nil {
		return ``, ErrNoCorpus
	}
	if p.overlayBroken {
		return ``, ErrOverlayBroken
	}
	rec, ok := Get(id)
	if !ok {
		return ``, ErrNoRecord
	}
	switch {
	case rec.Status == StatusRetired:
		return ``, ErrPromoteRetired
	case rec.Generator != GeneratorOpenAI:
		return ``, ErrPromoteNotModel
	case rec.PlayerKey:
		return ``, ErrPromotePlayerKey
	case rec.HandEdited: // Edit sets it and leaves Moderated: an edit is refused as an edit
		return ``, ErrPromoteEdited
	case !rec.Moderated:
		return ``, ErrPromoteUnmoderated
	case !rec.Tier.Valid():
		return ``, fmt.Errorf(`its tier %q is not a tier`, rec.Tier)
	}
	key, err := p.promotionKey(rec)
	if err != nil {
		return ``, err
	}
	for _, slots := range p.promoted {
		for _, s := range slots {
			if s.raw.FromRecord == rec.Id {
				return ``, ErrPromoteDuplicate
			}
		}
	}
	if p.nameInPool(key, rec.Name) {
		return ``, ErrPromoteNameTaken
	}
	entry := CorpusEntry{Name: rec.Name, NameSimple: rec.NameSimple, Description: rec.Description, Material: rec.Material, WeightLbs: rec.WeightLbs, Value: rec.Value}
	cleaned, err := checkEntry(entry)
	if err != nil {
		return ``, err
	}
	if rec.Source == SourcePickpocket && TooBigFor(cleaned.reply(), SourcePickpocket) {
		return ``, ErrPromoteTooBig
	}
	slot := promotedSlot{
		raw: PromotedEntry{
			CorpusEntry: entry, FromRecord: rec.Id, Zone: rec.Zone, Biome: rec.Biome,
			Model: rec.Model, PromptVersion: rec.PromptVersion, PromotedAt: time.Now().UTC(),
		},
		use: cleaned,
		ok:  true,
	}
	next := p.withPromoted(func(m map[string][]promotedSlot) { m[key] = append(m[key], slot) })
	if err := saveOverlay(next.overlayPath, next.promoted); err != nil {
		return ``, err
	}
	corpus.Store(next)
	return key, nil
}

// RemoveCorpusEntry removes one promoted entry from a key, named by its
// name (any case) or by the record it was promoted from (FromRecord), never
// by a position that shifts as entries come and go. A name two entries
// share is refused: name the record instead. Seed entries are tracked
// content: edit the file.
func RemoveCorpusEntry(key, which string) (PromotedEntry, error) {
	corpusWriteMu.Lock()
	defer corpusWriteMu.Unlock()
	p := corpus.Load()
	if p == nil {
		return PromotedEntry{}, ErrNoCorpus
	}
	if p.overlayBroken {
		return PromotedEntry{}, ErrOverlayBroken
	}
	k, w := normKey(key), normKey(which)
	at := -1
	for i, s := range p.promoted[k] {
		if normKey(s.raw.Name) != w && normKey(s.raw.FromRecord) != w {
			continue
		}
		if at >= 0 {
			return PromotedEntry{}, fmt.Errorf(`more than one promoted entry in %s matches %q; name its record instead (bauble corpus list %s)`, k, which, k)
		}
		at = i
	}
	if at < 0 {
		for _, e := range p.seed[k] {
			if normKey(e.Name) == w {
				return PromotedEntry{}, fmt.Errorf(`%q is a seed entry: edit bauble-corpus.yaml, then bauble corpus reload`, e.Name)
			}
		}
		return PromotedEntry{}, fmt.Errorf(`%s has no promoted entry called %q`, k, which)
	}
	removed := p.promoted[k][at].raw
	next := p.withPromoted(func(m map[string][]promotedSlot) {
		s := m[k]
		m[k] = append(append([]promotedSlot(nil), s[:at]...), s[at+1:]...)
	})
	if err := saveOverlay(next.overlayPath, next.promoted); err != nil {
		return PromotedEntry{}, err
	}
	corpus.Store(next)
	return removed, nil
}

// removePromotedFrom removes every overlay entry promoted from recordId
// (Retire, Edit, ApplyRegenerated). It returns how many went. With the
// overlay broken it cannot know what the file holds, so it refuses.
func removePromotedFrom(recordId string) (int, error) {
	corpusWriteMu.Lock()
	defer corpusWriteMu.Unlock()
	p := corpus.Load()
	if p == nil {
		return 0, nil
	}
	if p.overlayBroken {
		return 0, ErrOverlayBroken
	}
	count := 0
	next := p.withPromoted(func(m map[string][]promotedSlot) {
		for k, slots := range m {
			kept := slots[:0]
			for _, s := range slots {
				if s.raw.FromRecord == recordId {
					count++
					continue
				}
				kept = append(kept, s)
			}
			m[k] = kept
		}
	})
	if count == 0 {
		return 0, nil
	}
	if err := saveOverlay(next.overlayPath, next.promoted); err != nil {
		return 0, err
	}
	corpus.Store(next)
	return count, nil
}

// CorpusKeyCount is one row of CorpusKeys.
type CorpusKeyCount struct {
	Key      string
	Seed     int
	Promoted int // every overlay entry, used or not
}

// CorpusKeys lists every key with entries, sorted.
func CorpusKeys() []CorpusKeyCount {
	p := corpus.Load()
	if p == nil {
		return nil
	}
	counts := map[string]*CorpusKeyCount{}
	get := func(k string) *CorpusKeyCount {
		c, ok := counts[k]
		if !ok {
			c = &CorpusKeyCount{Key: k}
			counts[k] = c
		}
		return c
	}
	for k, list := range p.seed {
		get(k).Seed = len(list)
	}
	for k, slots := range p.promoted {
		get(k).Promoted = len(slots)
	}
	out := make([]CorpusKeyCount, 0, len(counts))
	for _, c := range counts {
		out = append(out, *c)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Key < out[b].Key })
	return out
}

// CorpusListing is one key's entries.
type CorpusListing struct {
	Seed     []CorpusEntry
	Promoted []PromotedEntry // in overlay order
	Unused   map[int]string  // position in Promoted, from 1 -> why it is not used
}

// CorpusList lists one key's entries.
func CorpusList(key string) CorpusListing {
	out := CorpusListing{Unused: map[int]string{}}
	p := corpus.Load()
	if p == nil {
		return out
	}
	k := normKey(key)
	out.Seed = append(out.Seed, p.seed[k]...)
	for i, s := range p.promoted[k] {
		out.Promoted = append(out.Promoted, s.raw)
		if !s.ok {
			out.Unused[i+1] = s.why
		}
	}
	return out
}

// ExportPromoted renders the usable promoted entries in the seed's format
// (no provenance), to copy into bauble-corpus.yaml.
func ExportPromoted() (string, error) {
	doc := seedDoc{Entries: map[string][]CorpusEntry{}}
	if p := corpus.Load(); p != nil {
		for k, slots := range p.promoted {
			for _, s := range slots {
				if s.ok {
					doc.Entries[k] = append(doc.Entries[k], s.raw.CorpusEntry)
				}
			}
		}
	}
	data, err := yaml.Marshal(doc)
	return string(data), err
}
```

In `internal/baubles/admin.go` replace `Retire` with:

```go
// Retire withdraws a record's text: every item pointing at it shows a plain
// "Trinket" until it is restored. Value, weight and provenance are kept.
// For a name or description that should not be in the game, which is why
// any corpus entry promoted from it goes too.
func Retire(id string, admin string) error {
	if _, ok := Update(id, func(r *Record) {
		r.Status = StatusRetired
		r.EditedBy = admin
	}); !ok {
		return ErrNoRecord
	}
	if _, err := removePromotedFrom(id); err != nil {
		return fmt.Errorf(`%w: %w`, ErrCorpusCleanup, err)
	}
	return nil
}
```

`Edit` and `ApplyRegenerated` change a record's text, so an overlay entry promoted from it would keep text nobody approved any more. Both now remove those entries after the record is saved, and return how many. In `admin.go`:

1. Change `Edit`'s signature to `func Edit(id string, field string, value string, admin string) (Record, int, error) {`, and in its body change every `return Record{}, ` to `return Record{}, 0, `.
2. Change `ApplyRegenerated`'s signature to `func ApplyRegenerated(id string, res GenResult, admin string, randn func(n int) int) (Record, int, error) {` (keep H2's `randn`), and in its body change every `return Record{}, ` to `return Record{}, 0, `. In its doc comment also replace `// the theft fields are kept. It refuses a generic trinket: regenerating is` with `// the theft fields are kept. It refuses a fallback (corpus or generic): regenerating is`.
3. In each of the two, replace the final `return updated, nil` with:

```go
	// Text promoted from this record no longer matches it.
	removed, err := removePromotedFrom(id)
	if err != nil {
		return updated, 0, fmt.Errorf(`%w: %w`, ErrCorpusCleanup, err)
	}
	return updated, removed, nil
```

4. Add a last sentence to each doc comment: `Entries promoted into the fallback corpus from the record are removed; the int is how many. ErrCorpusCleanup means the record changed but they could not be.`

Let the compiler list the callers:

```bash
cd /c/tmp/dogmud-baubles-c && go vet ./internal/baubles/ ./internal/actions/ ./internal/usercommands/ 2>&1 | grep -E "assignment mismatch|too many|not enough" | head -30
```

Expected at `b5ac8b0fa`: the `Edit` and `ApplyRegenerated` calls in `internal/baubles/admin_test.go` (`TestEdit`, `TestApplyRegenerated`, `TestApplyRegeneratedSetsPlayerKey`, `TestApplyRegeneratedRollsPlayerKeyValue`, and Task 1's three tests), in `internal/baubles/generate_test.go` (`TestFinderOnlyReachesTheRecordAndRegenClearsIt`, line 493), `internal/actions/bauble_admin.go` and `internal/usercommands/admin.bauble.go`. In the two test files, only in calls to `Edit(` and `ApplyRegenerated(` (never `Get(`), rewrite `got, err :=` as `got, _, err :=`, `got, err =` as `got, _, err =` (`TestApplyRegeneratedRollsPlayerKeyValue`'s second call), `got, _ :=` as `got, _, _ :=` and `_, err :=` as `_, _, err :=`; every `ApplyRegenerated` call keeps its `first` argument.

In `internal/actions/bauble_admin.go` `RegenerateBauble`, add `"errors"` to the imports and replace

```go
		updated, err := baubles.ApplyRegenerated(id, res, adminName, util.Rand)
		if err != nil {
			tellBaubleAdmin(adminUserId, fmt.Sprintf(`Bauble %s was not regenerated: %s.`, id, err))
			return
		}
		tellBaubleAdmin(adminUserId, fmt.Sprintf(`Bauble %s is now <ansi fg="itemname">%s</ansi> (%d gold, %.1f lb): %s`,
			id, updated.Name, updated.Value, updated.WeightLbs, updated.Description))
```

with

```go
		updated, removed, err := baubles.ApplyRegenerated(id, res, adminName, util.Rand)
		if err != nil && !errors.Is(err, baubles.ErrCorpusCleanup) {
			tellBaubleAdmin(adminUserId, fmt.Sprintf(`Bauble %s was not regenerated: %s.`, id, err))
			return
		}
		tellBaubleAdmin(adminUserId, fmt.Sprintf(`Bauble %s is now <ansi fg="itemname">%s</ansi> (%d gold, %.1f lb): %s`,
			id, updated.Name, updated.Value, updated.WeightLbs, updated.Description))
		if removed > 0 {
			tellBaubleAdmin(adminUserId, fmt.Sprintf(`Its old text left the fallback corpus (promoted entries removed: %d).`, removed))
		}
		if err != nil {
			tellBaubleAdmin(adminUserId, fmt.Sprintf(`<ansi fg="red">%s.</ansi>`, err))
		}
```

In `internal/usercommands/admin.bauble.go` add `"errors"` to the imports. In `baubleEdit` replace

```go
	updated, err := baubles.Edit(rec.Id, args[fieldAt], strings.Join(args[fieldAt+1:], ` `), user.Character.Name)
	if err != nil {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`Not changed: %s.`, err))
		return true, nil
	}
```

with

```go
	updated, removed, err := baubles.Edit(rec.Id, args[fieldAt], strings.Join(args[fieldAt+1:], ` `), user.Character.Name)
	if err != nil && !errors.Is(err, baubles.ErrCorpusCleanup) {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`Not changed: %s.`, err))
		return true, nil
	}
	if removed > 0 {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`Its old text left the fallback corpus (promoted entries removed: %d).`, removed))
	}
	if err != nil {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="red">%s.</ansi>`, err))
	}
```

(the "Bauble %s is now ..." line that follows stays). In `baubleRetire` replace

```go
		if err := baubles.Retire(rec.Id, user.Character.Name); err != nil {
			user.SendText(messaging.CategorySystem, err.Error())
			return true, nil
		}
```

with

```go
		if err := baubles.Retire(rec.Id, user.Character.Name); err != nil && !errors.Is(err, baubles.ErrCorpusCleanup) {
			user.SendText(messaging.CategorySystem, err.Error())
			return true, nil
		} else if err != nil {
			user.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="red">%s.</ansi>`, err))
		}
```

- [ ] **Step 4: Run to see them pass, then the package.**

```bash
cd /c/tmp/dogmud-baubles-c && go test ./internal/baubles/ -run 'TestPromote|TestRetireRemoves|TestEditAndRegenRemove|TestRemoveCorpusEntry|TestABrokenOverlay|TestAnUnusableOverlay|TestOverlaySurvives|TestExportPromoted' -v 2>&1 | grep -E "^(--- |ok|FAIL)"
cd /c/tmp/dogmud-baubles-c && go test ./internal/baubles/ 2>&1 | tail -3
```

Expected: all PASS, then `ok`.

- [ ] **Step 5: FinalTwist's catalog prune.** Search every file of the package (the prune may live anywhere in it, not only in `catalog.go` or `store.go`):

```bash
cd /c/tmp/dogmud-baubles-c && grep -n -i "prune" internal/baubles/*.go | grep -v "_test.go" | grep -v "window.go"
```

Read each hit. The one wanted removes catalog records (and rewrites or deletes shard files); `window.go`'s `pruneLocked` sweeps search windows and does not count. Replace the `// PRUNE:` comment line in `TestOverlaySurvivesTheCatalog` with a call to it, using its real signature and doc comment: before the call, seed a record it will remove (for example one old enough, or sold long enough ago, by its rule), and after the call assert that `Get` no longer finds that record, so the test proves the prune ran over the overlay's directory. Rerun the test and confirm green.

If no catalog prune exists, STOP and report BLOCKED to the controller: FinalTwist's fix round was expected to land it before this slice branched (Task 0 Step 6). Do not leave the `// PRUNE:` line in and carry on; the gate (Task 14 Step 4) fails while it is there.

At `b5ac8b0fa` the prune exists (Task 0 Step 6): `applySweep(now time.Time, refs map[string]bool, keep time.Duration) (referenced int, pruned int, shardErrors int)` in `sweep.go`, callable from this in-package test. A record becomes prunable only after `minUnseenSweeps` (2) calls in a row that leave it out of `refs`, and only once `keep` has passed since its `lastEvidence()`, which the first such call moves to `now` for a record with no `LastSeenAt`. So call it at least twice with an empty `refs` and a `keep` the record has outlived (0 with the same `now` works), check that `pruned` counts the seeded record and `shardErrors` is 0, then that `Get` no longer finds it. `RunSweep` is not the tool here: it also walks the live sources and every save file under DataFiles.

- [ ] **Step 6: Null probes (five).** One at a time, restoring after each: (a) in `Promote`, move `corpus.Store(next)` above the `saveOverlay` call; `TestPromoteChangesNothingWhenTheSaveFails` must fail with "nothing is published in memory". (b) In `Promote`, delete the `case !rec.Moderated:` pair; `TestPromoteRefusals` must fail naming `unmoderated`. (c) In `Promote`, change `case rec.HandEdited:` to `case rec.EditedBy != "":`; `TestPromoteLooksAtHandEditedNotEditedBy` must fail with "freshly regenerated". (d) In `Promote`, delete the `if p.nameInPool(key, rec.Name) {` block; `TestPromoteRefusals` must fail naming `a name in its group's seed`. (e) In `Edit`, replace the `removePromotedFrom` block with `return updated, 0, nil`; `TestEditAndRegenRemoveTheRecordsCorpusEntry` must fail with "an edit removes". Confirm green.

- [ ] **Step 7: The packages the signature change reached.**

```bash
cd /c/tmp/dogmud-baubles-c && go build ./... && go test ./internal/baubles/ ./internal/actions/ ./internal/usercommands/ 2>&1 | tail -5
```

Expected: `ok` three times.

- [ ] **Step 8: Commit.**

```bash
cd /c/tmp/dogmud-baubles-c && git add internal/baubles/corpus_admin.go internal/baubles/admin.go internal/baubles/corpus_admin_test.go internal/baubles/admin_test.go internal/baubles/generate_test.go internal/actions/bauble_admin.go internal/usercommands/admin.bauble.go
git commit -F - <<'EOF'
feat(baubles): promote server-key model names into the corpus overlay

Promote refuses player-key, unmoderated, hand-edited, retired and
non-model records, and a name its pool already has; a sold, restored or
regenerated record is promotable. Retire, Edit and regen remove the
record's overlay entries (Edit and regen say how many), and a removal is
by name or record id. Every write saves the overlay before the new pool
is published, an unusable overlay entry is kept rather than dropped, and
an overlay that could not be set aside is never written.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 8: Load the corpus at boot

**Files:**
- Modify: `main.go` (`loadAllDataFiles`, directly after the `if !isReload { ... baubles.Load() ... }` block)

- [ ] **Step 1: Implement.** After the closing brace of the `if !isReload {` block that calls `baubles.Load()`, insert:

```go
	// Baubles: the fallback corpus (bauble-corpus.yaml, plus the promoted
	// overlay in baubles/). After items, because every entry is checked
	// against the authored item names, and after the catalog, whose
	// directory holds the overlay. On a data reload too, so an edit to the
	// seed takes effect. It never fails: a broken seed is logged at ERROR
	// and a corrupt overlay is quarantined.
	baubles.LoadCorpus()
```

- [ ] **Step 2: Build.**

```bash
cd /c/tmp/dogmud-baubles-c && go build -o /dev/null . 2>&1 | tail -5
```

Expected: no output. (No server boot here; Task 13 boots it through the playtest harness.)

- [ ] **Step 3: Commit.**

```bash
cd /c/tmp/dogmud-baubles-c && git add main.go
git commit -F - <<'EOF'
feat(baubles): load the fallback corpus at boot and on data reload

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 9: Admin command, help and status text

**Files:**
- Modify: `internal/usercommands/admin.bauble.go`
- Modify: `_datafiles/world/dogmud/templates/admincommands/help/command.bauble.template`
- Test: `internal/usercommands/admin.bauble_test.go`

- [ ] **Step 1: Write the failing test.** In `internal/usercommands/admin.bauble_test.go` add `"os"`, `"path/filepath"`, `"github.com/GoMudEngine/GoMud/internal/rooms"` and `"github.com/GoMudEngine/GoMud/internal/users"` to the imports (`events` is already imported at `b5ac8b0fa`; adding it again would not compile) and append:

```go
// adminSaid runs one bauble subcommand and returns what the admin was sent,
// whitespace folded, so a line the renderer wrapped still matches.
func adminSaid(t *testing.T, cmd string, admin *users.UserRecord, room *rooms.Room) string {
	t.Helper()
	events.DrainQueuedMessagesForTest(admin.UserId)
	_, err := Bauble(cmd, admin, room, 0)
	require.NoError(t, err, cmd)
	return strings.Join(strings.Fields(strings.Join(events.DrainQueuedMessagesForTest(admin.UserId), " ")), " ")
}

// bauble promote and bauble corpus (slice C): promote puts a record's text
// in the overlay under its biome and tier; remove takes it out again by
// name; every subcommand says what it did.
func TestAdminBauble_PromoteAndCorpus(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreItems := items.SeedItemsForTest(map[int]*items.ItemSpec{
		items.BaubleItemId: {ItemId: items.BaubleItemId, Name: "Curious Trinket", NameSimple: "trinket",
			Type: items.Object, Subtype: items.Mundane, Weight: 0.2, Value: 1, NotSalable: true},
	})
	defer restoreItems()
	dir := t.TempDir()
	baubles.SetDirForTest(dir)
	defer items.SetBaubleResolver(nil)
	seedPath := filepath.Join(dir, "bauble-corpus.yaml")
	require.NoError(t, os.WriteFile(seedPath, []byte("groups:\n  interior: dwelling\nentries: {}\n"), 0o644))
	baubles.LoadCorpusFrom(seedPath, filepath.Join(dir, "corpus.promoted.yaml"))
	defer baubles.ClearCorpusForTest()

	admin, room := getTestUserAndRoom(t)
	rec, err := baubles.Create(baubles.Record{
		Name: "Painted Wooden Spool", NameSimple: "spool", Tier: baubles.TierCheap, Value: 4, WeightLbs: 0.2,
		Description: "A wooden thread spool painted with a band of faded blue.",
		Status:      baubles.StatusReady, Generator: baubles.GeneratorOpenAI, Moderated: true,
		Source: baubles.SourceSearch, Biome: "interior", Zone: "ashwick",
	})
	require.NoError(t, err)

	out := adminSaid(t, "promote "+rec.Id, admin, room)
	require.Len(t, baubles.CorpusList("interior-cheap").Promoted, 1, "promote puts it under its biome and tier")
	assert.Contains(t, out, "is now in the fallback corpus under interior-cheap")
	assert.Contains(t, adminSaid(t, "promote "+rec.Id, admin, room), "Not promoted: it is already in the corpus.")

	assert.Contains(t, adminSaid(t, "corpus", admin, room), "Usage: bauble corpus list")
	out = adminSaid(t, "corpus list", admin, room)
	assert.Contains(t, out, "Fallback corpus: 0 seed and 1 promoted entries in use.")
	assert.Contains(t, out, "interior-cheap seed 0 promoted 1")
	out = adminSaid(t, "corpus list interior-cheap", admin, room)
	assert.Contains(t, out, "interior-cheap: 0 seed, 1 promoted.")
	assert.Contains(t, out, "Painted Wooden Spool (spool, 0.2 lb, 4 gold) from "+rec.Id+", zone ashwick")
	out = adminSaid(t, "corpus export", admin, room)
	assert.Contains(t, out, "Promoted entries in the seed's format")
	assert.Contains(t, out, "name: Painted Wooden Spool")
	assert.NotContains(t, out, "from_record", "an export is seed format, no provenance")

	assert.Contains(t, adminSaid(t, "corpus remove interior-cheap Silver Spoon", admin, room),
		"Not removed: interior-cheap has no promoted entry called")
	assert.Contains(t, adminSaid(t, "corpus remove interior-cheap", admin, room), "Usage: bauble corpus remove")
	require.Len(t, baubles.CorpusList("interior-cheap").Promoted, 1, "a bad remove changes nothing")

	assert.Contains(t, adminSaid(t, "status", admin, room), "Fallback corpus: 0 seed and 1 promoted entries")
	assert.Contains(t, adminSaid(t, "stats", admin, room), "from the corpus 0")

	out = adminSaid(t, "corpus remove interior-cheap painted wooden spool", admin, room)
	assert.Contains(t, out, "Removed Painted Wooden Spool (promoted from "+rec.Id+") from interior-cheap.")
	assert.Empty(t, baubles.CorpusList("interior-cheap").Promoted)
	assert.Contains(t, adminSaid(t, "corpus export", admin, room), "No promoted entries to export.")

	out = adminSaid(t, "corpus reload", admin, room)
	assert.Contains(t, out, "Corpus reloaded: 0 seed and 0 promoted entries in use, 0 skipped.")
	_, promoted := baubles.CorpusCounts()
	assert.Equal(t, 0, promoted, "reload reads the saved overlay back from the corpus's own files")

	// A seed that breaks after boot: the reload says it kept the one in use.
	require.NoError(t, os.WriteFile(seedPath, []byte("entries: [not a map\n"), 0o644))
	out = adminSaid(t, "corpus reload", admin, room)
	assert.Contains(t, out, "The seed file could not be read")
	assert.Contains(t, out, "the seed already in use is kept")
}
```

- [ ] **Step 2: Run to see it fail.**

```bash
cd /c/tmp/dogmud-baubles-c && go test ./internal/usercommands/ -run TestAdminBauble_PromoteAndCorpus -v 2>&1 | tail -8
```

Expected: FAIL at "promote puts it under its biome and tier" (the `promote` subcommand falls to usage).

- [ ] **Step 3: Implement.** In `internal/usercommands/admin.bauble.go`:

In the `Bauble` doc comment replace `what names baubles (model or generic) and the search settings` with `what names baubles (model, corpus or generic) and the search settings`, and add, after the `restore` line:

```go
//	bauble promote <bauble>               copy a model name into the fallback corpus
//	bauble corpus list [key]              the corpus pools, or one pool's entries
//	bauble corpus remove <key> <entry>    remove a promoted entry, by its name or record id
//	bauble corpus reload                  read the seed and the promoted file again
//	bauble corpus export                  the promoted entries in the seed's format
```

Add two cases to the switch, after the `restore` case:

```go
	case `promote`:
		return baublePromote(args[1:], user, room)
	case `corpus`:
		return baubleCorpus(args[1:], user)
```

In `baubleUsage`'s fallback text, after `"  bauble restore <bauble>\r\n"+` add:

```go
			"  bauble promote <bauble>\r\n"+
			"  bauble corpus list [key] | remove <key> <name or record id> | reload | export\r\n"+
```

In `baubleSpawn` replace the comment

```go
	// The same path a search find takes: named in the background (by the
	// model when one is set up, otherwise a generic trinket), then delivered
	// to your pack after at least BaubleRevealSeconds.
```

with

```go
	// The same path a search find takes: named in the background (by the
	// model when one is set up, otherwise from the fallback corpus), then
	// delivered to your pack after at least BaubleRevealSeconds.
```

and replace `how := `a generic trinket (no model is set up)`` with:

```go
	how := `from the fallback corpus (no model is set up)`
```

In `baubleStatus` replace the `else` branch's line with:

```go
		b.WriteString("Naming: <ansi fg=\"yellow\">fallback corpus</ansi>. No model is set up: Modules.baubles is off or no OpenAI API key was found.\r\n")
```

and directly after that `if/else` add:

```go
	seedN, promotedN := baubles.CorpusCounts()
	fmt.Fprintf(&b, "Fallback corpus: %d seed and %d promoted entries (bauble corpus list); a plain Trinket where none fits.\r\n", seedN, promotedN)
```

In `baubleStats` replace the "Named:" pair with:

```go
	fmt.Fprintf(&b, "  Named:   by model %d, from the corpus %d, generic %d, admin-edited %d\r\n",
		st.ByGenerator[baubles.GeneratorOpenAI], st.ByGenerator[baubles.GeneratorCorpus], st.ByGenerator[baubles.GeneratorLocal], st.Edited)
```

Append the two handlers at the end of the file:

```go
// baublePromote copies a record's text into the fallback corpus.
func baublePromote(args []string, user *users.UserRecord, room *rooms.Room) (bool, error) {
	rec, ok := resolveBaubleArg(strings.Join(args, ` `), user, room)
	if !ok {
		return true, nil
	}
	key, err := baubles.Promote(rec.Id)
	if err != nil {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`Not promoted: %s.`, err))
		return true, nil
	}
	user.SendText(messaging.CategorySystem, fmt.Sprintf(`Bauble %s (%s) is now in the fallback corpus under %s. Finds with no model may use its text.`, rec.Id, rec.Name, key))
	return true, nil
}

// baubleCorpus lists, removes from, reloads and exports the fallback corpus.
func baubleCorpus(args []string, user *users.UserRecord) (bool, error) {
	usage := `Usage: bauble corpus list [key] | remove <key> <name or record id> | reload | export`
	if len(args) == 0 {
		user.SendText(messaging.CategorySystem, usage)
		return true, nil
	}
	var b strings.Builder
	switch strings.ToLower(args[0]) {
	case `list`:
		if len(args) > 1 {
			key := strings.ToLower(args[1])
			l := baubles.CorpusList(key)
			fmt.Fprintf(&b, "%s: %d seed, %d promoted.\r\n", key, len(l.Seed), len(l.Promoted))
			for _, e := range l.Seed {
				fmt.Fprintf(&b, "  seed  %s (%s, %s, %.1f lb, %d gold)\r\n", e.Name, e.NameSimple, e.Material, e.WeightLbs, e.Value)
			}
			for i, e := range l.Promoted {
				fmt.Fprintf(&b, "  promoted  %s (%s, %.1f lb, %d gold) from %s, zone %s, promoted %s",
					e.Name, e.NameSimple, e.WeightLbs, e.Value, e.FromRecord, e.Zone, e.PromotedAt.Format(`2006-01-02`))
				if why := l.Unused[i+1]; why != `` {
					fmt.Fprintf(&b, " <ansi fg=\"red\">NOT USED: %s</ansi>", why)
				}
				b.WriteString("\r\n")
			}
			break
		}
		seedN, promotedN := baubles.CorpusCounts()
		fmt.Fprintf(&b, "Fallback corpus: %d seed and %d promoted entries in use.\r\n", seedN, promotedN)
		for _, c := range baubles.CorpusKeys() {
			fmt.Fprintf(&b, "  %-26s seed %3d  promoted %3d\r\n", c.Key, c.Seed, c.Promoted)
		}
	case `remove`:
		if len(args) < 3 {
			b.WriteString(`Usage: bauble corpus remove <key> <name or record id>. "bauble corpus list <key>" shows both.`)
			break
		}
		removed, err := baubles.RemoveCorpusEntry(args[1], strings.Join(args[2:], ` `))
		if err != nil {
			fmt.Fprintf(&b, `Not removed: %s.`, err)
			break
		}
		fmt.Fprintf(&b, `Removed %s (promoted from %s) from %s.`, removed.Name, removed.FromRecord, strings.ToLower(args[1]))
	case `reload`:
		rep := baubles.ReloadCorpus()
		fmt.Fprintf(&b, "Corpus reloaded: %d seed and %d promoted entries in use, %d skipped.\r\n", rep.Seed, rep.Promoted, len(rep.Skipped))
		if rep.SeedErr != nil {
			fmt.Fprintf(&b, "<ansi fg=\"red\">The seed file could not be read: %s.</ansi>\r\n", rep.SeedErr)
			if rep.SeedKept {
				b.WriteString("Nothing was lost: the seed already in use is kept until a reload reads the file.\r\n")
			}
		}
		if rep.Quarantined != `` {
			fmt.Fprintf(&b, "<ansi fg=\"red\">The promoted file could not be read and is set aside at %s.</ansi>\r\n", rep.Quarantined)
		}
		if rep.OverlayBroken {
			b.WriteString("<ansi fg=\"red\">The promoted file could not be read or set aside: promote and remove are refused until a reload succeeds (see the log).</ansi>\r\n")
		}
		for i, s := range rep.Skipped {
			if i == 10 {
				fmt.Fprintf(&b, "  ...and %d more (see the log).\r\n", len(rep.Skipped)-10)
				break
			}
			fmt.Fprintf(&b, "  skipped: %s\r\n", s)
		}
	case `export`:
		if _, promotedN := baubles.CorpusCounts(); promotedN == 0 {
			b.WriteString(`No promoted entries to export.`)
			break
		}
		out, err := baubles.ExportPromoted()
		if err != nil {
			fmt.Fprintf(&b, `Could not export: %s.`, err)
			break
		}
		b.WriteString("Promoted entries in the seed's format (copy them under entries: in bauble-corpus.yaml and wrap the descriptions):\r\n")
		b.WriteString(strings.ReplaceAll(out, "\n", "\r\n"))
	default:
		b.WriteString(usage)
	}
	user.SendText(messaging.CategorySystem, b.String())
	return true, nil
}
```

In `_datafiles/world/dogmud/templates/admincommands/help/command.bauble.template`:

Replace

```
  what names them (a finder's own key, the server key, or generic
  trinkets), the shared token budget, and the search settings.
```

with

```
  what names them (a finder's own key, the server key, or the
  fallback corpus), the shared token budget, and the search settings.
```

After the two `bauble restore` lines (`<ansi fg="command">bauble restore <bauble></ansi>` and `  Undo retire.`) insert:

```

<ansi fg="command">bauble promote <bauble></ansi>
  Copy a good model name into the fallback corpus, so finds made
  with no model can use its text. Only a find the model named on
  the server's key, passed by moderation and never changed by hand
  can be promoted, and only a name its pool does not have yet; a
  sold, restored or regenerated one can. Retiring, editing or
  regenerating it later removes it from the corpus again.

<ansi fg="yellow-bold">Fallback corpus:</ansi>

<ansi fg="command">bauble corpus list [key]</ansi>
  Every pool and its size, or one pool's entries. A key is a biome
  or a group and a tier (<ansi fg="cyan">interior-cheap</ansi>, <ansi fg="cyan">dwelling-rare</ansi>),
  <ansi fg="cyan">pocket-average</ansi> for pickpocketed finds, or a tier alone.
<ansi fg="command">bauble corpus remove <key> <entry></ansi>
  Remove a promoted entry from a pool, named by its name or by the
  bauble it was promoted from (<ansi fg="cyan">B0000012</ansi>). Seed entries are
  changed in bauble-corpus.yaml.
<ansi fg="command">bauble corpus reload</ansi>
  Read bauble-corpus.yaml and the promoted file again.
<ansi fg="command">bauble corpus export</ansi>
  Print the promoted entries in the seed file's format, to copy
  into bauble-corpus.yaml.
```

Under `Notes:` replace

```
  - Records live under <ansi fg="cyan">_datafiles/world/dogmud/baubles/</ansi>.
```

with

```
  - Records live under <ansi fg="cyan">_datafiles/world/dogmud/baubles/</ansi>.
  - With no model, a find takes its text from the fallback corpus:
    <ansi fg="cyan">bauble-corpus.yaml</ansi> (tracked) and <ansi fg="cyan">baubles/corpus.promoted.yaml</ansi>
    (promoted names), by biome, biome group and tier. A plain
    Trinket only when nothing there fits.
```

- [ ] **Step 4: Run to see it pass, then the package.**

```bash
cd /c/tmp/dogmud-baubles-c && go test ./internal/usercommands/ -run TestAdminBauble -v 2>&1 | grep -E "^(--- |ok|FAIL)"
cd /c/tmp/dogmud-baubles-c && go test ./internal/usercommands/ 2>&1 | tail -3
```

Expected: PASS, then `ok`.

- [ ] **Step 5: Commit.**

```bash
cd /c/tmp/dogmud-baubles-c && git add internal/usercommands/admin.bauble.go internal/usercommands/admin.bauble_test.go _datafiles/world/dogmud/templates/admincommands/help/command.bauble.template
git commit -F - <<'EOF'
feat(usercommands): bauble promote and bauble corpus

Admin-only, like bauble. status, spawn and stats name the corpus.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 10: The seed file format and its CI validation

**Files:**
- Create: `_datafiles/world/dogmud/bauble-corpus.yaml`
- Create: `internal/baubles/corpus_seed_test.go`

**Format (exact).** Two top-level keys, nothing else (`decodeStrict` refuses unknown fields):

- `groups:` a map from biome id (lowercase, as in `biomes/<id>.yaml`) to group name. Group names: `dwelling`, `street`, `underground`, `ruins`, `waterside`, `wild`. `pocket` is reserved for pickpocketed finds and may not be a group.
- `entries:` a map from key to a list of entries. A key is `<group>-<tier>`, `<biome>-<tier>` (a biome that appears in `groups`), `pocket-<tier>`, or a bare `<tier>`, where tier is `cheap`, `average` or `rare`.
- Each entry has exactly these fields: `name` (1 to 6 words, Title Case, no digits, at most 40 characters, never an existing item's name), `name_simple` (one lowercase word, 2 to 20 letters, not a reserved noun and not a word any real item answers to), `material` (optional, one or two lowercase words, at most 30 characters), `weight_lbs` (tenths of a pound, 0.1 to 25; 1.0 or less for `pocket-*` and bare-tier pools), `value` (whole gold inside the tier), `description` (folded block scalar `>-`, 20 to 400 characters, two short sentences, third person).

**Value and weight ranges from config** (`_datafiles/config.yaml` at `b5ac8b0fa`; `grep -n "Bauble.*Value:" _datafiles/config.yaml`):

| Tier | Value (gold) | Midpoint | Mean tolerance (15% of width) | Weight |
|---|---|---|---|---|
| cheap | 1 to 6 (lines 1546-1547) | 3.5 | 0.75 | 0.1 to 25 lb; pocket and bare tier 0.1 to 1.0 lb |
| average | 10 to 15 (1548-1549) | 12.5 | 0.75 | same |
| rare | 40 to 200 (1550-1551) | 120 | 24 | same |

Weight is not tiered anywhere in config: the 0.1 and 25 lb bounds are Go constants (`weight.go:15-16`) and the pocket limit is `BaublePickpocketMaxWeight: 1.0` (`config.yaml` 1487). Use the model's own scale (`WeightGuidance`, `weight.go:100-105`): tiny 0.1 to 0.5 (coin, ring, button, charm), small 0.5 to 2 (toy, figurine, cup, pipe), medium 2 to 8 (candlestick, jug, small box), large 8 to 25 (big vase, small chest).

**Three written examples per tier** (from different groups; these are the first entries of the seed):

| Tier | Key | Name | Keyword | Material | Weight | Value |
|---|---|---|---|---|---|---|
| cheap | dwelling-cheap | Bent Tin Thimble | thimble | tin | 0.1 | 3 |
| cheap | street-cheap | Chipped Clay Marble | marble | clay | 0.1 | 2 |
| cheap | underground-cheap | Corroded Copper Button | button | copper | 0.1 | 4 |
| average | waterside-average | Driftwood Heron Carving | heron | driftwood | 0.4 | 12 |
| average | ruins-average | Faded Mosaic Tile | tile | fired clay | 0.3 | 13 |
| average | wild-average | Polished Agate Pebble | agate | agate | 0.2 | 12 |
| rare | street-rare | Enamelled Guild Medallion | medallion | bronze | 0.5 | 125 |
| rare | pocket-rare | Garnet Hair Comb | comb | silver | 0.2 | 140 |
| rare | wild-rare | Spiral Snail Fossil | fossil | stone | 1.2 | 95 |

The nine keywords were grepped against every item file's `name:` and `namesimple:` (zero hits each on 2026-09-28; the same grep finds `lantern` twice, so it could succeed) and none is in `reservedNouns`. The CI test below is the authority.

- [ ] **Step 1: Write the seed file.** Create `_datafiles/world/dogmud/bauble-corpus.yaml`:

```yaml
# Bauble fallback corpus: the seed (docs/baubles/implementation-plan.md,
# Phase 6e). When no model names a find, its text comes from here, or from
# names an admin promoted (baubles/corpus.promoted.yaml, living state).
#
# groups: the corpus group of each biome. A biome with no group (water and
# ether, where nothing is ever found) goes straight to the tier pool.
# entries: pools of finds, keyed "<group or biome>-<tier>", "pocket-<tier>"
# for pickpocketed finds, or a bare tier as the last resort.
#
# Every entry is checked like a model's answer when it loads: no digits in
# the name, a keyword no real item answers to, a weight in tenths of a pound
# from 0.1 to 25. Values stay inside the tier (cheap 1 to 6, average 10 to
# 15, rare 40 to 200 gold) and each pool's average sits near the middle.
# Pocket and bare-tier entries weigh at most 1 lb and name nothing too big
# for a pocket. Write for players: wrap at 80 columns, no numbers in the
# text, plain words. TestShippedCorpusSeed checks all of it.
groups:
  interior: dwelling
  fort: dwelling
  city_backstreet: street
  city_thoroughfare: street
  sewer: underground
  dungeon: underground
  cave: underground
  ruins: ruins
  road: waterside
  shore: waterside
  river: waterside
  farmland: waterside
  land: waterside
  forest: wild
  dense_forest: wild
  plains: wild
  swamp: wild
  cliffs: wild
  mountains: wild
  desert: wild
  snow: wild
  spiderweb: wild
entries:
  dwelling-cheap:
    - name: Bent Tin Thimble
      name_simple: thimble
      material: tin
      weight_lbs: 0.1
      value: 3
      description: >-
        A tin thimble, pressed a little out of shape. The dimples on its top
        are worn smooth from years of pushing needles through heavy cloth.
  street-cheap:
    - name: Chipped Clay Marble
      name_simple: marble
      material: clay
      weight_lbs: 0.1
      value: 2
      description: >-
        A small clay marble, glazed blue a long time ago. One side is
        chipped flat, as if a cart wheel rolled over it.
  underground-cheap:
    - name: Corroded Copper Button
      name_simple: button
      material: copper
      weight_lbs: 0.1
      value: 4
      description: >-
        A copper button gone green with damp. Two thread holes still show,
        and a scrap of dark wool is caught in one of them.
  waterside-average:
    - name: Driftwood Heron Carving
      name_simple: heron
      material: driftwood
      weight_lbs: 0.4
      value: 12
      description: >-
        A heron carved from pale driftwood, standing on one leg. The water
        has rubbed it smooth, but the long beak is still sharp.
  ruins-average:
    - name: Faded Mosaic Tile
      name_simple: tile
      material: fired clay
      weight_lbs: 0.3
      value: 13
      description: >-
        A square tile from an old floor, painted with half of a red bird.
        The glaze is cracked, and grey mortar still clings to its back.
  wild-average:
    - name: Polished Agate Pebble
      name_simple: agate
      material: agate
      weight_lbs: 0.2
      value: 12
      description: >-
        A pebble of banded agate, red and cream, polished smooth by a
        stream. Held up to the light, its stripes glow like a sunset.
  street-rare:
    - name: Enamelled Guild Medallion
      name_simple: medallion
      material: bronze
      weight_lbs: 0.5
      value: 125
      description: >-
        A heavy medallion on a broken chain, enamelled green and gold with
        the mark of a merchant guild. The enamel is chipped at one edge.
  pocket-rare:
    - name: Garnet Hair Comb
      name_simple: comb
      material: silver
      weight_lbs: 0.2
      value: 140
      description: >-
        A silver comb for pinning up hair, set with a row of small, dark red
        garnets. A few fine hairs are still caught between its teeth.
  wild-rare:
    - name: Spiral Snail Fossil
      name_simple: fossil
      material: stone
      weight_lbs: 1.2
      value: 95
      description: >-
        A stone the size of a palm, split open to show a coiled snail shell
        inside. It turned to stone long before any town was built.
```

- [ ] **Step 2: Write the validation tests.** Create `internal/baubles/corpus_seed_test.go`:

```go
package baubles

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"gopkg.in/yaml.v3"
)

// shippedWorld is the real dogmud world, anchored on this file: the shared
// test binary's working directory is not the package's.
func shippedWorld(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal(`runtime.Caller failed`)
	}
	return filepath.Join(filepath.Dir(here), `..`, `..`, `_datafiles`, `world`, `dogmud`)
}

// loadShippedItems loads the real conditions and items, as boot does before
// the corpus, and puts the test binary's items back afterwards.
func loadShippedItems(t *testing.T, world string) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(world)
	cfg.Network.LogoutRounds = 3 // condition 0 refuses 0; a test binary never reads config.yaml
	configs.SetConfigForTest(t, cfg)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{}))
	conditions.LoadDataFiles()
	items.LoadDataFiles()
	// Without this the name and keyword checks below would pass anything.
	if !items.AuthoredKeyword(`lantern`) || !items.AuthoredName(`Hooded Lantern`) {
		t.Fatal(`the authored item snapshots are empty: items did not load`)
	}
}

func readShippedSeed(t *testing.T, world string) ([]byte, seedDoc) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(world, seedFileName))
	if err != nil {
		t.Fatalf(`read the seed: %v`, err)
	}
	var doc seedDoc
	if err := decodeStrict(data, &doc); err != nil {
		t.Fatalf(`the seed does not parse: %v`, err)
	}
	return data, doc
}

// TestShippedCorpusSeed is the seed's CI gate: every entry loads, reads as
// player copy, keeps the keyword its author wrote, and fits its pool.
func TestShippedCorpusSeed(t *testing.T) {
	world := shippedWorld(t)
	loadShippedItems(t, world)
	data, doc := readShippedSeed(t, world)

	for i, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		if n := utf8.RuneCountInString(line); n > 80 {
			t.Errorf(`line %d is %d columns; wrap at 80`, i+1, n)
		}
		if strings.ContainsAny(line, "\u2013\u2014") {
			t.Errorf(`line %d has an en or em dash`, i+1)
		}
	}

	names := map[string]string{}
	for _, key := range sortedKeys(doc.Entries) {
		prefix, tier, ok := parseCorpusKey(key)
		if !ok {
			t.Errorf(`%q is not a pool key`, key)
			continue
		}
		smallOnly := prefix == pocketPrefix || prefix == ``
		for i, e := range doc.Entries[key] {
			where := fmt.Sprintf(`%s #%d %q`, key, i+1, e.Name)
			if prev, dup := names[normKey(e.Name)]; dup {
				t.Errorf(`%s: the same name as %s`, where, prev)
			}
			names[normKey(e.Name)] = where
			if !tier.Range().Contains(e.Value) {
				t.Errorf(`%s: value %d is outside %s %+v`, where, e.Value, tier, tier.Range())
			}
			if strings.IndexFunc(e.Description+e.Material, unicode.IsDigit) >= 0 {
				t.Errorf(`%s: no numbers in the text`, where)
			}
			cleaned, err := checkEntry(e)
			if err != nil {
				t.Errorf(`%s: %v`, where, err)
				continue
			}
			if cleaned.NameSimple != strings.ToLower(e.NameSimple) {
				t.Errorf(`%s: keyword %q is taken (reserved, or a real item's word); players would type %q. Write that, or pick another noun`, where, e.NameSimple, cleaned.NameSimple)
			}
			if smallOnly && TooBigFor(cleaned.reply(), SourcePickpocket) {
				t.Errorf(`%s: pocket and bare-tier entries must fit a pocket`, where)
			}
		}
	}

	rep := LoadCorpusFrom(filepath.Join(world, seedFileName), filepath.Join(t.TempDir(), overlayFileName))
	t.Cleanup(ClearCorpusForTest)
	if rep.SeedErr != nil || len(rep.Skipped) > 0 {
		t.Fatalf("every seed entry must load:\n%v\n%s", rep.SeedErr, strings.Join(rep.Skipped, "\n"))
	}
}

// Every biome where a search can find something has a group, so its finds
// reach a group pool. BaseChance is the Go default here, which
// TestBaubleShippedConfigMatchesDefaults pins to the shipped config.
func TestEveryFindableBiomeHasACorpusGroup(t *testing.T) {
	world := shippedWorld(t)
	_, doc := readShippedSeed(t, world)
	files, err := os.ReadDir(filepath.Join(world, `biomes`))
	if err != nil || len(files) == 0 {
		t.Fatalf(`no biome files read (%v): this test would prove nothing`, err)
	}
	known := map[string]bool{`dwelling`: true, `street`: true, `underground`: true, `ruins`: true, `waterside`: true, `wild`: true}
	checked := 0
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), `.yaml`) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(world, `biomes`, f.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var b struct {
			BiomeId string `yaml:"biomeid"`
		}
		if err := yaml.Unmarshal(data, &b); err != nil {
			t.Fatalf(`%s: %v`, f.Name(), err)
		}
		id := normKey(b.BiomeId)
		if BaseChance(id) <= 0 {
			continue
		}
		checked++
		g, ok := doc.Groups[id]
		if !ok {
			t.Errorf(`biome %s finds baubles (%.2f%% per roll) but has no corpus group`, id, BaseChance(id))
			continue
		}
		if !known[g] {
			t.Errorf(`biome %s is in group %q, which is not one of the six`, id, g)
		}
	}
	if checked == 0 {
		t.Fatal(`no biome with a chance above zero: the check ran on nothing`)
	}
}
```

- [ ] **Step 3: Run.**

```bash
cd /c/tmp/dogmud-baubles-c && go test ./internal/baubles/ -run 'TestShippedCorpusSeed|TestEveryFindableBiomeHasACorpusGroup' -v 2>&1 | tail -10
```

Expected: both PASS. If `TestShippedCorpusSeed` reports a taken keyword, change that example's `name_simple` to what the message says players would type, or pick another noun, and update the table above to match.

- [ ] **Step 4: Null probes (three).** One at a time, restoring after each: (a) delete the `forest: wild` line from the seed; `TestEveryFindableBiomeHasACorpusGroup` must fail naming `forest`. (b) Change the thimble's `name_simple` to `key`; `TestShippedCorpusSeed` must fail naming `dwelling-cheap #1`. (c) Comment out `items.LoadDataFiles()` in `loadShippedItems`; the test must fail with "the authored item snapshots are empty". Confirm green after restoring.

- [ ] **Step 5: Run the root guards the new YAML file and the overlay writer meet** (text-surface registry, the word guard, durable writes). `TestNoStringOrDataSaysBuff` lists files with `git ls-files`, so an untracked seed is silently not scanned: stage it first. Then check the guard could see it.

```bash
cd /c/tmp/dogmud-baubles-c && git add _datafiles/world/dogmud/bauble-corpus.yaml && git ls-files _datafiles/world/dogmud/bauble-corpus.yaml
cd /c/tmp/dogmud-baubles-c && go test . -run 'TestEveryTextSurfaceIsRegistered|TestNoStringOrDataSaysBuff|TestLivingStateWritesAreDurable|TestNoHandRolledTempRename' -v 2>&1 | grep -E "^(--- |ok|FAIL)"
```

Expected: the path printed once, then four `--- PASS` and `ok`. (`saveOverlay` writes through `util.Save`, which is what `TestLivingStateWritesAreDurable` asks of a living-state writer.)

- [ ] **Step 6: Commit.**

```bash
cd /c/tmp/dogmud-baubles-c && git add _datafiles/world/dogmud/bauble-corpus.yaml internal/baubles/corpus_seed_test.go
git commit -F - <<'EOF'
feat(baubles): the corpus seed file, its groups, and its CI validation

Nine example entries, three per tier. The test loads items first and
proves the authored snapshots are not empty before it trusts a pass.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 11: Seed content (delegated drafting, owner review before merge)

**Files:**
- Modify: `_datafiles/world/dogmud/bauble-corpus.yaml`
- Modify: `internal/baubles/corpus_seed_test.go`

- [ ] **Step 1: Write the failing completeness test.** Append to `internal/baubles/corpus_seed_test.go` (add `"math"` to its imports):

```go
// The seed has every pool the lookup can reach, about ten entries per group
// pool and about five per bare-tier pool, and each pool's average value
// sits near its tier's midpoint, so a corpus find pays what a generic one
// did on average (owner ruling 8).
func TestShippedCorpusSeedIsComplete(t *testing.T) {
	_, doc := readShippedSeed(t, shippedWorld(t))
	want := map[string]int{}
	for _, g := range []string{`dwelling`, `street`, `underground`, `ruins`, `waterside`, `wild`, pocketPrefix} {
		for _, tier := range Tiers() {
			want[corpusKey(g, tier)] = 8
		}
	}
	for _, tier := range Tiers() {
		want[corpusKey(``, tier)] = 4
	}
	for _, key := range sortedKeys(want) {
		list := doc.Entries[key]
		if len(list) < want[key] {
			t.Errorf(`%s has %d entries, want at least %d`, key, len(list), want[key])
			continue
		}
		_, tier, _ := parseCorpusKey(key)
		r := tier.Range()
		sum := 0
		for _, e := range list {
			sum += tier.ClampValue(e.Value)
		}
		mean := float64(sum) / float64(len(list))
		mid := float64(r.Min+r.Max) / 2
		if tol := 0.15 * float64(r.Max-r.Min); math.Abs(mean-mid) > tol {
			t.Errorf(`%s: average value %.2f, want within %.2f of the %s midpoint %.1f`, key, mean, tol, tier, mid)
		}
	}
}
```

- [ ] **Step 2: Run to see it fail.**

```bash
cd /c/tmp/dogmud-baubles-c && go test ./internal/baubles/ -run TestShippedCorpusSeedIsComplete 2>&1 | tail -30
```

Expected: FAIL listing 24 short pools (21 group pools plus 3 bare tiers).

- [ ] **Step 3: Draft, delegated (controller).** Dispatch eight drafting subagents in parallel (model: sonnet), one per unit: `dwelling`, `street`, `underground`, `ruins`, `waterside`, `wild`, `pocket`, and `tiers` (the bare `cheap`, `average`, `rare` pools). Each writes ONLY to `<scratchpad>/corpus-draft-<unit>.yaml`, never to the repo, and returns when done. Prompt for each (fill in `<unit>` and its biomes, and `<plan path>`: `C:\tmp\dogmud-baubles-c\docs\superpowers\plans\2026-09-28-slice-c-bauble-corpus.md` when the docs-only PR had merged before this branch was cut, else the path the controller is reading this plan from):

> Load the skills `dogmud-player-copy` and `dogmud-authoring-content`. Read `<plan path>` Task 10 (the format, the value table and the nine examples) and `C:\tmp\dogmud-baubles-c\docs\world.md` for the setting. Draft the `<unit>` pools for the bauble fallback corpus: ten entries each for `<unit>-cheap`, `<unit>-average` and `<unit>-rare` (for `tiers`: five each for `cheap`, `average`, `rare`). Places: `<biomes of the unit>`. Baubles are small, sell-only objects someone lost or left: no weapons, armour, tools, food, keys, coins, potions, books or anything usable. Rules: YAML exactly as in Task 10, keys indented two spaces under `entries:`; descriptions as `>-` folded scalars wrapped so no line passes 78 columns; two short third-person sentences, plain words a non-native reader follows, no idioms, no numbers, no em or en dashes, never the letters "buff" in any word; names 2 to 5 words, Title Case, no digits; `name_simple` a lowercase noun from the name that is not in `reservedNouns` (`internal/baubles/validate.go`) and, for `pocket` and `tiers`, no name word in `notPocketSized` (`internal/baubles/weight.go`) and `weight_lbs` at most 1.0; values whole gold inside the tier with each pool's average near the midpoint (cheap about 3.5, average about 12.5, rare about 120); weights in tenths from the weight scale in Task 10. Rare finds are worth their price: fine materials or real craft, never a plain object. Every description must stay true wherever the object ends up (in a pack, a shop, a chest, sold on): say what it is and what it looks like, never who wants it back, how warm or fresh it is, or anything that hints at a reward for returning it. Check every `name_simple` by grepping `_datafiles/world/dogmud/items` for `^(name|namesimple):.*\b<word>\b` (case-insensitive) and pick another noun on any hit. Make every name different from the nine examples and from each other. Return the path of your draft and a one-line note per pool of anything you were unsure of.

- [ ] **Step 4: Merge (controller).** For each draft in turn, read it in full, then use the Edit tool to add its pools under `entries:` in `_datafiles/world/dogmud/bauble-corpus.yaml` (keep the example entries first in their pools; never a Python read-modify-write). After each merge run:

```bash
cd /c/tmp/dogmud-baubles-c && go test ./internal/baubles/ -run 'TestShippedCorpusSeed$' 2>&1 | tail -20
```

Fix what it reports in the file (a taken keyword, a long line, a value out of range, a duplicate name) before the next merge.

- [ ] **Step 5: All seed tests green.**

```bash
cd /c/tmp/dogmud-baubles-c && go test ./internal/baubles/ -run 'TestShippedCorpusSeed|TestEveryFindableBiome' -v 2>&1 | grep -E "^(--- |ok|FAIL)"
cd /c/tmp/dogmud-baubles-c && git add _datafiles/world/dogmud/bauble-corpus.yaml && go test . -run 'TestEveryTextSurfaceIsRegistered|TestNoStringOrDataSaysBuff' 2>&1 | tail -3
```

Expected: three PASS, then `ok`. (The `git add` first: the word guard reads only tracked files, and the merged seed is not staged yet.)

- [ ] **Step 6: Commit.**

```bash
cd /c/tmp/dogmud-baubles-c && git add _datafiles/world/dogmud/bauble-corpus.yaml internal/baubles/corpus_seed_test.go
git commit -F - <<'EOF'
content(baubles): draft the fallback corpus seed, about 225 entries

Six biome groups and the pocket pool, ten per tier, plus small bare-tier
pools. Drafted per group; awaiting the owner's review before merge.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

- [ ] **Step 7: OWNER REVIEW GATE.** The owner reads `_datafiles/world/dogmud/bauble-corpus.yaml` in full before the PR merges. Apply every change they ask for with the Edit tool, rerun Step 5, and commit with a message naming the review. Record the approval (date and any rulings) in the PR description. The PR is not merged until this step is checked.

---

### Task 12: Docs and config comments

**Files:**
- Modify: `internal/baubles/context.md`, `modules/baubles/context.md`, `internal/actions/context.md`, `modules/context.md`
- Modify: `docs/baubles/implementation-plan.md`, `docs/aicompanion/settings.md`
- Modify: `_datafiles/config.yaml`
- Modify: `docs/README.md`
- Modify: `internal/baubles/find.go`, `internal/baubles/weight.go`, `internal/configs/config.balance.go`, `modules/baubles/baubles.go`, `modules/baubles/generate.go` (Step 9: comments, one log value, one status line)

- [ ] **Step 1: `internal/baubles/context.md`.** With the Edit tool:

Replace the `fallback.go` bullet in `## Files` with:

```markdown
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
```

In the `generate.go` bullet, change "`GenResult`, `Generate`, `RecentNames`." to "`GenResult`, `Generate`, `RecentNames`, `RecentFallbackNames`.".

In the `## API` block, leave the `GenResult` comment alone (slice H already lists `PlayerKey` and `FinderOnly`). The block already declares `Edit` and `ApplyRegenerated` (the latter with H's old three-argument form); replace those two lines

```go
func Edit(id string, field string, value string, admin string) (Record, error) // EditFields
func ApplyRegenerated(id string, res GenResult, admin string) (Record, error)
```

with

```go
func Edit(id string, field string, value string, admin string) (Record, int, error) // EditFields; int: corpus entries removed
func ApplyRegenerated(id string, res GenResult, admin string, randn func(n int) int) (Record, int, error)
```

and append before the closing fence:

```go
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
type CorpusKeyCount struct{ Key string; Seed, Promoted int }
func CorpusKeys() []CorpusKeyCount
type CorpusListing struct{ Seed []CorpusEntry; Promoted []PromotedEntry; Unused map[int]string }
func CorpusList(key string) CorpusListing
func ExportPromoted() (string, error)
```

Append to `## Rules`:

```markdown
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
```

In `## Gotchas`, replace the bullet that starts "**No API key means generic trinkets, always.**" with:

```markdown
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
```

In the `Generate blocks` gotcha, change "all give a generic trinket." to "all fall back through `FallbackFor` (the corpus, else a generic trinket).".

In the pickpocket bullet of `## Rules`, replace the two lines

```markdown
  book...): its text would name that thing, so it is a generic trinket
  instead of a clamped strongbox.
```

with

```markdown
  book...): its text would name that thing, so it falls back to the
  corpus's pocket pool instead of being a clamped strongbox.
```

In `## Consumers` (sweep-era text at `b5ac8b0fa`, keep everything already there): in the `main.go` bullet replace `` - `main.go` (`Load` at boot, `StartSweeper` before Server Ready, `` with `` - `main.go` (`Load` at boot, `LoadCorpus` at boot and data reload, `StartSweeper` before Server Ready, `` and rewrap to 80 columns; in the admin bullet replace `` (`bauble spawn|show|list`; `` with `` (`bauble spawn|show|list|promote|corpus`; ``; and add "`internal/actions/search_bauble.go` and `steal_pocket.go` (`FallbackFor`)." to the actions consumer lines.

- [ ] **Step 2: `modules/baubles/context.md`.** Replace

```
find with, every find is a generic "Trinket" (value and weight random within
the tier). With naming on, each find is named, described, weighed and priced
```

with

```
find with, every find takes its text from the engine's fallback corpus
(`baubles.FallbackFor`; a generic "Trinket" only when nothing fits). With
naming on, each find is named, described, weighed and priced
```

Replace `5. Neither route: `errNoRoute`, a generic trinket.` with `5. Neither route: `errNoRoute`; the engine falls back to the corpus.`, and `` `ApplyLimits`; any failure anywhere is a generic trinket. `` with `` `ApplyLimits`; any failure anywhere falls back to the corpus. ``

- [ ] **Step 3: `internal/actions/context.md`.** (A " / " below separates two consecutive lines of the file; rewrap any line the change pushes past 80 columns.) Replace `then given up (a generic` / `     trinket).` (the two lines in the pickpocket paragraph) with `then given up (from the` / `     fallback corpus).`; replace `goroutine. That goroutine names it with `baubles.Generate` (the model, or a` / `generic trinket) WITHOUT the mud lock` with `goroutine. That goroutine names it with `baubles.Generate` (the model, or the` / `fallback corpus) WITHOUT the mud lock`; replace `named if its naming came back, otherwise the generic trinket it would have` / `been.` with `named if its naming came back, otherwise the corpus fallback it would have` / `been (`baubles.FallbackFor`).`; and replace `The minimum wait applies to generic trinkets too` with `The minimum wait applies to corpus and generic finds too`.

- [ ] **Step 4: `docs/baubles/implementation-plan.md`.** The file already has `### Phase 6c: Stolen goods, fences and owners (written, after PR #175)` (line 739 at `b5ac8b0fa`) and `### Phase 6d: The owner's fix round on PR #175 (written)` (line 879), and no `Phase 6: Theft readiness` heading, so this section is Phase 6e. Confirm where it goes:

```bash
cd /c/tmp/dogmud-baubles-c && grep -n "^### Phase 6\|^### Phase 7" docs/baubles/implementation-plan.md
```

Expected: `### Phase 6a`, `6b`, `6c` and `6d` headings, then `### Phase 7: Optional` (line 931 at `b5ac8b0fa`), and no `Phase 6e` yet. Insert directly before the `### Phase 7: Optional` heading:

```markdown
### Phase 6e: Fallback corpus (hardening slice C) (written)

Design: `docs/superpowers/specs/2026-09-28-baubles-hardening-and-corpus-design.md`,
slice C. Plan: `docs/superpowers/plans/2026-09-28-slice-c-bauble-corpus.md`.

- **No key no longer means "Trinket".** A find no model names takes
  hand-written text from the fallback corpus, chosen by where it was found:
  the biome and its group (dwelling, street, underground, ruins, waterside,
  wild) and the tier, `pocket` for pickpocketed finds, then the tier alone.
  A generic "Trinket" is left only for when nothing fits.
- Two layers: the tracked seed `_datafiles/world/dogmud/bauble-corpus.yaml`
  (about 225 entries, reviewed by the owner) and the living-state overlay
  `_datafiles/world/dogmud/baubles/corpus.promoted.yaml`, filled by
  `bauble promote <bauble>` from moderated model names made on the
  server's key and never edited by hand. Retiring, editing or regenerating
  a record takes its promoted text out again.
- Values are clamped into the tier when used; each seed pool's average sits
  near its tier's midpoint, so a corpus find pays what a generic one did.
  A known name hinting at its price is accepted (owner ruling 8).
- Admin: `bauble promote`, `bauble corpus list|remove|reload|export`. The
  logic is `baubles.Promote` and `baubles.RemoveCorpusEntry`, so the /build
  queue (web builder rework arc) can call the same functions.
```

- [ ] **Step 5: `_datafiles/config.yaml` comments, from the HEAD blob.**

```bash
cd /c/tmp/dogmud-baubles-c && git ls-files -v _datafiles/config.yaml
cd /c/tmp/dogmud-baubles-c && git diff --stat HEAD -- _datafiles/config.yaml
cd /c/tmp/dogmud-baubles-c && grep -n -i "generic" _datafiles/config.yaml
```

Expected: `H _datafiles/config.yaml`, no diff output, and the stale comments listed by the third command. If the diff is not empty, STOP and ask the controller: the disk copy has drifted from the blob and a commit would carry it. At `b5ac8b0fa` the bauble comments that say generic are lines 1485, 2692, 2701, 2705 and 2707-2708 (the text below is the merged text, re-read 2026-09-29). Edit only bauble comments (the grep also finds unrelated hits such as "Generic melee never interrupts a cast"). Line 2717's `a plain "Trinket" to everyone else` is the finder-only view and stays. With the Edit tool, one replacement each:

1. `  # is given up, and the bauble is a generic trinket.` becomes `  # is given up, and the bauble's text comes from the fallback corpus.`
2. Slice H's line (the second of the pair; the first, `  # else through the server's key (APIFramework: its one daily budget and`, stays)

```yaml
  # breaker, shared with the AI companion), else it is a generic "Trinket".
```

becomes

```yaml
  # breaker, shared with the AI companion), else its text comes from the
  # fallback corpus (bauble-corpus.yaml and promoted names).
```

3. `    # keep it short. Failures and timeouts become generic trinkets.` becomes `    # keep it short. Failures and timeouts fall back to the corpus.`
4. The `MaxConcurrent: 4` line, in H's form (S5's alternative form did not land): `    MaxConcurrent: 4           # server-key calls at once; more are generic` becomes `    MaxConcurrent: 4           # server-key calls at once; more use the corpus`.
5. S5's lines (S5 #193 reworded the allowance sentence: `bauble spawn` now charges the admin's own allowance, `bauble regen` nobody; only the first two lines change)

```yaml
    # or their own (about 8 to 10 names). Over it, a find is a generic
    # trinket. `bauble spawn` charges the admin's own allowance, as a find
```

become

```yaml
    # or their own (about 8 to 10 names). Over it, a find falls back to the
    # corpus. `bauble spawn` charges the admin's own allowance, as a find
```

Then:

```bash
cd /c/tmp/dogmud-baubles-c && git diff HEAD -- _datafiles/config.yaml | grep -c "^[-+] "
cd /c/tmp/dogmud-baubles-c && grep -n -i "generic" _datafiles/config.yaml
```

Expected: `13` (six lines out, seven in), and no bauble comment left in the second list. Any other count means an unintended change or merged text that differs from the above; inspect with `git diff` and account for every line in the task report.

- [ ] **Step 6: `docs/README.md`.** In the `## Reference` table, after the `worldbuilding/` row, add:

```markdown
| [`../_datafiles/world/dogmud/bauble-corpus.yaml`](../_datafiles/world/dogmud/bauble-corpus.yaml) | The bauble fallback corpus seed: hand-written bauble text by biome group (dwelling, street, underground, ruins, waterside, wild), pocket and tier, used when no model names a find. Checked by `TestShippedCorpusSeed`; promoted model names live beside the catalog in `baubles/corpus.promoted.yaml` (living state, gitignored) |
```

This plan gets no row here: it reaches master, with its row, through the separate docs-only PR (owner ruling 11).

In the `baubles/implementation-plan.md` row (line 196 at `b5ac8b0fa`, which already names Phase 6d, the owner's fix round), replace `(a generic "Trinket" when no API key is set)` with `(hand-written text from a fallback corpus when no API key is set)`, and replace `fences as real shopkeepers),` with `fences as real shopkeepers), the fallback corpus of hand-written bauble text by biome group, with admin promotion of good model names (Phase 6e),`.

- [ ] **Step 7: Audit context.md symbols.**

```bash
cd /c/tmp/dogmud-baubles-c && python tools/context_md_audit.py 2>&1 | tail -10
```

Expected: no phantom symbol reported for `internal/baubles`, `internal/actions` or `modules/baubles`.

- [ ] **Step 8: `docs/aicompanion/settings.md` and `modules/context.md`.** In `settings.md`'s bauble paragraph (as slice H rewrote it), the sentence reads `Unticked, their finds use the server's key, or stay generic trinkets when there is none.` across two lines (at `b5ac8b0fa` the break falls after `use the`, line 357; `or stay generic trinkets when there is none.` is on line 358). Replace `or stay generic trinkets when there is none.` with `or take hand-written text from the fallback corpus when there is none.`, then rewrap that paragraph's lines to 80 columns or less. The paragraph's earlier `everyone else sees a plain "Trinket"` (line 353) is the finder-only view and stays. In `modules/context.md`'s `baubles` row, replace `without a key every find is a generic trinket |` with `without a key every find takes hand-written text from the engine's fallback corpus |`.

- [ ] **Step 9: The code comments, log value and status line that still say "generic trinket".** None of these changes behaviour. With the Edit tool, one replacement each:

1. `internal/baubles/find.go` (`RevealDelay`'s doc comment): `// named by the model and a generic trinket arrive at the same pace.` becomes `// named by the model and a fallback one arrive at the same pace.`
2. `internal/configs/config.balance.go` (the `BaublePickpocketGraceSecs` field comment): `waited for before it is a generic trinket (default 5)` becomes `waited for before it falls back to the corpus (default 5)`. The trailing comment's width does not change gofmt's alignment of the field block.
3. `modules/baubles/baubles.go`, the package comment:

```go
// Off by default. Switched off, it installs nothing and every bauble is a
// generic trinket. Switched on, a find is named through the finder's own
// key when they allowed it, else the server's key, else it is a generic
// trinket.
```

becomes

```go
// Off by default. Switched off, it installs nothing and every bauble takes
// its text from the engine's fallback corpus. Switched on, a find is named
// through the finder's own key when they allowed it, else the server's key,
// else it falls back to the corpus (baubles.FallbackFor).
```

4. `modules/baubles/baubles.go`, the switched-off log: `` `naming`, `generic trinkets`, `reason`, `Modules.baubles.Enabled is false` `` becomes `` `naming`, `fallback corpus`, `reason`, `Modules.baubles.Enabled is false` ``.
5. `modules/baubles/baubles.go` `info`, the status detail as H2 rewrote it (line 199 at `b5ac8b0fa`): `` ` No server key: finds named on a finder's own key are shown to that finder alone (nothing can moderate them); every other find is a generic trinket.` `` becomes `` ` No server key: finds named on a finder's own key are shown to that finder alone (nothing can moderate them); every other find comes from the fallback corpus.` `` (`bauble status` shows it). Key the Edit on `every other find is a generic trinket.`.
6. `modules/baubles/baubles.go`, slice H's `takeServerSlot` comment:

```go
// reports none free. A find beyond them is not queued: it is a generic
// trinket.
```

becomes

```go
// reports none free. A find beyond them is not queued: it falls back to
// the corpus.
```

7. `modules/baubles/generate.go`, slice H's `moderate` policy comment: `//   - A flag always keeps the text out of the world: a generic trinket.` becomes `//   - A flag always keeps the text out of the world: a corpus fallback.`
8. `internal/baubles/weight.go` (`TooBigFor`'s doc comment): `// it (a generic, small trinket) rather than only clamping the number.` becomes `// it (a fallback from the corpus's pocket pool) rather than only clamping the number.`

Then confirm nothing that says a find "is a generic trinket" is left outside tests and `fallback.go` (the corpus's own last resort legitimately says so), and that the packages still build and pass:

```bash
cd /c/tmp/dogmud-baubles-c && grep -rn -i "generic trinket" --include=*.go internal modules | grep -v _test.go
cd /c/tmp/dogmud-baubles-c && grep -rn -i "generic trinket" --include=*.md internal modules docs/README.md docs/aicompanion
cd /c/tmp/dogmud-baubles-c && go build ./... && go test ./modules/baubles/ ./internal/configs/ 2>&1 | tail -3
```

Expected: the first two lists hold only lines that describe the last resort when the corpus has nothing (`fallback.go`, `corpus.go`, `record.go`'s new `StatusFallback` comment, and the corpus rules and gotchas written in Steps 1 and 3), plus the finder-only view that H2 added, which stays generic because this plan does not change what others see of a finder-only record: `generate.go` (the `GenResult.FinderOnly` comment), `record.go` (`View`), `modules/baubles/generate.go` (the `finderOnly` policy line), `internal/items/bauble_viewer.go`, `internal/mobs/mobs.go` (the bare carrier), `internal/baubles/context.md` (the viewer-agnostic gotcha), `internal/items/context.md` and `modules/baubles/context.md` (`FinderOnly`). Read each and rewrite any other. Then `ok` twice.

- [ ] **Step 10: Commit.**

```bash
cd /c/tmp/dogmud-baubles-c && git add internal/baubles/context.md modules/baubles/context.md internal/actions/context.md modules/context.md docs/baubles/implementation-plan.md docs/aicompanion/settings.md _datafiles/config.yaml docs/README.md internal/baubles/find.go internal/baubles/weight.go internal/configs/config.balance.go modules/baubles/baubles.go modules/baubles/generate.go
git commit -F - <<'EOF'
docs(baubles): the fallback corpus in context.md, the phase doc and config

Every comment, doc line, log value and status line that still said a find
with no model is a generic trinket now names the corpus.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 13: Adversarial playtest (controller)

**Files:** none committed. Player-facing content ends with an in-game adversarial review (`dogmud-authoring-content`, "The playtest gate").

- [ ] **Step 1: Load `dogmud-playtesting`** and follow it for the harness location, the port, and never killing the owner's server.

- [ ] **Step 2: A throwaway checkout with finds switched on.**

```bash
cd /c/tmp/dogmud-baubles-c && git worktree add --detach C:/tmp/dogmud-baubles-c-playtest HEAD
```

In `C:/tmp/dogmud-baubles-c-playtest/_datafiles/config.yaml` (never committed) set, with the Edit tool: `BaublesEnabled: true`, `BaubleSearchChancePct: 100`, every non-zero value under `BaubleBiomeChancePct` to `100`, `BaubleRollsPerWindow: 20`, `BaublePickpocketChancePct: 100`. Leave `Modules.baubles.Enabled: false`, so every find is a corpus find.

- [ ] **Step 3: Goals file** at `C:/tmp/dogmud-baubles-c-playtest/tools/playtest/goals/2026-09-28-bauble-corpus.yaml`:

```yaml
# Bauble fallback corpus playtest. Finds are switched on at a very high
# rate in this checkout and no model is set up, so EVERY find is drawn
# from the hand-written corpus. You are here to read them critically.
ephemeral:
  profile: fresh
  # 462 is Thornwall City, past the Awakening Rite gate, with people to
  # rob. 5200 cannot walk until the rite is done.
  start_room: 462
  budgets:
    wall_clock: 40m

goals:
  - >-
    Search in as many kinds of place as you can reach: inside houses and
    shops, on main streets and back alleys, in sewers or caves, in ruins,
    on roads and shores, and out in the wild. Search each place several
    times. For every find, write down where you were, its name, and look
    at it. Report any name or description that does not fit where it was
    found, that repeats too soon, that is hard to understand, that shows a
    number, or that reads like a usable item (a weapon, food, a key).
  - >-
    Pickpocket several townspeople with steal. Report every bauble you get:
    does it sound like something a person carries in a pocket? Report
    anything too big for a pocket.
  - >-
    Appraise and sell your finds at a general store. Report whether the
    price matches how the thing is described, and whether cheap-sounding
    things sell for a lot or fine-sounding things for a little.
  - >-
    Use each find's keyword (get, drop, look) as a player would. Report
    any keyword that picks up the wrong thing or does not work.
```

- [ ] **Step 4: Run** (the skill gives the exact command form):

```
/playtest local --checkout C:/tmp/dogmud-baubles-c-playtest bug-finder 2026-09-28-bauble-corpus.yaml
```

- [ ] **Step 5: Triage.** Read the whole report. Fix content defects in `C:\tmp\dogmud-baubles-c\_datafiles\world\dogmud\bauble-corpus.yaml` with the Edit tool (rerun the Task 11 Step 5 tests after each batch) and code defects with a failing test first. Re-run the playtest if anything beyond wording changed. Extract the findings to memory (reports are gitignored), then remove the throwaway worktree after diffing it:

```bash
cd /c/tmp/dogmud-baubles-c-playtest && git status --short
cd /c/tmp/dogmud-baubles-c && git worktree remove --force C:/tmp/dogmud-baubles-c-playtest
```

Expected before removal: only `_datafiles/config.yaml` and the goals file differ.

- [ ] **Step 6: Commit content fixes** (if any):

```bash
cd /c/tmp/dogmud-baubles-c && git add _datafiles/world/dogmud/bauble-corpus.yaml
git commit -F - <<'EOF'
content(baubles): corpus fixes from the adversarial playtest

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 14: Final gate

**Files:** none, unless a check fails.

Every diff below is taken from `$BASE`, the master commit this branch was cut from (Task 0 Step 2). Each command block sets it with `BASE=b5ac8b0fa`, the sha Task 0 recorded (never `git merge-base HEAD master`: the main checkout's local `master` is stale at `62f9027a2`, so that would give the wrong commit).

- [ ] **Step 1: The branch is ours alone and still fits CI's lint.** Nothing of FinalTwist's branch is in it, and the size stays under the lint inversion (owner ruling 11: under 20k lines and 300 files):

```bash
cd /c/tmp/dogmud-baubles-c && BASE=b5ac8b0fa && echo "BASE=$BASE" && git log --oneline $BASE..HEAD
cd /c/tmp/dogmud-baubles-c && BASE=b5ac8b0fa && git diff --shortstat $BASE..HEAD
cd /c/tmp/dogmud-baubles-c && BASE=b5ac8b0fa && git diff --name-only $BASE..HEAD -- docs/superpowers
```

Expected: `BASE` is the sha Task 0 recorded; the log lists only this plan's commits; the shortstat shows fewer than 300 files changed and fewer than 20000 insertions plus deletions (about 30 files and 3000 lines is the expected size); the last command prints nothing (the spec and plans ship in the docs-only PR, never here).

- [ ] **Step 2: gofmt, on the committed blobs.** Windows `gofmt -l` false-positives on a CRLF working copy of an LF blob, so check what git holds for every Go file the branch added or changed:

```bash
cd /c/tmp/dogmud-baubles-c && BASE=b5ac8b0fa && for f in $(git diff --name-only --diff-filter=AM $BASE..HEAD -- '*.go'); do out=$(git show HEAD:$f | gofmt -l); [ -n "$out" ] && echo "UNFORMATTED: $f"; done; echo done
```

Expected: only `done`.

- [ ] **Step 3: build, vet and lint** (the lint is `dogmud-shipping`'s pre-push step 3, the only local gate that sees what CI's lint sees):

```bash
cd /c/tmp/dogmud-baubles-c && go build ./... && go vet ./internal/items/... ./internal/apiframework/... ./internal/baubles/... ./internal/actions/... ./internal/usercommands/... ./internal/configs/... ./modules/baubles/... . 2>&1 | tail -5
cd /c/tmp/dogmud-baubles-c && git fetch origin master && ~/go/bin/golangci-lint run --new-from-merge-base=origin/master 2>&1 | tail -15
```

Expected: no output from the first; `0 issues.` from the second. A finding on a line this branch did not write is still ours to clear (check `git log -L` first, per `dogmud-shipping`).

- [ ] **Step 4: Tests for every touched package, and the repo root.**

```bash
cd /c/tmp/dogmud-baubles-c && go test ./internal/items/... ./internal/apiframework/... ./internal/baubles/... ./internal/actions/... ./internal/usercommands/... ./internal/configs/... ./modules/baubles/... 2>&1 | tail -10
cd /c/tmp/dogmud-baubles-c && go test . 2>&1 | tail -3
cd /c/tmp/dogmud-baubles-c && go test . -run 'TestEveryTextSurfaceIsRegistered|TestNoStringOrDataSaysBuff|TestLivingStateWritesAreDurable|TestNoHandRolledTempRename' -v 2>&1 | grep -E "^(--- |ok|FAIL)"
```

Expected: `ok` for each package; `ok` for the root; four `--- PASS` for the named root guards (text-surface registry, word guard, durable writes, no hand-rolled temp rename). `internal/items` and `internal/apiframework` are not edited here, but the corpus leans on slice H's `AuthoredName` and sits beside S5's ledger, so both are run.

- [ ] **Step 5: Boot smoke: the corpus loads at boot.** Per `dogmud-shipping` ("Boot the server and confirm Server Ready"), in a detached worktree, built to a fixed path, never `go run`, and never killing any server by name or port (the `timeout` ends ours). Run each line on its own:

```bash
cd /c/tmp/dogmud-baubles-c && git worktree list
cd /c/tmp/dogmud-baubles-c && git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp /c/tmp/dogmud-baubles-c/_datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd /c/tmp/dogmud-boot-check && go build -o boot-check.exe .
cd /c/tmp/dogmud-boot-check && timeout 180 ./boot-check.exe > boot.log 2>&1; echo "exit=$?"
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" /c/tmp/dogmud-boot-check/boot.log
grep -c "Server Ready" /c/tmp/dogmud-boot-check/boot.log
grep -n "LoadCorpus" /c/tmp/dogmud-boot-check/boot.log
cd /c/tmp/dogmud-baubles-c && git worktree remove --force C:/tmp/dogmud-boot-check
```

Expected: `C:/tmp/dogmud-boot-check` absent from the first list (if it is there, another session owns it: wait, never remove it); `exit=124` (the server stayed up until the timeout); `0`; `1`; one `baubles.LoadCorpus()` line with `seed` about 225, `promoted` 0 and `skipped` 0, and no `baubles.LoadCorpus` ERROR or `skipped` WARN line. If `Logging.LogToFile` sends the log to a file instead, grep that file for the same lines. If Windows holds a lock on removal, `rm -rf C:/tmp/dogmud-boot-check` then `git worktree prune`.

- [ ] **Step 6: Structural checks** (run each on its own; a `grep` that finds nothing exits 1):

```bash
cd /c/tmp/dogmud-baubles-c && grep -rn "GenericTrinket(" --include=*.go . | grep -v _test.go
cd /c/tmp/dogmud-baubles-c && grep -n -i "prune" internal/baubles/*.go | grep -v "_test.go" | grep -v "window.go"
cd /c/tmp/dogmud-baubles-c && grep -n "PRUNE:" internal/baubles/corpus_admin_test.go
cd /c/tmp/dogmud-baubles-c && grep -n -A45 "func TestOverlaySurvivesTheCatalog" internal/baubles/corpus_admin_test.go | grep -i "prune"
cd /c/tmp/dogmud-baubles-c && grep -nP "\x{2013}|\x{2014}" _datafiles/world/dogmud/bauble-corpus.yaml internal/baubles/context.md docs/baubles/implementation-plan.md _datafiles/world/dogmud/templates/admincommands/help/command.bauble.template
```

Expected: (1) exactly `fallback.go` and `corpus.go`. (2) at least one catalog prune function (read the hits; `window.go`'s search-window sweep is excluded). (3) nothing. (4) the call to that prune inside `TestOverlaySurvivesTheCatalog`. **If (2) finds no catalog prune, or (3) finds the `PRUNE:` line, or (4) finds no call, the gate FAILS**: report it to the controller and do not open the PR; the overlay's survival of the prune is a spec requirement, and a test that never runs the prune cannot show it. (5) no dashes in any line this plan added (compare hits against `git diff` if the files had dashes before).

- [ ] **Step 7: Docs are complete.** Confirm each is in the branch diff:

```bash
cd /c/tmp/dogmud-baubles-c && BASE=b5ac8b0fa && git diff --stat $BASE..HEAD -- internal/baubles/context.md modules/baubles/context.md internal/actions/context.md modules/context.md docs/baubles/implementation-plan.md docs/aicompanion/settings.md docs/README.md _datafiles/world/dogmud/templates/admincommands/help/command.bauble.template _datafiles/config.yaml
```

Expected: all nine files listed. `docs/README.md` carries the seed row and the Phase 6e wording, and no row for this plan (that row ships in the docs-only PR).

- [ ] **Step 8: Owner review gate.** Task 11 Step 7 is checked, with the owner's approval recorded in the PR description. If not, the branch does not merge.

- [ ] **Step 9: Hand off.** Push `feature/bauble-corpus` and open its PR against `master` following `dogmud-shipping` (every `gh` command carries `--repo pruuk/DOGMud`; never push to or rebase onto FinalTwist's branch). The owner runs deploys; this plan deploys nothing.

---

## Self-review

- **Spec coverage.** Seam (`Fallback` with the spec's signature, four call sites, `Generator: corpus`, `Model: "corpus:<key>"`, `Moderated`/`PlayerKey` false, `Mint` ready, record comment, stats line, pickpocket log, no migration): Tasks 1, 4, 5, 6, 9. Files (seed with `groups`/`entries`, overlay with provenance, ignore rules, prune survival test): Tasks 3, 7, 10. Loading and concurrency (after items, boot and reload, atomic snapshot, writers build/save/swap, seed ERROR without crash, overlay living-state contract, quarantine restarts empty, every entry through `CleanReply` and weight bounds, CI loads items and asserts the snapshot): Tasks 3, 7, 8, 10. Lookup (merge, empty or unmapped biome, pocket filter and empty-after-filter, recency sibling and least-recent, value clamp, weight limit): Task 4. Promotion and admin (refusals, sold promotable, exact key, `Edit` sets `HandEdited` and keeps `Moderated` (ruling 6), `Retire` removes, four `corpus` subcommands admin-only, logic in `Promote`/`RemoveCorpusEntry`): Tasks 1, 7, 9. Seed content (player-copy rules, pocket entries pass `TooBigFor`, pool means tested, owner review): Tasks 10, 11. Docs share: Task 12.
- **Requested tests, each mapped:** fallback order and merge (`TestFallbackMergesBiomeAndGroupThenTier`); empty biome (same, biomes `""` and `water`); pocket filter and empty-after-filter (`TestFallbackPocketFindsFitAPocket`); recent-avoidance with an all-recent pool (`TestFallbackAvoidsRecentNamesWithoutFallingThrough`); value clamp (`TestFallbackClampsTheValueIntoTheTier`); pool mean near midpoint (`TestShippedCorpusSeedIsComplete`); corrupt overlay quarantined (`TestCorruptOverlayIsQuarantined`); promotion refusals and sold-is-promotable (`TestPromoteRefusals`, `TestPromoteASoldFindAndUseIt`); retire removes overlay entries (`TestRetireRemovesItsCorpusEntry`); overlay survives the catalog, prune included (`TestOverlaySurvivesTheCatalog`); CI seed validation with a non-empty snapshot (`TestShippedCorpusSeed`); every findable biome maps to a group (`TestEveryFindableBiomeHasACorpusGroup`).
- **Plan-review findings, each mapped.** Delivery from fresh master with `$BASE`, anchors re-checked, size and lint gate, boot smoke: Tasks 0 and 14. Phase 6e (6d is the owner's fix round) before Phase 7 and the seed header: Tasks 10 and 12. `HandEdited` as the promotion test (regenerated and restored records promotable, hand-edited not): Task 1 (`TestEditKeepsModeratedAndMarksHandEdited` since ruling 6, `TestOnlyEditMarksHandEdited`), Task 7 (`TestPromoteLooksAtHandEditedNotEditedBy`). Edit and regen drop promoted entries and count them: Task 7 (`TestEditAndRegenRemoveTheRecordsCorpusEntry`). Stale "generic trinket" wording: Tasks 5, 6, 9 and 12 Steps 1, 5, 6, 8 and 9. A reload keeps the seed in use and a broken overlay refuses writes: Task 3 (`TestReloadWithABrokenSeedKeepsTheSeedInUse`, `TestAnOverlayThatCannotBeQuarantinedIsBroken`), Task 7 (`TestABrokenOverlayRefusesEveryWrite`). Durable-write guard named and the seed staged before the word guard: Tasks 10 and 11. Promotion refuses a name already in its merged pool: Task 7 (`TestPromoteRefusals`, `TestPromoteASoldFindAndUseIt`). Seed examples that stay true after sale and promise no reward: Task 10, and the drafting prompt in Task 11. Config line numbers at `b5ac8b0fa` (first `e711ee9de`): facts table and Task 10. `TestApplyRegeneratedRefusesACorpusAnswer` marked a guard; removal by name or record id (`TestRemoveCorpusEntry`, `TestRemoveCorpusEntryRefusesAnAmbiguousName`); admin output asserted per subcommand (`TestAdminBauble_PromoteAndCorpus`): Tasks 1, 7 and 9. No README row for this plan: Task 12 Step 6.
- **Type consistency.** `Fallback`, `FallbackFor`, `RecentFallbackNames`, `LoadCorpusFrom`, `ReloadCorpus`, `ClearCorpusForTest`, `CorpusCounts`, `CorpusList`, `CorpusKeys`, `Promote`, `RemoveCorpusEntry(key, which string)`, `ExportPromoted`, `GroupOf`, `Generator.Named`, `Record.HandEdited`, `Edit` and `ApplyRegenerated` returning `(Record, int, error)`, `CorpusReport.SeedKept` and `.OverlayBroken`, `ErrOverlayBroken`, `ErrCorpusCleanup` and the `ErrPromote*` names are used with the signatures defined in Tasks 1 to 7 throughout Tasks 8 to 12.
