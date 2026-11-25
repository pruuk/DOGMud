package events

import (
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/stretchr/testify/require"
)

// pendingFixture pins the cap, empties the queue and the deferred events,
// and swaps in no listeners. Input and InputQueueChanged listeners are the
// test's own.
func pendingFixture(t *testing.T, cap int) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Network.MaxPendingInputs = configs.ConfigInt(cap)
	configs.SetConfigForTest(t, cfg)

	restore := SwapListenersForTest()
	flush := func() {
		// Run what is deferred with nothing listening, so it all pops.
		ProcessEvents()
		DrainAllQueuedEventsForTest()
	}
	flush()
	t.Cleanup(func() { restore(); flush() })
}

func clientLine(userId int, text string) Input {
	return Input{UserId: userId, InputText: text, Flags: CmdTyped | CmdFromClient}
}

// noticesFor drains the queued InputQueueChanged events for userId.
func noticesFor(t *testing.T, userId int) []InputQueueChanged {
	t.Helper()
	var out []InputQueueChanged
	for _, e := range DrainQueuedByTypeForTest(`InputQueueChanged`) {
		if n := e.(InputQueueChanged); n.UserId == userId {
			out = append(out, n)
		}
	}
	return out
}

// cap+1 commands from one client: cap are queued and the extra is refused.
func TestPendingInputs_CapPlusOneLeavesCapQueued(t *testing.T) {
	pendingFixture(t, 10)

	for i := range 10 {
		require.True(t, TryAddToQueue(clientLine(7, "look")), "command %d fits", i+1)
	}
	require.False(t, TryAddToQueue(clientLine(7, "look")), "the eleventh is refused")
	require.Equal(t, 10, PendingInputs(7))
	require.Len(t, DrainQueuedUserInputsForTest(7), 10)
	require.Equal(t, 0, PendingInputs(7), "a drain recounts")
}

// One player's full queue leaves another's alone.
func TestPendingInputs_AnotherPlayerIsUnaffected(t *testing.T) {
	pendingFixture(t, 3)

	for range 4 {
		TryAddToQueue(clientLine(7, "look"))
	}
	require.Equal(t, 3, PendingInputs(7))
	require.True(t, TryAddToQueue(clientLine(8, "look")))
	require.Equal(t, 1, PendingInputs(8))
}

// Commands the engine queues (no CmdFromClient), mob commands and other
// events never count and are never refused.
func TestPendingInputs_EngineCommandsDoNotCount(t *testing.T) {
	pendingFixture(t, 2)

	TryAddToQueue(clientLine(7, "look"))
	TryAddToQueue(clientLine(7, "look"))
	require.False(t, TryAddToQueue(clientLine(7, "look")))

	for range 5 {
		require.True(t, TryAddToQueue(Input{UserId: 7, InputText: "emote"}), "an engine command for a full player")
		require.True(t, TryAddToQueue(Input{UserId: 7, InputText: "say", Flags: CmdTyped}), "typed but not from the client")
		require.True(t, TryAddToQueue(Input{MobInstanceId: 7, InputText: "north", Flags: CmdFromClient}), "a mob")
	}
	require.Equal(t, 2, PendingInputs(7))
}

// Every command that runs leaves the count: after a pass the count is 0 and
// the player can queue a full set again.
func TestPendingInputs_DrainsToZero(t *testing.T) {
	pendingFixture(t, 4)
	ran := 0
	RegisterListener(Input{}, func(e Event) ListenerReturn { ran++; return Continue })

	for range 4 {
		TryAddToQueue(clientLine(7, "look"))
	}
	ProcessEvents()
	require.Equal(t, 4, ran)
	require.Equal(t, 0, PendingInputs(7))
	for range 4 {
		require.True(t, TryAddToQueue(clientLine(7, "look")))
	}
}

// A command its listener defers (CancelAndRequeue: the client pace, a
// command waiting for its turn) still counts, pass after pass, and leaves
// the count once it finally runs. Nothing leaks either way.
func TestPendingInputs_RequeuedCommandsKeepTheirPlace(t *testing.T) {
	pendingFixture(t, 3)
	hold := true
	RegisterListener(Input{}, func(e Event) ListenerReturn {
		if hold {
			return CancelAndRequeue
		}
		return Continue
	})

	for range 3 {
		TryAddToQueue(clientLine(7, "look"))
	}
	for pass := range 3 {
		ProcessEvents()
		require.Equal(t, 3, PendingInputs(7), "pass %d", pass)
		require.False(t, TryAddToQueue(clientLine(7, "look")), "still full on pass %d", pass)
	}

	hold = false
	ProcessEvents() // the deferred three re-enter and run
	require.Equal(t, 0, PendingInputs(7))
}

