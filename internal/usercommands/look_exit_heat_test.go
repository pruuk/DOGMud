package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// Lighting plan 6, owner ruling O6: `look <exit>` with infravision, when the
// light here is too poor to see through the exit, shows the next room's
// occupants as the roster's anonymous figures: never a name, the room's
// title, description or items. With light enough to see through, the look is
// the ordinary one, names and all.
//
// The fixture (seedDarknessGateRoom) puts Aliceia alone in the Dark Cave at a
// pinned lamp; south is the Town Square, lit at 60, holding Bobrick and the
// Skeleton.
func TestLookExit_HeatShowsFiguresThroughAnExit(t *testing.T) {
	cases := []struct {
		name      string
		lamp      int
		condition int
		want      []string
		wantNot   []string
	}{
		{
			name: "infravision, too dark to see through: two figures by heat",
			lamp: 30, condition: gateInfraConditionId,
			want:    []string{"You peer toward the south.", "but you sense the warmth of:", "a figure</ansi>, <ansi fg=\"combat-anon\">a figure"},
			wantNot: []string{"Bobrick", "Skeleton", "Town Square", "bustling", "too dark to see anything in that direction"},
		},
		{
			name: "normal eyes, too dark to see through: nothing",
			lamp: 30, condition: 0,
			want:    []string{"too dark to see anything in that direction"},
			wantNot: []string{"Bobrick", "Skeleton", "a figure", "warmth"},
		},
		{
			name: "infravision, light enough to see through: the ordinary look",
			lamp: 90, condition: gateInfraConditionId,
			// The test world renders no room template, so the ordinary look
			// is told by its peer line and the room-description block.
			want:    []string{"You peer toward the south.", "room-description"},
			wantNot: []string{"warmth", "too dark", "a figure"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			user, room := seedDarknessGateRoom(t, c.lamp)
			if c.condition != 0 {
				require.True(t, user.Character.Conditions.AddCondition(c.condition, true))
			}
			out := runGate(t, user, func() (bool, error) { return Look("south", user, room, 0) })
			for _, w := range c.want {
				require.Contains(t, out, w)
			}
			for _, w := range c.wantNot {
				require.NotContains(t, out, w)
			}
		})
	}
}

// An empty next room reads as empty to heat, not as too dark.
func TestLookExit_HeatInAnEmptyRoomFindsNothingWarm(t *testing.T) {
	user, room := seedDarknessGateRoom(t, 30)
	require.True(t, user.Character.Conditions.AddCondition(gateInfraConditionId, true))
	out := runGate(t, user, func() (bool, error) { return Look("north", user, room, 0) })
	// The Dark Cave has only a south exit; north is not an exit at all.
	require.NotContains(t, out, "warmth")

	square := rooms.LoadRoom(1)
	require.NotNil(t, square)
	for _, id := range square.GetPlayers(rooms.FindAll) {
		square.RemovePlayer(id)
	}
	for _, id := range square.GetMobs(rooms.FindAll) {
		square.RemoveMob(id)
	}
	out = runGate(t, user, func() (bool, error) { return Look("south", user, room, 0) })
	require.Contains(t, out, "nothing warm moves there")
	require.NotContains(t, out, "a figure")
}
