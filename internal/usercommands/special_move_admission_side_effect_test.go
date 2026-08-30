package usercommands

import (
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/crimes"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/factions"
	"github.com/GoMudEngine/GoMud/internal/items"
	mobcmd "github.com/GoMudEngine/GoMud/internal/mobcommands"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/opinions"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type specialMoveCommand func(string, *users.UserRecord, *rooms.Room, events.EventFlag) (bool, error)

type staleWrapperCooldownActor struct {
	actions.Actor
	characterCalls int
}

func (a *staleWrapperCooldownActor) GetCharacter() *characters.Character {
	a.characterCalls++
	char := a.Actor.GetCharacter()
	if a.characterCalls == 2 {
		char.Cooldowns["special-move"] = 3
	}
	return char
}

func specialMoveWrapperCases() []struct {
	name      string
	command   specialMoveCommand
	speciesID int
} {
	return []struct {
		name      string
		command   specialMoveCommand
		speciesID int
	}{
		{name: "bash", command: Bash, speciesID: 10},
		{name: "trip", command: Trip, speciesID: 11},
		{name: "kick", command: Kick, speciesID: 11},
		{name: "grapple", command: Grapple, speciesID: 11},
		{name: "rake", command: Rake, speciesID: 12},
		{name: "maul", command: Maul, speciesID: 13},
		{name: "pounce", command: Pounce, speciesID: 13},
		{name: "gore", command: Gore, speciesID: 14},
		{name: "drain", command: Drain, speciesID: 15},
		{name: "throttle", command: Throttle, speciesID: 13},
	}
}

func replaceStagedBaseActor(t *testing.T, staged actions.Actor, replacement actions.Actor) {
	t.Helper()
	value := reflect.ValueOf(staged)
	require.Equal(t, reflect.Pointer, value.Kind())
	value = value.Elem()
	field := value.FieldByName("Actor")
	require.True(t, field.IsValid(), "real StageMeleeTarget result must retain its embedded actor")
	require.True(t, field.CanSet(), "real StageMeleeTarget embedded actor must be replaceable inside the test seam")
	field.Set(reflect.ValueOf(replacement))
}

func stagedBaseActor(t *testing.T, staged actions.Actor) actions.Actor {
	t.Helper()
	value := reflect.ValueOf(staged)
	require.Equal(t, reflect.Pointer, value.Kind())
	field := value.Elem().FieldByName("Actor")
	require.True(t, field.IsValid())
	base, ok := field.Interface().(actions.Actor)
	require.True(t, ok)
	return base
}

func seedSpecialMoveSpecies() func() {
	return species.SeedSpeciesForTest(map[int]*species.Species{
		0:  {SpeciesId: 0, Name: "featureless", BodyParts: nil},
		10: {SpeciesId: 10, Name: "living ram", NaturalBash: true, BodyParts: []string{"arms", "legs"}},
		11: {SpeciesId: 11, Name: "humanoid", BodyParts: []string{"arms", "hands", "legs"}},
		12: {SpeciesId: 12, Name: "clawed beast", NaturalAttack: items.Claws, BodyParts: []string{"legs", "mouth"}},
		13: {SpeciesId: 13, Name: "fanged beast", NaturalAttack: items.Bite, BodyParts: []string{"legs", "mouth"}},
		14: {SpeciesId: 14, Name: "horned beast", NaturalAttack: items.Gore, BodyParts: []string{"legs", "horns"}},
		15: {SpeciesId: 15, Name: "life drainer", LifeDrain: true, BodyParts: []string{"mouth"}},
	})
}

func configureSpecialMoveFaction(t *testing.T) {
	t.Helper()
	definitions := t.TempDir()
	t.Setenv("DOGMUD_FACTIONS_DIR_OVERRIDE", definitions)
	t.Setenv("DOGMUD_FACTIONS_REP_DIR_OVERRIDE", t.TempDir())
	t.Setenv("DOGMUD_FACTIONS_CRIMES_DIR_OVERRIDE", t.TempDir())
	t.Setenv("DOGMUD_OPINIONS_DIR_OVERRIDE", t.TempDir())
	require.NoError(t, os.WriteFile(filepath.Join(definitions, "thornwall_citizens.yaml"), []byte(`faction_id: thornwall_citizens
display_name: "Citizens"
description: "test faction"
default_rep: 0
allies: []
enemies: []
`), 0644))
	require.NoError(t, factions.LoadAllDefinitions())
	factions.ClearCache()
	crimes.ClearCache()
	opinions.ClearCache()
}

func resetSpecialMoveWrapperFixture(t *testing.T, user *users.UserRecord, target *mobs.Mob, speciesID int, stamina int) {
	t.Helper()
	user.Character = &characters.Character{
		Name:      "Aliceia",
		RoomId:    1,
		SpeciesId: speciesID,
		Health:    100,
		Stamina:   stamina,
		Buffs:     buffs.New(),
		Cooldowns: map[string]int{},
	}
	user.Character.HealthMax.Value = 100
	user.Character.StaminaMax.Value = 100
	user.Character.Stats.Strength.ValueAdj = 100
	user.Character.Stats.Dexterity.ValueAdj = 100
	target.Groups = []string{"thornwall_citizens"}
	target.Character.Health = 1_000_000
	target.Character.HealthMax.Value = 1_000_000
	target.Character.Stamina = 1_000_000
	target.Character.StaminaMax.Value = 1_000_000
	target.Character.EndAggro()
	events.DrainQueuedMessagesForTest(user.UserId)
	events.DrainQueuedPlayerAttackedMobsForTest(0)
}

func assertNoSpecialMoveEngagement(t *testing.T, user *users.UserRecord, target *mobs.Mob) {
	t.Helper()
	assert.False(t, user.Character.IsInCombat(), "refused action must not set aggro")
	assert.Empty(t, events.DrainQueuedPlayerAttackedMobsForTest(user.UserId), "refused action must not queue aggression")
	assert.Equal(t, 0, opinions.Get(int(target.MobId), user.UserId), "refused action must not change opinion")
	assert.Empty(t, crimes.AllForFaction("thornwall_citizens", false), "refused action must not record assault")
}

func TestSpecialMoveWrappersRefusalHasNoEngagementSideEffects(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	speciesCleanup := seedSpecialMoveSpecies()
	defer speciesCleanup()

	user, room := getTestUserAndRoom(t)
	target := mobs.GetInstance(100)
	require.NotNil(t, target)

	cases := []struct {
		name      string
		command   specialMoveCommand
		speciesID int
	}{
		{name: "bash", command: Bash, speciesID: 10},
		{name: "trip", command: Trip, speciesID: 11},
		{name: "kick", command: Kick, speciesID: 11},
		{name: "grapple", command: Grapple, speciesID: 11},
		{name: "rake", command: Rake, speciesID: 12},
		{name: "maul", command: Maul, speciesID: 13},
		{name: "pounce", command: Pounce, speciesID: 13},
		{name: "gore", command: Gore, speciesID: 14},
		{name: "drain", command: Drain, speciesID: 15},
		{name: "throttle", command: Throttle, speciesID: 13},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			configureSpecialMoveFaction(t)
			resetSpecialMoveWrapperFixture(t, user, target, tc.speciesID, 0)

			handled, err := tc.command("#100", user, room, 0)
			require.NoError(t, err)
			require.True(t, handled)
			require.Equal(t, 0, user.Character.Stamina)
			require.Empty(t, user.Character.Cooldowns)
			assertVoluntaryRefusalOutput(t, events.DrainQueuedMessagesForTest(user.UserId), characters.PoolStamina)
			assertNoSpecialMoveEngagement(t, user, target)
		})
	}
}

