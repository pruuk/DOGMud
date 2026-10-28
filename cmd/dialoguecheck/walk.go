package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// Finding classes, in report order. The first four are errors.
const (
	ClassValidation  = "VALIDATION"
	ClassDeadEnd     = "DEAD END"
	ClassReOffer     = "RE-OFFER"
	ClassUnstartable = "UNSTARTABLE QUEST"
	ClassOrphan      = "ORPHAN STEP"
	ClassUnreachable = "UNREACHABLE TOPIC"
	ClassAssumed     = "ASSUMED"
	ClassRetired     = "RETIRED"
)

var classOrder = []string{ClassValidation, ClassDeadEnd, ClassReOffer, ClassUnstartable, ClassOrphan, ClassUnreachable, ClassAssumed, ClassRetired}

func isError(class string) bool {
	switch class {
	case ClassValidation, ClassDeadEnd, ClassReOffer, ClassUnstartable:
		return true
	}
	return false
}

// Finding is one report line. Zones are the zone folders it belongs to;
// none means it is shown under every filter.
type Finding struct {
	Class string
	Zones []string
	Text  string
}

func (f Finding) String() string { return f.Class + ": " + f.Text }

// node is one quest's progress: the index of the current step (-1 for none)
// and whether the grant that put it there fires quest_granted triggers.
// Progress only moves forward (characters.GiveQuestToken) and HasQuest is
// true for the current step and every earlier one (characters.HasQuest).
type node struct {
	idx   int
	chain bool
}

var none = node{idx: -1}

type qmodel struct {
	q     quests.Quest
	steps []string
	idx   map[string]int
	end   int // index of the "end" step, -1 when there is none
	zones map[string]bool
}

// grantTo is where granting step from cur lands, mirroring
// quests.IsTokenAfter: from no progress only "start" (or the end of a
// one-step quest) lands; otherwise only a later step does.
func (m *qmodel) grantTo(cur int, step string) (int, bool) {
	j, ok := m.idx[step]
	if !ok {
		return 0, false
	}
	if cur < 0 {
		if step == "start" || (len(m.steps) == 1 && step == "end") {
			return j, true
		}
		return 0, false
	}
	return j, j > cur
}

func (m *qmodel) held(cur int, step string) bool {
	j, ok := m.idx[step]
	return ok && cur >= 0 && cur >= j
}

func (m *qmodel) token(i int) string {
	if i < 0 {
		return fmt.Sprintf("%d (no progress)", m.q.QuestId)
	}
	return fmt.Sprintf("%d-%s", m.q.QuestId, m.steps[i])
}

// source is anything that can fire for a player and change quest state: a
// topic, a quest trigger, a quest's completion rewards, or a behavior-tree
// branch.
type source struct {
	where    string
	zones    []string
	req, exc []string
	flagReq  map[string]string
	flagExc  map[string]string
	items    []int
	heard    []string
	// pin: the source fires only at the moment this token is granted
	// (quest_granted triggers, pinChain) or once it is held (rewards).
	pin      string
	pinChain bool
	assumed  []string
	dead     string // never fires, with the reason
	grants   []string
	chains   bool // its grants fire quest_granted triggers
	gives    []int
	flags    [][2]string
	heardKey string
	// via: topic sources whose choice fires this behavior branch.
	via      []int
	viaWhy   string
	isTopic  bool
	topic    *dialogue.Topic
	mobId    int
	ownerQ   int // the quest a trigger or reward belongs to
	fired    bool
	selfToks map[string]bool // tokens this source grants (not consumers of themselves)
}

type edge struct {
	from, to node
	src      int
}

type walker struct {
	in        Input
	qs        map[int]*qmodel
	srcs      []*source
	reach     map[int]map[node]bool
	edges     map[int][]edge
	given     map[int]bool // items a fired source hands out
	flags     map[string]bool
	heard     map[string]bool
	mobFolder map[int]string
	refs      map[string]map[int]bool // token -> sources (or -1 for openers and trees) that gate on it
	findings  []Finding
}

func parseToken(tok string) (int, string, bool) {
	i := strings.Index(tok, "-")
	if i <= 0 {
		return 0, "", false
	}
	id, err := strconv.Atoi(tok[:i])
	if err != nil {
		return 0, "", false
	}
	return id, tok[i+1:], true
}

// Walk runs the gate walk and returns its findings (validation problems are
// added by the caller).
func Walk(in Input) []Finding {
	w := newWalker(in)
	w.fixpoint()
	w.check()
	return w.findings
}

