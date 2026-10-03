package npcidle

import (
	"encoding/json"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"github.com/GoMudEngine/GoMud/internal/npcidle"
)

// PromptVersion is logged with every call. Bump it whenever the prompt
// changes meaningfully.
const PromptVersion = 1

// systemPrompt is fixed per PromptVersion.
var systemPrompt = strings.Join([]string{
	`You write one small idle moment for a non-player character (an NPC) in a text adventure set in Gaius, a cool, rugged, roughly medieval world of forests, marshes, steppe, old roads and small towns. Coin is gold. There is no modern technology. Most people carry the Chrysalis, a living presence that makes conviction real and slowly marks the body.`,
	`An idle moment is what the NPC does or mutters while nothing much is happening, the way a shopkeeper polishes a cup or a guard stamps against the cold. Yours replaces one of its usual set lines (examples shows some), so keep its voice and station, but make it fresher and more specific than those: a small, vivid, often wryly funny incident drawn from what is actually there. For example a bird lands on its head and leaves a gift, a shelf keeps dropping the same jar, a draught steals a ledger page, it argues under its breath with a stubborn knot, it sniffs a ware and pulls a face, it trades a look with another NPC present.`,
	`Use the NPC's description, the room's description and what is listed as present: other NPCs by name, things lying about, and a merchant's wares. Small things only: no deaths, fires, fights, magic, thefts, new arrivals or anything that would change the world, and no new named people. Do not invent exits, doors or features the room does not have.`,
	`Players are present (travellers counts them). Never mention, address, look at, approach, react to or interact with them or any traveller, customer, visitor or stranger. Never use the word you. The moment happens around them, not to them.`,
	`Choose kind "emote" for something the NPC does, or "say" for a short remark it makes aloud to itself, another NPC present, an animal or an object. For an emote, write the action in the third person present tense WITHOUT the NPC's name or a pronoun at the start, because the name is put in front of it: "swats at a moth circling the lamp and misses badly". For a say, write only the spoken words, no quotes and no speaker.`,
	`One or two short sentences, at most 35 words in all. Plain text in ASCII letters with ordinary punctuation ' " , . ! ? only: no markup, no asterisks, no emoji, no dashes, no curly quotes. No gore, nothing sexual, nothing hateful, no real-world people or brands.`,
}, "\n\n")

// promptMoment is the user message: the NPC and its surroundings, as data.
// Every field is authored game text or a game fact; nothing a player typed,
// and no player is named.
type promptMoment struct {
	NPC              string   `json:"npc"`
	NPCDescription   string   `json:"npc_description,omitempty"`
	Merchant         bool     `json:"merchant,omitempty"`
	Wares            []string `json:"wares,omitempty"`
	Examples         []string `json:"examples,omitempty"`
	Room             string   `json:"room"`
	RoomDescription  string   `json:"room_description,omitempty"`
	Terrain          string   `json:"terrain,omitempty"`
	Area             string   `json:"area,omitempty"`
	TimeOfDay        string   `json:"time_of_day,omitempty"`
	OtherNPCs        []string `json:"other_npcs_present,omitempty"`
	ThingsLyingAbout []string `json:"things_lying_about,omitempty"`
	Travellers       int      `json:"travellers"`
}

// buildMessages is the system prompt and the moment.
func buildMessages(req npcidle.Request) []apiframework.Message {
	p := promptMoment{
		NPC:              req.NPC.Name,
		NPCDescription:   req.NPC.Description,
		Merchant:         req.NPC.Merchant,
		Wares:            req.NPC.Wares,
		Examples:         req.NPC.Examples,
		Room:             req.Place.Title,
		RoomDescription:  req.Place.Description,
		Terrain:          req.Place.Biome,
		Area:             req.Place.Zone,
		TimeOfDay:        req.TimeOfDay,
		OtherNPCs:        req.Others,
		ThingsLyingAbout: req.FloorItems,
		Travellers:       req.Travellers,
	}
	b, _ := json.MarshalIndent(p, ``, `  `)
	return []apiframework.Message{
		{Role: `system`, Content: systemPrompt},
		{Role: `user`, Content: `Write one idle moment for this NPC.` + "\n\n" + string(b)},
	}
}
