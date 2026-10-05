package apiframework

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"gopkg.in/yaml.v2"
)

// R1, R3, R4, R8, R41: every charge is checked, and a refusal anywhere
// holds nothing anywhere.
func TestReserveChargesEveryAllowanceOrNone(t *testing.T) {
	ResetBudgetForTest(``)
	stranger := Charge{Dim: DimCompanionStranger, UserId: 2, Limit: 1000}
	perOwner := Charge{Dim: DimCompanionStrangersFor, UserId: 5, Limit: 1500}
	if _, err := budget.reserve(ConsumerCompanion, 900, 0, 0, true, []Charge{stranger, perOwner}); err != nil {
		t.Fatal(err)
	}
	other := Charge{Dim: DimCompanionStranger, UserId: 3, Limit: 1000}
	if _, err := budget.reserve(ConsumerCompanion, 700, 0, 0, true, []Charge{other, perOwner}); !errors.Is(err, ErrOverAllowance) {
		t.Fatalf("the second charge refuses the whole reservation: %v", err)
	}
	if Allowance(DimCompanionStranger, 3) != 0 || Allowance(DimCompanionStrangersFor, 5) != 900 || Today().Tokens != 900 {
		t.Fatalf("a refusal holds nothing anywhere: other=%d perOwner=%d server=%d",
			Allowance(DimCompanionStranger, 3), Allowance(DimCompanionStrangersFor, 5), Today().Tokens)
	}
	// The server's budget refusing charges no allowance either.
	if _, err := budget.reserve(ConsumerCompanion, 200, 1000, 0, true, []Charge{other}); !errors.Is(err, ErrOverBudget) {
		t.Fatalf("over the server's budget: %v", err)
	}
	if Allowance(DimCompanionStranger, 3) != 0 {
		t.Fatal("a server refusal leaves the allowance untouched")
	}
	// A limit of 0 is no cap, and the spend is still counted.
	if _, err := budget.reserve(ConsumerCompanion, 5000, 0, 0, true, []Charge{{Dim: DimCompanionOwner, UserId: 7}}); err != nil {
		t.Fatal(err)
	}
	if Allowance(DimCompanionOwner, 7) != 5000 {
		t.Fatalf("counted with no cap: %d", Allowance(DimCompanionOwner, 7))
	}
}

// R11: a relayed call (the player's own key) is charged to its allowances
// and to nothing of the server's, and the server's budget cannot refuse it.
func TestARelayReserveLeavesTheServerAlone(t *testing.T) {
	ResetBudgetForTest(``)
	c := Charge{Dim: DimCompanionStranger, UserId: 2, Limit: 1000}
	h, err := budget.reserve(ConsumerCompanion, 400, 1000, 0, false, []Charge{c})
	if err != nil {
		t.Fatal(err)
	}
	if h.SpendServer || len(h.Charges) != 1 || h.Tokens != 400 || h.Day != Today().Day {
		t.Fatalf("the hold records what it touched: %+v", h)
	}
	if u := Today(); u.Tokens != 0 || u.Outstanding != 0 || u.Calls != 0 || len(u.ByConsumer) != 0 {
		t.Fatalf("nothing of the server's: %+v", u)
	}
	SetSpentForTest(1000, 0)
	if _, err := budget.reserve(ConsumerCompanion, 400, 1000, 0, false, []Charge{c}); err != nil {
		t.Fatal("the server's spent budget does not refuse the player's own key")
	}
	if Allowance(DimCompanionStranger, 2) != 800 {
		t.Fatalf("both charged to the allowance: %d", Allowance(DimCompanionStranger, 2))
	}
}

// The ledger's clock is the only day.
func TestDayIsTheLedgersClock(t *testing.T) {
	k := NewBooksForTest()
	k.SetClockForTest(func() time.Time { return time.Date(2020, 3, 4, 23, 30, 0, 0, time.FixedZone(`x`, -5*3600)) })
	if k.Day() != `2020-03-05` {
		t.Fatalf("the ledger's clock, as a UTC date: %s", k.Day())
	}
}