func newWalker(in Input) *walker {
	w := &walker{
		in: in, qs: map[int]*qmodel{}, reach: map[int]map[node]bool{}, edges: map[int][]edge{},
		given: map[int]bool{}, flags: map[string]bool{}, heard: map[string]bool{},
		mobFolder: map[int]string{}, refs: map[string]map[int]bool{},
	}
	for _, q := range in.Quests {
		m := &qmodel{q: q, idx: map[string]int{}, end: -1, zones: map[string]bool{}}
		for i, s := range q.Steps {
			m.steps = append(m.steps, s.Id)
			m.idx[s.Id] = i
			if s.Id == "end" {
				m.end = i
			}
		}
		w.qs[q.QuestId] = m
		w.reach[q.QuestId] = map[node]bool{none: true}
		if in.Retired[q.QuestId] {
			// Nothing grants a retired quest's start, but characters saved
			// holding it exist: they stand at its first step.
			if j, ok := m.idx["start"]; ok {
				w.reach[q.QuestId][node{idx: j, chain: true}] = true
			}
		}
	}
	for _, f := range in.Files {
		w.mobFolder[f.File.MobId] = f.Folder
	}
	w.buildTopics()
	w.buildTriggers()
	w.buildTrees()
	w.buildItemAndCodeGrants()
	w.assignQuestZones()
	return w
}

func (w *walker) addRef(tok string, src int) {
	if w.refs[tok] == nil {
		w.refs[tok] = map[int]bool{}
	}
	w.refs[tok][src] = true
}

func (w *walker) add(s *source) int {
	s.selfToks = map[string]bool{}
	for _, g := range s.grants {
		s.selfToks[g] = true
	}
	w.srcs = append(w.srcs, s)
	i := len(w.srcs) - 1
	for _, t := range append(append([]string{}, s.req...), s.exc...) {
		if !s.selfToks[t] {
			w.addRef(t, i)
		}
	}
	if s.pin != "" {
		w.addRef(s.pin, i)
	}
	return i
}

func (w *walker) buildTopics() {
	for _, f := range w.in.Files {
		df := f.File
		if df.Opener != nil {
			for _, v := range df.Opener.Variants {
				for _, t := range append(append([]string{}, v.QuestRequired...), v.QuestExcluded...) {
					w.addRef(t, -1)
				}
			}
		}
		for i := range df.Topics {
			t := &df.Topics[i]
			s := &source{
				where:   fmt.Sprintf("mob %d topic %q", df.MobId, t.Key),
				zones:   []string{f.Folder},
				req:     t.QuestRequired,
				exc:     t.QuestExcluded,
				flagReq: t.QuestFlagRequired,
				flagExc: t.QuestFlagExcluded,
				chains:  true,
				isTopic: true,
				topic:   t,
				mobId:   df.MobId,
			}
			for _, id := range []int{t.RequiresItem, t.CarriesItem} {
				if id > 0 {
					s.items = append(s.items, id)
				}
			}
			for _, k := range t.HeardRequired {
				s.heard = append(s.heard, dialogue.HeardKey(df.MobId, k))
			}
			if t.GoldRequired > 0 || t.ChargesGold > 0 {
				s.assumed = append(s.assumed, "gold not modeled (goldRequired/chargesGold)")
			}
			if t.MasterworkRequired > 0 {
				s.assumed = append(s.assumed, "masterworkRequired not modeled")
			}
			if t.GrantsQuest != "" {
				s.grants = []string{t.GrantsQuest}
			}
			if t.GivesItem > 0 {
				s.gives = []int{t.GivesItem}
			}
			if t.SetsQuestFlag != nil {
				s.flags = [][2]string{{t.SetsQuestFlag.Key, t.SetsQuestFlag.Value}}
			}
			if t.Kind == dialogue.KindInfo {
				s.heardKey = dialogue.HeardKey(df.MobId, t.Key)
			}
			w.add(s)
		}
	}
}

func actionEffects(acts []quests.ActionDef, s *source) {
	for _, a := range acts {
		if a.Grant != "" {
			s.grants = append(s.grants, a.Grant)
		}
		if a.GiveItem > 0 {
			s.gives = append(s.gives, a.GiveItem)
		}
		if a.SpawnItem != nil && a.SpawnItem.Id > 0 {
			s.gives = append(s.gives, a.SpawnItem.Id)
		}
		if a.SetFlag != nil {
			s.flags = append(s.flags, [2]string{a.SetFlag.Key, a.SetFlag.Value})
		}
		if a.Sequence != nil {
			actionEffects(a.Sequence.OnComplete, s)
		}
	}
}

