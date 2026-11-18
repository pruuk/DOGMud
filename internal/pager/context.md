# pager: Package Documentation

## Purpose

`internal/pager` cuts a long reply into pages that fit the reader's screen
(#842, spec `docs/superpowers/specs/2026-11-18-general-pager-design.md`). It
is pure: no I/O, no locks, no state. `internal/users` keeps a player's
pages (`userrecord.pager.go`) and `internal/hooks` sends them.

## Surface

- `Size(reported, setting int, screenReader bool) (int, bool)`: the page size
  and whether paging is on. `setting` is `UserRecord.PageSize`:
  `SettingOff` (-1) turns paging off, a positive number is the size, 0
  follows the screen. With no setting a screen reader account gets
  `ScreenReaderSize` (20); everyone else gets the reported height less 2
  (the stop line and the prompt), never below `MinSize` (10). `MaxSize`
  (200) bounds a setting or a report.
- `Lines(text) []string`: sent text as screen lines; the final line break
  does not add an empty line.
- `Cut(lines, size) [][]string`: pages of at most `size` lines; a blank line
  that would open a page is dropped.
- `Join(page) string`: the text to send for a page, ending with a colour
  reset so a colour left open does not reach the stop line.
- `StopLine(page, of) string`: the prompt while pages wait, "More (page 1
  of 3): press Enter for the next page, or type q to stop." No line break,
  so clients treat it as a prompt.

Lines are counted as the server wrapped them (to `WrapWidth`); a telnet
client narrower than that wraps again and shows more rows than counted.
