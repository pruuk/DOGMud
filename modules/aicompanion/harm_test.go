package aicompanion

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// harmWorld is one lit room with her owner (user 1), a passer-by (user 2)
// and the companion herself (mob 42), PvP set as asked.
func harmWorld(t *testing.T, pvp string) (*users.UserRecord, *users.UserRecord, *rooms.Room, *mobs.Mob) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.GamePlay.PVP = configs.ConfigString(pvp)
	cfg.GamePlay.PVPMinimumSkillRanks = 0
	configs.SetConfigForTest(t, cfg)

	room := &rooms.Room{RoomId: 1, Zone: `HarmZone`, Title: `A Clearing`, Biome: `city`, Lamp: rooms.LampPtr(90)}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{1: room},
		map[string]*rooms.ZoneConfig{`HarmZone`: {Name: `HarmZone`, RoomId: 1, RoomIds: map[int]struct{}{1: {}}}}))

	owner := users.NewTestUser(1, `corvin`, `Corvin`, 0)
	other := users.NewTestUser(2, `bram`, `Bram`, 0)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{1: owner, 2: other}))
	room.AddPlayer(1)
	room.AddPlayer(2)

	her := harmMob(t, room, 42, `Mara`)
	return owner, other, room, her
}

func harmMob(t *testing.T, room *rooms.Room, id int, name string) *mobs.Mob {
	t.Helper()
	m := &mobs.Mob{InstanceId: id, Character: characters.Character{
		Name: name, RoomId: room.RoomId, Health: 100, Conditions: conditions.New(),
	}}
	m.Character.HealthMax.Value = 100
	mobs.SetInstanceForTest(id, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(id, nil) })
	room.AddMob(id)
	return m
}

// harmScene shows her the creature (t1) and the passer-by (t2).
func harmScene(mobId int, userId int) *scene {
	sc := &scene{RoomId: 1, byRef: map[string]*thing{}}
	sc.byRef[`t1`] = &thing{Ref: `t1`, Kind: `npc`, Name: `the caravan guard`, MobInstanceId: mobId}
	sc.byRef[`t2`] = &thing{Ref: `t2`, Kind: `player`, Name: `Bram`, UserId: userId}
	return sc
}

func TestHarmFollowsTheOwnersRules(t *testing.T) {
	owner, other, room, _ := harmWorld(t, configs.PVPDisabled)
	immune := harmMob(t, room, 300, `a caravan guard`)
	immune.PlayerAttackImmune = true
	quiet := harmMob(t, room, 301, `a quiet scribe`)
	quiet.NonCombatant = true
	wolf := harmMob(t, room, 302, `a grey wolf`)

	if ok, _ := harmAllowed(owner, room, immune.InstanceId, 0); ok {
		t.Fatal("her owner cannot attack a player_attack_immune creature, so she cannot either")
	}
	if ok, _ := harmAllowed(owner, room, quiet.InstanceId, 0); ok {
		t.Fatal("nor a non-combatant")
	}
	if ok, _ := harmAllowed(owner, room, wolf.InstanceId, 0); !ok {
		t.Fatal("a wolf is fair game for anyone")
	}
	if ok, _ := harmAllowed(owner, room, 0, other.UserId); ok {
		t.Fatal("with PvP off her owner could not fight Bram, so she will not")
	}
	if ok, _ := harmAllowed(owner, room, 0, owner.UserId); ok {
		t.Fatal("she never turns on her owner")
	}
	if ok, _ := harmAllowed(nil, room, wolf.InstanceId, 0); ok {
		t.Fatal("with nobody to answer for it, she starts nothing")
	}
}

func TestHarmRespectsPvpAndTheParty(t *testing.T) {
	owner, other, room, _ := harmWorld(t, configs.PVPEnabled)
	if ok, reason := harmAllowed(owner, room, 0, other.UserId); !ok {
		t.Fatalf("with PvP on, her owner could fight Bram: %s", reason)
	}
	p := parties.New(owner.UserId)
	p.UserIds = append(p.UserIds, other.UserId)
	t.Cleanup(p.Disband)
	if ok, _ := harmAllowed(owner, room, 0, other.UserId); ok {
		t.Fatal("but not someone in her owner's party")
	}
}