func (w *walker) buildTriggers() {
	ids := make([]int, 0, len(w.qs))
	for id := range w.qs {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		q := w.qs[id].q
		for i, t := range q.Triggers {
			if w.in.Plan != nil && w.in.Plan.DeleteTriggers[id][i] {
				continue
			}
			s := &source{
				where:   fmt.Sprintf("quest %d trigger %d (%s)", id, i, t.Event),
				ownerQ:  id,
				req:     t.Conditions.Has,
				exc:     t.Conditions.Missing,
				flagReq: t.Conditions.HasFlag,
				flagExc: t.Conditions.MissingFlag,
				chains:  true,
				mobId:   t.Mob,
			}
			switch t.Event {
			case "item_give", "item_gain":
				if t.Item > 0 {
					s.items = append(s.items, t.Item)
				}
			case "quest_granted":
				s.pin, s.pinChain = t.QuestToken, true
			case "dialogue":
				s.dead = "event dialogue: the ask chain that fired it is gone once talk topics replace ask (Task 19)"
			default:
				s.assumed = append(s.assumed, fmt.Sprintf("event %s not modeled (room, mob, command or time)", t.Event))
			}
			if t.Conditions.HasItem > 0 {
				s.items = append(s.items, t.Conditions.HasItem)
			}
			if t.Conditions.HasGold > 0 {
				s.assumed = append(s.assumed, "has_gold not modeled")
			}
			if t.Conditions.HasMasterwork > 0 {
				s.assumed = append(s.assumed, "has_masterwork not modeled")
			}
			if t.Conditions.InRoom > 0 {
				s.assumed = append(s.assumed, "in_room not modeled")
			}
			actionEffects(t.Actions, s)
			w.add(s)
		}
		r := q.Rewards
		rs := &source{where: fmt.Sprintf("quest %d rewards", id), ownerQ: id, pin: fmt.Sprintf("%d-end", id), chains: true}
		if r.QuestId != "" {
			rs.grants = append(rs.grants, r.QuestId)
		}
		if r.ItemId > 0 {
			rs.gives = append(rs.gives, r.ItemId)
		}
		for _, part := range strings.Split(r.ItemInfo, ",") {
			if n, err := strconv.Atoi(strings.TrimSpace(strings.SplitN(part, ":", 2)[0])); err == nil && n > 0 {
				rs.gives = append(rs.gives, n)
			}
		}
		if len(rs.grants)+len(rs.gives) > 0 {
			w.add(rs)
		}
	}
}

// branch is one group of quest-relevant actions in a behavior tree and the
// event and conditions in front of it.
type branch struct {
	event string
	askN  int
	conds []behaviortree.NodeDef
	acts  []behaviortree.NodeDef
}

var questActions = map[string]bool{"grant_quest": true, "grant_quest_to_user": true, "give_item": true, "set_quest_flag": true}

func paramStr(p map[string]any, k string) string {
	if v, ok := p[k]; ok && v != nil {
		return strings.TrimSpace(fmt.Sprint(v))
	}
	return ""
}

func paramInt(p map[string]any, k string) int {
	n, _ := strconv.Atoi(paramStr(p, k))
	return n
}

// treeBranches finds every quest-relevant action group. player_ask
// branches are numbered in preorder, 1-based, as the converter numbers
// them, without counting one nested inside another.
func treeBranches(root behaviortree.NodeDef) []branch {
	var out []branch
	askCount := 0
	var walk func(n behaviortree.NodeDef, ctx branch)
	walk = func(n behaviortree.NodeDef, ctx branch) {
		if n.Event != "" {
			ctx.event = n.Event
			if n.Event == "player_ask" && ctx.askN == 0 {
				askCount++
				ctx.askN = askCount
			}
		}
		switch n.Type {
		case "sequence":
			conds := append([]behaviortree.NodeDef(nil), ctx.conds...)
			var group *branch
			for _, c := range n.Children {
				switch {
				case c.Type == "condition" && c.Event == "":
					conds = append(conds, c)
					group = nil
				case c.Type == "decorator" && c.Event == "" && c.Child != nil && c.Child.Type == "condition":
					conds = append(conds, behaviortree.NodeDef{Type: "condition", Check: "decorator " + c.Mod + " over " + c.Child.Check})
					group = nil
				case c.Type == "action" && c.Event == "" && questActions[c.Do]:
					if group == nil {
						out = append(out, branch{event: ctx.event, askN: ctx.askN, conds: append([]behaviortree.NodeDef(nil), conds...)})
						group = &out[len(out)-1]
					}
					group.acts = append(group.acts, c)
				default:
					sub := ctx
					sub.conds = conds
					walk(c, sub)
				}
			}
		case "selector":
			for _, c := range n.Children {
				walk(c, ctx)
			}
		case "decorator":
			if n.Child != nil {
				walk(*n.Child, ctx)
			}
		case "action":
			if questActions[n.Do] {
				b := ctx
				b.acts = []behaviortree.NodeDef{n}
				out = append(out, b)
			}
		}
	}
	walk(root, branch{})
	return out
}

func (w *walker) topicSources(mobId int, key string) (idx []int, notBehavior bool) {
	for i, s := range w.srcs {
		if s.isTopic && s.mobId == mobId && s.topic.Key == key {
			if s.topic.Behavior {
				idx = append(idx, i)
			} else {
				notBehavior = true
			}
		}
	}
	return idx, notBehavior
}

