package rifts

// lost.go: what a rift keeps, and gives back to someone else.
//
// A player who dies in a rift loses every item they carry loose, all the
// gold they carry, and each worn piece at the profile's chance
// (losses.worn_chance). A player who logs out inside a rift loses what they
// carry loose (they are told when they next log in). Keys, quest items and
// tickets are never taken; rift-only things are purged as always.
//
// Every item taken (not gold) goes into the lost-items record, which
// modules/rifts persists (LostSnapshot, RestoreLost, SetLostChanged). The
// first search of a rubble pile has losses.find_chance percent to turn up 1
// to losses.find_max of them, chosen at random from the whole record; an
// item found is gone from the record, so nobody else can find it again.

import (
	"fmt"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// LossSpec is a profile's losses: what death and logging out inside cost,
// how many lost items the record keeps, and how often rubble gives one back.
type LossSpec struct {
	WornChance  int  `yaml:"worn_chance"`   // percent per worn item, on death
	GoldOnDeath bool `yaml:"gold_on_death"` // carried gold is lost on death
	Keep        int  `yaml:"keep"`          // most items the record holds; the oldest go first
	FindChance  int  `yaml:"find_chance"`   // percent of first rubble searches that turn some up
	FindMax     int  `yaml:"find_max"`      // 1 to this many per find

	Death  string `yaml:"death"`  // to the dead: %s what was kept (a list)
	Gold   string `yaml:"gold"`   // to the dead: %d gold kept
	Logout string `yaml:"logout"` // at the next login: %s what was kept
	Found  string `yaml:"found"`  // to the searcher: %s what turned up
}

func (l *LossSpec) validate() error {
	if l.WornChance < 0 || l.WornChance > 100 || l.FindChance < 0 || l.FindChance > 100 {
		return fmt.Errorf(`worn_chance and find_chance are percents`)
	}
	if l.Keep < 1 || l.FindMax < 1 {
		return fmt.Errorf(`keep and find_max must be at least 1`)
	}
	for name, s := range map[string]string{`death`: l.Death, `logout`: l.Logout, `found`: l.Found} {
		if strings.Count(s, `%s`) != 1 {
			return fmt.Errorf(`%s needs one %%s`, name)
		}
	}
	if l.GoldOnDeath && strings.Count(l.Gold, `%d`) != 1 {
		return fmt.Errorf(`gold needs one %%d`)
	}
	return nil
}

// LostItem is one item a player lost in a rift.
type LostItem struct {
	Item    items.Item `yaml:"item"`
	Owner   string     `yaml:"owner"`   // the character who lost it
	Profile string     `yaml:"profile"` // where
	Lost    string     `yaml:"lost"`    // when (RFC 3339, UTC)
	How     string     `yaml:"how"`     // death | logout
}

// LostState is the record as modules/rifts persists it.
type LostState struct {
	Items []LostItem `yaml:"items"`
}

// LostRecord is the lost-items record, a store of live items: the bauble
// catalog sweep walks it (bauble_sweep.go), so a lost bauble keeps its
// record while it waits to be found.
type LostRecord []LostItem

// WalkItems calls fn with a pointer to each item in the record.
func (r LostRecord) WalkItems(fn func(*items.Item)) {
	for i := range r {
		if r[i].Item.ItemId > 0 {
			fn(&r[i].Item)
		}
	}
}

// WalkLostItems walks the record (the bauble sweep's rifts source). Under
// the mud lock.
func WalkLostItems(fn func(*items.Item)) { lostItems.WalkItems(fn) }

var (
	lostItems     LostRecord
	onLostChanged func()
)

// SetLostChanged sets what is called whenever the record changes (the
// module saves it).
func SetLostChanged(fn func()) { onLostChanged = fn }

// LostSnapshot is a copy of the record.
func LostSnapshot() LostState {
	return LostState{Items: append([]LostItem(nil), lostItems...)}
}

// RestoreLost replaces the record (the module, at load).
func RestoreLost(st LostState) {
	lostItems = nil
	for _, li := range st.Items {
		if li.Item.ItemId != 0 {
			lostItems = append(lostItems, li)
		}
	}
}

// LostCount is how many items the record holds.
func LostCount() int { return len(lostItems) }

func lostChanged() {
	if onLostChanged != nil {
		onLostChanged()
	}
}

// keepable reports whether a rift may take itm: never a key, a quest item
// or a ticket (those belong to the player's story, not the Obelisk), and
// never a rift-only thing (purged instead).
func keepable(itm items.Item, riftOnly map[int]bool) bool {
	if riftOnly[itm.ItemId] {
		return false
	}
	// Bound to an account (a housing deed) or a house guest key: worthless
	// to anyone else, and not the Obelisk's to give away.
	if itm.BoundUserId != 0 || itm.HouseKeyOwner != 0 {
		return false
	}
	spec := itm.GetSpec()
	if spec.ItemId == 0 {
		return false
	}
	return spec.Type != items.Key && spec.Type != items.Service && spec.QuestToken == ``
}

// forfeit takes from u what the rift keeps: everything carried loose, and
// on death the gold and each worn item at the profile's chance. The items
// go into the record. It returns their names and the gold taken.
func forfeit(u *users.UserRecord, p *Profile, how string) (names []string, gold int) {
	if u == nil || p == nil || p.Losses == nil {
		return nil, 0
	}
	l := p.Losses
	riftOnly := riftOnlyItemIds()
	stamp := now().UTC().Format(time.RFC3339)
	take := func(itm items.Item) {
		lostItems = append(lostItems, LostItem{Item: itm, Owner: u.Character.Name, Profile: p.Id, Lost: stamp, How: how})
		names = append(names, itm.DisplayName())
		events.AddToQueue(events.ItemOwnership{UserId: u.UserId, Item: itm, Gained: false})
	}
	// Worn pieces first: a lost belt or component bag spills what it held
	// into the pack, and the pack goes next.
	var removed []items.Item
	if how == `death` {
		for _, itm := range u.Character.GetAllWornItems() {
			if keepable(itm, riftOnly) && rng(100) < l.WornChance && u.Character.RemoveFromBody(itm) {
				take(itm)
				removed = append(removed, itm)
			}
		}
	}
	// Everything carried loose: the pack, the component bag, the bandolier.
	for _, itm := range u.Character.GetAllCarriedItems() {
		if keepable(itm, riftOnly) && u.Character.RemoveItem(itm) {
			take(itm)
		}
	}
	if how == `death` && l.GoldOnDeath && u.Character.Gold > 0 {
		gold = u.Character.Gold
		u.Character.Gold = 0
		events.AddToQueue(events.EquipmentChange{UserId: u.UserId, GoldChange: -gold})
	}
	if len(removed) > 0 {
		// As taking something off does: the body's stats and the client's
		// view of it follow.
		events.AddToQueue(events.EquipmentChange{UserId: u.UserId, ItemsRemoved: removed})
		_ = u.Character.Validate()
	}
	if over := len(lostItems) - l.Keep; over > 0 {
		lostItems = append(LostRecord(nil), lostItems[over:]...)
	}
	if len(names) > 0 {
		lostChanged()
	}
	if len(names) > 0 || gold > 0 {
		mudlog.Info(`rifts.lost`, `user`, u.UserId, `profile`, p.Id, `how`, how, `items`, len(names), `gold`, gold)
	}
	return names, gold
}

// forfeitOnDeath is OnPlayerDeath's part: take, and tell the dead.
func forfeitOnDeath(u *users.UserRecord, p *Profile) {
	names, gold := forfeit(u, p, `death`)
	if len(names) > 0 {
		u.SendText(messaging.CategoryDeath, wrap(fmt.Sprintf(p.Losses.Death, listNames(names))))
	}
	if gold > 0 {
		u.SendText(messaging.CategoryDeath, wrap(fmt.Sprintf(p.Losses.Gold, gold)))
	}
}

// The notice a player who logged out inside a rift gets at their next login.
func logoutNoticeKey() string { return `rift-logout-losses` }

// forfeitOnLogout is OnPlayerDespawn's part: take, and leave word for the
// next login.
func forfeitOnLogout(u *users.UserRecord, p *Profile) {
	names, _ := forfeit(u, p, `logout`)
	if len(names) > 0 {
		u.Character.SetMiscData(logoutNoticeKey(), wrap(fmt.Sprintf(p.Losses.Logout, listNames(names))))
	}
}

// tellLogoutLosses shows (once) what a logout inside a rift cost.
func tellLogoutLosses(u *users.UserRecord) {
	if s, _ := u.Character.GetMiscData(logoutNoticeKey()).(string); s != `` {
		u.Character.SetMiscData(logoutNoticeKey(), nil)
		u.SendText(messaging.CategorySystem, s)
	}
}

// findLost is a rubble search's chance at lost things: at the profile's
// find chance, 1 to find_max items, chosen at random from the record and
// taken out of it for good, are given to u.
func findLost(u *users.UserRecord, p *Profile) []string {
	if u == nil || p == nil || p.Losses == nil || len(lostItems) == 0 || rng(100) >= p.Losses.FindChance {
		return nil
	}
	n := 1 + rng(p.Losses.FindMax)
	var names []string
	for i := 0; i < n && len(lostItems) > 0; i++ {
		k := rng(len(lostItems))
		itm := lostItems[k].Item
		// What a merchant once marked as stolen was lost and found since:
		// the finder is no thief.
		itm.StolenFrom, itm.StolenFromMob, itm.StolenBy, itm.StolenAt, itm.StolenZone, itm.StolenSeen = ``, 0, 0, 0, ``, false
		if !u.Character.StoreItem(itm) {
			// No room to carry it: it stays lost, for whoever comes next
			// (a rift room's floor is gone once the room is).
			break
		}
		lostItems = append(lostItems[:k:k], lostItems[k+1:]...)
		events.AddToQueue(events.ItemOwnership{UserId: u.UserId, Item: itm, Gained: true})
		names = append(names, itm.DisplayName())
	}
	if len(names) == 0 {
		return nil
	}
	lostChanged()
	u.SendText(messaging.CategoryLoot, wrap(fmt.Sprintf(p.Losses.Found, listNames(names))))
	return names
}

func listNames(names []string) string {
	switch len(names) {
	case 0:
		return ``
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], `, `) + ` and ` + names[len(names)-1]
}

// linkdead reports whether u's connection has dropped and the character is
// only waiting in the world (a zombie connection).
func linkdead(u *users.UserRecord) bool {
	return u != nil && users.IsZombieConnection(connections.ConnectionId(u.ConnectionId()))
}
