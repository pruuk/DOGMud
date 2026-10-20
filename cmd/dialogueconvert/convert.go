package main

import (
	"fmt"
	"sort"
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
	in     *Input
	res    *Result
	rep    *Report
	titles map[int]string
}

// Convert applies the rules of plan Task 13 (spec section 6.1) in order.
func Convert(in *Input) *Result {
	c := &converter{in: in, rep: newReport(), titles: map[int]string{}}
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
		base := slug(n.Id)
		key := f.uniqueKey(base)
		if key != base {
			c.rep.Notes = append(c.rep.Notes, fmt.Sprintf("%s: key %q taken, written as %q", where, base, key))
		}
		t := c.newTopic(key, n.GrantsQuest, where)
		t.Text, t.Hints, t.MoodChange = n.Text, n.Hints, n.MoodChange
		t.Gates = dialogue.Gates{
			QuestRequired: copyStrings(n.QuestRequired), QuestExcluded: copyStrings(n.QuestExcluded),
			QuestFlagRequired: copyMap(n.QuestFlagRequired), QuestFlagExcluded: copyMap(n.QuestFlagExcluded),
			RequiresItem: n.RequiresItem, ItemExcluded: n.ItemExcluded,
			GoldRequired: n.GoldRequired, MasterworkRequired: n.MasterworkRequired,
		}
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
		if _, dup := nodeTopic[n.Id]; !dup {
			nodeTopic[n.Id] = t
		}
	}

	// Rule 2: requires to heardRequired.
	for _, n := range nodes {
		if len(n.Requires) == 0 {
			continue
		}
		t := nodeTopic[n.Id]
		for _, req := range n.Requires {
			where := fmt.Sprintf("mob %d node %s requires %q", lf.MobId, n.Id, req)
			target, how := nodeTopic[req], "a node id"
			if target == nil {
				var unlockers []string
				for _, u := range nodes {
					for _, x := range u.Unlocks {
						if x == req {
							unlockers = append(unlockers, u.Id)
							break
						}
					}
				}
				if len(unlockers) == 0 {
					c.rep.Requires = append(c.rep.Requires, where+": no node has that id or unlocks it; nothing written")
					continue
				}
				target = nodeTopic[unlockers[0]]
				how = "unlocked by node " + unlockers[0]
				line := fmt.Sprintf("%s: no node has that id; unlocked by node %s (key %s)", where, unlockers[0], target.Key)
				if len(unlockers) > 1 {
					line += fmt.Sprintf("; also unlocked by %v, first used", unlockers[1:])
				}
				c.rep.Requires = append(c.rep.Requires, line)
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
		gated := ""
		if len(p.QuestRequired)+len(p.QuestExcluded) > 0 {
			gated = fmt.Sprintf(" (it was gated: questRequired %v questExcluded %v)", p.QuestRequired, p.QuestExcluded)
		}
		first := ""
		if len(p.Responses) > 0 {
			first = p.Responses[0]
		}
		switch class {
		case "catchall":
			if lf.Tree == nil && firstGreeting < 0 && f.Opener == nil && first != "" {
				f.Opener = &dialogue.Opener{Text: first}
				c.rep.Dropped = append(c.rep.Dropped, where+": catchall, first response became the opener (tree-less, no greeting)"+gated)
			} else {
				c.rep.Dropped = append(c.rep.Dropped, where+": catchall, dropped"+gated)
			}
		case "greeting":
			if lf.Tree == nil && i == firstGreeting && f.Opener == nil && first != "" {
				f.Opener = &dialogue.Opener{Text: first}
				c.rep.Dropped = append(c.rep.Dropped, where+": greeting, first response became the opener (tree-less)"+gated)
			} else {
				c.rep.Dropped = append(c.rep.Dropped, where+": greeting, dropped"+gated)
			}
		case "overlap":
			c.rep.Dropped = append(c.rep.Dropped, where+": overlap with a tree trigger, dropped"+gated)
		case "other":
			var kw string
			for _, k := range p.Keywords {
				if strings.TrimSpace(k) != "" {
					kw = k
					break
				}
			}
			key := f.uniqueKey(slug(kw))
			t := c.newTopic(key, p.GrantsQuest, where)
			t.Text = strings.Join(p.Responses, joinSep)
			t.MoodChange = p.MoodChange
			t.Gates = dialogue.Gates{
				QuestRequired: copyStrings(p.QuestRequired), QuestExcluded: copyStrings(p.QuestExcluded),
				QuestFlagRequired: copyMap(p.QuestFlagRequired), QuestFlagExcluded: copyMap(p.QuestFlagExcluded),
				RequiresItem: p.RequiresItem, ItemExcluded: p.ItemExcluded,
				GoldRequired: p.GoldRequired, MasterworkRequired: p.MasterworkRequired,
			}
			t.GivesItem, t.SetsQuestFlag, t.BumpsRep = p.GivesItem, p.SetsQuestFlag, p.BumpsRep
			t.GivesGold, t.ChargesGold = p.GivesGold, p.ChargesGold
			if p.GrantsQuest != "" {
				t.grants = append(t.grants, srcPattern)
			}
			if len(p.Moods) > 0 {
				c.rep.Notes = append(c.rep.Notes, fmt.Sprintf("%s: moods filter %v dropped (topics are never gated by mood), topic %s", where, p.Moods, key))
			}
			f.add(t)
		}
	}
	if f.Opener == nil {
		c.rep.Notes = append(c.rep.Notes, fmt.Sprintf("mob %d: no opener (no tree root, greeting or catch-all)", lf.MobId))
	}
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
		return
	}
	w.c.rep.Triggers = append(w.c.rep.Triggers, "delete "+w.where)
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
	w := &triggerWork{c: c, where: fmt.Sprintf("quest %s trigger %d", q.File, i), mob: tr.Mob}
	defer w.finish()

	var grant string
	for _, a := range tr.Actions {
		if a.Grant != "" {
			grant = a.Grant
			break
		}
	}
	if grant == "" {
		c.rep.Notes = append(c.rep.Notes, w.where+": item_give with no grant, left in the quest file")
		w.held = append(w.held, "no grant")
		return
	}
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
		f.add(t)
		c.rep.Notes = append(c.rep.Notes, fmt.Sprintf("mob %d topic %s created from %s (%s)", tr.Mob, key, w.where, t.Kind))
		return
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

	// The ask route and the give route were alternatives that paid the same
	// reward: an effect the topic already carries is not added again.
	grantSeen := false
	for k, a := range tr.Actions {
		switch actionName(a) {
		case "grant":
			if grantSeen {
				w.unmappable(k, a, fmt.Sprintf("second grant %q (a topic carries one)", a.Grant))
			}
			grantSeen = true
		case "consume_item":
			if a.ConsumeItem != tr.Item {
				w.unmappable(k, a, fmt.Sprintf("consumes %d, not the trigger item %d", a.ConsumeItem, tr.Item))
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
			}
		case "set_flag":
			nf := dialogue.QuestFlagSet{Key: a.SetFlag.Key, Value: a.SetFlag.Value}
			switch {
			case target.SetsQuestFlag == nil:
				target.SetsQuestFlag = &nf
			case *target.SetsQuestFlag != nf:
				w.clash(fmt.Sprintf("set_flag %s=%s clashes with topic %s setsQuestFlag %s=%s", nf.Key, nf.Value, target.Key, target.SetsQuestFlag.Key, target.SetsQuestFlag.Value))
			}
		case "give_item":
			switch {
			case target.GivesItem == 0:
				target.GivesItem = a.GiveItem
			case target.GivesItem != a.GiveItem:
				w.clash(fmt.Sprintf("give_item %d clashes with topic %s givesItem %d", a.GiveItem, target.Key, target.GivesItem))
			}
		case "give_gold":
			switch {
			case target.GivesGold == 0:
				target.GivesGold = a.GiveGold
			case target.GivesGold != a.GiveGold:
				w.clash(fmt.Sprintf("give_gold %d clashes with topic %s givesGold %d", a.GiveGold, target.Key, target.GivesGold))
			}
		case "npc_say":
			w.sayLine(k, a)
			c.rep.MergeLines = append(c.rep.MergeLines, fmt.Sprintf("mob %d topic %s: npc_say to fold into the topic text: %q", tr.Mob, target.Key, npcSayText(a.NpcSay)))
		default:
			w.unmappable(k, a, describeAction(a))
		}
	}
	target.grants = append(target.grants, srcItemGive)
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

func (c *converter) convertBehavior(b behaviorSource) {
	var branches []btNode
	askBranches(b.Tree, &branches)
	if len(branches) == 0 {
		return
	}
	for n, br := range branches {
		c.convertBranch(b, n+1, br)
	}
}

func (c *converter) convertBranch(b behaviorSource, n int, br btNode) {
	var conds, acts []btNode
	flatten(br, &conds, &acts)

	where := fmt.Sprintf("`%s` branch %d", b.Path, n)
	var keywords, treeOnly []string
	var gates dialogue.Gates
	for _, cd := range conds {
		switch cd.Check {
		case "keyword_match":
			if keywords == nil {
				keywords = paramList(cd.Params, "keywords")
			}
		case "player_has_quest":
			gates.QuestRequired = appendUnique(gates.QuestRequired, paramString(cd.Params, "quest"))
		case "player_missing_quest":
			gates.QuestExcluded = appendUnique(gates.QuestExcluded, paramString(cd.Params, "quest"))
		default:
			treeOnly = append(treeOnly, cd.Check)
		}
	}
	if len(keywords) > 0 {
		where += fmt.Sprintf(" (keywords %v)", keywords)
	}

	var speech []string
	firstSpeech := -1
	var grants, others []string
	var ferry, portal *btNode
	for k, a := range acts {
		switch {
		case speechActions[a.Do]:
			if s := spoken(a); s != "" {
				speech = append(speech, s)
				if firstSpeech < 0 {
					firstSpeech = k + 1
				}
			}
		case a.Do == "grant_quest":
			grants = append(grants, paramString(a.Params, "quest"))
		default:
			others = append(others, a.Do)
			if a.Do == "board_ferry" && ferry == nil {
				ferry = &acts[k]
			}
			if a.Do == "open_instance_portal" && portal == nil {
				portal = &acts[k]
			}
		}
	}
	firstText := ""
	if len(speech) > 0 {
		firstText = speech[0]
	}
	keep := strings.Join(append(append([]string(nil), treeOnly...), others...), ", ")
	if keep == "" {
		keep = "nothing else"
	}

	f := c.fileFor(b.MobId, "behavior-only mob, "+b.Path)
	if f == nil {
		c.rep.Behavior = append(c.rep.Behavior, where+": no file for the mob, not converted")
		return
	}
	baseKey := ""
	if len(keywords) > 0 {
		baseKey = slug(keywords[0])
	}
	if baseKey == "" {
		baseKey = fmt.Sprintf("ask_%d", n)
	}
	if len(keywords) == 0 && ferry == nil && portal == nil {
		c.rep.Clashes = append(c.rep.Clashes, fmt.Sprintf("%s: no keyword_match, so it ran on every ask; as a topic it runs only when chosen; owner decision", where))
	}
	newInfo := func(key string) *cTopic {
		t := &cTopic{}
		t.Key, t.Kind = key, dialogue.KindInfo
		t.Gates = dialogue.Gates{QuestRequired: copyStrings(gates.QuestRequired), QuestExcluded: copyStrings(gates.QuestExcluded)}
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
		t.Gates = dialogue.Gates{QuestRequired: copyStrings(gates.QuestRequired), QuestExcluded: copyStrings(gates.QuestExcluded)}
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
	ids := make([]int, 0, len(c.res.Files))
	for id := range c.res.Files {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		f := c.res.Files[id]
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
	TopicsCarryingGrants int

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
}

func newReport() *Report {
	return &Report{Patterns: map[string]int{}, Nodes: map[string]int{}, GrantsIn: map[string]int{}, GrantsOut: map[string]int{}}
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

// Reconciled reports whether every component and the total agree.
func (r *Report) Reconciled() bool {
	for _, s := range grantSources {
		if r.GrantsIn[s] != r.GrantsOut[s] {
			return false
		}
	}
	return r.GrantsInTotal() == r.GrantsOutTotal()
}

// Markdown renders the report.
func (r *Report) Markdown() string {
	var b strings.Builder
	b.WriteString("# Dialogue conversion report\n\n## Counts\n\n")
	b.WriteString("Patterns by class:\n\n")
	for _, k := range []string{"catchall", "greeting", "overlap", "other"} {
		fmt.Fprintf(&b, "- %s: %d\n", k, r.Patterns[k])
	}
	fmt.Fprintf(&b, "- total: %d\n\nNodes by kind:\n\n", sum(r.Patterns))
	for _, k := range []string{"offer", "handin", "info marked", "info"} {
		fmt.Fprintf(&b, "- %s: %d\n", k, r.Nodes[k])
	}
	fmt.Fprintf(&b, "- total: %d\n\n", sum(r.Nodes))
	b.WriteString("Grants by source (input: counted in the old content; output: carried by the converted topics):\n\n")
	b.WriteString("| source | input | output |\n|---|---|---|\n")
	for _, s := range grantSources {
		fmt.Fprintf(&b, "| %s | %d | %d |\n", s, r.GrantsIn[s], r.GrantsOut[s])
	}
	fmt.Fprintf(&b, "| total | %d | %d |\n\n", r.GrantsInTotal(), r.GrantsOutTotal())
	fmt.Fprintf(&b, "Topics carrying a grant: %d (merged item_give grants share their topic)\n\n", r.TopicsCarryingGrants)
	if r.Reconciled() {
		b.WriteString("Reconciled: yes\n\n")
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
	section("Requires mappings", r.Requires)
	section("Unmappable actions", r.Unmappable)
	section("Clashes and mismatches", r.Clashes)
	section("Merge lines", r.MergeLines)
	section("Triggers to delete", r.Triggers)
	section("Triggers held for a decision", r.HeldTriggers)
	section("Behavior branches", r.Behavior)
	section("New files", r.NewFiles)
	section("Gated offers", r.GatedOffers)
	section("Dropped patterns", r.Dropped)
	section("Notes", r.Notes)
	return b.String()
}
