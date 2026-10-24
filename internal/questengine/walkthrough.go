package questengine

import (
	"cmp"
	"fmt"
	"slices"
	"sort"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// Walk drives every quest in w through the real trigger engine and the real
// dialogue topics, from its offer to its hand-in (spec 7), and reports what
// cannot be completed: walk-dead-end, walk-no-source, walk-unspawned. Findings
// share questloop's shape and backlog. A quest with no offer is skipped: the
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
	ways := questloop.WaysInto(w)
	var out []questloop.Finding
	for _, q := range w.Quests {
		if q == nil || len(q.Steps) == 0 {
			continue
		}
		out = append(out, walkQuest(eng, q, steps, ways, src)...)
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

func walkQuest(eng *Engine, q *quests.Quest, steps map[int][]string, ways map[string][]questloop.Way, src questloop.Sources) []questloop.Finding {
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

	p := newWalkPlayer(steps)
	out = append(out, checkWay(q, *offer, src)...)
	trail := fireWay(eng, p, *offer, q.QuestId)
	if !trail[stepToken(q, 0)] {
		dead(offer.Where, "the offer cannot be taken")
		return out
	}
	cur := stepIndex(p, q)
	if k := skipped(trail, q, 0, cur); k >= 0 {
		dead(offer.Where, fmt.Sprintf("skips to %d-%s, passing %s", q.QuestId, p.progress[q.QuestId], stepToken(q, k)))
	}
	out = append(out, checkChained(q, ways, src, 1, cur)...)

	last := len(q.Steps) - 1
	for cur < last {
		i := cur + 1
		tok, want := stepToken(q, i), q.Steps[i].Id

		// Skip probe: a Way into a later step must not fire from here, or a
		// player can jump over step i. A Way that also grants step i is the
		// step's own Way and is judged below.
		for j := i + 1; j <= last; j++ {
			for _, wy := range ways[stepToken(q, j)] {
				if wy.Reward || sharesWay(ways[tok], wy) || !itemsHeldEarly(p, wy, q.QuestId, src) {
					continue
				}
				c := p.clone()
				tr := fireWay(eng, c, wy, q.QuestId)
				if got := stepIndex(c, q); got > i && skipped(tr, q, i, got) >= 0 {
					dead(wy.Where, fmt.Sprintf("skips to %d-%s from %d-%s, passing %s",
						q.QuestId, c.progress[q.QuestId], q.QuestId, q.Steps[cur].Id, tok))
				}
			}
		}

		var next *walkPlayer
		for _, wy := range ways[tok] {
			if wy.Reward {
				continue
			}
			out = append(out, checkWay(q, wy, src)...)
			c := p.clone()
			tr := fireWay(eng, c, wy, q.QuestId)
			got := stepIndex(c, q)
			k := skipped(tr, q, i, got)
			switch {
			case got < i:
				dead(wy.Where, "does not reach "+tok)
			case k >= 0:
				dead(wy.Where, fmt.Sprintf("skips to %d-%s, passing %s", q.QuestId, c.progress[q.QuestId], stepToken(q, k)))
			default:
				if next == nil {
					next = c
				}
			}
		}
		if next == nil {
			dead("step "+want, "no way in reaches "+tok)
			return out
		}
		// A quest_granted chain may have carried the player past step i in
		// the same firing, through every step between (skipped checked it).
		got := stepIndex(next, q)
		out = append(out, checkChained(q, ways, src, i+1, got)...)
		p, cur = next, got
	}
	return out
}

// wayItemNeeds lists the items a Way asks the player to hold or hand over:
// has_item, an item_give or item_gain trigger's item, a topic's requiresItem
// and carriesItem.
func wayItemNeeds(wy questloop.Way) []int {
	var ids []int
	switch {
	case wy.Trigger != nil:
		ids = append(ids, wy.Trigger.Conditions.HasItem)
		if wy.Trigger.Event == "item_give" || wy.Trigger.Event == "item_gain" {
			ids = append(ids, wy.Trigger.Item)
		}
	case wy.Topic != nil:
		ids = append(ids, wy.Topic.RequiresItem, wy.Topic.CarriesItem)
	}
	return ids
}

// itemsHeldEarly reports whether the player at p's step could hold every
// item wy needs: p already holds it (an earlier step of the walk handed it
// over), or Sources.ItemAvailable says it can be had at p's progress without
// quest own's later steps. The skip probe fires a later Way only then; an
// item only those later steps hand out proves they were done.
func itemsHeldEarly(p *walkPlayer, wy questloop.Way, own int, src questloop.Sources) bool {
	for _, id := range wayItemNeeds(wy) {
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
func checkChained(q *quests.Quest, ways map[string][]questloop.Way, src questloop.Sources, from, to int) []questloop.Finding {
	var out []questloop.Finding
	for k := from; k <= to; k++ {
		for _, wy := range ways[stepToken(q, k)] {
			if !wy.Reward {
				out = append(out, checkWay(q, wy, src)...)
			}
		}
	}
	return out
}

// sharesWay reports whether wy is the same trigger or topic as one of ws.
func sharesWay(ws []questloop.Way, wy questloop.Way) bool {
	for _, x := range ws {
		if (wy.Trigger != nil && x.Trigger == wy.Trigger) || (wy.Topic != nil && x.Topic == wy.Topic) {
			return true
		}
	}
	return false
}

// fireWay meets a Way's conditions on p and fires it, returning the steps the
// firing put p on.
func fireWay(eng *Engine, p *walkPlayer, wy questloop.Way, own int) walkTrail {
	trail := walkTrail{}
	switch {
	case wy.Trigger != nil:
		fireTrigger(eng, p, wy.Trigger, own, trail)
	case wy.Topic != nil:
		meetTopic(p, wy.Topic, own)
		fireTopic(eng, p, wy, trail)
	}
	return trail
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

// meetTopic gives p what a topic's gates ask for.
func meetTopic(p *walkPlayer, t *dialogue.Topic, own int) {
	for _, tok := range t.QuestRequired {
		meetToken(p, tok, own)
	}
	for k, v := range t.QuestFlagRequired {
		p.flags[k] = v
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
// a fresh dialogue grant, because the dialogue grant path does not).
func fireTopic(eng *Engine, p *walkPlayer, wy questloop.Way, trail walkTrail) bool {
	ps := p.dialogueState()
	give := ps.GiveQuest
	ps.GiveQuest = func(token string) {
		give(token)
		trail.note(p, token)
	}
	if wy.File == nil || !wy.File.Eligible(wy.Topic, ps) {
		return false
	}
	tok := wy.Topic.GrantsQuest
	id, _ := splitToken(tok)
	before, had := p.progress[id]
	if !wy.File.Apply(wy.Topic, ps) {
		return false
	}
	if after, has := p.progress[id]; tok != "" && has && (!had || after != before) {
		eng.Notify("quest_granted", EventDetails{RoomId: p.roomId, QuestToken: tok}, p, &trailCtx{walkCtx{p: p}, trail})
	}
	return true
}

// fireTrigger meets the trigger's conditions on p, then sends the event its
// filters describe through the engine.
func fireTrigger(eng *Engine, p *walkPlayer, t *quests.TriggerDef, own int, trail walkTrail) bool {
	c := t.Conditions
	for _, tok := range c.Has {
		meetToken(p, tok, own)
	}
	if c.HasItem > 0 {
		p.items[c.HasItem] = true
	}
	for k, v := range c.HasFlag {
		p.flags[k] = v
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
	// quest that is the moment p lands on that step, so it fires only from
	// exactly that step; another quest's token is a prerequisite and is held.
	if t.Event == "quest_granted" && t.QuestToken != "" {
		id, step := splitToken(t.QuestToken)
		if id == own && p.progress[own] != step {
			return false
		}
		meetToken(p, t.QuestToken, own)
	}
	d := EventDetails{RoomId: t.Room, MobId: t.Mob, ItemId: t.Item, Skill: t.Skill,
		Command: t.Command, Noun: t.Noun, Verb: t.Verb, QuestToken: t.QuestToken}
	if d.RoomId == 0 {
		d.RoomId = c.InRoom
	}
	if d.RoomId != 0 {
		p.roomId = d.RoomId
	}
	return eng.Notify(t.Event, d, p, &trailCtx{walkCtx{p: p}, trail}).Handled
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
// that check.
func checkWay(q *quests.Quest, wy questloop.Way, src questloop.Sources) []questloop.Finding {
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
	case wy.Topic != nil:
		needItem(wy.Topic.RequiresItem, "requiresItem")
		needItem(wy.Topic.CarriesItem, "carriesItem")
		needMob(wy.MobId, "the NPC who has this topic")
	}
	return out
}
