package apiframework

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/util"
	"gopkg.in/yaml.v3"
)

// ONE daily token budget for the server's key (APIFramework.DailyTokenBudget),
// whatever feature spends it. Each call reserves its worst case before it is
// made (Reserve), so several callers cannot all pass the check at once and
// overspend together, and settles to what it really used afterwards
// (Settle). Spending is also kept per consumer ("companion", "baubles") for
// the admin views.
//
// The day is the UTC date. Calls still in flight at the rollover keep their
// reservation into the new day (it starts at what is still held), or
// settling them afterwards would credit back tokens the new day never
// spent.
//
// A player's own key (the companion key relay) is not the server's and is
// never reserved here.
//
// The day's spending is living state, saved to <DataFiles>/apiframework/
// budget.yaml (SaveBudget, from the modules' save hooks and at shutdown) and
// read on first use, so a restart does not hand out a fresh budget.

// ErrOverBudget is a reservation refused because the day's budget is spent.
var ErrOverBudget = errors.New(`daily token budget spent`)

// ErrOverAllowance is a reservation refused because one of its per-user
// allowances (a Charge) is spent.
var ErrOverAllowance = errors.New(`daily allowance spent`)

// ErrOverShare is a reservation refused because its consumer has held all
// of its share of the day's budget (ServerSettings.SharePercent).
var ErrOverShare = errors.New(`consumer's share of the daily token budget spent`)

// Per-user allowance dimensions. Each is one daily counter per user id,
// kept by the ledger beside the server's totals.
const (
	DimCompanionOwner        = `companion.owner`        // an owner's own companion calls
	DimCompanionStranger     = `companion.stranger`     // one passer-by, any companion
	DimCompanionStrangersFor = `companion.strangersfor` // all passers-by, one owner's companion
	DimBaublesFinder         = `baubles.finder`         // one finder's namings
	DimNPCIdleKeyholder      = `npcidle.keyholder`      // townsfolk idle moments on one player's key
	DimRoomLifeKeyholder     = `roomlife.keyholder`     // ambient room events on one player's key
	DimLookDetailKeyholder   = `lookdetail.keyholder`   // closer looks on one player's key
	DimRiftsKeyholder        = `rifts.keyholder`        // new rift rooms on one player's key
)

// Charge names one per-user daily allowance a reservation also counts
// against. Limit is that allowance's size, read by the caller from its own
// config at the moment of the call (the ledger cannot read a module's
// config); 0 or less is no cap, and the spend is still counted.
type Charge struct {
	Dim    string
	UserId int
	Limit  int
}

func allowanceKey(dim string, userId int) string { return dim + `:` + strconv.Itoa(userId) }

func (c Charge) key() string { return allowanceKey(c.Dim, c.UserId) }

// The counters a reservation can be refused by, besides a Charge's own
// dimension (RefusedBy).
const (
	RefusedGlobal = `global` // the day's budget (ErrOverBudget)
	RefusedShare  = `share`  // the consumer's share of it (ErrOverShare)
)

// RefusalError is a reservation Reserve refused, naming the counter that
// refused it: RefusedGlobal, RefusedShare, or a Charge's Dim. errors.Is
// still matches ErrOverBudget, ErrOverShare and ErrOverAllowance.
type RefusalError struct {
	Counter string
	err     error
}

func (e *RefusalError) Error() string { return e.err.Error() + ` (` + e.Counter + `)` }
func (e *RefusalError) Unwrap() error { return e.err }

func refused(counter string, err error) error { return &RefusalError{Counter: counter, err: err} }

// RefusedBy is the counter that refused err's reservation, or "" when err
// is not a refusal (nil, or any other failure).
func RefusedBy(err error) string {
	var r *RefusalError
	if errors.As(err, &r) {
		return r.Counter
	}
	return ``
}

// Consumer names who spends: the budget reports spending per consumer.
const (
	ConsumerCompanion  = `companion`
	ConsumerBaubles    = `baubles`
	ConsumerNPCIdle    = `npcidle`
	ConsumerRoomLife   = `roomlife`
	ConsumerLookDetail = `lookdetail`
	ConsumerRifts      = `rifts`
)

// Hold is one call's reservation, returned by Reserve and given back to
// Settle. It records every counter it touched: the server's (SpendServer)
// and each Charge.
type Hold struct {
	Consumer    string
	Tokens      int
	Day         string
	SpendServer bool
	Charges     []Charge
}

