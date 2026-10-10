package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #458: a sneak-hidden player quitting read to the room as "Ordel Quist sits
// down and begins to meditate." and "Ordel Quist continues meditating." A
// room line about a condition's holder goes only to the players who perceive
// that holder, by the rule the quit line uses (playersNotPerceiving,
// characters.Character.Perceives). The holder's own lines are unaffected.

const (
	meditateTestId  = 7020 // mirrors shipped condition 0, with an authored end_observer
	seeHiddenTestId = 7021 // grants see-hidden; RoundInterval 0, so it never ticks
)

// hiddenHolderFixture seeds the registries and a condition set holding a
// meditate twin, the two stealth records with their shipped flags and text,
// and a see-hidden grant. Room 1 is lit; users 1 (the holder) and 2 (the
// watcher) and the Skeleton (instance 100) stand in it, all visible, with
// their Awareness cascades wired.
func hiddenHolderFixture(t *testing.T) (holder, watcher *users.UserRecord, m *mobs.Mob) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	rooms.ClearEndLineSnapshots()
	t.Cleanup(rooms.ClearEndLineSnapshots)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		meditateTestId: {ConditionId: meditateTestId, Name: "Test Meditating", RoundInterval: 1, TriggerCount: 5,
			StartUserText:   "You sit down and begin your meditation.",
			StartRoomText:   "{actee} sits down and begins to meditate.",
			TriggerUserText: "You continue your meditation...",
			TriggerRoomText: "{actee} continues meditating.",
			EndUserText:     "Your meditation ends.",
			EndRoomText:     "{actee} stops meditating."},
		9: {ConditionId: 9, Name: "Hidden",
			Flags:         []conditions.Flag{conditions.Hidden, conditions.CancelIfCombat},
			StartRoomText: "{actee_plain} disappears into the shadows.",
			EndRoomText:   "{actee_plain} emerges from the shadows."},
		conditions.ConditionIdEmpathicShroud: {ConditionId: conditions.ConditionIdEmpathicShroud, Name: "Empathic Shroud",
			TriggerCount: 16, RoundInterval: 1,
			Flags:         []conditions.Flag{conditions.Hidden, conditions.CancelIfCombat},
			StartRoomText: "{actee} seems to shimmer and fade from view.",
			EndRoomText:   "{actee} shimmers back into view."},
		seeHiddenTestId: {ConditionId: seeHiddenTestId, Name: "Test True Sight",
			Flags: []conditions.Flag{conditions.SeeHidden}},
	}))
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	holder, watcher = users.GetByUserId(1), users.GetByUserId(2)
	m = mobs.GetInstance(100)
	require.NotNil(t, m)
	for _, c := range []*characters.Character{holder.Character, watcher.Character, &m.Character} {
		require.NoError(t, c.Validate()) // wires the cascades once
		c := c
		t.Cleanup(func() { c.Awareness.ForceVisible(state.TransitionReason{Trigger: "test cleanup"}) })
	}
	for _, uid := range []int{1, 2} {
		events.DrainQueuedMessagesForTest(uid)
		events.DrainQueuedConditionsForTest(uid)
	}
	return holder, watcher, m
}

// sneakHide hides c the way a successful sneak does.
func sneakHide(t *testing.T, c *characters.Character) {
	t.Helper()
	r := state.TransitionReason{Trigger: "hidden_holder_condition_lines_test"}
	require.NoError(t, c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, r))
	c.Awareness.ResolveConcealment(true, r)
	require.True(t, c.IsHidden())
}

// grantSeeHidden gives u see-hidden and proves it perceives c.
func grantSeeHidden(t *testing.T, u *users.UserRecord, c *characters.Character) {
	t.Helper()
	require.True(t, u.Character.Conditions.AddCondition(seeHiddenTestId, true))
	require.True(t, u.Character.Perceives(c))
}

// meditateLines runs the meditate twin's three phases on player holder and
// returns what watcher read and what the holder read.
func meditateLines(t *testing.T, holder, watcher *users.UserRecord) (watched, own []string) {
	t.Helper()
	require.Equal(t, events.Continue, ApplyConditions(events.Condition{UserId: holder.UserId, ConditionId: meditateTestId}))
	UserRoundTick(events.NewRound{RoundNumber: 1})
	expire(t, holder.Character.Conditions.List, meditateTestId)
	PruneConditions(events.NewTurn{TurnNumber: 1})
	return drainPlain(watcher.UserId), drainPlain(holder.UserId)
}

