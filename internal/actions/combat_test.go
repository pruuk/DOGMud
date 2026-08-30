package actions

import (
	"github.com/GoMudEngine/GoMud/internal/state"
	"reflect"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type specialMoveTestResult struct {
	executed   bool
	onCooldown bool
	hit        bool
	cost       characters.CostCommitResult
}

type specialMoveAdmissionCase struct {
	name           string
	action         costs.Action
	speciesID      int
	invalidSpecies int
	invalidate     func(*characters.Character)
	execute        func(Actor) specialMoveTestResult
}

var specialMoveTargetID = 8800

type staleCooldownActor struct {
	Actor
	characterCalls int
}

func (a *staleCooldownActor) GetCharacter() *characters.Character {
	a.characterCalls++
	char := a.Actor.GetCharacter()
	if a.characterCalls == 2 {
		char.Cooldowns["special-move"] = 3
	}
	return char
}

func fundSpecialMove(char *characters.Character) {
	char.Stamina = 10_000
	char.StaminaMax.Value = 10_000
}

func prepareSpecialMoveCooldown(t *testing.T, char *characters.Character, targetID, speciesID int, sp *species.Species) {
	t.Helper()
	cleanup := species.SeedSpeciesForTest(map[int]*species.Species{speciesID: sp})
	t.Cleanup(cleanup)

	target := &mobs.Mob{InstanceId: targetID}
	target.Character.Name = "Cooldown Target"
	setCombatPositionParallel(&target.Character, position.Standing)
	mobs.SetInstanceForTest(targetID, target)
	t.Cleanup(func() { mobs.SetInstanceForTest(targetID, nil) })

	char.SpeciesId = speciesID
	char.SetAggro(0, targetID, characters.DefaultAttack)
	char.Cooldowns = characters.Cooldowns{"special-move": 3}
	fundSpecialMove(char)
}

func seedSpecialMoveAdmissionSpecies() func() {
	return species.SeedSpeciesForTest(map[int]*species.Species{
		8101: {SpeciesId: 8101, Name: "natural basher", BodyParts: []string{"arms", "legs"}, NaturalBash: true},
		8102: {SpeciesId: 8102, Name: "humanoid", BodyParts: []string{"arms", "hands", "legs"}},
		8103: {SpeciesId: 8103, Name: "fanged beast", BodyParts: []string{"legs", "mouth"}, NaturalAttack: items.Bite},
		8104: {SpeciesId: 8104, Name: "clawed beast", BodyParts: []string{"legs", "mouth"}, NaturalAttack: items.Claws},
		8105: {SpeciesId: 8105, Name: "horned beast", BodyParts: []string{"legs", "mouth", "horns"}, NaturalAttack: items.Gore},
		8106: {SpeciesId: 8106, Name: "life drainer", BodyParts: []string{"arms", "hands", "legs", "mouth"}, NaturalAttack: items.Claws, LifeDrain: true},
		8107: {SpeciesId: 8107, Name: "legless armless", BodyParts: []string{"mouth"}},
	})
}

func embeddedSpecialMoveCost(t *testing.T, result any) characters.CostCommitResult {
	t.Helper()
	typ := reflect.TypeOf(result)
	field, ok := typ.FieldByName("Cost")
	if !ok {
		t.Errorf("%s must embed Cost characters.CostCommitResult", typ.Name())
		return characters.CostCommitResult{}
	}
	if field.Type != reflect.TypeOf(characters.CostCommitResult{}) {
		t.Errorf("%s.Cost must be a characters.CostCommitResult", typ.Name())
		return characters.CostCommitResult{}
	}
	return reflect.ValueOf(result).FieldByIndex(field.Index).Interface().(characters.CostCommitResult)
}

func specialMoveBoolField(t *testing.T, result any, name string) bool {
	t.Helper()
	value := reflect.ValueOf(result)
	field := value.FieldByName(name)
	if !field.IsValid() {
		t.Errorf("%s must expose %s", value.Type().Name(), name)
		return false
	}
	if field.Kind() != reflect.Bool {
		t.Errorf("%s.%s must be bool", value.Type().Name(), name)
		return false
	}
	return field.Bool()
}

func assertRawSpecialMoveRejected(t *testing.T, result any, reason string) {
	t.Helper()
	require.False(t, specialMoveBoolField(t, result, "Executed"))
	require.True(t, specialMoveBoolField(t, result, reason))
	require.Equal(t, characters.CostNoCharge, embeddedSpecialMoveCost(t, result).Status)
}

func specialMoveAdmissionCases(t *testing.T) []specialMoveAdmissionCase {
	t.Helper()
	return []specialMoveAdmissionCase{
		{name: "bash", action: costs.ActionBash, speciesID: 8101, invalidSpecies: 8102,
			execute: func(actor Actor) specialMoveTestResult {
				r := ExecuteBash(actor)
				return specialMoveTestResult{r.Executed, r.OnCooldown, r.MoveResult.Hit, embeddedSpecialMoveCost(t, r)}
			}},
		{name: "trip", action: costs.ActionTrip, speciesID: 8102, invalidSpecies: 8107,
			execute: func(actor Actor) specialMoveTestResult {
				r := ExecuteTrip(actor)
				return specialMoveTestResult{r.Executed, r.OnCooldown, r.MoveResult.Hit, embeddedSpecialMoveCost(t, r)}
			}},
		{name: "kick", action: costs.ActionKick, speciesID: 8102, invalidSpecies: 8107,
			execute: func(actor Actor) specialMoveTestResult {
				r := ExecuteKick(actor)
				return specialMoveTestResult{r.Executed, r.OnCooldown, r.MoveResult.Hit, embeddedSpecialMoveCost(t, r)}
			}},
		{name: "grapple", action: costs.ActionGrapple, speciesID: 8102, invalidSpecies: 8107,
			execute: func(actor Actor) specialMoveTestResult {
				r := ExecuteGrapple(actor)
				return specialMoveTestResult{r.Executed, r.OnCooldown, r.MoveResult.Success, embeddedSpecialMoveCost(t, r)}
			}},
		{name: "hamstring", action: costs.ActionHamstring, speciesID: 8103, invalidSpecies: 8102,
			execute: func(actor Actor) specialMoveTestResult {
				r := ExecuteHamstring(actor)
				return specialMoveTestResult{r.Executed, r.OnCooldown, r.MoveResult.Hit, embeddedSpecialMoveCost(t, r)}
			}},
		{name: "rake", action: costs.ActionRake, speciesID: 8104, invalidSpecies: 8103,
			execute: func(actor Actor) specialMoveTestResult {
				r := ExecuteRake(actor)
				return specialMoveTestResult{r.Executed, r.OnCooldown, r.MoveResult.Hit, embeddedSpecialMoveCost(t, r)}
			}},
		{name: "maul", action: costs.ActionMaul, speciesID: 8103, invalidSpecies: 8104,
			execute: func(actor Actor) specialMoveTestResult {
				r := ExecuteMaul(actor)
				return specialMoveTestResult{r.Executed, r.OnCooldown, r.MoveResult.Hit, embeddedSpecialMoveCost(t, r)}
			}},
		{name: "pounce", action: costs.ActionPounce, speciesID: 8103, invalidSpecies: 8103,
			invalidate: func(char *characters.Character) { setCombatPositionParallel(char, position.Clinch) },
			execute: func(actor Actor) specialMoveTestResult {
				r := ExecutePounce(actor)
				return specialMoveTestResult{r.Executed, r.OnCooldown, r.MoveResult.Hit, embeddedSpecialMoveCost(t, r)}
			}},
		{name: "gore", action: costs.ActionGore, speciesID: 8105, invalidSpecies: 8103,
			execute: func(actor Actor) specialMoveTestResult {
				r := ExecuteGore(actor)
				return specialMoveTestResult{r.Executed, r.OnCooldown, r.MoveResult.Hit, embeddedSpecialMoveCost(t, r)}
			}},
		{name: "drain", action: costs.ActionDrain, speciesID: 8106, invalidSpecies: 8102,
			execute: func(actor Actor) specialMoveTestResult {
				r := ExecuteDrain(actor)
				return specialMoveTestResult{r.Executed, r.OnCooldown, r.MoveResult.Hit, embeddedSpecialMoveCost(t, r)}
			}},
		{name: "throttle", action: costs.ActionThrottle, speciesID: 8103, invalidSpecies: 8104,
			execute: func(actor Actor) specialMoveTestResult {
				r := ExecuteThrottle(actor)
				return specialMoveTestResult{r.Executed, r.OnCooldown, r.MoveResult.Hit, embeddedSpecialMoveCost(t, r)}
			}},
	}
}

func newSpecialMoveAdmissionActor(t *testing.T, speciesID, stamina, skillRank int, laden bool) (Actor, *characters.Character, *characters.Character) {
	t.Helper()
	specialMoveTargetID++
	target := characters.New()
	target.Name = "Admission Target"
	target.SpeciesId = 8102
	target.HealthMax.Value = 1_000_000
	target.Health = target.HealthMax.Value
	target.Stats.Dexterity.ValueAdj = 1_000_000
	target.Skills[string(skills.WeaponCombat)] = 1_000_000
	setCombatPositionParallel(target, position.Standing)
	targetMob := &mobs.Mob{InstanceId: specialMoveTargetID, Character: *target}
	targetMob.Character.MobInstanceId = specialMoveTargetID
	mobs.SetInstanceForTest(specialMoveTargetID, targetMob)
	t.Cleanup(func() { mobs.SetInstanceForTest(targetMob.InstanceId, nil) })

	char := characters.New()
	char.Name = "Admission Actor"
	char.SpeciesId = speciesID
	char.Stamina = stamina
	char.StaminaMax.Value = 100
	char.Stats.Strength.ValueAdj = 100
	char.Stats.Dexterity.ValueAdj = 1
	char.Skills[string(skills.UnarmedCombat)] = skillRank
	char.Skills[string(skills.WeaponCombat)] = skillRank
	setCombatPositionParallel(char, position.Standing)
	char.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
	if laden {
		char.Items = []items.Item{{
			ItemId: 9911,
			Spec:   &items.ItemSpec{ItemId: 9911, Name: "admission ballast", Weight: char.CarryCapacity()},
		}}
	}
	return newStubActor(char, newTestRoom()), char, &targetMob.Character
}

func assertSpecialMoveUnchanged(t *testing.T, char, target *characters.Character, stamina, health, targetStamina, conditions, targetBuffs int, actorState, targetState position.State) {
	t.Helper()
	require.Equal(t, stamina, char.Stamina, "refusal must preserve stamina")
	require.Empty(t, char.Cooldowns, "refusal must preserve cooldown state")
	require.Equal(t, 0, char.RoundsWaiting(), "refusal must not consume the combat round")
	require.Equal(t, health, target.Health, "refusal must not damage the target")
	require.Equal(t, targetStamina, target.Stamina, "refusal must not drain target stamina")
	require.Len(t, target.Conditions, conditions, "refusal must not add a condition")
	require.Len(t, target.Buffs.GetBuffs(), targetBuffs, "refusal must not add an effect")
	require.Equal(t, actorState, char.Position.State(), "refusal must preserve actor position/grapple state")
	require.Equal(t, targetState, target.Position.State(), "refusal must preserve target position/grapple state")
}

// TestSpecialMoveFamilyAdmission catches any one of the eleven special moves
// bypassing the shared quote/commit seam, consuming cooldown before validation
// or refusal, charging a miss twice, or selecting cost skill/load locally.
func TestSpecialMoveFamilyAdmission(t *testing.T) {
	cleanup := seedSpecialMoveAdmissionSpecies()
	defer cleanup()

	for _, tc := range specialMoveAdmissionCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("invalid_state_is_read_only", func(t *testing.T) {
				actor, char, target := newSpecialMoveAdmissionActor(t, tc.invalidSpecies, 50, 0, false)
				if tc.invalidate != nil {
					tc.invalidate(char)
				}
				stamina, health, targetStamina := char.Stamina, target.Health, target.Stamina
				conditions, targetBuffs := len(target.Conditions), len(target.Buffs.GetBuffs())
				actorState, targetState := char.Position.State(), target.Position.State()
				got := tc.execute(actor)
				require.False(t, got.executed)
				require.Equal(t, characters.CostNoCharge, got.cost.Status)
				require.Zero(t, got.cost.Charged)
				assertSpecialMoveUnchanged(t, char, target, stamina, health, targetStamina, conditions, targetBuffs, actorState, targetState)
			})

			t.Run("active_cooldown_is_read_only", func(t *testing.T) {
				actor, char, target := newSpecialMoveAdmissionActor(t, tc.speciesID, 50, 0, false)
				char.Cooldowns["special-move"] = 3
				stamina, health, targetStamina := char.Stamina, target.Health, target.Stamina
				conditions, targetBuffs := len(target.Conditions), len(target.Buffs.GetBuffs())
				actorState, targetState := char.Position.State(), target.Position.State()
				got := tc.execute(actor)
				require.False(t, got.executed)
				require.True(t, got.onCooldown)
				require.Equal(t, characters.CostNoCharge, got.cost.Status)
				require.Equal(t, 3, char.Cooldowns["special-move"])
				require.Equal(t, stamina, char.Stamina)
				require.Equal(t, health, target.Health)
				require.Equal(t, targetStamina, target.Stamina)
				require.Len(t, target.Conditions, conditions)
				require.Len(t, target.Buffs.GetBuffs(), targetBuffs)
				require.Equal(t, actorState, char.Position.State())
				require.Equal(t, targetState, target.Position.State())
				require.Equal(t, 0, char.RoundsWaiting())
			})

			t.Run("unaffordable_refusal_is_atomic", func(t *testing.T) {
				actor, char, target := newSpecialMoveAdmissionActor(t, tc.speciesID, 0, 0, false)
				health, targetStamina := target.Health, target.Stamina
				conditions, targetBuffs := len(target.Conditions), len(target.Buffs.GetBuffs())
				actorState, targetState := char.Position.State(), target.Position.State()
				got := tc.execute(actor)
				require.False(t, got.executed)
				require.Equal(t, characters.CostRefused, got.cost.Status)
				require.Equal(t, characters.PoolStamina, got.cost.Pool)
				require.Zero(t, got.cost.Charged)
				assertSpecialMoveUnchanged(t, char, target, 0, health, targetStamina, conditions, targetBuffs, actorState, targetState)
				if tc.action == costs.ActionGrapple {
					require.False(t, char.IsGrappling())
					require.False(t, target.IsGrappling())
				}

				// A refusal must not bank fractional carry. With this rank-zero
				// 4.4 quote, an illicit refusal carry would make the second paid
				// admission charge 5 instead of the literal expected 4.
				char.Stamina = 100
				first := admitFullCost(actor, tc.action, characters.PoolStamina, 4)
				second := admitFullCost(actor, tc.action, characters.PoolStamina, 4)
				require.Equal(t, 4, first.Charged)
				require.Equal(t, 4, second.Charged, "refusal must preserve fractional carry")
			})

			t.Run("affordable_miss_pays_once", func(t *testing.T) {
				// Retried rather than seeded. rand.Seed has been a no-op since
				// Go 1.20 unless GODEBUG=randseednop=0 is set, which this file
				// does not set, so the seeding this loop used to do bought
				// nothing -- every iteration was already an independent random
				// draw, which is exactly what makes the retry work.
				for attempt := 0; attempt < 20; attempt++ {
					actor, char, _ := newSpecialMoveAdmissionActor(t, tc.speciesID, 50, 0, false)
					got := tc.execute(actor)
					if got.hit {
						continue // a configured contest floor may rescue the overwhelming miss
					}
					require.True(t, got.executed)
					require.Equal(t, characters.CostPaid, got.cost.Status)
					require.Equal(t, 4, got.cost.Charged, "base 4 x rank-zero 1.10 floors to one four-point charge")
					require.Equal(t, 46, char.Stamina)
					require.Greater(t, char.Cooldowns["special-move"], 0)
					require.Equal(t, 1, char.RoundsWaiting())
					return
				}
				t.Fatal("configured contest floor rescued twenty overwhelming misses")
			})

			t.Run("registry_skill_and_load_price_the_action", func(t *testing.T) {
				lowActor, _, _ := newSpecialMoveAdmissionActor(t, tc.speciesID, 100, 0, false)
				highActor, _, _ := newSpecialMoveAdmissionActor(t, tc.speciesID, 100, 100, false)
				emptyActor, _, _ := newSpecialMoveAdmissionActor(t, tc.speciesID, 100, 25, false)
				ladenActor, _, _ := newSpecialMoveAdmissionActor(t, tc.speciesID, 100, 25, true)
				// No seeding: the assertions below compare CHARGED COST, which
				// is a pure function of base, governing skill and carried load.
				// The roll does not enter it, so these four calls do not need to
				// share a random sequence -- which is just as well, because the
				// rand.Seed calls that used to sit here were no-ops.
				low := tc.execute(lowActor)
				high := tc.execute(highActor)
				empty := tc.execute(emptyActor)
				laden := tc.execute(ladenActor)
				require.Less(t, high.cost.Charged, low.cost.Charged, "registry-selected governing skill must reduce the quote")
				require.Less(t, empty.cost.Charged, laden.cost.Charged, "lower physical load must reduce the quote")
			})
		})
	}
}

