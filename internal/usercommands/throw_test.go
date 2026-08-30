package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const throwCostItemID = 79061

func seedThrowCostFixture(t *testing.T) (*users.UserRecord, *rooms.Room, *mobs.Mob, items.Item, items.Item) {
	t.Helper()

	cleanupItems := items.SeedItemsForTest(map[int]*items.ItemSpec{
		throwCostItemID: {
			ItemId:           throwCostItemID,
			Name:             "clockwork bomb",
			NameSimple:       "bomb",
			Subtype:          items.Throwable,
			Uses:             5,
			DamageMultiplier: 1,
		},
	})
	t.Cleanup(cleanupItems)

	targetChar := characters.New()
	targetChar.Name = "Target"
	targetChar.RoomId = 1
	targetChar.Health = 500
	targetChar.HealthMax.Value = 500
	targetChar.Stats.Dexterity.ValueAdj = 1
	targetChar.Stats.Perception.ValueAdj = 1
	target := &mobs.Mob{MobId: 1, InstanceId: 79062, HomeRoomId: 1, Character: *targetChar}
	cleanupMobs := mobs.SeedMobsForTest(
		map[int]*mobs.Mob{1: {MobId: 1, Character: characters.Character{Name: "Target"}}},
		map[int]*mobs.Mob{target.InstanceId: target},
	)
	t.Cleanup(cleanupMobs)

	cleanupBiomes := rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"test": {BiomeId: "test", Name: "Test", LitArea: true},
	})
	t.Cleanup(cleanupBiomes)
	room := &rooms.Room{RoomId: 1, Zone: "test", Biome: "test"}
	cleanupRooms := rooms.SeedRoomsForTest(map[int]*rooms.Room{1: room}, nil)
	t.Cleanup(cleanupRooms)
	room.AddMob(target.InstanceId)

	user := users.NewTestUser(79063, "thrower", "Thrower", 0)
	user.Character.RoomId = room.RoomId
	user.Character.Stats.Dexterity.ValueAdj = 100000
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{user.UserId: user})
	t.Cleanup(cleanupUsers)

	first := items.New(throwCostItemID)
	first.Uses = 2
	selected := items.New(throwCostItemID)
	selected.Uses = 5
	user.Character.Items = []items.Item{first, selected}

	return user, room, target, first, selected
}

