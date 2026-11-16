package baubles

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/util"
)

func TestParseCorpusKey(t *testing.T) {
	cases := []struct {
		key    string
		prefix string
		tier   ValueTier
		ok     bool
	}{
		{`interior-cheap`, `interior`, TierCheap, true},
		{`city_backstreet-rare`, `city_backstreet`, TierRare, true},
		{`Pocket-Average`, `pocket`, TierAverage, true},
		{`cheap`, ``, TierCheap, true},
		{`interior`, ``, ``, false},
		{`interior-legendary`, ``, ``, false},
		{`-cheap`, ``, ``, false},
	}
	for _, c := range cases {
		prefix, tier, ok := parseCorpusKey(c.key)
		if prefix != c.prefix || tier != c.tier || ok != c.ok {
			t.Errorf("%q: got (%q, %q, %v), want (%q, %q, %v)", c.key, prefix, tier, ok, c.prefix, c.tier, c.ok)
		}
	}
	if corpusKey(`dwelling`, TierRare) != `dwelling-rare` || corpusKey(``, TierCheap) != `cheap` {
		t.Fatal("corpusKey is parseCorpusKey's inverse")
	}
}

// An entry gets the checks a model's answer gets, plus an exact weight.
func TestCheckEntry(t *testing.T) {
	good := CorpusEntry{Name: `Bent Tin Thimble`, NameSimple: `thimble`, Description: `A tin thimble, pressed a little out of shape.`, Material: `Tin`, WeightLbs: 0.1, Value: 3}
	got, err := checkEntry(good)
	if err != nil {
		t.Fatal(err)
	}
	if got.Material != `tin` || got.NameSimple != `thimble` || got.Value != 3 || got.WeightLbs != 0.1 {
		t.Fatalf("cleaned like a reply, numbers untouched: %+v", got)
	}
	bad := map[string]func(e *CorpusEntry){
		`digits in the name`: func(e *CorpusEntry) { e.Name = `Tin Thimble 2` },
		`too heavy`:          func(e *CorpusEntry) { e.WeightLbs = 30 },
		`not a tenth`:        func(e *CorpusEntry) { e.WeightLbs = 0.15 },
		`no weight`:          func(e *CorpusEntry) { e.WeightLbs = 0 },
		`short description`:  func(e *CorpusEntry) { e.Description = `Tiny.` },
	}
	for name, change := range bad {
		e := good
		change(&e)
		if _, err := checkEntry(e); err == nil {
			t.Errorf("%s: must be refused", name)
		}
	}
}

// A typo in a field name is an error, not a silently empty field.
func TestDecodeStrictRefusesUnknownFields(t *testing.T) {
	var doc seedDoc
	if err := decodeStrict([]byte("entries:\n  cheap:\n    - name: X\n      valeu: 3\n"), &doc); err == nil {
		t.Fatal("an unknown field must be refused")
	}
	if err := decodeStrict([]byte(``), &doc); err != nil {
		t.Fatalf("an empty document is empty, not an error: %v", err)
	}
}

// testSeed is a small seed: interior and fort in the dwelling group, ruins
// its own group, a bare cheap pool whose one entry is priced far above the
// tier (the clamp test), and a pocket pool with one entry too heavy for a
// pocket.
const testSeed = `groups:
  interior: dwelling
  fort: dwelling
  ruins: ruins
entries:
  interior-cheap:
    - name: Bent Tin Thimble
      name_simple: thimble
      description: A tin thimble, pressed a little out of shape by a careless heel.
      material: tin
      weight_lbs: 0.1
      value: 3
  dwelling-cheap:
    - name: Chipped Clay Marble
      name_simple: marble
      description: A small clay marble, glazed blue long ago and chipped since.
      material: clay
      weight_lbs: 0.1
      value: 2
  cheap:
    - name: Knotted Twine Bracelet
      name_simple: bracelet
      description: A bracelet of knotted brown twine, frayed where a wrist rubbed it.
      material: twine
      weight_lbs: 0.1
      value: 99
  pocket-cheap:
    - name: Brass Snuff Spoon
      name_simple: spoon
      description: A tiny brass spoon for snuff, its bowl no bigger than a fingernail.
      material: brass
      weight_lbs: 0.1
      value: 2
    - name: Heavy Pewter Tankard
      name_simple: tankard
      description: A pewter tankard with a hinged lid, far too heavy for any pocket.
      material: pewter
      weight_lbs: 1.5
      value: 5
  ruins-average:
    - name: Faded Mosaic Tile
      name_simple: tile
      description: A square tile from an old floor, painted with half of a red bird.
      material: fired clay
      weight_lbs: 0.3
      value: 13
`

