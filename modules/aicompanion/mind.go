package aicompanion

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/plugins"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// MindSchemaVersion is bumped whenever Mind's shape changes. Older files are
// migrated in migrateMind, never discarded.
//
//	1: recent lines, flat notes, mood.
//	2: episodic memories, facts, promises, opinion, session summaries.
//	3: impressions of NPCs and places, interaction memory, loot arrangement.
//	4: the companion's own map of visited rooms, and hearsay about places.
//	5: goals, shop and price memory, protected items, autonomy level.
//	6: the romance track, and her own description before any of it.
const MindSchemaVersion = 6

// Mind is everything the companion remembers and feels that the game itself
// does not already store. Mechanical state (inventory, skills, health, gold,
// location) is never kept here: DOGMud owns it and it is always read live.
//
// Concurrency: a Mind is only read or written while holding the mud lock.
// Model-call goroutines never touch one.
//
// Times are Unix seconds rather than time.Time so the file round-trips
// through any YAML library unchanged.
type Mind struct {
	SchemaVersion int    `yaml:"schema_version"`
	OwnerUserId   int    `yaml:"owner_user_id"`
	MobId         int    `yaml:"mob_id"`
	ProfileId     string `yaml:"profile_id"`

	Mood        string `yaml:"mood,omitempty"`
	MoodSetUnix int64  `yaml:"mood_set_unix,omitempty"`

	FirstMetUnix int64 `yaml:"first_met_unix,omitempty"`
	// FirstMetKept is the first meeting having been written down and her
	// introduction queued (firstMet, or keepFirstMeeting when her owner
	// agreed later), so agreeing again never repeats it.
	FirstMetKept bool  `yaml:"first_met_kept,omitempty"`
	LastSeenUnix int64 `yaml:"last_seen_unix,omitempty"`
	SessionCount int   `yaml:"session_count,omitempty"`

	// Working memory: recent conversation and events, word for word.
	RecentLines []Line `yaml:"recent_lines,omitempty"`

	// Long-term memory.
	Memories      []Memory  `yaml:"memories,omitempty"`
	NextMemoryId  int64     `yaml:"next_memory_id,omitempty"`
	Facts         []Fact    `yaml:"facts,omitempty"`
	Promises      []Promise `yaml:"promises,omitempty"`
	NextPromiseId int       `yaml:"next_promise_id,omitempty"`
	Summaries     []Summary `yaml:"summaries,omitempty"`

	// Opinion of the owner, and the audit trail of every change to it.
	Opinion    Opinion         `yaml:"opinion"`
	OpinionLog []OpinionChange `yaml:"opinion_log,omitempty"`

	// What it thinks of the people and places it has met (F5.9, F3.9),
	// what it has recently handled (F7.7), and the loot arrangement agreed
	// with the owner (F7.4).
	NPCs         map[int]*Impression `yaml:"npcs,omitempty"`
	Places       map[int]*Impression `yaml:"places,omitempty"`
	Interactions []Interaction       `yaml:"interactions,omitempty"`
	LootRule     string              `yaml:"loot_rule,omitempty"`

	// Its own map: only rooms it has stood in, only exits it has walked
	// (F11.1, F11.2), and what it has been told about places (F11.9).
	Map     map[int]*RoomRecord `yaml:"map,omitempty"`
	Hearsay []PlaceTip          `yaml:"hearsay,omitempty"`

	// OwnPhrases are the companion's own recent lines, across sessions, so
	// it can avoid repeating a greeting or a joke (F2.12).
	OwnPhrases []string `yaml:"own_phrases,omitempty"`

	// Purpose and economy (phase 5): goals, what shops sell and for how
	// much, items it will not part with, and how freely it may roam.
	Goals           []Goal              `yaml:"goals,omitempty"`
	NextGoalId      int                 `yaml:"next_goal_id,omitempty"`
	AmbitionsSeeded bool                `yaml:"ambitions_seeded,omitempty"`
	Shops           map[int]*ShopRecord `yaml:"shops,omitempty"`
	Protected       map[int]int         `yaml:"protected,omitempty"`
	ProtectedWhy    map[int]string      `yaml:"protected_why,omitempty"`
	Autonomy        string              `yaml:"autonomy,omitempty"` // close, normal, free
	AssistToRestore string              `yaml:"assist_to_restore,omitempty"`

	// Romance is its own track (see romance.go). BaseDescription is how she
	// looked before any of it, so the stage line can be reapplied cleanly.
	Romance      Romance      `yaml:"romance,omitempty"`
	CoreMemories []CoreMemory `yaml:"core_memories,omitempty"`
	// Wipes counts the times this mind was wiped at a parting
	// (forgetOwner). A model reply launched before a wipe carries the
	// count it saw, and is dropped if it changed: a reflection still in
	// flight when she leaves must not write her old memories back.
	Wipes int `yaml:"wipes,omitempty"`

	// protectedInstances are the exact keepsakes, by item uuid. Uuids are
	// not saved by the game, so this lasts a session and the counts in
	// Protected carry the rest.
	protectedInstances map[string]bool
	BaseDescription    string `yaml:"base_description,omitempty"`

	// FallenUntilUnix is when it is back on its feet after being killed,
	// kept across sessions so a relog cannot undo a death.
	FallenUntilUnix int64 `yaml:"fallen_until,omitempty"`

	// Courtship is her side of meeting this person in the Waystone Hollow,
	// before they set out together (hollow.go).
	Courtship Courtship `yaml:"courtship,omitempty"`

	DeathCount     int   `yaml:"death_count,omitempty"`
	LastDeathUnix  int64 `yaml:"last_death_unix,omitempty"`
	TokensLifetime int64 `yaml:"tokens_lifetime,omitempty"`

	// Notes is schema 1's flat memory list. It is read only to migrate it
	// into Memories and is always empty after load.
	Notes []Note `yaml:"notes,omitempty"`
}

