package actions

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Search's bauble tier (docs/baubles, Phases 3 and 4).
//
// A find is not handed over on the spot. The search takes a moment longer:
// the player is told they have spotted something and are working it loose,
// and the bauble arrives once it has been named. Naming may be a call to the
// model, so it runs on a goroutine that does NOT hold the mud lock; the
// result is applied under the lock, once, when it is ready. The wait is at
// least BaubleRevealSeconds whether or not the model is used, so a player
// cannot tell a model-named find from a fallback one by its timing, and
// at most the generator's own timeout (hard-capped by baubles.MaxGenerateTime).
//
// The catalog record is created only when the text is final: there is never
// a placeholder or "unexamined" bauble.
//
// Rules this tier keeps from actions/search.go:
//
//   - Offering a roll never makes a room a progression candidate: a roll that
//     finds nothing awards nothing (the secret-exit farm, in search.go).
//   - A FIND trains search: it counts as a won search (see awardSearch in
//     search.go). The chance grows with search skill (BaubleSkillFactor) and
//     with where the search is (buildings high, wilderness low), so a find is
//     partly the searcher's doing. At a few percent per roll, two rolls per
//     room per hour, finds are too rare to farm.
//   - Its find is in FoundAnything, so the player is not also told "You find
//     nothing of interest".
//   - A spent window, an excluded room and a failed roll are all silent and
//     identical.

// searchBaubleRoll is the roll itself. A variable so tests can stand in for
// the dice.
var searchBaubleRoll = func(o baubles.FindOpts) (baubles.ValueTier, bool) {
	return baubles.RollFind(o)
}

// startBaubleDelivery runs a delivery. Production runs it on its own
// goroutine, which takes the mud lock to finish; tests replace it to run
// in line.
var startBaubleDelivery = func(d BaubleDelivery) {
	// Tracked here, under the lock the caller holds, before the goroutine
	// exists: a copyover in the same pass of the game loop, before the
	// goroutine has even started, still finds it to finish.
	id, p := trackFind(d, d.dice())
	go func() {
		defer func() {
			if r := recover(); r != nil {
				mudlog.Error(`baubles`, `action`, `deliver`, `panic`, r)
			}
		}()
		d.runTracked(id, p, true)
	}()
}

// dice is the delivery's random source: Randn, or util.Rand.
func (d BaubleDelivery) dice() func(n int) int {
	if d.Randn != nil {
		return d.Randn
	}
	return util.Rand
}

// baubleRecipient is whoever a delivery goes to, resolved at delivery time
// because the finder may have moved or logged off meanwhile.
type baubleRecipient struct {
	char *characters.Character
	room *rooms.Room
	send func(text string)
}

// findBaubleRecipient resolves an online player. A variable so tests can
// deliver to a fake actor.
var findBaubleRecipient = func(userId int) (baubleRecipient, bool) {
	u := users.GetByUserId(userId)
	if u == nil || u.Character == nil {
		return baubleRecipient{}, false
	}
	room := rooms.LoadRoom(u.Character.RoomId)
	if room == nil {
		return baubleRecipient{}, false
	}
	return baubleRecipient{
		char: u.Character,
		room: room,
		send: func(text string) { u.SendText(messaging.CategorySystem, text) },
	}, true
}

// findBaubleRoom is the room a find is left in when its finder is gone.
var findBaubleRoom = rooms.LoadRoom

// BaubleDelivery is one find on its way to a player: what to ask for, who
// found it, and where.
type BaubleDelivery struct {
	Request  baubles.GenRequest
	UserId   int
	MinDelay time.Duration
	Randn    func(n int) int // the fallback's dice; nil means util.Rand

	// Spot is where the find lies if it is left in the room it was found in
	// ("on the bookshelf"); empty for a search of the whole room.
	Spot string

	// Household is a find rolled as a household's (one of the household was
	// about when the search was made), so its tier came from the household
	// weights, which lean richer. It stays the household's at delivery even
	// if nobody of the household is about by then: a richer find always has
	// to be stolen.
	Household bool
}

