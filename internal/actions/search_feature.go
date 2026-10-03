package actions

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Targeted search: `search bookshelf` (owner rulings, 2026-09-26 and 27).
//
// `search <feature>` is the whole room search, every tier exactly as a
// plain `search` (secret exits, hidden containers, stashes, hiders, hidden
// nouns: quests hide their items behind `search shelf`, `search floor` and
// the like, so a targeted search must never find less), plus one thing:
// its bauble roll is the feature's own. A feature is a room noun (what
// `look <noun>` describes), a hidden noun the player has already
// discovered, or a container they can see. Words that name no feature are
// a plain search, as `search <anything>` always was.
//
// Only BAUBLES are limited per feature: each feature offers one bauble roll
// per BaubleFeatureWindowMinutes (60, real time), by anyone (or per player
// with BaubleWindowPerPlayer), apart from the room's two, and the search
// claims it whatever it turns up. When it is spent the search quietly takes
// the room's roll instead, exactly as a plain search would. Searching
// itself is never refused.
//
// A find is said to be on the feature ("on the bookshelf") when it is left
// lying in the room: see deliver in search_bauble.go.
//
// Anti-oracle: an undiscovered hidden noun or container never matches, so
// naming one is a plain search, exactly like a word that means nothing.

// FeatureKind is what sort of thing a targeted search named.
type FeatureKind string

const (
	FeatureNoun       FeatureKind = `noun`
	FeatureHiddenNoun FeatureKind = `hidden noun` // discovered by this player
	FeatureContainer  FeatureKind = `container`
)

// SearchFeature is one searchable thing in a room.
type SearchFeature struct {
	Name        string // the room's own key for it (canonical, not an alias)
	Kind        FeatureKind
	Description string // authored text; empty for a container
}

// Spot is where a find from this feature lies when it is left in the room,
// as shown after its name: "on the bookshelf", or "beside the chest" for a
// container (the find lies in the room, not inside the container).
func (f SearchFeature) Spot() string {
	if f.Name == `` {
		return ``
	}
	if f.Kind == FeatureContainer {
		return `beside the ` + f.Name
	}
	return `on the ` + f.Name
}

// WindowName is the feature's roll-window key: its canonical name, lower
// case, whatever kind it is, so a noun and a container that share a name
// share one window rather than offering two.
func (f SearchFeature) WindowName() string {
	return strings.ToLower(strings.TrimSpace(f.Name))
}

// searchLeadWords are dropped from the front of what the player typed, so
// `search under the table` and `search in the barrel` name the table and
// the barrel.
var searchLeadWords = map[string]bool{
	`the`: true, `a`: true, `an`: true,
	`in`: true, `inside`: true, `into`: true, `on`: true, `onto`: true,
	`under`: true, `underneath`: true, `beneath`: true, `below`: true,
	`behind`: true, `around`: true, `through`: true, `among`: true,
	`at`: true, `by`: true, `near`: true, `beside`: true, `over`: true,
}

// normalizeFeatureName lower-cases and trims what the player typed and drops
// leading prepositions and articles. It can return "".
func normalizeFeatureName(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for len(words) > 0 && searchLeadWords[words[0]] {
		words = words[1:]
	}
	return strings.Join(words, ` `)
}

// FindSearchFeature resolves what a player typed after `search` to a
// feature of the room they can search. Room nouns come first (the same
// lookup `look` uses, aliases and plurals included), then hidden nouns this
// character has discovered, then containers they can see. A hidden noun or
// container they have not discovered NEVER matches.
func FindSearchFeature(char *characters.Character, room *rooms.Room, typed string) (SearchFeature, bool) {
	name := normalizeFeatureName(typed)
	if name == `` || room == nil {
		return SearchFeature{}, false
	}

	if noun, desc := room.FindNoun(name); noun != `` {
		return SearchFeature{Name: noun, Kind: FeatureNoun, Description: desc}, true
	}

	if char != nil && len(room.HiddenNouns) > 0 {
		known := make([]string, 0, len(room.HiddenNouns))
		for key := range room.HiddenNouns {
			if char.HasDiscovery(room.RoomId, key) {
				known = append(known, key)
			}
		}
		sort.Strings(known)
		if key := bestMatch(name, known); key != `` {
			return SearchFeature{Name: key, Kind: FeatureHiddenNoun, Description: room.HiddenNouns[key].Description}, true
		}
	}

	if len(room.Containers) > 0 {
		visible := make([]string, 0, len(room.Containers))
		for key, c := range room.Containers {
			if c.Hidden && (char == nil || !char.HasDiscovery(room.RoomId, key)) {
				continue
			}
			visible = append(visible, key)
		}
		sort.Strings(visible)
		if key := bestMatch(name, visible); key != `` {
			return SearchFeature{Name: key, Kind: FeatureContainer}, true
		}
	}

	return SearchFeature{}, false
}

