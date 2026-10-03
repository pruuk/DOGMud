package npcidle

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/baubles"
)

// The two kinds of moment, and the mob commands that act them.
const (
	KindEmote = `emote`
	KindSay   = `say`
)

// ReplySchemaName names the reply's strict JSON schema. The companion's key
// relay (modules/aicompanion/relayweb/relay.js LIVELY_SCHEMAS) relays a
// request under this name only for a player who left "Make the world
// livelier" ticked on the key page; a test holds it in that list.
const ReplySchemaName = `npc_idle`

// Bounds on a moment's text, after cleaning.
const (
	MinTextRunes = 8
	MaxTextRunes = 240
)

// ReplySchema is the strict JSON schema of a reply: one moment.
func ReplySchema() map[string]any {
	return map[string]any{
		`type`: `object`,
		`properties`: map[string]any{
			`kind`: map[string]any{
				`type`:        `string`,
				`enum`:        []string{KindEmote, KindSay},
				`description`: `emote for something the NPC does, say for something it says aloud`,
			},
			`text`: map[string]any{
				`type`:        `string`,
				`description`: `for an emote, the action in the third person WITHOUT the NPC's name (it is put in front); for a say, only the spoken words`,
			},
		},
		`required`:             []string{`kind`, `text`},
		`additionalProperties`: false,
	}
}

// ErrUnusable is a reply that parsed but cannot be shown: the model
// answered, so it is not the key failing.
var ErrUnusable = errors.New(`unusable idle moment`)

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

// secondPerson finds a word addressing the reader. An emote is narrated to
// everyone in the room, so "you" in it would be a player, which a moment
// must never involve.
var secondPerson = regexp.MustCompile(`(?i)\b(you|your|yours|yourself|you're|youre|you'll|you've|you'd)\b`)

// CleanResult holds a moment to what may be shown: a known kind; plain
// printable ASCII once markup is stripped and typography folded (no tags,
// no control characters, nothing a player's provider could smuggle in);
// one line of MinTextRunes to MaxTextRunes; an emote that does not start
// with the NPC's own name (the emote command puts it there), is more than
// one word (a single word is an emote alias), and does not address "you";
// a say without wrapping quotes. Dashes become commas, the game's prose
// rule. A failure wraps ErrUnusable.
func CleanResult(r Result, npcName string) (Result, error) {
	kind := strings.ToLower(strings.TrimSpace(r.Kind))
	if kind != KindEmote && kind != KindSay {
		return Result{}, fmt.Errorf(`%w: kind %q`, ErrUnusable, r.Kind)
	}
	text := baubles.PlainText(r.Text)
	text = strings.Trim(text, ` *_~`)
	if kind == KindSay {
		text = strings.TrimSpace(strings.Trim(text, `"'`))
	}
	if kind == KindEmote {
		name := baubles.PlainText(npcName)
		for _, n := range []string{name, strings.TrimPrefix(strings.TrimPrefix(name, `The `), `the `)} {
			if n != `` && len(text) > len(n) && strings.EqualFold(text[:len(n)], n) && text[len(n)] == ' ' {
				text = strings.TrimSpace(text[len(n):])
				break
			}
		}
	}
	text = dashes.ReplaceAllString(text, `, `)
	text = strings.Join(strings.Fields(text), ` `)
	text = strings.ReplaceAll(text, ` ,`, `,`)
	text = strings.ReplaceAll(text, `,,`, `,`)

	for _, c := range text {
		if c < 0x20 || c > 0x7e || c == '<' || c == '>' || c == '`' || c == '\\' {
			return Result{}, fmt.Errorf(`%w: character outside plain text`, ErrUnusable)
		}
	}
	n := len([]rune(text))
	if n < MinTextRunes || n > MaxTextRunes {
		return Result{}, fmt.Errorf(`%w: %d characters`, ErrUnusable, n)
	}
	if kind == KindEmote {
		if !strings.Contains(text, ` `) {
			return Result{}, fmt.Errorf(`%w: a one-word emote`, ErrUnusable)
		}
		if secondPerson.MatchString(text) {
			return Result{}, fmt.Errorf(`%w: an emote addressing the reader`, ErrUnusable)
		}
	}
	return Result{Kind: kind, Text: text, KeyholderOnly: r.KeyholderOnly}, nil
}

// dashes is a dash used as punctuation: a hyphen with a space on either
// side, or two or more hyphens.
var dashes = regexp.MustCompile(`\s+-+\s+|\s*--+\s*`)
