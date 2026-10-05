package rifts

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
	rifts "github.com/GoMudEngine/GoMud/internal/rifts"
)

// RoomPromptVersion is logged with every call and saved with every room
// written. Bump it whenever the prompt changes meaningfully.
const RoomPromptVersion = 5

var roomSystemPrompt = strings.Join([]string{
	`You write one new room for a dungeon in a text adventure. Players read its rooms one after another, so each room must be an experience to read: vivid, specific, strange and never generic. The dungeon's setting is given (setting); keep to it exactly and contradict none of it. The kind of room you are writing is given (room_kind); follow it. Examples shows rooms of the same kind already in the dungeon, in the exact shape you must answer in: match their voice, richness and length, but write something new.`,
	`Be unique. Your room must have a central idea of its own, unlike every example and every title in used_titles, and one specific thing a player will remember it by. Seeds offers a few words: let one or more of them start your idea, loosely. Give the title in Title Case, at most six words.`,
	`The description is one paragraph whose length is given (description_words). It mentions every thing in nouns, so a reader knows what to look at. Each noun is one lowercase word of at least three letters that a player types, and its look adds something the description did not say: a detail, a mark, an oddity, a clue, a hint of the lives of those who made this place. Never use a noun in reserved_nouns (the game places those itself) and never name a noun the same as one of your doors.`,
	`Doors: exactly the number given (doors), each exit one lowercase word of three or more letters only, all different, never in reserved_nouns, never a direction or a command and never starting with one (north, up, look, get, go, drop, say, enter, out: a door named gorge would swallow go), and varied (arch, stair, cleft, crawl, ramp, slit, spiral, chute, sill, vault, gap, lintel, threshold, shaft, notch, culvert, landing, aisle, ledge, hollow, span, nave, loft and the like). Each door's description says in one or two sentences what is seen of that way on. Idle messages are one or two short lines, in the present tense, of something that happens in the room.`,
	`A puzzle or trap room: a puzzle, a trap, or both (the other null). The puzzle is always the game's lens table: never describe it or use its noun; you choose only which door it holds shut (sealed_door) and what the solver sees as the last lenses lock and the way opens (solved); the game gives the solver a key. Both the description and that door's description say it is sealed. A trap is sprung on entry unless the player notices it; triggered and avoided say what each one experiences, and the effect should match the text. A monster room: the game places the creatures; hint at movement, never name a creature and never say how many.`,
	`Tone: quiet, uncanny, melancholy, precise. Concrete sensory detail beats adjectives. Use you sparingly and naturally. No gore, nothing sexual, nothing hateful, nothing silly, no modern words, no real-world people or places, and no cliches (eerie silence, shivers down your spine, ancient evil, palpable). Plain text in ASCII letters with ordinary punctuation only: no markup, no asterisks, no emoji, no dashes, no curly quotes.`,
}, "\n\n")

type promptRoom struct {
	Dungeon          string            `json:"dungeon"`
	Setting          string            `json:"setting"`
	RoomKind         string            `json:"room_kind"`
	DescriptionWords string            `json:"description_words"`
	Doors            int               `json:"doors"`
	WayOut           string            `json:"way_out,omitempty"`
	ReservedNouns    []string          `json:"reserved_nouns"`
	Effects          []string          `json:"effects,omitempty"`
	Seeds            []string          `json:"seeds"`
	UsedTitles       []string          `json:"used_titles"`
	Examples         []json.RawMessage `json:"examples"`
}

// buildRoomMessages is the system prompt and the room asked for.
func buildRoomMessages(req rifts.GenRequest) []apiframework.Message {
	p := promptRoom{
		Dungeon:          req.ProfileName,
		Setting:          req.Setting,
		RoomKind:         req.Guide,
		DescriptionWords: fmt.Sprintf(`%d to %d`, req.Words.Min, req.Words.Max),
		Doors:            req.Doors,
		ReservedNouns:    req.ReservedNouns,
		Seeds:            req.Seeds,
		UsedTitles:       req.PromptTitles,
	}
	if req.ExitName != `` {
		p.WayOut = fmt.Sprintf(`The game adds the way out itself, an exit named %q: never list it among the doors, but name it in the description, as "the %s".`, req.ExitName, req.ExitName)
	}
	if req.Pool == rifts.PoolPuzzle {
		p.Effects = req.Effects
	}
	for _, ex := range req.Examples {
		p.Examples = append(p.Examples, json.RawMessage(ex))
	}
	if p.UsedTitles == nil {
		p.UsedTitles = []string{}
	}
	b, _ := json.MarshalIndent(p, ``, `  `)
	return []apiframework.Message{
		{Role: `system`, Content: roomSystemPrompt},
		{Role: `user`, Content: `Write one new room of this kind for this dungeon.` + "\n\n" + string(b)},
	}
}
