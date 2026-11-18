// Package pager cuts a long reply into pages that fit a reader's screen
// (#842). It is pure: no I/O and no state. internal/users keeps the pages
// and internal/hooks sends them.
package pager

import (
	"fmt"
	"strings"
)

const (
	// SettingOff is the saved page size that turns paging off
	// (UserRecord.PageSize; 0 follows the screen).
	SettingOff = -1
	// ScreenReaderSize is the page size of a screen reader account with no
	// setting of its own: long enough for a menu and its NPC's words, short
	// enough to reach the choices without a long wait.
	ScreenReaderSize = 20
	// MinSize is the smallest page, however small the screen reports.
	MinSize = 10
	// MaxSize is the largest page a setting or a report may ask for.
	MaxSize = 200
	// reservedRows are the rows a page leaves for the stop line and the
	// prompt.
	reservedRows = 2
	// resetColour ends a page, so a colour opened on its last line does not
	// run into the stop line.
	resetColour = "\x1b[0m"
)

// Size is the page size for a reader and whether paging is on. setting is
// UserRecord.PageSize: SettingOff, 0 to follow the screen, or a size.
// reported is the screen's height in rows. When paging is on, the size is
// clamped to MinSize..MaxSize, whichever of the three it came from.
func Size(reported, setting int, screenReader bool) (int, bool) {
	var size int
	switch {
	case setting == SettingOff:
		return 0, false
	case setting > 0:
		size = setting
	case screenReader:
		size = ScreenReaderSize
	default:
		size = reported - reservedRows
	}
	return min(max(size, MinSize), MaxSize), true
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

// Cut cuts lines into pages of at most size lines. A blank line that would
// open a page, the first included, is dropped, so no page starts with a
// blank line.
func Cut(lines []string, size int) [][]string {
	size = max(size, 1)
	var pages [][]string
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 {
		n := min(size, len(lines))
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