func TestAttackOnAProtectedCreatureIsRefused(t *testing.T) {
	owner, _, room, her := harmWorld(t, configs.PVPDisabled)
	immune := harmMob(t, room, 300, `a caravan guard`)
	immune.PlayerAttackImmune = true
	m, c, _ := strangerModule()

	out := m.performAction(c, her, owner, harmScene(immune.InstanceId, 2),
		ActionProposal{Verb: `attack`, Ref: `t1`}, []stimulus{{Kind: `heard`, FromOwner: true}}, 0, 0)
	if out.Issued || out.Refused == `` {
		t.Fatalf("her owner asking does not make a protected creature fair game: %+v", out)
	}

	immune.PlayerAttackImmune = false
	out = m.performAction(c, her, owner, harmScene(immune.InstanceId, 2),
		ActionProposal{Verb: `attack`, Ref: `t1`}, []stimulus{{Kind: `heard`, FromOwner: true}}, 0, 0)
	if !out.Issued {
		t.Fatalf("an unprotected one her owner points at is: %+v", out)
	}
}

func TestTheRefusalListIsStillHers(t *testing.T) {
	owner, _, room, her := harmWorld(t, configs.PVPDisabled)
	kid := harmMob(t, room, 300, `a frightened child`)
	m, c, _ := strangerModule()
	c.profile = &Profile{Name: `Mara`, Combat: CombatProfile{Refuse: []string{`child`}}}

	out := m.performAction(c, her, owner, harmScene(kid.InstanceId, 2),
		ActionProposal{Verb: `attack`, Ref: `t1`}, []stimulus{{Kind: `heard`, FromOwner: true}}, 0, 0)
	if out.Refused != `you will not raise a hand to them` {
		t.Fatalf("the engine would allow it, and she still will not: %+v", out)
	}
}

// harmSpells is one harmful and one helpful spell, both aimed at someone.
func harmSpells(t *testing.T, her *mobs.Mob) {
	t.Helper()
	t.Cleanup(spells.SeedSpellsForTest(map[string]*spells.SpellData{
		`burn`: {SpellId: `burn`, Name: `Burn`, EffectType: `damage`,
			AttackType: combatvocab.AttackSpell, DamageType: combatvocab.DamagePhysical, Targeting: combatvocab.TargetSingle},
		`mend`: {SpellId: `mend`, Name: `Mend`, EffectType: `heal`,
			AttackType: combatvocab.AttackNone, DamageType: combatvocab.DamageNonHarm, Targeting: combatvocab.TargetSingle},
	}))
	her.Character.SpellBook = map[string]int{`burn`: 1, `mend`: 1}
	her.Character.Conviction = 100
}

// spellRef is the ref she was shown for a spell.
func spellRef(t *testing.T, her *mobs.Mob, id string) string {
	t.Helper()
	for _, o := range spellsReady(her) {
		if o.Id == id {
			return o.Ref
		}
	}
	t.Fatalf("%s is not offered", id)
	return ``
}

func TestHarmfulCastsAreOwnerDrivenAndGated(t *testing.T) {
	owner, _, room, her := harmWorld(t, configs.PVPDisabled)
	harmSpells(t, her)
	wolf := harmMob(t, room, 302, `a grey wolf`)
	burn := spellRef(t, her, `burn`)
	m, c, _ := strangerModule()
	cast := func(to string, stims []stimulus) actionOutcome {
		return m.performAction(c, her, owner, harmScene(wolf.InstanceId, 2),
			ActionProposal{Verb: `cast`, Ref: burn, To: to}, stims, 0, 0)
	}

	if out := cast(`t1`, []stimulus{{Kind: `heard`, Speaker: `Bram`, AskerUserId: 2}}); out.Refused != `that is not a stranger's to ask for` {
		t.Fatalf("a stranger cannot have her burn anything: %+v", out)
	}
	if out := cast(`t1`, []stimulus{{Kind: `heard`, FromOwner: true}}); !out.Issued {
		t.Fatalf("her owner can, at a creature he could harm: %+v", out)
	}
	if out := cast(`t2`, []stimulus{{Kind: `heard`, FromOwner: true}}); out.Issued {
		t.Fatalf("but not at a person he could not fight: %+v", out)
	}
	if out := cast(`owner`, []stimulus{{Kind: `heard`, FromOwner: true}}); out.Issued {
		t.Fatalf("and never at him: %+v", out)
	}
	wolf.PlayerAttackImmune = true
	if out := cast(`t1`, []stimulus{{Kind: `heard`, FromOwner: true}}); out.Issued {
		t.Fatalf("nor at a creature players may not attack: %+v", out)
	}
}