func (w *walker) buildTrees() {
	for _, tf := range w.in.Trees {
		for _, b := range treeBranches(tf.Def.Tree) {
			s := &source{where: fmt.Sprintf("%s (%s branch)", tf.Path, orNone(b.event)), mobId: tf.MobId}
			if tf.Zone != "" {
				s.zones = []string{tf.Zone}
			}
			var topicKeys []string
			removed := map[string]bool{}
			for _, c := range b.conds {
				switch c.Check {
				case "player_has_quest":
					s.req = append(s.req, paramStr(c.Params, "quest"))
				case "player_missing_quest":
					s.exc = append(s.exc, paramStr(c.Params, "quest"))
				case "topic_is":
					topicKeys = append(topicKeys, behaviortree.TopicIsKeys(c.Params)...)
				case "item_matches", "player_has_item":
					if id := paramInt(c.Params, "item_id"); id > 0 {
						s.items = append(s.items, id)
					}
				case "player_has_flag":
					if s.flagReq == nil {
						s.flagReq = map[string]string{}
					}
					s.flagReq[paramStr(c.Params, "flag_key")] = paramStr(c.Params, "flag_value")
				case "keyword_match":
					if b.event != "player_ask" {
						s.assumed = append(s.assumed, "check keyword_match not modeled")
					}
				default:
					s.assumed = append(s.assumed, fmt.Sprintf("check %s not modeled", c.Check))
				}
			}
			switch b.event {
			case "player_topic":
				if len(topicKeys) == 0 {
					s.dead = "player_topic branch with no topic_is check"
				}
			case "player_ask":
				s.where = fmt.Sprintf("%s (player_ask branch %d)", tf.Path, b.askN)
				var bp branchPlan
				ok := false
				if w.in.Plan != nil {
					bp, ok = w.in.Plan.Branches[tf.Path][b.askN]
				}
				if !ok {
					s.dead = "player_ask never fires once talk topics replace ask (Task 19), and no converter report rewrites this branch"
				} else {
					topicKeys = append(topicKeys, bp.Topics...)
					for _, g := range bp.RemoveGrants {
						removed[g] = true
					}
					s.assumed = append(s.assumed, fmt.Sprintf("read as Task 17 will rewrite it: player_topic with topic_is %s", strings.Join(bp.Topics, ", ")))
				}
			case "player_give":
			default:
				s.assumed = append(s.assumed, fmt.Sprintf("event %s not modeled", orNone(b.event)))
			}
			for _, a := range b.acts {
				switch a.Do {
				case "grant_quest", "grant_quest_to_user":
					if g := paramStr(a.Params, "quest"); g != "" && !removed[g] {
						s.grants = append(s.grants, g)
					}
				case "give_item":
					if id := paramInt(a.Params, "item_id"); id > 0 {
						s.gives = append(s.gives, id)
					}
				case "set_quest_flag":
					s.flags = append(s.flags, [2]string{paramStr(a.Params, "flag_key"), paramStr(a.Params, "flag_value")})
				}
			}
			if s.dead == "" && len(topicKeys) > 0 {
				if tf.MobId == 0 {
					s.dead = "topic_is in a room tree: no NPC topic fires it"
				}
				var whyNot []string
				for _, k := range topicKeys {
					idx, notB := w.topicSources(tf.MobId, k)
					s.via = append(s.via, idx...)
					if len(idx) == 0 {
						if notB {
							whyNot = append(whyNot, fmt.Sprintf("topic %q on mob %d is not behavior: true", k, tf.MobId))
						} else {
							whyNot = append(whyNot, fmt.Sprintf("mob %d has no topic %q", tf.MobId, k))
						}
					}
				}
				if s.dead == "" && len(s.via) == 0 {
					s.dead = strings.Join(whyNot, "; ")
				}
				if len(s.via) > 0 {
					s.viaWhy = strings.Join(topicKeys, ", ")
					s.assumed = append(s.assumed, "the firing topic's quest gates and the branch's are checked separately (the topic's effects land first)")
				}
			}
			// Tree conditions on quest tokens are consumers even on branches
			// that grant nothing, so treeRefs walks the whole tree for them.
			if len(s.grants)+len(s.gives)+len(s.flags) > 0 {
				w.add(s)
			}
		}
		w.treeRefs(tf.Def.Tree)
	}
}

// buildItemAndCodeGrants adds the two token sources outside dialogue,
// quests and trees: an item whose spec carries a quest token grants it when
// gained, and Go code grants a few tokens by literal at character creation.
func (w *walker) buildItemAndCodeGrants() {
	ids := make([]int, 0, len(w.in.ItemTokens))
	for id := range w.in.ItemTokens {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		w.add(&source{
			where:  fmt.Sprintf("item %d (questtoken %s)", id, w.in.ItemTokens[id]),
			items:  []int{id},
			grants: []string{w.in.ItemTokens[id]},
			chains: true,
		})
	}
	for _, g := range w.in.CodeGrants {
		w.add(&source{
			where:   fmt.Sprintf("Go code %s", g.Where),
			grants:  []string{g.Token},
			chains:  true,
			assumed: []string{"a Go-coded grant (character creation or a command) is taken as available to some fresh character"},
		})
	}
}

