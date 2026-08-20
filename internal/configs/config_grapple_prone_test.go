package configs

import "testing"

// U6b Task 13: the two prone literals in combat.AttemptGrapple moved into
// config. THE DIRECTION MATTERS and an earlier plan draft had it SWAPPED:
// the code multiplies the DEFENDER's score by 0.3 when the defender is down
// and the ATTACKER's score by 0.5 when the attacker is down, so the knobs
// ship as GrappleProneDefenderMod 0.3 / GrappleProneAttackerMod 0.5.
// Shipping the swapped values would be a silent double flip (prone attackers
// worse, prone defenders better).
func TestGrappleProneMods_Defaults(t *testing.T) {
	b := Balance{}
	b.Validate()
	if b.GrappleProneAttackerMod != 0.5 {
		t.Fatalf("GrappleProneAttackerMod default must be 0.5 (the old `AttackScore *= 0.5` literal), got %v", b.GrappleProneAttackerMod)
	}
	if b.GrappleProneDefenderMod != 0.3 {
		t.Fatalf("GrappleProneDefenderMod default must be 0.3 (the old `DefenseScore *= 0.3` literal), got %v", b.GrappleProneDefenderMod)
	}
}

// These are score multipliers, not off-switches: a non-positive value is a
// config error and falls back to the default rather than zeroing a side's
// score.
func TestGrappleProneMods_NonPositiveFallsBackToDefault(t *testing.T) {
	b := Balance{GrappleProneAttackerMod: -1, GrappleProneDefenderMod: 0}
	b.Validate()
	if b.GrappleProneAttackerMod != 0.5 {
		t.Fatalf("non-positive GrappleProneAttackerMod must fall back to 0.5, got %v", b.GrappleProneAttackerMod)
	}
	if b.GrappleProneDefenderMod != 0.3 {
		t.Fatalf("non-positive GrappleProneDefenderMod must fall back to 0.3, got %v", b.GrappleProneDefenderMod)
	}
}
