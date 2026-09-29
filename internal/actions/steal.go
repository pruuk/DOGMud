package actions

import (
	"fmt"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/crimes"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/factions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/justice"
	"github.com/GoMudEngine/GoMud/internal/knowledge"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/seeders"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// StealOptions parameterizes a theft attempt.
// Exactly one of TargetMobInstanceId / TargetUserId / ContainerNoun /
// HouseholdItem must be set. ItemNoun narrows the steal to a specific item;
// when empty, the action defaults to gold-or-random-item per the
// existing player-side logic.
type StealOptions struct {
	TargetMobInstanceId int
	TargetUserId        int
	ContainerNoun       string
	ItemNoun            string
	// HouseholdItem is a bauble on the floor that belongs to this room's
	// household (found by searching their home: household_bauble.go).
	// Taking it is the container theft's contest (stealObserverPass).
	HouseholdItem items.Item
}

// StealResult is the structured outcome of a steal attempt.
type StealResult struct {
	Succeeded     bool   // skill check passed and transfer happened
	Detected      bool   // detection roll fired and the defender noticed
	StoleGold     int    // gold transferred (0 if item-only or failed)
	StoleItemId   int    // item id transferred (0 if gold-only or failed)
	StoleItemName string // for messaging
	DefenderName  string // who/what was robbed
	OnCooldown    bool   // attempt was blocked by skullduggery cooldown
	Reason        string // when Succeeded==false and !OnCooldown, why
	// Pending: a player's pickpocket of an NPC was rolled and its outcome
	// is held back for the pause (steal_pocket.go); it is revealed, and
	// its loot handed over, when the pause ends.
	Pending bool
}

// stealVictimScore is the defender's half of every theft/plant contest:
// Perception + skullduggery x SkillWeight. Before U6b Task 15 this was raw
// Perception -- the defender's skill entered the contest at x0.
//
// The counter-craft is SKULLDUGGERY, and that is a deliberate, documented
// choice: noticing fingers in your pocket (or a hand slipping something into
// a chest you are standing next to) is craft knowledge -- a practiced thief
// recognises the technique being worked on them. Search is the ACTIVE
// looking skill and already answers the sneak/hidden-detection family via
// CalcDetectionScore (Task 16); routing theft defence through search would
// double-book that skill and leave the thief's own craft worthless on
// defence.
//
// There is deliberately NO crit tier anywhere in steal/plant: the outcomes
// are caught/unseen, not damage, so a margin-scaled multiplier has nothing
// to scale (spec section 4.5: a documented reason where a defence set is not
// meaningful).
//
// room is the VICTIM's room (lighting plan 5b): noticing a hand in your pocket
// is an observer's roll, so the victim or bystander pays their own sight ramp
// (messaging.SightMult) here, once. The thief pays theirs on the attack
// score in Steal/Plant. A nil room is unity; pass combat.SightRoom for a
// *rooms.Room that may be nil.
//
// A sleeping merchant's Perception counts at
// merchantchests.SleepingPerceptionMult (sleepingMerchantPerceptionMult,
// merchant_chest.go); CalcDetectionScore applies the same factor.
func stealVictimScore(c *characters.Character, room messaging.RoomVisibility) float64 {
	return (float64(c.Stats.Perception.ValueAdj)*sleepingMerchantPerceptionMult(c) +
		float64(c.GetSkillLevel(skills.Skullduggery))*
			float64(configs.GetBalanceConfig().SkillWeight)) *
		messaging.SightMult(c, room)
}

// Steal runs a skullduggery theft attempt from actor against the
// resolved target. Both UserActor and MobActor are supported.
func Steal(actor Actor, opts StealOptions) StealResult {
	char := actor.GetCharacter()

	// Combat gate — can't steal while actively fighting.
	if char.IsInCombat() {
		actor.SendText(messaging.CategorySystem, "You can't do that while in combat!")
		return StealResult{Reason: "in combat"}
	}

	room := actor.GetRoom()
	if room == nil {
		return StealResult{Reason: "no room"}
	}

	// Under-attack gate — mobs with userId=0 short-circuit this harmlessly.
	if room.AreMobsAttacking(actor.GetUserId()) {
		actor.SendText(messaging.CategorySystem, "You can't do that while you are under attack!")
		return StealResult{Reason: "under attack"}
	}

	// Require a target.
	if opts.TargetMobInstanceId == 0 && opts.TargetUserId == 0 &&
		opts.ContainerNoun == "" && opts.HouseholdItem.ItemId == 0 {
		actor.SendText(messaging.CategorySystem, "Steal from whom?")
		return StealResult{Reason: "no target"}
	}

	cfg := configs.GetBalanceConfig()
	cooldownKey := skills.Skullduggery.String(`steal`)

	// One pickpocket at a time: a thief still in one's pause starts no other
	// (the cooldown normally covers this, but it may be set to nothing).
	if actor.IsPlayer() && pocketPending(actor.GetUserId()) {
		actor.SendText(messaging.CategorySystem, "Your hand is still in someone's pocket.")
		return StealResult{Reason: "busy"}
	}

	// Check cooldown before doing target resolution.
	if !char.TryCooldown(cooldownKey,
		fmt.Sprintf(`%d real seconds`, int(cfg.StealCooldown))) {
		return StealResult{
			OnCooldown: true,
			Reason: fmt.Sprintf("%d rounds remaining",
				char.GetCooldown(cooldownKey)),
		}
	}

	isHidden := char.IsHidden()

	// Compute attacker score. U6b Task 15: linear rank x SkillWeight, the
	// same shape as every other contest in the game. The old regime was
	// (Dex + sqrt-curve x25) times a steal-specific global balance knob;
	// both the sqrt term and the knob (deleted from the balance config)
	// die here.
	rank := char.GetSkillLevel(skills.Skullduggery)
	attackerScore := float64(char.Stats.Dexterity.ValueAdj) +
		float64(rank)*float64(cfg.SkillWeight)
	if isHidden {
		attackerScore += float64(cfg.StealHiddenBonus)
	}
	// sight ramp (plan 5b): the thief needs to see. Once here; the score
	// feeds all three theft contests below.
	attackerScore *= messaging.SightMult(char, room)

	// Dispatch to the appropriate path.
	if opts.TargetMobInstanceId > 0 {
		return stealFromMob(actor, opts.TargetMobInstanceId, attackerScore, rank, cfg)
	}

	if opts.TargetUserId > 0 {
		return stealFromPlayer(actor, opts.TargetUserId, attackerScore, rank, cfg)
	}

	if opts.HouseholdItem.ItemId > 0 {
		return stealHouseholdBauble(actor, opts.HouseholdItem, attackerScore, rank)
	}

	// Container path.
	return stealFromContainer(actor, opts.ContainerNoun, attackerScore, rank)
}

// stealFromMob handles the creature steal path. attackerScore arrives with
// the thief's sight ramp already applied in Steal; do not apply SightMult
// again.
func stealFromMob(actor Actor, mobInstanceId int, attackerScore float64,
	rank int, cfg configs.Balance) StealResult {

	m := mobs.GetInstance(mobInstanceId)
	if m == nil {
		actor.SendText(messaging.CategorySystem, "They seem to have vanished.")
		return StealResult{Reason: "target not found"}
	}

	// Deliberately NOT mobs.CheckPlayerHarm: that policy also blocks charmed
	// companions, and stealing from a companion is currently allowed. Widening
	// it here would be a gameplay change, not a finding-3 fix. Keep the two
	// protections that do apply.
	if m.IsNonCombatant() || m.PlayerAttackImmune {
		actor.SendText(messaging.CategorySystem, fmt.Sprintf(
			`You can't steal from <ansi fg="mobname">%s</ansi>.`,
			m.Character.Name))
		return StealResult{
			DefenderName: m.Character.Name,
			Reason:       "immune",
		}
	}

	// Skill rank 2 required for the actual steal mechanic. Checked
	// AFTER the immune gate so a low-rank thief targeting an immune mob
	// gets the immune rebuff (which doesn't imply skill would help)
	// rather than the skill rebuff (which does).
	if rank < 2 {
		actor.SendText(messaging.CategorySystem, "You aren't advanced enough at skullduggery for that.")
		return StealResult{
			DefenderName: m.Character.Name,
			Reason:       "not advanced enough",
		}
	}

	// Quest engine notification — player actors only.
	if actor.IsPlayer() {
		if u := users.GetByUserId(actor.GetUserId()); u != nil {
			room := actor.GetRoom()
			bridge := questengine.NewGameBridge(u, room.RoomId)
			questengine.GetEngine().Notify("command", questengine.EventDetails{
				UserId:  actor.GetUserId(),
				RoomId:  room.RoomId,
				Command: "steal",
			}, bridge, bridge)
		}
	}

	defenderScore := stealVictimScore(&m.Character, combat.SightRoom(actor.GetRoom()))
	success := combat.RunContest(attackerScore, []contest.Entry{{Score: defenderScore}}).Success
	// A player's pickpocket takes a moment, Dexterity-scaled: the roll above
	// is final, the outcome is revealed when the pause ends
	// (steal_pocket.go), and it is awarded THERE, with the outcome: awarded
	// now, its skill-up line would give the roll away before the reveal,
	// and a thief who walked off to dodge being caught would still train.
	// A mob thief's is at once.
	if actor.IsPlayer() {
		return startPocketAttempt(actor, m, success)
	}
	// U10b-1 Task 18: moved DOWN from before the contest, and it now carries
	// the outcome. This fired unconditionally at full weight -- the comment
	// it replaced said "always fire regardless of roll outcome" -- so a
	// thief who was caught trained exactly as much as one who got away.
	// This site is a CUT on failure.
	actor.AwardResolved(success, actor.GetCharacter().CandidateFor(string(skills.Skullduggery)))
	if success {
		return takeFromMob(actor, m, nil)
	}
	return caughtByMob(actor, m, actor.GetRoom())
}

// takeFromMob is a successful theft from a mob: some of its gold, one of
// its items at random, and extra (a pickpocketed bauble), with one line
// naming everything taken.
func takeFromMob(actor Actor, m *mobs.Mob, extra []items.Item) StealResult {
	{
		result := StealResult{
			Succeeded:    true,
			DefenderName: m.Character.Name,
		}

		stolenStuff := []string{}

		if m.Character.Gold > 0 {
			quarterGold := m.Character.Gold >> 2
			minGold := m.Character.Gold - quarterGold
			goldStolen := util.Rand(quarterGold) + minGold
			if goldStolen > 0 {
				m.Character.Gold -= goldStolen
				actor.GetCharacter().Gold += goldStolen
				result.StoleGold = goldStolen
				stolenStuff = append(stolenStuff,
					fmt.Sprintf(`<ansi fg="yellow-bold">%d gold</ansi>`, goldStolen))

				if actor.IsPlayer() {
					events.AddToQueue(events.EquipmentChange{
						UserId:     actor.GetUserId(),
						GoldChange: goldStolen,
					})
				}
			}
		}

		if itemStolen, found := m.Character.GetRandomItem(); found {
			m.Character.RemoveItem(itemStolen)
			actor.GetCharacter().StoreItem(itemStolen)
			if itemStolen.IsBauble() && actor.IsPlayer() {
				markPocketStolen(itemStolen, actor.GetUserId(), actor.GetRoom(), m)
			}
			result.StoleItemId = itemStolen.ItemId
			result.StoleItemName = itemStolen.DisplayName()

			events.AddToQueue(events.ItemOwnership{
				MobInstanceId: m.InstanceId,
				Item:          itemStolen,
				Gained:        false,
			})

			if actor.IsPlayer() {
				events.AddToQueue(events.ItemOwnership{
					UserId: actor.GetUserId(),
					Item:   itemStolen,
					Gained: true,
				})
			} else {
				events.AddToQueue(events.ItemOwnership{
					MobInstanceId: actor.GetMobInstanceId(),
					Item:          itemStolen,
					Gained:        true,
				})
			}

			stolenStuff = append(stolenStuff,
				fmt.Sprintf(`<ansi fg="itemname">%s</ansi>`, itemStolen.DisplayName()))
		}

		// A pickpocketed bauble, already out of the mark's pocket (or made
		// for it; the caller queued the mark's loss when it had one). Too
		// heavy to carry after all, it falls at the thief's feet.
		for _, b := range extra {
			if !actor.GetCharacter().StoreItem(b) {
				b.LeaveBaubleAt(``, 0, baubleNow())
				actor.GetRoom().AddItem(b, false)
				stolenStuff = append(stolenStuff,
					fmt.Sprintf(`<ansi fg="itemname">%s</ansi> (too much to carry: it falls at your feet)`, b.DisplayName()))
				continue
			}
			if actor.IsPlayer() {
				events.AddToQueue(events.ItemOwnership{UserId: actor.GetUserId(), Item: b, Gained: true})
			}
			stolenStuff = append(stolenStuff,
				fmt.Sprintf(`<ansi fg="itemname">%s</ansi>`, b.DisplayName()))
		}

		if len(stolenStuff) == 0 {
			actor.SendText(messaging.CategorySystem, fmt.Sprintf(
				`You deftly rifle through <ansi fg="mobname">%s</ansi>'s `+
					`belongings but find nothing worth taking.`,
				m.Character.Name))
		} else {
			actor.SendText(messaging.CategoryLoot, fmt.Sprintf(
				`You successfully steal %s from <ansi fg="mobname">%s</ansi>.`,
				strings.Join(stolenStuff, ` and `), m.Character.Name))

			// Rule 5: seed revenge goals on the victim and any witnesses
			// in the same room. Only fires for player thieves; mob-on-mob
			// theft is not a supported use-case for this path.
			if actor.IsPlayer() {
				seeders.OnTheft(actor.GetUserId(), m, items.Item{})
			}
		}

		return result
	}
}

// caughtByMob is a failed theft from a mob: caught in the act.
func caughtByMob(actor Actor, m *mobs.Mob, room *rooms.Room) StealResult {
	actor.SendText(messaging.CategorySystem, fmt.Sprintf(
		`<ansi fg="mobname">%s</ansi> catches you in the act!`,
		m.Character.Name))

	if room != nil {
		room.SendTextVisual(messaging.CategoryMobEmote,
			fmt.Sprintf(
				`<ansi fg="username">%s</ansi> gets caught trying to steal `+
					`from <ansi fg="mobname">%s</ansi>!`,
				actor.GetName(), m.Character.Name),
			actor.GetUserId(),
		)
	}

	thiefCaught(actor, m, room)

	return StealResult{
		Detected:     true,
		DefenderName: m.Character.Name,
		Reason:       "detected",
	}
}

// stealFromPlayer handles the mob-on-player theft path. The steal
// mechanic is symmetric with stealFromMob: the actor's Dex+skill
// score is rolled against the target player's Perception. On
// success, gold is lifted (no item steal against players). An
// independent detection roll then decides whether the victim
// notices. attackerScore arrives with the thief's sight ramp already
// applied in Steal; do not apply SightMult again.
func stealFromPlayer(actor Actor, targetUserId int, attackerScore float64,
	rank int, cfg configs.Balance) StealResult {

	targetUser := users.GetByUserId(targetUserId)
	if targetUser == nil {
		actor.SendText(messaging.CategorySystem, "They seem to have vanished.")
		return StealResult{Reason: "target not found"}
	}

	// Skill rank 2 required.
	if rank < 2 {
		actor.SendText(messaging.CategorySystem, "You aren't advanced enough at skullduggery for that.")
		return StealResult{
			DefenderName: targetUser.Character.Name,
			Reason:       "not advanced enough",
		}
	}

	defenderScore := stealVictimScore(targetUser.Character, combat.SightRoom(actor.GetRoom()))
	success := combat.RunContest(attackerScore, []contest.Entry{{Score: defenderScore}}).Success
	// U10b-1 Task 18: moved DOWN from before the contest, and it now carries
	// the outcome. This fired unconditionally at full weight -- the comment
	// it replaced said "always fire regardless of roll outcome" -- so a
	// thief who was caught trained exactly as much as one who got away.
	// This site is a CUT on failure.
	actor.AwardResolved(success, actor.GetCharacter().CandidateFor(string(skills.Skullduggery)))

	if !success {
		actor.SendText(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="username">%s</ansi> catches you in the act!`,
			targetUser.Character.Name))
		actor.GetCharacter().Awareness.TransitionToRevealing(state.TransitionReason{
			Trigger: awareness.TriggerSkullduggeryFailed,
		})

		// Chunk 3.3: failed theft wakes a sleeping victim.
		if targetUser.Character.HasConditionFlag(conditions.Sleeping) {
			targetUser.Character.CancelConditionsWithFlag(conditions.Sleeping)
			mobs.OnSleeperWoken(targetUser.Character)
		}

		return StealResult{
			Detected:     true,
			DefenderName: targetUser.Character.Name,
			Reason:       "detected",
		}
	}

	result := StealResult{
		Succeeded:    true,
		DefenderName: targetUser.Character.Name,
	}

	if targetUser.Character.Gold > 0 {
		quarterGold := targetUser.Character.Gold >> 2
		minGold := targetUser.Character.Gold - quarterGold
		goldStolen := util.Rand(quarterGold) + minGold
		if goldStolen > 0 {
			targetUser.Character.Gold -= goldStolen
			actor.GetCharacter().Gold += goldStolen
			result.StoleGold = goldStolen

			if actor.IsPlayer() {
				events.AddToQueue(events.EquipmentChange{
					UserId:     actor.GetUserId(),
					GoldChange: goldStolen,
				})
			}
			events.AddToQueue(events.EquipmentChange{
				UserId:     targetUserId,
				GoldChange: -goldStolen,
			})
		}
	}

	// Independent detection roll: victim may notice even on success.
	if !actor.IsPlayer() {
		searchScore := CalcDetectionScore(targetUser.Character, combat.SightRoom(actor.GetRoom()))
		roomLight := messaging.FixedLight(actor.GetRoom().LightLevel())
		sneakScore := CalcSneakScoreVsObserver(actor.GetCharacter(), targetUser.Character, roomLight)
		detected := combat.RunContest(searchScore, []contest.Entry{{Score: sneakScore}}).Success
		if detected {
			targetUser.SendText(messaging.CategorySystem, fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi> lifts `+
					`<ansi fg="yellow-bold">%d gold</ansi> from your pocket!`,
				actor.GetName(), result.StoleGold))
			result.Detected = true
		}
	}

	return result
}

