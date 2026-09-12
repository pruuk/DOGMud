# Text Utility Context

## Purpose

`internal/textutil` is the thin adapter between the single-string narration
stores (buffs, spells, quests: one authored line per lifecycle phase and
audience) and the rendering core in `internal/narration`. It owns the
four-token vocabulary those stores are written in and the one function they
render through.

## Files

- **tokens.go**: `TokenContext`, `Tokens`, `SubstituteTokens`, `ValidateTokens`.
- **narrate.go**: `Pool`, `Narrate`.

## API

```go
type TokenContext struct { SourceName, SourcePlainName, TargetName, TargetPlainName string }

func (ctx TokenContext) Tokens() map[string]string     // all four keys, always
func Pool(text string) []string                        // nil for "", else one variant
func Narrate(v narration.Variants, ctx TokenContext) narration.Roles
func SubstituteTokens(text string, ctx TokenContext) string
func ValidateTokens(text string) []string
```

A store assembles its `narration.Variants` (which line is Actor, Actee,
Observer) and calls `Narrate`; the site delivers each role on its own channel.
`SubstituteTokens` is a one-variant `Narrate`. Two production callers remain:
the dialogue store (`internal/behaviortree/actions_dialogue.go`, which the
messaging arc migrates in its item 7) and `buffs.AuthoredStartLine`, the door
for a silent-start buff's applier. It cannot be deleted at item 7 without
moving the second.

## Gotchas

- **`Narrate` always passes `narration.FirstPicker`.** A single-variant store
  has nothing to choose, and the default picker would consume a global random
  draw per narrated phase (`util.Rand(1)` still draws). The root guard
  `narration_render_callers_guard_test.go` fails the build if this file names
  the default picker or if a store calls `narration.Render` itself. An empty
  pool renders nothing without building the token map; the buff tick calls
  this every round for every buffed character.
- **`Tokens` always carries all four keys**, so an absent target renders as an
  empty string. That is what `SubstituteTokens` has always done; a line that
  names `{target}` with no target has a hole in it. Since M3 item 5b the sites
  gate delivery on the RENDERED role being non-empty, not on the raw field, so
  a line made only of tokens that resolve empty is dropped rather than sent
  blank. No shipped line has that shape; the old gate would have sent it.
- **A misspelled token is left in the line verbatim and does not error.** The
  core substitutes only the four known keys, so `{targat}` reaches the player
  as written (quest 77 once showed a literal `{source}` this way).
  `ValidateTokens` exists to catch it at load; buffs and spells only WARN on
  it today, and quests fail on `room_text` only (`internal/quests/roomtext.go`);
  `send_text` and the reward messages are not token-checked.
- **`Pool` keeps whitespace.** A whitespace-only authored line must reach
  `narration.ValidateVariants` and be refused at load, not trimmed into
  silence.
- **`SendPhaseText` is gone** (deleted 2026-09-12, messaging M3 item 5b). Sites
  render through the store's `Narrate` door and deliver inline; delivery
  through `messaging.SendTrio` is the arc's M4.

## Dependencies

`internal/narration` only.

## Consumers

`internal/buffs`, `internal/spells`, `internal/quests` (their `Narrate` doors and
validators), `internal/questengine` (the bridge), the hook, command, action
and justice sites that build a `TokenContext` (`internal/hooks`,
`internal/usercommands`, `internal/mobcommands`, `internal/actions`,
`internal/justice`), and `internal/behaviortree` (dialogue, via
`SubstituteTokens`).
