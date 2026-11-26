package questengine

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// walkPlayer is the walkthrough's player: quest progress with the game's real
// step order, read from the same path helpers Character uses
// (quests.Quest.IsStepPassedOnPath and IsStepAfterOnPath) with the walk
// player's own flags: HasQuest holds for the current step and every earlier
// one on the player's path, never another side's; a grant only moves forward
// on that path; a quest is entered only at "start" or the only step of a
// one-step quest, as characters.GiveQuestToken does. Plus items, flags, gold
// and room. It satisfies PlayerState for the trigger engine and backs a
// dialogue PlayerState for topics.
type walkPlayer struct {
	quests     map[int]*quests.Quest // quest id -> quest, shared by every walk
	progress   map[int]string        // quest id -> current step id
	items      map[int]bool
	flags      map[string]string
	heard      map[string]bool // dialogue.HeardKey -> heard
	spells     map[string]bool // spells a teach_spell taught during the walk
	gold       int
	roomId     int
	masterwork bool
}

// newWalkPlayer is a player holding no quest, items, flags or gold, whose
// step order reads qs (every quest the walk registered).
func newWalkPlayer(qs map[int]*quests.Quest) *walkPlayer {
	return &walkPlayer{quests: qs, progress: map[int]string{}, items: map[int]bool{},
		flags: map[string]string{}, heard: map[string]bool{}, spells: map[string]bool{}}
}

func splitToken(token string) (int, string) {
	id, step, ok := strings.Cut(token, "-")
	if !ok {
		return 0, ""
	}
	n, err := strconv.Atoi(id)
	if err != nil {
		return 0, ""
	}
	return n, step
}

// quest is the walk's quest for token, and its step id, when the quest has
// that step.
func (p *walkPlayer) quest(token string) (*quests.Quest, int, string) {
	id, step := splitToken(token)
	q := p.quests[id]
	if q == nil {
		return nil, id, step
	}
	if _, ok := q.Step(step); !ok {
		return nil, id, step
	}
	return q, id, step
}

func (p *walkPlayer) HasQuest(token string) bool {
	q, id, step := p.quest(token)
	cur, ok := p.progress[id]
	if q == nil || !ok {
		return false
	}
	return step == cur || q.IsStepPassedOnPath(step, cur, p.GetQuestFlag)
}

// grant moves a quest forward the way GiveQuestToken does
// (quests.IsTokenAfterOnPath): a quest not yet begun is entered only at
// "start", or at the only step of a one-step quest, and a begun one moves
// only forward on the player's path.
func (p *walkPlayer) grant(token string) {
	q, id, step := p.quest(token)
	if q == nil {
		return
	}
	if q.IsStepAfterOnPath(p.progress[id], step, p.GetQuestFlag) {
		p.progress[id] = step
	}
}

// hold puts the player on a token's step directly, for prerequisites.
func (p *walkPlayer) hold(token string) {
	if q, id, step := p.quest(token); q != nil {
		p.progress[id] = step
	}
}

func (p *walkPlayer) clone() *walkPlayer {
	c := *p
	c.progress = maps.Clone(p.progress)
	c.items = maps.Clone(p.items)
	c.flags = maps.Clone(p.flags)
	c.heard = maps.Clone(p.heard)
	c.spells = maps.Clone(p.spells)
	return &c
}

// fingerprint names p's state, so the walk can keep one copy of each
// distinct branch.
func (p *walkPlayer) fingerprint() string {
	var b strings.Builder
	for _, id := range slices.Sorted(maps.Keys(p.progress)) {
		fmt.Fprintf(&b, "q%d=%s;", id, p.progress[id])
	}
	for _, id := range slices.Sorted(maps.Keys(p.items)) {
		if p.items[id] {
			fmt.Fprintf(&b, "i%d;", id)
		}
	}
	for _, k := range slices.Sorted(maps.Keys(p.flags)) {
		fmt.Fprintf(&b, "f%s=%s;", k, p.flags[k])
	}
	for _, k := range slices.Sorted(maps.Keys(p.heard)) {
		fmt.Fprintf(&b, "h%s;", k)
	}
	for _, k := range slices.Sorted(maps.Keys(p.spells)) {
		fmt.Fprintf(&b, "s%s;", k)
	}
	fmt.Fprintf(&b, "g%d;r%d;m%t", p.gold, p.roomId, p.masterwork)
	return b.String()
}

