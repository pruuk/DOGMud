package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/stretchr/testify/require"
)

// TestPartyListShowsNoRank: the party table's Position column printed a rank
// (front, middle, back) that no command could change (#295). With the rank
// feature hidden until positioning is designed (#600), the table must not
// show it.
func TestPartyListShowsNoRank(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	useDogmudTemplates(t)

	user, _ := getTestUserAndRoom(t)
	p := parties.New(user.UserId)
	require.NotNil(t, p)
	defer p.Disband()

	msgs, stop := captureAllMessages(t)
	cmdPartyList(user, p)
	stop()
	var all strings.Builder
	for _, m := range *msgs {
		if m.UserId == user.UserId {
			all.WriteString(m.Text)
		}
	}
	out := all.String()
	require.Contains(t, out, "Party Members", "the party table must have rendered")
	require.NotContains(t, out, "Position", "the table must not carry the rank column")
	require.NotContains(t, out, "middle", "no rank may print")
}
