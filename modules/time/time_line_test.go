package time

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/gametime"
)

var timeLineTags = regexp.MustCompile(`<[^>]*>`)

// #382 playtest: the time line read "It is daytime" at dusk and had no final
// full stop.
func TestTimeLine_NamesThePeriodAndEndsWithAFullStop(t *testing.T) {
	gd := gametime.GameDate{Year: 3, Month: 2, Day: 40, Hour: 4, Minute: 12, AmPm: "PM"}
	cases := map[string]string{
		"day":   "It is daytime on day 40",
		"night": "It is nighttime on day 40",
		"dusk":  "It is dusk on day 40",
		"dawn":  "It is dawn on day 40",
	}
	for period, want := range cases {
		plain := timeLineTags.ReplaceAllString(timeLine(gd, period), "")
		if !strings.Contains(plain, want) {
			t.Errorf("period %q: %q does not contain %q", period, plain, want)
		}
		if !strings.HasSuffix(plain, ".") {
			t.Errorf("period %q: %q lacks a final full stop", period, plain)
		}
	}
}
