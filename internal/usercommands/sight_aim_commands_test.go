package usercommands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/pets"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #454: a typed name resolves only at full sight. The scene is
// seedAllRegistries' room 1: Aliceia (1, the actor), Bobrick (2) and the
// Skeleton (mob 100). Aliceia's figures at shapes are Bobrick (shape 1) and
// the Skeleton (shape 2).

const aimInfraredConditionId = 9454

type aimBand int

const (
	aimFull aimBand = iota
	aimShapes
	aimDark
)

// aimScene seeds the registries and sets room 1 to the band asked for:
// lit (lamp 60), or pitch dark with Aliceia given infrared (shapes) or not.
func aimScene(t *testing.T, band aimBand) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		aimInfraredConditionId: {ConditionId: aimInfraredConditionId, Name: "Test Infrared",
			RoundInterval: 1, TriggerCount: 1, Flags: []conditions.Flag{conditions.InfraredVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}}},
	}))
	user := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	if band != aimFull {
		room.Lamp = nil
		room.Biome = "cave"
		require.Equal(t, 0, room.LightLevel(), "the dark bands need a pitch-dark room")
	}
	if band == aimShapes {
		require.True(t, user.Character.Conditions.AddCondition(aimInfraredConditionId, true))
	}
	events.DrainQueuedMessagesForTest(1)
	events.DrainQueuedMessagesForTest(2)
	return user, room
}

var aimTagPattern = regexp.MustCompile(`<[^>]*>`)

// aimTold is everything userId was sent since the last drain, tags stripped.
func aimTold(userId int) string {
	return aimTagPattern.ReplaceAllString(strings.Join(events.DrainQueuedMessagesForTest(userId), ""), "")
}

func TestAttackSight_ClearSightResolvesAName(t *testing.T) {
	user, room := aimScene(t, aimFull)
	_, err := Attack("skeleton", user, room, events.EventFlag(0))
	require.NoError(t, err)
	assert.Equal(t, 100, user.Character.CurrentCombatTarget().MobInstanceId)
}

func TestAttackSight_ShapesHintsANameAndTakesAShape(t *testing.T) {
	user, room := aimScene(t, aimShapes)
	_, _ = Attack("skeleton", user, room, events.EventFlag(0))
	told := aimTold(1)
	assert.Contains(t, told, "You can only make out shapes here.")
	assert.Contains(t, told, "attack shape")
	assert.Equal(t, 0, user.Character.CurrentCombatTarget().MobInstanceId, "a typed name engaged at shapes")

	_, _ = Attack("2.shape", user, room, events.EventFlag(0))
	assert.Equal(t, 100, user.Character.CurrentCombatTarget().MobInstanceId, "shape 2 is the Skeleton")
	assert.NotContains(t, aimTold(1), "Skeleton", "the attacker who aimed at a shape read its name")
}

func TestAttackSight_NoSightResolvesNothing(t *testing.T) {
	for _, rest := range []string{"skeleton", "shape", "*"} {
		user, room := aimScene(t, aimDark)
		_, _ = Attack(rest, user, room, events.EventFlag(0))
		assert.Equal(t, 0, user.Character.CurrentCombatTarget().MobInstanceId, "%q engaged with no sight", rest)
		told := aimTold(1)
		if rest == "skeleton" {
			assert.Contains(t, told, actions.AimNotHereLine)
		} else {
			assert.Contains(t, told, actions.AimNothingLine, "%q", rest)
		}
	}
}

// Every melee special stages its target through StageMeleeTarget.
func TestMeleeStageSight_KickAtEachBand(t *testing.T) {
	user, room := aimScene(t, aimFull)
	_, handled := actions.StageMeleeTarget(user, room, "skeleton", actions.MeleeTargetOpts{Verb: "kick"})
	assert.False(t, handled, "clear sight stages a named target")

	user, room = aimScene(t, aimShapes)
	_, handled = actions.StageMeleeTarget(user, room, "skeleton", actions.MeleeTargetOpts{Verb: "kick"})
	assert.True(t, handled)
	assert.Contains(t, aimTold(1), "kick 2.shape")
	_, handled = actions.StageMeleeTarget(user, room, "2.shape", actions.MeleeTargetOpts{Verb: "kick"})
	assert.False(t, handled, "a shape stages at shapes")

	user, room = aimScene(t, aimDark)
	_, handled = actions.StageMeleeTarget(user, room, "skeleton", actions.MeleeTargetOpts{Verb: "kick"})
	assert.True(t, handled, "no sight stages nothing")
	assert.Contains(t, aimTold(1), actions.AimNotHereLine)
}

