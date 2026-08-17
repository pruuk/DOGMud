# Amortise Plugin Saves Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the plugin write out of the world lock so `plugins.Save()` stops costing 20-22 ms of every autosave cycle, without weakening durability.

**Architecture:** `plugins.PrepareAll()` sets a registry-wide collector, runs each plugin's existing `onSave`, and captures the marshalled bytes as `savequeue.PendingWrite`s instead of writing them. Those join the same atomic set as rooms and users and drain through the existing queue. `WriteStruct`/`WriteBytes` keep their exact signatures, so third-party modules are unaffected; only their behaviour changes while a collector is active.

**Tech Stack:** Go, `internal/plugins`, `internal/savequeue`, `internal/hooks`, `gopkg.in/yaml.v2`, standard `testing`.

**Spec:** `docs/superpowers/specs/2026-08-11-plugin-save-amortisation-design.md`

---

## Background you need before starting

Read the spec first. The three things most likely to trip you up:

1. **`internal/savequeue` has no mutex on purpose.** Every caller runs on
   `World.MainWorker`. Do not add locking to make something callable from a
   goroutine. See `internal/savequeue/context.md`.
2. **Guard G2 is the subtle one.** A plugin can write outside an autosave
   (`weather.persistState()` does, every ~20 minutes). If a queued write for the
   same path commits afterwards, it overwrites newer bytes with older ones.
   Task 5 exists entirely for this.
3. **`plugins.Save()` must keep working unchanged** for shutdown and copyover.
   Only the autosave hook switches to `PrepareAll`.

Run tests from the repo root. `internal/plugins` currently has **no test file**;
Task 1 creates the first one, which means it also needs a `TestMain` that sets
up the logger (`mudlog.Error` dereferences a nil logger and panics otherwise —
this has bitten three packages already).

## File Structure

| File | Responsibility |
|---|---|
| `internal/plugins/collector.go` **(create)** | The collector type, the `collecting` package variable, `PrepareAll`, `SetAutosaveQueue`. All new behaviour lives here so `plugins.go` stays a registry file. |
| `internal/plugins/collector_test.go` **(create)** | Tests for everything in `collector.go`, plus `TestMain`. |
| `internal/plugins/plugins.go` **(modify)** | `WriteBytes` gains the collector branch and the G2 cancel. Nothing else changes. |
| `internal/hooks/autosave_prepare.go` **(modify)** | Wire the shared queue into plugins; add plugin writes to the one atomic set. |
| `internal/hooks/NewTurn_AutoSave.go` **(modify)** | Stop calling `plugins.Save()`; the writes now arrive via the prepared set. |
| `internal/plugins/context.md` **(create)** | Package doc, required by repo convention for a package whose API changes. Documents the `onSave` contract. |

---

## Task 1: The collector, with no callers yet

**Files:**
- Create: `internal/plugins/collector.go`
- Create: `internal/plugins/collector_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/plugins/collector_test.go`:

```go
package plugins

import (
	"os"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/savequeue"
)

// mudlog.Error dereferences a package-level logger that is nil until it is set
// up, so any test touching a failure path panics instead of failing. This is
// the first test file in the package, so it owns TestMain.
func TestMain(m *testing.M) {
	mudlog.SetupLogger(nil, "", "", false)
	os.Exit(m.Run())
}

func TestCollector_CollectsInOrderAndReports(t *testing.T) {
	c := newPendingCollector()

	p1 := &Plugin{name: "alpha", version: "1.0"}
	p2 := &Plugin{name: "beta", version: "1.0"}

	c.add(p1, savequeue.PendingWrite{Kind: "plugin", Path: "/tmp/a.dat", Data: []byte("a")})
	c.add(p2, savequeue.PendingWrite{Kind: "plugin", Path: "/tmp/b.dat", Data: []byte("b")})

	got := c.writes()
	if len(got) != 2 {
		t.Fatalf("collected %d writes, want 2", len(got))
	}
	if got[0].Path != "/tmp/a.dat" || got[1].Path != "/tmp/b.dat" {
		t.Errorf("writes out of order: %q then %q", got[0].Path, got[1].Path)
	}
}

// A callback that fails may have collected a PARTIAL snapshot. Persisting half
// of a plugin's state is worse than leaving the previous complete file alone
// for another cycle, so its writes are dropped.
func TestCollector_DiscardDropsOnlyThatPluginsWrites(t *testing.T) {
	c := newPendingCollector()

	good := &Plugin{name: "good", version: "1.0"}
	bad := &Plugin{name: "bad", version: "1.0"}

	c.add(good, savequeue.PendingWrite{Path: "/tmp/good.dat", Data: []byte("g")})
	c.add(bad, savequeue.PendingWrite{Path: "/tmp/bad1.dat", Data: []byte("b1")})
	c.add(bad, savequeue.PendingWrite{Path: "/tmp/bad2.dat", Data: []byte("b2")})

	c.discard(bad)

	got := c.writes()
	if len(got) != 1 {
		t.Fatalf("after discard got %d writes, want 1", len(got))
	}
	if got[0].Path != "/tmp/good.dat" {
		t.Errorf("kept the wrong write: %q", got[0].Path)
	}
}

func TestCollector_DiscardOfAnUnknownPluginIsSafe(t *testing.T) {
	c := newPendingCollector()
	c.add(&Plugin{name: "a", version: "1.0"}, savequeue.PendingWrite{Path: "/tmp/a.dat"})

	c.discard(&Plugin{name: "never-added", version: "1.0"})

	if len(c.writes()) != 1 {
		t.Errorf("discarding an unknown plugin changed the collected set")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/plugins/ -run TestCollector -v`
