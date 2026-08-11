package plugins

import (
	"fmt"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/savequeue"
)

// pendingCollector captures the writes a plugin's onSave produces, instead of
// letting them go to disk immediately.
//
// Writes are kept in call order and tagged with the plugin that produced them,
// so a callback that fails partway can have its own (possibly half-gathered)
// writes dropped without disturbing anyone else's.
//
// NOT SAFE FOR CONCURRENT USE, AND DELIBERATELY SO. Every caller runs on
// World.MainWorker: PrepareAll drives the callbacks, and the callbacks write
// through the same goroutine. That is the same precondition
// internal/savequeue documents, and for the same reason -- adding a mutex here
// would make the code look safe to call from anywhere while the real hazard is
// two prepares interleaving, which a mutex does not fix. Post the work to
// MainWorker instead.
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

// collecting is the one active collector for the whole registry, or nil.
//
// Set by PrepareAll and cleared before it returns. NOT protected by a mutex,
// and deliberately so: every caller runs on World.MainWorker, the same
// precondition internal/savequeue documents. If you are reaching for a lock
// here, post the work to MainWorker instead.
var collecting *pendingCollector

// slowPrepareThreshold is the point at which a single plugin's prepare is
// reported.
//
// 5ms is what one fsync cost before this work, so crossing it means the
// plugin's prepare now costs as much as the write it replaced -- the point at
// which amortising it stopped helping. A diagnostic tied to a measured physical
// cost, not a balance value, so it is fixed here rather than made a config knob.
//
// A var rather than a const ONLY so tests can lower it. Nothing in production
// assigns to it. As a const the warning branch could not be exercised without a
// real sleep in a test, which would be both slow and flaky.
var slowPrepareThreshold = 5 * time.Millisecond

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
				"prepareTook", took.String(),
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
