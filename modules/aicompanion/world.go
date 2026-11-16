package aicompanion

import (
	"fmt"
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/factions"
	"github.com/GoMudEngine/GoMud/internal/justice"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/worldevents"
)

// The world the two of them are walking through: what is wrong with either
// of them, what the people here think of her owner, whether the guards
// want him, what he is trying to do, and what is being said on the roads.
// All of it is read only, all of it is what a person standing there could
// know, and none of it lets her do anything a player cannot.

// conditionLines names what ails either of them, in the words the game
// uses. A condition is visible: it is in the look, and the sufferer knows
// their own.
func conditionLines(mob *mobs.Mob, owner *users.UserRecord, canSeeOwner bool) []string {
	var out []string
	if names := conditionNames(&mob.Character); len(names) > 0 {
		out = append(out, `You are `+strings.Join(names, `, `)+`.`)
	}
	if owner != nil && owner.Character != nil && canSeeOwner {
		if names := conditionNames(owner.Character); len(names) > 0 {
			out = append(out, owner.Character.Name+` is `+strings.Join(names, `, `)+`.`)
		}
	}
	return out
}

func conditionNames(ch *characters.Character) []string {
	var out []string
	for _, cond := range ch.GetConditions() {
		if cond == nil {
			continue
		}
		spec := conditions.GetConditionSpec(cond.ConditionId)
		if spec == nil || spec.Name == `` || cond.ConditionId == 0 {
			continue // condition 0 is the logout meditation, not an ailment
		}
		out = append(out, strings.ToLower(spec.Name))
	}
	sort.Strings(out)
	if len(out) > 6 {
		out = out[:6]
	}
	return out
}

// factionLines say how the people here regard her owner, and whether the
// guards among them want him. A scout reads a room this way before she
// walks into it.
func factionLines(room *rooms.Room, mob *mobs.Mob, owner *users.UserRecord) []string {
	if room == nil || owner == nil || owner.Character == nil || cannotSee(mob, room) {
		return nil
	}
	seen := map[string]bool{}
	var here []string
	for _, id := range room.GetMobs() {
		m := mobs.GetInstance(id)
		if m == nil || id == mob.InstanceId || !mob.Character.Perceives(&m.Character) {
			continue
		}
		for _, f := range factions.FactionsForMob(m) {
			if seen[f] {
				continue
			}
			seen[f] = true
			here = append(here, f)
		}
	}
	sort.Strings(here)

	var out []string
	for _, f := range here {
		def := factions.GetDefinition(f)
		name := f
		if def != nil && def.DisplayName != `` {
			name = def.DisplayName
		}
		out = append(out, fmt.Sprintf(`The %s here %s %s.`, name, factionWords(factions.GetRep(f, owner.UserId)), owner.Character.Name))
	}
	if sev := justice.Verdict(here, owner.UserId); sev > justice.SeverityNone {
		warning := `The guards here have a word out about ` + owner.Character.Name + `; it would be wiser not to linger.`
		if sev >= justice.SeverityAttack {
			warning = `The guards here will draw on ` + owner.Character.Name + ` on sight. Say so, now.`
		}
		out = append(out, warning)
	}
	return out
}

func factionWords(rep int) string {
	switch {
	case rep <= -50:
		return `would gladly see the back of`
	case rep <= -15:
		return `have no love for`
	case rep < 15:
		return `neither know nor care about`
	case rep < 50:
		return `think well enough of`
	}
	return `count as one of their own`
}

