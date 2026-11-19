package events

import (
	"container/heap"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/GoMudEngine/GoMud/internal/util"
)

type EventType string

var (
	qLock     = sync.Mutex{}
	allQueues = map[string]*Queue[Event]{}

	requeues = []requeue{}

	globalQueue  priorityQueue
	orderCounter uint64                      // global counter to maintain insertion order.
	uniqueMap    = make(map[string]struct{}) // map to enforce uniqueness

	eventDebugging bool

	// replyWindow counts, per user, the typed lines whose reply is being
	// built (#842): processInput opens one, ProcessEvents closes it as the
	// line's InputDone pops.
	// Guarded by qLock.
	replyWindow = map[int]int{}

	// enteringWorld holds the players who entered the world since the last
	// event pass ended (owner ruling 8, #842): nothing queued for them is
	// marked as a reply until the pass that runs their login burst ends.
	// Guarded by qLock.
	enteringWorld = map[int]struct{}{}

	// tickRunning is true while the listeners of a tick event run: a
	// NewRound or NewTurn, or any event a tick's listeners queued, however
	// deep (#696, owner 2026-11-19: round and turn ticks never join a typed
	// line's reply). What they queue is not marked as a reply, so a craft's
	// outcome, a fight line or a hunger notice goes out at once and is never
	// paged behind a reply or dropped with its unread pages.
	// Guarded by qLock.
	tickRunning bool
)

type requeue struct {
	evt      Event
	priority int
	fromTick bool
}

// Event is the common interface for events.
type Event interface {
	Type() string
}

// Generic events are mostly used for plugins
type GenericEvent interface {
	Event
	Data(name string) any
}

// prioritizedEvent wraps an Event with a priority and an order field.
type prioritizedEvent struct {
	event    Event
	priority int    // Lower numbers indicate higher priority. Default is 0.
	order    uint64 // Used to preserve FIFO order among events with the same priority.
	fromTick bool   // queued by a tick's listeners (tickRunning)
}

// isTick reports whether e is a round or turn tick (#696).
func isTick(e Event) bool {
	switch e.(type) {
	case NewRound, NewTurn:
		return true
	}
	return false
}

// queuedFromTick reports whether an event queued now comes from a tick:
// a tick's listeners are running. A player's typed line never does, though
// a connection goroutine may queue one while a tick runs on MainWorker.
// Any other event queued off MainWorker in that moment is taken as the
// tick's, which only means its lines are not paged: the safe direction.
// The caller holds qLock.
func queuedFromTick(e Event) bool {
	if !tickRunning {
		return false
	}
	if in, ok := e.(Input); ok && in.Flags.Has(CmdTyped) {
		return false
	}
	return true
}

// UniqueEvent is implemented by events that should be unique in the queue.
type uniqueEvent interface {
	Event
	UniqueID() string
}

// PriorityQueue implements heap.Interface for *PrioritizedEvent.
type priorityQueue []*prioritizedEvent

func (pq priorityQueue) Len() int { return len(pq) }

// Less returns true if element i has a higher priority than element j.
// Here, "higher priority" means a lower numeric value. If priorities are equal,
// the one with the lower order (i.e. inserted earlier) is considered higher.
func (pq priorityQueue) Less(i, j int) bool {
	if pq[i].priority == pq[j].priority {
		return pq[i].order < pq[j].order
	}
	return pq[i].priority < pq[j].priority
}

func (pq priorityQueue) Swap(i, j int) { pq[i], pq[j] = pq[j], pq[i] }

func (pq *priorityQueue) Push(x any) {
	item := x.(*prioritizedEvent)
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}

// Enqueue adds an event to the global queue.
// The caller can optionally pass a priority value.
// If omitted, the default priority is 0.
func AddToQueue(e Event, priority ...int) {

	qLock.Lock()
	defer qLock.Unlock()

	prio := 0
	if len(priority) > 0 {
		prio = priority[0]
	}

	// #842: a message for a player whose typed line is being handled is
	// part of that line's reply. Marked here, the one place every message
	// is queued, so no sender has to know. Never while the player is
	// entering the world: the login burst is not a reply (owner ruling 8).
	// Never from a round or turn tick either (#696).
	fromTick := queuedFromTick(e)
	if m, ok := e.(Message); ok && m.UserId > 0 && replyWindow[m.UserId] > 0 && !isEnteringWorld(m.UserId) && !fromTick {
		m.Reply = true
		e = m
	}

	// Check for uniqueness if the event implements UniqueEvent.
	if ue, ok := e.(uniqueEvent); ok {
		uid := ue.UniqueID()
		// If we already have an entry for this uniqueID, skip it!
		if _, exists := uniqueMap[uid]; exists {
			return
		}

		uniqueMap[ue.UniqueID()] = struct{}{}
	}

	orderCounter++
	pe := &prioritizedEvent{
		event:    e,
		priority: prio,
		order:    orderCounter,
		fromTick: fromTick,
	}

	if eventDebugging {
		fmt.Println(`events.AddToQueue`, "type:", e.Type(), `priority`, prio, `order`, orderCounter)
	}

	heap.Push(&globalQueue, pe)
}

