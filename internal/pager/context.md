# pager: Package Documentation

## Purpose

`internal/pager` cuts a long reply into pages that fit the reader's screen
(#842, spec `docs/superpowers/specs/completed/2026-11-18-general-pager-design.md`). It
is pure: no I/O, no locks, no state. `internal/users` keeps a player's
pages (`userrecord.pager.go`) and `internal/hooks` sends them.

## Surface

Pages are measured in screen rows at the reader's width (owner ruling 6):
a line the client wraps counts for each row it fills.

- `Rows(line, width) int`: the rows a line fills: visible width (ANSI
  escape sequences removed with the shared `term.EscapeSeqRe`, wide
  characters two columns, via `go-runewidth`) divided by `width`, rounded
  up, at least 1. A width below 1 counts one row.
- `Size(reported, width, wrapWidth, setting int, screenReader bool) (int, bool)`:
  the page size in rows and whether paging is on. `setting` is
  `UserRecord.PageSize`: `SettingOff` (-1) turns paging off, a positive
  number is the size, 0 follows the screen. With no setting a screen reader
  account gets `ScreenReaderSize` (20); everyone else gets the reported
  height less the rows the stop line fills as drawn, less 1 for the prompt.
  `Size` clamps the result to `MinSize` (10) .. `MaxSize` (200) in every
  "on" case. `MinSize`..`MaxSize` is also the range `set pagesize` accepts.
- The stop line's rows (unexported `stopRows`): `StopLine` word wrapped at
  `wrapWidth` by `messaging.WrapPrompt`, as `users.CommandPromptBody` wraps
  every prompt, each wrapped line counting its `Rows` at `width`. So a word
  wrap that needs a row more than the columns alone, a wrap narrower than
  the screen (`set linewidth`) and a wrap wider than it (the 30-column floor
  on a narrower telnet screen) are all counted as drawn.
- `Pages(lines, width, wrapWidth, reported, setting, screenReader) [][]string`:
  `Size` then `Cut`, cut again with more rows reserved when the reply's own
  stop line ("page 10 of 12") fills more rows than "page 1 of 1". nil when
  paging is off. `internal/users` calls this.
- `Lines(text) []string`: sent text as screen lines; the final line break
  does not add an empty line.
- `Cut(lines, size, width) [][]string`: pages of at most `size` rows, each
  line counting its `Rows`; a line taller than a page gets one of its own;
  no page starts with a blank line (leading blanks are dropped, page 1
  included).
- `Join(page) string`: the text to send for a page, ending with a colour
  reset so a colour left open does not reach the stop line.
- `StopLine(page, of) string`: the prompt while pages wait, "Page 1 of 3:
  Enter, q to stop " (one trailing space, where the cursor sits; 32 columns
  at page 10 of 12, so one row down to a 33-column phone). No line break of
  its own, so clients treat it as a prompt; `users.CommandPromptBody` word
  wraps it like any prompt.

The caller supplies the widths: `width` is `UserRecord.PagerWidth` (a web
client's reported columns, a telnet client's NAWS width, else `WrapWidth`)
and `wrapWidth` is `UserRecord.WrapWidth`, the width the stop line is
wrapped at. The package imports `internal/messaging` (for `WrapPrompt`) and
`internal/term` (for `EscapeSeqRe`); neither imports it back.
