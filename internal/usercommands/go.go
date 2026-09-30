package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/conversationadapter"
	"github.com/GoMudEngine/GoMud/internal/conversations"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/relationships"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// unlockExit performs the shared tail of a successful exit unlock: it tells the
// actor, narrates to the room, plays the unlock sound, and clears the lock on
// both the caller's local exitInfo copy and the room's real exit.
//
// The three unlock paths in Go() (known lockpick sequence, key ring, and a
// loose key in the backpack) each repeated this exact sequence, differing only
// in the two messages — so a change to one, e.g. the sound or the SetExitLock
// call, silently diverged the others.
//
// exitInfo is a pointer because GetExitInfo returns a copy and Lock.SetUnlocked
// has a pointer receiver: the backpack path re-reads exitInfo.Lock.IsLocked()
// afterwards to decide whether to show the failure message, so the local copy
// must reflect the unlock.
func unlockExit(user *users.UserRecord, room *rooms.Room, exitName string, exitInfo *exit.RoomExit, playerMsg, roomMsg string) {
	user.SendText(messaging.CategorySystem, playerMsg)
	room.SendTextVisual(messaging.CategoryMobEmote, roomMsg, user.UserId)
	room.PlaySound(`change`, `other`)
	exitInfo.Lock.SetUnlocked()
	room.SetExitLock(exitName, false)
}

