package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/costs"
)

// admitFullCost admits a voluntary action through the shared quote/commit
// contract. Callers perform their own read-only action validation first.
func admitFullCost(actor Actor, action costs.Action, pool characters.Pool, base float64) characters.CostCommitResult {
	character := actor.GetCharacter()
	quote := character.QuoteActionCost(characters.ActionCostRequest{
		Action:   action,
		Pool:     pool,
		Base:     base,
		Modifier: 1.0,
		Units:    1,
	})
	return character.CommitCost(quote, characters.CostFullOrRefuse)
}

// costRefusalText supplies private player-facing prose for a refused voluntary
// action. Shared actions return the structured result only; user-command
// wrappers decide whether to render this text, while mob wrappers stay silent.
func costRefusalText(result characters.CostCommitResult) string {
	if result.Status != characters.CostRefused {
		return ""
	}

	switch result.Pool {
	case characters.PoolStamina:
		return "You are too spent to manage that right now."
	case characters.PoolConviction:
		return "You cannot muster the resolve for that right now."
	default:
		return ""
	}
}

// CostRefusalText supplies the pool-aware refusal prose that player-command
// wrappers render for a refused voluntary action. Mob wrappers remain silent.
func CostRefusalText(result characters.CostCommitResult) string {
	return costRefusalText(result)
}
