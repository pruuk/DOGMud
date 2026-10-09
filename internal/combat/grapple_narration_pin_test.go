package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/movenarration"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGrappleNarrationPinnedToOriginalLiterals is the proof for M4e-1's last
// slice: the six lines HandleGrappleCritFailure and AttemptCritDisarm used to
// hold as Go literals now come from the movenarration store, and must render
// BYTE IDENTICAL to what those literals produced.
//
// The 118-row byte-identity net (internal/mobcommands/special_move_net_test.go)
// covers only the thirteen mob-command files migrated in M4e-1 PR 1a/1b; these
// six lines live in internal/combat and had no proof of their own until this
// test. The wanted strings below are copied verbatim from the pre-migration
// source (git history of grapple.go / criteffects.go), not derived from the
// YAML, so a change to either side that drifts the wording fails here.
func TestGrappleNarrationPinnedToOriginalLiterals(t *testing.T) {
	if err := movenarration.LoadFrom("../../_datafiles/world/dogmud/narration/special-moves"); err != nil {
		t.Fatalf("loading the shipped store: %v", err)
	}

	t.Run("crit_failure", func(t *testing.T) {
		wantActor := `<ansi fg="red-bold">You overextend badly and fall to the ground!</ansi>`
		wantActee := `<ansi fg="yellow-bold">Your opponent overextends and falls - you see an opening!</ansi>`
		wantObserver := `<ansi fg="combat">The failed grapple sends them sprawling!</ansi>`

		attacker := &characters.Character{Name: "Attacker", Position: position.NewMachine()}
		defender := &characters.Character{Name: "Defender", Position: position.NewMachine()}
		require.True(t, attacker.IsStanding(), "precondition: attacker starts standing")

		res := HandleGrappleCritFailure(attacker, defender)

		assert.Equal(t, wantActor, res.Message, "attacker (actor) line")
		assert.Equal(t, wantActee, res.TargetMessage, "defender (actee) line")
		assert.Equal(t, wantObserver, res.RoomMessage, "observer line")
	})

	t.Run("disarm", func(t *testing.T) {
		const sourceName = "Grappler"
		const targetName = "Victim"
		const weaponName = "rusty dagger"

		// {weapon} sits in an item tag (spec F2) so the pipeline can hide it
		// from a spectator who sees only shapes.
		taggedWeapon := `<ansi fg="item">` + weaponName + `</ansi>`
		wantMessage := `<ansi fg="yellow-bold">You disarm ` + targetName + `, loosening their grip on their ` + taggedWeapon + `!</ansi>`
		wantTargetMsg := `<ansi fg="red-bold">` + sourceName + ` disarms you! Your ` + taggedWeapon + ` slips from your grasp!</ansi>`
		wantRoomMessage := `<ansi fg="combat">` + sourceName + ` disarms ` + targetName + `, knocking their ` + taggedWeapon + ` loose!</ansi>`

		source := characters.New()
		source.Name = sourceName
		target := characters.New()
		target.Name = targetName
		target.Equipment.Weapon = items.Item{ItemId: 999901, Spec: &items.ItemSpec{NameSimple: weaponName}}

		res := AttemptCritDisarm(source, target, 100.0) // 100% forces success deterministically

		require.True(t, res.Success, "precondition: disarm must succeed to render its messages")
		assert.Equal(t, wantMessage, res.Message, "disarmer (actor) line")
		assert.Equal(t, wantTargetMsg, res.TargetMsg, "disarmed (actee) line")
		assert.Equal(t, wantRoomMessage, res.RoomMessage, "observer line")
	})
}
