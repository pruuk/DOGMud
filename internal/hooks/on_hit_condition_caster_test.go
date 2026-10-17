package hooks

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const onHitCasterConditionId = 7114

// An on-hit mutation condition names the attacker who landed the hit. Evil
// Eye's 114 Cursed landed as "A cold, watching malice settles over you." with
// no source (2026-10-17 playtest): applyCombatDamageBonuses queued it with no
// caster, so a start line authoring {actor} would have read "something". Owner
// ruling 2026-10-10: a harmful condition's target and room start lines name
// the caster.
//
// Drives the real producer (applyCombatDamageBonuses) and the real listener
// (ApplyConditions).
func TestOnHitMutationCondition_NamesTheAttacker(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		onHitCasterConditionId: {ConditionId: onHitCasterConditionId, Name: "Test Curse",
			RoundInterval: 1, TriggerCount: 3,
			StartUserText: "{actor}'s gaze settles on you."},
	}))
	t.Cleanup(mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"test-evil-eye": {MutationId: "test-evil-eye", Name: "Test Evil Eye",
			Pros: []mutations.MutationEffect{{Type: "on_hit_condition", Value: onHitCasterConditionId}}},
	}))
	drainEventQueueOnCleanup(t)

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	user := users.GetByUserId(1)
	require.NotNil(t, user)
	room := rooms.LoadRoom(1)
	require.NotNil(t, room)
	// Fully lit, so the holder can make out the attacker's name.
	room.Lamp = rooms.LampPtr(90)
	mob.Character.Mutations = map[string]int{"test-evil-eye": 1}
	t.Cleanup(func() { mob.Character.Mutations = nil })

	events.DrainQueuedConditionsForTest(0)
	drainPlain(1)

	res := &combat.AttackResult{Hit: true, DamageToTarget: 1}
	applyCombatDamageBonuses(&actions.MobActor{Mob: mob, Room: room}, &actions.UserActor{User: user, Room: room}, res)

	queued := events.DrainQueuedConditionsForTest(user.UserId)
	require.Len(t, queued, 1, "the on-hit condition must be queued on the struck player")
	require.Equal(t, mob.InstanceId, queued[0].Caster.MobInstanceId,
		"the on-hit condition must carry the attacker as its caster")

	require.Equal(t, events.Continue, ApplyConditions(queued[0]))
	lines := drainPlain(1)
	require.Equal(t, 1, countContaining(lines, mob.Character.Name+"'s gaze settles on you."),
		"the holder's start line must name the attacker: %v", lines)
}

// The shipped Cursed record authors {actor} in both start lines, so the
// caster the engine now carries actually reaches the holder and the room.
func TestCursedStartLinesNameTheCaster(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "..", "..",
		"_datafiles", "world", "dogmud", "conditions", "114-cursed.yaml"))
	require.NoError(t, err)
	var spec struct {
		StartActee    string `yaml:"start_actee"`
		StartObserver string `yaml:"start_observer"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &spec))
	require.True(t, strings.Contains(spec.StartActee, "{actor}"), "start_actee must name the caster: %q", spec.StartActee)
	require.True(t, strings.Contains(spec.StartObserver, "{actor}"), "start_observer must name the caster: %q", spec.StartObserver)
}
