package roomlife

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Limits on what a request carries.
const (
	maxRoomDescription = 900
	maxListed          = 10
	maxExamples        = 5
)

// Request is everything a generator is told: authored text and game facts
// only. Players are counted (Travellers), never named; floor items are
// named by their model-safe names (items.Item.ModelName).
type Request struct {
	Title       string
	Description string
	Biome       string
	Zone        string
	Indoors     bool
	TimeOfDay   string   // "day" or "night"; empty in a timeless place
	Setting     string   // where the place really is, when not the usual world (Place.Setting)
	Examples    []string // the place's own set ambient lines, for its voice
	NPCs        []string // NPCs present, by name
	FloorItems  []string // what lies about, by name
	Travellers  int      // players present

	// CanSee is whether the keyholder sees the room clearly. When false the
	// event must be heard (KindHeard): a dark room, a blinded keyholder.
	CanSee bool
}

// Snapshot builds a Request for room, with pool its set ambient lines.
// Runs under the mud lock.
func Snapshot(room *rooms.Room, pool []string) Request {
	req := Request{
		Title:       plain(room.Title, 120),
		Description: plain(room.GetDescription(), maxRoomDescription),
		Biome:       plain(room.Biome, 40),
		Zone:        plain(room.Zone, 60),
		Travellers:  len(room.GetPlayers()),
		TimeOfDay:   `day`,
	}
	if b := room.GetBiome(); b != nil {
		req.Indoors = b.Indoor
	}
	if gametime.GetDate().Night {
		req.TimeOfDay = `night`
	}
	if place, ok := placeFor(room); ok {
		req.Setting = plain(place.Setting, maxRoomDescription)
		if place.Timeless {
			req.TimeOfDay = ``
		}
	}
	seen := map[string]bool{}
	for _, m := range pool {
		if len(req.Examples) >= maxExamples {
			break
		}
		if m = plain(m, 240); m != `` && !seen[m] {
			seen[m] = true
			req.Examples = append(req.Examples, m)
		}
	}
	npcs := newNameList()
	for _, id := range room.GetMobs() {
		if mob := mobs.GetInstance(id); mob != nil {
			npcs.add(mob.Character.Name)
		}
	}
	req.NPCs = npcs.names
	floor := newNameList()
	for i := range room.Items {
		floor.add(room.Items[i].ModelName())
	}
	req.FloorItems = floor.names
	return req
}

type nameList struct {
	seen  map[string]bool
	names []string
}

func newNameList() *nameList { return &nameList{seen: map[string]bool{}} }

func (l *nameList) add(name string) {
	name = plain(name, 60)
	key := strings.ToLower(name)
	if name == `` || l.seen[key] || len(l.names) >= maxListed {
		return
	}
	l.seen[key] = true
	l.names = append(l.names, name)
}

// plain strips markup and folds typography (baubles.PlainText), then cuts
// it to at most n runes.
func plain(s string, n int) string {
	s = baubles.PlainText(s)
	if r := []rune(s); len(r) > n {
		s = strings.TrimSpace(string(r[:n])) + `...`
	}
	return s
}

// The two kinds of event.
const (
	KindSeen  = `seen`  // something that happens in view
	KindHeard = `heard` // a sound, heard whatever the light
)

// ReplySchemaName names the reply's strict JSON schema. It must be listed
// in modules/aicompanion/relayweb/relay.js LIVELY_SCHEMAS (a test holds
// it).
const ReplySchemaName = `room_event`

// Bounds on an event's text, after cleaning.
const (
	MinTextRunes = 12
	MaxTextRunes = 300
)

// ReplySchema is the strict JSON schema of a reply: one event.
func ReplySchema() map[string]any {
	return map[string]any{
		`type`: `object`,
		`properties`: map[string]any{
			`kind`: map[string]any{
				`type`:        `string`,
				`enum`:        []string{KindSeen, KindHeard},
				`description`: `seen for something that happens in view, heard for a sound only`,
			},
			`text`: map[string]any{
				`type`:        `string`,
				`description`: `the event as narration; a heard event begins "You hear"`,
			},
		},
		`required`:             []string{`kind`, `text`},
		`additionalProperties`: false,
	}
}

// ErrUnusable is a reply that parsed but cannot be shown.
var ErrUnusable = errors.New(`unusable room event`)

// ParseReply reads a model's reply content.
func ParseReply(content string) (Result, error) {
	var r struct {
		Kind string `json:"kind"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &r); err != nil {
		return Result{}, fmt.Errorf(`reply is not the schema's JSON: %w`, err)
	}
	return Result{Kind: r.Kind, Text: r.Text}, nil
}

var (
	secondPerson = regexp.MustCompile(`(?i)\b(you|your|yours|yourself|you're|youre|you'll|you've|you'd)\b`)
	heardPrefix  = regexp.MustCompile(`(?i)^you hear\b`)
	dashes       = regexp.MustCompile(`\s+-+\s+|\s*--+\s*`)
)

// CleanResult holds an event to what may be shown: a known kind (only
// heard when canSee is false, the light rule); plain printable ASCII once
// markup is stripped and typography folded; one line of MinTextRunes to
// MaxTextRunes. A heard event must begin "You hear" (capitalised here) and
// say "you" nowhere else; a seen one never says "you": the event happens
// around the players, never to them. Dashes become commas. A failure wraps
// ErrUnusable.
func CleanResult(r Result, canSee bool) (Result, error) {
	kind := strings.ToLower(strings.TrimSpace(r.Kind))
	if kind != KindSeen && kind != KindHeard {
		return Result{}, fmt.Errorf(`%w: kind %q`, ErrUnusable, r.Kind)
	}
	if kind == KindSeen && !canSee {
		return Result{}, fmt.Errorf(`%w: a seen event for a keyholder who cannot see`, ErrUnusable)
	}
	text := baubles.PlainText(r.Text)
	text = strings.Trim(text, ` *_~"`)
	text = dashes.ReplaceAllString(text, `, `)
	text = strings.Join(strings.Fields(text), ` `)
	text = strings.ReplaceAll(text, ` ,`, `,`)
	text = strings.ReplaceAll(text, `,,`, `,`)
	for _, c := range text {
		if c < 0x20 || c > 0x7e || c == '<' || c == '>' || c == '`' || c == '\\' {
			return Result{}, fmt.Errorf(`%w: character outside plain text`, ErrUnusable)
		}
	}
	if n := len([]rune(text)); n < MinTextRunes || n > MaxTextRunes {
		return Result{}, fmt.Errorf(`%w: %d characters`, ErrUnusable, n)
	}
	rest := text
	if kind == KindHeard {
		loc := heardPrefix.FindStringIndex(text)
		if loc == nil {
			return Result{}, fmt.Errorf(`%w: a heard event that does not begin "You hear"`, ErrUnusable)
		}
		text = `You hear` + text[loc[1]:]
		rest = text[loc[1]:]
	}
	if secondPerson.MatchString(rest) {
		return Result{}, fmt.Errorf(`%w: an event addressing the reader`, ErrUnusable)
	}
	return Result{Kind: kind, Text: text, KeyholderOnly: r.KeyholderOnly}, nil
}
