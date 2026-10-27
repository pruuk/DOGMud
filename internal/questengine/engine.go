package questengine

import (
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/GoMudEngine/GoMud/internal/areas"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// indexedTrigger pairs a definition trigger with the engine-assigned identity
// the evaluator needs (visit tracking, tracing). Replaces the old pattern of
// mutating unexported fields ON the definition struct at register time —
// definitions are immutable shared data owned by internal/quests.
type indexedTrigger struct {
	def     *TriggerDef
	questId int
	trigId  string
}

// Engine holds quest definitions and a trigger index for fast lookup by event type.
type Engine struct {
	quests       map[int]*QuestDef
	triggerIndex map[string][]*indexedTrigger
	// Explore (distinct) triggers by area key and by quest id (#375 phase 3b
	// slice 4): CheckExplore reads the first, the step-start check the
	// second. They are in triggerIndex too, where they never match.
	exploreByArea  map[string][]*indexedTrigger
	exploreByQuest map[int][]*indexedTrigger
}

// NewEngine creates an empty quest engine.
func NewEngine() *Engine {
	return &Engine{
		quests:         make(map[int]*QuestDef),
		triggerIndex:   make(map[string][]*indexedTrigger),
		exploreByArea:  make(map[string][]*indexedTrigger),
		exploreByQuest: make(map[int][]*indexedTrigger),
	}
}

// RegisterQuest stores a quest definition and indexes its triggers by event type.
func (e *Engine) RegisterQuest(q *QuestDef) {
	e.quests[q.QuestId] = q
	for i := range q.Triggers {
		t := &q.Triggers[i]
		it := &indexedTrigger{
			def:     t,
			questId: q.QuestId,
			trigId:  fmt.Sprintf("q%d-t%d", q.QuestId, i),
		}
		e.triggerIndex[t.Event] = append(e.triggerIndex[t.Event], it)
		if t.Distinct != "" && t.Area != "" {
			e.exploreByArea[t.Area] = append(e.exploreByArea[t.Area], it)
			e.exploreByQuest[q.QuestId] = append(e.exploreByQuest[q.QuestId], it)
		}
	}
}

// GetQuest returns a quest definition by ID, or nil if not found.
func (e *Engine) GetQuest(questId int) *QuestDef {
	return e.quests[questId]
}

// AllQuests returns every registered quest, ordered by quest id. For audits and
// gates that need to walk the whole quest set, e.g. the marker-decision smoke
// gate.
//
// The order is guaranteed because callers print and diff these results; map
// iteration order made audit output shuffle between runs for no reason.
func (e *Engine) AllQuests() []*QuestDef {
	out := make([]*QuestDef, 0, len(e.quests))
	for _, q := range e.quests {
		out = append(out, q)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].QuestId < out[j].QuestId })
	return out
}

// Notify is the main entry point. It evaluates all triggers matching the given
// event type and returns a result indicating whether anything fired and whether
// an item should be consumed.
func (e *Engine) Notify(eventType string, details EventDetails, player PlayerState, ctx ActionContext) NotifyResult {
	return e.NotifyAll(eventType, []EventDetails{details}, player, ctx)
}

// NotifyAll raises one event for several details under ONE guard, so each
// trigger fires at most once however many of the details it matches: a
// command aimed at three mobs advances an untargeted step once, and a step
// that names one of them still fires on that one (#375 phase 3b slice 2).
func (e *Engine) NotifyAll(eventType string, details []EventDetails, player PlayerState, ctx ActionContext) NotifyResult {
	userId := 0
	if len(details) > 0 {
		userId = details[len(details)-1].UserId
	}
	run := beginGuarded()
	guard := NewEvalGuard(run.maxDepth)
	result := NotifyResult{}
	for _, d := range details {
		result.merge(e.evaluate(eventType, d, player, ctx, guard))
	}
	run.end(guard, eventType, userId)
	return result
}

// guardedRun is the limits and start time of one evaluation under one fresh
// EvalGuard: beginGuarded starts it, the caller makes the guard from
// maxDepth, and end logs a slow run and a depth-limit hit. It is shared by
// NotifyAll and CheckExplore, so every evaluation starts from the same
// limits. The guard is made by the caller, not returned or reached through
// a callback, so it stays on the caller's stack: room_enter runs on every
// step a player takes, and a heap guard costs an allocation each time.
type guardedRun struct {
	start    time.Time
	maxDepth int
	warnMs   int64
}

func beginGuarded() guardedRun {
	maxDepth, warnMs := configs.QuestGuardLimits()
	return guardedRun{
		start:    time.Now(),
		maxDepth: maxDepth,
		warnMs:   warnMs,
	}
}

func (r guardedRun) end(guard *EvalGuard, eventType string, userId int) {
	elapsed := time.Since(r.start).Milliseconds()
	if elapsed > r.warnMs {
		LogWarn("Notify(%s) for user %d took %dms (threshold %dms), trace: %v",
			eventType, userId, elapsed, r.warnMs, guard.GetTrace())
	}
	if guard.DepthExceeded() {
		LogError("depth limit %d exceeded for user %d event %s, trace: %v",
			r.maxDepth, userId, eventType, guard.GetTrace())
	}
}

