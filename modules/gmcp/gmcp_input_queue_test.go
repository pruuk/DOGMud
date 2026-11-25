package gmcp

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// Owner rulings 2026-11-25: a full queue is told to a GMCP client as
// Char.Input.Queue, only when it fills and when it frees; a client other
// than the web client gets one plain line on the first refusal of a full
// spell.

func inputQueueFrames(t *testing.T, userId int) []GMCPInput_Queue {
	t.Helper()
	var out []GMCPInput_Queue
	for _, o := range drainGMCPOut(t) {
		if o.UserId == userId && o.Module == `Char.Input.Queue` {
			out = append(out, o.Payload.(GMCPInput_Queue))
		}
	}
	return out
}

func TestInputQueue_GMCPClientIsToldFullAndFree(t *testing.T) {
	u := inputDoneUser(t, 9861, 98861, true)
	events.DrainQueuedMessagesForTest(u.UserId)

	inputQueueHandler(events.InputQueueChanged{UserId: u.UserId, Depth: 10, Full: true})
	require.Equal(t, []GMCPInput_Queue{{Depth: 10, Full: true}}, inputQueueFrames(t, u.UserId))

	inputQueueHandler(events.InputQueueChanged{UserId: u.UserId, Depth: 10, Full: true, Refused: true})
	require.Empty(t, inputQueueFrames(t, u.UserId), "a refusal adds no frame: full was already sent")
	lines := events.DrainQueuedMessagesForTest(u.UserId)
	require.Len(t, lines, 1, "a refusal always prints, in the web client too (a typed line would vanish)")
	require.Contains(t, lines[0], refusal.InputQueueFullLine)

	inputQueueHandler(events.InputQueueChanged{UserId: u.UserId, Depth: 9})
	require.Equal(t, []GMCPInput_Queue{{Depth: 9, Full: false}}, inputQueueFrames(t, u.UserId))
}

// A telnet player, with or without GMCP, gets the plain line on a refusal.
// A GMCP telnet client also gets the frames; a plain one gets nothing else.
func TestInputQueue_TelnetGetsOneLineOnRefusal(t *testing.T) {
	for _, gmcpOn := range []bool{false, true} {
		uid := 9862
		if gmcpOn {
			uid = 9863
		}
		u := inputDoneUser(t, uid, uint64(uid*10), false)
		if !gmcpOn {
			gmcpModule.cache.Remove(u.ConnectionId())
		}
		events.DrainQueuedMessagesForTest(u.UserId)

		inputQueueHandler(events.InputQueueChanged{UserId: u.UserId, Depth: 10, Full: true})
		require.Empty(t, events.DrainQueuedMessagesForTest(u.UserId), "filling alone says nothing in text")
		frames := inputQueueFrames(t, u.UserId)
		if gmcpOn {
			require.Len(t, frames, 1)
		} else {
			require.Empty(t, frames)
		}

		inputQueueHandler(events.InputQueueChanged{UserId: u.UserId, Depth: 10, Full: true, Refused: true})
		lines := events.DrainQueuedMessagesForTest(u.UserId)
		require.Len(t, lines, 1, "gmcp=%v", gmcpOn)
		require.Contains(t, lines[0], refusal.InputQueueFullLine)
	}
}

// End to end through the queue: a GMCP client filling its queue sees one
// full frame however many more it sends, then one free frame.
func TestInputQueue_FramesOnlyOnTransitions(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Network.MaxPendingInputs = 3
	configs.SetConfigForTest(t, cfg)
	u := inputDoneUser(t, 9864, 98864, true)
	events.DrainQueuedByTypeForTest(`Input`)
	events.DrainQueuedByTypeForTest(`InputQueueChanged`)
	t.Cleanup(func() {
		events.DrainQueuedByTypeForTest(`Input`)
		events.DrainQueuedByTypeForTest(`InputQueueChanged`)
	})

	for i := 0; i < 8; i++ {
		events.AddToQueue(events.Input{UserId: u.UserId, InputText: "look", Flags: events.CmdFromClient})
	}
	for _, e := range events.DrainQueuedByTypeForTest(`InputQueueChanged`) {
		inputQueueHandler(e)
	}
	require.Equal(t, []GMCPInput_Queue{{Depth: 3, Full: true}}, inputQueueFrames(t, u.UserId))
}

