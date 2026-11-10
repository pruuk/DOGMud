package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// conditionEntry is one row of the `conditions` template.
type conditionEntry struct {
	Name        string
	Description string
	RoundsLeft  int
	Permanent   bool
}

// Conditions lists everything currently affecting the player: one entry per
// held, unexpired, listed condition record, with its display name, description and
// a duration.
//
// There is one loop because there is one source. This command used to print
// the condition list and then a second list of combat conditions from an enum with
// its own tick, which is why warcry and rally needed a mirror flag to keep
// them out of the first list. The enum is gone; records are the conditions.
func Conditions(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	tplTxt, _ := templates.Process("character/conditions", conditionEntries(user.Character), user.UserId)
	user.SendText(messaging.CategoryScreen, tplTxt)
	return true, nil
}

// conditionEntries builds the rows. ConditionSpec.Listed decides what appears, the
// same predicate the Char.Conditions GMCP payload uses, so the text list and
// the web client show the same records. conditions.DisplayName appends a stacking
// record's live count.
func conditionEntries(c *characters.Character) []conditionEntry {
	entries := []conditionEntry{}
	for _, condition := range c.GetConditions() {
		spec := conditions.GetConditionSpec(condition.ConditionId)
		if spec == nil || !spec.Listed() {
			continue
		}
		roundsLeft, _ := conditions.GetDurations(condition, spec)
		entries = append(entries, conditionEntry{
			Name:        conditions.DisplayName(condition, spec),
			Description: spec.Description,
			RoundsLeft:  roundsLeft,
			Permanent:   condition.Permanent,
		})
	}
	return entries
}
