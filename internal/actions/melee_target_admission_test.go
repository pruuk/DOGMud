package actions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/crimes"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/factions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/opinions"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func configureStagedAdmissionFaction(t *testing.T) {
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

func stagedAdmissionFixture(t *testing.T, speciesID int) (*users.UserRecord, *rooms.Room, *mobs.Mob, Actor) {
	t.Helper()
	cleanup := seedTargetResolutionRegistries(t)
	t.Cleanup(cleanup)
	configureStagedAdmissionFaction(t)

	user := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	target := mobs.GetInstance(100)
	require.NotNil(t, user)
	require.NotNil(t, room)
	require.NotNil(t, target)

	user.Character = characters.New()
	user.Character.Name = "Alice"
	user.Character.RoomId = 1
	user.Character.SpeciesId = speciesID
	user.Character.Stamina = 100
	user.Character.StaminaMax.Value = 100
	user.Character.Stats.Strength.ValueAdj = 100
	user.Character.Stats.Dexterity.ValueAdj = 100
	setCombatPositionParallel(user.Character, position.Standing)

	target.Groups = []string{"thornwall_citizens"}
	target.Character.Health = 1_000_000
	target.Character.HealthMax.Value = 1_000_000
	target.Character.Stamina = 1_000_000
	target.Character.StaminaMax.Value = 1_000_000
	target.Character.Buffs = buffs.New()
	setCombatPositionParallel(&target.Character, position.Standing)

	events.DrainQueuedPlayerAttackedMobsForTest(0)
	actor, handled := StageMeleeTarget(user, room, "#100", MeleeTargetOpts{Verb: "test"})
	require.False(t, handled)
	staged, ok := actor.(*stagedMeleeActor)
	require.True(t, ok)
	staged.Actor = &staleCooldownActor{Actor: staged.Actor}
	return user, room, target, staged
}

// TestStagedSpecialMoveStaleCooldownDoesNotCommitEngagement catches any
// physical wrapper/action pair committing staged aggression before the
// consuming cooldown succeeds. Admission is already paid on this race path,
// but engagement, resolution, effects, and round consumption remain forbidden.
func TestStagedSpecialMoveStaleCooldownDoesNotCommitEngagement(t *testing.T) {
	speciesCleanup := seedSpecialMoveAdmissionSpecies()
	defer speciesCleanup()

	for _, tc := range specialMoveAdmissionCases(t) {
		if tc.name == "hamstring" { // mob-only: no staged player wrapper
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			user, _, target, actor := stagedAdmissionFixture(t, tc.speciesID)
			healthBefore := target.Character.Health
			staminaBefore := target.Character.Stamina
			conditionsBefore := len(target.Character.Conditions)
			buffsBefore := len(target.Character.Buffs.GetBuffs())
			actorPosition := user.Character.Position.State()
			targetPosition := target.Character.Position.State()

			got := tc.execute(actor)

			require.False(t, got.executed)
			require.True(t, got.onCooldown)
			require.Equal(t, characters.CostPaid, got.cost.Status)
			require.Equal(t, 4, got.cost.Charged)
			require.Equal(t, 96, user.Character.Stamina)
			require.Equal(t, 3, user.Character.Cooldowns["special-move"])
			assert.Nil(t, user.Character.Aggro, "stale cooldown must not commit staged aggro or a round")
			assert.Empty(t, events.DrainQueuedPlayerAttackedMobsForTest(user.UserId))
			assert.Equal(t, 0, opinions.Get(int(target.MobId), user.UserId))
			assert.Empty(t, crimes.AllForFaction("thornwall_citizens", false))
			require.Equal(t, healthBefore, target.Character.Health)
			require.Equal(t, staminaBefore, target.Character.Stamina)
			require.Len(t, target.Character.Conditions, conditionsBefore)
			require.Len(t, target.Character.Buffs.GetBuffs(), buffsBefore)
			require.Equal(t, actorPosition, user.Character.Position.State())
			require.Equal(t, targetPosition, target.Character.Position.State())
		})
	}
}