func (p *walkPlayer) HasItem(itemId int) bool        { return p.items[itemId] }
func (p *walkPlayer) GetRoomId() int                 { return p.roomId }
func (p *walkPlayer) GetQuestFlag(key string) string { return p.flags[key] }
func (p *walkPlayer) GetGold() int                   { return p.gold }
func (p *walkPlayer) HasOwnMasterwork(int) bool      { return p.masterwork }

// MappedCount reads a full map (MapReader): the walk proves an explore step
// reachable, so every room of its area counts as seen.
func (p *walkPlayer) MappedCount(roomIds []int) int { return len(roomIds) }

// dialogueState is a dialogue PlayerState over p. A topic is heard once the
// walk marked it (walkEnv.hear, or Apply of an info topic); every quest
// counts as open, since the walk tests reachability, not pacing.
func (p *walkPlayer) dialogueState() *dialogue.PlayerState {
	return &dialogue.PlayerState{
		HasQuest:    p.HasQuest,
		HasItem:     p.HasItem,
		CarriesItem: p.HasItem,
		RemoveItem: func(id int) bool {
			if !p.items[id] {
				return false
			}
			delete(p.items, id)
			return true
		},
		GiveQuest:        p.grant,
		GiveItem:         func(id int) bool { p.items[id] = true; return true },
		GetQuestFlag:     p.GetQuestFlag,
		SetQuestFlag:     func(k, v string) { p.flags[k] = v },
		BumpRep:          func(string, int) {},
		GiveGold:         func(n int) { p.gold += n },
		HasOwnMasterwork: p.HasOwnMasterwork,
		HasGold:          func(n int) bool { return p.gold >= n },
		ChargeGold: func(n int) bool {
			if p.gold < n {
				return false
			}
			p.gold -= n
			return true
		},
		HasHeard:  func(key string) bool { return p.heard[key] },
		MarkHeard: func(key string) { p.heard[key] = true },
		QuestOpen: func(int) bool { return true },
	}
}

// walkCtx is the walkthrough's ActionContext: grants, items, gold, flags and
// taught spells change the walk player; everything else (narration, spawns, teleports,
// timed scenes) is a no-op, because the walk follows quest state only.
// QueueSequence runs the sequence's on_complete actions, as the real bridge
// does after the lines finish.
type walkCtx struct{ p *walkPlayer }

// BumpStepCount reaches every count: N at once (StepCounter): the walk
// proves a step is reachable, not that it can be repeated.
func (c *walkCtx) BumpStepCount(int, string, string) (int, bool) { return 1 << 30, true }

func (c *walkCtx) GrantQuest(token string)                 { c.p.grant(token) }
func (c *walkCtx) ConsumeItem(itemId int)                  { delete(c.p.items, itemId) }
func (c *walkCtx) GiveItem(itemId int) error               { c.p.items[itemId] = true; return nil }
func (c *walkCtx) GiveGold(amount int)                     { c.p.gold += amount }
func (c *walkCtx) ChargeGold(amount int) error             { c.p.gold -= amount; return nil }
func (c *walkCtx) Narrate(narration.Variants)              {}
func (c *walkCtx) SpawnMob(SpawnDef)                       {}
func (c *walkCtx) SpawnItem(SpawnDef)                      {}
func (c *walkCtx) TeachSpell(spellId string)               { c.p.spells[spellId] = true }
func (c *walkCtx) TrainSkill(string, int)                  {}
func (c *walkCtx) IncreaseStat(string, int)                {}
func (c *walkCtx) LearnRecipe(string)                      {}
func (c *walkCtx) ApplyStatusCondition(StatusConditionDef) {}
func (c *walkCtx) Teleport(int)                            {}
func (c *walkCtx) QueueNpcSay(NpcSayDef)                   {}
func (c *walkCtx) GiveMutation()                           {}
func (c *walkCtx) DiscoverHiddenNoun(string)               {}
func (c *walkCtx) SetQuestFlag(key, value string)          { c.p.flags[key] = value }
func (c *walkCtx) BumpRep(string, int)                     {}
func (c *walkCtx) GetUserId() int                          { return 0 }

func (c *walkCtx) QueueSequence(s SequenceDef) {
	for _, a := range s.OnComplete {
		_ = ExecuteAction(a, c)
	}
}