// stealFromContainer handles the room-container steal path. attackerScore
// arrives with the thief's sight ramp already applied in Steal; do not apply
// SightMult again.
func stealFromContainer(actor Actor, containerName string,
	attackerScore float64, rank int) StealResult {

	room := actor.GetRoom()
	container, ok := room.Containers[containerName]
	if !ok {
		actor.SendText(messaging.CategorySystem, "You don't see that here.")
		return StealResult{Reason: "not found"}
	}

	// Skill rank 2 required for the actual steal mechanic. Mirrors the
	// gate in stealFromMob; checked AFTER target validation so the rebuff
	// order is consistent.
	if rank < 2 {
		actor.SendText(messaging.CategorySystem, "You aren't advanced enough at skullduggery for that.")
		return StealResult{Reason: "not advanced enough"}
	}

	// A locked container has to be picked first, as for get, look and put:
	// a quick hand is no way through a lock.
	if container.Lock.IsLocked() {
		actor.SendText(messaging.CategorySystem, fmt.Sprintf(
			`The <ansi fg="container">%s</ansi> is locked.`, containerName))
		return StealResult{Reason: "locked"}
	}

	if len(container.Items) == 0 && container.Gold == 0 {
		actor.SendText(messaging.CategorySystem, fmt.Sprintf(
			`You root around inside the <ansi fg="itemname">%s</ansi> `+
				`but find it empty.`,
			containerName))
		return StealResult{Reason: "empty"}
	}

	// Quest engine notification — player actors only.
	if actor.IsPlayer() {
		if u := users.GetByUserId(actor.GetUserId()); u != nil {
			bridge := questengine.NewGameBridge(u, room.RoomId)
			questengine.GetEngine().Notify("command", questengine.EventDetails{
				UserId:  actor.GetUserId(),
				RoomId:  room.RoomId,
				Command: "steal",
			}, bridge, bridge)
		}
	}

	success, spotterName, _ := stealObserverPass(actor, room, attackerScore)
	// U10b-1 Task 18: moved DOWN from before the contest. success here means
	// NOT SPOTTED: it starts true (no observer present is an uncontested
	// win) and only a lost observer contest clears it. Previously this
	// fired unconditionally at full weight, so being caught trained as
	// much as slipping away. A CUT on failure.
	actor.AwardResolved(success, actor.GetCharacter().CandidateFor(string(skills.Skullduggery)))

	if !success {
		actor.SendText(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi> spots you reaching into the `+
				`<ansi fg="itemname">%s</ansi>!`,
			spotterName, containerName))

		room.SendTextVisual(messaging.CategoryMobEmote,
			fmt.Sprintf(
				`<ansi fg="username">%s</ansi> is caught stealing from `+
					`the <ansi fg="itemname">%s</ansi>!`,
				actor.GetName(), containerName),
			actor.GetUserId(),
		)

		actor.GetCharacter().Awareness.TransitionToRevealing(state.TransitionReason{
			Trigger: awareness.TriggerSkullduggeryFailed,
		})

		// A merchant's chest (internal/merchantchests) with its merchant
		// present: the theft is from the merchant, a crime as stealing from
		// it would be (thiefCaught wakes it and records the crime).
		if owner := merchantChestOwner(room, containerName); owner != nil {
			merchantCatchesThief(actor, owner, room, containerName)
		}

		return StealResult{
			Detected:     true,
			DefenderName: spotterName,
			Reason:       "detected",
		}
	}

	// Success — take a random item (or gold) from the container.
	result := StealResult{
		Succeeded: true,
	}

	if len(container.Items) > 0 {
		idx := util.Rand(len(container.Items))
		stolen := container.Items[idx]
		container.RemoveItem(stolen)
		room.Containers[containerName] = container
		// A merchant's goods leaving its chest are stolen from here on:
		// hot in this area for a while (baubles.GoodsHotIn).
		stolen.MarkTaken(actor.GetUserId(), room.Zone, stolenNow())
		actor.GetCharacter().StoreItem(stolen)
		result.StoleItemId = stolen.ItemId
		result.StoleItemName = stolen.DisplayName()

		if actor.IsPlayer() {
			events.AddToQueue(events.ItemOwnership{
				UserId: actor.GetUserId(),
				Item:   stolen,
				Gained: true,
			})
		} else {
			events.AddToQueue(events.ItemOwnership{
				MobInstanceId: actor.GetMobInstanceId(),
				Item:          stolen,
				Gained:        true,
			})
		}

		actor.SendText(messaging.CategoryLoot, fmt.Sprintf(
			`You quietly slip <ansi fg="itemname">%s</ansi> from the `+
				`<ansi fg="itemname">%s</ansi>.`,
			stolen.DisplayName(), containerName))
		if stolen.IsStolen() {
			actor.SendText(messaging.CategorySystem, StolenGoodsNote(stolen))
		}
	} else {
		// Container only has gold.
		quarterGold := container.Gold >> 2
		minGold := container.Gold - quarterGold
		goldStolen := util.Rand(quarterGold) + minGold
		if goldStolen > 0 {
			container.Gold -= goldStolen
			room.Containers[containerName] = container
			actor.GetCharacter().Gold += goldStolen
			result.StoleGold = goldStolen

			if actor.IsPlayer() {
				events.AddToQueue(events.EquipmentChange{
					UserId:     actor.GetUserId(),
					GoldChange: goldStolen,
				})
			}

			actor.SendText(messaging.CategoryLoot, fmt.Sprintf(
				`You quietly lift <ansi fg="yellow-bold">%d gold</ansi> `+
					`from the <ansi fg="itemname">%s</ansi>.`,
				goldStolen, containerName))
		}
	}

	return result
}

// thiefCaught is what follows when m catches actor stealing in room: the
// thief is revealed, a sleeping m wakes, the theft is recorded as a crime
// against m's factions (reputation, bounty, witnesses' knowledge), and m
// attacks. Shared by stealFromMob and taking a household's bauble
// (household_bauble.go); the caller sends its own "caught" messages first.
func thiefCaught(actor Actor, m *mobs.Mob, room *rooms.Room) {
	theftReported(actor, m, room)

	// A victim that cannot be fought (a non-combatant shopkeeper, a
	// player-attack-immune NPC) does not attack; it has already raised the
	// crime above. stealFromMob never reaches here with one (it refuses to
	// steal from them), so for `steal` this changes nothing.
	if !m.IsNonCombatant() && !m.PlayerAttackImmune {
		m.Command(fmt.Sprintf(`attack @%d`, actor.GetUserId()))
	}
}

// theftReported is thiefCaught without the attack: the thief revealed, a
// sleeping m woken, and the theft recorded against m's factions with its
// reputation, bounty and witnesses' knowledge. A guard who recognises stolen
// goods on a thief uses it alone (stolen_bauble.go): what follows is the
// justice tick's (warn, then arrest), not a brawl on the spot.
func theftReported(actor Actor, m *mobs.Mob, room *rooms.Room) {
	// Harmless if it fails (already revealed); combat_fire.go does the same.
	_ = actor.GetCharacter().Awareness.TransitionToRevealing(state.TransitionReason{
		Trigger: awareness.TriggerSkullduggeryFailed,
	})

	// Chunk 3.3: failed theft wakes a sleeping victim.
	if m.Character.HasConditionFlag(conditions.Sleeping) {
		m.Character.CancelConditionsWithFlag(conditions.Sleeping)
		mobs.OnSleeperWoken(&m.Character)
	}

	// chunk 1.3: record theft crime on faction-aligned victim.
	if factionIds := factions.FactionsForMob(m); len(factionIds) > 0 {
		// All witnesses including the victim (excludeInstanceId=0).
		witnesses := crimes.WitnessesInRoom(factionIds, room, 0)
		perp := crimes.IdentifiedPerp(actor.GetUserId(), witnesses)
		// External witnesses (excluding victim) for HadExternalWitness.
		externalWitnesses := crimes.WitnessesInRoom(factionIds, room, m.InstanceId)
		// HadExternalWitness asks whether the theft was identified by
		// someone other than the victim, not merely noticed, so it reads
		// Identifying.
		hadExternal := len(externalWitnesses.Identifying) > 0
		delta := int(configs.GetBalanceConfig().CrimeRepDeltaTheft)
		for _, fid := range factionIds {
			crimeIds := crimes.Record([]string{fid}, crimes.KindTheft, perp,
				m, m.InstanceId, room.RoomId, m.Character.Zone, hadExternal)
			if perp.Type == crimes.PerpPlayer {
				factions.BumpRep(fid, actor.GetUserId(), delta)
				justice.MaybeDeclareBounty(fid, actor.GetUserId(), crimes.KindTheft)
				// Knowledge: each witness records the player as the perp of
				// these crimes. Range Identifying only. perp is computed
				// once for the whole room, so a single clear-sighted
				// witness makes perp.Type PerpPlayer for everyone present;
				// writing this player-subject knowledge for a shapes-only
				// witness would record that mob knowing exactly who it was
				// when all it saw was a figure.
				subject := knowledge.PlayerSubject(actor.GetUserId())
				for _, witnessInstId := range witnesses.Identifying {
					w := mobs.GetInstance(witnessInstId)
					if w == nil {
						continue
					}
					for _, crimeId := range crimeIds {
						knowledge.RecordCrimeWitnessed(int(w.MobId), subject, crimeId)
					}
					knowledge.RecordMet(int(w.MobId), subject, room.RoomId,
						knowledge.SourceWitnessed)
				}
			}
		}
	}

}

// stealObserverPass is the theft observer contest: the thief's
// attackerScore against the best-placed observer in the room (players
// outside the thief's party, and mobs), each scored stealVictimScore. With
// no observer the thief is unseen without a roll. spotter is that observer's
// name, and spotterMob the mob when it was one (nil for a player).
//
// Shared by stealing from a room container and taking a household's bauble
// (stealHouseholdBauble): both are lifting something from a place while
// whoever is there might notice.
func stealObserverPass(actor Actor, room *rooms.Room, attackerScore float64) (success bool, spotterName string, spotterMob *mobs.Mob) {
	// Find the best observer (players + mobs, excluding party). U6b Task 15:
	// this is the FOURTH steal contest, and it was scored on raw
	// highest-observer Perception -- the same x0-skill defender class as the
	// victim contests above. Observers now score stealVictimScore
	// (Perception + skullduggery x SkillWeight): spotting a theft in
	// progress is the same counter-craft as noticing one worked on you.
	partySet := map[int]bool{}
	if uid := actor.GetUserId(); uid > 0 {
		partySet[uid] = true
		if party := parties.Get(uid); party != nil {
			for _, memberId := range party.GetMembers() {
				partySet[memberId] = true
			}
		}
	}
	selfMobId := actor.GetMobInstanceId()

	highestObserverScore := 0.0
	hasObserver := false

	for _, observerId := range room.GetPlayers() {
		if partySet[observerId] {
			continue
		}
		observer := users.GetByUserId(observerId)
		if observer == nil {
			continue
		}
		obsScore := stealVictimScore(observer.Character, combat.SightRoom(room))
		if obsScore > highestObserverScore {
			highestObserverScore = obsScore
			spotterName = observer.Character.Name
			spotterMob = nil
			hasObserver = true
		}
	}

	for _, mobInstanceId := range room.GetMobs() {
		if mobInstanceId == selfMobId {
			continue
		}
		m := mobs.GetInstance(mobInstanceId)
		if m == nil {
			continue
		}
		obsScore := stealVictimScore(&m.Character, combat.SightRoom(room))
		if obsScore > highestObserverScore {
			highestObserverScore = obsScore
			spotterName = m.Character.Name
			spotterMob = m
			hasObserver = true
		}
	}

	success = true
	if hasObserver {
		success = combat.RunContest(attackerScore, []contest.Entry{{Score: highestObserverScore}}).Success
	}
	return success, spotterName, spotterMob
}

// stealHouseholdBauble takes a bauble that belongs to this room's household
// (`steal doll`): the steal checks throughout. The cooldown, the combat
// gates and the thief's score (Dexterity + skullduggery, plus the hidden
// bonus when sneaking) are Steal's; skullduggery rank 2 is required, as for
// every theft; and the contest is the container theft's observer pass.
//
// Caught by one of the household (a resident, isResident) is caught
// stealing from them: thiefCaught, the crime against their factions and
// their attack, exactly as stealing from a mob. Spotted by anyone else, the
// thief is revealed, as in a container theft. Either way the bauble stays.
// Taken unseen, it is marked stolen in the catalog.
func stealHouseholdBauble(actor Actor, itm items.Item, attackerScore float64, rank int) StealResult {
	room := actor.GetRoom()
	char := actor.GetCharacter()
	name := itm.DisplayName()

	if !itm.BaubleBelongsTo(room.RoomId) {
		actor.SendText(messaging.CategorySystem, "You don't see that here.")
		return StealResult{Reason: "not found"}
	}
	if rank < 2 {
		actor.SendText(messaging.CategorySystem, "You aren't advanced enough at skullduggery for that.")
		return StealResult{Reason: "not advanced enough"}
	}
	// Too heavy to lift is not a theft.
	if char.GetCarriedWeight()+itm.GetSpec().GetWeight() > char.CarryCapacity()*2.0 {
		actor.SendText(messaging.CategorySystem,
			fmt.Sprintf(`You can't carry the <ansi fg="itemname">%s</ansi> - you're already overloaded!`, name))
		return StealResult{Reason: "overloaded"}
	}

	if actor.IsPlayer() {
		if u := users.GetByUserId(actor.GetUserId()); u != nil {
			bridge := questengine.NewGameBridge(u, room.RoomId)
			questengine.GetEngine().Notify("command", questengine.EventDetails{
				UserId:  actor.GetUserId(),
				RoomId:  room.RoomId,
				Command: "steal",
			}, bridge, bridge)
		}
	}

	success, spotterName, spotterMob := stealObserverPass(actor, room, attackerScore)
	actor.AwardResolved(success, char.CandidateFor(string(skills.Skullduggery)))

	if !success {
		actor.SendText(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi> spots you reaching for the <ansi fg="itemname">%s</ansi>!`,
			spotterName, name))
		room.SendTextVisual(messaging.CategoryMobEmote,
			fmt.Sprintf(`<ansi fg="username">%s</ansi> is caught trying to pocket the <ansi fg="itemname">%s</ansi>!`,
				actor.GetName(), name),
			actor.GetUserId(),
		)
		if spotterMob != nil && householdMember(spotterMob, room) {
			householdCaught(actor, spotterMob, room)
		} else {
			_ = actor.GetCharacter().Awareness.TransitionToRevealing(state.TransitionReason{
				Trigger: awareness.TriggerSkullduggeryFailed,
			})
		}
		return StealResult{Detected: true, DefenderName: spotterName, Reason: "detected"}
	}

	if !char.StoreItem(itm) {
		actor.SendText(messaging.CategorySystem,
			fmt.Sprintf(`You can't carry the <ansi fg="itemname">%s</ansi> - you're already overloaded!`, name))
		return StealResult{Reason: "overloaded"}
	}
	room.RemoveItem(itm, false)
	if actor.IsPlayer() {
		events.AddToQueue(events.ItemOwnership{UserId: actor.GetUserId(), Item: itm, Gained: true})
	} else {
		events.AddToQueue(events.ItemOwnership{MobInstanceId: actor.GetMobInstanceId(), Item: itm, Gained: true})
	}

	theft := baubles.Theft{ByUserId: actor.GetUserId(), RoomId: room.RoomId, Zone: room.Zone}
	if resident, ok := HouseholdResident(room); ok {
		theft.FromMob = int(resident.MobId)
		theft.FromName = resident.Character.Name
		if fids := factions.FactionsForMob(resident); len(fids) > 0 {
			theft.Faction = fids[0]
		}
	}
	baubles.MarkStolen(itm.Bauble, theft, time.Now())

	actor.SendText(messaging.CategoryLoot, fmt.Sprintf(
		`You quietly slip the <ansi fg="itemname">%s</ansi> into your pack.`, name))
	return StealResult{Succeeded: true, StoleItemId: itm.ItemId, StoleItemName: name}
}