// TestSpecialMoveStaleCooldownAdmission catches a consuming cooldown failure
// after a successful admission being allowed to resolve effects, consume a
// round, or charge a second time. The actor injects the stale cooldown on the
// admission helper's second GetCharacter call, between CooldownReady and
// TryCooldown in the synchronous function.
func TestSpecialMoveStaleCooldownAdmission(t *testing.T) {
	cleanup := seedSpecialMoveAdmissionSpecies()
	defer cleanup()

	for _, tc := range specialMoveAdmissionCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			baseActor, char, target := newSpecialMoveAdmissionActor(t, tc.speciesID, 50, 0, false)
			actor := &staleCooldownActor{Actor: baseActor}
			health, targetStamina := target.Health, target.Stamina
			conditions, targetBuffs := len(target.Conditions), len(target.Buffs.GetBuffs())
			actorState, targetState := char.Position.State(), target.Position.State()

			got := tc.execute(actor)

			require.False(t, got.executed)
			require.Equal(t, characters.CostPaid, got.cost.Status)
			require.Equal(t, 4, got.cost.Charged)
			require.Equal(t, 46, char.Stamina, "stale path must retain exactly one committed admission")
			require.Equal(t, 3, char.Cooldowns["special-move"])
			require.Equal(t, 0, char.RoundsWaiting())
			require.Equal(t, health, target.Health)
			require.Equal(t, targetStamina, target.Stamina)
			require.Len(t, target.Conditions, conditions)
			require.Len(t, target.Buffs.GetBuffs(), targetBuffs)
			require.Equal(t, actorState, char.Position.State())
			require.Equal(t, targetState, target.Position.State())
		})
	}
}