// merge folds another result into r.
func (r *NotifyResult) merge(o NotifyResult) {
	r.Handled = r.Handled || o.Handled
	r.ConsumeItem = r.ConsumeItem || o.ConsumeItem
}

// evaluate is the internal recursive function that matches triggers and
// executes actions. It chains through quest_granted events when tokens are
// granted, and a quest_granted event is also a step start: the granted
// quest's explore triggers are checked against the map (#375 phase 3b
// slice 4).
func (e *Engine) evaluate(eventType string, details EventDetails, player PlayerState, ctx ActionContext, guard *EvalGuard) NotifyResult {
	if !guard.PushDepth() {
		return NotifyResult{}
	}
	defer guard.PopDepth()

	result := NotifyResult{}

	for _, t := range e.fieldMatches(eventType, details) {
		result.merge(e.fire(eventType, t, details, player, ctx, guard, nil))
	}

	if eventType == "quest_granted" && details.QuestToken != "" {
		result.merge(e.exploreAtStepStart(details, player, ctx, guard))
	}

	return result
}

// fire runs one field-matched trigger: its conditions, the visit mark, its
// count, then (when it fires) onFire, its actions, item consumption and the
// quest_granted chain of every token granted. It is the one firing path:
// evaluate and the explore checks both use it, so an explore step is
// guarded, marked and chained exactly as any other trigger.
func (e *Engine) fire(eventType string, t *indexedTrigger, details EventDetails, player PlayerState, ctx ActionContext, guard *EvalGuard, onFire func()) NotifyResult {
	result := NotifyResult{}

	// Condition evaluation
	if !EvalConditions(t.def.Conditions, player) {
		return result
	}

	// Visit tracking: "visited" means "fired in this chain". A trigger is
	// marked before its actions run, so it cannot re-enter itself, and it
	// fires at most once per Notify/NotifyAll. A trigger whose conditions
	// failed is not marked, so a later detail can still fire it.
	if !guard.MarkVisited(t.trigId) {
		return result
	}

	// count: N (#375 phase 3b slice 4). The match is counted once per
	// event (the trigger is already marked visited) and is a handled
	// event, but the actions wait for the Nth.
	if !countReached(t, ctx) {
		result.Handled = true
		return result
	}

	guard.AddToTrace(fmt.Sprintf("%s:%s", eventType, t.trigId))
	LogVerboseF(details.UserId, "trigger %s matched for event %s", t.trigId, eventType)

	if onFire != nil {
		onFire()
	}

	// Execute actions
	grantedTokens := e.executeActions(t, ctx, guard, details.UserId)

	result.Handled = true

	// Check for item consumption on item_give events
	if eventType == "item_give" {
		for _, a := range t.def.Actions {
			if a.ConsumeItem > 0 {
				result.ConsumeItem = true
				break
			}
		}
	}

	// Chain: for each granted token, evaluate quest_granted triggers
	for _, token := range grantedTokens {
		chainDetails := EventDetails{
			UserId:     details.UserId,
			RoomId:     details.RoomId,
			QuestToken: token,
		}
		chainResult := e.evaluate("quest_granted", chainDetails, player, ctx, guard)
		if chainResult.Handled {
			result.Handled = true
		}
	}

	return result
}

// fieldMatches returns, in index order, the triggers for eventType whose field
// filters match the event details. It is the match step shared by evaluate
// and WouldFire, so a dry run and a real notify can never disagree on which
// triggers an event reaches.
//
// Conditions are deliberately not checked here. evaluate checks them per
// trigger, after earlier triggers' actions (and their quest_granted chains)
// have run, because those actions change the player state a later trigger's
// conditions read. Field matching depends only on the event, so computing it
// up front changes nothing.
func (e *Engine) fieldMatches(eventType string, details EventDetails) []*indexedTrigger {
	triggers := e.triggerIndex[eventType]

	// A trigger noun matches the typed noun exactly, then as a singular/plural
	// variant ("look slabs" for `noun: slab`), then as the room noun the typed
	// words name ("look floor board" for `noun: floorboard` where the room
	// aliases `floor board` to it). The first tier that matches any trigger
	// is the only one used, so a quest that authors two spellings of one
	// thing never double-fires.
	nounTier := nounExact
	if details.Noun != "" {
		nounTier = pickNounTier(triggers, details)
	}

	var out []*indexedTrigger
	for _, t := range triggers {
		if matchTriggerFields(t.def, details, nounTier) {
			out = append(out, t)
		}
	}
	return out
}

// nounTier is how widely a trigger noun may match the event's noun. Each tier
// includes the ones before it.
type nounTier int

const (
	// nounExact: the trigger noun equals the typed noun.
	nounExact nounTier = iota
	// nounNumber: or differs from it only by singular/plural
	// (util.NounNumberVariants).
	nounNumber
	// nounRoom: or equals the room noun the typed words name
	// (EventDetails.RoomNoun), or a singular/plural spelling of it.
	nounRoom
)

