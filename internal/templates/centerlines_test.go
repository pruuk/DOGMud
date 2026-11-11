package templates

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/util"
)

// centerlines centres an alert between the room view's rules (plan 3b-2):
// whole when it fits inside a 2-column margin, else a piece (a sentence, or
// text set apart by a run of spaces) to a line inside a 1-column margin,
// else wrapped; every line padded on the left only.
func TestCenterLines(t *testing.T) {
	const bank = `<ansi fg="yellow-bold">This is a bank!</ansi> Type <ansi fg="command">bank</ansi> to deposit/withdraw.`
	for _, tc := range []struct {
		name, in string
		width    int
		want     []string
	}{
		{"fits on one line", bank, 80, []string{"                 This is a bank! Type bank to deposit/withdraw."}},
		{"a sentence to a line", bank, 42, []string{"             This is a bank!", "      Type bank to deposit/withdraw."}},
		{"a sentence too long for the span wraps the whole", bank, 30, []string{"  This is a bank! Type bank", "     to deposit/withdraw."}},
		{"runs of spaces after a sentence go with the break", "!!!      Fire here!      !!!", 20, []string{"        !!!", "     Fire here!", "        !!!"}},
		{"leading padding is not counted", "      Quake now!", 30, []string{"          Quake now!"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := centerLines(tc.in, tc.width)
			plain := strings.Split(util.StripANSI(TplAnsiParse(out)), "\n")
			if strings.Join(plain, "|") != strings.Join(tc.want, "|") {
				t.Errorf("got\n%q\nwant\n%q", plain, tc.want)
			}
			for _, line := range strings.Split(out, "\n") {
				if w := util.VisibleWidth(line); w > tc.width-2 {
					t.Errorf("line %q is %d wide, past the 1-column margin of %d", line, w, tc.width)
				}
				if strings.Count(line, "<ansi") != strings.Count(line, "</ansi>") {
					t.Errorf("line %q does not close the colours it opens", line)
				}
			}
		})
	}
}