func TestMobSpecialMoveWrappersRefuseSilentlyWithoutMutation(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	speciesCleanup := seedSpecialMoveSpecies()
	defer speciesCleanup()

	cases := []struct {
		name      string
		command   func(string, *mobs.Mob, *rooms.Room) (bool, error)
		speciesID int
	}{
		{"bash", mobcmd.Bash, 10},
		{"trip", mobcmd.Trip, 11},
		{"kick", mobcmd.Kick, 11},
		{"grapple", mobcmd.Grapple, 11},
		{"rake", mobcmd.Rake, 12},
		{"maul", mobcmd.Maul, 13},
		{"pounce", mobcmd.Pounce, 13},
		{"gore", mobcmd.Gore, 14},
		{"drain", mobcmd.Drain, 15},
		{"throttle", mobcmd.Throttle, 13},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mob, room := mobs.GetInstance(100), rooms.LoadRoom(1)
			mob.Character.SpeciesId = tc.speciesID
			mob.Character.Stamina = 0
			mob.Character.Cooldowns = characters.Cooldowns{}
			mob.Character.SetAggro(1, 0, characters.DefaultAttack)
			beforeAggro := mob.Character.CurrentCombatTarget()
			beforeHealth := mob.Character.Health
			for _, userID := range []int{1, 2} {
				events.DrainQueuedMessagesForTest(userID)
			}

			handled, err := tc.command("", mob, room)
			require.NoError(t, err)
			require.True(t, handled)
			require.Zero(t, mob.Character.Stamina)
			require.Equal(t, beforeHealth, mob.Character.Health)
			require.Equal(t, beforeAggro, mob.Character.CurrentCombatTarget())
			require.Empty(t, mob.Character.Cooldowns)
			require.Zero(t, mob.Character.AttacksThisRound)
			for _, userID := range []int{1, 2} {
				require.Empty(t, events.DrainQueuedMessagesForTest(userID), "mob refusal leaked to user %d", userID)
			}
		})
	}
}

