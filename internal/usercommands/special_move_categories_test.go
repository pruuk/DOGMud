package usercommands

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/stretchr/testify/require"
)

// #449: a special move sent the player's own lines as CategorySystem, which
// never wraps (messaging.shouldWrap), while the same command's defence lines
// already rode the move's category. Every role now takes the move's
// category, so a long kick line wraps and colours like the room's.
func TestSpecialMovePersonalLinesRideTheMoveCategory(t *testing.T) {
	cases := map[string]moveCategories{
		"bash": bashCategories, "drain": drainCategories, "gore": goreCategories,
		"grapple": grappleCategories, "kick": kickCategories, "maul": maulCategories,
		"pounce": pounceCategories, "rake": rakeCategories, "throttle": throttleCategories,
		"throttle cast interrupt": throttleCastInterruptCategories,
		"throw cast interrupt":    throwCastInterruptCategories,
		"trip":                    tripCategories,
	}
	for name, cats := range cases {
		require.NotEqual(t, messaging.CategorySystem, cats.Observer, "%s: fixture", name)
		require.Equal(t, cats.Observer, cats.Actor, "%s: the actor's own line must ride the move's category", name)
		require.Equal(t, cats.Observer, cats.Actee, "%s: the actee's line must ride the move's category", name)
	}
}

var specialMoveTagPattern = regexp.MustCompile(`<[^>]*>`)

func TestKickActorLineWrapsAt80(t *testing.T) {
	long := `You plant your feet and drive a brutal kick into the goblin's knee, ` +
		`and it buckles sideways with a wet crack that echoes off the walls.`
	require.Greater(t, utf8.RuneCountInString(long), 80, "fixture: the line must be over 80")

	out := messaging.RenderForRecipient(messaging.RenderInput{
		Category: kickCategories.Actor, Text: long, Channel: messaging.ChannelAudio, LineWidth: 80,
	})

	lines := strings.Split(out, "\n")
	require.Greater(t, len(lines), 1, "a long kick line must wrap: %q", out)
	for _, line := range lines {
		visible := specialMoveTagPattern.ReplaceAllString(line, "")
		require.LessOrEqual(t, utf8.RuneCountInString(visible), 80, "line over 80: %q", visible)
	}
}
