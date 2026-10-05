package actions

import (
	"fmt"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/companionai"
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

	// One pickpocket at a time: a thief still in one's pause starts no other
	// (the cooldown normally covers this, but it may be set to nothing).
	if actor.IsPlayer() && pocketPending(actor.GetUserId()) {
		actor.SendText(messaging.CategorySystem, "Your hand is still in someone's pocket.")
		return StealResult{Reason: "busy"}
	}

	// Only CHECK the cooldown here. It is armed by the attempt itself
	// (theftAttempt.score), so a refusal below spends nothing.
	if !char.CooldownReady(skullduggeryCooldownKey) {
		return StealResult{
			OnCooldown: true,
			Reason:     "still recovering",
		}
	}

	t := newTheftAttempt(char, room, cfg)

	// Dispatch to the appropriate path.
	if opts.TargetMobInstanceId > 0 {
		return stealFromMob(actor, opts.TargetMobInstanceId, t)
	}

	if opts.TargetUserId > 0 {
		return stealFromPlayer(actor, opts.TargetUserId, t)
	}

	if opts.HouseholdItem.ItemId > 0 {
		return stealHouseholdBauble(actor, opts.HouseholdItem, t)
	}

	// Container path.
	return stealFromContainer(actor, opts.ContainerNoun, t)
}

// skullduggeryCooldownKey is the one cooldown steal and plant share.
var skullduggeryCooldownKey = skills.Skullduggery.String(`steal`)

// theftAttempt is a thief's (or planter's) side of a steal or plant contest,
// carried from Steal and Plant down to the path that resolves the target.
//
// It exists to put the skullduggery cooldown in one place. Steal and Plant
// only check the cooldown; each path then runs its own refusals (target
// gone, a companion, an immune mob, rank below 2, an empty container) and
// only then asks for the score to roll with. score() is the only way to
// read it, and it arms the cooldown as it hands it over, so no refusal can
// spend the cooldown and no contest can run without spending it.
type theftAttempt struct {
	char   *characters.Character
	rank   int
	points float64
	period string
}

// newTheftAttempt computes the attacker's score once, for every theft and
// plant contest. U6b Task 15: linear rank x SkillWeight, the same shape as
// every other contest in the game; the old sqrt-curve x25 regime and the
// steal-specific knob it multiplied by are both gone. The sight ramp (plan
// 5b) is applied here too, once: the thief needs to see, and no path
// applies SightMult again.
func newTheftAttempt(char *characters.Character, room *rooms.Room, cfg configs.Balance) theftAttempt {
	rank := char.GetSkillLevel(skills.Skullduggery)
	points := float64(char.Stats.Dexterity.ValueAdj) +
		float64(rank)*float64(cfg.SkillWeight)
	if char.IsHidden() {
		points += float64(cfg.StealHiddenBonus)
	}
	points *= messaging.SightMult(char, room)
	return theftAttempt{
		char:   char,
		rank:   rank,
		points: points,
		period: fmt.Sprintf(`%d real seconds`, int(cfg.StealCooldown)),
	}
}

// score arms the skullduggery cooldown and returns the attacker's score.
// Call it once per attempt, immediately before the contest it feeds, and
// after every refusal.
func (t theftAttempt) score() float64 {
	t.char.TryCooldown(skullduggeryCooldownKey, t.period)
	return t.points
}

