# Banner Context

## Purpose

`internal/banner` formats the SKILL ADVANCEMENT / STATISTIC INCREASED block
the progression system shows when a skill or stat goes up: a rule line, a
centred title, the centred skill or stat name, an optional tier transition,
and a closing rule. It is a leaf package (imports only `strings` and
`unicode/utf8`) so `internal/characters` can use it without an import cycle.

## API

```go
type Kind int // Skill, Stat
type TierChange struct{ From, To string }
const RuleWidth = 64

func Format(kind Kind, name string, tier *TierChange, width int) string
```

`tier` is optional: pass nil when no tier boundary was crossed. `width` is the
reader's screen width (`WrapWidth()`); 0 or less means none is known. The
result has no trailing newline.

## Gotchas

- **The rule is `min(width, RuleWidth)` glyphs of `━`** and every line is
  centred against it by rune count (plan 3b-3). A desktop, at 64 columns and
  wider, gets exactly the 64-glyph banner it always had
  (`testdata/banner_80.golden`, recorded from master); a 30-column phone gets
  a 30-glyph rule. A name longer than the rule is returned unpadded.
- **Width-aware Go, never a measured form.** The banner goes out on a
  per-round path, so it is laid out for the width given and never rendered
  twice to pick a form (spec ruling 12).
- **The returned string is plain text with no ANSI tags.** Color comes from
  delivery: since messaging M3 item 6 the banner is sent on
  `messaging.CategorySkillProgress`, whose color stage wraps the whole block in
  the `skill-progress` alias (179, gold).
- **`Format` returns text; it does not send.** Its only consumer,
  `internal/characters/progression_notify.go` (`notifyProgressionBanner`),
  hands the callback `main.go` registers (`hooks.ProgressionNotifyCallback`) a
  render function; the callback calls it at the user's `WrapWidth()`.

## Dependencies

`strings`, `unicode/utf8`. Nothing in the repo.

## Consumers

`internal/characters/progression_notify.go` only (skill and stat advancement,
called from `progression.go`).
