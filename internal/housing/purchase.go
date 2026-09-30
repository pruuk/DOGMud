package housing

import (
	"fmt"
	"time"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/factions"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/opinions"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// PurchaseResult is the outcome of one attempt to buy a house.
type PurchaseResult int

const (
	PurchaseOk PurchaseResult = iota
	PurchaseUnknownBuilding
	PurchaseUnknownTier
	PurchaseAlreadyOwner
	PurchaseRepTooLow
	PurchaseNoGold
	PurchaseNoVacancy
	PurchaseError
)

// repTierFor is the buyer's standing with a faction. A seam for tests, which
// have no faction store.
var repTierFor = factions.TierFor

// SetRepTierForTest replaces the standing lookup and returns a restore func.
func SetRepTierForTest(fn func(factionId string, userId int) opinions.Tier) func() {
	prev := repTierFor
	repTierFor = fn
	return func() { repTierFor = prev }
}

// now is the purchase timestamp source; a seam for tests.
var now = time.Now

// Purchase runs the landlord's side of buying a house: standing, one house per
// account per building, a vacant unit, the price (carried gold first, then the
// bank, as paying a fine does), and the record. The landlord speaks every
// refusal through say. The house is written to disk BEFORE gold is taken or
// the registry changes, so a failed write costs the player nothing and a
// crash after it never leaves a paid-for room unrecorded.
func Purchase(user *users.UserRecord, say func(string), buildingId string, tierId string) PurchaseResult {
	b, ok := GetBuilding(buildingId)
	if !ok {
		say(`I've no rooms to let just now.`)
		return PurchaseUnknownBuilding
	}
	tier, ok := b.Tier(tierId)
	if !ok {
		say(`I don't let that kind of room.`)
		return PurchaseUnknownTier
	}

	if _, owns := HouseOf(user.UserId, b.BuildingId); owns {
		say(fmt.Sprintf(`You've already got a room. The %s's behind me. It knows you. Go on.`, b.DoorExit))
		return PurchaseAlreadyOwner
	}

	if repTierFor(b.Faction, user.UserId) < b.MinTier() {
		say(`The Widow only lets to people the quarter vouches for, and nobody's vouched for you. Do some good round the Common Quarter and come back. I'll still be here. I'm always here.`)
		return PurchaseRepTooLow
	}

	if user.Character.Gold+user.Character.Bank < tier.Price {
		say(fmt.Sprintf(`It's %d gold for %s, and you haven't got it, not even counting the bank. Come back when you have.`, tier.Price, tier.Name))
		return PurchaseNoGold
	}

	// Hold the registry lock across choose, write and publish, so two buyers
	// in the same round cannot be handed the same room.
	mu.Lock()
	if _, owns := ownerHouse[ownerKey{b.BuildingId, user.UserId}]; owns {
		mu.Unlock()
		say(`You've already got a room.`)
		return PurchaseAlreadyOwner
	}
	if heldOwners[ownerKey{b.BuildingId, user.UserId}] {
		mu.Unlock()
		say(`Your name's in the ledger already, but the page is in a state. The Widow's clerk is going over it. Nothing I can let you till it's straight.`)
		return PurchaseAlreadyOwner
	}
	if _, isFrozen := frozen[b.BuildingId]; isFrozen {
		mu.Unlock()
		say(`The ledger's in a state. The Widow's clerk is going over it, and I'm to let nothing till he's done. Come back another day.`)
		return PurchaseNoVacancy
	}
	vacant := vacantUnitsLocked(b.BuildingId)
	if len(vacant) < tier.Rooms {
		mu.Unlock()
		say(`Every room's let. Nothing I can do. Try another day.`)
		return PurchaseNoVacancy
	}
	h := House{
		BuildingId:  b.BuildingId,
		OwnerUserId: user.UserId,
		OwnerName:   user.Character.Name,
		TierId:      tier.TierId,
		RoomIds:     append([]int{}, vacant[:tier.Rooms]...),
		PricePaid:   tier.Price,
		RoomsPaid:   tier.Price,
		PurchasedAt: now().UTC(),
	}
	// A new lodger's rooms start empty: whatever an old instance save or a
	// previous tenancy left in them is not theirs.
	for _, roomId := range h.RoomIds {
		h.Floors = append(h.Floors, HouseFloor{RoomId: roomId})
	}
	if err := saveHouse(h); err != nil {
		mu.Unlock()
		mudlog.Error(`housing.Purchase`, `user`, user.UserId, `building`, b.BuildingId, `error`, err.Error())
		say(`The ledger's in a state. I'll not take your coin till it's straight. Come back later.`)
		return PurchaseError
	}
	hh := h
	indexLocked(&hh)
	mu.Unlock()

	fromGold, fromBank := chargeGold(user, tier.Price)
	events.AddToQueue(events.EquipmentChange{UserId: user.UserId, GoldChange: -fromGold, BankChange: -fromBank})
	saveUser(user) // the house is on disk; the payment must be too
	for _, roomId := range h.RoomIds {
		if roomLoaded(roomId) {
			ApplyOverlay(liveRoom(roomId))
		}
	}

	mudlog.Info(`housing.Purchase`, `user`, user.UserId, `character`, user.Character.Name, `building`, b.BuildingId, `tier`, tier.TierId, `rooms`, fmt.Sprint(h.RoomIds), `price`, tier.Price)

	paid := fmt.Sprintf(`You count out <ansi fg="gold">%d gold</ansi>`, tier.Price)
	if fromBank > 0 && fromGold > 0 {
		paid = fmt.Sprintf(`You count out <ansi fg="gold">%d gold</ansi> and sign over <ansi fg="gold">%d</ansi> more from the bank`, fromGold, fromBank)
	} else if fromBank > 0 {
		paid = fmt.Sprintf(`You sign over <ansi fg="gold">%d gold</ansi> from the bank`, fromBank)
	}
	say(fmt.Sprintf(`Stamped. Welcome to %s. Try not to set anything on fire.`, b.Name))
	user.SendText(messaging.CategorySystem, util.SplitStringNL(fmt.Sprintf(
		`%s. %s writes your name into the ledger, then takes your hand and presses it flat to the brass plate of the <ansi fg="exit">%s</ansi>. Something in the lock ticks, once, as if it has learned you.`,
		paid, landlordName(b), b.DoorExit), 80))
	user.SendText(messaging.CategorySystem, util.SplitStringNL(fmt.Sprintf(
		`You now have a room in %s. Type <ansi fg="command">enter %s</ansi> to go in, and <ansi fg="command">list</ansi> here for extensions and redecorating.`, b.Name, b.DoorExit), 80))
	return PurchaseOk
}

// chargeGold takes price from carried gold first, then the bank. The caller
// has already checked the two together cover it.
func chargeGold(user *users.UserRecord, price int) (fromGold int, fromBank int) {
	fromGold = price
	if user.Character.Gold < fromGold {
		fromGold = user.Character.Gold
	}
	fromBank = price - fromGold
	user.Character.Gold -= fromGold
	user.Character.Bank -= fromBank
	return fromGold, fromBank
}
