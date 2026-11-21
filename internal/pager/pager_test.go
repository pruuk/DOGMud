package pager

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/stretchr/testify/require"
)

func TestSize(t *testing.T) {
	cases := []struct {
		name                                string
		reported, width, wrapWidth, setting int
		screenReader                        bool
		wantSize                            int
		wantOn                              bool
	}{
		{"off wins over everything", 50, 80, 80, SettingOff, true, 0, false},
		{"a setting wins over the screen", 50, 80, 80, 30, false, 30, true},
		{"a setting wins for a screen reader too", 50, 80, 80, 30, true, 30, true},
		{"a screen reader without a setting gets the fixed size", 50, 80, 80, 0, true, ScreenReaderSize, true},
		{"the screen less a one-row stop line and the prompt", 24, 80, 80, 0, false, 22, true},
		{"a 36-column phone: the stop line fits 1 row", 16, 36, 36, 0, false, 14, true},
		{"a 30-column phone: the stop line fits 1 row", 16, 30, 30, 0, false, 14, true},
		{"a 20-column screen: the stop line fills 2 rows", 16, 20, 20, 0, false, 13, true},
		{"a 12-column screen: the stop line fills 3 rows", 16, 12, 12, 0, false, 12, true},
		{"an 80-column screen wrapping at 20 (set linewidth): the stop line is 2 lines", 16, 80, 20, 0, false, 13, true},
		{"never below the floor", 6, 80, 80, 0, false, MinSize, true},
		{"a setting is clamped to the ceiling", 50, 80, 80, 500, false, MaxSize, true},
		{"a setting below the floor is raised", 50, 80, 80, 3, false, MinSize, true},
		{"a huge report is clamped to the ceiling", 999, 80, 80, 0, false, MaxSize, true},
		{"a huge report is clamped for a screen reader's setting too", 0, 80, 80, 999, false, MaxSize, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			size, on := Size(c.reported, c.width, c.wrapWidth, c.setting, c.screenReader)
			require.Equal(t, c.wantOn, on)
			require.Equal(t, c.wantSize, size)
		})
	}
}

// colouredLine is n visible x's wrapped in a colour, as a held reply carries
// them after AnsiParse.
func colouredLine(n int) string {
	return "\x1b[1;33m" + strings.Repeat("x", n) + "\x1b[0m"
}

func TestRows(t *testing.T) {
	require.Equal(t, 1, Rows("", 36), "a blank line is a row")
	require.Equal(t, 1, Rows(strings.Repeat("x", 36), 36), "an exact fit is one row")
	require.Equal(t, 2, Rows(strings.Repeat("x", 37), 36))
	require.Equal(t, 2, Rows(colouredLine(39), 36), "colour codes take no columns")
	require.Equal(t, 1, Rows(colouredLine(36), 36))
	require.Equal(t, 1, Rows("abc\r", 3), "a carriage return takes no column")
	require.Equal(t, 2, Rows(strings.Repeat("日", 19), 36), "a wide character takes two columns")
	require.Equal(t, 1, Rows(strings.Repeat("x", 99), 0), "no width counts one row")
}

// A 36-column reader sees each 39-column line as 2 rows, so a 13-row page
// holds 6 of them: what fits in lines does not fit in rows.
func TestCut_CountsRowsAtTheWidth(t *testing.T) {
	var lines []string
	for range 10 {
		lines = append(lines, colouredLine(39))
	}
	pages := Cut(lines, 13, 36)
	require.Len(t, pages, 2)
	require.Len(t, pages[0], 6)
	require.Len(t, pages[1], 4)
}

// A line taller than the page gets a page of its own.
func TestCut_ATallLineGetsItsOwnPage(t *testing.T) {
	tall := strings.Repeat("x", 36*5)
	pages := Cut([]string{"a", tall, "b"}, 3, 36)
	require.Equal(t, [][]string{{"a"}, {tall}, {"b"}}, pages)
}

