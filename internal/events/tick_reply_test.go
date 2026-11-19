package events

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// #696 (owner 2026-11-19): a round or turn tick never joins a typed line's
// reply. A tick handled in the same pass as a typed line, while its window is
// open, used to have its lines marked as the reply: a craft's outcome could be
// paged behind a long answer and dropped with the unread pages. Everything
// the tick queues, however deep, goes out at once; the typed line's own text
// is still its reply, even a typed line queued while the tick runs.
func TestTick_NeverJoinsAReply(t *testing.T) {
	DrainAllQueuedEventsForTest()
	defer DrainAllQueuedEventsForTest()
	restore := SwapListenersForTest()
	defer restore()

	var delivered []Message
	RegisterListener(NewRound{}, func(e Event) ListenerReturn {
		AddToQueue(Message{UserId: 7, Text: "the craft is done\n"})
		AddToQueue(ItemOwnership{UserId: 7, Gained: true})
		// A connection goroutine may queue a typed line while a tick runs.
		AddToQueue(Input{UserId: 7, InputText: "look", Flags: CmdTyped})
		return Continue
	})
	RegisterListener(NewTurn{}, func(e Event) ListenerReturn {
		AddToQueue(Message{UserId: 7, Text: "a turn line\n"})
		return Continue
	})
	RegisterListener(ItemOwnership{}, func(e Event) ListenerReturn {
		AddToQueue(Message{UserId: 7, Text: "a quest step, two events deep\n"})
		return Continue
	})
	RegisterListener(Input{}, func(e Event) ListenerReturn {
		AddToQueue(Message{UserId: 7, Text: "the typed line's answer\n"})
		return Continue
	})
	RegisterListener(Message{}, func(e Event) ListenerReturn {
		delivered = append(delivered, e.(Message))
		return Continue
	})

	OpenReply(7)
	AddToQueue(Message{UserId: 7, Text: "the reply\n"})
	AddToQueue(NewRound{RoundNumber: 1})
	AddToQueue(NewTurn{TurnNumber: 1})
	AddToQueue(InputDone{UserId: 7, Seq: 1}, InputDonePriority)
	ProcessEvents()

	reply := map[string]bool{}
	for _, m := range delivered {
		reply[m.Text] = m.Reply
	}
	require.Len(t, reply, 5, "%v", reply)
	require.True(t, reply["the reply\n"], "the typed line's own text is its reply")
	require.False(t, reply["the craft is done\n"], "a round tick's line is not")
	require.False(t, reply["a turn line\n"], "a turn tick's line is not")
	require.False(t, reply["a quest step, two events deep\n"], "nor anything a tick queued")
	require.True(t, reply["the typed line's answer\n"], "a typed line queued during a tick is still a reply")
}
