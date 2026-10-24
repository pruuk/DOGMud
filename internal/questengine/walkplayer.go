package questengine

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/narration"
)

// walkPlayer is the walkthrough's player: quest progress with the game's real
// step order (HasQuest holds for the current step and every earlier one; a
// grant only moves forward; a quest is entered only at its first step, as
// characters.GiveQuestToken does), plus items, flags, gold and room. It
// satisfies PlayerState for the trigger engine and backs a dialogue
// PlayerState for topics.
type walkPlayer struct {
	steps      map[int][]string // quest id -> step ids in order
	progress   map[int]string   // quest id -> current step id
	items      map[int]bool
	flags      map[string]string
	gold       int
	roomId     int
	masterwork bool
}

func newWalkPlayer(steps map[int][]string) *walkPlayer {
	return &walkPlayer{steps: steps, progress: map[int]string{}, items: map[int]bool{}, flags: map[string]string{}}
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

func (p *walkPlayer) index(questId int, step string) int {
	return slices.Index(p.steps[questId], step)
}

func (p *walkPlayer) HasQuest(token string) bool {
	id, step := splitToken(token)
	cur, ok := p.progress[id]
	if !ok {
		return false
	}
	want := p.index(id, step)
	return want >= 0 && want <= p.index(id, cur)
}

// grant moves a quest forward the way GiveQuestToken does.
func (p *walkPlayer) grant(token string) {
	id, step := splitToken(token)
	next := p.index(id, step)
	if next < 0 {
		return
	}
	cur, ok := p.progress[id]
	if !ok {
		if next == 0 {
			p.progress[id] = step
		}
		return
	}
	if next > p.index(id, cur) {
		p.progress[id] = step
	}
}

// hold puts the player on a token's step directly, for prerequisites.
func (p *walkPlayer) hold(token string) {
	id, step := splitToken(token)
	if p.index(id, step) >= 0 {
		p.progress[id] = step
	}
}

func (p *walkPlayer) clone() *walkPlayer {
	c := *p
	c.progress = maps.Clone(p.progress)
	c.items = maps.Clone(p.items)
	c.flags = maps.Clone(p.flags)
	return &c
}

func (p *walkPlayer) HasItem(itemId int) bool        { return p.items[itemId] }
func (p *walkPlayer) GetRoomId() int                 { return p.roomId }
func (p *walkPlayer) GetQuestFlag(key string) string { return p.flags[key] }
func (p *walkPlayer) GetGold() int                   { return p.gold }
func (p *walkPlayer) HasOwnMasterwork(int) bool      { return p.masterwork }

// dialogueState is a dialogue PlayerState over p. Every info topic counts as
// heard and every quest as open: the walk tests reachability, not pacing.
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
		HasHeard:  func(string) bool { return true },
		MarkHeard: func(string) {},
		QuestOpen: func(int) bool { return true },
	}
}

// walkCtx is the walkthrough's ActionContext: grants, items, gold and flags
// change the walk player; everything else (narration, spawns, teleports,
// timed scenes) is a no-op, because the walk follows quest state only.
// QueueSequence runs the sequence's on_complete actions, as the real bridge
// does after the lines finish.
type walkCtx struct{ p *walkPlayer }

func (c *walkCtx) GrantQuest(token string)                 { c.p.grant(token) }
func (c *walkCtx) ConsumeItem(itemId int)                  { delete(c.p.items, itemId) }
func (c *walkCtx) GiveItem(itemId int) error               { c.p.items[itemId] = true; return nil }
func (c *walkCtx) GiveGold(amount int)                     { c.p.gold += amount }
func (c *walkCtx) ChargeGold(amount int) error             { c.p.gold -= amount; return nil }
func (c *walkCtx) Narrate(narration.Variants)              {}
func (c *walkCtx) SpawnMob(SpawnDef)                       {}
func (c *walkCtx) SpawnItem(SpawnDef)                      {}
func (c *walkCtx) TeachSpell(string)                       {}
func (c *walkCtx) TrainSkill(string, int)                  {}
func (c *walkCtx) IncreaseStat(string, int)                {}
func (c *walkCtx) LearnRecipe(string)                      {}
func (c *walkCtx) ApplyStatusCondition(StatusConditionDef) {}
func (c *walkCtx) Teleport(int)                            {}
func (c *walkCtx) LockExits(ExitLock)                      {}
func (c *walkCtx) UnlockExits(ExitLock)                    {}
func (c *walkCtx) QueueNpcSay(NpcSayDef)                   {}
func (c *walkCtx) GiveMutation()                           {}
func (c *walkCtx) SetQuestFlag(key, value string)          { c.p.flags[key] = value }
func (c *walkCtx) BumpRep(string, int)                     {}
func (c *walkCtx) GetUserId() int                          { return 0 }

func (c *walkCtx) QueueSequence(s SequenceDef) {
	for _, a := range s.OnComplete {
		_ = ExecuteAction(a, c)
	}
}
