# internal/narration

## Purpose

Owns the seam every narration store uses to CHOOSE a line from a pool, so a
snapshot run can be deterministic without touching global randomness. That is
all it does. It does not render text, does not know about categories or
audiences, and deliberately does not deliver anything: delivery is
`internal/messaging`, which this package must never import (see Gotchas).

## Files

| File | Holds |
|---|---|
| `picker.go` | `Picker`, `DefaultPicker`, `SequencePicker`. The whole public surface. |
| `picker_test.go` | Unit tests for the two pickers. |
| `snapshot_test.go` | The M1 harness: golden snapshots of the message stores, raw and post-pipeline. |
| `testdata/stores/` | Those goldens. |

## Public API

```go
// Picker chooses an index in [0,n).
type Picker func(n int) int

// DefaultPicker is production. Routes through util.Rand, the engine's single
// randomness seam, NOT stdlib rand.
func DefaultPicker(n int) int

// SequencePicker walks indices in order, wrapping at n.
func SequencePicker() Picker
```

A nil `Picker` means production behaviour: stores that accept one treat nil as
`DefaultPicker` rather than panicking.

## Gotchas

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

## Dependencies

`internal/util` only.

## Consumers

`internal/items` (defence and item-voice stores). Any store that wants
deterministic selection under snapshot.
