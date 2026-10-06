# internal/narration

## Purpose

Owns two things, both on the RENDERING side of narration:

1. The seam that CHOOSES a line from a pool (`Picker`), so a snapshot run can
   be deterministic without touching global randomness.
2. The core that renders one event for its audiences from a SINGLE coordinated
   variant index (`Render`), so the room never sees a different event than the
   participants did.

It does not deliver anything. Delivery is `internal/messaging`, which this
package must never import (see Gotchas).

## Files

| File | Holds |
|---|---|
| `picker.go` | `Picker`, `DefaultPicker`, `SequencePicker`, `FirstPicker`. |
| `render.go` | `Selector`, `Variants`, `Roles`, `Render`, `ValidateVariants`, `Substitute`, `Role` and the four canonical token constants. The core. |
| `picker_test.go` | Unit tests for the two pickers. |
| `render_test.go` | Unit tests for the core, including the coordinated-index probe. |
| `snapshot_test.go` | The M1 harness: golden snapshots of the message stores, raw and post-pipeline. |
| `testdata/stores/` | Those goldens: 17 files, one per store plus the post-pipeline snapshot (`light_notices.golden`, lighting plan 3d, is the most recent addition; see the correction under `position_control.golden` below). |

## Public API

```go
// Picker chooses an index in [0,n).
type Picker func(n int) int
func DefaultPicker(n int) int
func SequencePicker() Picker
func FirstPicker(n int) int

// Selector names which variant group an event draws from.
type Selector string

// Variants holds one candidate list per role; Roles is the rendered result.
type Variants struct{ Actor, Actee, Observer, ActeeObserver []string }
type Roles    struct{ Actor, Actee, Observer, ActeeObserver string }

// Role names one audience, for stores that declare which roles they author.
type Role uint8
const (RoleActor Role = iota; RoleActee; RoleObserver; RoleActeeObserver)

func (v Variants) Len() int
func Render(v Variants, tokens map[string]string, pick Picker, indexOverride ...int) Roles
func ValidateVariants(v Variants, minVariants int, expected ...Role) error

// Substitute is the one token substitution engine (messaging arc M4a). It
// replaces every token in one pass, so a substituted value that happens to
// contain a token spelling is not substituted again.
func Substitute(s string, tokens map[string]string) string

// The canonical name-token vocabulary. Every store's shipped YAML spells
// names these four ways and no other; event tokens stay store-specific.
const (
	TokenActor      = "{actor}"
	TokenActee      = "{actee}"
	TokenActorPlain = "{actor_plain}"
	TokenActeePlain = "{actee_plain}"
)
```

A nil `Picker` means production behaviour: stores that accept one treat nil as
`DefaultPicker` rather than panicking.

`internal/items` keeps a typed mirror of the two name tokens
(`items.TokenActor`, `items.TokenActee`) for its `map[TokenName]string` token
maps. They are DEFINED FROM the constants above rather than respelled, so the
two vocabularies cannot drift; `items.TokenStrings` converts such a map to the
plain-string form `Render` and `Substitute` take.

## The assembly rule

**The store assembles the pools and computes the selector. The core only
coordinates the index and substitutes tokens.**

The core deliberately knows nothing about bands, skill tiers or cooldowns.
`items.SkillTieredMessages.PoolFor` UNIONS beginner, expert and
master by skill level, and `grapplemessaging.PickTemplate` FILTERS
recently-used templates out; both hand `Render` an already-assembled
`[]string`. Pulling either into the core would turn the messaging arc's M4
parameter flip into a core rewrite.

The same rule is why banding stays in the stores, not the core: it is
`items.RenderDefenseMessage` that owns the band decision (crit plus
normalized margin), for melee and every other channel alike. It did not
always: melee used to band separately, on `items.GetDefenseMessage`'s
self-relative z-score, and the two disagreed deliberately until M4c
reconciled them onto one function and one config knob, which was a
behaviour change and belonged to M4, not to this package.

## Consumers

