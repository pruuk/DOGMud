package questengine

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/assert"
)

// seedTargetQuest: step 1 needs a throw at mob 9112, step 2 any throw.
func seedTargetQuest() *Engine {
	q := &QuestDef{
		QuestId: 300,
		Name:    "Target Test Quest",
		Steps: []QuestStep{
			{Id: "300-start", Description: "Begin"},
			{Id: "300-boar", Description: "Throw at the boar"},
			{Id: "300-any", Description: "Throw at anything"},
		},
		Triggers: []TriggerDef{
			{
				Event: "command", Command: "throw", Target: quests.TargetMob, Mob: 9112,
				Conditions: Conditions{Has: []string{"300-start"}, Missing: []string{"300-boar"}},
				Actions:    []ActionDef{{Grant: "300-boar"}},
			},
			{
				Event: "command", Command: "throw",
				Conditions: Conditions{Has: []string{"300-boar"}, Missing: []string{"300-any"}},
				Actions:    []ActionDef{{Grant: "300-any"}},
			},
		},
	}
	e := NewEngine()
	e.RegisterQuest(q)
	return e
}

func TestTargetFilterNeedsTheNamedMob(t *testing.T) {
	e := seedTargetQuest()
	player := newFullMockPlayer(100)
	player.quests["300-start"] = true

	// A throw at another mob does not count.
	r := e.Notify("command", EventDetails{UserId: 1, Command: "throw", TargetKind: quests.TargetMob, MobId: 7}, player, newFullMockActionContext(1, player))
	assert.False(t, player.HasQuest("300-boar"), "wrong mob advanced the step: %+v", r)

	// A throw at nothing does not count.
	e.Notify("command", EventDetails{UserId: 1, Command: "throw", TargetKind: quests.TargetNone}, player, newFullMockActionContext(1, player))
	assert.False(t, player.HasQuest("300-boar"))

	// A throw at the boar does.
	e.Notify("command", EventDetails{UserId: 1, Command: "throw", TargetKind: quests.TargetMob, MobId: 9112}, player, newFullMockActionContext(1, player))
	assert.True(t, player.HasQuest("300-boar"))
}

func TestTargetFilterKindMustMatch(t *testing.T) {
	e := seedTargetQuest()
	player := newFullMockPlayer(100)
	player.quests["300-start"] = true

	// MobId set but the kind is item (sell's shopkeeper rides along on an item target).
	e.Notify("command", EventDetails{UserId: 1, Command: "throw", TargetKind: quests.TargetItem, MobId: 9112}, player, newFullMockActionContext(1, player))
	assert.False(t, player.HasQuest("300-boar"))
}

func TestNotifyAllFiresEachTriggerOnce(t *testing.T) {
	q := &QuestDef{
		QuestId: 301,
		Name:    "Once Per Command",
		Steps:   []QuestStep{{Id: "301-start"}, {Id: "301-thrown"}},
		Triggers: []TriggerDef{{
			Event: "command", Command: "throw",
			Conditions: Conditions{Has: []string{"301-start"}},
			Actions:    []ActionDef{{GiveGold: 5}},
		}},
	}
	e := NewEngine()
	e.RegisterQuest(q)
	player := newFullMockPlayer(100)
	player.quests["301-start"] = true
	ctx := newFullMockActionContext(1, player)

	three := []EventDetails{
		{UserId: 1, Command: "throw", TargetKind: quests.TargetMob, MobId: 1},
		{UserId: 1, Command: "throw", TargetKind: quests.TargetMob, MobId: 2},
		{UserId: 1, Command: "throw", TargetKind: quests.TargetMob, MobId: 3},
	}
	r := e.NotifyAll("command", three, player, ctx)
	assert.True(t, r.Handled)
	assert.Equal(t, 5, ctx.givenGold, "an untargeted trigger must fire once per command, not once per target")
}

func TestNotifyAllMatchesALaterTarget(t *testing.T) {
	e := seedTargetQuest()
	player := newFullMockPlayer(100)
	player.quests["300-start"] = true
	details := []EventDetails{
		{UserId: 1, Command: "throw", TargetKind: quests.TargetMob, MobId: 7},
		{UserId: 1, Command: "throw", TargetKind: quests.TargetMob, MobId: 9112},
	}
	e.NotifyAll("command", details, player, newFullMockActionContext(1, player))
	assert.True(t, player.HasQuest("300-boar"), "the boar was the second target")
	assert.True(t, player.HasQuest("300-any"), "the any-throw trigger must retry on the boar after failing its conditions on mob 7")

	// The other order reaches the same end state.
	e = seedTargetQuest()
	player = newFullMockPlayer(100)
	player.quests["300-start"] = true
	details = []EventDetails{details[1], details[0]}
	e.NotifyAll("command", details, player, newFullMockActionContext(1, player))
	assert.True(t, player.HasQuest("300-boar"))
	assert.True(t, player.HasQuest("300-any"))
}

func TestCommandDetailsCarryTheTarget(t *testing.T) {
	d := CommandDetails(1, 5305, "cast", NoTarget().WithNoun("heal"))
	assert.Equal(t, EventDetails{UserId: 1, RoomId: 5305, Command: "cast", Noun: "heal", TargetKind: quests.TargetNone}, d)

	d = CommandDetails(1, 5305, "throw", Target{})
	assert.Equal(t, quests.TargetNone, d.TargetKind, "the zero Target reads as none")

	d = CommandDetails(1, 5305, "sell", TargetItem(10009).WithMob(&mobs.Mob{MobId: 9116}))
	assert.Equal(t, quests.TargetItem, d.TargetKind)
	assert.Equal(t, 10009, d.ItemId)
	assert.Equal(t, 9116, d.MobId)

	d = CommandDetails(1, 5305, "plant", TargetContainer("crate").WithItem(4))
	assert.Equal(t, EventDetails{UserId: 1, RoomId: 5305, Command: "plant", Noun: "crate", ItemId: 4, TargetKind: quests.TargetContainer}, d)
}

func TestTargetConstructors(t *testing.T) {
	assert.Equal(t, quests.TargetMob, TargetMob(&mobs.Mob{MobId: 9112}).Kind)
	assert.Equal(t, 9112, TargetMob(&mobs.Mob{MobId: 9112}).MobId)
	assert.Equal(t, quests.TargetNone, TargetMob(nil).Kind, "a vanished mob is no target")
	assert.Equal(t, quests.TargetNone, TargetItem(0).Kind)
	assert.Equal(t, quests.TargetSelf, TargetActor(1, 1, 0).Kind)
	assert.Equal(t, quests.TargetPlayer, TargetActor(1, 2, 0).Kind)
	assert.Equal(t, quests.TargetNone, TargetActor(1, 0, 0).Kind)
	// A mob instance id that resolves to nothing (no world loaded here) is no target.
	assert.Equal(t, quests.TargetNone, TargetActor(1, 0, 999999).Kind)
}