func TestHelpfulCastsStayHerOwnJudgement(t *testing.T) {
	owner, _, _, her := harmWorld(t, configs.PVPDisabled)
	harmSpells(t, her)
	mend := spellRef(t, her, `mend`)
	m, c, _ := strangerModule()
	out := m.performAction(c, her, owner, harmScene(0, 2),
		ActionProposal{Verb: `cast`, Ref: mend, To: `owner`}, nil, 0, 0)
	if !out.Issued {
		t.Fatalf("mending her owner in a quiet moment needs nobody's word: %+v", out)
	}
}

// An area harm spell is filtered when it resolves
// (internal/hooks/mob_area_harm.go): whoever her owner could not harm is
// spared there. So starting one is refused only when it would land on
// nobody at all, or on someone she will not fight of her own accord.
func TestAreaHarmStartsWhenResolutionWouldLandIt(t *testing.T) {
	owner, _, room, her := harmWorld(t, configs.PVPDisabled)
	p := &Profile{}
	if ok, _ := areaHarmAllowed(owner, room, her, p); ok {
		t.Fatal("control: with only Bram, whom her owner could not fight, it lands on nobody")
	}
	harmMob(t, room, 302, `a grey wolf`)
	if ok, reason := areaHarmAllowed(owner, room, her, p); !ok {
		t.Fatalf("Bram is spared when it resolves, so a wolf is reason enough: %s", reason)
	}
	immune := harmMob(t, room, 300, `a caravan guard`)
	immune.PlayerAttackImmune = true
	if ok, reason := areaHarmAllowed(owner, room, her, p); !ok {
		t.Fatalf("a protected creature is spared when it resolves, too: %s", reason)
	}
}

// She will not set about anyone on her refusal list who is not already
// fighting, whoever asks, and an area spell would land on them: attack's
// rule holds for a cast and for the room.
func TestHarmfulCastsKeepHerRefusals(t *testing.T) {
	owner, _, room, her := harmWorld(t, configs.PVPDisabled)
	harmSpells(t, her)
	child := harmMob(t, room, 303, `a small child`)
	burn := spellRef(t, her, `burn`)
	m, c, _ := strangerModule()
	c.profile.Combat.Refuse = []string{`child`}
	owners := []stimulus{{Kind: `heard`, FromOwner: true}}

	out := m.performAction(c, her, owner, harmScene(child.InstanceId, 2),
		ActionProposal{Verb: `cast`, Ref: burn, To: `t1`}, owners, 0, 0)
	if out.Issued {
		t.Fatalf("she will not burn a child who is not fighting: %+v", out)
	}
	if ok, _ := areaHarmAllowed(owner, room, her, c.profile); ok {
		t.Fatal("nor loose an area spell that would land on one")
	}
	child.Character.SetAggro(0, her.InstanceId, characters.DefaultAttack)
	if ok, reason := areaHarmAllowed(owner, room, her, c.profile); !ok {
		t.Fatalf("one already fighting is fair, as attack has it: %s", reason)
	}
	out = m.performAction(c, her, owner, harmScene(child.InstanceId, 2),
		ActionProposal{Verb: `cast`, Ref: burn, To: `t1`}, owners, 0, 0)
	if !out.Issued {
		t.Fatalf("and may be cast at: %+v", out)
	}
}

func TestCombatTargetsFollowTheOwnersRules(t *testing.T) {
	owner, _, room, _ := harmWorld(t, configs.PVPDisabled)
	immune := harmMob(t, room, 300, `a caravan guard`)
	immune.PlayerAttackImmune = true
	wolf := harmMob(t, room, 302, `a grey wolf`)
	if mayStrike(owner, room, immune.InstanceId) {
		t.Fatal("the plan and her reflexes cannot turn her on a protected creature")
	}
	if !mayStrike(owner, room, wolf.InstanceId) {
		t.Fatal("but can on a wolf")
	}
	m, c, _ := strangerModule()
	c.fight = &fightState{Refs: map[string]int{`e1`: immune.InstanceId, `e2`: wolf.InstanceId}}
	m.applyCombatProposal(c, CombatProposal{Target: `e1`}, owner)
	if c.fight.TargetId != 0 {
		t.Fatal("the model choosing a protected creature is not taken")
	}
	m.applyCombatProposal(c, CombatProposal{Target: `e2`}, owner)
	if c.fight.TargetId != wolf.InstanceId {
		t.Fatal("choosing a wolf is")
	}
}

