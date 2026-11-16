package baubles

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/util"
)

func TestMain(m *testing.M) {
	mudlog.SetupLogger(nil, "", "", false)
	// Baubles are off unless the config turns them on (Balance.
	// BaublesEnabled); these tests are about how they work when on.
	if err := configs.AddOverlayOverrides(map[string]any{`Balance.BaublesEnabled`: true}); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// withCatalog points the catalog at a fresh temp dir and seeds the carrier.
func withCatalog(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	restore := items.SeedItemsForTest(map[int]*items.ItemSpec{
		items.BaubleItemId: {
			ItemId:     items.BaubleItemId,
			Name:       `Curious Trinket`,
			NameSimple: `trinket`,
			Type:       items.Object,
			Subtype:    items.Mundane,
			Weight:     0.2,
			Value:      1,
		},
	})
	SetDirForTest(dir)
	// An empty corpus, as a world with no corpus files boots with: its
	// writers refuse an unloaded corpus (ErrNoCorpus). withCorpus replaces it.
	LoadCorpusFrom(filepath.Join(dir, `no-seed`, seedFileName), filepath.Join(dir, `no-overlay`, overlayFileName))
	t.Cleanup(func() {
		restore()
		items.SetBaubleResolver(nil)
		ClearCorpusForTest()
	})
	return dir
}

func first(n int) int { return 0 }

func TestCreateAssignsSequentialIdsAndPersists(t *testing.T) {
	dir := withCatalog(t)

	a, err := Create(Record{Name: `A`, Tier: TierCheap, Value: 2})
	if err != nil || a.Id != `B0000001` || a.FoundAt.IsZero() {
		t.Fatalf("first: %+v %v", a, err)
	}
	b, _ := Create(Record{Name: `B`, Tier: TierRare, Value: 90})
	if b.Id != `B0000002` {
		t.Fatalf("second id: %s", b.Id)
	}
	if _, err := os.Stat(filepath.Join(dir, `catalog-0000.yaml`)); err != nil {
		t.Fatal("the shard is written on create")
	}
	if _, err := os.Stat(filepath.Join(dir, metaFileName)); err != nil {
		t.Fatal("the meta file is written on create")
	}

	// Reload from disk: same records, and the next id continues.
	if err := loadFrom(dir); err != nil {
		t.Fatal(err)
	}
	if got, ok := Get(`B0000002`); !ok || got.Name != `B` || got.Value != 90 || got.Tier != TierRare {
		t.Fatalf("reload: %+v", got)
	}
	c, _ := Create(Record{Name: `C`})
	if c.Id != `B0000003` {
		t.Fatalf("id after reload: %s", c.Id)
	}
}

func TestIdsAreNeverReusedAcrossShards(t *testing.T) {
	dir := withCatalog(t)
	cat.mu.Lock()
	cat.nextSeq = ShardSize // B0000500, the last of shard 0 (ids start at 1)
	cat.mu.Unlock()

	a, _ := Create(Record{Name: `last of shard 0`})
	b, _ := Create(Record{Name: `first of shard 1`})
	if shardOf(mustSeq(t, a.Id)) != 0 || shardOf(mustSeq(t, b.Id)) != 1 {
		t.Fatalf("shards: %s %s", a.Id, b.Id)
	}
	if _, err := os.Stat(shardPath(dir, 1)); err != nil {
		t.Fatal("shard 1 written")
	}
}

// A catalog written with the old layout (shards from id 0, so B0000500 in
// shard 1) loads with every record's copy from its own shard winning, and
// is rewritten so each record is in its own shard only.
func TestOldShardLayoutIsMovedIntoPlace(t *testing.T) {
	dir := withCatalog(t)
	fresh := &Record{Id: `B0000500`, Name: `Sold Doll`, Status: StatusSold}
	stale := &Record{Id: `B0000500`, Name: `Doll`, Status: StatusReady}
	next := &Record{Id: `B0000501`, Name: `Tin Horse`, Status: StatusReady}
	if err := writeShard(dir, 0, []*Record{fresh}); err != nil {
		t.Fatal(err)
	}
	if err := writeShard(dir, 1, []*Record{stale, next}); err != nil { // read last
		t.Fatal(err)
	}
	if err := loadFrom(dir); err != nil {
		t.Fatal(err)
	}
	if got, _ := Get(`B0000500`); got.Status != StatusSold {
		t.Fatalf("the copy in its own shard wins: %+v", got)
	}
	res, _ := loadDir(dir)
	if len(res.rewrite) != 0 {
		t.Fatalf("rewritten: nothing is out of place any more: %v", res.rewrite)
	}
	if got := res.records[`B0000500`]; got == nil || got.Status != StatusSold || res.records[`B0000501`] == nil {
		t.Fatalf("and nothing was lost: %+v", res.records)
	}
}

func mustSeq(t *testing.T, id string) uint64 {
	t.Helper()
	s, ok := seqOf(id)
	if !ok {
		t.Fatalf("bad id %q", id)
	}
	return s
}

func TestCorruptShardIsQuarantinedAndItsIdsSkipped(t *testing.T) {
	dir := withCatalog(t)
	if _, err := Create(Record{Name: `kept`}); err != nil {
		t.Fatal(err)
	}
	// A broken shard 3, and no meta file to say how far ids have gone.
	if err := os.WriteFile(shardPath(dir, 3), []byte("records: [unclosed"), 0644); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(dir, metaFileName))

	if err := loadFrom(dir); err != nil {
		t.Fatal(err)
	}
	if _, ok := Get(`B0000001`); !ok {
		t.Fatal("good shards still load")
	}
	if _, err := os.Stat(shardPath(dir, 3)); !os.IsNotExist(err) {
		t.Fatal("the corrupt shard is moved aside")
	}
	entries, _ := os.ReadDir(dir)
	aside := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), `catalog-0003.yaml.corrupt-`) {
			aside = true
		}
	}
	if !aside {
		t.Fatal("the corrupt shard is kept for inspection")
	}
	// Reboot before any new find: the bump survives, in the meta file.
	if err := loadFrom(dir); err != nil {
		t.Fatal(err)
	}
	next, _ := Create(Record{Name: `after`})
	if mustSeq(t, next.Id) <= 4*ShardSize { // shard 3 is B0001501 to B0002000
		t.Fatalf("ids of a lost shard must never be handed out again: %s", next.Id)
	}
}

