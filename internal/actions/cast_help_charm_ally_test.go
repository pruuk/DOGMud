package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Slice 3b playtest (run b0ac6af57a0c9004, room 472): a healer partied with a
// thug's charmer got `You don't see "thug" here.` for `cast heal thug`, while
// the same healer's area heal reached the thug. Single-target help allowed
// only the caster's own pet. Owner ruling 2026-09-28: helpful spells reach
// mobs charmed by the caster or by any member of the caster's party, and
// single-target and area help decide that with one rule (HelpCharmAlly).
// ---------------------------------------------------------------------------

// helpPartyForCastTest makes user 1 lead a party user 2 has joined; user 3
// stays outside it.
func helpPartyForCastTest(t *testing.T) {
	t.Helper()
	p := parties.New(1)
	require.NotNil(t, p)
	require.True(t, p.InvitePlayer(2))
	require.True(t, p.AcceptInvite(2))
	t.Cleanup(p.Disband)
}

func seedHelpSingleSpell(t *testing.T, id string) {
	t.Helper()
	_, cleanup := seedTestSpell(id, combatvocab.NonHarm(combatvocab.TargetSingle), 4)
	t.Cleanup(cleanup)
}

func charmedTo(userId int) func(*mobs.Mob) {
	return func(m *mobs.Mob) { m.Character.Charm(userId, -1, "") }
}

func TestInitiateCast_HelpSingle_PlayerHealsAPartyMembersPet(t *testing.T) {
	seedHelpSingleSpell(t, "help-party-pet")
	helpPartyForCastTest(t)
	actor, _, room := newPlayerActor()
	defer seedRoomMob(t, room, 9101, "Thornwall Thug", charmedTo(2))()

	result := InitiateCast(actor, "help-party-pet", "thug")

	require.True(t, result.Initiated, "a party member's companion is a help target")
	assert.Equal(t, []int{9101}, result.TargetMobInstanceIds)
}

func TestInitiateCast_HelpSingle_PlayerCannotHealAStrangersPet(t *testing.T) {
	seedHelpSingleSpell(t, "help-stranger-pet")
	helpPartyForCastTest(t)
	actor, _, room := newPlayerActor()
	defer seedRoomMob(t, room, 9102, "Thornwall Thug", charmedTo(3))()

	result := InitiateCast(actor, "help-stranger-pet", "thug")

	assert.True(t, result.NoTarget, "a stranger's pet is not on the caster's side")
	assert.False(t, result.Initiated)
	assert.Empty(t, result.TargetMobInstanceIds)
}

func TestInitiateCast_HelpSingle_PlayerHealsTheirOwnPet(t *testing.T) {
	seedHelpSingleSpell(t, "help-own-pet")
	actor, _, room := newPlayerActor()
	defer seedRoomMob(t, room, 9103, "Thornwall Thug", charmedTo(1))()

	result := InitiateCast(actor, "help-own-pet", "thug")

	require.True(t, result.Initiated, "the caster's own pet is a help target")
	assert.Equal(t, []int{9103}, result.TargetMobInstanceIds)
}

func TestInitiateCast_HelpSingle_PlayerCannotHealAWildMob(t *testing.T) {
	seedHelpSingleSpell(t, "help-wild-mob")
	actor, _, room := newPlayerActor()
	defer seedRoomMob(t, room, 9104, "Thornwall Thug", nil)()

	result := InitiateCast(actor, "help-wild-mob", "thug")

	assert.True(t, result.NoTarget, "an uncharmed mob is no player's ally")
}

// mobCasterForHelpTest seeds a caster mob (instance 9110) in a lit room.
func mobCasterForHelpTest(t *testing.T, mutate func(*mobs.Mob)) (*MobActor, *rooms.Room) {
	t.Helper()
	room := newTestRoom()
	t.Cleanup(seedRoomMob(t, room, 9110, "Hedge Witch", mutate))
	return &MobActor{Mob: mobs.GetInstance(9110), Room: room}, room
}

