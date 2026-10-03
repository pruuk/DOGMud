package rifts

// Generated rooms. Now and then, as a room of one of a profile's
// Generation.Pools is built, a model writes a brand new room of that pool in
// the background, on the own key of a player in the run (modules/rifts
// installs the Generator, a lively feature: never the server's key). The
// player never waits for it: the room being built uses the bank as it is.
// A reply that passes every check here is saved beside the authored rooms
// (rooms/<profile>/gen-<pool>-<slug>.yaml, loaded at every boot like the
// rest) and joins the pool at once, so the pools grow over time.
//
// The engine owns everything that decides what may enter the bank: the
// request (the canon, the pool's guide, examples from the bank, the titles
// already used, the exact door count, the nouns the game places itself),
// the strict reply schema, and the rules a reply must pass (BuildGenerated).
// The module only writes the prompt and makes the call.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/casing"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// RoomSchemaName names the reply's strict JSON schema. It must be listed in
// modules/aicompanion/relayweb/relay.js LIVELY_SCHEMAS (a test holds it).
const RoomSchemaName = `rift_room`

// ErrUnusableRoom wraps a reply that parsed but breaks the bank's rules.
var ErrUnusableRoom = errors.New(`unusable rift room`)

// Generator is what modules/rifts installs.
type Generator interface {
	// Chance is the percent of rooms built (of a pool the profile lets a
	// model write for) that ask for a new room of that pool. 0 is off.
	Chance() int
	// MaxPerPool caps the generated rooms one pool of a profile may hold;
	// 0 is no cap.
	MaxPerPool() int
	// Reserve takes userId's turn when their key may write a room now.
	// Under the mud lock.
	Reserve(userId int) bool
	// Generate writes the room on userId's key, gives their turn back, and
	// returns it built by BuildGenerated and moderated. Off the mud lock.
	Generate(ctx context.Context, userId int, req GenRequest) (*Template, error)
}

var generator atomic.Pointer[Generator]

// SetGenerator installs the generator (the module, at load); nil removes it.
func SetGenerator(g Generator) {
	if g == nil {
		generator.Store(nil)
		return
	}
	generator.Store(&g)
}

func installedGenerator() Generator {
	if g := generator.Load(); g != nil {
		return *g
	}
	return nil
}

// WordRange bounds a text's length in words.
type WordRange struct{ Min, Max int }

// descriptionWords is how long a generated room's description may be, by
// pool: a little looser than the authoring brief, never far from it.
var descriptionWords = map[Pool]WordRange{
	PoolPassage: {90, 200},
	PoolFeature: {120, 260},
	PoolPuzzle:  {120, 260},
	PoolMonster: {100, 220},
	PoolBoss:    {120, 260},
	PoolExit:    {120, 260},
}

// The other bounds on a generated room.
var (
	nounLookWords   = WordRange{15, 120}
	doorWords       = WordRange{6, 50}
	idleWords       = WordRange{3, 30}
	resultWords     = WordRange{4, 80} // a puzzle's solved/wrong, a trap's triggered/avoided
	nounCount       = WordRange{3, 6}
	idleCount       = WordRange{1, 2}
	trapDifficulty  = WordRange{90, 140}
	examplesPerCall = 3
	seedsPerCall    = 3
	maxPromptTitles = 300
)

// GenRequest is everything a model is given to write one room: built under
// the mud lock (genRequest), read off it. Examples and UsedTitles are
// snapshots, so nothing here points into the live bank.
type GenRequest struct {
	ProfileId   string
	ProfileName string
	Pool        Pool
	Setting     string // the profile's canon
	Guide       string // what a room of this pool is
	Doors       int    // exactly this many doors
	Words       WordRange
	ExitName    string // the way out an exit room's game adds (F)

	// Nouns the game places in rooms itself (lore, rubble, writings, ore,
	// the portal): a room must never name one.
	ReservedNouns []string
	Effects       []string // effect words a trap or a wrong answer may choose
	Examples      []string // a few of the pool's rooms, in the reply's shape (JSON)
	UsedTitles    []string // every title the pool holds
	PromptTitles  []string // the ones shown to the model (the latest maxPromptTitles)
	Openings      []string // the opening words of every description in the pool
	Seeds         []string // a few words to start from, for variety

	effectIds map[string]int
	profile   *Profile
}