// A deferred command counts again at once, not only when the pass ends: a
// command arriving later in the same pass still finds the queue full.
func TestPendingInputs_DeferredCommandsCountWithinThePass(t *testing.T) {
	pendingFixture(t, 2)
	RegisterListener(Input{}, func(e Event) ListenerReturn { return CancelAndRequeue })
	lateAccepted := true
	RegisterListener(System{}, func(e Event) ListenerReturn {
		lateAccepted = TryAddToQueue(clientLine(7, "late"))
		return Continue
	})

	TryAddToQueue(clientLine(7, "look"))
	TryAddToQueue(clientLine(7, "look"))
	AddToQueue(System{Command: "probe"}, 10) // pops after both commands
	ProcessEvents()
	require.False(t, lateAccepted, "the two deferred commands still fill the queue")
	require.Equal(t, 2, PendingInputs(7))
}

// A command leaves the count as it pops, before its listener runs, so a
// macro's line does not hold a slot its own pieces need: a full macro of
// cap pieces fits an otherwise empty queue.
func TestPendingInputs_AMacroOfCapPiecesFits(t *testing.T) {
	pendingFixture(t, 10)
	queued := 0
	RegisterListener(Input{}, func(e Event) ListenerReturn {
		if in := e.(Input); in.InputText == "=1" {
			for range 10 {
				if TryAddToQueue(clientLine(7, "piece")) {
					queued++
				}
			}
			return Continue
		}
		return CancelAndRequeue // hold the pieces so the count shows them
	})

	TryAddToQueue(clientLine(7, "=1"))
	ProcessEvents()
	require.Equal(t, 10, queued)
	require.Equal(t, 10, PendingInputs(7))
}

// The count is rebuilt from the queue at the end of every pass, so a count
// that ever went wrong cannot shut a player out: the next pass sets it right.
func TestPendingInputs_AWrongCountHealsInOnePass(t *testing.T) {
	pendingFixture(t, 3)

	qLock.Lock()
	pendingInputs[7] = 99 // as if decrements had been lost
	qLock.Unlock()
	require.False(t, TryAddToQueue(clientLine(7, "look")), "the bad count refuses")

	ProcessEvents()
	require.Equal(t, 0, PendingInputs(7))
	require.True(t, TryAddToQueue(clientLine(7, "look")))
}

// The client is told only on a change: once when the queue fills, once on
// the first refusal of that full spell (for a client without GMCP), and once
// when it frees. Further refusals and further commands say nothing.
func TestPendingInputs_NoticesOnlyOnFullAndFree(t *testing.T) {
	pendingFixture(t, 2)
	hold := true
	RegisterListener(Input{}, func(e Event) ListenerReturn {
		if hold {
			return CancelAndRequeue
		}
		return Continue
	})

	TryAddToQueue(clientLine(7, "look"))
	require.Empty(t, noticesFor(t, 7), "not full yet")
	TryAddToQueue(clientLine(7, "look"))
	require.Equal(t, []InputQueueChanged{{UserId: 7, Depth: 2, Full: true}}, noticesFor(t, 7))

	for range 5 {
		TryAddToQueue(clientLine(7, "look"))
	}
	require.Equal(t, []InputQueueChanged{{UserId: 7, Depth: 2, Full: true, Refused: true}}, noticesFor(t, 7),
		"one refusal notice for the whole full spell")

	ProcessEvents() // both deferred, still full
	require.Empty(t, noticesFor(t, 7), "still full: nothing new")

	hold = false
	ProcessEvents()
	require.Equal(t, []InputQueueChanged{{UserId: 7, Depth: 0}}, noticesFor(t, 7), "freed")
	ProcessEvents()
	require.Empty(t, noticesFor(t, 7), "free stays quiet")

	// A new full spell is announced, and its first refusal reported, again.
	hold = true
	TryAddToQueue(clientLine(7, "look"))
	TryAddToQueue(clientLine(7, "look"))
	TryAddToQueue(clientLine(7, "look"))
	require.ElementsMatch(t, []InputQueueChanged{
		{UserId: 7, Depth: 2, Full: true},
		{UserId: 7, Depth: 2, Full: true, Refused: true},
	}, noticesFor(t, 7), "drained in heap order, so compared as a set")
}

