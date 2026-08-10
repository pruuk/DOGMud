package users

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// User-scale prepare benchmarks for roadmap chunk 3.6b-1.
//
// This is the HEADLESS instrument the design picked over the alternative of
// raising the local AI login cap from 20 to 100 and connecting harness agents.
// The question being answered is "what does the snapshot pass cost at N users",
// and driving it directly isolates exactly that: no network, no AI agents, no
// config change, nothing else scaling with player count to confuse the result.
//
// The harness run is still owed once, at the end of 3.6b, as a whole-server
// sanity check before the 100-player target is claimed met. It measures many
// things at once, which makes it the wrong tool for this specific number and
// the right tool for that one.
//
//	go test ./internal/users/ -bench PrepareAllUserWrites -benchtime 20x -run '^$'
//
// What matters is the number at 100: that is the user half of the world-lock
// pause, and since chunk 2.8 made user saves durable it is the LARGER half.
//
// Measured 2026-08-10, ~0.145ms/user and linear across 10 / 100 / 1000:
//
//	10 users      1.4 ms
//	100 users    15.7 ms     <- the user half of the lock-held pause
//	1000 users  142.0 ms
//
// Against the pre-split cost of 100 x 3.873ms = 387ms held in one block, that
// is a ~24x reduction in lock time; the writes themselves still happen, spread
// across subsequent ticks.
//
// Re-run before trusting a single reading. The first measurement of the 100
// case came out at 190ms and did not reproduce on either of two re-runs -- an
// outlier from the machine, not a step in the curve. A number that breaks
// monotonicity is noise until it repeats.

func benchUsers(b *testing.B, n int) {
	dir := b.TempDir()

	prev := configs.GetFilePathsConfig()
	if err := configs.AddOverlayOverrides(map[string]any{
		"FilePaths.DataFiles":        dir,
		"FilePaths.CarefulSaveFiles": true,
	}); err != nil {
		b.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "users"), 0o755); err != nil {
		b.Fatal(err)
	}

	userManager.mu.Lock()
	prevUsers := userManager.Users
	seeded := make(map[int]*UserRecord, n)
	for i := 0; i < n; i++ {
		id := 950000 + i
		u := NewUserRecord(id, 0)
		u.Username = fmt.Sprintf("benchuser%d", i)
		u.Character.Name = u.Username
		seeded[id] = u
	}
	userManager.Users = seeded
	userManager.mu.Unlock()

	defer func() {
		userManager.mu.Lock()
		userManager.Users = prevUsers
		userManager.mu.Unlock()
		_ = configs.AddOverlayOverrides(map[string]any{
			"FilePaths.DataFiles":        prev.DataFiles.String(),
			"FilePaths.CarefulSaveFiles": bool(prev.CarefulSaveFiles),
		})
	}()
	defer b.StopTimer()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := PrepareAllUserWrites(); err != nil {
			b.Fatal(err)
		}
	}
}

// A quiet evening.
func BenchmarkPrepareAllUserWrites_10(b *testing.B) { benchUsers(b, 10) }

// The target. This is the figure the 100-player claim rests on.
func BenchmarkPrepareAllUserWrites_100(b *testing.B) { benchUsers(b, 100) }

// A fork that found a bigger audience than we expect to.
func BenchmarkPrepareAllUserWrites_1000(b *testing.B) { benchUsers(b, 1000) }
