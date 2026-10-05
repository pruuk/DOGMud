package aicompanion

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Model tiers (phase 7). Different moments need different models:
//
//   - fast: the moment needs an answer now more than a fine one: a combat
//     plan, noticing something, a quiet moment, a follow-up after a look,
//     a trip update, a finished goal. A small, low-latency model.
//   - main: conversation and relationship: someone speaks, asks, emotes,
//     gives, hurts or heals; greetings and goodbyes. The model that makes
//     the companion feel like a person.
//   - deep: the private reflection after a session. Nobody is waiting, so
//     the strongest model is worth its time.
//
// Every tier falls back to Model when its own model is not configured.

const (
	tierFast = `fast`
	tierMain = `main`
	tierDeep = `deep`
)

// tierOfKind is the tier each stimulus needs on its own.
var tierOfKind = map[string]string{
	`heard`:         tierMain,
	`asked`:         tierMain,
	`emote`:         tierMain,
	`gift`:          tierMain,
	`attacked`:      tierMain,
	`healed`:        tierMain,
	`first_meeting`: tierMain,
	`session_start`: tierMain,
	// A goodbye has about twenty seconds of logout meditation to arrive in,
	// so it goes to the quick model rather than the considered one.
	`farewell`:    tierFast,
	`recovered`:   tierMain,
	`arrived`:     tierMain,
	`fight_over`:  tierMain,
	`romance`:     tierMain,
	`romance_yes`: tierMain,
	`romance_no`:  tierMain,
	`night`:       tierMain,
	`party`:       tierMain,
	`fight`:       tierFast,
	`noticed`:     tierFast,
	// A quiet moment is where she acts on her own purposes, so it gets the
	// full picture: goals, pack, prices and map. Noticing something does
	// not.
	`idle`:        tierMain,
	`quiet`:       tierFast,
	`looked`:      tierFast,
	`trip`:        tierFast,
	`goal_done`:   tierFast,
	`remembering`: tierMain,
	`witnessed`:   tierMain,
	`errand_ask`:  tierMain,
	`ailing`:      tierFast,
	`trouble`:     tierFast,
	`grew`:        tierFast,
	// What a search turned up is a remark, not a decision.
	`searched`: tierFast,
	`found`:    tierFast,
}

// tierFor picks the tier for a batch of stimuli. A fight always goes fast,
// because the fight will not wait; otherwise anything conversational makes
// the whole batch main.
func tierFor(stims []stimulus) string {
	tier := tierFast
	for _, s := range stims {
		if s.Kind == `fight` {
			return tierFast
		}
		if t, ok := tierOfKind[s.Kind]; !ok || t == tierMain {
			tier = tierMain
		}
	}
	return tier
}

// tierSettings is what a call uses for its tier.
type tierSettings struct {
	Model     string
	MaxTokens int
	Effort    string // reasoning effort, sent only when set
	Timeout   time.Duration
}

// settingsFor resolves a tier to a model and limits, falling back to the
// main model when a tier has none of its own.
func (c Config) settingsFor(tier string) tierSettings {
	s := tierSettings{
		Model:     c.Model,
		MaxTokens: c.MaxCompletionTokens,
		Effort:    c.MainReasoningEffort,
		Timeout:   time.Duration(c.RequestTimeoutSeconds) * time.Second,
	}
	switch tier {
	case tierFast:
		s.Effort = c.FastReasoningEffort
		if c.FastModel != `` {
			s.Model = c.FastModel
		}
		if c.FastMaxCompletionTokens > 0 {
			s.MaxTokens = c.FastMaxCompletionTokens
		}
		if c.FastTimeoutSeconds > 0 {
			s.Timeout = time.Duration(c.FastTimeoutSeconds) * time.Second
		}
	case tierDeep:
		s.Effort = c.DeepReasoningEffort
		if c.DeepModel != `` {
			s.Model = c.DeepModel
		}
		if c.DeepMaxCompletionTokens > 0 {
			s.MaxTokens = c.DeepMaxCompletionTokens
		}
		s.Timeout = 2 * s.Timeout
	}
	return s
}