const testSpoolOverlay = `entries:
  interior-cheap:
    - name: Painted Wooden Spool
      name_simple: spool
      description: A wooden thread spool painted with a band of faded blue.
      material: wood
      weight_lbs: 0.2
      value: 4
      from_record: B0000007
      zone: ashwick
      biome: interior
      model: gpt-test
      prompt_version: 3
      promoted_at: 2026-09-28T12:00:00Z
`

func writeTestFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}

// withCorpus writes a seed and an overlay (either may be empty, meaning no
// file) to a temp dir and loads them. The corpus is cleared afterwards.
func withCorpus(t *testing.T, seed, overlay string) (seedPath, overlayPath string, rep CorpusReport) {
	t.Helper()
	dir := t.TempDir()
	seedPath = filepath.Join(dir, seedFileName)
	overlayPath = filepath.Join(dir, `baubles`, overlayFileName)
	if seed != `` {
		writeTestFile(t, seedPath, seed)
	}
	if overlay != `` {
		writeTestFile(t, overlayPath, overlay)
	}
	t.Cleanup(ClearCorpusForTest)
	return seedPath, overlayPath, LoadCorpusFrom(seedPath, overlayPath)
}

func TestLoadCorpusReadsBothLayers(t *testing.T) {
	_, _, rep := withCorpus(t, testSeed, testSpoolOverlay)
	if rep.Seed != 6 || rep.Promoted != 1 || len(rep.Skipped) != 0 || rep.SeedErr != nil || rep.Quarantined != `` {
		t.Fatalf("report: %+v", rep)
	}
	if g, ok := GroupOf(`Interior`); !ok || g != `dwelling` {
		t.Fatalf("interior is in dwelling, got %q %v", g, ok)
	}
	if _, ok := GroupOf(`water`); ok {
		t.Fatal("water has no group")
	}
	if seed, promoted := CorpusCounts(); seed != 6 || promoted != 1 {
		t.Fatalf("counts: %d seed, %d promoted", seed, promoted)
	}
}

// Every entry must pass what a model's answer passes; one that fails is
// skipped and reported, and a key no pool can have is skipped whole.
func TestLoadCorpusSkipsWhatAModelAnswerCouldNotPass(t *testing.T) {
	seed := testSeed + `  interior-rare:
    - name: Silver Cup Number 2
      name_simple: cup
      description: A silver cup with a number scratched on its base.
      weight_lbs: 0.5
      value: 90
    - name: Stone Idol Head
      name_simple: idol
      description: A stone head broken from a small idol, far heavier than it looks.
      weight_lbs: 40
      value: 120
    - name: Plain Pewter Cup
      name_simple: cup
      description: A plain pewter drinking cup, dented on one side.
      material: pewter
      weight_lbs: 0.5
      value: 60
  castle-cheap:
    - name: Rusty Castle Nail
      name_simple: nail
      description: A square iron nail, rusted almost through.
      weight_lbs: 0.1
      value: 1
`
	_, _, rep := withCorpus(t, seed, ``)
	if rep.Seed != 7 || len(rep.Skipped) != 3 {
		t.Fatalf("seven usable, three skipped (digits, weight, unknown key): %+v", rep)
	}
}

