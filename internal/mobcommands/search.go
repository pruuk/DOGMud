package mobcommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/companionai"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Search is a mob's search: the engine's own, every tier. A bonded AI
// companion's search may also roll for a bauble on its owner's behalf, when
// the module says so (companionai.BaubleSearchFor), and the module hears
// what it turned up (companionai.RouteSearched) so the companion can say
// so. For every other mob both are no-ops.
func Search(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {
	actor := &actions.MobActor{Mob: mob, Room: room}
	result := actions.Search(actor, actions.SearchOptions{BaubleForUserId: companionai.BaubleSearchFor(mob.InstanceId)})
	if !result.OnCooldown {
		companionai.RouteSearched(mob.InstanceId, searchFoundWords(result, room))
	}
	return true, nil
}

// searchFoundWords puts what a search turned up into plain words.
func searchFoundWords(r actions.SearchResult, room *rooms.Room) []string {
	var out []string
	for _, e := range r.HiddenExitsFound {
		out = append(out, fmt.Sprintf(`a hidden way out, %s`, e))
	}
	for _, c := range r.HiddenContainersFound {
		out = append(out, fmt.Sprintf(`a hidden %s`, c))
	}
	for _, s := range r.StashedItemsFound {
		out = append(out, fmt.Sprintf(`%s, stashed out of sight`, s.DisplayName))
	}
	for _, id := range r.HiddenMobsFound {
		if m := mobs.GetInstance(id); m != nil {
			out = append(out, fmt.Sprintf(`%s, hiding`, m.Character.Name))
		}
	}
	for _, id := range r.HiddenPlayersFound {
		if u := users.GetByUserId(id); u != nil && u.Character != nil {
			out = append(out, fmt.Sprintf(`%s, hiding`, u.Character.Name))
		}
	}
	for _, n := range r.HiddenNounsFound {
		out = append(out, fmt.Sprintf(`something about the %s worth a closer look`, n))
	}
	if r.BaubleFound {
		out = append(out, `something small glinting, which you are working loose`)
	}
	return out
}
