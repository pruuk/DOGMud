package lookdetail

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

// Request is everything a generator is told: authored room text and game
// facts only, never a player.
type Request struct {
	Thing       string // what was looked at, as the description names it
	Excerpt     string // the sentence of the description naming it
	Title       string
	Description string
	Biome       string
	Zone        string
	Indoors     bool
	TimeOfDay   string   // "day" or "night"
	NPCs        []string // NPCs present, by name
	FloorItems  []string // what lies about, by model-safe name
}

const (
	maxRoomDescription = 1200
	maxListed          = 10
)

// Snapshot builds a Request for a closer look at thing in room. Runs under
// the mud lock.
func Snapshot(room *rooms.Room, thing string, excerpt string) Request {
	req := Request{
		Thing:       thing,
		Excerpt:     excerpt,
		Title:       plain(room.Title, 120),
		Description: plain(room.GetDescription(), maxRoomDescription),
		Biome:       plain(room.Biome, 40),
		Zone:        plain(room.Zone, 60),
		TimeOfDay:   `day`,
	}
	if b := room.GetBiome(); b != nil {
		req.Indoors = b.Indoor
	}
	if gametime.GetDate().Night {
		req.TimeOfDay = `night`
	}
	seen := map[string]bool{}
	for _, id := range room.GetMobs() {
		if mob := mobs.GetInstance(id); mob != nil && len(req.NPCs) < maxListed {
			if n := plain(mob.Character.Name, 60); n != `` && !seen[n] {
				seen[n] = true
				req.NPCs = append(req.NPCs, n)
			}
		}
	}
	for i := range room.Items {
		if len(req.FloorItems) >= maxListed {
			break
		}
		if n := plain(room.Items[i].ModelName(), 60); n != `` && !seen[n] {
			seen[n] = true
			req.FloorItems = append(req.FloorItems, n)
		}
	}
	return req
}

// plain strips markup and folds typography, then cuts to at most n runes.
func plain(s string, n int) string {
	s = baubles.PlainText(s)
	if r := []rune(s); len(r) > n {
		s = strings.TrimSpace(string(r[:n])) + `...`
	}
	return s
}

// ReplySchemaName names the reply's strict JSON schema. It must be listed in
// modules/aicompanion/relayweb/relay.js LIVELY_SCHEMAS (a test holds it).
const ReplySchemaName = `look_detail`

// Bounds on a detail's text, after cleaning.
const (
	MinTextRunes = 20
	MaxTextRunes = 700
)

// ReplySchema is the strict JSON schema of a reply: one detail.
func ReplySchema() map[string]any {
	return map[string]any{
		`type`: `object`,
		`properties`: map[string]any{
			`text`: map[string]any{
				`type`:        `string`,
				`description`: `what a closer look at the thing shows, 2 to 4 sentences`,
			},
		},
		`required`:             []string{`text`},
		`additionalProperties`: false,
	}
}

// ErrUnusable is a reply that parsed but cannot be shown.
var ErrUnusable = errors.New(`unusable closer look`)

// ParseReply reads a model's reply content.
func ParseReply(content string) (Result, error) {
	var r struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &r); err != nil {
		return Result{}, fmt.Errorf(`reply is not the schema's JSON: %w`, err)
	}
	return Result{Text: r.Text}, nil
}

var dashes = regexp.MustCompile(`\s+-+\s+|\s*--+\s*`)

// CleanResult holds a detail to what may be shown: plain printable ASCII
// once markup is stripped and typography folded, one paragraph of
// MinTextRunes to MaxTextRunes. Dashes become commas. "You" is allowed: a
// closer look is the looker's own. A failure wraps ErrUnusable.
func CleanResult(r Result) (Result, error) {
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
	return Result{Text: text, KeyholderOnly: r.KeyholderOnly}, nil
}