// A broken seed is logged and the corpus runs without it: the overlay
// still loads (a bare tier key needs no groups) and nothing panics.
func TestMalformedSeedIsLoggedNotFatal(t *testing.T) {
	overlay := `entries:
  cheap:
    - name: Painted Wooden Spool
      name_simple: spool
      description: A wooden thread spool painted with a band of faded blue.
      weight_lbs: 0.2
      value: 4
      from_record: B0000007
      promoted_at: 2026-09-28T12:00:00Z
`
	_, _, rep := withCorpus(t, "entries: [this is not a map\n", overlay)
	if rep.SeedErr == nil || rep.Seed != 0 || rep.Promoted != 1 {
		t.Fatalf("report: %+v", rep)
	}
}

// A corrupt overlay is living state: moved aside (never deleted), logged,
// and the overlay restarts empty. An empty file is empty, not corrupt.
func TestCorruptOverlayIsQuarantined(t *testing.T) {
	seedPath, overlayPath, rep := withCorpus(t, testSeed, "entries: {this is: [not closed\n")
	if rep.Quarantined == `` || rep.Promoted != 0 || rep.Seed != 6 {
		t.Fatalf("report: %+v", rep)
	}
	if _, err := os.Stat(overlayPath); !os.IsNotExist(err) {
		t.Fatalf("the corrupt file is moved aside: %v", err)
	}
	if _, err := os.Stat(rep.Quarantined); err != nil {
		t.Fatalf("the quarantined copy is kept for recovery: %v", err)
	}
	writeTestFile(t, overlayPath, ``)
	if rep := LoadCorpusFrom(seedPath, overlayPath); rep.Quarantined != `` {
		t.Fatalf("an empty overlay must not be quarantined: %+v", rep)
	}
}

func TestNoCorpusFilesIsAnEmptyCorpus(t *testing.T) {
	_, _, rep := withCorpus(t, ``, ``)
	if rep.Seed != 0 || rep.Promoted != 0 || rep.SeedErr != nil || rep.Quarantined != `` {
		t.Fatalf("report: %+v", rep)
	}
}

// A reload that cannot read the seed keeps the seed already in use (a typo
// in a hand edit must not empty every pool); the overlay is still read
// again. A first load has nothing to keep.
func TestReloadWithABrokenSeedKeepsTheSeedInUse(t *testing.T) {
	seedPath, overlayPath, _ := withCorpus(t, testSeed, ``)
	writeTestFile(t, seedPath, "entries: [this is not a map\n")
	writeTestFile(t, overlayPath, testSpoolOverlay)
	rep := LoadCorpusFrom(seedPath, overlayPath)
	if rep.SeedErr == nil || !rep.SeedKept || rep.Seed != 6 || rep.Promoted != 1 {
		t.Fatalf("report: %+v", rep)
	}
	if g, ok := GroupOf(`fort`); !ok || g != `dwelling` {
		t.Fatalf("the kept seed keeps its groups too, got %q %v", g, ok)
	}
	if seed, _ := CorpusCounts(); seed != 6 {
		t.Fatalf("the kept seed is in use: %d", seed)
	}
}

// A corrupt overlay that cannot be moved aside is still where a save would
// write: the pool marks it broken (writers refuse, Task 7) and the next
// reload that can move it aside clears the mark.
func TestAnOverlayThatCannotBeQuarantinedIsBroken(t *testing.T) {
	quarantineOverlay = func(string) (string, error) { return ``, errors.New(`disk says no`) }
	t.Cleanup(func() { quarantineOverlay = util.QuarantineCorrupt })
	corrupt := "entries: {this is: [not closed\n"
	seedPath, overlayPath, rep := withCorpus(t, testSeed, corrupt)
	if !rep.OverlayBroken || rep.Quarantined != `` || rep.Promoted != 0 || rep.Seed != 6 {
		t.Fatalf("report: %+v", rep)
	}
	if data, err := os.ReadFile(overlayPath); err != nil || string(data) != corrupt {
		t.Fatalf("the file is left exactly as it was: %v", err)
	}
	quarantineOverlay = util.QuarantineCorrupt
	if rep := LoadCorpusFrom(seedPath, overlayPath); rep.OverlayBroken || rep.Quarantined == `` {
		t.Fatalf("moved aside on the next reload, and writable again: %+v", rep)
	}
}

