package baubles

import (
	"slices"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The catalog's side of a bauble's life after it is found: taken from a
// household (MarkStolen), noted as left in one (MarkHousehold), or gone
// because nobody took it (MarkVanished). The game side is in
// internal/actions (household_bauble.go) and internal/rooms (the untaken
// sweep).

// Theft is who took a household's bauble, and from whom.
type Theft struct {
	ByUserId int
	RoomId   int    // the household's room
	FromMob  int    // mob id of the resident who was watching; 0 if nobody was
	FromName string // that resident's name
	Faction  string // the resident's first faction, if any
	Zone     string // the zone the theft happened in (rooms' Zone name)
}

// MarkStolen records that a bauble was stolen. It returns false when there
// is no such record.
func MarkStolen(id string, t Theft, at time.Time) bool {
	_, ok := Update(id, func(r *Record) {
		r.Stolen = true
		r.StolenByUserId = t.ByUserId
		r.StolenFromRoom = t.RoomId
		r.StolenFromMob = t.FromMob
		r.StolenFromName = t.FromName
		r.StolenFaction = t.Faction
		r.StolenAt = at.UTC()
		r.StolenZone = t.Zone
		r.GivenToMob = 0
	})
	if ok {
		mudlog.Info(`baubles`, `action`, `stolen`, `id`, id, `byUserId`, t.ByUserId, `roomId`, t.RoomId, `fromMob`, t.FromMob)
	}
	return ok
}

// MarkGiven records that a player gave the bauble to a mob (by mob id)
// that does not own it. It returns false when there is no such record.
func MarkGiven(id string, mobId int) bool {
	_, ok := Update(id, func(r *Record) { r.GivenToMob = mobId })
	return ok
}

// GivenTo reports whether the bauble was given to the mob mobId by a
// player and not stolen since: it is not that mob's own.
func (r Record) GivenTo(mobId int) bool {
	return mobId > 0 && r.GivenToMob == mobId
}

// MarkHousehold records that a find was left in the room because it belongs
// to the household there.
func MarkHousehold(id string) bool {
	_, ok := Update(id, func(r *Record) { r.Household = true })
	return ok
}

// MarkVanished records that a bauble lay untaken for BaubleUntakenHours and
// was removed from the world. It returns false when there is no such record.
func MarkVanished(id string, at time.Time) bool {
	_, ok := Update(id, func(r *Record) { r.VanishedAt = at.UTC() })
	if ok {
		mudlog.Info(`baubles`, `action`, `vanished`, `id`, id)
	}
	return ok
}

// UntakenLimit is how long a found bauble may lie untaken before it
// vanishes (Balance.BaubleUntakenHours).
func UntakenLimit() time.Duration {
	return time.Duration(configs.GetBalanceConfig().BaubleUntakenHours) * time.Hour
}

// HeatDuration is how long a bauble stays hot after it is stolen
// (Balance.BaubleStolenHeatHours).
func HeatDuration() time.Duration {
	return time.Duration(configs.GetBalanceConfig().BaubleStolenHeatHours) * time.Hour
}

// Hot reports whether the record is a stolen bauble whose theft is recent:
// stolen less than HeatDuration before now, and not given back to its owner
// since. Its owner may recognise it wherever they meet while it is hot.
// Where it may be sold or stored is HotIn: only in the area of the theft.
func (r Record) Hot(now time.Time) bool {
	if !r.StolenGoods() {
		return false // never stolen, or given back since the theft
	}
	return now.Before(r.StolenAt.Add(HeatDuration()))
}

// StolenGoods reports whether the record is stolen and has not been given
// back to its owner since its latest theft: what a fence pays its premium
// for.
func (r Record) StolenGoods() bool {
	return r.Stolen && !r.StolenAt.IsZero() && r.ReturnedAt.Before(r.StolenAt)
}

// HotIn reports whether the record is hot (Hot) in zone: the theft is
// recent and zone is in the same heat area as the zone it was stolen in
// (HeatArea). There, honest merchants, storage and the auction house refuse
// it; anywhere else it is just a trinket. A record whose theft zone is not
// known is hot everywhere while it is hot.
func (r Record) HotIn(zone string, now time.Time) bool {
	if !r.Hot(now) {
		return false
	}
	return r.StolenZone == `` || HeatArea(r.StolenZone) == HeatArea(zone)
}

// HeatArea is the heat area zone belongs to: the name of the group in
// Balance.BaubleHeatAreas that lists it (a city made of several zones), or
// else the zone itself. Matching ignores case and surrounding space.
func HeatArea(zone string) string {
	zone = strings.TrimSpace(zone)
	for area, zones := range configs.GetBalanceConfig().BaubleHeatAreas {
		for _, z := range zones {
			if strings.EqualFold(strings.TrimSpace(z), zone) {
				return `area:` + strings.ToLower(strings.TrimSpace(area))
			}
		}
	}
	return `zone:` + strings.ToLower(zone)
}

// RecognizedSinceTheft reports whether the owner has already recognised the
// bauble since its latest theft. Recognition happens once per theft.
func (r Record) RecognizedSinceTheft() bool {
	return !r.RecognizedAt.IsZero() && !r.RecognizedAt.Before(r.StolenAt)
}

// ItemIsHotIn reports whether itm is hot now in zone: a bauble whose record
// is (Record.HotIn), or stolen goods from a merchant's chest (GoodsHotIn,
// goods.go). Anything else, or a bauble with no record, is not.
func ItemIsHotIn(itm items.Item, zone string, now time.Time) bool {
	if !itm.IsBauble() {
		return GoodsHotIn(itm, zone, now)
	}
	rec, ok := Get(itm.Bauble)
	return ok && rec.HotIn(zone, now)
}

// MarkRecognized records that the bauble's owner recognised it on someone.
func MarkRecognized(id string, byUserId int, at time.Time) bool {
	_, ok := Update(id, func(r *Record) { r.RecognizedAt = at.UTC() })
	if ok {
		mudlog.Info(`baubles`, `action`, `recognized`, `id`, id, `carrierUserId`, byUserId)
	}
	return ok
}

// MarkReturned records that the bauble was given back to its owner, which
// cools it. credited lists the factions whose reputation the return earned
// its thief byUserId; empty when it earned nothing. A record earns credit
// once only: a later return never replaces the first credit.
func MarkReturned(id string, byUserId int, credited []string, at time.Time) bool {
	_, ok := Update(id, func(r *Record) {
		r.ReturnedAt = at.UTC()
		if len(credited) > 0 && r.ReturnCreditAt.IsZero() {
			r.ReturnCreditUserId = byUserId
			r.ReturnCreditFactions = append([]string(nil), credited...)
			r.ReturnCreditAt = at.UTC()
			r.ReturnCreditRound = util.GetRoundCount()
		}
	})
	if ok {
		mudlog.Info(`baubles`, `action`, `returned`, `id`, id, `byUserId`, byUserId, `credited`, credited)
	}
	return ok
}

// ReturnCredits counts the returns that have earned userId reputation with
// faction since game round sinceRound (the round of the oldest catch the
// faction still holds against them, so returns and catches are counted over
// the same stretch: a sentence served clears both). The count sets the next
// return's share of a catch (actions.returnShare), so that every
// BaubleReturnsPerCatch returns earn back exactly one catch, and caps it at
// what open catches have cost. It reads only that thief's credited records,
// through the catalog's index.
func ReturnCredits(userId int, faction string, sinceRound uint64) int {
	cat.mu.RLock()
	defer cat.mu.RUnlock()
	count := 0
	for id := range cat.credits[userId] {
		r, ok := cat.records[id]
		if !ok || r.ReturnCreditAt.IsZero() || r.ReturnCreditRound < sinceRound {
			continue
		}
		if slices.Contains(r.ReturnCreditFactions, faction) {
			count++
		}
	}
	return count
}
