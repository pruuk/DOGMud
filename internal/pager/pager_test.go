package pager

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSize(t *testing.T) {
	cases := []struct {
		name              string
		reported, setting int
		screenReader      bool
		wantSize          int
		wantOn            bool
	}{
		{"off wins over everything", 50, SettingOff, true, 0, false},
		{"a setting wins over the screen", 50, 30, false, 30, true},
		{"a setting wins for a screen reader too", 50, 30, true, 30, true},
		{"a screen reader without a setting gets the fixed size", 50, 0, true, ScreenReaderSize, true},
		{"the screen less the stop line and prompt", 24, 0, false, 22, true},
		{"never below the floor", 6, 0, false, MinSize, true},
		{"a setting is clamped to the ceiling", 50, 500, false, MaxSize, true},
		{"a setting below the floor is raised", 50, 3, false, MinSize, true},
		{"a huge report is clamped to the ceiling", 999, 0, false, MaxSize, true},
		{"a huge report is clamped for a screen reader's setting too", 0, 999, false, MaxSize, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			size, on := Size(c.reported, c.setting, c.screenReader)
			require.Equal(t, c.wantOn, on)
			require.Equal(t, c.wantSize, size)
		})
	}
}

func TestLines(t *testing.T) {
	require.Equal(t, []string{"a", "", "b"}, Lines("a\n\nb\n"))
	require.Equal(t, []string{"a"}, Lines("a"))
	require.Empty(t, Lines(""))
}

func TestCut(t *testing.T) {
	lines := []string{"1", "2", "3", "4", "5"}
	require.Equal(t, [][]string{{"1", "2"}, {"3", "4"}, {"5"}}, Cut(lines, 2))
	require.Equal(t, [][]string{{"1", "2", "3", "4", "5"}}, Cut(lines, 5), "an exact fit is one page")
	require.Equal(t, [][]string{{"1"}, {"2"}, {"3"}, {"4"}, {"5"}}, Cut(lines, 0), "a size below 1 counts as 1")
	require.Equal(t, [][]string{{"1", "2"}, {"3"}}, Cut([]string{"1", "2", "", "3"}, 2),
		"a blank line is not carried to the top of the next page")
	require.Equal(t, [][]string{{"x"}}, Cut([]string{"", "", "x"}, 2), "page 1 does not open blank either")
	require.Empty(t, Cut([]string{"", ""}, 2), "all-blank input has no pages")
	require.Empty(t, Cut(nil, 3))
}

func TestJoin(t *testing.T) {
	require.Equal(t, "a\nb\n"+resetColour, Join([]string{"a", "b"}))
}

func TestStopLine(t *testing.T) {
	s := StopLine(1, 3)
	require.Equal(t, "More (page 1 of 3): press Enter for the next page, or type q to stop. ", s)
	require.False(t, strings.HasSuffix(s, "\n"), "the stop line is a prompt, with no line break")
}
