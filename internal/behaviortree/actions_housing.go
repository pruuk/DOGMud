package behaviortree

import (
	"strings"

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

	say := func(line string) { mob.Command(`say ` + line) }

	// A question that mentions buying ("how much to buy a room?", "do you
	// buy furniture?") is a question: it is answered, and nothing is sold.
	if isQuestion(ctx.Event.Text) {
		if housing.DescribeTerms(user, say, buildingId, tierId) {
			return Success
		}
		return Failure
	}

	// "buy a deed" or "buy a voucher" reach those offers; "buy a room" means a
	// home to someone without one and an extension to someone with one.
	// Anything else is not a purchase: nothing is charged on a guess.
	_, owns := housing.HouseOf(user.UserId, buildingId)
	key, ok := housing.MatchOffer(ctx.Event.Text, owns)
	if !ok {
		say(`Buy what? Type list and I'll show you what the Widow sells. Then buy home, buy deed, and so on.`)
		return Success
	}
	housing.Buy(user, say, buildingId, tierId, key)
	return Success
}

// questionWords open a question in English.
var questionWords = map[string]bool{
	`how`: true, `what`: true, `whats`: true, `what's`: true, `which`: true, `can`: true,
	`could`: true, `do`: true, `does`: true, `did`: true, `will`: true, `would`: true,
	`is`: true, `are`: true, `may`: true, `should`: true, `why`: true, `where`: true,
	`when`: true, `who`: true,
}

// isQuestion reports whether an ask reads as a question rather than a
// request: it has a question mark, or opens with a question word ("to" and
// "if" skipped: "ask hobb if I can buy a room").
func isQuestion(text string) bool {
	if strings.Contains(text, `?`) {
		return true
	}
	for _, w := range strings.Fields(strings.ToLower(text)) {
		w = strings.Trim(w, `.,!"'()`)
		if w == `to` || w == `if` || w == `whether` || w == `` {
			if w == `if` || w == `whether` {
				return true
			}
			continue
		}
		return questionWords[w]
	}
	return false
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
