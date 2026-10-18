package behaviortree

// actions_state.go — MobState read/write actions:
// actSetState, actIncrementState, actDecrementState

import "strconv"

// stateKey is the BehaviorState key a set_state or state_equals node names.
// With `per_player: true` the key is kept once per triggering player (the
// key plus ":" plus the player's user id), so a flag one player sets does not
// gate another: Cleric Hadwen's rite guard (9100) runs a rite for each
// newcomer who asks, and two asking at once each get their own. A per-player
// node with no triggering player (a mob_idle tick) names no key at all.
func stateKey(params map[string]any, ctx *EvalContext) (string, bool) {
	key := getStringParam(params, "key")
	if !getBoolParam(params, "per_player") {
		return key, true
	}
	if key == "" || ctx.Event.UserId <= 0 {
		return "", false
	}
	return key + ":" + strconv.Itoa(ctx.Event.UserId), true
}

func actSetState(params map[string]any, ctx *EvalContext) Result {
	if ctx.MobState == nil {
		return Failure
	}
	key, ok := stateKey(params, ctx)
	if !ok || key == "" {
		return Failure
	}
	value := params["value"]
	ctx.MobState.Set(key, value)
	return Success
}

// actIncrementState adds amount (default 1) to a numeric MobState key.
// params: key (string), amount (int, default 1)
func actIncrementState(params map[string]any, ctx *EvalContext) Result {
	if ctx.MobState == nil {
		return Failure
	}
	key := getStringParam(params, "key")
	if key == "" {
		return Failure
	}
	amount := getIntParam(params, "amount")
	if amount == 0 {
		amount = 1
	}
	ctx.MobState.Set(key, ctx.MobState.GetInt(key)+amount)
	return Success
}

// actDecrementState subtracts amount (default 1) from a numeric MobState key.
// params: key (string), amount (int, default 1)
func actDecrementState(params map[string]any, ctx *EvalContext) Result {
	if ctx.MobState == nil {
		return Failure
	}
	key := getStringParam(params, "key")
	if key == "" {
		return Failure
	}
	amount := getIntParam(params, "amount")
	if amount == 0 {
		amount = 1
	}
	ctx.MobState.Set(key, ctx.MobState.GetInt(key)-amount)
	return Success
}