// GenRequest is the request a model is given for a new room of pool (for
// tests and tools; the engine builds its own as rooms are built). Under the
// mud lock.
func (p *Profile) GenRequest(pool Pool) GenRequest { return p.genRequest(pool) }

// genRequest is the request for a new room of pool. Under the mud lock.
func (p *Profile) genRequest(pool Pool) GenRequest {
	req := GenRequest{
		ProfileId:   p.Id,
		ProfileName: p.Name,
		Pool:        pool,
		Setting:     strings.TrimSpace(p.Generation.Setting),
		Guide:       strings.TrimSpace(p.Generation.Guides[pool]),
		Doors:       p.Doors[pool].Max,
		Words:       descriptionWords[pool],
		effectIds:   map[string]int{},
		profile:     p.frozen(),
	}
	if pool == PoolExit {
		req.ExitName = p.ExitName
	}
	req.ReservedNouns = p.reservedNouns()
	for word, id := range p.Generation.Effects {
		req.Effects = append(req.Effects, word)
		req.effectIds[word] = id
	}
	sort.Strings(req.Effects)

	all := p.templates[pool]
	for _, t := range all {
		req.UsedTitles = append(req.UsedTitles, t.Title)
		req.Openings = append(req.Openings, opening(t.Description))
	}
	// Every title is checked (BuildGenerated); the prompt shows the most
	// recent few hundred, so it does not grow without end.
	req.PromptTitles = req.UsedTitles[max(0, len(req.UsedTitles)-maxPromptTitles):]
	// Examples: authored rooms first (the house style), then any.
	order := make([]*Template, 0, len(all))
	for _, t := range all {
		if t.Source != `generated` {
			order = append(order, t)
		}
	}
	shuffle(order)
	var gen []*Template
	for _, t := range all {
		if t.Source == `generated` {
			gen = append(gen, t)
		}
	}
	shuffle(gen)
	order = append(order, gen...)
	for i := 0; i < len(order) && i < examplesPerCall; i++ {
		req.Examples = append(req.Examples, exampleJSON(order[i], p.Generation.Effects))
	}
	seeds := append([]string{}, p.Generation.Seeds...)
	shuffle(seeds)
	req.Seeds = seeds[:min(seedsPerCall, len(seeds))]
	return req
}

func shuffle[T any](s []T) {
	for i := len(s) - 1; i > 0; i-- {
		j := rng(i + 1)
		s[i], s[j] = s[j], s[i]
	}
}

// frozen is a copy of the profile without its bank, for validating a reply
// off the mud lock: a template's validation reads only the profile's fixed
// settings, never the pools.
func (p *Profile) frozen() *Profile {
	cp := *p
	cp.templates = nil
	return &cp
}

