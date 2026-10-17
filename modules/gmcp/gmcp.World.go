package gmcp

import (
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/plugins"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func init() {

	g := &GMCPWorldModule{
		plug: plugins.New(`gmcp.World`, `1.0`),
	}

	events.RegisterListener(events.PlayerSpawn{}, g.playerSpawnHandler)
	events.RegisterListener(events.NewRound{}, g.newRoundHandler)
	events.RegisterListener(GMCPWorldUpdate{}, g.buildAndSendGMCPPayload)
}

type GMCPWorldModule struct {
	plug          *plugins.Plugin
	lastTimeOfDay string
}

// Tell the system a wish to send specific GMCP World update data
type GMCPWorldUpdate struct {
	UserId     int
	Identifier string
}

func (g GMCPWorldUpdate) Type() string { return `GMCPWorldUpdate` }

// timeOfDayLabel names the part of the day for World.Time from the same sun
// state the game uses (gametime.DayPeriod, which the `time` command reads):
// "Night" exactly while it is night, "Dawn" or "Evening" while the lamps are
// lit before night ends or after the sun is too low to show faces, and a
// clock word through the lit day. It used fixed clock hours before, while
// night is seasonal, so a midwinter 4PM sent "Afternoon" beside
// "is_night": true (2026-10-17 playtest).
func timeOfDayLabel(night bool, lampsLit bool, hour24 int) string {
	switch gametime.DayPeriod(night, lampsLit, hour24) {
	case "night":
		return "Night"
	case "dawn":
		return "Dawn"
	case "dusk":
		return "Evening"
	}
	switch {
	case hour24 < 12:
		return "Morning"
	case hour24 < 14:
		return "Midday"
	case hour24 < 18:
		return "Afternoon"
	default:
		return "Evening"
	}
}

// currentTimeOfDayLabel is timeOfDayLabel on the current round.
func currentTimeOfDayLabel(gd gametime.GameDate) string {
	return timeOfDayLabel(gd.Night, gametime.LampsLit(), gd.Hour24)
}

func (g *GMCPWorldModule) playerSpawnHandler(e events.Event) events.ListenerReturn {

	evt, typeOk := e.(events.PlayerSpawn)
	if !typeOk {
		return events.Continue
	}

	if evt.UserId == 0 {
		return events.Continue
	}

	events.AddToQueue(GMCPWorldUpdate{
		UserId:     evt.UserId,
		Identifier: `World.Time`,
	})

	return events.Continue
}

func (g *GMCPWorldModule) newRoundHandler(e events.Event) events.ListenerReturn {

	_, typeOk := e.(events.NewRound)
	if !typeOk {
		return events.Continue
	}

	gd := gametime.GetDate()
	tod := currentTimeOfDayLabel(gd)

	// Only broadcast when the qualitative time-of-day label changes.
	if tod == g.lastTimeOfDay {
		return events.Continue
	}
	g.lastTimeOfDay = tod

	for _, user := range users.GetAllActiveUsers() {
		events.AddToQueue(GMCPWorldUpdate{
			UserId:     user.UserId,
			Identifier: `World.Time`,
		})
	}

	return events.Continue
}

func (g *GMCPWorldModule) buildAndSendGMCPPayload(e events.Event) events.ListenerReturn {

	evt, typeOk := e.(GMCPWorldUpdate)
	if !typeOk {
		mudlog.Error("Event", "Expected Type", "GMCPWorldUpdate", "Actual Type", e.Type())
		return events.Cancel
	}

	if evt.UserId < 1 {
		return events.Continue
	}

	user := users.GetByUserId(evt.UserId)
	if user == nil {
		return events.Continue
	}

	if !isGMCPEnabled(user.ConnectionId()) {
		return events.Cancel
	}

	gd := gametime.GetDate()

	payload := GMCPWorldModule_Payload_Time{
		Year:      gd.Year,
		Month:     gd.Month,
		Day:       gd.Day,
		TimeOfDay: currentTimeOfDayLabel(gd),
		IsNight:   gd.Night,
	}

	events.AddToQueue(GMCPOut{
		UserId:  evt.UserId,
		Module:  `World.Time`,
		Payload: payload,
	})

	return events.Continue
}

// /////////////////
// World.Time
// /////////////////
type GMCPWorldModule_Payload_Time struct {
	Year      int    `json:"year"`
	Month     int    `json:"month"`
	Day       int    `json:"day"`
	TimeOfDay string `json:"time_of_day"` // "Dawn","Morning","Midday","Afternoon","Evening","Night"
	IsNight   bool   `json:"is_night"`
}
