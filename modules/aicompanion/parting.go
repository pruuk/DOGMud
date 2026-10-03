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

// Partings. When a companion goes back to the Waystone Hollow, what she had
// with that person does not come with her: her memories of them, the
// moments that mattered, what she learned about them, any romance, are
// wiped from the mind she kept of them (forgetOwner). What stays is one
// sentence, in her own voice, of what they were to her and how it ended
// ("He was kind to me until the night he left me to the wolves."). It is
// kept with her, in the roster, keyed by that player, and it is only ever
// read in the Hollow, when that same player comes to talk to her: it is
// how she knows them again. When they win her back it is deleted, and a
// new one is written the next time they part.
//
// How she feels about them (Opinion) is kept as well, because it is what
// the Hollow's rules weigh (wouldTravelWith): someone she left because she
// could not bear them does not start from nothing.

// Parting is what a companion keeps of someone she travelled with.
type Parting struct {
	Name string `yaml:"name"`          // their character name when they parted
	Text string `yaml:"text"`          // one sentence, in her own voice
	Unix int64  `yaml:"t"`             // when they parted
	Why  string `yaml:"why,omitempty"` // how it ended, as the server saw it
}

// Why a parting happened, as the server saw it. The leave paths pass their
// own words (the model's intent, or her reason for abandoning them).
const (
	partSentAway = `They sent me away.`
	partLapsed   = `They left me waiting so long that I came back here.`
	partReleased = `Our roads parted.`
	partCantBear = `I could not bear to travel with them any longer.`
)

// partingOf is what she keeps of a player, or nil.
func (m *AICompanionModule) partingOf(p *Profile, userId int) *Parting {
	if p == nil || userId <= 0 {
		return nil
	}
	if e := m.roster.Profiles[p.Id]; e != nil {
		return e.Partings[userId]
	}
	return nil
}

// forgetParting deletes what she kept of a player: they have won her back,
// and what happens now is new.
func (m *AICompanionModule) forgetParting(p *Profile, userId int) {
	if e := m.roster.Profiles[p.Id]; e != nil && e.Partings[userId] != nil {
		delete(e.Partings, userId)
		m.saveRoster()
	}
}

// rosterOwnerName is the name of the player a roster entry is held by: kept
// when she was claimed, else read from them online, else their account
// name, which is their character name in DOGMud.
func rosterOwnerName(e *rosterEntry) string {
	if e.OwnerName != `` {
		return e.OwnerName
	}
	if u := users.GetByUserId(e.Owner); u != nil && u.Character != nil {
		return u.Character.Name
	}
	if name, ok := users.NewUserIndex().FindByUserId(int64(e.Owner)); ok && name != `` {
		return name
	}
	return `someone`
}

// partWith writes what she keeps of the player she is leaving, wipes the
// rest of what she had with them, and asks the model, on that player's own
// route, to put the parting into one sentence of her own. Called from
// unclaim, under the mud lock, before the roster forgets who held her.
func (m *AICompanionModule) partWith(p *Profile, e *rosterEntry, why string) {
	ownerId := e.Owner
	if ownerId <= 0 {
		return
	}
	if strings.TrimSpace(why) == `` {
		why = partReleased
	}
	name := rosterOwnerName(e)
	now := time.Now().Unix()
	together := int64(0)
	if e.Since > 0 && now > e.Since {
		together = now - e.Since
	}
	mind := m.getMind(ownerId, p)

	pt := &Parting{Name: name, Unix: now, Why: why, Text: fallbackParting(mind.Opinion, why, together)}
	if e.Partings == nil {
		e.Partings = map[int]*Parting{}
	}
	e.Partings[ownerId] = pt

	// What the model gets to sum up, read before it is wiped.
	material := partingMaterial{
		Opinion: mind.Opinion, Together: together, Why: why,
		Core:  coreLines(mind, now),
		Lines: mind.lastLines(16),
	}
	for i := len(mind.Summaries) - 1; i >= 0 && len(material.Summaries) < 4; i-- {
		material.Summaries = append([]string{mind.Summaries[i].Text}, material.Summaries...)
	}

	forgetOwner(mind)
	// A reflection or talk summaries kept for their next session (a relay
	// owner's) would write the old memories back: they go too.
	if d := m.deferredReflect[ownerId]; d != nil && d.mind == mind {
		delete(m.deferredReflect, ownerId)
	}
	if list := m.deferredSummaries[ownerId]; len(list) > 0 {
		kept := list[:0]
		for _, s := range list {
			if s.mind != mind {
				kept = append(kept, s)
			}
		}
		if len(kept) == 0 {
			delete(m.deferredSummaries, ownerId)
		} else {
			m.deferredSummaries[ownerId] = kept
		}
	}
	if m.plug != nil {
		if err := saveMind(m.plug, mind); err != nil {
			mudlog.Error(`aicompanion`, `action`, `saveMind`, `owner`, ownerId, `error`, err)
		}
	}
	m.askParting(p, ownerId, name, pt.Unix, material)
}

