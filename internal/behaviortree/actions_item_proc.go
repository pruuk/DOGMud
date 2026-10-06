package behaviortree

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The proc node (item behaviour slice 3, Rules 20 to 22). A proc is an item
// tree branch under one of the proc events, usually wrapped in a `random`
// decorator (its chance; omitted at 100% so no number is drawn) inside a
// `cooldown` decorator. `proc` runs one of four effects, written in Go
// because this is the combat hot path, and succeeds only when the effect
// did something, so the cooldown is armed exactly when it took effect.

// procEvents are the events a proc branch may sit under (spec P2).
var procEvents = map[string]bool{
	"on_hit": true, "on_kill": true, "on_block": true, "on_grapple": true, "on_spell_hit": true,
}

// procEffectParams are the effects and the numeric params each reads.
var procEffectParams = map[string]map[string]bool{
	"lifesteal":       {"ratio": true},
	"steal_pool":      {"pool": true, "amount_pct": true},
	"aoe_stun":        {"stun_rounds": true},
	"apply_condition": {"condition": true, "duration": true, "magnitude": true},
}

// ProcEvent carries a proc trigger's participants: the bearer's opponent
// (nil on a kill), the room (nil where the caller does not know it) and
// the damage the trigger dealt.
type ProcEvent struct {
	Other  *characters.Character
	Room   *rooms.Room
	Damage int
}

// ItemProcsOn reports the ItemProcsEnabled switch. The dispatcher reads it
// before running a tree, so a disabled proc draws nothing.
func ItemProcsOn() bool {
	return bool(configs.GetConfig().GamePlay.ItemProcsEnabled)
}

// actProc runs the effect named by `effect` for the item's holder. The
// other params are the effect's, as numbers.
func actProc(params map[string]any, ctx *EvalContext) Result {
	if !ItemProcsOn() {
		return Failure
	}
	owner := itemHolder(ctx)
	if owner == nil {
		return Failure
	}
	var ev ProcEvent
	if ctx.Event.Proc != nil {
		ev = *ctx.Event.Proc
	}
	effect := getStringParam(params, `effect`)
	p := make(map[string]float64, len(params))
	for k := range procEffectParams[effect] {
		if _, ok := params[k]; ok {
			p[k] = getFloatParam(params, k, 0)
		}
	}
	executed := false
	switch effect {
	case `lifesteal`:
		executed = procLifesteal(owner, ev.Damage, p) > 0
	case `steal_pool`:
		executed = procStealPool(owner, ev.Other, p)
	case `aoe_stun`:
		executed = procAoeStun(owner, ev.Room, p)
	case `apply_condition`:
		executed = procApplyCondition(ev.Other, p)
	}
	if executed {
		return Success
	}
	return Failure
}

// ProcCooldownDecorator is a `cooldown` decorator over a proc branch (Rule
// 22, ruling R1). Its state lives in the holder's MiscData, keyed by the
// item's template id and the branch's path, not in the item's tree state:
// two copies of one item share it and it survives a relog. It stores the
// round the branch may fire again.
type ProcCooldownDecorator struct {
	Rounds int
	Path   string // the decorator's compile path; names the branch
	Child  Node
}

// procCooldownKey is the holder's MiscData key for one proc branch.
func procCooldownKey(itemId int, path string) string {
	return fmt.Sprintf("item_proc_cd_%d_%s", itemId, path)
}

func (d *ProcCooldownDecorator) Evaluate(ctx *EvalContext) Result {
	c := itemHolder(ctx)
	if c == nil {
		return Failure
	}
	key := procCooldownKey(ctx.Item.ItemId, d.Path)
	now := util.GetRoundCount()
	if until, ok := characters.MiscRound(c.GetMiscData(key)); ok && now < until {
		return Failure
	}
	result := d.Child.Evaluate(ctx)
	if result == Success && d.Rounds > 0 {
		c.SetMiscData(key, now+uint64(d.Rounds))
	}
	return result
}