// OpenReply starts marking every Message queued for userId as the reply to
// the typed line being handled (#842). world.processInput calls it for a
// line the pager did not take; ProcessEvents closes it when it pops that
// line's InputDone, under qLock, before any listener runs.
func OpenReply(userId int) {
	qLock.Lock()
	defer qLock.Unlock()
	replyWindow[userId]++
}

// CloseReply ends one OpenReply for userId. Closing more than was opened
// does nothing. The game closes windows in ProcessEvents; this is for tests
// and any caller that opened a window no InputDone will close.
func CloseReply(userId int) {
	qLock.Lock()
	defer qLock.Unlock()
	closeReplyLocked(userId)
}

// closeReplyLocked is CloseReply with qLock already held.
func closeReplyLocked(userId int) {
	if replyWindow[userId] <= 1 {
		delete(replyWindow, userId)
		return
	}
	replyWindow[userId]--
}

// EnteringWorld says userId is entering the world (owner ruling 8: the
// login burst is never paged). Until the end of the next event pass, the one
// that runs their PlayerSpawn, login commands and first look, no Message for
// them is marked as a reply, even inside a typed line's window: a line
// handled in that pass ahead of the login commands would otherwise hold the
// message of the day and the room as its reply. world.enterWorld calls it,
// between passes, for every login, reconnect and copyover reconnect.
func EnteringWorld(userId int) {
	qLock.Lock()
	defer qLock.Unlock()
	enteringWorld[userId] = struct{}{}
}

// isEnteringWorld reports whether userId entered the world since the last
// pass ended. qLock must be held.
func isEnteringWorld(userId int) bool {
	_, ok := enteringWorld[userId]
	return ok
}

// Same as AddToQueue but avoids a mutex lock for optimization purposes
// Should only be used when a mutex lock is already held
// fromTick carries over from the event's first queueing (#696).
func reAddToQueue(e Event, prio int, fromTick bool) {

	// Check for uniqueness if the event implements UniqueEvent.
	if ue, ok := e.(uniqueEvent); ok {
		uid := ue.UniqueID()
		// If we already have an entry for this uniqueID, skip it!
		if _, exists := uniqueMap[uid]; exists {
			return
		}

		uniqueMap[ue.UniqueID()] = struct{}{}
	}

	orderCounter++
	pe := &prioritizedEvent{
		event:    e,
		priority: prio,
		order:    orderCounter,
		fromTick: fromTick,
	}

	if eventDebugging {
		fmt.Println(`events.reAddToQueue`, "type:", e.Type(), `priority:`, prio, `order:`, orderCounter)
	}

	heap.Push(&globalQueue, pe)
}

func addToRequeue(e Event, prio int, fromTick bool) {
	qLock.Lock()
	defer qLock.Unlock()

	requeues = append(requeues, requeue{
		evt:      e,
		priority: prio,
		fromTick: fromTick,
	})
}

