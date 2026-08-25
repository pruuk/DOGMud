package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// engage_aggro_type_test.go — the surviving half of the retired
// surprise_aggro_test.go / surprise_attack_test.go pair.
//
// U10d deleted the uncontested pre-combat burst (actions.SurpriseAttack). The
// EngageAggroType contract it used to be derived from SURVIVES verbatim, so
// the assertions that were really about that contract are ported here:
//
//  1. a non-hidden attacker opens as an ordinary attack;
//  2. a missing target opens as an ordinary attack;
//  3. a HIDDEN attacker whose special-move cooldown is unavailable opens as an
//     ordinary attack (this was surprise_attack_test.go's on-cooldown case);
//  4. a hidden attacker with a free cooldown opens as a surprise, and paying
//     for it claims the cooldown.
//
// Dropped with the burst: the SurpriseAttackResult.BlockReason strings and the
// per-weapon StrikeCount assertions — both described a type and a code path
// that no longer exist.
// ---------------------------------------------------------------------------

// newAggroTestRoom builds a minimal room for EngageAggroType tests.
func newAggroTestRoom() *rooms.Room {
	return &rooms.Room{RoomId: 9810}
}

// newAggroAttackerMob builds a minimal attacker mob with Cooldowns
// initialized. Call addHiddenBuff(&mob.Character) to make it hidden.
func newAggroAttackerMob(instanceId int) *mobs.Mob {
	m := &mobs.Mob{InstanceId: instanceId}
	m.Character.Cooldowns = make(map[string]int)
	return m
}

// TestEngageAggroType pins the rule that four call sites previously derived
// themselves and got wrong in different directions (2026-07-20 audit finding
// 0.5): usercommands/attack.go hardcoded DefaultAttack even when opening from
// stealth, while the mob paths keyed off IsHidden() alone and so ambushed on a
// cooldown they had not paid.
func TestEngageAggroType(t *testing.T) {
	newActor := func() *recordingActor {
		c := &characters.Character{Name: "Sneak"}
		c.Validate()
		return &recordingActor{char: c}
	}

	t.Run("not_hidden_is_a_default_attack", func(t *testing.T) {
		a, target := newActor(), newActor()
		require.False(t, a.char.IsHidden(), "precondition: attacker is not hidden")

		assert.Equal(t, characters.DefaultAttack, EngageAggroType(a, target),
			"an ordinary opener must not be typed as a surprise attack")
	})

	t.Run("no_target_is_a_default_attack", func(t *testing.T) {
		a := newActor()

		assert.Equal(t, characters.DefaultAttack, EngageAggroType(a, nil),
			"a missing target cannot produce a surprise attack")
	})

	t.Run("hidden_but_on_cooldown_is_a_default_attack", func(t *testing.T) {
		room := newAggroTestRoom()
		attacker := newAggroAttackerMob(9810)
		addHiddenBuff(&attacker.Character)
		// Pre-seed the cooldown so TryCooldown returns false.
		attacker.Character.Cooldowns["special-move"] = 5
		victim := newAggroAttackerMob(9811)
		require.True(t, attacker.Character.IsHidden(), "precondition: attacker is hidden")

		got := EngageAggroType(
			NewMobActorInRoom(attacker, room),
			NewMobActorInRoom(victim, room),
		)

		assert.Equal(t, characters.DefaultAttack, got,
			"a hidden attacker who cannot pay the special-move cooldown opens as an ordinary attack")
	})

	t.Run("hidden_with_free_cooldown_is_a_surprise_and_claims_it", func(t *testing.T) {
		room := newAggroTestRoom()
		attacker := newAggroAttackerMob(9812)
		addHiddenBuff(&attacker.Character)
		victim := newAggroAttackerMob(9813)
		require.True(t, attacker.Character.IsHidden(), "precondition: attacker is hidden")
		require.Zero(t, attacker.Character.Cooldowns["special-move"],
			"precondition: the special-move cooldown is free")

		got := EngageAggroType(
			NewMobActorInRoom(attacker, room),
			NewMobActorInRoom(victim, room),
		)

		assert.Equal(t, characters.SurpriseAttack, got,
			"a hidden attacker with a free cooldown opens as a surprise attack")
		assert.NotZero(t, attacker.Character.Cooldowns["special-move"],
			"the surprise opener must claim the special-move cooldown")
	})
}