// tierStats is running metrics per tier (F19.4).
type tierStats struct {
	Calls      int
	Errors     int
	Tokens     int
	LatencySum time.Duration
}

func (m *AICompanionModule) recordCall(tier string, res modelResult) {
	if errors.Is(res.Err, errServerResting) {
		return // held back for a breaker's probe: no call was made
	}
	if m.stats == nil {
		m.stats = map[string]*tierStats{}
	}
	st, ok := m.stats[tier]
	if !ok {
		st = &tierStats{}
		m.stats[tier] = st
	}
	st.Calls++
	st.Tokens += res.Tokens
	st.LatencySum += res.Latency
	if res.Err != nil && !res.Canceled {
		st.Errors++
	}
}

func (m *AICompanionModule) statsLines() []string {
	tiers := make([]string, 0, len(m.stats))
	for t := range m.stats {
		tiers = append(tiers, t)
	}
	sort.Strings(tiers)
	var out []string
	for _, t := range tiers {
		st := m.stats[t]
		avg := time.Duration(0)
		if st.Calls > 0 {
			avg = st.LatencySum / time.Duration(st.Calls)
		}
		out = append(out, fmt.Sprintf(`  %s: calls=%d errors=%d tokens=%d avgLatency=%s`, t, st.Calls, st.Errors, st.Tokens, avg.Round(time.Millisecond)))
	}
	return out
}

// Circuit breakers on the server's key (apiframework): the companion's own,
// fed by every failure of its calls exactly as its breaker always was, and
// the provider's, shared with every feature and fed only by failures that
// say the provider or key is unwell (so a bauble model the provider refuses
// never pauses her). APIFramework.BreakerErrors failures in a row stop calls
// for BreakerSeconds; then exactly one call is let through as a probe, and
// its success closes the breaker again. The companion runs on fallback lines
// while either is open. A player's own key has its own breaker (relayTable).

func (m *AICompanionModule) breakerOpen(now time.Time) bool {
	return m.fw().Blocked(apiframework.ConsumerCompanion, now)
}

func (m *AICompanionModule) breakerResult(t apiframework.Ticket, err error, now time.Time) {
	if errors.Is(err, errNoConsent) || errors.Is(err, apiframework.ErrNotAdmitted) ||
		errors.Is(err, errServerResting) || errors.Is(err, context.Canceled) {
		// A request that never left the server (the door refused it, or the
		// breaker was resting), or one given up on: not the provider
		// failing, and must not pause every other companion. Its leave is
		// handed back unjudged.
		m.fw().Release(apiframework.ConsumerCompanion, t)
		return
	}
	m.fw().Record(apiframework.ConsumerCompanion, t, err, now)
}

// ownerBudgetLeft reports whether one companion has any daily tokens left
// at all (the ledger's DimCompanionOwner). Admission of a particular call
// goes through reserveRoute, which weighs that call's worst case.
func (m *AICompanionModule) ownerBudgetLeft(ownerId int) bool {
	if m.cfg.DailyTokensPerCompanion <= 0 {
		return true
	}
	return m.fw().Allowance(apiframework.DimCompanionOwner, ownerId) < m.cfg.DailyTokensPerCompanion
}

// traceEntry is one decision kept for the admin trace view.
type traceEntry struct {
	Unix     int64
	Tier     string
	Model    string
	Triggers string
	Intent   string
	Lines    int
	Action   string
	Tokens   int
	Latency  time.Duration
	Tools    int
	Err      string
}

func (c *controller) addTrace(t traceEntry) {
	c.traces = append(c.traces, t)
	if len(c.traces) > 12 {
		c.traces = append([]traceEntry(nil), c.traces[len(c.traces)-12:]...)
	}
}