// ProcessEvents runs the event loop until the queue is empty.
// It processes events one at a time in order of priority.
// Any events enqueued (even from within a handler) will be picked up in order.
func ProcessEvents() {

	// Since this is intended to run frequently and quickly
	// Only sample the runtime 1 in 100 times
	eventCounter := 0
	if eventDebugging || util.Rand(100) == 0 { // seedable (#594)
		start := time.Now()
		defer func() {
			util.TrackTime(`events.ProcessEvents()`, time.Since(start).Seconds())
			if eventDebugging {
				if time.Since(start).Seconds() > 0.00125 {
					fmt.Println(`events.ProcessEvents`, "events handled:", eventCounter, "time taken:", time.Since(start).Seconds())
				}
			}
		}()
	}

	qLock.Lock()

	// Requeues are a special group that has been deferred to the next processevents loop
	// They are added back into the event queue at the top of the process events function
	for _, itm := range requeues {
		reAddToQueue(itm.evt, itm.priority, itm.fromTick)
	}
	requeues = requeues[:0]

	var evtResult ListenerReturn
	for {

		if globalQueue.Len() < 1 {
			break
		}

		pe := heap.Pop(&globalQueue).(*prioritizedEvent)

		if eventDebugging {
			eventCounter++
			fmt.Println(`events.ProcessEvents`, "type:", pe.event.Type(), `remain:`, globalQueue.Len())
		}

		// If this is a unique event, remove it from the uniqueMap.
		if ue, ok := pe.event.(uniqueEvent); ok {
			delete(uniqueMap, ue.UniqueID())
		}

		// #842: a typed line's reply ends as its InputDone pops. Closed here,
		// still under qLock, so nothing queued once it has popped (by its own
		// listeners included) is marked as the reply.
		if done, ok := pe.event.(InputDone); ok {
			closeReplyLocked(done.UserId)
		}

		// #696: a tick, and everything a tick queued, runs as a tick, so
		// nothing it queues joins a typed line's reply.
		tickRunning = pe.fromTick || isTick(pe.event)

		qLock.Unlock()

		evtResult = DoListeners(pe.event)
		if evtResult == CancelAndRequeue {
			addToRequeue(pe.event, pe.priority, pe.fromTick)
		}

		qLock.Lock()
		tickRunning = false

	}

	// The pass that ran the login bursts is over (EnteringWorld).
	clear(enteringWorld)

	qLock.Unlock()
}

func SetDebug(on bool) {
	qLock.Lock()
	defer qLock.Unlock()
	eventDebugging = on
}

// InspectQueuedInputForTest scans the global events queue (without draining it)
// for the first Input event with a matching MobInstanceId whose InputText has
// the given prefix. Returns the InputText on match, or "" if not found.
//
// FOR TEST USE ONLY. Not safe to call from production paths.
func InspectQueuedInputForTest(instanceId int, prefix string) string {
	qLock.Lock()
	defer qLock.Unlock()
	for _, pe := range globalQueue {
		inp, ok := pe.event.(Input)
		if !ok {
			continue
		}
		if inp.MobInstanceId != instanceId {
			continue
		}
		if prefix == "" || len(inp.InputText) >= len(prefix) && inp.InputText[:len(prefix)] == prefix {
			return inp.InputText
		}
	}
	return ""
}

