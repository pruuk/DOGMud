package housing

import (
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
		say(b.Line(`terms.owner`))
		if ext.Available {
			say(b.Line(`terms.extension`, `price`, ext.Price, `voucher`, deco.Price))
		} else {
			say(b.Line(`terms.no_extension`, `reason`, ext.Note, `voucher`, deco.Price))
		}
		if key.Available {
			say(b.Line(`terms.guest_key`, `price`, key.Price))
		}
		if box.Available {
			say(b.Line(`terms.storage`, `container`, box.Price, `strongbox`, safe.Price))
		}
		return true
	}

	tier, _ := b.Tier(tierId)
	if !home.Available && home.Price == 0 {
		say(b.Line(`terms.full`))
		return true
	}
	say(b.Line(`terms.pitch`, `Tier`, capitalise(tier.Name), `tier`, tier.Name, `price`, tier.Price))
	if !home.Available {
		say(b.Line(`terms.not_vouched`))
		return true
	}
	if user.Character.Gold+user.Character.Bank < tier.Price {
		say(b.Line(`terms.no_gold`))
		return true
	}
	if !b.ChecksStanding() {
		say(b.Line(`terms.open`))
		return true
	}
	say(b.Line(`terms.ready`))
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
