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
