package aicompanion

import (
	"encoding/json"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Decision is what the model returns for one decision. Only Speech acts on
// the world, and only through ordinary say and emote commands. Everything
// else is a proposal that server code validates, bounds and records.
type Decision struct {
	Intent     string             `json:"intent"`
	Speech     []SpeechLine       `json:"speech"`
	Action     ActionProposal     `json:"action"`
	Mood       string             `json:"mood"`
	Memory     MemoryProposal     `json:"memory"`
	Facts      []string           `json:"facts"`
	Opinion    OpinionProposal    `json:"opinion"`
	Promise    PromiseProposal    `json:"promise"`
	Impression ImpressionProposal `json:"impression"`
	LootRule   string             `json:"loot_rule"`
	PlaceTip   string             `json:"place_tip"`
	Goal       GoalProposal       `json:"goal"`
	Autonomy   string             `json:"autonomy"`
	Combat     CombatProposal     `json:"combat"`
	Leave      bool               `json:"leave"`
}

// CombatProposal is the companion's plan for the fight in progress. Local
// reflexes carry it out; the fight never waits for the model.
type CombatProposal struct {
	Stance string `json:"stance"`  // unchanged, fight, protect, hold_back, flee
	Target string `json:"target"`  // e-ref of the enemy to go for, or empty
	FleeAt string `json:"flee_at"` // unchanged, never, badly_hurt, about_to_die
	Style  string `json:"style"`   // unchanged, melee, ranged
	Move   string `json:"move"`    // a special move to try next, none, or unchanged
	// Spell is the [m] ref of a spell to cast next in the fight, and
	// SpellAt who it is for: "owner", "self", an [e] ref, or empty (the
	// enemy she is fighting for a harmful spell, herself otherwise).
	Spell   string `json:"spell"`
	SpellAt string `json:"spell_at"`
}

var (
	combatStances = []string{`unchanged`, `fight`, `protect`, `hold_back`, `flee`}
	combatFleeAt  = []string{`unchanged`, `never`, `badly_hurt`, `about_to_die`}
	combatStyles  = []string{`unchanged`, `melee`, `ranged`}
	// combatMoves are the engine's special moves. Which of them a
	// particular companion can use depends on its body and its gear, not on
	// its skills: a bash needs a shield, a kick needs legs, hamstring needs
	// fangs or claws and no hands. The engine decides whether one would
	// land right now; skills decide how well it goes. A profile can point
	// at any mob template, so the list is deliberately wider than the
	// shipped humanoid can use. The first two are not moves: unchanged
	// keeps whatever move the plan already holds (a plan that says nothing
	// about it is not a change of mind), none drops it.
	combatMoves = []string{`unchanged`, `none`, `taunt`, `bash`, `kick`, `trip`, `grapple`, `hamstring`, `rally`, `warcry`}
)

// ActionProposal is at most one thing the companion does besides talking.
// Ref names a thing from the scene ("t2", "p1", "w1"); To names the
// recipient for give and show. Code validates both against the scene and
// builds the actual mob command; the model never writes a command.
type ActionProposal struct {
	Verb  string `json:"verb"`
	Ref   string `json:"ref"`
	To    string `json:"to"`
	Query string `json:"query"`
}

// ImpressionProposal records how the companion feels about someone present
// or about the current place ("here").
type ImpressionProposal struct {
	Ref     string `json:"ref"`
	Feeling string `json:"feeling"`
	Note    string `json:"note"`
}

type SpeechLine struct {
	Kind string `json:"kind"` // say | emote
	Text string `json:"text"`
}

// MemoryProposal is one thing worth remembering long term. Empty Text means
// nothing is.
type MemoryProposal struct {
	Text       string `json:"text"`
	Importance int    `json:"importance"`
	Emotion    string `json:"emotion"`
}

// OpinionProposal is the change the model thinks this moment warrants. It is
// bounded by boundDelta before anything is applied.
type OpinionProposal struct {
	Trust     int    `json:"trust"`
	Respect   int    `json:"respect"`
	Affection int    `json:"affection"`
	Reason    string `json:"reason"`
}

// PromiseProposal records a promise being made, kept or broken.
type PromiseProposal struct {
	Kind string `json:"kind"` // none, made_by_me, made_by_them, kept, broken
	Text string `json:"text"`
	Ref  int    `json:"ref"` // id of an open promise, for kept/broken
}

const (
	maxSpeechLines = 3
	// A spoken line is capped generously so she can tell a story when one
	// is called for. It is not said in one breath: speakInChunks breaks it
	// at sentence ends into pieces of maxSayChunkRunes and paces them.
	maxSpeechRunes   = 1200
	maxEmoteRunes    = 320
	maxSayChunkRunes = 240
	maxSayChunks     = 8
	maxRememberRunes = 220
	maxIntentRunes   = 200
	maxFactsPerCall  = 2
	maxFactRunes     = 160
)

// moods is the closed set the model may choose from. It is also the enum in
// the response schema.
var moods = []string{
	`calm`, `cheerful`, `amused`, `curious`, `thoughtful`, `affectionate`,
	`wary`, `tense`, `irritated`, `angry`, `sad`, `tired`, `hurt`,
}

// emotions tag memories.
var emotions = []string{
	`neutral`, `joy`, `gratitude`, `affection`, `amusement`, `pride`, `relief`,
	`curiosity`, `sadness`, `fear`, `anger`, `hurt`, `shame`, `disgust`,
}

var promiseKinds = []string{`none`, `made_by_me`, `made_by_them`, `kept`, `broken`}

// actionVerbs is the whole of what the companion can do besides speak.
// look_at and consider are perception: the module answers them from what a
// player would see. The rest become ordinary mob commands.
var actionVerbs = []string{
	`none`, `look_at`, `consider`, `get`, `drop`, `give`, `show`,
	`equip`, `remove`, `eat`, `drink`, `forage`, `search`,
	`go_to`, `explore`, `find_place`, `put`, `sayto`,
	`browse`, `buy`, `sell`, `loot`, `take_from`, `craft`, `attack`,
	`cast`, `rest`, `stand`, `sneak`, `salvage`,
}

var autonomyLevels = []string{`unchanged`, autonomyClose, autonomyNormal, autonomyFree}

var feelings = []string{`none`, `like`, `dislike`, `wary`, `trust`, `distrust`, `neutral`}

var lootRules = []string{`unchanged`, lootAskFirst, lootTakeFreely, lootLeaveIt}

func inSet(set []string, s string) bool {
	for _, v := range set {
		if v == s {
			return true
		}
	}
	return false
}

func isMood(s string) bool { return inSet(moods, s) }

func str(desc string) map[string]any {
	return map[string]any{`type`: `string`, `description`: desc}
}

func integer(desc string) map[string]any {
	return map[string]any{`type`: `integer`, `description`: desc}
}

func object(required []string, props map[string]any) map[string]any {
	return map[string]any{
		`type`:                 `object`,
		`additionalProperties`: false,
		`required`:             required,
		`properties`:           props,
	}
}

// decisionSchema is the strict JSON schema sent with every decision.
// Strict structured outputs require every property to be listed as required
// and additionalProperties false; limits the schema cannot express are
// enforced in sanitizeDecision.
func decisionSchema() map[string]any {
	return object(
		[]string{`intent`, `speech`, `action`, `mood`, `memory`, `facts`, `opinion`, `promise`, `impression`, `loot_rule`, `place_tip`, `goal`, `autonomy`, `combat`, `leave`},
		map[string]any{
			`intent`: str(`One short private sentence: what you mean to do and why. Never shown to anyone.`),
			`speech`: map[string]any{
				`type`:        `array`,
				`description`: `Zero to three things you say or do, in order. Empty means you stay silent.`,
				`items`: object([]string{`kind`, `text`}, map[string]any{
					`kind`: map[string]any{`type`: `string`, `enum`: []string{`say`, `emote`}},
					`text`: str(`What you say, or a short action in the third person without your name.`),
				}),
			},
			`action`: object([]string{`verb`, `ref`, `to`, `query`}, map[string]any{
				`verb`:  map[string]any{`type`: `string`, `enum`: actionVerbs},
				`ref`:   str(`The [ref] the action is about: a thing (t2, p1, w1), a place (r123), a recipe (k1), a spell you know (m1), "owner" for go_to, or an exit name for explore. Empty for none, forage, search, find_place, rest and stand.`),
				`to`:    str(`For give and show: the [ref] of who receives it, or "owner". For put: the [ref] of the container. For cast: the [ref] of who it is aimed at, or "owner", or empty for yourself. Otherwise empty.`),
				`query`: str(`For find_place: what you are trying to remember the way to. For go_to: what you are going there to do, in a few words ("buy a shirt"). For sayto: the words you say. For buy: how many (a number). For take_from: which item in the container. Otherwise empty.`),
			}),
			`place_tip`: str(`Something you were just told about where a place is or how to reach it, in a few words. Empty otherwise.`),
			`goal`: object([]string{`action`, `text`, `level`, `ref`}, map[string]any{
				`action`: map[string]any{`type`: `string`, `enum`: goalActions},
				`text`:   str(`For add: the goal in a few words.`),
				`level`:  map[string]any{`type`: `string`, `enum`: []string{`medium`, `long`}},
				`ref`:    str(`For done or drop: the goal's [ref], e.g. g4. Otherwise empty.`),
			}),
			`combat`: object([]string{`stance`, `target`, `flee_at`, `style`, `move`, `spell`, `spell_at`}, map[string]any{
				`stance`:   map[string]any{`type`: `string`, `enum`: combatStances},
				`target`:   str(`During a fight: the [e] ref of the enemy to go for. Otherwise empty.`),
				`flee_at`:  map[string]any{`type`: `string`, `enum`: combatFleeAt},
				`style`:    map[string]any{`type`: `string`, `enum`: combatStyles},
				`move`:     map[string]any{`type`: `string`, `enum`: combatMoves},
				`spell`:    str(`During a fight: the [m] ref of a spell you know to cast next. Otherwise empty.`),
				`spell_at`: str(`Who that spell is for: "owner" for your companion, "self", or the [e] ref of an enemy. Empty means the enemy you are fighting for a spell that harms, and yourself for any other.`),
			}),
			`leave`: map[string]any{
				`type`:        `boolean`,
				`description`: `True only if you are parting ways for good: your companion has clearly told you to go, or you truly cannot bear them any longer. Say your goodbye in speech in the same reply.`,
			},
			`autonomy`: map[string]any{
				`type`:        `string`,
				`enum`:        autonomyLevels,
				`description`: `Only when your companion has just told you how far to roam: close (stay by them), normal, or free. Otherwise unchanged.`,
			},
			`impression`: object([]string{`ref`, `feeling`, `note`}, map[string]any{
				`ref`:     str(`The [ref] of a person or creature present, "here" for this place, or empty for none.`),
				`feeling`: map[string]any{`type`: `string`, `enum`: feelings},
				`note`:    str(`A few words on why. Empty if nothing new.`),
			}),
			`loot_rule`: map[string]any{
				`type`:        `string`,
				`enum`:        lootRules,
				`description`: `Only when you and your companion have just agreed how you handle things you find; otherwise unchanged.`,
			},
			`mood`: map[string]any{`type`: `string`, `enum`: moods},
			`memory`: object([]string{`text`, `importance`, `emotion`}, map[string]any{
				`text`:       str(`One thing from this moment worth remembering for weeks, in your own words. Empty string if nothing is.`),
				`importance`: integer(`1 trivial, 4 notable, 7 significant, 10 unforgettable.`),
				`emotion`:    map[string]any{`type`: `string`, `enum`: emotions},
			}),
			`facts`: map[string]any{
				`type`:        `array`,
				`description`: `New facts you just learned about the person you travel with (who they are, their past, likes, fears, plans). Usually empty.`,
				`items`:       map[string]any{`type`: `string`},
			},
			`opinion`: object([]string{`trust`, `respect`, `affection`, `reason`}, map[string]any{
				`trust`:     integer(`How much this moment changes your trust in them. Usually 0. Small numbers only.`),
				`respect`:   integer(`How much this moment changes your respect for them. Usually 0.`),
				`affection`: integer(`How much this moment changes how much you like them. Usually 0.`),
				`reason`:    str(`Why, in a few words. Empty if nothing changed.`),
			}),
			`promise`: object([]string{`kind`, `text`, `ref`}, map[string]any{
				`kind`: map[string]any{`type`: `string`, `enum`: promiseKinds},
				`text`: str(`The promise in a few words, for made_by_me or made_by_them. Otherwise empty.`),
				`ref`:  integer(`For kept or broken: the number of the open promise. Otherwise 0.`),
			}),
		},
	)
}

// parseDecision decodes the model's JSON content.
func parseDecision(content string) (Decision, error) {
	var d Decision
	err := parseJSONContent(content, &d)
	return d, err
}

// breaksCharacter catches lines that would reveal the companion is not a
// person of Gaius. Such lines are dropped, never spoken.
var characterBreakers = []string{
	`language model`, `as an ai`, `an ai model`, `artificial intelligence`,
	`chatgpt`, `openai`, `system prompt`, `my instructions`, `my programming`,
	`i am a bot`, `i'm a bot`, `i am an ai`, `i'm an ai`, `json`,
}

func breaksCharacter(s string) bool {
	l := strings.ToLower(s)
	for _, b := range characterBreakers {
		if strings.Contains(l, b) {
			return true
		}
	}
	return false
}

// cleanText makes model text safe to hand to a single mob command: no
// control characters or newlines, no command separators, bounded length.
// ANSI tag escaping happens at execution time (util.EscapeAnsiTags).
func cleanText(s string, maxRunes int) string {
	var b strings.Builder
	lastSpace := false
	for _, r := range s {
		switch {
		case r == ';':
			r = ','
		case r == '\n' || r == '\r' || r == '\t':
			r = ' '
		case r == utf8.RuneError || unicode.IsControl(r):
			continue
		}
		if r == ' ' {
			if lastSpace {
				continue
			}
			lastSpace = true
		} else {
			lastSpace = false
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if utf8.RuneCountInString(out) > maxRunes {
		runes := []rune(out)
		out = string(runes[:maxRunes])
		if i := strings.LastIndexAny(out, ` .,!?`); i > maxRunes/2 {
			out = out[:i+1]
		}
		out = strings.TrimSpace(out)
	}
	return out
}

// sanitizeDecision applies every rule the schema cannot express. It returns
// the decision that is safe to act on. prevMood is kept when the model's
// mood is invalid. Opinion deltas are passed through untouched here; they
// are bounded by boundDelta, which knows what triggered the decision.
func sanitizeDecision(d Decision, companionName string, prevMood string) Decision {
	out := Decision{
		Intent:  cleanText(d.Intent, maxIntentRunes),
		Mood:    strings.ToLower(strings.TrimSpace(d.Mood)),
		Opinion: d.Opinion,
		Leave:   d.Leave,
	}
	if !isMood(out.Mood) {
		out.Mood = prevMood
	}
	out.Opinion.Reason = cleanText(d.Opinion.Reason, maxIntentRunes)

	// Memory.
	out.Memory.Text = cleanText(d.Memory.Text, maxRememberRunes)
	if breaksCharacter(out.Memory.Text) {
		out.Memory.Text = ``
	}
	out.Memory.Importance = clampInt(d.Memory.Importance, 1, 10)
	out.Memory.Emotion = strings.ToLower(strings.TrimSpace(d.Memory.Emotion))
	if !inSet(emotions, out.Memory.Emotion) {
		out.Memory.Emotion = `neutral`
	}

	// Facts.
	for _, f := range d.Facts {
		if len(out.Facts) >= maxFactsPerCall {
			break
		}
		f = cleanText(f, maxFactRunes)
		if f == `` || breaksCharacter(f) {
			continue
		}
		out.Facts = append(out.Facts, f)
	}

	// Promise.
	out.Promise.Kind = strings.ToLower(strings.TrimSpace(d.Promise.Kind))
	if !inSet(promiseKinds, out.Promise.Kind) {
		out.Promise.Kind = `none`
	}
	out.Promise.Text = cleanText(d.Promise.Text, maxFactRunes)
	out.Promise.Ref = d.Promise.Ref
	if (out.Promise.Kind == `made_by_me` || out.Promise.Kind == `made_by_them`) && out.Promise.Text == `` {
		out.Promise.Kind = `none`
	}

	// Action. The ref is only shape-checked here; runtime validates it
	// against the scene the decision was made from.
	out.Action.Verb = strings.ToLower(strings.TrimSpace(d.Action.Verb))
	if !inSet(actionVerbs, out.Action.Verb) {
		out.Action.Verb = `none`
	}
	out.Action.Ref = strings.ToLower(strings.TrimSpace(d.Action.Ref))
	out.Action.To = strings.ToLower(strings.TrimSpace(d.Action.To))
	out.Action.Query = cleanText(d.Action.Query, maxEmoteRunes)
	if out.Action.Verb == `sayto` {
		out.Action.Query = strings.TrimSpace(strings.Trim(out.Action.Query, `"`))
		if out.Action.Query == `` || breaksCharacter(out.Action.Query) {
			out.Action.Verb = `none`
		}
	}

	// Goal change.
	out.Goal.Action = strings.ToLower(strings.TrimSpace(d.Goal.Action))
	if !inSet(goalActions, out.Goal.Action) {
		out.Goal.Action = `none`
	}
	out.Goal.Text = cleanText(d.Goal.Text, maxFactRunes)
	out.Goal.Level = strings.ToLower(strings.TrimSpace(d.Goal.Level))
	out.Goal.Ref = strings.ToLower(strings.TrimSpace(d.Goal.Ref))
	if out.Goal.Action == `add` && (out.Goal.Text == `` || breaksCharacter(out.Goal.Text)) {
		out.Goal.Action = `none`
	}

	// Combat plan.
	out.Combat.Stance = strings.ToLower(strings.TrimSpace(d.Combat.Stance))
	if !inSet(combatStances, out.Combat.Stance) {
		out.Combat.Stance = `unchanged`
	}
	out.Combat.FleeAt = strings.ToLower(strings.TrimSpace(d.Combat.FleeAt))
	if !inSet(combatFleeAt, out.Combat.FleeAt) {
		out.Combat.FleeAt = `unchanged`
	}
	out.Combat.Style = strings.ToLower(strings.TrimSpace(d.Combat.Style))
	if !inSet(combatStyles, out.Combat.Style) {
		out.Combat.Style = `unchanged`
	}
	out.Combat.Target = strings.ToLower(strings.TrimSpace(d.Combat.Target))
	out.Combat.Move = strings.ToLower(strings.TrimSpace(d.Combat.Move))
	if !inSet(combatMoves, out.Combat.Move) {
		out.Combat.Move = `unchanged`
	}
	out.Combat.Spell = strings.ToLower(strings.TrimSpace(d.Combat.Spell))
	out.Combat.SpellAt = strings.ToLower(strings.TrimSpace(d.Combat.SpellAt))
	if out.Combat.Spell == `` {
		out.Combat.SpellAt = ``
	}

	// Autonomy.
	out.Autonomy = strings.ToLower(strings.TrimSpace(d.Autonomy))
	if !inSet(autonomyLevels, out.Autonomy) {
		out.Autonomy = `unchanged`
	}

	// Hearsay about places.
	out.PlaceTip = cleanText(d.PlaceTip, maxFactRunes)
	if breaksCharacter(out.PlaceTip) {
		out.PlaceTip = ``
	}

	// Impression.
	out.Impression.Ref = strings.ToLower(strings.TrimSpace(d.Impression.Ref))
	out.Impression.Feeling = strings.ToLower(strings.TrimSpace(d.Impression.Feeling))
	if !inSet(feelings, out.Impression.Feeling) || out.Impression.Feeling == `none` {
		out.Impression = ImpressionProposal{}
	} else {
		out.Impression.Note = cleanText(d.Impression.Note, maxFactRunes)
		if breaksCharacter(out.Impression.Note) {
			out.Impression.Note = ``
		}
	}

	// Loot arrangement.
	out.LootRule = strings.ToLower(strings.TrimSpace(d.LootRule))
	if !inSet(lootRules, out.LootRule) {
		out.LootRule = `unchanged`
	}

	// Speech.
	names := namePrefixes(companionName)
	for _, s := range d.Speech {
		if len(out.Speech) >= maxSpeechLines {
			break
		}
		kind := strings.ToLower(strings.TrimSpace(s.Kind))
		if kind != `say` && kind != `emote` {
			continue
		}
		limit := maxSpeechRunes
		if kind == `emote` {
			limit = maxEmoteRunes
		}
		text := cleanText(s.Text, limit)

		if kind == `say` {
			text = strings.TrimSpace(strings.Trim(text, `"`))
		} else {
			// The game prepends the name to an emote, so strip it if the
			// model wrote it, and drop a stray leading "emote".
			lower := strings.ToLower(text)
			if strings.HasPrefix(lower, `emote `) {
				text = strings.TrimSpace(text[len(`emote `):])
				lower = strings.ToLower(text)
			}
			for _, n := range names {
				if strings.HasPrefix(lower, n+` `) {
					text = strings.TrimSpace(text[len(n)+1:])
					break
				}
			}
		}

		if text == `` || breaksCharacter(text) {
			continue
		}
		out.Speech = append(out.Speech, SpeechLine{Kind: kind, Text: text})
	}
	return out
}

// namePrefixes returns lower-cased forms of the name an emote might start
// with, longest first.
func namePrefixes(name string) []string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == `` {
		return nil
	}
	out := []string{name}
	if f := strings.Fields(name); len(f) > 1 {
		out = append(out, f[0])
	}
	return out
}

// parseJSONContent decodes model content into any struct, tolerating a
// fenced block from providers that ignore response_format.
func parseJSONContent(content string, out any) error {
	content = strings.TrimSpace(content)
	if content == `` {
		return errors.New(`empty model content`)
	}
	if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(strings.TrimSpace(content), "```")
	}
	return json.Unmarshal([]byte(content), out)
}
