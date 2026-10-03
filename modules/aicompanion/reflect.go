package aicompanion

import (
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// End-of-session reflection (F1.6, F3.8, F3.12). When the owner leaves, the
// companion looks back over the session in private: a short account of it,
// at most two higher-level conclusions, and any facts about the owner it
// missed at the time. Nothing here is spoken; it only shapes what the
// companion remembers and believes next time.

// Reflection is the model's reply to a reflection request.
type Reflection struct {
	Summary     string   `json:"summary"`
	Conclusions []string `json:"conclusions"`
	Facts       []string `json:"facts"`
	Mood        string   `json:"mood"`
}

func reflectionSchema() map[string]any {
	return object(
		[]string{`summary`, `conclusions`, `facts`, `mood`},
		map[string]any{
			`summary`: str(`Two or three sentences, in your own voice, about what happened today and how it went between you. Empty if nothing happened worth recalling.`),
			`conclusions`: map[string]any{
				`type`:        `array`,
				`description`: `Zero to two broader conclusions you draw about them, yourself or the two of you. Only real insights.`,
				`items`:       map[string]any{`type`: `string`},
			},
			`facts`: map[string]any{
				`type`:        `array`,
				`description`: `Facts about them you learned today and have not already noted. Usually empty.`,
				`items`:       map[string]any{`type`: `string`},
			},
			`mood`: map[string]any{`type`: `string`, `enum`: moods},
		},
	)
}

// reflectionInput is the plain data a reflection is built from.
type reflectionInput struct {
	Profile     *Profile
	OwnerName   string
	Opinion     Opinion
	Facts       []Fact
	Reflections []Memory
	Lines       []Line
	Session     int
}

func buildReflectionMessages(in reflectionInput) []chatMessage {
	p := in.Profile
	var sys strings.Builder
	fmt.Fprintf(&sys, "You are %s, a person in the world of Gaius. %s\n", p.Name, strings.TrimSpace(p.Summary))
	writeList(&sys, `Personality`, p.Personality)
	fmt.Fprintf(&sys, "Your travelling companion %s has gone to rest. Alone, you think back over the time you just spent together.\n", in.OwnerName)
	sys.WriteString("Think in character. Be honest and specific; do not invent events that did not happen. You know nothing of computers, games or artificial minds.\n")

	var usr strings.Builder
	for _, w := range opinionWords(in.Opinion, in.OwnerName) {
		usr.WriteString(w)
		usr.WriteString("\n")
	}
	if len(in.Facts) > 0 {
		fmt.Fprintf(&usr, "\nWhat you already know about %s:\n", in.OwnerName)
		for _, f := range in.Facts {
			fmt.Fprintf(&usr, "- %s\n", f.Text)
		}
	}
	if len(in.Reflections) > 0 {
		usr.WriteString("\nConclusions you had already drawn:\n")
		for _, r := range in.Reflections {
			fmt.Fprintf(&usr, "- %s\n", r.Text)
		}
	}
	usr.WriteString("\nWhat happened this time (oldest first; quoted text is exactly what was said or done):\n")
	for _, l := range in.Lines {
		usr.WriteString(formatLine(l, p.Name))
		usr.WriteString("\n")
	}
	usr.WriteString("\nReflect.")
	return []chatMessage{
		{Role: `system`, Content: sys.String()},
		{Role: `user`, Content: usr.String()},
	}
}

// deferredReflection is one session's reflection, taken as the session
// ended and started later. It is a copy, so a new session writing to the
// same Mind does not change what the old one is remembered as.
type deferredReflection struct {
	mind    *Mind
	in      reflectionInput
	session int
}

// detachReflection is the reflection at the end of a session: started at
// once, or, for an owner who used their own key this session, kept until
// they are back with their relay up. At logout their browser is closing,
// so a call sent to it now would only fail.
//
// A server that also has its own key could run a relay owner's reflection
// on it at logout instead. It deliberately does not: an owner on their own
// key pays for their own companion, her private thoughts included, and the
// server's budget is kept for the owners who have no key of their own.
func (m *AICompanionModule) detachReflection(c *controller, ownerName string) {
	owner := c.mind.OwnerUserId
	if c.relaySeen || m.route(owner).kind == routeRelay {
		m.deferReflection(c.mind, c.profile, ownerName, c.sessionStartUnix)
		return
	}
	m.startReflection(c.mind, c.profile, ownerName, c.sessionStartUnix)
}

// deferReflection keeps a relay owner's reflection until startDueReflection
// finds them online with their relay up. One waits per owner: a newer
// session's replaces an older one's. Called under the mud lock.
func (m *AICompanionModule) deferReflection(mind *Mind, p *Profile, ownerName string, sessionStart int64) {
	in, ok := m.prepareReflection(mind, p, ownerName, sessionStart)
	if !ok {
		return
	}
	if m.deferredReflect == nil {
		m.deferredReflect = map[int]*deferredReflection{}
	}
	m.deferredReflect[mind.OwnerUserId] = &deferredReflection{mind: mind, in: in, session: mind.SessionCount}
}

// dueReflection takes the owner's waiting reflection when it may start:
// the owner is online and their relay is live. A relay that comes back
// while they are logged out starts nothing. Called under the mud lock.
func (m *AICompanionModule) dueReflection(ownerId int, online bool) *deferredReflection {
	d := m.deferredReflect[ownerId]
	if d == nil || !online || m.route(ownerId).kind != routeRelay {
		return nil
	}
	delete(m.deferredReflect, ownerId)
	return d
}

// startDueReflection starts the owner's waiting reflection if it is due.
// It is called from the round tick for an owner who is online, under the
// mud lock, never from the connection goroutine a relay's Ready arrives
// on. The launch passes consent again, and the call the consent door.
func (m *AICompanionModule) startDueReflection(ownerId int) {
	if d := m.dueReflection(ownerId, true); d != nil {
		m.launchReflection(d)
	}
}

// startReflection launches a background reflection for a mind whose owner
// has just left. Called under the mud lock from detach. The mind pointer
// stays in the module's cache, so if the owner returns before the reply
// lands, both the new session and the reflection work on the same Mind.
func (m *AICompanionModule) startReflection(mind *Mind, p *Profile, ownerName string, sessionStart int64) {
	in, ok := m.prepareReflection(mind, p, ownerName, sessionStart)
	if !ok {
		return
	}
	m.launchReflection(&deferredReflection{mind: mind, in: in, session: mind.SessionCount})
}

// prepareReflection gathers what a session's reflection is built from, as
// it stands now, or reports that there is to be none.
func (m *AICompanionModule) prepareReflection(mind *Mind, p *Profile, ownerName string, sessionStart int64) (reflectionInput, bool) {
	// Consent covers everything that leaves the server, not only what is
	// said in the moment: a player who declined must not have their session
	// posted to OpenAI the instant they log out.
	if !m.consented(mind.OwnerUserId) || !m.cfg.ReflectOnLogout {
		return reflectionInput{}, false
	}
	lines := mind.linesSince(sessionStart)
	if len(lines) < m.cfg.MinSessionLinesForReflection {
		return reflectionInput{}, false
	}
	if len(lines) > 80 {
		lines = lines[len(lines)-80:]
	}
	return reflectionInput{
		Profile:     p,
		OwnerName:   ownerName,
		Opinion:     mind.Opinion,
		Facts:       append([]Fact(nil), mind.Facts...),
		Reflections: append([]Memory(nil), mind.reflections(3)...),
		Lines:       append([]Line(nil), lines...),
		Session:     mind.SessionCount,
	}, true
}

// launchReflection starts a prepared reflection's call. Called under the
// mud lock.
func (m *AICompanionModule) launchReflection(d *deferredReflection) {
	mind, in := d.mind, d.in
	// Asked again at launch: a deferred one may start long after it was
	// taken, and the owner may have withdrawn in between.
	if !m.consented(mind.OwnerUserId) || !m.cfg.ReflectOnLogout || !m.modelReady(mind.OwnerUserId) {
		return
	}
	ts := m.settingsFor(tierDeep, false)
	call := modelCall{
		BaseURL:     m.baseURL(),
		APIKey:      m.apiKey(),
		Model:       ts.Model,
		Timeout:     ts.Timeout,
		MaxTokens:   ts.MaxTokens,
		Temperature: m.cfg.Temperature,
		Messages:    buildReflectionMessages(in),
		SchemaName:  `companion_reflection`,
		Schema:      reflectionSchema(),
		Effort:      ts.Effort,
		Retry:       m.cfg.RetryTransient,
		OwnerUserId: mind.OwnerUserId,
	}
	m.applyRoute(&call)
	rt := call.Route
	if rt.kind == routeNone || call.Model == `` {
		return
	}
	if rt.kind == routeRelay {
		// Her owner reads this prompt in their browser (relaySafeLines).
		in.Lines = relaySafeLines(in.Lines, in.OwnerName, in.Profile.Name)
		call.Messages = buildReflectionMessages(in)
	}
	reserved := worstCaseTokens(estimateTokens(call.Messages)+requestOverhead(call), ts.MaxTokens, 0, call.Retry)
	held, ok := m.reserveRoute(rt, mind.OwnerUserId, 0, reserved)
	if !ok {
		return // the day's thinking is spent; the session simply goes unrecorded
	}
	m.countCall()
	key := mindIdentifier(mind.OwnerUserId, mind.MobId)
	session := d.session // the session reflected on, not the one running now
	wipes := mind.Wipes

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
				mudlog.Error(`aicompanion`, `action`, `reflection`, `panic`, r, `stack`, string(debug.Stack()))
			}
			// It never reached applyReflection, which settles first thing.
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
		m.applyReflection(key, call.OwnerUserId, session, wipes, call.Model, held, rt, res)
	}()
}

