package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Empathic Shroud is a real hide (#444, owner 2026-10-09). These tests run
// with the Awareness cascade wired (Awareness_Cascades.go), which the
// characters package tests cannot.

const shroudTestStartLine = "seems to shimmer and fade from view"

// shroudLines is everything queued for userId, tags stripped, as one string.
func shroudLines(userId int) string { return strings.Join(drainPlain(userId), "\n") }

// shroudFixture seeds records 9 and 31 with their shipped flags and text,
// puts users 1 and 2 in cave room 2 under a lamp of 90, and returns them
// visible. User 1's cascade is proven wired: a sneak hide adds record 9 and
// its reveal cancels it.
func shroudFixture(t *testing.T) (holder, other *users.UserRecord, room *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		9: {ConditionId: 9, Name: "Hidden",
			Flags:         []conditions.Flag{conditions.Hidden, conditions.CancelIfCombat},
			StartRoomText: "{actee_plain} disappears into the shadows.",
			EndRoomText:   "{actee_plain} emerges from the shadows."},
		conditions.ConditionIdEmpathicShroud: {ConditionId: conditions.ConditionIdEmpathicShroud, Name: "Empathic Shroud",
			TriggerCount: 16, RoundInterval: 1,
			Flags:         []conditions.Flag{conditions.Hidden, conditions.CancelIfCombat},
			StartRoomText: "{actee} seems to shimmer and fade from view.",
			EndRoomText:   "{actee} shimmers back into view."},
	}))
	room = rooms.LoadRoom(2)
	require.NotNil(t, room)
	room.Biome = "cave"
	room.Lamp = rooms.LampPtr(90)
	holder, other = users.GetByUserId(1), users.GetByUserId(2)
	for _, u := range []*users.UserRecord{holder, other} {
		rooms.LoadRoom(1).RemovePlayer(u.UserId)
		u.Character.RoomId = 2
		room.AddPlayer(u.UserId)
		require.NoError(t, u.Character.Validate()) // wires the cascades once
		u := u
		t.Cleanup(func() {
			u.Character.Awareness.ForceVisible(state.TransitionReason{Trigger: "test cleanup"})
		})
	}

	// Probe: the cascade must be live, or "no record 9" below proves nothing.
	r := state.TransitionReason{Trigger: "test"}
	require.NoError(t, holder.Character.Awareness.TransitionToConcealing(awareness.ConcealingData{}, r))
	holder.Character.Awareness.ResolveConcealment(true, r)
	require.NotEmpty(t, holder.Character.GetConditions(9), "fixture: the Hidden cascade must add record 9")
	require.NoError(t, holder.Character.Awareness.TransitionToRevealing(r))
	require.Empty(t, holder.Character.GetConditions(9), "fixture: the reveal cascade must cancel record 9")
	holder.Character.Conditions.Prune()

	for _, uid := range []int{1, 2} {
		events.DrainQueuedMessagesForTest(uid)
		events.DrainQueuedConditionsForTest(uid) // left by earlier tests in the package
	}
	return holder, other, room
}

// The spell stamps the caster's score on record 31; landing, it hides the
// holder at that score with no record 9.
func TestShroudSpell_HidesAtTheCastersScoreWithNoRecord9(t *testing.T) {
	holder, caster, _ := shroudFixture(t)
	caster.Character.SetSkill("spellcasting", 5)
	spell := &spells.SpellData{SpellId: "empathic-shroud", Name: "Empathic Shroud", PrimaryStat: "willpower",
		EffectType: "condition", ConditionIds: []int{conditions.ConditionIdEmpathicShroud}}
	want := characters.ShroudScore(spell.CasterStatValue(caster.Character.Stats), caster.Character.GetSkillLevel(skills.Spellcasting))
	require.Positive(t, want)

	applySpellCondition(holder, spell, caster.Character, conditions.ConditionIdEmpathicShroud, state.ActorRef{}, false)
	q := events.DrainQueuedConditionsForTest(holder.UserId)
	require.Len(t, q, 1)
	require.Equal(t, want, q[0].Magnitude, "the record carries the CASTER's score")
	require.Zero(t, q[0].Triggers, "the authored duration stands")

	require.Equal(t, events.Continue, ApplyConditions(q[0]))
	require.True(t, holder.Character.IsHidden())
	require.True(t, holder.Character.HiddenByShroud())
	require.Equal(t, want, holder.Character.HideBaseScore())
	require.Empty(t, holder.Character.GetConditions(9), "a shroud hide carries no record 9")
	require.Contains(t, shroudLines(caster.UserId), shroudTestStartLine, "the room watched the holder fade")
}

// Breaking: an observer spotting the hide, logging out, and the combat strip
// all reveal it, and record 31 ends with the hide.
func TestShroudHide_RevealCancelsRecord31(t *testing.T) {
	cases := map[string]func(u *users.UserRecord){
		"spotted": func(u *users.UserRecord) {
			_ = u.Character.Awareness.TransitionToRevealing(state.TransitionReason{Trigger: awareness.TriggerObserverSearch})
		},
		"logout": func(u *users.UserRecord) {
			onPlayerDespawnForAwareness(events.PlayerDespawn{UserId: u.UserId})
		},
		"combat strip": func(u *users.UserRecord) {
			u.Character.CancelCombatConditions()
		},
	}
	for name, reveal := range cases {
		t.Run(name, func(t *testing.T) {
			holder, _, _ := shroudFixture(t)
			require.NoError(t, holder.Character.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 150, "spell"))
			require.True(t, holder.Character.HiddenByShroud())

			reveal(holder)

			require.False(t, holder.Character.IsHidden())
			require.Empty(t, holder.Character.GetConditions(conditions.ConditionIdEmpathicShroud), "record 31 must end with the hide")
		})
	}
}

