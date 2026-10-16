package usercommands

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"

	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// adminConditionTarget is what setcondition applies a condition to: a player
// record or a mob, both of which queue the narrating events.Condition.
type adminConditionTarget interface {
	AddCondition(conditionId int, source string)
	AddConditionMagnitude(conditionId int, triggers int, magnitude float64, source string)
}

// applyAdminCondition applies spec to target. A condition reading one of
// conditions.ScaledKinds from its magnitude would land at magnitude 0 through
// a plain add (a Heat Sight with reach 0), so it is applied at a new
// character's spell value for its kind instead, capped like the spell, with
// its authored trigger count (triggers 0 means the spec's own). That keeps
// `setcondition` useful for playtesting the vision and light conditions
// (lighting plan 5c).
func applyAdminCondition(target adminConditionTarget, spec *conditions.ConditionSpec) {
	if kind, ok := spec.ScaledKind(); ok {
		magnitude := conditions.SpellScaledMagnitude(kind, conditions.NewCharacterSpellStat, conditions.NewCharacterSpellSkill)
		target.AddConditionMagnitude(spec.ConditionId, 0, magnitude, `admin`)
		return
	}
	target.AddCondition(spec.ConditionId, `admin`)
}

/*
* Role Permissions:
* setcondition 				(All)
 */
func SetCondition(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	// args should look like one of the following:
	// target conditionId - put condition on target if in the room
	// conditionId - put condition on self
	// search searchTerm - search for condition by name, display results
	args := util.SplitButRespectQuotes(rest)

	if len(args) > 0 {

		if (len(args) >= 2 && args[0] == "search") || (len(args) == 1 && args[0] == "list") {

			var foundConditionIds []int

			if args[0] == "list" {
				foundConditionIds = conditions.GetAllConditionIds()
			} else {
				foundConditionIds = conditions.SearchConditions(args[1])
			}

			sort.Ints(foundConditionIds)

			headers := []string{"Id", "Description", "Flags"}
			rows := [][]string{}

			if len(foundConditionIds) == 0 {
				rows = append(rows, []string{"No Matches", "No Matches", "No Matches"})
			} else {
				for _, conditionId := range foundConditionIds {
					if conditionSpec := conditions.GetConditionSpec(conditionId); conditionSpec != nil {
						flags := []string{}
						for _, flag := range conditionSpec.Flags {
							flags = append(flags, string(flag))
						}
						rows = append(rows, []string{strconv.Itoa(conditionSpec.ConditionId), conditionSpec.Name, strings.Join(flags, ", ")})
						rows = append(rows, []string{``, `-` + conditionSpec.Description, ``})
					}
				}
			}

			searchResultsTable := templates.GetTable("Search Results", headers, rows)
			tplTxt, _ := templates.Process("tables/generic", searchResultsTable, user.UserId, user.UserId)
			user.SendText(messaging.CategoryAdmin, tplTxt)
			return true, nil
		} else {

			targetUserId := 0
			targetMobInstanceId := 0
			conditionId := 0

			if len(args) >= 2 {

				room := rooms.LoadRoom(user.Character.RoomId)
				if room == nil {
					return false, fmt.Errorf(`room %d not found`, user.Character.RoomId)
				}

				if target, err := actions.ResolveTargetActor(room, args[0]); err == nil {
					if target.IsPlayer() {
						targetUserId = target.(*actions.UserActor).User.UserId
					} else {
						targetMobInstanceId = target.(*actions.MobActor).Mob.InstanceId
					}
				}
				// (on err, both IDs stay 0; downstream `if targetUserId > 0` /
				// `if targetMobInstanceId > 0` branches won't fire)

				conditionId, _ = strconv.Atoi(args[1])
				if conditionId == 0 {
					// Grab the first match
					foundConditionIds := conditions.SearchConditions(args[1])
					if len(foundConditionIds) > 0 {
						conditionId = foundConditionIds[0]
					}
				}

			} else if len(args) == 1 {
				targetUserId = user.UserId
				conditionId, _ = strconv.Atoi(args[0])
				if conditionId == 0 {
					// Grab the first match
					foundConditionIds := conditions.SearchConditions(args[0])
					if len(foundConditionIds) > 0 {
						conditionId = foundConditionIds[0]
					}
				}
			}

			if conditionId == 0 {
				user.SendText(messaging.CategoryAdmin, "conditionId must be an integer > 0.")
				return true, nil

			}

			if targetUserId > 0 {
				// get the user
				if targetUser := users.GetByUserId(targetUserId); targetUser != nil {
					// Get the condition
					if conditionSpec := conditions.GetConditionSpec(conditionId); conditionSpec != nil {
						// A stacking record can only be added through
						// AddConditionMagnitude, which supplies the rounds and
						// amount a stack needs; the queued add this command
						// sends carries neither, and Conditions.AddCondition now
						// refuses it. Catch that here instead of telling the
						// admin it applied when nothing landed.
						if conditionSpec.IsStacking() {
							user.SendText(messaging.CategoryAdmin, fmt.Sprintf("Condition %d (%s) stacks and can only be applied by whatever move or proc grants it, not this command.", conditionId, conditionSpec.Name))
						} else {
							applyAdminCondition(targetUser, conditionSpec)
							user.SendText(messaging.CategoryAdmin, fmt.Sprintf("Condition %d (%s) applied to %s.", conditionId, conditionSpec.Name, targetUser.Character.Name))
						}

					} else {
						user.SendText(messaging.CategoryAdmin, fmt.Sprintf("Condition %d not found.", conditionId))
					}

					return true, nil
				}
			}

			if targetMobInstanceId > 0 {
				// get the user
				if targetMob := mobs.GetInstance(targetMobInstanceId); targetMob != nil {
					// Get the condition
					if conditionSpec := conditions.GetConditionSpec(conditionId); conditionSpec != nil {
						// See the matching comment in the player branch above.
						if conditionSpec.IsStacking() {
							user.SendText(messaging.CategoryAdmin, fmt.Sprintf("Condition %d (%s) stacks and can only be applied by whatever move or proc grants it, not this command.", conditionSpec.ConditionId, conditionSpec.Name))
						} else {
							applyAdminCondition(targetMob, conditionSpec)
							user.SendText(messaging.CategoryAdmin, fmt.Sprintf("Condition %d (%s) applied to %s.", conditionSpec.ConditionId, conditionSpec.Name, targetMob.Character.Name))
						}

					} else {
						user.SendText(messaging.CategoryAdmin, fmt.Sprintf("Condition %d not found.", conditionId))
					}

					return true, nil
				}
			}

		}
	}

	user.SendText(messaging.CategoryAdmin, "target not found.")

	// send some sort of help info?
	infoOutput, _ := templates.Process("admincommands/help/command.setcondition", nil, user.UserId, user.UserId)
	user.SendText(messaging.CategoryUsage, infoOutput)

	return true, nil
}
