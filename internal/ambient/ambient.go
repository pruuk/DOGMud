// Package ambient is the one gate every unprompted ("ambient") line passes
// on its way to players (#317). It owns the per-listener budget
// (Balance.AmbientListenerBudgetRounds), the once-per-visit rule for a room
// with a single idle line, and the telemetry that measures how much each
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
	"sync"

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
)

// Sources lists every counted source, in report order.
var Sources = []Source{SourceRoomIdle, SourceMobChatter, SourceWeather, SourceGossip, SourceItemChatter}

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
}

// Routing is Plan's answer. Exclude goes to the room sender as its
// excludeUserIds; Delivered is every player who takes the line in and was
// charged for it.
type Routing struct {
	Exclude   []int
	Delivered []int
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
)

// Sees is the Perceives for a visual line in room: a reader who makes out at
// least shapes takes it in, the judgement the visual senders make
// (messaging.ReaderSight).
func Sees(room *rooms.Room) func(*users.UserRecord) bool {
	return func(u *users.UserRecord) bool {
		return messaging.ReaderSight(u.Character, room) != messaging.SightNone
	}
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

// Plan decides who an ambient line reaches and charges each of them. A
// player over budget is added to Exclude: that player skips the line, and
// everyone else in the room still gets it.
func Plan(l Line) Routing {
	r := Routing{Exclude: slices.Clone(l.Exclude)}
	if l.Room == nil {
		return r
	}
	now := util.GetRoundCount()
	window, hushed := budgetWindow(l.Room.Zone)

	budgetMu.Lock()
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
		if hushed || (window > 0 && now < budgetNext[uid]) {
			r.Exclude = append(r.Exclude, uid)
			continue
		}
		if window > 0 {
			budgetNext[uid] = now + window
		}
		r.Delivered = append(r.Delivered, uid)
	}
	budgetMu.Unlock()

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
// of players who left or logged out, and drops spent budget entries, so
// neither map grows past the players online.
func RoundTick(round uint64) {
	visitMu.Lock()
	for uid, roomId := range singleHeard {
		if u := users.GetByUserId(uid); u == nil || u.Character.RoomId != roomId {
			delete(singleHeard, uid)
		}
	}
	visitMu.Unlock()

	now := util.GetRoundCount()
	budgetMu.Lock()
	for uid, next := range budgetNext {
		if now >= next {
			delete(budgetNext, uid)
		}
	}
	budgetMu.Unlock()
}