// SetAllowanceForTest sets one counter, for tests that start mid-day.
func TestSetAllowanceForTest(t *testing.T) {
	k := NewBooksForTest()
	k.SetAllowanceForTest(DimCompanionOwner, 5, 1234)
	if k.Allowance(DimCompanionOwner, 5) != 1234 || k.Allowance(DimCompanionOwner, 6) != 0 {
		t.Fatal("one counter, by dimension and user")
	}
}

// R43: a refusal says which counter refused it, so a log line can tell a
// spent server day from one player's spent allowance.
func TestARefusalNamesItsCounter(t *testing.T) {
	ResetBudgetForTest(``)
	_, err := budget.reserve(ConsumerCompanion, 200, 100, 0, true, nil)
	if !errors.Is(err, ErrOverBudget) || RefusedBy(err) != RefusedGlobal {
		t.Fatalf("the day's budget: %v (%q)", err, RefusedBy(err))
	}
	c := Charge{Dim: DimCompanionStrangersFor, UserId: 5, Limit: 100}
	_, err = budget.reserve(ConsumerCompanion, 200, 0, 0, true, []Charge{{Dim: DimCompanionStranger, UserId: 2}, c})
	if !errors.Is(err, ErrOverAllowance) || RefusedBy(err) != DimCompanionStrangersFor {
		t.Fatalf("the charge that refused: %v (%q)", err, RefusedBy(err))
	}
	if RefusedBy(nil) != `` || RefusedBy(errors.New(`other`)) != `` {
		t.Fatal("anything else names no counter")
	}
}

// R14, R15, R20: a relayed count is held between nothing and its hold; a
// server-key count is trusted, overage included; no counter goes negative.
func TestSettleClampsAndFloors(t *testing.T) {
	ResetBudgetForTest(``)
	c := Charge{Dim: DimCompanionStranger, UserId: 2, Limit: 10000}
	relay, _ := budget.reserve(ConsumerCompanion, 400, 0, 0, false, []Charge{c})
	budget.settle(relay, 5000, false)
	if Allowance(DimCompanionStranger, 2) != 400 {
		t.Fatalf("a relayed count never charges past its hold: %d", Allowance(DimCompanionStranger, 2))
	}
	relay2, _ := budget.reserve(ConsumerCompanion, 400, 0, 0, false, []Charge{c})
	budget.settle(relay2, -900, false)
	if Allowance(DimCompanionStranger, 2) != 400 {
		t.Fatalf("nor below nothing: %d", Allowance(DimCompanionStranger, 2))
	}
	if u := Today(); u.Tokens != 0 || u.Outstanding != 0 {
		t.Fatalf("a relayed settlement touches nothing of the server's: %+v", u)
	}

	owner := Charge{Dim: DimCompanionOwner, UserId: 5}
	h, _ := budget.reserve(ConsumerCompanion, 300, 0, 0, true, []Charge{owner})
	budget.settle(h, 450, false)
	share := 0
	for _, cu := range Today().ByConsumer {
		if cu.Consumer == ConsumerCompanion {
			share = cu.Tokens
		}
	}
	if Today().Tokens != 450 || share != 450 || Allowance(DimCompanionOwner, 5) != 450 || Today().Outstanding != 0 {
		t.Fatalf("server-key overage is charged everywhere: total=%d share=%d owner=%d", Today().Tokens, share, Allowance(DimCompanionOwner, 5))
	}

	// Both counters set below the hold, so the refund would take them
	// negative: the allowance and the server total (budget.go:186) floor.
	h2, _ := budget.reserve(ConsumerCompanion, 300, 0, 0, true, []Charge{owner})
	shared.SetAllowanceForTest(DimCompanionOwner, 5, 100)
	SetSpentForTest(100, 300)
	budget.settle(h2, 0, false)
	if Allowance(DimCompanionOwner, 5) != 0 {
		t.Fatalf("an allowance floors at nothing: %d", Allowance(DimCompanionOwner, 5))
	}
	if u := Today(); u.Tokens != 0 || u.Outstanding != 0 {
		t.Fatalf("the server total floors at nothing: %+v", u)
	}
}