Expected: FAIL to build, `undefined: newPendingCollector`

- [ ] **Step 3: Write the implementation**

Create `internal/plugins/collector.go`:

```go
package plugins

import (
	"github.com/GoMudEngine/GoMud/internal/savequeue"
)

// pendingCollector captures the writes a plugin's onSave produces, instead of
// letting them go to disk immediately.
//
// Writes are kept in call order and tagged with the plugin that produced them,
// so a callback that fails partway can have its own (possibly half-gathered)
// writes dropped without disturbing anyone else's.
type pendingCollector struct {
	entries []collectedWrite
}

type collectedWrite struct {
	owner *Plugin
	write savequeue.PendingWrite
}

func newPendingCollector() *pendingCollector {
	return &pendingCollector{}
}

func (c *pendingCollector) add(p *Plugin, w savequeue.PendingWrite) {
	c.entries = append(c.entries, collectedWrite{owner: p, write: w})
}

// discard drops every write collected for one plugin.
//
// Called when that plugin's onSave returned an error: it may have written some
// of its identifiers and not others, and persisting a partial snapshot is worse
// than keeping the previous complete file for another cycle.
func (c *pendingCollector) discard(p *Plugin) {
	kept := c.entries[:0]
	for _, e := range c.entries {
		if e.owner != p {
			kept = append(kept, e)
		}
	}
	c.entries = kept
}

func (c *pendingCollector) writes() []savequeue.PendingWrite {
	out := make([]savequeue.PendingWrite, 0, len(c.entries))
	for _, e := range c.entries {
		out = append(out, e.write)
	}
	return out
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/plugins/ -run TestCollector -v`
Expected: PASS, three tests

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/plugins/
git add internal/plugins/collector.go internal/plugins/collector_test.go
git commit -m "feat(plugins): add the pending-write collector for autosave (chunk 4.7 task 1)"
```

---

## Task 2: `WriteBytes` routes through the collector when one is active

**Files:**
- Modify: `internal/plugins/plugins.go:341-371` (the `WriteBytes` body)
- Modify: `internal/plugins/collector.go` (add the `collecting` variable)
- Modify: `internal/plugins/collector_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/plugins/collector_test.go`:

```go
import "path/filepath"  // add to the existing import block

// While a collector is active, WriteBytes must NOT touch the disk.
func TestWriteBytes_CollectsInsteadOfWritingWhileCollecting(t *testing.T) {
	dir := t.TempDir()
	restore := useTempWriteFolder(t, dir)
	defer restore()

	p := &Plugin{name: "probe", version: "1.0"}

	collecting = newPendingCollector()
	defer func() { collecting = nil }()

	if err := p.WriteBytes("state", []byte("payload")); err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}

	got := collecting.writes()
	if len(got) != 1 {
		t.Fatalf("collected %d writes, want 1", len(got))
	}
	if string(got[0].Data) != "payload" {
		t.Errorf("collected data = %q, want %q", got[0].Data, "payload")
	}
	if got[0].Kind != "plugin" {
		t.Errorf("Kind = %q, want plugin", got[0].Kind)
	}

	// Nothing may exist on disk yet.
	if entries, _ := os.ReadDir(filepath.Join(dir, "probe-v1-0")); len(entries) != 0 {
		t.Errorf("WriteBytes wrote to disk while collecting: %d file(s)", len(entries))
	}
}

