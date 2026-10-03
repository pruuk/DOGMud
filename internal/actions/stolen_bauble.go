package actions

import (
	"fmt"
	"time"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/crimes"
	"github.com/GoMudEngine/GoMud/internal/factions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/merchantchests"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Stolen baubles after the theft (docs/baubles Phase 6c).
//
// A stolen bauble is hot for BaubleStolenHeatHours after its latest theft
// (baubles.Record.Hot), for selling and storing only in the area it was
// stolen in (baubles.Record.HotIn). Selling it is sell_bauble.go's (honest
// merchants there refuse a hot one; fences buy any stolen one); storage
// and the auction house there refuse a hot one in their own commands. This
// file is the owner's side, which follows the owner, not the area:
//
//   - Recognition. While a bauble is hot, the NPC it was taken from may
//     recognise it on whoever carries it, when either walks in on the other
//     (a RoomChange, hooks/RoomChange_StolenBaubleRecognition.go). It is a
//     contest: the owner's noticing score (stealVictimScore, their own eyes
//     included) against the thief's sleight of hand (the steal score:
//     Dexterity, skullduggery, hidden). Recognised, the thief is caught
//     exactly as a thief caught in the act (thiefCaught: the crime, its
//     reputation and bounty, and the owner attacks unless it cannot fight).
//     Once per theft, and only on the thief: anyone else carrying it is
//     never accused, so a thief can neither frame a bystander nor spend the
//     recognition on a friend.
//   - Return. Giving a stolen bauble back to its owner cools it. When the
//     giver is its thief, each of the owner's factions credits them with
//     1/BaubleReturnsPerCatch of the reputation a catch costs
//     (CrimeRepDeltaTheft), so that that many returns earn back one catch.
//     A return only earns back reputation actually lost: a faction credits
//     no more returns than BaubleReturnsPerCatch for every theft it caught
//     the thief at (the crimes log), so a thief who is never caught gains
//     nothing, and stealing and returning cannot farm reputation. A bauble
//     earns credit once ever.
//
// The owner is the mob TEMPLATE a bauble was lifted from (instance ids do
// not survive a respawn), so every instance of a template counts as it: a
// bauble lifted from one of several identical town guards is recognised by
// any of them.

// stolenNow is the clock for heat, recognition and returns. A variable for
// tests.
var stolenNow = time.Now

// Seams for tests, each the real thing on a server: the catch that follows
// recognition, and the reputation a return earns with the owner's
// factions.
var (
	stolenCaught   = thiefCaught
	stolenReported = theftReported
	returnRepBump  = factions.BumpRep
	ownerFactions  = factions.FactionsForMob
	theftCatches   = identifiedTheftCatches
)

// identifiedTheftCatches counts the thefts faction caught userId at and
// still holds against them: the UNRESOLVED theft crimes in its log naming
// them as the identified perpetrator. Each cost them CrimeRepDeltaTheft
// with that faction (thiefCaught). A resolved crime no longer counts:
// serving a sentence resolves them and restores reputation
// (justice.ClearFactionRecord), and a crime left long enough goes stale
// (crimes.PruneStale), so returns never earn back what was already given
// back or forgotten. since is the game round of the oldest of them: the
// returns counted against these catches are only those credited from then
// on (baubles.ReturnCredits), so returns from before a sentence served are
// not held against a thief caught again after it.
func identifiedTheftCatches(userId int, faction string) (n int, since uint64) {
	for _, c := range crimes.AllForFaction(faction, false) {
		if c.Kind == crimes.KindTheft && c.Perpetrator.Type == crimes.PerpPlayer && c.Perpetrator.Id == userId {
			if n == 0 || c.Round < since {
				since = c.Round
			}
			n++
		}
	}
	return n, since
}

// isBaubleOwner reports whether m is who rec was stolen from: the mob
// template it was lifted from, or (for a household's bauble taken with
// nobody of the household watching) one of that household who lives there
// (home room the house), at home. A passer-by in the house is not.
func isBaubleOwner(rec baubles.Record, m *mobs.Mob, room *rooms.Room) bool {
	if m == nil || !rec.Stolen {
		return false
	}
	if rec.StolenFromMob > 0 {
		return int(m.MobId) == rec.StolenFromMob
	}
	return rec.StolenFromRoom > 0 && room != nil && room.RoomId == rec.StolenFromRoom &&
		m.HomeRoomId == rec.StolenFromRoom && householdMember(m, room)
}

// canRecognize reports whether the owner is in a state to notice anything:
// alive, awake, and nobody's companion.
func canRecognize(m *mobs.Mob) bool {
	c := &m.Character
	return !c.IsDead() && !c.HasConditionFlag(conditions.Sleeping) && !c.IsCharmed()
}

// carrierScore is how well the carrier keeps a stolen bauble out of sight:
// the steal score (Dexterity plus skullduggery rank times SkillWeight, and
// the hidden bonus), without the carrier's own sight, which does not help
// them hide anything.
func carrierScore(c *characters.Character) float64 {
	cfg := configs.GetBalanceConfig()
	score := float64(c.Stats.Dexterity.ValueAdj) +
		float64(c.GetSkillLevel(skills.Skullduggery))*float64(cfg.SkillWeight)
	if c.IsHidden() {
		score += float64(cfg.StealHiddenBonus)
	}
	return score
}

// stolenRecognitionRoll is the recognition contest in room: the owner's
// noticing score (stealVictimScore, which pays the owner's own sight
// ramp) against the carrier's (carrierScore). True when the owner
// recognises it.
func stolenRecognitionRoll(owner *characters.Character, carrier *characters.Character, room *rooms.Room) bool {
	return combat.RunContest(stealVictimScore(owner, combat.SightRoom(room)),
		[]contest.Entry{{Score: carrierScore(carrier)}}).Success
}

// recognitionRoll is stolenRecognitionRoll. A variable so tests can set the
// dice.
var recognitionRoll = stolenRecognitionRoll

// stolenCarriers are the players who may be carrying a stolen bauble, as
// RecognizeStolenBaubles needs them. A variable so tests can stand in a
// fake actor.
var stolenCarriers = func(room *rooms.Room, userId int) []Actor {
	ids := room.GetPlayers()
	if userId > 0 {
		ids = []int{userId}
	}
	out := make([]Actor, 0, len(ids))
	for _, id := range ids {
		u := users.GetByUserId(id)
		if u == nil || u.Character == nil || u.Character.RoomId != room.RoomId {
			continue
		}
		out = append(out, &UserActor{User: u, Room: room})
	}
	return out
}

// RecognizeStolenBaubles gives the owners in a room their chance to
// recognise their stolen baubles, after someone has moved into it: with
// userId set, a player walked in, so only that player's baubles are looked
// at; with mobInstanceId set, a mob walked in, so only that mob looks.
// Call under the mud lock (the RoomChange listener).
func RecognizeStolenBaubles(roomId int, userId int, mobInstanceId int) {
	room := rooms.LoadRoom(roomId)
	if room == nil {
		return
	}
	recognizeIn(room, userId, mobInstanceId)
}

// recognizeIn is RecognizeStolenBaubles for a loaded room.
func recognizeIn(room *rooms.Room, userId int, mobInstanceId int) {
	owners := []*mobs.Mob{}
	if mobInstanceId > 0 {
		// The mob may have moved on again before the move was handled.
		if m := mobs.GetInstance(mobInstanceId); m != nil && m.Character.RoomId == room.RoomId {
			owners = append(owners, m)
		}
	} else {
		for _, id := range room.GetMobs() {
			if m := mobs.GetInstance(id); m != nil {
				owners = append(owners, m)
			}
		}
	}
	if len(owners) == 0 {
		return
	}

	now := stolenNow()
	for _, carrier := range stolenCarriers(room, userId) {
		recognizeOn(carrier, owners, room, now)
	}
}

// recognizeOn looks through what carrier carries for a hot bauble one of
// owners was robbed of, then for hot goods from a merchant's chest
// (recognizeGoodsOn). The first recognised is a catch, and the only one:
// the carrier is already caught.
func recognizeOn(carrier Actor, owners []*mobs.Mob, room *rooms.Room, now time.Time) {
	if recognizeBaublesOn(carrier, owners, room, now) {
		return
	}
	recognizeGoodsOn(carrier, owners, room, now)
}

// recognizeBaublesOn is recognizeOn for baubles; true on a catch.
func recognizeBaublesOn(carrier Actor, owners []*mobs.Mob, room *rooms.Room, now time.Time) bool {
	char := carrier.GetCharacter()
	for _, itm := range append([]items.Item(nil), char.Items...) {
		if !itm.IsBauble() {
			continue
		}
		rec, ok := baubles.Get(itm.Bauble)
		if !ok || !rec.Hot(now) || rec.RecognizedSinceTheft() {
			continue
		}
		if carrier.GetUserId() != rec.StolenByUserId {
			continue // only the thief is ever accused
		}
		for _, m := range owners {
			if !isBaubleOwner(rec, m, room) || !canRecognize(m) {
				continue
			}
			// The owner must see at least shapes, as a household resident
			// must (isResident): a blinded owner, or one in the dark,
			// recognises nothing, spends no recognition and attacks nobody.
			if !messaging.CanSeeShapes(&m.Character, room) {
				continue
			}
			if !recognitionRoll(&m.Character, char, room) {
				continue
			}
			ownerRecognizes(carrier, m, itm, room, now)
			return true
		}
	}
	return false
}

// Stolen goods from merchant chests (internal/merchantchests,
// sell_stolen.go) are recognised on the same terms as a stolen bauble:
// while hot (baubles.GoodsHot), once per theft (items.Item.StolenSeen), only
// on their thief (StolenBy), by a watcher awake and able to see shapes, and
// only by winning the same contest (recognitionRoll). Two kinds of watcher
// know them: the merchant they were taken from (its mob template,
// StolenFromMob), anywhere, as a bauble's owner does; and a town guard
// (mobs.IsGuardMob), or one of the catalog's lookouts
// (merchantchests.IsLookout: a dock constable, a gate warden), in the heat
// area they were taken in (baubles.GoodsHotIn), where word of the theft has
// spread. Worn or wielded goods count: the watcher sees them on the thief.
// Recognised by the merchant, the thief is caught as a thief caught in the
// act (thiefCaught). Recognised by a guard or lookout, the theft is reported
// (theftReported: its factions record it, with the reputation and bounty
// that brings) and the justice tick takes it from there; the guard does not
// simply start swinging.

// recognizeGoodsOn looks through everything carrier carries for hot
// merchant-chest goods one of watchers knows; true on a catch.
func recognizeGoodsOn(carrier Actor, watchers []*mobs.Mob, room *rooms.Room, now time.Time) bool {
	char := carrier.GetCharacter()
	for _, itm := range carriedGoods(char) {
		if itm.IsBauble() || !itm.IsStolen() || itm.StolenSeen || !baubles.GoodsHot(itm, now) {
			continue
		}
		if itm.StolenBy == 0 || carrier.GetUserId() != itm.StolenBy {
			continue // only the thief is ever accused
		}
		for _, m := range watchers {
			owner := itm.StolenFromMob > 0 && int(m.MobId) == itm.StolenFromMob
			watcher := mobs.IsGuardMob(m.Groups) || merchantchests.IsLookout(int(m.MobId))
			guard := !owner && watcher && baubles.GoodsHotIn(itm, room.Zone, now)
			if !owner && !guard {
				continue
			}
			if !canRecognize(m) || !messaging.CanSeeShapes(&m.Character, room) {
				continue
			}
			if !recognitionRoll(&m.Character, char, room) {
				continue
			}
			goodsRecognized(carrier, m, itm, owner, room)
			return true
		}
	}
	return false
}

// carriedGoods is everything char has on them: backpack, bandolier,
// component bag, and worn or wielded gear.
func carriedGoods(char *characters.Character) []items.Item {
	out := make([]items.Item, 0, len(char.Items)+len(char.PotionItems)+len(char.ComponentItems)+8)
	out = append(out, char.Items...)
	out = append(out, char.PotionItems...)
	out = append(out, char.ComponentItems...)
	for _, p := range char.Equipment.GetAllItemPtrs() {
		out = append(out, *p)
	}
	return out
}

// markGoodsSeen sets StolenSeen on the carried item with itm's identity,
// wherever the carrier keeps it, worn slots included.
func markGoodsSeen(char *characters.Character, itm items.Item) {
	for _, pool := range [][]items.Item{char.Items, char.PotionItems, char.ComponentItems} {
		for i := range pool {
			if pool[i].Equals(itm) {
				pool[i].StolenSeen = true
				return
			}
		}
	}
	for _, p := range char.Equipment.GetAllItemPtrs() {
		if p.Equals(itm) {
			p.StolenSeen = true
			return
		}
	}
}

// goodsRecognized is m recognising stolen goods on carrier: said aloud,
// marked seen, and then the catch. The room hears the thief named only
// when m sees clearly, as for a bauble (ownerRecognizes).
func goodsRecognized(carrier Actor, m *mobs.Mob, itm items.Item, owner bool, room *rooms.Room) {
	markGoodsSeen(carrier.GetCharacter(), itm)
	name := itm.DisplayName()
	shout := `"That's mine! Thief!"`
	if !owner {
		shout = fmt.Sprintf(`"That was taken from %s. Thief!"`, ownerPhrase(itm.StolenFrom))
	}
	carrier.SendText(messaging.CategorySystem, fmt.Sprintf(
		`<ansi fg="mobname">%s</ansi> stares at the <ansi fg="itemname">%s</ansi> you are carrying. %s`,
		m.Character.Name, name, shout))
	who := `a figure`
	if messaging.CanSeeClearly(&m.Character, room) {
		who = fmt.Sprintf(`<ansi fg="username">%s</ansi>`, carrier.GetCharacter().Name)
	}
	room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(
		`<ansi fg="mobname">%s</ansi> points at %s. %s`,
		m.Character.Name, who, shout), carrier.GetUserId())
	mudlog.Info(`merchantchests`, `action`, `recognized`, `item`, itm.ItemId, `by`, m.Character.Name, `owner`, owner, `carrierUserId`, carrier.GetUserId())
	if owner {
		stolenCaught(carrier, m, room)
		return
	}
	stolenReported(carrier, m, room)
}

