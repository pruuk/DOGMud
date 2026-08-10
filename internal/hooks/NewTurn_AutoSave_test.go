package hooks

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/savequeue"
)

func writeFile(t *testing.T, path, content string) error {
	t.Helper()
	return os.WriteFile(path, []byte(content), 0o644)
}

// ---------------------------------------------------------------------------
// The finding-35 contract, under amortised commit.
//
// Autosave used to be three synchronous stages, so each could announce its own
// outcome the moment it finished. It now snapshots on one tick and commits over
// many, so at snapshot time NOTHING has been written yet. A "Done." there would
// be finding 35 in a new costume: telling players the save succeeded before it
// has been attempted.
// ---------------------------------------------------------------------------

// completionBroadcasts filters the queued broadcasts down to the ones that
// claim an outcome, ignoring the "Saving..." progress line.
func completionBroadcasts(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, b := range events.DrainQueuedBroadcastsForTest() {
		if strings.Contains(b, "Done.") || strings.Contains(b, "Saved with errors.") {
			out = append(out, b)
		}
	}
	return out
}

func TestAutoSaveDrain_ReportsNoOutcomeUntilTheLastWriteLands(t *testing.T) {
	dir := t.TempDir()
	q := AutosaveQueue()
	events.DrainQueuedBroadcastsForTest() // clear anything a previous test queued

	// More entries than one tick's budget, so a single drain cannot finish the
	// cycle. The shipped budget is 3; ten guarantees several partial drains
	// regardless of how that default is retuned later.
	pending := make([]savequeue.PendingWrite, 0, 10)
	for i := 0; i < 10; i++ {
		pending = append(pending, savequeue.PendingWrite{
			Kind:    "room",
			Id:      i,
			Path:    fmt.Sprintf("%s/%d.yaml", dir, i),
			Data:    []byte("payload"),
			Careful: true,
		})
	}
	q.Supersede(pending)
	cycleWaiting = true

	// Partial drains must stay silent. The outcome is genuinely unknown while
	// writes remain: any one of them can still fail.
	drainPendingWrites()
	if q.Pending() == 0 {
		t.Fatal("the fixture drained everything in one tick; it cannot test partial drains")
	}
	if got := completionBroadcasts(t); len(got) != 0 {
		t.Fatalf("an outcome was announced with writes still pending: %q", got)
	}

	// Drain to completion.
	for i := 0; i < 20 && q.Pending() > 0; i++ {
		drainPendingWrites()
	}
	drainPendingWrites() // the drain that observes the transition to empty

	got := completionBroadcasts(t)
	if len(got) != 1 {
		t.Fatalf("expected exactly one outcome broadcast at completion, got %d: %q", len(got), got)
	}
	if !strings.Contains(got[0], "Done.") {
		t.Errorf("clean cycle reported %q, want Done.", got[0])
	}
}

func TestAutoSaveDrain_ReportsFailureRatherThanSuccess(t *testing.T) {
	dir := t.TempDir()
	events.DrainQueuedBroadcastsForTest()

	// A path whose parent is a regular file cannot be written on any platform.
	blocker := dir + "/blocker"
	if err := writeFile(t, blocker, "i am a file"); err != nil {
		t.Fatal(err)
	}

	q := AutosaveQueue()
	q.Supersede([]savequeue.PendingWrite{
		{Kind: "room", Id: 1, Path: blocker + "/doomed.yaml", Data: []byte("x"), Careful: true},
	})
	cycleWaiting = true

	for i := 0; i < 10 && q.Pending() > 0; i++ {
		drainPendingWrites()
	}

	got := completionBroadcasts(t)
	if len(got) != 1 {
		t.Fatalf("expected exactly one outcome broadcast, got %d: %q", len(got), got)
	}
	if !strings.Contains(got[0], "Saved with errors.") {
		t.Errorf("a failed cycle reported %q; silence or false success here is finding 35", got[0])
	}
}

func TestAutoSaveDrain_IsCheapWhenThereIsNothingToDo(t *testing.T) {
	// This runs on EVERY turn, so the idle path has to be nearly free. If it
	// ever starts doing work when the queue is empty, that is 20 wasted wakeups
	// a second.
	q := AutosaveQueue()
	for q.Pending() > 0 {
		q.Drain(100)
	}
	cycleWaiting = false
	events.DrainQueuedBroadcastsForTest()

	drainPendingWrites()

	if got := completionBroadcasts(t); len(got) != 0 {
		t.Errorf("the idle drain path announced something: %q", got)
	}
}