// baubleNow is the clock for when a find is left lying. A variable for
// tests.
var baubleNow = time.Now

// pendingFind is one delivery on its way: tracked from the moment it
// starts until it is delivered, so a copyover or shutdown can finish it
// (FlushBaubleDeliveries) rather than lose a find the player was already
// told about ("Something glints...").
type pendingFind struct {
	d      BaubleDelivery
	randn  func(n int) int
	ctx    context.Context
	cancel context.CancelFunc

	mu     sync.Mutex
	res    *baubles.GenResult // the naming, once it has come back
	claimd bool               // delivered, or being delivered, by someone
}

var pendingFinds = struct {
	sync.Mutex
	next uint64
	m    map[uint64]*pendingFind
}{m: map[uint64]*pendingFind{}}

func trackFind(d BaubleDelivery, randn func(n int) int) (uint64, *pendingFind) {
	ctx, cancel := context.WithCancel(context.Background())
	p := &pendingFind{d: d, randn: randn, ctx: ctx, cancel: cancel}
	pendingFinds.Lock()
	defer pendingFinds.Unlock()
	pendingFinds.next++
	pendingFinds.m[pendingFinds.next] = p
	return pendingFinds.next, p
}

func untrackFind(id uint64) {
	pendingFinds.Lock()
	p := pendingFinds.m[id]
	delete(pendingFinds.m, id)
	pendingFinds.Unlock()
	if p != nil {
		p.cancel()
	}
}

// claim makes the caller the one who delivers it; false when someone did.
func (p *pendingFind) claim() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.claimd {
		return false
	}
	p.claimd = true
	return true
}

func (p *pendingFind) setResult(res baubles.GenResult) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.res = &res
}

func (p *pendingFind) result() (baubles.GenResult, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.res == nil {
		return baubles.GenResult{}, false
	}
	return *p.res, true
}

// PendingBaubleDeliveries is how many finds are on their way.
func PendingBaubleDeliveries() int {
	pendingFinds.Lock()
	defer pendingFinds.Unlock()
	return len(pendingFinds.m)
}

// FlushBaubleDeliveries finishes every find still on its way, now: named if
// its naming has come back, otherwise from the fallback corpus, as it would
// have been had the model not answered. Call it under the mud lock, before
// rooms and players are saved, at copyover and at shutdown: the process is
// about to end, and a delivery goroutine cannot finish without the lock.
// Each delivery's naming is cancelled, and its goroutine, should the
// process go on (a copyover that aborts), finds it delivered and stops.
// Returns how many it delivered.
func FlushBaubleDeliveries() int {
	pendingFinds.Lock()
	list := make([]*pendingFind, 0, len(pendingFinds.m))
	for _, p := range pendingFinds.m {
		list = append(list, p)
	}
	pendingFinds.Unlock()
	sort.Slice(list, func(a, b int) bool { return list[a].d.UserId < list[b].d.UserId })

	n := 0
	for _, p := range list {
		if !p.claim() {
			continue
		}
		p.cancel()
		res, ok := p.result()
		if !ok {
			res = baubles.FallbackFor(p.d.Request, p.randn)
		}
		p.d.deliver(res, p.randn)
		n++
	}
	if n > 0 {
		mudlog.Info(`baubles`, `action`, `flush deliveries`, `delivered`, n)
	}
	return n
}

// run names the find (blocking, off the lock), waits out the rest of
// MinDelay, then mints and delivers it. lock says whether to take the mud
// lock for the last step; false only for tests that already run in line.
// It is tracked the whole way, so FlushBaubleDeliveries can finish it
// instead, in which case this one stands down.
func (d BaubleDelivery) run(lock bool) {
	id, p := trackFind(d, d.dice())
	d.runTracked(id, p, lock)
}