`internal/items` (defence and combat-message stores), `internal/behaviortree` (an item tree's speech pools, through the `speak` node; item behaviour slice 2 retired `internal/itemvoices`),
`internal/spells` (casting), `internal/combat` (taunt),
`internal/grapplemessaging`, `internal/gossip` (gossip template pools, single
role), `internal/movenarration` (special moves), `internal/lightnotice`
(lighting plan 3d's transition notices, single role, `line()` calls
`narration.Render` directly with a nil picker). Any store that wants
deterministic selection under snapshot, or coordinated multi-role
rendering. `internal/textutil` (the door for the condition, spell, quest
and crafting stores, which do not call `Render` themselves).

## The two-tier loader policy (messaging M4b-1)

Every narration store sits in one of two tiers, and the rule that decides is
**what silence costs the player**.

**Event tier: bad data FAILS THE BOOT.** These stores narrate something that is
happening to the player right now, so a store that loads empty does not read as
"quiet", it reads as the game not telling them what just hit them. A silent
fight, a silent grapple or a silent submission misleads a player mid-action,
and the operator sees nothing but a log line nobody is watching. Members:
combat messages, defence, taunt, grapple outcomes, spells, conditions, quests,
crafting, casting, item tree speech (`behaviortree.ValidateItemBehaviors`), position_control. Each panics from its loader,
and each loader is CALLED FROM `main.go` so that "fails at boot" means the boot,
not the first cast: a check that runs inside a package `init()` or inside a
`sync.Once` a test may already have spent is not a boot check. `hooks.LoadGrappleMessaging`
and `hooks.LoadPositionMessages` both spend their store's `sync.Once` rather
than consulting it, for exactly that reason.

**Ambient tier: silence is tolerated.** Weather, tips and gossip. A missing
ambient line costs nothing: the room is simply not described as rainy this tick,
and no player is left without information they were acting on. The tolerance is
not identical across the three, and the difference is worth knowing before
changing any of them:

- **Weather fails soft on BAD data.** `loadContent` logs a warning and runs with
  whatever tables loaded before the bad file, which is silence for the rest.
  Its loader carries an explicit "Do not 'fix' this into a panic or a hard fail"
  comment (`modules/weather/content/emotes.go`) and it stays.
- **Tips and gossip tolerate an ABSENT file only.** `tips.Load` and
  `gossip.Load` return quietly when the world ships no `tips.yaml` or
  `gossip_templates.yaml`, which is what lets a world without them boot, but a
  file that IS present and fails to parse or validate still panics.

Weather's only net is therefore a BUILD-time one:
`shipped_narration_data_guard_test.go` at the repo root loads all fourteen
stores from `_datafiles/world/dogmud` and fails the build on bad data. That
guard covers the event tier too, where it is a nicety (it names the offending
record instead of handing an operator a stack trace) rather than the only thing
standing there.

## Gotchas

**`Render` takes ONE index for ALL roles, and that is the whole point.**
Variant N of each role describes the same moment, so picking per role gives
every audience a coherent-looking line about a DIFFERENT event. That defect has
shipped twice in this codebase: melee defence (PR #112) and taunt (PR #115).
`TestRenderCoordinatesOneIndexAcrossRoles` guards it, and was verified red
against a per-role sabotage before being trusted.

**An index override still CONSUMES a pick.** `Render` calls `pick(n)` and then
discards the draw when an override is supplied. This is bug-compatibility with
the defence store's original behaviour, and it matters because `DefaultPicker`
routes through `util.Rand`, which is global engine randomness: returning early
would consume one fewer random number and shift every subsequent draw in the
process.

**Pass `expected` to `ValidateVariants` if your store has more than one role.**
Without it the function cannot tell a role that is deliberately absent (a condition
has no actee) from one that went MISSING (a defence band that lost its observer
pool), because both look like an empty slice. A three-role store that omits it
can boot happily while narrating a real event to two audiences and silence to
the third, which is the exact defect this package exists to prevent. A blind
adversarial review found this gap in the core's first version; naming the roles
turns it into a boot failure.

**Unequal role pools render NOTHING.** `Variants.Len()` returns 0 when the
non-empty roles disagree, because index N cannot mean the same moment in a
five-entry pool and a three-entry one. Call `ValidateVariants` at LOAD time so
this surfaces as a boot failure rather than as silence in play. The taunt store
shipped an 8/8/6 band for exactly this reason: nothing checked it.

**`SequencePicker` returns a CLOSURE, and is not a seed. That is the whole
point.** Seeding global randomness would be process-wide, and this repo runs
every test in a package in ONE binary, so a seeded snapshot's output would
depend on which tests ran before it. Relative state that passes or fails by
ORDER is a trap this codebase has already been bitten by. Each caller gets its
own counter instead.

**This package can never import `internal/messaging`.** `internal/items`
imports `narration` (for the defence store's picker), and `messaging` reaches
`items` through `characters`, so the edge would close a cycle. This is why the
M2 narration seam (`SendTrio`) lives in `messaging` and not here, even though
the name would suggest otherwise: rendering-side selection lives low in the
graph, delivery lives high.

**The goldens under `testdata/stores/` cover message STORES only.** They do not
cover hand-rolled `fmt.Sprintf` text in command files. M2 needed two separate
guards at the repo root for that (`TestM2LiteralsAreFrozen` and
`TestM2RoutingIsFrozen`), and the gap was not obvious: a store golden freezing
`RenderDefenseMessage` proved nothing about `GetDefenseMessage`, which is how
the melee defence triad bug survived M1.

**`defense_messages.golden` keys its rows by the AUTHORED role name**
(`actee`, `actor`, `observer`, spelled `todefender`/`toattacker`/`toroom` until
M4b-1). That is deliberate and load-bearing: it makes the golden able to catch
a swap of which authored pool lands in which role, which is the mistake this
core makes easiest to introduce.

The rename made the authored spellings and the core's role names COINCIDE, and
the property survives that only because of how the row is built: the builder
pairs a hardcoded label with a GO FIELD (`triad.ToDefender`), and the field's
`yaml:` tag is exactly what a swap would corrupt. Swapping the `actor` and
`actee` tags on `DefenseTogetherMessages` was verified to turn this golden red
(M4b-1 task 4). Re-recording the golden rather than translating its labels
would still destroy the property, which is why `tools/messaging_role_key_check.py`
exists and why the rename never ran the snapshot with `-update`.

**`combat_messages.golden` records one COORDINATED VARIANT per row** (M3 item
8), keyed `subtype|intensity|split|tier|index` with every authored role named
inside the row. 2,280 rows covering all 6,975 dogmud lines, each exactly once.
Reading across a row is how you check that the three audiences describe one
moment, which no assertion can do for you.

It is keyed per TIER, not per skill level: the union is cumulative, so a
skill-level key would record every beginner line three times.

Two earlier shapes are worth knowing about, because both were blind. Until PR
1 of item 8 it took a fresh `SequencePicker` per tuple and so recorded only
index 0 of each tier, which meant a 984-line content pad would have moved 6 of
1,632 rows. And its intensity list had 8 entries, omitting `CoupDeGrace`
entirely, so `generic`'s 25 coup-de-grace lines had never been covered by any
golden. Both are fixed; `coupdegrace` is recorded for `generic` only, since
every other subtype reaches it through `GetPreAttackMessage`'s Generic
fallback and would otherwise duplicate those rows 19 times.

A short pool renders as `<none>` in a row. There are none, which is an
independent check that every group is equal.

**A single-variant store renders with `FirstPicker`, never the default.**
`Render` always calls `pick(n)`, `DefaultPicker` always calls `util.Rand`, and
`util.Rand(1)` still calls `rand.Intn`, so a one-line pool rendered through the
default picker consumes a global random draw and shifts every later combat
roll. `Render` cannot special-case `n == 1` because an item tree's speech pool
may legitimately hold one line, and its draw count must not change (a seeded
run replays it). The condition, spell, quest and crafting stores reach `Render` only
through `textutil.Narrate`, which passes `FirstPicker`; the root guard
`narration_render_callers_guard_test.go` pins both facts.

**The Kind B golden headers describe the recording, not today's builder.**
`conditions.golden`, `spells.golden`, `quests.golden` and `crafting.golden`
were recorded from pre-migration code (M3 item 5b, Task 0; `crafting.golden`
in M3 item 6, Task 0) and their header lines are frozen bytes; the builders
now read through the store doors and must reproduce the files exactly.
Re-recording them is a deliberate act for a content change, never a way to
make a red run green.

**`gossip.golden` and `tips.golden` (M3 item 7) freeze the two non-combat
stores.** `gossip.golden` keys each row `gossip|<key>|<index> => <line>`,
substituting the stand-in `"<the stand-in event>"` through
`gossip.RenderWithForTest` with a fixed index picker, since the real pick
(`util.Rand`) cannot be pinned; the header says so. `tips.golden` keys each
row `tip|<index> => <text>` in file order, read through `tips.Load` and
`tips.Next`. Unlike the Kind B goldens, both builders read through their
store's real API end to end rather than re-implementing the pre-store logic,
because the stores existed before either golden was recorded.

**`position_control.golden` (M4a) was the fifteenth golden when it was
recorded, and the store it covers had none at all before the token flip
needed a net.** `_datafiles/messages/position_control.yaml` is numbered
Store 15 in `snapshot_test.go` and was the last store in the messaging arc
to gain a snapshot. It was recorded from PRE-migration code, the local
`substitute` in `internal/hooks/Position_Messaging.go`, looping over the
three separate authored name vocabularies the file used to carry. Its
`gradient_messages` and `transition_messages` blocks have NO Go reader
today, so those rows guard the DATA rather than a render path: a row
deleted there would be invisible to every other test in the repo.

**Corrected 2026-09-25: "fifteenth" is no longer the count, it is a
historical fact about when this golden was added.** Two more stores have
gained snapshots since: `special_moves.golden` (messaging M4e) and
`light_notices.golden` (lighting plan 3d, below), bringing
`testdata/stores/` to 17 files and `TestSnapshotStores` to 17 subtests.
`snapshot_test.go`'s own `// Store N` comments now carry a duplicate: both
`position_control` and `special_moves` are commented `Store 15`, and
`light_notices` carries no number at all. That is a pre-existing label
drift in the test file's comments, not a bug in what the goldens cover;
verify a store count by listing `testdata/stores/` or counting
`t.Run(` calls in `TestSnapshotStores`, never by reading a `// Store N`
comment.

**`light_notices.golden` (lighting plan 3d) freezes every line of every
`internal/lightnotice` cause, transition and setting, at every index.**
Built by `buildLightNoticesGolden`, keyed
`<cause>|<transition>|<setting>|<index> => <line>` where `setting` is
`any`, `outdoor` or `indoor`. Unlike the Kind B goldens above, it reads the
shipped YAML straight through `fileloader.LoadAllFlatFiles` and
`lightnotice.Causes()`/`Transitions()`, the store's own public vocabulary,
rather than re-implementing pre-migration logic, because the store and this
golden were born together in the same plan.

**`Substitute` is the only NARRATION token engine, and a root guard says so.**
`token_engine_guard_test.go` (repo root) parses every non-test Go file under
`internal/` and `modules/`, skips `internal/narration/`, and fails on any
`strings.Replace`, `strings.ReplaceAll` or `strings.NewReplacer` call handed a
string literal containing a brace. Its allow-list is empty and should stay that
way. Three engines survived M0 through M3 because nothing checked
(`items.SetTokenValue`, `grapplemessaging.RenderTemplate` and the `hooks` local
`substitute`); all three rendered the same stores through different code, and
all three are gone. One brace engine survives on purpose and is a different
job: the status prompt in `internal/users/userrecord.prompt.go`, a regexp over
its own HUD vocabulary (`{hp}`, `{target}`, `{tnl}`), which the matcher never
reaches because it holds no `strings.Replace` call. The matcher is deliberately
narrow and does not recognise a `regexp` or `bytes` based engine; its own
comment says so rather than claiming coverage it does not have.

## Dependencies

`internal/util` only, plus stdlib. Keeping it that way is what lets every store
package import this one without risking a cycle, and it is why the core lives
here rather than in `internal/items`: `items` imports `internal/conditions`, so the
conditions store could never have reached a core hosted there.
