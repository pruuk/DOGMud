package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// Sight gates close-out, #274 (owner R3, 2026-10-08): a hidden actor does not
// emote. A hidden mob's behaviour-tree greet ("A Liveried Footman pays you
// little mind.") gave it away; the same path carries a hidden player's emote.
// Silence for every listener, not an anonymous line.
func TestSendSeen_HiddenActorReachesNoOne(t *testing.T) {
	cases := []struct {
		name   string
		player bool
		cat    messaging.Category
		line   string
	}{
		{"hidden player", true, messaging.CategoryEmote,
			FormatEmoteText("Kesh", "waves.", "username")},
		{"hidden mob", false, messaging.CategoryMobEmote,
			FormatMobEmoteText("Grel", "pays you little mind.")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sc := newSpeechScene(t)
			hideForMove(t, sc.player.Character)
			hideForMove(t, &sc.mob.Character)
			checkSpeech(t, sc, c.player, func(a Actor) { SendSeen(a, c.cat, c.line, false) },
				speechWant{})
		})
	}
}
