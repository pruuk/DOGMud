package aicompanion

import (
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// What a conversation leaves behind. A four-line exchange used to be able
// to leave four separate memories, one per reply, each a fragment of the
// same talk. That grows fast and remembers badly: a month later the
// companion holds a hundred half-lines about bread and nothing about the
// afternoon.
//
// Instead, talk is gathered into a conversation: an exchange in one room
// that keeps going while neither side leaves it for long. Anything the
// model wanted to remember mid-talk is held aside, not written. When the
// conversation ends, one cheap call turns the whole of it into a single
// memory in her own words, plus any facts worth keeping, and the held
// fragments are dropped. Only something genuinely weighty said mid-talk
// (importance 8 or more: a confession, a promise, a death) is written
// straight away, because it should survive even if the talk is cut short.
//
// The running lines are still there in her working memory while it happens,
// so she never loses the thread of what is being said.

// conversation is one exchange in progress.
type conversation struct {
	RoomId      int
	Partner     string
	StartUnix   int64
	LastUnix    int64
	Exchanges   int // turns from the other side
	Lines       []Line
	Provisional []Memory // what she thought worth remembering, pending the whole

	OwnerSpoke    bool        // her owner said something to her in it
	StrangerTurns map[int]int // turns each passer-by spoke in it, by user id
}

// payer is who the summary of this talk is charged to: the passer-by who
// said the most in it when it was passers-by alone, or 0 for her owner. A
// talk her owner took part in is the owner's, whoever else joined it; one
// with only a creature is her own business, which the owner's allowance
// pays for as always. The one who said the most pays, not the last to
// speak: otherwise one word at the end of somebody else's long talk would
// hand them the bill. A tie goes to the lower user id, so the choice does
// not depend on map order.
func (cv *conversation) payer() int {
	if cv.OwnerSpoke {
		return 0
	}
	best, most := 0, 0
	for id, n := range cv.StrangerTurns {
		if n > most || (n == most && id < best) {
			best, most = id, n
		}
	}
	return best
}

const maxConversationLines = 40

// noteConversation records one turn of talk. partner is empty for her own;
// partnerUserId is the player who spoke, 0 for her own turn or a creature.
func (m *AICompanionModule) noteConversation(c *controller, roomId int, partner string, partnerUserId int, l Line) {
	l.Text = capRunes(l.Text) // the whole talk goes out again in its summary
	now := time.Now().Unix()
	if c.convo != nil && (c.convo.RoomId != roomId || now-c.convo.LastUnix > int64(m.cfg.ConversationGapSeconds)) {
		m.closeConversation(c, `the talk moved on`)
	}
	if c.convo == nil {
		c.convo = &conversation{RoomId: roomId, Partner: partner, StartUnix: now}
	}
	if partner != `` {
		c.convo.Partner = partner
		c.convo.Exchanges++
	}
	switch {
	case partnerUserId <= 0:
	case partnerUserId == c.ownerUserId:
		c.convo.OwnerSpoke = true
	default:
		if c.convo.StrangerTurns == nil {
			c.convo.StrangerTurns = map[int]int{}
		}
		c.convo.StrangerTurns[partnerUserId]++
	}
	c.convo.LastUnix = now
	c.convo.Lines = append(c.convo.Lines, l)
	if len(c.convo.Lines) > maxConversationLines {
		c.convo.Lines = c.convo.Lines[len(c.convo.Lines)-maxConversationLines:]
	}
}

// holdMemory keeps what the model wanted to remember until the talk is
// over, unless it is weighty enough to stand on its own.
func (m *AICompanionModule) holdMemory(c *controller, mem Memory) bool {
	if c.convo == nil || mem.Importance >= 8 {
		return false
	}
	c.convo.Provisional = append(c.convo.Provisional, mem)
	if len(c.convo.Provisional) > 8 {
		c.convo.Provisional = c.convo.Provisional[len(c.convo.Provisional)-8:]
	}
	return true
}

// closeConversation ends an exchange and turns it into one memory. A short
// exchange (fewer than MinConversationExchanges turns) leaves only what was
// already worth keeping on its own.
func (m *AICompanionModule) closeConversation(c *controller, why string) {
	convo := c.convo
	c.convo = nil
	if convo == nil {
		return
	}
	now := time.Now().Unix()

	if convo.Exchanges < m.cfg.MinConversationExchanges || len(convo.Lines) < 2 {
		// Barely a conversation: keep the best of what she noted, if any.
		best := Memory{}
		for _, mem := range convo.Provisional {
			if mem.Importance > best.Importance {
				best = mem
			}
		}
		if best.Text != `` && best.Importance >= 6 {
			best.Unix = now
			c.mind.addMemory(best, m.cfg.MaxMemories)
			c.dirty = true
		}
		return
	}

	// A talk with a passer-by alone is summed up on their allowance, the
	// way their questions are answered on it, so it is not refused because
	// her owner's is spent, and never charged to it (reserveRoute).
	asker := convo.payer()
	// An owner whose own key was live this session pays for their own
	// companion's thinking, her summaries included. With their relay down
	// (at logout their browser is closing) the talk waits for it, as the
	// reflection does, rather than moving to the server's key.
	if m.cfg.ConversationSummaries && m.consented(c.ownerUserId) && c.relaySeen && m.route(c.ownerUserId).kind != routeRelay {
		m.deferSummary(c, convo, asker)
		return
	}
	if m.cfg.ConversationSummaries && m.consented(c.ownerUserId) && m.modelReadyFor(c.ownerUserId, asker) &&
		m.summariseConversation(c, convo, asker) {
		return
	}

	// No model to sum it up, or nobody's allowance to pay for it: keep the
	// single best note rather than the whole exchange.
	m.keepBestNote(c.mind, convo, now)
	c.dirty = true
}

// keepBestNote keeps the single best note of a talk that is not summed up.
func (m *AICompanionModule) keepBestNote(mind *Mind, convo *conversation, now int64) {
	best := Memory{}
	for _, mem := range convo.Provisional {
		if mem.Importance > best.Importance {
			best = mem
		}
	}
	if best.Text == `` {
		best = Memory{Kind: `conversation`, Importance: 3, Emotion: `neutral`,
			Text: fmt.Sprintf(`I talked with %s for a while.`, convo.Partner)}
	}
	best.Unix = now
	mind.addMemory(best, m.cfg.MaxMemories)
}

// deferredSummary is a finished talk of a relay owner's, kept until they
// are back with their relay up (deferSummary, startDueSummaries).
type deferredSummary struct {
	mind    *Mind
	profile *Profile
	convo   *conversation
	asker   int
}

// maxDeferredSummaries is how many talks wait per owner; the oldest is
// kept as its best note when a newer one pushes it out.
const maxDeferredSummaries = 4

// deferSummary keeps a relay owner's finished talk until startDueSummaries
// finds them online with their relay up. Called under the mud lock.
func (m *AICompanionModule) deferSummary(c *controller, convo *conversation, asker int) {
	if m.deferredSummaries == nil {
		m.deferredSummaries = map[int][]*deferredSummary{}
	}
	list := append(m.deferredSummaries[c.ownerUserId], &deferredSummary{mind: c.mind, profile: c.profile, convo: convo, asker: asker})
	for len(list) > maxDeferredSummaries {
		m.keepBestNote(list[0].mind, list[0].convo, list[0].convo.LastUnix)
		c.dirty = true
		list = list[1:]
	}
	m.deferredSummaries[c.ownerUserId] = list
}

// startDueSummaries sums up the owner's waiting talks once they are online
// with their relay up; a talk that still cannot be summed up (consent
// withdrawn, strangers off, nothing left to pay) keeps its best note. It
// is called from the round tick, under the mud lock, beside
// startDueReflection.
func (m *AICompanionModule) startDueSummaries(ownerId int) {
	list := m.deferredSummaries[ownerId]
	if len(list) == 0 || m.route(ownerId).kind != routeRelay {
		return
	}
	delete(m.deferredSummaries, ownerId)
	for _, d := range list {
		if m.consented(ownerId) && m.modelReadyFor(ownerId, d.asker) && m.summariseFor(d.mind, d.profile, ownerId, d.convo, d.asker) {
			continue
		}
		m.keepBestNote(d.mind, d.convo, d.convo.LastUnix)
		if c := m.ctrls[ownerId]; c != nil {
			c.dirty = true
		}
	}
}

// ConversationSummary is what the model gives back for a finished talk.
type ConversationSummary struct {
	Summary    string   `json:"summary"`
	Importance int      `json:"importance"`
	Emotion    string   `json:"emotion"`
	Facts      []string `json:"facts"`
}

func conversationSchema() map[string]any {
	return object([]string{`summary`, `importance`, `emotion`, `facts`}, map[string]any{
		`summary`:    str(`One or two sentences, in your own voice, on what the talk was about and how it went. "We talked about bread, and he told me his mother baked it." Not a transcript.`),
		`importance`: integer(`1 idle chat, 4 worth recalling, 7 significant, 10 unforgettable.`),
		`emotion`:    map[string]any{`type`: `string`, `enum`: emotions},
		`facts`:      map[string]any{`type`: `array`, `description`: `Anything you learned about them that is worth keeping. Usually none.`, `items`: map[string]any{`type`: `string`}},
	})
}

// summariseConversation turns a finished exchange into one memory, on the
// fast tier, off the game loop. asker is the passer-by who pays for it, or
// 0 for her owner. It reports whether the call was started; when it was
// not, the caller keeps the best note instead, so a talk is never lost to
// a budget.
func (m *AICompanionModule) summariseConversation(c *controller, convo *conversation, asker int) bool {
	return m.summariseFor(c.mind, c.profile, c.ownerUserId, convo, asker)
}

// summariseFor is summariseConversation for a mind and profile, so a talk
// kept for later (deferSummary) can be summed up without its session.
func (m *AICompanionModule) summariseFor(mind *Mind, p *Profile, ownerId int, convo *conversation, asker int) bool {
	// The whole talk is about to be posted: checked here as well as by the
	// caller, because this is the function that sends it.
	if !m.consented(ownerId) {
		return false
	}
	// A talk with passers-by alone is theirs to prompt, and her owner has
	// asked that they prompt nothing: the best note is kept instead.
	if !m.strangerMayPrompt(ownerId, asker) {
		return false
	}
	var b strings.Builder
	fmt.Fprintf(&b, "You have just finished talking with %s. Here is the whole of it, oldest first:\n", convo.Partner)
	for _, l := range convo.Lines {
		b.WriteString(formatLine(l, p.Name))
		b.WriteString("\n")
	}
	if len(convo.Provisional) > 0 {
		b.WriteString("\nWhat struck you while it was going on:\n")
		for _, mem := range convo.Provisional {
			fmt.Fprintf(&b, "- %s\n", mem.Text)
		}
	}
	b.WriteString("\nSum the whole talk up as one memory, the way you would remember it in a month.")

	ts := m.settingsFor(tierFast, false)
	messages := []chatMessage{
		{Role: `system`, Content: fmt.Sprintf("You are %s. %s\nYou are looking back on a conversation you have just had. Answer in your own voice, briefly, and never with a transcript.",
			p.Name, strings.TrimSpace(p.Summary))},
		{Role: `user`, Content: b.String()},
	}
	call := modelCall{
		BaseURL: m.baseURL(), APIKey: m.apiKey(), Model: ts.Model,
		Timeout: ts.Timeout, MaxTokens: ts.MaxTokens, Temperature: m.cfg.Temperature,
		Messages: messages, SchemaName: `companion_conversation`, Schema: conversationSchema(),
		Effort: ts.Effort, Retry: false, OwnerUserId: ownerId,
	}
	m.applyRoute(&call)
	rt := call.Route
	if rt.kind == routeNone || call.Model == `` || (asker > 0 && m.strangersOffOn(ownerId, rt)) {
		return false
	}
	reserved := worstCaseTokens(estimateTokens(call.Messages)+requestOverhead(call), ts.MaxTokens, 0, false)
	held, ok := m.reserveRoute(rt, ownerId, asker, reserved)
	if !ok {
		return false
	}
	m.countCall()
	key := mindIdentifier(mind.OwnerUserId, mind.MobId)
	partner := convo.Partner
	wipes := mind.Wipes
	place := convo.RoomId

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
				mudlog.Error(`aicompanion`, `action`, `conversationSummary`, `panic`, r, `stack`, string(debug.Stack()))
			}
			// It never reached applyConversationSummary, which settles
			// first thing: settle here, or the reservation is held all day.
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
		m.applyConversationSummary(key, call.OwnerUserId, wipes, partner, place, asker, held, rt, res)
	}()
	return true
}

