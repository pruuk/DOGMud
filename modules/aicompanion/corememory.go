package aicompanion

import (
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Core memories. Ordinary memories come and go: they fade, they are pruned,
// a hundred of them are about bread. A core memory is one of the few things
// that changed what the two of them are to each other, and it is written
// only when that changes: when what is between them deepens, and when it is
// damaged. They are never pruned, they are few, and she brings one up now
// and then, unprompted, years later.
//
// What it was is not guessed at by the module: when the moment lands, the
// model is asked to say, in her own words, what actually happened, where
// they were, and whether it was good or bad.

// CoreMemory is one of the handful of moments that made them what they are.
type CoreMemory struct {
	Unix     int64  `yaml:"t"`
	Text     string `yaml:"text"`
	Place    string `yaml:"place,omitempty"`
	PlaceId  int    `yaml:"place_id,omitempty"`
	Positive bool   `yaml:"positive"`
	Stage    string `yaml:"stage,omitempty"` // what it became, or what it cost
	Told     int    `yaml:"told,omitempty"`  // times she has brought it up
	LastTold int64  `yaml:"last_told,omitempty"`
}

const maxCoreMemories = 24

// addCore stores a core memory. They are kept, not pruned; the oldest is
// only dropped when there are more than a life's worth.
func (m *Mind) addCore(cm CoreMemory) {
	cm.Text = capRunes(cm.Text)
	if cm.Text == `` {
		return
	}
	m.CoreMemories = append(m.CoreMemories, cm)
	if len(m.CoreMemories) > maxCoreMemories {
		m.CoreMemories = m.CoreMemories[len(m.CoreMemories)-maxCoreMemories:]
	}
}

// coreLines are the handful she carries, for the prompt. They go in every
// request: these are the things she is, not things she might recall.
func coreLines(mind *Mind, nowUnix int64) []string {
	var out []string
	for _, cm := range mind.CoreMemories {
		mark := `for the better`
		if !cm.Positive {
			mark = `for the worse`
		}
		where := ``
		if cm.Place != `` {
			where = ` at ` + cm.Place
		}
		out = append(out, fmt.Sprintf(`(%s ago%s, %s) %s`, humanizeElapsed(nowUnix-cm.Unix), where, mark, cm.Text))
	}
	return out
}

// CoreMemoryNote is what the model gives back when something between them
// has changed. Only the words: whether it was for better or worse is
// settled by what the server saw happen.
type CoreMemoryNote struct {
	Text string `json:"text"`
}

func coreSchema() map[string]any {
	return object([]string{`text`}, map[string]any{
		`text`: str(`One or two sentences, in your own voice, on what actually happened just now between you: what they did or said, and what it meant. Name the thing itself, not the feeling alone. "You gave me the last of the bread in the dark under Thornwall, and said nothing about it."`),
	})
}

// recordCore asks the model what just happened and keeps the answer for
// good. Called when the romance between them moves either way.
func (m *AICompanionModule) recordCore(c *controller, ownerName string, stage string, positive bool) {
	// A core memory names her owner, so, as with every deed, nothing is
	// written before they have agreed (mayRemember).
	if !m.mayRemember(c) {
		return
	}
	mob := mobs.GetInstance(c.instanceId)
	place, placeId := ``, 0
	if mob != nil {
		placeId = mob.Character.RoomId
		if room := rooms.LoadRoom(placeId); room != nil && !cannotSee(mob, room) {
			place = strings.TrimSpace(room.Title)
		}
	}
	now := time.Now().Unix()

	// No model to put words to it: keep the bare fact, which is still worth
	// more than nothing. A call that fails keeps it too (applyCore).
	bare := CoreMemory{Unix: now, Text: fmt.Sprintf(`Something changed between %s and me here.`, ownerName),
		Place: place, PlaceId: placeId, Positive: positive, Stage: stage}
	if !positive {
		bare.Text = fmt.Sprintf(`Something between %s and me was spoiled here.`, ownerName)
	}
	bareFact := func() {
		c.mind.addCore(bare)
		c.dirty = true
	}
	if !m.consented(c.ownerUserId) || !m.modelReady(c.ownerUserId) {
		bareFact()
		return
	}
	ts := m.settingsFor(tierFast, false)

	call := modelCall{
		BaseURL: m.baseURL(), APIKey: m.apiKey(), Model: ts.Model,
		Timeout: ts.Timeout, MaxTokens: ts.MaxTokens, Temperature: m.cfg.Temperature,
		SchemaName: `companion_core_memory`, Schema: coreSchema(), Effort: ts.Effort,
		OwnerUserId: c.ownerUserId,
	}
	m.applyRoute(&call)
	rt := call.Route
	if rt.kind == routeNone || call.Model == `` {
		bareFact()
		return
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Something has just changed between you and %s", ownerName)
	if place != `` {
		fmt.Fprintf(&b, ", here at %s", place)
	}
	if positive {
		fmt.Fprintf(&b, ": it has become %s.\n", stage)
	} else {
		b.WriteString(": it has been set back, or ended.\n")
	}
	b.WriteString("\nWhat led to it, most recent last:\n")
	lines := c.mind.lastLines(16)
	if rt.kind == routeRelay {
		// Her owner reads this prompt in their browser (relaySafeLines).
		lines = relaySafeLines(lines, ownerName, c.profile.Name)
	}
	for _, l := range lines {
		b.WriteString(formatLine(l, c.profile.Name))
		b.WriteString("\n")
	}
	b.WriteString("\nSay what happened, in your own words, the way you will still tell it years from now.")

	messages := []chatMessage{
		{Role: `system`, Content: fmt.Sprintf("You are %s. %s\nYou are setting down one of the few moments you will never forget. Be plain and concrete about what was done or said.",
			c.profile.Name, strings.TrimSpace(c.profile.Summary))},
		{Role: `user`, Content: b.String()},
	}
	call.Messages = messages
	reserved := worstCaseTokens(estimateTokens(call.Messages)+requestOverhead(call), ts.MaxTokens, 0, false)
	held, ok := m.reserveRoute(rt, c.ownerUserId, 0, reserved)
	if !ok {
		// The day's allowance cannot cover it: the moment is still kept.
		bareFact()
		return
	}
	m.countCall()
	key := mindIdentifier(c.mind.OwnerUserId, c.mind.MobId)
	wipes := c.mind.Wipes

	go func() {
		// The breakers' leave is always handed back last: a no-op once its
		// outcome was recorded, and what frees a half-open breaker's probe
		// if a panic kept the outcome from ever being recorded.
		var tk apiframework.Ticket
		call.ticketOut = &tk
		defer func() { m.fw().Release(apiframework.ConsumerCompanion, tk) }()
		applied, used := false, 0
		defer func() {
			if r := recover(); r != nil {
				mudlog.Error(`aicompanion`, `action`, `coreMemory`, `panic`, r, `stack`, string(debug.Stack()))
			}
			// It never reached applyCore, which settles first thing.
			if !applied {
				util.LockMud()
				defer util.UnlockMud()
				m.settleRoute(held, used)
			}
		}()
		res := m.callModel(call)
		used = res.Tokens

		util.LockMud()
		defer util.UnlockMud()
		applied = true
		m.applyCore(key, call.OwnerUserId, wipes, bare, held, rt, res)
	}()
}

// applyCore writes the model's account of the moment. cm arrives holding
// the bare fact, which is what is kept when the model gives no usable
// account: the moment is never lost to a failed call.
func (m *AICompanionModule) applyCore(key string, ownerId int, wipes int, cm CoreMemory, held hold, rt route, res modelResult) {
	// Settled first, so nothing below can leave the reservation held.
	m.settleRoute(held, res.Tokens)
	m.rollCounters()
	m.recordCall(tierFast, res)
	m.routeResult(rt, ownerId, res.Ticket, res.Err, time.Now())

	mind := m.minds[key]
	if mind == nil || mind.Wipes != wipes {
		return // gone, or wiped at a parting since this was asked for
	}
	keepBare := func() {
		if cm.Text == `` {
			return
		}
		mind.addCore(cm)
		if c := m.ctrls[mind.OwnerUserId]; c != nil {
			c.dirty = true
		} else if err := saveMind(m.plug, mind); err != nil {
			mudlog.Error(`aicompanion`, `action`, `saveMind`, `owner`, mind.OwnerUserId, `error`, err)
		}
	}
	if res.Err != nil {
		m.logModelError(res.Err)
		keepBare()
		return
	}
	var note CoreMemoryNote
	if err := parseJSONContent(res.Content, &note); err != nil {
		m.logModelError(fmt.Errorf(`parse core memory: %w`, err))
		keepBare()
		return
	}
	text := cleanText(note.Text, maxRememberRunes)
	if text == `` || breaksCharacter(text) {
		keepBare()
		return
	}
	cm.Text = text
	// Whether it was for better or worse is the server's, from what
	// actually happened. The model tells what happened, not what it meant.
	mind.addCore(cm)
	// It is also an ordinary memory, so retrieval can surface it in talk.
	mind.addMemory(Memory{Unix: cm.Unix, Kind: `event`, Text: text, Importance: 10,
		Emotion: map[bool]string{true: `affection`, false: `hurt`}[cm.Positive], PlaceId: cm.PlaceId, Place: cm.Place}, m.cfg.MaxMemories)
	if c := m.ctrls[mind.OwnerUserId]; c != nil {
		c.dirty = true
	} else if err := saveMind(m.plug, mind); err != nil {
		mudlog.Error(`aicompanion`, `action`, `saveMind`, `owner`, mind.OwnerUserId, `error`, err)
	}
}

// pickCoreToTell chooses one core memory she might bring up out of
// nowhere: not one she has just told, and the older and less-told ones
// first.
func (mind *Mind) pickCoreToTell(nowUnix int64) *CoreMemory {
	best := -1
	bestScore := 0.0
	for i := range mind.CoreMemories {
		cm := &mind.CoreMemories[i]
		if nowUnix-cm.LastTold < 3*86400 {
			continue
		}
		score := 1.0/float64(cm.Told+1) + float64(nowUnix-cm.Unix)/(30*86400)
		if score > bestScore {
			best, bestScore = i, score
		}
	}
	if best < 0 {
		return nil
	}
	return &mind.CoreMemories[best]
}
