package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
)

// #454: party auto-assist types `attack #<id>` for every member in the room.
// A member who sees nothing cannot pick the mob out, so assisting would only
// tell them "You don't see them here." every round. They are skipped; a member
// who can see still assists (the control).
func TestPartyAutoAssist_SkipsAMemberWhoSeesNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	p := parties.New(1)
	t.Cleanup(p.Disband)
	p.InvitePlayer(2)
	p.AcceptInvite(2)
	mob := mobs.GetInstance(100)
	defender := users.GetByUserId(1)

	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	events.DrainQueuedUserInputsForTest(2)
	handlePartyAutoAttack(mob, defender)
	assert.Equal(t, []string{"attack #100"}, events.DrainQueuedUserInputsForTest(2), "a member who can see assists")

	rooms.LoadRoom(1).Lamp = nil
	darken(t, 1)
	handlePartyAutoAttack(mob, defender)
	assert.Empty(t, events.DrainQueuedUserInputsForTest(2), "a member who sees nothing was sent to attack")
}

// companionScene makes mob 100 user 2's auto-assisting companion in room 1,
// lit or pitch dark.
func companionScene(t *testing.T, lit bool) *mobs.Mob {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	mob := mobs.GetInstance(100)
	mob.Character.IsMob = true
	mob.Character.Charm(2, -1, ``)
	owner := users.GetByUserId(2)
	owner.Character.Companions = []characters.CompanionInfo{{InstanceId: 100, Name: mob.Character.Name, MobId: int(mob.MobId), AutoAssist: true}}
	if lit {
		rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	} else {
		rooms.LoadRoom(1).Lamp = nil
		darken(t, 1)
	}
	events.DrainQueuedUserInputsForTest(2)
	return mob
}

// #454 review F3: a companion's owner is sent to `attack @<id>` when the
// companion is attacked. An owner who sees nothing would read "You don't see
// them here." every round; they are skipped as party members are.
func TestCompanionOwnerAssist_SkipsAnOwnerWhoSeesNothing(t *testing.T) {
	mob := companionScene(t, true)
	handleCompanionOwnerAssist(mob, "@1")
	assert.Equal(t, []string{"attack @1"}, events.DrainQueuedUserInputsForTest(2), "an owner who can see assists")

	mob = companionScene(t, false)
	handleCompanionOwnerAssist(mob, "@1")
	assert.Empty(t, events.DrainQueuedUserInputsForTest(2), "an owner who sees nothing was sent to attack")
}

// The reactive path in CombatPhase_CompanionAssist.go makes the same call.
func TestCompanionAssistReactive_SkipsAnOwnerWhoSeesNothing(t *testing.T) {
	mob := companionScene(t, true)
	wireCompanionAssist(&mob.Character)
	mob.Character.CombatPhase.RecordInboundAttacker(state.ActorRef{UserId: 1})
	assert.Contains(t, events.DrainQueuedUserInputsForTest(2), "attack @1", "an owner who can see assists")

	mob = companionScene(t, false)
	wireCompanionAssist(&mob.Character)
	mob.Character.CombatPhase.RecordInboundAttacker(state.ActorRef{UserId: 1})
	assert.Empty(t, events.DrainQueuedUserInputsForTest(2), "an owner who sees nothing was sent to attack")
}