// reservedNouns are the nouns the game places in a room by itself.
func (p *Profile) reservedNouns() []string {
	set := map[string]bool{p.ExitName: true, p.PortalExit: true}
	memoryNoun := ``
	if p.Memory != nil {
		memoryNoun = p.Memory.Noun
	}
	for _, n := range []string{p.Lore.Noun, p.Rubble.Noun, p.DefaultOreNoun, memoryNoun} {
		if n != `` {
			set[n] = true
		}
	}
	for _, w := range p.Writings {
		set[w.Noun] = true
	}
	out := make([]string, 0, len(set))
	for n := range set {
		if n != `` {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// ---- the reply ----

// RoomReply is a model's room, in the schema's shape.
type RoomReply struct {
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	Nouns        []NounReply     `json:"nouns"`
	IdleMessages []string        `json:"idle_messages"`
	Doors        []DoorReply     `json:"doors"`
	Puzzle       *PuzzleReply    `json:"puzzle,omitempty"`
	Trap         *TrapReply      `json:"trap,omitempty"`
	Encounter    *EncounterReply `json:"encounter,omitempty"`
}

// NounReply is one thing a player can look at.
type NounReply struct {
	Name string `json:"name"`
	Look string `json:"look"`
}

// DoorReply is one way on.
type DoorReply struct {
	Exit        string `json:"exit"`
	Description string `json:"description"`
}

// PuzzleReply is a C room's puzzle: always a lens table (memory.go), which
// the game places and lays out itself, and which always gives its solver a
// key. The model chooses only the door it holds shut and what the solver
// sees.
type PuzzleReply struct {
	SealedDoor string `json:"sealed_door"`
	Solved     string `json:"solved"`
}

// TrapReply is a C room's trap.
type TrapReply struct {
	Difficulty int    `json:"difficulty"`
	Effect     string `json:"effect"`
	Triggered  string `json:"triggered"`
	Avoided    string `json:"avoided"`
}

// EncounterReply is a D room's encounter.
type EncounterReply struct {
	Tier string `json:"tier"`
}

// ParseRoomReply reads a model's reply content. An error is the schema
// ignored.
func ParseRoomReply(content string) (RoomReply, error) {
	var r RoomReply
	dec := json.NewDecoder(strings.NewReader(strings.TrimSpace(content)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return RoomReply{}, fmt.Errorf(`reply is not the schema's JSON: %w`, err)
	}
	if dec.More() {
		return RoomReply{}, fmt.Errorf(`reply has more than the schema's JSON`)
	}
	return r, nil
}

// RoomSchema is the strict JSON schema of a reply for req's pool: a C room
// has a puzzle and a trap (either may be null), a D room an encounter, and
// the other pools neither.
func RoomSchema(req GenRequest) map[string]any {
	str := func(desc string) map[string]any { return map[string]any{`type`: `string`, `description`: desc} }
	strs := func(desc string) map[string]any {
		return map[string]any{`type`: `array`, `items`: map[string]any{`type`: `string`}, `description`: desc}
	}
	enum := func(desc string, vals ...string) map[string]any {
		return map[string]any{`type`: `string`, `enum`: vals, `description`: desc}
	}
	obj := func(props map[string]any, nullable bool) map[string]any {
		req := make([]string, 0, len(props))
		for k := range props {
			req = append(req, k)
		}
		sort.Strings(req)
		o := map[string]any{`type`: `object`, `properties`: props, `required`: req, `additionalProperties`: false}
		if nullable {
			o[`type`] = []string{`object`, `null`}
		}
		return o
	}
	props := map[string]any{
		`title`:       str(`the room's name in Title Case, new and unlike every used title`),
		`description`: str(fmt.Sprintf(`one paragraph of %d to %d words`, req.Words.Min, req.Words.Max)),
		`nouns`: map[string]any{`type`: `array`, `description`: fmt.Sprintf(`%d to %d things to look at`, nounCount.Min, nounCount.Max),
			`items`: obj(map[string]any{
				`name`: str(`one lowercase word a player types to look at it`),
				`look`: str(fmt.Sprintf(`what a closer look shows, %d to %d words`, nounLookWords.Min, nounLookWords.Max)),
			}, false)},
		`idle_messages`: strs(`one or two short present-tense lines of ambience`),
		`doors`: map[string]any{`type`: `array`, `description`: fmt.Sprintf(`exactly %d ways on`, req.Doors),
			`items`: obj(map[string]any{
				`exit`:        str(`one lowercase word, letters only`),
				`description`: str(`what is seen of that way on`),
			}, false)},
	}
	effects := req.Effects
	if len(effects) == 0 {
		effects = []string{`none`}
	}
	switch req.Pool {
	case PoolPuzzle:
		props[`puzzle`] = obj(map[string]any{
			`sealed_door`: str(`the exit of the door the lens table holds shut`),
			`solved`:      str(`what the solver sees as the last lenses lock and the sealed way opens`),
		}, true)
		props[`trap`] = obj(map[string]any{
			`difficulty`: map[string]any{`type`: `integer`, `description`: fmt.Sprintf(`%d easy to %d hard`, trapDifficulty.Min, trapDifficulty.Max)},
			`effect`:     enum(`what the trap does to the one caught`, effects...),
			`triggered`:  str(`what happens to the one caught`),
			`avoided`:    str(`what the wary one notices and steps around`),
		}, true)
	case PoolMonster:
		props[`encounter`] = obj(map[string]any{
			`tier`: enum(`trash: a few lesser creatures; elite: one strong one`, `trash`, `elite`),
		}, false)
	}
	return obj(props, false)
}

// ---- the rules ----

var (
	slugRE    = regexp.MustCompile(`[^a-z0-9]+`)
	dashRunRE = regexp.MustCompile(`\s+-+\s+|\s*--+\s*`)
)

// plainProse is s as plain one-line ASCII prose, or an error: markup is
// stripped, typography folded, dashes become commas.
func plainProse(s string) (string, error) {
	s = baubles.PlainText(s)
	s = dashRunRE.ReplaceAllString(s, `, `)
	s = strings.Join(strings.Fields(s), ` `)
	for _, c := range s {
		if c < 0x20 || c > 0x7e || c == '<' || c == '>' || c == '`' || c == '\\' || c == '{' || c == '}' || c == '*' || c == '_' || c == '#' {
			return ``, fmt.Errorf(`a character outside plain prose (%q)`, c)
		}
	}
	return s, nil
}

func words(s string) int { return len(strings.Fields(s)) }

// opening is a description's first words, lowercased: two rooms that open
// alike are too alike.
func opening(s string) string {
	f := strings.Fields(strings.ToLower(s))
	return strings.Join(f[:min(8, len(f))], ` `)
}

// BuildGenerated turns a reply into a template of req's pool, or an error
// wrapping ErrUnusableRoom: every text plain prose of the pool's length,
// the exact door count, nouns that are single words the game does not
// place itself and that the room's text mentions, a title and opening no
// room of the pool already has, a C room with a puzzle or a trap (a puzzle
// that never rewards a key), a D room with an encounter, and then the
// loader's own validation. It touches no live state.
func BuildGenerated(req GenRequest, r RoomReply) (*Template, error) {
	bad := func(format string, a ...any) (*Template, error) {
		return nil, fmt.Errorf(`%w: %s`, ErrUnusableRoom, fmt.Sprintf(format, a...))
	}
	text := func(field, s string, wr WordRange) (string, error) {
		s, err := plainProse(s)
		if err != nil {
			return ``, fmt.Errorf(`%s: %v`, field, err)
		}
		if n := words(s); n < wr.Min || n > wr.Max {
			return ``, fmt.Errorf(`%s: %d words, want %d to %d`, field, n, wr.Min, wr.Max)
		}
		return s, nil
	}
	var err error
	t := &Template{Pool: req.Pool, Source: `generated`, Nouns: map[string]string{}}

	title, err := plainProse(r.Title)
	if err != nil || words(title) < 1 || words(title) > 7 || len(title) > 60 {
		return bad(`title %q`, r.Title)
	}
	t.Title = casing.Title(strings.Trim(title, ` .,!?'"`))
	for _, used := range req.UsedTitles {
		if strings.EqualFold(used, t.Title) {
			return bad(`title %q is already used`, t.Title)
		}
	}
	if t.Description, err = text(`description`, r.Description, req.Words); err != nil {
		return bad(`%v`, err)
	}
	open := opening(t.Description)
	for _, o := range req.Openings {
		if o == open {
			return bad(`the description opens like another room's`)
		}
	}

	reserved := map[string]bool{}
	for _, n := range req.ReservedNouns {
		reserved[n] = true
	}

	// Doors.
	if len(r.Doors) != req.Doors {
		return bad(`%d doors, want exactly %d`, len(r.Doors), req.Doors)
	}
	exits := map[string]bool{}
	for i, d := range r.Doors {
		exit := strings.ToLower(strings.TrimSpace(d.Exit))
		if !goodName(exit) || reserved[exit] || exits[exit] {
			return bad(`door exit %q`, d.Exit)
		}
		if cmd := shadowsCommand(exit); cmd != `` {
			return bad(`door exit %q would take over the command %q`, d.Exit, cmd)
		}
		exits[exit] = true
		desc, err := text(fmt.Sprintf(`door %d`, i+1), d.Description, doorWords)
		if err != nil {
			return bad(`%v`, err)
		}
		t.Doors = append(t.Doors, DoorSpec{Exit: exit, Description: desc})
	}

	// Nouns.
	if len(r.Nouns) < nounCount.Min || len(r.Nouns) > nounCount.Max {
		return bad(`%d nouns, want %d to %d`, len(r.Nouns), nounCount.Min, nounCount.Max)
	}
	for _, n := range r.Nouns {
		name := strings.ToLower(strings.TrimSpace(n.Name))
		if !goodName(name) || reserved[name] || exits[name] {
			return bad(`noun %q`, n.Name)
		}
		if _, dup := t.Nouns[name]; dup {
			return bad(`noun %q twice`, name)
		}
		look, err := text(`noun `+name, n.Look, nounLookWords)
		if err != nil {
			return bad(`%v`, err)
		}
		t.Nouns[name] = look
	}
	// Every noun is mentioned somewhere a reader will see it first: the
	// description, or another thing's look.
	for name := range t.Nouns {
		found := mentions(t.Description, name)
		for other, look := range t.Nouns {
			found = found || (other != name && mentions(look, name))
		}
		if !found {
			return bad(`noun %q is never mentioned`, name)
		}
	}

	// Ambience.
	if len(r.IdleMessages) < idleCount.Min || len(r.IdleMessages) > idleCount.Max {
		return bad(`%d idle messages, want %d to %d`, len(r.IdleMessages), idleCount.Min, idleCount.Max)
	}
	for i, m := range r.IdleMessages {
		line, err := text(fmt.Sprintf(`idle message %d`, i+1), m, idleWords)
		if err != nil {
			return bad(`%v`, err)
		}
		t.IdleMessages = append(t.IdleMessages, line)
	}

	// The pool's own parts.
	switch req.Pool {
	case PoolPuzzle:
		if r.Puzzle == nil && r.Trap == nil {
			return bad(`a puzzle room needs a puzzle, a trap, or both`)
		}
		if r.Puzzle != nil {
			if t.Puzzle, err = req.buildPuzzle(r.Puzzle, t, text); err != nil {
				return bad(`puzzle: %v`, err)
			}
			if !strings.Contains(strings.ToLower(t.Description+` `+doorText(t, t.Puzzle.SealedDoor)), `seal`) {
				return bad(`the room never says a way out is sealed`)
			}
		}
		if r.Trap != nil {
			if t.Trap, err = req.buildTrap(r.Trap, text); err != nil {
				return bad(`trap: %v`, err)
			}
		}
	case PoolMonster:
		if r.Encounter == nil {
			return bad(`a monster room needs an encounter`)
		}
		switch r.Encounter.Tier {
		case `trash`:
			t.Encounter = &EncounterSpec{Tier: `trash`, Count: IntRange{Min: 2, Max: 3}}
		case `elite`:
			t.Encounter = &EncounterSpec{Tier: `elite`, Count: IntRange{Min: 1, Max: 1}}
		default:
			return bad(`encounter tier %q`, r.Encounter.Tier)
		}
	case PoolExit:
		if req.ExitName != `` && !strings.Contains(strings.ToLower(t.Description), req.ExitName) {
			return bad(`an exit room's description must name the %s`, req.ExitName)
		}
	}

	t.Id = generatedId(req.Pool, t.Title)
	if req.profile != nil {
		if err := t.validate(req.profile, false); err != nil {
			return bad(`%v`, err)
		}
	}
	return t, nil
}

// mentions reports whether text names noun, or its singular (a "dishes"
// noun is named by "a shallow dish"), as the start of a word: "arch" is
// named by "arches" and "archway", not by "search".
func mentions(text, noun string) bool {
	text = strings.ToLower(text)
	for _, form := range []string{noun, strings.TrimSuffix(noun, `s`), strings.TrimSuffix(noun, `es`)} {
		if len(form) >= 3 && regexp.MustCompile(`\b`+regexp.QuoteMeta(form)).MatchString(text) {
			return true
		}
	}
	return false
}

// commandWords are words a door or noun must never be named: directions
// and the commonest commands, which a player typing them means otherwise.
var commandWords = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`north south east west up down northeast northwest southeast southwest ` +
		`look get take give put go enter exit leave out in quit say tell kill attack flee sneak hide ` +
		`search touch answer open close lock unlock read use wear equip remove inventory help who all self me here`) {
		commandWords[w] = true
	}
}

// goodName is a door or noun name a player can type: one lowercase word of
// 3 to 16 letters that is no direction or common command.
func goodName(s string) bool {
	return exitNameRE.MatchString(s) && len(s) >= 3 && len(s) <= 16 && !commandWords[s]
}

// commandNames lists every command a player can type (modules/rifts sets it
// from the user command registry; the internal package cannot import that).
var commandNames func() []string

// SetCommandNames sets where the player command names come from.
func SetCommandNames(fn func() []string) { commandNames = fn }

// shadowsCommand reports the command or emote a door named exit would take
// over, or "". A typed word is tried as an exit, by prefix, before any
// command: a door named "drop" walks a player who types "drop sword", and
// one named "gorge" walks a player who types "go stair".
func shadowsCommand(exit string) string {
	var words []string
	if commandNames != nil {
		words = commandNames()
	}
	for w := range actions.EmoteAliases {
		words = append(words, w)
	}
	for w := range commandWords {
		words = append(words, w)
	}
	for _, w := range words {
		if len(w) >= 2 && !strings.ContainsAny(w, " ") && strings.HasPrefix(exit, w) {
			return w
		}
	}
	return ``
}

func doorText(t *Template, exit string) string {
	for _, d := range t.Doors {
		if d.Exit == exit {
			return d.Description
		}
	}
	return ``
}

func (req GenRequest) effect(word string) (int, error) {
	id, ok := req.effectIds[word]
	if !ok {
		return 0, fmt.Errorf(`unknown effect %q`, word)
	}
	return id, nil
}

func (req GenRequest) buildPuzzle(r *PuzzleReply, t *Template, text func(string, string, WordRange) (string, error)) (*PuzzleSpec, error) {
	if req.profile == nil || req.profile.Memory == nil {
		return nil, fmt.Errorf(`this rift has no lens table`)
	}
	ps := &PuzzleSpec{Kind: `memory`, SealedDoor: strings.ToLower(strings.TrimSpace(r.SealedDoor)), Hard: true, Reward: `key`}
	if doorText(t, ps.SealedDoor) == `` {
		return nil, fmt.Errorf(`sealed door %q is not one of the room's doors`, r.SealedDoor)
	}
	var err error
	if ps.Solved, err = text(`solved`, r.Solved, resultWords); err != nil {
		return nil, err
	}
	return ps, nil
}

func (req GenRequest) buildTrap(r *TrapReply, text func(string, string, WordRange) (string, error)) (*TrapSpec, error) {
	id, err := req.effect(r.Effect)
	if err != nil {
		return nil, err
	}
	ts := &TrapSpec{Difficulty: min(max(r.Difficulty, trapDifficulty.Min), trapDifficulty.Max), ConditionIds: []int{id}}
	if ts.Triggered, err = text(`triggered`, r.Triggered, resultWords); err != nil {
		return nil, err
	}
	if ts.Avoided, err = text(`avoided`, r.Avoided, resultWords); err != nil {
		return nil, err
	}
	return ts, nil
}

// generatedId is a generated room's id before any clash is resolved.
func generatedId(pool Pool, title string) string {
	slug := strings.Trim(slugRE.ReplaceAllString(strings.ToLower(title), `-`), `-`)
	if len(slug) > 48 {
		slug = strings.TrimRight(slug[:48], `-`)
	}
	return `gen-` + strings.ToLower(string(pool)) + `-` + slug
}

// exampleJSON renders a template in the reply's shape, for a prompt.
func exampleJSON(t *Template, effects map[string]int) string {
	word := func(ids []int) string {
		for _, id := range ids {
			for w, eid := range effects {
				if eid == id {
					return w
				}
			}
		}
		return ``
	}
	r := RoomReply{Title: t.Title, Description: strings.TrimSpace(t.Description), IdleMessages: t.IdleMessages}
	names := make([]string, 0, len(t.Nouns))
	for n := range t.Nouns {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		r.Nouns = append(r.Nouns, NounReply{Name: n, Look: strings.TrimSpace(t.Nouns[n])})
	}
	for _, d := range t.Doors {
		r.Doors = append(r.Doors, DoorReply{Exit: d.Exit, Description: strings.TrimSpace(d.Description)})
	}
	if ps := t.Puzzle; ps != nil {
		r.Puzzle = &PuzzleReply{SealedDoor: ps.SealedDoor, Solved: strings.TrimSpace(ps.Solved)}
	}
	if ts := t.Trap; ts != nil {
		r.Trap = &TrapReply{Difficulty: ts.Difficulty, Effect: word(ts.ConditionIds), Triggered: strings.TrimSpace(ts.Triggered), Avoided: strings.TrimSpace(ts.Avoided)}
	}
	if t.Encounter != nil {
		r.Encounter = &EncounterReply{Tier: t.Encounter.Tier}
	}
	var b []byte
	if t.Pool == PoolPuzzle {
		// A puzzle room's reply names both, null or not.
		b, _ = json.MarshalIndent(struct {
			RoomReply
			Puzzle *PuzzleReply `json:"puzzle"`
			Trap   *TrapReply   `json:"trap"`
		}{r, r.Puzzle, r.Trap}, ``, `  `)
	} else {
		b, _ = json.MarshalIndent(r, ``, `  `)
	}
	return string(b)
}

// ---- the trigger and the bank ----

var (
	genMu   sync.Mutex
	genBusy = map[string]bool{} // profile/pool with a room being written
)

// genWait bounds one background generation; the module's own timeout is
// shorter.
var genWait = 3 * time.Minute

// genDir is where a profile's rooms live; tests point it elsewhere.
var genDir = func(p *Profile) string { return filepath.Join(DataDir(), `rooms`, p.Id) }

// startGen runs a generation on its own goroutine. Tests run it in line.
var startGen = func(job func(lock bool)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				mudlog.Error(`rifts.generate`, `panic`, r)
			}
		}()
		job(true)
	}()
}