// Line is one line of recent conversation or notable action, as the
// companion experienced it. Text is stored raw and quoted when prompted.
type Line struct {
	Unix    int64  `yaml:"t"`
	Speaker string `yaml:"who"`
	Kind    string `yaml:"kind"` // said, asked, emoted, event
	ToMe    bool   `yaml:"to_me,omitempty"`
	Text    string `yaml:"text"`
	// Plain is the line without what another player looks like or
	// carries, for a prompt that goes through her owner's own browser
	// (relaySafeLines). Empty when Text carries nothing of the kind.
	Plain string `yaml:"plain,omitempty"`
}

// Note is schema 1's long-term memory. Kept only for migration.
type Note struct {
	Unix int64  `yaml:"t"`
	Text string `yaml:"text"`
}

// Memory is one long-term, episodic memory.
type Memory struct {
	Id           int64    `yaml:"id"`
	Unix         int64    `yaml:"t"`
	Kind         string   `yaml:"kind"` // conversation, event, reflection, gift, attack, death, promise, note
	Text         string   `yaml:"text"`
	Importance   int      `yaml:"importance"` // 1 trivial .. 10 unforgettable
	Emotion      string   `yaml:"emotion,omitempty"`
	People       []string `yaml:"people,omitempty"`
	Place        string   `yaml:"place,omitempty"`
	PlaceId      int      `yaml:"place_id,omitempty"`
	RecalledUnix int64    `yaml:"recalled,omitempty"`
}

// Fact is something durable the companion knows about its owner.
type Fact struct {
	Unix       int64  `yaml:"t"`
	Text       string `yaml:"text"`
	Source     string `yaml:"source"`     // told, observed, overheard
	Confidence string `yaml:"confidence"` // high, medium, low
	// Quote is what the owner actually said in the moment the fact was
	// noted, when there was one. A fact with no quote behind it is the
	// companion's own reading of things, and is held less firmly.
	Quote string `yaml:"quote,omitempty"`
}

// Promise is a promise made by either side.
type Promise struct {
	Id           int    `yaml:"id"`
	Unix         int64  `yaml:"t"`
	By           string `yaml:"by"` // me, them
	Text         string `yaml:"text"`
	Status       string `yaml:"status"` // open, kept, broken
	ResolvedUnix int64  `yaml:"resolved,omitempty"`
}

// Impression is the companion's view of one NPC (keyed by mob template) or
// one place (keyed by room id).
type Impression struct {
	Name    string `yaml:"name"`
	Feeling string `yaml:"feeling,omitempty"` // like, dislike, wary, trust, distrust, neutral
	Note    string `yaml:"note,omitempty"`
	Unix    int64  `yaml:"t,omitempty"`
	Visits  int    `yaml:"visits,omitempty"` // places: times entered; NPCs: times met
}

// Interaction is one thing the companion did to something in the world.
type Interaction struct {
	Key  string `yaml:"key"`
	Verb string `yaml:"verb"`
	Unix int64  `yaml:"t"`
	OK   bool   `yaml:"ok"`
}

// Autonomy levels (F12.10), set in conversation.
const (
	autonomyClose  = `close`
	autonomyNormal = `normal`
	autonomyFree   = `free`
)

// Loot arrangements.
const (
	lootAskFirst   = `ask_first`
	lootTakeFreely = `take_freely`
	lootLeaveIt    = `leave_it`
)