// runTracked is run for a delivery already tracked (startBaubleDelivery
// tracks it before its goroutine starts).
func (d BaubleDelivery) runTracked(id uint64, p *pendingFind, lock bool) {
	randn := p.randn
	defer untrackFind(id)
	start := time.Now()
	res := baubles.Generate(p.ctx, d.Request, randn)
	p.setResult(res)
	if wait := d.MinDelay - time.Since(start); wait > 0 {
		select {
		case <-time.After(wait):
		case <-p.ctx.Done():
		}
	}
	if lock {
		util.LockMud()
		defer util.UnlockMud()
	}
	if !p.claim() {
		return // finished by FlushBaubleDeliveries
	}
	d.deliver(res, randn)
}

// deliver mints the named find and hands it over. Runs under the mud lock.
//
//   - Found in a household (an indoor room with a resident about, checked
//     now, or a find rolled as a household's at the search): it stays where
//     it was found, on the feature searched, and belongs to the household;
//     taking it is theft (household_bauble.go).
//   - Otherwise into the finder's pack; at their feet if they cannot carry
//     it; onto the floor where it was found if they have logged off.
//
// Anything left lying is marked with its spot and the time, and vanishes if
// nobody takes it within BaubleUntakenHours (rooms' untaken sweep).
func (d BaubleDelivery) deliver(res baubles.GenResult, randn func(n int) int) {
	// Where it goes is settled before anything is minted: a finder who has
	// gone, from a room that can no longer be loaded, leaves it nowhere, and
	// a record for a find that exists nowhere would be a ghost.
	who, online := findBaubleRecipient(d.UserId)
	foundRoom := findBaubleRoom(d.Request.Place.RoomId)
	if !online && foundRoom == nil {
		mudlog.Info(`baubles`, `action`, `deliver`, `userId`, d.UserId, `result`, `finder offline and the room is gone; nothing minted`, `roomId`, d.Request.Place.RoomId)
		return
	}

	itm, rec, err := baubles.Mint(baubles.MintOpts{
		Source:       d.Request.Source,
		Place:        d.Request.Place,
		FinderUserId: d.UserId,
		Tier:         d.Request.Tier,
		FoundIn:      d.Request.Container,
		Result:       &res,
		Randn:        randn,
	})
	if err != nil {
		mudlog.Error(`baubles`, `action`, `deliver`, `userId`, d.UserId, `error`, err)
		if online {
			who.send(`Whatever it was crumbles away as you pull it free. You come up empty-handed.`)
		}
		return
	}

	now := baubleNow()
	// Every line naming it goes to the finder alone (who.send): their own
	// view of a finder-only bauble. The room line below names no item.
	name := itm.DisplayNameFor(d.UserId)

	// A household keeps what is found in it. A find rolled as a household's
	// stays theirs even with nobody of the household about now: its richer
	// tier is for stealing, not for picking up.
	if foundRoom != nil {
		resident, ok := HouseholdResident(foundRoom)
		if ok || d.Household {
			itm.LeaveBaubleAt(d.Spot, foundRoom.RoomId, now)
			foundRoom.AddItem(itm, false)
			baubles.MarkHousehold(rec.Id)
			residentName := ``
			if resident != nil {
				residentName = resident.Character.Name
			}
			mudlog.Info(`baubles`, `action`, `deliver`, `id`, rec.Id, `result`, `household; left in room`, `roomId`, foundRoom.RoomId, `resident`, residentName, `rolledAsHousehold`, d.Household)
			if online {
				where := itm.BaubleSpotSuffix() // " (on the bookshelf)", its own colour
				if who.room != nil && who.room.RoomId == foundRoom.RoomId {
					if resident != nil {
						who.send(fmt.Sprintf(`You uncover <ansi fg="itemname">%s</ansi>%s. It belongs to this household, and <ansi fg="mobname">%s</ansi> is close by, so you leave it where it lies.`, name, where, resident.Character.Name))
					} else {
						who.send(fmt.Sprintf(`You uncover <ansi fg="itemname">%s</ansi>%s. It belongs to this household, so you leave it where it lies.`, name, where))
					}
				} else {
					who.send(fmt.Sprintf(`You had uncovered <ansi fg="itemname">%s</ansi>%s, but it belongs to that household, so you left it where it lay.`, name, where))
				}
			}
			return
		}
	}

	if !online {
		itm.LeaveBaubleAt(d.Spot, 0, now)
		foundRoom.AddItem(itm, false)
		mudlog.Info(`baubles`, `action`, `deliver`, `id`, rec.Id, `result`, `finder offline; left in room`, `roomId`, d.Request.Place.RoomId)
		return
	}

	if who.char.StoreItem(itm) {
		events.AddToQueue(events.ItemOwnership{UserId: d.UserId, Item: itm, Gained: true})
		who.send(fmt.Sprintf(`You work it free: <ansi fg="itemname">%s</ansi>. You pocket it.`, name))
	} else {
		// The spot only means something in the room it was found in.
		spot := ``
		if who.room.RoomId == d.Request.Place.RoomId {
			spot = d.Spot
		}
		itm.LeaveBaubleAt(spot, 0, now)
		who.room.AddItem(itm, false)
		who.send(fmt.Sprintf(`You work it free: <ansi fg="itemname">%s</ansi>. You are carrying too much to take it, so you leave it%s.`, name, leftWhere(spot)))
	}
	who.room.SendTextVisual(messaging.CategoryMobEmote,
		fmt.Sprintf(`<ansi fg="username">%s</ansi> turns up something small.`, who.char.Name),
		d.UserId,
	)
}

