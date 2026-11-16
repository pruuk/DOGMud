package actions

import "github.com/GoMudEngine/GoMud/internal/configs"

// bleedPerRound is one bleed stack's per-round health loss: the attacker's
// Strength divided by the move's <Move>BleedStrengthDivisor knob, never less
// than its <Move>BleedMin knob. Both knobs are validated to at least 1, so
// the division is safe. Every bleed move passes the result as the stack's
// magnitude (negated) and <Move>BleedRounds as its rounds.
func bleedPerRound(strength int, divisor, floor configs.ConfigInt) int {
	amt := max(strength/int(divisor), int(floor))
	return amt
}
