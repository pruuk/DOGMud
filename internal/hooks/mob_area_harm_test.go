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
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// areaHarmWorld is one room holding Corvin (user 1), Bram (user 2), a mob
// charmed by Corvin (the caster, 42), an ordinary creature (43) and one
// players may not attack (44), with PvP as given.
func areaHarmWorld(t *testing.T, pvp string) (*mobs.Mob, *rooms.Room) {
	t.Helper()
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

// A spell that spares allies (SpellData.SparesAllies) misses its caster's own
// side (a mob sharing one of its groups, a boss's adds) unless that mob is
// fighting it, and still strikes every other creature and every player.
// Without the flag, nothing is spared.
func TestAllySparingAreaHarmSparesItsOwnSide(t *testing.T) {
	caster, room := areaHarmWorld(t, configs.PVPDisabled)
	caster.Character.RemoveCharm()
	caster.Groups = []string{`obelisk`}
	ally := mobs.GetInstance(43)
	ally.Groups = []string{`obelisk`}

	if mobIds, _ := mobAreaHarmTargets(caster, room); !slices.Contains(mobIds, 43) {
		t.Fatalf("without the flag an ally is struck: %v", mobIds)
	}
	mobIds, userIds := mobAreaHarmTargetsSparing(caster, room, true)
	slices.Sort(mobIds)
	if !slices.Equal(mobIds, []int{44}) || !slices.Equal(userIds, []int{1, 2}) {
		t.Fatalf("spares its own kind only: mobs %v users %v", mobIds, userIds)
	}

	// An ally turned on it is fair game.
	ally.Character.SetAggro(0, caster.InstanceId, characters.DefaultAttack)
	mobIds, _ = mobAreaHarmTargetsSparing(caster, room, true)
	if !slices.Contains(mobIds, 43) {
		t.Fatalf("a packmate fighting the caster is struck: %v", mobIds)
	}
}

// A wild caster's area harm does not reach a player it has not detected.
func TestWildAreaHarmMissesAnUndetectedSneaker(t *testing.T) {
	caster, room := areaHarmWorld(t, configs.PVPDisabled)
	caster.Character.RemoveCharm()
	bram := users.GetByUserId(2)
	reason := state.TransitionReason{Trigger: `area_test`}
	if err := bram.Character.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason); err != nil {
		t.Fatal(err)
	}
	bram.Character.Awareness.ResolveConcealment(true, reason)

	_, userIds := mobAreaHarmTargets(caster, room)
	if !slices.Equal(userIds, []int{1}) {
		t.Fatalf("only the player it can know of: %v", userIds)
	}
}