func TestSpecialMoveWrappersInvalidAnatomyHasNoEngagementSideEffects(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	speciesCleanup := seedSpecialMoveSpecies()
	defer speciesCleanup()

	user, room := getTestUserAndRoom(t)
	target := mobs.GetInstance(100)
	require.NotNil(t, target)

	cases := []struct {
		name    string
		command specialMoveCommand
	}{
		{name: "bash", command: Bash},
		{name: "trip", command: Trip},
		{name: "kick", command: Kick},
		{name: "grapple", command: Grapple},
		{name: "rake", command: Rake},
		{name: "maul", command: Maul},
		{name: "pounce", command: Pounce},
		{name: "gore", command: Gore},
		{name: "drain", command: Drain},
		{name: "throttle", command: Throttle},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			configureSpecialMoveFaction(t)
			resetSpecialMoveWrapperFixture(t, user, target, 0, 100)

			handled, err := tc.command("#100", user, room, 0)
			require.NoError(t, err)
			require.True(t, handled)
			require.Equal(t, 100, user.Character.Stamina)
			require.Empty(t, user.Character.Cooldowns)
			assertNoSpecialMoveEngagement(t, user, target)
		})
	}
}

func TestStagedSpecialMovePaidMissCommitsEngagement(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	speciesCleanup := seedSpecialMoveSpecies()
	defer speciesCleanup()
	configureSpecialMoveFaction(t)

	user, room := getTestUserAndRoom(t)
	target := mobs.GetInstance(100)
	require.NotNil(t, target)
	resetSpecialMoveWrapperFixture(t, user, target, 11, 100)
	target.Character.Stats.Dexterity.ValueAdj = 1_000_000

	actor, handled := actions.StageMeleeTarget(user, room, "#100", actions.MeleeTargetOpts{Verb: "trip"})
	require.False(t, handled)
	t.Setenv("GODEBUG", "randseednop=0")
	// nolint:staticcheck // Seeding the GLOBAL source is the point: the code under
	// test uses package-level rand, and there is no non-deprecated way to make
	// that deterministic. The GODEBUG above is what un-does the Go 1.20 no-op,
	// so unlike the other two former call sites this one actually takes effect.
	rand.Seed(1)
	res := actions.ExecuteTrip(actor)

	require.True(t, res.Executed)
	require.False(t, res.MoveResult.Hit, "fixture must exercise the paid-miss path")
	require.Less(t, user.Character.Stamina, 100)
	require.True(t, user.Character.IsInCombat())
	require.Equal(t, 100, user.Character.CurrentCombatTarget().MobInstanceId)
	require.Len(t, events.DrainQueuedPlayerAttackedMobsForTest(user.UserId), 1)
	require.Equal(t, -15, opinions.Get(int(target.MobId), user.UserId))
	require.Len(t, crimes.AllForFaction("thornwall_citizens", false), 1)
}

func TestStagedSpecialMoveTargetGoneBeforeAdmissionHasNoSideEffects(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	speciesCleanup := seedSpecialMoveSpecies()
	defer speciesCleanup()
	configureSpecialMoveFaction(t)

	user, room := getTestUserAndRoom(t)
	target := mobs.GetInstance(100)
	require.NotNil(t, target)
	resetSpecialMoveWrapperFixture(t, user, target, 11, 100)

	actor, handled := actions.StageMeleeTarget(user, room, "#100", actions.MeleeTargetOpts{Verb: "trip"})
	require.False(t, handled)
	room.RemoveMob(100)
	mobs.SetInstanceForTest(100, nil)
	defer func() {
		mobs.SetInstanceForTest(100, target)
		room.AddMob(100)
	}()

	res := actions.ExecuteTrip(actor)
	require.True(t, res.NoTarget)
	require.Equal(t, 100, user.Character.Stamina)
	require.Empty(t, user.Character.Cooldowns)
	assert.False(t, user.Character.IsInCombat())
	assert.Empty(t, events.DrainQueuedPlayerAttackedMobsForTest(user.UserId))
	assert.Equal(t, 0, opinions.Get(int(target.MobId), user.UserId))
	assert.Empty(t, crimes.AllForFaction("thornwall_citizens", false))
}

