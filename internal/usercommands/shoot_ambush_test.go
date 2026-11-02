package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #375 phase 7 (quest 100): a same-room shot from hiding is an ambush
// (help ambush: "a hidden shot is always an ambush"), and ExecuteFire marks
// it FireResult.Revealed. ambushQuestTargets turns that into the targets of
// the `command ambush` event; any other shot raises none.
func TestAmbushQuestTargets_OnlyAShotFromCover(t *testing.T) {
	const instId = 99704
	mobs.SetInstanceForTest(instId, &mobs.Mob{MobId: 9121, InstanceId: instId})
	t.Cleanup(func() { mobs.SetInstanceForTest(instId, nil) })

	targets, ok := ambushQuestTargets(1, actions.FireResult{Revealed: true, TargetMobInstanceId: instId})
	require.True(t, ok, "a shot from cover is an ambush")
	require.Len(t, targets, 1)
	require.Equal(t, quests.TargetMob, targets[0].Kind)
	require.Equal(t, 9121, targets[0].MobId)

	_, ok = ambushQuestTargets(1, actions.FireResult{TargetMobInstanceId: instId})
	require.False(t, ok, "an ordinary shot is no ambush")

	// A hidden sniper in the next room takes no opener (ExecuteFire's
	// surpriseShot needs the same room) and is not revealed by the shot.
	_, ok = ambushQuestTargets(1, actions.FireResult{IsSneaking: true, CrossRoom: true, TargetMobInstanceId: instId})
	require.False(t, ok, "a cross-room shot takes no opener, so it is no ambush")
}

const (
	shotAmbushProbeQuest = 99705
	shotAmbushToken      = "99705-start"
	shotFiredProbeQuest  = 99706
	shotFiredToken       = "99706-start"
)

// shotAmbushFixture is user 1 with a loaded bow beside skeleton mob 100, and
// two probes: `ambush`, under test, and `fire`, which proves a shot really went
// off and reached the engine, so that no ambush token cannot mean no shot.
func shotAmbushFixture(t *testing.T) (*users.UserRecord, *rooms.Room, *mobs.Mob) {
	t.Helper()
	pinContestFloorOff(t)
	t.Cleanup(seedAllRegistries())
	isolateOpinions(t)

	user, room := getTestUserAndRoom(t)
	room.Lamp = rooms.LampPtr(90) // pin fully lit; see combat_blind_warning_test.go in internal/hooks
	user.Character.Stats.Perception.ValueAdj = 300
	user.Character.Stats.Strength.ValueAdj = 1
	user.Character.EndAggro()
	t.Cleanup(user.Character.EndAggro)

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	mob.Character.EndAggro()

	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: shotAmbushProbeQuest,
		Name:    "Shot Ambush Probe",
		Triggers: []quests.TriggerDef{
			{Event: "command", Command: "ambush", Target: quests.TargetMob, Actions: []quests.ActionDef{{Grant: shotAmbushToken}}},
		},
	})
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: shotFiredProbeQuest,
		Name:    "Shot Fired Probe",
		Triggers: []quests.TriggerDef{
			{Event: "command", Command: "fire", Target: quests.TargetMob, Actions: []quests.ActionDef{{Grant: shotFiredToken}}},
		},
	})
	return user, room, mob
}

func fireForAmbushTest(t *testing.T, user *users.UserRecord, room *rooms.Room) {
	t.Helper()
	equipBow(user.Character, true)
	delete(user.Character.Cooldowns, "special-move")
	_, err := Fire("skeleton", user, room, 0)
	require.NoError(t, err)
	require.False(t, user.Character.Equipment.Weapon.Loaded, "the shot must really have gone off")
}

func TestFire_ShotFromCoverRaisesTheAmbushEvent(t *testing.T) {
	user, room, mob := shotAmbushFixture(t)
	mob.Character.Health = 100000
	mob.Character.HealthMax.Value = 100000
	hideForAmbushTest(t, user.Character)

	fireForAmbushTest(t, user, room)

	require.True(t, user.Character.HasQuest(shotAmbushToken),
		"a shot from cover must raise command ambush aimed at the mob")
}

func TestFire_OrdinaryShotRaisesNoAmbushEvent(t *testing.T) {
	user, room, mob := shotAmbushFixture(t)
	mob.Character.Health = 100000
	mob.Character.HealthMax.Value = 100000

	fireForAmbushTest(t, user, room)

	require.True(t, user.Character.HasQuest(shotFiredToken), "the probe must hear the shot itself")
	require.False(t, user.Character.HasQuest(shotAmbushToken),
		"a shooter who was not hidden has no opener to raise")
}

// A shot that kills must not lose its target before the event names it. A
// shot can fumble (about 2.3%, see shootUntilItLands), so this hides and fires
// until one lands, and each try must raise the event, landed or not.
func TestFire_LethalShotFromCoverRaisesTheAmbushEvent(t *testing.T) {
	user, room, mob := shotAmbushFixture(t)
	mob.Character.Health = 1

	for i := 0; i < 12 && mob.Character.Health > 0; i++ {
		user.Character.EndAggro()
		hideForAmbushTest(t, user.Character)
		user.Character.ClearQuestToken(shotAmbushToken)

		fireForAmbushTest(t, user, room)

		require.True(t, user.Character.HasQuest(shotAmbushToken),
			"every shot from cover must raise command ambush aimed at the mob")
	}
	require.LessOrEqual(t, mob.Character.Health, 0, "the fixture's shot must kill, or this proves nothing")
}
