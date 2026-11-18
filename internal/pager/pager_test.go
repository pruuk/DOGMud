package pager

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSize(t *testing.T) {
	cases := []struct {
		name                     string
		reported, width, setting int
		screenReader             bool
		wantSize                 int
		wantOn                   bool
	}{
		{"off wins over everything", 50, 80, SettingOff, true, 0, false},
		{"a setting wins over the screen", 50, 80, 30, false, 30, true},
		{"a setting wins for a screen reader too", 50, 80, 30, true, 30, true},
		{"a screen reader without a setting gets the fixed size", 50, 80, 0, true, ScreenReaderSize, true},
		{"the screen less a one-row stop line and the prompt", 24, 80, 0, false, 22, true},
		{"a 36-column phone: the stop line fills 2 rows", 16, 36, 0, false, 13, true},
		{"a 30-column phone: the stop line fills 3 rows", 16, 30, 0, false, 12, true},
		{"never below the floor", 6, 80, 0, false, MinSize, true},
		{"a setting is clamped to the ceiling", 50, 80, 500, false, MaxSize, true},
		{"a setting below the floor is raised", 50, 80, 3, false, MinSize, true},
		{"a huge report is clamped to the ceiling", 999, 80, 0, false, MaxSize, true},
		{"a huge report is clamped for a screen reader's setting too", 0, 80, 999, false, MaxSize, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			size, on := Size(c.reported, c.width, c.setting, c.screenReader)
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

// The stop line grows with the page count: at 70 columns "page 1 of 1" fits
// one row and "page 10 of 10" needs two, so the pages are cut again for the
// longer stop line.
func TestPages_ReservesTheStopLineForThePageCount(t *testing.T) {
	require.Equal(t, 1, Rows(StopLine(1, 1), 70))
	require.Equal(t, 2, Rows(StopLine(10, 10), 70))
	lines := make([]string, 180)
	for i := range lines {
		lines[i] = "x"
	}
	pages := Pages(lines, 70, 20, 0, false)
	require.Len(t, pages, 11, "17 rows a page once the stop line takes two")
	for _, p := range pages {
		require.LessOrEqual(t, len(p), 17)
	}
	require.Nil(t, Pages(lines, 70, 20, SettingOff, false), "paging off cuts nothing")
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
	require.Equal(t, "More (page 1 of 3): press Enter for the next page, or type q to stop. ", s)
	require.False(t, strings.HasSuffix(s, "\n"), "the stop line is a prompt, with no line break")
}