// DrainQueuedCharacterDiedForTest removes all CharacterDied events from the
// global queue and returns them.
//
// FOR TEST USE ONLY. Mutates the queue. Call it once to discard leftovers from
// an earlier test, then again to assert on what the code under test queued.
func DrainQueuedCharacterDiedForTest() []CharacterDied {
	qLock.Lock()
	defer qLock.Unlock()

	var found []CharacterDied
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		if d, ok := pe.event.(CharacterDied); ok {
			found = append(found, d)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

// DrainQueuedCommunicationsForTest removes every queued Communication event and
// returns them in queue order. FOR TEST USE ONLY.
func DrainQueuedCommunicationsForTest() []Communication {
	qLock.Lock()
	defer qLock.Unlock()

	type seen struct {
		order uint64
		ev    Communication
	}
	var found []seen
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		if c, ok := pe.event.(Communication); ok {
			found = append(found, seen{order: pe.order, ev: c})
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)

	sort.Slice(found, func(i, j int) bool { return found[i].order < found[j].order })
	out := make([]Communication, 0, len(found))
	for _, f := range found {
		out = append(out, f.ev)
	}
	return out
}

// DrainQueuedBroadcastEventsForTest removes every queued Broadcast event and
// returns the whole events in queue order (DrainQueuedBroadcastsForTest returns
// only their Text). FOR TEST USE ONLY.
func DrainQueuedBroadcastEventsForTest() []Broadcast {
	qLock.Lock()
	defer qLock.Unlock()

	type seen struct {
		order uint64
		ev    Broadcast
	}
	var found []seen
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		if b, ok := pe.event.(Broadcast); ok {
			found = append(found, seen{order: pe.order, ev: b})
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)

	sort.Slice(found, func(i, j int) bool { return found[i].order < found[j].order })
	out := make([]Broadcast, 0, len(found))
	for _, f := range found {
		out = append(out, f.ev)
	}
	return out
}

// DrainQueuedMenuChangedForTest removes the MenuChanged events for userId from
// the global queue and returns them in the order they were queued.
//
// FOR TEST USE ONLY. Mutates the queue. Call it once to discard leftovers from
// an earlier test, then again to assert on what the code under test queued.
func DrainQueuedMenuChangedForTest(userId int) []MenuChanged {
	qLock.Lock()
	defer qLock.Unlock()

	type seen struct {
		order uint64
		ev    MenuChanged
	}
	var found []seen
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		if m, ok := pe.event.(MenuChanged); ok && m.UserId == userId {
			found = append(found, seen{order: pe.order, ev: m})
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)

	sort.Slice(found, func(i, j int) bool { return found[i].order < found[j].order })
	out := make([]MenuChanged, 0, len(found))
	for _, f := range found {
		out = append(out, f.ev)
	}
	return out
}

// DrainQueuedMobDeathsForTest removes all MobDeath events from the global
// queue and returns them.
//
// FOR TEST USE ONLY. Mutates the queue. Call it once to discard leftovers from
// an earlier test, then again to assert on what the code under test queued.
func DrainQueuedMobDeathsForTest() []MobDeath {
	qLock.Lock()
	defer qLock.Unlock()

	var found []MobDeath
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		if d, ok := pe.event.(MobDeath); ok {
			found = append(found, d)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

// DrainQueuedQuestsForTest removes all Quest events from the global queue
// and returns them.
//
// FOR TEST USE ONLY. Mutates the queue. Call it once to discard leftovers from
// an earlier test, then again to assert on what the code under test queued.
func DrainQueuedQuestsForTest() []Quest {
	qLock.Lock()
	defer qLock.Unlock()

	var found []Quest
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		if q, ok := pe.event.(Quest); ok {
			found = append(found, q)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

// DrainQueuedInputsForTest removes all Input events from the global queue for
// the given mob instance id and returns their InputText values.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedInputsForTest(instanceId int) []string {
	qLock.Lock()
	defer qLock.Unlock()
	var found []string
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		inp, ok := pe.event.(Input)
		if ok && inp.MobInstanceId == instanceId {
			found = append(found, inp.InputText)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

// DrainQueuedUserInputsForTest removes all Input events from the global queue
// for the given user id (UserRecord.Command queues them with MobInstanceId 0)
// and returns their InputText values. The player twin of
// DrainQueuedInputsForTest.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedUserInputsForTest(userId int) []string {
	qLock.Lock()
	defer qLock.Unlock()
	var found []string
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		inp, ok := pe.event.(Input)
		if ok && inp.MobInstanceId == 0 && inp.UserId == userId {
			found = append(found, inp.InputText)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

// DrainQueuedBroadcastsForTest removes all Broadcast events from the global
// queue and returns their Text values.
//
// Added for chunk 3.6b-1: autosave now spreads its writes across ticks, so the
// completion broadcast must NOT be emitted at snapshot time. Asserting that
// requires seeing what the hook queued and when, which is otherwise invisible.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedBroadcastsForTest() []string {
	qLock.Lock()
	defer qLock.Unlock()
	var found []string
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		if b, ok := pe.event.(Broadcast); ok {
			found = append(found, b.Text)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

// DrainQueuedMessagesForTest removes all Message events from the global queue
// for the given userId and returns their Text values.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedMessagesForTest(userId int) []string {
	qLock.Lock()
	defer qLock.Unlock()
	var found []string
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		msg, ok := pe.event.(Message)
		if ok && msg.UserId == userId {
			found = append(found, msg.Text)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

// DrainQueuedMessageEventsForTest is DrainQueuedMessagesForTest returning the
// whole Message, so a test can read IsCommunication as well as the text.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedMessageEventsForTest(userId int) []Message {
	qLock.Lock()
	defer qLock.Unlock()
	var found []Message
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		msg, ok := pe.event.(Message)
		if ok && msg.UserId == userId {
			found = append(found, msg)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

// DrainQueuedRoomMessagesForTest removes every RoomId-keyed Message queued for
// roomId (Room.SendTextCommunication, Room.SendTextToExits) and returns them.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedRoomMessagesForTest(roomId int) []Message {
	qLock.Lock()
	defer qLock.Unlock()
	var found []Message
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		msg, ok := pe.event.(Message)
		if ok && msg.UserId == 0 && msg.RoomId == roomId {
			found = append(found, msg)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

// DrainQueuedPlayerAttackedMobsForTest removes all PlayerAttackedMob events
// for the given user and returns them. Pass 0 to drain every such event.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedPlayerAttackedMobsForTest(userId int) []PlayerAttackedMob {
	qLock.Lock()
	defer qLock.Unlock()
	var found []PlayerAttackedMob
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		attacked, ok := pe.event.(PlayerAttackedMob)
		if !ok {
			remaining = append(remaining, pe)
			continue
		}
		if userId == 0 || attacked.UserId == userId {
			found = append(found, attacked)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

// DrainQueuedGoldGivenForTest removes all GoldGiven events for the given
// giver and returns them. Pass 0 to drain every such event.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedGoldGivenForTest(userId int) []GoldGiven {
	qLock.Lock()
	defer qLock.Unlock()
	var found []GoldGiven
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		given, ok := pe.event.(GoldGiven)
		if !ok || (userId != 0 && given.UserId != userId) {
			remaining = append(remaining, pe)
			continue
		}
		found = append(found, given)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

// DrainQueuedHealedForTest removes all Healed events for the given healer
// and returns them. Pass 0 to drain every such event.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedHealedForTest(healerUserId int) []Healed {
	qLock.Lock()
	defer qLock.Unlock()
	var found []Healed
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		healed, ok := pe.event.(Healed)
		if !ok || (healerUserId != 0 && healed.HealerUserId != healerUserId) {
			remaining = append(remaining, pe)
			continue
		}
		found = append(found, healed)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

// DrainQueuedPatrolWaypointArrivalsForTest removes all PatrolWaypointArrival
// events from the global queue for the given mob instance id and returns them.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedPatrolWaypointArrivalsForTest(instanceId int) []PatrolWaypointArrival {
	qLock.Lock()
	defer qLock.Unlock()
	var found []PatrolWaypointArrival
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		arr, ok := pe.event.(PatrolWaypointArrival)
		if ok && arr.MobInstanceId == instanceId {
			found = append(found, arr)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

// DrainQueuedPatrolCompletedForTest pops and returns every queued
// PatrolCompleted event whose MobInstanceId matches. Mirrors the
// drain pattern used by other test helpers in this file. Pass 0
// to drain all (for a global reset between tests). Chunk 3.8.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedPatrolCompletedForTest(instanceId int) []PatrolCompleted {
	qLock.Lock()
	defer qLock.Unlock()
	var found []PatrolCompleted
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		pc, ok := pe.event.(PatrolCompleted)
		if !ok {
			remaining = append(remaining, pe)
			continue
		}
		if instanceId == 0 || pc.MobInstanceId == instanceId {
			found = append(found, pc)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

// DrainQueuedItemOwnershipForTest removes all ItemOwnership events from the
// global queue for the given userId and returns them. Pass 0 to drain every
// ItemOwnership event regardless of owner.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedItemOwnershipForTest(userId int) []ItemOwnership {
	qLock.Lock()
	defer qLock.Unlock()
	var found []ItemOwnership
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		io, ok := pe.event.(ItemOwnership)
		if !ok {
			remaining = append(remaining, pe)
			continue
		}
		if userId == 0 || io.UserId == userId {
			found = append(found, io)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

// DrainQueuedEquipmentChangesForTest removes every EquipmentChange event from
// the global queue and returns them.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedEquipmentChangesForTest() []EquipmentChange {
	qLock.Lock()
	defer qLock.Unlock()
	var found []EquipmentChange
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		if ec, ok := pe.event.(EquipmentChange); ok {
			found = append(found, ec)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

// DrainQueuedVitalsChangedForTest removes all CharacterVitalsChanged events
// from the global queue for the given userId and returns them. Pass 0 to drain
// every CharacterVitalsChanged event regardless of user.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedVitalsChangedForTest(userId int) []CharacterVitalsChanged {
	qLock.Lock()
	defer qLock.Unlock()
	var found []CharacterVitalsChanged
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		vc, ok := pe.event.(CharacterVitalsChanged)
		if !ok {
			remaining = append(remaining, pe)
			continue
		}
		if userId == 0 || vc.UserId == userId {
			found = append(found, vc)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

// DrainQueuedSightBandChangedForTest removes all SightBandChanged events from
// the global queue for the given userId and returns them. Pass 0 to drain
// every SightBandChanged event regardless of user.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedSightBandChangedForTest(userId int) []SightBandChanged {
	qLock.Lock()
	defer qLock.Unlock()
	var found []SightBandChanged
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		sc, ok := pe.event.(SightBandChanged)
		if !ok {
			remaining = append(remaining, pe)
			continue
		}
		if userId == 0 || sc.UserId == userId {
			found = append(found, sc)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

// DrainQueuedSkillUsedForTest removes all SkillUsed events from the global
// queue for the given userId and returns them. Pass 0 to drain every SkillUsed
// event regardless of user.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedSkillUsedForTest(userId int) []SkillUsed {
	qLock.Lock()
	defer qLock.Unlock()
	var found []SkillUsed
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		su, ok := pe.event.(SkillUsed)
		if !ok {
			remaining = append(remaining, pe)
			continue
		}
		if userId == 0 || su.UserId == userId {
			found = append(found, su)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

// DrainQueuedConditionsForTest removes and returns the queued Condition events for a
// user. A userId of 0 drains every queued Condition event, mob ones included, since
// a mob's condition event carries MobInstanceId and leaves UserId zero. Condition
// application is the one door
// that narrates a condition's start, so a test that wants to prove a code path
// applies a player condition correctly asserts on the event, not on the character.
//
// FOR TEST USE ONLY. Mutates the queue. Call it once to discard leftovers from
// an earlier test, then again to assert on what the code under test queued.
func DrainQueuedConditionsForTest(userId int) []Condition {
	qLock.Lock()
	defer qLock.Unlock()
	var found []Condition
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		b, ok := pe.event.(Condition)
		if !ok {
			remaining = append(remaining, pe)
			continue
		}
		if userId == 0 || b.UserId == userId {
			found = append(found, b)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

// DrainQueuedMobConditionsForTest removes and returns the queued Condition
// events for a mob instance, the mob twin of DrainQueuedConditionsForTest. A
// mobInstanceId of 0 drains every queued Condition event, user ones included.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedMobConditionsForTest(mobInstanceId int) []Condition {
	qLock.Lock()
	defer qLock.Unlock()
	var found []Condition
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		b, ok := pe.event.(Condition)
		if !ok {
			remaining = append(remaining, pe)
			continue
		}
		if mobInstanceId == 0 || b.MobInstanceId == mobInstanceId {
			found = append(found, b)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

// DrainAllQueuedEventsForTest removes every event from the global queue,
// whatever its type, and returns how many were removed. A fixture that
// seeds rooms via Room.AddMob (which queues a RoomChange as a side effect)
// should call this in cleanup so no queued event leaks into a later test.
// It also closes every reply window (#842), so a test that fails between
// OpenReply and CloseReply cannot leave later tests' messages marked as a
// reply, and forgets every player entering the world (EnteringWorld).
//
// FOR TEST USE ONLY. Mutates the queue and the reply windows.
func DrainAllQueuedEventsForTest() int {
	qLock.Lock()
	defer qLock.Unlock()
	clear(replyWindow)
	clear(enteringWorld)
	n := len(globalQueue)
	// Free each drained unique event's slot, as ProcessEvents does, or the
	// next one queued (a player's NpcMarkersChanged, say) is dropped as a
	// duplicate of an event that no longer exists.
	for _, pe := range globalQueue {
		if ue, ok := pe.event.(uniqueEvent); ok {
			delete(uniqueMap, ue.UniqueID())
		}
	}
	globalQueue = priorityQueue{}
	heap.Init(&globalQueue)
	return n
}

// DrainQueuedByTypeForTest removes every queued event whose Type() is
// typeName and returns them (heap order, not delivery order). For event types declared outside
// this package (a module's own GMCPOut, say), which the typed drains above
// cannot name.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedByTypeForTest(typeName string) []Event {
	qLock.Lock()
	defer qLock.Unlock()
	var found []Event
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		if pe.event.Type() == typeName {
			found = append(found, pe.event)
			if ue, ok := pe.event.(uniqueEvent); ok {
				delete(uniqueMap, ue.UniqueID()) // as ProcessEvents does
			}
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

// QueuedTypesInOrderForTest returns the Type() of every queued event in the
// order ProcessEvents would pop them, without removing anything.
func QueuedTypesInOrderForTest() []string {
	qLock.Lock()
	defer qLock.Unlock()
	cp := make(priorityQueue, len(globalQueue))
	copy(cp, globalQueue)
	var order []string
	for cp.Len() > 0 {
		order = append(order, heap.Pop(&cp).(*prioritizedEvent).event.Type())
	}
	return order
}

// Initialize the priority queue.
func init() {
	heap.Init(&globalQueue)
}
