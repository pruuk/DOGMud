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

// Shared actions return the structured CostCommitResult only; user-command
// wrappers render a refused cost with refusal.Cost (or refusal.CostText),
// while mob wrappers stay silent.
