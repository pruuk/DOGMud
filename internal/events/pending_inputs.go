package events

import "github.com/GoMudEngine/GoMud/internal/configs"

// Each player's client may have at most Network.MaxPendingInputs commands
// waiting to run (owner ruling 2026-11-25). A command the client sent is an
// Input carrying CmdFromClient: a typed line, a GMCP Char.Action.Try, or a
// piece of a macro one of those named. Engine-queued Inputs never count.
//
// The count lives here, beside the queue it counts, under qLock:
//
//   - TryAddToQueue adds one as it queues a counted Input, and refuses one
//     past the cap.
//   - ProcessEvents takes one away as it pops a counted Input, before the
//     listeners run, so a macro's line no longer counts when its pieces are
//     queued; a CancelAndRequeue adds it back (addToRequeue).
//   - At the end of every pass the queue is empty apart from the deferred
//     events, and the count is rebuilt from them (recountPendingInputsLocked).
//     A count that ever went wrong is right again within one pass, so a
//     player can never be shut out by a lost decrement. A player who
//     disconnects with commands waiting needs nothing: the commands still
//     pop (processInput finds no user and returns), and the count follows.
//
// Between a pop and its requeue a command from another goroutine may take
// the freed slot, so the queue can briefly hold one more than the cap; the
// next pass's pops cannot dip below the cap again, so it never grows past
// that.

// InputQueueChanged reports a change in one player's pending inputs to their
// client (modules/gmcp sends Char.Input.Queue). It is queued only on a
// transition: Full when the count reaches the cap, not Full when it drops
// below it again, so a full queue is announced once, not once per command.
// Refused is set on the first command refused in a full spell, for a client
// that reads no GMCP and gets one plain line instead.
type InputQueueChanged struct {
	UserId  int
	Depth   int
	Full    bool
	Refused bool
}

func (e InputQueueChanged) Type() string { return `InputQueueChanged` }

var (
	// pendingInputs is the count of each player's queued or deferred
	// CmdFromClient Inputs. Guarded by qLock.
	pendingInputs = map[int]int{}

	// inputQueueFull holds each player whose client was told the queue is
	// full and not yet told it freed. The value is whether a refusal was
	// reported in this full spell. Guarded by qLock.
	inputQueueFull = map[int]bool{}
)

// clientInputUser returns the player a counted Input belongs to, or 0 for
// any other event.
func clientInputUser(e Event) int {
	in, ok := e.(Input)
	if !ok || in.MobInstanceId != 0 || in.UserId < 1 || !in.Flags.Has(CmdFromClient) {
		return 0
	}
	return in.UserId
}

// maxPendingInputs is the shipped cap. Read outside qLock.
func maxPendingInputs() int {
	return int(configs.GetNetworkConfig().MaxPendingInputs)
}

// syncInputQueueLocked queues an InputQueueChanged when userId's count has
// crossed the cap since the client was last told. qLock must be held.
func syncInputQueueLocked(userId int, limit int) {
	depth := pendingInputs[userId]
	_, full := inputQueueFull[userId]
	switch {
	case depth >= limit && !full:
		inputQueueFull[userId] = false
		reAddToQueue(InputQueueChanged{UserId: userId, Depth: depth, Full: true}, 0, false)
	case depth < limit && full:
		delete(inputQueueFull, userId)
		reAddToQueue(InputQueueChanged{UserId: userId, Depth: depth}, 0, false)
	}
}

// refuseInputLocked notes a command refused at the cap: the first refusal of
// a full spell is reported, later ones are not. qLock must be held.
func refuseInputLocked(userId int, limit int) {
	syncInputQueueLocked(userId, limit)
	if inputQueueFull[userId] {
		return
	}
	inputQueueFull[userId] = true
	reAddToQueue(InputQueueChanged{UserId: userId, Depth: pendingInputs[userId], Full: true, Refused: true}, 0, false)
}

// addPendingLocked and takePendingLocked move one player's count by one.
// qLock must be held.
func addPendingLocked(e Event) {
	if uid := clientInputUser(e); uid > 0 {
		pendingInputs[uid]++
	}
}

func takePendingLocked(e Event) {
	uid := clientInputUser(e)
	if uid < 1 {
		return
	}
	if pendingInputs[uid] <= 1 {
		delete(pendingInputs, uid)
		return
	}
	pendingInputs[uid]--
}

// recountPendingInputsLocked rebuilds every count from what is queued and
// deferred. qLock must be held.
func recountPendingInputsLocked() {
	clear(pendingInputs)
	for _, pe := range globalQueue {
		addPendingLocked(pe.event)
	}
	for _, r := range requeues {
		addPendingLocked(r.evt)
	}
}

// settleInputQueuesLocked rebuilds the counts and tells each client whose
// queue filled or freed. ProcessEvents calls it at the end of every pass.
// qLock must be held.
func settleInputQueuesLocked(limit int) {
	recountPendingInputsLocked()
	for uid := range pendingInputs {
		syncInputQueueLocked(uid, limit)
	}
	// Deleting the entry being visited is safe in a range.
	for uid := range inputQueueFull {
		syncInputQueueLocked(uid, limit)
	}
}

// PendingInputs returns how many commands userId's client has waiting to
// run (MaxPendingInputs).
func PendingInputs(userId int) int {
	qLock.Lock()
	defer qLock.Unlock()
	return pendingInputs[userId]
}
