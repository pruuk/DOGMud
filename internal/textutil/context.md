# Text Utility Context

## Purpose

`internal/textutil` is the thin adapter between the single-string narration
stores (conditions, spells, quests, crafting: one authored line per lifecycle
phase and audience) and the rendering core in `internal/narration`. It maps
those stores onto the canonical name-token vocabulary and owns the one
function they render through.

## Files

- **tokens.go**: `TokenContext`, `Tokens`, `SubstituteTokens`, `ValidateTokens`.
- **narrate.go**: `Pool`, `Narrate`.

## API

```go
type TokenContext struct { ActorName, ActorPlainName, ActeeName, ActeePlainName string }

func (ctx TokenContext) Tokens() map[string]string     // all four keys, always
func Pool(text string) []string                        // nil for "", else one variant
func Narrate(v narration.Variants, ctx TokenContext) narration.Roles
func SubstituteTokens(text string, ctx TokenContext) string
func ValidateTokens(text string) []string
```

A store assembles its `narration.Variants` (which line is Actor, Actee,
Observer) and calls `Narrate`; the site delivers each role on its own channel.
The four keys `Tokens` emits are `narration.TokenActor`, `TokenActee`,
`TokenActorPlain` and `TokenActeePlain` (`{actor}`, `{actee}`, `{actor_plain}`,
`{actee_plain}`), the one vocabulary every store's YAML is written in since
messaging M4a. `ValidateTokens` knows those four and nothing else.
`SubstituteTokens` is a one-variant `Narrate`. Two production callers remain:
the dialogue store (`internal/behaviortree/actions_dialogue.go`, which the
messaging arc migrates in its item 7) and `conditions.AuthoredStartLine`, the door
for a silent-start condition's applier. It cannot be deleted at item 7 without
moving the second.

## Gotchas

- **`Narrate` always passes `narration.FirstPicker`.** A single-variant store
  has nothing to choose, and the default picker would consume a global random
  draw per narrated phase (`util.Rand(1)` still draws). The guard
  `narration_render_callers_guard_test.go` (internal/guards) fails the build if this file names
  the default picker or if a store calls `narration.Render` itself. An empty
  pool renders nothing without building the token map; the condition tick calls
  this every round for every character holding a condition.
- **`Tokens` always carries all four keys**, so an absent actee renders as an
  empty string. That is what `SubstituteTokens` has always done; a line that
  names `{actee}` with no actee has a hole in it. Since M3 item 5b the sites
  gate delivery on the RENDERED role being non-empty, not on the raw field, so
  a line made only of tokens that resolve empty is dropped rather than sent
  blank. No shipped line has that shape; the old gate would have sent it.
- **A misspelled token is left in the line verbatim and does not error.** The
  core substitutes only the four known keys, so `{actae}` reaches the player
  as written (quest 77 once showed a literal name token this way).
  `ValidateTokens` catches it at load, and since M4a the event stores FAIL on
  it: conditions, spells and crafting return it from `Validate`, which their
  loaders turn into a boot panic, and quests fail on an action's `observer`
  line (`internal/quests/roomtext.go`, which also requires `{actor}`). A quest
  action's `actor` line and the reward lines are still not token-checked.
  `internal/crafting` also requires `{actor}` in a room message. The ambient
  stores (weather, gossip, tips) still only warn, until M4b sets the two-tier
  loader policy.
- **`Pool` keeps whitespace.** A whitespace-only authored line must reach
  `narration.ValidateVariants` and be refused at load, not trimmed into
  silence.
- **`SendPhaseText` is gone** (deleted 2026-09-12, messaging M3 item 5b). Sites
  render through the store's `Narrate` door and deliver inline; delivery
  through `messaging.SendTrio` is the arc's M4.

## Dependencies

`internal/narration` only.

## Consumers

`internal/conditions`, `internal/spells`, `internal/quests`, `internal/crafting`
(their `Narrate` doors and validators), `internal/questengine` (the bridge),
the hook and command sites that build a `TokenContext` (`internal/hooks`,
`internal/usercommands`, `internal/mobcommands`), and `internal/behaviortree`
(dialogue, via `SubstituteTokens`).

**Conditions is the exception that no longer costs anything.** Its holder is
the ACTEE, so `ConditionSpec.Narrate` and `AuthoredStartLine` take the holder's
two names and build the context themselves; their call sites (`internal/hooks`,
`internal/actions`, `internal/justice`, `internal/usercommands`) never see a
`TokenContext` and so cannot fill the wrong slot.
