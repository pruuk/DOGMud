package savequeue

import (
	"fmt"
	"path/filepath"
	"testing"
)

// Acceptance benchmark for roadmap chunk 3.6b-1's second criterion: no single
// tick's autosave contribution may exceed the per-tick budget.
//
// A turn is TurnMs = 50ms. AutosaveWritesPerTick ships at 3, and 3.6a measured
// a durable write at 3.46ms, so the predicted cost is ~10.4ms per turn, about
// 20% of a turn.
//
// TREAT THE ABSOLUTE NUMBERS HERE WITH SUSPICION. Measured 2026-08-10 on a
// Windows dev box they came out at 11.8ms for ONE 64-byte write, 18.3ms for
// three, and 160ms for ten -- non-monotonic against payload size and
// sub-linear then super-linear against count. A durable write is an fsync, so
// this is dominated by the filesystem, the directory's current size, and
// whatever else the machine is doing, not by anything in this package. 3.6a
// measured the same operation at 3.46ms under a different file-creation
// pattern.
//
// So this benchmark is useful for RELATIVE comparisons on a quiet machine and
// for measuring the droplet, and it is not a source for "the per-tick budget is
// N ms" claims. The prod AutoSave log line is the real check, and the fact that
// the right value is environment-dependent is precisely why
// AutosaveWritesPerTick is a config knob rather than a constant.
//
//	go test ./internal/savequeue/ -bench Drain -benchtime 50x -run '^$'
//
// A room instance overlay is typically tens of bytes ("gold: 250"); a user file
// is ~48KB for an established character. Both are measured, because a tick can
// draw either from the same queue and the user case is the expensive one.

func benchDrain(b *testing.B, perTick, payloadBytes int) {
	dir := b.TempDir()
	payload := make([]byte, payloadBytes)

	// Build every queue up front. The obvious shape -- construct a queue inside
	// the loop with StopTimer/StartTimer around it -- produced nonsense: a
	// 64-byte write appeared to cost MORE than a 48KB one, because the
	// timer-toggle overhead per iteration swamped the write it was trying to
	// isolate. Setting up outside the measurement removes that entirely.
	queues := make([]*Queue, b.N)
	for i := range queues {
		writes := make([]PendingWrite, 0, perTick)
		for j := 0; j < perTick; j++ {
			writes = append(writes, PendingWrite{
				Kind:    "room",
				Id:      j,
				Path:    filepath.Join(dir, fmt.Sprintf("%d-%d.yaml", i, j)),
				Data:    payload,
				Careful: true,
			})
		}
		q := New()
		q.Supersede(writes)
		queues[i] = q
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		queues[i].Drain(perTick)
	}
}

// The shipped budget against a room-sized payload: this is what a normal tick
// costs while a cycle is draining.
func BenchmarkDrain_ShippedBudget_RoomPayload(b *testing.B) { benchDrain(b, 3, 64) }

// The shipped budget against user-sized payloads, the expensive case.
func BenchmarkDrain_ShippedBudget_UserPayload(b *testing.B) { benchDrain(b, 3, 48*1024) }

// One write, to isolate the per-write cost the budget is derived from.
func BenchmarkDrain_SingleWrite_RoomPayload(b *testing.B) { benchDrain(b, 1, 64) }
func BenchmarkDrain_SingleWrite_UserPayload(b *testing.B) { benchDrain(b, 1, 48*1024) }

// What raising the knob costs, for an operator deciding whether to drain a
// backlog faster.
func BenchmarkDrain_Budget10_RoomPayload(b *testing.B) { benchDrain(b, 10, 64) }