func (w *walker) treeRefs(n behaviortree.NodeDef) {
	switch n.Check {
	case "player_has_quest", "player_missing_quest", "player_in_room_has_quest", "player_in_room_missing_quest":
		if q := paramStr(n.Params, "quest"); q != "" {
			w.addRef(q, -1)
		}
	}
	for _, c := range n.Children {
		w.treeRefs(c)
	}
	if n.Child != nil {
		w.treeRefs(*n.Child)
	}
}

func orNone(s string) string {
	if s == "" {
		return "no event"
	}
	return s
}

// assignQuestZones: a quest is in a zone when a topic there offers, hands
// in or grants it, or a tree or give trigger there grants it.
func (w *walker) assignQuestZones() {
	for _, s := range w.srcs {
		qids := map[int]bool{}
		if s.isTopic && s.topic.Quest > 0 {
			qids[s.topic.Quest] = true
		}
		for _, g := range s.grants {
			if id, _, ok := parseToken(g); ok {
				qids[id] = true
			}
		}
		zones := s.zones
		if !s.isTopic && s.mobId > 0 && w.mobFolder[s.mobId] != "" && len(zones) == 0 {
			zones = []string{w.mobFolder[s.mobId]}
		}
		for id := range qids {
			if m := w.qs[id]; m != nil {
				for _, z := range zones {
					m.zones[z] = true
				}
			}
		}
	}
	// A quest no NPC offers or hands in belongs to the zones of the rooms
	// its triggers watch and of its steps' destinations (room or NPC).
	for _, m := range w.qs {
		if len(m.zones) > 0 {
			continue
		}
		for _, t := range m.q.Triggers {
			for _, r := range []int{t.Room, t.Conditions.InRoom} {
				if z := w.in.RoomZones[r]; z != "" {
					m.zones[z] = true
				}
			}
		}
		for _, st := range m.q.Steps {
			if z := w.in.RoomZones[st.Destination.Room]; z != "" {
				m.zones[z] = true
			}
			for _, mob := range st.Destination.Mobs {
				if z := w.mobFolder[mob]; z != "" {
					m.zones[z] = true
				}
			}
		}
	}
	for _, s := range w.srcs {
		if len(s.zones) > 0 {
			continue
		}
		// A trigger or reward belongs to its quest's zones; an item or Go
		// code grant to the zones of the quests it grants.
		zs := map[string]bool{}
		qids := []int{s.ownerQ}
		if s.ownerQ == 0 {
			for _, t := range s.grants {
				if id, _, ok := parseToken(t); ok {
					qids = append(qids, id)
				}
			}
		}
		for _, id := range qids {
			if w.qs[id] != nil {
				for z := range w.qs[id].zones {
					zs[z] = true
				}
			}
		}
		if s.mobId > 0 && w.mobFolder[s.mobId] != "" {
			zs[w.mobFolder[s.mobId]] = true
		}
		s.zones = sortedKeys(zs)
		if len(s.zones) == 0 {
			s.zones = []string{unzoned}
		}
	}
	for _, m := range w.qs {
		if len(m.zones) == 0 {
			m.zones[unzoned] = true
		}
	}
}

// unzoned marks a finding no zone folder claims. A zone filter hides it;
// the whole-world report shows it.
const unzoned = "(no zone)"

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (w *walker) haveItem(id int) bool {
	return w.given[id] || len(w.in.WorldItems[id]) > 0
}

