package dialogue

import "github.com/GoMudEngine/GoMud/internal/menu"

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
	for _, k := range t.HeardRequired {
		if ps.HasHeard == nil || !ps.HasHeard(HeardKey(df.MobId, k)) {
			return false
		}
	}
	return true
}

// TopicMarker is the marker on an eligible topic's menu line.
func (df *DialogueFile) TopicMarker(t *Topic, ps *PlayerState) string {
	switch t.Kind {
	case KindHandIn:
		return menu.MarkerHandIn
	case KindOffer:
		return menu.MarkerQuest
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

// ItemTopic is the first eligible topic wanting itemId (its requiresItem),
// and whether any topic on this NPC wants that item at all. A wanted item
// with no eligible topic is handed back by give (R8).
func (df *DialogueFile) ItemTopic(itemId int, ps *PlayerState) (t *Topic, wanted bool) {
	if df == nil || itemId <= 0 {
		return nil, false
	}
	for i := range df.Topics {
		tp := &df.Topics[i]
		if tp.RequiresItem != itemId {
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
// the caller re-checks right before.
func (df *DialogueFile) Apply(t *Topic, ps *PlayerState) bool {
	if df == nil || t == nil {
		return false
	}
	if !tryCharge(t.ChargesGold, ps) {
		return false
	}
	if !applyPaidEffects(t.ChargesGold, t.GrantsQuest, t.RequiresItem, t.GivesItem,
		t.SetsQuestFlag, t.BumpsRep, t.GivesGold, ps) {
		return false
	}
	if t.Kind == KindInfo && ps != nil && ps.MarkHeard != nil {
		ps.MarkHeard(HeardKey(df.MobId, t.Key))
	}
	return true
}