// Search finds merge the overlay and the seed at biome-tier and
// group-tier into one pool, and fall to the bare tier only when that pool
// is empty. An empty or unmapped biome skips both.
func TestFallbackMergesBiomeAndGroupThenTier(t *testing.T) {
	withCorpus(t, testSeed, testSpoolOverlay)

	seen := map[string]bool{}
	for i := range 3 {
		drewFrom := 0
		res := Fallback(Place{Biome: `interior`}, TierCheap, SourceSearch, nil, func(n int) int { drewFrom = n; return i % n })
		if drewFrom != 3 {
			t.Fatalf("interior-cheap (overlay and seed) and dwelling-cheap are one pool of three, drew from %d", drewFrom)
		}
		if res.Generator != GeneratorCorpus || res.Moderated || res.PlayerKey {
			t.Fatalf("corpus result: %+v", res)
		}
		seen[res.Reply.Name] = true
	}
	for _, name := range []string{`Painted Wooden Spool`, `Bent Tin Thimble`, `Chipped Clay Marble`} {
		if !seen[name] {
			t.Errorf("%s is in the merged pool", name)
		}
	}

	if res := Fallback(Place{Biome: `fort`}, TierCheap, SourceSearch, nil, first); res.Reply.Name != `Chipped Clay Marble` || res.Model != `corpus:dwelling-cheap` {
		t.Fatalf("fort has no pool of its own: its group's, got %+v", res)
	}
	for _, biome := range []string{`ruins`, ``, `water`} {
		if res := Fallback(Place{Biome: biome}, TierCheap, SourceSearch, nil, first); res.Reply.Name != `Knotted Twine Bracelet` || res.Model != `corpus:cheap` {
			t.Fatalf("biome %q: nothing at biome or group, so the bare tier, got %+v", biome, res)
		}
	}
	// ruins is both a biome and a group: its pool is counted once.
	drewFrom := 0
	res := Fallback(Place{Biome: `ruins`}, TierAverage, SourceSearch, nil, func(n int) int { drewFrom = n; return 0 })
	if res.Reply.Name != `Faded Mosaic Tile` || drewFrom != 0 {
		t.Fatalf("one entry, drawn without a roll: %+v (drew from %d)", res, drewFrom)
	}
}

// Values are stored as written and clamped into the tier when used.
func TestFallbackClampsTheValueIntoTheTier(t *testing.T) {
	withCorpus(t, testSeed, ``)
	if res := Fallback(Place{}, TierCheap, SourceSearch, nil, first); res.Reply.Value != TierCheap.Range().Max {
		t.Fatalf("99 gold in the cheap pool is clamped to the cheap maximum, got %d", res.Reply.Value)
	}
}

// Pickpocketed finds use pocket-tier then tier, and only what fits a
// pocket (TooBigFor). Nothing that fits: a generic trinket.
func TestFallbackPocketFindsFitAPocket(t *testing.T) {
	withCorpus(t, testSeed, ``)
	for i := range 4 {
		res := Fallback(Place{Biome: `interior`}, TierCheap, SourcePickpocket, nil, func(n int) int { return i % n })
		if res.Reply.Name != `Brass Snuff Spoon` || res.Model != `corpus:pocket-cheap` {
			t.Fatalf("the tankard is too heavy for a pocket; only the spoon: %+v", res)
		}
	}
	if res := Fallback(Place{}, TierAverage, SourcePickpocket, nil, first); res.Generator != GeneratorLocal || res.Reply.Name != `Trinket` {
		t.Fatalf("no pocket-average and no average pool: a generic trinket, got %+v", res)
	}
	setBaubleConfig(t, func(b *configs.Balance) { b.BaublePickpocketMaxWeight = 0.05 })
	if res := Fallback(Place{}, TierCheap, SourcePickpocket, nil, first); res.Generator != GeneratorLocal || res.Reply.Name != `Trinket` {
		t.Fatalf("every entry is over the pocket limit, the bare tier's too: a generic trinket, got %+v", res)
	}
}

