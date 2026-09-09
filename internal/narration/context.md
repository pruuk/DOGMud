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
| `picker.go` | `Picker`, `DefaultPicker`, `SequencePicker`. |
| `render.go` | `Selector`, `Variants`, `Roles`, `Render`, `ValidateVariants`. The core. |
| `picker_test.go` | Unit tests for the two pickers. |
| `render_test.go` | Unit tests for the core, including the coordinated-index probe. |
| `snapshot_test.go` | The M1 harness: golden snapshots of the message stores, raw and post-pipeline. |
| `testdata/stores/` | Those goldens. |

## Public API

```go
// Picker chooses an index in [0,n).
type Picker func(n int) int
func DefaultPicker(n int) int
func SequencePicker() Picker

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
```

A nil `Picker` means production behaviour: stores that accept one treat nil as
`DefaultPicker` rather than panicking.

## The assembly rule

**The store assembles the pools and computes the selector. The core only
coordinates the index and substitutes tokens.**

The core deliberately knows nothing about bands, skill tiers or cooldowns.
`items.SkillTieredMessages.GetForSkillLevelWith` UNIONS beginner, expert and
master by skill level, and `grapplemessaging.PickTemplate` FILTERS
recently-used templates out; both hand `Render` an already-assembled
`[]string`. Pulling either into the core would turn the messaging arc's M4
parameter flip into a core rewrite.

The same rule is why banding stays in the stores: `items.GetDefenseMessage`
bands on a z-score while `items.RenderDefenseMessage` bands on crit plus
margin, and those two disagree deliberately. Reconciling them changes which
band fires, which is a behaviour change and belongs to M4, not to this package.

## Consumers

`internal/items` (defence and combat-message stores), `internal/itemvoices`,
`internal/spells` (casting), `internal/combat` (taunt),
`internal/grapplemessaging`. Any store that wants deterministic selection under
snapshot, or coordinated multi-role rendering.

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
Without it the function cannot tell a role that is deliberately absent (a buff
has no actee) from one that went MISSING (a defence band that lost its toroom
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
(`todefender`, `toattacker`, `toroom`), not by the core's role names. That is
deliberate and load-bearing: it makes the golden able to catch a swap of which
authored pool lands in which role, which is the mistake this core makes easiest
to introduce. Re-recording it under the core's vocabulary would destroy that
property.

## Dependencies

`internal/util` only, plus stdlib. Keeping it that way is what lets every store
package import this one without risking a cycle, and it is why the core lives
here rather than in `internal/items`: `items` imports `internal/buffs`, so the
buffs store could never have reached a core hosted there.
