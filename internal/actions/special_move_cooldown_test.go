package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
)

// TestSpecialMoveCooldownClaimIsAtomic pins the property the old idiom could not
// offer: asking and taking are ONE call. Sixteen verbs used to call
// CooldownReady first and TryCooldown many lines later, and reload's two halves
// sat 47 lines apart -- which is exactly where "reload denies the ambush" hid.
func TestSpecialMoveCooldownClaimIsAtomic(t *testing.T) {
	c := &characters.Character{}

	if !SpecialMoveReady(c) {
		t.Fatal("a fresh character must have the special-move cooldown free")
	}
	if !ClaimSpecialMove(c) {
		t.Fatal("the first claim must succeed")
	}
	if SpecialMoveReady(c) {
		t.Error("the cooldown must read as busy after a claim")
	}
	if ClaimSpecialMove(c) {
		t.Error("a second claim must fail while the cooldown is running")
	}
}

// TestSpecialMoveCooldownUsesTheConfiguredDuration pins that the duration comes
// from Balance.SpecialMoveCooldown and is not baked into the code. A dead
// helper in combat_helpers.go used to hardcode "1 rounds" while sixteen verbs
// read the config, and nothing could notice because nothing compared them.
//
// ⚠️ The pinned value is deliberately NOT the shipped or default one. An
// earlier version of this test asserted only `got > 1`, which any hardcoded
// literal above one would satisfy -- including the Go default of 5, so an
// implementation that ignored the config entirely passed it. Pinning an
// arbitrary value is what makes this test able to tell "reads the config" from
// "happens to match the config".
func TestSpecialMoveCooldownUsesTheConfiguredDuration(t *testing.T) {
	const pinned = 7 // neither the Go default (5) nor the shipped value (4)

	cfg := configs.GetConfig()
	cfg.Balance.SpecialMoveCooldown = pinned
	configs.SetConfigForTest(t, cfg)

	c := &characters.Character{}
	ClaimSpecialMove(c)

	if got := c.GetCooldown(SpecialMoveCooldownTag); got != pinned {
		t.Errorf("cooldown = %d rounds, want %d from Balance.SpecialMoveCooldown; "+
			"a value that ignores the pin means the duration is hardcoded", got, pinned)
	}
}

// TestReleaseSpecialMoveClearsIt covers the refund path the mutation and mob
// cast paths need when an action aborts after claiming.
func TestReleaseSpecialMoveClearsIt(t *testing.T) {
	c := &characters.Character{}
	ClaimSpecialMove(c)
	ReleaseSpecialMove(c)

	if !SpecialMoveReady(c) {
		t.Error("Release must return the cooldown to free")
	}
	if !ClaimSpecialMove(c) {
		t.Error("a claim must succeed again after a release")
	}
}

// TestReleaseSpecialMoveOnNilMapDoesNotPanic guards the zero-value path. A
// Character built by a test or a fresh spawn can carry a nil Cooldowns map, and
// an abort can call Release before anything has ever claimed.
func TestReleaseSpecialMoveOnNilMapDoesNotPanic(t *testing.T) {
	c := &characters.Character{}
	ReleaseSpecialMove(c) // must not panic on a nil map

	if !SpecialMoveReady(c) {
		t.Error("releasing a never-claimed cooldown must leave it free")
	}
}