// R17, R21: a hold made yesterday gives nothing back to today's
// allowances, and its overage is still charged.
func TestAHoldFromYesterdayRefundsNoAllowance(t *testing.T) {
	ResetBudgetForTest(``)
	day1 := time.Date(2026, 9, 26, 23, 59, 0, 0, time.UTC)
	SetClockForTest(func() time.Time { return day1 })
	t.Cleanup(func() { SetClockForTest(time.Now) })
	owner := Charge{Dim: DimCompanionOwner, UserId: 5}
	stranger := Charge{Dim: DimCompanionStranger, UserId: 2}
	hs, _ := budget.reserve(ConsumerCompanion, 900, 0, 0, true, []Charge{owner})
	hs2, _ := budget.reserve(ConsumerCompanion, 100, 0, 0, true, []Charge{owner})
	hr, _ := budget.reserve(ConsumerCompanion, 400, 0, 0, false, []Charge{stranger})

	SetClockForTest(func() time.Time { return day1.Add(2 * time.Minute) })
	if Allowance(DimCompanionOwner, 5) != 0 || Allowance(DimCompanionStranger, 2) != 0 {
		t.Fatal("a new day's allowances start at nothing")
	}
	shared.SetAllowanceForTest(DimCompanionOwner, 5, 500)
	shared.SetAllowanceForTest(DimCompanionStranger, 2, 300)
	budget.settle(hs, 100, false)
	budget.settle(hs2, 250, false)
	budget.settle(hr, 0, false)
	if Allowance(DimCompanionOwner, 5) != 650 {
		t.Fatalf("no refund from yesterday, overage still charged: %d", Allowance(DimCompanionOwner, 5))
	}
	if Allowance(DimCompanionStranger, 2) != 300 {
		t.Fatalf("the relayed hold gives nothing back: %d", Allowance(DimCompanionStranger, 2))
	}
	if u := Today(); u.Outstanding != 0 || u.Tokens != 350 {
		t.Fatalf("the server settles as before: %+v", u)
	}
}

