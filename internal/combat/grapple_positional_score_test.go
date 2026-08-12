package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/control"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Chunk 5.11c — positional modifiers move from the crit threshold onto the
// attack score.
//
// Prone already worked this way (ProneAttackMultiplier /
// ProneVulnerabilityMultiplier in calcAttackScore). Grapple did not: it
// subtracted from calcCritThreshold instead. Same category of effect, two
// mechanisms, two files.
//
// Moving grapple onto the score also fixes a latent self-cancellation. Both
// participants of a ground grapple satisfy IsGroundGrapple() -- it is a
// POSITION state, while IsController() is a separate CONTROL fsm. So the old
// code did:
//
//	source: IsController && IsGroundGrapple      -> critThreshold -= 0.4
//	target: IsGroundGrapple && !IsController     -> critThreshold += 0.4
//	                                                net ZERO
//
// The ground-grapple crit bonus cancelled itself out in exactly the situation
// it was written for. Standing grapple escaped it (-0.2 net) only because the
// target is not ground-grappled there. As attack-score multipliers the two
// effects COMPOUND instead, which is plainly the intent.
// ---------------------------------------------------------------------------

// newGrappleChar builds a character with every input to calcAttackScore pinned,
// so the ONLY thing varying between cases is grapple position.
//
// characters.New() RANDOMISES stats. An earlier version of this file omitted
// the pinning and the tests passed or failed depending on the roll -- three
// separate constructions produced effective dexterity 103, 84 and 114. A
// positional test that compares two randomly-statted characters measures
// nothing.
func newGrappleChar(uid int) *characters.Character {
	ch := characters.New()
	ch.SetUserId(uid)
	ch.Stats.Dexterity.Value = 100
	ch.Stats.Dexterity.ValueAdj = 100
	ch.Stamina = 100
	ch.StaminaMax.Value = 100
	return ch
}

// clinch puts a and b into a standing grapple with a controlling.
func clinch(t *testing.T, a, b *characters.Character) {
	t.Helper()
	assert.NoError(t, a.Position.TransitionToClinch(
		position.GrappleData{Partner: state.ActorRef{UserId: b.GetUserId()}},
		state.TransitionReason{Trigger: position.TriggerGrappleEntry}))
	assert.NoError(t, b.Position.TransitionToClinch(
		position.GrappleData{Partner: state.ActorRef{UserId: a.GetUserId()}},
		state.TransitionReason{Trigger: position.TriggerGrappleEntry}))
	assert.NoError(t, a.Control.TransitionToControlling(
		state.TransitionReason{Trigger: control.TriggerDriftWin}))
	assert.NoError(t, b.Control.TransitionToControlled(
		state.TransitionReason{Trigger: control.TriggerDriftLoss}))
}

// mount puts a and b into a ground grapple with a controlling (a mounted).
func mount(t *testing.T, a, b *characters.Character) {
	t.Helper()
	clinch(t, a, b)
	assert.NoError(t, a.Position.TransitionToMount(
		position.GrappleData{Partner: state.ActorRef{UserId: b.GetUserId()}},
		state.TransitionReason{Trigger: position.TriggerGrappleEntry}))
	assert.NoError(t, b.Position.TransitionToMount(
		position.GrappleData{Partner: state.ActorRef{UserId: a.GetUserId()}},
		state.TransitionReason{Trigger: position.TriggerGrappleEntry}))
}

func TestGrapple_GroundControllerGainsAttackScore(t *testing.T) {
	ctx := combatContext{sourceCanSee: true, targetCanSee: true}

	plainA, plainB := newGrappleChar(1), newGrappleChar(2)
	base := calcAttackScore(plainA, plainB, 0, ctx)

	a, b := newGrappleChar(1), newGrappleChar(2)
	mount(t, a, b)
	grappled := calcAttackScore(a, b, 0, ctx)

	assert.Greater(t, grappled, base,
		"a ground-grapple controller should attack at a higher score than standing")
}

func TestGrapple_StandingControllerGainsAttackScore(t *testing.T) {
	ctx := combatContext{sourceCanSee: true, targetCanSee: true}

	plainA, plainB := newGrappleChar(1), newGrappleChar(2)
	base := calcAttackScore(plainA, plainB, 0, ctx)

	a, b := newGrappleChar(1), newGrappleChar(2)
	clinch(t, a, b)
	grappled := calcAttackScore(a, b, 0, ctx)

	assert.Greater(t, grappled, base,
		"a standing-grapple controller should attack at a higher score than standing free")
}

// THE regression guard for the self-cancellation. Ground control must beat
// standing control, because ground is the stronger position. Under the old
// crit-threshold implementation ground netted ZERO while standing netted -0.2,
// i.e. ground was strictly WORSE than standing.
func TestGrapple_GroundBeatsStanding_NoSelfCancellation(t *testing.T) {
	ctx := combatContext{sourceCanSee: true, targetCanSee: true}

	sa, sb := newGrappleChar(1), newGrappleChar(2)
	clinch(t, sa, sb)
	standing := calcAttackScore(sa, sb, 0, ctx)

	ga, gb := newGrappleChar(1), newGrappleChar(2)
	mount(t, ga, gb)
	ground := calcAttackScore(ga, gb, 0, ctx)

	assert.Greater(t, ground, standing,
		"ground control must beat standing control; the old crit-threshold form cancelled ground to net zero")
}

// The crit threshold must no longer respond to position at all — that job now
// belongs to the attack score. Buff modifiers (Accuracy/Blink) stay.
//
// This uses a STANDING grapple deliberately. A ground grapple would pass
// against the OLD code too, because that is precisely where the -0.4/+0.4
// self-cancellation lands — the test would prove nothing. Standing grapple
// nets -0.2 under the old code, so it can actually detect the removal.
func TestGrapple_CritThresholdNoLongerPositional(t *testing.T) {
	plainA, plainB := newGrappleChar(1), newGrappleChar(2)
	base := calcCritThreshold(plainA, plainB)

	a, b := newGrappleChar(1), newGrappleChar(2)
	clinch(t, a, b)
	grappled := calcCritThreshold(a, b)

	assert.Equal(t, base, grappled,
		"calcCritThreshold must ignore grapple position after chunk 5.11c")
}

// And the ground case, which the old code cancelled to net zero. Kept as a
// separate test so the two failure modes stay distinguishable.
func TestGrapple_CritThresholdIgnoresGroundGrapple(t *testing.T) {
	plainA, plainB := newGrappleChar(1), newGrappleChar(2)
	base := calcCritThreshold(plainA, plainB)

	a, b := newGrappleChar(1), newGrappleChar(2)
	mount(t, a, b)

	assert.Equal(t, base, calcCritThreshold(a, b),
		"calcCritThreshold must ignore ground grapple after chunk 5.11c")
}
