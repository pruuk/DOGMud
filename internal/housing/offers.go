package housing

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// A landlord's list. The shop system prices each item once for everybody;
// what a landlord sells depends on who is asking (the next extension costs a
// multiple of what THIS lodger has paid), so the list is built per player here
// and rendered by the list command in the same table as any shop.

// Offer keys, which are also what "buy" matches.
const (
	OfferHome       = `home`
	OfferExtension  = `deed`
	OfferRedecorate = `voucher`
)

// Offer is one line of a landlord's list for one player.
type Offer struct {
	Key   string
	Name  string
	Price int // what it would cost this player now; 0 when not for sale to them
	// Note says why an offer is not for sale to this player, or what it is.
	Note string
	// Available is true when buying it now would go through, gold aside.
	Available bool
}

// Offers is the landlord's list for one player in one building.
func Offers(user *users.UserRecord, buildingId string, tierId string) []Offer {
	b, ok := GetBuilding(buildingId)
	if !ok {
		return nil
	}
	tier, _ := b.Tier(tierId)
	house, owns := HouseOf(user.UserId, b.BuildingId)
	vacant := len(VacantUnits(b.BuildingId))

	home := Offer{Key: OfferHome, Name: `Home (` + tier.Name + `)`, Price: tier.Price}
	switch {
	case owns:
		home.Price, home.Note = 0, `You already lodge here`
	case repTierFor(b.Faction, user.UserId) < b.MinTier():
		home.Note = `The quarter must vouch for you first`
	case vacant < tier.Rooms:
		home.Price, home.Note = 0, `Every room is let`
	default:
		home.Note, home.Available = `Four walls and a door that knows you`, true
	}

	ext := Offer{Key: OfferExtension, Name: itemName(b.ExtensionItemId, `Room Extension Deed`)}
	switch {
	case !owns:
		ext.Note = `Needs a home here first`
	case len(house.RoomIds) >= b.MaxRooms:
		ext.Note = `Your lodging is as big as the company allows`
	case vacant < 1:
		ext.Note = `No rooms left to add`
	default:
		ext.Price = extensionPrice(b, house)
		ext.Note, ext.Available = `Adds one room to your lodging`, true
	}

	deco := Offer{Key: OfferRedecorate, Name: itemName(b.RedecorateItemId, `Redecorating Voucher`)}
	if owns {
		deco.Price = b.RedecoratePrice
		deco.Note, deco.Available = `Rewrites one room's description, once`, true
	} else {
		deco.Note = `Needs a home here first`
	}

	return []Offer{home, ext, deco}
}

func extensionPrice(b Building, h House) int {
	return b.ExtensionPriceMultiplier * h.Spent()
}

func itemName(itemId int, fallback string) string {
	if spec := items.GetItemSpec(itemId); spec != nil && spec.Name != `` {
		return spec.Name
	}
	return fallback
}

// MatchOffer reads what a player asked to buy. "room" means a home to someone
// without one and an extension to someone with one. It returns false when the
// request is not for anything a landlord sells, so the ordinary shop can try.
func MatchOffer(request string, ownsHome bool) (string, bool) {
	for _, word := range strings.Fields(strings.ToLower(request)) {
		word = strings.Trim(word, `.,!?"'()`)
		word = strings.TrimSuffix(word, `s`)
		switch word {
		case `home`, `lodging`, `lease`:
			return OfferHome, true
		case `deed`, `extension`, `extend`, `addition`:
			return OfferExtension, true
		case `voucher`, `redecorating`, `redecorate`, `redecoration`, `decorating`, `description`:
			return OfferRedecorate, true
		case `room`:
			if ownsHome {
				return OfferExtension, true
			}
			return OfferHome, true
		}
	}
	return ``, false
}

// Buy sells one offer to a player, the landlord speaking through say.
func Buy(user *users.UserRecord, say func(string), buildingId string, tierId string, key string) {
	switch key {
	case OfferHome:
		Purchase(user, say, buildingId, tierId)
	case OfferExtension:
		buyExtension(user, say, buildingId)
	case OfferRedecorate:
		buyRedecorate(user, say, buildingId)
	}
}