// A consumer holds at most its share of the day's budget; with no global
// cap there is no share cap; a player's own key is never held to one.
func TestAConsumerIsHeldToItsShare(t *testing.T) {
	ResetBudgetForTest(``)
	restore := SetServerForTest(ServerSettings{Endpoint: Endpoint{BaseURL: DefaultBaseURL},
		DailyTokenBudget: 1000, BaublesSharePercent: 25, BreakerErrors: 2, BreakerSeconds: 60})
	t.Cleanup(func() { restore(); ResetBudgetForTest(``) })

	h, err := Reserve(ConsumerBaubles, 200, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Reserve(ConsumerBaubles, 100, true); !errors.Is(err, ErrOverShare) || RefusedBy(err) != RefusedShare {
		t.Fatalf("over a 250-token share, and it says so: %v (%q)", err, RefusedBy(err))
	}
	if _, err := Reserve(ConsumerCompanion, 700, true); err != nil {
		t.Fatal("a consumer with no share cap spends the rest")
	}
	Settle(h, 50, false)
	if _, err := Reserve(ConsumerBaubles, 200, true); err != nil {
		t.Fatal("settling frees share")
	}
	if _, err := Reserve(ConsumerBaubles, 5000, false); err != nil {
		t.Fatal("a player's own key is held to no share")
	}

	ResetBudgetForTest(``)
	noCap := SetServerForTest(ServerSettings{Endpoint: Endpoint{BaseURL: DefaultBaseURL},
		DailyTokenBudget: 0, BaublesSharePercent: 25, BreakerErrors: 2, BreakerSeconds: 60})
	defer noCap()
	if _, err := Reserve(ConsumerBaubles, 100000, true); err != nil {
		t.Fatal("no global cap is no share cap")
	}
}

func TestSharePercentByConsumer(t *testing.T) {
	s := ServerSettings{CompanionSharePercent: 60, BaublesSharePercent: 25}
	if s.SharePercent(ConsumerCompanion) != 60 || s.SharePercent(ConsumerBaubles) != 25 || s.SharePercent(`other`) != 0 {
		t.Fatal("each consumer's own share; an unknown one has none")
	}
}

// R38, R39: allowances and seed marks are living state with the rest of
// the day. A dimension is seeded once a day, today only; a same-day restart
// finds the mark and seeds nothing; a quarantine loses the counts and the
// marks together, so the next seed applies again.
func TestAllowancesSaveLoadAndSeedOnce(t *testing.T) {
	dir := t.TempDir()
	ResetBudgetForTest(dir)
	t.Cleanup(func() { ResetBudgetForTest(``) })
	h, err := Reserve(ConsumerBaubles, 500, true, Charge{Dim: DimBaublesFinder, UserId: 7, Limit: 20000})
	if err != nil {
		t.Fatal(err)
	}
	Settle(h, 123, false)
	day := Today().Day
	backup := map[int]int{5: 777, 8: 0}

	SeedAllowances(DimCompanionOwner, `1999-01-01`, map[int]int{6: 50})
	if Allowance(DimCompanionOwner, 6) != 0 {
		t.Fatal("a stale day seeds nothing, and leaves no mark")
	}
	SeedAllowances(DimCompanionOwner, day, backup)
	SeedAllowances(DimCompanionOwner, day, backup)
	if Allowance(DimCompanionOwner, 5) != 777 || Allowance(DimCompanionOwner, 8) != 0 {
		t.Fatalf("seeded once a day per dimension: %d", Allowance(DimCompanionOwner, 5))
	}
	if got := Allowances(DimCompanionOwner); !reflect.DeepEqual(got, map[int]int{5: 777}) {
		t.Fatalf("one dimension's spends, nothing spent left out: %v", got)
	}
	if got := Allowances(DimCompanionStranger); len(got) != 0 {
		t.Fatalf("a dimension is its own: %v", got)
	}
	SaveBudget()

	ResetBudgetForTest(dir) // a same-day restart
	if Allowance(DimBaublesFinder, 7) != 123 || Allowance(DimCompanionOwner, 5) != 777 {
		t.Fatalf("a restart keeps the day's allowances: finder=%d owner=%d", Allowance(DimBaublesFinder, 7), Allowance(DimCompanionOwner, 5))
	}
	SeedAllowances(DimCompanionOwner, day, backup)
	if Allowance(DimCompanionOwner, 5) != 777 {
		t.Fatalf("the mark is saved too: a same-day restart seeds nothing: %d", Allowance(DimCompanionOwner, 5))
	}
	raw, err := os.ReadFile(filepath.Join(dir, `budget.yaml`))
	if err != nil || !strings.Contains(string(raw), `by_user:`) || !strings.Contains(string(raw), `baubles.finder:7`) ||
		!strings.Contains(string(raw), `seeded:`) {
		t.Fatalf("by_user and seeded in budget.yaml: %s", raw)
	}

	if err := os.WriteFile(filepath.Join(dir, `budget.yaml`), []byte("day: [unclosed"), 0644); err != nil {
		t.Fatal(err)
	}
	ResetBudgetForTest(dir) // a boot that finds the file corrupt
	if Allowance(DimBaublesFinder, 7) != 0 || Allowance(DimCompanionOwner, 5) != 0 {
		t.Fatal("a quarantine restarts the day's allowances with its totals")
	}
	SeedAllowances(DimCompanionOwner, day, backup)
	if Allowance(DimCompanionOwner, 5) != 777 {
		t.Fatalf("the marks went with the counts, so the backup seeds again: %d", Allowance(DimCompanionOwner, 5))
	}
}

// R38: SaveBudget marshals copies, never the live maps. Only -race can see
// the shared map; run it in CI or the Docker test image (see the gate). The
// key space is bounded (ten users per goroutine, and one seed dimension per
// goroutine per save), so the maps stay small and it runs in seconds under
// -race. The goroutines run until the saves are done.
//
// ResetBudgetForTest(dir) leaves the ledger unloaded, and SaveBudget returns
// at once while it is, so the ledger is loaded before any goroutine starts,
// and the saves begin only once the goroutines are writing and end before
// they stop: every save overlaps real writes to ByUser and Seeded.
func TestSaveBudgetCopiesEveryMapUnderTheLock(t *testing.T) {
	dir := t.TempDir()
	ResetBudgetForTest(dir)
	restore := SetServerForTest(ServerSettings{Endpoint: Endpoint{BaseURL: DefaultBaseURL}, BreakerErrors: 2, BreakerSeconds: 60})
	t.Cleanup(func() { restore(); ResetBudgetForTest(``) })
	day := Today().Day // loads the ledger
	stop := make(chan struct{})
	var progress, round atomic.Int64
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; ; i++ {
				// The work comes first, so goroutine 0 writes key 0 (read
				// back below) before it can see stop.
				c := Charge{Dim: DimBaublesFinder, UserId: g*10 + i%10}
				if h, err := Reserve(ConsumerBaubles, 1, true, c); err == nil {
					Settle(h, 1, false)
				}
				// A new dimension each save round, so Seeded and ByUser are
				// written again after every save, and the maps grow by only
				// four entries a round.
				SeedAllowances(fmt.Sprintf(`test.seed%d.%d`, g, round.Load()), day, map[int]int{1: 1})
				progress.Add(1)
				select {
				case <-stop:
					return
				default:
				}
			}
		}(g)
	}
	for progress.Load() < 20 {
		runtime.Gosched()
	}
	before := progress.Load()
	// At least 100 saves, and more until a write has landed between them:
	// on a fast machine the scheduler can let 100 saves finish before any
	// writer runs again, which says nothing about the code under test. The
	// time limit only keeps a real deadlock from hanging the suite.
	start := time.Now()
	for i := 0; i < 100 || (progress.Load() == before && time.Since(start) < 10*time.Second); i++ {
		SaveBudget()
		round.Add(1)
		if i >= 100 {
			runtime.Gosched()
		}
	}
	during := progress.Load() - before
	close(stop)
	wg.Wait()
	if during == 0 {
		t.Fatal("fixture: the saves overlapped no writes")
	}
	SaveBudget()
	want := Allowance(DimBaublesFinder, 0)
	ResetBudgetForTest(dir)
	if want < 1 || Allowance(DimBaublesFinder, 0) != want {
		t.Fatalf("what was saved reads back: saved %d, read %d", want, Allowance(DimBaublesFinder, 0))
	}
}

