package main

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// Grant sources, in report order. Each is one component of the
// reconciliation: the input side counts grants where the old content keeps
// them, the output side counts the grants the converted topics carry.
const (
	srcNodeStart = "node start"
	srcNodeEnd   = "node end"
	srcNodeMid   = "node mid"
	srcVariant   = "root variant"
	srcPattern   = "pattern"
	srcItemGive  = "item_give"
	srcDialogue  = "dialogue trigger"
	srcBehavior  = "behavior grant_quest"
)

var grantSources = []string{srcNodeStart, srcNodeEnd, srcNodeMid, srcVariant, srcPattern, srcItemGive, srcDialogue, srcBehavior}

// joinSep separates folded responses for the zone pass to merge.
const joinSep = "\n---\n"

// greetingWords are pattern keywords that only greet.
var greetingWords = map[string]bool{
	"hello": true, "hi": true, "greet": true, "greeting": true, "greetings": true,
	"hey": true, "hail": true, "howdy": true, "welcome": true,
	"good day": true, "good morning": true, "good evening": true,
}

// speechActions only speak; a behavior branch made of nothing else needs no
// tree once its text lives in a topic.
var speechActions = map[string]bool{"say": true, "respond": true, "send_user_text": true, "emote": true}

// patternClass is "catchall", "greeting", "overlap" or "other".
func patternClass(p legacyPattern, triggers map[string]bool) string {
	var kws []string
	for _, k := range p.Keywords {
		if k = strings.ToLower(strings.TrimSpace(k)); k != "" {
			kws = append(kws, k)
		}
	}
	if len(kws) == 0 {
		return "catchall"
	}
	allGreet := true
	for _, k := range kws {
		if !greetingWords[k] {
			allGreet = false
		}
	}
	if allGreet {
		return "greeting"
	}
	for _, k := range kws {
		if triggers[k] {
			return "overlap"
		}
	}
	return "other"
}

// kindOf classifies a node by its grant token: "-start" offer, "-end"
// hand-in, any other grant a marked info topic (a mid-quest step), none an
// unmarked info topic.
func kindOf(grant string) (kind string, questId int, marked bool) {
	if grant == "" {
		return dialogue.KindInfo, 0, false
	}
	id, step := quests.TokenToParts(grant)
	switch step {
	case "start":
		return dialogue.KindOffer, id, false
	case "end":
		return dialogue.KindHandIn, id, false
	}
	return dialogue.KindInfo, 0, true
}

// grantSourceOfNode names the reconciliation component of a tree node grant.
func grantSourceOfNode(grant string) string {
	_, step := quests.TokenToParts(grant)
	switch step {
	case "start":
		return srcNodeStart
	case "end":
		return srcNodeEnd
	}
	return srcNodeMid
}

// slug lowercases s and turns every character outside [a-z0-9_] into "_".
func slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// zoneNameSanitize matches internal/dialogue's loader: the folder a zone's
// dialogue files live in.
func zoneNameSanitize(zone string) string {
	return strings.ToLower(strings.ReplaceAll(zone, " ", "_"))
}

func endToken(questId int) string { return fmt.Sprintf("%d-end", questId) }

// appendUnique appends each value not already present.
func appendUnique(list []string, vals ...string) []string {
	for _, v := range vals {
		found := false
		for _, have := range list {
			if have == v {
				found = true
				break
			}
		}
		if !found {
			list = append(list, v)
		}
	}
	return list
}

func copyStrings(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return append([]string(nil), s...)
}

func copyMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// ---------------------------------------------------------------- input

type dialogueSource struct {
	Folder string // zone folder under dialogue/
	Path   string // for messages
	File   legacyFile
}

type questSource struct {
	File  string // basename, as Task 16 names it
	Quest quests.Quest
}

type behaviorSource struct {
	Path  string // relative to the data root, forward slashes
	MobId int
	Tree  btNode
}

// Input is everything the converter reads.
type Input struct {
	Dialogues []dialogueSource
	Quests    []questSource
	Behaviors []behaviorSource
	MobZones  map[int]string // mob template id to its spec's zone: field
}

func NewInput() *Input { return &Input{MobZones: map[int]string{}} }

// ---------------------------------------------------------------- output

// outputFile is the converted file. It has the shape of dialogue.DialogueFile
// with the old patterns, tree and memory fields left out: DialogueFile still
// declares those (without omitempty) until Task 21 deletes them, so
// marshalling it would write "patterns: []" and "memory:" into every file.
// The test loads each output back into dialogue.DialogueFile strictly.
type outputFile struct {
	MobId       int                 `yaml:"mobid"`
	Zone        string              `yaml:"zone"`
	DefaultMood string              `yaml:"defaultMood"`
	Greetings   []dialogue.Greeting `yaml:"greetings,omitempty"`
	Opener      *dialogue.Opener    `yaml:"opener,omitempty"`
	Topics      []dialogue.Topic    `yaml:"topics,omitempty"`
}

// cTopic is a topic under construction plus the grant sources mapped into it.
type cTopic struct {
	dialogue.Topic
	grants []string
}

// cFile is one mob's converted file.
type cFile struct {
	MobId       int
	Folder      string
	Zone        string
	DefaultMood string
	Greetings   []dialogue.Greeting
	Opener      *dialogue.Opener
	topics      []*cTopic
	keys        map[string]bool
	isNew       bool
	suffixed    []string // report lines for keys that took a suffix
}

// Output is the file as written.
func (f *cFile) Output() outputFile {
	out := outputFile{MobId: f.MobId, Zone: f.Zone, DefaultMood: f.DefaultMood, Greetings: f.Greetings, Opener: f.Opener}
	for _, t := range f.topics {
		out.Topics = append(out.Topics, t.Topic)
	}
	return out
}

// uniqueKey returns base, or base_2, base_3 ... when base is taken.
func (f *cFile) uniqueKey(base string) string {
	if base == "" {
		base = "topic"
	}
	key := base
	for n := 2; f.keys[key]; n++ {
		key = fmt.Sprintf("%s_%d", base, n)
	}
	f.keys[key] = true
	if key != base {
		f.suffixed = append(f.suffixed, fmt.Sprintf("mob %d: key %s (sibling %s)", f.MobId, key, base))
	}
	return key
}

func (f *cFile) add(t *cTopic) { f.topics = append(f.topics, t) }

func (f *cFile) find(key string) *cTopic {
	for _, t := range f.topics {
		if t.Key == key {
			return t
		}
	}
	return nil
}

// Result is the converted world and its report.
type Result struct {
	Files  map[int]*cFile
	Report *Report
}

// Topic returns the converted topic, or nil.
func (r *Result) Topic(mobId int, key string) *dialogue.Topic {
	f := r.Files[mobId]
	if f == nil {
		return nil
	}
	if t := f.find(key); t != nil {
		return &t.Topic
	}
	return nil
}

// Keys lists a mob's topic keys in file order.
func (r *Result) Keys(mobId int) []string {
	var keys []string
	if f := r.Files[mobId]; f != nil {
		for _, t := range f.topics {
			keys = append(keys, t.Key)
		}
	}
	return keys
}

// ---------------------------------------------------------------- convert

type converter struct {
	in      *Input
	res     *Result
	rep     *Report
	titles  map[int]string
	deleted map[string]bool // trigger delete lines already listed
}

// Convert applies the rules of plan Task 13 (spec section 6.1) in order,
// with the conversion rulings R15 to R21 (spec section 2).
func Convert(in *Input) *Result {
	c := &converter{in: in, rep: newReport(), titles: map[int]string{}, deleted: map[string]bool{}}
	c.res = &Result{Files: map[int]*cFile{}, Report: c.rep}
	for _, q := range in.Quests {
		c.titles[q.Quest.QuestId] = q.Quest.Name
	}
	c.rep.GrantsIn = countInputGrants(in)

	dlg := append([]dialogueSource(nil), in.Dialogues...)
	sort.Slice(dlg, func(i, j int) bool { return dlg[i].File.MobId < dlg[j].File.MobId })
	for _, d := range dlg {
		c.convertDialogue(d)
	}

	qs := append([]questSource(nil), in.Quests...)
	sort.Slice(qs, func(i, j int) bool { return qs[i].Quest.QuestId < qs[j].Quest.QuestId })
	for _, q := range qs {
		for i, t := range q.Quest.Triggers {
			switch t.Event {
			case "item_give":
				c.itemGive(q, i, t)
			case "dialogue":
				c.dialogueTrigger(q, i, t)
			}
		}
	}

	bs := append([]behaviorSource(nil), in.Behaviors...)
	sort.Slice(bs, func(i, j int) bool { return bs[i].Path < bs[j].Path })
	for _, b := range bs {
		c.convertBehavior(b)
	}

	c.finish()
	return c.res
}

// label is the menu label the converter can write: the quest title for an
// offer or hand-in, empty for info (the zone pass writes it).
func (c *converter) label(kind string, questId int, where string) string {
	if kind == dialogue.KindInfo {
		return ""
	}
	title, ok := c.titles[questId]
	if !ok {
		c.rep.Notes = append(c.rep.Notes, fmt.Sprintf("%s: quest %d not found, label left empty", where, questId))
	}
	return title
}

// newTopic builds a topic of the grant's kind.
func (c *converter) newTopic(key, grant, where string) *cTopic {
	kind, qid, marked := kindOf(grant)
	t := &cTopic{}
	t.Key = key
	t.Kind = kind
	t.Quest = qid
	t.Marked = marked
	t.Label = c.label(kind, qid, where)
	t.GrantsQuest = grant
	return t
}

// guardCreatedGrant puts the grant and the quest's end token in
// questExcluded on a topic the converter made up (not copied from a node),
// so a finished quest is never offered again.
func guardCreatedGrant(t *cTopic) {
	if t.GrantsQuest == "" {
		return
	}
	id, _ := quests.TokenToParts(t.GrantsQuest)
	t.QuestExcluded = appendUnique(t.QuestExcluded, t.GrantsQuest, endToken(id))
}

func (c *converter) fileFor(mobId int, why string) *cFile {
	if f := c.res.Files[mobId]; f != nil {
		return f
	}
	zone, ok := c.in.MobZones[mobId]
	if !ok || zone == "" {
		c.rep.Notes = append(c.rep.Notes, fmt.Sprintf("mob %d: no dialogue file and no mob spec with a zone: nothing written (%s)", mobId, why))
		return nil
	}
	f := &cFile{MobId: mobId, Folder: zoneNameSanitize(zone), Zone: zone, DefaultMood: "neutral", keys: map[string]bool{}, isNew: true}
	c.res.Files[mobId] = f
	c.rep.NewFiles = append(c.rep.NewFiles, fmt.Sprintf("dialogue/%s/%d.yaml (%s, zone %s, defaultMood neutral)", f.Folder, mobId, why, zone))
	return f
}

// ---------------------------------------------------------------- rules 1 to 5

