package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/bounties"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/knowledge"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/life"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Owner ruling 2026-10-17: when a mob dies, every member of a killer's party
// who is in the room gets the kill for quest triggers and kill stats. The
// 1017 newbie-duo verify2 playtest found Ossaly stuck on First Blood because
// her leader's party autoattack landed every kill, and the mob_death quest
// notify reached only players in the damage map.
//
// Cast: the leader (1) damaged the mob in room 1; member 2 stands beside
// them; member 3 is in room 2; bystander 4 is in room 1 and in no party.

const (
	pkcMobId        = 7791
	pkcInstanceId   = 7791
	pkcRoom         = 1
	pkcOtherRoom    = 2
	pkcLeader       = 1
	pkcMember       = 2
	pkcAwayMember   = 3
	pkcBystander    = 4
	pkcProbeQuest   = 999310
	pkcBountyGold   = 40
	pkcLeaderDamage = 30
)

func seedPartyKill(t *testing.T) {
	t.Helper()
	place := map[int]int{pkcLeader: pkcRoom, pkcMember: pkcRoom, pkcAwayMember: pkcOtherRoom, pkcBystander: pkcRoom}
	recs := map[int]*users.UserRecord{}
	for id, room := range place {
		u := users.NewTestUser(id, "pkc", "Pkc", uint64(7700+id))
		u.Character.RoomId = room
		u.Character.Gold = 0
		recs[id] = u
	}
	t.Cleanup(users.SeedUsersForTest(recs))

	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		pkcRoom:      {RoomId: pkcRoom, Zone: "TestZone", Title: "Drill Yard", Exits: map[string]exit.RoomExit{}},
		pkcOtherRoom: {RoomId: pkcOtherRoom, Zone: "TestZone", Title: "Elsewhere", Exits: map[string]exit.RoomExit{}},
	}, map[string]*rooms.ZoneConfig{}))

	spec := &mobs.Mob{MobId: pkcMobId, Zone: "TestZone", Character: characters.Character{Name: "Training Dummy"}}
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{pkcMobId: spec}, map[int]*mobs.Mob{}))

	p := parties.New(pkcLeader)
	if p == nil {
		t.Fatal("fixture: leader already in a party")
	}
	for _, id := range []int{pkcMember, pkcAwayMember} {
		if !p.InvitePlayer(id) || !p.AcceptInvite(id) {
			t.Fatalf("fixture: user %d could not join the party", id)
		}
	}
	t.Cleanup(p.Disband)
}

func partyKillEvent() events.MobDeath {
	return events.MobDeath{
		MobId:         pkcMobId,
		InstanceId:    pkcInstanceId,
		RoomId:        pkcRoom,
		CharacterName: "Training Dummy",
		PlayerDamage:  map[int]int{pkcLeader: pkcLeaderDamage},
	}
}

func TestMobDeathQuestNotify_PartyInRoomGetsTheKill(t *testing.T) {
	seedPartyKill(t)
	t.Cleanup(questengine.ResetEngineForTest())
	// Each mob_death notification pays one gold, so a player's gold counts
	// how many times they were notified.
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: pkcProbeQuest,
		Name:    "mob_death probe",
		Steps:   []questengine.QuestStep{{Id: "start"}},
		Triggers: []questengine.TriggerDef{{
			Event:   "mob_death",
			Mob:     pkcMobId,
			Actions: []questengine.ActionDef{{GiveGold: 1}},
		}},
	})

	MobDeathQuestNotify(partyKillEvent())

	want := map[int]int{pkcLeader: 1, pkcMember: 1, pkcAwayMember: 0, pkcBystander: 0}
	for id, n := range want {
		if got := users.GetByUserId(id).Character.Gold; got != n {
			t.Errorf("user %d: notified %d times, want %d", id, got, n)
		}
	}
}

func TestCreditMobKill_PartyInRoomGetsTheKill(t *testing.T) {
	seedPartyKill(t)
	m := &mobs.Mob{
		MobId:      pkcMobId,
		InstanceId: pkcInstanceId,
		Character: characters.Character{
			Name:       "Training Dummy",
			RoomId:     pkcRoom,
			Zone:       "TestZone",
			Conditions: conditions.New(),
		},
	}
	m.Character.Life = life.NewMachine()
	if err := m.Character.Life.TransitionToDead(
		life.DeadData{DamageMap: map[int]int{pkcLeader: pkcLeaderDamage}},
		state.TransitionReason{Trigger: life.TriggerHealthZero},
	); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	creditMobKill(m)

	want := map[int]int{pkcLeader: 1, pkcMember: 1, pkcAwayMember: 0, pkcBystander: 0}
	for id, n := range want {
		kd := users.GetByUserId(id).Character.KD
		if kd.TotalKills != n || kd.Kills[pkcMobId] != n {
			t.Errorf("user %d: kills %d (of this mob %d), want %d", id, kd.TotalKills, kd.Kills[pkcMobId], n)
		}
	}
}

func TestMobDeathBountyClaim_PartyKillPaysTopDamagerOnce(t *testing.T) {
	seedPartyKill(t)
	if _, err := bounties.Declare(bounties.NPCIssuer(1), knowledge.MobSubject(pkcMobId),
		bounties.ConditionKill, 0, bounties.DeclareOpts{GoldOverride: pkcBountyGold}); err != nil {
		t.Fatalf("fixture: declare bounty: %v", err)
	}

	MobDeathBountyClaim(partyKillEvent())
	MobDeathBountyClaim(partyKillEvent())

	want := map[int]int{pkcLeader: pkcBountyGold, pkcMember: 0, pkcAwayMember: 0, pkcBystander: 0}
	for id, g := range want {
		if got := users.GetByUserId(id).Character.Gold; got != g {
			t.Errorf("user %d: gold %d, want %d", id, got, g)
		}
	}
}