// leftWhere finishes "you leave it...": " on the bookshelf", or " on the
// ground" with no spot.
func leftWhere(spot string) string {
	if spot == `` {
		return ` on the ground`
	}
	return ` ` + spot
}

// BaublePlace is the room as the bauble catalog records it: id, zone, the
// zone's region (the zone again when it has none) and the biome.
func BaublePlace(room *rooms.Room) baubles.Place {
	region := ``
	if zc := rooms.GetZoneConfig(room.Zone); zc != nil {
		region = zc.Region
	}
	biome := ``
	if b := room.GetBiome(); b != nil {
		biome = b.BiomeId
	}
	return baubles.NewPlace(room.RoomId, room.Zone, region, biome)
}

const (
	maxPromptDescription = 900 // characters of room description sent to the model
	maxPromptNouns       = 12
	maxRecentNames       = 12
)

// BaubleRequest is the generation request for a find in this room, built
// under the mud lock. It copies AUTHORED text only: the room's title,
// description and noun keys. Signs (player writing), player names and
// anything a player typed never go to the model. container is what was
// searched; empty means the room itself.
func BaubleRequest(room *rooms.Room, tier baubles.ValueTier, source baubles.Source, container string) baubles.GenRequest {
	nouns := make([]string, 0, len(room.Nouns))
	for n := range room.Nouns {
		nouns = append(nouns, n)
	}
	sort.Strings(nouns)
	if len(nouns) > maxPromptNouns {
		nouns = nouns[:maxPromptNouns]
	}
	desc := strings.TrimSpace(room.Description)
	if len(desc) > maxPromptDescription {
		desc = desc[:maxPromptDescription]
	}
	timeOfDay := `day`
	if gametime.IsNight() {
		timeOfDay = `night`
	}
	return baubles.GenRequest{
		Tier:            tier,
		Source:          source,
		Place:           BaublePlace(room),
		RoomTitle:       room.Title,
		RoomDescription: desc,
		RoomNouns:       nouns,
		Container:       container,
		TimeOfDay:       timeOfDay,
		RecentNames:     baubles.RecentNames(room.Zone, maxRecentNames),
	}
}