func TestHiddenHolder_PlayerConditionLinesReachOnlyPerceivers(t *testing.T) {
	t.Run("hidden holder, watcher without see-hidden reads nothing", func(t *testing.T) {
		holder, watcher, _ := hiddenHolderFixture(t)
		sneakHide(t, holder.Character)
		require.False(t, watcher.Character.Perceives(holder.Character))
		events.DrainQueuedMessagesForTest(watcher.UserId)

		watched, own := meditateLines(t, holder, watcher)
		assert.Zero(t, countContaining(watched, "Aliceia"), "a hidden holder was named: %v", watched)
		assert.Zero(t, countContaining(watched, "meditat"), "a non-perceiver read a hidden holder's condition line: %v", watched)
		// The holder's own lines are unaffected.
		assert.Equal(t, 1, countContaining(own, "You sit down and begin your meditation."), "%v", own)
		assert.Equal(t, 1, countContaining(own, "You continue your meditation..."), "%v", own)
		assert.Equal(t, 1, countContaining(own, "Your meditation ends."), "%v", own)
	})
	t.Run("visible holder is narrated (null probe)", func(t *testing.T) {
		holder, watcher, _ := hiddenHolderFixture(t)
		watched, _ := meditateLines(t, holder, watcher)
		assert.Equal(t, 1, countContaining(watched, "Aliceia sits down and begins to meditate."), "%v", watched)
		assert.Equal(t, 1, countContaining(watched, "Aliceia continues meditating."), "%v", watched)
		assert.Equal(t, 1, countContaining(watched, "Aliceia stops meditating."), "%v", watched)
	})
	t.Run("hidden holder, see-hidden watcher reads every line", func(t *testing.T) {
		holder, watcher, _ := hiddenHolderFixture(t)
		sneakHide(t, holder.Character)
		grantSeeHidden(t, watcher, holder.Character)
		events.DrainQueuedMessagesForTest(watcher.UserId)

		watched, _ := meditateLines(t, holder, watcher)
		assert.Equal(t, 1, countContaining(watched, "Aliceia sits down and begins to meditate."), "%v", watched)
		assert.Equal(t, 1, countContaining(watched, "Aliceia continues meditating."), "%v", watched)
		assert.Equal(t, 1, countContaining(watched, "Aliceia stops meditating."), "%v", watched)
	})
}

// mobMeditateLines runs the meditate twin's three phases on the Skeleton and
// returns what watcher read.
func mobMeditateLines(t *testing.T, m *mobs.Mob, watcher *users.UserRecord) []string {
	t.Helper()
	require.Equal(t, events.Continue, ApplyConditions(events.Condition{MobInstanceId: m.InstanceId, ConditionId: meditateTestId}))
	tickMobConditions(m, m.InstanceId)
	expire(t, m.Character.Conditions.List, meditateTestId)
	PruneConditions(events.NewTurn{TurnNumber: 1})
	return drainPlain(watcher.UserId)
}

func TestHiddenHolder_MobConditionLinesReachOnlyPerceivers(t *testing.T) {
	t.Run("hidden mob, watcher without see-hidden reads nothing", func(t *testing.T) {
		_, watcher, m := hiddenHolderFixture(t)
		sneakHide(t, &m.Character)
		require.False(t, watcher.Character.Perceives(&m.Character))
		events.DrainQueuedMessagesForTest(watcher.UserId)

		watched := mobMeditateLines(t, m, watcher)
		assert.Zero(t, countContaining(watched, "meditat"), "a non-perceiver read a hidden mob's condition line: %v", watched)
	})
	t.Run("visible mob is narrated (null probe)", func(t *testing.T) {
		_, watcher, m := hiddenHolderFixture(t)
		watched := mobMeditateLines(t, m, watcher)
		assert.Equal(t, 1, countContaining(watched, "Skeleton sits down and begins to meditate."), "%v", watched)
		assert.Equal(t, 1, countContaining(watched, "Skeleton continues meditating."), "%v", watched)
		assert.Equal(t, 1, countContaining(watched, "Skeleton stops meditating."), "%v", watched)
	})
	t.Run("hidden mob, see-hidden watcher reads every line by name", func(t *testing.T) {
		_, watcher, m := hiddenHolderFixture(t)
		sneakHide(t, &m.Character)
		grantSeeHidden(t, watcher, &m.Character)
		events.DrainQueuedMessagesForTest(watcher.UserId)

		watched := mobMeditateLines(t, m, watcher)
		assert.Equal(t, 1, countContaining(watched, "Skeleton sits down and begins to meditate."), "%v", watched)
		assert.Equal(t, 1, countContaining(watched, "Skeleton continues meditating."), "%v", watched)
		assert.Equal(t, 1, countContaining(watched, "Skeleton stops meditating."), "%v", watched)
	})
}