// With no collector, the existing synchronous behaviour is unchanged.
func TestWriteBytes_WritesSynchronouslyWithNoCollector(t *testing.T) {
	dir := t.TempDir()
	restore := useTempWriteFolder(t, dir)
	defer restore()

	collecting = nil
	p := &Plugin{name: "probe", version: "1.0"}

	if err := p.WriteBytes("state", []byte("payload")); err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}

	got, err := p.ReadBytes("state")
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	if string(got) != "payload" {
		t.Errorf("read back %q, want %q", got, "payload")
	}
}

// useTempWriteFolder points plugin storage at a temp dir for the duration of a
// test. writeFolderPath and writeFolderReady are package-level state set by
// plugins.Load(), which a unit test does not run.
func useTempWriteFolder(t *testing.T, dir string) func() {
	t.Helper()
	prevPath, prevReady := writeFolderPath, writeFolderReady
	writeFolderPath, writeFolderReady = dir, true
	return func() { writeFolderPath, writeFolderReady = prevPath, prevReady }
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/plugins/ -run TestWriteBytes -v`
Expected: FAIL to build, `undefined: collecting`

- [ ] **Step 3: Add the `collecting` variable**

Append to `internal/plugins/collector.go`:

```go
// collecting is the one active collector for the whole registry, or nil.
//
// Set by PrepareAll and cleared before it returns. NOT protected by a mutex,
// and deliberately so: every caller runs on World.MainWorker, the same
// precondition internal/savequeue documents. If you are reaching for a lock
// here, post the work to MainWorker instead.
var collecting *pendingCollector
```

- [ ] **Step 4: Add the collector branch to `WriteBytes`**

In `internal/plugins/plugins.go`, replace the write at the end of `WriteBytes`
(currently the `util.Save` call and its comment) with:

```go
	// Chunk 4.7: while an autosave prepare is collecting, hand over the bytes
	// instead of writing them. The durable write still happens, just on a later
	// tick, through the same queue rooms and users use.
	if collecting != nil {
		collecting.add(p, savequeue.PendingWrite{
			Kind:    "plugin",
			Path:    fullPath,
			Data:    bytes,
			Careful: true,
		})
		return nil
	}

	// Durable atomic write (chunk 2.8). This was a BARE write with no
	// atomicity at all, over plugin state that includes auction history,
	// leaderboards and weather simulation state.
	if err := util.Save(fullPath, bytes); err != nil {
		mudlog.Error(`plugin.WriteBytes`, `name`, p.name, `path`, fullPath, `error`, err)
		return err
	}

	return nil
```

Add `"github.com/GoMudEngine/GoMud/internal/savequeue"` to the import block in
`plugins.go`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/plugins/ -v`
Expected: PASS, five tests

- [ ] **Step 6: Commit**

```bash
gofmt -w internal/plugins/
go build ./...
git add internal/plugins/
git commit -m "feat(plugins): WriteBytes collects instead of writing during a prepare (chunk 4.7 task 2)"
```

---

## Task 3: `PrepareAll`

**Files:**
- Modify: `internal/plugins/collector.go`
- Modify: `internal/plugins/collector_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/plugins/collector_test.go`:

```go
import "errors"  // add to the existing import block

// registerTestPlugin puts a plugin in the registry with an onSave, and removes
// it afterwards. The registry is package-level state shared by all tests.
func registerTestPlugin(t *testing.T, name string, onSave func() error) *Plugin {
	t.Helper()
	p := &Plugin{name: name, version: "1.0"}
	p.Callbacks.SetOnSave(onSave)
	registry = append(registry, p)
	t.Cleanup(func() {
		kept := registry[:0]
		for _, rp := range registry {
			if rp != p {
				kept = append(kept, rp)
			}
		}
		registry = kept
	})
	return p
}

func TestPrepareAll_CollectsFromEveryPlugin(t *testing.T) {
	restore := useTempWriteFolder(t, t.TempDir())
	defer restore()

	var a, b *Plugin
	a = registerTestPlugin(t, "alpha", func() error { return a.WriteBytes("state", []byte("A")) })
	b = registerTestPlugin(t, "beta", func() error { return b.WriteBytes("state", []byte("B")) })

	writes, err := PrepareAll()
	if err != nil {
		t.Fatalf("PrepareAll: %v", err)
	}
	if len(writes) != 2 {
		t.Fatalf("got %d writes, want 2", len(writes))
	}
	if collecting != nil {
		t.Error("collector left active after PrepareAll returned")
	}
}

// One plugin may write several identifiers, so the result is one PendingWrite
// per identifier written, not per plugin.
func TestPrepareAll_OnePluginCanProduceSeveralWrites(t *testing.T) {
	restore := useTempWriteFolder(t, t.TempDir())
	defer restore()

	var p *Plugin
	p = registerTestPlugin(t, "multi", func() error {
		if err := p.WriteBytes("state", []byte("S")); err != nil {
			return err
		}
		return p.WriteBytes("cache", []byte("C"))
	})

	writes, err := PrepareAll()
	if err != nil {
		t.Fatalf("PrepareAll: %v", err)
	}
	if len(writes) != 2 {
		t.Fatalf("got %d writes, want 2 (one per identifier)", len(writes))
	}
}

func TestPrepareAll_FailingPluginDiscardsOnlyItsOwnWrites(t *testing.T) {
	restore := useTempWriteFolder(t, t.TempDir())
	defer restore()

	var good, bad *Plugin
	good = registerTestPlugin(t, "good", func() error { return good.WriteBytes("state", []byte("G")) })
	bad = registerTestPlugin(t, "bad", func() error {
		_ = bad.WriteBytes("partial", []byte("P")) // collected, then the callback fails
		return errors.New("gather failed halfway")
	})

	writes, err := PrepareAll()
	if err == nil {
		t.Fatal("expected an error naming the failed plugin")
	}
	if len(writes) != 1 {
		t.Fatalf("got %d writes, want 1: a failed plugin's partial snapshot must be dropped", len(writes))
	}
	if string(writes[0].Data) != "G" {
		t.Errorf("kept the wrong write: %q", writes[0].Data)
	}
}

// The spec's regression requirement: shutdown and copyover still call
// plugins.Save(), and it must still put bytes on disk. WriteBytes now has two
// modes, so a collector left set would make Save() silently write NOTHING while
// reporting success -- and shutdown is the one moment that cannot be retried.
func TestSave_StillWritesSynchronously(t *testing.T) {
	restore := useTempWriteFolder(t, t.TempDir())
	defer restore()

	collecting = nil
	var p *Plugin
	p = registerTestPlugin(t, "shutdownprobe", func() error { return p.WriteBytes("state", []byte("PERSISTED")) })

	if err := Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := p.ReadBytes("state")
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	if string(got) != "PERSISTED" {
		t.Errorf("Save() left %q on disk, want PERSISTED", got)
	}
}

// A panicking callback must not leave the registry stuck in collecting mode,
// which would make every later synchronous write silently vanish.
func TestPrepareAll_ClearsTheCollectorOnPanic(t *testing.T) {
	restore := useTempWriteFolder(t, t.TempDir())
	defer restore()

	registerTestPlugin(t, "panicker", func() error { panic("boom") })

	defer func() {
		_ = recover()
		if collecting != nil {
			t.Error("collector left active after a panicking callback")
		}
	}()

	_, _ = PrepareAll()
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/plugins/ -run TestPrepareAll -v`
Expected: FAIL to build, `undefined: PrepareAll`

- [ ] **Step 3: Write the implementation**

Append to `internal/plugins/collector.go`:

```go
import (
	"fmt"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// slowPrepareThreshold is the point at which a single plugin's prepare is
// reported.
//
// 5ms is what one fsync cost before this work, so crossing it means the
// plugin's prepare now costs as much as the write it replaced -- the point at
// which amortising it stopped helping. A diagnostic tied to a measured physical
// cost, not a balance value, so it is fixed here rather than made a config knob.
const slowPrepareThreshold = 5 * time.Millisecond

// PrepareAll runs every plugin's onSave with writes COLLECTED rather than
// committed, and returns them for the caller to enqueue.
//
// Caller must hold the world lock: the callbacks read live game state.
//
// A callback that returns an error has its own writes discarded (it may have
// gathered only part of its state) and is named in the returned error; the
// other plugins still prepare.
func PrepareAll() ([]savequeue.PendingWrite, error) {

	collecting = newPendingCollector()
	defer func() { collecting = nil }()

	failed := []string{}

	for _, p := range registry {
		if p.Callbacks.onSave == nil {
			continue
		}

		start := time.Now()
		err := p.Callbacks.onSave()
		took := time.Since(start)

		if took > slowPrepareThreshold {
			// Named, because pluginsMs is an aggregate: without this one slow
			// plugin is indistinguishable from several ordinary ones.
			mudlog.Warn("plugins.PrepareAll",
				"plugin", p.name,
				"prepareMs", took.Milliseconds(),
				"message", "plugin prepare is slow and runs under the world lock",
				"hint", "onSave must do work proportional to the plugin's own state, never to the size of the world")
		}

		if err != nil {
			mudlog.Error("plugins.PrepareAll", "plugin", p.name, "error", err)
			collecting.discard(p)
			failed = append(failed, p.name)
		}
	}

	writes := collecting.writes()

	if len(failed) > 0 {
		return writes, fmt.Errorf("plugins.PrepareAll: %d plugin(s) failed to prepare: %s",
			len(failed), strings.Join(failed, ", "))
	}
	return writes, nil
}
```

Merge that `import` block into the existing one at the top of `collector.go`
rather than adding a second block.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/plugins/ -v`
Expected: PASS, ten tests

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/plugins/
go build ./...
git add internal/plugins/
git commit -m "feat(plugins): PrepareAll collects every plugin's writes for the autosave set (chunk 4.7 task 3)"
```

---

## Task 4: Wire plugins into the one atomic prepare

**Files:**
- Modify: `internal/hooks/autosave_prepare.go:20-22` (the `init`) and `PrepareAutosaveSet`
- Modify: `internal/hooks/NewTurn_AutoSave.go:170-180` (the plugins stage)
- Modify: `internal/hooks/autosave_prepare_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/hooks/autosave_prepare_test.go`:

```go
// Plugins must land in the SAME set as rooms and users. A bid deducts player
// gold AND writes auction history, so preparing them in separate passes carries
// the tear risk G1 exists to prevent.
func TestPrepareAutosaveSet_IncludesPluginWrites(t *testing.T) {
	dir := withAutosaveTestEnv(t)
	seedG1Room(t, dir, 920003)

	writes, err := PrepareAutosaveSet()
	if err != nil {
		t.Fatalf("PrepareAutosaveSet: %v", err)
	}

	kinds := map[string]int{}
	for _, w := range writes {
		kinds[w.Kind]++
	}
	if kinds["room"] == 0 {
		t.Error("no room writes in the set")
	}
	if kinds["plugin"] == 0 {
		t.Error("no plugin writes in the set; plugins are preparing in a separate pass")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/hooks/ -run TestPrepareAutosaveSet_IncludesPluginWrites -v`
Expected: FAIL, "no plugin writes in the set"

- [ ] **Step 3: Add plugins to the prepare pass**

In `internal/hooks/autosave_prepare.go`, add the import
`"github.com/GoMudEngine/GoMud/internal/plugins"`, then replace the body of
`PrepareAutosaveSet` with:

```go
	roomWrites, roomErr := rooms.PrepareAllInstanceWrites()
	userWrites, userErr := users.PrepareAllUserWrites()
	pluginWrites, pluginErr := plugins.PrepareAll()

	writes := make([]savequeue.PendingWrite, 0, len(roomWrites)+len(userWrites)+len(pluginWrites))
	writes = append(writes, roomWrites...)
	writes = append(writes, userWrites...)
	writes = append(writes, pluginWrites...)

	// A prepare failure for one entity must not discard the rest of the
	// snapshot: the other players' progress is still worth persisting. Report
	// the first problem and hand back everything that did prepare.
	if roomErr != nil {
		return writes, roomErr
	}
	if userErr != nil {
		return writes, userErr
	}
	return writes, pluginErr
```

- [ ] **Step 4: Stop the hook saving plugins synchronously**

In `internal/hooks/NewTurn_AutoSave.go`, delete the whole `SAVE ALL PLUGINS`
block (the `pluginsStart` / `plugins.Save()` / `pluginsDur` lines and the
`if pluginErr != nil` broadcast), and remove `pluginsMs` from the
`mudlog.Info("AutoSave", "stage", "prepared", ...)` call. Remove the now-unused
`plugins` import from that file.

Replace the log line's `pluginsMs` argument with `pluginWrites`, computed by
counting entries of `Kind == "plugin"` in the prepared set:

```go
	pluginWrites := 0
	for _, w := range writes {
		if w.Kind == "plugin" {
			pluginWrites++
		}
	}

	mudlog.Info("AutoSave",
		"stage", "prepared",
		"prepareMs", prepDur.Milliseconds(),
		"pendingWrites", len(writes),
		"loadedRooms", loadedRooms,
		"activeUsers", activeUsers,
		"pluginWrites", pluginWrites,
	)
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/hooks/ -v`
Expected: PASS, including the new test

- [ ] **Step 6: Commit**

```bash
gofmt -w internal/hooks/
go build ./...
git add internal/hooks/
git commit -m "feat(autosave): fold plugin writes into the one atomic prepare (chunk 4.7 task 4)"
```

---

## Task 5: Guard G2 — a synchronous plugin write cancels a pending one

This is the task the spec's adversarial review added. Do not skip it.

**Files:**
- Modify: `internal/plugins/collector.go` (add `SetAutosaveQueue`)
- Modify: `internal/plugins/plugins.go` (the synchronous branch of `WriteBytes`)
- Modify: `internal/hooks/autosave_prepare.go:21-23` (the `init`)
- Modify: `internal/plugins/collector_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/plugins/collector_test.go`:

```go
// The stale-write sequence this guard exists for:
//
//	prepare       -> pending write, bytes A
//	plugin tick   -> synchronous write, bytes B
//	drain         -> commits A over B
//
// weather.persistState() writes on its own cadence, so this is a real sequence,
// not a hypothetical one.
func TestWriteBytes_SynchronousWriteCancelsAPendingOne(t *testing.T) {
	dir := t.TempDir()
	restore := useTempWriteFolder(t, dir)
	defer restore()

	q := savequeue.New()
	SetAutosaveQueue(q)
	defer SetAutosaveQueue(nil)

	p := &Plugin{name: "probe", version: "1.0"}

	// A prepare collected the OLD bytes.
	collecting = newPendingCollector()
	if err := p.WriteBytes("state", []byte("OLD")); err != nil {
		t.Fatal(err)
	}
	stale := collecting.writes()
	collecting = nil
	q.Supersede(stale)

	// The plugin then writes NEW bytes on its own cadence.
	if err := p.WriteBytes("state", []byte("NEW")); err != nil {
		t.Fatal(err)
	}
	if q.Pending() != 0 {
		t.Fatalf("pending %d after a synchronous write, want 0", q.Pending())
	}

	q.Drain(10)

	got, err := p.ReadBytes("state")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "NEW" {
		t.Errorf("file holds %q; a stale queued write overwrote a newer one", got)
	}
}

func TestSetAutosaveQueue_NilIsSafe(t *testing.T) {
	dir := t.TempDir()
	restore := useTempWriteFolder(t, dir)
	defer restore()

	SetAutosaveQueue(nil)
	p := &Plugin{name: "probe", version: "1.0"}

	// Must not panic when no queue has been wired in (unit tests, early boot).
	if err := p.WriteBytes("state", []byte("x")); err != nil {
		t.Fatalf("WriteBytes with no queue: %v", err)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/plugins/ -run 'SynchronousWriteCancels|SetAutosaveQueue' -v`
Expected: FAIL to build, `undefined: SetAutosaveQueue`

- [ ] **Step 3: Add the queue reference**

Append to `internal/plugins/collector.go`:

```go
// autosaveQueue is the shared pending set, injected at startup by the autosave
// hook. Nil until then, and in unit tests.
var autosaveQueue *savequeue.Queue

// SetAutosaveQueue points this package at the pending set shared with rooms and
// users. Called once at startup.
func SetAutosaveQueue(q *savequeue.Queue) {
	autosaveQueue = q
}

// cancelPending drops any queued write for path.
//
// Guard G2. A queued entry holds an OLDER snapshot by definition, so letting it
// commit after a synchronous write would roll the plugin's state backwards.
// Plugins write outside autosave on their own cadence (weather.persistState
// does, every ~20 minutes), which is exactly when this happens.
func cancelPending(path string) {
	if autosaveQueue == nil {
		return
	}
	autosaveQueue.Cancel(path)
}
```

- [ ] **Step 4: Call it from the synchronous branch**

In `internal/plugins/plugins.go`, in `WriteBytes`, immediately before the
`util.Save` call added in Task 2:

```go
	// Guard G2 (chunk 4.7). See cancelPending.
	cancelPending(fullPath)

	// Durable atomic write (chunk 2.8). This was a BARE write with no
	// atomicity at all, over plugin state that includes auction history,
	// leaderboards and weather simulation state.
	if err := util.Save(fullPath, bytes); err != nil {
```

- [ ] **Step 5: Wire the queue in at startup**

In `internal/hooks/autosave_prepare.go`, extend the `init`:

```go
func init() {
	users.SetAutosaveQueue(autosaveQueue)
	plugins.SetAutosaveQueue(autosaveQueue)
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/plugins/ ./internal/hooks/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
gofmt -w internal/plugins/ internal/hooks/
go build ./...
git add internal/plugins/ internal/hooks/
git commit -m "fix(plugins): cancel a pending write when a plugin writes synchronously (chunk 4.7 task 5, guard G2)"
```

---

## Task 6: `context.md` and the `onSave` contract

**Files:**
- Create: `internal/plugins/context.md`

- [ ] **Step 1: Write the file**

Create `internal/plugins/context.md`. Required by repo convention for a package
whose API changes, and it is where the `onSave` contract now lives.

```markdown
# internal/plugins

## Purpose

Registry and lifecycle for GoMud plugins (the `modules/` tree). Owns plugin
registration, config, embedded files, exported functions, and the persistence
helpers plugins use to store their own state.

Deliberately does NOT decide when plugin state is written. Since chunk 4.7 the
autosave hook drives that.

## Files

- `plugins.go` — registry, lifecycle callbacks, `WriteBytes`/`ReadBytes` and the
  struct helpers over them.
- `plugincallbacks.go` — the callback set a plugin can register, including
  `onSave`.
- `collector.go` — chunk 4.7: `PrepareAll`, the pending-write collector, and the
  G2 cancellation seam.

## The `onSave` contract

`onSave` runs **under the world lock**. It may gather live state and marshal it,
but only work proportional to the plugin's OWN state — never work proportional
to the size of the world (all rooms, all players, all mobs). It must not do
I/O; the write is scheduled for you.

`modules/auctions.save` is the reference example: it snapshots six NPC wallet
balances from a compile-time-fixed slice and marshals. Bounded, tiny, correct.
A plugin that walked every room would have the identical shape and be
catastrophic.

A prepare taking longer than 5ms is logged at WARN, naming the plugin.

## Public API

```go
func PrepareAll() ([]savequeue.PendingWrite, error)  // collect, don't write
func SetAutosaveQueue(q *savequeue.Queue)            // inject the shared queue
func Save() error                                    // synchronous, for shutdown/copyover
func (p *Plugin) WriteBytes(identifier string, b []byte) error
func (p *Plugin) WriteStruct(identifier string, in any) error
func (p *Plugin) ReadBytes(identifier string) ([]byte, error)
func (p *Plugin) ReadIntoStruct(identifier string, out any) error
```

## Gotchas

**`WriteBytes` has two modes and the signature does not show it.** While
`PrepareAll` is collecting, it captures bytes and returns nil without touching
the disk. Otherwise it writes synchronously. That is what keeps the plugin API
source-compatible for third-party modules.

**Writes made OUTSIDE a prepare must stay synchronous.** They are not part of
the atomic set, and guard G3 in `internal/savequeue` DISCARDS an undrained set
on supersede — so a write enqueued from a plugin's own tick could be silently
thrown away by the next autosave. That is data loss, not a delay. The collector
being active only during `PrepareAll` gives the right behaviour for free.

**A synchronous write cancels a pending one for the same path** (guard G2),
because the queued entry holds older bytes. `weather.persistState` writes on its
own cadence, so this is a live sequence, not a theoretical one.

**`collecting` and `autosaveQueue` carry no mutex, deliberately.** Every caller
runs on `World.MainWorker`. See `internal/savequeue/context.md`.

**`Save()` is still the synchronous path** and is what shutdown and copyover
call. Only the autosave hook uses `PrepareAll`.

## Dependencies

`internal/savequeue`, `internal/util`, `internal/mudlog`, `internal/configs`,
`internal/usercommands`, `internal/mobcommands`.

## Consumers

`internal/hooks` (autosave), `copyover.go`, `main.go`, and every module under
`modules/`.
```

- [ ] **Step 2: Commit**

```bash
git add internal/plugins/context.md
git commit -m "docs(plugins): context.md and the onSave contract (chunk 4.7 task 6)"
```

---

## Task 7: Verify, measure, and ship

**Files:**
- Modify: `docs/PATCH_NOTES.md`
- Modify: `docs/roadmaps/ADVERSARIAL_REVIEW_REMEDIATION_ROADMAP.md`

- [ ] **Step 1: Full local gates**

```bash
gofmt -l internal/ modules/          # must print nothing
go build ./...
go test ./internal/... ./modules/... .
golangci-lint run --new-from-rev=HEAD
```

Expected: all pass. `internal/relationships` may fail to build locally because
Windows Defender quarantines its fresh test binary; that is known and CI runs it.

- [ ] **Step 2: Boot test with autosave forced fast**

```bash
git worktree add --detach C:/tmp/dogmud-boot-47 HEAD
cd "C:/Users/Calabe Davis/workspace/DOGMud" && git diff > C:/tmp/47.patch
cd C:/tmp/dogmud-boot-47 && git apply C:/tmp/47.patch
cp "C:/Users/Calabe Davis/workspace/DOGMud/_datafiles/config.yaml" _datafiles/config.yaml
```

Then force a cycle inside the test window, and run:

```bash
python3 -c "
import io
f='_datafiles/config.yaml'
s=io.open(f,encoding='utf-8').read()
s=s.replace('  RoundsPerAutoSave: 225','  RoundsPerAutoSave: 15')
io.open(f,'w',encoding='utf-8',newline='').write(s)
"
```


```bash
timeout 200 go run . > boot.log 2>&1; echo "exit=$?"
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
grep "AutoSave" boot.log | grep -v Config
```

**Exit code 124 is the success case** — the timeout fired because the server
stayed up.

Expected: `prepareMs` roughly unchanged from today's 3-5ms (the plugin marshal
is microseconds), `pluginWrites=4`, `turnsDelayed=0`, and the four plugin files
under `_datafiles/world/dogmud/plugin-data/` still rewritten.