// eval reports whether s can fire with the quests in fixed at those nodes
// and every other quest at any reachable node. why names the first unmet
// gate.
func (w *walker) eval(s *source, fixed map[int]node) (bool, string) {
	type qg struct{ req, exc []string }
	byQ := map[int]*qg{}
	get := func(id int) *qg {
		if byQ[id] == nil {
			byQ[id] = &qg{}
		}
		return byQ[id]
	}
	for _, t := range s.req {
		id, step, ok := parseToken(t)
		if !ok || w.qs[id] == nil {
			return false, fmt.Sprintf("questRequired %s names no quest", t)
		}
		if _, ok := w.qs[id].idx[step]; !ok {
			return false, fmt.Sprintf("questRequired %s names no step of quest %d", t, id)
		}
		get(id).req = append(get(id).req, step)
	}
	for _, t := range s.exc {
		id, step, ok := parseToken(t)
		if !ok || w.qs[id] == nil {
			continue // never held, never blocks
		}
		get(id).exc = append(get(id).exc, step)
	}
	ids := make([]int, 0, len(byQ))
	for id := range byQ {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		m, g := w.qs[id], byQ[id]
		passes := func(n node) bool {
			for _, st := range g.req {
				if !m.held(n.idx, st) {
					return false
				}
			}
			for _, st := range g.exc {
				if m.held(n.idx, st) {
					return false
				}
			}
			return true
		}
		if n, ok := fixed[id]; ok {
			if !passes(n) {
				return false, fmt.Sprintf("at %s: %s", m.token(n.idx), describeQG(id, g.req, g.exc))
			}
			continue
		}
		found := false
		maxIdx := -1
		for n := range w.reach[id] {
			if n.idx > maxIdx {
				maxIdx = n.idx
			}
			if passes(n) {
				found = true
				break
			}
		}
		if !found {
			for _, st := range g.req {
				if m.idx[st] > maxIdx {
					return false, fmt.Sprintf("questRequired %d-%s is never reached (quest %d gets no further than %s)", id, st, id, m.token(maxIdx))
				}
			}
			return false, fmt.Sprintf("no reachable progress of quest %d meets %s", id, describeQG(id, g.req, g.exc))
		}
	}
	for _, id := range s.items {
		if !w.haveItem(id) {
			return false, fmt.Sprintf("item %d: nothing reachable gives it and nothing in the world supplies it", id)
		}
	}
	for _, k := range sortedMapKeys(s.flagReq) {
		v := s.flagReq[k]
		// An empty value asks for the flag to be unset (GetQuestFlag returns
		// "" for a missing key), which every player starts with.
		if v != "" && !w.flags[k+"="+v] {
			return false, fmt.Sprintf("questFlagRequired %s=%s is never set", k, v)
		}
		if s.flagExc != nil && s.flagExc[k] == v {
			return false, fmt.Sprintf("questFlagRequired and questFlagExcluded both name %s=%s", k, v)
		}
	}
	for _, h := range s.heard {
		if !w.heard[h] {
			return false, fmt.Sprintf("heardRequired %s is never heard", h)
		}
	}
	if len(s.via) > 0 {
		any := false
		for _, v := range s.via {
			if w.srcs[v].fired {
				any = true
				break
			}
		}
		if !any {
			return false, fmt.Sprintf("fired only by topic %s, which never fires", s.viaWhy)
		}
	}
	return true, ""
}

func describeQG(id int, req, exc []string) string {
	var parts []string
	if len(req) > 0 {
		parts = append(parts, "needs "+joinTok(id, req))
	}
	if len(exc) > 0 {
		parts = append(parts, "excludes "+joinTok(id, exc))
	}
	return strings.Join(parts, ", ")
}

func joinTok(id int, steps []string) string {
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = fmt.Sprintf("%d-%s", id, s)
	}
	return strings.Join(out, " ")
}

func sortedMapKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// contexts lists the fixed-quest assignments s may fire under: one per
// reachable pin node, or one empty assignment.
func (w *walker) contexts(s *source) []map[int]node {
	if s.pin == "" {
		return []map[int]node{{}}
	}
	id, step, ok := parseToken(s.pin)
	m := w.qs[id]
	if !ok || m == nil {
		return nil
	}
	j, ok := m.idx[step]
	if !ok {
		return nil
	}
	var out []map[int]node
	for n := range w.reach[id] {
		if n.idx == j && (n.chain || !s.pinChain) {
			out = append(out, map[int]node{id: n})
		}
	}
	return out
}

// fire tries s under every context and applies what lands. It reports
// whether anything new became reachable.
func (w *walker) fire(si int) bool {
	s := w.srcs[si]
	if s.dead != "" {
		return false
	}
	changed := false
	byQ := map[int][]string{}
	var order []int
	for _, g := range s.grants {
		id, step, ok := parseToken(g)
		if !ok || w.qs[id] == nil {
			continue
		}
		if _, seen := byQ[id]; !seen {
			order = append(order, id)
		}
		byQ[id] = append(byQ[id], step)
	}
	fired := false
	for _, ctx := range w.contexts(s) {
		if len(order) == 0 {
			if ok, _ := w.eval(s, ctx); ok {
				fired = true
			}
			continue
		}
		for _, id := range order {
			m := w.qs[id]
			var starts []node
			if n, ok := ctx[id]; ok {
				starts = []node{n}
			} else {
				for n := range w.reach[id] {
					starts = append(starts, n)
				}
			}
			for _, n := range starts {
				f := map[int]node{}
				for k, v := range ctx {
					f[k] = v
				}
				f[id] = n
				if ok, _ := w.eval(s, f); !ok {
					continue
				}
				fired = true
				cur := n
				for _, st := range byQ[id] {
					j, ok := m.grantTo(cur.idx, st)
					if !ok {
						continue
					}
					next := node{idx: j, chain: s.chains}
					if !w.hasEdge(id, cur, next, si) {
						w.edges[id] = append(w.edges[id], edge{from: cur, to: next, src: si})
					}
					if !w.reach[id][next] {
						w.reach[id][next] = true
						changed = true
					}
					cur = next
				}
			}
		}
	}
	if fired {
		if !s.fired {
			s.fired = true
			changed = true
		}
		for _, it := range s.gives {
			if !w.given[it] {
				w.given[it] = true
				changed = true
			}
		}
		for _, fl := range s.flags {
			if k := fl[0] + "=" + fl[1]; !w.flags[k] {
				w.flags[k] = true
				changed = true
			}
		}
		if s.heardKey != "" && !w.heard[s.heardKey] {
			w.heard[s.heardKey] = true
			changed = true
		}
	}
	return changed
}