// A charmed mob stands on its owner's side, as its area help does.
func TestInitiateCast_HelpSingle_CharmedMobFollowsItsOwnersSide(t *testing.T) {
	seedHelpSingleSpell(t, "help-charmed-caster")
	helpPartyForCastTest(t)
	actor, room := mobCasterForHelpTest(t, charmedTo(1))
	defer seedRoomMob(t, room, 9111, "Party Hound", charmedTo(2))()
	defer seedRoomMob(t, room, 9112, "Stray Hound", charmedTo(3))()

	ok := InitiateCast(actor, "help-charmed-caster", "party")
	require.True(t, ok.Initiated, "the owner's party member's pet is a help target")
	assert.Equal(t, []int{9111}, ok.TargetMobInstanceIds)

	refused := InitiateCast(actor, "help-charmed-caster", "stray")
	assert.True(t, refused.NoTarget, "a stranger's pet is not on the owner's side")
	assert.Empty(t, refused.TargetMobInstanceIds)

	seedHelpPlayerInRoom(t, room)
	actor.Mob.Character.Cooldowns = nil // the first cast took the special-move slot
	player := InitiateCast(actor, "help-charmed-caster", "Wanderer")
	require.True(t, player.Initiated, "a charmed mob's side takes in every player")
	assert.Equal(t, []int{helpPlayerUserId}, player.TargetUserIds)
}

const helpPlayerUserId = 3

// seedHelpPlayerInRoom puts the stranger, user 3 ("Wanderer"), in the room.
func seedHelpPlayerInRoom(t *testing.T, room *rooms.Room) {
	t.Helper()
	u := seedDrainAreaPlayer(helpPlayerUserId, "wanderer", "Wanderer")
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{helpPlayerUserId: u}))
	room.AddPlayer(helpPlayerUserId)
	t.Cleanup(func() { room.RemovePlayer(helpPlayerUserId) })
}

// An uncharmed mob helps mobs on no player's side (its packmates, and a boss
// add's named boss: the Repair Frame heals Warden-Prime by name), never a
// player's pet.
func TestInitiateCast_HelpSingle_WildMobHelpsWildMobsNotPets(t *testing.T) {
	seedHelpSingleSpell(t, "help-wild-caster")
	actor, room := mobCasterForHelpTest(t, nil)
	defer seedRoomMob(t, room, 9113, "Warden-Prime", nil)()
	defer seedRoomMob(t, room, 9114, "Pet Hound", charmedTo(1))()

	ok := InitiateCast(actor, "help-wild-caster", "Warden-Prime")
	require.True(t, ok.Initiated, "a wild mob may help another wild mob")
	assert.Equal(t, []int{9113}, ok.TargetMobInstanceIds)

	refused := InitiateCast(actor, "help-wild-caster", "hound")
	assert.True(t, refused.NoTarget, "a player's pet is not a wild mob's ally")
	assert.Empty(t, refused.TargetMobInstanceIds)

	seedHelpPlayerInRoom(t, room)
	actor.Mob.Character.Cooldowns = nil // the first cast took the special-move slot
	player := InitiateCast(actor, "help-wild-caster", "Wanderer")
	assert.True(t, player.NoTarget, "no player is a wild mob's ally")
	assert.Empty(t, player.TargetUserIds)
}

// #480 playtest: "Repair Frame crackles with hard blue-white arc-light,
// current arcing toward ." The Repair Frame's tree casts repair-pulse at a
// named boss; when the boss had died before the delayed cast ran, the name
// matched no one, the mob branch fell through with no target at all, and the
// cast began anyway, rendering {actee} empty. A named target that is not
// here is NoTarget, as the player branch answers.
func TestInitiateCast_HelpSingle_MobNamedTargetGoneIsNoTarget(t *testing.T) {
	seedHelpSingleSpell(t, "help-gone-target")
	actor, _ := mobCasterForHelpTest(t, nil)

	res := InitiateCast(actor, "help-gone-target", "The Core Guardian")
	assert.True(t, res.NoTarget, "a named help target who is not here is NoTarget")
	assert.False(t, res.Initiated, "and no cast begins")
	assert.Empty(t, res.TargetMobInstanceIds)
	assert.Empty(t, res.TargetUserIds)
}