// applyReflection stores a reflection. Runs under the mud lock.
func (m *AICompanionModule) applyReflection(key string, ownerId int, session int, wipes int, model string, held hold, rt route, res modelResult) {
	// Settled first, to the owner it was held against, even when nobody is
	// left to remember it, so nothing below can leave it held.
	m.settleRoute(held, res.Tokens)
	m.rollCounters()
	m.recordCall(tierDeep, res)
	m.routeResult(rt, ownerId, res.Ticket, res.Err, time.Now())
	// A model the player's provider refused says nothing about the
	// server's choice of models.
	if rt.kind == routeServer && modelRefused(res) {
		m.models.refuse(model)
	}

	mind := m.minds[key]
	if mind == nil || mind.Wipes != wipes {
		return // gone, or wiped at a parting since this was asked for
	}
	mind.TokensLifetime += int64(res.Tokens)
	if res.Err != nil {
		m.logModelError(res.Err)
		return
	}
	var r Reflection
	if err := parseJSONContent(res.Content, &r); err != nil {
		m.logModelError(fmt.Errorf(`parse reflection: %w`, err))
		return
	}

	now := time.Now().Unix()
	if text := cleanText(r.Summary, 600); text != `` && !breaksCharacter(text) {
		mind.addSummary(Summary{Unix: now, Session: session, Text: text}, m.cfg.MaxSummaries)
	}
	for i, c := range r.Conclusions {
		if i >= 2 {
			break
		}
		if text := cleanText(c, maxRememberRunes); text != `` && !breaksCharacter(text) {
			mind.addMemory(Memory{Unix: now, Kind: `reflection`, Text: text, Importance: 7, Emotion: `neutral`}, m.cfg.MaxMemories)
		}
	}
	for i, f := range r.Facts {
		if i >= 3 {
			break
		}
		if text := cleanText(f, maxFactRunes); text != `` && !breaksCharacter(text) {
			// These come out of her own thinking afterwards, not from
			// anything the owner said in the moment, so they are held as
			// her reading of things and never as "they told me".
			mind.addFact(Fact{Unix: now, Text: text, Source: `inferred`, Confidence: `low`}, m.cfg.MaxFacts)
		}
	}
	if mood := strings.ToLower(strings.TrimSpace(r.Mood)); isMood(mood) {
		mind.Mood = mood
		mind.MoodSetUnix = now
	}

	if err := saveMind(m.plug, mind); err != nil {
		mudlog.Error(`aicompanion`, `action`, `saveMind`, `owner`, mind.OwnerUserId, `error`, err)
	}
	if c := m.ctrls[mind.OwnerUserId]; c != nil && c.mind == mind {
		c.dirty = false
	}
	if m.cfg.LogDecisions {
		mudlog.Info(`aicompanion`, `action`, `reflection`, `owner`, mind.OwnerUserId, `session`, session,
			`conclusions`, len(r.Conclusions), `facts`, len(r.Facts), `tokens`, res.Tokens)
	}
}
