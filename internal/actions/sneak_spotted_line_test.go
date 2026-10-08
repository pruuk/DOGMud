package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// Sight gates close-out, #215: a failed sneak tells the sneaker who spotted
// them only as far as the SNEAKER's own sight allows. Sneak carries the
// spotter's identity (SpottedBy), not a bare name, and SpottedLine renders it:
// the name at clear sight, "a figure" at shapes, "something" when the sneaker
// sees nothing.
func TestSpottedLine_FollowsSneakerSight(t *testing.T) {
	cases := []struct {
		name  string
		lamp  int
		infra bool
		want  string
	}{
		{"lit room names the spotter", 80, false,
			"You try to blend into the shadows but Watcher notices you."},
		{"heat sight in the dark sees a figure", 0, true,
			"You try to blend into the shadows but a figure notices you."},
		{"pitch dark sees nothing", 0, false,
			"You try to blend into the shadows but something notices you."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newDarkDetectWorld(t, 0.75)
			w.dest.Lamp = rooms.LampPtr(c.lamp)
			_, obs := w.place(t, "player", 9811, "Watcher")
			obs.Stats.Perception.ValueAdj = 1000
			require.True(t, obs.Conditions.AddCondition(hearSuperCond, true), "hears even where it cannot see")
			actor, mc := w.place(t, "player", 9810, "Sneak")
			mc.Stats.Dexterity.ValueAdj = 0
			if c.infra {
				require.True(t, mc.Conditions.AddCondition(hearInfraCond, true))
			}

			got := Sneak(actor)

			require.False(t, got.Success)
			require.NotNil(t, got.SpottedBy)
			require.Equal(t, "Watcher", got.SpottedBy.Name)
			line := SpottedLine(mc, w.dest, got.SpottedBy)
			require.Equal(t, c.want, hearTag.ReplaceAllString(line, ""))
		})
	}
}

// A spotter the sneaker cannot perceive (hidden, no see-hidden) is never
// named, even in a lit room.
func TestSpottedLine_HiddenSpotterIsNotNamed(t *testing.T) {
	w := newDarkDetectWorld(t, 0.75)
	w.dest.Lamp = rooms.LampPtr(80)
	_, obs := w.place(t, "player", 9811, "Watcher")
	hideForMove(t, obs)
	_, mc := w.place(t, "player", 9810, "Sneak")
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(mc, w.dest))

	line := SpottedLine(mc, w.dest, obs)

	require.Equal(t, "You try to blend into the shadows but something notices you.",
		hearTag.ReplaceAllString(line, ""))
}