func TestUpdateChangesAndPersists(t *testing.T) {
	dir := withCatalog(t)
	r, _ := Create(Record{Name: `Old`, Value: 3})
	got, ok := Update(r.Id, func(x *Record) {
		x.Name = `New`
		x.Id = `B9999999` // ignored
	})
	if !ok || got.Name != `New` || got.Id != r.Id {
		t.Fatalf("update: %+v", got)
	}
	if _, ok := Update(`B0000404`, func(*Record) {}); ok {
		t.Fatal("no such record")
	}
	_ = loadFrom(dir)
	if again, _ := Get(r.Id); again.Name != `New` {
		t.Fatal("update is on disk")
	}
}

func TestCreateWithoutCatalogFails(t *testing.T) {
	cat.mu.Lock()
	saved := cat.dir
	cat.dir = ``
	cat.mu.Unlock()
	defer func() {
		cat.mu.Lock()
		cat.dir = saved
		cat.mu.Unlock()
	}()
	if _, err := Create(Record{}); err != ErrNoCatalog {
		t.Fatalf("got %v", err)
	}
}

func TestRecordViewByStatus(t *testing.T) {
	r := Record{Name: `Painted Horse`, NameSimple: `horse`, Description: `d`, Value: 4, WeightLbs: 0.6, Status: StatusReady}
	if v := r.View(); v.Name != `Painted Horse` || v.DisplayName != `` || v.WeightLbs != 0.6 {
		t.Fatalf("ready: %+v", v)
	}
	r.Status = StatusRetired
	if v := r.View(); v.Name != retiredName || v.NameSimple != `trinket` || v.Value != 4 {
		t.Fatalf("retired keeps value, hides text: %+v", v)
	}
	for _, st := range []Status{StatusReady, StatusFallback, StatusSold, StatusRetired} {
		r.Status = st
		if strings.Contains(strings.ToLower(r.View().Name+r.View().DisplayName+r.View().Description), `unexamined`) {
			t.Fatal("no bauble is ever shown as unexamined")
		}
	}
}
func TestMintMakesAUniqueCatalogBackedItem(t *testing.T) {
	withCatalog(t)
	place := NewPlace(4033, `ashwick`, ``, `forest`)
	if place.Region != `ashwick` {
		t.Fatal("region falls back to the zone")
	}

	a, recA, err := Mint(MintOpts{Source: SourceAdmin, Place: place, FinderUserId: 7, Tier: TierAverage, Randn: first})
	if err != nil {
		t.Fatal(err)
	}
	b, _, _ := Mint(MintOpts{Place: place, Tier: TierRare})

	if a.ItemId != items.BaubleItemId || a.Bauble != recA.Id || a.Bauble == b.Bauble {
		t.Fatalf("items: %+v %+v", a, b)
	}
	if items.SameStack(a, b) {
		t.Fatal("two minted baubles never stack")
	}
	if recA.Status != StatusFallback || recA.Generator != GeneratorLocal || recA.Source != SourceAdmin {
		t.Fatalf("record: %+v", recA)
	}
	if recA.Zone != `ashwick` || recA.RoomId != 4033 || recA.Biome != `forest` || recA.FoundByUserId != 7 || recA.Stolen {
		t.Fatalf("provenance: %+v", recA)
	}
	if !TierAverage.Range().Contains(recA.Value) || recA.WeightLbs <= 0 {
		t.Fatalf("value %d weight %v", recA.Value, recA.WeightLbs)
	}
	// The item shows the catalog's name, value and weight.
	if a.Name() != recA.Name || a.GetSpec().Value != recA.Value || a.GetSpec().Weight != recA.WeightLbs {
		t.Fatalf("overlay: %q %d %v", a.Name(), a.GetSpec().Value, a.GetSpec().Weight)
	}
}