func (w *walker) hasEdge(id int, a, b node, si int) bool {
	for _, e := range w.edges[id] {
		if e.from == a && e.to == b && e.src == si {
			return true
		}
	}
	return false
}

func (w *walker) fixpoint() {
	for changed := true; changed; {
		changed = false
		for i := range w.srcs {
			if w.fire(i) {
				changed = true
			}
		}
	}
}

func (w *walker) emit(class string, zones []string, format string, a ...any) {
	w.findings = append(w.findings, Finding{Class: class, Zones: zones, Text: fmt.Sprintf(format, a...)})
}

func (w *walker) check() {
	ids := make([]int, 0, len(w.qs))
	for id := range w.qs {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		w.checkQuest(w.qs[id])
	}
	w.checkReOffers()
	w.checkOrphans()
	w.checkTopics()
	w.checkAssumed()
}

func (w *walker) checkQuest(m *qmodel) {
	id := m.q.QuestId
	zones := sortedKeys(m.zones)
	if w.in.Retired[id] {
		w.emit(ClassRetired, zones, "quest %d %q: not startable by design; holders finish through its triggers", id, m.q.Name)
	}
	started := false
	for n := range w.reach[id] {
		if n.idx >= 0 {
			started = true
		}
	}
	if !started {
		var why []string
		for _, s := range w.srcs {
			if s.dead != "" || !grantsStep(s, id, "start") {
				continue
			}
			_, reason := w.eval(s, map[int]node{id: none})
			if len(w.contexts(s)) == 0 {
				reason = "fires when " + s.pin + " is granted, which never happens"
			}
			why = append(why, fmt.Sprintf("%s: %s", s.where, reason))
		}
		if len(why) > 0 {
			sort.Strings(why)
			w.emit(ClassUnstartable, zones, "quest %d %q: %d-start is never granted; %s", id, m.q.Name, id, strings.Join(why, "; "))
		} else if len(m.q.Triggers) > 0 || w.mentioned(id) {
			w.emit(ClassUnstartable, zones, "quest %d %q: nothing grants %d-start, yet its triggers or some topic expect it to run", id, m.q.Name, id)
		}
		return
	}
	if m.end < 0 {
		w.emit(ClassDeadEnd, zones, "quest %d %q has no end step", id, m.q.Name)
		return
	}
	// canEnd: nodes from which some path of edges reaches the end step.
	canEnd := map[node]bool{}
	for n := range w.reach[id] {
		if n.idx == m.end {
			canEnd[n] = true
		}
	}
	for grew := true; grew; {
		grew = false
		for _, e := range w.edges[id] {
			if canEnd[e.to] && !canEnd[e.from] {
				canEnd[e.from] = true
				grew = true
			}
		}
	}
	out := map[node]bool{}
	for _, e := range w.edges[id] {
		out[e.from] = true
	}
	stuck := map[int][]node{}
	for n := range w.reach[id] {
		if n.idx >= 0 && n.idx != m.end && !canEnd[n] && !out[n] {
			stuck[n.idx] = append(stuck[n.idx], n)
		}
	}
	idxs := make([]int, 0, len(stuck))
	for i := range stuck {
		idxs = append(idxs, i)
	}
	sort.Ints(idxs)
	for _, i := range idxs {
		ns := stuck[i]
		how := ""
		if len(ns) == 1 && !ns[0].chain && w.reach[id][node{idx: i, chain: true}] {
			how = " when it is granted by a behavior tree (no quest_granted chain fires)"
		}
		var next []string
		for _, s := range w.srcs {
			if s.dead != "" {
				continue
			}
			later := false
			for _, g := range s.grants {
				if gid, st, ok := parseToken(g); ok && gid == id {
					if j, ok := m.idx[st]; ok && j > i {
						later = true
					}
				}
			}
			if !later {
				continue
			}
			ok, reason := w.eval(s, map[int]node{id: ns[0]})
			if ok {
				reason = "its other gates or its pin never line up here"
			}
			next = append(next, fmt.Sprintf("%s: %s", s.where, reason))
		}
		sort.Strings(next)
		need := "nothing grants a later step of quest " + strconv.Itoa(id)
		if len(next) > 0 {
			need = "blocked: " + strings.Join(next, "; ")
		}
		w.emit(ClassDeadEnd, zones, "quest %d %q: a player can reach %s%s and never %d-end; %s", id, m.q.Name, m.token(i), how, id, need)
	}
}