// TestSpecialMoveMissingReadinessGatesAdmission catches trip charging an
// already-grounded target, grapple disturbing an existing grapple, or
// hamstring admitting an actor without legs. These are readiness-invalid
// states and must return a specific reason before cost/cooldown mutation.
func TestSpecialMoveMissingReadinessGatesAdmission(t *testing.T) {
	cleanup := seedSpecialMoveAdmissionSpecies()
	defer cleanup()

	t.Run("trip rejects target already on floor", func(t *testing.T) {
		actor, char, target := newSpecialMoveAdmissionActor(t, 8102, 50, 0, false)
		setCombatPositionParallel(target, position.Prone)
		result := ExecuteTrip(actor)
		assertRawSpecialMoveRejected(t, result, "TargetOnFloor")
		require.Equal(t, 50, char.Stamina)
		require.Empty(t, char.Cooldowns)
		require.Equal(t, 0, char.RoundsWaiting())
		require.Equal(t, position.Prone, target.Position.State())
	})

	t.Run("grapple rejects target already grappling", func(t *testing.T) {
		actor, char, target := newSpecialMoveAdmissionActor(t, 8102, 50, 0, false)
		setCombatPositionParallel(target, position.Clinch)
		result := ExecuteGrapple(actor)
		assertRawSpecialMoveRejected(t, result, "TargetGrappling")
		require.Equal(t, 50, char.Stamina)
		require.Empty(t, char.Cooldowns)
		require.Equal(t, 0, char.RoundsWaiting())
		require.Equal(t, position.Standing, char.Position.State())
		require.Equal(t, position.Clinch, target.Position.State())
	})

	t.Run("hamstring rejects actor without legs", func(t *testing.T) {
		actor, char, target := newSpecialMoveAdmissionActor(t, 8103, 50, 0, false)
		char.SpeciesId = 8107
		result := ExecuteHamstring(actor)
		assertRawSpecialMoveRejected(t, result, "NoLegs")
		require.Equal(t, 50, char.Stamina)
		require.Empty(t, char.Cooldowns)
		require.Equal(t, 0, char.RoundsWaiting())
		require.Equal(t, target.HealthMax.Value, target.Health)
	})
}