func (c *converter) convertDialogue(d dialogueSource) {
	lf := d.File
	if c.res.Files[lf.MobId] != nil {
		c.rep.Notes = append(c.rep.Notes, fmt.Sprintf("mob %d: second dialogue file %s ignored", lf.MobId, d.Path))
		return
	}
	f := &cFile{MobId: lf.MobId, Folder: d.Folder, Zone: lf.Zone, DefaultMood: lf.DefaultMood, Greetings: lf.Greetings, keys: map[string]bool{}}
	c.res.Files[lf.MobId] = f

	triggers := map[string]bool{}
	nodeTopic := map[string]*cTopic{}
	var nodeTopics []*cTopic // aligned with nodes
	var nodes []legacyTreeNode
	if lf.Tree != nil {
		nodes = lf.Tree.Nodes
		for _, n := range nodes {
			for _, tr := range n.Triggers {
				if tr = strings.ToLower(strings.TrimSpace(tr)); tr != "" {
					triggers[tr] = true
				}
			}
		}
	}

	// Rule 1: nodes.
	for _, n := range nodes {
		where := fmt.Sprintf("mob %d node %s", lf.MobId, n.Id)
		key := f.uniqueKey(slug(n.Id))
		t := c.newTopic(key, n.GrantsQuest, where)
		t.Text, t.Hints, t.MoodChange = n.Text, n.Hints, n.MoodChange
		t.Gates = nodeGates(n)
		t.GivesItem, t.SetsQuestFlag, t.BumpsRep = n.GivesItem, n.SetsQuestFlag, n.BumpsRep
		t.GivesGold, t.ChargesGold = n.GivesGold, n.ChargesGold
		if n.GrantsQuest != "" {
			t.grants = append(t.grants, grantSourceOfNode(n.GrantsQuest))
		}
		switch {
		case t.Kind == dialogue.KindOffer:
			c.rep.Nodes["offer"]++
		case t.Kind == dialogue.KindHandIn:
			c.rep.Nodes["handin"]++
		case t.Marked:
			c.rep.Nodes["info marked"]++
		default:
			c.rep.Nodes["info"]++
		}
		f.add(t)
		nodeTopics = append(nodeTopics, t)
		if _, dup := nodeTopic[n.Id]; !dup {
			nodeTopic[n.Id] = t
		}
	}

	// Rule 2: requires to heardRequired. The old engine passed a requires id
	// once the player had heard the node of that id OR any node that unlocks
	// it. A topic needs every heardRequired key, so one is picked: the
	// candidate earliest in file order, the one most likely heard first.
	for _, n := range nodes {
		if len(n.Requires) == 0 {
			continue
		}
		t := nodeTopic[n.Id]
		for _, req := range n.Requires {
			where := fmt.Sprintf("mob %d node %s requires %q", lf.MobId, n.Id, req)
			var cands []string // node ids, file order
			for _, u := range nodes {
				hit := u.Id == req
				for _, x := range u.Unlocks {
					if x == req {
						hit = true
					}
				}
				if hit {
					cands = appendUnique(cands, u.Id)
				}
			}
			if len(cands) == 0 {
				c.rep.Requires = append(c.rep.Requires, where+": no node has that id or unlocks it; nothing written")
				continue
			}
			target := nodeTopic[cands[0]]
			how := "node " + cands[0]
			if cands[0] != req {
				how = "unlocked by node " + cands[0]
			}
			if _, isNode := nodeTopic[req]; !isNode || len(cands) > 1 {
				line := fmt.Sprintf("%s: %s (key %s)", where, how, target.Key)
				if !isNode {
					line = fmt.Sprintf("%s: no node has that id; %s (key %s)", where, how, target.Key)
				}
				if len(cands) > 1 {
					line += fmt.Sprintf("; also reached through %v; picked the earliest in file order, zone pass to confirm", cands[1:])
				}
				c.rep.Requires = append(c.rep.Requires, line)
			}
			if target.Kind == dialogue.KindOffer && target.GrantsQuest != "" {
				// Settled ruling (mobs 95, 96): hearing an offer meant taking
				// it, so the gate is the quest the offer starts.
				t.QuestRequired = appendUnique(t.QuestRequired, target.GrantsQuest)
				c.rep.Requires = append(c.rep.Requires, fmt.Sprintf("%s: requires names an offer (%s, key %s); questRequired %s written (settled ruling: requires on an offer becomes a quest gate)", where, how, target.Key, target.GrantsQuest))
				continue
			}
			if target.Kind != dialogue.KindInfo {
				c.rep.Requires = append(c.rep.Requires, fmt.Sprintf("%s: requires names an offer (%s, key %s, kind %s); no heardRequired written", where, how, target.Key, target.Kind))
				continue
			}
			t.HeardRequired = appendUnique(t.HeardRequired, target.Key)
		}
	}

	// Rule 3: root and variants.
	if lf.Tree != nil {
		root := lf.Tree.Root
		f.Opener = &dialogue.Opener{Text: root.Text, Hints: root.Hints}
		for i, v := range root.Variants {
			gates := dialogue.Gates{
				QuestRequired: copyStrings(v.QuestRequired), QuestExcluded: copyStrings(v.QuestExcluded),
				QuestFlagRequired: copyMap(v.QuestFlagRequired), QuestFlagExcluded: copyMap(v.QuestFlagExcluded),
				ItemExcluded: v.ItemExcluded, GoldRequired: v.GoldRequired, MasterworkRequired: v.MasterworkRequired,
			}
			// The old Greet never checked requiresItem on a variant but did
			// take the item, so on a variant it is an effect.
			hasEffect := v.GrantsQuest != "" || v.GivesItem != 0 || v.RequiresItem != 0 || v.SetsQuestFlag != nil ||
				len(v.BumpsRep) > 0 || v.GivesGold != 0 || v.ChargesGold != 0
			if !hasEffect {
				f.Opener.Variants = append(f.Opener.Variants, dialogue.OpenerVariant{Gates: gates, Text: v.Text, Hints: v.Hints})
				continue
			}
			key := f.uniqueKey(fmt.Sprintf("variant_%d", i+1))
			where := fmt.Sprintf("mob %d root variant %d", lf.MobId, i+1)
			t := c.newTopic(key, v.GrantsQuest, where)
			t.Gates = gates
			t.RequiresItem = v.RequiresItem
			t.Text, t.Hints = v.Text, v.Hints
			t.GivesItem, t.SetsQuestFlag, t.BumpsRep = v.GivesItem, v.SetsQuestFlag, v.BumpsRep
			t.GivesGold, t.ChargesGold = v.GivesGold, v.ChargesGold
			if v.GrantsQuest != "" {
				t.grants = append(t.grants, srcVariant)
			}
			f.add(t)
			c.rep.Notes = append(c.rep.Notes, fmt.Sprintf("%s carried effects: left the opener and became topic %s (%s)", where, key, t.Kind))
		}
	}

	// Rule 4: patterns.
	firstGreeting := -1
	for i, p := range lf.Patterns {
		if patternClass(p, triggers) == "greeting" {
			firstGreeting = i
			break
		}
	}
	for i, p := range lf.Patterns {
		class := patternClass(p, triggers)
		c.rep.Patterns[class]++
		where := fmt.Sprintf("mob %d pattern %d %v", lf.MobId, i+1, p.Keywords)
		gated := patternExtras(p)
		all := strings.Join(p.Responses, joinSep)
		switch class {
		case "catchall":
			if lf.Tree == nil && firstGreeting < 0 && f.Opener == nil && all != "" {
				f.Opener = &dialogue.Opener{Text: all}
				c.rep.Dropped = append(c.rep.Dropped, where+": catchall, its responses became the opener (tree-less, no greeting)"+gated)
			} else {
				c.rep.Dropped = append(c.rep.Dropped, where+": catchall, dropped"+gated)
			}
		case "greeting":
			if lf.Tree == nil && i == firstGreeting && f.Opener == nil && all != "" {
				f.Opener = &dialogue.Opener{Text: all}
				c.rep.Dropped = append(c.rep.Dropped, where+": greeting, its responses became the opener (tree-less)"+gated)
			} else {
				c.rep.Dropped = append(c.rep.Dropped, where+": greeting, dropped"+gated)
			}
		case "overlap":
			c.overlapPattern(f, nodes, nodeTopics, triggers, p, where, gated)
		case "other":
			kw := firstKeyword(p, nil)
			c.patternTopic(f, p, slug(kw), p.Responses, where)
		}
	}
}

// overlapPattern converts a pattern that shares a keyword with a tree node.
// Under the old engine it answered whenever the tree did not: for its other
// keywords, and for a player outside the gates of every node it shares a
// keyword with. Its responses are compared (normText) with the text and
// hints of those nodes' topics:
//   - every response already there: dropped, the node says it;
//   - otherwise, when its keyword set equals one shared node's trigger set,
//     its gates equal that node's, the node has no requires and grants no
//     quest, and the pattern carries no effect and no different moodChange,
//     it is the same question: the missing responses are appended to that
//     node's topic (joinSep, for the zone pass to merge);
//   - otherwise it becomes its own info topic by the "other" path, keyed by
//     its first keyword that is not a tree trigger (else its first keyword),
//     with only the missing responses.
//
// Each kept pattern is listed in the report's OverlapKept.
func (c *converter) overlapPattern(f *cFile, nodes []legacyTreeNode, nodeTopics []*cTopic, triggers map[string]bool, p legacyPattern, where, gated string) {
	var shared, ungated []string
	var covers []string
	for i, n := range nodes {
		if !nodeSharesKeyword(n, p) {
			continue
		}
		shared = append(shared, n.Id)
		if !nodeGated(n) {
			ungated = append(ungated, n.Id)
		}
		covers = append(covers, normText(nodeTopics[i].Text), normText(nodeTopics[i].Hints))
	}
	why := fmt.Sprintf("overlap with a tree trigger (ungated node %v)", ungated)
	if len(ungated) == 0 {
		why = fmt.Sprintf("overlap, live fallback, breadcrumb candidate (every node sharing a keyword is gated: %v)", shared)
	}

	var missing []string
	for _, r := range p.Responses {
		nr := normText(r)
		if nr == "" {
			continue
		}
		found := false
		for _, cv := range covers {
			if strings.Contains(cv, nr) {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, r)
		}
	}
	if len(missing) == 0 {
		c.rep.OverlapOutcomes["covered"]++
		c.rep.Dropped = append(c.rep.Dropped, fmt.Sprintf("%s: %s, text already in node %v, dropped%s", where, why, shared, gated))
		return
	}
	partial := ""
	if len(missing) < len(p.Responses) {
		partial = fmt.Sprintf(", %d of %d responses (the rest are already in the node)", len(missing), len(p.Responses))
	}

	if i := sameQuestionNode(nodes, nodeTopics, p); i >= 0 {
		t := nodeTopics[i]
		parts := missing
		if t.Text != "" {
			parts = append([]string{t.Text}, missing...)
		}
		t.Text = strings.Join(parts, joinSep)
		c.rep.OverlapOutcomes["merged"]++
		c.rep.OverlapKept = append(c.rep.OverlapKept, fmt.Sprintf("%s: %s; same keyword set and gates as node %s, merged into topic %s%s", where, why, nodes[i].Id, t.Key, partial))
		return
	}

	key := c.patternTopic(f, p, slug(firstKeyword(p, triggers)), missing, where)
	c.rep.OverlapOutcomes["topic"]++
	near := ""
	if share := nearestShare(missing, covers); share >= nearShare {
		near = fmt.Sprintf("; close to the node's text (%.0f%% of its words), zone pass: check for a paraphrase", 100*share)
	}
	c.rep.OverlapKept = append(c.rep.OverlapKept, fmt.Sprintf("%s: %s; text not in the tree, kept as topic %s%s%s%s", where, why, key, partial, near, gated))
}

