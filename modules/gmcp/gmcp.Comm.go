package gmcp

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/channels"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/plugins"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/ansitags"
)

// ////////////////////////////////////////////////////////////////////
// NOTE: The init function in Go is a special function that is
// automatically executed before the main function within a package.
// It is used to initialize variables, set up configurations, or
// perform any other setup tasks that need to be done before the
// program starts running.
// ////////////////////////////////////////////////////////////////////
func init() {

	//
	// We can use all functions only, but this demonstrates
	// how to use a struct
	//
	g := GMCPCommModule{
		plug: plugins.New(`gmcp.Comm`, `1.0`),
	}

	events.RegisterListener(events.Communication{}, g.onComm)

}

type GMCPCommModule struct {
	// Keep a reference to the plugin when we create it so that we can call ReadBytes() and WriteBytes() on it.
	plug *plugins.Plugin
}

func (g *GMCPCommModule) onComm(e events.Event) events.ListenerReturn {

	evt, typeOk := e.(events.Communication)
	if !typeOk {
		mudlog.Error("Event", "Expected Type", "RoomChange", "Actual Type", e.Type())
		return events.Cancel
	}

	payload := GMCPCommModule_Payload{
		Channel: evt.CommType,
		Sender:  evt.Name,
		Text:    ansitags.Parse(evt.Message, ansitags.StripTags),
	}

	if evt.SourceUserId > 0 {
		payload.Source = `player`
	} else if evt.SourceMobInstanceId > 0 {
		payload.Source = `mob`
	}

	// Sent to everyone.
	// say, party, broadcast, whisper

	sendToUserIds := []int{}

	if evt.CommType == `say` {

		// Say is room speech and follows each listener, as the text lane does
		// (actions.sendSpoken): its own payload per listener.
		for _, d := range sayDeliveries(evt, payload) {
			events.AddToQueue(GMCPOut{
				UserId:  d.UserId,
				Module:  `Comm.Channel`,
				Payload: d.Payload,
			})
		}
		return events.Continue

	} else if evt.CommType == `party` {

		if evt.SourceUserId > 0 {
			if party := parties.Get(evt.SourceUserId); party != nil {
				sendToUserIds = append([]int{}, party.UserIds...)
			}
		}

	} else if evt.CommType == `broadcast` {

		sendToUserIds = append([]int{}, users.GetOnlineUserIds()...)

	} else if evt.CommType == `whisper` {

		if evt.TargetUserId > 0 {
			sendToUserIds = append(sendToUserIds, evt.TargetUserId)
		}

	} else if ch, ok := channels.Get(evt.CommType); ok {

		// Global chat channels: every online user who has the channel on (the
		// sender always sees their own line), mirroring the terminal fan-out.
		for _, u := range users.GetAllActiveUsers() {
			if channels.ShouldReceive(u.UserId == evt.SourceUserId, u.Deafened, u.GetConfigOption(ch.ConfigKey)) {
				sendToUserIds = append(sendToUserIds, u.UserId)
			}
		}

	}

	for _, userId := range sendToUserIds {

		// Exclude user from receiving their own messages?
		//if userId == evt.SourceUserId && evt.CommType != `broadcast` {
		//continue
		//}

		events.AddToQueue(GMCPOut{
			UserId:  userId,
			Module:  `Comm.Channel`,
			Payload: payload,
		})

	}

	return events.Continue
}

// commDelivery is one listener's copy of a Comm.Channel payload.
type commDelivery struct {
	UserId  int
	Payload GMCPCommModule_Payload
}

// sayDeliveries is who receives a say over GMCP and what each reads, matching
// the text lane (actions.sendSpoken, #252):
//
//   - A say aimed at one player (a mob's sayto, TargetUserId) reaches only
//     that player and the speaker; any other say reaches everyone in the
//     speaker's room.
//   - A player's words are chatter: a deafened listener does not receive
//     them. An NPC's are authored and reach a deafened listener (ruling 6).
//   - The sender reads by the listener's sight of the room: the name at clear
//     sight, "A figure" at shapes, "Someone" when they see nothing, and
//     "Someone" for everyone when the speaker spoke still hidden. The speaker
//     always reads their own name.
func sayDeliveries(evt events.Communication, base GMCPCommModule_Payload) []commDelivery {

	var room *rooms.Room
	if evt.SourceUserId > 0 {
		if user := users.GetByUserId(evt.SourceUserId); user != nil {
			room = rooms.LoadRoom(user.Character.RoomId)
		}
	}
	if evt.SourceMobInstanceId > 0 {
		if mob := mobs.GetInstance(evt.SourceMobInstanceId); mob != nil {
			room = rooms.LoadRoom(mob.Character.RoomId)
		}
	}
	if room == nil {
		return nil
	}

	listenerIds := room.GetPlayers()
	if evt.TargetUserId > 0 {
		listenerIds = []int{evt.TargetUserId}
	}

	deliveries := []commDelivery{}
	for _, uid := range listenerIds {
		u := users.GetByUserId(uid)
		if u == nil {
			continue
		}

		p := base
		if uid != evt.SourceUserId {
			if evt.SourceUserId > 0 && u.Deafened {
				continue
			}
			d := messaging.ParticipantSight(u.Character, room)
			if evt.SpeakerHidden {
				d = messaging.SightNone
			}
			if d != messaging.SightFull {
				noun := messaging.SpeakerNoun(d) // "a figure" or "someone", ASCII
				p.Sender = strings.ToUpper(noun[:1]) + noun[1:]
			}
		}
		deliveries = append(deliveries, commDelivery{UserId: uid, Payload: p})
	}
	return deliveries
}

type GMCPCommModule_Payload struct {
	Channel string `json:"channel"`
	Sender  string `json:"sender"`
	Source  string `json:"source"`
	Text    string `json:"text"`
}
