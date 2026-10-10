package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A room line about a hidden mob never names it (epic #382). mobDisplayName
// fed GetMobNameIndexed, whose adjectives include "hidden", so a room-wide
// line named a hidden mob to every reader who sees faces, as
// "Skeleton (hidden)". Since #458 a room line whose actor or holder is a
// hidden mob goes only to the readers who perceive it, named, and is silent
// to the rest (owner R3, #274). Only a disruption (a break, fizzle or
// falter) still reads "Something" to the room, through mobSubjectName, since
// a disruption is heard (owner R4, #242).

// hideMob puts m into the Hidden awareness state the way sneak does.
func hideMob(t *testing.T, m *mobs.Mob) {
	t.Helper()
	reason := state.TransitionReason{Trigger: "hidden_mob_room_lines_test"}
	require.NoError(t, m.Character.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason))
	m.Character.Awareness.ResolveConcealment(true, reason)
	require.True(t, m.Character.IsHidden())
}

// litRoomOneWithHiddenSkeleton seeds the registries and the narration
// conditions, pins room 1 lit, hides the Skeleton (instance 100) and drains
// user 2, who reads faces in room 1.
func litRoomOneWithHiddenSkeleton(t *testing.T) *mobs.Mob {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(seedNarrationConditions())
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	m := mobs.GetInstance(100)
	require.NotNil(t, m)
	m.Character.Validate()
	hideMob(t, m)
	events.DrainQueuedMessagesForTest(2)
	return m
}

// requireUnnamed asserts got holds want exactly once and never the mob's
// name or its hidden adjective.
func requireUnnamed(t *testing.T, got []string, want string) {
	t.Helper()
	assert.Zero(t, countContaining(got, "Skeleton"), "a faces reader must not read a hidden mob's name: %v", got)
	assert.Zero(t, countContaining(got, "hidden"), "nor its hidden adjective: %v", got)
	assert.Equal(t, 1, countContaining(got, want), "%v", got)
}

// A condition line about a hidden mob goes only to the readers who perceive
// it (#458), so a reader without see-hidden reads neither its name nor a
// "Something" line. hidden_holder_condition_lines_test.go has the see-hidden
// reader, who reads the name, and a hide's reveal line, which reaches all.
func TestHiddenMob_ConditionStartLineIsNotTold(t *testing.T) {
	litRoomOneWithHiddenSkeleton(t)
	ApplyConditions(events.Condition{MobInstanceId: 100, ConditionId: glowConditionId})
	got := drainPlain(2)
	assert.Zero(t, countContaining(got, "glows"), "%v", got)
}

func TestHiddenMob_ConditionEndLineIsNotTold(t *testing.T) {
	m := litRoomOneWithHiddenSkeleton(t)
	for _, id := range []int{fadeConditionId, shadeConditionId} {
		require.True(t, m.Character.Conditions.AddCondition(id, false))
		expire(t, m.Character.Conditions.List, id)
	}
	PruneConditions(events.NewTurn{TurnNumber: 1})
	got := drainPlain(2)
	// fade authors {actee}, the tagged name; shade authors a bare
	// {actee_plain}. Neither is a hide, so neither is a reveal.
	assert.Zero(t, countContaining(got, "fades"), "%v", got)
	assert.Zero(t, countContaining(got, "emerges"), "%v", got)
}

func TestHiddenMob_RoundTickTriggerLineIsNotTold(t *testing.T) {
	m := litRoomOneWithHiddenSkeleton(t)
	require.True(t, m.Character.Conditions.AddCondition(shiverConditionId, false))
	tickMobConditions(m, 100)
	got := drainPlain(2)
	assert.Zero(t, countContaining(got, "shivers"), "%v", got)
}

