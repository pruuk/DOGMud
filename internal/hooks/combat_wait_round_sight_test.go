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
// handleCombatWaitRound now judges sight with the swing path's own
// predicate (messaging.CanSeeSightImpairedOnly) and, for a participant
// without clear sight, sends one fixed dark line instead of the authored
// MessagesToSource/MessagesToTarget line. This pins the fix: the authored
// lines name the weapon as well as the foe, so hiding just the name is not
// enough -- a reader without clear sight must not see the weapon phrase
// either, and must read the authored line unchanged once they can see.

// waitTodefenderLine is the authored shape from
// _datafiles/world/dogmud/combat-messages/generic.yaml:104 under
// `todefender`, with a weapon phrase folded in the way the real per-weapon
// files (e.g. cleaving.yaml) do -- this is the exact defect example: "Something
// holds their Rusted Cleaver steady, eyes fixed on you."
const waitTodefenderLine = `<ansi fg="{sourcetype}">{source}</ansi> holds their <ansi fg="item">Rusted Cleaver</ansi> steady, eyes fixed on you.`

// waitToattackerLine is the authored shape from
// _datafiles/world/dogmud/combat-messages/generic.yaml:94 under
// `wait/together/toattacker`. Every one of the 20 weapon files has a wait
// toattacker line naming {target} the same way, so this side of the drain
// (MessagesToSource) is just as load-bearing as the todefender side.
const waitToattackerLine = `You watch <ansi fg="{targettype}">{target}</ansi> closely, waiting for the perfect moment.`

// seedWaitMessageFixture installs a minimal attackMessages registry whose
// Generic/Wait/Together.ToDefender and .ToAttacker entries are the observed
// authored lines, and returns the restore func. seedAllRegistries does not
// seed combat messages at all (attackMessages starts as an empty
// package-level map), and GetPreAttackMessage's Generic fallback returns a
// zero AttackOptions with no entry, so GetWaitMessages would send nothing
// without this.
func seedWaitMessageFixture() func() {
	toDefender := items.SkillTieredMessages{Beginner: items.MessageOptions{items.ItemMessage(waitTodefenderLine)}}
	toAttacker := items.SkillTieredMessages{Beginner: items.MessageOptions{items.ItemMessage(waitToattackerLine)}}
	options := items.AttackOptions{
		Together: items.TogetherMessages{
			ToAttacker: toAttacker,
			ToDefender: toDefender,
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

// armWaitOnMob is armWait with the roles reversed: a player character waits
// against a mob instance.
func armWaitOnMob(attackerChar *characters.Character, mobInstanceId int) {
	attackerChar.SetAggro(0, mobInstanceId, characters.DefaultAttack, 3)
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
	require.Len(t, lines, 1, "dark room: defender should receive exactly the fixed dark line: %v", lines)
	require.Equal(t, plainText(waitRoundDarkDefenderLine), lines[0])
	require.Zero(t, countContaining(lines, "Skeleton"), "dark room: defender must not read the mob's name: %v", lines)
	require.Zero(t, countContaining(lines, "Rusted Cleaver"), "dark room: defender must not read the weapon: %v", lines)
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
	require.Len(t, lines, 1, "lit room: defender should receive exactly the authored line: %v", lines)
	require.NotZero(t, countContaining(lines, "Skeleton"), "lit room: defender should read the mob's name: %v", lines)
	require.NotZero(t, countContaining(lines, "Rusted Cleaver"), "lit room: defender should read the weapon: %v", lines)
}

// TestWaitRound_DarkRoomHidesTargetNameFromAttacker pins the other half of
// the drain: MessagesToSource, sent to the ATTACKER. Both tests above pass a
// nil attackerUser (attacker is the mob), so neither one ever reaches the
// `:66` send. Here the player is the attacker waiting on a mob target, so
// the toattacker line's {target} names the mob and must be hidden the same
// way.
func TestWaitRound_DarkRoomHidesTargetNameFromAttacker(t *testing.T) {
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

	armWaitOnMob(user1.Character, 100)

	handled := handleCombatWaitRound(user1.Character, &mob.Character, combat.User, combat.Mob, user1, nil, room, room, 1)
	require.True(t, handled, "attacker should still be in its wait round")

	lines := drainPlain(1)
	require.Len(t, lines, 1, "dark room: attacker should receive exactly the fixed dark line: %v", lines)
	require.Equal(t, plainText(waitRoundDarkAttackerLine), lines[0])
	require.Zero(t, countContaining(lines, "Skeleton"), "dark room: attacker must not read the target's name: %v", lines)
	require.Zero(t, countContaining(lines, "Rusted Cleaver"), "dark room: attacker must not read the weapon: %v", lines)
}

// TestWaitRound_DarkRoomInfraredDefenderStillGetsDarkLine pins the swing
// path's convention that infrared is not clear sight: CanSeeSightImpairedOnly
// checks buffs.NightVision, not buffs.InfraredVision, so a defender who can
// only see shapes in the dark still reads the fixed dark line, not the
// authored one.
func TestWaitRound_DarkRoomInfraredDefenderStillGetsDarkLine(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreMsgs := seedWaitMessageFixture()
	defer restoreMsgs()
	restore := seedNarrationBuffs()
	defer restore()

	util.SetRoundCountForTest(100)
	defer util.ResetRoundCountForTest()

	darken(t, 1)

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	user1 := users.GetByUserId(1)
	require.NotNil(t, user1)
	room := rooms.LoadRoom(1)
	require.NotNil(t, room)

	require.True(t, users.GetByUserId(1).Character.Buffs.AddBuff(heatEyesBuffId, true))

	armWait(&mob.Character, 1)

	handled := handleCombatWaitRound(&mob.Character, user1.Character, combat.Mob, combat.User, nil, user1, room, room, 1)
	require.True(t, handled, "attacker should still be in its wait round")

	lines := drainPlain(1)
	require.Len(t, lines, 1, "infrared: defender should still receive exactly the fixed dark line: %v", lines)
	require.Equal(t, plainText(waitRoundDarkDefenderLine), lines[0])
}

// TestWaitRound_EmptyAuthoredListStaysSilentInTheDark pins that the dark
// line is only sent when there was an authored line to replace. No wait
// messages are seeded for the weapon subtype here (attackMessages stays the
// empty map seedAllRegistries leaves it as), so a dark defender must receive
// nothing at all, not an invented line.
func TestWaitRound_EmptyAuthoredListStaysSilentInTheDark(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

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
	require.Empty(t, lines, "no authored line means no invented dark line either: %v", lines)
}
