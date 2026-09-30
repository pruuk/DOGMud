package behaviortree

import (
	"github.com/GoMudEngine/GoMud/internal/housing"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// actBuyHousing handles "ask <landlord> to buy a room / deed / voucher".
// Params:
//
//	building: back_court_lodgings   # a housing building_id
//	tier: simple                    # the tier_id sold as a home
//
// It picks the offer from the ask text (housing.MatchOffer) and delegates the
// whole sale (standing, one home per account, vacancy, the escalating
// extension price, the durable record, messaging) to housing.Buy, the same
// path the list/buy commands use, with the landlord speaking every refusal. Returns Failure only when the tree is
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

	// "buy a deed" or "buy a voucher" reach those offers; "buy a room" means a
	// home to someone without one and an extension to someone with one; a bare
	// "buy" is a home, as it always was.
	_, owns := housing.HouseOf(user.UserId, buildingId)
	key, ok := housing.MatchOffer(ctx.Event.Text, owns)
	if !ok {
		key = housing.OfferHome
	}
	housing.Buy(user, func(line string) { mob.Command(`say ` + line) }, buildingId, tierId, key)
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