// nodeDefNamesAction reports whether a tree names the action anywhere.
func nodeDefNamesAction(def NodeDef, action string) bool {
	if def.Do == action {
		return true
	}
	for _, ch := range def.Children {
		if nodeDefNamesAction(ch, action) {
			return true
		}
	}
	return def.Child != nil && nodeDefNamesAction(*def.Child, action)
}

// checkProcNodes refuses a proc node that would never fire or fire wrongly:
// one not under a proc event, an unknown effect, a param its effect does
// not read or that is not a number, and a `random` decorator over a proc
// outside 1 to 99 (at 100 the branch omits it, so nothing is drawn; X19).
// event is the nearest enclosing event.
func checkProcNodes(def NodeDef, event, path string) error {
	if def.Event != `` {
		event = def.Event
	}
	if def.Type == `decorator` && def.Mod == `random` && nodeDefNamesAction(def, `proc`) {
		if pct := getIntParam(def.Params, `percent`); pct < 1 || pct > 99 {
			return fmt.Errorf("%s: random percent %d over a proc: want 1 to 99 (omit the decorator at 100)", path, pct)
		}
	}
	if def.Do == `proc` {
		if !procEvents[event] {
			return fmt.Errorf("%s: proc under event %q: want one of on_hit, on_kill, on_block, on_grapple, on_spell_hit", path, event)
		}
		effect := getStringParam(def.Params, `effect`)
		allowed, ok := procEffectParams[effect]
		if !ok {
			return fmt.Errorf("%s: proc effect %q: want lifesteal, steal_pool, aoe_stun or apply_condition", path, effect)
		}
		for k, v := range cleanParams(def) {
			if k == `effect` {
				continue
			}
			if !allowed[k] {
				return fmt.Errorf("%s: proc effect %s does not read %q", path, effect, k)
			}
			switch v.(type) {
			case int, float64:
			default:
				return fmt.Errorf("%s: proc param %s %v: want a number", path, k, v)
			}
		}
	}
	for i, ch := range def.Children {
		if err := checkProcNodes(ch, event, fmt.Sprintf("%s.%d", path, i)); err != nil {
			return err
		}
	}
	if def.Child != nil {
		return checkProcNodes(*def.Child, event, path+".child")
	}
	return nil
}

// procLifesteal heals the attacker for ratio*damage, clamped to HealthMax.
// Returns the amount actually healed.
func procLifesteal(attacker *characters.Character, damage int, params map[string]float64) int {
	if attacker == nil {
		return 0
	}
	ratio := params["ratio"]
	if ratio <= 0 || damage <= 0 {
		return 0
	}
	amt := int(float64(damage) * ratio)
	if amt < 1 {
		amt = 1
	}
	return attacker.Heal(amt)
}

// procStealPool drains a pool from the target into the owner. Params:
// pool (3=conviction; 1=health/2=stamina reserved, unimplemented — YAGNI
// until an item needs them), amount_pct (fraction of the TARGET's pool
// max, capped by what they actually have). Executes only when something
// was actually stolen (so an empty-pool target does not burn the cooldown).
func procStealPool(owner, other *characters.Character, params map[string]float64) bool {
	if owner == nil || other == nil {
		return false
	}
	pct := params["amount_pct"]
	if pct <= 0 {
		return false
	}
	switch int(params["pool"]) {
	case 3: // conviction
		amt := int(float64(other.ConvictionMax.Value) * pct)
		if amt < 1 {
			amt = 1
		}
		if amt > other.Conviction {
			amt = other.Conviction
		}
		if amt <= 0 {
			return false
		}
		// A TRANSFER, not a restore: the owner may only absorb what was
		// actually drained, or conviction would be created or destroyed. amt
		// is still pre-clamped to the target's pool above so the drain and the
		// gain stay equal today.
		ownerRef := state.ActorRef{UserId: owner.GetUserId(), MobInstanceId: owner.MobInstanceId}
		drained := other.ApplyHarm(characters.PoolConviction, amt, ownerRef)
		owner.ApplyRestore(characters.PoolConviction, drained)
		return true
	}
	return false
}

