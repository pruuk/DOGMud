package templates

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
)

func TestLayoutHelp(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		width int
		want  string
	}{
		{"a paragraph that fits keeps its breaks",
			"Hello world.\nSecond line.", 30,
			"Hello world.\nSecond line."},
		{"a paragraph that does not fit rewraps",
			"The quick brown fox jumps over\nthe lazy dog.", 20,
			"The quick brown fox\njumps over the lazy\ndog."},
		{"an indented paragraph keeps its indent",
			"  aaa bbb ccc ddd", 10,
			"  aaa bbb\n  ccc ddd"},
		{"a line ending in a colon ends its paragraph",
			"Usage:\n  look at the very long thing now", 20,
			"Usage:\n  look at the very\n  long thing now"},
		{"a row takes the same-line form and wraps two in",
			"::rows\n  look  Look at the room you are in.\n::", 30,
			"  look  Look at the room you\n    are in."},
		{"a row is laid out the same way at 80",
			"::rows\n  look          Look at the room.\n::", 80,
			"  look  Look at the room."},
		{"a label ending in a colon takes one space",
			"::rows\nName:        Blood Boil\n::", 30,
			"Name: Blood Boil"},
		{"a label too long to share its line takes its own",
			"::rows\n  bank withdraw [amount/all]  Withdraw gold from the bank\n::", 30,
			"  bank withdraw [amount/all]\n    Withdraw gold from the\n    bank"},
		{"a row's deeper lines continue it",
			"::rows\n  look  Look at\n        the room.\n::", 80,
			"  look  Look at the room."},
		{"a list item hangs under its text when it does not fit",
			"::list\n  - Heavy armor (reduces mobility)\n::", 20,
			"  - Heavy armor\n    (reduces\n    mobility)"},
		{"a list that fits keeps its lines",
			"::list\n  - Heavy armor\n  - Being prone\n::", 30,
			"  - Heavy armor\n  - Being prone"},
		{"kept lines are never joined",
			"::keep\n  quaff poultice\n  quaff draught\n::", 30,
			"  quaff poultice\n  quaff draught"},
		{"a rule heading drops marks until it fits",
			`<ansi fg="yellow">━━━ What Kind of Game Is This? ━━━</ansi>`, 30,
			`<ansi fg="yellow">━ What Kind of Game Is This? ━</ansi>`},
		{"a command that fits a line is never split",
			`See <ansi fg="command">help item-names</ansi> now`, 16,
			"See\n" + `<ansi fg="command">help item-names</ansi>` + "\nnow"},
		{"blank lines inside and between blocks are kept",
			"One.\n\n::rows\n  a  b\n\n  c  d\n::\n\nTwo.", 30,
			"One.\n\n  a  b\n\n  c  d\n\nTwo."},
		{"carriage returns are dropped",
			"a\r\nb", 30,
			"a\nb"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, LayoutHelp(c.in, c.width))
		})
	}
}

// A colour span cut by a line break is closed at the end of the line and
// reopened on the next, so each line stands alone (WrapAnsi's rule).
func TestLayoutHelpBalancesTagsAtEachBreak(t *testing.T) {
	out := LayoutHelp(`<ansi fg="white">aaa bbb ccc</ansi>`, 7)
	lines := strings.Split(out, "\n")
	assert.Len(t, lines, 2)
	for _, l := range lines {
		assert.Equal(t, strings.Count(l, "<ansi"), strings.Count(l, "</ansi>"), l)
		assert.LessOrEqual(t, util.VisibleWidth(l), 7, l)
	}
}

// The spell page's labels carry their padding inside the colour span.
func TestLayoutHelpRowLabelPaddedInsideItsSpan(t *testing.T) {
	in := "::rows\n" + `<ansi fg="yellow">SpellID:     </ansi> <ansi fg="white-bold">blood-boil</ansi>` + "\n::"
	out := LayoutHelp(in, 30)
	assert.Equal(t, "SpellID: blood-boil", ansiTag.ReplaceAllString(out, ""))
}

// Directive lines never reach the reader, even an unclosed block.
func TestLayoutHelpDropsMarkers(t *testing.T) {
	out := LayoutHelp("::keep\n  x\n", 30)
	assert.NotContains(t, out, "::")
}
