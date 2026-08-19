package usercommands

import (
	"fmt"
	"math"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

type throwItemLocation uint8

const (
	throwItemBackpack throwItemLocation = iota
	throwItemBandolier
)

type stagedThrowTarget struct {
	instanceID int
	mob        *mobs.Mob
}

var admitThrowCost = func(char *characters.Character, base float64) characters.CostCommitResult {
	quote := char.QuoteActionCost(characters.ActionCostRequest{
		Action: costs.ActionThrow, Pool: characters.PoolStamina,
		Base: base, Modifier: 1, Units: 1,
	})
	return char.CommitCost(quote, characters.CostFullOrRefuse)
}

func findThrowItem(char *characters.Character, noun string) (items.Item, throwItemLocation, bool) {
	if match, found := char.FindInBackpack(noun); found {
		return match, throwItemBackpack, true
	}
	match, found := char.FindInPotions(noun)
	return match, throwItemBandolier, found
}

func revalidateThrowItem(char *characters.Character, snapshot items.Item, location throwItemLocation) (items.Item, bool) {
	pool := char.Items
	if location == throwItemBandolier {
		pool = char.PotionItems
	}
	for _, current := range pool {
		if current.Equals(snapshot) && current.Uses == snapshot.Uses && current.GetSpec().Subtype == items.Throwable {
			return current, true
		}
	}
	return items.Item{}, false
}

func stageThrowTargets(room *rooms.Room) []stagedThrowTarget {
	targets := make([]stagedThrowTarget, 0)
	for _, instanceID := range room.GetMobs(rooms.FindAll) {
		mob := mobs.GetInstance(instanceID)
		if mob == nil || mobs.CheckPlayerHarm(mob).Blocked() {
			continue
		}
		targets = append(targets, stagedThrowTarget{instanceID: instanceID, mob: mob})
	}
	return targets
}

func revalidateThrowTarget(room *rooms.Room, staged stagedThrowTarget, liveIDs map[int]bool) *mobs.Mob {
	if !liveIDs[staged.instanceID] {
		return nil
	}
	mob := mobs.GetInstance(staged.instanceID)
	if mob == nil || mob != staged.mob || mob.Character.RoomId != room.RoomId || mobs.CheckPlayerHarm(mob).Blocked() {
		return nil
	}
	return mob
}

// maybeInterruptOnThrow cancels a mob's in-progress fold-cast if the thrown
// item id is a configured boss-interrupt disruptor
// (Balance.BossInterruptItemIds) AND the mob is currently casting
// (Character.IsCasting()). Generic (non-allowlisted) thrown items never
// interrupt a cast, even on a hit. Reuses the shared InterruptTargetCast
// primitive (conviction refund + TriggerCastCancel) rather than reimplementing
// cast cancellation here. Returns true if a cast was actually interrupted.
func maybeInterruptOnThrow(mob *mobs.Mob, thrownItemId int, by state.ActorRef) bool {
	if mob == nil {
		return false
	}
	if !configs.GetBalanceConfig().IsBossInterruptItem(thrownItemId) {
		return false
	}
	if !mob.Character.IsCasting() {
		return false
	}
	return actions.InterruptTargetCast(&mob.Character, by)
}

// engageAfterThrow puts the thrower and everything the throw actually hit into
// combat with each other, and records the aggression against each mob hit.
//
// Throwing used to be refused outright unless melee was already joined, so a
// thrown explosive could never open a fight -- which is the one thing a
// firebomb is for. Allowing the opener means the throw has to start the fight
// itself: ApplyHarm is a pure pool change and sets no aggro, so without this a
// player could bomb a room from outside combat and neither side would engage.
//
// Existing aggro on either side is left alone. Re-pointing the thrower would
// yank them off their chosen target mid-fight, and re-pointing a mob would pull
// it off whoever it was already fighting.
//
// Freshness is judged PER MOB, from the mob's own prior aggro, which is where
// this departs from the single-target moves. Those compare the attacker's aggro
// against the one target they hit; a throw hits several at once and the
// attacker's aggro can only point at one of them, so attacker-side gating would
// record the assault on the first mob and quietly miss the rest. A mob that was
// already fighting you is not freshly assaulted by the next grenade.
func engageAfterThrow(user *users.UserRecord, room *rooms.Room, hitMobs []*mobs.Mob) {
	if user == nil || len(hitMobs) == 0 {
		return
	}
	thrower := user.Character

	for _, mob := range hitMobs {
		if mob == nil {
			continue
		}
		freshAggro := mob.Character.Aggro == nil
		if freshAggro {
			mob.Character.SetAggro(user.UserId, 0, characters.DefaultAttack)
		}
		actions.SeedAggression(user, mob, room, freshAggro)
	}

	if thrower == nil || thrower.Aggro != nil {
		return
	}
	for _, mob := range hitMobs {
		if mob != nil {
			thrower.SetAggro(0, mob.InstanceId, characters.DefaultAttack)
			return
		}
	}
}

func Throw(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if refuseWhileBusy(user, `throw anything`) {
		return true, nil
	}

	if rest == "" {
		user.SendText(messaging.CategorySystem, "Throw what? Specify a throwable item.")
		return true, nil
	}

	// Deliberately NOT gated on being in combat. A thrown explosive is a ranged
	// opener; requiring melee first made the whole throwable category unusable
	// for the thing it is for. engageAfterThrow starts the fight for a throw
	// that connects.
	//
	// DESIGN DECISION (2026-08-14): `throw` is the GRENADE verb and stays
	// untargeted on purpose. It takes an item, never a target, and resolves as
	// a room AoE rolled independently against every hostile present -- the same
	// shape as an AoE spell. Do not add a target argument here.
	//
	// Aimed thrown weapons (darts, javelins, throwing knives) belong under
	// ranged-combat and ExecuteFire in internal/actions, which already has
	// single-target resolution, Perception-based aiming and the reload
	// machinery. Skullduggery suits an improvised explosive; it does not suit a
	// javelin. See the matching note on ExecuteFire for the one open problem
	// (a thrown weapon is its own ammunition).

	cfg := configs.GetBalanceConfig()

	// Find throwable in backpack or bandolier
	matchItem, itemLocation, found := findThrowItem(user.Character, rest)
	if !found {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`You don't have a "%s" to throw.`, rest))
		return true, nil
	}

	spec := matchItem.GetSpec()
	if spec.Subtype != items.Throwable {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="itemname">%s</ansi> isn't something you can throw.`, matchItem.DisplayName()))
		return true, nil
	}

	// Determine spoilage without consuming the item. Every mutation waits until
	// full cost admission and a consuming cooldown both succeed.
	spoiled := false
	if spec.Aging.HasAging() && matchItem.CraftedRound > 0 {
		currentRound := util.GetRoundCount()
		var elapsed uint64
		if currentRound >= matchItem.CraftedRound {
			elapsed = currentRound - matchItem.CraftedRound
		}
		effSpeed := items.CalcEffectiveAgingSpeed(matchItem.BottleMultiplier, matchItem.CraftSkill)
		phase, _ := items.GetAgingPhase(elapsed, spec.Aging, effSpeed)
		spoiled = phase == items.PhaseSpoiled
	}

	stagedTargets := stageThrowTargets(room)
	if !user.Character.CooldownReady("special-move") {
		user.SendText(messaging.CategorySystem, "You need a moment to recover before attempting another special move.")
		return true, nil
	}

	cost := admitThrowCost(user.Character, float64(cfg.SpecialMoveBaseStaminaCost))
	if cost.Status == characters.CostRefused {
		user.SendText(messaging.CategorySystem, actions.CostRefusalText(cost))
		return true, nil
	}

	// Admission can synchronously invalidate a staged inventory identity in a
	// test seam (or future hook). The paid admission remains paid, but no other
	// item, cooldown, resolution, progression, or round is mutated.
	matchItem, found = revalidateThrowItem(user.Character, matchItem, itemLocation)
	if !found {
		return true, nil
	}
	if !user.Character.TryCooldown("special-move", fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown)) {
		return true, nil
	}

	// Consume the item
	usesLeft := 0
	if itemLocation == throwItemBandolier {
		usesLeft = user.Character.UseItemFromPotions(matchItem)
	} else {
		usesLeft = user.Character.UseItem(matchItem)
	}
	if usesLeft < 1 {
		events.AddToQueue(events.ItemOwnership{
			UserId: user.UserId,
			Item:   matchItem,
			Gained: false,
		})
	}
	if spoiled {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="red-bold">You pull out the <ansi fg="itemname">%s</ansi> and it crumbles apart in your hand -- it's gone bad!</ansi>`,
			matchItem.DisplayName()))
		return true, nil
	}

	// Quest engine: command notification — a successful throw advances
	// "throw a grenade" quest steps (e.g. the Spoke C grenade lesson).
	questBridge := questengine.NewGameBridge(user, room.RoomId)
	questengine.GetEngine().Notify("command", questengine.EventDetails{
		UserId:  user.UserId,
		RoomId:  room.RoomId,
		Command: "throw",
	}, questBridge, questBridge)

	// AoE resolution: opposed roll per hostile mob in room
	skillWeight := float64(cfg.SkillWeight)
	skullduggery := user.Character.GetSkillLevel(skills.Skullduggery)
	dexterity := user.Character.GetEffectiveDexterity()

	attackerScore := float64(dexterity) + float64(skullduggery)*skillWeight

	user.SendText(messaging.CategorySystem, fmt.Sprintf(
		`<ansi fg="yellow-bold">You hurl the <ansi fg="itemname">%s</ansi> into the fray!</ansi>`,
		matchItem.DisplayName()))
	room.SendTextVisual(messaging.CategoryHitRanged, fmt.Sprintf(
		`<ansi fg="yellow-bold"><ansi fg="username">%s</ansi> hurls a <ansi fg="itemname">%s</ansi> into the fray!</ansi>`,
		user.Character.Name, matchItem.DisplayName()),
		user.UserId)

	hasDamage := spec.DamageMultiplier > 0
	hasBuffs := len(spec.BuffIds) > 0
	hitCount := 0
	fumbled := false
	// Everything the throw connected with, so an out-of-combat opener can
	// engage both sides once the blast is resolved. See engageAfterThrow.
	hitMobs := []*mobs.Mob{}

	liveTargetIDs := make(map[int]bool, len(stagedTargets))
	for _, instanceID := range room.GetMobs(rooms.FindAll) {
		liveTargetIDs[instanceID] = true
	}
	for _, stagedTarget := range stagedTargets {
		mob := revalidateThrowTarget(room, stagedTarget, liveTargetIDs)
		if mob == nil {
			continue
		}

		defenderScore := float64(mob.Character.GetEffectiveDexterity()) +
			float64(mob.Character.GetEffectivePerception())*skillWeight*0.5

		res := combat.RunContest(attackerScore, []contest.Entry{{Score: defenderScore}})

		// Fumble check: effect hits thrower instead
		if res.AttackRoll.ZScore <= -2.0 {
			fumbled = true
			user.SendText(messaging.CategorySystem, `<ansi fg="red-bold">Your throw goes horribly wrong — the projectile detonates in your hand!</ansi>`)
			room.SendTextVisual(messaging.CategoryHitRanged, fmt.Sprintf(
				`<ansi fg="red"><ansi fg="username">%s</ansi>'s throw backfires spectacularly!</ansi>`,
				user.Character.Name), user.UserId)

			// Apply effects to thrower
			if hasDamage {
				rawDmg := combat.CalcRawDamage(dexterity, skullduggery, spec.DamageMultiplier, combat.ChannelPhysical)
				dmg := int(math.Round(rawDmg))
				if dmg < 1 {
					dmg = 1
				}
				user.Character.ApplyHarm(characters.PoolHealth, dmg,
					state.ActorRef{UserId: user.UserId})
				dmgDesc := combat.GetDamageDescription(dmg, user.Character.HealthMax.Value)
				user.SendText(messaging.CategorySystem, fmt.Sprintf(
					`<ansi fg="red">The explosion sears you! (%s)</ansi>`, dmgDesc))
			}
			if hasBuffs {
				for _, buffId := range spec.BuffIds {
					user.AddBuff(buffId, `grenade-fumble`)
				}
			}
			break // Fumble ends the AoE loop
		}

		// Boss-interrupt: a configured disruptor thrown at a mid-fold-cast mob
		// cancels the cast whether or not the throw wins its damage roll — the
		// interrupt is the disruptor's purpose, not a side effect of a hit. Fires
		// before the attack-success gate so a tanky boss can't simply dodge the
		// interrupt. (Fumbles break above, so a botched throw still can't cancel.)
		if maybeInterruptOnThrow(mob, matchItem.ItemId, state.ActorRef{UserId: user.UserId}) {
			user.SendText(messaging.CategorySpellDisruption, fmt.Sprintf(
				`<ansi fg="cyan-bold">The blast shatters %s's concentration -- its spell collapses!</ansi>`,
				mob.Character.Name))
			room.SendTextVisual(messaging.CategorySpellDisruption, fmt.Sprintf(
				`<ansi fg="cyan">%s's spell collapses as the blast strikes!</ansi>`,
				mob.Character.Name), user.UserId)
		}

		if !res.Success {
			continue
		}

		hitCount++
		hitMobs = append(hitMobs, mob)

		// Apply effects to mob
		if hasDamage {
			rawDmg := combat.CalcRawDamage(dexterity, skullduggery, spec.DamageMultiplier, combat.ChannelPhysical)
			mitPct := mob.Character.GetPhysicalMitigation()
			mitCap := combat.MitigationCap(combat.ChannelPhysical)
			mitigated := combat.ApplyMitigation(rawDmg, mitPct, mitCap)
			finalRoll := dice.RollStat(mitigated)
			dmg := int(math.Round(finalRoll.Value))
			if dmg < 1 {
				dmg = 1
			}
			mob.Character.ApplyHarm(characters.PoolHealth, dmg,
				state.ActorRef{UserId: user.UserId})
			dmgDesc := combat.GetDamageDescription(dmg, mob.Character.HealthMax.Value)
			user.SendText(messaging.CategorySystem, fmt.Sprintf(
				`The explosion catches <ansi fg="mobname">%s</ansi>! (<ansi fg="damage">%s</ansi>)`,
				mob.Character.Name, dmgDesc))
		}

		if hasBuffs {
			for _, buffId := range spec.BuffIds {
				mob.AddBuff(buffId, `grenade`)
			}
			user.SendText(messaging.CategorySystem, fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi> is caught in the blast!`,
				mob.Character.Name))
		}

		combat.RecordSpecialMove(combat.User, combat.Mob, "throw",
			true, 0, user.Character, &mob.Character, util.GetRoundCount())
	}

	if !fumbled {
		if hitCount == 0 {
			user.SendText(messaging.CategorySystem, "The projectile misses everything and shatters harmlessly.")
		} else if hitCount == 1 {
			user.SendText(messaging.CategorySystem, fmt.Sprintf(
				`<ansi fg="green">Your throw strikes true!</ansi>`))
		} else {
			user.SendText(messaging.CategorySystem, fmt.Sprintf(
				`<ansi fg="green">Your throw catches multiple targets!</ansi>`))
		}
	}

	// Progress skullduggery skill (also advances dexterity as primary stat)
	user.Character.OnSkillUse(string(skills.Skullduggery), user.UserId)

	// A throw that connected starts the fight if one was not already running.
	// A fumble hurts only the thrower, so it engages nobody.
	if !fumbled {
		engageAfterThrow(user, room, hitMobs)
	}

	if user.Character.Aggro != nil {
		user.Character.Aggro.RoundsWaiting = 1
	}

	return true, nil
}