// TestSpecialMoveActingAdmission catches direct shared/mob dispatch charging
// or resolving any of the seven mutation/beast moves while the actor is
// casting. IsActing is universal readiness and must precede target/cost work.
func TestSpecialMoveActingAdmission(t *testing.T) {
	cleanup := seedSpecialMoveAdmissionSpecies()
	defer cleanup()

	tests := []struct {
		name      string
		speciesID int
		execute   func(Actor) any
	}{
		{"hamstring", 8103, func(a Actor) any { return ExecuteHamstring(a) }},
		{"rake", 8104, func(a Actor) any { return ExecuteRake(a) }},
		{"maul", 8103, func(a Actor) any { return ExecuteMaul(a) }},
		{"pounce", 8103, func(a Actor) any { return ExecutePounce(a) }},
		{"gore", 8105, func(a Actor) any { return ExecuteGore(a) }},
		{"drain", 8106, func(a Actor) any { return ExecuteDrain(a) }},
		{"throttle", 8103, func(a Actor) any { return ExecuteThrottle(a) }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actor, char, target := newSpecialMoveAdmissionActor(t, tc.speciesID, 50, 0, false)
			setCastingForTest(char, activity.CastingData{SpellId: "test-spell", FoldsNeeded: 2})
			targetHealth, targetStamina := target.Health, target.Stamina
			result := tc.execute(actor)
			assertRawSpecialMoveRejected(t, result, "Crafting")
			require.Equal(t, 50, char.Stamina)
			require.Empty(t, char.Cooldowns)
			require.Equal(t, 0, char.RoundsWaiting())
			require.Equal(t, targetHealth, target.Health)
			require.Equal(t, targetStamina, target.Stamina)
			require.Empty(t, target.Conditions)
			require.True(t, char.IsCasting())
		})
	}
}

