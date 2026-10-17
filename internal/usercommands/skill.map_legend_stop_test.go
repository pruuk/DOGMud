package usercommands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

var mapLegendTagRE = regexp.MustCompile(`</?ansi[^>]*>`)

// TestMapLegendEndsWithoutAStop renders the live maps/map template with the
// map command's own frame, sends it as the command does, and checks the
// legend's last line ends on the last name. The 2026-10-17 playtest saw
// "[o City Thoroughfare [v Cliffs [~ Shore." because the normalizer treated
// the map as a sentence.
func TestMapLegendEndsWithoutAStop(t *testing.T) {
	useDogmudTemplates(t)

	legend := map[rune]string{'o': "City Thoroughfare", 'v': "Cliffs", '~': "Shore"}
	lines := []string{"  o-o  ", "  v ~  "}
	mapTxt, err := templates.Process("maps/map", mapScreenData("Thornwall", lines, 7, legend))
	require.NoError(t, err)
	require.Contains(t, mapTxt, "Shore", "the live template must have rendered the legend")

	const userId = 98117
	events.DrainQueuedMessagesForTest(userId)
	sendMapScreen(&users.UserRecord{UserId: userId}, mapTxt)
	got := strings.Join(events.DrainQueuedMessagesForTest(userId), "")

	for _, mode := range []string{"utf-8", "ascii"} {
		text := mapLegendTagRE.ReplaceAllString(got, "")
		if mode == "ascii" {
			text = util.ConvertToAscii(text)
		}
		var last string
		for _, l := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
			if strings.TrimSpace(l) != "" {
				last = strings.TrimRight(l, " ")
			}
		}
		require.Contains(t, last, "[", "%s: the last line is not the legend, so this check proves nothing: %q", mode, last)
		require.False(t, strings.HasSuffix(last, "."), "%s: the legend ends with a stop: %q", mode, last)
	}
}
