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

// costSpent reports whether an action's cost was actually taken from a pool.
func costSpent(result characters.CostCommitResult) bool {
	return result.Charged > 0 &&
		(result.Status == characters.CostPaid || result.Status == characters.CostPartiallyPaid)
}

// NoTargetSpentText answers a move that paid its cost and then found its
// target gone (#546). It names the spent effort and offers no retry hint,
// since a second try would pay again.
func NoTargetSpentText(verb string) string {
	return `Your target is gone before you can ` + verb + `, and the effort is spent.`
}

// CooldownSpentText answers a move that paid its cost and then lost the
// shared special-move timer to another action (#546).
func CooldownSpentText(doing string) string {
	return `You need a moment to recover before you can ` + doing + `, and the effort is spent.`
}

// NoTargetAfterCost is NoTarget, or NoTargetSpentText when the attempt had
// already paid its cost. Pass the action result's cost as it came back.
func NoTargetAfterCost(to Recipient, verb string, cost characters.CostCommitResult) {
	if costSpent(cost) {
		send(to, NoTargetSpentText(verb))
		return
	}
	NoTarget(to, verb)
}

// CooldownAfterCost is Cooldown, or CooldownSpentText when the attempt had
// already paid its cost.
func CooldownAfterCost(to Recipient, doing string, cost characters.CostCommitResult) {
	if costSpent(cost) {
		send(to, CooldownSpentText(doing))
		return
	}
	Cooldown(to, doing)
}

// Cost sends CostText when the cost was refused, and nothing otherwise. A mob
// actor's SendText is a no-op, so mob wrappers stay silent as before.
func Cost(to Recipient, result characters.CostCommitResult) {
	if text := CostText(result); text != `` {
		send(to, text)
	}
}
