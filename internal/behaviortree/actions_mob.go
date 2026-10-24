package behaviortree

// actions_mob.go — mob movement, spawning, and instance actions:
// actSpawnMob, actSummonCompanion, actCommand, actCommandMob, actCommandBestOf,
// actTrySpecialMove, actSweepCompanions, actMove, actOpenInstancePortal,
// actCreateInstance

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

func actSpawnMob(params map[string]any, ctx *EvalContext) Result {
	mobId := getIntParam(params, "mob_id")
	if mobId == 0 {
		return Failure
	}
	roomId := getIntParam(params, "room_id")
	if roomId == 0 {
		roomId = ctx.RoomId
	}
	spawned := mobs.NewMobById(mobs.MobId(mobId), roomId)
	if spawned == nil {
		return Failure
	}
	return Success
}

// actSummonCompanion spawns a mob as either a hostile ally (attacks the
// player) or a charmed companion of the summoning mob.
//
// params: mob_id (int), count (int, default 1), base_pool (int, default 50),
//
//	hostile (string "true"/"false", default "false")
//
// When hostile=true: spawned mobs aggro the triggering player immediately.
// When hostile=false: spawned mobs are charmed to the summoning mob (companion).
func actSummonCompanion(params map[string]any, ctx *EvalContext) Result {
	mob := mobs.GetInstance(ctx.InstanceId)
	if mob == nil {
		return Failure
	}
	mobId := getIntParam(params, "mob_id")
	if mobId == 0 {
		return Failure
	}
	count := getIntParam(params, "count")
	if count <= 0 {
		count = 1
	}
	room := rooms.LoadRoom(ctx.RoomId)
	if room == nil {
		return Failure
	}
	manifestSkill := mob.Character.GetSkillLevel(skills.Manifestation)
	charisma := mob.Character.Stats.Charisma.ValueAdj
	basePool := getIntParam(params, "base_pool")
	if basePool <= 0 {
		basePool = 50
	}
	hostile := getBoolParam(params, "hostile")
	// CalcSpawnPoolFromBase, not CalcCompanionPool: this path spawns authored
	// boss adds whose base_pool values were tuned against the old curve, and it
	// is not a player companion (it reserves nothing and has no pet multiplier).
	pool := characters.CalcSpawnPoolFromBase(basePool, charisma, manifestSkill)
	for i := 0; i < count; i++ {
		companion := mobs.NewMobByIdFresh(mobs.MobId(mobId), room.RoomId, pool)
		if companion == nil {
			continue
		}
		room.AddMob(companion.InstanceId)
		if hostile {
			// Hostile ally: aggro the triggering player and engage.
			//
			// ctx.Event.UserId is only populated by SOME trigger events
			// (e.g. mob_hurt carries the attacking player). Events fired
			// from the round driver's per-round combat tick (mob_combat_round
			// — used by e.g. the Guardian's recurring add re-summons) never
			// carry a UserId (see internal/hooks/NewRound_DoCombat.go), so
			// requiring it here silently no-oped every hostile summon
			// triggered from that event: the companion spawned into the
			// room with no Aggro at all and had to wait on its own
			// mob_idle→lookfortrouble roll to ever pick a fight — a roll
			// that competes with idle flavor text and, worse, never gets a
			// chance if the encounter ends first (confirmed by live
			// calibration: adds summoned this way sat idle for the whole
			// fight). Fall back to an eligible present player so a hostile
			// summon ALWAYS acquires a target immediately, matching the
			// documented "aggro the triggering player immediately" intent
			// even when the trigger event itself had no player attached.
			targetUserId := ctx.Event.UserId
			if targetUserId <= 0 {
				targetUserId = pickEligibleRoomPlayer(room)
			}
			if targetUserId > 0 {
				targeting.Commit(&companion.Character,
					state.ActorRef{UserId: targetUserId}, targeting.ReasonAttack)
			}
			// Still queued as a fallback/safety net — a no-op if Aggro is
			// already set (LookForTrouble returns immediately when
			// already in combat), but ensures the companion still tries
			// to find a fight on its own if no eligible target was found
			// above (e.g. every player in the room was hidden/downed at
			// the exact moment of the summon).
			companion.Command(`lookfortrouble`, 4)
		} else {
			// Charmed companion of the summoning mob
			companion.Character.Charm(0, 99999, "")
			targeting.Release(&companion.Character, targeting.ReasonDisengage)
			info := characters.CompanionInfo{
				MobId:      int(companion.MobId),
				InstanceId: companion.InstanceId,
				SourceType: characters.CompanionSummoned,
				Name:       companion.Character.Name,
				BaseName:   companion.Character.Name,
				AutoAssist: true,
			}
			mob.Character.AddCompanion(info)
		}
	}
	return Success
}