// SeedAllowances hands over only real spends: a count of 0 or less seeds
// nothing, so a bad backup cannot lower an allowance.
func TestSeedAllowancesSkipsNothingSpent(t *testing.T) {
	k := NewBooksForTest()
	k.SetAllowanceForTest(DimCompanionOwner, 9, 0)
	k.SeedAllowances(DimCompanionOwner, k.Day(), map[int]int{5: 300, 8: 0, 9: -50})
	if k.Allowance(DimCompanionOwner, 5) != 300 || k.Allowance(DimCompanionOwner, 9) != 0 {
		t.Fatalf("seeded 300, and nothing from -50: %d, %d", k.Allowance(DimCompanionOwner, 5), k.Allowance(DimCompanionOwner, 9))
	}
}

// Allowances leaves out a user whose counter holds nothing, even when the
// counter exists (charged, then settled back to 0).
func TestAllowancesOmitsNothingSpent(t *testing.T) {
	k := NewBooksForTest()
	h, err := k.Reserve(ConsumerCompanion, 400, false, Charge{Dim: DimCompanionStranger, UserId: 2})
	if err != nil {
		t.Fatal(err)
	}
	k.Settle(h, 0, false)
	k.SetAllowanceForTest(DimCompanionStranger, 3, 250)
	if got := k.Allowances(DimCompanionStranger); !reflect.DeepEqual(got, map[int]int{3: 250}) {
		t.Fatalf("the settled-to-nothing counter is left out: %v", got)
	}
}

// A share of -1 is no share cap (ServerSettings keeps "no cap" as 0, but a
// negative share reaching the ledger must not refuse everything).
func TestANegativeShareIsNoShareCap(t *testing.T) {
	ResetBudgetForTest(``)
	if _, err := budget.reserve(ConsumerBaubles, 900, 1000, -1, true, nil); err != nil {
		t.Fatalf("-1 is no share cap: %v", err)
	}
}

