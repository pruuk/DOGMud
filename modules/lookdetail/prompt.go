package lookdetail

import (
	"encoding/json"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"github.com/GoMudEngine/GoMud/internal/lookdetail"
)

// PromptVersion is logged with every call. Bump it whenever the prompt
// changes meaningfully.
const PromptVersion = 1

var systemPrompt = strings.Join([]string{
	`You write what a closer look shows, in a text adventure set in Gaius, a cool, rugged, roughly medieval world of forests, marshes, steppe, old roads and small towns. Coin is gold. There is no modern technology. Most people carry the Chrysalis, a living presence that makes conviction real and slowly marks the body.`,
	`A player has looked at something the room's description mentions (thing; excerpt is the sentence naming it). Describe it up close: vivid, specific, colourful and true to this place, the way a good writer rewards a curious reader. Texture, wear, smell, small telling details, a hint of who made or uses it, and now and then a dry or playful touch. It must agree with the room's description and the time of day, and suit everything listed as present.`,
	`It is scenery, nothing more. Never invent anything a player could take, use, open, read in full or act on: no hidden objects, keys, levers, passages, compartments, treasure, clues, messages, quests, warnings or magic. Never contradict the description or add exits. No named people. Do not mention any other traveller or player; the looker may be addressed as you ("You notice...").`,
	`Two to four sentences, at most 90 words. Plain text in ASCII letters with ordinary punctuation ' " , . ! ? only: no markup, no asterisks, no emoji, no dashes, no curly quotes, no lists. No gore, nothing sexual, nothing hateful, no real-world people or brands.`,
}, "\n\n")

type promptLook struct {
	Thing            string   `json:"thing"`
	Excerpt          string   `json:"excerpt,omitempty"`
	Room             string   `json:"room"`
	RoomDescription  string   `json:"room_description,omitempty"`
	Terrain          string   `json:"terrain,omitempty"`
	Area             string   `json:"area,omitempty"`
	Indoors          bool     `json:"indoors"`
	TimeOfDay        string   `json:"time_of_day,omitempty"`
	NPCsPresent      []string `json:"npcs_present,omitempty"`
	ThingsLyingAbout []string `json:"things_lying_about,omitempty"`
}

// buildMessages is the system prompt and the look.
func buildMessages(req lookdetail.Request) []apiframework.Message {
	p := promptLook{
		Thing:            req.Thing,
		Excerpt:          req.Excerpt,
		Room:             req.Title,
		RoomDescription:  req.Description,
		Terrain:          req.Biome,
		Area:             req.Zone,
		Indoors:          req.Indoors,
		TimeOfDay:        req.TimeOfDay,
		NPCsPresent:      req.NPCs,
		ThingsLyingAbout: req.FloorItems,
	}
	b, _ := json.MarshalIndent(p, ``, `  `)
	return []apiframework.Message{
		{Role: `system`, Content: systemPrompt},
		{Role: `user`, Content: `Describe what a closer look at the thing shows.` + "\n\n" + string(b)},
	}
}