// nearShare is the share of a response's content words found in one node
// text above which a kept overlap is flagged as a likely paraphrase (the
// dropped-pattern audit's threshold).
const nearShare = 0.6

// stopWords are left out of the word share (the audit's list).
var stopWords = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields("a an the and or but if then of to in on at for with from by as is are was were be been am it its this that these those i you he she we they me my your his her our their us them him not no so do does did have has had will would can could shall should may might just very too also than there here what who whom which when where why how all any some into out up down over about one yes oh well now again still even only own same such more most much many few") {
		m[w] = true
	}
	return m
}()

func contentWords(norm string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(norm) {
		if len(w) > 1 && !stopWords[w] {
			m[w] = true
		}
	}
	return m
}

// nearestShare is the highest share, over the responses and the normalized
// covers, of a response's content words that one cover contains.
func nearestShare(responses, covers []string) float64 {
	best := 0.0
	for _, r := range responses {
		rw := contentWords(normText(r))
		if len(rw) == 0 {
			continue
		}
		for _, cv := range covers {
			cw := contentWords(cv)
			n := 0
			for w := range rw {
				if cw[w] {
					n++
				}
			}
			if s := float64(n) / float64(len(rw)); s > best {
				best = s
			}
		}
	}
	return best
}

// sameQuestionNode returns the index of the node an overlap pattern merges
// into (see overlapPattern), or -1.
func sameQuestionNode(nodes []legacyTreeNode, nodeTopics []*cTopic, p legacyPattern) int {
	if p.GrantsQuest != "" || p.GivesItem != 0 || p.SetsQuestFlag != nil || len(p.BumpsRep) > 0 || p.GivesGold != 0 || p.ChargesGold != 0 {
		return -1
	}
	kws := keywordSet(p.Keywords)
	for i, n := range nodes {
		t := nodeTopics[i]
		if !reflect.DeepEqual(kws, keywordSet(n.Triggers)) || len(n.Requires) > 0 || n.GrantsQuest != "" || t.Kind != dialogue.KindInfo {
			continue
		}
		if p.MoodChange != "" && p.MoodChange != n.MoodChange {
			continue
		}
		if reflect.DeepEqual(patternGates(p), nodeGates(n)) {
			return i
		}
	}
	return -1
}

func keywordSet(kws []string) map[string]bool {
	m := map[string]bool{}
	for _, k := range normKeywords(kws) {
		m[k] = true
	}
	return m
}

// firstKeyword is the pattern's first keyword that is not in skip, else its
// first keyword.
func firstKeyword(p legacyPattern, skip map[string]bool) string {
	first := ""
	for _, k := range p.Keywords {
		if strings.TrimSpace(k) == "" {
			continue
		}
		if first == "" {
			first = k
		}
		if !skip[strings.ToLower(strings.TrimSpace(k))] {
			return k
		}
	}
	return first
}

func patternGates(p legacyPattern) dialogue.Gates {
	return dialogue.Gates{
		QuestRequired: copyStrings(p.QuestRequired), QuestExcluded: copyStrings(p.QuestExcluded),
		QuestFlagRequired: copyMap(p.QuestFlagRequired), QuestFlagExcluded: copyMap(p.QuestFlagExcluded),
		RequiresItem: p.RequiresItem, ItemExcluded: p.ItemExcluded,
		GoldRequired: p.GoldRequired, MasterworkRequired: p.MasterworkRequired,
	}
}

func nodeGates(n legacyTreeNode) dialogue.Gates {
	return dialogue.Gates{
		QuestRequired: copyStrings(n.QuestRequired), QuestExcluded: copyStrings(n.QuestExcluded),
		QuestFlagRequired: copyMap(n.QuestFlagRequired), QuestFlagExcluded: copyMap(n.QuestFlagExcluded),
		RequiresItem: n.RequiresItem, ItemExcluded: n.ItemExcluded,
		GoldRequired: n.GoldRequired, MasterworkRequired: n.MasterworkRequired,
	}
}

// patternTopic makes a kept pattern an info topic (or the kind its grant
// names) carrying its gates and effects, with the given responses as text.
// It returns the key.
func (c *converter) patternTopic(f *cFile, p legacyPattern, base string, responses []string, where string) string {
	key := f.uniqueKey(base)
	t := c.newTopic(key, p.GrantsQuest, where)
	t.Text = strings.Join(responses, joinSep)
	t.MoodChange = p.MoodChange
	t.Gates = patternGates(p)
	t.GivesItem, t.SetsQuestFlag, t.BumpsRep = p.GivesItem, p.SetsQuestFlag, p.BumpsRep
	t.GivesGold, t.ChargesGold = p.GivesGold, p.ChargesGold
	if p.GrantsQuest != "" {
		t.grants = append(t.grants, srcPattern)
	}
	if len(p.Moods) > 0 {
		c.rep.Notes = append(c.rep.Notes, fmt.Sprintf("%s: moods filter %v dropped (topics are never gated by mood), topic %s", where, p.Moods, key))
	}
	if extra := nonGreetingKeywords(p); extra <= 1 && extra < countKeywords(p) {
		c.rep.FoldIntoOpener = append(c.rep.FoldIntoOpener, fmt.Sprintf("%s: a greeting plus %d other word; topic %s kept, the zone pass may fold it into the opener", where, extra, key))
	}
	f.add(t)
	return key
}

// patternExtras names everything a dropped pattern carried besides its
// keywords and responses, so the report shows what dropping it loses.
func patternExtras(p legacyPattern) string {
	var parts []string
	if len(p.Moods) > 0 {
		parts = append(parts, fmt.Sprintf("moods %v", p.Moods))
	}
	if p.MoodChange != "" {
		parts = append(parts, "moodChange "+p.MoodChange)
	}
	if len(p.QuestRequired) > 0 {
		parts = append(parts, fmt.Sprintf("questRequired %v", p.QuestRequired))
	}
	if len(p.QuestExcluded) > 0 {
		parts = append(parts, fmt.Sprintf("questExcluded %v", p.QuestExcluded))
	}
	if len(p.QuestFlagRequired) > 0 {
		parts = append(parts, fmt.Sprintf("questFlagRequired %v", p.QuestFlagRequired))
	}
	if len(p.QuestFlagExcluded) > 0 {
		parts = append(parts, fmt.Sprintf("questFlagExcluded %v", p.QuestFlagExcluded))
	}
	for _, x := range []struct {
		name string
		v    int
	}{{"requiresItem", p.RequiresItem}, {"itemExcluded", p.ItemExcluded}, {"givesItem", p.GivesItem}, {"givesGold", p.GivesGold},
		{"chargesGold", p.ChargesGold}, {"goldRequired", p.GoldRequired}, {"masterworkRequired", p.MasterworkRequired}} {
		if x.v != 0 {
			parts = append(parts, fmt.Sprintf("%s %d", x.name, x.v))
		}
	}
	if p.GrantsQuest != "" {
		parts = append(parts, "grantsQuest "+p.GrantsQuest)
	}
	if p.SetsQuestFlag != nil {
		parts = append(parts, fmt.Sprintf("setsQuestFlag %s=%s", p.SetsQuestFlag.Key, p.SetsQuestFlag.Value))
	}
	if len(p.BumpsRep) > 0 {
		parts = append(parts, fmt.Sprintf("bumpsRep %v", p.BumpsRep))
	}
	if len(parts) == 0 {
		return ""
	}
	return " (it carried: " + strings.Join(parts, "; ") + ")"
}

var markupTag = regexp.MustCompile(`<[^>]*>`)

