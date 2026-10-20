package dialogue

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/menu"
)

// QuestInfo is what the dialogue validators need about one quest.
type QuestInfo struct {
	Title      string
	StartToken string
	EndToken   string
}

// Problem is one validator finding. Every finding refuses: the phase 1
// conversion clears the backlog in one pass, so there is no warning tier.
type Problem struct {
	MobId int
	Where string
	Msg   string
}

func (p Problem) String() string { return fmt.Sprintf("mob %d %s: %s", p.MobId, p.Where, p.Msg) }

// World is everything the dialogue validators read. Boot and CI build it
// from the loaded registries (RegistryWorld); the phase 6 builder overlays
// pending edits on it, so a save is checked against the world as it would
// be. Nil funcs report "does not exist".
type World struct {
	Files        []*DialogueFile
	QuestById    func(questId int) (QuestInfo, bool)
	QuestOfToken func(token string) (QuestInfo, bool)
	FlagDeclared func(key, value string) bool
	ItemExists   func(id int) bool
	// BehaviorTopics lists the topic_is keys in a mob's behavior tree.
	// Nil skips the behavior checks (no trees in this world).
	BehaviorTopics func(mobId int) []string
	// ItemGiveGrants names quest item_give triggers that carry a grant
	// action ("quest 10 trigger 2"). Hand-ins are NPC topics (R7).
	ItemGiveGrants []string
}

var topicKeyPattern = regexp.MustCompile(`^[a-z0-9_]+$`)

// dashRunes are the em and en dash, built from code points so no dash
// character sits in source.
var dashRunes = string([]rune{0x2014, 0x2013})

// ValidateWorld runs every dialogue rule over every file in w.
func ValidateWorld(w World) []Problem {
	var out []Problem
	for _, df := range w.Files {
		out = append(out, validateTopicFile(w, df)...)
	}
	for _, where := range w.ItemGiveGrants {
		out = append(out, Problem{Where: where, Msg: "item_give triggers may not grant a quest token: hand-ins are NPC topics"})
	}
	return out
}

func validateTopicFile(w World, df *DialogueFile) []Problem {
	var out []Problem
	add := func(where, format string, a ...any) {
		out = append(out, Problem{MobId: df.MobId, Where: where, Msg: fmt.Sprintf(format, a...)})
	}
	semi := func(where, text string) {
		if strings.Contains(text, ";") {
			add(where, "semicolons are the command separator and truncate spoken text, rewrite without ';'")
		}
	}

	for i, g := range df.Greetings {
		semi(fmt.Sprintf("greeting %d", i+1), g.Text)
	}
	if df.Opener != nil {
		semi("opener", df.Opener.Text)
		for i, v := range df.Opener.Variants {
			where := fmt.Sprintf("opener variant %d", i+1)
			semi(where, v.Text)
			out = append(out, gateRefProblems(w, df.MobId, where, v.Gates, Effects{})...)
		}
	}

	keys, infoKeys, infoLabels := map[string]bool{}, map[string]bool{}, map[string]string{}
	for _, t := range df.Topics {
		if t.Kind == KindInfo {
			infoKeys[t.Key] = true
		}
	}
	for i := range df.Topics {
		t := &df.Topics[i]
		where := fmt.Sprintf("topic %q", t.Key)
		if !topicKeyPattern.MatchString(t.Key) {
			add(where, "key must match ^[a-z0-9_]+$")
		}
		if keys[t.Key] {
			add(where, "duplicate key")
		}
		keys[t.Key] = true

		switch t.Kind {
		case KindInfo:
			if t.Quest != 0 || t.PlayerLine != "" {
				add(where, "quest and playerLine belong on offer and handin topics only")
			}
			if t.Breadcrumb != "" {
				add(where, "breadcrumb belongs on offer topics only")
			}
			lower := strings.ToLower(t.Label)
			if prev, dup := infoLabels[lower]; dup {
				add(where, "label repeats topic %q", prev)
			} else {
				infoLabels[lower] = t.Key
			}
		case KindOffer, KindHandIn:
			if t.Quest == 0 || t.PlayerLine == "" {
				add(where, "%s topics need quest and playerLine", t.Kind)
			}
			if t.Marked {
				add(where, "marked belongs on info topics only")
			}
			if t.Kind == KindHandIn && t.Breadcrumb != "" {
				add(where, "breadcrumb belongs on offer topics only")
			}
			if q, ok := lookupQuest(w, t.Quest); !ok {
				add(where, "quest %d does not exist", t.Quest)
			} else {
				want := q.StartToken
				if t.Kind == KindHandIn {
					want = q.EndToken
				}
				if t.GrantsQuest != want {
					add(where, "%s must grant %q, not %q", t.Kind, want, t.GrantsQuest)
				}
				if t.Label != q.Title {
					add(where, "label must be the quest title %q", q.Title)
				}
			}
		default:
			add(where, "kind must be info, offer or handin, not %q", t.Kind)
		}

		lint := menu.Menu{Entries: []menu.Entry{{Id: t.Key, Label: t.Label, Detail: t.PlayerLine}}}
		for _, p := range menu.LintLabels(lint) {
			add(where, "%s", p)
		}
		if strings.ContainsAny(t.Breadcrumb, dashRunes) {
			add(where+" breadcrumb", "no em or en dash in new text")
		}
		semi(where+" text", t.Text)
		semi(where+" breadcrumb", t.Breadcrumb)
		for _, k := range t.HeardRequired {
			if !infoKeys[k] {
				add(where, "heardRequired %q is not an info topic in this file", k)
			}
		}
		if t.ChargesGold > 0 && t.GoldRequired < t.ChargesGold {
			add(where, "chargesGold %d needs goldRequired of at least %d", t.ChargesGold, t.ChargesGold)
		}
		out = append(out, gateRefProblems(w, df.MobId, where, t.Gates, t.Effects)...)
	}

	if cyc := heardCycle(df); cyc != "" {
		add("topics", "heardRequired cycle through %q", cyc)
	}

	if w.BehaviorTopics != nil {
		tree := map[string]bool{}
		for _, k := range w.BehaviorTopics(df.MobId) {
			tree[k] = true
		}
		for _, t := range df.Topics {
			if t.Behavior && !tree[t.Key] {
				add(fmt.Sprintf("topic %q", t.Key), "behavior: true but the mob's tree has no topic_is %q", t.Key)
			}
		}
		treeKeys := make([]string, 0, len(tree))
		for k := range tree {
			treeKeys = append(treeKeys, k)
		}
		sort.Strings(treeKeys)
		for _, k := range treeKeys {
			if !keys[k] {
				add("behavior tree", "topic_is %q names no topic on this mob", k)
			}
		}
	}
	return out
}

