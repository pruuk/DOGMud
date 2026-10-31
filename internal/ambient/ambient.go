// Package ambient is the one gate every unprompted ("ambient") line passes
// on its way to players (#317). It owns the per-listener budget
// (Balance.AmbientListenerBudgetRounds), the once-per-visit rule for a room
// with a single idle line, the repeat limit (Balance.AmbientRepeatLimit per
// AmbientRepeatWindowRounds), and the telemetry that measures how much each
// room hears.
//
// A counted send site asks Plan who the line may reach and passes the
// returned Exclude to the room sender it already used, so sight gating,
// name hiding and categories stay exactly where they were. Sends that are
// not ambient never come here.
package ambient

import (
	"math"
	"slices"
	"strconv"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Source names one counted ambient source. The strings are telemetry keys
// tools/ambient_report.py reads, so they never change.
type Source string

const (
	SourceRoomIdle    Source = "room_idle"
	SourceMobChatter  Source = "mob_chatter"
	SourceWeather     Source = "weather"
	SourceGossip      Source = "gossip"
	SourceItemChatter Source = "item_chatter"
	// SourceSayEcho is "You hear someone talking." in a room next to an
	// ambient say, counted in the receiving room (owner 2026-10-30).
	SourceSayEcho Source = "say_echo"
)

// Sources lists every counted source, in report order.
var Sources = []Source{SourceRoomIdle, SourceMobChatter, SourceWeather, SourceGossip, SourceItemChatter, SourceSayEcho}

// ParseSource maps a source word, as the `ambient` mob command carries it,
// back to its Source.
func ParseSource(word string) (Source, bool) {
	for _, s := range Sources {
		if string(s) == word {
			return s, true
		}
	}
	return "", false
}

// Line is one ambient line about to be sent into a room.
type Line struct {
	Room   *rooms.Room
	Source Source
	// Perceives reports whether a player takes the line in at all (a visual
	// line reaches nobody who cannot see). nil means everyone present does,
	// as for a heard line. A player who does not perceive it is neither
	// charged nor counted.
	Perceives func(u *users.UserRecord) bool
	// Exclude lists players the caller already leaves out. They are never
	// charged or counted, and they come back in Routing.Exclude.
	Exclude []int
	// Key is the line's repeat-limit identity, from LineKey. Empty skips
	// the repeat limit (owner 2026-10-31).
	Key string
}

// Routing is Plan's answer. Exclude goes to the room sender as its
// excludeUserIds; Delivered is every player who takes the line in and was
// charged for it.
type Routing struct {
	Exclude   []int
	Delivered []int
}

// LineKey is an ambient line's repeat-limit identity: the kind of speaker
// (mob, room, item, weather), its template or room id, and the raw text
// before names are filled in, so two mobs of one template share it.
func LineKey(kind string, id int, text string) string {
	return kind + `:` + strconv.Itoa(id) + `:` + text
}

var (
	budgetMu sync.Mutex
	// budgetNext is the round each player may next hear an ambient line.
	// In memory only, on the shape of the item chatter listener cap
	// (behaviortree's listenerCapNext).
	budgetNext = map[int]uint64{}

	visitMu sync.Mutex
	// singleHeard is the room whose only idle line each player has seen
	// since walking in. RoundTick drops an entry once the player is anywhere
	// else, which ends the visit.
	singleHeard = map[int]int{}

	repeatMu sync.Mutex
	// repeatSeen is, per player, the rounds each line key reached them,
	// oldest first. Taken inside budgetMu when both are held. In memory
	// only; RoundTick prunes it.
	repeatSeen = map[int]map[string][]uint64{}
)

// Sees is the Perceives for a visual line in room: a reader who makes out at
// least shapes takes it in, the judgement the visual senders make
// (messaging.ReaderSight).
func Sees(room *rooms.Room) func(*users.UserRecord) bool {
	return func(u *users.UserRecord) bool {
		return messaging.ReaderSight(u.Character, room) != messaging.SightNone
	}
}

// HearsQuiet is the Perceives for a quiet line (a say's echo next door):
// the delivery hook gives a quiet message only to a player with the
// superhearing flag (hooks/Message_SendMessages.go), so only such a player
// takes it in and is charged.
func HearsQuiet(u *users.UserRecord) bool {
	return u.Character.HasConditionFlag(conditions.SuperHearing)
}

// budgetWindow is the budget window in rounds for a room in zone: the knob
// divided by the zone's ambientmultiplier, at least 1. A window of 0 means
// the budget is off. hushed means the budget is on and the zone's
// multiplier is 0: the window is endless, so no budgeted line gets through.
func budgetWindow(zone string) (window uint64, hushed bool) {
	base := float64(configs.GetBalanceConfig().AmbientListenerBudgetRounds)
	if base <= 0 {
		return 0, false
	}
	mult := rooms.ZoneAmbientMultiplier(zone)
	if mult <= 0 {
		return 0, true
	}
	w := math.Round(base / mult)
	if w < 1 {
		w = 1
	}
	return uint64(w), false
}

// repeatLimits is the repeat limit and its window in rounds; 0, 0 when
// either knob is 0 (the limit is off).
func repeatLimits() (limit int, window uint64) {
	b := configs.GetBalanceConfig()
	if b.AmbientRepeatLimit <= 0 || b.AmbientRepeatWindowRounds <= 0 {
		return 0, 0
	}
	return int(b.AmbientRepeatLimit), uint64(b.AmbientRepeatWindowRounds)
}

// seenLocked counts the times key reached userId inside the window ending
// at now. repeatMu held.
func seenLocked(userId int, key string, now, window uint64) int {
	n := 0
	for _, at := range repeatSeen[userId][key] {
		if at+window > now {
			n++
		}
	}
	return n
}

// Plan decides who an ambient line reaches and charges each of them. A
// player who has taken in this line's Key the repeat limit's times inside
// its window, or who is over budget, is added to Exclude: that player skips
// the line, and everyone else in the room still gets it.
func Plan(l Line) Routing {
	r := Routing{Exclude: slices.Clone(l.Exclude)}
	if l.Room == nil {
		return r
	}
	now := util.GetRoundCount()
	window, hushed := budgetWindow(l.Room.Zone)
	limit, repeatWindow := repeatLimits()
	checkRepeat := l.Key != `` && limit > 0
	suppressed, repeated := 0, 0

	budgetMu.Lock()
	repeatMu.Lock()
	for _, uid := range l.Room.GetPlayers() {
		if slices.Contains(l.Exclude, uid) {
			continue
		}
		u := users.GetByUserId(uid)
		if u == nil {
			continue
		}
		if l.Perceives != nil && !l.Perceives(u) {
			continue
		}
		// A repeat is held back before the budget is asked, so it charges
		// nothing and the next fresh line still gets through.
		if checkRepeat && seenLocked(uid, l.Key, now, repeatWindow) >= limit {
			r.Exclude = append(r.Exclude, uid)
			repeated++
			continue
		}
		if hushed || (window > 0 && now < budgetNext[uid]) {
			r.Exclude = append(r.Exclude, uid)
			suppressed++
			continue
		}
		if window > 0 {
			budgetNext[uid] = now + window
		}
		if checkRepeat {
			if repeatSeen[uid] == nil {
				repeatSeen[uid] = map[string][]uint64{}
			}
			repeatSeen[uid][l.Key] = append(repeatSeen[uid][l.Key], now)
		}
		r.Delivered = append(r.Delivered, uid)
	}
	repeatMu.Unlock()
	budgetMu.Unlock()

	countLine(l.Room, l.Source, len(r.Delivered), suppressed, repeated)
	return r
}

// AllHaveBudget reports whether every player in room could hear an ambient
// line now, without charging anyone. An NPC conversation starts only when
// this holds, so none is cut off halfway.
func AllHaveBudget(room *rooms.Room) bool {
	if room == nil {
		return true
	}
	window, hushed := budgetWindow(room.Zone)
	if window == 0 && !hushed {
		return true
	}
	players := room.GetPlayers()
	if len(players) == 0 {
		return true
	}
	if hushed {
		return false
	}
	now := util.GetRoundCount()
	budgetMu.Lock()
	defer budgetMu.Unlock()
	for _, uid := range players {
		if now < budgetNext[uid] {
			return false
		}
	}
	return true
}

// SingleLineHeard returns the players among players who have already seen
// roomId's only idle line during their current visit.
func SingleLineHeard(roomId int, players []int) []int {
	visitMu.Lock()
	defer visitMu.Unlock()
	var heard []int
	for _, uid := range players {
		if singleHeard[uid] == roomId {
			heard = append(heard, uid)
		}
	}
	return heard
}

// MarkSingleLineHeard records that userIds saw roomId's only idle line.
func MarkSingleLineHeard(roomId int, userIds []int) {
	visitMu.Lock()
	defer visitMu.Unlock()
	for _, uid := range userIds {
		singleHeard[uid] = roomId
	}
}

// RoundTick runs once a round from hooks.UserRoundTick. It ends the visits
// of players who left or logged out, drops spent budget entries, and drops
// the repeat memory of lines whose window has passed or of players no longer
// online, so no map grows past the players online. Then it runs the telemetry
// tick (telemetry.go), which does nothing while the switch is off.
func RoundTick(round uint64) {
	visitMu.Lock()
	for uid, roomId := range singleHeard {
		if u := users.GetByUserId(uid); u == nil || u.Character.RoomId != roomId {
			delete(singleHeard, uid)
		}
	}
	visitMu.Unlock()

	budgetMu.Lock()
	for uid, next := range budgetNext {
		if round >= next {
			delete(budgetNext, uid)
		}
	}
	budgetMu.Unlock()

	_, repeatWindow := repeatLimits()
	repeatMu.Lock()
	for uid, keys := range repeatSeen {
		if repeatWindow == 0 || users.GetByUserId(uid) == nil {
			delete(repeatSeen, uid)
			continue
		}
		for key, rounds := range keys {
			i := 0
			for i < len(rounds) && rounds[i]+repeatWindow <= round {
				i++
			}
			if i == len(rounds) {
				delete(keys, key)
			} else if i > 0 {
				keys[key] = rounds[i:]
			}
		}
		if len(keys) == 0 {
			delete(repeatSeen, uid)
		}
	}
	repeatMu.Unlock()

	telemetryTick(round)
}
