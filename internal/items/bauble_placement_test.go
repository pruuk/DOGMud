package items

import (
	"strings"
	"testing"
	"time"
)

func TestBaublePlacement(t *testing.T) {
	now := time.Unix(6_000_000, 0)
	b := Item{ItemId: BaubleItemId, Bauble: `B0000009`}

	if _, untaken := b.BaubleUntakenFor(now); untaken || b.BaubleSpotSuffix() != `` || b.BaubleBelongsTo(12) {
		t.Fatal("a bauble never left anywhere has no placement")
	}

	b.LeaveBaubleAt(`on the bookshelf`, 12, now)
	if !b.BaubleBelongsTo(12) || b.BaubleBelongsTo(13) || b.BaubleBelongsTo(0) {
		t.Fatal("it belongs to room 12's household only")
	}
	if age, untaken := b.BaubleUntakenFor(now.Add(3 * time.Hour)); !untaken || age != 3*time.Hour {
		t.Fatalf("untaken for %v %v", age, untaken)
	}
	if !strings.Contains(b.BaubleSpotSuffix(), `(on the bookshelf)`) {
		t.Fatalf("suffix %q", b.BaubleSpotSuffix())
	}

	b.ClearBaublePlacement()
	if b.BaubleSpot != `` || b.BaubleHousehold != 0 || b.BaubleLeftAt != 0 {
		t.Fatalf("carried, it lies nowhere: %+v", b)
	}

	plain := Item{ItemId: 1}
	plain.LeaveBaubleAt(`on the table`, 12, now)
	if plain.BaubleBelongsTo(12) || plain.BaubleSpotSuffix() != `` {
		t.Fatal("only baubles have a placement")
	}
	if _, untaken := plain.BaubleUntakenFor(now); untaken {
		t.Fatal("an ordinary item never vanishes")
	}
}

// A loaded item answers to its keyword and every word of its name; the bauble
// carrier does not count.
func TestAuthoredKeyword(t *testing.T) {
	restore := SeedItemsForTest(map[int]*ItemSpec{
		10:           {ItemId: 10, Name: `Hooded Lantern`, NameSimple: `lamp`},
		11:           {ItemId: 11, Name: `Miller's Daughter's`, NameSimple: `token`},
		BaubleItemId: {ItemId: BaubleItemId, Name: `Curious Trinket`, NameSimple: `trinket`},
	})
	defer restore()
	for w, want := range map[string]bool{`lamp`: true, `lantern`: true, `LANTERN`: true, `daughter`: true, `miller`: true, `hooded`: true, `trinket`: false, `whistle`: false} {
		if AuthoredKeyword(w) != want {
			t.Errorf("AuthoredKeyword(%q) = %v", w, !want)
		}
	}
}

// A loaded item's whole name, compared after NFKC, lower case and collapsed
// spaces, so a model cannot pass a real item's name off by case, spacing or
// fullwidth letters (spec S3). The bauble carrier does not count.
func TestAuthoredName(t *testing.T) {
	restore := SeedItemsForTest(map[int]*ItemSpec{
		10:           {ItemId: 10, Name: `Hooded Lantern`, NameSimple: `lamp`},
		12:           {ItemId: 12, Name: `A Beeswax Votive Candle`, NameSimple: `candle`},
		13:           {ItemId: 13, Name: `Amber`, NameSimple: `amber`},
		14:           {ItemId: 14, Name: `Iron Dagger`, NameSimple: `dagger`},
		BaubleItemId: {ItemId: BaubleItemId, Name: `Curious Trinket`, NameSimple: `trinket`},
	})
	defer restore()
	for name, want := range map[string]bool{
		`Hooded Lantern`:                   true,
		`hooded lantern`:                   true,
		"  Hooded\U000000A0\t Lantern ":    true,
		"\U0000FF28ooded \U0000FF2Cantern": true, // fullwidth H and L
		`Hooded Lanterns`:                  false,
		`Lantern`:                          false,
		`Curious Trinket`:                  false,
		// Near-misses of a loaded item's authored name still count as that
		// item (review of the exact-match gap): a leading article, trailing
		// punctuation, or a hyphen standing in for a space must not let a
		// model dodge the collision check by rephrasing.
		`The Hooded Lantern`:    true, // leading article stripped
		`Hooded Lantern.`:       true, // trailing punctuation stripped
		`Beeswax Votive Candle`: true, // authored name itself carries the article
		// An unrelated name that merely starts with the letter sequence "A "
		// is not the article-stripped form of anything: only an exact
		// normalised match counts.
		`Amber Bead`: false,
		// Double-space collapsing still works after the extended normalisation.
		`Iron  Dagger`: true,
	} {
		if AuthoredName(name) != want {
			t.Errorf("AuthoredName(%q) = %v, want %v", name, !want, want)
		}
	}
}

// A real item beats a bauble that the name matches in full too, wherever
// each is in the list; an explicit N. above 1 keeps list order.
func TestRealItemsBeatBaublesOnFullMatches(t *testing.T) {
	restore := SeedItemsForTest(map[int]*ItemSpec{
		10:           {ItemId: 10, Name: `Brass Lantern`, NameSimple: `lantern`},
		BaubleItemId: {ItemId: BaubleItemId, Name: `Curious Trinket`, NameSimple: `trinket`},
	})
	defer restore()
	SetBaubleResolver(func(id string) (BaubleView, bool) {
		return BaubleView{Name: `Brass Lantern`, NameSimple: `lantern`, Description: `x`}, true
	})
	defer SetBaubleResolver(nil)
	b := New(BaubleItemId)
	b.Bauble = `B0000001`
	real := New(10)
	if _, full := b.NameMatch(`lantern`, false); !full {
		t.Fatal("fixture: the bauble matches in full")
	}
	if _, m := FindMatchIn(`lantern`, b, real); m.IsBauble() || m.ItemId != 10 {
		t.Fatalf("the real item wins: %+v", m)
	}
	if _, m := FindMatchIn(`2.lantern`, b, real); m.ItemId != 10 {
		t.Fatalf("N. is list order: %+v", m)
	}
}

// The keyword check is read from goroutines off the mud lock (a bauble
// being named) while the items map can be written (an item saved or
// created): it must read a snapshot, never the live map. Run with -race.
func TestAuthoredKeywordIsSafeWhileItemsAreWritten(t *testing.T) {
	restore := SeedItemsForTest(map[int]*ItemSpec{10: {ItemId: 10, Name: `Hooded Lantern`, NameSimple: `lamp`}})
	defer restore()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 2000 {
			_ = AuthoredKeyword(`lantern`)
			_ = AuthoredName(`Hooded Lantern`)
		}
	}()
	for i := range 200 {
		RegisterTestItemSpec(&ItemSpec{ItemId: 1000 + i, Name: `Test Candle`, NameSimple: `candle`})
	}
	<-done
	if !AuthoredKeyword(`candle`) || !AuthoredKeyword(`lantern`) || !AuthoredName(`test candle`) {
		t.Fatal("the snapshot follows every write")
	}
}
