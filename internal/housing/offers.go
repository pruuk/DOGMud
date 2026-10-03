package housing

import (
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
	OfferContainer  = `container`
	OfferStrongbox  = `strongbox`
	OfferBed        = `bed`
	OfferStation    = `station`
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
	_, homeFrozen := Frozen(b.BuildingId)
	switch {
	case owns:
		home.Price, home.Note = 0, `You already lodge here`
	case homeFrozen:
		home.Price, home.Note = 0, `Not letting rooms just now`
	case !b.welcomes(user.UserId):
		home.Note = capitalise(b.VouchedBy) + ` must vouch for you first`
	case vacant-buildingOutstanding(b) < tier.Rooms:
		home.Price, home.Note = 0, `Every room is let`
	default:
		home.Note, home.Available = b.Line(`list.home_note`), true
	}

	ext := Offer{Key: OfferExtension, Name: itemName(b.ExtensionItemId, `Room Extension Deed`)}
	_, isFrozen := Frozen(b.BuildingId)
	switch {
	case !owns:
		ext.Note = `Needs a home here first`
	case house.Outstanding(b) > 0:
		ext.Note = `You have one not yet used (lost? buy deed)`
	case isFrozen:
		ext.Note = `Not selling rooms just now`
	case len(house.RoomIds) >= b.MaxRooms:
		ext.Note = `Your lodging is as big as the company allows`
	case vacant <= buildingOutstanding(b):
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

	key := Offer{Key: OfferGuestKey, Name: itemName(b.GuestKeyItemId, `Guest Key`)}
	switch {
	case !owns:
		key.Note = `Needs a home here first`
	case len(house.Guests) >= b.MaxGuests:
		key.Note = `Your lock knows as many guests as allowed`
	default:
		key.Price = b.GuestKeyPrice
		key.Note, key.Available = `Lets one friend in. Give it to them`, true
	}

	box := Offer{Key: OfferContainer, Name: itemName(b.ContainerItemId, `Container Deed`)}
	safe := Offer{Key: OfferStrongbox, Name: itemName(b.StrongboxItemId, `Strongbox Deed`)}
	switch {
	case !owns:
		box.Note = `Needs a home here first`
		safe.Note = box.Note
	case house.containerSpace(b) <= carriedContainerDeeds(user, b):
		box.Note = `No free corner left for another`
		safe.Note = box.Note
	default:
		box.Price, safe.Price = b.ContainerPrice, b.StrongboxPrice
		box.Note, box.Available = `A container you name. Guests can use it`, true
		safe.Note, safe.Available = `A container only you can open`, true
	}

	return append([]Offer{home, ext, deco, key, box, safe}, furnishingOffers(user, b, house, owns)...)
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
	words := []string{}
	for _, word := range strings.Fields(strings.ToLower(request)) {
		word = strings.Trim(word, `.,!?"'()`)
		if word != `strongbox` {
			word = strings.TrimSuffix(word, `s`)
		}
		words = append(words, word)
	}
	// The container and furnishing words win over "deed", so "container
	// deed", "bed deed" and "deed for a strongbox" are not read as an
	// extension deed.
	for _, word := range words {
		switch word {
		case `bed`, `mattress`, `bedding`, `cot`:
			return OfferBed, true
		case `station`, `crafting`, `craft`, `workbench`, `workshop`, `forge`, `loom`, `bench`:
			return OfferStation, true
		}
	}
	for _, word := range words {
		switch word {
		case `strongbox`, `strongboxe`, `lockable`, `locked`, `lock`, `private`, `safe`:
			return OfferStrongbox, true
		}
	}
	for _, word := range words {
		switch word {
		case `container`, `box`, `storage`:
			return OfferContainer, true
		}
	}
	for _, word := range words {
		switch word {
		case `home`, `lodging`, `lease`:
			return OfferHome, true
		case `deed`, `extension`, `extend`, `addition`:
			return OfferExtension, true
		case `voucher`, `redecorating`, `redecorate`, `redecoration`, `decorating`, `description`:
			return OfferRedecorate, true
		case `key`, `guest`:
			return OfferGuestKey, true
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
	case OfferGuestKey:
		buyGuestKey(user, say, buildingId)
	case OfferContainer:
		buyContainerDeed(user, say, buildingId, false)
	case OfferStrongbox:
		buyContainerDeed(user, say, buildingId, true)
	case OfferBed, OfferStation:
		buyFurnishing(user, say, buildingId, key)
	}
}

// buyContainerDeed sells a container deed (a strongbox deed when ownerOnly)
// at the building's flat price. It is not bound: it can only be used inside
// its holder's own lodging anyway.
func buyContainerDeed(user *users.UserRecord, say func(string), buildingId string, ownerOnly bool) {
	b, ok := GetBuilding(buildingId)
	if !ok {
		return
	}
	itemId, price, what := b.ContainerItemId, b.ContainerPrice, `container`
	if ownerOnly {
		itemId, price, what = b.StrongboxItemId, b.StrongboxPrice, `strongbox`
	}
	house, owns := HouseOf(user.UserId, b.BuildingId)
	if !owns {
		say(b.Line(`storage.no_home`, `what`, what))
		return
	}
	if space := house.containerSpace(b); space <= carriedContainerDeeds(user, b) {
		if space == 0 {
			say(b.Line(`storage.full`))
		} else {
			say(b.Line(`storage.carrying`))
		}
		return
	}
	if user.Character.Gold+user.Character.Bank < price {
		say(b.Line(`storage.no_gold`, `price`, price, `what`, what))
		return
	}
	deed := items.New(itemId)
	if deed.ItemId == 0 {
		mudlog.Error(`housing.buyContainerDeed`, `building`, b.BuildingId, `error`, what+` item does not exist`)
		return
	}
	if !user.Character.StoreItem(deed) {
		say(b.Line(`storage.too_heavy`))
		return
	}
	fromGold, fromBank := chargeGold(user, price)
	events.AddToQueue(events.EquipmentChange{UserId: user.UserId, GoldChange: -fromGold, BankChange: -fromBank})
	events.AddToQueue(events.ItemOwnership{UserId: user.UserId, Item: deed, Gained: true})
	mudlog.Info(`housing.buyContainerDeed`, `user`, user.UserId, `building`, b.BuildingId, `ownerOnly`, ownerOnly, `price`, price)
	if ownerOnly {
		say(b.Line(`storage.strongbox_sold`, `price`, price))
	} else {
		say(b.Line(`storage.container_sold`, `price`, price))
	}
}

// buyExtension sells an extension deed made out to this account. What was
// paid is added to the house's room spend, and the deed counted as issued,
// BEFORE the deed is handed over or any gold is taken, so the next deed is
// priced up even if this one is never used, and a failed write costs nothing.
//
// A deed is a promise of one room, so none is sold that could not be kept:
// not while this lodger still has an unused deed (a lost one is replaced
// free instead), not past max_rooms counting unused deeds, and not unless the
// building has a vacant unit for every unused deed of every lodger.
func buyExtension(user *users.UserRecord, say func(string), buildingId string) {
	b, ok := GetBuilding(buildingId)
	if !ok {
		return
	}
	house, owns := HouseOf(user.UserId, b.BuildingId)
	if !owns {
		say(b.Line(`deed.no_home`))
		return
	}
	if _, isFrozen := Frozen(b.BuildingId); isFrozen {
		say(b.Line(`deed.frozen`))
		return
	}
	if house.Outstanding(b) > 0 {
		for _, itm := range user.Character.Items {
			if itm.ItemId == b.ExtensionItemId && itm.BoundUserId == user.UserId {
				say(b.Line(`deed.unused`))
				return
			}
		}
		// Sold, not used, and not carried: lost, stored or junked. Replace
		// it free. Whichever copy is used first spends the one promise.
		deed := items.New(b.ExtensionItemId)
		if deed.ItemId == 0 {
			mudlog.Error(`housing.buyExtension`, `building`, b.BuildingId, `error`, `extension item does not exist`)
			return
		}
		deed.BoundUserId = user.UserId
		if !user.Character.StoreItem(deed) {
			say(b.Line(`deed.too_heavy`))
			return
		}
		events.AddToQueue(events.ItemOwnership{UserId: user.UserId, Item: deed, Gained: true})
		mudlog.Info(`housing.buyExtension`, `user`, user.UserId, `building`, b.BuildingId, `reissued`, true)
		say(b.Line(`deed.reissued`))
		return
	}
	if len(house.RoomIds) >= b.MaxRooms {
		say(b.Line(`deed.max_rooms`))
		return
	}
	price := extensionPrice(b, house)
	if user.Character.Gold+user.Character.Bank < price {
		say(b.Line(`deed.no_gold`, `price`, price))
		return
	}

	deed := items.New(b.ExtensionItemId)
	if deed.ItemId == 0 {
		mudlog.Error(`housing.buyExtension`, `building`, b.BuildingId, `error`, `extension item does not exist`)
		return
	}
	deed.BoundUserId = user.UserId

	mu.Lock()
	cur, still := ownerHouse[ownerKey{b.BuildingId, user.UserId}]
	if !still {
		mu.Unlock()
		return
	}
	if len(vacantUnitsLocked(b.BuildingId)) <= outstandingLocked(b) {
		mu.Unlock()
		say(b.Line(`deed.no_units`))
		return
	}
	next := cur.clone()
	next.RoomsPaid = cur.Spent() + price
	next.DeedsIssued = cur.DeedsIssued + 1
	if err := saveHouse(next); err != nil {
		mu.Unlock()
		mudlog.Error(`housing.buyExtension`, `user`, user.UserId, `error`, err.Error())
		say(b.Line(`ledger_error`))
		return
	}
	if !user.Character.StoreItem(deed) {
		// Nothing was taken yet: put the ledger back as it was.
		if err := saveHouse(*cur); err != nil {
			mudlog.Error(`housing.buyExtension`, `user`, user.UserId, `rollbackError`, err.Error())
			nn := next.clone()
			indexLocked(&nn) // the file says sold; keep memory in step with it
		}
		mu.Unlock()
		say(b.Line(`deed.too_heavy`))
		return
	}
	nn := next.clone()
	indexLocked(&nn)
	mu.Unlock()

	fromGold, fromBank := chargeGold(user, price)
	events.AddToQueue(events.EquipmentChange{UserId: user.UserId, GoldChange: -fromGold, BankChange: -fromBank})
	events.AddToQueue(events.ItemOwnership{UserId: user.UserId, Item: deed, Gained: true})
	saveUser(user) // the ledger is on disk; the payment and the deed must be too
	mudlog.Info(`housing.buyExtension`, `user`, user.UserId, `building`, b.BuildingId, `price`, price, `roomsPaid`, next.RoomsPaid, `deedsIssued`, next.DeedsIssued)

	say(b.Line(`deed.sold`, `price`, price))
}

// carriedContainerDeeds counts the container and strongbox deeds of this
// building in a player's backpack: bought, not yet placed.
func carriedContainerDeeds(user *users.UserRecord, b Building) int {
	n := 0
	for _, itm := range user.Character.Items {
		if itm.ItemId == b.ContainerItemId || itm.ItemId == b.StrongboxItemId {
			n++
		}
	}
	return n
}

func buildingOutstanding(b Building) int {
	mu.RLock()
	defer mu.RUnlock()
	return outstandingLocked(b)
}

// outstandingLocked is every unused extension deed of every lodger in a
// building: each is a promise of one vacant unit.
func outstandingLocked(b Building) int {
	n := 0
	for _, h := range houses {
		if h.BuildingId == b.BuildingId {
			n += h.Outstanding(b)
		}
	}
	return n
}

// buyRedecorate sells a redecorating voucher at the building's flat price.
func buyRedecorate(user *users.UserRecord, say func(string), buildingId string) {
	b, ok := GetBuilding(buildingId)
	if !ok {
		return
	}
	if _, owns := HouseOf(user.UserId, b.BuildingId); !owns {
		say(b.Line(`voucher.no_home`))
		return
	}
	if user.Character.Gold+user.Character.Bank < b.RedecoratePrice {
		say(b.Line(`voucher.no_gold`, `price`, b.RedecoratePrice))
		return
	}
	voucher := items.New(b.RedecorateItemId)
	if voucher.ItemId == 0 {
		mudlog.Error(`housing.buyRedecorate`, `building`, b.BuildingId, `error`, `redecorate item does not exist`)
		return
	}
	if !user.Character.StoreItem(voucher) {
		say(b.Line(`voucher.too_heavy`))
		return
	}
	fromGold, fromBank := chargeGold(user, b.RedecoratePrice)
	events.AddToQueue(events.EquipmentChange{UserId: user.UserId, GoldChange: -fromGold, BankChange: -fromBank})
	events.AddToQueue(events.ItemOwnership{UserId: user.UserId, Item: voucher, Gained: true})
	say(b.Line(`voucher.sold`, `price`, b.RedecoratePrice))
}
