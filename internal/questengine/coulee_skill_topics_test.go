package questengine

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
)

// Skill coverage in Pothole Coulee (#375 phase 7 spec, "Skill coverage"): the
// NPCs who teach by talk name each skill's command, and the corrections hold.

// openInfo returns an info topic's text and hints, squashed, and requires it
// to be open to every player (no quest gate).
func openInfo(t *testing.T, dfile, key string) (text, hints string) {
	t.Helper()
	df := loadCouleeTopics(t, dfile)
	tp := df.FindTopic(key)
	require.NotNil(t, tp, "%s has no topic %q", dfile, key)
	require.Equal(t, dialogue.KindInfo, tp.Kind, "%s topic %s", dfile, key)
	require.Empty(t, tp.QuestRequired, "%s topic %s is open to everyone", dfile, key)
	return squash(tp.Text), squash(tp.Hints)
}

func TestVorn9108_TeachesTheFightBeyondTheSwing(t *testing.T) {
	cases := map[string][]string{
		"grapple_help": {"type grapple", "help grapple", "cannot flee"},
		"bash_help":    {"type bash", "help bash", "shield"},
		"kick_help":    {"stomp", "knee", "tailsweep", "help kick"},
		"defence_help": {"dodge", "parry", "block", "help combat"},
	}
	for key, wants := range cases {
		text, hints := openInfo(t, "9108.yaml", key)
		for _, w := range wants {
			require.Contains(t, text+" "+hints, w, "Vorn's %s must say %q", key, w)
		}
	}
}

// Shouts do not come later: rally has no rank gate, and quest 28 already
// teaches warcry (spec "Corrections").
func TestVorn9108_ShoutsAreNotForLater(t *testing.T) {
	text, hints := openInfo(t, "9108.yaml", "shouts_help")
	require.NotContains(t, text+" "+hints, "come to you later")
	require.NotContains(t, text+" "+hints, "come later")
	require.Contains(t, text, "warcry")
	require.Contains(t, hints, "type rally")
}
