package usercommands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// The sit command (#637) through the real dispatcher, with the shipped
// keywords.yaml command aliases loaded, so `rest` is proven to reach it the
// way a player types it.

func seedShippedCommandAliases(t *testing.T) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(helpDataRoot(t), "keywords.yaml"))
	require.NoError(t, err)
	var kw keywords.Aliases
	require.NoError(t, yaml.Unmarshal(raw, &kw))
	require.NotEmpty(t, kw.CommandAliases, "keywords.yaml command-aliases did not parse")
	t.Cleanup(keywords.SeedKeywordsForTest(keywords.Aliases{CommandAliases: kw.CommandAliases}))
}

func TestSitCommand_SitAndRestBothSeatAndStandGetsUp(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	seedShippedCommandAliases(t)
	alice, _, _ := speechWrapperScene(t)

	for _, verb := range []string{"sit", "rest"} {
		handled, err := TryCommand(verb, "", alice.UserId, events.CmdSkipScripts)
		require.True(t, handled, verb)
		require.NoError(t, err, verb)
		require.True(t, alice.Character.IsSitting(), "%s must seat the player, state %v", verb, alice.Character.Position.State())
		require.Equal(t, []string{"You sit down."}, speechWrapperHeard(1), verb)
		require.Equal(t, []string{"Aliceia sits down."}, speechWrapperHeard(2), verb)

		handled, err = TryCommand("stand", "", alice.UserId, events.CmdSkipScripts)
		require.True(t, handled)
		require.NoError(t, err)
		require.True(t, alice.Character.IsStanding(), "stand must get a sitter up")
		require.Equal(t, []string{"You stand up."}, speechWrapperHeard(1))
		require.Equal(t, []string{"Aliceia stands up."}, speechWrapperHeard(2))
	}
}

func TestSitCommand_BlindObserverReadsNoName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	alice, bob, _ := speechWrapperScene(t)
	blindForSpeechTest(t, bob)

	_, err := TryCommand("sit", "", alice.UserId, events.CmdSkipScripts)
	require.NoError(t, err)
	require.True(t, alice.Character.IsSitting())
	for _, line := range speechWrapperHeard(2) {
		require.NotContains(t, line, "Aliceia", "a blinded observer must not read the sitter's name")
	}
}

func TestSitCommand_AlreadySittingAndInCombatRefuse(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	alice, _, _ := speechWrapperScene(t)

	_, _ = TryCommand("sit", "", alice.UserId, events.CmdSkipScripts)
	speechWrapperHeard(1)
	_, _ = TryCommand("sit", "", alice.UserId, events.CmdSkipScripts)
	require.Equal(t, []string{"You're already sitting."}, speechWrapperHeard(1))

	_, _ = TryCommand("stand", "", alice.UserId, events.CmdSkipScripts)
	alice.Character.SetAggro(0, 100, characters.DefaultAttack)
	speechWrapperHeard(1)
	_, _ = TryCommand("sit", "", alice.UserId, events.CmdSkipScripts)
	require.True(t, alice.Character.IsStanding(), "a fighter must not sit")
	require.Equal(t, []string{"You can't do that while fighting."}, speechWrapperHeard(1))
}

// A hidden player's sit and stand reach no one (#274, owner R3): the
// observer gets nothing, the hider gets their own line and the hidden note.
func TestSitCommand_HiddenPlayerIsUnseen(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	alice, _, _ := speechWrapperScene(t)
	reason := state.TransitionReason{Trigger: "sit_hidden_test"}
	require.NoError(t, alice.Character.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason))
	alice.Character.Awareness.ResolveConcealment(true, reason)
	require.True(t, alice.Character.IsHidden())
	speechWrapperHeard(1)
	speechWrapperHeard(2)

	_, err := TryCommand("sit", "", alice.UserId, events.CmdSkipScripts)
	require.NoError(t, err)
	require.True(t, alice.Character.IsSitting())
	require.True(t, alice.Character.IsHidden(), "sitting must not reveal the hider")
	require.Equal(t, []string{"You sit down.", actions.HiddenPostureNote}, speechWrapperHeard(1))
	require.Empty(t, speechWrapperHeard(2), "an observer must get no line for a hidden sitter")

	_, err = TryCommand("stand", "", alice.UserId, events.CmdSkipScripts)
	require.NoError(t, err)
	require.True(t, alice.Character.IsStanding())
	require.Equal(t, []string{"You stand up.", actions.HiddenPostureNote}, speechWrapperHeard(1))
	require.Empty(t, speechWrapperHeard(2), "an observer must get no line for a hidden stander")
}

// A sleeping player who types sit or rest stays asleep and is told to type
// 'wake' (owner ruling 2026-10-19): sit is exempt from the dispatcher's
// cancel-on-action wake (keepsSleep). Typing wake then wakes them.
func TestSitCommand_SleepingPlayerStaysAsleepUntilWake(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		15: {ConditionId: 15, Name: "Sleeping", TriggerCount: 1000000,
			Flags: []conditions.Flag{conditions.Sleeping, conditions.CancelOnAction, conditions.CancelOnDamage}},
	})()
	seedShippedCommandAliases(t)
	alice, _, _ := speechWrapperScene(t)
	require.NoError(t, alice.Character.AddCondition(15, false))
	require.True(t, alice.Character.HasConditionFlag(conditions.Sleeping))
	speechWrapperHeard(1)

	for _, verb := range []string{"sit", "rest"} {
		_, err := TryCommand(verb, "", alice.UserId, events.CmdSkipScripts)
		require.NoError(t, err, verb)
		require.True(t, alice.Character.HasConditionFlag(conditions.Sleeping), "%s must not wake the sleeper", verb)
		require.True(t, alice.Character.IsStanding(), "%s must not seat a sleeper, state %v", verb, alice.Character.Position.State())
		require.Equal(t, []string{actions.SitRefuseAsleep}, speechWrapperHeard(1), verb)
	}

	_, err := TryCommand("wake", "", alice.UserId, events.CmdSkipScripts)
	require.NoError(t, err)
	require.False(t, alice.Character.HasConditionFlag(conditions.Sleeping), "wake must wake the sleeper")
}

func TestRoomListing_ShowsASitter(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	alice, bob, room := speechWrapperScene(t)

	listed := func() string {
		return strings.Join(rooms.GetDetails(room, bob).VisiblePlayers, "\n")
	}
	require.NotContains(t, listed(), "(sitting)", "a standing player must not list as sitting")
	_, _ = TryCommand("sit", "", alice.UserId, events.CmdSkipScripts)
	require.True(t, alice.Character.IsSitting())
	require.Contains(t, listed(), "(sitting)", "look must show the sitter seated")
}