func (t traceEntry) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, `%s %s/%s [%s]`, time.Unix(t.Unix, 0).Format(`15:04:05`), t.Tier, t.Model, t.Triggers)
	if t.Err != `` {
		fmt.Fprintf(&b, ` ERROR %s`, t.Err)
		return b.String()
	}
	fmt.Fprintf(&b, ` lines=%d action=%q questions=%d tokens=%d %s intent=%q`, t.Lines, t.Action, t.Tools, t.Tokens, t.Latency.Round(time.Millisecond), t.Intent)
	return b.String()
}

// Zero-configuration model choice. With no model configured, each tier
// picks the first model from its preference list that the API key can use,
// learned from the API's model list at start-up. If a model is refused at
// runtime (retired, or not available to the key), the tier moves on to the
// next one by itself. Setting Model, FastModel or DeepModel overrides this.
//
// The lists run newest first and end in long-lived fallbacks, so the module
// keeps working as OpenAI's line-up changes.
var modelPreferences = map[string][]string{
	tierFast: {`gpt-5.4-nano`, `gpt-5-nano`, `gpt-4.1-nano`, `gpt-4o-mini`},
	tierMain: {`gpt-5.4-mini`, `gpt-5-mini`, `gpt-4.1-mini`, `gpt-4o-mini`},
	tierDeep: {`gpt-5.5`, `gpt-5.4`, `gpt-5`, `gpt-4.1`, `gpt-4o`},
}

// autoEffort is the reasoning effort each tier asks for when none is
// configured: none where speed matters (and where function tools require
// it on current models), medium for the unhurried reflection.
var autoEffort = map[string]string{
	tierFast: `none`,
	tierMain: `none`,
	tierDeep: `medium`,
}

// effortFor adapts a wanted reasoning effort to what a model family
// accepts, or returns "" when the model takes no reasoning effort at all.
//
//   - gpt-5.1 and later (gpt-5.4, gpt-5.5, ...): none, low, medium, high,
//     xhigh. Function tools on Chat Completions need none, so a call that
//     offers tools always sends none.
//   - the original gpt-5 family: minimal, low, medium, high (none becomes
//     minimal, xhigh becomes high).
//   - o-series: low, medium, high.
//   - everything else (gpt-4.x, gpt-4o): not sent.
func effortFor(model string, want string, tools bool) string {
	m := strings.ToLower(model)
	switch {
	case strings.HasPrefix(m, `gpt-5.`):
		if tools || want == `` {
			return `none`
		}
		if want == `minimal` {
			return `none`
		}
		return want
	case m == `gpt-5` || strings.HasPrefix(m, `gpt-5-`):
		switch want {
		case ``, `none`:
			return `minimal`
		case `xhigh`:
			return `high`
		}
		return want
	case strings.HasPrefix(m, `o1`) || strings.HasPrefix(m, `o3`) || strings.HasPrefix(m, `o4`):
		switch want {
		case ``, `none`, `minimal`:
			return `low`
		case `xhigh`:
			return `high`
		}
		return want
	}
	return ``
}

// modelChooser holds the resolved model per tier and what has been refused.
type modelChooser struct {
	available map[string]bool // model ids the key can use, from /models; nil = unknown
	refused   map[string]bool // models the API refused at runtime
	chosen    map[string]string
}

// pick returns the first preferred model for a tier that is available (when
// known) and not refused. With nothing left it returns the last preference.
func (mc *modelChooser) pick(tier string) string {
	if mc.chosen == nil {
		mc.chosen = map[string]string{}
	}
	if m, ok := mc.chosen[tier]; ok && !mc.refused[m] {
		return m
	}
	prefs := modelPreferences[tier]
	for _, m := range prefs {
		if mc.refused[m] {
			continue
		}
		if mc.available != nil && !mc.available[m] {
			continue
		}
		mc.chosen[tier] = m
		return m
	}
	// Nothing known to work: try the preferences in order regardless of the
	// model list, skipping only what has been refused.
	for _, m := range prefs {
		if !mc.refused[m] {
			mc.chosen[tier] = m
			return m
		}
	}
	// Every model this tier knows has been refused. Saying so stops the
	// module hammering a model the API will not serve; an operator can set
	// one explicitly, and `aicompanion models` shows the empty tier.
	return ``
}

