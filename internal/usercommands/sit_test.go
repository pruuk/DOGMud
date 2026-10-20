package usercommands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/rooms"
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
