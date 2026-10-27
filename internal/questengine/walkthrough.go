package questengine

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/areas"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Walk drives every quest in w through the real trigger engine and the real
// dialogue topics, from its offer to its hand-in (spec 7), and reports what
// cannot be completed: walk-dead-end, walk-no-source, walk-unspawned,
// walk-no-spell (a spell_landed Way no state on the path holds a spell for).
// Findings share questloop's shape and backlog. A quest with no offer is skipped: the
// start rule reports it.
func Walk(w questloop.World, src questloop.Sources) []questloop.Finding {
	// Sorted by id, so the engine's trigger order (and so every firing) does
	// not depend on the order the caller lists the quests in.
	var qs []*quests.Quest
	for _, q := range w.Quests {
		if q != nil {
			qs = append(qs, q)
		}
	}
	slices.SortStableFunc(qs, func(a, b *quests.Quest) int { return cmp.Compare(a.QuestId, b.QuestId) })
	w.Quests = qs

	eng := NewEngine()
	steps := map[int][]string{}
	for _, q := range w.Quests {
		if q == nil {
			continue
		}
		eng.RegisterQuest(q)
		for _, s := range q.Steps {
			steps[q.QuestId] = append(steps[q.QuestId], s.Id)
		}
	}
	ways := questloop.WaysInto(w) // external grants included (w.External)
	env := newWalkEnv(eng, src, w.Quests, w.Dialogue)
	var out []questloop.Finding
	for _, q := range w.Quests {
		if q == nil || len(q.Steps) == 0 {
			continue
		}
		out = append(out, walkQuest(env, q, steps, ways)...)
	}
	// Stable, so that when one Way is reported twice under one key the first
	// report (the skip probe runs before the Way's own step) is the one kept.
	sort.SliceStable(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return dedupeFindings(out)
}

func stepToken(q *quests.Quest, i int) string { return fmt.Sprintf("%d-%s", q.QuestId, q.Steps[i].Id) }

// stepIndex is the index of p's current step in q, or -1 before the offer.
func stepIndex(p *walkPlayer, q *quests.Quest) int {
	cur, ok := p.progress[q.QuestId]
	if !ok {
		return -1
	}
	return p.index(q.QuestId, cur)
}

// walkTrail records every step a firing put the player on, so a firing that
// ends past step i can be told apart: it passed through i (a quest_granted
// chain finished the next step too, as the game does) or it jumped over i.
type walkTrail map[string]bool

func (s walkTrail) note(p *walkPlayer, token string) {
	id, _ := splitToken(token)
	if st, ok := p.progress[id]; ok {
		s[fmt.Sprintf("%d-%s", id, st)] = true
	}
}

// trailCtx is walkCtx with every grant noted on the trail, sequence
// on_complete grants included.
type trailCtx struct {
	walkCtx
	trail walkTrail
}

func (c *trailCtx) GrantQuest(token string) {
	c.walkCtx.GrantQuest(token)
	c.trail.note(c.p, token)
}

func (c *trailCtx) QueueSequence(s SequenceDef) {
	for _, a := range s.OnComplete {
		_ = ExecuteAction(a, c)
	}
}

// walkEnv is what every firing reads: the engine, the world's sources and
// the dialogue files by NPC (a tree grant checks its NPC's topics).
type walkEnv struct {
	eng   *Engine
	src   questloop.Sources
	files map[int]*dialogue.DialogueFile
	// heardKeys lists, per NPC in id order, the topics some heardRequired
	// names: the only topics whose hearing a gate can read.
	heardKeys []heardKey
	// setters lists, per "key=value", the triggers and topics that set
	// that quest flag, with the tokens each also grants (flagSetter).
	setters map[string][]flagSetter
	depth   int // setter nesting, bounded by maxSetterDepth
	// teachers lists, per spell id, the tokens whose holder knows the spell:
	// every token a trigger that teaches it grants (and a quest_granted
	// trigger's own token), and a quest's final step when its rewards.spellid
	// teaches it. It covers a prerequisite quest held by meetToken, which
	// the walk never fires.
	teachers map[string][]string
	// spellRefused is set when fireTrigger refused a top-level (not a flag
	// setter's) spell_landed trigger because no held spell can land it.
	spellRefused bool
	// probing is set while the skip probe fires a later Way. The spells the
	// walk holds are a minimum (starters, teach_spell, spellid rewards),
	// right for proving a step reachable; a skip needs the opposite, since a
	// player may know any spell (random discovery, a book), so a probing
	// spell_landed trigger is never refused for want of a spell.
	probing bool
}

// flagSetter is a trigger or topic that sets a quest flag.
type flagSetter struct {
	way    questloop.Way
	grants []string
}

// maxSetterDepth bounds how deep one setter's own flag gates may reach for
// further setters.
const maxSetterDepth = 3

func flagKV(key, value string) string { return key + "=" + value }

type heardKey struct {
	mobId int
	key   string
}

func newWalkEnv(eng *Engine, src questloop.Sources, qs []*quests.Quest, dialogues []*dialogue.DialogueFile) *walkEnv {
	env := &walkEnv{eng: eng, src: src, files: map[int]*dialogue.DialogueFile{}, setters: map[string][]flagSetter{},
		teachers: map[string][]string{}}
	for _, q := range qs {
		for i := range q.Triggers {
			t := &q.Triggers[i]
			var grants []string
			var sets []string
			var teaches []string
			var walk func([]quests.ActionDef)
			walk = func(as []quests.ActionDef) {
				for _, a := range as {
					if a.Grant != "" && !strings.HasPrefix(a.Grant, "-") {
						grants = append(grants, a.Grant)
					}
					if a.SetFlag != nil {
						sets = append(sets, flagKV(a.SetFlag.Key, a.SetFlag.Value))
					}
					if a.TeachSpell != "" {
						teaches = append(teaches, a.TeachSpell)
					}
					if a.Sequence != nil {
						walk(a.Sequence.OnComplete)
					}
				}
			}
			walk(t.Actions)
			for _, kv := range sets {
				env.setters[kv] = append(env.setters[kv], flagSetter{way: questloop.Way{Trigger: t, Owner: q.QuestId}, grants: grants})
			}
			knowers := grants
			if t.Event == "quest_granted" && t.QuestToken != "" {
				// It fires as its token is granted: every holder ran it.
				knowers = append(slices.Clone(grants), t.QuestToken)
			}
			for _, s := range teaches {
				env.teachers[s] = append(env.teachers[s], knowers...)
			}
		}
		if s := q.Rewards.SpellId; s != "" && len(q.Steps) > 0 {
			env.teachers[s] = append(env.teachers[s], stepToken(q, len(q.Steps)-1))
		}
	}
	seen := map[heardKey]bool{}
	for _, df := range dialogues {
		if df == nil {
			continue
		}
		env.files[df.MobId] = df
		for i := range df.Topics {
			t := &df.Topics[i]
			for _, k := range t.HeardRequired {
				if hk := (heardKey{df.MobId, k}); !seen[hk] {
					seen[hk] = true
					env.heardKeys = append(env.heardKeys, hk)
				}
			}
			if s := t.SetsQuestFlag; s != nil {
				var grants []string
				if g := t.GrantsQuest; g != "" && !strings.HasPrefix(g, "-") {
					grants = []string{g}
				}
				kv := flagKV(s.Key, s.Value)
				env.setters[kv] = append(env.setters[kv], flagSetter{
					way: questloop.Way{Topic: t, MobId: df.MobId, File: df}, grants: grants})
			}
		}
	}
	slices.SortFunc(env.heardKeys, func(a, b heardKey) int {
		return cmp.Or(cmp.Compare(a.mobId, b.mobId), cmp.Compare(a.key, b.key))
	})
	return env
}

// holds reports whether p knows a spell on quest own's walk: a starter, a
// spell a teach_spell taught during this walk, or a spell another quest
// teaches to whoever holds a token of it p holds (teachers). The walked
// quest's own teachers are not read: the walk fired its Ways, so p.spells
// already says exactly which of them this state ran.
func (env *walkEnv) holds(p *walkPlayer, spellId string, own int) bool {
	if slices.Contains(env.src.StarterSpells, spellId) || p.spells[spellId] {
		return true
	}
	return slices.ContainsFunc(env.teachers[spellId], func(tok string) bool {
		id, _ := splitToken(tok)
		return id != own && p.HasQuest(tok)
	})
}

// spellReachable reports whether p holds a spell that can land a
// spell_landed trigger t: its noun (the spell id) and its effect both
// match. A nil Sources.SpellEffect skips the check.
func (env *walkEnv) spellReachable(p *walkPlayer, t *quests.TriggerDef, own int) bool {
	if env.src.SpellEffect == nil {
		return true
	}
	fits := func(id string) bool {
		return (t.Noun == "" || id == t.Noun) && (t.Effect == "" || env.src.SpellEffect(id) == t.Effect) &&
			env.holds(p, id, own)
	}
	if t.Noun != "" {
		return fits(t.Noun)
	}
	return slices.ContainsFunc(env.src.StarterSpells, fits) ||
		slices.ContainsFunc(slices.Collect(maps.Keys(p.spells)), fits) ||
		slices.ContainsFunc(slices.Collect(maps.Keys(env.teachers)), fits)
}

// hear marks as heard every info topic a heardRequired names that is open
// to p now, until nothing more opens: a player can listen to any topic on
// the menu before choosing the one that needs it. Applying an info topic
// also marks it (dialogue Apply).
func (env *walkEnv) hear(p *walkPlayer) {
	if env == nil {
		return
	}
	ps := p.dialogueState()
	for changed := true; changed; {
		changed = false
		for _, hk := range env.heardKeys {
			key := dialogue.HeardKey(hk.mobId, hk.key)
			if p.heard[key] {
				continue
			}
			df := env.files[hk.mobId]
			if t := df.FindTopic(hk.key); t != nil && t.Kind == dialogue.KindInfo && df.Eligible(t, ps) {
				p.heard[key] = true
				changed = true
			}
		}
	}
}

func walkQuest(env *walkEnv, q *quests.Quest, steps map[int][]string, ways map[string][]questloop.Way) []questloop.Finding {
	var out []questloop.Finding
	dead := func(where, msg string) {
		out = append(out, questloop.Finding{Rule: "walk-dead-end", QuestId: q.QuestId, Where: where, Msg: msg})
	}

	var offer *questloop.Way
	for _, wy := range ways[stepToken(q, 0)] {
		if wy.Topic != nil && wy.Topic.Kind == dialogue.KindOffer {
			wy := wy
			offer = &wy
			break
		}
	}
	if offer == nil {
		return nil
	}

	// Source checks need no walk state: every Way into every step, once,
	// so a step after a dead one is still checked.
	last := len(q.Steps) - 1
	out = append(out, env.checkWay(q, *offer)...)
	out = append(out, env.checkChained(q, ways, 1, last)...)

	p := newWalkPlayer(steps)
	env.hear(p)
	trail := fireWay(env, p, *offer, q.QuestId)
	if !trail[stepToken(q, 0)] {
		dead(offer.Where, "the offer cannot be taken")
		return out
	}
	cur := stepIndex(p, q)
	if k := skipped(trail, q, 0, cur); k >= 0 {
		dead(offer.Where, fmt.Sprintf("skips to %d-%s, passing %s", q.QuestId, p.progress[q.QuestId], stepToken(q, k)))
	}

	// The walk carries every distinct state forward, not only the first Way
	// that worked: two Ways into one step can leave different flags or items
	// (a branch), and a later Way may need one branch's. rest[c] holds the
	// states resting on step c (a quest_granted chain may carry a firing past
	// several steps, so states rest at different steps). Each step keeps at
	// most maxWalkStates distinct states, in Way order; the cost per step is
	// (Ways into it plus the skip probe's) times its states, and shipped
	// quests carry one or two.
	rest := map[int]*walkStates{cur: newWalkStates(p)}
	reach := cur // the furthest step any state has reached
	for c := 0; c < last; c++ {
		states := rest[c].list()
		if len(states) == 0 {
			continue // passed through by a chain, never rested on
		}
		i := c + 1
		tok, want := stepToken(q, i), q.Steps[i].Id

		for _, p := range states {
			env.hear(p)
			// Skip probe: a Way into a later step must not fire from here,
			// or a player can jump over step i. A Way that also grants step
			// i is the step's own Way and is judged below.
			for j := i + 1; j <= last; j++ {
				for _, wy := range ways[stepToken(q, j)] {
					if wy.Reward || ownGranted(wy, q.QuestId) || isTreeGrant(wy) || sharesWay(ways[tok], wy) || !env.itemsHeldEarly(p, wy, q.QuestId) {
						continue
					}
					cl := p.clone()
					env.probing = true
					tr := fireWay(env, cl, wy, q.QuestId)
					env.probing = false
					if got := stepIndex(cl, q); got > i && skipped(tr, q, i, got) >= 0 {
						dead(wy.Where, fmt.Sprintf("skips to %d-%s from %d-%s, passing %s",
							q.QuestId, cl.progress[q.QuestId], q.QuestId, q.Steps[c].Id, tok))
					}
				}
			}
		}

		// A Way that fails from one branch's state and works from another's
		// is that branch's Way, not a broken one: it is reported only when
		// it works from no state. A skip from any state is reported.
		var atGrant []questloop.Way
		worked := map[string]bool{}
		failed := map[string]bool{}
		// otherFail marks a Way that failed from some state for a reason
		// other than a spell no state holds; one that failed only for want
		// of a spell is reported as walk-no-spell, not as a dead end.
		otherFail := map[string]bool{}
		var failOrder []string
		for _, wy := range ways[tok] {
			if wy.Reward {
				continue
			}
			if ownGranted(wy, q.QuestId) {
				// It fires only inside the chain of the grant that lands on
				// its token, which the firing that reached this step ran.
				atGrant = append(atGrant, wy)
				continue
			}
			for _, p := range states {
				cl := p.clone()
				env.spellRefused = false
				tr := fireWay(env, cl, wy, q.QuestId)
				got := stepIndex(cl, q)
				k := skipped(tr, q, i, got)
				switch {
				case got < i:
					if !failed[wy.Where] {
						failed[wy.Where] = true
						failOrder = append(failOrder, wy.Where)
					}
					if !env.spellRefused {
						otherFail[wy.Where] = true
					}
				case k >= 0:
					dead(wy.Where, fmt.Sprintf("skips to %d-%s, passing %s", q.QuestId, cl.progress[q.QuestId], stepToken(q, k)))
				default:
					worked[wy.Where] = true
					// A quest_granted chain may have carried the player past
					// step i, through every step between (skipped checked it).
					if rest[got] == nil {
						rest[got] = &walkStates{}
					}
					rest[got].add(cl)
					reach = max(reach, got)
				}
			}
		}
		for _, where := range failOrder {
			switch {
			case worked[where]:
			case !otherFail[where]:
				out = append(out, questloop.Finding{Rule: "walk-no-spell", QuestId: q.QuestId, Where: where,
					Msg: fmt.Sprintf("does not reach %s: it needs a spell that lands (%s), and no spell a player holds on this path (starters, teach_spell, spellid rewards) does", tok, spellWant(ways[tok], where))})
			default:
				dead(where, "does not reach "+tok)
			}
		}
		if reach < i {
			for _, wy := range atGrant {
				dead(wy.Where, fmt.Sprintf("fires only when %s is granted, and did not carry the player on to %s", wy.Trigger.QuestToken, tok))
			}
			dead("step "+want, "no way in reaches "+tok)
			return out
		}
	}
	return out
}

// spellWant names what the spell_landed Way at where asks a spell for.
func spellWant(ws []questloop.Way, where string) string {
	for _, wy := range ws {
		if wy.Where == where && wy.Trigger != nil {
			return fmt.Sprintf("effect %q, spell %q", wy.Trigger.Effect, wy.Trigger.Noun)
		}
	}
	return "?"
}

// maxWalkStates bounds the distinct states the walk keeps on one step.
const maxWalkStates = 16

// walkStates is the distinct states resting on one step, in arrival order.
type walkStates struct {
	seen   map[string]bool
	states []*walkPlayer
}

func newWalkStates(p *walkPlayer) *walkStates {
	s := &walkStates{}
	s.add(p)
	return s
}

func (s *walkStates) add(p *walkPlayer) {
	if s.seen == nil {
		s.seen = map[string]bool{}
	}
	fp := p.fingerprint()
	if s.seen[fp] || len(s.states) >= maxWalkStates {
		return
	}
	s.seen[fp] = true
	s.states = append(s.states, p)
}

func (s *walkStates) list() []*walkPlayer {
	if s == nil {
		return nil
	}
	return s.states
}

// heardTopic is the NPC's topic a topic_heard trigger listens to, or nil.
func (env *walkEnv) heardTopic(t *quests.TriggerDef) *dialogue.Topic {
	if env == nil || t == nil || t.Event != "topic_heard" {
		return nil
	}
	return env.files[t.Mob].FindTopic(t.Topic)
}

// wayItemNeeds lists the items a Way asks the player to hold or hand over:
// has_item, an item_give or item_gain trigger's item, a topic's requiresItem
// and carriesItem (a topic_heard trigger's topic included, since the walk
// plays it).
func (env *walkEnv) wayItemNeeds(wy questloop.Way) []int {
	var ids []int
	switch {
	case wy.Trigger != nil:
		ids = append(ids, wy.Trigger.Conditions.HasItem)
		if wy.Trigger.Event == "item_give" || wy.Trigger.Event == "item_gain" {
			ids = append(ids, wy.Trigger.Item)
		}
		if tp := env.heardTopic(wy.Trigger); tp != nil {
			ids = append(ids, tp.RequiresItem, tp.CarriesItem)
		}
	case wy.Topic != nil:
		ids = append(ids, wy.Topic.RequiresItem, wy.Topic.CarriesItem)
	case wy.External != nil:
		ids = append(ids, wy.External.ItemId)
	}
	return ids
}

// isTreeGrant reports whether wy is a behaviour tree's grant. The walk
// cannot run a tree's own gates, so it takes the grant only from the step
// before it (fireExternal) and never fires it in the skip probe.
func isTreeGrant(wy questloop.Way) bool { return wy.External != nil && wy.External.ItemId == 0 }

// itemsHeldEarly reports whether the player at p's step could hold every
// item wy needs: p already holds it (an earlier step of the walk handed it
// over), or Sources.ItemAvailable says it can be had at p's progress without
// quest own's later steps. The skip probe fires a later Way only then; an
// item only those later steps hand out proves they were done.
func (env *walkEnv) itemsHeldEarly(p *walkPlayer, wy questloop.Way, own int) bool {
	src := env.src
	for _, id := range env.wayItemNeeds(wy) {
		if id <= 0 || p.items[id] {
			continue
		}
		if src.ItemAvailable != nil && !src.ItemAvailable(id, own, p.HasQuest) {
			return false
		}
	}
	return true
}

// skipped is the first step from index from to index to that the trail never
// put the player on, or -1 when the firing passed through every one.
func skipped(trail walkTrail, q *quests.Quest, from, to int) int {
	for k := from; k <= to; k++ {
		if !trail[stepToken(q, k)] {
			return k
		}
	}
	return -1
}

// checkChained source-checks the Ways into steps from index from to index to,
// which a quest_granted chain reached inside one firing. They are not fired:
// no walk state sits on the step before them.
func (env *walkEnv) checkChained(q *quests.Quest, ways map[string][]questloop.Way, from, to int) []questloop.Finding {
	var out []questloop.Finding
	for k := from; k <= to; k++ {
		for _, wy := range ways[stepToken(q, k)] {
			if !wy.Reward {
				out = append(out, env.checkWay(q, wy)...)
			}
		}
	}
	return out
}

// ownGranted reports whether wy is a quest_granted trigger on a token of
// quest own. The game fires it only in the chain of the grant that lands on
// that token (the engine's chain, or the quest hook on a fresh dialogue
// grant), so the walk never fires it directly: it counts only when a firing
// that grants its token runs it.
func ownGranted(wy questloop.Way, own int) bool {
	if wy.Trigger == nil || wy.Trigger.Event != "quest_granted" {
		return false
	}
	id, _ := splitToken(wy.Trigger.QuestToken)
	return id == own
}

// sharesWay reports whether wy is the same trigger or topic as one of ws.
func sharesWay(ws []questloop.Way, wy questloop.Way) bool {
	for _, x := range ws {
		if (wy.Trigger != nil && x.Trigger == wy.Trigger) || (wy.Topic != nil && x.Topic == wy.Topic) ||
			(wy.External != nil && x.External == wy.External) {
			return true
		}
	}
	return false
}

// fireWay meets a Way's conditions on p and fires it, returning the steps the
// firing put p on.
func fireWay(env *walkEnv, p *walkPlayer, wy questloop.Way, own int) walkTrail {
	trail := walkTrail{}
	switch {
	case wy.Trigger != nil:
		fireTrigger(env, p, wy.Trigger, own, trail)
	case wy.Topic != nil:
		env.meetTopic(p, wy.File, wy.Topic, own)
		env.hear(p)
		fireTopic(env.eng, p, wy, trail)
	case wy.External != nil:
		env.hear(p)
		fireExternal(env, p, wy.External, own, trail)
	}
	return trail
}

// fireExternal takes an external grant.
//
// An item questtoken grants its token once the player holds the item: it
// fires when p holds it or Sources.ItemAvailable says it can be had now. The
// quest hook then sees a fresh grant and notifies quest_granted, so the walk
// runs that chain too.
//
// A behaviour tree's grant is taken as given, since the walk cannot run the
// tree: from the step before it, once one of the topics its branch answers is
// open (Eligible) at the tree's NPC, or at once when the branch keys on no
// topic the NPC's dialogue holds. The tree sets the token before the quest
// hook runs, so the hook sees no fresh grant and runs no quest_granted chain
// (behaviortree actGrantQuest, Quest_HandleQuestUpdate), and neither does the
// walk.
func fireExternal(env *walkEnv, p *walkPlayer, g *questloop.ExternalGrant, own int, trail walkTrail) bool {
	if g.ItemId > 0 {
		avail := env.src.ItemAvailable
		if !p.items[g.ItemId] && avail != nil && !avail(g.ItemId, own, p.HasQuest) {
			return false
		}
		p.items[g.ItemId] = true
	} else if !treeTopicOpen(env, p, g) {
		return false
	}
	id, _ := splitToken(g.Token)
	before, had := p.progress[id]
	p.grant(g.Token)
	trail.note(p, g.Token)
	after, has := p.progress[id]
	if !has || (had && after == before) {
		return false
	}
	if g.ItemId > 0 {
		env.eng.Notify("quest_granted", EventDetails{RoomId: p.roomId, QuestToken: g.Token}, p, &trailCtx{walkCtx{p: p}, trail})
	}
	return true
}

// treeTopicOpen reports whether a tree grant's branch can be reached: one of
// its topic_is keys is a topic open to p at the tree's NPC, or the NPC's
// dialogue holds none of them (the branch keys on something the walk cannot
// see).
func treeTopicOpen(env *walkEnv, p *walkPlayer, g *questloop.ExternalGrant) bool {
	df := env.files[g.MobId]
	found := false
	for _, k := range g.Topics {
		t := df.FindTopic(k)
		if t == nil {
			continue
		}
		found = true
		if df.Eligible(t, p.dialogueState()) {
			return true
		}
	}
	return !found
}

// meetToken holds a token a condition or gate names, when p lacks it.
//
// A token of the walked quest itself is never held. The walk is already on
// that quest's real progress, so every earlier step is held; a condition
// naming a later step (or any own step p has not reached) can only be met by
// cheating, and holding it would let a trigger gated on its own future step
// pass the walk while no player could ever fire it. Leaving it unmet makes the
// Way fail, and the walk reports it as a dead end under the Way's own name.
// Tokens of OTHER quests are prerequisites (finish quest 7 before this one is
// offered) and are held, since the walk of quest 7 proves them reachable.
func meetToken(p *walkPlayer, token string, own int) {
	if p.HasQuest(token) {
		return
	}
	if id, _ := splitToken(token); id == own {
		return
	}
	p.hold(token)
}

// ownFlag reports whether a quest flag key ("53-route") is quest own's.
func ownFlag(key string, own int) bool {
	return strings.HasPrefix(key, strconv.Itoa(own)+"-")
}

// meetFlag gives p a flag a condition or gate asks for. Another quest's
// flag is a prerequisite and is held, like its tokens. A flag of the walked
// quest is never held: only a flag the walk really sets counts. The walk
// sets it by firing, from p's state, a trigger or topic that sets it and
// grants no token of the walked quest (quest 72's room_interact set_flag, a
// bystander's topic): when that firing sets the flag without moving the
// quest, p takes the flag. A setter that also grants a step is a Way, and
// the walk fires it as one. With no setter that fires, the gate stays shut.
func (env *walkEnv) meetFlag(p *walkPlayer, key, value string, own int) {
	if p.flags[key] == value {
		return
	}
	if !ownFlag(key, own) {
		p.flags[key] = value
		return
	}
	if env == nil || env.depth >= maxSetterDepth {
		return
	}
	env.depth++
	defer func() { env.depth-- }()
	for _, s := range env.setters[flagKV(key, value)] {
		if slices.ContainsFunc(s.grants, func(tok string) bool { id, _ := splitToken(tok); return id == own }) {
			continue
		}
		cl := p.clone()
		fireWay(env, cl, s.way, own)
		if cl.flags[key] == value && cl.progress[own] == p.progress[own] {
			p.flags[key] = value
			return
		}
	}
}

// meetTopic gives p what a topic's gates ask for, and the prerequisites (other
// quests' tokens and flags) of each info topic its heardRequired names, so
// the walk can hear that topic when it is open (walkEnv.hear).
func (env *walkEnv) meetTopic(p *walkPlayer, df *dialogue.DialogueFile, t *dialogue.Topic, own int) {
	for _, k := range t.HeardRequired {
		if h := df.FindTopic(k); h != nil {
			for _, tok := range h.QuestRequired {
				meetToken(p, tok, own)
			}
			for k, v := range h.QuestFlagRequired {
				env.meetFlag(p, k, v, own)
			}
		}
	}
	for _, tok := range t.QuestRequired {
		meetToken(p, tok, own)
	}
	for k, v := range t.QuestFlagRequired {
		env.meetFlag(p, k, v, own)
	}
	if t.RequiresItem > 0 {
		p.items[t.RequiresItem] = true
	}
	if t.CarriesItem > 0 {
		p.items[t.CarriesItem] = true
	}
	// The gate shows the topic; the charge is taken by Apply. Meet both.
	if gold := max(t.GoldRequired, t.ChargesGold); gold > p.gold {
		p.gold = gold
	}
	if t.MasterworkRequired > 0 {
		p.masterwork = true
	}
}

// fireTopic runs a topic as the talk menu does: Eligible, then Apply. A grant
// that moved the quest is then chained through quest_granted triggers, as
// the game's hook does (Quest_HandleQuestUpdate.go notifies quest_granted on
// a fresh dialogue grant, because the dialogue grant path does not). Before
// that grant lands it raises topic_heard, as conversation.applyWithHandover
// does.
func fireTopic(eng *Engine, p *walkPlayer, wy questloop.Way, trail walkTrail) bool {
	ps := p.dialogueState()
	// The game queues a dialogue grant (conversation PlayerState.GiveQuest
	// adds an events.Quest), while flags, items and gold land at once. So
	// topic_heard, raised synchronously by conversation.applyWithHandover, sees the
	// topic's other effects but not its own grant, which lands afterwards.
	var queued []string
	ps.GiveQuest = func(token string) { queued = append(queued, token) }
	if wy.File == nil || !wy.File.Eligible(wy.Topic, ps) {
		return false
	}
	if !wy.File.Apply(wy.Topic, ps) {
		return false
	}
	// conversation.applyWithHandover raises topic_heard for every played topic, so a
	// trigger may listen to an offer, a hand-in or an info topic (#375 3b).
	eng.Notify("topic_heard", EventDetails{RoomId: p.roomId, MobId: wy.MobId, Topic: wy.Topic.Key}, p, &trailCtx{walkCtx{p: p}, trail})
	for _, tok := range queued {
		id, _ := splitToken(tok)
		before, had := p.progress[id]
		p.grant(tok)
		trail.note(p, tok)
		if after, has := p.progress[id]; has && (!had || after != before) {
			eng.Notify("quest_granted", EventDetails{RoomId: p.roomId, QuestToken: tok}, p, &trailCtx{walkCtx{p: p}, trail})
		}
	}
	return true
}

// fireTrigger meets the trigger's conditions on p, then sends the event its
// filters describe through the engine.
// templateRoomZones is the template room to zone index the walk reads for a
// zone trigger's room; a seam so tests need no world.
var templateRoomZones = rooms.TemplateRoomZones

// walkRoomIn is the room the walk's synthetic event stands in for a trigger
// that names no room: the lowest room of its area, else of its zone, else 0
// (#375 phase 3b slice 4).
func walkRoomIn(area, zone string) int {
	if area != "" {
		if rs := areas.Rooms(area); len(rs) > 0 {
			return rs[0]
		}
		return 0
	}
	if zone == "" {
		return 0
	}
	best := 0
	for id, z := range templateRoomZones() {
		if z == zone && (best == 0 || id < best) {
			best = id
		}
	}
	return best
}

func fireTrigger(env *walkEnv, p *walkPlayer, t *quests.TriggerDef, own int, trail walkTrail) bool {
	c := t.Conditions
	for _, tok := range c.Has {
		meetToken(p, tok, own)
	}
	if c.HasItem > 0 {
		p.items[c.HasItem] = true
	}
	for k, v := range c.HasFlag {
		env.meetFlag(p, k, v, own)
	}
	if c.HasGold > p.gold {
		p.gold = c.HasGold
	}
	if c.HasMasterwork > 0 {
		p.masterwork = true
	}
	if c.InRoom > 0 {
		p.roomId = c.InRoom
	}
	if t.Item > 0 && (t.Event == "item_give" || t.Event == "item_gain") {
		p.items[t.Item] = true
	}
	// quest_granted fires only when its token is granted. For the walked
	// quest that is inside the chain of the grant itself, never fired
	// directly (ownGranted); another quest's token is a prerequisite and is
	// held.
	if t.Event == "quest_granted" && t.QuestToken != "" {
		if id, _ := splitToken(t.QuestToken); id == own {
			return false
		}
		meetToken(p, t.QuestToken, own)
	}
	d := EventDetails{RoomId: t.Room, MobId: t.Mob, ItemId: t.Item, Skill: t.Skill,
		Command: t.Command, Noun: t.Noun, QuestToken: t.QuestToken, TargetKind: t.Target, Effect: t.Effect, Topic: t.Topic}
	if t.Event == "room_interact" {
		// The dispatcher always sends a verb family; an empty trigger verb
		// means look (#375 phase 3a).
		d.Verb = quests.InteractVerb(t.Verb)
	}
	if d.RoomId == 0 {
		d.RoomId = c.InRoom
	}
	if d.RoomId == 0 {
		d.RoomId = walkRoomIn(t.Area, t.Zone)
	}
	if d.RoomId != 0 {
		p.roomId = d.RoomId
	}
	// topic_heard is raised only by playing the topic (conversation.applyWithHandover), so
	// the walk plays it: the NPC's topic, its gates met, Eligible, then Apply,
	// whose effects (a flag, an item) later steps may read. fireTopic then
	// sends the event, as the game does after every played topic. A topic the
	// NPC lacks, or one that never opens, cannot be heard.
	if t.Event == "topic_heard" {
		df := env.files[t.Mob]
		tp := df.FindTopic(t.Topic)
		if df == nil || tp == nil {
			return false
		}
		env.meetTopic(p, df, tp, own)
		env.hear(p)
		return fireTopic(env.eng, p, questloop.Way{Topic: tp, MobId: t.Mob, File: df}, trail)
	}
	// A spell_landed event needs a spell p holds that can land it (its
	// conditions met first, since a prerequisite token they hold may be what
	// teaches it). Refused, nothing fires; when the trigger would have fired
	// with the spell, a top-level firing notes it, for walk-no-spell. The skip
	// probe assumes any spell (walkEnv.probing). The probe asks whether THIS
	// trigger would fire (its fields, as the engine matches them, and its
	// conditions): another spell_landed trigger the same event reaches is its
	// own Way, and its fire says nothing about this one.
	if t.Event == "spell_landed" && !env.probing && !env.spellReachable(p, t, own) {
		if env.depth == 0 {
			env.spellRefused = slices.ContainsFunc(env.eng.fieldMatches(t.Event, d), func(it *indexedTrigger) bool {
				return it.def == t
			}) && EvalConditions(t.Conditions, p)
		}
		return false
	}
	return env.eng.Notify(t.Event, d, p, &trailCtx{walkCtx{p: p}, trail}).Handled
}

func dedupeFindings(fs []questloop.Finding) []questloop.Finding {
	seen := map[string]bool{}
	var out []questloop.Finding
	for _, f := range fs {
		if !seen[f.Key()] {
			seen[f.Key()] = true
			out = append(out, f)
		}
	}
	return out
}

// checkWay reports a Way's needs the world cannot meet (spec 7.2): an item
// nothing provides, a mob or NPC nothing spawns. A nil Sources func skips
// that check. A topic_heard trigger needs what its topic needs: the NPC, and
// the topic's requiresItem and carriesItem.
func (env *walkEnv) checkWay(q *quests.Quest, wy questloop.Way) []questloop.Finding {
	src := env.src
	var out []questloop.Finding
	needItem := func(id int, what string) {
		if id > 0 && src.ItemSources != nil && len(src.ItemSources(id)) == 0 {
			out = append(out, questloop.Finding{Rule: "walk-no-source", QuestId: q.QuestId, Where: wy.Where,
				Msg: fmt.Sprintf("needs item %d (%s), which no mob, shop, room, forage table, recipe, quest or topic provides", id, what)})
		}
	}
	needMob := func(id int, what string) {
		if id > 0 && src.MobSpawned != nil && !src.MobSpawned(id) {
			out = append(out, questloop.Finding{Rule: "walk-unspawned", QuestId: q.QuestId, Where: wy.Where,
				Msg: fmt.Sprintf("needs mob %d (%s), which no room spawns and no quest spawns", id, what)})
		}
	}
	switch {
	case wy.Trigger != nil:
		t := wy.Trigger
		needItem(t.Conditions.HasItem, "has_item")
		if t.Event == "item_give" || t.Event == "item_gain" {
			needItem(t.Item, t.Event)
		}
		if t.Event == "mob_death" || t.Event == "item_give" {
			needMob(t.Mob, t.Event)
		}
		if t.Event == "topic_heard" {
			needMob(t.Mob, "the NPC who has this topic")
			if tp := env.heardTopic(t); tp != nil {
				needItem(tp.RequiresItem, "requiresItem")
				needItem(tp.CarriesItem, "carriesItem")
			}
		}
	case wy.Topic != nil:
		needItem(wy.Topic.RequiresItem, "requiresItem")
		needItem(wy.Topic.CarriesItem, "carriesItem")
		needMob(wy.MobId, "the NPC who has this topic")
	case wy.External != nil:
		needItem(wy.External.ItemId, "questtoken")
		needMob(wy.External.MobId, "the NPC whose tree grants it")
	}
	return out
}
