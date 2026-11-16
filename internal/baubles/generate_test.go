package baubles

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

func goodReply() Reply {
	return Reply{
		Name:        `Painted Wooden Horse`,
		NameSimple:  `horse`,
		Description: `A child's toy horse, its red paint flaking from the mane. One wheel is missing.`,
		Material:    `pine`,
		WeightLbs:   0.6,
		Value:       14,
	}
}

func installGenerator(t *testing.T, fn GeneratorFunc) {
	t.Helper()
	SetGenerator(fn, func() GeneratorInfo { return GeneratorInfo{Name: `test`, Model: `m`} })
	t.Cleanup(func() { SetGenerator(nil, nil) })
}

func TestGenerateWithNoGeneratorIsAGenericTrinket(t *testing.T) {
	SetGenerator(nil, nil)
	res := Generate(context.Background(), GenRequest{Tier: TierRare}, nil)
	if res.Generator != GeneratorLocal || res.Reply.Name != `Trinket` || !TierRare.Range().Contains(res.Reply.Value) {
		t.Fatalf("no key, no generator: %+v", res)
	}
	if _, ok := CurrentGenerator(); ok {
		t.Fatal("nothing installed")
	}
}

func TestGenerateUsesTheModelsAnswer(t *testing.T) {
	var got GenRequest
	installGenerator(t, func(ctx context.Context, req GenRequest) (GenResult, error) {
		got = req
		return GenResult{Reply: goodReply(), Model: `gpt-test`, Tokens: 321, PromptVersion: 1}, nil
	})
	res := Generate(context.Background(), GenRequest{Tier: TierAverage, RoomTitle: `Toy Shop`}, nil)
	if res.Generator != GeneratorOpenAI || res.Reply.Name != `Painted Wooden Horse` || res.Tokens != 321 {
		t.Fatalf("model answer: %+v", res)
	}
	if got.RoomTitle != `Toy Shop` || got.Tier != TierAverage {
		t.Fatalf("request passed through: %+v", got)
	}
	if info, ok := CurrentGenerator(); !ok || info.Name != `test` {
		t.Fatal("installed generator is reported")
	}
}

func TestGenerateFallsBackOnErrorOrBadText(t *testing.T) {
	installGenerator(t, func(ctx context.Context, req GenRequest) (GenResult, error) {
		return GenResult{}, errors.New(`boom`)
	})
	if res := Generate(context.Background(), GenRequest{Tier: TierCheap}, nil); res.Generator != GeneratorLocal || res.Reply.Name != `Trinket` {
		t.Fatalf("error: %+v", res)
	}

	installGenerator(t, func(ctx context.Context, req GenRequest) (GenResult, error) {
		r := goodReply()
		r.Name = `Horse 3000`
		return GenResult{Reply: r}, nil
	})
	if res := Generate(context.Background(), GenRequest{Tier: TierCheap}, nil); res.Generator != GeneratorLocal {
		t.Fatalf("a name with digits is unusable: %+v", res)
	}
}

// logTee keeps every log line written while it is installed.
type logTee struct {
	mu    sync.Mutex
	lines []string
}

func (l *logTee) Println(level string, v ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, ansiRE.ReplaceAllString(level+` `+fmt.Sprint(v...), ``))
}

// count is how many kept lines contain s.
func (l *logTee) count(s string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, line := range l.lines {
		if strings.Contains(line, s) {
			n++
		}
	}
	return n
}

