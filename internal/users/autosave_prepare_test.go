package users

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/savequeue"
)

// ---------------------------------------------------------------------------
// Chunk 3.6b-1 — SaveUser split into prepare (marshals live character state)
// and commit (the durable write).
//
// Users were originally out of scope. 3.6a reported usersMs=0, but with zero
// players connected. Once chunk 2.8 made user saves durable they went from
// 0.696ms to 3.873ms per file, which at 100 players is 387ms — the LARGER half
// of the autosave pause. Users were cheap because they were not durable.
// ---------------------------------------------------------------------------

// withUserDataDir points the user save path at a temp dir and restores the
// shared autosave queue afterwards, so tests cannot leak state into each other.
func withUserDataDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	prev := configs.GetFilePathsConfig()
	if err := configs.AddOverlayOverrides(map[string]any{
		"FilePaths.DataFiles":        dir,
		"FilePaths.CarefulSaveFiles": true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "users"), 0o755); err != nil {
		t.Fatal(err)
	}

	prevQueue := autosaveQueue
	autosaveQueue = savequeue.New()

	t.Cleanup(func() {
		autosaveQueue = prevQueue
		_ = configs.AddOverlayOverrides(map[string]any{
			"FilePaths.DataFiles":        prev.DataFiles.String(),
			"FilePaths.CarefulSaveFiles": bool(prev.CarefulSaveFiles),
		})
	})
	return dir
}

// probeUser builds a real record. NewUserRecord matters here: a bare
// &UserRecord{} leaves Character nil, and yaml.Marshal of a nil pointer field
// is not what a live user save looks like.
func probeUser(id int, name string) *UserRecord {
	u := NewUserRecord(id, 0)
	u.Username = name
	u.Character.Name = name
	return u
}

func TestPrepareUserWrite_ProducesTheBytesSaveWouldHaveWritten(t *testing.T) {
	withUserDataDir(t)
	u := probeUser(4001, "prepareprobe")

	p, err := PrepareUserWrite(u)
	if err != nil {
		t.Fatalf("PrepareUserWrite: %v", err)
	}
	if p.IsDelete() {
		t.Fatal("a user prepared as a delete; autosave must never remove a user file")
	}
	if p.Kind != "user" || p.Id != u.UserId {
		t.Errorf("kind/id = %q/%d, want user/%d", p.Kind, p.Id, u.UserId)
	}

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
	if err := SaveUser(u); err != nil {
		t.Fatalf("SaveUser: %v", err)
	}
	viaSave, err := os.ReadFile(p.Path)
	if err != nil {
		t.Fatalf("read synchronous output: %v", err)
	}

	// Byte-identical is the bar. Anything else means the split changed what a
	// player's character file looks like on disk.
	if string(viaPrepare) != string(viaSave) {
		t.Errorf("prepare+commit differs from SaveUser.\nprepare: %q\nsave:    %q", viaPrepare, viaSave)
	}
}

func TestPrepareUserWrite_PayloadIsImmutableOncePrepared(t *testing.T) {
	withUserDataDir(t)
	u := probeUser(4002, "immutableprobe")
	u.Character.Gold = 100

	p, err := PrepareUserWrite(u)
	if err != nil {
		t.Fatalf("PrepareUserWrite: %v", err)
	}
	before := string(p.Data)

	// THE property the whole design rests on. A UserRecord holds maps and
	// slices; a deferred writer holding the record would marshal a character
	// the game is mutating underneath it. Marshalling first removes the hazard.
	u.Character.Gold = 999999
	u.Username = "mutated after prepare"

	if string(p.Data) != before {
		t.Error("the prepared payload changed when the character was mutated afterwards")
	}
}

func TestSaveUser_CancelsAPendingWriteForTheSameUser(t *testing.T) {
	withUserDataDir(t)
	u := probeUser(4003, "cancelprobe")

	// Queue an OLD snapshot, then save synchronously. A stale user write is
	// worse than a stale room write: it can resurrect spent gold or a consumed
	// item by rolling the character back to whenever autosave last looked.
	u.Character.Gold = 10
	stale, err := PrepareUserWrite(u)
	if err != nil {
		t.Fatal(err)
	}
	autosaveQueue.Supersede([]savequeue.PendingWrite{stale})

	u.Character.Gold = 20
	if err := SaveUser(u); err != nil {
		t.Fatalf("SaveUser: %v", err)
	}
	if autosaveQueue.Pending() != 0 {
		t.Fatalf("pending %d after a synchronous save, want 0", autosaveQueue.Pending())
	}

	autosaveQueue.Drain(10)

	reloaded, err := loadUserFromPath(stale.Path, true)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Character.Gold != 20 {
		t.Errorf("gold on disk = %d, want 20 (a stale queued write rolled the character back)",
			reloaded.Character.Gold)
	}
}

func TestPrepareAllUserWrites_CoversEveryActiveUser(t *testing.T) {
	withUserDataDir(t)

	userManager.mu.Lock()
	prevUsers := userManager.Users
	userManager.Users = map[int]*UserRecord{
		4101: probeUser(4101, "alpha"),
		4102: probeUser(4102, "beta"),
		4103: probeUser(4103, "gamma"),
	}
	userManager.mu.Unlock()
	t.Cleanup(func() {
		userManager.mu.Lock()
		userManager.Users = prevUsers
		userManager.mu.Unlock()
	})

	writes, err := PrepareAllUserWrites()
	if err != nil {
		t.Fatalf("PrepareAllUserWrites: %v", err)
	}
	if len(writes) != 3 {
		t.Fatalf("prepared %d writes, want 3", len(writes))
	}

	seen := map[int]bool{}
	for _, w := range writes {
		if w.Kind != "user" {
			t.Errorf("kind = %q, want user", w.Kind)
		}
		if len(w.Data) == 0 {
			t.Errorf("user %d prepared an empty payload", w.Id)
		}
		seen[w.Id] = true
	}
	for _, id := range []int{4101, 4102, 4103} {
		if !seen[id] {
			t.Errorf("user %d was not prepared", id)
		}
	}
}

func TestCancelPendingUserWrite_NilUserIsANoOp(t *testing.T) {
	withUserDataDir(t)
	// Defensive: the logout path can reach this with a record already cleared
	// from the manager, and a panic there would take the server down on a
	// disconnect.
	CancelPendingUserWrite(nil)
}

func TestSetAutosaveQueue_IgnoresNil(t *testing.T) {
	withUserDataDir(t)
	before := autosaveQueue

	// A nil here would make every later Cancel panic, and the failure would
	// surface far from the cause.
	SetAutosaveQueue(nil)

	if autosaveQueue != before {
		t.Error("SetAutosaveQueue(nil) replaced the live queue")
	}
}
