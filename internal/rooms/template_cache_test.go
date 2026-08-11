package rooms

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"gopkg.in/yaml.v2"
)

// ---------------------------------------------------------------------------
// Chunk 4.6 — cache the room template used by the autosave compare.
//
// Measured: PrepareInstanceWrite costs 0.1215ms/room, of which the template
// read alone is 0.0983ms (81%). The template is immutable authored content and
// was being re-read and re-parsed from disk for every room, every cycle.
// ---------------------------------------------------------------------------

// setupCacheRoom seeds a template on disk and clears the cache around the test.
func setupCacheRoom(t *testing.T, roomId int, title string) (dir string, restore func()) {
	t.Helper()
	dir = t.TempDir()

	prev := configs.GetFilePathsConfig()
	if err := configs.AddOverlayOverrides(map[string]any{
		"FilePaths.DataFiles":        dir,
		"FilePaths.CarefulSaveFiles": true,
	}); err != nil {
		t.Fatal(err)
	}

	writeCacheTemplate(t, dir, roomId, title)
	roomManager.roomIdToFileCache[roomId] = fmt.Sprintf("cachezone/%d.yaml", roomId)
	if err := os.MkdirAll(filepath.Join(dir, "rooms.instances", "cachezone"), 0o755); err != nil {
		t.Fatal(err)
	}

	PurgeTemplateCache()

	return dir, func() {
		PurgeTemplateCache()
		delete(roomManager.roomIdToFileCache, roomId)
		_ = configs.AddOverlayOverrides(map[string]any{
			"FilePaths.DataFiles":        prev.DataFiles.String(),
			"FilePaths.CarefulSaveFiles": bool(prev.CarefulSaveFiles),
		})
	}
}

func writeCacheTemplate(t *testing.T, dir string, roomId int, title string) {
	t.Helper()
	writeCacheTemplateWithGold(t, dir, roomId, title, 0)
}

// writeCacheTemplateWithGold sets Gold, which unlike Title is NOT
// instance:"skip" and therefore actually participates in the instance diff.
// Any test of template-cache invalidation must use a diffed field.
func writeCacheTemplateWithGold(t *testing.T, dir string, roomId int, title string, gold int) {
	t.Helper()
	tpl := &Room{RoomId: roomId, Zone: "cachezone", Title: title, Description: "Template cache test room.", Gold: gold}
	data, err := yaml.Marshal(tpl)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "rooms", "cachezone", fmt.Sprintf("%d.yaml", roomId))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTemplateForCompare_ReturnsTheSameObjectOnEveryCall(t *testing.T) {
	const roomId = 940001
	_, restore := setupCacheRoom(t, roomId, "Cached Room")
	defer restore()

	first := templateForCompare(roomId)
	if first == nil {
		t.Fatal("first load returned nil")
	}
	second := templateForCompare(roomId)

	// Identity, not equality: the point is that the second call did no disk I/O.
	if first != second {
		t.Error("templateForCompare re-read the template instead of caching it")
	}
	if templateCacheSize() != 1 {
		t.Errorf("cache holds %d entries, want 1", templateCacheSize())
	}
}

// THE regression guard for what caching could have broken.
//
// LoadRoomInstance calls LoadRoomTemplate TWICE and requires two INDEPENDENT
// objects: one becomes the live room, the other is a scratch copy the instance
// overlay is unmarshalled onto. If they were the same object, a partially
// applied overlay would write straight into the live room -- which is review
// finding 15, the template/runtime hybrid, arriving by a new route.
//
// This is why the cache is a separate accessor and NOT a cache inside
// LoadRoomTemplate.
func TestLoadRoomTemplate_StillReturnsIndependentObjects(t *testing.T) {
	const roomId = 940002
	_, restore := setupCacheRoom(t, roomId, "Independent Room")
	defer restore()

	a := LoadRoomTemplate(roomId)
	b := LoadRoomTemplate(roomId)
	if a == nil || b == nil {
		t.Fatal("template load returned nil")
	}
	if a == b {
		t.Fatal("LoadRoomTemplate handed out a shared pointer; LoadRoomInstance's scratch copy depends on it not doing that")
	}

	// Mutating one must not touch the other, including through shared maps.
	a.Title = "mutated"
	a.Gold = 999
	if b.Title == "mutated" || b.Gold == 999 {
		t.Error("the two templates share state")
	}
}

// The cached template must not be reachable as something a caller can mutate.
func TestTemplateForCompare_IsNotHandedOutByLoadRoomTemplate(t *testing.T) {
	const roomId = 940003
	_, restore := setupCacheRoom(t, roomId, "Isolation Room")
	defer restore()

	cached := templateForCompare(roomId)
	fresh := LoadRoomTemplate(roomId)

	if cached == fresh {
		t.Error("LoadRoomTemplate returned the shared cached object")
	}
}

