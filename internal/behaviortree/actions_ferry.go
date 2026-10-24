package behaviortree

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/ferry"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// boardFerry is the boarding flow; a variable so tests can record the
// route chosen without a live schedule.
var boardFerry = ferry.Board

// actBoardFerry boards the route a talk topic names (#375 R14). Params:
//
//	routes:            # destination key → routeid
//	  plymouth: stillwater_np_packet
//	  confluence: stillwater_confluence_barge
//	destination: plymouth   # required: a key of routes
//
// One talk topic per destination: the branch's destination param picks the
// route, and ferry.Board does the schedule check, fare, move and messaging.
// A tree whose destination is missing or is not a key of routes does not
// load (checkBoardFerryParams). Returns Failure when the agent is not at
// port so other branches can try, and Success on every other outcome.
func actBoardFerry(params map[string]any, ctx *EvalContext) Result {
	mob := mobs.GetInstance(ctx.InstanceId)
	if mob == nil {
		return Failure
	}
	user := users.GetByUserId(ctx.Event.UserId)
	if user == nil {
		return Failure
	}
	dest := ferryDestination(params)
	routeId, ok := ferryRoutes(params)[dest]
	if !ok {
		mudlog.Error("actBoardFerry", "error", "destination is not a key of routes", "destination", dest, "mobId", ctx.MobId)
		return Failure
	}
	if boardFerry(user, mob, ctx.RoomId, routeId) == ferry.BoardNotAtPort {
		return Failure // fall through to other tree branches
	}
	return Success
}

// ferryDestination is the branch's destination param, trimmed and lowered
// to match the routes keys.
func ferryDestination(params map[string]any) string {
	return strings.ToLower(strings.TrimSpace(getStringParam(params, "destination")))
}

// ferryRoutes reads the routes param (destination key → route id), keys
// lowered.
func ferryRoutes(params map[string]any) map[string]string {
	routes := map[string]string{}
	switch m := params["routes"].(type) {
	case map[string]any:
		for k, v := range m {
			if s, ok := v.(string); ok {
				routes[strings.ToLower(k)] = s
			}
		}
	case map[any]any:
		for k, v := range m {
			ks, kok := k.(string)
			vs, vok := v.(string)
			if kok && vok {
				routes[strings.ToLower(ks)] = vs
			}
		}
	case map[string]string:
		for k, v := range m {
			routes[strings.ToLower(k)] = v
		}
	}
	return routes
}

// checkBoardFerryParams refuses, at load, a board_ferry node whose
// destination is missing or is not a key of routes: with no player words
// to read, such a node could never board anyone.
func checkBoardFerryParams(params map[string]any) error {
	dest := ferryDestination(params)
	if dest == "" {
		return fmt.Errorf("board_ferry needs a destination param (a key of routes)")
	}
	if _, ok := ferryRoutes(params)[dest]; !ok {
		return fmt.Errorf("board_ferry destination %q is not a key of routes", dest)
	}
	return nil
}
