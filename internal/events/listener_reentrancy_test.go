package events

import (
	"sync"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// The unhandled-event path calls mudlog.Error once it crosses the sample
// threshold, and mudlog panics on a nil logger. Initialize it for this
// package's tests so that path is genuinely exercised rather than avoided.
func init() {
	mudlog.SetupLogger(nil, "", "", false)
}

// Finding 22: DoListeners held the EXCLUSIVE listenerLock across every
// callback. A listener that registered or removed a listener deadlocked the
// server (sync.RWMutex is not reentrant), and a slow handler blocked all
// registration for its whole duration.
//
// These tests would hang forever against the old code, so each one runs in a
// goroutine behind a timeout rather than deadlocking the test binary.

type reentrancyEvent struct{ name string }

func (e reentrancyEvent) Type() string { return e.name }

// runBounded fails the test if fn has not finished within d.
func runBounded(t *testing.T, d time.Duration, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("%s did not complete within %s; dispatch is almost certainly holding the listener lock across callbacks", what, d)
	}
}

func resetListeners(t *testing.T) {
	t.Helper()
	listenerLock.Lock()
	eventListeners = map[string][]ListenerWrapper{}
	hasWildcardListener = false
	listenerLock.Unlock()
}

// The headline deadlock: a listener that registers another listener.
func TestDoListeners_ListenerCanRegisterDuringDispatch(t *testing.T) {
	resetListeners(t)

	registered := false
	RegisterListener(reentrancyEvent{name: "reentrancy.add"}, func(e Event) ListenerReturn {
		RegisterListener(reentrancyEvent{name: "reentrancy.other"}, func(Event) ListenerReturn {
			return Continue
		})
		registered = true
		return Continue
	})

	runBounded(t, 5*time.Second, "dispatch with a registering listener", func() {
		DoListeners(reentrancyEvent{name: "reentrancy.add"})
	})

	if !registered {
		t.Error("listener did not run")
	}
}

// The other half: a listener that removes a listener during dispatch.
func TestDoListeners_ListenerCanRemoveDuringDispatch(t *testing.T) {
	resetListeners(t)

	victim := RegisterListener(reentrancyEvent{name: "reentrancy.victim"}, func(Event) ListenerReturn {
		return Continue
	})

	ran := false
	RegisterListener(reentrancyEvent{name: "reentrancy.remove"}, func(e Event) ListenerReturn {
		UnregisterListener(reentrancyEvent{name: "reentrancy.victim"}, victim)
		ran = true
		return Continue
	})

	runBounded(t, 5*time.Second, "dispatch with a removing listener", func() {
		DoListeners(reentrancyEvent{name: "reentrancy.remove"})
	})

	if !ran {
		t.Error("listener did not run")
	}
}

// A slow handler must not block registration on another goroutine.
func TestDoListeners_SlowHandlerDoesNotBlockRegistration(t *testing.T) {
	resetListeners(t)

	inHandler := make(chan struct{})
	release := make(chan struct{})

	RegisterListener(reentrancyEvent{name: "reentrancy.slow"}, func(Event) ListenerReturn {
		close(inHandler)
		<-release
		return Continue
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		DoListeners(reentrancyEvent{name: "reentrancy.slow"})
	}()

	<-inHandler // handler is now mid-flight

	runBounded(t, 5*time.Second, "RegisterListener while a handler is mid-flight", func() {
		RegisterListener(reentrancyEvent{name: "reentrancy.unrelated"}, func(Event) ListenerReturn {
			return Continue
		})
	})

	close(release)
	wg.Wait()
}

// Concurrent dispatch and registration must not race or crash. eventListeners
// is a map, so an unguarded write here would be a fatal error, not a panic.
func TestDoListeners_ConcurrentDispatchAndRegistration(t *testing.T) {
	resetListeners(t)

	RegisterListener(reentrancyEvent{name: "reentrancy.mixed"}, func(Event) ListenerReturn {
		return Continue
	})

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				if g%2 == 0 {
					DoListeners(reentrancyEvent{name: "reentrancy.mixed"})
				} else {
					id := RegisterListener(reentrancyEvent{name: "reentrancy.churn"}, func(Event) ListenerReturn {
						return Continue
					})
					UnregisterListener(reentrancyEvent{name: "reentrancy.churn"}, id)
				}
			}
		}(g)
	}

	runBounded(t, 20*time.Second, "concurrent dispatch and registration", wg.Wait)
}

// An unhandled event still increments its counter. That counter used to be
// guarded incidentally by the dispatch write lock, so this pins that the
// explicit locking replacing it actually runs.
func TestDoListeners_UnhandledEventIsCountedSafely(t *testing.T) {
	resetListeners(t)

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				DoListeners(reentrancyEvent{name: "reentrancy.nobody"})
			}
		}()
	}
	runBounded(t, 20*time.Second, "concurrent unhandled events", wg.Wait)

	listenerLock.RLock()
	got := eventsWithoutListeners["reentrancy.nobody"]
	listenerLock.RUnlock()

	if got != 8*50 {
		t.Errorf("unhandled count = %d, want %d", got, 8*50)
	}
}