// Summary is the companion's own account of one session.
type Summary struct {
	Unix    int64  `yaml:"t"`
	Session int    `yaml:"session"`
	Text    string `yaml:"text"`
}

func mindIdentifier(ownerUserId int, mobId int) string {
	return fmt.Sprintf(`mind-%d-%d`, ownerUserId, mobId)
}

func newMind(ownerUserId int, p *Profile) *Mind {
	return &Mind{
		SchemaVersion: MindSchemaVersion,
		OwnerUserId:   ownerUserId,
		MobId:         p.MobId,
		ProfileId:     p.Id,
		Mood:          `calm`,
		Opinion:       p.OpinionBaseline(),
		NPCs:          map[int]*Impression{},
		Places:        map[int]*Impression{},
		LootRule:      lootAskFirst,
		Map:           map[int]*RoomRecord{},
		Shops:         map[int]*ShopRecord{},
		Autonomy:      autonomyNormal,
	}
}

// loadMind reads a companion's mind from disk, or starts a fresh one. A
// corrupt file is quarantined by the plugin layer (ReadIntoStruct) and a
// fresh mind is used, which is logged loudly because it means lost memories.
// Callers go through AICompanionModule.getMind, which caches the pointer.
func loadMind(plug *plugins.Plugin, ownerUserId int, p *Profile) *Mind {
	m := &Mind{}
	err := plug.ReadIntoStruct(mindIdentifier(ownerUserId, p.MobId), m)
	if err != nil {
		if errors.Is(err, util.ErrStateAbsent) {
			return newMind(ownerUserId, p)
		}
		// The file was damaged and has been quarantined. Fall back to the
		// newest readable backup before giving up on the memories (F17.5).
		if b := loadNewestBackup(plug, ownerUserId, p); b != nil {
			mudlog.Error(`aicompanion`, `action`, `loadMind`, `owner`, ownerUserId, `profile`, p.Id,
				`error`, err, `message`, `restored from backup`, `session`, b.SessionCount)
			migrateMind(b, ownerUserId, p)
			return b
		}
		mudlog.Error(`aicompanion`, `action`, `loadMind`, `owner`, ownerUserId, `profile`, p.Id,
			`error`, err, `message`, `starting a fresh mind; the old one was quarantined and no backup was readable`)
		return newMind(ownerUserId, p)
	}
	migrateMind(m, ownerUserId, p)
	return m
}

// Backups (F17.6): three rotating copies per mind, written at the end of
// every BackupEverySessions-th session.
const mindBackups = 3

func backupIdentifier(ownerUserId int, mobId int, slot int) string {
	return fmt.Sprintf(`mind-%d-%d-bak%d`, ownerUserId, mobId, slot)
}

// saveMindBackup writes the mind to the backup slot for its session.
func saveMindBackup(plug *plugins.Plugin, m *Mind) error {
	return plug.WriteStruct(backupIdentifier(m.OwnerUserId, m.MobId, m.SessionCount%mindBackups), m)
}

// loadNewestBackup returns the readable backup with the most sessions.
func loadNewestBackup(plug *plugins.Plugin, ownerUserId int, p *Profile) *Mind {
	var best *Mind
	for slot := 0; slot < mindBackups; slot++ {
		b := &Mind{}
		if err := plug.ReadIntoStruct(backupIdentifier(ownerUserId, p.MobId, slot), b); err != nil {
			continue
		}
		if best == nil || b.SessionCount > best.SessionCount {
			best = b
		}
	}
	return best
}

func migrateMind(m *Mind, ownerUserId int, p *Profile) {
	if m.SchemaVersion < 2 {
		// Schema 1 had no opinion; start from the profile baseline.
		m.Opinion = p.OpinionBaseline()
		for _, n := range m.Notes {
			m.addMemory(Memory{Unix: n.Unix, Kind: `note`, Text: n.Text, Importance: 5}, 0)
		}
		m.Notes = nil
		m.SchemaVersion = 2
	}
	if m.SchemaVersion < MindSchemaVersion {
		m.SchemaVersion = MindSchemaVersion
	}
	if m.Shops == nil {
		m.Shops = map[int]*ShopRecord{}
	}
	if m.Autonomy == `` {
		m.Autonomy = autonomyNormal
	}
	if m.Romance.Stage == `` {
		m.Romance.Stage = romanceNone
	}
	if m.Map == nil {
		m.Map = map[int]*RoomRecord{}
	}
	if m.NPCs == nil {
		m.NPCs = map[int]*Impression{}
	}
	if m.Places == nil {
		m.Places = map[int]*Impression{}
	}
	if m.LootRule == `` {
		m.LootRule = lootAskFirst
	}
	// Identity fields always reflect where the file was loaded from.
	m.OwnerUserId = ownerUserId
	m.MobId = p.MobId
	m.ProfileId = p.Id
	if m.Mood == `` {
		m.Mood = `calm`
	}
	m.Opinion = m.Opinion.clamped()
}

