# pager: Package Documentation

## Purpose

`internal/pager` cuts a long reply into pages that fit the reader's screen
(#842, spec `docs/superpowers/specs/2026-11-18-general-pager-design.md`). It
is pure: no I/O, no locks, no state. `internal/users` keeps a player's
pages (`userrecord.pager.go`) and `internal/hooks` sends them.

## Surface

Pages are measured in screen rows at the reader's width (owner ruling 6):
a line the client wraps counts for each row it fills.

- `Rows(line, width) int`: the rows a line fills: visible width (ANSI
  escape sequences removed, wide characters two columns, via
  `go-runewidth`) divided by `width`, rounded up, at least 1. A width below
  1 counts one row.
- `Size(reported, width, setting int, screenReader bool) (int, bool)`: the
  page size in rows and whether paging is on. `setting` is
  `UserRecord.PageSize`: `SettingOff` (-1) turns paging off, a positive
  number is the size, 0 follows the screen. With no setting a screen reader
  account gets `ScreenReaderSize` (20); everyone else gets the reported
  height less the rows `StopLine(1, 1)` fills at `width`, less 1 for the
  prompt. `Size` clamps the result to `MinSize` (10) .. `MaxSize` (200) in
  every "on" case. `MinSize`..`MaxSize` is also the range `set pagesize`
  accepts.
- `Pages(lines, width, reported, setting, screenReader) [][]string`: `Size`
  then `Cut`, cut again with more rows reserved when the reply's own stop
  line ("page 10 of 12") fills more rows than "page 1 of 1". nil when
  paging is off. `internal/users` calls this.
- `Lines(text) []string`: sent text as screen lines; the final line break
  does not add an empty line.
- `Cut(lines, size, width) [][]string`: pages of at most `size` rows, each
  line counting its `Rows`; a line taller than a page gets one of its own;
  no page starts with a blank line (leading blanks are dropped, page 1
  included).
- `Join(page) string`: the text to send for a page, ending with a colour
  reset so a colour left open does not reach the stop line.
- `StopLine(page, of) string`: the prompt while pages wait, "More (page 1
  of 3): press Enter for the next page, or type q to stop." No line break,
  so clients treat it as a prompt.

The caller supplies the width: `UserRecord.PagerWidth` (a web client's
reported columns, a telnet client's NAWS width, else `WrapWidth`).