// A find the ledger refuses (a spent allowance or share) is logged at most
// once a minute, as the companion logs its refusals (logBudgetRefusal): a
// finder over their allowance searching all day is one line a minute, not
// one a find. Any other failure is logged every time, as before.
func TestRefusedFindsAreLoggedOnceAMinute(t *testing.T) {
	tee := &logTee{}
	mudlog.SetupLogger(tee, "", "", false)
	t.Cleanup(func() { mudlog.SetupLogger(nil, "", "", false) })
	clock := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	refusalLog.mu.Lock()
	refusalLog.now, refusalLog.last = func() time.Time { return clock }, time.Time{}
	refusalLog.mu.Unlock()
	t.Cleanup(func() {
		refusalLog.mu.Lock()
		refusalLog.now, refusalLog.last = nil, time.Time{}
		refusalLog.mu.Unlock()
	})

	books := apiframework.NewBooksForTest()
	_, allowance := books.Reserve(apiframework.ConsumerBaubles, 10, false,
		apiframework.Charge{Dim: apiframework.DimBaublesFinder, UserId: 7, Limit: 1})
	t.Cleanup(apiframework.SetServerForTest(apiframework.ServerSettings{DailyTokenBudget: 100, BaublesSharePercent: 10}))
	_, share := books.Reserve(apiframework.ConsumerBaubles, 50, true)
	if apiframework.RefusedBy(allowance) != apiframework.DimBaublesFinder || apiframework.RefusedBy(share) != apiframework.RefusedShare {
		t.Fatalf("fixture: an allowance and a share refusal: %v / %v", allowance, share)
	}

	for _, refusal := range []error{allowance, share} {
		installGenerator(t, func(ctx context.Context, req GenRequest) (GenResult, error) { return GenResult{}, refusal })
		for range 5 {
			if res := Generate(context.Background(), GenRequest{Tier: TierCheap}, nil); res.Generator != GeneratorLocal {
				t.Fatalf("a refused find is a generic trinket: %+v", res)
			}
		}
	}
	if n := tee.count(`action="generate"`); n != 1 {
		t.Fatalf("ten refused finds inside one minute are one log line, got %d", n)
	}
	if tee.count(`generator="local"`) == 0 {
		t.Fatalf("the throttled refusal log must still name the generator the fallback used: %v", tee.lines)
	}
	clock = clock.Add(time.Minute)
	_ = Generate(context.Background(), GenRequest{Tier: TierCheap}, nil)
	_ = Generate(context.Background(), GenRequest{Tier: TierCheap}, nil)
	if n := tee.count(`action="generate"`); n != 2 {
		t.Fatalf("a minute later, one more line: %d", n)
	}

	// With a corpus loaded, a refused find draws from it rather than falling
	// all the way to a generic trinket, and the throttled log names the
	// corpus, not "local" (review finding 7: the two checks above only ever
	// exercised the no-corpus path).
	withCorpus(t, testSeed, ``)
	clock = clock.Add(time.Minute)
	installGenerator(t, func(ctx context.Context, req GenRequest) (GenResult, error) { return GenResult{}, allowance })
	for range 5 {
		if res := Generate(context.Background(), GenRequest{Tier: TierCheap}, nil); res.Generator != GeneratorCorpus {
			t.Fatalf("a refused find with a corpus loaded draws from it: %+v", res)
		}
	}
	if tee.count(`generator="corpus"`) == 0 {
		t.Fatalf("the throttled refusal log must name the corpus when that is what the fallback used: %v", tee.lines)
	}

	installGenerator(t, func(ctx context.Context, req GenRequest) (GenResult, error) { return GenResult{}, errors.New(`boom`) })
	for range 3 {
		_ = Generate(context.Background(), GenRequest{Tier: TierCheap}, nil)
	}
	if n := tee.count(`boom`); n != 3 {
		t.Fatalf("any other failure is logged every time: %d", n)
	}
}