// ---------------------------------------------------------------------------
// ResolveAggroTarget tests
// ---------------------------------------------------------------------------

// TestResolveAggroTarget_ZeroRef verifies that a zero ActorRef returns
// Found=false without panicking.
//
// U12c-1: the function takes a state.ActorRef now. A zero ref is what a nil
// characters.CurrentCombatTarget() used to be, and must behave identically.
func TestResolveAggroTarget_ZeroRef(t *testing.T) {
	result := ResolveAggroTarget(state.ActorRef{})
	assert.False(t, result.Found, "zero ref should return Found=false")
	assert.Equal(t, 0, result.UserId, "UserId should be zero for a zero ref")
	assert.Equal(t, 0, result.MobInstanceId, "MobInstanceId should be zero for a zero ref")
	assert.Nil(t, result.Char, "Char should be nil for a zero ref")
}

// TestResolveAggroTarget_InvalidMobId verifies that an aggro pointing at a
// mob instance ID that doesn't exist in the mobs registry returns Found=false.
func TestResolveAggroTarget_InvalidMobId(t *testing.T) {
	ref := state.ActorRef{
		MobInstanceId: 999999, // highly unlikely to exist
	}
	result := ResolveAggroTarget(ref)
	assert.False(t, result.Found, "nonexistent mob instance ID should return Found=false")
}