func TestPrepareInstanceWrite_UsesTheCacheAndStillDiffsCorrectly(t *testing.T) {
	const roomId = 940004
	_, restore := setupCacheRoom(t, roomId, "Diff Room")
	defer restore()

	r := LoadRoomInstance(roomId)
	if r == nil {
		t.Fatal("room load returned nil")
	}

	// Clean room: matches its template, so it prepares a DELETE.
	p, err := PrepareInstanceWrite(*r)
	if err != nil {
		t.Fatalf("PrepareInstanceWrite: %v", err)
	}
	if !p.IsDelete() {
		t.Errorf("clean room prepared a write of %q, want a delete", p.Data)
	}

	// Dirty it: the diff must still see the difference through the cache.
	r.Gold = 42
	p, err = PrepareInstanceWrite(*r)
	if err != nil {
		t.Fatalf("PrepareInstanceWrite: %v", err)
	}
	if p.IsDelete() {
		t.Fatal("dirty room prepared a delete")
	}
	if want := "gold: 42\n"; string(p.Data) != want {
		t.Errorf("payload = %q, want %q", p.Data, want)
	}

	if templateCacheSize() != 1 {
		t.Errorf("cache holds %d entries, want 1", templateCacheSize())
	}
}

// If a stale template survived a builder edit, every field the builder changed
// would look like INSTANCE state to the diff and get baked into the room's
// overlay -- where it then shadows future template edits.
//
// THIS TEST MUST USE A DIFFED FIELD. An earlier version asserted on Title,
// which is instance:"skip": PrepareInstanceWrite skips those fields before
// comparing, so it returned a delete whether the cache was fresh or stale and
// proved nothing. Gold is not skip-tagged, so it genuinely exercises the
// comparison against the cached template.
func TestPrepareInstanceWrite_SeesATemplateEditAfterInvalidation(t *testing.T) {
	const roomId = 940005
	dir, restore := setupCacheRoom(t, roomId, "Before Edit")
	defer restore()

	r := LoadRoomInstance(roomId)
	if r == nil {
		t.Fatal("room load returned nil")
	}

	// The room carries gold the template does not, so it is genuinely dirty and
	// this warms the cache with a template whose Gold is 0.
	r.Gold = 500
	p, err := PrepareInstanceWrite(*r)
	if err != nil {
		t.Fatal(err)
	}
	if p.IsDelete() {
		t.Fatal("fixture wrong: a room with gold its template lacks should be dirty")
	}

	// A builder now sets that same gold ON THE TEMPLATE. The room matches it, so
	// the room has no instance state any more and its overlay should be removed.
	writeCacheTemplateWithGold(t, dir, roomId, "Before Edit", 500)
	InvalidateTemplateCache(roomId)

	p, err = PrepareInstanceWrite(*r)
	if err != nil {
		t.Fatalf("PrepareInstanceWrite: %v", err)
	}
	if !p.IsDelete() {
		t.Errorf("a template edit was baked into the instance overlay: %q", p.Data)
	}
}

// The negative case: WITHOUT invalidation the stale cache must produce the
// wrong answer. Without this, the test above could pass for the wrong reason
// and nobody would know.
func TestPrepareInstanceWrite_StaleTemplateWouldLeakWithoutInvalidation(t *testing.T) {
	const roomId = 940008
	dir, restore := setupCacheRoom(t, roomId, "Stale Probe")
	defer restore()

	r := LoadRoomInstance(roomId)
	if r == nil {
		t.Fatal("room load returned nil")
	}
	r.Gold = 500

	// Warm the cache with the OLD template (gold 0).
	if _, err := PrepareInstanceWrite(*r); err != nil {
		t.Fatal(err)
	}

	// Builder sets gold on the template, but nothing invalidates.
	writeCacheTemplateWithGold(t, dir, roomId, "Stale Probe", 500)

	p, err := PrepareInstanceWrite(*r)
	if err != nil {
		t.Fatal(err)
	}
	if p.IsDelete() {
		t.Fatal("the cache was invalidated by something; this test can no longer detect a stale one")
	}
	if want := "gold: 500\n"; string(p.Data) != want {
		t.Errorf("stale-cache payload = %q, want %q", p.Data, want)
	}
}

func TestInvalidateTemplateCache_ForcesAReread(t *testing.T) {
	const roomId = 940006
	_, restore := setupCacheRoom(t, roomId, "Reread Room")
	defer restore()

	first := templateForCompare(roomId)
	InvalidateTemplateCache(roomId)
	second := templateForCompare(roomId)

	if first == second {
		t.Error("invalidation did not force a fresh read")
	}
}

func TestPurgeTemplateCache_ClearsEverything(t *testing.T) {
	const roomId = 940007
	_, restore := setupCacheRoom(t, roomId, "Purge Room")
	defer restore()

	templateForCompare(roomId)
	if templateCacheSize() == 0 {
		t.Fatal("nothing was cached")
	}

	PurgeTemplateCache()
	if templateCacheSize() != 0 {
		t.Errorf("cache holds %d entries after purge, want 0", templateCacheSize())
	}
}

// A missing template must not be cached as nil: a builder can restore the file
// while the server runs, and caching the failure would make it stick until
// restart.
func TestTemplateForCompare_DoesNotCacheAFailure(t *testing.T) {
	PurgeTemplateCache()
	defer PurgeTemplateCache()

	if got := templateForCompare(949999); got != nil {
		t.Fatalf("expected nil for an unknown room, got %+v", got)
	}
	if templateCacheSize() != 0 {
		t.Errorf("a failed load was cached (%d entries)", templateCacheSize())
	}
}