// maybeGenerate is called as a room of pool is built, under the mud lock:
// at the generator's chance it asks for a new room of that pool in the
// background, on the key of a player in the run whose key may be used now.
// One room per profile and pool is written at a time, and none past the
// pool's cap. The room being built never waits for it.
func (run *Run) maybeGenerate(pool Pool) {
	g := installedGenerator()
	p := run.Profile
	if g == nil || !p.Generation.allows(pool) || len(run.Members) == 0 {
		return
	}
	if c := g.Chance(); c <= 0 || rng(100) >= c {
		return
	}
	if most := g.MaxPerPool(); most > 0 && p.generatedCount(pool) >= most {
		return
	}
	key := p.Id + `/` + string(pool)
	genMu.Lock()
	busy := genBusy[key]
	genMu.Unlock()
	if busy {
		return
	}
	userId := 0
	for _, uid := range run.memberIdsShuffled() {
		if g.Reserve(uid) {
			userId = uid
			break
		}
	}
	if userId == 0 {
		return
	}
	run.startGeneration(g, pool, userId)
}

// startGeneration writes a room of pool on userId's key (their turn already
// taken) in the background and banks it. Under the mud lock.
func (run *Run) startGeneration(g Generator, pool Pool, userId int) {
	p := run.Profile
	key := p.Id + `/` + string(pool)
	req := p.genRequest(pool)
	genMu.Lock()
	genBusy[key] = true
	genMu.Unlock()
	startGen(func(lock bool) {
		defer func() {
			genMu.Lock()
			delete(genBusy, key)
			genMu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), genWait)
		t, err := g.Generate(ctx, userId, req)
		cancel()
		if lock {
			util.LockMud()
			defer util.UnlockMud()
		}
		if err != nil {
			mudlog.Info(`rifts.generate`, `profile`, p.Id, `pool`, pool, `result`, `none`, `error`, err)
			return
		}
		if err := p.addGenerated(t); err != nil {
			mudlog.Warn(`rifts.generate`, `profile`, p.Id, `pool`, pool, `result`, `refused`, `error`, err)
			return
		}
		mudlog.Info(`rifts.generate`, `profile`, p.Id, `pool`, pool, `result`, `saved`, `id`, t.Id, `title`, t.Title)
	})
}

