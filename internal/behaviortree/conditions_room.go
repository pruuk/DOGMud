package behaviortree

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// condCommandMatches checks if ctx.Event.Command matches any string in the
// "commands" list param.
func condCommandMatches(params map[string]any, ctx *EvalContext) Result {
	cmdList, ok := params["commands"]
	if !ok {
		return Failure
	}
	list, ok := cmdList.([]any)
	if !ok {
		return Failure
	}
	cmd := strings.ToLower(ctx.Event.Command)
	for _, v := range list {
		if s, ok := v.(string); ok && strings.ToLower(s) == cmd {
			return Success
		}
	}
	return Failure
}

// condCommandRestWords succeeds when the command's arguments hold at least
// "min" words, split as the commands split them (quotes respected). A tree
// that intercepts say or ask uses it so a bare verb, which the command itself
// refuses, is not taken for the real thing.
func condCommandRestWords(params map[string]any, ctx *EvalContext) Result {
	if len(util.SplitButRespectQuotes(ctx.Event.Rest)) >= getIntParam(params, "min") {
		return Success
	}
	return Failure
}

// condCommandNamesMob succeeds when the command's first argument (after an
// optional "to", as talk accepts) names a mob of template "mob_id" in the
// room, resolved for the typing player exactly as talk and ask resolve it:
// actions.AimBySight first, so a name the player could not aim at in the
// dark does not count, then actions.ResolveTargetActor.
func condCommandNamesMob(params map[string]any, ctx *EvalContext) Result {
	args := util.SplitButRespectQuotes(ctx.Event.Rest)
	if len(args) > 1 && strings.ToLower(args[0]) == `to` {
		args = args[1:]
	}
	if len(args) == 0 {
		return Failure
	}
	user := users.GetByUserId(ctx.Event.UserId)
	room := rooms.LoadRoom(ctx.RoomId)
	if user == nil || user.Character == nil || room == nil {
		return Failure
	}
	name, refused := actions.AimBySight(user.Character, user.UserId, room, args[0], strings.ToLower(ctx.Event.Command))
	if refused != `` {
		return Failure
	}
	target, err := actions.ResolveTargetActor(room, name, actions.ResolveTargetOptions{Viewer: user.Character})
	if err != nil {
		return Failure
	}
	if m, ok := target.(*actions.MobActor); ok && m.Mob != nil && int(m.Mob.MobId) == getIntParam(params, "mob_id") {
		return Success
	}
	return Failure
}

// condCommandRestContains checks if ctx.Event.Rest contains any keyword from
// the "keywords" list param.
func condCommandRestContains(params map[string]any, ctx *EvalContext) Result {
	kwList, ok := params["keywords"]
	if !ok {
		return Failure
	}
	list, ok := kwList.([]any)
	if !ok {
		return Failure
	}
	rest := strings.ToLower(ctx.Event.Rest)
	for _, v := range list {
		if s, ok := v.(string); ok && strings.Contains(rest, strings.ToLower(s)) {
			return Success
		}
	}
	return Failure
}