// target is attack's sibling: attack hands a target switch to it.
func TestTargetSight_NoSightResolvesNothing(t *testing.T) {
	user, room := aimScene(t, aimDark)
	require.True(t, targeting.Commit(user.Character, state.ActorRef{MobInstanceId: 100}, targeting.ReasonAttack))
	_, _ = Target("bobrick", user, room, events.EventFlag(0))
	assert.Contains(t, aimTold(1), actions.AimNotHereLine)
	assert.Equal(t, 100, user.Character.CurrentCombatTarget().MobInstanceId, "the target did not change")
}

func TestGiveSight_ClearSightNamesBothSides(t *testing.T) {
	user, room := aimScene(t, aimFull)
	user.Character.Gold = 50
	_, _ = Give("10 gold bobrick", user, room, events.EventFlag(0))
	assert.Equal(t, 40, user.Character.Gold)
	assert.Contains(t, aimTold(1), "to Bobrick.")
	assert.Contains(t, aimTold(2), "Aliceia gives you")
}

func TestGiveSight_ShapesHintsANameAndHidesNamesAfterAShape(t *testing.T) {
	user, room := aimScene(t, aimShapes)
	user.Character.Gold = 50
	_, _ = Give("10 gold bobrick", user, room, events.EventFlag(0))
	told := aimTold(1)
	assert.Contains(t, told, "give 10 gold shape")
	assert.Equal(t, 50, user.Character.Gold, "a typed name was given to at shapes")

	_, _ = Give("10 gold 1.shape", user, room, events.EventFlag(0))
	assert.Equal(t, 40, user.Character.Gold, "shape 1 is Bobrick")
	told = aimTold(1)
	assert.Contains(t, told, "to a figure.")
	assert.NotContains(t, told, "Bobrick", "the giver aimed at a shape and read its name")
	// Bobrick has no infrared: he sees nothing, and reads "Something".
	recipient := aimTold(2)
	assert.Contains(t, recipient, "Something gives you")
	assert.NotContains(t, recipient, "Aliceia", "a recipient who sees nothing read the giver's name")
}

func TestGiveSight_NoSightGivesNothing(t *testing.T) {
	user, room := aimScene(t, aimDark)
	user.Character.Gold = 50
	_, _ = Give("10 gold bobrick", user, room, events.EventFlag(0))
	assert.Equal(t, 50, user.Character.Gold)
	assert.Contains(t, aimTold(1), actions.AimNotHereLine)
}

func TestShowSight_ShapesHidesTheNameAfterAShape(t *testing.T) {
	user, room := aimScene(t, aimShapes)
	user.Character.StoreItem(items.New(10001))
	_, _ = Show("sword 2.shape", user, room, events.EventFlag(0))
	told := aimTold(1)
	assert.Contains(t, told, "to a figure.")
	assert.NotContains(t, told, "Skeleton")
}

func TestStealSight_EachBand(t *testing.T) {
	user, room := aimScene(t, aimFull)
	opts := parseStealArgs([]string{"skeleton"}, room, user)
	require.NotNil(t, opts)
	assert.Equal(t, 100, opts.TargetMobInstanceId)

	user, room = aimScene(t, aimShapes)
	assert.Nil(t, parseStealArgs([]string{"skeleton"}, room, user))
	assert.Contains(t, aimTold(1), "steal 2.shape")
	opts = parseStealArgs([]string{"2.shape"}, room, user)
	require.NotNil(t, opts, "a shape is stolen from at shapes")
	assert.Equal(t, 100, opts.TargetMobInstanceId)

	user, room = aimScene(t, aimDark)
	assert.Nil(t, parseStealArgs([]string{"skeleton"}, room, user))
	assert.Contains(t, aimTold(1), actions.AimNotHereLine)
}

func TestConsiderSight_ShapesConsidersAFigure(t *testing.T) {
	user, room := aimScene(t, aimShapes)
	_, _ = Consider("2.shape", user, room, events.EventFlag(0))
	told := aimTold(1)
	assert.Contains(t, told, "You consider a figure")
	assert.NotContains(t, told, "Skeleton")
}

// Owner call 2: talk, ask, party invite and rep find their target among the
// room's occupants, so a typed name in the dark would confirm who is there.
func TestTalkAskSight_NoSightResolvesNothing(t *testing.T) {
	user, room := aimScene(t, aimDark)
	_, _ = Talk("skeleton", user, room, events.EventFlag(0))
	assert.Contains(t, aimTold(1), actions.AimNotHereLine)
	_, _ = Ask("skeleton quest", user, room, events.EventFlag(0))
	assert.Contains(t, aimTold(1), actions.AimNotHereLine)
}

