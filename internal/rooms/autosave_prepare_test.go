package rooms

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/savequeue"
	"gopkg.in/yaml.v2"
)

// ---------------------------------------------------------------------------
// Chunk 3.6b-1 — SaveRoomInstance split into prepare (reads live state) and
// commit (touches only bytes).
//
// The migration's whole claim is "behaviour is unchanged, the work just happens
// at two different times". These tests are what makes that claim checkable.
// ---------------------------------------------------------------------------

// setupPrepareRoom seeds one template room on disk and returns its id plus a
// cleanup func. Mirrors benchSetupOneRoom, but for *testing.T.
func setupPrepareRoom(t *testing.T, roomId int) (restore func()) {
	t.Helper()
	tempDir := t.TempDir()

	prev := configs.GetFilePathsConfig()
	if err := configs.AddOverlayOverrides(map[string]any{
		"FilePaths.DataFiles":        tempDir,
		"FilePaths.CarefulSaveFiles": true,
	}); err != nil {
		t.Fatal(err)
	}

	tpl := &Room{RoomId: roomId, Zone: "preparezone", Title: "Prepare Room", Description: "A room for the prepare/commit split."}
	data, err := yaml.Marshal(tpl)
	if err != nil {
		t.Fatal(err)
	}
	templatePath := filepath.Join(tempDir, "rooms", "preparezone", fmt.Sprintf("%d.yaml", roomId))
	if err := os.MkdirAll(filepath.Dir(templatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(templatePath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	roomManager.roomIdToFileCache[roomId] = fmt.Sprintf("preparezone/%d.yaml", roomId)
	if err := os.MkdirAll(filepath.Join(tempDir, "rooms.instances", "preparezone"), 0o755); err != nil {
		t.Fatal(err)
	}

	return func() {
		delete(roomManager.roomIdToFileCache, roomId)
		autosaveQueue = savequeue.New()
		_ = configs.AddOverlayOverrides(map[string]any{
			"FilePaths.DataFiles":        prev.DataFiles.String(),
			"FilePaths.CarefulSaveFiles": bool(prev.CarefulSaveFiles),
		})
	}
}

func TestPrepareInstanceWrite_ProducesTheBytesSaveWouldHaveWritten(t *testing.T) {
	const roomId = 910001
	restore := setupPrepareRoom(t, roomId)
	defer restore()

	r := LoadRoomInstance(roomId)
	if r == nil {
		t.Fatal("room load returned nil")
	}
	r.Gold = 250

	p, err := PrepareInstanceWrite(*r)
	if err != nil {
		t.Fatalf("PrepareInstanceWrite: %v", err)
	}
	if p.IsDelete() {
		t.Fatal("a dirty room prepared as a delete")
	}

	// Commit the prepared bytes, then compare against what the synchronous path
	// writes for the same room. Byte-identical is the bar: anything else means
	// the split changed what lands on disk.
	if err := savequeue.Commit(p); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	viaPrepare, err := os.ReadFile(p.Path)
	if err != nil {
		t.Fatalf("read prepared output: %v", err)
	}

	if err := os.Remove(p.Path); err != nil {
		t.Fatal(err)
	}
	if err := SaveRoomInstance(*r); err != nil {
		t.Fatalf("SaveRoomInstance: %v", err)
	}
	viaSave, err := os.ReadFile(p.Path)
	if err != nil {
		t.Fatalf("read synchronous output: %v", err)
	}

	if string(viaPrepare) != string(viaSave) {
		t.Errorf("prepare+commit differs from SaveRoomInstance.\nprepare: %q\nsave:    %q", viaPrepare, viaSave)
	}
	if len(viaSave) == 0 {
		t.Error("nothing was written")
	}
}

func TestPrepareInstanceWrite_CleanRoomPreparesADelete(t *testing.T) {
	const roomId = 910002
	restore := setupPrepareRoom(t, roomId)
	defer restore()

	r := LoadRoomInstance(roomId)
	if r == nil {
		t.Fatal("room load returned nil")
	}

	p, err := PrepareInstanceWrite(*r)
	if err != nil {
		t.Fatalf("PrepareInstanceWrite: %v", err)
	}

	// A room matching its template must prepare as a DELETE, not as an empty
	// write. A stale overlay left on disk is re-applied on the next room load
	// and resurrects state the room no longer has.
	if !p.IsDelete() {
		t.Errorf("clean room prepared a write of %q, want a delete", p.Data)
	}
}

func TestPrepareInstanceWrite_PayloadIsImmutableOncePrepared(t *testing.T) {
	const roomId = 910003
	restore := setupPrepareRoom(t, roomId)
	defer restore()

	r := LoadRoomInstance(roomId)
	if r == nil {
		t.Fatal("room load returned nil")
	}
	r.Gold = 100

	p, err := PrepareInstanceWrite(*r)
	if err != nil {
		t.Fatalf("PrepareInstanceWrite: %v", err)
	}
	before := string(p.Data)

	// THE guard that makes deferral safe. A Room holds maps and slices, so a
	// deferred writer holding a Room would marshal state mutating underneath it.
	// Marshalling first is what removes the hazard, and this is where that gets
	// argued out if anyone ever changes PendingWrite to carry live state.
	r.Gold = 999999
	r.Title = "mutated after prepare"

	if string(p.Data) != before {
		t.Error("the prepared payload changed when the room was mutated afterwards")
	}

	if err := savequeue.Commit(p); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	written, err := os.ReadFile(p.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != before {
		t.Errorf("committed bytes reflect a post-prepare mutation: %q", written)
	}
}

func TestPrepareInstanceWrite_EphemeralRoomsAreRefused(t *testing.T) {
	r := Room{RoomId: ephemeralRoomIdMinimum + 1, Zone: "preparezone"}
	if _, err := PrepareInstanceWrite(r); err == nil {
		t.Error("expected an error preparing an ephemeral room, got nil")
	}
}

func TestPrepareInstanceWrite_MissingTemplateIsAnError(t *testing.T) {
	r := Room{RoomId: 919999, Zone: "nosuchzone"}
	if _, err := PrepareInstanceWrite(r); err == nil {
		t.Error("expected an error for a room with no template, got nil")
	}
}

// ---------------------------------------------------------------------------
// Guard G2 — cancellation
// ---------------------------------------------------------------------------

func TestSaveRoomInstance_CancelsAPendingWriteForTheSameRoom(t *testing.T) {
	const roomId = 910004
	restore := setupPrepareRoom(t, roomId)
	defer restore()

	r := LoadRoomInstance(roomId)
	if r == nil {
		t.Fatal("room load returned nil")
	}

	// Queue an OLD snapshot, then take a newer one synchronously. The pending
	// entry holds older bytes by definition, so letting it commit afterwards
	// would clobber the newer state.
	r.Gold = 111
	stale, err := PrepareInstanceWrite(*r)
	if err != nil {
		t.Fatal(err)
	}
	autosaveQueue.Supersede([]savequeue.PendingWrite{stale})

	r.Gold = 222
	if err := SaveRoomInstance(*r); err != nil {
		t.Fatalf("SaveRoomInstance: %v", err)
	}
	if autosaveQueue.Pending() != 0 {
		t.Errorf("pending %d after a synchronous save, want 0", autosaveQueue.Pending())
	}

	autosaveQueue.Drain(10)

	raw, err := os.ReadFile(stale.Path)
	if err != nil {
		t.Fatal(err)
	}
	overlay := map[string]interface{}{}
	if err := yaml.Unmarshal(raw, &overlay); err != nil {
		t.Fatal(err)
	}
	if got := overlay["gold"]; got != 222 {
		t.Errorf("gold on disk = %v, want 222 (a stale queued write clobbered a newer save)", got)
	}
}

func TestClearRoomCache_CancelsAPendingWrite(t *testing.T) {
	const roomId = 910005
	restore := setupPrepareRoom(t, roomId)
	defer restore()

	r := LoadRoomInstance(roomId)
	if r == nil {
		t.Fatal("room load returned nil")
	}
	r.Gold = 50
	if err := addRoomToMemory(r); err != nil {
		t.Fatal(err)
	}

	p, err := PrepareInstanceWrite(*r)
	if err != nil {
		t.Fatal(err)
	}
	autosaveQueue.Supersede([]savequeue.PendingWrite{p})

	// Once the room leaves the cache nothing owns that state any more, and the
	// pending bytes are a snapshot of a room the game has forgotten.
	if err := ClearRoomCache(roomId); err != nil {
		t.Fatalf("ClearRoomCache: %v", err)
	}
	if autosaveQueue.Pending() != 0 {
		t.Errorf("pending %d after ClearRoomCache, want 0", autosaveQueue.Pending())
	}
}

func TestDeleteRoomTemplate_CancelsAPendingWrite(t *testing.T) {
	const roomId = 910006
	restore := setupPrepareRoom(t, roomId)
	defer restore()

	r := LoadRoomInstance(roomId)
	if r == nil {
		t.Fatal("room load returned nil")
	}
	r.Gold = 75
	if err := addRoomToMemory(r); err != nil {
		t.Fatal(err)
	}

	p, err := PrepareInstanceWrite(*r)
	if err != nil {
		t.Fatal(err)
	}
	autosaveQueue.Supersede([]savequeue.PendingWrite{p})

	// This path deletes room data without going through SaveRoomInstance, so
	// nothing else would cancel the queued write. Left queued, the commit writes
	// an instance overlay for a room whose template no longer exists.
	if err := DeleteRoomTemplate(roomId); err != nil {
		t.Fatalf("DeleteRoomTemplate: %v", err)
	}
	if autosaveQueue.Pending() != 0 {
		t.Errorf("pending %d after DeleteRoomTemplate, want 0", autosaveQueue.Pending())
	}

	autosaveQueue.Drain(10)
	if _, err := os.Stat(p.Path); !os.IsNotExist(err) {
		t.Error("an orphan instance overlay was written for a deleted room")
	}
}