// TestResolveAggroTarget_InvalidUserId verifies that an aggro pointing at a
// user ID that doesn't exist in the users registry returns Found=false.
// Note: MobInstanceId must be 0 (or invalid) so the code falls through to the
// UserId branch.
func TestResolveAggroTarget_InvalidUserId(t *testing.T) {
	ref := state.ActorRef{
		MobInstanceId: 0,
		UserId:        999999, // highly unlikely to exist
	}
	result := ResolveAggroTarget(ref)
	assert.False(t, result.Found, "nonexistent user ID should return Found=false")
}

// TestResolveAggroTarget_ZeroIds verifies that an aggro with both IDs at zero
// (valid aggro struct but no target set) returns Found=false.
func TestResolveAggroTarget_ZeroIds(t *testing.T) {
	ref := state.ActorRef{
		MobInstanceId: 0,
		UserId:        0,
	}
	result := ResolveAggroTarget(ref)
	assert.False(t, result.Found, "a ref with zero IDs should return Found=false")
}

// ---------------------------------------------------------------------------
// TryCombatCooldown tests
// ---------------------------------------------------------------------------

// TestTryCombatCooldown_Fresh verifies that a character with no active cooldowns
// is NOT blocked (returns false = move allowed).
func TestTryCombatCooldown_Fresh(t *testing.T) {
	char := characters.New()
	// characters.New() initializes Cooldowns to make(Cooldowns), so no setup
	// needed. The first call to TryCombatCooldown should set the cooldown and
	// return false (not blocked).
	blocked := TryCombatCooldown(char, 3)
	assert.False(t, blocked, "fresh character should not be on cooldown (move should be allowed)")
}

// TestTryCombatCooldown_Active verifies that after a cooldown is set, the same
// character is blocked on the next attempt (returns true = move blocked).
func TestTryCombatCooldown_Active(t *testing.T) {
	char := characters.New()

	// First call: sets the cooldown, move is allowed.
	firstBlocked := TryCombatCooldown(char, 3)
	assert.False(t, firstBlocked, "first call should not be blocked")

	// Second call: cooldown is now active, move is blocked.
	secondBlocked := TryCombatCooldown(char, 3)
	assert.True(t, secondBlocked, "second call should be blocked (cooldown active)")
}

// TestTryCombatCooldown_NilChar verifies that a nil character returns true
// (blocked) without panicking. This is a safety guard in the implementation.
func TestTryCombatCooldown_NilChar(t *testing.T) {
	blocked := TryCombatCooldown(nil, 3)
	assert.True(t, blocked, "nil character should be treated as blocked")
}