// StartBaubleFind names and delivers one find of the given tier to a player,
// in the background. Search calls it on a successful roll; the admin
// `bauble spawn` command calls it to exercise the same path. Call under the
// mud lock.
func StartBaubleFind(userId int, room *rooms.Room, tier baubles.ValueTier, source baubles.Source) {
	startBaubleFind(userId, room, tier, source, SearchFeature{}, false)
}

// startBaubleFind is StartBaubleFind for a find in one feature of the room
// (search_feature.go); an empty feature means the room itself. household is
// a find rolled as a household's (BaubleDelivery.Household).
func startBaubleFind(userId int, room *rooms.Room, tier baubles.ValueTier, source baubles.Source, feature SearchFeature, household bool) {
	req := BaubleRequest(room, tier, source, feature.Name)
	req.ContainerDescription = feature.Description
	req.FinderUserId = userId
	startBaubleDelivery(BaubleDelivery{
		Request:   req,
		UserId:    userId,
		MinDelay:  baubles.RevealDelay(),
		Spot:      feature.Spot(),
		Household: household,
	})
}

// householdFind reports whether a find in room now would be the
// household's: the room is indoors and one of the household is about. It is
// asked at the search, so the find's tier can come from the household
// weights (baubles.FindOpts.Household).
func householdFind(room *rooms.Room) bool {
	_, ok := HouseholdResident(room)
	return ok
}

// BaubleSkillFactor is how much a searcher's search skill raises their
// bauble chance, from 0 (untrained) to 1 (at SkillSoftCap). It is
// combat.SkillMultiplier's curve (the square root of rank over the soft cap,
// so early ranks count for the most, with its own fallbacks), read as a
// fraction of the way from SkillMultiplierBase to SkillMultiplierMax. See
// baubles.ChanceFor.
func BaubleSkillFactor(char *characters.Character) float64 {
	if char == nil {
		return 0
	}
	bal := configs.GetBalanceConfig()
	base, top := float64(bal.SkillMultiplierBase), float64(bal.SkillMultiplierMax)
	if top <= base {
		return 0 // a flat curve: skill raises nothing
	}
	return (combat.SkillMultiplier(char.GetSkillLevel(skills.Search)) - base) / (top - base)
}

// baubleRoomAllowed rules out the rooms that never offer baubles whatever
// the config says: instance and other temporary rooms (they are rebuilt, and
// a paid instance must not become a bauble farm), banks, storage rooms,
// character rooms, and private rooms (housing lodgings: bought, safe, and
// furnished with as many searchable containers as the owner likes, so a farm
// for the same reason). Excluded zones are checked by the baubles package.
func baubleRoomAllowed(room *rooms.Room) bool {
	if room.IsEphemeral() || room.IsBank || room.IsStorage || room.IsCharacterRoom {
		return false
	}
	if rooms.IsPrivateRoom(room.RoomId) {
		return false
	}
	if rooms.GetInstanceRegistry().FindByRoomId(room.RoomId) != nil {
		return false
	}
	return true
}

// searchForBauble rolls the bauble tier for a player's search. On a find it
// tells the player they have spotted something and starts the delivery; the
// bauble itself arrives when it has been named.
func searchForBauble(actor Actor, room *rooms.Room) bool {
	if !baubleRoomAllowed(room) {
		return false
	}
	household := householdFind(room)
	tier, found := searchBaubleRoll(baubles.FindOpts{
		Place:       BaublePlace(room),
		UserId:      actor.GetUserId(),
		SkillFactor: BaubleSkillFactor(actor.GetCharacter()),
		// sight ramp (plan 5b): the searcher needs to see what glints.
		SightPenalty: 1 - messaging.SightMult(actor.GetCharacter(), room),
		Household:    household,
	})
	if !found {
		return false
	}
	actor.SendText(messaging.CategorySystem,
		`Something glints among the clutter. You set about working it loose...`)
	startBaubleFind(actor.GetUserId(), room, tier, baubles.SourceSearch, SearchFeature{}, household)
	return true
}