// Entries a zone found lately are avoided. When every entry of the pool was
// found lately, the least recent is taken; recency never widens the key.
func TestFallbackAvoidsRecentNamesWithoutFallingThrough(t *testing.T) {
	withCorpus(t, testSeed, ``)
	for i := range 4 {
		res := Fallback(Place{Biome: `interior`}, TierCheap, SourceSearch, []string{`Bent Tin Thimble`}, func(n int) int { return i % n })
		if res.Reply.Name != `Chipped Clay Marble` {
			t.Fatalf("the only entry not found lately, got %q", res.Reply.Name)
		}
	}
	recent := []string{`Bent Tin Thimble`, `Chipped Clay Marble`} // newest first
	if res := Fallback(Place{Biome: `interior`}, TierCheap, SourceSearch, recent, first); res.Reply.Name != `Chipped Clay Marble` {
		t.Fatalf("all recent: the least recent, never the bare tier's bracelet, got %q", res.Reply.Name)
	}
	recent = []string{`chipped clay marble`, `Bent Tin Thimble`}
	if res := Fallback(Place{Biome: `interior`}, TierCheap, SourceSearch, recent, first); res.Reply.Name != `Bent Tin Thimble` {
		t.Fatalf("names match without case: the thimble is now least recent, got %q", res.Reply.Name)
	}
}

func TestFallbackWithNoCorpusIsAGenericTrinket(t *testing.T) {
	ClearCorpusForTest()
	res := Fallback(Place{Biome: `interior`}, TierAverage, SourceSearch, nil, nil)
	if res.Generator != GeneratorLocal || res.Reply.Name != `Trinket` || res.Reply.Value != TierAverage.RollValue(nil) {
		t.Fatalf("empty corpus: exactly the old generic trinket, got %+v", res)
	}
}

// The names to avoid are the zone's newest model and corpus finds; a
// generic trinket is not a name.
func TestRecentFallbackNamesCountsModelAndCorpusFinds(t *testing.T) {
	withCatalog(t)
	_, _ = Create(Record{Name: `Old Cup`, Zone: `ashwick`, Generator: GeneratorOpenAI})
	_, _ = Create(Record{Name: `Trinket`, Zone: `ashwick`, Generator: GeneratorLocal})
	_, _ = Create(Record{Name: `Bent Tin Thimble`, Zone: `ashwick`, Generator: GeneratorCorpus})
	_, _ = Create(Record{Name: `Elsewhere`, Zone: `thornwall`, Generator: GeneratorCorpus})
	got := RecentFallbackNames(`ashwick`, 5)
	if len(got) != 2 || got[0] != `Bent Tin Thimble` || got[1] != `Old Cup` {
		t.Fatalf("newest first, model and corpus finds in the zone only: %v", got)
	}
}

// Every way Generate gives up (no generator, an error, a pocket-sized
// refusal) now goes to the corpus.
func TestGenerateFallsBackToTheCorpus(t *testing.T) {
	withCatalog(t)
	withCorpus(t, testSeed, ``)
	SetGenerator(nil, nil)
	req := GenRequest{Tier: TierCheap, Source: SourceSearch, Place: Place{Zone: `ashwick`, Biome: `fort`}}
	if res := Generate(context.Background(), req, nil); res.Generator != GeneratorCorpus || res.Reply.Name != `Chipped Clay Marble` {
		t.Fatalf("no generator: the corpus, got %+v", res)
	}

	installGenerator(t, func(ctx context.Context, req GenRequest) (GenResult, error) {
		return GenResult{}, errors.New(`boom`)
	})
	if res := Generate(context.Background(), req, nil); res.Generator != GeneratorCorpus {
		t.Fatalf("a failed call: the corpus, got %+v", res)
	}

	installGenerator(t, func(ctx context.Context, req GenRequest) (GenResult, error) {
		r := goodReply()
		r.Name, r.NameSimple = `Bronze Funeral Urn`, `urn`
		return GenResult{Reply: r}, nil
	})
	pocket := GenRequest{Tier: TierCheap, Source: SourcePickpocket, Place: Place{Zone: `ashwick`, Biome: `fort`}}
	if res := Generate(context.Background(), pocket, nil); res.Generator != GeneratorCorpus || res.Reply.Name != `Brass Snuff Spoon` {
		t.Fatalf("an urn is too big for a pocket: a pocket entry, got %+v", res)
	}
}