// A hide landing on a visible holder is judged against the room before it
// lands, when everyone could see them: the watchers saw them vanish.
func TestHiddenHolder_HideStartOnAVisibleHolderStillReachesWatchers(t *testing.T) {
	t.Run("player", func(t *testing.T) {
		holder, watcher, _ := hiddenHolderFixture(t)
		require.Equal(t, events.Continue, ApplyConditions(events.Condition{UserId: holder.UserId, ConditionId: 9}))
		require.True(t, holder.Character.IsHidden())
		assert.Equal(t, 1, countContaining(drainPlain(watcher.UserId), "Aliceia disappears into the shadows."))
	})
	t.Run("mob", func(t *testing.T) {
		_, watcher, m := hiddenHolderFixture(t)
		require.Equal(t, events.Continue, ApplyConditions(events.Condition{MobInstanceId: m.InstanceId, ConditionId: 9}))
		require.True(t, m.Character.IsHidden())
		got := drainPlain(watcher.UserId)
		assert.Equal(t, 1, countContaining(got, "Skeleton disappears into the shadows."), "%v", got)
	})
}

// A hide's end line announces the holder coming back into view, so it reaches
// the watchers who could not see them while it held, and names them. The
// shroud is the hard case: it runs out in the prune pass, whose end line goes
// out before the Validate that reveals the holder.
func TestHiddenHolder_RevealLineReachesWatchers(t *testing.T) {
	t.Run("player shroud runs out", func(t *testing.T) {
		holder, watcher, _ := hiddenHolderFixture(t)
		require.NoError(t, holder.Character.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 150, "spell"))
		require.True(t, holder.Character.HiddenByShroud())
		require.False(t, watcher.Character.Perceives(holder.Character))
		events.DrainQueuedMessagesForTest(watcher.UserId)

		runOutShroud(t, holder.Character)
		PruneConditions(events.NewTurn{TurnNumber: 1})
		require.False(t, holder.Character.IsHidden())
		got := drainPlain(watcher.UserId)
		assert.Equal(t, 1, countContaining(got, "Aliceia shimmers back into view."), "%v", got)
	})
	t.Run("mob shroud runs out", func(t *testing.T) {
		_, watcher, m := hiddenHolderFixture(t)
		require.NoError(t, m.Character.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 150, "spell"))
		require.True(t, m.Character.HiddenByShroud())
		require.False(t, watcher.Character.Perceives(&m.Character))
		events.DrainQueuedMessagesForTest(watcher.UserId)

		runOutShroud(t, &m.Character)
		PruneConditions(events.NewTurn{TurnNumber: 1})
		require.False(t, m.Character.IsHidden())
		got := drainPlain(watcher.UserId)
		assert.Equal(t, 1, countContaining(got, "Skeleton shimmers back into view."), "%v", got)
	})
	t.Run("player sneak spotted", func(t *testing.T) {
		holder, watcher, _ := hiddenHolderFixture(t)
		sneakHide(t, holder.Character)
		events.DrainQueuedMessagesForTest(watcher.UserId)

		require.NoError(t, holder.Character.Awareness.TransitionToRevealing(
			state.TransitionReason{Trigger: awareness.TriggerObserverSearch}))
		PruneConditions(events.NewTurn{TurnNumber: 1})
		got := drainPlain(watcher.UserId)
		assert.Equal(t, 1, countContaining(got, "Aliceia emerges from the shadows."), "%v", got)
	})
}