func lookupQuest(w World, id int) (QuestInfo, bool) {
	if w.QuestById == nil {
		return QuestInfo{}, false
	}
	return w.QuestById(id)
}

// gateRefProblems carries the old validator's reference rules over to
// topics: a grant must be excluded (or the offer repeats), the quest's end
// token must be excluded (or a finished quest is re-offered), and every
// token, flag and item must exist.
func gateRefProblems(w World, mobId int, where string, g Gates, e Effects) []Problem {
	var out []Problem
	add := func(format string, a ...any) {
		out = append(out, Problem{MobId: mobId, Where: where, Msg: fmt.Sprintf(format, a...)})
	}
	excluded := func(tok string) bool {
		for _, x := range g.QuestExcluded {
			if x == tok {
				return true
			}
		}
		return false
	}
	quest := func(tok string) (QuestInfo, bool) {
		if w.QuestOfToken == nil {
			return QuestInfo{}, false
		}
		return w.QuestOfToken(tok)
	}
	if e.GrantsQuest != "" {
		if !excluded(e.GrantsQuest) {
			add("grantsQuest %q must also appear in questExcluded, or the offer repeats forever", e.GrantsQuest)
		}
		if q, ok := quest(e.GrantsQuest); ok && !excluded(q.EndToken) {
			add("questExcluded must include the end token %q, or a player who finished the quest gets it again", q.EndToken)
		}
	}
	for _, tok := range append(append([]string{e.GrantsQuest}, g.QuestRequired...), g.QuestExcluded...) {
		if tok == "" {
			continue
		}
		if _, ok := quest(tok); !ok {
			add("quest token %q does not exist", tok)
		}
	}
	flag := func(k, v string) bool { return w.FlagDeclared != nil && w.FlagDeclared(k, v) }
	for _, m := range []map[string]string{g.QuestFlagRequired, g.QuestFlagExcluded} {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if !flag(k, m[k]) {
				add("quest flag %q=%q is not declared by any quest", k, m[k])
			}
		}
	}
	if e.SetsQuestFlag != nil && !flag(e.SetsQuestFlag.Key, e.SetsQuestFlag.Value) {
		add("setsQuestFlag %q=%q is not declared by any quest", e.SetsQuestFlag.Key, e.SetsQuestFlag.Value)
	}
	for _, id := range []int{g.RequiresItem, g.ItemExcluded, e.GivesItem} {
		if id != 0 && (w.ItemExists == nil || !w.ItemExists(id)) {
			add("item %d does not exist", id)
		}
	}
	return out
}

// heardCycle returns a key on a heardRequired cycle, or "".
func heardCycle(df *DialogueFile) string {
	deps := map[string][]string{}
	for _, t := range df.Topics {
		deps[t.Key] = t.HeardRequired
	}
	const (
		unseen = iota
		active
		done
	)
	state := map[string]int{}
	var visit func(k string) string
	visit = func(k string) string {
		switch state[k] {
		case active:
			return k
		case done:
			return ""
		}
		state[k] = active
		for _, d := range deps[k] {
			if c := visit(d); c != "" {
				return c
			}
		}
		state[k] = done
		return ""
	}
	for _, t := range df.Topics {
		if c := visit(t.Key); c != "" {
			return c
		}
	}
	return ""
}
