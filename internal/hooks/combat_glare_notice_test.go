package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// File: combat_glare_notice_test.go
//
// #319. A dazzled fighter sees every face (SightFull), so the blind notice
// never fires for them, yet glare lowers their SightMult all the same. They
// are told once per fight, through the same production seam as the blind
// notice (dispatchCritAndMessaging marks, flushGlareCombatNotices sends).

// glareNoticeNeedle is a substring unique to glareCombatNoticeText.
const glareNoticeNeedle = "glare is too bright"

// dimGlareLamp is a lamp level below the dim edge, where ordinary eyes pay
// SightMult for the dark ramp and the bright fraction is 0.
const dimGlareLamp = 30

type glareScene struct {
	u1       *users.UserRecord
	atk, def actions.Actor
	captured *[]events.Message
}

// newGlareScene puts user 1 in combat with mob 100 in skyless room 2 at a
// pinned lamp, with both notice sets reset.
func newGlareScene(t *testing.T, lamp int) glareScene {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(seedNarrationConditions())
	roundBlindCombatants = map[int]bool{}
	roundGlareCombatants = map[int]bool{}
	glareToldThisFight = map[int]bool{}

	room2 := rooms.LoadRoom(2)
	require.NotNil(t, room2)
	room2.Biome = "cave"
	room2.Lamp = rooms.LampPtr(lamp)
	require.Equal(t, lamp, room2.LightLevel(), "room 2 must sit at the pinned lamp")

	u1 := users.GetByUserId(1)
	require.NotNil(t, u1)
	u1.Character.RoomId = 2
	rooms.LoadRoom(1).RemovePlayer(1)
	room2.AddPlayer(1)

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	u1.Character.SetAggro(0, mob.InstanceId, characters.DefaultAttack)
	require.True(t, u1.Character.IsInCombat(), "fixture: user 1 must be in a fight")
	t.Cleanup(u1.Character.EndAggro)

	captured, capCleanup := captureMessages(t)
	t.Cleanup(capCleanup)
	return glareScene{
		u1:       u1,
		atk:      actions.NewUserActorInRoom(u1, room2),
		def:      actions.NewMobActorInRoom(mob, room2),
		captured: captured,
	}
}

// round runs one combat round's seam and returns the glare notices user 1
// read in it.
func (s glareScene) round() int {
	before := countContaining(textsForUser(*s.captured, 1), glareNoticeNeedle)
	dispatchCritAndMessaging(s.atk, s.def, vbLandingResult())
	flushBlindCombatNotices()
	flushGlareCombatNotices()
	events.ProcessEvents()
	return countContaining(textsForUser(*s.captured, 1), glareNoticeNeedle) - before
}

func TestGlareCombatNotice(t *testing.T) {
	t.Run("a dazzled fighter is told once, on the first round", func(t *testing.T) {
		s := newGlareScene(t, 95)
		room := s.atk.GetRoom()
		require.Equal(t, messaging.SightFull, messaging.ParticipantSight(s.u1.Character, room),
			"precondition: a dazzled fighter sees faces")
		_, bright := messaging.ComfortDistance(s.u1.Character, room)
		require.Greater(t, bright, 0.0, "precondition: the room is past the dazzle ramp's start")
		require.Less(t, messaging.SightMult(s.u1.Character, room), 1.0, "precondition: the glare costs")

		assert.Equal(t, 1, s.round(), "the first round of the fight tells them")
		assert.Equal(t, 0, s.round(), "the second round of the same fight does not")
		assert.Equal(t, 0, countContaining(textsForUser(*s.captured, 1), blindNoticeNeedle),
			"the blind notice stays silent for a fighter who sees clearly")
	})

	t.Run("a later fight tells them again", func(t *testing.T) {
		s := newGlareScene(t, 95)
		require.Equal(t, 1, s.round())

		// The fight ends: the round closes with the player out of combat.
		s.u1.Character.EndAggro()
		require.False(t, s.u1.Character.IsInCombat())
		flushGlareCombatNotices()

		s.u1.Character.SetAggro(0, 100, characters.DefaultAttack)
		assert.Equal(t, 1, s.round(), "a new fight is told afresh")
	})

	t.Run("a fighter at full sight with no glare is never told", func(t *testing.T) {
		s := newGlareScene(t, 60)
		require.Equal(t, messaging.SightFull, messaging.ParticipantSight(s.u1.Character, s.atk.GetRoom()))
		require.Equal(t, 1.0, messaging.SightMult(s.u1.Character, s.atk.GetRoom()),
			"precondition: a comfortable room costs nothing")
		assert.Equal(t, 0, s.round())
		assert.Equal(t, 0, s.round())
	})

	// SightMult falls for a dark ramp too, but the line blames glare, so a
	// dim room's cost must never send it. Only the bright fraction tells the
	// two apart. Through the round seam the dark case cannot reach
	// markGlareCombatant with a clear-sight verdict today: SightFull needs the
	// light at or above the dim edge, where the dark fraction is 0. So the
	// test hands markGlareCombatant the verdict directly, pinning its own
	// guard for the day a caller's verdict and the room's ramp disagree, and
	// then checks the seam stays silent too.
	t.Run("a fighter in a dim room is never told of glare", func(t *testing.T) {
		s := newGlareScene(t, dimGlareLamp)
		room := s.atk.GetRoom()
		_, bright := messaging.ComfortDistance(s.u1.Character, room)
		require.Equal(t, 0.0, bright, "precondition: the dim room is nowhere near too bright")
		require.Less(t, messaging.SightMult(s.u1.Character, room), 1.0,
			"precondition: the dim room costs sight, or this proves nothing about the bright check")

		markGlareCombatant(s.atk, true)
		flushGlareCombatNotices()
		events.ProcessEvents()
		assert.Equal(t, 0, countContaining(textsForUser(*s.captured, 1), glareNoticeNeedle),
			"a fighter in a dim room was told the glare was too bright")
		assert.Equal(t, 0, s.round(), "the round seam must stay silent in a dim room as well")
	})

	t.Run("light verbosity suppresses it like the blind notice", func(t *testing.T) {
		s := newGlareScene(t, 95)
		orig := s.u1.CombatVerbosity
		t.Cleanup(func() { s.u1.CombatVerbosity = orig })
		s.u1.CombatVerbosity = "light"
		assert.Equal(t, 0, s.round())
		s.u1.CombatVerbosity = "medium"
		assert.Equal(t, 1, s.round(), "medium keeps the notice, and a suppressed one did not spend it")
	})
}