// A hidden mob's drain-area landing line is its own act, so it follows owner
// ruling R3 (#274, "a hidden actor does not emote. Silence, not an anonymous
// line."): a reader who does not perceive the mob reads nothing, one with
// see-hidden reads its name (#458). The drain that finds no one is a fizzle,
// a disruption, and keeps its "Something's" line (ruling R4).
func TestHiddenMob_DrainAreaLandingLineIsSilentToNonPerceivers(t *testing.T) {
	const line = "Skeleton's Core Recharge tears the life from everyone in the room!"
	run := func(t *testing.T, hide bool) *spellParityFixture {
		t.Helper()
		f := newSpellParityFixture(t, spellContestAttackWin())
		t.Cleanup(combat.SetChannelAttackContestRunnerForTest(attackWinContest(t)))
		const seeHiddenId = 7103
		t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
			seeHiddenId: {ConditionId: seeHiddenId, Name: "Test True Sight",
				Flags: []conditions.Flag{conditions.SeeHidden}},
		}))
		t.Cleanup(conditions.SeedConditionRecordsForTest())
		require.True(t, f.casterUser.Character.Conditions.AddCondition(seeHiddenId, true))
		f.casterMob.Character.Validate()
		if hide {
			hideMob(t, f.casterMob)
			require.True(t, f.casterUser.Character.Perceives(&f.casterMob.Character))
			require.False(t, f.watcher.Character.Perceives(&f.casterMob.Character))
		}
		events.DrainQueuedMessagesForTest(f.casterUser.UserId)
		events.DrainQueuedMessagesForTest(f.watcher.UserId)
		spell := &spells.SpellData{SpellId: "test-core-recharge", Name: "Core Recharge", EffectType: "drain_area",
			AttackType: combatvocab.AttackSpell, DamageType: combatvocab.DamagePhysical, Targeting: combatvocab.TargetArea}
		resolveMobDrainArea(f.casterMob, f.room, spell)
		return f
	}
	t.Run("hidden mob, reader without see-hidden reads nothing", func(t *testing.T) {
		f := run(t, true)
		got := drainPlain(f.watcher.UserId)
		assert.Zero(t, countContaining(got, "tears the life"), "%v", got)
		assert.Zero(t, countContaining(got, "Skeleton"), "%v", got)
	})
	t.Run("hidden mob, see-hidden reader reads its name", func(t *testing.T) {
		f := run(t, true)
		got := drainPlain(f.casterUser.UserId)
		assert.Equal(t, 1, countContaining(got, line), "%v", got)
	})
	t.Run("visible mob is named to every reader (null probe)", func(t *testing.T) {
		f := run(t, false)
		got := drainPlain(f.watcher.UserId)
		assert.Equal(t, 1, countContaining(got, line), "%v", got)
	})
}

