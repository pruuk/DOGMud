package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// The instance-zone amount prompt (#375 R14) is a free-text prompt, which
// would swallow a typed command as its answer (#641). The stake command
// closes the prompt on any non-number and runs a known command as typed.

const (
	psUserId     = 9161
	psRoom       = 999161
	psNorthRoom  = 999162
	psMobId      = 315
	psInstanceId = 991610
)

func seedPortalStakeScene(t *testing.T) *users.UserRecord {
	t.Helper()
	events.DrainAllQueuedEventsForTest()
	// The shipped say shortcut: "." is an alias of say (keywords.yaml).
	t.Cleanup(keywords.SeedKeywordsForTest(keywords.Aliases{CommandAliases: map[string][]string{"say": {"."}}}))
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		psRoom:      {RoomId: psRoom, Zone: "TestZone", Title: "Archway", Exits: map[string]exit.RoomExit{"north": {RoomId: psNorthRoom}}},
		psNorthRoom: {RoomId: psNorthRoom, Zone: "TestZone", Title: "Street", Exits: map[string]exit.RoomExit{}},
	}, map[string]*rooms.ZoneConfig{}))
	newSable := func() *mobs.Mob {
		return &mobs.Mob{MobId: psMobId, InstanceId: psInstanceId, HomeRoomId: psRoom, Character: characters.Character{
			Name: "Sable", RoomId: psRoom, Conditions: conditions.New(), Awareness: awareness.NewMachine(),
		}}
	}
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{psMobId: newSable()}, map[int]*mobs.Mob{psInstanceId: newSable()}))
	u := users.NewTestUser(psUserId, "staker", "Staker", uint64(psUserId))
	u.Character.RoomId = psRoom
	u.Character.Gold = 1000
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{psUserId: u}))

	open := behaviortree.LookupAction("open_instance_portal")
	require.NotNil(t, open)
	res := open(map[string]any{
		"min_gold": 100,
		"zone":     "arena",
		"guidance": "Name a stake.",
		"zones":    map[string]any{"arena": "Instance Arena"},
	}, &behaviortree.EvalContext{
		Event:      behaviortree.EventContext{EventType: "player_topic", UserId: psUserId, Text: "open_arena", RoomId: psRoom},
		InstanceId: psInstanceId,
		MobId:      psMobId,
		RoomId:     psRoom,
	})
	require.Equal(t, behaviortree.Success, res)
	require.NotNil(t, u.GetPrompt(), "fixture check: the amount prompt is open")
	events.DrainAllQueuedEventsForTest()
	return u
}

// typeAtPrompt routes a typed line the way the input loop does and runs the
// command it hands back through the real dispatcher.
func typeAtPrompt(t *testing.T, u *users.UserRecord, line string) {
	t.Helper()
	next, had := u.RoutePromptInput(line, true)
	require.True(t, had)
	cmd, rest, _ := strings.Cut(strings.TrimSpace(next), " ")
	require.Equal(t, behaviortree.PortalStakeCommand, cmd)
	handled, err := TryCommand(cmd, rest, u.UserId, events.CmdTyped)
	require.NoError(t, err)
	require.True(t, handled, "the stake command is registered and handles the answer")
}

func TestPortalStake_MovementClosesThePromptAndStillMoves(t *testing.T) {
	u := seedPortalStakeScene(t)
	typeAtPrompt(t, u, "north")

	require.Nil(t, u.GetPrompt(), "a non-number closes the prompt")
	require.Equal(t, []string{"north"}, events.DrainQueuedUserInputsForTest(psUserId),
		"a known command typed at the prompt still runs as a command")
	require.Equal(t, 1000, u.Character.Gold)
	require.Len(t, events.DrainQueuedMessagesForTest(psUserId), 1, "one line says the way stays closed")
}

// Every kind of line TryCommand runs also runs from the prompt, once: an
// emote, the say shortcut, and a compass direction with no exit (the
// "bump into a wall" line).
func TestPortalStake_EveryCommandKindRunsOnce(t *testing.T) {
	for _, line := range []string{"smile", ".hello", "`hello", "east", "say hello"} {
		t.Run(line, func(t *testing.T) {
			u := seedPortalStakeScene(t)
			if line == "`hello" {
				t.Cleanup(keywords.SeedKeywordsForTest(keywords.Aliases{CommandAliases: map[string][]string{"broadcast": {"`"}}}))
			}
			typeAtPrompt(t, u, line)
			require.Nil(t, u.GetPrompt(), "a non-number closes the prompt")
			require.Equal(t, []string{line}, events.DrainQueuedUserInputsForTest(psUserId),
				"the line runs as a command exactly once")
		})
	}
}

func TestPortalStake_KnownSpellNameRuns(t *testing.T) {
	u := seedPortalStakeScene(t)
	spellId := "stakeglow"
	t.Cleanup(spells.SeedSpellsForTest(map[string]*spells.SpellData{spellId: {SpellId: spellId, Name: "Stake Glow"}}))
	require.Equal(t, spellId, spells.ResolveSpellId(spellId), "fixture check: the spell resolves")
	u.Character.SpellBook = map[string]int{spellId: 1}
	typeAtPrompt(t, u, spellId)
	require.Equal(t, []string{spellId}, events.DrainQueuedUserInputsForTest(psUserId),
		"a spell the player knows, typed by name, is a command")
}

func TestPortalStake_UnknownWordClosesWithoutRunning(t *testing.T) {
	u := seedPortalStakeScene(t)
	typeAtPrompt(t, u, "lots")

	require.Nil(t, u.GetPrompt())
	require.Empty(t, events.DrainQueuedUserInputsForTest(psUserId), "a word that is no command is not run")
	require.Len(t, events.DrainQueuedMessagesForTest(psUserId), 1, "one refusal line")
}

func TestPortalStake_RegisteredUnderTheConstant(t *testing.T) {
	info, ok := userCommands[behaviortree.PortalStakeCommand]
	require.True(t, ok, "the prompt re-runs behaviortree.PortalStakeCommand, so it must be the registry key")
	require.False(t, info.AdminOnly)
}

func TestPortalStake_TypedWithNoPromptIsNotACommand(t *testing.T) {
	u := seedPortalStakeScene(t)
	u.ClearPrompt()
	handled, err := PortalStake("", u, rooms.LoadRoom(psRoom), events.CmdTyped)
	require.NoError(t, err)
	require.False(t, handled, "typed bare, the hidden command reads as unknown")
}
