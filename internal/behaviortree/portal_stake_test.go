package behaviortree

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// R14 (#375): a talk topic opens an instance zone through a private amount
// prompt. These tests pin that the prompt shows the authored guidance, that
// an amount opens the portal (charge, clone, exit and lines), that a bad
// answer closes the prompt, and that a node with no usable zone does not
// load.

const (
	stakeRoom       = 7101
	stakeAwayRoom   = 7102
	stakeMobId      = 315
	stakeInstanceId = 31501
	stakeUserId     = 1
	stakeEntryRoom  = 7199
)

type zoneCall struct {
	zone  string
	gold  int
	owner int
}

// seedStakeScene seeds Sable's room, Sable and a player holding 1000 gold,
// and swaps the zone cloner for a recorder.
func seedStakeScene(t *testing.T) *[]zoneCall {
	t.Helper()
	events.DrainAllQueuedEventsForTest()
	cleanRooms := rooms.SeedRoomsForTest(map[int]*rooms.Room{
		stakeRoom:     {RoomId: stakeRoom, Zone: "TestZone", Title: "Archway", Exits: map[string]exit.RoomExit{}},
		stakeAwayRoom: {RoomId: stakeAwayRoom, Zone: "TestZone", Title: "Street", Exits: map[string]exit.RoomExit{}},
	}, map[string]*rooms.ZoneConfig{})
	t.Cleanup(cleanRooms)
	t.Cleanup(seedTestMob(t, stakeMobId, stakeInstanceId, stakeRoom, "Sable"))
	t.Cleanup(seedTestUser(t, stakeUserId, "stakeplayer", "Stakeplayer", stakeRoom))
	requireUser(t, stakeUserId).Character.Gold = 1000

	calls := &[]zoneCall{}
	orig := createZoneInstance
	createZoneInstance = func(zone string, gold int, owner int, authorized []int, overworld int) (*rooms.ZoneInstance, error) {
		*calls = append(*calls, zoneCall{zone, gold, owner})
		return &rooms.ZoneInstance{TemplateZone: zone, EntryRoomId: stakeEntryRoom}, nil
	}
	t.Cleanup(func() { createZoneInstance = orig })
	return calls
}

func sableParams(zone string) map[string]any {
	p := map[string]any{
		"min_gold":     100,
		"exit_expires": "30 real minutes",
		"zones": map[string]any{
			"arena": "Instance Arena",
			"oasis": "Instance Planar Oasis",
		},
	}
	if zone != "" {
		p["zone"] = zone
		p["guidance"] = "A small stake makes a short, gentle fight.\nA large stake makes a long, deadly one."
	}
	return p
}

func stakeCtx(eventType, text string) *EvalContext {
	return &EvalContext{
		Event:      EventContext{EventType: eventType, UserId: stakeUserId, Text: text, RoomId: stakeRoom},
		InstanceId: stakeInstanceId,
		MobId:      stakeMobId,
		RoomId:     stakeRoom,
		MobName:    "Sable",
	}
}

// portalOutcome is everything a portal opening leaves behind.
type portalOutcome struct {
	gold      int
	calls     []zoneCall
	mobSaid   []string
	userLines []string
	roomLines []string
	tempExits []string
}

func captureOutcome(t *testing.T, calls *[]zoneCall) portalOutcome {
	t.Helper()
	room := rooms.LoadRoom(stakeRoom)
	require.NotNil(t, room)
	exits := []string{}
	for k := range room.ExitsTemp {
		exits = append(exits, k)
	}
	sort.Strings(exits)
	roomLines := []string{}
	for _, m := range events.DrainQueuedRoomMessagesForTest(stakeRoom) {
		roomLines = append(roomLines, m.Text)
	}
	return portalOutcome{
		gold:      requireUser(t, stakeUserId).Character.Gold,
		calls:     append([]zoneCall{}, *calls...),
		mobSaid:   events.DrainQueuedInputsForTest(stakeInstanceId),
		userLines: events.DrainQueuedMessagesForTest(stakeUserId),
		roomLines: roomLines,
		tempExits: exits,
	}
}

// answerStake types line at the open prompt the way the input loop does,
// then runs the prompt's command.
func answerStake(t *testing.T, line string) (StakeOutcome, string) {
	t.Helper()
	u := requireUser(t, stakeUserId)
	next, had := u.RoutePromptInput(line, true)
	require.True(t, had, "a prompt was open")
	require.True(t, strings.HasPrefix(next, PortalStakeCommand), "the answer re-runs the stake command, got %q", next)
	return AnswerStake(u)
}

func chooseArenaTopic(t *testing.T) {
	t.Helper()
	require.Equal(t, Success, actOpenInstancePortal(sableParams("arena"), stakeCtx("player_topic", "open_arena")))
}

