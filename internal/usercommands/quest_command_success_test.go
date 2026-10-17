package usercommands

import (
	"fmt"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// The 2026-10-17 pre-deploy playtest finished tutorial steps on refused
// commands: a bare `say` ("Say what?") and a `trip` at a target already on the
// ground both counted, because the steps listened on command_issued, which
// fires whenever a command is dispatched. The steps now listen on `command`,
// the success notification. These tests pin which outcomes send it.

// listenForCommand registers a one-trigger quest that grants questId-start on
// a `command` event for cmd, and returns a probe reporting whether it fired.
func listenForCommand(t *testing.T, user *users.UserRecord, questId int, cmd string) func() bool {
	t.Helper()
	return listenForCommandNoun(t, user, questId, cmd, "")
}

// listenForCommandNoun is listenForCommand with a noun filter on the trigger.
func listenForCommandNoun(t *testing.T, user *users.UserRecord, questId int, cmd string, noun string) func() bool {
	t.Helper()
	restore := questengine.ResetEngineForTest()
	t.Cleanup(restore)
	token := fmt.Sprintf("%d-start", questId)
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: questId,
		Name:    "command success probe",
		Steps:   []questengine.QuestStep{{Id: "start"}},
		Triggers: []questengine.TriggerDef{{
			Event:   "command",
			Command: cmd,
			Noun:    noun,
			Actions: []questengine.ActionDef{{Grant: token}},
		}},
	})
	user.Character.ClearQuestToken(token)
	t.Cleanup(func() { user.Character.ClearQuestToken(token) })
	return func() bool { return user.Character.HasQuest(token) }
}

func TestSay_QuestCommandEventOnlyWhenSomethingIsSaid(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	alice, _, room := speechWrapperScene(t)
	fired := listenForCommand(t, alice, 999201, "say")

	_, err := Say("", alice, room, 0)
	require.NoError(t, err)
	require.False(t, fired(), "a refused bare say must not send the say command event")

	_, err = Say("hello", alice, room, 0)
	require.NoError(t, err)
	require.True(t, fired(), "a say that is spoken must send the say command event")
}

func TestTrip_QuestCommandEventOnTargetAlreadyDownButNotOnCostRefusal(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	speciesCleanup := seedSpecialMoveSpecies()
	defer speciesCleanup()

	user, room := getTestUserAndRoom(t)
	target := mobs.GetInstance(100)
	require.NotNil(t, target)

	t.Run("cost refusal is silent", func(t *testing.T) {
		configureSpecialMoveFaction(t)
		resetSpecialMoveWrapperFixture(t, user, target, 11, 0)
		fired := listenForCommand(t, user, 999202, "trip")
		_, err := Trip("#100", user, room, 0)
		require.NoError(t, err)
		require.False(t, fired(), "a trip refused for cost must not send the trip command event")
	})

	t.Run("target already on the ground counts", func(t *testing.T) {
		configureSpecialMoveFaction(t)
		resetSpecialMoveWrapperFixture(t, user, target, 11, 100)
		setCombatPositionParallel(&target.Character, position.Prone)
		t.Cleanup(func() { setCombatPositionParallel(&target.Character, position.Standing) })
		fired := listenForCommand(t, user, 999203, "trip")
		_, err := Trip("#100", user, room, 0)
		require.NoError(t, err)
		require.True(t, fired(), "trip at a target already down must send the trip command event")
	})
}

func TestWarcry_QuestCommandEventOnlyUnderAWarcry(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	room, user := roomAndUserForWarcry(t)

	fired := listenForCommand(t, user, 999204, "warcry")
	user.Character.Conviction = 0
	_, err := Warcry("", user, room, 0)
	require.NoError(t, err)
	require.False(t, fired(), "a warcry refused for cost must not send the warcry command event")

	user.Character.Conviction = 1000
	user.Character.ConvictionMax.Value = 1000
	_, err = Warcry("", user, room, 0)
	require.NoError(t, err)
	require.True(t, user.Character.HasCondition(conditions.ConditionIdWarcry), "fixture: the warcry must land")
	require.True(t, fired(), "a warcry that lands must send the warcry command event")
}

// The tutorial's cast step names one spell. The cast success event carries
// the canonical spell id as its noun, so casting by alias still matches and a
// refused cast sends nothing.
func TestCast_QuestCommandEventNamesTheSpellById(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	alice, _, room := speechWrapperScene(t)

	t.Cleanup(spells.SeedSpellsForTest(map[string]*spells.SpellData{
		"testquest-spike": {
			SpellId: "testquest-spike", Name: "Test Quest Spike", Aliases: []string{"tqspike"},
			AttackType: combatvocab.AttackNone, DamageType: combatvocab.DamageNonHarm,
			Targeting: combatvocab.TargetSelf,
			Schools:   []string{spells.SchoolManifestation},
			BaseFolds: 4, Cost: 5,
		},
	}))
	alice.Character.Activity = activity.NewMachine()
	alice.Character.Skills = map[string]int{"manifestation": 5}
	alice.Character.SpellBook = map[string]int{"testquest-spike": 1}
	fired := listenForCommandNoun(t, alice, 999205, "cast", "testquest-spike")

	alice.Character.ConvictionMax.Value, alice.Character.Conviction = 4000, 0
	_, err := Cast("tqspike", alice, room, 0)
	require.NoError(t, err)
	require.False(t, alice.Character.Activity.IsCasting(), "fixture: the cast is refused")
	require.False(t, fired(), "a refused cast must not send the cast command event")

	alice.Character.Conviction = 4000
	_, err = Cast("tqspike", alice, room, 0)
	require.NoError(t, err)
	require.True(t, alice.Character.Activity.IsCasting(), "fixture: the cast began")
	require.True(t, fired(), "a cast begun by alias must send the cast event naming the spell id")
}

func roomAndUserForWarcry(t *testing.T) (*rooms.Room, *users.UserRecord) {
	t.Helper()
	user, room := getTestUserAndRoom(t)
	user.Character.Cooldowns = characters.Cooldowns{}
	user.Character.Skills = map[string]int{}
	user.Character.Conditions = conditions.New()
	return room, user
}
