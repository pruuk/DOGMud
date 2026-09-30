package usercommands

import (
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/housing"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// A housing landlord (internal/housing) is not a shop: what he sells is
// priced per lodger. These helpers let list, buy and use treat him like one.

type landlordHere struct {
	mob      *mobs.Mob
	building housing.Building
}

// landlordsIn returns every awake landlord in the room with his building.
func landlordsIn(room *rooms.Room) []landlordHere {
	out := []landlordHere{}
	for _, instId := range room.GetMobs() {
		mob := mobs.GetInstance(instId)
		if mob == nil || actions.TargetAsleep(&mob.Character) {
			continue
		}
		for _, b := range housing.LandlordBuildings(int(mob.MobId)) {
			out = append(out, landlordHere{mob: mob, building: b})
		}
	}
	return out
}

func homeTierId(b housing.Building) string {
	if len(b.Tiers) == 0 {
		return ``
	}
	return b.Tiers[0].TierId
}

// renderHousingListings shows each landlord's list in the shop table style.
// It reports whether anything was listed.
func renderHousingListings(user *users.UserRecord, room *rooms.Room) bool {
	listed := false
	for _, l := range landlordsIn(room) {
		offers := housing.Offers(user, l.building.BuildingId, homeTierId(l.building))
		if len(offers) == 0 {
			continue
		}
		rows := make([][]string, 0, len(offers))
		// The footer names every offer's buy word, taken from the offers
		// themselves so it cannot fall behind the list.
		buyWords := make([]string, 0, len(offers))
		for _, o := range offers {
			buyWords = append(buyWords, `<ansi fg="command">buy `+o.Key+`</ansi>`)
		}
		for _, o := range offers {
			price := `-`
			if o.Price > 0 {
				price = strconv.Itoa(o.Price)
			}
			rows = append(rows, []string{o.Name, price, o.Note})
		}
		renderShopTable(user, `Lodgings and services`, `cyan`, l.mob.Character.Name, `mobname`,
			[]string{`Name`, `Price`, `Note`}, rows,
			`To buy, type: `+strings.Join(buyWords[:len(buyWords)-1], `, `)+` or `+buyWords[len(buyWords)-1]+`. Prices are in gold, and the bank counts.`)
		listed = true
	}
	return listed
}

// tryHousingBuy sells a landlord's offer when one is in the room and the
// request names one. Otherwise it returns false and the ordinary shop runs.
func tryHousingBuy(rest string, user *users.UserRecord, room *rooms.Room) bool {
	for _, l := range landlordsIn(room) {
		_, owns := housing.HouseOf(user.UserId, l.building.BuildingId)
		key, ok := housing.MatchOffer(rest, owns)
		if !ok {
			continue
		}
		// Same rule as every shop's list and buy: below the faces band you
		// cannot deal (lighting plan 5b).
		if actions.ShopSightRefusal(user.Character, room) {
			user.SendText(messaging.CategorySystem, actions.ShopSightRefusalText)
			return true
		}
		mob := l.mob
		housing.Buy(user, func(line string) { mob.Command(`say ` + line) }, l.building.BuildingId, homeTierId(l.building), key)
		return true
	}
	return false
}

// tryHousingItemUse handles "use deed north" and "use voucher [text]". The
// item name is the shortest leading run of words that names a housing item
// in the backpack; the rest are its arguments. Anything else returns false.
func tryHousingItemUse(rest string, user *users.UserRecord, room *rooms.Room) bool {
	// If the whole text already names an ordinary item ("use room key"), it is
	// that item, even if a prefix would also match a deed.
	if whole, found := user.Character.FindInBackpack(rest); found && !housing.IsHousingItem(whole.ItemId) {
		return false
	}
	// Longest name first, so "use container deed mug" is the container deed
	// named mug, not a "container" item given the words "deed mug".
	words := strings.Fields(rest)
	for i := len(words); i >= 1; i-- {
		itm, found := user.Character.FindInBackpack(strings.Join(words[:i], ` `))
		if !found || !housing.IsHousingItem(itm.ItemId) {
			continue
		}
		return housing.UseItem(user, room, itm, strings.Join(words[i:], ` `), rest)
	}
	return false
}

// routedPickKey holds a destination chosen by `visit` for the next routed
// exit this player takes, so `visit alice` can skip the menu.
const routedPickKey = `housing.routedPick`

// pickRoutedDestination chooses between a routed exit's destinations: a
// room pre-picked by `visit`, else a menu (home first, then each host). The
// prompt re-runs the go command with each answer, so it reports done=false
// until the player has answered. A zero room with done=true is a cancel.
func pickRoutedDestination(user *users.UserRecord, rest string, choices []rooms.ExitChoice) (roomId int, done bool) {
	if v, ok := user.GetTempData(routedPickKey).(int); ok && v != 0 {
		user.SetTempData(routedPickKey, nil)
		for _, c := range choices {
			if c.RoomId == v {
				return v, true
			}
		}
	}
	options := make([]string, 0, len(choices)+1)
	for _, c := range choices {
		options = append(options, c.Label)
	}
	options = append(options, `cancel`)

	cmdPrompt, _ := user.StartPrompt(`go`, rest)
	q := cmdPrompt.Ask(`Whose lodging?`, options, `cancel`)
	if !q.Done {
		return 0, false
	}
	user.ClearPrompt()
	for _, c := range choices {
		if strings.EqualFold(c.Label, q.Response) {
			return c.RoomId, true
		}
	}
	return 0, true
}