type ledgerState struct {
	Day        string         `yaml:"day"`
	Tokens     int            `yaml:"tokens"`
	Calls      int            `yaml:"calls"`
	Failures   int            `yaml:"failures,omitempty"`
	ByConsumer map[string]int `yaml:"by_consumer,omitempty"`
	CallsBy    map[string]int `yaml:"calls_by,omitempty"`
	// ByUser is each per-user allowance's spend today, keyed
	// "<dim>:<userId>".
	ByUser map[string]int `yaml:"by_user,omitempty"`
	// Seeded marks each dimension SeedAllowances has already seeded today,
	// one mark per dimension (a new day starts with none).
	Seeded map[string]bool `yaml:"seeded,omitempty"`
}

type ledger struct {
	mu          sync.Mutex
	st          ledgerState
	outstanding int
	loaded      bool
	dirty       bool
	dir         string // "" = <DataFiles>/apiframework
	now         func() time.Time
}

var budget = &ledger{now: time.Now}

func today(t time.Time) string { return t.UTC().Format(`2006-01-02`) }

// rollLocked starts a new day at the UTC boundary. Caller holds mu.
func (l *ledger) rollLocked() {
	if d := today(l.now()); d != l.st.Day {
		l.st = ledgerState{Day: d, Tokens: l.outstanding}
		l.dirty = true
	}
	if l.st.ByConsumer == nil {
		l.st.ByConsumer = map[string]int{}
	}
	if l.st.CallsBy == nil {
		l.st.CallsBy = map[string]int{}
	}
	if l.st.ByUser == nil {
		l.st.ByUser = map[string]int{}
	}
	if l.st.Seeded == nil {
		l.st.Seeded = map[string]bool{}
	}
}

func (l *ledger) budgetDir() string {
	if l.dir != `` {
		return l.dir
	}
	return util.FilePath(configs.GetFilePathsConfig().DataFiles.String(), `/`, `apiframework`)
}

func (l *ledger) path() string { return util.FilePath(l.budgetDir(), `/`, `budget.yaml`) }

// loadLocked reads today's saved spending once. A stale day is simply a new
// day; a corrupt file is quarantined and the day starts from nothing.
// Caller holds mu.
func (l *ledger) loadLocked() {
	if l.loaded {
		return
	}
	l.loaded = true
	raw, err := util.ReadLivingState(l.path())
	if err != nil {
		if errors.Is(err, util.ErrStateCorrupt) {
			l.quarantine(err)
		}
		return
	}
	var st ledgerState
	if err := yaml.Unmarshal(raw, &st); err != nil {
		l.quarantine(err)
		return
	}
	if st.Day == today(l.now()) {
		l.st = st
	}
}

// quarantine moves a corrupt budget file aside and logs it; the day starts
// from nothing (the living-state contract).
func (l *ledger) quarantine(cause error) {
	moved, err := util.QuarantineCorrupt(l.path())
	mudlog.Error(`apiframework`, `action`, `loadBudget`, `error`, cause, `quarantinedTo`, moved, `quarantineError`, err)
}

// Reserve holds tokens for consumer, all or nothing, under the ledger's one
// lock: when spendServer, against today's server budget
// (Server().DailyTokenBudget); and against every per-user allowance in
// charges. spendServer false is a player's own key: its allowances are
// charged and nothing of the server's is. It refuses with a *RefusalError
// wrapping ErrOverBudget, ErrOverShare or ErrOverAllowance and naming the
// counter (RefusedBy), holding nothing.
func Reserve(consumer string, tokens int, spendServer bool, charges ...Charge) (Hold, error) {
	return shared.Reserve(consumer, tokens, spendServer, charges...)
}

// Reserve on these books.
func (k *Books) Reserve(consumer string, tokens int, spendServer bool, charges ...Charge) (Hold, error) {
	s := Server()
	return k.l.reserve(consumer, tokens, s.DailyTokenBudget, s.SharePercent(consumer), spendServer, charges)
}

