package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// vitalsWatchUser seeds one GMCP-negotiated user and clears any record of a
// prior Char.Vitals push to them.
func vitalsWatchUser(t *testing.T, userId int, connId uint64) *users.UserRecord {
	t.Helper()
	u := users.NewTestUser(userId, "watched", "Watched", connId)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{userId: u}))
	gmcpModule.cache.Add(connId, GMCPSettings{GMCPAccepted: true, HelloReceived: true})
	t.Cleanup(func() { gmcpModule.cache.Remove(connId) })
	forgetSentVitals(userId)
	t.Cleanup(func() { forgetSentVitals(userId) })
	events.DrainQueuedVitalsChangedForTest(0)
	t.Cleanup(func() { events.DrainQueuedVitalsChangedForTest(0) })
	return u
}

// A pool that moves with no CharacterVitalsChanged queued (combat damage, a
// spell's cost, stamina regen) must still reach the client within a round.
// Before the watcher the Vitals window froze until the player moved
// (playtest 2026-10-17).
func TestVitalsWatch_PushesAPoolChangeNobodyAnnounced(t *testing.T) {
	u := vitalsWatchUser(t, 9801, 98801)
	g := &GMCPCharModule{}

	// The client has been sent the current state.
	recordSentVitals(u.UserId, buildVitalsPayload(u.Character))

	g.vitalsWatchHandler(events.NewRound{})
	require.Empty(t, events.DrainQueuedVitalsChangedForTest(u.UserId),
		"an unchanged round must push nothing")

	// Stamina spent by an action, conviction by a spell: neither emits an event.
	u.Character.Stamina -= 7
	g.vitalsWatchHandler(events.NewRound{})
	require.Len(t, events.DrainQueuedVitalsChangedForTest(u.UserId), 1,
		"a stamina change must queue one CharacterVitalsChanged")

	// Until the push is actually sent, the change is still pending...
	g.vitalsWatchHandler(events.NewRound{})
	require.Len(t, events.DrainQueuedVitalsChangedForTest(u.UserId), 1)

	// ...and once the Char.Vitals that answers it is sent, it is not repeated.
	recordSentVitals(u.UserId, buildVitalsPayload(u.Character))
	g.vitalsWatchHandler(events.NewRound{})
	require.Empty(t, events.DrainQueuedVitalsChangedForTest(u.UserId))

	u.Character.Conviction -= 3
	u.Character.Health -= 11
	g.vitalsWatchHandler(events.NewRound{})
	require.Len(t, events.DrainQueuedVitalsChangedForTest(u.UserId), 1,
		"health and conviction changes in one round queue one event")
}

// Sending Char.Vitals (alone, or inside a full Char push) is what records
// the baseline, so an event-driven push is not echoed by the next round.
func TestVitalsWatch_SendPathRecordsTheBaseline(t *testing.T) {
	u := vitalsWatchUser(t, 9802, 98802)
	g := &GMCPCharModule{}
	t.Cleanup(func() { events.DrainQueuedByTypeForTest(`GMCPOut`) })

	for _, ident := range []string{`Char.Vitals`, `Char`} {
		forgetSentVitals(u.UserId)
		u.Character.Stamina -= 2

		g.buildAndSendGMCPPayload(GMCPCharUpdate{UserId: u.UserId, Identifier: ident})
		g.vitalsWatchHandler(events.NewRound{})
		require.Empty(t, events.DrainQueuedVitalsChangedForTest(u.UserId),
			"a %s push must count as the client having the current vitals", ident)
	}
}

// A client that has not negotiated GMCP is never pushed.
func TestVitalsWatch_SkipsClientsWithoutGMCP(t *testing.T) {
	u := vitalsWatchUser(t, 9803, 98803)
	gmcpModule.cache.Remove(98803)

	(&GMCPCharModule{}).vitalsWatchHandler(events.NewRound{})
	require.Empty(t, events.DrainQueuedVitalsChangedForTest(u.UserId))
}

// A player who reconnects inside the zombie window is still in their party.
// Their fresh client must be sent the full Party package (with Members), or
// the web client's Vitals window falls back to the solo layout and shows the
// viewer twice (playtest 2026-10-17).
func TestPartySpawn_SendsTheFullPartyToAReturningMember(t *testing.T) {
	leader := users.NewTestUser(9811, "leader", "Leader", 98811)
	member := users.NewTestUser(9812, "member", "Member", 98812)
	loner := users.NewTestUser(9813, "loner", "Loner", 98813)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		9811: leader, 9812: member, 9813: loner,
	}))
	party := parties.New(9811)
	require.NotNil(t, party)
	require.True(t, party.InvitePlayer(9812))
	require.True(t, party.AcceptInvite(9812))
	t.Cleanup(party.Disband)
	events.DrainQueuedByTypeForTest(`GMCPOut`)
	t.Cleanup(func() { events.DrainQueuedByTypeForTest(`GMCPOut`) })

	g := &GMCPPartyModule{}
	g.playerSpawnHandler(events.PlayerSpawn{UserId: 9812})
	g.playerSpawnHandler(events.PlayerSpawn{UserId: 9813})

	assertOnePartyPush := func(when string) {
		t.Helper()
		var sent []GMCPOut
		for _, e := range events.DrainQueuedByTypeForTest(`GMCPOut`) {
			out := e.(GMCPOut)
			require.NotEqual(t, 9813, out.UserId, "%s: a player in no party is sent no Party", when)
			if out.UserId == 9812 {
				sent = append(sent, out)
			}
		}
		require.Len(t, sent, 1, "%s: one Party push to the returning member", when)
		require.Equal(t, `Party`, sent[0].Module)
		p, ok := sent[0].Payload.(GMCPPartyModule_Payload)
		require.True(t, ok, "payload type %T", sent[0].Payload)
		names := []string{}
		for _, m := range p.Members {
			names = append(names, m.Name)
		}
		require.ElementsMatch(t, []string{"Leader", "Member"}, names, "%s: Members must be filled", when)
		require.Equal(t, "Leader", p.Leader)
	}

	assertOnePartyPush("at spawn")

	// The next round re-sends once, for a web client whose GMCP hello raced
	// the login burst, then the pending set is empty.
	g.newRoundHandler(events.NewRound{})
	assertOnePartyPush("next round")
	g.newRoundHandler(events.NewRound{})
	require.Empty(t, events.DrainQueuedByTypeForTest(`GMCPOut`), "the re-push happens once")
}