Compare against the pre-change baseline of `pluginsMs=20-22`.

Copy in any untracked files by hand: `git diff` only carries tracked ones, and a
new file silently missing from the worktree has produced a false 404 before.

- [ ] **Step 3: Clean up the worktree**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git worktree remove --force C:/tmp/dogmud-boot-47
rm -rf C:/tmp/dogmud-boot-47 C:/tmp/47.patch
git worktree prune
```

- [ ] **Step 4: Patch notes**

Add a dated entry to `docs/PATCH_NOTES.md`, newest first. Player-facing framing,
no raw numbers, no em dashes, wrapped at 80 characters. The substance: the last
part of saving that briefly held the game still now happens in the background
like the rest of it.

- [ ] **Step 5: Roadmap**

Mark chunk 4.7 done in
`docs/roadmaps/ADVERSARIAL_REVIEW_REMEDIATION_ROADMAP.md`, with the measured
before/after and a note that guard G2 for plugins was found by adversarial
review of the spec rather than in the original design.

- [ ] **Step 6: Ship**

```bash
git add docs/
git commit -m "docs: chunk 4.7 complete, plugin saves amortised"
git push -u origin perf/chunk-4.7-plugin-save-amortisation
gh pr create --repo pruuk/DOGMud --base master --head perf/chunk-4.7-plugin-save-amortisation --fill
gh pr checks <n> --repo pruuk/DOGMud --watch
```

`gh` defaults to the fork PARENT, so `--repo pruuk/DOGMud` is required on every
command. A bare `gh pr create` opened a PR against upstream on 2026-08-08.

Check annotations before merging — a green check is not proof:

```bash
gh api repos/pruuk/DOGMud/check-runs/<id>/annotations
```

---

## Self-review notes

**Spec coverage.** Every spec section maps to a task: the collector and
`PrepareAll` (1-3), folding into the atomic prepare (4), guard G2 (5), the
`onSave` contract and per-plugin timing (3 and 6), acceptance measurement (7).
Self-review caught one gap: the spec's regression requirement that
`plugins.Save()` still writes synchronously had no test. It matters more than it
looks -- `WriteBytes` now has two modes, so a collector left set would make
`Save()` write nothing while reporting success, and shutdown cannot be retried.
Added to Task 3.
The accepted trade-off (plugin state one drain-cycle staler on a hard crash)
needs no task; it is a consequence, not work.

**Out of scope, per the spec.** Skipping unchanged writes, a writer goroutine,
and cleaning up the stale `weather-v0.1.0/` plugin-data directories.

**Type consistency.** `newPendingCollector`, `collecting`, `autosaveQueue`,
`cancelPending`, `PrepareAll`, `SetAutosaveQueue`, `slowPrepareThreshold` are
used identically in every task. `PendingWrite` fields (`Kind`, `Path`, `Data`,
`Careful`) match `internal/savequeue`. `p.Callbacks.onSave` is the real field
name, lowercase and package-private, which is why `PrepareAll` can read it
directly from inside `internal/plugins`.
