package usercommands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

/*
* Role Permissions:
* teleport 				(All)
* teleport.direction	(Teleport through walls in a direction)
* teleport.playername	(Teleport to a player name)
* teleport.roomid		(Teleport to a roomId)
 */
func Teleport(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	if len(rest) == 0 {
		// send some sort of help info?
		infoOutput, _ := templates.Process("admincommands/help/command.teleport", nil, user.UserId)
		user.SendText(messaging.CategorySystem, infoOutput)

		return true, nil
	}

	targetUser := user

	args := util.SplitButRespectQuotes(rest)
	if len(args) > 1 {

		if searchUser := users.GetByCharacterName(args[0]); searchUser != nil {
			rest = strings.Join(args[1:], ` `)
			targetUser = searchUser
		}
	}

	gotoRoomId, numError := strconv.Atoi(rest)
	// If not a number, check if it's a direction
	if numError != nil {

		if mapper.IsCompassDirection(rest) {

			if !user.HasRolePermission(`teleport.direction`) {
				user.SendText(messaging.CategorySystem, `you do not have <ansi fg="command">teleport.direction</ansi> permission`)
				return true, nil
			}

			zMapper := mapper.GetMapper(room.RoomId)
			if zMapper == nil {
				err := fmt.Errorf("Could not find mapper for zone: %s", room.Zone)
				mudlog.Error("Map", "error", err)
				user.SendText(messaging.CategorySystem, `No map found (or an error occured)"`)
				return true, err
			}

			gotoRoomId, _ = zMapper.FindAdjacentRoom(user.Character.RoomId, rest)

		} else {

			// Finally, try a player name
			if locateUser := users.GetByCharacterName(rest); locateUser != nil {

				if !user.HasRolePermission(`teleport.playername`) {
					user.SendText(messaging.CategorySystem, `you do not have <ansi fg="command">teleport.direction</ansi> permission`)
					return true, nil
				}

				gotoRoomId = locateUser.Character.RoomId
			}

		}

	} else {
		if !user.HasRolePermission(`teleport.roomid`) {
			user.SendText(messaging.CategorySystem, `you do not have <ansi fg="command">teleport.direction</ansi> permission`)
			return true, nil
		}
	}

	if gotoRoomId != 0 || rest == `0` {

		// The room the target leaves, captured before the move. The party
		// follows from HERE, not from the admin's room: `teleport Bob 500`
		// used to move the admin's roommates and their charmed mobs (#428
		// review).
		fromRoom := rooms.LoadRoom(targetUser.Character.RoomId)

		if err := rooms.MoveToRoom(targetUser.UserId, gotoRoomId); err != nil {
			user.SendText(messaging.CategorySystem, err.Error())

		} else {

			user.SendText(messaging.CategorySystem, fmt.Sprintf("Moved to room %d.", gotoRoomId))

			// The arrival is seen: the destination room's sight path, so a
			// viewer who makes out shapes only reads a figure (#428).
			gotoRoom := rooms.LoadRoom(gotoRoomId)
			gotoRoom.SendTextVisualHidingNames(messaging.CategorySystem,
				fmt.Sprintf(`<ansi fg="username">%s</ansi> appears in a flash of light!`, targetUser.Character.Name),
				[]string{targetUser.Character.Name},
				targetUser.UserId,
			)

			if party := parties.Get(targetUser.UserId); party != nil {

				// Party leaders can move the whole party.
				if party.LeaderUserId == targetUser.UserId && fromRoom != nil {

					newRoom := rooms.LoadRoom(gotoRoomId)
					for _, uid := range fromRoom.GetPlayers() {
						if party.IsMember(uid) {

							partyUser := users.GetByUserId(uid)
							if partyUser == nil {
								continue
							}

							if partyUser.Character.RoomId != fromRoom.RoomId {
								continue
							}

							rooms.MoveToRoom(partyUser.UserId, gotoRoomId)
							partyUser.SendText(messaging.CategorySystem, fmt.Sprintf("Moved to room %d.", gotoRoomId))
							gotoRoom.SendTextVisualHidingNames(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> appears in a flash of light!`, partyUser.Character.Name), []string{partyUser.Character.Name}, partyUser.UserId)

							Look(``, partyUser, gotoRoom, flags)

							for _, mInstanceId := range fromRoom.GetMobs(rooms.FindCharmed) {
								if mob := mobs.GetInstance(mInstanceId); mob != nil {
									if mob.Character.IsCharmed(partyUser.UserId) {
										fromRoom.RemoveMob(mob.InstanceId)
										newRoom.AddMob(mob.InstanceId)
									}
								}
							}
						}
					}

				}

			}

			Look(``, targetUser, gotoRoom, flags)

		}
	} else {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`Invalid teleport command: <ansi fg="command">%s</ansi> (No RoomId, direction, or character name match)`, rest))
	}

	return true, nil
}
