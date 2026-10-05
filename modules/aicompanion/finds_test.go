package aicompanion

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// A share of a companion's searches rolls for a bauble, and only for an
// owner who has agreed to the model and has their own key to name it with.
func TestBaubleRollsOnlyForAnOwnerWithAKey(t *testing.T) {
	hw := newHollowWorld(t)
	m := hw.m
	c := &controller{ownerUserId: 1, profile: hw.p, mind: newMind(1, hw.p), instanceId: 4242}
	m.ctrls[1] = c
	m.cfg.CompanionBaubleChance = 100

	if got := m.baubleSearchFor(4242); got != 0 {
		t.Fatal("no consent, no key: no roll")
	}
	m.bonds.Users[1] = &bondRecord{Consented: true}
	m.syncConsent()
	if got := m.baubleSearchFor(4242); got != 0 {
		t.Fatal("consent but no key of their own: no roll")
	}
	m.relays.ready(1, `player-model`, false)
	if got := m.baubleSearchFor(4242); got != 1 {
		t.Fatalf("a key: rolls for her owner, got %d", got)
	}
	m.cfg.CompanionBaubleChance = 0
	if got := m.baubleSearchFor(4242); got != 0 {
		t.Fatal("chance 0: never")
	}
	m.cfg.CompanionBaubleChance = 15
	rolls := 0
	for i := 0; i < 4000; i++ {
		if m.baubleSearchFor(4242) == 1 {
			rolls++
		}
	}
	if rolls < 450 || rolls > 750 {
		t.Fatalf("about 15%% of searches roll: %d of 4000", rolls)
	}
	if got := m.baubleSearchFor(9999); got != 0 {
		t.Fatal("a mob that is no bonded companion never rolls")
	}
}

// What a search turns up is put to her, and judges the search.
func TestSearchFindingsReachHer(t *testing.T) {
	owner, _, _, her := harmWorld(t, configs.PVPDisabled)
	m, c, _ := strangerModule()
	m.ctrls = map[int]*controller{1: c}
	m.onSearched(her.InstanceId, []string{`a hidden way out, north`})
	if len(c.pending) == 0 || c.pending[len(c.pending)-1].Kind != `searched` {
		t.Fatalf("she is told what she found: %+v", c.pending)
	}
	if !strings.Contains(formatStimulus(c.pending[len(c.pending)-1], `Corvin`), `a hidden way out, north`) {
		t.Fatal("in words")
	}
	c.pendingAct = &pendingAction{Verb: `search`, Before: snapshotOf(her)}
	m.verifyPending(c, her, owner.Character.Name)
	last := c.mind.RecentLines[len(c.mind.RecentLines)-1].Text
	if !strings.Contains(last, `turned up: a hidden way out, north`) {
		t.Fatalf("the search is remembered for what it found: %q", last)
	}
	n := len(c.pending)
	m.onSearched(her.InstanceId, nil)
	if len(c.pending) != n {
		t.Fatal("finding nothing is not news")
	}
	m.onBaubleFound(her.InstanceId, `a tarnished silver locket`, true)
	if c.pending[len(c.pending)-1].Kind != `found` {
		t.Fatal("a bauble in her pack is put to her")
	}
}

// corpseWorld puts a game animal's body, killed by her owner, in the room.
func corpseWorld(t *testing.T, room *rooms.Room, groups []string, loot bool) *rooms.Corpse {
	t.Helper()
	spec := &mobs.Mob{MobId: 9139, Groups: groups}
	spec.Character.Name = `pronghorn`
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{9139: spec}, nil))
	// Her owner's kill, still held for them a while (another player may
	// not touch it until RoundOwnedUntil passes).
	c := rooms.Corpse{MobId: 9139, RoundCreated: 77, OwnerUserIds: []int{1}, RoundOwnedUntil: 1 << 40}
	c.Character.Name = `pronghorn`
	if loot {
		c.Loot.Gold = 3
	}
	room.Corpses = append(room.Corpses, c)
	return &room.Corpses[len(room.Corpses)-1]
}

// Asked to butcher a body, she can: the engine's salvage, aimed at that one,
// once it is picked clean.
func TestTheModelCanButcherABody(t *testing.T) {
	owner, _, room, her := harmWorld(t, configs.PVPDisabled)
	corpse := corpseWorld(t, room, []string{`animal`}, true)
	m, c, _ := strangerModule()
	sc := &scene{RoomId: room.RoomId, byRef: map[string]*thing{
		`t1`: {Ref: `t1`, Kind: `corpse`, Name: `the body of pronghorn`, CorpseRef: corpseRef(corpse)},
	}}
	asked := []stimulus{{Kind: `heard`, FromOwner: true, Text: `Butcher that, would you?`}}
	if out := m.performAction(c, her, owner, sc, ActionProposal{Verb: `salvage`, Ref: `t1`}, asked, 0, 0); out.Issued {
		t.Fatalf("not while there is loot on it: %+v", out)
	}
	corpse.Loot.Gold = 0
	if out := m.performAction(c, her, owner, sc, ActionProposal{Verb: `salvage`, Ref: `t1`},
		[]stimulus{{Kind: `heard`, Speaker: `Bram`, AskerUserId: 2}}, 0, 0); out.Issued {
		t.Fatalf("not at a stranger's word: %+v", out)
	}
	out := m.performAction(c, her, owner, sc, ActionProposal{Verb: `salvage`, Ref: `t1`}, asked, 0, 0)
	if !out.Issued || out.Pending == nil || out.Pending.Verb != `salvage` {
		t.Fatalf("asked, she butchers it: %+v", out)
	}
	if got := salvageCommand(corpse); got != `salvage 9139:77` {
		t.Fatalf("that body and no other: %q", got)
	}
}

// Idle, a hunter or a cook butchers game her owner killed, once it is
// picked clean, even when the arrangement is to ask first; never a body
// still holding loot, never someone else's kill, never humanoids.
func TestButcheringGameIdle(t *testing.T) {
	_, _, room, _ := harmWorld(t, configs.PVPDisabled)
	corpse := corpseWorld(t, room, []string{`animal`}, true)
	if butcherHere(room, 1) != nil {
		t.Fatal("still holding loot")
	}
	corpse.Loot.Gold = 0
	if butcherHere(room, 1) == nil {
		t.Fatal("game her owner killed, picked clean")
	}
	if butcherHere(room, 2) != nil {
		t.Fatal("not someone else's kill while it is still theirs")
	}
	person := &mobs.Mob{MobId: 9139, Groups: []string{`humanoid`}}
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{9139: person}, nil))
	if butcherHere(room, 1) != nil {
		t.Fatal("game only: nobody butchers a person for the pot")
	}
	profiles, _ := loadProfiles()
	if profiles[`hal`].pastimeWeight(`butcher`) == 0 || profiles[`mara`].pastimeWeight(`butcher`) == 0 {
		t.Fatal("Hal and Mara butcher game")
	}
	if profiles[`liesl`].pastimeWeight(`butcher`) != 0 {
		t.Fatal("Liesl does not")
	}
}
