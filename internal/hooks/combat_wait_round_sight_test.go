package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// File: combat_wait_round_sight_test.go
//
// handleCombatWaitRound drains the authored wait-round {source}/{target}
// lines to the participants with plain SendText and no sight step, unlike
// the swing path (dispatchCritAndMessaging), which calls
// replaceDarknessMessages first. This pins the fix: the defender must not
// read the attacking mob's name while they cannot see it, and must read it
// normally once they can.

// waitTodefenderLine is the authored shape from
// _datafiles/world/dogmud/combat-messages/generic.yaml:104 under `todefender`.
const waitTodefenderLine = `<ansi fg="{sourcetype}">{source}</ansi> watches you intently, waiting to strike.`

// seedWaitMessageFixture installs a minimal attackMessages registry whose
// Generic/Wait/Together.ToDefender entry is the observed authored line, and
// returns the restore func. seedAllRegistries does not seed combat messages
// at all (attackMessages starts as an empty package-level map), and
// GetPreAttackMessage's Generic fallback returns a zero AttackOptions with no
// entry, so GetWaitMessages would send nothing without this.
func seedWaitMessageFixture() func() {
	line := items.MessageOptions{items.ItemMessage(waitTodefenderLine)}
	tiered := items.SkillTieredMessages{Beginner: line}
	options := items.AttackOptions{
		Together: items.TogetherMessages{
			ToDefender: tiered,
		},
	}
	return items.SeedAttackMessagesForTest(map[items.ItemSubType]*items.WeaponAttackMessageGroup{
		items.Generic: {
			OptionId: items.Generic,
			Options: items.AttackTypes{
				items.Wait: options,
			},
		},
	})
}

// armWait puts mobChar into a waiting state against userId: SetAggro with a
// positive wait-round count builds the CombatPhase machine, engages the
// target, and seeds RoundsWaiting so the next ConsumeRoundWaiting() call
// returns true exactly once.
func armWait(mobChar *characters.Character, userId int) {
	mobChar.SetAggro(userId, 0, characters.DefaultAttack, 3)
}

func TestWaitRound_DarkRoomHidesAttackerName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreMsgs := seedWaitMessageFixture()
	defer restoreMsgs()

	util.SetRoundCountForTest(100)
	defer util.ResetRoundCountForTest()

	darken(t, 1)

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	user1 := users.GetByUserId(1)
	require.NotNil(t, user1)
	room := rooms.LoadRoom(1)
	require.NotNil(t, room)

	armWait(&mob.Character, 1)

	handled := handleCombatWaitRound(&mob.Character, user1.Character, combat.Mob, combat.User, nil, user1, room, room, 1)
	require.True(t, handled, "attacker should still be in its wait round")

	lines := drainPlain(1)
	require.NotEmpty(t, lines, "defender should receive at least one wait-round line")
	require.Zero(t, countContaining(lines, "Skeleton"), "dark room: defender must not read the mob's name: %v", lines)
	require.NotZero(t, countContaining(lines, "something"), "dark room: defender should read \"something\" in place of the name: %v", lines)
}

func TestWaitRound_LitRoomShowsAttackerName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreMsgs := seedWaitMessageFixture()
	defer restoreMsgs()

	util.SetRoundCountForTest(100)
	defer util.ResetRoundCountForTest()

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	user1 := users.GetByUserId(1)
	require.NotNil(t, user1)
	room := rooms.LoadRoom(1)
	require.NotNil(t, room)

	armWait(&mob.Character, 1)

	handled := handleCombatWaitRound(&mob.Character, user1.Character, combat.Mob, combat.User, nil, user1, room, room, 1)
	require.True(t, handled, "attacker should still be in its wait round")

	lines := drainPlain(1)
	require.NotEmpty(t, lines, "defender should receive at least one wait-round line")
	require.NotZero(t, countContaining(lines, "Skeleton"), "lit room: defender should read the mob's name: %v", lines)
}