// stolenGoodsGiven handles a player having given stolen goods from a
// merchant's chest to m: given to the merchant they were taken from, they
// are its own again (every stolen mark cleared on the item it now holds).
// Unlike a bauble, a return earns no reputation back. Reports whether it
// was a return.
func stolenGoodsGiven(giver Actor, m *mobs.Mob, itm items.Item) bool {
	if itm.StolenFromMob <= 0 || int(m.MobId) != itm.StolenFromMob {
		return false
	}
	c := &m.Character
	for _, pool := range [][]items.Item{c.Items, c.PotionItems, c.ComponentItems} {
		for i := range pool {
			if pool[i].Equals(itm) {
				pool[i].ClearStolen()
			}
		}
	}
	name := itm.DisplayName()
	giver.SendText(messaging.CategorySystem, fmt.Sprintf(
		`<ansi fg="mobname">%s</ansi> snatches back the <ansi fg="itemname">%s</ansi>. "My %s! Where did you find that?"`,
		m.Character.Name, name, name))
	if room := giver.GetRoom(); room != nil {
		room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi> looks relieved to have the <ansi fg="itemname">%s</ansi> back.`,
			m.Character.Name, name), giver.GetUserId())
	}
	mudlog.Info(`merchantchests`, `action`, `returned`, `item`, itm.ItemId, `to`, m.Character.Name, `byUserId`, giver.GetUserId())
	return true
}

// ownerRecognizes is m recognising its stolen itm on carrier: said aloud,
// recorded, and then the catch. It follows the crime-witnessing tiers: an
// owner who sees clearly knows the thief (thiefCaught records them as the
// identified perpetrator); one who sees only shapes knows its bauble on a
// figure it cannot name, so the room hears no name, and thiefCaught's
// witness count records the crime against an unknown perpetrator.
func ownerRecognizes(carrier Actor, m *mobs.Mob, itm items.Item, room *rooms.Room, now time.Time) {
	baubles.MarkRecognized(itm.Bauble, carrier.GetUserId(), now)
	name := itm.DisplayName()
	carrier.SendText(messaging.CategorySystem, fmt.Sprintf(
		`<ansi fg="mobname">%s</ansi> stares at the <ansi fg="itemname">%s</ansi> you are carrying. "That's mine! Thief!"`,
		m.Character.Name, name))
	who := `a figure`
	if messaging.CanSeeClearly(&m.Character, room) {
		who = fmt.Sprintf(`<ansi fg="username">%s</ansi>`, carrier.GetCharacter().Name)
	}
	room.SendTextVisualHidingNames(messaging.CategoryMobEmote, fmt.Sprintf(
		`<ansi fg="mobname">%s</ansi> points at %s. "That's mine! Thief!"`,
		m.Character.Name, who), []string{carrier.GetCharacter().Name}, carrier.GetUserId())
	mudlog.Info(`baubles`, `action`, `recognized`, `id`, itm.Bauble, `owner`, m.Character.Name, `carrierUserId`, carrier.GetUserId())
	stolenCaught(carrier, m, room)
}

// returnShare is the reputation the next credited return earns, when prior
// returns have already been credited with this faction: the whole catch
// (catchRep) split into perCatch parts, the remainder spread so that every
// perCatch returns add up to exactly one catch (a catch of 5 in thirds
// pays 1, 2, 2).
func returnShare(prior int, catchRep int, perCatch int) int {
	if catchRep <= 0 || perCatch <= 0 || prior < 0 {
		return 0
	}
	return (prior+1)*catchRep/perCatch - prior*catchRep/perCatch
}

// StolenBaubleGiven handles a player having given itm to m (usercommands'
// give, after the transfer). When itm is a bauble stolen from m, it is a
// return: m says so, the bauble cools, and its thief may earn back some
// reputation (see the top of this file). Any other bauble a player gives a
// mob is marked as a gift (baubles.MarkGiven), so picking it back out of
// that mob's pocket does not make it the mob's stolen goods. It reports
// whether it was a return. Call under the mud lock.
func StolenBaubleGiven(giver Actor, m *mobs.Mob, itm items.Item) bool {
	if m != nil && !itm.IsBauble() && itm.IsStolen() {
		return stolenGoodsGiven(giver, m, itm)
	}
	if !itm.IsBauble() || m == nil {
		return false
	}
	rec, ok := baubles.Get(itm.Bauble)
	if !ok {
		return false
	}
	if !isBaubleOwner(rec, m, giver.GetRoom()) {
		if giver.IsPlayer() {
			baubles.MarkGiven(itm.Bauble, int(m.MobId))
		}
		return false
	}

	now := stolenNow()
	userId := giver.GetUserId()
	credited := []string{}
	if userId > 0 && userId == rec.StolenByUserId && rec.ReturnCreditAt.IsZero() {
		cfg := configs.GetBalanceConfig()
		catchRep := -int(cfg.CrimeRepDeltaTheft) // the delta is a loss
		perCatch := int(cfg.BaubleReturnsPerCatch)
		for _, fid := range ownerFactions(m) {
			catches, since := theftCatches(userId, fid)
			if catches == 0 {
				continue // nothing to earn back with this faction
			}
			prior := baubles.ReturnCredits(userId, fid, since)
			if prior >= perCatch*catches {
				continue // all of it earned back already
			}
			if share := returnShare(prior, catchRep, perCatch); share > 0 {
				returnRepBump(fid, userId, share)
			}
			// Counted even when the share rounds to nothing, so the
			// remainder lands on a later return.
			credited = append(credited, fid)
		}
	}
	baubles.MarkReturned(itm.Bauble, userId, credited, now)

	name := itm.DisplayName()
	giver.SendText(messaging.CategorySystem, fmt.Sprintf(
		`<ansi fg="mobname">%s</ansi> turns the <ansi fg="itemname">%s</ansi> over in disbelief. "My %s! I never thought to see it again."`,
		m.Character.Name, name, name))
	if room := giver.GetRoom(); room != nil {
		room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi> looks overjoyed to have the <ansi fg="itemname">%s</ansi> back.`,
			m.Character.Name, name), userId)
	}
	return true
}