// A hidden mob's own acts (stat-gain and prone-recovery emotes, the weave,
// the focus shift) are silent to every reader who does not perceive it,
// owner ruling R3 (#274): "Silence, not an anonymous line." They read
// "Something ..." to the whole room until #458. A see-hidden reader still
// reads the mob's name; a visible mob is named to everyone.
func TestHiddenMob_OwnActsAreSilentToNonPerceivers(t *testing.T) {
	cases := []struct {
		name   string
		send   func(t *testing.T, m *mobs.Mob, room *rooms.Room, target *users.UserRecord)
		named  string
		phrase string
	}{
		{"stat gain", func(t *testing.T, m *mobs.Mob, room *rooms.Room, _ *users.UserRecord) {
			emitMobStatGains(actions.NewMobActorInRoom(m, room),
				map[string]int{"dexterity": m.Character.Stats.Dexterity.Training - 1})
		}, "Skeleton moves with increasing swiftness.", "moves with increasing swiftness"},
		{"prone recovery", func(t *testing.T, m *mobs.Mob, _ *rooms.Room, _ *users.UserRecord) {
			require.NoError(t, m.Character.Position.TransitionToProne(position.ProneData{},
				state.TransitionReason{Trigger: "hidden_mob_room_lines_test"}))
			tickMobProneRecovery(m)
		}, "Skeleton clambers to their feet in a rushed panic.", "clambers to their feet"},
		{"weave", func(t *testing.T, m *mobs.Mob, room *rooms.Room, _ *users.UserRecord) {
			sendMobWeaving(m, room)
		}, "Skeleton weaves magic with focused intent.", "weaves magic"},
		{"focus shift", func(t *testing.T, m *mobs.Mob, room *rooms.Room, target *users.UserRecord) {
			sendMobShiftsFocus(m, room, target)
		}, "Skeleton shifts focus to Aliceia!", "shifts focus"},
	}
	for _, tc := range cases {
		t.Run(tc.name+": hidden mob, reader without see-hidden reads nothing", func(t *testing.T) {
			seer, watcher, m := hiddenHolderFixture(t)
			sneakHide(t, &m.Character)
			require.False(t, watcher.Character.Perceives(&m.Character))
			events.DrainQueuedMessagesForTest(watcher.UserId)
			tc.send(t, m, rooms.LoadRoom(1), seer)
			got := drainPlain(watcher.UserId)
			assert.Zero(t, countContaining(got, tc.phrase), "%v", got)
			assert.Zero(t, countContaining(got, "Skeleton"), "%v", got)
			assert.Zero(t, countContaining(got, "Something"), "%v", got)
		})
		t.Run(tc.name+": hidden mob, see-hidden reader reads its name", func(t *testing.T) {
			seer, _, m := hiddenHolderFixture(t)
			sneakHide(t, &m.Character)
			grantSeeHidden(t, seer, &m.Character)
			events.DrainQueuedMessagesForTest(seer.UserId)
			tc.send(t, m, rooms.LoadRoom(1), seer)
			got := drainPlain(seer.UserId)
			assert.Equal(t, 1, countContaining(got, tc.named), "%v", got)
		})
		t.Run(tc.name+": visible mob is named (null probe)", func(t *testing.T) {
			seer, watcher, m := hiddenHolderFixture(t)
			tc.send(t, m, rooms.LoadRoom(1), seer)
			got := drainPlain(watcher.UserId)
			assert.Equal(t, 1, countContaining(got, tc.named), "%v", got)
		})
	}
}

// A visible mob is still named: the null probe for the four tests above.
func TestVisibleMob_ConditionStartLineNamesIt(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	t.Cleanup(seedNarrationConditions())
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	events.DrainQueuedMessagesForTest(2)
	ApplyConditions(events.Condition{MobInstanceId: 100, ConditionId: glowConditionId})
	assert.Equal(t, 1, countContaining(drainPlain(2), "Skeleton glows."))
}

// A per-viewer name follows that viewer's Perceives: a viewer who sees the
// hidden (a see-hidden condition) reads the name, one who does not reads
// "something". Room-wide (viewer 0) is "something" whoever is in the room.
func TestMobDisplayName_HiddenMobFollowsTheViewer(t *testing.T) {
	m := litRoomOneWithHiddenSkeleton(t)
	room := rooms.LoadRoom(1)
	const seeHiddenId = 7101
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		seeHiddenId: {ConditionId: seeHiddenId, Name: "Test True Sight",
			Flags: []conditions.Flag{conditions.SeeHidden}},
	}))
	seer := users.GetByUserId(1)
	require.True(t, seer.Character.Conditions.AddCondition(seeHiddenId, true))
	require.True(t, seer.Character.Perceives(&m.Character))
	require.False(t, users.GetByUserId(2).Character.Perceives(&m.Character))

	assert.Contains(t, plainText(mobDisplayName(m, room, 1)), "Skeleton")
	assert.Equal(t, "something", plainText(mobDisplayName(m, room, 2)))
	assert.Equal(t, "something", plainText(mobDisplayName(m, room, 0)))
	assert.Equal(t, "Something", plainText(mobSubjectName(m, room)))
}
