package housing

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/users"
)

// DescribeTerms has the landlord explain what a room costs and, for this
// player, what stands between them and one. It changes nothing. Every fact
// comes from the authored building, so the prose cannot drift from the price.
func DescribeTerms(user *users.UserRecord, say func(string), buildingId string, tierId string) bool {
	b, ok := GetBuilding(buildingId)
	if !ok {
		return false
	}
	tier, ok := b.Tier(tierId)
	if !ok {
		return false
	}

	if _, owns := HouseOf(user.UserId, b.BuildingId); owns {
		say(fmt.Sprintf(`You've a room here already. Lay your hand on the plate and go on through the %s.`, b.DoorExit))
		return true
	}

	vacant := len(VacantUnits(b.BuildingId))
	if vacant < tier.Rooms {
		say(`Every room I have is let just now. I'm sorry for it. Ask me another day.`)
		return true
	}

	say(fmt.Sprintf(`I let %s for %d gold, paid once, up front. Four walls, a floor, a window and a door that opens for you and nobody else. What you do with it after is your own affair.`, tier.Name, tier.Price))

	if repTierFor(b.Faction, user.UserId) < b.MinTier() {
		say(`But I only let to people the quarter can vouch for, and nobody's vouched for you yet. Do right by the folk around here and come back.`)
		return true
	}
	if user.Character.Gold+user.Character.Bank < tier.Price {
		say(`You'd want the gold first, mind. What's in the bank counts. Come back and ask me to buy one when you have it.`)
		return true
	}
	say(`The quarter speaks well enough of you. Ask me to buy one and it's yours.`)
	return true
}