// procApplyCondition applies the Bleeding record to the target. Params:
// condition (1=bleeding — the switch is the extension point for future
// condition ids; only bleeding is wired here, YAGNI), duration (the stack's
// rounds, default 4 if unset/<1), magnitude (per-round health loss, default 2
// if unset/<1). Each proc that fires adds one stack; see the Stacking flag.
// Unknown condition ids do not execute (so the branch's cooldown isn't
// armed).
func procApplyCondition(target *characters.Character, params map[string]float64) bool {
	if target == nil {
		return false
	}
	dur := int(params["duration"])
	if dur < 1 {
		dur = 4
	}
	mag := params["magnitude"]
	if mag < 1 {
		mag = 2
	}
	switch int(params["condition"]) {
	case 1:
		return target.AddConditionMagnitude(conditions.ConditionIdBleeding, dur, -mag, "itemproc") == nil
	}
	return false
}

// procAoeStun applies the stagger-stun condition (84 — a 1-round Stunned) to every
// hostile, stun-eligible mob in the owner's room. Non-combatants,
// attack-immune, and charmed mobs are never targeted — stunning someone's
// companion or a town NPC would be a prod incident. Returns true if
// at least one target was stunned (only then is the branch's cooldown armed).
//
// Mob owners are a no-op: no Stage-2 mob wields an aoe_stun item, and "hostile
// to a mob" has no clean definition here, so we return false (cooldown
// unburned) rather than guess. owner.GetUserId() is 0 for mobs.
//
// The stun_rounds param is intentionally IGNORED: condition 84 is a fixed 1-round
// stagger (triggercount:1 in its YAML) and cannot be duration-scaled from data
// without hacking condition internals. The Aegis of Mockery tunes its
// strength through its branch's chance and cooldown instead.
func procAoeStun(owner *characters.Character, room *rooms.Room, params map[string]float64) bool {
	if owner == nil {
		return false
	}
	ownerUserId := owner.GetUserId()
	if ownerUserId <= 0 {
		// Mob (or unassigned) owner — no-op, see doc comment.
		return false
	}

	if room == nil {
		room = rooms.LoadRoom(owner.RoomId)
	}
	if room == nil {
		return false
	}

	stunned := 0
	for _, mobId := range room.GetMobs(rooms.FindAll) {
		m := mobs.GetInstance(mobId)
		if m == nil {
			continue
		}
		// Spare non-combatants, attack-immune mobs, and ALL charmed
		// companions whoever their master is — a non-party bystander's
		// companion caught in the shockwave would be a prod incident just as
		// surely as a party member's. mobs.CheckPlayerHarm is the same policy
		// the player-cast HarmArea path applies in resolveSpell.
		if mobs.CheckPlayerHarm(m).Blocked() {
			continue
		}
		_ = m.Character.AddCondition(84, false)
		stunned++
	}

	if stunned == 0 {
		return false
	}

	// Room-wide narration, no raw numbers (project rule). CategorySubmission
	// matches condition 84's own submission-stagger flavor.
	//
	// Observer-only SendTrio, not a raw SendTextVisual. The line names nobody,
	// so this is not a leak fix: it is what lets CategorySubmission join
	// sendTrioOnlyCategories, since this was the category's last raw sender.
	// Both names are NoName because there is no actor and no actee to hide.
	messaging.SendTrio(messaging.Trio{
		// Room flavour with no participants: the shockwave is the condition's,
		// not any character's. Both personal roles are NoLine so the silence
		// reads as considered rather than forgotten.
		Actor: messaging.NoLine,
		Actee: messaging.NoLine,
		Observer: messaging.Say(messaging.CategorySubmission,
			`<ansi fg="yellow">A jarring shockwave ripples outward, staggering the hostile creatures nearby!</ansi>`),
	}, messaging.Audience{
		ActorName: messaging.NoName,
		ActeeName: messaging.NoName,
		Room:      room,
	})
	return true
}
