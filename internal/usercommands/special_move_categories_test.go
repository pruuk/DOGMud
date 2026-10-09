package usercommands

import (
	"os"
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

// systemNoticeMarkers are the only things a special-move file may still send
// as CategorySystem: refusals and cost notices, never narration. Matched
// against a SendText line joined with the line after it.
var systemNoticeMarkers = []string{
	"CostRefusalText", "focused on your work", "need a moment", "no target",
	"already rallied", "still echoes", "Throw what?", "to throw.", "isn't something you can throw",
	"crumbles apart", "ChannelDefenceShortageText", "text)",
}

// #449 siblings: taunt, throw, rally and warcry kept narration on
// CategorySystem. A source-level pin, like the other guards in this package:
// any SendText on CategorySystem in these files must be a refusal or notice.
func TestSpecialMoveFilesKeepOnlyNoticesOnSystem(t *testing.T) {
	for _, file := range []string{"taunt.go", "throw.go", "rally.go", "warcry.go"} {
		raw, err := os.ReadFile(file)
		require.NoError(t, err, file)
		lines := strings.Split(string(raw), "\n")
		for i, line := range lines {
			if !strings.Contains(line, "messaging.CategorySystem") {
				continue
			}
			joined := line
			if i+1 < len(lines) {
				joined += lines[i+1]
			}
			ok := false
			for _, marker := range systemNoticeMarkers {
				if strings.Contains(joined, marker) {
					ok = true
					break
				}
			}
			require.True(t, ok, "%s:%d narration still on CategorySystem: %s", file, i+1, strings.TrimSpace(line))
		}
	}
}

func TestTauntAndThrowNarrationCategories(t *testing.T) {
	read := func(name string) string {
		raw, err := os.ReadFile(name)
		require.NoError(t, err)
		return string(raw)
	}
	taunt := read("taunt.go")
	for _, want := range []string{"user.SendText(cat, atkMsg)", "targetPlayer.SendText(cat, defMsg)",
		"cat = messaging.CategoryTauntResist", "cat = messaging.CategoryTauntFailure",
		"user.SendText(messaging.CategoryTauntSuccess, fmt.Sprintf(pullMsgs"} {
		require.Contains(t, taunt, want)
	}
	throw := read("throw.go")
	for _, want := range []string{"The explosion sears you!", "The explosion catches", "is caught in the blast!",
		"shatters harmlessly", "Your throw strikes true!", "Your throw catches multiple targets!"} {
		idx := strings.Index(throw, want)
		require.GreaterOrEqual(t, idx, 0, want)
		start := strings.LastIndex(throw[:idx], "SendText(")
		require.Contains(t, throw[start:idx], "messaging.CategoryHitRanged", "%s must ride CategoryHitRanged", want)
	}
}
