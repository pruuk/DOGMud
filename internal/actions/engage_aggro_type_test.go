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
// U10d follow-up: the second return value is pinned in BOTH directions in every
// case. It is what tells a caller WHICH of the two DefaultAttack reasons it
// got, and the player-facing melee path speaks it (sendMeleeAmbushDenial in
// internal/usercommands/attack.go). A version of it that were true whenever the
// opener was merely absent would announce a refused ambush to every ordinary
// attacker in the game, so "false when nobody asked for an opener" is the
// assertion doing the most work here.
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

		got, onCooldown := EngageAggroType(a, target)

		assert.Equal(t, characters.DefaultAttack, got,
			"an ordinary opener must not be typed as a surprise attack")
		assert.False(t, onCooldown,
			"an attacker who was never hidden asked for no opener, so none was refused: "+
				"reporting one here would tell every ordinary attacker in the game that "+
				"their ambush failed")
	})

	// The attacker here MUST be hidden with a free cooldown, or the IsHidden
	// gate short-circuits and the target==nil guard is never reached. The
	// first cut of this subtest used a plain non-hidden actor and could not
	// fail: deleting `target == nil ||` from EngageAggroType left it green.
	t.Run("no_target_is_a_default_attack", func(t *testing.T) {
		room := newAggroTestRoom()
		attacker := newAggroAttackerMob(9814)
		addHiddenBuff(&attacker.Character)
		require.True(t, attacker.Character.IsHidden(),
			"precondition: attacker is hidden, so the nil-target guard is reachable")
		require.Zero(t, attacker.Character.Cooldowns["special-move"],
			"precondition: the cooldown is free, so only the nil target can refuse")

		got, onCooldown := EngageAggroType(NewMobActorInRoom(attacker, room), nil)

		assert.Equal(t, characters.DefaultAttack, got,
			"a missing target cannot produce a surprise attack")
		assert.False(t, onCooldown,
			"a missing target is not a cooldown refusal: the timer was never asked")
		assert.Zero(t, attacker.Character.Cooldowns["special-move"],
			"a refused opener must not have claimed the special-move cooldown")
	})

	// The guard's second half. A nil character reaches IsHidden() on a nil
	// receiver if the guard is removed, so this subtest fails loudly rather
	// than silently when the check goes missing.
	t.Run("nil_character_is_a_default_attack", func(t *testing.T) {
		a := &recordingActor{char: nil}
		target := newActor()

		got, onCooldown := EngageAggroType(a, target)

		assert.Equal(t, characters.DefaultAttack, got,
			"an actor with no character cannot produce a surprise attack")
		assert.False(t, onCooldown,
			"an actor with no character is not a cooldown refusal")
	})

	t.Run("hidden_but_on_cooldown_is_a_default_attack", func(t *testing.T) {
		room := newAggroTestRoom()
		attacker := newAggroAttackerMob(9810)
		addHiddenBuff(&attacker.Character)
		// Pre-seed the cooldown so TryCooldown returns false.
		attacker.Character.Cooldowns["special-move"] = 5
		victim := newAggroAttackerMob(9811)
		require.True(t, attacker.Character.IsHidden(), "precondition: attacker is hidden")

		got, onCooldown := EngageAggroType(
			NewMobActorInRoom(attacker, room),
			NewMobActorInRoom(victim, room),
		)

		assert.Equal(t, characters.DefaultAttack, got,
			"a hidden attacker who cannot pay the special-move cooldown opens as an ordinary attack")
		assert.True(t, onCooldown,
			"the ONE case a caller has to be able to see: the attacker wanted an ambush, the "+
				"shared timer refused it, and SetAggro will reveal them anyway")
	})

	t.Run("hidden_with_free_cooldown_is_a_surprise_and_claims_it", func(t *testing.T) {
		room := newAggroTestRoom()
		attacker := newAggroAttackerMob(9812)
		addHiddenBuff(&attacker.Character)
		victim := newAggroAttackerMob(9813)
		require.True(t, attacker.Character.IsHidden(), "precondition: attacker is hidden")
		require.Zero(t, attacker.Character.Cooldowns["special-move"],
			"precondition: the special-move cooldown is free")

		got, onCooldown := EngageAggroType(
			NewMobActorInRoom(attacker, room),
			NewMobActorInRoom(victim, room),
		)

		assert.Equal(t, characters.SurpriseAttack, got,
			"a hidden attacker with a free cooldown opens as a surprise attack")
		assert.False(t, onCooldown,
			"the opener was GRANTED: telling the player it was refused would contradict the "+
				"crit they are about to land")
		assert.NotZero(t, attacker.Character.Cooldowns["special-move"],
			"the surprise opener must claim the special-move cooldown")
	})
}