// A hold keeps its own copy of the charges: the caller's slice changing
// after Reserve changes nothing the hold will settle.
func TestAHoldCopiesItsCharges(t *testing.T) {
	ResetBudgetForTest(``)
	cs := []Charge{{Dim: DimCompanionStranger, UserId: 2, Limit: 1000}}
	h, err := Reserve(ConsumerCompanion, 400, false, cs...)
	if err != nil {
		t.Fatal(err)
	}
	cs[0].UserId = 99
	Settle(h, 100, false)
	if Allowance(DimCompanionStranger, 2) != 100 || Allowance(DimCompanionStranger, 99) != 0 {
		t.Fatalf("settled on user 2 only: 2=%d 99=%d", Allowance(DimCompanionStranger, 2), Allowance(DimCompanionStranger, 99))
	}
}

// A huge DailyTokenBudget does not overflow the share: 25% of the largest
// int is held exactly, and one token more is refused.
func TestAShareOfAHugeBudgetDoesNotOverflow(t *testing.T) {
	ResetBudgetForTest(``)
	limit := math.MaxInt
	share := limit/100*25 + limit%100*25/100
	if _, err := budget.reserve(ConsumerBaubles, share, limit, 25, true, nil); err != nil {
		t.Fatalf("the whole share: %v", err)
	}
	if _, err := budget.reserve(ConsumerBaubles, 1, limit, 25, true, nil); !errors.Is(err, ErrOverShare) {
		t.Fatalf("one token past it: %v", err)
	}
}

// Two charges on one allowance in one reservation are checked together:
// each would fit alone, the two do not.
func TestDuplicateChargesAreCheckedTogether(t *testing.T) {
	ResetBudgetForTest(``)
	c := Charge{Dim: DimCompanionStranger, UserId: 2, Limit: 1000}
	if _, err := budget.reserve(ConsumerCompanion, 600, 0, 0, true, []Charge{c, c}); !errors.Is(err, ErrOverAllowance) {
		t.Fatalf("1200 of a 1000 allowance: %v", err)
	}
	if Allowance(DimCompanionStranger, 2) != 0 {
		t.Fatalf("refused, so nothing held: %d", Allowance(DimCompanionStranger, 2))
	}
	if _, err := budget.reserve(ConsumerCompanion, 400, 0, 0, true, []Charge{c, c}); err != nil {
		t.Fatalf("800 fits: %v", err)
	}
	if Allowance(DimCompanionStranger, 2) != 800 {
		t.Fatalf("both charged: %d", Allowance(DimCompanionStranger, 2))
	}
}

// A positive share of a positive budget that rounds down to nothing is one
// token, not a cap that refuses every call.
func TestATinyShareIsAtLeastOneToken(t *testing.T) {
	ResetBudgetForTest(``)
	if _, err := budget.reserve(ConsumerBaubles, 1, 10, 5, true, nil); err != nil {
		t.Fatalf("5%% of 10 is at least one token: %v", err)
	}
	if _, err := budget.reserve(ConsumerBaubles, 1, 10, 5, true, nil); !errors.Is(err, ErrOverShare) {
		t.Fatalf("and no more: %v", err)
	}
}

func TestSharePercentsResolve(t *testing.T) {
	s := resolveServer(configs.APIFramework{}, legacyConfig{})
	if s.CompanionSharePercent != 0 || s.BaublesSharePercent != 25 {
		t.Fatalf("absent: the companion uncapped, baubles 25: %d/%d", s.CompanionSharePercent, s.BaublesSharePercent)
	}
	s = resolveServer(configs.APIFramework{CompanionSharePercent: 60, BaublesSharePercent: -1}, legacyConfig{})
	if s.CompanionSharePercent != 60 || s.BaublesSharePercent != 0 {
		t.Fatalf("set, and -1 is no cap: %d/%d", s.CompanionSharePercent, s.BaublesSharePercent)
	}
	if s := resolveServer(configs.APIFramework{BaublesSharePercent: 100}, legacyConfig{}); s.BaublesSharePercent != 0 {
		t.Fatal("100 is no cap")
	}
}

// The yaml tags decode: a tag on the wrong field is a silent no-op.
func TestShareKnobsDecode(t *testing.T) {
	var a configs.APIFramework
	if err := yaml.Unmarshal([]byte("CompanionSharePercent: 60\nBaublesSharePercent: 30\n"), &a); err != nil {
		t.Fatal(err)
	}
	if a.CompanionSharePercent != 60 || a.BaublesSharePercent != 30 {
		t.Fatalf("decoded: %+v", a)
	}
}