// pickNounTier returns the narrowest tier at which any trigger's fields match.
// When none does, nothing fires whatever the tier, so the widest is returned.
func pickNounTier(triggers []*indexedTrigger, d EventDetails) nounTier {
	for _, tier := range []nounTier{nounExact, nounNumber} {
		for _, t := range triggers {
			if t.def.Noun != "" && matchTriggerFields(t.def, d, tier) {
				return tier
			}
		}
	}
	return nounRoom
}

// matchTriggerFields checks whether a trigger's field filters match the event
// details, with the trigger noun matched at the given tier.
// roomZoneOf is the zone of a template room, for the zone: filter. A seam so
// tests need no world; the live index costs a map read, no file read.
var roomZoneOf = rooms.TemplateRoomZone

func matchTriggerFields(t *TriggerDef, d EventDetails, tier nounTier) bool {
	// An explore (distinct) trigger is read from the map at two moments only,
	// CheckExplore and step start, never matched on an event (#375 phase 3b
	// slice 4).
	if t.Distinct != "" {
		return false
	}
	if t.Mob > 0 && t.Mob != d.MobId {
		return false
	}
	if t.Room > 0 && t.Room != d.RoomId {
		return false
	}
	// Area and zone filters read the event's template room (#375 phase 3b
	// slice 4): every event source passes rooms.OriginalRoomId.
	if t.Area != "" && !areas.Contains(t.Area, d.RoomId) {
		return false
	}
	if t.Zone != "" && roomZoneOf(d.RoomId) != t.Zone {
		return false
	}
	if t.Item > 0 && t.Item != d.ItemId {
		return false
	}
	if t.Skill != "" && t.Skill != d.Skill {
		return false
	}
	if t.Command != "" && t.Command != d.Command {
		return false
	}
	if t.QuestToken != "" && t.QuestToken != d.QuestToken {
		return false
	}
	if t.Target != "" && t.Target != d.TargetKind {
		return false
	}
	if t.Effect != "" && t.Effect != d.Effect {
		return false
	}
	if t.Topic != "" && t.Topic != d.Topic {
		return false
	}
	if t.Noun != "" && !nounMatches(t.Noun, d, tier) {
		return false
	}
	// Only room_interact carries a verb: an empty trigger verb means look
	// (#375 phase 3a), so a look trigger never fires on search.
	if t.Verb != "" || d.Verb != "" {
		if quests.InteractVerb(t.Verb) != d.Verb {
			return false
		}
	}
	return true
}

// nounMatches reports whether trigger noun n matches the event's noun at tier.
//
// A hyphen matches a space (util.NounHyphenFold) in the trigger noun, the
// typed noun and the room noun alike.
func nounMatches(n string, d EventDetails, tier nounTier) bool {
	n = util.NounHyphenFold(n)
	d.Noun = util.NounHyphenFold(d.Noun)
	d.RoomNoun = util.NounHyphenFold(d.RoomNoun)
	if n == d.Noun {
		return true
	}
	if tier >= nounNumber && d.Noun != "" && slices.Contains(util.NounNumberVariants(d.Noun), n) {
		return true
	}
	if tier >= nounRoom && d.RoomNoun != "" {
		return n == d.RoomNoun || slices.Contains(util.NounNumberVariants(d.RoomNoun), n)
	}
	return false
}

// executeActions runs a trigger's actions in order, with panic recovery per
// action, and returns the tokens granted.
//
// A trigger's actions are applied as a unit: if one fails or panics, the
// remaining actions are ABANDONED. Previously each action was isolated and the
// loop carried on, which let a quest step give the player the item but never
// set the flag — a state no content author designed and one that is invisible
// apart from a log line. There is still no rollback of what already ran, so a
// failure leaves a partial effect; aborting simply stops it compounding.
//
// A duplicate grant is not a failure. It is skipped and the loop continues.
func (e *Engine) executeActions(t *indexedTrigger, ctx ActionContext, guard *EvalGuard, userId int) []string {
	var granted []string

	for i, a := range t.def.Actions {
		// ok reports whether the loop may proceed to the next action.
		ok := func() (ok bool) {
			defer func() {
				if r := recover(); r != nil {
					LogError("panic in action %d of trigger %s: %v (abandoning %d remaining action(s))",
						i, t.trigId, r, len(t.def.Actions)-i-1)
					ok = false
				}
			}()

			// For grant actions, check the guard first. A duplicate grant is a
			// no-op, not an error — keep going.
			if a.Grant != "" {
				if !guard.MarkGranted(a.Grant) {
					LogVerboseF(userId, "skipping duplicate grant %s", a.Grant)
					return true
				}
			}

			if err := ExecuteAction(a, ctx); err != nil {
				LogError("action %d of trigger %s failed: %v (abandoning %d remaining action(s))",
					i, t.trigId, err, len(t.def.Actions)-i-1)
				return false
			}

			if a.Grant != "" {
				granted = append(granted, a.Grant)
			}
			return true
		}()

		if !ok {
			break
		}
	}

	return granted
}
