package baubles

import (
	"sync"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The keep period is at least a week: sales stats read the last seven days.
func TestKeepDurationIsAtLeastAWeek(t *testing.T) {
	setBaubleConfig(t, func(b *configs.Balance) { b.BaubleCatalogKeepDays = 2; b.Validate() })
	if KeepDuration() != 7*24*time.Hour {
		t.Fatalf("keep %v", KeepDuration())
	}
}

// Reads never wait on a disk write: while a shard write is held open,
// Get (and so GetSpec, through the resolver) answers at once.
func TestReadsDoNotWaitForAWrite(t *testing.T) {
	SetDirForTest(t.TempDir())
	t.Cleanup(func() { items.SetBaubleResolver(nil) })
	r, err := Create(Record{Name: `Bone Dice`, NameSimple: `dice`, Tier: TierCheap, Value: 3, Status: StatusReady})
	if err != nil {
		t.Fatal(err)
	}

	entered, release := make(chan struct{}), make(chan struct{})
	orig := shardWriter
	shardWriter = func(dir string, shard int, recs []*Record) error {
		close(entered)
		<-release
		return orig(dir, shard, recs)
	}
	t.Cleanup(func() { shardWriter = orig })

	done := make(chan struct{})
	go func() {
		Update(r.Id, func(r *Record) { r.Value = 4 })
		close(done)
	}()
	<-entered

	got := make(chan Record, 1)
	go func() {
		rec, _ := Get(r.Id)
		got <- rec
	}()
	select {
	case rec := <-got:
		if rec.Value != 4 {
			t.Fatalf("the change is visible before its write lands: %+v", rec)
		}
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("Get waited for the disk write")
	}
	close(release)
	<-done
}

// Writes stay in order: the last write of a shard carries the latest
// records, so a reload finds every change.
func TestConcurrentUpdatesAllReachDisk(t *testing.T) {
	dir := t.TempDir()
	SetDirForTest(dir)
	t.Cleanup(func() { items.SetBaubleResolver(nil) })
	ids := []string{}
	for range 8 {
		r, err := Create(Record{Name: `Glass Marble`, NameSimple: `marble`, Tier: TierCheap, Value: 1, Status: StatusReady})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, r.Id)
	}
	done := make(chan struct{})
	for _, id := range ids {
		go func(id string) {
			for v := 2; v <= 6; v++ {
				Update(id, func(r *Record) { r.Value = v })
			}
			done <- struct{}{}
		}(id)
	}
	for range ids {
		<-done
	}
	if err := loadFrom(dir); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if r, ok := Get(id); !ok || r.Value != 6 {
			t.Fatalf("%s after reload: %+v", id, r)
		}
	}
}

// ReturnCredits counts only credits from the given round on.
func TestReturnCreditsCountFromARound(t *testing.T) {
	SetDirForTest(t.TempDir())
	t.Cleanup(func() { items.SetBaubleResolver(nil) })
	orig := util.GetRoundCount()
	t.Cleanup(func() { util.SetRoundCountForTest(orig) })
	t0 := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	for _, round := range []uint64{100, 200, 300} {
		r, err := Create(Record{Name: `Bone Dice`, NameSimple: `dice`, Tier: TierCheap, Value: 3, Status: StatusReady})
		if err != nil {
			t.Fatal(err)
		}
		MarkStolen(r.Id, Theft{ByUserId: 7, FromMob: 2}, t0)
		util.SetRoundCountForTest(round)
		MarkReturned(r.Id, 7, []string{`town`}, t0)
	}
	for since, want := range map[uint64]int{0: 3, 150: 2, 300: 1, 301: 0} {
		if n := ReturnCredits(7, `town`, since); n != want {
			t.Errorf("since %d: %d, want %d", since, n, want)
		}
	}
}

// Disk writes land in the order of the changes: a write taken after a
// second change cannot be overtaken by an earlier, slower write carrying
// the older record.
func TestAnEarlierSlowWriteDoesNotOverwriteALaterOne(t *testing.T) {
	dir := t.TempDir()
	SetDirForTest(dir)
	t.Cleanup(func() { items.SetBaubleResolver(nil) })
	r, err := Create(Record{Name: `Bone Dice`, NameSimple: `dice`, Tier: TierCheap, Value: 3, Status: StatusReady})
	if err != nil {
		t.Fatal(err)
	}

	entered, release := make(chan struct{}), make(chan struct{})
	orig := shardWriter
	var first sync.Once
	shardWriter = func(dir string, shard int, recs []*Record) error {
		held := false
		first.Do(func() { held = true })
		if held {
			close(entered)
			<-release // the first write is slow
		}
		return orig(dir, shard, recs)
	}
	t.Cleanup(func() { shardWriter = orig })

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); Update(r.Id, func(r *Record) { r.Value = 4 }) }()
	<-entered
	go func() { defer wg.Done(); Update(r.Id, func(r *Record) { r.Value = 5 }) }()
	time.Sleep(50 * time.Millisecond) // the second change is made; its write waits or races
	close(release)
	wg.Wait()

	if err := loadFrom(dir); err != nil {
		t.Fatal(err)
	}
	if got, _ := Get(r.Id); got.Value != 5 {
		t.Fatalf("the last change is what is on disk: value %d", got.Value)
	}
}
