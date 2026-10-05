package roomlife

import (
	"encoding/json"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"github.com/GoMudEngine/GoMud/internal/roomlife"
)

// PromptVersion is logged with every call. Bump it whenever the prompt
// changes meaningfully.
const PromptVersion = 2

var systemPrompt = strings.Join([]string{
	`You write one small ambient event for a place in a text adventure set in Gaius, a cool, rugged, roughly medieval world of forests, marshes, steppe, old roads and small towns. Coin is gold. There is no modern technology. Most people carry the Chrysalis, a living presence that makes conviction real and slowly marks the body.`,
	`An ambient event is something that happens around the people standing in a place, the way a street or a wood is never quite still. Yours replaces one of the place's usual set lines (examples shows some), so keep their voice, but make it fresher and more specific: a small, vivid, sometimes wryly funny incident that belongs exactly here, at this time of day. For example a rat scurries out of a hole and runs along the foot of a wall, a gull steals a crust from a windowsill, a shutter bangs twice in the wind, a cart wheel squeals somewhere out of sight, a man and his wife argue behind a closed door about whose turn it was to fetch water.`,
	`Use the room's description and what is listed as present: the NPCs there by name (they may react in passing), things lying about, the terrain, indoors or out. Unnamed passers-by, animals, weather and distant people are fine. Small things only: no deaths, fires, fights, magic, crimes, arrivals of anyone who stays, or anything that would change the place; no new named people. Do not invent exits, doors or features the description rules out (no street noise deep in a cave, no birdsong in a sealed cellar).`,
	`Some places are not in Gaius at all. When setting is given it describes where this place really is, and it overrides the world above: follow it exactly, keep to what it says may and may not be there, and bring nothing of Gaius into it (no townsfolk, animals, birds, weather or sky unless the setting allows them). Such a place may have no time_of_day.`,
	`Players are present (travellers counts them). The event happens around them, never to them: never mention, address, touch, follow or react to any traveller, customer, visitor or stranger.`,
	`Choose kind "seen" for something that happens in view, written as plain third-person narration in the present tense, never using the word you: "A rat noses out of a gap in the boards and darts along the wall." Choose kind "heard" for a sound only, which must begin with the words "You hear" and use you nowhere else: "You hear a woman behind the bakery door telling someone, at length, exactly where the good knife went." When can_see is false the place is too dark to see (or the watcher cannot see): write a heard event only.`,
	`One or two short sentences, at most 40 words in all. Plain text in ASCII letters with ordinary punctuation ' " , . ! ? only: no markup, no asterisks, no emoji, no dashes, no curly quotes. No gore, nothing sexual, nothing hateful, no real-world people or brands.`,
}, "\n\n")

type promptEvent struct {
	Room             string   `json:"room"`
	RoomDescription  string   `json:"room_description,omitempty"`
	Terrain          string   `json:"terrain,omitempty"`
	Area             string   `json:"area,omitempty"`
	Indoors          bool     `json:"indoors"`
	TimeOfDay        string   `json:"time_of_day,omitempty"`
	Setting          string   `json:"setting,omitempty"`
	Examples         []string `json:"examples,omitempty"`
	NPCsPresent      []string `json:"npcs_present,omitempty"`
	ThingsLyingAbout []string `json:"things_lying_about,omitempty"`
	Travellers       int      `json:"travellers"`
	CanSee           bool     `json:"can_see"`
}

// buildMessages is the system prompt and the place.
func buildMessages(req roomlife.Request) []apiframework.Message {
	p := promptEvent{
		Room:             req.Title,
		RoomDescription:  req.Description,
		Terrain:          req.Biome,
		Area:             req.Zone,
		Indoors:          req.Indoors,
		TimeOfDay:        req.TimeOfDay,
		Setting:          req.Setting,
		Examples:         req.Examples,
		NPCsPresent:      req.NPCs,
		ThingsLyingAbout: req.FloorItems,
		Travellers:       req.Travellers,
		CanSee:           req.CanSee,
	}
	b, _ := json.MarshalIndent(p, ``, `  `)
	return []apiframework.Message{
		{Role: `system`, Content: systemPrompt},
		{Role: `user`, Content: `Write one ambient event for this place.` + "\n\n" + string(b)},
	}
}