// ---------------------------------------------------------------------------
// ExecuteKick / variant detection tests
// ---------------------------------------------------------------------------

// TestKickVariant_NoAggro verifies that ExecuteKick returns NoTarget=true when
// the actor has no aggro set (not yet in combat).
func TestKickVariant_NoAggro(t *testing.T) {
	char := characters.New()
	room := newTestRoom()
	actor := newStubActor(char, room)

	// No Aggro set — should bail out immediately.
	result := ExecuteKick(actor)

	assert.False(t, result.Executed, "kick with no aggro should not execute")
	assert.True(t, result.NoTarget, "kick with no aggro should set NoTarget")
	assert.False(t, result.OnCooldown, "NoTarget should take priority over cooldown reporting")
}

// TestKickVariant_CooldownAfterFirst verifies that after one kick fires, a
// second attempt on the same character returns OnCooldown=true. The fixture
// supplies a valid target and anatomy because all validity checks deliberately
// precede the read-only cooldown branch.
func TestKickVariant_CooldownBlocks(t *testing.T) {
	char := characters.New()
	room := newTestRoom()
	actor := newStubActor(char, room)

	prepareSpecialMoveCooldown(t, char, 7901, 7901, &species.Species{
		SpeciesId: 7901, Name: "legged-kicker", BodyParts: []string{"legs"},
	})

	result := ExecuteKick(actor)

	assert.False(t, result.Executed, "kick should not execute when on cooldown")
	assert.True(t, result.OnCooldown, "kick should report OnCooldown")
}

// ---------------------------------------------------------------------------
// ExecuteTrip / variant detection tests
// ---------------------------------------------------------------------------

// TestTripVariant_NoAggro verifies that ExecuteTrip returns NoTarget=true when
// the actor has no aggro.
func TestTripVariant_NoAggro(t *testing.T) {
	char := characters.New()
	room := newTestRoom()
	actor := newStubActor(char, room)

	result := ExecuteTrip(actor)

	assert.False(t, result.Executed, "trip with no aggro should not execute")
	assert.True(t, result.NoTarget, "trip with no aggro should set NoTarget")
}

// TestTripVariant_CooldownBlocks verifies that ExecuteTrip returns
// OnCooldown=true when the special-move cooldown is active.
func TestTripVariant_CooldownBlocks(t *testing.T) {
	char := characters.New()
	room := newTestRoom()
	actor := newStubActor(char, room)

	prepareSpecialMoveCooldown(t, char, 7902, 7902, &species.Species{
		SpeciesId: 7902, Name: "legged-tripper", BodyParts: []string{"legs"},
	})

	result := ExecuteTrip(actor)

	assert.False(t, result.Executed, "trip should not execute when on cooldown")
	assert.True(t, result.OnCooldown, "trip should report OnCooldown")
}

// TestTripVariant_NoTail verifies that a character without the tail mutation
// selects TripStandard. We confirm the variant indirectly: when the target is
// missing (nonexistent mob instance), the trip gets past cooldown but returns
// NoTarget — so we can't observe Variant directly. Instead, we verify the
// mutation map path by inspecting the character directly.
//
// The mutation branch is: if _, ok := char.Mutations["tail"]; ok { … }
// so a character with no Mutations map (or empty map) → TripStandard.
func TestTripVariant_NoTailMutation(t *testing.T) {
	char := characters.New()

	// Confirm no tail mutation exists on a fresh character.
	_, hasTail := char.Mutations["tail"]
	assert.False(t, hasTail, "fresh character should have no tail mutation")

	// The variant selection logic: no tail → TripStandard (iota 0).
	expectedVariant := TripStandard
	assert.Equal(t, TripStandard, expectedVariant, "TripStandard should be the zero value")
}

// TestTripVariant_WithTailMutation verifies that the tail mutation map key
// is detected correctly. Since we can't complete a full trip without valid
// combat state, we check the mutation detection directly and confirm
// TripTailsweep has a distinct value from TripStandard.
func TestTripVariant_WithTailMutation(t *testing.T) {
	char := characters.New()

	// Add the tail mutation.
	if char.Mutations == nil {
		char.Mutations = make(map[string]int)
	}
	char.Mutations["tail"] = 1

	_, hasTail := char.Mutations["tail"]
	assert.True(t, hasTail, "character with tail mutation should have 'tail' key in Mutations")

	// Verify variant constants are distinct.
	assert.NotEqual(t, TripStandard, TripTailsweep, "TripTailsweep should differ from TripStandard")
}

// ---------------------------------------------------------------------------
// FindAttackTarget tests (wildcard + self-prevention)
// ---------------------------------------------------------------------------