func saveMind(plug *plugins.Plugin, m *Mind) error {
	return plug.WriteStruct(mindIdentifier(m.OwnerUserId, m.MobId), m)
}

// maxStoredRunes bounds any one piece of text kept in her mind. A player's
// words reach it whole (what they said, their emote, a name they chose),
// and each stored line goes out again in every prompt, so without a bound
// one long speech is paid for on every call until it is forgotten.
const maxStoredRunes = 300

// capRunes trims text to at most maxStoredRunes runes, cutting on a rune
// boundary, never inside a character.
func capRunes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxStoredRunes {
		return s // bytes bound runes: short enough either way
	}
	r := []rune(s)
	if len(r) <= maxStoredRunes {
		return s
	}
	return strings.TrimSpace(string(r[:maxStoredRunes]))
}

// addLine appends to recent memory. Words are kept apart from goings-on:
// what was said, asked or done at each other (kinds said, asked, emoted)
// has its own allowance, so a busy hour of searching, walking and looting
// can never push a conversation out of her head. Routine events (kind
// event) get a smaller allowance of their own.
func (m *Mind) addLine(l Line, max int) {
	l.Text = capRunes(l.Text)
	l.Plain = capRunes(l.Plain)
	if l.Text == `` {
		return
	}
	if l.Unix == 0 {
		l.Unix = time.Now().Unix()
	}
	m.RecentLines = append(m.RecentLines, l)
	m.trimLines(max)
}

// isTalk reports whether a line is something said or done between people,
// rather than the day's goings-on.
func isTalk(l Line) bool {
	switch l.Kind {
	case `said`, `asked`, `emoted`:
		return true
	}
	return false
}

// trimLines keeps the newest talk up to talkMax and the newest events up to
// a quarter of that (at least four), in the order they happened.
func (m *Mind) trimLines(talkMax int) {
	if talkMax <= 0 {
		return
	}
	eventMax := talkMax / 4
	if eventMax < 4 {
		eventMax = 4
	}
	talk, events := 0, 0
	keep := make([]bool, len(m.RecentLines))
	for i := len(m.RecentLines) - 1; i >= 0; i-- {
		if isTalk(m.RecentLines[i]) {
			if talk < talkMax {
				talk++
				keep[i] = true
			}
			continue
		}
		if events < eventMax {
			events++
			keep[i] = true
		}
	}
	out := make([]Line, 0, talk+events)
	for i, l := range m.RecentLines {
		if keep[i] {
			out = append(out, l)
		}
	}
	m.RecentLines = out
}

// lastLines returns up to n of the most recent lines.
func (m *Mind) lastLines(n int) []Line {
	if n <= 0 || len(m.RecentLines) == 0 {
		return nil
	}
	if len(m.RecentLines) <= n {
		return m.RecentLines
	}
	return m.RecentLines[len(m.RecentLines)-n:]
}

// linesSince returns the recent lines at or after a Unix time.
func (m *Mind) linesSince(unix int64) []Line {
	for i, l := range m.RecentLines {
		if l.Unix >= unix {
			return m.RecentLines[i:]
		}
	}
	return nil
}

// addMemory stores a memory, ignoring a near-duplicate of one of the last
// few, and prunes to max (0 = no pruning). Returns whether it was stored.
func (m *Mind) addMemory(mem Memory, max int) bool {
	mem.Text = capRunes(mem.Text)
	if mem.Text == `` {
		return false
	}
	if mem.Unix == 0 {
		mem.Unix = time.Now().Unix()
	}
	mem.Importance = clampInt(mem.Importance, 1, 10)

	lower := strings.ToLower(mem.Text)
	start := len(m.Memories) - 12
	if start < 0 {
		start = 0
	}
	for _, old := range m.Memories[start:] {
		if strings.ToLower(old.Text) == lower {
			return false
		}
	}

	m.NextMemoryId++
	mem.Id = m.NextMemoryId
	m.Memories = append(m.Memories, mem)
	if max > 0 {
		m.Memories = pruneMemories(m.Memories, max, mem.Unix)
	}
	return true
}

