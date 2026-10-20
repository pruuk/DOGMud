package behaviortree

import (
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/ferry"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// boardFerry is the boarding flow; a variable so tests can record the
// route chosen without a live schedule.
var boardFerry = ferry.Board

// actBoardFerry handles "ask <agent> passage <destination>". Params:
//
//	routes:            # destination keyword → routeid
//	  plymouth: stillwater_np_packet
//	  confluence: stillwater_confluence_barge
//	destination: plymouth   # optional: a key of routes (#375 R14)
//
// With destination set (a talk topic per destination, where Text is the
// bare topic key), that route is boarded and the text is not read; a
// destination that is not a key of routes logs an error and fails.
// Otherwise it resolves the destination word from the ask text, then delegates the
// entire boarding flow (schedule check, fare, move, messaging) to
// ferry.Board. Returns Failure when params are unusable so other tree
// branches can try; returns Success (with mob dialogue) on every handled
// outcome. The keyword_match condition gating this action in the tree
// establishes boarding intent, so an unrecognized destination lists the
// options rather than falling through.
func actBoardFerry(params map[string]any, ctx *EvalContext) Result {
	mob := mobs.GetInstance(ctx.InstanceId)
	if mob == nil {
		return Failure
	}
	user := users.GetByUserId(ctx.Event.UserId)
	if user == nil {
		return Failure
	}

	routesMap := map[string]string{}
	if raw, ok := params["routes"]; ok {
		switch m := raw.(type) {
		case map[string]any:
			for k, v := range m {
				if s, ok := v.(string); ok {
					routesMap[strings.ToLower(k)] = s
				}
			}
		case map[any]any:
			for k, v := range m {
				ks, kok := k.(string)
				vs, vok := v.(string)
				if kok && vok {
					routesMap[strings.ToLower(ks)] = vs
				}
			}
		case map[string]string:
			for k, v := range m {
				routesMap[strings.ToLower(k)] = v
			}
		}
	}
	if len(routesMap) == 0 {
		return Failure
	}

	// Sorted keyword order makes the pick deterministic when the ask text
	// contains more than one destination ("plymouth or confluence?").
	dests := make([]string, 0, len(routesMap))
	for k := range routesMap {
		dests = append(dests, k)
	}
	sort.Strings(dests)

	// A talk topic names its destination as a param (#375 R14): under
	// player_topic, Text is a bare topic key, not the player's words.
	if dest := strings.ToLower(strings.TrimSpace(getStringParam(params, "destination"))); dest != `` {
		routeId, ok := routesMap[dest]
		if !ok {
			mudlog.Error("actBoardFerry", "error", "destination is not a key of routes", "destination", dest, "mobId", ctx.MobId)
			return Failure
		}
		if boardFerry(user, mob, ctx.RoomId, routeId) == ferry.BoardNotAtPort {
			return Failure
		}
		return Success
	}

	text := strings.ToLower(ctx.Event.Text)
	routeId := ``
	for _, keyword := range dests {
		if strings.Contains(text, keyword) {
			routeId = routesMap[keyword]
			break
		}
	}

	if routeId == `` {
		mob.Command(`say Passage where? I book for ` + strings.Join(dests, ` and `) + `.`)
		return Success
	}

	if boardFerry(user, mob, ctx.RoomId, routeId) == ferry.BoardNotAtPort {
		return Failure // fall through to other tree branches
	}
	return Success
}