func TestTalkSight_ShapesHintsAName(t *testing.T) {
	user, room := aimScene(t, aimShapes)
	_, _ = Talk("skeleton", user, room, events.EventFlag(0))
	assert.Contains(t, aimTold(1), "talk 2.shape")
}

func TestPartyInviteSight_EachBand(t *testing.T) {
	user, room := aimScene(t, aimDark)
	t.Cleanup(func() {
		if p := parties.Get(1); p != nil {
			p.Disband()
		}
	})
	_, _ = Party("invite bobrick", user, room, events.EventFlag(0))
	assert.Contains(t, aimTold(1), actions.AimNotHereLine)
	assert.Empty(t, aimTold(2), "a player nobody could see was invited")

	user, room = aimScene(t, aimShapes)
	_, _ = Party("invite 1.shape", user, room, events.EventFlag(0))
	told := aimTold(1)
	assert.Contains(t, told, "You invited a figure to your party.")
	assert.NotContains(t, told, "Bobrick")
	invitee := aimTold(2)
	assert.Contains(t, invitee, "Someone invited you", "Bobrick sees nothing, so his inviter is someone")
}

func TestRepSight_EachBand(t *testing.T) {
	user, room := aimScene(t, aimDark)
	_, _ = Report("bobrick", user, room, events.EventFlag(0))
	assert.Contains(t, aimTold(1), actions.AimNotHereLine)
	assert.Empty(t, aimTold(2), "a whisper reached a player nobody could see")

	user, room = aimScene(t, aimShapes)
	_, _ = Report("1.shape", user, room, events.EventFlag(0))
	told := aimTold(1)
	assert.Contains(t, told, "You report to a figure:")
	assert.NotContains(t, told, "Bobrick")
	assert.Contains(t, aimTold(2), "Someone reports to you:")
}

// An aimed shot needs full sight of the room it is aimed into, judged before
// any name resolves. Resolving first answered "Bobrick is in your party!" to a
// shooter in pitch dark, which told them Bobrick was there.
func TestFireSight_NoSightRefusesBeforeANameResolves(t *testing.T) {
	user, room := aimScene(t, aimDark)
	p := parties.New(1)
	t.Cleanup(p.Disband)
	p.InvitePlayer(2)
	p.AcceptInvite(2)
	for _, rest := range []string{"bobrick", "nobody"} {
		_, _ = Fire(rest, user, room, events.EventFlag(0))
		told := aimTold(1)
		assert.Contains(t, told, "It is too dark to aim.", "%q", rest)
		assert.NotContains(t, told, "Bobrick", "%q", rest)
	}
}

// #454 review F7: a shot through an exit that finds no one beyond retries the
// whole phrase in the shooter's own room. ShotSight judged only the room
// beyond, so a shooter who made out only shapes at home had a party member
// whose name the phrase began read "is in your party!" where an absent one
// read the next refusal. Reachable when LightExitsAbove sits below
// LightDimBelow, which the config allows (it is checked against
// LightBlindBelow only); the shipped 55 over 50 hides it.
func TestFireSight_OwnRoomFallbackNeedsSightOfTheOwnRoom(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Balance.LightBlindBelow = 25
	cfg.Balance.LightDimBelow = 50
	cfg.Balance.LightExitsAbove = 30
	configs.SetConfigForTest(t, cfg)
	told := map[bool]string{}
	for _, present := range []bool{true, false} {
		user, room := aimScene(t, aimFull)
		room.Biome, room.Lamp = "cave", rooms.LampPtr(40)
		require.Equal(t, messaging.SightShapes, messaging.ParticipantSight(user.Character, room), "the shooter's own room must be at shapes")
		rooms.LoadRoom(2).Biome, rooms.LoadRoom(2).Lamp = "cave", rooms.LampPtr(60)
		require.Equal(t, messaging.SightFull, actions.ShotSight(user.Character, room, "bob north"), "the room beyond must be in full sight")
		users.GetByUserId(2).Character.Name = "Bob Northgate"
		p := parties.New(1)
		p.InvitePlayer(2)
		p.AcceptInvite(2)
		if !present {
			room.RemovePlayer(2)
		}
		_, _ = Fire("bob north", user, room, events.EventFlag(0))
		p.Disband()
		told[present] = aimTold(1)
		assert.NotContains(t, told[present], "Northgate", "present=%v", present)
	}
	assert.Equal(t, told[true], told[false], "the shot told a present party member from an absent one")
}