// pickEligibleRoomPlayer returns a random player instance id in room who is
// a valid aggro target — alive, not sneaking/hidden, and not under a
// no-aggro-target grace flag (e.g. post-respawn grace) — mirroring the
// eligibility filter mobcommands.LookForTrouble applies to its own
// candidate pool. Returns 0 if no eligible player is present.
func pickEligibleRoomPlayer(room *rooms.Room) int {
	candidates := make([]int, 0, 4)
	for _, playerId := range room.GetPlayers(rooms.FindAll) {
		user := users.GetByUserId(playerId)
		if user == nil {
			continue
		}
		if user.Character.HasConditionFlag(conditions.NoAggroTarget) {
			continue
		}
		if user.Character.Health < 1 {
			continue
		}
		if user.Character.IsHidden() {
			continue
		}
		candidates = append(candidates, playerId)
	}
	if len(candidates) == 0 {
		return 0
	}
	return candidates[util.Rand(len(candidates))]
}

func actCommand(params map[string]any, ctx *EvalContext) Result {
	mob := mobs.GetInstance(ctx.InstanceId)
	if mob == nil {
		return Failure
	}
	cmd := getStringParam(params, "cmd")
	if cmd == "" {
		return Failure
	}
	mob.Command(cmd)
	return Success
}

// actCommandMob finds the first mob in the room with the given mob_id and
// issues it a command string. Returns Failure if no matching mob is found.
// params: mob_id (int), cmd (string)
func actCommandMob(params map[string]any, ctx *EvalContext) Result {
	targetMobId := getIntParam(params, "mob_id")
	if targetMobId == 0 {
		return Failure
	}
	cmd := getStringParam(params, "cmd")
	if cmd == "" {
		return Failure
	}
	room := rooms.LoadRoom(ctx.RoomId)
	if room == nil {
		return Failure
	}
	for _, instId := range room.GetMobs(rooms.FindAll) {
		m := mobs.GetInstance(instId)
		if m != nil && int(m.MobId) == targetMobId {
			m.Command(cmd)
			return Success
		}
	}
	return Failure
}

// actCommandBestOf iterates the `cmds` list in order; for each, checks
// actions.CommandIsReady and issues the first ready command via mob.Command.
// Returns Success if any command was issued, Failure if all were not-ready.
//
// Mirrors cast_best_in_category for command-style moves. Used by the
// tank_taunter and generic_fighter archetypes.
//
// params: cmds (list of strings — command names in priority order)
func actCommandBestOf(params map[string]any, ctx *EvalContext) Result {
	mob := mobs.GetInstance(ctx.InstanceId)
	if mob == nil {
		return Failure
	}
	cmds := getStringListParam(params, "cmds")
	for _, cmd := range cmds {
		if actions.CommandIsReady(&actions.MobActor{Mob: mob}, cmd) {
			mob.Command(cmd)
			return Success
		}
	}
	return Failure
}

// beastMoves is the set of special-move names gated by the species'
// beast_moves list. Every one but drain also requires a body without hands,
// so a humanoid mob passes none of them unless its species lists drain (the
// vampire has hands and drains). Filtering on this set lets the tactical
// cascade (command_best_of bash/grapple/trip) still fire for a humanoid
// whose species lists no beast move.
var beastMoves = map[string]bool{
	"rake":      true,
	"maul":      true,
	"pounce":    true,
	"gore":      true,
	"throttle":  true,
	"hamstring": true,
	"drain":     true,
	"smother":   true,
}

