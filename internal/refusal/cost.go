package refusal

import "github.com/GoMudEngine/GoMud/internal/characters"

// CostText is the refusal for a voluntary action whose pool could not pay;
// empty when the cost was not refused.
func CostText(result characters.CostCommitResult) string {
	if result.Status != characters.CostRefused {
		return ``
	}
	switch result.Pool {
	case characters.PoolStamina:
		return `You are too spent to manage that right now.`
	case characters.PoolConviction:
		return `You cannot muster the resolve for that right now.`
	}
	return ``
}

// Cost sends CostText when the cost was refused, and nothing otherwise. A mob
// actor's SendText is a no-op, so mob wrappers stay silent as before.
func Cost(to Recipient, result characters.CostCommitResult) {
	if text := CostText(result); text != `` {
		send(to, text)
	}
}
