package housing

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/users"
)

// DescribeTerms has the landlord explain, in words, what his list says for
// this player: the price of a home, or of the next extension and a voucher,
// and what stands in the way. It changes nothing. Every fact comes from
// Offers, so the speech cannot drift from the list.
func DescribeTerms(user *users.UserRecord, say func(string), buildingId string, tierId string) bool {
	b, ok := GetBuilding(buildingId)
	if !ok {
		return false
	}
	if _, ok := b.Tier(tierId); !ok {
		return false
	}
	offers := map[string]Offer{}
	for _, o := range Offers(user, buildingId, tierId) {
		offers[o.Key] = o
	}
	home, ext, deco := offers[OfferHome], offers[OfferExtension], offers[OfferRedecorate]

	if _, owns := HouseOf(user.UserId, b.BuildingId); owns {
		say(`You've got a room already. Door's behind me, hand on the plate, same as always.`)
		if ext.Available {
			say(fmt.Sprintf(`If you want it bigger, an extension deed is %d gold for you. Goes up every time, that's the Widow's rule. A redecorating voucher is %d. Type list.`, ext.Price, deco.Price))
		} else {
			say(fmt.Sprintf(`No extensions for you just now. %s. A redecorating voucher is %d, if you're bored of the walls. Type list.`, ext.Note, deco.Price))
		}
		return true
	}

	tier, _ := b.Tier(tierId)
	if !home.Available && home.Price == 0 {
		say(`Every room's let. Nothing I can do. Try again another day.`)
		return true
	}
	say(fmt.Sprintf(`%s, %d gold, paid once. Four walls, a window, and a door that opens for you and nobody else. Extensions and redecorating come after, for lodgers. It's all on the list.`, capitalise(tier.Name), tier.Price))
	if !home.Available {
		say(`But the Widow only lets to people the quarter can vouch for, and nobody's vouched for you. Do some good round the Common Quarter and come back.`)
		return true
	}
	if user.Character.Gold+user.Character.Bank < tier.Price {
		say(`You'd want the gold first. The bank counts. Type buy home when you've got it.`)
		return true
	}
	say(`Quarter speaks well enough of you. Type buy home and I'll get the stamp out.`)
	return true
}

func capitalise(s string) string {
	if s == `` {
		return s
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-'a'+'A') + s[1:]
	}
	return s
}