// A stronger shroud taking over a sneak hide: the holder was already hidden,
// so nobody watches them fade, and the room line is not told.
func TestShroudOverSneak_TellsTheRoomNothing(t *testing.T) {
	holder, other, _ := shroudFixture(t)
	r := state.TransitionReason{Trigger: "test"}
	require.NoError(t, holder.Character.Awareness.TransitionToConcealing(awareness.ConcealingData{}, r))
	holder.Character.Awareness.ResolveConcealment(true, r)
	require.NotEmpty(t, holder.Character.GetConditions(9))
	events.DrainQueuedMessagesForTest(other.UserId)

	holder.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 9999, "spell")
	q := events.DrainQueuedConditionsForTest(holder.UserId)
	require.Len(t, q, 1)
	require.Equal(t, events.Continue, ApplyConditions(q[0]))

	require.True(t, holder.Character.HiddenByShroud(), "the stronger shroud took over")
	require.False(t, holder.Character.Conditions.HasCondition(9), "record 9 is discarded, so no end line")
	require.NotContains(t, shroudLines(other.UserId), shroudTestStartLine,
		"a holder already hidden fades in front of nobody")
}

// A shroud hide carries no record 9, even on a holder whose own sneak base
// beats the shroud's score. With a weaker sneak base a stray 9 from the
// cascade would lose the stronger-hide contest and be discarded, which would
// hide the bug; here it would win and take the hide over.
func TestShroudHide_NoRecord9EvenWhenTheSneakBaseIsStronger(t *testing.T) {
	holder, _, _ := shroudFixture(t)
	holder.Character.Stats.Dexterity.Base = 500
	require.NoError(t, holder.Character.Validate())
	require.Greater(t, holder.Character.SneakBaseScore(), 50.0, "fixture: the sneak base must beat the shroud")

	require.NoError(t, holder.Character.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 50, "spell"))
	require.True(t, holder.Character.HiddenByShroud())
	require.Equal(t, 50.0, holder.Character.HideBaseScore())
	require.Empty(t, holder.Character.GetConditions(9), "a shroud hide carries no record 9")
	require.NotEmpty(t, holder.Character.GetConditions(conditions.ConditionIdEmpathicShroud))
}

// runOutShroud ticks c's record 31 through every one of its triggers, as the
// round ticks do, so the next prune removes it.
func runOutShroud(t *testing.T, c *characters.Character) {
	t.Helper()
	recs := c.GetConditions(conditions.ConditionIdEmpathicShroud)
	require.Len(t, recs, 1)
	for recs[0].TriggersLeft > 0 {
		c.Conditions.Trigger()
	}
}

// Running out: the round prune (PruneConditions) removes the spent 31 and
// validates, which reveals the holder, a player or a mob, and tells the room
// 31's end line.
func TestShroudHide_RunsOutThroughThePrunePass(t *testing.T) {
	t.Run("player", func(t *testing.T) {
		holder, other, room := shroudFixture(t)
		// The prune walks rooms the manager lists as holding players.
		rooms.MarkRoomOccupancy(room.RoomId, 2, 0)
		require.NoError(t, holder.Character.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 150, "spell"))
		require.True(t, holder.Character.HiddenByShroud())
		events.DrainQueuedMessagesForTest(other.UserId)

		runOutShroud(t, holder.Character)
		require.True(t, holder.Character.IsHidden(), "fixture: still hidden until the prune")
		PruneConditions(events.NewTurn{TurnNumber: 1})

		require.False(t, holder.Character.IsHidden(), "the hide ends with its record")
		require.Contains(t, shroudLines(other.UserId), "shimmers back into view")
	})
	t.Run("mob", func(t *testing.T) {
		shroudFixture(t)
		m := mobs.GetInstance(100)
		require.NotNil(t, m)
		require.NoError(t, m.Character.Validate())
		t.Cleanup(func() {
			m.Character.Awareness.ForceVisible(state.TransitionReason{Trigger: "test cleanup"})
		})
		require.NoError(t, m.Character.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 150, "spell"))
		require.True(t, m.Character.HiddenByShroud())

		runOutShroud(t, &m.Character)
		require.True(t, m.Character.IsHidden(), "fixture: still hidden until the prune")
		PruneConditions(events.NewTurn{TurnNumber: 1})

		require.False(t, m.Character.IsHidden(), "the hide ends with its record")
		require.Empty(t, m.Character.GetConditions(conditions.ConditionIdEmpathicShroud))
	})
}

// Every exit from Hidden clears the sneak's "sneaking" misc flag. Left set,
// usercommands.Go reads it and moves a visible player as a sneaker.
func TestLeavingHidden_ClearsTheSneakingFlag(t *testing.T) {
	holder, _, _ := shroudFixture(t)
	r := state.TransitionReason{Trigger: "test"}
	require.NoError(t, holder.Character.Awareness.TransitionToConcealing(awareness.ConcealingData{}, r))
	holder.Character.Awareness.ResolveConcealment(true, r)
	holder.Character.SetMiscData(`sneaking`, true)

	holder.Character.RevealForCombat()
	require.False(t, holder.Character.IsHidden())
	require.Nil(t, holder.Character.GetMiscData(`sneaking`))
}
