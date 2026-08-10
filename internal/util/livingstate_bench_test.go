package util

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// The fsync added to SafeSave for the living-state contract (chunk 2.1) buys
// durability at the cost of waiting on the storage device. That matters here
// because autosave writes many files while holding the world lock, and review
// finding 36 is precisely "unmeasured autosave world-lock pauses".
//
// These benchmarks size the cost so chunk 3.6a has a number to budget against
// instead of an argument. Run:
//
//	go test ./internal/util/ -bench 'Save' -benchtime 200x -run '^$'
//
// Compare BenchmarkSafeSave against BenchmarkUnsyncedWrite: the delta is what
// durability costs per file on this machine's disk. Expect the gap to be far
// wider on a spinning disk or a throttled cloud volume than on a local SSD, so
// re-measure on the droplet before drawing conclusions about production.

var benchPayload = make([]byte, 4096)

func BenchmarkSafeSave(b *testing.B) {
	dir := b.TempDir()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := filepath.Join(dir, fmt.Sprintf("state-%d.yaml", i))
		if err := SafeSave(path, benchPayload); err != nil {
			b.Fatal(err)
		}
	}
}

// Control leg: the pre-2026-08-10 behaviour, a plain write with no flush and
// no rename. Not a candidate implementation — it is here only to isolate how
// much of SafeSave's cost is the durability guarantee.
func BenchmarkUnsyncedWrite(b *testing.B) {
	dir := b.TempDir()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := filepath.Join(dir, fmt.Sprintf("state-%d.yaml", i))
		if err := os.WriteFile(path, benchPayload, 0o644); err != nil {
			b.Fatal(err)
		}
	}
}

// Overwriting the same path repeatedly is the shape autosave actually has:
// the file already exists, so the rename replaces rather than creates.
func BenchmarkSafeSaveOverwrite(b *testing.B) {
	path := filepath.Join(b.TempDir(), "state.yaml")
	if err := SafeSave(path, benchPayload); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := SafeSave(path, benchPayload); err != nil {
			b.Fatal(err)
		}
	}
}

// The honest control: the PREVIOUS SafeSave shape — temp write plus rename,
// no flush. Comparing this against BenchmarkSafeSave isolates the cost of the
// fsync alone, rather than attributing the rename's cost to it too.
func BenchmarkWriteAndRenameNoSync(b *testing.B) {
	dir := b.TempDir()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := filepath.Join(dir, fmt.Sprintf("state-%d.yaml", i))
		tmp := path + ".new"
		if err := os.WriteFile(tmp, benchPayload, 0o644); err != nil {
			b.Fatal(err)
		}
		if err := os.Rename(tmp, path); err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// User-save cost at realistic size, for roadmap chunk 3.6b.
//
// Real user files are ~48KB for an established character (2KB for a brand-new
// one). SaveUser used to hand-roll its own write-.new-then-rename with no
// fsync, which is atomic but NOT durable; chunk 2.8 routed it through
// util.Save. These benchmarks keep both shapes side by side so the price of
// that durability stays visible to 3.6b, which has to budget a user save
// inside the world lock.
//
//   go test ./internal/util/ -bench 'UserSave' -benchtime 200x -run '^$'
// ---------------------------------------------------------------------------

var userPayload48K = make([]byte, 48*1024)

// The OLD SaveUser shape, kept as the baseline: write a .new file, rename over
// the target, no flush. Atomic against a torn read, but a power loss can leave
// the renamed file empty or partial. No longer what SaveUser does.
func BenchmarkUserSave_CurrentNoFsync(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "user.yaml")
	tmp := path + ".new"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := os.WriteFile(tmp, userPayload48K, 0o600); err != nil {
			b.Fatal(err)
		}
		if err := os.Rename(tmp, path); err != nil {
			b.Fatal(err)
		}
	}
}

// What SaveUser costs now: routed through util.Save, with the fsync every other
// living-state store got in chunk 2.1.
func BenchmarkUserSave_DurableViaUtilSave(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "user.yaml")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := Save(path, userPayload48K); err != nil {
			b.Fatal(err)
		}
	}
}