// TestThrowCostRefusalPreservesExactItemAndCombatState catches any path that
// mutates the selected UUID-backed stack, cooldown, progression, target/user
// health, aggression, round state, or fractional carry before full admission.
func TestThrowCostRefusalPreservesExactItemAndCombatState(t *testing.T) {
	user, room, target, first, selected := seedThrowCostFixture(t)
	user.Character.Stamina = 0
	user.Character.SetAggro(0, target.InstanceId, characters.DefaultAttack)
	user.Character.SetRoundsWaiting(0)
	events.DrainQueuedMessagesForTest(user.UserId)

	itemsBefore := append([]items.Item(nil), user.Character.Items...)
	cooldownsBefore := user.Character.GetAllCooldowns()
	userHealthBefore := user.Character.Health
	targetHealthBefore := target.Character.Health
	progressionBefore := user.Character.GetSkillUseCount(string(skills.Skullduggery))

	handled, err := Throw(characters.ItemHandleSigil+selected.UUID.String(), user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	assert.Equal(t, itemsBefore, user.Character.Items, "refusal must preserve both exact item instances and their Uses")
	assert.Equal(t, first.UUID, user.Character.Items[0].UUID)
	assert.Equal(t, selected.UUID, user.Character.Items[1].UUID)
	assert.Equal(t, cooldownsBefore, user.Character.GetAllCooldowns())
	assert.Equal(t, userHealthBefore, user.Character.Health)
	assert.Equal(t, targetHealthBefore, target.Character.Health)
	assert.Nil(t, target.Character.Aggro, "refusal must not seed target aggression")
	require.NotNil(t, user.Character.Aggro)
	assert.Equal(t, 0, user.Character.RoundsWaiting(), "refusal must not consume the combat round")
	assert.Equal(t, progressionBefore, user.Character.GetSkillUseCount(string(skills.Skullduggery)))
	assert.Equal(t, 0, user.Character.Stamina)
	assertVoluntaryRefusalOutput(t, events.DrainQueuedMessagesForTest(user.UserId), characters.PoolStamina)

	// A refused quote must not advance fractional carry. Rank-zero throw costs
	// 4.4 from a zero carry, so two fresh commits charge 4 then 4 (not 4 then 5).
	user.Character.Stamina = 100
	charged := make([]int, 0, 2)
	for range 2 {
		quote := user.Character.QuoteActionCost(characters.ActionCostRequest{
			Action: costs.ActionThrow, Pool: characters.PoolStamina,
			Base: float64(configs.GetBalanceConfig().SpecialMoveBaseStaminaCost), Modifier: 1, Units: 1,
		})
		charged = append(charged, user.Character.CommitCost(quote, characters.CostFullOrRefuse).Charged)
	}
	assert.Equal(t, []int{4, 4}, charged)
}

func TestThrowCostPaidAdmissionConsumesOnlySelectedStack(t *testing.T) {
	user, room, target, first, selected := seedThrowCostFixture(t)
	room.RemoveMob(target.InstanceId)
	mobs.SetInstanceForTest(target.InstanceId, nil)
	user.Character.Stamina = 100

	handled, err := Throw(characters.ItemHandleSigil+selected.UUID.String(), user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	require.Len(t, user.Character.Items, 2)
	assert.Equal(t, first.UUID, user.Character.Items[0].UUID)
	assert.Equal(t, 2, user.Character.Items[0].Uses)
	assert.Equal(t, selected.UUID, user.Character.Items[1].UUID)
	assert.Equal(t, 4, user.Character.Items[1].Uses)
	assert.Equal(t, 96, user.Character.Stamina)
	assert.Equal(t, 5, user.Character.Cooldowns["special-move"])
	assert.Equal(t, 1, user.Character.GetSkillUseCount(string(skills.Skullduggery)))
}

func TestThrowCostPaidAdmissionStaleItemDoesNotConsumeReplacement(t *testing.T) {
	user, room, _, first, selected := seedThrowCostFixture(t)
	user.Character.Stamina = 100
	replacement := items.New(throwCostItemID)
	replacement.Uses = 3

	originalAdmit := admitThrowCost
	admitThrowCost = func(char *characters.Character, base float64) characters.CostCommitResult {
		result := originalAdmit(char, base)
		char.Items = []items.Item{first, replacement}
		return result
	}
	t.Cleanup(func() { admitThrowCost = originalAdmit })

	handled, err := Throw(characters.ItemHandleSigil+selected.UUID.String(), user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	assert.Equal(t, 96, user.Character.Stamina, "already-paid admission remains paid")
	assert.Equal(t, []items.Item{first, replacement}, user.Character.Items)
	assert.Empty(t, user.Character.Cooldowns)
	assert.Zero(t, user.Character.GetSkillUseCount(string(skills.Skullduggery)))
}

func TestThrowCostPaidAdmissionStaleCooldownPreservesItem(t *testing.T) {
	user, room, _, _, selected := seedThrowCostFixture(t)
	user.Character.Stamina = 100
	itemsBefore := append([]items.Item(nil), user.Character.Items...)

	originalAdmit := admitThrowCost
	admitThrowCost = func(char *characters.Character, base float64) characters.CostCommitResult {
		result := originalAdmit(char, base)
		char.Cooldowns["special-move"] = 3
		return result
	}
	t.Cleanup(func() { admitThrowCost = originalAdmit })

	handled, err := Throw(characters.ItemHandleSigil+selected.UUID.String(), user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	assert.Equal(t, 96, user.Character.Stamina, "already-paid admission remains paid")
	assert.Equal(t, itemsBefore, user.Character.Items)
	assert.Equal(t, 3, user.Character.Cooldowns["special-move"])
	assert.Zero(t, user.Character.GetSkillUseCount(string(skills.Skullduggery)))
}

func TestThrowCostPaidAdmissionStaleTargetDoesNotResolveReplacement(t *testing.T) {
	user, room, originalTarget, _, selected := seedThrowCostFixture(t)
	user.Character.Stamina = 100
	replacementChar := characters.New()
	replacementChar.Name = "Replacement"
	replacementChar.RoomId = room.RoomId
	replacementChar.Health = 500
	replacementChar.HealthMax.Value = 500
	replacementChar.Stats.Dexterity.ValueAdj = 1
	replacementChar.Stats.Perception.ValueAdj = 1
	replacement := &mobs.Mob{
		MobId: 1, InstanceId: originalTarget.InstanceId, HomeRoomId: room.RoomId,
		Character: *replacementChar,
	}

	originalAdmit := admitThrowCost
	admitThrowCost = func(char *characters.Character, base float64) characters.CostCommitResult {
		result := originalAdmit(char, base)
		mobs.SetInstanceForTest(originalTarget.InstanceId, replacement)
		return result
	}
	t.Cleanup(func() { admitThrowCost = originalAdmit })

	handled, err := Throw(characters.ItemHandleSigil+selected.UUID.String(), user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	assert.Equal(t, 96, user.Character.Stamina)
	assert.Equal(t, 4, user.Character.Items[1].Uses)
	assert.Equal(t, 500, originalTarget.Character.Health)
	assert.Nil(t, originalTarget.Character.Aggro)
	assert.Equal(t, 500, replacement.Character.Health)
	assert.Nil(t, replacement.Character.Aggro, "same-ID replacement must not be resolved")
	assert.Nil(t, user.Character.Aggro)
	assert.Equal(t, 1, user.Character.GetSkillUseCount(string(skills.Skullduggery)))
}

func seedSneakCommandCostFixture(t *testing.T) (*users.UserRecord, *rooms.Room, *mobs.Mob) {
	t.Helper()

	cfg := configs.GetConfig()
	cfg.Balance.ContestFloor = 0
	cfg.Balance.BaseProgressionChance = 1
	cfg.Balance.SneakFailCooldown = 0
	configs.SetConfigForTest(t, cfg)

	observerChar := characters.New()
	observerChar.Name = "Watcher"
	observerChar.RoomId = 1
	observerChar.IsMob = true
	observerChar.Stats.Dexterity.ValueAdj = 100000
	observerChar.Stats.Perception.ValueAdj = 100000
	observer := &mobs.Mob{MobId: 1, InstanceId: 79073, HomeRoomId: 1, Character: *observerChar}
	cleanupMobs := mobs.SeedMobsForTest(
		map[int]*mobs.Mob{1: {MobId: 1, Character: characters.Character{Name: "Watcher"}}},
		map[int]*mobs.Mob{observer.InstanceId: observer},
	)
	t.Cleanup(cleanupMobs)

	cleanupBiomes := rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"test": {BiomeId: "test", Name: "Test", LitArea: true},
	})
	t.Cleanup(cleanupBiomes)
	room := &rooms.Room{RoomId: 1, Zone: "test", Biome: "test"}
	cleanupRooms := rooms.SeedRoomsForTest(map[int]*rooms.Room{1: room}, nil)
	t.Cleanup(cleanupRooms)
	room.AddMob(observer.InstanceId)

	user := users.NewTestUser(79074, "sneaker", "Sneaker", 0)
	user.Character.RoomId = room.RoomId
	user.Character.Stats.Dexterity.ValueAdj = 1
	user.Character.Skills = map[string]int{}
	user.Character.Skills[string(skills.Skullduggery)] = 1
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{user.UserId: user})
	t.Cleanup(cleanupUsers)
	events.DrainQueuedMessagesForTest(user.UserId)
	t.Cleanup(func() { events.DrainQueuedMessagesForTest(user.UserId) })

	return user, room, observer
}

func TestSneakUserCostRefusalPreservesCooldownAwarenessProgressionAndRound(t *testing.T) {
	user, room, _ := seedSneakCommandCostFixture(t)
	user.Character.Stamina = 0
	user.Character.Cooldowns = nil
	skillBefore := user.Character.GetSkillLevel(skills.Skullduggery)
	roundBefore := user.Character.AttacksThisRound

	handled, err := Sneak("", user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	assert.Nil(t, user.Character.Cooldowns, "read-only prior-failure probe and cost refusal must not initialize cooldowns")
	assert.Equal(t, awareness.Visible, user.Character.Awareness.State())
	assert.False(t, user.Character.IsHidden())
	assert.Nil(t, user.Character.GetMiscData("sneaking"))
	assert.Equal(t, skillBefore, user.Character.GetSkillLevel(skills.Skullduggery))
	assert.Zero(t, user.Character.GetSkillUseCount(string(skills.Skullduggery)))
	assert.Equal(t, roundBefore, user.Character.AttacksThisRound)
	assert.Equal(t, 0, user.Character.Stamina)
	lines := events.DrainQueuedMessagesForTest(user.UserId)
	assertVoluntaryRefusalOutput(t, lines, characters.PoolStamina)
	msgs := strings.Join(lines, "\n")
	assert.NotContains(t, msgs, "You slip into the shadows.")
}

func TestSneakUserCostAffordableFailurePaysProgressesAndKeepsZeroCooldown(t *testing.T) {
	user, room, _ := seedSneakCommandCostFixture(t)
	user.Character.Stamina = 100
	user.Character.Cooldowns = nil

	handled, err := Sneak("", user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	assert.Equal(t, 98, user.Character.Stamina)
	assert.Equal(t, awareness.Visible, user.Character.Awareness.State())
	assert.False(t, user.Character.IsHidden())
	// U10b-1 Task 18: a FAILED sneak still awards, but at
	// ProgressionFailureFraction rather than full weight, so whether the level
	// actually ADVANCES is now a 0.35-weighted roll. Asserting the level here
	// would be asserting a coin flip.
	//
	// The property this test cares about -- "a real failed opposed attempt must
	// retain player progression" -- is intact and is now read off the use
	// counter, which OnSkillUseScaled tracks unconditionally.
	assert.Positive(t, user.Character.GetSkillUseCount(string(skills.Skullduggery)),
		"a real failed opposed attempt must still fire a progression award")
	assert.True(t, user.Character.CooldownReady(skills.Skullduggery.String("sneak")),
		"shipped zero SneakFailCooldown must remain effective zero")
	assert.Zero(t, user.Character.GetCooldown(skills.Skullduggery.String("sneak")))
	msgs := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	assert.Contains(t, msgs, "notices you")
}

func TestSneakUserCostPriorFailureCooldownIsReadOnly(t *testing.T) {
	user, room, _ := seedSneakCommandCostFixture(t)
	key := skills.Skullduggery.String("sneak")
	user.Character.Stamina = 100
	user.Character.Cooldowns = characters.Cooldowns{key: 2, "other": -1}
	cooldownsBefore := user.Character.GetAllCooldowns()

	handled, err := Sneak("", user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	assert.Equal(t, cooldownsBefore, user.Character.GetAllCooldowns())
	assert.Equal(t, 100, user.Character.Stamina)
	assert.Equal(t, awareness.Visible, user.Character.Awareness.State())
	assert.Zero(t, user.Character.GetSkillUseCount(string(skills.Skullduggery)))
}

func TestSneakUserCostAffordableFailureAppliesConfiguredCooldownAfterSpotting(t *testing.T) {
	user, room, _ := seedSneakCommandCostFixture(t)
	cfg := configs.GetConfig()
	cfg.Balance.SneakFailCooldown = 3
	configs.SetConfigForTest(t, cfg)
	key := skills.Skullduggery.String("sneak")
	user.Character.Stamina = 100

	handled, err := Sneak("", user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	assert.Equal(t, 98, user.Character.Stamina)
	assert.Equal(t, 3, user.Character.Cooldowns[key])
	assert.Equal(t, awareness.Visible, user.Character.Awareness.State())
	// See the note in the sibling test above: a failed sneak's award is
	// fraction-weighted since U10b-1 Task 18, so the use counter is the stable
	// observable rather than the level.
	assert.Positive(t, user.Character.GetSkillUseCount(string(skills.Skullduggery)))
}

// setMobCastingForTest puts a mob's Character into the Casting activity
// state. Mirrors internal/actions/cast_interrupt_test.go's setCastingForTest
// helper (unexported there, so replicated here for this package's tests).
func setMobCastingForTest(mob *mobs.Mob, spellId string) {
	mob.Character.Activity = activity.NewMachine()
	_ = mob.Character.Activity.TransitionToCasting(
		activity.CastingData{SpellId: spellId},
		state.TransitionReason{Trigger: activity.TriggerCastBegin},
	)
}

// newTestMob builds a minimal standalone mob instance (not registered in the
// mob registry) suitable for exercising maybeInterruptOnThrow directly.
func newTestMob(instanceId int) *mobs.Mob {
	m := &mobs.Mob{
		MobId:      1,
		InstanceId: instanceId,
		HomeRoomId: 1,
		Character: characters.Character{
			Name:      "Warden-Prime",
			RoomId:    1,
			Health:    500,
			Buffs:     buffs.New(),
			Cooldowns: map[string]int{},
		},
	}
	m.Character.HealthMax.Value = 500
	return m
}

// TestMaybeInterruptOnThrow_DisruptorInterruptsCastingMob: a configured
// disruptor item (the flashbang, 30057) thrown at a mid-cast mob cancels the
// cast via the shared InterruptTargetCast primitive.
func TestMaybeInterruptOnThrow_DisruptorInterruptsCastingMob(t *testing.T) {
	mob := newTestMob(500)
	setMobCastingForTest(mob, "core-discharge")
	require.True(t, mob.Character.IsCasting(), "pre-condition: mob should be casting")

	by := state.ActorRef{UserId: 1}
	interrupted := maybeInterruptOnThrow(mob, 30057, by)

	assert.True(t, interrupted, "a configured disruptor must interrupt a casting mob")
	assert.False(t, mob.Character.IsCasting(), "mob's cast should be cancelled after the interrupt")
}

// TestMaybeInterruptOnThrow_NonDisruptorDoesNotInterrupt: a thrown item NOT
// in BossInterruptItemIds (generic grenade / rock / etc.) must never
// interrupt a cast — only the allowlisted disruptor does.
func TestMaybeInterruptOnThrow_NonDisruptorDoesNotInterrupt(t *testing.T) {
	mob := newTestMob(501)
	setMobCastingForTest(mob, "core-discharge")
	require.True(t, mob.Character.IsCasting(), "pre-condition: mob should be casting")

	by := state.ActorRef{UserId: 1}
	interrupted := maybeInterruptOnThrow(mob, 40001, by) // arbitrary non-disruptor item id

	assert.False(t, interrupted, "a non-disruptor item must not interrupt a cast")
	assert.True(t, mob.Character.IsCasting(), "mob should still be casting")
}

// TestMaybeInterruptOnThrow_DisruptorOnNonCastingMob_NoOp: throwing a
// disruptor at a mob that isn't casting anything is a no-op — nothing to
// interrupt.
func TestMaybeInterruptOnThrow_DisruptorOnNonCastingMob_NoOp(t *testing.T) {
	mob := newTestMob(502)
	require.False(t, mob.Character.IsCasting(), "pre-condition: mob should not be casting")

	by := state.ActorRef{UserId: 1}
	interrupted := maybeInterruptOnThrow(mob, 30057, by)

	assert.False(t, interrupted, "a disruptor thrown at a non-casting mob is a no-op")
}

// TestMaybeInterruptOnThrow_NilMob_NoPanic: defensive nil guard.
func TestMaybeInterruptOnThrow_NilMob_NoPanic(t *testing.T) {
	by := state.ActorRef{UserId: 1}
	assert.NotPanics(t, func() {
		interrupted := maybeInterruptOnThrow(nil, 30057, by)
		assert.False(t, interrupted)
	})
}
