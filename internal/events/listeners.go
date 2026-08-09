package events

import (
	"runtime/debug"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

type ListenerReturn int8

type ListenerId uint64

type ListenerWrapper struct {
	id       ListenerId
	listener Listener
	isFinal  bool
}

// Return false to stop further handling of this event.
type Listener func(Event) ListenerReturn
type QueueFlag int

var (
	listenerLock = sync.RWMutex{}
	// listeners that want to handle an event first.
	listenerCt          ListenerId = 0
	eventListeners      map[string][]ListenerWrapper
	hasWildcardListener bool = false

	eventsWithoutListeners map[string]int = map[string]int{}
)

const (
	NoListenerSampleSize = 20

	First QueueFlag = 1
	Last  QueueFlag = 2
	//
	// Event return codes
	//
	// Convention for the "wrong event type" branch: return Continue, not
	// Cancel. A listener that cannot interpret an event has no business
	// vetoing it for every other listener, which is what Cancel does.
	//
	// The codebase is currently split on this (roughly 18 Cancel vs 23
	// Continue across the type-assertion branches in internal/hooks and
	// world.go). The split is harmless today because that branch is
	// unreachable — DoListeners dispatches by e.Type(), so a listener only ever
	// receives the type it registered for, which TestDispatchRoutesOnlyMatchingTypes
	// pins. New code should use Continue; existing sites are not worth churning
	// for a branch that cannot execute.
	//
	// Allows the event to continu to the next listener
	Continue ListenerReturn = 0b00000001
	// Cancels any further processing of the event
	Cancel ListenerReturn = 0b00000010
	// Cancels processing, but adds back into the queue for the next event loop.
	CancelAndRequeue ListenerReturn = 0b00000100
)

func ClearListeners() {
	listenerLock.Lock()
	defer listenerLock.Unlock()
	eventListeners = map[string][]ListenerWrapper{}
}

// Returns an ID for the listener which can be used to unregister later.
func RegisterListener(emptyEvent any, cbFunc Listener, qFlag ...QueueFlag) ListenerId {
	listenerLock.Lock()
	defer listenerLock.Unlock()

	if eventListeners == nil {
		eventListeners = map[string][]ListenerWrapper{}
	}

	listenerCt++

	eType := `*`

	if emptyEvent != nil {
		if evt, ok := emptyEvent.(Event); ok {
			eType = evt.Type()
		} else if evtString, ok := emptyEvent.(string); ok {
			eType = evtString
		}
	}

	if _, ok := eventListeners[eType]; !ok {
		eventListeners[eType] = []ListenerWrapper{}
	}

	listenerDetails := ListenerWrapper{
		id:       listenerCt,
		listener: cbFunc,
		isFinal:  len(qFlag) > 0 && qFlag[0] == Last,
	}

	frontOfQueue := len(qFlag) > 0 && qFlag[0] == First

	if frontOfQueue {
		eventListeners[eType] = append([]ListenerWrapper{listenerDetails}, eventListeners[eType]...)

	} else if listenerDetails.isFinal {
		eventListeners[eType] = append(eventListeners[eType], listenerDetails)

	} else { // end of the list, but before any "final" listeners

		insertPosition := 0

		for idx := len(eventListeners[eType]) - 1; idx >= 0; idx-- {
			// If we're looking at a "final" listener, we can't go any farther down the list
			if !eventListeners[eType][idx].isFinal {
				insertPosition = idx + 1
				break
			}
		}

		eventListeners[eType] = append(eventListeners[eType], ListenerWrapper{})
		copy(eventListeners[eType][insertPosition+1:], eventListeners[eType][insertPosition:])
		eventListeners[eType][insertPosition] = listenerDetails
	}

	// Write it to debug out
	//mudlog.Debug("Listener Registered", "Event", eType, "Function", runtime.FuncForPC(reflect.ValueOf(cbFunc).Pointer()).Name())

	if eType == `*` {
		hasWildcardListener = true
	}

	return listenerCt
}

// Returns true if listener found and removed.
func UnregisterListener(emptyEvent Event, id ListenerId) bool {

	listenerLock.Lock()
	defer listenerLock.Unlock()

	eType := `*`
	if emptyEvent != nil {
		eType = emptyEvent.Type()
	}

	if vals, ok := eventListeners[eType]; ok {

		for idx, wrapper := range vals {
			if wrapper.id == id {
				vals = append(vals[:idx], vals[idx+1:]...)
				eventListeners[eType] = vals
				return true
			}
		}
	}

	if eType == `*` {
		hasWildcardListener = len(eventListeners[eType]) > 0
	}

	return false

}

// invokeListenerSafely runs a single listener with panic recovery, so one
// misbehaving handler cannot take down the process.
//
// DoListeners is the sole dispatch point for combat rounds, quest events,
// command execution and mob AI — a surface spanning ~150 files — and neither it
// nor anything above it (ProcessEvents, EventLoop, MainWorker) recovered. A nil
// dereference or failed type assertion anywhere in that surface killed the
// server and disconnected every player.
//
// A panicking listener is treated as Continue: the event proceeds to the
// remaining listeners rather than being silently cancelled, so one broken
// handler degrades to "that handler did nothing this round".
//
// This mirrors the recovery already applied to individual callbacks elsewhere
// (behaviortree.invokePlannerSafely, goals.invokeContextScore).
func invokeListenerSafely(lw ListenerWrapper, e Event) (ret ListenerReturn) {

	defer func() {
		if r := recover(); r != nil {
			mudlog.Error(`DoListeners`,
				`error`, `listener panicked`,
				`event`, e.Type(),
				`panic`, r,
				`stack`, string(debug.Stack()))
			ret = Continue
		}
	}()

	return lw.listener(e)
}

// snapshotListeners copies the listeners that should run for this event,
// wildcard first, under a READ lock. Returning a copy is what lets DoListeners
// invoke callbacks with no lock held.
func snapshotListeners(eventType string) (toRun []ListenerWrapper, found bool) {

	listenerLock.RLock()
	defer listenerLock.RUnlock()

	if len(eventListeners) == 0 {
		return nil, false
	}

	// wildcard listener is really for debugging purpose
	if hasWildcardListener {
		if vals, ok := eventListeners[`*`]; ok {
			found = true
			toRun = append(toRun, vals...)
		}
	}

	if vals, ok := eventListeners[eventType]; ok {
		found = true
		toRun = append(toRun, vals...)
	}

	return toRun, found
}

// noteMissingListener counts events nobody handled, sampling the log.
// eventsWithoutListeners used to be guarded incidentally by the dispatch
// write lock; now that dispatch only takes a read lock it needs the write
// lock explicitly, since read locks do not exclude each other.
func noteMissingListener(eventType string) {

	listenerLock.Lock()
	defer listenerLock.Unlock()

	eventsWithoutListeners[eventType] = eventsWithoutListeners[eventType] + 1
	if eventsWithoutListeners[eventType]%NoListenerSampleSize == 0 {
		mudlog.Error(`DoListeners`, "Event", eventType, "error", "no listener for event", "sample-size", NoListenerSampleSize)
	}
}

// DoListeners dispatches an event to every registered listener.
//
// Review finding 22. This used to hold the EXCLUSIVE listenerLock across every
// callback, which had two consequences:
//
//  1. Any listener that registered or removed a listener deadlocked the
//     server outright. sync.RWMutex is not reentrant, so AddListener's
//     Lock() could never be acquired from inside a dispatch that already
//     held it.
//  2. A single slow handler blocked ALL registration and removal for its
//     entire duration.
//
// The listener set is now snapshotted under a read lock and the callbacks run
// with no lock held.
//
// Tradeoff, deliberate: a listener removed after the snapshot but before its
// turn still runs for this one event. That is the standard behaviour for an
// event bus and is strictly preferable to a deadlock. Registration during
// dispatch takes effect on the NEXT event, not this one.
func DoListeners(e Event) ListenerReturn {

	toRun, listenerFound := snapshotListeners(e.Type())

	if !listenerFound {
		noteMissingListener(e.Type())
		return Continue
	}

	for _, lw := range toRun {
		if result := invokeListenerSafely(lw, e); result != Continue {
			return result
		}
	}

	return Continue
}