// applyConversationSummary stores the one memory a talk left behind.
func (m *AICompanionModule) applyConversationSummary(key string, ownerId int, wipes int, partner string, placeId int, asker int, held hold, rt route, res modelResult) {
	// Settled first, against whoever the reservation was held against
	// (the owner the mind is keyed by, even when nobody is left to
	// remember it), so nothing below can leave it held.
	m.settleRoute(held, res.Tokens)
	m.rollCounters()
	m.recordCall(tierFast, res)
	m.routeResult(rt, ownerId, res.Ticket, res.Err, time.Now())

	mind := m.minds[key]
	if mind == nil || mind.Wipes != wipes {
		return // gone, or wiped at a parting since this was asked for
	}
	mind.TokensLifetime += int64(res.Tokens)
	if res.Err != nil {
		m.logModelError(res.Err)
		return
	}

	var sum ConversationSummary
	if err := parseJSONContent(res.Content, &sum); err != nil {
		m.logModelError(fmt.Errorf(`parse conversation summary: %w`, err))
		return
	}
	text := cleanText(sum.Summary, maxRememberRunes)
	if text == `` || breaksCharacter(text) {
		return
	}
	emotion := strings.ToLower(strings.TrimSpace(sum.Emotion))
	if !inSet(emotions, emotion) {
		emotion = `neutral`
	}
	now := time.Now().Unix()
	mind.addMemory(Memory{Unix: now, Kind: `conversation`, Text: text,
		Importance: clampInt(sum.Importance, 1, 10), Emotion: emotion,
		People: []string{partner}, PlaceId: placeId}, m.cfg.MaxMemories)
	// Facts are things she knows about her owner. What a stranger in a
	// tavern told her belongs in the memory of the talk, not there.
	if ownerName := ownerNameFor(mind); ownerName != `` && strings.EqualFold(partner, ownerName) {
		for i, f := range sum.Facts {
			if i >= 2 {
				break
			}
			if t := cleanText(f, maxFactRunes); t != `` && !breaksCharacter(t) {
				mind.addFact(Fact{Unix: now, Text: t, Source: `told`, Confidence: `medium`}, m.cfg.MaxFacts)
			}
		}
	}
	if c := m.ctrls[mind.OwnerUserId]; c != nil {
		c.dirty = true
	} else if err := saveMind(m.plug, mind); err != nil {
		mudlog.Error(`aicompanion`, `action`, `saveMind`, `owner`, mind.OwnerUserId, `error`, err)
	}
}

// ownerNameFor is the owner's character name, when they are online.
func ownerNameFor(mind *Mind) string {
	if u := users.GetByUserId(mind.OwnerUserId); u != nil && u.Character != nil {
		return u.Character.Name
	}
	return ``
}