// A Char.Action.Try is a command from the client: it counts, and at the cap
// it is answered "deferred" with nothing queued, so the client tries again.
func TestActionTry_FullQueueIsDeferred(t *testing.T) {
	cd, _ := aiLimitFixture(t, 9865, connections.ConnHuman)
	cfg := configs.GetConfig()
	cfg.Network.MaxPendingInputs = 2
	configs.SetConfigForTest(t, cfg)
	id := cd.ConnectionId()

	_, queued := tryAction(t, id, 1)
	require.Equal(t, 1, queued)

	// Fill the queue, then Try again.
	for i := 0; i < 2; i++ {
		events.AddToQueue(events.Input{UserId: 9865, InputText: "look", Flags: events.CmdFromClient})
	}
	events.DrainQueuedByTypeForTest(`GMCPOut`)
	(&GMCPModule{}).handleCharOp(GMCPCharOp{ConnectionId: id, Command: `Char.Action.Try`,
		Payload: []byte(`{"id":2,"cmd":"look"}`)})
	var result GMCPAction_Result
	for _, o := range drainGMCPOut(t) {
		if o.Module == `Char.Action.Result` {
			result = o.Payload.(GMCPAction_Result)
		}
	}
	require.Equal(t, GMCPAction_Result{Id: 2, Status: "deferred", Reason: actionQueueFullReason}, result)
	require.Len(t, events.DrainQueuedByTypeForTest(`Input`), 2, "only the two that filled it")
}

// The command a Try queues carries CmdFromClient, so it counts and is paced.
func TestActionTry_QueuesAClientCommand(t *testing.T) {
	cd, _ := aiLimitFixture(t, 9866, connections.ConnHuman)
	events.DrainQueuedByTypeForTest(`Input`)
	(&GMCPModule{}).handleCharOp(GMCPCharOp{ConnectionId: cd.ConnectionId(), Command: `Char.Action.Try`,
		Payload: []byte(`{"id":1,"cmd":"look"}`)})
	queued := events.DrainQueuedByTypeForTest(`Input`)
	require.Len(t, queued, 1)
	require.True(t, queued[0].(events.Input).Flags.Has(events.CmdFromClient))
}

// A Try the readiness probe rejects still runs, so the player sees the real
// error, and that command is the client's too: it carries CmdFromClient.
func TestActionTry_RejectedStillQueuesAClientCommand(t *testing.T) {
	cd, _ := aiLimitFixture(t, 9871, connections.ConnHuman)
	events.DrainQueuedByTypeForTest(`Input`)
	events.DrainQueuedByTypeForTest(`GMCPOut`)
	(&GMCPModule{}).handleCharOp(GMCPCharOp{ConnectionId: cd.ConnectionId(), Command: `Char.Action.Try`,
		Payload: []byte(`{"id":1,"cmd":"cast no-such-spell-9871"}`)})
	var status string
	for _, o := range drainGMCPOut(t) {
		if o.Module == `Char.Action.Result` {
			status = o.Payload.(GMCPAction_Result).Status
		}
	}
	require.Equal(t, "rejected", status, "fixture: an unknown spell is rejected")
	queued := events.DrainQueuedByTypeForTest(`Input`)
	require.Len(t, queued, 1)
	require.True(t, queued[0].(events.Input).Flags.Has(events.CmdFromClient))
}

// Char.Automation.Set past a cap stores nothing, tells the player why, and
// re-sends nothing.
func TestAutomationSet_CapsAreRefusedWithALine(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Network.MaxAutomationTriggers = 1
	cfg.Network.MaxAutomationFieldLength = 20
	configs.SetConfigForTest(t, cfg)
	u := menuTestUser(t, 9867, 98867, true)
	events.DrainQueuedMessagesForTest(u.UserId)
	events.DrainQueuedByTypeForTest(`AutomationChanged`)
	t.Cleanup(func() { events.DrainQueuedByTypeForTest(`AutomationChanged`) })

	set := func(payload string) {
		(&GMCPModule{}).handleCharOp(GMCPCharOp{ConnectionId: 98867, Command: `Char.Automation.Set`,
			Payload: []byte(payload)})
	}

	set(`{"kind":"trigger","name":"one","pattern":"*"}`)
	require.Len(t, u.Triggers, 1)
	require.Len(t, events.DrainQueuedByTypeForTest(`AutomationChanged`), 1)

	set(`{"kind":"trigger","name":"two","pattern":"*"}`)
	require.Len(t, u.Triggers, 1, "past the count cap nothing is stored")
	require.Empty(t, events.DrainQueuedByTypeForTest(`AutomationChanged`))
	lines := events.DrainQueuedMessagesForTest(u.UserId)
	require.Len(t, lines, 1)
	require.Contains(t, lines[0], refusal.AutomationLimitText(`triggers`, 1))

	set(`{"kind":"tick","name":"` + strings.Repeat("x", 21) + `"}`)
	require.Empty(t, u.Ticks, "an over-long field is not stored")
	lines = events.DrainQueuedMessagesForTest(u.UserId)
	require.Len(t, lines, 1)
	require.Contains(t, lines[0], refusal.AutomationTooLongText(`tick`))
}

