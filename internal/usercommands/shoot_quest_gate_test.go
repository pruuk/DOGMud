package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/GoMudEngine/GoMud/internal/actions"
)

// A shot through a room_gate reads as a shot through a locked door. Without
// this, `shoot s east` at Marn's told a player with no quest "You can't attack
// Sefton", a name oracle, and a combat mob behind a gate could really be shot.

const shootGateQuestId = 999650

// shootGateScene stands an armed shooter in room 1 with a gate on room 2 to the
// north, and a mob called Sefton in room 2.
func shootGateScene(t *testing.T, nonCombatant bool) (*users.UserRecord, *rooms.Room, *mobs.Mob) {
	t.Helper()
	pinContestFloorOff(t)
	t.Cleanup(seedAllRegistries())
	isolateOpinions(t)

	user, room := getTestUserAndRoom(t)
	room.Lamp = rooms.LampPtr(90)
	user.Character.Stats.Perception.ValueAdj = 300
	user.Character.Stats.Strength.ValueAdj = 1
	user.Character.EndAggro()
	user.Character.QuestProgress = map[int]string{}
	equipBow(user.Character, true)

	target := &mobs.Mob{
		MobId:      1,
		InstanceId: 650,
		HomeRoomId: 2,
		Character: characters.Character{
			Name:         "Sefton",
			RoomId:       2,
			Health:       100000,
			NonCombatant: nonCombatant,
			Conditions:   conditions.New(),
			Cooldowns:    map[string]int{},
		},
	}
	target.Character.HealthMax.Value = 100000
	target.Character.Stats.Dexterity.ValueAdj = 80
	mobs.SetInstanceForTest(650, target)
	t.Cleanup(func() { mobs.SetInstanceForTest(650, nil) })
	next := rooms.LoadRoom(2)
	require.NotNil(t, next)
	next.Lamp = rooms.LampPtr(90)
	next.AddMob(650)
	t.Cleanup(func() { next.RemoveMob(650) })

	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: shootGateQuestId,
		Name:    "Shoot Gate Quest",
		Steps:   []questengine.QuestStep{{Id: "start"}},
		Triggers: []questengine.TriggerDef{{
			Event:      "room_gate",
			Room:       2,
			Conditions: questengine.Conditions{Missing: []string{"999650-start"}},
			Actions:    []questengine.ActionDef{{SendText: gateTestLine}},
		}},
	})
	return user, room, target
}

func TestShoot_AGatedExitReadsAsLocked(t *testing.T) {
	for _, c := range []struct {
		name         string
		nonCombatant bool
	}{
		{"a non-combatant behind the gate is not named", true},
		{"a combat mob behind the gate cannot be shot", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			user, room, target := shootGateScene(t, c.nonCombatant)
			out := runGate(t, user, func() (bool, error) { return Fire("sefton north", user, room, 0) })

			require.Contains(t, out, `north</ansi> exit is locked`)
			require.NotContains(t, out, "Sefton", "the refusal must not confirm who is in there")
			require.NotContains(t, out, gateTestLine, "a shot narrates no gate, as a scan does not")
			assert.Equal(t, 100000, target.Character.Health)
			assert.True(t, user.Character.Equipment.Weapon.Loaded, "a refused shot fires nothing")
		})
	}
}

// Anyone at all answers the same, so the line cannot be read for a name.
func TestShoot_AGatedExitAnswersEveryNameAlike(t *testing.T) {
	user, room, _ := shootGateScene(t, true)
	there := runGate(t, user, func() (bool, error) { return Fire("sefton north", user, room, 0) })
	absent := runGate(t, user, func() (bool, error) { return Fire("nobody north", user, room, 0) })
	require.Equal(t, there, absent)
}

func TestShoot_AGateAllowsThePlayerItAllows(t *testing.T) {
	user, room, target := shootGateScene(t, false)
	user.Character.QuestProgress[shootGateQuestId] = "start"
	shootUntilItLands(t, user, room, "sefton north", func() int { return target.Character.Health })
	assert.Less(t, target.Character.Health, 100000)

	user, room, _ = shootGateScene(t, true)
	user.Character.QuestProgress[shootGateQuestId] = "start"
	out := runGate(t, user, func() (bool, error) { return Fire("sefton north", user, room, 0) })
	require.Contains(t, out, "You can't attack")
}

// The pre-fire guards resolve the target before ExecuteFire refuses the exit, so
// a closed exit must resolve no one there, or "X is in your party!" and the PvP
// refusal answer for who stands behind it. A locked door is the same case.
func TestResolveShootTarget_AClosedExitResolvesNoOne(t *testing.T) {
	user, room, _ := shootGateScene(t, false)
	shooter := &actions.UserActor{User: user, Room: room}

	_, mobId, tRoom, cross := resolveShootTarget(room, "sefton north", user.Character, shooter)
	assert.Zero(t, mobId, "behind a gate")
	assert.False(t, cross)
	assert.Equal(t, room, tRoom)

	user.Character.QuestProgress[shootGateQuestId] = "start"
	_, mobId, tRoom, cross = resolveShootTarget(room, "sefton north", user.Character, shooter)
	assert.Equal(t, 650, mobId, "the gate allows this player")
	assert.True(t, cross)
	assert.Equal(t, 2, tRoom.RoomId)

	north := room.Exits["north"]
	north.Lock = gamelock.Lock{Difficulty: 10}
	room.Exits["north"] = north
	require.True(t, room.Exits["north"].Lock.IsLocked())
	_, mobId, _, _ = resolveShootTarget(room, "sefton north", user.Character, shooter)
	assert.Zero(t, mobId, "behind a locked door")
}