// generatedCount is how many of pool's rooms a model wrote.
func (p *Profile) generatedCount(pool Pool) int {
	n := 0
	for _, t := range p.templates[pool] {
		if t.Source == `generated` {
			n++
		}
	}
	return n
}

// addGenerated saves a generated room to the profile's bank and adds it to
// its pool, under the mud lock. The bank may have grown since the request
// was made, so the title is checked again here, and the id made unique.
func (p *Profile) addGenerated(t *Template) error {
	if t == nil || !p.Generation.allows(t.Pool) {
		return fmt.Errorf(`not a room this profile generates`)
	}
	t.Source = `generated`
	t.profileId = p.Id
	ids := map[string]bool{}
	for _, pool := range AllPools {
		for _, have := range p.templates[pool] {
			ids[have.Id] = true
			if have.Pool == t.Pool && strings.EqualFold(have.Title, t.Title) {
				return fmt.Errorf(`title %q is already used`, t.Title)
			}
		}
	}
	if err := t.validate(p, false); err != nil {
		return err
	}
	dir := genDir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	base := generatedId(t.Pool, t.Title)
	t.Id = base
	for i := 2; ids[t.Id] || fileExists(filepath.Join(dir, t.Id+`.yaml`)); i++ {
		t.Id = fmt.Sprintf(`%s-%d`, base, i)
	}
	if t.Created == `` {
		t.Created = time.Now().UTC().Format(time.RFC3339)
	}
	b, err := yaml.Marshal(t)
	if err != nil {
		return err
	}
	if err := util.Save(filepath.Join(dir, t.Id+`.yaml`), b); err != nil {
		return err
	}
	p.templates[t.Pool] = append(p.templates[t.Pool], t)
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// memberIdsShuffled is the run's members in a random order, so the turns of
// writing rooms fall on the party evenly.
func (run *Run) memberIdsShuffled() []int {
	ids := make([]int, 0, len(run.Members))
	for uid := range run.Members {
		ids = append(ids, uid)
	}
	sort.Ints(ids)
	shuffle(ids)
	return ids
}

// ReadProfiles loads every profile under dir and its rooms, structurally
// validated only (no world ids are checked): for tools and tests.
func ReadProfiles(dir string) (map[string]*Profile, error) {
	loaded, err := loadFrom(dir)
	if err != nil {
		return nil, err
	}
	for _, p := range loaded {
		if err := p.validate(false); err != nil {
			return nil, err
		}
	}
	return loaded, nil
}

// ErrNotGenerating is a forced generation that cannot run: no generator is
// installed, the profile does not generate that pool, or the player's key
// may not be used now.
var ErrNotGenerating = errors.New(`no room can be written now`)

// ForceGenerate asks for a new room of pool for the run userId is in, on
// their own key, whatever the chance (admin `rift gen`). Under the mud
// lock. The room is written in the background and saved like any other.
func ForceGenerate(userId int, pool Pool) error {
	g := installedGenerator()
	var run *Run
	for _, r := range runs {
		if r.Members[userId] {
			run = r
		}
	}
	if run == nil {
		return fmt.Errorf(`you are not in a rift`)
	}
	if g == nil {
		return fmt.Errorf(`%w: room writing is off`, ErrNotGenerating)
	}
	if !run.Profile.Generation.allows(pool) {
		return fmt.Errorf(`%w: %s does not generate pool %s`, ErrNotGenerating, run.Profile.Id, pool)
	}
	key := run.Profile.Id + `/` + string(pool)
	genMu.Lock()
	busy := genBusy[key]
	genMu.Unlock()
	if busy {
		return fmt.Errorf(`%w: a room of pool %s is already being written`, ErrNotGenerating, pool)
	}
	if !g.Reserve(userId) {
		return fmt.Errorf(`%w: your key may not write one now (relay down, "Make the world livelier" off, allowance spent, too soon, or no server key to moderate with)`, ErrNotGenerating)
	}
	run.startGeneration(g, pool, userId)
	return nil
}
