package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #242: a mob's spell-channel lines went out on plain Room.SendText, the
// unfiltered audio channel, so a shapes-only or blind observer read the
// caster's name. Owner ruling R4 (2026-10-08): the disruptions (concentration
// breaks, fizzles, falters) have a sound line for a reader who sees nothing;
// the quiet weave and focus-shift lines are sight-only.
//
// seedFallbackRoom (dark_room_fallback_sight_test.go) puts users 1 and 2 in
// cave room 2 at a pinned lamp and gives user 1 the named eyes. At lamp 10
// user 1 with heat eyes reads shapes and user 2 reads nothing; at lamp 60
// both read faces.

func spellChannelMob(t *testing.T) *mobs.Mob {
	t.Helper()
	m := mobs.GetInstance(100)
	require.NotNil(t, m)
	return m
}

func TestMobSpellDisruption_ShapesReadAFigure_BlindHearTheSound(t *testing.T) {
	cases := []struct {
		name  string
		send  func(*mobs.Mob, *rooms.Room)
		seen  string
		sound string
	}{
		{"concentration breaks", sendMobConcentrationBroke, "concentration breaks", messaging.SoundChantBreaksOff},
		{"spell fizzles", func(m *mobs.Mob, r *rooms.Room) { sendMobSpellFailed(m, r, "fizzles") }, "spell fizzles", messaging.SoundSpellSputtersOut},
		{"spell falters", func(m *mobs.Mob, r *rooms.Room) { sendMobSpellFailed(m, r, "falters") }, "spell falters", messaging.SoundSpellSputtersOut},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			room := seedFallbackRoom(t, 10, heatEyesConditionId)
			require.Equal(t, messaging.SightShapes, messaging.ParticipantSight(users.GetByUserId(1).Character, room))
			require.Equal(t, messaging.SightNone, messaging.ParticipantSight(users.GetByUserId(2).Character, room))

			tc.send(spellChannelMob(t), room)

			shapes, blind := drainPlain(1), drainPlain(2)
			require.Equal(t, 1, countContaining(shapes, tc.seen), "shapes reader sees it: %v", shapes)
			require.Zero(t, countContaining(shapes, "Skeleton"), "but not whose: %v", shapes)
			require.Zero(t, countContaining(shapes, tc.sound))
			require.Equal(t, 1, countContaining(blind, tc.sound), "blind reader hears it: %v", blind)
			require.Zero(t, countContaining(blind, "Skeleton"), "%v", blind)
		})
	}
}

func TestMobSpellChannel_WeaveIsSightOnly(t *testing.T) {
	room := seedFallbackRoom(t, 10, heatEyesConditionId)
	sendMobWeaving(spellChannelMob(t), room)

	shapes, blind := drainPlain(1), drainPlain(2)
	require.Equal(t, 1, countContaining(shapes, "weaves magic"), "%v", shapes)
	require.Zero(t, countContaining(shapes, "Skeleton"), "%v", shapes)
	require.Empty(t, blind, "a reader who sees nothing gets nothing for a quiet weave")
}

func TestMobSpellChannel_LitRoomNamesTheCaster(t *testing.T) {
	room := seedFallbackRoom(t, 60, heatEyesConditionId)
	m := spellChannelMob(t)
	sendMobConcentrationBroke(m, room)
	sendMobWeaving(m, room)
	got := drainPlain(2)
	require.Equal(t, 1, countContaining(got, "Skeleton's concentration breaks."), "%v", got)
	require.Equal(t, 1, countContaining(got, "Skeleton weaves magic"), "%v", got)
}

// The player caster's break (prone, grapple, and the pain of a hit) gets the
// same treatment as the mob's, so mob and player casters read alike. The
// hit path used plain Room.SendText and named the caster to everyone.
func TestPlayerConcentrationBroke_FollowsTheObserversSight(t *testing.T) {
	t.Run("shapes reads a figure", func(t *testing.T) {
		room := seedFallbackRoom(t, 10, heatEyesConditionId)
		caster := users.GetByUserId(2)
		sendPlayerConcentrationBroke(caster, room)
		got := drainPlain(1)
		require.Equal(t, 1, countContaining(got, "concentration breaks"), "%v", got)
		require.Zero(t, countContaining(got, caster.Character.Name), "%v", got)
		require.Empty(t, drainPlain(2), "the caster reads its own line, not the room's")
	})

	t.Run("sees nothing, hears the chant break off", func(t *testing.T) {
		room := seedFallbackRoom(t, 0, nightEyesConditionId)
		caster := users.GetByUserId(2)
		sendPlayerConcentrationBroke(caster, room)
		got := drainPlain(1)
		require.Equal(t, 1, countContaining(got, messaging.SoundChantBreaksOff), "%v", got)
		require.Zero(t, countContaining(got, caster.Character.Name), "%v", got)
	})
}

func TestMobShiftsFocus_IsSightOnlyAndHidesBothNames(t *testing.T) {
	room := seedFallbackRoom(t, 10, heatEyesConditionId)
	target := users.GetByUserId(2)
	sendMobShiftsFocus(spellChannelMob(t), room, target)

	shapes, blind := drainPlain(1), drainPlain(2)
	require.Equal(t, 1, countContaining(shapes, "shifts focus"), "%v", shapes)
	require.Zero(t, countContaining(shapes, "Skeleton"), "%v", shapes)
	require.Zero(t, countContaining(shapes, target.Character.Name), "%v", shapes)
	require.Empty(t, blind, "a reader who sees nothing gets nothing for a focus shift")
}

// A hidden mob caster is unseen in its own spell-channel lines by every
// reader, even one who reads faces in a lit room: the line names
// "Something", never the mob, as sendSpoken does for a speaker still hidden
// and as SendSeen's silence does for a hidden emote (#274, owner R3).
func TestMobSpellChannel_HiddenCasterIsNeverNamed(t *testing.T) {
	cases := []struct {
		name string
		send func(*mobs.Mob, *rooms.Room)
		want string
	}{
		{"concentration breaks", sendMobConcentrationBroke, "Something's concentration breaks."},
		{"spell fizzles", func(m *mobs.Mob, r *rooms.Room) { sendMobSpellFailed(m, r, "fizzles") }, "Something's spell fizzles."},
		{"spell falters", func(m *mobs.Mob, r *rooms.Room) { sendMobSpellFailed(m, r, "falters") }, "Something's spell falters."},
		{"weave", sendMobWeaving, "Something weaves magic with focused intent."},
		{"focus shift", func(m *mobs.Mob, r *rooms.Room) { sendMobShiftsFocus(m, r, users.GetByUserId(1)) }, "Something shifts focus to"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			room := seedFallbackRoom(t, 60, heatEyesConditionId)
			require.Equal(t, messaging.SightFull, messaging.ParticipantSight(users.GetByUserId(2).Character, room))
			m := spellChannelMob(t)
			m.Character.Validate()
			reason := state.TransitionReason{Trigger: "spell_channel_hidden_test"}
			require.NoError(t, m.Character.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason))
			m.Character.Awareness.ResolveConcealment(true, reason)
			require.True(t, m.Character.IsHidden())
			events.DrainQueuedMessagesForTest(1)
			events.DrainQueuedMessagesForTest(2)

			tc.send(m, room)

			got := drainPlain(2)
			require.Zero(t, countContaining(got, "Skeleton"), "a faces reader must not see a hidden caster's name: %v", got)
			require.Equal(t, 1, countContaining(got, tc.want), "%v", got)
		})
	}
}