// Mint with no result (tests only) draws from the corpus too, and the
// record says so.
func TestMintWithNoResultDrawsFromTheCorpus(t *testing.T) {
	withCatalog(t)
	withCorpus(t, testSeed, ``)
	_, rec, err := Mint(MintOpts{Source: SourceSearch, Place: Place{RoomId: 1, Zone: `ashwick`, Biome: `fort`}, Tier: TierCheap, Randn: first})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Generator != GeneratorCorpus || rec.Status != StatusReady || rec.Name != `Chipped Clay Marble` || rec.Model != `corpus:dwelling-cheap` {
		t.Fatalf("record: %+v", rec)
	}
}

// A player-key name never reaches a fallback's recent list, like
// RecentNames: it is its finder's text, never compared for anyone else.
func TestRecentFallbackNamesSkipsPlayerKeyNames(t *testing.T) {
	withCatalog(t)
	_, _ = Create(Record{Name: `Old Cup`, Zone: `ashwick`, Generator: GeneratorOpenAI})
	_, _ = Create(Record{Name: `Player Written`, Zone: `ashwick`, Generator: GeneratorOpenAI, PlayerKey: true})
	got := RecentFallbackNames(`ashwick`, 5)
	if len(got) != 1 || got[0] != `Old Cup` {
		t.Fatalf("the player-key name is left out: %v", got)
	}
}

// The loader keeps the CLEANED entry, seed and overlay alike: markup, ANSI
// tags and escape sequences stripped, the keyword and material lowercased
// (an over-long material dropped), and the value clamped into the tier at
// use.
func TestLoadCorpusKeepsTheCleanedEntry(t *testing.T) {
	seed := `groups:
  interior: dwelling
entries:
  interior-cheap:
    - name: "<b>Bent</b> Tin <ansi fg=\"red\">Thimble</ansi>\e[31m"
      name_simple: THIMBLE
      description: "A tin thimble, \e[1mpressed</ansi> a little out of shape."
      material: POLISHED TIN
      weight_lbs: 0.1
      value: 99
`
	overlay := `entries:
  cheap:
    - name: "Painted <i>Wooden</i> \e[32mSpool"
      name_simple: Spool
      description: "A wooden thread spool <ansi fg=\"blue\">painted</ansi> with a band of blue."
      material: wood that was painted many many times over the years
      weight_lbs: 0.2
      value: 500
      from_record: B0000007
      promoted_at: 2026-09-28T12:00:00Z
`
	_, _, rep := withCorpus(t, seed, overlay)
	if rep.Seed != 1 || rep.Promoted != 1 || len(rep.Skipped) != 0 {
		t.Fatalf("report: %+v", rep)
	}
	max := TierCheap.Range().Max

	s := Fallback(Place{Biome: `interior`}, TierCheap, SourceSearch, nil, first).Reply
	if s.Name != `Bent Tin Thimble` || s.NameSimple != `thimble` || s.Material != `polished tin` ||
		s.Description != `A tin thimble, pressed a little out of shape.` || s.Value != max {
		t.Fatalf("the seed entry is used cleaned: %+v", s)
	}
	o := Fallback(Place{}, TierCheap, SourceSearch, nil, first).Reply
	if o.Name != `Painted Wooden Spool` || o.NameSimple != `spool` || o.Material != `` ||
		o.Description != `A wooden thread spool painted with a band of blue.` || o.Value != max {
		t.Fatalf("the overlay entry is used cleaned: %+v", o)
	}
}