// stealFromMob handles the creature steal path. t.score() carries the
// thief's sight ramp already; do not apply SightMult again.
func stealFromMob(actor Actor, mobInstanceId int, t theftAttempt) StealResult {

	m := mobs.GetInstance(mobInstanceId)
	if m == nil {
		actor.SendText(messaging.CategorySystem, "They seem to have vanished.")
		return StealResult{Reason: "target not found"}
	}

	// Any companion is off-limits to theft, the thief's own included: a
	// charmed one (IsCharmed, the predicate mobs.CheckPlayerHarm refuses
	// first) or one bonded to the AI companion, which need not be charmed.
	// Its pocket is its owner's (owner ruling 2026-09-29). This holds for a
	// mob thief as well, as the two protections below do.
	if m.Character.IsCharmed() || companionai.IsBondedCompanion(m.InstanceId) {
		actor.SendText(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi> is someone's companion. You can't steal from them.`,
			m.Character.Name))
		return StealResult{
			DefenderName: m.Character.Name,
			Reason:       "companion",
		}
	}

	// The rest of mobs.CheckPlayerHarm's policy.
	if block := mobs.CheckPlayerHarm(m); block.Blocked() {
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
	if t.rank < 2 {
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
	success := combat.RunContest(t.score(), []contest.Entry{{Score: defenderScore}}).Success
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
				fmt.Sprintf(`<ansi fg="itemname">%s</ansi>`, itemStolen.DisplayNameFor(actor.GetUserId())))
		}

		// A pickpocketed bauble, already out of the mark's pocket (or made
		// for it; the caller queued the mark's loss when it had one). Too
		// heavy to carry after all, it falls at the thief's feet.
		for _, b := range extra {
			if !actor.GetCharacter().StoreItem(b) {
				b.LeaveBaubleAt(``, 0, baubleNow())
				actor.GetRoom().AddItem(b, false)
				stolenStuff = append(stolenStuff,
					fmt.Sprintf(`<ansi fg="itemname">%s</ansi> (too much to carry: it falls at your feet)`, b.DisplayNameFor(actor.GetUserId())))
				continue
			}
			if actor.IsPlayer() {
				events.AddToQueue(events.ItemOwnership{UserId: actor.GetUserId(), Item: b, Gained: true})
			}
			stolenStuff = append(stolenStuff,
				fmt.Sprintf(`<ansi fg="itemname">%s</ansi>`, b.DisplayNameFor(actor.GetUserId())))
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
// notices. t.score() carries the thief's sight ramp already; do not apply
// SightMult again.
func stealFromPlayer(actor Actor, targetUserId int, t theftAttempt) StealResult {

	targetUser := users.GetByUserId(targetUserId)
	if targetUser == nil {
		actor.SendText(messaging.CategorySystem, "They seem to have vanished.")
		return StealResult{Reason: "target not found"}
	}

	// Skill rank 2 required.
	if t.rank < 2 {
		actor.SendText(messaging.CategorySystem, "You aren't advanced enough at skullduggery for that.")
		return StealResult{
			DefenderName: targetUser.Character.Name,
			Reason:       "not advanced enough",
		}
	}

	defenderScore := stealVictimScore(targetUser.Character, combat.SightRoom(actor.GetRoom()))
	success := combat.RunContest(t.score(), []contest.Entry{{Score: defenderScore}}).Success
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

// stealFromContainer handles the room-container steal path. t.score()
// carries the thief's sight ramp already; do not apply SightMult again.
func stealFromContainer(actor Actor, containerName string, t theftAttempt) StealResult {

	room := actor.GetRoom()
	container, ok := room.Containers[containerName]
	if !ok {
		actor.SendText(messaging.CategorySystem, "You don't see that here.")
		return StealResult{Reason: "not found"}
	}
	if container.IsSealedShut() {
		actor.SendText(messaging.CategorySystem, fmt.Sprintf(`The <ansi fg="container">%s</ansi> is locked, and it opens for its owner and nobody else.`, containerName))
		return StealResult{Reason: "sealed"}
	}

	// Skill rank 2 required for the actual steal mechanic. Mirrors the
	// gate in stealFromMob; checked AFTER target validation so the rebuff
	// order is consistent.
	if t.rank < 2 {
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

	success, spotterName, _ := stealObserverPass(actor, room, t.score())
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

	markAttacksThief(actor, m)
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

	theftCrime(actor.GetUserId(), m, room, false, messaging.SightNone)
}

// markAttacksThief is a mark that caught actor stealing turning on them.
// A victim that cannot be fought (a non-combatant shopkeeper, a
// player-attack-immune NPC) does not attack; its caller has already raised
// the crime. stealFromMob never reaches here with one (it refuses to steal
// from them), so for `steal` this changes nothing. Nor does it attack a
// thief who is dead or downed (health below 1, rooms.FindDowned's test),
// which a pickpocket's reveal can find beside the mark.
func markAttacksThief(actor Actor, m *mobs.Mob) {
	if thief := actor.GetCharacter(); !thief.IsAlive() || thief.Health < 1 {
		return
	}
	if !m.IsNonCombatant() && !m.PlayerAttackImmune {
		m.Command(fmt.Sprintf(`attack @%d`, actor.GetUserId()))
	}
}

// theftWitnesses is who witnessed userId's theft from m in room, and
// whether anyone but m identified the thief. In the act: every faction mob
// in room that saw it, m included, by its sight (crimes.WitnessesInRoom).
// Away (a pickpocket's failed roll revealed after the thief had gone; slice
// H review finding b): m alone, which felt the hand, by markSaw, its sight
// of the theft room at the attempt (not the room's light now, which the
// thief may have carried off); bystanders saw nothing, so hadExternal is
// false. markSaw is ignored in the act.
func theftWitnesses(factionIds []string, m *mobs.Mob, room *rooms.Room, away bool, markSaw messaging.SightDecision) (crimes.Witnesses, bool) {
	if !away {
		// All witnesses including the victim (excludeInstanceId=0), and the
		// external ones (excluding the victim) for HadExternalWitness, which
		// asks whether the theft was identified by someone other than the
		// victim, not merely noticed, so it reads Identifying.
		witnesses := crimes.WitnessesInRoom(factionIds, room, 0)
		external := crimes.WitnessesInRoom(factionIds, room, m.InstanceId)
		return witnesses, len(external.Identifying) > 0
	}
	var w crimes.Witnesses
	switch markSaw {
	case messaging.SightFull:
		w.Identifying = []int{m.InstanceId}
	case messaging.SightShapes:
		w.ShapesOnly = []int{m.InstanceId}
	}
	return w, false
}

// theftCrime is the mark's side of a caught theft by userId in room (the
// room the theft happened in): a sleeping m wakes, and the theft is
// recorded as a crime against m's factions (reputation, bounty, witnesses'
// knowledge). Every part of it goes by user id, so it holds for a thief
// who has left or logged out. away is a pickpocket's failed roll revealed
// after the thief walked away (steal_pocket.go, pocketCrime): the mark is
// the only witness (theftWitnesses), judged by markSaw, its sight at the
// attempt. A mark that saw clearly still learns who robbed it
// (knowledge.RecordCrimeWitnessed makes a record with HasMet set); it only
// gets no last-seen room or round (RecordMet is skipped), since it did not
// see where the thief went. Bystanders learn nothing. thiefCaught runs it
// in the act.
func theftCrime(userId int, m *mobs.Mob, room *rooms.Room, away bool, markSaw messaging.SightDecision) {
	// Chunk 3.3: failed theft wakes a sleeping victim.
	if m.Character.HasConditionFlag(conditions.Sleeping) {
		m.Character.CancelConditionsWithFlag(conditions.Sleeping)
		mobs.OnSleeperWoken(&m.Character)
	}

	// chunk 1.3: record theft crime on faction-aligned victim.
	if factionIds := factions.FactionsForMob(m); len(factionIds) > 0 {
		witnesses, hadExternal := theftWitnesses(factionIds, m, room, away, markSaw)
		perp := crimes.IdentifiedPerp(userId, witnesses)
		delta := int(configs.GetBalanceConfig().CrimeRepDeltaTheft)
		for _, fid := range factionIds {
			crimeIds := crimes.Record([]string{fid}, crimes.KindTheft, perp,
				m, m.InstanceId, room.RoomId, m.Character.Zone, hadExternal)
			if perp.Type == crimes.PerpPlayer {
				factions.BumpRep(fid, userId, delta)
				justice.MaybeDeclareBounty(fid, userId, crimes.KindTheft)
				// Knowledge: each witness records the player as the perp of
				// these crimes. Range Identifying only. perp is computed
				// once for the whole room, so a single clear-sighted
				// witness makes perp.Type PerpPlayer for everyone present;
				// writing this player-subject knowledge for a shapes-only
				// witness would record that mob knowing exactly who it was
				// when all it saw was a figure.
				subject := knowledge.PlayerSubject(userId)
				for _, witnessInstId := range witnesses.Identifying {
					w := mobs.GetInstance(witnessInstId)
					if w == nil {
						continue
					}
					for _, crimeId := range crimeIds {
						knowledge.RecordCrimeWitnessed(int(w.MobId), subject, crimeId)
					}
					// Away, the mark knows who robbed it (the record above,
					// HasMet included) but not where the thief was last
					// seen: it felt the hand after they had gone.
					if !away {
						knowledge.RecordMet(int(w.MobId), subject, room.RoomId,
							knowledge.SourceWitnessed)
					}
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
// (`steal doll`): the steal checks throughout. The cooldown check, the
// combat gates and the thief's score (Dexterity + skullduggery, plus the
// hidden bonus when sneaking) are Steal's, and the cooldown is armed only
// when the contest runs (theftAttempt.score); skullduggery rank 2 is required, as for
// every theft; and the contest is the container theft's observer pass.
//
// Caught by one of the household (a resident, isResident) is caught
// stealing from them: thiefCaught, the crime against their factions and
// their attack, exactly as stealing from a mob. Spotted by anyone else, the
// thief is revealed, as in a container theft. Either way the bauble stays.
// Taken unseen, it is marked stolen in the catalog.
func stealHouseholdBauble(actor Actor, itm items.Item, t theftAttempt) StealResult {
	room := actor.GetRoom()
	char := actor.GetCharacter()
	name := itm.DisplayName()

	if !itm.BaubleBelongsTo(room.RoomId) {
		actor.SendText(messaging.CategorySystem, "You don't see that here.")
		return StealResult{Reason: "not found"}
	}
	if t.rank < 2 {
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

	success, spotterName, spotterMob := stealObserverPass(actor, room, t.score())
	actor.AwardResolved(success, char.CandidateFor(string(skills.Skullduggery)))

	if !success {
		actor.SendText(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi> spots you reaching for the <ansi fg="itemname">%s</ansi>!`,
			spotterName, name))
		room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
			fmt.Sprintf(`<ansi fg="username">%s</ansi> is caught trying to pocket the <ansi fg="itemname">%s</ansi>!`,
				actor.GetName(), name),
			[]string{actor.GetName()},
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
