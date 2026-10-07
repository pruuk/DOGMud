package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/users"
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
			want:    []string{"You peer toward the south.", "but you sense the warmth of <ansi", "a figure</ansi>, <ansi fg=\"combat-anon\">a figure"},
			wantNot: []string{"Bobrick", "Skeleton", "Town Square", "bustling", "too dark to see anything in that direction", "warmth of:"},
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

// An empty next room reads as empty to heat, not as too dark: the same exit,
// first with its two occupants (two figures), then emptied.
func TestLookExit_HeatInAnEmptyRoomFindsNothingWarm(t *testing.T) {
	user, room := seedDarknessGateRoom(t, 30)
	require.True(t, user.Character.Conditions.AddCondition(gateInfraConditionId, true))
	out := runGate(t, user, func() (bool, error) { return Look("south", user, room, 0) })
	require.Equal(t, 2, strings.Count(out, "a figure"), "fixture: the square's two occupants by heat; got:\n%s", out)

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

// Heat counts whom the square's roster would list (actions.FiguresSensedIn,
// the filter scan shares): a hidden mob and a sneaking player the looker does
// not perceive are left out. A blinded looker senses nothing; a looker whose
// own room is unlit but who sees there by heat (light 0, reach 30: shapes)
// still senses through the exit.
func TestLookExit_HeatCountsTheRostersOccupants(t *testing.T) {
	hide := func(t *testing.T, c *characters.Character) {
		t.Helper()
		if c.Awareness == nil {
			c.Awareness = awareness.NewMachine()
		}
		r := state.TransitionReason{Trigger: "look_exit_heat_test"}
		require.NoError(t, c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, r))
		c.Awareness.ResolveConcealment(true, r)
		require.True(t, c.IsHidden(), "fixture: %s must be hidden", c.Name)
	}
	squareOccupants := func(t *testing.T) (*characters.Character, *characters.Character) {
		t.Helper()
		square := rooms.LoadRoom(1)
		require.NotNil(t, square)
		var player, mob *characters.Character
		for _, id := range square.GetPlayers(rooms.FindAll) {
			if u := users.GetByUserId(id); u != nil {
				player = u.Character
			}
		}
		for _, id := range square.GetMobs(rooms.FindAll) {
			if m := mobs.GetInstance(id); m != nil {
				mob = &m.Character
			}
		}
		require.NotNil(t, player, "fixture: a player in the square")
		require.NotNil(t, mob, "fixture: a mob in the square")
		return player, mob
	}

	t.Run("a hidden mob and an unperceived sneaking player are left out", func(t *testing.T) {
		user, room := seedDarknessGateRoom(t, 30)
		require.True(t, user.Character.Conditions.AddCondition(gateInfraConditionId, true))
		player, mob := squareOccupants(t)
		hide(t, mob)
		out := runGate(t, user, func() (bool, error) { return Look("south", user, room, 0) })
		require.Equal(t, 1, strings.Count(out, "a figure"), "hidden mob left out; got:\n%s", out)
		hide(t, player)
		out = runGate(t, user, func() (bool, error) { return Look("south", user, room, 0) })
		require.Contains(t, out, "nothing warm moves there")
		require.NotContains(t, out, "a figure")
	})

	t.Run("a blinded looker with infravision senses nothing", func(t *testing.T) {
		user, room := seedDarknessGateRoom(t, 30)
		require.True(t, user.Character.Conditions.AddCondition(gateInfraConditionId, true))
		user.Character.Perception = characters.New().Perception
		require.NoError(t, user.Character.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}))
		out := runGate(t, user, func() (bool, error) { return Look("south", user, room, 0) })
		require.Contains(t, out, blindLookLine)
		require.NotContains(t, out, "warmth")
		require.NotContains(t, out, "a figure")
	})

	t.Run("a looker in an unlit room who sees by heat senses through", func(t *testing.T) {
		user, room := seedDarknessGateRoom(t, 0)
		require.True(t, user.Character.Conditions.AddCondition(gateInfraConditionId, true))
		out := runGate(t, user, func() (bool, error) { return Look("south", user, room, 0) })
		require.Contains(t, out, "but you sense the warmth of <ansi")
		require.Equal(t, 2, strings.Count(out, "a figure"), "got:\n%s", out)
		require.NotContains(t, out, "Bobrick")
		require.NotContains(t, out, "Skeleton")
	})
}