func Go(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	exitResult := actions.FindExit(room, rest)
	exitName, goRoomId := exitResult.ExitName, exitResult.RoomId

	// If no valid exit, check if it's a recognized cardinal direction (handled below
	// as "bumping into walls"). Otherwise return false so the dispatcher shows
	// "not recognized" instead of a confusing "can't do that in combat" message.
	isCardinal := false
	if exitName == `` {
		switch rest {
		case "north", "south", "east", "west", "up", "down",
			"northwest", "northeast", "southwest", "southeast":
			isCardinal = true
		}
		if !isCardinal {
			return false, nil
		}
	}

	if user.Character.IsInCombat() {
		// The death-recovery escape hatch that sat here was removed on
		// 2026-08-31. It let a player walk out of combat while standing in the
		// Shadow Realm, because stale aggro must never trap someone in a room
		// they were teleported into and could not fight their way out of.
		//
		// There is no Shadow Realm any more: death teleports to the player's
		// home, an ordinary room they can flee from normally. And with
		// DeathRecoveryRoom now 0 the hatch would have opened in room 0
		// instead, letting anyone standing there ignore combat entirely.
		user.SendText(messaging.CategorySystem, "You can't do that! You are in combat!")
		return true, nil
	}

	// Block movement during quest sequences (e.g., Awakening Rite ceremony)
	if lockMsg, ok := user.GetTempData(`questSequenceLock`).(string); ok && lockMsg != "" {
		user.SendText(messaging.CategorySystem, lockMsg)
		return true, nil
	}

	// Movement cancels crafting and salvaging — route through Activity
	// machine for both. Casting is NOT interrupted by movement (policy).
	if user.Character.Activity != nil && !user.Character.Activity.IsFree() {
		switch user.Character.Activity.State() {
		case activity.Crafting:
			_ = user.Character.Activity.TransitionToFree(state.TransitionReason{
				Trigger: activity.TriggerMovementInterrupt,
				Actor:   state.ActorRef{UserId: user.UserId},
			})
			user.SendText(messaging.CategorySystem, `<ansi fg="red">Your movement interrupts your crafting.</ansi>`)
		case activity.Salvaging:
			_ = user.Character.Activity.TransitionToFree(state.TransitionReason{
				Trigger: activity.TriggerMovementInterrupt,
				Actor:   state.ActorRef{UserId: user.UserId},
			})
			user.SendText(messaging.CategorySystem, `<ansi fg="red">Your movement interrupts your salvaging.</ansi>`)
		}
	}
	// If has a condition that prevents combat, skip the player
	if user.Character.HasConditionFlag(conditions.NoMovement) {
		user.SendText(messaging.CategorySystem, "You can't do that!")
		return true, nil
	}

	c := configs.GetTextFormatsConfig()

	// Check both the condition flag (set by event queue on next tick) and the
	// misc-data flag (set synchronously by sneak command). This handles
	// the case where the player sneaks then immediately moves before the
	// condition event processes.
	isSneaking := user.Character.IsHidden()
	if !isSneaking {
		if sneakFlag, ok := user.Character.GetMiscData(`sneaking`).(bool); ok && sneakFlag {
			isSneaking = true
		}
	}

	handled := false

	if exitName != `` {

		if user.Character.IsDisabled() {
			user.SendText(messaging.CategorySystem, "You can't do that — you're dead. Hold on; you'll be pulled back to safety.")
			return true, nil
		}

		destRoom := rooms.LoadRoom(goRoomId)
		if destRoom == nil {
			return false, fmt.Errorf(`room %d not found`, goRoomId)
		}

		originRoomId := user.Character.RoomId

		exitInfo, _ := room.GetExitInfo(exitName)

		if exitInfo.Lock.IsLocked() {

			lockId := fmt.Sprintf(`%d-%s`, room.RoomId, exitName)

			hasKey, hasSequence := user.Character.HasKey(lockId, int(room.Exits[exitName].Lock.Difficulty))

			lockpickItm := items.Item{}
			// Only look for a lockpick kit if they know the sequence
			if hasSequence {
				for _, itm := range user.Character.GetAllBackpackItems() {
					if itm.GetSpec().Type == items.Lockpicks {
						lockpickItm = itm
						break
					}
				}
			}

			if lockpickItm.ItemId > 0 && hasSequence {

				unlockExit(user, room, exitName, &exitInfo,
					`You know this lock well, you quickly pick it.`,
					fmt.Sprintf(`<ansi fg="username">%s</ansi> quickly picks the lock on the <ansi fg="exit">%s</ansi> exit.`, user.Character.Name, exitName))

			} else if hasKey {

				unlockExit(user, room, exitName, &exitInfo,
					fmt.Sprintf(`You use the key on your key ring to unlock the <ansi fg="exit">%s</ansi> exit.`, exitName),
					fmt.Sprintf(`<ansi fg="username">%s</ansi> uses a key to unlock the <ansi fg="exit">%s</ansi> exit.`, user.Character.Name, exitName))

			} else {

				// check for a key item on their person
				if backpackKeyItm, hasBackpackKey := user.Character.FindKeyInBackpack(lockId); hasBackpackKey {

					itmSpec := backpackKeyItm.GetSpec()

					// Key entries look like:
					// "key-<roomid>-<exitname>": "<itemid>"
					user.Character.SetKey(`key-`+lockId, fmt.Sprintf(`%d`, backpackKeyItm.ItemId))
					user.Character.RemoveItem(backpackKeyItm)

					events.AddToQueue(events.ItemOwnership{
						UserId: user.UserId,
						Item:   backpackKeyItm,
						Gained: false,
					})

					unlockExit(user, room, exitName, &exitInfo,
						fmt.Sprintf(`You use your <ansi fg="item">%s</ansi> to unlock the <ansi fg="exit">%s</ansi> exit, and add it to your key ring for the future.`, itmSpec.Name, exitName),
						fmt.Sprintf(`<ansi fg="username">%s</ansi> uses a key to unlock the <ansi fg="exit">%s</ansi> exit.`, user.Character.Name, exitName))
				}

				if exitInfo.Lock.IsLocked() {
					user.SendText(messaging.CategorySystem, `There's a lock preventing you from going that way. You'll need a <ansi fg="item">Key</ansi> or to <ansi fg="command">pick</ansi> the lock with <ansi fg="item">lockpicks</ansi>.`)
					// Send GMCP message
					if f, ok := GetExportedFunction(`SendGMCPEvent`); ok {
						if gmcpSendFunc, ok := f.(func(int, string, any)); ok { // make sure the func definition is `func(int, string, any)`
							gmcpSendFunc(user.UserId, `Room.WrongDir`, fmt.Sprintf(`"%s"`, exitName))
						}
					}

					return true, nil
				}
			}

		}

		if exitInfo.ExitMessage != `` && !flags.Has(events.CmdIsRequeue) {
			user.SendText(messaging.CategoryRoomDescription, exitInfo.ExitMessage)
			user.CommandFlagged(rest, flags|events.CmdIsRequeue|events.CmdBlockInputUntilComplete, 1)
			return true, nil
		}

		// Movement parity 4b: the step is paid here, after the lock and the
		// exit-message requeue, so a door that stays locked costs nothing and
		// a requeued step is charged once, not twice. The price and the charge
		// are actions.ChargeMove, shared with mobs. U5b-2: movement REFUSES
		// when unaffordable, leaving no debt behind.
		charge := actions.ChargeMove(actions.NewUserActorInRoom(user, room), destRoom)
		switch charge.Refusal {
		case actions.MoveRefuseEncumbered:
			user.SendText(messaging.CategorySystem, "You're too encumbered to move (<ansi fg=\"command\">help encumbrance</ansi>)!")
			return true, nil
		case actions.MoveRefuseTired:
			user.SendText(messaging.CategorySystem, "You're too tired to move (slow down)!")
			mudlog.Debug("No ActionPoints", "AP", user.Character.ActionPoints, "Needed", charge.ActionCost)
			return true, nil
		case actions.MoveRefuseExhausted:
			user.SendText(messaging.CategorySystem, "You're too exhausted to move! Rest and recover your stamina.")
			return true, nil
		}
		if charge.Winded {
			user.SendText(messaging.CategorySystem, "<ansi fg=\"yellow\">You're feeling winded. Consider resting to recover your stamina.</ansi>")
		}

		// Grab the exit in the target room that leads to this room (if any)
		enterFromExit := destRoom.FindExitTo(room.RoomId)

		if len(enterFromExit) < 1 {
			enterFromExit = "somewhere"
		} else {

			// Entering through the other side unlocks this side
			exitInfo := destRoom.Exits[enterFromExit]
			if exitInfo.Lock.IsLocked() {
				exitInfo.Lock.SetUnlocked()
				destRoom.SetExitLock(enterFromExit, false)
			}

			enterFromExit = fmt.Sprintf(`the <ansi fg="exit">%s</ansi>`, enterFromExit)
		}

		behaviortree.TryRoomBehavior(room.RoomId, behaviortree.EventContext{
			EventType: "room_exit",
			UserId:    user.UserId,
			RoomId:    room.RoomId,
			Direction: exitName,
		})

		if err := rooms.MoveToRoom(user.UserId, destRoom.RoomId); err != nil {
			user.SendText(messaging.CategorySystem, "Oops, couldn't move there!")
		} else {

			// Quest engine: room_enter notification. For an ephemeral room
			// (tutorial antechamber, dungeons) the trigger must match the
			// TEMPLATE id -- the ephemeral id is generated and unauthorable.
			// OriginalRoomId returns the room's own id for non-ephemeral rooms,
			// so this is a no-op there. The bridge keeps the REAL room id so
			// npc_say and other actions resolve mobs/exits in the instance the
			// player is actually in.
			matchRoom, _ := rooms.OriginalRoomId(destRoom.RoomId)
			bridge := questengine.NewGameBridge(user, destRoom.RoomId)
			questengine.GetEngine().Notify("room_enter", questengine.EventDetails{
				UserId: user.UserId,
				RoomId: matchRoom,
			}, bridge, bridge)

			// Record this room as visited for fog-of-war web map. Use the
			// TEMPLATE id (matchRoom) for ephemeral rooms -- otherwise the raw,
			// unauthorable instance id (e.g. 1000000000) is permanently baked
			// into the saved VisitedRooms set and leaks onto the Zone.Map
			// snapshot. OriginalRoomId returns the room's own id for
			// non-ephemeral rooms, so this is a no-op there.
			user.Character.MarkRoomVisited(destRoom.Zone, matchRoom)

			// U7 Task 10: a completed move rarely trains search. Inside the
			// MoveToRoom success branch on purpose: a refused or locked move
			// never reaches here. Shared with mobs (movement parity 4b).
			actions.TrainSearchOnMove(actions.NewUserActorInRoom(user, destRoom))

			// Tell the player they are moving
			if isSneaking {
				user.SendText(messaging.CategoryRoomExit,
					fmt.Sprintf(string(c.ExitRoomMessageWrapper),
						fmt.Sprintf(`You <ansi fg="black-bold">sneak</ansi> towards the <ansi fg="exit">%s</ansi> exit.`, exitName),
					))
			} else {
				user.SendText(messaging.CategoryRoomExit,
					fmt.Sprintf(string(c.ExitRoomMessageWrapper),
						fmt.Sprintf(`You head towards the <ansi fg="exit">%s</ansi> exit.`, exitName),
					))

				// Tell the old room they are leaving
				if user.Character.Pet.Exists() {

					room.SendTextVisualWithAudio(messaging.CategoryRoomExit,
						fmt.Sprintf(string(c.ExitRoomMessageWrapper),
							fmt.Sprintf(`<ansi fg="username">%s</ansi> and %s leave towards the <ansi fg="exit">%s</ansi> exit.`, user.Character.Name, user.Character.Pet.DisplayName(), exitName),
						),
						`You hear someone leave the room.`,
						user.UserId)

				} else {
					room.SendTextVisualWithAudio(messaging.CategoryRoomExit,
						fmt.Sprintf(string(c.ExitRoomMessageWrapper),
							fmt.Sprintf(`<ansi fg="username">%s</ansi> leaves towards the <ansi fg="exit">%s</ansi> exit.`, user.Character.Name, exitName),
						),
						`You hear someone leave the room.`,
						user.UserId)
				}

				// Tell everyone if the pet is following
				if user.Character.Pet.Exists() {

					user.SendText(messaging.CategorySystem, fmt.Sprintf(`%s follows you.`, user.Character.Pet.DisplayName()))

					// SendTextVisualWithAudio, not SendText: the named line is
					// pure sight and used to reach a player in a pitch dark
					// cave complete with a compass direction. The room EXIT
					// lines above were already sight-gated, so entry and exit
					// disagreed about the same event.
					destRoom.SendTextVisualWithAudio(messaging.CategoryRoomEntry,
						fmt.Sprintf(string(c.ExitRoomMessageWrapper),
							fmt.Sprintf(`<ansi fg="username">%s</ansi> and %s enters from <ansi fg="exit">%s</ansi>.`, user.Character.Name, user.Character.Pet.DisplayName(), exitName),
						),
						`You hear someone enter the room.`,
						user.UserId)

				} else {

					// Tell the new room they have arrived
					destRoom.SendTextVisualWithAudio(messaging.CategoryRoomEntry,
						fmt.Sprintf(string(c.EnterRoomMessageWrapper),
							fmt.Sprintf(`<ansi fg="username">%s</ansi> enters from <ansi fg="exit">%s</ansi>.`, user.Character.Name, enterFromExit),
						),
						`You hear someone enter the room.`,
						user.UserId)

				}

				destRoom.SendTextToExits(`You hear someone moving around.`, true, room.GetPlayers(rooms.FindAll)...)
			}

			if currentParty := parties.Get(user.UserId); currentParty != nil {

				if currentParty.IsLeader(user.UserId) {

					for _, partyMemberId := range currentParty.UserIds {
						if partyMemberId == user.UserId {
							continue
						}
						if partyUser := users.GetByUserId(partyMemberId); partyUser != nil {
							if partyUser.Character.RoomId == room.RoomId {
								partyUser.SendText(messaging.CategorySystem, `You follow the party leader.`)
								partyUser.Command(rest)
							}
						}
					}

				}
			}

			for _, instId := range room.GetMobs(rooms.FindCharmed) {
				mob := mobs.GetInstance(instId)
				if mob == nil {
					continue
				}
				// They only follow if they're in the same room as the player
				if mob.Character.RoomId != originRoomId {
					continue
				}
				if mob.Character.IsCharmed(user.UserId) { // Charmed mobs follow
					// Companions interrupt casting to follow owner — Activity
					// cascade (Activity_Cascades.go movement interrupt) handles
					// the machine transition; no direct field manipulation needed.
					mob.Command(rest)
				}
			}

			// Hidden detection on room entry, both directions: the sneaking
			// mover against the room, then the newcomer against the room's
			// hiders. Shared with mobs (movement parity 4b).
			isSneaking = actions.EntryDetection(actions.NewUserActorInRoom(user, destRoom), destRoom, isSneaking).StillSneaking

			if !isSneaking {
				// U10b-1 Task 19 DELETED the mob-follow roll that stood here.
				//
				// It was a bare util.Rand(100) against
				// 20 + Charisma + a dexterity delta, entirely off the contest core,
				// deciding whether an engaged mob chased a leaving player. The
				// arc's ruling is that MOB PURSUIT IS AUTHORED BEHAVIOUR, not a
				// roll: a mob pursues because its behaviour tree says to, and a
				// mob with no such behaviour does not pursue at all.
				//
				// ⚠️ CONSEQUENCE, and it is a real balance change rather than a
				// cleanup: a player who walks out of a fight is no longer
				// chased by default. Until pursuit behaviour is authored (the
				// behavior unification arc), fleeing by walking is strictly
				// safer than it was.
				//
				// Deliberately NOT deleted with it: this `if !isSneaking`
				// wrapper and the destination TryRoomBehavior below, which are
				// unrelated to the roll and are what actually fires room_enter.

				// Room behavior tree: fire room_enter for the destination room
				behaviortree.TryRoomBehavior(destRoom.RoomId, behaviortree.EventContext{
					EventType: "room_enter",
					UserId:    user.UserId,
					RoomId:    destRoom.RoomId,
					Direction: exitName,
				})

				// Behavior tree: notify mobs that a player entered
				if !isSneaking {
					for _, mobInstId := range destRoom.GetMobs(rooms.FindAll) {
						mob := mobs.GetInstance(mobInstId)
						if mob == nil || mob.Character.IsCharmed() {
							continue
						}
						behaviortree.TryMobBehavior(mobInstId, behaviortree.EventContext{
							EventType: "player_enter",
							UserId:    user.UserId,
							RoomId:    destRoom.RoomId,
						})
					}
				}

				//
				// When entering a room, mobs might be waiting to attack
				//
				// Declared here now rather than reused: the mob-follow loop that
				// used to declare mobInstanceIds was deleted by U10b-1 Task 19.
				mobInstanceIds := destRoom.GetMobs(rooms.FindAll)
				for _, mobInstanceId := range mobInstanceIds {
					mob := mobs.GetInstance(mobInstanceId)
					if mob == nil {
						continue
					}
					if mob.Character.IsInCombat() {
						continue
					}
					if mob.Character.IsCharmed() {
						continue
					}
					// Non-combatants never aggro on entry even if their
					// group has been flagged hostile by prior attacks.
					if mob.IsNonCombatant() {
						continue
					}

					if !mob.AutoAggro { // Is it automatically hostile?
						continue
					}

					// Hidden mobs attack silently — no "notices you" message.
					// They still trigger lookfortrouble for the surprise attack.
					if !mob.Character.IsHidden() {
						// Was an inline lit-or-nightvision check. Now the same
						// predicate the message pipeline itself uses, so this
						// site cannot drift from SendTextVisual and a blinded
						// player in a lit room stops reading names too.
						if messaging.CanSeeClearly(user.Character, destRoom) {
							user.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> notices you as you enter!`, mob.Character.Name))
						} else {
							user.SendText(messaging.CategorySystem, `<ansi fg="yellow">Something notices you in the darkness!</ansi>`)
						}
					}

					mob.Command(`lookfortrouble`, 4)

				}

			}

			// Chunk 3.3: a player arriving with a light source wakes any
			// sleepers in the destination room. False positives possible if
			// the room was already lit; acceptable for chunk 3.3 scope (most
			// NPC sleep rooms are dim/dark indoors).
			if user.Character.EmitsLight() {
				for _, otherUserId := range destRoom.GetPlayers() {
					if otherUserId == user.UserId {
						continue
					}
					if other := users.GetByUserId(otherUserId); other != nil &&
						other.Character.HasConditionFlag(conditions.Sleeping) {
						other.Character.CancelConditionsWithFlag(conditions.Sleeping)
						mobs.OnSleeperWoken(other.Character)
					}
				}
				for _, mobInstId := range destRoom.GetMobs() {
					if m := mobs.GetInstance(mobInstId); m != nil &&
						m.Character.HasConditionFlag(conditions.Sleeping) {
						m.Character.CancelConditionsWithFlag(conditions.Sleeping)
						mobs.OnSleeperWoken(&m.Character)
					}
				}
			}

			// 5a: NPC greetings. The first unoccupied NPC with an authored
			// greeting for its current mood welcomes the arriving player —
			// once per mob instance per player per boot, at most one greeting
			// per entry, and never for a hidden player: being hailed by name
			// would silently defeat stealth. IsHidden() is checked fresh here
			// rather than reusing move-time isSneaking, because a reveal
			// during the move (spotted by a hidden-mob check) should restore
			// the greeting. Runs before the conversation boost so the player
			// is welcomed before NPCs start talking amongst themselves.
			if !user.Character.IsHidden() {
				for _, greeterInstId := range destRoom.GetMobs() {
					gMob := mobs.GetInstance(greeterInstId)
					if gMob == nil {
						continue
					}
					if dialogue.HasGreeted(greeterInstId, user.UserId) {
						continue
					}
					if !conversations.IsFullyIdle(conversationadapter.AdaptMob(gMob)) {
						continue
					}
					df := dialogue.Load(int(gMob.MobId), gMob.Zone)
					if df == nil || len(df.Greetings) == 0 {
						continue
					}
					text, ok := dialogue.PickGreeting(df.Greetings, dialogue.GetMood(greeterInstId, df.DefaultMood))
					if !ok {
						continue
					}
					dialogue.MarkGreeted(greeterInstId, user.UserId)
					gMob.Command(`say ` + text)
					break // at most one greeting per entry (measured: 9 two-greeter rooms, none higher)
				}
			}

			// Chunk 3.6: player-arrival conversation boost. When the player
			// lands in a room with 2+ NPCs that are related and fully idle,
			// roll once at the boosted chance for an opening exchange so the
			// player is more likely to witness conversations rather than
			// always arriving between them.
			{
				cfg := configs.GetBalanceConfig()
				boostPct := int(cfg.ConversationPlayerArrivalBoostPct)
				if boostPct > 0 && util.Rand(100) < boostPct {
					if pairs := findRelateableEligiblePairsInRoom(destRoom); len(pairs) > 0 {
						p := pairs[util.Rand(len(pairs))]
						conversations.TryStartBetween(
							conversationadapter.AdaptMob(p.A),
							conversationadapter.AdaptMob(p.B),
						)
					}
				}
			}

			handled = true

			// Skip onEnter scripts when hidden — NPCs shouldn't react
			// to a player they can't see. Still show the room via Look.
			if isSneaking {
				Look(``, user, destRoom, events.CmdSecretly)
			} else {
				Look(``, user, destRoom, events.CmdSecretly)
			}

			room.PlaySound(`room-exit`, `movement`, user.UserId)
			destRoom.PlaySound(`room-enter`, `movement`, user.UserId)

		}

	}

	if !handled {

		if rest == "north" || rest == "south" || rest == "east" || rest == "west" || rest == "up" || rest == "down" || rest == "northwest" || rest == "northeast" || rest == "southwest" || rest == "southeast" {
			user.SendText(messaging.CategorySystem, "You're bumping into walls.")

			// Send GMCP message
			if f, ok := GetExportedFunction(`SendGMCPEvent`); ok {
				if gmcpSendFunc, ok := f.(func(int, string, any)); ok { // make sure the func definition is `func(int, string, any)`
					gmcpSendFunc(user.UserId, `Room.WrongDir`, fmt.Sprintf(`"%s"`, rest))
				}
			}

			if !user.Character.IsHidden() {

				room.SendTextVisual(messaging.CategoryMobEmote,
					fmt.Sprintf(string(c.ExitRoomMessageWrapper),
						fmt.Sprintf(`<ansi fg="username">%s</ansi> is bumping into walls.`, user.Character.Name),
					),
					user.UserId)
			}
			handled = true
		}

	}

	return handled, nil
}

// relateableMobPair holds an unordered pair of mobs that share a
// relationship edge and are both eligible to start a conversation.
type relateableMobPair struct {
	A *mobs.Mob
	B *mobs.Mob
}

// findRelateableEligiblePairsInRoom enumerates all unordered (a, b) mob
// pairs in the room where both mobs:
//   - have a relationship edge per chunk 1.6 (relationships.AreRelated)
//   - are not in combat and have no pending aggro
//   - are not sleeping
//   - have no in-flight path step
//   - are not already in a conversation (partner_id MiscData check)
//   - are not on conversation cooldown
func findRelateableEligiblePairsInRoom(room *rooms.Room) []relateableMobPair {
	if room == nil {
		return nil
	}
	mobIds := room.GetMobs()
	if len(mobIds) < 2 {
		return nil
	}

	mobList := make([]*mobs.Mob, 0, len(mobIds))
	for _, id := range mobIds {
		m := mobs.GetInstance(id)
		if m == nil {
			continue
		}
		if m.Character.IsInCombat() {
			continue
		}
		if m.Character.HasConditionFlag(conditions.Sleeping) {
			continue
		}
		if m.Path.Len() > 0 || m.Path.Current() != nil {
			continue
		}
		if pid, ok := m.Character.GetMiscData(conversations.MiscDataPartnerId).(int); ok && pid > 0 {
			continue
		}
		if until, ok := m.Character.GetMiscData(conversations.MiscDataCooldownUntilRound).(uint64); ok &&
			uint64(util.GetRoundCount()) < until {
			continue
		}
		mobList = append(mobList, m)
	}

	var out []relateableMobPair
	for i := 0; i < len(mobList); i++ {
		for j := i + 1; j < len(mobList); j++ {
			a, b := mobList[i], mobList[j]
			if relationships.AreRelated(int(a.MobId), int(b.MobId)) {
				out = append(out, relateableMobPair{A: a, B: b})
			}
		}
	}
	return out
}