func TestMintNeedsTheCarrier(t *testing.T) {
	withCatalog(t)
	restore := items.SeedItemsForTest(map[int]*items.ItemSpec{})
	defer restore()
	if _, _, err := Mint(MintOpts{}); err != ErrNoCarrier {
		t.Fatalf("got %v", err)
	}
	if Count() != 0 {
		t.Fatal("no record without an item")
	}
}

func TestGenericTrinket(t *testing.T) {
	r := GenericTrinket(TierAverage, nil)
	if r.Name != `Trinket` || r.NameSimple != `trinket` || r.Description != genericDescriptions[0] {
		t.Fatalf("generic: %+v", r)
	}
	if r.WeightLbs != 0.1 || r.Value != TierAverage.RollValue(nil) {
		t.Fatalf("nil randn: lightest weight and the tier midpoint, got %v %d", r.WeightLbs, r.Value)
	}
	for range 200 {
		g := GenericTrinket(TierRare, util.Rand)
		if g.Name != `Trinket` || !TierRare.Range().Contains(g.Value) || g.WeightLbs < 0.1 || g.WeightLbs > 0.8 {
			t.Fatalf("random generic out of bounds: %+v", g)
		}
		if _, err := CleanReply(g); err != nil {
			t.Fatalf("a generic trinket must pass validation: %v", err)
		}
	}
}

func TestMarkSold(t *testing.T) {
	withCatalog(t)
	r, _ := Create(Record{Name: `Cup`, Tier: TierAverage, Value: 12, Status: StatusFallback})

	start := time.Now().UTC().Add(-time.Second)
	if !MarkSold(r.Id, 6, 42) {
		t.Fatal("mark sold")
	}
	if MarkSold(`B0000404`, 1, 42) {
		t.Fatal("no such record")
	}
	got, _ := Get(r.Id)
	if got.Status != StatusSold || got.SoldValue != 6 || got.SoldAt.IsZero() {
		t.Fatalf("sold: %+v", got)
	}
	if n, gold := SalesSince(start); n != 1 || gold != 6 {
		t.Fatalf("sales: %d %d", n, gold)
	}
	if n, _ := SalesSince(time.Now().UTC().Add(time.Hour)); n != 0 {
		t.Fatal("nothing sold in the future")
	}
}

// A record whose write fails is not created, and Mint hands nothing out:
// no item can point at a record a crash would lose.
func TestCreateIsDurableOrNothing(t *testing.T) {
	dir := withCatalog(t) // the carrier is seeded, so Mint can only fail at the write
	notADir := t.TempDir() + "/catalog"
	if err := os.WriteFile(notADir, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	SetDirForTest(notADir) // a file where the catalog directory should be: every write fails, even as root
	if _, err := Create(Record{Name: "Doll", Tier: TierCheap}); err == nil {
		t.Fatal("the write failed, so Create fails")
	}
	if Count() != 0 {
		t.Fatalf("and holds nothing: %d", Count())
	}
	if itm, _, err := Mint(MintOpts{Tier: TierCheap}); err == nil || errors.Is(err, ErrNoCarrier) || itm.Bauble != `` {
		t.Fatalf("Mint hands out nothing, for the write: %+v %v", itm, err)
	}
	SetDirForTest(dir)
	rec, err := Create(Record{Name: "Doll", Tier: TierCheap})
	if err != nil || rec.Id == `` {
		t.Fatalf("a good write creates it: %v", err)
	}
}