// A special move lands on whoever she is already fighting. A player in a
// fight may use one on their foe whoever it is (actions.StageMeleeTarget
// stages no target checks for a player in combat), so she may use one on a
// foe that is fighting HER, even one her owner could not have picked a fight
// with. A foe she is fighting that is not fighting her is still held to her
// owner's rules, and her owner never is a fair target.
func TestSpecialMoveAtAFoeAlreadyFightingHer(t *testing.T) {
	owner, _, room, her := harmWorld(t, configs.PVPDisabled)
	immune := harmMob(t, room, 300, `a caravan guard`)
	immune.PlayerAttackImmune = true

	her.Character.SetAggro(0, immune.InstanceId, characters.DefaultAttack)
	if mayStrikeCurrent(owner, room, her) {
		t.Fatal("control: a protected creature that is not fighting her is refused")
	}
	immune.Character.SetAggro(0, her.InstanceId, characters.DefaultAttack)
	if !mayStrikeCurrent(owner, room, her) {
		t.Fatal("a protected creature fighting her may be met with a move, as a player in a fight may")
	}

	owner.Character.SetAggro(0, her.InstanceId, characters.DefaultAttack)
	her.Character.SetAggro(owner.UserId, 0, characters.DefaultAttack)
	if mayStrikeCurrent(owner, room, her) {
		t.Fatal("she never turns a move on her owner, even one fighting her")
	}
}

// What she says is her owner's to answer for: a muted owner's companion
// says no authored battle line either, say or emote.
func TestBattleLinesRespectMute(t *testing.T) {
	owner, _, _, her := harmWorld(t, configs.PVPDisabled)
	m, c, _ := strangerModule()
	c.instanceId = her.InstanceId
	pool := []string{`"For the road!"`, `bares her teeth.`}
	spoke := func() int {
		n := 0
		for i := 0; i < 60; i++ {
			if m.combatLine(c, her, uint64(i*10), pool) {
				n++
			}
		}
		return n
	}
	if spoke() == 0 {
		t.Fatal("control: unmuted, she says some of her battle lines")
	}
	owner.Muted = true
	if n := spoke(); n != 0 {
		t.Fatalf("muted, she says none: %d", n)
	}
}

// A household's bauble is not hers to take (slice H, 14a): the get is
// refused before any command is issued, with a reason she is told, so she
// does not keep trying a pickup that would quietly fail.
func TestHouseholdBaubleIsRefusedUpFront(t *testing.T) {
	owner, _, room, her := harmWorld(t, configs.PVPDisabled)
	b := items.Item{ItemId: items.BaubleItemId, Bauble: `b0000001`, BaubleHousehold: room.RoomId}
	room.Items = append(room.Items, b)
	m, c, _ := strangerModule()
	sc := &scene{RoomId: room.RoomId, byRef: map[string]*thing{}}
	sc.byRef[`t1`] = &thing{Ref: `t1`, Kind: `item`, Name: `Trinket`, Item: b, HasItem: true}

	out := m.performAction(c, her, owner, sc, ActionProposal{Verb: `get`, Ref: `t1`},
		[]stimulus{{Kind: `heard`, FromOwner: true}}, 0, 0)
	if out.Issued || out.Refused != `it belongs to the household here` {
		t.Fatalf("refused up front, with the reason: %+v", out)
	}
}

// A fixture is part of the room (lighting 5e): the get is refused up front,
// as for a household's bauble, rather than issued to quietly fail.
func TestFixtureIsRefusedUpFront(t *testing.T) {
	const fixtureId = 999994
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		fixtureId: {ItemId: fixtureId, Name: `Arch Lantern`, NameSimple: `lantern`, Type: items.Object,
			Fixture: items.FixtureLight},
	}))
	owner, _, room, her := harmWorld(t, configs.PVPDisabled)
	f := items.New(fixtureId)
	room.Items = append(room.Items, f)
	m, c, _ := strangerModule()
	sc := &scene{RoomId: room.RoomId, byRef: map[string]*thing{}}
	sc.byRef[`t1`] = &thing{Ref: `t1`, Kind: `item`, Name: `Arch Lantern`, Item: f, HasItem: true}

	out := m.performAction(c, her, owner, sc, ActionProposal{Verb: `get`, Ref: `t1`},
		[]stimulus{{Kind: `heard`, FromOwner: true}}, 0, 0)
	if out.Issued || out.Refused != `it is fixed in place` {
		t.Fatalf("refused up front, with the reason: %+v", out)
	}
}