// bestMatch is util.FindMatchIn's exact match, else its close match.
func bestMatch(name string, candidates []string) string {
	if len(candidates) == 0 {
		return ``
	}
	exact, closeMatch := util.FindMatchIn(name, candidates...)
	if exact != `` {
		return exact
	}
	return closeMatch
}

// searchFeatureForBauble is a feature search's bauble roll. used is false
// when the feature's roll is spent this window (the caller takes the
// room's roll instead); a room that offers no baubles still counts as used,
// so it takes no roll at all. On a find the request names the feature and
// carries its authored description.
func searchFeatureForBauble(actor Actor, room *rooms.Room, feature SearchFeature) (found bool, used bool) {
	now := time.Now()
	if ok, _, _ := baubles.FeatureSearchable(room.RoomId, actor.GetUserId(), feature.WindowName(), now); !ok {
		return false, false
	}
	baubles.ClaimFeatureSearch(room.RoomId, actor.GetUserId(), feature.WindowName(), now)
	if !baubleRoomAllowed(room) {
		return false, true
	}
	household := householdFind(room)
	tier, found := searchBaubleRoll(baubles.FindOpts{
		Place:       BaublePlace(room),
		UserId:      actor.GetUserId(),
		SkillFactor: BaubleSkillFactor(actor.GetCharacter()),
		// sight ramp (plan 5b): the searcher needs to see what glints.
		SightPenalty: 1 - messaging.SightMult(actor.GetCharacter(), room),
		Feature:      feature.WindowName(),
		Household:    household,
	})
	if !found {
		return false, true
	}
	actor.SendText(messaging.CategorySystem,
		fmt.Sprintf(`Something glints about the <ansi fg="noun">%s</ansi>. You set about working it loose...`, feature.Name))
	startBaubleFind(actor.GetUserId(), room, tier, baubles.SourceSearch, feature, household)
	return true, true
}

// RoomSearchFeatures lists every feature of the room a targeted search could
// ever name, hidden ones included, sorted by name, for the admin `bauble
// window` view. Noun aliases (a noun whose text is ":other") are left out:
// they resolve to, and share the window of, the noun they point at.
func RoomSearchFeatures(room *rooms.Room) []SearchFeature {
	if room == nil {
		return nil
	}
	seen := map[string]bool{}
	out := []SearchFeature{}
	add := func(f SearchFeature) {
		if seen[f.WindowName()] {
			return
		}
		seen[f.WindowName()] = true
		out = append(out, f)
	}
	for noun, desc := range room.Nouns {
		if strings.HasPrefix(desc, `:`) {
			continue
		}
		add(SearchFeature{Name: noun, Kind: FeatureNoun, Description: desc})
	}
	for key, hn := range room.HiddenNouns {
		add(SearchFeature{Name: key, Kind: FeatureHiddenNoun, Description: hn.Description})
	}
	for key := range room.Containers {
		add(SearchFeature{Name: key, Kind: FeatureContainer})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// FeatureCacheHook lets a subsystem own the find of one searchable feature:
// a rift's rubble pile is a cache, searched once, not a feature with a bauble
// roll window. It is asked whenever a player's `search <feature>` names a
// feature, in place of that feature's bauble roll. handled false means "not
// mine": the feature takes its ordinary roll. handled true means the hook has
// dealt with the feature (delivering a find, or saying there is nothing left)
// and found reports whether a find is on its way, which counts as a won search
// like any bauble find. Set once at boot by the subsystem (modules/rifts).
type FeatureCacheHook func(actor Actor, room *rooms.Room, feature SearchFeature) (found bool, handled bool)

var featureCacheHook FeatureCacheHook

// SetFeatureCacheHook registers the feature-cache hook. Passing nil clears it.
func SetFeatureCacheHook(fn FeatureCacheHook) {
	featureCacheHook = fn
}
