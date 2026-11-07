package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Owner ruling 2026-11-07: broadcast is an admin announcement. Every online
// player gets it, deafened players included, in the game text and the
// Broadcasts tab, as "(announcement) <admin>: <message>".
func TestBroadcastIsAnAdminAnnouncementEveryoneGets(t *testing.T) {
	require.True(t, userCommands[`broadcast`].AdminOnly, "only admins post to Broadcasts")

	admin := users.NewTestUser(9811, "calabe", "Calabe", 99811)
	deaf := users.NewTestUser(9812, "deaf", "Deaf", 99812)
	deaf.Deafened = true
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9811: admin, 9812: deaf}))
	events.DrainQueuedBroadcastEventsForTest()
	events.DrainQueuedCommunicationsForTest()

	_, err := Broadcast(`The server restarts in five minutes.`, admin, nil, 0)
	require.NoError(t, err)

	b := events.DrainQueuedBroadcastEventsForTest()
	require.Len(t, b, 1)
	require.Contains(t, b[0].Text, `(announcement)`)
	require.Contains(t, b[0].Text, `Calabe`)
	require.True(t, b[0].IsCommunication && b[0].SourceIsMod, "a mod's communication reaches deafened players")

	comms := events.DrainQueuedCommunicationsForTest()
	require.Len(t, comms, 2, "exactly one Comm per online player")
	got := map[int]string{}
	for _, c := range comms {
		require.Equal(t, `broadcast`, c.CommType)
		got[c.TargetUserId] = c.Name + `: ` + c.Message
	}
	require.Equal(t, `(announcement) Calabe: The server restarts in five minutes.`, got[9812])
	require.Equal(t, `(announcement) Calabe: The server restarts in five minutes.`, got[9811])
}

func TestBroadcastWithNothingToSayPostsNothing(t *testing.T) {
	admin := users.NewTestUser(9813, "calabe2", "Calabe", 99813)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9813: admin}))
	events.DrainQueuedBroadcastEventsForTest()
	events.DrainQueuedMessagesForTest(9813)
	events.DrainQueuedCommunicationsForTest()
	_, err := Broadcast(`   `, admin, nil, 0)
	require.NoError(t, err)
	require.Empty(t, events.DrainQueuedBroadcastEventsForTest())
	require.Empty(t, events.DrainQueuedCommunicationsForTest())
	require.Contains(t, strings.Join(events.DrainQueuedMessagesForTest(9813), ""), `Usage`)
}
