package behaviortree

import (
	"github.com/GoMudEngine/GoMud/internal/housing"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// actBuyHousing handles "ask <landlord> to buy a room". Params:
//
//	building: back_court_lodgings   # a housing building_id
//	tier: simple                    # a tier_id of that building
//
// It delegates the whole purchase (standing, one per account, vacancy,
// price, the durable record, messaging) to housing.Purchase, with the
// landlord speaking every refusal. Returns Failure only when the tree is
// misconfigured or the actors are gone, so later branches can answer;
// every real outcome, refusals included, is Success because it was handled.
func actBuyHousing(params map[string]any, ctx *EvalContext) Result {
	mob := mobs.GetInstance(ctx.InstanceId)
	if mob == nil {
		return Failure
	}
	user := users.GetByUserId(ctx.Event.UserId)
	if user == nil {
		return Failure
	}
	buildingId := getStringParam(params, "building")
	tierId := getStringParam(params, "tier")
	if buildingId == `` || tierId == `` {
		return Failure
	}

	housing.Purchase(user, func(line string) { mob.Command(`say ` + line) }, buildingId, tierId)
	return Success
}

// actHousingTerms handles "ask <landlord> about a room". Same params as
// buy_housing. The landlord states the price and, for this player, whether
// their standing, their gold or a room they already own changes the answer.
// Nothing is charged. Failure (misconfigured, or actors gone) falls through.
func actHousingTerms(params map[string]any, ctx *EvalContext) Result {
	mob := mobs.GetInstance(ctx.InstanceId)
	if mob == nil {
		return Failure
	}
	user := users.GetByUserId(ctx.Event.UserId)
	if user == nil {
		return Failure
	}
	buildingId := getStringParam(params, "building")
	tierId := getStringParam(params, "tier")
	if buildingId == `` || tierId == `` {
		return Failure
	}
	if !housing.DescribeTerms(user, func(line string) { mob.Command(`say ` + line) }, buildingId, tierId) {
		return Failure
	}
	return Success
}