// addFact stores a fact about the owner, ignoring duplicates, capped at max.
func (m *Mind) addFact(f Fact, max int) bool {
	f.Text = capRunes(f.Text)
	if f.Text == `` {
		return false
	}
	lower := strings.ToLower(f.Text)
	for _, old := range m.Facts {
		if strings.ToLower(old.Text) == lower {
			return false
		}
	}
	if f.Unix == 0 {
		f.Unix = time.Now().Unix()
	}
	m.Facts = append(m.Facts, f)
	if max > 0 && len(m.Facts) > max {
		m.Facts = append([]Fact(nil), m.Facts[len(m.Facts)-max:]...)
	}
	return true
}

// addPromise records a new open promise and returns its id.
func (m *Mind) addPromise(by string, text string) int {
	text = capRunes(text)
	if text == `` {
		return 0
	}
	for _, p := range m.Promises {
		if p.Status == `open` && p.By == by && strings.EqualFold(p.Text, text) {
			return p.Id
		}
	}
	m.NextPromiseId++
	m.Promises = append(m.Promises, Promise{
		Id: m.NextPromiseId, Unix: time.Now().Unix(), By: by, Text: text, Status: `open`,
	})
	// Keep every open promise; keep only the 20 most recent resolved ones.
	resolved := 0
	for i := len(m.Promises) - 1; i >= 0; i-- {
		if m.Promises[i].Status == `open` {
			continue
		}
		resolved++
		if resolved > 20 {
			m.Promises = append(m.Promises[:i], m.Promises[i+1:]...)
		}
	}
	return m.NextPromiseId
}

// resolvePromise marks an open promise kept or broken. Returns the promise
// (by value) and whether it was found open.
func (m *Mind) resolvePromise(id int, status string) (Promise, bool) {
	for i := range m.Promises {
		if m.Promises[i].Id == id && m.Promises[i].Status == `open` {
			m.Promises[i].Status = status
			m.Promises[i].ResolvedUnix = time.Now().Unix()
			return m.Promises[i], true
		}
	}
	return Promise{}, false
}

func (m *Mind) openPromises() []Promise {
	var out []Promise
	for _, p := range m.Promises {
		if p.Status == `open` {
			out = append(out, p)
		}
	}
	return out
}

func (m *Mind) addSummary(s Summary, max int) {
	s.Text = strings.TrimSpace(s.Text)
	if s.Text == `` {
		return
	}
	m.Summaries = append(m.Summaries, s)
	if max > 0 && len(m.Summaries) > max {
		m.Summaries = append([]Summary(nil), m.Summaries[len(m.Summaries)-max:]...)
	}
}

// reflections returns up to n of the most recent reflection memories.
func (m *Mind) reflections(n int) []Memory {
	var out []Memory
	for i := len(m.Memories) - 1; i >= 0 && len(out) < n; i-- {
		if m.Memories[i].Kind == `reflection` {
			out = append(out, m.Memories[i])
		}
	}
	// Oldest first.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// recordInteraction remembers that the companion did something to a thing.
func (m *Mind) recordInteraction(key string, verb string, ok bool, nowUnix int64) {
	if key == `` {
		return
	}
	m.Interactions = append(m.Interactions, Interaction{Key: key, Verb: verb, Unix: nowUnix, OK: ok})
	if len(m.Interactions) > 200 {
		m.Interactions = append([]Interaction(nil), m.Interactions[len(m.Interactions)-200:]...)
	}
}

// lastInteraction returns when the companion last dealt with a thing, and
// how many of its attempts on it in the last day failed.
func (m *Mind) lastInteraction(key string) (lastUnix int64, recentFails int) {
	if m == nil {
		return 0, 0
	}
	for i := len(m.Interactions) - 1; i >= 0; i-- {
		in := m.Interactions[i]
		if in.Key != key {
			continue
		}
		if lastUnix == 0 {
			lastUnix = in.Unix
		}
		if !in.OK && lastUnix-in.Unix < 86400 {
			recentFails++
		}
	}
	return lastUnix, recentFails
}

// impressionOf returns the impression record for a key, creating it.
func impressionOf(store map[int]*Impression, id int, name string) *Impression {
	imp, ok := store[id]
	if !ok {
		imp = &Impression{Name: name, Feeling: `neutral`}
		store[id] = imp
	}
	if name != `` {
		imp.Name = name
	}
	return imp
}

// addOwnPhrase remembers something the companion said or did, keeping the
// most recent max.
func (m *Mind) addOwnPhrase(text string, max int) {
	text = capRunes(text)
	if text == `` {
		return
	}
	m.OwnPhrases = append(m.OwnPhrases, text)
	if max > 0 && len(m.OwnPhrases) > max {
		m.OwnPhrases = append([]string(nil), m.OwnPhrases[len(m.OwnPhrases)-max:]...)
	}
}