// forgetOwner wipes what she had with one person from the mind she keeps
// of them: the conversation, every memory, the moments that mattered, what
// she learned about them, promises, romance, the loot arrangement, her
// courtship of them, keepsakes from them (her gear went back to them).
// What she knows of the world (her map, shops, places, people met, her own
// goals) is hers, and stays. So does how she feels about them (Opinion),
// which the Hollow weighs alongside the one sentence she keeps.
func forgetOwner(mind *Mind) {
	mind.Wipes++ // replies still in flight from before are dropped (Mind.Wipes)
	mind.Autonomy = autonomyNormal
	mind.AssistToRestore = ``
	mind.RecentLines = nil
	mind.Memories = nil
	mind.Facts = nil
	mind.Promises = nil
	mind.Summaries = nil
	mind.CoreMemories = nil
	mind.Romance = Romance{}
	mind.OwnPhrases = nil
	mind.LootRule = lootAskFirst
	mind.Courtship = Courtship{}
	mind.Protected = nil
	mind.ProtectedWhy = nil
	mind.protectedInstances = nil
	mind.FirstMetUnix = 0
	mind.FirstMetKept = false
}

// fallbackParting is the sentence she keeps when no model can put it in her
// own words: how long, how it ended, and how she felt about them by then.
// It does not name them (Parting.Name does), so nothing naming a player
// who never agreed to the model is written in her words.
func fallbackParting(o Opinion, why string, together int64) string {
	var feeling string
	switch mannerBand(o) {
	case mannerDisdain:
		feeling = `By the end I could not stand them.`
	case mannerCold:
		feeling = `By the end I did not much like them.`
	case mannerFriendly:
		feeling = `I liked them well enough.`
	case mannerWarm:
		feeling = `I was fond of them.`
	case mannerClose:
		feeling = `They mattered to me more than anyone.`
	default:
		feeling = `It was a working arrangement and not much more.`
	}
	how := `for a while`
	if together > 0 {
		how = `for ` + humanizeElapsed(together)
	}
	return capRunes(fmt.Sprintf(`We travelled together %s. %s %s`, how, strings.TrimSpace(why), feeling))
}

// partingMaterial is what she had with them, read before it was wiped.
type partingMaterial struct {
	Opinion   Opinion
	Together  int64
	Why       string
	Core      []string
	Summaries []string
	Lines     []Line
}

// PartingNote is the model's sentence.
type PartingNote struct {
	Text string `json:"text"`
}

func partingSchema() map[string]any {
	return object([]string{`text`}, map[string]any{
		`text`: str(`One sentence, in your own voice, on what they were to you and how it ended. Concrete, not a list of feelings: "He was kind to me until the night he left me to the wolves at Scrub Draw."`),
	})
}