// reserve checks everything before it adds anything, so a refusal holds
// nothing anywhere. sharePct caps consumer's part of limit (0 or less, or
// 100 and above, is no share cap).
func (l *ledger) reserve(consumer string, tokens int, limit int, sharePct int, spendServer bool, charges []Charge) (Hold, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.loadLocked()
	l.rollLocked()
	if tokens < 0 {
		tokens = 0
	}
	if spendServer && limit > 0 && l.st.Tokens+tokens > limit {
		return Hold{}, refused(RefusedGlobal, ErrOverBudget)
	}
	if spendServer && limit > 0 && sharePct > 0 && sharePct < 100 &&
		l.st.ByConsumer[consumer]+tokens > shareOf(limit, sharePct) {
		return Hold{}, refused(RefusedShare, ErrOverShare)
	}
	// Two charges on one allowance in one reservation each add tokens, so
	// each is checked against what all of them add together.
	adds := make(map[string]int, len(charges))
	for _, c := range charges {
		adds[c.key()] += tokens
	}
	for _, c := range charges {
		if c.Limit > 0 && l.st.ByUser[c.key()]+adds[c.key()] > c.Limit {
			return Hold{}, refused(c.Dim, ErrOverAllowance)
		}
	}
	if spendServer {
		l.st.Tokens += tokens
		l.st.ByConsumer[consumer] += tokens
		l.st.Calls++
		l.st.CallsBy[consumer]++
		l.outstanding += tokens
	}
	for _, c := range charges {
		l.st.ByUser[c.key()] += tokens
	}
	l.dirty = true
	return Hold{Consumer: consumer, Tokens: tokens, Day: l.st.Day, SpendServer: spendServer,
		Charges: append([]Charge(nil), charges...)}, nil
}

// shareOf is pct percent of limit (both above 0), rounded down but never
// below one token, so a small share of a small budget still admits a call.
// It is worked out without multiplying limit, which a huge DailyTokenBudget
// would overflow.
func shareOf(limit int, pct int) int {
	share := limit/100*pct + limit%100*pct/100
	if share < 1 {
		share = 1
	}
	return share
}

// Settle replaces a reservation with what the call really used (use
// Charged to work that out), on every counter the hold touched. used below
// 0 is 0; on a player's own key (SpendServer false) it is at most the hold.
// failed counts a failed server-key call in the day's figures. A hold from
// an earlier day settles the server's total against today, since today
// started at what was still held, and gives no refund to a consumer share
// or an allowance.
func Settle(h Hold, used int, failed bool) {
	shared.Settle(h, used, failed)
}

// Settle on these books.
func (k *Books) Settle(h Hold, used int, failed bool) {
	k.l.settle(h, used, failed)
}

func (l *ledger) settle(h Hold, used int, failed bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.loadLocked()
	l.rollLocked()
	if used < 0 {
		used = 0
	}
	// A count relayed through a player's browser, which that player can
	// write, may lower a charge below its hold, never raise it past it. The
	// provider's own count on the server's key is trusted: usage past the
	// hold is charged.
	if !h.SpendServer && used > h.Tokens {
		used = h.Tokens
	}
	diff := used - h.Tokens
	earlier := h.Day != l.st.Day
	if h.SpendServer {
		l.outstanding -= h.Tokens
		if l.outstanding < 0 {
			l.outstanding = 0
		}
		l.st.Tokens += diff
		if l.st.Tokens < 0 {
			l.st.Tokens = 0
		}
		// What a consumer is shown is what it spent today; a hold from an
		// earlier day gives nothing back to today's share.
		share := diff
		if earlier && share < 0 {
			share = 0
		}
		l.st.ByConsumer[h.Consumer] += share
		if l.st.ByConsumer[h.Consumer] < 0 {
			l.st.ByConsumer[h.Consumer] = 0
		}
		if failed {
			l.st.Failures++
		}
	}
	// Each allowance started the new day at nothing, so a hold from an
	// earlier day gives it nothing back; usage past the hold is still
	// charged.
	each := diff
	if earlier && each < 0 {
		each = 0
	}
	for _, c := range h.Charges {
		k := c.key()
		l.st.ByUser[k] += each
		if l.st.ByUser[k] < 0 {
			l.st.ByUser[k] = 0
		}
	}
	l.dirty = true
}

// HasRoom reports whether the day's budget is not yet spent.
func HasRoom() bool {
	return shared.HasRoom()
}

// HasRoom on these books.
func (k *Books) HasRoom() bool {
	limit := Server().DailyTokenBudget
	k.l.mu.Lock()
	defer k.l.mu.Unlock()
	k.l.loadLocked()
	k.l.rollLocked()
	return limit <= 0 || k.l.st.Tokens < limit
}