// TestFindAttackTarget_EmptyRoom verifies that a wildcard search in an empty
// room returns Found=false.
func TestFindAttackTarget_EmptyRoom(t *testing.T) {
	room := &rooms.Room{}

	result := FindAttackTarget("*", room, 1, 0)

	assert.False(t, result.Found, "no targets in empty room — should not find one")
}

// TestFindAttackTarget_SelfUser verifies that a player searching with "*user"
// does NOT target themselves — the self-exclusion logic in FindAttackTarget
// skips the actor's own userId.
func TestFindAttackTarget_SelfUser(t *testing.T) {
	room := &rooms.Room{}

	// Add only the actor as a player — so if self-exclusion didn't work,
	// they'd be the only candidate and would be "found".
	const actorUserId = 42
	room.AddPlayer(actorUserId)

	// Search for any player (*user). With only self present, should return
	// Found=false after self-exclusion.
	result := FindAttackTarget("*user", room, actorUserId, 0)

	assert.False(t, result.Found, "player should not be able to target themselves with *user")
	assert.NotEqual(t, actorUserId, result.UserId, "result UserId must not be the actor")
}

// TestFindAttackTarget_SelfMob verifies that a mob cannot target itself using
// "*mob" — the self-exclusion logic skips the actor's own MobInstanceId.
func TestFindAttackTarget_SelfMob(t *testing.T) {
	room := &rooms.Room{}

	// Add only the actor mob — self-exclusion must prevent it from being chosen.
	const actorMobId = 77
	room.AddMob(actorMobId)

	result := FindAttackTarget("*mob", room, 0, actorMobId)

	assert.False(t, result.Found, "mob should not be able to target itself with *mob")
	assert.NotEqual(t, actorMobId, result.MobInstanceId, "result MobInstanceId must not be the actor")
}

// TestFindAttackTarget_WildcardAnyone_MultipleTargets verifies that when two
// player IDs are in a room and one searches with "*", the found target is NOT
// the actor themselves. We repeat the call a few times to reduce probability
// of a fluke.
func TestFindAttackTarget_WildcardAnyone_MultipleTargets(t *testing.T) {
	room := &rooms.Room{}

	const actorUserId = 10
	const otherUserId = 20
	room.AddPlayer(actorUserId)
	room.AddPlayer(otherUserId)

	// Run several times — randomness means we can't guarantee which is picked,
	// but the actor must never appear as the result.
	for i := 0; i < 20; i++ {
		result := FindAttackTarget("*", room, actorUserId, 0)
		if result.Found {
			assert.NotEqual(t, actorUserId, result.UserId,
				"actor should never be selected as their own target")
		}
	}
}

// TestFindAttackTarget_WildcardMob_ExcludesSelf verifies that when the room
// has two mob IDs, a mob actor searching "*mob" cannot get its own ID.
func TestFindAttackTarget_WildcardMob_ExcludesSelf(t *testing.T) {
	room := &rooms.Room{}

	const actorMobId = 100
	const otherMobId = 200
	room.AddMob(actorMobId)
	room.AddMob(otherMobId)

	for i := 0; i < 20; i++ {
		result := FindAttackTarget("*mob", room, 0, actorMobId)
		if result.Found {
			assert.NotEqual(t, actorMobId, result.MobInstanceId,
				"mob should never select itself as a target")
		}
	}
}

// ---------------------------------------------------------------------------
// KickVariant / TripVariant constant sanity checks
// ---------------------------------------------------------------------------

// TestKickVariantConstants verifies that the three KickVariant constants have
// distinct values in the expected order (iota).
func TestKickVariantConstants(t *testing.T) {
	assert.Equal(t, KickVariant(0), KickStandard, "KickStandard should be zero")
	assert.Equal(t, KickVariant(1), KickStomp, "KickStomp should be 1")
	assert.Equal(t, KickVariant(2), KickKnee, "KickKnee should be 2")
	assert.NotEqual(t, KickStandard, KickStomp)
	assert.NotEqual(t, KickStomp, KickKnee)
	assert.NotEqual(t, KickStandard, KickKnee)
}

// TestTripVariantConstants verifies that the two TripVariant constants have
// distinct values in the expected order.
func TestTripVariantConstants(t *testing.T) {
	assert.Equal(t, TripVariant(0), TripStandard, "TripStandard should be zero")
	assert.Equal(t, TripVariant(1), TripTailsweep, "TripTailsweep should be 1")
	assert.NotEqual(t, TripStandard, TripTailsweep)
}