// One name is one chance: a name at both the biome key and the group key,
// or in both the overlay and the seed, is drawn once, and the overlay's
// text wins.
func TestFallbackDrawsEachNameOnce(t *testing.T) {
	seed := `groups:
  interior: dwelling
entries:
  interior-cheap:
    - name: Chipped Clay Marble
      name_simple: marble
      description: A clay marble from the interior pool, glazed blue long ago.
      weight_lbs: 0.1
      value: 2
  dwelling-cheap:
    - name: chipped clay marble
      name_simple: marble
      description: A clay marble from the dwelling pool, chipped since.
      weight_lbs: 0.1
      value: 2
    - name: Bent Tin Thimble
      name_simple: thimble
      description: A tin thimble, pressed a little out of shape by a careless heel.
      weight_lbs: 0.1
      value: 3
`
	overlay := `entries:
  dwelling-cheap:
    - name: Chipped Clay Marble
      name_simple: marble
      description: A clay marble the overlay holds, promoted from a model find.
      weight_lbs: 0.1
      value: 2
      from_record: B0000007
      promoted_at: 2026-09-28T12:00:00Z
`
	withCorpus(t, seed, overlay)
	descs := map[string]string{}
	for i := range 2 {
		drewFrom := 0
		res := Fallback(Place{Biome: `interior`}, TierCheap, SourceSearch, nil, func(n int) int { drewFrom = n; return i % n })
		if drewFrom != 2 {
			t.Fatalf("two names, two chances: drew from %d", drewFrom)
		}
		descs[normKey(res.Reply.Name)] = res.Reply.Description
	}
	if len(descs) != 2 || descs[`chipped clay marble`] != `A clay marble the overlay holds, promoted from a model find.` {
		t.Fatalf("each name once, the overlay's text for the marble: %v", descs)
	}
}

// Every key the loader accepts is one some find can draw, and nothing is
// drawn from a key the loader refused: knownPrefix and candidates agree.
func TestEveryLoadedKeyIsReachable(t *testing.T) {
	seed := `groups:
  interior: dwelling
  fort: dwelling
  ruins: ruins
  crypt: ruins
entries:
`
	keys := []string{`interior-cheap`, `fort-cheap`, `dwelling-cheap`, `ruins-cheap`, `crypt-cheap`, `pocket-cheap`, `cheap`, `castle-cheap`, `water-cheap`}
	for i, k := range keys {
		seed += fmt.Sprintf("  %s:\n    - name: Small Plain Find %s\n      name_simple: find\n      description: A small plain find, kept for a reachability test.\n      weight_lbs: 0.1\n      value: 2\n", k, strings.Repeat(`a`, i+1))
	}
	withCorpus(t, seed, ``)
	loaded := map[string]bool{}
	for k := range corpus.Load().seed {
		loaded[k] = true
	}
	drawn := map[string]bool{}
	draw := func(biome string, source Source) {
		for i := range 8 {
			res := Fallback(Place{Biome: biome}, TierCheap, source, nil, func(n int) int { return i % n })
			drawn[strings.TrimPrefix(res.Model, `corpus:`)] = true
		}
	}
	for _, b := range []string{``, `interior`, `fort`, `ruins`, `crypt`, `castle`, `water`, `dwelling`} {
		draw(b, SourceSearch)
	}
	draw(``, SourcePickpocket)
	for k := range loaded {
		if !drawn[k] {
			t.Errorf("%s loads but no find can draw it", k)
		}
	}
	for k := range drawn {
		if !loaded[k] {
			t.Errorf("%s was drawn but never loaded", k)
		}
	}
	if loaded[`castle-cheap`] || loaded[`water-cheap`] {
		t.Errorf("a biome with no group is refused at load: %v", loaded)
	}
}