// refuse records that the API would not serve a model, so every tier using
// it moves on.
func (mc *modelChooser) refuse(model string) {
	if mc.refused == nil {
		mc.refused = map[string]bool{}
	}
	mc.refused[model] = true
	for tier, m := range mc.chosen {
		if m == model {
			delete(mc.chosen, tier)
		}
	}
}

// modelRefused reports whether an API error means "this model is not
// available", as opposed to a transient or request problem.
func modelRefused(r modelResult) bool {
	if r.Err == nil {
		return false
	}
	msg := strings.ToLower(r.Err.Error())
	if r.Status == http.StatusNotFound {
		return true
	}
	if r.Status == http.StatusBadRequest || r.Status == http.StatusForbidden {
		return strings.Contains(msg, `model_not_found`) ||
			strings.Contains(msg, `does not exist`) ||
			strings.Contains(msg, `do not have access`) ||
			strings.Contains(msg, `does not have access`) ||
			strings.Contains(msg, `unsupported model`)
	}
	return false
}

// settingsFor resolves a tier to the model, effort and limits a call uses:
// the configured model if one is set, otherwise the automatic choice.
// tools says whether the call offers function tools.
func (m *AICompanionModule) settingsFor(tier string, tools bool) tierSettings {
	s := m.cfg.settingsFor(tier)
	configured := m.cfg.Model
	switch tier {
	case tierFast:
		if m.cfg.FastModel != `` {
			configured = m.cfg.FastModel
		}
	case tierDeep:
		if m.cfg.DeepModel != `` {
			configured = m.cfg.DeepModel
		}
	}
	if configured != `` {
		s.Model = configured
	} else {
		s.Model = m.models.pick(tier)
	}
	want := s.Effort
	if want == `` {
		want = autoEffort[tier]
	}
	s.Effort = effortFor(s.Model, want, tools)
	return s
}

// probeModels asks the API which models the key can use, off the game loop,
// and stores the answer under the mud lock. A failure leaves the list
// unknown and the tiers simply try their preferences in order.
func (m *AICompanionModule) probeModels() {
	baseURL, key := m.baseURL(), m.apiKey()
	if key == `` {
		return
	}
	go func() {
		ids := listModels(baseURL, key)
		if ids == nil {
			return
		}
		util.LockMud()
		defer util.UnlockMud()
		m.models.available = ids
		m.models.chosen = nil
		mudlog.Info(`aicompanion`, `action`, `probeModels`, `models`, len(ids),
			`fast`, m.models.pick(tierFast), `main`, m.models.pick(tierMain), `deep`, m.models.pick(tierDeep))
	}()
}

// Budgets are held before a call, not after it: the worst case is charged
// up front so several companions cannot all pass the check at once and
// overspend together, and the reservation is settled once the real usage
// is known.

// estimateTokens is a rough count of a request's prompt, about four
// characters to the token (apiframework.EstimateTokens).
func estimateTokens(msgs []chatMessage) int {
	return apiframework.EstimateTokens(msgs)
}

// requestOverhead is the schema and the tool definitions, which are sent
// with every call and are not in the messages.
func requestOverhead(call modelCall) int {
	n := 0
	if call.Schema != nil {
		n += 400 // the decision schema, in round numbers
	}
	for _, t := range call.Tools {
		n += (len(t.Function.Name) + len(t.Function.Description) + 200) / 4
	}
	return n
}

// worstCaseTokens is everything one decision could spend: the prompt and a
// full completion for each round the model may ask the game something, and
// again if the call is retried.
func worstCaseTokens(prompt int, maxTokens int, toolRounds int, retry bool) int {
	// Every round of questions sends the whole conversation again, with the
	// answers so far added to it, so the prompt grows as it goes.
	total := 0
	grown := prompt
	for i := 0; i <= toolRounds; i++ {
		total += grown + maxTokens
		// The model's questions (at most a completion), and the game's
		// answers: as many as a reply may ask, each as long as an answer
		// may be, at a token a rune (estimateTokens' worst case for bytes)
		// plus a message's framing.
		grown += maxTokens + maxToolCallsPerReply*(maxToolAnswerRunes+8)
	}
	if retry {
		total *= 2
	}
	return total
}