// Allowance is one per-user allowance's spend today (Charge.Dim and
// UserId), for read-only checks and the admin views.
func Allowance(dim string, userId int) int {
	return shared.Allowance(dim, userId)
}

// Allowance on these books.
func (k *Books) Allowance(dim string, userId int) int {
	k.l.mu.Lock()
	defer k.l.mu.Unlock()
	k.l.loadLocked()
	k.l.rollLocked()
	return k.l.st.ByUser[allowanceKey(dim, userId)]
}

// Day is the ledger's day: the UTC date on its clock. Every daily count a
// feature keeps rolls on this, so no feature keeps a clock of its own.
func (k *Books) Day() string {
	k.l.mu.Lock()
	defer k.l.mu.Unlock()
	k.l.loadLocked()
	k.l.rollLocked()
	return k.l.st.Day
}

// Usage is the day's spending, for the admin views.
type Usage struct {
	Day         string
	Tokens      int // spent and still held, all consumers
	Outstanding int // held by calls still in flight
	Calls       int
	Failures    int
	Limit       int
	ByConsumer  []ConsumerUsage // most tokens first
}

// ConsumerUsage is one consumer's share of the day.
type ConsumerUsage struct {
	Consumer string
	Tokens   int
	Calls    int
}

// Today is the day's spending.
func Today() Usage {
	return shared.Today()
}

// Today on these books.
func (k *Books) Today() Usage {
	limit := Server().DailyTokenBudget
	k.l.mu.Lock()
	defer k.l.mu.Unlock()
	k.l.loadLocked()
	k.l.rollLocked()
	u := Usage{Day: k.l.st.Day, Tokens: k.l.st.Tokens, Outstanding: k.l.outstanding,
		Calls: k.l.st.Calls, Failures: k.l.st.Failures, Limit: limit}
	for c, n := range k.l.st.ByConsumer {
		u.ByConsumer = append(u.ByConsumer, ConsumerUsage{Consumer: c, Tokens: n, Calls: k.l.st.CallsBy[c]})
	}
	sort.Slice(u.ByConsumer, func(a, b int) bool {
		if u.ByConsumer[a].Tokens != u.ByConsumer[b].Tokens {
			return u.ByConsumer[a].Tokens > u.ByConsumer[b].Tokens
		}
		return u.ByConsumer[a].Consumer < u.ByConsumer[b].Consumer
	})
	return u
}

// SeedTokens adds tokens a consumer spent today before the framework kept
// the books (the AI companion's own saved day, on the first boot after the
// move), so moving does not hand out a second budget. It only ever applies
// once per day and only to a fresh day.
func SeedTokens(consumer string, day string, tokens int) {
	shared.SeedTokens(consumer, day, tokens)
}

// SeedTokens on these books.
func (k *Books) SeedTokens(consumer string, day string, tokens int) {
	k.l.mu.Lock()
	defer k.l.mu.Unlock()
	k.l.loadLocked()
	k.l.rollLocked()
	if tokens <= 0 || day != k.l.st.Day || k.l.st.Tokens != k.l.outstanding || k.l.st.ByConsumer[consumer] != 0 {
		return
	}
	k.l.st.Tokens += tokens
	k.l.st.ByConsumer[consumer] += tokens
	k.l.dirty = true
}

// SeedAllowances hands the ledger one dimension's per-user spends from
// today, kept by a feature before the ledger kept them or as its own backup
// (the AI companion's saved day), so neither the move to the ledger nor a
// quarantined budget.yaml hands out a second allowance. It applies once per
// dimension per day: the mark is saved with the day, so a normal restart
// seeds nothing, and a quarantine, which loses the marks with the counts,
// lets the next boot seed again. A dimension that already has spending
// today seeds nothing either, marked or not (as SeedTokens refuses a
// consumer that has spent): a day that began with a rollover, or on a boot
// that seeded nothing, has no marks, and its own counts already hold what
// the backup would add. A stale day seeds nothing.
func SeedAllowances(dim string, day string, spent map[int]int) {
	shared.SeedAllowances(dim, day, spent)
}

