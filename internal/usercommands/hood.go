package usercommands

import (
	"fmt"
	"slices"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/lightnotice"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// hoodedLight returns the adjustable light records of the item in the user's
// light slot: the hooded lantern's (lighting plan 5a). When there are none it
// sends the refusal itself and reports ok false: an empty slot and a worn
// light with no hood are told apart.
func hoodedLight(user *users.UserRecord) (recs []*conditions.Condition, ok bool) {
	lightItem := &user.Character.Equipment.Light
	if lightItem.ItemId < 1 {
		user.SendText(messaging.CategorySystem, `You have no lantern with a hood.`)
		return nil, false
	}
	for _, id := range lightItem.GetSpec().WornConditionIds {
		spec := conditions.GetConditionSpec(id)
		if spec == nil || !spec.IsLightSource() || !slices.Contains(spec.Flags, conditions.Adjustable) {
			continue
		}
		recs = append(recs, user.Character.Conditions.GetConditions(id)...)
	}
	if len(recs) == 0 {
		user.SendText(messaging.CategorySystem,
			fmt.Sprintf(`Your <ansi fg="item">%s</ansi> has no hood.`, lightItem.DisplayName()))
		return nil, false
	}
	return recs, true
}

// Hood closes the hood of the lantern in the light slot: it stays lit and
// held, and sheds no light until unhooded or re-equipped.
func Hood(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	recs, ok := hoodedLight(user)
	if !ok {
		return true, nil
	}
	if recs[0].Hooded {
		user.SendText(messaging.CategorySystem, `Your lantern is already hooded.`)
		return true, nil
	}
	// Judged against the room just before the hood goes down (owner rule,
	// 2026-10-05; #220): the hood is the end of a light, and judged by the
	// room after it, the line would be silenced for everyone who was seeing
	// by the lantern, while a watcher who could not see even by it learns
	// nothing.
	var beforeHood rooms.VisualSnapshot
	if room != nil {
		beforeHood = room.VisualSnapshot()
	}
	for _, rec := range recs {
		rec.Hooded = true
	}
	user.SendText(messaging.CategorySystem, `You lower the hood over your lantern, and its light narrows to nothing.`)
	if room != nil {
		room.SendTextVisualToSnapshot(beforeHood, messaging.CategoryMobEmote,
			fmt.Sprintf(`<ansi fg="username">%s</ansi> lowers the hood of their lantern, and its glow goes dark.`, user.Character.Name),
			[]string{user.Character.Name}, user.UserId)
	}
	// The band notice rides this command's own output. Left to the next
	// command's pre-check, "darkness closes in" arrives after whatever the
	// player types next and reads backwards.
	lightnotice.Check(user, lightnotice.TriggerCommand)
	return true, nil
}

// Unhood opens the hood at full strength. The next room the bearer enters
// trims it back to their eyes.
func Unhood(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	recs, ok := hoodedLight(user)
	if !ok {
		return true, nil
	}
	if !recs[0].Hooded {
		user.SendText(messaging.CategorySystem, `Your lantern's hood is already open.`)
		return true, nil
	}
	for _, rec := range recs {
		rec.ResetLight()
	}
	user.SendText(messaging.CategorySystem, `You throw back the hood of your lantern, and light floods out around you.`)
	if room != nil {
		room.SendTextVisual(messaging.CategoryMobEmote,
			fmt.Sprintf(`<ansi fg="username">%s</ansi> throws back the hood of their lantern, and light floods out.`, user.Character.Name),
			user.UserId)
	}
	lightnotice.Check(user, lightnotice.TriggerCommand)
	return true, nil
}