// budgetState is the companion's own day on disk: its calls and "you
// notice" moments. Tokens is the companion's share of the server key's
// day, still written so a server rolled back to the code before the
// framework resumes the day where it was. Owners, Strangers and
// StrangersFor are the ledger's allowances (apiframework Allowances),
// written here as a backup: every boot hands them to SeedAllowances, which
// applies once per dimension per day, so they seed the first boot after
// the move and a boot after budget.yaml was quarantined, and nothing on a
// normal restart.
type budgetState struct {
	Day       string      `yaml:"day"`
	Tokens    int         `yaml:"tokens"`
	Calls     int         `yaml:"calls"`
	Owners    map[int]int `yaml:"owners,omitempty"`
	Strangers map[int]int `yaml:"strangers,omitempty"`
	// StrangersFor is what passers-by together spent of each owner's
	// companion (StrangerTokensPerOwner), by owner.
	StrangersFor map[int]int `yaml:"strangers_for,omitempty"`
	// Notices is each owner's "you notice" moments today (NoticeCallsPerDay).
	Notices map[int]int `yaml:"notices,omitempty"`
}

const budgetStateId = `budget-state`

func (m *AICompanionModule) loadBudget() {
	var st budgetState
	if err := m.plug.ReadIntoStruct(budgetStateId, &st); err != nil {
		return // nothing recorded yet, or unreadable: start the day fresh
	}
	m.restoreBudget(st)
}

// restoreBudget takes up a saved day, when it is today on the ledger's
// clock. Its server total and its allowances are handed to the ledger
// (SeedTokens, SeedAllowances), which takes them only when it has no day of
// its own to go on: the first boot after the move to the ledger, or a boot
// after budget.yaml was quarantined. On a normal restart the ledger's seed
// marks turn them away, so nothing is counted twice and a corrupt file
// hands out no second allowance.
func (m *AICompanionModule) restoreBudget(st budgetState) {
	if st.Day != m.fw().Day() {
		return // a stale day is simply a new day
	}
	m.countersDay = st.Day
	m.fw().SeedTokens(apiframework.ConsumerCompanion, st.Day, st.Tokens)
	m.fw().SeedAllowances(apiframework.DimCompanionOwner, st.Day, st.Owners)
	m.fw().SeedAllowances(apiframework.DimCompanionStranger, st.Day, st.Strangers)
	m.fw().SeedAllowances(apiframework.DimCompanionStrangersFor, st.Day, st.StrangersFor)
	m.callsToday = st.Calls
	m.noticesToday = st.Notices
	if m.noticesToday == nil {
		m.noticesToday = map[int]int{}
	}
}

func (m *AICompanionModule) saveBudget() {
	if !m.cfg.Enabled {
		return
	}
	apiframework.SaveBudget()
	st := m.budgetStateToSave()
	if err := m.plug.WriteStruct(budgetStateId, &st); err != nil {
		mudlog.Error(`aicompanion`, `action`, `saveBudget`, `error`, err)
	}
}

// budgetStateToSave is the companion's own day as saveBudget writes it,
// with the ledger's allowances copied in as the backup restoreBudget seeds
// from.
func (m *AICompanionModule) budgetStateToSave() budgetState {
	m.rollCounters()
	st := budgetState{Day: m.countersDay, Calls: m.callsToday, Notices: m.noticesToday,
		Owners:       m.fw().Allowances(apiframework.DimCompanionOwner),
		Strangers:    m.fw().Allowances(apiframework.DimCompanionStranger),
		StrangersFor: m.fw().Allowances(apiframework.DimCompanionStrangersFor)}
	if u := m.fw().Today(); u.Day == m.countersDay {
		for _, c := range u.ByConsumer {
			if c.Consumer == apiframework.ConsumerCompanion {
				st.Tokens = c.Tokens
			}
		}
	}
	return st
}
