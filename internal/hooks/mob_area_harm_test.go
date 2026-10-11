package hooks

import (
	"slices"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/companionai"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// areaHarmWorld is one room holding Corvin (user 1), Bram (user 2), a mob
// charmed by Corvin (the caster, 42), an ordinary creature (43) and one
// players may not attack (44), with PvP as given.
func areaHarmWorld(t *testing.T, pvp string) (*mobs.Mob, *rooms.Room) {
	t.Helper()
	drainEventQueueOnCleanup(t)
	cfg := configs.GetConfig()
	cfg.GamePlay.PVP = configs.ConfigString(pvp)
	cfg.GamePlay.PVPMinimumSkillRanks = 0
	configs.SetConfigForTest(t, cfg)

	room := &rooms.Room{RoomId: 1, Zone: `AreaZone`, Title: `A Clearing`, Biome: `city`, Lamp: rooms.LampPtr(90)}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{1: room},
		map[string]*rooms.ZoneConfig{`AreaZone`: {Name: `AreaZone`, RoomId: 1, RoomIds: map[int]struct{}{1: {}}}}))
	owner := users.NewTestUser(1, `corvin`, `Corvin`, 0)
	other := users.NewTestUser(2, `bram`, `Bram`, 0)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{1: owner, 2: other}))
	room.AddPlayer(1)
	room.AddPlayer(2)

	add := func(id int, name string) *mobs.Mob {
		m := &mobs.Mob{InstanceId: id, Character: characters.Character{
			Name: name, RoomId: 1, Health: 100, Conditions: conditions.New(),
		}}
		m.Character.HealthMax.Value = 100
		mobs.SetInstanceForTest(id, m)
		t.Cleanup(func() { mobs.SetInstanceForTest(id, nil) })
		room.AddMob(id)
		return m
	}
	caster := add(42, `Mara`)
	caster.Character.Charm(1, characters.CharmPermanent, ``)
	add(43, `a wolf`)
	add(44, `a caravan guard`).PlayerAttackImmune = true
	return caster, room
}

func bondedIs(t *testing.T, ids ...int) {
	t.Helper()
	companionai.SetBondedCheck(func(id int) bool { return slices.Contains(ids, id) })
	t.Cleanup(func() { companionai.SetBondedCheck(nil) })
}

// A bonded companion's area harm, at resolution, spares whatever her owner
// could not harm: a creature players may not attack, and a person her owner
// could not fight here. Her owner is spared as before.
func TestBondedAreaHarmSparesWhatHerOwnerCouldNotHarm(t *testing.T) {
	caster, room := areaHarmWorld(t, configs.PVPDisabled)
	bondedIs(t, caster.InstanceId)
	mobIds, userIds := mobAreaHarmTargets(caster, room)
	if !slices.Equal(mobIds, []int{43}) {
		t.Fatalf("only the creature her owner could attack: %v", mobIds)
	}
	if len(userIds) != 0 {
		t.Fatalf("with PvP off nobody is caught: %v", userIds)
	}
}

// With PvP on, a person her owner could fight is caught, unless they travel
// with her owner.
func TestBondedAreaHarmFollowsPvpAndTheParty(t *testing.T) {
	caster, room := areaHarmWorld(t, configs.PVPEnabled)
	bondedIs(t, caster.InstanceId)
	if _, userIds := mobAreaHarmTargets(caster, room); !slices.Equal(userIds, []int{2}) {
		t.Fatalf("with PvP on, Bram is caught and her owner is not: %v", userIds)
	}

	p := parties.New(1)
	t.Cleanup(p.Disband)
	p.InvitePlayer(2)
	p.AcceptInvite(2)
	if _, userIds := mobAreaHarmTargets(caster, room); len(userIds) != 0 {
		t.Fatalf("Bram travels with her owner, so he is spared: %v", userIds)
	}
}

// Playtest of 89c58a9f6: Core Discharge struck the Guardian's own Grapnel
// Warden and Hull Sweeper (one crit for 3721), and an add dying scattered
// its packmates, the Guardian among them, out of its windup. A wild mob's
// area harm spares the mobs it ConsidersAnAlly (its own kind or species,
// the rule callforhelp and the pack scatter use), and still hits the
// players and any other creature.
func TestWildAreaHarmSparesTheCastersAllies(t *testing.T) {
	_, room := areaHarmWorld(t, configs.PVPDisabled)
	boss := mobs.GetInstance(42)
	boss.Character.RemoveCharm() // fixture: this caster is wild, not charmed
	boss.MobId, boss.Character.SpeciesId = 9562, 45
	addMob := mobs.GetInstance(43)
	addMob.MobId, addMob.Character.SpeciesId = 9586, 45 // its summoned add, same kind
	stranger := mobs.GetInstance(44)
	stranger.MobId, stranger.Character.SpeciesId = 9999, 7
	stranger.PlayerAttackImmune = false

	mobIds, userIds := mobAreaHarmTargets(boss, room)
	if !slices.Equal(mobIds, []int{44}) {
		t.Fatalf("the add is spared and the stranger is not: %v", mobIds)
	}
	slices.Sort(userIds)
	if !slices.Equal(userIds, []int{1, 2}) {
		t.Fatalf("every player is still hit: %v", userIds)
	}
}

// The two exceptions to the ally skip: a same-kind creature charmed by a
// player serves that player and is hit, and two of a species whose groups
// hate each other (a warden and a bandit) are no allies, so it is hit too.
func TestWildAreaHarmHitsCharmedKinAndHatedKin(t *testing.T) {
	_, room := areaHarmWorld(t, configs.PVPDisabled)
	boss := mobs.GetInstance(42)
	boss.Character.RemoveCharm()
	boss.MobId, boss.Character.SpeciesId = 9562, 45
	boss.Groups, boss.Hates = []string{"warden"}, []string{"bandit"}
	pet := mobs.GetInstance(43)
	pet.MobId, pet.Character.SpeciesId = 9586, 45
	pet.Character.Charm(1, characters.CharmPermanent, ``)
	rival := mobs.GetInstance(44)
	rival.MobId, rival.Character.SpeciesId = 9999, 45
	rival.Groups = []string{"bandit"}
	rival.PlayerAttackImmune = false

	mobIds, _ := mobAreaHarmTargets(boss, room)
	slices.Sort(mobIds)
	if !slices.Equal(mobIds, []int{43, 44}) {
		t.Fatalf("charmed kin and hated kin are both hit: %v", mobIds)
	}
}

// The control: a charmed mob that is not bonded (and a bonded one while the
// module is off) is not gated, exactly as before.
func TestUnbondedAreaHarmIsUnchanged(t *testing.T) {
	caster, room := areaHarmWorld(t, configs.PVPDisabled)
	mobIds, userIds := mobAreaHarmTargets(caster, room)
	slices.Sort(mobIds)
	if !slices.Equal(mobIds, []int{43, 44}) || !slices.Equal(userIds, []int{2}) {
		t.Fatalf("an ordinary charmed caster spares only its owner: mobs %v users %v", mobIds, userIds)
	}
}