// The lit control: in a lit own room the full phrase finds the member.
func TestFireSight_LitOwnRoomFallbackFindsTheName(t *testing.T) {
	user, room := aimScene(t, aimFull)
	room.Lamp = rooms.LampPtr(90)
	rooms.LoadRoom(2).Lamp = rooms.LampPtr(90)
	require.Equal(t, messaging.SightFull, actions.ShotSight(user.Character, room, "bob north"), "the room beyond must be in full sight")
	users.GetByUserId(2).Character.Name = "Bob Northgate"
	p := parties.New(1)
	t.Cleanup(p.Disband)
	p.InvitePlayer(2)
	p.AcceptInvite(2)
	_, _ = Fire("bob north", user, room, events.EventFlag(0))
	assert.Contains(t, aimTold(1), "Bob Northgate is in your party!")
}

// A shot through an exit is judged by what the shooter makes out of the room
// beyond (scanReach), not of their own lit room.
func TestShotSight_ThroughAnExitReadsTheRoomBeyond(t *testing.T) {
	user, room := aimScene(t, aimFull)
	rooms.LoadRoom(2).Biome = "cave"
	require.Equal(t, 0, rooms.LoadRoom(2).LightLevel())
	assert.Equal(t, messaging.SightFull, actions.ShotSight(user.Character, room, "skeleton"))
	assert.Equal(t, messaging.SightNone, actions.ShotSight(user.Character, room, "skeleton north"))
}

// #454 review F1: give's split between item and recipient must not ask the
// room who is there below full sight. A two-word recipient that is present
// and one that is absent read the same refusal in the dark.
func TestGiveSight_DarkSplitReadsTheSameWhetherPresentOrNot(t *testing.T) {
	told := map[bool]string{}
	for _, present := range []bool{true, false} {
		user, room := aimScene(t, aimDark)
		mobs.GetInstance(100).Character.Name = "Smith Rusk"
		if !present {
			room.RemoveMob(100)
		}
		user.Character.StoreItem(items.New(10001))
		_, _ = Give("sword smith rusk", user, room, events.EventFlag(0))
		told[present] = aimTold(1)
		_, has := user.Character.FindInBackpack("sword")
		assert.True(t, has, "present=%v: the sword left the giver in the dark", present)
	}
	assert.Equal(t, told[true], told[false], "the refusal told a present recipient from an absent one")
	assert.Contains(t, told[true], actions.AimNotHereLine)
}

// The lit control: at full sight the two-word recipient resolves.
func TestGiveSight_LitSplitTakesATwoWordRecipient(t *testing.T) {
	user, room := aimScene(t, aimFull)
	mobs.GetInstance(100).Character.Name = "Smith Rusk"
	user.Character.StoreItem(items.New(10001))
	_, _ = Give("sword smith rusk", user, room, events.EventFlag(0))
	_, has := user.Character.FindInBackpack("sword")
	assert.False(t, has, "a lit giver could not hand a sword to Smith Rusk")
	assert.Contains(t, aimTold(1), "to Smith Rusk.")
}

// #454 review F5: `give X pet` is the giver's own pet, even with a player
// whose name starts "pet" standing there.
func TestGiveSight_PetWordIsTheOwnPetBeforeAPlayer(t *testing.T) {
	user, room := aimScene(t, aimFull)
	users.GetByUserId(2).Character.Name = "Petra"
	user.Character.Pet = pets.Pet{Name: "Fang", Type: "dog", Capacity: 5}
	user.Character.StoreItem(items.New(10001))
	_, _ = Give("sword pet", user, room, events.EventFlag(0))
	assert.Len(t, user.Character.Pet.Items, 1, "the sword did not reach the giver's pet")
	_, petraHas := users.GetByUserId(2).Character.FindInBackpack("sword")
	assert.False(t, petraHas, "`give sword pet` went to Petra")
}

// #454 review F6: `pet <name>` and `get x from <name>` find another player's
// pet by name. Below full sight a pet that is there and one that is not read
// the same line. Lit control: the pet is found.
func TestPetSight_OthersPetReadsTheSameWhetherPresentOrNot(t *testing.T) {
	for _, band := range []aimBand{aimShapes, aimDark} {
		told := map[bool][2]string{}
		for _, present := range []bool{true, false} {
			user, room := aimScene(t, band)
			if present {
				users.GetByUserId(2).Character.Pet = pets.Pet{Name: "Rex", Type: "dog", Capacity: 5}
			}
			_, _ = Pet("rex", user, room, events.EventFlag(0))
			petTold := aimTold(1)
			_, _ = Get("sword from rex", user, room, events.EventFlag(0))
			told[present] = [2]string{petTold, aimTold(1)}
		}
		assert.Equal(t, told[true][0], told[false][0], "band %d: `pet rex` told a present pet from an absent one", band)
		assert.Equal(t, told[true][1], told[false][1], "band %d: `get sword from rex` told a present pet from an absent one", band)
	}
}

