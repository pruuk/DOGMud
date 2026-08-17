package usercommands

import (
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/crimes"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/factions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/opinions"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type specialMoveCommand func(string, *users.UserRecord, *rooms.Room, events.EventFlag) (bool, error)

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
	target.Character.Aggro = nil
	events.DrainQueuedMessagesForTest(user.UserId)
	events.DrainQueuedPlayerAttackedMobsForTest(0)
}

func assertNoSpecialMoveEngagement(t *testing.T, user *users.UserRecord, target *mobs.Mob) {
	t.Helper()
	assert.Nil(t, user.Character.Aggro, "refused action must not set aggro")
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
			require.Contains(t, strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n"),
				"You are too spent to manage that right now.")
			assertNoSpecialMoveEngagement(t, user, target)
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
	rand.Seed(1)
	res := actions.ExecuteTrip(actor)

	require.True(t, res.Executed)
	require.False(t, res.MoveResult.Hit, "fixture must exercise the paid-miss path")
	require.Less(t, user.Character.Stamina, 100)
	require.NotNil(t, user.Character.Aggro)
	require.Equal(t, 100, user.Character.Aggro.MobInstanceId)
	require.Len(t, events.DrainQueuedPlayerAttackedMobsForTest(user.UserId), 1)
	require.Equal(t, -15, opinions.Get(int(target.MobId), user.UserId))
	require.Len(t, crimes.AllForFaction("thornwall_citizens", false), 1)
}

func TestStagedSpecialMoveTargetGoneBeforeAdmissionHasNoSideEffects(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	speciesCleanup := seedSpecialMoveSpecies()
	defer speciesCleanup()

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
	require.Nil(t, user.Character.Aggro)
	require.Empty(t, events.DrainQueuedPlayerAttackedMobsForTest(user.UserId))
}
