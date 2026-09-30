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
	home, ext, deco, key := offers[OfferHome], offers[OfferExtension], offers[OfferRedecorate], offers[OfferGuestKey]
	box, safe := offers[OfferContainer], offers[OfferStrongbox]

	if _, owns := HouseOf(user.UserId, b.BuildingId); owns {
		say(`You've got a room already. Door's behind me, hand on the plate, same as always.`)
		if ext.Available {
			say(fmt.Sprintf(`If you want it bigger, an extension deed is %d gold for you. Goes up every time, that's %s's rule. A redecorating voucher is %d. Type list.`, ext.Price, b.Proprietor, deco.Price))
		} else {
			say(fmt.Sprintf(`No extensions for you just now. %s. A redecorating voucher is %d, if you're bored of the walls. Type list.`, ext.Note, deco.Price))
		}
		if key.Available {
			say(fmt.Sprintf(`Want to let a friend in? A guest key's %d. Give it to them, they use it on the door, done. Type house to see who's got in.`, key.Price))
		}
		if box.Available {
			say(fmt.Sprintf(`Somewhere to keep your things? A container deed's %d, a strongbox deed's %d. You name it, it turns up. Anyone you let in can use a container. A strongbox opens for you alone.`, box.Price, safe.Price))
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
		say(fmt.Sprintf(`But %s only lets to people %s can vouch for, and nobody's vouched for you. %s`, b.Proprietor, b.VouchedBy, b.StandingHint))
		return true
	}
	if user.Character.Gold+user.Character.Bank < tier.Price {
		say(`You'd want the gold first. The bank counts. Type buy home when you've got it.`)
		return true
	}
	say(fmt.Sprintf(`%s speaks well enough of you. Type buy home and I'll get the stamp out.`, capitalise(b.VouchedBy)))
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
