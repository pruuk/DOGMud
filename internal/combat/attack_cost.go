package combat

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
)

// ChargeAttackCost admits one already-planned autoattack round through the
// shared quote/commit contract. The request stays raw: QuoteActionCost owns the
// attack registry lookup, equipped combat-skill selection, encumbrance and
// modifier composition, then applies Units to the unrounded one-swing amount.
//
// Autoattack is life-preserving, so an unaffordable round commits partially and
// still resolves. CostCommitResult.Short tells the resolver to omit only the
// equipped combat-skill addend from hit scoring. A nil attacker is a defensive
// no-charge result; non-positive swings flow through the quote contract and are
// likewise no-charge rather than short.
func ChargeAttackCost(attacker *characters.Character, swings int) characters.CostCommitResult {
	if attacker == nil {
		return characters.CostCommitResult{
			Status: characters.CostNoCharge,
			Pool:   characters.PoolStamina,
		}
	}

	cfg := configs.GetBalanceConfig()
	quote := attacker.QuoteActionCost(characters.ActionCostRequest{
		Action:   costs.ActionAttack,
		Pool:     characters.PoolStamina,
		Base:     float64(cfg.AttackBaseStaminaCost),
		Modifier: float64(cfg.AttackCostModifier),
		Units:    swings,
	})
	return attacker.CommitCost(quote, characters.CostPartial)
}