// A full queue is reported free only two below the cap, so steady input at
// the cap (one command run, one arriving, each tick) does not flicker the
// client between full and free. The refusal itself stays at the cap.
func TestPendingInputs_FreeOnlyTwoBelowTheCap(t *testing.T) {
	pendingFixture(t, 4)
	release := 0
	RegisterListener(Input{}, func(e Event) ListenerReturn {
		if release > 0 {
			release--
			return Continue
		}
		return CancelAndRequeue
	})

	for range 4 {
		TryAddToQueue(clientLine(7, "look"))
	}
	require.Len(t, noticesFor(t, 7), 1, "full")

	// Steady input at the cap: one runs, one arrives, several times over.
	for i := range 5 {
		release = 1
		ProcessEvents()
		require.Equal(t, 3, PendingInputs(7))
		require.True(t, TryAddToQueue(clientLine(7, "look")), "the slot inside the band is taken")
		require.Empty(t, noticesFor(t, 7), "no flicker at round %d", i)
	}
	require.False(t, TryAddToQueue(clientLine(7, "look")), "refused at the cap")
	noticesFor(t, 7) // the one refusal notice of this spell

	release = 1
	ProcessEvents()
	require.Empty(t, noticesFor(t, 7), "one below the cap is still full")
	release = 1
	ProcessEvents()
	require.Equal(t, []InputQueueChanged{{UserId: 7, Depth: 2}}, noticesFor(t, 7), "two below frees")
}

// Within one full spell, automatic sends are reported once; a typed line is
// reported again, at most once a second, so it never vanishes silently
// because a tick took the spell's one report.
func TestPendingInputs_TypedRefusalReportedAgainEachSecond(t *testing.T) {
	pendingFixture(t, 1)
	clock := time.Unix(1000, 0)
	refusalClock = func() time.Time { return clock }
	t.Cleanup(func() { refusalClock = time.Now })
	refused := func() int {
		n := 0
		for _, c := range noticesFor(t, 7) {
			if c.Refused {
				n++
			}
		}
		return n
	}
	auto := Input{UserId: 7, InputText: "tick", Flags: CmdFromClient}

	TryAddToQueue(auto) // fills
	noticesFor(t, 7)
	require.False(t, TryAddToQueue(auto))
	require.Equal(t, 1, refused(), "the spell's first refusal")
	TryAddToQueue(auto)
	require.Equal(t, 0, refused(), "automatic sends stay at one")

	TryAddToQueue(clientLine(7, "look"))
	require.Equal(t, 1, refused(), "a typed line is reported though the spell's report went")
	TryAddToQueue(clientLine(7, "look"))
	require.Equal(t, 0, refused(), "not twice in a second")

	clock = clock.Add(time.Second)
	TryAddToQueue(auto)
	require.Equal(t, 0, refused())
	TryAddToQueue(clientLine(7, "look"))
	require.Equal(t, 1, refused(), "again a second later")
}

// QueuedEngineInputs counts a deferred engine command too (a wear waiting
// for its turn sits with the deferred events between passes), and never a
// client command.
func TestQueuedEngineInputs_CountsDeferredCommands(t *testing.T) {
	pendingFixture(t, 10)
	RegisterListener(Input{}, func(e Event) ListenerReturn { return CancelAndRequeue })

	AddToQueue(Input{UserId: 7, InputText: "wear !5"})
	AddToQueue(Input{UserId: 7, InputText: "wear !6", Flags: CmdFromClient})
	require.Equal(t, 1, QueuedEngineInputs(7, "wear !"))
	ProcessEvents() // both deferred
	require.Equal(t, 1, QueuedEngineInputs(7, "wear !"), "the deferred wear still counts")
	require.Equal(t, 0, QueuedEngineInputs(7, "look"))
	require.Equal(t, 0, QueuedEngineInputs(8, "wear !"))
}

// The shipped default is 10 (owner ruling 2026-11-25), and a cap below 1,
// which would refuse every command, falls back to it.
func TestPendingInputs_DefaultCap(t *testing.T) {
	n := configs.Network{MaxPendingInputs: 0}
	n.Validate()
	require.Equal(t, 10, int(n.MaxPendingInputs))
}