// normText is the text a response is compared by: markup tags removed,
// lowercased, apostrophes dropped, every other character outside [a-z0-9]
// a space, runs of spaces collapsed. It matches the normalization of the
// dropped-pattern audit (measure.py) so the two agree on "already present".
func normText(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(markupTag.ReplaceAllString(s, "")) {
		switch {
		case r == '\'':
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		default:
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func normKeywords(kws []string) []string {
	var out []string
	for _, k := range kws {
		if k = strings.ToLower(strings.TrimSpace(k)); k != "" {
			out = append(out, k)
		}
	}
	return out
}

func countKeywords(p legacyPattern) int { return len(normKeywords(p.Keywords)) }

func nonGreetingKeywords(p legacyPattern) int {
	n := 0
	for _, k := range normKeywords(p.Keywords) {
		if !greetingWords[k] {
			n++
		}
	}
	return n
}

func nodeSharesKeyword(n legacyTreeNode, p legacyPattern) bool {
	trig := map[string]bool{}
	for _, t := range normKeywords(n.Triggers) {
		trig[t] = true
	}
	for _, k := range normKeywords(p.Keywords) {
		if trig[k] {
			return true
		}
	}
	return false
}

// nodeGated reports whether a node only answered some players.
func nodeGated(n legacyTreeNode) bool {
	return len(n.QuestRequired) > 0 || len(n.QuestExcluded) > 0 || len(n.QuestFlagRequired) > 0 || len(n.QuestFlagExcluded) > 0 ||
		n.RequiresItem != 0 || n.ItemExcluded != 0 || n.GoldRequired != 0 || n.MasterworkRequired != 0 || n.ChargesGold != 0 ||
		len(n.Requires) > 0
}

// ---------------------------------------------------------------- rule 6

// actionName names the one field an ActionDef sets.
func actionName(a quests.ActionDef) string {
	switch {
	case a.Grant != "":
		return "grant"
	case a.ConsumeItem != 0:
		return "consume_item"
	case a.GiveItem != 0:
		return "give_item"
	case a.GiveGold != 0:
		return "give_gold"
	case a.ChargeGold != 0:
		return "charge_gold"
	case a.NpcSay != nil:
		return "npc_say"
	case a.SendText != "":
		return "actor"
	case a.RoomText != "":
		return "observer"
	case a.SpawnMob != nil:
		return "spawn_mob"
	case a.SpawnItem != nil:
		return "spawn_item"
	case a.LockExits != nil:
		return "lock_exits"
	case a.UnlockExits != nil:
		return "unlock_exits"
	case a.TeachSpell != "":
		return "teach_spell"
	case a.TrainSkill != nil:
		return "train_skill"
	case a.TrainStat != nil:
		return "train_stat"
	case a.LearnRecipe != nil:
		return "learn_recipe"
	case a.ApplyStatusCondition != nil:
		return "apply_condition"
	case a.Teleport != 0:
		return "teleport"
	case a.GiveMutation:
		return "give_mutation"
	case a.SetFlag != nil:
		return "set_flag"
	case a.Sequence != nil:
		return "sequence"
	case a.BumpRep != nil:
		return "bump_rep"
	case a.DeclareBounty != nil:
		return "declare_bounty"
	}
	return "empty"
}

func npcSayText(s *quests.NpcSayDef) string {
	var parts []string
	for _, l := range s.Lines {
		if t := strings.Join(strings.Fields(l.Text), " "); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, " ")
}

// triggerWork collects what one trigger's conversion found to report, and
// decides whether the trigger may be deleted.
type triggerWork struct {
	c      *converter
	where  string // "quest <file> trigger <i>"
	held   []string
	mob    int
	npcSay []string
	give   bool     // an item_give trigger: its narration is dropped (R16)
	why    string   // reason appended to the delete line, when a ruling deleted it
	chain  []string // further delete lines (quest_granted chains), held with this trigger
}

// narration drops an actor or observer line from a give route (R16: the
// hand-in narration only the old give route printed goes; no narration
// field). It reports whether the action was narration it dropped.
func (w *triggerWork) narration(k int, a quests.ActionDef) bool {
	if !w.give {
		return false
	}
	name := actionName(a)
	if name != "actor" && name != "observer" {
		return false
	}
	w.c.rep.DroppedNarration = append(w.c.rep.DroppedNarration, fmt.Sprintf("%s action %d: %s dropped (R16): %s", w.where, k, name, describeAction(a)))
	return true
}

// deleteChain lists a quest_granted trigger for deletion with this one.
func (w *triggerWork) deleteChain(q questSource, idx int, why string) {
	w.chain = append(w.chain, fmt.Sprintf("quest %s trigger %d (%s)", q.File, idx, why))
}

func (w *triggerWork) unmappable(k int, a quests.ActionDef, detail string) {
	line := fmt.Sprintf("%s action %d: %s", w.where, k, actionName(a))
	if detail != "" {
		line += ": " + detail
	}
	w.c.rep.Unmappable = append(w.c.rep.Unmappable, line)
	w.held = append(w.held, "unmappable "+actionName(a))
}

func (w *triggerWork) clash(msg string) {
	w.c.rep.Clashes = append(w.c.rep.Clashes, w.where+": "+msg)
	w.held = append(w.held, "clash")
}

func (w *triggerWork) finish() {
	if len(w.held) > 0 {
		w.c.rep.HeldTriggers = append(w.c.rep.HeldTriggers, fmt.Sprintf("hold %s until decided (%s)", w.where, strings.Join(w.held, ", ")))
		for _, ch := range w.chain {
			w.c.rep.HeldTriggers = append(w.c.rep.HeldTriggers, fmt.Sprintf("hold %s with %s", ch, w.where))
		}
		return
	}
	line := "delete " + w.where
	if w.why != "" {
		line += " (" + w.why + ")"
	}
	w.c.deleteTrigger(line)
	for _, ch := range w.chain {
		w.c.deleteTrigger("delete " + ch)
	}
}

// deleteTrigger lists a trigger for Task 16 once; two give routes can share
// one quest_granted chain (quest 63).
func (c *converter) deleteTrigger(line string) {
	key := line
	if i := strings.Index(line, " ("); i >= 0 {
		key = line[:i]
	}
	if c.deleted[key] {
		return
	}
	c.deleted[key] = true
	c.rep.Triggers = append(c.rep.Triggers, line)
}

// conditionGates maps a trigger's conditions to topic gates. Conditions with
// no topic gate are reported and hold the trigger.
func (w *triggerWork) conditionGates(cnd quests.Conditions, item int) dialogue.Gates {
	g := dialogue.Gates{
		QuestRequired: copyStrings(cnd.Has), QuestExcluded: copyStrings(cnd.Missing),
		QuestFlagRequired: copyMap(cnd.HasFlag), QuestFlagExcluded: copyMap(cnd.MissingFlag),
		ItemExcluded: cnd.MissingItem, GoldRequired: cnd.HasGold, MasterworkRequired: cnd.HasMasterwork,
	}
	if cnd.InRoom != 0 {
		w.c.rep.Unmappable = append(w.c.rep.Unmappable, fmt.Sprintf("%s condition in_room %d: no topic gate", w.where, cnd.InRoom))
		w.held = append(w.held, "condition in_room")
	}
	if cnd.HasItem != 0 && cnd.HasItem != item {
		w.c.rep.Unmappable = append(w.c.rep.Unmappable, fmt.Sprintf("%s condition has_item %d: no topic gate besides requiresItem %d", w.where, cnd.HasItem, item))
		w.held = append(w.held, "condition has_item")
	}
	return g
}

// applyFresh maps a trigger's actions onto a topic the converter just made.
func (w *triggerWork) applyFresh(t *cTopic, actions []quests.ActionDef, item int) {
	grantSeen := false
	for k, a := range actions {
		switch actionName(a) {
		case "grant":
			if grantSeen {
				w.unmappable(k, a, fmt.Sprintf("second grant %q (a topic carries one)", a.Grant))
			}
			grantSeen = true
		case "consume_item":
			if a.ConsumeItem != item {
				w.unmappable(k, a, fmt.Sprintf("consumes %d, not the trigger item %d", a.ConsumeItem, item))
			}
		case "give_item":
			if t.GivesItem != 0 && t.GivesItem != a.GiveItem {
				w.clash(fmt.Sprintf("second give_item %d", a.GiveItem))
				continue
			}
			t.GivesItem = a.GiveItem
		case "give_gold":
			t.GivesGold += a.GiveGold
		case "bump_rep":
			t.BumpsRep = append(t.BumpsRep, dialogue.RepBump{Faction: a.BumpRep.Faction, Delta: a.BumpRep.Delta})
		case "set_flag":
			if t.SetsQuestFlag != nil {
				w.clash(fmt.Sprintf("second set_flag %s=%s", a.SetFlag.Key, a.SetFlag.Value))
				continue
			}
			t.SetsQuestFlag = &dialogue.QuestFlagSet{Key: a.SetFlag.Key, Value: a.SetFlag.Value}
		case "npc_say":
			w.sayLine(k, a)
			if t.Text == "" {
				t.Text = npcSayText(a.NpcSay)
			} else {
				t.Text += joinSep + npcSayText(a.NpcSay)
			}
		default:
			if w.narration(k, a) {
				continue
			}
			w.unmappable(k, a, describeAction(a))
		}
	}
}

func (w *triggerWork) sayLine(k int, a quests.ActionDef) {
	if a.NpcSay.Mob != 0 && a.NpcSay.Mob != w.mob {
		w.c.rep.Notes = append(w.c.rep.Notes, fmt.Sprintf("%s action %d: npc_say speaker is mob %d, not the trigger mob %d", w.where, k, a.NpcSay.Mob, w.mob))
	}
}

func describeAction(a quests.ActionDef) string {
	switch {
	case a.SendText != "":
		return fmt.Sprintf("%q", a.SendText)
	case a.RoomText != "":
		return fmt.Sprintf("%q", a.RoomText)
	}
	return ""
}

// itemGive folds one quest item_give trigger into a topic on its mob.
func (c *converter) itemGive(q questSource, i int, tr quests.TriggerDef) {
	w := &triggerWork{c: c, where: fmt.Sprintf("quest %s trigger %d", q.File, i), mob: tr.Mob, give: true}
	defer w.finish()

	var grants []string
	for _, a := range tr.Actions {
		if a.Grant != "" {
			grants = append(grants, a.Grant)
		}
	}
	if len(grants) == 0 {
		c.rep.Notes = append(c.rep.Notes, w.where+": item_give with no grant, left in the quest file")
		w.held = append(w.held, "no grant")
		return
	}
	grant := grants[0]
	f := c.fileFor(tr.Mob, fmt.Sprintf("give-only hand-in, %s", w.where))
	if f == nil {
		w.held = append(w.held, "no file for mob")
		return
	}
	gates := w.conditionGates(tr.Conditions, tr.Item)

	var wanting []*cTopic
	for _, t := range f.topics {
		if t.RequiresItem == tr.Item {
			wanting = append(wanting, t)
		}
	}
	end := endToken(q.Quest.QuestId)

	// R15: a give that starts the quest when a hand-in on this mob takes the
	// same item (the player found it before accepting) is deleted, not
	// merged: talk shows the offer, and a give before accepting hands the
	// item back.
	if kind, qid, _ := kindOf(grant); kind == dialogue.KindOffer && qid == q.Quest.QuestId && len(grants) == 1 {
		for _, t := range wanting {
			if t.GrantsQuest == end {
				c.giveBeforeAccepting(w, q, tr, grant, t)
				return
			}
		}
	}

	if len(wanting) == 0 {
		key := f.uniqueKey(fmt.Sprintf("give_%d", tr.Item))
		t := c.newTopic(key, grant, w.where)
		t.Gates = gates
		t.RequiresItem = tr.Item
		guardCreatedGrant(t)
		w.applyFresh(t, tr.Actions, tr.Item)
		if t.Text == "" {
			c.rep.Notes = append(c.rep.Notes, fmt.Sprintf("mob %d topic %s: no npc_say, text to write", tr.Mob, key))
		}
		t.grants = append(t.grants, srcItemGive)
		c.showNotTake(tr.Mob, t, tr.Item, w.where)
		f.add(t)
		c.rep.Notes = append(c.rep.Notes, fmt.Sprintf("mob %d topic %s created from %s (%s)", tr.Mob, key, w.where, t.Kind))
		return
	}

	// Settled ruling (quests 14, 63, 65, 67; quest 66): a give route whose
	// quest_granted chain grants the quest's end is a direct -end hand-in.
	for _, g := range grants {
		path, ok := chainToEnd(q, g)
		if !ok {
			continue
		}
		var to *cTopic
		for _, t := range wanting {
			if t.GrantsQuest == end {
				to = t
				break
			}
		}
		if to == nil {
			for _, t := range wanting {
				for _, own := range grants {
					if to == nil && t.GrantsQuest == own {
						to = t
					}
				}
			}
		}
		if to != nil {
			c.directEnd(w, q, tr, g, path, to, gates)
			return
		}
		break
	}

	var target *cTopic
	var matching []string
	for _, t := range wanting {
		if t.GrantsQuest == grant {
			matching = append(matching, t.Key)
			if target == nil {
				target = t
			}
		}
	}
	if target == nil {
		var others []string
		for _, t := range wanting {
			others = append(others, fmt.Sprintf("%s (%s)", t.GrantsQuest, t.Key))
		}
		w.clash(fmt.Sprintf("grant mismatch: trigger grants %s, topics on mob %d taking item %d grant %v; not merged", grant, tr.Mob, tr.Item, others))
		return
	}
	if len(matching) > 1 {
		c.rep.Notes = append(c.rep.Notes, fmt.Sprintf("%s: topics %v on mob %d all take item %d and grant %s; merged into %s", w.where, matching, tr.Mob, tr.Item, grant, target.Key))
	}

	// Gates are reported, not changed: the topic's own gates were authored
	// for the same hand-in.
	if diff := gateDiff(gates, target.Gates); diff != "" {
		c.rep.MergeLines = append(c.rep.MergeLines, fmt.Sprintf("mob %d topic %s: trigger gates differ (%s): %s", tr.Mob, target.Key, w.where, diff))
	}

	grantSeen := false
	for k, a := range tr.Actions {
		if a.Grant == "" {
			continue
		}
		if grantSeen {
			w.unmappable(k, a, fmt.Sprintf("second grant %q (a topic carries one)", a.Grant))
		}
		grantSeen = true
	}
	w.mergeEffects(target, tr.Actions, tr.Item, w.where)
	target.grants = append(target.grants, srcItemGive)
	c.showNotTake(tr.Mob, target, tr.Item, w.where)
}

// mergeEffects folds a trigger's actions, other than its grants, into a
// topic that already exists. The ask route and the give route were
// alternatives that paid the same reward: an effect the topic already
// carries is not added again. One it lacks is added and named, since the ask
// route never paid it.
func (w *triggerWork) mergeEffects(target *cTopic, actions []quests.ActionDef, item int, from string) {
	c := w.c
	added := func(what string) {
		c.rep.MergeLines = append(c.rep.MergeLines, fmt.Sprintf("mob %d topic %s: added %s from %s (the ask route did not pay it)", w.mob, target.Key, what, from))
	}
	for k, a := range actions {
		switch actionName(a) {
		case "grant":
			// The caller accounts for grants.
		case "consume_item":
			if a.ConsumeItem != item {
				w.unmappable(k, a, fmt.Sprintf("consumes %d, not the trigger item %d", a.ConsumeItem, item))
			}
		case "bump_rep":
			nb := dialogue.RepBump{Faction: a.BumpRep.Faction, Delta: a.BumpRep.Delta}
			same, other := false, false
			for _, b := range target.BumpsRep {
				if b == nb {
					same = true
				} else if b.Faction == nb.Faction {
					other = true
				}
			}
			switch {
			case same:
			case other:
				w.clash(fmt.Sprintf("bump_rep %s %d clashes with topic %s bumpsRep %v", nb.Faction, nb.Delta, target.Key, target.BumpsRep))
			default:
				target.BumpsRep = append(target.BumpsRep, nb)
				added(fmt.Sprintf("bumpsRep %s %d", nb.Faction, nb.Delta))
			}
		case "set_flag":
			nf := dialogue.QuestFlagSet{Key: a.SetFlag.Key, Value: a.SetFlag.Value}
			switch {
			case target.SetsQuestFlag == nil:
				target.SetsQuestFlag = &nf
				added(fmt.Sprintf("setsQuestFlag %s=%s", nf.Key, nf.Value))
			case *target.SetsQuestFlag != nf:
				w.clash(fmt.Sprintf("set_flag %s=%s clashes with topic %s setsQuestFlag %s=%s", nf.Key, nf.Value, target.Key, target.SetsQuestFlag.Key, target.SetsQuestFlag.Value))
			}
		case "give_item":
			switch {
			case target.GivesItem == 0:
				target.GivesItem = a.GiveItem
				added(fmt.Sprintf("givesItem %d", a.GiveItem))
			case target.GivesItem != a.GiveItem:
				w.clash(fmt.Sprintf("give_item %d clashes with topic %s givesItem %d", a.GiveItem, target.Key, target.GivesItem))
			}
		case "give_gold":
			switch {
			case target.GivesGold == 0:
				target.GivesGold = a.GiveGold
				added(fmt.Sprintf("givesGold %d", a.GiveGold))
			case target.GivesGold != a.GiveGold:
				w.clash(fmt.Sprintf("give_gold %d clashes with topic %s givesGold %d", a.GiveGold, target.Key, target.GivesGold))
			}
		case "npc_say":
			w.sayLine(k, a)
			c.rep.MergeLines = append(c.rep.MergeLines, fmt.Sprintf("mob %d topic %s: npc_say to fold into the topic text: %q", w.mob, target.Key, npcSayText(a.NpcSay)))
		default:
			if w.narration(k, a) {
				continue
			}
			w.unmappable(k, a, describeAction(a))
		}
	}
}

// showNotTake applies the settled carriesItem rule (the Threshold Keeper's
// disc, Rusk's dagger): a route that takes the item and hands the same item
// back only checks that the player carries it, so the topic shows to holders
// and never takes it.
func (c *converter) showNotTake(mob int, t *cTopic, item int, from string) {
	if item == 0 || t.RequiresItem != item || t.GivesItem != item {
		return
	}
	t.RequiresItem, t.GivesItem, t.CarriesItem = 0, 0, item
	c.rep.Rulings = append(c.rep.Rulings, fmt.Sprintf("mob %d topic %s: carriesItem %d from %s (it took the item and handed it back: shown, never taken; settled ruling)", mob, t.Key, item, from))
}

// chainToEnd follows the quest's quest_granted triggers from token through
// the grants they make. When one of them grants the quest's end token it
// returns the trigger indexes passed through, in order.
func chainToEnd(q questSource, token string) ([]int, bool) {
	end := endToken(q.Quest.QuestId)
	seen := map[string]bool{}
	var walk func(tok string) ([]int, bool)
	walk = func(tok string) ([]int, bool) {
		if seen[tok] {
			return nil, false
		}
		seen[tok] = true
		for i, t := range q.Quest.Triggers {
			if t.Event != "quest_granted" || t.QuestToken != tok {
				continue
			}
			for _, a := range t.Actions {
				if a.Grant == end {
					return []int{i}, true
				}
				if a.Grant != "" {
					if rest, ok := walk(a.Grant); ok {
						return append([]int{i}, rest...), true
					}
				}
			}
		}
		return nil, false
	}
	return walk(token)
}

// byRuling counts a grant no topic carries because a ruling says so, and
// names it.
func (c *converter) byRuling(src, line string) {
	c.rep.GrantsByRuling[src]++
	c.rep.RulingGrants = append(c.rep.RulingGrants, fmt.Sprintf("%s: %s", src, line))
}

// giveBeforeAccepting applies R15 to a give route that starts the quest.
func (c *converter) giveBeforeAccepting(w *triggerWork, q questSource, tr quests.TriggerDef, grant string, handIn *cTopic) {
	w.why = "R15: a give before accepting hands the item back; talk shows the offer"
	c.byRuling(srcItemGive, fmt.Sprintf("%s grant %s: dropped (R15: found the item before accepting; the player accepts the offer, then hands in through topic %s)", w.where, grant, handIn.Key))
	var lost []string
	for k, a := range tr.Actions {
		switch actionName(a) {
		case "grant":
		case "npc_say":
			lost = append(lost, fmt.Sprintf("action %d npc_say %q", k, npcSayText(a.NpcSay)))
		default:
			lost = append(lost, fmt.Sprintf("action %d %s %s", k, actionName(a), describeAction(a)))
		}
	}
	line := fmt.Sprintf("%s: R15, deleted, not merged; mob %d topic %s keeps %s", w.where, tr.Mob, handIn.Key, handIn.GrantsQuest)
	if len(lost) > 0 {
		line += "; dropped with it: " + strings.Join(lost, "; ")
	}
	c.rep.Rulings = append(c.rep.Rulings, line)
	if path, ok := chainToEnd(q, grant); ok {
		for _, idx := range path {
			w.deleteChain(q, idx, fmt.Sprintf("R15: quest_granted %s completed the quest on accept, a live bug", q.Quest.Triggers[path[0]].QuestToken))
		}
		c.rep.Rulings = append(c.rep.Rulings, fmt.Sprintf("quest %s triggers %v: R15, the quest_granted chain from %s to %s is deleted (it completed the quest on accept)", q.File, path, grant, endToken(q.Quest.QuestId)))
	}
}

// directEnd merges a give route whose quest_granted chain ends the quest into
// the hand-in topic as a direct -end grant (settled ruling for the -report
// routes of quests 14, 63, 65, 67 and the escort route of quest 66). The
// chain's effects merge with the trigger's, and the chain is deleted with it.
func (c *converter) directEnd(w *triggerWork, q questSource, tr quests.TriggerDef, chained string, path []int, target *cTopic, gates dialogue.Gates) {
	qid := q.Quest.QuestId
	end := endToken(qid)
	if target.GrantsQuest != end {
		// The topic granted a step the give route passed through: it now
		// grants the end, and the step's grant is named.
		old := target.GrantsQuest
		for _, s := range target.grants {
			c.byRuling(s, fmt.Sprintf("mob %d topic %s grant %s: replaced by %s (settled ruling: the hand-in grants the end directly)", tr.Mob, target.Key, old, end))
		}
		target.grants = nil
		target.GrantsQuest, target.Kind, target.Quest, target.Marked = end, dialogue.KindHandIn, qid, false
		target.Label = c.label(dialogue.KindHandIn, qid, w.where)
		target.QuestExcluded = appendUnique(target.QuestExcluded, end)
		c.rep.Rulings = append(c.rep.Rulings, fmt.Sprintf("mob %d topic %s: now the hand-in for quest %d, granting %s instead of %s (settled ruling: one hand-in topic; %s and its chain deleted)", tr.Mob, target.Key, qid, end, old, w.where))
	}
	if diff := gateDiff(gates, target.Gates); diff != "" {
		c.rep.MergeLines = append(c.rep.MergeLines, fmt.Sprintf("mob %d topic %s: trigger gates differ (%s): %s", tr.Mob, target.Key, w.where, diff))
	}
	for _, a := range tr.Actions {
		switch {
		case a.Grant == "":
		case a.Grant == chained:
			target.grants = append(target.grants, srcItemGive)
			c.rep.MergeLines = append(c.rep.MergeLines, fmt.Sprintf("mob %d topic %s: grant %s from %s merged as %s (its quest_granted chain, triggers %v, completes the quest; settled ruling)", tr.Mob, target.Key, chained, w.where, end, path))
		default:
			c.byRuling(srcItemGive, fmt.Sprintf("%s grant %s: dropped (settled ruling: the step is skipped, topic %s grants %s directly)", w.where, a.Grant, target.Key, end))
		}
	}
	w.mergeEffects(target, tr.Actions, tr.Item, w.where)
	for _, idx := range path {
		from := fmt.Sprintf("quest %s trigger %d", q.File, idx)
		w.mergeEffects(target, q.Quest.Triggers[idx].Actions, tr.Item, from)
		w.deleteChain(q, idx, fmt.Sprintf("quest_granted %s chain to %s, folded into mob %d topic %s", q.Quest.Triggers[idx].QuestToken, end, tr.Mob, target.Key))
	}
	c.showNotTake(tr.Mob, target, tr.Item, w.where)
}

// gateDiff describes how a trigger's gates differ from a topic's.
func gateDiff(trig, topic dialogue.Gates) string {
	var parts []string
	missingFrom := func(want, have []string) []string {
		var out []string
		for _, w := range want {
			if len(appendUnique(copyStrings(have), w)) != len(have) {
				out = append(out, w)
			}
		}
		return out
	}
	if m := missingFrom(trig.QuestRequired, topic.QuestRequired); len(m) > 0 {
		parts = append(parts, fmt.Sprintf("has %v not in questRequired %v", m, topic.QuestRequired))
	}
	if m := missingFrom(trig.QuestExcluded, topic.QuestExcluded); len(m) > 0 {
		parts = append(parts, fmt.Sprintf("missing %v not in questExcluded %v", m, topic.QuestExcluded))
	}
	return strings.Join(parts, "; ")
}

// dialogueTrigger turns a quest dialogue trigger into a marked info topic.
func (c *converter) dialogueTrigger(q questSource, i int, tr quests.TriggerDef) {
	w := &triggerWork{c: c, where: fmt.Sprintf("quest %s trigger %d", q.File, i), mob: tr.Mob}
	defer w.finish()

	var grant string
	for _, a := range tr.Actions {
		if a.Grant != "" {
			grant = a.Grant
			break
		}
	}
	f := c.fileFor(tr.Mob, "dialogue trigger, "+w.where)
	if f == nil {
		w.held = append(w.held, "no file for mob")
		return
	}
	if tr.Topic != "" {
		c.rep.Notes = append(c.rep.Notes, fmt.Sprintf("%s: topic filter %q dropped (the topic is chosen from the menu)", w.where, tr.Topic))
	}
	key := f.uniqueKey(fmt.Sprintf("asked_%d", q.Quest.QuestId))
	t := &cTopic{}
	t.Key, t.Kind, t.Marked = key, dialogue.KindInfo, true
	t.GrantsQuest = grant
	if kind, _, _ := kindOf(grant); kind != dialogue.KindInfo {
		w.clash(fmt.Sprintf("grant %s would make an %s, written as a marked info topic", grant, kind))
	}
	t.Gates = w.conditionGates(tr.Conditions, 0)
	guardCreatedGrant(t)
	w.applyFresh(t, tr.Actions, 0)
	if grant != "" {
		t.grants = append(t.grants, srcDialogue)
	}
	f.add(t)

	var gated []string
	for _, o := range f.topics {
		for _, r := range o.QuestRequired {
			if r == grant && o != t {
				gated = append(gated, o.Key)
			}
		}
	}
	note := fmt.Sprintf("mob %d topic %s created from %s; text to write", tr.Mob, key, w.where)
	if len(gated) > 0 {
		note += fmt.Sprintf("; topics gated on %s that answered the same ask: %v (fold their text into %s)", grant, gated, key)
	}
	c.rep.Notes = append(c.rep.Notes, note)
}

// ---------------------------------------------------------------- rule 7

// flatten collects a branch's conditions and actions, depth first.
func flatten(n btNode, conds, acts *[]btNode) {
	if n.Check != "" {
		*conds = append(*conds, n)
	}
	if n.Do != "" {
		*acts = append(*acts, n)
	}
	for _, ch := range n.Children {
		flatten(ch, conds, acts)
	}
	if n.Child != nil {
		flatten(*n.Child, conds, acts)
	}
}

// askBranches finds every node with event player_ask, not looking inside one.
func askBranches(n btNode, out *[]btNode) {
	if n.Event == "player_ask" {
		*out = append(*out, n)
		return
	}
	for _, ch := range n.Children {
		askBranches(ch, out)
	}
	if n.Child != nil {
		askBranches(*n.Child, out)
	}
}

func paramString(p map[string]any, key string) string {
	if v, ok := p[key]; ok && v != nil {
		return strings.TrimSpace(fmt.Sprint(v))
	}
	return ""
}

func paramList(p map[string]any, key string) []string {
	var out []string
	if l, ok := p[key].([]any); ok {
		for _, v := range l {
			out = append(out, fmt.Sprint(v))
		}
	}
	return out
}

// paramKeys returns a map param's keys, sorted.
func paramKeys(p map[string]any, key string) []string {
	var out []string
	switch m := p[key].(type) {
	case map[any]any:
		for k := range m {
			out = append(out, fmt.Sprint(k))
		}
	case map[string]any:
		for k := range m {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// spoken is the text a speech action says to the player.
func spoken(a btNode) string {
	switch a.Do {
	case "respond":
		return paramString(a.Params, "user_text")
	case "emote":
		return "[emote] " + paramString(a.Params, "text")
	}
	return paramString(a.Params, "text")
}

// branchInfo is one player_ask branch, read once.
type branchInfo struct {
	n           int
	where       string
	keywords    []string // lowercased
	conds       []btNode // every condition except keyword_match
	gates       dialogue.Gates
	treeOnly    []string
	speech      []string
	firstSpeech int
	grants      []string
	others      []string
	ferry       *btNode
	portal      *btNode
	opener      bool      // became the opener or an opener variant
	topics      []*cTopic // the topics it became
}

// speechOnly: the branch only speaks, so its text can live outside the tree.
func (bi *branchInfo) speechOnly() bool {
	return len(bi.others) == 0 && len(bi.grants) == 0 && len(bi.treeOnly) == 0 && len(bi.speech) > 0
}

func (bi *branchInfo) greets() bool {
	for _, k := range bi.keywords {
		if greetingWords[k] {
			return true
		}
	}
	return false
}

// shadows reports whether this earlier branch saw a question before later
// did: a branch with no keyword_match sees every question; otherwise they
// must share a keyword.
func (bi *branchInfo) shadows(later *branchInfo) bool {
	if len(bi.keywords) == 0 {
		return true
	}
	for _, k := range bi.keywords {
		for _, l := range later.keywords {
			if k == l {
				return true
			}
		}
	}
	return false
}

func analyzeBranch(path string, n int, br btNode) *branchInfo {
	var conds, acts []btNode
	flatten(br, &conds, &acts)
	bi := &branchInfo{n: n, where: fmt.Sprintf("`%s` branch %d", path, n), firstSpeech: -1}
	for _, cd := range conds {
		switch cd.Check {
		case "keyword_match":
			if bi.keywords == nil {
				bi.keywords = normKeywords(paramList(cd.Params, "keywords"))
			}
			continue
		case "player_has_quest":
			bi.gates.QuestRequired = appendUnique(bi.gates.QuestRequired, paramString(cd.Params, "quest"))
		case "player_missing_quest":
			bi.gates.QuestExcluded = appendUnique(bi.gates.QuestExcluded, paramString(cd.Params, "quest"))
		case "player_has_item":
			// A key the player keeps: the topic shows only to holders, and
			// the tree keeps the check.
			bi.gates.CarriesItem = paramInt(cd.Params, "item_id")
			bi.treeOnly = append(bi.treeOnly, cd.Check)
		default:
			bi.treeOnly = append(bi.treeOnly, cd.Check)
		}
		bi.conds = append(bi.conds, cd)
	}
	if len(bi.keywords) > 0 {
		bi.where += fmt.Sprintf(" (keywords %v)", bi.keywords)
	}
	for k, a := range acts {
		switch {
		case speechActions[a.Do]:
			if s := spoken(a); s != "" {
				bi.speech = append(bi.speech, s)
				if bi.firstSpeech < 0 {
					bi.firstSpeech = k + 1
				}
			}
		case a.Do == "grant_quest":
			bi.grants = append(bi.grants, paramString(a.Params, "quest"))
		default:
			bi.others = append(bi.others, a.Do)
			if a.Do == "board_ferry" && bi.ferry == nil {
				bi.ferry = &acts[k]
			}
			if a.Do == "open_instance_portal" && bi.portal == nil {
				bi.portal = &acts[k]
			}
		}
	}
	return bi
}

func paramInt(p map[string]any, key string) int {
	switch v := p[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(v))
		return n
	}
	return 0
}

func cloneGates(g dialogue.Gates) dialogue.Gates {
	g.QuestRequired, g.QuestExcluded = copyStrings(g.QuestRequired), copyStrings(g.QuestExcluded)
	g.QuestFlagRequired, g.QuestFlagExcluded = copyMap(g.QuestFlagRequired), copyMap(g.QuestFlagExcluded)
	return g
}

func (c *converter) convertBehavior(b behaviorSource) {
	var nodes []btNode
	askBranches(b.Tree, &nodes)
	if len(nodes) == 0 {
		return
	}
	var branches []*branchInfo
	for n, br := range nodes {
		branches = append(branches, analyzeBranch(b.Path, n+1, br))
	}
	if rule, ok := behaviorRulings[b.Path]; ok && rule(c, b, branches) {
		return
	}
	f := c.fileFor(b.MobId, "behavior-only mob, "+b.Path)
	if f == nil {
		for _, bi := range branches {
			c.rep.Behavior = append(c.rep.Behavior, bi.where+": no file for the mob, not converted")
		}
		return
	}

	// A file with no opener takes its first unconditional speech-only
	// greeting branch. Speech-only greeting branches above it that share a
	// keyword and are gated only on quests were tried first: they become
	// opener variants, which also win first.
	if f.Opener == nil {
		for g, bi := range branches {
			if !bi.speechOnly() || !bi.greets() || len(bi.conds) > 0 {
				continue
			}
			f.Opener = &dialogue.Opener{Text: strings.Join(bi.speech, joinSep)}
			bi.opener = true
			c.rep.Behavior = append(c.rep.Behavior, bi.where+": speech only greeting: became the opener: delete the branch")
			for _, v := range branches[:g] {
				if v.speechOnly() && v.greets() && v.shadows(bi) && v.gates.CarriesItem == 0 {
					f.Opener.Variants = append(f.Opener.Variants, dialogue.OpenerVariant{Gates: cloneGates(v.gates), Text: strings.Join(v.speech, joinSep)})
					v.opener = true
					c.rep.Behavior = append(c.rep.Behavior, v.where+": gated speech only greeting: became an opener variant: delete the branch")
				}
			}
			break
		}
	}

	for _, bi := range branches {
		if !bi.opener {
			c.emitBranch(f, bi)
		}
	}

	// Selector order: a later branch sharing a keyword with an earlier gated
	// one was its fallback, heard only when the earlier gate failed.
	for j, later := range branches {
		if later.opener || len(later.topics) == 0 {
			continue
		}
		for _, earlier := range branches[:j] {
			if earlier.opener || len(earlier.conds) == 0 || !earlier.shadows(later) {
				continue
			}
			c.gateFallback(earlier, later)
		}
	}
}

// behaviorRuling applies an owner ruling to one behavior file in place of the
// general branch rules. It reports false, leaving the general rules to run
// and the report to say why, when the file no longer has the shape the
// ruling was made for.
type behaviorRuling func(c *converter, b behaviorSource, branches []*branchInfo) bool

// behaviorRulings are the one-off rulings on single behavior trees (spec R17,
// R19), keyed by path.
var behaviorRulings = map[string]behaviorRuling{
	"behaviors/pothole_coulee/9100-cleric_hadwen.yaml":   hadwenRite,
	"behaviors/marches_spur_road/254-bandit_leader.yaml": banditLeaderAsk,
}

// rulingShape reports a ruling not applied because its file changed.
func (c *converter) rulingShape(b behaviorSource, ruling, why string) bool {
	c.rep.Clashes = append(c.rep.Clashes, fmt.Sprintf("`%s`: ruling %s not applied (%s); general rules used, owner decision", b.Path, ruling, why))
	return false
}

// banditLeaderAsk applies R19: the bandit leader's ask branch (no
// keyword_match; it cancelled his ambush countdown on any ask) becomes no
// topic. Nothing in conversation stops his ambush.
func banditLeaderAsk(c *converter, b behaviorSource, branches []*branchInfo) bool {
	for _, bi := range branches {
		if len(bi.keywords) > 0 || len(bi.grants) > 0 || len(bi.speech) > 0 {
			return c.rulingShape(b, "R19", fmt.Sprintf("branch %d has keywords, grants or speech", bi.n))
		}
	}
	for _, bi := range branches {
		c.rep.Behavior = append(c.rep.Behavior, fmt.Sprintf("%s: no topic (R19: the talk option is dropped; nothing in conversation stops his ambush): delete the branch", bi.where))
	}
	c.rep.Rulings = append(c.rep.Rulings, fmt.Sprintf("mob %d (`%s`): R19, its %d ask branch(es) become no topic; the dialogue file stays info only", b.MobId, b.Path, len(branches)))
	return true
}

// Hadwen's rite (R17): the grant the tree keeps and the one it stops making.
const (
	hadwenKeeps = "30-end"
	hadwenDrops = "29-start"
)

// hadwenRite applies R17: the mid-rite answer and the rite (both keyed on
// begin) answer one behavior topic begin, gated on 30-start and not 30-end.
// The topic carries no grant: the rite keeps granting 30-end at its close,
// and stops granting 29-start (quest 29 is replaced by two quests Hadwen
// offers after the rite).
func hadwenRite(c *converter, b behaviorSource, branches []*branchInfo) bool {
	var rite *branchInfo
	var waits []*branchInfo
	for _, bi := range branches {
		begin := false
		for _, k := range bi.keywords {
			begin = begin || k == "begin"
		}
		if !begin {
			return c.rulingShape(b, "R17", fmt.Sprintf("branch %d is not a begin branch", bi.n))
		}
		if len(bi.grants) > 0 {
			if rite != nil {
				return c.rulingShape(b, "R17", "more than one granting begin branch")
			}
			rite = bi
			continue
		}
		for _, cd := range bi.conds {
			if cd.Check != "state_equals" {
				return c.rulingShape(b, "R17", fmt.Sprintf("branch %d has condition %s", bi.n, cd.Check))
			}
		}
		waits = append(waits, bi)
	}
	if rite == nil {
		return c.rulingShape(b, "R17", "no begin branch grants a quest")
	}
	for _, g := range rite.grants {
		if g != hadwenKeeps && g != hadwenDrops {
			return c.rulingShape(b, "R17", "the rite grants "+g)
		}
	}
	f := c.fileFor(b.MobId, "behavior-only mob, "+b.Path)
	if f == nil {
		return c.rulingShape(b, "R17", "no file for the mob")
	}

	key := f.uniqueKey("begin")
	t := &cTopic{}
	t.Key, t.Kind, t.Behavior = key, dialogue.KindInfo, true
	t.Gates = cloneGates(rite.gates)
	if len(rite.speech) > 0 {
		t.Text = rite.speech[0]
	}
	f.add(t)

	for _, w := range waits {
		c.rep.Behavior = append(c.rep.Behavior, fmt.Sprintf("%s: topic %s (R17, shared with branch %d): event: player_topic, check: topic_is with topic: %s replaces keyword_match; keep: state_equals (the mid-rite answer)",
			w.where, key, rite.n, key))
	}
	took := "no speech to remove"
	if rite.firstSpeech > 0 {
		took = fmt.Sprintf("remove the speech action %d that the topic text now speaks", rite.firstSpeech)
	}
	keep := strings.Join(append(append([]string(nil), rite.treeOnly...), rite.others...), ", ")
	c.rep.Behavior = append(c.rep.Behavior, fmt.Sprintf("%s: topic %s (R17): event: player_topic, check: topic_is with topic: %s replaces keyword_match; %s; remove grant_quest %s; keep grant_quest %s and: %s",
		rite.where, key, key, took, hadwenDrops, hadwenKeeps, keep))

	for _, g := range rite.grants {
		if g == hadwenKeeps {
			c.byRuling(srcBehavior, fmt.Sprintf("%s grant_quest %s: stays in the tree, granted at the rite's close; topic %s carries no grant (R17)", rite.where, g, key))
		} else {
			c.byRuling(srcBehavior, fmt.Sprintf("%s grant_quest %s: dropped (R17: Hadwen grants no new quest from his tree; quest 29 is replaced by two quests offered after the rite)", rite.where, g))
		}
	}
	var ns []string
	for _, w := range waits {
		ns = append(ns, strconv.Itoa(w.n))
	}
	ns = append(ns, strconv.Itoa(rite.n))
	c.rep.Rulings = append(c.rep.Rulings, fmt.Sprintf("mob %d (`%s`): R17, branches %s become one behavior topic %s (questRequired %v, questExcluded %v, no grant); the tree keeps grant_quest %s and stops granting %s; the two bridge quests that replace quest 29 are content work",
		b.MobId, b.Path, strings.Join(ns, ", "), key, t.QuestRequired, t.QuestExcluded, hadwenKeeps, hadwenDrops))
	return true
}

// gateFallback gives the later branch's topics the inverse of the earlier
// branch's one condition. A conjunction has no single inverse gate, and a
// tree-state condition has none at all: both are reported.
func (c *converter) gateFallback(earlier, later *branchInfo) {
	var checks []string
	for _, cd := range earlier.conds {
		checks = append(checks, cd.Check)
	}
	cannot := func(why string) {
		c.rep.Clashes = append(c.rep.Clashes, fmt.Sprintf("%s falls back from branch %d (conditions %v): could not gate (%s); owner decision", later.where, earlier.n, checks, why))
	}
	if len(earlier.conds) > 1 {
		cannot("more than one condition, so no single inverse gate")
		return
	}
	cd := earlier.conds[0]
	var what string
	apply := func(t *cTopic) {}
	switch cd.Check {
	case "player_has_quest":
		q := paramString(cd.Params, "quest")
		what = "questExcluded " + q
		apply = func(t *cTopic) { t.QuestExcluded = appendUnique(t.QuestExcluded, q) }
	case "player_missing_quest":
		q := paramString(cd.Params, "quest")
		what = "questRequired " + q
		apply = func(t *cTopic) { t.QuestRequired = appendUnique(t.QuestRequired, q) }
	case "player_has_item":
		id := paramInt(cd.Params, "item_id")
		what = fmt.Sprintf("itemExcluded %d", id)
		apply = func(t *cTopic) {
			if t.ItemExcluded != 0 && t.ItemExcluded != id {
				c.rep.Clashes = append(c.rep.Clashes, fmt.Sprintf("%s: topic %s already has itemExcluded %d, not %d", later.where, t.Key, t.ItemExcluded, id))
				return
			}
			t.ItemExcluded = id
		}
	default:
		cannot(cd.Check + " has no topic gate")
		return
	}
	var keys []string
	for _, t := range later.topics {
		apply(t)
		keys = append(keys, t.Key)
	}
	c.rep.Fallbacks = append(c.rep.Fallbacks, fmt.Sprintf("%s falls back from branch %d (%s): %s added to %v", later.where, earlier.n, cd.Check, what, keys))
}

// emitBranch writes the topics one branch becomes.
func (c *converter) emitBranch(f *cFile, bi *branchInfo) {
	before := len(f.topics)
	defer func() { bi.topics = append([]*cTopic(nil), f.topics[before:]...) }()

	where, gates, speech, firstSpeech := bi.where, bi.gates, bi.speech, bi.firstSpeech
	grants, others, treeOnly, ferry, portal := bi.grants, bi.others, bi.treeOnly, bi.ferry, bi.portal
	firstText := ""
	if len(speech) > 0 {
		firstText = speech[0]
	}
	keep := strings.Join(append(append([]string(nil), treeOnly...), others...), ", ")
	if keep == "" {
		keep = "nothing else"
	}
	baseKey := ""
	if len(bi.keywords) > 0 {
		baseKey = slug(bi.keywords[0])
	}
	if baseKey == "" {
		baseKey = fmt.Sprintf("ask_%d", bi.n)
	}
	if len(bi.keywords) == 0 && ferry == nil && portal == nil {
		c.rep.Clashes = append(c.rep.Clashes, fmt.Sprintf("%s: no keyword_match, so it ran on every ask; as a topic it runs only when chosen; owner decision", where))
	}
	newInfo := func(key string) *cTopic {
		t := &cTopic{}
		t.Key, t.Kind = key, dialogue.KindInfo
		t.Gates = cloneGates(gates)
		return t
	}

	switch {
	case ferry != nil:
		dests := paramKeys(ferry.Params, "routes")
		if len(dests) == 0 {
			c.rep.Behavior = append(c.rep.Behavior, where+": board_ferry with no routes, not converted")
			return
		}
		for _, d := range dests {
			key := f.uniqueKey("to_" + slug(d))
			t := newInfo(key)
			t.Behavior = true
			t.Text = firstText
			f.add(t)
			c.rep.Behavior = append(c.rep.Behavior, fmt.Sprintf("%s: topic %s: one branch per destination: event: player_topic, check: topic_is with topic: %s, board_ferry with destination: %s (routes unchanged); keep: %s",
				where, key, key, d, keep))
		}
	case portal != nil:
		zones := paramKeys(portal.Params, "zones")
		if len(zones) == 0 {
			c.rep.Behavior = append(c.rep.Behavior, where+": open_instance_portal with no zones, not converted")
			return
		}
		for _, z := range zones {
			key := f.uniqueKey("open_" + slug(z))
			t := newInfo(key)
			t.Behavior = true
			t.Text = firstText
			f.add(t)
			c.rep.Behavior = append(c.rep.Behavior, fmt.Sprintf("%s: topic %s: one branch per zone: event: player_topic, check: topic_is with topic: %s, open_instance_portal with zone: %s and guidance: (to write, owner samples); keep in the tree: %s",
				where, key, key, z, keep))
		}
	case len(grants) > 0:
		key := f.uniqueKey(baseKey)
		t := c.newTopic(key, grants[0], where)
		t.Gates = cloneGates(gates)
		guardCreatedGrant(t)
		t.grants = append(t.grants, srcBehavior)
		for _, g := range grants[1:] {
			c.rep.Clashes = append(c.rep.Clashes, fmt.Sprintf("%s: second grant_quest %s: a topic carries one grant", where, g))
		}
		if len(others) == 0 && len(treeOnly) == 0 {
			t.Text = strings.Join(speech, joinSep)
			f.add(t)
			c.rep.Behavior = append(c.rep.Behavior, fmt.Sprintf("%s: topic %s (%s, quest %d): grant_quest becomes the %s: delete the branch", where, key, t.Kind, t.Quest, t.Kind))
			return
		}
		t.Text = firstText
		t.Behavior = true
		f.add(t)
		c.rep.Clashes = append(c.rep.Clashes, fmt.Sprintf("%s: grant_quest %v in a branch that also runs %s: the topic grant and the tree grant would both fire; owner decision", where, grants, keep))
		c.rep.Behavior = append(c.rep.Behavior, fmt.Sprintf("%s: topic %s (%s, quest %d, behavior): owner decision before any edit (grant_quest beside %s)", where, key, t.Kind, t.Quest, keep))
	case len(others) > 0 || len(treeOnly) > 0:
		key := f.uniqueKey(baseKey)
		t := newInfo(key)
		t.Behavior = true
		t.Text = firstText
		f.add(t)
		took := "no speech to remove"
		if firstSpeech > 0 {
			took = fmt.Sprintf("remove the speech action %d that the topic text now speaks", firstSpeech)
		}
		c.rep.Behavior = append(c.rep.Behavior, fmt.Sprintf("%s: topic %s: event: player_topic, check: topic_is with topic: %s replaces keyword_match; %s; keep: %s",
			where, key, key, took, keep))
	default:
		key := f.uniqueKey(baseKey)
		t := newInfo(key)
		t.Text = strings.Join(speech, joinSep)
		f.add(t)
		c.rep.Behavior = append(c.rep.Behavior, fmt.Sprintf("%s: topic %s: speech only: delete the branch", where, key))
	}
}

// ---------------------------------------------------------------- finish

func (c *converter) finish() {
	// Delete lines keep quest order; within one quest file, trigger order
	// (a shared chain is listed when its first give route is).
	first := map[string]int{}
	fileAndIndex := func(line string) (string, int) {
		fs := strings.Fields(line) // delete quest <file> trigger <n> ...
		if len(fs) < 5 {
			return line, 0
		}
		n, _ := strconv.Atoi(fs[4])
		return fs[2], n
	}
	for i, l := range c.rep.Triggers {
		if f, _ := fileAndIndex(l); first[f] == 0 {
			first[f] = i + 1
		}
	}
	sort.SliceStable(c.rep.Triggers, func(i, j int) bool {
		fi, ni := fileAndIndex(c.rep.Triggers[i])
		fj, nj := fileAndIndex(c.rep.Triggers[j])
		if fi != fj {
			return first[fi] < first[fj]
		}
		return ni < nj
	})

	ids := make([]int, 0, len(c.res.Files))
	for id := range c.res.Files {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		f := c.res.Files[id]
		c.rep.Suffixed = append(c.rep.Suffixed, f.suffixed...)
		if f.Opener == nil {
			c.rep.NoOpener = append(c.rep.NoOpener, fmt.Sprintf("mob %d (dialogue/%s/%d.yaml): no tree root, greeting pattern, catch-all or ungated greeting branch; the zone pass writes one", id, f.Folder, id))
		}
		for _, t := range f.topics {
			// Rule 5: a priced topic is hidden from a player who cannot pay.
			if t.ChargesGold > t.GoldRequired {
				t.GoldRequired = t.ChargesGold
			}
			for _, g := range t.grants {
				c.rep.GrantsOut[g]++
			}
			if t.GrantsQuest != "" {
				c.rep.TopicsCarryingGrants++
				if len(t.grants) == 0 {
					c.rep.Notes = append(c.rep.Notes, fmt.Sprintf("mob %d topic %s carries %s with no grant source", id, t.Key, t.GrantsQuest))
				}
			}
			if t.Kind == dialogue.KindOffer {
				g := t.Gates
				if len(g.QuestRequired) > 0 || len(g.QuestFlagRequired) > 0 || len(t.HeardRequired) > 0 || g.RequiresItem != 0 ||
					g.GoldRequired != 0 || g.MasterworkRequired != 0 || g.ItemExcluded != 0 || len(g.QuestFlagExcluded) > 0 {
					c.rep.GatedOffers = append(c.rep.GatedOffers, fmt.Sprintf("mob %d topic %s (quest %d): a breadcrumb may fit", id, t.Key, t.Quest))
				}
			}
		}
	}
}

// countInputGrants counts every grant where the old content keeps it. It
// reads the input only, never the converted topics, so it is the
// independent side of the reconciliation.
func countInputGrants(in *Input) map[string]int {
	m := map[string]int{}
	for _, d := range in.Dialogues {
		for _, p := range d.File.Patterns {
			if p.GrantsQuest != "" {
				m[srcPattern]++
			}
		}
		if d.File.Tree == nil {
			continue
		}
		for _, n := range d.File.Tree.Nodes {
			if n.GrantsQuest != "" {
				m[grantSourceOfNode(n.GrantsQuest)]++
			}
		}
		for _, v := range d.File.Tree.Root.Variants {
			if v.GrantsQuest != "" {
				m[srcVariant]++
			}
		}
	}
	for _, q := range in.Quests {
		for _, t := range q.Quest.Triggers {
			for _, a := range t.Actions {
				if a.Grant == "" {
					continue
				}
				switch t.Event {
				case "item_give":
					m[srcItemGive]++
				case "dialogue":
					m[srcDialogue]++
				}
			}
		}
	}
	for _, b := range in.Behaviors {
		var branches []btNode
		askBranches(b.Tree, &branches)
		for _, br := range branches {
			var conds, acts []btNode
			flatten(br, &conds, &acts)
			for _, a := range acts {
				if a.Do == "grant_quest" {
					m[srcBehavior]++
				}
			}
		}
	}
	return m
}

// ---------------------------------------------------------------- report

// Report is what the converter could not decide alone, and the counts that
// prove nothing was lost.
type Report struct {
	Patterns             map[string]int
	Nodes                map[string]int
	GrantsIn             map[string]int
	GrantsOut            map[string]int
	GrantsByRuling       map[string]int // grants no topic carries, each named in RulingGrants
	TopicsCarryingGrants int

	Rulings          []string // owner rulings and settled rules applied, with what each did
	RulingGrants     []string // one line per grant counted in GrantsByRuling
	DroppedNarration []string // R16: give-route narration dropped

	Requires     []string
	Unmappable   []string
	Clashes      []string
	MergeLines   []string
	Triggers     []string
	HeldTriggers []string
	Behavior     []string
	NewFiles     []string
	GatedOffers  []string
	Dropped      []string
	Notes        []string

	Fallbacks      []string // behavior branches gated as the fallback of the branch above
	Suffixed       []string // keys that took a _2, _3 suffix, with their sibling
	FoldIntoOpener []string // "other" patterns that are a greeting plus at most one word
	NoOpener       []string // files the converter could give no opener

	// OverlapKept lists each overlap pattern whose text the tree lacked: the
	// topic it became or the node topic it merged into, for the zone pass.
	OverlapKept     []string
	OverlapOutcomes map[string]int // "topic", "merged", "covered"
}

func newReport() *Report {
	return &Report{Patterns: map[string]int{}, Nodes: map[string]int{}, GrantsIn: map[string]int{}, GrantsOut: map[string]int{}, GrantsByRuling: map[string]int{},
		OverlapOutcomes: map[string]int{}}
}

func sum(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}

func (r *Report) GrantsInTotal() int  { return sum(r.GrantsIn) }
func (r *Report) GrantsOutTotal() int { return sum(r.GrantsOut) }

// GrantsByRulingTotal counts the grants no topic carries by a ruling.
func (r *Report) GrantsByRulingTotal() int { return sum(r.GrantsByRuling) }

// Reconciled reports whether every component and the total agree: each
// input grant is carried by a topic or named under a ruling.
func (r *Report) Reconciled() bool {
	for _, s := range grantSources {
		if r.GrantsIn[s] != r.GrantsOut[s]+r.GrantsByRuling[s] {
			return false
		}
	}
	return len(r.RulingGrants) == r.GrantsByRulingTotal() &&
		r.GrantsInTotal() == r.GrantsOutTotal()+r.GrantsByRulingTotal()
}

// Markdown renders the report.
func (r *Report) Markdown() string {
	var b strings.Builder
	b.WriteString("# Dialogue conversion report\n\n## Counts\n\n")
	b.WriteString("Patterns by class:\n\n")
	for _, k := range []string{"catchall", "greeting", "overlap", "other"} {
		fmt.Fprintf(&b, "- %s: %d\n", k, r.Patterns[k])
	}
	fmt.Fprintf(&b, "- total: %d\n\nOverlap patterns by outcome (each kept one is listed under \"Overlap patterns kept\"):\n\n", sum(r.Patterns))
	fmt.Fprintf(&b, "- overlap kept as a topic: %d\n- overlap merged into a node topic: %d\n- overlap dropped, text already in the node: %d\n\nNodes by kind:\n\n",
		r.OverlapOutcomes["topic"], r.OverlapOutcomes["merged"], r.OverlapOutcomes["covered"])
	for _, k := range []string{"offer", "handin", "info marked", "info"} {
		fmt.Fprintf(&b, "- %s: %d\n", k, r.Nodes[k])
	}
	fmt.Fprintf(&b, "- total: %d\n\n", sum(r.Nodes))
	b.WriteString("Grants by source (input: counted in the old content; output: carried by the converted topics; by ruling: carried by no topic because a ruling says so, each named under \"Grants accounted for by a ruling\"):\n\n")
	b.WriteString("| source | input | output | by ruling |\n|---|---|---|---|\n")
	for _, s := range grantSources {
		fmt.Fprintf(&b, "| %s | %d | %d | %d |\n", s, r.GrantsIn[s], r.GrantsOut[s], r.GrantsByRuling[s])
	}
	fmt.Fprintf(&b, "| total | %d | %d | %d |\n\n", r.GrantsInTotal(), r.GrantsOutTotal(), r.GrantsByRulingTotal())
	fmt.Fprintf(&b, "Topics carrying a grant: %d (merged item_give grants share their topic)\n\n", r.TopicsCarryingGrants)
	if r.Reconciled() {
		b.WriteString("Reconciled: yes (input = output + by ruling, for every source and the total)\n\n")
	} else {
		b.WriteString("Reconciled: NO (a component or the total disagrees; see Clashes and Unmappable)\n\n")
	}
	section := func(title string, lines []string) {
		fmt.Fprintf(&b, "## %s\n\n", title)
		if len(lines) == 0 {
			b.WriteString("None.\n\n")
			return
		}
		for _, l := range lines {
			fmt.Fprintf(&b, "- %s\n", l)
		}
		b.WriteString("\n")
	}
	section("Grants accounted for by a ruling", r.RulingGrants)
	section("Rulings applied", r.Rulings)
	section("Requires mappings", r.Requires)
	section("Unmappable actions", r.Unmappable)
	section("Dropped narration", r.DroppedNarration)
	section("Clashes and mismatches", r.Clashes)
	section("Merge lines", r.MergeLines)
	section("Triggers to delete", r.Triggers)
	section("Triggers held for a decision", r.HeldTriggers)
	section("Behavior branches", r.Behavior)
	section("Branch fallbacks gated", r.Fallbacks)
	section("New files", r.NewFiles)
	section("Files without an opener", r.NoOpener)
	section("Suffixed keys", r.Suffixed)
	section("Patterns to fold into the opener", r.FoldIntoOpener)
	section("Gated offers", r.GatedOffers)
	section("Overlap patterns kept", r.OverlapKept)
	section("Dropped patterns", r.Dropped)
	section("Notes", r.Notes)
	return b.String()
}
