package connections

import (
	"sync"
	"testing"
)

// TestUpdateClientSettingsKeepsConcurrentWrites: two goroutines each change
// their own field of one connection's settings many times. Every change must
// survive. A get, change and overwrite sequence loses some (#910); the run
// below with OverwriteClientSettings proves the test can see that.
func TestUpdateClientSettingsKeepsConcurrentWrites(t *testing.T) {
	const id ConnectionId = 91000
	const n = 2000
	RegisterTestConnection(id)
	t.Cleanup(func() { UnregisterTestConnection(id) })

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range n {
			UpdateClientSettings(id, func(cs *ClientSettings) { cs.TermTypeReplies++ })
		}
	}()
	go func() {
		defer wg.Done()
		for range n {
			UpdateClientSettings(id, func(cs *ClientSettings) { cs.Display.ScreenHeight++ })
		}
	}()
	wg.Wait()

	cs := GetClientSettings(id)
	if cs.TermTypeReplies != n || cs.Display.ScreenHeight != n {
		t.Fatalf("lost writes: TermTypeReplies %d, ScreenHeight %d, want %d each", cs.TermTypeReplies, cs.Display.ScreenHeight, n)
	}
}

// TestOverwriteClientSettingsCanLoseWrites is the null probe for the test
// above: the old get, change and overwrite pattern under the same load. It
// only logs, because a lost write is likely but not certain on a given run.
func TestOverwriteClientSettingsCanLoseWrites(t *testing.T) {
	const id ConnectionId = 91001
	const n = 2000
	RegisterTestConnection(id)
	t.Cleanup(func() { UnregisterTestConnection(id) })

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range n {
			cs := GetClientSettings(id)
			cs.TermTypeReplies++
			OverwriteClientSettings(id, cs)
		}
	}()
	go func() {
		defer wg.Done()
		for range n {
			cs := GetClientSettings(id)
			cs.Display.ScreenHeight++
			OverwriteClientSettings(id, cs)
		}
	}()
	wg.Wait()

	cs := GetClientSettings(id)
	t.Logf("get, change, overwrite kept TermTypeReplies %d and ScreenHeight %d of %d each", cs.TermTypeReplies, cs.Display.ScreenHeight, n)
}