// SeedAllowances on these books.
func (k *Books) SeedAllowances(dim string, day string, spent map[int]int) {
	k.l.mu.Lock()
	defer k.l.mu.Unlock()
	k.l.loadLocked()
	k.l.rollLocked()
	if day != k.l.st.Day || k.l.st.Seeded[dim] || k.l.spentInLocked(dim) {
		return
	}
	for userId, tokens := range spent {
		if tokens > 0 {
			k.l.st.ByUser[allowanceKey(dim, userId)] += tokens
		}
	}
	k.l.st.Seeded[dim] = true
	k.l.dirty = true
}

// spentInLocked reports whether anyone has spent anything today in dim.
// Caller holds mu.
func (l *ledger) spentInLocked(dim string) bool {
	prefix := dim + `:`
	for key, tokens := range l.st.ByUser {
		if tokens > 0 && strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// Allowances is every user's spend today in one dimension, by user id: a
// copy, with nothing-spent users left out. A feature that keeps its own
// backup of its allowances (the AI companion) writes it from this.
func Allowances(dim string) map[int]int {
	return shared.Allowances(dim)
}

// Allowances on these books.
func (k *Books) Allowances(dim string) map[int]int {
	k.l.mu.Lock()
	defer k.l.mu.Unlock()
	k.l.loadLocked()
	k.l.rollLocked()
	prefix := dim + `:`
	out := map[int]int{}
	for key, tokens := range k.l.st.ByUser {
		if tokens <= 0 || !strings.HasPrefix(key, prefix) {
			continue
		}
		if userId, err := strconv.Atoi(key[len(prefix):]); err == nil {
			out[userId] = tokens
		}
	}
	return out
}

// SaveBudget writes the day's spending when it changed. Safe to call often.
func SaveBudget() {
	budget.mu.Lock()
	if !budget.loaded || !budget.dirty {
		budget.mu.Unlock()
		return
	}
	st := budget.st
	byC := make(map[string]int, len(st.ByConsumer))
	for k, v := range st.ByConsumer {
		byC[k] = v
	}
	callsBy := make(map[string]int, len(st.CallsBy))
	for k, v := range st.CallsBy {
		callsBy[k] = v
	}
	byUser := make(map[string]int, len(st.ByUser))
	for k, v := range st.ByUser {
		byUser[k] = v
	}
	seeded := make(map[string]bool, len(st.Seeded))
	for k, v := range st.Seeded {
		seeded[k] = v
	}
	st.ByConsumer, st.CallsBy, st.ByUser, st.Seeded = byC, callsBy, byUser, seeded
	path := budget.path()
	budget.dirty = false
	budget.mu.Unlock()

	raw, err := yaml.Marshal(&st)
	if err == nil {
		err = os.MkdirAll(filepath.Dir(path), 0755)
	}
	if err == nil {
		err = util.Save(path, raw)
	}
	if err != nil {
		mudlog.Error(`apiframework`, `action`, `saveBudget`, `error`, err)
		budget.mu.Lock()
		budget.dirty = true
		budget.mu.Unlock()
	}
}

// ResetBudgetForTest starts a fresh, unsaved day in dir ("" keeps nothing
// on disk: the ledger is marked loaded and never written unless SaveBudget
// is called).
func ResetBudgetForTest(dir string) {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	budget.st = ledgerState{}
	budget.outstanding = 0
	budget.loaded = dir == ``
	budget.dirty = false
	budget.dir = dir
	budget.now = time.Now
}

// SetSpentForTest sets today's total spending and in-flight holds.
func SetSpentForTest(tokens int, outstanding int) {
	shared.SetSpentForTest(tokens, outstanding)
}

// SetSpentForTest on these books.
func (k *Books) SetSpentForTest(tokens int, outstanding int) {
	k.l.mu.Lock()
	defer k.l.mu.Unlock()
	k.l.loadLocked()
	k.l.rollLocked()
	k.l.st.Tokens = tokens
	k.l.outstanding = outstanding
}

// SetAllowanceForTest sets one per-user allowance's spend today.
func (k *Books) SetAllowanceForTest(dim string, userId int, tokens int) {
	k.l.mu.Lock()
	defer k.l.mu.Unlock()
	k.l.loadLocked()
	k.l.rollLocked()
	k.l.st.ByUser[allowanceKey(dim, userId)] = tokens
}

// SetClockForTest replaces the ledger's clock.
func SetClockForTest(now func() time.Time) {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	budget.now = now
}