// askParting asks the model, on the departing player's own route (their
// key, or the server's where that is allowed), for the sentence she keeps.
// The fallback is already in place; a failed or refused call keeps it.
func (m *AICompanionModule) askParting(p *Profile, ownerId int, name string, stamp int64, mat partingMaterial) {
	if !m.consented(ownerId) || !m.modelReady(ownerId) {
		return
	}
	ts := m.settingsFor(tierFast, false)
	call := modelCall{
		BaseURL: m.baseURL(), APIKey: m.apiKey(), Model: ts.Model,
		Timeout: ts.Timeout, MaxTokens: ts.MaxTokens, Temperature: m.cfg.Temperature,
		SchemaName: `companion_parting`, Schema: partingSchema(), Effort: ts.Effort,
		OwnerUserId: ownerId,
	}
	m.applyRoute(&call)
	rt := call.Route
	if rt.kind == routeNone || call.Model == `` {
		return
	}
	lines := mat.Lines
	if rt.kind == routeRelay {
		// The player reads this prompt in their own browser (relaySafeLines).
		lines = relaySafeLines(lines, name, p.Name)
	}
	call.Messages = buildPartingMessages(p, name, mat, lines)
	reserved := worstCaseTokens(estimateTokens(call.Messages)+requestOverhead(call), ts.MaxTokens, 0, false)
	held, ok := m.reserveRoute(rt, ownerId, 0, reserved)
	if !ok {
		return
	}
	m.countCall()

	go func() {
		var tk apiframework.Ticket
		call.ticketOut = &tk
		defer func() { m.fw().Release(apiframework.ConsumerCompanion, tk) }()
		applied, used := false, 0
		defer func() {
			if r := recover(); r != nil {
				mudlog.Error(`aicompanion`, `action`, `parting`, `panic`, r, `stack`, string(debug.Stack()))
			}
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
		m.applyParting(p.Id, ownerId, stamp, held, rt, res)
	}()
}

// buildPartingMessages is the prompt for the sentence she keeps.
func buildPartingMessages(p *Profile, name string, mat partingMaterial, lines []Line) []chatMessage {
	var b strings.Builder
	fmt.Fprintf(&b, "You have just parted ways with %s and gone back to wait in the Waystone Hollow.\n", name)
	fmt.Fprintf(&b, "How it ended: %s\n", strings.TrimSpace(mat.Why))
	if mat.Together > 0 {
		fmt.Fprintf(&b, "You travelled together for %s.\n", humanizeElapsed(mat.Together))
	}
	b.WriteString("\nHow you feel about them now:\n")
	for _, w := range opinionWords(mat.Opinion, name) {
		b.WriteString(w)
		b.WriteString("\n")
	}
	if len(mat.Core) > 0 {
		b.WriteString("\nThe moments that mattered most:\n")
		for _, l := range mat.Core {
			b.WriteString("- " + l + "\n")
		}
	}
	if len(mat.Summaries) > 0 {
		b.WriteString("\nWhat you remember of your time together:\n")
		for _, s := range mat.Summaries {
			b.WriteString("- " + s + "\n")
		}
	}
	if len(lines) > 0 {
		b.WriteString("\nThe last things that passed between you:\n")
		for _, l := range lines {
			b.WriteString(formatLine(l, p.Name))
			b.WriteString("\n")
		}
	}
	fmt.Fprintf(&b, "\nIn one sentence, in your own voice, say what %s was to you and how it ended. It is all you will keep of them: if they ever come looking for you again, it is what you will remember.", name)
	return []chatMessage{
		{Role: `system`, Content: fmt.Sprintf("You are %s. %s\nYou are putting someone you travelled with behind you. Be plain and concrete.",
			p.Name, strings.TrimSpace(p.Summary))},
		{Role: `user`, Content: b.String()},
	}
}

// applyParting puts the model's sentence in place of the fallback, if the
// parting it was asked for is still the one she keeps (they have not won
// her back, or parted again, since).
func (m *AICompanionModule) applyParting(profileId string, ownerId int, stamp int64, held hold, rt route, res modelResult) {
	m.settleRoute(held, res.Tokens)
	m.rollCounters()
	m.recordCall(tierFast, res)
	m.routeResult(rt, ownerId, res.Ticket, res.Err, time.Now())

	e := m.roster.Profiles[profileId]
	if e == nil || e.Partings[ownerId] == nil || e.Partings[ownerId].Unix != stamp {
		return
	}
	if res.Err != nil {
		m.logModelError(res.Err)
		return
	}
	var note PartingNote
	if err := parseJSONContent(res.Content, &note); err != nil {
		m.logModelError(fmt.Errorf(`parse parting: %w`, err))
		return
	}
	text := cleanText(note.Text, maxRememberRunes)
	if text == `` || breaksCharacter(text) {
		return
	}
	e.Partings[ownerId].Text = text
	m.saveRoster()
}
