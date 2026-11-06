package questengine

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// Destinations is the static destination rule (#375 phase 4, spec section 5).
// Boot and the CI sweep refuse on any of its findings (since the #375
// closing step). Secret quests and each quest's final step are skipped. It
// returns the findings sorted by key, and "quest <id> step <step>" for each
// step whose destination is inferred.
//
//	walk-destination           the step resolves to unknown
//	walk-destination-anywhere  an anywhere step whose triggers name a place
func Destinations(qs []*quests.Quest) (findings []questloop.Finding, inferred []string) {
	return destinationsWith(qs, spawnRoomsOf)
}

// SpawnChangeFindings is the before-and-after check for a spawn list changed
// on a running server (admin `room set spawninfo clear`, `room copy
// spawninfo`). It returns the Destinations findings over the loaded quests
// that giving template room roomId a spawn list placing mobIds would add.
// Boot refuses any Destinations finding, so a change that adds one would stop
// the next restart; its caller refuses the change instead. A finding the
// world already had is not the change's doing and is not returned.
func SpawnChangeFindings(roomId int, mobIds []int) []questloop.Finding {
	all := quests.GetAllQuests()
	qs := make([]*quests.Quest, 0, len(all))
	for i := range all {
		qs = append(qs, &all[i])
	}
	return spawnChangeFindings(qs, roomId, mobIds)
}

// spawnChangeFindings is SpawnChangeFindings over any quests, for tests.
func spawnChangeFindings(qs []*quests.Quest, roomId int, mobIds []int) []questloop.Finding {
	before, _ := destinationsWith(qs, spawnRoomsOf)
	after, _ := destinationsWith(qs, func(mobId int) []int {
		ids := slices.DeleteFunc(slices.Clone(spawnRoomsOf(mobId)), func(id int) bool { return id == roomId })
		if slices.Contains(mobIds, mobId) {
			ids = append(ids, roomId)
		}
		return ids
	})
	had := make(map[string]bool, len(before))
	for _, f := range before {
		had[f.Key()] = true
	}
	var added []questloop.Finding
	for _, f := range after {
		if !had[f.Key()] {
			added = append(added, f)
		}
	}
	return added
}

// destinationsWith is Destinations with spawnsOf answering which template
// rooms place a mob, so a proposed spawn change can be judged unsaved.
func destinationsWith(qs []*quests.Quest, spawnsOf func(int) []int) (findings []questloop.Finding, inferred []string) {
	sorted := slices.Clone(qs)
	slices.SortFunc(sorted, func(a, b *quests.Quest) int { return cmp.Compare(a.QuestId, b.QuestId) })
	for _, q := range sorted {
		if q == nil || q.Secret {
			continue
		}
		for i, s := range q.Steps {
			if i == len(q.Steps)-1 {
				continue
			}
			where := "step " + s.Id
			p := placesOf(q, s.Id)
			d := s.Destination
			switch {
			case d.Anywhere:
				if !p.empty() {
					findings = append(findings, questloop.Finding{Rule: "walk-destination-anywhere", QuestId: q.QuestId, Where: where,
						Msg: "marked anywhere, but a trigger on this step names a place; drop anywhere so the destination is inferred"})
				}
			case !d.IsZero():
				// A mob list with no declared fallback needs only one of
				// its mobs to have spawn rooms (owner ruling 16).
				if len(d.Mobs) > 0 && d.Room == 0 && d.Area == "" && d.Zone == "" &&
					!slices.ContainsFunc(d.Mobs, func(m int) bool { return len(spawnsOf(m)) > 0 }) {
					findings = append(findings, questloop.Finding{Rule: "walk-destination", QuestId: q.QuestId, Where: where,
						Msg: fmt.Sprintf("%s has no fallback and no room spawns it; add room, area or zone", mobIdsText(d.Mobs))})
				}
			default:
				if staticInferable(p, spawnsOf) {
					inferred = append(inferred, fmt.Sprintf("quest %d step %s", q.QuestId, s.Id))
				} else {
					findings = append(findings, questloop.Finding{Rule: "walk-destination", QuestId: q.QuestId, Where: where,
						Msg: "no destination and no trigger on this step names a place; give it destination: (room, mob, area or zone) or destination: anywhere"})
				}
			}
		}
	}
	slices.SortFunc(findings, func(a, b questloop.Finding) int { return cmp.Compare(a.Key(), b.Key()) })
	return findings, inferred
}

// mobIdsText names a destination's mobs for a finding: "mob 94", or
// "mobs 9316, 9300" for a list. A single id keeps the message the old backlog
// already records.
func mobIdsText(ids []int) string {
	if len(ids) == 1 {
		return fmt.Sprintf("mob %d", ids[0])
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprint(id)
	}
	return "mobs " + strings.Join(parts, ", ")
}

// staticInferable reports whether a step's trigger places resolve without a
// live world: any room, area or zone, or a mob some room spawns.
func staticInferable(p stepPlaces, spawnsOf func(int) []int) bool {
	if len(p.rooms)+len(p.areas)+len(p.zones) > 0 {
		return true
	}
	for _, m := range p.mobs {
		if len(spawnsOf(m)) > 0 {
			return true
		}
	}
	return false
}