// questLines are what her owner is in the middle of: the quests he is on
// and the step he is at, from his own log. She cannot move any of it; she
// can remember where the last one sent him and say so.
func questLines(owner *users.UserRecord, max int) []string {
	if owner == nil || owner.Character == nil {
		return nil
	}
	progress := owner.Character.GetQuestProgress()
	ids := make([]int, 0, len(progress))
	for id := range progress {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	var out []string
	for _, id := range ids {
		step := progress[id]
		if step == `end` || step == `all+` {
			continue
		}
		q := quests.GetQuest(fmt.Sprintf(`%d-%s`, id, step))
		if q == nil || q.Secret {
			continue
		}
		var line strings.Builder
		line.WriteString(q.Name)
		for _, s := range q.Steps {
			if s.Id != step {
				continue
			}
			if s.Description != `` {
				line.WriteString(`: ` + strings.TrimSpace(s.Description))
			} else if s.Hint != `` {
				line.WriteString(`: ` + strings.TrimSpace(s.Hint))
			}
			break
		}
		out = append(out, line.String())
		if len(out) >= max {
			break
		}
	}
	return out
}

// talkLines are what is being said on the roads: the world's recent notable
// happenings, which anyone in a town with people in it would have heard.
func talkLines(room *rooms.Room, mob *mobs.Mob, max int) []string {
	if room == nil || max <= 0 || cannotSee(mob, room) {
		return nil
	}
	people := 0
	for _, id := range room.GetMobs() {
		if m := mobs.GetInstance(id); m != nil && id != mob.InstanceId && m.IsNonCombatant() {
			people++
		}
	}
	if people == 0 {
		return nil // nobody here to have heard it from
	}
	var out []string
	for _, evt := range worldevents.GetRecentWorldEvents(max, nil) {
		text := strings.TrimSpace(evt.Description)
		if text == `` {
			continue
		}
		where := evt.ZoneName
		if where == `` {
			where = evt.RegionName
		}
		if where != `` {
			text += ` (` + where + `)`
		}
		out = append(out, text)
	}
	return out
}

// cannotSeeOwner is whether the dark, or her eyes, keep her from seeing the
// state of her owner.
func cannotSeeOwner(mob *mobs.Mob, owner *users.UserRecord) bool {
	room := rooms.LoadRoom(mob.Character.RoomId)
	if room == nil || owner.Character.RoomId != mob.Character.RoomId {
		return true
	}
	return cannotSee(mob, room) || !mob.Character.Perceives(owner.Character)
}

// roomTrouble reports a reason to speak up on walking in: the guards here
// want her owner, or these are people who would gladly see the back of
// him. It returns the plain fact; what she makes of it is hers to say.
func roomTrouble(room *rooms.Room, mob *mobs.Mob, owner *users.UserRecord) string {
	if room == nil || owner == nil || owner.Character == nil || cannotSee(mob, room) {
		return ``
	}
	seen := map[string]bool{}
	var here []string
	worst := 0
	for _, id := range room.GetMobs() {
		m := mobs.GetInstance(id)
		if m == nil || id == mob.InstanceId || !mob.Character.Perceives(&m.Character) {
			continue
		}
		for _, f := range factions.FactionsForMob(m) {
			if seen[f] {
				continue
			}
			seen[f] = true
			here = append(here, f)
			if rep := factions.GetRep(f, owner.UserId); rep <= -50 && rep < worst {
				worst = rep
			}
		}
	}
	if sev := justice.Verdict(here, owner.UserId); sev >= justice.SeverityAttack {
		return `the guards here will draw on ` + owner.Character.Name + ` on sight`
	} else if sev > justice.SeverityNone {
		return `the guards here have a warrant out for ` + owner.Character.Name
	}
	if worst <= -50 {
		return `the people here would gladly see the back of ` + owner.Character.Name
	}
	return ``
}

// newAilments returns conditions that have appeared since last look, on
// either of them, so something going badly wrong is noticed when it
// happens rather than whenever she next speaks.
func newAilments(mob *mobs.Mob, owner *users.UserRecord, known map[int]bool, canSeeOwner bool) ([]string, map[int]bool) {
	now := map[int]bool{}
	var fresh []string
	note := func(ch *characters.Character, who string) {
		for _, cond := range ch.GetConditions() {
			if cond == nil || cond.ConditionId == 0 {
				continue
			}
			spec := conditions.GetConditionSpec(cond.ConditionId)
			if spec == nil || spec.Name == `` {
				continue
			}
			key := cond.ConditionId
			if who != `` {
				key = -cond.ConditionId
			}
			now[key] = true
			if !known[key] {
				if who == `` {
					fresh = append(fresh, `you are `+strings.ToLower(spec.Name))
				} else {
					fresh = append(fresh, who+` is `+strings.ToLower(spec.Name))
				}
			}
		}
	}
	note(&mob.Character, ``)
	if owner != nil && owner.Character != nil && canSeeOwner {
		note(owner.Character, owner.Character.Name)
	}
	sort.Strings(fresh)
	return fresh, now
}

// ownerIfPresent is the owner when she is standing with them: what they are
// in the middle of is something she picks up in their company, not news
// that reaches her two rooms away on an errand.
func ownerIfPresent(mob *mobs.Mob, owner *users.UserRecord) *users.UserRecord {
	if owner == nil || owner.Character == nil || mob == nil {
		return nil
	}
	if owner.Character.RoomId != mob.Character.RoomId {
		return nil
	}
	return owner
}