// The committed config.yaml, read by repo path as the shipped-config tests
// in internal/configs do (TestBaubleShippedConfigMatchesDefaults), resolves
// to the shipped shares: a knob in the wrong block or with a typo is a
// silent no-op otherwise. CI checks out the HEAD blob; locally, run it with
// the disk copy equal to HEAD (Task 11 Step 5).
func TestTheShippedShareKnobs(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(`..`, `..`, `_datafiles`, `config.yaml`))
	if err != nil {
		t.Fatalf("read shipped config: %v", err)
	}
	var shipped struct {
		APIFramework configs.APIFramework `yaml:"APIFramework"`
	}
	if err := yaml.Unmarshal(data, &shipped); err != nil {
		t.Fatalf("decode shipped config: %v", err)
	}
	if shipped.APIFramework.BaublesSharePercent != 25 || shipped.APIFramework.CompanionSharePercent != 0 {
		// The share knobs and the budget only: the struct also carries the
		// key, which no test prints.
		a := shipped.APIFramework
		t.Fatalf("shipped knobs: BaublesSharePercent=%d CompanionSharePercent=%d DailyTokenBudget=%d",
			a.BaublesSharePercent, a.CompanionSharePercent, a.DailyTokenBudget)
	}
	s := resolveServer(shipped.APIFramework, legacyConfig{})
	if s.BaublesSharePercent != 25 || s.CompanionSharePercent != 0 {
		t.Fatalf("shipped shares resolve to baubles 25, the companion uncapped: %d/%d", s.BaublesSharePercent, s.CompanionSharePercent)
	}
}

// A dimension with spending today seeds nothing, mark or no mark: its own
// counts already hold what a backup would add. Another dimension, spent in
// by no one, still seeds.
func TestSeedAllowancesRefusesASpentDimension(t *testing.T) {
	k := NewBooksForTest()
	h, err := k.Reserve(ConsumerCompanion, 400, true, Charge{Dim: DimCompanionOwner, UserId: 5})
	if err != nil {
		t.Fatal(err)
	}
	k.Settle(h, 400, false)
	k.SeedAllowances(DimCompanionOwner, k.Day(), map[int]int{5: 400, 6: 50})
	if k.Allowance(DimCompanionOwner, 5) != 400 || k.Allowance(DimCompanionOwner, 6) != 0 {
		t.Fatalf("a spent dimension seeds nothing: %d, %d", k.Allowance(DimCompanionOwner, 5), k.Allowance(DimCompanionOwner, 6))
	}
	k.SeedAllowances(DimCompanionStranger, k.Day(), map[int]int{2: 300})
	if k.Allowance(DimCompanionStranger, 2) != 300 {
		t.Fatalf("an unspent dimension still seeds: %d", k.Allowance(DimCompanionStranger, 2))
	}
}

// R9, on the ledger alone: its own lock makes check and hold one step, with
// no mud lock around the callers. Many goroutines reserving against one
// allowance at once hold, all told, never more than its cap, and exactly as
// many fit as the cap allows. Run it under -race too.
func TestConcurrentReservationsNeverPassAnAllowanceTogether(t *testing.T) {
	restore := SetServerForTest(ServerSettings{Endpoint: Endpoint{BaseURL: DefaultBaseURL}, BreakerErrors: 2, BreakerSeconds: 60})
	t.Cleanup(restore)
	k := NewBooksForTest()
	const limit, tokens, callers = 1000, 30, 200
	owner := Charge{Dim: DimCompanionOwner, UserId: 5, Limit: limit}
	start := make(chan struct{})
	var held atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, err := k.Reserve(ConsumerCompanion, tokens, true, owner); err == nil {
				held.Add(tokens)
			} else if RefusedBy(err) != DimCompanionOwner {
				t.Errorf("refused by the owner's allowance, not %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if got := k.Allowance(DimCompanionOwner, 5); got > limit || int64(got) != held.Load() || got != limit/tokens*tokens {
		t.Fatalf("held %d on the allowance (%d granted), cap %d, want %d", got, held.Load(), limit, limit/tokens*tokens)
	}
}