// The stop line grows with the page count: at 30 columns "Page 1 of 1" fits
// one row and "Page 10 of 10" needs two, so the pages are cut again for the
// longer stop line.
func TestPages_ReservesTheStopLineForThePageCount(t *testing.T) {
	require.Equal(t, 1, Rows(StopLine(1, 1), 30))
	require.Equal(t, 2, Rows(StopLine(10, 10), 30))
	lines := make([]string, 180)
	for i := range lines {
		lines[i] = "x"
	}
	pages := Pages(lines, 30, 30, 20, 0, false)
	require.Len(t, pages, 11, "17 rows a page once the stop line takes two")
	for _, p := range pages {
		require.LessOrEqual(t, len(p), 17)
	}
	require.Nil(t, Pages(lines, 70, 70, 20, SettingOff, false), "paging off cuts nothing")
}

// The stop line is shown word wrapped (messaging.WrapPrompt, as
// users.CommandPromptBody wraps every prompt), so the rows reserved for it are
// the rows of its wrapped lines, which word wrapping can make more than its
// columns divided by the width.
func TestStopRows_CountsTheWrappedStopLine(t *testing.T) {
	for _, tt := range []struct {
		name             string
		width, wrapWidth int
		want             int
	}{
		{"a 38-column phone: one line", 38, 38, 1},
		{"a 33-column phone: one line", 33, 33, 1},
		{"80 columns: one line", 80, 80, 1},
		{"a 20-column screen: two lines", 20, 20, 2},
		// 30 columns over 16 is 2 rows, but no word is split, so it takes 3.
		{"word wrapping needs a row more than the columns", 16, 16, 3},
		// set linewidth 20 on an 80-column screen: wrapped to 2 lines of 1 row.
		{"wrapped narrower than the screen", 80, 20, 2},
		// Wrapped at the 30-column floor on a 20-column telnet screen: the
		// one 30-column line fills 2 rows.
		{"wrapped wider than the screen", 20, 30, 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, stopRows(StopLine(1, 1), tt.width, tt.wrapWidth))
		})
	}
	require.Equal(t, 2, Rows(StopLine(1, 1), 16), "the column count alone says 2 at 16")
}

// A page on a 20-column reader leaves the wrapped stop line its two rows and
// the prompt its row: a 16-row screen holds 13 one-row lines.
func TestPages_ReservesTheWrappedStopLine(t *testing.T) {
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = "x"
	}
	pages := Pages(lines, 80, 20, 16, 0, false)
	require.Len(t, pages[0], 13, "16 rows less 2 for the stop line wrapped at 20, less the prompt row")
}

func TestLines(t *testing.T) {
	require.Equal(t, []string{"a", "", "b"}, Lines("a\n\nb\n"))
	require.Equal(t, []string{"a"}, Lines("a"))
	require.Empty(t, Lines(""))
}

func TestCut(t *testing.T) {
	lines := []string{"1", "2", "3", "4", "5"}
	require.Equal(t, [][]string{{"1", "2"}, {"3", "4"}, {"5"}}, Cut(lines, 2, 80))
	require.Equal(t, [][]string{{"1", "2", "3", "4", "5"}}, Cut(lines, 5, 80), "an exact fit is one page")
	require.Equal(t, [][]string{{"1"}, {"2"}, {"3"}, {"4"}, {"5"}}, Cut(lines, 0, 80), "a size below 1 counts as 1")
	require.Equal(t, [][]string{{"1", "2"}, {"3"}}, Cut([]string{"1", "2", "", "3"}, 2, 80),
		"a blank line is not carried to the top of the next page")
	require.Equal(t, [][]string{{"x"}}, Cut([]string{"", "", "x"}, 2, 80), "page 1 does not open blank either")
	require.Empty(t, Cut([]string{"", ""}, 2, 80), "all-blank input has no pages")
	require.Empty(t, Cut(nil, 3, 80))
}

func TestJoin(t *testing.T) {
	require.Equal(t, "a\nb\n"+resetColour, Join([]string{"a", "b"}))
}

func TestStopLine(t *testing.T) {
	s := StopLine(1, 3)
	require.Equal(t, "Page 1 of 3: Enter, q to stop ", s)
	require.False(t, strings.HasSuffix(s, "\n"), "the stop line is a prompt, with no line break")
}

// The stop line fits one row on the 33-column phone, even at page 10 of 12.
func TestStopLine_FitsOneRowAt33Columns(t *testing.T) {
	wrapped := messaging.WrapPrompt(StopLine(10, 12), 33)
	require.NotContains(t, wrapped, "\n")
	require.Equal(t, 1, stopRows(StopLine(10, 12), 33, 33))
}