// mentioned reports whether any topic names quest id (its quest field or a
// gate token).
func (w *walker) mentioned(id int) bool {
	for _, s := range w.srcs {
		if !s.isTopic {
			continue
		}
		if s.topic.Quest == id {
			return true
		}
		for _, t := range append(append([]string{}, s.req...), s.exc...) {
			if tid, _, ok := parseToken(t); ok && tid == id {
				return true
			}
		}
	}
	return false
}

func grantsStep(s *source, id int, step string) bool {
	for _, g := range s.grants {
		if gid, st, ok := parseToken(g); ok && gid == id && st == step {
			return true
		}
	}
	return false
}

// checkReOffers: an offer topic granting <q>-start must carry a quest q
// token in questExcluded. HasQuest is true for every step at or before the
// current one, so once <q>-end is held any q token excludes the offer; the
// skill's rule names <q>-end, and <q>-start works as well.
func (w *walker) checkReOffers() {
	for _, s := range w.srcs {
		if !s.isTopic || s.topic.GrantsQuest == "" {
			continue
		}
		id, step, ok := parseToken(s.topic.GrantsQuest)
		m := w.qs[id]
		if !ok || m == nil || step != "start" || m.q.Repeatable {
			continue
		}
		guarded := false
		for _, t := range s.exc {
			if eid, est, ok := parseToken(t); ok && eid == id {
				if _, valid := m.idx[est]; valid {
					guarded = true
				}
			}
		}
		if !guarded {
			w.emit(ClassReOffer, s.zones, "%s grants %d-start but questExcluded holds no quest %d token, so it is offered again after %d-end", s.where, id, id, id)
		}
	}
}

func (w *walker) checkOrphans() {
	granted := map[string][]string{}
	for _, s := range w.srcs {
		if s.dead != "" {
			continue
		}
		for _, g := range s.grants {
			granted[g] = append(granted[g], s.where)
		}
	}
	toks := make([]string, 0, len(granted))
	for t := range granted {
		toks = append(toks, t)
	}
	sort.Strings(toks)
	for _, t := range toks {
		id, step, ok := parseToken(t)
		m := w.qs[id]
		if !ok || m == nil || step == "end" {
			continue
		}
		if _, valid := m.idx[step]; !valid {
			continue // the validator names a token with no step
		}
		if len(w.refs[t]) > 0 {
			continue
		}
		srcs := granted[t]
		sort.Strings(srcs)
		w.emit(ClassOrphan, sortedKeys(m.zones), "%s (quest %d %q) is granted by %s but no gate, trigger or tree checks it", t, id, m.q.Name, strings.Join(srcs, ", "))
	}
}

func (w *walker) checkTopics() {
	for _, s := range w.srcs {
		if !s.isTopic || s.fired {
			continue
		}
		_, why := w.eval(s, map[int]node{})
		if why == "" {
			why = "no reachable progress meets its quest gates together"
		}
		w.emit(ClassUnreachable, s.zones, "%s (%s): %s", s.where, s.topic.Kind, why)
	}
}

func (w *walker) checkAssumed() {
	for _, s := range w.srcs {
		if s.dead != "" {
			if !s.isTopic && len(s.grants)+len(s.gives)+len(s.flags) > 0 {
				w.emit(ClassAssumed, s.zones, "%s never fires: %s", s.where, s.dead)
			}
			continue
		}
		for _, a := range s.assumed {
			w.emit(ClassAssumed, s.zones, "%s: %s", s.where, a)
		}
		for _, id := range s.items {
			if !w.given[id] && len(w.in.WorldItems[id]) > 0 {
				src := w.in.WorldItems[id]
				more := ""
				if len(src) > 1 {
					more = fmt.Sprintf(" (and %d more)", len(src)-1)
				}
				w.emit(ClassAssumed, s.zones, "%s: item %d assumed obtainable from %s%s", s.where, id, src[0], more)
			}
		}
		qs := map[int]bool{}
		for _, t := range s.req {
			if id, _, ok := parseToken(t); ok {
				qs[id] = true
			}
		}
		if s.isTopic && s.topic.Quest > 0 {
			qs[s.topic.Quest] = true
		}
		for _, t := range s.grants {
			if id, _, ok := parseToken(t); ok {
				qs[id] = true
			}
		}
		if len(s.req) > 0 {
			for _, t := range s.exc {
				if id, _, ok := parseToken(t); ok && !qs[id] {
					w.emit(ClassAssumed, s.zones, "%s: questExcluded %s taken as independent of its other quest gates (order across quests not modeled)", s.where, t)
				}
			}
		}
	}
}