func TestOpenInstancePortal_ZoneParamStartsPromptWithGuidance(t *testing.T) {
	calls := seedStakeScene(t)
	chooseArenaTopic(t)

	u := requireUser(t, stakeUserId)
	p := u.GetPrompt()
	require.NotNil(t, p, "choosing the zone's topic opens the amount prompt")
	require.Equal(t, PortalStakeCommand, p.Command)
	q := p.GetNextQuestion()
	require.NotNil(t, q)
	require.Equal(t, "How much gold will you stake?", q.Question)

	got := captureOutcome(t, calls)
	require.Equal(t, 1000, got.gold, "nothing is charged until the player names a stake")
	require.Empty(t, got.calls, "nothing opens until the player names a stake")
	require.Empty(t, got.mobSaid, "the prompt is private: the mob says nothing yet")
	require.Empty(t, got.roomLines, "the prompt is private: the room hears nothing")
	require.Len(t, got.userLines, 1, "the player sees one block before the question: the guidance")
	require.Contains(t, got.userLines[0], "A small stake makes a short, gentle fight.\nA large stake makes a long, deadly one.",
		"the player sees the zone's guidance before the question")
}

func TestStakePrompt_AmountOpensThePortal(t *testing.T) {
	calls := seedStakeScene(t)
	chooseArenaTopic(t)
	events.DrainQueuedMessagesForTest(stakeUserId) // the guidance
	outcome, _ := answerStake(t, "250")
	require.Equal(t, StakeHandled, outcome)
	got := captureOutcome(t, calls)

	require.Equal(t, 750, got.gold, "the stake is charged")
	require.Equal(t, []zoneCall{{"Instance Arena", 250, stakeUserId}}, got.calls, "the zone is cloned at the stake")
	require.Equal(t, []string{"arena-1"}, got.tempExits, "the exit opens")
	require.Equal(t, []string{"say The rift is open. Type arena-1 to enter.", "say It will hold for a time. Do not tarry."}, got.mobSaid)
	require.Nil(t, requireUser(t, stakeUserId).GetPrompt(), "the prompt closes once the portal opens")
}

// The event text is never read for a stake: under player_topic it is the
// bare topic key, and no live event carries the player's own words since
// #375 retired player_ask.
func TestOpenInstancePortal_TextIsNotReadForAStake(t *testing.T) {
	calls := seedStakeScene(t)
	require.Equal(t, Success, actOpenInstancePortal(sableParams("arena"), stakeCtx("player_topic", "arena 250")))
	require.NotNil(t, requireUser(t, stakeUserId).GetPrompt(), "the amount prompt opens")
	require.Empty(t, *calls, "nothing opens from the text")
}

func TestOpenInstancePortal_MissingZoneParamOpensNothing(t *testing.T) {
	calls := seedStakeScene(t)
	require.Equal(t, Failure, actOpenInstancePortal(sableParams(""), stakeCtx("player_topic", "arena 250")))
	require.Nil(t, requireUser(t, stakeUserId).GetPrompt())
	require.Empty(t, *calls)
}

const portalTreeYAML = `tree:
  type: sequence
  event: player_topic
  children:
    - type: condition
      check: topic_is
      topic: open_arena
    - type: action
      do: open_instance_portal
%s      zones:
        arena: "Instance Arena"
`

// An open_instance_portal node that could never open anything fails the
// tree's load.
func TestOpenInstancePortal_LoadRefusesAMissingOrUnknownZone(t *testing.T) {
	_, err := LoadTreeFromBytes([]byte(fmt.Sprintf(portalTreeYAML, "      zone: arena\n")))
	require.NoError(t, err, "fixture check: a zone that is a key of zones loads")

	_, err = LoadTreeFromBytes([]byte(fmt.Sprintf(portalTreeYAML, "")))
	require.ErrorContains(t, err, "open_instance_portal needs a zone param")

	_, err = LoadTreeFromBytes([]byte(fmt.Sprintf(portalTreeYAML, "      zone: moon\n")))
	require.ErrorContains(t, err, `open_instance_portal zone "moon" is not a key of zones`)
}

func TestStakePrompt_BelowMinimumRefusesNamingTheMinimum(t *testing.T) {
	calls := seedStakeScene(t)
	chooseArenaTopic(t)
	events.DrainQueuedMessagesForTest(stakeUserId)

	outcome, _ := answerStake(t, "50")
	require.Equal(t, StakeHandled, outcome)
	got := captureOutcome(t, calls)
	require.Equal(t, 1000, got.gold)
	require.Empty(t, got.calls)
	require.Equal(t, []string{"say That barely covers the runes. I need at least 100 gold."}, got.mobSaid)
	require.Nil(t, requireUser(t, stakeUserId).GetPrompt(), "a refused stake closes the prompt")
}

func TestStakePrompt_NonNumberClosesWithOneLine(t *testing.T) {
	calls := seedStakeScene(t)
	chooseArenaTopic(t)
	events.DrainQueuedMessagesForTest(stakeUserId)

	outcome, answer := answerStake(t, "lots")
	require.Equal(t, StakeNotAmount, outcome)
	require.Equal(t, "lots", answer, "the answer comes back so the caller can run it as a command")
	got := captureOutcome(t, calls)
	require.Equal(t, 1000, got.gold)
	require.Empty(t, got.calls)
	require.Empty(t, got.mobSaid)
	require.Len(t, got.userLines, 1, "one refusal line")
	require.Nil(t, requireUser(t, stakeUserId).GetPrompt(), "the prompt closes")
}

