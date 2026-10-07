package lightnotice

import (
	"math"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Trigger names why a check is running, which decides what it may announce.
type Trigger uint8

const (
	// TriggerMove runs after a player arrives in a room. It announces only a
	// darker band or dazzle: walking into light needs no notice, the room
	// description already says it. This is the move rule; decide applies it
	// to ANY trigger that finds the room already changed, not only this one,
	// so a check that happens to run between MoveToRoom and the queued
	// RoomChange listener cannot announce a lighter band as if it were a
	// non-movement cause.
	TriggerMove Trigger = iota
	// TriggerCombatRound runs once per combat round the player is fighting in.
	// A room change seen under this trigger still obeys the move rule above.
	TriggerCombatRound
	// TriggerCommand runs before every command the player issues, so the
	// notice lands before the command's own output. A room change seen under
	// this trigger still obeys the move rule above.
	TriggerCommand
	// TriggerQuiet records the current band and never speaks: login. Waking
	// and the end of blindness are NOT this trigger: they are handled by the
	// quiet flag decide sets on the record, which a per-round attention sweep
	// sets when a player falls asleep or is blinded, so the next attentive
	// check (whatever trigger it runs under) resyncs silently.
	TriggerQuiet
)

// observation is one moment of one player's light, gathered by Check.
type observation struct {
	roomId  int
	band    messaging.Band
	terms   rooms.LightTerms
	indoor  bool
	asleep  bool
	blinded bool
	// bandAt reports the band the observer's CURRENT sight reads at a given
	// light level. attribute uses it to ask whether the old light alone would
	// already give the new band, in which case the observer's sight changed,
	// not the light. A nil bandAt skips that check entirely (it is optional
	// so decide's other rules stay testable without it).
	bandAt func(light int) messaging.Band
}

// record is what was last announced (or silently recorded) to a player.
type record struct {
	roomId int
	band   messaging.Band
	terms  rooms.LightTerms
	// quiet says the player was asleep or blinded since the last record, so
	// the next attentive check re-records silently. Their end must never read
	// as the light changing.
	quiet bool
}

// notice is what decide asks Check to say.
type notice struct {
	cause      Cause
	transition Transition
	indoor     bool
}

// decide applies the trigger rules. It returns the notice to send, if any, and
// the record to store. It is pure so every rule is table-testable.
//
// The move rule (a lighter band needs no notice) keys off the ROOM having
// changed, not off TriggerMove itself: any trigger that runs after the room
// id has already changed, such as a combat round or command check that lands
// between MoveToRoom and the queued RoomChange listener, sees the same
// crossing TriggerMove would and must suppress it the same way.
func decide(prev record, known bool, now observation, trigger Trigger) (notice, bool, record) {
	if now.asleep || now.blinded {
		// On a player's very first check, known is false and prev is the zero
		// record (room 0, BandDark). If that first check also finds them
		// asleep or blinded, this returns that zero record, only marked
		// quiet: it is a placeholder, not a claim they are actually in room 0
		// in the dark. The next attentive check overwrites it silently, same
		// as any other quiet resync.
		prev.quiet = true
		return notice{}, false, prev
	}
	next := record{roomId: now.roomId, band: now.band, terms: now.terms}
	if !known || prev.quiet || trigger == TriggerQuiet || now.band == prev.band {
		return notice{}, false, next
	}
	tr := transitionOf(prev.band, now.band)
	// The move rule (walking into better light needs no notice) applies
	// whenever the room actually changed, not only under TriggerMove: a
	// combat-round or command check that runs after MoveToRoom but before
	// the queued RoomChange listener also sees the new room and must not
	// announce a lighter band with the movement cause.
	if (trigger == TriggerMove || now.roomId != prev.roomId) && (tr == LighterShapes || tr == LighterFaces) {
		return notice{}, false, next
	}
	return notice{cause: attribute(prev, now), transition: tr, indoor: now.indoor}, true, next
}

// transitionOf names a band change. Bands run darkest to brightest. It
// assumes from != to; decide filters an unchanged band before calling it.
func transitionOf(from, to messaging.Band) Transition {
	switch {
	case to == messaging.BandDazzled:
		return IntoDazzle
	case to > from && to == messaging.BandFaces:
		return LighterFaces
	case to > from:
		return LighterShapes
	case to == messaging.BandFaces:
		return DarkerFaces
	case to == messaging.BandShapes:
		return DarkerShapes
	}
	return DarkerDark
}

// attribute names the likeliest cause of a band change.
//
// A room change is movement. Otherwise it asks a counterfactual: would the
// OLD light, read through the observer's CURRENT sight, already give the NEW
// band? If so the light never had to move; the observer's own sight did (a
// draught wearing off or taking hold), and that is checked BEFORE the terms.
//
// A plain direction test (did Level move the way the band moved) is not
// enough: the sky drifts a little almost every round, and when it drifts the
// same direction as an eyes-caused change, a direction test blames the sky
// for a change the observer's sight alone already explains. The
// counterfactual does not have that failure mode, because it holds the light
// fixed at its OLD value and only varies the sight.
//
// Otherwise, the first term that moved, in the order darkness (carried or a
// fixture's), carried light (present, or its strength), the room's own light
// (its lamp and its light fixtures), weather, sky. Darkness comes first
// (lighting plan 5d, ruling D4): a darkness arriving or lapsing is the
// deliberate act in the room, and a light that trims around it is not.
func attribute(prev record, now observation) Cause {
	if prev.roomId != now.roomId {
		return CauseMovement
	}
	a, b := prev.terms, now.terms
	if now.bandAt != nil && now.bandAt(prev.terms.Level) == now.band {
		return CauseEyes
	}
	switch {
	case a.Darkened != b.Darkened || termMoved(a.Dark, b.Dark):
		return CauseDarkness
	// A carried light arriving or leaving, and one that changes while lit (a
	// lantern its schedule dims, a second light joining one already here)
	// (lighting 5e, X5).
	case a.Carried != b.Carried || termMoved(a.CarriedLight, b.CarriedLight):
		return CauseCarried
	// The room's own light: its lamp, and its fixtures (lighting 5e).
	case a.HasLamp != b.HasLamp || a.Lamp != b.Lamp || termMoved(a.Fixture, b.Fixture):
		return CauseLamp
	// Exact comparison is safe while the shipped fractions are 0.5 and 0.7:
	// their products are identical in any order. Two different fractions
	// that are not powers of two could differ by float noise if the active
	// mutator list reordered.
	case a.SkyFilter != b.SkyFilter:
		return CauseWeather
	case termMoved(a.Sky, b.Sky):
		return CauseSky
	}
	return CauseEyes
}

// termMoved reports whether a light-scale term changed: appeared, went out
// (0 since lighting plan 6, when a combine of nothing reads 0; Absent is still
// accepted from a caller that passes one),
// or moved by more than float noise. The sky, the darkness, the carried light
// and the fixtures share it.
func termMoved(a, b float64) bool {
	aAbsent, bAbsent := math.IsInf(a, -1), math.IsInf(b, -1)
	if aAbsent || bAbsent {
		return aAbsent != bAbsent
	}
	return math.Abs(a-b) > 1e-9
}

// Per-player state, in memory only: cleared on logout, never saved.
//
// sentBands is the band last handed to GMCP (Char.Sight), kept apart from
// records because a record deliberately keeps its old band while its player
// sleeps or is blinded (decide), and the Game-window border must still follow
// what LightBand reads (lighting plan 5d, ruling D7).
var (
	mu        sync.Mutex
	records   = map[int]record{}
	sentBands = map[int]messaging.Band{}
)

// Check compares the player's current band with the last one recorded for
// them and, if the trigger's rule allows, sends one notice.
func Check(user *users.UserRecord, trigger Trigger) {
	if user == nil || user.Character == nil {
		return
	}
	room := rooms.LoadRoom(user.Character.RoomId)
	if room == nil {
		return
	}
	now := observe(user.Character, room)

	mu.Lock()
	prev, known := records[user.UserId]
	n, speak, next := decide(prev, known, now, trigger)
	records[user.UserId] = next
	sent, hadSent := sentBands[user.UserId]
	bandMoved := !hadSent || sent != now.band
	if bandMoved {
		sentBands[user.UserId] = now.band
	}
	mu.Unlock()

	// The Game-window border (lighting plan 5d, ruling D7) rides this check,
	// which already computes the band on the 3d cadence: every command, every
	// combat round, every move, login, and the light commands. Only a change
	// is queued, so a repeat check costs GMCP nothing.
	if bandMoved {
		events.AddToQueue(events.SightBandChanged{UserId: user.UserId})
	}

	if !speak {
		return
	}
	if text, ok := line(n.cause, n.transition, n.indoor, nil); ok {
		user.SendText(messaging.CategoryLight, text)
	}
}

// NoteAttention marks a sleeping or blinded player so the first check after
// they wake or see again records silently. It computes no light and sends no
// text, so it is cheap enough to run for every player every round. It is the
// seam for waking: sleep ends at many hand-rolled sites and by expiry, and no
// event announces it.
func NoteAttention(user *users.UserRecord) {
	if user == nil || user.Character == nil || !inattentive(user.Character) {
		return
	}
	mu.Lock()
	r := records[user.UserId]
	r.quiet = true
	records[user.UserId] = r
	mu.Unlock()
}

// Forget drops a player's record, at logout.
func Forget(userId int) {
	mu.Lock()
	delete(records, userId)
	delete(sentBands, userId)
	mu.Unlock()
}

// ResetForTest unloads the store and clears every record.
func ResetForTest() {
	mu.Lock()
	records = map[int]record{}
	sentBands = map[int]messaging.Band{}
	mu.Unlock()
	loaded = nil
}

func inattentive(c *characters.Character) bool {
	return c.HasConditionFlag(conditions.Sleeping) ||
		(c.Perception != nil && c.Perception.State() == perception.Blinded)
}

// fixedLight hands LightBand the level LightTerms already computed, so band
// and terms come from one computation rather than two.
type fixedLight int

func (l fixedLight) LightLevel() int { return int(l) }

func observe(c *characters.Character, room *rooms.Room) observation {
	terms := room.LightTerms()
	indoor := false
	if b := room.GetBiome(); b != nil {
		indoor = b.Indoor
	}
	cfg := configs.GetLightingConfig()
	strength, reach := c.NightVisionStrength(), c.InfraReach()
	return observation{
		roomId:  room.RoomId,
		band:    messaging.LightBand(c, fixedLight(terms.Level)),
		terms:   terms,
		indoor:  indoor,
		asleep:  c.HasConditionFlag(conditions.Sleeping),
		blinded: c.Perception != nil && c.Perception.State() == perception.Blinded,
		bandAt: func(light int) messaging.Band {
			return messaging.BandThroughWindow(light, strength, reach, cfg.BlindBelow, cfg.DimBelow, cfg.DazzleAbove)
		},
	}
}