// TestSpecialMoveWrappersStagedRacesHaveNoEngagementSideEffects catches any
// player wrapper bypassing the shared staging seam or committing engagement
// before its shared executor has both admitted cost and consumed cooldown.
// Every row invokes the real command entry point; the seam only creates the
// target-loss/cooldown transition after real target validation has staged it.
func TestSpecialMoveWrappersStagedRacesHaveNoEngagementSideEffects(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	speciesCleanup := seedSpecialMoveSpecies()
	defer speciesCleanup()

	user, room := getTestUserAndRoom(t)
	target := mobs.GetInstance(100)
	require.NotNil(t, target)

	scenarios := []struct {
		name            string
		wantStamina     int
		wantCooldown    int
		transitionStage func(*testing.T, actions.Actor, *rooms.Room, *mobs.Mob) actions.Actor
	}{
		{
			name:         "target_gone",
			wantStamina:  100,
			wantCooldown: 0,
			transitionStage: func(t *testing.T, actor actions.Actor, room *rooms.Room, target *mobs.Mob) actions.Actor {
				room.RemoveMob(target.InstanceId)
				mobs.SetInstanceForTest(target.InstanceId, nil)
				t.Cleanup(func() {
					mobs.SetInstanceForTest(target.InstanceId, target)
					room.AddMob(target.InstanceId)
				})
				return actor
			},
		},
		{
			name:         "cooldown_stale_after_readiness",
			wantStamina:  96,
			wantCooldown: 3,
			transitionStage: func(t *testing.T, actor actions.Actor, _ *rooms.Room, _ *mobs.Mob) actions.Actor {
				base := stagedBaseActor(t, actor)
				replaceStagedBaseActor(t, actor, &staleWrapperCooldownActor{Actor: base})
				return actor
			},
		},
	}

	for _, scenario := range scenarios {
		for _, tc := range specialMoveWrapperCases() {
			t.Run(scenario.name+"/"+tc.name, func(t *testing.T) {
				configureSpecialMoveFaction(t)
				resetSpecialMoveWrapperFixture(t, user, target, tc.speciesID, 100)
				setCombatPositionParallel(user.Character, position.Standing)
				setCombatPositionParallel(&target.Character, position.Standing)

				targetHealth := target.Character.Health
				targetStamina := target.Character.Stamina
				targetConditions := len(target.Character.Conditions)
				targetBuffs := len(target.Character.Buffs.GetBuffs())
				actorHealth := user.Character.Health
				actorConditions := len(user.Character.Conditions)
				actorBuffs := len(user.Character.Buffs.GetBuffs())
				actorPosition := user.Character.Position.State()
				targetPosition := target.Character.Position.State()

				originalStage := stageSpecialMoveTarget
				stageCalls := 0
				stageSpecialMoveTarget = func(user *users.UserRecord, room *rooms.Room, rest string, opts actions.MeleeTargetOpts) (actions.Actor, bool) {
					stageCalls++
					actor, handled := actions.StageMeleeTarget(user, room, rest, opts)
					if handled {
						return actor, handled
					}
					return scenario.transitionStage(t, actor, room, target), false
				}
				t.Cleanup(func() { stageSpecialMoveTarget = originalStage })

				handled, err := tc.command("#100", user, room, 0)
				require.NoError(t, err)
				require.True(t, handled)
				require.Equal(t, 1, stageCalls, "wrapper must enter the exact shared staging seam once")
				require.Equal(t, scenario.wantStamina, user.Character.Stamina)
				if scenario.wantCooldown == 0 {
					require.Empty(t, user.Character.Cooldowns)
				} else {
					require.Equal(t, scenario.wantCooldown, user.Character.Cooldowns["special-move"])
				}

				assert.False(t, user.Character.IsInCombat(), "staged race must not commit aggro")
				assert.Equal(t, 0, specialMoveRoundsWaiting(user.Character), "staged race must not consume a combat round")
				assert.Empty(t, events.DrainQueuedPlayerAttackedMobsForTest(user.UserId))
				assert.Equal(t, 0, opinions.Get(int(target.MobId), user.UserId))
				assert.Empty(t, crimes.AllForFaction("thornwall_citizens", false))
				require.Equal(t, actorHealth, user.Character.Health)
				require.Len(t, user.Character.Conditions, actorConditions)
				require.Len(t, user.Character.Buffs.GetBuffs(), actorBuffs)
				require.Equal(t, targetHealth, target.Character.Health)
				require.Equal(t, targetStamina, target.Character.Stamina)
				require.Len(t, target.Character.Conditions, targetConditions)
				require.Len(t, target.Character.Buffs.GetBuffs(), targetBuffs)
				require.False(t, target.Character.IsInCombat())
				require.Equal(t, actorPosition, user.Character.Position.State())
				require.Equal(t, targetPosition, target.Character.Position.State())
			})
		}
	}
}

func specialMoveRoundsWaiting(char *characters.Character) int {
	if !char.IsInCombat() {
		return 0
	}
	return char.RoundsWaiting()
}