// actTrySpecialMove delegates to combat.SelectSpecialMove (the chance-gate-free
// weighted selector) and issues the chosen command when it is a beast move.
// Returns Success when a beast move was issued; Failure otherwise so the
// existing tactical cascade (command_best_of) retains control.
//
// Beast moves are anatomy/identity-gated in their CanUse* functions, so a
// humanoid mob's SelectSpecialMove will never return one — the filter only
// costs a map lookup.
func actTrySpecialMove(params map[string]any, ctx *EvalContext) Result {
	mob := mobs.GetInstance(ctx.InstanceId)
	if mob == nil || !mob.Character.IsInCombat() {
		return Failure
	}

	// Resolve the aggro target's Character.
	tgt := mob.Character.CurrentCombatTarget()
	var target *characters.Character
	if tgt.UserId > 0 {
		if u := users.GetByUserId(tgt.UserId); u != nil {
			target = u.Character
		}
	} else if tgt.MobInstanceId > 0 {
		if tm := mobs.GetInstance(tgt.MobInstanceId); tm != nil {
			target = &tm.Character
		}
	}
	if target == nil {
		return Failure
	}

	move := combat.SelectSpecialMove(mob, target)
	if move == "" || !beastMoves[move] {
		// Either no move available or a humanoid tactical move — let the
		// existing command_best_of cascade handle it.
		return Failure
	}
	mob.Command(move)
	return Success
}

// actGoToCallerRoom resolves the event's caller mob (ctx.Event.MobId)
// and issues a `go <exit>` command toward the exit in the self's room
// that leads to the caller's room. Returns Failure if the caller mob
// or the connecting exit can't be resolved.
//
// Used by heard_callforhelp handlers in archetypes (generic_fighter,
// lookout) to navigate back toward a packmate who called for help.
func actGoToCallerRoom(params map[string]any, ctx *EvalContext) Result {
	self := mobs.GetInstance(ctx.InstanceId)
	if self == nil {
		return Failure
	}
	callerId := ctx.Event.MobId
	if callerId == 0 {
		return Failure
	}
	caller := mobs.GetInstance(callerId)
	if caller == nil {
		return Failure
	}
	myRoom := rooms.LoadRoom(self.Character.RoomId)
	if myRoom == nil {
		return Failure
	}
	for exitName, info := range myRoom.Exits {
		if info.RoomId == caller.Character.RoomId {
			// Movement parity 4b: a tired mob fails the node so a selector
			// falls through, rather than issuing a step it cannot pay.
			if !actions.QuoteMobStep(self, exitName).OK() {
				return Failure
			}
			self.Command(fmt.Sprintf("go %s", exitName))
			return Success
		}
	}
	return Failure
}

// actSweepCompanions relocates every player-in-room's live companions to a
// destination room (the "airlock"), gear intact. Used by the Hull Sweeper
// boss add to shove conjured allies (golems/wolves/undead) out of the boss
// fight without destroying them.
//
// params: dest_room (int) — the room id companions are pushed into.
//
// Delegates to hooks.PushCompanionsToRoom via the companionSweep callback
// (wired in main.go) to avoid a behaviortree → hooks import cycle (hooks
// already imports behaviortree).
func actSweepCompanions(params map[string]any, ctx *EvalContext) Result {
	if companionSweep == nil {
		return Failure
	}
	destRoomId := getIntParam(params, "dest_room")
	if destRoomId == 0 {
		return Failure
	}
	room := rooms.LoadRoom(ctx.RoomId)
	if room == nil {
		return Failure
	}
	for _, playerId := range room.GetPlayers() {
		companionSweep(playerId, destRoomId)
	}
	return Success
}

func actMove(params map[string]any, ctx *EvalContext) Result {
	mob := mobs.GetInstance(ctx.InstanceId)
	if mob == nil {
		return Failure
	}
	direction := getStringParam(params, "direction")
	if direction == "" {
		return Failure
	}
	// Movement parity 4b: fail when the mob cannot pay for the step.
	if !actions.QuoteMobStep(mob, direction).OK() {
		return Failure
	}
	mob.Command("go " + direction)
	return Success
}