func TestPetSight_LitFindsOthersPet(t *testing.T) {
	user, room := aimScene(t, aimFull)
	users.GetByUserId(2).Character.Pet = pets.Pet{Name: "Rex", Type: "dog", Capacity: 5}
	_, _ = Pet("rex", user, room, events.EventFlag(0))
	assert.Contains(t, aimTold(1), "You pet Rex")
	_, _ = Get("sword from rex", user, room, events.EventFlag(0))
	assert.Contains(t, aimTold(1), "You can't do that!")
}

// The own pet is at hand in the dark too.
func TestPetSight_OwnPetInTheDark(t *testing.T) {
	user, room := aimScene(t, aimDark)
	user.Character.Pet = pets.Pet{Name: "Fang", Type: "dog", Capacity: 5}
	_, _ = Pet("fang", user, room, events.EventFlag(0))
	assert.Contains(t, aimTold(1), "You pet Fang")
}

// #454 review F4: the sleep refusal named a sleeper the asker aimed at as a
// shape. Lit control: at full sight it names them.
func TestAskSight_SleepRefusalHidesAShapesName(t *testing.T) {
	const sleepId = 9455
	for _, band := range []aimBand{aimFull, aimShapes} {
		user, room := aimScene(t, band)
		t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
			sleepId: {ConditionId: sleepId, Name: "Test Sleep", RoundInterval: 1, TriggerCount: 100,
				Flags: []conditions.Flag{conditions.Sleeping}},
			// Reseeding replaces the registry: keep aimScene's infrared.
			aimInfraredConditionId: {ConditionId: aimInfraredConditionId, Name: "Test Infrared",
				RoundInterval: 1, TriggerCount: 1, Flags: []conditions.Flag{conditions.InfraredVision},
				Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}}},
		}))
		require.NoError(t, mobs.GetInstance(100).Character.AddCondition(sleepId, true))
		aim := "skeleton quest"
		if band == aimShapes {
			aim = "2.shape quest"
		}
		_, _ = Ask(aim, user, room, events.EventFlag(0))
		told := aimTold(1)
		assert.Contains(t, told, "is fast asleep.", "band %d", band)
		if band == aimFull {
			assert.Contains(t, told, "Skeleton is fast asleep.")
		} else {
			assert.NotContains(t, told, "Skeleton", "an asker who aimed at a shape read its name")
			assert.Contains(t, told, "A figure is fast asleep.")
		}
	}
}

// #454 review F2: share pays the party members in the room directly. It typed
// `give N gold to @uid` per member, which the sight gate refused in the dark.
// Party membership is known, so paying a member tells no one who is there;
// each line hides the other's name at its reader's sight.
func TestShareSight_EachBandPaysTheParty(t *testing.T) {
	for _, band := range []aimBand{aimFull, aimDark} {
		user, room := aimScene(t, band)
		p := parties.New(1)
		p.InvitePlayer(2)
		p.AcceptInvite(2)
		user.Character.Gold = 50
		bob := users.GetByUserId(2)
		bob.Character.Gold = 0
		_, _ = Share("10 gold", user, room, events.EventFlag(0))
		p.Disband()
		assert.Equal(t, 45, user.Character.Gold, "band %d: the sharer kept Bobrick's share", band)
		assert.Equal(t, 5, bob.Character.Gold, "band %d: Bobrick got no share", band)
		told, bobTold := aimTold(1), aimTold(2)
		assert.NotContains(t, told, actions.AimNotHereLine, "band %d", band)
		if band == aimFull {
			assert.Contains(t, told, "to Bobrick.")
			assert.Contains(t, bobTold, "Aliceia gives you")
		} else {
			assert.NotContains(t, told, "Bobrick", "a sharer in the dark read the name")
			assert.NotContains(t, bobTold, "Aliceia", "a member in the dark read the sharer's name")
			assert.Contains(t, bobTold, "gives you")
		}
	}
}

// The own pet by its own name is at hand at shapes: no hint.
func TestGiveSight_OwnPetByNameAtShapes(t *testing.T) {
	user, room := aimScene(t, aimShapes)
	user.Character.Pet = pets.Pet{Name: "Fang", Type: "dog", Capacity: 5}
	user.Character.StoreItem(items.New(10001))
	_, _ = Give("sword fang", user, room, events.EventFlag(0))
	assert.Len(t, user.Character.Pet.Items, 1, "the own pet by name was refused at shapes")
	assert.NotContains(t, aimTold(1), "make out shapes")
}
