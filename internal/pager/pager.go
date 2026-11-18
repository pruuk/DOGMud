// Package pager cuts a long reply into pages that fit a reader's screen
// (#842). It is pure: no I/O and no state. internal/users keeps the pages
// and internal/hooks sends them.
package pager

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mattn/go-runewidth"
)

const (
	// SettingOff is the saved page size that turns paging off
	// (UserRecord.PageSize; 0 follows the screen).
	SettingOff = -1
	// ScreenReaderSize is the page size, in rows, of a screen reader account
	// with no setting of its own: long enough for a menu and its NPC's
	// words, short enough to reach the choices without a long wait.
	ScreenReaderSize = 20
	// MinSize is the smallest page in rows, however small the screen
	// reports, and the smallest `set pagesize`.
	MinSize = 10
	// MaxSize is the largest page a setting or a report may ask for.
	MaxSize = 200
	// promptRows is the row a page leaves under the stop line for the
	// cursor the player types at.
	promptRows = 1
	// resetColour ends a page, so a colour opened on its last line does not
	// run into the stop line.
	resetColour = "\x1b[0m"
)

// escapeSeq matches one ANSI control sequence (a colour, a cursor move, an
// erase), which takes no column on screen.
var escapeSeq = regexp.MustCompile("\x1b\\[[0-?]*[ -/]*[@-~]")

// Rows is the number of screen rows line fills on a reader width columns
// wide (owner ruling 6): its visible width (escape sequences removed, a wide
// character counted as two columns) divided by the width, rounded up, and at
// least 1, so a blank line is a row. A width below 1 counts every line as
// one row.
func Rows(line string, width int) int {
	if width < 1 {
		return 1
	}
	cols := runewidth.StringWidth(escapeSeq.ReplaceAllString(line, ``))
	return max(1, (cols+width-1)/width)
}

// Size is the page size in screen rows for a reader and whether paging is
// on. setting is UserRecord.PageSize: SettingOff, 0 to follow the screen, or
// a size. reported is the screen's height in rows and width its width in
// columns. Following the screen, a page is the height less the rows a stop
// line fills at that width (as "page 1 of 1"; Pages reserves a longer one)
// and less the prompt's row. When paging is on, the size is clamped to
// MinSize..MaxSize, whichever of the three it came from.
func Size(reported, width, setting int, screenReader bool) (int, bool) {
	return sizeFor(reported, Rows(StopLine(1, 1), width), setting, screenReader)
}

// sizeFor is Size with the stop line's rows given.
func sizeFor(reported, stopRows, setting int, screenReader bool) (int, bool) {
	var n int
	switch {
	case setting == SettingOff:
		return 0, false
	case setting > 0:
		n = setting
	case screenReader:
		n = ScreenReaderSize
	default:
		n = reported - stopRows - promptRows
	}
	return min(max(n, MinSize), MaxSize), true
}

// Pages cuts lines into pages for a reader width columns wide whose screen
// is reported rows high (Size and Cut). The stop line names the page count,
// so a reply of many pages can need a longer stop line than "page 1 of 1";
// when it fills more rows, the lines are cut again with those rows
// reserved. nil when paging is off.
func Pages(lines []string, width, reported, setting int, screenReader bool) [][]string {
	stopRows := Rows(StopLine(1, 1), width)
	for {
		n, on := sizeFor(reported, stopRows, setting, screenReader)
		if !on {
			return nil
		}
		pages := Cut(lines, n, width)
		need := Rows(StopLine(len(pages), len(pages)), width)
		if need <= stopRows {
			return pages
		}
		stopRows = need
	}
}

// Lines splits sent text into its screen lines. The text's own final line
// break does not make an extra empty line.
func Lines(text string) []string {
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// Cut cuts lines into pages of at most size screen rows on a reader width
// columns wide, each line counting the rows it fills (Rows). A line taller
// than a page gets a page of its own. A blank line that would open a page,
// the first included, is dropped, so no page starts with a blank line.
func Cut(lines []string, size, width int) [][]string {
	size = max(size, 1)
	var pages [][]string
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 {
		n, rows := 0, 0
		for n < len(lines) {
			r := Rows(lines[n], width)
			if n > 0 && rows+r > size {
				break
			}
			rows += r
			n++
		}
		pages = append(pages, lines[:n])
		lines = lines[n:]
		for len(lines) > 0 && lines[0] == "" {
			lines = lines[1:]
		}
	}
	return pages
}

// Join is the text to send for a page.
func Join(page []string) string {
	return strings.Join(page, "\n") + "\n" + resetColour
}

// StopLine is the prompt shown while pages wait: page is the page just
// shown (from 1), of the number of pages. Plain words, so a screen reader
// reads it as it is.
func StopLine(page, of int) string {
	return fmt.Sprintf("More (page %d of %d): press Enter for the next page, or type q to stop. ", page, of)
}
