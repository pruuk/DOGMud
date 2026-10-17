package templates

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var tableFrameTagRE = regexp.MustCompile(`</?ansi[^>]*>`)

// TestTableFramesJoinAtTheColumnBorders pins the corner and joiner glyphs of
// the two boxed table templates. The 2026-10-17 playtest saw the `who` table
// at login (tables/generic) draw `╕` mid-row on its top line and `└┘` pairs as
// its header rule and bottom; the shop list (tables/shoplist) had the same
// frame. Every joiner must sit on the column border the rows draw with `│`.
func TestTableFramesJoinAtTheColumnBorders(t *testing.T) {
	table := GetTable(
		"Test Table",
		[]string{"Name", "Value", "Type"},
		[][]string{
			{"alpha", "100", "x"},
			{"beta", "200", "y"},
		},
	)
	for _, tpl := range []string{"tables/generic", "tables/shoplist"} {
		out, err := Process(tpl, table)
		require.NoError(t, err)
		var lines []string
		for _, l := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
			l = strings.TrimRight(tableFrameTagRE.ReplaceAllString(l, ""), " ")
			if l != "" {
				lines = append(lines, l)
			}
		}
		// Title, top, header, rule, two rows, bottom.
		require.Len(t, lines, 7, "%s rendered:\n%s", tpl, strings.Join(lines, "\n"))
		top, header, rule, row, bottom := []rune(lines[1]), []rune(lines[2]), []rune(lines[3]), []rune(lines[4]), []rune(lines[6])

		for name, frame := range map[string][]rune{"top": top, "header": header, "rule": rule, "bottom": bottom} {
			require.Equal(t, len(row), len(frame), "%s %s line is not as wide as a row:\n%s\n%s", tpl, name, string(frame), string(row))
		}
		last := len(row) - 1
		for i, r := range row {
			if r != '│' {
				continue
			}
			var want [4]rune
			switch i {
			case 0:
				want = [4]rune{'╒', '│', '├', '└'}
			case last:
				want = [4]rune{'╕', '│', '┤', '┘'}
			default:
				want = [4]rune{'╤', '│', '┼', '┴'}
			}
			got := [4]rune{top[i], header[i], rule[i], bottom[i]}
			if got != want {
				t.Errorf("%s column border at %d: top/header/rule/bottom are %q, want %q\n%s\n%s\n%s\n%s",
					tpl, i, string(got[:]), string(want[:]), string(top), string(header), string(rule), string(bottom))
			}
		}
	}
}