// A tick or trigger with more commands than one chain may hold is refused
// with a line naming the limit, and Char.Automation carries that limit so the
// editors can count against it.
func TestAutomationSet_CommandChainLimit(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Network.MaxPendingInputs = 3
	configs.SetConfigForTest(t, cfg)
	u := menuTestUser(t, 9870, 98870, true)
	events.DrainQueuedMessagesForTest(u.UserId)
	events.DrainQueuedByTypeForTest(`AutomationChanged`)
	t.Cleanup(func() { events.DrainQueuedByTypeForTest(`AutomationChanged`) })

	(&GMCPModule{}).handleCharOp(GMCPCharOp{ConnectionId: 98870, Command: `Char.Automation.Set`,
		Payload: []byte(`{"kind":"trigger","name":"walk","pattern":"*","thenCmds":"n;e;s;w"}`)})
	require.Empty(t, u.Triggers)
	require.Empty(t, events.DrainQueuedByTypeForTest(`AutomationChanged`))
	lines := events.DrainQueuedMessagesForTest(u.UserId)
	require.Len(t, lines, 1)
	require.Contains(t, lines[0], refusal.CommandChainTooLongText(`trigger`, 3))

	require.Equal(t, 3, buildAutomationPayload(nil, nil, nil, nil).MaxCommands)
}

// Zone.Map requests for one player collapse while one waits (owner ruling
// 2026-11-25): however many are queued, one map is built.
func TestZoneMap_UpdatesForOnePlayerCollapse(t *testing.T) {
	events.DrainQueuedByTypeForTest(`GMCPZoneUpdate`)
	t.Cleanup(func() { events.DrainQueuedByTypeForTest(`GMCPZoneUpdate`) })

	for i := 0; i < 5; i++ {
		events.AddToQueue(GMCPZoneUpdate{UserId: 9868})
	}
	events.AddToQueue(GMCPZoneUpdate{UserId: 9869})
	require.Len(t, events.DrainQueuedByTypeForTest(`GMCPZoneUpdate`), 2, "one per player")
}

// A run of Zone.Map.Resync is answered once per tick: the rest wait for the
// next tick (deferred, not dropped), and repeated requests from one
// connection collapse while one waits.
func TestZoneMap_ResyncIsAnsweredOncePerTick(t *testing.T) {
	u := zoneDiffFixture(t)
	g := &GMCPZoneModule{zoneSent: map[int]*zoneSentState{}}
	events.DrainQueuedByTypeForTest(`GMCPZoneUpdate`)
	events.DrainQueuedByTypeForTest(`GMCPZoneResync`)
	t.Cleanup(func() {
		events.DrainQueuedByTypeForTest(`GMCPZoneUpdate`)
		events.DrainQueuedByTypeForTest(`GMCPZoneResync`)
	})

	req := GMCPZoneResync{ConnectionId: u.ConnectionId()}
	require.Equal(t, events.Continue, g.zoneResyncHandler(req))
	require.Equal(t, events.CancelAndRequeue, g.zoneResyncHandler(req), "a second in the same tick waits")
	require.Len(t, events.DrainQueuedByTypeForTest(`GMCPZoneUpdate`), 1, "one map for the tick")

	util.IncrementTurnCount()
	require.Equal(t, events.Continue, g.zoneResyncHandler(req), "the next tick answers it")
	require.Len(t, events.DrainQueuedByTypeForTest(`GMCPZoneUpdate`), 1)

	for i := 0; i < 5; i++ {
		events.AddToQueue(req)
	}
	require.Len(t, events.DrainQueuedByTypeForTest(`GMCPZoneResync`), 1, "one waiting request per connection")

	g.zoneDespawnHandler(events.PlayerDespawn{UserId: u.UserId})
	require.Equal(t, events.Continue, g.zoneResyncHandler(req), "a new login is answered at once")
}