// actCreateInstance clones an instanced zone and stores the resulting entry
// room ID in BehaviorState under the key "instance_entry_room_id". The
// caller should follow up with add_temp_exit using that state value.
//
// params:
//
//	zone_name   (string) — zone template name (e.g. "Instance Arena")
//	gold_amount (int)    — gold investment; must be >= 100
//	state_key   (string, default "instance_entry_room_id") — key to store result
//
// Precondition: take_gold should be called before this action. If instance
// creation fails, Returns Failure (caller should refund gold via give_gold).
func actCreateInstance(params map[string]any, ctx *EvalContext) Result {
	if ctx.MobState == nil {
		return Failure
	}
	zoneName := getStringParam(params, "zone_name")
	if zoneName == "" {
		return Failure
	}
	goldAmount := getIntParam(params, "gold_amount")
	if goldAmount <= 0 {
		return Failure
	}
	userId := ctx.Event.UserId
	if userId == 0 {
		return Failure
	}

	// Build authorized user list: owner + any party members.
	authorizedUsers := []int{userId}
	if party := parties.Get(userId); party != nil {
		for _, memberId := range party.GetMembers() {
			if memberId != userId {
				authorizedUsers = append(authorizedUsers, memberId)
			}
		}
	}

	inst, err := rooms.CreateZoneInstance(zoneName, goldAmount, userId, authorizedUsers, ctx.RoomId)
	if err != nil {
		mudlog.Error("actCreateInstance", "zone", zoneName, "userId", userId, "error", err)
		return Failure
	}

	stateKey := getStringParam(params, "state_key")
	if stateKey == "" {
		stateKey = "instance_entry_room_id"
	}
	ctx.MobState.Set(stateKey, inst.EntryRoomId)
	return Success
}

// actOpenInstancePortal starts the portal-vendor flow for the zone a talk
// topic names (#375 R14): it shows the zone's guidance and asks the player
// for the stake through a private amount prompt (startStakePrompt). The
// answer opens through openInstancePortal, which checks the minimum and
// the purse, charges, clones the instance, wires up the temporary exit and
// speaks the result; failures refund.
//
// params:
//
//	zones        (map: shortname → template zone name, e.g. arena: "Instance Arena")
//	zone         (string, required: a key of zones)
//	min_gold     (int, default 100)
//	exit_expires (string, default "30 real minutes")
//	guidance     (string, optional: shown above the amount question)
//
// A tree whose zone is missing or is not a key of zones does not load
// (checkOpenInstancePortalParams).
func actOpenInstancePortal(params map[string]any, ctx *EvalContext) Result {
	mob := mobs.GetInstance(ctx.InstanceId)
	if mob == nil {
		return Failure
	}
	user := users.GetByUserId(ctx.Event.UserId)
	if user == nil {
		return Failure
	}
	zone := portalZone(params)
	req, ok := portalRequestFor(params, zone)
	if !ok {
		mudlog.Error("actOpenInstancePortal", "error", "zone is not a key of zones", "zone", zone, "mobId", ctx.MobId)
		return Failure
	}
	startStakePrompt(user, mob, ctx.RoomId, req, getStringParam(params, "guidance"))
	return Success
}

// portalZone is the branch's zone param, trimmed and lowered.
func portalZone(params map[string]any) string {
	return strings.ToLower(strings.TrimSpace(getStringParam(params, "zone")))
}

// checkOpenInstancePortalParams refuses, at load, an open_instance_portal
// node whose zone is missing or is not a key of zones: with no player
// words to read, such a node could never open anything.
func checkOpenInstancePortalParams(params map[string]any) error {
	zone := portalZone(params)
	if zone == "" {
		return fmt.Errorf("open_instance_portal needs a zone param (a key of zones)")
	}
	if _, ok := portalRequestFor(params, zone); !ok {
		return fmt.Errorf("open_instance_portal zone %q is not a key of zones", zone)
	}
	return nil
}

