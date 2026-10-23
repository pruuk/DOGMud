package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/ferry"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func TestBoardFerry_RegisteredInActionRegistry(t *testing.T) {
	if _, ok := actionRegistry["board_ferry"]; !ok {
		t.Fatal(`board_ferry not in actionRegistry`)
	}
}

const (
	ferryDockRoom   = 7201
	ferryMobId      = 9571
	ferryInstanceId = 95711
	ferryUserId     = 1
)

// seedFerryScene seeds the agent and a player at the dock, and swaps the
// boarding flow for a recorder of the route it was asked to board.
func seedFerryScene(t *testing.T) *[]string {
	t.Helper()
	t.Cleanup(seedTestRoom(t, ferryDockRoom, "TestZone"))
	t.Cleanup(seedTestMob(t, ferryMobId, ferryInstanceId, ferryDockRoom, "the ferry agent"))
	t.Cleanup(seedTestUser(t, ferryUserId, "rider", "Rider", ferryDockRoom))

	boarded := &[]string{}
	orig := boardFerry
	boardFerry = func(user *users.UserRecord, mob *mobs.Mob, roomId int, routeId string) ferry.BoardResult {
		*boarded = append(*boarded, routeId)
		return ferry.BoardOk
	}
	t.Cleanup(func() { boardFerry = orig })
	return boarded
}

func ferryParams(destination string) map[string]any {
	p := map[string]any{
		"routes": map[string]any{
			"plymouth":   "stillwater_np_packet",
			"confluence": "stillwater_confluence_barge",
		},
	}
	if destination != "" {
		p["destination"] = destination
	}
	return p
}

func ferryCtx(eventType, text string) *EvalContext {
	return &EvalContext{
		Event:      EventContext{EventType: eventType, UserId: ferryUserId, Text: text, RoomId: ferryDockRoom},
		InstanceId: ferryInstanceId,
		MobId:      ferryMobId,
		RoomId:     ferryDockRoom,
	}
}

// R14 (#375): under player_topic, Text is a bare topic key, so the route
// comes from the destination param, not from scanning the text.
func TestBoardFerry_DestinationParamPicksTheRoute(t *testing.T) {
	boarded := seedFerryScene(t)
	require.Equal(t, Success, actBoardFerry(ferryParams("confluence"), ferryCtx("player_topic", "to_confluence")))
	require.Equal(t, []string{"stillwater_confluence_barge"}, *boarded)
}

func TestBoardFerry_DestinationParamWinsOverText(t *testing.T) {
	boarded := seedFerryScene(t)
	require.Equal(t, Success, actBoardFerry(ferryParams("confluence"), ferryCtx("player_topic", "plymouth")))
	require.Equal(t, []string{"stillwater_confluence_barge"}, *boarded)
}

func TestBoardFerry_UnknownDestinationFails(t *testing.T) {
	boarded := seedFerryScene(t)
	require.Equal(t, Failure, actBoardFerry(ferryParams("atlantis"), ferryCtx("player_topic", "to_atlantis")))
	require.Empty(t, *boarded, "an unknown destination boards nothing")
}

// The text path matches a destination keyword in Event.Text. The action
// ignores the event type, so the test uses a neutral one: no live mob event
// carries the player's own words since #375 retired player_ask.
func TestBoardFerry_TextPathStillWorks(t *testing.T) {
	boarded := seedFerryScene(t)
	require.Equal(t, Success, actBoardFerry(ferryParams(""), ferryCtx("test_event", "passage to plymouth")))
	require.Equal(t, []string{"stillwater_np_packet"}, *boarded)
}