func TestGenerateCapsTheWait(t *testing.T) {
	installGenerator(t, func(ctx context.Context, req GenRequest) (GenResult, error) {
		<-ctx.Done()
		return GenResult{}, ctx.Err()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	res := Generate(ctx, GenRequest{Tier: TierCheap}, nil)
	if time.Since(start) > 2*time.Second || res.Generator != GeneratorLocal {
		t.Fatalf("a cancelled call returns promptly with a generic trinket: %+v", res)
	}
}

// Each of Generate's fallback logs (an error, an unusable or player-key
// reply, and too big for a pocket) names the generator the fallback
// actually used (corpus or local), as the pickpocket log does
// (steal_pocket.go).
func TestFallbackLogsNameTheGeneratorUsed(t *testing.T) {
	tee := &logTee{}
	mudlog.SetupLogger(tee, "", "", false)
	t.Cleanup(func() { mudlog.SetupLogger(nil, "", "", false) })

	// No corpus loaded: the fallback is a generic trinket, generator local.
	installGenerator(t, func(ctx context.Context, req GenRequest) (GenResult, error) {
		return GenResult{}, errors.New(`boom`)
	})
	_ = Generate(context.Background(), GenRequest{Tier: TierCheap}, nil)
	if tee.count(`generator="local"`) == 0 {
		t.Fatalf("the error-path fallback must name its generator: %v", tee.lines)
	}

	// A corpus loaded and matching: the fallback draws from it, and the log
	// names it, for the unusable-reply path and the too-big-for-a-pocket one.
	withCorpus(t, testSeed, ``)

	installGenerator(t, func(ctx context.Context, req GenRequest) (GenResult, error) {
		r := goodReply()
		r.Name = `Horse 3000` // digits: unusable
		return GenResult{Reply: r}, nil
	})
	_ = Generate(context.Background(), GenRequest{Tier: TierCheap, Place: Place{Biome: `interior`}}, first)
	if tee.count(`generator="corpus"`) == 0 {
		t.Fatalf("the unusable-reply fallback must name the corpus: %v", tee.lines)
	}

	installGenerator(t, func(ctx context.Context, req GenRequest) (GenResult, error) {
		return GenResult{Reply: Reply{Name: `Bronze Funeral Urn`, NameSimple: `urn`,
			Description: `A small bronze urn with a chipped lid, heavier than it looks.`,
			WeightLbs:   0.3, Value: 3}, Model: `m`}, nil
	})
	_ = Generate(context.Background(), GenRequest{Tier: TierCheap, Source: SourcePickpocket, Place: Place{Biome: `interior`}}, first)
	if tee.count(`generator="corpus"`) < 2 {
		t.Fatalf("the too-big-for-a-pocket fallback must name the corpus too: %v", tee.lines)
	}
}

func TestCleanReply(t *testing.T) {
	r := goodReply()
	r.Name = "  Painted\t<ansi fg=\"red\">Wooden</ansi>   Horse "
	r.NameSimple = `Horse`
	r.Description = "A toy.\x1b[31m It has\nno wheels, and a tail of real horsehair."
	got, err := CleanReply(r)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != `Painted Wooden Horse` || got.NameSimple != `horse` || strings.ContainsAny(got.Description, "\x1b\n<>") {
		t.Fatalf("cleaned: %+v", got)
	}

	cases := map[string]func(*Reply){
		`empty name`:        func(r *Reply) { r.Name = `  ` },
		`digits`:            func(r *Reply) { r.Name = `Lot 42 Cup` },
		`long name`:         func(r *Reply) { r.Name = strings.Repeat(`Long `, 10) },
		`short description`: func(r *Reply) { r.Description = `A cup.` },
		`long description`:  func(r *Reply) { r.Description = strings.Repeat(`word `, 100) },
	}
	for name, change := range cases {
		r := goodReply()
		change(&r)
		if _, err := CleanReply(r); !errors.Is(err, ErrUnusableReply) {
			t.Fatalf("%s must be refused, got %v", name, err)
		}
	}
}

func TestCleanReplyKeywords(t *testing.T) {
	r := goodReply()
	r.NameSimple = `sword`
	if got, _ := CleanReply(r); got.NameSimple != `horse` {
		t.Fatalf("a real item keyword would hijack commands; the name's own last word instead, got %q", got.NameSimple)
	}
	r.NameSimple = `two words`
	if got, _ := CleanReply(r); got.NameSimple != `horse` {
		t.Fatalf("a bad keyword falls back to the last word of the name, got %q", got.NameSimple)
	}
	r.Name = `Bent Iron Key`
	r.NameSimple = ``
	if got, _ := CleanReply(r); got.Name != `Bent Iron Key` || got.NameSimple != `iron` {
		t.Fatalf("the name may keep the word; the keyword may not (the name's next word instead): %+v", got)
	}
	r.Name = `Odd Wee Key`
	if got, _ := CleanReply(r); got.NameSimple != `trinket` {
		t.Fatalf("no word of four letters or more left: trinket, got %q", got.NameSimple)
	}

	// Natural finds: general words belong to real items (the quest's
	// Reckoning Bone, the weighted stones, crafting materials); a specific
	// noun is fine.
	for _, general := range []string{`bone`, `stone`, `shell`, `gem`, `crystal`, `pearl`, `amber`} {
		r = goodReply()
		r.Name = `Weathered Fox Jawbone`
		r.NameSimple = general
		if got, _ := CleanReply(r); got.NameSimple != `jawbone` {
			t.Fatalf("%q is a real item's keyword; the specific noun of the name instead, got %q", general, got.NameSimple)
		}
	}
	for _, specific := range []string{`jawbone`, `quartz`, `agate`, `fossil`, `antler`} {
		r = goodReply()
		r.Name = `Small Natural Curio`
		r.NameSimple = specific
		if got, _ := CleanReply(r); got.NameSimple != specific {
			t.Fatalf("%q is fine as a keyword, got %q", specific, got.NameSimple)
		}
	}
}

func TestMintFromAModelResult(t *testing.T) {
	withCatalog(t)
	r := goodReply()
	r.Value = 900     // outside average
	r.WeightLbs = 400 // outside the bounds
	res := GenResult{Reply: r, Generator: GeneratorOpenAI, Model: `gpt-test`, PromptVersion: 1, Tokens: 250, Moderated: true}

	itm, rec, err := Mint(MintOpts{Source: SourceSearch, Place: NewPlace(1, `z`, ``, `city`), Tier: TierAverage, Result: &res})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != StatusReady || rec.Generator != GeneratorOpenAI || rec.Model != `gpt-test` || rec.Tokens != 250 || !rec.Moderated {
		t.Fatalf("audit fields: %+v", rec)
	}
	if rec.Value != 15 || rec.ValueProposed != 900 || rec.WeightLbs != MaxWeightLbs || rec.WeightProposed != 400 {
		t.Fatalf("clamped, with the proposal kept: %+v", rec)
	}
	if itm.Name() != `Painted Wooden Horse` || itm.GetSpec().Weight != MaxWeightLbs {
		t.Fatal("the item shows the model's name and the clamped weight")
	}
	if !itm.IsBauble() || itm.ItemId != items.BaubleItemId {
		t.Fatal("carrier item")
	}
}

func TestRecentNames(t *testing.T) {
	withCatalog(t)
	for _, n := range []string{`Old Cup`, `Tin Bell`, `Bone Comb`} {
		_, _ = Create(Record{Name: n, Zone: `ashwick`, Generator: GeneratorOpenAI})
	}
	_, _ = Create(Record{Name: `Trinket`, Zone: `ashwick`, Generator: GeneratorLocal})
	_, _ = Create(Record{Name: `Elsewhere`, Zone: `thornwall`, Generator: GeneratorOpenAI})

	got := RecentNames(`ashwick`, 2)
	if len(got) != 2 || got[0] != `Bone Comb` || got[1] != `Tin Bell` {
		t.Fatalf("newest model names in the zone only: %v", got)
	}
}

// A name a player's own key wrote never goes into another find's prompt
// (spec S3): RecentNames is sent to the model for every later find in the
// zone, whoever's key names it.
func TestRecentNamesSkipsPlayerKeyNames(t *testing.T) {
	withCatalog(t)
	_, _ = Create(Record{Name: `Old Cup`, Zone: `ashwick`, Generator: GeneratorOpenAI})
	_, _ = Create(Record{Name: `Player Written`, Zone: `ashwick`, Generator: GeneratorOpenAI, PlayerKey: true})
	got := RecentNames(`ashwick`, 5)
	if len(got) != 1 || got[0] != `Old Cup` {
		t.Fatalf("server-key names only: %v", got)
	}
}

// A keyword a loaded, authored item answers to (its keyword or its head
// noun) is refused as well as the fixed list, so `get lantern` is never a
// model-named trinket (analysis: normal-item collisions).
func TestCleanReplyKeepsOffLoadedItemsKeywords(t *testing.T) {
	orig := authoredKeyword
	authored := map[string]bool{`lantern`: true, `horse`: true}
	authoredKeyword = func(w string) bool { return authored[w] }
	t.Cleanup(func() { authoredKeyword = orig })

	r := goodReply() // "Painted Wooden Horse"
	r.NameSimple = `lantern`
	if got, _ := CleanReply(r); got.NameSimple != `wooden` {
		t.Fatalf("a loaded item's keyword, and the name's last word is one too: the next word, got %q", got.NameSimple)
	}
	authored[`wooden`], authored[`painted`] = true, true
	if got, _ := CleanReply(r); got.NameSimple != `trinket` {
		t.Fatalf("every word taken: trinket, got %q", got.NameSimple)
	}
	r.Name, r.NameSimple = `Tarnished Tin Whistle`, `lantern`
	if got, _ := CleanReply(r); got.NameSimple != `whistle` {
		t.Fatalf("falls back to the name's own last word, got %q", got.NameSimple)
	}
	r.NameSimple = `whistle`
	if got, _ := CleanReply(r); got.NameSimple != `whistle` {
		t.Fatal("a keyword no item answers to is kept")
	}
}

// With a real "Silver Dagger" and "Brass Locket" loaded, a "Tarnished Silver
// Locket" is keyed by neither word: a bauble keyed "silver" would fully
// match `get silver` and be taken instead of the dagger, which that word
// only partly matches (review of the keyword fallback).
func TestKeywordFallbackAvoidsRealItemsNameWords(t *testing.T) {
	restore := items.SeedItemsForTest(map[int]*items.ItemSpec{
		10: {ItemId: 10, Name: `Silver Dagger`, NameSimple: `dagger`},
		11: {ItemId: 11, Name: `Brass Locket`, NameSimple: `locket`},
	})
	defer restore()
	r := goodReply()
	r.Name, r.NameSimple = `Tarnished Silver Locket`, `locket`
	got, err := CleanReply(r)
	if err != nil {
		t.Fatal(err)
	}
	if got.NameSimple != `tarnished` {
		t.Fatalf("neither silver nor locket: the name's one word no real item uses, got %q", got.NameSimple)
	}
}

// Pickpocketed finds are pocket-sized: the weight is clamped to the pocket
// limit (Balance.BaublePickpocketMaxWeight, 1 lb) wherever it is set, and
// a naming the model itself weighed over the limit, or whose name is a
// thing no pocket holds, is not used at all.
func TestPickpocketFindsArePocketSized(t *testing.T) {
	if MaxWeightFor(SourcePickpocket) != 1.0 || MaxWeightFor(SourceSearch) != MaxWeightLbs {
		t.Fatalf("limits: pocket %.1f, search %.1f", MaxWeightFor(SourcePickpocket), MaxWeightFor(SourceSearch))
	}
	if got := ClampWeightFor(1.6, SourcePickpocket); got != 1.0 {
		t.Fatalf("a little over is clamped to the pocket: %.1f", got)
	}
	if got := ClampWeightFor(1.6, SourceSearch); got != 1.6 {
		t.Fatalf("a search find is not: %.1f", got)
	}
	if l := ApplyLimitsFor(Reply{WeightLbs: 1.8, Value: 3}, TierCheap, SourcePickpocket); l.Reply.WeightLbs != 1.0 || l.ProposedWeight != 1.8 {
		t.Fatalf("limits keep what was proposed, apply the pocket: %+v", l)
	}

	heavy := goodReply()
	heavy.Name, heavy.NameSimple, heavy.WeightLbs = `Iron Strongbox`, `strongbox`, 6
	installGenerator(t, func(ctx context.Context, req GenRequest) (GenResult, error) {
		return GenResult{Reply: heavy, Model: `m`}, nil
	})
	if res := Generate(context.Background(), GenRequest{Tier: TierCheap, Source: SourcePickpocket}, nil); res.Generator != GeneratorLocal || res.Reply.Name != `Trinket` {
		t.Fatalf("a strongbox from a pocket is refused: %+v", res.Reply)
	}
	if res := Generate(context.Background(), GenRequest{Tier: TierCheap, Source: SourceSearch}, nil); res.Reply.Name != `Iron Strongbox` {
		t.Fatalf("the same object found by searching is fine: %+v", res.Reply)
	}
	for name, r := range map[string]Reply{
		`over the limit`:           {Name: `Tarnished Silver Snuffbox`, NameSimple: `snuffbox`, Description: `A silver snuffbox, heavier than it looks.`, WeightLbs: 1.9, Value: 3},
		`a big thing, heavy`:       {Name: `Heavy Iron Candlestick`, NameSimple: `candlestick`, Description: `A heavy iron candlestick, crusted with wax.`, WeightLbs: 1.9, Value: 3},
		`a big thing called light`: {Name: `Small Bronze Urn`, NameSimple: `urn`, Description: `A small bronze urn with a chipped lip.`, WeightLbs: 0.4, Value: 3},
	} {
		installGenerator(t, func(ctx context.Context, req GenRequest) (GenResult, error) {
			return GenResult{Reply: r, Model: `m`}, nil
		})
		if res := Generate(context.Background(), GenRequest{Tier: TierCheap, Source: SourcePickpocket}, nil); res.Generator != GeneratorLocal {
			t.Errorf("%s: refused, a generic trinket: %+v", name, res.Reply)
		}
	}
	small := goodReply()
	small.Name, small.NameSimple, small.WeightLbs = `Tarnished Brass Thimble`, `thimble`, 0.1
	installGenerator(t, func(ctx context.Context, req GenRequest) (GenResult, error) {
		return GenResult{Reply: small, Model: `m`}, nil
	})
	if res := Generate(context.Background(), GenRequest{Tier: TierCheap, Source: SourcePickpocket}, nil); res.Generator != GeneratorOpenAI {
		t.Fatalf("a pocket-sized thing is kept: %+v", res)
	}
}

// Minting a pickpocketed find clamps even the generic trinket to the pocket.
func TestMintHoldsAPickpocketFindToThePocket(t *testing.T) {
	withCatalog(t)
	_, rec, err := Mint(MintOpts{Source: SourcePickpocket, Tier: TierRare,
		Result: &GenResult{Reply: Reply{Name: `Silver Snuff Box`, NameSimple: `snuffbox`, Description: `A small silver snuff box with a hinged lid.`, WeightLbs: 1.9, Value: 90}, Generator: GeneratorOpenAI}})
	if err != nil || rec.WeightLbs != 1.0 || rec.WeightProposed != 1.9 || rec.Source != SourcePickpocket {
		t.Fatalf("pocket weight: %+v %v", rec, err)
	}
}

// The engine holds every generator to the player-key rules, whatever the
// module did (spec S3; owner ruling 2026-09-29): player-key text is plain,
// and either moderated (everyone reads it) or kept to its finder
// (FinderOnly, which needs a finder). Nothing else is ever finder-only.
func TestGenerateHoldsPlayerKeyTextToItsRules(t *testing.T) {
	odd := goodReply()
	odd.Name = "P\U00000430inted Wooden Horse"
	refused := map[string]struct {
		res    GenResult
		finder int
	}{
		`unmoderated, not kept to the finder`: {GenResult{Reply: goodReply(), PlayerKey: true}, 7},
		`not plain`:                           {GenResult{Reply: odd, PlayerKey: true, Moderated: true}, 7},
		`not plain, finder-only`:              {GenResult{Reply: odd, PlayerKey: true, FinderOnly: true}, 7},
		`finder-only with no finder`:          {GenResult{Reply: goodReply(), PlayerKey: true, FinderOnly: true}, 0},
		`finder-only on the server's key`:     {GenResult{Reply: goodReply(), FinderOnly: true}, 7},
	}
	for name, c := range refused {
		installGenerator(t, func(ctx context.Context, req GenRequest) (GenResult, error) { return c.res, nil })
		if got := Generate(context.Background(), GenRequest{Tier: TierAverage, FinderUserId: c.finder}, nil); got.Generator != GeneratorLocal {
			t.Errorf("%v: a generic trinket, got %+v", name, got)
		}
	}
	kept := map[string]struct {
		res            GenResult
		wantFinderOnly bool
	}{
		`moderated`:   {GenResult{Reply: goodReply(), PlayerKey: true, Moderated: true}, false},
		`finder-only`: {GenResult{Reply: goodReply(), PlayerKey: true, FinderOnly: true}, true},
		// Moderated wins over FinderOnly: moderated text is everyone's, so a
		// module setting both must still come out unhidden from anyone else
		// (Generate clears FinderOnly on the moderated branch).
		`moderated and finder-only`: {GenResult{Reply: goodReply(), PlayerKey: true, Moderated: true, FinderOnly: true}, false},
	}
	for name, c := range kept {
		installGenerator(t, func(ctx context.Context, req GenRequest) (GenResult, error) { return c.res, nil })
		got := Generate(context.Background(), GenRequest{Tier: TierAverage, FinderUserId: 7}, nil)
		if got.Generator != GeneratorOpenAI || !got.PlayerKey || got.FinderOnly != c.wantFinderOnly {
			t.Errorf("%v: used as it came: %+v", name, got)
		}
	}
}

// A player-key call whose text fails validation falls back cleanly: the
// fallback is nobody's key, unmoderated, and a record minted from it is
// never kept to its finder.
func TestFailedPlayerKeyCallFallsBackCleanly(t *testing.T) {
	withCatalog(t)
	odd := goodReply()
	odd.Name = "P\U00000430inted Wooden Horse" // not plain: fails CheckPlayerKeyText
	installGenerator(t, func(ctx context.Context, req GenRequest) (GenResult, error) {
		return GenResult{Reply: odd, PlayerKey: true, Moderated: true}, nil
	})
	res := Generate(context.Background(), GenRequest{Tier: TierAverage, FinderUserId: 7}, nil)
	if res.PlayerKey || res.Moderated {
		t.Fatalf("the fallback is nobody's key: %+v", res)
	}
	_, rec, err := Mint(MintOpts{Source: SourceSearch, Place: NewPlace(1, `z`, ``, `city`), FinderUserId: 7, Tier: TierAverage, Result: &res})
	if err != nil {
		t.Fatal(err)
	}
	if rec.PlayerKey || rec.Moderated || rec.KeptToFinder() {
		t.Fatalf("the record is never kept to a finder: %+v", rec)
	}
}

// A player's own key proposes a value the server does not trust, even
// clamped: Mint rolls it in the tier instead, keeping the proposal for the
// record (spec S3). A server-key value is kept, clamped.
func TestMintRollsAPlayerKeyFindsValue(t *testing.T) {
	withCatalog(t)
	r := goodReply()
	r.Value = 14 // inside average (10 to 15), so a clamp alone would keep it
	res := GenResult{Reply: r, Generator: GeneratorOpenAI, Moderated: true, PlayerKey: true}
	_, rec, err := Mint(MintOpts{Source: SourceSearch, Place: NewPlace(1, `z`, ``, `city`), Tier: TierAverage, Result: &res, Randn: first})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Value != TierAverage.Range().Min || rec.ValueProposed != 14 || !rec.PlayerKey {
		t.Fatalf("rolled by the server (first die: the tier's minimum), proposal kept: %+v", rec)
	}

	res.PlayerKey = false
	_, rec, err = Mint(MintOpts{Source: SourceSearch, Place: NewPlace(1, `z`, ``, `city`), Tier: TierAverage, Result: &res, Randn: first})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Value != 14 {
		t.Fatalf("a server-key value stands: %+v", rec)
	}
}

// A finder-only result reaches the record kept to the finder Mint records
// (owner ruling 2026-09-29), whatever GenResult.FinderOnly said: the record
// derives it from PlayerKey and Moderated. A regeneration, always on the
// server's key, makes it everyone's.
func TestFinderOnlyReachesTheRecordAndRegenClearsIt(t *testing.T) {
	withCatalog(t)
	// FinderOnly deliberately left false: the record must not trust it.
	res := GenResult{Reply: goodReply(), Generator: GeneratorOpenAI, PlayerKey: true, Moderated: false}
	_, rec, err := Mint(MintOpts{Source: SourceSearch, Place: NewPlace(1, `z`, ``, `city`), FinderUserId: 7, Tier: TierAverage, Result: &res, Randn: first})
	if err != nil {
		t.Fatal(err)
	}
	if !rec.KeptToFinder() || rec.FoundByUserId != 7 || rec.View().Finder == nil {
		t.Fatalf("finder-only, kept to user 7: %+v", rec)
	}
	got, _, err := ApplyRegenerated(rec.Id, GenResult{Reply: goodReply(), Generator: GeneratorOpenAI, Moderated: true}, `Admin`, first)
	if err != nil {
		t.Fatal(err)
	}
	if got.KeptToFinder() || got.PlayerKey || got.View().Finder != nil {
		t.Fatalf("named again on the server's key: everyone's: %+v", got)
	}
}

// A find drawn from the corpus is named text: Mint marks it ready and keeps
// its generator and pool.
func TestMintMarksACorpusResultReady(t *testing.T) {
	withCatalog(t)
	res := GenResult{Reply: Reply{Name: `Chipped Clay Marble`, NameSimple: `marble`, Description: `A small clay marble, glazed blue long ago.`, WeightLbs: 0.1, Value: 2}, Generator: GeneratorCorpus, Model: `corpus:street-cheap`}
	_, rec, err := Mint(MintOpts{Source: SourceSearch, Tier: TierCheap, Result: &res})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != StatusReady || rec.Generator != GeneratorCorpus || rec.Model != `corpus:street-cheap` || rec.Moderated || rec.PlayerKey {
		t.Fatalf("corpus record: %+v", rec)
	}
}
