package dialogue

import (
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// itemValue is the item's sale value, a seam for tests.
var itemValue = func(itemId int) int {
	if s := items.GetItemSpec(itemId); s != nil {
		return s.Value
	}
	return 0
}

// SetItemValueForTest replaces itemValue and returns a restore func.
func SetItemValueForTest(values map[int]int) func() {
	prev := itemValue
	itemValue = func(id int) int { return values[id] }
	return func() { itemValue = prev }
}

// resuming reports whether choosing t takes a shelved quest back up: t grants
// a quest's start step and the player has that quest on the shelf.
func resuming(t *Topic, ps *PlayerState) bool {
	if t == nil || t.GrantsQuest == "" || ps == nil || ps.Shelved == nil {
		return false
	}
	qid, step := quests.TokenToParts(t.GrantsQuest)
	return step == `start` && qid > 0 && ps.Shelved(qid)
}

// resumeGivesItem is owner ruling 2: on a resume, only an item worth nothing
// that the player does not carry is handed over again.
func resumeGivesItem(itemId int, ps *PlayerState) bool {
	return itemId > 0 && itemValue(itemId) == 0 && !carriesItemPasses(itemId, ps)
}

// markerRank orders name markers: [?] over [!] over [*] (requirements rule 9).
var markerRank = map[string]int{menu.MarkerInfo: 1, menu.MarkerQuest: 2, menu.MarkerHandIn: 3}

// FindTopic returns the topic with key, or nil.
func (df *DialogueFile) FindTopic(key string) *Topic {
	if df == nil {
		return nil
	}
	for i := range df.Topics {
		if df.Topics[i].Key == key {
			return &df.Topics[i]
		}
	}
	return nil
}

func (df *DialogueFile) heard(t *Topic, ps *PlayerState) bool {
	return ps != nil && ps.HasHeard != nil && ps.HasHeard(HeardKey(df.MobId, t.Key))
}

// Eligible reports whether t is open to the player now: every gate passes
// and every heardRequired key has been heard. Mood never gates a topic. A
// nil PlayerState skips the checks; a nil HasHeard fails heardRequired
// closed, the way every interrogative callback in this package does.
func (df *DialogueFile) Eligible(t *Topic, ps *PlayerState) bool {
	if df == nil || t == nil || !t.Gates.pass(ps) {
		return false
	}
	if ps == nil {
		return true
	}
	if t.Kind == KindOffer && t.Quest > 0 && (ps.QuestOpen == nil || !ps.QuestOpen(t.Quest)) {
		return false
	}
	for _, k := range t.HeardRequired {
		if ps.HasHeard == nil || !ps.HasHeard(HeardKey(df.MobId, k)) {
			return false
		}
	}
	return true
}

// TopicMarker is the marker on an eligible topic's menu line.
func (df *DialogueFile) TopicMarker(t *Topic, ps *PlayerState) string {
	if df == nil || t == nil {
		return ""
	}
	switch t.Kind {
	case KindHandIn:
		return menu.MarkerHandIn
	case KindOffer:
		return menu.MarkerQuest
	}
	if t.Kind == KindInfo && ps != nil && ps.Objective != nil && ps.Objective(df.MobId, t.Key) {
		return menu.MarkerInfo
	}
	if t.Marked && !df.heard(t, ps) {
		return menu.MarkerInfo
	}
	return ""
}

// Marker is the marker before the NPC's name for this player, or "".
func (df *DialogueFile) Marker(ps *PlayerState) string {
	if df == nil {
		return ""
	}
	out := ""
	for i := range df.Topics {
		t := &df.Topics[i]
		if !df.Eligible(t, ps) {
			continue
		}
		if m := df.TopicMarker(t, ps); markerRank[m] > markerRank[out] {
			out = m
		}
	}
	return out
}

// OpenerFor is the opener for this player: the first variant whose gates
// pass, else the root text.
func (df *DialogueFile) OpenerFor(ps *PlayerState) (text, hints string) {
	if df == nil || df.Opener == nil {
		return "", ""
	}
	for _, v := range df.Opener.Variants {
		if v.Gates.pass(ps) {
			return v.Text, v.Hints
		}
	}
	return df.Opener.Text, df.Opener.Hints
}

// Breadcrumbs are the lines the NPC says after the opener for offers hidden
// by their gates. An offer whose quest the player already holds never
// breadcrumbs: "come back later" is for quests not yet open.
func (df *DialogueFile) Breadcrumbs(ps *PlayerState) []string {
	if df == nil {
		return nil
	}
	var out []string
	for i := range df.Topics {
		t := &df.Topics[i]
		if t.Kind != KindOffer || t.Breadcrumb == "" || df.Eligible(t, ps) {
			continue
		}
		if ps != nil && ps.HasQuest != nil && t.GrantsQuest != "" && ps.HasQuest(t.GrantsQuest) {
			continue
		}
		out = append(out, t.Breadcrumb)
	}
	return out
}

// Menu builds the talk menu: eligible offers and hand-ins first, then
// eligible info topics, each group in authored order. Heard info is Dim.
func (df *DialogueFile) Menu(npcName string, ps *PlayerState) menu.Menu {
	m := menu.Menu{Heading: "Talking with " + npcName, Leave: "Leave"}
	if df == nil {
		return m
	}
	var quests, info []menu.Entry
	for i := range df.Topics {
		t := &df.Topics[i]
		if !df.Eligible(t, ps) {
			continue
		}
		e := menu.Entry{Id: t.Key, Marker: df.TopicMarker(t, ps), Label: t.Label}
		if t.Kind == KindInfo {
			e.Dim = df.heard(t, ps)
			info = append(info, e)
			continue
		}
		e.Detail = t.PlayerLine
		quests = append(quests, e)
	}
	m.Entries = append(quests, info...)
	return m
}

// ItemTopic is the first eligible topic wanting itemId, in authored order,
// and whether any topic on this NPC wants that item at all. A topic wants an
// item through its requiresItem (handed over) or its carriesItem (shown, never
// taken: Apply leaves it with the player). A wanted item with no eligible
// topic is handed back by give (R8).
func (df *DialogueFile) ItemTopic(itemId int, ps *PlayerState) (t *Topic, wanted bool) {
	if df == nil || itemId <= 0 {
		return nil, false
	}
	for i := range df.Topics {
		tp := &df.Topics[i]
		if tp.RequiresItem != itemId && tp.CarriesItem != itemId {
			continue
		}
		wanted = true
		if df.Eligible(tp, ps) {
			return tp, true
		}
	}
	return nil, wanted
}

// Apply takes a chosen topic's price and runs its effects in the engine's
// fixed order (delivery first, then the rest; see applyQuestEffects), and
// records an info topic as heard once its effects landed. It reports false,
// with nothing changed and any charge refunded, when the price could not be
// taken or an item could not be delivered. It does not check eligibility:
// the caller re-checks right before. A topic's moodChange is the caller's job
// (dialogue.ShiftMood), because Apply has no mob instance.
func (df *DialogueFile) Apply(t *Topic, ps *PlayerState) bool {
	if df == nil || t == nil {
		return false
	}
	charge, give, gold := t.ChargesGold, t.GivesItem, t.GivesGold
	if resuming(t, ps) {
		// No handout with resale value on a resume (owner ruling 2).
		charge, gold = 0, 0
		if !resumeGivesItem(give, ps) {
			give = 0
		}
	}
	if !tryCharge(charge, ps) {
		return false
	}
	if !applyPaidEffects(charge, t.GrantsQuest, t.RequiresItem, give,
		t.SetsQuestFlag, t.BumpsRep, gold, ps) {
		return false
	}
	if t.Kind == KindInfo && ps != nil && ps.MarkHeard != nil {
		ps.MarkHeard(HeardKey(df.MobId, t.Key))
	}
	return true
}