func TestStakePrompt_MobGoneOpensNothing(t *testing.T) {
	calls := seedStakeScene(t)
	chooseArenaTopic(t)
	events.DrainQueuedMessagesForTest(stakeUserId)

	mobs.GetInstance(stakeInstanceId).Character.RoomId = stakeAwayRoom

	outcome, _ := answerStake(t, "250")
	require.Equal(t, StakeHandled, outcome)
	got := captureOutcome(t, calls)
	require.Equal(t, 1000, got.gold, "no stake is taken when the vendor has gone")
	require.Empty(t, got.calls)
	require.Empty(t, got.mobSaid)
	require.Len(t, got.userLines, 1, "one line says why")
	require.Nil(t, requireUser(t, stakeUserId).GetPrompt())
}

func TestOpenInstancePortal_UnknownZoneParamFails(t *testing.T) {
	calls := seedStakeScene(t)
	require.Equal(t, Failure, actOpenInstancePortal(sableParams("moon"), stakeCtx("player_topic", "open_moon")))
	require.Nil(t, requireUser(t, stakeUserId).GetPrompt())
	require.Empty(t, *calls)
}

func TestParseStake_AcceptsGoldSuffixAndCommas(t *testing.T) {
	for in, want := range map[string]int{
		"250": 250, "250 gold": 250, "250g": 250, "250 g": 250, "250 Gold": 250,
		"1,000": 1000, "1,000 gold": 1000, "0": 0,
	} {
		got, ok := parseStake(in)
		require.True(t, ok, "%q is a stake", in)
		require.Equal(t, want, got, "%q", in)
	}
	for _, in := range []string{"", "gold", "lots", "250 silver", "-5", "2 50", "250gg"} {
		_, ok := parseStake(in)
		require.False(t, ok, "%q is not a stake", in)
	}
}

func TestStakePrompt_GoldSuffixOpens(t *testing.T) {
	calls := seedStakeScene(t)
	chooseArenaTopic(t)
	events.DrainQueuedMessagesForTest(stakeUserId)

	outcome, _ := answerStake(t, "250 gold")
	require.Equal(t, StakeHandled, outcome)
	got := captureOutcome(t, calls)
	require.Equal(t, 750, got.gold)
	require.Equal(t, []zoneCall{{"Instance Arena", 250, stakeUserId}}, got.calls)
}

func TestStakePrompt_CloneFailureRefunds(t *testing.T) {
	calls := seedStakeScene(t)
	createZoneInstance = func(zone string, gold int, owner int, authorized []int, overworld int) (*rooms.ZoneInstance, error) {
		*calls = append(*calls, zoneCall{zone, gold, owner})
		return nil, errors.New("no template")
	}
	chooseArenaTopic(t)
	events.DrainQueuedMessagesForTest(stakeUserId)

	outcome, _ := answerStake(t, "250")
	require.Equal(t, StakeHandled, outcome)
	got := captureOutcome(t, calls)
	require.Len(t, got.calls, 1, "fixture check: the clone was attempted")
	require.Equal(t, 1000, got.gold, "a failed clone refunds the stake")
	require.Equal(t, []string{"say The planes resist me. Your gold is returned."}, got.mobSaid)
	require.Empty(t, got.tempExits)
	require.Nil(t, requireUser(t, stakeUserId).GetPrompt())
}

func TestStakePrompt_EmptyLineKeepsThePromptOpen(t *testing.T) {
	calls := seedStakeScene(t)
	chooseArenaTopic(t)
	events.DrainQueuedMessagesForTest(stakeUserId)

	outcome, _ := answerStake(t, "")
	require.Equal(t, StakePending, outcome)
	p := requireUser(t, stakeUserId).GetPrompt()
	require.NotNil(t, p, "Enter on an empty line asks again")
	require.Equal(t, PortalStakeCommand, p.Command)
	require.NotNil(t, p.GetNextQuestion())
	got := captureOutcome(t, calls)
	require.Equal(t, 1000, got.gold)
	require.Empty(t, got.calls)
	require.Empty(t, got.userLines, "nothing is said; the question stays on the prompt line")
}

func TestStakePrompt_RoomChangeClosesWithTheSetAsideLine(t *testing.T) {
	seedStakeScene(t)
	chooseArenaTopic(t)
	events.DrainQueuedMessagesForTest(stakeUserId)

	u := requireUser(t, stakeUserId)
	u.CloseMenusForRoomChange()
	require.Nil(t, u.GetPrompt(), "walking away closes the amount prompt")
	require.Contains(t, strings.Join(events.DrainQueuedMessagesForTest(stakeUserId), "\n"),
		"You name no stake, so the way stays closed.")
}

func TestAnswerStake_NoPromptIsNotHandled(t *testing.T) {
	seedStakeScene(t)
	outcome, _ := AnswerStake(requireUser(t, stakeUserId))
	require.Equal(t, StakeNoPrompt, outcome)
}
