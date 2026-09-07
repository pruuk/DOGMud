package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
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

// TestSpecialMoveCooldownUsesTheConfiguredDuration pins the outlier out of
// existence. combat_helpers.go hardcoded "1 rounds" while sixteen verbs used the
// configured value, so one path ran a fraction of the intended cooldown and
// nothing anywhere could notice.
func TestSpecialMoveCooldownUsesTheConfiguredDuration(t *testing.T) {
	c := &characters.Character{}
	ClaimSpecialMove(c)

	if got := c.GetCooldown(SpecialMoveCooldownTag); got <= 1 {
		t.Errorf("cooldown = %d rounds, want the configured SpecialMoveCooldown (>1); "+
			"a value of 1 means the hardcoded outlier came back", got)
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
