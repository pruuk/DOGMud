package rooms

import "sync"

// Read-only room-template cache for the autosave compare path.
//
// Roadmap chunk 4.6 (deferred out of 3.6b-1). Measured 2026-08-10: a room's
// PrepareInstanceWrite costs 0.1215ms, of which the template read alone is
// 0.0983ms -- **81%**. Diff and marshal together are 0.023ms. The template is
// immutable authored content and the autosave pass re-read and re-parsed it
// from disk for every room, every cycle.
//
// WHY THIS IS NOT JUST A CACHE INSIDE LoadRoomTemplate
//
// LoadRoomTemplate must keep returning a FRESH object per call. LoadRoomInstance
// calls it TWICE on purpose: once for the room it returns to the caller, and
// once for a scratch copy it unmarshals the instance overlay onto. Its comment
// states the requirement outright -- the second load "is a fresh read from disk,
// so it is an independent object and cannot share maps or slices with `room`".
//
// Handing both of those the same pointer would make the overlay unmarshal write
// straight into the live room, which is review finding 15 (the half-applied
// template/runtime hybrid) coming back by another route.
//
// So the cache is a separate accessor with a narrower contract, used by exactly
// one caller: the autosave diff, which only ever READS the template.

var (
	templateCacheMu sync.RWMutex
	templateCache   = map[int]*Room{}
)

// templateForCompare returns a SHARED, READ-ONLY parsed template.
//
// THE RETURNED ROOM MUST NOT BE MUTATED, and must not be handed to anything
// that might mutate it. It is shared with every other caller and cached across
// autosave cycles. If you need a template you can change, call
// LoadRoomTemplate, which still reads a fresh copy from disk every time.
//
// Returns nil when the template cannot be loaded, matching LoadRoomTemplate.
func templateForCompare(roomId int) *Room {

	templateCacheMu.RLock()
	cached, ok := templateCache[roomId]
	templateCacheMu.RUnlock()
	if ok {
		return cached
	}

	// Load outside the write lock: this is disk I/O and a YAML parse, and
	// holding the lock across it would serialise every room's first save.
	loaded := LoadRoomTemplate(roomId)
	if loaded == nil {
		// Deliberately NOT cached. A nil here means the template is missing or
		// unreadable, which is a condition that can be fixed while the server
		// runs (a builder restoring a file); caching it would make the failure
		// stick until restart.
		return nil
	}

	templateCacheMu.Lock()
	// Another goroutine may have won the race. Prefer the entry already
	// published so every caller shares one object.
	if existing, raced := templateCache[roomId]; raced {
		templateCacheMu.Unlock()
		return existing
	}
	templateCache[roomId] = loaded
	templateCacheMu.Unlock()

	return loaded
}

// InvalidateTemplateCache drops one room's cached template.
//
// Call this from EVERY path that writes or removes a room's template file.
// Unlike a mutation-tracking dirty set, this surface is small and closed: the
// template file is authored content, and only the builder and the zone tools
// write it. Missing one shows the old title or description until restart --
// annoying, and nothing like the silent permanent data loss a missed dirty-mark
// would cause.
func InvalidateTemplateCache(roomId int) {
	templateCacheMu.Lock()
	delete(templateCache, roomId)
	templateCacheMu.Unlock()
}

// PurgeTemplateCache drops every cached template. For bulk operations that
// rewrite many files at once (a zone rename or delete), where enumerating the
// affected ids is more error-prone than starting over.
func PurgeTemplateCache() {
	templateCacheMu.Lock()
	templateCache = map[int]*Room{}
	templateCacheMu.Unlock()
}

// templateCacheSize reports how many templates are cached. Test-only.
func templateCacheSize() int {
	templateCacheMu.RLock()
	defer templateCacheMu.RUnlock()
	return len(templateCache)
}