// buyExtension sells an extension deed made out to this account. What was
// paid is added to the house's room spend BEFORE the deed is handed over or
// any gold is taken, so the next deed is priced up even if this one is never
// used, and a failed write costs nothing.
func buyExtension(user *users.UserRecord, say func(string), buildingId string) {
	b, ok := GetBuilding(buildingId)
	if !ok {
		return
	}
	house, owns := HouseOf(user.UserId, b.BuildingId)
	if !owns {
		say(`Extensions are for lodgers. You'd want a home here first. It's on the list.`)
		return
	}
	if len(house.RoomIds) >= b.MaxRooms {
		say(`The company won't let one lodger have more of the building than you've got. Rules. Not mine.`)
		return
	}
	if len(VacantUnits(b.BuildingId)) < 1 {
		say(`No rooms left to add on. The whole place is let. Try again when somebody moves out.`)
		return
	}
	price := extensionPrice(b, house)
	if user.Character.Gold+user.Character.Bank < price {
		say(fmt.Sprintf(`An extension runs you %d gold, the bank counting. You're short.`, price))
		return
	}

	deed := items.New(b.ExtensionItemId)
	if deed.ItemId == 0 {
		mudlog.Error(`housing.buyExtension`, `building`, b.BuildingId, `error`, `extension item does not exist`)
		return
	}
	deed.BoundUserId = user.UserId

	next := house.clone()
	next.RoomsPaid = house.Spent() + price
	if err := saveHouse(next); err != nil {
		mudlog.Error(`housing.buyExtension`, `user`, user.UserId, `error`, err.Error())
		say(`The ledger's in a state. I'll not take your coin till it's straight. Come back later.`)
		return
	}
	if !user.Character.StoreItem(deed) {
		// Nothing was taken yet: put the ledger back as it was.
		if err := saveHouse(house); err != nil {
			mudlog.Error(`housing.buyExtension`, `user`, user.UserId, `rollbackError`, err.Error())
		} else {
			publish(house)
		}
		say(`You're carrying too much to take a sheet of paper. Impressive. Put something down.`)
		return
	}
	publish(next)
	fromGold, fromBank := chargeGold(user, price)
	events.AddToQueue(events.EquipmentChange{UserId: user.UserId, GoldChange: -fromGold, BankChange: -fromBank})
	events.AddToQueue(events.ItemOwnership{UserId: user.UserId, Item: deed, Gained: true})
	mudlog.Info(`housing.buyExtension`, `user`, user.UserId, `building`, b.BuildingId, `price`, price, `roomsPaid`, next.RoomsPaid)

	say(fmt.Sprintf(`%d gold. Your name's on it, so don't bother selling it on. Stand in your lodging and type use deed. It'll ask you which way the room goes.`, price))
}

// buyRedecorate sells a redecorating voucher at the building's flat price.
func buyRedecorate(user *users.UserRecord, say func(string), buildingId string) {
	b, ok := GetBuilding(buildingId)
	if !ok {
		return
	}
	if _, owns := HouseOf(user.UserId, b.BuildingId); !owns {
		say(`A voucher's no use without a room to spend it on. Get a home first.`)
		return
	}
	if user.Character.Gold+user.Character.Bank < b.RedecoratePrice {
		say(fmt.Sprintf(`It's %d gold for a voucher. You haven't got it.`, b.RedecoratePrice))
		return
	}
	voucher := items.New(b.RedecorateItemId)
	if voucher.ItemId == 0 {
		mudlog.Error(`housing.buyRedecorate`, `building`, b.BuildingId, `error`, `redecorate item does not exist`)
		return
	}
	if !user.Character.StoreItem(voucher) {
		say(`You're carrying too much to take a slip of paper. Put something down.`)
		return
	}
	fromGold, fromBank := chargeGold(user, b.RedecoratePrice)
	events.AddToQueue(events.EquipmentChange{UserId: user.UserId, GoldChange: -fromGold, BankChange: -fromBank})
	events.AddToQueue(events.ItemOwnership{UserId: user.UserId, Item: voucher, Gained: true})
	say(fmt.Sprintf(`%d gold. Use it in whichever room you want to look different, then write what you'd like it to look like. One room, one go.`, b.RedecoratePrice))
}

// publish replaces a house in the registry. The caller has already saved it.
func publish(h House) {
	mu.Lock()
	defer mu.Unlock()
	hh := h.clone()
	indexLocked(&hh)
}
