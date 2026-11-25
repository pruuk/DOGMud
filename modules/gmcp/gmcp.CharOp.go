package gmcp

import (
	"encoding/json"
	"errors"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/usercommands"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// GMCPCharOp carries an inbound, state-touching Char.* request from the
// per-connection goroutine to MainWorker.
//
// HandleIAC runs on the connection goroutine, which is genuinely concurrent
// with the world tick. Char.Automation.*, Char.Action.Try and Char.Quests.Focus
// all read/write shared game state (user records, the room manager's plain maps
// via rooms.LoadRoom, quest progress). Doing that work inline races MainWorker
// and can trigger Go's uncatchable "fatal error: concurrent map read and map
// write". This mirrors GMCPBuildOp: copy the payload, queue the op, and do the
// real work in handleCharOp, which runs on MainWorker under util.LockMud().
type GMCPCharOp struct {
	ConnectionId uint64
	Command      string
	Payload      []byte
}

func (e GMCPCharOp) Type() string { return `GMCPCharOp` }

// handleCharOp processes a deferred Char.* request on the MainWorker goroutine.
// The bodies below are the former inline HandleIAC cases, unchanged apart from
// resolving the connection id from the event.
func (g *GMCPModule) handleCharOp(e events.Event) events.ListenerReturn {

	evt, ok := e.(GMCPCharOp)
	if !ok {
		return events.Cancel
	}

	switch evt.Command {

	case `Char.Automation.Set`:
		// Peek the kind, then unmarshal the payload into the matching type.
		var kindOnly struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(evt.Payload, &kindOnly); err == nil {
			switch kindOnly.Kind {
			case `tick`:
				var decoded users.UserTick
				if err := json.Unmarshal(evt.Payload, &decoded); err == nil {
					if uid := userIdForConnection(evt.ConnectionId); uid > 0 {
						if u := users.GetByUserId(uid); u != nil {
							if _, err := u.SetTick(decoded); err != nil {
								refuseAutomation(u, err, `tick`, `ticks`)
								break
							}
							events.AddToQueue(events.AutomationChanged{UserId: uid})
						}
					}
				}
			case `trigger`:
				var decoded users.UserTrigger
				if err := json.Unmarshal(evt.Payload, &decoded); err == nil {
					if uid := userIdForConnection(evt.ConnectionId); uid > 0 {
						if u := users.GetByUserId(uid); u != nil {
							if _, err := u.SetTrigger(decoded); err != nil {
								refuseAutomation(u, err, `trigger`, `triggers`)
								break
							}
							events.AddToQueue(events.AutomationChanged{UserId: uid})
						}
					}
				}
			}
		}

	case `Char.Automation.Remove`:
		var decoded struct {
			Kind string `json:"kind"`
			Id   int    `json:"id"`
		}
		if err := json.Unmarshal(evt.Payload, &decoded); err == nil {
			if uid := userIdForConnection(evt.ConnectionId); uid > 0 {
				if u := users.GetByUserId(uid); u != nil {
					switch decoded.Kind {
					case `tick`:
						u.RemoveTick(decoded.Id)
						events.AddToQueue(events.AutomationChanged{UserId: uid})
					case `trigger`:
						u.RemoveTrigger(decoded.Id)
						events.AddToQueue(events.AutomationChanged{UserId: uid})
					}
				}
			}
		}

	case `Char.Action.Try`:
		var req struct {
			Id  int    `json:"id"`
			Cmd string `json:"cmd"`
		}
		if err := json.Unmarshal(evt.Payload, &req); err != nil {
			break
		}
		// ⚠️ EVERY exit from here MUST reply. The web client parks the action
		// queue on `queueInFlight` until a Char.Action.Result arrives for this id,
		// and a silent break leaves it parked -- entries visibly pile up in the
		// Triggers panel and nothing fires again until the player hits Clear. The
		// client now also times out, but the queue should not depend on that.
		uid := userIdForConnection(evt.ConnectionId)
		if uid <= 0 {
			break // no user to reply TO; the client timeout is the only recourse
		}
		u := users.GetByUserId(uid)
		if u == nil {
			sendActionResult(uid, req.Id, "rejected", "no character")
			break
		}
		// A Try spends the AI port's per-round command budget exactly as a
		// typed line does, from the same counter; a refused Try is dropped
		// with the same line the typed path sends. Other ports are unlimited.
		if cd := connections.Get(evt.ConnectionId); cd != nil && cd.AIRateLimited() {
			sendActionResult(uid, req.Id, "rejected", "rate limit")
			break
		}
		// Belt and suspenders for the client's pause (#375): a queued action
		// never runs while a menu or prompt is open.
		if u.MenuOrPromptOpen() {
			sendActionResult(uid, req.Id, "deferred", "menu open")
			break
		}
		room := rooms.LoadRoom(u.Character.RoomId)
		actor := actions.NewUserActorInRoom(u, room)
		result := actions.ActionReadiness(actor, req.Cmd)
		// A Try is a command from the player's client: it counts against
		// their pending inputs and runs at the client pace. Refused at the
		// cap, it is answered "deferred", so the client tries it again later.
		switch result.Status {
		case actions.ActionReady:
			if !u.CommandFlagged(req.Cmd, events.CmdFromClient) {
				sendActionResult(uid, req.Id, "deferred", actionQueueFullReason)
				break
			}
			sendActionResult(uid, req.Id, "fired", "")
		case actions.ActionDeferred:
			sendActionResult(uid, req.Id, "deferred", result.Reason)
		case actions.ActionRejected:
			// Run normally so the player sees the real error message.
			if !u.CommandFlagged(req.Cmd, events.CmdFromClient) {
				sendActionResult(uid, req.Id, "deferred", actionQueueFullReason)
				break
			}
			sendActionResult(uid, req.Id, "rejected", result.Reason)
		}

	case `Char.Quests.Focus`:
		var req struct {
			Id int `json:"id"`
		}
		if err := json.Unmarshal(evt.Payload, &req); err != nil {
			break
		}
		uid := userIdForConnection(evt.ConnectionId)
		if uid <= 0 {
			break
		}
		u := users.GetByUserId(uid)
		if u == nil {
			break
		}
		// Only an active quest, and never a secret one (FocusQuest checks).
		if !usercommands.FocusQuest(u, req.Id) {
			break
		}
		// Re-emit Char.Quests so the panel, marker, and `hint` all follow.
		events.AddToQueue(GMCPCharUpdate{
			UserId:     uid,
			Identifier: `Char.Quests`,
		})

	case `Char.Quests.Abandon`:
		var req struct {
			Id int `json:"id"`
		}
		if err := json.Unmarshal(evt.Payload, &req); err != nil {
			break
		}
		uid := userIdForConnection(evt.ConnectionId)
		if uid <= 0 {
			break
		}
		u := users.GetByUserId(uid)
		if u == nil {
			break
		}
		// AbandonQuest queues the MarkerOnly quest event whose listener
		// re-sends Char.Quests, so no GMCPCharUpdate is queued here.
		ok, line := usercommands.AbandonQuest(u, req.Id)
		usercommands.SendQuestLogLine(u, ok, line)

	}

	return events.Continue
}

// refuseAutomation tells the player why a tick or trigger was not saved
// (users.ErrAutomationFull, users.ErrAutomationTooLong). Nothing changed, so
// no Char.Automation is re-sent. one and many name the kind ("trigger",
// "triggers").
func refuseAutomation(u *users.UserRecord, err error, one, many string) {
	if errors.Is(err, users.ErrAutomationFull) {
		most := configs.GetNetworkConfig().MaxAutomationTriggers
		if one == `tick` {
			most = configs.GetNetworkConfig().MaxAutomationTicks
		}
		refusal.AutomationLimit(u, many, int(most))
		return
	}
	if errors.Is(err, users.ErrAutomationTooManyCommands) {
		refusal.CommandChainTooLong(u, one, users.MaxCommandChain())
		return
	}
	refusal.AutomationTooLong(u, one)
}