// portalRequest is a resolved instance-zone buy-in: everything the open
// needs except the stake, which the amount prompt carries to
// openInstancePortal.
type portalRequest struct {
	Zone         string // short name, the zones key ("arena")
	TemplateZone string // the zone cloned ("Instance Arena")
	MinGold      int
	Expires      string
}

// portalRequestFor resolves a zones key against the action's params,
// applying the min_gold and exit_expires defaults. It reports false when
// zoneName is not a key of zones.
func portalRequestFor(params map[string]any, zoneName string) (portalRequest, bool) {
	templateZone := ""
	if zonesRaw, ok := params["zones"]; ok {
		switch zm := zonesRaw.(type) {
		case map[string]any:
			if v, ok := zm[zoneName]; ok {
				templateZone, _ = v.(string)
			}
		case map[any]any:
			if v, ok := zm[zoneName]; ok {
				templateZone, _ = v.(string)
			}
		case map[string]string:
			templateZone = zm[zoneName]
		}
	}
	if templateZone == "" {
		return portalRequest{}, false
	}

	minGold := getIntParam(params, "min_gold")
	if minGold <= 0 {
		minGold = 100
	}
	expires := getStringParam(params, "exit_expires")
	if expires == "" {
		expires = "30 real minutes"
	}
	return portalRequest{Zone: zoneName, TemplateZone: templateZone, MinGold: minGold, Expires: expires}, true
}

// openInstancePortal opens the portal once the amount prompt has a stake:
// the minimum and purse checks, the charge, the
// clone (scaled by the stake), the temporary exit and the mob's lines.
// Every outcome is spoken by the mob; failures refund.
func openInstancePortal(user *users.UserRecord, mob *mobs.Mob, roomId int, req portalRequest, goldAmount int) {
	zoneName, templateZone, minGold := req.Zone, req.TemplateZone, req.MinGold
	if goldAmount < minGold {
		mob.Command(fmt.Sprintf("say That barely covers the runes. I need at least %s gold.", util.FormatNumber(minGold)))
		return
	}
	if user.Character.Gold < goldAmount {
		mob.Command("say You do not have that much gold.")
		return
	}

	// Charge gold before creating the instance.
	user.Character.Gold -= goldAmount

	authorizedUsers := []int{user.UserId}
	if party := parties.Get(user.UserId); party != nil {
		for _, memberId := range party.GetMembers() {
			if memberId != user.UserId {
				authorizedUsers = append(authorizedUsers, memberId)
			}
		}
	}

	inst, err := createZoneInstance(templateZone, goldAmount, user.UserId, authorizedUsers, roomId)
	if err != nil {
		mudlog.Error("actOpenInstancePortal", "zone", templateZone, "userId", user.UserId, "error", err)
		user.Character.Gold += goldAmount // refund
		mob.Command("say The planes resist me. Your gold is returned.")
		return
	}

	exitKey := zoneName + "-" + fmt.Sprintf("%d", user.UserId)
	room := rooms.LoadRoom(roomId)
	if room == nil {
		user.Character.Gold += goldAmount // refund
		mudlog.Error("actOpenInstancePortal", "zone", templateZone, "userId", user.UserId, "error", "room did not load", "roomId", roomId)
		mob.Command("say The way will not open. Your gold is returned.")
		return
	}
	added := room.AddTemporaryExit(exitKey, exit.TemporaryRoomExit{
		RoomId:  inst.EntryRoomId,
		Title:   exitKey + " rift",
		UserId:  user.UserId,
		Expires: req.Expires,
	})
	if !added {
		user.Character.Gold += goldAmount // refund
		mob.Command("say The archway is already sustaining too many rifts. Your gold is returned.")
		return
	}

	mob.Command("say The rift is open. Type " + exitKey + " to enter.")
	mob.Command("say It will hold for a time. Do not tarry.")
	room.SendTextToExits("You hear someone talking.", true)
	room.SendText(messaging.CategoryMobEmote, "The stone archway flares with energy. A shimmering portal appears.")
}
